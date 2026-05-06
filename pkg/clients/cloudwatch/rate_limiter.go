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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

type APIRateLimit struct {
	Count    int
	Duration time.Duration
}

type RateLimiterConfig struct {
	ListMetrics         *APIRateLimit
	GetMetricData       *APIRateLimit
	GetMetricStatistics *APIRateLimit
}

type GlobalRateLimiter struct {
	listMetrics         *rateLimiterBuckets
	getMetricData       *rateLimiterBuckets
	getMetricStatistics *rateLimiterBuckets
}

type rateLimiterBucketKey struct {
	accountID string
	region    string
}

type rateLimiterBuckets struct {
	cfg      APIRateLimit
	limiters map[rateLimiterBucketKey]*rate.Limiter
	mu       sync.Mutex
}

func NewGlobalRateLimiter(config RateLimiterConfig) (*GlobalRateLimiter, error) {
	limiter := &GlobalRateLimiter{}

	if config.ListMetrics != nil {
		l, err := newRateLimiterBuckets(config.ListMetrics)
		if err != nil {
			return nil, fmt.Errorf("invalid ListMetrics rate limit: %w", err)
		}
		limiter.listMetrics = l
	}

	if config.GetMetricData != nil {
		l, err := newRateLimiterBuckets(config.GetMetricData)
		if err != nil {
			return nil, fmt.Errorf("invalid GetMetricData rate limit: %w", err)
		}
		limiter.getMetricData = l
	}

	if config.GetMetricStatistics != nil {
		l, err := newRateLimiterBuckets(config.GetMetricStatistics)
		if err != nil {
			return nil, fmt.Errorf("invalid GetMetricStatistics rate limit: %w", err)
		}
		limiter.getMetricStatistics = l
	}

	return limiter, nil
}

func newRateLimiterBuckets(cfg *APIRateLimit) (*rateLimiterBuckets, error) {
	if _, err := createLimiter(cfg); err != nil {
		return nil, err
	}

	return &rateLimiterBuckets{
		cfg:      *cfg,
		limiters: make(map[rateLimiterBucketKey]*rate.Limiter),
	}, nil
}

func (b *rateLimiterBuckets) get(accountID string, region string) *rate.Limiter {
	key := rateLimiterBucketKey{
		accountID: accountID,
		region:    region,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	limiter, ok := b.limiters[key]
	if ok {
		return limiter
	}

	limiter, _ = createLimiter(&b.cfg)
	b.limiters[key] = limiter
	return limiter
}

func createLimiter(cfg *APIRateLimit) (*rate.Limiter, error) {
	if cfg.Count <= 0 {
		return nil, fmt.Errorf("count must be positive, got %d", cfg.Count)
	}
	if cfg.Duration <= 0 {
		return nil, fmt.Errorf("duration must be positive, got %v", cfg.Duration)
	}

	ratePerSecond := float64(cfg.Count) / cfg.Duration.Seconds()
	return rate.NewLimiter(rate.Limit(ratePerSecond), cfg.Count), nil
}

func NewRateLimitedClient(client Client, globalLimiter *GlobalRateLimiter, region string, accountID string, role string) Client {
	if globalLimiter == nil {
		return client
	}
	return &SimpleRateLimitedClient{
		Client:    client,
		limiter:   globalLimiter,
		region:    region,
		accountID: accountID,
		role:      role,
	}
}

type SimpleRateLimitedClient struct {
	Client    Client
	limiter   *GlobalRateLimiter
	region    string
	accountID string
	role      string
}

func (c *SimpleRateLimitedClient) ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, recentlyActiveOnly bool, fn func(page []*model.Metric)) error {
	if err := c.limit(ctx, c.limiter.listMetrics, listMetricsCall, c.region, c.accountID, c.role, namespace); err != nil {
		return err
	}
	return c.Client.ListMetrics(ctx, namespace, metric, recentlyActiveOnly, fn)
}

func (c *SimpleRateLimitedClient) GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []MetricDataResult {
	if err := c.limit(ctx, c.limiter.getMetricData, getMetricDataCall, c.region, c.accountID, c.role, namespace); err != nil {
		return nil
	}
	return c.Client.GetMetricData(ctx, getMetricData, namespace, startTime, endTime)
}

func (c *SimpleRateLimitedClient) GetMetricStatistics(ctx context.Context, logger *slog.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.MetricStatisticsResult {
	if err := c.limit(ctx, c.limiter.getMetricStatistics, getMetricStatisticsCall, c.region, c.accountID, c.role, namespace); err != nil {
		return nil
	}
	return c.Client.GetMetricStatistics(ctx, logger, dimensions, namespace, metric)
}

func (c *SimpleRateLimitedClient) limit(ctx context.Context, buckets *rateLimiterBuckets, apiName string, region string, accountID string, role string, namespace string) error {
	if buckets == nil {
		return nil
	}
	limiter := buckets.get(accountID, region)
	if limiter.Allow() {
		promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(apiName, region, accountID, role, namespace).Inc()
		return nil
	}
	promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(apiName, region, accountID, role, namespace).Inc()
	return limiter.Wait(ctx)
}
