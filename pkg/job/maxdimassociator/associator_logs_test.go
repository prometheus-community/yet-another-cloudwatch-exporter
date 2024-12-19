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

var logGroup1 = &model.TaggedResource{
	ARN:       "arn:aws:logs:eu-central-1:123456789012:log-group:/aws/lambda/log-group-1",
	Namespace: "AWS/Logs",
}

var logGroup2 = &model.TaggedResource{
	ARN:       "arn:aws:logs:eu-central-1:123456789012:log-group:/custom/log-group-2",
	Namespace: "AWS/Logs",
}

var logGroupResources = []*model.TaggedResource{
	logGroup1,
	logGroup2,
}

func TestAssociatorLogs(t *testing.T) {
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
			name: "should match with log group one dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Logs").ToModelDimensionsRegexp(),
				resources:        logGroupResources,
				metric: &model.Metric{
					MetricName: "DeliveryThrottling",
					Namespace:  "AWS/Logs",
					Dimensions: []model.Dimension{
						{Name: "LogGroupName", Value: "/aws/lambda/log-group-1"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: logGroup1,
		},
		{
			name: "should match with log group two dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Logs").ToModelDimensionsRegexp(),
				resources:        logGroupResources,
				metric: &model.Metric{
					MetricName: "IncomingBytes",
					Namespace:  "AWS/Logs",
					Dimensions: []model.Dimension{
						{Name: "LogGroupName", Value: "/custom/log-group-2"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: logGroup2,
		},
		{
			name: "should not match with any log group",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Logs").ToModelDimensionsRegexp(),
				resources:        logGroupResources,
				metric: &model.Metric{
					MetricName: "ForwardingLogEvents",
					Namespace:  "AWS/Logs",
					Dimensions: []model.Dimension{
						{Name: "LogGroupName", Value: "/custom/nonexisting/log-group-3"},
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
