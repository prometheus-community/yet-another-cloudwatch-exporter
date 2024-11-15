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

var apiGatewayV1 = &model.TaggedResource{
	ARN:       "arn:aws:apigateway:us-east-2::/restapis/test-api",
	Namespace: "AWS/ApiGateway",
}

var apiGatewayV1Stage = &model.TaggedResource{
	ARN:       "arn:aws:apigateway:us-east-2::/restapis/test-api/stages/test",
	Namespace: "AWS/ApiGateway",
}

var apiGatewayV2 = &model.TaggedResource{
	ARN:       "arn:aws:apigateway:us-east-2::/apis/98765fghij",
	Namespace: "AWS/ApiGateway",
}

var apiGatewayV2Stage = &model.TaggedResource{
	ARN:       "arn:aws:apigateway:us-east-2::/apis/98765fghij/stages/$default",
	Namespace: "AWS/ApiGateway",
}

var apiGatewayResources = []*model.TaggedResource{apiGatewayV1, apiGatewayV1Stage, apiGatewayV2, apiGatewayV2Stage}

func TestAssociatorAPIGateway(t *testing.T) {
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
			name: "should match API Gateway V2 with ApiId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ApiGateway").ToModelDimensionsRegexp(),
				resources:        apiGatewayResources,
				metric: &model.Metric{
					MetricName: "5xx",
					Namespace:  "AWS/ApiGateway",
					Dimensions: []model.Dimension{
						{Name: "ApiId", Value: "98765fghij"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: apiGatewayV2,
		},
		{
			name: "should match API Gateway V2 with ApiId and Stage dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ApiGateway").ToModelDimensionsRegexp(),
				resources:        apiGatewayResources,
				metric: &model.Metric{
					MetricName: "5xx",
					Namespace:  "AWS/ApiGateway",
					Dimensions: []model.Dimension{
						{Name: "ApiId", Value: "98765fghij"},
						{Name: "Stage", Value: "$default"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: apiGatewayV2Stage,
		},
		{
			name: "should match API Gateway V1 with ApiName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ApiGateway").ToModelDimensionsRegexp(),
				resources:        apiGatewayResources,
				metric: &model.Metric{
					MetricName: "5xx",
					Namespace:  "AWS/ApiGateway",
					Dimensions: []model.Dimension{
						{Name: "ApiName", Value: "test-api"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: apiGatewayV1,
		},
		{
			name: "should match API Gateway V1 with ApiName and Stage dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ApiGateway").ToModelDimensionsRegexp(),
				resources:        apiGatewayResources,
				metric: &model.Metric{
					MetricName: "5xx",
					Namespace:  "AWS/ApiGateway",
					Dimensions: []model.Dimension{
						{Name: "ApiName", Value: "test-api"},
						{Name: "Stage", Value: "test"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: apiGatewayV1Stage,
		},
		{
			name: "should match API Gateway V1 with ApiName (Stage is not matched)",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/ApiGateway").ToModelDimensionsRegexp(),
				resources:        apiGatewayResources,
				metric: &model.Metric{
					MetricName: "5xx",
					Namespace:  "AWS/ApiGateway",
					Dimensions: []model.Dimension{
						{Name: "ApiName", Value: "test-api"},
						{Name: "Stage", Value: "dev"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: apiGatewayV1,
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
