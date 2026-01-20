package main

import (
	"fmt"
	"log"
	"merchantcache/internal/abr"
	"merchantcache/internal/config"
	"merchantcache/internal/google"
	"merchantcache/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := config.LoadFromEnv()

	// Initialize clients
	googleClient, err := google.NewClient(cfg.GoogleAPIKey, cfg.GoogleSearchEngineID, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.Timeout)
	if err != nil {
		log.Fatalf("Failed to initialize Google client: %v", err)
	}

	abrClient := abr.NewClient(cfg.ABRGuid, cfg.ABREndpoint, cfg.Timeout)

	// Start web server
	srv := server.NewServer(googleClient, abrClient, "enriched_merchants_demo.csv")
	if err := srv.LoadResults(); err != nil {
		log.Fatalf("Failed to load results: %v", err)
	}

	fmt.Println("🌐 Web Dashboard Started")
	fmt.Println("Open: http://localhost:8080")
	fmt.Println("Press Ctrl+C to stop")

	if err := srv.Start("8080"); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
