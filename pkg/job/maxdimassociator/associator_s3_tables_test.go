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

var s3TablesBucket = &model.TaggedResource{
	ARN:       "arn:aws:s3tables:us-east-1:012345678901:bucket/my-table-bucket",
	Namespace: "AWS/S3/Tables",
}

var s3TablesOtherBucket = &model.TaggedResource{
	ARN:       "arn:aws:s3tables:us-east-1:012345678901:bucket/other-bucket",
	Namespace: "AWS/S3/Tables",
}

func TestAssociatorS3Tables(t *testing.T) {
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
			name: "should match table bucket with TableBucketName dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/S3/Tables").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{s3TablesOtherBucket, s3TablesBucket},
				metric: &model.Metric{
					MetricName: "AllRequests",
					Namespace:  "AWS/S3/Tables",
					Dimensions: []model.Dimension{
						{Name: "TableBucketName", Value: "my-table-bucket"},
						{Name: "Namespace", Value: "my-department"},
						{Name: "TableName", Value: "transactions"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: s3TablesBucket,
		},
		{
			name: "should skip metric for a table bucket that is not tagged",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/S3/Tables").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{s3TablesOtherBucket},
				metric: &model.Metric{
					MetricName: "AllRequests",
					Namespace:  "AWS/S3/Tables",
					Dimensions: []model.Dimension{
						{Name: "TableBucketName", Value: "my-table-bucket"},
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
