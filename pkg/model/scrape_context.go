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
package model

import "context"

type scrapeContextKey struct{}

// CtxWithScrapeContext injects a ScrapeContext inside a given context.Context, so that
// lower layers (e.g. the CloudWatch client) can read the region and account being scraped.
func CtxWithScrapeContext(ctx context.Context, sc *ScrapeContext) context.Context {
	return context.WithValue(ctx, scrapeContextKey{}, sc)
}

// ScrapeContextFromCtx retrieves the ScrapeContext from a context.Context.
// The returned bool is false if none was injected.
func ScrapeContextFromCtx(ctx context.Context) (*ScrapeContext, bool) {
	sc, ok := ctx.Value(scrapeContextKey{}).(*ScrapeContext)
	return sc, ok && sc != nil
}
