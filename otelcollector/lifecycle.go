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
	"fmt"
	"log/slog"
	"sync"
	"time"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/exp/zapslog"
)

type factoryBuilderFunc func(*slog.Logger, model.JobsConfig, bool) (refreshingFactory, error)

type lifecycleManager struct {
	logger       *slog.Logger
	newFactory   factoryBuilderFunc
	buildMetrics metricsBuilderFunc

	cancel    context.CancelFunc
	wg        sync.WaitGroup
	factory   refreshingFactory
	collector *cachedCollector
}

type scrapeSession struct {
	ctx     context.Context
	jobsCfg model.JobsConfig
	options []exporter.OptionsFunc
}

func newLifecycleManager() *lifecycleManager {
	return &lifecycleManager{
		newFactory: func(logger *slog.Logger, jobsCfg model.JobsConfig, fips bool) (refreshingFactory, error) {
			return clients.NewFactory(logger, jobsCfg, fips)
		},
		buildMetrics: exporter.BuildPrometheusMetrics,
	}
}

func (m *lifecycleManager) Start(ctx context.Context, set receiver.Settings, exporterConfig prombridge.Config) (*prometheus.Registry, error) {
	cfg, ok := exporterConfig.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid exporter config type: %T", exporterConfig)
	}

	logger := loggerFromSettings(set)
	m.logger = logger

	jobsCfg, err := cfg.jobsConfig(logger)
	if err != nil {
		return nil, err
	}

	factory, err := m.newFactory(logger, jobsCfg, cfg.FIPSEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to construct aws client factory: %w", err)
	}

	options, err := cfg.exporterOptions()
	if err != nil {
		return nil, err
	}

	interval, err := cfg.awsScrapeInterval()
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	collector := newCachedCollector()

	registry := prometheus.NewRegistry()
	for _, metric := range exporter.Metrics {
		if err := registry.Register(metric); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to register exporter metric %T: %w", metric, err)
		}
	}
	if err := registry.Register(collector); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to register yace collector: %w", err)
	}

	session := scrapeSession{
		ctx:     runCtx,
		jobsCfg: jobsCfg,
		options: append([]exporter.OptionsFunc(nil), options...),
	}

	if err := m.scrapeOnce(session, collector, factory); err != nil {
		m.logger.Error("initial YACE scrape failed", "err", err)
	}

	if m.cancel != nil {
		m.cancel()
	}
	m.cancel = cancel
	m.factory = factory
	m.collector = collector
	m.wg.Add(1)
	go m.runLoop(session, collector, factory, interval)

	return registry, nil
}

func (m *lifecycleManager) Shutdown(_ context.Context) error {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.wg.Wait()
	if m.factory != nil {
		m.factory.Clear()
		m.factory = nil
	}
	m.collector = nil
	return nil
}

func (m *lifecycleManager) runLoop(session scrapeSession, collector *cachedCollector, factory refreshingFactory, interval time.Duration) {
	defer m.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-session.ctx.Done():
			return
		case <-ticker.C:
			if err := m.scrapeOnce(session, collector, factory); err != nil {
				m.logger.Error("background YACE scrape failed", "err", err)
			}
		}
	}
}

func (m *lifecycleManager) scrapeOnce(session scrapeSession, collector *cachedCollector, factory refreshingFactory) error {
	factory.Refresh()
	defer factory.Clear()

	metrics, err := m.buildMetrics(session.ctx, m.logger, session.jobsCfg, factory, session.options...)
	if err != nil {
		return err
	}

	collector.update(metrics)
	return nil
}

func loggerFromSettings(set receiver.Settings) *slog.Logger {
	if set.Logger == nil {
		return discardLogger()
	}
	return slog.New(zapslog.NewHandler(set.Logger.Core()))
}
