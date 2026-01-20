package main

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL          string
	BrandfetchAPIKey     string
	BrandfetchClientID   string
	TransactionsFilePath string
	CountryTLDPreference string
}

func loadConfig() (Config, error) {
	cfg := Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		BrandfetchAPIKey:     os.Getenv("BRANDFETCH_API_KEY"),
		BrandfetchClientID:   os.Getenv("BRANDFETCH_CLIENT_ID"),
		TransactionsFilePath: getenvDefault("TRANSACTIONS_FILE", "transactions.txt"),
		CountryTLDPreference: getenvDefault("COUNTRY_TLD_PREFERENCE", ".au"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("DATABASE_URL is required")
	}
	if cfg.BrandfetchAPIKey == "" {
		return cfg, errors.New("BRANDFETCH_API_KEY is required")
	}
	if cfg.BrandfetchClientID == "" {
		return cfg, errors.New("BRANDFETCH_CLIENT_ID is required")
	}
	return cfg, nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
