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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"golang.org/x/exp/maps"
)

var (
	CloudwatchAPIErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "yace_cloudwatch_request_errors",
		Help: "Help is not implemented yet.",
	}, []string{"api_name"})
	CloudwatchAPICounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "yace_cloudwatch_requests_total",
		Help: "Number of calls made to the CloudWatch APIs",
	}, []string{"api_name"})
	CloudwatchGetMetricDataAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_getmetricdata_requests_total",
		Help: "DEPRECATED: replaced by yace_cloudwatch_requests_total with api_name label",
	})
	CloudwatchGetMetricDataAPIMetricsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_getmetricdata_metrics_requested_total",
		Help: "Number of metrics requested from the CloudWatch GetMetricData API which is how AWS bills",
	})
	CloudwatchGetMetricStatisticsAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_getmetricstatistics_requests_total",
		Help: "DEPRECATED: replaced by yace_cloudwatch_requests_total with api_name label",
	})
	ResourceGroupTaggingAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_resourcegrouptaggingapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	AutoScalingAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_autoscalingapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	TargetGroupsAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_targetgroupapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	APIGatewayAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_apigatewayapi_requests_total",
	})
	APIGatewayAPIV2Counter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_apigatewayapiv2_requests_total",
	})
	Ec2APICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_ec2api_requests_total",
		Help: "Help is not implemented yet.",
	})
	ShieldAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_shieldapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	ManagedPrometheusAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_managedprometheusapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	StoragegatewayAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_storagegatewayapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	DmsAPICounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_dmsapi_requests_total",
		Help: "Help is not implemented yet.",
	})
	DuplicateMetricsFilteredCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "yace_cloudwatch_duplicate_metrics_filtered",
		Help: "Help is not implemented yet.",
	})
)

var replacer = strings.NewReplacer(
	" ", "_",
	",", "_",
	"\t", "_",
	"/", "_",
	"\\", "_",
	".", "_",
	"-", "_",
	":", "_",
	"=", "_",
	"“", "_",
	"@", "_",
	"<", "_",
	">", "_",
	"(", "_",
	")", "_",
	"%", "_percent",
)

type PrometheusMetric struct {
	Name             string
	Labels           map[string]string
	Value            float64
	IncludeTimestamp bool
	Timestamp        time.Time
}

type PrometheusCollector struct {
	metrics []prometheus.Metric
}

func NewPrometheusCollector(metrics []*PrometheusMetric) *PrometheusCollector {
	return &PrometheusCollector{
		metrics: toConstMetrics(metrics),
	}
}

func (p *PrometheusCollector) Describe(_ chan<- *prometheus.Desc) {
	// The exporter produces a dynamic set of metrics and the docs for prometheus.Collector Describe say
	// 	Sending no descriptor at all marks the Collector as “unchecked”,
	// 	i.e. no checks will be performed at registration time, and the
	// 	Collector may yield any Metric it sees fit in its Collect method.
	// Based on our use an "unchecked" collector is perfectly fine
}

func (p *PrometheusCollector) Collect(metrics chan<- prometheus.Metric) {
	for _, metric := range p.metrics {
		metrics <- metric
	}
}

func toConstMetrics(metrics []*PrometheusMetric) []prometheus.Metric {
	// We keep two fast lookup maps here one for the prometheus.Desc of a metric which can be reused for each metric with
	// the same name and the expected label key order of a particular metric name.
	// The prometheus.Desc object is expensive to create and being able to reuse it for all metrics with the same name
	// results in large performance gain. We use the other map because metrics created using the Desc only provide label
	// values and they must be provided in the exact same order as registered in the Desc.
	metricToDesc := map[string]*prometheus.Desc{}
	metricToExpectedLabelOrder := map[string][]string{}

	result := make([]prometheus.Metric, 0, len(metrics))
	for _, metric := range metrics {
		metricName := metric.Name
		if _, ok := metricToDesc[metricName]; !ok {
			labelKeys := maps.Keys(metric.Labels)
			metricToDesc[metricName] = prometheus.NewDesc(metricName, "Help is not implemented yet.", labelKeys, nil)
			metricToExpectedLabelOrder[metricName] = labelKeys
		}
		metricsDesc := metricToDesc[metricName]

		// Create the label values using the label order of the Desc
		labelValues := make([]string, 0, len(metric.Labels))
		for _, labelKey := range metricToExpectedLabelOrder[metricName] {
			labelValues = append(labelValues, metric.Labels[labelKey])
		}

		promMetric, err := prometheus.NewConstMetric(metricsDesc, prometheus.GaugeValue, metric.Value, labelValues...)
		if err != nil {
			// If for whatever reason the metric or metricsDesc is considered invalid this will ensure the error is
			// reported through the collector
			promMetric = prometheus.NewInvalidMetric(metricsDesc, err)
		} else if metric.IncludeTimestamp {
			promMetric = prometheus.NewMetricWithTimestamp(metric.Timestamp, promMetric)
		}

		result = append(result, promMetric)
	}

	return result
}

func PromString(text string) string {
	text = splitString(text)
	return strings.ToLower(sanitize(text))
}

func PromStringTag(text string, labelsSnakeCase bool) (bool, string) {
	var s string
	if labelsSnakeCase {
		s = PromString(text)
	} else {
		s = sanitize(text)
	}
	return model.LabelName(s).IsValid(), s
}

// sanitize replaces some invalid chars with an underscore
func sanitize(text string) string {
	if strings.ContainsAny(text, "“%") {
		// fallback to the replacer for complex cases:
		// - '“' is non-ascii rune
		// - '%' is replaced with a whole string
		return replacer.Replace(text)
	}

	b := []byte(text)
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case ' ', ',', '\t', '/', '\\', '.', '-', ':', '=', '@', '<', '>', '(', ')':
			b[i] = '_'
		}
	}
	return string(b)
}

// splitString replaces consecutive occurrences of a lowercase and uppercase letter,
// or a number and an upper case letter, by putting a dot between the two chars.
//
// This is an optimised version of the original implementation:
//
//	  var splitRegexp = regexp.MustCompile(`([a-z0-9])([A-Z])`)
//
//		func splitString(text string) string {
//		  return splitRegexp.ReplaceAllString(text, `$1.$2`)
//		}
func splitString(text string) string {
	sb := strings.Builder{}
	sb.Grow(len(text) + 4) // make some room for replacements

	i := 0
	for i < len(text) {
		c := text[i]
		sb.WriteByte(c)
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			if i < (len(text) - 1) {
				c = text[i+1]
				if c >= 'A' && c <= 'Z' {
					sb.WriteByte('.')
					sb.WriteByte(c)
					i++
				}
			}
		}
		i++
	}
	return sb.String()
}
