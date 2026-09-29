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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/valkey-io/valkey-go"
)

// CacheTTL is the default TTL for cached tagging API responses
const CacheTTL = 10 * time.Minute

// ResourceTagMappingCache represents the cached response from AWS GetResources API
type ResourceTagMappingCache struct {
	ResourceARN string      `json:"resource_arn"`
	Tags        []CachedTag `json:"tags"`
}

// CachedTag is a single resource tag. Tags are kept as a list to preserve the
// order returned by AWS, so cached and uncached discovery behave identically.
type CachedTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Cache defines the interface for caching tagging API responses
type Cache interface {
	// Get retrieves cached resource tag mappings for the given cache key
	Get(ctx context.Context, key string) ([]ResourceTagMappingCache, error)
	// Set stores resource tag mappings with the given cache key and TTL
	Set(ctx context.Context, key string, mappings []ResourceTagMappingCache, ttl time.Duration) error
	// Close closes the cache connection
	Close()
}

// valkeyBackend is a narrow interface around Valkey operations used by this cache.
// It exists to make ValkeyCache unit-testable without a real Valkey server.
type valkeyBackend interface {
	Get(ctx context.Context, key string) (string, error)
	SetEX(ctx context.Context, key string, value string, ttl time.Duration) error
	Close()
}

type valkeyBackendAdapter struct {
	client valkey.Client
}

func (a *valkeyBackendAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Do(ctx, a.client.B().Get().Key(key).Build()).ToString()
}

func (a *valkeyBackendAdapter) SetEX(ctx context.Context, key string, value string, ttl time.Duration) error {
	return a.client.Do(ctx, a.client.B().Set().Key(key).Value(value).Ex(ttl).Build()).Error()
}

func (a *valkeyBackendAdapter) Close() {
	a.client.Close()
}

// ValkeyCache implements the Cache interface using Valkey
type ValkeyCache struct {
	backend valkeyBackend
	logger  *slog.Logger
}

// ValkeyConfig holds configuration for connecting to Valkey
type ValkeyConfig struct {
	// Address is the Valkey server address (e.g., "localhost:6379")
	Address string
	// Username for authentication (optional)
	Username string
	// Password for authentication (optional)
	Password string
	// DB is the database number to use (default 0)
	DB int
}

// NewValkeyCache creates a new ValkeyCache instance
func NewValkeyCache(ctx context.Context, logger *slog.Logger, cfg ValkeyConfig) (*ValkeyCache, error) {
	opts := valkey.ClientOption{
		InitAddress:  []string{cfg.Address},
		Username:     cfg.Username,
		Password:     cfg.Password,
		SelectDB:     cfg.DB,
		DisableCache: true,
		TLSConfig: &tls.Config{
			// If you use a public CA or system trust store:
			// RootCAs: nil,
			//
			// For self-signed / custom CA, load it into RootCAs here.
			//
			// If Valkey requires client certs (mTLS), set Certificates:
			// Certificates: []tls.Certificate{clientCert},
			//
			MinVersion: tls.VersionTLS12,
		},
	}

	client, err := valkey.NewClient(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create valkey client: %w", err)
	}

	// Test connection
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to valkey: %w", err)
	}

	logger.Info("Connected to Valkey cache", "address", cfg.Address)

	return &ValkeyCache{
		backend: &valkeyBackendAdapter{client: client},
		logger:  logger,
	}, nil
}

// Get retrieves cached resource tag mappings for the given cache key
func (c *ValkeyCache) Get(ctx context.Context, key string) ([]ResourceTagMappingCache, error) {
	result, err := c.backend.Get(ctx, key)
	if err != nil {
		if valkey.IsValkeyNil(err) {
			c.logger.Debug("Cache miss", "key", key)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var mappings []ResourceTagMappingCache
	if err := json.Unmarshal([]byte(result), &mappings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached data: %w", err)
	}

	c.logger.Debug("Cache hit", "key", key, "count", len(mappings))
	return mappings, nil
}

// Set stores resource tag mappings with the given cache key and TTL
func (c *ValkeyCache) Set(ctx context.Context, key string, mappings []ResourceTagMappingCache, ttl time.Duration) error {
	data, err := json.Marshal(mappings)
	if err != nil {
		return fmt.Errorf("failed to marshal data for cache: %w", err)
	}

	err = c.backend.SetEX(ctx, key, string(data), ttl)
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	c.logger.Debug("Cache set", "key", key, "count", len(mappings), "ttl", ttl)
	return nil
}

// Close closes the Valkey connection
func (c *ValkeyCache) Close() {
	c.backend.Close()
}

// BuildCacheKey creates a cache key from resource filters and tag filter keys
// The key format is: "yace:tagging:<region>:<sorted_filters>:<sorted_tag_keys>"
// This ensures that the same filters and tag keys always produce the same cache key
func BuildCacheKey(region string, resourceFilters []string, tagFilterKeys []string) string {
	// Sort filters for consistent key generation
	sortedFilters := make([]string, len(resourceFilters))
	copy(sortedFilters, resourceFilters)
	sort.Strings(sortedFilters)

	sortedTagKeys := make([]string, len(tagFilterKeys))
	copy(sortedTagKeys, tagFilterKeys)
	sort.Strings(sortedTagKeys)

	return fmt.Sprintf("yace:tagging:%s:%s:%s",
		region,
		strings.Join(sortedFilters, ","),
		strings.Join(sortedTagKeys, ","))
}
