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
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/amp"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/databasemigrationservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var jobsCfgWithDefaultRoleAndRegion1 = model.JobsConfig{
	DiscoveryJobs: []model.DiscoveryJob{
		{
			Roles:   []model.Role{{}},
			Regions: []string{"region1"},
		},
	},
}

func TestNewFactory_initializes_clients(t *testing.T) {
	role1 := model.Role{
		RoleArn:    "role1",
		ExternalID: "external1",
	}
	role2 := model.Role{
		RoleArn:    "role2",
		ExternalID: "external2",
	}
	role3 := model.Role{
		RoleArn:    "role3",
		ExternalID: "external3",
	}

	region1 := "region1"
	region2 := "region2"
	region3 := "region3"
	tests := []struct {
		name       string
		jobsCfg    model.JobsConfig
		onlyStatic *bool
	}{
		{
			name: "from discovery config",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{{
					Regions: []string{region1, region2, region3},
					Roles:   []model.Role{defaultRole, role1, role2, role3},
				}},
			},
			onlyStatic: aws.Bool(false),
		},
		{
			name: "from static config",
			jobsCfg: model.JobsConfig{
				StaticJobs: []model.StaticJob{{
					Regions: []string{region1, region2, region3},
					Roles:   []model.Role{defaultRole, role1, role2, role3},
				}},
			},
			onlyStatic: aws.Bool(true),
		},
		{
			name: "from custom config",
			jobsCfg: model.JobsConfig{
				CustomNamespaceJobs: []model.CustomNamespaceJob{{
					Regions: []string{region1, region2, region3},
					Roles:   []model.Role{defaultRole, role1, role2, role3},
				}},
			},
			onlyStatic: aws.Bool(true),
		},
		{
			name: "from all configs",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{{
					Regions: []string{region1, region2},
					Roles:   []model.Role{defaultRole, role1, role2},
				}},
				StaticJobs: []model.StaticJob{{
					Regions: []string{region2, region3},
					Roles:   []model.Role{defaultRole, role2, role3},
				}},
				CustomNamespaceJobs: []model.CustomNamespaceJob{{
					Regions: []string{region1, region3},
					Roles:   []model.Role{defaultRole, role1, role3},
				}},
			},
			onlyStatic: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := NewFactory(logging.NewNopLogger(), test.jobsCfg, false)
			require.NoError(t, err)

			assert.False(t, output.refreshed)
			assert.False(t, output.cleared)

			require.Len(t, output.clients, 4)
			assert.Contains(t, output.clients, defaultRole)
			assert.Contains(t, output.clients, role1)
			assert.Contains(t, output.clients, role2)
			assert.Contains(t, output.clients, role3)

			for role, regionalClients := range output.clients {
				require.Len(t, regionalClients, 3)

				assert.Contains(t, regionalClients, region1)
				assert.Contains(t, regionalClients, region2)
				assert.Contains(t, regionalClients, region3)

				for region, clients := range regionalClients {
					assert.NotNil(t, clients, "role %s region %s had nil clients", role, region)
					if test.onlyStatic != nil {
						assert.Equal(t, *test.onlyStatic, clients.onlyStatic, "role %s region %s had unexpected onlyStatic value", role, region)
					}

					assert.Equal(t, region, clients.awsConfig.Region)
				}
			}
		})
	}
}

func TestNewFactory_respects_stsregion(t *testing.T) {
	stsRegion := "custom-sts-region"
	cfg := model.JobsConfig{
		StsRegion: stsRegion,
		DiscoveryJobs: []model.DiscoveryJob{{
			Regions: []string{"region1"},
			Roles:   []model.Role{defaultRole},
		}},
	}

	output, err := NewFactory(logging.NewNopLogger(), cfg, false)
	require.NoError(t, err)
	require.Len(t, output.clients, 1)
	stsOptions := sts.Options{}
	output.stsOptions(&stsOptions)
	assert.Equal(t, stsRegion, stsOptions.Region)
}

