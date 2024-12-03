package maxdimassociator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/model"
)

var directory = &model.TaggedResource{
	ARN:       "arn:aws:ds::012345678901:directory/d-abc123",
	Namespace: "AWS/DirectoryService",
}

func TestAssociatorDirectoryService(t *testing.T) {
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
			name: "should match directory id with Directory ID dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/DirectoryService").ToModelDimensionsRegexp(),
				resources:        []*model.TaggedResource{directory},
				metric: &model.Metric{
					MetricName: "Current Bandwidth",
					Namespace:  "AWS/DirectoryService",
					Dimensions: []model.Dimension{
						{Name: "Metric Category", Value: "NTDS"},
						{Name: "Domain Controller IP", Value: "123.123.123.123"},
						{Name: "Directory ID", Value: "d-abc123"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: directory,
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
