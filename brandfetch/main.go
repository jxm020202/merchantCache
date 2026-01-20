package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	DatabaseURL          string
	BrandfetchAPIKey     string
	BrandfetchClientID   string
	TransactionsFilePath string
	CountryTLDPreference string
}

type SearchHit struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Domain       string   `json:"domain"`
	QualityScore float64  `json:"qualityScore"`
	Aliases      []string `json:"aliases"`
}

type BrandProfile struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Domain       string  `json:"domain"`
	QualityScore float64 `json:"qualityScore"`
	Company      Company `json:"company"`
	Raw          json.RawMessage
}

type Company struct {
	Location Location `json:"location"`
}

type Location struct {
	City        string `json:"city"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

type RawTransaction struct {
	ID          string
	Description string
}

func main() {
	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		exitErr(err)
	}

	pgxCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		exitErr(fmt.Errorf("parse db config: %w", err))
	}
	pgxCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		exitErr(fmt.Errorf("connect db: %w", err))
	}
	defer pool.Close()

	transactions, err := loadTransactions(cfg.TransactionsFilePath)
	if err != nil {
		exitErr(err)
	}
	if len(transactions) == 0 {
		exitErr(errors.New("no transactions found to process"))
	}

	if err := seedRawTransactions(ctx, pool, transactions); err != nil {
		exitErr(fmt.Errorf("seed raw: %w", err))
	}

	rawRows, err := fetchPending(ctx, pool, transactions)
	if err != nil {
		exitErr(fmt.Errorf("fetch pending: %w", err))
	}
	if len(rawRows) == 0 {
		fmt.Println("Nothing to process (all provided lines already processed).")
		return
	}

	matches, misses, err := enrich(ctx, pool, rawRows, cfg)
	if err != nil {
		exitErr(fmt.Errorf("enrich: %w", err))
	}

	if err := cleanupNulls(ctx, pool); err != nil {
		exitErr(fmt.Errorf("cleanup: %w", err))
	}

	fmt.Println("\n--- Summary ---")
	fmt.Printf("Matched: %d\n", matches)
	fmt.Printf("No match: %d\n", misses)
	fmt.Println("Check Postgres table 'enriched_merchants' for results.")
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

func loadTransactions(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transactions file: %w", err)
	}
	defer f.Close()

	var lines []string
	seen := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan transactions: %w", err)
	}
	return lines, nil
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

type EnrichedRow struct {
	TransactionCache string
	BrandName        string
	WebsiteURL       string
	Logo             string
	ConfidenceScore  float64
	BrandfetchID     string
	FullResponse     json.RawMessage
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

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func domainToURL(domain string) string {
	if domain == "" {
		return ""
	}
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain
	}
	return "https://" + domain
}

func logoURL(domain, clientID string) string {
	if domain == "" {
		return ""
	}
	return fmt.Sprintf("https://cdn.brandfetch.io/%s/theme/dark/logo?c=%s", domain, clientID)
}

func cleanupNulls(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		delete from enriched_merchants
		where brand_name is null
		   or website_url is null
	`)
	return err
}

func searchBrand(ctx context.Context, client *http.Client, name string, cfg Config) (*SearchHit, error) {
	url := fmt.Sprintf("https://api.brandfetch.io/v2/search/%s?c=%s", urlEncode(name), cfg.BrandfetchClientID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search status %d", resp.StatusCode)
	}
	var hits []SearchHit
	if err := json.NewDecoder(resp.Body).Decode(&hits); err != nil {
		return nil, err
	}
	return pickPreferredHit(hits, cfg.CountryTLDPreference), nil
}

func fetchBrandProfile(ctx context.Context, client *http.Client, domain string, cfg Config) (*BrandProfile, error) {
	if domain == "" {
		return nil, nil
	}
	url := fmt.Sprintf("https://api.brandfetch.io/v2/brands/%s", urlEncode(domain))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.BrandfetchAPIKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brand api status %d", resp.StatusCode)
	}
	var prof BrandProfile
	body, err := ioReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &prof); err != nil {
		return nil, err
	}
	prof.Raw = body
	return &prof, nil
}

func pickPreferredHit(hits []SearchHit, tld string) *SearchHit {
	if len(hits) == 0 {
		return nil
	}
	if tld != "" {
		for _, h := range hits {
			if strings.HasSuffix(h.Domain, tld) {
				return &h
			}
		}
	}
	return &hits[0]
}

type choiceProfile struct {
	ID           string
	Name         string
	Domain       string
	QualityScore float64
}

func pickProfile(profile *BrandProfile, hit *SearchHit) choiceProfile {
	if profile != nil {
		return choiceProfile{
			ID:           profile.ID,
			Name:         profile.Name,
			Domain:       profile.Domain,
			QualityScore: profile.QualityScore,
		}
	}
	if hit != nil {
		return choiceProfile{
			ID:           hit.ID,
			Name:         hit.Name,
			Domain:       hit.Domain,
			QualityScore: hit.QualityScore,
		}
	}
	return choiceProfile{}
}

func rawJSON(profile *BrandProfile, hit *SearchHit) json.RawMessage {
	if profile != nil && profile.Raw != nil {
		return profile.Raw
	}
	if hit != nil {
		b, _ := json.Marshal(hit)
		return b
	}
	return json.RawMessage(`null`)
}

func urlEncode(s string) string {
	return url.QueryEscape(strings.TrimSpace(s))
}

func ioReadAll(r io.Reader) ([]byte, error) {
	const max = 4 << 20 // 4MB safety
	var b []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if len(b)+n > max {
				return nil, fmt.Errorf("response too large")
			}
			b = append(b, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b, nil
			}
			return nil, err
		}
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
