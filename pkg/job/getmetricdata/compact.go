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

// compact iterates over a slice of pointers and deletes
// unwanted elements as per the keep function return value.
// The slice is modified in-place without copying elements.
func compact[T any](input []*T, keep func(el *T) bool) []*T {
	// move all elements that must be kept at the beginning
	i := 0
	for _, d := range input {
		if keep(d) {
			input[i] = d
			i++
		}
	}
	// nil out any left element
	for j := i; j < len(input); j++ {
		input[j] = nil
	}
	// set new slice length to allow released elements to be collected
	return input[:i]
}
