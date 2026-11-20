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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_cloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func Test_billableMetricRequests(t *testing.T) {
	dims := func(vals ...string) []model.Dimension {
		out := make([]model.Dimension, 0, len(vals)/2)
		for i := 0; i+1 < len(vals); i += 2 {
			out = append(out, model.Dimension{Name: vals[i], Value: vals[i+1]})
		}
		return out
	}
	datum := func(metric string, d []model.Dimension, period int64, stat string) *model.CloudwatchData {
		return &model.CloudwatchData{
			MetricName:                    metric,
			Dimensions:                    d,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: period, Statistic: stat},
		}
	}
	stats := func(metric string, d []model.Dimension, period int64, names ...string) []*model.CloudwatchData {
		out := make([]*model.CloudwatchData, 0, len(names))
		for _, n := range names {
			out = append(out, datum(metric, d, period, n))
		}
		return out
	}
	concat := func(parts ...[]*model.CloudwatchData) []*model.CloudwatchData {
		var out []*model.CloudwatchData
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	d1 := dims("InstanceId", "i-1")
	d2 := dims("InstanceId", "i-2")

	testCases := []struct {
		name     string
		data     []*model.CloudwatchData
		expected int
	}{
		{name: "empty", data: nil, expected: 0},
		{name: "one statistic", data: stats("CPU", d1, 300, "Average"), expected: 1},
		{name: "five statistics of one metric", data: stats("CPU", d1, 300, "Average", "Sum", "Minimum", "Maximum", "SampleCount"), expected: 1},
		{name: "six statistics of one metric", data: stats("CPU", d1, 300, "Average", "Sum", "Minimum", "Maximum", "SampleCount", "p99"), expected: 2},
		{name: "same statistic twice counts once", data: stats("CPU", d1, 300, "Average", "Average"), expected: 1},
		{name: "different dimensions are different metrics", data: concat(stats("CPU", d1, 300, "Average"), stats("CPU", d2, 300, "Average")), expected: 2},
		{name: "different metric names are different metrics", data: concat(stats("CPU", d1, 300, "Average"), stats("Net", d1, 300, "Average")), expected: 2},
		{name: "different periods are different metrics", data: concat(stats("CPU", d1, 300, "Average"), stats("CPU", d1, 60, "Average")), expected: 2},
		{
			name:     "dimension order does not matter",
			data:     concat(stats("CPU", dims("a", "1", "b", "2"), 300, "Average"), stats("CPU", dims("b", "2", "a", "1"), 300, "Sum")),
			expected: 1,
		},
		{
			name:     "sums over metrics",
			data:     concat(stats("CPU", d1, 300, "Average", "Sum"), stats("CPU", d2, 300, "Average", "Sum", "Minimum", "Maximum", "SampleCount", "p99")),
			expected: 3,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, billableMetricRequests(tc.data))
		})
	}
}

func Test_toMetricDataResult(t *testing.T) {
	ts := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name                      string
		exportAllDataPoints       bool
		getMetricDataOutput       aws_cloudwatch.GetMetricDataOutput
		expectedMetricDataResults []MetricDataResult
	}

	testCases := []testCase{
		{
			name:                "all metrics present",
			exportAllDataPoints: false,
			getMetricDataOutput: aws_cloudwatch.GetMetricDataOutput{
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
			expectedMetricDataResults: []MetricDataResult{
				{
					ID: "metric-1", DataPoints: []DataPoint{
						{Value: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
					},
				},
				{
					ID: "metric-2", DataPoints: []DataPoint{
						{Value: aws.Float64(2.0), Timestamp: ts},
					},
				},
			},
		},
		{
			name:                "metric with no values",
			exportAllDataPoints: false,
			getMetricDataOutput: aws_cloudwatch.GetMetricDataOutput{
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
			expectedMetricDataResults: []MetricDataResult{
				{
					ID: "metric-1", DataPoints: []DataPoint{
						{Value: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
					},
				},
				{
					ID:         "metric-2",
					DataPoints: []DataPoint{},
				},
			},
		},
		{
			name:                "export all data points",
			exportAllDataPoints: true,
			getMetricDataOutput: aws_cloudwatch.GetMetricDataOutput{
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
			expectedMetricDataResults: []MetricDataResult{
				{
					ID: "metric-1", DataPoints: []DataPoint{
						{Value: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
						{Value: aws.Float64(2.0), Timestamp: ts.Add(5 * time.Minute)},
						{Value: aws.Float64(3.0), Timestamp: ts},
					},
				},
				{
					ID: "metric-2", DataPoints: []DataPoint{
						{Value: aws.Float64(2.0), Timestamp: ts},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metricDataResults := toMetricDataResult(tc.getMetricDataOutput, tc.exportAllDataPoints)
			require.Equal(t, tc.expectedMetricDataResults, metricDataResults)
		})
	}
}
