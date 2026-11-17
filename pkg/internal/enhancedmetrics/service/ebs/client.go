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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// maxVolumeIDsPerFilter is the maximum number of values the DescribeVolumes "volume-id" filter
// accepts. Larger filters are rejected with
// "FilterLimitExceeded: The maximum number of filter values specified on a single call is 200".
const maxVolumeIDsPerFilter = 200

// awsClient is kept for test mocking; production code uses method-value closures.
type awsClient interface {
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

// AWSEBSClient wraps the AWS EC2 client
type AWSEBSClient struct {
	describeVolumesFunc func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

// NewEBSClientWithConfig creates a new EBS client with custom AWS configuration
func NewEBSClientWithConfig(cfg aws.Config) Client {
	c := ec2.NewFromConfig(cfg)
	return &AWSEBSClient{
		describeVolumesFunc: c.DescribeVolumes,
	}
}

func (c *AWSEBSClient) describeVolumes(ctx context.Context, input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	result, err := c.describeVolumesFunc(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe volumes: %w", err)
	}
	return result, nil
}

// DescribeVolumes retrieves the volumes identified by volumeIDs (passed in the "volume-id" filter
// in chunks of maxVolumeIDsPerFilter), handling pagination.
// The IDs are passed as a filter rather than the VolumeIds parameter, because
// VolumeIds fails the whole request if any one volume no longer exists, and the tagging API
// can still return a volume for a while after it is deleted.
func (c *AWSEBSClient) DescribeVolumes(ctx context.Context, logger *slog.Logger, volumeIDs []string) ([]types.Volume, error) {
	if len(volumeIDs) == 0 {
		return nil, nil
	}

	logger.Debug("Describing EBS volumes", slog.Int("requestedVolumes", len(volumeIDs)))
	var allVolumes []types.Volume

	for chunk := range slices.Chunk(volumeIDs, maxVolumeIDsPerFilter) {
		var nextToken *string
		for {
			output, err := c.describeVolumes(ctx, &ec2.DescribeVolumesInput{
				NextToken: nextToken,
				Filters: []types.Filter{
					{
						Name:   aws.String("volume-id"),
						Values: chunk,
					},
				},
			})
			if err != nil {
				return nil, err
			}

			allVolumes = append(allVolumes, output.Volumes...)

			if output.NextToken == nil {
				break
			}
			nextToken = output.NextToken
		}
	}

	logger.Debug("Completed describing EBS volumes", slog.Int("totalVolumes", len(allVolumes)))
	return allVolumes, nil
}
