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
	"testing"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	cloudwatchclient "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestCachedCollectorCollect(t *testing.T) {
	t.Parallel()

	collector := newCachedCollector()
	collector.update([]*promutil.PrometheusMetric{{
		Name:  "yace_test_metric",
		Value: 42,
	}})

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("registry.Register() error = %v", err)
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather() error = %v", err)
	}

	if !containsMetric(metricFamilies, "yace_test_metric") {
		t.Fatal("yace_test_metric not found in gathered metrics")
	}
}

func TestCachedCollectorCollectEmptySnapshot(t *testing.T) {
	t.Parallel()

	collector := newCachedCollector()

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("registry.Register() error = %v", err)
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather() error = %v", err)
	}
	if len(metricFamilies) != 0 {
		t.Fatalf("len(metricFamilies) = %d, want 0", len(metricFamilies))
	}
}

type stubFactory struct {
	refreshCount int
	clearCount   int
}

func (f *stubFactory) GetCloudwatchClient(string, model.Role, cloudwatchclient.ConcurrencyConfig) cloudwatchclient.Client {
	return nil
}

func (f *stubFactory) GetTaggingClient(string, model.Role, int) tagging.Client {
	return nil
}

func (f *stubFactory) GetAccountClient(string, model.Role) account.Client {
	return nil
}

func (f *stubFactory) Refresh() {
	f.refreshCount++
}

func (f *stubFactory) Clear() {
	f.clearCount++
}

func containsMetric(metricFamilies []*dto.MetricFamily, name string) bool {
	for _, metricFamily := range metricFamilies {
		if metricFamily.GetName() == name {
			return true
		}
	}
	return false
}
