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
package getmetricdata

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompact(t *testing.T) {
	type data struct {
		n int
	}

	type testCase struct {
		name        string
		input       []*data
		keepFunc    func(el *data) bool
		expectedRes []*data
	}

	testCases := []testCase{
		{
			name:        "empty",
			input:       []*data{},
			keepFunc:    nil,
			expectedRes: []*data{},
		},
		{
			name:        "one element input, one element result",
			input:       []*data{{n: 0}},
			keepFunc:    func(_ *data) bool { return true },
			expectedRes: []*data{{n: 0}},
		},
		{
			name:        "one element input, empty result",
			input:       []*data{{n: 0}},
			keepFunc:    func(_ *data) bool { return false },
			expectedRes: []*data{},
		},
		{
			name:        "two elements input, two elements result",
			input:       []*data{{n: 0}, {n: 1}},
			keepFunc:    func(_ *data) bool { return true },
			expectedRes: []*data{{n: 0}, {n: 1}},
		},
		{
			name:        "two elements input, one element result (first)",
			input:       []*data{{n: 0}, {n: 1}},
			keepFunc:    func(el *data) bool { return el.n == 1 },
			expectedRes: []*data{{n: 1}},
		},
		{
			name:        "two elements input, one element result (last)",
			input:       []*data{{n: 0}, {n: 1}},
			keepFunc:    func(el *data) bool { return el.n == 0 },
			expectedRes: []*data{{n: 0}},
		},
		{
			name:        "two elements input, empty result",
			input:       []*data{{n: 0}, {n: 1}},
			keepFunc:    func(_ *data) bool { return false },
			expectedRes: []*data{},
		},
		{
			name:        "three elements input, empty result",
			input:       []*data{{n: 0}, {n: 1}, {n: 2}},
			keepFunc:    func(el *data) bool { return el.n < 0 },
			expectedRes: []*data{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := compact(tc.input, tc.keepFunc)
			require.Equal(t, tc.expectedRes, res)
		})
	}
}
