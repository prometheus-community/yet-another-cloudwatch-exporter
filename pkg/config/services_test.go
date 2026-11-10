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
package config

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func TestSupportedServices(t *testing.T) {
	for i, svc := range SupportedServices {
		require.NotNil(t, svc.Namespace, fmt.Sprintf("Nil Namespace for service at index '%d'", i))
		require.NotNil(t, svc.Alias, fmt.Sprintf("Nil Alias for service '%s' at index '%d'", svc.Namespace, i))

		if svc.ResourceFilters != nil {
			require.NotEmpty(t, svc.ResourceFilters)

			for _, filter := range svc.ResourceFilters {
				require.NotEmpty(t, aws.ToString(filter))
			}
		}

		if svc.DimensionRegexps != nil {
			require.NotEmpty(t, svc.DimensionRegexps)

			for _, regex := range svc.DimensionRegexps {
				require.NotEmpty(t, regex.String())
				require.Positive(t, regex.NumSubexp())
			}
		}
	}
}

func TestPartitionForRegion(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{"us-east-1", "aws"},
		{"eu-west-2", "aws"},
		{"ap-southeast-4", "aws"},
		{"cn-north-1", "aws-cn"},
		{"cn-northwest-1", "aws-cn"},
		{"us-gov-west-1", "aws-us-gov"},
		{"us-gov-east-1", "aws-us-gov"},
	}
	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			require.Equal(t, tt.want, partitionForRegion(tt.region))
		})
	}
}

