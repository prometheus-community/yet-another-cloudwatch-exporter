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
package rds

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const awsRdsNamespace = "AWS/RDS"

type Client interface {
	DescribeDBInstances(ctx context.Context, logger *slog.Logger, dbInstances []string) ([]types.DBInstance, error)
}

// dbInstanceIdentifierFromARN extracts the DB instance identifier from an RDS DB instance ARN,
// whose resource segment has the form "db:<identifier>", e.g.
//
//	arn:aws:rds:eu-west-1:123456789012:db:my-db -> ("my-db", true)
//
// It returns ok=false for any other RDS ARN (clusters, etc.): those identifiers are
// not valid values for the DescribeDBInstances "db-instance-id" filter, which would otherwise
// reject the whole request. The identifier (not the ARN) is what the filter accepts.
func dbInstanceIdentifierFromARN(resourceARN string) (string, bool) {
	parsed, err := arn.Parse(resourceARN)
	if err != nil {
		return "", false
	}

	resourceType, id, found := strings.Cut(parsed.Resource, ":")
	if !found || resourceType != "db" {
		return "", false
	}

	return id, true
}

type buildCloudwatchData func(*model.TaggedResource, *types.DBInstance, []string) (*model.CloudwatchData, error)

type supportedMetric struct {
	name                    string
	buildCloudwatchDataFunc buildCloudwatchData
	requiredPermissions     []string
}

func (sm *supportedMetric) buildCloudwatchData(resource *model.TaggedResource, instance *types.DBInstance, metrics []string) (*model.CloudwatchData, error) {
	return sm.buildCloudwatchDataFunc(resource, instance, metrics)
}

type RDS struct {
	supportedMetrics map[string]supportedMetric
	buildClientFunc  func(cfg aws.Config) Client
}

func NewRDSService(buildClientFunc func(cfg aws.Config) Client) *RDS {
	if buildClientFunc == nil {
		buildClientFunc = NewRDSClientWithConfig
	}

	rds := &RDS{
		buildClientFunc: buildClientFunc,
	}

	// The storage capacity in gibibytes (GiB) allocated for the DB instance.
	allocatedStorageMetrics := supportedMetric{
		name:                    "AllocatedStorage",
		buildCloudwatchDataFunc: buildAllocatedStorageMetric,
		requiredPermissions:     []string{"rds:DescribeDBInstances"},
	}
	rds.supportedMetrics = map[string]supportedMetric{
		allocatedStorageMetrics.name: allocatedStorageMetrics,
	}

	return rds
}

// GetNamespace returns the AWS CloudWatch namespace for RDS
func (s *RDS) GetNamespace() string {
	return awsRdsNamespace
}

// loadMetricsMetadata loads any metadata needed for RDS enhanced metrics for the given region and role.
func (s *RDS) loadMetricsMetadata(
	ctx context.Context,
	logger *slog.Logger,
	region string,
	role model.Role,
	configProvider config.RegionalConfigProvider,
	dbInstances []string,
) (map[string]*types.DBInstance, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	instances, err := client.DescribeDBInstances(ctx, logger, dbInstances)
	if err != nil {
		return nil, fmt.Errorf("error describing RDS DB instances in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.DBInstance, len(instances))
	for i := range instances {
		regionalData[*instances[i].DBInstanceArn] = &instances[i]
	}

	return regionalData, nil
}

func (s *RDS) IsMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *RDS) GetMetrics(ctx context.Context, logger *slog.Logger, resources []*model.TaggedResource, enhancedMetricConfigs []*model.EnhancedMetricConfig, exportedTagOnMetrics []string, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetricConfigs) == 0 {
		return nil, nil
	}

	// Only DB instance identifiers (not ARNs) are accepted by the DescribeDBInstances
	// "db-instance-id" filter. Filter out everything that isn't a supported DB instance up
	// front (non-instance RDS resources such as clusters and db-proxies, and resources from
	// another namespace) so their ARNs never reach the API, which would reject the request.
	dbInstances := make([]string, 0, len(resources))
	instanceResources := make([]*model.TaggedResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Namespace != s.GetNamespace() {
			logger.Warn("RDS enhanced metrics service cannot process resource with different namespace", "namespace", resource.Namespace, "arn", resource.ARN)
			continue
		}

		id, ok := dbInstanceIdentifierFromARN(resource.ARN)
		if !ok {
			logger.Warn("Skipping RDS resource: only DB instances are supported", "arn", resource.ARN)
			continue
		}

		dbInstances = append(dbInstances, id)
		instanceResources = append(instanceResources, resource)
	}

	data, err := s.loadMetricsMetadata(
		ctx,
		logger,
		region,
		role,
		regionalConfigProvider,
		dbInstances,
	)
	if err != nil {
		return nil, fmt.Errorf("error loading RDS metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, resource := range instanceResources {
		dbInstance, exists := data[resource.ARN]
		if !exists {
			logger.Warn("RDS DB instance not found in metadata", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricConfigs {
			supportedMetric, ok := s.supportedMetrics[enhancedMetric.Name]
			if !ok {
				logger.Warn("Unsupported RDS enhanced metric requested", "metric", enhancedMetric.Name)
				continue
			}

			em, err := supportedMetric.buildCloudwatchData(resource, dbInstance, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building RDS enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *RDS) ListRequiredPermissions() map[string][]string {
	requiredPermissions := make(map[string][]string, len(s.supportedMetrics))
	for metricName, metric := range s.supportedMetrics {
		requiredPermissions[metricName] = metric.requiredPermissions
	}
	return requiredPermissions
}

func (s *RDS) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *RDS) Instance() service.EnhancedMetricsService {
	// do not use NewRDSService to avoid extra map allocation
	return &RDS{
		supportedMetrics: s.supportedMetrics,
		buildClientFunc:  s.buildClientFunc,
	}
}

func buildAllocatedStorageMetric(resource *model.TaggedResource, instance *types.DBInstance, exportedTags []string) (*model.CloudwatchData, error) {
	if instance.AllocatedStorage == nil {
		return nil, fmt.Errorf("AllocatedStorage is nil for DB instance %s", resource.ARN)
	}

	var dimensions []model.Dimension

	if instance.DBInstanceIdentifier != nil && len(*instance.DBInstanceIdentifier) > 0 {
		dimensions = append(dimensions, model.Dimension{
			Name:  "DBInstanceIdentifier",
			Value: *instance.DBInstanceIdentifier,
		})
	}

	if instance.DBInstanceClass != nil && len(*instance.DBInstanceClass) > 0 {
		dimensions = append(dimensions, model.Dimension{
			Name:  "DatabaseClass",
			Value: *instance.DBInstanceClass,
		})
	}

	if instance.Engine != nil && len(*instance.Engine) > 0 {
		dimensions = append(dimensions, model.Dimension{
			Name:  "EngineName",
			Value: *instance.Engine,
		})
	}

	// Convert from GiB to bytes
	valueInBytes := float64(*instance.AllocatedStorage) * 1024 * 1024 * 1024

	return &model.CloudwatchData{
		MetricName:   "AllocatedStorage",
		ResourceName: resource.ARN,
		Namespace:    awsRdsNamespace,
		Dimensions:   dimensions,
		Tags:         resource.MetricTags(exportedTags),

		// Store the value as a single data point
		GetMetricDataResult: &model.GetMetricDataResult{
			DataPoints: []model.DataPoint{
				{
					Value:     &valueInBytes,
					Timestamp: time.Now(),
				},
			},
		},
	}, nil
}
