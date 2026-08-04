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

func Test_billedGetMetricDataMetricsCount(t *testing.T) {
	dataWithStat := func(metricName string, dimensions []model.Dimension, statistic string) *model.CloudwatchData {
		return &model.CloudwatchData{
			MetricName: metricName,
			Dimensions: dimensions,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Statistic: statistic,
			},
		}
	}

	volumeDimensions := []model.Dimension{{Name: "VolumeId", Value: "vol-1"}}

	testCases := []struct {
		name          string
		getMetricData []*model.CloudwatchData
		expected      float64
	}{
		{
			name:          "no data",
			getMetricData: []*model.CloudwatchData{},
			expected:      0,
		},
		{
			name: "single metric single statistic bills as one metric",
			getMetricData: []*model.CloudwatchData{
				dataWithStat("VolumeReadBytes", volumeDimensions, "Average"),
			},
			expected: 1,
		},
		{
			name: "single metric with up to 5 statistics bills as one metric",
			getMetricData: []*model.CloudwatchData{
				dataWithStat("VolumeReadBytes", volumeDimensions, "Minimum"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "Maximum"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "Average"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "Sum"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "SampleCount"),
			},
			expected: 1,
		},
		{
			name: "single metric with 6 statistics bills as two metrics",
			getMetricData: []*model.CloudwatchData{
				dataWithStat("VolumeReadBytes", volumeDimensions, "Minimum"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "Maximum"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "Average"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "Sum"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "SampleCount"),
				dataWithStat("VolumeReadBytes", volumeDimensions, "p99"),
			},
			expected: 2,
		},
		{
			name: "distinct metrics with the same dimensions bill separately",
			getMetricData: []*model.CloudwatchData{
				dataWithStat("VolumeReadBytes", volumeDimensions, "Average"),
				dataWithStat("VolumeWriteBytes", volumeDimensions, "Average"),
			},
			expected: 2,
		},
		{
			name: "same metric name on different resources bills separately",
			getMetricData: []*model.CloudwatchData{
				dataWithStat("VolumeReadBytes", []model.Dimension{{Name: "VolumeId", Value: "vol-1"}}, "Average"),
				dataWithStat("VolumeReadBytes", []model.Dimension{{Name: "VolumeId", Value: "vol-2"}}, "Average"),
			},
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, billedGetMetricDataMetricsCount("AWS/EBS", tc.getMetricData))
		})
	}
}
