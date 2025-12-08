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
package rds

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/rds/types"

	rdsclient "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/enhanced/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// Service provides enhanced metrics for AWS RDS
type Service struct {
	logger  *slog.Logger
	clients map[string]rdsclient.ClientInterface // region -> client

	// Cache: region -> ARN -> DBInstance
	cache struct {
		sync.RWMutex
		data map[string]map[string]*types.DBInstance
	}
}

// NewService creates a new RDS enhanced metrics service
func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger:  logger,
		clients: make(map[string]rdsclient.ClientInterface),
	}
}

// RegisterClient registers an RDS client for a specific region
func (s *Service) RegisterClient(region string, client rdsclient.ClientInterface) {
	s.clients[region] = client
}

// GetNamespace returns the AWS namespace
func (s *Service) GetNamespace() string {
	return "AWS/RDS"
}

// GetSupportedMetrics returns list of supported enhanced metric names
func (s *Service) GetSupportedMetrics() []string {
	return []string{"StorageSpace", "AllocatedStorage"}
}

// FetchEnhancedMetrics fetches AWS API data and builds enhanced metrics
func (s *Service) FetchEnhancedMetrics(
	ctx context.Context,
	resources []*model.TaggedResource,
	requestedMetrics []string,
	exportedTags []string,
) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 {
		return nil, nil
	}

	// Group resources by region for efficient API calls
	resourcesByRegion := groupResourcesByRegion(resources)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allEnhancedData := make([]*model.CloudwatchData, 0, len(resources)*len(requestedMetrics))

	for region, regionResources := range resourcesByRegion {
		wg.Add(1)
		go func(region string, regionResources []*model.TaggedResource) {
			defer wg.Done()

			// Fetch DB instances for this region (with caching)
			instances, err := s.fetchDBInstancesForRegion(ctx, region)
			if err != nil {
				s.logger.Error("Failed to fetch RDS instances", "region", region, "err", err)
				return
			}

			// Build enhanced metrics for each resource
			for _, resource := range regionResources {
				instance, ok := instances[resource.ARN]
				if !ok {
					s.logger.Debug("No DB instance found for resource", "arn", resource.ARN)
					continue
				}

				// Build metrics from the instance data
				for _, metricName := range requestedMetrics {
					metricData := s.buildMetric(resource, instance, metricName, exportedTags)
					if metricData != nil {
						mu.Lock()
						allEnhancedData = append(allEnhancedData, metricData)
						mu.Unlock()
					}
				}
			}
		}(region, regionResources)
	}

	wg.Wait()
	return allEnhancedData, nil
}

func (s *Service) fetchDBInstancesForRegion(
	ctx context.Context,
	region string,
) (map[string]*types.DBInstance, error) {
	s.cache.RLock()
	cached := s.cache.data[region]
	s.cache.RUnlock()

	if cached != nil {
		s.logger.Debug("Using cached RDS instances", "region", region, "count", len(cached))
		return cached, nil
	}

	// Fetch all DB instances for this region
	client := s.clients[region]
	if client == nil {
		return nil, fmt.Errorf("no RDS client for region: %s", region)
	}

	s.logger.Debug("Fetching RDS instances from API", "region", region)
	instances, err := client.DescribeAllDBInstances(ctx)
	if err != nil {
		return nil, err
	}

	// Build ARN -> Instance map
	instanceMap := make(map[string]*types.DBInstance, len(instances))
	for i := range instances {
		inst := &instances[i]
		if inst.DBInstanceArn != nil {
			instanceMap[*inst.DBInstanceArn] = inst
		}
	}

	// Cache the results
	s.cache.Lock()
	if s.cache.data == nil {
		s.cache.data = make(map[string]map[string]*types.DBInstance)
	}
	s.cache.data[region] = instanceMap
	s.cache.Unlock()

	s.logger.Debug("Cached RDS instances", "region", region, "count", len(instanceMap))
	return instanceMap, nil
}

func (s *Service) buildMetric(
	resource *model.TaggedResource,
	instance *types.DBInstance,
	metricName string,
	exportedTags []string,
) *model.CloudwatchData {
	switch metricName {
	case "StorageSpace", "AllocatedStorage":
		return s.buildStorageSpaceMetric(resource, instance, exportedTags)
	default:
		return nil
	}
}

func (s *Service) buildStorageSpaceMetric(
	resource *model.TaggedResource,
	instance *types.DBInstance,
	exportedTags []string,
) *model.CloudwatchData {
	if instance.AllocatedStorage == nil {
		return nil
	}

	if instance.DBInstanceIdentifier == nil || instance.DBInstanceClass == nil || instance.Engine == nil {
		s.logger.Warn("Missing required fields on DB instance", "arn", resource.ARN)
		return nil
	}

	// Build dimensions from instance attributes
	dimensions := []model.Dimension{
		{Name: "DBInstanceIdentifier", Value: *instance.DBInstanceIdentifier},
		{Name: "DatabaseClass", Value: *instance.DBInstanceClass},
		{Name: "EngineName", Value: *instance.Engine},
	}

	// Get tags to export (uses existing TaggedResource.MetricTags method!)
	tags := resource.MetricTags(exportedTags)

	// Convert AllocatedStorage (int32) to float64 for Prometheus
	value := float64(*instance.AllocatedStorage)

	return &model.CloudwatchData{
		MetricName:   "StorageSpace",
		ResourceName: resource.ARN,
		Namespace:    "AWS/RDS",
		Dimensions:   dimensions,
		Tags:         tags,

		// Enhanced metrics are instant values, not time series
		MetricMigrationParams: model.MetricMigrationParams{
			NilToZero:              false,
			AddCloudwatchTimestamp: false,
			ExportAllDataPoints:    false,
		},

		// Store the value as a single data point
		GetMetricDataResult: &model.GetMetricDataResult{
			Statistic: "Value",
			DataPoints: []model.DataPoint{
				{
					Value:     &value,
					Timestamp: time.Now(),
				},
			},
		},
	}
}

func groupResourcesByRegion(resources []*model.TaggedResource) map[string][]*model.TaggedResource {
	grouped := make(map[string][]*model.TaggedResource)
	for _, r := range resources {
		grouped[r.Region] = append(grouped[r.Region], r)
	}
	return grouped
}
