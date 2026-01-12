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
package enhancedmetrics

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// mockFactory is a mock implementation of clients.Factory that also implements config.RegionalConfigProvider
type mockFactory struct {
	configs map[string]*aws.Config
}

func (m *mockFactory) GetAWSRegionalConfig(region string, _ model.Role) *aws.Config {
	if m.configs == nil {
		return &aws.Config{}
	}
	if cfg, ok := m.configs[region]; ok {
		return cfg
	}
	return &aws.Config{}
}

func (m *mockFactory) GetCloudwatchClient(_ string, _ model.Role, _ cloudwatch_client.ConcurrencyConfig) cloudwatch_client.Client {
	return nil
}

func (m *mockFactory) GetTaggingClient(string, model.Role, int) tagging.Client {
	return nil
}

func (m *mockFactory) GetAccountClient(string, model.Role) account.Client {
	return nil
}

// mockNonRegionalFactory is a mock that does NOT implement config.RegionalConfigProvider
type mockNonRegionalFactory struct{}

func (m *mockNonRegionalFactory) GetCloudwatchClient(string, model.Role, cloudwatch_client.ConcurrencyConfig) cloudwatch_client.Client {
	return nil
}

func (m *mockNonRegionalFactory) GetTaggingClient(string, model.Role, int) tagging.Client {
	return nil
}

func (m *mockNonRegionalFactory) GetAccountClient(string, model.Role) account.Client {
	return nil
}

// mockMetricsService is a mock implementation of service.EnhancedMetricsService
type mockMetricsService struct {
	getMetricsCalled int
	err              error
	result           []*model.CloudwatchData
	mu               sync.Mutex
}

func (m *mockMetricsService) GetMetrics(context.Context, *slog.Logger, []*model.TaggedResource, []*model.EnhancedMetricConfig, []string, string, model.Role, config.RegionalConfigProvider) ([]*model.CloudwatchData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getMetricsCalled++
	return m.result, m.err
}

func (m *mockMetricsService) IsMetricSupported(_ string) bool {
	return true
}

func (m *mockMetricsService) getGetMetricsCalled() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getMetricsCalled
}

// mockMetricsServiceRegistry is a mock implementation of MetricsServiceRegistry
type mockMetricsServiceRegistry struct {
	services map[string]service.EnhancedMetricsService
	getErr   error
}

func (m *mockMetricsServiceRegistry) GetEnhancedMetricsService(namespace string) (service.EnhancedMetricsService, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if svc, ok := m.services[namespace]; ok {
		return svc, nil
	}
	return nil, errors.New("service not found")
}

func TestNewService(t *testing.T) {
	tests := []struct {
		name    string
		factory clients.Factory
		wantErr bool
		errMsg  string
	}{
		{
			name:    "success with factory implementing RegionalConfigProvider",
			factory: &mockFactory{},
			wantErr: false,
		},
		{
			name:    "failure with factory not implementing RegionalConfigProvider",
			factory: &mockNonRegionalFactory{},
			wantErr: true,
			errMsg:  "cannot create enhanced metric service with a factory type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewService(tt.factory, &mockMetricsServiceRegistry{})

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, svc)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, svc)
				require.NotNil(t, svc.configProvider)
			}
		})
	}
}

func TestService_GetMetrics(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	namespace := "AWS/RDS"
	region := "us-east-1"
	role := model.Role{RoleArn: "arn:aws:iam::123456789012:role/test"}
	resources := []*model.TaggedResource{
		{
			ARN:       "arn:aws:rds:us-east-1:123456789012:db:test",
			Namespace: namespace,
			Region:    region,
		},
	}
	metrics := []*model.EnhancedMetricConfig{
		{Name: "AllocatedStorage"},
	}
	exportedTags := []string{"Name"}

	tests := []struct {
		name                 string
		namespace            string
		registry             MetricsServiceRegistry
		wantErr              bool
		errMsg               string
		wantData             []*model.CloudwatchData
		wantGetMetricsCalled int
	}{
		{
			name:      "successfully get metrics",
			namespace: namespace,
			registry: &mockMetricsServiceRegistry{
				services: map[string]service.EnhancedMetricsService{
					namespace: &mockMetricsService{
						result: []*model.CloudwatchData{
							{
								MetricName:   "AllocatedStorage",
								ResourceName: "arn:aws:rds:us-east-1:123456789012:db:test",
								Namespace:    namespace,
							},
						},
					},
				},
			},
			wantErr: false,
			wantData: []*model.CloudwatchData{
				{
					MetricName:   "AllocatedStorage",
					ResourceName: "arn:aws:rds:us-east-1:123456789012:db:test",
					Namespace:    namespace,
				},
			},
			wantGetMetricsCalled: 1,
		},
		{
			name:      "failure when service not found in registry",
			namespace: namespace,
			registry: &mockMetricsServiceRegistry{
				services: map[string]service.EnhancedMetricsService{},
			},
			wantErr: true,
			errMsg:  "service not found",
		},
		{
			name:      "failure when service GetMetrics returns error",
			namespace: namespace,
			registry: &mockMetricsServiceRegistry{
				services: map[string]service.EnhancedMetricsService{
					namespace: &mockMetricsService{
						err: errors.New("get metric error"),
					},
				},
			},
			wantErr:              true,
			errMsg:               "get metric error",
			wantGetMetricsCalled: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewService(
				&mockFactory{},
				tt.registry,
			)
			require.NoError(t, err)

			data, err := svc.GetMetrics(ctx, logger, tt.namespace, resources, metrics, exportedTags, region, role)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, data)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantData, data)
			}

			if tt.wantGetMetricsCalled > 0 {
				mockSvc := tt.registry.(*mockMetricsServiceRegistry).services[tt.namespace].(*mockMetricsService)
				require.Equal(t, tt.wantGetMetricsCalled, mockSvc.getGetMetricsCalled())
			}
		})
	}
}
