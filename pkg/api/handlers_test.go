package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nicolas-limadev/navis-paas/pkg/models"
	"github.com/nicolas-limadev/navis-paas/pkg/offers"
)

type mockAppService struct {
	apps map[string]models.Application
}

func newMockAppService() *mockAppService {
	return &mockAppService{
		apps: make(map[string]models.Application),
	}
}

func (m *mockAppService) CreateOrUpdateApp(ctx context.Context, req models.CreateAppRequest) (*models.Application, error) {
	app := models.Application{
		Name:         req.Name,
		Namespace:    "navis-apps",
		Image:        req.Image,
		Port:         req.Port,
		Replicas:     1,
		ReadyCount:   1,
		Status:       "Running",
		LinkedOffers: []models.LinkedOffer{},
		CreatedAt:    time.Now(),
	}
	m.apps[req.Name] = app
	return &app, nil
}

func (m *mockAppService) ListApps(ctx context.Context, namespace string) ([]models.Application, error) {
	var list []models.Application
	for _, a := range m.apps {
		list = append(list, a)
	}
	return list, nil
}

func (m *mockAppService) GetApp(ctx context.Context, namespace, name string) (*models.Application, error) {
	app, exists := m.apps[name]
	if !exists {
		return nil, http.ErrMissingFile
	}
	return &app, nil
}

