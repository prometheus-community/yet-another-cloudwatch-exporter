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
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func float64Ptr(v float64) *float64 { return &v }

func TestCachingProcessor_AdjustsLengthToMinPeriods(t *testing.T) {
	// A metric with period=60, length=60 should get extended to length=300 (5*60)
	now := time.Now()
	cache := NewTimeseriesCache()
	defer cache.Stop()

	var capturedLength int64
	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			// Capture the length that was set on the data
			if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
				capturedLength = data[0].GetMetricDataProcessingParams.Length
			}
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60, // Will stay at 60 (1 * 60, cold start with MinPeriods=1)
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// The length should have been adjusted to 1 * 60 = 60 (MinPeriods=1)
	assert.Equal(t, int64(60), capturedLength)
}

func TestCachingProcessor_DeduplicatesDataPoints(t *testing.T) {
	now := time.Now()
	oldTimestamp := now.Add(-5 * time.Minute)
	newTimestamp := now.Add(-1 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	// Pre-populate cache with old timestamp
	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: oldTimestamp,
		Interval:      60,
	}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(50), Timestamp: oldTimestamp},                   // Should be filtered (not after cached)
						{Value: aws.Float64(70), Timestamp: newTimestamp},                   // Should be kept
						{Value: aws.Float64(30), Timestamp: oldTimestamp.Add(-time.Minute)}, // Should be filtered (before cached)
					},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    300,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Only the new real data point should be present (no NaN since GapValue is nil by default)
	dps := results[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps, 1, "should have exactly 1 real data point")
	assert.Equal(t, float64(70), *dps[0].Value)
	assert.Equal(t, newTimestamp, dps[0].Timestamp)

	// Cache should be updated with newest timestamp
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, newTimestamp, entry.LastTimestamp)
}

func TestCachingProcessor_GapDetection(t *testing.T) {
	now := time.Now()
	// Simulate a gap: last seen 10 minutes ago, period is 60 seconds
	lastSeen := now.Add(-10 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastSeen,
		Interval:      60,
	}, 1*time.Hour)

	var capturedLength int64
	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
				capturedLength = data[0].GetMetricDataProcessingParams.Length
			}
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now.Add(-1 * time.Minute)}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	_, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)

	// Gap is ~600 seconds, effectiveMaxPeriods(60)=10 → cap at 10*60=600.
	assert.Equal(t, int64(600), capturedLength, "length should be capped at effectiveMaxPeriods * period")
}

func TestCachingProcessor_GapCappedByMaxPeriods(t *testing.T) {
	now := time.Now()
	// Simulate a very large gap: 2 hours ago
	lastSeen := now.Add(-2 * time.Hour)

	cache := NewTimeseriesCache() // TTL larger than gap for test
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastSeen,
		Interval:      60,
	}, 1*time.Hour)

	var capturedLength int64
	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
				capturedLength = data[0].GetMetricDataProcessingParams.Length
			}
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now.Add(-1 * time.Minute)}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	_, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)

	// Gap is 2 hours, effectiveMaxPeriods(60)=10 → cap at 10*60=600
	assert.Equal(t, int64(600), capturedLength, "length should be capped at effectiveMaxPeriods * period")
}

func TestCachingProcessor_CacheMiss_NoDedup(t *testing.T) {
	now := time.Now()
	cache := NewTimeseriesCache()
	defer cache.Stop()

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(10), Timestamp: now.Add(-4 * time.Minute)},
						{Value: aws.Float64(20), Timestamp: now.Add(-3 * time.Minute)},
						{Value: aws.Float64(30), Timestamp: now.Add(-2 * time.Minute)},
					},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    300,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// All 3 data points should be kept (no cache entry to dedup against), no NaN (flag off)
	dps := results[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps, 3, "3 real data points kept, no NaN")

	// Cache should now have the newest timestamp
	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, now.Add(-2*time.Minute), entry.LastTimestamp)
	assert.Equal(t, int64(60), entry.Interval)
}

