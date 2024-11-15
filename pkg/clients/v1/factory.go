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
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigateway"
	"github.com/aws/aws-sdk-go/service/apigateway/apigatewayiface"
	"github.com/aws/aws-sdk-go/service/apigatewayv2"
	"github.com/aws/aws-sdk-go/service/apigatewayv2/apigatewayv2iface"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go/service/databasemigrationservice/databasemigrationserviceiface"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/prometheusservice"
	"github.com/aws/aws-sdk-go/service/prometheusservice/prometheusserviceiface"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go/service/shield"
	"github.com/aws/aws-sdk-go/service/shield/shieldiface"
	"github.com/aws/aws-sdk-go/service/storagegateway"
	"github.com/aws/aws-sdk-go/service/storagegateway/storagegatewayiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	account_v1 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account/v1"
	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	cloudwatch_v1 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch/v1"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	tagging_v1 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging/v1"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type CachingFactory struct {
	stsRegion        string
	session          *session.Session
	endpointResolver endpoints.ResolverFunc
	stscache         map[model.Role]stsiface.STSAPI
	iamcache         map[model.Role]iamiface.IAMAPI
	clients          map[model.Role]map[string]*cachedClients
	cleared          bool
	refreshed        bool
	mu               sync.Mutex
	fips             bool
	logger           logging.Logger
}

