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
	processCalled int
	processErr    error
	processResult []*model.CloudwatchData
	mu            sync.Mutex
}

func (m *mockMetricsService) Process(
	context.Context,
	*slog.Logger,
	string,
	[]*model.TaggedResource,
	[]*model.EnhancedMetricConfig,
	[]string,
	string,
	model.Role,
	config.RegionalConfigProvider,
) ([]*model.CloudwatchData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processCalled++
	return m.processResult, m.processErr
}

func (m *mockMetricsService) getProcessCalled() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processCalled
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

func TestNewProcessor(t *testing.T) {
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
			errMsg:  "cannot create enhanced metric processor with a factory type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := NewProcessor(tt.factory)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, processor)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, processor)
				require.NotNil(t, processor.ConfigProvider)
				require.NotNil(t, processor.EnhancedMetricsServices)
				require.Empty(t, processor.EnhancedMetricsServices)
			}
		})
	}
}

func TestProcessor_Process(t *testing.T) {
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
		name              string
		setupProcessor    func() *Processor
		setupRegistry     func() MetricsServiceRegistry
		namespace         string
		wantErr           bool
		errMsg            string
		wantData          []*model.CloudwatchData
		wantProcessCalled int
	}{
		{
			name: "successfully process metrics",
			setupProcessor: func() *Processor {
				expectedData := []*model.CloudwatchData{
					{
						MetricName:   "AllocatedStorage",
						ResourceName: "arn:aws:rds:us-east-1:123456789012:db:test",
						Namespace:    namespace,
					},
				}
				return &Processor{
					ConfigProvider: &mockFactory{},
					EnhancedMetricsServices: map[string]service.EnhancedMetricsService{
						namespace: &mockMetricsService{
							processResult: expectedData,
						},
					},
				}
			},
			setupRegistry: func() MetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					services: map[string]service.EnhancedMetricsService{
						namespace: &mockMetricsService{},
					},
				}
			},
			namespace: namespace,
			wantErr:   false,
			wantData: []*model.CloudwatchData{
				{
					MetricName:   "AllocatedStorage",
					ResourceName: "arn:aws:rds:us-east-1:123456789012:db:test",
					Namespace:    namespace,
				},
			},
			wantProcessCalled: 1,
		},
		{
			name: "failure when service not found in registry",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider:          &mockFactory{},
					EnhancedMetricsServices: make(map[string]service.EnhancedMetricsService),
				}
			},
			setupRegistry: func() MetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					services: map[string]service.EnhancedMetricsService{},
				}
			},
			namespace: namespace,
			wantErr:   true,
			errMsg:    "service not found",
		},
		{
			name: "failure when service Process returns error",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider: &mockFactory{},
					EnhancedMetricsServices: map[string]service.EnhancedMetricsService{
						namespace: &mockMetricsService{
							processErr: errors.New("process error"),
						},
					},
				}
			},
			setupRegistry: func() MetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					services: map[string]service.EnhancedMetricsService{
						namespace: &mockMetricsService{},
					},
				}
			},
			namespace:         namespace,
			wantErr:           true,
			errMsg:            "process error",
			wantProcessCalled: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := tt.setupProcessor()
			registry := tt.setupRegistry()

			data, err := processor.Process(ctx, logger, tt.namespace, resources, metrics, exportedTags, region, role, registry)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, data)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantData, data)
			}

			if tt.wantProcessCalled > 0 {
				svc := processor.EnhancedMetricsServices[tt.namespace].(*mockMetricsService)
				require.Equal(t, tt.wantProcessCalled, svc.getProcessCalled())
			}
		})
	}
}

func TestProcessor_Concurrency(t *testing.T) {
	// Test that the Processor can handle concurrent Process calls safely
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	namespace1 := "AWS/RDS"
	namespace2 := "AWS/ElastiCache"
	region := "us-east-1"
	role := model.Role{RoleArn: "arn:aws:iam::123456789012:role/test"}

	processor := &Processor{
		ConfigProvider:          &mockFactory{},
		EnhancedMetricsServices: make(map[string]service.EnhancedMetricsService),
	}

	registry := &mockMetricsServiceRegistry{
		services: map[string]service.EnhancedMetricsService{
			namespace1: &mockMetricsService{},
			namespace2: &mockMetricsService{},
		},
	}

	resources := []*model.TaggedResource{
		{
			ARN:       "arn:aws:rds:us-east-1:123456789012:db:test",
			Namespace: namespace1,
			Region:    region,
		},
	}
	metrics := []*model.EnhancedMetricConfig{{Name: "TestMetric"}}
	exportedTags := []string{"Name"}

	var wg sync.WaitGroup
	errChan := make(chan error, 40)

	// Run multiple goroutines to test concurrent Process calls
	for i := 0; i < 10; i++ {
		wg.Add(4)

		go func() {
			defer wg.Done()
			_, err := processor.Process(ctx, logger, namespace1, resources, metrics, exportedTags, region, role, registry)
			if err != nil {
				errChan <- err
			}
		}()

		go func() {
			defer wg.Done()
			_, err := processor.Process(ctx, logger, namespace2, resources, metrics, exportedTags, region, role, registry)
			if err != nil {
				errChan <- err
			}
		}()

		go func() {
			defer wg.Done()
			_, err := processor.Process(ctx, logger, namespace1, resources, metrics, exportedTags, region, role, registry)
			if err != nil {
				errChan <- err
			}
		}()

		go func() {
			defer wg.Done()
			_, err := processor.Process(ctx, logger, namespace2, resources, metrics, exportedTags, region, role, registry)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "Should not have errors during concurrent Process calls")

	// Verify both services were initialized
	require.Len(t, processor.EnhancedMetricsServices, 2)
	require.Contains(t, processor.EnhancedMetricsServices, namespace1)
	require.Contains(t, processor.EnhancedMetricsServices, namespace2)
}