func TestCachingProcessor_EmptyRequests(t *testing.T) {
	cache := NewTimeseriesCache()
	defer cache.Stop()

	inner := NewDefaultProcessor(promslog.NewNopLogger(), testClient{}, 500, 1)
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

	results, err := cp.Run(context.Background(), "AWS/EC2", []*model.CloudwatchData{})
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCachingProcessor_UpdatesCacheWithInterval(t *testing.T) {
	now := time.Now()
	cache := NewTimeseriesCache()
	defer cache.Stop()

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

	requests := []*model.CloudwatchData{
		{
			MetricName:   "RequestCount",
			ResourceName: "my-lb",
			Namespace:    "AWS/ELB",
			Dimensions:   []model.Dimension{{Name: "LoadBalancer", Value: "my-lb"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    300,
				Length:    1500,
				Delay:     0,
				Statistic: "Sum",
			},
		},
	}

	_, err := cp.Run(context.Background(), "AWS/ELB", requests)
	require.NoError(t, err)

	cacheKey := BuildCacheKey("AWS/ELB", "RequestCount", []model.Dimension{{Name: "LoadBalancer", Value: "my-lb"}}, "Sum")
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, int64(300), entry.Interval, "interval should match the metric's period")
}

// =============================================================================
// Smart Lookback Window Tests
// =============================================================================

func TestCachingProcessor_SmartLookback_ColdStart(t *testing.T) {
	// Cold start (no cache entry) should use MinPeriods * period
	// Default MinPeriods=1 means fetch exactly 1 period on first scrape
	testCases := []struct {
		name       string
		period     int64
		minPeriods int64
		expected   int64
	}{
		{"60s period, 1 minPeriod (default)", 60, 1, 60},
		{"300s period, 1 minPeriod (default)", 300, 1, 300},
		{"60s period, 2 minPeriods", 60, 2, 120},
		{"120s period, 3 minPeriods", 120, 3, 360},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			cache := NewTimeseriesCache()
			defer cache.Stop()

			var capturedLength int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedLength = data[0].GetMetricDataProcessingParams.Length
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			config := CachingProcessorConfig{
				MinPeriods: tc.minPeriods,
				MaxPeriods: 5,
			}
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

			requests := []*model.CloudwatchData{
				{
					MetricName:   "TestMetric",
					ResourceName: "test-resource",
					Namespace:    "AWS/Test",
					Dimensions:   []model.Dimension{{Name: "TestDim", Value: "test-val"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    tc.period,
						Length:    tc.period, // Original length doesn't matter on cold start
						Delay:     0,
						Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, capturedLength, "cold start should use minPeriods * period")
		})
	}
}

func TestCachingProcessor_SmartLookback_SteadyState(t *testing.T) {
	// Steady state: cache hit with recent timestamp
	// If no gap (timeSinceLast <= period): fetch only 1 period
	// If gap (timeSinceLast > period): fetch timeSinceLast + period
	testCases := []struct {
		name              string
		period            int64
		timeSinceLast     time.Duration
		expectedLengthMin int64 // Allow some timing variance
		expectedLengthMax int64
	}{
		{
			name:              "60s period, 1 minute since last",
			period:            60,
			timeSinceLast:     1 * time.Minute,
			expectedLengthMin: 55, // 1 period (60s) with timing variance
			expectedLengthMax: 65,
		},
		{
			name:              "60s period, 2 minutes since last",
			period:            60,
			timeSinceLast:     2 * time.Minute,
			expectedLengthMin: 55, // steady state: 1 period with delay (120s <= 3*period)
			expectedLengthMax: 65,
		},
		{
			name:              "300s period, 5 minutes since last",
			period:            300,
			timeSinceLast:     5 * time.Minute,
			expectedLengthMin: 295, // 1 period (300s) with timing variance (no gap)
			expectedLengthMax: 305,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			lastTimestamp := now.Add(-tc.timeSinceLast)

			cache := NewTimeseriesCache()
			defer cache.Stop()

			// Pre-populate cache
			cacheKey := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "TestDim", Value: "test-val"}}, "Average")
			cache.Set(cacheKey, TimeseriesCacheEntry{
				LastTimestamp: lastTimestamp,
				Interval:      tc.period,
			}, 1*time.Hour)

			var capturedLength int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedLength = data[0].GetMetricDataProcessingParams.Length
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			config := CachingProcessorConfig{
				MinPeriods: 1,
				MaxPeriods: 5,
			}
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

			requests := []*model.CloudwatchData{
				{
					MetricName:   "TestMetric",
					ResourceName: "test-resource",
					Namespace:    "AWS/Test",
					Dimensions:   []model.Dimension{{Name: "TestDim", Value: "test-val"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    tc.period,
						Length:    tc.period * 5, // Original length should be ignored
						Delay:     0,
						Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, capturedLength, tc.expectedLengthMin,
				"length should match expected range")
			assert.LessOrEqual(t, capturedLength, tc.expectedLengthMax,
				"length should match expected range")
		})
	}
}

func TestCachingProcessor_SmartLookback_GapWithinLimits(t *testing.T) {
	// Gap detected but within MaxPeriods * period - should extend to cover gap
	now := time.Now()
	// 3 minute gap with period=60 and MaxPeriods=5 → needed=240s, cap=300s → fits
	lastTimestamp := now.Add(-3 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "TestDim", Value: "test-val"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastTimestamp,
		Interval:      60,
	}, 1*time.Hour)

	var capturedLength int64
	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
				capturedLength = data[0].GetMetricDataProcessingParams.Length
			}
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "TestMetric",
			ResourceName: "test-resource",
			Namespace:    "AWS/Test",
			Dimensions:   []model.Dimension{{Name: "TestDim", Value: "test-val"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	_, err := cp.Run(context.Background(), "AWS/Test", requests)
	require.NoError(t, err)

	// 3 min gap (180s) <= 3*period (180s) → steady state with delay
	// length = period = 60
	assert.Equal(t, int64(60), capturedLength, "3-min gap is within steady-state threshold (3*period)")
}

func TestCachingProcessor_SmartLookback_MaxPeriodsCapping(t *testing.T) {
	// Gap exceeds effectiveMaxPeriods * period - should be capped
	testCases := []struct {
		name           string
		gapDuration    time.Duration
		period         int64
		expectedLength int64
	}{
		{
			name:           "2 hour gap with 60s period",
			gapDuration:    2 * time.Hour,
			period:         60,
			expectedLength: 600, // effectiveMaxPeriods(60)=10 → 10*60
		},
		{
			name:           "1 day gap with 300s period",
			gapDuration:    24 * time.Hour,
			period:         300,
			expectedLength: 1500, // effectiveMaxPeriods(300)=5 → 5*300
		},
		{
			name:           "6 hour gap with 3600s period",
			gapDuration:    6 * time.Hour,
			period:         3600,
			expectedLength: 7200, // effectiveMaxPeriods(3600)=2 → 2*3600
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			lastTimestamp := now.Add(-tc.gapDuration)

			cache := NewTimeseriesCache() // Large TTL to not evict
			defer cache.Stop()

			cacheKey := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "TestDim", Value: "test-val"}}, "Average")
			cache.Set(cacheKey, TimeseriesCacheEntry{
				LastTimestamp: lastTimestamp,
				Interval:      tc.period,
			}, 1*time.Hour)

			var capturedLength int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedLength = data[0].GetMetricDataProcessingParams.Length
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			config := CachingProcessorConfig{
				MinPeriods: 1,
				MaxPeriods: 5,
			}
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

			requests := []*model.CloudwatchData{
				{
					MetricName:   "TestMetric",
					ResourceName: "test-resource",
					Namespace:    "AWS/Test",
					Dimensions:   []model.Dimension{{Name: "TestDim", Value: "test-val"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    tc.period,
						Length:    tc.period,
						Delay:     0,
						Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedLength, capturedLength, "length should be capped at effectiveMaxPeriods * period")
		})
	}
}

