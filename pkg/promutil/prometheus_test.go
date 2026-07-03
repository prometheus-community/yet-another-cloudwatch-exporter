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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitize(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{
		{
			input:  "Global.Topic.Count",
			output: "Global_Topic_Count",
		},
		{
			input:  "Status.Check.Failed_Instance",
			output: "Status_Check_Failed_Instance",
		},
		{
			input:  "IHaveA%Sign",
			output: "IHaveA_percentSign",
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.output, sanitize(tc.input))
	}
}

func TestPromStringTag(t *testing.T) {
	originalValidationScheme := model.NameValidationScheme //nolint:staticcheck
	model.NameValidationScheme = model.LegacyValidation    //nolint:staticcheck
	defer func() {
		model.NameValidationScheme = originalValidationScheme //nolint:staticcheck
	}()

	testCases := []struct {
		name        string
		label       string
		toSnakeCase bool
		ok          bool
		out         string
	}{
		{
			name:        "valid",
			label:       "labelName",
			toSnakeCase: false,
			ok:          true,
			out:         "labelName",
		},
		{
			name:        "valid, convert to snake case",
			label:       "labelName",
			toSnakeCase: true,
			ok:          true,
			out:         "label_name",
		},
		{
			name:        "valid (snake case)",
			label:       "label_name",
			toSnakeCase: false,
			ok:          true,
			out:         "label_name",
		},
		{
			name:        "valid (snake case) unchanged",
			label:       "label_name",
			toSnakeCase: true,
			ok:          true,
			out:         "label_name",
		},
		{
			name:        "invalid chars",
			label:       "invalidChars@$",
			toSnakeCase: false,
			ok:          false,
			out:         "",
		},
		{
			name:        "invalid chars, convert to snake case",
			label:       "invalidChars@$",
			toSnakeCase: true,
			ok:          false,
			out:         "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok, out := PromStringTag(tc.label, tc.toSnakeCase)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.out, out)
			}
		})
	}
}

// TestPromStringTag_IgnoresGlobalValidationScheme guards against regressing to
// reading the process-global model.NameValidationScheme. PromStringTag must
// apply legacy label-name rules unconditionally, even when the global is set to
// UTF8Validation (the prometheus/common default), so that embedding YACE never
// depends on, nor needs to mutate, that global.
func TestPromStringTag_IgnoresGlobalValidationScheme(t *testing.T) {
	originalValidationScheme := model.NameValidationScheme //nolint:staticcheck
	model.NameValidationScheme = model.UTF8Validation      //nolint:staticcheck
	defer func() {
		model.NameValidationScheme = originalValidationScheme //nolint:staticcheck
	}()

	// "1stLabel" is a valid UTF-8 label name but invalid under legacy rules
	// (leading digit). It must be rejected regardless of the global scheme.
	ok, _ := PromStringTag("1stLabel", false)
	assert.False(t, ok, "leading-digit label must be rejected under legacy rules even when the global scheme is UTF8")

	// A legacy-valid label is still accepted.
	ok, out := PromStringTag("labelName", false)
	assert.True(t, ok)
	assert.Equal(t, "labelName", out)
}

func TestNewPrometheusCollector_CanReportMetricsAndErrors(t *testing.T) {
	originalValidationScheme := model.NameValidationScheme //nolint:staticcheck
	model.NameValidationScheme = model.LegacyValidation    //nolint:staticcheck
	defer func() {
		model.NameValidationScheme = originalValidationScheme //nolint:staticcheck
	}()

	metrics := []*PrometheusMetric{
		{
			Name:             "this*is*not*valid",
			Labels:           map[string]string{},
			Value:            0,
			IncludeTimestamp: false,
		},
		{
			Name:             "this_is_valid",
			Labels:           map[string]string{"key": "value1"},
			Value:            0,
			IncludeTimestamp: false,
		},
	}
	collector := NewPrometheusCollector(metrics)
	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(collector))
	families, err := registry.Gather()
	assert.Error(t, err)
	assert.Len(t, families, 1)
	family := families[0]
	assert.Equal(t, "this_is_valid", family.GetName())
}

