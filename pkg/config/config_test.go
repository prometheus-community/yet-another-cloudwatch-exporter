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
package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
)

func TestConfLoad(t *testing.T) {
	testCases := []struct {
		configFile string
	}{
		{configFile: "config_test.yml"},
		{configFile: "empty_rolearn.ok.yml"},
		{configFile: "sts_region.ok.yml"},
		{configFile: "multiple_roles.ok.yml"},
		{configFile: "custom_namespace.ok.yml"},
	}
	for _, tc := range testCases {
		config := ScrapeConf{}
		configFile := fmt.Sprintf("testdata/%s", tc.configFile)
		if _, err := config.Load(configFile, promslog.NewNopLogger()); err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
}

func TestBadConfigs(t *testing.T) {
	testCases := []struct {
		configFile string
		errorMsg   string
	}{
		{
			configFile: "externalid_without_rolearn.bad.yml",
			errorMsg:   "RoleArn should not be empty",
		},
		{
			configFile: "externalid_with_empty_rolearn.bad.yml",
			errorMsg:   "RoleArn should not be empty",
		},
		{
			configFile: "unknown_version.bad.yml",
			errorMsg:   "unknown apiVersion value 'invalidVersion'",
		},
		{
			configFile: "custom_namespace_without_name.bad.yml",
			errorMsg:   "Name should not be empty",
		},
		{
			configFile: "custom_namespace_without_namespace.bad.yml",
			errorMsg:   "Namespace should not be empty",
		},
		{
			configFile: "custom_namespace_without_region.bad.yml",
			errorMsg:   "Regions should not be empty",
		},
		{
			configFile: "discovery_job_type_unknown.bad.yml",
			errorMsg:   "Discovery job [0]: Service is not in known list!: AWS/FancyNewNamespace",
		},
		{
			configFile: "discovery_job_type_alias.bad.yml",
			errorMsg:   "Discovery job [0]: Invalid 'type' field, use namespace \"AWS/S3\" rather than alias \"s3\"",
		},
		{
			configFile: "discovery_job_exported_tags_alias.bad.yml",
			errorMsg:   "Discovery jobs: Invalid key in 'exportedTagsOnMetrics', use namespace \"AWS/S3\" rather than alias \"s3\"",
		},
		{
			configFile: "discovery_job_exported_tags_mismatch.bad.yml",
			errorMsg:   "Discovery jobs: 'exportedTagsOnMetrics' key \"AWS/RDS\" does not match with any discovery job type",
		},
	}

	for _, tc := range testCases {
		config := ScrapeConf{}
		configFile := fmt.Sprintf("testdata/%s", tc.configFile)
		if _, err := config.Load(configFile, promslog.NewNopLogger()); err != nil {
			if !strings.Contains(err.Error(), tc.errorMsg) {
				t.Errorf("expecter error for config file %q to contain %q but got: %s", tc.configFile, tc.errorMsg, err)
				t.FailNow()
			}
		} else {
			t.Log("expected validation error")
			t.FailNow()
		}
	}
}

func TestValidateConfigFailuresWhenUsingAsLibrary(t *testing.T) {
	type testcase struct {
		config   ScrapeConf
		errorMsg string
	}
	testCases := map[string]testcase{
		"empty role should be configured when environment role is desired": {
			config: ScrapeConf{
				APIVersion: "v1alpha1",
				StsRegion:  "us-east-2",
				Discovery: Discovery{
					Jobs: []*Job{{
						Regions: []string{"us-east-2"},
						Type:    "AWS/SQS",
						Metrics: []*Metric{{
							Name:       "NumberOfMessagesSent",
							Statistics: []string{"Average"},
						}},
					}},
				},
			},
			errorMsg: "no IAM roles configured. If the current IAM role is desired, an empty Role should be configured",
		},
		"enhanced metric are not supported for the namespace": {
			config: ScrapeConf{
				Discovery: Discovery{
					Jobs: []*Job{{
						Regions: []string{"us-east-2"},
						Type:    "AWS/S3",
						Roles:   []Role{{RoleArn: "arn:aws:iam::123456789012:role/test"}},
						Metrics: []*Metric{{
							Name:       "BucketSizeBytes",
							Statistics: []string{"Average"},
						}},
						EnhancedMetrics: []*EnhancedMetric{{
							Name: "SomeEnhancedMetric",
						}},
					}},
				},
			},
			errorMsg: "Discovery job [AWS/S3/0]: enhanced metrics are not supported for this namespace: enhanced metrics service for namespace AWS/S3 not found",
		},
		"enhanced metric are not supported for the enhanced mertrics service": {
			config: ScrapeConf{
				Discovery: Discovery{
					Jobs: []*Job{{
						Regions: []string{"us-east-2"},
						Type:    "AWS/Lambda",
						Roles:   []Role{{RoleArn: "arn:aws:iam::123456789012:role/test"}},
						Metrics: []*Metric{{
							Name:       "BucketSizeBytes",
							Statistics: []string{"Average"},
						}},
						EnhancedMetrics: []*EnhancedMetric{{
							Name: "SomeEnhancedMetric",
						}},
					}},
				},
			},
			errorMsg: "Discovery job [AWS/Lambda/0]: enhanced metric \"SomeEnhancedMetric\" is not supported for this namespace",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := tc.config.Validate(promslog.NewNopLogger())
			require.Error(t, err, "Expected config validation to fail")
			require.Equal(t, tc.errorMsg, err.Error())
		})
	}
}