func TestCachingProcessor_SmartLookback_MixedCacheStates(t *testing.T) {
	// Multiple requests: some with cache hits, some misses
	now := time.Now()
	cache := NewTimeseriesCache()
	defer cache.Stop()

	// Pre-populate cache for metric1 only
	cacheKey1 := BuildCacheKey("AWS/Test", "Metric1", []model.Dimension{{Name: "Dim", Value: "val1"}}, "Average")
	cache.Set(cacheKey1, TimeseriesCacheEntry{
		LastTimestamp: now.Add(-1 * time.Minute), // 1 minute ago
		Interval:      60,
	}, 1*time.Hour)
	// Metric2 has no cache entry (cold start)

	capturedLengths := make(map[string]int64)
	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				if d.GetMetricDataProcessingParams != nil {
					capturedLengths[d.MetricName] = d.GetMetricDataProcessingParams.Length
				}
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "Metric1", // Has cache entry
			ResourceName: "test-resource",
			Namespace:    "AWS/Test",
			Dimensions:   []model.Dimension{{Name: "Dim", Value: "val1"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
		{
			MetricName:   "Metric2", // No cache entry (cold start)
			ResourceName: "test-resource",
			Namespace:    "AWS/Test",
			Dimensions:   []model.Dimension{{Name: "Dim", Value: "val2"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	_, err := cp.Run(context.Background(), "AWS/Test", requests)
	require.NoError(t, err)

	// Metric1: cache hit, should have small length (~120s = 60s gap + 60s buffer)
	assert.Less(t, capturedLengths["Metric1"], int64(200),
		"cache hit should result in small lookback")

	// Metric2: cold start, should have MinPeriods length (60s = 1 * 60)
	assert.Equal(t, int64(60), capturedLengths["Metric2"],
		"cold start should use minPeriods * period")
}

func TestCachingProcessor_SmartLookback_EfficiencyComparison(t *testing.T) {
	// This test demonstrates the efficiency improvement
	// In old behavior: always query 5 periods
	// In new behavior: query based on actual gap
	now := time.Now()

	testCases := []struct {
		name              string
		timeSinceLast     time.Duration
		period            int64
		oldBehaviorLength int64 // Always 5 * period
		maxNewLength      int64 // Should be much smaller in steady state
	}{
		{
			name:              "1 minute scrape interval",
			timeSinceLast:     1 * time.Minute,
			period:            60,
			oldBehaviorLength: 300, // 5 * 60
			maxNewLength:      150, // ~60s gap + 60s buffer + variance
		},
		{
			name:              "30 second scrape interval",
			timeSinceLast:     30 * time.Second,
			period:            60,
			oldBehaviorLength: 300, // 5 * 60
			maxNewLength:      130, // ~30s gap + 60s buffer + variance
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewTimeseriesCache()
			defer cache.Stop()

			cacheKey := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "Dim", Value: "val"}}, "Average")
			cache.Set(cacheKey, TimeseriesCacheEntry{
				LastTimestamp: now.Add(-tc.timeSinceLast),
				Interval:      tc.period,
			}, 1*time.Hour)

			var capturedLength int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedLength = data[0].GetMetricDataProcessingParams.Length
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			config := CachingProcessorConfig{
				MinPeriods: 1,
				MaxPeriods: 5,
			}
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

			requests := []*model.CloudwatchData{
				{
					MetricName:   "TestMetric",
					ResourceName: "test-resource",
					Namespace:    "AWS/Test",
					Dimensions:   []model.Dimension{{Name: "Dim", Value: "val"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    tc.period,
						Length:    tc.period * 5,
						Delay:     0,
						Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)

			assert.Less(t, capturedLength, tc.oldBehaviorLength,
				"new smart lookback should be more efficient than old 5-period lookback")
			assert.LessOrEqual(t, capturedLength, tc.maxNewLength,
				"length should be close to actual gap + buffer")

			// Calculate efficiency improvement
			savings := float64(tc.oldBehaviorLength-capturedLength) / float64(tc.oldBehaviorLength) * 100
			t.Logf("Efficiency improvement: %.1f%% reduction (old: %ds, new: %ds)",
				savings, tc.oldBehaviorLength, capturedLength)
		})
	}
}

func TestCachingProcessor_SmartLookback_ConsecutiveScrapes(t *testing.T) {
	// Simulate multiple consecutive scrapes with realistic timing
	// Note: We use time.Now() relative timestamps since the processor uses time.Now() internally
	cache := NewTimeseriesCache()
	defer cache.Stop()

	period := int64(60)
	numScrapes := 3

	var lengthsPerScrape []int64

	for scrape := 0; scrape < numScrapes; scrape++ {
		var capturedLength int64
		now := time.Now()

		client := testClient{
			GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
				if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
					capturedLength = data[0].GetMetricDataProcessingParams.Length
				}
				results := make([]cloudwatch.MetricDataResult, 0, len(data))
				for _, d := range data {
					// Return data point at "now" - this will be cached
					results = append(results, cloudwatch.MetricDataResult{
						ID:         d.GetMetricDataProcessingParams.QueryID,
						DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
					})
				}
				return results
			},
		}

		inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
		config := CachingProcessorConfig{
			MinPeriods: 1,
			MaxPeriods: 5,
		}
		cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

		requests := []*model.CloudwatchData{
			{
				MetricName:   "ConsecutiveTestMetric",
				ResourceName: "test-resource",
				Namespace:    "AWS/Test",
				Dimensions:   []model.Dimension{{Name: "Dim", Value: "val"}},
				GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
					Period:    period,
					Length:    period,
					Delay:     0,
					Statistic: "Average",
				},
			},
		}

		_, err := cp.Run(context.Background(), "AWS/Test", requests)
		require.NoError(t, err)

		lengthsPerScrape = append(lengthsPerScrape, capturedLength)

		// Wait a tiny bit between scrapes to simulate real-world timing
		time.Sleep(10 * time.Millisecond)
	}

	// First scrape (cold start) should use MinPeriods (1 * 60 = 60)
	assert.Equal(t, int64(60), lengthsPerScrape[0], "first scrape should use MinPeriods")

	// Subsequent scrapes should use floor (1 period) since cache is very recent
	for i := 1; i < numScrapes; i++ {
		assert.Equal(t, period, lengthsPerScrape[i],
			"subsequent scrapes should use floor (1 period) when cache is very recent")
	}

	t.Logf("Lengths per scrape: %v", lengthsPerScrape)
}

func TestCachingProcessor_SmartLookback_FloorEnforcement(t *testing.T) {
	// Test that length never goes below 1 period (floor enforcement)
	// This handles edge cases like clock skew or very rapid scraping
	testCases := []struct {
		name           string
		timeSinceLast  time.Duration // Negative means cached timestamp is in the future
		period         int64
		expectedLength int64
	}{
		{
			name:           "cached timestamp in the future (clock skew)",
			timeSinceLast:  -5 * time.Minute, // 5 minutes in future
			period:         60,
			expectedLength: 60, // Floor at 1 period
		},
		{
			name:           "cached timestamp is now (instant scrape)",
			timeSinceLast:  0,
			period:         60,
			expectedLength: 60, // 0 + 60 buffer = 60, which is >= floor
		},
		{
			name:           "very recent timestamp (1ms ago)",
			timeSinceLast:  1 * time.Millisecond,
			period:         60,
			expectedLength: 60, // ~0 + 60 buffer = 60, which is >= floor
		},
		{
			name:           "larger period with negative gap",
			timeSinceLast:  -10 * time.Minute,
			period:         300,
			expectedLength: 300, // Floor at 1 period (300s)
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			cache := NewTimeseriesCache()
			defer cache.Stop()

			// Set cached timestamp based on timeSinceLast (can be negative)
			cachedTimestamp := now.Add(-tc.timeSinceLast)
			cacheKey := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "Dim", Value: "val"}}, "Average")
			cache.Set(cacheKey, TimeseriesCacheEntry{
				LastTimestamp: cachedTimestamp,
				Interval:      tc.period,
			}, 1*time.Hour)

			var capturedLength int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedLength = data[0].GetMetricDataProcessingParams.Length
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			config := CachingProcessorConfig{
				MinPeriods: 1,
				MaxPeriods: 5,
			}
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

			requests := []*model.CloudwatchData{
				{
					MetricName:   "TestMetric",
					ResourceName: "test-resource",
					Namespace:    "AWS/Test",
					Dimensions:   []model.Dimension{{Name: "Dim", Value: "val"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    tc.period,
						Length:    tc.period,
						Delay:     0,
						Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedLength, capturedLength,
				"length should respect floor of 1 period")
			assert.GreaterOrEqual(t, capturedLength, tc.period,
				"length should never be less than 1 period")
		})
	}
}

