package dynamodb

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestAWSDynamoDBClient_DescribeAllTables(t *testing.T) {
	tests := []struct {
		name    string
		client  AWSClient
		want    []types.TableDescription
		wantErr bool
	}{
		{
			name: "success - single page",
			client: &mockDynamoDBClient{
				listTablesFunc: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
					return &dynamodb.ListTablesOutput{
						TableNames:             []string{"table-1"},
						LastEvaluatedTableName: nil,
					}, nil
				},
				describeTableFunc: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
					return &dynamodb.DescribeTableOutput{
						Table: &types.TableDescription{
							TableName: aws.String("table-1"),
						},
					}, nil
				},
			},
			want: []types.TableDescription{
				{TableName: aws.String("table-1")},
			},
			wantErr: false,
		},
		{
			name: "success - multiple pages",
			client: &mockDynamoDBClient{
				listTablesFunc: func() func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
					callCount := 0
					return func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
						callCount++
						if callCount == 1 {
							return &dynamodb.ListTablesOutput{
								TableNames:             []string{"table-1"},
								LastEvaluatedTableName: aws.String("table-1"),
							}, nil
						}
						return &dynamodb.ListTablesOutput{
							TableNames:             []string{"table-2"},
							LastEvaluatedTableName: nil,
						}, nil
					}
				}(),
				describeTableFunc: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
					return &dynamodb.DescribeTableOutput{
						Table: &types.TableDescription{
							TableName: params.TableName,
						},
					}, nil
				},
			},
			want: []types.TableDescription{
				{TableName: aws.String("table-1")},
				{TableName: aws.String("table-2")},
			},
			wantErr: false,
		},
		{
			name: "error - ListTables failure",
			client: &mockDynamoDBClient{
				listTablesFunc: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "partial success - DescribeTable failure",
			client: &mockDynamoDBClient{
				listTablesFunc: func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
					return &dynamodb.ListTablesOutput{
						TableNames:             []string{"table-1", "table-2"},
						LastEvaluatedTableName: nil,
					}, nil
				},
				describeTableFunc: func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
					if *params.TableName == "table-1" {
						return nil, fmt.Errorf("describe error")
					}
					return &dynamodb.DescribeTableOutput{
						Table: &types.TableDescription{
							TableName: params.TableName,
						},
					}, nil
				},
			},
			want: []types.TableDescription{
				{TableName: aws.String("table-2")},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AWSDynamoDBClient{
				client: tt.client,
			}
			got, err := c.DescribeAllTables(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
			if (err != nil) != tt.wantErr {
				t.Errorf("DescribeAllTables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DescribeAllTables() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockDynamoDBClient is a mock implementation of AWSClient
type mockDynamoDBClient struct {
	listTablesFunc    func(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
	describeTableFunc func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDynamoDBClient) ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	return m.listTablesFunc(ctx, params, optFns...)
}

func (m *mockDynamoDBClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return m.describeTableFunc(ctx, params, optFns...)
}
