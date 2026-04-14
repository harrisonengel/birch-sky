// Command server runs the market-platform service.
//
// It is the single binary for the marketplace backend. It exposes two
// network surfaces:
//
//   - HTTP API on cfg.HTTPPort (default :8080) — REST endpoints for
//     sellers, listings, search, purchases, and buy orders. This is what
//     human-facing tools, internal services, and CLI tooling call.
//   - MCP server on cfg.MCPPort (default :8081) — Model Context Protocol
//     SSE transport that exposes search/get/analyze tools to buyer agents
//     running inside the agent sandbox.
//
// On startup the binary connects to Postgres and runs any pending embedded
// migrations, then wires up the object store, search engine, embedder,
// payment processor, services, and HTTP routes via constructor injection.
// All credentials and connection strings come from the environment via
// internal/config; see .env.example for the full list.
//
// Shutdown is triggered by SIGINT or SIGTERM and gives in-flight HTTP
// requests up to 10 seconds to drain before the process exits.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/api"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/config"
	mcpserver "github.com/harrisonengel/birch-sky/src/market-platform/internal/mcp"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/payments"
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

	// Database. Credentials, host, and database name are all carried in
	// cfg.DatabaseURL (a libpq-format connection string loaded from the
	// DATABASE_URL env var, e.g.
	// "postgres://user:password@host:5432/dbname?sslmode=require"). Connect
	// does not bypass auth — Postgres rejects the connection if the URL
	// lacks valid credentials. The local docker-compose ships a development
	// password for convenience; production deployments must inject real
	// secrets via env (see follow-up issue on env var deployment strategy).
	db, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations complete")

	// Repositories. These are Go data-access objects (the "repository"
	// pattern), not git repos or new database schemas — each one is a thin
	// struct that holds a reference to the shared *sqlx.DB and exposes
	// typed CRUD methods over an existing table created by the migrations
	// above. No new persistent state is created here.
	sellerRepo := postgres.NewSellerRepo(db)
	listingRepo := postgres.NewListingRepo(db)
	transactionRepo := postgres.NewTransactionRepo(db)
	ownershipRepo := postgres.NewOwnershipRepo(db)
	buyOrderRepo := postgres.NewBuyOrderRepo(db)

	// Object store
	objStore, err := objectstore.NewMinIOStore(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}

	// Search
	var embedder search.Embedder
	if cfg.HasBedrock() {
		embedder, err = search.NewBedrockEmbedder(cfg.AWSRegion)
		if err != nil {
			log.Fatalf("bedrock embedder: %v", err)
		}
	} else {
		embedder = search.NewLocalEmbedder()
		log.Println("using local embedder (no AWS credentials configured)")
	}

	searchEngine, err := search.NewOpenSearchEngine(cfg.OpenSearchURL)
	if err != nil {
		log.Fatalf("opensearch: %v", err)
	}
	if err := searchEngine.EnsureIndex(context.Background()); err != nil {
		log.Fatalf("opensearch index: %v", err)
	}

	indexer := search.NewIndexer(searchEngine, embedder)

	// Payments
	var paymentProcessor payments.PaymentProcessor
	if cfg.StripeKey != "" {
		paymentProcessor = payments.NewStripeProcessor(cfg.StripeKey)
	} else {
		paymentProcessor = payments.NewStubProcessor()
		log.Println("using stub payment processor (no STRIPE_SECRET_KEY configured)")
	}

	// Services
	listingSvc := service.NewListingService(listingRepo, sellerRepo, objStore, indexer)
	searchSvc := service.NewSearchService(searchEngine, embedder)
	purchaseSvc := service.NewPurchaseService(transactionRepo, ownershipRepo, listingRepo, paymentProcessor, objStore, cfg.MinIOBucket)
	buyOrderSvc := service.NewBuyOrderService(buyOrderRepo, listingRepo)

	// HTTP router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/ready", api.ReadyHandler(db, cfg.OpenSearchURL, cfg.MinIOEndpoint))

	api.RegisterRoutes(r, listingSvc, searchSvc, purchaseSvc, buyOrderSvc)

	// MCP server (separate goroutine)
	var analyzer mcpserver.DataAnalyzer
	if cfg.AnthropicKey != "" {
		analyzer = mcpserver.NewClaudeAnalyzer(cfg.AnthropicKey)
	} else {
		analyzer = mcpserver.NewStubAnalyzer()
		log.Println("using stub data analyzer (no ANTHROPIC_API_KEY configured)")
	}

	go func() {
		mcpAddr := fmt.Sprintf(":%d", cfg.MCPPort)
		log.Printf("MCP server listening on %s", mcpAddr)
		if err := mcpserver.Serve(mcpAddr, searchSvc, listingRepo, analyzer, objStore, cfg.MinIOBucket); err != nil {
			log.Printf("MCP server error: %v", err)
		}
	}()

	// HTTP server with graceful shutdown
	httpAddr := fmt.Sprintf(":%d", cfg.HTTPPort)
	srv := &http.Server{
		Addr:    httpAddr,
		Handler: r,
	}

	go func() {
		log.Printf("HTTP server listening on %s", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
