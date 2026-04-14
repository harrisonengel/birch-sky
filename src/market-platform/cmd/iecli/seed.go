package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// seedSampleDatasets connects directly to Postgres and populates the
// sample_* dataset tables created by migration 002. These structured
// tables represent the kind of data that sellers list for sale, and
// exist so the query command can demonstrate SQL analysis.
func seedSampleDatasets() error {
	dsn := getDatabaseURL()
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	ctx := context.Background()

	if err := seedConsumerElectronicsPricing(ctx, db); err != nil {
		return fmt.Errorf("consumer electronics pricing: %w", err)
	}

	if err := seedEcommercePriceComparison(ctx, db); err != nil {
		return fmt.Errorf("ecommerce price comparison: %w", err)
	}

	if err := seedShoppingAdsBenchmark(ctx, db); err != nil {
		return fmt.Errorf("shopping ads benchmark: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Sample Consumer Electronics Pricing
// ---------------------------------------------------------------------------

func seedConsumerElectronicsPricing(ctx context.Context, db *sqlx.DB) error {
	var count int
	if err := db.GetContext(ctx, &count, "SELECT COUNT(*) FROM sample_consumer_electronics_pricing"); err != nil {
		return err
	}
	if count > 0 {
		log.Println("  sample_consumer_electronics_pricing: already seeded, skipping")
		return nil
	}

	const q = `INSERT INTO sample_consumer_electronics_pricing
		(product_name, category, brand, retailer, price_usd, msrp_usd, discount_pct, in_stock, price_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	rows := []struct {
		product, category, brand, retailer string
		price, msrp, discount              float64
		inStock                            bool
		date                               string
	}{
		{"iPhone 16 Pro 256GB", "smartphones", "Apple", "Amazon", 999.00, 999.00, 0.00, true, "2026-03-15"},
		{"iPhone 16 Pro 256GB", "smartphones", "Apple", "Best Buy", 999.00, 999.00, 0.00, true, "2026-03-15"},
		{"iPhone 16 Pro 256GB", "smartphones", "Apple", "Walmart", 979.00, 999.00, 2.00, true, "2026-03-15"},
		{"Galaxy S26 Ultra", "smartphones", "Samsung", "Amazon", 1199.99, 1299.99, 7.69, true, "2026-03-15"},
		{"Galaxy S26 Ultra", "smartphones", "Samsung", "Best Buy", 1249.99, 1299.99, 3.85, true, "2026-03-15"},
		{"Galaxy S26 Ultra", "smartphones", "Samsung", "Walmart", 1199.99, 1299.99, 7.69, false, "2026-03-15"},
		{"MacBook Air M4 15\"", "laptops", "Apple", "Amazon", 1249.00, 1299.00, 3.85, true, "2026-03-15"},
		{"MacBook Air M4 15\"", "laptops", "Apple", "Best Buy", 1199.00, 1299.00, 7.70, true, "2026-03-15"},
		{"MacBook Air M4 15\"", "laptops", "Apple", "Target", 1279.00, 1299.00, 1.54, true, "2026-03-15"},
		{"ThinkPad X1 Carbon Gen 13", "laptops", "Lenovo", "Amazon", 1429.00, 1599.00, 10.63, true, "2026-03-15"},
		{"ThinkPad X1 Carbon Gen 13", "laptops", "Lenovo", "Best Buy", 1499.00, 1599.00, 6.25, true, "2026-03-15"},
		{"ThinkPad X1 Carbon Gen 13", "laptops", "Lenovo", "Walmart", 1449.00, 1599.00, 9.38, false, "2026-03-15"},
		{"AirPods Pro 3", "audio", "Apple", "Amazon", 249.00, 249.00, 0.00, true, "2026-03-15"},
		{"AirPods Pro 3", "audio", "Apple", "Best Buy", 249.00, 249.00, 0.00, true, "2026-03-15"},
		{"AirPods Pro 3", "audio", "Apple", "Target", 234.99, 249.00, 5.62, true, "2026-03-15"},
		{"Sony WH-1000XM6", "audio", "Sony", "Amazon", 348.00, 399.99, 13.00, true, "2026-03-15"},
		{"Sony WH-1000XM6", "audio", "Sony", "Best Buy", 349.99, 399.99, 12.50, true, "2026-03-15"},
		{"Sony WH-1000XM6", "audio", "Sony", "Walmart", 339.00, 399.99, 15.25, true, "2026-03-15"},
		{"iPad Pro M4 13\"", "tablets", "Apple", "Amazon", 1099.00, 1099.00, 0.00, true, "2026-03-15"},
		{"iPad Pro M4 13\"", "tablets", "Apple", "Best Buy", 1049.00, 1099.00, 4.55, true, "2026-03-15"},
		{"iPad Pro M4 13\"", "tablets", "Apple", "Target", 1079.00, 1099.00, 1.82, true, "2026-03-15"},
		{"Galaxy Tab S10 Ultra", "tablets", "Samsung", "Amazon", 1049.99, 1199.99, 12.50, true, "2026-03-15"},
		{"Galaxy Tab S10 Ultra", "tablets", "Samsung", "Best Buy", 1099.99, 1199.99, 8.33, true, "2026-03-15"},
		{"Galaxy Tab S10 Ultra", "tablets", "Samsung", "Walmart", 1029.99, 1199.99, 14.17, true, "2026-03-15"},
		{"Apple Watch Ultra 3", "wearables", "Apple", "Amazon", 799.00, 799.00, 0.00, true, "2026-03-15"},
		{"Apple Watch Ultra 3", "wearables", "Apple", "Best Buy", 799.00, 799.00, 0.00, true, "2026-03-15"},
		{"Apple Watch Ultra 3", "wearables", "Apple", "Target", 779.00, 799.00, 2.50, true, "2026-03-15"},
		{"RTX 5080 16GB", "graphics-cards", "NVIDIA", "Amazon", 999.99, 999.99, 0.00, false, "2026-03-15"},
		{"RTX 5080 16GB", "graphics-cards", "NVIDIA", "Best Buy", 999.99, 999.99, 0.00, false, "2026-03-15"},
		{"RTX 5080 16GB", "graphics-cards", "NVIDIA", "Walmart", 1049.99, 999.99, 0.00, false, "2026-03-15"},
	}

	for _, r := range rows {
		if _, err := db.ExecContext(ctx, q, r.product, r.category, r.brand, r.retailer, r.price, r.msrp, r.discount, r.inStock, r.date); err != nil {
			return fmt.Errorf("row %s/%s: %w", r.product, r.retailer, err)
		}
	}
	log.Printf("  sample_consumer_electronics_pricing: inserted %d rows", len(rows))
	return nil
}

// ---------------------------------------------------------------------------
// Sample Ecommerce Price Comparison
// ---------------------------------------------------------------------------

func seedEcommercePriceComparison(ctx context.Context, db *sqlx.DB) error {
	var count int
	if err := db.GetContext(ctx, &count, "SELECT COUNT(*) FROM sample_ecommerce_price_comparison"); err != nil {
		return err
	}
	if count > 0 {
		log.Println("  sample_ecommerce_price_comparison: already seeded, skipping")
		return nil
	}

	const q = `INSERT INTO sample_ecommerce_price_comparison
		(product_name, category, amazon_price_usd, walmart_price_usd, target_price_usd, best_price_usd, best_platform, price_spread_pct, snapshot_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	rows := []struct {
		product, category             string
		amazon, walmart, target, best float64
		bestPlatform                  string
		spread                        float64
		date                          string
	}{
		{"iPhone 16 Pro 256GB", "smartphones", 999.00, 979.00, 0, 979.00, "Walmart", 2.04, "2026-03-15"},
		{"Galaxy S26 Ultra", "smartphones", 1199.99, 1199.99, 0, 1199.99, "Amazon", 0.00, "2026-03-15"},
		{"MacBook Air M4 15\"", "laptops", 1249.00, 0, 1279.00, 1249.00, "Amazon", 2.40, "2026-03-15"},
		{"ThinkPad X1 Carbon Gen 13", "laptops", 1429.00, 1449.00, 0, 1429.00, "Amazon", 1.40, "2026-03-15"},
		{"AirPods Pro 3", "audio", 249.00, 0, 234.99, 234.99, "Target", 5.96, "2026-03-15"},
		{"Sony WH-1000XM6", "audio", 348.00, 339.00, 0, 339.00, "Walmart", 2.65, "2026-03-15"},
		{"iPad Pro M4 13\"", "tablets", 1099.00, 0, 1079.00, 1079.00, "Target", 1.85, "2026-03-15"},
		{"Galaxy Tab S10 Ultra", "tablets", 1049.99, 1029.99, 0, 1029.99, "Walmart", 1.94, "2026-03-15"},
		{"Apple Watch Ultra 3", "wearables", 799.00, 0, 779.00, 779.00, "Target", 2.57, "2026-03-15"},
		{"RTX 5080 16GB", "graphics-cards", 999.99, 1049.99, 0, 999.99, "Amazon", 5.00, "2026-03-15"},
		{"Dyson V15 Detect", "home", 749.99, 699.99, 729.99, 699.99, "Walmart", 7.14, "2026-03-15"},
		{"LG C4 65\" OLED", "tvs", 1499.99, 1399.99, 1449.99, 1399.99, "Walmart", 7.14, "2026-03-15"},
		{"Bose QuietComfort Ultra", "audio", 429.00, 399.00, 419.00, 399.00, "Walmart", 7.52, "2026-03-15"},
		{"PS5 Pro", "gaming", 699.99, 699.99, 699.99, 699.99, "Amazon", 0.00, "2026-03-15"},
		{"Nintendo Switch 2", "gaming", 449.99, 449.99, 449.99, 449.99, "Amazon", 0.00, "2026-03-15"},
	}

	for _, r := range rows {
		if _, err := db.ExecContext(ctx, q, r.product, r.category, r.amazon, r.walmart, r.target, r.best, r.bestPlatform, r.spread, r.date); err != nil {
			return fmt.Errorf("row %s: %w", r.product, err)
		}
	}
	log.Printf("  sample_ecommerce_price_comparison: inserted %d rows", len(rows))
	return nil
}

// ---------------------------------------------------------------------------
// Sample Shopping Ads Benchmark
// ---------------------------------------------------------------------------

func seedShoppingAdsBenchmark(ctx context.Context, db *sqlx.DB) error {
	var count int
	if err := db.GetContext(ctx, &count, "SELECT COUNT(*) FROM sample_shopping_ads_benchmark"); err != nil {
		return err
	}
	if count > 0 {
		log.Println("  sample_shopping_ads_benchmark: already seeded, skipping")
		return nil
	}

	const q = `INSERT INTO sample_shopping_ads_benchmark
		(category, platform, avg_cpc_usd, avg_ctr_pct, avg_conversion_rate_pct, avg_roas, monthly_spend_index, num_advertisers, report_month)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	rows := []struct {
		category, platform       string
		cpc, ctr, convRate, roas float64
		spendIndex, numAdv       int
		month                    string
	}{
		{"smartphones", "Google Shopping", 1.85, 2.10, 3.20, 4.50, 95, 142, "2026-03"},
		{"smartphones", "Meta Ads", 1.20, 1.80, 2.80, 3.80, 78, 98, "2026-03"},
		{"smartphones", "Amazon Ads", 0.95, 3.40, 4.10, 5.20, 100, 87, "2026-03"},
		{"laptops", "Google Shopping", 2.40, 1.60, 2.50, 3.80, 82, 95, "2026-03"},
		{"laptops", "Meta Ads", 1.80, 1.20, 1.90, 3.10, 65, 72, "2026-03"},
		{"laptops", "Amazon Ads", 1.30, 2.80, 3.50, 4.60, 90, 68, "2026-03"},
		{"audio", "Google Shopping", 1.10, 2.50, 3.80, 5.10, 70, 118, "2026-03"},
		{"audio", "Meta Ads", 0.85, 2.20, 3.40, 4.80, 55, 89, "2026-03"},
		{"audio", "Amazon Ads", 0.65, 4.10, 5.20, 6.50, 85, 76, "2026-03"},
		{"tablets", "Google Shopping", 1.95, 1.90, 2.80, 4.20, 60, 78, "2026-03"},
		{"tablets", "Meta Ads", 1.45, 1.50, 2.10, 3.40, 45, 56, "2026-03"},
		{"tablets", "Amazon Ads", 1.05, 3.20, 3.90, 5.00, 72, 52, "2026-03"},
		{"wearables", "Google Shopping", 1.30, 2.80, 3.50, 4.90, 55, 92, "2026-03"},
		{"wearables", "Meta Ads", 0.95, 2.40, 3.10, 4.40, 48, 75, "2026-03"},
		{"wearables", "Amazon Ads", 0.70, 3.80, 4.50, 5.80, 65, 61, "2026-03"},
		{"graphics-cards", "Google Shopping", 2.80, 3.20, 4.50, 3.20, 45, 38, "2026-03"},
		{"graphics-cards", "Meta Ads", 2.10, 2.50, 3.20, 2.80, 30, 25, "2026-03"},
		{"graphics-cards", "Amazon Ads", 1.60, 4.50, 5.80, 4.10, 55, 22, "2026-03"},
		{"tvs", "Google Shopping", 2.20, 1.40, 2.10, 3.60, 75, 85, "2026-03"},
		{"tvs", "Meta Ads", 1.70, 1.10, 1.60, 2.90, 58, 62, "2026-03"},
		{"tvs", "Amazon Ads", 1.25, 2.60, 3.20, 4.40, 82, 55, "2026-03"},
		{"gaming", "Google Shopping", 1.50, 2.90, 3.80, 4.70, 68, 105, "2026-03"},
		{"gaming", "Meta Ads", 1.10, 2.60, 3.30, 4.20, 58, 82, "2026-03"},
		{"gaming", "Amazon Ads", 0.80, 4.20, 5.10, 5.90, 78, 70, "2026-03"},
	}

	for _, r := range rows {
		if _, err := db.ExecContext(ctx, q, r.category, r.platform, r.cpc, r.ctr, r.convRate, r.roas, r.spendIndex, r.numAdv, r.month); err != nil {
			return fmt.Errorf("row %s/%s: %w", r.category, r.platform, err)
		}
	}
	log.Printf("  sample_shopping_ads_benchmark: inserted %d rows", len(rows))
	return nil
}
