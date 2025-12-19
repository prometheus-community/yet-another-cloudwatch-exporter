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

type MetricsServiceRegistry interface {
	GetEnhancedMetricsService(namespace string) (service.EnhancedMetricsService, error)
}

type Processor struct {
	ConfigProvider          config.RegionalConfigProvider
	EnhancedMetricsServices map[string]service.EnhancedMetricsService
	m                       sync.RWMutex
}

func (ep *Processor) ensureServiceInitialized(namespace string, enhancedMetricsServiceRegistry MetricsServiceRegistry) (bool, error) {
	ep.m.Lock()
	defer ep.m.Unlock()
	if ep.EnhancedMetricsServices == nil {
		ep.EnhancedMetricsServices = make(map[string]service.EnhancedMetricsService)
	}

	_, exists := ep.EnhancedMetricsServices[namespace]
	if exists {
		return false, nil
	}

	svc, err := enhancedMetricsServiceRegistry.GetEnhancedMetricsService(namespace)
	if err != nil {
		return false, fmt.Errorf("could not get enhanced metric service for namespace %s: %w", namespace, err)
	}

	ep.EnhancedMetricsServices[namespace] = svc
	return true, nil
}

func (ep *Processor) LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, namespace string, enhancedMetricsServiceRegistry MetricsServiceRegistry) error {
	wasInitialized, err := ep.ensureServiceInitialized(namespace, enhancedMetricsServiceRegistry)
	if err != nil {
		return fmt.Errorf("Processor LoadMetricsMetadata ensureServiceInitialized error: %w", err)
	}

	logger.Debug("Enhanced metrics service was initialized before", "yes", !wasInitialized)

	ep.m.RLock()
	defer ep.m.RUnlock()

	svc, ok := ep.EnhancedMetricsServices[namespace]
	if !ok {
		// should not happen because of ensureServiceInitialized
		return fmt.Errorf("enhanced metrics service for namespace %s not initialized", namespace)
	}

	return svc.LoadMetricsMetadata(ctx, logger, region, role, ep.ConfigProvider)
}

func (ep *Processor) Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, metrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string) ([]*model.CloudwatchData, error) {
	ep.m.RLock()
	defer ep.m.RUnlock()

	svc, ok := ep.EnhancedMetricsServices[namespace]
	if !ok {
		return nil, fmt.Errorf("enhanced metrics service for namespace %s not initialized", namespace)
	}

	return svc.Process(ctx, logger, namespace, resources, metrics, exportedTagOnMetrics)
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
