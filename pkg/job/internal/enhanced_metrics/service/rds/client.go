package rds

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// todo: change logging to debug where appropriate

// AWSRDSClient wraps the AWS RDS client
type AWSRDSClient struct {
	client *rds.Client
}

// NewRDSClientWithConfig creates a new RDS client with custom AWS configuration
func NewRDSClientWithConfig(cfg aws.Config) Client {
	return &AWSRDSClient{
		client: rds.NewFromConfig(cfg),
	}
}

// DescribeDBInstancesInput contains parameters for describeDBInstances
type DescribeDBInstancesInput struct {
	DBInstanceIdentifier *string
	Filters              []types.Filter
	MaxRecords           *int32
	Marker               *string
}

// DescribeDBInstancesOutput contains the response from describeDBInstances
type DescribeDBInstancesOutput struct {
	DBInstances []types.DBInstance
	Marker      *string
}

// describeDBInstances retrieves information about provisioned RDS instances
func (c *AWSRDSClient) describeDBInstances(ctx context.Context, input *DescribeDBInstancesInput) (*DescribeDBInstancesOutput, error) {
	rdsInput := &rds.DescribeDBInstancesInput{}

	if input != nil {
		rdsInput.DBInstanceIdentifier = input.DBInstanceIdentifier
		rdsInput.Filters = input.Filters
		rdsInput.MaxRecords = input.MaxRecords
		rdsInput.Marker = input.Marker
	}

	result, err := c.client.DescribeDBInstances(ctx, rdsInput)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}

	return &DescribeDBInstancesOutput{
		DBInstances: result.DBInstances,
		Marker:      result.Marker,
	}, nil
}

// DescribeAllDBInstances retrieves all DB instances by handling pagination
func (c *AWSRDSClient) DescribeAllDBInstances(ctx context.Context, logger *slog.Logger) ([]types.DBInstance, error) {
	logger.Info("Looking for all DB instances")
	var allInstances []types.DBInstance
	var marker *string
	var maxRecords int32 = 100

	for {
		output, err := c.describeDBInstances(ctx, &DescribeDBInstancesInput{
			Marker:     marker,
			MaxRecords: &maxRecords,
		})
		if err != nil {
			return nil, err
		}

		allInstances = append(allInstances, output.DBInstances...)

		if output.Marker == nil {
			break
		}
		marker = output.Marker
	}

	return allInstances, nil
}

// describeDBInstance retrieves information about a specific DB instance
func (c *AWSRDSClient) describeDBInstance(ctx context.Context, dbInstanceIdentifier string) (*types.DBInstance, error) {
	output, err := c.describeDBInstances(ctx, &DescribeDBInstancesInput{
		DBInstanceIdentifier: &dbInstanceIdentifier,
	})
	if err != nil {
		return nil, err
	}

	if len(output.DBInstances) == 0 {
		return nil, fmt.Errorf("DB instance %s not found", dbInstanceIdentifier)
	}

	return &output.DBInstances[0], nil
}
