# MerchantCache - Go Version

Complete verification pipeline for merchant lookup, verification, and enrichment.

## Architecture

1. **ABN Lookup** - Get ABN, legal name, state from Australian Business Register (ABR)
2. **Google Verification** - Verify legal name & ABN are correct using Google Custom Search
3. **Address Lookup** - Find head office address
4. **Output** - Save enriched merchant data with verification confidence

## Prerequisites

- Go 1.21+
- `.env` file with credentials:
  ```
  ABR_GUID=your_guid
  ABR_ENDPOINT=https://abr.business.gov.au/abrxmlsearch/AbrXmlSearch.asmx/ABRSearchByNameSimpleProtocol
  GOOGLE_API_KEY=your_key
  GOOGLE_SEARCH_ENGINE_ID=your_id
  GOOGLE_CLIENT_ID=your_client_id
  GOOGLE_CLIENT_SECRET=your_secret
  TIMEOUT=5
  OUTPUT_FILE=enriched_merchants_demo.csv
  ENABLE_VERIFICATION=true
  ```

## Build

```bash
go build -o merchantcache ./cmd
```

## Run

```bash
go run ./cmd
```

## Project Structure

```
├── cmd/
│   └── main.go                 # Entry point
├── internal/
│   ├── abr/
│   │   └── client.go           # ABR API client
│   ├── google/
│   │   └── client.go           # Google Custom Search client
│   ├── config/
│   │   └── config.go           # Configuration loading
│   └── data/
│       └── processor.go        # Data processing and output
├── go.mod                      # Go module definition
└── .env                        # Environment variables (not in git)
```

## Output

Results are saved to CSV file specified in `OUTPUT_FILE` environment variable with:
- merchant_name
- abn
- state
- legal_name
- score
- verified
- confidence
- head_office_address
- google_abn
- google_legal_name
