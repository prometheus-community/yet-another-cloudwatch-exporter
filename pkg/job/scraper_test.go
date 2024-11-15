// Copyright 2024 The Prometheus Authors
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
package job_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/r3labs/diff/v3"
	"github.com/stretchr/testify/assert"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/cloudwatchrunner"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type testRunnerFactory struct {
	GetAccountAliasFunc func() (string, error)
	GetAccountFunc      func() (string, error)
	MetadataRunFunc     func(ctx context.Context, region string, job model.DiscoveryJob) ([]*model.TaggedResource, error)
	CloudwatchRunFunc   func(ctx context.Context, job cloudwatchrunner.Job) ([]*model.CloudwatchData, error)
}

func (t *testRunnerFactory) GetAccountAlias(context.Context) (string, error) {
	return t.GetAccountAliasFunc()
}

func (t *testRunnerFactory) GetAccount(context.Context) (string, error) {
	return t.GetAccountFunc()
}

func (t *testRunnerFactory) Run(ctx context.Context, region string, job model.DiscoveryJob) ([]*model.TaggedResource, error) {
	return t.MetadataRunFunc(ctx, region, job)
}

func (t *testRunnerFactory) GetAccountClient(string, model.Role) account.Client {
	return t
}

func (t *testRunnerFactory) NewResourceMetadataRunner(logging.Logger, string, model.Role) job.ResourceMetadataRunner {
	return &testMetadataRunner{RunFunc: t.MetadataRunFunc}
}

func (t *testRunnerFactory) NewCloudWatchRunner(_ logging.Logger, _ string, _ model.Role, job cloudwatchrunner.Job) job.CloudwatchRunner {
	return &testCloudwatchRunner{Job: job, RunFunc: t.CloudwatchRunFunc}
}

type testMetadataRunner struct {
	RunFunc func(ctx context.Context, region string, job model.DiscoveryJob) ([]*model.TaggedResource, error)
}

func (t testMetadataRunner) Run(ctx context.Context, region string, job model.DiscoveryJob) ([]*model.TaggedResource, error) {
	return t.RunFunc(ctx, region, job)
}

type testCloudwatchRunner struct {
	RunFunc func(ctx context.Context, job cloudwatchrunner.Job) ([]*model.CloudwatchData, error)
	Job     cloudwatchrunner.Job
}

func (t testCloudwatchRunner) Run(ctx context.Context) ([]*model.CloudwatchData, error) {
	return t.RunFunc(ctx, t.Job)
}

