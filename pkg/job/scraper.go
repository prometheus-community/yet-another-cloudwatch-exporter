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
	"fmt"
	"sync"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/cloudwatchrunner"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Scraper struct {
	jobsCfg       model.JobsConfig
	logger        logging.Logger
	runnerFactory runnerFactory
}

type runnerFactory interface {
	GetAccountClient(region string, role model.Role) account.Client
	NewResourceMetadataRunner(logger logging.Logger, region string, role model.Role) ResourceMetadataRunner
	NewCloudWatchRunner(logger logging.Logger, region string, role model.Role, job cloudwatchrunner.Job) CloudwatchRunner
}

type ResourceMetadataRunner interface {
	Run(ctx context.Context, region string, job model.DiscoveryJob) ([]*model.TaggedResource, error)
}

type CloudwatchRunner interface {
	Run(ctx context.Context) ([]*model.CloudwatchData, error)
}

func NewScraper(logger logging.Logger,
	jobsCfg model.JobsConfig,
	runnerFactory runnerFactory,
) *Scraper {
	return &Scraper{
		runnerFactory: runnerFactory,
		logger:        logger,
		jobsCfg:       jobsCfg,
	}
}

type ErrorType string

var (
	AccountErr              ErrorType = "Account for job was not found"
	ResourceMetadataErr     ErrorType = "Failed to run resource metadata for job"
	CloudWatchCollectionErr ErrorType = "Failed to gather cloudwatch metrics for job"
)

type Account struct {
	ID    string
	Alias string
}

func (s Scraper) Scrape(ctx context.Context) ([]model.TaggedResourceResult, []model.CloudwatchMetricResult, []Error) {
	// Setup so we only do one GetAccount call per region + role combo when running jobs
	roleRegionToAccount := map[model.Role]map[string]func() (Account, error){}
	jobConfigVisitor(s.jobsCfg, func(_ any, role model.Role, region string) {
		if _, exists := roleRegionToAccount[role]; !exists {
			roleRegionToAccount[role] = map[string]func() (Account, error){}
		}
		roleRegionToAccount[role][region] = sync.OnceValues[Account, error](func() (Account, error) {
			client := s.runnerFactory.GetAccountClient(region, role)
			accountID, err := client.GetAccount(ctx)
			if err != nil {
				return Account{}, fmt.Errorf("failed to get Account: %w", err)
			}
			a := Account{
				ID: accountID,
			}
			accountAlias, err := client.GetAccountAlias(ctx)
			if err != nil {
				s.logger.Warn("Failed to get optional account alias from account", "err", err, "account_id", accountID)
			} else {
				a.Alias = accountAlias
			}
			return a, nil
		})
	})

	var wg sync.WaitGroup
	mux := &sync.Mutex{}
	jobErrors := make([]Error, 0)
	metricResults := make([]model.CloudwatchMetricResult, 0)
	resourceResults := make([]model.TaggedResourceResult, 0)
	s.logger.Debug("Starting job runs")

	jobConfigVisitor(s.jobsCfg, func(job any, role model.Role, region string) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var namespace string
			jobAction(s.logger, job, func(job model.DiscoveryJob) {
				namespace = job.Type
			}, func(job model.CustomNamespaceJob) {
				namespace = job.Namespace
			})
			jobContext := JobContext{
				Namespace: namespace,
				Region:    region,
				RoleARN:   role.RoleArn,
			}
			jobLogger := s.logger.With("namespace", jobContext.Namespace, "region", jobContext.Region, "arn", jobContext.RoleARN)

			account, err := roleRegionToAccount[role][region]()
			if err != nil {
				jobError := NewError(jobContext, AccountErr, err)
				mux.Lock()
				jobErrors = append(jobErrors, jobError)
				mux.Unlock()
				return
			}
			jobContext.Account = account
			jobLogger = jobLogger.With("account_id", jobContext.Account.ID)

			var jobToRun cloudwatchrunner.Job
			jobAction(jobLogger, job,
				func(job model.DiscoveryJob) {
					jobLogger.Debug("Starting resource discovery")
					rmRunner := s.runnerFactory.NewResourceMetadataRunner(jobLogger, region, role)
					resources, err := rmRunner.Run(ctx, region, job)
					if err != nil {
						jobError := NewError(jobContext, ResourceMetadataErr, err)
						mux.Lock()
						jobErrors = append(jobErrors, jobError)
						mux.Unlock()

						return
					}
					if len(resources) > 0 {
						result := model.TaggedResourceResult{
							Context: jobContext.ToScrapeContext(job.CustomTags),
							Data:    resources,
						}
						mux.Lock()
						resourceResults = append(resourceResults, result)
						mux.Unlock()
					} else {
						jobLogger.Debug("No tagged resources")
					}
					jobLogger.Debug("Resource discovery finished", "number_of_discovered_resources", len(resources))

					jobToRun = cloudwatchrunner.DiscoveryJob{Job: job, Resources: resources}
				}, func(job model.CustomNamespaceJob) {
					jobToRun = cloudwatchrunner.CustomNamespaceJob{Job: job}
				},
			)
			if jobToRun == nil {
				jobLogger.Debug("Ending job run early due to job error see job errors")
				return
			}

			jobLogger.Debug("Starting cloudwatch metrics runner")
			cwRunner := s.runnerFactory.NewCloudWatchRunner(jobLogger, region, role, jobToRun)
			metricResult, err := cwRunner.Run(ctx)
			if err != nil {
				jobError := NewError(jobContext, CloudWatchCollectionErr, err)
				mux.Lock()
				jobErrors = append(jobErrors, jobError)
				mux.Unlock()

				return
			}

			if len(metricResult) == 0 {
				jobLogger.Debug("No metrics data found")
				return
			}

			jobLogger.Debug("Job run finished", "number_of_metrics", len(metricResult))

			result := model.CloudwatchMetricResult{
				Context: jobContext.ToScrapeContext(jobToRun.CustomTags()),
				Data:    metricResult,
			}

			mux.Lock()
			defer mux.Unlock()
			metricResults = append(metricResults, result)
		}()
	})
	wg.Wait()
	s.logger.Debug("Finished job runs", "resource_results", len(resourceResults), "metric_results", len(metricResults))
	return resourceResults, metricResults, jobErrors
}

