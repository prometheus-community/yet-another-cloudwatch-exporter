// Copyright The Prometheus Authors
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
package tagging

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	dmstypes "github.com/aws/aws-sdk-go-v2/service/databasemigrationservice/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestValidServiceFilterNames(t *testing.T) {
	for svc, filter := range ServiceFilters {
		if config.SupportedServices.GetService(svc) == nil {
			t.Errorf("invalid service name '%s' in ServiceFilters", svc)
		}

		if filter.FilterFunc == nil && filter.ResourceFunc == nil {
			t.Errorf("no filter functions defined for service name '%s'", svc)
		}
	}
}

// mockAPIOption returns middleware that intercepts AWS SDK v2 API calls and returns
// mock responses keyed by operation name, short-circuiting before the HTTP call.
func mockAPIOption(responses map[string]interface{}) func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Finalize.Add(
			middleware.FinalizeMiddlewareFunc("mock",
				func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
					opName := middleware.GetOperationName(ctx)
					if resp, ok := responses[opName]; ok {
						return middleware.FinalizeOutput{Result: resp}, middleware.Metadata{}, nil
					}
					return middleware.FinalizeOutput{}, middleware.Metadata{}, fmt.Errorf("unexpected operation: %s", opName)
				},
			),
			middleware.Before,
		)
	}
}

func TestApiGatewayFilterFunc(t *testing.T) {
	tests := []struct {
		name            string
		apiGatewayAPI   *apigateway.Client
		apiGatewayV2API *apigatewayv2.Client
		inputResources  []*model.TaggedResource
		outputResources []*model.TaggedResource
	}{
		{
			name: "API Gateway v1 REST API: stages are filtered and IDs replaced with names",
			apiGatewayAPI: apigateway.New(apigateway.Options{
				Region: "us-east-1",
				APIOptions: []func(*middleware.Stack) error{
					mockAPIOption(map[string]interface{}{
						"GetRestApis": &apigateway.GetRestApisOutput{
							Items: []apigtypes.RestApi{
								{
									Id:   aws.String("gwid1234"),
									Name: aws.String("apiname"),
								},
							},
						},
					}),
				},
			}),
			apiGatewayV2API: apigatewayv2.New(apigatewayv2.Options{
				Region: "us-east-1",
				APIOptions: []func(*middleware.Stack) error{
					mockAPIOption(map[string]interface{}{
						"GetApis": &apigatewayv2.GetApisOutput{
							Items: []apigv2types.Api{},
						},
					}),
				},
			}),
			inputResources: []*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/restapis/gwid1234/stages/main",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags:      []model.Tag{{Key: "Test", Value: "Value"}},
				},
				{
					ARN:       "arn:aws:apigateway:us-east-1::/restapis/gwid1234",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags:      []model.Tag{{Key: "Test", Value: "Value 2"}},
				},
			},
			outputResources: []*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/restapis/apiname",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags:      []model.Tag{{Key: "Test", Value: "Value 2"}},
				},
			},
		},
		{
			name: "API Gateway v2 REST API: stages are filtered",
			apiGatewayAPI: apigateway.New(apigateway.Options{
				Region: "us-east-1",
				APIOptions: []func(*middleware.Stack) error{
					mockAPIOption(map[string]interface{}{
						"GetRestApis": &apigateway.GetRestApisOutput{
							Items: []apigtypes.RestApi{},
						},
					}),
				},
			}),
			apiGatewayV2API: apigatewayv2.New(apigatewayv2.Options{
				Region: "us-east-1",
				APIOptions: []func(*middleware.Stack) error{
					mockAPIOption(map[string]interface{}{
						"GetApis": &apigatewayv2.GetApisOutput{
							Items: []apigv2types.Api{
								{
									ApiId: aws.String("gwid9876"),
								},
							},
						},
					}),
				},
			}),
			inputResources: []*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/apis/gwid9876/stages/$default",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags:      []model.Tag{{Key: "Test", Value: "Value"}},
				},
				{
					ARN:       "arn:aws:apigateway:us-east-1::/apis/gwid9876",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags:      []model.Tag{{Key: "Test", Value: "Value 2"}},
				},
			},
			outputResources: []*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/apis/gwid9876",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags:      []model.Tag{{Key: "Test", Value: "Value 2"}},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := client{
				apiGatewayAPI:   tc.apiGatewayAPI,
				apiGatewayV2API: tc.apiGatewayV2API,
			}

			filter := ServiceFilters["AWS/ApiGateway"]
			require.NotNil(t, filter.FilterFunc)

			outputResources, err := filter.FilterFunc(context.Background(), c, tc.inputResources)
			require.NoError(t, err)
			require.Equal(t, tc.outputResources, outputResources)
		})
	}
}

