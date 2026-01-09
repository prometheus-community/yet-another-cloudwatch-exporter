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
package lambda

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type awsClient interface {
	ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
}

// AWSLambdaClient wraps the AWS Lambda client
type AWSLambdaClient struct {
	client awsClient
}

// NewLambdaClientWithConfig creates a new Lambda client with custom AWS configuration
func NewLambdaClientWithConfig(cfg aws.Config) Client {
	return &AWSLambdaClient{
		client: lambda.NewFromConfig(cfg),
	}
}

// listFunctions retrieves a list of Lambda regionalData
func (c *AWSLambdaClient) listFunctions(ctx context.Context, input *lambda.ListFunctionsInput) (*lambda.ListFunctionsOutput, error) {
	result, err := c.client.ListFunctions(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list Lambda regionalData: %w", err)
	}

	return result, nil
}

// ListAllFunctions retrieves all Lambda regionalData by handling pagination
func (c *AWSLambdaClient) ListAllFunctions(ctx context.Context, logger *slog.Logger) ([]types.FunctionConfiguration, error) {
	logger.Debug("Listing all Lambda functions")
	var allFunctions []types.FunctionConfiguration
	var marker *string
	var maxItems int32 = 50

	for {
		output, err := c.listFunctions(ctx, &lambda.ListFunctionsInput{
			Marker:   marker,
			MaxItems: &maxItems,
		})
		if err != nil {
			return nil, err
		}

		allFunctions = append(allFunctions, output.Functions...)

		if output.NextMarker == nil {
			break
		}
		marker = output.NextMarker
	}

	logger.Debug("Completed listing all Lambda functions", slog.Int("totalFunctions", len(allFunctions)))
	return allFunctions, nil
}
