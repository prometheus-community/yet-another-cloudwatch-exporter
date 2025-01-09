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

var dxVif = &model.TaggedResource{
	ARN:       "arn:aws:directconnect::012345678901:dxvif/dxvif-abc123",
	Namespace: "AWS/DX",
}

func TestAssociatorDX(t *testing.T) {
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
			name: "should match Virtual Interface with VirtualInterfaceId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/DX").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{dxVif},
				metric: &model.Metric{
					MetricName: "VirtualInterfaceBpsIngress",
					Namespace:  "AWS/DX",
					Dimensions: []model.Dimension{
						{Name: "ConnectionId", Value: "dxlag-abc123"},
						{Name: "VirtualInterfaceId", Value: "dxvif-abc123"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: dxVif,
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
