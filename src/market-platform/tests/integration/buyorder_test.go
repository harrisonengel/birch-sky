//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/harrisonengel/birch-sky/src/market-platform/tests/helpers"
)

func TestCreateBuyOrder(t *testing.T) {
	ts := newTestServer(t)
	resp, err := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id":        "buyer_bo_" + uuid.New().String()[:6],
		"query":           "Consumer electronics pricing data",
		"max_price_cents": 10000,
		"category":        "electronics",
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	m := readJSON(t, resp)
	if m["id"] == nil || m["id"] == "" {
		t.Fatal("expected id")
	}
	if m["status"] != "open" {
		t.Fatalf("expected status open, got %v", m["status"])
	}
}

func TestListBuyOrdersPaginated(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_list_" + uuid.New().String()[:6]
	for i := 0; i < 3; i++ {
		resp, err := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
			"buyer_id":        buyerID,
			"query":           "test query",
			"max_price_cents": 5000,
		})
		if err != nil {
			t.Fatal(err)
		}
		helpers.AssertStatus(t, resp, http.StatusCreated)
		resp.Body.Close()
	}

	resp, err := http.Get(ts.URL + "/api/v1/buy-orders?buyer_id=" + buyerID + "&limit=2")
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	m := readJSON(t, resp)
	if m["total"] == nil {
		t.Fatal("expected total field")
	}
}

func TestGetBuyOrder(t *testing.T) {
	ts := newTestServer(t)
	resp, err := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id":        "buyer_get_" + uuid.New().String()[:6],
		"query":           "real estate data",
		"max_price_cents": 20000,
		"category":        "real-estate",
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	created := readJSON(t, resp)
	orderID := created["id"].(string)

	resp2, err := http.Get(ts.URL + "/api/v1/buy-orders/" + orderID)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp2, http.StatusOK)
	m := readJSON(t, resp2)
	if m["id"] != orderID {
		t.Fatalf("expected id %s, got %v", orderID, m["id"])
	}
}

func TestFillBuyOrderWithValidListing(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerViaAPI(t, ts.URL, "FillSeller-"+uuid.New().String()[:6], "fill-"+uuid.New().String()[:6]+"@test.com")
	listingID := createListingViaAPI(t, ts.URL, sellerID, "Fill Listing", "Data for filling", "finance", 5000)

	resp, err := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id":        "buyer_fill",
		"query":           "financial data",
		"max_price_cents": 10000,
		"category":        "finance",
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	created := readJSON(t, resp)
	orderID := created["id"].(string)

	resp2, err := postJSON(ts.URL+"/api/v1/buy-orders/"+orderID+"/fill", map[string]string{
		"listing_id": listingID,
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp2, http.StatusOK)
	filled := readJSON(t, resp2)
	if filled["status"] != "filled" {
		t.Fatalf("expected status filled, got %v", filled["status"])
	}
	if filled["filled_by_listing_id"] != listingID {
		t.Fatalf("expected filled_by_listing_id %s, got %v", listingID, filled["filled_by_listing_id"])
	}
}

func TestFillAlreadyFilledBuyOrderConflict(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerViaAPI(t, ts.URL, "FillTwice-"+uuid.New().String()[:6], "filltwice-"+uuid.New().String()[:6]+"@test.com")
	listingID := createListingViaAPI(t, ts.URL, sellerID, "Fill2 Listing", "Data", "test", 5000)

	resp, _ := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id": "buyer_fill2", "query": "data", "max_price_cents": 10000,
	})
	helpers.AssertStatus(t, resp, http.StatusCreated)
	orderID := readJSON(t, resp)["id"].(string)

	resp2, _ := postJSON(ts.URL+"/api/v1/buy-orders/"+orderID+"/fill", map[string]string{"listing_id": listingID})
	helpers.AssertStatus(t, resp2, http.StatusOK)
	resp2.Body.Close()

	resp3, err := postJSON(ts.URL+"/api/v1/buy-orders/"+orderID+"/fill", map[string]string{"listing_id": listingID})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp3, http.StatusConflict)
}

func TestCancelBuyOrder(t *testing.T) {
	ts := newTestServer(t)
	resp, _ := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id": "buyer_cancel", "query": "cancel test", "max_price_cents": 5000,
	})
	helpers.AssertStatus(t, resp, http.StatusCreated)
	orderID := readJSON(t, resp)["id"].(string)

	resp2, err := deleteReq(ts.URL + "/api/v1/buy-orders/" + orderID)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	resp3, _ := http.Get(ts.URL + "/api/v1/buy-orders/" + orderID)
	m := readJSON(t, resp3)
	if m["status"] != "cancelled" {
		t.Fatalf("expected cancelled, got %v", m["status"])
	}
}

func TestFillCancelledBuyOrderConflict(t *testing.T) {
	ts := newTestServer(t)
	sellerID := createSellerViaAPI(t, ts.URL, "FillCancel-"+uuid.New().String()[:6], "fillcancel-"+uuid.New().String()[:6]+"@test.com")
	listingID := createListingViaAPI(t, ts.URL, sellerID, "FC Listing", "Data", "test", 5000)

	resp, _ := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id": "buyer_fc", "query": "data", "max_price_cents": 10000,
	})
	helpers.AssertStatus(t, resp, http.StatusCreated)
	orderID := readJSON(t, resp)["id"].(string)

	deleteReq(ts.URL + "/api/v1/buy-orders/" + orderID)

	resp2, err := postJSON(ts.URL+"/api/v1/buy-orders/"+orderID+"/fill", map[string]string{"listing_id": listingID})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp2, http.StatusConflict)
}

func TestCreateBuyOrderZeroPriceBadRequest(t *testing.T) {
	ts := newTestServer(t)
	resp, err := postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id": "buyer_zero", "query": "data", "max_price_cents": 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusBadRequest)
}

func TestListBuyOrdersFilteredByBuyerID(t *testing.T) {
	ts := newTestServer(t)
	buyerA := "buyer_a_" + uuid.New().String()[:6]
	buyerB := "buyer_b_" + uuid.New().String()[:6]

	postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id": buyerA, "query": "data A", "max_price_cents": 5000,
	})
	postJSON(ts.URL+"/api/v1/buy-orders", map[string]interface{}{
		"buyer_id": buyerB, "query": "data B", "max_price_cents": 5000,
	})

	resp, err := http.Get(ts.URL + "/api/v1/buy-orders?buyer_id=" + buyerA)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	m := readJSON(t, resp)
	data, ok := m["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array")
	}
	for _, item := range data {
		order := item.(map[string]interface{})
		if order["buyer_id"] != buyerA {
			t.Fatalf("expected buyer_id %s, got %v", buyerA, order["buyer_id"])
		}
	}
}
