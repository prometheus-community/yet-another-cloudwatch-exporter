package client

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// todo: change logging to debug where appropriate

// LambdaClient wraps the AWS Lambda client
type LambdaClient struct {
	client *lambda.Client
}

// NewLambdaClient creates a new Lambda client with default AWS configuration
func NewLambdaClient(ctx context.Context) (*LambdaClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &LambdaClient{
		client: lambda.NewFromConfig(cfg),
	}, nil
}

// NewLambdaClientWithConfig creates a new Lambda client with custom AWS configuration
func NewLambdaClientWithConfig(cfg aws.Config) *LambdaClient {
	return &LambdaClient{
		client: lambda.NewFromConfig(cfg),
	}
}

// ListFunctionsInput contains parameters for ListFunctions
type ListFunctionsInput struct {
	FunctionVersion *string
	Marker          *string
	MaxItems        *int32
	MasterRegion    *string
}

// ListFunctionsOutput contains the response from ListFunctions
type ListFunctionsOutput struct {
	Functions  []types.FunctionConfiguration
	NextMarker *string
}

// ListFunctions retrieves a list of Lambda functions
func (c *LambdaClient) ListFunctions(ctx context.Context, input *ListFunctionsInput) (*ListFunctionsOutput, error) {
	lambdaInput := &lambda.ListFunctionsInput{}

	if input != nil {
		// lambdaInput.FunctionVersion = types.FunctionVersionAll
		lambdaInput.Marker = input.Marker
		lambdaInput.MaxItems = input.MaxItems
		// lambdaInput.MasterRegion = input.MasterRegion
	}

	result, err := c.client.ListFunctions(ctx, lambdaInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list Lambda functions: %w", err)
	}

	return &ListFunctionsOutput{
		Functions:  result.Functions,
		NextMarker: result.NextMarker,
	}, nil
}

// ListAllFunctions retrieves all Lambda functions by handling pagination
func (c *LambdaClient) ListAllFunctions(ctx context.Context, logger *slog.Logger) ([]types.FunctionConfiguration, error) {
	logger.Info("Looking for all Lambda functions")
	var allFunctions []types.FunctionConfiguration
	var marker *string
	var maxItems int32 = 50

	for {
		output, err := c.ListFunctions(ctx, &ListFunctionsInput{
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

	return allFunctions, nil
}
