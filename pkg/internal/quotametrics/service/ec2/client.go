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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
)

type Client interface {
	DescribeRunningInstances(ctx context.Context) ([]types.Reservation, error)
	DescribeAddresses(ctx context.Context) ([]types.Address, error)
	QuotasClient() quotas.Client
}

type awsEC2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
}

type AWSClient struct {
	ec2Client    awsEC2Client
	quotasClient quotas.Client
}

func NewClientWithConfig(cfg aws.Config) Client {
	return &AWSClient{
		ec2Client:    ec2.NewFromConfig(cfg),
		quotasClient: quotas.NewServiceQuotasClient(cfg),
	}
}

func (c *AWSClient) DescribeRunningInstances(ctx context.Context) ([]types.Reservation, error) {
	var allReservations []types.Reservation
	var nextToken *string

	for {
		output, err := c.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("instance-state-name"),
					Values: []string{"running"},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		allReservations = append(allReservations, output.Reservations...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return allReservations, nil
}

func (c *AWSClient) DescribeAddresses(ctx context.Context) ([]types.Address, error) {
	output, err := c.ec2Client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe addresses: %w", err)
	}
	return output.Addresses, nil
}

func (c *AWSClient) QuotasClient() quotas.Client {
	return c.quotasClient
}
