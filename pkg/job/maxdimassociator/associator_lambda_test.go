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

var lambdaFunction = &model.TaggedResource{
	ARN:       "arn:aws:lambda:us-east-2:123456789012:function:lambdaFunction",
	Namespace: "AWS/Lambda",
}

var lambdaResources = []*model.TaggedResource{lambdaFunction}

func TestAssociatorLambda(t *testing.T) {
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
			name: "should match with FunctionName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Lambda").ToModelDimensionsRegexp(),
				resources:        lambdaResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Lambda",
					Dimensions: []model.Dimension{
						{Name: "FunctionName", Value: "lambdaFunction"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: lambdaFunction,
		},
		{
			name: "should skip with unmatched FunctionName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Lambda").ToModelDimensionsRegexp(),
				resources:        lambdaResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Lambda",
					Dimensions: []model.Dimension{
						{Name: "FunctionName", Value: "anotherLambdaFunction"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should match with FunctionName and Resource dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Lambda").ToModelDimensionsRegexp(),
				resources:        lambdaResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Lambda",
					Dimensions: []model.Dimension{
						{Name: "FunctionName", Value: "lambdaFunction"},
						{Name: "Resource", Value: "lambdaFunction"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: lambdaFunction,
		},
		{
			name: "should not skip when empty dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Lambda").ToModelDimensionsRegexp(),
				resources:        lambdaResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Lambda",
					Dimensions: []model.Dimension{},
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
