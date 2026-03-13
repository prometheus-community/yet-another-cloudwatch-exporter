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
package quotametrics

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// Service is responsible for getting quota metrics using appropriate services.
type Service struct {
	configProvider config.RegionalConfigProvider
	registry       *Registry
}

// NewService creates a new quota metrics service.
func NewService(factory clients.Factory, registry *Registry) (*Service, error) {
	emf, ok := factory.(config.RegionalConfigProvider)
	if !ok {
		return nil, fmt.Errorf("cannot create quota metric service with a factory type %T", factory)
	}

	return &Service{
		configProvider: emf,
		registry:       registry,
	}, nil
}

// GetMetrics returns the quota metrics for the specified namespace.
func (s *Service) GetMetrics(ctx context.Context, logger *slog.Logger, namespace string, region string, role model.Role) ([]model.QuotaMetricData, error) {
	svc, err := s.registry.GetService(namespace)
	if err != nil {
		return nil, fmt.Errorf("could not get quota metric service for namespace %s: %w", namespace, err)
	}

	limits, err := svc.GetLimits(ctx, logger, region, role, s.configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to get limits for namespace %s: %w", namespace, err)
	}

	result := make([]model.QuotaMetricData, 0, len(limits))
	for _, limit := range limits {
		result = append(result, model.QuotaMetricData{
			ServiceCode: svc.GetServiceCode(),
			LimitName:   limit.LimitName,
			LimitValue:  limit.LimitValue,
			UsageValue:  limit.Usage,
		})
	}

	return result, nil
}
