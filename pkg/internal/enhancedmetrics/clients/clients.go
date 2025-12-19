package clients

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// separator is NULL byte (0x00)
const separator = "\x00"

// Clients manages AWS service clients per region and role.
// It is used by enhanced metrics services and allows them to initialize a client only once per region and role and then reuse it.
type Clients[T any] struct {
	regionalClients map[string]T
	clientsM        sync.RWMutex

	// buildClientFunc is a function that builds a client of type T given an AWS config.
	buildClientFunc func(cfg aws.Config) T
}

// NewClients creates a new Clients instance.
func NewClients[T any](buildClientFunc func(cfg aws.Config) T) *Clients[T] {
	return &Clients[T]{
		buildClientFunc: buildClientFunc,
		regionalClients: make(map[string]T),
	}
}

// InitializeClient initializes and stores a client for the given region and role. It should be invoked before GetClient.
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

// GetClient retrieves a previously initialized client for the given region and role.
// If no client was initialized for the given region and role, it returns the zero value of Client type.
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

// getClientKey generates a unique key for the given region and role.
func (c *Clients[T]) getClientKey(region string, role model.Role) string {
	return region + separator + role.RoleArn + separator + role.ExternalID
}
