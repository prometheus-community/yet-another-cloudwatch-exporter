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
package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/internal/quotametrics/quotas"
)

type Client interface {
	ListBuckets(ctx context.Context) ([]types.Bucket, error)
	QuotasClient() quotas.Client
}

type awsS3Client interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

type AWSClient struct {
	s3Client     awsS3Client
	quotasClient quotas.Client
}

func NewClientWithConfig(cfg aws.Config) Client {
	return &AWSClient{
		s3Client:     s3.NewFromConfig(cfg),
		quotasClient: quotas.NewServiceQuotasClient(cfg),
	}
}

func (c *AWSClient) ListBuckets(ctx context.Context) ([]types.Bucket, error) {
	output, err := c.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}
	return output.Buckets, nil
}

func (c *AWSClient) QuotasClient() quotas.Client {
	return c.quotasClient
}
