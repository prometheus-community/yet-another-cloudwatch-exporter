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
	"fmt"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func cmpCache(t *testing.T, initialCache *CachingFactory, cache *CachingFactory) {
	for role := range initialCache.stscache {
		if _, ok := cache.stscache[role]; !ok {
			t.Logf("`role` not in sts cache %s", role.RoleArn)
			t.Fail()
		}
	}

	for role, regionMap := range initialCache.clients {
		if _, ok := cache.clients[role]; !ok {
			t.Logf("`role` not in client cache %s", role.RoleArn)
			t.Fail()
			continue
		}

		for region, client := range regionMap {
			if _, ok := cache.clients[role][region]; !ok {
				t.Logf("`region` %s not found in role %s", region, role.RoleArn)
				t.Fail()
			}

			if client == nil {
				t.Logf("`client cache` is nil for region %s and role %v", region, role)
				continue
			}

			if cache.clients[role][region] == nil {
				t.Logf("comparison `client cache` is nil for region %s and role %v", region, role)
				continue
			}

			if *client != *cache.clients[role][region] {
				t.Logf("`client` %v is not equal to %v for role %v in region %s", *client, *cache.clients[role][region], role, region)
				t.Logf("The cache for this client is %v", cache.clients[role])
				t.Logf("The cache for the comparison client is %v", client)
				t.Fail()
			}
		}
	}
}

