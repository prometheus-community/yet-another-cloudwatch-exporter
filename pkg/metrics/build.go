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
package metrics

import (
	"context"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	prom "github.com/prometheus/common/model"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// Scraper owns the configuration and scrape instrumentation for one embedded YACE instance.
type Scraper struct {
	logger        *slog.Logger
	cfg           config.Config
	jobsCfg       model.JobsConfig
	factory       clients.Factory
	scrapeMetrics *promutil.ScrapeMetrics
}

// NewScraper creates a scraper with its own scrape instrumentation collectors.
// Call 'cfg.Validate()' before calling this function.
func NewScraper(
	logger *slog.Logger,
	cfg config.Config,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
) (*Scraper, error) {
	return newScraperWithMetrics(logger, cfg, jobsCfg, factory, promutil.NewScrapeMetrics())
}

// NewLegacyScraperWithMetrics creates a scraper using caller-provided scrape instrumentation collectors.
//
// Deprecated: use NewScraper for isolated scrape instrumentation.
func NewLegacyScraperWithMetrics(
	logger *slog.Logger,
	cfg config.Config,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
	scrapeMetrics *promutil.ScrapeMetrics,
) (*Scraper, error) {
	return newScraperWithMetrics(logger, cfg, jobsCfg, factory, scrapeMetrics)
}

func newScraperWithMetrics(
	logger *slog.Logger,
	cfg config.Config,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
	scrapeMetrics *promutil.ScrapeMetrics,
) (*Scraper, error) {
	cfg.FeatureFlags = append([]string(nil), cfg.FeatureFlags...)
	if instrumentedFactory, ok := factory.(clients.InstrumentedFactory); ok {
		factory = instrumentedFactory.WithScrapeMetrics(scrapeMetrics)
	}
	return &Scraper{
		logger:        logger,
		cfg:           cfg,
		jobsCfg:       jobsCfg,
		factory:       factory,
		scrapeMetrics: scrapeMetrics,
	}, nil
}

// RegisterCollectors registers the scraper's instrumentation collectors with registry.
func (s *Scraper) RegisterCollectors(registry *prometheus.Registry) error {
	for _, collector := range s.scrapeMetrics.Collectors() {
		if err := registry.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

// Scrape performs one CloudWatch scrape and converts the result into Prometheus metrics.
func (s *Scraper) Scrape(ctx context.Context) ([]*promutil.PrometheusMetric, error) {
	// Use legacy validation as that's the behaviour of former releases.
	prom.NameValidationScheme = prom.LegacyValidation //nolint:staticcheck

	ctx = config.CtxWithFlags(ctx, featureFlagsMapFromSlice(s.cfg.FeatureFlags))

	tagsData, cloudwatchData := job.ScrapeAwsData(
		ctx,
		s.logger,
		s.jobsCfg,
		s.factory,
		s.cfg.MetricsPerQuery,
		toCloudWatchConcurrency(s.cfg.CloudwatchConcurrency),
		s.cfg.TaggingAPIConcurrency,
	)

	metrics, observedMetricLabels, err := promutil.BuildMetrics(cloudwatchData, s.cfg.LabelsSnakeCase, s.logger)
	if err != nil {
		return nil, err
	}
	metrics, observedMetricLabels = promutil.BuildNamespaceInfoMetrics(tagsData, metrics, observedMetricLabels, s.cfg.LabelsSnakeCase, s.logger)
	metrics = promutil.EnsureLabelConsistencyAndRemoveDuplicates(metrics, observedMetricLabels, s.scrapeMetrics)

	return metrics, nil
}

type featureFlagsMap map[string]struct{}

func (ff featureFlagsMap) IsFeatureEnabled(flag string) bool {
	_, ok := ff[flag]
	return ok
}

func featureFlagsMapFromSlice(flags []string) featureFlagsMap {
	ret := make(featureFlagsMap, len(flags))
	for _, flag := range flags {
		ret[flag] = struct{}{}
	}
	return ret
}

func toCloudWatchConcurrency(cfg config.CloudWatchConcurrencyConfig) cloudwatch.ConcurrencyConfig {
	return cloudwatch.ConcurrencyConfig{
		SingleLimit:         cfg.SingleLimit,
		PerAPILimitEnabled:  cfg.PerAPILimitEnabled,
		ListMetrics:         cfg.ListMetrics,
		GetMetricData:       cfg.GetMetricData,
		GetMetricStatistics: cfg.GetMetricStatistics,
	}
}
