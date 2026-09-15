package offers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/nicolas-limadev/navis-paas/pkg/k8s"
	"github.com/nicolas-limadev/navis-paas/pkg/models"
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
	c.Register(&MySQLOffer{})
	c.Register(&MongoDBOffer{})
	c.Register(&KongGatewayOffer{})
	c.Register(&NginxOffer{})
	c.Register(&RabbitMQOffer{})
	c.Register(&NATSOffer{})
	c.Register(&MinIOOffer{})

	// Load dynamic/custom YAML-based offers from disk
	// 1. Scan current working directory 'custom-offers'
	cwd, err := os.Getwd()
	if err == nil {
		customOffers, _ := LoadCustomOffers(filepath.Join(cwd, "custom-offers"))
		for _, o := range customOffers {
			c.Register(o)
		}
	}

	// 2. Scan $HOME/.navis/offers
	homeDir, err := os.UserHomeDir()
	if err == nil {
		customOffers, _ := LoadCustomOffers(filepath.Join(homeDir, ".navis", "offers"))
		for _, o := range customOffers {
			c.Register(o)
		}
	}

	return c
}

func (c *Catalog) Register(handler OfferHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	def := handler.GetDefinition()
	c.handlers[def.ID] = handler
}

func (c *Catalog) ListOffers(ctx context.Context) []models.OfferDefinition {
	if c.cm != nil {
		_ = c.cm.EnsureConnected(ctx)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []models.OfferDefinition
	for _, handler := range c.handlers {
		def := handler.GetDefinition()
		if c.cm != nil && c.cm.Connected {
			installed, _ := handler.IsInstalled(ctx, c.cm)
			def.Installed = installed
		}
		result = append(result, def)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ID < result[j].ID
	})

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

func (c *Catalog) AddCustomOffer(def DynamicOfferDefinition) error {
	if err := SaveCustomOffer(def); err != nil {
		return err
	}
	c.Register(&DynamicOffer{Def: def})
	return nil
}

func (c *Catalog) RemoveCustomOffer(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.handlers, id)

	cwd, err := os.Getwd()
	if err == nil {
		_ = os.Remove(filepath.Join(cwd, "custom-offers", id+".yaml"))
		_ = os.Remove(filepath.Join(cwd, "custom-offers", id+".yml"))
	}
	return nil
}
