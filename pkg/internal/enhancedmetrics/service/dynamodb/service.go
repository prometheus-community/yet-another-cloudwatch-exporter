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

type Client interface {
	DescribeAllTables(ctx context.Context, logger *slog.Logger) ([]types.TableDescription, error)
}

type buildDynamoDBMetricFunc func(*model.TaggedResource, *types.TableDescription, []string) ([]*model.CloudwatchData, error)

type DynamoDB struct {
	supportedMetrics map[string]buildDynamoDBMetricFunc
	buildClientFunc  func(cfg aws.Config) Client
}

func NewDynamoDBService(buildClientFunc func(cfg aws.Config) Client) *DynamoDB {
	if buildClientFunc == nil {
		buildClientFunc = NewDynamoDBClientWithConfig
	}
	svc := &DynamoDB{
		buildClientFunc: buildClientFunc,
	}

	svc.supportedMetrics = map[string]buildDynamoDBMetricFunc{
		// The count of items in the table, updated approximately every six hours; may not reflect recent changes.
		"ItemCount": buildItemCountMetric,
	}

	return svc
}

func (s *DynamoDB) GetNamespace() string {
	return "AWS/DynamoDB"
}

func (s *DynamoDB) loadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) (map[string]*types.TableDescription, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	tables, err := client.DescribeAllTables(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("error listing DynamoDB tables in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.TableDescription, len(tables))

	for _, table := range tables {
		regionalData[*table.TableArn] = &table
	}

	return regionalData, nil
}

func (s *DynamoDB) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *DynamoDB) GetMetrics(
	ctx context.Context,
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
		return nil, fmt.Errorf("dynamodb enhanced metrics service cannot process namespace %s", namespace)
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
		return nil, fmt.Errorf("error loading dynamodb metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, resource := range resources {
		table, exists := data[resource.ARN]
		if !exists {
			logger.Warn("dynamodb table not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricsFiltered {
			em, err := s.supportedMetrics[enhancedMetric.Name](resource, table, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building dynamodb enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em...)
		}
	}

	return result, nil
}

func (s *DynamoDB) ListRequiredPermissions() []string {
	return []string{
		"dynamodb:DescribeTable",
		"dynamodb:ListTables",
	}
}

func (s *DynamoDB) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *DynamoDB) Instance() service.EnhancedMetricsService {
	return NewDynamoDBService(s.buildClientFunc)
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
