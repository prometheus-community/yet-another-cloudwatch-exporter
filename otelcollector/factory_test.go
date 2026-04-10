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
	"testing"

	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
