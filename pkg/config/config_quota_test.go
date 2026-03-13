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
package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
)

func TestQuotaMetricsConfig(t *testing.T) {
	testCases := []struct {
		name       string
		configFile string
		wantErr    bool
		errorMsg   string
	}{
		{
			name:       "valid config with quotaMetrics for supported namespace",
			configFile: "quota_metrics_ec2.ok.yml",
			wantErr:    false,
		},
		{
			name:       "invalid config with quotaMetrics for unsupported namespace",
			configFile: "quota_metrics_unsupported.bad.yml",
			wantErr:    true,
			errorMsg:   "quota metrics are not supported for namespace",
		},
		{
			name:       "config with only quotaMetrics is valid",
			configFile: "quota_metrics_only.ok.yml",
			wantErr:    false,
		},
		{
			name:       "empty config with no metrics, no enhanced, no quota is invalid",
			configFile: "quota_metrics_empty.bad.yml",
			wantErr:    true,
			errorMsg:   "Metrics, EnhancedMetrics, and QuotaMetrics should not all be empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ScrapeConf{}
			configFile := fmt.Sprintf("testdata/%s", tc.configFile)
			_, err := config.Load(configFile, promslog.NewNopLogger())
			if tc.wantErr {
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), tc.errorMsg),
					"expected error to contain %q but got: %s", tc.errorMsg, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
