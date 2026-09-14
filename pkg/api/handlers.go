package api

import (
	"context"

	"github.com/nicolas-limadev/navis-paas/pkg/offers"
)

// AppServiceProvider abstracts K8s app service
type AppServiceProvider interface {
	CreateOrUpdateApp(ctx context.Context, req CreateAppRequest) (*Application, error)
	ListApps(ctx context.Context, namespace string) ([]Application, error)
	GetApp(ctx context.Context, namespace, name string) (*Application, error)
	GetAppDetails(ctx context.Context, namespace, name string) (*AppDetailResponse, error)
	DeleteApp(ctx context.Context, namespace, name string) error
	GetAppLogs(ctx context.Context, namespace, name string, tailLines int64) (string, error)
}

// OfferServiceProvider abstracts the catalog & binding engine
type OfferServiceProvider interface {
	ListOffers(ctx context.Context) []OfferDefinition
	InstallOffer(ctx context.Context, offerID string, prometheus, grafana, otel bool) error
	GetOfferStatus(ctx context.Context, offerID string) (map[string]bool, error)
	BindOffer(ctx context.Context, app *Application, offerID string, params map[string]string) (map[string]string, error)
	UnbindOffer(ctx context.Context, app *Application, offerID string) error
	CreateCustomOffer(ctx context.Context, def offers.DynamicOfferDefinition) error
	DeleteCustomOffer(ctx context.Context, id string) error
}

// ClusterChecker checks K8s cluster status
type ClusterChecker interface {
	CheckStatus(ctx context.Context) ClusterStatus
	GetClusterContexts(ctx context.Context) ([]string, string, error)
	SwitchClusterContext(ctx context.Context, contextName string) error
	SetClusterKubeconfig(ctx context.Context, rawKubeconfig []byte, contextName string) error
}

// RegistryProvider manages private container registries
type RegistryProvider interface {
	GetRegistryStatus(ctx context.Context) RegistryStatusResponse
	SaveRegistryConfig(ctx context.Context, cfg RegistryConfig) error
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
