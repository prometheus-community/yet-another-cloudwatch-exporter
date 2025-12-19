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
