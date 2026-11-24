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
package cloudwatch

import (
	"log/slog"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func Test_createGetMetricStatisticsInput_statistics(t *testing.T) {
	tests := []struct {
		name                       string
		statistics                 []string
		expectedStatistics         []types.Statistic
		expectedExtendedStatistics []string
	}{
		{
			name:               "only standard statistics",
			statistics:         []string{"Average", "Sum"},
			expectedStatistics: []types.Statistic{types.StatisticAverage, types.StatisticSum},
		},
		{
			name:                       "only percentile statistics",
			statistics:                 []string{"p95"},
			expectedExtendedStatistics: []string{"p95"},
		},
		{
			name:                       "standard and percentile statistics",
			statistics:                 []string{"Maximum", "p99"},
			expectedStatistics:         []types.Statistic{types.StatisticMaximum},
			expectedExtendedStatistics: []string{"p99"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.DiscardHandler)
			namespace := "AWS/ApplicationELB"
			metric := &model.MetricConfig{
				Name:       "TargetResponseTime",
				Statistics: tc.statistics,
				Period:     300,
				Length:     1800,
			}

			require.NotPanics(t, func() {
				input := createGetMetricStatisticsInput(logger, nil, &namespace, metric)
				require.Equal(t, tc.expectedStatistics, input.Statistics)
				require.Equal(t, tc.expectedExtendedStatistics, input.ExtendedStatistics)
			})
		})
	}
}
