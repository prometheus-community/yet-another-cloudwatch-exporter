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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

const testRegion = "eu-west-1"

func TestNewEBSService(t *testing.T) {
	for _, buildClientFunc := range []func(aws.Config) Client{nil, func(aws.Config) Client { return nil }} {
		got := NewEBSService(buildClientFunc)
		require.NotNil(t, got)
		require.NotNil(t, got.buildClientFunc)
		require.Len(t, got.supportedMetrics, 3)
	}
}

func TestEBS_GetNamespace(t *testing.T) {
	require.Equal(t, "AWS/EBS", NewEBSService(nil).GetNamespace())
}

func TestEBS_ListRequiredPermissions(t *testing.T) {
	want := map[string][]string{
		"Iops":       {"ec2:DescribeVolumes"},
		"Throughput": {"ec2:DescribeVolumes"},
		"Size":       {"ec2:DescribeVolumes"},
	}
	require.Equal(t, want, NewEBSService(nil).ListRequiredPermissions())
}

func TestEBS_ListSupportedEnhancedMetrics(t *testing.T) {
	require.ElementsMatch(t, []string{"Iops", "Throughput", "Size"}, NewEBSService(nil).ListSupportedEnhancedMetrics())
}

func TestVolumeIDFromARN(t *testing.T) {
	tests := []struct {
		name   string
		arn    string
		wantID string
		wantOK bool
	}{
		{"volume", "arn:aws:ec2:eu-west-1:123456789012:volume/vol-0123", "vol-0123", true},
		{"snapshot", "arn:aws:ec2:eu-west-1::snapshot/snap-0123", "", false},
		{"instance", "arn:aws:ec2:eu-west-1:123456789012:instance/i-0123", "", false},
		{"non-ec2 service", "arn:aws:rds:eu-west-1:123456789012:volume/vol-0123", "", false},
		{"not an arn", "not-an-arn", "", false},
		{"empty id", "arn:aws:ec2:eu-west-1:123456789012:volume/", "", false},
		{"extra segments", "arn:aws:ec2:eu-west-1:123456789012:volume/vol-0123/x", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := volumeIDFromARN(tt.arn)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantID, id)
		})
	}
}

