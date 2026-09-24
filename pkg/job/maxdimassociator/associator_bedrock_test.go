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

var bedrockApplicationInferenceProfile = &model.TaggedResource{
	ARN:       "p152fiotth4d/var-review-agent",
	Namespace: "AWS/Bedrock",
}

var bedrockResources = []*model.TaggedResource{
	bedrockApplicationInferenceProfile,
}

func TestAssociatorBedrock(t *testing.T) {
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
			name: "should match application inference profile with ModelId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock").ToModelDimensionsRegexp(),
				resources:        bedrockResources,
				metric: &model.Metric{
					MetricName: "InputTokenCount",
					Namespace:  "AWS/Bedrock",
					Dimensions: []model.Dimension{
						{Name: "ModelId", Value: "p152fiotth4d"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: bedrockApplicationInferenceProfile,
		},
		{
			name: "should skip foundation model ModelId not backed by a tagged resource",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock").ToModelDimensionsRegexp(),
				resources:        bedrockResources,
				metric: &model.Metric{
					MetricName: "InputTokenCount",
					Namespace:  "AWS/Bedrock",
					Dimensions: []model.Dimension{
						{Name: "ModelId", Value: "anthropic.claude-3-haiku-20240307-v1:0"},
					},
				},
			},
			expectedSkip:     true,
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
