// Copyright The Prometheus Authors
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
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_cloudwatch "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimitConfig_Enabled(t *testing.T) {
	assert.False(t, RateLimitConfig{}.Enabled())
	assert.True(t, RateLimitConfig{ListMetrics: 1}.Enabled())
	assert.True(t, RateLimitConfig{GetMetricData: 1}.Enabled())
	assert.True(t, RateLimitConfig{GetMetricStatistics: 0.5}.Enabled())
}

func TestRateLimiter_SpacesRequestsOfALimitedOperation(t *testing.T) {
	limiter := NewRateLimiter(RateLimitConfig{GetMetricStatistics: 50})

	// 11 concurrent requests at 50 per second: the first one passes at once, the other ten
	// follow 20ms apart, as concurrent jobs would send them.
	start := time.Now()
	var wg sync.WaitGroup
	for range 11 {
		wg.Go(func() {
			assert.NoError(t, limiter.Wait(context.Background(), getMetricStatisticsCall))
		})
	}
	wg.Wait()

	// Lower bound only: a slow machine may take longer, never shorter.
	assert.GreaterOrEqual(t, time.Since(start), 180*time.Millisecond)
}

func TestRateLimiter_LeavesOperationsWithoutALimitUnlimited(t *testing.T) {
	limiter := NewRateLimiter(RateLimitConfig{GetMetricStatistics: 1})

	start := time.Now()
	for range 1000 {
		require.NoError(t, limiter.Wait(context.Background(), listMetricsCall))
		require.NoError(t, limiter.Wait(context.Background(), getMetricDataCall))
	}
	assert.Less(t, time.Since(start), time.Second)
}

func TestRateLimiter_StopsWaitingWhenTheContextEnds(t *testing.T) {
	limiter := NewRateLimiter(RateLimitConfig{GetMetricStatistics: 0.1})
	require.NoError(t, limiter.Wait(context.Background(), getMetricStatisticsCall))

	// The next request is 10s away, far beyond the deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	assert.Error(t, limiter.Wait(ctx, getMetricStatisticsCall))
	assert.Less(t, time.Since(start), time.Second)
}

// recordingHTTPClient records when each request is sent and answers with an error, which is all
// a test of request pacing needs.
type recordingHTTPClient struct {
	mu       sync.Mutex
	requests []time.Time
}

func (c *recordingHTTPClient) Do(*http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.requests = append(c.requests, time.Now())
	c.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestRateLimiter_APIOptionPacesEverySDKRequest(t *testing.T) {
	httpClient := &recordingHTTPClient{}
	limiter := NewRateLimiter(RateLimitConfig{GetMetricStatistics: 50})
	sdkClient := aws_cloudwatch.New(aws_cloudwatch.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String("http://localhost"),
		Credentials:  aws.AnonymousCredentials{},
		HTTPClient:   httpClient,
		Retryer:      aws.NopRetryer{},
		APIOptions:   []func(*middleware.Stack) error{limiter.APIOption()},
	})

	now := time.Now()
	input := &aws_cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/EC2"),
		MetricName: aws.String("CPUUtilization"),
		StartTime:  aws.Time(now.Add(-5 * time.Minute)),
		EndTime:    aws.Time(now),
		Period:     aws.Int32(60),
		Statistics: []types.Statistic{types.StatisticAverage},
	}

	var wg sync.WaitGroup
	for range 11 {
		wg.Go(func() {
			// The recording client answers every request with an error.
			_, _ = sdkClient.GetMetricStatistics(context.Background(), input)
		})
	}
	wg.Wait()

	httpClient.mu.Lock()
	defer httpClient.mu.Unlock()
	require.Len(t, httpClient.requests, 11)
	first, last := httpClient.requests[0], httpClient.requests[0]
	for _, sent := range httpClient.requests {
		if sent.Before(first) {
			first = sent
		}
		if sent.After(last) {
			last = sent
		}
	}
	assert.GreaterOrEqual(t, last.Sub(first), 180*time.Millisecond)
}

func TestRateLimiter_APIOptionWaitsBeforeTheRequestIsSigned(t *testing.T) {
	var finalize []string
	sdkClient := aws_cloudwatch.New(aws_cloudwatch.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String("http://localhost"),
		Credentials:  aws.AnonymousCredentials{},
		HTTPClient:   &recordingHTTPClient{},
		APIOptions: []func(*middleware.Stack) error{
			NewRateLimiter(RateLimitConfig{GetMetricStatistics: 1}).APIOption(),
			func(stack *middleware.Stack) error {
				finalize = stack.Finalize.List()
				return nil
			},
		},
	})
	now := time.Now()
	_, _ = sdkClient.GetMetricStatistics(context.Background(), &aws_cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String("AWS/EC2"),
		MetricName: aws.String("CPUUtilization"),
		StartTime:  aws.Time(now.Add(-5 * time.Minute)),
		EndTime:    aws.Time(now),
		Period:     aws.Int32(60),
		Statistics: []types.Statistic{types.StatisticAverage},
	})

	// Right after the retry middleware, so every attempt waits, and before signing, so a request
	// is signed only once it may be sent.
	rateLimit := slices.Index(finalize, rateLimitMiddlewareID)
	require.Positive(t, rateLimit, "finalize step: %v", finalize)
	assert.Equal(t, retryMiddlewareID, finalize[rateLimit-1])
	assert.Less(t, rateLimit, slices.Index(finalize, "Signing"), "finalize step: %v", finalize)
}
