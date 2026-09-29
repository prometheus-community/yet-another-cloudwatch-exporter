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
package tagging

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// fakeTaggingAPI serves a fixed set of resources and counts GetResources calls.
type fakeTaggingAPI struct {
	mappings []types.ResourceTagMapping
	err      error
	calls    int
}

func (f *fakeTaggingAPI) adapter() taggingClientAdapter {
	return taggingClientAdapter{getResources: func(_ context.Context, _ *resourcegroupstaggingapi.GetResourcesInput, _ ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error) {
		f.calls++
		if f.err != nil {
			return nil, f.err
		}
		return &resourcegroupstaggingapi.GetResourcesOutput{ResourceTagMappingList: f.mappings}, nil
	}}
}

// erroringCache fails every operation, like an unreachable Valkey.
type erroringCache struct{}

func (erroringCache) Get(context.Context, string) ([]ResourceTagMappingCache, bool, error) {
	return nil, false, errors.New("connection refused")
}

func (erroringCache) Set(context.Context, string, []ResourceTagMappingCache) error {
	return errors.New("connection refused")
}

func (erroringCache) Close() {}

func bucket(name string, tags ...string) types.ResourceTagMapping {
	m := types.ResourceTagMapping{ResourceARN: aws.String("arn:aws:s3:::" + name)}
	for i := 0; i < len(tags); i += 2 {
		m.Tags = append(m.Tags, types.Tag{Key: aws.String(tags[i]), Value: aws.String(tags[i+1])})
	}
	return m
}

func s3Job(tenant string) model.DiscoveryJob {
	return model.DiscoveryJob{
		Namespace: "AWS/S3",
		SearchTags: []model.SearchTag{
			{Key: "ManagedBy", Value: regexp.MustCompile(".+/" + tenant)},
		},
	}
}

// newTestClient returns a tagging client using the given AWS fake and cache, plus
// the registry holding its scrape metrics.
func newTestClient(api *fakeTaggingAPI, cache Cache) (client, *prometheus.Registry) {
	reg := prometheus.NewRegistry()
	return client{
		logger:        slog.New(slog.DiscardHandler),
		scrapeMetrics: promutil.NewScrapeMetrics(reg),
		taggingAPI:    api.adapter(),
		cache:         cache,
	}, reg
}

func taggingAPIRequests(t *testing.T, reg *prometheus.Registry) float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() == "yace_cloudwatch_resourcegrouptaggingapi_requests_total" {
			return f.GetMetric()[0].GetCounter().GetValue()
		}
	}
	return 0
}

func arns(resources []*model.TaggedResource) []string {
	out := make([]string, 0, len(resources))
	for _, r := range resources {
		out = append(out, r.ARN)
	}
	return out
}

var sharedAccountBuckets = []types.ResourceTagMapping{
	bucket("tenant-a-logs", "ManagedBy", "cnp/tenant-a", "team", "a"),
	bucket("tenant-b-data", "ManagedBy", "cnp/tenant-b"),
}

func TestGetResources_WithoutCacheAlwaysCallsAWS(t *testing.T) {
	api := &fakeTaggingAPI{mappings: sharedAccountBuckets}
	c, reg := newTestClient(api, nil)

	for range 2 {
		resources, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
		require.NoError(t, err)
		require.Equal(t, []string{"arn:aws:s3:::tenant-a-logs"}, arns(resources))
	}
	require.Equal(t, 2, api.calls)
	require.Equal(t, 2.0, taggingAPIRequests(t, reg))
}

