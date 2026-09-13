package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize NavisPaaS configuration",
	Long:  `Create a .navis.yml configuration file in the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config := `# NavisPaaS Configuration
# Documentation: https://github.com/nicolas-limadev/navis-paas

# Application settings
app:
  name: my-app
  image: my-image:latest
  port: 8080
  replicas: 1

# Infrastructure offers to link
offers:
  - name: postgresql
    enabled: true
    params:
      username: postgres
      password: postgres
      databaseName: mydb
  - name: redis
    enabled: false
  - name: kafka
    enabled: false
    params:
      topic: my-topic
      consumerGroup: my-group
  - name: monitoring
    enabled: false
    components:
      prometheus: true
      grafana: true
      otel: false
  - name: rabbitmq
    enabled: false
  - name: kong
    enabled: false

# Server connection (optional)
server:
  url: http://localhost:8080
`

		if _, err := os.Stat(".navis.yml"); err == nil {
			return fmt.Errorf(".navis.yml already exists. Use 'navis deploy' to deploy your app")
		}

		if err := os.WriteFile(".navis.yml", []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}

		fmt.Println("✅ Created .navis.yml configuration file")
		fmt.Println("📝 Edit the file with your app details, then run: navis deploy")
		return nil
	},
}
