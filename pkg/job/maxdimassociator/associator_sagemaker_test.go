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
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var sagemakerEndpointInvocationOne = &model.TaggedResource{
	ARN:       "arn:aws:sagemaker:us-west-2:123456789012:endpoint/example-endpoint-one",
	Namespace: "AWS/SageMaker",
}

var sagemakerEndpointInvocationTwo = &model.TaggedResource{
	ARN:       "arn:aws:sagemaker:us-west-2:123456789012:endpoint/example-endpoint-two",
	Namespace: "AWS/SageMaker",
}

var sagemakerEndpointInvocationUpper = &model.TaggedResource{
	ARN:       "arn:aws:sagemaker:us-west-2:123456789012:endpoint/example-endpoint-upper",
	Namespace: "AWS/SageMaker",
}

var sagemakerInferenceComponentInvocationOne = &model.TaggedResource{
	ARN:       "arn:aws:sagemaker:us-west-2:123456789012:inference-component/example-inference-component-one",
	Namespace: "AWS/SageMaker",
}

var sagemakerInferenceComponentInvocationUpper = &model.TaggedResource{
	ARN:       "arn:aws:sagemaker:us-west-2:123456789012:inference-component/example-inference-component-upper",
	Namespace: "AWS/SageMaker",
}

var sagemakerInvocationResources = []*model.TaggedResource{
	sagemakerEndpointInvocationOne,
	sagemakerEndpointInvocationTwo,
	sagemakerEndpointInvocationUpper,
	sagemakerInferenceComponentInvocationOne,
	sagemakerInferenceComponentInvocationUpper,
}

func TestAssociatorSagemaker(t *testing.T) {
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
			name: "3 dimensions should match",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/SageMaker").ToModelDimensionsRegexp(),
				resources:        sagemakerInvocationResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/SageMaker",
					Dimensions: []model.Dimension{
						{Name: "EndpointName", Value: "example-endpoint-one"},
						{Name: "VariantName", Value: "example-endpoint-one-variant-one"},
						{Name: "EndpointConfigName", Value: "example-endpoint-one-endpoint-config"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: sagemakerEndpointInvocationOne,
		},
		{
			name: "2 dimensions should match",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/SageMaker").ToModelDimensionsRegexp(),
				resources:        sagemakerInvocationResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/SageMaker",
					Dimensions: []model.Dimension{
						{Name: "EndpointName", Value: "example-endpoint-two"},
						{Name: "VariantName", Value: "example-endpoint-two-variant-one"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: sagemakerEndpointInvocationTwo,
		},
		{
			name: "2 dimensions should not match",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/SageMaker").ToModelDimensionsRegexp(),
				resources:        sagemakerInvocationResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/SageMaker",
					Dimensions: []model.Dimension{
						{Name: "EndpointName", Value: "example-endpoint-three"},
						{Name: "VariantName", Value: "example-endpoint-three-variant-one"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "2 dimensions should match in Upper case",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/SageMaker").ToModelDimensionsRegexp(),
				resources:        sagemakerInvocationResources,
				metric: &model.Metric{
					MetricName: "ModelLatency",
					Namespace:  "AWS/SageMaker",
					Dimensions: []model.Dimension{
						{Name: "EndpointName", Value: "Example-Endpoint-Upper"},
						{Name: "VariantName", Value: "example-endpoint-two-variant-one"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: sagemakerEndpointInvocationUpper,
		},
		{
			name: "inference component match",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/SageMaker").ToModelDimensionsRegexp(),
				resources:        sagemakerInvocationResources,
				metric: &model.Metric{
					MetricName: "ModelLatency",
					Namespace:  "AWS/SageMaker",
					Dimensions: []model.Dimension{
						{Name: "InferenceComponentName", Value: "example-inference-component-one"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: sagemakerInferenceComponentInvocationOne,
		},
		{
			name: "inference component match in Upper case",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/SageMaker").ToModelDimensionsRegexp(),
				resources:        sagemakerInvocationResources,
				metric: &model.Metric{
					MetricName: "ModelLatency",
					Namespace:  "AWS/SageMaker",
					Dimensions: []model.Dimension{
						{Name: "InferenceComponentName", Value: "Example-Inference-Component-Upper"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: sagemakerInferenceComponentInvocationUpper,
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
