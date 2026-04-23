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
	"log/slog"
	"sync/atomic"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
	"github.com/prometheus/client_golang/prometheus"
)

type metricsBuilderFunc func(context.Context, *slog.Logger, model.JobsConfig, clients.Factory, ...exporter.OptionsFunc) ([]*promutil.PrometheusMetric, error)

type refreshingFactory interface {
	clients.Factory
	Refresh()
	Clear()
}

type metricSnapshot struct {
	metrics []*promutil.PrometheusMetric
}

type cachedCollector struct {
	snapshot atomic.Pointer[metricSnapshot]
}

func newCachedCollector() *cachedCollector {
	collector := &cachedCollector{}
	collector.snapshot.Store(&metricSnapshot{})
	return collector
}

func (c *cachedCollector) update(metrics []*promutil.PrometheusMetric) {
	c.snapshot.Store(&metricSnapshot{metrics: metrics})
}

func (c *cachedCollector) Describe(chan<- *prometheus.Desc) {}

func (c *cachedCollector) Collect(ch chan<- prometheus.Metric) {
	snapshot := c.snapshot.Load()
	if snapshot == nil || len(snapshot.metrics) == 0 {
		return
	}

	promutil.NewPrometheusCollector(snapshot.metrics).Collect(ch)
}