func TestNewClientCache(t *testing.T) {
	tests := []struct {
		descrip string
		jobsCfg model.JobsConfig
		fips    bool
		cache   *CachingFactory
	}{
		{
			"an empty config gives an empty cache",
			model.JobsConfig{},
			false,
			&CachingFactory{logger: logging.NewNopLogger()},
		},
		{
			"if fips is set then the clients has fips",
			model.JobsConfig{},
			true,
			&CachingFactory{
				fips:   true,
				logger: logging.NewNopLogger(),
			},
		},
		{
			"a ScrapeConf with only discovery jobs creates a cache",
			model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1", "us-west-2", "ap-northeast-3"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn:    "some-arn",
								ExternalID: "thing",
							},
						},
					},
					{
						Regions: []string{"ap-northeast-3"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn5",
							},
						},
					},
				},
			},
			false,
			&CachingFactory{
				stscache: map[model.Role]stsiface.STSAPI{
					{RoleArn: "some-arn"}:                      nil,
					{RoleArn: "some-arn", ExternalID: "thing"}: nil,
					{RoleArn: "some-arn2"}:                     nil,
					{RoleArn: "some-arn5"}:                     nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{RoleArn: "some-arn"}: {
						"ap-northeast-3": &cachedClients{},
						"us-east-1":      &cachedClients{},
						"us-west-2":      &cachedClients{},
					},
					{RoleArn: "some-arn", ExternalID: "thing"}: {
						"ap-northeast-3": &cachedClients{},
						"us-east-1":      &cachedClients{},
						"us-west-2":      &cachedClients{},
					},
					{RoleArn: "some-arn2"}: {
						"ap-northeast-3": &cachedClients{},
						"us-east-1":      &cachedClients{},
						"us-west-2":      &cachedClients{},
					},
					{RoleArn: "some-arn5"}: {
						"ap-northeast-3": &cachedClients{},
					},
				},
				logger: logging.NewNopLogger(),
			},
		},
		{
			"a ScrapeConf with only static jobs creates a cache",
			model.JobsConfig{
				StaticJobs: []model.StaticJob{
					{
						Name:    "scrape-thing",
						Regions: []string{"us-east-1", "eu-west-2"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn3",
							},
						},
					},
					{
						Name:    "scrape-other-thing",
						Regions: []string{"us-east-1"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn:    "some-arn",
								ExternalID: "thing",
							},
						},
					},
					{
						Name:    "scrape-third-thing",
						Regions: []string{"ap-northeast-1"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn4",
							},
						},
					},
				},
			},
			false,
			&CachingFactory{
				stscache: map[model.Role]stsiface.STSAPI{
					{RoleArn: "some-arn"}:                      nil,
					{RoleArn: "some-arn", ExternalID: "thing"}: nil,
					{RoleArn: "some-arn2"}:                     nil,
					{RoleArn: "some-arn3"}:                     nil,
					{RoleArn: "some-arn4"}:                     nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{RoleArn: "some-arn"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-east-1":      &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn", ExternalID: "thing"}: {
						"us-east-1": &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn2"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-east-1":      &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn3"}: {
						"eu-west-2": &cachedClients{onlyStatic: true},
						"us-east-1": &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn4"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
					},
				},
				logger: logging.NewNopLogger(),
			},
		},
		{
			"a ScrapeConf with some overlapping static and discovery jobs creates a cache",
			model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1", "us-west-2", "ap-northeast-3"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn3",
							},
						},
					},
					{
						Regions: []string{"ap-northeast-3"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn5",
							},
						},
					},
				},
				StaticJobs: []model.StaticJob{
					{
						Name:    "scrape-thing",
						Regions: []string{"us-east-1", "eu-west-2"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn3",
							},
						},
					},
					{
						Name:    "scrape-other-thing",
						Regions: []string{"us-east-1"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn:    "some-arn",
								ExternalID: "thing",
							},
						},
					},
					{
						Name:    "scrape-third-thing",
						Regions: []string{"ap-northeast-1"},
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn4",
							},
						},
					},
				},
			},
			false,
			&CachingFactory{
				stscache: map[model.Role]stsiface.STSAPI{
					{RoleArn: "some-arn"}:                      nil,
					{RoleArn: "some-arn", ExternalID: "thing"}: nil,
					{RoleArn: "some-arn2"}:                     nil,
					{RoleArn: "some-arn3"}:                     nil,
					{RoleArn: "some-arn4"}:                     nil,
					{RoleArn: "some-arn5"}:                     nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{RoleArn: "some-arn"}: {
						"ap-northeast-3": &cachedClients{},
						"us-east-1":      &cachedClients{},
						"ap-northeast-1": &cachedClients{onlyStatic: true},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-west-2":      &cachedClients{},
					},
					{RoleArn: "some-arn", ExternalID: "thing"}: {
						"us-east-1": &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn2"}: {
						"ap-northeast-3": &cachedClients{},
						"ap-northeast-1": &cachedClients{onlyStatic: true},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-east-1":      &cachedClients{},
						"us-west-2":      &cachedClients{},
					},
					{RoleArn: "some-arn3"}: {
						"ap-northeast-3": &cachedClients{},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-east-1":      &cachedClients{},
						"us-west-2":      &cachedClients{},
					},
					{RoleArn: "some-arn4"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn5"}: {
						"ap-northeast-3": &cachedClients{},
					},
				},
				logger: logging.NewNopLogger(),
			},
		},
		{
			"a ScrapeConf with only custom dimension jobs creates a cache",
			model.JobsConfig{
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{
						Name:      "scrape-thing",
						Regions:   []string{"us-east-1", "eu-west-2"},
						Namespace: "CustomDimension",
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn3",
							},
						},
					},
					{
						Name:      "scrape-other-thing",
						Regions:   []string{"us-east-1"},
						Namespace: "CustomDimension",
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn:    "some-arn",
								ExternalID: "thing",
							},
						},
					},
					{
						Name:      "scrape-third-thing",
						Regions:   []string{"ap-northeast-1"},
						Namespace: "CustomDimension",
						Roles: []model.Role{
							{
								RoleArn: "some-arn",
							},
							{
								RoleArn: "some-arn2",
							},
							{
								RoleArn: "some-arn4",
							},
						},
					},
				},
			},
			false,
			&CachingFactory{
				stscache: map[model.Role]stsiface.STSAPI{
					{RoleArn: "some-arn"}:                      nil,
					{RoleArn: "some-arn", ExternalID: "thing"}: nil,
					{RoleArn: "some-arn2"}:                     nil,
					{RoleArn: "some-arn3"}:                     nil,
					{RoleArn: "some-arn4"}:                     nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{RoleArn: "some-arn"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-east-1":      &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn", ExternalID: "thing"}: {
						"us-east-1": &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn2"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
						"eu-west-2":      &cachedClients{onlyStatic: true},
						"us-east-1":      &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn3"}: {
						"eu-west-2": &cachedClients{onlyStatic: true},
						"us-east-1": &cachedClients{onlyStatic: true},
					},
					{RoleArn: "some-arn4"}: {
						"ap-northeast-1": &cachedClients{onlyStatic: true},
					},
				},
				logger: logging.NewNopLogger(),
			},
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			t.Parallel()
			cache := NewFactory(logging.NewNopLogger(), test.jobsCfg, test.fips)
			t.Logf("the cache is: %v", cache)

			if test.cache.cleared != cache.cleared {
				t.Logf("`cleared` not equal got %v, expected %v", cache.cleared, test.cache.cleared)
				t.Fail()
			}

			if test.cache.refreshed != cache.refreshed {
				t.Logf("`refreshed` not equal got %v, expected %v", cache.refreshed, test.cache.refreshed)
				t.Fail()
			}

			if test.cache.fips != cache.fips {
				t.Logf("`fips` not equal got %v, expected %v", cache.fips, test.cache.fips)
				t.Fail()
			}

			// Strict equality requires each set containing each other
			cmpCache(t, test.cache, cache)
			cmpCache(t, cache, test.cache)
		})
	}
}

