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
package ec2

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const (
	namespace   = "AWS/EC2"
	serviceCode = "ec2"
)

// instanceFamilyQuota describes an EC2 on-demand instance family quota.
type instanceFamilyQuota struct {
	// awsQuotaName is the name as returned by the Service Quotas API.
	awsQuotaName string
	// metricName is the clean name used in the Prometheus metric.
	metricName string
	// familyKey is the instance type prefix used to count usage.
	familyKey string
}

var instanceFamilyQuotas = []instanceFamilyQuota{
	{"Running On-Demand Standard (A, C, D, H, I, M, R, T, Z) instances", "Running On-Demand Standard instances", "standard"},
	{"Running On-Demand F instances", "Running On-Demand F instances", "f"},
	{"Running On-Demand G and VT instances", "Running On-Demand G and VT instances", "g"},
	{"Running On-Demand Inf instances", "Running On-Demand Inf instances", "inf"},
	{"Running On-Demand P instances", "Running On-Demand P instances", "p"},
	{"Running On-Demand X instances", "Running On-Demand X instances", "x"},
}

// standardPrefixes are the instance type prefixes that count as "standard".
var standardPrefixes = []string{"a", "c", "d", "h", "i", "m", "r", "t", "z"}

type EC2Service struct {
	buildClientFunc func(cfg aws.Config) Client
}

func NewEC2Service(buildClientFunc func(cfg aws.Config) Client) *EC2Service {
	if buildClientFunc == nil {
		buildClientFunc = NewClientWithConfig
	}
	return &EC2Service{buildClientFunc: buildClientFunc}
}

func (s *EC2Service) GetNamespace() string   { return namespace }
func (s *EC2Service) GetServiceCode() string { return serviceCode }

func (s *EC2Service) GetLimits(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) ([]service.LimitResult, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	logger.Debug("Fetching EC2 quota limits")
	quotaLimits, err := quotas.FetchQuotaLimits(ctx, client.QuotasClient(), serviceCode)
	if err != nil {
		return nil, err
	}

	var results []service.LimitResult

	// Instance family quotas
	reservations, err := client.DescribeRunningInstances(ctx)
	if err != nil {
		logger.Error("Failed to describe running instances for quota usage", "err", err)
	}

	familyCounts := countInstancesByFamily(reservations)

	for _, q := range instanceFamilyQuotas {
		limit, ok := quotaLimits[q.awsQuotaName]
		if !ok {
			continue
		}
		count := familyCounts[q.familyKey]
		usage := float64(count)
		results = append(results, service.LimitResult{
			LimitName:  q.metricName,
			LimitValue: limit,
			Usage:      &usage,
		})
	}

	// Elastic IPs
	if eipLimit, ok := quotaLimits["EC2-VPC Elastic IPs"]; ok {
		addresses, err := client.DescribeAddresses(ctx)
		if err != nil {
			logger.Error("Failed to describe addresses for quota usage", "err", err)
		} else {
			usage := float64(len(addresses))
			results = append(results, service.LimitResult{
				LimitName:  "EC2-VPC Elastic IPs",
				LimitValue: eipLimit,
				Usage:      &usage,
			})
		}
	}

	return results, nil
}

func countInstancesByFamily(reservations []types.Reservation) map[string]int {
	counts := make(map[string]int)
	for _, reservation := range reservations {
		for _, instance := range reservation.Instances {
			if instance.InstanceType == "" {
				continue
			}
			instType := string(instance.InstanceType)
			prefix := strings.SplitN(instType, ".", 2)[0]
			// Extract just the letter prefix (e.g., "m5" -> "m", "c5a" -> "c")
			var sb strings.Builder
			for _, c := range prefix {
				if c >= 'a' && c <= 'z' {
					sb.WriteRune(c)
				} else {
					break
				}
			}
			letterPrefix := sb.String()

			family := classifyInstanceFamily(letterPrefix)
			counts[family]++
		}
	}
	return counts
}

func classifyInstanceFamily(prefix string) string {
	if slices.Contains(standardPrefixes, prefix) {
		return "standard"
	}
	switch {
	case strings.HasPrefix(prefix, "f"):
		return "f"
	case strings.HasPrefix(prefix, "g") || strings.HasPrefix(prefix, "vt"):
		return "g"
	case strings.HasPrefix(prefix, "inf"):
		return "inf"
	case strings.HasPrefix(prefix, "p"):
		return "p"
	case strings.HasPrefix(prefix, "x"):
		return "x"
	default:
		return "standard"
	}
}
