package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	guid := os.Getenv("ABR_GUID")
	endpoint := os.Getenv("ABR_ENDPOINT")
	businessName := "Bunnings"

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

	resp, err := client.Get(endpoint + "?" + params.Encode())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading body: %v\n", err)
		return
	}

	// Save to file
	err = os.WriteFile("/tmp/abr_bunnings.xml", body, 0644)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("Saved %d bytes to /tmp/abr_bunnings.xml\n", len(body))

	// Show some content
	content := string(body)
	if len(content) > 2000 {
		fmt.Printf("First 2000 chars:\n%s\n\n", content[:2000])
		fmt.Printf("Last 500 chars:\n%s\n", content[len(content)-500:])
	} else {
		fmt.Printf("Full content:\n%s\n", content)
	}
}
