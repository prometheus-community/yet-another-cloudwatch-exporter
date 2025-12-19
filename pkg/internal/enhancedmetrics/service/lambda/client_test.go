package lambda

import (
	"context"
	"fmt"
	"io"
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
				listFunctionsFunc: func() func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
					callCount := 0
					return func(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
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
			got, err := c.ListAllFunctions(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
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
