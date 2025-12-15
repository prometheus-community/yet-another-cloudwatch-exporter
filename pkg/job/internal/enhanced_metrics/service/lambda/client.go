package lambda

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// todo: change logging to debug where appropriate

// AWSLambdaClient wraps the AWS Lambda client
type AWSLambdaClient struct {
	client *lambda.Client
}

// NewLambdaClientWithConfig creates a new Lambda client with custom AWS configuration
func NewLambdaClientWithConfig(cfg aws.Config) Client {
	return &AWSLambdaClient{
		client: lambda.NewFromConfig(cfg),
	}
}

// ListFunctionsInput contains parameters for listFunctions
type ListFunctionsInput struct {
	FunctionVersion *string
	Marker          *string
	MaxItems        *int32
	MasterRegion    *string
}

// ListFunctionsOutput contains the response from listFunctions
type ListFunctionsOutput struct {
	Functions  []types.FunctionConfiguration
	NextMarker *string
}

// listFunctions retrieves a list of Lambda regionalData
func (c *AWSLambdaClient) listFunctions(ctx context.Context, input *ListFunctionsInput) (*ListFunctionsOutput, error) {
	lambdaInput := &lambda.ListFunctionsInput{}

	if input != nil {
		// lambdaInput.FunctionVersion = types.FunctionVersionAll
		lambdaInput.Marker = input.Marker
		lambdaInput.MaxItems = input.MaxItems
		// lambdaInput.MasterRegion = input.MasterRegion
	}

	result, err := c.client.ListFunctions(ctx, lambdaInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list Lambda regionalData: %w", err)
	}

	return &ListFunctionsOutput{
		Functions:  result.Functions,
		NextMarker: result.NextMarker,
	}, nil
}

// ListAllFunctions retrieves all Lambda regionalData by handling pagination
func (c *AWSLambdaClient) ListAllFunctions(ctx context.Context, logger *slog.Logger) ([]types.FunctionConfiguration, error) {
	logger.Info("Looking for all Lambda regionalData")
	var allFunctions []types.FunctionConfiguration
	var marker *string
	var maxItems int32 = 50

	for {
		output, err := c.listFunctions(ctx, &ListFunctionsInput{
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
