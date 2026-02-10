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
package oam

import "context"

type Client interface {
	// ListLinkedAccounts calls OAM ListAttachedLinks for the given sink identifier
	// and returns a map of source account ID to resolved label.
	// Requires oam:ListAttachedLinks permission.
	ListLinkedAccounts(ctx context.Context, sinkIdentifier string) (map[string]string, error)
}
