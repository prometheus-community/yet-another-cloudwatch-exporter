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
package tagging

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
)

type fakeValkeyBackend struct {
	getFn   func(ctx context.Context, key string) (string, error)
	setExFn func(ctx context.Context, key string, value string, ttl time.Duration) error
}

func (b fakeValkeyBackend) Get(ctx context.Context, key string) (string, error) {
	if b.getFn == nil {
		return "", valkey.Nil
	}
	return b.getFn(ctx, key)
}

func (b fakeValkeyBackend) SetEX(ctx context.Context, key string, value string, ttl time.Duration) error {
	if b.setExFn == nil {
		return nil
	}
	return b.setExFn(ctx, key, value, ttl)
}

func (b fakeValkeyBackend) Close() {}

// newMapBackedCache returns a ValkeyCache backed by an in-memory map.
func newMapBackedCache(ttl time.Duration) (*ValkeyCache, map[string]string) {
	stored := map[string]string{}
	return &ValkeyCache{
		backend: fakeValkeyBackend{
			getFn: func(_ context.Context, key string) (string, error) {
				value, ok := stored[key]
				if !ok {
					return "", valkey.Nil
				}
				return value, nil
			},
			setExFn: func(_ context.Context, key string, value string, _ time.Duration) error {
				stored[key] = value
				return nil
			},
		},
		logger: slog.New(slog.DiscardHandler),
		ttl:    ttl,
	}, stored
}

func TestValkeyCache_GetMissIsNotFound(t *testing.T) {
	cache, _ := newMapBackedCache(time.Minute)

	mappings, found, err := cache.Get(context.Background(), "missing")
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, mappings)
}

func TestValkeyCache_SetThenGetRoundTrip(t *testing.T) {
	cache, _ := newMapBackedCache(time.Minute)

	key := "somekey"
	original := []ResourceTagMappingCache{
		{ResourceARN: "arn:aws:ec2:us-east-1:123456789012:instance/i-123", Tags: []CachedTag{{Key: "env", Value: "prod"}}},
		{ResourceARN: "arn:aws:s3:::bucket", Tags: []CachedTag{{Key: "team", Value: "core"}, {Key: "app", Value: "yace"}}},
	}

	require.NoError(t, cache.Set(context.Background(), key, original))
	loaded, found, err := cache.Get(context.Background(), key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, original, loaded)
}

func TestValkeyCache_EmptyResultIsAHit(t *testing.T) {
	for name, empty := range map[string][]ResourceTagMappingCache{
		"nil slice":   nil,
		"empty slice": {},
	} {
		t.Run(name, func(t *testing.T) {
			cache, stored := newMapBackedCache(time.Minute)

			require.NoError(t, cache.Set(context.Background(), "empty", empty))
			require.Equal(t, "[]", stored["empty"])

			mappings, found, err := cache.Get(context.Background(), "empty")
			require.NoError(t, err)
			require.True(t, found, "an empty cached result must be a hit, not a miss")
			require.Empty(t, mappings)
		})
	}
}

func TestValkeyCache_SetUsesConfiguredTTL(t *testing.T) {
	var gotTTL time.Duration
	cache := &ValkeyCache{
		backend: fakeValkeyBackend{setExFn: func(_ context.Context, _ string, _ string, ttl time.Duration) error {
			gotTTL = ttl
			return nil
		}},
		logger: slog.New(slog.DiscardHandler),
		ttl:    42 * time.Second,
	}

	require.NoError(t, cache.Set(context.Background(), "k", nil))
	require.Equal(t, 42*time.Second, gotTTL)
}

func TestValkeyCache_GetInvalidJSONReturnsError(t *testing.T) {
	cache := &ValkeyCache{
		backend: fakeValkeyBackend{getFn: func(_ context.Context, _ string) (string, error) {
			return "not-json", nil
		}},
		logger: slog.New(slog.DiscardHandler),
	}

	_, found, err := cache.Get(context.Background(), "bad")
	require.Error(t, err)
	require.False(t, found)
}

func TestValkeyCache_SetStoresJSON(t *testing.T) {
	cache, stored := newMapBackedCache(time.Minute)

	original := []ResourceTagMappingCache{{ResourceARN: "arn:aws:s3:::bucket", Tags: []CachedTag{{Key: "team", Value: "core"}}}}
	require.NoError(t, cache.Set(context.Background(), "k", original))

	var decoded []ResourceTagMappingCache
	require.NoError(t, json.Unmarshal([]byte(stored["k"]), &decoded))
	require.Equal(t, original, decoded)
}

func TestNewValkeyCache_RejectsNonPositiveTTL(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Second} {
		_, err := NewValkeyCache(context.Background(), slog.New(slog.DiscardHandler), ValkeyConfig{Address: "localhost:6379", TTL: ttl})
		require.ErrorContains(t, err, "TTL must be positive")
	}
}
