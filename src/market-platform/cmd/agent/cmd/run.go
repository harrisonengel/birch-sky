package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var (
	configPath string
	userInput  string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a buyer agent against the marketplace",
	Long:  "Execute the Python agent harness with the given config and user input.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}

		proc := exec.Command(
			"python", "-m", "harness",
			"-c", configPath,
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
	runCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to YAML config file (required)")
	runCmd.Flags().StringVarP(&userInput, "input", "i", "", "User input / instruction for the agent (required)")
	runCmd.MarkFlagRequired("config")
	runCmd.MarkFlagRequired("input")
	rootCmd.AddCommand(runCmd)
}
