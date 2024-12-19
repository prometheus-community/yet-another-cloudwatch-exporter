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
package cloudwatch

import (
	"context"
	"time"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const (
	listMetricsCall         = "ListMetrics"
	getMetricDataCall       = "GetMetricData"
	getMetricStatisticsCall = "GetMetricStatistics"
)

type Client interface {
	// ListMetrics returns the list of metrics and dimensions for a given namespace
	// and metric name. Results pagination is handled automatically: the caller can
	// optionally pass a non-nil func in order to handle results pages.
	ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, recentlyActiveOnly bool, fn func(page []*model.Metric)) error

	// GetMetricData returns the output of the GetMetricData CloudWatch API.
	// Results pagination is handled automatically.
	GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []MetricDataResult

	// GetMetricStatistics returns the output of the GetMetricStatistics CloudWatch API.
	GetMetricStatistics(ctx context.Context, logger logging.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.Datapoint
}

// ConcurrencyLimiter limits the concurrency when calling AWS CloudWatch APIs. The functions implemented
// by this interface follow the same as a normal semaphore, but accept and operation identifier. Some
// implementations might use this to keep a different semaphore, with different reentrance values, per
// operation.
type ConcurrencyLimiter interface {
	// Acquire takes one "ticket" from the concurrency limiter for op. If there's none available, the caller
	// routine will be blocked until there's room available.
	Acquire(op string)

	// Release gives back one "ticket" to the concurrency limiter identified by op. If there's one or more
	// routines waiting for one, one will be woken up.
	Release(op string)
}

type MetricDataResult struct {
	ID string
	// A nil datapoint is a marker for no datapoint being found
	Datapoint *float64
	Timestamp time.Time
}

type limitedConcurrencyClient struct {
	client  Client
	limiter ConcurrencyLimiter
}

func NewLimitedConcurrencyClient(client Client, limiter ConcurrencyLimiter) Client {
	return &limitedConcurrencyClient{
		client:  client,
		limiter: limiter,
	}
}

func (c limitedConcurrencyClient) GetMetricStatistics(ctx context.Context, logger logging.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.Datapoint {
	c.limiter.Acquire(getMetricStatisticsCall)
	res := c.client.GetMetricStatistics(ctx, logger, dimensions, namespace, metric)
	c.limiter.Release(getMetricStatisticsCall)
	return res
}

func (c limitedConcurrencyClient) GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []MetricDataResult {
	c.limiter.Acquire(getMetricDataCall)
	res := c.client.GetMetricData(ctx, getMetricData, namespace, startTime, endTime)
	c.limiter.Release(getMetricDataCall)
	return res
}

func (c limitedConcurrencyClient) ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, recentlyActiveOnly bool, fn func(page []*model.Metric)) error {
	c.limiter.Acquire(listMetricsCall)
	err := c.client.ListMetrics(ctx, namespace, metric, recentlyActiveOnly, fn)
	c.limiter.Release(listMetricsCall)
	return err
}
