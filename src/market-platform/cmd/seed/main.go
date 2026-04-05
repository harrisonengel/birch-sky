package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/config"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/search"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/objectstore"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	sellerRepo := postgres.NewSellerRepo(db)
	listingRepo := postgres.NewListingRepo(db)
	objStore, err := objectstore.NewMinIOStore(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}

	var embedder search.Embedder
	if cfg.HasBedrock() {
		embedder, err = search.NewBedrockEmbedder(cfg.AWSRegion)
		if err != nil {
			log.Fatalf("bedrock: %v", err)
		}
	} else {
		embedder = search.NewLocalEmbedder()
	}

	searchEngine, err := search.NewOpenSearchEngine(cfg.OpenSearchURL)
	if err != nil {
		log.Fatalf("opensearch: %v", err)
	}
	if err := searchEngine.EnsureIndex(context.Background()); err != nil {
		log.Fatalf("index: %v", err)
	}

	indexer := search.NewIndexer(searchEngine, embedder)
	listingSvc := service.NewListingService(listingRepo, sellerRepo, objStore, indexer)

	ctx := context.Background()

	// Create sellers
	seller1, err := listingSvc.CreateSeller(ctx, "DataCorp Analytics", "datacorp@example.com")
	if err != nil {
		log.Printf("seller1 may already exist: %v", err)
		seller1Existing, _ := sellerRepo.GetByEmail(ctx, "datacorp@example.com")
		if seller1Existing != nil {
			seller1 = seller1Existing
		} else {
			log.Fatalf("cannot create seller1: %v", err)
		}
	}
	log.Printf("Seller 1: %s (%s)", seller1.Name, seller1.ID)

	seller2, err := listingSvc.CreateSeller(ctx, "Global Insights Ltd", "insights@example.com")
	if err != nil {
		log.Printf("seller2 may already exist: %v", err)
		seller2Existing, _ := sellerRepo.GetByEmail(ctx, "insights@example.com")
		if seller2Existing != nil {
			seller2 = seller2Existing
		} else {
			log.Fatalf("cannot create seller2: %v", err)
		}
	}
	log.Printf("Seller 2: %s (%s)", seller2.Name, seller2.ID)

	// Create listings with sample data
	type seedListing struct {
		sellerID    string
		title       string
		description string
		category    string
		priceCents  int
		tags        []string
		sampleData  string
		dataFormat  string
	}

	listings := []seedListing{
		{
			sellerID:    seller1.ID,
			title:       "Consumer Electronics Pricing Trends 2024",
			description: "Comprehensive pricing data for consumer electronics across 50+ product categories in North America and Europe. Includes quarterly trends, seasonal patterns, and competitive pricing analysis for smartphones, laptops, tablets, wearables, and home electronics.",
			category:    "consumer-electronics",
			priceCents:  25000,
			tags:        []string{"electronics", "pricing", "trends", "retail", "gadgets", "cost analysis"},
			sampleData:  "category,product,region,q1_price,q2_price,q3_price,q4_price,trend\nSmartphones,Flagship,North America,999,979,949,899,declining\nSmartphones,Mid-range,North America,499,489,479,449,declining\nLaptops,Premium,Europe,1499,1519,1549,1599,increasing\nTablets,Standard,North America,329,319,299,279,declining\nWearables,Smartwatch,Europe,299,289,279,269,declining",
			dataFormat:  "text/csv",
		},
		{
			sellerID:    seller1.ID,
			title:       "Real Estate Pricing Data - Major US Markets",
			description: "Monthly median home prices, rental yields, and price-to-income ratios for the top 30 US metropolitan areas. Data spans 2020-2024 with demographic overlays and housing supply metrics.",
			category:    "real-estate",
			priceCents:  45000,
			tags:        []string{"real estate", "housing", "property prices", "rental yields", "demographics"},
			sampleData:  "city,state,median_price,rental_yield_pct,price_to_income,supply_months,yoy_change_pct\nSan Francisco,CA,1250000,3.2,12.5,2.1,-5.3\nAustin,TX,450000,4.8,7.2,3.5,-8.1\nMiami,FL,520000,5.1,8.9,2.8,2.4\nNew York,NY,680000,3.8,10.1,3.2,-2.1\nSeattle,WA,750000,3.5,9.8,2.5,-4.7",
			dataFormat:  "text/csv",
		},
		{
			sellerID:    seller2.ID,
			title:       "SaaS Industry Metrics Benchmark 2024",
			description: "Benchmarking data for 200+ SaaS companies including ARR growth rates, churn rates, LTV:CAC ratios, net revenue retention, and gross margins. Segmented by company size, vertical, and pricing model.",
			category:    "saas-metrics",
			priceCents:  35000,
			tags:        []string{"SaaS", "benchmarks", "ARR", "churn", "LTV", "CAC"},
			sampleData:  `{"metrics": [{"company_size": "SMB", "arr_growth_pct": 45, "monthly_churn_pct": 2.5, "ltv_cac_ratio": 3.2, "nrr_pct": 105, "gross_margin_pct": 72}, {"company_size": "Mid-Market", "arr_growth_pct": 35, "monthly_churn_pct": 1.2, "ltv_cac_ratio": 4.1, "nrr_pct": 115, "gross_margin_pct": 78}]}`,
			dataFormat:  "application/json",
		},
		{
			sellerID:    seller2.ID,
			title:       "Global Supply Chain Disruption Index",
			description: "Weekly supply chain health scores across 15 major trade routes and 8 commodity categories. Includes port congestion data, shipping costs, lead time estimates, and disruption probability forecasts.",
			category:    "supply-chain",
			priceCents:  30000,
			tags:        []string{"supply chain", "logistics", "shipping", "trade", "disruption"},
			sampleData:  "route,commodity,health_score,congestion_days,shipping_cost_usd,lead_time_days,disruption_prob\nAsia-NorthAmerica,Electronics,72,4.2,4500,28,0.15\nEurope-Asia,Automotive,85,1.8,3200,21,0.08\nAsia-Europe,Textiles,68,5.1,3800,25,0.22\nSouthAmerica-NorthAmerica,Agriculture,91,0.9,2100,14,0.05",
			dataFormat:  "text/csv",
		},
		{
			sellerID:    seller1.ID,
			title:       "Cryptocurrency Market Microstructure Data",
			description: "Tick-level order book data for top 20 cryptocurrency pairs across 5 major exchanges. Includes bid-ask spreads, depth profiles, trade flow toxicity metrics, and market maker activity indicators.",
			category:    "crypto",
			priceCents:  50000,
			tags:        []string{"cryptocurrency", "trading", "order book", "market microstructure", "exchanges"},
			sampleData:  "exchange,pair,timestamp,bid,ask,spread_bps,depth_1pct_usd,toxicity_score\nBinance,BTC-USDT,2024-01-15T10:00:00Z,42150.50,42152.30,0.43,5200000,0.32\nCoinbase,ETH-USD,2024-01-15T10:00:00Z,2520.10,2520.85,0.30,1800000,0.28\nKraken,BTC-EUR,2024-01-15T10:00:00Z,38890.20,38893.50,0.85,2100000,0.41",
			dataFormat:  "text/csv",
		},
		{
			sellerID:    seller2.ID,
			title:       "Healthcare AI Patent Landscape Analysis",
			description: "Comprehensive patent analysis covering 5000+ AI/ML patents in healthcare from 2019-2024. Includes assignee mapping, technology clustering, citation networks, and white space analysis for drug discovery, diagnostics, and clinical trials.",
			category:    "healthcare",
			priceCents:  40000,
			tags:        []string{"healthcare", "AI", "patents", "drug discovery", "diagnostics"},
			sampleData:  `{"patent_landscape": {"total_patents": 5247, "top_assignees": ["Google Health", "IBM Watson", "Philips", "Siemens Healthineers"], "clusters": [{"name": "Drug Discovery ML", "count": 1230, "growth_rate": 0.45}, {"name": "Medical Imaging AI", "count": 980, "growth_rate": 0.32}]}}`,
			dataFormat:  "application/json",
		},
	}

	for _, l := range listings {
		tags, _ := json.Marshal(l.tags)
		listing, err := listingSvc.CreateListing(ctx, l.sellerID, l.title, l.description, l.category, l.priceCents, "usd", tags)
		if err != nil {
			log.Printf("failed to create listing %q: %v", l.title, err)
			continue
		}
		log.Printf("Created listing: %s (%s)", listing.Title, listing.ID)

		// Upload sample data
		reader := strings.NewReader(l.sampleData)
		if err := listingSvc.UploadData(ctx, listing.ID, cfg.MinIOBucket, reader, int64(len(l.sampleData)), l.dataFormat, "sample-data"); err != nil {
			log.Printf("failed to upload data for %q: %v", l.title, err)
		} else {
			log.Printf("  Uploaded data (%d bytes)", len(l.sampleData))
		}
	}

	log.Println("Seed complete!")
}

