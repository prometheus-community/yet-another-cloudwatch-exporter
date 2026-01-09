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
package rds

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const awsRdsNamespace = "AWS/RDS"

type Client interface {
	DescribeAllDBInstances(ctx context.Context, logger *slog.Logger) ([]types.DBInstance, error)
}

type buildRDSMetricFunc func(*model.TaggedResource, *types.DBInstance, []string) (*model.CloudwatchData, error)

type RDS struct {
	supportedMetrics map[string]buildRDSMetricFunc
	buildClientFunc  func(cfg aws.Config) Client
}

func NewRDSService(buildClientFunc func(cfg aws.Config) Client) *RDS {
	if buildClientFunc == nil {
		buildClientFunc = NewRDSClientWithConfig
	}

	rds := &RDS{
		buildClientFunc: buildClientFunc,
	}

	rds.supportedMetrics = map[string]buildRDSMetricFunc{
		// The storage capacity in gibibytes (GiB) allocated for the DB instance.
		"AllocatedStorage": buildAllocatedStorageMetric,
	}

	return rds
}

// GetNamespace returns the AWS CloudWatch namespace for RDS
func (s *RDS) GetNamespace() string {
	return awsRdsNamespace
}

// loadMetricsMetadata loads any metadata needed for RDS enhanced metrics for the given region and role
func (s *RDS) loadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) (map[string]*types.DBInstance, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	instances, err := client.DescribeAllDBInstances(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("error describing RDS DB instances in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.DBInstance, len(instances))

	for _, instance := range instances {
		regionalData[*instance.DBInstanceArn] = &instance
	}

	return regionalData, nil
}

func (s *RDS) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *RDS) GetMetrics(ctx context.Context,
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
		return nil, fmt.Errorf("RDS enhanced metrics service cannot process namespace %s", namespace)
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
		return nil, fmt.Errorf("error loading RDS metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, resource := range resources {
		dbInstance, exists := data[resource.ARN]
		if !exists {
			logger.Warn("RDS DB instance not found in metadata", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricsFiltered {
			em, err := s.supportedMetrics[enhancedMetric.Name](resource, dbInstance, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building RDS enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *RDS) ListRequiredPermissions() []string {
	return []string{
		"rds:DescribeDBInstances",
	}
}

func (s *RDS) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *RDS) Instance() service.EnhancedMetricsService {
	return NewRDSService(s.buildClientFunc)
}

func buildAllocatedStorageMetric(resource *model.TaggedResource, instance *types.DBInstance, exportedTags []string) (*model.CloudwatchData, error) {
	if instance.AllocatedStorage == nil {
		return nil, fmt.Errorf("AllocatedStorage is nil for DB instance %s", resource.ARN)
	}

	var dimensions []model.Dimension

	if instance.DBInstanceIdentifier != nil && len(*instance.DBInstanceIdentifier) > 0 {
		dimensions = append(dimensions, model.Dimension{
			Name:  "DBInstanceIdentifier",
			Value: *instance.DBInstanceIdentifier,
		})
	}

	if instance.DBInstanceClass != nil && len(*instance.DBInstanceClass) > 0 {
		dimensions = append(dimensions, model.Dimension{
			Name:  "DatabaseClass",
			Value: *instance.DBInstanceClass,
		})
	}

	if instance.Engine != nil && len(*instance.Engine) > 0 {
		dimensions = append(dimensions, model.Dimension{
			Name:  "EngineName",
			Value: *instance.Engine,
		})
	}

	// Convert from GiB to bytes
	valueInBytes := float64(*instance.AllocatedStorage) * 1024 * 1024 * 1024

	return &model.CloudwatchData{
		MetricName:   "AllocatedStorage",
		ResourceName: resource.ARN,
		Namespace:    awsRdsNamespace,
		Dimensions:   dimensions,
		Tags:         resource.MetricTags(exportedTags),

		// Store the value as a single data point
		GetMetricDataResult: &model.GetMetricDataResult{
			DataPoints: []model.DataPoint{
				{
					Value:     &valueInBytes,
					Timestamp: time.Now(),
				},
			},
		},
	}, nil
}