func TestCachingFactory_Clear(t *testing.T) {
	cache := &CachingFactory{
		logger: logging.NewNopLogger(),
		clients: map[model.Role]map[awsRegion]*cachedClients{
			defaultRole: {
				"region1": &cachedClients{
					awsConfig:  nil,
					cloudwatch: testClient{},
					tagging:    testClient{},
					account:    testClient{},
				},
			},
		},
		refreshed: true,
		cleared:   false,
	}

	cache.Clear()
	assert.True(t, cache.cleared)
	assert.False(t, cache.refreshed)

	clients := cache.clients[defaultRole]["region1"]
	require.NotNil(t, clients)
	assert.Nil(t, clients.cloudwatch)
	assert.Nil(t, clients.account)
	assert.Nil(t, clients.tagging)
}

func TestCachingFactory_Refresh(t *testing.T) {
	t.Run("creates all clients when config contains only discovery jobs", func(t *testing.T) {
		output, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, false)
		require.NoError(t, err)

		output.Refresh()
		assert.False(t, output.cleared)
		assert.True(t, output.refreshed)

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		assert.NotNil(t, clients.cloudwatch)
		assert.NotNil(t, clients.account)
		assert.NotNil(t, clients.tagging)
	})

	t.Run("creates only cloudwatch when config is only static jobs", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			StaticJobs: []model.StaticJob{{
				Regions: []string{"region1"},
				Roles:   []model.Role{{}},
			}},
			CustomNamespaceJobs: []model.CustomNamespaceJob{{
				Regions: []string{"region1"},
				Roles:   []model.Role{{}},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		output.Refresh()
		assert.False(t, output.cleared)
		assert.True(t, output.refreshed)

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		assert.NotNil(t, clients.cloudwatch)
		assert.Nil(t, clients.account)
		assert.Nil(t, clients.tagging)
	})
}

