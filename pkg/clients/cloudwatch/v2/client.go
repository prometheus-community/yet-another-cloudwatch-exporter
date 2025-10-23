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
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

type client struct {
	logger        *slog.Logger
	cloudwatchAPI *cloudwatch.Client
}

func NewClient(logger *slog.Logger, cloudwatchAPI *cloudwatch.Client) cloudwatch_client.Client {
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

	c.logger.Debug("ListMetrics", "input", filter)

	paginator := cloudwatch.NewListMetricsPaginator(c.cloudwatchAPI, filter, func(options *cloudwatch.ListMetricsPaginatorOptions) {
		options.StopOnDuplicateToken = true
	})

	for paginator.HasMorePages() {
		promutil.CloudwatchAPICounter.WithLabelValues("ListMetrics").Inc()
		page, err := paginator.NextPage(ctx)
		if err != nil {
			promutil.CloudwatchAPIErrorCounter.WithLabelValues("ListMetrics").Inc()
			c.logger.Error("ListMetrics error", "err", err)
			return err
		}

		metricsPage := toModelMetric(page)
		c.logger.Debug("ListMetrics", "output", metricsPage)

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
	exportAllDataPoints := false
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
		exportAllDataPoints = exportAllDataPoints || data.MetricMigrationParams.ExportAllDataPoints
	}

	input := &cloudwatch.GetMetricDataInput{
		EndTime:           &endTime,
		StartTime:         &startTime,
		MetricDataQueries: metricDataQueries,
		ScanBy:            "TimestampDescending",
	}
	var resp cloudwatch.GetMetricDataOutput
	promutil.CloudwatchGetMetricDataAPIMetricsCounter.Add(float64(len(input.MetricDataQueries)))
	c.logger.Debug("GetMetricData", "input", input)

	paginator := cloudwatch.NewGetMetricDataPaginator(c.cloudwatchAPI, input, func(options *cloudwatch.GetMetricDataPaginatorOptions) {
		options.StopOnDuplicateToken = true
	})
	for paginator.HasMorePages() {
		promutil.CloudwatchAPICounter.WithLabelValues("GetMetricData").Inc()
		promutil.CloudwatchGetMetricDataAPICounter.Inc()

		page, err := paginator.NextPage(ctx)
		if err != nil {
			promutil.CloudwatchAPIErrorCounter.WithLabelValues("GetMetricData").Inc()
			c.logger.Error("GetMetricData error", "err", err)
			return nil
		}
		resp.MetricDataResults = append(resp.MetricDataResults, page.MetricDataResults...)
	}

	c.logger.Debug("GetMetricData", "output", resp)

	return toMetricDataResult(resp, exportAllDataPoints)
}

func toMetricDataResult(resp cloudwatch.GetMetricDataOutput, exportAllDataPoints bool) []cloudwatch_client.MetricDataResult {
	output := make([]cloudwatch_client.MetricDataResult, 0, len(resp.MetricDataResults))
	for _, metricDataResult := range resp.MetricDataResults {
		mappedResult := cloudwatch_client.MetricDataResult{
			ID:         *metricDataResult.Id,
			DataPoints: make([]cloudwatch_client.DataPoint, 0, len(metricDataResult.Timestamps)),
		}
		for i := 0; i < len(metricDataResult.Timestamps); i++ {
			mappedResult.DataPoints = append(mappedResult.DataPoints, cloudwatch_client.DataPoint{
				Value:     &metricDataResult.Values[i],
				Timestamp: metricDataResult.Timestamps[i],
			})

			if !exportAllDataPoints {
				break
			}
		}
		output = append(output, mappedResult)
	}
	return output
}

func (c client) GetMetricStatistics(ctx context.Context, logger *slog.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.MetricStatisticsResult {
	filter := createGetMetricStatisticsInput(logger, dimensions, &namespace, metric)
	c.logger.Debug("GetMetricStatistics", "input", filter)

	resp, err := c.cloudwatchAPI.GetMetricStatistics(ctx, filter)

	c.logger.Debug("GetMetricStatistics", "output", resp)

	promutil.CloudwatchAPICounter.WithLabelValues("GetMetricStatistics").Inc()
	promutil.CloudwatchGetMetricStatisticsAPICounter.Inc()

	if err != nil {
		promutil.CloudwatchAPIErrorCounter.WithLabelValues("GetMetricStatistics").Inc()
		c.logger.Error("Failed to get metric statistics", "err", err)
		return nil
	}

	ptrs := make([]*types.Datapoint, 0, len(resp.Datapoints))
	for _, datapoint := range resp.Datapoints {
		ptrs = append(ptrs, &datapoint)
	}

	return toModelDataPoints(ptrs)
}

func toModelDataPoints(cwDataPoints []*types.Datapoint) []*model.MetricStatisticsResult {
	modelDataPoints := make([]*model.MetricStatisticsResult, 0, len(cwDataPoints))

	for _, cwDatapoint := range cwDataPoints {
		extendedStats := make(map[string]*float64, len(cwDatapoint.ExtendedStatistics))
		for name, value := range cwDatapoint.ExtendedStatistics {
			extendedStats[name] = &value
		}
		modelDataPoints = append(modelDataPoints, &model.MetricStatisticsResult{
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
