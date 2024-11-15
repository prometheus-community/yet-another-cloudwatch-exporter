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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeatureFlagsInContext_DefaultsToNonEnabled(t *testing.T) {
	flags := FlagsFromCtx(context.Background())
	require.False(t, flags.IsFeatureEnabled("some-feature"))
	require.False(t, flags.IsFeatureEnabled("some-other-feature"))
}

type flags struct{}

func (f flags) IsFeatureEnabled(_ string) bool {
	return true
}

func TestFeatureFlagsInContext_RetrievesFlagsFromContext(t *testing.T) {
	ctx := CtxWithFlags(context.Background(), flags{})
	require.True(t, FlagsFromCtx(ctx).IsFeatureEnabled("some-feature"))
	require.True(t, FlagsFromCtx(ctx).IsFeatureEnabled("some-other-feature"))
}
