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
package enhanced

import (
	"context"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// mockService implements the Service interface for testing
type mockService struct {
	namespace        string
	supportedMetrics []string
	fetchFunc        func(ctx context.Context, resources []*model.TaggedResource, requestedMetrics []string, exportedTags []string) ([]*model.CloudwatchData, error)
}

func (m *mockService) GetNamespace() string {
	return m.namespace
}

func (m *mockService) GetSupportedMetrics() []string {
	return m.supportedMetrics
}

func (m *mockService) FetchEnhancedMetrics(ctx context.Context, resources []*model.TaggedResource, requestedMetrics []string, exportedTags []string) ([]*model.CloudwatchData, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx, resources, requestedMetrics, exportedTags)
	}
	return nil, nil
}

func TestProcessor_Process(t *testing.T) {
	ctx := context.Background()
	logger := promslog.NewNopLogger()

	tests := []struct {
		name             string
		namespace        string
		resources        []*model.TaggedResource
		requestedMetrics []string
		exportedTags     []string
		setupServices    func(*Processor)
		wantMetrics      int
		wantErr          bool
	}{
		{
			name:      "no service registered for namespace",
			namespace: "AWS/UnknownService",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/UnknownService", ARN: "arn:aws:unknown:us-east-1:123:resource/test"},
			},
			requestedMetrics: []string{"SomeMetric"},
			setupServices:    func(p *Processor) {},
			wantMetrics:      0,
			wantErr:          false,
		},
		{
			name:      "no resources in namespace",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/EC2", ARN: "arn:aws:ec2:us-east-1:123:instance/i-123"},
			},
			requestedMetrics: []string{"StorageSpace"},
			setupServices: func(p *Processor) {
				p.RegisterService(&mockService{
					namespace:        "AWS/RDS",
					supportedMetrics: []string{"StorageSpace"},
				})
			},
			wantMetrics: 0,
			wantErr:     false,
		},
		{
			name:      "no supported metrics requested",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/RDS", ARN: "arn:aws:rds:us-east-1:123:db:my-db"},
			},
			requestedMetrics: []string{"UnsupportedMetric"},
			setupServices: func(p *Processor) {
				p.RegisterService(&mockService{
					namespace:        "AWS/RDS",
					supportedMetrics: []string{"StorageSpace"},
				})
			},
			wantMetrics: 0,
			wantErr:     false,
		},
		{
			name:      "successful fetch with matching resources",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/RDS", ARN: "arn:aws:rds:us-east-1:123:db:my-db", Region: "us-east-1"},
				{Namespace: "AWS/RDS", ARN: "arn:aws:rds:us-east-1:123:db:my-db-2", Region: "us-east-1"},
			},
			requestedMetrics: []string{"StorageSpace"},
			exportedTags:     []string{"Environment"},
			setupServices: func(p *Processor) {
				p.RegisterService(&mockService{
					namespace:        "AWS/RDS",
					supportedMetrics: []string{"StorageSpace", "AllocatedStorage"},
					fetchFunc: func(ctx context.Context, resources []*model.TaggedResource, requestedMetrics []string, exportedTags []string) ([]*model.CloudwatchData, error) {
						// Return one metric per resource
						metrics := make([]*model.CloudwatchData, len(resources))
						for i, r := range resources {
							metrics[i] = &model.CloudwatchData{
								MetricName:   "StorageSpace",
								ResourceName: r.ARN,
								Namespace:    "AWS/RDS",
							}
						}
						return metrics, nil
					},
				})
			},
			wantMetrics: 2,
			wantErr:     false,
		},
		{
			name:      "multiple requested metrics intersection",
			namespace: "AWS/RDS",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/RDS", ARN: "arn:aws:rds:us-east-1:123:db:my-db", Region: "us-east-1"},
			},
			requestedMetrics: []string{"StorageSpace", "AllocatedStorage", "UnsupportedMetric"},
			setupServices: func(p *Processor) {
				p.RegisterService(&mockService{
					namespace:        "AWS/RDS",
					supportedMetrics: []string{"StorageSpace", "AllocatedStorage"},
					fetchFunc: func(ctx context.Context, resources []*model.TaggedResource, requestedMetrics []string, exportedTags []string) ([]*model.CloudwatchData, error) {
						// Should only receive supported metrics
						assert.ElementsMatch(t, []string{"StorageSpace", "AllocatedStorage"}, requestedMetrics)
						return []*model.CloudwatchData{
							{MetricName: "StorageSpace", ResourceName: resources[0].ARN, Namespace: "AWS/RDS"},
						}, nil
					},
				})
			},
			wantMetrics: 1,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessor(logger)
			tt.setupServices(p)

			got, err := p.Process(ctx, tt.namespace, tt.resources, tt.requestedMetrics, tt.exportedTags)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.wantMetrics >= 0 {
				assert.Len(t, got, tt.wantMetrics)
			}
		})
	}
}

