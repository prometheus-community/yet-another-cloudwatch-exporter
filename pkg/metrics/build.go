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
	scrapeMetrics *promutil.ScrapeMetrics,
	cfg config.Config,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
) (*Scraper, error) {
	if scrapeMetrics == nil {
		scrapeMetrics = promutil.Discard
	}
	cfg.FeatureFlags = append([]string(nil), cfg.FeatureFlags...)
	return &Scraper{
		logger:        logger,
		scrapeMetrics: scrapeMetrics,
		cfg:           cfg,
		jobsCfg:       jobsCfg,
		factory:       factory,
	}, nil
}

// Scrape performs one CloudWatch scrape and converts the result into Prometheus metrics.
func (s *Scraper) Scrape(ctx context.Context) ([]*promutil.PrometheusMetric, error) {
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
	metrics = promutil.EnsureLabelConsistencyAndRemoveDuplicates(s.scrapeMetrics, metrics, observedMetricLabels)

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
