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
	"math"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestBuildNamespaceInfoMetrics(t *testing.T) {
	type testCase struct {
		name                 string
		resources            []model.TaggedResourceResult
		metrics              []*PrometheusMetric
		observedMetricLabels map[string]model.LabelSet
		labelsSnakeCase      bool
		expectedMetrics      []*PrometheusMetric
		expectedLabels       map[string]model.LabelSet
	}
	testCases := []testCase{
		{
			name: "metric with tag",
			resources: []model.TaggedResourceResult{
				{
					Context: nil,
					Data: []*model.TaggedResource{
						{
							ARN:       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
							Namespace: "AWS/ElastiCache",
							Region:    "us-east-1",
							Tags: []model.Tag{
								{
									Key:   "CustomTag",
									Value: "tag_Value",
								},
							},
						},
					},
				},
			},
			metrics:              []*PrometheusMetric{},
			observedMetricLabels: map[string]model.LabelSet{},
			labelsSnakeCase:      false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_elasticache_info",
					Labels: map[string]string{
						"name":          "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"tag_CustomTag": "tag_Value",
					},
					Value: 0,
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_info": map[string]struct{}{
					"name":          {},
					"tag_CustomTag": {},
				},
			},
		},
		{
			name: "label snake case",
			resources: []model.TaggedResourceResult{
				{
					Context: nil,
					Data: []*model.TaggedResource{
						{
							ARN:       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
							Namespace: "AWS/ElastiCache",
							Region:    "us-east-1",
							Tags: []model.Tag{
								{
									Key:   "CustomTag",
									Value: "tag_Value",
								},
							},
						},
					},
				},
			},
			metrics:              []*PrometheusMetric{},
			observedMetricLabels: map[string]model.LabelSet{},
			labelsSnakeCase:      true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_elasticache_info",
					Labels: map[string]string{
						"name":           "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"tag_custom_tag": "tag_Value",
					},
					Value: 0,
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_info": map[string]struct{}{
					"name":           {},
					"tag_custom_tag": {},
				},
			},
		},
		{
			name: "with observed metrics and labels",
			resources: []model.TaggedResourceResult{
				{
					Context: nil,
					Data: []*model.TaggedResource{
						{
							ARN:       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
							Namespace: "AWS/ElastiCache",
							Region:    "us-east-1",
							Tags: []model.Tag{
								{
									Key:   "CustomTag",
									Value: "tag_Value",
								},
							},
						},
					},
				},
			},
			metrics: []*PrometheusMetric{
				{
					Name: "aws_ec2_cpuutilization_maximum",
					Labels: map[string]string{
						"name":                 "arn:aws:ec2:us-east-1:123456789012:instance/i-abc123",
						"dimension_InstanceId": "i-abc123",
					},
					Value: 0,
				},
			},
			observedMetricLabels: map[string]model.LabelSet{
				"aws_ec2_cpuutilization_maximum": map[string]struct{}{
					"name":                 {},
					"dimension_InstanceId": {},
				},
			},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_ec2_cpuutilization_maximum",
					Labels: map[string]string{
						"name":                 "arn:aws:ec2:us-east-1:123456789012:instance/i-abc123",
						"dimension_InstanceId": "i-abc123",
					},
					Value: 0,
				},
				{
					Name: "aws_elasticache_info",
					Labels: map[string]string{
						"name":           "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"tag_custom_tag": "tag_Value",
					},
					Value: 0,
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_ec2_cpuutilization_maximum": map[string]struct{}{
					"name":                 {},
					"dimension_InstanceId": {},
				},
				"aws_elasticache_info": map[string]struct{}{
					"name":           {},
					"tag_custom_tag": {},
				},
			},
		},
		{
			name: "context on info metrics",
			resources: []model.TaggedResourceResult{
				{
					Context: &model.ScrapeContext{
						Region:    "us-east-2",
						AccountID: "12345",
						CustomTags: []model.Tag{{
							Key:   "billable-to",
							Value: "api",
						}},
					},
					Data: []*model.TaggedResource{
						{
							ARN:       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
							Namespace: "AWS/ElastiCache",
							Region:    "us-east-1",
							Tags: []model.Tag{
								{
									Key:   "cache_name",
									Value: "cache_instance_1",
								},
							},
						},
					},
				},
			},
			metrics:              []*PrometheusMetric{},
			observedMetricLabels: map[string]model.LabelSet{},
			labelsSnakeCase:      true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_elasticache_info",
					Labels: map[string]string{
						"name":                   "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"tag_cache_name":         "cache_instance_1",
						"account_id":             "12345",
						"region":                 "us-east-2",
						"custom_tag_billable_to": "api",
					},
					Value: 0,
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_info": map[string]struct{}{
					"name":                   {},
					"tag_cache_name":         {},
					"account_id":             {},
					"region":                 {},
					"custom_tag_billable_to": {},
				},
			},
		},
		{
			name: "metric with nonstandard namespace",
			resources: []model.TaggedResourceResult{
				{
					Context: nil,
					Data: []*model.TaggedResource{
						{
							ARN:       "arn:aws:sagemaker:us-east-1:123456789012:training-job/sagemaker-xgboost",
							Namespace: "/aws/sagemaker/TrainingJobs",
							Region:    "us-east-1",
							Tags: []model.Tag{
								{
									Key:   "CustomTag",
									Value: "tag_Value",
								},
							},
						},
					},
				},
			},
			metrics:              []*PrometheusMetric{},
			observedMetricLabels: map[string]model.LabelSet{},
			labelsSnakeCase:      false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name: "aws_sagemaker_trainingjobs_info",
					Labels: map[string]string{
						"name":          "arn:aws:sagemaker:us-east-1:123456789012:training-job/sagemaker-xgboost",
						"tag_CustomTag": "tag_Value",
					},
					Value: 0,
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_sagemaker_trainingjobs_info": map[string]struct{}{
					"name":          {},
					"tag_CustomTag": {},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics, labels := BuildNamespaceInfoMetrics(tc.resources, tc.metrics, tc.observedMetricLabels, tc.labelsSnakeCase, logging.NewNopLogger())
			require.Equal(t, tc.expectedMetrics, metrics)
			require.Equal(t, tc.expectedLabels, labels)
		})
	}
}