func TestCachingProcessor_SmartLookback_DifferentStatisticsSameMetric(t *testing.T) {
	// Same metric with different statistics should have separate cache entries
	now := time.Now()
	cache := NewTimeseriesCache()
	defer cache.Stop()

	// Pre-populate cache for Average only
	cacheKeyAvg := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "Dim", Value: "val"}}, "Average")
	cache.Set(cacheKeyAvg, TimeseriesCacheEntry{
		LastTimestamp: now.Add(-1 * time.Minute),
		Interval:      60,
	}, 1*time.Hour)
	// Sum has no cache entry

	capturedLengths := make(map[string]int64)
	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				if d.GetMetricDataProcessingParams != nil {
					key := d.MetricName + "_" + d.GetMetricDataProcessingParams.Statistic
					capturedLengths[key] = d.GetMetricDataProcessingParams.Length
				}
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "TestMetric",
			ResourceName: "test-resource",
			Namespace:    "AWS/Test",
			Dimensions:   []model.Dimension{{Name: "Dim", Value: "val"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
		{
			MetricName:   "TestMetric",
			ResourceName: "test-resource",
			Namespace:    "AWS/Test",
			Dimensions:   []model.Dimension{{Name: "Dim", Value: "val"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Sum",
			},
		},
	}

	_, err := cp.Run(context.Background(), "AWS/Test", requests)
	require.NoError(t, err)

	// Average: cache hit, small lookback
	assert.Less(t, capturedLengths["TestMetric_Average"], int64(200),
		"Average should have cache hit with small lookback")

	// Sum: cold start, MinPeriods=1 lookback (1 * 60 = 60)
	assert.Equal(t, int64(60), capturedLengths["TestMetric_Sum"],
		"Sum should use MinPeriods (cold start)")
}

func TestCachingProcessor_SmartLookback_BoundaryConditions(t *testing.T) {
	// Test boundary conditions with effectiveMaxPeriods cap
	testCases := []struct {
		name          string
		timeSinceLast time.Duration
		period        int64
		description   string
	}{
		{
			name:          "gap within effectiveMaxPeriods boundary",
			timeSinceLast: 3 * time.Minute, // 180s → steady state (≤ effectiveDelay(60)+2*60 = 240)
			period:        60,
			description:   "should not be capped (steady state)",
		},
		{
			name:          "gap exceeds effectiveMaxPeriods boundary",
			timeSinceLast: 20 * time.Minute, // 1200s gap, effectiveMaxPeriods(60)=10 → cap=600
			period:        60,
			description:   "should be capped at effectiveMaxPeriods",
		},
		{
			name:          "very short gap (faster than period)",
			timeSinceLast: 10 * time.Second,
			period:        60,
			description:   "should still add period buffer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			cache := NewTimeseriesCache()
			defer cache.Stop()

			cacheKey := BuildCacheKey("AWS/Test", "TestMetric", []model.Dimension{{Name: "Dim", Value: "val"}}, "Average")
			cache.Set(cacheKey, TimeseriesCacheEntry{
				LastTimestamp: now.Add(-tc.timeSinceLast),
				Interval:      tc.period,
			}, 1*time.Hour)

			var capturedLength int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedLength = data[0].GetMetricDataProcessingParams.Length
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			config := CachingProcessorConfig{
				MinPeriods: 1,
				MaxPeriods: 5,
			}
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

			requests := []*model.CloudwatchData{
				{
					MetricName:   "TestMetric",
					ResourceName: "test-resource",
					Namespace:    "AWS/Test",
					Dimensions:   []model.Dimension{{Name: "Dim", Value: "val"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    tc.period,
						Length:    tc.period,
						Delay:     0,
						Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)

			maxLengthSeconds := tc.period * effectiveMaxPeriods(tc.period)
			if capturedLength > maxLengthSeconds {
				t.Errorf("Length %d exceeds effectiveMaxPeriods cap %d", capturedLength, maxLengthSeconds)
			}

			t.Logf("%s: timeSinceLast=%v, period=%d, effectiveMaxPeriods=%d, capturedLength=%d",
				tc.description, tc.timeSinceLast, tc.period, effectiveMaxPeriods(tc.period), capturedLength)
		})
	}
}

// =============================================================================
// effectiveDelay Tests
// =============================================================================

func TestEffectiveDelay(t *testing.T) {
	assert.Equal(t, int64(120), effectiveDelay(60), "1 min period → 120s")
	assert.Equal(t, int64(120), effectiveDelay(120), "2 min period → 120s")
	assert.Equal(t, int64(120), effectiveDelay(180), "3 min period → 120s")
	assert.Equal(t, int64(120), effectiveDelay(240), "4 min period → 120s")
	assert.Equal(t, int64(300), effectiveDelay(300), "5 min period → 300s")
	assert.Equal(t, int64(300), effectiveDelay(600), "10 min period → 300s")
	assert.Equal(t, int64(300), effectiveDelay(900), "15 min period → 300s")
	assert.Equal(t, int64(300), effectiveDelay(3600), "1 hour period → 300s")
}

func TestCachingProcessor_DelayApplied_SteadyState(t *testing.T) {
	// Verify the correct delay is set on requests during steady-state for each period tier
	testCases := []struct {
		name          string
		period        int64
		expectedDelay int64
	}{
		{"60s period → 120s delay", 60, 120},
		{"180s period → 120s delay", 180, 120},
		{"240s period → 120s delay", 240, 120},
		{"300s period → 300s delay", 300, 300},
		{"3600s period → 300s delay", 3600, 300},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			cache := NewTimeseriesCache()
			defer cache.Stop()

			cacheKey := BuildCacheKey("AWS/Test", "M", []model.Dimension{{Name: "D", Value: "v"}}, "Average")
			cache.Set(cacheKey, TimeseriesCacheEntry{
				LastTimestamp: now.Add(-time.Duration(tc.period) * time.Second),
				Interval:      tc.period,
			}, 1*time.Hour)

			var capturedDelay int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedDelay = data[0].GetMetricDataProcessingParams.Delay
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(1), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

			requests := []*model.CloudwatchData{
				{
					MetricName: "M", ResourceName: "r", Namespace: "AWS/Test",
					Dimensions: []model.Dimension{{Name: "D", Value: "v"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period: tc.period, Length: tc.period, Delay: 0, Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDelay, capturedDelay)
		})
	}
}

func TestCachingProcessor_DelayApplied_ColdStart(t *testing.T) {
	// Cold start (no cache entry) should also use the tiered delay
	testCases := []struct {
		name          string
		period        int64
		expectedDelay int64
	}{
		{"60s cold start → 120s delay", 60, 120},
		{"300s cold start → 300s delay", 300, 300},
		{"3600s cold start → 300s delay", 3600, 300},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			cache := NewTimeseriesCache()
			defer cache.Stop()

			var capturedDelay int64
			client := testClient{
				GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
					if len(data) > 0 && data[0].GetMetricDataProcessingParams != nil {
						capturedDelay = data[0].GetMetricDataProcessingParams.Delay
					}
					results := make([]cloudwatch.MetricDataResult, 0, len(data))
					for _, d := range data {
						results = append(results, cloudwatch.MetricDataResult{
							ID:         d.GetMetricDataProcessingParams.QueryID,
							DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(1), Timestamp: now}},
						})
					}
					return results
				},
			}

			inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
			cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

			requests := []*model.CloudwatchData{
				{
					MetricName: "M", ResourceName: "r", Namespace: "AWS/Test",
					Dimensions: []model.Dimension{{Name: "D", Value: "v"}},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period: tc.period, Length: tc.period, Delay: 0, Statistic: "Average",
					},
				},
			}

			_, err := cp.Run(context.Background(), "AWS/Test", requests)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedDelay, capturedDelay)
		})
	}
}

