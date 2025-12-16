package dynamodb

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/cache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	DescribeAllTables(ctx context.Context, logger *slog.Logger) ([]types.TableDescription, error)
}

type buildDynamoDBMetricFunc func(context.Context, *slog.Logger, *model.TaggedResource, *types.TableDescription, []string) ([]*model.CloudwatchData, error)

type DynamoDB struct {
	clients *cache.Clients[Client]

	regionalData map[string]*types.TableDescription
	dataM        sync.RWMutex

	supportedMetrics map[string]buildDynamoDBMetricFunc
}

func NewDynamoDBService(buildClientFunc func(cfg aws.Config) Client) *DynamoDB {
	if buildClientFunc == nil {
		buildClientFunc = NewDynamoDBClientWithConfig
	}
	svc := &DynamoDB{
		clients:      cache.NewClients[Client](buildClientFunc),
		regionalData: make(map[string]*types.TableDescription),
	}

	svc.supportedMetrics = map[string]buildDynamoDBMetricFunc{
		"ItemCount": svc.buildItemCountMetric,
	}

	return svc
}

func (s *DynamoDB) GetNamespace() string {
	return "AWS/DynamoDB"
}

func (s *DynamoDB) LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) error {
	var err error
	client := s.clients.GetClient(region, role)
	if client == nil {
		client, err = s.clients.InitializeClient(ctx, logger, region, role, configProvider)
		if err != nil {
			return fmt.Errorf("error initializing DynamoDB client for region %s: %w", region, err)
		}
	}

	s.dataM.Lock()
	defer s.dataM.Unlock()

	if s.regionalData != nil {
		return nil
	}

	s.regionalData = make(map[string]*types.TableDescription)

	tables, err := client.DescribeAllTables(ctx, logger)
	if err != nil {
		return fmt.Errorf("error listing DynamoDB tables in region %s: %w", region, err)
	}

	for _, table := range tables {
		s.regionalData[*table.TableArn] = &table
	}

	return nil
}

func (s *DynamoDB) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *DynamoDB) Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, enhancedMetrics []*model.EnhancedMetricConfig, exportedTags []string) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetrics) == 0 {
		return nil, nil
	}

	if namespace != s.GetNamespace() {
		return nil, fmt.Errorf("dynamodb enhanced metrics service cannot process namespace %s", namespace)
	}

	if s.regionalData == nil {
		logger.Info("dynamodb metadata not loaded, skipping metric processing")
		return nil, nil
	}

	var result []*model.CloudwatchData
	s.dataM.RLock()
	defer s.dataM.RUnlock()

	for _, resource := range resources {
		table, exists := s.regionalData[resource.ARN]
		if !exists {
			logger.Warn("dynamodb table not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetrics {
			if !s.isMetricSupported(enhancedMetric.Name) {
				logger.Warn("dynamodb enhanced metric not supported", "metric", enhancedMetric.Name)
				continue
			}
			em, err := s.supportedMetrics[enhancedMetric.Name](ctx, logger, resource, table, exportedTags)
			if err != nil {
				logger.Warn("Error building dynamodb enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em...)
		}
	}

	return result, nil
}

func (s *DynamoDB) buildItemCountMetric(_ context.Context, _ *slog.Logger, resource *model.TaggedResource, table *types.TableDescription, exportedTags []string) ([]*model.CloudwatchData, error) {
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
			Statistic: "Sum",
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
					Statistic: "Sum",
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