func TestBuildMetrics(t *testing.T) {
	ts := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name            string
		data            []model.CloudwatchMetricResult
		labelsSnakeCase bool
		expectedMetrics []*PrometheusMetric
		expectedLabels  map[string]model.LabelSet
		expectedErr     error
	}

	testCases := []testCase{
		{
			name: "metric with GetMetricDataResult and non-nil datapoint",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:     "us-east-1",
					AccountID:  "123456789012",
					CustomTags: nil,
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "CPUUtilization",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              true,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
					{
						MetricName: "FreeableMemory",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(2),
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
					{
						MetricName: "NetworkBytesIn",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              true,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(3),
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
					{
						MetricName: "NetworkBytesOut",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              true,
							AddCloudwatchTimestamp: true,
						},
						Namespace: "AWS/ElastiCache",
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(4),
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
				},
			}},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_elasticache_cpuutilization_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
				},
				{
					Name:      "aws_elasticache_freeable_memory_average",
					Value:     2,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
				},
				{
					Name:      "aws_elasticache_network_bytes_in_average",
					Value:     3,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
				},
				{
					Name:             "aws_elasticache_network_bytes_out_average",
					Value:            4,
					Timestamp:        ts,
					IncludeTimestamp: true,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_cpuutilization_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
				"aws_elasticache_freeable_memory_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
				"aws_elasticache_network_bytes_in_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
				"aws_elasticache_network_bytes_out_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "metric with GetMetricDataResult and nil datapoint",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:     "us-east-1",
					AccountID:  "123456789012",
					CustomTags: nil,
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "CPUUtilization",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              true,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: nil,
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
					{
						MetricName: "FreeableMemory",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",

						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: nil,
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
					{
						MetricName: "NetworkBytesIn",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              true,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: nil,
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
					{
						MetricName: "NetworkBytesOut",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              true,
							AddCloudwatchTimestamp: true,
						},
						Namespace: "AWS/ElastiCache",
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: nil,
							Timestamp: ts,
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
				},
			}},
			labelsSnakeCase: false,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_elasticache_cpuutilization_average",
					Value:     0,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
					IncludeTimestamp: false,
				},
				{
					Name:      "aws_elasticache_freeable_memory_average",
					Value:     math.NaN(),
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
					IncludeTimestamp: false,
				},
				{
					Name:      "aws_elasticache_network_bytes_in_average",
					Value:     0,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":               "123456789012",
						"name":                     "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                   "us-east-1",
						"dimension_CacheClusterId": "redis-cluster",
					},
					IncludeTimestamp: false,
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_cpuutilization_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
				"aws_elasticache_freeable_memory_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
				"aws_elasticache_network_bytes_in_average": {
					"account_id":               {},
					"name":                     {},
					"region":                   {},
					"dimension_CacheClusterId": {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "label snake case",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:     "us-east-1",
					AccountID:  "123456789012",
					CustomTags: nil,
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "CPUUtilization",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
				},
			}},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_elasticache_cpuutilization_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":                 "123456789012",
						"name":                       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                     "us-east-1",
						"dimension_cache_cluster_id": "redis-cluster",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_cpuutilization_average": {
					"account_id":                 {},
					"name":                       {},
					"region":                     {},
					"dimension_cache_cluster_id": {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "metric with nonstandard namespace",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:     "us-east-1",
					AccountID:  "123456789012",
					CustomTags: nil,
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "CPUUtilization",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "/aws/sagemaker/TrainingJobs",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "Host",
								Value: "sagemaker-xgboost",
							},
						},
						ResourceName: "arn:aws:sagemaker:us-east-1:123456789012:training-job/sagemaker-xgboost",
					},
				},
			}},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_sagemaker_trainingjobs_cpuutilization_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":     "123456789012",
						"name":           "arn:aws:sagemaker:us-east-1:123456789012:training-job/sagemaker-xgboost",
						"region":         "us-east-1",
						"dimension_host": "sagemaker-xgboost",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_sagemaker_trainingjobs_cpuutilization_average": {
					"account_id":     {},
					"name":           {},
					"region":         {},
					"dimension_host": {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "metric with metric name that does duplicates part of the namespace as a prefix",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:     "us-east-1",
					AccountID:  "123456789012",
					CustomTags: nil,
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "glue.driver.aggregate.bytesRead",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "Glue",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "JobName",
								Value: "test-job",
							},
						},
						ResourceName: "arn:aws:glue:us-east-1:123456789012:job/test-job",
					},
				},
			}},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_glue_driver_aggregate_bytes_read_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":         "123456789012",
						"name":               "arn:aws:glue:us-east-1:123456789012:job/test-job",
						"region":             "us-east-1",
						"dimension_job_name": "test-job",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_glue_driver_aggregate_bytes_read_average": {
					"account_id":         {},
					"name":               {},
					"region":             {},
					"dimension_job_name": {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "metric with metric name that does not duplicate part of the namespace as a prefix",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:     "us-east-1",
					AccountID:  "123456789012",
					CustomTags: nil,
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "aggregate.glue.jobs.bytesRead",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "Glue",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "JobName",
								Value: "test-job",
							},
						},
						ResourceName: "arn:aws:glue:us-east-1:123456789012:job/test-job",
					},
				},
			}},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_glue_aggregate_glue_jobs_bytes_read_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":         "123456789012",
						"name":               "arn:aws:glue:us-east-1:123456789012:job/test-job",
						"region":             "us-east-1",
						"dimension_job_name": "test-job",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_glue_aggregate_glue_jobs_bytes_read_average": {
					"account_id":         {},
					"name":               {},
					"region":             {},
					"dimension_job_name": {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "custom tag",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:    "us-east-1",
					AccountID: "123456789012",
					CustomTags: []model.Tag{{
						Key:   "billable-to",
						Value: "api",
					}},
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "CPUUtilization",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
				},
			}},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_elasticache_cpuutilization_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":                 "123456789012",
						"name":                       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                     "us-east-1",
						"dimension_cache_cluster_id": "redis-cluster",
						"custom_tag_billable_to":     "api",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_cpuutilization_average": {
					"account_id":                 {},
					"name":                       {},
					"region":                     {},
					"dimension_cache_cluster_id": {},
					"custom_tag_billable_to":     {},
				},
			},
			expectedErr: nil,
		},
		{
			name: "scraping with aws account alias",
			data: []model.CloudwatchMetricResult{{
				Context: &model.ScrapeContext{
					Region:       "us-east-1",
					AccountID:    "123456789012",
					AccountAlias: "billingacct",
				},
				Data: []*model.CloudwatchData{
					{
						MetricName: "CPUUtilization",
						MetricMigrationParams: model.MetricMigrationParams{
							NilToZero:              false,
							AddCloudwatchTimestamp: false,
						},
						Namespace: "AWS/ElastiCache",
						GetMetricDataResult: &model.GetMetricDataResult{
							Statistic: "Average",
							Datapoint: aws.Float64(1),
							Timestamp: ts,
						},
						Dimensions: []model.Dimension{
							{
								Name:  "CacheClusterId",
								Value: "redis-cluster",
							},
						},
						ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
					},
				},
			}},
			labelsSnakeCase: true,
			expectedMetrics: []*PrometheusMetric{
				{
					Name:      "aws_elasticache_cpuutilization_average",
					Value:     1,
					Timestamp: ts,
					Labels: map[string]string{
						"account_id":                 "123456789012",
						"account_alias":              "billingacct",
						"name":                       "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
						"region":                     "us-east-1",
						"dimension_cache_cluster_id": "redis-cluster",
					},
				},
			},
			expectedLabels: map[string]model.LabelSet{
				"aws_elasticache_cpuutilization_average": {
					"account_id":                 {},
					"account_alias":              {},
					"name":                       {},
					"region":                     {},
					"dimension_cache_cluster_id": {},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, labels, err := BuildMetrics(tc.data, tc.labelsSnakeCase, logging.NewNopLogger())
			if tc.expectedErr != nil {
				require.Equal(t, tc.expectedErr, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, replaceNaNValues(tc.expectedMetrics), replaceNaNValues(res))
				require.Equal(t, tc.expectedLabels, labels)
			}
		})
	}
}

