package enhancedmetrics

import (
	"fmt"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/dynamodb"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/elasticache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/lambda"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
)

var DefaultRegistry = (&Registry{}).
	Register(rds.NewRDSService(nil)).
	Register(lambda.NewLambdaService(nil)).
	Register(dynamodb.NewDynamoDBService(nil)).
	Register(elasticache.NewElastiCacheService(nil))

type MetricsServiceFactory interface {
	Instance() service.EnhancedMetricsService
	GetNamespace() string
}
type Registry struct {
	m sync.RWMutex

	templates map[string]func() service.EnhancedMetricsService
}

func (receiver *Registry) Register(t MetricsServiceFactory) *Registry {
	receiver.m.Lock()
	defer receiver.m.Unlock()

	if receiver.templates == nil {
		receiver.templates = map[string]func() service.EnhancedMetricsService{}
	}
	receiver.templates[t.GetNamespace()] = t.Instance

	return receiver
}

func (receiver *Registry) GetEnhancedMetricsService(namespace string) (service.EnhancedMetricsService, error) {
	receiver.m.RLock()
	defer receiver.m.RUnlock()

	if constructor, exists := receiver.templates[namespace]; exists {
		return constructor(), nil
	}

	return nil, fmt.Errorf("enhanced metrics service for namespace %s not found", namespace)
}
