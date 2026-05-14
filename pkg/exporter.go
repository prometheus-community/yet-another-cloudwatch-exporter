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
package exporter

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/metrics"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

const (
	// Deprecated: use config.DefaultMetricsPerQuery.
	DefaultMetricsPerQuery = config.DefaultMetricsPerQuery
	// Deprecated: use config.DefaultLabelsSnakeCase.
	DefaultLabelsSnakeCase = config.DefaultLabelsSnakeCase
	// Deprecated: use config.DefaultTaggingAPIConcurrency.
	DefaultTaggingAPIConcurrency = config.DefaultTaggingAPIConcurrency
)

// Deprecated: use config.DefaultCloudwatchConcurrency.
var DefaultCloudwatchConcurrency = toClientCloudWatchConcurrency(config.DefaultCloudwatchConcurrency)

// featureFlagsMap is a map that contains the enabled feature flags. If a key is not present, it means the feature flag
// is disabled.
type featureFlagsMap map[string]struct{}

type options struct {
	metricsPerQuery       int
	labelsSnakeCase       bool
	taggingAPIConcurrency int
	featureFlags          featureFlagsMap
	cloudwatchConcurrency cloudwatch.ConcurrencyConfig
}

// IsFeatureEnabled implements the FeatureFlags interface, allowing us to inject the options-configure feature flags in the rest of the code.
func (ff featureFlagsMap) IsFeatureEnabled(flag string) bool {
	_, ok := ff[flag]
	return ok
}

type OptionsFunc func(*options) error

func MetricsPerQuery(metricsPerQuery int) OptionsFunc {
	return func(o *options) error {
		if metricsPerQuery <= 0 {
			return fmt.Errorf("MetricsPerQuery must be a positive value")
		}

		o.metricsPerQuery = metricsPerQuery
		return nil
	}
}

func LabelsSnakeCase(labelsSnakeCase bool) OptionsFunc {
	return func(o *options) error {
		o.labelsSnakeCase = labelsSnakeCase
		return nil
	}
}

func CloudWatchAPIConcurrency(maxConcurrency int) OptionsFunc {
	return func(o *options) error {
		if maxConcurrency <= 0 {
			return fmt.Errorf("CloudWatchAPIConcurrency must be a positive value")
		}

		o.cloudwatchConcurrency.SingleLimit = maxConcurrency
		return nil
	}
}

func CloudWatchPerAPILimitConcurrency(listMetrics, getMetricData, getMetricStatistics int) OptionsFunc {
	return func(o *options) error {
		if listMetrics <= 0 {
			return fmt.Errorf("LitMetrics concurrency limit must be a positive value")
		}
		if getMetricData <= 0 {
			return fmt.Errorf("GetMetricData concurrency limit must be a positive value")
		}
		if getMetricStatistics <= 0 {
			return fmt.Errorf("GetMetricStatistics concurrency limit must be a positive value")
		}

		o.cloudwatchConcurrency.PerAPILimitEnabled = true
		o.cloudwatchConcurrency.ListMetrics = listMetrics
		o.cloudwatchConcurrency.GetMetricData = getMetricData
		o.cloudwatchConcurrency.GetMetricStatistics = getMetricStatistics
		return nil
	}
}

func TaggingAPIConcurrency(maxConcurrency int) OptionsFunc {
	return func(o *options) error {
		if maxConcurrency <= 0 {
			return fmt.Errorf("TaggingAPIConcurrency must be a positive value")
		}

		o.taggingAPIConcurrency = maxConcurrency
		return nil
	}
}

// EnableFeatureFlag is an option that enables a feature flag on the YACE's entrypoint.
func EnableFeatureFlag(flags ...string) OptionsFunc {
	return func(o *options) error {
		for _, flag := range flags {
			o.featureFlags[flag] = struct{}{}
		}
		return nil
	}
}

func defaultOptions() options {
	return options{
		metricsPerQuery:       config.DefaultMetricsPerQuery,
		labelsSnakeCase:       config.DefaultLabelsSnakeCase,
		taggingAPIConcurrency: config.DefaultTaggingAPIConcurrency,
		featureFlags:          make(featureFlagsMap),
		cloudwatchConcurrency: toClientCloudWatchConcurrency(config.DefaultCloudwatchConcurrency),
	}
}

