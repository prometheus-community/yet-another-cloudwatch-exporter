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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestCreateGetMetricStatisticsInput_OnlyExtendedStatistics(t *testing.T) {
	// A metric configured with only percentile (extended) statistics, e.g. p50/p90/p99,
	// leaves the classic `statistics` slice empty. createGetMetricStatisticsInput must
	// not panic while building its debug CLI helper string in that case.
	logger := promslog.NewNopLogger()
	metric := &model.MetricConfig{
		Name:       "IngestionToInvocationStartLatency",
		Statistics: []string{"p50", "p90", "p99"},
		Period:     60,
		Length:     120,
		Delay:      300,
	}

	require.NotPanics(t, func() {
		output := createGetMetricStatisticsInput(logger, nil, aws.String("AWS/Events"), metric)
		require.Empty(t, output.Statistics)
		require.ElementsMatch(t, []string{"p50", "p90", "p99"}, output.ExtendedStatistics)
	})
}
