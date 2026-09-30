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
package maxdimassociator

import (
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var syntheticsCanary = &model.TaggedResource{
	ARN:       "arn:aws:synthetics:us-east-1:123456789012:canary:my-canary",
	Namespace: "CloudWatchSynthetics",
}

var syntheticsResources = []*model.TaggedResource{syntheticsCanary}

func TestAssociatorSynthetics(t *testing.T) {
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
			name: "should match with CanaryName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("CloudWatchSynthetics").ToModelDimensionsRegexp(),
				resources:        syntheticsResources,
				metric: &model.Metric{
					MetricName: "SuccessPercent",
					Namespace:  "CloudWatchSynthetics",
					Dimensions: []model.Dimension{
						{Name: "CanaryName", Value: "my-canary"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: syntheticsCanary,
		},
		{
			name: "should skip with unmatched CanaryName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("CloudWatchSynthetics").ToModelDimensionsRegexp(),
				resources:        syntheticsResources,
				metric: &model.Metric{
					MetricName: "SuccessPercent",
					Namespace:  "CloudWatchSynthetics",
					Dimensions: []model.Dimension{
						{Name: "CanaryName", Value: "another-canary"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should not skip when empty dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("CloudWatchSynthetics").ToModelDimensionsRegexp(),
				resources:        syntheticsResources,
				metric: &model.Metric{
					MetricName: "SuccessPercent",
					Namespace:  "CloudWatchSynthetics",
					Dimensions: []model.Dimension{},
				},
			},
			expectedSkip:     false,
			expectedResource: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			associator := NewAssociator(promslog.NewNopLogger(), tc.args.dimensionRegexps, tc.args.resources)
			res, skip := associator.AssociateMetricToResource(tc.args.metric)
			require.Equal(t, tc.expectedSkip, skip)
			require.Equal(t, tc.expectedResource, res)
		})
	}
}
