// Copyright 2024 The Prometheus Authors
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
package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type scraper struct {
	registry     atomic.Pointer[prometheus.Registry]
	featureFlags []string
}

type cachingFactory interface {
	clients.Factory
	Refresh()
	Clear()
}

func NewScraper(featureFlags []string) *scraper { //nolint:revive
	s := &scraper{
		registry:     atomic.Pointer[prometheus.Registry]{},
		featureFlags: featureFlags,
	}
	s.registry.Store(prometheus.NewRegistry())
	return s
}

func (s *scraper) makeHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handler := promhttp.HandlerFor(s.registry.Load(), promhttp.HandlerOpts{
			DisableCompression: false,
		})
		handler.ServeHTTP(w, r)
	}
}

func (s *scraper) decoupled(ctx context.Context, logger logging.Logger, jobsCfg model.JobsConfig, cache cachingFactory) {
	logger.Debug("Starting scraping async")
	s.scrape(ctx, logger, jobsCfg, cache)

	scrapingDuration := time.Duration(scrapingInterval) * time.Second
	ticker := time.NewTicker(scrapingDuration)
	logger.Debug("Initial scrape completed", "scraping_interval", scrapingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logger.Debug("Starting scraping async")
			go s.scrape(ctx, logger, jobsCfg, cache)
		}
	}
}

func (s *scraper) scrape(ctx context.Context, logger logging.Logger, jobsCfg model.JobsConfig, cache cachingFactory) {
	if !sem.TryAcquire(1) {
		// This shouldn't happen under normal use, users should adjust their configuration when this occurs.
		// Let them know by logging a warning.
		logger.Warn("Another scrape is already in process, will not start a new one. " +
			"Adjust your configuration to ensure the previous scrape completes first.")
		return
	}
	defer sem.Release(1)

	newRegistry := prometheus.NewRegistry()
	for _, metric := range exporter.Metrics {
		if err := newRegistry.Register(metric); err != nil {
			logger.Warn("Could not register cloudwatch api metric")
		}
	}

	// since we have called refresh, we have loaded all the credentials
	// into the clients and it is now safe to call concurrently. Defer the
	// clearing, so we always clear credentials before the next scrape
	cache.Refresh()
	defer cache.Clear()

	options := []exporter.OptionsFunc{
		exporter.MetricsPerQuery(metricsPerQuery),
		exporter.LabelsSnakeCase(labelsSnakeCase),
		exporter.EnableFeatureFlag(s.featureFlags...),
		exporter.TaggingAPIConcurrency(tagConcurrency),
	}

	if cloudwatchConcurrency.PerAPILimitEnabled {
		options = append(options, exporter.CloudWatchPerAPILimitConcurrency(cloudwatchConcurrency.ListMetrics, cloudwatchConcurrency.GetMetricData, cloudwatchConcurrency.GetMetricStatistics))
	} else {
		options = append(options, exporter.CloudWatchAPIConcurrency(cloudwatchConcurrency.SingleLimit))
	}

	err := exporter.UpdateMetrics(
		ctx,
		logger,
		jobsCfg,
		newRegistry,
		cache,
		options...,
	)
	if err != nil {
		logger.Error(err, "error updating metrics")
	}

	s.registry.Store(newRegistry)
	logger.Debug("Metrics scraped")
}
