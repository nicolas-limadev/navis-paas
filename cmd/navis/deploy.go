package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App struct {
		Name     string `yaml:"name" json:"name"`
		Image    string `yaml:"image" json:"image"`
		Port     int    `yaml:"port" json:"port"`
		Replicas int    `yaml:"replicas" json:"replicas"`
	} `yaml:"app" json:"app"`
	Offers []struct {
		Name       string            `yaml:"name" json:"name"`
		Enabled    bool              `yaml:"enabled" json:"enabled"`
		Params     map[string]string `yaml:"params" json:"params,omitempty"`
		Components map[string]bool   `yaml:"components" json:"components,omitempty"`
	} `yaml:"offers" json:"offers"`
	Server struct {
		URL string `yaml:"url" json:"url"`
	} `yaml:"server" json:"server"`
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy application to Kubernetes",
	Long:  `Deploy your application based on .navis.yml configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read config file
		configData, err := os.ReadFile(".navis.yml")
		if err != nil {
			return fmt.Errorf("no .navis.yml found. Run 'navis init' first")
		}

		var config Config
		if err := yaml.Unmarshal(configData, &config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}

		// Use server URL from config or flag
		server := serverURL
		if config.Server.URL != "" {
			server = config.Server.URL
		}

		fmt.Printf("🚀 Deploying %s to %s...\n", config.App.Name, server)

		// Create app
		appPayload := map[string]interface{}{
			"name":     config.App.Name,
			"image":    config.App.Image,
			"port":     config.App.Port,
			"replicas": config.App.Replicas,
			"offers":   []string{},
		}

		// Collect enabled offers
		var enabledOffers []string
		for _, offer := range config.Offers {
			if offer.Enabled {
				enabledOffers = append(enabledOffers, offer.Name)
			}
		}
		appPayload["offers"] = enabledOffers

		jsonData, _ := json.Marshal(appPayload)
		resp, err := http.Post(server+"/api/v1/apps", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("deploy failed: %s", string(body))
		}

		fmt.Printf("✅ Application '%s' deployed successfully!\n", config.App.Name)

		// Link offers with custom params
		for _, offer := range config.Offers {
			if offer.Enabled && offer.Params != nil {
				fmt.Printf("🔗 Linking %s with custom parameters...\n", offer.Name)
				linkPayload := map[string]interface{}{
					"offerId":    offer.Name,
					"parameters": offer.Params,
				}
				linkData, _ := json.Marshal(linkPayload)
				linkResp, err := http.Post(
					server+fmt.Sprintf("/api/v1/apps/%s/links", config.App.Name),
					"application/json",
					bytes.NewBuffer(linkData),
				)
				if err == nil {
					linkResp.Body.Close()
				}
			}
		}

		fmt.Println("\n📊 Deployment status:")
		fmt.Printf("   App: %s\n", config.App.Name)
		fmt.Printf("   Image: %s\n", config.App.Image)
		fmt.Printf("   Port: %d\n", config.App.Port)
		if len(enabledOffers) > 0 {
			fmt.Printf("   Offers: %v\n", enabledOffers)
		}

		return nil
	},
}
