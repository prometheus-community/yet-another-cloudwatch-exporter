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
	"fmt"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	"golang.org/x/time/rate"
)

// RateLimitConfig sets the maximum number of requests per second for each CloudWatch API.
// A value of 0 leaves that API unlimited.
type RateLimitConfig struct {
	ListMetrics         float64
	GetMetricData       float64
	GetMetricStatistics float64
}

// Enabled reports whether at least one API has a rate limit.
func (c RateLimitConfig) Enabled() bool {
	return c.ListMetrics > 0 || c.GetMetricData > 0 || c.GetMetricStatistics > 0
}

// RateLimiter paces CloudWatch API requests to the rates of a RateLimitConfig.
//
// CloudWatch applies its API quotas per AWS account and region, while the concurrency limiters
// apply per job: with many jobs, every job may send its requests at once. A RateLimiter is meant
// to be shared by all clients that send requests to the same account and region, so that their
// requests together stay within the configured rates.
type RateLimiter struct {
	limiters map[string]*rate.Limiter
}

// NewRateLimiter creates a RateLimiter for cfg. Requests are spaced evenly: with a burst of one,
// no one-second window holds more requests than the configured rate, which is the window
// CloudWatch evaluates its quotas over.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	limiters := map[string]*rate.Limiter{}
	for operation, requestsPerSecond := range map[string]float64{
		listMetricsCall:         cfg.ListMetrics,
		getMetricDataCall:       cfg.GetMetricData,
		getMetricStatisticsCall: cfg.GetMetricStatistics,
	} {
		if requestsPerSecond > 0 {
			limiters[operation] = rate.NewLimiter(rate.Limit(requestsPerSecond), 1)
		}
	}
	return &RateLimiter{limiters: limiters}
}

// Wait blocks until a request to the CloudWatch API operation may be sent, or ctx is done.
// Operations without a rate limit return immediately.
func (l *RateLimiter) Wait(ctx context.Context, operation string) error {
	limiter, ok := l.limiters[operation]
	if !ok {
		return nil
	}
	return limiter.Wait(ctx)
}

const (
	rateLimitMiddlewareID = "YACERateLimit"
	// retryMiddlewareID is the AWS SDK middleware that runs every attempt of a request.
	retryMiddlewareID = "Retry"
)

// APIOption returns an AWS SDK API option that makes every request of a CloudWatch client wait
// for l. The wait sits at the start of every attempt, right after the retry middleware: retries
// and further pages of a paginated call are paced like the first request, and a request is
// signed only once it may be sent, so no signature ages while it waits.
func (l *RateLimiter) APIOption() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Insert(middleware.FinalizeMiddlewareFunc(rateLimitMiddlewareID,
			func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
				if err := l.Wait(ctx, awsmiddleware.GetOperationName(ctx)); err != nil {
					return middleware.FinalizeOutput{}, middleware.Metadata{}, fmt.Errorf("waiting for the CloudWatch API rate limit: %w", err)
				}
				return next.HandleFinalize(ctx, in)
			},
		), retryMiddlewareID, middleware.After)
	}
}
