package elasticache

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/cache"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	DescribeAllCacheClusters(ctx context.Context, logger *slog.Logger) ([]types.CacheCluster, error)
}

type buildElastiCacheMetricFunc func(context.Context, *slog.Logger, *model.TaggedResource, *types.CacheCluster, []string) (*model.CloudwatchData, error)

type ElastiCache struct {
	regionalClients     map[string]Client
	clientsM            sync.RWMutex
	clientFactoryMethod func(cfg aws.Config) Client

	regionalData map[string]*types.CacheCluster

	// dataM protects access to regionalData, for the concurrent metric processing
	dataM sync.RWMutex

	supportedMetrics map[string]buildElastiCacheMetricFunc
}

func NewElastiCacheService(clientFactoryMethod func(cfg aws.Config) Client) *ElastiCache {
	if clientFactoryMethod == nil {
		clientFactoryMethod = NewElastiCacheClientWithConfig
	}
	svc := &ElastiCache{
		clientFactoryMethod: clientFactoryMethod,
		regionalClients:     make(map[string]Client),
		regionalData:        make(map[string]*types.CacheCluster),
	}

	svc.supportedMetrics = map[string]buildElastiCacheMetricFunc{
		"NumCacheNodes": svc.buildNumCacheNodesMetric,
	}

	return svc
}

func (s *ElastiCache) GetNamespace() string {
	return "AWS/ElastiCache"
}

func (s *ElastiCache) initializeClient(_ context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) (Client, error) {
	regionalConfig := configProvider.GetAWSRegionalConfig(region, role)
	if regionalConfig == nil {
		return nil, fmt.Errorf("could not get AWS config for region %s", region)
	}

	s.clientsM.Lock()
	defer s.clientsM.Unlock()

	key := cache.GetClientKey(region, role)
	if s.regionalClients == nil {
		s.regionalClients = make(map[string]Client)
	}
	_, exists := s.regionalClients[key]
	if !exists {
		s.regionalClients[key] = s.clientFactoryMethod(*regionalConfig)
	}

	return s.regionalClients[key], nil
}

func (s *ElastiCache) LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) error {
	var err error
	client := s.getClient(region, role)
	if client == nil {
		client, err = s.initializeClient(ctx, logger, region, role, configProvider)
		if err != nil {
			return fmt.Errorf("error initializing ElastiCache client for region %s: %w", region, err)
		}
	}

	s.dataM.Lock()
	defer s.dataM.Unlock()

	if s.regionalData != nil {
		return nil
	}

	s.regionalData = make(map[string]*types.CacheCluster)

	instances, err := client.DescribeAllCacheClusters(ctx, logger)
	if err != nil {
		return fmt.Errorf("error listing cache clusters in region %s: %w", region, err)
	}

	for _, instance := range instances {
		s.regionalData[*instance.ARN] = &instance
	}

	return nil
}

func (s *ElastiCache) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *ElastiCache) Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, enhancedMetrics []*model.EnhancedMetricConfig, exportedTags []string) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetrics) == 0 {
		return nil, nil
	}

	if namespace != s.GetNamespace() {
		return nil, fmt.Errorf("elasticache enhanced metrics service cannot process namespace %s", namespace)
	}

	if s.regionalData == nil {
		logger.Info("elasticache metadata not loaded, skipping metric processing")
		return nil, nil
	}

	var result []*model.CloudwatchData
	s.dataM.RLock()
	defer s.dataM.RUnlock()

	for _, resource := range resources {
		cluster, exists := s.regionalData[resource.ARN]
		if !exists {
			logger.Warn("elasticache cluster not found in data", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetrics {
			if !s.isMetricSupported(enhancedMetric.Name) {
				logger.Warn("elasticache enhanced metric not supported", "metric", enhancedMetric.Name)
				continue
			}
			em, err := s.supportedMetrics[enhancedMetric.Name](ctx, logger, resource, cluster, exportedTags)
			if err != nil {
				logger.Warn("Error building elasticache enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *ElastiCache) getClient(region string, role model.Role) (client Client) {
	s.clientsM.RLock()
	defer s.clientsM.RUnlock()

	key := cache.GetClientKey(region, role)
	client, ok := s.regionalClients[key]
	if !ok {
		return nil
	}
	return client
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
			Statistic: "Sum",
			DataPoints: []model.DataPoint{
				{
					Value:     &value,
					Timestamp: time.Now(),
				},
			},
		},
	}, nil
}
