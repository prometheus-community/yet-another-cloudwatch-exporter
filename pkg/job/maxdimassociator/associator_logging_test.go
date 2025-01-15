package maxdimassociator

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/stretchr/testify/require"
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
