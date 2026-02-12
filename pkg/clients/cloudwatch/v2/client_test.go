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
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func Test_toMetricDataResult(t *testing.T) {
	ts := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name                      string
		exportAllDataPoints       bool
		getMetricDataOutput       cloudwatch.GetMetricDataOutput
		expectedMetricDataResults []cloudwatch_client.MetricDataResult
	}

	testCases := []testCase{
		{
			name:                "all metrics present",
			exportAllDataPoints: false,
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
				{
					ID: "metric-1", DataPoints: []cloudwatch_client.DataPoint{
						{Value: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
					},
				},
				{
					ID: "metric-2", DataPoints: []cloudwatch_client.DataPoint{
						{Value: aws.Float64(2.0), Timestamp: ts},
					},
				},
			},
		},
		{
			name:                "metric with no values",
			exportAllDataPoints: false,
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
				{
					ID: "metric-1", DataPoints: []cloudwatch_client.DataPoint{
						{Value: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
					},
				},
				{
					ID:         "metric-2",
					DataPoints: []cloudwatch_client.DataPoint{},
				},
			},
		},
		{
			name:                "export all data points",
			exportAllDataPoints: true,
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
				{
					ID: "metric-1", DataPoints: []cloudwatch_client.DataPoint{
						{Value: aws.Float64(1.0), Timestamp: ts.Add(10 * time.Minute)},
						{Value: aws.Float64(2.0), Timestamp: ts.Add(5 * time.Minute)},
						{Value: aws.Float64(3.0), Timestamp: ts},
					},
				},
				{
					ID: "metric-2", DataPoints: []cloudwatch_client.DataPoint{
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
	type testCase struct {
		name                  string
		listMetricsOutput     *cloudwatch.ListMetricsOutput
		includeLinkedAccounts []string
		expectedMetrics       []*model.Metric
	}

	testCases := []testCase{
		{
			name: "no linked accounts filter - original behavior",
			listMetricsOutput: &cloudwatch.ListMetricsOutput{
				Metrics: []types.Metric{
					{
						MetricName: aws.String("CPUUtilization"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-12345")},
						},
					},
					{
						MetricName: aws.String("NetworkIn"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-67890")},
						},
					},
				},
			},
			includeLinkedAccounts: nil,
			expectedMetrics: []*model.Metric{
				{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-12345"},
					},
				},
				{
					MetricName: "NetworkIn",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-67890"},
					},
				},
			},
		},
		{
			name: "with wildcard linked accounts - include all",
			listMetricsOutput: &cloudwatch.ListMetricsOutput{
				Metrics: []types.Metric{
					{
						MetricName: aws.String("CPUUtilization"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-12345")},
						},
					},
					{
						MetricName: aws.String("NetworkIn"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-67890")},
						},
					},
				},
				OwningAccounts: []string{
					"111111111111",
					"222222222222",
				},
			},
			includeLinkedAccounts: []string{"*"},
			expectedMetrics: []*model.Metric{
				{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-12345"},
					},
					LinkedAccountID: "111111111111",
				},
				{
					MetricName: "NetworkIn",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-67890"},
					},
					LinkedAccountID: "222222222222",
				},
			},
		},
		{
			name: "with specific linked accounts - filter by account ID",
			listMetricsOutput: &cloudwatch.ListMetricsOutput{
				Metrics: []types.Metric{
					{
						MetricName: aws.String("CPUUtilization"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-12345")},
						},
					},
					{
						MetricName: aws.String("NetworkIn"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-67890")},
						},
					},
					{
						MetricName: aws.String("DiskReadOps"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-11111")},
						},
					},
				},
				OwningAccounts: []string{
					"111111111111",
					"222222222222",
					"333333333333",
				},
			},
			includeLinkedAccounts: []string{"111111111111", "333333333333"},
			expectedMetrics: []*model.Metric{
				{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-12345"},
					},
					LinkedAccountID: "111111111111",
				},
				{
					MetricName: "DiskReadOps",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-11111"},
					},
					LinkedAccountID: "333333333333",
				},
			},
		},
		{
			name: "with linked accounts filter - no matches",
			listMetricsOutput: &cloudwatch.ListMetricsOutput{
				Metrics: []types.Metric{
					{
						MetricName: aws.String("CPUUtilization"),
						Namespace:  aws.String("AWS/EC2"),
						Dimensions: []types.Dimension{
							{Name: aws.String("InstanceId"), Value: aws.String("i-12345")},
						},
					},
				},
				OwningAccounts: []string{
					"111111111111",
				},
			},
			includeLinkedAccounts: []string{"999999999999"},
			expectedMetrics:       []*model.Metric{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := toModelMetric(tc.listMetricsOutput, tc.includeLinkedAccounts)
			require.Equal(t, tc.expectedMetrics, result)
		})
	}
}
