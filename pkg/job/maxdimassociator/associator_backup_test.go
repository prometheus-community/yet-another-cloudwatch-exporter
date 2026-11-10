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

// Copy job metrics are published in the source vault's region, so the tagging filter for AWS/Backup returns both the
// source vault and the destination vault (which may live in another region) of every copy job sourced from the region.
var (
	backupSourceVault = &model.TaggedResource{
		ARN:       "arn:aws:backup:us-east-1:123456789012:backup-vault:source-vault",
		Namespace: "AWS/Backup",
		Region:    "us-east-1",
	}
	backupDestinationVault = &model.TaggedResource{
		ARN:       "arn:aws:backup:us-east-2:123456789012:backup-vault:destination-vault",
		Namespace: "AWS/Backup",
		Region:    "us-east-1",
	}
)

func TestAssociatorBackup(t *testing.T) {
	type testCase struct {
		name             string
		metric           *model.Metric
		expectedSkip     bool
		expectedResource *model.TaggedResource
	}

	testcases := []testCase{
		{
			name: "should match source backup vault with BackupVaultName dimension",
			metric: &model.Metric{
				MetricName: "NumberOfCopyJobsCompleted",
				Namespace:  "AWS/Backup",
				Dimensions: []model.Dimension{
					{Name: "BackupVaultName", Value: "source-vault"},
				},
			},
			expectedSkip:     false,
			expectedResource: backupSourceVault,
		},
		{
			name: "should match destination backup vault from another region with BackupVaultName dimension",
			metric: &model.Metric{
				MetricName: "NumberOfCopyJobsCompleted",
				Namespace:  "AWS/Backup",
				Dimensions: []model.Dimension{
					{Name: "BackupVaultName", Value: "destination-vault"},
				},
			},
			expectedSkip:     false,
			expectedResource: backupDestinationVault,
		},
	}

	logger := promslog.NewNopLogger()
	dimensionRegexps := config.SupportedServices.GetService("AWS/Backup").ToModelDimensionsRegexp()
	resources := []*model.TaggedResource{backupSourceVault, backupDestinationVault}
	associator := NewAssociator(logger, dimensionRegexps, resources)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			res, skip := associator.AssociateMetricToResource(tc.metric)
			require.Equal(t, tc.expectedSkip, skip)
			require.Equal(t, tc.expectedResource, res)
		})
	}
}
