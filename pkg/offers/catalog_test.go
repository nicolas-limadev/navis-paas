package offers

import (
	"context"
	"testing"

	"navispaas/pkg/k8s"
)

func TestCatalogOffersRegistration(t *testing.T) {
	cm := &k8s.ClientManager{}
	catalog := NewCatalog(cm)

	ctx := context.Background()
	offersList := catalog.ListOffers(ctx)

	if len(offersList) < 4 {
		t.Fatalf("expected at least 4 registered offers, got %d", len(offersList))
	}

	expectedIDs := map[string]bool{
		"kafka":      false,
		"monitoring": false,
		"redis":      false,
		"postgresql": false,
	}

	for _, o := range offersList {
		if _, exists := expectedIDs[o.ID]; exists {
			expectedIDs[o.ID] = true
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected offer '%s' to be in catalog, but it was not found", id)
		}
	}
}

func TestKafkaOfferDefinition(t *testing.T) {
	kafka := &KafkaOffer{}
	def := kafka.GetDefinition()

	if def.ID != "kafka" {
		t.Errorf("expected ID 'kafka', got '%s'", def.ID)
	}
	if def.Category != "Messaging" {
		t.Errorf("expected Category 'Messaging', got '%s'", def.Category)
	}
	if len(def.Parameters) == 0 {
		t.Errorf("expected parameters for Kafka offer, got 0")
	}

	foundTopic := false
	for _, p := range def.Parameters {
		if p.Key == "topic" {
			foundTopic = true
			if !p.Required {
				t.Errorf("topic parameter should be required")
			}
		}
	}
	if !foundTopic {
		t.Errorf("expected 'topic' parameter in Kafka offer")
	}
}

func TestMonitoringOfferDefinition(t *testing.T) {
	monitoring := &MonitoringOffer{}
	def := monitoring.GetDefinition()

	if def.ID != "monitoring" {
		t.Errorf("expected ID 'monitoring', got '%s'", def.ID)
	}
	if def.Category != "Observability" {
		t.Errorf("expected Category 'Observability', got '%s'", def.Category)
	}

	foundMetricsPath := false
	for _, p := range def.Parameters {
		if p.Key == "metricsPath" {
			foundMetricsPath = true
			if p.Default != "/metrics" {
				t.Errorf("expected default metricsPath to be '/metrics', got '%s'", p.Default)
			}
		}
	}
	if !foundMetricsPath {
		t.Errorf("expected 'metricsPath' parameter in Monitoring offer")
	}
}
