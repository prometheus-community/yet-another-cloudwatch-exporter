// Copyright 2024 The Prometheus Authors
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
package v1

import (
	"context"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/apigateway/apigatewayiface"
	"github.com/aws/aws-sdk-go/service/apigatewayv2"
	"github.com/aws/aws-sdk-go/service/apigatewayv2/apigatewayv2iface"
	"github.com/aws/aws-sdk-go/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go/service/databasemigrationservice/databasemigrationserviceiface"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestValidServiceNames(t *testing.T) {
	for svc, filter := range ServiceFilters {
		if config.SupportedServices.GetService(svc) == nil {
			t.Errorf("invalid service name '%s'", svc)
			t.Fail()
		}

		if filter.FilterFunc == nil && filter.ResourceFunc == nil {
			t.Errorf("no filter functions defined for service name '%s'", svc)
			t.FailNow()
		}
	}
}

func TestApiGatewayFilterFunc(t *testing.T) {
	tests := []struct {
		name            string
		iface           client
		inputResources  []*model.TaggedResource
		outputResources []*model.TaggedResource
	}{
		{
			"api gateway resources skip stages",
			client{
				apiGatewayAPI: apiGatewayClient{
					getRestApisOutput: &apigateway.GetRestApisOutput{
						Items: []*apigateway.RestApi{
							{
								ApiKeySource:              nil,
								BinaryMediaTypes:          nil,
								CreatedDate:               nil,
								Description:               nil,
								DisableExecuteApiEndpoint: nil,
								EndpointConfiguration:     nil,
								Id:                        aws.String("gwid1234"),
								MinimumCompressionSize:    nil,
								Name:                      aws.String("apiname"),
								Policy:                    nil,
								Tags:                      nil,
								Version:                   nil,
								Warnings:                  nil,
							},
						},
						Position: nil,
					},
				},
				apiGatewayV2API: apiGatewayV2Client{
					getRestApisOutput: &apigatewayv2.GetApisOutput{
						Items: []*apigatewayv2.Api{},
					},
				},
			},
			[]*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/restapis/gwid1234/stages/main",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value",
						},
					},
				},
				{
					ARN:       "arn:aws:apigateway:us-east-1::/restapis/gwid1234",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 2",
						},
					},
				},
			},
			[]*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/restapis/apiname",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 2",
						},
					},
				},
			},
		},
		{
			"api gateway v2",
			client{
				apiGatewayAPI: apiGatewayClient{
					getRestApisOutput: &apigateway.GetRestApisOutput{
						Items: []*apigateway.RestApi{},
					},
				},
				apiGatewayV2API: apiGatewayV2Client{
					getRestApisOutput: &apigatewayv2.GetApisOutput{
						Items: []*apigatewayv2.Api{
							{
								CreatedDate:               nil,
								Description:               nil,
								DisableExecuteApiEndpoint: nil,
								ApiId:                     aws.String("gwid9876"),
								Name:                      aws.String("apiv2name"),
								Tags:                      nil,
								Version:                   nil,
								Warnings:                  nil,
							},
						},
						NextToken: nil,
					},
				},
			},
			[]*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/apis/gwid9876/stages/$default",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value",
						},
					},
				},
				{
					ARN:       "arn:aws:apigateway:us-east-1::/apis/gwid9876",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 2",
						},
					},
				},
			},
			[]*model.TaggedResource{
				{
					ARN:       "arn:aws:apigateway:us-east-1::/apis/gwid9876",
					Namespace: "apigateway",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 2",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			apigateway := ServiceFilters["AWS/ApiGateway"]

			outputResources, err := apigateway.FilterFunc(context.Background(), test.iface, test.inputResources)
			if err != nil {
				t.Logf("Error from FilterFunc: %v", err)
				t.FailNow()
			}
			if len(outputResources) != len(test.outputResources) {
				t.Logf("len(outputResources) = %d, want %d", len(outputResources), len(test.outputResources))
				t.Fail()
			}
			for i, resource := range outputResources {
				if len(test.outputResources) <= i {
					break
				}
				wantResource := *test.outputResources[i]
				if !reflect.DeepEqual(*resource, wantResource) {
					t.Errorf("outputResources[%d] = %+v, want %+v", i, *resource, wantResource)
				}
			}
		})
	}
}

