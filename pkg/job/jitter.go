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
	"math/rand"
	"time"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// calculateJitter returns a random duration between min and max (inclusive)
func calculateJitter(cfg *model.JitterConfig) time.Duration {
	if cfg == nil {
		return 0
	}

	minDelay := cfg.MinDelay
	maxDelay := cfg.MaxDelay

	// If they're the same, return that value
	if minDelay == maxDelay {
		return minDelay
	}

	// Calculate random value in range [minDelay, maxDelay]
	diff := maxDelay - minDelay
	return minDelay + time.Duration(rand.Int63n(int64(diff)+1))
}
