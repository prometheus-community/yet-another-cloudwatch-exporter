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
package exporter

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// Metrics is a slice of prometheus metrics specific to the scraping process such API call counters
var Metrics = []prometheus.Collector{
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

const (
	DefaultMetricsPerQuery       = 500
	DefaultLabelsSnakeCase       = false
	DefaultTaggingAPIConcurrency = 5
)

var DefaultCloudwatchConcurrency = cloudwatch.ConcurrencyConfig{
	SingleLimit:        5,
	PerAPILimitEnabled: false,

	// If PerAPILimitEnabled is enabled, then use the same limit as the single limit by default.
	ListMetrics:         5,
	GetMetricData:       5,
	GetMetricStatistics: 5,
}

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
		metricsPerQuery:       DefaultMetricsPerQuery,
		labelsSnakeCase:       DefaultLabelsSnakeCase,
		taggingAPIConcurrency: DefaultTaggingAPIConcurrency,
		featureFlags:          make(featureFlagsMap),
		cloudwatchConcurrency: DefaultCloudwatchConcurrency,
	}
}

// UpdateMetrics is the entrypoint to scrape metrics from AWS on demand.
//
// Parameters are:
// - `ctx`: a context for the request
// - `config`: this is the struct representation of the configuration defined in top-level configuration
// - `logger`: any implementation of the `logging.Logger` interface
// - `registry`: any prometheus compatible registry where scraped AWS metrics will be written
// - `factory`: any implementation of the `clients.Factory` interface
// - `optFuncs`: (optional) any number of options funcs
//
// You can pre-register any of the default metrics from `Metrics` with the provided `registry` if you want them
// included in the AWS scrape results. If you are using multiple instances of `registry` it
// might make more sense to register these metrics in the application using YACE as a library to better
// track them over the lifetime of the application.
func UpdateMetrics(
	ctx context.Context,
	logger logging.Logger,
	jobsCfg model.JobsConfig,
	registry *prometheus.Registry,
	factory clients.Factory,
	optFuncs ...OptionsFunc,
) error {
	options := defaultOptions()
	for _, f := range optFuncs {
		if err := f(&options); err != nil {
			return err
		}
	}

	// add feature flags to context passed down to all other layers
	ctx = config.CtxWithFlags(ctx, options.featureFlags)

	tagsData, cloudwatchData := job.ScrapeAwsData(
		ctx,
		logger,
		jobsCfg,
		factory,
		options.metricsPerQuery,
		options.cloudwatchConcurrency,
		options.taggingAPIConcurrency,
	)

	metrics, observedMetricLabels, err := promutil.BuildMetrics(cloudwatchData, options.labelsSnakeCase, logger)
	if err != nil {
		logger.Error(err, "Error migrating cloudwatch metrics to prometheus metrics")
		return nil
	}
	metrics, observedMetricLabels = promutil.BuildNamespaceInfoMetrics(tagsData, metrics, observedMetricLabels, options.labelsSnakeCase, logger)
	metrics = promutil.EnsureLabelConsistencyAndRemoveDuplicates(metrics, observedMetricLabels)

	registry.MustRegister(promutil.NewPrometheusCollector(metrics))
	return nil
}
