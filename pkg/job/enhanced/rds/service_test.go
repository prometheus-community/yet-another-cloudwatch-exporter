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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// mockRDSClient implements a mock RDS client for testing
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

func TestService_GetNamespace(t *testing.T) {
	svc := NewService(promslog.NewNopLogger())
	assert.Equal(t, "AWS/RDS", svc.GetNamespace())
}

func TestService_GetSupportedMetrics(t *testing.T) {
	svc := NewService(promslog.NewNopLogger())
	metrics := svc.GetSupportedMetrics()
	assert.Contains(t, metrics, "StorageSpace")
	assert.Contains(t, metrics, "AllocatedStorage")
}

func TestService_buildStorageSpaceMetric(t *testing.T) {
	logger := promslog.NewNopLogger()
	svc := NewService(logger)

	tests := []struct {
		name         string
		resource     *model.TaggedResource
		instance     *types.DBInstance
		exportedTags []string
		wantNil      bool
		wantValue    float64
		wantDims     int
	}{
		{
			name: "valid instance with all fields",
			resource: &model.TaggedResource{
				ARN:       "arn:aws:rds:us-east-1:123456:db:my-db",
				Namespace: "AWS/RDS",
				Region:    "us-east-1",
				Tags: []model.Tag{
					{Key: "Environment", Value: "production"},
				},
			},
			instance: &types.DBInstance{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456:db:my-db"),
				DBInstanceIdentifier: aws.String("my-db"),
				DBInstanceClass:      aws.String("db.t3.micro"),
				Engine:               aws.String("postgres"),
				AllocatedStorage:     aws.Int32(100),
			},
			exportedTags: []string{"Environment"},
			wantNil:      false,
			wantValue:    100.0,
			wantDims:     3,
		},
		{
			name: "instance with no allocated storage",
			resource: &model.TaggedResource{
				ARN:       "arn:aws:rds:us-east-1:123456:db:my-db",
				Namespace: "AWS/RDS",
				Region:    "us-east-1",
			},
			instance: &types.DBInstance{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456:db:my-db"),
				DBInstanceIdentifier: aws.String("my-db"),
				DBInstanceClass:      aws.String("db.t3.micro"),
				Engine:               aws.String("postgres"),
				AllocatedStorage:     nil,
			},
			exportedTags: []string{},
			wantNil:      true,
		},
		{
			name: "instance with missing identifier",
			resource: &model.TaggedResource{
				ARN:       "arn:aws:rds:us-east-1:123456:db:my-db",
				Namespace: "AWS/RDS",
				Region:    "us-east-1",
			},
			instance: &types.DBInstance{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456:db:my-db"),
				DBInstanceIdentifier: nil,
				DBInstanceClass:      aws.String("db.t3.micro"),
				Engine:               aws.String("postgres"),
				AllocatedStorage:     aws.Int32(100),
			},
			exportedTags: []string{},
			wantNil:      true,
		},
		{
			name: "large storage value",
			resource: &model.TaggedResource{
				ARN:       "arn:aws:rds:us-east-1:123456:db:large-db",
				Namespace: "AWS/RDS",
				Region:    "us-east-1",
			},
			instance: &types.DBInstance{
				DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456:db:large-db"),
				DBInstanceIdentifier: aws.String("large-db"),
				DBInstanceClass:      aws.String("db.r5.xlarge"),
				Engine:               aws.String("mysql"),
				AllocatedStorage:     aws.Int32(16384),
			},
			exportedTags: []string{},
			wantNil:      false,
			wantValue:    16384.0,
			wantDims:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.buildStorageSpaceMetric(tt.resource, tt.instance, tt.exportedTags)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, "StorageSpace", got.MetricName)
				assert.Equal(t, tt.resource.ARN, got.ResourceName)
				assert.Equal(t, "AWS/RDS", got.Namespace)
				assert.Len(t, got.Dimensions, tt.wantDims)

				// Check the metric value
				assert.NotNil(t, got.GetMetricDataResult)
				assert.Len(t, got.GetMetricDataResult.DataPoints, 1)
				assert.NotNil(t, got.GetMetricDataResult.DataPoints[0].Value)
				assert.Equal(t, tt.wantValue, *got.GetMetricDataResult.DataPoints[0].Value)

				// Check dimensions
				dimNames := make(map[string]string)
				for _, dim := range got.Dimensions {
					dimNames[dim.Name] = dim.Value
				}
				assert.Contains(t, dimNames, "DBInstanceIdentifier")
				assert.Contains(t, dimNames, "DatabaseClass")
				assert.Contains(t, dimNames, "EngineName")
			}
		})
	}
}

func Test_groupResourcesByRegion(t *testing.T) {
	tests := []struct {
		name      string
		resources []*model.TaggedResource
		wantCount int
	}{
		{
			name:      "empty resources",
			resources: []*model.TaggedResource{},
			wantCount: 0,
		},
		{
			name: "single region",
			resources: []*model.TaggedResource{
				{ARN: "arn1", Region: "us-east-1"},
				{ARN: "arn2", Region: "us-east-1"},
			},
			wantCount: 1,
		},
		{
			name: "multiple regions",
			resources: []*model.TaggedResource{
				{ARN: "arn1", Region: "us-east-1"},
				{ARN: "arn2", Region: "us-west-2"},
				{ARN: "arn3", Region: "eu-west-1"},
				{ARN: "arn4", Region: "us-east-1"},
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupResourcesByRegion(tt.resources)
			assert.Len(t, got, tt.wantCount)

			// Verify all resources are in correct groups
			for _, r := range tt.resources {
				assert.Contains(t, got, r.Region)
				found := false
				for _, grouped := range got[r.Region] {
					if grouped.ARN == r.ARN {
						found = true
						break
					}
				}
				assert.True(t, found, "Resource %s not found in region %s", r.ARN, r.Region)
			}
		})
	}
}

func TestService_buildMetric(t *testing.T) {
	logger := promslog.NewNopLogger()
	svc := NewService(logger)

	resource := &model.TaggedResource{
		ARN:       "arn:aws:rds:us-east-1:123456:db:my-db",
		Namespace: "AWS/RDS",
		Region:    "us-east-1",
	}

	instance := &types.DBInstance{
		DBInstanceArn:        aws.String("arn:aws:rds:us-east-1:123456:db:my-db"),
		DBInstanceIdentifier: aws.String("my-db"),
		DBInstanceClass:      aws.String("db.t3.micro"),
		Engine:               aws.String("postgres"),
		AllocatedStorage:     aws.Int32(100),
	}

	tests := []struct {
		name       string
		metricName string
		wantNil    bool
	}{
		{
			name:       "StorageSpace metric",
			metricName: "StorageSpace",
			wantNil:    false,
		},
		{
			name:       "AllocatedStorage metric (alias)",
			metricName: "AllocatedStorage",
			wantNil:    false,
		},
		{
			name:       "unsupported metric",
			metricName: "UnsupportedMetric",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.buildMetric(resource, instance, tt.metricName, []string{})
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
			}
		})
	}
}
