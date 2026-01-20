package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func enrich(ctx context.Context, pool *pgxpool.Pool, rows []RawTransaction, cfg Config) (int, int, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	matches, misses := 0, 0

	for _, tx := range rows {
		desc := tx.Description
		fmt.Printf("Processing: %s\n", desc)

		searchHit, err := searchBrand(ctx, client, desc, cfg)
		if err != nil {
			fmt.Printf("  search error: %v\n", err)
		}

		var domain string
		if searchHit != nil {
			domain = searchHit.Domain
		}

		var profile *BrandProfile
		if domain != "" {
			p, err := fetchBrandProfile(ctx, client, domain, cfg)
			if err != nil {
				fmt.Printf("  profile error: %v\n", err)
			}
			profile = p
		}

		if searchHit != nil || profile != nil {
			choice := pickProfile(profile, searchHit)
			domain = choice.Domain
			fullResp := rawJSON(profile, searchHit)

			if err := upsertEnriched(ctx, pool, EnrichedRow{
				TransactionCache: desc,
				BrandName:        choice.Name,
				WebsiteURL:       domainToURL(domain),
				Logo:             logoURL(domain, cfg.BrandfetchClientID),
				ConfidenceScore:  choice.QualityScore,
				BrandfetchID:     choice.ID,
				FullResponse:     fullResp,
			}); err != nil {
				return matches, misses, err
			}
			matches++
		} else {
			if err := upsertEnriched(ctx, pool, EnrichedRow{
				TransactionCache: desc,
				ConfidenceScore:  0,
				FullResponse:     json.RawMessage(`null`),
			}); err != nil {
				return matches, misses, err
			}
			misses++
		}

		if _, err := pool.Exec(ctx, `
			update raw_transactions
			set processed = true
			where id = $1
		`, tx.ID); err != nil {
			return matches, misses, err
		}
	}

	return matches, misses, nil
}
