package main

import (
	"fmt"
	"log"
	"merchantcache/internal/abr"
	"merchantcache/internal/config"
	"merchantcache/internal/data"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env
	_ = godotenv.Load()

	// Load configuration from environment
	cfg := config.LoadFromEnv()

	// Initialize ABR client
	abrClient := abr.NewClient(cfg.ABRGuid, cfg.ABREndpoint, cfg.Timeout)
	fmt.Println("✓ ABN Registry (ABR) client initialized\n")

	// Initialize data processor
	processor := data.NewProcessor(cfg.OutputFile)

	// Process each merchant
	merchants := cfg.GetMerchants()
	fmt.Printf("Processing %d merchants with ABN lookup only...\n\n", len(merchants))

	for i, merchant := range merchants {
		fmt.Printf("[%2d/%d] %s\n", i+1, len(merchants), merchant)

		// Lookup ABN using merchant name
		fmt.Printf("    [ABN Lookup] Using merchant name: %s\n", merchant)
		
		abn, abnState, abnLegalName, score := abrClient.Lookup(merchant)
		
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
		fmt.Printf("      Legal Name: %s\n", abnLegalName)
		fmt.Printf("      State: %s\n", abnState)

		// Add result
		processor.AddResult(data.Result{
			MerchantName: merchant,
			ABN:          abn,
			State:        abnState,
			LegalName:    abnLegalName,
			Score:        score,
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