func TestEffectiveMaxPeriods(t *testing.T) {
	assert.Equal(t, int64(10), effectiveMaxPeriods(60), "1 min period → 10")
	assert.Equal(t, int64(10), effectiveMaxPeriods(120), "2 min period → 10")
	assert.Equal(t, int64(10), effectiveMaxPeriods(180), "3 min period → 10")
	assert.Equal(t, int64(5), effectiveMaxPeriods(240), "4 min period → 5")
	assert.Equal(t, int64(5), effectiveMaxPeriods(300), "5 min period → 5")
	assert.Equal(t, int64(5), effectiveMaxPeriods(600), "10 min period → 5")
	assert.Equal(t, int64(2), effectiveMaxPeriods(900), "15 min period → 2")
	assert.Equal(t, int64(2), effectiveMaxPeriods(3600), "1 hour period → 2")
}

func TestCacheTTL(t *testing.T) {
	testCases := []struct {
		name     string
		period   int64
		expected time.Duration
	}{
		{"1 min period", 60, 11 * time.Minute},                   // (10+1)*60 = 660s
		{"5 min period", 300, 30 * time.Minute},                  // (5+1)*300 = 1800s
		{"10 min period", 600, 60 * time.Minute},                 // (5+1)*600 = 3600s
		{"1 hour period", 3600, 3 * time.Hour},                   // (2+1)*3600 = 10800s
		{"5 hour period (>4h override)", 18000, 10 * time.Hour},  // 2*18000 = 36000s
		{"24 hour period (>4h override)", 86400, 48 * time.Hour}, // 2*86400 = 172800s
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, CacheTTL(tc.period))
		})
	}
}

// =============================================================================
// Gap Fill NaN Tests
// =============================================================================

func TestCachingProcessor_GapFill_NoNaNWhenFlagOff(t *testing.T) {
	// With GapValue=nil (default), no NaN points should be emitted even with gaps
	now := time.Now().Truncate(time.Minute)
	lastSeen := now.Add(-4 * time.Minute)
	returnedTS := now

	cache := NewTimeseriesCache()
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastSeen,
		Interval:      60,
	}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: returnedTS}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    300,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Only 1 real data point, no NaN (flag is off)
	dps := results[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps, 1, "expected only 1 real data point, no NaN")
	assert.False(t, math.IsNaN(*dps[0].Value), "should be real data")
}

func TestCachingProcessor_GapFill_NoDataReturned_EmitsNaN(t *testing.T) {
	// With GapValue set: CW returns 0 data points → single NaN at lastTimestamp + period
	now := time.Now().Truncate(time.Minute)
	lastSeen := now.Add(-3 * time.Minute) // 10:00

	cache := NewTimeseriesCache()
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastSeen,
		Interval:      60,
	}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{}, // no data — series went silent
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    300,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Exactly 1 NaN at lastSeen + 1 period to terminate the series
	dps := results[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps, 1, "should have exactly 1 NaN point")
	require.NotNil(t, dps[0].Value)
	assert.True(t, math.IsNaN(*dps[0].Value), "point should be NaN")

	expectedTS := lastSeen.Add(time.Minute).Truncate(time.Minute)
	assert.Equal(t, expectedTS, dps[0].Timestamp, "NaN should be at lastTimestamp + period")

	// Cache should NOT advance — only NaN was produced, no real data
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, lastSeen, entry.LastTimestamp, "cache must not advance past NaN")
}

func TestCachingProcessor_GapFill_NoNaNWhenRealDataReturned(t *testing.T) {
	// With GapValue set: CW returns real data → no NaN emitted (series is alive)
	now := time.Now().Truncate(time.Minute)
	lastSeen := now.Add(-30 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastSeen,
		Interval:      60,
	}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    600,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	dps := results[0].GetMetricDataResult.DataPoints
	// Only 1 real point, no NaN (CW returned data, series is alive)
	assert.Len(t, dps, 1, "should have only 1 real data point")
	assert.False(t, math.IsNaN(*dps[0].Value), "point should be real data")

	// Cache should advance to the real point
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, now, entry.LastTimestamp)
}