func TestClear(t *testing.T) {
	region := "us-east-1"
	role := model.Role{}

	tests := []struct {
		description string
		cache       *CachingFactory
	}{
		{
			"a new clear clears all clients",
			&CachingFactory{
				session: mock.Session,
				cleared: false,
				mu:      sync.Mutex{},
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: createCloudWatchClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							tagging:    createTaggingClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							account:    createAccountClient(logging.NewNopLogger(), nil, nil),
							onlyStatic: true,
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
		},
		{
			"A second call to clear does nothing",
			&CachingFactory{
				cleared: true,
				mu:      sync.Mutex{},
				session: mock.Session,
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: nil,
							tagging:    nil,
							account:    nil,
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.description, func(t *testing.T) {
			test.cache.Clear()
			if !test.cache.cleared {
				t.Log("Cache cleared flag not set")
				t.Fail()
			}
			if test.cache.refreshed {
				t.Log("Cache cleared flag set")
				t.Fail()
			}

			for role, client := range test.cache.stscache {
				if client != nil {
					t.Logf("STS `client` %v not cleared", role)
					t.Fail()
				}
			}

			for role, regionMap := range test.cache.clients {
				for region, client := range regionMap {
					if client.cloudwatch != nil {
						t.Logf("`cloudwatch client` %v in region %v is not nil", role, region)
						t.Fail()
					}
					if client.tagging != nil {
						t.Logf("`tagging client` %v in region %v is not nil", role, region)
						t.Fail()
					}
					if client.account != nil {
						t.Logf("`asg client` %v in region %v is not nil", role, region)
						t.Fail()
					}
				}
			}
		})
	}
}

func TestRefresh(t *testing.T) {
	region := "us-east-1"
	role := model.Role{}

	tests := []struct {
		descrip    string
		cache      *CachingFactory
		cloudwatch bool
	}{
		{
			"a new refresh creates clients",
			&CachingFactory{
				session:   mock.Session,
				refreshed: false,
				mu:        sync.Mutex{},
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: nil,
							tagging:    nil,
							account:    nil,
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
			false,
		},
		{
			"a new refresh with static only creates only cloudwatch",
			&CachingFactory{
				session:   mock.Session,
				refreshed: false,
				mu:        sync.Mutex{},
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: nil,
							tagging:    nil,
							account:    nil,
							onlyStatic: true,
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
			true,
		},
		{
			"A second call to refreshed does nothing",
			&CachingFactory{
				refreshed: true,
				mu:        sync.Mutex{},
				session:   mock.Session,
				stscache: map[model.Role]stsiface.STSAPI{
					{}: createStsSession(mock.Session, role, "", false, false),
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: createCloudWatchClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							tagging:    createTaggingClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							account:    createAccountClient(logging.NewNopLogger(), createStsSession(mock.Session, role, "", false, false), createIamSession(mock.Session, role, false, false)),
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
			false,
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			t.Parallel()
			test.cache.Refresh()

			if !test.cache.refreshed {
				t.Log("Cache refreshed flag not set")
				t.Fail()
			}

			if test.cache.cleared {
				t.Log("Cache cleared flag set")
				t.Fail()
			}

			for role, client := range test.cache.stscache {
				if client == nil {
					t.Logf("STS `client` %v not refreshed", role)
					t.Fail()
				}
			}

			for role, regionMap := range test.cache.clients {
				for region, client := range regionMap {
					if client.cloudwatch == nil {
						t.Logf("`cloudwatch client` %v in region %v still nil", role, region)
						t.Fail()
					}
					if test.cloudwatch {
						continue
					}
					if client.tagging == nil {
						t.Logf("`tagging client` %v in region %v still nil", role, region)
						t.Fail()
					}
					if client.account == nil {
						t.Logf("`asg client` %v in region %v still nil", role, region)
						t.Fail()
					}
				}
			}
		})
	}
}

func TestClientCacheGetCloudwatchClient(t *testing.T) {
	testGetAWSClient(
		t, "Cloudwatch",
		func(t *testing.T, cache *CachingFactory, region string, role model.Role) {
			iface := cache.GetCloudwatchClient(region, role, cloudwatch.ConcurrencyConfig{SingleLimit: 1})
			if iface == nil {
				t.Fail()
				return
			}
		})
}

func TestClientCacheGetTagging(t *testing.T) {
	testGetAWSClient(
		t, "Tagging",
		func(t *testing.T, cache *CachingFactory, region string, role model.Role) {
			iface := cache.GetTaggingClient(region, role, 1)
			if iface == nil {
				t.Fail()
				return
			}
		})
}

func TestClientCacheGetAccount(t *testing.T) {
	testGetAWSClient(
		t, "Account",
		func(t *testing.T, cache *CachingFactory, region string, role model.Role) {
			iface := cache.GetAccountClient(region, role)
			if iface == nil {
				t.Fail()
				return
			}
		})
}

func testGetAWSClient(
	t *testing.T,
	name string,
	testClientGet func(*testing.T, *CachingFactory, string, model.Role),
) {
	region := "us-east-1"
	role := model.Role{}
	tests := []struct {
		descrip     string
		cache       *CachingFactory
		parallelRun bool
	}{
		{
			"locks during unrefreshed parallel call",
			&CachingFactory{
				refreshed: false,
				mu:        sync.Mutex{},
				session:   mock.Session,
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: createCloudWatchClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							tagging:    createTaggingClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							account:    createAccountClient(logging.NewNopLogger(), createStsSession(mock.Session, role, "", false, false), createIamSession(mock.Session, role, false, false)),
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
			true,
		},
		{
			"returns clients if available",
			&CachingFactory{
				refreshed: true,
				session:   mock.Session,
				mu:        sync.Mutex{},
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{
							cloudwatch: createCloudWatchClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							tagging:    createTaggingClient(logging.NewNopLogger(), mock.Session, &region, role, false),
							account:    createAccountClient(logging.NewNopLogger(), createStsSession(mock.Session, role, "", false, false), createIamSession(mock.Session, role, false, false)),
						},
					},
				},
				logger: logging.NewNopLogger(),
			},
			false,
		},
		{
			"creates a new clients if not available",
			&CachingFactory{
				refreshed: true,
				session:   mock.Session,
				mu:        sync.Mutex{},
				stscache: map[model.Role]stsiface.STSAPI{
					{}: nil,
				},
				clients: map[model.Role]map[string]*cachedClients{
					{}: {
						"us-east-1": &cachedClients{},
					},
				},
				logger: logging.NewNopLogger(),
			},
			false,
		},
	}

	for _, l := range tests {
		test := l
		t.Run(name+" "+test.descrip, func(t *testing.T) {
			t.Parallel()
			if test.parallelRun {
				go testClientGet(t, test.cache, region, role)
			}
			testClientGet(t, test.cache, region, role)

			if test.cache.clients[role][region] == nil {
				t.Log("cache is nil when it should be populated")
				t.Fail()
			}
		})
	}
}

func TestSetExternalID(t *testing.T) {
	tests := []struct {
		descrip string
		ID      string
		isSet   bool
	}{
		{
			"sets the external ID if not empty",
			"should-be-set",
			true,
		},
		{
			"external ID not set if empty",
			"",
			false,
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			f := setExternalID(test.ID)
			p := &stscreds.AssumeRoleProvider{}
			f(p)
			if test.isSet {
				if *p.ExternalID != test.ID {
					t.Fail()
				}
			}
		})
	}
}

func TestSetSTSCreds(t *testing.T) {
	tests := []struct {
		descrip        string
		role           model.Role
		credentialsNil bool
		externalID     string
	}{
		{
			"sets the sts creds if the role arn is set",
			model.Role{
				RoleArn: "this:arn",
			},
			false,
			"",
		},
		{
			"does not set the creds if role arn is not set",
			model.Role{},
			true,
			"",
		},
		{
			"does not set the creds if role arn is not set & external id is set",
			model.Role{
				ExternalID: "thing",
			},
			true,
			"",
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			t.Parallel()
			conf := setSTSCreds(mock.Session, &aws.Config{}, test.role)
			if test.credentialsNil {
				if conf.Credentials != nil {
					t.Fail()
				}
			} else {
				if conf.Credentials == nil {
					t.Fail()
				}
			}
		})
	}
}

func TestCreateAWSSession(t *testing.T) {
	tests := []struct {
		descrip string
	}{
		{
			"creates an aws clients",
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			s := createAWSSession(endpoints.DefaultResolver().EndpointFor, false)
			if s == nil {
				t.Fail()
			}
		})
	}
}

func TestCreateStsSession(t *testing.T) {
	tests := []struct {
		descrip   string
		role      model.Role
		stsRegion string
	}{
		{
			"creates an sts clients with an empty role",
			model.Role{},
			"",
		},
		{
			"creates an sts clients with region",
			model.Role{},
			"eu-west-1",
		},
		{
			"creates an sts clients with an empty external id",
			model.Role{
				RoleArn: "some:arn",
			},
			"",
		},
		{
			"creates an sts clients with an empty role arn",
			model.Role{
				ExternalID: "some-id",
			},
			"",
		},
		{
			"creates an sts clients with an sts full role",
			model.Role{
				RoleArn:    "some:arn",
				ExternalID: "some-id",
			},
			"",
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			t.Parallel()
			// just exercise the code path
			iface := createStsSession(mock.Session, test.role, test.stsRegion, false, false)
			if iface == nil {
				t.Fail()
			}
		})
	}
}

