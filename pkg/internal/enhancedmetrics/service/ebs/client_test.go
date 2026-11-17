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

func TestAWSEBSClient_DescribeVolumes_ChunksFilterValues(t *testing.T) {
	tests := []struct {
		name            string
		volumeCount     int
		paginateCallNum int // 1-based call that returns a NextToken, splitting its chunk into two pages; 0 disables
		failCallNum     int // 1-based call that returns an error; 0 disables
		wantChunkSizes  []int
		wantErr         bool
	}{
		{name: "exactly the limit", volumeCount: 200, wantChunkSizes: []int{200}},
		{name: "one above the limit", volumeCount: 201, wantChunkSizes: []int{200, 1}},
		{name: "above the limit", volumeCount: 529, wantChunkSizes: []int{200, 200, 129}},
		{name: "pagination in the last chunk", volumeCount: 300, paginateCallNum: 2, wantChunkSizes: []int{200, 100, 100}},
		{name: "pagination in a non-last chunk", volumeCount: 300, paginateCallNum: 1, wantChunkSizes: []int{200, 200, 100}},
		{name: "error in a later chunk", volumeCount: 300, failCallNum: 2, wantChunkSizes: []int{200, 100}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeIDs := make([]string, tt.volumeCount)
			want := make([]types.Volume, tt.volumeCount)
			for i := range volumeIDs {
				volumeIDs[i] = fmt.Sprintf("vol-%d", i)
				want[i] = types.Volume{VolumeId: aws.String(volumeIDs[i])}
			}

			var gotChunkSizes []int
			var gotChunkValues []string
			var prevValues []string
			var pendingToken *string
			mock := &mockEBSClient{
				describeVolumesFunc: func(_ context.Context, params *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					require.Len(t, params.Filters, 1)
					require.Equal(t, "volume-id", aws.ToString(params.Filters[0].Name))
					values := params.Filters[0].Values
					require.LessOrEqual(t, len(values), maxVolumeIDsPerFilter)
					require.Equal(t, pendingToken, params.NextToken, "token must continue the current chunk and reset for a new one")

					gotChunkSizes = append(gotChunkSizes, len(values))
					if params.NextToken == nil {
						gotChunkValues = append(gotChunkValues, values...)
					} else {
						require.Equal(t, prevValues, values, "token page must repeat the chunk's filter values")
					}
					prevValues, pendingToken = values, nil

					callNum := len(gotChunkSizes)
					if callNum == tt.failCallNum {
						return nil, fmt.Errorf("API error")
					}

					page := values
					output := &ec2.DescribeVolumesOutput{}
					switch {
					case callNum == tt.paginateCallNum:
						page = values[:len(values)/2]
						pendingToken = aws.String(fmt.Sprintf("t-%d", callNum))
						output.NextToken = pendingToken
					case params.NextToken != nil:
						page = values[len(values)/2:]
					}
					for _, id := range page {
						output.Volumes = append(output.Volumes, types.Volume{VolumeId: aws.String(id)})
					}
					return output, nil
				},
			}

			c := &AWSEBSClient{describeVolumesFunc: mock.DescribeVolumes}
			got, err := c.DescribeVolumes(context.Background(), slog.New(slog.DiscardHandler), volumeIDs)

			require.Equal(t, tt.wantChunkSizes, gotChunkSizes, "filter sizes per call")
			require.Equal(t, volumeIDs, gotChunkValues, "each volume ID must be sent exactly once, in order")
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

type mockEBSClient struct {
	describeVolumesFunc func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

func (m *mockEBSClient) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return m.describeVolumesFunc(ctx, params, optFns...)
}