type cachedClients struct {
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

// NewFactory creates a new client factory to use when fetching data from AWS with sdk v1
func NewFactory(logger logging.Logger, jobsCfg model.JobsConfig, fips bool) *CachingFactory {
	stscache := map[model.Role]stsiface.STSAPI{}
	iamcache := map[model.Role]iamiface.IAMAPI{}
	cache := map[model.Role]map[string]*cachedClients{}

	for _, discoveryJob := range jobsCfg.DiscoveryJobs {
		for _, role := range discoveryJob.Roles {
			if _, ok := stscache[role]; !ok {
				stscache[role] = nil
			}
			if _, ok := iamcache[role]; !ok {
				iamcache[role] = nil
			}
			if _, ok := cache[role]; !ok {
				cache[role] = map[string]*cachedClients{}
			}
			for _, region := range discoveryJob.Regions {
				cache[role][region] = &cachedClients{}
			}
		}
	}

	for _, staticJob := range jobsCfg.StaticJobs {
		for _, role := range staticJob.Roles {
			if _, ok := stscache[role]; !ok {
				stscache[role] = nil
			}
			if _, ok := iamcache[role]; !ok {
				iamcache[role] = nil
			}

			if _, ok := cache[role]; !ok {
				cache[role] = map[string]*cachedClients{}
			}

			for _, region := range staticJob.Regions {
				// Only write a new region in if the region does not exist
				if _, ok := cache[role][region]; !ok {
					cache[role][region] = &cachedClients{
						onlyStatic: true,
					}
				}
			}
		}
	}

	for _, customNamespaceJob := range jobsCfg.CustomNamespaceJobs {
		for _, role := range customNamespaceJob.Roles {
			if _, ok := stscache[role]; !ok {
				stscache[role] = nil
			}
			if _, ok := iamcache[role]; !ok {
				iamcache[role] = nil
			}

			if _, ok := cache[role]; !ok {
				cache[role] = map[string]*cachedClients{}
			}

			for _, region := range customNamespaceJob.Regions {
				// Only write a new region in if the region does not exist
				if _, ok := cache[role][region]; !ok {
					cache[role][region] = &cachedClients{
						onlyStatic: true,
					}
				}
			}
		}
	}

	endpointResolver := endpoints.DefaultResolver().EndpointFor

	endpointURLOverride := os.Getenv("AWS_ENDPOINT_URL")
	if endpointURLOverride != "" {
		// allow override of all endpoints for local testing
		endpointResolver = func(_ string, _ string, _ ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
			return endpoints.ResolvedEndpoint{
				URL: endpointURLOverride,
			}, nil
		}
	}

	return &CachingFactory{
		stsRegion:        jobsCfg.StsRegion,
		session:          nil,
		endpointResolver: endpointResolver,
		stscache:         stscache,
		iamcache:         iamcache,
		clients:          cache,
		fips:             fips,
		cleared:          false,
		refreshed:        false,
		logger:           logger,
	}
}

// Refresh and Clear help to avoid using lock primitives by asserting that
// there are no ongoing writes to the map.
func (c *CachingFactory) Clear() {
	if c.cleared {
		return
	}

	for role := range c.stscache {
		c.stscache[role] = nil
	}

	for role := range c.iamcache {
		c.iamcache[role] = nil
	}

	for role, regions := range c.clients {
		for region := range regions {
			cachedClient := c.clients[role][region]
			cachedClient.account = nil
			cachedClient.cloudwatch = nil
			cachedClient.tagging = nil
		}
	}
	c.cleared = true
	c.refreshed = false
}

func (c *CachingFactory) Refresh() {
	if c.refreshed {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double check Refresh wasn't called concurrently
	if c.refreshed {
		return
	}

	// sessions really only need to be constructed once at runtime
	if c.session == nil {
		c.session = createAWSSession(c.endpointResolver, c.logger.IsDebugEnabled())
	}

	for role := range c.stscache {
		c.stscache[role] = createStsSession(c.session, role, c.stsRegion, c.fips, c.logger.IsDebugEnabled())
	}

	for role := range c.iamcache {
		c.iamcache[role] = createIamSession(c.session, role, c.fips, c.logger.IsDebugEnabled())
	}

	for role, regions := range c.clients {
		for region := range regions {
			cachedClient := c.clients[role][region]
			// if the role is just used in static jobs, then we
			// can skip creating other sessions and potentially running
			// into permissions errors or taking up needless cycles
			cachedClient.cloudwatch = createCloudWatchClient(c.logger, c.session, &region, role, c.fips)
			if cachedClient.onlyStatic {
				continue
			}
			cachedClient.tagging = createTaggingClient(c.logger, c.session, &region, role, c.fips)
			cachedClient.account = createAccountClient(c.logger, c.stscache[role], c.iamcache[role])
		}
	}

	c.cleared = false
	c.refreshed = true
}

func createCloudWatchClient(logger logging.Logger, s *session.Session, region *string, role model.Role, fips bool) cloudwatch_client.Client {
	return cloudwatch_v1.NewClient(
		logger,
		createCloudwatchSession(s, region, role, fips, logger.IsDebugEnabled()),
	)
}

func createTaggingClient(logger logging.Logger, session *session.Session, region *string, role model.Role, fips bool) tagging.Client {
	// The createSession function for a service which does not support FIPS does not take a fips parameter
	// This currently applies to createTagSession(Resource Groups Tagging), ASG (EC2 autoscaling), and Prometheus (Amazon Managed Prometheus)
	// AWS FIPS Reference: https://aws.amazon.com/compliance/fips/
	return tagging_v1.NewClient(
		logger,
		createTagSession(session, region, role, logger.IsDebugEnabled()),
		createASGSession(session, region, role, logger.IsDebugEnabled()),
		createAPIGatewaySession(session, region, role, fips, logger.IsDebugEnabled()),
		createAPIGatewayV2Session(session, region, role, fips, logger.IsDebugEnabled()),
		createEC2Session(session, region, role, fips, logger.IsDebugEnabled()),
		createDMSSession(session, region, role, fips, logger.IsDebugEnabled()),
		createPrometheusSession(session, region, role, logger.IsDebugEnabled()),
		createStorageGatewaySession(session, region, role, fips, logger.IsDebugEnabled()),
		createShieldSession(session, region, role, fips, logger.IsDebugEnabled()),
	)
}

func createAccountClient(logger logging.Logger, sts stsiface.STSAPI, iam iamiface.IAMAPI) account.Client {
	return account_v1.NewClient(logger, sts, iam)
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
	c.clients[role][region].cloudwatch = createCloudWatchClient(c.logger, c.session, &region, role, c.fips)
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
	c.clients[role][region].tagging = createTaggingClient(c.logger, c.session, &region, role, c.fips)
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
	c.clients[role][region].account = createAccountClient(c.logger, c.stscache[role], c.iamcache[role])
	return c.clients[role][region].account
}

func setExternalID(ID string) func(p *stscreds.AssumeRoleProvider) {
	return func(p *stscreds.AssumeRoleProvider) {
		if ID != "" {
			p.ExternalID = aws.String(ID)
		}
	}
}

func setSTSCreds(sess *session.Session, config *aws.Config, role model.Role) *aws.Config {
	if role.RoleArn != "" {
		config.Credentials = stscreds.NewCredentials(
			sess, role.RoleArn, setExternalID(role.ExternalID))
	}
	return config
}

func getAwsRetryer() aws.RequestRetryer {
	return client.DefaultRetryer{
		NumMaxRetries: 5,
		// MaxThrottleDelay and MinThrottleDelay used for throttle errors
		MaxThrottleDelay: 10 * time.Second,
		MinThrottleDelay: 1 * time.Second,
		// For other errors
		MaxRetryDelay: 3 * time.Second,
		MinRetryDelay: 1 * time.Second,
	}
}

func createAWSSession(resolver endpoints.ResolverFunc, isDebugEnabled bool) *session.Session {
	config := aws.Config{
		CredentialsChainVerboseErrors: aws.Bool(true),
		EndpointResolver:              resolver,
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config:            config,
	}))
	return sess
}

