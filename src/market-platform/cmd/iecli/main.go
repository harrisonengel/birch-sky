// Command iecli is the operator CLI for the Information Exchange market
// platform. It talks to the running HTTP API — it does not connect to
// databases directly.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var apiBase string

func main() {
	root := &cobra.Command{
		Use:   "iecli",
		Short: "Information Exchange CLI",
	}
	root.PersistentFlags().StringVar(&apiBase, "api", "http://localhost:8080", "market-platform base URL")

	root.AddCommand(seedCmd())
	root.AddCommand(searchCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// seed
// ---------------------------------------------------------------------------

func seedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Populate the marketplace with sample sellers, listings, and data",
		RunE:  runSeed,
	}
}

// Sample data that will live in Postgres, OpenSearch, and MinIO.
type seedSeller struct {
	Name  string
	Email string
}

type seedListing struct {
	Title       string
	Description string
	Category    string
	PriceCents  int
	Tags        []string
	DataCSV     string // inline CSV content to upload
}

var sellers = []seedSeller{
	{Name: "RetailMetrics Inc.", Email: "data@retailmetrics.example.com"},
	{Name: "DataHarvest", Email: "sales@dataharvest.example.com"},
	{Name: "ShopIntel", Email: "info@shopintel.example.com"},
	{Name: "SatView Analytics", Email: "hello@satview.example.com"},
}

