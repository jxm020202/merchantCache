package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/joho/godotenv"
	"os"
)

func main() {
	_ = godotenv.Load()

	guid := os.Getenv("ABR_GUID")
	endpoint := os.Getenv("ABR_ENDPOINT")
	businessName := "bunnings"

	fmt.Printf("Testing ABR API\n")
	fmt.Printf("GUID: %s\n", guid)
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Printf("Business: %s\n\n", businessName)

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
	params.Set("authenticationGuid", guid)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	fullURL := endpoint + "?" + params.Encode()
	fmt.Printf("Full URL: %s\n\n", fullURL)

	resp, err := client.Get(fullURL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status Code: %d\n\n", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading body: %v\n", err)
		return
	}

	fmt.Printf("Response Body:\n%s\n", string(body[:min(1000, len(body))]))
	if len(body) > 1000 {
		fmt.Printf("... (truncated, total %d bytes)\n", len(body))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
