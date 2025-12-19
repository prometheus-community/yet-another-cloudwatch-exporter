package service

import (
	"context"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type EnhancedMetricsService interface {
	// LoadMetricsMetadata should load any metadata needed for the enhanced metrics service. It should be concurrent safe.
	LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) error

	// Process processes the given resources and metrics, returning CloudWatch data points
	Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, metrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string) ([]*model.CloudwatchData, error)
}
