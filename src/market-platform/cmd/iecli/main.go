// Command iecli is the operator CLI for the Information Exchange
// marketplace. It provides subcommands to seed demo data and run
// sample SQL queries against the local Postgres database.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var databaseURL string

var rootCmd = &cobra.Command{
	Use:   "iecli",
	Short: "Information Exchange marketplace CLI",
	Long:  "Operator CLI for seeding data, running sample queries, and exercising the IE marketplace.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&databaseURL, "database-url", "", "Postgres connection string (default: $DATABASE_URL or local docker)")
}

func getDatabaseURL() string {
	if databaseURL != "" {
		return databaseURL
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://ieuser:iepass@localhost:5432/iemarket?sslmode=disable"
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
