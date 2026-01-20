package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/joho/godotenv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Load .env file explicitly
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found or couldn't be loaded")
	}

	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		exitErr(err)
	}

	// Debug: Show first 50 chars of DATABASE_URL (for troubleshooting)
	dbURLpreview := cfg.DatabaseURL
	if len(dbURLpreview) > 50 {
		dbURLpreview = dbURLpreview[:50] + "..."
	}
	fmt.Printf("DEBUG: Using DATABASE_URL: %s\n", dbURLpreview)

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
