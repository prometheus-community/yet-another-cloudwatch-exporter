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
package maxdimassociator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var ecServerless = &model.TaggedResource{
	ARN:       "arn:aws:elasticache:eu-east-1:123456789012:serverlesscache:test-serverless-cluster",
	Namespace: "AWS/ElastiCache",
}

var ecCluster = &model.TaggedResource{
	ARN:       "arn:aws:elasticache:eu-east-1:123456789012:cluster:test-cluster-0001-001",
	Namespace: "AWS/ElastiCache",
}

var ecResources = []*model.TaggedResource{
	ecServerless,
	ecCluster,
}

func TestAssociatorEC(t *testing.T) {
	type args struct {
		dimensionRegexps []model.DimensionsRegexp
		resources        []*model.TaggedResource
		metric           *model.Metric
	}

	type testCase struct {
		name             string
		args             args
		expectedSkip     bool
		expectedResource *model.TaggedResource
	}

	testcases := []testCase{
		{
			name: "should match with clusterId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ElastiCache").ToModelDimensionsRegexp(),
				resources:        ecResources,
				metric: &model.Metric{
					MetricName: "TotalCmdsCount",
					Namespace:  "AWS/ElastiCache",
					Dimensions: []model.Dimension{
						{Name: "clusterId", Value: "test-serverless-cluster"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ecServerless,
		},
		{
			name: "should match with CacheClusterId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ElastiCache").ToModelDimensionsRegexp(),
				resources:        ecResources,
				metric: &model.Metric{
					MetricName: "EngineCPUUtilization",
					Namespace:  "AWS/ElastiCache",
					Dimensions: []model.Dimension{
						{Name: "CacheClusterId", Value: "test-cluster-0001-001"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ecCluster,
		},
		{
			name: "should skip with unmatched CacheClusterId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ElastiCache").ToModelDimensionsRegexp(),
				resources:        ecResources,
				metric: &model.Metric{
					MetricName: "EngineCPUUtilization",
					Namespace:  "AWS/ElastiCache",
					Dimensions: []model.Dimension{
						{Name: "CacheClusterId", Value: "test-cluster-0001-002"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should skip with unmatched clusterId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ElastiCache").ToModelDimensionsRegexp(),
				resources:        ecResources,
				metric: &model.Metric{
					MetricName: "TotalCmdsCount",
					Namespace:  "AWS/ElastiCache",
					Dimensions: []model.Dimension{
						{Name: "clusterId", Value: "test-unmatched-serverless-cluster"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			associator := NewAssociator(logging.NewNopLogger(), tc.args.dimensionRegexps, tc.args.resources)
			res, skip := associator.AssociateMetricToResource(tc.args.metric)
			require.Equal(t, tc.expectedSkip, skip)
			require.Equal(t, tc.expectedResource, res)
		})
	}
}
