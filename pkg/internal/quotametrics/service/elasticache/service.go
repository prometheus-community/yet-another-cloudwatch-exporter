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
package elasticache

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
	namespace   = "AWS/ElastiCache"
	serviceCode = "elasticache"
)

type ElastiCacheService struct {
	buildClientFunc func(cfg aws.Config) Client
}

func NewElastiCacheService(buildClientFunc func(cfg aws.Config) Client) *ElastiCacheService {
	if buildClientFunc == nil {
		buildClientFunc = NewClientWithConfig
	}
	return &ElastiCacheService{buildClientFunc: buildClientFunc}
}

func (s *ElastiCacheService) GetNamespace() string   { return namespace }
func (s *ElastiCacheService) GetServiceCode() string { return serviceCode }

func (s *ElastiCacheService) GetLimits(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) ([]service.LimitResult, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	logger.Debug("Fetching ElastiCache quota limits")
	quotaLimits, err := quotas.FetchQuotaLimits(ctx, client.QuotasClient(), serviceCode)
	if err != nil {
		return nil, err
	}

	var results []service.LimitResult

	// Nodes
	if nodesLimit, ok := quotaLimits["Nodes per Region"]; ok {
		clusters, err := client.DescribeAllCacheClusters(ctx)
		if err != nil {
			logger.Error("Failed to describe cache clusters for quota usage", "err", err)
		} else {
			var totalNodes int
			for _, cluster := range clusters {
				if cluster.NumCacheNodes != nil {
					totalNodes += int(*cluster.NumCacheNodes)
				}
			}
			usage := float64(totalNodes)
			results = append(results, service.LimitResult{
				LimitName:  "Nodes per Region",
				LimitValue: nodesLimit,
				Usage:      &usage,
			})
		}
	}

	// Subnet Groups
	if subnetLimit, ok := quotaLimits["Subnet Groups per Region"]; ok {
		groups, err := client.DescribeAllCacheSubnetGroups(ctx)
		if err != nil {
			logger.Error("Failed to describe cache subnet groups for quota usage", "err", err)
		} else {
			usage := float64(len(groups))
			results = append(results, service.LimitResult{
				LimitName:  "Subnet Groups per Region",
				LimitValue: subnetLimit,
				Usage:      &usage,
			})
		}
	}

	return results, nil
}
