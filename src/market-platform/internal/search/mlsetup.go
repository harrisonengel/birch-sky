package search

// OpenSearch ML Commons bootstrap.
//
// This package bootstraps a pre-trained sentence-transformer model inside
// the local OpenSearch cluster and wires up an ingest pipeline that
// auto-embeds listing text on write. With this in place, the application
// no longer needs to call out to any external embedding service (e.g.
// AWS Bedrock) for the demo / low-volume path — embeddings happen
// server-side as a normal OpenSearch ingest step.
//
// Flow on startup (idempotent):
//   1. Set ML Commons cluster settings to allow models on data nodes
//      and disable model-access-control for the demo.
//   2. Look up an already-registered+deployed model by name; if found,
//      reuse it. Otherwise:
//      a. Register (or reuse) a model group.
//      b. Register the pre-trained model from OpenSearch's HF mirror.
//      c. Wait for the registration task to complete.
//      d. Deploy the model and wait for the deploy task to complete.
//   3. Create the ingest pipeline (`listings-embed`) with a
//      text_embedding processor that maps `embedding_text` →
//      `embedding`.
//   4. Caller (OpenSearchEngine.EnsureIndex) sets the index's
//      `default_pipeline` to the pipeline name so every PUT /_doc
//      auto-embeds.
//
// We use `huggingface/sentence-transformers/all-MiniLM-L6-v2` because
// it is small (~80MB), fast on CPU, produces 384-dim vectors, and is
// a strong general-purpose English text embedding model. It ships as a
// pre-trained model in OpenSearch ML Commons so no manual upload is
// needed.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	// MLModelName is the OpenSearch ML Commons identifier for the
	// pre-trained sentence-transformer we embed listings with.
	MLModelName = "huggingface/sentence-transformers/all-MiniLM-L6-v2"

	// MLModelVersion pins the model release. OpenSearch's pre-trained
	// model catalog tracks versions; bump deliberately.
	MLModelVersion = "1.0.2"

	// MLModelFormat selects the runtime serialization. TORCH_SCRIPT is
	// the broadest-compatibility option for ML Commons.
	MLModelFormat = "TORCH_SCRIPT"

	// MLModelDimension is the embedding dimension produced by the model
	// above. all-MiniLM-L6-v2 outputs 384-dim vectors.
	MLModelDimension = 384

	// MLModelGroupName is the model group container we register the
	// model under. Reused on every boot.
	MLModelGroupName = "ie-embeddings"

	// MLPipelineName is the OpenSearch ingest pipeline that maps
	// embedding_text → embedding via the text_embedding processor.
	MLPipelineName = "listings-embed"

	// MLEmbeddingTextField is the source text field on each listing
	// document. The pipeline reads this and writes the resulting
	// vector to MLEmbeddingField.
	MLEmbeddingTextField = "embedding_text"

	// MLEmbeddingField is the knn_vector destination written by the
	// pipeline.
	MLEmbeddingField = "embedding"

	// mlSetupTimeout caps how long we wait for the model to register
	// + deploy. First-boot includes downloading ~80MB and loading it
	// into JVM-managed native memory, which can take a while on a
	// cold container.
	mlSetupTimeout = 5 * time.Minute
)

// MLClient drives the OpenSearch ML Commons + neural-search plugins.
// It is safe to use from concurrent goroutines once SetupModel has
// returned, because the only mutable field (modelID) is set once.
type MLClient struct {
	baseURL    string
	httpClient *http.Client
	modelID    string
}

