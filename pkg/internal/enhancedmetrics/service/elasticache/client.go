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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

type awsClient interface {
	DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

// AWSElastiCacheClient wraps the AWS ElastiCache client
type AWSElastiCacheClient struct {
	client awsClient
}

// NewElastiCacheClientWithConfig creates a new ElastiCache client with custom AWS configuration
func NewElastiCacheClientWithConfig(cfg aws.Config) Client {
	return &AWSElastiCacheClient{
		client: elasticache.NewFromConfig(cfg),
	}
}

// describeCacheClusters retrieves information about cache clusters
func (c *AWSElastiCacheClient) describeCacheClusters(ctx context.Context, input *elasticache.DescribeCacheClustersInput) (*elasticache.DescribeCacheClustersOutput, error) {
	result, err := c.client.DescribeCacheClusters(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cache clusters: %w", err)
	}

	return result, nil
}

// DescribeAllCacheClusters retrieves all cache clusters with pagination support
func (c *AWSElastiCacheClient) DescribeAllCacheClusters(ctx context.Context, logger *slog.Logger) ([]types.CacheCluster, error) {
	logger.Debug("Describing all ElastiCache cache clusters")
	var allClusters []types.CacheCluster
	var marker *string
	var maxRecords int32 = 100
	showNodeInfo := true

	for {
		output, err := c.describeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
			MaxRecords:        &maxRecords,
			Marker:            marker,
			ShowCacheNodeInfo: &showNodeInfo,
		})
		if err != nil {
			return nil, err
		}

		allClusters = append(allClusters, output.CacheClusters...)

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	logger.Debug("Completed describing ElastiCache cache clusters", slog.Int("totalClusters", len(allClusters)))

	return allClusters, nil
}