func TestCachingFactory_GetAccountClient(t *testing.T) {
	t.Run("refreshed cache does not create new client", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			DiscoveryJobs: []model.DiscoveryJob{{
				Roles:   []model.Role{{}},
				Regions: []string{"region1"},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		output.Refresh()

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		assert.Equal(t, clients.account, output.GetAccountClient("region1", defaultRole))
	})

	t.Run("unrefreshed cache creates a new client", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			DiscoveryJobs: []model.DiscoveryJob{{
				Roles:   []model.Role{{}},
				Regions: []string{"region1"},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		require.Nil(t, clients.account)

		client := output.GetAccountClient("region1", defaultRole)
		assert.Equal(t, clients.account, client)
	})
}

func TestCachingFactory_GetCloudwatchClient(t *testing.T) {
	t.Run("refreshed cache does not create new client", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			DiscoveryJobs: []model.DiscoveryJob{{
				Roles:   []model.Role{{}},
				Regions: []string{"region1"},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		output.Refresh()

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		// Can't do equality comparison due to concurrency limiter
		assert.NotNil(t, output.GetCloudwatchClient("region1", defaultRole, cloudwatch_client.ConcurrencyConfig{SingleLimit: 1}))
	})

	t.Run("unrefreshed cache creates a new client", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			DiscoveryJobs: []model.DiscoveryJob{{
				Roles:   []model.Role{{}},
				Regions: []string{"region1"},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		require.Nil(t, clients.cloudwatch)

		output.GetCloudwatchClient("region1", defaultRole, cloudwatch_client.ConcurrencyConfig{SingleLimit: 1})
		assert.NotNil(t, clients.cloudwatch)
	})
}

func TestCachingFactory_GetTaggingClient(t *testing.T) {
	t.Run("refreshed cache does not create new client", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			DiscoveryJobs: []model.DiscoveryJob{{
				Roles:   []model.Role{{}},
				Regions: []string{"region1"},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		output.Refresh()

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		// Can't do equality comparison due to concurrency limiter
		assert.NotNil(t, output.GetTaggingClient("region1", defaultRole, 1))
	})

	t.Run("unrefreshed cache creates a new client", func(t *testing.T) {
		jobsCfg := model.JobsConfig{
			DiscoveryJobs: []model.DiscoveryJob{{
				Roles:   []model.Role{{}},
				Regions: []string{"region1"},
			}},
		}

		output, err := NewFactory(logging.NewNopLogger(), jobsCfg, false)
		require.NoError(t, err)

		clients := output.clients[defaultRole]["region1"]
		require.NotNil(t, clients)
		require.Nil(t, clients.tagging)

		output.GetTaggingClient("region1", defaultRole, 1)
		assert.NotNil(t, clients.tagging)
	})
}

func TestCachingFactory_createTaggingClient_DoesNotEnableFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createTaggingClient(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[resourcegroupstaggingapi.Client, resourcegroupstaggingapi.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateUnset)
}

func TestCachingFactory_createAPIGatewayClient_EnablesFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createAPIGatewayClient(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[apigateway.Client, apigateway.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateEnabled)
}

func TestCachingFactory_createAPIGatewayV2Client_EnablesFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createAPIGatewayV2Client(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[apigatewayv2.Client, apigatewayv2.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateEnabled)
}

func TestCachingFactory_createAutoScalingClient_DoesNotEnableFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createAutoScalingClient(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[autoscaling.Client, autoscaling.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateUnset)
}

func TestCachingFactory_createEC2Client_EnablesFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createEC2Client(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[ec2.Client, ec2.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateEnabled)
}

func TestCachingFactory_createDMSClient_EnablesFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createDMSClient(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[databasemigrationservice.Client, databasemigrationservice.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateEnabled)
}

func TestCachingFactory_createStorageGatewayClient_EnablesFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createStorageGatewayClient(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[storagegateway.Client, storagegateway.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateEnabled)
}

func TestCachingFactory_createPrometheusClient_DoesNotEnableFIPS(t *testing.T) {
	factory, err := NewFactory(logging.NewNopLogger(), jobsCfgWithDefaultRoleAndRegion1, true)
	require.NoError(t, err)

	client := factory.createPrometheusClient(factory.clients[defaultRole]["region1"].awsConfig)
	require.NotNil(t, client)

	options := getOptions[amp.Client, amp.Options](client)
	require.NotNil(t, options)

	assert.Equal(t, options.EndpointOptions.UseFIPSEndpoint, aws.FIPSEndpointStateUnset)
}

// getOptions uses reflection to pull the unexported options field off of any AWS Client
// the options of the client carries around a lot of info about how the client will behave and is helpful for
// testing lower level sdk configuration
func getOptions[T any, V any](awsClient *T) V {
	field := reflect.ValueOf(awsClient).Elem().FieldByName("options")
	options := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Interface().(V)
	return options
}

type testClient struct{}

func (t testClient) GetResources(_ context.Context, _ model.DiscoveryJob, _ string) ([]*model.TaggedResource, error) {
	return nil, nil
}

func (t testClient) GetAccount(_ context.Context) (string, error) {
	return "", nil
}

func (t testClient) GetAccountAlias(_ context.Context) (string, error) {
	return "", nil
}

func (t testClient) ListMetrics(_ context.Context, _ string, _ *model.MetricConfig, _ bool, _ func(page []*model.Metric)) error {
	return nil
}

func (t testClient) GetMetricData(_ context.Context, _ []*model.CloudwatchData, _ string, _ time.Time, _ time.Time) []cloudwatch_client.MetricDataResult {
	return nil
}

func (t testClient) GetMetricStatistics(_ context.Context, _ logging.Logger, _ []model.Dimension, _ string, _ *model.MetricConfig) []*model.Datapoint {
	return nil
}
