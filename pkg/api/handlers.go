package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nicolas-limadev/navis-paas/pkg/offers"
)

// AppServiceProvider abstracts K8s app service
type AppServiceProvider interface {
	CreateOrUpdateApp(ctx *gin.Context, req CreateAppRequest) (*Application, error)
	ListApps(ctx *gin.Context, namespace string) ([]Application, error)
	GetApp(ctx *gin.Context, namespace, name string) (*Application, error)
	GetAppDetails(ctx *gin.Context, namespace, name string) (*AppDetailResponse, error)
	DeleteApp(ctx *gin.Context, namespace, name string) error
	GetAppLogs(ctx *gin.Context, namespace, name string, tailLines int64) (string, error)
}

// OfferServiceProvider abstracts the catalog & binding engine
type OfferServiceProvider interface {
	ListOffers(ctx *gin.Context) []OfferDefinition
	InstallOffer(ctx *gin.Context, offerID string, prometheus, grafana, otel bool) error
	GetOfferStatus(ctx *gin.Context, offerID string) (map[string]bool, error)
	BindOffer(ctx *gin.Context, app *Application, offerID string, params map[string]string) (map[string]string, error)
	UnbindOffer(ctx *gin.Context, app *Application, offerID string) error
	CreateCustomOffer(ctx *gin.Context, def offers.DynamicOfferDefinition) error
	DeleteCustomOffer(ctx *gin.Context, id string) error
}

// ClusterChecker checks K8s cluster status
type ClusterChecker interface {
	CheckStatus(ctx *gin.Context) ClusterStatus
	GetClusterContexts(ctx *gin.Context) ([]string, string, error)
	SwitchClusterContext(ctx *gin.Context, contextName string) error
	SetClusterKubeconfig(ctx *gin.Context, rawKubeconfig []byte, contextName string) error
}

// RegistryProvider manages private container registries
type RegistryProvider interface {
	GetRegistryStatus(ctx *gin.Context) RegistryStatusResponse
	SaveRegistryConfig(ctx *gin.Context, cfg RegistryConfig) error
}

type Handler struct {
	apps     AppServiceProvider
	offers   OfferServiceProvider
	cluster  ClusterChecker
	registry RegistryProvider
}

func NewHandler(apps AppServiceProvider, offers OfferServiceProvider, cluster ClusterChecker, registry RegistryProvider) *Handler {
	return &Handler{
		apps:     apps,
		offers:   offers,
		cluster:  cluster,
		registry: registry,
	}
}

// GetHealth returns system and kubernetes connectivity health
func (h *Handler) GetHealth(c *gin.Context) {
	status := h.cluster.CheckStatus(c)
	c.JSON(http.StatusOK, status)
}

// ListApps returns all managed applications
func (h *Handler) ListApps(c *gin.Context) {
	namespace := c.Query("namespace")
	apps, err := h.apps.ListApps(c, namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if apps == nil {
		apps = []Application{}
	}
	c.JSON(http.StatusOK, apps)
}

// CreateApp deploys a new application and links optional offers
func (h *Handler) CreateApp(c *gin.Context) {
	var req CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	app, err := h.apps.CreateOrUpdateApp(c, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "deployment failed: " + err.Error()})
		return
	}

	// Auto-bind any requested offers
	for _, offerID := range req.Offers {
		_, bindErr := h.offers.BindOffer(c, app, offerID, nil)
		if bindErr == nil {
			// Refresh app data after binding
			if updatedApp, err := h.apps.GetApp(c, app.Namespace, app.Name); err == nil {
				app = updatedApp
			}
		}
	}

	c.JSON(http.StatusCreated, app)
}

// GetAppDetails returns full metadata and pod statuses for an app
func (h *Handler) GetAppDetails(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")

	details, err := h.apps.GetAppDetails(c, namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("app '%s' not found: %s", name, err.Error())})
		return
	}

	c.JSON(http.StatusOK, details)
}

// DeleteApp removes an application and its associated resources
func (h *Handler) DeleteApp(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")

	if err := h.apps.DeleteApp(c, namespace, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("application '%s' successfully deleted", name)})
}

