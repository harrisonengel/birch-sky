package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/service"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/objectstore"
	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type DataAnalyzer interface {
	Analyze(ctx context.Context, data io.Reader, questions []string) ([]string, error)
}

// toolDeps holds shared dependencies for tool handlers.
type toolDeps struct {
	turnMarketSvc *service.TurnMarketService
	listingRepo *postgres.ListingRepo
	analyzer    DataAnalyzer
	objStore    objectstore.ObjectStore
	bucket      string
}

func Serve(addr string, turnMarketSvc *service.TurnMarketService, listingRepo *postgres.ListingRepo, analyzer DataAnalyzer, objStore objectstore.ObjectStore, bucket string) error {
	s := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "Information Exchange Market Platform",
		Version: "1.0.0",
	}, nil)

	deps := &toolDeps{
		turnMarketSvc: turnMarketSvc,
		listingRepo: listingRepo,
		analyzer:    analyzer,
		objStore:    objStore,
		bucket:      bucket,
	}

	registerEnterTool(s, deps)
	registerGetListingTool(s, deps)
	registerAnalyzeDataTool(s, deps)

	handler := mcpsdk.NewSSEHandler(func(r *http.Request) *mcpsdk.Server {
		return s
	}, nil)
	return http.ListenAndServe(addr, handler)
}

// --- enter_marketplace ---

type EnterInput struct {
	Query         string  `json:"query" jsonschema:"Natural language query"`
	Category      *string `json:"category,omitempty" jsonschema:"Filter by category"`
	MaxPriceCents *int    `json:"max_price_cents,omitempty" jsonschema:"Maximum price in cents"`
}

type EnterOutput struct {
	Text string `json:"text"`
}

func registerEnterTool(s *mcpsdk.Server, deps *toolDeps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "enter_marketplace",
		Description: "Enter the Information Exchange marketplace to find data listings using natural language. Returns matching listings ranked by relevance.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input EnterInput) (*mcpsdk.CallToolResult, EnterOutput, error) {
		if input.Query == "" {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("query is required"))
			return result, EnterOutput{}, nil
		}

		enterReq := service.EnterRequest{
			Query: input.Query,
			Mode:  "hybrid",
		}
		if input.Category != nil {
			enterReq.Category = *input.Category
		}
		if input.MaxPriceCents != nil {
			enterReq.MaxPriceCents = input.MaxPriceCents
		}

		resp, err := deps.turnMarketSvc.Enter(ctx, enterReq)
		if err != nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("enter failed: %v", err))
			return result, EnterOutput{}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d results:\n\n", resp.Total))
		for i, r := range resp.Results {
			sb.WriteString(fmt.Sprintf("%d. **%s** (ID: %s)\n", i+1, r.Title, r.ListingID))
			sellerInfo := r.Category
			if r.SellerName != "" {
				sellerInfo = r.SellerName + " | " + r.Category
			}
			sb.WriteString(fmt.Sprintf("   %s | Price: $%.2f | Score: %.4f\n", sellerInfo, float64(r.PriceCents)/100, r.Score))
			sb.WriteString(fmt.Sprintf("   %s\n\n", r.Description))
		}

		text := sb.String()
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
		}, EnterOutput{Text: text}, nil
	})
}

// --- get_listing ---

type GetListingInput struct {
	ListingID string `json:"listing_id" jsonschema:"The listing UUID"`
}

type GetListingOutput struct {
	Text string `json:"text"`
}

func registerGetListingTool(s *mcpsdk.Server, deps *toolDeps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "get_listing",
		Description: "Get full public metadata for a listing by its ID.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input GetListingInput) (*mcpsdk.CallToolResult, GetListingOutput, error) {
		if input.ListingID == "" {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("listing_id is required"))
			return result, GetListingOutput{}, nil
		}

		listing, err := deps.listingRepo.GetByID(ctx, input.ListingID)
		if err != nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("error: %v", err))
			return result, GetListingOutput{}, nil
		}
		if listing == nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("listing not found"))
			return result, GetListingOutput{}, nil
		}

		text := fmt.Sprintf("**%s**\n\nID: %s\nSeller: %s\nCategory: %s\nPrice: $%.2f\nStatus: %s\nFormat: %s\nSize: %d bytes\n\n%s",
			listing.Title, listing.ID, listing.SellerID, listing.Category,
			float64(listing.PriceCents)/100, listing.Status, listing.DataFormat,
			listing.DataSizeBytes, listing.Description)

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
		}, GetListingOutput{Text: text}, nil
	})
}

// --- analyze_data ---

type AnalyzeDataInput struct {
	ListingID string   `json:"listing_id" jsonschema:"The listing UUID whose data to analyze"`
	Questions []string `json:"questions" jsonschema:"Questions to ask about the data"`
}

type AnalyzeDataOutput struct {
	Text string `json:"text"`
}

func registerAnalyzeDataTool(s *mcpsdk.Server, deps *toolDeps) {
	mcpsdk.AddTool(s, &mcpsdk.Tool{
		Name:        "analyze_data",
		Description: "Ask questions about a dataset without seeing the raw data. The service loads the data and uses AI to answer your questions, preserving data confidentiality.",
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest, input AnalyzeDataInput) (*mcpsdk.CallToolResult, AnalyzeDataOutput, error) {
		if input.ListingID == "" {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("listing_id is required"))
			return result, AnalyzeDataOutput{}, nil
		}
		if len(input.Questions) == 0 {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("at least one question is required"))
			return result, AnalyzeDataOutput{}, nil
		}

		listing, err := deps.listingRepo.GetByID(ctx, input.ListingID)
		if err != nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("error: %v", err))
			return result, AnalyzeDataOutput{}, nil
		}
		if listing == nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("listing not found"))
			return result, AnalyzeDataOutput{}, nil
		}
		if listing.DataRef == "" {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("listing has no uploaded data"))
			return result, AnalyzeDataOutput{}, nil
		}

		dataReader, err := deps.objStore.Download(ctx, deps.bucket, listing.DataRef)
		if err != nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("failed to load data: %v", err))
			return result, AnalyzeDataOutput{}, nil
		}
		defer dataReader.Close()

		answers, err := deps.analyzer.Analyze(ctx, dataReader, input.Questions)
		if err != nil {
			result := &mcpsdk.CallToolResult{}
			result.SetError(fmt.Errorf("analysis failed: %v", err))
			return result, AnalyzeDataOutput{}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Analysis of listing: %s\n\n", listing.Title))
		for i, q := range input.Questions {
			sb.WriteString(fmt.Sprintf("**Q: %s**\n", q))
			if i < len(answers) {
				sb.WriteString(fmt.Sprintf("A: %s\n\n", answers[i]))
			}
		}

		text := sb.String()
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: text}},
		}, AnalyzeDataOutput{Text: text}, nil
	})
}
