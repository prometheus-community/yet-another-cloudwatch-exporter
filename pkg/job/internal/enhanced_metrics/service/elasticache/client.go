package elasticache

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

// todo: change logging to debug where appropriate

// AWSElastiCacheClient wraps the AWS ElastiCache client
type AWSElastiCacheClient struct {
	client *elasticache.Client
}

// NewElastiCacheClient creates a new ElastiCache client with default AWS configuration
func NewElastiCacheClient(ctx context.Context) (*AWSElastiCacheClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSElastiCacheClient{
		client: elasticache.NewFromConfig(cfg),
	}, nil
}

// NewElastiCacheClientWithConfig creates a new ElastiCache client with custom AWS configuration
func NewElastiCacheClientWithConfig(cfg aws.Config) *AWSElastiCacheClient {
	return &AWSElastiCacheClient{
		client: elasticache.NewFromConfig(cfg),
	}
}

// DescribeCacheClustersInput contains parameters for DescribeCacheClusters
type DescribeCacheClustersInput struct {
	CacheClusterId    *string
	MaxRecords        *int32
	Marker            *string
	ShowCacheNodeInfo *bool
}

// DescribeCacheClustersOutput contains the response from DescribeCacheClusters
type DescribeCacheClustersOutput struct {
	CacheClusters []types.CacheCluster
	Marker        *string
}

// DescribeCacheClusters retrieves information about cache clusters
func (c *AWSElastiCacheClient) DescribeCacheClusters(ctx context.Context, input *DescribeCacheClustersInput) (*DescribeCacheClustersOutput, error) {
	elasticacheInput := &elasticache.DescribeCacheClustersInput{}

	if input != nil {
		elasticacheInput.CacheClusterId = input.CacheClusterId
		elasticacheInput.MaxRecords = input.MaxRecords
		elasticacheInput.Marker = input.Marker
		elasticacheInput.ShowCacheNodeInfo = input.ShowCacheNodeInfo
	}

	result, err := c.client.DescribeCacheClusters(ctx, elasticacheInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cache clusters: %w", err)
	}

	return &DescribeCacheClustersOutput{
		CacheClusters: result.CacheClusters,
		Marker:        result.Marker,
	}, nil
}

// DescribeAllCacheClusters retrieves all cache clusters with pagination support
func (c *AWSElastiCacheClient) DescribeAllCacheClusters(ctx context.Context, logger *slog.Logger) ([]types.CacheCluster, error) {
	logger.Info("Looking for all ElastiCache clusters")
	var allClusters []types.CacheCluster
	var marker *string
	var maxRecords int32 = 100
	showNodeInfo := true

	for {
		output, err := c.DescribeCacheClusters(ctx, &DescribeCacheClustersInput{
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

	return allClusters, nil
}
