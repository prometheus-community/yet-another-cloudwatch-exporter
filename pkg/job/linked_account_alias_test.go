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
package job

import (
	"context"
	"fmt"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
)

type mockOAMClient struct {
	accounts map[string]string
	err      error
}

func (m *mockOAMClient) ListLinkedAccounts(_ context.Context, _ string) (map[string]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.accounts, nil
}

func TestLinkedAccountAliasResolver_Resolve(t *testing.T) {
	oamClient := &mockOAMClient{
		accounts: map[string]string{
			"111111111111": "dev-account",
			"222222222222": "prod-account",
		},
	}
	resolver := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "arn:aws:oam:us-west-2:123456789012:sink/test")
	ctx := context.Background()

	got := resolver.Resolve(ctx, "111111111111")
	require.Equal(t, "dev-account", got)

	got = resolver.Resolve(ctx, "222222222222")
	require.Equal(t, "prod-account", got)

	// Cached result
	got = resolver.Resolve(ctx, "111111111111")
	require.Equal(t, "dev-account", got)
}

func TestLinkedAccountAliasResolver_ResolveEmpty(t *testing.T) {
	oamClient := &mockOAMClient{
		accounts: map[string]string{},
	}
	resolver := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "arn:aws:oam:us-west-2:123456789012:sink/test")

	// Empty account ID returns empty string
	got := resolver.Resolve(context.Background(), "")
	require.Equal(t, "", got)
}

func TestLinkedAccountAliasResolver_ResolveError(t *testing.T) {
	oamClient := &mockOAMClient{
		err: fmt.Errorf("OAM API error"),
	}
	resolver := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "arn:aws:oam:us-west-2:123456789012:sink/test")

	got := resolver.Resolve(context.Background(), "333333333333")
	require.Equal(t, "", got)

	// Cached empty result on error
	got = resolver.Resolve(context.Background(), "333333333333")
	require.Equal(t, "", got)
}

func TestLinkedAccountAliasResolver_ResolveUnknownAccount(t *testing.T) {
	oamClient := &mockOAMClient{
		accounts: map[string]string{
			"111111111111": "dev-account",
		},
	}
	resolver := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "arn:aws:oam:us-west-2:123456789012:sink/test")

	// Known account
	got := resolver.Resolve(context.Background(), "111111111111")
	require.Equal(t, "dev-account", got)

	// Unknown account returns empty string
	got = resolver.Resolve(context.Background(), "999999999999")
	require.Equal(t, "", got)
}

func TestLinkedAccountAliasResolver_NilResolver(t *testing.T) {
	var resolver *linkedAccountAliasResolver
	got := resolver.Resolve(context.Background(), "111111111111")
	require.Equal(t, "", got)
}