// Walk through each custom namespace and discovery jobs and take an action
func jobConfigVisitor(jobsCfg model.JobsConfig, action func(job any, role model.Role, region string)) {
	for _, job := range jobsCfg.DiscoveryJobs {
		for _, role := range job.Roles {
			for _, region := range job.Regions {
				action(job, role, region)
			}
		}
	}

	for _, job := range jobsCfg.CustomNamespaceJobs {
		for _, role := range job.Roles {
			for _, region := range job.Regions {
				action(job, role, region)
			}
		}
	}
}

// Take an action depending on the job type, only supports discovery and custom job types
func jobAction(logger logging.Logger, job any, discovery func(job model.DiscoveryJob), custom func(job model.CustomNamespaceJob)) {
	// Type switches are free https://stackoverflow.com/a/28027945
	switch typedJob := job.(type) {
	case model.DiscoveryJob:
		discovery(typedJob)
	case model.CustomNamespaceJob:
		custom(typedJob)
	default:
		logger.Error(fmt.Errorf("config type of %T is not supported", typedJob), "Unexpected job type")
		return
	}
}

// JobContext exists to track data we want for logging, errors, or other output context that's learned as the job runs
// This makes it easier to track the data additively and morph it to the final shape necessary be it a model.ScrapeContext
// or an Error. It's an exported type for tests but is not part of the public interface
type JobContext struct { //nolint:revive
	Account   Account
	Namespace string
	Region    string
	RoleARN   string
}

func (jc JobContext) ToScrapeContext(customTags []model.Tag) *model.ScrapeContext {
	return &model.ScrapeContext{
		AccountID:    jc.Account.ID,
		Region:       jc.Region,
		CustomTags:   customTags,
		AccountAlias: jc.Account.Alias,
	}
}

type Error struct {
	JobContext
	ErrorType ErrorType
	Err       error
}

func NewError(context JobContext, errorType ErrorType, err error) Error {
	return Error{
		JobContext: context,
		ErrorType:  errorType,
		Err:        err,
	}
}

func (e Error) ToLoggerKeyVals() []interface{} {
	return []interface{}{
		"account_id", e.Account.ID,
		"namespace", e.Namespace,
		"region", e.Region,
		"role_arn", e.RoleARN,
	}
}
