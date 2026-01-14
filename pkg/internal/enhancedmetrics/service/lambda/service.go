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

const awsLambdaNamespace = "AWS/Lambda"

type Client interface {
	ListAllFunctions(ctx context.Context, logger *slog.Logger) ([]types.FunctionConfiguration, error)
}

type buildCloudwatchDataFunc func(*model.TaggedResource, *types.FunctionConfiguration, []string) (*model.CloudwatchData, error)

type supportedMetric struct {
	name                    string
	buildCloudwatchDataFunc buildCloudwatchDataFunc
	requiredPermissions     []string
}

func (sm *supportedMetric) buildCloudwatchData(resource *model.TaggedResource, functionConfiguration *types.FunctionConfiguration, exportedTagOnMetrics []string) (*model.CloudwatchData, error) {
	return sm.buildCloudwatchDataFunc(resource, functionConfiguration, exportedTagOnMetrics)
}

type Lambda struct {
	supportedMetrics map[string]supportedMetric
	buildClientFunc  func(cfg aws.Config) Client
}

func NewLambdaService(buildClientFunc func(cfg aws.Config) Client) *Lambda {
	if buildClientFunc == nil {
		buildClientFunc = NewLambdaClientWithConfig
	}
	svc := &Lambda{
		buildClientFunc: buildClientFunc,
	}

	// The maximum execution duration permitted for the function before termination.
	timeoutMetric := supportedMetric{
		name:                    "Timeout",
		buildCloudwatchDataFunc: buildTimeoutMetric,
		requiredPermissions:     []string{"lambda:ListFunctions"},
	}

	svc.supportedMetrics = map[string]supportedMetric{
		timeoutMetric.name: timeoutMetric,
	}

	return svc
}

func (s *Lambda) GetNamespace() string {
	return awsLambdaNamespace
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

func (s *Lambda) IsMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *Lambda) GetMetrics(ctx context.Context, logger *slog.Logger, resources []*model.TaggedResource, enhancedMetricConfigs []*model.EnhancedMetricConfig, exportedTagOnMetrics []string, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetricConfigs) == 0 {
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
		if resource.Namespace != s.GetNamespace() {
			logger.Warn("Resource namespace does not match Lambda namespace, skipping", "arn", resource.ARN, "namespace", resource.Namespace)
			continue
		}

		functionConfiguration, exists := data[resource.ARN]
		if !exists {
			logger.Warn("Lambda function not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricConfigs {
			supportedMetric, ok := s.supportedMetrics[enhancedMetric.Name]
			if !ok {
				logger.Warn("Unsupported Lambda enhanced metric, skipping", "metric", enhancedMetric.Name)
				continue
			}

			em, err := supportedMetric.buildCloudwatchData(resource, functionConfiguration, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building Lambda enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *Lambda) ListRequiredPermissions() map[string][]string {
	permissions := make(map[string][]string, len(s.supportedMetrics))
	for _, metric := range s.supportedMetrics {
		permissions[metric.name] = metric.requiredPermissions
	}
	return permissions
}

func (s *Lambda) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *Lambda) Instance() service.EnhancedMetricsService {
	// do not use NewLambdaService to avoid extra map allocation
	return &Lambda{
		supportedMetrics: s.supportedMetrics,
		buildClientFunc:  s.buildClientFunc,
	}
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
