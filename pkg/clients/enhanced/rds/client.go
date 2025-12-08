// Copyright 2024 The Prometheus Authors
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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// ClientInterface defines the interface for RDS clients
type ClientInterface interface {
	DescribeAllDBInstances(ctx context.Context) ([]types.DBInstance, error)
}

// Client wraps the AWS RDS client for enhanced metrics
type Client struct {
	client *rds.Client
}

// NewClient creates a new RDS client from AWS config
func NewClient(cfg aws.Config) *Client {
	return &Client{
		client: rds.NewFromConfig(cfg),
	}
}

// DescribeDBInstances retrieves information about provisioned RDS instances
func (c *Client) DescribeDBInstances(ctx context.Context, marker *string, maxRecords int32) ([]types.DBInstance, *string, error) {
	input := &rds.DescribeDBInstancesInput{
		Marker:     marker,
		MaxRecords: &maxRecords,
	}

	result, err := c.client.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}

	return result.DBInstances, result.Marker, nil
}

// DescribeAllDBInstances retrieves all DB instances by handling pagination
func (c *Client) DescribeAllDBInstances(ctx context.Context) ([]types.DBInstance, error) {
	var allInstances []types.DBInstance
	var marker *string
	maxRecords := int32(100)

	for {
		instances, nextMarker, err := c.DescribeDBInstances(ctx, marker, maxRecords)
		if err != nil {
			return nil, err
		}

		allInstances = append(allInstances, instances...)

		if nextMarker == nil {
			break
		}
		marker = nextMarker
	}

	return allInstances, nil
}
