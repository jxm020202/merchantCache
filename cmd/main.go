package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

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

	// Initialize ABR client
	abrClient := abr.NewClient(cfg.ABRGuid, cfg.ABREndpoint, cfg.Timeout)

	// Initialize Google Search client
	var googleClient *google.Client
	if cfg.EnableVerification {
		var err error
		googleClient, err = google.NewClient(
			cfg.GoogleAPIKey,
			cfg.GoogleSearchEngineID,
			cfg.GoogleClientID,
			cfg.GoogleClientSecret,
			cfg.Timeout,
		)
		if err == nil {
			fmt.Println("✓ Google Custom Search API initialized\n")
		} else {
			fmt.Printf("⚠️  Google verification disabled: %v\n\n", err)
			googleClient = nil
		}
	}

	// Initialize data processor
	processor := data.NewProcessor(cfg.OutputFile)

	// Process each merchant
	merchants := cfg.GetMerchants()
	fmt.Printf("Processing %d merchants...\n\n", len(merchants))
	fmt.Println("Architecture: ABN Lookup → Google Verification → Address Lookup → Output\n")

	for i, merchant := range merchants {
		// Normalize merchant name
		brandName := merchant

		// STEP 1: ABN Lookup
		abn, state, legalName, score := abrClient.Lookup(brandName)

		// STEP 2: Google Verification
		verified := false
		confidence := 0.0
		address := ""
		googleABN := ""
		googleLegalName := ""

		if googleClient != nil && abn != "" {
			enriched, err := googleClient.VerifyAndEnrich(abn, legalName, state)
			if err == nil {
				verification := enriched["verification"].(map[string]interface{})
				verified = verification["verified"].(bool)
				confidence = verification["confidence"].(float64)

				if headOffice, ok := enriched["head_office"].(map[string]interface{}); ok {
					if addr, ok := headOffice["address"].(string); ok {
						address = addr
					}
				}

				if googleFound, ok := enriched["google_found"].(map[string]interface{}); ok {
					if ga, ok := googleFound["abn"].(string); ok {
						googleABN = ga
					}
					if gn, ok := googleFound["legal_name"].(string); ok {
						googleLegalName = gn
					}
				}
			}
		}

		// STEP 4: Store enriched result
		processor.AddResult(data.Result{
			MerchantName:    merchant,
			ABN:             abn,
			State:           state,
			LegalName:       legalName,
			Score:           score,
			Verified:        verified,
			Confidence:      confidence,
			Address:         address,
			GoogleABN:       googleABN,
			GoogleLegalName: googleLegalName,
		})

		// Progress indicator
		abnStatus := "✓"
		if abn == "" {
			abnStatus = "✗"
		}

		verifyStatus := "○"
		if verified {
			verifyStatus = "✓"
		} else if abn == "" {
			verifyStatus = "—"
		}

		addrStatus := "—"
		if address != "" {
			addrStatus = "✓"
		}

		fmt.Printf("[%2d/%d] ABN:%s Verify:%s Addr:%s %-30s\n", i+1, len(merchants), abnStatus, verifyStatus, addrStatus, merchant)
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
