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

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// MetricsServiceRegistry defines an interface to get enhanced metrics services by namespace
type MetricsServiceRegistry interface {
	GetEnhancedMetricsService(namespace string) (service.EnhancedMetricsService, error)
}

// Service is responsible for processing enhanced metrics using appropriate services. It manages multiple enhanced metrics services for different namespaces.
// It ensures that each service is initialized only once and provides thread-safe access to these services.
// The Service uses a RegionalConfigProvider to obtain AWS configurations for different regions and roles.
// It is intended to be used in the scope of a single scrape operation.
type Service struct {
	ConfigProvider config.RegionalConfigProvider
}

// GetMetrics processes the enhanced metrics for the specified namespace using the appropriate enhanced metrics service.
func (ep *Service) GetMetrics(
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
	svc, err := enhancedMetricsServiceRegistry.GetEnhancedMetricsService(namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get enhanced metric service for namespace %s: %w", namespace, err)
	}

	// filter out resources that do not match the service's namespace, it should not happen in the current scenario
	var filteredResources []*model.TaggedResource
	for _, res := range resources {
		if res.Namespace == namespace {
			filteredResources = append(filteredResources, res)
		}
	}

	return svc.GetMetrics(
		ctx,
		logger,
		namespace,
		filteredResources,
		metrics,
		exportedTagOnMetrics,
		region,
		role,
		ep.ConfigProvider,
	)
}

func NewService(
	factory clients.Factory,
) (*Service, error) {
	emf, ok := factory.(config.RegionalConfigProvider)
	if !ok {
		return nil, fmt.Errorf("cannot create enhanced metric service with a factory type %T", factory)
	}

	return &Service{
		ConfigProvider: emf,
	}, nil
}
