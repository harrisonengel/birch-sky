package search

const IndexName = "listings"

var IndexMapping = map[string]interface{}{
	"settings": map[string]interface{}{
		"index": map[string]interface{}{
			"knn": true,
		},
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
			"embedding": map[string]interface{}{
				"type":      "knn_vector",
				"dimension": EmbeddingDimension,
				"method": map[string]interface{}{
					"name":       "hnsw",
					"space_type": "cosinesimil",
					"engine":     "nmslib",
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
