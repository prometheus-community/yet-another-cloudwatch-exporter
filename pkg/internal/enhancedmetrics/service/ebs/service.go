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
package ebs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/enhancedmetrics/service"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const (
	awsEbsNamespace = "AWS/EBS"

	bytesPerMebibyte = 1024 * 1024
	bytesPerGibibyte = 1024 * 1024 * 1024
)

type Client interface {
	DescribeVolumes(ctx context.Context, logger *slog.Logger, volumeIDs []string) ([]types.Volume, error)
}

// volumeIDFromARN extracts the volume ID from an EBS volume ARN, e.g.
//
//	arn:aws:ec2:eu-west-1:123456789012:volume/vol-0123456789abcdef0 -> ("vol-0123456789abcdef0", true)
//
// It returns ok=false for anything else, so no other value reaches the "volume-id" filter.
func volumeIDFromARN(resourceARN string) (string, bool) {
	parsed, err := arn.Parse(resourceARN)
	if err != nil || parsed.Service != "ec2" {
		return "", false
	}

	resourceType, id, found := strings.Cut(parsed.Resource, "/")
	if !found || resourceType != "volume" || id == "" || strings.Contains(id, "/") {
		return "", false
	}

	return id, true
}

// volumeValue returns the metric's value for the volume, and false when the volume type has none.
type volumeValue func(*types.Volume) (float64, bool)

type supportedMetric struct {
	name                string
	value               volumeValue
	requiredPermissions []string
}

type EBS struct {
	supportedMetrics map[string]supportedMetric
	buildClientFunc  func(cfg aws.Config) Client
}

func NewEBSService(buildClientFunc func(cfg aws.Config) Client) *EBS {
	if buildClientFunc == nil {
		buildClientFunc = NewEBSClientWithConfig
	}

	permissions := []string{"ec2:DescribeVolumes"}
	metrics := []supportedMetric{
		// The provisioned IOPS: the baseline for gp2, unset for st1, sc1 and standard.
		{name: "Iops", value: iops, requiredPermissions: permissions},
		// The provisioned throughput in bytes per second, set only for gp3.
		{name: "Throughput", value: throughput, requiredPermissions: permissions},
		// The volume size in bytes.
		{name: "Size", value: size, requiredPermissions: permissions},
	}

	ebs := &EBS{
		buildClientFunc:  buildClientFunc,
		supportedMetrics: make(map[string]supportedMetric, len(metrics)),
	}
	for _, m := range metrics {
		ebs.supportedMetrics[m.name] = m
	}

	return ebs
}

// GetNamespace returns the AWS CloudWatch namespace for EBS
func (s *EBS) GetNamespace() string {
	return awsEbsNamespace
}

func (s *EBS) loadMetricsMetadata(
	ctx context.Context,
	logger *slog.Logger,
	region string,
	role model.Role,
	configProvider config.RegionalConfigProvider,
	volumeIDs []string,
) (map[string]*types.Volume, error) {
	client := s.buildClientFunc(*configProvider.GetAWSRegionalConfig(region, role))

	volumes, err := client.DescribeVolumes(ctx, logger, volumeIDs)
	if err != nil {
		return nil, fmt.Errorf("error describing EBS volumes in region %s: %w", region, err)
	}

	regionalData := make(map[string]*types.Volume, len(volumes))
	for i := range volumes {
		regionalData[aws.ToString(volumes[i].VolumeId)] = &volumes[i]
	}

	return regionalData, nil
}

func (s *EBS) IsMetricSupported(metricName string) bool {
	_, exists := s.supportedMetrics[metricName]
	return exists
}