func TestNewPrometheusCollector_CanReportMetrics(t *testing.T) {
	ts := time.Now()

	labelSet1 := map[string]string{"key1": "value", "key2": "value", "key3": "value"}
	labelSet2 := map[string]string{"key2": "out", "key3": "of", "key1": "order"}
	labelSet3 := map[string]string{"key2": "out", "key1": "of", "key3": "order"}
	metrics := []*PrometheusMetric{
		{
			Name:             "metric_with_labels",
			Labels:           labelSet1,
			Value:            1,
			IncludeTimestamp: false,
		},
		{
			Name:             "metric_with_labels",
			Labels:           labelSet2,
			Value:            2,
			IncludeTimestamp: false,
		},
		{
			Name:             "metric_with_labels",
			Labels:           labelSet3,
			Value:            3,
			IncludeTimestamp: false,
		},
		{
			Name:             "metric_with_timestamp",
			Labels:           map[string]string{},
			Value:            1,
			IncludeTimestamp: true,
			Timestamp:        ts,
		},
	}

	collector := NewPrometheusCollector(metrics)
	registry := prometheus.NewRegistry()
	require.NoError(t, registry.Register(collector))
	families, err := registry.Gather()
	assert.NoError(t, err)
	assert.Len(t, families, 2)

	var metricWithLabels *dto.MetricFamily
	var metricWithTs *dto.MetricFamily

	for _, metricFamily := range families {
		assert.Equal(t, dto.MetricType_GAUGE, metricFamily.GetType())

		switch {
		case metricFamily.GetName() == "metric_with_labels":
			metricWithLabels = metricFamily
		case metricFamily.GetName() == "metric_with_timestamp":
			metricWithTs = metricFamily
		default:
			require.Failf(t, "Encountered an unexpected metric family %s", metricFamily.GetName())
		}
	}
	require.NotNil(t, metricWithLabels)
	require.NotNil(t, metricWithTs)

	assert.Len(t, metricWithLabels.Metric, 3)
	for _, metric := range metricWithLabels.Metric {
		assert.Len(t, metric.Label, 3)
		var labelSetToMatch map[string]string
		switch *metric.Gauge.Value {
		case 1.0:
			labelSetToMatch = labelSet1
		case 2.0:
			labelSetToMatch = labelSet2
		case 3.0:
			labelSetToMatch = labelSet3
		default:
			require.Fail(t, "Encountered an metric value value %v", *metric.Gauge.Value)
		}

		for _, labelPairs := range metric.Label {
			require.Contains(t, labelSetToMatch, *labelPairs.Name)
			require.Equal(t, labelSetToMatch[*labelPairs.Name], *labelPairs.Value)
		}
	}

	require.Len(t, metricWithTs.Metric, 1)
	tsMetric := metricWithTs.Metric[0]
	assert.Equal(t, ts.UnixMilli(), *tsMetric.TimestampMs)
	assert.Equal(t, 1.0, *tsMetric.Gauge.Value)
}

func TestNewScrapeMetrics_DiscardingRegisterers(t *testing.T) {
	for _, tc := range []struct {
		name string
		reg  prometheus.Registerer
	}{
		{"nil registerer", nil},
		{"NoopRegisterer", NoopRegisterer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := NewScrapeMetrics(tc.reg)
			require.NotNil(t, sm)
			require.NotPanics(t, func() { exerciseCounters(sm) })

			// A fresh registry the caller might construct sees no yace_* families.
			reg := prometheus.NewRegistry()
			families, err := reg.Gather()
			require.NoError(t, err)
			require.Empty(t, families)
		})
	}
}

func TestNewScrapeMetrics_RealRegisterer(t *testing.T) {
	reg := prometheus.NewRegistry()
	sm := NewScrapeMetrics(reg)

	sm.CloudwatchAPICounter.Inc("ListMetrics")
	sm.CloudwatchAPICounter.Inc("ListMetrics")
	sm.DuplicateMetricsFilteredCounter.Inc()

	require.Equal(t, float64(2), readCounterValue(t, sm.CloudwatchAPICounter.Raw().WithLabelValues("ListMetrics")))
	require.Equal(t, float64(1), readCounterValue(t, sm.DuplicateMetricsFilteredCounter.Raw()))

	families, err := reg.Gather()
	require.NoError(t, err)
	require.NotEmpty(t, families)
}

func TestScrapeMetrics_ZeroValueAndNilReceiver(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var sm ScrapeMetrics
		require.NotPanics(t, func() { exerciseCounters(&sm) })
		require.Empty(t, sm.Collectors())
	})
	t.Run("nil receiver", func(t *testing.T) {
		var sm *ScrapeMetrics
		require.Nil(t, sm.Collectors())
	})
}

func exerciseCounters(sm *ScrapeMetrics) {
	sm.CloudwatchAPICounter.Inc("ListMetrics")
	sm.CloudwatchAPICounter.Add(2, "GetMetricData")
	sm.DuplicateMetricsFilteredCounter.Inc()
	sm.CloudwatchGetMetricDataAPIMetricsCounter.Add(42)
}

func readCounterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	require.NotNil(t, c)
	var m dto.Metric
	require.NoError(t, c.(prometheus.Metric).Write(&m))
	return m.GetCounter().GetValue()
}
