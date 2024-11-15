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
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestYACEApp_FeatureFlagsParsedCorrectly(t *testing.T) {
	app := NewYACEApp()

	// two feature flags
	app.Action = func(c *cli.Context) error {
		featureFlags := c.StringSlice(enableFeatureFlag)
		require.Equal(t, []string{"feature1", "feature2"}, featureFlags)
		return nil
	}

	require.NoError(t, app.Run([]string{"yace", "-enable-feature=feature1,feature2"}), "error running test command")

	// empty feature flags
	app.Action = func(c *cli.Context) error {
		featureFlags := c.StringSlice(enableFeatureFlag)
		require.Len(t, featureFlags, 0)
		return nil
	}

	require.NoError(t, app.Run([]string{"yace"}), "error running test command")
}
