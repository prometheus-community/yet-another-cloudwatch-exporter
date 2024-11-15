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
package v2

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/shield"
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	aws_logging "github.com/aws/smithy-go/logging"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	account_v2 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account/v2"
	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	cloudwatch_v2 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch/v2"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	tagging_v2 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging/v2"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type awsRegion = string

type CachingFactory struct {
	logger              logging.Logger
	stsOptions          func(*sts.Options)
	clients             map[model.Role]map[awsRegion]*cachedClients
	mu                  sync.Mutex
	refreshed           bool
	cleared             bool
	fipsEnabled         bool
	endpointURLOverride string
}

type cachedClients struct {
	awsConfig *aws.Config
	// if we know that this job is only used for static
	// then we don't have to construct as many cached connections
	// later on
	onlyStatic bool
	cloudwatch cloudwatch_client.Client
	tagging    tagging.Client
	account    account.Client
}

// Ensure the struct properly implements the interface
var _ clients.Factory = &CachingFactory{}

// NewFactory creates a new client factory to use when fetching data from AWS with sdk v2
func NewFactory(logger logging.Logger, jobsCfg model.JobsConfig, fips bool) (*CachingFactory, error) {
	var options []func(*aws_config.LoadOptions) error
	options = append(options, aws_config.WithLogger(aws_logging.LoggerFunc(func(classification aws_logging.Classification, format string, v ...interface{}) {
		if classification == aws_logging.Debug {
			if logger.IsDebugEnabled() {
				logger.Debug(fmt.Sprintf(format, v...))
			}
		} else if classification == aws_logging.Warn {
			logger.Warn(fmt.Sprintf(format, v...))
		} else { // AWS logging only supports debug or warn, log everything else as error
			logger.Error(fmt.Errorf("unexected aws error classification: %s", classification), fmt.Sprintf(format, v...))
		}
	})))

	options = append(options, aws_config.WithLogConfigurationWarnings(true))

	endpointURLOverride := os.Getenv("AWS_ENDPOINT_URL")

	options = append(options, aws_config.WithRetryMaxAttempts(5))

	c, err := aws_config.LoadDefaultConfig(context.TODO(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to load default aws config: %w", err)
	}

	stsOptions := createStsOptions(jobsCfg.StsRegion, logger.IsDebugEnabled(), endpointURLOverride, fips)
	cache := map[model.Role]map[awsRegion]*cachedClients{}
	for _, discoveryJob := range jobsCfg.DiscoveryJobs {
		for _, role := range discoveryJob.Roles {
			if _, ok := cache[role]; !ok {
				cache[role] = map[awsRegion]*cachedClients{}
			}
			for _, region := range discoveryJob.Regions {
				regionConfig := awsConfigForRegion(role, &c, region, stsOptions)
				cache[role][region] = &cachedClients{
					awsConfig:  regionConfig,
					onlyStatic: false,
				}
			}
		}
	}

	for _, staticJob := range jobsCfg.StaticJobs {
		for _, role := range staticJob.Roles {
			if _, ok := cache[role]; !ok {
				cache[role] = map[awsRegion]*cachedClients{}
			}
			for _, region := range staticJob.Regions {
				// Discovery job client definitions have precedence
				if _, exists := cache[role][region]; !exists {
					regionConfig := awsConfigForRegion(role, &c, region, stsOptions)
					cache[role][region] = &cachedClients{
						awsConfig:  regionConfig,
						onlyStatic: true,
					}
				}
			}
		}
	}

	for _, customNamespaceJob := range jobsCfg.CustomNamespaceJobs {
		for _, role := range customNamespaceJob.Roles {
			if _, ok := cache[role]; !ok {
				cache[role] = map[awsRegion]*cachedClients{}
			}
			for _, region := range customNamespaceJob.Regions {
				// Discovery job client definitions have precedence
				if _, exists := cache[role][region]; !exists {
					regionConfig := awsConfigForRegion(role, &c, region, stsOptions)
					cache[role][region] = &cachedClients{
						awsConfig:  regionConfig,
						onlyStatic: true,
					}
				}
			}
		}
	}

	return &CachingFactory{
		logger:              logger,
		clients:             cache,
		fipsEnabled:         fips,
		stsOptions:          stsOptions,
		endpointURLOverride: endpointURLOverride,
	}, nil
}

func (c *CachingFactory) GetCloudwatchClient(region string, role model.Role, concurrency cloudwatch_client.ConcurrencyConfig) cloudwatch_client.Client {
	if !c.refreshed {
		// if we have not refreshed then we need to lock in case we are accessing concurrently
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if client := c.clients[role][region].cloudwatch; client != nil {
		return cloudwatch_client.NewLimitedConcurrencyClient(client, concurrency.NewLimiter())
	}
	c.clients[role][region].cloudwatch = cloudwatch_v2.NewClient(c.logger, c.createCloudwatchClient(c.clients[role][region].awsConfig))
	return cloudwatch_client.NewLimitedConcurrencyClient(c.clients[role][region].cloudwatch, concurrency.NewLimiter())
}

func (c *CachingFactory) GetTaggingClient(region string, role model.Role, concurrencyLimit int) tagging.Client {
	if !c.refreshed {
		// if we have not refreshed then we need to lock in case we are accessing concurrently
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if client := c.clients[role][region].tagging; client != nil {
		return tagging.NewLimitedConcurrencyClient(client, concurrencyLimit)
	}
	c.clients[role][region].tagging = tagging_v2.NewClient(
		c.logger,
		c.createTaggingClient(c.clients[role][region].awsConfig),
		c.createAutoScalingClient(c.clients[role][region].awsConfig),
		c.createAPIGatewayClient(c.clients[role][region].awsConfig),
		c.createAPIGatewayV2Client(c.clients[role][region].awsConfig),
		c.createEC2Client(c.clients[role][region].awsConfig),
		c.createDMSClient(c.clients[role][region].awsConfig),
		c.createPrometheusClient(c.clients[role][region].awsConfig),
		c.createStorageGatewayClient(c.clients[role][region].awsConfig),
		c.createShieldClient(c.clients[role][region].awsConfig),
	)
	return tagging.NewLimitedConcurrencyClient(c.clients[role][region].tagging, concurrencyLimit)
}

func (c *CachingFactory) GetAccountClient(region string, role model.Role) account.Client {
	if !c.refreshed {
		// if we have not refreshed then we need to lock in case we are accessing concurrently
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if client := c.clients[role][region].account; client != nil {
		return client
	}

	stsClient := c.createStsClient(c.clients[role][region].awsConfig)
	iamClient := c.createIAMClient(c.clients[role][region].awsConfig)
	c.clients[role][region].account = account_v2.NewClient(c.logger, stsClient, iamClient)
	return c.clients[role][region].account
}

func (c *CachingFactory) Refresh() {
	if c.refreshed {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Avoid double refresh in the event Refresh() is called concurrently
	if c.refreshed {
		return
	}

	for _, regionClients := range c.clients {
		for _, cache := range regionClients {
			cache.cloudwatch = cloudwatch_v2.NewClient(c.logger, c.createCloudwatchClient(cache.awsConfig))
			if cache.onlyStatic {
				continue
			}

			cache.tagging = tagging_v2.NewClient(
				c.logger,
				c.createTaggingClient(cache.awsConfig),
				c.createAutoScalingClient(cache.awsConfig),
				c.createAPIGatewayClient(cache.awsConfig),
				c.createAPIGatewayV2Client(cache.awsConfig),
				c.createEC2Client(cache.awsConfig),
				c.createDMSClient(cache.awsConfig),
				c.createPrometheusClient(cache.awsConfig),
				c.createStorageGatewayClient(cache.awsConfig),
				c.createShieldClient(cache.awsConfig),
			)

			cache.account = account_v2.NewClient(c.logger, c.createStsClient(cache.awsConfig), c.createIAMClient(cache.awsConfig))
		}
	}

	c.refreshed = true
	c.cleared = false
}

func (c *CachingFactory) Clear() {
	if c.cleared {
		return
	}
	// Prevent concurrent reads/write if clear is called during execution
	c.mu.Lock()
	defer c.mu.Unlock()
	// Avoid double clear in the event Refresh() is called concurrently
	if c.cleared {
		return
	}

	for _, regions := range c.clients {
		for _, cache := range regions {
			cache.cloudwatch = nil
			cache.account = nil
			cache.tagging = nil
		}
	}

	c.refreshed = false
	c.cleared = true
}

func (c *CachingFactory) createCloudwatchClient(regionConfig *aws.Config) *cloudwatch.Client {
	return cloudwatch.NewFromConfig(*regionConfig, func(options *cloudwatch.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}

		// Setting an explicit retryer will override the default settings on the config
		options.Retryer = retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = 5
			options.MaxBackoff = 3 * time.Second
		})

		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func (c *CachingFactory) createTaggingClient(regionConfig *aws.Config) *resourcegroupstaggingapi.Client {
	return resourcegroupstaggingapi.NewFromConfig(*regionConfig, func(options *resourcegroupstaggingapi.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		// The FIPS setting is ignored because FIPS is not available for resource groups tagging apis
		// If enabled the SDK will try to use non-existent FIPS URLs, https://github.com/aws/aws-sdk-go-v2/issues/2138#issuecomment-1570791988
		// AWS FIPS Reference: https://aws.amazon.com/compliance/fips/
	})
}

func (c *CachingFactory) createAutoScalingClient(assumedConfig *aws.Config) *autoscaling.Client {
	return autoscaling.NewFromConfig(*assumedConfig, func(options *autoscaling.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		// The FIPS setting is ignored because FIPS is not available for EC2 autoscaling apis
		// If enabled the SDK will try to use non-existent FIPS URLs, https://github.com/aws/aws-sdk-go-v2/issues/2138#issuecomment-1570791988
		// AWS FIPS Reference: https://aws.amazon.com/compliance/fips/
		// 	EC2 autoscaling has FIPS compliant URLs for govcloud, but they do not use any FIPS prefixing, and should work
		//	with sdk v2s EndpointResolverV2
	})
}

func (c *CachingFactory) createAPIGatewayClient(assumedConfig *aws.Config) *apigateway.Client {
	return apigateway.NewFromConfig(*assumedConfig, func(options *apigateway.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func (c *CachingFactory) createAPIGatewayV2Client(assumedConfig *aws.Config) *apigatewayv2.Client {
	return apigatewayv2.NewFromConfig(*assumedConfig, func(options *apigatewayv2.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func (c *CachingFactory) createEC2Client(assumedConfig *aws.Config) *ec2.Client {
	return ec2.NewFromConfig(*assumedConfig, func(options *ec2.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func (c *CachingFactory) createDMSClient(assumedConfig *aws.Config) *databasemigrationservice.Client {
	return databasemigrationservice.NewFromConfig(*assumedConfig, func(options *databasemigrationservice.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func (c *CachingFactory) createStorageGatewayClient(assumedConfig *aws.Config) *storagegateway.Client {
	return storagegateway.NewFromConfig(*assumedConfig, func(options *storagegateway.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func (c *CachingFactory) createPrometheusClient(assumedConfig *aws.Config) *amp.Client {
	return amp.NewFromConfig(*assumedConfig, func(options *amp.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		// The FIPS setting is ignored because FIPS is not available for amp apis
		// If enabled the SDK will try to use non-existent FIPS URLs, https://github.com/aws/aws-sdk-go-v2/issues/2138#issuecomment-1570791988
		// AWS FIPS Reference: https://aws.amazon.com/compliance/fips/
	})
}

func (c *CachingFactory) createStsClient(awsConfig *aws.Config) *sts.Client {
	return sts.NewFromConfig(*awsConfig, c.stsOptions)
}

func (c *CachingFactory) createIAMClient(awsConfig *aws.Config) *iam.Client {
	return iam.NewFromConfig(*awsConfig)
}

func (c *CachingFactory) createShieldClient(awsConfig *aws.Config) *shield.Client {
	return shield.NewFromConfig(*awsConfig, func(options *shield.Options) {
		if c.logger.IsDebugEnabled() {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if c.endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(c.endpointURLOverride)
		}
		if c.fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	})
}

func createStsOptions(stsRegion string, isDebugLoggingEnabled bool, endpointURLOverride string, fipsEnabled bool) func(*sts.Options) {
	return func(options *sts.Options) {
		if stsRegion != "" {
			options.Region = stsRegion
		}
		if isDebugLoggingEnabled {
			options.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody
		}
		if endpointURLOverride != "" {
			options.BaseEndpoint = aws.String(endpointURLOverride)
		}
		if fipsEnabled {
			options.EndpointOptions.UseFIPSEndpoint = aws.FIPSEndpointStateEnabled
		}
	}
}

var defaultRole = model.Role{}

func awsConfigForRegion(r model.Role, c *aws.Config, region awsRegion, stsOptions func(*sts.Options)) *aws.Config {
	regionalConfig := c.Copy()
	regionalConfig.Region = region

	if r == defaultRole {
		return &regionalConfig
	}

	// based on https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/credentials/stscreds#hdr-Assume_Role
	// found via https://github.com/aws/aws-sdk-go-v2/issues/1382
	regionalSts := sts.NewFromConfig(*c, stsOptions)
	credentials := stscreds.NewAssumeRoleProvider(regionalSts, r.RoleArn, func(options *stscreds.AssumeRoleOptions) {
		if r.ExternalID != "" {
			options.ExternalID = aws.String(r.ExternalID)
		}
	})
	regionalConfig.Credentials = aws.NewCredentialsCache(credentials)

	return &regionalConfig
}