func Benchmark_BuildMetrics(b *testing.B) {
	ts := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	data := []model.CloudwatchMetricResult{{
		Context: &model.ScrapeContext{
			Region:     "us-east-1",
			AccountID:  "123456789012",
			CustomTags: nil,
		},
		Data: []*model.CloudwatchData{
			{
				MetricName: "CPUUtilization",
				MetricMigrationParams: model.MetricMigrationParams{
					NilToZero:              true,
					AddCloudwatchTimestamp: false,
				},
				Namespace: "AWS/ElastiCache",
				GetMetricDataResult: &model.GetMetricDataResult{
					Statistic: "Average",
					Datapoint: aws.Float64(1),
					Timestamp: ts,
				},
				Dimensions: []model.Dimension{
					{
						Name:  "CacheClusterId",
						Value: "redis-cluster",
					},
				},
				ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
				Tags: []model.Tag{{
					Key:   "managed_by",
					Value: "terraform",
				}},
			},
			{
				MetricName: "FreeableMemory",
				MetricMigrationParams: model.MetricMigrationParams{
					NilToZero:              false,
					AddCloudwatchTimestamp: false,
				},
				Namespace: "AWS/ElastiCache",
				Dimensions: []model.Dimension{
					{
						Name:  "CacheClusterId",
						Value: "redis-cluster",
					},
				},
				GetMetricDataResult: &model.GetMetricDataResult{
					Statistic: "Average",
					Datapoint: aws.Float64(2),
					Timestamp: ts,
				},
				ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
				Tags: []model.Tag{{
					Key:   "managed_by",
					Value: "terraform",
				}},
			},
			{
				MetricName: "NetworkBytesIn",
				MetricMigrationParams: model.MetricMigrationParams{
					NilToZero:              true,
					AddCloudwatchTimestamp: false,
				},
				Namespace: "AWS/ElastiCache",
				Dimensions: []model.Dimension{
					{
						Name:  "CacheClusterId",
						Value: "redis-cluster",
					},
				},
				GetMetricDataResult: &model.GetMetricDataResult{
					Statistic: "Average",
					Datapoint: aws.Float64(3),
					Timestamp: ts,
				},
				ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
				Tags: []model.Tag{{
					Key:   "managed_by",
					Value: "terraform",
				}},
			},
			{
				MetricName: "NetworkBytesOut",
				MetricMigrationParams: model.MetricMigrationParams{
					NilToZero:              true,
					AddCloudwatchTimestamp: true,
				},
				Namespace: "AWS/ElastiCache",
				Dimensions: []model.Dimension{
					{
						Name:  "CacheClusterId",
						Value: "redis-cluster",
					},
				},
				GetMetricDataResult: &model.GetMetricDataResult{
					Statistic: "Average",
					Datapoint: aws.Float64(4),
					Timestamp: ts,
				},
				ResourceName: "arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster",
				Tags: []model.Tag{{
					Key:   "managed_by",
					Value: "terraform",
				}},
			},
		},
	}}

	var labels map[string]model.LabelSet
	var err error

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, labels, err = BuildMetrics(data, false, logging.NewNopLogger())
	}

	expectedLabels := map[string]model.LabelSet{
		"aws_elasticache_cpuutilization_average": {
			"account_id":               {},
			"name":                     {},
			"region":                   {},
			"dimension_CacheClusterId": {},
			"tag_managed_by":           {},
		},
		"aws_elasticache_freeable_memory_average": {
			"account_id":               {},
			"name":                     {},
			"region":                   {},
			"dimension_CacheClusterId": {},
			"tag_managed_by":           {},
		},
		"aws_elasticache_network_bytes_in_average": {
			"account_id":               {},
			"name":                     {},
			"region":                   {},
			"dimension_CacheClusterId": {},
			"tag_managed_by":           {},
		},
		"aws_elasticache_network_bytes_out_average": {
			"account_id":               {},
			"name":                     {},
			"region":                   {},
			"dimension_CacheClusterId": {},
			"tag_managed_by":           {},
		},
	}

	require.NoError(b, err)
	require.Equal(b, expectedLabels, labels)
}

