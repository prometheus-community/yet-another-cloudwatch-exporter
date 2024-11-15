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
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []cloudwatch.MetricDataResult
}

type IteratorFactory interface {
	// Build returns an ideal batch iterator based on the provided CloudwatchData
	Build(requests []*model.CloudwatchData) Iterator
}

type Iterator interface {
	// Next returns the next batch of CloudWatch data be used when calling GetMetricData and the start + end time for
	// the GetMetricData call
	// If called when there are no more batches default values will be returned
	Next() ([]*model.CloudwatchData, StartAndEndTimeParams)

	// HasMore returns true if there are more batches to iterate otherwise false. Should be used in a loop
	// to govern calls to Next()
	HasMore() bool
}

type StartAndEndTimeParams struct {
	Period int64
	Length int64
	Delay  int64
}

type Processor struct {
	client           Client
	concurrency      int
	windowCalculator MetricWindowCalculator
	logger           logging.Logger
	factory          IteratorFactory
}

func NewDefaultProcessor(logger logging.Logger, client Client, metricsPerQuery int, concurrency int) Processor {
	return NewProcessor(logger, client, concurrency, MetricWindowCalculator{clock: TimeClock{}}, &iteratorFactory{metricsPerQuery: metricsPerQuery})
}

func NewProcessor(logger logging.Logger, client Client, concurrency int, windowCalculator MetricWindowCalculator, factory IteratorFactory) Processor {
	return Processor{
		logger:           logger,
		client:           client,
		concurrency:      concurrency,
		windowCalculator: windowCalculator,
		factory:          factory,
	}
}

func (p Processor) Run(ctx context.Context, namespace string, requests []*model.CloudwatchData) ([]*model.CloudwatchData, error) {
	if len(requests) == 0 {
		return requests, nil
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(p.concurrency)

	iterator := p.factory.Build(requests)
	for iterator.HasMore() {
		batch, batchParams := iterator.Next()
		g.Go(func() error {
			batch = addQueryIDsToBatch(batch)
			startTime, endTime := p.windowCalculator.Calculate(toSecondDuration(batchParams.Period), toSecondDuration(batchParams.Length), toSecondDuration(batchParams.Delay))
			if p.logger.IsDebugEnabled() {
				p.logger.Debug("GetMetricData Window", "start_time", startTime.Format(TimeFormat), "end_time", endTime.Format(TimeFormat))
			}

			data := p.client.GetMetricData(gCtx, batch, namespace, startTime, endTime)
			if data != nil {
				mapResultsToBatch(p.logger, data, batch)
			} else {
				p.logger.Warn("GetMetricData partition empty result", "start", startTime, "end", endTime)
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("GetMetricData work group error: %w", err)
	}

	// Remove unprocessed/unknown elements in place, if any. Since getMetricDatas
	// is a slice of pointers, the compaction can be easily done in-place.
	requests = compact(requests, func(m *model.CloudwatchData) bool {
		return m.GetMetricDataResult != nil
	})

	return requests, nil
}

func addQueryIDsToBatch(batch []*model.CloudwatchData) []*model.CloudwatchData {
	for i, entry := range batch {
		entry.GetMetricDataProcessingParams.QueryID = indexToQueryID(i)
	}

	return batch
}

func mapResultsToBatch(logger logging.Logger, results []cloudwatch.MetricDataResult, batch []*model.CloudwatchData) {
	for _, entry := range results {
		id, err := queryIDToIndex(entry.ID)
		if err != nil {
			logger.Warn("GetMetricData returned unknown Query ID", "err", err, "query_id", id)
			continue
		}
		if batch[id].GetMetricDataResult == nil {
			cloudwatchData := batch[id]
			cloudwatchData.GetMetricDataResult = &model.GetMetricDataResult{
				Statistic: cloudwatchData.GetMetricDataProcessingParams.Statistic,
				Datapoint: entry.Datapoint,
				Timestamp: entry.Timestamp,
			}

			// All GetMetricData processing is done clear the params
			cloudwatchData.GetMetricDataProcessingParams = nil
		}
	}
}

func indexToQueryID(i int) string {
	return fmt.Sprintf("id_%d", i)
}

func queryIDToIndex(queryID string) (int, error) {
	noID := strings.TrimPrefix(queryID, "id_")
	id, err := strconv.Atoi(noID)
	return id, err
}

func toSecondDuration(i int64) time.Duration {
	return time.Duration(i) * time.Second
}
