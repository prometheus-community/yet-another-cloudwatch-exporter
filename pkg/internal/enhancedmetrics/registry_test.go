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
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// registryMockMetricsService is a mock implementation of service.EnhancedMetricsService for testing the registry
type registryMockMetricsService struct {
	namespace string
	procFunc  func(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, metrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) ([]*model.CloudwatchData, error)
}

func (m *registryMockMetricsService) Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, metrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) ([]*model.CloudwatchData, error) {
	if m.procFunc != nil {
		return m.procFunc(ctx, logger, namespace, resources, metrics, exportedTagOnMetrics, region, role, regionalConfigProvider)
	}
	return nil, nil
}

// registryMockMetricsServiceWrapper wraps the mock service to implement MetricsService interface
type registryMockMetricsServiceWrapper struct {
	namespace string
	service   *registryMockMetricsService
}

func (m *registryMockMetricsServiceWrapper) GetNamespace() string {
	return m.namespace
}

func (m *registryMockMetricsServiceWrapper) Instance() service.EnhancedMetricsService {
	return m.service
}

func newMockService(namespace string) *registryMockMetricsServiceWrapper {
	return &registryMockMetricsServiceWrapper{
		namespace: namespace,
		service: &registryMockMetricsService{
			namespace: namespace,
		},
	}
}

func TestRegistry_Register(t *testing.T) {
	t.Run("register single service", func(t *testing.T) {
		registry := &Registry{}
		mockSvc := newMockService("AWS/Test")

		result := registry.Register(mockSvc)

		assert.NotNil(t, result)
		assert.Equal(t, registry, result, "Register should return the registry for chaining")
		assert.NotNil(t, registry.services)
		assert.Contains(t, registry.services, "AWS/Test")
	})

	t.Run("register multiple services", func(t *testing.T) {
		registry := &Registry{}
		mockSvc1 := newMockService("AWS/Test1")
		mockSvc2 := newMockService("AWS/Test2")

		registry.Register(mockSvc1).Register(mockSvc2)

		assert.Len(t, registry.services, 2)
		assert.Contains(t, registry.services, "AWS/Test1")
		assert.Contains(t, registry.services, "AWS/Test2")
	})

	t.Run("replace existing service", func(t *testing.T) {
		registry := &Registry{}
		mockSvc1 := newMockService("AWS/Test")
		mockSvc2 := newMockService("AWS/Test")

		registry.Register(mockSvc1)
		registry.Register(mockSvc2)

		assert.Len(t, registry.services, 1)
		svc, err := registry.GetEnhancedMetricsService("AWS/Test")
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("register on nil services map", func(t *testing.T) {
		registry := &Registry{}
		mockSvc := newMockService("AWS/Test")

		assert.Nil(t, registry.services)
		registry.Register(mockSvc)
		assert.NotNil(t, registry.services)
	})
}

func TestRegistry_GetEnhancedMetricsService(t *testing.T) {
	t.Run("get existing service", func(t *testing.T) {
		registry := &Registry{}
		registry.Register(rds.NewRDSService(nil))

		svc, err := registry.GetEnhancedMetricsService("AWS/RDS")

		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("get non-existent service", func(t *testing.T) {
		registry := &Registry{}
		registry.Register(rds.NewRDSService(nil))

		svc, err := registry.GetEnhancedMetricsService("AWS/NonExistent")

		assert.Error(t, err)
		assert.Nil(t, svc)
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "AWS/NonExistent")
	})

	t.Run("get service from empty registry", func(t *testing.T) {
		registry := &Registry{}

		svc, err := registry.GetEnhancedMetricsService("AWS/Test")

		assert.Error(t, err)
		assert.Nil(t, svc)
	})

	t.Run("service instance is independent", func(t *testing.T) {
		registry := &Registry{}
		registry.Register(rds.NewRDSService(nil))
		svc1, err1 := registry.GetEnhancedMetricsService("AWS/RDS")
		svc2, err2 := registry.GetEnhancedMetricsService("AWS/RDS")

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotNil(t, svc1)
		assert.NotNil(t, svc2)

		// Each call to Instance() should return a new instance
		// This test verifies that the constructor function is being called

		// copy the pointer addresses to compare
		assert.NotSame(t, svc1, svc2, "Each call to GetEnhancedMetricsService should return a new instance")
	})
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Run("concurrent registration", func(t *testing.T) {
		registry := &Registry{}
		var wg sync.WaitGroup

		// Register multiple services concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				mockSvc := newMockService("AWS/Test" + string(rune('0'+idx)))
				registry.Register(mockSvc)
			}(i)
		}

		wg.Wait()
		assert.Len(t, registry.services, 10)
	})

	t.Run("concurrent read and write", func(t *testing.T) {
		registry := &Registry{}
		mockSvc := newMockService("AWS/Test")
		registry.Register(mockSvc)

		var wg sync.WaitGroup
		errors := make(chan error, 20)

		// Concurrent reads
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := registry.GetEnhancedMetricsService("AWS/Test")
				if err != nil {
					errors <- err
				}
			}()
		}

		// Concurrent writes
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				mockSvc := newMockService("AWS/NewTest" + string(rune('0'+idx)))
				registry.Register(mockSvc)
			}(i)
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			assert.NoError(t, err)
		}
	})
}

