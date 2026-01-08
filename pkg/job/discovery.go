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
package job

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/maxdimassociator"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type resourceAssociator interface {
	AssociateMetricToResource(cwMetric *model.Metric) (*model.TaggedResource, bool)
}

// getMetricDataProcessor defines the interface for fetching CloudWatch metric data in batches.
//
// Implementations must process CloudWatch metric requests by:
// - Fetching actual metric data from AWS CloudWatch using GetMetricData API
// - Populating the GetMetricDataResult field in each successful request
// - Handling batching and concurrency efficiently
// - Filtering out requests that failed to retrieve data
//
// The Run method modifies the input requests in-place and returns only those that successfully
// retrieved metric data. It should return an error only if a fatal error occurs during processing.
type getMetricDataProcessor interface {
	Run(ctx context.Context, namespace string, requests []*model.CloudwatchData) ([]*model.CloudwatchData, error)
}

// runDiscoveryJob orchestrates the discovery and retrieval of CloudWatch metrics for a given job.
// It performs the following steps:
// 1. Discovers AWS resources matching the job's tag filters in the specified region
// 2. Generates metric data queries based on available CloudWatch metrics for those resources
// 3. Fetches actual metric data from CloudWatch using the provided processor
//
// Returns the discovered tagged resources and their associated CloudWatch metric data.
// If any step fails, appropriate errors are logged and partial results may be returned.
func runDiscoveryJob(
	ctx context.Context,
	logger *slog.Logger,
	job model.DiscoveryJob,
	region string,
	clientTag tagging.Client,
	clientCloudwatch cloudwatch.Client,
	gmdProcessor getMetricDataProcessor,
) ([]*model.TaggedResource, []*model.CloudwatchData) {
	logger.Debug("Get tagged resources")

	resources, err := clientTag.GetResources(ctx, job, region)
	if err != nil {
		if errors.Is(err, tagging.ErrExpectedToFindResources) {
			logger.Error("No tagged resources made it through filtering", "err", err)
		} else {
			logger.Error("Couldn't describe resources", "err", err)
		}
		return nil, nil
	}

	if len(resources) == 0 {
		logger.Debug("No tagged resources", "region", region, "namespace", job.Namespace)
	}

	svc := config.SupportedServices.GetService(job.Namespace)
	getMetricDatas := getMetricDataForQueries(ctx, logger, job, svc, clientCloudwatch, resources)
	if len(getMetricDatas) == 0 {
		logger.Info("No metrics data found")
		return resources, nil
	}

	getMetricDatas, err = gmdProcessor.Run(ctx, svc.Namespace, getMetricDatas)
	if err != nil {
		logger.Error("Failed to get metric data", "err", err)
		return nil, nil
	}

	return resources, getMetricDatas
}

// getMetricDataForQueries retrieves and processes CloudWatch metrics for a discovery job.
// It performs the following operations:
// 1. Sets up a resource associator based on available dimension regex patterns and discovered resources:
//   - If both dimension regexes and resources exist, uses maxdimassociator for intelligent resource matching
//   - Otherwise, uses a no-op associator that doesn't skip any metrics
//
// 2. Concurrently calls the CloudWatch ListMetrics API for each metric defined in the discovery job:
//   - Fetches all existing dimension combinations with available data
//   - Optionally filters for recently active metrics only
//   - Processes results in paginated batches via callback
//
// 3. For each page of metrics, filters and associates them with resources to create CloudwatchData requests
// 4. Aggregates all metric data requests from concurrent operations in a thread-safe manner
//
// Returns a consolidated list of CloudwatchData requests ready for metric data retrieval.
// Errors from individual ListMetrics calls are logged but don't halt processing of other metrics.
func getMetricDataForQueries(
	ctx context.Context,
	logger *slog.Logger,
	discoveryJob model.DiscoveryJob,
	svc *config.ServiceConfig,
	clientCloudwatch cloudwatch.Client,
	resources []*model.TaggedResource,
) []*model.CloudwatchData {
	mux := &sync.Mutex{}
	var getMetricDatas []*model.CloudwatchData

	var assoc resourceAssociator
	if len(svc.DimensionRegexps) > 0 && len(resources) > 0 {
		assoc = maxdimassociator.NewAssociator(logger, discoveryJob.DimensionsRegexps, resources)
	} else {
		// If we don't have dimension regex's and resources there's nothing to associate but metrics shouldn't be skipped
		assoc = nopAssociator{}
	}

	var wg sync.WaitGroup
	wg.Add(len(discoveryJob.Metrics))

	// For every metric of the job call the ListMetrics API
	// to fetch the existing combinations of dimensions and
	// value of dimensions with data.
	for _, metric := range discoveryJob.Metrics {
		go func(metric *model.MetricConfig) {
			defer wg.Done()

			err := clientCloudwatch.ListMetrics(ctx, svc.Namespace, metric, discoveryJob.RecentlyActiveOnly, func(page []*model.Metric) {
				data := getFilteredMetricDatas(logger, discoveryJob.Namespace, discoveryJob.ExportedTagsOnMetrics, page, discoveryJob.DimensionNameRequirements, metric, assoc)

				mux.Lock()
				getMetricDatas = append(getMetricDatas, data...)
				mux.Unlock()
			})
			if err != nil {
				logger.Error("Failed to get full metric list", "metric_name", metric.Name, "namespace", svc.Namespace, "err", err)
				return
			}
		}(metric)
	}

	wg.Wait()
	return getMetricDatas
}

