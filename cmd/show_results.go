package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"merchantcache/internal/abr"
	"merchantcache/internal/config"
)

func main() {
	_ = godotenv.Load()

	cfg := config.LoadFromEnv()
	client := abr.NewClient(cfg.ABRGuid, cfg.ABREndpoint, cfg.Timeout)

	merchants := cfg.GetMerchants()

	fmt.Println("ABN Lookup Results")
	fmt.Println("=" + string([]byte{61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61, 61}))

	for _, merchant := range merchants {
		abn, state, legalName, score := client.Lookup(merchant)

		fmt.Printf("\n%-25s | ABN: %-15s | State: %-4s | Score: %-3s\n", merchant, abn, state, score)
		fmt.Printf("%-25s | Legal Name: %s\n", "", legalName)
		fmt.Println("-" + string([]byte{45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45, 45}))
	}
}
