package k8s

import (
	"context"
	"testing"

	"navispaas/pkg/models"
)

func TestRegistryServiceResolveImage(t *testing.T) {
	cm := &ClientManager{}
	svc := NewRegistryService(cm)

	// Case 1: Registry disabled
	if svc.ResolveImage("order-service:1.0") != "order-service:1.0" {
		t.Errorf("expected unchanged image when registry disabled")
	}

	// Case 2: Registry enabled with default prefix
	_ = svc.SaveConfig(context.Background(), models.RegistryConfig{
		Enabled:       true,
		DefaultPrefix: "ghcr.io/company",
		Username:      "user",
		Password:      "pass",
	})

	resolved := svc.ResolveImage("order-service:1.0")
	expected := "ghcr.io/company/order-service:1.0"
	if resolved != expected {
		t.Errorf("expected '%s', got '%s'", expected, resolved)
	}

	// Case 3: Image already has registry/org prefix
	alreadyQualified := "quay.io/other/tool:v2"
	if svc.ResolveImage(alreadyQualified) != alreadyQualified {
		t.Errorf("expected fully qualified image to remain unchanged")
	}
}

func TestRegistryServicePullSecrets(t *testing.T) {
	cm := &ClientManager{}
	svc := NewRegistryService(cm)

	// Disabled -> nil secrets
	if svc.GetImagePullSecrets() != nil {
		t.Errorf("expected nil pull secrets when disabled")
	}

	// Enabled
	_ = svc.SaveConfig(context.Background(), models.RegistryConfig{
		Enabled:    true,
		Username:   "user",
		Password:   "pass",
		SecretName: "custom-registry-key",
	})

	secrets := svc.GetImagePullSecrets()
	if len(secrets) != 1 || secrets[0].Name != "custom-registry-key" {
		t.Errorf("expected 1 pull secret named 'custom-registry-key', got %+v", secrets)
	}
}
