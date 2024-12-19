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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/apigatewayv2"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/prometheusservice"
	"github.com/aws/aws-sdk-go/service/shield"
	"github.com/aws/aws-sdk-go/service/storagegateway"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

type ServiceFilter struct {
	// ResourceFunc can be used to fetch additional resources
	ResourceFunc func(context.Context, client, model.DiscoveryJob, string) ([]*model.TaggedResource, error)

	// FilterFunc can be used to the input resources or to drop based on some condition
	FilterFunc func(context.Context, client, []*model.TaggedResource) ([]*model.TaggedResource, error)
}

// ServiceFilters maps a service namespace to (optional) ServiceFilter
var ServiceFilters = map[string]ServiceFilter{
	"AWS/ApiGateway": {
		// ApiGateway ARNs use the Id (for v1 REST APIs) and ApiId (for v2 APIs) instead of
		// the ApiName (display name). See https://docs.aws.amazon.com/apigateway/latest/developerguide/arn-format-reference.html
		// However, in metrics, the ApiId dimension uses the ApiName as value.
		//
		// Here we use the ApiGateway API to map resource correctly. For backward compatibility,
		// in v1 REST APIs we change the ARN to replace the ApiId with ApiName, while for v2 APIs
		// we leave the ARN as-is.
		FilterFunc: func(ctx context.Context, client client, inputResources []*model.TaggedResource) ([]*model.TaggedResource, error) {
			var limit int64 = 500 // max number of results per page. default=25, max=500
			const maxPages = 10
			input := apigateway.GetRestApisInput{Limit: &limit}
			output := apigateway.GetRestApisOutput{}
			var pageNum int

			err := client.apiGatewayAPI.GetRestApisPagesWithContext(ctx, &input, func(page *apigateway.GetRestApisOutput, _ bool) bool {
				promutil.APIGatewayAPICounter.Inc()
				pageNum++
				output.Items = append(output.Items, page.Items...)
				return pageNum <= maxPages
			})
			if err != nil {
				return nil, fmt.Errorf("error calling apiGatewayAPI.GetRestApisPages, %w", err)
			}

			outputV2, err := client.apiGatewayV2API.GetApisWithContext(ctx, &apigatewayv2.GetApisInput{})
			promutil.APIGatewayAPIV2Counter.Inc()
			if err != nil {
				return nil, fmt.Errorf("error calling apiGatewayAPIv2.GetApis, %w", err)
			}

			var outputResources []*model.TaggedResource
			for _, resource := range inputResources {
				for i, gw := range output.Items {
					if strings.HasSuffix(resource.ARN, "/restapis/"+*gw.Id) {
						r := resource
						r.ARN = strings.ReplaceAll(resource.ARN, *gw.Id, *gw.Name)
						outputResources = append(outputResources, r)
						output.Items = append(output.Items[:i], output.Items[i+1:]...)
						break
					}
				}
				for i, gw := range outputV2.Items {
					if strings.HasSuffix(resource.ARN, "/apis/"+*gw.ApiId) {
						outputResources = append(outputResources, resource)
						outputV2.Items = append(outputV2.Items[:i], outputV2.Items[i+1:]...)
						break
					}
				}
			}
			return outputResources, nil
		},
	},
	"AWS/AutoScaling": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			pageNum := 0
			var resources []*model.TaggedResource
			err := client.autoscalingAPI.DescribeAutoScalingGroupsPagesWithContext(ctx, &autoscaling.DescribeAutoScalingGroupsInput{},
				func(page *autoscaling.DescribeAutoScalingGroupsOutput, _ bool) bool {
					pageNum++
					promutil.AutoScalingAPICounter.Inc()

					for _, asg := range page.AutoScalingGroups {
						resource := model.TaggedResource{
							ARN:       aws.StringValue(asg.AutoScalingGroupARN),
							Namespace: job.Type,
							Region:    region,
						}

						for _, t := range asg.Tags {
							resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
						}

						if resource.FilterThroughTags(job.SearchTags) {
							resources = append(resources, &resource)
						}
					}
					return pageNum < 100
				},
			)
			if err != nil {
				return nil, fmt.Errorf("error calling autoscalingAPI.DescribeAutoScalingGroups, %w", err)
			}
			return resources, nil
		},
	},
	"AWS/DMS": {
		// Append the replication instance identifier to DMS task and instance ARNs
		FilterFunc: func(ctx context.Context, client client, inputResources []*model.TaggedResource) ([]*model.TaggedResource, error) {
			if len(inputResources) == 0 {
				return inputResources, nil
			}

			replicationInstanceIdentifiers := make(map[string]string)
			pageNum := 0
			if err := client.dmsAPI.DescribeReplicationInstancesPagesWithContext(ctx, nil,
				func(page *databasemigrationservice.DescribeReplicationInstancesOutput, _ bool) bool {
					pageNum++
					promutil.DmsAPICounter.Inc()

					for _, instance := range page.ReplicationInstances {
						replicationInstanceIdentifiers[aws.StringValue(instance.ReplicationInstanceArn)] = aws.StringValue(instance.ReplicationInstanceIdentifier)
					}

					return pageNum < 100
				},
			); err != nil {
				return nil, fmt.Errorf("error calling dmsAPI.DescribeReplicationInstances, %w", err)
			}
			pageNum = 0
			if err := client.dmsAPI.DescribeReplicationTasksPagesWithContext(ctx, nil,
				func(page *databasemigrationservice.DescribeReplicationTasksOutput, _ bool) bool {
					pageNum++
					promutil.DmsAPICounter.Inc()

					for _, task := range page.ReplicationTasks {
						taskInstanceArn := aws.StringValue(task.ReplicationInstanceArn)
						if instanceIdentifier, ok := replicationInstanceIdentifiers[taskInstanceArn]; ok {
							replicationInstanceIdentifiers[aws.StringValue(task.ReplicationTaskArn)] = instanceIdentifier
						}
					}

					return pageNum < 100
				},
			); err != nil {
				return nil, fmt.Errorf("error calling dmsAPI.DescribeReplicationTasks, %w", err)
			}

			var outputResources []*model.TaggedResource
			for _, resource := range inputResources {
				r := resource
				// Append the replication instance identifier to replication instance and task ARNs
				if instanceIdentifier, ok := replicationInstanceIdentifiers[r.ARN]; ok {
					r.ARN = fmt.Sprintf("%s/%s", r.ARN, instanceIdentifier)
				}
				outputResources = append(outputResources, r)
			}
			return outputResources, nil
		},
	},
	"AWS/EC2Spot": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			pageNum := 0
			var resources []*model.TaggedResource
			err := client.ec2API.DescribeSpotFleetRequestsPagesWithContext(ctx, &ec2.DescribeSpotFleetRequestsInput{},
				func(page *ec2.DescribeSpotFleetRequestsOutput, _ bool) bool {
					pageNum++
					promutil.Ec2APICounter.Inc()

					for _, ec2Spot := range page.SpotFleetRequestConfigs {
						resource := model.TaggedResource{
							ARN:       aws.StringValue(ec2Spot.SpotFleetRequestId),
							Namespace: job.Type,
							Region:    region,
						}

						for _, t := range ec2Spot.Tags {
							resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
						}

						if resource.FilterThroughTags(job.SearchTags) {
							resources = append(resources, &resource)
						}
					}
					return pageNum < 100
				},
			)
			if err != nil {
				return nil, fmt.Errorf("error calling describing ec2API.DescribeSpotFleetRequests, %w", err)
			}
			return resources, nil
		},
	},
	"AWS/Prometheus": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			pageNum := 0
			var resources []*model.TaggedResource
			err := client.prometheusSvcAPI.ListWorkspacesPagesWithContext(ctx, &prometheusservice.ListWorkspacesInput{},
				func(page *prometheusservice.ListWorkspacesOutput, _ bool) bool {
					pageNum++
					promutil.ManagedPrometheusAPICounter.Inc()

					for _, ws := range page.Workspaces {
						resource := model.TaggedResource{
							ARN:       aws.StringValue(ws.Arn),
							Namespace: job.Type,
							Region:    region,
						}

						for key, value := range ws.Tags {
							resource.Tags = append(resource.Tags, model.Tag{Key: key, Value: *value})
						}

						if resource.FilterThroughTags(job.SearchTags) {
							resources = append(resources, &resource)
						}
					}
					return pageNum < 100
				},
			)
			if err != nil {
				return nil, fmt.Errorf("error while calling prometheusSvcAPI.ListWorkspaces, %w", err)
			}
			return resources, nil
		},
	},
	"AWS/StorageGateway": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			pageNum := 0
			var resources []*model.TaggedResource
			err := client.storageGatewayAPI.ListGatewaysPagesWithContext(ctx, &storagegateway.ListGatewaysInput{},
				func(page *storagegateway.ListGatewaysOutput, _ bool) bool {
					pageNum++
					promutil.StoragegatewayAPICounter.Inc()

					for _, gwa := range page.Gateways {
						resource := model.TaggedResource{
							ARN:       fmt.Sprintf("%s/%s", *gwa.GatewayId, *gwa.GatewayName),
							Namespace: job.Type,
							Region:    region,
						}

						tagsRequest := &storagegateway.ListTagsForResourceInput{
							ResourceARN: gwa.GatewayARN,
						}
						tagsResponse, _ := client.storageGatewayAPI.ListTagsForResource(tagsRequest)
						promutil.StoragegatewayAPICounter.Inc()

						for _, t := range tagsResponse.Tags {
							resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
						}

						if resource.FilterThroughTags(job.SearchTags) {
							resources = append(resources, &resource)
						}
					}

					return pageNum < 100
				},
			)
			if err != nil {
				return nil, fmt.Errorf("error calling storageGatewayAPI.ListGateways, %w", err)
			}
			return resources, nil
		},
	},
	"AWS/TransitGateway": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			pageNum := 0
			var resources []*model.TaggedResource
			err := client.ec2API.DescribeTransitGatewayAttachmentsPagesWithContext(ctx, &ec2.DescribeTransitGatewayAttachmentsInput{},
				func(page *ec2.DescribeTransitGatewayAttachmentsOutput, _ bool) bool {
					pageNum++
					promutil.Ec2APICounter.Inc()

					for _, tgwa := range page.TransitGatewayAttachments {
						resource := model.TaggedResource{
							ARN:       fmt.Sprintf("%s/%s", *tgwa.TransitGatewayId, *tgwa.TransitGatewayAttachmentId),
							Namespace: job.Type,
							Region:    region,
						}

						for _, t := range tgwa.Tags {
							resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
						}

						if resource.FilterThroughTags(job.SearchTags) {
							resources = append(resources, &resource)
						}
					}
					return pageNum < 100
				},
			)
			if err != nil {
				return nil, fmt.Errorf("error calling ec2API.DescribeTransitGatewayAttachments, %w", err)
			}
			return resources, nil
		},
	},
	"AWS/DDoSProtection": {
		// Resource discovery only targets the protections, protections are global, so they will only be discoverable in us-east-1.
		// Outside us-east-1 no resources are going to be found. We use the shield.ListProtections API to get the protections +
		// protected resources to add to the tagged resources. This data is eventually usable for joining with metrics.
		ResourceFunc: func(ctx context.Context, c client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			var output []*model.TaggedResource
			pageNum := 0
			// Default page size is only 20 which can easily lead to throttling
			input := &shield.ListProtectionsInput{MaxResults: aws.Int64(1000)}
			err := c.shieldAPI.ListProtectionsPagesWithContext(ctx, input, func(page *shield.ListProtectionsOutput, _ bool) bool {
				promutil.ShieldAPICounter.Inc()
				for _, protection := range page.Protections {
					protectedResourceArn := *protection.ResourceArn
					protectionArn := *protection.ProtectionArn
					protectedResource, err := arn.Parse(protectedResourceArn)
					if err != nil {
						continue
					}

					// Shield covers regional services,
					// 		EC2 (arn:aws:ec2:<REGION>:<ACCOUNT_ID>:eip-allocation/*)
					// 		load balancers (arn:aws:elasticloadbalancing:<REGION>:<ACCOUNT_ID>:loadbalancer:*)
					// 	where the region of the protectedResource ARN should match the region for the job to prevent
					// 	duplicating resources across all regions
					// Shield also covers other global services,
					// 		global accelerator (arn:aws:globalaccelerator::<ACCOUNT_ID>:accelerator/*)
					//		route53 (arn:aws:route53:::hostedzone/*)
					//	where the protectedResource contains no region. Just like other global services the metrics for
					//	these land in us-east-1 so any protected resource without a region should be added when the job
					//	is for us-east-1
					if protectedResource.Region == region || (protectedResource.Region == "" && region == "us-east-1") {
						taggedResource := &model.TaggedResource{
							ARN:       protectedResourceArn,
							Namespace: job.Type,
							Region:    region,
							Tags:      []model.Tag{{Key: "ProtectionArn", Value: protectionArn}},
						}
						output = append(output, taggedResource)
					}
				}
				return pageNum < 100
			})
			if err != nil {
				return nil, fmt.Errorf("error calling shiled.ListProtections, %w", err)
			}
			return output, nil
		},
	},
}
