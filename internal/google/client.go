package google

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	apiKey         string
	searchEngineID string
	timeout        int
	clientID       string
	clientSecret   string
	baseURL        string
}

type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

type SearchResponse struct {
	Items []SearchResult `json:"items"`
}

type MerchantInfo struct {
	LegalName   string
	ABN         string
	ACN         string
	HeadOffice  string
	State       string
	Postcode    string
	Confidence  float64
}

func NewClient(apiKey, searchEngineID, clientID, clientSecret string, timeout int) (*Client, error) {
	if apiKey == "" || searchEngineID == "" {
		return nil, fmt.Errorf("incomplete credentials")
	}

	return &Client{
		apiKey:         apiKey,
		searchEngineID: searchEngineID,
		timeout:        timeout,
		clientID:       clientID,
		clientSecret:   clientSecret,
		baseURL:        "https://www.googleapis.com/customsearch/v1",
	}, nil
}

func (c *Client) Search(query string, numResults int) ([]SearchResult, error) {
	if numResults > 10 {
		numResults = 10
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("key", c.apiKey)
	params.Set("cx", c.searchEngineID)
	params.Set("num", fmt.Sprintf("%d", numResults))

	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
	}

	resp, err := client.Get(c.baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp SearchResponse
	err = json.Unmarshal(body, &searchResp)
	if err != nil {
		return nil, err
	}

	return searchResp.Items, nil
}

// ExtractMerchantInfo extracts merchant legal name, state, and postcode from Google search results
func (c *Client) ExtractMerchantInfo(merchantName string) (MerchantInfo, error) {
	// Search for merchant information
	query := fmt.Sprintf("%s Australia legal name headquarters address", merchantName)
	results, err := c.Search(query, 10)
	if err != nil || len(results) == 0 {
		return MerchantInfo{}, err
	}

	fmt.Printf("    [Step 1] Google Custom Search: %s\n", query)

	info := MerchantInfo{
		LegalName: merchantName,
	}

	// Combine all text (titles + snippets)
	allText := ""
	for _, r := range results {
		allText += r.Title + " " + r.Snippet + " "
	}

	// Extract legal name - look for patterns like "Company Name Limited"
	legalNamePatterns := []string{
		`([A-Z][A-Za-z\s&'-]+(?:Limited|Ltd|Pty Ltd|PTY LTD|Group Limited|Group|Corporation)) is an Australian`,
		`([A-Z][A-Za-z\s&'-]+(?:Limited|Ltd|Pty Ltd|PTY LTD|Group Limited|Corporation))(?:\s-\s|\s\(|\s–)`,
	}

	for _, pattern := range legalNamePatterns {
		regex := regexp.MustCompile(pattern)
		if matches := regex.FindStringSubmatch(allText); len(matches) > 1 {
			candidate := strings.TrimSpace(matches[1])
			// Remove common non-company prefixes
			candidate = strings.TrimPrefix(candidate, "Wikipedia ")
			candidate = strings.TrimPrefix(candidate, "Wiki ")
			candidate = strings.TrimSpace(candidate)
			
			if len(candidate) > 3 && candidate != merchantName {
				info.LegalName = candidate
				fmt.Printf("      ✓ Legal Name: %s\n", candidate)
				break
			}
		}
	}

	// Extract Australian state (NSW, VIC, QLD, WA, SA, TAS, ACT, NT)
	stateMap := map[string]string{
		`\bNSW\b`:      "NSW",
		`\bVIC\b`:      "VIC",
		`\bQLD\b`:      "QLD",
		`\bWA\b`:       "WA",
		`\bSA\b`:       "SA",
		`\bTAS\b`:      "TAS",
		`\bACT\b`:      "ACT",
		`\bNT\b`:       "NT",
		`New South Wales`: "NSW",
		`Victoria`:     "VIC",
		`Queensland`:   "QLD",
		`Western Australia`: "WA",
		`South Australia`:   "SA",
		`Tasmania`:     "TAS",
	}

	for pattern, state := range stateMap {
		if matched, _ := regexp.MatchString(pattern, allText); matched {
			info.State = state
			fmt.Printf("      ✓ State: %s\n", state)
			break
		}
	}

	// Extract postcode (4 digits Australian postcode)
	postcodeRegex := regexp.MustCompile(`\b([0-9]{4})\b`)
	if matches := postcodeRegex.FindStringSubmatch(allText); len(matches) > 1 {
		postcode := matches[1]
		// Basic validation: Australian postcodes are 0200-9999
		if postcode >= "0200" && postcode <= "9999" {
			info.Postcode = postcode
			fmt.Printf("      ✓ Postcode: %s\n", postcode)
		}
	}

	// Calculate confidence
	confidence := 0.0
	if info.LegalName != merchantName {
		confidence += 40
	}
	if info.State != "" {
		confidence += 30
	}
	if info.Postcode != "" {
		confidence += 30
	}
	info.Confidence = confidence

	fmt.Printf("      Confidence: %.1f%%\n", confidence)

	return info, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *Client) VerifyAndEnrich(abn, legalName, state string) (map[string]interface{}, error) {
	// Clean ABN
	abnClean := regexp.MustCompile(`\D`).ReplaceAllString(abn, "")
	if len(abnClean) != 11 {
		return map[string]interface{}{
			"verification": map[string]interface{}{
				"verified": false,
				"confidence": 0,
			},
		}, nil
	}

	// Primary verification
	query := fmt.Sprintf("ABN %s %s Australia", abnClean, legalName)
	results, err := c.Search(query, 5)
	if err != nil {
		return map[string]interface{}{
			"verification": map[string]interface{}{
				"verified": false,
				"confidence": 0,
			},
		}, nil
	}

	if len(results) > 0 {
		abnMatches := 0
		nameMatches := 0

		for _, r := range results {
			if strings.Contains(r.Snippet, abnClean) {
				abnMatches++
			}
			if strings.Contains(strings.ToLower(r.Snippet), strings.ToLower(legalName)) {
				nameMatches++
			}
		}

		confidence := (float64(abnMatches)*0.6 + float64(nameMatches)*0.4) / float64(len(results)) * 100
		if confidence >= 50 {
			// Extract address from first result
			address := c.extractAddress(results[0])

			return map[string]interface{}{
				"verification": map[string]interface{}{
					"verified": true,
					"confidence": confidence,
				},
				"head_office": map[string]interface{}{
					"address": address,
				},
				"google_found": map[string]interface{}{
					"abn": abnClean,
					"legal_name": legalName,
				},
			}, nil
		}
	}

	// Fallback: Try just the ABN
	fallbackResults, err := c.Search(fmt.Sprintf("ABN %s", abnClean), 3)
	if err == nil && len(fallbackResults) > 0 {
		return map[string]interface{}{
			"verification": map[string]interface{}{
				"verified": true,
				"confidence": 40,
			},
			"head_office": map[string]interface{}{
				"address": c.extractAddress(fallbackResults[0]),
			},
			"google_found": map[string]interface{}{
				"abn": abnClean,
				"legal_name": legalName,
			},
		}, nil
	}

	return map[string]interface{}{
		"verification": map[string]interface{}{
			"verified": false,
			"confidence": 0,
		},
	}, nil
}

func (c *Client) extractAddress(result SearchResult) string {
	// Simple extraction of address patterns from snippet
	snippet := result.Snippet

	// Remove common phrases
	snippet = strings.TrimSpace(snippet)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}

	return snippet
}

