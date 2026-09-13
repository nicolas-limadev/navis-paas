package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// SetupRouter configures the Gin engine with all routes and static file handlers
func SetupRouter(h *Handler, staticDir string) *gin.Engine {
	r := gin.Default()

	// Enable CORS for Backstage or external UI frontends
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Serve Static UI Dashboard if available
	if staticDir != "" {
		if _, err := os.Stat(staticDir); err == nil {
			r.Static("/static", staticDir)
			r.GET("/", func(c *gin.Context) {
				c.File(filepath.Join(staticDir, "index.html"))
			})
		}
	}

	// API Routes Group
	v1 := r.Group("/api/v1")
	{
		// Cluster Health & Info
		v1.GET("/health", h.GetHealth)

		// Applications Management
		v1.GET("/apps", h.ListApps)
		v1.POST("/apps", h.CreateApp)
		v1.GET("/apps/:name", h.GetAppDetails)
		v1.DELETE("/apps/:name", h.DeleteApp)
		v1.GET("/apps/:name/logs", h.GetAppLogs)

		// Offers Catalog
		v1.GET("/offers", h.ListOffers)
		v1.POST("/offers/:id/install", h.InstallOffer)

		// Service Binding
		v1.POST("/apps/:name/links", h.LinkOffer)
		v1.DELETE("/apps/:name/links/:offerId", h.UnlinkOffer)

		// Registry Management
		v1.GET("/registry", h.GetRegistry)
		v1.POST("/registry", h.UpdateRegistry)

		// OpenAPI Spec endpoint for Backstage / Swagger
		v1.GET("/openapi.json", func(c *gin.Context) {
			c.JSON(http.StatusOK, getOpenAPISpec())
		})
	}

	return r
}

func getOpenAPISpec() gin.H {
	return gin.H{
		"openapi": "3.0.0",
		"info": gin.H{
			"title":       "NavisPaaS Core Engine API",
			"version":     "1.0.0",
			"description": "Developer-Centric Kubernetes PaaS for Deployments, Messaging (Kafka), and Observability.",
		},
		"paths": gin.H{
			"/api/v1/health": gin.H{
				"get": gin.H{"summary": "Check Kubernetes cluster connectivity and status"},
			},
			"/api/v1/apps": gin.H{
				"get":  gin.H{"summary": "List deployed applications"},
				"post": gin.H{"summary": "Deploy a new application with optional offer bindings"},
			},
			"/api/v1/apps/{name}": gin.H{
				"get":    gin.H{"summary": "Get application details and pod status"},
				"delete": gin.H{"summary": "Delete an application"},
			},
			"/api/v1/apps/{name}/logs": gin.H{
				"get": gin.H{"summary": "Get recent pod logs for the application"},
			},
			"/api/v1/offers": gin.H{
				"get": gin.H{"summary": "List available infrastructure offers (Kafka, Monitoring, etc.)"},
			},
			"/api/v1/offers/{id}/install": gin.H{
				"post": gin.H{"summary": "Install offer cluster components"},
			},
			"/api/v1/apps/{name}/links": gin.H{
				"post": gin.H{"summary": "Link an offer to an application and inject configurations"},
			},
			"/api/v1/apps/{name}/links/{offerId}": gin.H{
				"delete": gin.H{"summary": "Unlink an offer from an application"},
			},
			"/api/v1/registry": gin.H{
				"get":  gin.H{"summary": "Get private Docker registry configuration status"},
				"post": gin.H{"summary": "Update private Docker registry credentials and sync secrets"},
			},
		},
	}
}
