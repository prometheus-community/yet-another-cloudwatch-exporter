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
	"fmt"
	"io"
	"log/slog"
	"time"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	yaceconfig "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
)

const defaultAWSScrapeInterval = "60s"

// Config maps OTel receiver configuration into YACE's runtime configuration.
type Config struct {
	ConfigFile             string `mapstructure:"config_file"`
	FIPSEnabled            bool   `mapstructure:"fips_enabled"`
	AWSScrapeInterval      string `mapstructure:"aws_scrape_interval"`
	exporter.RuntimeConfig `mapstructure:",squash"`
}

var _ prombridge.Config = (*Config)(nil)

func defaultConfig() *Config {
	return &Config{
		AWSScrapeInterval: defaultAWSScrapeInterval,
		RuntimeConfig:     exporter.DefaultRuntimeConfig(),
	}
}

func defaultComponentDefaults() map[string]interface{} {
	cfg := defaultConfig()
	return map[string]interface{}{
		"fips_enabled":            cfg.FIPSEnabled,
		"metrics_per_query":       cfg.MetricsPerQuery,
		"labels_snake_case":       cfg.LabelsSnakeCase,
		"tagging_api_concurrency": cfg.TaggingAPIConcurrency,
		"feature_flags":           cfg.FeatureFlags,
		"aws_scrape_interval":     cfg.AWSScrapeInterval,
		"cloudwatch_concurrency": map[string]interface{}{
			"single_limit":          cfg.CloudwatchConcurrency.SingleLimit,
			"per_api_limit_enabled": cfg.CloudwatchConcurrency.PerAPILimitEnabled,
			"list_metrics":          cfg.CloudwatchConcurrency.ListMetrics,
			"get_metric_data":       cfg.CloudwatchConcurrency.GetMetricData,
			"get_metric_statistics": cfg.CloudwatchConcurrency.GetMetricStatistics,
		},
	}
}

func (c *Config) Validate() error {
	if c.ConfigFile == "" {
		return fmt.Errorf("config_file must not be empty")
	}
	if _, err := c.awsScrapeInterval(); err != nil {
		return err
	}
	if _, err := c.Options(); err != nil {
		return err
	}
	_, err := c.jobsConfig(discardLogger())
	return err
}

func (c *Config) jobsConfig(logger *slog.Logger) (model.JobsConfig, error) {
	if c.ConfigFile == "" {
		return model.JobsConfig{}, fmt.Errorf("config_file must not be empty")
	}
	if logger == nil {
		logger = discardLogger()
	}

	scrapeConf := yaceconfig.ScrapeConf{}
	return scrapeConf.Load(c.ConfigFile, logger)
}

func (c *Config) exporterOptions() ([]exporter.OptionsFunc, error) {
	return c.Options()
}

func (c *Config) awsScrapeInterval() (time.Duration, error) {
	raw := c.AWSScrapeInterval
	if raw == "" {
		raw = defaultAWSScrapeInterval
	}

	interval, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("aws_scrape_interval: invalid duration %q: %w", raw, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("aws_scrape_interval must be greater than 0")
	}
	return interval, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type configUnmarshaler struct{}

func (configUnmarshaler) GetConfigStruct() prombridge.Config {
	return defaultConfig()
}