func TestScrapeRunner_Run(t *testing.T) {
	tests := []struct {
		name                string
		jobsCfg             model.JobsConfig
		getAccountFunc      func() (string, error)
		getAccountAliasFunc func() (string, error)
		metadataRunFunc     func(ctx context.Context, region string, job model.DiscoveryJob) ([]*model.TaggedResource, error)
		cloudwatchRunFunc   func(ctx context.Context, job cloudwatchrunner.Job) ([]*model.CloudwatchData, error)
		expectedResources   []model.TaggedResourceResult
		expectedMetrics     []model.CloudwatchMetricResult
		expectedErrs        []job.Error
	}{
		{
			name: "can run a discovery job",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1"},
						Type:    "aws-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-1", ExternalID: "external-id-1"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "aws-account-1", nil
			},
			getAccountAliasFunc: func() (string, error) {
				return "my-aws-account", nil
			},
			metadataRunFunc: func(_ context.Context, _ string, _ model.DiscoveryJob) ([]*model.TaggedResource, error) {
				return []*model.TaggedResource{{
					ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}},
				}}, nil
			},
			cloudwatchRunFunc: func(_ context.Context, _ cloudwatchrunner.Job) ([]*model.CloudwatchData, error) {
				return []*model.CloudwatchData{
					{
						MetricName:          "metric-1",
						ResourceName:        "resource-1",
						Namespace:           "aws-namespace",
						Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
						Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
						GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
					},
				}, nil
			},
			expectedResources: []model.TaggedResourceResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.TaggedResource{
						{ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}}},
					},
				},
			},
			expectedMetrics: []model.CloudwatchMetricResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-1",
							ResourceName:        "resource-1",
							Namespace:           "aws-namespace",
							Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
							Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
						},
					},
				},
			},
		},
		{
			name: "can run a custom namespace job",
			jobsCfg: model.JobsConfig{
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{
						Regions:   []string{"us-east-2"},
						Name:      "my-custom-job",
						Namespace: "custom-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-2", ExternalID: "external-id-2"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "aws-account-1", nil
			},
			getAccountAliasFunc: func() (string, error) {
				return "my-aws-account", nil
			},
			cloudwatchRunFunc: func(_ context.Context, _ cloudwatchrunner.Job) ([]*model.CloudwatchData, error) {
				return []*model.CloudwatchData{
					{
						MetricName:          "metric-2",
						ResourceName:        "resource-2",
						Namespace:           "custom-namespace",
						Dimensions:          []model.Dimension{{Name: "dimension2", Value: "value2"}},
						GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Minimum", Datapoint: aws.Float64(2.0), Timestamp: time.Time{}},
					},
				}, nil
			},
			expectedMetrics: []model.CloudwatchMetricResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-2", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-2",
							ResourceName:        "resource-2",
							Namespace:           "custom-namespace",
							Dimensions:          []model.Dimension{{Name: "dimension2", Value: "value2"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Minimum", Datapoint: aws.Float64(2.0), Timestamp: time.Time{}},
						},
					},
				},
			},
		},
		{
			name: "can run a discovery and custom namespace job",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1"},
						Type:    "aws-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-1", ExternalID: "external-id-1"},
						},
					},
				},
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{
						Regions:   []string{"us-east-2"},
						Name:      "my-custom-job",
						Namespace: "custom-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-2", ExternalID: "external-id-2"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "aws-account-1", nil
			},
			getAccountAliasFunc: func() (string, error) {
				return "my-aws-account", nil
			},
			metadataRunFunc: func(_ context.Context, _ string, _ model.DiscoveryJob) ([]*model.TaggedResource, error) {
				return []*model.TaggedResource{{
					ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}},
				}}, nil
			},
			cloudwatchRunFunc: func(_ context.Context, job cloudwatchrunner.Job) ([]*model.CloudwatchData, error) {
				if job.Namespace() == "custom-namespace" {
					return []*model.CloudwatchData{
						{
							MetricName:          "metric-2",
							ResourceName:        "resource-2",
							Namespace:           "custom-namespace",
							Dimensions:          []model.Dimension{{Name: "dimension2", Value: "value2"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Minimum", Datapoint: aws.Float64(2.0), Timestamp: time.Time{}},
						},
					}, nil
				}
				return []*model.CloudwatchData{
					{
						MetricName:          "metric-1",
						ResourceName:        "resource-1",
						Namespace:           "aws-namespace",
						Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
						Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
						GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
					},
				}, nil
			},
			expectedResources: []model.TaggedResourceResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.TaggedResource{
						{ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}}},
					},
				},
			},
			expectedMetrics: []model.CloudwatchMetricResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-1",
							ResourceName:        "resource-1",
							Namespace:           "aws-namespace",
							Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
							Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
						},
					},
				},
				{
					Context: &model.ScrapeContext{Region: "us-east-2", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-2",
							ResourceName:        "resource-2",
							Namespace:           "custom-namespace",
							Dimensions:          []model.Dimension{{Name: "dimension2", Value: "value2"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Minimum", Datapoint: aws.Float64(2.0), Timestamp: time.Time{}},
						},
					},
				},
			},
		},
		{
			name: "returns errors from GetAccounts",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1"},
						Type:    "aws-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-1", ExternalID: "external-id-1"},
						},
					},
				},
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{
						Regions:   []string{"us-east-2"},
						Name:      "my-custom-job",
						Namespace: "custom-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-2", ExternalID: "external-id-2"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "", errors.New("failed to get account")
			},
			expectedErrs: []job.Error{
				{JobContext: job.JobContext{Account: job.Account{}, Namespace: "aws-namespace", Region: "us-east-1", RoleARN: "aws-arn-1"}, ErrorType: job.AccountErr},
				{JobContext: job.JobContext{Account: job.Account{}, Namespace: "custom-namespace", Region: "us-east-2", RoleARN: "aws-arn-2"}, ErrorType: job.AccountErr},
			},
		},
		{
			name: "ignores errors from GetAccountAlias",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1"},
						Type:    "aws-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-1", ExternalID: "external-id-1"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "aws-account-1", nil
			},
			getAccountAliasFunc: func() (string, error) { return "", errors.New("No alias here") },
			metadataRunFunc: func(_ context.Context, _ string, _ model.DiscoveryJob) ([]*model.TaggedResource, error) {
				return []*model.TaggedResource{{
					ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}},
				}}, nil
			},
			cloudwatchRunFunc: func(_ context.Context, _ cloudwatchrunner.Job) ([]*model.CloudwatchData, error) {
				return []*model.CloudwatchData{
					{
						MetricName:          "metric-1",
						ResourceName:        "resource-1",
						Namespace:           "aws-namespace",
						Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
						Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
						GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
					},
				}, nil
			},
			expectedResources: []model.TaggedResourceResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: ""},
					Data: []*model.TaggedResource{
						{ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}}},
					},
				},
			},
			expectedMetrics: []model.CloudwatchMetricResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: ""},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-1",
							ResourceName:        "resource-1",
							Namespace:           "aws-namespace",
							Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
							Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
						},
					},
				},
			},
		},
		{
			name: "returns errors from resource discovery without failing scrape",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1"},
						Type:    "aws-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-1", ExternalID: "external-id-1"},
						},
					},
				},
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{
						Regions:   []string{"us-east-2"},
						Name:      "my-custom-job",
						Namespace: "custom-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-2", ExternalID: "external-id-2"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "aws-account-1", nil
			},
			getAccountAliasFunc: func() (string, error) {
				return "my-aws-account", nil
			},
			metadataRunFunc: func(_ context.Context, _ string, _ model.DiscoveryJob) ([]*model.TaggedResource, error) {
				return nil, errors.New("I failed you")
			},
			cloudwatchRunFunc: func(_ context.Context, _ cloudwatchrunner.Job) ([]*model.CloudwatchData, error) {
				return []*model.CloudwatchData{
					{
						MetricName:          "metric-2",
						ResourceName:        "resource-2",
						Namespace:           "custom-namespace",
						Dimensions:          []model.Dimension{{Name: "dimension2", Value: "value2"}},
						GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Minimum", Datapoint: aws.Float64(2.0), Timestamp: time.Time{}},
					},
				}, nil
			},
			expectedMetrics: []model.CloudwatchMetricResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-2", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-2",
							ResourceName:        "resource-2",
							Namespace:           "custom-namespace",
							Dimensions:          []model.Dimension{{Name: "dimension2", Value: "value2"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Minimum", Datapoint: aws.Float64(2.0), Timestamp: time.Time{}},
						},
					},
				},
			},
			expectedErrs: []job.Error{
				{
					JobContext: job.JobContext{
						Account:   job.Account{ID: "aws-account-1", Alias: "my-aws-account"},
						Namespace: "aws-namespace",
						Region:    "us-east-1",
						RoleARN:   "aws-arn-1",
					},
					ErrorType: job.ResourceMetadataErr,
				},
			},
		},
		{
			name: "returns errors from cloudwatch metrics runner without failing scrape",
			jobsCfg: model.JobsConfig{
				DiscoveryJobs: []model.DiscoveryJob{
					{
						Regions: []string{"us-east-1"},
						Type:    "aws-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-1", ExternalID: "external-id-1"},
						},
					},
				},
				CustomNamespaceJobs: []model.CustomNamespaceJob{
					{
						Regions:   []string{"us-east-2"},
						Name:      "my-custom-job",
						Namespace: "custom-namespace",
						Roles: []model.Role{
							{RoleArn: "aws-arn-2", ExternalID: "external-id-2"},
						},
					},
				},
			},
			getAccountFunc: func() (string, error) {
				return "aws-account-1", nil
			},
			getAccountAliasFunc: func() (string, error) {
				return "my-aws-account", nil
			},
			metadataRunFunc: func(_ context.Context, _ string, _ model.DiscoveryJob) ([]*model.TaggedResource, error) {
				return []*model.TaggedResource{{
					ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}},
				}}, nil
			},
			cloudwatchRunFunc: func(_ context.Context, job cloudwatchrunner.Job) ([]*model.CloudwatchData, error) {
				if job.Namespace() == "custom-namespace" {
					return nil, errors.New("I failed you")
				}
				return []*model.CloudwatchData{
					{
						MetricName:          "metric-1",
						ResourceName:        "resource-1",
						Namespace:           "aws-namespace",
						Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
						Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
						GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
					},
				}, nil
			},
			expectedResources: []model.TaggedResourceResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.TaggedResource{
						{ARN: "resource-1", Namespace: "aws-namespace", Region: "us-east-1", Tags: []model.Tag{{Key: "tag1", Value: "value1"}}},
					},
				},
			},
			expectedMetrics: []model.CloudwatchMetricResult{
				{
					Context: &model.ScrapeContext{Region: "us-east-1", AccountID: "aws-account-1", AccountAlias: "my-aws-account"},
					Data: []*model.CloudwatchData{
						{
							MetricName:          "metric-1",
							ResourceName:        "resource-1",
							Namespace:           "aws-namespace",
							Tags:                []model.Tag{{Key: "tag1", Value: "value1"}},
							Dimensions:          []model.Dimension{{Name: "dimension1", Value: "value1"}},
							GetMetricDataResult: &model.GetMetricDataResult{Statistic: "Maximum", Datapoint: aws.Float64(1.0), Timestamp: time.Time{}},
						},
					},
				},
			},
			expectedErrs: []job.Error{
				{
					JobContext: job.JobContext{
						Account:   job.Account{ID: "aws-account-1", Alias: "my-aws-account"},
						Namespace: "custom-namespace",
						Region:    "us-east-2",
						RoleARN:   "aws-arn-2",
					},
					ErrorType: job.CloudWatchCollectionErr,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rf := testRunnerFactory{
				GetAccountFunc:      tc.getAccountFunc,
				GetAccountAliasFunc: tc.getAccountAliasFunc,
				MetadataRunFunc:     tc.metadataRunFunc,
				CloudwatchRunFunc:   tc.cloudwatchRunFunc,
			}
			sr := job.NewScraper(logging.NewLogger("", true), tc.jobsCfg, &rf)
			resources, metrics, errs := sr.Scrape(context.Background())

			changelog, err := diff.Diff(tc.expectedResources, resources)
			assert.NoError(t, err, "failed to diff resources")
			assert.Len(t, changelog, 0, changelog)

			changelog, err = diff.Diff(tc.expectedMetrics, metrics)
			assert.NoError(t, err, "failed to diff metrics")
			assert.Len(t, changelog, 0, changelog)

			// We don't want to check the exact error just the message
			changelog, err = diff.Diff(tc.expectedErrs, errs, diff.Filter(func(_ []string, _ reflect.Type, field reflect.StructField) bool {
				return !(field.Name == "Err")
			}))
			assert.NoError(t, err, "failed to diff errs")
			assert.Len(t, changelog, 0, changelog)
		})
	}
}
