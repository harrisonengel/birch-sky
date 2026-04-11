package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	configPath  string
	sessionPath string
	userInput   string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a buyer agent against the marketplace",
	Long:  "Execute the Python agent harness with the given config, session, and user input.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}
		if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
			return fmt.Errorf("session file not found: %s", sessionPath)
		}

		proc := exec.Command(
			"python", "-m", "harness",
			"-c", configPath,
			"-s", sessionPath,
			"-i", userInput,
		)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr

		if err := proc.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run agent harness: %w", err)
		}
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to infrastructure YAML config (required)")
	runCmd.Flags().StringVarP(&sessionPath, "session", "s", "", "Path to session YAML with starting_context (required)")
	runCmd.Flags().StringVarP(&userInput, "input", "i", "", "User input / instruction for the agent (required)")
	runCmd.MarkFlagRequired("config")
	runCmd.MarkFlagRequired("session")
	runCmd.MarkFlagRequired("input")
	rootCmd.AddCommand(runCmd)
}
