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
package v1

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/require"

	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestDimensionsToCliString(t *testing.T) {
	// Setup Test

	// Arrange
	dimensions := []model.Dimension{}
	expected := ""

	// Act
	actual := dimensionsToCliString(dimensions)

	// Assert
	if actual != expected {
		t.Fatalf("\nexpected: %q\nactual:  %q", expected, actual)
	}
}

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
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{
						Id:         aws.String("metric-1"),
						Values:     []*float64{aws.Float64(1.0), aws.Float64(2.0), aws.Float64(3.0)},
						Timestamps: []*time.Time{aws.Time(ts.Add(10 * time.Minute)), aws.Time(ts.Add(5 * time.Minute)), aws.Time(ts)},
					},
					{
						Id:         aws.String("metric-2"),
						Values:     []*float64{aws.Float64(2.0)},
						Timestamps: []*time.Time{aws.Time(ts)},
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
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{
						Id:         aws.String("metric-1"),
						Values:     []*float64{aws.Float64(1.0), aws.Float64(2.0), aws.Float64(3.0)},
						Timestamps: []*time.Time{aws.Time(ts.Add(10 * time.Minute)), aws.Time(ts.Add(5 * time.Minute)), aws.Time(ts)},
					},
					{
						Id:         aws.String("metric-2"),
						Values:     []*float64{},
						Timestamps: []*time.Time{},
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
