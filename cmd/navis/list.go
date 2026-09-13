package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List deployed applications",
	Long:  `List all applications deployed on the Kubernetes cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := http.Get(serverURL + "/api/v1/apps")
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to list apps: %s", string(body))
		}

		var apps []map[string]interface{}
		if err := json.Unmarshal(body, &apps); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(apps) == 0 {
			fmt.Println("No applications found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tREPLICAS\tOFFERS\tACCESS")
		fmt.Fprintln(w, "----\t------\t--------\t------\t------")

		for _, app := range apps {
			name := app["name"].(string)
			status := app["status"].(string)
			replicas := fmt.Sprintf("%v/%v", app["readyCount"], app["replicas"])
			offers := ""
			if linkedOffers, ok := app["linkedOffers"].([]interface{}); ok && len(linkedOffers) > 0 {
				offers = fmt.Sprintf("%d linked", len(linkedOffers))
			} else {
				offers = "none"
			}
			access := ""
			if url, ok := app["accessURL"].(string); ok && url != "" {
				access = url
			} else if nodePort, ok := app["nodePort"].(float64); ok && nodePort > 0 {
				access = fmt.Sprintf(":%.0f", nodePort)
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", name, status, replicas, offers, access)
		}

		w.Flush()
		return nil
	},
}
