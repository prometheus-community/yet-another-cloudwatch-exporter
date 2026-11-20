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

func Test_toModelMetric(t *testing.T) {
	page := &aws_cloudwatch.ListMetricsOutput{
		Metrics: []types.Metric{
			{MetricName: aws.String("CPUUtilization"), Namespace: aws.String("AWS/EC2"), Dimensions: []types.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-1")}}},
			{MetricName: aws.String("CPUUtilization"), Namespace: aws.String("AWS/EC2"), Dimensions: []types.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-2")}}},
			{MetricName: aws.String("CPUUtilization"), Namespace: aws.String("AWS/EC2"), Dimensions: []types.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-3")}}},
		},
		OwningAccounts: []string{"111111111111", "222222222222", "333333333333"},
	}

	instanceIDs := func(metrics []*model.Metric) []string {
		ids := make([]string, 0, len(metrics))
		for _, m := range metrics {
			ids = append(ids, m.Dimensions[0].Value)
		}
		return ids
	}

	t.Run("without linked accounts", func(t *testing.T) {
		metrics := toModelMetric(page, nil)
		require.Equal(t, []string{"i-1", "i-2", "i-3"}, instanceIDs(metrics))
		for _, m := range metrics {
			require.Empty(t, m.LinkedAccountID)
		}
	})

	t.Run("all linked accounts", func(t *testing.T) {
		metrics := toModelMetric(page, []string{"*"})
		require.Equal(t, []string{"i-1", "i-2", "i-3"}, instanceIDs(metrics))
		require.Equal(t, "111111111111", metrics[0].LinkedAccountID)
		require.Equal(t, "222222222222", metrics[1].LinkedAccountID)
		require.Equal(t, "333333333333", metrics[2].LinkedAccountID)
	})

	t.Run("selected linked accounts", func(t *testing.T) {
		metrics := toModelMetric(page, []string{"111111111111", "333333333333"})
		require.Equal(t, []string{"i-1", "i-3"}, instanceIDs(metrics))
		require.Equal(t, "111111111111", metrics[0].LinkedAccountID)
		require.Equal(t, "333333333333", metrics[1].LinkedAccountID)
	})

	t.Run("missing owning accounts", func(t *testing.T) {
		metrics := toModelMetric(&aws_cloudwatch.ListMetricsOutput{Metrics: page.Metrics}, []string{"*"})
		require.Equal(t, []string{"i-1", "i-2", "i-3"}, instanceIDs(metrics))
		for _, m := range metrics {
			require.Empty(t, m.LinkedAccountID)
		}
	})
}
