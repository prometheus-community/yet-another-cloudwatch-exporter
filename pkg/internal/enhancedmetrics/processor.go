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
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// MetricsServiceRegistry defines an interface to get enhanced metrics services by namespace
type MetricsServiceRegistry interface {
	GetEnhancedMetricsService(namespace string) (service.EnhancedMetricsService, error)
}

// Processor is responsible for processing enhanced metrics using appropriate services. It manages multiple enhanced metrics services for different namespaces.
// It ensures that each service is initialized only once and provides thread-safe access to these services.
// The Processor uses a RegionalConfigProvider to obtain AWS configurations for different regions and roles.
// It is intended to be used in the scope of a single scrape operation.
type Processor struct {
	ConfigProvider          config.RegionalConfigProvider
	EnhancedMetricsServices map[string]service.EnhancedMetricsService
	m                       sync.Mutex
}

// Process processes the enhanced metrics for the specified namespace using the appropriate enhanced metrics service.
func (ep *Processor) Process(
	ctx context.Context,
	logger *slog.Logger,
	namespace string,
	resources []*model.TaggedResource,
	metrics []*model.EnhancedMetricConfig,
	exportedTagOnMetrics []string,
	region string,
	role model.Role,
	enhancedMetricsServiceRegistry MetricsServiceRegistry,
) ([]*model.CloudwatchData, error) {

	svc, err := ep.ensureServiceInitialized(namespace, enhancedMetricsServiceRegistry)
	if err != nil {
		return nil, fmt.Errorf("could not get enhanced metric service for namespace %s: %w", namespace, err)
	}

	return svc.Process(
		ctx,
		logger,
		namespace,
		resources,
		metrics,
		exportedTagOnMetrics,
		region,
		role,
		ep.ConfigProvider,
	)
}

func (ep *Processor) ensureServiceInitialized(namespace string, enhancedMetricsServiceRegistry MetricsServiceRegistry) (service.EnhancedMetricsService, error) {
	ep.m.Lock()
	defer ep.m.Unlock()

	if ep.EnhancedMetricsServices == nil {
		ep.EnhancedMetricsServices = make(map[string]service.EnhancedMetricsService)
	}

	svc, exists := ep.EnhancedMetricsServices[namespace]
	if exists {
		return svc, nil
	}

	svc, err := enhancedMetricsServiceRegistry.GetEnhancedMetricsService(namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get enhanced metric service for namespace %s: %w", namespace, err)
	}

	ep.EnhancedMetricsServices[namespace] = svc
	return ep.EnhancedMetricsServices[namespace], nil
}

func NewProcessor(
	factory clients.Factory,
) (*Processor, error) {
	emf, ok := factory.(config.RegionalConfigProvider)
	if !ok {
		return nil, fmt.Errorf("cannot create enhanced metric processor with a factory type %T", factory)
	}

	return &Processor{
		EnhancedMetricsServices: make(map[string]service.EnhancedMetricsService),
		ConfigProvider:          emf,
	}, nil
}
