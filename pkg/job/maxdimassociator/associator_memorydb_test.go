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

var memoryDBCluster1 = &model.TaggedResource{
	ARN:       "arn:aws:memorydb:us-east-1:123456789012:cluster/mycluster",
	Namespace: "AWS/MemoryDB",
}

var memoryDBCluster2 = &model.TaggedResource{
	ARN:       "arn:aws:memorydb:us-east-1:123456789012:cluster/othercluster",
	Namespace: "AWS/MemoryDB",
}

var memoryDBClusters = []*model.TaggedResource{
	memoryDBCluster1,
	memoryDBCluster2,
}

func TestAssociatorMemoryDB(t *testing.T) {
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
			name: "should match with ClusterName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/MemoryDB").ToModelDimensionsRegexp(),
				resources:        memoryDBClusters,
				metric: &model.Metric{
					Namespace:  "AWS/MemoryDB",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "ClusterName", Value: "mycluster"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: memoryDBCluster1,
		},
		{
			name: "should match another instance with ClusterName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/MemoryDB").ToModelDimensionsRegexp(),
				resources:        memoryDBClusters,
				metric: &model.Metric{
					Namespace:  "AWS/MemoryDB",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "ClusterName", Value: "othercluster"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: memoryDBCluster2,
		},
		{
			name: "should skip with unmatched ClusterName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/MemoryDB").ToModelDimensionsRegexp(),
				resources:        memoryDBClusters,
				metric: &model.Metric{
					Namespace:  "AWS/MemoryDB",
					MetricName: "CPUUtilization",
					Dimensions: []model.Dimension{
						{Name: "ClusterName", Value: "blahblah"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should not skip when unmatching because of non-ARN dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/MemoryDB").ToModelDimensionsRegexp(),
				resources:        memoryDBClusters,
				metric: &model.Metric{
					Namespace:  "AWS/MemoryDB",
					MetricName: "BytesUsedForMemoryDB",
					Dimensions: []model.Dimension{
						{Name: "OtherName", Value: "some-other-value"},
					},
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
