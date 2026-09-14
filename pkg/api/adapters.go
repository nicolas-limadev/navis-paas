package api

import (
	"context"

	"github.com/nicolas-limadev/navis-paas/pkg/k8s"
	"github.com/nicolas-limadev/navis-paas/pkg/offers"

	"github.com/gin-gonic/gin"
)

type AppAdapter struct {
	svc *k8s.AppService
}

func NewAppAdapter(svc *k8s.AppService) *AppAdapter {
	return &AppAdapter{svc: svc}
}

func (a *AppAdapter) CreateOrUpdateApp(c *gin.Context, req CreateAppRequest) (*Application, error) {
	return a.svc.CreateOrUpdateApp(c.Request.Context(), req)
}

func (a *AppAdapter) ListApps(c *gin.Context, namespace string) ([]Application, error) {
	return a.svc.ListApps(c.Request.Context(), namespace)
}

func (a *AppAdapter) GetApp(c *gin.Context, namespace, name string) (*Application, error) {
	return a.svc.GetApp(c.Request.Context(), namespace, name)
}

func (a *AppAdapter) GetAppDetails(c *gin.Context, namespace, name string) (*AppDetailResponse, error) {
	return a.svc.GetAppDetails(c.Request.Context(), namespace, name)
}

func (a *AppAdapter) DeleteApp(c *gin.Context, namespace, name string) error {
	return a.svc.DeleteApp(c.Request.Context(), namespace, name)
}

func (a *AppAdapter) GetAppLogs(c *gin.Context, namespace, name string, tailLines int64) (string, error) {
	return a.svc.GetAppLogs(c.Request.Context(), namespace, name, tailLines)
}

type OfferAdapter struct {
	catalog *offers.Catalog
	cm      *k8s.ClientManager
}

func NewOfferAdapter(catalog *offers.Catalog, cm *k8s.ClientManager) *OfferAdapter {
	return &OfferAdapter{catalog: catalog, cm: cm}
}

func (o *OfferAdapter) ListOffers(c *gin.Context) []OfferDefinition {
	return o.catalog.ListOffers(c.Request.Context())
}

func (o *OfferAdapter) InstallOffer(c *gin.Context, offerID string, prometheus, grafana, otel bool) error {
	handler, err := o.catalog.GetOffer(offerID)
	if err != nil {
		return err
	}

	// If it's the monitoring offer, use component-level install
	if offerID == "monitoring" {
		if m, ok := handler.(*offers.MonitoringOffer); ok {
			return m.InstallWithComponents(c.Request.Context(), o.cm, prometheus, grafana, otel)
		}
	}

	return handler.Install(c.Request.Context(), o.cm)
}

func (o *OfferAdapter) GetOfferStatus(c *gin.Context, offerID string) (map[string]bool, error) {
	handler, err := o.catalog.GetOffer(offerID)
	if err != nil {
		return nil, err
	}

	// If it's the monitoring offer, get component status
	if offerID == "monitoring" {
		if m, ok := handler.(*offers.MonitoringOffer); ok {
			return m.GetComponentStatus(c.Request.Context(), o.cm), nil
		}
	}

	// For other offers, just return installed status
	installed, err := handler.IsInstalled(c.Request.Context(), o.cm)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"installed": installed}, nil
}

func (o *OfferAdapter) BindOffer(c *gin.Context, app *Application, offerID string, params map[string]string) (map[string]string, error) {
	handler, err := o.catalog.GetOffer(offerID)
	if err != nil {
		return nil, err
	}
	return handler.Bind(c.Request.Context(), o.cm, app, params)
}

func (o *OfferAdapter) UnbindOffer(c *gin.Context, app *Application, offerID string) error {
	handler, err := o.catalog.GetOffer(offerID)
	if err != nil {
		return err
	}
	return handler.Unbind(c.Request.Context(), o.cm, app)
}

func (o *OfferAdapter) CreateCustomOffer(c *gin.Context, def offers.DynamicOfferDefinition) error {
	return o.catalog.AddCustomOffer(def)
}

func (o *OfferAdapter) DeleteCustomOffer(c *gin.Context, id string) error {
	return o.catalog.RemoveCustomOffer(id)
}

type ClusterAdapter struct {
	cm *k8s.ClientManager
}

func NewClusterAdapter(cm *k8s.ClientManager) *ClusterAdapter {
	return &ClusterAdapter{cm: cm}
}

func (ca *ClusterAdapter) CheckStatus(c *gin.Context) ClusterStatus {
	ctx := context.Background()
	connected, version, err := ca.cm.CheckConnectivity(ctx)

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	serverURL := ""
	if ca.cm.Config != nil {
		serverURL = ca.cm.Config.Host
	}

	// Detect provider
	provider := ca.cm.GetProvider()
	providerName := string(provider.Name)
	providerInfo := ""
	switch provider.Name {
	case k8s.ProviderMinikube:
		providerInfo = "Minikube - Use 'make tunnel' for LoadBalancer access"
	case k8s.ProviderK3d:
		providerInfo = "k3d - Services exposed on localhost ports"
	case k8s.ProviderKind:
		providerInfo = "Kind - Use 'kubectl port-forward' for service access"
	case k8s.ProviderDockerDesktop:
		providerInfo = "Docker Desktop - Use 'kubectl port-forward' for service access"
	case k8s.ProviderRancherDesktop:
		providerInfo = "Rancher Desktop - Use 'kubectl port-forward' for service access"
	case k8s.ProviderMicroK8s:
		providerInfo = "MicroK8s - Use 'kubectl port-forward' for service access"
	default:
		providerInfo = "Unknown provider - Use 'kubectl port-forward' for service access"
	}

	return ClusterStatus{
		Connected:      connected,
		ClusterVersion: version,
		Context:        ca.cm.ContextName,
		ServerURL:      serverURL,
		MinikubeActive: connected && provider.Name == k8s.ProviderMinikube,
		Provider:       providerName,
		ProviderInfo:   providerInfo,
		ErrorMessage:   errMsg,
	}
}

func (ca *ClusterAdapter) GetClusterContexts(c *gin.Context) ([]string, string, error) {
	return ca.cm.GetAvailableContexts()
}

func (ca *ClusterAdapter) SwitchClusterContext(c *gin.Context, contextName string) error {
	return ca.cm.ConnectWithContext(contextName)
}

func (ca *ClusterAdapter) SetClusterKubeconfig(c *gin.Context, rawKubeconfig []byte, contextName string) error {
	return ca.cm.SetKubeconfigContent(rawKubeconfig, contextName)
}

type RegistryAdapter struct {
	svc *k8s.RegistryService
}

func NewRegistryAdapter(svc *k8s.RegistryService) *RegistryAdapter {
	return &RegistryAdapter{svc: svc}
}

func (ra *RegistryAdapter) GetRegistryStatus(c *gin.Context) RegistryStatusResponse {
	return ra.svc.GetStatus()
}

func (ra *RegistryAdapter) SaveRegistryConfig(c *gin.Context, cfg RegistryConfig) error {
	return ra.svc.SaveConfig(c.Request.Context(), cfg)
}
