// Copyright 2026 The Prometheus Authors
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

package otelcollector

import (
	"context"
	"log/slog"
	"time"

	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var receiverType = component.MustNewType("yace_exporter")

func NewFactory() receiver.Factory {
	return newFactoryWithLifecycleManagerBuilder(func() prombridge.ExporterLifecycleManager {
		return newLifecycleManager(slog.Default())
	})
}

func newFactoryWithLifecycleManagerBuilder(builder func() prombridge.ExporterLifecycleManager) receiver.Factory {
	if builder == nil {
		panic("lifecycle manager builder must not be nil")
	}

	return receiver.NewFactory(
		receiverType,
		func() component.Config {
			return &prombridge.ReceiverConfig{
				ScrapeInterval: 30 * time.Second,
				ExporterConfig: defaultComponentDefaults(),
			}
		},
		receiver.WithMetrics(
			func(
				ctx context.Context,
				set receiver.Settings,
				cfg component.Config,
				next consumer.Metrics,
			) (receiver.Metrics, error) {
				factory := prombridge.NewFactory(receiverType, builder(), configUnmarshaler{})
				return factory.CreateMetrics(ctx, set, cfg, next)
			},
			component.StabilityLevelAlpha,
		),
	)
}

var (
	_ prombridge.ExporterLifecycleManager = (*lifecycleManager)(nil)
	_ prombridge.ConfigUnmarshaler        = (configUnmarshaler{})
)
