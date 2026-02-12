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
	"log/slog"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/oam"
)

type linkedAccountAliasResolver struct {
	logger         *slog.Logger
	oamClient      oam.Client
	sinkIdentifier string
	aliases        map[string]string
	loaded         bool
	mu             sync.Mutex
}

func newLinkedAccountAliasResolver(logger *slog.Logger, oamClient oam.Client, sinkIdentifier string) *linkedAccountAliasResolver {
	return &linkedAccountAliasResolver{
		logger:         logger,
		oamClient:      oamClient,
		sinkIdentifier: sinkIdentifier,
	}
}

func (r *linkedAccountAliasResolver) Resolve(ctx context.Context, accountID string) string {
	if r == nil || accountID == "" {
		return ""
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.loaded {
		r.loadAliases(ctx)
	}

	return r.aliases[accountID]
}

func (r *linkedAccountAliasResolver) loadAliases(ctx context.Context) {
	aliases, err := r.oamClient.ListLinkedAccounts(ctx, r.sinkIdentifier)
	if err != nil {
		r.logger.Warn("Failed to list linked accounts from OAM", "err", err, "sink_identifier", r.sinkIdentifier)
		r.aliases = map[string]string{}
	} else {
		r.aliases = aliases
	}
	r.loaded = true
}
