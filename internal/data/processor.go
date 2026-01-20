package data

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Result struct {
	MerchantName    string
	ABN             string
	State           string
	LegalName       string
	Score           string
	Verified        bool
	Confidence      float64
	Address         string
	GoogleABN       string
	GoogleLegalName string
}

type Processor struct {
	outputFile string
	rows       []Result
}

func NewProcessor(outputFile string) *Processor {
	return &Processor{
		outputFile: outputFile,
		rows:       make([]Result, 0),
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
	fmt.Println("Google Verification:")
	fmt.Printf("  ✓ Verified:               %d\n", verified)
	verifyRate := 0.0
	if found > 0 {
		verifyRate = float64(verified) / float64(found) * 100
	}
	fmt.Printf("  • Verification rate:      %.1f%%\n", verifyRate)
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

	// Print table
	fmt.Printf("%-30s | %-15s | %-15s | %-10s | %-10s\n", "Merchant", "ABN", "Legal Name", "Verified", "Confidence")
	fmt.Println(strings.Repeat("-", 100))

	for _, r := range p.rows {
		verStr := "No"
		if r.Verified {
			verStr = "Yes"
		}
		fmt.Printf("%-30s | %-15s | %-15s | %-10s | %-10.2f\n",
			truncate(r.MerchantName, 30),
			r.ABN,
			truncate(r.LegalName, 15),
			verStr,
			r.Confidence)
	}
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