func (m *mockAppService) GetAppDetails(ctx context.Context, namespace, name string) (*models.AppDetailResponse, error) {
	app, err := m.GetApp(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return &models.AppDetailResponse{
		Application: *app,
		Pods: []models.PodInfo{
			{Name: name + "-pod-xyz", Status: "Running", Restarts: 0},
		},
	}, nil
}

func (m *mockAppService) DeleteApp(ctx context.Context, namespace, name string) error {
	delete(m.apps, name)
	return nil
}

func (m *mockAppService) GetAppLogs(ctx context.Context, namespace, name string, tailLines int64) (string, error) {
	return "mock container logs line 1\nmock container logs line 2", nil
}

type mockOfferService struct {
	offers []models.OfferDefinition
}

func newMockOfferService() *mockOfferService {
	return &mockOfferService{
		offers: []models.OfferDefinition{
			{ID: "kafka", Name: "Apache Kafka", Category: "Messaging"},
			{ID: "monitoring", Name: "Observability", Category: "Observability"},
		},
	}
}

func (m *mockOfferService) ListOffers(ctx context.Context) []models.OfferDefinition {
	return m.offers
}

func (m *mockOfferService) InstallOffer(ctx context.Context, offerID string, prometheus, grafana, otel bool) error {
	return nil
}

func (m *mockOfferService) GetOfferStatus(ctx context.Context, offerID string) (map[string]bool, error) {
	return map[string]bool{"installed": true}, nil
}

func (m *mockOfferService) BindOffer(ctx context.Context, app *models.Application, offerID string, params map[string]string) (map[string]string, error) {
	envs := map[string]string{
		"OFFER_BOUND": offerID,
	}
	app.LinkedOffers = append(app.LinkedOffers, models.LinkedOffer{
		OfferID:   offerID,
		OfferName: offerID,
		BoundAt:   time.Now(),
		Config:    envs,
	})
	return envs, nil
}

func (m *mockOfferService) UnbindOffer(ctx context.Context, app *models.Application, offerID string) error {
	var filtered []models.LinkedOffer
	for _, o := range app.LinkedOffers {
		if o.OfferID != offerID {
			filtered = append(filtered, o)
		}
	}
	app.LinkedOffers = filtered
	return nil
}

func (m *mockOfferService) CreateCustomOffer(ctx context.Context, def offers.DynamicOfferDefinition) error {
	m.offers = append(m.offers, models.OfferDefinition{
		ID:          def.ID,
		Name:        def.Name,
		Category:    def.Category,
		Description: def.Description,
		Version:     def.Version,
	})
	return nil
}

func (m *mockOfferService) DeleteCustomOffer(ctx context.Context, id string) error {
	var filtered []models.OfferDefinition
	for _, o := range m.offers {
		if o.ID != id {
			filtered = append(filtered, o)
		}
	}
	m.offers = filtered
	return nil
}

type mockClusterChecker struct{}

func (m *mockClusterChecker) CheckStatus(ctx context.Context) models.ClusterStatus {
	return models.ClusterStatus{
		Connected:      true,
		ClusterVersion: "v1.32.0",
		MinikubeActive: true,
	}
}

func (m *mockClusterChecker) GetClusterContexts(ctx context.Context) ([]string, string, error) {
	return []string{"minikube", "raspberry-pi-cluster"}, "minikube", nil
}

func (m *mockClusterChecker) SwitchClusterContext(ctx context.Context, contextName string) error {
	return nil
}

func (m *mockClusterChecker) SetClusterKubeconfig(ctx context.Context, rawKubeconfig []byte, contextName string) error {
	return nil
}

type mockRegistryProvider struct {
	config models.RegistryConfig
}

func (m *mockRegistryProvider) GetRegistryStatus(ctx context.Context) models.RegistryStatusResponse {
	return models.RegistryStatusResponse{
		Server:        m.config.Server,
		Username:      m.config.Username,
		HasPassword:   m.config.Password != "",
		DefaultPrefix: m.config.DefaultPrefix,
		Enabled:       m.config.Enabled,
		SecretName:    "navis-registry-secret",
	}
}

func (m *mockRegistryProvider) SaveRegistryConfig(ctx context.Context, cfg models.RegistryConfig) error {
	m.config = cfg
	return nil
}

func setupTestRouter() http.Handler {
	appSvc := newMockAppService()
	offerSvc := newMockOfferService()
	clusterChecker := &mockClusterChecker{}
	regProvider := &mockRegistryProvider{
		config: models.RegistryConfig{
			Server:   "ghcr.io",
			Username: "orguser",
			Enabled:  true,
		},
	}

	h := NewHandler(appSvc, offerSvc, clusterChecker, regProvider)
	return SetupRouter(h, "")
}

func TestHealthEndpoint(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var status models.ClusterStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !status.Connected || !status.MinikubeActive {
		t.Errorf("expected cluster connected and minikube active")
	}
}

func TestOffersEndpoint(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/offers", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var offers []models.OfferDefinition
	if err := json.Unmarshal(w.Body.Bytes(), &offers); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(offers) != 2 {
		t.Errorf("expected 2 offers, got %d", len(offers))
	}
}

func TestCreateAndListApps(t *testing.T) {
	router := setupTestRouter()

	// 1. Create app
	payload := models.CreateAppRequest{
		Name:     "order-service",
		Image:    "hashicorp/http-echo:0.2.3",
		Port:     5678,
		Replicas: 1,
		Offers:   []string{"kafka"},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "/api/v1/apps", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// 2. List apps
	reqList, _ := http.NewRequest("GET", "/api/v1/apps", nil)
	wList := httptest.NewRecorder()
	router.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wList.Code)
	}

	var apps []models.Application
	_ = json.Unmarshal(wList.Body.Bytes(), &apps)
	if len(apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(apps))
	}
	if apps[0].Name != "order-service" {
		t.Errorf("expected app name 'order-service', got '%s'", apps[0].Name)
	}
}

func TestOpenAPISpecEndpoint(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("failed to decode OpenAPI spec: %v", err)
	}

	if openapi, ok := spec["openapi"].(string); !ok || !strings.HasPrefix(openapi, "3.") {
		t.Errorf("expected openapi 3.x, got %v", spec["openapi"])
	}
}

func TestRegistryEndpoints(t *testing.T) {
	router := setupTestRouter()

	// 1. GET /api/v1/registry
	req, _ := http.NewRequest("GET", "/api/v1/registry", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var status models.RegistryStatusResponse
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	if status.Server != "ghcr.io" {
		t.Errorf("expected server 'ghcr.io', got '%s'", status.Server)
	}

	// 2. POST /api/v1/registry
	payload := models.RegistryConfig{
		Server:        "docker.io",
		Username:      "dockeruser",
		Password:      "secrettoken",
		DefaultPrefix: "docker.io/myteam",
		Enabled:       true,
	}
	body, _ := json.Marshal(payload)

	reqPost, _ := http.NewRequest("POST", "/api/v1/registry", bytes.NewBuffer(body))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	router.ServeHTTP(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected status 200 on update registry, got %d: %s", wPost.Code, wPost.Body.String())
	}
}
