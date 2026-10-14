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
package config

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/grafana/regexp"
	"github.com/stretchr/testify/require"
)

// TestToModelDimensionsRegexpDoubleUnderscoreBecomesHyphen verifies that double
// underscores in regex named groups are decoded as hyphens in dimension names,
// allowing dimension names like "Per-VPC-Metrics" to be encoded as
// "Per__VPC__Metrics" in a Go regex named group.
func TestToModelDimensionsRegexpDoubleUnderscoreBecomesHyphen(t *testing.T) {
	sc := ServiceConfig{
		Namespace: "test",
		Alias:     "test",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("vpc/(?P<Per__VPC__Metrics>[^/]+)"),
		},
	}
	dr := sc.ToModelDimensionsRegexp()
	require.Len(t, dr, 1)
	require.Equal(t, []string{"Per-VPC-Metrics"}, dr[0].DimensionsNames)
}

// TestEC2ServiceConfigIncludesVPC verifies that the AWS/EC2 service config
// discovers both EC2 instances and VPCs, enabling resolution of VPC-scoped
// metrics like NetworkAddressUsage that use the Per-VPC-Metrics dimension.
func TestEC2ServiceConfigIncludesVPC(t *testing.T) {
	svc := SupportedServices.GetService("AWS/EC2")
	require.NotNil(t, svc)

	filters := make([]string, 0, len(svc.ResourceFilters))
	for _, f := range svc.ResourceFilters {
		filters = append(filters, aws.ToString(f))
	}
	require.Contains(t, filters, "ec2:vpc", "expected ec2:vpc in AWS/EC2 ResourceFilters")

	dr := svc.ToModelDimensionsRegexp()
	var allDimensionNames []string
	for _, d := range dr {
		allDimensionNames = append(allDimensionNames, d.DimensionsNames...)
	}
	require.Contains(t, allDimensionNames, "Per-VPC-Metrics", "expected 'Per-VPC-Metrics' dimension in AWS/EC2 service config")
}

func TestSupportedServices(t *testing.T) {
	for i, svc := range SupportedServices {
		require.NotNil(t, svc.Namespace, fmt.Sprintf("Nil Namespace for service at index '%d'", i))
		require.NotNil(t, svc.Alias, fmt.Sprintf("Nil Alias for service '%s' at index '%d'", svc.Namespace, i))

		if svc.ResourceFilters != nil {
			require.NotEmpty(t, svc.ResourceFilters)

			for _, filter := range svc.ResourceFilters {
				require.NotEmpty(t, aws.ToString(filter))
			}
		}

		if svc.DimensionRegexps != nil {
			require.NotEmpty(t, svc.DimensionRegexps)

			for _, regex := range svc.DimensionRegexps {
				require.NotEmpty(t, regex.String())
				require.Positive(t, regex.NumSubexp())
			}
		}
	}
}
