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

// Service is responsible for getting enhanced metrics using appropriate services.
type Service struct {
	configProvider                 config.RegionalConfigProvider
	enhancedMetricsServiceRegistry MetricsServiceRegistry
}

// GetMetrics returns the enhanced metrics for the specified namespace using the appropriate enhanced metrics service.
func (ep *Service) GetMetrics(
	ctx context.Context,
	logger *slog.Logger,
	namespace string,
	resources []*model.TaggedResource,
	metrics []*model.EnhancedMetricConfig,
	exportedTagOnMetrics []string,
	region string,
	role model.Role,
) ([]*model.CloudwatchData, error) {
	svc, err := ep.enhancedMetricsServiceRegistry.GetEnhancedMetricsService(namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get enhanced metric service for namespace %s: %w", namespace, err)
	}

	// filter out resources that do not match the service's namespace, it should not happen in the current scenario
	var filteredResources []*model.TaggedResource
	for _, res := range resources {
		if res.Namespace == namespace {
			filteredResources = append(filteredResources, res)
		} else {
			// Resource validation should have happened earlier, this log will identify any unexpected issues
			logger.Warn("Skipping resource for enhanced metric service due to namespace mismatch",
				"expected_namespace", namespace,
				"resource_namespace", res.Namespace,
				"resource_arn", res.ARN,
			)
		}
	}

	// filter out metrics that are not supported by the service
	var filteredMetrics []*model.EnhancedMetricConfig
	for _, metric := range metrics {
		if svc.IsMetricSupported(metric.Name) {
			filteredMetrics = append(filteredMetrics, metric)
		} else {
			// Metrics validation should have happened earlier, this log will identify any unexpected issues
			logger.Warn("Skipping unsupported enhanced metric for service",
				"namespace", namespace,
				"metric", metric.Name,
			)
		}
	}

	return svc.GetMetrics(ctx, logger, filteredResources, filteredMetrics, exportedTagOnMetrics, region, role, ep.configProvider)
}

func NewService(
	factory clients.Factory,
	enhancedMetricsServiceRegistry MetricsServiceRegistry,
) (*Service, error) {
	emf, ok := factory.(config.RegionalConfigProvider)
	if !ok {
		return nil, fmt.Errorf("cannot create enhanced metric service with a factory type %T", factory)
	}

	return &Service{
		configProvider:                 emf,
		enhancedMetricsServiceRegistry: enhancedMetricsServiceRegistry,
	}, nil
}
