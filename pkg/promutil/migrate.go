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
	"maps"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/grafana/regexp"
	prom_model "github.com/prometheus/common/model"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var Percentile = regexp.MustCompile(`^p(\d{1,2}(\.\d{0,2})?|100)$`)

func BuildMetricName(namespace, metricName, statistic string) string {
	sb := strings.Builder{}
	promNs := PromString(strings.ToLower(namespace))
	// Some namespaces have a leading forward slash like
	// /aws/sagemaker/TrainingJobs, which gets converted to
	// a leading _ by PromString().
	promNs = strings.TrimPrefix(promNs, "_")
	if !strings.HasPrefix(promNs, "aws") {
		sb.WriteString("aws_")
	}
	sb.WriteString(promNs)
	sb.WriteString("_")
	promMetricName := PromString(metricName)
	// Some metric names duplicate parts of the namespace as a prefix,
	// For example, the `Glue` namespace metrics have names prefixed also by `glue``
	for _, part := range strings.Split(promNs, "_") {
		promMetricName = strings.TrimPrefix(promMetricName, part)
	}
	promMetricName = strings.TrimPrefix(promMetricName, "_")
	sb.WriteString(promMetricName)
	if statistic != "" {
		sb.WriteString("_")
		sb.WriteString(PromString(statistic))
	}
	return sb.String()
}

func BuildNamespaceInfoMetrics(tagData []model.TaggedResourceResult, metrics []*PrometheusMetric, observedMetricLabels map[string]model.LabelSet, labelsSnakeCase bool, logger logging.Logger) ([]*PrometheusMetric, map[string]model.LabelSet) {
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

func BuildMetrics(results []model.CloudwatchMetricResult, labelsSnakeCase bool, logger logging.Logger) ([]*PrometheusMetric, map[string]model.LabelSet, error) {
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
				dataPoint, ts, err := getDatapoint(metric, statistic)
				if err != nil {
					return nil, nil, err
				}
				var exportedDatapoint float64
				if dataPoint == nil && metric.MetricMigrationParams.AddCloudwatchTimestamp {
					// If we did not get a datapoint then the timestamp is a default value making it unusable in the
					// exported metric. Attempting to put a fake timestamp on the metric will likely conflict with
					// future CloudWatch timestamps which are always in the past. It's safer to skip here than guess
					continue
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

				output = append(output, &PrometheusMetric{
					Name:             name,
					Labels:           promLabels,
					Value:            exportedDatapoint,
					Timestamp:        ts,
					IncludeTimestamp: metric.MetricMigrationParams.AddCloudwatchTimestamp,
				})

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

func getDatapoint(cwd *model.CloudwatchData, statistic string) (*float64, time.Time, error) {
	// Not possible but for sanity
	if cwd.GetMetricStatisticsResult == nil && cwd.GetMetricDataResult == nil {
		return nil, time.Time{}, fmt.Errorf("cannot map a data point with no results on %s", cwd.MetricName)
	}

	if cwd.GetMetricDataResult != nil {
		return cwd.GetMetricDataResult.Datapoint, cwd.GetMetricDataResult.Timestamp, nil
	}

	var averageDataPoints []*model.Datapoint

	// sorting by timestamps so we can consistently export the most updated datapoint
	// assuming Timestamp field in cloudwatch.Datapoint struct is never nil
	for _, datapoint := range sortByTimestamp(cwd.GetMetricStatisticsResult.Datapoints) {
		switch {
		case statistic == "Maximum":
			if datapoint.Maximum != nil {
				return datapoint.Maximum, *datapoint.Timestamp, nil
			}
		case statistic == "Minimum":
			if datapoint.Minimum != nil {
				return datapoint.Minimum, *datapoint.Timestamp, nil
			}
		case statistic == "Sum":
			if datapoint.Sum != nil {
				return datapoint.Sum, *datapoint.Timestamp, nil
			}
		case statistic == "SampleCount":
			if datapoint.SampleCount != nil {
				return datapoint.SampleCount, *datapoint.Timestamp, nil
			}
		case statistic == "Average":
			if datapoint.Average != nil {
				averageDataPoints = append(averageDataPoints, datapoint)
			}
		case Percentile.MatchString(statistic):
			if data, ok := datapoint.ExtendedStatistics[statistic]; ok {
				return data, *datapoint.Timestamp, nil
			}
		default:
			return nil, time.Time{}, fmt.Errorf("invalid statistic requested on metric %s: %s", cwd.MetricName, statistic)
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
		return &average, timestamp, nil
	}
	return nil, time.Time{}, nil
}

func sortByTimestamp(datapoints []*model.Datapoint) []*model.Datapoint {
	sort.Slice(datapoints, func(i, j int) bool {
		jTimestamp := *datapoints[j].Timestamp
		return datapoints[i].Timestamp.After(jTimestamp)
	})
	return datapoints
}

func createPrometheusLabels(cwd *model.CloudwatchData, labelsSnakeCase bool, contextLabels map[string]string, logger logging.Logger) map[string]string {
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

func contextToLabels(context *model.ScrapeContext, labelsSnakeCase bool, logger logging.Logger) map[string]string {
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

		metricKey := fmt.Sprintf("%s-%d", metric.Name, prom_model.LabelsToSignature(metric.Labels))
		if _, exists := metricKeys[metricKey]; !exists {
			metricKeys[metricKey] = struct{}{}
			output = append(output, metric)
		} else {
			DuplicateMetricsFilteredCounter.Inc()
		}
	}

	return output
}
