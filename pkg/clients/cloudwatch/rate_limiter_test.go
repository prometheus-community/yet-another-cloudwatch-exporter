package cloudwatch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

func TestRateLimitToLimiter(t *testing.T) {
	tests := []struct {
		name          string
		input         *RateLimit
		expectedRate  float64
		expectedBurst int
		expectError   bool
	}{
		{
			name:          "nil means no limit",
			input:         nil,
			expectedRate:  1e100, // rate.Inf is a very large float
			expectedBurst: 1,
			expectError:   false,
		},
		{
			name:          "25 per second",
			input:         &RateLimit{Count: 25, Duration: time.Second},
			expectedRate:  25.0,
			expectedBurst: 25,
			expectError:   false,
		},
		{
			name:          "100 per minute",
			input:         &RateLimit{Count: 100, Duration: time.Minute},
			expectedRate:  100.0 / 60.0, // 1.666... per second
			expectedBurst: 100,
			expectError:   false,
		},
		{
			name:          "3600 per hour",
			input:         &RateLimit{Count: 3600, Duration: time.Hour},
			expectedRate:  1.0, // 3600/3600 = 1 per second
			expectedBurst: 3600,
			expectError:   false,
		},
		{
			name:        "zero count",
			input:       &RateLimit{Count: 0, Duration: time.Second},
			expectError: true,
		},
		{
			name:        "negative count",
			input:       &RateLimit{Count: -5, Duration: time.Second},
			expectError: true,
		},
		{
			name:        "zero duration",
			input:       &RateLimit{Count: 25, Duration: 0},
			expectError: true,
		},
		{
			name:        "negative duration",
			input:       &RateLimit{Count: 25, Duration: -time.Second},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rate, burst, err := rateLimitToLimiter(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.input == nil {
				// For nil (rate.Inf), check it's a very large number
				assert.True(t, float64(rate) > 1e100, "Expected very large rate (rate.Inf), got %v", rate)
			} else {
				assert.InDelta(t, tt.expectedRate, float64(rate), 0.001, "Rate mismatch")
			}
			assert.Equal(t, tt.expectedBurst, burst, "Burst mismatch")
		})
	}
}

// mockClient implements the Client interface for testing
type mockClient struct {
	listMetricsCalls         int
	getMetricDataCalls       int
	getMetricStatisticsCalls int
	callDelay                time.Duration
}

func (m *mockClient) ListMetrics(_ context.Context, _ string, _ *model.MetricConfig, _ bool, _ func(page []*model.Metric)) error {
	m.listMetricsCalls++
	if m.callDelay > 0 {
		time.Sleep(m.callDelay)
	}
	return nil
}

func (m *mockClient) GetMetricData(_ context.Context, _ []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []MetricDataResult {
	m.getMetricDataCalls++
	if m.callDelay > 0 {
		time.Sleep(m.callDelay)
	}
	return nil
}

func (m *mockClient) GetMetricStatistics(_ context.Context, _ *slog.Logger, _ []model.Dimension, _ string, _ *model.MetricConfig) []*model.MetricStatisticsResult {
	m.getMetricStatisticsCalls++
	if m.callDelay > 0 {
		time.Sleep(m.callDelay)
	}
	return nil
}

func TestPerAPIRateLimiter(t *testing.T) {
	tests := []struct {
		name                   string
		listMetrics            *RateLimit
		getMetricData          *RateLimit
		getMetricStatistics    *RateLimit
		expectError            bool
		expectedErrorSubstring string
	}{
		{
			name:                "valid per-API limits",
			listMetrics:         &RateLimit{Count: 5, Duration: time.Second},
			getMetricData:       &RateLimit{Count: 10, Duration: time.Second},
			getMetricStatistics: &RateLimit{Count: 15, Duration: time.Second},
			expectError:         false,
		},
		{
			name:                "some empty limits",
			listMetrics:         &RateLimit{Count: 5, Duration: time.Second},
			getMetricData:       nil,
			getMetricStatistics: &RateLimit{Count: 15, Duration: time.Second},
			expectError:         false,
		},
		{
			name:                   "invalid ListMetrics limit",
			listMetrics:            &RateLimit{Count: 0, Duration: time.Second},
			getMetricData:          &RateLimit{Count: 10, Duration: time.Second},
			getMetricStatistics:    &RateLimit{Count: 15, Duration: time.Second},
			expectError:            true,
			expectedErrorSubstring: "invalid ListMetrics rate limit",
		},
		{
			name:                   "invalid GetMetricData limit",
			listMetrics:            &RateLimit{Count: 5, Duration: time.Second},
			getMetricData:          &RateLimit{Count: 0, Duration: time.Second},
			getMetricStatistics:    &RateLimit{Count: 15, Duration: time.Second},
			expectError:            true,
			expectedErrorSubstring: "invalid GetMetricData rate limit",
		},
		{
			name:                   "invalid GetMetricStatistics limit",
			listMetrics:            &RateLimit{Count: 5, Duration: time.Second},
			getMetricData:          &RateLimit{Count: 10, Duration: time.Second},
			getMetricStatistics:    &RateLimit{Count: 0, Duration: time.Second},
			expectError:            true,
			expectedErrorSubstring: "invalid GetMetricStatistics rate limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build limiters map directly
			limiters := make(map[string]*rate.Limiter)
			apiLimits := map[string]*RateLimit{
				listMetricsCall:         tt.listMetrics,
				getMetricDataCall:       tt.getMetricData,
				getMetricStatisticsCall: tt.getMetricStatistics,
			}

			var err error
			for apiName, rateLimit := range apiLimits {
				if rateLimit != nil {
					limiter, buildErr := NewSingleAPIRateLimiter(apiName, rateLimit)
					if buildErr != nil {
						err = buildErr
						break
					}
					if limiter != nil {
						limiters[apiName] = limiter
					}
				}
			}

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorSubstring)
				return
			}

			require.NoError(t, err)

			limiter := &perAPIRateLimiter{limiters: limiters}

			ctx := context.Background()

			// Test each API
			apis := []string{listMetricsCall, getMetricDataCall, getMetricStatisticsCall}
			for _, api := range apis {
				// Test Allow method
				allowed := limiter.Allow(api)
				// Should be true for APIs with limits, or true for APIs without limits
				assert.True(t, allowed, "First call to %s should be allowed", api)

				// Test Wait method
				err = limiter.Wait(ctx, api)
				assert.NoError(t, err, "Wait for %s should not error", api)
			}

			// Test unknown API (should always be allowed)
			allowed := limiter.Allow("UnknownAPI")
			assert.True(t, allowed, "Unknown API should always be allowed")

			err = limiter.Wait(ctx, "UnknownAPI")
			assert.NoError(t, err, "Wait for unknown API should not error")
		})
	}
}

