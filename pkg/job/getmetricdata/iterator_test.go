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
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestIteratorFactory_Build(t *testing.T) {
	tests := []struct {
		name             string
		input            []*model.CloudwatchData
		expectedIterator Iterator
	}{
		{
			name:             "empty returns nothing to iterator",
			input:            []*model.CloudwatchData{},
			expectedIterator: nothingToIterate{},
		},
		{
			name: "input with consistent period and delay returns simple batching",
			input: []*model.CloudwatchData{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 100}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 100}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 100}},
			},
			expectedIterator: &simpleBatchingIterator{},
		},
		{
			name: "input with inconsistent period returns time param batching",
			input: []*model.CloudwatchData{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 100}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 11, Delay: 100}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 12, Delay: 100}},
			},
			expectedIterator: &timeParameterBatchingIterator{},
		},
		{
			name: "input with inconsistent delay returns time param batching",
			input: []*model.CloudwatchData{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 100}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 101}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 102}},
			},
			expectedIterator: &timeParameterBatchingIterator{},
		},
		{
			name: "input with inconsistent period and delay returns time param batching",
			input: []*model.CloudwatchData{
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 10, Delay: 100}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 11, Delay: 101}},
				{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 12, Delay: 102}},
			},
			expectedIterator: &timeParameterBatchingIterator{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			factory := iteratorFactory{100}
			iterator := factory.Build(tc.input)
			assert.IsType(t, tc.expectedIterator, iterator)
		})
	}
}

func TestSimpleBatchingIterator_SetsLengthAndDelay(t *testing.T) {
	data := []*model.CloudwatchData{
		{GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{Period: 101, Delay: 100}},
	}
	params := StartAndEndTimeParams{
		Period: 102,
		Length: 101,
		Delay:  100,
	}
	iterator := NewSimpleBatchIterator(1, data, params)
	_, out := iterator.Next()
	assert.Equal(t, params, out)
}

func TestSimpleBatchingIterator_IterateFlow(t *testing.T) {
	tests := []struct {
		name                        string
		metricsPerQuery             int
		lengthOfCloudwatchData      int
		expectedNumberOfCallsToNext int
	}{
		{
			name:                        "1 per batch",
			metricsPerQuery:             1,
			lengthOfCloudwatchData:      10,
			expectedNumberOfCallsToNext: 10,
		},
		{
			name:                        "divisible batches and requests",
			metricsPerQuery:             5,
			lengthOfCloudwatchData:      100,
			expectedNumberOfCallsToNext: 20,
		},
		{
			name:                        "indivisible batches and requests",
			metricsPerQuery:             5,
			lengthOfCloudwatchData:      94,
			expectedNumberOfCallsToNext: 19,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]*model.CloudwatchData, 0, tc.lengthOfCloudwatchData)
			for i := 0; i < tc.lengthOfCloudwatchData; i++ {
				data = append(data, getSampleMetricDatas(strconv.Itoa(i)))
			}
			params := StartAndEndTimeParams{
				Period: data[0].GetMetricDataProcessingParams.Period,
				Length: data[0].GetMetricDataProcessingParams.Length,
				Delay:  data[0].GetMetricDataProcessingParams.Delay,
			}
			iterator := NewSimpleBatchIterator(tc.metricsPerQuery, data, params)

			outputData := make([]*model.CloudwatchData, 0, len(data))
			numberOfCallsToNext := 0
			for iterator.HasMore() {
				numberOfCallsToNext++
				batch, _ := iterator.Next()
				outputData = append(outputData, batch...)
			}

			assert.ElementsMatch(t, data, outputData)
			assert.Equal(t, tc.expectedNumberOfCallsToNext, numberOfCallsToNext)
		})
	}
}

