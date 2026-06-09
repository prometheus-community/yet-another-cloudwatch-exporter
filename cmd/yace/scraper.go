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
package main

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	yacemetrics "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/metrics"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// Scraper holds two registries served together via prometheus.Gatherers:
//   - stableReg: scrape instrumentation counters that accumulate across scrapes.
//   - resultReg: latest-scrape result collector, swapped atomically each scrape.
type Scraper struct {
	stableReg     *prometheus.Registry
	resultReg     atomic.Pointer[prometheus.Registry]
	scrapeMetrics *promutil.ScrapeMetrics
	config        config.Config
}

type cachingFactory interface {
	clients.Factory
	Refresh()
	Clear()
}

func NewScraper(cfg config.Config) *Scraper {
	cfg.FeatureFlags = append([]string(nil), cfg.FeatureFlags...)
	stableReg := prometheus.NewRegistry()
	s := &Scraper{
		stableReg:     stableReg,
		scrapeMetrics: promutil.NewScrapeMetrics(stableReg),
		config:        cfg,
	}
	s.resultReg.Store(prometheus.NewRegistry())
	return s
}

func (s *Scraper) makeHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		gatherers := prometheus.Gatherers{s.stableReg, s.resultReg.Load()}
		handler := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{
			DisableCompression: false,
		})
		handler.ServeHTTP(w, r)
	}
}

func (s *Scraper) decoupled(ctx context.Context, logger *slog.Logger, jobsCfg model.JobsConfig, cache cachingFactory) {
	metricsScraper, err := yacemetrics.NewScraper(logger, s.config, jobsCfg, cache, s.scrapeMetrics)
	if err != nil {
		logger.Error("invalid runtime scrape configuration", "err", err)
		return
	}

	logger.Debug("Starting scraping async")
	s.scrape(ctx, logger, metricsScraper, cache)

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
			go s.scrape(ctx, logger, metricsScraper, cache)
		}
	}
}

func (s *Scraper) scrape(ctx context.Context, logger *slog.Logger, scraper *yacemetrics.Scraper, cache cachingFactory) {
	if !sem.TryAcquire(1) {
		// This shouldn't happen under normal use, users should adjust their configuration when this occurs.
		// Let them know by logging a warning.
		logger.Warn("Another scrape is already in process, will not start a new one. " +
			"Adjust your configuration to ensure the previous scrape completes first.")
		return
	}
	defer sem.Release(1)

	// since we have called refresh, we have loaded all the credentials
	// into the clients and it is now safe to call concurrently. Defer the
	// clearing, so we always clear credentials before the next scrape
	cache.Refresh()
	defer cache.Clear()

	metrics, err := scraper.Scrape(ctx)
	if err != nil {
		logger.Error("error updating metrics", "err", err)
		return
	}

	newResultReg := prometheus.NewRegistry()
	newResultReg.MustRegister(promutil.NewPrometheusCollector(metrics))
	s.resultReg.Store(newResultReg)
	logger.Debug("Metrics scraped")
}
