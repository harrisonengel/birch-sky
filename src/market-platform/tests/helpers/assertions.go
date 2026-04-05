package helpers

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func AssertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected status %d, got %d: %s", expected, resp.StatusCode, string(body))
	}
}

func AssertJSONField(t *testing.T, body []byte, field, expected string) {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	got, ok := m[field]
	if !ok {
		t.Fatalf("field %q not found in response", field)
	}
	if str, ok := got.(string); ok && str != expected {
		t.Fatalf("field %q: expected %q, got %q", field, expected, str)
	}
}

func ReadBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

func DecodeJSON(t *testing.T, body []byte, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decode JSON: %v (body: %s)", err, string(body))
	}
}
