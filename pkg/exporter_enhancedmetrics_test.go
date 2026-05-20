package exporter

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	dynamodbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	elasticacheTypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics"
	enhancedmetricsDynamoDBService "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/dynamodb"
	enhancedmetricsElastiCacheService "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/elasticache"
	enhancedmetricsLambdaService "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/lambda"
	enhancedmetricsService "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var (
	_ account.Client    = &mockAccountClient{}
	_ cloudwatch.Client = &mockCloudwatchClient{}
	_ tagging.Client    = &mockTaggingClient{}
)

// mockFactory is a local mock that implements both clients.Factory and config.RegionalConfigProvider
type mockFactoryForEnhancedMetrics struct {
	accountClient    account.Client
	cloudwatchClient cloudwatch.Client
	taggingClient    tagging.Client
	awsConfig        *aws.Config
}

// GetAccountClient implements clients.Factory
func (m *mockFactoryForEnhancedMetrics) GetAccountClient(string, model.Role) account.Client {
	return m.accountClient
}

// GetCloudwatchClient implements clients.Factory
func (m *mockFactoryForEnhancedMetrics) GetCloudwatchClient(string, model.Role, cloudwatch.ConcurrencyConfig) cloudwatch.Client {
	return m.cloudwatchClient
}

// GetTaggingClient implements clients.Factory
func (m *mockFactoryForEnhancedMetrics) GetTaggingClient(string, model.Role, int) tagging.Client {
	return m.taggingClient
}

// GetAWSRegionalConfig implements config.RegionalConfigProvider
func (m *mockFactoryForEnhancedMetrics) GetAWSRegionalConfig(string, model.Role) *aws.Config {
	return m.awsConfig
}

// mockRDSClient implements the RDS Client interface for testing
type mockRDSClient struct {
	instances []types.DBInstance
	err       error
}

func (m *mockRDSClient) DescribeDBInstances(context.Context, *slog.Logger, []string) ([]types.DBInstance, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.instances, nil
}

// mockLambdaClient implements the Lambda Client interface for testing
type mockLambdaClient struct {
	functions []lambdaTypes.FunctionConfiguration
	err       error
}

func (m *mockLambdaClient) ListAllFunctions(context.Context, *slog.Logger) ([]lambdaTypes.FunctionConfiguration, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.functions, nil
}

// mockElastiCacheClient implements the ElastiCache Client interface for testing
type mockElastiCacheClient struct {
	clusters []elasticacheTypes.CacheCluster
	err      error
}

func (m *mockElastiCacheClient) DescribeAllCacheClusters(context.Context, *slog.Logger) ([]elasticacheTypes.CacheCluster, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clusters, nil
}

// mockDynamoDBClient implements the DynamoDB Client interface for testing
type mockDynamoDBClient struct {
	tables []dynamodbTypes.TableDescription
	err    error
}

func (m *mockDynamoDBClient) DescribeTables(context.Context, *slog.Logger, []string) ([]dynamodbTypes.TableDescription, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tables, nil
}

func TestUpdateMetrics_WithEnhancedMetrics_RDS(t *testing.T) {
	defer enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsService.NewRDSService(nil),
	)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

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
		metrics:           []*model.Metric{},
		metricDataResults: []cloudwatch.MetricDataResult{},
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
	mockRDSClientBuilder := func(_ aws.Config) enhancedmetricsService.Client {
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
	enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsService.NewRDSService(mockRDSClientBuilder),
	)

	factory := &mockFactoryForEnhancedMetrics{
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
	require.NotNil(t, metrics)
	require.Len(t, metrics, 2)

	expectedMetric := `
		# HELP aws_rds_info Help is not implemented yet.
		# TYPE aws_rds_info gauge
		aws_rds_info{name="arn:aws:rds:us-east-1:123456789012:db:test-db",tag_Name="test-db"} 0
		# HELP aws_rds_allocated_storage Help is not implemented yet.
		# TYPE aws_rds_allocated_storage gauge
		aws_rds_allocated_storage{account_alias="test-account",account_id="123456789012",dimension_DBInstanceIdentifier="test-db",name="arn:aws:rds:us-east-1:123456789012:db:test-db",region="us-east-1",tag_Name="test-db"} 1.073741824e+11
`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric))
	require.NoError(t, err)
}

