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
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/aws-sdk-go-v2/service/shield"
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// Compile-time checks that the adapters satisfy the interfaces the AWS SDK
// paginators expect, catching any signature drift early.
var (
	_ resourcegroupstaggingapi.GetResourcesAPIClient                 = taggingClientAdapter{}
	_ autoscaling.DescribeAutoScalingGroupsAPIClient                 = autoscalingClientAdapter{}
	_ apigateway.GetRestApisAPIClient                                = apiGatewayClientAdapter{}
	_ ec2.DescribeSpotFleetRequestsAPIClient                         = ec2ClientAdapter{}
	_ ec2.DescribeTransitGatewayAttachmentsAPIClient                 = ec2ClientAdapter{}
	_ databasemigrationservice.DescribeReplicationInstancesAPIClient = dmsClientAdapter{}
	_ databasemigrationservice.DescribeReplicationTasksAPIClient     = dmsClientAdapter{}
	_ amp.ListWorkspacesAPIClient                                    = prometheusClientAdapter{}
	_ storagegateway.ListGatewaysAPIClient                           = storageGatewayClientAdapter{}
	_ shield.ListProtectionsAPIClient                                = shieldClientAdapter{}
)

type Client interface {
	GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error)
}

var ErrExpectedToFindResources = errors.New("expected to discover resources but none were found")

type client struct {
	logger            *slog.Logger
	scrapeMetrics     *promutil.ScrapeMetrics
	taggingAPI        taggingClientAdapter
	autoscalingAPI    autoscalingClientAdapter
	apiGatewayAPI     apiGatewayClientAdapter
	apiGatewayV2API   apiGatewayV2ClientAdapter
	ec2API            ec2ClientAdapter
	dmsAPI            dmsClientAdapter
	prometheusSvcAPI  prometheusClientAdapter
	storageGatewayAPI storageGatewayClientAdapter
	shieldAPI         shieldClientAdapter
}

func NewClient(
	logger *slog.Logger,
	scrapeMetrics *promutil.ScrapeMetrics,
	taggingAPI *resourcegroupstaggingapi.Client,
	autoscalingAPI *autoscaling.Client,
	apiGatewayAPI *apigateway.Client,
	apiGatewayV2API *apigatewayv2.Client,
	ec2API *ec2.Client,
	dmsClient *databasemigrationservice.Client,
	prometheusClient *amp.Client,
	storageGatewayAPI *storagegateway.Client,
	shieldAPI *shield.Client,
) Client {
	if scrapeMetrics == nil {
		scrapeMetrics = promutil.Discard
	}
	return &client{
		logger:            logger,
		scrapeMetrics:     scrapeMetrics,
		taggingAPI:        newTaggingClientAdapter(taggingAPI),
		autoscalingAPI:    newAutoscalingClientAdapter(autoscalingAPI),
		apiGatewayAPI:     newAPIGatewayClientAdapter(apiGatewayAPI),
		apiGatewayV2API:   newAPIGatewayV2ClientAdapter(apiGatewayV2API),
		ec2API:            newEC2ClientAdapter(ec2API),
		dmsAPI:            newDMSClientAdapter(dmsClient),
		prometheusSvcAPI:  newPrometheusClientAdapter(prometheusClient),
		storageGatewayAPI: newStorageGatewayClientAdapter(storageGatewayAPI),
		shieldAPI:         newShieldClientAdapter(shieldAPI),
	}
}

func (c client) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	svc := config.SupportedServices.GetService(job.Namespace)
	var resources []*model.TaggedResource
	shouldHaveDiscoveredResources := false

	if len(svc.ResourceFilters) > 0 {
		shouldHaveDiscoveredResources = true
		filters := make([]string, 0, len(svc.ResourceFilters))
		for _, filter := range svc.ResourceFilters {
			filters = append(filters, *filter)
		}
		var tagFilters []types.TagFilter
		if len(job.SearchTags) > 0 {
			for i := range job.SearchTags {
				// Because everything with the AWS APIs is pointers we need a pointer to the `Key` field from the SearchTag.
				// We can't take a pointer to any fields from loop variable or the pointer will always be the same and this logic will be broken.
				st := job.SearchTags[i]

				// AWS's GetResources has a TagFilter option which matches the semantics of our SearchTags where all filters must match
				// Their value matching implementation is different though so instead of mapping the Key and Value we only map the Keys.
				// Their API docs say, "If you don't specify a value for a key, the response returns all resources that are tagged with that key, with any or no value."
				// which makes this a safe way to reduce the amount of data we need to filter out.
				// https://docs.aws.amazon.com/resourcegroupstagging/latest/APIReference/API_GetResources.html#resourcegrouptagging-GetResources-request-TagFilters
				tagFilters = append(tagFilters, types.TagFilter{Key: &st.Key})
			}
		}
		inputparams := &resourcegroupstaggingapi.GetResourcesInput{
			ResourceTypeFilters: filters,
			ResourcesPerPage:    aws.Int32(int32(100)), // max allowed value according to API docs
			TagFilters:          tagFilters,
		}

		paginator := resourcegroupstaggingapi.NewGetResourcesPaginator(c.taggingAPI, inputparams, func(options *resourcegroupstaggingapi.GetResourcesPaginatorOptions) {
			options.StopOnDuplicateToken = true
		})
		for paginator.HasMorePages() {
			c.scrapeMetrics.ResourceGroupTaggingAPICounter.Inc()
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, err
			}

			for _, resourceTagMapping := range page.ResourceTagMappingList {
				resource := model.TaggedResource{
					ARN:       *resourceTagMapping.ResourceARN,
					Namespace: job.Namespace,
					Region:    region,
					Tags:      make([]model.Tag, 0, len(resourceTagMapping.Tags)),
				}

				for _, t := range resourceTagMapping.Tags {
					resource.Tags = append(resource.Tags, model.Tag{Key: *t.Key, Value: *t.Value})
				}

				if resource.FilterThroughTags(job.SearchTags) {
					resources = append(resources, &resource)
				} else {
					c.logger.Debug("Skipping resource because search tags do not match", "arn", resource.ARN)
				}
			}
		}

		c.logger.Debug("GetResourcesPages finished", "total", len(resources))
	}

	if ext, ok := ServiceFilters[svc.Namespace]; ok {
		if ext.ResourceFunc != nil {
			shouldHaveDiscoveredResources = true
			newResources, err := ext.ResourceFunc(ctx, c, job, region)
			if err != nil {
				return nil, fmt.Errorf("failed to apply ResourceFunc for %s, %w", svc.Namespace, err)
			}
			resources = append(resources, newResources...)
			c.logger.Debug("ResourceFunc finished", "total", len(resources))
		}

		if ext.FilterFunc != nil {
			filteredResources, err := ext.FilterFunc(ctx, c, resources)
			if err != nil {
				return nil, fmt.Errorf("failed to apply FilterFunc for %s, %w", svc.Namespace, err)
			}
			resources = filteredResources
			c.logger.Debug("FilterFunc finished", "total", len(resources))
		}
	}

	if shouldHaveDiscoveredResources && len(resources) == 0 {
		return nil, ErrExpectedToFindResources
	}

	return resources, nil
}
