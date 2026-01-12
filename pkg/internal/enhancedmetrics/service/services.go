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
package service

import (
	"context"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type EnhancedMetricsService interface {
	// GetMetrics returns enhanced metrics for the given resources and enhancedMetricConfigs.
	// filteredResources are the resources that belong to the service's namespace.
	// filteredEnhancedMetricConfigs are the enhanced metric configs that belong to the service's namespace and are supported by the service.
	GetMetrics(
		ctx context.Context,
		logger *slog.Logger,
		filteredResources []*model.TaggedResource,
		filteredEnhancedMetricConfigs []*model.EnhancedMetricConfig,
		exportedTagOnMetrics []string,
		region string,
		role model.Role,
		regionalConfigProvider config.RegionalConfigProvider,
	) ([]*model.CloudwatchData, error)

	// IsMetricSupported checks if the given metric name is supported by this service.
	IsMetricSupported(metricName string) bool
}
