package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var (
	installPrometheus bool
	installGrafana    bool
	installOTEL       bool
)

func init() {
	installCmd.Flags().BoolVar(&installPrometheus, "prometheus", true, "Install Prometheus")
	installCmd.Flags().BoolVar(&installGrafana, "grafana", true, "Install Grafana")
	installCmd.Flags().BoolVar(&installOTEL, "otel", false, "Install OpenTelemetry Collector")
}

var installCmd = &cobra.Command{
	Use:   "install [offer-id]",
	Short: "Install offer components to the cluster",
	Long:  `Install infrastructure offer components (Kafka, PostgreSQL, Monitoring, etc.).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		offerID := args[0]

		fmt.Printf("📦 Installing %s...\n", offerID)

		var payload map[string]interface{}

		if offerID == "monitoring" {
			payload = map[string]interface{}{
				"prometheus": installPrometheus,
				"grafana":    installGrafana,
				"otel":       installOTEL,
			}
		} else {
			payload = map[string]interface{}{}
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(
			serverURL+fmt.Sprintf("/api/v1/offers/%s/install", offerID),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to install offer: %s", string(body))
		}

		var result map[string]interface{}
		json.Unmarshal(body, &result)

		if msg, ok := result["message"].(string); ok {
			fmt.Printf("✅ %s\n", msg)
		}

		return nil
	},
}