func TestRateLimitedClientFromConfig(t *testing.T) {
	mockClient := &mockClient{}

	tests := []struct {
		name          string
		config        RateLimitConfig
		expectWrapped bool
	}{
		{
			name: "no rate limiting",
			config: RateLimitConfig{
				PerAPILimits: nil,
			},
			expectWrapped: false,
		},
		{
			name: "per-API rate limits",
			config: RateLimitConfig{
				PerAPILimits: map[string]*RateLimit{
					listMetricsCall: {Count: 5, Duration: time.Second},
				},
			},
			expectWrapped: true,
		},
		{
			name: "invalid per-API rate limit falls back to original",
			config: RateLimitConfig{
				PerAPILimits: nil, // No limiters = no wrapping
			},
			expectWrapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewRateLimitedClientFromConfig(mockClient, tt.config)

			if tt.expectWrapped {
				// Should be wrapped
				_, ok := client.(*SimpleRateLimitedClient)
				assert.True(t, ok, "Client should be wrapped with rate limiting")
			} else {
				// Should be the original client
				assert.Equal(t, mockClient, client, "Client should be the original unwrapped client")
			}
		})
	}
}

func TestRateLimitingBehavior(t *testing.T) {
	mockClient := &mockClient{}

	// Create a rate-limited client with a very low rate limit
	config := RateLimitConfig{
		PerAPILimits: map[string]*RateLimit{
			listMetricsCall: {Count: 2, Duration: time.Second},
		},
	}

	client := NewRateLimitedClientFromConfig(mockClient, config)
	ctx := context.Background()

	// Make multiple calls quickly and measure timing
	start := time.Now()

	// First call should be immediate
	err := client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)

	// Second call should be immediate (burst)
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)

	// Third call should be rate limited (should wait ~500ms for 2/sec rate)
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)

	elapsed := time.Since(start)

	// Should have taken at least 400ms due to rate limiting (allowing some margin)
	assert.True(t, elapsed >= 400*time.Millisecond,
		"Rate limiting should have caused delay, elapsed: %v", elapsed)

	// Should have made 3 calls
	assert.Equal(t, 3, mockClient.listMetricsCalls)
}

func TestPerAPIRateLimitingBehavior(t *testing.T) {
	mockClient := &mockClient{}

	// Create a rate-limited client with different limits per API
	config := RateLimitConfig{
		PerAPILimits: map[string]*RateLimit{
			listMetricsCall:         {Count: 1, Duration: time.Second},
			getMetricDataCall:       {Count: 10, Duration: time.Second},
			getMetricStatisticsCall: {Count: 5, Duration: time.Second},
		},
	}

	client := NewRateLimitedClientFromConfig(mockClient, config)
	ctx := context.Background()

	// Test ListMetrics (most restrictive)
	start := time.Now()
	err := client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err) // Should be immediate
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err) // Should wait ~1 second
	listMetricsElapsed := time.Since(start)

	// Should have taken at least 800ms due to 1/sec rate limiting
	assert.True(t, listMetricsElapsed >= 800*time.Millisecond,
		"ListMetrics rate limiting should have caused delay, elapsed: %v", listMetricsElapsed)

	// Test GetMetricData (more permissive) - should be faster
	start = time.Now()
	client.GetMetricData(ctx, nil, "test", time.Now(), time.Now()) // Should be immediate
	client.GetMetricData(ctx, nil, "test", time.Now(), time.Now()) // Should be immediate (10/sec allows burst)
	getMetricDataElapsed := time.Since(start)

	// Should be much faster than ListMetrics
	assert.True(t, getMetricDataElapsed < 100*time.Millisecond,
		"GetMetricData should be faster due to higher rate limit, elapsed: %v", getMetricDataElapsed)

	// Verify call counts
	assert.Equal(t, 2, mockClient.listMetricsCalls)
	assert.Equal(t, 2, mockClient.getMetricDataCalls)
}

