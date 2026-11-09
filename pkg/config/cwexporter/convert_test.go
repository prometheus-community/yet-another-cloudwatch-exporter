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

package cwexporter

import (
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
)

func TestConvert(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		input    string
		expected string
		warnings []string
	}{
		{
			name: "dimension select with a value for every dimension becomes static jobs",
			input: `
region: eu-west-1
metrics:
  - aws_namespace: AWS/ELB
    aws_metric_name: RequestCount
    aws_dimensions: [LoadBalancerName]
    aws_dimension_select:
      LoadBalancerName: [lb1, lb2]
    aws_statistics: [Sum]
`,
			expected: `
apiVersion: v1alpha1
static:
  - name: AWS/ELB LoadBalancerName=lb1
    namespace: AWS/ELB
    regions: [eu-west-1]
    dimensions:
      - {name: LoadBalancerName, value: lb1}
    metrics:
      - {name: RequestCount, statistics: [Sum], period: 60, length: 600, addCloudwatchTimestamp: true}
  - name: AWS/ELB LoadBalancerName=lb2
    namespace: AWS/ELB
    regions: [eu-west-1]
    dimensions:
      - {name: LoadBalancerName, value: lb2}
    metrics:
      - {name: RequestCount, statistics: [Sum], period: 60, length: 600, addCloudwatchTimestamp: true}
`,
			warnings: []string{
				"static jobs (first one from metrics[0] (AWS/ELB RequestCount)): YACE static jobs don't support a delay, so they request the most recent data instead of data that is delay_seconds old",
			},
		},
		{
			name: "tag select becomes a discovery job",
			input: `
region: eu-west-1
role_arn: arn:aws:iam::123456789012:role/cloudwatch
metrics:
  - aws_namespace: AWS/ELB
    aws_metric_name: RequestCount
    aws_dimensions: [AvailabilityZone, LoadBalancerName]
    aws_tag_select:
      tag_selections:
        Monitoring: ["enabled", "yes.please"]
        Team: []
      resource_type_selection: "elasticloadbalancing:loadbalancer"
      resource_id_dimension: LoadBalancerName
    aws_statistics: [Sum]
  - aws_namespace: AWS/ELB
    aws_metric_name: Latency
    aws_dimensions: [AvailabilityZone, LoadBalancerName]
    aws_tag_select:
      tag_selections:
        Monitoring: ["enabled", "yes.please"]
        Team: []
      resource_type_selection: "elasticloadbalancing:loadbalancer"
      resource_id_dimension: LoadBalancerName
    aws_extended_statistics: [p99]
`,
			expected: `
apiVersion: v1alpha1
discovery:
  jobs:
    - type: AWS/ELB
      regions: [eu-west-1]
      roles:
        - roleArn: arn:aws:iam::123456789012:role/cloudwatch
      searchTags:
        - {key: Monitoring, value: '^(?:enabled|yes\.please)$'}
        - {key: Team, value: '.*'}
      dimensionNameRequirements: [AvailabilityZone, LoadBalancerName]
      period: 60
      length: 600
      delay: 600
      addCloudwatchTimestamp: true
      metrics:
        - {name: RequestCount, statistics: [Sum]}
        - {name: Latency, statistics: [p99]}
`,
		},
		{
			name: "no dimension select becomes a custom namespace job with the global settings",
			input: `
region: us-east-1
period_seconds: 300
range_seconds: 900
delay_seconds: 0
set_timestamp: false
metrics:
  - aws_namespace: AWS/SQS
    aws_metric_name: NumberOfMessagesSent
    aws_dimensions: [QueueName]
  - aws_namespace: AWS/SQS
    aws_metric_name: ApproximateAgeOfOldestMessage
    aws_dimensions: [QueueName]
    aws_statistics: [Maximum]
    aws_extended_statistics: [p90]
`,
			expected: `
apiVersion: v1alpha1
customNamespace:
  - name: AWS/SQS
    namespace: AWS/SQS
    regions: [us-east-1]
    dimensionNameRequirements: [QueueName]
    period: 300
    length: 900
    addCloudwatchTimestamp: false
    metrics:
      - {name: NumberOfMessagesSent, statistics: [Sum, SampleCount, Minimum, Maximum, Average]}
      - {name: ApproximateAgeOfOldestMessage, statistics: [Maximum, p90]}
`,
		},
		{
			name: "per metric settings override the global ones and split jobs",
			input: `
region: us-east-1
metrics:
  - aws_namespace: AWS/SQS
    aws_metric_name: NumberOfMessagesSent
    aws_dimensions: [QueueName]
    aws_statistics: [Sum]
  - aws_namespace: AWS/SQS
    aws_metric_name: NumberOfMessagesDeleted
    aws_dimensions: [QueueName]
    aws_statistics: [Sum]
    period_seconds: 300
`,
			expected: `
apiVersion: v1alpha1
customNamespace:
  - name: AWS/SQS
    namespace: AWS/SQS
    regions: [us-east-1]
    dimensionNameRequirements: [QueueName]
    period: 60
    length: 600
    delay: 600
    addCloudwatchTimestamp: true
    metrics:
      - {name: NumberOfMessagesSent, statistics: [Sum]}
  - name: AWS/SQS (2)
    namespace: AWS/SQS
    regions: [us-east-1]
    dimensionNameRequirements: [QueueName]
    period: 300
    length: 600
    delay: 600
    addCloudwatchTimestamp: true
    metrics:
      - {name: NumberOfMessagesDeleted, statistics: [Sum]}
`,
		},
		{
			name: "regex dimension select exports every value",
			input: `
region: eu-west-1
delay_seconds: 0
metrics:
  - aws_namespace: AWS/ELB
    aws_metric_name: RequestCount
    aws_dimensions: [LoadBalancerName]
    aws_dimension_select_regex:
      LoadBalancerName: ["prod-.*"]
    aws_statistics: [Sum]
`,
			expected: `
apiVersion: v1alpha1
customNamespace:
  - name: AWS/ELB
    namespace: AWS/ELB
    regions: [eu-west-1]
    dimensionNameRequirements: [LoadBalancerName]
    period: 60
    length: 600
    addCloudwatchTimestamp: true
    metrics:
      - {name: RequestCount, statistics: [Sum]}
`,
			warnings: []string{
				"metrics[0] (AWS/ELB RequestCount): YACE has no dimension value filter, so every value of LoadBalancerName is exported; filter on the dimension labels with metric_relabel_configs instead",
			},
		},
		{
			name: "dimension select without a value for every dimension exports every value",
			input: `
region: eu-west-1
delay_seconds: 0
metrics:
  - aws_namespace: AWS/ELB
    aws_metric_name: RequestCount
    aws_dimensions: [AvailabilityZone, LoadBalancerName]
    aws_dimension_select:
      LoadBalancerName: [lb1]
    aws_statistics: [Sum]
`,
			expected: `
apiVersion: v1alpha1
customNamespace:
  - name: AWS/ELB
    namespace: AWS/ELB
    regions: [eu-west-1]
    dimensionNameRequirements: [AvailabilityZone, LoadBalancerName]
    period: 60
    length: 600
    addCloudwatchTimestamp: true
    metrics:
      - {name: RequestCount, statistics: [Sum]}
`,
			warnings: []string{
				"metrics[0] (AWS/ELB RequestCount): aws_dimension_select doesn't give values for every dimension, and YACE has no dimension value filter, so every value of LoadBalancerName is exported; filter on the dimension labels with metric_relabel_configs instead",
			},
		},
		{
			name: "metrics without dimensions become a static job without dimensions",
			opts: Options{Region: "us-east-1"},
			input: `
delay_seconds: 0
metrics:
  - aws_namespace: AWS/Billing
    aws_metric_name: EstimatedCharges
    aws_statistics: [Maximum]
    range_seconds: 86400
    period_seconds: 21600
`,
			expected: `
apiVersion: v1alpha1
static:
  - name: AWS/Billing
    namespace: AWS/Billing
    regions: [us-east-1]
    dimensions: []
    metrics:
      - {name: EstimatedCharges, statistics: [Maximum], period: 21600, length: 86400, addCloudwatchTimestamp: true}
`,
		},
		{
			name: "tag select on a namespace YACE can't discover",
			input: `
region: eu-west-1
delay_seconds: 0
use_get_metric_data: true
metrics:
  - aws_namespace: MyApp/Custom
    aws_metric_name: Jobs
    aws_dimensions: [Queue]
    aws_tag_select:
      resource_type_selection: "sqs:"
      resource_id_dimension: Queue
    aws_statistics: [Sum]
`,
			expected: `
apiVersion: v1alpha1
customNamespace:
  - name: MyApp/Custom
    namespace: MyApp/Custom
    regions: [eu-west-1]
    dimensionNameRequirements: [Queue]
    period: 60
    length: 600
    addCloudwatchTimestamp: true
    metrics:
      - {name: Jobs, statistics: [Sum]}
`,
			warnings: []string{
				"use_get_metric_data is ignored: YACE uses GetMetricData for discovery and custom namespace jobs and GetMetricStatistics for static jobs",
				"metrics[0] (MyApp/Custom Jobs): YACE can't discover MyApp/Custom resources by tag, so aws_tag_select is dropped and every resource is exported",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Convert([]byte(tc.input), tc.opts)
			require.NoError(t, err)

			var expected Output
			require.NoError(t, yaml.Unmarshal([]byte(tc.expected), &expected))
			require.Equal(t, &expected, res.Config)
			require.Equal(t, tc.warnings, res.Warnings)

			out, err := res.YAML()
			require.NoError(t, err)
			_, err = (&config.ScrapeConf{}).Parse(out, promslog.NewNopLogger())
			require.NoError(t, err, "YACE rejects the converted config:\n%s", out)
		})
	}
}

