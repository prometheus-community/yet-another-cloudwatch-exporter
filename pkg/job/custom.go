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
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func runCustomNamespaceJob(
	ctx context.Context,
	logger logging.Logger,
	job model.CustomNamespaceJob,
	clientCloudwatch cloudwatch.Client,
	gmdProcessor getMetricDataProcessor,
) []*model.CloudwatchData {
	cloudwatchDatas := getMetricDataForQueriesForCustomNamespace(ctx, job, clientCloudwatch, logger)
	if len(cloudwatchDatas) == 0 {
		logger.Debug("No metrics data found")
		return nil
	}

	var err error
	cloudwatchDatas, err = gmdProcessor.Run(ctx, job.Namespace, cloudwatchDatas)
	if err != nil {
		logger.Error(err, "Failed to get metric data")
		return nil
	}

	return cloudwatchDatas
}

func getMetricDataForQueriesForCustomNamespace(
	ctx context.Context,
	customNamespaceJob model.CustomNamespaceJob,
	clientCloudwatch cloudwatch.Client,
	logger logging.Logger,
) []*model.CloudwatchData {
	mux := &sync.Mutex{}
	var getMetricDatas []*model.CloudwatchData

	var wg sync.WaitGroup
	wg.Add(len(customNamespaceJob.Metrics))

	for _, metric := range customNamespaceJob.Metrics {
		// For every metric of the job get the full list of metrics.
		// This includes, for this metric the possible combinations
		// of dimensions and value of dimensions with data.

		go func(metric *model.MetricConfig) {
			defer wg.Done()
			err := clientCloudwatch.ListMetrics(ctx, customNamespaceJob.Namespace, metric, customNamespaceJob.RecentlyActiveOnly, func(page []*model.Metric) {
				var data []*model.CloudwatchData

				for _, cwMetric := range page {
					if len(customNamespaceJob.DimensionNameRequirements) > 0 && !metricDimensionsMatchNames(cwMetric, customNamespaceJob.DimensionNameRequirements) {
						continue
					}

					for _, stat := range metric.Statistics {
						data = append(data, &model.CloudwatchData{
							MetricName:   metric.Name,
							ResourceName: customNamespaceJob.Name,
							Namespace:    customNamespaceJob.Namespace,
							Dimensions:   cwMetric.Dimensions,
							GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
								Period:    metric.Period,
								Length:    metric.Length,
								Delay:     metric.Delay,
								Statistic: stat,
							},
							MetricMigrationParams: model.MetricMigrationParams{
								NilToZero:              metric.NilToZero,
								AddCloudwatchTimestamp: metric.AddCloudwatchTimestamp,
							},
							Tags:                      nil,
							GetMetricDataResult:       nil,
							GetMetricStatisticsResult: nil,
						})
					}
				}

				mux.Lock()
				getMetricDatas = append(getMetricDatas, data...)
				mux.Unlock()
			})
			if err != nil {
				logger.Error(err, "Failed to get full metric list", "metric_name", metric.Name, "namespace", customNamespaceJob.Namespace)
				return
			}
		}(metric)
	}

	wg.Wait()
	return getMetricDatas
}
