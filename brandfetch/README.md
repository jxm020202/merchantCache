# Merchant Enrichment (Go)

Enrich merchant names into brand profiles using Brandfetch and Postgres.  
Flow: load names → seed `raw_transactions` → Brand Search → Brand API → upsert `enriched_merchants` → clean nulls.

## Prerequisites
- Go 1.22+
- Postgres (Supabase works; use the DB connection string)
- Brandfetch API key (Bearer) and Client ID

## Setup
1) Copy env example and fill secrets:
```bash
cp env.example .env
# fill DATABASE_URL, BRANDFETCH_API_KEY, BRANDFETCH_CLIENT_ID
```
2) Install deps (if needed):
```bash
go mod tidy
```

## Run
```bash
go run main.go
```
- Transactions are read from `transactions.txt` by default (one merchant name per line). Override with `TRANSACTIONS_FILE`.

## Environment variables
- `DATABASE_URL` (required) – Postgres DSN, e.g. `postgres://user:pass@host:5432/db?sslmode=require`
- `BRANDFETCH_API_KEY` (required) – Bearer token for Brand API
- `BRANDFETCH_CLIENT_ID` (required) – Client ID for Brand Search / logo hotlink
- `TRANSACTIONS_FILE` (optional) – default `transactions.txt`
- `COUNTRY_TLD_PREFERENCE` (optional) – default `.au`; used to prefer `.au` domains in Brand Search

## What the program does
1. Reads merchant names from the transactions file (deduped, non-empty).
2. Inserts into `raw_transactions` (on conflict do nothing).
3. Fetches unprocessed rows matching those descriptions.
4. For each row:
   - Brand Search (`/v2/search/{name}?c=CLIENT_ID`), prefer `.au` domain when present.
   - Brand API (`/v2/brands/{domain}` with Bearer key) to fetch full profile.
   - Upsert into `enriched_merchants` keyed by `transaction_cache`.
   - Mark the raw row processed.
5. Deletes rows in `enriched_merchants` where `brand_name` or `website_url` is NULL.

## Tables (expected)
`raw_transactions`
- id uuid primary key default gen_random_uuid()
- description text unique not null
- processed boolean default false
- created_at timestamptz default now()

`enriched_merchants`
- id uuid primary key default gen_random_uuid()
- transaction_cache text unique not null
- brand_name text
- legal_name text
- logo text
- anzsic_class_code text
- abn_head_office text
- acn_head_office text
- head_office_address text
- website_url text
- bpay_biller_code text
- mcc_code_test text
- wemoney_category text
- confidence_score double precision
- brandfetch_id text
- full_response jsonb
- created_at timestamptz default now()

> The program does not create tables; ensure they exist (matching your schema).

## Notes on Brandfetch usage
- Brand Search has no country filter; we only prefer `.au` domains when available.
- Brand API returns the full profile once we have a domain.
- If both calls fail for a line, we upsert a stub (confidence 0); later we delete null brand/website rows.

## Changing the input
- Edit `transactions.txt` (one merchant name per line). Rerun `go run main.go`.

## Logs & summary
- Logs each processed description.
- Prints matched vs. no-match at the end. The final data is in `enriched_merchants`.
