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

var ec2Instance1 = &model.TaggedResource{
	ARN:       "arn:aws:ec2:us-east-1:123456789012:instance/i-abc123",
	Namespace: "AWS/EC2",
}

var ec2Instance2 = &model.TaggedResource{
	ARN:       "arn:aws:ec2:us-east-1:123456789012:instance/i-def456",
	Namespace: "AWS/EC2",
}

var ec2Resources = []*model.TaggedResource{
	ec2Instance1,
	ec2Instance2,
}

func TestAssociatorEC2(t *testing.T) {
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
			name: "should match with InstanceId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/EC2").ToModelDimensionsRegexp(),
				resources:        ec2Resources,
				metric: &model.Metric{
					Namespace:  "AWS/EC2",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-abc123"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ec2Instance1,
		},
		{
			name: "should match another instance with InstanceId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/EC2").ToModelDimensionsRegexp(),
				resources:        ec2Resources,
				metric: &model.Metric{
					Namespace:  "AWS/EC2",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-def456"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ec2Instance2,
		},
		{
			name: "should skip with unmatched InstanceId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/EC2").ToModelDimensionsRegexp(),
				resources:        ec2Resources,
				metric: &model.Metric{
					Namespace:  "AWS/EC2",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-blahblah"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should not skip when unmatching because of non-ARN dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/EC2").ToModelDimensionsRegexp(),
				resources:        ec2Resources,
				metric: &model.Metric{
					Namespace:  "AWS/EC2",
					MetricName: "StatusCheckFailed_System",
					Dimensions: []model.Dimension{
						{Name: "AutoScalingGroupName", Value: "some-asg-name"},
					},
				},
			},
			expectedSkip:     false,
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
