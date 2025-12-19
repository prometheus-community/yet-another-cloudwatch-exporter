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
