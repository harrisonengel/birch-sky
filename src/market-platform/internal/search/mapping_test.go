package search

import "testing"

func TestIndexMappingFor_PipelineMode(t *testing.T) {
	m := IndexMappingFor("listings-embed")

	settings, ok := m["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("settings missing or wrong type: %T", m["settings"])
	}
	indexSettings, ok := settings["index"].(map[string]interface{})
	if !ok {
		t.Fatalf("index settings missing")
	}
	if got := indexSettings["default_pipeline"]; got != "listings-embed" {
		t.Errorf("expected default_pipeline=listings-embed, got %v", got)
	}
	if got, _ := indexSettings["knn"].(bool); !got {
		t.Errorf("expected index.knn=true")
	}

	// Embedding-source field is non-indexed text consumed by the
	// text_embedding processor.
	props := mustProps(t, m)
	src, _ := props[MLEmbeddingTextField].(map[string]interface{})
	if src == nil || src["index"] != false {
		t.Errorf("expected %s field with index:false, got %#v", MLEmbeddingTextField, src)
	}

	emb, _ := props[MLEmbeddingField].(map[string]interface{})
	if emb == nil {
		t.Fatalf("missing %s field", MLEmbeddingField)
	}
	if emb["type"] != "knn_vector" {
		t.Errorf("expected knn_vector, got %v", emb["type"])
	}
	if emb["dimension"] != EmbeddingDimension {
		t.Errorf("expected dimension=%d, got %v", EmbeddingDimension, emb["dimension"])
	}
}

func TestIndexMappingFor_NoPipeline(t *testing.T) {
	m := IndexMappingFor("")

	settings, _ := m["settings"].(map[string]interface{})
	indexSettings, _ := settings["index"].(map[string]interface{})
	if _, present := indexSettings["default_pipeline"]; present {
		t.Errorf("expected no default_pipeline when pipeline is disabled")
	}
}

func mustProps(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	mappings, _ := m["mappings"].(map[string]interface{})
	props, _ := mappings["properties"].(map[string]interface{})
	if props == nil {
		t.Fatalf("mappings.properties missing")
	}
	return props
}
