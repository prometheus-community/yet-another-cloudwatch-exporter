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

var keyspacesTable = &model.TaggedResource{
	ARN:       "arn:aws:cassandra:eu-west-1:123456789012:/keyspace/my_keyspace/table/my_table",
	Namespace: "AWS/Cassanrdra",
}

var cassandraResources = []*model.TaggedResource{
	keyspacesTable,
}

func TestAssociatorCassandra(t *testing.T) {
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
			name: "keyspace should match with clusterId dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Cassandra").ToModelDimensionsRegexp(),
				resources:        cassandraResources,
				metric: &model.Metric{
					MetricName: "BillableTableSizeInBytes",
					Namespace:  "AWS/Cassandra",
					Dimensions: []model.Dimension{
						{Name: "Keyspace", Value: "my_keyspace"},
						{Name: "TableName", Value: "my_table"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: keyspacesTable,
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
