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
package ec2

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	sqtypes "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type mockEC2Client struct {
	reservations    []types.Reservation
	addresses       []types.Address
	quotaLimits     map[string]float64
	describeInstErr bool
	describeAddrErr bool
	quotaErr        bool
}

func (m *mockEC2Client) DescribeRunningInstances(_ context.Context) ([]types.Reservation, error) {
	if m.describeInstErr {
		return nil, fmt.Errorf("mock describe instances error")
	}
	return m.reservations, nil
}

func (m *mockEC2Client) DescribeAddresses(_ context.Context) ([]types.Address, error) {
	if m.describeAddrErr {
		return nil, fmt.Errorf("mock describe addresses error")
	}
	return m.addresses, nil
}

func (m *mockEC2Client) QuotasClient() quotas.Client {
	return &mockQuotasClient{limits: m.quotaLimits, err: m.quotaErr}
}

type mockQuotasClient struct {
	limits map[string]float64
	err    bool
}

func (m *mockQuotasClient) ListServiceQuotas(_ context.Context, _ *servicequotas.ListServiceQuotasInput, _ ...func(*servicequotas.Options)) (*servicequotas.ListServiceQuotasOutput, error) {
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

type mockEC2ConfigProvider struct {
	c *aws.Config
}

func (m *mockEC2ConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}

func TestNewEC2Service(t *testing.T) {
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
			got := NewEC2Service(tt.buildClientFunc)
			require.NotNil(t, got)
		})
	}
}

func TestEC2_GetNamespace(t *testing.T) {
	service := NewEC2Service(nil)
	require.Equal(t, "AWS/EC2", service.GetNamespace())
}

func TestEC2_GetServiceCode(t *testing.T) {
	service := NewEC2Service(nil)
	require.Equal(t, "ec2", service.GetServiceCode())
}

func TestEC2_GetLimits(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	tests := []struct {
		name            string
		client          *mockEC2Client
		wantErr         bool
		wantResultCount int
		checkUsage      map[string]float64
	}{
		{
			name: "quota API error returns error",
			client: &mockEC2Client{
				quotaErr: true,
			},
			wantErr: true,
		},
		{
			name: "standard instances counted correctly",
			client: &mockEC2Client{
				quotaLimits: map[string]float64{
					"Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances": 1152,
				},
				reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{InstanceType: types.InstanceTypeM5Xlarge},
							{InstanceType: types.InstanceTypeC5Large},
							{InstanceType: types.InstanceTypeT3Micro},
						},
					},
				},
			},
			wantResultCount: 1,
			checkUsage: map[string]float64{
				"Running On-Demand Standard instances": 3,
			},
		},
		{
			name: "elastic IPs counted correctly",
			client: &mockEC2Client{
				quotaLimits: map[string]float64{
					"EC2-VPC Elastic IPs": 5,
				},
				addresses: []types.Address{
					{PublicIp: aws.String("1.2.3.4")},
					{PublicIp: aws.String("5.6.7.8")},
				},
			},
			wantResultCount: 1,
			checkUsage: map[string]float64{
				"EC2-VPC Elastic IPs": 2,
			},
		},
		{
			name: "mixed instance families counted separately",
			client: &mockEC2Client{
				quotaLimits: map[string]float64{
					"Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances": 1152,
					"Running On-Demand P instances": 10,
				},
				reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{InstanceType: types.InstanceTypeM5Xlarge},
							{InstanceType: types.InstanceTypeP32xlarge},
							{InstanceType: types.InstanceTypeP38xlarge},
							{InstanceType: types.InstanceTypeC5Large},
						},
					},
				},
			},
			wantResultCount: 2,
			checkUsage: map[string]float64{
				"Running On-Demand Standard instances": 2,
				"Running On-Demand P instances": 2,
			},
		},
		{
			name: "describe instances error still returns quota limits",
			client: &mockEC2Client{
				quotaLimits: map[string]float64{
					"Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances": 1152,
					"EC2-VPC Elastic IPs": 5,
				},
				describeInstErr: true,
				addresses:       []types.Address{{PublicIp: aws.String("1.2.3.4")}},
			},
			// Instance quotas still get added (with zero usage from empty family counts),
			// and EIP quota is added too.
			wantResultCount: 2,
		},
		{
			name: "no matching quotas returns empty results",
			client: &mockEC2Client{
				quotaLimits: map[string]float64{
					"Some Other Quota": 100,
				},
			},
			wantResultCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewEC2Service(func(_ aws.Config) Client {
				return tt.client
			})

			configProvider := &mockEC2ConfigProvider{c: &aws.Config{Region: "us-east-1"}}
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
