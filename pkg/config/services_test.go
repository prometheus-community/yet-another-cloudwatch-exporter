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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestSupportedServices(t *testing.T) {
	for i, svc := range SupportedServices {
		require.NotNil(t, svc.Namespace, fmt.Sprintf("Nil Namespace for service at index '%d'", i))
		require.NotNil(t, svc.Alias, fmt.Sprintf("Nil Alias for service '%s' at index '%d'", svc.Namespace, i))

		if svc.ResourceFilters != nil {
			require.NotEmpty(t, svc.ResourceFilters)

			for _, filter := range svc.ResourceFilters {
				require.NotEmpty(t, aws.StringValue(filter))
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