// replaceNaNValues replaces any NaN floating-point values with a marker value (54321.0)
// so that require.Equal() can compare them. By default, require.Equal() will fail if any
// struct values are NaN because NaN != NaN
func replaceNaNValues(metrics []*PrometheusMetric) []*PrometheusMetric {
	for _, metric := range metrics {
		if math.IsNaN(metric.Value) {
			metric.Value = 54321.0
		}
	}
	return metrics
}

// TestSortByTimeStamp validates that sortByTimestamp() sorts in descending order.
func TestSortByTimeStamp(t *testing.T) {
	ts := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	dataPointMiddle := &model.Datapoint{
		Timestamp: aws.Time(ts.Add(time.Minute * 2 * -1)),
		Maximum:   aws.Float64(2),
	}

	dataPointNewest := &model.Datapoint{
		Timestamp: aws.Time(ts.Add(time.Minute * -1)),
		Maximum:   aws.Float64(1),
	}

	dataPointOldest := &model.Datapoint{
		Timestamp: aws.Time(ts.Add(time.Minute * 3 * -1)),
		Maximum:   aws.Float64(3),
	}

	cloudWatchDataPoints := []*model.Datapoint{
		dataPointMiddle,
		dataPointNewest,
		dataPointOldest,
	}

	sortedDataPoints := sortByTimestamp(cloudWatchDataPoints)

	expectedDataPoints := []*model.Datapoint{
		dataPointNewest,
		dataPointMiddle,
		dataPointOldest,
	}

	require.Equal(t, expectedDataPoints, sortedDataPoints)
}

