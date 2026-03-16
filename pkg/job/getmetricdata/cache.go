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
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/segmentio/fasthash/fnv1a"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// TimeseriesCacheEntry holds the cached state for a single CloudWatch timeseries.
type TimeseriesCacheEntry struct {
	// LastTimestamp is the most recent data point timestamp observed for this timeseries.
	LastTimestamp time.Time
	// Interval is the metric's period/interval in seconds, stored so we can make gap decisions
	// without re-reading request params.
	Interval int64
}

// TimeseriesCache is a TTL-based cache that tracks the last seen timestamp and interval
// per CloudWatch timeseries. It is used to deduplicate data points across scrapes and
// to detect gaps that require wider lookback windows.
type TimeseriesCache struct {
	cache *ttlcache.Cache[uint64, TimeseriesCacheEntry]
}

// NewTimeseriesCache creates a new TimeseriesCache with no default TTL.
// Per-entry TTLs are set on each Set call based on the metric's period.
// It starts the automatic cleanup goroutine provided by ttlcache.
func NewTimeseriesCache() *TimeseriesCache {
	cache := ttlcache.New[uint64, TimeseriesCacheEntry](
		ttlcache.WithTTL[uint64, TimeseriesCacheEntry](ttlcache.NoTTL),
	)
	go cache.Start()
	return &TimeseriesCache{cache: cache}
}

// Get retrieves the cached entry for the given key. Returns the entry and true if found,
// or a zero-value entry and false if not found or expired.
func (tc *TimeseriesCache) Get(key uint64) (TimeseriesCacheEntry, bool) {
	item := tc.cache.Get(key)
	if item == nil {
		return TimeseriesCacheEntry{}, false
	}
	return item.Value(), true
}

// Set stores or updates the cache entry for the given key with the given TTL.
func (tc *TimeseriesCache) Set(key uint64, entry TimeseriesCacheEntry, ttl time.Duration) {
	tc.cache.Set(key, entry, ttl)
}

// Stop halts the automatic cleanup goroutine. Should be called on shutdown.
func (tc *TimeseriesCache) Stop() {
	tc.cache.Stop()
}

// BuildCacheKey constructs a unique cache key from the CloudWatch timeseries identity:
// namespace, metric name, dimensions, and statistic.
// Uses FNV-1a hash for zero-allocation key computation.
// Each field is hashed independently and XORed together, making the result order-independent.
func BuildCacheKey(namespace string, metricName string, dimensions []model.Dimension, statistic string) uint64 {
	return BuildCacheKeyWithPrefix("", namespace, metricName, dimensions, statistic)
}

func BuildCacheKeyWithPrefix(prefix string, namespace string, metricName string, dimensions []model.Dimension, statistic string) uint64 {
	result := fnvHashKeyValue("namespace", namespace)
	result ^= fnvHashKeyValue("metric", metricName)
	result ^= fnvHashKeyValue("statistic", statistic)

	if prefix != "" {
		result ^= fnvHashKeyValue("prefix", prefix)
	}

	for _, d := range dimensions {
		result ^= fnvHashKeyValue(d.Name, d.Value)
	}

	return result
}

// fnvHashKeyValue hashes a key-value pair into a single uint64 using FNV-1a.
func fnvHashKeyValue(key, value string) uint64 {
	hash := fnv1a.HashString64(key)
	hash = fnv1a.AddString64(hash, value)
	return hash
}
