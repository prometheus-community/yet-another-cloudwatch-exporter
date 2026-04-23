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

package otelcollector

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mitchellh/mapstructure"
	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	yaceconfig "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
)

const defaultAWSScrapeInterval = "60s"

// Config maps OTel receiver configuration into YACE's runtime configuration.
type Config struct {
	ConfigFile             string `mapstructure:"config_file,omitempty"`
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
	defaults := make(map[string]interface{})

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  &defaults,
		TagName: "mapstructure",
	})
	if err != nil {
		panic(fmt.Sprintf("create component defaults decoder: %v", err))
	}
	if err := decoder.Decode(defaultConfig()); err != nil {
		panic(fmt.Sprintf("decode component defaults: %v", err))
	}

	return defaults
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
