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
package quotametrics_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// stubConfigProvider implements config.RegionalConfigProvider using real AWS credentials.
type stubConfigProvider struct{}

func (s *stubConfigProvider) GetAWSRegionalConfig(region string, _ model.Role) *aws.Config {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile := os.Getenv("AWS_PROFILE"); profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		panic(fmt.Sprintf("failed to load AWS config for region %s: %v", region, err))
	}
	return &cfg
}

func TestIntegration_QuotaMetrics(t *testing.T) {
	if os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("Skipping integration test: no AWS credentials configured (set AWS_PROFILE or AWS_ACCESS_KEY_ID)")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()
	region := "us-east-1"
	role := model.Role{}

	provider := &stubConfigProvider{}

	namespaces := []string{"AWS/EC2", "AWS/ElastiCache", "AWS/S3"}

	var allResults []model.QuotaResult

	for _, ns := range namespaces {
		svc, err := quotametrics.DefaultQuotaMetricServiceRegistry.GetService(ns)
		if err != nil {
			t.Fatalf("failed to get service for %s: %v", ns, err)
		}

		limits, err := svc.GetLimits(ctx, logger, region, role, provider)
		if err != nil {
			t.Errorf("failed to get limits for %s: %v", ns, err)
			continue
		}

		var data []model.QuotaMetricData
		for _, l := range limits {
			data = append(data, model.QuotaMetricData{
				ServiceCode: svc.GetServiceCode(),
				LimitName:   l.LimitName,
				LimitValue:  l.LimitValue,
				UsageValue:  l.Usage,
			})
		}

		allResults = append(allResults, model.QuotaResult{
			Context: &model.ScrapeContext{
				Region:    region,
				AccountID: "integration-test",
			},
			Data: data,
		})
	}

	metrics, _ := promutil.BuildQuotaMetrics(allResults, false, logger)

	fmt.Println()
	fmt.Println("=== Quota Metrics ===")
	for _, m := range metrics {
		labels := ""
		for k, v := range m.Labels {
			if labels != "" {
				labels += ","
			}
			labels += fmt.Sprintf("%s=%q", k, v)
		}
		fmt.Printf("%s{%s} %g\n", m.Name, labels, m.Value)
	}
	fmt.Printf("\nTotal metrics: %d\n", len(metrics))
}