func TestUpdateMetrics_WithEnhancedMetrics_Lambda(t *testing.T) {
	defer enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsLambdaService.NewLambdaService(nil),
	)

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

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
		metrics:           []*model.Metric{},
		metricDataResults: []cloudwatch.MetricDataResult{},
	}

	mockTagClient := &mockTaggingClient{
		resources: []*model.TaggedResource{
			{
				ARN:       "arn:aws:lambda:us-east-1:123456789012:function:test-function",
				Namespace: "AWS/Lambda",
				Region:    "us-east-1",
				Tags: []model.Tag{
					{Key: "Name", Value: "test-function"},
				},
			},
		},
	}

	// Create a mock Lambda client builder function for testing
	mockLambdaClientBuilder := func(_ aws.Config) enhancedmetricsLambdaService.Client {
		return &mockLambdaClient{
			functions: []lambdaTypes.FunctionConfiguration{
				{
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:test-function"),
					FunctionName: aws.String("test-function"),
					Timeout:      aws.Int32(300),
				},
			},
		}
	}

	// Register the Lambda service with the mock builder in the default registry
	enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsLambdaService.NewLambdaService(mockLambdaClientBuilder),
	)

	factory := &mockFactoryForEnhancedMetrics{
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
				Namespace: "AWS/Lambda",
				Roles:     []model.Role{{RoleArn: "arn:aws:iam::123456789012:role/test-role"}},
				EnhancedMetrics: []*model.EnhancedMetricConfig{
					{
						Name: "Timeout",
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
	require.NotNil(t, metrics)
	require.Len(t, metrics, 2)

	expectedMetric := `
		# HELP aws_lambda_info Help is not implemented yet.
		# TYPE aws_lambda_info gauge
		aws_lambda_info{name="arn:aws:lambda:us-east-1:123456789012:function:test-function",tag_Name="test-function"} 0
		# HELP aws_lambda_timeout Help is not implemented yet.
		# TYPE aws_lambda_timeout gauge
		aws_lambda_timeout{account_alias="test-account",account_id="123456789012",dimension_FunctionName="test-function",name="arn:aws:lambda:us-east-1:123456789012:function:test-function",region="us-east-1",tag_Name="test-function"} 300
`
	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric))
	require.NoError(t, err)
}

func TestUpdateMetrics_WithEnhancedMetrics_ElastiCache(t *testing.T) {
	defer enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsElastiCacheService.NewElastiCacheService(nil),
	)

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

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
		metrics:           []*model.Metric{},
		metricDataResults: []cloudwatch.MetricDataResult{},
	}

	mockTagClient := &mockTaggingClient{
		resources: []*model.TaggedResource{
			{
				ARN:       "arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster",
				Namespace: "AWS/ElastiCache",
				Region:    "us-east-1",
				Tags: []model.Tag{
					{Key: "Name", Value: "test-cluster"},
				},
			},
		},
	}

	// Create a mock ElastiCache client builder function for testing
	mockElastiCacheClientBuilder := func(_ aws.Config) enhancedmetricsElastiCacheService.Client {
		return &mockElastiCacheClient{
			clusters: []elasticacheTypes.CacheCluster{
				{
					ARN:            aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster"),
					CacheClusterId: aws.String("test-cluster"),
					NumCacheNodes:  aws.Int32(3),
				},
			},
		}
	}

	// Register the ElastiCache service with the mock builder in the default registry
	enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsElastiCacheService.NewElastiCacheService(mockElastiCacheClientBuilder),
	)

	factory := &mockFactoryForEnhancedMetrics{
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
				Namespace: "AWS/ElastiCache",
				Roles:     []model.Role{{RoleArn: "arn:aws:iam::123456789012:role/test-role"}},
				EnhancedMetrics: []*model.EnhancedMetricConfig{
					{
						Name: "NumCacheNodes",
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
	require.NotNil(t, metrics)
	require.Len(t, metrics, 2)

	expectedMetric := `
		# HELP aws_elasticache_info Help is not implemented yet.
		# TYPE aws_elasticache_info gauge
		aws_elasticache_info{name="arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster",tag_Name="test-cluster"} 0
		# HELP aws_elasticache_num_cache_nodes Help is not implemented yet.
		# TYPE aws_elasticache_num_cache_nodes gauge
		aws_elasticache_num_cache_nodes{account_alias="test-account",account_id="123456789012",dimension_CacheClusterId="test-cluster",name="arn:aws:elasticache:us-east-1:123456789012:cluster:test-cluster",region="us-east-1",tag_Name="test-cluster"} 3
`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric))
	require.NoError(t, err)
}

