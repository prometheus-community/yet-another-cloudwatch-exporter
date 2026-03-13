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
package quotametrics

import (
	"fmt"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/service/ec2"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/service/elasticache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/service/s3"
)

// DefaultQuotaMetricServiceRegistry is the default registry containing all built-in quota metrics services.
var DefaultQuotaMetricServiceRegistry = (&Registry{}).
	Register(ec2.NewEC2Service(nil)).
	Register(elasticache.NewElastiCacheService(nil)).
	Register(s3.NewS3Service(nil))

// Registry maintains a mapping of quota metrics services by their namespaces.
type Registry struct {
	m        sync.RWMutex
	services map[string]service.QuotaMetricsService
}

// Register adds a new quota metrics service to the registry.
func (r *Registry) Register(svc service.QuotaMetricsService) *Registry {
	r.m.Lock()
	defer r.m.Unlock()

	if r.services == nil {
		r.services = make(map[string]service.QuotaMetricsService)
	}
	r.services[svc.GetNamespace()] = svc

	return r
}

// GetService retrieves a quota metrics service by its namespace.
func (r *Registry) GetService(namespace string) (service.QuotaMetricsService, error) {
	r.m.RLock()
	defer r.m.RUnlock()

	if svc, exists := r.services[namespace]; exists {
		return svc, nil
	}

	return nil, fmt.Errorf("quota metrics service for namespace %s not found", namespace)
}

// IsNamespaceSupported returns true if the namespace has a registered quota metrics service.
func (r *Registry) IsNamespaceSupported(namespace string) bool {
	r.m.RLock()
	defer r.m.RUnlock()

	_, exists := r.services[namespace]
	return exists
}
