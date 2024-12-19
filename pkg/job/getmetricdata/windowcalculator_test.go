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
	"time"
)

// StubClock stub implementation of Clock interface that allows tests
// to control time.Now()
type StubClock struct {
	currentTime time.Time
}

func (mt StubClock) Now() time.Time {
	return mt.currentTime
}

func Test_MetricWindow(t *testing.T) {
	type data struct {
		roundingPeriod    time.Duration
		length            time.Duration
		delay             time.Duration
		clock             StubClock
		expectedStartTime time.Time
		expectedEndTime   time.Time
	}

	testCases := []struct {
		testName string
		data     data
	}{
		{
			testName: "Go back four minutes and round to the nearest two minutes with two minute delay",
			data: data{
				roundingPeriod: 120 * time.Second,
				length:         120 * time.Second,
				delay:          120 * time.Second,
				clock: StubClock{
					currentTime: time.Date(2021, 11, 20, 0, 0, 0, 0, time.UTC),
				},
				expectedStartTime: time.Date(2021, 11, 19, 23, 56, 0, 0, time.UTC),
				expectedEndTime:   time.Date(2021, 11, 19, 23, 58, 0, 0, time.UTC),
			},
		},
		{
			testName: "Go back four minutes with two minute delay nad no rounding",
			data: data{
				roundingPeriod: 0,
				length:         120 * time.Second,
				delay:          120 * time.Second,
				clock: StubClock{
					currentTime: time.Date(2021, 1, 1, 0, 0o2, 22, 33, time.UTC),
				},
				expectedStartTime: time.Date(2020, 12, 31, 23, 58, 22, 33, time.UTC),
				expectedEndTime:   time.Date(2021, 1, 1, 0, 0, 22, 33, time.UTC),
			},
		},
		{
			testName: "Go back two days and round to the nearest day (midnight) with zero delay",
			data: data{
				roundingPeriod: 86400 * time.Second,  // 1 day
				length:         172800 * time.Second, // 2 days
				delay:          0,
				clock: StubClock{
					currentTime: time.Date(2021, 11, 20, 8, 33, 44, 0, time.UTC),
				},
				expectedStartTime: time.Date(2021, 11, 18, 0, 0, 0, 0, time.UTC),
				expectedEndTime:   time.Date(2021, 11, 20, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			testName: "Go back two days and round to the nearest 5 minutes with zero delay",
			data: data{
				roundingPeriod: 300 * time.Second,    // 5 min
				length:         172800 * time.Second, // 2 days
				delay:          0,
				clock: StubClock{
					currentTime: time.Date(2021, 11, 20, 8, 33, 44, 0, time.UTC),
				},
				expectedStartTime: time.Date(2021, 11, 18, 8, 30, 0, 0, time.UTC),
				expectedEndTime:   time.Date(2021, 11, 20, 8, 30, 0, 0, time.UTC),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			startTime, endTime := MetricWindowCalculator{tc.data.clock}.Calculate(tc.data.roundingPeriod, tc.data.length, tc.data.delay)
			if !startTime.Equal(tc.data.expectedStartTime) {
				t.Errorf("start time incorrect. Expected: %s, Actual: %s", tc.data.expectedStartTime.Format(TimeFormat), startTime.Format(TimeFormat))
				t.Errorf("end time incorrect. Expected: %s, Actual: %s", tc.data.expectedEndTime.Format(TimeFormat), endTime.Format(TimeFormat))
			}
		})
	}
}
