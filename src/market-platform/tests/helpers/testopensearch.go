package helpers

import (
	"context"
	"fmt"
	"testing"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/search"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestOpenSearch struct {
	Engine    *search.OpenSearchEngine
	URL       string
	Container testcontainers.Container
}

func SetupTestOpenSearch(t *testing.T) *TestOpenSearch {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "opensearchproject/opensearch:3.0.0",
		ExposedPorts: []string{"9200/tcp"},
		Env: map[string]string{
			"discovery.type":           "single-node",
			"DISABLE_SECURITY_PLUGIN":  "true",
			"plugins.security.disabled": "true",
			"OPENSEARCH_JAVA_OPTS":     "-Xms256m -Xmx256m",
			"knn.plugin.enabled":       "true",
		},
		WaitingFor: wait.ForHTTP("/").WithPort("9200/tcp").WithStatusCodeMatcher(func(status int) bool {
			return status == 200
		}),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start opensearch container: %v", err)
	}
	t.Cleanup(func() {
		container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "9200")
	if err != nil {
		t.Fatalf("get container port: %v", err)
	}

	url := fmt.Sprintf("http://%s:%s", host, port.Port())

	engine, err := search.NewOpenSearchEngine(url)
	if err != nil {
		t.Fatalf("create opensearch engine: %v", err)
	}

	if err := engine.EnsureIndex(ctx); err != nil {
		t.Fatalf("ensure index: %v", err)
	}

	return &TestOpenSearch{Engine: engine, URL: url, Container: container}
}