func TestUpdateMetrics_WithEnhancedMetrics_DynamoDB(t *testing.T) {
	defer enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsDynamoDBService.NewDynamoDBService(nil),
	)

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

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
		metrics:           []*model.Metric{},
		metricDataResults: []cloudwatch.MetricDataResult{},
	}

	mockTagClient := &mockTaggingClient{
		resources: []*model.TaggedResource{
			{
				ARN:       "arn:aws:dynamodb:us-east-1:123456789012:table/test-table",
				Namespace: "AWS/DynamoDB",
				Region:    "us-east-1",
				Tags: []model.Tag{
					{Key: "Name", Value: "test-table"},
				},
			},
		},
	}

	// Create a mock DynamoDB client builder function for testing
	mockDynamoDBClientBuilder := func(_ aws.Config) enhancedmetricsDynamoDBService.Client {
		return &mockDynamoDBClient{
			tables: []dynamodbTypes.TableDescription{
				{
					TableArn:  aws.String("arn:aws:dynamodb:us-east-1:123456789012:table/test-table"),
					TableName: aws.String("test-table"),
					ItemCount: aws.Int64(1000),
					GlobalSecondaryIndexes: []dynamodbTypes.GlobalSecondaryIndexDescription{
						{
							IndexName: aws.String("GSI1"),
							ItemCount: aws.Int64(500),
						},
						{
							IndexName: aws.String("GSI2"),
							ItemCount: aws.Int64(300),
						},
					},
				},
			},
		}
	}

	// Register the DynamoDB service with the mock builder in the default registry
	enhancedmetrics.DefaultEnhancedMetricServiceRegistry.Register(
		enhancedmetricsDynamoDBService.NewDynamoDBService(mockDynamoDBClientBuilder),
	)

	factory := &mockFactoryForEnhancedMetrics{
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
				Namespace: "AWS/DynamoDB",
				Roles:     []model.Role{{RoleArn: "arn:aws:iam::123456789012:role/test-role"}},
				EnhancedMetrics: []*model.EnhancedMetricConfig{
					{
						Name: "ItemCount",
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
	require.NotNil(t, metrics)
	require.Len(t, metrics, 2)

	expectedMetric := `
		# HELP aws_dynamodb_info Help is not implemented yet.
		# TYPE aws_dynamodb_info gauge
		aws_dynamodb_info{name="arn:aws:dynamodb:us-east-1:123456789012:table/test-table",tag_Name="test-table"} 0
		# HELP aws_dynamodb_item_count Help is not implemented yet.
		# TYPE aws_dynamodb_item_count gauge
		aws_dynamodb_item_count{account_alias="test-account",account_id="123456789012",dimension_GlobalSecondaryIndexName="",dimension_TableName="test-table",name="arn:aws:dynamodb:us-east-1:123456789012:table/test-table",region="us-east-1",tag_Name="test-table"} 1000
		aws_dynamodb_item_count{account_alias="test-account",account_id="123456789012",dimension_GlobalSecondaryIndexName="GSI1",dimension_TableName="test-table",name="arn:aws:dynamodb:us-east-1:123456789012:table/test-table",region="us-east-1",tag_Name="test-table"} 500
		aws_dynamodb_item_count{account_alias="test-account",account_id="123456789012",dimension_GlobalSecondaryIndexName="GSI2",dimension_TableName="test-table",name="arn:aws:dynamodb:us-east-1:123456789012:table/test-table",region="us-east-1",tag_Name="test-table"} 300
`

	err = testutil.GatherAndCompare(registry, strings.NewReader(expectedMetric))
	require.NoError(t, err)
}
