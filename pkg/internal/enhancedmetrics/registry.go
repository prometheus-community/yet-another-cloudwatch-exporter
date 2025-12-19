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

type MetricsService interface {
	Instance() service.MetricsService
	GetNamespace() string
}
type Registry struct {
	m sync.RWMutex

	templates map[string]func() service.MetricsService
}

func (receiver *Registry) Register(t MetricsService) *Registry {
	receiver.m.Lock()
	defer receiver.m.Unlock()

	if receiver.templates == nil {
		receiver.templates = map[string]func() service.MetricsService{}
	}
	receiver.templates[t.GetNamespace()] = t.Instance

	return receiver
}

func (receiver *Registry) GetEnhancedMetricsService(namespace string) (service.MetricsService, error) {
	receiver.m.RLock()
	defer receiver.m.RUnlock()

	if constructor, exists := receiver.templates[namespace]; exists {
		return constructor(), nil
	}

	return nil, fmt.Errorf("enhanced metrics service for namespace %s not found", namespace)
}