func TestDMSFilterFunc(t *testing.T) {
	tests := []struct {
		name            string
		iface           client
		inputResources  []*model.TaggedResource
		outputResources []*model.TaggedResource
	}{
		{
			"empty input resources",
			client{},
			[]*model.TaggedResource{},
			[]*model.TaggedResource{},
		},
		{
			"replication tasks and instances",
			client{
				dmsAPI: dmsClient{
					describeReplicationInstancesOutput: &databasemigrationservice.DescribeReplicationInstancesOutput{
						ReplicationInstances: []*databasemigrationservice.ReplicationInstance{
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
					describeReplicationTasksOutput: &databasemigrationservice.DescribeReplicationTasksOutput{
						ReplicationTasks: []*databasemigrationservice.ReplicationTask{
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
				},
			},
			[]*model.TaggedResource{
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:rep:ABCDEFG1234567890",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:rep:WXYZ987654321",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 2",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:task:9999999999999999",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 3",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:task:5555555555555555",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 4",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:subgrp:demo-subgrp",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 5",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:endpoint:1111111111111111",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 6",
						},
					},
				},
			},
			[]*model.TaggedResource{
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:rep:ABCDEFG1234567890/repl-instance-identifier-1",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:rep:WXYZ987654321",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 2",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:task:9999999999999999/repl-instance-identifier-2",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 3",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:task:5555555555555555",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 4",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:subgrp:demo-subgrp",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 5",
						},
					},
				},
				{
					ARN:       "arn:aws:dms:us-east-1:123123123123:endpoint:1111111111111111",
					Namespace: "dms",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{
							Key:   "Test",
							Value: "Value 6",
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dms := ServiceFilters["AWS/DMS"]

			outputResources, err := dms.FilterFunc(context.Background(), test.iface, test.inputResources)
			if err != nil {
				t.Logf("Error from FilterFunc: %v", err)
				t.FailNow()
			}
			if len(outputResources) != len(test.outputResources) {
				t.Logf("len(outputResources) = %d, want %d", len(outputResources), len(test.outputResources))
				t.Fail()
			}
			for i, resource := range outputResources {
				if len(test.outputResources) <= i {
					break
				}
				wantResource := *test.outputResources[i]
				if !reflect.DeepEqual(*resource, wantResource) {
					t.Errorf("outputResources[%d] = %+v, want %+v", i, *resource, wantResource)
				}
			}
		})
	}
}

type apiGatewayClient struct {
	apigatewayiface.APIGatewayAPI
	getRestApisOutput *apigateway.GetRestApisOutput
}

func (apigateway apiGatewayClient) GetRestApisPagesWithContext(_ aws.Context, _ *apigateway.GetRestApisInput, fn func(*apigateway.GetRestApisOutput, bool) bool, _ ...request.Option) error {
	fn(apigateway.getRestApisOutput, true)
	return nil
}

type apiGatewayV2Client struct {
	apigatewayv2iface.ApiGatewayV2API
	getRestApisOutput *apigatewayv2.GetApisOutput
}

func (apigateway apiGatewayV2Client) GetApisWithContext(_ aws.Context, _ *apigatewayv2.GetApisInput, _ ...request.Option) (*apigatewayv2.GetApisOutput, error) {
	return apigateway.getRestApisOutput, nil
}

type dmsClient struct {
	databasemigrationserviceiface.DatabaseMigrationServiceAPI
	describeReplicationInstancesOutput *databasemigrationservice.DescribeReplicationInstancesOutput
	describeReplicationTasksOutput     *databasemigrationservice.DescribeReplicationTasksOutput
}

func (dms dmsClient) DescribeReplicationInstancesPagesWithContext(_ aws.Context, _ *databasemigrationservice.DescribeReplicationInstancesInput, fn func(*databasemigrationservice.DescribeReplicationInstancesOutput, bool) bool, _ ...request.Option) error {
	fn(dms.describeReplicationInstancesOutput, true)
	return nil
}

func (dms dmsClient) DescribeReplicationTasksPagesWithContext(_ aws.Context, _ *databasemigrationservice.DescribeReplicationTasksInput, fn func(*databasemigrationservice.DescribeReplicationTasksOutput, bool) bool, _ ...request.Option) error {
	fn(dms.describeReplicationTasksOutput, true)
	return nil
}
