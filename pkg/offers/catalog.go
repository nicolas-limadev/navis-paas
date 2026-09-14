package offers

import (
	"context"
	"fmt"
	"sync"

	"github.com/nicolas-limadev/navis-paas/pkg/models"
	"github.com/nicolas-limadev/navis-paas/pkg/k8s"
)

// OfferHandler defines the contract for any addon/offer in NavisPaaS
type OfferHandler interface {
	GetDefinition() models.OfferDefinition
	IsInstalled(ctx context.Context, cm *k8s.ClientManager) (bool, error)
	Install(ctx context.Context, cm *k8s.ClientManager) error
	Bind(ctx context.Context, cm *k8s.ClientManager, app *models.Application, params map[string]string) (map[string]string, error)
	Unbind(ctx context.Context, cm *k8s.ClientManager, app *models.Application) error
}

type Catalog struct {
	mu       sync.RWMutex
	handlers map[string]OfferHandler
	cm       *k8s.ClientManager
}

func NewCatalog(cm *k8s.ClientManager) *Catalog {
	c := &Catalog{
		handlers: make(map[string]OfferHandler),
		cm:       cm,
	}

	// Register core offers
	c.Register(&KafkaOffer{})
	c.Register(&MonitoringOffer{})
	c.Register(&RedisOffer{})
	c.Register(&PostgresOffer{})
	c.Register(&KongGatewayOffer{})
	c.Register(&RabbitMQOffer{})

	return c
}

func (c *Catalog) Register(handler OfferHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	def := handler.GetDefinition()
	c.handlers[def.ID] = handler
}

func (c *Catalog) ListOffers(ctx context.Context) []models.OfferDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []models.OfferDefinition
	for _, handler := range c.handlers {
		def := handler.GetDefinition()
		if c.cm.Connected {
			installed, _ := handler.IsInstalled(ctx, c.cm)
			def.Installed = installed
		}
		result = append(result, def)
	}
	return result
}

func (c *Catalog) GetOffer(id string) (OfferHandler, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	handler, exists := c.handlers[id]
	if !exists {
		return nil, fmt.Errorf("offer '%s' not found in catalog", id)
	}
	return handler, nil
}
