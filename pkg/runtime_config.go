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

package exporter

import "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"

// RuntimeConfig contains runtime-only scrape knobs that are not part of the YACE YAML config file.
type RuntimeConfig struct {
	MetricsPerQuery       int                          `mapstructure:"metrics_per_query"`
	LabelsSnakeCase       bool                         `mapstructure:"labels_snake_case"`
	TaggingAPIConcurrency int                          `mapstructure:"tagging_api_concurrency"`
	FeatureFlags          []string                     `mapstructure:"feature_flags"`
	CloudwatchConcurrency cloudwatch.ConcurrencyConfig `mapstructure:"cloudwatch_concurrency"`
}

func DefaultRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MetricsPerQuery:       DefaultMetricsPerQuery,
		LabelsSnakeCase:       DefaultLabelsSnakeCase,
		TaggingAPIConcurrency: DefaultTaggingAPIConcurrency,
		FeatureFlags:          []string{},
		CloudwatchConcurrency: DefaultCloudwatchConcurrency,
	}
}

func (c RuntimeConfig) Options() ([]OptionsFunc, error) {
	opts := []OptionsFunc{
		MetricsPerQuery(c.MetricsPerQuery),
		LabelsSnakeCase(c.LabelsSnakeCase),
		TaggingAPIConcurrency(c.TaggingAPIConcurrency),
		EnableFeatureFlag(c.FeatureFlags...),
	}

	if c.CloudwatchConcurrency.PerAPILimitEnabled {
		opts = append(opts,
			CloudWatchPerAPILimitConcurrency(
				c.CloudwatchConcurrency.ListMetrics,
				c.CloudwatchConcurrency.GetMetricData,
				c.CloudwatchConcurrency.GetMetricStatistics,
			),
		)
	} else {
		opts = append(opts, CloudWatchAPIConcurrency(c.CloudwatchConcurrency.SingleLimit))
	}

	checked := defaultOptions()
	for _, option := range opts {
		if err := option(&checked); err != nil {
			return nil, err
		}
	}

	return opts, nil
}
