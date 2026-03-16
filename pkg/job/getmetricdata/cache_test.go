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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestBuildCacheKey_Deterministic(t *testing.T) {
	key1 := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	key2 := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	assert.Equal(t, key1, key2, "same inputs should produce same hash")
}

func TestBuildCacheKey_DifferentInputs(t *testing.T) {
	key1 := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	key2 := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-456"}}, "Average")
	assert.NotEqual(t, key1, key2, "different dimensions should produce different hashes")

	key3 := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Sum")
	assert.NotEqual(t, key1, key3, "different statistics should produce different hashes")

	key4 := BuildCacheKey("AWS/RDS", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	assert.NotEqual(t, key1, key4, "different namespaces should produce different hashes")
}

func TestBuildCacheKey_DimensionOrderDoesNotMatter(t *testing.T) {
	key1 := BuildCacheKey("AWS/EC2", "NetworkIn", []model.Dimension{
		{Name: "B", Value: "2"},
		{Name: "A", Value: "1"},
		{Name: "C", Value: "3"},
	}, "Sum")
	key2 := BuildCacheKey("AWS/EC2", "NetworkIn", []model.Dimension{
		{Name: "C", Value: "3"},
		{Name: "A", Value: "1"},
		{Name: "B", Value: "2"},
	}, "Sum")
	assert.Equal(t, key1, key2, "dimension order should not affect the hash")
}

func TestBuildCacheKey_DimensionsNotMutated(t *testing.T) {
	dims := []model.Dimension{
		{Name: "Z", Value: "last"},
		{Name: "A", Value: "first"},
	}
	original := make([]model.Dimension, len(dims))
	copy(original, dims)

	BuildCacheKey("ns", "metric", dims, "stat")

	assert.Equal(t, original, dims)
}

func TestBuildCacheKey_NoDimensions(t *testing.T) {
	key1 := BuildCacheKey("AWS/S3", "BucketSizeBytes", []model.Dimension{}, "Average")
	key2 := BuildCacheKey("AWS/S3", "BucketSizeBytes", []model.Dimension{}, "Average")
	assert.Equal(t, key1, key2)
}

func TestBuildCacheKeyWithPrefix_DifferentPrefixesDifferentKeys(t *testing.T) {
	dims := []model.Dimension{{Name: "FunctionName", Value: "HelloLogger"}}
	key1 := BuildCacheKeyWithPrefix("integration-a", "AWS/Lambda", "Invocations", dims, "Sum")
	key2 := BuildCacheKeyWithPrefix("integration-b", "AWS/Lambda", "Invocations", dims, "Sum")
	assert.NotEqual(t, key1, key2, "different prefixes should produce different keys")
}

func TestBuildCacheKeyWithPrefix_SamePrefixSameKey(t *testing.T) {
	dims := []model.Dimension{{Name: "FunctionName", Value: "HelloLogger"}}
	key1 := BuildCacheKeyWithPrefix("my-integration", "AWS/Lambda", "Invocations", dims, "Sum")
	key2 := BuildCacheKeyWithPrefix("my-integration", "AWS/Lambda", "Invocations", dims, "Sum")
	assert.Equal(t, key1, key2, "same prefix and inputs should produce same key")
}

func TestBuildCacheKeyWithPrefix_EmptyPrefixMatchesNonPrefixed(t *testing.T) {
	dims := []model.Dimension{{Name: "InstanceId", Value: "i-123"}}
	keyNoPrefix := BuildCacheKey("AWS/EC2", "CPUUtilization", dims, "Average")
	keyEmptyPrefix := BuildCacheKeyWithPrefix("", "AWS/EC2", "CPUUtilization", dims, "Average")
	assert.Equal(t, keyNoPrefix, keyEmptyPrefix, "empty prefix should match non-prefixed key")
}

func TestBuildCacheKeyWithPrefix_PrefixIsolatesSameMetric(t *testing.T) {
	dims := []model.Dimension{{Name: "FunctionName", Value: "HelloLogger"}}
	// Simulate two integrations scraping the same metric
	key1 := BuildCacheKeyWithPrefix("test-erez-5", "AWS/Lambda", "Invocations", dims, "Sum")
	key2 := BuildCacheKeyWithPrefix("aviv-test-cw", "AWS/Lambda", "Invocations", dims, "Sum")
	keyNoPrefix := BuildCacheKey("AWS/Lambda", "Invocations", dims, "Sum")

	assert.NotEqual(t, key1, key2, "different integrations should have different keys")
	assert.NotEqual(t, key1, keyNoPrefix, "prefixed key should differ from non-prefixed")
	assert.NotEqual(t, key2, keyNoPrefix, "prefixed key should differ from non-prefixed")
}

func TestTimeseriesCache_GetSet(t *testing.T) {
	cache := NewTimeseriesCache()
	defer cache.Stop()

	key := BuildCacheKey("test", "metric", []model.Dimension{{Name: "dim", Value: "val"}}, "Average")
	ts := time.Now()

	_, ok := cache.Get(key)
	require.False(t, ok)

	cache.Set(key, TimeseriesCacheEntry{
		LastTimestamp: ts,
		Interval:      60,
	}, 1*time.Hour)

	entry, ok := cache.Get(key)
	require.True(t, ok)
	assert.Equal(t, ts, entry.LastTimestamp)
	assert.Equal(t, int64(60), entry.Interval)
}

func TestTimeseriesCache_TTLExpiry(t *testing.T) {
	cache := NewTimeseriesCache()
	defer cache.Stop()

	key := BuildCacheKey("test", "metric", []model.Dimension{{Name: "dim", Value: "val"}}, "Average")
	cache.Set(key, TimeseriesCacheEntry{
		LastTimestamp: time.Now(),
		Interval:      60,
	}, 50*time.Millisecond)

	_, ok := cache.Get(key)
	require.True(t, ok)

	time.Sleep(100 * time.Millisecond)

	_, ok = cache.Get(key)
	assert.False(t, ok, "entry should have been evicted after TTL")
}

func TestTimeseriesCache_Update(t *testing.T) {
	cache := NewTimeseriesCache()
	defer cache.Stop()

	key := BuildCacheKey("test", "metric", []model.Dimension{{Name: "dim", Value: "val"}}, "Average")
	ts1 := time.Now().Add(-5 * time.Minute)
	ts2 := time.Now()

	cache.Set(key, TimeseriesCacheEntry{LastTimestamp: ts1, Interval: 60}, 1*time.Hour)
	entry, ok := cache.Get(key)
	require.True(t, ok)
	assert.Equal(t, ts1, entry.LastTimestamp)

	cache.Set(key, TimeseriesCacheEntry{LastTimestamp: ts2, Interval: 60}, 1*time.Hour)
	entry, ok = cache.Get(key)
	require.True(t, ok)
	assert.Equal(t, ts2, entry.LastTimestamp)
}
