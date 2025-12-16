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

type awsClient interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

// AWSRDSClient wraps the AWS RDS client
type AWSRDSClient struct {
	client awsClient
}

// NewRDSClientWithConfig creates a new RDS client with custom AWS configuration
func NewRDSClientWithConfig(cfg aws.Config) Client {
	return &AWSRDSClient{
		client: rds.NewFromConfig(cfg),
	}
}

// describeDBInstances retrieves information about provisioned RDS instances
func (c *AWSRDSClient) describeDBInstances(ctx context.Context, input *rds.DescribeDBInstancesInput) (*rds.DescribeDBInstancesOutput, error) {
	result, err := c.client.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe DB instances: %w", err)
	}
	return result, nil
}

// DescribeAllDBInstances retrieves all DB instances by handling pagination
func (c *AWSRDSClient) DescribeAllDBInstances(ctx context.Context, logger *slog.Logger) ([]types.DBInstance, error) {
	logger.Info("Looking for all DB instances")
	var allInstances []types.DBInstance
	var marker *string
	maxRecords := aws.Int32(100)

	for {
		output, err := c.describeDBInstances(ctx, &rds.DescribeDBInstancesInput{
			Marker:     marker,
			MaxRecords: maxRecords,
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
