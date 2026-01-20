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
