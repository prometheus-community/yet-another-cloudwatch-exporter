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
	"math"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type iteratorFactory struct {
	metricsPerQuery int
}

func (b iteratorFactory) Build(data []*model.CloudwatchData) Iterator {
	if len(data) == 0 {
		return nothingToIterate{}
	}

	batchSizesByPeriodAndDelay, longestLengthForBatch := mapProcessingParams(data)

	if len(batchSizesByPeriodAndDelay) == 1 {
		// Only 1 period use value from data
		period := data[0].GetMetricDataProcessingParams.Period
		if len(batchSizesByPeriodAndDelay[period]) == 1 {
			// Only 1 period with 1 delay use value from data and do simple batching
			delay := data[0].GetMetricDataProcessingParams.Delay
			params := StartAndEndTimeParams{
				Period: period,
				Length: longestLengthForBatch[period][delay],
				Delay:  delay,
			}

			return NewSimpleBatchIterator(b.metricsPerQuery, data, params)
		}
	}

	return NewVaryingTimeParameterBatchingIterator(b.metricsPerQuery, data, batchSizesByPeriodAndDelay, longestLengthForBatch)
}

type (
	periodDelayToBatchSize     = map[int64]map[int64]int
	periodDelayToLongestLength = map[int64]map[int64]int64
)

// mapProcessingParams loops through all the incoming CloudwatchData to pre-compute important information
// to be used when initializing the batching iterator
// Knowing the period + delay combinations with their batch sizes will allow us to pre-allocate the batch slices that could
// be very large ahead of time without looping again later
// Similarly we need to know the largest length for a period + delay combination later so gathering it while we are already
// iterating will save some cycles later
func mapProcessingParams(data []*model.CloudwatchData) (periodDelayToBatchSize, periodDelayToLongestLength) {
	batchSizesByPeriodAndDelay := periodDelayToBatchSize{}
	longestLengthForBatch := periodDelayToLongestLength{}

	for _, datum := range data {
		period := datum.GetMetricDataProcessingParams.Period
		delay := datum.GetMetricDataProcessingParams.Delay
		if _, exists := batchSizesByPeriodAndDelay[period]; !exists {
			batchSizesByPeriodAndDelay[period] = map[int64]int{delay: 0}
			longestLengthForBatch[period] = map[int64]int64{delay: 0}
		}
		if _, exists := batchSizesByPeriodAndDelay[period][delay]; !exists {
			batchSizesByPeriodAndDelay[period][delay] = 0
			longestLengthForBatch[period][delay] = 0
		}
		batchSizesByPeriodAndDelay[period][delay]++
		if longestLengthForBatch[period][delay] < datum.GetMetricDataProcessingParams.Length {
			longestLengthForBatch[period][delay] = datum.GetMetricDataProcessingParams.Length
		}
	}

	return batchSizesByPeriodAndDelay, longestLengthForBatch
}

type nothingToIterate struct{}

func (n nothingToIterate) Next() ([]*model.CloudwatchData, StartAndEndTimeParams) {
	return nil, StartAndEndTimeParams{}
}

func (n nothingToIterate) HasMore() bool {
	return false
}

type simpleBatchingIterator struct {
	size            int
	currentBatch    int
	data            []*model.CloudwatchData
	entriesPerBatch int
	batchParams     StartAndEndTimeParams
}

func (s *simpleBatchingIterator) Next() ([]*model.CloudwatchData, StartAndEndTimeParams) {
	// We are out of data return defaults
	if s.currentBatch >= s.size {
		return nil, StartAndEndTimeParams{}
	}

	startingIndex := s.currentBatch * s.entriesPerBatch
	endingIndex := startingIndex + s.entriesPerBatch
	if endingIndex > len(s.data) {
		endingIndex = len(s.data)
	}

	// TODO are we technically doing this https://go.dev/wiki/SliceTricks#batching-with-minimal-allocation and if not
	// would it change allocations to do this ahead of time?
	result := s.data[startingIndex:endingIndex]
	s.currentBatch++

	return result, s.batchParams
}

func (s *simpleBatchingIterator) HasMore() bool {
	return s.currentBatch < s.size
}

// NewSimpleBatchIterator returns an iterator which slices the data in place based on the metricsPerQuery.
func NewSimpleBatchIterator(metricsPerQuery int, data []*model.CloudwatchData, batchParams StartAndEndTimeParams) Iterator {
	return &simpleBatchingIterator{
		size:            int(math.Ceil(float64(len(data)) / float64(metricsPerQuery))),
		batchParams:     batchParams,
		data:            data,
		entriesPerBatch: metricsPerQuery,
	}
}

type timeParameterBatchingIterator struct {
	current   Iterator
	remaining []Iterator
}

func (t *timeParameterBatchingIterator) Next() ([]*model.CloudwatchData, StartAndEndTimeParams) {
	batch, params := t.current.Next()

	// Doing this before returning from Next drastically simplifies HasMore because it can depend on
	// t.current.HasMore() being accurate.
	if !t.current.HasMore() {
		// Current iterator is out and there's none left, set current to nothingToIterate
		if len(t.remaining) == 0 {
			t.remaining = nil
			t.current = nothingToIterate{}
		} else {
			// Pop from https://go.dev/wiki/SliceTricks
			next, remaining := t.remaining[len(t.remaining)-1], t.remaining[:len(t.remaining)-1]
			t.current = next
			t.remaining = remaining
		}
	}

	return batch, params
}

func (t *timeParameterBatchingIterator) HasMore() bool {
	return t.current.HasMore()
}

func NewVaryingTimeParameterBatchingIterator(
	metricsPerQuery int,
	data []*model.CloudwatchData,
	batchSizes periodDelayToBatchSize,
	longestLengthForBatch periodDelayToLongestLength,
) Iterator {
	batches := make(map[int64]map[int64][]*model.CloudwatchData, len(batchSizes))
	numberOfIterators := 0
	// Pre-allocate batch slices
	for period, delays := range batchSizes {
		batches[period] = make(map[int64][]*model.CloudwatchData, len(delays))
		for delay, batchSize := range delays {
			numberOfIterators++
			batches[period][delay] = make([]*model.CloudwatchData, 0, batchSize)
		}
	}

	// Fill the batches
	for _, datum := range data {
		params := datum.GetMetricDataProcessingParams
		batch := batches[params.Period][params.Delay]
		batches[params.Period][params.Delay] = append(batch, datum)
	}

	var firstIterator Iterator
	iterators := make([]Iterator, 0, numberOfIterators-1)
	// We are ranging a map, and we won't have an index to mark the first iterator
	isFirst := true
	for period, delays := range batches {
		for delay, batch := range delays {
			batchParams := StartAndEndTimeParams{
				Period: period,
				Delay:  delay,
			}
			// Make sure to set the length to the longest length for the batch
			batchParams.Length = longestLengthForBatch[period][delay]
			iterator := NewSimpleBatchIterator(metricsPerQuery, batch, batchParams)
			if isFirst {
				firstIterator = iterator
				isFirst = false
			} else {
				iterators = append(iterators, iterator)
			}
		}
	}

	return &timeParameterBatchingIterator{
		current:   firstIterator,
		remaining: iterators,
	}
}
