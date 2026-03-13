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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
)

type Client interface {
	DescribeAllCacheClusters(ctx context.Context) ([]types.CacheCluster, error)
	DescribeAllCacheSubnetGroups(ctx context.Context) ([]types.CacheSubnetGroup, error)
	QuotasClient() quotas.Client
}

type awsElastiCacheClient interface {
	DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
	DescribeCacheSubnetGroups(ctx context.Context, params *elasticache.DescribeCacheSubnetGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error)
}

type AWSClient struct {
	ecClient     awsElastiCacheClient
	quotasClient quotas.Client
}

func NewClientWithConfig(cfg aws.Config) Client {
	return &AWSClient{
		ecClient:     elasticache.NewFromConfig(cfg),
		quotasClient: quotas.NewServiceQuotasClient(cfg),
	}
}

func (c *AWSClient) DescribeAllCacheClusters(ctx context.Context) ([]types.CacheCluster, error) {
	var allClusters []types.CacheCluster
	var marker *string
	var maxRecords int32 = 100

	for {
		output, err := c.ecClient.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
			MaxRecords: &maxRecords,
			Marker:     marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe cache clusters: %w", err)
		}

		allClusters = append(allClusters, output.CacheClusters...)

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	return allClusters, nil
}

func (c *AWSClient) DescribeAllCacheSubnetGroups(ctx context.Context) ([]types.CacheSubnetGroup, error) {
	var allGroups []types.CacheSubnetGroup
	var marker *string
	var maxRecords int32 = 100

	for {
		output, err := c.ecClient.DescribeCacheSubnetGroups(ctx, &elasticache.DescribeCacheSubnetGroupsInput{
			MaxRecords: &maxRecords,
			Marker:     marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe cache subnet groups: %w", err)
		}

		allGroups = append(allGroups, output.CacheSubnetGroups...)

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	return allGroups, nil
}

func (c *AWSClient) QuotasClient() quotas.Client {
	return c.quotasClient
}
