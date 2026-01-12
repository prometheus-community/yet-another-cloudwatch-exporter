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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestAWSDynamoDBClient_DescribeAllTables(t *testing.T) {
	tests := []struct {
		name    string
		client  awsClient
		want    []types.TableDescription
		wantErr bool
		tables  []string
	}{
		{
			name:   "success - single page",
			tables: []string{"table-1"},
			client: &mockDynamoDBClient{
				describeTableFunc: func(_ context.Context, params *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
					if *params.TableName != "table-1" {
						return nil, fmt.Errorf("unexpected table name: %s", *params.TableName)
					}

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
			name:   "describeTable failure",
			tables: []string{"table-1", "table-2"},
			client: &mockDynamoDBClient{
				describeTableFunc: func(_ context.Context, params *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
					if *params.TableName == "table-1" {
						return nil, fmt.Errorf("describe error")
					}

					if *params.TableName != "table-2" {
						return nil, fmt.Errorf("unexpected table name: %s", *params.TableName)
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
			got, err := c.DescribeTables(context.Background(), slog.New(slog.DiscardHandler), tt.tables)
			if (err != nil) != tt.wantErr {
				t.Errorf("DescribeTables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DescribeTables() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockDynamoDBClient is a mock implementation of sdk AWS DynamoDB Client
type mockDynamoDBClient struct {
	describeTableFunc func(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

func (m *mockDynamoDBClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return m.describeTableFunc(ctx, params, optFns...)
}
