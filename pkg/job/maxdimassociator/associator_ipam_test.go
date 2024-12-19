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

var ec2IpamPool = &model.TaggedResource{
	ARN:       "arn:aws:ec2::123456789012:ipam-pool/ipam-pool-1ff5e4e9ad2c28b7b",
	Namespace: "AWS/IPAM",
}

var ipamResources = []*model.TaggedResource{
	ec2IpamPool,
}

func TestAssociatorIpam(t *testing.T) {
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
			name: "should match with IpamPoolId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/IPAM").ToModelDimensionsRegexp(),
				resources:        ipamResources,
				metric: &model.Metric{
					MetricName: "VpcIPUsage",
					Namespace:  "AWS/IPAM",
					Dimensions: []model.Dimension{
						{Name: "IpamPoolId", Value: "ipam-pool-1ff5e4e9ad2c28b7b"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: ec2IpamPool,
		},
		{
			name: "should skip with unmatched IpamPoolId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/IPAM").ToModelDimensionsRegexp(),
				resources:        ipamResources,
				metric: &model.Metric{
					MetricName: "VpcIPUsage",
					Namespace:  "AWS/IPAM",
					Dimensions: []model.Dimension{
						{Name: "IpamPoolId", Value: "ipam-pool-blahblah"},
					},
				},
			},
			expectedSkip:     true,
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
