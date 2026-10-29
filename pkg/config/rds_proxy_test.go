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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRDSProxyDimensionRegexpUsesCloudWatchDimensionName(t *testing.T) {
	service := SupportedServices.GetService("AWS/RDS")
	require.NotNil(t, service)

	const proxyARN = "arn:aws:rds:us-east-1:123456789012:db-proxy:example-proxy"

	var matched bool
	for _, dimensionRegexp := range service.ToModelDimensionsRegexp() {
		if dimensionRegexp.Regexp.MatchString(proxyARN) {
			matched = true
			require.Equal(t, []string{"ProxyName"}, dimensionRegexp.DimensionsNames)
			require.Equal(t, []string{"ProxyName", "example-proxy"}, dimensionRegexp.Regexp.FindStringSubmatch(proxyARN))
		}
	}

	require.True(t, matched, "RDS Proxy ARN should match a dimension regexp")
}
