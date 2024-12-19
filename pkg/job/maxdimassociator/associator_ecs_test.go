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

var ecsCluster = &model.TaggedResource{
	ARN:       "arn:aws:ecs:af-south-1:123456789222:cluster/sampleCluster",
	Namespace: "AWS/ECS",
}

var ecsService1 = &model.TaggedResource{
	ARN:       "arn:aws:ecs:af-south-1:123456789222:service/sampleCluster/service1",
	Namespace: "AWS/ECS",
}

var ecsService2 = &model.TaggedResource{
	ARN:       "arn:aws:ecs:af-south-1:123456789222:service/sampleCluster/service2",
	Namespace: "AWS/ECS",
}

var ecsResources = []*model.TaggedResource{
	ecsCluster,
	ecsService1,
	ecsService2,
}

func TestAssociatorECS(t *testing.T) {
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
			name: "cluster metric should be assigned cluster resource",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ECS").ToModelDimensionsRegexp(),
				resources:        ecsResources,
				metric: &model.Metric{
					MetricName: "MemoryReservation",
					Namespace:  "AWS/ECS",
					Dimensions: []model.Dimension{
						{Name: "ClusterName", Value: "sampleCluster"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ecsCluster,
		},
		{
			name: "service metric should be assigned service1 resource",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ECS").ToModelDimensionsRegexp(),
				resources:        ecsResources,
				metric: &model.Metric{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/ECS",
					Dimensions: []model.Dimension{
						{Name: "ClusterName", Value: "sampleCluster"},
						{Name: "ServiceName", Value: "service1"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ecsService1,
		},
		{
			name: "service metric should be assigned service2 resource",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ECS").ToModelDimensionsRegexp(),
				resources:        ecsResources,
				metric: &model.Metric{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/ECS",
					Dimensions: []model.Dimension{
						{Name: "ClusterName", Value: "sampleCluster"},
						{Name: "ServiceName", Value: "service2"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ecsService2,
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
