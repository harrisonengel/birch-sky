package main

// `iecli ml-status` — quick read-out of the OpenSearch ML embedding
// pipeline so an operator can verify that semantic search is wired up
// without spelunking the OpenSearch REST API by hand.
//
// It hits OpenSearch directly (not the market-platform API) because
// ML state lives in cluster metadata, not in the application.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func mlStatusCmd() *cobra.Command {
	var osURL string
	cmd := &cobra.Command{
		Use:   "ml-status",
		Short: "Show the OpenSearch ML embedding model + ingest pipeline status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMLStatus(osURL)
		},
	}
	cmd.Flags().StringVar(&osURL, "opensearch", defaultOSURL(), "OpenSearch base URL")
	return cmd
}

func defaultOSURL() string {
	if v := os.Getenv("OPENSEARCH_URL"); v != "" {
		return v
	}
	return "http://localhost:9200"
}

func runMLStatus(osURL string) error {
	osURL = strings.TrimRight(osURL, "/")

	// 1. Find the deployed embedding model by name.
	const modelName = "huggingface/sentence-transformers/all-MiniLM-L6-v2"
	const pipelineName = "listings-embed"
	const indexName = "listings"

	modelQuery := map[string]interface{}{
		"size": 5,
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"name.keyword": modelName,
			},
		},
	}
	modelResp, err := postJSONTo(osURL+"/_plugins/_ml/models/_search", modelQuery)
	if err != nil {
		return fmt.Errorf("model search: %w", err)
	}

	fmt.Printf("OpenSearch:    %s\n", osURL)
	fmt.Printf("Model:         %s\n", modelName)

	var modelID, modelState string
	if hits, ok := digHits(modelResp); ok && len(hits) > 0 {
		for _, h := range hits {
			src, _ := h["_source"].(map[string]interface{})
			// Skip chunk shards.
			if _, isChunk := src["chunk_number"]; isChunk {
				continue
			}
			modelID, _ = h["_id"].(string)
			modelState, _ = src["model_state"].(string)
			break
		}
	}
	if modelID == "" {
		fmt.Println("Model state:   NOT REGISTERED — start market-platform to bootstrap")
	} else {
		fmt.Printf("Model id:      %s\n", modelID)
		fmt.Printf("Model state:   %s\n", modelState)
	}

	// 2. Pipeline.
	pipeResp, status, err := getJSON(osURL + "/_ingest/pipeline/" + pipelineName)
	if err != nil {
		return fmt.Errorf("pipeline get: %w", err)
	}
	switch {
	case status == 404:
		fmt.Printf("Pipeline:      %s — NOT CREATED\n", pipelineName)
	case status == 200:
		fmt.Printf("Pipeline:      %s — present\n", pipelineName)
		if buf, err := json.MarshalIndent(pipeResp, "  ", "  "); err == nil {
			fmt.Printf("  %s\n", string(buf))
		}
	default:
		fmt.Printf("Pipeline:      %s — unexpected status %d\n", pipelineName, status)
	}

	// 3. Index settings (does default_pipeline point at our pipeline?).
	idxResp, status, err := getJSON(osURL + "/" + indexName + "/_settings")
	if err != nil {
		return fmt.Errorf("index settings: %w", err)
	}
	if status != 200 {
		fmt.Printf("Index:         %s — status %d\n", indexName, status)
		return nil
	}
	if root, ok := idxResp[indexName].(map[string]interface{}); ok {
		if settings, ok := root["settings"].(map[string]interface{}); ok {
			if idx, ok := settings["index"].(map[string]interface{}); ok {
				dp, _ := idx["default_pipeline"].(string)
				if dp == "" {
					fmt.Printf("Index:         %s — no default_pipeline set\n", indexName)
				} else {
					fmt.Printf("Index:         %s — default_pipeline=%s\n", indexName, dp)
				}
			}
		}
	}

	// 4. Doc count for sanity.
	countResp, _, err := getJSON(osURL + "/" + indexName + "/_count")
	if err == nil {
		if n, ok := countResp["count"].(float64); ok {
			fmt.Printf("Doc count:     %d\n", int(n))
		}
	}

	return nil
}

func postJSONTo(url string, payload interface{}) (map[string]interface{}, error) {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s -> %d: %s", url, resp.StatusCode, string(data))
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func getJSON(url string) (map[string]interface{}, int, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 404 {
		return nil, 404, nil
	}
	var out map[string]interface{}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	return out, resp.StatusCode, nil
}

func digHits(m map[string]interface{}) ([]map[string]interface{}, bool) {
	hits, ok := m["hits"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	inner, ok := hits["hits"].([]interface{})
	if !ok {
		return nil, false
	}
	out := make([]map[string]interface{}, 0, len(inner))
	for _, h := range inner {
		if hm, ok := h.(map[string]interface{}); ok {
			out = append(out, hm)
		}
	}
	return out, true
}
