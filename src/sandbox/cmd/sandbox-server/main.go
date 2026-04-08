// Command sandbox-server runs the Models Sandbox MVP.
//
// It is a single binary that exposes the public HTTP API on
// SANDBOX_HTTP_PORT (default :8090) and runs the harness worker pool
// in the same process. The MVP wires everything to in-memory stores
// and the deterministic stub LLM so it boots with zero configuration:
//
//	go run ./cmd/sandbox-server
//
// Set SANDBOX_API_KEY to require an API key on protected endpoints.
// Set SANDBOX_WORKERS to scale the harness worker pool.
//
// Replacing the in-memory stores with the eventual Postgres + SQS
// implementations is a localized change in this file — `Store` and
// `Queue` are interfaces.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/harrisonengel/birch-sky/src/sandbox/internal/api"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/audit"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/config"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/harness"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/jobstore"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/llm"
	"github.com/harrisonengel/birch-sky/src/sandbox/internal/toolproxy"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// In-memory stores. Swap with Postgres + SQS later.
	store := jobstore.NewMemoryStore()
	queue := jobstore.NewChannelQueue(cfg.QueueBuffer)
	logger := audit.NewMemoryLogger()

	// Tool proxy with the hello-world data source.
	proxy := toolproxy.NewProxy(logger, toolproxy.NewProviderDirectorySource())

	// Harness engine.
	registry := harness.NewRegistry()
	model := llm.NewStubClient()
	engine := harness.NewEngine(registry, model, proxy, logger)

	// Worker pool.
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	var workerWg sync.WaitGroup
	for i := 0; i < cfg.WorkerCount; i++ {
		w := harness.NewWorker(i, store, queue, engine, logger)
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			w.Start(workerCtx)
		}()
	}

	// HTTP server.
	srv := &api.Server{
		APIKey:   cfg.APIKey,
		Store:    store,
		Queue:    queue,
		Registry: registry,
		Audit:    logger,
	}

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("sandbox-server listening on %s (workers=%d, api_key=%v)",
			httpSrv.Addr, cfg.WorkerCount, cfg.APIKey != "")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	cancelWorkers()
	workerWg.Wait()
}
