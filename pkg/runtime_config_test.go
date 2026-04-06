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

import "testing"

func TestDefaultRuntimeConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultRuntimeConfig()
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

func TestRuntimeConfigOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       RuntimeConfig
		wantError bool
	}{
		{
			name: "default runtime config",
			cfg:  DefaultRuntimeConfig(),
		},
		{
			name: "per api concurrency",
			cfg: func() RuntimeConfig {
				cfg := DefaultRuntimeConfig()
				cfg.CloudwatchConcurrency.PerAPILimitEnabled = true
				return cfg
			}(),
		},
		{
			name: "invalid metrics per query",
			cfg: func() RuntimeConfig {
				cfg := DefaultRuntimeConfig()
				cfg.MetricsPerQuery = 0
				return cfg
			}(),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.cfg.Options()
			if tt.wantError && err == nil {
				t.Fatal("Options() error = nil, want non-nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("Options() error = %v, want nil", err)
			}
		})
	}
}
