package enhanced_metrics

import (
	"fmt"
	"sync"
)

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
