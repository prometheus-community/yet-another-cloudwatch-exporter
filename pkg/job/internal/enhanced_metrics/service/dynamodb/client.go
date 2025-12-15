package dynamodb

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// todo: change logging to debug where appropriate

// AWSDynamoDBClient wraps the AWS DynamoDB client
type AWSDynamoDBClient struct {
	client *dynamodb.Client
}

// NewDynamoDBClient creates a new DynamoDB client with default AWS configuration
func NewDynamoDBClient(ctx context.Context) (*AWSDynamoDBClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSDynamoDBClient{
		client: dynamodb.NewFromConfig(cfg),
	}, nil
}

// NewDynamoDBClientWithConfig creates a new DynamoDB client with custom AWS configuration
func NewDynamoDBClientWithConfig(cfg aws.Config) *AWSDynamoDBClient {
	return &AWSDynamoDBClient{
		client: dynamodb.NewFromConfig(cfg),
	}
}

// ListTablesInput contains parameters for ListTables
type ListTablesInput struct {
	ExclusiveStartTableName *string
	Limit                   *int32
}

// ListTablesOutput contains the response from ListTables
type ListTablesOutput struct {
	TableNames             []string
	LastEvaluatedTableName *string
}

// ListTables retrieves a list of DynamoDB tables
func (c *AWSDynamoDBClient) ListTables(ctx context.Context, logger *slog.Logger, input *ListTablesInput) (*ListTablesOutput, error) {
	dynamoInput := &dynamodb.ListTablesInput{}

	if input != nil {
		dynamoInput.ExclusiveStartTableName = input.ExclusiveStartTableName
		dynamoInput.Limit = input.Limit
	}

	result, err := c.client.ListTables(ctx, dynamoInput)
	if err != nil {
		return nil, fmt.Errorf("failed to list DynamoDB tables: %w", err)
	}

	return &ListTablesOutput{
		TableNames:             result.TableNames,
		LastEvaluatedTableName: result.LastEvaluatedTableName,
	}, nil
}

// DescribeTable retrieves detailed information about a DynamoDB table
func (c *AWSDynamoDBClient) DescribeTable(ctx context.Context, logger *slog.Logger, tableName string) (*types.TableDescription, error) {
	result, err := c.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe table %s: %w", tableName, err)
	}

	return result.Table, nil
}

// DescribeAllTables retrieves all DynamoDB tables with their descriptions
func (c *AWSDynamoDBClient) DescribeAllTables(ctx context.Context, logger *slog.Logger) ([]types.TableDescription, error) {
	logger.Info("Looking for all DynamoDB tables")
	var allTables []types.TableDescription
	var startTableName *string
	var limit int32 = 100

	for {
		output, err := c.ListTables(ctx, logger, &ListTablesInput{
			ExclusiveStartTableName: startTableName,
			Limit:                   &limit,
		})
		if err != nil {
			return nil, err
		}

		for _, tableName := range output.TableNames {
			tableDesc, err := c.DescribeTable(ctx, logger, tableName)
			if err != nil {
				logger.Error("Failed to describe table", err, "table", tableName)
				continue
			}
			allTables = append(allTables, *tableDesc)
		}

		if output.LastEvaluatedTableName == nil {
			break
		}
		startTableName = output.LastEvaluatedTableName
	}

	logger.Info("Looking for all DynamoDB tables finished", "total_tables", len(allTables))
	return allTables, nil
}
