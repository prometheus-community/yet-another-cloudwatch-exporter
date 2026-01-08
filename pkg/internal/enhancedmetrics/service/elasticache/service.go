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
package elasticache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	DescribeAllCacheClusters(ctx context.Context, logger *slog.Logger) ([]types.CacheCluster, error)
}

type buildElastiCacheMetricFunc func(context.Context, *slog.Logger, *model.TaggedResource, *types.CacheCluster, []string) (*model.CloudwatchData, error)

type ElastiCache struct {
	supportedMetrics map[string]buildElastiCacheMetricFunc
	buildClientFunc  func(cfg aws.Config) Client
}

func NewElastiCacheService(buildClientFunc func(cfg aws.Config) Client) *ElastiCache {
	if buildClientFunc == nil {
		buildClientFunc = NewElastiCacheClientWithConfig
	}
	svc := &ElastiCache{
		buildClientFunc: buildClientFunc,
	}

	svc.supportedMetrics = map[string]buildElastiCacheMetricFunc{
		// The count of cache nodes in the cluster; must be 1 for Valkey or Redis OSS clusters, or between 1 and 40 for Memcached clusters.
		"NumCacheNodes": svc.buildNumCacheNodesMetric,
	}

	return svc
}

func (s *ElastiCache) GetNamespace() string {
	return "AWS/ElastiCache"
}

func (s *ElastiCache) loadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) (map[string]*types.CacheCluster, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	instances, err := client.DescribeAllCacheClusters(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("error listing cache clusters in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.CacheCluster, len(instances))

	for _, instance := range instances {
		regionalData[*instance.ARN] = &instance
	}

	return regionalData, nil
}

func (s *ElastiCache) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *ElastiCache) GetMetrics(ctx context.Context,
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
		return nil, fmt.Errorf("elasticache enhanced metrics service cannot process namespace %s", namespace)
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
		return nil, fmt.Errorf("couldn't load elasticache metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, resource := range resources {
		cluster, exists := data[resource.ARN]
		if !exists {
			logger.Warn("elasticache cluster not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricsFiltered {
			em, err := s.supportedMetrics[enhancedMetric.Name](ctx, logger, resource, cluster, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building elasticache enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *ElastiCache) buildNumCacheNodesMetric(_ context.Context, _ *slog.Logger, resource *model.TaggedResource, cluster *types.CacheCluster, exportedTags []string) (*model.CloudwatchData, error) {
	if cluster.NumCacheNodes == nil {
		return nil, fmt.Errorf("NumCacheNodes is nil for ElastiCache cluster %s", resource.ARN)
	}

	var dimensions []model.Dimension

	if cluster.CacheClusterId != nil {
		dimensions = []model.Dimension{
			{Name: "CacheClusterId", Value: *cluster.CacheClusterId},
		}
	}

	if cluster.ReplicationGroupId != nil {
		dimensions = append(dimensions, model.Dimension{
			Name:  "ReplicationGroupId",
			Value: *cluster.ReplicationGroupId,
		})
	}

	value := float64(*cluster.NumCacheNodes)
	return &model.CloudwatchData{
		MetricName:   "NumCacheNodes",
		ResourceName: resource.ARN,
		Namespace:    "AWS/ElastiCache",
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

func (s *ElastiCache) ListRequiredPermissions() []string {
	return []string{
		"elasticache:DescribeCacheClusters",
	}
}

func (s *ElastiCache) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *ElastiCache) Instance() service.EnhancedMetricsService {
	return NewElastiCacheService(s.buildClientFunc)
}