func TestDMSFilterFunc(t *testing.T) {
	tests := []struct {
		name            string
		dmsAPI          *databasemigrationservice.Client
		inputResources  []*model.TaggedResource
		outputResources []*model.TaggedResource
	}{
		{
			name:            "empty input resources",
			inputResources:  []*model.TaggedResource{},
			outputResources: []*model.TaggedResource{},
		},
		{
			name: "replication instance identifiers appended to task and instance ARNs",
			dmsAPI: databasemigrationservice.New(databasemigrationservice.Options{
				Region: "us-east-1",
				APIOptions: []func(*middleware.Stack) error{
					mockAPIOption(map[string]interface{}{
						"DescribeReplicationInstances": &databasemigrationservice.DescribeReplicationInstancesOutput{
							ReplicationInstances: []dmstypes.ReplicationInstance{
								{
									ReplicationInstanceArn:        aws.String("arn:aws:dms:us-east-1:123123123123:rep:ABCDEFG1234567890"),
									ReplicationInstanceIdentifier: aws.String("repl-instance-identifier-1"),
								},
								{
									ReplicationInstanceArn:        aws.String("arn:aws:dms:us-east-1:123123123123:rep:ZZZZZZZZZZZZZZZZZ"),
									ReplicationInstanceIdentifier: aws.String("repl-instance-identifier-2"),
								},
								{
									ReplicationInstanceArn:        aws.String("arn:aws:dms:us-east-1:123123123123:rep:YYYYYYYYYYYYYYYYY"),
									ReplicationInstanceIdentifier: aws.String("repl-instance-identifier-3"),
								},
							},
						},
						"DescribeReplicationTasks": &databasemigrationservice.DescribeReplicationTasksOutput{
							ReplicationTasks: []dmstypes.ReplicationTask{
								{
									ReplicationTaskArn:     aws.String("arn:aws:dms:us-east-1:123123123123:task:9999999999999999"),
									ReplicationInstanceArn: aws.String("arn:aws:dms:us-east-1:123123123123:rep:ZZZZZZZZZZZZZZZZZ"),
								},
								{
									ReplicationTaskArn:     aws.String("arn:aws:dms:us-east-1:123123123123:task:2222222222222222"),
									ReplicationInstanceArn: aws.String("arn:aws:dms:us-east-1:123123123123:rep:ZZZZZZZZZZZZZZZZZ"),
								},
								{
									ReplicationTaskArn:     aws.String("arn:aws:dms:us-east-1:123123123123:task:3333333333333333"),
									ReplicationInstanceArn: aws.String("arn:aws:dms:us-east-1:123123123123:rep:WWWWWWWWWWWWWWWWW"),
								},
							},
						},
					}),
				},
			}),
			inputResources: []*model.TaggedResource{
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:rep:ABCDEFG1234567890", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:rep:WXYZ987654321", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 2"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:task:9999999999999999", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 3"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:task:5555555555555555", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 4"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:subgrp:demo-subgrp", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 5"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:endpoint:1111111111111111", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 6"}},
				},
			},
			outputResources: []*model.TaggedResource{
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:rep:ABCDEFG1234567890/repl-instance-identifier-1", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:rep:WXYZ987654321", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 2"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:task:9999999999999999/repl-instance-identifier-2", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 3"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:task:5555555555555555", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 4"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:subgrp:demo-subgrp", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 5"}},
				},
				{
					ARN: "arn:aws:dms:us-east-1:123123123123:endpoint:1111111111111111", Namespace: "dms", Region: "us-east-1",
					Tags: []model.Tag{{Key: "Test", Value: "Value 6"}},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := client{
				dmsAPI: tc.dmsAPI,
			}

			filter := ServiceFilters["AWS/DMS"]
			require.NotNil(t, filter.FilterFunc)

			outputResources, err := filter.FilterFunc(context.Background(), c, tc.inputResources)
			require.NoError(t, err)
			require.Equal(t, tc.outputResources, outputResources)
		})
	}
}
