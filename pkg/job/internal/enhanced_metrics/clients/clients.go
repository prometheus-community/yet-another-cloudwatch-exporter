package clients

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// separator is NULL byte (0x00)
const separator = "\x00"

type Clients[T any] struct {
	regionalClients map[string]T
	clientsM        sync.RWMutex

	buildClientFunc func(cfg aws.Config) T
}

func NewClients[T any](buildClientFunc func(cfg aws.Config) T) *Clients[T] {
	return &Clients[T]{
		buildClientFunc: buildClientFunc,
		regionalClients: make(map[string]T),
	}
}

func (c *Clients[T]) InitializeClient(region string, role model.Role, configProvider config.RegionalConfigProvider) (T, error) {
	var zero T
	regionalConfig := configProvider.GetAWSRegionalConfig(region, role)
	if regionalConfig == nil {
		return zero, fmt.Errorf("could not get AWS config for region %s", region)
	}

	c.clientsM.Lock()
	defer c.clientsM.Unlock()

	key := c.getClientKey(region, role)
	if c.regionalClients == nil {
		c.regionalClients = make(map[string]T)
	}
	_, exists := c.regionalClients[key]
	if !exists {
		if c.buildClientFunc == nil {
			return zero, fmt.Errorf("could not get client for region %s, because buildClientFunc is not provided", region)
		}
		c.regionalClients[key] = c.buildClientFunc(*regionalConfig)
	}

	return c.regionalClients[key], nil
}

func (c *Clients[T]) GetClient(region string, role model.Role) T {
	var zero T
	c.clientsM.RLock()
	defer c.clientsM.RUnlock()

	key := c.getClientKey(region, role)
	client, ok := c.regionalClients[key]
	if !ok {
		return zero
	}
	return client
}

func (c *Clients[T]) getClientKey(region string, role model.Role) string {
	return region + separator + role.RoleArn + separator + role.ExternalID
}
