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
package quotas

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
)

// Client is an interface for the Service Quotas API calls needed.
type Client interface {
	ListServiceQuotas(ctx context.Context, params *servicequotas.ListServiceQuotasInput, optFns ...func(*servicequotas.Options)) (*servicequotas.ListServiceQuotasOutput, error)
}

// FetchQuotaLimits returns a map of quota_name → limit_value for a service code.
func FetchQuotaLimits(ctx context.Context, client Client, serviceCode string) (map[string]float64, error) {
	limits := make(map[string]float64)
	var nextToken *string

	for {
		output, err := client.ListServiceQuotas(ctx, &servicequotas.ListServiceQuotasInput{
			ServiceCode: aws.String(serviceCode),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list service quotas for %s: %w", serviceCode, err)
		}

		for _, quota := range output.Quotas {
			if quota.QuotaName != nil && quota.Value != nil {
				limits[*quota.QuotaName] = *quota.Value
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return limits, nil
}

// NewServiceQuotasClient creates a new Service Quotas client from an AWS config.
func NewServiceQuotasClient(cfg aws.Config) Client {
	return servicequotas.NewFromConfig(cfg)
}