func TestCreateCloudwatchSession(t *testing.T) {
	testAWSClient(
		t,
		"Cloudwatch",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createCloudwatchSession(s, region, role, fips, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateTagSession(t *testing.T) {
	testAWSClient(
		t,
		"Tag",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createTagSession(s, region, role, fips)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateAPIGatewaySession(t *testing.T) {
	testAWSClient(
		t,
		"APIGateway",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createAPIGatewaySession(s, region, role, fips, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateAPIGatewayV2Session(t *testing.T) {
	testAWSClient(
		t,
		"APIGatewayV2",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createAPIGatewayV2Session(s, region, role, fips, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateASGSession(t *testing.T) {
	testAWSClient(
		t,
		"ASG",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createASGSession(s, region, role, fips)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateEC2Session(t *testing.T) {
	testAWSClient(
		t,
		"EC2",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createEC2Session(s, region, role, fips, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreatePrometheusSession(t *testing.T) {
	testAWSClient(
		t,
		"Prometheus",
		func(t *testing.T, s *session.Session, region *string, role model.Role, _ bool) {
			iface := createPrometheusSession(s, region, role, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateDMSSession(t *testing.T) {
	testAWSClient(
		t,
		"DMS",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createDMSSession(s, region, role, fips, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestCreateStorageGatewaySession(t *testing.T) {
	testAWSClient(
		t,
		"StorageGateway",
		func(t *testing.T, s *session.Session, region *string, role model.Role, fips bool) {
			iface := createStorageGatewaySession(s, region, role, fips, false)
			if iface == nil {
				t.Fail()
			}
		})
}

func TestSTSResolvesFIPSEnabledEndpoints(t *testing.T) {
	type testcase struct {
		region           string
		expectedEndpoint string
	}

	for _, tc := range []testcase{
		{
			region:           "us-east-1",
			expectedEndpoint: "http://sts-fips.us-east-1.amazonaws.com",
		},
		{
			region:           "us-west-1",
			expectedEndpoint: "http://sts-fips.us-west-1.amazonaws.com",
		},
		{
			region:           "us-gov-east-1",
			expectedEndpoint: "http://sts.us-gov-east-1.amazonaws.com",
		},
	} {
		t.Run(tc.region, func(t *testing.T) {
			var resolverError error
			resolvedEndpoint := endpoints.ResolvedEndpoint{}
			called := false

			mockSession := mock.Session
			mockEndpoint := *mockSession.Config.Endpoint
			previousResolver := mock.Session.Config.EndpointResolver

			// restore mock endpoint after
			t.Cleanup(func() {
				mockSession.Config.Endpoint = aws.String(mockEndpoint)
				mockSession.Config.EndpointResolver = previousResolver
			})

			mockResolverFunc := mock.Session.Config.EndpointResolver.EndpointFor
			mockSession.Config.EndpointResolver = endpoints.ResolverFunc(func(service, region string, opts ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
				resolvedEndpoint, resolverError = mockResolverFunc(service, region, opts...)

				called = true

				return endpoints.ResolvedEndpoint{URL: mockEndpoint}, resolverError
			})

			mockSession.Config.Endpoint = nil

			sess := createStsSession(mock.Session, model.Role{}, tc.region, true, false)
			require.NotNil(t, sess)

			require.True(t, called, "expected endpoint resolver to be called")
			require.NoError(t, resolverError, "no error expected when resolving endpoint")
			require.Equal(t, tc.expectedEndpoint, resolvedEndpoint.URL)
		})
	}
}

func testAWSClient(
	t *testing.T,
	name string,
	testClientCreation func(*testing.T, *session.Session, *string, model.Role, bool),
) {
	tests := []struct {
		descrip string
		region  string
		role    model.Role
		fips    bool
	}{
		{
			fmt.Sprintf("%s client without role and fips is created", name),
			"us-east-1",
			model.Role{},
			false,
		},
		{
			fmt.Sprintf("%s client without role and with fips is created", name),
			"us-east-1",
			model.Role{},
			true,
		},
		{
			fmt.Sprintf("%s client with roleARN and without external id is created", name),
			"us-east-1",
			model.Role{
				RoleArn: "some:arn",
			},
			false,
		},
		{
			fmt.Sprintf("%s client with roleARN and with external id is created", name),
			"us-east-1",
			model.Role{
				RoleArn:    "some:arn",
				ExternalID: "some-id",
			},
			false,
		},
		{
			fmt.Sprintf("%s client without roleARN and with external id is created", name),
			"us-east-1",
			model.Role{
				ExternalID: "some-id",
			},
			false,
		},
	}

	for _, l := range tests {
		test := l
		t.Run(test.descrip, func(t *testing.T) {
			t.Parallel()
			testClientCreation(t, mock.Session, &test.region, test.role, test.fips)
		})
	}
}
