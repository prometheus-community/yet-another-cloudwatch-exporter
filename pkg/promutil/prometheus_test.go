package promutil

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
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

func Benchmark_CreateMetrics(b *testing.B) {
	tests := []struct {
		uniqueMetrics    int
		numberOfLabels   int
		metricsToMigrate int
	}{
		{10, 1, 1000},
		{10, 1, 10000},
		{10, 1, 100000},
		{10, 5, 1000},
		{10, 5, 10000},
		{10, 5, 100000},
		{10, 10, 1000},
		{10, 10, 10000},
		{10, 10, 100000},
		{10, 20, 1000},
		{10, 20, 10000},
		{10, 20, 100000},
		{20, 20, 1000},
		{20, 20, 10000},
		{20, 20, 100000},
	}

	for _, tc := range tests {
		metricsToMigrate := createTestData(tc.uniqueMetrics, tc.numberOfLabels, tc.metricsToMigrate)

		b.Run(fmt.Sprintf("current: unique metrics %d, number of labels %d, metrics to migrate %d", tc.uniqueMetrics, tc.numberOfLabels, tc.metricsToMigrate), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				toMetrics(metricsToMigrate)
			}
		})

		b.Run(fmt.Sprintf("new: unique metrics %d, number of labels %d, metrics to migrate %d", tc.uniqueMetrics, tc.numberOfLabels, tc.metricsToMigrate), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				toConstMetrics(metricsToMigrate)
			}
		})
	}
}

func createTestData(uniqueMetrics int, numberOfLabels int, totalToMigrate int) []*PrometheusMetric {
	result := make([]*PrometheusMetric, 0, totalToMigrate)
	for i := 0; i < totalToMigrate; i++ {
		metricName := fmt.Sprintf("metric_%d", rand.IntN(uniqueMetrics-1))
		labels := make(map[string]string, numberOfLabels)
		for j := 0; j < numberOfLabels; j++ {
			labels[fmt.Sprintf("label_%d", j)] = fmt.Sprintf("label-value-%d", j)
		}
		result = append(result, &PrometheusMetric{
			Name:             metricName,
			Labels:           labels,
			Value:            0,
			IncludeTimestamp: false,
		})
	}

	return result
}
