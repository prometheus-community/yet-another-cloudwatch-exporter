package enhancedmetrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// mockMetricsService is a mock implementation of service.MetricsService
type mockMetricsService struct {
	loadMetadataCalled int
	processCalled      int
	loadMetadataErr    error
	processErr         error
	processResult      []*model.CloudwatchData
	mu                 sync.Mutex
}

func (m *mockMetricsService) LoadMetricsMetadata(context.Context, *slog.Logger, string, model.Role, config.RegionalConfigProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadMetadataCalled++
	return m.loadMetadataErr
}

func (m *mockMetricsService) Process(context.Context, *slog.Logger, string, []*model.TaggedResource, []*model.EnhancedMetricConfig, []string) ([]*model.CloudwatchData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processCalled++
	return m.processResult, m.processErr
}

func (m *mockMetricsService) getLoadMetadataCalled() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadMetadataCalled
}

func (m *mockMetricsService) getProcessCalled() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processCalled
}

// mockMetricsServiceRegistry is a mock implementation of MetricsServiceRegistry
type mockMetricsServiceRegistry struct {
	services map[string]service.MetricsService
	getErr   error
}

func (m *mockMetricsServiceRegistry) GetEnhancedMetricsService(namespace string) (service.MetricsService, error) {
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

func TestProcessor_LoadMetricsMetadata(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	region := "us-east-1"
	role := model.Role{RoleArn: "arn:aws:iam::123456789012:role/test"}
	namespace := "AWS/RDS"

	tests := []struct {
		name                   string
		setupProcessor         func() *Processor
		setupRegistry          func() *mockMetricsServiceRegistry
		namespace              string
		wantErr                bool
		errMsg                 string
		wantServiceInitialized bool
		wantMetadataCallCount  int
	}{
		{
			name: "successfully load metadata for new namespace",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider:          &mockFactory{},
					EnhancedMetricsServices: make(map[string]service.MetricsService),
				}
			},
			setupRegistry: func() *mockMetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					services: map[string]service.MetricsService{
						namespace: &mockMetricsService{},
					},
				}
			},
			namespace:              namespace,
			wantErr:                false,
			wantServiceInitialized: true,
			wantMetadataCallCount:  1,
		},
		{
			name: "successfully load metadata for already initialized namespace",
			setupProcessor: func() *Processor {
				svc := &mockMetricsService{}
				return &Processor{
					ConfigProvider: &mockFactory{},
					EnhancedMetricsServices: map[string]service.MetricsService{
						namespace: svc,
					},
				}
			},
			setupRegistry: func() *mockMetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					services: map[string]service.MetricsService{
						namespace: &mockMetricsService{},
					},
				}
			},
			namespace:              namespace,
			wantErr:                false,
			wantServiceInitialized: true,
			wantMetadataCallCount:  1,
		},
		{
			name: "failure when registry cannot get service",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider:          &mockFactory{},
					EnhancedMetricsServices: make(map[string]service.MetricsService),
				}
			},
			setupRegistry: func() *mockMetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					getErr: errors.New("registry error"),
				}
			},
			namespace: namespace,
			wantErr:   true,
			errMsg:    "ensureServiceInitialized error",
		},
		{
			name: "failure when service LoadMetricsMetadata returns error",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider:          &mockFactory{},
					EnhancedMetricsServices: make(map[string]service.MetricsService),
				}
			},
			setupRegistry: func() *mockMetricsServiceRegistry {
				return &mockMetricsServiceRegistry{
					services: map[string]service.MetricsService{
						namespace: &mockMetricsService{
							loadMetadataErr: errors.New("load metadata error"),
						},
					},
				}
			},
			namespace:              namespace,
			wantErr:                true,
			errMsg:                 "load metadata error",
			wantServiceInitialized: true,
			wantMetadataCallCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := tt.setupProcessor()
			registry := tt.setupRegistry()

			err := processor.LoadMetricsMetadata(ctx, logger, region, role, tt.namespace, registry)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}

			if tt.wantServiceInitialized {
				require.Contains(t, processor.EnhancedMetricsServices, tt.namespace)
				if tt.wantMetadataCallCount > 0 {
					svc := processor.EnhancedMetricsServices[tt.namespace].(*mockMetricsService)
					require.Equal(t, tt.wantMetadataCallCount, svc.getLoadMetadataCalled())
				}
			}
		})
	}
}

func TestProcessor_Process(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	namespace := "AWS/RDS"
	resources := []*model.TaggedResource{
		{
			ARN:       "arn:aws:rds:us-east-1:123456789012:db:test",
			Namespace: namespace,
			Region:    "us-east-1",
		},
	}
	metrics := []*model.EnhancedMetricConfig{
		{Name: "AllocatedStorage"},
	}
	exportedTags := []string{"Name"}

	tests := []struct {
		name              string
		setupProcessor    func() *Processor
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
					EnhancedMetricsServices: map[string]service.MetricsService{
						namespace: &mockMetricsService{
							processResult: expectedData,
						},
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
			name: "failure when service not initialized",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider:          &mockFactory{},
					EnhancedMetricsServices: make(map[string]service.MetricsService),
				}
			},
			namespace: namespace,
			wantErr:   true,
			errMsg:    "not initialized",
		},
		{
			name: "failure when service Process returns error",
			setupProcessor: func() *Processor {
				return &Processor{
					ConfigProvider: &mockFactory{},
					EnhancedMetricsServices: map[string]service.MetricsService{
						namespace: &mockMetricsService{
							processErr: errors.New("process error"),
						},
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

			data, err := processor.Process(ctx, logger, tt.namespace, resources, metrics, exportedTags)

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
	// Test that the Processor can handle concurrent LoadMetricsMetadata and Process calls safely
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	namespace1 := "AWS/RDS"
	namespace2 := "AWS/ElastiCache"
	region := "us-east-1"
	role := model.Role{RoleArn: "arn:aws:iam::123456789012:role/test"}

	processor := &Processor{
		ConfigProvider:          &mockFactory{},
		EnhancedMetricsServices: make(map[string]service.MetricsService),
	}

	registry := &mockMetricsServiceRegistry{
		services: map[string]service.MetricsService{
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
	errChan := make(chan error, 100)

	// Run multiple goroutines to test concurrent access
	for i := 0; i < 10; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			err := processor.LoadMetricsMetadata(ctx, logger, region, role, namespace1, registry)
			if err != nil {
				errChan <- err
			}
		}()

		go func() {
			defer wg.Done()
			err := processor.LoadMetricsMetadata(ctx, logger, region, role, namespace2, registry)
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
	require.Empty(t, errs, "Should not have errors during concurrent LoadMetricsMetadata calls")

	// Verify both services were initialized
	require.Len(t, processor.EnhancedMetricsServices, 2)
	require.Contains(t, processor.EnhancedMetricsServices, namespace1)
	require.Contains(t, processor.EnhancedMetricsServices, namespace2)

	// Now test concurrent Process calls
	errChan = make(chan error, 20)
	for i := 0; i < 10; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			_, err := processor.Process(ctx, logger, namespace1, resources, metrics, exportedTags)
			if err != nil {
				errChan <- err
			}
		}()

		go func() {
			defer wg.Done()
			_, err := processor.Process(ctx, logger, namespace2, resources, metrics, exportedTags)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	errs = nil
	for err := range errChan {
		errs = append(errs, err)
	}
	require.Empty(t, errs, "Should not have errors during concurrent Process calls")
}
