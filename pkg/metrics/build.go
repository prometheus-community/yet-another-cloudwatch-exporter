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

// ScrapeCollectors contains metrics specific to the scraping process, such as API call counters.
var ScrapeCollectors = []prometheus.Collector{
	promutil.CloudwatchAPIErrorCounter,
	promutil.CloudwatchAPICounter,
	promutil.CloudwatchGetMetricDataAPICounter,
	promutil.CloudwatchGetMetricDataAPIMetricsCounter,
	promutil.CloudwatchGetMetricStatisticsAPICounter,
	promutil.ResourceGroupTaggingAPICounter,
	promutil.AutoScalingAPICounter,
	promutil.TargetGroupsAPICounter,
	promutil.APIGatewayAPICounter,
	promutil.Ec2APICounter,
	promutil.DmsAPICounter,
	promutil.StoragegatewayAPICounter,
	promutil.DuplicateMetricsFilteredCounter,
}

// Scrape performs one CloudWatch scrape and converts the result into Prometheus metrics.
func Scrape(
	ctx context.Context,
	logger *slog.Logger,
	cfg config.Config,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
) ([]*promutil.PrometheusMetric, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Use legacy validation as that's the behaviour of former releases.
	prom.NameValidationScheme = prom.LegacyValidation //nolint:staticcheck

	ctx = config.CtxWithFlags(ctx, featureFlagsMapFromSlice(cfg.FeatureFlags))

	tagsData, cloudwatchData := job.ScrapeAwsData(
		ctx,
		logger,
		jobsCfg,
		factory,
		cfg.MetricsPerQuery,
		toCloudWatchConcurrency(cfg.CloudwatchConcurrency),
		cfg.TaggingAPIConcurrency,
	)

	metrics, observedMetricLabels, err := promutil.BuildMetrics(cloudwatchData, cfg.LabelsSnakeCase, logger)
	if err != nil {
		return nil, err
	}
	metrics, observedMetricLabels = promutil.BuildNamespaceInfoMetrics(tagsData, metrics, observedMetricLabels, cfg.LabelsSnakeCase, logger)
	metrics = promutil.EnsureLabelConsistencyAndRemoveDuplicates(metrics, observedMetricLabels)

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
