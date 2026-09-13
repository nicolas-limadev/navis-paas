package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "1.0.0"
	serverURL string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "navis",
		Short: "NavisPaaS CLI - Developer-Centric Kubernetes PaaS",
		Long: `NavisPaaS CLI allows you to deploy and manage microservices
on Kubernetes with pre-configured infrastructure offers (Kafka, RabbitMQ,
PostgreSQL, Redis, Monitoring, Kong API Gateway).`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Server URL can be set via flag or environment variable
			if serverURL == "" {
				serverURL = os.Getenv("NAVIS_SERVER")
			}
			if serverURL == "" {
				serverURL = "http://localhost:8080"
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "NavisPaaS server URL (default: http://localhost:8080)")

	// Add commands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(unlinkCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
