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
	"log/slog"
	"math"
	"time"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// steadyStateThreshold returns the maximum timeSinceSeconds that is still considered
// "steady state" for a given period. It accounts for the delay we apply (which pushes
// cached timestamps into the past) plus 2 periods of buffer for scrape-interval jitter.
// If timeSinceSeconds exceeds this, the metric has a genuine gap.
func steadyStateThreshold(period int64) int64 {
	return effectiveDelay(period) + 2*period
}

// CachingProcessorConfig holds configuration for the CachingProcessor.
type CachingProcessorConfig struct {
	// KeyPrefix is mixed into every cache key to isolate cache entries between
	// different integration instances that may scrape the same CloudWatch metrics.
	// Without a prefix, two integrations scraping the same metric would share
	// cache state, causing incorrect deduplication.
	// Typically set to the integration name or ID.
	KeyPrefix string

	// MinPeriods is the number of periods to look back on cache miss (cold start).
	// On the first scrape for a timeseries there is no cached state, so we query
	// this many periods. Keeping it at 1 means "just fetch 1 period; if CW has
	// nothing yet the next scrape will try again".
	// Default: 1
	MinPeriods int64

	// MaxPeriods is the maximum number of periods the lookback window can grow to
	// on cache hit. When a gap is detected the window expands from 1 period up to
	// MaxPeriods * period, then stays capped there. This prevents excessively
	// large queries after long outages while still recovering short gaps.
	// Default: 5
	MaxPeriods int64

	// GapValue, when non-nil, is the value emitted at cachedLastTimestamp + period
	// when CloudWatch returns zero data points for a cached series (gap termination).
	// Callers typically pass a pointer to decimal.StaleNaN for VictoriaMetrics.
	// Default: nil (no gap value emitted).
	GapValue *float64
}

// DefaultCachingProcessorConfig returns a CachingProcessorConfig with sensible defaults.
func DefaultCachingProcessorConfig() CachingProcessorConfig {
	return CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
}

// requestMetadata holds pre-computed information about a request that we need
// after the inner processor clears GetMetricDataProcessingParams.
type requestMetadata struct {
	cacheKey  uint64
	period    int64
	statistic string
}

// CachingProcessor wraps an inner processor (typically getmetricdata.Processor) and adds
// a timeseries cache layer. It adjusts the lookback window to at least minPeriods * period,
// detects gaps via the cache to extend the window further, and deduplicates data points
// that have already been seen in previous scrapes.
type CachingProcessor struct {
	inner  Processor
	cache  *TimeseriesCache
	config CachingProcessorConfig
	logger *slog.Logger
}

// NewCachingProcessor creates a CachingProcessor that wraps the given inner processor
// with cache-based window adjustment and deduplication.
func NewCachingProcessor(logger *slog.Logger, inner Processor, cache *TimeseriesCache, config CachingProcessorConfig) *CachingProcessor {
	return &CachingProcessor{
		inner:  inner,
		cache:  cache,
		config: config,
		logger: logger,
	}
}

// Run implements the same interface as Processor.Run. It adjusts the lookback window
// on each request based on cached state, delegates to the inner processor, then
// deduplicates results and updates the cache.
func (cp *CachingProcessor) Run(ctx context.Context, namespace string, requests []*model.CloudwatchData) ([]*model.CloudwatchData, error) {
	if len(requests) == 0 {
		return requests, nil
	}

	metadata := cp.adjustRequestWindows(namespace, requests)

	// Delegate to inner processor. Steady-state metrics have delay=period and gapped
	// metrics have delay=0, so the Iterator naturally creates separate batches with
	// different time windows — no manual splitting needed.
	results, err := cp.inner.Run(ctx, namespace, requests)
	if err != nil {
		return nil, err
	}

	cp.deduplicateAndUpdateCache(results, metadata)

	return results, nil
}

// effectiveMaxPeriods returns the maximum number of lookback periods based on the metric
// period. Short-period metrics can afford more lookback; long-period metrics are capped lower
// to avoid expensive queries.
//
//	Period 1-3 min  (60-180s)  → 10 periods
//	Period 4-10 min (240-600s) → 5  periods
//	Period > 10 min (>600s)    → 2  periods
func effectiveMaxPeriods(periodSeconds int64) int64 {
	switch {
	case periodSeconds <= 180: // 1-3 minutes
		return 10
	case periodSeconds <= 600: // 4-10 minutes
		return 5
	default: // > 10 minutes
		return 2
	}
}

