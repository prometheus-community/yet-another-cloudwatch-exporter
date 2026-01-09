// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package enhancedmetrics

import (
	"fmt"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/dynamodb"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/elasticache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/lambda"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
)

// DefaultEnhancedMetricServiceRegistry is the default registry containing all built-in enhanced metrics services
// It allows registering additional services if needed, or replacing existing ones, e.g. for testing purposes.
//
// Note:In the future, it can be removed in favor of being injected via dependency injection.
// However, it will require changes in the YACE's API.
var DefaultEnhancedMetricServiceRegistry = (&Registry{}).
	Register(rds.NewRDSService(nil)).
	Register(lambda.NewLambdaService(nil)).
	Register(dynamodb.NewDynamoDBService(nil)).
	Register(elasticache.NewElastiCacheService(nil))

// MetricsService represents an enhanced metrics service with methods to get its instance and namespace.
// Services implementing this interface can be registered in the Registry.
type MetricsService interface {
	Instance() service.EnhancedMetricsService
	GetNamespace() string
}

// Registry maintains a mapping of enhanced metrics services by their namespaces.
type Registry struct {
	m sync.RWMutex

	services map[string]func() service.EnhancedMetricsService
}

// Register adds a new enhanced metrics service to the registry or replaces an existing one with the same namespace.
func (receiver *Registry) Register(t MetricsService) *Registry {
	receiver.m.Lock()
	defer receiver.m.Unlock()

	if receiver.services == nil {
		receiver.services = map[string]func() service.EnhancedMetricsService{}
	}
	receiver.services[t.GetNamespace()] = t.Instance

	return receiver
}

// GetEnhancedMetricsService retrieves an enhanced metrics service by its namespace.
func (receiver *Registry) GetEnhancedMetricsService(namespace string) (service.EnhancedMetricsService, error) {
	receiver.m.RLock()
	defer receiver.m.RUnlock()

	if constructor, exists := receiver.services[namespace]; exists {
		return constructor(), nil
	}

	return nil, fmt.Errorf("enhanced metrics service for namespace %s not found", namespace)
}
