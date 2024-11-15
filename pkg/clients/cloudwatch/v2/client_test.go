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
package v2

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/stretchr/testify/require"

	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
)

func Test_toMetricDataResult(t *testing.T) {
	ts := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name                      string
		getMetricDataOutput       cloudwatch.GetMetricDataOutput
		expectedMetricDataResults []cloudwatch_client.MetricDataResult
	}

	testCases := []testCase{
		{
			name: "all metrics present",
			getMetricDataOutput: cloudwatch.GetMetricDataOutput{
				MetricDataResults: []types.MetricDataResult{
					{
						Id:         aws.String("metric-1"),
						Values:     []float64{1.0, 2.0, 3.0},
						Timestamps: []time.Time{ts.Add(10 * time.Minute), ts.Add(5 * time.Minute), ts},
					},
					{
						Id:         aws.String("metric-2"),
						Values:     []float64{2.0},
						Timestamps: []time.Time{ts},
					},
				},
			},
			expectedMetricDataResults: []cloudwatch_client.MetricDataResult{
				{ID: "metric-1", Datapoint: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
				{ID: "metric-2", Datapoint: aws.Float64(2.0), Timestamp: ts},
			},
		},
		{
			name: "metric with no values",
			getMetricDataOutput: cloudwatch.GetMetricDataOutput{
				MetricDataResults: []types.MetricDataResult{
					{
						Id:         aws.String("metric-1"),
						Values:     []float64{1.0, 2.0, 3.0},
						Timestamps: []time.Time{ts.Add(10 * time.Minute), ts.Add(5 * time.Minute), ts},
					},
					{
						Id:         aws.String("metric-2"),
						Values:     []float64{},
						Timestamps: []time.Time{},
					},
				},
			},
			expectedMetricDataResults: []cloudwatch_client.MetricDataResult{
				{ID: "metric-1", Datapoint: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
				{ID: "metric-2", Datapoint: nil, Timestamp: time.Time{}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metricDataResults := toMetricDataResult(tc.getMetricDataOutput)
			require.Equal(t, tc.expectedMetricDataResults, metricDataResults)
		})
	}
}
