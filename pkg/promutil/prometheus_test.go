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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitString(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{
		{
			input:  "GlobalTopicCount",
			output: "Global.Topic.Count",
		},
		{
			input:  "CPUUtilization",
			output: "CPUUtilization",
		},
		{
			input:  "StatusCheckFailed_Instance",
			output: "Status.Check.Failed_Instance",
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.output, splitString(tc.input))
	}
}

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

func TestNewPrometheusCollector_CanReportMetricsAndErrors(t *testing.T) {
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
