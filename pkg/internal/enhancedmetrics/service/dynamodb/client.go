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
	ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
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

// listTables retrieves a list of DynamoDB tables
func (c *AWSDynamoDBClient) listTables(ctx context.Context, exclusiveStartTableName *string, limit *int32) ([]string, *string, error) {
	dynamoInput := &dynamodb.ListTablesInput{
		ExclusiveStartTableName: exclusiveStartTableName,
		Limit:                   limit,
	}

	result, err := c.client.ListTables(ctx, dynamoInput)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list DynamoDB tables: %w", err)
	}

	return result.TableNames, result.LastEvaluatedTableName, nil
}

// describeTable retrieves detailed information about a DynamoDB table
func (c *AWSDynamoDBClient) describeTable(ctx context.Context, tableName string) (*types.TableDescription, error) {
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
	logger.Debug("Describing all DynamoDB tables started")
	var allTables []types.TableDescription
	var startTableName *string
	var limit int32 = 100

	for {
		tableNames, lastEvaluatedTableName, err := c.listTables(ctx, startTableName, &limit)
		if err != nil {
			return nil, err
		}

		for _, tableName := range tableNames {
			tableDesc, err := c.describeTable(ctx, tableName)
			if err != nil {
				logger.Error("Failed to describe table", "error", err.Error(), "table", tableName)
				continue
			}
			allTables = append(allTables, *tableDesc)
		}

		if lastEvaluatedTableName == nil {
			break
		}
		startTableName = lastEvaluatedTableName
	}

	logger.Debug("Describing all DynamoDB tables completed", "total_tables", len(allTables))
	return allTables, nil
}