// NewMLClient constructs an unbootstrapped client. Call SetupModel to
// register/deploy the model and learn its model_id.
func NewMLClient(baseURL string) *MLClient {
	return &MLClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ModelID returns the deployed model_id. Empty string until SetupModel
// has run successfully.
func (c *MLClient) ModelID() string {
	return c.modelID
}

// SetupModel performs the full bootstrap (cluster settings → model
// group → register → deploy → ingest pipeline). It is idempotent: a
// re-run after a successful first boot finds the existing deployed
// model and returns its id without re-downloading anything.
func (c *MLClient) SetupModel(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, mlSetupTimeout)
	defer cancel()

	if err := c.applyClusterSettings(ctx); err != nil {
		return fmt.Errorf("apply cluster settings: %w", err)
	}

	// Fast path: model is already registered + deployed from a
	// previous boot. Reuse it without spending the registration
	// budget again.
	if id, err := c.findDeployedModel(ctx); err == nil && id != "" {
		c.modelID = id
		log.Printf("ml: reusing deployed model %s (%s)", MLModelName, id)
	} else {
		groupID, err := c.ensureModelGroup(ctx)
		if err != nil {
			return fmt.Errorf("ensure model group: %w", err)
		}

		log.Printf("ml: registering pre-trained model %s@%s", MLModelName, MLModelVersion)
		registerTask, err := c.registerModel(ctx, groupID)
		if err != nil {
			return fmt.Errorf("register model: %w", err)
		}
		modelID, err := c.waitForTask(ctx, registerTask, "register")
		if err != nil {
			return fmt.Errorf("wait for register: %w", err)
		}

		log.Printf("ml: deploying model %s", modelID)
		deployTask, err := c.deployModel(ctx, modelID)
		if err != nil {
			return fmt.Errorf("deploy model: %w", err)
		}
		if _, err := c.waitForTask(ctx, deployTask, "deploy"); err != nil {
			return fmt.Errorf("wait for deploy: %w", err)
		}
		c.modelID = modelID
		log.Printf("ml: model deployed (%s)", modelID)
	}

	if err := c.ensurePipeline(ctx); err != nil {
		return fmt.Errorf("ensure pipeline: %w", err)
	}
	return nil
}

// applyClusterSettings flips the ML Commons knobs that let pre-trained
// models run on a single-node demo cluster (no dedicated ML node, no
// access control). Safe to call repeatedly; OpenSearch dedupes.
func (c *MLClient) applyClusterSettings(ctx context.Context) error {
	body := map[string]interface{}{
		"persistent": map[string]interface{}{
			"plugins.ml_commons.only_run_on_ml_node":           false,
			"plugins.ml_commons.model_access_control_enabled":  false,
			"plugins.ml_commons.native_memory_threshold":       99,
			"plugins.ml_commons.allow_registering_model_via_url": true,
		},
	}
	return c.doJSON(ctx, http.MethodPut, "/_cluster/settings", body, nil)
}

