package config

import (
	"os"
	"strconv"
)

type Config struct {
	ABRGuid              string
	ABREndpoint          string
	Timeout              int
	GoogleAPIKey         string
	GoogleSearchEngineID string
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURI    string
	OutputFile           string
	EnableVerification   bool
}

func LoadFromEnv() Config {
	return Config{
		ABRGuid:              os.Getenv("ABR_GUID"),
		ABREndpoint:          os.Getenv("ABR_ENDPOINT"),
		Timeout:              parseIntOrDefault(os.Getenv("TIMEOUT"), 5),
		GoogleAPIKey:         os.Getenv("GOOGLE_API_KEY"),
		GoogleSearchEngineID: os.Getenv("GOOGLE_SEARCH_ENGINE_ID"),
		GoogleClientID:       os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:   os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:    getOrDefault(os.Getenv("GOOGLE_REDIRECT_URI"), "http://localhost:8080/callback"),
		OutputFile:           getOrDefault(os.Getenv("OUTPUT_FILE"), "enriched_merchants_demo.csv"),
		EnableVerification:   os.Getenv("ENABLE_VERIFICATION") != "false",
	}
}

func (c Config) GetMerchants() []string {
	return []string{
		"Afterpay",
		"Woolworths",
		"McDonald's",
		"Coles",
		"Apple",
		"Uber Eats",
		"Uber",
		"7-Eleven",
		"Kmart",
		"IGA",
		"KFC",
		"DoorDash",
		"Sportsbet",
		"ATM Cash Out",
		"Ampol",
		"ALDI",
		"PayPal",
		"BP",
		"Transport for NSW",
		"ATM Operator Fee",
		"Zip",
		"Hungry Jack's",
		"Amazon",
		"Chemist Warehouse",
		"Bunnings Warehouse",
		"Saved Up",
		"DiDi",
		"Big W",
		"Optus",
		"Reddy Express",
		"Linkt",
		"Spotify",
		"BWS",
		"Guzman y Gomez",
		"Google Play",
		"EG Ampol",
	}
}

func parseIntOrDefault(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return defaultVal
}

func getOrDefault(s string, defaultVal string) string {
	if s != "" {
		return s
	}
	return defaultVal
}
