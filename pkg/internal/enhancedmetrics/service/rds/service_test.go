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
			got := NewRDSService(tt.buildClientFunc)
			require.NotNil(t, got)
			require.NotNil(t, got.clients)
			require.Len(t, got.supportedMetrics, 1)
			require.NotNil(t, got.supportedMetrics["AllocatedStorage"])
		})
	}
}

func TestRDS_GetNamespace(t *testing.T) {
	service := NewRDSService(nil)
	expectedNamespace := "AWS/RDS"
	require.Equal(t, expectedNamespace, service.GetNamespace())
}

func TestRDS_ListRequiredPermissions(t *testing.T) {
	service := NewRDSService(nil)
	expectedPermissions := []string{
		"rds:DescribeDBInstances",
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

func TestRDS_Process(t *testing.T) {
	testInstance := makeTestDBInstance("test-instance", 100)
	testARN := *testInstance.DBInstanceArn

	tests := []struct {
		name            string
		namespace       string
		resources       []*model.TaggedResource
		enhancedMetrics []*model.EnhancedMetricConfig
		regionalData    map[string]*types.DBInstance
		wantErr         bool
		wantResultCount int
	}{
		{
			name:            "empty resources",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			namespace:       "AWS/EC2",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantErr:         true,
		},
		{
			name:            "metadata not loaded",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    nil,
			wantResultCount: 0,
		},
		{
			name:            "successfully process metric",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 1,
		},
		{
			name:            "resource not found in metadata",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:rds:us-east-1:123456789012:db:non-existent"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:            "unsupported metric",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: testARN}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			regionalData:    map[string]*types.DBInstance{testARN: testInstance},
			wantResultCount: 0,
		},
		{
			name:      "multiple resources",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance-1"},
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance-2"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData: map[string]*types.DBInstance{
				"arn:aws:rds:us-east-1:123456789012:db:test-instance-1": makeTestDBInstance("test-instance-1", 100),
				"arn:aws:rds:us-east-1:123456789012:db:test-instance-2": makeTestDBInstance("test-instance-2", 200),
			},
			wantResultCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestRDSService(tt.regionalData)
			result, err := service.Process(
				context.Background(),
				slog.New(slog.DiscardHandler),
				tt.namespace,
				tt.resources,
				tt.enhancedMetrics,
				nil,
				"us-east-1",
				model.Role{},
				&mockConfigProvider{c: &aws.Config{Region: "us-east-1"}},
			)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, result, tt.wantResultCount)

			for _, metric := range result {
				require.Equal(t, "AWS/RDS", metric.Namespace)
				require.NotEmpty(t, metric.Dimensions)
				require.NotNil(t, metric.GetMetricDataResult)
				require.Nil(t, metric.GetMetricStatisticsResult)
			}
		})
	}
}

type mockServiceRDSClient struct {
	instances   []types.DBInstance
	describeErr bool
}

func (m *mockServiceRDSClient) DescribeAllDBInstances(_ context.Context, _ *slog.Logger) ([]types.DBInstance, error) {
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
