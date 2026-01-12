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
package rds

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

type awsClient interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

// AWSRDSClient wraps the AWS RDS client
type AWSRDSClient struct {
	client awsClient
}

// NewRDSClientWithConfig creates a new RDS client with custom AWS configuration
func NewRDSClientWithConfig(cfg aws.Config) Client {
	return &AWSRDSClient{
		client: rds.NewFromConfig(cfg),
	}
}

// describeDBInstances retrieves information about provisioned RDS instances
func (c *AWSRDSClient) describeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	result, err := c.client.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}
	return result, nil
}

// DescribeAllDBInstances retrieves all DB instances by handling pagination
func (c *AWSRDSClient) DescribeDBInstances(ctx context.Context, logger *slog.Logger, dbInstances []string) ([]types.DBInstance, error) {
	logger.Debug("Describing all RDS DB instances")
	var allInstances []types.DBInstance
	var marker *string
	maxRecords := aws.Int32(100)

	for {
		output, err := c.describeDBInstances(ctx, &rds.DescribeDBInstancesInput{
			Marker:     marker,
			MaxRecords: maxRecords,
			Filters: []types.Filter{
				{
					Name:   aws.String("db-instance-id"),
					Values: dbInstances,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		allInstances = append(allInstances, output.DBInstances...)

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	logger.Debug("Completed describing RDS DB instances", slog.Int("totalInstances", len(allInstances)))
	return allInstances, nil
}
