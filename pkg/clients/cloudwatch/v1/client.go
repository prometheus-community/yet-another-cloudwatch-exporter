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
package v1

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"

	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

type client struct {
	logger        *slog.Logger
	cloudwatchAPI cloudwatchiface.CloudWatchAPI
}

func NewClient(logger *slog.Logger, cloudwatchAPI cloudwatchiface.CloudWatchAPI) cloudwatch_client.Client {
	return &client{
		logger:        logger,
		cloudwatchAPI: cloudwatchAPI,
	}
}

func (c client) ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, includeLinkedAccounts []string, recentlyActiveOnly bool, fn func(page []*model.Metric)) error {
	filter := &cloudwatch.ListMetricsInput{
		MetricName: aws.String(metric.Name),
		Namespace:  aws.String(namespace),
	}
	if len(includeLinkedAccounts) > 0 {
		filter.IncludeLinkedAccounts = aws.Bool(true)
	}
	if recentlyActiveOnly {
		filter.RecentlyActive = aws.String("PT3H")
	}

	c.logger.Debug("ListMetrics", "input", filter)

	err := c.cloudwatchAPI.ListMetricsPagesWithContext(ctx, filter, func(page *cloudwatch.ListMetricsOutput, lastPage bool) bool {
		promutil.CloudwatchAPICounter.WithLabelValues("ListMetrics").Inc()

		metricsPage := toModelMetric(page, includeLinkedAccounts)

		c.logger.Debug("ListMetrics", "output", metricsPage, "last_page", lastPage)

		fn(metricsPage)
		return !lastPage
	})
	if err != nil {
		promutil.CloudwatchAPIErrorCounter.WithLabelValues("ListMetrics").Inc()
		c.logger.Error("ListMetrics error", "err", err)
		return err
	}

	return nil
}

func toModelMetric(page *cloudwatch.ListMetricsOutput, includeLinkedAccounts []string) []*model.Metric {
	modelMetrics := make([]*model.Metric, 0, len(page.Metrics))
	if len(includeLinkedAccounts) > 0 {
		for i := 0; i < len(page.Metrics); i++ {
			linkedAccountId := *page.OwningAccounts[i]
			if !slices.Contains(includeLinkedAccounts, "*") && !slices.Contains(includeLinkedAccounts, linkedAccountId) {
				continue
			}
			modelMetric := &model.Metric{
				MetricName:      *page.Metrics[i].MetricName,
				Namespace:       *page.Metrics[i].Namespace,
				Dimensions:      toModelDimensions(page.Metrics[i].Dimensions),
				LinkedAccountId: linkedAccountId,
			}
			modelMetrics = append(modelMetrics, modelMetric)
		}
	} else {
		for _, cloudwatchMetric := range page.Metrics {
			modelMetric := &model.Metric{
				MetricName: *cloudwatchMetric.MetricName,
				Namespace:  *cloudwatchMetric.Namespace,
				Dimensions: toModelDimensions(cloudwatchMetric.Dimensions),
			}
			modelMetrics = append(modelMetrics, modelMetric)
		}
	}
	return modelMetrics
}

func toModelDimensions(dimensions []*cloudwatch.Dimension) []model.Dimension {
	modelDimensions := make([]model.Dimension, 0, len(dimensions))
	for _, dimension := range dimensions {
		modelDimension := model.Dimension{
			Name:  *dimension.Name,
			Value: *dimension.Value,
		}
		modelDimensions = append(modelDimensions, modelDimension)
	}
	return modelDimensions
}

