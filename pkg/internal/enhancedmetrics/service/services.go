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

// MetricsService is the interface that enhanced metrics services should implement to be used by the enhanced metrics processor.
type MetricsService interface {
	// LoadMetricsMetadata should load any metadata needed for the enhanced metrics service. It should be concurrent safe.
	LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) error

	// Process processes the given resources and metrics, returning CloudWatch data points.
	// metrics should be filtered by the implementation to only include metrics supported by this service.
	// It should be concurrent safe.
	Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, metrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string) ([]*model.CloudwatchData, error)
}