// GetAppLogs returns recent container logs
func (h *Handler) GetAppLogs(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")
	lines, _ := strconv.ParseInt(c.DefaultQuery("lines", "100"), 10, 64)

	logs, err := h.apps.GetAppLogs(c, namespace, name, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// ListOffers returns available offers from the catalog
func (h *Handler) ListOffers(c *gin.Context) {
	offers := h.offers.ListOffers(c)
	c.JSON(http.StatusOK, offers)
}

// InstallOffer triggers installation of the offer's cluster components
func (h *Handler) InstallOffer(c *gin.Context) {
	offerID := c.Param("id")

	// Parse optional body for component flags
	var req struct {
		Prometheus bool `json:"prometheus"`
		Grafana    bool `json:"grafana"`
		OTEL       bool `json:"otel"`
	}
	// Default to all components if no body
	req.Prometheus = true
	req.Grafana = true
	req.OTEL = false
	_ = c.ShouldBindJSON(&req)

	if err := h.offers.InstallOffer(c, offerID, req.Prometheus, req.Grafana, req.OTEL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to install offer '%s': %s", offerID, err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("offer '%s' installed successfully", offerID)})
}

// GetOfferStatus returns component status for monitoring offer
func (h *Handler) GetOfferStatus(c *gin.Context) {
	offerID := c.Param("id")
	status, err := h.offers.GetOfferStatus(c, offerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// LinkOffer binds an offer to a specific application
func (h *Handler) LinkOffer(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")

	var req LinkOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	app, err := h.apps.GetApp(c, namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("app '%s' not found: %s", name, err.Error())})
		return
	}

	injectedConfig, err := h.offers.BindOffer(c, app, req.OfferID, req.Parameters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to link offer '%s': %s", req.OfferID, err.Error())})
		return
	}

	// Fetch updated app
	updatedApp, _ := h.apps.GetApp(c, namespace, name)

	c.JSON(http.StatusOK, gin.H{
		"message":        fmt.Sprintf("offer '%s' linked to '%s'", req.OfferID, name),
		"injectedConfig": injectedConfig,
		"application":    updatedApp,
	})
}

// UnlinkOffer detaches an offer from an application
func (h *Handler) UnlinkOffer(c *gin.Context) {
	name := c.Param("name")
	offerID := c.Param("offerId")
	namespace := c.Query("namespace")

	app, err := h.apps.GetApp(c, namespace, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("app '%s' not found: %s", name, err.Error())})
		return
	}

	if err := h.offers.UnbindOffer(c, app, offerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to unlink offer '%s': %s", offerID, err.Error())})
		return
	}

	// Fetch updated app
	updatedApp, _ := h.apps.GetApp(c, namespace, name)

	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("offer '%s' unlinked from '%s'", offerID, name),
		"application": updatedApp,
	})
}

// GetRegistry returns the current private registry configuration status
func (h *Handler) GetRegistry(c *gin.Context) {
	status := h.registry.GetRegistryStatus(c)
	c.JSON(http.StatusOK, status)
}

// UpdateRegistry configures private registry credentials and synchronizes secrets
func (h *Handler) UpdateRegistry(c *gin.Context) {
	var cfg RegistryConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	if err := h.registry.SaveRegistryConfig(c, cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save registry config: " + err.Error()})
		return
	}

	status := h.registry.GetRegistryStatus(c)
	c.JSON(http.StatusOK, gin.H{
		"message":  "registry configuration saved successfully",
		"registry": status,
	})
}

// CreateCustomOffer registers a user-defined offer dynamically
func (h *Handler) CreateCustomOffer(c *gin.Context) {
	var def offers.DynamicOfferDefinition
	if err := c.ShouldBindJSON(&def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	if err := h.offers.CreateCustomOffer(c, def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create custom offer: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": fmt.Sprintf("custom offer '%s' created successfully", def.Name)})
}

// DeleteCustomOffer removes a user-defined custom offer
func (h *Handler) DeleteCustomOffer(c *gin.Context) {
	id := c.Param("id")
	if err := h.offers.DeleteCustomOffer(c, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete custom offer: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("custom offer '%s' deleted successfully", id)})
}

// GetClusterContexts lists all available contexts in kubeconfig
func (h *Handler) GetClusterContexts(c *gin.Context) {
	contexts, current, err := h.cluster.GetClusterContexts(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contexts": contexts,
		"current":  current,
	})
}

// SwitchClusterContext switches active Kubernetes context
func (h *Handler) SwitchClusterContext(c *gin.Context) {
	var req struct {
		ContextName string `json:"contextName" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	if err := h.cluster.SwitchClusterContext(c, req.ContextName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to switch context to '%s': %s", req.ContextName, err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("switched to cluster context '%s'", req.ContextName)})
}

// SetClusterKubeconfig connects to an external cluster (e.g. Raspberry Pi) via uploaded/pasted Kubeconfig
func (h *Handler) SetClusterKubeconfig(c *gin.Context) {
	var req struct {
		Kubeconfig  string `json:"kubeconfig" binding:"required"`
		ContextName string `json:"contextName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload: " + err.Error()})
		return
	}

	if err := h.cluster.SetClusterKubeconfig(c, []byte(req.Kubeconfig), req.ContextName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to external cluster: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "connected to external cluster successfully"})
}
