package cloudwatch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

func TestCreateLimiter(t *testing.T) {
	tests := []struct {
		name          string
		input         *APIRateLimit
		expectedRate  float64
		expectedBurst int
		expectError   bool
	}{
		{
			name:          "25 per second",
			input:         &APIRateLimit{Count: 25, Duration: time.Second},
			expectedRate:  25.0,
			expectedBurst: 25,
			expectError:   false,
		},
		{
			name:          "100 per minute",
			input:         &APIRateLimit{Count: 100, Duration: time.Minute},
			expectedRate:  100.0 / 60.0,
			expectedBurst: 100,
			expectError:   false,
		},
		{
			name:          "3600 per hour",
			input:         &APIRateLimit{Count: 3600, Duration: time.Hour},
			expectedRate:  1.0,
			expectedBurst: 3600,
			expectError:   false,
		},
		{
			name:        "zero count",
			input:       &APIRateLimit{Count: 0, Duration: time.Second},
			expectError: true,
		},
		{
			name:        "negative count",
			input:       &APIRateLimit{Count: -5, Duration: time.Second},
			expectError: true,
		},
		{
			name:        "zero duration",
			input:       &APIRateLimit{Count: 25, Duration: 0},
			expectError: true,
		},
		{
			name:        "negative duration",
			input:       &APIRateLimit{Count: 25, Duration: -time.Second},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := createLimiter(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.InDelta(t, tt.expectedRate, float64(limiter.Limit()), 0.001, "Rate mismatch")
			assert.Equal(t, tt.expectedBurst, limiter.Burst(), "Burst mismatch")
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

func TestNewGlobalRateLimiter(t *testing.T) {
	tests := []struct {
		name                   string
		config                 RateLimiterConfig
		expectError            bool
		expectedErrorSubstring string
	}{
		{
			name: "valid per-API limits",
			config: RateLimiterConfig{
				ListMetrics:         &APIRateLimit{Count: 5, Duration: time.Second},
				GetMetricData:       &APIRateLimit{Count: 10, Duration: time.Second},
				GetMetricStatistics: &APIRateLimit{Count: 15, Duration: time.Second},
			},
			expectError: false,
		},
		{
			name: "some nil limits",
			config: RateLimiterConfig{
				ListMetrics:         &APIRateLimit{Count: 5, Duration: time.Second},
				GetMetricData:       nil,
				GetMetricStatistics: &APIRateLimit{Count: 15, Duration: time.Second},
			},
			expectError: false,
		},
		{
			name: "invalid ListMetrics limit",
			config: RateLimiterConfig{
				ListMetrics:         &APIRateLimit{Count: 0, Duration: time.Second},
				GetMetricData:       &APIRateLimit{Count: 10, Duration: time.Second},
				GetMetricStatistics: &APIRateLimit{Count: 15, Duration: time.Second},
			},
			expectError:            true,
			expectedErrorSubstring: "invalid ListMetrics rate limit",
		},
		{
			name: "invalid GetMetricData limit",
			config: RateLimiterConfig{
				ListMetrics:         &APIRateLimit{Count: 5, Duration: time.Second},
				GetMetricData:       &APIRateLimit{Count: 0, Duration: time.Second},
				GetMetricStatistics: &APIRateLimit{Count: 15, Duration: time.Second},
			},
			expectError:            true,
			expectedErrorSubstring: "invalid GetMetricData rate limit",
		},
		{
			name: "invalid GetMetricStatistics limit",
			config: RateLimiterConfig{
				ListMetrics:         &APIRateLimit{Count: 5, Duration: time.Second},
				GetMetricData:       &APIRateLimit{Count: 10, Duration: time.Second},
				GetMetricStatistics: &APIRateLimit{Count: 0, Duration: time.Second},
			},
			expectError:            true,
			expectedErrorSubstring: "invalid GetMetricStatistics rate limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter, err := NewGlobalRateLimiter(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorSubstring)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, limiter)
		})
	}
}

func TestNewRateLimitedClient(t *testing.T) {
	mockClient := &mockClient{}

	tests := []struct {
		name          string
		config        RateLimiterConfig
		expectWrapped bool
	}{
		{
			name:          "no rate limiting",
			config:        RateLimiterConfig{},
			expectWrapped: false,
		},
		{
			name: "per-API rate limits",
			config: RateLimiterConfig{
				ListMetrics: &APIRateLimit{Count: 5, Duration: time.Second},
			},
			expectWrapped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client Client
			if tt.expectWrapped {
				limiter, err := NewGlobalRateLimiter(tt.config)
				require.NoError(t, err)
				client = NewRateLimitedClient(mockClient, limiter, "us-east-1", "111111111111", "test-role")
			} else {
				client = NewRateLimitedClient(mockClient, nil, "us-east-1", "111111111111", "test-role")
			}

			if tt.expectWrapped {
				_, ok := client.(*SimpleRateLimitedClient)
				assert.True(t, ok, "Client should be wrapped with rate limiting")
			} else {
				assert.Equal(t, mockClient, client, "Client should be the original unwrapped client")
			}
		})
	}
}

func TestRateLimitingBehavior(t *testing.T) {
	mockClient := &mockClient{}

	config := RateLimiterConfig{
		ListMetrics: &APIRateLimit{Count: 2, Duration: time.Second},
	}

	limiter, err := NewGlobalRateLimiter(config)
	require.NoError(t, err)
	client := NewRateLimitedClient(mockClient, limiter, "us-east-1", "111111111111", "test-role")
	ctx := context.Background()

	// Make multiple calls quickly and measure timing
	start := time.Now()

	// First call should be immediate
	err = client.ListMetrics(ctx, "test", nil, false, nil)
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

func TestRateLimitingUsesIndependentAccountRegionBuckets(t *testing.T) {
	config := RateLimiterConfig{
		ListMetrics: &APIRateLimit{Count: 1, Duration: time.Minute},
	}

	limiter, err := NewGlobalRateLimiter(config)
	require.NoError(t, err)

	firstBucketClient := NewRateLimitedClient(&mockClient{}, limiter, "us-east-1", "111111111111", "test-role")
	sameRegionDifferentAccountClient := NewRateLimitedClient(&mockClient{}, limiter, "us-east-1", "222222222222", "test-role")
	sameAccountDifferentRegionClient := NewRateLimitedClient(&mockClient{}, limiter, "us-west-2", "111111111111", "test-role")

	err = firstBucketClient.ListMetrics(context.Background(), "test", nil, false, nil)
	require.NoError(t, err)

	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = sameRegionDifferentAccountClient.ListMetrics(shortCtx, "test", nil, false, nil)
	require.NoError(t, err)

	err = sameAccountDifferentRegionClient.ListMetrics(shortCtx, "test", nil, false, nil)
	require.NoError(t, err)
}

func TestPerAPIRateLimitingBehavior(t *testing.T) {
	mockClient := &mockClient{}

	config := RateLimiterConfig{
		ListMetrics:         &APIRateLimit{Count: 1, Duration: time.Second},
		GetMetricData:       &APIRateLimit{Count: 10, Duration: time.Second},
		GetMetricStatistics: &APIRateLimit{Count: 5, Duration: time.Second},
	}

	limiter, err := NewGlobalRateLimiter(config)
	require.NoError(t, err)
	client := NewRateLimitedClient(mockClient, limiter, "us-east-1", "111111111111", "test-role")
	ctx := context.Background()

	// Test ListMetrics (most restrictive)
	start := time.Now()
	err = client.ListMetrics(ctx, "test", nil, false, nil)
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

	config := RateLimiterConfig{
		ListMetrics: &APIRateLimit{Count: 1, Duration: time.Minute},
	}

	limiter, err := NewGlobalRateLimiter(config)
	require.NoError(t, err)
	client := NewRateLimitedClient(mockClient, limiter, "us-east-1", "111111111111", "test-role")

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// First call should work
	err = client.ListMetrics(ctx, "test", nil, false, nil)
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
	promutil.CloudwatchRateLimitWaitCounter.Reset()
	promutil.CloudwatchRateLimitAllowedCounter.Reset()

	mockClient := &mockClient{}

	config := RateLimiterConfig{
		ListMetrics: &APIRateLimit{Count: 1, Duration: time.Second},
	}

	limiter, err := NewGlobalRateLimiter(config)
	require.NoError(t, err)
	client := NewRateLimitedClient(mockClient, limiter, "us-east-1", "111111111111", "test-role")
	ctx := context.Background()

	// First call should be allowed immediately
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)

	// Check that allowed counter was incremented
	allowedCount := testutil.ToFloat64(promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(listMetricsCall, "us-east-1", "111111111111", "test-role", "test"))
	assert.Equal(t, float64(1), allowedCount, "First call should be counted as allowed")

	// Second call should be rate limited (will wait)
	start := time.Now()
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err)
	elapsed := time.Since(start)

	// Should have waited due to rate limiting
	assert.True(t, elapsed >= 800*time.Millisecond, "Second call should have been rate limited")

	// Check that wait counter was incremented
	waitCount := testutil.ToFloat64(promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(listMetricsCall, "us-east-1", "111111111111", "test-role", "test"))
	assert.Equal(t, float64(1), waitCount, "Second call should be counted as rate limited")

	// Verify both calls were made
	assert.Equal(t, 2, mockClient.listMetricsCalls)
}

func TestPerAPIRateLimitingMetrics(t *testing.T) {
	promutil.CloudwatchRateLimitWaitCounter.Reset()
	promutil.CloudwatchRateLimitAllowedCounter.Reset()

	mockClient := &mockClient{}

	config := RateLimiterConfig{
		ListMetrics:   &APIRateLimit{Count: 1, Duration: time.Second},
		GetMetricData: &APIRateLimit{Count: 10, Duration: time.Second},
	}

	limiter, err := NewGlobalRateLimiter(config)
	require.NoError(t, err)
	client := NewRateLimitedClient(mockClient, limiter, "us-east-1", "111111111111", "test-role")
	ctx := context.Background()

	// Test ListMetrics (restrictive)
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err) // Should be allowed
	err = client.ListMetrics(ctx, "test", nil, false, nil)
	assert.NoError(t, err) // Should be rate limited

	// Test GetMetricData (permissive)
	client.GetMetricData(ctx, nil, "test", time.Now(), time.Now()) // Should be allowed
	client.GetMetricData(ctx, nil, "test", time.Now(), time.Now()) // Should also be allowed

	// Check ListMetrics metrics
	listAllowedCount := testutil.ToFloat64(promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(listMetricsCall, "us-east-1", "111111111111", "test-role", "test"))
	listWaitCount := testutil.ToFloat64(promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(listMetricsCall, "us-east-1", "111111111111", "test-role", "test"))
	assert.Equal(t, float64(1), listAllowedCount, "ListMetrics should have 1 allowed call")
	assert.Equal(t, float64(1), listWaitCount, "ListMetrics should have 1 rate limited call")

	// Check GetMetricData metrics
	dataAllowedCount := testutil.ToFloat64(promutil.CloudwatchRateLimitAllowedCounter.WithLabelValues(getMetricDataCall, "us-east-1", "111111111111", "test-role", "test"))
	dataWaitCount := testutil.ToFloat64(promutil.CloudwatchRateLimitWaitCounter.WithLabelValues(getMetricDataCall, "us-east-1", "111111111111", "test-role", "test"))
	assert.Equal(t, float64(2), dataAllowedCount, "GetMetricData should have 2 allowed calls")
	assert.Equal(t, float64(0), dataWaitCount, "GetMetricData should have 0 rate limited calls")

	// Verify call counts
	assert.Equal(t, 2, mockClient.listMetricsCalls)
	assert.Equal(t, 2, mockClient.getMetricDataCalls)
}
