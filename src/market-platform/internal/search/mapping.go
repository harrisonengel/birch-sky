package search

const IndexName = "listings"

// EmbeddingDimension is the dimensionality of the vectors stored in
// the `embedding` knn_vector field. It must match the output of the
// embedder in use.
//
// We default to 384 because the OpenSearch-managed pre-trained model
// (sentence-transformers/all-MiniLM-L6-v2 — see mlsetup.go) produces
// 384-dim vectors. The Bedrock Titan v2 path produces 1024-dim
// vectors, so production deployments that flip back to Bedrock must
// also recreate the index with this constant set to 1024.
const EmbeddingDimension = MLModelDimension

// IndexMappingFor builds the index settings + mapping. When
// defaultPipeline is non-empty, OpenSearch will run it on every
// indexed document — that's how we get server-side embedding via the
// text_embedding processor without the application having to call any
// embedding service itself.
func IndexMappingFor(defaultPipeline string) map[string]interface{} {
	indexSettings := map[string]interface{}{
		"knn": true,
	}
	if defaultPipeline != "" {
		indexSettings["default_pipeline"] = defaultPipeline
	}

	return map[string]interface{}{
		"settings": map[string]interface{}{
			"index": indexSettings,
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"listing_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "standard",
						"filter":    []string{"lowercase", "stop", "snowball"},
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":     "text",
					"analyzer": "listing_analyzer",
				},
				"description": map[string]interface{}{
					"type":     "text",
					"analyzer": "listing_analyzer",
				},
				"tags": map[string]interface{}{
					"type":     "text",
					"analyzer": "listing_analyzer",
				},
				"content_text": map[string]interface{}{
					"type":     "text",
					"analyzer": "listing_analyzer",
				},
				// embedding_text is the source field consumed by the
				// text_embedding ingest processor. We mark it as
				// non-searchable text since the indexed analyzed form
				// is redundant with the per-field text fields above —
				// it exists purely to feed the pipeline.
				MLEmbeddingTextField: map[string]interface{}{
					"type":  "text",
					"index": false,
				},
				MLEmbeddingField: map[string]interface{}{
					"type":      "knn_vector",
					"dimension": EmbeddingDimension,
					"method": map[string]interface{}{
						"name":       "hnsw",
						"space_type": "cosinesimil",
						"engine":     "lucene",
					},
				},
				"category": map[string]interface{}{
					"type": "keyword",
				},
				"status": map[string]interface{}{
					"type": "keyword",
				},
				"price_cents": map[string]interface{}{
					"type": "integer",
				},
				"listing_id": map[string]interface{}{
					"type": "keyword",
				},
				"seller_name": map[string]interface{}{
					"type": "keyword",
				},
			},
		},
	}
}