// CacheTTL computes the per-entry TTL for the timeseries cache based on the metric period.
// The TTL equals (effectiveMaxPeriods + 1) * period so that entries survive long enough
// to cover the full backfill window plus one additional period of headroom.
// For very long periods (> 4 hours) the TTL is capped at 2 * period.
func CacheTTL(periodSeconds int64) time.Duration {
	const fourHours = 4 * 3600
	if periodSeconds > fourHours {
		return time.Duration(2*periodSeconds) * time.Second
	}
	return time.Duration((effectiveMaxPeriods(periodSeconds)+1)*periodSeconds) * time.Second
}

// effectiveDelay returns the query delay (in seconds) based on the metric period.
// Short-period metrics need more delay because CloudWatch publishing latency (~2 min)
// is significant relative to their period. Long-period metrics don't need delay since
// the data is already well in the past by the time we query.
//
//	Period ≤ 4 min  (≤240s) → 120s (2 minutes)
//	Period > 4 min  (>240s) → 300s (5 minutes)
func effectiveDelay(periodSeconds int64) int64 {
	switch {
	case periodSeconds <= 240: // ≤ 4 minutes
		return 120
	default: // > 4 minutes
		return 300
	}
}

// adjustRequestWindows sets the Length and Delay on each request based on cached state.
// Returns metadata for post-processing.
func (cp *CachingProcessor) adjustRequestWindows(namespace string, requests []*model.CloudwatchData) map[*model.CloudwatchData]requestMetadata {
	metadata := make(map[*model.CloudwatchData]requestMetadata, len(requests))
	now := time.Now()

	for _, req := range requests {
		if req.GetMetricDataProcessingParams == nil {
			continue
		}

		period := req.GetMetricDataProcessingParams.Period
		statistic := req.GetMetricDataProcessingParams.Statistic
		key := BuildCacheKeyWithPrefix(cp.config.KeyPrefix, namespace, req.MetricName, req.Dimensions, statistic)

		metadata[req] = requestMetadata{
			cacheKey:  key,
			period:    period,
			statistic: statistic,
		}

		cached, ok := cp.cache.Get(key)
		if !ok {
			promutil.TimeseriesCacheMissCounter.Inc()
			cp.applyColdStartWindow(req, period)
			continue
		}

		promutil.TimeseriesCacheHitCounter.Inc()

		periodDuration := time.Duration(period) * time.Second
		roundedNow := now.Truncate(periodDuration)
		timeSinceSeconds := int64(roundedNow.Sub(cached.LastTimestamp).Seconds())
		if timeSinceSeconds <= steadyStateThreshold(period) {
			cp.applySteadyStateWindow(req, period)
		} else {
			maxPeriods := effectiveMaxPeriods(period)
			cp.applyGapRecoveryWindow(req, namespace, period, timeSinceSeconds, period*maxPeriods)
		}
	}

	return metadata
}

// applyColdStartWindow sets the window for a metric with no cached state.
// Uses MinPeriods lookback with a capped delay to fetch guaranteed-published data.
func (cp *CachingProcessor) applyColdStartWindow(req *model.CloudwatchData, period int64) {
	req.GetMetricDataProcessingParams.Length = period * cp.config.MinPeriods
	req.GetMetricDataProcessingParams.Delay = effectiveDelay(period)
}

// applySteadyStateWindow sets a tight 1-period window with delay for normal operation.
// The delay shifts the window into the past where CW has definitely published,
// eliminating empty fetches and producing adjacent non-overlapping windows
// between consecutive scrapes (zero dedup waste).
func (cp *CachingProcessor) applySteadyStateWindow(req *model.CloudwatchData, period int64) {
	req.GetMetricDataProcessingParams.Length = period
	req.GetMetricDataProcessingParams.Delay = effectiveDelay(period)
}

