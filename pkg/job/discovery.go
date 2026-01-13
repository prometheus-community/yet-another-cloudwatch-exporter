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

type getMetricDataProcessor interface {
	Run(ctx context.Context, namespace string, requests []*model.CloudwatchData) ([]*model.CloudwatchData, error)
}

type enhancedMetricsService interface {
	GetMetrics(
		ctx context.Context,
		logger *slog.Logger,
		namespace string,
		resources []*model.TaggedResource,
		metrics []*model.EnhancedMetricConfig,
		exportedTagOnMetrics []string,
		region string,
		role model.Role,
	) ([]*model.CloudwatchData, error)
}

func runDiscoveryJob(
	ctx context.Context,
	logger *slog.Logger,
	job model.DiscoveryJob,
	region string,
	clientTag tagging.Client,
	clientCloudwatch cloudwatch.Client,
	gmdProcessor getMetricDataProcessor,
	enhancedMetricsService enhancedMetricsService,
	role model.Role,
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
	metricData := getMetricDataForQueries(ctx, logger, job, svc, clientCloudwatch, resources)

	if len(metricData) > 0 && svc != nil {
		metricData, err = gmdProcessor.Run(ctx, svc.Namespace, metricData)
		if err != nil {
			logger.Error("Failed to get metric data", "err", err)

			// ensure we do not return cw metrics on data processing failure
			metricData = nil
		}
	}

	if enhancedMetricsService == nil || !job.HasEnhancedMetrics() || svc == nil {
		if len(metricData) == 0 {
			logger.Info("No metrics data found")
		}
		return resources, metricData
	}

	logger.Debug("Processing enhanced metrics", "count", len(job.EnhancedMetrics), "namespace", svc.Namespace)
	enhancedMetricData, err := enhancedMetricsService.GetMetrics(
		ctx,
		logger,
		svc.Namespace,
		resources,
		job.EnhancedMetrics,
		job.ExportedTagsOnMetrics,
		region,
		role,
	)
	if err != nil {
		logger.Error("Failed to get enhanced metrics", "err", err)
		return resources, metricData
	}

	metricData = append(metricData, enhancedMetricData...)

	if len(metricData) == 0 {
		logger.Info("No metrics data found")
	}

	return resources, metricData
}

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
