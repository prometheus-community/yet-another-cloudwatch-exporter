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

import "time"

const TimeFormat = "2006-01-02T15:04:05.999999-07:00"

// Clock small interface which allows for stubbing the time.Now() function for unit testing
type Clock interface {
	Now() time.Time
}

// TimeClock implementation of Clock interface which delegates to Go's Time package
type TimeClock struct{}

func (tc TimeClock) Now() time.Time {
	return time.Now()
}

type MetricWindowCalculator struct {
	clock Clock
}

// Calculate computes the start and end time for the GetMetricData request to AWS
// Always uses the wall clock time as starting point for calculations to ensure that
// a variety of exporter configurations will work reliably.
func (m MetricWindowCalculator) Calculate(period time.Duration, length time.Duration, delay time.Duration) (time.Time, time.Time) {
	now := m.clock.Now()
	if period > 0 {
		// Round down the time to a factor of the period:
		// https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html#API_GetMetricData_RequestParameters
		now = now.Add(-period / 2).Round(period)
	}

	startTime := now.Add(-(length + delay))
	endTime := now.Add(-delay)
	return startTime, endTime
}
