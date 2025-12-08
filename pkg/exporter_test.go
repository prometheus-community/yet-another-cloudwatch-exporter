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
package exporter

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	rdsclient "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/enhanced/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// mockFactory implements the clients.Factory interface for testing
type mockFactory struct {
	cloudwatchClient mockCloudwatchClient
	taggingClient    mockTaggingClient
	accountClient    mockAccountClient
	rdsClient        *mockRDSClient
}

func (f *mockFactory) GetCloudwatchClient(region string, role model.Role, concurrency cloudwatch.ConcurrencyConfig) cloudwatch.Client {
	return &f.cloudwatchClient
}

func (f *mockFactory) GetTaggingClient(region string, role model.Role, concurrencyLimit int) tagging.Client {
	return f.taggingClient
}

func (f *mockFactory) GetAccountClient(region string, role model.Role) account.Client {
	return f.accountClient
}

// GetRDSClient returns a mock RDS client for testing
func (f *mockFactory) GetRDSClient() rdsclient.ClientInterface {
	if f.rdsClient == nil {
		return nil
	}
	return f.rdsClient
}

// mockAccountClient implements the account.Client interface
type mockAccountClient struct {
	accountID    string
	accountAlias string
	err          error
}

func (m mockAccountClient) GetAccount(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.accountID, nil
}

func (m mockAccountClient) GetAccountAlias(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.accountAlias, nil
}

// mockTaggingClient implements the tagging.Client interface
type mockTaggingClient struct {
	resources []*model.TaggedResource
	err       error
}

func (m mockTaggingClient) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resources, nil
}

// mockCloudwatchClient implements the cloudwatch.Client interface
type mockCloudwatchClient struct {
	metrics           []*model.Metric
	metricDataResults []cloudwatch.MetricDataResult
	err               error
}

func (m *mockCloudwatchClient) ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, recentlyActive bool, fn func(page []*model.Metric)) error {
	if m.err != nil {
		return m.err
	}
	if len(m.metrics) > 0 {
		fn(m.metrics)
	}
	return nil
}

func (m *mockCloudwatchClient) GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []cloudwatch.MetricDataResult {
	return m.metricDataResults
}

func (m *mockCloudwatchClient) GetMetricStatistics(ctx context.Context, logger *slog.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.MetricStatisticsResult {
	// Return a simple metric statistics result for testing
	now := time.Now()
	avg := 42.0
	return []*model.MetricStatisticsResult{
		{
			Timestamp: &now,
			Average:   &avg,
		},
	}
}

// mockRDSClient implements the rdsclient.Client interface for testing
type mockRDSClient struct {
	instances []types.DBInstance
	err       error
}

func (m *mockRDSClient) DescribeAllDBInstances(ctx context.Context) ([]types.DBInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

func TestUpdateMetrics_StaticJob(t *testing.T) {
	ctx := context.Background()
	logger := promslog.NewNopLogger()

	// Create a simple static job configuration
	jobsCfg := model.JobsConfig{
		StaticJobs: []model.StaticJob{
			{
				Name:      "test-static-job",
				Regions:   []string{"us-east-1"},
				Roles:     []model.Role{{}},
				Namespace: "AWS/EC2",
				Dimensions: []model.Dimension{
					{Name: "InstanceId", Value: "i-1234567890abcdef0"},
				},
				Metrics: []*model.MetricConfig{
					{
						Name:       "CPUUtilization",
						Statistics: []string{"Average"},
						Period:     300,
						Length:     300,
					},
				},
			},
		},
	}

	factory := &mockFactory{
		accountClient: mockAccountClient{
			accountID:    "123456789012",
			accountAlias: "test-account",
		},
		cloudwatchClient: mockCloudwatchClient{},
	}

	registry := prometheus.NewRegistry()

	err := UpdateMetrics(ctx, logger, jobsCfg, registry, factory)
	require.NoError(t, err)

	// Verify the expected metric exists using testutil
	expectedMetric := `
		# HELP aws_ec2_cpuutilization_average Help is not implemented yet.
		# TYPE aws_ec2_cpuutilization_average gauge
		aws_ec2_cpuutilization_average{account_alias="test-account",account_id="123456789012",dimension_InstanceId="i-1234567890abcdef0",name="test-static-job",region="us-east-1"} 42
	`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric), "aws_ec2_cpuutilization_average")
	require.NoError(t, err, "Metric aws_ec2_cpuutilization_average should match expected output")
}

