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
	"testing"
	"time"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestCalculateJitter(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *model.JitterConfig
		wantMin   time.Duration
		wantMax   time.Duration
		wantExact *time.Duration
	}{
		{
			name:      "nil config returns 0",
			cfg:       nil,
			wantExact: durationPtr(0),
		},
		{
			name: "both zero returns 0",
			cfg: &model.JitterConfig{
				MinDelay: 0,
				MaxDelay: 0,
			},
			wantExact: durationPtr(0),
		},
		{
			name: "same min and max returns exact value",
			cfg: &model.JitterConfig{
				MinDelay: 500 * time.Millisecond,
				MaxDelay: 500 * time.Millisecond,
			},
			wantExact: durationPtr(500 * time.Millisecond),
		},
		{
			name: "range returns value between min and max",
			cfg: &model.JitterConfig{
				MinDelay: 400 * time.Millisecond,
				MaxDelay: 600 * time.Millisecond,
			},
			wantMin: 400 * time.Millisecond,
			wantMax: 600 * time.Millisecond,
		},
		{
			name: "only max set",
			cfg: &model.JitterConfig{
				MinDelay: 0,
				MaxDelay: 1 * time.Second,
			},
			wantMin: 0,
			wantMax: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to test randomness when applicable
			iterations := 1
			if tt.wantExact == nil {
				iterations = 100
			}

			for i := 0; i < iterations; i++ {
				got := calculateJitter(tt.cfg)

				if tt.wantExact != nil {
					if got != *tt.wantExact {
						t.Errorf("calculateJitter() = %v, want %v", got, *tt.wantExact)
					}
				} else {
					if got < tt.wantMin || got > tt.wantMax {
						t.Errorf("calculateJitter() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
					}
				}
			}
		})
	}
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