func TestDefaultRegistry(t *testing.T) {
	t.Run("default registry contains expected services", func(t *testing.T) {
		expectedNamespaces := []string{
			"AWS/RDS",
			"AWS/Lambda",
			"AWS/DynamoDB",
			"AWS/ElastiCache",
		}

		for _, namespace := range expectedNamespaces {
			svc, err := DefaultEnhancedMetricServiceRegistry.GetEnhancedMetricsService(namespace)
			assert.NoError(t, err, "Expected namespace %s to be registered", namespace)
			assert.NotNil(t, svc, "Expected service for namespace %s to be non-nil", namespace)
		}
	})

	t.Run("default registry returns error for unknown namespace", func(t *testing.T) {
		svc, err := DefaultEnhancedMetricServiceRegistry.GetEnhancedMetricsService("AWS/Unknown")
		assert.Error(t, err)
		assert.Nil(t, svc)
	})
}

func TestRegistry_ChainedRegistration(t *testing.T) {
	t.Run("chained registration", func(t *testing.T) {
		registry := (&Registry{}).
			Register(newMockService("AWS/Test1")).
			Register(newMockService("AWS/Test2")).
			Register(newMockService("AWS/Test3"))

		assert.Len(t, registry.services, 3)

		for i := 1; i <= 3; i++ {
			namespace := "AWS/Test" + string(rune('0'+i))
			svc, err := registry.GetEnhancedMetricsService(namespace)
			require.NoError(t, err)
			assert.NotNil(t, svc)
		}
	})
}

func TestRegistry_ServiceFactory(t *testing.T) {
	t.Run("service factory is called on each get", func(t *testing.T) {
		registry := &Registry{}
		callCount := 0

		// Create a wrapper that counts how many times Instance() is called
		wrapper := &registryMockMetricsServiceWrapper{
			namespace: "AWS/Test",
			service:   &registryMockMetricsService{namespace: "AWS/Test"},
		}

		// Override Instance to count calls
		originalInstance := wrapper.service
		customWrapper := &struct {
			MetricsService
			namespace string
			factory   func() service.EnhancedMetricsService
		}{
			namespace: "AWS/Test",
			factory: func() service.EnhancedMetricsService {
				callCount++
				return originalInstance
			},
		}

		registry.services = map[string]func() service.EnhancedMetricsService{
			"AWS/Test": customWrapper.factory,
		}

		// Call multiple times
		for i := 0; i < 3; i++ {
			svc, err := registry.GetEnhancedMetricsService("AWS/Test")
			require.NoError(t, err)
			assert.NotNil(t, svc)
		}

		assert.Equal(t, 3, callCount, "Factory should be called for each Get")
	})
}
