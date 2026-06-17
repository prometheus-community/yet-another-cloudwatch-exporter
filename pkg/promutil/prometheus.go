// Copyright The Prometheus Authors
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
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/model"
	"golang.org/x/exp/maps"
)

// NOTE: these should be removed once exporter.Metrics is removed.
var (
	deprecatedRegistry      = prometheus.NewRegistry()
	deprecatedScrapeMetrics = NewScrapeMetrics(deprecatedRegistry)
)

// Deprecated: new code should call NewScrapeMetrics with its own registry.
func DeprecatedScrapeMetrics() *ScrapeMetrics { return deprecatedScrapeMetrics }

var (
	// NoopRegisterer is a registry which discards every registration.
	NoopRegisterer prometheus.Registerer = noopRegisterer{}
	Discard                              = NewScrapeMetrics(NoopRegisterer)
)

type noopRegisterer struct{}

func (noopRegisterer) Register(prometheus.Collector) error  { return nil }
func (noopRegisterer) MustRegister(...prometheus.Collector) {}
func (noopRegisterer) Unregister(prometheus.Collector) bool { return true }

type ScrapeMetrics struct {
	CloudwatchAPIErrorCounter                CounterVec // labels: api_name
	CloudwatchAPICounter                     CounterVec // labels: api_name
	CloudwatchGetMetricDataAPICounter        Counter
	CloudwatchGetMetricDataAPIMetricsCounter Counter
	CloudwatchGetMetricStatisticsAPICounter  Counter
	ResourceGroupTaggingAPICounter           Counter
	AutoScalingAPICounter                    Counter
	TargetGroupsAPICounter                   Counter
	APIGatewayAPICounter                     Counter
	APIGatewayAPIV2Counter                   Counter
	Ec2APICounter                            Counter
	ShieldAPICounter                         Counter
	ManagedPrometheusAPICounter              Counter
	StoragegatewayAPICounter                 Counter
	DmsAPICounter                            Counter
	DuplicateMetricsFilteredCounter          Counter
}

func NewScrapeMetrics(r prometheus.Registerer) *ScrapeMetrics {
	if r == nil {
		r = NoopRegisterer
	}
	f := promauto.With(r)

	return &ScrapeMetrics{
		CloudwatchAPIErrorCounter: CounterVec{inner: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yace_cloudwatch_request_errors",
			Help: "Help is not implemented yet.",
		}, []string{"api_name"})},
		CloudwatchAPICounter: CounterVec{inner: f.NewCounterVec(prometheus.CounterOpts{
			Name: "yace_cloudwatch_requests_total",
			Help: "Number of calls made to the CloudWatch APIs",
		}, []string{"api_name"})},
		CloudwatchGetMetricDataAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_getmetricdata_requests_total",
			Help: "DEPRECATED: replaced by yace_cloudwatch_requests_total with api_name label",
		})},
		CloudwatchGetMetricDataAPIMetricsCounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_getmetricdata_metrics_requested_total",
			Help: "Number of metrics requested from the CloudWatch GetMetricData API which is how AWS bills",
		})},
		CloudwatchGetMetricStatisticsAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_getmetricstatistics_requests_total",
			Help: "DEPRECATED: replaced by yace_cloudwatch_requests_total with api_name label",
		})},
		ResourceGroupTaggingAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_resourcegrouptaggingapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		AutoScalingAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_autoscalingapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		TargetGroupsAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_targetgroupapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		APIGatewayAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_apigatewayapi_requests_total",
		})},
		APIGatewayAPIV2Counter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_apigatewayapiv2_requests_total",
		})},
		Ec2APICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_ec2api_requests_total",
			Help: "Help is not implemented yet.",
		})},
		ShieldAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_shieldapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		ManagedPrometheusAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_managedprometheusapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		StoragegatewayAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_storagegatewayapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		DmsAPICounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_dmsapi_requests_total",
			Help: "Help is not implemented yet.",
		})},
		DuplicateMetricsFilteredCounter: Counter{inner: f.NewCounter(prometheus.CounterOpts{
			Name: "yace_cloudwatch_duplicate_metrics_filtered",
			Help: "Help is not implemented yet.",
		})},
	}
}

// Collectors returns the underlying prometheus collectors for direct
// registration on a caller-supplied registry. Returns nil for a nil receiver
// or zero-value ScrapeMetrics.
func (m *ScrapeMetrics) Collectors() []prometheus.Collector {
	if m == nil {
		return nil
	}
	vecs := []CounterVec{
		m.CloudwatchAPIErrorCounter,
		m.CloudwatchAPICounter,
	}
	counters := []Counter{
		m.CloudwatchGetMetricDataAPICounter,
		m.CloudwatchGetMetricDataAPIMetricsCounter,
		m.CloudwatchGetMetricStatisticsAPICounter,
		m.ResourceGroupTaggingAPICounter,
		m.AutoScalingAPICounter,
		m.TargetGroupsAPICounter,
		m.APIGatewayAPICounter,
		m.APIGatewayAPIV2Counter,
		m.Ec2APICounter,
		m.ShieldAPICounter,
		m.ManagedPrometheusAPICounter,
		m.StoragegatewayAPICounter,
		m.DmsAPICounter,
		m.DuplicateMetricsFilteredCounter,
	}
	out := make([]prometheus.Collector, 0, len(vecs)+len(counters))
	for _, c := range vecs {
		if c.inner != nil {
			out = append(out, c.inner)
		}
	}
	for _, c := range counters {
		if c.inner != nil {
			out = append(out, c.inner)
		}
	}
	return out
}

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
	var buf strings.Builder
	PromStringToBuilder(text, &buf)
	return buf.String()
}

func PromStringToBuilder(text string, buf *strings.Builder) {
	buf.Grow(len(text))

	var prev rune
	for _, c := range text {
		switch c {
		case ' ', ',', '\t', '/', '\\', '.', '-', ':', '=', '@', '<', '>', '(', ')', '“':
			buf.WriteRune('_')
		case '%':
			buf.WriteString("_percent")
		default:
			if unicode.IsUpper(c) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
				buf.WriteRune('_')
			}
			buf.WriteRune(unicode.ToLower(c))
		}
		prev = c
	}
}

func PromStringTag(text string, labelsSnakeCase bool) (bool, string) {
	var s string
	if labelsSnakeCase {
		s = PromString(text)
	} else {
		s = sanitize(text)
	}
	return model.LabelName(s).IsValid(), s //nolint:staticcheck
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

// Counter wraps a prometheus.Counter so Inc and Add are no-ops when inner is nil.
type Counter struct {
	inner prometheus.Counter
}

func (c Counter) Inc() {
	if c.inner != nil {
		c.inner.Inc()
	}
}

func (c Counter) Add(v float64) {
	if c.inner != nil {
		c.inner.Add(v)
	}
}

func (c Counter) Raw() prometheus.Counter { return c.inner }

// CounterVec wraps a *prometheus.CounterVec so Inc and Add are no-ops when inner is nil.
type CounterVec struct {
	inner *prometheus.CounterVec
}

func (c CounterVec) Inc(labels ...string) {
	if c.inner != nil {
		c.inner.WithLabelValues(labels...).Inc()
	}
}

func (c CounterVec) Add(v float64, labels ...string) {
	if c.inner != nil {
		c.inner.WithLabelValues(labels...).Add(v)
	}
}

func (c CounterVec) Raw() *prometheus.CounterVec { return c.inner }
