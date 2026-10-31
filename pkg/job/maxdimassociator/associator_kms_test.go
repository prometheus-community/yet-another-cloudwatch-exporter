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

var kmsKey = &model.TaggedResource{
	ARN:       "arn:aws:kms:us-east-2:123456789012:key/12345678-1234-1234-1234-123456789012",
	Namespace: "AWS/KMS",
}

func TestAssociatorKMS(t *testing.T) {
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
			name: "should match with KMS dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/KMS").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{kmsKey},
				metric: &model.Metric{
					MetricName: "SecondsUntilKeyMaterialExpiration",
					Namespace:  "AWS/KMS",
					Dimensions: []model.Dimension{
						{Name: "KeyId", Value: "12345678-1234-1234-1234-123456789012"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: kmsKey,
		},
		{
			name: "should match with KeyArn and Operation dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/KMS").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{kmsKey},
				metric: &model.Metric{
					MetricName: "SuccessfulRequest",
					Namespace:  "AWS/KMS",
					Dimensions: []model.Dimension{
						{Name: "KeyArn", Value: "arn:aws:kms:us-east-2:123456789012:key/12345678-1234-1234-1234-123456789012"},
						{Name: "Operation", Value: "Decrypt"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: kmsKey,
		},
		{
			name: "should skip with KeyArn of a key that was not discovered",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/KMS").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{kmsKey},
				metric: &model.Metric{
					MetricName: "SuccessfulRequest",
					Namespace:  "AWS/KMS",
					Dimensions: []model.Dimension{
						{Name: "KeyArn", Value: "arn:aws:kms:us-east-2:123456789012:key/87654321-4321-4321-4321-210987654321"},
						{Name: "Operation", Value: "Decrypt"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should not match with only an Operation dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/KMS").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{kmsKey},
				metric: &model.Metric{
					MetricName: "SuccessfulRequest",
					Namespace:  "AWS/KMS",
					Dimensions: []model.Dimension{
						{Name: "Operation", Value: "GenerateRandom"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			associator := NewAssociator(promslog.NewNopLogger(), tc.args.dimensionRegexps, tc.args.resources)
			dimensionsBefore := len(tc.args.metric.Dimensions)
			res, skip := associator.AssociateMetricToResource(tc.args.metric)
			require.Len(t, tc.args.metric.Dimensions, dimensionsBefore, "the metric dimensions must not be modified")
			require.Equal(t, tc.expectedSkip, skip)
			require.Equal(t, tc.expectedResource, res)
		})
	}
}
