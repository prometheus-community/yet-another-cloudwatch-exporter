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
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	sqtypes "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type mockElastiCacheClient struct {
	clusters     []types.CacheCluster
	subnetGroups []types.CacheSubnetGroup
	quotaLimits  map[string]float64
	clusterErr   bool
	subnetErr    bool
	quotaErr     bool
}

func (m *mockElastiCacheClient) DescribeAllCacheClusters(_ context.Context) ([]types.CacheCluster, error) {
	if m.clusterErr {
		return nil, fmt.Errorf("mock describe clusters error")
	}
	return m.clusters, nil
}

func (m *mockElastiCacheClient) DescribeAllCacheSubnetGroups(_ context.Context) ([]types.CacheSubnetGroup, error) {
	if m.subnetErr {
		return nil, fmt.Errorf("mock describe subnet groups error")
	}
	return m.subnetGroups, nil
}

func (m *mockElastiCacheClient) QuotasClient() quotas.Client {
	return &mockElastiCacheQuotasClient{limits: m.quotaLimits, err: m.quotaErr}
}

type mockElastiCacheQuotasClient struct {
	limits map[string]float64
	err    bool
}

func (m *mockElastiCacheQuotasClient) ListServiceQuotas(_ context.Context, _ *servicequotas.ListServiceQuotasInput, _ ...func(*servicequotas.Options)) (*servicequotas.ListServiceQuotasOutput, error) {
	if m.err {
		return nil, fmt.Errorf("mock quota error")
	}
	var quotaList []sqtypes.ServiceQuota
	for name, value := range m.limits {
		n := name
		v := value
		quotaList = append(quotaList, sqtypes.ServiceQuota{
			QuotaName: &n,
			Value:     &v,
		})
	}
	return &servicequotas.ListServiceQuotasOutput{
		Quotas: quotaList,
	}, nil
}

type mockElastiCacheConfigProvider struct {
	c *aws.Config
}

func (m *mockElastiCacheConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}

func TestNewElastiCacheQuotaService(t *testing.T) {
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
		})
	}
}

func TestElastiCacheQuota_GetNamespace(t *testing.T) {
	service := NewElastiCacheService(nil)
	require.Equal(t, "AWS/ElastiCache", service.GetNamespace())
}

func TestElastiCacheQuota_GetServiceCode(t *testing.T) {
	service := NewElastiCacheService(nil)
	require.Equal(t, "elasticache", service.GetServiceCode())
}

func TestElastiCacheQuota_GetLimits(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	tests := []struct {
		name            string
		client          *mockElastiCacheClient
		wantErr         bool
		wantResultCount int
		checkUsage      map[string]float64
	}{
		{
			name: "quota API error returns error",
			client: &mockElastiCacheClient{
				quotaErr: true,
			},
			wantErr: true,
		},
		{
			name: "nodes per region counted correctly",
			client: &mockElastiCacheClient{
				quotaLimits: map[string]float64{
					"Nodes per Region": 300,
				},
				clusters: []types.CacheCluster{
					{NumCacheNodes: aws.Int32(3)},
					{NumCacheNodes: aws.Int32(2)},
				},
			},
			wantResultCount: 1,
			checkUsage: map[string]float64{
				"Nodes per Region": 5,
			},
		},
		{
			name: "subnet groups per region counted correctly",
			client: &mockElastiCacheClient{
				quotaLimits: map[string]float64{
					"Subnet Groups per Region": 50,
				},
				subnetGroups: []types.CacheSubnetGroup{
					{CacheSubnetGroupName: aws.String("group-1")},
					{CacheSubnetGroupName: aws.String("group-2")},
				},
			},
			wantResultCount: 1,
			checkUsage: map[string]float64{
				"Subnet Groups per Region": 2,
			},
		},
		{
			name: "both nodes and subnet groups returned",
			client: &mockElastiCacheClient{
				quotaLimits: map[string]float64{
					"Nodes per Region":         300,
					"Subnet Groups per Region": 50,
				},
				clusters: []types.CacheCluster{
					{NumCacheNodes: aws.Int32(1)},
				},
				subnetGroups: []types.CacheSubnetGroup{
					{CacheSubnetGroupName: aws.String("group-1")},
				},
			},
			wantResultCount: 2,
			checkUsage: map[string]float64{
				"Nodes per Region":         1,
				"Subnet Groups per Region": 1,
			},
		},
		{
			name: "describe clusters error returns empty for nodes, subnet groups still work",
			client: &mockElastiCacheClient{
				quotaLimits: map[string]float64{
					"Nodes per Region":         300,
					"Subnet Groups per Region": 50,
				},
				clusterErr: true,
				subnetGroups: []types.CacheSubnetGroup{
					{CacheSubnetGroupName: aws.String("group-1")},
				},
			},
			// Nodes quota is present but cluster describe fails, so no nodes result.
			// Subnet groups still work.
			wantResultCount: 1,
			checkUsage: map[string]float64{
				"Subnet Groups per Region": 1,
			},
		},
		{
			name: "no matching quotas returns empty results",
			client: &mockElastiCacheClient{
				quotaLimits: map[string]float64{
					"Some Other Quota": 100,
				},
			},
			wantResultCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewElastiCacheService(func(_ aws.Config) Client {
				return tt.client
			})

			configProvider := &mockElastiCacheConfigProvider{c: &aws.Config{Region: "us-east-1"}}
			results, err := svc.GetLimits(ctx, logger, "us-east-1", model.Role{}, configProvider)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, results, tt.wantResultCount)

			if tt.checkUsage != nil {
				for _, result := range results {
					if expectedUsage, ok := tt.checkUsage[result.LimitName]; ok {
						require.NotNil(t, result.Usage, "usage should not be nil for %s", result.LimitName)
						assert.Equal(t, expectedUsage, *result.Usage, "usage mismatch for %s", result.LimitName)
					}
				}
			}
		})
	}
}
