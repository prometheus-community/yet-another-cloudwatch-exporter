package enhanced_metrics

import (
	"context"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type EnhancedMetricsService interface {
	// GetNamespace returns the AWS CloudWatch namespace for the service
	GetNamespace() string

	// LoadMetricsMetadata should load any metadata needed for the enhanced metrics service. It should be concurrent safe.
	LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) error

	// Process processes the given resources and metrics, returning CloudWatch data points
	Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, metrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string) ([]*model.CloudwatchData, error)
}
