//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/api"
	"github.com/harrisonengel/birch-sky/src/market-platform/tests/helpers"
)

func TestSellerEndpoints(t *testing.T) {
	ts := newTestServer(t)

	t.Run("CreateSeller_201", func(t *testing.T) {
		resp, err := postJSON(ts.URL+"/api/v1/sellers", map[string]string{
			"name":  "Alice",
			"email": "alice-create@example.com",
		})
		if err != nil {
			t.Fatalf("POST /sellers: %v", err)
		}
		helpers.AssertStatus(t, resp, http.StatusCreated)
		m := readJSON(t, resp)
		if m["name"] != "Alice" {
			t.Fatalf("expected name Alice, got %v", m["name"])
		}
		if m["id"] == nil || m["id"] == "" {
			t.Fatal("expected non-empty id")
		}
	})

	t.Run("DuplicateEmail_Conflict", func(t *testing.T) {
		email := "dup-email@example.com"
		resp, err := postJSON(ts.URL+"/api/v1/sellers", map[string]string{"name": "Bob", "email": email})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusCreated)
		io.ReadAll(resp.Body)
		resp.Body.Close()

		resp2, err := postJSON(ts.URL+"/api/v1/sellers", map[string]string{"name": "Bob Again", "email": email})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp2, http.StatusConflict)
	})

	t.Run("GetSeller_200", func(t *testing.T) {
		sellerID := createSellerViaAPI(t, ts.URL, "Charlie", "charlie-get@example.com")
		resp, err := http.Get(ts.URL + "/api/v1/sellers/" + sellerID)
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusOK)
		m := readJSON(t, resp)
		if m["id"] != sellerID {
			t.Fatalf("expected id %s, got %v", sellerID, m["id"])
		}
	})
}

func TestListingEndpoints(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerViaAPI(t, ts.URL, "ListingSeller", "listing-seller@example.com")

	t.Run("CreateListing_201", func(t *testing.T) {
		resp, err := postJSON(ts.URL+"/api/v1/listings", map[string]interface{}{
			"seller_id": sellerID, "title": "Weather Data Q1", "description": "Quarterly weather data.",
			"category": "weather", "price_cents": 5000, "currency": "usd", "tags": []string{"weather"},
		})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusCreated)
		m := readJSON(t, resp)
		id, ok := m["id"].(string)
		if !ok || len(id) != 36 {
			t.Fatalf("expected UUID id, got %v", m["id"])
		}
		if m["status"] != "active" {
			t.Fatalf("expected status active, got %v", m["status"])
		}
	})

	t.Run("MissingFields_400", func(t *testing.T) {
		resp, err := postJSON(ts.URL+"/api/v1/listings", map[string]interface{}{
			"seller_id": sellerID, "price_cents": 1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("NegativePrice_400", func(t *testing.T) {
		resp, err := postJSON(ts.URL+"/api/v1/listings", map[string]interface{}{
			"seller_id": sellerID, "title": "Bad", "description": "Neg price", "price_cents": -100,
		})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("EmptyTitle_400", func(t *testing.T) {
		resp, err := postJSON(ts.URL+"/api/v1/listings", map[string]interface{}{
			"seller_id": sellerID, "title": "", "description": "No title", "price_cents": 500,
		})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("GetListing_200", func(t *testing.T) {
		listingID := createListingViaAPI(t, ts.URL, sellerID, "Get Me", "Fetchable", "finance", 2000)
		resp, err := http.Get(ts.URL + "/api/v1/listings/" + listingID)
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusOK)
		m := readJSON(t, resp)
		if m["id"] != listingID {
			t.Fatalf("expected %s, got %v", listingID, m["id"])
		}
	})

	t.Run("GetNonexistent_404", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/listings/00000000-0000-0000-0000-000000000000")
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("ListPaginated", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			createListingViaAPI(t, ts.URL, sellerID, fmt.Sprintf("Page %d", i), "desc", "pagination-test", 1000+i)
		}
		resp, err := http.Get(ts.URL + "/api/v1/listings?limit=2&offset=0&category=pagination-test")
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusOK)
		body := helpers.ReadBody(t, resp)
		var page api.PaginatedResponse
		helpers.DecodeJSON(t, body, &page)
		if page.Limit != 2 {
			t.Fatalf("expected limit 2, got %d", page.Limit)
		}
		if page.Total < 5 {
			t.Fatalf("expected total >= 5, got %d", page.Total)
		}
	})

	t.Run("FilterByCategory", func(t *testing.T) {
		cat := "unique-cat-filter"
		createListingViaAPI(t, ts.URL, sellerID, "Cat A", "desc", cat, 100)
		createListingViaAPI(t, ts.URL, sellerID, "Cat B", "desc", cat, 200)
		createListingViaAPI(t, ts.URL, sellerID, "Other", "desc", "other-cat", 300)

		resp, err := http.Get(ts.URL + "/api/v1/listings?category=" + cat)
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusOK)
		body := helpers.ReadBody(t, resp)
		var page api.PaginatedResponse
		helpers.DecodeJSON(t, body, &page)
		if page.Total < 2 {
			t.Fatalf("expected total >= 2, got %d", page.Total)
		}
	})

	t.Run("UpdateListing_200", func(t *testing.T) {
		listingID := createListingViaAPI(t, ts.URL, sellerID, "Original", "Original desc", "update-test", 3000)
		resp, err := putJSON(ts.URL+"/api/v1/listings/"+listingID, map[string]interface{}{
			"title": "Updated Title",
		})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusOK)
		m := readJSON(t, resp)
		if m["title"] != "Updated Title" {
			t.Fatalf("expected Updated Title, got %v", m["title"])
		}
	})

	t.Run("SoftDelete_Then404", func(t *testing.T) {
		listingID := createListingViaAPI(t, ts.URL, sellerID, "To Delete", "Will be deleted", "delete-test", 500)
		resp, err := deleteReq(ts.URL + "/api/v1/listings/" + listingID)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		resp2, err := http.Get(ts.URL + "/api/v1/listings/" + listingID)
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp2, http.StatusNotFound)
	})

	t.Run("UploadFile_200", func(t *testing.T) {
		listingID := createListingViaAPI(t, ts.URL, sellerID, "Upload Test", "For upload", "upload-test", 7500)

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "data.csv")
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte("col1,col2\na,b\n1,2\n"))
		writer.Close()

		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/listings/"+listingID+"/upload", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusOK)

		// Verify data_ref is now set
		resp2, _ := http.Get(ts.URL + "/api/v1/listings/" + listingID)
		helpers.AssertStatus(t, resp2, http.StatusOK)
		m := readJSON(t, resp2)
		if m["data_ref"] == nil || m["data_ref"] == "" {
			t.Fatal("expected data_ref to be populated after upload")
		}
	})
}
