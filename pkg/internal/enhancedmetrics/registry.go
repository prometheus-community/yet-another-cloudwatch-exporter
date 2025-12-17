package enhancedmetrics

import (
	"fmt"
	"sort"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/dynamodb"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/elasticache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/lambda"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
)

var DefaultRegistry = (&Registry{}).
	Register(rds.NewRDSService(nil)).
	Register(lambda.NewLambdaService(nil)).
	Register(elasticache.NewElastiCacheService(nil)).
	Register(dynamodb.NewDynamoDBService(nil))

type Registry struct {
	m sync.RWMutex

	// Map of enhanced metric processors by metric name: Namespace -> EnhancedMetricsService
	enhancedMetricProcessors map[string]EnhancedMetricsService
}

func (receiver *Registry) Register(ems EnhancedMetricsService) *Registry {
	receiver.m.Lock()
	defer receiver.m.Unlock()
	if receiver.enhancedMetricProcessors == nil {
		receiver.enhancedMetricProcessors = make(map[string]EnhancedMetricsService)
	}

	receiver.enhancedMetricProcessors[ems.GetNamespace()] = ems

	return receiver
}

func (receiver *Registry) Remove(namespace string) *Registry {
	receiver.m.Lock()
	defer receiver.m.Unlock()
	delete(receiver.enhancedMetricProcessors, namespace)

	return receiver
}

func (receiver *Registry) GetEnhancedMetricsService(namespace string) (EnhancedMetricsService, error) {
	receiver.m.RLock()
	defer receiver.m.RUnlock()
	ems, ok := receiver.enhancedMetricProcessors[namespace]
	if !ok {
		return nil, fmt.Errorf("enhanced metrics service for namespace %s not found", namespace)
	}
	return ems, nil
}

func (receiver *Registry) ListSupportedMetrics() map[string][]string {
	receiver.m.RLock()
	defer receiver.m.RUnlock()
	res := make(map[string][]string)
	for _, v := range receiver.enhancedMetricProcessors {
		res[v.GetNamespace()] = v.ListSupportedMetrics()
	}

	return res
}

func (receiver *Registry) ListRequiredPermissions() []string {
	receiver.m.RLock()
	defer receiver.m.RUnlock()
	res := make([]string, 0)
	for _, v := range receiver.enhancedMetricProcessors {
		res = append(res, v.ListRequiredPermissions()...)
	}

	sort.Strings(res)

	return res
}