func TestCachingProcessor_GapFill_NoCacheEntry(t *testing.T) {
	// Cold start — no cache entry → no NaN even with GapValue set
	now := time.Now()
	cache := NewTimeseriesCache()
	defer cache.Stop()

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(42), Timestamp: now}},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    60,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// 1 real data point, no NaN (no cache entry → cold start, can't determine gap)
	dps := results[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps, 1, "only 1 real data point")
	assert.False(t, math.IsNaN(*dps[0].Value), "should be real data")
}

func TestCachingProcessor_GapFill_PartialData_NoNaN(t *testing.T) {
	// Gap of 3 periods, CW returns some points → no NaN (series is alive)
	now := time.Now().Truncate(time.Minute)
	lastSeen := now.Add(-4 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", []model.Dimension{{Name: "InstanceId", Value: "i-123"}}, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{
		LastTimestamp: lastSeen,
		Interval:      60,
	}, 1*time.Hour)

	// CW returns data at T+2min and T+4min, missing T+1min and T+3min
	t2 := lastSeen.Add(2 * time.Minute)
	t4 := lastSeen.Add(4 * time.Minute)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(50), Timestamp: t2},
						{Value: aws.Float64(70), Timestamp: t4},
					},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName:   "CPUUtilization",
			ResourceName: "i-123",
			Namespace:    "AWS/EC2",
			Dimensions:   []model.Dimension{{Name: "InstanceId", Value: "i-123"}},
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period:    60,
				Length:    300,
				Delay:     0,
				Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	dps := results[0].GetMetricDataResult.DataPoints

	// Only 2 real points, no NaN (CW returned data, series is alive)
	assert.Len(t, dps, 2, "should have 2 real data points only")
	for _, dp := range dps {
		assert.False(t, math.IsNaN(*dp.Value), "all points should be real data")
	}

	// Cache should advance to T+4
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t4, entry.LastTimestamp)
}

// TestCachingProcessor_GapFill_MultiScrapeLifecycle simulates the full lifecycle:
//
//	Scrape 1: CW returns real data at T1, T2           → cache advances to T2
//	Scrape 2: CW returns nothing (gap)                 → single NaN at T3; cache stays at T2
//	Scrape 3: CW returns real data at T3, T4, T5, T6   → T3 is new; cache advances to T6
func TestCachingProcessor_GapFill_MultiScrapeLifecycle(t *testing.T) {
	t0 := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(1 * time.Minute)
	t2 := t0.Add(2 * time.Minute)
	t3 := t0.Add(3 * time.Minute)
	t4 := t0.Add(4 * time.Minute)
	t5 := t0.Add(5 * time.Minute)
	t6 := t0.Add(6 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	dims := []model.Dimension{{Name: "InstanceId", Value: "i-456"}}
	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", dims, "Average")
	nanOnGapConfig := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}

	makeRequests := func() []*model.CloudwatchData {
		return []*model.CloudwatchData{
			{
				MetricName:   "CPUUtilization",
				ResourceName: "i-456",
				Namespace:    "AWS/EC2",
				Dimensions:   dims,
				GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
					Period:    60,
					Length:    600,
					Delay:     0,
					Statistic: "Average",
				},
			},
		}
	}

	// =========================================================================
	// Scrape 1: CW returns real data at T1 and T2
	// =========================================================================
	client1 := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(10), Timestamp: t1},
						{Value: aws.Float64(20), Timestamp: t2},
					},
				})
			}
			return results
		},
	}

	inner1 := NewDefaultProcessor(promslog.NewNopLogger(), client1, 500, 1)
	cp1 := NewCachingProcessor(promslog.NewNopLogger(), inner1, cache, nanOnGapConfig)

	results1, err := cp1.Run(context.Background(), "AWS/EC2", makeRequests())
	require.NoError(t, err)
	require.Len(t, results1, 1)

	// 2 real points, no NaN (cold start, CW returned data)
	dps1 := results1[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps1, 2, "scrape 1: 2 real points, no NaN")

	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t2, entry.LastTimestamp, "scrape 1: cache should be at T2")

	// =========================================================================
	// Scrape 2: CW returns nothing (gap at T3, T4, T5)
	// =========================================================================
	client2 := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{}, // no data
				})
			}
			return results
		},
	}

	inner2 := NewDefaultProcessor(promslog.NewNopLogger(), client2, 500, 1)
	cp2 := NewCachingProcessor(promslog.NewNopLogger(), inner2, cache, nanOnGapConfig)

	results2, err := cp2.Run(context.Background(), "AWS/EC2", makeRequests())
	require.NoError(t, err)
	require.Len(t, results2, 1)

	dps2 := results2[0].GetMetricDataResult.DataPoints
	// Single NaN at T3 (T2 + period) to terminate the series
	assert.Len(t, dps2, 1, "scrape 2: single NaN to terminate series")
	require.NotNil(t, dps2[0].Value)
	assert.True(t, math.IsNaN(*dps2[0].Value), "scrape 2: should be NaN")
	assert.Equal(t, t3, dps2[0].Timestamp, "scrape 2: NaN at T3 (T2 + period)")

	// Cache should stay at T2 (no real data)
	entry, ok = cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t2, entry.LastTimestamp, "scrape 2: cache should stay at T2")

	// =========================================================================
	// Scrape 3: CW returns T3, T4, T5, T6 (backfill + new data)
	// =========================================================================
	client3 := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(30), Timestamp: t3},
						{Value: aws.Float64(40), Timestamp: t4},
						{Value: aws.Float64(50), Timestamp: t5},
						{Value: aws.Float64(60), Timestamp: t6},
					},
				})
			}
			return results
		},
	}

	inner3 := NewDefaultProcessor(promslog.NewNopLogger(), client3, 500, 1)
	cp3 := NewCachingProcessor(promslog.NewNopLogger(), inner3, cache, nanOnGapConfig)

	results3, err := cp3.Run(context.Background(), "AWS/EC2", makeRequests())
	require.NoError(t, err)
	require.Len(t, results3, 1)

	dps3 := results3[0].GetMetricDataResult.DataPoints

	realValues3 := map[time.Time]float64{}
	for _, dp := range dps3 {
		if dp.Value != nil && !math.IsNaN(*dp.Value) {
			realValues3[dp.Timestamp] = *dp.Value
		}
	}

	// T3, T4, T5, T6 are all after cached T2 → all kept as real data, no NaN
	assert.Len(t, dps3, 4, "scrape 3: 4 real data points")
	assert.Equal(t, float64(30), realValues3[t3])
	assert.Equal(t, float64(40), realValues3[t4])
	assert.Equal(t, float64(50), realValues3[t5])
	assert.Equal(t, float64(60), realValues3[t6])

	entry, ok = cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t6, entry.LastTimestamp, "scrape 3: cache should advance to T6")
}

