package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var unlinkCmd = &cobra.Command{
	Use:   "unlink [app-name] [offer-id]",
	Short: "Unlink an offer from an application",
	Long:  `Remove an infrastructure offer binding from your application.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]
		offerID := args[1]

		req, _ := http.NewRequest(
			http.MethodDelete,
			serverURL+fmt.Sprintf("/api/v1/apps/%s/links/%s", appName, offerID),
			nil,
		)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to unlink offer: %s", string(body))
		}

		fmt.Printf("✅ Offer '%s' unlinked from '%s'\n", offerID, appName)
		return nil
	},
}
