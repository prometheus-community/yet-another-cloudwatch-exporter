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
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	if cfg.MetricsPerQuery <= 0 {
		t.Fatalf("MetricsPerQuery = %d, want > 0", cfg.MetricsPerQuery)
	}
	if cfg.TaggingAPIConcurrency <= 0 {
		t.Fatalf("TaggingAPIConcurrency = %d, want > 0", cfg.TaggingAPIConcurrency)
	}
	if cfg.AWSScrapeInterval != defaultAWSScrapeInterval {
		t.Fatalf("AWSScrapeInterval = %q, want %q", cfg.AWSScrapeInterval, defaultAWSScrapeInterval)
	}

	defaults := defaultComponentDefaults()

	expectedDefaults := map[string]interface{}{
		"fips_enabled":            cfg.FIPSEnabled,
		"metrics_per_query":       cfg.MetricsPerQuery,
		"labels_snake_case":       cfg.LabelsSnakeCase,
		"tagging_api_concurrency": cfg.TaggingAPIConcurrency,
		"feature_flags":           cfg.FeatureFlags,
		"aws_scrape_interval":     cfg.AWSScrapeInterval,
	}
	for key, want := range expectedDefaults {
		if got := defaults[key]; !reflect.DeepEqual(got, want) {
			t.Fatalf("defaults[%s] = %#v, want %#v", key, got, want)
		}
	}
	if _, ok := defaults["config_file"]; ok {
		t.Fatal("defaults[config_file] present, want omitted")
	}

	cloudwatchDefaults, ok := defaults["cloudwatch_concurrency"].(map[string]interface{})
	if !ok {
		t.Fatalf("defaults[cloudwatch_concurrency] has type %T, want map[string]interface{}", defaults["cloudwatch_concurrency"])
	}

	expectedCloudwatchDefaults := map[string]interface{}{
		"single_limit":          cfg.CloudwatchConcurrency.SingleLimit,
		"per_api_limit_enabled": cfg.CloudwatchConcurrency.PerAPILimitEnabled,
		"list_metrics":          cfg.CloudwatchConcurrency.ListMetrics,
		"get_metric_data":       cfg.CloudwatchConcurrency.GetMetricData,
		"get_metric_statistics": cfg.CloudwatchConcurrency.GetMetricStatistics,
	}
	for key, want := range expectedCloudwatchDefaults {
		if got := cloudwatchDefaults[key]; !reflect.DeepEqual(got, want) {
			t.Fatalf("cloudwatch defaults %s = %#v, want %#v", key, got, want)
		}
	}
}

func TestConfigValidateAndJobsConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	cfg.ConfigFile = validConfigFile()
	cfg.FeatureFlags = []string{"always-return-info-metrics"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	jobsCfg, err := cfg.jobsConfig(nil)
	if err != nil {
		t.Fatalf("jobsConfig() error = %v", err)
	}
	if jobsCfg.StsRegion != "eu-west-1" {
		t.Fatalf("jobsCfg.StsRegion = %q, want %q", jobsCfg.StsRegion, "eu-west-1")
	}
	if len(jobsCfg.DiscoveryJobs) != 1 {
		t.Fatalf("len(jobsCfg.DiscoveryJobs) = %d, want 1", len(jobsCfg.DiscoveryJobs))
	}
	if len(jobsCfg.DiscoveryJobs[0].Roles) != 1 {
		t.Fatalf("len(jobsCfg.DiscoveryJobs[0].Roles) = %d, want 1", len(jobsCfg.DiscoveryJobs[0].Roles))
	}
	if jobsCfg.DiscoveryJobs[0].Namespace != "AWS/S3" {
		t.Fatalf("jobsCfg.DiscoveryJobs[0].Namespace = %q, want %q", jobsCfg.DiscoveryJobs[0].Namespace, "AWS/S3")
	}
}

func TestConfigValidateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "missing config file",
			cfg: func() *Config {
				cfg := validConfig()
				cfg.ConfigFile = ""
				return cfg
			}(),
		},
		{
			name: "invalid metrics_per_query",
			cfg: func() *Config {
				cfg := validConfig()
				cfg.MetricsPerQuery = 0
				return cfg
			}(),
		},
		{
			name: "invalid aws_scrape_interval",
			cfg: func() *Config {
				cfg := validConfig()
				cfg.AWSScrapeInterval = "nope"
				return cfg
			}(),
		},
		{
			name: "invalid config_file path",
			cfg: func() *Config {
				cfg := validConfig()
				cfg.ConfigFile = filepath.Join("testdata", "missing.yml")
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
		})
	}
}

func validConfig() *Config {
	cfg := defaultConfig()
	cfg.ConfigFile = validConfigFile()
	return cfg
}

func validConfigFile() string {
	return filepath.Join("..", "pkg", "config", "testdata", "sts_region.ok.yml")
}
