//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/api"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/payments"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/search"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/objectstore"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
	"github.com/harrisonengel/birch-sky/src/market-platform/tests/helpers"
	"github.com/jmoiron/sqlx"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Shared test infrastructure — set up once in TestMain, used by all test files.
var (
	sharedDB       *sqlx.DB
	sharedObjStore objectstore.ObjectStore
	sharedEngine   *search.OpenSearchEngine
)

// MockPaymentProcessor satisfies payments.PaymentProcessor for tests.
type MockPaymentProcessor struct{}

var _ payments.PaymentProcessor = (*MockPaymentProcessor)(nil)

func (m *MockPaymentProcessor) CreatePaymentIntent(_ context.Context, amountCents int, currency string) (string, string, error) {
	return "cs_test_secret", "pi_test_" + uuid.New().String()[:8], nil
}

func (m *MockPaymentProcessor) ConfirmPayment(_ context.Context, _ string) error {
	return nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	var code int
	defer func() { os.Exit(code) }()

	// --- Postgres ---
	pgCtr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("iemarket_test"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres: %v\n", err)
		code = 1
		return
	}
	defer pgCtr.Terminate(ctx)

	pgHost, _ := pgCtr.Host(ctx)
	pgPort, _ := pgCtr.MappedPort(ctx, "5432")
	dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%s/iemarket_test?sslmode=disable", pgHost, pgPort.Port())

	sharedDB, err = postgres.Connect(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect db: %v\n", err)
		code = 1
		return
	}
	defer sharedDB.Close()

	if err := postgres.RunMigrations(sharedDB); err != nil {
		fmt.Fprintf(os.Stderr, "migrations: %v\n", err)
		code = 1
		return
	}

	// --- MinIO ---
	minioCtr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "start minio: %v\n", err)
		code = 1
		return
	}
	defer minioCtr.Terminate(ctx)

	minioHost, _ := minioCtr.Host(ctx)
	minioPort, _ := minioCtr.MappedPort(ctx, "9000")
	minioEndpoint := fmt.Sprintf("%s:%s", minioHost, minioPort.Port())

	sharedObjStore, err = objectstore.NewMinIOStore(minioEndpoint, "minioadmin", "minioadmin", "market-data", false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "minio store: %v\n", err)
		code = 1
		return
	}

	// --- OpenSearch ---
	osCtr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "opensearchproject/opensearch:3.0.0",
			ExposedPorts: []string{"9200/tcp"},
			Env: map[string]string{
				"discovery.type":            "single-node",
				"DISABLE_SECURITY_PLUGIN":   "true",
				"plugins.security.disabled": "true",
				"OPENSEARCH_JAVA_OPTS":      "-Xms256m -Xmx256m",
				"knn.plugin.enabled":        "true",
			},
			WaitingFor: wait.ForHTTP("/").WithPort("9200/tcp").WithStatusCodeMatcher(func(status int) bool {
				return status == 200
			}),
		},
		Started: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "start opensearch: %v\n", err)
		code = 1
		return
	}
	defer osCtr.Terminate(ctx)

	osHost, _ := osCtr.Host(ctx)
	osPort, _ := osCtr.MappedPort(ctx, "9200")
	osURL := fmt.Sprintf("http://%s:%s", osHost, osPort.Port())

	sharedEngine, err = search.NewOpenSearchEngine(osURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "opensearch engine: %v\n", err)
		code = 1
		return
	}
	if err := sharedEngine.EnsureIndex(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ensure index: %v\n", err)
		code = 1
		return
	}

	code = m.Run()
}

// newTestServer builds a fully-wired httptest.Server using the shared containers.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	sellerRepo := postgres.NewSellerRepo(sharedDB)
	listingRepo := postgres.NewListingRepo(sharedDB)
	txnRepo := postgres.NewTransactionRepo(sharedDB)
	ownRepo := postgres.NewOwnershipRepo(sharedDB)
	buyOrderRepo := postgres.NewBuyOrderRepo(sharedDB)

	embedder := search.NewLocalEmbedder()
	indexer := search.NewIndexer(sharedEngine, embedder)

	listingSvc := service.NewListingService(listingRepo, sellerRepo, sharedObjStore, indexer)
	turnMarketSvc := service.NewTurnMarketService(sharedEngine, embedder)
	purchaseSvc := service.NewPurchaseService(txnRepo, ownRepo, listingRepo, &MockPaymentProcessor{}, sharedObjStore, "market-data")
	buyOrderSvc := service.NewBuyOrderService(buyOrderRepo, listingRepo)

	r := chi.NewRouter()
	api.RegisterRoutes(r, listingSvc, turnMarketSvc, purchaseSvc, buyOrderSvc)

	ts := httptest.NewServer(r)
	t.Cleanup(func() { ts.Close() })
	return ts
}

// --- JSON helpers ---

func postJSON(url string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return http.Post(url, "application/json", bytes.NewReader(body))
}

func putJSON(url string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func deleteReq(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func readJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body := helpers.ReadBody(t, resp)
	var m map[string]interface{}
	helpers.DecodeJSON(t, body, &m)
	return m
}

func createSellerViaAPI(t *testing.T, baseURL, name, email string) string {
	t.Helper()
	resp, err := postJSON(baseURL+"/api/v1/sellers", map[string]string{
		"name":  name,
		"email": email,
	})
	if err != nil {
		t.Fatalf("POST /sellers: %v", err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	m := readJSON(t, resp)
	id, ok := m["id"].(string)
	if !ok || id == "" {
		t.Fatal("seller id missing")
	}
	return id
}

func createListingViaAPI(t *testing.T, baseURL, sellerID, title, description, category string, priceCents int) string {
	t.Helper()
	resp, err := postJSON(baseURL+"/api/v1/listings", map[string]interface{}{
		"seller_id":   sellerID,
		"title":       title,
		"description": description,
		"category":    category,
		"price_cents": priceCents,
		"currency":    "usd",
		"tags":        []string{"test"},
	})
	if err != nil {
		t.Fatalf("POST /listings: %v", err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	m := readJSON(t, resp)
	id, ok := m["id"].(string)
	if !ok || id == "" {
		t.Fatal("listing id missing")
	}
	return id
}