func TestCachingProcessor_NoNaN_WhenRealDataReturned(t *testing.T) {
	// Real data at T1, T2 → no NaN (even with GapValue set, CW returned data)
	t0 := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(1 * time.Minute)
	t2 := t0.Add(2 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	dims := []model.Dimension{{Name: "InstanceId", Value: "i-789"}}
	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", dims, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{LastTimestamp: t0, Interval: 60}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(10), Timestamp: t1},
						{Value: aws.Float64(20), Timestamp: t2},
					},
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName: "CPUUtilization", ResourceName: "i-789", Namespace: "AWS/EC2",
			Dimensions: dims,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period: 60, Length: 300, Delay: 0, Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	dps := results[0].GetMetricDataResult.DataPoints
	// 2 real points, no NaN (series is alive)
	assert.Len(t, dps, 2, "should have 2 real data points, no NaN")
	for _, dp := range dps {
		assert.False(t, math.IsNaN(*dp.Value), "should be real data")
	}

	// Cache should advance to T2
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t2, entry.LastTimestamp)
}

func TestCachingProcessor_NoNaN_WhenFlagOff_NoData(t *testing.T) {
	// CW returned nothing, but GapValue is nil → no NaN emitted
	t0 := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	dims := []model.Dimension{{Name: "InstanceId", Value: "i-789"}}
	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", dims, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{LastTimestamp: t0, Interval: 60}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID:         d.GetMetricDataProcessingParams.QueryID,
					DataPoints: []cloudwatch.DataPoint{}, // no data from CW
				})
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, DefaultCachingProcessorConfig())

	requests := []*model.CloudwatchData{
		{
			MetricName: "CPUUtilization", ResourceName: "i-789", Namespace: "AWS/EC2",
			Dimensions: dims,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period: 60, Length: 300, Delay: 0, Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// No NaN emitted (flag is off)
	dps := results[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps, 0, "no NaN should be emitted when flag is off")

	// Cache should NOT advance (no real data)
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t0, entry.LastTimestamp, "cache should stay at T0")
}

// TestCachingProcessor_RealWorldBatch_MixedDataAndSilent simulates the real-world scenario
// observed via MCP metrics: a single scrape contains a batch of metrics where some
// (e.g., CPUUtilization) return real data and others (e.g., CPUCreditBalance for aggregated
// dimensions) return 0 data points. Only the silent series should get a NaN termination
// point; series with data should remain clean.
func TestCachingProcessor_RealWorldBatch_MixedDataAndSilent(t *testing.T) {
	t0 := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(1 * time.Minute)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	// Both metrics have cache entries from a previous scrape at T0
	cpuDims := []model.Dimension{{Name: "InstanceId", Value: "i-123"}}
	creditDims := []model.Dimension{{Name: "AutoScalingGroupName", Value: "eks-infra-asg"}}
	cpuKey := BuildCacheKey("AWS/EC2", "CPUUtilization", cpuDims, "Average")
	creditKey := BuildCacheKey("AWS/EC2", "CPUCreditBalance", creditDims, "Average")
	cache.Set(cpuKey, TimeseriesCacheEntry{LastTimestamp: t0, Interval: 60}, 1*time.Hour)
	cache.Set(creditKey, TimeseriesCacheEntry{LastTimestamp: t0, Interval: 60}, 1*time.Hour)

	client := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				switch d.MetricName {
				case "CPUUtilization":
					// CW returns real data for CPUUtilization
					results = append(results, cloudwatch.MetricDataResult{
						ID:         d.GetMetricDataProcessingParams.QueryID,
						DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(19.7), Timestamp: t1}},
					})
				case "CPUCreditBalance":
					// CW returns 0 data points for CPUCreditBalance (series went silent)
					results = append(results, cloudwatch.MetricDataResult{
						ID:         d.GetMetricDataProcessingParams.QueryID,
						DataPoints: []cloudwatch.DataPoint{},
					})
				}
			}
			return results
		},
	}

	inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
	config := CachingProcessorConfig{
		MinPeriods: 1,
		MaxPeriods: 5,
		GapValue:   float64Ptr(math.NaN()),
	}
	cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

	requests := []*model.CloudwatchData{
		{
			MetricName: "CPUUtilization", ResourceName: "i-123", Namespace: "AWS/EC2",
			Dimensions: cpuDims,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period: 60, Length: 60, Delay: 0, Statistic: "Average",
			},
		},
		{
			MetricName: "CPUCreditBalance", ResourceName: "eks-infra-asg", Namespace: "AWS/EC2",
			Dimensions: creditDims,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period: 60, Length: 60, Delay: 0, Statistic: "Average",
			},
		},
	}

	results, err := cp.Run(context.Background(), "AWS/EC2", requests)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Find results by metric name
	var cpuResult, creditResult *model.CloudwatchData
	for _, r := range results {
		switch r.MetricName {
		case "CPUUtilization":
			cpuResult = r
		case "CPUCreditBalance":
			creditResult = r
		}
	}
	require.NotNil(t, cpuResult, "should have CPUUtilization result")
	require.NotNil(t, creditResult, "should have CPUCreditBalance result")

	// CPUUtilization: real data, no NaN
	cpuDps := cpuResult.GetMetricDataResult.DataPoints
	assert.Len(t, cpuDps, 1, "CPUUtilization: 1 real data point, no NaN")
	assert.Equal(t, 19.7, *cpuDps[0].Value)
	assert.Equal(t, t1, cpuDps[0].Timestamp)

	// CPUCreditBalance: 0 data → single NaN at T0 + 1 period
	creditDps := creditResult.GetMetricDataResult.DataPoints
	assert.Len(t, creditDps, 1, "CPUCreditBalance: 1 NaN termination point")
	require.NotNil(t, creditDps[0].Value)
	assert.True(t, math.IsNaN(*creditDps[0].Value), "CPUCreditBalance: should be NaN")
	expectedNaNTS := t0.Add(time.Minute).Truncate(time.Minute) // T0 + period, truncated
	assert.Equal(t, expectedNaNTS, creditDps[0].Timestamp,
		"NaN should be at cachedLastTimestamp + period")

	// Cache: CPUUtilization advances to T1, CPUCreditBalance stays at T0
	cpuEntry, ok := cache.Get(cpuKey)
	require.True(t, ok)
	assert.Equal(t, t1, cpuEntry.LastTimestamp, "CPUUtilization cache should advance to T1")

	creditEntry, ok := cache.Get(creditKey)
	require.True(t, ok)
	assert.Equal(t, t0, creditEntry.LastTimestamp, "CPUCreditBalance cache should stay at T0")
}