func TestEBS_GetMetrics(t *testing.T) {
	gp3 := makeTestVolume("vol-gp3", types.VolumeTypeGp3, 100, aws.Int32(6000), aws.Int32(500))
	gp2 := makeTestVolume("vol-gp2", types.VolumeTypeGp2, 200, aws.Int32(600), nil)
	st1 := makeTestVolume("vol-st1", types.VolumeTypeSt1, 1000, nil, nil)

	type wantMetric struct {
		name       string
		volumeID   string
		volumeType string
		value      float64
	}

	tests := []struct {
		name      string
		resources []*model.TaggedResource
		metrics   []string
		volumes   []types.Volume
		wantIDs   []string
		want      []wantMetric
		wantErr   bool
	}{
		{
			name:    "empty resources",
			metrics: []string{"Iops"},
			volumes: []types.Volume{gp3},
		},
		{
			name:      "empty enhanced metrics",
			resources: []*model.TaggedResource{testResource("vol-gp3")},
			volumes:   []types.Volume{gp3},
		},
		{
			name:      "gp3 reports all metrics",
			resources: []*model.TaggedResource{testResource("vol-gp3")},
			metrics:   []string{"Iops", "Throughput", "Size"},
			volumes:   []types.Volume{gp3},
			wantIDs:   []string{"vol-gp3"},
			want: []wantMetric{
				{"Iops", "vol-gp3", "gp3", 6000},
				{"Throughput", "vol-gp3", "gp3", 500 * 1024 * 1024},
				{"Size", "vol-gp3", "gp3", 100 * 1024 * 1024 * 1024},
			},
		},
		{
			name:      "metrics a volume type lacks are skipped",
			resources: []*model.TaggedResource{testResource("vol-gp2"), testResource("vol-st1")},
			metrics:   []string{"Iops", "Throughput", "Size"},
			volumes:   []types.Volume{gp2, st1},
			wantIDs:   []string{"vol-gp2", "vol-st1"},
			want: []wantMetric{
				{"Iops", "vol-gp2", "gp2", 600},
				{"Size", "vol-gp2", "gp2", 200 * 1024 * 1024 * 1024},
				{"Size", "vol-st1", "st1", 1000 * 1024 * 1024 * 1024},
			},
		},
		{
			name: "other resources never reach the filter",
			resources: []*model.TaggedResource{
				testResource("vol-gp3"),
				{ARN: "arn:aws:ec2:eu-west-1::snapshot/snap-0123", Namespace: awsEbsNamespace},
				{ARN: "arn:aws:ec2:eu-west-1:123456789012:volume/vol-other", Namespace: "AWS/EC2"},
			},
			metrics: []string{"Size"},
			volumes: []types.Volume{gp3},
			wantIDs: []string{"vol-gp3"},
			want:    []wantMetric{{"Size", "vol-gp3", "gp3", 100 * 1024 * 1024 * 1024}},
		},
		{
			name:      "volume missing from DescribeVolumes",
			resources: []*model.TaggedResource{testResource("vol-gone"), testResource("vol-gp3")},
			metrics:   []string{"Size"},
			volumes:   []types.Volume{gp3},
			wantIDs:   []string{"vol-gone", "vol-gp3"},
			want:      []wantMetric{{"Size", "vol-gp3", "gp3", 100 * 1024 * 1024 * 1024}},
		},
		{
			name:      "unsupported metric",
			resources: []*model.TaggedResource{testResource("vol-gp3")},
			metrics:   []string{"Unsupported"},
			volumes:   []types.Volume{gp3},
			wantIDs:   []string{"vol-gp3"},
		},
		{
			name:      "describe error",
			resources: []*model.TaggedResource{testResource("vol-gp3")},
			metrics:   []string{"Size"},
			wantIDs:   []string{"vol-gp3"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockServiceEBSClient{volumes: tt.volumes, describeErr: tt.wantErr}
			service := NewEBSService(func(aws.Config) Client { return mock })

			configs := make([]*model.EnhancedMetricConfig, 0, len(tt.metrics))
			for _, name := range tt.metrics {
				configs = append(configs, &model.EnhancedMetricConfig{Name: name})
			}

			result, err := service.GetMetrics(context.Background(), slog.New(slog.DiscardHandler), tt.resources, configs, nil, testRegion, model.Role{}, &mockConfigProvider{c: &aws.Config{Region: testRegion}})
			require.Equal(t, tt.wantIDs, mock.volumeIDs)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			got := make([]wantMetric, 0, len(result))
			for _, metric := range result {
				require.Equal(t, awsEbsNamespace, metric.Namespace)
				require.Nil(t, metric.GetMetricStatisticsResult)
				require.Len(t, metric.GetMetricDataResult.DataPoints, 1)
				require.Len(t, metric.Dimensions, 2)
				require.Equal(t, "VolumeId", metric.Dimensions[0].Name)
				require.Equal(t, "VolumeType", metric.Dimensions[1].Name)
				got = append(got, wantMetric{
					name:       metric.MetricName,
					volumeID:   metric.Dimensions[0].Value,
					volumeType: metric.Dimensions[1].Value,
					value:      *metric.GetMetricDataResult.DataPoints[0].Value,
				})
			}
			require.ElementsMatch(t, tt.want, got)
		})
	}
}

type mockServiceEBSClient struct {
	volumes     []types.Volume
	describeErr bool

	// volumeIDs captures the filter input, for assertions.
	volumeIDs []string
}

func (m *mockServiceEBSClient) DescribeVolumes(_ context.Context, _ *slog.Logger, ids []string) ([]types.Volume, error) {
	m.volumeIDs = ids
	if m.describeErr {
		return nil, fmt.Errorf("mock describe error")
	}
	return m.volumes, nil
}

type mockConfigProvider struct {
	c *aws.Config
}

func (m *mockConfigProvider) GetAWSRegionalConfig(_ string, _ model.Role) *aws.Config {
	return m.c
}

func testResource(volumeID string) *model.TaggedResource {
	return &model.TaggedResource{
		ARN:       fmt.Sprintf("arn:aws:ec2:%s:123456789012:volume/%s", testRegion, volumeID),
		Namespace: awsEbsNamespace,
	}
}

func makeTestVolume(id string, volumeType types.VolumeType, sizeGiB int32, iops, throughput *int32) types.Volume {
	return types.Volume{
		VolumeId:   aws.String(id),
		VolumeType: volumeType,
		Size:       aws.Int32(sizeGiB),
		Iops:       iops,
		Throughput: throughput,
	}
}
