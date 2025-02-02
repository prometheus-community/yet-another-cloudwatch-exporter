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
package maxdimassociator

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestAssociatorLogging(t *testing.T) {
	type testcase struct {
		level slog.Level
	}
	for name, tc := range map[string]testcase{
		"debug enabled":  {level: slog.LevelDebug},
		"debug disabled": {level: slog.LevelInfo},
	} {
		t.Run(name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
				Level: tc.level,
			}))
			associator := NewAssociator(logger, config.SupportedServices.GetService("AWS/Logs").ToModelDimensionsRegexp(), logGroupResources)
			res, skip := associator.AssociateMetricToResource(&model.Metric{
				MetricName: "DeliveryThrottling",
				Namespace:  "AWS/Logs",
				Dimensions: []model.Dimension{
					{Name: "LogGroupName", Value: "/aws/lambda/log-group-1"},
				},
			})
			require.NotNil(t, res)
			require.False(t, skip)

			assertion := require.NotContains
			if tc.level == slog.LevelDebug {
				assertion = require.Contains
			}
			assertion(t, buf.String(), "found mapping")
		})
	}
}
