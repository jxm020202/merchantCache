package main

import (
	"fmt"
	"log"
	"merchantcache/internal/abr"
	"merchantcache/internal/config"
	"merchantcache/internal/data"
	"merchantcache/internal/google"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env
	_ = godotenv.Load()

	// Load configuration from environment
	cfg := config.LoadFromEnv()

	// Initialize Google Search client for address lookup
	googleClient, err := google.NewClient(
		cfg.GoogleAPIKey,
		cfg.GoogleSearchEngineID,
		cfg.GoogleClientID,
		cfg.GoogleClientSecret,
		cfg.Timeout,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Google Custom Search API: %v", err)
	}
	fmt.Println("✓ Google Custom Search API initialized")

	// Initialize ABR client
	abrClient := abr.NewClient(cfg.ABRGuid, cfg.ABREndpoint, cfg.Timeout)
	fmt.Println("✓ ABN Registry (ABR) client initialized\n")

	// Initialize data processor
	processor := data.NewProcessor(cfg.OutputFile)

	// Process each merchant
	merchants := cfg.GetMerchants()
	fmt.Printf("Processing %d merchants - ABN lookup + Head Office address search...\n\n", len(merchants))

	for i, merchant := range merchants {
		fmt.Printf("[%2d/%d] %s\n", i+1, len(merchants), merchant)

		// Lookup ABN using merchant name
		fmt.Printf("    [ABN Lookup] Using merchant name: %s\n", merchant)
		
		abn, acn, abnState, abnLegalName, score := abrClient.Lookup(merchant)
		
		if abn == "" {
			fmt.Printf("      ✗ ABN not found\n")
			processor.AddResult(data.Result{
				MerchantName: merchant,
				LegalName:    merchant,
			})
			fmt.Println()
			continue
		}

		fmt.Printf("      ✓ ABN Found: %s\n", abn)
		if acn != "" {
			fmt.Printf("      ✓ ACN Found: %s\n", acn)
		}
		fmt.Printf("      Legal Name: %s\n", abnLegalName)
		fmt.Printf("      State: %s\n", abnState)

		// Search for head office address using Google Custom Search
		fmt.Printf("    [Address Lookup] Searching for head office address...\n")
		address, err := googleClient.SearchHeadOfficeAddress(merchant, abnLegalName)
		if err != nil {
			fmt.Printf("      ✗ Address lookup failed: %v\n", err)
		} else if address != "" {
			fmt.Printf("      ✓ Address Found: %s\n", address)
		} else {
			fmt.Printf("      ✗ No address found\n")
		}

		// Add result
		processor.AddResult(data.Result{
			MerchantName: merchant,
			ABN:          abn,
			ACN:          acn,
			State:        abnState,
			LegalName:    abnLegalName,
			Score:        score,
			Address:      address,
			Verified:     true,
			Confidence:   100.0,
		})

		fmt.Println()
	}

	// Save results
	outputPath, err := processor.SaveToFile()
	if err != nil {
		log.Fatalf("Failed to save results: %v", err)
	}

	fmt.Printf("✓ Results saved to: %s\n", outputPath)

	// Print summary
	processor.PrintSummary()
}
