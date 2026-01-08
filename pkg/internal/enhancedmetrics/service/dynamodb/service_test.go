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
package dynamodb

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestNewDynamoDBService(t *testing.T) {
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
			got := NewDynamoDBService(tt.buildClientFunc)
			require.NotNil(t, got)
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["ItemCount"])
		})
	}
}

func TestDynamoDB_GetNamespace(t *testing.T) {
	service := NewDynamoDBService(nil)
	expectedNamespace := "AWS/DynamoDB"
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestDynamoDB_ListRequiredPermissions(t *testing.T) {
	service := NewDynamoDBService(nil)
	expectedPermissions := []string{
		"dynamodb:DescribeTable",
		"dynamodb:ListTables",
	}
	require.Equal(t, expectedPermissions, service.ListRequiredPermissions())
}

func TestDynamoDB_ListSupportedEnhancedMetrics(t *testing.T) {
	service := NewDynamoDBService(nil)
	expectedMetrics := []string{
		"ItemCount",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedEnhancedMetrics())
}

func TestDynamoDB_Process(t *testing.T) {
	defaultTables := []types.TableDescription{
		{
			TableArn:  aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
			TableName: aws.String("test-table"),
			ItemCount: aws.Int64(1000),
		},
	}

	tests := []struct {
		name                 string
		namespace            string
		resources            []*model.TaggedResource
		enhancedMetrics      []*model.EnhancedMetricConfig
		exportedTagOnMetrics []string
		tables               []types.TableDescription
		describeErr          bool
		wantErr              bool
		wantResultCount      int
	}{
		{
			name:            "empty resources",
			namespace:       "AWS/DynamoDB",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			tables:          defaultTables,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			namespace:       "AWS/DynamoDB",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			tables:          defaultTables,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			namespace:       "AWS/EC2",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			tables:          defaultTables,
			wantErr:         true,
			wantResultCount: 0,
		},
		{
			name:            "metadata not loaded",
			namespace:       "AWS/DynamoDB",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			describeErr:     true,
			wantErr:         true,
			wantResultCount: 0,
		},
		{
			name:      "successfully process metric",
			namespace: "AWS/DynamoDB",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test-table"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			tables:          defaultTables,
			wantErr:         false,
			wantResultCount: 1,
		},
		{
			name:      "successfully process metric with global secondary indexes",
			namespace: "AWS/DynamoDB",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test-table-with-gsi"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			tables: []types.TableDescription{
				{
					TableArn:  aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/test-table-with-gsi"),
					TableName: aws.String("test-table-with-gsi"),
					ItemCount: aws.Int64(1000),
					GlobalSecondaryIndexes: []types.GlobalSecondaryIndexDescription{
						{
							IndexName: aws.String("test-gsi-1"),
							ItemCount: aws.Int64(500),
						},
						{
							IndexName: aws.String("test-gsi-2"),
							ItemCount: aws.Int64(300),
						},
					},
				},
			},
			wantErr:         false,
			wantResultCount: 3, // 1 for table + 2 for GSIs
		},
		{
			name:      "resource not found in metadata",
			namespace: "AWS/DynamoDB",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/non-existent"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			tables:          defaultTables,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "unsupported metric",
			namespace: "AWS/DynamoDB",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test-table"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			tables:          defaultTables,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "multiple resources and metrics",
			namespace: "AWS/DynamoDB",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test-table-1"},
				{ARN: "arn:aws:dynamodb:us-east-1:123456789012:table/test-table-2"},
			},
			enhancedMetrics:      []*model.EnhancedMetricConfig{{Name: "ItemCount"}},
			exportedTagOnMetrics: []string{"Name"},
			tables: []types.TableDescription{
				{
					TableArn:  aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/test-table-1"),
					TableName: aws.String("test-table-1"),
					ItemCount: aws.Int64(1000),
				},
				{
					TableArn:  aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/test-table-2"),
					TableName: aws.String("test-table-2"),
					ItemCount: aws.Int64(2000),
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

			mockClient := &mockServiceDynamoDBClient{
				tables:      tt.tables,
				describeErr: tt.describeErr,
			}

			service := NewDynamoDBService(func(_ aws.Config) Client {
				return mockClient
			})

			mockConfig := &mockConfigProvider{
				c: &aws.Config{Region: "us-east-1"},
			}

			result, err := service.GetMetrics(ctx, logger, tt.namespace, tt.resources, tt.enhancedMetrics, tt.exportedTagOnMetrics, "us-east-1", model.Role{}, mockConfig)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, result, tt.wantResultCount)

			if tt.wantResultCount > 0 {
				for _, metric := range result {
					require.NotNil(t, metric)
					require.Equal(t, "AWS/DynamoDB", metric.Namespace)
					require.NotEmpty(t, metric.Dimensions)
					require.NotNil(t, metric.GetMetricDataResult)
					require.Nil(t, metric.GetMetricStatisticsResult)
				}
			}
		})
	}
}

type mockServiceDynamoDBClient struct {
	tables      []types.TableDescription
	describeErr bool
}

func (m *mockServiceDynamoDBClient) DescribeAllTables(_ context.Context, _ *slog.Logger) ([]types.TableDescription, error) {
	if m.describeErr {
		return nil, fmt.Errorf("mock describe error")
	}
	return m.tables, nil
}

type mockConfigProvider struct {
	c *aws.Config
}

func (m *mockConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}
