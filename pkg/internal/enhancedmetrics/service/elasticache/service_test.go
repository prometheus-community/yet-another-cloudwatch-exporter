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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestNewElastiCacheService(t *testing.T) {
	tests := []struct {
		name            string
		buildClientFunc func(cfg aws.Config) Client
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
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["NumCacheNodes"])
		})
	}
}

func TestElastiCache_GetNamespace(t *testing.T) {
	service := NewElastiCacheService(nil)
	expectedNamespace := awsElastiCacheNamespace
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestElastiCache_ListRequiredPermissions(t *testing.T) {
	service := NewElastiCacheService(nil)
	expectedPermissions := map[string][]string{
		"NumCacheNodes": {"elasticache:DescribeCacheClusters"},
	}
	require.Equal(t, expectedPermissions, service.ListRequiredPermissions())
}

func TestElastiCache_ListSupportedEnhancedMetrics(t *testing.T) {
	service := NewElastiCacheService(nil)
	expectedMetrics := []string{
		"NumCacheNodes",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedEnhancedMetrics())
}

func TestElastiCache_GetMetrics(t *testing.T) {
	// Common test data
	testCluster := types.CacheCluster{
		ARN:            aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"),
		CacheClusterId: aws.String("test-cluster"),
		NumCacheNodes:  aws.Int32(2),
	}

	tests := []struct {
		name            string
		resources       []*model.TaggedResource
		enhancedMetrics []*model.EnhancedMetricConfig
		clusters        []types.CacheCluster
		describeErr     bool
		wantErr         bool
		wantResultCount int
	}{
		{
			name:            "empty resources",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			clusters:        []types.CacheCluster{testCluster},
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			clusters:        []types.CacheCluster{testCluster},
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			wantErr:         false,
		},
		{
			name:            "describe error",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			describeErr:     true,
			wantErr:         true,
		},
		{
			name:            "successfully received metric",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster", Namespace: awsElastiCacheNamespace}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			clusters:        []types.CacheCluster{testCluster},
			wantResultCount: 1,
		},
		{
			name:            "resource not found in metadata",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:non-existent"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			clusters:        []types.CacheCluster{testCluster},
			wantResultCount: 0,
		},
		{
			name:            "unsupported metric",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			clusters:        []types.CacheCluster{testCluster},
			wantResultCount: 0,
		},
		{
			name: "multiple resources and metrics",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-1", Namespace: awsElastiCacheNamespace},
				{ARN: "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-2", Namespace: awsElastiCacheNamespace},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "NumCacheNodes"}},
			clusters: []types.CacheCluster{
				{
					ARN:            aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-1"),
					CacheClusterId: aws.String("test-cluster-1"),
					NumCacheNodes:  aws.Int32(1),
				},
				{
					ARN:            aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster-2"),
					CacheClusterId: aws.String("test-cluster-2"),
					NumCacheNodes:  aws.Int32(3),
				},
			},
			wantResultCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)

			mockClient := &mockServiceElastiCacheClient{
				clusters:    tt.clusters,
				describeErr: tt.describeErr,
			}

			service := NewElastiCacheService(func(_ aws.Config) Client {
				return mockClient
			})

			mockConfig := &mockConfigProvider{
				c: &aws.Config{Region: "us-east-1"},
			}

			result, err := service.GetMetrics(ctx, logger, tt.resources, tt.enhancedMetrics, nil, "us-east-1", model.Role{}, mockConfig)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, result, tt.wantResultCount)

			if tt.wantResultCount > 0 {
				for _, metric := range result {
					require.NotNil(t, metric)
					require.Equal(t, awsElastiCacheNamespace, metric.Namespace)
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
