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
package job

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/maxdimassociator"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func Test_getFilteredMetricDatas(t *testing.T) {
	type args struct {
		region                    string
		accountID                 string
		namespace                 string
		customTags                []model.Tag
		tagsOnMetrics             []string
		dimensionRegexps          []model.DimensionsRegexp
		dimensionNameRequirements []string
		resources                 []*model.TaggedResource
		metricsList               []*model.Metric
		m                         *model.MetricConfig
	}
	tests := []struct {
		name               string
		args               args
		wantGetMetricsData []model.CloudwatchData
	}{
		{
			"additional dimension",
			args{
				region:     "us-east-1",
				accountID:  "123123123123",
				namespace:  "efs",
				customTags: nil,
				tagsOnMetrics: []string{
					"Value1",
					"Value2",
				},
				dimensionRegexps: config.SupportedServices.GetService("AWS/EFS").ToModelDimensionsRegexp(),
				resources: []*model.TaggedResource{
					{
						ARN: "arn:aws:elasticfilesystem:us-east-1:123123123123:file-system/fs-abc123",
						Tags: []model.Tag{
							{
								Key:   "Tag",
								Value: "some-Tag",
							},
						},
						Namespace: "efs",
						Region:    "us-east-1",
					},
				},
				metricsList: []*model.Metric{
					{
						MetricName: "StorageBytes",
						Dimensions: []model.Dimension{
							{
								Name:  "FileSystemId",
								Value: "fs-abc123",
							},
							{
								Name:  "StorageClass",
								Value: "Standard",
							},
						},
						Namespace: "AWS/EFS",
					},
				},
				m: &model.MetricConfig{
					Name: "StorageBytes",
					Statistics: []string{
						"Average",
					},
					Period:                 60,
					Length:                 600,
					Delay:                  120,
					NilToZero:              false,
					AddCloudwatchTimestamp: false,
				},
			},
			[]model.CloudwatchData{
				{
					MetricName: "StorageBytes",
					Dimensions: []model.Dimension{
						{
							Name:  "FileSystemId",
							Value: "fs-abc123",
						},
						{
							Name:  "StorageClass",
							Value: "Standard",
						},
					},
					ResourceName: "arn:aws:elasticfilesystem:us-east-1:123123123123:file-system/fs-abc123",
					Namespace:    "efs",
					Tags: []model.Tag{
						{
							Key:   "Value1",
							Value: "",
						},
						{
							Key:   "Value2",
							Value: "",
						},
					},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Period:    60,
						Length:    600,
						Delay:     120,
						Statistic: "Average",
					},
					MetricMigrationParams: model.MetricMigrationParams{
						NilToZero:              false,
						AddCloudwatchTimestamp: false,
					},
				},
			},
		},
		{
			"ec2",
			args{
				region:     "us-east-1",
				accountID:  "123123123123",
				namespace:  "ec2",
				customTags: nil,
				tagsOnMetrics: []string{
					"Value1",
					"Value2",
				},
				dimensionRegexps: config.SupportedServices.GetService("AWS/EC2").ToModelDimensionsRegexp(),
				resources: []*model.TaggedResource{
					{
						ARN: "arn:aws:ec2:us-east-1:123123123123:instance/i-12312312312312312",
						Tags: []model.Tag{
							{
								Key:   "Name",
								Value: "some-Node",
							},
						},
						Namespace: "ec2",
						Region:    "us-east-1",
					},
				},
				metricsList: []*model.Metric{
					{
						MetricName: "CPUUtilization",
						Dimensions: []model.Dimension{
							{
								Name:  "InstanceId",
								Value: "i-12312312312312312",
							},
						},
						Namespace: "AWS/EC2",
					},
				},
				m: &model.MetricConfig{
					Name: "CPUUtilization",
					Statistics: []string{
						"Average",
					},
					Period:                 60,
					Length:                 600,
					Delay:                  120,
					NilToZero:              false,
					AddCloudwatchTimestamp: false,
				},
			},
			[]model.CloudwatchData{
				{
					MetricName:   "CPUUtilization",
					ResourceName: "arn:aws:ec2:us-east-1:123123123123:instance/i-12312312312312312",
					Namespace:    "ec2",
					Dimensions: []model.Dimension{
						{
							Name:  "InstanceId",
							Value: "i-12312312312312312",
						},
					},
					Tags: []model.Tag{
						{
							Key:   "Value1",
							Value: "",
						},
						{
							Key:   "Value2",
							Value: "",
						},
					},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Statistic: "Average",
						Period:    60,
						Length:    600,
						Delay:     120,
					},
					MetricMigrationParams: model.MetricMigrationParams{
						NilToZero:              false,
						AddCloudwatchTimestamp: false,
					},
				},
			},
		},
		{
			"kafka",
			args{
				region:     "us-east-1",
				accountID:  "123123123123",
				namespace:  "kafka",
				customTags: nil,
				tagsOnMetrics: []string{
					"Value1",
					"Value2",
				},
				dimensionRegexps: config.SupportedServices.GetService("AWS/Kafka").ToModelDimensionsRegexp(),
				resources: []*model.TaggedResource{
					{
						ARN: "arn:aws:kafka:us-east-1:123123123123:cluster/demo-cluster-1/12312312-1231-1231-1231-123123123123-12",
						Tags: []model.Tag{
							{
								Key:   "Test",
								Value: "Value",
							},
						},
						Namespace: "kafka",
						Region:    "us-east-1",
					},
				},
				metricsList: []*model.Metric{
					{
						MetricName: "GlobalTopicCount",
						Dimensions: []model.Dimension{
							{
								Name:  "Cluster Name",
								Value: "demo-cluster-1",
							},
						},
						Namespace: "AWS/Kafka",
					},
				},
				m: &model.MetricConfig{
					Name: "GlobalTopicCount",
					Statistics: []string{
						"Average",
					},
					Period:                 60,
					Length:                 600,
					Delay:                  120,
					NilToZero:              false,
					AddCloudwatchTimestamp: false,
				},
			},
			[]model.CloudwatchData{
				{
					MetricName: "GlobalTopicCount",
					Dimensions: []model.Dimension{
						{
							Name:  "Cluster Name",
							Value: "demo-cluster-1",
						},
					},
					ResourceName: "arn:aws:kafka:us-east-1:123123123123:cluster/demo-cluster-1/12312312-1231-1231-1231-123123123123-12",
					Namespace:    "kafka",
					Tags: []model.Tag{
						{
							Key:   "Value1",
							Value: "",
						},
						{
							Key:   "Value2",
							Value: "",
						},
					},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Statistic: "Average",
						Period:    60,
						Length:    600,
						Delay:     120,
					},
					MetricMigrationParams: model.MetricMigrationParams{
						NilToZero:              false,
						AddCloudwatchTimestamp: false,
					},
				},
			},
		},
		{
			"alb",
			args{
				region:                    "us-east-1",
				accountID:                 "123123123123",
				namespace:                 "alb",
				customTags:                nil,
				tagsOnMetrics:             nil,
				dimensionRegexps:          config.SupportedServices.GetService("AWS/ApplicationELB").ToModelDimensionsRegexp(),
				dimensionNameRequirements: []string{"LoadBalancer", "TargetGroup"},
				resources: []*model.TaggedResource{
					{
						ARN: "arn:aws:elasticloadbalancing:us-east-1:123123123123:loadbalancer/app/some-ALB/0123456789012345",
						Tags: []model.Tag{
							{
								Key:   "Name",
								Value: "some-ALB",
							},
						},
						Namespace: "alb",
						Region:    "us-east-1",
					},
				},
				metricsList: []*model.Metric{
					{
						MetricName: "RequestCount",
						Dimensions: []model.Dimension{
							{
								Name:  "LoadBalancer",
								Value: "app/some-ALB/0123456789012345",
							},
							{
								Name:  "TargetGroup",
								Value: "targetgroup/some-ALB/9999666677773333",
							},
							{
								Name:  "AvailabilityZone",
								Value: "us-east-1",
							},
						},
						Namespace: "AWS/ApplicationELB",
					},
					{
						MetricName: "RequestCount",
						Dimensions: []model.Dimension{
							{
								Name:  "LoadBalancer",
								Value: "app/some-ALB/0123456789012345",
							},
							{
								Name:  "TargetGroup",
								Value: "targetgroup/some-ALB/9999666677773333",
							},
						},
						Namespace: "AWS/ApplicationELB",
					},
					{
						MetricName: "RequestCount",
						Dimensions: []model.Dimension{
							{
								Name:  "LoadBalancer",
								Value: "app/some-ALB/0123456789012345",
							},
							{
								Name:  "AvailabilityZone",
								Value: "us-east-1",
							},
						},
						Namespace: "AWS/ApplicationELB",
					},
					{
						MetricName: "RequestCount",
						Dimensions: []model.Dimension{
							{
								Name:  "LoadBalancer",
								Value: "app/some-ALB/0123456789012345",
							},
						},
						Namespace: "AWS/ApplicationELB",
					},
				},
				m: &model.MetricConfig{
					Name: "RequestCount",
					Statistics: []string{
						"Sum",
					},
					Period:                 60,
					Length:                 600,
					Delay:                  120,
					NilToZero:              false,
					AddCloudwatchTimestamp: false,
				},
			},
			[]model.CloudwatchData{
				{
					MetricName: "RequestCount",
					Dimensions: []model.Dimension{
						{
							Name:  "LoadBalancer",
							Value: "app/some-ALB/0123456789012345",
						},
						{
							Name:  "TargetGroup",
							Value: "targetgroup/some-ALB/9999666677773333",
						},
					},
					ResourceName: "arn:aws:elasticloadbalancing:us-east-1:123123123123:loadbalancer/app/some-ALB/0123456789012345",
					Namespace:    "alb",
					Tags:         []model.Tag{},
					GetMetricDataProcessingParams: &model.GetMetricDataProcessingParams{
						Statistic: "Sum",
						Period:    60,
						Length:    600,
						Delay:     120,
					},
					MetricMigrationParams: model.MetricMigrationParams{
						NilToZero:              false,
						AddCloudwatchTimestamp: false,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assoc := maxdimassociator.NewAssociator(logging.NewNopLogger(), tt.args.dimensionRegexps, tt.args.resources)
			metricDatas := getFilteredMetricDatas(logging.NewNopLogger(), tt.args.namespace, tt.args.tagsOnMetrics, tt.args.metricsList, tt.args.dimensionNameRequirements, tt.args.m, assoc)
			if len(metricDatas) != len(tt.wantGetMetricsData) {
				t.Errorf("len(getFilteredMetricDatas()) = %v, want %v", len(metricDatas), len(tt.wantGetMetricsData))
			}
			for i, got := range metricDatas {
				want := tt.wantGetMetricsData[i]
				assert.Equal(t, want.MetricName, got.MetricName)
				assert.Equal(t, want.ResourceName, got.ResourceName)
				assert.Equal(t, want.Namespace, got.Namespace)
				assert.ElementsMatch(t, want.Dimensions, got.Dimensions)
				assert.ElementsMatch(t, want.Tags, got.Tags)
				assert.Equal(t, want.MetricMigrationParams, got.MetricMigrationParams)
				assert.Equal(t, want.GetMetricDataProcessingParams.Statistic, got.GetMetricDataProcessingParams.Statistic)
				assert.Equal(t, want.GetMetricDataProcessingParams.Length, got.GetMetricDataProcessingParams.Length)
				assert.Equal(t, want.GetMetricDataProcessingParams.Period, got.GetMetricDataProcessingParams.Period)
				assert.Equal(t, want.GetMetricDataProcessingParams.Delay, got.GetMetricDataProcessingParams.Delay)
				assert.Nil(t, got.GetMetricDataResult)
				assert.Nil(t, got.GetMetricStatisticsResult)
			}
		})
	}
}
