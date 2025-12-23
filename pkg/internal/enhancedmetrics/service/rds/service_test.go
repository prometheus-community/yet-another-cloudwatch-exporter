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
			require.Nil(t, got.regionalData)
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

func TestRDS_ListSupportedMetrics(t *testing.T) {
	service := NewRDSService(nil)
	expectedMetrics := []string{
		"AllocatedStorage",
	}
	require.Equal(t, expectedMetrics, service.ListSupportedMetrics())
}

func TestRDS_LoadMetricsMetadata(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func() *mockServiceRDSClient
		region         string
		existingData   map[string]*types.DBInstance
		wantErr        bool
		wantDataLoaded bool
	}{
		{
			name:   "successfully load metadata",
			region: "us-east-1",
			setupMock: func() *mockServiceRDSClient {
				mock := &mockServiceRDSClient{}
				instanceArn := "arn:aws:rds:us-east-1:123456789012:db:test-instance"
				instanceID := "test-instance"
				mock.instances = []types.DBInstance{
					{
						DBInstanceArn:        &instanceArn,
						DBInstanceIdentifier: &instanceID,
					},
				}
				return mock
			},
			wantErr:        false,
			wantDataLoaded: true,
		},
		{
			name:   "describe instances error",
			region: "us-east-1",
			setupMock: func() *mockServiceRDSClient {
				return &mockServiceRDSClient{describeErr: true}
			},
			wantErr:        true,
			wantDataLoaded: false,
		},
		{
			name:   "metadata already loaded - skip loading",
			region: "us-east-1",
			existingData: map[string]*types.DBInstance{
				"existing-arn": {},
			},
			setupMock: func() *mockServiceRDSClient {
				return &mockServiceRDSClient{}
			},
			wantErr:        false,
			wantDataLoaded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)
			var service *RDS

			if tt.setupMock == nil {
				service = NewRDSService(nil)
			} else {
				service = NewRDSService(func(_ aws.Config) Client {
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

func TestRDS_Process(t *testing.T) {
	rd := map[string]*types.DBInstance{
		"arn:aws:rds:us-east-1:123456789012:db:test-instance": {
			DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:test-instance"),
			DBInstanceIdentifier: aws.String("test-instance"),
			DBInstanceClass:      aws.String("db.t3.micro"),
			Engine:               aws.String("postgres"),
			AllocatedStorage:     aws.Int32(100),
		},
	}
	tests := []struct {
		name                 string
		namespace            string
		resources            []*model.TaggedResource
		enhancedMetrics      []*model.EnhancedMetricConfig
		exportedTagOnMetrics []string
		regionalData         map[string]*types.DBInstance
		wantErr              bool
		wantResultCount      int
	}{
		{
			name:            "empty resources",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "empty enhanced metrics",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:rds:us-east-1:123456789012:db:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:            "wrong namespace",
			namespace:       "AWS/EC2",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:rds:us-east-1:123456789012:db:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    rd,
			wantErr:         true,
			wantResultCount: 0,
		},
		{
			name:            "metadata not loaded",
			namespace:       "AWS/RDS",
			resources:       []*model.TaggedResource{{ARN: "arn:aws:rds:us-east-1:123456789012:db:test"}},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    nil,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "successfully process metric",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 1,
		},
		{
			name:      "resource not found in metadata",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:non-existent"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			regionalData:    rd,
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "unsupported metric",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance"},
			},
			enhancedMetrics: []*model.EnhancedMetricConfig{{Name: "UnsupportedMetric"}},
			regionalData: map[string]*types.DBInstance{
				"arn:aws:rds:us-east-1:123456789012:db:test-instance": {
					AllocatedStorage: aws.Int32(100),
				},
			},
			wantErr:         false,
			wantResultCount: 0,
		},
		{
			name:      "multiple resources and metrics",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance-1"},
				{ARN: "arn:aws:rds:us-east-1:123456789012:db:test-instance-2"},
			},
			enhancedMetrics:      []*model.EnhancedMetricConfig{{Name: "AllocatedStorage"}},
			exportedTagOnMetrics: []string{"Name"},
			regionalData: map[string]*types.DBInstance{
				"arn:aws:rds:us-east-1:123456789012:db:test-instance-1": {
					DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:test-instance-1"),
					DBInstanceIdentifier: aws.String("test-instance-1"),
					AllocatedStorage:     aws.Int32(100),
				},
				"arn:aws:rds:us-east-1:123456789012:db:test-instance-2": {
					DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:test-instance-2"),
					DBInstanceIdentifier: aws.String("test-instance-2"),
					AllocatedStorage:     aws.Int32(200),
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
			service := NewRDSService(nil)
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
					require.Equal(t, "AWS/RDS", metric.Namespace)
					require.NotEmpty(t, metric.Dimensions)
					require.NotNil(t, metric.GetMetricDataResult)
					require.Empty(t, metric.GetMetricDataResult.Statistic)
					require.Nil(t, metric.GetMetricStatisticsResult)
				}
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
