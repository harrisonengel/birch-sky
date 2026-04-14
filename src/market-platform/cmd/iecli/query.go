package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run sample SQL queries against the demo data",
	Long:  "Executes predefined sample SQL queries that demonstrate the kind of structured analysis a buyer agent would perform.",
	RunE:  runQuery,
}

var queryName string

func init() {
	queryCmd.Flags().StringVarP(&queryName, "name", "n", "", "Run a specific query by name (avg-price, price-spread, top-roas, best-deals, all)")
	rootCmd.AddCommand(queryCmd)
}

type sampleQuery struct {
	name  string
	title string
	sql   string
	cols  []string
}

var sampleQueries = []sampleQuery{
	{
		name:  "avg-price",
		title: "Average Price by Category Across Retailers",
		sql: `SELECT
    category,
    retailer,
    COUNT(*) AS products,
    ROUND(AVG(price_usd)::numeric, 2) AS avg_price,
    ROUND(AVG(discount_pct)::numeric, 1) AS avg_discount_pct
FROM sample_consumer_electronics_pricing
GROUP BY category, retailer
ORDER BY category, avg_price`,
		cols: []string{"CATEGORY", "RETAILER", "PRODUCTS", "AVG PRICE", "AVG DISCOUNT %"},
	},
	{
		name:  "price-spread",
		title: "Biggest Price Spreads Across Platforms",
		sql: `SELECT
    product_name,
    category,
    best_platform,
    best_price_usd AS best_price,
    price_spread_pct AS spread_pct,
    amazon_price_usd AS amazon,
    walmart_price_usd AS walmart,
    target_price_usd AS target
FROM sample_ecommerce_price_comparison
WHERE price_spread_pct > 0
ORDER BY price_spread_pct DESC
LIMIT 10`,
		cols: []string{"PRODUCT", "CATEGORY", "BEST AT", "BEST $", "SPREAD %", "AMAZON", "WALMART", "TARGET"},
	},
	{
		name:  "top-roas",
		title: "Top Advertising Categories by ROAS",
		sql: `SELECT
    category,
    platform,
    avg_roas,
    avg_cpc_usd AS cpc,
    avg_ctr_pct AS ctr,
    avg_conversion_rate_pct AS conv_rate,
    num_advertisers
FROM sample_shopping_ads_benchmark
ORDER BY avg_roas DESC
LIMIT 10`,
		cols: []string{"CATEGORY", "PLATFORM", "ROAS", "CPC", "CTR %", "CONV %", "ADVERTISERS"},
	},
	{
		name:  "best-deals",
		title: "Best Current Deals (Highest Discounts in Stock)",
		sql: `SELECT
    product_name,
    category,
    brand,
    retailer,
    price_usd,
    msrp_usd,
    discount_pct
FROM sample_consumer_electronics_pricing
WHERE in_stock = true AND discount_pct > 0
ORDER BY discount_pct DESC
LIMIT 10`,
		cols: []string{"PRODUCT", "CATEGORY", "BRAND", "RETAILER", "PRICE", "MSRP", "DISCOUNT %"},
	},
	{
		name:  "platform-comparison",
		title: "Platform Win Rate: Who Has the Best Prices?",
		sql: `SELECT
    best_platform,
    COUNT(*) AS wins,
    ROUND(AVG(best_price_usd)::numeric, 2) AS avg_best_price,
    ROUND(AVG(price_spread_pct)::numeric, 2) AS avg_spread_pct
FROM sample_ecommerce_price_comparison
WHERE price_spread_pct > 0
GROUP BY best_platform
ORDER BY wins DESC`,
		cols: []string{"PLATFORM", "WINS", "AVG BEST PRICE", "AVG SPREAD %"},
	},
	{
		name:  "ad-efficiency",
		title: "Ad Platform Efficiency: CPC vs Conversion Rate",
		sql: `SELECT
    platform,
    ROUND(AVG(avg_cpc_usd)::numeric, 2) AS avg_cpc,
    ROUND(AVG(avg_ctr_pct)::numeric, 2) AS avg_ctr,
    ROUND(AVG(avg_conversion_rate_pct)::numeric, 2) AS avg_conv_rate,
    ROUND(AVG(avg_roas)::numeric, 2) AS avg_roas,
    SUM(num_advertisers) AS total_advertisers
FROM sample_shopping_ads_benchmark
GROUP BY platform
ORDER BY avg_roas DESC`,
		cols: []string{"PLATFORM", "AVG CPC", "AVG CTR %", "AVG CONV %", "AVG ROAS", "ADVERTISERS"},
	},
}

func runQuery(cmd *cobra.Command, args []string) error {
	dsn := getDatabaseURL()
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	name := queryName

	// If a positional arg was given and no flag, use the arg.
	if name == "" && len(args) > 0 {
		name = args[0]
	}

	if name == "" || name == "all" {
		for i, q := range sampleQueries {
			if i > 0 {
				fmt.Println()
			}
			if err := executeQuery(ctx, db, q); err != nil {
				return err
			}
		}
		return nil
	}

	for _, q := range sampleQueries {
		if q.name == name {
			return executeQuery(ctx, db, q)
		}
	}

	fmt.Fprintf(os.Stderr, "unknown query %q. Available queries:\n", name)
	for _, q := range sampleQueries {
		fmt.Fprintf(os.Stderr, "  %-20s %s\n", q.name, q.title)
	}
	return fmt.Errorf("query not found")
}

func executeQuery(ctx context.Context, db *sqlx.DB, q sampleQuery) error {
	fmt.Printf("=== %s ===\n", q.title)
	fmt.Printf("SQL: %s\n\n", strings.TrimSpace(q.sql))

	rows, err := db.QueryxContext(ctx, q.sql)
	if err != nil {
		return fmt.Errorf("query %s: %w", q.name, err)
	}
	defer rows.Close()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(q.cols, "\t"))
	fmt.Fprintln(w, strings.Repeat("---\t", len(q.cols)))

	rowCount := 0
	for rows.Next() {
		cols, err := rows.SliceScan()
		if err != nil {
			return fmt.Errorf("scan %s: %w", q.name, err)
		}
		parts := make([]string, len(cols))
		for i, c := range cols {
			if c == nil {
				parts[i] = "-"
			} else {
				parts[i] = fmt.Sprintf("%v", c)
			}
		}
		fmt.Fprintln(w, strings.Join(parts, "\t"))
		rowCount++
	}
	w.Flush()
	fmt.Printf("\n(%d rows)\n", rowCount)
	return nil
}
