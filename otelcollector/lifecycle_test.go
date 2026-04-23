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

package otelcollector

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLifecycleManagerStart(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		factory := &stubFactory{}
		var buildCalls atomic.Int32
		mgr := newLifecycleManager()
		mgr.newFactory = func(*slog.Logger, model.JobsConfig, bool) (refreshingFactory, error) {
			return factory, nil
		}
		mgr.buildMetrics = func(context.Context, *slog.Logger, model.JobsConfig, clients.Factory, ...exporter.OptionsFunc) ([]*promutil.PrometheusMetric, error) {
			buildCalls.Add(1)
			return []*promutil.PrometheusMetric{{
				Name:  "yace_test_metric",
				Value: 7,
			}}, nil
		}

		cfg := validConfig()
		cfg.AWSScrapeInterval = "10ms"
		settings := receivertest.NewNopSettings(receiverType)

		registry, err := mgr.Start(context.Background(), settings, cfg)
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if registry == nil {
			t.Fatal("Start() returned nil registry")
		}

		metricFamilies, err := registry.Gather()
		if err != nil {
			t.Fatalf("registry.Gather() error = %v", err)
		}
		if buildCalls.Load() < 1 {
			t.Fatalf("buildCalls = %d, want at least 1", buildCalls.Load())
		}
		if !containsMetric(metricFamilies, "yace_test_metric") {
			t.Fatal("yace_test_metric not found in gathered metrics")
		}

		synctest.Wait()
		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		if buildCalls.Load() < 2 {
			t.Fatalf("buildCalls = %d, want at least 2 after background loop", buildCalls.Load())
		}

		if err := mgr.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if factory.clearCount < 1 {
			t.Fatalf("factory.clearCount = %d, want at least 1", factory.clearCount)
		}
	})
}

func TestLifecycleManagerStartErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  prombridge.Config
		mgr  *lifecycleManager
	}{
		{
			name: "invalid config type",
			cfg:  fakeConfig{},
			mgr:  newLifecycleManager(),
		},
		{
			name: "factory construction error",
			cfg:  validConfig(),
			mgr: func() *lifecycleManager {
				mgr := newLifecycleManager()
				mgr.newFactory = func(*slog.Logger, model.JobsConfig, bool) (refreshingFactory, error) {
					return nil, errors.New("boom")
				}
				return mgr
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry, err := tt.mgr.Start(context.Background(), receivertest.NewNopSettings(receiverType), tt.cfg)
			if err == nil {
				t.Fatal("Start() error = nil, want non-nil")
			}
			if registry != nil {
				t.Fatal("Start() registry != nil, want nil")
			}
		})
	}
}

func TestLifecycleManagerStartUsesReceiverLogger(t *testing.T) {
	t.Parallel()

	observedCore, observedLogs := observer.New(zapcore.ErrorLevel)
	settings := receivertest.NewNopSettings(receiverType)
	settings.Logger = zap.New(observedCore)

	mgr := newLifecycleManager()
	mgr.newFactory = func(*slog.Logger, model.JobsConfig, bool) (refreshingFactory, error) {
		return &stubFactory{}, nil
	}
	mgr.buildMetrics = func(context.Context, *slog.Logger, model.JobsConfig, clients.Factory, ...exporter.OptionsFunc) ([]*promutil.PrometheusMetric, error) {
		return nil, errors.New("boom")
	}

	cfg := validConfig()
	cfg.AWSScrapeInterval = "1h"

	registry, err := mgr.Start(context.Background(), settings, cfg)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if registry == nil {
		t.Fatal("Start() returned nil registry")
	}
	if err := mgr.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	entries := observedLogs.FilterMessage("initial YACE scrape failed").All()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
}

type fakeConfig struct{}

func (fakeConfig) Validate() error { return nil }
