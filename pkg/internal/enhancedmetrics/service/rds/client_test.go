package rds

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

func TestAWSRDSClient_DescribeAllDBInstances(t *testing.T) {
	tests := []struct {
		name    string
		client  awsClient
		want    []types.DBInstance
		wantErr bool
	}{
		{
			name: "success - single page",
			client: &mockRDSClient{
				describeDBInstancesFunc: func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
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
			name: "success - multiple pages",
			client: &mockRDSClient{
				describeDBInstancesFunc: func() func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
					callCount := 0
					return func(_ context.Context, _ *rds.DescribeDBInstancesInput, _ ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
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
			got, err := c.DescribeAllDBInstances(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
			if (err != nil) != tt.wantErr {
				t.Errorf("DescribeAllDBInstances() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DescribeAllDBInstances() got = %v, want %v", got, tt.want)
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
