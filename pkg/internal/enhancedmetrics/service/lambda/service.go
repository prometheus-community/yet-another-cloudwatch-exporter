// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package lambda

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	ListAllFunctions(ctx context.Context, logger *slog.Logger) ([]types.FunctionConfiguration, error)
}

type buildLambdaMetricFunc func(*model.TaggedResource, *types.FunctionConfiguration, []string) (*model.CloudwatchData, error)

type Lambda struct {
	supportedMetrics map[string]buildLambdaMetricFunc
	buildClientFunc  func(cfg aws.Config) Client
}

func NewLambdaService(buildClientFunc func(cfg aws.Config) Client) *Lambda {
	if buildClientFunc == nil {
		buildClientFunc = NewLambdaClientWithConfig
	}
	svc := &Lambda{
		buildClientFunc: buildClientFunc,
	}

	svc.supportedMetrics = map[string]buildLambdaMetricFunc{
		// The maximum execution duration permitted for the function before termination.
		"Timeout": buildTimeoutMetric,
	}

	return svc
}

func (s *Lambda) GetNamespace() string {
	return "AWS/Lambda"
}

func (s *Lambda) loadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) (map[string]*types.FunctionConfiguration, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	instances, err := client.ListAllFunctions(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("error listing functions in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.FunctionConfiguration, len(instances))
	for _, instance := range instances {
		regionalData[*instance.FunctionArn] = &instance
	}

	logger.Info("Loaded Lambda metrics metadata", "region", region)
	return regionalData, nil
}

func (s *Lambda) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *Lambda) GetMetrics(ctx context.Context,
	logger *slog.Logger,
	namespace string,
	resources []*model.TaggedResource,
	enhancedMetricConfigs []*model.EnhancedMetricConfig,
	exportedTagOnMetrics []string,
	region string,
	role model.Role,
	regionalConfigProvider config.RegionalConfigProvider,
) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetricConfigs) == 0 {
		return nil, nil
	}

	if namespace != s.GetNamespace() {
		return nil, fmt.Errorf("lambda enhanced metrics service cannot process namespace %s", namespace)
	}

	// filter only supported enhanced metrics
	var enhancedMetricsFiltered []*model.EnhancedMetricConfig
	for _, em := range enhancedMetricConfigs {
		if s.isMetricSupported(em.Name) {
			enhancedMetricsFiltered = append(enhancedMetricsFiltered, em)
		} else {
			logger.Warn("enhanced metric not supported, skipping", "metric", em.Name)
		}
	}

	if len(enhancedMetricsFiltered) == 0 {
		return nil, nil
	}

	data, err := s.loadMetricsMetadata(
		ctx,
		logger,
		region,
		role,
		regionalConfigProvider,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading lambda metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, resource := range resources {
		fn, exists := data[resource.ARN]
		if !exists {
			logger.Warn("Lambda function not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricsFiltered {
			em, err := s.supportedMetrics[enhancedMetric.Name](resource, fn, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building Lambda enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *Lambda) ListRequiredPermissions() []string {
	return []string{
		"lambda:ListFunctions",
	}
}

func (s *Lambda) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *Lambda) Instance() service.EnhancedMetricsService {
	return NewLambdaService(s.buildClientFunc)
}

func buildTimeoutMetric(resource *model.TaggedResource, fn *types.FunctionConfiguration, exportedTags []string) (*model.CloudwatchData, error) {
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
