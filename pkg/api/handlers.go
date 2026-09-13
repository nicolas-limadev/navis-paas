package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
	InstallOffer(ctx *gin.Context, offerID string) error
	BindOffer(ctx *gin.Context, app *Application, offerID string, params map[string]string) (map[string]string, error)
	UnbindOffer(ctx *gin.Context, app *Application, offerID string) error
}

// ClusterChecker checks K8s cluster status
type ClusterChecker interface {
	CheckStatus(ctx *gin.Context) ClusterStatus
}

type Handler struct {
	apps    AppServiceProvider
	offers  OfferServiceProvider
	cluster ClusterChecker
}

func NewHandler(apps AppServiceProvider, offers OfferServiceProvider, cluster ClusterChecker) *Handler {
	return &Handler{
		apps:    apps,
		offers:  offers,
		cluster: cluster,
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
	namespace := c.DefaultQuery("namespace", "navis-apps")

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
	namespace := c.DefaultQuery("namespace", "navis-apps")

	if err := h.apps.DeleteApp(c, namespace, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("application '%s' successfully deleted", name)})
}

// GetAppLogs returns recent container logs
func (h *Handler) GetAppLogs(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "navis-apps")
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
	if err := h.offers.InstallOffer(c, offerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to install offer '%s': %s", offerID, err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("offer '%s' installed successfully", offerID)})
}

// LinkOffer binds an offer to a specific application
func (h *Handler) LinkOffer(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "navis-apps")

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
	namespace := c.DefaultQuery("namespace", "navis-apps")

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