func TestVaryingTimeParameterBatchingIterator_IterateFlow(t *testing.T) {
	tests := []struct {
		name                                          string
		metricsPerQuery                               int
		lengthOfCloudwatchDataByStartAndEndTimeParams map[StartAndEndTimeParams]int
		expectedBatchesByStartAndEndTimeParams        map[StartAndEndTimeParams]int
	}{
		{
			name:            "1 per batch - two time parameters",
			metricsPerQuery: 1,
			lengthOfCloudwatchDataByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 10,
				{Period: 20, Length: 20, Delay: 20}: 10,
			},
			expectedBatchesByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 10,
				{Period: 20, Length: 20, Delay: 20}: 10,
			},
		},
		{
			name:            "1 per batch - uses max length for available period + delay",
			metricsPerQuery: 1,
			lengthOfCloudwatchDataByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 10,
				{Period: 10, Length: 30, Delay: 10}: 10,
				{Period: 20, Length: 20, Delay: 20}: 10,
				{Period: 20, Length: 40, Delay: 20}: 10,
			},
			expectedBatchesByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 30, Delay: 10}: 20,
				{Period: 20, Length: 40, Delay: 20}: 20,
			},
		},
		{
			name:            "divisible batches - two time parameters",
			metricsPerQuery: 5,
			lengthOfCloudwatchDataByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 100,
				{Period: 20, Length: 20, Delay: 20}: 100,
			},
			expectedBatchesByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 20,
				{Period: 20, Length: 20, Delay: 20}: 20,
			},
		},
		{
			name:            "divisible batches - uses max length for available period + delay",
			metricsPerQuery: 5,
			lengthOfCloudwatchDataByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 100,
				{Period: 10, Length: 30, Delay: 10}: 100,
				{Period: 20, Length: 20, Delay: 20}: 100,
				{Period: 20, Length: 40, Delay: 20}: 100,
			},
			expectedBatchesByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 30, Delay: 10}: 40,
				{Period: 20, Length: 40, Delay: 20}: 40,
			},
		},
		{
			name:            "indivisible batches - two time parameters",
			metricsPerQuery: 5,
			lengthOfCloudwatchDataByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 94,
				{Period: 20, Length: 20, Delay: 20}: 94,
			},
			expectedBatchesByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 19,
				{Period: 20, Length: 20, Delay: 20}: 19,
			},
		},
		{
			name:            "indivisible batches - uses max length for available period + delay",
			metricsPerQuery: 5,
			lengthOfCloudwatchDataByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 10, Delay: 10}: 94,
				{Period: 10, Length: 30, Delay: 10}: 94,
				{Period: 20, Length: 20, Delay: 20}: 94,
				{Period: 20, Length: 40, Delay: 20}: 94,
			},
			expectedBatchesByStartAndEndTimeParams: map[StartAndEndTimeParams]int{
				{Period: 10, Length: 30, Delay: 10}: 38,
				{Period: 20, Length: 40, Delay: 20}: 38,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := []*model.CloudwatchData{}
			for params, lengthOfCloudwatchData := range tc.lengthOfCloudwatchDataByStartAndEndTimeParams {
				for i := 0; i < lengthOfCloudwatchData; i++ {
					entry := getSampleMetricDatas(strconv.Itoa(rand.Int()))
					entry.GetMetricDataProcessingParams.Length = params.Length
					entry.GetMetricDataProcessingParams.Delay = params.Delay
					entry.GetMetricDataProcessingParams.Period = params.Period
					data = append(data, entry)
				}
			}
			iterator := iteratorFactory{metricsPerQuery: tc.metricsPerQuery}.Build(data)

			outputData := make([]*model.CloudwatchData, 0, len(data))
			numberOfBatchesByStartAndEndTimeParams := map[StartAndEndTimeParams]int{}
			for iterator.HasMore() {
				batch, params := iterator.Next()
				numberOfBatchesByStartAndEndTimeParams[params]++
				outputData = append(outputData, batch...)
			}

			assert.ElementsMatch(t, data, outputData)
			assert.Len(t, numberOfBatchesByStartAndEndTimeParams, len(tc.expectedBatchesByStartAndEndTimeParams))
			for params, count := range tc.expectedBatchesByStartAndEndTimeParams {
				actualCount, ok := numberOfBatchesByStartAndEndTimeParams[params]
				assert.True(t, ok, "output batches was missing expected batches of start and endtime params %+v", params)
				assert.Equal(t, count, actualCount, "%+v had an incorrect batch count", params)
			}
		})
	}
}
