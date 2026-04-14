package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
	"github.com/jmoiron/sqlx"
)

func RegisterRoutes(r chi.Router, listingSvc *service.ListingService, enterSvc *service.EnterService, purchaseSvc *service.PurchaseService, buyOrderSvc *service.BuyOrderService) {
	r.Use(CORS)

	r.Route("/api/v1", func(r chi.Router) {
		// Sellers
		sellerHandler := &SellerHandler{svc: listingSvc}
		r.Post("/sellers", sellerHandler.Create)
		r.Get("/sellers/{id}", sellerHandler.Get)

		// Listings
		listingHandler := &ListingHandler{svc: listingSvc}
		r.Post("/listings", listingHandler.Create)
		r.Get("/listings", listingHandler.List)
		r.Get("/listings/{id}", listingHandler.Get)
		r.Put("/listings/{id}", listingHandler.Update)
		r.Delete("/listings/{id}", listingHandler.Delete)
		r.Post("/listings/{id}/upload", listingHandler.Upload)

		// Enter
		enterHandler := &EnterHandler{svc: enterSvc}
		r.Post("/enter", enterHandler.Enter)

		// Purchases
		purchaseHandler := &PurchaseHandler{svc: purchaseSvc}
		r.Post("/purchases", purchaseHandler.Initiate)
		r.Post("/purchases/{id}/confirm", purchaseHandler.Confirm)
		r.Get("/purchases/{id}", purchaseHandler.GetStatus)
		r.Get("/ownership", purchaseHandler.ListOwnership)
		r.Get("/ownership/{listingID}/download", purchaseHandler.Download)

		// Buy Orders
		buyOrderHandler := &BuyOrderHandler{svc: buyOrderSvc}
		r.Post("/buy-orders", buyOrderHandler.Create)
		r.Get("/buy-orders", buyOrderHandler.List)
		r.Get("/buy-orders/{id}", buyOrderHandler.Get)
		r.Post("/buy-orders/{id}/fill", buyOrderHandler.Fill)
		r.Delete("/buy-orders/{id}", buyOrderHandler.Cancel)
	})
}

func ReadyHandler(db *sqlx.DB, opensearchURL, minioEndpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		checks := map[string]string{}

		// Check Postgres
		if err := db.DB.PingContext(r.Context()); err != nil {
			checks["postgres"] = fmt.Sprintf("error: %v", err)
		} else {
			checks["postgres"] = "ok"
		}

		// Check OpenSearch
		resp, err := http.Get(opensearchURL)
		if err != nil {
			checks["opensearch"] = fmt.Sprintf("error: %v", err)
		} else {
			resp.Body.Close()
			checks["opensearch"] = "ok"
		}

		// Check MinIO
		minioResp, err := http.Get(fmt.Sprintf("http://%s/minio/health/live", minioEndpoint))
		if err != nil {
			checks["minio"] = fmt.Sprintf("error: %v", err)
		} else {
			minioResp.Body.Close()
			checks["minio"] = "ok"
		}

		allOK := checks["postgres"] == "ok" && checks["opensearch"] == "ok" && checks["minio"] == "ok"
		status := http.StatusOK
		if !allOK {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": map[bool]string{true: "ok", false: "degraded"}[allOK],
			"checks": checks,
		})
	}
}

