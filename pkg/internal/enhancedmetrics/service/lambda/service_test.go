// Copyright 2024 The Prometheus Authors
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
package lambda

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestNewLambdaService(t *testing.T) {
	tests := []struct {
		name             string
		buildClientFunc  func(cfg aws.Config) Client
		wantNilClients   bool
		wantMetricsCount int
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
			got := NewLambdaService(tt.buildClientFunc)
			require.NotNil(t, got)
			require.NotNil(t, got.clients)
			require.Nil(t, got.regionalData)
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["Timeout"])
		})
	}
}

func TestLambda_GetNamespace(t *testing.T) {
	service := NewLambdaService(nil)
	expectedNamespace := "AWS/Lambda"
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestLambda_ListRequiredPermissions(t *testing.T) {
	service := NewLambdaService(nil)
	expectedPermissions := []string{
		"lambda:ListFunctions",
	}
	require.Equal(t, expectedPermissions, service.ListRequiredPermissions())
}

func TestLambda_ListSupportedMetrics(t *testing.T) {
	service := NewLambdaService(nil)
	expectedMetrics := []string{
		"Timeout",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedMetrics())
}

func TestLambda_LoadMetricsMetadata(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func() *mockServiceLambdaClient
		region         string
		existingData   map[string]*types.FunctionConfiguration
		wantErr        bool
		wantDataLoaded bool
	}{
		{
			name:   "successfully load metadata",
			region: "us-east-1",
			setupMock: func() *mockServiceLambdaClient {
				mock := &mockServiceLambdaClient{}
				functionArn := "arn:aws:lambda:us-east-1:123456789012:function:test-function"
				functionName := "test-function"
				mock.functions = []types.FunctionConfiguration{
					{
						FunctionArn:  &functionArn,
						FunctionName: &functionName,
					},
				}
				return mock
			},
			wantErr:        false,
			wantDataLoaded: true,
		},
		{
			name:   "list functions error",
			region: "us-east-1",
			setupMock: func() *mockServiceLambdaClient {
				return &mockServiceLambdaClient{listErr: true}
			},
			wantErr:        true,
			wantDataLoaded: false,
		},
		{
			name:   "metadata already loaded - skip loading",
			region: "us-east-1",
			existingData: map[string]*types.FunctionConfiguration{
				"existing-arn": {},
			},
			setupMock: func() *mockServiceLambdaClient {
				return &mockServiceLambdaClient{}
			},
			wantErr:        false,
			wantDataLoaded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)
			var service *Lambda

			if tt.setupMock == nil {
				service = NewLambdaService(nil)
			} else {
				service = NewLambdaService(func(_ aws.Config) Client {
					return tt.setupMock()
				})
			}

			if tt.existingData != nil {
				service.regionalData = tt.existingData
			}

			mockConfig := &mockConfigProvider{
				c: &aws.Config{
					Region: tt.region,
				},
			}
			err := service.LoadMetricsMetadata(ctx, logger, tt.region, model.Role{}, mockConfig)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.wantDataLoaded {
				require.NotEmpty(t, service.regionalData)
			}
		})
	}
}

func TestLambda_Process(t *testing.T) {
	rd := map[string]*types.FunctionConfiguration{
		"arn:aws:lambda:us-east-1:123456789012:function:test-function": {
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:test-function"),
			FunctionName: aws.String("test-function"),
			Timeout:      aws.Int32(300),
		},
	}

	tests := []struct {
		name                 string
		namespace            string
		resources            []*model.TaggedResource
		enhancedMetrics      []*model.EnhancedMetricConfig
		exportedTagOnMetrics []string
		regionalData         map[string]*types.FunctionConfiguration
		wantErr              bool
		wantResultCount      int
	}{
		{
			name:            "empty resources",
			namespace:       "AWS/Lambda",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			namespace:       "AWS/Lambda",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			namespace:       "AWS/EC2",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			regionalData:    rd,
			wantErr:         true,
			wantResultCount: 0,
		},
		{
			name:            "metadata not loaded",
			namespace:       "AWS/Lambda",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			regionalData:    nil,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "successfully process metric",
			namespace: "AWS/Lambda",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test-function"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 1,
		},
		{
			name:      "resource not found in metadata",
			namespace: "AWS/Lambda",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:non-existent"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "unsupported metric",
			namespace: "AWS/Lambda",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test-function"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "multiple resources and metrics",
			namespace: "AWS/Lambda",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test-function-1"},
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test-function-2"},
			},
			enhancedMetrics:      []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			exportedTagOnMetrics: []string{"Name"},
			regionalData: map[string]*types.FunctionConfiguration{
				"arn:aws:lambda:us-east-1:123456789012:function:test-function-1": {
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:test-function-1"),
					FunctionName: aws.String("test-function-1"),
					Timeout:      aws.Int32(300),
				},
				"arn:aws:lambda:us-east-1:123456789012:function:test-function-2": {
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:test-function-2"),
					FunctionName: aws.String("test-function-2"),
					Timeout:      aws.Int32(600),
				},
			},
			wantErr:         false,
			wantResultCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)
			service := NewLambdaService(
				func(_ aws.Config) Client {
					return nil
				},
			)
			// we directly set the regionalData for testing
			service.regionalData = tt.regionalData

			result, err := service.Process(ctx, logger, tt.namespace, tt.resources, tt.enhancedMetrics, tt.exportedTagOnMetrics)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, result, tt.wantResultCount)

			if tt.wantResultCount > 0 {
				for _, metric := range result {
					require.NotNil(t, metric)
					require.Equal(t, "AWS/Lambda", metric.Namespace)
					require.NotEmpty(t, metric.Dimensions)
					require.NotNil(t, metric.GetMetricDataResult)
					require.Empty(t, metric.GetMetricDataResult.Statistic)
					require.Nil(t, metric.GetMetricStatisticsResult)
				}
			}
		})
	}
}

type mockServiceLambdaClient struct {
	functions []types.FunctionConfiguration
	listErr   bool
}

func (m *mockServiceLambdaClient) ListAllFunctions(_ context.Context, _ *slog.Logger) ([]types.FunctionConfiguration, error) {
	if m.listErr {
		return nil, fmt.Errorf("mock list error")
	}
	return m.functions, nil
}

type mockConfigProvider struct {
	c *aws.Config
}

func (m *mockConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}
