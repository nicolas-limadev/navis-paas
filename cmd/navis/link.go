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
	linkOfferID   string
	linkParams    map[string]string
)

func init() {
	linkCmd.Flags().StringVarP(&linkOfferID, "offer", "o", "", "Offer ID to link (e.g., postgresql, redis, kafka)")
	linkCmd.Flags().StringToStringVarP(&linkParams, "param", "p", nil, "Custom parameters (e.g., --param username=admin,password=secret)")
	linkCmd.MarkFlagRequired("offer")
}

var linkCmd = &cobra.Command{
	Use:   "link [app-name]",
	Short: "Link an offer to an application",
	Long:  `Link an infrastructure offer (PostgreSQL, Redis, Kafka, etc.) to your application.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]

		payload := map[string]interface{}{
			"offerId":    linkOfferID,
			"parameters": linkParams,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(
			serverURL+fmt.Sprintf("/api/v1/apps/%s/links", appName),
			"application/json",
			bytes.NewBuffer(jsonData),
		)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to link offer: %s", string(body))
		}

		var result map[string]interface{}
		json.Unmarshal(body, &result)

		fmt.Printf("✅ Offer '%s' linked to '%s'\n", linkOfferID, appName)

		if injectedConfig, ok := result["injectedConfig"].(map[string]interface{}); ok {
			fmt.Println("\n📋 Injected environment variables:")
			for key, value := range injectedConfig {
				fmt.Printf("   %s=%v\n", key, value)
			}
		}

		return nil
	},
}
