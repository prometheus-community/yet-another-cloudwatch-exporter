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

// This file contains closure-based adapters for each AWS SDK client used by the
// tagging package.  Storing concrete SDK clients (e.g. *ec2.Client) as struct
// fields on a type that is itself boxed into an interface causes the Go linker
// to retain every exported method of those clients, defeating dead-code
// elimination.  Capturing only the used operations as method-value closures
// keeps the concrete clients out of any interface-reachable struct field, so
// the linker can drop the unused operations.

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/shield"
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"
)

// taggingClientAdapter wraps *resourcegroupstaggingapi.Client via closures.
type taggingClientAdapter struct {
	getResources func(ctx context.Context, params *resourcegroupstaggingapi.GetResourcesInput, optFns ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}

func newTaggingClientAdapter(c *resourcegroupstaggingapi.Client) taggingClientAdapter {
	return taggingClientAdapter{getResources: c.GetResources}
}

func (a taggingClientAdapter) GetResources(ctx context.Context, params *resourcegroupstaggingapi.GetResourcesInput, optFns ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error) {
	return a.getResources(ctx, params, optFns...)
}

// autoscalingClientAdapter wraps *autoscaling.Client via closures.
type autoscalingClientAdapter struct {
	describeAutoScalingGroups func(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

func newAutoscalingClientAdapter(c *autoscaling.Client) autoscalingClientAdapter {
	return autoscalingClientAdapter{describeAutoScalingGroups: c.DescribeAutoScalingGroups}
}

func (a autoscalingClientAdapter) DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	return a.describeAutoScalingGroups(ctx, params, optFns...)
}

// apiGatewayClientAdapter wraps *apigateway.Client via closures.
type apiGatewayClientAdapter struct {
	getRestApis func(ctx context.Context, params *apigateway.GetRestApisInput, optFns ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error)
}

func newAPIGatewayClientAdapter(c *apigateway.Client) apiGatewayClientAdapter {
	return apiGatewayClientAdapter{getRestApis: c.GetRestApis}
}

func (a apiGatewayClientAdapter) GetRestApis(ctx context.Context, params *apigateway.GetRestApisInput, optFns ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error) {
	return a.getRestApis(ctx, params, optFns...)
}

// apiGatewayV2ClientAdapter wraps *apigatewayv2.Client via closures.
type apiGatewayV2ClientAdapter struct {
	getApis func(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
}

func newAPIGatewayV2ClientAdapter(c *apigatewayv2.Client) apiGatewayV2ClientAdapter {
	return apiGatewayV2ClientAdapter{getApis: c.GetApis}
}

func (a apiGatewayV2ClientAdapter) GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return a.getApis(ctx, params, optFns...)
}

// ec2ClientAdapter wraps *ec2.Client via closures.
type ec2ClientAdapter struct {
	describeSpotFleetRequests         func(ctx context.Context, params *ec2.DescribeSpotFleetRequestsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSpotFleetRequestsOutput, error)
	describeTransitGatewayAttachments func(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error)
}

func newEC2ClientAdapter(c *ec2.Client) ec2ClientAdapter {
	return ec2ClientAdapter{
		describeSpotFleetRequests:         c.DescribeSpotFleetRequests,
		describeTransitGatewayAttachments: c.DescribeTransitGatewayAttachments,
	}
}

func (a ec2ClientAdapter) DescribeSpotFleetRequests(ctx context.Context, params *ec2.DescribeSpotFleetRequestsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSpotFleetRequestsOutput, error) {
	return a.describeSpotFleetRequests(ctx, params, optFns...)
}

func (a ec2ClientAdapter) DescribeTransitGatewayAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	return a.describeTransitGatewayAttachments(ctx, params, optFns...)
}

// dmsClientAdapter wraps *databasemigrationservice.Client via closures.
type dmsClientAdapter struct {
	describeReplicationInstances func(ctx context.Context, params *databasemigrationservice.DescribeReplicationInstancesInput, optFns ...func(*databasemigrationservice.Options)) (*databasemigrationservice.DescribeReplicationInstancesOutput, error)
	describeReplicationTasks     func(ctx context.Context, params *databasemigrationservice.DescribeReplicationTasksInput, optFns ...func(*databasemigrationservice.Options)) (*databasemigrationservice.DescribeReplicationTasksOutput, error)
}

func newDMSClientAdapter(c *databasemigrationservice.Client) dmsClientAdapter {
	return dmsClientAdapter{
		describeReplicationInstances: c.DescribeReplicationInstances,
		describeReplicationTasks:     c.DescribeReplicationTasks,
	}
}

func (a dmsClientAdapter) DescribeReplicationInstances(ctx context.Context, params *databasemigrationservice.DescribeReplicationInstancesInput, optFns ...func(*databasemigrationservice.Options)) (*databasemigrationservice.DescribeReplicationInstancesOutput, error) {
	return a.describeReplicationInstances(ctx, params, optFns...)
}

func (a dmsClientAdapter) DescribeReplicationTasks(ctx context.Context, params *databasemigrationservice.DescribeReplicationTasksInput, optFns ...func(*databasemigrationservice.Options)) (*databasemigrationservice.DescribeReplicationTasksOutput, error) {
	return a.describeReplicationTasks(ctx, params, optFns...)
}

// prometheusClientAdapter wraps *amp.Client via closures.
type prometheusClientAdapter struct {
	listWorkspaces func(ctx context.Context, params *amp.ListWorkspacesInput, optFns ...func(*amp.Options)) (*amp.ListWorkspacesOutput, error)
}

func newPrometheusClientAdapter(c *amp.Client) prometheusClientAdapter {
	return prometheusClientAdapter{listWorkspaces: c.ListWorkspaces}
}

func (a prometheusClientAdapter) ListWorkspaces(ctx context.Context, params *amp.ListWorkspacesInput, optFns ...func(*amp.Options)) (*amp.ListWorkspacesOutput, error) {
	return a.listWorkspaces(ctx, params, optFns...)
}

// storageGatewayClientAdapter wraps *storagegateway.Client via closures.
type storageGatewayClientAdapter struct {
	listGateways        func(ctx context.Context, params *storagegateway.ListGatewaysInput, optFns ...func(*storagegateway.Options)) (*storagegateway.ListGatewaysOutput, error)
	listTagsForResource func(ctx context.Context, params *storagegateway.ListTagsForResourceInput, optFns ...func(*storagegateway.Options)) (*storagegateway.ListTagsForResourceOutput, error)
}

func newStorageGatewayClientAdapter(c *storagegateway.Client) storageGatewayClientAdapter {
	return storageGatewayClientAdapter{
		listGateways:        c.ListGateways,
		listTagsForResource: c.ListTagsForResource,
	}
}

func (a storageGatewayClientAdapter) ListGateways(ctx context.Context, params *storagegateway.ListGatewaysInput, optFns ...func(*storagegateway.Options)) (*storagegateway.ListGatewaysOutput, error) {
	return a.listGateways(ctx, params, optFns...)
}

func (a storageGatewayClientAdapter) ListTagsForResource(ctx context.Context, params *storagegateway.ListTagsForResourceInput, optFns ...func(*storagegateway.Options)) (*storagegateway.ListTagsForResourceOutput, error) {
	return a.listTagsForResource(ctx, params, optFns...)
}

// shieldClientAdapter wraps *shield.Client via closures.
type shieldClientAdapter struct {
	listProtections func(ctx context.Context, params *shield.ListProtectionsInput, optFns ...func(*shield.Options)) (*shield.ListProtectionsOutput, error)
}

func newShieldClientAdapter(c *shield.Client) shieldClientAdapter {
	return shieldClientAdapter{listProtections: c.ListProtections}
}

func (a shieldClientAdapter) ListProtections(ctx context.Context, params *shield.ListProtectionsInput, optFns ...func(*shield.Options)) (*shield.ListProtectionsOutput, error) {
	return a.listProtections(ctx, params, optFns...)
}
