package main

// `iecli seed-eval` populates a dedicated evaluation environment
// (separate Postgres database, separate OpenSearch index) from JSON
// fixture files under eval/fixtures/. It's intentionally independent
// of `iecli seed`: the eval env is expected to grow in isolation from
// the hand-curated demo data used by the dev stack.
//
// This command talks directly to Postgres and OpenSearch — it does NOT
// go through the market-platform HTTP API. That decouples eval seeding
// from whichever version of the API the dev stack happens to be
// running, and lets us choose index shape / id scheme per eval needs
// (for example, using fixture external_ids as OpenSearch document ids
// so eval ground_truth is stable across re-seeds).

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/harrisonengel/birch-sky/src/market-platform/internal/storage/postgres"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

const (
	defaultEvalDBName    = "iemarket_eval"
	defaultEvalIndexName = "listings_eval"
)

type evalSeller struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
}

type evalListing struct {
	ExternalID       string   `json:"external_id"`
	SellerExternalID string   `json:"seller_external_id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	PriceCents       int      `json:"price_cents"`
	Tags             []string `json:"tags"`
}

type evalFixture struct {
	FixtureID string        `json:"fixture_id"`
	Sellers   []evalSeller  `json:"sellers"`
	Listings  []evalListing `json:"listings"`
}

func seedEvalCmd() *cobra.Command {
	var (
		fixtureName string
		all         bool
		reset       bool
		evalDB      string
		evalIndex   string
		osURL       string
		fixturesDir string
	)
	cmd := &cobra.Command{
		Use:   "seed-eval",
		Short: "Populate the eval-dedicated Postgres DB and OpenSearch index from eval/fixtures/",
		Long: `Seeds the evaluation environment (Postgres iemarket_eval and OpenSearch listings_eval by default)
from JSON fixture files. Pass --fixture NAME or --all. Use --reset to drop data before reseeding.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSeedEval(seedEvalOptions{
				fixtureName: fixtureName,
				all:         all,
				reset:       reset,
				evalDB:      evalDB,
				evalIndex:   evalIndex,
				osURL:       osURL,
				fixturesDir: fixturesDir,
			})
		},
	}
	cmd.Flags().StringVar(&fixtureName, "fixture", "", "name of a single fixture (without .json) to apply")
	cmd.Flags().BoolVar(&all, "all", false, "apply every fixture under the fixtures dir")
	cmd.Flags().BoolVar(&reset, "reset", false, "drop eval DB + OpenSearch index first")
	cmd.Flags().StringVar(&evalDB, "eval-db", defaultEvalDBName, "eval Postgres database name")
	cmd.Flags().StringVar(&evalIndex, "eval-index", defaultEvalIndexName, "eval OpenSearch index name")
	cmd.Flags().StringVar(&osURL, "opensearch", defaultOSURL(), "OpenSearch base URL")
	cmd.Flags().StringVar(&fixturesDir, "fixtures-dir", "", "directory containing fixture JSON files (default: <repo>/eval/fixtures)")
	return cmd
}

type seedEvalOptions struct {
	fixtureName string
	all         bool
	reset       bool
	evalDB      string
	evalIndex   string
	osURL       string
	fixturesDir string
}

func runSeedEval(opts seedEvalOptions) error {
	if opts.fixtureName == "" && !opts.all {
		return fmt.Errorf("must pass --fixture NAME or --all")
	}
	if opts.fixturesDir == "" {
		dir, err := defaultFixturesDir()
		if err != nil {
			return fmt.Errorf("resolve fixtures dir: %w", err)
		}
		opts.fixturesDir = dir
	}

	fixtures, err := loadFixtures(opts.fixturesDir, opts.fixtureName, opts.all)
	if err != nil {
		return err
	}

	if err := ensureEvalDatabase(opts.evalDB); err != nil {
		return fmt.Errorf("ensure eval database: %w", err)
	}

	evalDSN := buildEvalDSN(getDatabaseURL(), opts.evalDB)
	db, err := postgres.Connect(evalDSN)
	if err != nil {
		return fmt.Errorf("connect eval db: %w", err)
	}
	defer db.Close()

	if err := postgres.RunMigrations(db); err != nil {
		return fmt.Errorf("run eval migrations: %w", err)
	}

	osBase := strings.TrimRight(opts.osURL, "/")
	if opts.reset {
		if err := truncateEvalDB(db); err != nil {
			return fmt.Errorf("truncate eval db: %w", err)
		}
		if err := deleteIndex(osBase, opts.evalIndex); err != nil {
			return fmt.Errorf("delete eval index: %w", err)
		}
	}
	if err := ensureEvalIndex(osBase, opts.evalIndex); err != nil {
		return fmt.Errorf("ensure eval index: %w", err)
	}

	for _, f := range fixtures {
		fmt.Printf("Applying fixture %s (%d sellers, %d listings)\n",
			f.FixtureID, len(f.Sellers), len(f.Listings))
		if err := applyFixture(db, osBase, opts.evalIndex, f); err != nil {
			return fmt.Errorf("fixture %s: %w", f.FixtureID, err)
		}
	}

	fmt.Println("\nseed-eval complete.")
	return nil
}

