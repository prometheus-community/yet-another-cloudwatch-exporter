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
package s3

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const (
	namespace   = "AWS/S3"
	serviceCode = "s3"
)

type S3Service struct {
	buildClientFunc func(cfg aws.Config) Client
}

func NewS3Service(buildClientFunc func(cfg aws.Config) Client) *S3Service {
	if buildClientFunc == nil {
		buildClientFunc = NewClientWithConfig
	}
	return &S3Service{buildClientFunc: buildClientFunc}
}

func (s *S3Service) GetNamespace() string   { return namespace }
func (s *S3Service) GetServiceCode() string { return serviceCode }

func (s *S3Service) GetLimits(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) ([]service.LimitResult, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	logger.Debug("Fetching S3 quota limits")
	quotaLimits, err := quotas.FetchQuotaLimits(ctx, client.QuotasClient(), serviceCode)
	if err != nil {
		return nil, err
	}

	var results []service.LimitResult

	if bucketsLimit, ok := quotaLimits["Buckets"]; ok {
		buckets, err := client.ListBuckets(ctx)
		if err != nil {
			logger.Error("Failed to list buckets for quota usage", "err", err)
		} else {
			usage := float64(len(buckets))
			results = append(results, service.LimitResult{
				LimitName:  "Buckets",
				LimitValue: bucketsLimit,
				Usage:      &usage,
			})
		}
	}

	return results, nil
}
