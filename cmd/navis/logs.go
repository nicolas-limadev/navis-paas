package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var (
	logLines int
)

func init() {
	logsCmd.Flags().IntVarP(&logLines, "lines", "n", 100, "Number of log lines to fetch")
}

var logsCmd = &cobra.Command{
	Use:   "logs [app-name]",
	Short: "Fetch application logs",
	Long:  `Fetch recent logs from the application pods.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]

		url := fmt.Sprintf("%s/api/v1/apps/%s/logs?lines=%d", serverURL, appName, logLines)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to fetch logs: %s", string(body))
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if logs, ok := result["logs"].(string); ok {
			fmt.Println(logs)
		} else {
			fmt.Println("No logs available.")
		}

		return nil
	},
}