// Listings are grouped by seller index.
var listingsBySeller = [][]seedListing{
	// RetailMetrics Inc.
	{
		{
			Title:       "Consumer Electronics Pricing Index Q1 2026",
			Description: "Aggregated pricing data across 12 major retailers covering 50,000+ SKUs in consumer electronics. Updated quarterly with category breakdowns, price trends, and competitive positioning metrics.",
			Category:    "pricing",
			PriceCents:  250,
			Tags:        []string{"electronics", "pricing", "retail", "quarterly"},
			DataCSV: `sku,retailer,category,price_usd,date,brand
ELEC-001,Amazon,Smartphones,799.99,2026-01-15,Samsung
ELEC-002,BestBuy,Smartphones,749.99,2026-01-15,Samsung
ELEC-003,Walmart,Smartphones,779.00,2026-01-15,Samsung
ELEC-004,Amazon,Laptops,1299.99,2026-01-15,Dell
ELEC-005,BestBuy,Laptops,1249.99,2026-01-15,Dell
ELEC-006,Amazon,Tablets,449.99,2026-01-15,Apple
ELEC-007,Target,Tablets,449.99,2026-01-15,Apple
ELEC-008,Amazon,Headphones,349.99,2026-01-15,Sony
ELEC-009,BestBuy,Headphones,329.99,2026-01-15,Sony
ELEC-010,Amazon,Smartwatches,399.99,2026-01-15,Apple
`,
		},
		{
			Title:       "Grocery Price Tracker - US National Average",
			Description: "Weekly national average pricing for 200 staple grocery items tracked across Walmart, Kroger, Costco, and regional chains. Includes historical data back to 2024.",
			Category:    "pricing",
			PriceCents:  175,
			Tags:        []string{"grocery", "pricing", "weekly", "national"},
			DataCSV: `item,category,avg_price_usd,week,yoy_change_pct
Whole Milk 1gal,Dairy,4.29,2026-W12,3.2
Large Eggs 12ct,Dairy,3.89,2026-W12,8.1
White Bread Loaf,Bakery,3.49,2026-W12,2.5
Ground Beef 1lb,Meat,5.99,2026-W12,-1.3
Chicken Breast 1lb,Meat,4.49,2026-W12,4.7
Bananas 1lb,Produce,0.59,2026-W12,0.0
Avocados each,Produce,1.29,2026-W12,-5.1
Rice 5lb bag,Pantry,6.99,2026-W12,1.4
`,
		},
	},
	// DataHarvest
	{
		{
			Title:       "Amazon Category Pricing - Electronics Under $100",
			Description: "Daily pricing snapshots for 8,200 products in Amazon electronics under $100. Includes seller rank, review count, and price history over 30 days.",
			Category:    "pricing",
			PriceCents:  175,
			Tags:        []string{"amazon", "electronics", "daily", "ecommerce"},
			DataCSV: `asin,title,price_usd,seller_rank,reviews,rating,date
B09V3KXJPB,USB-C Hub 7-in-1,29.99,142,3847,4.3,2026-03-20
B0BN72P7DK,Wireless Mouse Ergonomic,24.99,89,12503,4.5,2026-03-20
B0C5B3M4WQ,Bluetooth Speaker Portable,39.99,234,8721,4.2,2026-03-20
B0BSHF7WHF,Phone Stand Adjustable,12.99,56,24102,4.6,2026-03-20
B0CX4Y9R5L,LED Desk Lamp,34.99,178,5632,4.4,2026-03-20
B0D2WQRKTM,Webcam 1080p,49.99,312,2198,4.1,2026-03-20
B0BFJT9JKQ,Lightning Cable 3-Pack,15.99,23,45201,4.7,2026-03-20
`,
		},
		{
			Title:       "Social Media Engagement Metrics - Tech Brands",
			Description: "Monthly engagement metrics for top 50 tech brands across Twitter/X, Instagram, TikTok, and LinkedIn. Includes follower growth, engagement rates, and sentiment scores.",
			Category:    "social-media",
			PriceCents:  325,
			Tags:        []string{"social-media", "engagement", "tech", "monthly", "sentiment"},
			DataCSV: `brand,platform,followers,engagement_rate,sentiment_score,month
Apple,Twitter/X,9200000,2.1,0.78,2026-03
Apple,Instagram,32500000,3.4,0.82,2026-03
Samsung,Twitter/X,5100000,1.8,0.71,2026-03
Samsung,TikTok,4200000,5.2,0.74,2026-03
Google,LinkedIn,28000000,1.2,0.69,2026-03
Microsoft,LinkedIn,22000000,1.5,0.75,2026-03
Tesla,Twitter/X,25000000,4.8,0.62,2026-03
`,
		},
	},
	// ShopIntel
	{
		{
			Title:       "Walmart vs Amazon Price Comparison Dataset",
			Description: "Side-by-side pricing on 15,000 overlapping catalog items between Walmart and Amazon. Updated weekly with price difference calculations and category analysis.",
			Category:    "pricing",
			PriceCents:  300,
			Tags:        []string{"walmart", "amazon", "comparison", "weekly"},
			DataCSV: `product,category,walmart_price,amazon_price,diff_pct,cheaper
Samsung Galaxy S24,Smartphones,799.99,799.99,0.0,tie
Sony WH-1000XM5,Headphones,348.00,328.00,-5.7,amazon
Instant Pot Duo 6qt,Kitchen,79.95,89.95,12.5,walmart
Dyson V15 Detect,Vacuum,749.99,699.99,-6.7,amazon
Nintendo Switch OLED,Gaming,349.99,349.99,0.0,tie
KitchenAid Stand Mixer,Kitchen,379.99,349.99,-7.9,amazon
Roomba j7+,Vacuum,599.99,549.99,-8.3,amazon
`,
		},
	},
	// SatView Analytics
	{
		{
			Title:       "Retail Parking Lot Satellite Imagery - Q1 2026",
			Description: "Weekly satellite imagery analysis of parking lot occupancy for 500 major retail locations across the US. Includes vehicle count estimates, occupancy rates, and week-over-week trends.",
			Category:    "satellite",
			PriceCents:  500,
			Tags:        []string{"satellite", "retail", "parking", "imagery", "weekly"},
			DataCSV: `location_id,retailer,city,state,date,vehicle_count,occupancy_pct,wow_change_pct
SAT-001,Walmart,Houston,TX,2026-03-15,342,78.2,2.1
SAT-002,Walmart,Phoenix,AZ,2026-03-15,289,72.1,-1.3
SAT-003,Costco,Seattle,WA,2026-03-15,456,91.2,5.4
SAT-004,Target,Chicago,IL,2026-03-15,198,55.0,-3.8
SAT-005,HomeDepot,Atlanta,GA,2026-03-15,267,66.8,1.7
SAT-006,Costco,Dallas,TX,2026-03-15,412,88.4,3.2
SAT-007,BestBuy,Miami,FL,2026-03-15,156,48.7,-6.1
SAT-008,Walmart,Denver,CO,2026-03-15,301,75.3,0.9
`,
		},
		{
			Title:       "US Port Container Volume Estimates",
			Description: "Daily container throughput estimates for 20 major US ports derived from satellite imagery. Includes TEU counts, ship queue lengths, and congestion indices.",
			Category:    "satellite",
			PriceCents:  450,
			Tags:        []string{"satellite", "shipping", "ports", "logistics", "daily"},
			DataCSV: `port,date,teu_estimate,ships_at_berth,ships_queued,congestion_index
Los Angeles,2026-03-20,28450,12,3,0.42
Long Beach,2026-03-20,22100,9,1,0.31
New York/NJ,2026-03-20,19800,11,2,0.38
Savannah,2026-03-20,15200,8,0,0.22
Houston,2026-03-20,12800,7,1,0.29
Seattle/Tacoma,2026-03-20,11500,6,0,0.19
Norfolk,2026-03-20,9800,5,0,0.15
Charleston,2026-03-20,8900,4,1,0.25
`,
		},
	},
}

