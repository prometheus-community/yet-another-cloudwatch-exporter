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

// ConcurrencyConfig configures how concurrency should be limited in a Cloudwatch API client. It allows
// one to pick between different limiter implementations: a single limit limiter, or one with a different limit per
// API call.
type ConcurrencyConfig struct {
	// PerAPIEnabled configures whether to have a limit per API call.
	PerAPILimitEnabled bool

	// SingleLimit configures the concurrency limit when using a single limiter for api calls.
	SingleLimit int

	// ListMetrics limits the number for ListMetrics API concurrent API calls.
	ListMetrics int

	// GetMetricData limits the number for GetMetricData API concurrent API calls.
	GetMetricData int

	// GetMetricStatistics limits the number for GetMetricStatistics API concurrent API calls.
	GetMetricStatistics int
}

// semaphore implements a simple semaphore using a channel.
type semaphore chan struct{}

// newSemaphore creates a new semaphore with the given limit.
func newSemaphore(limit int) semaphore {
	return make(semaphore, limit)
}

func (s semaphore) Acquire() {
	s <- struct{}{}
}

func (s semaphore) Release() {
	<-s
}

// NewLimiter creates a new ConcurrencyLimiter, according to the ConcurrencyConfig.
func (cfg ConcurrencyConfig) NewLimiter() ConcurrencyLimiter {
	if cfg.PerAPILimitEnabled {
		return NewPerAPICallLimiter(cfg.ListMetrics, cfg.GetMetricData, cfg.GetMetricStatistics)
	}
	return NewSingleLimiter(cfg.SingleLimit)
}

// perAPICallLimiter is a ConcurrencyLimiter that keeps a different concurrency limiter per different API call. This allows
// a more granular control of concurrency, allowing us to take advantage of different api limits. For example, ListMetrics
// has a limit of 25 TPS, while GetMetricData has none.
type perAPICallLimiter struct {
	listMetricsLimiter          semaphore
	getMetricsDataLimiter       semaphore
	getMetricsStatisticsLimiter semaphore
}

// NewPerAPICallLimiter creates a new PerAPICallLimiter.
func NewPerAPICallLimiter(listMetrics, getMetricData, getMetricStatistics int) ConcurrencyLimiter {
	return &perAPICallLimiter{
		listMetricsLimiter:          newSemaphore(listMetrics),
		getMetricsDataLimiter:       newSemaphore(getMetricData),
		getMetricsStatisticsLimiter: newSemaphore(getMetricStatistics),
	}
}

func (l *perAPICallLimiter) Acquire(op string) {
	switch op {
	case listMetricsCall:
		l.listMetricsLimiter.Acquire()
	case getMetricDataCall:
		l.getMetricsDataLimiter.Acquire()
	case getMetricStatisticsCall:
		l.getMetricsStatisticsLimiter.Acquire()
	}
}

func (l *perAPICallLimiter) Release(op string) {
	switch op {
	case listMetricsCall:
		l.listMetricsLimiter.Release()
	case getMetricDataCall:
		l.getMetricsDataLimiter.Release()
	case getMetricStatisticsCall:
		l.getMetricsStatisticsLimiter.Release()
	}
}

// singleLimiter is the current implementation of ConcurrencyLimiter, which has a single limit for all different API calls.
type singleLimiter struct {
	s semaphore
}

// NewSingleLimiter creates a new SingleLimiter.
func NewSingleLimiter(limit int) ConcurrencyLimiter {
	return &singleLimiter{
		s: newSemaphore(limit),
	}
}

func (sl *singleLimiter) Acquire(_ string) {
	sl.s.Acquire()
}

func (sl *singleLimiter) Release(_ string) {
	sl.s.Release()
}