// applyGapRecoveryWindow sets an extended window to recover missed data after a gap.
// Uses delay=0 to reach as close to "now" as possible, with length capped at maxLength.
func (cp *CachingProcessor) applyGapRecoveryWindow(req *model.CloudwatchData, namespace string, period, timeSinceSeconds, maxLength int64) {
	promutil.TimeseriesCacheGapDetectedCounter.Inc()
	neededLength := timeSinceSeconds + period
	if neededLength > maxLength {
		promutil.TimeseriesCacheGapCappedCounter.Inc()
		cp.logger.Warn("[GAP_CAPPED] lookback capped at max",
			"namespace", namespace,
			"metric", req.MetricName,
			"key", BuildCacheKeyWithPrefix(cp.config.KeyPrefix, namespace, req.MetricName, req.Dimensions, req.GetMetricDataProcessingParams.Statistic),
			"needed_seconds", neededLength,
			"capped_to", maxLength,
		)
		neededLength = maxLength
	}
	req.GetMetricDataProcessingParams.Length = neededLength
	req.GetMetricDataProcessingParams.Delay = 0
}

// deduplicateAndUpdateCache filters out already-seen datapoints, generates NaN gap-fill
// points for missing periods, and advances the cache.
func (cp *CachingProcessor) deduplicateAndUpdateCache(results []*model.CloudwatchData, metadata map[*model.CloudwatchData]requestMetadata) {
	for _, result := range results {
		if result.GetMetricDataResult == nil {
			continue
		}

		meta, ok := metadata[result]
		if !ok {
			continue
		}

		cp.filterSeenDataPoints(result, meta)
		cp.fillGapsWithNaN(result, meta)
		cp.updateCacheFromResult(result, meta)
	}
}

// filterSeenDataPoints filters out datapoints at or before the cached timestamp.
func (cp *CachingProcessor) filterSeenDataPoints(result *model.CloudwatchData, meta requestMetadata) {
	cached, hasCached := cp.cache.Get(meta.cacheKey)
	if !hasCached {
		return
	}

	totalRaw := len(result.GetMetricDataResult.DataPoints)
	filtered := make([]model.DataPoint, 0, totalRaw)
	for _, dp := range result.GetMetricDataResult.DataPoints {
		if dp.Timestamp.After(cached.LastTimestamp) {
			filtered = append(filtered, dp)
		}
	}

	if dedupCount := totalRaw - len(filtered); dedupCount > 0 {
		promutil.TimeseriesDedupCounter.Add(float64(dedupCount))
	}

	result.GetMetricDataResult.DataPoints = filtered
}

// fillGapsWithNaN injects a single gap-value data point at cachedLastTimestamp + period
// when a gap is detected and CloudWatch returned zero real data points after deduplication.
// The value used is provided by the caller via CachingProcessorConfig.GapValue (typically
// decimal.StaleNaN for VictoriaMetrics staleness termination).
//
// The gap value is only emitted when:
//  1. GapValue is non-nil in config
//  2. A cache entry exists (not a cold start)
//  3. Zero real data points remain after deduplication (the series went silent)
func (cp *CachingProcessor) fillGapsWithNaN(result *model.CloudwatchData, meta requestMetadata) {
	if cp.config.GapValue == nil {
		return
	}

	dps := result.GetMetricDataResult.DataPoints

	// Only emit gap value when CW returned zero real data points — the series went silent.
	if len(dps) > 0 {
		return
	}

	cached, hasCached := cp.cache.Get(meta.cacheKey)
	if !hasCached {
		return
	}

	periodDuration := time.Duration(meta.period) * time.Second
	gapVal := *cp.config.GapValue
	nanTS := cached.LastTimestamp.Add(periodDuration).Truncate(periodDuration)
	dps = append(dps, model.DataPoint{Value: &gapVal, Timestamp: nanTS})

	promutil.TimeseriesGapFillPointsCounter.Add(1)
	result.GetMetricDataResult.DataPoints = dps
}

// updateCacheFromResult advances the cache to the newest real (non-NaN) datapoint timestamp.
// NaN gap-fill points are excluded so the cache only advances based on actual CloudWatch data.
func (cp *CachingProcessor) updateCacheFromResult(result *model.CloudwatchData, meta requestMetadata) {
	var newestTimestamp time.Time
	for _, dp := range result.GetMetricDataResult.DataPoints {
		if dp.Value != nil && math.IsNaN(*dp.Value) {
			continue
		}
		if dp.Timestamp.After(newestTimestamp) {
			newestTimestamp = dp.Timestamp
		}
	}
	if !newestTimestamp.IsZero() {
		cp.cache.Set(meta.cacheKey, TimeseriesCacheEntry{
			LastTimestamp: newestTimestamp,
			Interval:      meta.period,
		}, CacheTTL(meta.period))
	}
}
