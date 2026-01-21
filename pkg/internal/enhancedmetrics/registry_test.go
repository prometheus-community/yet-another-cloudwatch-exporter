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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
)

// registryMockMetricsServiceWrapper wraps the mock service to implement MetricsService interface
type registryMockMetricsServiceWrapper struct {
	namespace    string
	instanceFunc func() service.EnhancedMetricsService
}

func (m *registryMockMetricsServiceWrapper) GetNamespace() string {
	return m.namespace
}

func (m *registryMockMetricsServiceWrapper) Instance() service.EnhancedMetricsService {
	if m.instanceFunc != nil {
		return m.instanceFunc()
	}
	return nil
}

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() *Registry
		services   []string
		assertions func(t *testing.T, registry *Registry)
	}{
		{
			name:     "register single service",
			setup:    func() *Registry { return &Registry{} },
			services: []string{"AWS/Test"},
			assertions: func(t *testing.T, registry *Registry) {
				assert.NotNil(t, registry.services)
				assert.Contains(t, registry.services, "AWS/Test")
				assert.Len(t, registry.services, 1)
			},
		},
		{
			name:     "register multiple services",
			setup:    func() *Registry { return &Registry{} },
			services: []string{"AWS/Test1", "AWS/Test2"},
			assertions: func(t *testing.T, registry *Registry) {
				assert.Len(t, registry.services, 2)
				assert.Contains(t, registry.services, "AWS/Test1")
				assert.Contains(t, registry.services, "AWS/Test2")
			},
		},
		{
			name:     "replace existing service",
			setup:    func() *Registry { return &Registry{} },
			services: []string{"AWS/Test", "AWS/Test"},
			assertions: func(t *testing.T, registry *Registry) {
				assert.Len(t, registry.services, 1)
				_, err := registry.GetEnhancedMetricsService("AWS/Test")
				require.NoError(t, err)
			},
		},
		{
			name:     "register on nil services map",
			setup:    func() *Registry { return &Registry{} },
			services: []string{"AWS/Test"},
			assertions: func(t *testing.T, registry *Registry) {
				assert.NotNil(t, registry.services)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()

			var result *Registry
			for _, ns := range tt.services {
				mockSvc := &registryMockMetricsServiceWrapper{
					namespace: ns,
				}
				result = registry.Register(mockSvc)
			}

			assert.NotNil(t, result)
			assert.Equal(t, registry, result, "Register should return the registry for chaining")
			tt.assertions(t, registry)
		})
	}
}

func TestRegistry_GetEnhancedMetricsService(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *Registry
		namespace   string
		expectError bool
		error       string
	}{
		{
			name: "get existing service",
			setup: func() *Registry {
				registry := &Registry{}
				registry.Register(rds.NewRDSService(nil))
				return registry
			},
			namespace:   "AWS/RDS",
			expectError: false,
		},
		{
			name: "get non-existent service",
			setup: func() *Registry {
				registry := &Registry{}
				registry.Register(rds.NewRDSService(nil))
				return registry
			},
			namespace:   "AWS/NonExistent",
			expectError: true,
			error:       "enhanced metrics service for namespace AWS/NonExistent not found",
		},
		{
			name: "get service from empty registry",
			setup: func() *Registry {
				return &Registry{}
			},
			namespace:   "AWS/Test",
			error:       "enhanced metrics service for namespace AWS/Test not found",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := tt.setup()
			svc, err := registry.GetEnhancedMetricsService(tt.namespace)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, err.Error(), tt.error)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, svc)
			}
		})
	}

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
				mockSvc := &registryMockMetricsServiceWrapper{
					namespace: "AWS/Test" + string(rune('0'+idx)),
				}
				registry.Register(mockSvc)
			}(i)
		}

		wg.Wait()
		assert.Len(t, registry.services, 10)
	})

	t.Run("concurrent read and write", func(t *testing.T) {
		registry := &Registry{}
		mockSvc := &registryMockMetricsServiceWrapper{
			namespace: "AWS/Test",
		}
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
				mockSvc := &registryMockMetricsServiceWrapper{
					namespace: "AWS/NewTest" + string(rune('0'+idx)),
				}
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
	tests := []struct {
		name        string
		namespace   string
		expectError bool
	}{
		{
			name:        "AWS/RDS is registered",
			namespace:   "AWS/RDS",
			expectError: false,
		},
		{
			name:        "AWS/Lambda is registered",
			namespace:   "AWS/Lambda",
			expectError: false,
		},
		{
			name:        "AWS/DynamoDB is registered",
			namespace:   "AWS/DynamoDB",
			expectError: false,
		},
		{
			name:        "AWS/ElastiCache is registered",
			namespace:   "AWS/ElastiCache",
			expectError: false,
		},
		{
			name:        "unknown namespace returns error",
			namespace:   "AWS/Unknown",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := DefaultEnhancedMetricServiceRegistry.GetEnhancedMetricsService(tt.namespace)

			assert.Len(t, DefaultEnhancedMetricServiceRegistry.services, 4, "Expected 4 services to be registered in the default registry")
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, svc)
			} else {
				assert.NoError(t, err, "Expected namespace %s to be registered", tt.namespace)
				assert.NotNil(t, svc, "Expected service for namespace %s to be non-nil", tt.namespace)
			}
		})
	}
}

func TestRegistry_ChainedRegistration(t *testing.T) {
	t.Run("chained registration", func(t *testing.T) {
		registry := (&Registry{}).
			Register(&registryMockMetricsServiceWrapper{
				namespace: "AWS/Test1",
			}).
			Register(&registryMockMetricsServiceWrapper{
				namespace: "AWS/Test2",
			}).
			Register(&registryMockMetricsServiceWrapper{
				namespace: "AWS/Test3",
			})

		assert.Len(t, registry.services, 3)

		for i := 1; i <= 3; i++ {
			namespace := "AWS/Test" + string(rune('0'+i))
			_, err := registry.GetEnhancedMetricsService(namespace)
			require.NoError(t, err)
		}
	})
}

func TestRegistry_ServiceFactory(t *testing.T) {
	t.Run("service factory is called on each get", func(t *testing.T) {
		registry := &Registry{}
		callCount := 0

		registry.services = map[string]func() service.EnhancedMetricsService{
			"AWS/Test": func() service.EnhancedMetricsService {
				callCount++
				return nil
			},
		}

		// Call multiple times
		for i := 0; i < 3; i++ {
			_, _ = registry.GetEnhancedMetricsService("AWS/Test")
		}

		assert.Equal(t, 3, callCount, "Factory should be called for each Get")
	})
}
