package main

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RawTransaction struct {
	ID          string
	Description string
}

type EnrichedRow struct {
	TransactionCache string
	BrandName        string
	WebsiteURL       string
	Logo             string
	ConfidenceScore  float64
	BrandfetchID     string
	FullResponse     []byte
}

func seedRawTransactions(ctx context.Context, pool *pgxpool.Pool, lines []string) error {
	for _, line := range lines {
		_, err := pool.Exec(ctx, `
			insert into raw_transactions (description)
			values ($1)
			on conflict (description) do nothing
		`, line)
		if err != nil {
			return err
		}
	}
	return nil
}

func fetchPending(ctx context.Context, pool *pgxpool.Pool, lines []string) ([]RawTransaction, error) {
	rows, err := pool.Query(ctx, `
		select id, description
		from raw_transactions
		where processed = false
		  and description = any($1)
	`, lines)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RawTransaction
	for rows.Next() {
		var r RawTransaction
		if err := rows.Scan(&r.ID, &r.Description); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func upsertEnriched(ctx context.Context, pool *pgxpool.Pool, r EnrichedRow) error {
	_, err := pool.Exec(ctx, `
		insert into enriched_merchants (
			transaction_cache,
			brand_name,
			website_url,
			logo,
			confidence_score,
			brandfetch_id,
			full_response
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (transaction_cache) do update set
			brand_name = excluded.brand_name,
			website_url = excluded.website_url,
			logo = excluded.logo,
			confidence_score = excluded.confidence_score,
			brandfetch_id = excluded.brandfetch_id,
			full_response = excluded.full_response
	`, r.TransactionCache, nullIfEmpty(r.BrandName), nullIfEmpty(r.WebsiteURL), nullIfEmpty(r.Logo), r.ConfidenceScore, nullIfEmpty(r.BrandfetchID), r.FullResponse)
	return err
}

func cleanupNulls(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		delete from enriched_merchants
		where brand_name is null
		   or website_url is null
	`)
	return err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
