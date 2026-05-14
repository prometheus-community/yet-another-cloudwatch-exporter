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

import (
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.MetricsPerQuery != DefaultMetricsPerQuery {
		t.Fatalf("MetricsPerQuery = %d, want %d", cfg.MetricsPerQuery, DefaultMetricsPerQuery)
	}
	if cfg.TaggingAPIConcurrency != DefaultTaggingAPIConcurrency {
		t.Fatalf("TaggingAPIConcurrency = %d, want %d", cfg.TaggingAPIConcurrency, DefaultTaggingAPIConcurrency)
	}
	if cfg.CloudwatchConcurrency != DefaultCloudwatchConcurrency {
		t.Fatalf("CloudwatchConcurrency = %+v, want %+v", cfg.CloudwatchConcurrency, DefaultCloudwatchConcurrency)
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*Config)
		wantError string
	}{
		{
			name: "default config",
		},
		{
			name: "invalid metrics per query",
			mutate: func(cfg *Config) {
				cfg.MetricsPerQuery = 0
			},
			wantError: "metrics per query",
		},
		{
			name: "invalid tagging concurrency",
			mutate: func(cfg *Config) {
				cfg.TaggingAPIConcurrency = 0
			},
			wantError: "tagging api concurrency",
		},
		{
			name: "invalid cloudwatch single concurrency",
			mutate: func(cfg *Config) {
				cfg.CloudwatchConcurrency.SingleLimit = 0
			},
			wantError: "cloudwatch api concurrency",
		},
		{
			name: "invalid list metrics concurrency",
			mutate: func(cfg *Config) {
				cfg.CloudwatchConcurrency.PerAPILimitEnabled = true
				cfg.CloudwatchConcurrency.ListMetrics = 0
			},
			wantError: "listmetrics concurrency",
		},
		{
			name: "invalid get metric data concurrency",
			mutate: func(cfg *Config) {
				cfg.CloudwatchConcurrency.PerAPILimitEnabled = true
				cfg.CloudwatchConcurrency.GetMetricData = 0
			},
			wantError: "getmetricdata concurrency",
		},
		{
			name: "invalid get metric statistics concurrency",
			mutate: func(cfg *Config) {
				cfg.CloudwatchConcurrency.PerAPILimitEnabled = true
				cfg.CloudwatchConcurrency.GetMetricStatistics = 0
			},
			wantError: "getmetricstatistics concurrency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := DefaultConfig()
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}

			err := cfg.Validate()
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want error containing %q", tt.wantError)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tt.wantError) {
				t.Fatalf("Validate() error = %q, want to contain %q", err.Error(), tt.wantError)
			}
		})
	}
}
