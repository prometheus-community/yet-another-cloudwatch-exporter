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
	"maps"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/shield"
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type ServiceFilter struct {
	// ResourceFunc can be used to fetch additional resources
	ResourceFunc func(context.Context, client, model.DiscoveryJob, string) ([]*model.TaggedResource, error)

	// FilterFunc can be used to modify the input resources or to drop based on some condition
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
			const maxPages = 10
			const maxResultsPerPage = 500 // increased to reduce throttling occurrences.
			pageNum := 0

			input := apigateway.GetRestApisInput{Limit: aws.Int32(maxResultsPerPage)}
			output := apigateway.GetRestApisOutput{}
			paginator := apigateway.NewGetRestApisPaginator(client.apiGatewayAPI, &input, func(options *apigateway.GetRestApisPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for paginator.HasMorePages() && pageNum <= maxPages {
				page, err := paginator.NextPage(ctx)
				client.scrapeMetrics.APIGatewayAPICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling apiGatewayAPI.GetRestApis, %w", err)
				}
				pageNum++
				output.Items = append(output.Items, page.Items...)
			}

			outputV2, err := client.apiGatewayV2API.GetApis(ctx, &apigatewayv2.GetApisInput{})
			if err != nil {
				return nil, fmt.Errorf("error calling apigatewayv2.GetApis, %w", err)
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
			const maxPages = 10
			pageNum := 0

			var resources []*model.TaggedResource
			paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(client.autoscalingAPI, &autoscaling.DescribeAutoScalingGroupsInput{}, func(options *autoscaling.DescribeAutoScalingGroupsPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for paginator.HasMorePages() && pageNum < maxPages {
				page, err := paginator.NextPage(ctx)
				client.scrapeMetrics.AutoScalingAPICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling autoscalingAPI.DescribeAutoScalingGroups, %w", err)
				}
				pageNum++

				for _, asg := range page.AutoScalingGroups {
					resource := model.TaggedResource{
						ARN:       *asg.AutoScalingGroupARN,
						Namespace: job.Namespace,
						Region:    region,
					}

					for _, t := range asg.Tags {
						resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
					}

					if resource.FilterThroughTags(job.SearchTags) {
						resources = append(resources, &resource)
					}
				}
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

			const maxInstancesPages = 100
			pageNum := 0

			replicationInstanceIdentifiers := make(map[string]string)

			instancesPaginator := databasemigrationservice.NewDescribeReplicationInstancesPaginator(client.dmsAPI, &databasemigrationservice.DescribeReplicationInstancesInput{}, func(options *databasemigrationservice.DescribeReplicationInstancesPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for instancesPaginator.HasMorePages() && pageNum < maxInstancesPages {
				page, err := instancesPaginator.NextPage(ctx)
				client.scrapeMetrics.DmsAPICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling dmsAPI.DescribeReplicationInstances, %w", err)
				}
				pageNum++

				for _, instance := range page.ReplicationInstances {
					replicationInstanceIdentifiers[*instance.ReplicationInstanceArn] = *instance.ReplicationInstanceIdentifier
				}
			}

			const maxTasksPages = 100
			pageNum = 0

			tasksPaginator := databasemigrationservice.NewDescribeReplicationTasksPaginator(client.dmsAPI, &databasemigrationservice.DescribeReplicationTasksInput{}, func(options *databasemigrationservice.DescribeReplicationTasksPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for tasksPaginator.HasMorePages() && pageNum < maxTasksPages {
				page, err := tasksPaginator.NextPage(ctx)
				client.scrapeMetrics.DmsAPICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling dmsAPI.DescribeReplicationTasks, %w", err)
				}
				pageNum++

				for _, task := range page.ReplicationTasks {
					taskInstanceArn := *task.ReplicationInstanceArn
					if instanceIdentifier, ok := replicationInstanceIdentifiers[taskInstanceArn]; ok {
						replicationInstanceIdentifiers[*task.ReplicationTaskArn] = instanceIdentifier
					}
				}
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
			const maxPages = 100
			pageNum := 0

			var resources []*model.TaggedResource
			paginator := ec2.NewDescribeSpotFleetRequestsPaginator(client.ec2API, &ec2.DescribeSpotFleetRequestsInput{}, func(options *ec2.DescribeSpotFleetRequestsPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for paginator.HasMorePages() && pageNum < maxPages {
				page, err := paginator.NextPage(ctx)
				client.scrapeMetrics.Ec2APICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling describing ec2API.DescribeSpotFleetRequests, %w", err)
				}
				pageNum++

				for _, ec2Spot := range page.SpotFleetRequestConfigs {
					resource := model.TaggedResource{
						ARN:       *ec2Spot.SpotFleetRequestId,
						Namespace: job.Namespace,
						Region:    region,
					}

					for _, t := range ec2Spot.Tags {
						resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
					}

					if resource.FilterThroughTags(job.SearchTags) {
						resources = append(resources, &resource)
					}
				}
			}

			return resources, nil
		},
	},
	"AWS/Prometheus": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			const maxPages = 100
			pageNum := 0

			var resources []*model.TaggedResource
			paginator := amp.NewListWorkspacesPaginator(client.prometheusSvcAPI, &amp.ListWorkspacesInput{}, func(options *amp.ListWorkspacesPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for paginator.HasMorePages() && pageNum < maxPages {
				page, err := paginator.NextPage(ctx)
				client.scrapeMetrics.ManagedPrometheusAPICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error while calling prometheusSvcAPI.ListWorkspaces, %w", err)
				}
				pageNum++

				for _, ws := range page.Workspaces {
					resource := model.TaggedResource{
						ARN:       *ws.Arn,
						Namespace: job.Namespace,
						Region:    region,
					}

					for key, value := range ws.Tags {
						resource.Tags = append(resource.Tags, model.Tag{Key: key, Value: value})
					}

					if resource.FilterThroughTags(job.SearchTags) {
						resources = append(resources, &resource)
					}
				}
			}

			return resources, nil
		},
	},
	"AWS/StorageGateway": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			const maxPages = 100
			pageNum := 0

			var resources []*model.TaggedResource
			paginator := storagegateway.NewListGatewaysPaginator(client.storageGatewayAPI, &storagegateway.ListGatewaysInput{}, func(options *storagegateway.ListGatewaysPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for paginator.HasMorePages() && pageNum < maxPages {
				page, err := paginator.NextPage(ctx)
				client.scrapeMetrics.StoragegatewayAPICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling storageGatewayAPI.ListGateways, %w", err)
				}
				pageNum++

				for _, gwa := range page.Gateways {
					resource := model.TaggedResource{
						ARN:       fmt.Sprintf("%s/%s", *gwa.GatewayId, *gwa.GatewayName),
						Namespace: job.Namespace,
						Region:    region,
					}

					tagsRequest := &storagegateway.ListTagsForResourceInput{
						ResourceARN: gwa.GatewayARN,
					}
					tagsResponse, _ := client.storageGatewayAPI.ListTagsForResource(ctx, tagsRequest)
					client.scrapeMetrics.StoragegatewayAPICounter.Inc()

					for _, t := range tagsResponse.Tags {
						resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
					}

					if resource.FilterThroughTags(job.SearchTags) {
						resources = append(resources, &resource)
					}
				}
			}

			return resources, nil
		},
	},
	"AWS/TransitGateway": {
		ResourceFunc: func(ctx context.Context, client client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			const maxPages = 100
			pageNum := 0

			var resources []*model.TaggedResource
			paginator := ec2.NewDescribeTransitGatewayAttachmentsPaginator(client.ec2API, &ec2.DescribeTransitGatewayAttachmentsInput{}, func(options *ec2.DescribeTransitGatewayAttachmentsPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})

			for paginator.HasMorePages() && pageNum < maxPages {
				page, err := paginator.NextPage(ctx)
				client.scrapeMetrics.Ec2APICounter.Inc()
				if err != nil {
					return nil, fmt.Errorf("error calling ec2API.DescribeTransitGatewayAttachments, %w", err)
				}
				pageNum++

				for _, tgwa := range page.TransitGatewayAttachments {
					resource := model.TaggedResource{
						ARN:       fmt.Sprintf("%s/%s", *tgwa.TransitGatewayId, *tgwa.TransitGatewayAttachmentId),
						Namespace: job.Namespace,
						Region:    region,
					}

					for _, t := range tgwa.Tags {
						resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
					}

					if resource.FilterThroughTags(job.SearchTags) {
						resources = append(resources, &resource)
					}
				}
			}

			return resources, nil
		},
	},
	"AWS/DDoSProtection": {
		// Resource discovery only targets the protections, protections are global, so they will only be discoverable in us-east-1.
		// Outside us-east-1 no resources are going to be found. We use the shield.ListProtections API to get the protections +
		// protected resources to add to the tagged resources. This data is eventually usable for joining with metrics.
		ResourceFunc: func(ctx context.Context, c client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			const maxPages = 100
			const maxResultsPerPage int32 = 100 // increased to reduce throttling occurrences.
			request := &shield.ListProtectionsInput{MaxResults: aws.Int32(maxResultsPerPage)}

			var output []*model.TaggedResource
			paginator := shield.NewListProtectionsPaginator(c.shieldAPI, request, func(options *shield.ListProtectionsPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})
			pageNum := 0
			for paginator.HasMorePages() && pageNum < maxPages {
				c.scrapeMetrics.ShieldAPICounter.Inc()
				page, err := paginator.NextPage(ctx)
				pageNum++
				if err != nil {
					return nil, fmt.Errorf("error calling shieldAPI.ListProtections, %w", err)
				}

				for _, protection := range page.Protections {
					protectedResourceArn := *protection.ResourceArn
					protectionArn := *protection.ProtectionArn
					protectedResource, err := arn.Parse(protectedResourceArn)
					if err != nil {
						return nil, fmt.Errorf("shieldAPI.ListProtections returned an invalid ProtectedResourceArn %s for Protection %s", protectedResourceArn, protectionArn)
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
							Namespace: job.Namespace,
							Region:    region,
							Tags:      []model.Tag{{Key: "ProtectionArn", Value: protectionArn}},
						}
						output = append(output, taggedResource)
					}
				}
			}

			return output, nil
		},
	},
	"AWS/Backup": {
		// Some copy job metrics are published in the region of the source backup vault, but has the destination vault name
		// as the BackupVaultName dimension which may not be in the same region. Add both the source and destination vaults
		// for copy jobs sourced from this region
		ResourceFunc: func(ctx context.Context, c client, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
			const maxPages = 100
			const maxResultsPerPage int32 = 100
			request := &backup.ListCopyJobsInput{MaxResults: aws.Int32(maxResultsPerPage)}

			var resources []*model.TaggedResource
			seen := map[string]struct{}{}
			paginator := backup.NewListCopyJobsPaginator(c.backupAPI, request, func(options *backup.ListCopyJobsPaginatorOptions) {
				options.StopOnDuplicateToken = true
			})
			pageNum := 0
			for paginator.HasMorePages() && pageNum < maxPages {
				c.scrapeMetrics.BackupAPICounter.Inc()
				page, err := paginator.NextPage(ctx)
				pageNum++
				if err != nil {
					return nil, fmt.Errorf("error calling backupAPI.ListCopyJobs, %w", err)
				}

				for _, copyJob := range page.CopyJobs {
					sourceBackupVaultArn := *copyJob.SourceBackupVaultArn
					destinationBackupVaultArn := *copyJob.DestinationBackupVaultArn

					sourceBackupVaultArnParsed, err := arn.Parse(*copyJob.SourceBackupVaultArn)
					if err != nil {
						return nil, fmt.Errorf("backupAPI.ListCopyJobs returned an invalid SourceBackupVaultArn %s", sourceBackupVaultArn)
					}

					// The metrics are published in the source region so only keep jobs sourced that belong to the current region
					if sourceBackupVaultArnParsed.Region != region {
						continue
					}

					destinationBackupVaultArnParsed, err := arn.Parse(*copyJob.DestinationBackupVaultArn)
					if err != nil {
						return nil, fmt.Errorf("backupAPI.ListCopyJobs returned an invalid DestinationBackupVaultArn %s", destinationBackupVaultArn)
					}

					// It's possible to have the same destination for multiple copy jobs so there may be duplicates
					// Using the resource name prevents collision (matches the dimension regexp) if destination matches a source
					if _, ok := seen[destinationBackupVaultArnParsed.Resource]; ok {
						continue
					}

					sourceResource := &model.TaggedResource{
						ARN:       sourceBackupVaultArn,
						Namespace: job.Namespace,
						Region:    region,
					}
					sourceBackupVaultTags, err := getBackupTags(ctx, c, sourceBackupVaultArn)
					if err != nil {
						return nil, fmt.Errorf("error retrieving tags for source backup vault %s: %w", sourceBackupVaultArn, err)
					}

					for key, value := range sourceBackupVaultTags {
						sourceResource.Tags = append(sourceResource.Tags, model.Tag{Key: key, Value: value})
					}

					// Only add the destination if the source wasn't filtered out
					if sourceResource.FilterThroughTags(job.SearchTags) {
						destinationResource := &model.TaggedResource{
							ARN:       destinationBackupVaultArn,
							Namespace: job.Namespace,
							Region:    region, // TODO: destinationBackupVaultArnParsed.Region?
						}

						// TODO: This requires a client that can connect to a different region
						destinationBackupVaultTags, err := getBackupTags(ctx, c, destinationBackupVaultArn)
						if err != nil {
							return nil, fmt.Errorf("error retrieving tags for destination backup vault %s: %w", destinationBackupVaultArn, err)
						}

						for key, value := range destinationBackupVaultTags {
							destinationResource.Tags = append(destinationResource.Tags, model.Tag{Key: key, Value: value})
						}

						// destination is a by-product of source so don't filter

						resources = append(resources, sourceResource)
						resources = append(resources, destinationResource)

						seen[sourceBackupVaultArnParsed.Resource] = struct{}{}
						seen[destinationBackupVaultArnParsed.Resource] = struct{}{}
					}
				}
			}

			return resources, nil
		},
	},
}

func getBackupTags(ctx context.Context, c client, resourceArn string) (map[string]string, error) {
	const maxPages = 100
	const maxResultsPerPage int32 = 100 // increased to reduce throttling occurrences.
	request := &backup.ListTagsInput{
		ResourceArn: &resourceArn,
		MaxResults:  aws.Int32(maxResultsPerPage),
	}

	tags := map[string]string{}
	paginator := backup.NewListTagsPaginator(c.backupAPI, request, func(options *backup.ListTagsPaginatorOptions) {
		options.StopOnDuplicateToken = true
	})
	pageNum := 0
	for paginator.HasMorePages() && pageNum < maxPages {
		c.scrapeMetrics.BackupAPICounter.Inc()
		page, err := paginator.NextPage(ctx)
		pageNum++
		if err != nil {
			return nil, fmt.Errorf("error calling backupAPI.ListTags for %s: %w", resourceArn, err)
		}

		maps.Copy(tags, page.Tags)
	}

	return tags, nil
}
