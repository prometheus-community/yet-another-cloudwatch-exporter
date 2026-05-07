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

package config

import "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"

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

// Config contains scrape-time settings used by embedders and CLI glue.
//
// ScrapeConf in scrapeconf.go models the YAML file that defines AWS jobs and
// resources. Config is intentionally separate: these fields control how a scrape
// is executed, and are commonly supplied by callers such as the YACE CLI or
// downstream tools that embed YACE.
type Config struct {
	MetricsPerQuery       int
	LabelsSnakeCase       bool
	TaggingAPIConcurrency int
	FeatureFlags          []string
	CloudwatchConcurrency cloudwatch.ConcurrencyConfig
}

func DefaultConfig() Config {
	return Config{
		MetricsPerQuery:       DefaultMetricsPerQuery,
		LabelsSnakeCase:       DefaultLabelsSnakeCase,
		TaggingAPIConcurrency: DefaultTaggingAPIConcurrency,
		FeatureFlags:          []string{},
		CloudwatchConcurrency: DefaultCloudwatchConcurrency,
	}
}
