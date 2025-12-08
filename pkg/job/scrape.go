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
package job

import (
	"context"
	"log/slog"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	rdsclient "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/enhanced/rds"
	v2 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/v2"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/enhanced"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/enhanced/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/getmetricdata"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

func ScrapeAwsData(
	ctx context.Context,
	logger *slog.Logger,
	jobsCfg model.JobsConfig,
	factory clients.Factory,
	metricsPerQuery int,
	cloudwatchConcurrency cloudwatch.ConcurrencyConfig,
	taggingAPIConcurrency int,
	enhancedMetricsConcurrency int,
) ([]model.TaggedResourceResult, []model.CloudwatchMetricResult) {
	mux := &sync.Mutex{}
	cwData := make([]model.CloudwatchMetricResult, 0)
	awsInfoData := make([]model.TaggedResourceResult, 0)
	var wg sync.WaitGroup

	// Create enhanced metrics processor if any job has enhanced metrics configured
	var enhancedProcessor *enhanced.Processor
	if hasEnhancedMetricsConfigured(jobsCfg) {
		enhancedProcessor = enhanced.NewProcessor(logger)
		// Register RDS clients for all regions and roles
		registerEnhancedMetricsClients(enhancedProcessor, jobsCfg, factory)
	}

	for _, discoveryJob := range jobsCfg.DiscoveryJobs {
		for _, role := range discoveryJob.Roles {
			for _, region := range discoveryJob.Regions {
				wg.Add(1)
				go func(discoveryJob model.DiscoveryJob, region string, role model.Role) {
					defer wg.Done()
					jobLogger := logger.With("namespace", discoveryJob.Namespace, "region", region, "arn", role.RoleArn)
					accountID, err := factory.GetAccountClient(region, role).GetAccount(ctx)
					if err != nil {
						jobLogger.Error("Couldn't get account Id", "err", err)
						return
					}
					jobLogger = jobLogger.With("account", accountID)

					accountAlias, err := factory.GetAccountClient(region, role).GetAccountAlias(ctx)
					if err != nil {
						jobLogger.Warn("Couldn't get account alias", "err", err)
					}

					cloudwatchClient := factory.GetCloudwatchClient(region, role, cloudwatchConcurrency)
					gmdProcessor := getmetricdata.NewDefaultProcessor(logger, cloudwatchClient, metricsPerQuery, cloudwatchConcurrency.GetMetricData)
					resources, metrics := runDiscoveryJob(ctx, jobLogger, discoveryJob, region, factory.GetTaggingClient(region, role, taggingAPIConcurrency), cloudwatchClient, gmdProcessor, enhancedProcessor)
					addDataToOutput := len(metrics) != 0
					if config.FlagsFromCtx(ctx).IsFeatureEnabled(config.AlwaysReturnInfoMetrics) {
						addDataToOutput = addDataToOutput || len(resources) != 0
					}
					if addDataToOutput {
						sc := &model.ScrapeContext{
							Region:       region,
							AccountID:    accountID,
							AccountAlias: accountAlias,
							CustomTags:   discoveryJob.CustomTags,
						}
						metricResult := model.CloudwatchMetricResult{
							Context: sc,
							Data:    metrics,
						}
						resourceResult := model.TaggedResourceResult{
							Data: resources,
						}
						if discoveryJob.IncludeContextOnInfoMetrics {
							resourceResult.Context = sc
						}

						mux.Lock()
						awsInfoData = append(awsInfoData, resourceResult)
						cwData = append(cwData, metricResult)
						mux.Unlock()
					}
				}(discoveryJob, region, role)
			}
		}
	}

	for _, staticJob := range jobsCfg.StaticJobs {
		for _, role := range staticJob.Roles {
			for _, region := range staticJob.Regions {
				wg.Add(1)
				go func(staticJob model.StaticJob, region string, role model.Role) {
					defer wg.Done()
					jobLogger := logger.With("static_job_name", staticJob.Name, "region", region, "arn", role.RoleArn)
					accountID, err := factory.GetAccountClient(region, role).GetAccount(ctx)
					if err != nil {
						jobLogger.Error("Couldn't get account Id", "err", err)
						return
					}
					jobLogger = jobLogger.With("account", accountID)

					accountAlias, err := factory.GetAccountClient(region, role).GetAccountAlias(ctx)
					if err != nil {
						jobLogger.Warn("Couldn't get account alias", "err", err)
					}

					metrics := runStaticJob(ctx, jobLogger, staticJob, factory.GetCloudwatchClient(region, role, cloudwatchConcurrency))
					metricResult := model.CloudwatchMetricResult{
						Context: &model.ScrapeContext{
							Region:       region,
							AccountID:    accountID,
							AccountAlias: accountAlias,
							CustomTags:   staticJob.CustomTags,
						},
						Data: metrics,
					}
					mux.Lock()
					cwData = append(cwData, metricResult)
					mux.Unlock()
				}(staticJob, region, role)
			}
		}
	}

	for _, customNamespaceJob := range jobsCfg.CustomNamespaceJobs {
		for _, role := range customNamespaceJob.Roles {
			for _, region := range customNamespaceJob.Regions {
				wg.Add(1)
				go func(customNamespaceJob model.CustomNamespaceJob, region string, role model.Role) {
					defer wg.Done()
					jobLogger := logger.With("custom_metric_namespace", customNamespaceJob.Namespace, "region", region, "arn", role.RoleArn)
					accountID, err := factory.GetAccountClient(region, role).GetAccount(ctx)
					if err != nil {
						jobLogger.Error("Couldn't get account Id", "err", err)
						return
					}
					jobLogger = jobLogger.With("account", accountID)

					accountAlias, err := factory.GetAccountClient(region, role).GetAccountAlias(ctx)
					if err != nil {
						jobLogger.Warn("Couldn't get account alias", "err", err)
					}

					cloudwatchClient := factory.GetCloudwatchClient(region, role, cloudwatchConcurrency)
					gmdProcessor := getmetricdata.NewDefaultProcessor(logger, cloudwatchClient, metricsPerQuery, cloudwatchConcurrency.GetMetricData)
					metrics := runCustomNamespaceJob(ctx, jobLogger, customNamespaceJob, cloudwatchClient, gmdProcessor)
					metricResult := model.CloudwatchMetricResult{
						Context: &model.ScrapeContext{
							Region:       region,
							AccountID:    accountID,
							AccountAlias: accountAlias,
							CustomTags:   customNamespaceJob.CustomTags,
						},
						Data: metrics,
					}
					mux.Lock()
					cwData = append(cwData, metricResult)
					mux.Unlock()
				}(customNamespaceJob, region, role)
			}
		}
	}
	wg.Wait()
	return awsInfoData, cwData
}

func hasEnhancedMetricsConfigured(cfg model.JobsConfig) bool {
	for _, job := range cfg.DiscoveryJobs {
		if len(job.EnhancedMetrics) > 0 {
			return true
		}
	}
	return false
}

func registerEnhancedMetricsClients(processor *enhanced.Processor, cfg model.JobsConfig, factory clients.Factory) {
	// Track which regions we've already registered clients for
	rdsRegions := make(map[string]bool)

	for _, job := range cfg.DiscoveryJobs {
		// Only register clients for jobs that have enhanced metrics
		if len(job.EnhancedMetrics) == 0 {
			continue
		}

		// Check if this job is for a namespace that supports enhanced metrics
		if job.Namespace == "AWS/RDS" {
			// Get the RDS service from the processor
			if svc := processor.GetService("AWS/RDS"); svc != nil {
				rdsSvc, ok := svc.(*rds.Service)
				if ok {
					// Register RDS clients for each region/role combination
					for _, role := range job.Roles {
						for _, region := range job.Regions {
							key := region + "-" + role.RoleArn
							if !rdsRegions[key] {
								// Get AWS config from factory
								awsCfg := getAWSConfigFromFactory(factory, region, role)
								if awsCfg != nil {
									client := rdsclient.NewClient(*awsCfg)
									rdsSvc.RegisterClient(region, client)
									rdsRegions[key] = true
								}
							}
						}
					}
				}
			}
		}
	}
}

func getAWSConfigFromFactory(factory clients.Factory, region string, role model.Role) *aws.Config {
	// Try v2 factory first (most common)
	if v2Factory, ok := factory.(*v2.CachingFactory); ok {
		return v2Factory.GetAWSConfig(region, role)
	}
	// v1 factory doesn't expose AWS config, so we can't support enhanced metrics with v1
	return nil
}
