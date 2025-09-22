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
package promutil

import (
	"fmt"
	"log/slog"
	"maps"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/grafana/regexp"
	prom_model "github.com/prometheus/common/model"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var Percentile = regexp.MustCompile(`^p(\d{1,2}(\.\d{0,2})?|100)$`)

func BuildMetricName(namespace, metricName, statistic string) string {
	sb := strings.Builder{}

	// Some namespaces have a leading forward slash like
	// /aws/sagemaker/TrainingJobs, which should be removed.
	var promNs string
	if strings.HasPrefix(namespace, "/") {
		promNs = PromString(strings.ToLower(namespace[1:]))
	} else {
		promNs = PromString(strings.ToLower(namespace))
	}

	if !strings.HasPrefix(promNs, "aws") {
		sb.WriteString("aws_")
	}
	sb.WriteString(promNs)

	sb.WriteString("_")

	promMetricName := PromString(metricName)
	// Some metric names duplicate parts of the namespace as a prefix,
	// For example, the `Glue` namespace metrics have names prefixed also by `glue``
	skip := 0
	for _, part := range strings.Split(promNs, "_") {
		if strings.HasPrefix(promMetricName[skip:], part) {
			skip = len(part)
		}
	}
	promMetricName = strings.TrimPrefix(promMetricName[skip:], "_")

	sb.WriteString(promMetricName)
	if statistic != "" {
		sb.WriteString("_")
		PromStringToBuilder(statistic, &sb)
	}
	return sb.String()
}

func BuildNamespaceInfoMetrics(tagData []model.TaggedResourceResult, metrics []*PrometheusMetric, observedMetricLabels map[string]model.LabelSet, labelsSnakeCase bool, logger *slog.Logger) ([]*PrometheusMetric, map[string]model.LabelSet) {
	for _, tagResult := range tagData {
		contextLabels := contextToLabels(tagResult.Context, labelsSnakeCase, logger)
		for _, d := range tagResult.Data {
			metricName := BuildMetricName(d.Namespace, "info", "")

			promLabels := make(map[string]string, len(d.Tags)+len(contextLabels)+1)
			maps.Copy(promLabels, contextLabels)
			promLabels["name"] = d.ARN
			for _, tag := range d.Tags {
				ok, promTag := PromStringTag(tag.Key, labelsSnakeCase)
				if !ok {
					logger.Warn("tag name is an invalid prometheus label name", "tag", tag.Key)
					continue
				}

				labelName := "tag_" + promTag
				promLabels[labelName] = tag.Value
			}

			observedMetricLabels = recordLabelsForMetric(metricName, promLabels, observedMetricLabels)
			metrics = append(metrics, &PrometheusMetric{
				Name:   metricName,
				Labels: promLabels,
				Value:  0,
			})
		}
	}

	return metrics, observedMetricLabels
}

func BuildMetrics(results []model.CloudwatchMetricResult, labelsSnakeCase bool, logger *slog.Logger) ([]*PrometheusMetric, map[string]model.LabelSet, error) {
	output := make([]*PrometheusMetric, 0)
	observedMetricLabels := make(map[string]model.LabelSet)

	for _, result := range results {
		contextLabels := contextToLabels(result.Context, labelsSnakeCase, logger)
		for _, metric := range result.Data {
			// This should not be possible but check just in case
			if metric.GetMetricStatisticsResult == nil && metric.GetMetricDataResult == nil {
				logger.Warn("Attempted to migrate metric with no result", "namespace", metric.Namespace, "metric_name", metric.MetricName, "resource_name", metric.ResourceName)
			}

			for _, statistic := range statisticsInCloudwatchData(metric) {
				dataPoints, err := getDataPoints(metric, statistic)
				for _, dataPoint := range dataPoints {
					ts := dataPoint.Timestamp
					dataPoint := dataPoint.Value
					if err != nil {
						return nil, nil, err
					}
					var exportedDatapoint float64
					if dataPoint == nil && metric.MetricMigrationParams.AddCloudwatchTimestamp {
						// If we did not get a datapoint then the timestamp is a default value making it unusable in the
						// exported metric. Attempting to put a fake timestamp on the metric will likely conflict with
						// future CloudWatch timestamps which are always in the past.
						if metric.MetricMigrationParams.ExportAllDataPoints {
							// If we're exporting all data points, we can skip this one and check for a historical datapoint
							continue
						}
						// If we are not exporting all data points, we better have nothing exported
						break
					}
					if dataPoint == nil {
						exportedDatapoint = math.NaN()
					} else {
						exportedDatapoint = *dataPoint
					}

					if metric.MetricMigrationParams.NilToZero && math.IsNaN(exportedDatapoint) {
						exportedDatapoint = 0
					}

					name := BuildMetricName(metric.Namespace, metric.MetricName, statistic)

					promLabels := createPrometheusLabels(metric, labelsSnakeCase, contextLabels, logger)
					observedMetricLabels = recordLabelsForMetric(name, promLabels, observedMetricLabels)

					if !metric.MetricMigrationParams.AddCloudwatchTimestamp {
						// if we're not adding the original timestamp, we have to zero it so we can validate the data in the exporter via EnsureLabelConsistencyAndRemoveDuplicates
						ts = time.Time{}
					}

					output = append(output, &PrometheusMetric{
						Name:             name,
						Labels:           promLabels,
						Value:            exportedDatapoint,
						Timestamp:        ts,
						IncludeTimestamp: metric.MetricMigrationParams.AddCloudwatchTimestamp,
					})

					if !metric.MetricMigrationParams.ExportAllDataPoints {
						// If we're not exporting all data points, we can skip the rest of the data points for this metric
						break
					}
				}
			}
		}
	}

	return output, observedMetricLabels, nil
}

func statisticsInCloudwatchData(d *model.CloudwatchData) []string {
	if d.GetMetricDataResult != nil {
		return []string{d.GetMetricDataResult.Statistic}
	}
	if d.GetMetricStatisticsResult != nil {
		return d.GetMetricStatisticsResult.Statistics
	}
	return []string{}
}

func getDataPoints(cwd *model.CloudwatchData, statistic string) ([]model.DataPoint, error) {
	// Not possible but for sanity
	if cwd.GetMetricStatisticsResult == nil && cwd.GetMetricDataResult == nil {
		return nil, fmt.Errorf("cannot map a data point with no results on %s", cwd.MetricName)
	}

	if cwd.GetMetricDataResult != nil {
		// If we have no dataPoints, we should return a single nil datapoint, which is then either dropped or converted to 0
		if len(cwd.GetMetricDataResult.DataPoints) == 0 && !cwd.MetricMigrationParams.AddCloudwatchTimestamp {
			return []model.DataPoint{{
				Value:     nil,
				Timestamp: time.Time{},
			}}, nil
		}

		return cwd.GetMetricDataResult.DataPoints, nil
	}

	var averageDataPoints []*model.MetricStatisticsResult

	// sorting by timestamps so we can consistently export the most updated datapoint
	// assuming Timestamp field in cloudwatch.Value struct is never nil
	for _, datapoint := range sortByTimestamp(cwd.GetMetricStatisticsResult.Results) {
		switch {
		case statistic == "Maximum":
			if datapoint.Maximum != nil {
				return []model.DataPoint{{Value: datapoint.Maximum, Timestamp: *datapoint.Timestamp}}, nil
			}
		case statistic == "Minimum":
			if datapoint.Minimum != nil {
				return []model.DataPoint{{Value: datapoint.Minimum, Timestamp: *datapoint.Timestamp}}, nil
			}
		case statistic == "Sum":
			if datapoint.Sum != nil {
				return []model.DataPoint{{Value: datapoint.Sum, Timestamp: *datapoint.Timestamp}}, nil
			}
		case statistic == "SampleCount":
			if datapoint.SampleCount != nil {
				return []model.DataPoint{{Value: datapoint.SampleCount, Timestamp: *datapoint.Timestamp}}, nil
			}
		case statistic == "Average":
			if datapoint.Average != nil {
				averageDataPoints = append(averageDataPoints, datapoint)
			}
		case Percentile.MatchString(statistic):
			if data, ok := datapoint.ExtendedStatistics[statistic]; ok {
				return []model.DataPoint{{Value: data, Timestamp: *datapoint.Timestamp}}, nil
			}
		default:
			return nil, fmt.Errorf("invalid statistic requested on metric %s: %s", cwd.MetricName, statistic)
		}
	}

	if len(averageDataPoints) > 0 {
		var total float64
		var timestamp time.Time

		for _, p := range averageDataPoints {
			if p.Timestamp.After(timestamp) {
				timestamp = *p.Timestamp
			}
			total += *p.Average
		}
		average := total / float64(len(averageDataPoints))
		return []model.DataPoint{{Value: &average, Timestamp: timestamp}}, nil
	}
	return nil, nil
}

func sortByTimestamp(dataPoints []*model.MetricStatisticsResult) []*model.MetricStatisticsResult {
	sort.Slice(dataPoints, func(i, j int) bool {
		jTimestamp := *dataPoints[j].Timestamp
		return dataPoints[i].Timestamp.After(jTimestamp)
	})
	return dataPoints
}

func createPrometheusLabels(cwd *model.CloudwatchData, labelsSnakeCase bool, contextLabels map[string]string, logger *slog.Logger) map[string]string {
	labels := make(map[string]string, len(cwd.Dimensions)+len(cwd.Tags)+len(contextLabels))
	labels["name"] = cwd.ResourceName

	// Inject the sfn name back as a label
	for _, dimension := range cwd.Dimensions {
		ok, promTag := PromStringTag(dimension.Name, labelsSnakeCase)
		if !ok {
			logger.Warn("dimension name is an invalid prometheus label name", "dimension", dimension.Name)
			continue
		}
		labels["dimension_"+promTag] = dimension.Value
	}

	for _, tag := range cwd.Tags {
		ok, promTag := PromStringTag(tag.Key, labelsSnakeCase)
		if !ok {
			logger.Warn("metric tag name is an invalid prometheus label name", "tag", tag.Key)
			continue
		}
		labels["tag_"+promTag] = tag.Value
	}

	maps.Copy(labels, contextLabels)

	return labels
}

func contextToLabels(context *model.ScrapeContext, labelsSnakeCase bool, logger *slog.Logger) map[string]string {
	if context == nil {
		return map[string]string{}
	}

	labels := make(map[string]string, 2+len(context.CustomTags))
	labels["region"] = context.Region
	labels["account_id"] = context.AccountID
	// If there's no account alias, omit adding an extra label in the series, it will work either way query wise
	if context.AccountAlias != "" {
		labels["account_alias"] = context.AccountAlias
	}

	for _, label := range context.CustomTags {
		ok, promTag := PromStringTag(label.Key, labelsSnakeCase)
		if !ok {
			logger.Warn("custom tag name is an invalid prometheus label name", "tag", label.Key)
			continue
		}
		labels["custom_tag_"+promTag] = label.Value
	}

	return labels
}

// recordLabelsForMetric adds any missing labels from promLabels in to the LabelSet for the metric name and returns
// the updated observedMetricLabels
func recordLabelsForMetric(metricName string, promLabels map[string]string, observedMetricLabels map[string]model.LabelSet) map[string]model.LabelSet {
	if _, ok := observedMetricLabels[metricName]; !ok {
		observedMetricLabels[metricName] = make(model.LabelSet, len(promLabels))
	}
	for label := range promLabels {
		if _, ok := observedMetricLabels[metricName][label]; !ok {
			observedMetricLabels[metricName][label] = struct{}{}
		}
	}

	return observedMetricLabels
}

// EnsureLabelConsistencyAndRemoveDuplicates ensures that every metric has the same set of labels based on the data
// in observedMetricLabels and that there are no duplicate metrics.
// Prometheus requires that all metrics with the same name have the same set of labels and that no duplicates are registered
func EnsureLabelConsistencyAndRemoveDuplicates(metrics []*PrometheusMetric, observedMetricLabels map[string]model.LabelSet) []*PrometheusMetric {
	metricKeys := make(map[string]struct{}, len(metrics))
	output := make([]*PrometheusMetric, 0, len(metrics))

	for _, metric := range metrics {
		for observedLabels := range observedMetricLabels[metric.Name] {
			if _, ok := metric.Labels[observedLabels]; !ok {
				metric.Labels[observedLabels] = ""
			}
		}

		// We are including the timestamp in the metric key to ensure that we don't have duplicate metrics
		// if we have AddCloudwatchTimestamp enabled its the real timestamp, otherwise its a zero value
		// the timestamp is needed to ensure valid date created by ExportAllDataPoints
		metricKey := fmt.Sprintf("%s-%d-%d", metric.Name, prom_model.LabelsToSignature(metric.Labels), metric.Timestamp.Unix())
		if _, exists := metricKeys[metricKey]; !exists {
			metricKeys[metricKey] = struct{}{}
			output = append(output, metric)
		} else {
			DuplicateMetricsFilteredCounter.Inc()
		}
	}

	return output
}
