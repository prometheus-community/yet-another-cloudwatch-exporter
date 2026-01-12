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
package dynamodb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type awsClient interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// AWSDynamoDBClient wraps the AWS DynamoDB client
type AWSDynamoDBClient struct {
	client awsClient
}

// NewDynamoDBClientWithConfig creates a new DynamoDB client with custom AWS configuration
func NewDynamoDBClientWithConfig(cfg aws.Config) Client {
	return &AWSDynamoDBClient{
		client: dynamodb.NewFromConfig(cfg),
	}
}

// describeTable retrieves detailed information about a DynamoDB table
func (c *AWSDynamoDBClient) describeTable(ctx context.Context, tableARN string) (*types.TableDescription, error) {
	result, err := c.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		// TableName can be either the table name or ARN
		TableName: aws.String(tableARN),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe table %s: %w", tableARN, err)
	}

	return result.Table, nil
}

// DescribeTables retrieves DynamoDB tables with their descriptions
func (c *AWSDynamoDBClient) DescribeTables(ctx context.Context, logger *slog.Logger, tablesARNs []string) ([]types.TableDescription, error) {
	logger.Debug("Describing DynamoDB tables", "count", len(tablesARNs))

	var tables []types.TableDescription

	for _, arn := range tablesARNs {
		tableDesc, err := c.describeTable(ctx, arn)
		if err != nil {
			logger.Error("Failed to describe table", "error", err.Error(), "arn", arn)
			continue
		}

		tables = append(tables, *tableDesc)
	}

	logger.Debug("Describing DynamoDB tables completed", "total_tables", len(tables))
	return tables, nil
}
