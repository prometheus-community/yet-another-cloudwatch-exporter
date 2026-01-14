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
			got := NewLambdaService(tt.buildClientFunc)
			require.NotNil(t, got)
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["Timeout"])
		})
	}
}

func TestLambda_GetNamespace(t *testing.T) {
	service := NewLambdaService(nil)
	expectedNamespace := awsLambdaNamespace
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestLambda_ListRequiredPermissions(t *testing.T) {
	service := NewLambdaService(nil)
	expectedPermissions := map[string][]string{
		"Timeout": {"lambda:ListFunctions"},
	}
	require.Equal(t, expectedPermissions, service.ListRequiredPermissions())
}

func TestLambda_ListSupportedEnhancedMetrics(t *testing.T) {
	service := NewLambdaService(nil)
	expectedMetrics := []string{
		"Timeout",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedEnhancedMetrics())
}

func TestLambda_GetMetrics(t *testing.T) {
	makeFunctionConfiguration := func(name string, timeout int32) types.FunctionConfiguration {
		arn := fmt.Sprintf("arn:aws:lambda:us-east-1:123456789012:function:%s", name)
		return types.FunctionConfiguration{
			FunctionArn:  aws.String(arn),
			FunctionName: aws.String(name),
			Timeout:      aws.Int32(timeout),
		}
	}

	tests := []struct {
		name            string
		resources       []*model.TaggedResource
		enhancedMetrics []*model.EnhancedMetricConfig
		functions       []types.FunctionConfiguration
		wantErr         bool
		wantCount       int
	}{
		{
			name:            "empty resources returns empty",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			functions:       []types.FunctionConfiguration{makeFunctionConfiguration("test", 300)},
			wantCount:       0,
		},
		{
			name:            "empty enhanced metrics returns empty",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			functions:       []types.FunctionConfiguration{makeFunctionConfiguration("test", 300)},
			wantCount:       0,
		},
		{
			name:            "wrong namespace returns error",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			wantErr:         false,
		},
		{
			name: "successfully received single metric",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test", Namespace: awsLambdaNamespace},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			functions:       []types.FunctionConfiguration{makeFunctionConfiguration("test", 300)},
			wantCount:       1,
		},
		{
			name: "skips unsupported metrics",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:test"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			functions:       []types.FunctionConfiguration{makeFunctionConfiguration("test", 300)},
			wantCount:       0,
		},
		{
			name: "processes multiple resources",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:func1", Namespace: awsLambdaNamespace},
				{ARN: "arn:aws:lambda:us-east-1:123456789012:function:func2", Namespace: awsLambdaNamespace},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "Timeout"}},
			functions:       []types.FunctionConfiguration{makeFunctionConfiguration("func1", 300), makeFunctionConfiguration("func2", 600)},
			wantCount:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewLambdaService(func(_ aws.Config) Client {
				return &mockServiceLambdaClient{functions: tt.functions}
			})

			result, err := service.GetMetrics(context.Background(), slog.New(slog.DiscardHandler), tt.resources, tt.enhancedMetrics, nil, "us-east-1", model.Role{}, &mockConfigProvider{c: &aws.Config{Region: "us-east-1"}})

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, result, tt.wantCount)

			for _, metric := range result {
				require.Equal(t, awsLambdaNamespace, metric.Namespace)
				require.NotEmpty(t, metric.Dimensions)
				require.NotNil(t, metric.GetMetricDataResult)
			}
		})
	}
}

type mockServiceLambdaClient struct {
	functions []types.FunctionConfiguration
}

func (m *mockServiceLambdaClient) ListAllFunctions(_ context.Context, _ *slog.Logger) ([]types.FunctionConfiguration, error) {
	return m.functions, nil
}

type mockConfigProvider struct {
	c *aws.Config
}

func (m *mockConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}
