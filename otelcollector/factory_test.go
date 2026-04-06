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
		"discovery": map[string]interface{}{
			"jobs": []map[string]interface{}{
				{
					"regions": []string{"us-east-1"},
					"type":    "AWS/EC2",
					"metrics": []map[string]interface{}{
						{
							"name":       "CPUUtilization",
							"statistics": []string{"Average"},
							"period":     300,
							"length":     300,
						},
					},
				},
			},
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
	if len(exporterCfg.Discovery.Jobs) != 1 {
		t.Fatalf("len(exporterCfg.Discovery.Jobs) = %d, want 1", len(exporterCfg.Discovery.Jobs))
	}
}
