// Command prepper-cli exercises the agent-prepper microservice end to end.
//
// Per CLAUDE.md, every API ships with a CLI. The prepper exposes a small
// HTTP surface (start / respond / session) that this CLI wraps so a human
// or another script can drive a clarification conversation from the
// terminal and inspect the resulting Briefing.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var prepperURL string

func main() {
	root := &cobra.Command{
		Use:   "prepper-cli",
		Short: "Drive the IE agent-prepper clarification service",
	}
	defaultURL := os.Getenv("PREPPER_URL")
	if defaultURL == "" {
		defaultURL = "http://localhost:8002"
	}
	root.PersistentFlags().StringVar(&prepperURL, "prepper-url", defaultURL,
		"agent-prepper base URL (env PREPPER_URL)")

	root.AddCommand(startCmd())
	root.AddCommand(respondCmd())
	root.AddCommand(sessionCmd())
	root.AddCommand(chatCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// turnResponse mirrors agent_prepper.api.TurnResponse.
type turnResponse struct {
	SessionID string                 `json:"session_id"`
	Status    string                 `json:"status"`
	Turn      int                    `json:"turn"`
	Question  string                 `json:"question,omitempty"`
	Briefing  map[string]interface{} `json:"briefing,omitempty"`
}

func startCmd() *cobra.Command {
	var query, buyerID string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Create a new prepper session and run the first turn",
		RunE: func(_ *cobra.Command, _ []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			if buyerID == "" {
				buyerID = "cli-buyer"
			}
			resp, err := postStart(buyerID, query)
			if err != nil {
				return err
			}
			printTurn(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "initial buyer query (required)")
	cmd.Flags().StringVar(&buyerID, "buyer-id", "", "buyer id (default: cli-buyer)")
	return cmd
}

func respondCmd() *cobra.Command {
	var sessionID, answer string
	cmd := &cobra.Command{
		Use:   "respond",
		Short: "Submit an answer to a clarifying question",
		RunE: func(_ *cobra.Command, _ []string) error {
			if sessionID == "" || answer == "" {
				return fmt.Errorf("--session-id and --answer are required")
			}
			resp, err := postRespond(sessionID, answer)
			if err != nil {
				return err
			}
			printTurn(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session-id", "", "prepper session id")
	cmd.Flags().StringVar(&answer, "answer", "", "answer to the latest clarifying question")
	return cmd
}

func sessionCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Dump full session state as JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			if sessionID == "" {
				return fmt.Errorf("--session-id is required")
			}
			data, err := getJSON(prepperURL + "/api/prepper/session/" + sessionID)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session-id", "", "prepper session id")
	return cmd
}

// chat runs an interactive stdin loop until the prepper finalizes a Briefing.
// This is the most useful command for end-to-end demos: one shot, walk
// through a conversation, see the final Briefing.
func chatCmd() *cobra.Command {
	var query, buyerID string
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interactive clarification loop until a Briefing is produced",
		RunE: func(_ *cobra.Command, _ []string) error {
			if query == "" {
				return fmt.Errorf("--query is required")
			}
			if buyerID == "" {
				buyerID = "cli-buyer"
			}
			turn, err := postStart(buyerID, query)
			if err != nil {
				return err
			}
			reader := bufio.NewReader(os.Stdin)
			for turn.Status == "asking" {
				fmt.Printf("\n[prepper turn %d] %s\n> ", turn.Turn, turn.Question)
				line, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				answer := strings.TrimSpace(line)
				if answer == "" {
					fmt.Println("(empty answer; please type something or Ctrl-C to abort)")
					continue
				}
				turn, err = postRespond(turn.SessionID, answer)
				if err != nil {
					return err
				}
			}
			fmt.Printf("\n[prepper] finalized after %d turns. Briefing:\n", turn.Turn)
			pretty, _ := json.MarshalIndent(turn.Briefing, "", "  ")
			fmt.Println(string(pretty))
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "initial buyer query (required)")
	cmd.Flags().StringVar(&buyerID, "buyer-id", "", "buyer id (default: cli-buyer)")
	return cmd
}

func postStart(buyerID, query string) (*turnResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"buyer_id":      buyerID,
		"initial_query": query,
	})
	return doPost(prepperURL+"/api/prepper/start", body)
}

func postRespond(sessionID, answer string) (*turnResponse, error) {
	body, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"answer":     answer,
	})
	return doPost(prepperURL+"/api/prepper/respond", body)
}

func doPost(url string, body []byte) (*turnResponse, error) {
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST %s -> %d: %s", url, resp.StatusCode, string(data))
	}
	var out turnResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w (body=%s)", err, string(data))
	}
	return &out, nil
}

func getJSON(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s -> %d: %s", url, resp.StatusCode, string(data))
	}
	// Pretty-print
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return data, nil
	}
	return pretty.Bytes(), nil
}

func printTurn(r *turnResponse) {
	fmt.Printf("session_id: %s\n", r.SessionID)
	fmt.Printf("status:     %s\n", r.Status)
	fmt.Printf("turn:       %d\n", r.Turn)
	if r.Question != "" {
		fmt.Printf("question:   %s\n", r.Question)
	}
	if r.Briefing != nil {
		fmt.Println("briefing:")
		pretty, _ := json.MarshalIndent(r.Briefing, "  ", "  ")
		fmt.Printf("  %s\n", string(pretty))
	}
}
