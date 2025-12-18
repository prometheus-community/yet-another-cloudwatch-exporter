package lambda

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	ListAllFunctions(ctx context.Context, logger *slog.Logger) ([]types.FunctionConfiguration, error)
}

type buildLambdaMetricFunc func(context.Context, *slog.Logger, *model.TaggedResource, *types.FunctionConfiguration, []string) (*model.CloudwatchData, error)

type Lambda struct {
	clients *clients.Clients[Client]

	regionalData map[string]*types.FunctionConfiguration

	// dataM protects access to regionalData, for the concurrent metric processing
	dataM sync.RWMutex

	supportedMetrics map[string]buildLambdaMetricFunc
}

func NewLambdaService(buildClientFunc func(cfg aws.Config) Client) *Lambda {
	if buildClientFunc == nil {
		buildClientFunc = NewLambdaClientWithConfig
	}
	svc := &Lambda{
		clients: clients.NewClients[Client](buildClientFunc),
	}

	svc.supportedMetrics = map[string]buildLambdaMetricFunc{
		"Timeout": svc.buildTimeoutMetric,
	}

	return svc
}

func (s *Lambda) GetNamespace() string {
	return "AWS/Lambda"
}

func (s *Lambda) LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) error {
	var err error
	client := s.clients.GetClient(region, role)
	if client == nil {
		client, err = s.clients.InitializeClient(region, role, configProvider)
		if err != nil {
			return fmt.Errorf("error initializing Lambda client for region %s: %w", region, err)
		}
	}

	s.dataM.Lock()
	defer s.dataM.Unlock()

	if s.regionalData != nil {
		return nil
	}

	s.regionalData = make(map[string]*types.FunctionConfiguration)

	instances, err := client.ListAllFunctions(ctx, logger)
	if err != nil {
		return fmt.Errorf("error listing functions in region %s: %w", region, err)
	}

	for _, instance := range instances {
		s.regionalData[*instance.FunctionArn] = &instance
	}

	return nil
}

func (s *Lambda) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *Lambda) Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, enhancedMetrics []*model.EnhancedMetricConfig, exportedTags []string) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetrics) == 0 {
		return nil, nil
	}

	if namespace != s.GetNamespace() {
		return nil, fmt.Errorf("lambda enhanced metrics service cannot process namespace %s", namespace)
	}

	if s.regionalData == nil {
		logger.Info("Lambda metadata not loaded, skipping metric processing")
		return nil, nil
	}

	var result []*model.CloudwatchData
	s.dataM.RLock()
	defer s.dataM.RUnlock()

	for _, resource := range resources {
		fn, exists := s.regionalData[resource.ARN]
		if !exists {
			logger.Warn("Lambda function not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetrics {
			if !s.isMetricSupported(enhancedMetric.Name) {
				logger.Warn("Lambda enhanced metric not supported", "metric", enhancedMetric.Name)
				continue
			}
			em, err := s.supportedMetrics[enhancedMetric.Name](ctx, logger, resource, fn, exportedTags)
			if err != nil {
				logger.Warn("Error building Lambda enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *Lambda) buildTimeoutMetric(_ context.Context, _ *slog.Logger, resource *model.TaggedResource, fn *types.FunctionConfiguration, exportedTags []string) (*model.CloudwatchData, error) {
	if fn.Timeout == nil {
		return nil, fmt.Errorf("timeout is nil for Lambda function %s", resource.ARN)
	}

	var dimensions []model.Dimension

	if fn.FunctionName != nil {
		dimensions = []model.Dimension{
			{Name: "FunctionName", Value: *fn.FunctionName},
		}
	}

	value := float64(*fn.Timeout)
	return &model.CloudwatchData{
		MetricName:   "Timeout",
		ResourceName: resource.ARN,
		Namespace:    "AWS/Lambda",
		Dimensions:   dimensions,
		Tags:         resource.MetricTags(exportedTags),
		GetMetricDataResult: &model.GetMetricDataResult{
			DataPoints: []model.DataPoint{
				{
					Value:     &value,
					Timestamp: time.Now(),
				},
			},
		},
	}, nil
}

func (s *Lambda) ListRequiredPermissions() []string {
	return []string{
		"lambda:ListFunctions",
	}
}

func (s *Lambda) ListSupportedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}
