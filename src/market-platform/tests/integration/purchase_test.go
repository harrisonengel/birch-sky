//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/harrisonengel/birch-sky/src/market-platform/tests/helpers"
)

func createSellerAndListingForPurchase(t *testing.T, baseURL string) (string, string) {
	t.Helper()
	sellerID := createSellerViaAPI(t, baseURL, "PurchaseSeller-"+uuid.New().String()[:6], "purchase-"+uuid.New().String()[:6]+"@test.com")
	listingID := createListingViaAPI(t, baseURL, sellerID, "Test Dataset", "Purchase test data", "finance", 5000)

	// Upload a file so data_ref is set for download tests
	uploadBody := "--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"data.csv\"\r\nContent-Type: text/csv\r\n\r\ncol1,col2\nval1,val2\n\r\n--boundary--\r\n"
	req, _ := http.NewRequest("POST", baseURL+"/api/v1/listings/"+listingID+"/upload", strings.NewReader(uploadBody))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	return sellerID, listingID
}

func initiatePurchaseHelper(t *testing.T, baseURL, buyerID, listingID string) string {
	t.Helper()
	resp, err := postJSON(baseURL+"/api/v1/purchases", map[string]string{
		"buyer_id":   buyerID,
		"listing_id": listingID,
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	m := readJSON(t, resp)
	txnID, _ := m["transaction_id"].(string)
	return txnID
}

func confirmPurchaseHelper(t *testing.T, baseURL, txnID string) {
	t.Helper()
	resp, err := postJSON(baseURL+"/api/v1/purchases/"+txnID+"/confirm", nil)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
}

func TestInitiatePurchase(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)

	resp, err := postJSON(ts.URL+"/api/v1/purchases", map[string]string{
		"buyer_id":   buyerID,
		"listing_id": listingID,
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	m := readJSON(t, resp)

	if m["transaction_id"] == nil || m["transaction_id"] == "" {
		t.Fatal("expected transaction_id")
	}
	if m["client_secret"] == nil || m["client_secret"] == "" {
		t.Fatal("expected client_secret")
	}
	if m["already_owned"] == true {
		t.Fatal("expected already_owned=false")
	}
}

func TestConfirmPurchaseRecordsOwnership(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)

	txnID := initiatePurchaseHelper(t, ts.URL, buyerID, listingID)

	resp, err := postJSON(ts.URL+"/api/v1/purchases/"+txnID+"/confirm", nil)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	m := readJSON(t, resp)
	if m["buyer_id"] != buyerID {
		t.Fatalf("expected buyer_id %s, got %v", buyerID, m["buyer_id"])
	}
	if m["listing_id"] != listingID {
		t.Fatalf("expected listing_id %s, got %v", listingID, m["listing_id"])
	}
}

func TestGetPurchaseStatusCompleted(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)
	txnID := initiatePurchaseHelper(t, ts.URL, buyerID, listingID)
	confirmPurchaseHelper(t, ts.URL, txnID)

	resp, err := http.Get(ts.URL + "/api/v1/purchases/" + txnID)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	m := readJSON(t, resp)
	if m["status"] != "completed" {
		t.Fatalf("expected completed, got %v", m["status"])
	}
}

func TestListOwnership(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)
	txnID := initiatePurchaseHelper(t, ts.URL, buyerID, listingID)
	confirmPurchaseHelper(t, ts.URL, txnID)

	resp, err := http.Get(ts.URL + "/api/v1/ownership?buyer_id=" + buyerID)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	body := helpers.ReadBody(t, resp)
	var ownerships []map[string]interface{}
	helpers.DecodeJSON(t, body, &ownerships)
	if len(ownerships) == 0 {
		t.Fatal("expected at least one ownership record")
	}
}

func TestDownloadOwnedData(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)
	txnID := initiatePurchaseHelper(t, ts.URL, buyerID, listingID)
	confirmPurchaseHelper(t, ts.URL, txnID)

	resp, err := http.Get(ts.URL + "/api/v1/ownership/" + listingID + "/download?buyer_id=" + buyerID)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusOK)
	m := readJSON(t, resp)
	if m["download_url"] == nil || m["download_url"] == "" {
		t.Fatal("expected download_url")
	}
}

func TestDownloadUnownedDataForbidden(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)

	resp, err := http.Get(ts.URL + "/api/v1/ownership/" + listingID + "/download?buyer_id=" + buyerID)
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusForbidden)
}

func TestPurchaseNonexistentListing(t *testing.T) {
	ts := newTestServer(t)
	resp, err := postJSON(ts.URL+"/api/v1/purchases", map[string]string{
		"buyer_id":   "buyer_test",
		"listing_id": uuid.New().String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusNotFound)
}

func TestPurchaseAlreadyOwnedListing(t *testing.T) {
	ts := newTestServer(t)
	buyerID := "buyer_" + uuid.New().String()[:8]
	_, listingID := createSellerAndListingForPurchase(t, ts.URL)
	txnID := initiatePurchaseHelper(t, ts.URL, buyerID, listingID)
	confirmPurchaseHelper(t, ts.URL, txnID)

	// Second purchase attempt
	resp, err := postJSON(ts.URL+"/api/v1/purchases", map[string]string{
		"buyer_id":   buyerID,
		"listing_id": listingID,
	})
	if err != nil {
		t.Fatal(err)
	}
	helpers.AssertStatus(t, resp, http.StatusCreated)
	m := readJSON(t, resp)
	if m["already_owned"] != true {
		t.Fatal("expected already_owned=true")
	}
}
