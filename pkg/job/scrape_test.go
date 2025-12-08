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
package job

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func Test_hasEnhancedMetricsConfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  model.JobsConfig
		want bool
	}{
		{
			name: "no jobs",
			cfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{},
			},
			want: false,
		},
		{
			name: "jobs without enhanced metrics",
			cfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Namespace:       "AWS/RDS",
						EnhancedMetrics: []model.EnhancedMetricConfig{},
					},
					{
						Namespace:       "AWS/EC2",
						EnhancedMetrics: []model.EnhancedMetricConfig{},
					},
				},
			},
			want: false,
		},
		{
			name: "one job with enhanced metrics",
			cfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Namespace:       "AWS/RDS",
						EnhancedMetrics: []model.EnhancedMetricConfig{},
					},
					{
						Namespace: "AWS/EC2",
						EnhancedMetrics: []model.EnhancedMetricConfig{
							{Name: "StorageSpace", Enabled: true},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "all jobs with enhanced metrics",
			cfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Namespace: "AWS/RDS",
						EnhancedMetrics: []model.EnhancedMetricConfig{
							{Name: "StorageSpace", Enabled: true},
						},
					},
					{
						Namespace: "AWS/Lambda",
						EnhancedMetrics: []model.EnhancedMetricConfig{
							{Name: "MemorySize", Enabled: true},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "jobs with disabled enhanced metrics",
			cfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Namespace: "AWS/RDS",
						EnhancedMetrics: []model.EnhancedMetricConfig{
							{Name: "StorageSpace", Enabled: false},
						},
					},
				},
			},
			want: true, // Still returns true because the slice is not empty, filtering happens elsewhere
		},
		{
			name: "mixed job types",
			cfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Namespace:       "AWS/RDS",
						EnhancedMetrics: []model.EnhancedMetricConfig{},
					},
				},
				StaticJobs: []model.StaticJob{
					{Name: "test-static"},
				},
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{Name: "test-custom"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasEnhancedMetricsConfigured(tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}
