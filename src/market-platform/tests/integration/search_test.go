//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/search"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
	"github.com/harrisonengel/birch-sky/src/market-platform/tests/helpers"
)

type searchReq struct {
	Query         string `json:"query"`
	Category      string `json:"category,omitempty"`
	MaxPriceCents *int   `json:"max_price_cents,omitempty"`
	Mode          string `json:"mode,omitempty"`
	PerPage       int    `json:"per_page,omitempty"`
}

type searchResp struct {
	Results []searchHit `json:"results"`
	Total   int         `json:"total"`
	Mode    string      `json:"mode"`
}

type searchHit struct {
	ListingID   string  `json:"listing_id"`
	Score       float64 `json:"score"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	PriceCents  int     `json:"price_cents"`
}

func doSearchHTTP(t *testing.T, baseURL string, req searchReq) (*http.Response, searchResp) {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(baseURL+"/api/v1/search", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /search: %v", err)
	}
	var sr searchResp
	if resp.StatusCode == http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		json.Unmarshal(respBody, &sr)
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
	}
	return resp, sr
}

func containsID(results []searchHit, id string) bool {
	for _, r := range results {
		if r.ListingID == id {
			return true
		}
	}
	return false
}

// createListingDirect uses the service layer to create a listing that's automatically indexed.
func createListingDirect(t *testing.T, sellerID, title, description, category string, priceCents int) string {
	t.Helper()
	sellerRepo := postgres.NewSellerRepo(sharedDB)
	listingRepo := postgres.NewListingRepo(sharedDB)
	embedder := search.NewLocalEmbedder()
	indexer := search.NewIndexer(sharedEngine, embedder)
	svc := service.NewListingService(listingRepo, sellerRepo, sharedObjStore, indexer)

	listing, err := svc.CreateListing(context.Background(), service.CreateListingInput{
		SellerID:    sellerID,
		Title:       title,
		Description: description,
		Category:    category,
		PriceCents:  priceCents,
		Currency:    "usd",
	})
	if err != nil {
		t.Fatalf("create listing %q: %v", title, err)
	}
	return listing.ID
}

func createSellerDirect(t *testing.T, name, email string) string {
	t.Helper()
	repo := postgres.NewSellerRepo(sharedDB)
	seller, err := repo.Create(context.Background(), name, email)
	if err != nil {
		t.Fatalf("create seller: %v", err)
	}
	return seller.ID
}

func TestSearch_EmptyIndex(t *testing.T) {
	ts := newTestServer(t)
	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "nonexistent query xyzzy42"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	if sr.Total != 0 {
		t.Fatalf("expected 0 results, got %d", sr.Total)
	}
}

func TestSearch_ExactTitleMatch(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "ExactTitle Seller", "exact-title@test.com")

	id1 := createListingDirect(t, sellerID, "Quantum Entanglement Dataset", "Experimental quantum physics measurements", "science", 5000)
	createListingDirect(t, sellerID, "Classical Mechanics Simulations", "Newtonian physics simulations", "science", 3000)
	createListingDirect(t, sellerID, "Organic Chemistry Reactions", "Reaction yield data", "science", 4000)

	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "Quantum Entanglement Dataset"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	if sr.Total == 0 {
		t.Fatal("expected at least 1 result")
	}
	if sr.Results[0].ListingID != id1 {
		t.Fatalf("expected top result %s, got %s", id1, sr.Results[0].ListingID)
	}
}

func TestSearch_CategoryFilter(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "Category Seller", "category-filter@test.com")

	createListingDirect(t, sellerID, "Financial Market Trends 2025", "Stock market analysis", "finance", 8000)
	healthID := createListingDirect(t, sellerID, "Health Market Trends 2025", "Healthcare market data", "health", 9000)

	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "Market Trends", Category: "health"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	if sr.Total == 0 {
		t.Fatal("expected results in health category")
	}
	for _, r := range sr.Results {
		if r.Category != "health" {
			t.Fatalf("expected category health, got %q", r.Category)
		}
	}
	if !containsID(sr.Results, healthID) {
		t.Fatalf("expected %s in results", healthID)
	}
}

func TestSearch_MaxPriceFilter(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "Price Seller", "price-filter@test.com")

	cheapID := createListingDirect(t, sellerID, "Affordable Geospatial Data", "Low-cost satellite imagery", "geo", 1000)
	createListingDirect(t, sellerID, "Premium Geospatial Data", "High-res satellite imagery", "geo", 50000)

	maxPrice := 5000
	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "Geospatial Data", MaxPriceCents: &maxPrice})
	helpers.AssertStatus(t, resp, http.StatusOK)
	for _, r := range sr.Results {
		if r.PriceCents > maxPrice {
			t.Fatalf("result %s price %d exceeds max %d", r.ListingID, r.PriceCents, maxPrice)
		}
	}
	if !containsID(sr.Results, cheapID) {
		t.Fatalf("expected %s in results", cheapID)
	}
}

func TestSearch_ModeText(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "TextMode Seller", "textmode@test.com")
	createListingDirect(t, sellerID, "Cryptocurrency Exchange Volumes", "Daily trading volumes", "crypto", 7000)

	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "Cryptocurrency Exchange Volumes", Mode: "text"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	if sr.Mode != "text" {
		t.Fatalf("expected mode=text, got %s", sr.Mode)
	}
	if sr.Total == 0 {
		t.Fatal("expected results")
	}
}

func TestSearch_ModeVector(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "VectorMode Seller", "vectormode@test.com")
	createListingDirect(t, sellerID, "Neural Network Benchmarks", "Performance benchmarks for transformers", "ai", 6000)

	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "Neural Network Benchmarks", Mode: "vector"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	if sr.Mode != "vector" {
		t.Fatalf("expected mode=vector, got %s", sr.Mode)
	}
	if sr.Total == 0 {
		t.Fatal("expected results")
	}
}

func TestSearch_ModeHybrid(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "HybridMode Seller", "hybridmode@test.com")
	createListingDirect(t, sellerID, "Climate Sensor Readings", "Temperature and humidity data", "climate", 4500)

	resp, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "Climate Sensor Readings", Mode: "hybrid"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	if sr.Mode != "hybrid" {
		t.Fatalf("expected mode=hybrid, got %s", sr.Mode)
	}
	if sr.Total == 0 {
		t.Fatal("expected results")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/v1/search", "application/json", strings.NewReader(`{"query": ""}`))
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestSearch_DeletedListingsExcluded(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "DeleteSearch Seller", "deletesearch@test.com")
	deletedID := createListingDirect(t, sellerID, "Obsolete Tidal Pattern Analysis", "Old tidal data", "ocean", 2000)

	// Verify it shows up
	_, srBefore := doSearchHTTP(t, ts.URL, searchReq{Query: "Obsolete Tidal Pattern Analysis"})
	if !containsID(srBefore.Results, deletedID) {
		t.Fatal("expected listing before deletion")
	}

	// Delete via API
	resp, _ := deleteReq(ts.URL + "/api/v1/listings/" + deletedID)
	resp.Body.Close()

	// Should no longer appear
	_, srAfter := doSearchHTTP(t, ts.URL, searchReq{Query: "Obsolete Tidal Pattern Analysis"})
	if containsID(srAfter.Results, deletedID) {
		t.Fatal("deleted listing should not appear")
	}
}

func TestSearch_CSVContentSearchable(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerDirect(t, "CSVSearch Seller", "csvsearch@test.com")
	listingID := createListingViaAPI(t, ts.URL, sellerID, "Demographic Survey Results", "Survey data from 2025", "demographics", 12000)

	csv := "respondent_id,annual_income_usd,zipcode,household_size\n1,55000,90210,3\n2,72000,10001,2\n"

	// Upload CSV
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "data.csv")
	part.Write([]byte(csv))
	writer.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/listings/"+listingID+"/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Search and look for column name
	_, sr := doSearchHTTP(t, ts.URL, searchReq{Query: "annual_income_usd"})
	if !containsID(sr.Results, listingID) {
		t.Fatalf("expected listing %s found via CSV column name", listingID)
	}
}
