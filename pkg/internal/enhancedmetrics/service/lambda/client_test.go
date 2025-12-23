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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func TestAWSLambdaClient_ListAllFunctions(t *testing.T) {
	tests := []struct {
		name    string
		client  awsClient
		want    []types.FunctionConfiguration
		wantErr bool
	}{
		{
			name: "success - single page",
			client: &mockLambdaClient{
				listFunctionsFunc: func(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
					return &lambda.ListFunctionsOutput{
						Functions: []types.FunctionConfiguration{
							{FunctionName: aws.String("function-1")},
						},
						NextMarker: nil,
					}, nil
				},
			},
			want: []types.FunctionConfiguration{
				{FunctionName: aws.String("function-1")},
			},
			wantErr: false,
		},
		{
			name: "success - multiple pages",
			client: &mockLambdaClient{
				listFunctionsFunc: func() func(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
					callCount := 0
					return func(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
						callCount++
						if callCount == 1 {
							return &lambda.ListFunctionsOutput{
								Functions: []types.FunctionConfiguration{
									{FunctionName: aws.String("function-1")},
								},
								NextMarker: aws.String("marker1"),
							}, nil
						}
						return &lambda.ListFunctionsOutput{
							Functions: []types.FunctionConfiguration{
								{FunctionName: aws.String("function-2")},
							},
							NextMarker: nil,
						}, nil
					}
				}(),
			},
			want: []types.FunctionConfiguration{
				{FunctionName: aws.String("function-1")},
				{FunctionName: aws.String("function-2")},
			},
			wantErr: false,
		},
		{
			name: "error - API failure",
			client: &mockLambdaClient{
				listFunctionsFunc: func(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AWSLambdaClient{
				client: tt.client,
			}
			got, err := c.ListAllFunctions(context.Background(), slog.New(slog.DiscardHandler))
			if (err != nil) != tt.wantErr {
				t.Errorf("ListAllFunctions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAllFunctions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockLambdaClient is a mock implementation of AWS Lambda Client
type mockLambdaClient struct {
	listFunctionsFunc func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
}

func (m *mockLambdaClient) ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return m.listFunctionsFunc(ctx, params, optFns...)
}