func TestUpdateMetrics_DiscoveryJob(t *testing.T) {
	ctx := context.Background()
	logger := promslog.NewNopLogger()

	// Create a ScrapeConf configuration and convert it to model.JobsConfig
	conf := &config.ScrapeConf{
		Discovery: config.Discovery{
			Jobs: []*config.Job{
				{
					Type:    "AWS/EC2",
					Regions: []string{"us-east-1"},
					Roles:   []config.Role{{}},
					SearchTags: []config.Tag{
						{Key: "Environment", Value: ".*"},
					},
					Metrics: []*config.Metric{
						{
							Name:       "CPUUtilization",
							Statistics: []string{"Average"},
							Period:     300,
							Length:     300,
						},
					},
				},
			},
		},
	}
	jobsCfg := conf.ToModelConfig()

	factory := &mockFactory{
		accountClient: mockAccountClient{
			accountID:    "123456789012",
			accountAlias: "test-account",
		},
		taggingClient: mockTaggingClient{
			resources: []*model.TaggedResource{
				{
					ARN:       "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
					Namespace: "AWS/EC2",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{Key: "Environment", Value: "production"},
						{Key: "Name", Value: "test-instance"},
					},
				},
			},
		},
		cloudwatchClient: mockCloudwatchClient{
			metrics: []*model.Metric{
				{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/EC2",
					Dimensions: []model.Dimension{
						{Name: "InstanceId", Value: "i-1234567890abcdef0"},
					},
				},
			},
			metricDataResults: []cloudwatch.MetricDataResult{
				{
					ID: "id_0",
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(42.5), Timestamp: time.Now()},
					},
				},
			},
		},
	}

	registry := prometheus.NewRegistry()

	err := UpdateMetrics(ctx, logger, jobsCfg, registry, factory)
	require.NoError(t, err)

	expectedMetric := `
		# HELP aws_ec2_cpuutilization_average Help is not implemented yet.
		# TYPE aws_ec2_cpuutilization_average gauge
		aws_ec2_cpuutilization_average{account_alias="test-account", account_id="123456789012",dimension_InstanceId="i-1234567890abcdef0",name="arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",region="us-east-1"} 42.5
	`
	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric), "aws_ec2_cpuutilization_average")
	require.NoError(t, err)
}

func TestUpdateMetrics_RDSStorageMetrics(t *testing.T) {
	ctx := context.Background()
	logger := promslog.NewNopLogger()

	// Create a ScrapeConf configuration and convert it to model.JobsConfig
	conf := &config.ScrapeConf{
		Discovery: config.Discovery{
			Jobs: []*config.Job{
				{
					Type:    "AWS/RDS",
					Regions: []string{"us-east-1"},
					Roles:   []config.Role{{}},
					SearchTags: []config.Tag{
						{Key: "Environment"},
					},
					Metrics: []*config.Metric{
						{
							Name:       "CPUUtilization",
							Statistics: []string{"Average"},
							Period:     300,
							Length:     300,
						},
					},
					EnhancedMetrics: []*config.EnhancedMetric{
						{
							Name:    "StorageSpace",
							Enabled: aws.Bool(true),
						},
					},
				},
			},
			ExportedTagsOnMetrics: config.ExportedTagsOnMetrics{
				"AWS/RDS": []string{"Environment"},
			},
		},
	}
	jobsCfg := conf.ToModelConfig()

	// Create mock RDS client with database instance data
	mockRDS := &mockRDSClient{
		instances: []types.DBInstance{
			{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:my-database"),
				DBInstanceIdentifier: aws.String("my-database"),
				DBInstanceClass:      aws.String("db.t3.micro"),
				Engine:               aws.String("postgres"),
				AllocatedStorage:     aws.Int32(100), // 100 GB
			},
		},
	}

	factory := &mockFactory{
		accountClient: mockAccountClient{
			accountID:    "123456789012",
			accountAlias: "test-account",
		},
		taggingClient: mockTaggingClient{
			resources: []*model.TaggedResource{
				{
					ARN:       "arn:aws:rds:us-east-1:123456789012:db:my-database",
					Namespace: "AWS/RDS",
					Region:    "us-east-1",
					Tags: []model.Tag{
						{Key: "Environment", Value: "production"},
						{Key: "Name", Value: "my-database"},
					},
				},
			},
		},
		cloudwatchClient: mockCloudwatchClient{
			metrics: []*model.Metric{
				{
					MetricName: "CPUUtilization",
					Namespace:  "AWS/RDS",
					Dimensions: []model.Dimension{
						{Name: "DBInstanceIdentifier", Value: "my-database"},
					},
				},
			},
			metricDataResults: []cloudwatch.MetricDataResult{
				{
					ID: "id_0",
					DataPoints: []cloudwatch.DataPoint{
						{Value: aws.Float64(25.5), Timestamp: time.Now()},
					},
				},
			},
		},
		rdsClient: mockRDS,
	}

	registry := prometheus.NewRegistry()

	err := UpdateMetrics(ctx, logger, jobsCfg, registry, factory)
	require.NoError(t, err)

	// Check for and verify CPU metric value using testutil
	expectedCPUMetric := `
		# HELP aws_rds_cpuutilization_average Help is not implemented yet.
		# TYPE aws_rds_cpuutilization_average gauge
		aws_rds_cpuutilization_average{account_alias="test-account",account_id="123456789012",dimension_DBInstanceIdentifier="my-database",name="arn:aws:rds:us-east-1:123456789012:db:my-database",region="us-east-1",tag_Environment="production"} 25.5
	        # HELP aws_rds_storage_space_value Help is not implemented yet.
# TYPE aws_rds_storage_space_value gauge
aws_rds_storage_space_value{account_alias="test-account",account_id="123456789012",dimension_DBInstanceIdentifier="my-database",dimension_DatabaseClass="db.t3.micro",dimension_EngineName="postgres",name="arn:aws:rds:us-east-1:123456789012:db:my-database",region="us-east-1",tag_Environment="production"} 100
	`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedCPUMetric), "aws_rds_cpuutilization_average", "aws_rds_storage_space_value")
	require.NoError(t, err)
}
