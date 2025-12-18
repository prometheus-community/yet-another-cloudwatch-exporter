package enhancedmetrics

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type EnhancedMetricsServiceRegistry interface {
	GetEnhancedMetricsService(namespace string) (EnhancedMetricsService, error)
}

// todo: do we need to have this type?
type EnhancedProcessor struct {
	EnhancedMetricsService
	ConfigProvider config.RegionalConfigProvider
}

func (ep *EnhancedProcessor) LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role) error {
	if ep.EnhancedMetricsService == nil {
		return errors.New("enhanced metrics service is not initialized")
	}

	return ep.EnhancedMetricsService.LoadMetricsMetadata(ctx, logger, region, role, ep.ConfigProvider)
}

func NewEnhancedProcessor(
	namespace string,
	factory clients.Factory,
	enhancedMetricsServiceRegistry EnhancedMetricsServiceRegistry,
) (*EnhancedProcessor, error) {
	emf, ok := factory.(config.RegionalConfigProvider)
	if !ok {
		return nil, fmt.Errorf("cannot create enhanced metric processor for namespace %s, with a factory type %T", namespace, factory)
	}

	ems, err := enhancedMetricsServiceRegistry.GetEnhancedMetricsService(namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get enhanced metric service for namespace %s: %w", namespace, err)
	}

	return &EnhancedProcessor{
		EnhancedMetricsService: ems,
		ConfigProvider:         emf,
	}, nil
}
