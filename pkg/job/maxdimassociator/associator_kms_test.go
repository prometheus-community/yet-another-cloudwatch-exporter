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
