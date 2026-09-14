package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nicolas-limadev/navis-paas/pkg/api"
	"github.com/nicolas-limadev/navis-paas/pkg/k8s"
	"github.com/nicolas-limadev/navis-paas/pkg/offers"

	"github.com/spf13/cobra"
)

var (
	startPort string
)

func init() {
	startCmd.Flags().StringVarP(&startPort, "port", "p", "8080", "Port to run the NavisPaaS server on")
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the NavisPaaS server and web dashboard",
	Long: `Starts the backend REST API engine and serves the responsive
native web dashboard, allowing developers to manage workloads via both web and CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("===============================================")
		fmt.Println("   _  _            _      ___             ___ ")
		fmt.Println("  | \\| |__ ___ ___(_)___ | _ \\__ _ __ _  / __|")
		fmt.Println("  | .  / _' \\ V / | (_-< |  _/ _' / _' | \\__ \\")
		fmt.Println("  |_|\\_\\__,_|\\_/|_|_/__/ |_| \\__,_\\__,_| |___/")
		fmt.Println("===============================================")
		fmt.Println("     Developer-Centric Kubernetes PaaS Engine  ")
		fmt.Println("===============================================")

		// Initialize Kubernetes Client Manager
		cm := k8s.GetClientManager()
		ctx := context.Background()
		connected, k8sVersion, err := cm.CheckConnectivity(ctx)
		if connected {
			log.Printf(" Connected to Kubernetes cluster (version: %s, context: %s)", k8sVersion, cm.ContextName)
		} else {
			log.Printf("⚠️ Kubernetes cluster not yet reachable: %v", err)
			log.Printf("ℹ️  You can start minikube anytime via 'minikube start'. NavisPaaS will reconnect automatically.")
		}

		// Initialize Core Services
		registryService := k8s.NewRegistryService(cm)
		appService := k8s.NewAppService(cm, registryService)
		catalog := offers.NewCatalog(cm)

		// Create API Adapters & Handlers
		appAdapter := api.NewAppAdapter(appService)
		offerAdapter := api.NewOfferAdapter(catalog, cm)
		clusterAdapter := api.NewClusterAdapter(cm)
		registryAdapter := api.NewRegistryAdapter(registryService)
		handler := api.NewHandler(appAdapter, offerAdapter, clusterAdapter, registryAdapter)

		// Locate static directory for UI (packaged in relative folder or gopath)
		cwd, _ := os.Getwd()
		staticDir := filepath.Join(cwd, "web", "static")
		if _, err := os.Stat(staticDir); os.IsNotExist(err) {
			// Try to find relative to executable, GOPATH or user home for global binary
			exePath, err := os.Executable()
			if err == nil {
				staticDir = filepath.Join(filepath.Dir(exePath), "web", "static")
			}
		}

		if _, err := os.Stat(staticDir); os.IsNotExist(err) {
			log.Printf("⚠️ Warning: Static web directory not found at '%s'. Web dashboard UI might be unavailable.", staticDir)
		} else {
			log.Printf("📂 Serving web dashboard static assets from: %s", staticDir)
		}

		router := api.SetupRouter(handler, staticDir)

		log.Printf("🚀 NavisPaaS Server listening on http://0.0.0.0:%s", startPort)
		log.Printf("🌐 Web Dashboard: http://localhost:%s/", startPort)
		log.Printf("📚 OpenAPI Docs / Interactive Swagger: http://localhost:%s/api/v1/docs", startPort)

		if err := http.ListenAndServe(":" + startPort, router); err != nil {
			log.Fatalf("Server failed to run: %v", err)
		}
	},
}