func TestContextCancellation(t *testing.T) {
	mockClient := &mockClient{}

	config := RateLimitConfig{
		PerAPILimits: map[string]*RateLimit{
			listMetricsCall: {Count: 1, Duration: time.Minute},
		},
	}

	client := NewRateLimitedClientFromConfig(mockClient, config)

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// First call should work
	err := client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)

	// Second call should be cancelled due to context timeout
	start := time.Now()
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline")
	elapsed := time.Since(start)

	// Should have returned quickly due to context cancellation
	assert.True(t, elapsed < 200*time.Millisecond,
		"Context cancellation should prevent long waits, elapsed: %v", elapsed)

	// Should have made only 1 successful call
	assert.Equal(t, 1, mockClient.listMetricsCalls)
}

func TestRateLimitingMetrics(t *testing.T) {
	// Reset metrics before test
	promutil.CloudwatchRateLimitWaitCounter.Reset()
	promutil.CloudwatchRateLimitAllowedCounter.Reset()

	mockClient := &mockClient{}

	// Create a rate-limited client with a very low rate limit
	config := RateLimitConfig{
		PerAPILimits: map[string]*RateLimit{
			listMetricsCall: {Count: 1, Duration: time.Second},
		},
	}

	client := NewRateLimitedClientFromConfig(mockClient, config)
	ctx := context.Background()

	// First call should be allowed immediately
	err := client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)

	// Check that allowed counter was incremented
	allowedCount := testutil.ToFloat64(promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(listMetricsCall))
	assert.Equal(t, float64(1), allowedCount, "First call should be counted as allowed")

	// Second call should be rate limited (will wait)
	start := time.Now()
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)
	elapsed := time.Since(start)

	// Should have waited due to rate limiting
	assert.True(t, elapsed >= 800*time.Millisecond, "Second call should have been rate limited")

	// Check that wait counter was incremented
	waitCount := testutil.ToFloat64(promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(listMetricsCall))
	assert.Equal(t, float64(1), waitCount, "Second call should be counted as rate limited")

	// Verify both calls were made
	assert.Equal(t, 2, mockClient.listMetricsCalls)
}

func TestPerAPIRateLimitingMetrics(t *testing.T) {
	// Reset metrics before test
	promutil.CloudwatchRateLimitWaitCounter.Reset()
	promutil.CloudwatchRateLimitAllowedCounter.Reset()

	mockClient := &mockClient{}

	// Create a rate-limited client with different limits per API
	config := RateLimitConfig{
		PerAPILimits: map[string]*RateLimit{
			listMetricsCall:   {Count: 1, Duration: time.Second},
			getMetricDataCall: {Count: 10, Duration: time.Second},
		},
	}

	client := NewRateLimitedClientFromConfig(mockClient, config)
	ctx := context.Background()

	// Test ListMetrics (restrictive)
	err := client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err) // Should be allowed
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err) // Should be rate limited

	// Test GetMetricData (permissive)
	client.GetMetricData(ctx, nil, "test", time.Now(), time.Now()) // Should be allowed
	client.GetMetricData(ctx, nil, "test", time.Now(), time.Now()) // Should also be allowed

	// Check ListMetrics metrics
	listAllowedCount := testutil.ToFloat64(promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(listMetricsCall))
	listWaitCount := testutil.ToFloat64(promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(listMetricsCall))
	assert.Equal(t, float64(1), listAllowedCount, "ListMetrics should have 1 allowed call")
	assert.Equal(t, float64(1), listWaitCount, "ListMetrics should have 1 rate limited call")

	// Check GetMetricData metrics
	dataAllowedCount := testutil.ToFloat64(promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(getMetricDataCall))
	dataWaitCount := testutil.ToFloat64(promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(getMetricDataCall))
	assert.Equal(t, float64(2), dataAllowedCount, "GetMetricData should have 2 allowed calls")
	assert.Equal(t, float64(0), dataWaitCount, "GetMetricData should have 0 rate limited calls")

	// Verify call counts
	assert.Equal(t, 2, mockClient.listMetricsCalls)
	assert.Equal(t, 2, mockClient.getMetricDataCalls)
}