func ConfigOptions(cfg config.Config) ([]OptionsFunc, error) {
	opts := []OptionsFunc{
		MetricsPerQuery(cfg.MetricsPerQuery),
		LabelsSnakeCase(cfg.LabelsSnakeCase),
		TaggingAPIConcurrency(cfg.TaggingAPIConcurrency),
		EnableFeatureFlag(cfg.FeatureFlags...),
	}

	if cfg.CloudwatchConcurrency.PerAPILimitEnabled {
		opts = append(opts,
			CloudWatchPerAPILimitConcurrency(
				cfg.CloudwatchConcurrency.ListMetrics,
				cfg.CloudwatchConcurrency.GetMetricData,
				cfg.CloudwatchConcurrency.GetMetricStatistics,
			),
		)
	} else {
		opts = append(opts, CloudWatchAPIConcurrency(cfg.CloudwatchConcurrency.SingleLimit))
	}

	checked := defaultOptions()
	for _, option := range opts {
		if err := option(&checked); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

// BuildPrometheusMetrics scrapes AWS data and converts it into Prometheus metrics.
//
// Deprecated: use metrics.NewScraper and (*metrics.Scraper).Scrape.
func BuildPrometheusMetrics(
	ctx context.Context,
	logger *slog.Logger,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
	optFuncs ...OptionsFunc,
) ([]*promutil.PrometheusMetric, error) {
	options := defaultOptions()
	for _, f := range optFuncs {
		if err := f(&options); err != nil {
			return nil, err
		}
	}

	scraper, err := metrics.NewScraper(logger, configFromOptions(options), jobsCfg, factory)
	if err != nil {
		return nil, err
	}
	return scraper.Scrape(ctx)
}

// UpdateMetrics is the entrypoint to scrape metrics from AWS on demand.
//
// Parameters are:
// - `ctx`: a context for the request
// - `config`: this is the struct representation of the configuration defined in top-level configuration
// - `logger`: an *slog.Logger
// - `registry`: any prometheus compatible registry where scraped AWS metrics will be written
// - `factory`: any implementation of the `clients.Factory` interface
// - `optFuncs`: (optional) any number of options funcs
//
// To include scrape instrumentation metrics, use metrics.NewScraper and register its collectors:
//
//	scraper, err := metrics.NewScraper(logger, cfg, jobsCfg, factory)
//	if err != nil {
//		return err
//	}
//	if err := scraper.RegisterCollectors(registry); err != nil {
//		return err
//	}
//	generatedMetrics, err := scraper.Scrape(ctx)
//	if err != nil {
//		return err
//	}
//	registry.MustRegister(promutil.NewPrometheusCollector(generatedMetrics))
//
// Deprecated: use metrics.NewScraper, (*metrics.Scraper).RegisterCollectors, and (*metrics.Scraper).Scrape.
func UpdateMetrics(
	ctx context.Context,
	logger *slog.Logger,
	jobsCfg model.JobsConfig,
	registry *prometheus.Registry,
	factory clients.Factory,
	optFuncs ...OptionsFunc,
) error {
	metrics, err := BuildPrometheusMetrics(ctx, logger, jobsCfg, factory, optFuncs...)
	if err != nil {
		return err
	}

	registry.MustRegister(promutil.NewPrometheusCollector(metrics))
	return nil
}

// configFromOptions is just a convenience function to convert the options to a config.Config struct.
// It is needed just to convert the deprecated options to the new config.Config struct.
func configFromOptions(o options) config.Config {
	return config.Config{
		MetricsPerQuery:       o.metricsPerQuery,
		LabelsSnakeCase:       o.labelsSnakeCase,
		TaggingAPIConcurrency: o.taggingAPIConcurrency,
		FeatureFlags:          featureFlagsFromMap(o.featureFlags),
		CloudwatchConcurrency: config.CloudWatchConcurrencyConfig{
			SingleLimit:         o.cloudwatchConcurrency.SingleLimit,
			PerAPILimitEnabled:  o.cloudwatchConcurrency.PerAPILimitEnabled,
			ListMetrics:         o.cloudwatchConcurrency.ListMetrics,
			GetMetricData:       o.cloudwatchConcurrency.GetMetricData,
			GetMetricStatistics: o.cloudwatchConcurrency.GetMetricStatistics,
		},
	}
}

func featureFlagsFromMap(flags featureFlagsMap) []string {
	ret := make([]string, 0, len(flags))
	for flag := range flags {
		ret = append(ret, flag)
	}
	return ret
}

func toClientCloudWatchConcurrency(cfg config.CloudWatchConcurrencyConfig) cloudwatch.ConcurrencyConfig {
	return cloudwatch.ConcurrencyConfig{
		SingleLimit:         cfg.SingleLimit,
		PerAPILimitEnabled:  cfg.PerAPILimitEnabled,
		ListMetrics:         cfg.ListMetrics,
		GetMetricData:       cfg.GetMetricData,
		GetMetricStatistics: cfg.GetMetricStatistics,
	}
}
