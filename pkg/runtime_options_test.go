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

import (
	"testing"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
)

func TestRuntimeOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       config.RuntimeConfig
		wantError bool
	}{
		{
			name: "default runtime config",
			cfg:  config.DefaultRuntimeConfig(),
		},
		{
			name: "per api concurrency",
			cfg: func() config.RuntimeConfig {
				cfg := config.DefaultRuntimeConfig()
				cfg.CloudwatchConcurrency.PerAPILimitEnabled = true
				return cfg
			}(),
		},
		{
			name: "invalid metrics per query",
			cfg: func() config.RuntimeConfig {
				cfg := config.DefaultRuntimeConfig()
				cfg.MetricsPerQuery = 0
				return cfg
			}(),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := RuntimeOptions(tt.cfg)
			if tt.wantError && err == nil {
				t.Fatal("RuntimeOptions() error = nil, want non-nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("RuntimeOptions() error = %v, want nil", err)
			}
		})
	}
}