// TestCachingProcessor_NaN_Idempotent_SecondScrapeNoDoublNaN verifies that if a series
// stays silent across multiple scrapes, the NaN is emitted once (on the first gap scrape)
// and then subsequent scrapes still emit a NaN (at lastTS + period) without advancing the
// cache. This matches the real-world pattern where credit metrics stay silent for multiple
// consecutive scrapes.
func TestCachingProcessor_NaN_Idempotent_SecondScrapeNoDoubleNaN(t *testing.T) {
	t0 := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(1 * time.Minute) // expected NaN timestamp

	cache := NewTimeseriesCache()
	defer cache.Stop()

	dims := []model.Dimension{{Name: "AutoScalingGroupName", Value: "eks-infra-asg"}}
	cacheKey := BuildCacheKey("AWS/EC2", "CPUCreditBalance", dims, "Average")
	cache.Set(cacheKey, TimeseriesCacheEntry{LastTimestamp: t0, Interval: 60}, 1*time.Hour)

	emptyClient := testClient{
		GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
			results := make([]cloudwatch.MetricDataResult, 0, len(data))
			for _, d := range data {
				results = append(results, cloudwatch.MetricDataResult{
					ID: d.GetMetricDataProcessingParams.QueryID, DataPoints: []cloudwatch.DataPoint{},
				})
			}
			return results
		},
	}

	config := CachingProcessorConfig{MinPeriods: 1, MaxPeriods: 5, GapValue: float64Ptr(math.NaN())}

	makeRequests := func() []*model.CloudwatchData {
		return []*model.CloudwatchData{{
			MetricName: "CPUCreditBalance", ResourceName: "eks-infra-asg", Namespace: "AWS/EC2",
			Dimensions: dims,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period: 60, Length: 60, Delay: 0, Statistic: "Average",
			},
		}}
	}

	// Scrape 1: 0 data → NaN at T1
	inner1 := NewDefaultProcessor(promslog.NewNopLogger(), emptyClient, 500, 1)
	cp1 := NewCachingProcessor(promslog.NewNopLogger(), inner1, cache, config)
	results1, err := cp1.Run(context.Background(), "AWS/EC2", makeRequests())
	require.NoError(t, err)
	require.Len(t, results1, 1)

	dps1 := results1[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps1, 1, "scrape 1: single NaN")
	assert.True(t, math.IsNaN(*dps1[0].Value))
	assert.Equal(t, t1, dps1[0].Timestamp, "scrape 1: NaN at T0+period")

	// Cache stays at T0 (NaN doesn't advance it)
	entry, ok := cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t0, entry.LastTimestamp)

	// Scrape 2: still 0 data → NaN at T1 again (same timestamp, cache didn't advance)
	inner2 := NewDefaultProcessor(promslog.NewNopLogger(), emptyClient, 500, 1)
	cp2 := NewCachingProcessor(promslog.NewNopLogger(), inner2, cache, config)
	results2, err := cp2.Run(context.Background(), "AWS/EC2", makeRequests())
	require.NoError(t, err)
	require.Len(t, results2, 1)

	dps2 := results2[0].GetMetricDataResult.DataPoints
	assert.Len(t, dps2, 1, "scrape 2: still single NaN")
	assert.True(t, math.IsNaN(*dps2[0].Value))
	assert.Equal(t, t1, dps2[0].Timestamp, "scrape 2: NaN at T0+period (unchanged)")

	// Cache still at T0
	entry, ok = cache.Get(cacheKey)
	require.True(t, ok)
	assert.Equal(t, t0, entry.LastTimestamp, "cache should not advance from NaN")
}

// TestCachingProcessor_SteadyState_NoNaN verifies that during normal steady-state operation
// (no gap, CW returns data every scrape) no NaN is ever appended — even with GapValue
// set. Simulates 3 consecutive scrapes where CW always returns data.
func TestCachingProcessor_SteadyState_NoNaN(t *testing.T) {
	t0 := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)

	cache := NewTimeseriesCache()
	defer cache.Stop()

	dims := []model.Dimension{{Name: "InstanceId", Value: "i-123"}}
	cacheKey := BuildCacheKey("AWS/EC2", "CPUUtilization", dims, "Average")
	config := CachingProcessorConfig{MinPeriods: 1, MaxPeriods: 5, GapValue: float64Ptr(math.NaN())}

	for scrape := 0; scrape < 3; scrape++ {
		ts := t0.Add(time.Duration(scrape+1) * time.Minute) // T1, T2, T3

		client := testClient{
			GetMetricDataFunc: func(_ context.Context, data []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch.MetricDataResult {
				results := make([]cloudwatch.MetricDataResult, 0, len(data))
				for _, d := range data {
					results = append(results, cloudwatch.MetricDataResult{
						ID:         d.GetMetricDataProcessingParams.QueryID,
						DataPoints: []cloudwatch.DataPoint{{Value: aws.Float64(20.0 + float64(scrape)), Timestamp: ts}},
					})
				}
				return results
			},
		}

		inner := NewDefaultProcessor(promslog.NewNopLogger(), client, 500, 1)
		cp := NewCachingProcessor(promslog.NewNopLogger(), inner, cache, config)

		requests := []*model.CloudwatchData{{
			MetricName: "CPUUtilization", ResourceName: "i-123", Namespace: "AWS/EC2",
			Dimensions: dims,
			GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
				Period: 60, Length: 60, Delay: 0, Statistic: "Average",
			},
		}}

		results, err := cp.Run(context.Background(), "AWS/EC2", requests)
		require.NoError(t, err)
		require.Len(t, results, 1)

		dps := results[0].GetMetricDataResult.DataPoints

		// Every data point must be real — zero NaN in any scrape
		for _, dp := range dps {
			require.NotNil(t, dp.Value, "scrape %d: value must not be nil", scrape)
			assert.False(t, math.IsNaN(*dp.Value),
				"scrape %d: no NaN should appear during steady-state", scrape)
		}

		// Cache should advance to the latest real point
		entry, ok := cache.Get(cacheKey)
		require.True(t, ok)
		assert.Equal(t, ts, entry.LastTimestamp,
			"scrape %d: cache should advance to latest real point", scrape)
	}
}
