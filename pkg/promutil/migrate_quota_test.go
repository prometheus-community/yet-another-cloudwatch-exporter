// Copyright 2026 The Prometheus Authors
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

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestBuildQuotaMetrics(t *testing.T) {
	logger := promslog.NewNopLogger()

	type testCase struct {
		name                       string
		results                    []model.QuotaResult
		labelsSnakeCase            bool
		expectedMetrics            []*PrometheusMetric
		expectedObservedLabelCount int
	}

	usage5 := float64(5)
	usage42 := float64(42)

	testCases := []testCase{
		{
			name:                       "empty results produce no metrics",
			results:                    []model.QuotaResult{},
			labelsSnakeCase:            false,
			expectedMetrics:            []*PrometheusMetric{},
			expectedObservedLabelCount: 0,
		},
		{
			name: "single limit metric without usage produces only _limit metric",
			results: []model.QuotaResult{
				{
					Context: &model.ScrapeContext{
						Region:    "us-east-1",
						AccountID: "123456789012",
					},
					Data: []model.QuotaMetricData{
						{
							ServiceCode: "ec2",
							LimitName:   "EC2-VPC Elastic IPs",
							LimitValue:  5,
							UsageValue:  nil,
						},
					},
				},
			},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_ec2_quota_" + PromString("EC2-VPC Elastic IPs") + "_limit",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 5,
				},
			},
			expectedObservedLabelCount: 1,
		},
		{
			name: "limit and usage produces both _limit and _usage metrics",
			results: []model.QuotaResult{
				{
					Context: &model.ScrapeContext{
						Region:    "eu-west-1",
						AccountID: "111111111111",
					},
					Data: []model.QuotaMetricData{
						{
							ServiceCode: "s3",
							LimitName:   "Buckets",
							LimitValue:  100,
							UsageValue:  &usage42,
						},
					},
				},
			},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_s3_quota_" + PromString("Buckets") + "_limit",
					Labels: map[string]string{
						"region":     "eu-west-1",
						"account_id": "111111111111",
					},
					Value: 100,
				},
				{
					Name: "aws_s3_quota_" + PromString("Buckets") + "_usage",
					Labels: map[string]string{
						"region":     "eu-west-1",
						"account_id": "111111111111",
					},
					Value: 42,
				},
			},
			expectedObservedLabelCount: 2,
		},
		{
			name: "metric naming uses PromString for snake_case conversion",
			results: []model.QuotaResult{
				{
					Context: &model.ScrapeContext{
						Region:    "us-east-1",
						AccountID: "123456789012",
					},
					Data: []model.QuotaMetricData{
						{
							ServiceCode: "ec2",
							LimitName:   "Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances",
							LimitValue:  1152,
							UsageValue:  &usage5,
						},
					},
				},
			},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_ec2_quota_" + PromString("Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances") + "_limit",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 1152,
				},
				{
					Name: "aws_ec2_quota_" + PromString("Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances") + "_usage",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 5,
				},
			},
			expectedObservedLabelCount: 2,
		},
		{
			name: "context labels include region, account_id, and account_alias",
			results: []model.QuotaResult{
				{
					Context: &model.ScrapeContext{
						Region:       "ap-southeast-1",
						AccountID:    "999999999999",
						AccountAlias: "my-account",
					},
					Data: []model.QuotaMetricData{
						{
							ServiceCode: "elasticache",
							LimitName:   "Nodes per Region",
							LimitValue:  300,
							UsageValue:  nil,
						},
					},
				},
			},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_elasticache_quota_" + PromString("Nodes per Region") + "_limit",
					Labels: map[string]string{
						"region":        "ap-southeast-1",
						"account_id":    "999999999999",
						"account_alias": "my-account",
					},
					Value: 300,
				},
			},
			expectedObservedLabelCount: 1,
		},
		{
			name: "multiple results from different services",
			results: []model.QuotaResult{
				{
					Context: &model.ScrapeContext{
						Region:    "us-east-1",
						AccountID: "123456789012",
					},
					Data: []model.QuotaMetricData{
						{
							ServiceCode: "ec2",
							LimitName:   "EC2-VPC Elastic IPs",
							LimitValue:  5,
							UsageValue:  &usage5,
						},
					},
				},
				{
					Context: &model.ScrapeContext{
						Region:    "us-east-1",
						AccountID: "123456789012",
					},
					Data: []model.QuotaMetricData{
						{
							ServiceCode: "s3",
							LimitName:   "Buckets",
							LimitValue:  100,
							UsageValue:  &usage42,
						},
					},
				},
			},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_ec2_quota_" + PromString("EC2-VPC Elastic IPs") + "_limit",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 5,
				},
				{
					Name: "aws_ec2_quota_" + PromString("EC2-VPC Elastic IPs") + "_usage",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 5,
				},
				{
					Name: "aws_s3_quota_" + PromString("Buckets") + "_limit",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 100,
				},
				{
					Name: "aws_s3_quota_" + PromString("Buckets") + "_usage",
					Labels: map[string]string{
						"region":     "us-east-1",
						"account_id": "123456789012",
					},
					Value: 42,
				},
			},
			expectedObservedLabelCount: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics, observedLabels := BuildQuotaMetrics(tc.results, tc.labelsSnakeCase, logger)

			require.Len(t, metrics, len(tc.expectedMetrics))
			for i, expected := range tc.expectedMetrics {
				assert.Equal(t, expected.Name, metrics[i].Name, "metric name mismatch at index %d", i)
				assert.Equal(t, expected.Labels, metrics[i].Labels, "metric labels mismatch at index %d", i)
				assert.Equal(t, expected.Value, metrics[i].Value, "metric value mismatch at index %d", i)
			}
			assert.Len(t, observedLabels, tc.expectedObservedLabelCount)
		})
	}
}