func TestGetResources_SecondInstanceIsServedFromSharedCache(t *testing.T) {
	cache, _ := newMapBackedCache(time.Minute)

	// Two YACE instances, each with its own AWS client and metrics, sharing one cache
	apiA := &fakeTaggingAPI{mappings: sharedAccountBuckets}
	instanceA, regA := newTestClient(apiA, cache)
	apiB := &fakeTaggingAPI{mappings: sharedAccountBuckets}
	instanceB, regB := newTestClient(apiB, cache)

	resourcesA, err := instanceA.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
	require.NoError(t, err)
	resourcesB, err := instanceB.GetResources(context.Background(), s3Job("tenant-b"), "eu-central-1")
	require.NoError(t, err)

	require.Equal(t, 1, apiA.calls)
	require.Equal(t, 1.0, taggingAPIRequests(t, regA))
	require.Equal(t, 0, apiB.calls, "instance B must be served from the cache entry written by A")
	require.Equal(t, 0.0, taggingAPIRequests(t, regB))

	// The cached payload is shared, but the tenant filtering still happens per instance
	require.Equal(t, []string{"arn:aws:s3:::tenant-a-logs"}, arns(resourcesA))
	require.Equal(t, []string{"arn:aws:s3:::tenant-b-data"}, arns(resourcesB))
}

func TestGetResources_CachedResultEqualsUncachedResult(t *testing.T) {
	cache, _ := newMapBackedCache(time.Minute)
	api := &fakeTaggingAPI{mappings: []types.ResourceTagMapping{
		bucket("tenant-a-logs", "ManagedBy", "cnp/tenant-a", "z", "1", "a", "2", "m", "3"),
	}}
	c, _ := newTestClient(api, cache)

	fromAWS, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
	require.NoError(t, err)
	fromCache, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
	require.NoError(t, err)

	require.Equal(t, 1, api.calls)
	require.Equal(t, fromAWS, fromCache)
	// Tag order as returned by AWS is preserved
	require.Equal(t, []model.Tag{
		{Key: "ManagedBy", Value: "cnp/tenant-a"}, {Key: "z", Value: "1"}, {Key: "a", Value: "2"}, {Key: "m", Value: "3"},
	}, fromCache[0].Tags)
}

func TestGetResources_EmptyResultIsServedFromCache(t *testing.T) {
	cache, stored := newMapBackedCache(time.Minute)
	api := &fakeTaggingAPI{}
	c, reg := newTestClient(api, cache)

	for range 3 {
		_, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
		require.ErrorIs(t, err, ErrExpectedToFindResources)
	}

	require.Equal(t, 1, api.calls, "an empty result must be cached and not be fetched from AWS again")
	require.Equal(t, 1.0, taggingAPIRequests(t, reg))
	require.Equal(t, "[]", stored[BuildCacheKey("eu-central-1", []string{"s3"}, []string{"ManagedBy"})])
}

func TestGetResources_CacheErrorFallsBackToAWS(t *testing.T) {
	api := &fakeTaggingAPI{mappings: sharedAccountBuckets}
	c, _ := newTestClient(api, erroringCache{})

	for range 2 {
		resources, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
		require.NoError(t, err)
		require.Equal(t, []string{"arn:aws:s3:::tenant-a-logs"}, arns(resources))
	}
	require.Equal(t, 2, api.calls)
}

func TestGetResources_AWSErrorIsNotCached(t *testing.T) {
	cache, stored := newMapBackedCache(time.Minute)
	api := &fakeTaggingAPI{err: errors.New("ThrottledException")}
	c, _ := newTestClient(api, cache)

	_, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
	require.ErrorContains(t, err, "ThrottledException")
	require.Empty(t, stored)

	api.err = nil
	api.mappings = sharedAccountBuckets
	resources, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
	require.NoError(t, err)
	require.Equal(t, []string{"arn:aws:s3:::tenant-a-logs"}, arns(resources))
	require.Equal(t, 2, api.calls)
}

func TestGetResources_DifferentTagKeysDoNotShareCacheEntries(t *testing.T) {
	cache, stored := newMapBackedCache(time.Minute)
	api := &fakeTaggingAPI{mappings: sharedAccountBuckets}
	c, _ := newTestClient(api, cache)

	_, err := c.GetResources(context.Background(), s3Job("tenant-a"), "eu-central-1")
	require.NoError(t, err)

	job := s3Job("tenant-a")
	job.SearchTags = append(job.SearchTags, model.SearchTag{Key: "team", Value: regexp.MustCompile("a")})
	_, err = c.GetResources(context.Background(), job, "eu-central-1")
	require.NoError(t, err)

	require.Equal(t, 2, api.calls)
	require.Len(t, stored, 2)
}
