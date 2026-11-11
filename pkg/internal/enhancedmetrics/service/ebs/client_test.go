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
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"
)

func TestAWSEBSClient_DescribeVolumes(t *testing.T) {
	tests := []struct {
		name    string
		volumes []string
		client  awsClient
		want    []types.Volume
		wantErr bool
	}{
		{
			name:    "success - single page",
			volumes: []string{"vol-1"},
			client: &mockEBSClient{
				describeVolumesFunc: func(_ context.Context, params *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					if len(params.VolumeIds) != 0 || len(params.Filters) != 1 || aws.ToString(params.Filters[0].Name) != "volume-id" {
						return nil, fmt.Errorf("unexpected input: %+v", params)
					}
					return &ec2.DescribeVolumesOutput{
						Volumes: []types.Volume{{VolumeId: aws.String("vol-1")}},
					}, nil
				},
			},
			want: []types.Volume{{VolumeId: aws.String("vol-1")}},
		},
		{
			name:    "success - multiple pages",
			volumes: []string{"vol-1", "vol-2"},
			client: &mockEBSClient{
				describeVolumesFunc: func(_ context.Context, params *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					if !slices.Equal(params.Filters[0].Values, []string{"vol-1", "vol-2"}) {
						return nil, fmt.Errorf("unexpected filter values: %v", params.Filters[0].Values)
					}
					if params.NextToken == nil {
						return &ec2.DescribeVolumesOutput{
							Volumes:   []types.Volume{{VolumeId: aws.String("vol-1")}},
							NextToken: aws.String("token1"),
						}, nil
					}
					if aws.ToString(params.NextToken) != "token1" {
						return nil, fmt.Errorf("unexpected token: %v", aws.ToString(params.NextToken))
					}
					return &ec2.DescribeVolumesOutput{
						Volumes: []types.Volume{{VolumeId: aws.String("vol-2")}},
					}, nil
				},
			},
			want: []types.Volume{{VolumeId: aws.String("vol-1")}, {VolumeId: aws.String("vol-2")}},
		},
		{
			name:    "error - API failure",
			volumes: []string{"vol-1"},
			client: &mockEBSClient{
				describeVolumesFunc: func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			wantErr: true,
		},
		{
			name:    "empty input - no API call",
			volumes: nil,
			client: &mockEBSClient{
				describeVolumesFunc: func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					return nil, fmt.Errorf("DescribeVolumes should not be called for empty input")
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AWSEBSClient{describeVolumesFunc: tt.client.DescribeVolumes}
			got, err := c.DescribeVolumes(context.Background(), slog.New(slog.DiscardHandler), tt.volumes)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

type mockEBSClient struct {
	describeVolumesFunc func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

func (m *mockEBSClient) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return m.describeVolumesFunc(ctx, params, optFns...)
}
