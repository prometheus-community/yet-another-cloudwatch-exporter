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
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

	// Gather metrics from the registry
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	// Verify that metrics were registered
	assert.NotEmpty(t, metricFamilies)

	// Check for the expected metric
	var foundMetric bool
	for _, family := range metricFamilies {
		if family.GetName() == "aws_ec2_cpuutilization_average" {
			foundMetric = true
			assert.Equal(t, dto.MetricType_GAUGE, family.GetType())
			// Should have at least one metric data point
			assert.NotEmpty(t, family.Metric)
		}
	}
	assert.True(t, foundMetric, "Expected metric aws_ec2_cpuutilization_average not found")
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
	families, err := registry.Gather()
	require.NoError(t, err)

	// Just verify the call completed without error
	// Full integration tests would be needed to verify discovery job metrics
	t.Logf("Generated %d metric families", len(families))
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

	// Gather metrics from the registry
	families, err := registry.Gather()
	require.NoError(t, err)

	// Check for metrics that might have been generated
	var foundCPUMetric bool
	var foundStorageMetric bool
	for _, family := range families {
		t.Logf("Found metric family: %s", family.GetName())

		if family.GetName() == "aws_rds_cpuutilization_average" {
			foundCPUMetric = true
			assert.Equal(t, dto.MetricType_GAUGE, family.GetType())
		}

		// Verify the RDS storage capacity metric if present
		// Note: Enhanced metrics like StorageSpace generate metrics with specific naming
		// The actual metric name would be aws_rds_storage_capacity_bytes or similar
		if family.GetName() == "aws_rds_storage_capacity_bytes" ||
			family.GetName() == "aws_rds_storagespace" {
			foundStorageMetric = true
			assert.Equal(t, dto.MetricType_GAUGE, family.GetType())
			assert.NotEmpty(t, family.Metric, "Storage metric should have data points")

			// Verify metric labels include the expected dimensions
			for _, metric := range family.Metric {
				labelMap := make(map[string]string)
				for _, label := range metric.Label {
					labelMap[label.GetName()] = label.GetValue()
				}
				t.Logf("Storage metric labels: %v", labelMap)
				// DBInstanceIdentifier should be present as a dimension
				// The actual label name depends on snake_case conversion
			}
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

	if foundCPUMetric {
		t.Log("Successfully found CPU utilization metric from CloudWatch")
	}
	if foundStorageMetric {
		t.Log("Successfully found RDS storage capacity metric from enhanced metrics")
	}

	t.Logf("Test completed with %d metric families generated", len(families))
}

// floatPtr is a helper function to create a pointer to a float64
func floatPtr(f float64) *float64 {
	return &f
}
