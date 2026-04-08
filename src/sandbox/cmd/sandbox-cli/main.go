// Command sandbox-cli is the operator-facing CLI for the Models
// Sandbox MVP. It exists for two reasons:
//
//  1. The project rule that every API ships with a CLI that lets a
//     human or Claude exercise it end to end.
//  2. To give the demo a one-command "happy path" so reviewers can
//     watch the sandbox accept a brief and emit a verdict without
//     touching curl.
//
// All commands talk to sandbox-server over HTTP — no in-process
// shortcuts.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagBaseURL string
	flagAPIKey  string
)

func main() {
	root := &cobra.Command{
		Use:   "sandbox-cli",
		Short: "Operator CLI for the Models Sandbox MVP",
		Long: `sandbox-cli drives the Models Sandbox HTTP API end to end.

It is the canonical way to populate demo data and exercise the
sandbox without touching curl. Every command talks to sandbox-server
over HTTP — there are no in-process shortcuts.`,
	}

	root.PersistentFlags().StringVar(&flagBaseURL, "base-url", envOr("SANDBOX_BASE_URL", "http://localhost:8090"), "sandbox-server base URL")
	root.PersistentFlags().StringVar(&flagAPIKey, "api-key", os.Getenv("SANDBOX_API_KEY"), "API key (X-API-Key header)")

	root.AddCommand(objectivesCmd())
	root.AddCommand(submitCmd())
	root.AddCommand(getCmd())
	root.AddCommand(waitCmd())
	root.AddCommand(auditCmd())
	root.AddCommand(demoCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func objectivesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "objectives",
		Short: "List registered objective templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			body, err := apiGet("/v1/objectives")
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
}

func submitCmd() *cobra.Command {
	var (
		objective string
		subject   string
		buyerID   string
		budget    int
	)
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a brief to the sandbox",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if objective == "" || subject == "" || buyerID == "" {
				return fmt.Errorf("--objective, --subject, and --buyer-id are required")
			}
			var subj map[string]any
			if err := json.Unmarshal([]byte(subject), &subj); err != nil {
				return fmt.Errorf("--subject must be a JSON object: %w", err)
			}
			payload := map[string]any{
				"objective": objective,
				"subject":   subj,
				"buyer_id":  buyerID,
			}
			if budget > 0 {
				payload["budget_cents"] = budget
			}
			body, err := apiPost("/v1/briefs", payload)
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
	cmd.Flags().StringVar(&objective, "objective", "", "objective template ID")
	cmd.Flags().StringVar(&subject, "subject", "", "subject as JSON object")
	cmd.Flags().StringVar(&buyerID, "buyer-id", "", "opaque buyer ID")
	cmd.Flags().IntVar(&budget, "budget", 0, "max cents the harness may spend (0 = template default)")
	return cmd
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [job-id]",
		Short: "Fetch a job by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/v1/jobs/" + args[0])
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
}

func waitCmd() *cobra.Command {
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "wait [job-id]",
		Short: "Poll a job until it reaches a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			job, err := pollJob(args[0], timeout)
			if err != nil {
				return err
			}
			return printJSONValue(job)
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "max time to wait")
	return cmd
}

func auditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit [job-id]",
		Short: "Show the audit trail for one job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet("/v1/audit/" + args[0])
			if err != nil {
				return err
			}
			return printJSON(body)
		},
	}
}

func demoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "demo",
		Short: "Run the canned hello-world brief end-to-end",
		Long: `demo submits a healthcare provider verification brief for the
canned subject "hello-001", waits for the verdict, and prints both the
verdict and the per-job audit trail. It is the simplest possible
proof that every layer of the sandbox is wired correctly.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			payload := map[string]any{
				"objective": "healthcare.provider.verification",
				"subject":   map[string]string{"provider_id": "hello-001"},
				"buyer_id":  "demo-buyer",
			}
			body, err := apiPost("/v1/briefs", payload)
			if err != nil {
				return err
			}
			var sub struct {
				JobID string `json:"job_id"`
				State string `json:"state"`
			}
			if err := json.Unmarshal(body, &sub); err != nil {
				return err
			}
			fmt.Println("submitted job:", sub.JobID)

			job, err := pollJob(sub.JobID, 30*time.Second)
			if err != nil {
				return err
			}
			fmt.Println("verdict:")
			if err := printJSONValue(job); err != nil {
				return err
			}

			auditBody, err := apiGet("/v1/audit/" + sub.JobID)
			if err != nil {
				return err
			}
			fmt.Println("audit trail:")
			return printJSON(auditBody)
		},
	}
}

// pollJob polls /v1/jobs/{id} until the job reaches a terminal state
// or the timeout expires.
func pollJob(id string, timeout time.Duration) (map[string]any, error) {
	deadline := time.Now().Add(timeout)
	for {
		body, err := apiGet("/v1/jobs/" + id)
		if err != nil {
			return nil, err
		}
		var job map[string]any
		if err := json.Unmarshal(body, &job); err != nil {
			return nil, err
		}
		state, _ := job["state"].(string)
		switch state {
		case "COMPLETED", "FAILED", "TIMEOUT":
			return job, nil
		}
		if time.Now().After(deadline) {
			return job, fmt.Errorf("timed out after %s waiting for job %s (last state %q)", timeout, id, state)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func apiGet(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, flagBaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return doRequest(req)
}

func apiPost(path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, flagBaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

func doRequest(req *http.Request) ([]byte, error) {
	if flagAPIKey != "" {
		req.Header.Set("X-API-Key", flagAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func printJSON(raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// Not JSON — just print as-is.
		fmt.Println(string(raw))
		return nil
	}
	return printJSONValue(v)
}

func printJSONValue(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