// findDeployedModel searches for an already-deployed model by name.
// Returns the model_id if exactly such a model exists, or "" if none.
func (c *MLClient) findDeployedModel(ctx context.Context) (string, error) {
	query := map[string]interface{}{
		"size": 5,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"name.keyword": MLModelName,
						},
					},
					map[string]interface{}{
						"terms": map[string]interface{}{
							"model_state": []string{"DEPLOYED", "PARTIALLY_DEPLOYED"},
						},
					},
				},
			},
		},
	}
	var resp struct {
		Hits struct {
			Hits []struct {
				ID     string                 `json:"_id"`
				Source map[string]interface{} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	// Search may 404 on a fresh cluster (the .plugins-ml-model index
	// is created lazily). Treat that as "no models yet".
	err := c.doJSON(ctx, http.MethodPost, "/_plugins/_ml/models/_search", query, &resp)
	if err != nil {
		return "", nil
	}
	for _, h := range resp.Hits.Hits {
		// Skip chunks (parent docs only) — chunks live under the
		// same index but represent shards of the model binary.
		if _, isChunk := h.Source["chunk_number"]; isChunk {
			continue
		}
		return h.ID, nil
	}
	return "", nil
}

// ensureModelGroup creates the model group if missing and returns its
// id. ML Commons rejects duplicate names with 400; we treat that as
// "already exists" and look it up via search instead.
func (c *MLClient) ensureModelGroup(ctx context.Context) (string, error) {
	body := map[string]interface{}{
		"name":        MLModelGroupName,
		"description": "Information Exchange embedding models",
	}
	var resp struct {
		ModelGroupID string `json:"model_group_id"`
		Status       string `json:"status"`
	}
	err := c.doJSON(ctx, http.MethodPost, "/_plugins/_ml/model_groups/_register", body, &resp)
	if err == nil && resp.ModelGroupID != "" {
		return resp.ModelGroupID, nil
	}

	// Fall back to looking up the existing group.
	query := map[string]interface{}{
		"size": 1,
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"name.keyword": MLModelGroupName,
			},
		},
	}
	var search struct {
		Hits struct {
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if serr := c.doJSON(ctx, http.MethodPost, "/_plugins/_ml/model_groups/_search", query, &search); serr != nil {
		return "", fmt.Errorf("register failed (%v) and lookup failed: %w", err, serr)
	}
	if len(search.Hits.Hits) == 0 {
		return "", fmt.Errorf("model group %q neither created nor found: %w", MLModelGroupName, err)
	}
	return search.Hits.Hits[0].ID, nil
}

// registerModel kicks off async registration of the pre-trained model
// and returns the task_id to poll.
func (c *MLClient) registerModel(ctx context.Context, groupID string) (string, error) {
	body := map[string]interface{}{
		"name":           MLModelName,
		"version":        MLModelVersion,
		"model_group_id": groupID,
		"model_format":   MLModelFormat,
	}
	var resp struct {
		TaskID string `json:"task_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/_plugins/_ml/models/_register", body, &resp); err != nil {
		return "", err
	}
	if resp.TaskID == "" {
		return "", fmt.Errorf("no task_id returned from register")
	}
	return resp.TaskID, nil
}

// deployModel asks OpenSearch to load the model into native memory.
// Returns the task_id to poll.
func (c *MLClient) deployModel(ctx context.Context, modelID string) (string, error) {
	var resp struct {
		TaskID string `json:"task_id"`
	}
	path := fmt.Sprintf("/_plugins/_ml/models/%s/_deploy", modelID)
	if err := c.doJSON(ctx, http.MethodPost, path, nil, &resp); err != nil {
		return "", err
	}
	if resp.TaskID == "" {
		return "", fmt.Errorf("no task_id returned from deploy")
	}
	return resp.TaskID, nil
}

// waitForTask polls a task to completion. Returns the model_id field
// from the final task document (set on register tasks; empty on deploy
// tasks but caller already knows the id).
func (c *MLClient) waitForTask(ctx context.Context, taskID, label string) (string, error) {
	path := fmt.Sprintf("/_plugins/_ml/tasks/%s", taskID)
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(mlSetupTimeout)
	}
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("%s task %s did not finish before deadline: %w", label, taskID, ctx.Err())
		default:
		}
		var resp struct {
			State   string `json:"state"`
			ModelID string `json:"model_id"`
			Error   string `json:"error"`
		}
		if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return "", err
		}
		switch resp.State {
		case "COMPLETED":
			return resp.ModelID, nil
		case "FAILED", "STOPPED":
			return "", fmt.Errorf("%s task %s ended with state %s: %s", label, taskID, resp.State, resp.Error)
		}
		// Still running; sleep a bit then poll again, but no
		// longer than the remaining deadline.
		sleep := 2 * time.Second
		if remaining := time.Until(deadline); remaining < sleep {
			sleep = remaining
		}
		if sleep <= 0 {
			return "", fmt.Errorf("%s task %s timed out", label, taskID)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(sleep):
		}
	}
}

// ensurePipeline creates the listings-embed ingest pipeline. PUT is
// upsert semantics so this is safe to repeat.
func (c *MLClient) ensurePipeline(ctx context.Context) error {
	if c.modelID == "" {
		return fmt.Errorf("modelID not set; SetupModel must succeed first")
	}
	body := map[string]interface{}{
		"description": "Auto-embed listing text into a knn_vector field",
		"processors": []interface{}{
			map[string]interface{}{
				"text_embedding": map[string]interface{}{
					"model_id": c.modelID,
					"field_map": map[string]interface{}{
						MLEmbeddingTextField: MLEmbeddingField,
					},
				},
			},
		},
	}
	return c.doJSON(ctx, http.MethodPut, "/_ingest/pipeline/"+MLPipelineName, body, nil)
}

// doJSON issues a JSON request against OpenSearch and decodes the
// response into out (if non-nil). Non-2xx responses become errors with
// the body included for debugging.
func (c *MLClient) doJSON(ctx context.Context, method, path string, body, out interface{}) error {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s failed (%d): %s", method, path, resp.StatusCode, string(respBody))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