func (c client) GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []cloudwatch_client.MetricDataResult {
	metricDataQueries := make([]*cloudwatch.MetricDataQuery, 0, len(getMetricData))
	for _, data := range getMetricData {
		metricStat := &cloudwatch.MetricStat{
			Metric: &cloudwatch.Metric{
				Dimensions: toCloudWatchDimensions(data.Dimensions),
				MetricName: &data.MetricName,
				Namespace:  &namespace,
			},
			Period: &data.GetMetricDataProcessingParams.Period,
			Stat:   &data.GetMetricDataProcessingParams.Statistic,
		}
		metricDataQuery := &cloudwatch.MetricDataQuery{
			Id:         &data.GetMetricDataProcessingParams.QueryID,
			MetricStat: metricStat,
			ReturnData: aws.Bool(true),
		}
		if data.LinkedAccountId != "" {
			metricDataQuery.AccountId = aws.String(data.LinkedAccountId)
		}
		metricDataQueries = append(metricDataQueries, metricDataQuery)
	}
	input := &cloudwatch.GetMetricDataInput{
		EndTime:           &endTime,
		StartTime:         &startTime,
		MetricDataQueries: metricDataQueries,
		ScanBy:            aws.String("TimestampDescending"),
	}
	promutil.CloudwatchGetMetricDataAPIMetricsCounter.Add(float64(len(input.MetricDataQueries)))
	c.logger.Debug("GetMetricData", "input", input)

	var resp cloudwatch.GetMetricDataOutput
	// Using the paged version of the function
	err := c.cloudwatchAPI.GetMetricDataPagesWithContext(ctx, input,
		func(page *cloudwatch.GetMetricDataOutput, lastPage bool) bool {
			promutil.CloudwatchGetMetricDataAPICounter.Inc()
			promutil.CloudwatchAPICounter.WithLabelValues("GetMetricData").Inc()
			resp.MetricDataResults = append(resp.MetricDataResults, page.MetricDataResults...)
			return !lastPage
		})

	c.logger.Debug("GetMetricData", "output", resp)

	if err != nil {
		promutil.CloudwatchAPIErrorCounter.WithLabelValues("GetMetricData").Inc()
		c.logger.Error("GetMetricData error", "err", err)
		return nil
	}
	return toMetricDataResult(resp)
}

func toMetricDataResult(resp cloudwatch.GetMetricDataOutput) []cloudwatch_client.MetricDataResult {
	output := make([]cloudwatch_client.MetricDataResult, 0, len(resp.MetricDataResults))
	for _, metricDataResult := range resp.MetricDataResults {
		mappedResult := cloudwatch_client.MetricDataResult{ID: *metricDataResult.Id}
		if len(metricDataResult.Values) > 0 {
			mappedResult.Datapoint = metricDataResult.Values[0]
			mappedResult.Timestamp = *metricDataResult.Timestamps[0]
		}
		output = append(output, mappedResult)
	}
	return output
}

func (c client) GetMetricStatistics(ctx context.Context, logger *slog.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.Datapoint {
	filter := createGetMetricStatisticsInput(dimensions, &namespace, metric, logger)

	c.logger.Debug("GetMetricStatistics", "input", filter)

	resp, err := c.cloudwatchAPI.GetMetricStatisticsWithContext(ctx, filter)

	c.logger.Debug("GetMetricStatistics", "output", resp)

	promutil.CloudwatchGetMetricStatisticsAPICounter.Inc()
	promutil.CloudwatchAPICounter.WithLabelValues("GetMetricStatistics").Inc()

	if err != nil {
		promutil.CloudwatchAPIErrorCounter.WithLabelValues("GetMetricStatistics").Inc()
		c.logger.Error("Failed to get metric statistics", "err", err)
		return nil
	}

	return toModelDatapoints(resp.Datapoints)
}

func toModelDatapoints(cwDatapoints []*cloudwatch.Datapoint) []*model.Datapoint {
	modelDataPoints := make([]*model.Datapoint, 0, len(cwDatapoints))

	for _, cwDatapoint := range cwDatapoints {
		modelDataPoints = append(modelDataPoints, &model.Datapoint{
			Average:            cwDatapoint.Average,
			ExtendedStatistics: cwDatapoint.ExtendedStatistics,
			Maximum:            cwDatapoint.Maximum,
			Minimum:            cwDatapoint.Minimum,
			SampleCount:        cwDatapoint.SampleCount,
			Sum:                cwDatapoint.Sum,
			Timestamp:          cwDatapoint.Timestamp,
		})
	}
	return modelDataPoints
}
