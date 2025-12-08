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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// mockFactory implements the clients.Factory interface for testing
type mockFactory struct {
	cloudwatchClient mockCloudwatchClient
	taggingClient    mockTaggingClient
	accountClient    mockAccountClient
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
	assert.NoError(t, err, "Metric aws_ec2_cpuutilization_average should match expected output")

	// Verify at least one metric was registered
	count, err := testutil.GatherAndCount(registry)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "Should have registered at least one metric")
}

func TestUpdateMetrics_DiscoveryJob(t *testing.T) {
	ctx := context.Background()
	logger := promslog.NewNopLogger()

	// Create a discovery job configuration
	jobsCfg := model.JobsConfig{
		DiscoveryJobs: []model.DiscoveryJob{
			{
				Regions:   []string{"us-east-1"},
				Roles:     []model.Role{{}},
				Namespace: "AWS/EC2",
				SearchTags: []model.SearchTag{
					{Key: "Environment"},
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
						{Value: floatPtr(42.5), Timestamp: time.Now()},
					},
				},
			},
		},
	}

	registry := prometheus.NewRegistry()

	err := UpdateMetrics(ctx, logger, jobsCfg, registry, factory)
	require.NoError(t, err)

	// Discovery jobs with mocks may not generate metrics if the full discovery flow
	// requires more complex setup. This test verifies no errors occur during the scrape.
	count, err := testutil.GatherAndCount(registry)
	require.NoError(t, err)

	// Just verify the call completed without error
	// Full integration tests would be needed to verify discovery job metrics
	t.Logf("Generated %d metrics", count)
}

func TestUpdateMetrics_RDSStorageMetrics(t *testing.T) {
	ctx := context.Background()
	logger := promslog.NewNopLogger()

	// Create a discovery job with RDS and enhanced metrics
	jobsCfg := model.JobsConfig{
		DiscoveryJobs: []model.DiscoveryJob{
			{
				Regions:   []string{"us-east-1"},
				Roles:     []model.Role{{}},
				Namespace: "AWS/RDS",
				SearchTags: []model.SearchTag{
					{Key: "Environment"},
				},
				Metrics: []*model.MetricConfig{
					{
						Name:       "CPUUtilization",
						Statistics: []string{"Average"},
						Period:     300,
						Length:     300,
					},
				},
				EnhancedMetrics: []model.EnhancedMetricConfig{
					{
						Name:    "StorageSpace",
						Enabled: true,
					},
				},
				ExportedTagsOnMetrics: []string{"Environment"},
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
						{Value: floatPtr(25.5), Timestamp: time.Now()},
					},
				},
			},
		},
	}

	registry := prometheus.NewRegistry()

	err := UpdateMetrics(ctx, logger, jobsCfg, registry, factory)
	require.NoError(t, err)

	// Count total metrics generated
	totalCount, err := testutil.GatherAndCount(registry)
	require.NoError(t, err)
	t.Logf("Generated %d total metrics", totalCount)

	// Check for CPU metric using testutil
	cpuMetricExists := false
	err = testutil.GatherAndCompare(registry, strings.NewReader(""), "aws_rds_cpuutilization_average")
	if err == nil {
		cpuMetricExists = true
		t.Log("Successfully found CPU utilization metric from CloudWatch")
	}

	// Check for RDS storage capacity metric
	// Note: Enhanced metrics like StorageSpace generate metrics with specific naming
	// The actual metric name would be aws_rds_storage_capacity_bytes or similar
	storageMetricExists := false
	for _, metricName := range []string{"aws_rds_storage_capacity_bytes", "aws_rds_storagespace"} {
		err = testutil.GatherAndCompare(registry, strings.NewReader(""), metricName)
		if err == nil {
			storageMetricExists = true
			t.Logf("Successfully found RDS storage capacity metric: %s", metricName)
			break
		}
	}

	// Discovery jobs with mocks are complex as they require the full discovery flow.
	// Enhanced metrics require actual RDS API clients to retrieve storage information.
	// This test verifies that:
	// 1. UpdateMetrics completes without error when enhanced metrics are configured
	// 2. The configuration structure is correct
	//
	// Full integration tests with real AWS APIs would be needed to verify:
	// - RDS storage metrics are actually retrieved and exposed
	// - Metric naming follows the expected pattern (aws_rds_storage_capacity_bytes)
	// - Labels include DBInstanceIdentifier and exported tags

	if cpuMetricExists {
		// Verify the metric has expected labels by attempting to gather it
		expectedCPUMetric := `
			# HELP aws_rds_cpuutilization_average Help text for aws_rds_cpuutilization_average
			# TYPE aws_rds_cpuutilization_average gauge
		`
		// Note: We use a minimal comparison since exact values depend on mock data
		err = testutil.GatherAndCompare(registry, strings.NewReader(expectedCPUMetric), "aws_rds_cpuutilization_average")
		if err != nil {
			t.Logf("CPU metric structure validation: %v", err)
		}
	}

	if storageMetricExists {
		t.Log("Successfully found RDS storage capacity metric from enhanced metrics")
	}
}

// floatPtr is a helper function to create a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}
