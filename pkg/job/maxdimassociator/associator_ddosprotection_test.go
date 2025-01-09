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

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var protectedResources1 = &model.TaggedResource{
	ARN:       "arn:aws:ec2:us-east-1:123456789012:instance/i-abc123",
	Namespace: "AWS/DDoSProtection",
}

var protectedResources2 = &model.TaggedResource{
	ARN:       "arn:aws:ec2:us-east-1:123456789012:instance/i-def456",
	Namespace: "AWS/DDoSProtection",
}

var protectedResources = []*model.TaggedResource{
	protectedResources1,
	protectedResources2,
}

func TestAssociatorDDoSProtection(t *testing.T) {
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
			name: "should match with ResourceArn dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/DDoSProtection").ToModelDimensionsRegexp(),
				resources:        protectedResources,
				metric: &model.Metric{
					Namespace:  "AWS/DDoSProtection",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "ResourceArn", Value: "arn:aws:ec2:us-east-1:123456789012:instance/i-abc123"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: protectedResources1,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			associator := NewAssociator(promslog.NewNopLogger(), tc.args.dimensionRegexps, tc.args.resources)
			res, skip := associator.AssociateMetricToResource(tc.args.metric)
			assert.Equal(t, tc.expectedSkip, skip)
			assert.Equal(t, tc.expectedResource, res)
		})
	}
}