func createStsSession(sess *session.Session, role model.Role, region string, fips bool, isDebugEnabled bool) *sts.STS {
	maxStsRetries := 5
	config := &aws.Config{MaxRetries: &maxStsRetries}

	if region != "" {
		config = config.WithRegion(region).WithSTSRegionalEndpoint(endpoints.RegionalSTSEndpoint)
	}

	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return sts.New(sess, setSTSCreds(sess, config, role))
}

func createIamSession(sess *session.Session, role model.Role, fips bool, isDebugEnabled bool) *iam.IAM {
	maxStsRetries := 5
	config := &aws.Config{MaxRetries: &maxStsRetries}

	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return iam.New(sess, setSTSCreds(sess, config, role))
}

func createCloudwatchSession(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) *cloudwatch.CloudWatch {
	config := &aws.Config{Region: region, Retryer: getAwsRetryer()}

	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return cloudwatch.New(sess, setSTSCreds(sess, config, role))
}

func createTagSession(sess *session.Session, region *string, role model.Role, isDebugEnabled bool) *resourcegroupstaggingapi.ResourceGroupsTaggingAPI {
	maxResourceGroupTaggingRetries := 5
	config := &aws.Config{
		Region:                        region,
		MaxRetries:                    &maxResourceGroupTaggingRetries,
		CredentialsChainVerboseErrors: aws.Bool(true),
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return resourcegroupstaggingapi.New(sess, setSTSCreds(sess, config, role))
}

func createAPIGatewaySession(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) apigatewayiface.APIGatewayAPI {
	maxAPIGatewayAPIRetries := 5
	config := &aws.Config{Region: region, MaxRetries: &maxAPIGatewayAPIRetries}
	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return apigateway.New(sess, setSTSCreds(sess, config, role))
}

func createAPIGatewayV2Session(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) apigatewayv2iface.ApiGatewayV2API {
	maxAPIGatewayAPIRetries := 5
	config := &aws.Config{Region: region, MaxRetries: &maxAPIGatewayAPIRetries}
	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return apigatewayv2.New(sess, setSTSCreds(sess, config, role))
}

func createASGSession(sess *session.Session, region *string, role model.Role, isDebugEnabled bool) autoscalingiface.AutoScalingAPI {
	maxAutoScalingAPIRetries := 5
	config := &aws.Config{Region: region, MaxRetries: &maxAutoScalingAPIRetries}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return autoscaling.New(sess, setSTSCreds(sess, config, role))
}

func createStorageGatewaySession(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) storagegatewayiface.StorageGatewayAPI {
	maxStorageGatewayAPIRetries := 5
	config := &aws.Config{Region: region, MaxRetries: &maxStorageGatewayAPIRetries}

	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return storagegateway.New(sess, setSTSCreds(sess, config, role))
}

func createEC2Session(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) ec2iface.EC2API {
	maxEC2APIRetries := 10
	config := &aws.Config{Region: region, MaxRetries: &maxEC2APIRetries}
	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return ec2.New(sess, setSTSCreds(sess, config, role))
}

func createPrometheusSession(sess *session.Session, region *string, role model.Role, isDebugEnabled bool) prometheusserviceiface.PrometheusServiceAPI {
	maxPrometheusAPIRetries := 10
	config := &aws.Config{Region: region, MaxRetries: &maxPrometheusAPIRetries}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return prometheusservice.New(sess, setSTSCreds(sess, config, role))
}

func createDMSSession(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) databasemigrationserviceiface.DatabaseMigrationServiceAPI {
	maxDMSAPIRetries := 5
	config := &aws.Config{Region: region, MaxRetries: &maxDMSAPIRetries}
	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return databasemigrationservice.New(sess, setSTSCreds(sess, config, role))
}

func createShieldSession(sess *session.Session, region *string, role model.Role, fips bool, isDebugEnabled bool) shieldiface.ShieldAPI {
	maxShieldAPIRetries := 5
	config := &aws.Config{Region: region, MaxRetries: &maxShieldAPIRetries}
	if fips {
		config.UseFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
	}

	if isDebugEnabled {
		config.LogLevel = aws.LogLevel(aws.LogDebugWithHTTPBody)
	}

	return shield.New(sess, setSTSCreds(sess, config, role))
}
