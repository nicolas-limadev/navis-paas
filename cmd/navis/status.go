package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster and offer status",
	Long:  `Display the current cluster status and installed offers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get health status
		healthResp, err := http.Get(serverURL + "/api/v1/health")
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer healthResp.Body.Close()

		healthBody, _ := io.ReadAll(healthResp.Body)
		var health map[string]interface{}
		json.Unmarshal(healthBody, &health)

		fmt.Println("🖥️  Cluster Status")
		fmt.Println("─────────────────")
		if connected, ok := health["connected"].(bool); ok && connected {
			fmt.Printf("   Status: ✅ Connected\n")
			fmt.Printf("   Provider: %v\n", health["provider"])
			fmt.Printf("   Version: %v\n", health["clusterVersion"])
			fmt.Printf("   Context: %v\n", health["context"])
		} else {
			fmt.Printf("   Status: ❌ Disconnected\n")
			if errMsg, ok := health["errorMessage"].(string); ok && errMsg != "" {
				fmt.Printf("   Error: %s\n", errMsg)
			}
		}

		// Get offers status
		offersResp, err := http.Get(serverURL + "/api/v1/offers")
		if err != nil {
			return nil // Non-critical, just skip offers
		}
		defer offersResp.Body.Close()

		offersBody, _ := io.ReadAll(offersResp.Body)
		var offers []map[string]interface{}
		json.Unmarshal(offersBody, &offers)

		fmt.Println("\n📦 Installed Offers")
		fmt.Println("──────────────────")
		for _, offer := range offers {
			name := offer["name"].(string)
			installed := offer["installed"].(bool)
			status := "❌"
			if installed {
				status = "✅"
			}
			fmt.Printf("   %s %s\n", status, name)
		}

		return nil
	},
}
