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
package s3

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	sqtypes "github.com/aws/aws-sdk-go-v2/service/servicequotas/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type mockS3Client struct {
	buckets     []types.Bucket
	quotaLimits map[string]float64
	listErr     bool
	quotaErr    bool
}

func (m *mockS3Client) ListBuckets(_ context.Context) ([]types.Bucket, error) {
	if m.listErr {
		return nil, fmt.Errorf("mock list buckets error")
	}
	return m.buckets, nil
}

func (m *mockS3Client) QuotasClient() quotas.Client {
	return &mockS3QuotasClient{limits: m.quotaLimits, err: m.quotaErr}
}

type mockS3QuotasClient struct {
	limits map[string]float64
	err    bool
}

func (m *mockS3QuotasClient) ListServiceQuotas(_ context.Context, _ *servicequotas.ListServiceQuotasInput, _ ...func(*servicequotas.Options)) (*servicequotas.ListServiceQuotasOutput, error) {
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

type mockS3ConfigProvider struct {
	c *aws.Config
}

func (m *mockS3ConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}

func TestNewS3Service(t *testing.T) {
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
			got := NewS3Service(tt.buildClientFunc)
			require.NotNil(t, got)
		})
	}
}

func TestS3_GetNamespace(t *testing.T) {
	service := NewS3Service(nil)
	require.Equal(t, "AWS/S3", service.GetNamespace())
}

func TestS3_GetServiceCode(t *testing.T) {
	service := NewS3Service(nil)
	require.Equal(t, "s3", service.GetServiceCode())
}

func TestS3_GetLimits(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	tests := []struct {
		name            string
		client          *mockS3Client
		wantErr         bool
		wantResultCount int
		wantUsage       *float64
	}{
		{
			name: "quota API error returns error",
			client: &mockS3Client{
				quotaErr: true,
			},
			wantErr: true,
		},
		{
			name: "buckets counted correctly",
			client: &mockS3Client{
				quotaLimits: map[string]float64{
					"Buckets": 100,
				},
				buckets: []types.Bucket{
					{Name: aws.String("bucket-1")},
					{Name: aws.String("bucket-2")},
					{Name: aws.String("bucket-3")},
				},
			},
			wantResultCount: 1,
			wantUsage:       aws.Float64(3),
		},
		{
			name: "no Buckets quota returns empty results",
			client: &mockS3Client{
				quotaLimits: map[string]float64{
					"Some Other Quota": 50,
				},
			},
			wantResultCount: 0,
		},
		{
			name: "list buckets error returns empty results",
			client: &mockS3Client{
				quotaLimits: map[string]float64{
					"Buckets": 100,
				},
				listErr: true,
			},
			wantResultCount: 0,
		},
		{
			name: "zero buckets returns usage of zero",
			client: &mockS3Client{
				quotaLimits: map[string]float64{
					"Buckets": 100,
				},
				buckets: []types.Bucket{},
			},
			wantResultCount: 1,
			wantUsage:       aws.Float64(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewS3Service(func(_ aws.Config) Client {
				return tt.client
			})

			configProvider := &mockS3ConfigProvider{c: &aws.Config{Region: "us-east-1"}}
			results, err := svc.GetLimits(ctx, logger, "us-east-1", model.Role{}, configProvider)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, results, tt.wantResultCount)

			if tt.wantUsage != nil && tt.wantResultCount > 0 {
				require.NotNil(t, results[0].Usage)
				assert.Equal(t, *tt.wantUsage, *results[0].Usage)
				assert.Equal(t, "Buckets", results[0].LimitName)
			}
		})
	}
}