func defaultFixturesDir() (string, error) {
	// Walk up from cwd looking for eval/fixtures — lets the command work
	// regardless of where under the repo the user invokes it.
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, "eval", "fixtures")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find eval/fixtures from %s", cwd)
		}
		dir = parent
	}
}

func loadFixtures(dir, single string, all bool) ([]evalFixture, error) {
	var files []string
	if all {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("read fixtures dir: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	} else {
		files = []string{filepath.Join(dir, single+".json")}
	}

	out := make([]evalFixture, 0, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var f evalFixture
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if f.FixtureID == "" {
			return nil, fmt.Errorf("%s: missing fixture_id", path)
		}
		out = append(out, f)
	}
	return out, nil
}

// ensureEvalDatabase connects to the admin database ("postgres") and
// creates the eval database if it doesn't exist yet.
func ensureEvalDatabase(name string) error {
	adminDSN := buildEvalDSN(getDatabaseURL(), "postgres")
	db, err := sqlx.Connect("postgres", adminDSN)
	if err != nil {
		return fmt.Errorf("connect admin db: %w", err)
	}
	defer db.Close()

	var exists bool
	if err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", name); err != nil {
		return err
	}
	if exists {
		return nil
	}
	// CREATE DATABASE can't be parameterized; `name` comes from a flag,
	// not user data, so accept the shape. Still, quote it defensively.
	if !validDBName(name) {
		return fmt.Errorf("invalid eval db name %q", name)
	}
	_, err = db.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, name))
	return err
}

func validDBName(n string) bool {
	if n == "" {
		return false
	}
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

// buildEvalDSN rewrites the path of the base DSN to target a specific
// database. We don't want to require the operator to maintain a second
// env var just for eval — they configured DATABASE_URL once and we
// point at a different database on the same server.
func buildEvalDSN(base, dbName string) string {
	u, err := url.Parse(base)
	if err != nil {
		// Fall back to string concatenation if the DSN is in keyword
		// form instead of URL form. This is a best-effort shim.
		return fmt.Sprintf("postgres://ieuser:iepass@localhost:5432/%s?sslmode=disable", dbName)
	}
	u.Path = "/" + dbName
	return u.String()
}

func truncateEvalDB(db *sqlx.DB) error {
	// Order matters due to FKs. Listings depend on sellers; ownership +
	// transactions + buy_orders depend on listings. Just wipe the
	// eval-relevant tables and leave the rest alone.
	_, err := db.Exec(`
		TRUNCATE ownership, transactions, buy_orders RESTART IDENTITY CASCADE;
		TRUNCATE listings RESTART IDENTITY CASCADE;
		TRUNCATE sellers RESTART IDENTITY CASCADE;
	`)
	return err
}

// ensureEvalIndex creates a minimal text-only mapping for the eval
// index. The dev `listings` index has a knn_vector field for semantic
// search; the Python harness's search_opensearch tool is text-only, so
// we deliberately skip the embedding pipeline here. If eval ever needs
// vector search, mirror the dev index mapping.
func ensureEvalIndex(osBase, index string) error {
	// HEAD /<index> → 200 if exists, 404 if not.
	req, err := http.NewRequest("HEAD", osBase+"/"+index, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == 200 {
		return nil
	}

	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
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
				"title":        map[string]interface{}{"type": "text", "analyzer": "listing_analyzer"},
				"description":  map[string]interface{}{"type": "text", "analyzer": "listing_analyzer"},
				"tags":         map[string]interface{}{"type": "text", "analyzer": "listing_analyzer"},
				"content_text": map[string]interface{}{"type": "text", "analyzer": "listing_analyzer"},
				"category":     map[string]interface{}{"type": "keyword"},
				"status":       map[string]interface{}{"type": "keyword"},
				"price_cents":  map[string]interface{}{"type": "integer"},
				"listing_id":   map[string]interface{}{"type": "keyword"},
				"seller_id":    map[string]interface{}{"type": "keyword"},
				"seller_name":  map[string]interface{}{"type": "keyword"},
			},
		},
	}
	body, _ := json.Marshal(mapping)
	req, err = http.NewRequest("PUT", osBase+"/"+index, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT %s -> %d: %s", index, resp.StatusCode, string(b))
	}
	return nil
}

