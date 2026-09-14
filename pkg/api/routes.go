package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	navispaas "github.com/nicolas-limadev/navis-paas"
	"github.com/nicolas-limadev/navis-paas/pkg/offers"
)

// SetupRouter configures the standard net/http router and Huma v2 API engine
func SetupRouter(h *Handler, staticDir string) http.Handler {
	mux := http.NewServeMux()

	// 1. CORS Middleware Wrapper
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		mux.ServeHTTP(w, r)
	})

	// 2. Serve Static UI Dashboard (prefer disk fallback to embedded)
	served := false
	if staticDir != "" {
		if _, err := os.Stat(staticDir); err == nil {
			fileServer := http.FileServer(http.Dir(staticDir))
			mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					http.NotFound(w, r)
					return
				}
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			})
			served = true
		}
	}

	if !served {
		// Fallback to embedded static files
		subFS, err := fs.Sub(navispaas.StaticFS, "web/static")
		if err == nil {
			fileServer := http.FileServer(http.FS(subFS))
			mux.Handle("/static/", http.StripPrefix("/static/", fileServer))
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					http.NotFound(w, r)
					return
				}
				data, err := fs.ReadFile(subFS, "index.html")
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data)
			})
		}
	}

	// 3. Configure Huma v2 OpenAPI Engine
	config := huma.DefaultConfig("NavisPaaS Core Engine API", "1.0.0")
	config.OpenAPIPath = "/api/v1/openapi.json"
	config.DocsPath = "/api/v1/docs"
	api := humago.New(mux, config)

	// Explicit handler for /api/v1/openapi.json
	mux.HandleFunc("GET /api/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, err := api.OpenAPI().Downgrade()
		if err != nil {
			b, _ = api.OpenAPI().MarshalJSON()
		}
		w.Write(b)
	})

	// --- Register Huma Operations ---

	// GET /api/v1/health
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/api/v1/health",
		Summary:     "Check Kubernetes cluster connectivity and status",
	}, func(ctx context.Context, input *struct{}) (*struct{ Body ClusterStatus }, error) {
		status := h.cluster.CheckStatus(ctx)
		return &struct{ Body ClusterStatus }{Body: status}, nil
	})

	// GET /api/v1/cluster/contexts
	huma.Register(api, huma.Operation{
		OperationID: "get-cluster-contexts",
		Method:      http.MethodGet,
		Path:        "/api/v1/cluster/contexts",
		Summary:     "List available cluster contexts in kubeconfig",
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Contexts []string `json:"contexts"`
			Current  string   `json:"current"`
		}
	}, error) {
		contexts, current, err := h.cluster.GetClusterContexts(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Contexts []string `json:"contexts"`
				Current  string   `json:"current"`
			}
		}{}
		res.Body.Contexts = contexts
		res.Body.Current = current
		return &res, nil
	})

	// POST /api/v1/cluster/context
	type SwitchContextInput struct {
		Body struct {
			ContextName string `json:"contextName" doc:"Name of context to switch to"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "switch-cluster-context",
		Method:      http.MethodPost,
		Path:        "/api/v1/cluster/context",
		Summary:     "Switch active Kubernetes cluster context",
	}, func(ctx context.Context, input *SwitchContextInput) (*struct {
		Body struct {
			Message string `json:"message"`
		}
	}, error) {
		if err := h.cluster.SwitchClusterContext(ctx, input.Body.ContextName); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Message string `json:"message"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("switched to cluster context '%s'", input.Body.ContextName)
		return &res, nil
	})

	// POST /api/v1/cluster/kubeconfig
	type SetKubeconfigInput struct {
		Body struct {
			Kubeconfig  string `json:"kubeconfig" doc:"Raw YAML kubeconfig content"`
			ContextName string `json:"contextName,omitempty" doc:"Optional custom context name"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "set-cluster-kubeconfig",
		Method:      http.MethodPost,
		Path:        "/api/v1/cluster/kubeconfig",
		Summary:     "Connect to external cluster (e.g. Raspberry Pi) via raw kubeconfig",
	}, func(ctx context.Context, input *SetKubeconfigInput) (*struct {
		Body struct {
			Message string `json:"message"`
		}
	}, error) {
		if err := h.cluster.SetClusterKubeconfig(ctx, []byte(input.Body.Kubeconfig), input.Body.ContextName); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Message string `json:"message"`
			}
		}{}
		res.Body.Message = "connected to external cluster successfully"
		return &res, nil
	})

	// GET /api/v1/apps
	type ListAppsInput struct {
		Namespace string `query:"namespace" doc:"Optional namespace filter"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "list-apps",
		Method:      http.MethodGet,
		Path:        "/api/v1/apps",
		Summary:     "List deployed applications",
	}, func(ctx context.Context, input *ListAppsInput) (*struct{ Body []Application }, error) {
		apps, err := h.apps.ListApps(ctx, input.Namespace)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		if apps == nil {
			apps = []Application{}
		}
		return &struct{ Body []Application }{Body: apps}, nil
	})

	// POST /api/v1/apps
	type CreateAppInput struct {
		Body CreateAppRequest
	}
	huma.Register(api, huma.Operation{
		OperationID:   "create-app",
		Method:        http.MethodPost,
		Path:          "/api/v1/apps",
		Summary:       "Deploy a new application with optional offer bindings",
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *CreateAppInput) (*struct{ Body *Application }, error) {
		app, err := h.apps.CreateOrUpdateApp(ctx, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}

		// Auto-bind any requested offers
		for _, offerID := range input.Body.Offers {
			_, bindErr := h.offers.BindOffer(ctx, app, offerID, nil)
			if bindErr == nil {
				if updatedApp, err := h.apps.GetApp(ctx, app.Namespace, app.Name); err == nil {
					app = updatedApp
				}
			}
		}

		return &struct{ Body *Application }{Body: app}, nil
	})

	// GET /api/v1/apps/{name}
	type GetAppInput struct {
		Name      string `path:"name"`
		Namespace string `query:"namespace"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-app-details",
		Method:      http.MethodGet,
		Path:        "/api/v1/apps/{name}",
		Summary:     "Get application details and pod status",
	}, func(ctx context.Context, input *GetAppInput) (*struct{ Body *AppDetailResponse }, error) {
		details, err := h.apps.GetAppDetails(ctx, input.Namespace, input.Name)
		if err != nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("app '%s' not found: %s", input.Name, err.Error()))
		}
		return &struct{ Body *AppDetailResponse }{Body: details}, nil
	})

	// DELETE /api/v1/apps/{name}
	type DeleteAppInput struct {
		Name      string `path:"name"`
		Namespace string `query:"namespace"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "delete-app",
		Method:      http.MethodDelete,
		Path:        "/api/v1/apps/{name}",
		Summary:     "Delete an application",
	}, func(ctx context.Context, input *DeleteAppInput) (*struct {
		Body struct {
			Message string `json:"message"`
		}
	}, error) {
		if err := h.apps.DeleteApp(ctx, input.Namespace, input.Name); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Message string `json:"message"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("application '%s' successfully deleted", input.Name)
		return &res, nil
	})

	// GET /api/v1/apps/{name}/logs
	type GetAppLogsInput struct {
		Name      string `path:"name"`
		Namespace string `query:"namespace"`
		Lines     int64  `query:"lines" default:"100"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-app-logs",
		Method:      http.MethodGet,
		Path:        "/api/v1/apps/{name}/logs",
		Summary:     "Get recent pod logs for the application",
	}, func(ctx context.Context, input *GetAppLogsInput) (*struct {
		Body struct {
			Logs string `json:"logs"`
		}
	}, error) {
		lines := input.Lines
		if lines <= 0 {
			lines = 100
		}
		logs, err := h.apps.GetAppLogs(ctx, input.Namespace, input.Name, lines)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Logs string `json:"logs"`
			}
		}{}
		res.Body.Logs = logs
		return &res, nil
	})

	// GET /api/v1/offers
	huma.Register(api, huma.Operation{
		OperationID: "list-offers",
		Method:      http.MethodGet,
		Path:        "/api/v1/offers",
		Summary:     "List available infrastructure offers",
	}, func(ctx context.Context, input *struct{}) (*struct{ Body []OfferDefinition }, error) {
		offers := h.offers.ListOffers(ctx)
		return &struct{ Body []OfferDefinition }{Body: offers}, nil
	})

	// POST /api/v1/offers/{id}/install
	type InstallOfferInput struct {
		ID   string `path:"id"`
		Body struct {
			Prometheus bool `json:"prometheus"`
			Grafana    bool `json:"grafana"`
			OTEL       bool `json:"otel"`
		}
	}
	huma.Register(api, huma.Operation{
		OperationID: "install-offer",
		Method:      http.MethodPost,
		Path:        "/api/v1/offers/{id}/install",
		Summary:     "Install offer cluster components",
	}, func(ctx context.Context, input *InstallOfferInput) (*struct {
		Body struct {
			Message string `json:"message"`
		}
	}, error) {
		// Default prometheus and grafana to true if body is uninitialized
		prom := input.Body.Prometheus
		graf := input.Body.Grafana
		otel := input.Body.OTEL
		if !prom && !graf && !otel {
			prom = true
			graf = true
		}

		if err := h.offers.InstallOffer(ctx, input.ID, prom, graf, otel); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to install offer '%s': %s", input.ID, err.Error()))
		}
		res := struct {
			Body struct {
				Message string `json:"message"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("offer '%s' installed successfully", input.ID)
		return &res, nil
	})

	// GET /api/v1/offers/{id}/status
	type GetOfferStatusInput struct {
		ID string `path:"id"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "get-offer-status",
		Method:      http.MethodGet,
		Path:        "/api/v1/offers/{id}/status",
		Summary:     "Get component installation status for an offer",
	}, func(ctx context.Context, input *GetOfferStatusInput) (*struct{ Body map[string]bool }, error) {
		status, err := h.offers.GetOfferStatus(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		return &struct{ Body map[string]bool }{Body: status}, nil
	})

	// POST /api/v1/offers/custom
	type CreateCustomOfferInput struct {
		Body offers.DynamicOfferDefinition
	}
	huma.Register(api, huma.Operation{
		OperationID: "create-custom-offer",
		Method:      http.MethodPost,
		Path:        "/api/v1/offers/custom",
		Summary:     "Register a custom YAML offer dynamically",
	}, func(ctx context.Context, input *CreateCustomOfferInput) (*struct {
		Body struct {
			Message string `json:"message"`
		}
	}, error) {
		if err := h.offers.CreateCustomOffer(ctx, input.Body); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Message string `json:"message"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("custom offer '%s' created successfully", input.Body.Name)
		return &res, nil
	})

	// DELETE /api/v1/offers/custom/{id}
	type DeleteCustomOfferInput struct {
		ID string `path:"id"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "delete-custom-offer",
		Method:      http.MethodDelete,
		Path:        "/api/v1/offers/custom/{id}",
		Summary:     "Delete a custom offer definition",
	}, func(ctx context.Context, input *DeleteCustomOfferInput) (*struct {
		Body struct {
			Message string `json:"message"`
		}
	}, error) {
		if err := h.offers.DeleteCustomOffer(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError(err.Error())
		}
		res := struct {
			Body struct {
				Message string `json:"message"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("custom offer '%s' deleted successfully", input.ID)
		return &res, nil
	})

	// POST /api/v1/apps/{name}/links
	type LinkOfferInput struct {
		Name      string `path:"name"`
		Namespace string `query:"namespace"`
		Body      LinkOfferRequest
	}
	huma.Register(api, huma.Operation{
		OperationID: "link-offer",
		Method:      http.MethodPost,
		Path:        "/api/v1/apps/{name}/links",
		Summary:     "Link an offer to an application and inject configurations",
	}, func(ctx context.Context, input *LinkOfferInput) (*struct {
		Body struct {
			Message        string            `json:"message"`
			InjectedConfig map[string]string `json:"injectedConfig"`
			Application    *Application      `json:"application"`
		}
	}, error) {
		app, err := h.apps.GetApp(ctx, input.Namespace, input.Name)
		if err != nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("app '%s' not found: %s", input.Name, err.Error()))
		}

		injectedConfig, err := h.offers.BindOffer(ctx, app, input.Body.OfferID, input.Body.Parameters)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to link offer '%s': %s", input.Body.OfferID, err.Error()))
		}

		updatedApp, _ := h.apps.GetApp(ctx, input.Namespace, input.Name)

		res := struct {
			Body struct {
				Message        string            `json:"message"`
				InjectedConfig map[string]string `json:"injectedConfig"`
				Application    *Application      `json:"application"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("offer '%s' linked to '%s'", input.Body.OfferID, input.Name)
		res.Body.InjectedConfig = injectedConfig
		res.Body.Application = updatedApp

		return &res, nil
	})

	// DELETE /api/v1/apps/{name}/links/{offerId}
	type UnlinkOfferInput struct {
		Name      string `path:"name"`
		OfferID   string `path:"offerId"`
		Namespace string `query:"namespace"`
	}
	huma.Register(api, huma.Operation{
		OperationID: "unlink-offer",
		Method:      http.MethodDelete,
		Path:        "/api/v1/apps/{name}/links/{offerId}",
		Summary:     "Unlink an offer from an application",
	}, func(ctx context.Context, input *UnlinkOfferInput) (*struct {
		Body struct {
			Message     string       `json:"message"`
			Application *Application `json:"application"`
		}
	}, error) {
		app, err := h.apps.GetApp(ctx, input.Namespace, input.Name)
		if err != nil {
			return nil, huma.Error404NotFound(fmt.Sprintf("app '%s' not found: %s", input.Name, err.Error()))
		}

		if err := h.offers.UnbindOffer(ctx, app, input.OfferID); err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("failed to unlink offer '%s': %s", input.OfferID, err.Error()))
		}

		updatedApp, _ := h.apps.GetApp(ctx, input.Namespace, input.Name)

		res := struct {
			Body struct {
				Message     string       `json:"message"`
				Application *Application `json:"application"`
			}
		}{}
		res.Body.Message = fmt.Sprintf("offer '%s' unlinked from '%s'", input.OfferID, input.Name)
		res.Body.Application = updatedApp

		return &res, nil
	})

	// GET /api/v1/registry
	huma.Register(api, huma.Operation{
		OperationID: "get-registry",
		Method:      http.MethodGet,
		Path:        "/api/v1/registry",
		Summary:     "Get private Docker registry configuration status",
	}, func(ctx context.Context, input *struct{}) (*struct{ Body RegistryStatusResponse }, error) {
		status := h.registry.GetRegistryStatus(ctx)
		return &struct{ Body RegistryStatusResponse }{Body: status}, nil
	})

	// POST /api/v1/registry
	type UpdateRegistryInput struct {
		Body RegistryConfig
	}
	huma.Register(api, huma.Operation{
		OperationID: "update-registry",
		Method:      http.MethodPost,
		Path:        "/api/v1/registry",
		Summary:     "Update private Docker registry credentials and sync secrets",
	}, func(ctx context.Context, input *UpdateRegistryInput) (*struct {
		Body struct {
			Message  string                 `json:"message"`
			Registry RegistryStatusResponse `json:"registry"`
		}
	}, error) {
		if err := h.registry.SaveRegistryConfig(ctx, input.Body); err != nil {
			return nil, huma.Error500InternalServerError("failed to save registry config: " + err.Error())
		}

		status := h.registry.GetRegistryStatus(ctx)

		res := struct {
			Body struct {
				Message  string                 `json:"message"`
				Registry RegistryStatusResponse `json:"registry"`
			}
		}{}
		res.Body.Message = "registry configuration saved successfully"
		res.Body.Registry = status

		return &res, nil
	})

	return corsHandler
}