func Test_EnsureLabelConsistencyAndRemoveDuplicates(t *testing.T) {
	testCases := []struct {
		name           string
		metrics        []*PrometheusMetric
		observedLabels map[string]model.LabelSet
		output         []*PrometheusMetric
	}{
		{
			name: "adds missing labels",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
					Value:  1.0,
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label2": "value2"},
					Value:  2.0,
				},
				{
					Name:   "metric1",
					Labels: map[string]string{},
					Value:  3.0,
				},
			},
			observedLabels: map[string]model.LabelSet{"metric1": {"label1": {}, "label2": {}, "label3": {}}},
			output: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1", "label2": "", "label3": ""},
					Value:  1.0,
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "", "label3": "", "label2": "value2"},
					Value:  2.0,
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "", "label2": "", "label3": ""},
					Value:  3.0,
				},
			},
		},
		{
			name: "duplicate metric",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
			},
			observedLabels: map[string]model.LabelSet{},
			output: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
			},
		},
		{
			name: "duplicate metric, multiple labels",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1", "label2": "value2"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label2": "value2", "label1": "value1"},
				},
			},
			observedLabels: map[string]model.LabelSet{},
			output: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1", "label2": "value2"},
				},
			},
		},
		{
			name: "metric with different labels",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label2": "value2"},
				},
			},
			observedLabels: map[string]model.LabelSet{},
			output: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label2": "value2"},
				},
			},
		},
		{
			name: "two metrics",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric2",
					Labels: map[string]string{"label1": "value1"},
				},
			},
			observedLabels: map[string]model.LabelSet{},
			output: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric2",
					Labels: map[string]string{"label1": "value1"},
				},
			},
		},
		{
			name: "two metrics with different labels",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric2",
					Labels: map[string]string{"label2": "value2"},
				},
			},
			observedLabels: map[string]model.LabelSet{},
			output: []*PrometheusMetric{
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric2",
					Labels: map[string]string{"label2": "value2"},
				},
			},
		},
		{
			name: "multiple duplicates and non-duplicates",
			metrics: []*PrometheusMetric{
				{
					Name:   "metric2",
					Labels: map[string]string{"label2": "value2"},
				},
				{
					Name:   "metric2",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
			},
			observedLabels: map[string]model.LabelSet{},
			output: []*PrometheusMetric{
				{
					Name:   "metric2",
					Labels: map[string]string{"label2": "value2"},
				},
				{
					Name:   "metric2",
					Labels: map[string]string{"label1": "value1"},
				},
				{
					Name:   "metric1",
					Labels: map[string]string{"label1": "value1"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := EnsureLabelConsistencyAndRemoveDuplicates(tc.metrics, tc.observedLabels)
			require.ElementsMatch(t, tc.output, actual)
		})
	}
}
