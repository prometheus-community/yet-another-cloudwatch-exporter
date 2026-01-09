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
				describeCacheClustersFunc: func(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
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
				describeCacheClustersFunc: func() func(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
					callCount := 0
					return func(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
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
				describeCacheClustersFunc: func(_ context.Context, _ *elasticache.DescribeCacheClustersInput, _ ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
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
			got, err := c.DescribeAllCacheClusters(context.Background(), slog.New(slog.DiscardHandler))
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

// mockElastiCacheClient is a mock implementation of AWS ElastiCache Client
type mockElastiCacheClient struct {
	describeCacheClustersFunc func(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

func (m *mockElastiCacheClient) DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error) {
	return m.describeCacheClustersFunc(ctx, params, optFns...)
}
