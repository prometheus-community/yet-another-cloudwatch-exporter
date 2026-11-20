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
package job

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
)

type testOAMClient struct {
	accounts map[string]string
	err      error
	calls    int
}

func (c *testOAMClient) ListLinkedAccounts(_ context.Context, _ string) (map[string]string, error) {
	c.calls++
	return c.accounts, c.err
}

func TestLinkedAccountAliasResolver(t *testing.T) {
	t.Run("resolves aliases and loads them once", func(t *testing.T) {
		oamClient := &testOAMClient{accounts: map[string]string{"222222222222": "Account A"}}
		r := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "sink-arn")

		require.Equal(t, "Account A", r.Resolve(context.Background(), "222222222222"))
		require.Empty(t, r.Resolve(context.Background(), "333333333333"))
		require.Equal(t, 1, oamClient.calls)
	})

	t.Run("empty account ID doesn't load aliases", func(t *testing.T) {
		oamClient := &testOAMClient{}
		r := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "sink-arn")

		require.Empty(t, r.Resolve(context.Background(), ""))
		require.Equal(t, 0, oamClient.calls)
	})

	t.Run("OAM error results in empty aliases", func(t *testing.T) {
		oamClient := &testOAMClient{err: errors.New("access denied")}
		r := newLinkedAccountAliasResolver(promslog.NewNopLogger(), oamClient, "sink-arn")

		require.Empty(t, r.Resolve(context.Background(), "222222222222"))
		require.Empty(t, r.Resolve(context.Background(), "222222222222"))
		require.Equal(t, 1, oamClient.calls)
	})

	t.Run("nil resolver", func(t *testing.T) {
		var r *linkedAccountAliasResolver
		require.Empty(t, r.Resolve(context.Background(), "222222222222"))
	})
}
