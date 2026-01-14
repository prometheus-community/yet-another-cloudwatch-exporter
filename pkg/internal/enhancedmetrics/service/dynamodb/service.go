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
package dynamodb

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const awsDynamoDBNamespace = "AWS/DynamoDB"

type Client interface {
	// DescribeTables retrieves DynamoDB tables with their descriptions. tables is a list of table ARNs or table names.
	DescribeTables(ctx context.Context, logger *slog.Logger, tables []string) ([]types.TableDescription, error)
}

type buildCloudwatchDataFunc func(*model.TaggedResource, *types.TableDescription, []string) ([]*model.CloudwatchData, error)

type supportedMetric struct {
	name                    string
	buildCloudwatchDataFunc buildCloudwatchDataFunc
	requiredPermissions     []string
}

func (sm *supportedMetric) buildCloudwatchData(resource *model.TaggedResource, table *types.TableDescription, metrics []string) ([]*model.CloudwatchData, error) {
	return sm.buildCloudwatchDataFunc(resource, table, metrics)
}

type DynamoDB struct {
	supportedMetrics map[string]supportedMetric
	buildClientFunc  func(cfg aws.Config) Client
}

func NewDynamoDBService(buildClientFunc func(cfg aws.Config) Client) *DynamoDB {
	if buildClientFunc == nil {
		buildClientFunc = NewDynamoDBClientWithConfig
	}
	svc := &DynamoDB{
		buildClientFunc: buildClientFunc,
	}

	// The count of items in the table, updated approximately every six hours; may not reflect recent changes.
	itemCountMetric := supportedMetric{
		name:                    "ItemCount",
		buildCloudwatchDataFunc: buildItemCountMetric,
		requiredPermissions: []string{
			"dynamodb:DescribeTable",
		},
	}

	svc.supportedMetrics = map[string]supportedMetric{
		itemCountMetric.name: itemCountMetric,
	}

	return svc
}

func (s *DynamoDB) GetNamespace() string {
	return awsDynamoDBNamespace
}

func (s *DynamoDB) loadMetricsMetadata(
	ctx context.Context,
	logger *slog.Logger,
	region string,
	role model.Role,
	configProvider config.RegionalConfigProvider,
	tablesARNs []string,
) (map[string]*types.TableDescription, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	tables, err := client.DescribeTables(ctx, logger, tablesARNs)
	if err != nil {
		return nil, fmt.Errorf("error listing DynamoDB tables in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.TableDescription, len(tables))

	for _, table := range tables {
		regionalData[*table.TableArn] = &table
	}

	return regionalData, nil
}

func (s *DynamoDB) IsMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *DynamoDB) GetMetrics(ctx context.Context, logger *slog.Logger, resources []*model.TaggedResource, enhancedMetricConfigs []*model.EnhancedMetricConfig, exportedTagOnMetrics []string, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetricConfigs) == 0 {
		return nil, nil
	}

	tablesARNs := make([]string, 0, len(resources))
	for _, resource := range resources {
		tablesARNs = append(tablesARNs, resource.ARN)
	}

	data, err := s.loadMetricsMetadata(
		ctx,
		logger,
		region,
		role,
		regionalConfigProvider,
		tablesARNs,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading DynamoDB metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, resource := range resources {
		if resource.Namespace != s.GetNamespace() {
			logger.Warn("Resource namespace does not match DynamoDB namespace, skipping", "arn", resource.ARN, "namespace", resource.Namespace)
			continue
		}

		table, exists := data[resource.ARN]
		if !exists {
			logger.Warn("DynamoDB table not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricConfigs {
			supportedMetric, ok := s.supportedMetrics[enhancedMetric.Name]
			if !ok {
				logger.Warn("Unsupported DynamoDB enhanced metric, skipping", "metric", enhancedMetric.Name)
				continue
			}

			em, err := supportedMetric.buildCloudwatchData(resource, table, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building DynamoDB enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em...)
		}
	}

	return result, nil
}

func (s *DynamoDB) ListRequiredPermissions() map[string][]string {
	permissions := make(map[string][]string, len(s.supportedMetrics))
	for _, metric := range s.supportedMetrics {
		permissions[metric.name] = metric.requiredPermissions
	}
	return permissions
}

func (s *DynamoDB) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *DynamoDB) Instance() service.EnhancedMetricsService {
	// do not use NewDynamoDBService to avoid extra map allocation
	return &DynamoDB{
		supportedMetrics: s.supportedMetrics,
		buildClientFunc:  s.buildClientFunc,
	}
}

func buildItemCountMetric(resource *model.TaggedResource, table *types.TableDescription, exportedTags []string) ([]*model.CloudwatchData, error) {
	if table.ItemCount == nil {
		return nil, fmt.Errorf("ItemCount is nil for DynamoDB table %s", resource.ARN)
	}

	var dimensions []model.Dimension

	if table.TableName != nil {
		dimensions = []model.Dimension{
			{Name: "TableName", Value: *table.TableName},
		}
	}

	value := float64(*table.ItemCount)
	result := []*model.CloudwatchData{{
		MetricName:   "ItemCount",
		ResourceName: resource.ARN,
		Namespace:    "AWS/DynamoDB",
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
	}}

	if len(table.GlobalSecondaryIndexes) > 0 {
		for _, globalSecondaryIndex := range table.GlobalSecondaryIndexes {
			if globalSecondaryIndex.ItemCount == nil || globalSecondaryIndex.IndexName == nil {
				continue
			}

			var secondaryIndexesDimensions []model.Dimension
			globalSecondaryIndexesItemsCount := float64(*globalSecondaryIndex.ItemCount)

			if table.TableName != nil {
				secondaryIndexesDimensions = append(secondaryIndexesDimensions, model.Dimension{
					Name:  "TableName",
					Value: *table.TableName,
				})
			}

			if globalSecondaryIndex.IndexName != nil {
				secondaryIndexesDimensions = append(secondaryIndexesDimensions, model.Dimension{
					Name:  "GlobalSecondaryIndexName",
					Value: *globalSecondaryIndex.IndexName,
				})
			}

			result = append(result, &model.CloudwatchData{
				MetricName:   "ItemCount",
				ResourceName: resource.ARN,
				Namespace:    "AWS/DynamoDB",
				Dimensions:   secondaryIndexesDimensions,
				Tags:         resource.MetricTags(exportedTags),
				GetMetricDataResult: &model.GetMetricDataResult{
					DataPoints: []model.DataPoint{
						{
							Value:     &globalSecondaryIndexesItemsCount,
							Timestamp: time.Now(),
						},
					},
				},
			})
		}
	}

	return result, nil
}
