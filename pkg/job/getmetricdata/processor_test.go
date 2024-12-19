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
package getmetricdata

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type cloudwatchDataInput struct {
	MetricName                    string
	GetMetricDataProcessingParams *model.GetMetricDataProcessingParams
}

type cloudwatchDataOutput struct {
	MetricName string
	*model.GetMetricDataResult
}

type metricDataResultForMetric struct {
	MetricName string
	result     cloudwatch.MetricDataResult
}

type testClient struct {
	GetMetricDataFunc             func(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []cloudwatch.MetricDataResult
	GetMetricDataResultForMetrics []metricDataResultForMetric
}

func (t testClient) GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []cloudwatch.MetricDataResult {
	if t.GetMetricDataResultForMetrics != nil {
		var result []cloudwatch.MetricDataResult
		for _, datum := range getMetricData {
			for _, response := range t.GetMetricDataResultForMetrics {
				if datum.MetricName == response.MetricName {
					response.result.ID = datum.GetMetricDataProcessingParams.QueryID
					result = append(result, response.result)
				}
			}
		}
		return result
	}
	return t.GetMetricDataFunc(ctx, getMetricData, namespace, startTime, endTime)
}

func TestProcessor_Run(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                       string
		requests                   []*cloudwatchDataInput
		metricDataResultForMetrics []metricDataResultForMetric
		want                       []cloudwatchDataOutput
		metricsPerBatch            int
	}{
		{
			name: "successfully maps input to output when GetMetricData returns data",
			requests: []*cloudwatchDataInput{
				{MetricName: "metric-1", GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Average"}},
			},
			metricDataResultForMetrics: []metricDataResultForMetric{
				{MetricName: "metric-1", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(1000), Timestamp: now}},
			},
			want: []cloudwatchDataOutput{
				{MetricName: "metric-1", GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Average", Datapoint: aws.Float64(1000), Timestamp: now}},
			},
		},
		{
			name: "handles duplicate results",
			requests: []*cloudwatchDataInput{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Min"}, MetricName: "MetricOne"},
			},
			metricDataResultForMetrics: []metricDataResultForMetric{
				{MetricName: "MetricOne", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(5), Timestamp: time.Date(2023, time.June, 7, 1, 9, 8, 0, time.UTC)}},
				{MetricName: "MetricOne", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(15), Timestamp: time.Date(2023, time.June, 7, 2, 9, 8, 0, time.UTC)}},
			},
			want: []cloudwatchDataOutput{
				{MetricName: "MetricOne", GetMetricDataResult: &model.GetMetricDataResult{
					Statistic: "Min",
					Datapoint: aws.Float64(5),
					Timestamp: time.Date(2023, time.June, 7, 1, 9, 8, 0, time.UTC),
				}},
			},
		},
		{
			name: "does not return a request when QueryID is not in MetricDataResult",
			requests: []*cloudwatchDataInput{
				{MetricName: "metric-1", GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Average"}},
				{MetricName: "metric-2", GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Average"}},
			},
			metricDataResultForMetrics: []metricDataResultForMetric{
				{MetricName: "metric-1", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(1000), Timestamp: now}},
			},
			want: []cloudwatchDataOutput{
				{MetricName: "metric-1", GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Average", Datapoint: aws.Float64(1000), Timestamp: now}},
			},
		},
		{
			name: "maps nil metric datapoints",
			requests: []*cloudwatchDataInput{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Min"}, MetricName: "MetricOne"},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Max"}, MetricName: "MetricTwo"},
			},
			metricDataResultForMetrics: []metricDataResultForMetric{
				{MetricName: "MetricOne", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(5), Timestamp: time.Date(2023, time.June, 7, 1, 9, 8, 0, time.UTC)}},
				{MetricName: "MetricTwo"},
			},
			want: []cloudwatchDataOutput{
				{
					MetricName: "MetricOne",
					GetMetricDataResult: &model.GetMetricDataResult{
						Statistic: "Min",
						Datapoint: aws.Float64(5),
						Timestamp: time.Date(2023, time.June, 7, 1, 9, 8, 0, time.UTC),
					},
				},
				{
					MetricName: "MetricTwo",
					GetMetricDataResult: &model.GetMetricDataResult{
						Statistic: "Max",
						Datapoint: nil,
						Timestamp: time.Time{},
					},
				},
			},
		},
		{
			name:            "successfully maps input to output when multiple batches are involved",
			metricsPerBatch: 1,
			requests: []*cloudwatchDataInput{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Min"}, MetricName: "MetricOne"},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Max"}, MetricName: "MetricTwo"},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Sum"}, MetricName: "MetricThree"},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Statistic: "Count"}, MetricName: "MetricFour"},
			},
			metricDataResultForMetrics: []metricDataResultForMetric{
				{MetricName: "MetricOne", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(5), Timestamp: time.Date(2023, time.June, 7, 1, 9, 8, 0, time.UTC)}},
				{MetricName: "MetricTwo", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(12), Timestamp: time.Date(2023, time.June, 7, 2, 9, 8, 0, time.UTC)}},
				{MetricName: "MetricThree", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(15), Timestamp: time.Date(2023, time.June, 7, 3, 9, 8, 0, time.UTC)}},
				{MetricName: "MetricFour", result: cloudwatch.MetricDataResult{Datapoint: aws.Float64(20), Timestamp: time.Date(2023, time.June, 7, 4, 9, 8, 0, time.UTC)}},
			},
			want: []cloudwatchDataOutput{
				{
					MetricName: "MetricOne",
					GetMetricDataResult: &model.GetMetricDataResult{
						Statistic: "Min",
						Datapoint: aws.Float64(5),
						Timestamp: time.Date(2023, time.June, 7, 1, 9, 8, 0, time.UTC),
					},
				},
				{
					MetricName: "MetricTwo",
					GetMetricDataResult: &model.GetMetricDataResult{
						Statistic: "Max",
						Datapoint: aws.Float64(12),
						Timestamp: time.Date(2023, time.June, 7, 2, 9, 8, 0, time.UTC),
					},
				},
				{
					MetricName: "MetricThree",
					GetMetricDataResult: &model.GetMetricDataResult{
						Statistic: "Sum",
						Datapoint: aws.Float64(15),
						Timestamp: time.Date(2023, time.June, 7, 3, 9, 8, 0, time.UTC),
					},
				},
				{
					MetricName: "MetricFour",
					GetMetricDataResult: &model.GetMetricDataResult{
						Statistic: "Count",
						Datapoint: aws.Float64(20),
						Timestamp: time.Date(2023, time.June, 7, 4, 9, 8, 0, time.UTC),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsPerQuery := 500
			if tt.metricsPerBatch != 0 {
				metricsPerQuery = tt.metricsPerBatch
			}
			r := NewDefaultProcessor(logging.NewNopLogger(), testClient{GetMetricDataResultForMetrics: tt.metricDataResultForMetrics}, metricsPerQuery, 1)
			cloudwatchData, err := r.Run(context.Background(), "anything_is_fine", ToCloudwatchData(tt.requests))
			require.NoError(t, err)
			require.Len(t, cloudwatchData, len(tt.want))
			got := make([]cloudwatchDataOutput, 0, len(cloudwatchData))
			for _, data := range cloudwatchData {
				assert.Nil(t, data.GetMetricStatisticsResult)
				assert.Nil(t, data.GetMetricDataProcessingParams)
				assert.NotNil(t, data.GetMetricDataResult)
				got = append(got, cloudwatchDataOutput{
					MetricName:          data.MetricName,
					GetMetricDataResult: data.GetMetricDataResult,
				})
			}

			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func ToCloudwatchData(input []*cloudwatchDataInput) []*model.CloudwatchData {
	output := make([]*model.CloudwatchData, 0, len(input))
	for _, i := range input {
		cloudwatchData := &model.CloudwatchData{
			MetricName:                    i.MetricName,
			ResourceName:                  "test",
			Namespace:                     "test",
			Tags:                          []model.Tag{{Key: "tag", Value: "value"}},
			Dimensions:                    []model.Dimension{{Name: "dimension", Value: "value"}},
			GetMetricDataProcessingParams: i.GetMetricDataProcessingParams,
			GetMetricDataResult:           nil,
			GetMetricStatisticsResult:     nil,
		}
		output = append(output, cloudwatchData)
	}
	return output
}

func getSampleMetricDatas(id string) *model.CloudwatchData {
	return &model.CloudwatchData{
		MetricName: "StorageBytes",
		Dimensions: []model.Dimension{
			{
				Name:  "FileSystemId",
				Value: "fs-abc123",
			},
			{
				Name:  "StorageClass",
				Value: "Standard",
			},
		},
		ResourceName: id,
		Namespace:    "efs",
		Tags: []model.Tag{
			{
				Key:   "Value1",
				Value: "",
			},
			{
				Key:   "Value2",
				Value: "",
			},
		},
		MetricMigrationParams: model.MetricMigrationParams{
			NilToZero:              false,
			AddCloudwatchTimestamp: false,
		},
		GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
			Period:    60,
			Length:    60,
			Delay:     0,
			Statistic: "Average",
		},
	}
}

func BenchmarkProcessorRun(b *testing.B) {
	type testcase struct {
		concurrency        int
		metricsPerQuery    int
		testResourcesCount int
	}

	for name, tc := range map[string]testcase{
		"small case": {
			concurrency:        10,
			metricsPerQuery:    500,
			testResourcesCount: 10,
		},
		"medium case": {
			concurrency:        10,
			metricsPerQuery:    500,
			testResourcesCount: 1000,
		},
		"big case": {
			concurrency:        10,
			metricsPerQuery:    500,
			testResourcesCount: 2000,
		},
	} {
		b.Run(name, func(b *testing.B) {
			doBench(b, tc.metricsPerQuery, tc.testResourcesCount, tc.concurrency)
		})
	}
}

func doBench(b *testing.B, metricsPerQuery, testResourcesCount int, concurrency int) {
	testResourceIDs := make([]string, testResourcesCount)
	for i := 0; i < testResourcesCount; i++ {
		testResourceIDs[i] = fmt.Sprintf("test-resource-%d", i)
	}

	client := testClient{GetMetricDataFunc: func(_ context.Context, getMetricData []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
		b.StopTimer()
		results := make([]cloudwatch.MetricDataResult, 0, len(getMetricData))
		for _, entry := range getMetricData {
			results = append(results, cloudwatch.MetricDataResult{
				ID:        entry.GetMetricDataProcessingParams.QueryID,
				Datapoint: aws.Float64(1),
				Timestamp: time.Now(),
			})
		}
		b.StartTimer()
		return results
	}}

	for i := 0; i < b.N; i++ {
		// stop timer to not affect benchmark run
		// this has to do in every run, since running the processor mutates the metric datas slice
		b.StopTimer()
		datas := make([]*model.CloudwatchData, 0, testResourcesCount)
		for i := 0; i < testResourcesCount; i++ {
			datas = append(datas, getSampleMetricDatas(testResourceIDs[i]))
		}
		r := NewDefaultProcessor(logging.NewNopLogger(), client, metricsPerQuery, concurrency)
		// re-start timer
		b.ReportAllocs()
		b.StartTimer()

		//nolint:errcheck
		r.Run(context.Background(), "anything_is_fine", datas)
	}
}