func deleteIndex(osBase, index string) error {
	req, err := http.NewRequest("DELETE", osBase+"/"+index, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 || resp.StatusCode < 300 {
		return nil
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("DELETE %s -> %d: %s", index, resp.StatusCode, string(b))
}

func applyFixture(db *sqlx.DB, osBase, index string, f evalFixture) error {
	ctx := context.Background()

	// UPSERT sellers by email (the UNIQUE constraint in the schema).
	sellerUUIDs := make(map[string]string, len(f.Sellers))
	for _, s := range f.Sellers {
		var id string
		err := db.QueryRowContext(ctx, `
			INSERT INTO sellers (name, email) VALUES ($1, $2)
			ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		`, s.Name, s.Email).Scan(&id)
		if err != nil {
			return fmt.Errorf("seller %s: %w", s.ExternalID, err)
		}
		sellerUUIDs[s.ExternalID] = id
		fmt.Printf("  seller %-35s -> %s\n", s.ExternalID, id)
	}

	for _, l := range f.Listings {
		sellerUUID, ok := sellerUUIDs[l.SellerExternalID]
		if !ok {
			return fmt.Errorf("listing %s references unknown seller %s",
				l.ExternalID, l.SellerExternalID)
		}
		tagsJSON, _ := json.Marshal(l.Tags)

		// No unique constraint on listings — running seed-eval without
		// --reset will duplicate rows. That's accepted for an internal
		// dev tool; the help text tells users to reset for a clean env.
		var listingID string
		err := db.QueryRowContext(ctx, `
			INSERT INTO listings
				(seller_id, title, description, category, price_cents, currency, tags, status, data_ref)
			VALUES ($1, $2, $3, $4, $5, 'usd', $6::jsonb, 'active', $7)
			RETURNING id
		`, sellerUUID, l.Title, l.Description, l.Category, l.PriceCents, string(tagsJSON), l.ExternalID).Scan(&listingID)
		if err != nil {
			return fmt.Errorf("listing %s: %w", l.ExternalID, err)
		}

		// Index into OpenSearch with listing_id == external_id so eval
		// ground_truth can reference human-readable ids that survive
		// across re-seeds. content_text and tags give the full-text
		// search something substantial to chew on.
		sellerName := ""
		for _, s := range f.Sellers {
			if s.ExternalID == l.SellerExternalID {
				sellerName = s.Name
				break
			}
		}
		doc := map[string]interface{}{
			"listing_id":   l.ExternalID,
			"seller_id":    sellerUUID,
			"seller_name":  sellerName,
			"title":        l.Title,
			"description":  l.Description,
			"category":     l.Category,
			"price_cents":  l.PriceCents,
			"status":       "active",
			"tags":         strings.Join(l.Tags, " "),
			"content_text": l.Description, // no data upload for eval fixtures
		}
		if err := indexDoc(osBase, index, l.ExternalID, doc); err != nil {
			return fmt.Errorf("index %s: %w", l.ExternalID, err)
		}
		fmt.Printf("  listing %-35s -> db=%s  os_id=%s\n",
			l.ExternalID, listingID, l.ExternalID)
	}

	// Refresh index so searches see docs immediately (eval runs follow
	// seeds closely — don't rely on the default 1s refresh interval).
	if err := refreshIndex(osBase, index); err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	return nil
}

func indexDoc(osBase, index, id string, doc map[string]interface{}) error {
	body, _ := json.Marshal(doc)
	req, err := http.NewRequest("PUT",
		fmt.Sprintf("%s/%s/_doc/%s", osBase, index, url.PathEscape(id)),
		strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT doc %s -> %d: %s", id, resp.StatusCode, string(b))
	}
	return nil
}

func refreshIndex(osBase, index string) error {
	resp, err := http.Post(osBase+"/"+index+"/_refresh", "application/json", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
