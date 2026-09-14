package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nicolas-limadev/navis-paas/pkg/api"
	"github.com/nicolas-limadev/navis-paas/pkg/k8s"
	"github.com/nicolas-limadev/navis-paas/pkg/offers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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

	// Locate static directory for UI
	cwd, _ := os.Getwd()
	staticDir := filepath.Join(cwd, "web", "static")
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Fallback check if running from another dir
		altStatic := filepath.Join(cwd, "..", "web", "static")
		if _, err := os.Stat(altStatic); err == nil {
			staticDir = altStatic
		}
	}

	router := api.SetupRouter(handler, staticDir)

	log.Printf("🚀 NavisPaaS Server listening on http://0.0.0.0:%s", port)
	log.Printf("🌐 Web Dashboard: http://localhost:%s/", port)
	log.Printf("📚 API Docs: http://localhost:%s/api/v1/openapi.json", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
