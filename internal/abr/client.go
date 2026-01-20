package abr

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Client struct {
	guid     string
	endpoint string
	timeout  int
}

type SearchResultsRecord struct {
	ABN struct {
		IdentifierValue  string `xml:"identifierValue"`
		IdentifierStatus string `xml:"identifierStatus"`
	} `xml:"ABN"`
	MainBusinessPhysicalAddress struct {
		StateCode string `xml:"stateCode"`
	} `xml:"mainBusinessPhysicalAddress"`
	BusinessName struct {
		OrganisationName string `xml:"organisationName"`
		Score            string `xml:"score"`
	} `xml:"businessName"`
	MainName struct {
		OrganisationName string `xml:"organisationName"`
	} `xml:"mainName"`
	MainTradingName struct {
		OrganisationName string `xml:"organisationName"`
		Score            string `xml:"score"`
	} `xml:"mainTradingName"`
}

type ABRResponse struct {
	Records []SearchResultsRecord `xml:"searchResultsRecord"`
}

func NewClient(guid, endpoint string, timeout int) *Client {
	return &Client{
		guid:     guid,
		endpoint: endpoint,
		timeout:  timeout,
	}
}

func (c *Client) searchByName(businessName string) (string, error) {
	params := url.Values{}
	params.Set("name", businessName)
	params.Set("postcode", "")
	params.Set("legalName", "Y")
	params.Set("tradingName", "Y")
	params.Set("NSW", "Y")
	params.Set("VIC", "Y")
	params.Set("QLD", "Y")
	params.Set("WA", "Y")
	params.Set("SA", "Y")
	params.Set("NT", "Y")
	params.Set("ACT", "Y")
	params.Set("TAS", "Y")
	params.Set("authenticationGuid", c.guid)

	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
	}

	resp, err := client.Get(c.endpoint + "?" + params.Encode())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (c *Client) getAllResults(xmlText string) []Result {
	if xmlText == "" {
		return nil
	}

	var response ABRResponse
	err := xml.Unmarshal([]byte(xmlText), &response)
	if err != nil {
		return nil
	}

	var results []Result
	abnRegex := regexp.MustCompile(`^\d{11}$`)

	for _, rec := range response.Records {
		abn := strings.TrimSpace(rec.ABN.IdentifierValue)
		status := strings.TrimSpace(rec.ABN.IdentifierStatus)

		if !abnRegex.MatchString(abn) || status != "Active" {
			continue
		}

		state := strings.TrimSpace(rec.MainBusinessPhysicalAddress.StateCode)
		legalName := strings.TrimSpace(rec.BusinessName.OrganisationName)

		if legalName == "" {
			legalName = strings.TrimSpace(rec.MainName.OrganisationName)
		}
		if legalName == "" {
			legalName = strings.TrimSpace(rec.MainTradingName.OrganisationName)
		}

		score := strings.TrimSpace(rec.BusinessName.Score)
		if score == "" {
			score = strings.TrimSpace(rec.MainTradingName.Score)
		}

		results = append(results, Result{
			ABN:       abn,
			State:     state,
			LegalName: legalName,
			Score:     score,
		})
	}

	return results
}

func (c *Client) findBestResult(businessName string, results []Result) Result {
	if len(results) == 0 {
		return Result{}
	}

	searchLower := strings.ToLower(strings.TrimSpace(businessName))
	searchWords := stringToSet(strings.Fields(searchLower))

	companyKeywords := []string{"pty", "limited", "ltd", "inc", "corporation", "corp", "group", "holding"}
	unrelatedKeywords := []string{"cleaning", "freight", "toners", "candles", "music", "ads", "dogwash"}

	type scoredResult struct {
		score  float64
		result Result
	}

	var scoredResults []scoredResult

	for _, result := range results {
		nameLower := strings.ToLower(result.LegalName)
		resultWords := stringToSet(strings.Fields(nameLower))

		// Must be company entity
		isCompany := false
		for _, keyword := range companyKeywords {
			if strings.Contains(nameLower, keyword) {
				isCompany = true
				break
			}
		}
		if !isCompany {
			continue
		}

		// Check for common words
		commonWords := intersection(searchWords, resultWords)
		if len(commonWords) == 0 {
			continue
		}

		// Check for unrelated business type
		hasUnrelated := false
		for _, keyword := range unrelatedKeywords {
			if strings.Contains(nameLower, keyword) {
				hasUnrelated = true
				break
			}
		}
		if hasUnrelated && len(commonWords) < 2 {
			continue
		}

		// Calculate score
		scoreValue := 50.0
		if scoreInt, err := fmt.Sscanf(result.Score, "%f", &scoreValue); err == nil {
			_ = scoreInt
		}

		exactMatch := 0.0
		if searchLower == nameLower {
			exactMatch = 1000
		}

		containsMatch := 0.0
		if strings.Contains(searchLower, nameLower) || strings.Contains(nameLower, searchLower) {
			containsMatch = 500
		}

		wordMatch := float64(len(commonWords)) * 100

		totalScore := exactMatch + containsMatch + wordMatch + scoreValue
		scoredResults = append(scoredResults, scoredResult{totalScore, result})
	}

	if len(scoredResults) == 0 {
		return Result{}
	}

	// Find max score
	maxScore := scoredResults[0].score
	maxResult := scoredResults[0].result
	for _, sr := range scoredResults[1:] {
		if sr.score > maxScore {
			maxScore = sr.score
			maxResult = sr.result
		}
	}

	return maxResult
}

func (c *Client) Lookup(businessName string) (abn, state, legalName, score string) {
	// Stage 1: Initial search
	xmlResponse, err := c.searchByName(businessName)
	if err != nil {
		return
	}

	allResults := c.getAllResults(xmlResponse)
	bestResult := c.findBestResult(businessName, allResults)

	if bestResult.ABN == "" {
		return
	}

	// Stage 2: Verification search
	if bestResult.LegalName != "" {
		xmlResponse2, err := c.searchByName(bestResult.LegalName)
		if err == nil {
			allResults2 := c.getAllResults(xmlResponse2)

			// Look for exact ABN match
			for _, result := range allResults2 {
				if result.ABN == bestResult.ABN {
					bestResult = result
					break
				}
			}
		}
	}

	return bestResult.ABN, bestResult.State, bestResult.LegalName, bestResult.Score
}

type Result struct {
	ABN       string
	State     string
	LegalName string
	Score     string
}

func stringToSet(strs []string) map[string]bool {
	m := make(map[string]bool)
	for _, s := range strs {
		m[s] = true
	}
	return m
}

func intersection(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for key := range a {
		if b[key] {
			result[key] = true
		}
	}
	return result
}