type nopAssociator struct{}

func (ns nopAssociator) AssociateMetricToResource(_ *model.Metric) (*model.TaggedResource, bool) {
	return nil, false
}

// getFilteredMetricDatas processes a list of CloudWatch metrics and generates metric data requests.
// It performs the following operations:
// 1. Filters metrics based on dimension name requirements to match only those with expected dimensions
// 2. Associates each metric with a specific AWS resource using the provided associator:
//   - The associator matches metrics to tagged resources based on dimension values and regex patterns
//   - If a metric matches a resource, it inherits that resource's tags for export
//   - If a metric cannot be associated with any resource, it uses a global placeholder resource
//   - Metrics explicitly skipped by the associator (no match found when matching is required) are excluded
//
// 3. Creates CloudwatchData requests for each metric statistic, including:
//   - Resource tags filtered by the exportedTagsOnMetrics list
//   - Query parameters (period, length, delay, statistic)
//   - Migration settings (nilToZero, timestamp handling, data point export)
//
// Returns a list of CloudwatchData requests ready for metric data retrieval from CloudWatch.
func getFilteredMetricDatas(
	logger *slog.Logger,
	namespace string,
	tagsOnMetrics []string,
	metricsList []*model.Metric,
	dimensionNameList []string,
	m *model.MetricConfig,
	assoc resourceAssociator,
) []*model.CloudwatchData {
	getMetricsData := make([]*model.CloudwatchData, 0, len(metricsList))
	for _, cwMetric := range metricsList {
		if len(dimensionNameList) > 0 && !metricDimensionsMatchNames(cwMetric, dimensionNameList) {
			continue
		}

		matchedResource, skip := assoc.AssociateMetricToResource(cwMetric)
		if skip {
			dimensions := make([]string, 0, len(cwMetric.Dimensions))
			for _, dim := range cwMetric.Dimensions {
				dimensions = append(dimensions, fmt.Sprintf("%s=%s", dim.Name, dim.Value))
			}
			logger.Debug("skipping metric unmatched by associator", "metric", m.Name, "dimensions", strings.Join(dimensions, ","))

			continue
		}

		resource := matchedResource
		if resource == nil {
			resource = &model.TaggedResource{
				ARN:       "global",
				Namespace: namespace,
			}
		}

		metricTags := resource.MetricTags(tagsOnMetrics)
		for _, stat := range m.Statistics {
			getMetricsData = append(getMetricsData, &model.CloudwatchData{
				MetricName:   m.Name,
				ResourceName: resource.ARN,
				Namespace:    namespace,
				Dimensions:   cwMetric.Dimensions,
				GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
					Period:    m.Period,
					Length:    m.Length,
					Delay:     m.Delay,
					Statistic: stat,
				},
				MetricMigrationParams: model.MetricMigrationParams{
					NilToZero:              m.NilToZero,
					AddCloudwatchTimestamp: m.AddCloudwatchTimestamp,
					ExportAllDataPoints:    m.ExportAllDataPoints,
				},
				Tags:                      metricTags,
				GetMetricDataResult:       nil,
				GetMetricStatisticsResult: nil,
			})
		}
	}
	return getMetricsData
}

func metricDimensionsMatchNames(metric *model.Metric, dimensionNameRequirements []string) bool {
	if len(dimensionNameRequirements) != len(metric.Dimensions) {
		return false
	}
	for _, dimension := range metric.Dimensions {
		foundMatch := false
		for _, dimensionName := range dimensionNameRequirements {
			if dimension.Name == dimensionName {
				foundMatch = true
				break
			}
		}
		if !foundMatch {
			return false
		}
	}
	return true
}
