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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Client interface {
	DescribeAllDBInstances(ctx context.Context, logger *slog.Logger) ([]types.DBInstance, error)
}

type buildRDSMetricFunc func(context.Context, *slog.Logger, *model.TaggedResource, *types.DBInstance, []string) (*model.CloudwatchData, error)

type RDS struct {
	clients *clients.Clients[Client]

	regionalData map[string]*types.DBInstance

	// dataM protects access to regionalData, for the concurrent metric processing
	dataM sync.RWMutex

	supportedMetrics map[string]buildRDSMetricFunc
	buildClientFunc  func(cfg aws.Config) Client
}

func NewRDSService(buildClientFunc func(cfg aws.Config) Client) *RDS {
	if buildClientFunc == nil {
		buildClientFunc = NewRDSClientWithConfig
	}

	rds := &RDS{
		clients:         clients.NewClients[Client](buildClientFunc),
		buildClientFunc: buildClientFunc,
	}

	rds.supportedMetrics = map[string]buildRDSMetricFunc{
		"AllocatedStorage": rds.buildAllocatedStorageMetric,
	}

	return rds
}

// GetNamespace returns the AWS CloudWatch namespace for RDS
func (s *RDS) GetNamespace() string {
	return "AWS/RDS"
}

// LoadMetricsMetadata loads any metadata needed for RDS enhanced metrics for the given region and role
func (s *RDS) LoadMetricsMetadata(ctx context.Context, logger *slog.Logger, region string, role model.Role, configProvider config.RegionalConfigProvider) error {
	var err error
	client := s.clients.GetClient(region, role)
	if client == nil {
		client, err = s.clients.InitializeClient(region, role, configProvider)
		if err != nil {
			return fmt.Errorf("error initializing RDS client for region %s: %w", region, err)
		}
	}

	s.dataM.Lock()
	defer s.dataM.Unlock()

	if s.regionalData != nil {
		return nil
	}

	s.regionalData = make(map[string]*types.DBInstance)

	instances, err := client.DescribeAllDBInstances(ctx, logger)
	if err != nil {
		return fmt.Errorf("error describing RDS DB instances in region %s: %w", region, err)
	}

	for _, instance := range instances {
		s.regionalData[*instance.DBInstanceArn] = &instance
	}

	return nil
}

func (s *RDS) isMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *RDS) Process(ctx context.Context, logger *slog.Logger, namespace string, resources []*model.TaggedResource, enhancedMetrics []*model.EnhancedMetricConfig, exportedTagOnMetrics []string) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetrics) == 0 {
		return nil, nil
	}

	if namespace != s.GetNamespace() {
		return nil, fmt.Errorf("RDS enhanced metrics service cannot process namespace %s", namespace)
	}

	// filter only supported enhanced metrics
	var enhancedMetricsFiltered []*model.EnhancedMetricConfig
	for _, em := range enhancedMetrics {
		if s.isMetricSupported(em.Name) {
			enhancedMetricsFiltered = append(enhancedMetricsFiltered, em)
		} else {
			logger.Warn("enhanced metric not supported, skipping", "metric", em.Name)
		}
	}

	var result []*model.CloudwatchData
	s.dataM.RLock()
	defer s.dataM.RUnlock()

	if s.regionalData == nil {
		logger.Info("RDS metadata not loaded, skipping metric processing")
		return nil, nil
	}

	for _, resource := range resources {
		dbInstance, exists := s.regionalData[resource.ARN]
		if !exists {
			logger.Warn("RDS DB instance not found in metadata", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricsFiltered {
			em, err := s.supportedMetrics[enhancedMetric.Name](ctx, logger, resource, dbInstance, exportedTagOnMetrics)
			if err != nil || em == nil {
				logger.Warn("Error building RDS enhanced metric", "metric", enhancedMetric.Name, "error", err)
				continue
			}

			result = append(result, em)
		}
	}

	return result, nil
}

func (s *RDS) buildAllocatedStorageMetric(_ context.Context, _ *slog.Logger, resource *model.TaggedResource, instance *types.DBInstance, exportedTags []string) (*model.CloudwatchData, error) {
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

	valueInGiB := float64(*instance.AllocatedStorage)

	return &model.CloudwatchData{
		MetricName:   "StorageCapacity",
		ResourceName: resource.ARN,
		Namespace:    "AWS/RDS",
		Dimensions:   dimensions,
		Tags:         resource.MetricTags(exportedTags),

		// Store the value as a single data point
		GetMetricDataResult: &model.GetMetricDataResult{
			DataPoints: []model.DataPoint{
				{
					Value:     &valueInGiB,
					Timestamp: time.Now(),
				},
			},
		},
	}, nil
}

func (s *RDS) ListRequiredPermissions() []string {
	return []string{
		"rds:DescribeDBInstances",
	}
}

func (s *RDS) ListSupportedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *RDS) Instance() service.MetricsService {
	return NewRDSService(s.buildClientFunc)
}
