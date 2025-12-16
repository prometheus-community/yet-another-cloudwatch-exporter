package elasticache

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

func TestAWSElastiCacheClient_DescribeAllCacheClusters(t *testing.T) {
	tests := []struct {
		name    string
		client  awsClient
		want    []types.CacheCluster
		wantErr bool
	}{
		{
			name: "success - single page",
			client: &mockElastiCacheClient{
				describeCacheClustersFunc: func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
					return &elasticache.DescribeCacheClustersOutput{
						CacheClusters: []types.CacheCluster{
							{CacheClusterId: aws.String("cluster-1")},
						},
						Marker: nil,
					}, nil
				},
			},
			want: []types.CacheCluster{
				{CacheClusterId: aws.String("cluster-1")},
			},
			wantErr: false,
		},
		{
			name: "success - multiple pages",
			client: &mockElastiCacheClient{
				describeCacheClustersFunc: func() func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
					callCount := 0
					return func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
						callCount++
						if callCount == 1 {
							return &elasticache.DescribeCacheClustersOutput{
								CacheClusters: []types.CacheCluster{
									{CacheClusterId: aws.String("cluster-1")},
								},
								Marker: aws.String("marker1"),
							}, nil
						}
						return &elasticache.DescribeCacheClustersOutput{
							CacheClusters: []types.CacheCluster{
								{CacheClusterId: aws.String("cluster-2")},
							},
							Marker: nil,
						}, nil
					}
				}(),
			},
			want: []types.CacheCluster{
				{CacheClusterId: aws.String("cluster-1")},
				{CacheClusterId: aws.String("cluster-2")},
			},
			wantErr: false,
		},
		{
			name: "error - API failure",
			client: &mockElastiCacheClient{
				describeCacheClustersFunc: func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AWSElastiCacheClient{
				client: tt.client,
			}
			got, err := c.DescribeAllCacheClusters(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
			if (err != nil) != tt.wantErr {
				t.Errorf("DescribeAllCacheClusters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DescribeAllCacheClusters() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockElastiCacheClient is a mock implementation of awsClient
type mockElastiCacheClient struct {
	describeCacheClustersFunc func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

func (m *mockElastiCacheClient) DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return m.describeCacheClustersFunc(ctx, params, optFns...)
}
