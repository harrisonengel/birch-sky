package search

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/domain"
)

type Indexer struct {
	engine   SearchEngine
	embedder Embedder
}

func NewIndexer(engine SearchEngine, embedder Embedder) *Indexer {
	return &Indexer{engine: engine, embedder: embedder}
}

func (idx *Indexer) Engine() SearchEngine {
	return idx.engine
}

func (idx *Indexer) IndexListing(ctx context.Context, listing *domain.Listing, dataReader io.Reader, dataFormat string) error {
	contentText := ""
	if dataReader != nil {
		var err error
		contentText, err = extractContent(dataReader, dataFormat)
		if err != nil {
			contentText = "" // non-fatal: log and continue
		}
	}

	// Build text for embedding
	tagsStr := ""
	if listing.Tags != nil {
		var tags []string
		json.Unmarshal(listing.Tags, &tags)
		tagsStr = strings.Join(tags, " ")
	}

	embeddingText := strings.Join([]string{listing.Title, listing.Description, tagsStr, contentText}, " ")
	embedding, err := idx.embedder.Embed(ctx, embeddingText)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}

	return idx.engine.IndexListing(ctx,
		listing.ID, listing.Title, listing.Description, listing.Category,
		string(listing.Status), listing.PriceCents, tagsStr, contentText, embedding)
}

func extractContent(reader io.Reader, format string) (string, error) {
	switch strings.ToLower(format) {
	case "csv", "text/csv":
		return extractCSV(reader)
	case "json", "application/json":
		return extractJSON(reader)
	default:
		return extractPlainText(reader)
	}
}

func extractCSV(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	var lines []string
	lineCount := 0
	for scanner.Scan() && lineCount <= 50 {
		lines = append(lines, scanner.Text())
		lineCount++
	}
	return strings.Join(lines, "\n"), scanner.Err()
}

func extractJSON(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, 50*1024))
	if err != nil {
		return "", err
	}

	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return string(data), nil // return raw if not valid JSON
	}

	keys := collectKeys(obj, "", 3)
	return strings.Join(keys, " "), nil
}

func collectKeys(v interface{}, prefix string, maxDepth int) []string {
	if maxDepth <= 0 {
		return nil
	}
	var keys []string
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			keys = append(keys, path)
			keys = append(keys, collectKeys(child, path, maxDepth-1)...)
		}
	case []interface{}:
		if len(val) > 0 {
			keys = append(keys, collectKeys(val[0], prefix, maxDepth-1)...)
		}
	case string:
		if len(val) < 200 {
			keys = append(keys, val)
		}
	}
	return keys
}

func extractPlainText(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, 50*1024))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
