package data

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Result struct {
	MerchantName    string  `json:"merchant_name"`
	ABN             string  `json:"abn"`
	ACN             string  `json:"acn"`
	State           string  `json:"state"`
	LegalName       string  `json:"legal_name"`
	Score           string  `json:"score"`
	Verified        bool    `json:"verified"`
	Confidence      float64 `json:"confidence"`
	Address         string  `json:"head_office_address"`
	GoogleABN       string  `json:"google_abn"`
	GoogleLegalName string  `json:"google_legal_name"`
}

type Processor struct {
	outputFile string
	rows       []Result
	supabase   SupabaseConfig
}

type SupabaseConfig struct {
	URL   string
	Key   string
	Table string
}

func (p SupabaseConfig) Enabled() bool {
	return p.URL != "" && p.Key != "" && p.Table != ""
}

func NewProcessor(outputFile string, supabase SupabaseConfig) *Processor {
	return &Processor{
		outputFile: outputFile,
		rows:       make([]Result, 0),
		supabase:   supabase,
	}
}

func (p *Processor) AddResult(r Result) {
	p.rows = append(p.rows, r)
}

func (p *Processor) SaveToFile() (string, error) {
	outPath := filepath.Join(".", p.outputFile)

	file, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"merchant_name",
		"abn",
		"acn",
		"state",
		"legal_name",
		"score",
		"verified",
		"confidence",
		"head_office_address",
		"google_abn",
		"google_legal_name",
	}
	writer.Write(header)

	// Write data
	for _, r := range p.rows {
		row := []string{
			r.MerchantName,
			r.ABN,
			r.ACN,
			r.State,
			r.LegalName,
			r.Score,
			boolToYesNo(r.Verified),
			fmt.Sprintf("%.2f", r.Confidence),
			r.Address,
			r.GoogleABN,
			r.GoogleLegalName,
		}
		writer.Write(row)
	}

	return outPath, nil
}

func (p *Processor) PrintSummary() {
	total := len(p.rows)
	found := 0
	verified := 0
	withAddress := 0

	for _, r := range p.rows {
		if r.ABN != "" {
			found++
		}
		if r.Verified {
			verified++
		}
		if r.Address != "" {
			withAddress++
		}
	}

	notFound := total - found

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("Complete Verification Pipeline Summary")
	fmt.Println("============================================================")
	fmt.Printf("Total merchants:            %d\n", total)
	fmt.Printf("  ✓ ABN Found:              %d\n", found)
	fmt.Printf("  ✗ ABN Not Found:          %d\n", notFound)
	fmt.Printf("  • Success rate:           %.1f%%\n", float64(found)/float64(total)*100)
	fmt.Println()
	fmt.Println("Address Lookup:")
	fmt.Printf("  ✓ Head Office Found:      %d\n", withAddress)
	coverage := 0.0
	if found > 0 {
		coverage = float64(withAddress) / float64(found) * 100
	}
	fmt.Printf("  • Coverage:               %.1f%%\n", coverage)
	fmt.Println("============================================================")
	fmt.Println()

	// Print detailed results table
	fmt.Println("DETAILED RESULTS:")
	fmt.Println("============================================================")
	fmt.Printf("%-4s | %-20s | %-12s | %-10s | %-25s | %-50s\n",
		"No.", "Merchant", "ABN", "ACN", "Legal Name", "Head Office Address")
	fmt.Println(strings.Repeat("-", 150))

	for idx, r := range p.rows {
		fmt.Printf("%-4d | %-20s | %-12s | %-10s | %-25s | %-50s\n",
			idx+1,
			truncate(r.MerchantName, 20),
			r.ABN,
			r.ACN,
			truncate(r.LegalName, 25),
			truncate(r.Address, 50))
	}
	fmt.Println("============================================================")
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// SyncSupabase sends all processed rows to Supabase via REST if configured.
func (p *Processor) SyncSupabase() error {
	if !p.supabase.Enabled() {
		fmt.Println("Supabase sync skipped: config not provided")
		return nil
	}

	if len(p.rows) == 0 {
		fmt.Println("Supabase sync skipped: no rows to send")
		return nil
	}

	payload, err := json.Marshal(p.rows)
	if err != nil {
		return fmt.Errorf("marshal supabase payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/v1/%s", strings.TrimSuffix(p.supabase.URL, "/"), p.supabase.Table)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build supabase request: %w", err)
	}

	req.Header.Set("apikey", p.supabase.Key)
	req.Header.Set("Authorization", "Bearer "+p.supabase.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("supabase request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("supabase returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fmt.Printf("✓ Supabase sync complete (%d rows)\n", len(p.rows))
	return nil
}
