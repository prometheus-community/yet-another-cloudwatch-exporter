package enhancedmetrics

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/dynamodb"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/elasticache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/lambda"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
)

var DefaultRegistry = &Registry{}

type Registry struct {
	buildClientFunctions map[string]any

	m sync.RWMutex
}

// SetCustomBuildClientFunctions sets a custom function to build service clients for a specific namespace.
// It is used for testing purposes.
func (receiver *Registry) SetCustomBuildClientFunctions(namespace string, f any) {
	receiver.m.Lock()
	defer receiver.m.Unlock()

	if receiver.buildClientFunctions == nil {
		receiver.buildClientFunctions = map[string]any{}
	}
	receiver.buildClientFunctions[namespace] = f
}

// DeleteCustomBuildClientFunctions deletes a custom function to build service clients for a specific namespace.
// It is used for testing purposes.
func (receiver *Registry) DeleteCustomBuildClientFunctions(namespace string) {
	receiver.m.Lock()
	defer receiver.m.Unlock()

	delete(receiver.buildClientFunctions, namespace)
}

func (receiver *Registry) GetEnhancedMetricsService(namespace string) (EnhancedMetricsService, error) {
	receiver.m.RLock()
	defer receiver.m.RUnlock()

	bcf, ok := receiver.buildClientFunctions[namespace]
	if !ok {
		bcf = nil
	}

	switch namespace {
	case "AWS/RDS":
		bcf, ok := bcf.(func(cfg aws.Config) rds.Client)
		if !ok {
			bcf = nil
		}
		return rds.NewRDSService(bcf), nil
	case "AWS/Lambda":
		bcf, ok := bcf.(func(cfg aws.Config) lambda.Client)
		if !ok {
			bcf = nil
		}
		return lambda.NewLambdaService(bcf), nil
	case "AWS/ElastiCache":
		bcf, ok := bcf.(func(cfg aws.Config) elasticache.Client)
		if !ok {
			bcf = nil
		}
		return elasticache.NewElastiCacheService(bcf), nil
	case "AWS/DynamoDB":
		bcf, ok := bcf.(func(cfg aws.Config) dynamodb.Client)
		if !ok {
			bcf = nil
		}
		return dynamodb.NewDynamoDBService(bcf), nil
	}

	return nil, fmt.Errorf("enhanced metrics service for namespace %s not found", namespace)
}
