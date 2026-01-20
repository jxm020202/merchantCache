package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"merchantcache/internal/abr"
)

func main() {
	_ = godotenv.Load()

	guid := os.Getenv("ABR_GUID")
	endpoint := os.Getenv("ABR_ENDPOINT")

	client := abr.NewClient(guid, endpoint, 10)

	fmt.Println("Testing: Bunnings")
	results := client.GetAllResults("bunnings")
	fmt.Printf("Results: %v\n", results)

	fmt.Println("\nTesting: Woolworths")
	results = client.GetAllResults("Woolworths")
	fmt.Printf("Results: %v\n", results)
}