func (s *EBS) GetMetrics(ctx context.Context, logger *slog.Logger, resources []*model.TaggedResource, enhancedMetricConfigs []*model.EnhancedMetricConfig, exportedTagOnMetrics []string, region string, role model.Role, regionalConfigProvider config.RegionalConfigProvider) ([]*model.CloudwatchData, error) {
	if len(resources) == 0 || len(enhancedMetricConfigs) == 0 {
		return nil, nil
	}

	volumeIDs := make([]string, 0, len(resources))
	volumeResources := make(map[string]*model.TaggedResource, len(resources))
	for _, resource := range resources {
		if resource.Namespace != s.GetNamespace() {
			logger.Warn("EBS enhanced metrics service cannot process resource with different namespace", "namespace", resource.Namespace, "arn", resource.ARN)
			continue
		}

		id, ok := volumeIDFromARN(resource.ARN)
		if !ok {
			logger.Warn("Skipping EBS resource: only volumes are supported", "arn", resource.ARN)
			continue
		}

		volumeIDs = append(volumeIDs, id)
		volumeResources[id] = resource
	}

	if len(volumeIDs) == 0 {
		return nil, nil
	}

	data, err := s.loadMetricsMetadata(ctx, logger, region, role, regionalConfigProvider, volumeIDs)
	if err != nil {
		return nil, fmt.Errorf("error loading EBS metrics metadata: %w", err)
	}

	var result []*model.CloudwatchData

	for _, id := range volumeIDs {
		resource := volumeResources[id]
		volume, exists := data[id]
		if !exists {
			// The tagging API keeps returning a volume for a while after it's deleted, so this is
			// routine wherever volumes come and go.
			logger.Debug("EBS volume not found in metadata", "arn", resource.ARN)
			continue
		}

		for _, enhancedMetric := range enhancedMetricConfigs {
			supportedMetric, ok := s.supportedMetrics[enhancedMetric.Name]
			if !ok {
				logger.Warn("Unsupported EBS enhanced metric requested", "metric", enhancedMetric.Name)
				continue
			}

			value, ok := supportedMetric.value(volume)
			if !ok {
				logger.Debug("EBS volume has no value for metric", "metric", supportedMetric.name, "arn", resource.ARN, "volumeType", volume.VolumeType)
				continue
			}

			result = append(result, buildCloudwatchData(supportedMetric.name, value, resource, volume, exportedTagOnMetrics))
		}
	}

	return result, nil
}

func (s *EBS) ListRequiredPermissions() map[string][]string {
	requiredPermissions := make(map[string][]string, len(s.supportedMetrics))
	for metricName, metric := range s.supportedMetrics {
		requiredPermissions[metricName] = metric.requiredPermissions
	}
	return requiredPermissions
}

func (s *EBS) ListSupportedEnhancedMetrics() []string {
	var metrics []string
	for metric := range s.supportedMetrics {
		metrics = append(metrics, metric)
	}
	return metrics
}

func (s *EBS) Instance() service.EnhancedMetricsService {
	// do not use NewEBSService to avoid extra map allocation
	return &EBS{
		supportedMetrics: s.supportedMetrics,
		buildClientFunc:  s.buildClientFunc,
	}
}

func iops(volume *types.Volume) (float64, bool) {
	if volume.Iops == nil {
		return 0, false
	}
	return float64(*volume.Iops), true
}

func throughput(volume *types.Volume) (float64, bool) {
	if volume.Throughput == nil {
		return 0, false
	}
	return float64(*volume.Throughput) * bytesPerMebibyte, true
}

func size(volume *types.Volume) (float64, bool) {
	if volume.Size == nil {
		return 0, false
	}
	return float64(*volume.Size) * bytesPerGibibyte, true
}

func buildCloudwatchData(metricName string, value float64, resource *model.TaggedResource, volume *types.Volume, exportedTags []string) *model.CloudwatchData {
	dimensions := []model.Dimension{
		{Name: "VolumeId", Value: aws.ToString(volume.VolumeId)},
	}
	if volume.VolumeType != "" {
		dimensions = append(dimensions, model.Dimension{Name: "VolumeType", Value: string(volume.VolumeType)})
	}

	return &model.CloudwatchData{
		MetricName:   metricName,
		ResourceName: resource.ARN,
		Namespace:    awsEbsNamespace,
		Dimensions:   dimensions,
		Tags:         resource.MetricTags(exportedTags),
		GetMetricDataResult: &model.GetMetricDataResult{
			DataPoints: []model.DataPoint{
				{
					Value:     &value,
					Timestamp: time.Now(),
				},
			},
		},
	}
}
