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
package rds

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestNewRDSService(t *testing.T) {
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
			got := NewRDSService(tt.buildClientFunc)
			require.NotNil(t, got)
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["AllocatedStorage"])
		})
	}
}

func TestRDS_GetNamespace(t *testing.T) {
	service := NewRDSService(nil)
	expectedNamespace := awsRdsNamespace
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestRDS_ListRequiredPermissions(t *testing.T) {
	service := NewRDSService(nil)
	expectedPermissions := map[string][]string{
		"AllocatedStorage": {"rds:DescribeDBInstances"},
	}
	require.Equal(t, expectedPermissions, service.ListRequiredPermissions())
}

func TestRDS_ListSupportedEnhancedMetrics(t *testing.T) {
	service := NewRDSService(nil)
	expectedMetrics := []string{
		"AllocatedStorage",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedEnhancedMetrics())
}

func TestRDS_GetMetrics(t *testing.T) {
	testInstance := makeTestDBInstance("test-instance", 100)
	testARN := *testInstance.DBInstanceArn

	tests := []struct {
		name            string
		resources       []*model.TaggedResource
		enhancedMetrics []*model.EnhancedMetricConfig
		regionalData    map[string]*types.DBInstance
		wantErr         bool
		wantResultCount int
		wantValues      []float64 // Expected values in bytes
	}{
		{
			name:            "empty resources",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantErr:         false,
		},
		{
			name:            "metadata not loaded",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    nil,
			wantResultCount: 0,
		},
		{
			name:            "successfully received metric",
			resources:       []*model.TaggedResource{{ARN: testARN, Namespace: awsRdsNamespace}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 1,
			wantValues:      []float64{107374182400}, // 100 GiB in bytes
		},
		{
			name:            "resource not found in metadata",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:rds:us-east-1:123456789012:db:non-existent"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:            "unsupported metric",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name: "multiple resources",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance-1", Namespace: awsRdsNamespace},
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance-2", Namespace: awsRdsNamespace},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData: map[string]*types.DBInstance{
				"arn:aws:rds:us-east-1:123456789012:db:test-instance-1": makeTestDBInstance("test-instance-1", 100),
				"arn:aws:rds:us-east-1:123456789012:db:test-instance-2": makeTestDBInstance("test-instance-2", 200),
			},
			wantResultCount: 2,
			wantValues:      []float64{107374182400, 214748364800}, // 100 and 200 GiB in bytes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestRDSService(tt.regionalData)
			result, err := service.GetMetrics(context.Background(), slog.New(slog.DiscardHandler), tt.resources, tt.enhancedMetrics, nil, "us-east-1", model.Role{}, &mockConfigProvider{c: &aws.Config{Region: "us-east-1"}})

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, result, tt.wantResultCount)

			for i, metric := range result {
				require.Equal(t, awsRdsNamespace, metric.Namespace)
				require.NotEmpty(t, metric.Dimensions)
				require.NotNil(t, metric.GetMetricDataResult)
				require.Nil(t, metric.GetMetricStatisticsResult)

				// Validate the actual value if wantValues is specified
				if len(tt.wantValues) > 0 {
					require.NotNil(t, metric.GetMetricDataResult.DataPoints)
					require.Len(t, metric.GetMetricDataResult.DataPoints, 1)
					require.NotNil(t, metric.GetMetricDataResult.DataPoints[0].Value)
					require.Equal(t, tt.wantValues[i], *metric.GetMetricDataResult.DataPoints[0].Value,
						"expected value in bytes for AllocatedStorage")
				}
			}
		})
	}
}

type mockServiceRDSClient struct {
	instances   []types.DBInstance
	describeErr bool
}

func (m *mockServiceRDSClient) DescribeDBInstances(context.Context, *slog.Logger, []string) ([]types.DBInstance, error) {
	if m.describeErr {
		return nil, fmt.Errorf("mock describe error")
	}
	return m.instances, nil
}

type mockConfigProvider struct {
	c *aws.Config
}

func (m *mockConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}

// Helper functions for test setup

func makeTestDBInstance(name string, storage int32) *types.DBInstance {
	arn := fmt.Sprintf("arn:aws:rds:us-east-1:123456789012:db:%s", name)
	return &types.DBInstance{
		DBInstanceArn:        aws.String(arn),
		DBInstanceIdentifier: aws.String(name),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("postgres"),
		AllocatedStorage:     aws.Int32(storage),
	}
}

func newTestRDSService(regionalData map[string]*types.DBInstance) *RDS {
	return NewRDSService(func(_ aws.Config) Client {
		return &mockServiceRDSClient{
			instances: convertRegionalDataToInstances(regionalData),
		}
	})
}

// convertRegionalDataToInstances converts the regionalData map to a slice of DBInstance
func convertRegionalDataToInstances(regionalData map[string]*types.DBInstance) []types.DBInstance {
	if regionalData == nil {
		return nil
	}
	instances := make([]types.DBInstance, 0, len(regionalData))
	for _, instance := range regionalData {
		if instance != nil {
			instances = append(instances, *instance)
		}
	}
	return instances
}