func runSeed(cmd *cobra.Command, args []string) error {
	fmt.Println("Seeding marketplace data...")
	fmt.Printf("API: %s\n\n", apiBase)

	// Check API is up
	resp, err := http.Get(apiBase + "/health")
	if err != nil {
		return fmt.Errorf("cannot reach API at %s: %w", apiBase, err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("API returned %d on /health", resp.StatusCode)
	}

	sellerIDs := make([]string, len(sellers))

	for i, s := range sellers {
		id, err := createSeller(s.Name, s.Email)
		if err != nil {
			return fmt.Errorf("create seller %q: %w", s.Name, err)
		}
		sellerIDs[i] = id
		fmt.Printf("  Seller: %s (%s)\n", s.Name, id)
	}

	fmt.Println()

	for sellerIdx, listings := range listingsBySeller {
		sellerID := sellerIDs[sellerIdx]
		for _, l := range listings {
			listingID, err := createListing(sellerID, l)
			if err != nil {
				return fmt.Errorf("create listing %q: %w", l.Title, err)
			}
			fmt.Printf("  Listing: %s (%s)\n", l.Title, listingID)

			if l.DataCSV != "" {
				if err := uploadData(listingID, l.DataCSV, l.Title); err != nil {
					return fmt.Errorf("upload data for %q: %w", l.Title, err)
				}
				fmt.Printf("    -> uploaded %d bytes CSV\n", len(l.DataCSV))
			}
		}
	}

	fmt.Println("\nSeed complete!")
	return nil
}

func createSeller(name, email string) (string, error) {
	body, _ := json.Marshal(map[string]string{"name": name, "email": email})
	resp, err := http.Post(apiBase+"/api/v1/sellers", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	// If seller already exists (409), try to find them via email
	// The API doesn't have a get-by-email endpoint, so we just report the error
	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("seller with email %s already exists — run with a clean database", email)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

func createListing(sellerID string, l seedListing) (string, error) {
	tagsJSON, _ := json.Marshal(l.Tags)
	body, _ := json.Marshal(map[string]interface{}{
		"seller_id":   sellerID,
		"title":       l.Title,
		"description": l.Description,
		"category":    l.Category,
		"price_cents": l.PriceCents,
		"currency":    "usd",
		"tags":        json.RawMessage(tagsJSON),
	})

	resp, err := http.Post(apiBase+"/api/v1/listings", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

func uploadData(listingID, csvContent, title string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	filename := strings.ReplaceAll(strings.ToLower(title), " ", "-") + ".csv"
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(part, csvContent); err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST",
		fmt.Sprintf("%s/api/v1/listings/%s/upload", apiBase, listingID),
		&buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(data))
	}
	return nil
}

// ---------------------------------------------------------------------------
// search
// ---------------------------------------------------------------------------

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search the marketplace",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			return runSearch(query)
		},
	}
	return cmd
}

func runSearch(query string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"query":    query,
		"mode":     "text",
		"per_page": 10,
	})

	resp, err := http.Post(apiBase+"/api/v1/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("search failed (%d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		Results []struct {
			ListingID   string  `json:"listing_id"`
			Title       string  `json:"title"`
			Description string  `json:"description"`
			Category    string  `json:"category"`
			PriceCents  int     `json:"price_cents"`
			Score       float64 `json:"score"`
		} `json:"results"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	fmt.Printf("Found %d results for %q:\n\n", result.Total, query)
	for i, r := range result.Results {
		fmt.Printf("%d. %s (ID: %s)\n", i+1, r.Title, r.ListingID)
		fmt.Printf("   Category: %s | Price: $%.2f | Score: %.4f\n", r.Category, float64(r.PriceCents)/100, r.Score)
		fmt.Printf("   %s\n\n", r.Description)
	}
	return nil
}
