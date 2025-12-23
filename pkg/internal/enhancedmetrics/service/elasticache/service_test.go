package elasticache

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestNewElastiCacheService(t *testing.T) {
	tests := []struct {
		name             string
		buildClientFunc  func(cfg aws.Config) Client
		wantNilClients   bool
		wantMetricsCount int
	}{
		{
			name:            "with nil buildClientFunc",
			buildClientFunc: nil,
		},
		{
			name: "with custom buildClientFunc",
			buildClientFunc: func(_ aws.Config) Client {
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewElastiCacheService(tt.buildClientFunc)
			require.NotNil(t, got)
			require.NotNil(t, got.clients)
			require.Nil(t, got.regionalData)
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["NumCacheNodes"])
		})
	}
}

func TestElastiCache_GetNamespace(t *testing.T) {
	service := NewElastiCacheService(nil)
	expectedNamespace := "AWS/ElastiCache"
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestElastiCache_ListRequiredPermissions(t *testing.T) {
	service := NewElastiCacheService(nil)
	expectedPermissions := []string{
		"elasticache:DescribeCacheClusters",
	}
	require.Equal(t, expectedPermissions, service.ListRequiredPermissions())
}

func TestElastiCache_ListSupportedMetrics(t *testing.T) {
	service := NewElastiCacheService(nil)
	expectedMetrics := []string{
		"NumCacheNodes",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedMetrics())
}

func TestElastiCache_LoadMetricsMetadata(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func() *mockServiceElastiCacheClient
		region         string
		existingData   map[string]*types.CacheCluster
		wantErr        bool
		wantDataLoaded bool
	}{
		{
			name:   "successfully load metadata",
			region: "us-east-1",
			setupMock: func() *mockServiceElastiCacheClient {
				mock := &mockServiceElastiCacheClient{}
				clusterArn := "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"
				clusterID := "test-cluster"
				mock.clusters = []types.CacheCluster{
					{
						ARN:            &clusterArn,
						CacheClusterId: &clusterID,
					},
				}
				return mock
			},
			wantErr:        false,
			wantDataLoaded: true,
		},
		{
			name:   "describe clusters error",
			region: "us-east-1",
			setupMock: func() *mockServiceElastiCacheClient {
				return &mockServiceElastiCacheClient{describeErr: true}
			},
			wantErr:        true,
			wantDataLoaded: false,
		},
		{
			name:   "metadata already loaded - skip loading",
			region: "us-east-1",
			existingData: map[string]*types.CacheCluster{
				"existing-arn": {},
			},
			setupMock: func() *mockServiceElastiCacheClient {
				return &mockServiceElastiCacheClient{}
			},
			wantErr:        false,
			wantDataLoaded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)
			var service *ElastiCache

			if tt.setupMock == nil {
				service = NewElastiCacheService(nil)
			} else {
				service = NewElastiCacheService(func(_ aws.Config) Client {
					return tt.setupMock()
				})
			}

			if tt.existingData != nil {
				service.regionalData = tt.existingData
			}

			mockConfig := &mockConfigProvider{
				c: &aws.Config{
					Region: tt.region,
				},
			}
			err := service.LoadMetricsMetadata(ctx, logger, tt.region, model.Role{}, mockConfig)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.wantDataLoaded {
				require.NotEmpty(t, service.regionalData)
			}
		})
	}
}

func TestElastiCache_Process(t *testing.T) {
	rd := map[string]*types.CacheCluster{
		"arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster": {
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"),
			CacheClusterId:     aws.String("test-cluster"),
			ReplicationGroupId: aws.String("test-replication-group"),
			CacheNodeType:      aws.String("cache.t3.micro"),
			Engine:             aws.String("redis"),
			NumCacheNodes:      aws.Int32(2),
		},
	}

	tests := []struct {
		name                 string
		namespace            string
		resources            []*model.TaggedResource
		enhancedMetrics      []*model.EnhancedMetricConfig
		exportedTagOnMetrics []string
		regionalData         map[string]*types.CacheCluster
		wantErr              bool
		wantResultCount      int
	}{
		{
			name:            "empty resources",
			namespace:       "AWS/ElastiCache",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			namespace:       "AWS/ElastiCache",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			namespace:       "AWS/EC2",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			regionalData:    rd,
			wantErr:         true,
			wantResultCount: 0,
		},
		{
			name:            "metadata not loaded",
			namespace:       "AWS/ElastiCache",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			regionalData:    nil,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "successfully process metric",
			namespace: "AWS/ElastiCache",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 1,
		},
		{
			name:      "resource not found in metadata",
			namespace: "AWS/ElastiCache",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:non-existent"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "unsupported metric",
			namespace: "AWS/ElastiCache",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "multiple resources and metrics",
			namespace: "AWS/ElastiCache",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-1"},
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-2"},
			},
			enhancedMetrics:      []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			exportedTagOnMetrics: []string{"Name"},
			regionalData: map[string]*types.CacheCluster{
				"arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-1": {
					ARN:            aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-1"),
					CacheClusterId: aws.String("test-cluster-1"),
					NumCacheNodes:  aws.Int32(1),
				},
				"arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-2": {
					ARN:            aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-2"),
					CacheClusterId: aws.String("test-cluster-2"),
					NumCacheNodes:  aws.Int32(3),
				},
			},
			wantErr:         false,
			wantResultCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)
			service := NewElastiCacheService(
				func(_ aws.Config) Client {
					return nil
				},
			)
			// we directly set the regionalData for testing
			service.regionalData = tt.regionalData

			result, err := service.Process(ctx, logger, tt.namespace, tt.resources, tt.enhancedMetrics, tt.exportedTagOnMetrics)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, result, tt.wantResultCount)

			if tt.wantResultCount > 0 {
				for _, metric := range result {
					require.NotNil(t, metric)
					require.Equal(t, "AWS/ElastiCache", metric.Namespace)
					require.NotEmpty(t, metric.Dimensions)
					require.NotNil(t, metric.GetMetricDataResult)
					require.Empty(t, metric.GetMetricDataResult.Statistic)
					require.Nil(t, metric.GetMetricStatisticsResult)
				}
			}
		})
	}
}

type mockServiceElastiCacheClient struct {
	clusters    []types.CacheCluster
	describeErr bool
}

func (m *mockServiceElastiCacheClient) DescribeAllCacheClusters(_ context.Context, _ *slog.Logger) ([]types.CacheCluster, error) {
	if m.describeErr {
		return nil, fmt.Errorf("mock describe error")
	}
	return m.clusters, nil
}

type mockConfigProvider struct {
	c *aws.Config
}

func (m *mockConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}