func TestServiceConfig_toInferArnFromDimensionsFunc(t *testing.T) {
	const (
		region    = "us-east-1"
		accountID = "123456789012"
	)

	tests := []struct {
		name       string
		namespace  string
		region     string
		dimensions []model.Dimension
		want       string
		wantOK     bool
	}{
		{
			name:       "standard region and account",
			namespace:  "AWS/EC2",
			dimensions: []model.Dimension{{Name: "InstanceId", Value: "i-0abc123def456"}},
			want:       "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def456",
			wantOK:     true,
		},
		{
			name:       "colon-separated resource",
			namespace:  "AWS/Lambda",
			dimensions: []model.Dimension{{Name: "FunctionName", Value: "my-function"}},
			want:       "arn:aws:lambda:us-east-1:123456789012:function:my-function",
			wantOK:     true,
		},
		{
			name:       "bare resource name",
			namespace:  "AWS/SQS",
			dimensions: []model.Dimension{{Name: "QueueName", Value: "my-queue"}},
			want:       "arn:aws:sqs:us-east-1:123456789012:my-queue",
			wantOK:     true,
		},
		{
			name:       "global service omits region",
			namespace:  "AWS/CloudFront",
			dimensions: []model.Dimension{{Name: "DistributionId", Value: "E1234ABCDEF56"}},
			want:       "arn:aws:cloudfront::123456789012:distribution/E1234ABCDEF56",
			wantOK:     true,
		},
		{
			name:       "global service omits region and account",
			namespace:  "AWS/Route53",
			dimensions: []model.Dimension{{Name: "HealthCheckId", Value: "abcd1234-5678"}},
			want:       "arn:aws:route53:::healthcheck/abcd1234-5678",
			wantOK:     true,
		},
		{
			name:       "s3 omits region and account",
			namespace:  "AWS/S3",
			dimensions: []model.Dimension{{Name: "BucketName", Value: "my-bucket"}},
			want:       "arn:aws:s3:::my-bucket",
			wantOK:     true,
		},
		{
			name:       "dimension value is already an ARN",
			namespace:  "AWS/CertificateManager",
			dimensions: []model.Dimension{{Name: "CertificateArn", Value: "arn:aws:acm:us-east-1:123456789012:certificate/abcd-1234"}},
			want:       "arn:aws:acm:us-east-1:123456789012:certificate/abcd-1234",
			wantOK:     true,
		},
		{
			name:       "empty ARN dimension is rejected",
			namespace:  "AWS/CertificateManager",
			dimensions: []model.Dimension{{Name: "CertificateArn", Value: ""}},
			wantOK:     false,
		},
		{
			name:      "firstOf prefers the more specific dimension",
			namespace: "AWS/ApplicationELB",
			dimensions: []model.Dimension{
				{Name: "LoadBalancer", Value: "app/my-lb/0123456789abcdef"},
				{Name: "TargetGroup", Value: "targetgroup/my-tg/0123456789abcdef"},
			},
			want:   "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/0123456789abcdef",
			wantOK: true,
		},
		{
			name:       "firstOf falls through to the less specific dimension",
			namespace:  "AWS/ApplicationELB",
			dimensions: []model.Dimension{{Name: "LoadBalancer", Value: "app/my-lb/0123456789abcdef"}},
			want:       "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-lb/0123456789abcdef",
			wantOK:     true,
		},
		{
			// Regression: the real dimension is "Directory ID" with a space, the table key is
			// "Directory_ID". Without the space-to-underscore conversion this returns false.
			name:       "real dimension name contains a space",
			namespace:  "AWS/DirectoryService",
			dimensions: []model.Dimension{{Name: "Directory ID", Value: "d-abc123"}},
			want:       "arn:aws:ds:us-east-1:123456789012:directory/d-abc123",
			wantOK:     true,
		},
		{
			name:       "real dimension name contains several spaces",
			namespace:  "AWS/PrivateLinkEndpoints",
			dimensions: []model.Dimension{{Name: "VPC Endpoint Id", Value: "vpce-0123456789abcdef"}},
			want:       "arn:aws:ec2:us-east-1:123456789012:vpc-endpoint/vpce-0123456789abcdef",
			wantOK:     true,
		},
		{
			name:       "real dimension name contains a space, service variant",
			namespace:  "AWS/PrivateLinkServices",
			dimensions: []model.Dimension{{Name: "Service Id", Value: "vpce-svc-0123456789abcdef"}},
			want:       "arn:aws:ec2:us-east-1:123456789012:vpc-endpoint-service/vpce-svc-0123456789abcdef",
			wantOK:     true,
		},
		{
			// The counterpart to the case above: AWS/RUM's real dimension name genuinely contains an
			// underscore, so the conversion must leave it alone.
			name:       "real dimension name genuinely contains an underscore",
			namespace:  "AWS/RUM",
			dimensions: []model.Dimension{{Name: "application_name", Value: "my-app-monitor"}},
			want:       "arn:aws:rum:us-east-1:123456789012:appmonitor/my-app-monitor",
			wantOK:     true,
		},
		{
			name:       "china partition",
			namespace:  "AWS/EC2",
			region:     "cn-north-1",
			dimensions: []model.Dimension{{Name: "InstanceId", Value: "i-0abc123def456"}},
			want:       "arn:aws-cn:ec2:cn-north-1:123456789012:instance/i-0abc123def456",
			wantOK:     true,
		},
		{
			name:       "govcloud partition",
			namespace:  "AWS/EC2",
			region:     "us-gov-west-1",
			dimensions: []model.Dimension{{Name: "InstanceId", Value: "i-0abc123def456"}},
			want:       "arn:aws-us-gov:ec2:us-gov-west-1:123456789012:instance/i-0abc123def456",
			wantOK:     true,
		},
		{
			// An aggregate metric whose dimensions identify a group rather than one resource. No ARN
			// is derivable, and none should be: the metric is still ingested with ARN "global".
			// See Test_getFilteredMetricDatas_inferArnFromDimensions for that end-to-end behaviour.
			name:       "aggregate metric carries no resource dimension",
			namespace:  "AWS/EC2",
			dimensions: []model.Dimension{{Name: "AutoScalingGroupName", Value: "my-asg"}},
			wantOK:     false,
		},
		{
			name:       "required dimension present but empty",
			namespace:  "AWS/EC2",
			dimensions: []model.Dimension{{Name: "InstanceId", Value: ""}},
			wantOK:     false,
		},
		{
			name:       "no dimensions at all",
			namespace:  "AWS/EC2",
			dimensions: nil,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := SupportedServices.GetService(tt.namespace)
			require.NotNil(t, svc, "unknown namespace %q", tt.namespace)

			infer := svc.toInferArnFromDimensionsFunc()
			require.NotNil(t, infer, "namespace %q has no ArnFromDimensions", tt.namespace)

			r := tt.region
			if r == "" {
				r = region
			}

			got, ok := infer(r, accountID, tt.dimensions)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.want, got)
			}
		})
	}
}

func TestServiceConfig_toInferArnFromDimensionsFunc_unsupportedNamespace(t *testing.T) {
	// AWS/Kafka deliberately has no ArnFromDimensions: the canonical MSK ARN embeds a UUID that
	// CloudWatch doesn't publish as a dimension.
	svc := SupportedServices.GetService("AWS/Kafka")
	require.NotNil(t, svc)
	require.Nil(t, svc.ArnFromDimensions)
	require.Nil(t, svc.toInferArnFromDimensionsFunc())
}
