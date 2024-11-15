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
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

func toCloudWatchDimensions(dimensions []model.Dimension) []types.Dimension {
	cwDim := make([]types.Dimension, 0, len(dimensions))
	for _, dim := range dimensions {
		// Don't take pointers directly to loop variables
		cDim := dim
		cwDim = append(cwDim, types.Dimension{
			Name:  &cDim.Name,
			Value: &cDim.Value,
		})
	}
	return cwDim
}

func createGetMetricStatisticsInput(logger logging.Logger, dimensions []model.Dimension, namespace *string, metric *model.MetricConfig) *cloudwatch.GetMetricStatisticsInput {
	period := metric.Period
	length := metric.Length
	delay := metric.Delay
	endTime := time.Now().Add(-time.Duration(delay) * time.Second)
	startTime := time.Now().Add(-(time.Duration(length) + time.Duration(delay)) * time.Second)

	var statistics []types.Statistic
	var extendedStatistics []string
	for _, statistic := range metric.Statistics {
		if promutil.Percentile.MatchString(statistic) {
			extendedStatistics = append(extendedStatistics, statistic)
		} else {
			statistics = append(statistics, types.Statistic(statistic))
		}
	}

	output := &cloudwatch.GetMetricStatisticsInput{
		Dimensions:         toCloudWatchDimensions(dimensions),
		Namespace:          namespace,
		StartTime:          &startTime,
		EndTime:            &endTime,
		Period:             aws.Int32(int32(period)),
		MetricName:         &metric.Name,
		Statistics:         statistics,
		ExtendedStatistics: extendedStatistics,
	}

	if logger.IsDebugEnabled() {
		logger.Debug("CLI helper - " +
			"aws cloudwatch get-metric-statistics" +
			" --metric-name " + metric.Name +
			" --dimensions " + dimensionsToCliString(dimensions) +
			" --namespace " + *namespace +
			" --statistics " + string(statistics[0]) +
			" --period " + strconv.FormatInt(period, 10) +
			" --start-time " + startTime.Format(time.RFC3339) +
			" --end-time " + endTime.Format(time.RFC3339))

		logger.Debug("createGetMetricStatisticsInput", "output", *output)
	}

	return output
}

func dimensionsToCliString(dimensions []model.Dimension) string {
	out := strings.Builder{}
	for _, dim := range dimensions {
		out.WriteString("Name=")
		out.WriteString(dim.Name)
		out.WriteString(",Value=")
		out.WriteString(dim.Value)
		out.WriteString(" ")
	}
	return out.String()
}
