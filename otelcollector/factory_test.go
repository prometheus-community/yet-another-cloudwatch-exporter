// Copyright The Prometheus Authors
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
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	if factory == nil {
		t.Fatal("NewFactory() returned nil")
	}
	if factory.Type() != receiverType {
		t.Fatalf("factory.Type() = %q, want %q", factory.Type(), receiverType)
	}
}

func TestFactoryCreateMetrics(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*prombridge.ReceiverConfig)
	cfg.ExporterConfig = map[string]interface{}{
		"config_file": validConfigFile(),
	}

	recv, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(receiverType),
		cfg,
		new(consumertest.MetricsSink),
	)
	if err != nil {
		t.Fatalf("CreateMetrics() error = %v", err)
	}
	if recv == nil {
		t.Fatal("CreateMetrics() returned nil")
	}

	exporterCfg, ok := cfg.GetExporterConfig().(*Config)
	if !ok {
		t.Fatalf("GetExporterConfig() type = %T, want *Config", cfg.GetExporterConfig())
	}
	if exporterCfg.ConfigFile != validConfigFile() {
		t.Fatalf("exporterCfg.ConfigFile = %q, want %q", exporterCfg.ConfigFile, validConfigFile())
	}
}

func TestFactoryCreateMetrics_DecodeCloudwatchConcurrency(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*prombridge.ReceiverConfig)
	cfg.ExporterConfig = map[string]interface{}{
		"config_file": validConfigFile(),
		"cloudwatch_concurrency": map[string]interface{}{
			"single_limit":          17,
			"per_api_limit_enabled": true,
			"list_metrics":          11,
			"get_metric_data":       13,
			"get_metric_statistics": 15,
		},
	}

	recv, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(receiverType),
		cfg,
		new(consumertest.MetricsSink),
	)
	if err != nil {
		t.Fatalf("CreateMetrics() error = %v", err)
	}
	if recv == nil {
		t.Fatal("CreateMetrics() returned nil")
	}

	exporterCfg, ok := cfg.GetExporterConfig().(*Config)
	if !ok {
		t.Fatalf("GetExporterConfig() type = %T, want *Config", cfg.GetExporterConfig())
	}

	if exporterCfg.CloudwatchConcurrency.SingleLimit != 17 {
		t.Fatalf("SingleLimit = %d, want 17", exporterCfg.CloudwatchConcurrency.SingleLimit)
	}
	if !exporterCfg.CloudwatchConcurrency.PerAPILimitEnabled {
		t.Fatal("PerAPILimitEnabled = false, want true")
	}
	if exporterCfg.CloudwatchConcurrency.ListMetrics != 11 {
		t.Fatalf("ListMetrics = %d, want 11", exporterCfg.CloudwatchConcurrency.ListMetrics)
	}
	if exporterCfg.CloudwatchConcurrency.GetMetricData != 13 {
		t.Fatalf("GetMetricData = %d, want 13", exporterCfg.CloudwatchConcurrency.GetMetricData)
	}
	if exporterCfg.CloudwatchConcurrency.GetMetricStatistics != 15 {
		t.Fatalf("GetMetricStatistics = %d, want 15", exporterCfg.CloudwatchConcurrency.GetMetricStatistics)
	}
}

func TestFactoryCreateMetrics_UsesDistinctLifecycleManagers(t *testing.T) {
	t.Parallel()

	var managers []*trackingLifecycleManager
	factory := newFactoryWithLifecycleManagerBuilder(func() prombridge.ExporterLifecycleManager {
		manager := &trackingLifecycleManager{}
		managers = append(managers, manager)
		return manager
	})

	cfgA := factory.CreateDefaultConfig().(*prombridge.ReceiverConfig)
	cfgA.ExporterConfig = map[string]interface{}{
		"config_file": validConfigFile(),
	}

	cfgB := factory.CreateDefaultConfig().(*prombridge.ReceiverConfig)
	cfgB.ExporterConfig = map[string]interface{}{
		"config_file": validConfigFile(),
	}

	recvA, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(receiverType),
		cfgA,
		new(consumertest.MetricsSink),
	)
	if err != nil {
		t.Fatalf("CreateMetrics() for receiver A error = %v", err)
	}

	recvB, err := factory.CreateMetrics(
		context.Background(),
		receivertest.NewNopSettings(receiverType),
		cfgB,
		new(consumertest.MetricsSink),
	)
	if err != nil {
		t.Fatalf("CreateMetrics() for receiver B error = %v", err)
	}

	if len(managers) != 2 {
		t.Fatalf("len(managers) = %d, want 2", len(managers))
	}

	host := componenttest.NewNopHost()
	if err := recvA.Start(context.Background(), host); err != nil {
		t.Fatalf("receiver A Start() error = %v", err)
	}
	if err := recvB.Start(context.Background(), host); err != nil {
		t.Fatalf("receiver B Start() error = %v", err)
	}

	if managers[0].startCalls != 1 {
		t.Fatalf("receiver A startCalls = %d, want 1", managers[0].startCalls)
	}
	if managers[1].startCalls != 1 {
		t.Fatalf("receiver B startCalls = %d, want 1", managers[1].startCalls)
	}

	if err := recvA.Shutdown(context.Background()); err != nil {
		t.Fatalf("receiver A Shutdown() error = %v", err)
	}

	if managers[0].shutdownCalls != 1 {
		t.Fatalf("receiver A shutdownCalls = %d, want 1", managers[0].shutdownCalls)
	}
	if managers[1].shutdownCalls != 0 {
		t.Fatalf("receiver B shutdownCalls = %d, want 0 before receiver B shutdown", managers[1].shutdownCalls)
	}

	if err := recvB.Shutdown(context.Background()); err != nil {
		t.Fatalf("receiver B Shutdown() error = %v", err)
	}
	if managers[1].shutdownCalls != 1 {
		t.Fatalf("receiver B shutdownCalls = %d, want 1", managers[1].shutdownCalls)
	}
}

type trackingLifecycleManager struct {
	startCalls    int
	shutdownCalls int
}

func (m *trackingLifecycleManager) Start(context.Context, receiver.Settings, prombridge.Config) (*prometheus.Registry, error) {
	m.startCalls++
	return prometheus.NewRegistry(), nil
}

func (m *trackingLifecycleManager) Shutdown(context.Context) error {
	m.shutdownCalls++
	return nil
}