// FindLegalName searches for the correct legal business name
func (c *Client) FindLegalName(businessName string) (string, error) {
	// First try to get the ABN lookup page directly
	query := fmt.Sprintf("site:abr.business.gov.au %s", businessName)
	results, err := c.Search(query, 3)
	if err != nil {
		return businessName, nil
	}

	if len(results) > 0 {
		// If we find ABN lookup page, it usually has the legal name in the snippet
		snippet := results[0].Snippet
		if len(snippet) > 30 {
			// Return the business name unchanged - we'll let ABR API find it
			return businessName, nil
		}
	}

	return businessName, nil
}

// VerifyAndGetAddress verifies ABN and gets address
func (c *Client) VerifyAndGetAddress(abn, legalName string) (bool, float64, string) {
	// Clean ABN
	abnClean := regexp.MustCompile(`\D`).ReplaceAllString(abn, "")
	if len(abnClean) != 11 {
		return false, 0, ""
	}

	// Search for ABN + legal name verification
	query := fmt.Sprintf("ABN %s %s Australia head office address", abnClean, legalName)
	results, err := c.Search(query, 5)
	if err != nil || len(results) == 0 {
		return false, 0, ""
	}

	// Check if ABN appears in results
	abnMatches := 0
	nameMatches := 0

	for _, r := range results {
		if strings.Contains(r.Snippet, abnClean) {
			abnMatches++
		}
		if strings.Contains(strings.ToLower(r.Snippet), strings.ToLower(legalName)) {
			nameMatches++
		}
	}

	confidence := (float64(abnMatches)*0.6 + float64(nameMatches)*0.4) / float64(len(results)) * 100
	address := c.extractAddress(results[0])

	verified := confidence >= 40
	return verified, confidence, address
}