func TestConvertManyDimensionCombinations(t *testing.T) {
	input := `
region: eu-west-1
delay_seconds: 0
metrics:
  - aws_namespace: AWS/ELB
    aws_metric_name: RequestCount
    aws_dimensions: [AvailabilityZone, LoadBalancerName]
    aws_dimension_select:
      AvailabilityZone: [a, b, c, d, e, f, g, h, i, j, k]
      LoadBalancerName: [l1, l2, l3, l4, l5, l6, l7, l8, l9, l10]
    aws_statistics: [Sum]
`
	res, err := Convert([]byte(input), Options{})
	require.NoError(t, err)
	require.Empty(t, res.Config.Static)
	require.Len(t, res.Config.CustomNamespace, 1)
	require.Equal(t, []string{
		"metrics[0] (AWS/ELB RequestCount): aws_dimension_select expands to more than 100 dimension combinations, so it is converted to a custom namespace job that exports every value; filter on the dimension labels with metric_relabel_configs instead",
	}, res.Warnings)
}

func TestConvertErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "no region",
			input: "metrics: [{aws_namespace: AWS/ELB, aws_metric_name: RequestCount}]",
			err:   "cloudwatch_exporter config has no region; set one with the region option",
		},
		{
			name:  "no metrics",
			input: "region: eu-west-1",
			err:   "cloudwatch_exporter config has no metrics",
		},
		{
			name:  "missing metric name",
			input: "region: eu-west-1\nmetrics: [{aws_namespace: AWS/ELB}]",
			err:   "metrics[0]: aws_namespace and aws_metric_name are required",
		},
		{
			name: "both kinds of dimension select",
			input: `
region: eu-west-1
metrics:
  - aws_namespace: AWS/ELB
    aws_metric_name: RequestCount
    aws_dimensions: [LoadBalancerName]
    aws_dimension_select: {LoadBalancerName: [lb1]}
    aws_dimension_select_regex: {LoadBalancerName: ["lb.*"]}
`,
			err: "metrics[0]: aws_dimension_select and aws_dimension_select_regex can't be used together",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Convert([]byte(tc.input), Options{})
			require.EqualError(t, err, tc.err)
		})
	}
}
