package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [app-name]",
	Short: "Delete an application",
	Long:  `Delete an application and its namespace from the cluster.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appName := args[0]

		// Confirm deletion
		fmt.Printf("⚠️  Are you sure you want to delete '%s'? (y/N): ", appName)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Deletion cancelled.")
			return nil
		}

		req, _ := http.NewRequest(
			http.MethodDelete,
			serverURL+fmt.Sprintf("/api/v1/apps/%s", appName),
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
			return fmt.Errorf("failed to delete app: %s", string(body))
		}

		fmt.Printf("✅ Application '%s' deleted successfully\n", appName)
		return nil
	},
}
