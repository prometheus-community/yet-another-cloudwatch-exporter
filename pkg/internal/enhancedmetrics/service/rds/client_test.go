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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/require"
)

func TestAWSRDSClient_DescribeDBInstances(t *testing.T) {
	tests := []struct {
		name      string
		client    awsClient
		want      []types.DBInstance
		wantErr   bool
		instances []string
	}{
		{
			name:      "success - single page",
			instances: []string{"db-1"},
			client: &mockRDSClient{
				describeDBInstancesFunc: func(_ context.Context, params *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					if len(params.Filters) != 1 || *params.Filters[0].Name != "db-instance-id" {
						return nil, fmt.Errorf("unexpected filter: %v", params.Filters)
					}
					return &rds.DescribeDBInstancesOutput{
						DBInstances: []types.DBInstance{
							{DBInstanceIdentifier: aws.String("db-1")},
						},
						Marker: nil,
					}, nil
				},
			},
			want: []types.DBInstance{
				{DBInstanceIdentifier: aws.String("db-1")},
			},
			wantErr: false,
		},
		{
			name:      "success - multiple pages",
			instances: []string{"db-1", "db-2"},
			client: &mockRDSClient{
				describeDBInstancesFunc: func() func(_ context.Context, params *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					callCount := 0
					return func(_ context.Context, params *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
						if len(params.Filters) != 1 || *params.Filters[0].Name != "db-instance-id" {
							return nil, fmt.Errorf("unexpected filter: %v", params.Filters)
						}
						if params.Filters[0].Values[0] != "db-1" || params.Filters[0].Values[1] != "db-2" {
							return nil, fmt.Errorf("unexpected filter values: %v", params.Filters[0].Values)
						}

						callCount++
						if callCount == 1 {
							return &rds.DescribeDBInstancesOutput{
								DBInstances: []types.DBInstance{
									{DBInstanceIdentifier: aws.String("db-1")},
								},
								Marker: aws.String("marker1"),
							}, nil
						}
						return &rds.DescribeDBInstancesOutput{
							DBInstances: []types.DBInstance{
								{DBInstanceIdentifier: aws.String("db-2")},
							},
							Marker: nil,
						}, nil
					}
				}(),
			},
			want: []types.DBInstance{
				{DBInstanceIdentifier: aws.String("db-1")},
				{DBInstanceIdentifier: aws.String("db-2")},
			},
			wantErr: false,
		},
		{
			name:      "error - API failure",
			instances: []string{"db-1"},
			client: &mockRDSClient{
				describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:      "empty input - no API call",
			instances: nil,
			client: &mockRDSClient{
				describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					return nil, fmt.Errorf("DescribeDBInstances should not be called for empty input")
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AWSRDSClient{
				describeDBInstancesFunc: tt.client.DescribeDBInstances,
			}
			got, err := c.DescribeDBInstances(context.Background(), slog.New(slog.DiscardHandler), tt.instances)
			if (err != nil) != tt.wantErr {
				t.Errorf("DescribeDBInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DescribeDBInstances() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAWSRDSClient_DescribeDBInstances_ChunksFilterValues(t *testing.T) {
	tests := []struct {
		name            string
		instanceCount   int
		paginateCallNum int // 1-based call that returns a marker, splitting its chunk into two pages; 0 disables
		failCallNum     int // 1-based call that returns an error; 0 disables
		wantChunkSizes  []int
		wantErr         bool
	}{
		{name: "exactly the limit", instanceCount: 100, wantChunkSizes: []int{100}},
		{name: "one above the limit", instanceCount: 101, wantChunkSizes: []int{100, 1}},
		{name: "above the limit", instanceCount: 250, wantChunkSizes: []int{100, 100, 50}},
		{name: "pagination in the last chunk", instanceCount: 150, paginateCallNum: 2, wantChunkSizes: []int{100, 50, 50}},
		{name: "pagination in a non-last chunk", instanceCount: 250, paginateCallNum: 1, wantChunkSizes: []int{100, 100, 100, 50}},
		{name: "error in a later chunk", instanceCount: 150, failCallNum: 2, wantChunkSizes: []int{100, 50}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instances := make([]string, tt.instanceCount)
			want := make([]types.DBInstance, tt.instanceCount)
			for i := range instances {
				instances[i] = fmt.Sprintf("db-%d", i)
				want[i] = types.DBInstance{DBInstanceIdentifier: aws.String(instances[i])}
			}

			var gotChunkSizes []int
			var gotChunkValues []string
			var prevValues []string
			var pendingMarker *string
			mock := &mockRDSClient{
				describeDBInstancesFunc: func(_ context.Context, params *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					require.Len(t, params.Filters, 1)
					require.Equal(t, "db-instance-id", aws.ToString(params.Filters[0].Name))
					values := params.Filters[0].Values
					require.LessOrEqual(t, len(values), maxDBInstanceIdentifiersPerFilter)
					require.Equal(t, pendingMarker, params.Marker, "marker must continue the current chunk and reset for a new one")

					gotChunkSizes = append(gotChunkSizes, len(values))
					if params.Marker == nil {
						gotChunkValues = append(gotChunkValues, values...)
					} else {
						require.Equal(t, prevValues, values, "marker page must repeat the chunk's filter values")
					}
					prevValues, pendingMarker = values, nil

					callNum := len(gotChunkSizes)
					if callNum == tt.failCallNum {
						return nil, fmt.Errorf("API error")
					}

					page := values
					output := &rds.DescribeDBInstancesOutput{}
					switch {
					case callNum == tt.paginateCallNum:
						page = values[:len(values)/2]
						pendingMarker = aws.String(fmt.Sprintf("m-%d", callNum))
						output.Marker = pendingMarker
					case params.Marker != nil:
						page = values[len(values)/2:]
					}
					for _, id := range page {
						output.DBInstances = append(output.DBInstances, types.DBInstance{DBInstanceIdentifier: aws.String(id)})
					}
					return output, nil
				},
			}

			c := &AWSRDSClient{describeDBInstancesFunc: mock.DescribeDBInstances}
			got, err := c.DescribeDBInstances(context.Background(), slog.New(slog.DiscardHandler), instances)

			require.Equal(t, tt.wantChunkSizes, gotChunkSizes, "filter sizes per call")
			require.Equal(t, instances, gotChunkValues, "each identifier must be sent exactly once, in order")
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

// mockRDSClient is a mock implementation of AWS RDS Client
type mockRDSClient struct {
	describeDBInstancesFunc func(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

func (m *mockRDSClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return m.describeDBInstancesFunc(ctx, params, optFns...)
}
