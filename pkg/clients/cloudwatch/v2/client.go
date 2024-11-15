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
package v2

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

type client struct {
	logger        logging.Logger
	cloudwatchAPI *cloudwatch.Client
}

func NewClient(logger logging.Logger, cloudwatchAPI *cloudwatch.Client) cloudwatch_client.Client {
	return &client{
		logger:        logger,
		cloudwatchAPI: cloudwatchAPI,
	}
}

func (c client) ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, recentlyActiveOnly bool, fn func(page []*model.Metric)) error {
	filter := &cloudwatch.ListMetricsInput{
		MetricName: aws.String(metric.Name),
		Namespace:  aws.String(namespace),
	}
	if recentlyActiveOnly {
		filter.RecentlyActive = types.RecentlyActivePt3h
	}

	if c.logger.IsDebugEnabled() {
		c.logger.Debug("ListMetrics", "input", filter)
	}

	paginator := cloudwatch.NewListMetricsPaginator(c.cloudwatchAPI, filter, func(options *cloudwatch.ListMetricsPaginatorOptions) {
		options.StopOnDuplicateToken = true
	})

	for paginator.HasMorePages() {
		promutil.CloudwatchAPICounter.WithLabelValues("ListMetrics").Inc()
		page, err := paginator.NextPage(ctx)
		if err != nil {
			promutil.CloudwatchAPIErrorCounter.WithLabelValues("ListMetrics").Inc()
			c.logger.Error(err, "ListMetrics error")
			return err
		}

		metricsPage := toModelMetric(page)
		if c.logger.IsDebugEnabled() {
			c.logger.Debug("ListMetrics", "output", metricsPage)
		}

		fn(metricsPage)
	}

	return nil
}

func toModelMetric(page *cloudwatch.ListMetricsOutput) []*model.Metric {
	modelMetrics := make([]*model.Metric, 0, len(page.Metrics))
	for _, cloudwatchMetric := range page.Metrics {
		modelMetric := &model.Metric{
			MetricName: *cloudwatchMetric.MetricName,
			Namespace:  *cloudwatchMetric.Namespace,
			Dimensions: toModelDimensions(cloudwatchMetric.Dimensions),
		}
		modelMetrics = append(modelMetrics, modelMetric)
	}
	return modelMetrics
}

func toModelDimensions(dimensions []types.Dimension) []model.Dimension {
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
	metricDataQueries := make([]types.MetricDataQuery, 0, len(getMetricData))
	for _, data := range getMetricData {
		metricStat := &types.MetricStat{
			Metric: &types.Metric{
				Dimensions: toCloudWatchDimensions(data.Dimensions),
				MetricName: &data.MetricName,
				Namespace:  &namespace,
			},
			Period: aws.Int32(int32(data.GetMetricDataProcessingParams.Period)),
			Stat:   &data.GetMetricDataProcessingParams.Statistic,
		}
		metricDataQueries = append(metricDataQueries, types.MetricDataQuery{
			Id:         &data.GetMetricDataProcessingParams.QueryID,
			MetricStat: metricStat,
			ReturnData: aws.Bool(true),
		})
	}

	input := &cloudwatch.GetMetricDataInput{
		EndTime:           &endTime,
		StartTime:         &startTime,
		MetricDataQueries: metricDataQueries,
		ScanBy:            "TimestampDescending",
	}
	var resp cloudwatch.GetMetricDataOutput
	promutil.CloudwatchGetMetricDataAPIMetricsCounter.Add(float64(len(input.MetricDataQueries)))
	if c.logger.IsDebugEnabled() {
		c.logger.Debug("GetMetricData", "input", input)
	}

	paginator := cloudwatch.NewGetMetricDataPaginator(c.cloudwatchAPI, input, func(options *cloudwatch.GetMetricDataPaginatorOptions) {
		options.StopOnDuplicateToken = true
	})
	for paginator.HasMorePages() {
		promutil.CloudwatchAPICounter.WithLabelValues("GetMetricData").Inc()
		promutil.CloudwatchGetMetricDataAPICounter.Inc()

		page, err := paginator.NextPage(ctx)
		if err != nil {
			promutil.CloudwatchAPIErrorCounter.WithLabelValues("GetMetricData").Inc()
			c.logger.Error(err, "GetMetricData error")
			return nil
		}
		resp.MetricDataResults = append(resp.MetricDataResults, page.MetricDataResults...)
	}

	if c.logger.IsDebugEnabled() {
		c.logger.Debug("GetMetricData", "output", resp)
	}

	return toMetricDataResult(resp)
}

func toMetricDataResult(resp cloudwatch.GetMetricDataOutput) []cloudwatch_client.MetricDataResult {
	output := make([]cloudwatch_client.MetricDataResult, 0, len(resp.MetricDataResults))
	for _, metricDataResult := range resp.MetricDataResults {
		mappedResult := cloudwatch_client.MetricDataResult{ID: *metricDataResult.Id}
		if len(metricDataResult.Values) > 0 {
			mappedResult.Datapoint = &metricDataResult.Values[0]
			mappedResult.Timestamp = metricDataResult.Timestamps[0]
		}
		output = append(output, mappedResult)
	}
	return output
}

func (c client) GetMetricStatistics(ctx context.Context, logger logging.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.Datapoint {
	filter := createGetMetricStatisticsInput(logger, dimensions, &namespace, metric)
	if c.logger.IsDebugEnabled() {
		c.logger.Debug("GetMetricStatistics", "input", filter)
	}

	resp, err := c.cloudwatchAPI.GetMetricStatistics(ctx, filter)

	if c.logger.IsDebugEnabled() {
		c.logger.Debug("GetMetricStatistics", "output", resp)
	}

	promutil.CloudwatchAPICounter.WithLabelValues("GetMetricStatistics").Inc()
	promutil.CloudwatchGetMetricStatisticsAPICounter.Inc()

	if err != nil {
		promutil.CloudwatchAPIErrorCounter.WithLabelValues("GetMetricStatistics").Inc()
		c.logger.Error(err, "Failed to get metric statistics")
		return nil
	}

	ptrs := make([]*types.Datapoint, 0, len(resp.Datapoints))
	for _, datapoint := range resp.Datapoints {
		ptrs = append(ptrs, &datapoint)
	}

	return toModelDatapoints(ptrs)
}

func toModelDatapoints(cwDatapoints []*types.Datapoint) []*model.Datapoint {
	modelDataPoints := make([]*model.Datapoint, 0, len(cwDatapoints))

	for _, cwDatapoint := range cwDatapoints {
		extendedStats := make(map[string]*float64, len(cwDatapoint.ExtendedStatistics))
		for name, value := range cwDatapoint.ExtendedStatistics {
			extendedStats[name] = &value
		}
		modelDataPoints = append(modelDataPoints, &model.Datapoint{
			Average:            cwDatapoint.Average,
			ExtendedStatistics: extendedStats,
			Maximum:            cwDatapoint.Maximum,
			Minimum:            cwDatapoint.Minimum,
			SampleCount:        cwDatapoint.SampleCount,
			Sum:                cwDatapoint.Sum,
			Timestamp:          cwDatapoint.Timestamp,
		})
	}
	return modelDataPoints
}
