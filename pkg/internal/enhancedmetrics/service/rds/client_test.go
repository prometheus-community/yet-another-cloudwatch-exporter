// Copyright 2026 The Prometheus Authors
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
			name: "error - API failure",
			client: &mockRDSClient{
				describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AWSRDSClient{
				client: tt.client,
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

// mockRDSClient is a mock implementation of AWS RDS Client
type mockRDSClient struct {
	describeDBInstancesFunc func(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

func (m *mockRDSClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	return m.describeDBInstancesFunc(ctx, params, optFns...)
}
