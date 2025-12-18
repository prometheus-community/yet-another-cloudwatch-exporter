package exporter

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics"
	enhancedmetricsService "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

// mockFactory is a local mock that implements both clients.Factory and config.RegionalConfigProvider
type mockFactory struct {
	accountClient    account.Client
	cloudwatchClient cloudwatch.Client
	taggingClient    tagging.Client
	awsConfig        *aws.Config
}

// Ensure mockFactory implements both interfaces at compile time
var (
	_ account.Client    = &mockAccountClient{}
	_ cloudwatch.Client = &mockCloudwatchClient{}
	_ tagging.Client    = &mockTaggingClient{}
)

// GetAccountClient implements clients.Factory
func (m *mockFactory) GetAccountClient(region string, role model.Role) account.Client {
	return m.accountClient
}

// GetCloudwatchClient implements clients.Factory
func (m *mockFactory) GetCloudwatchClient(region string, role model.Role, concurrency cloudwatch.ConcurrencyConfig) cloudwatch.Client {
	return m.cloudwatchClient
}

// GetTaggingClient implements clients.Factory
func (m *mockFactory) GetTaggingClient(region string, role model.Role, concurrency int) tagging.Client {
	return m.taggingClient
}

// GetAWSRegionalConfig implements config.RegionalConfigProvider
func (m *mockFactory) GetAWSRegionalConfig(region string, role model.Role) *aws.Config {
	return m.awsConfig
}

// mockAccountClient implements account.Client
type mockAccountClient struct {
	accountID    string
	accountAlias string
	err          error
}

func (m *mockAccountClient) GetAccount(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.accountID, nil
}

func (m *mockAccountClient) GetAccountAlias(_ context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.accountAlias, nil
}

// mockCloudwatchClient implements cloudwatch.Client
type mockCloudwatchClient struct {
	listMetricsResult       []*model.Metric
	getMetricDataResult     []cloudwatch.MetricDataResult
	getMetricStatisticsData []*model.MetricStatisticsResult
	err                     error
}

func (m *mockCloudwatchClient) ListMetrics(ctx context.Context, namespace string, metric *model.MetricConfig, recentlyActiveOnly bool, fn func([]*model.Metric)) error {
	if m.err != nil {
		return m.err
	}
	if fn != nil {
		fn(m.listMetricsResult)
	}
	return nil
}

func (m *mockCloudwatchClient) GetMetricData(ctx context.Context, getMetricData []*model.CloudwatchData, namespace string, startTime time.Time, endTime time.Time) []cloudwatch.MetricDataResult {
	return m.getMetricDataResult
}

func (m *mockCloudwatchClient) GetMetricStatistics(ctx context.Context, logger *slog.Logger, dimensions []model.Dimension, namespace string, metric *model.MetricConfig) []*model.MetricStatisticsResult {
	return m.getMetricStatisticsData
}

// mockTaggingClient implements tagging.Client
type mockTaggingClient struct {
	resources []*model.TaggedResource
	err       error
}

func (m *mockTaggingClient) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resources, nil
}

// mockRDSClient implements the RDS Client interface for testing
type mockRDSClient struct {
	instances []types.DBInstance
	err       error
}

func (m *mockRDSClient) DescribeAllDBInstances(ctx context.Context, logger *slog.Logger) ([]types.DBInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

func TestUpdateMetrics_WithEnhancedMetrics_RDS(t *testing.T) {
	// restore the original state after the test, it is important to avoid side effects on other tests.
	// However, it should be changed to use a separate registry for each test in the future.
	defer enhancedmetrics.DefaultRegistry.Remove("AWS/RDS").Register(enhancedmetricsService.NewRDSService(nil))

	ctx := context.Background()
	//logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logger := slog.Default()

	// Create a test AWS config
	testAWSConfig := &aws.Config{
		Region: "us-east-1",
	}

	// Create mock clients
	mockAcctClient := &mockAccountClient{
		accountID:    "123456789012",
		accountAlias: "test-account",
	}

	mockCWClient := &mockCloudwatchClient{
		listMetricsResult:   []*model.Metric{},
		getMetricDataResult: []cloudwatch.MetricDataResult{},
	}

	mockTagClient := &mockTaggingClient{
		resources: []*model.TaggedResource{
			{
				ARN:       "arn:aws:rds:us-east-1:123456789012:db:test-db",
				Namespace: "AWS/RDS",
				Region:    "us-east-1",
				Tags: []model.Tag{
					{Key: "Name", Value: "test-db"},
				},
			},
		},
	}

	// Create a mock RDS client builder function for testing
	mockRDSClientBuilder := func(cfg aws.Config) enhancedmetricsService.Client {
		return &mockRDSClient{
			instances: []types.DBInstance{
				{
					DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456789012:db:test-db"),
					DBInstanceIdentifier: aws.String("test-db"),
					AllocatedStorage:     aws.Int32(100),
				},
			},
		}
	}

	// Register the RDS service with the mock builder in the default registry
	enhancedmetrics.DefaultRegistry.Remove("AWS/RDS").Register(
		enhancedmetricsService.NewRDSService(mockRDSClientBuilder))

	// Create the mock factory that implements both interfaces
	factory := &mockFactory{
		accountClient:    mockAcctClient,
		cloudwatchClient: mockCWClient,
		taggingClient:    mockTagClient,
		awsConfig:        testAWSConfig,
	}

	// Create a test job config with enhanced metrics
	jobsCfg := model.JobsConfig{
		DiscoveryJobs: []model.DiscoveryJob{
			{
				Regions:   []string{"us-east-1"},
				Namespace: "AWS/RDS",
				Roles:     []model.Role{{RoleArn: "arn:aws:iam::123456789012:role/test-role"}},
				EnhancedMetrics: []*model.EnhancedMetricConfig{
					{
						Name: "AllocatedStorage",
					},
				},
				ExportedTagsOnMetrics: []string{"Name"},
			},
		},
	}

	registry := prometheus.NewRegistry()

	err := UpdateMetrics(ctx, logger, jobsCfg, registry, factory)
	require.NoError(t, err)

	metrics, err := registry.Gather()
	require.NoError(t, err)

	// We verify the process completes successfully
	// In a real scenario, enhanced metrics would be generated
	require.NotNil(t, metrics)

	require.Len(t, metrics, 2)

	expectedMetric := `
		# HELP aws_rds_info Help is not implemented yet.
		# TYPE aws_rds_info gauge
		aws_rds_info{name="arn:aws:rds:us-east-1:123456789012:db:test-db",tag_Name="test-db"} 0
`
	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric), "aws_rds_info")

	require.NoError(t, err)
	expectedMetric = `
		# HELP aws_rds_storage_capacity Help is not implemented yet.
		# TYPE aws_rds_storage_capacity gauge
		aws_rds_storage_capacity{account_alias="test-account",account_id="123456789012",dimension_DBInstanceIdentifier="test-db",name="arn:aws:rds:us-east-1:123456789012:db:test-db",region="us-east-1",tag_Name="test-db"} 100
`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric), "aws_rds_storage_capacity")
	require.NoError(t, err)
}