func Test_filterResourcesByNamespace(t *testing.T) {
	tests := []struct {
		name      string
		resources []*model.TaggedResource
		namespace string
		want      int
	}{
		{
			name:      "no resources",
			resources: []*model.TaggedResource{},
			namespace: "AWS/RDS",
			want:      0,
		},
		{
			name: "all match",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/RDS", ARN: "arn1"},
				{Namespace: "AWS/RDS", ARN: "arn2"},
			},
			namespace: "AWS/RDS",
			want:      2,
		},
		{
			name: "none match",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/EC2", ARN: "arn1"},
				{Namespace: "AWS/Lambda", ARN: "arn2"},
			},
			namespace: "AWS/RDS",
			want:      0,
		},
		{
			name: "some match",
			resources: []*model.TaggedResource{
				{Namespace: "AWS/RDS", ARN: "arn1"},
				{Namespace: "AWS/EC2", ARN: "arn2"},
				{Namespace: "AWS/RDS", ARN: "arn3"},
				{Namespace: "AWS/Lambda", ARN: "arn4"},
			},
			namespace: "AWS/RDS",
			want:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterResourcesByNamespace(tt.resources, tt.namespace)
			assert.Len(t, got, tt.want)
			// Verify all returned resources match the namespace
			for _, r := range got {
				assert.Equal(t, tt.namespace, r.Namespace)
			}
		})
	}
}

func Test_intersect(t *testing.T) {
	tests := []struct {
		name      string
		requested []string
		supported []string
		want      []string
	}{
		{
			name:      "empty requested",
			requested: []string{},
			supported: []string{"StorageSpace", "MemorySize"},
			want:      []string{},
		},
		{
			name:      "empty supported",
			requested: []string{"StorageSpace", "MemorySize"},
			supported: []string{},
			want:      []string{},
		},
		{
			name:      "all match",
			requested: []string{"StorageSpace", "MemorySize"},
			supported: []string{"StorageSpace", "MemorySize"},
			want:      []string{"StorageSpace", "MemorySize"},
		},
		{
			name:      "no match",
			requested: []string{"Metric1", "Metric2"},
			supported: []string{"Metric3", "Metric4"},
			want:      []string{},
		},
		{
			name:      "partial match",
			requested: []string{"StorageSpace", "MemorySize", "ItemCount"},
			supported: []string{"StorageSpace", "NodeCount"},
			want:      []string{"StorageSpace"},
		},
		{
			name:      "requested subset of supported",
			requested: []string{"StorageSpace"},
			supported: []string{"StorageSpace", "MemorySize", "ItemCount"},
			want:      []string{"StorageSpace"},
		},
		{
			name:      "supported subset of requested",
			requested: []string{"StorageSpace", "MemorySize", "ItemCount"},
			supported: []string{"StorageSpace"},
			want:      []string{"StorageSpace"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersect(tt.requested, tt.supported)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
