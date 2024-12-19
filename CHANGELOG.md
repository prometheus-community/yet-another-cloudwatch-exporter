# main (unreleased)

**Important news and breaking changes**

* ...

**Bugfixes and features**

Features:
* ...

Bugs:
* ...

Docs:
* ...

Refactoring:
* ...

**Dependencies**

* ...

**New contributors**

* ...

**Full Changelog**: https://github.com/...

# 0.62.0

**Important news and breaking changes**

* As of November 2024, YACE is part of prometheus-community. Read more about it in these announcement posts:
- https://prometheus.io/blog/2024/11/19/yace-joining-prometheus-community/
- https://grafana.com/blog/2024/11/19/yace-moves-to-prometheus-community/

**Bugfixes and features**

Features:
* Add ContainerInsights service by @JetSquirrel
* Add AWS/Scheduler and AWS/ECR services by @andriikushch
* Add AWS/VpcLattice service by @greymd
* Add AWS/QuickSight service by @choppedpork
* Add AWS/Timestream service by @andriikushch
* Add Network Manager / Cloud WAN support by @kylehodgetts
* RDS: include RDS Proxy metrics within the RDS namespace by @vitaliyf
* Mediapackage: include mediapackagev2 namespace by @henrylaiBrightcove

Bugs:
* Add parentheses to sanitize list to prevent invalid metric name generation by @nixargh

Docs:
* Review and update supported services in README by @cristiangreco
* Mention support for AWS/MediaPackage by @prathamesh-sonpatki
* Update README and MAINTAINERS files to mention the move to prometheus-community by @cristiangreco

Refactoring:
* Start a unified scraper by @kgeckhart
* Refactor prom metric creation by @kgeckhart
* Update for Prometheus Community by @SuperQ
* Update Docker build by @SuperQ
* Fix linting issues detected by golangci-lint 1.60.3 by @cristiangreco
* Update build tools and CI to Go 1.23 by @cristiangreco

**Dependencies**

* Bump actions/checkout from 4.2.0 to 4.2.2
* Bump alpine from 3.20.1 to 3.20.3
* Bump github.com/aws/aws-sdk-go from 1.54.7 to 1.55.5
* Bump github.com/aws/smithy-go from 1.22.0 to 1.22.1
* Bump github.com/prometheus/client_golang from 1.19.1 to 1.20.5
* Bump github.com/prometheus/common from 0.54.0 to 0.60.1
* Bump github.com/stretchr/testify from 1.9.0 to 1.10.0
* Bump github.com/urfave/cli/v2 from 2.27.2 to 2.27.5
* Bump golang from 1.22 to 1.23
* Bump golang.org/x/sync from 0.7.0 to 0.9.0
* Bump golangci/golangci-lint-action from 6.0.1 to 6.1.1
* Bump grafana/regexp to `20240607082908-2cb410fa05da`
* Bump the aws-sdk-v2 group

**New contributors**

* @prathamesh-sonpatki made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1465
* @JetSquirrel made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1463
* @greymd made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1506
* @choppedpork made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1477
* @SuperQ made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1568
* @prombot made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1570
* @nixargh made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1563
* @kylehodgetts made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1580
* @vitaliyf made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1501
* @henrylaiBrightcove made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1544

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.61.2...v0.62.0

# 0.61.2

Bugfix release to update the `goreleaser` configuration (again!), please refer to the release notes for `0.61.0` for actual code changes.

https://github.com/prometheus-community/yet-another-cloudwatch-exporter/releases/tag/v0.61.0

# 0.61.1

Bugfix release to update the `goreleaser` configuration, please refer to the release notes for `0.61.0` for actual code changes.

https://github.com/prometheus-community/yet-another-cloudwatch-exporter/releases/tag/v0.61.0

# 0.61.0

**Important news and breaking changes**

* This release adds support for AWS account aliases (by @thepalbi). If the role used by YACE has `"iam:ListAccountAliases"` permission, the account alias (if any) is added as a label to the `aws_account_info` metric.

**Bugfixes and features**

Features:
* Add AWS/EC2CapacityReservations to the services list by @luismy
* Add support for MediaPackage metrics by @theunissenne
* Add AWS/AppRunner as supported service by @fabiiw05

Bugs:
* Fix association with gwlb by @vainiusd

Refactoring:
* Add support for batching by time params by @kgeckhart

**Dependencies**

* Bump alpine from 3.19.1 to 3.20.1
* Bump github.com/aws/aws-sdk-go from 1.53.1 to 1.54.7
* Bump github.com/aws/aws-sdk-go-v2/service/ec2 from 1.161.4 to 1.162.0 in the aws-sdk-v2 group
* Bump github.com/prometheus/common from 0.53.0 to 0.54.0
* Bump golangci/golangci-lint-action from 5.3.0 to 6.0.1
* Bump goreleaser/goreleaser-action from 5 to 6
* Bump the aws-sdk-v2 group

**New contributors**

* @luismy made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1341
* @fabiiw05 made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1433

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.60.0...v0.61.0

# 0.60.0

**Bugfixes and features**

Features:
* add cloudwatch log metric support by @vainiusd
* feat: add AWS/RUM as supported service by @hexionas

Bugs:
* Fix all value for function_name variable in lambda dashboard by @thepalbi
* Fix rounding period deprecation notice by @cristiangreco

Docs:
* README: update config example by @cristiangreco
* Fix ElastiCache metric namespace typo on README by @Roberdvs

Refactoring:
* getmetricdata: Move batching to an iterator by @kgeckhart

**Dependencies**
* Bump github.com/aws/aws-sdk-go from 1.51.21 to 1.53.1
* Bump github.com/aws/aws-sdk-go-v2/service/ec2 from 1.156.0 to 1.160.0
* Bump github.com/prometheus/client_golang from 1.19.0 to 1.19.1
* Bump github.com/prometheus/common from 0.52.3 to 0.53.0
* Bump github.com/urfave/cli/v2 from 2.27.1 to 2.27.2
* Bump golangci/golangci-lint-action from 4.0.0 to 5.3.0
* Bump the aws-sdk-v2 group with 13 updates

**New contributors**

* @Roberdvs made their first contribution
* @hexionas made their first contribution

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.59.0...v0.60.0

# 0.59.0

**Important news and breaking changes**

This release brings a bunch of breaking changes:
* Setting `roundingPeriod` for discovery jobs is deprecated, a warning will be logged at startup. This is being deprecated in favor of always using the metric period. The implementation for `roundingPeriod` can result in inconsistent Start and EndTime between batches. This negates its intent to ensure Start and EndTimes align with the metric period for [CloudWatch best practices](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html). This has the potential to produce data which will look inaccurate when compared against CloudWatch itself driving a lot of confusion. See https://github.com/prometheus-community/yet-another-cloudwatch-exporter/issues/1290 for further context.
* Setting `delay` at the metric level is deprecated, a warning will be logged at startup. This `delay` configuration has existed for a long time but was never utilized. Deprecating it and eventually removing it was chosen to simplify the configuration. See https://github.com/prometheus-community/yet-another-cloudwatch-exporter/issues/1290#issuecomment-1948904375 for further context.
* For discovery jobs, the `type` field and the keys of `exportedTagsOnMetrics` must be the AWS namespace rather than the alias (the README contains an up-to-date list of namespaces). Aliases are not allowed anymore. An error will be thrown at startup in an invalid namespace or an alias is used.
* Some metric names have been changed to avoid duplicating the namespace. This includes:
  - `aws_es_esreporting_failed_request_sys_err_count` is `aws_es_reporting_failed_request_sys_err_count`
  - `aws_es_esreporting_failed_request_user_err_count` is `aws_es_reporting_failed_request_user_err_count`
  - `aws_es_esreporting_request_count` is `aws_es_reporting_request_count`
  - `aws_es_esreporting_success_count` is `aws_es_reporting_success_count`
  - `aws_kafka_kafka_app_logs_disk_used` is `aws_kafka_app_logs_disk_used`
  - `aws_kafka_kafka_data_logs_disk_used` is `aws_kafka_data_logs_disk_used`
  - `aws_rds_rdsto_aurora_postgre_sqlreplica_lag` is  `aws_rds_to_aurora_postgre_sqlreplica_lag`
  - `aws_glue_glue_.*` is `aws_glue_.*`

These breaking changes will allow making the configuration easier to understand and less error prone, and also to build better documentation around supported services.

**Bugfixes and features**

Features:
* Add AWS/SecretsManager to the services list by @taraspos
* Support partner events buses by @HristoStoyanovYotpo
* `discovery.exportedTagsOnMetrics`: validate that keys match one of the job types defined by @cristiangreco

Refactoring:
* Update comment in factory.go by @andriikushch
* getmetricdata: move window calculator to processor by @kgeckhart
* promutil: clean up prom metric names that duplicate parts of the namespace by @tristanburgess
* promutil: rewrite sanitisation funcs for memory optimisation by @cristiangreco
* Do not allow using aliases as job types in discovery jobs by @cristiangreco

**Dependencies**

* Bump github.com/aws/aws-sdk-go from 1.51.16 to 1.51.21
* Bump github.com/aws/aws-sdk-go-v2 group
* Bump github.com/prometheus/common from 0.52.2 to 0.52.3

**New contributors**

* @taraspos made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1330
* @HristoStoyanovYotpo made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1359

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.58.0...v0.59.0

# 0.58.0

**Bugfixes and features**

Features:
* Simplify CloudWatch API call counters by @kgeckhart

Bugs:
* Fixed issue with generated Prometheus metric name when working with AWS namespaces which have a leading special character, like `/aws/sagemaker/TrainingJobs` by @tristanburgess

Refactoring:
* Add abstraction for `GetMetricsData` processing by @kgeckhart
* `GetMetricData`: refactor QueryID generation and result mapping by @kgeckhart
* Refactored out the name-building part of `promutil.BuildNamespaceInfoMetrics()` and `promutil.BuildMetrics()` into `promutil.BuildMetricName()` by @tristanburgess
* Set initial maps size in promutil/migrate by @cristiangreco

**Dependencies**

* Bump github.com/aws/aws-sdk-go from 1.50.30 to 1.51.16
* Bump github.com/prometheus/common from 0.49.0 to 0.52.2
* Bump golang.org/x/sync from 0.6.0 to 0.7.0
* Bump the aws-sdk-v2 group with 14 updates

**New contributors**

* @tristanburgess made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1351

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.57.1...v0.58.0


# 0.57.1

**Important news and breaking changes**

* Reverted a change from 0.57.0 to fix scraping of ApiGateway resources.

**Bugfixes and features**

Bugs:
* ApiGateway: bugfix to restore FilterFunc for correct mapping of resources by @cristiangreco

**Dependencies**

## What's Changed
* Bump github.com/aws/aws-sdk-go from 1.50.26 to 1.50.30
* Bump github.com/prometheus/client_golang from 1.18.0 to 1.19.0
* Bump github.com/prometheus/common from 0.48.0 to 0.49.0
* Bump github.com/stretchr/testify from 1.8.4 to 1.9.0
* Bump the aws-sdk-v2 group

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.57.0...v0.57.1

# v0.57.0

**Important news and breaking changes**

* New job setting `includeContextOnInfoMetrics` can be used to include contextual information (account_id, region, and customTags) on "info" metrics and cloudwatch metrics. This can be particularly useful when cloudwatch metrics might not be present or when using "info" metrics to understand where your resources exist.
* No more need to add the `apigateway:GET` permissions for ApiGateway discovery jobs, as that API is not being used anymore.

**Bugfixes and features**

Features:
* Add serverless ElastiCache support by @pkubicsek-sb
* Add GWLB support by @vainiusd
* Add support for KMS metrics by @daharon
* Optionally include context labels (account, region, customTags) on info metrics with `includeContextOnInfoMetrics` by @kgeckhart
* Improve usability and performance of searchTags by @kgeckhart
* Add metric yace_cloudwatch_getmetricdata_metrics_total by @keyolk

Bugs:
* Fix race condition in scraper registry usage by @cristiangreco
* Restore default behaviour of returning nil/absent metrics as NaN by @nhinds
* Remove filtering of ApiGateway namespace resources by @cristiangreco

Refactoring:
* Refactor dimensions regexp usage for discovery jobs by @cristiangreco
* Simplify associator usage by @kgeckhart
* Update build tools and CI to go 1.22 by @cristiangreco
* Restructure fields on CloudwatchData by @kgeckhart

**Dependencies**

* Bump alpine from 3.19.0 to 3.19.1
* Bump github.com/aws/aws-sdk-go from 1.49.19 to 1.50.26
* Bump github.com/aws/smithy-go from 1.19.0 to 1.20.1
* Bump github.com/prometheus/common from 0.45.0 to 0.48.0
* Bump golang from 1.21 to 1.22
* Bump golangci/golangci-lint-action from 3.7.0 to 4.0.0
* Bump the aws-sdk-v2 group

**New contributors**

* @vainiusd made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1093
* @daharon made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/1306
* @keyolk made their first contribution in https://github.com/prometheus-community/yet-another-cloudwatch-exporter/pull/939

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.56.0...v0.57.0

# v0.56.0

**Important news and breaking changes**

* Release v0.55.0 didn't include binaries artifact due to an issue with the release pipeline.
* The `list-metrics-callback` and `max-dimensions-associator` feature flags have been removed: their behaviour is now the new default.

**Bugfixes and features**

Features:
* Add new CloudWatch API concurrency limiter by @thepalbi
* Remove feature flag `list-metrics-callback` by @cristiangreco
* Remove feature flag `max-dimensions-associator` by @cristiangreco
* Add support for AWS/Bedrock metrics by @thepalbi
* Add support for AWS/Events by @raanand-dig
* Add support for AWS/DataSync by @wkneewalden
* Add support for AWS/IPAM by @pkubicsek-sb

Bugs:
* Remove unsupported MWAA resource filter by @matej-g
* DDoSProtection: Include regionless protectedResources in us-east-1 by @kgeckhart
* aws sdk v2: ensure region is respected for all aws clients by @kgeckhart
* SageMaker: Associator buildLabelsMap to lower case EndpointName to match ARN by @GGonzalezGomez
* Update goreleaser action by @cristiangreco

Refactoring:
* Decouple config models from internal models by @cristiangreco
* Change config Validate() signature to include model conversion by @cristiangreco

**Dependencies**

* Bump actions/setup-go from 4 to 5
* Bump alpine from 3.18.3 to 3.19.0
* Bump docker/setup-buildx-action from 2 to 3
* Bump docker/setup-qemu-action from 2 to 3
* Bump github.com/aws/aws-sdk-go from 1.45.24 to 1.49.19
* Bump github.com/aws/smithy-go from 1.17.0 to 1.19.0
* Bump github.com/prometheus/client_golang from 1.16.0 to 1.18.0
* Bump github.com/prometheus/common from 0.44.0 to 0.45.0
* Bump github.com/urfave/cli/v2 from 2.25.7 to 2.27.1
* Bump golang.org/x/sync from 0.3.0 to 0.6.0
* Bump goreleaser/goreleaser-action from 4 to 5
* Bump the aws-sdk-v2 group dependencies

**New contributors**

* @GGonzalezGomez
* @wkneewalden
* @pkubicsek-sb

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.55.0...v0.56.0

# v0.55.0

**Important news and breaking changes**

* jobs of type `customNamespace`, which were deprecated in `v0.51.0`, are now **un-deprecated** due to customers' feedback
* new feature flag `always-return-info-metrics`: return info metrics even if there are no CloudWatch metrics for the resource. This is useful if you want to get a complete picture of your estate, for example if you have some resources which have not yet been used.

**Bugfixes and features**

Features:
* Un-deprecate custom namespace jobs by @cristiangreco
* scrape: Return resources even if there are no metrics by @iainlane
* kinesisanalytics application: add tags support by @raanand-dig
* Add support for AWS/ClientVPN by @hc2p
* Add support for QLDB by @alexandre-alvarengazh

Bugs:
* main: Initialise logger when exiting if needed by @iainlane

Docs:
* Create sqs.yml example file by @dverzolla

Refactoring:
* Update code to go 1.21 by @cristiangreco
* aws sdk v2 use EndpointResolverV2 by @kgeckhart
* move duplicated fields from CloudwatchData to a new JobContext by @kgeckhart

**Dependencies**

* Bump github.com/aws/aws-sdk-go from 1.44.328 to 1.45.7
* Bump the aws-sdk-v2 group with 2 updates
* Bump actions/checkout from 3 to 4 by

**New Contributors**

* @raanand-dig
* @dverzolla
* @iainlane
* @hc2p
* @alexandre-alvarengazh

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.54.1...v0.55.0


# v0.54.1

Bugs:
* sdk v2: Set RetryMaxAttempts on root config instead client options by @kgeckhart
* Match FIPS implementation between sdk v1 and sdk v2 by @kgeckhart
* Fix regex for vpc-endpoint-service by @cristiangreco

**Dependencies**

* Bump golangci/golangci-lint-action from 3.6.0 to 3.7.0
* Bump github.com/aws/aws-sdk-go from 1.44.327 to 1.44.328

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.54.0...v0.54.1

# v0.54.0

**Bugfixes and features**

Features:
* Log features enabled at startup by @cristiangreco
* Use go-kit logger and add `log.format` flag by @cristiangreco

Bugs:
* Remove tagged resource requirement from TrustedAdvisor by @kgeckhart
* Fix: RDS dashboard filtering by job value by @andriikushch
* Review dimensions regexps for APIGateway by @cristiangreco
* Fix syntax in rds.libsonnet by @andriikushch
* Fix the `FilterId` label value selection for s3 dashboard by @andriikushch
* MaxDimAssociator: loop through all mappings by @cristiangreco
* MaxDimAssociator: wrap some expensive debug logs by @cristiangreco
* MaxDimAssociator: compile AmazonMQ broker suffix regex once by @cristiangreco
* Limit number of goroutines for GetMetricData calls by @cristiangreco
* Reduce uncessary pointer usage in getmetricdata code path by @kgeckhart
* Improve perf in discovery jobs metrics to data lookup by @thepalbi
* Improve FIPS endpoints resolve logic for sdk v1 by @thepalbi

Docs:
* Add more config examples (ApiGW, SES, SNS, ECS) by @cristiangreco

Refactoring:
* Refactor clients.Cache -> clients.Factory by @kgeckhart
* dependabot: use group updates for aws sdk v2 by @cristiangreco
* Add debug logging to maxdimassociator by @cristiangreco

**Dependencies**

New dependecies:
* github.com/go-kit/log v0.2.1

Updates:
* Docker image: bump alpine from 3.18.2 to 3.18.3
* Docker image: bump golang from 1.20 to 1.21
* Bump github.com/aws/smithy-go from 1.13.5 to 1.14.2
* Bump github.com/aws/aws-sdk-go and aws-sdk-go-v2 to latest versions

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.53.0...v0.54.0

# v0.53.0

**Bugfixes and features**

Services:
* Add Auto Discovery Support For Sagemaker by @charleschangdp
* Add support for AWS/TrustedAdvisor by @cristiangreco

Bugs:
* fix(kafkaconnect): update resource filter by @cgowthaman
* Validate should fail when no roles are configured by @thepalbi
* Fix default value for nilToZero and addCloudwatchTimestamp in static job by @cristiangreco
* ddos protection: Discover resources outside us-east-1

**Dependencies**
* Bump github.com/aws/aws-sdk-go from 1.44.284 to 1.44.290
* Bump github.com/aws/aws-sdk-go-v2/service/amp from 1.16.12 to 1.16.13
* Bump github.com/aws/aws-sdk-go-v2/service/apigatewayv2 from 1.13.12 to 1.13.13
* Bump github.com/aws/aws-sdk-go-v2/service/cloudwatch from 1.26.1 to 1.26.2
* Bump github.com/aws/aws-sdk-go-v2/service/ec2 from 1.100.0 to 1.102.0
* Bump github.com/prometheus/client_golang from 1.15.1 to 1.16.0
* Bump github.com/prometheus/common from 0.43.0 to 0.44.0
* Bump github.com/urfave/cli/v2 from 2.25.6 to 2.25.7

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.52.0...v0.53.0

# v0.52.0

**Important news and breaking changes**

This releases introduces the feature flag `aws-sdk-v2` (by @kgeckhart), which changes YACE networking layer to use the AWS sdk v2 package. Read on for more details and considerations.

  * The main benefit of sdk v2 is deserialization/serialization is done via code generation vs reflection which drastically lowers memory/cpu usage for large scrape jobs
  * Considerations before enabling sdk v2:
    1. FIPS is not supported in v2 as v2 delegates all URL resolution to the sdk and AWS does not have FIPS compliant endpoints for AutoScaling API and Tagging API. The v1 implementation worked around this by hard coding FIPS URLs where they existed and using non-FIPS URLs otherwise. This work around was not ported to v2 and is unlikely to be ported.
    2. sdk v2 uses regional sts endpoints by default vs global sts which is [considered legacy by aws](https://docs.aws.amazon.com/sdkref/latest/guide/feature-sts-regionalized-endpoints.html). The `sts-region` job configuration is still respected when setting the region for sts and will be used if provided. If you still require global sts instead of regional set the `sts-region` to `aws-global`.

**Bugfixes and features**

Features:
* Discovery jobs support `recentlyActiveOnly` parameter to reduce number of old metrics returned by CloudWatch API by @PerGon
* Feature flag `aws-sdk-v2`: use the more performant AWS sdk v2 (see above section) by @kgeckhart

Services:
* Add support for API Gateway V2 by @matej-g
* Add support for MediaConvert by @theunissenne
* Add support for CWAgent by @cristiangreco
* Add support for memorydb by @glebpom

Docs:
* ALB example: use Average for ConsumedLCUs by @cristiangreco
* Update configuration.md: deprecated custom namespace jobs by @wimsymons
* Update permissions examples and docs in readme by @kgeckhart
* Add example for ElastiCache by @cristiangreco
* Update mixin readme by @cristiangreco

Bugs:
* Fix AmazonMQ Broker name dimension match by @cristiangreco
* Fix invalid GH action file and broken test case by @cristiangreco
* Fix namespace case in metrics conversion by @cristiangreco
* Make exporter options a non-global type by @kgeckhart
* Fix debug logging in discovery jobs by @cristiangreco

Refactoring:
* Refactor AWS sdk client usage to hide behind new ClientCache by @kgeckhart
* Introduce model types to replace sdk types in cloudwatch client by @kgeckhart

**Dependencies**

New dependencies:
* github.com/aws/aws-sdk-go-v2/config 1.18.27
* github.com/aws/aws-sdk-go-v2/service/amp 1.16.11
* github.com/aws/aws-sdk-go-v2/service/apigateway 1.13.13
* github.com/aws/aws-sdk-go-v2/service/autoscaling 1.28.9
* github.com/aws/aws-sdk-go-v2/service/cloudwatch 1.26.1
* github.com/aws/aws-sdk-go-v2/service/databasemigrationservice 1.25.7
* github.com/aws/aws-sdk-go-v2/service/ec2 1.100.0
* github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi 1.14.14
* github.com/aws/aws-sdk-go-v2/service/storagegateway 1.18.14

Updates:
* Bump alpine from 3.17.3 to 3.18.2
* Bump github.com/aws/aws-sdk-go from 1.44.249 to 1.44.284
* Bump github.com/prometheus/common from 0.42.0 to 0.43.0
* Bump github.com/sirupsen/logrus from 1.9.0 to 1.9.3
* Bump github.com/stretchr/testify from 1.8.2 to 1.8.4
* Bump github.com/urfave/cli/v2 from 2.25.1 to 2.25.6
* Bump golang.org/x/sync from 0.1.0 to 0.3.0
* Bump golangci/golangci-lint-action from 3.4.0 to 3.6.0

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.51.0...v0.52.0

# v0.51.0

**Important breaking changes**
* Jobs of type `customNamespace` are **deprecated** and might be removed in a future release (please reach out if you're still using this feature)

**Bugfixes and features**

Features:
* Add feature flags support by @thepalbi
* Feature flag `max-dimensions-associator`: new resource-matching algorithm for discovery jobs. It fixes metrics attribution for ECS. Please test it out and report any issue!
* Feature flag `list-metrics-callback`: reduce memory usage of ListMetrics API requests

Services:
* Add support for AWS/Usage namespace by @cristiangreco
* Fix ECS regexes by @cristiangreco

Docs:
* Add docker compose support for easier development by @thepalbi
* Add more config examples by @cristiangreco
* Review docs about embedding yace by @cristiangreco

Bugs:
* Fix for Dockerfile smell DL3007 by @grosa1

Refactoring:
* Refactor Tagging/CloudWatch clients by @cristiangreco
* CloudWatch client: split out input builders into separate file by @cristiangreco
* Refactor promutils migrate functions by @cristiangreco
* Use grafana/regexp by @cristiangreco
* Refactor implementation of getFilteredMetricDatas by @cristiangreco
* Remove uneeded Describe implementation by @kgeckhart
* Add counter to see if duplicate metrics are still a problem by @kgeckhart
* Refactor label consistency and duplicates by @kgeckhart
* Refactor GetMetricData calls in discovery jobs by @cristiangreco

**Dependencies**
* Bump github.com/aws/aws-sdk-go from 1.44.235 to 1.44.249
* Bump github.com/prometheus/common from 0.41.0 to 0.42.0

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.50.0...v0.51.0

# v0.50.0

**Important breaking changes**
* Change `UpdateMetrics` signature to accept options and return error by @cristiangreco -- if you embed YACE as a Go library this is a breaking change.

**Bugfixes and features**
Features:
* Refactor API clients concurrency handling by @cristiangreco
* Add feature flags support by @thepalbi
* Allow discovery jobs to return result even if there are no resources by @kgeckhart
* Add flag to enable pprof profiling endpoints by @cristiangreco

Services:
* Add a ResourceFilter to ElasticBeanstalk by @benbridts

Docs:
* Update config docs format by @cristiangreco

Refactoring:
* Linting: fix revive issues by @cristiangreco
* Remove extra error log when no resources are found by @kgeckhart
* Wrap debug logging in FilterMetricData by @cristiangreco
* Minor internal refactorings by @cristiangreco

**Dependencies**
* Bump actions/setup-go from 3 to 4
* Bump github.com/aws/aws-sdk-go from 1.44.215 to 1.44.235
* Bump github.com/urfave/cli/v2 from 2.25.0 to 2.25.1

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.49.2...v0.50.0

# v0.49.2

## Bugfixes and features
* Update release action to use goreleaser docker image v1.16.0

# v0.49.1

## Bugfixes and features
* Update release action to use Go 1.20

# v0.49.0

## Important breaking changes
* From now on we're dropping the `-alpha` suffix from the version number. YACE will be considered alpha quality until v1.0.0.
* The helm chart is now hosted at https://github.com/nerdswords/helm-charts, please refer to the instructions in the new repo.

## Bugfixes and features
Helm chart:
* Move helm chart out of this repo by @cristiangreco
* Update helm repo link in README.md by @cristiangreco

New services:
* Add support for Container, queue, and database metrics for MWAA by @millin
* Add support for acm-pca service by @jutley

Docs updates:
* Docs review: move "install" and "configuration" in separate docs by @cristiangreco
* Docs: Fix example config link by @matej-g
* Add example config files by @cristiangreco

Internal refactoring:
* Code refactoring: split out job and api code by @cristiangreco
* Minor refactoring of pkg/apicloudwatch and pkg/apitagging by @cristiangreco
* Refactor CW metrics to resource association logic and add tests by @thepalbi
* Wrap service filter errors by @kgeckhart

## Dependencies
* Bump github.com/aws/aws-sdk-go from 1.44.194 to 1.44.215
* Bump github.com/prometheus/common from 0.37.0 to 0.41.0
* Bump github.com/stretchr/testify from 1.8.1 to 1.8.2
* Bump github.com/urfave/cli/v2 from 2.24.3 to 2.25.0
* Bump golang.org/x/sync from 0.0.0-20220722155255-886fb9371eb4 to 0.1.0

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.48.0-alpha...v0.49.0

# v0.48.0-alpha

**Bugfixes and features**:
* Revert "Publish helm chart before releasing binaries".

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.47.0-alpha...v0.48.0-alpha

# v0.47.0-alpha

**Bugfixes and features**:
* Add Elemental MediaLive, MediaConnect to supported services by @davemt
* Add support for OpenSearch Serverless by @Hussainoxious
* Makefile: always add build version ldflags by @cristiangreco
* Publish helm chart before releasing binaries by @cristiangreco
* Build with Go 1.20 by @cristiangreco

**Dependencies**:
* Bump github.com/aws/aws-sdk-go from 1.44.192 to 1.44.194
* Bump github.com/urfave/cli/v2 from 2.24.2 to 2.24.3

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.46.0-alpha...v0.47.0-alpha

# 0.46.0-alpha

**Breaking changes**:
- If you use Yace as a library: this release changes the package
  name `pkg/logger` to `pkg/logging`.

**Bugfixes and features**:
* Fix to set logging level correctly by @cristiangreco
* ct: disable validate-maintainers by @cristiangreco

**Dependencies**:
* Bump github.com/aws/aws-sdk-go from 1.44.189 to 1.44.192

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/helm-chart-0.11.0...v0.46.0-alpha

# 0.45.0-alpha

**Breaking changes**:
- Note if you use Yace as a library: this release changes the signature
  of `config.Load` method.

**Bugfixes and features**:
* Helm chart update to customize port name by @nikosmeds
* Clear up docs and re-organize sections by @thepalbi
* Helm: add README file template by @cristiangreco
* Config parsing: emit warning messages for invalid configs by @cristiangreco
* Pre-compile dimensions regexps for supported services by @cristiangreco
* AWS/DX: add more dimension regexps by @cristiangreco

**Dependencies**:
* Bump github.com/aws/aws-sdk-go from 1.44.182 to 1.44.189
* Bump github.com/urfave/cli/v2 from 2.23.7 to 2.24.2
* Bump golangci/golangci-lint-action from 3.3.1 to 3.4.0

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.44.0-alpha...v0.45.0-alpha

# 0.44.0-alpha

**Breaking changes**:
- Note if you use Yace as a library: this release changes the packages
  and funcs exported publicly, you will need to review the imports
  (although signatures are mostly unchanged)

**Bugfixes and features**:
* Refactor code into separate packages by @cristiangreco
* Refactor list of supported services and filter funcs by @cristiangreco
* Wrap debug logging to avoid expensive operations by @cristiangreco
* Fix to use length of metrics level on customNamespace by @masshash
* feat: bump helm chart by @rasta-rocket
* feat: release helm chart when Chart.yml is updated by @rasta-rocket
* Add test for configuration of services list by @cristiangreco
* GolangCI: review linters settings by @cristiangreco

**Dependencies**:
* Bump azure/setup-helm from 1 to 3
* Bump docker/setup-buildx-action from 1 to 2
* Bump docker/setup-qemu-action from 1 to 2
* Bump github.com/aws/aws-sdk-go from 1.44.175 to 1.44.182
* Bump github.com/prometheus/client_golang from 1.13.0 to 1.14.0
* Bump helm/chart-releaser-action from 1.4.1 to 1.5.0
* Bump helm/kind-action from 1.2.0 to 1.5.0

**Full Changelog**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/compare/v0.43.0-alpha...v0.44.0-alpha

# 0.43.0-alpha

* add support to custom namespaces with their dimensions (by @arielly-parussulo)
* Optimise support for custom namespaces to use GetMetricData API (by @code-haven)
* GH workflows: run "publish" workflows only in this repo. (by @cristiangreco)
* Bump Go version to 1.19 for CI and docker image. (by @cristiangreco)
* Fix not to refer to loop variable in a goroutine (by @masshash)
* Validate tags when converting to prometheus labels (by @cristiangreco)
* Bump github.com/aws/aws-sdk-go from 1.44.127 to 1.44.167
* Bump golangci/golangci-lint-action from 3.3.0 to 3.3.1
* Bump github.com/urfave/cli/v2 from 2.23.0 to 2.23.7

# 0.42.0-alpha

* Resolve logging issue (@datsabk)
* MediaTailor - Correct dimension regex for MT (@scott-mccracken)
* Helm chart update for optional test-connection pod (@nikosmeds)
* Helm chart update to set priorityClassName (@nikosmeds)
* Bump github.com/aws/aws-sdk-go from 1.44.122 to 1.44.127
* Bump github.com/urfave/cli/v2 from 2.20.3 to 2.23.0

# 0.41.0-alpha

* Clean up unused variables. (@cristiangreco)
* Fix typo: sts-endpoint should be sts-region. (@cristiangreco)
* Enabled Managed prometheus metrics (@datsabk)
* Add support for AWS Kafka Connect (@cgowthaman)
* Import CloudWatch mixin. (@jeschkies)
* main.go refactoring: define cmd action as a separate func. (@cristiangreco)
* Add support for EMR Serverless (@cgowthaman)

# 0.40.0-alpha
* Fix typo in Charts.yml (@yasharne)
* Subcommand `verify-config` actually validates the config file. (@cristiangreco)
* Add dimensions regex for AmazonMQ. (@cristiangreco)
* Fix metrics with additional dimensions being not being scraped. (@cristiangreco)
* Remove unused code, add test for RemoveDuplicateMetrics. (@cristiangreco)
* Bump github.com/sirupsen/logrus
* Bump github.com/urfave/cli/v2
* Bump github.com/aws/aws-sdk-go
* Bump actions/setup-python

# 0.39.0-alpha
* Improve code quality and unblock this release (cristiangreco)
* Add helm chart (vkobets)
* Fix DX metrics (paulojmdias)
* Fix searchTags and bad dimension name (femiagbabiaka)
* Handle empty list in filter metric tests (mtt88)
* Add AWS Elemental MediaTailor support (scott-mccracken)
* Support storagegateway metrics (sedan07)
* Filter api gateway resources to skip "stages" (ch4rms)
* Bump aws-sdk, urfave/cli, prometheus/client_golang

# 0.38.0-alpha

* Set max page size for tagging API requests (#617)
* Build with Go 1.18

# 0.37.0-alpha
* New config `dimensionNameRequirements` allows autodiscovery jobs to only
  fetch metrics that include specified dimensions (jutley)
* Update deps

# 0.36.2-alpha
* Cost Reduction - Use less API requests if no tagged resources are found (cristiangreco)
* Update deps

# 0.36.1-alpha
* Use structured logs for logging interface (kgeckhart)

# 0.36.0-alpha

* *BREAKING CHANGE FOR LIBRARY USERS* Major refactoring of usage of logging library (kgeckhart)
* Minor update of deps and security patches (urfave/cli/v2, golangci/golangci-lint-action, github.com/prometheus/client_golang, github.com/stretchr/testify, github.com/aws/aws-sdk-go
* Updates of Readme (markwallsgrove)

# 0.35.0-alpha
* Update dependencies
* Improve / Document way how to use the exporter as external library (kgeckhart)
* Refactor label consistency (kgeckhart)
* Add suppot for vpc-endpoint (AWS/PrivateLinkEndpoints) (aleslash)
* Add support for vpc-endpoint-service (AWS/PrivateLinkServices) (aleslash)

# 0.34.0-alpha
* Update dependencies
* Add weekly dependabot updates (jylitalo)
* Add support for regional sts endpoints (matt-mercer)
* Add multi-arch docker build (charlie-haley)

New services
* Add global accelerator support (charlie-haley)
* Add AppStream support (jhuesemann)
* Add Managed Apache Airflow support (sdenham)
* Add KinesisAnalytics support (gumpt)

Bug Fixes
* Fix targetgroup arn lookup (domcyrus)
* Fix WorkGroup Dimension are not showing in Athena Metrics (sahajavidya)
* Improve regex performance (kgeckhart)
* Fix prometheus reload causing a goroutine leak (gumpt / cristiangreco)

Docs
* Added help for new contributors (aleslash)

# 0.33.0-alpha
* Add /healthz route which allows to deploy more secure with helm (aleslash)
* Read DMS replication instance identifier from the DMS API (nhinds)

# 0.32.0-alpha
* [BREAKING] Fix the calculation of start and end times for GetMetricData (csquire)
```
floating-time-window is now replaced with roundingPeriod

Specifies how the current time is rounded before calculating start/end times for CloudWatch GetMetricData requests. This rounding is optimize performance of the CloudWatch request. This setting only makes sense to use if, for example, you specify a very long period (such as 1 day) but want your times rounded to a shorter time (such as 5 minutes). to For example, a value of 300 will round the current time to the nearest 5 minutes. If not specified, the roundingPeriod defaults to the same value as shortest period in the job.
```
* Improve testing / linting (cristiangreco)
* Verify cli parameters and improve cli parsing (a0s)
* Allow to configure yace cli parameters via env variables (a0s)
* Improve error handling of cloudwatch (matthewnolf)
* Add support for directconnect and route53 health checks
* Improve throttling handling to AWS APIs (anilkun)
* Add issue templates to improve support (NickLarsenNZ)
* Allow setting default values for statistics (surminus)
* Fix apigateway method and resouce dimension bug (aleslash)

Thanks a lot to all contributors! - Lovely to see so much efforts especially in testing
to get this project more and more stable. - I know we are far away from a nice tested
code base but we are improving in the right direction and I really love to see all
of your efforts there. It is really appreciated from my side.

I just contacted AWS to get some open source credits so we can build some kind of
end to end tests. This shoud allow us to find tricky bugs earlier and not only when we ship
things.

Love to all of you, Thomas!

# 0.31.0-alpha
* [BREAKING] Decoupled scraping is now default. Removed code which allowed to use scraper without it.
```
# Those flags are just ignored
-decoupled-scraping=false
-decoupled-scraping=true
```
* [BREAKING] Small timeframes of scraping can be used again now. In the past yace decided the scraping
  interval based on config. This magic was removed for simplicity.
```
# In the past this would have in some cases still set --scraping-interval 600
--scraping-interval 10
# Now it really would scrape every 10 seconds which could introduce big API costs. So please watch
# your API requests!
--scraping-interval 10
```
* Fix problems with start/endtime of scrapes (klarrio-dlamb)
* Add support for Database Migration Service metrics
* Allow to hotreload config via /reload (antoniomerlin)

# 0.30.1-alpha
* *SECURITY* Fix issue with building binaries. Please update to mitigate (https://nvd.nist.gov/vuln/detail/CVE-2020-14039)
* Thanks jeason81 for reporting this security incident!

# 0.30.0-alpha
* *BREAKING* Introduce new version field to config file (jylitalo)
```
# Before
discovery:
  jobs:
# After
apiVersion: v1alpha1
discovery:
  jobs:
```
* [BUG] Fix issues with nilToZero (eminugurkenar)
* [BUG] Fix race condition setting end time for discovery jobs (cristiangreco)
* Simplify session creation code (jylitalo)
* Major improvement of aws discovery code (jylitalo)
* Major rewrite of the async scraping logic (rabunkosar-dd)
* Add support for AWS/ElasticBeanstalk (andyzasl)
* Upgrade golang to 1.17
* Upgrade golang libraries to newest versions

# 0.29.0-alpha
Okay, private things settled. We have a new organisation for
the project. Lets boost it and get the open PRs merged!
This version is like 0.28.0-alpha but docker images hosted on ghcr.io
and published via new github organisation nerdswords. Find
details [here](https://medium.com/@IT_Supertramp/reorganizing-yace-79d7149b9584).

Thanks to all there waiting and using the product! :)

- *BREAKING CHANGE* Using a new docker registry / organisation:
```yaml
# Before
quay.io/invisionag/yet-another-cloudwatch-exporter:v0.29.0-alpha
# Now
ghcr.io/nerdswords/yet-another-cloudwatch-exporter:v0.29.0-alpha
```

# 0.28.0-alpha
Sorry folks, I currently struggle a little bit
to get things merged fast due to a lot of private
stuff. Really appreciate all your PRs and
hope to get the bigger ones (which are sadly
still not merged yet) into next release.

Really appreciate any person working on this
project! - Have a nice day :)

- *BREAKING CHANGE* Added support for specifying an External ID with IAM role Arns (cristiangreco)
```yaml
# Before
discovery:
  jobs:
  - type: rds
    roleArns:
    - "arn:aws:iam::123456789012:role/Prometheus"
# After
discovery:
  jobs:
  - type: rds
    roles:
    - roleArn: "arn:aws:iam::123456789012:role/Prometheus"
      externalId: "shared-external-identifier" # optional
```
- Add alias for AWS/Cognito service (tohjustin)
- Fix logic in dimensions for Transit Gateway Attachments (rhys-evans)
- Fix bug with scraping intervals (boazreicher)
- Support arm64 builds (alias-dev)
- Fix IgnoreLength logic (dctrwatson)
- Simplify code base (jylitalo)
- Simplify k8s deployments for new users (mahmoud-abdelhafez)
- Handle metrics with '%' in their name (darora)
- Fix classic elb name (nhinds)
- Skip metrics in edge cases (arvidsnet)

Freshly shipped new integrations:
- Certificate Manager (mksh)
- WorkSpaces (kl4w)
- DDoSProtection / Shield (arvidsnet)

# 0.27.0-alpha

- Make exporter a library. (jeschkies)
- Add CLI option to validate config file (zswanson)
- Fix multidimensional static metric (nmiculinic)
- Fix scrapes running in EKS fail after first scrape (rrusso1982)
- Fix Docker build (jeschkies)
- Allow to use this project in China (insectme)
- Fix error retrieving kafka metrics (friedrichg)

Freshly integrated:
- Add AWS/NetworkFirewall (rhys-evans)
- Add AWS/Cassandra (bjhaid)
- Add AWS/AmazonMQ (saez0pub)
- Add AWS/Athena (haarchri)
- Add AWS/Neptune (benjaminaaron)

Thanks to doc fixes: calvinbui

# 0.26.1-alpha / 0.26.2-alpha / 0.26.3-alpha

- Fix CI issue

# 0.26.0-alpha

- *BREAKING CHANGE* Removed a need to use static dimensions in dynamic jobs in cases, when they cannot be parsed from ARNs (AndrewChubatiuk)
    ```
      # Before
      metrics:
      - name: NumberOfObjects
        statistics:
          - Average
        additionalDimensions:
          - name: StorageType
            value: AllStorageTypes
      # After
      metrics:
      - name: NumberOfObjects
        statistics:
          - Average
    ```
* *BREAKING CHANGE* Use small case for searchTags config option (AndrewChubatiuk)
    ```
    # Before
    searchTags:
    - Key: type
      Value: public
    # After
    searchTags:
    - key: type
      value: public
      ```
* *BREAKING CHANGE* CloudFront renamed from `cf` to `cloudfront`
    ```
    # Before
    - type: cf
    # After
    - type: cloudfront
      ```

- Added regular expressions to parse dimensions from resources (AndrewChubatiuk)
- Added option to use floating time windows (zqad)
- Added CLI option to validate config file (zswanson)
- Added AWS network Firewall (rhys-evans)
- Fixed multidimensional static metric (nmiculinic)
- Tidy up code (jylitalo)

# 0.25.0-alpha

- *BREAKING CHANGE* Use NaN as default if AWS returns nil (arnitolog)
- Add autodiscovery for AWS/EC2Spot (singhjagmohan1000)
- Add autodiscovery for DocumentDB (haarchri)
- Add autodiscovery for GameLift (jp)
- Added support for fips compliant endpoints (smcavallo)
- Update deps and build with golang 1.15 (smcavallo)

# 0.24.0-alpha

- Add API Gateway IAM info to README (Botono)
- Fix sorting of datapoints, add test util functions (Botono)
- Fix missing DataPoints and improve yace in various ways (vishalraina)
- Added Github action file to basic validation of incoming PR (vishalraina)
- Fix info metrics missing (goya)
- Add rds db clusters (goya)
- Fix missing labels (goya)

# 0.23.0-alpha

- Add sampleCount statistics (udhos)
- Add WAFv2 support (mksh)

# 0.22.0-alpha

- Fix alb issues (reddoggad)
- Add nlb support (reddoggad)

# 0.21.0-alpha

- Big tidy up of code, remove old methods and refactor used ones (jylitalo)
- Fix crashes where labels are not collected correctly (rrusso1982)
- Fix pointer bug causing metrics to be missing (jylitalo)
- Allow more then 25 apigateways to be discovered (udhos)

# 0.20.0-alpha

- Add api-gateway support (smcavallo)
- Improve metrics validation (jylitalo)
- Fix metrics with '<', '>' chars

# 0.19.1-alpha

- Remove error during build

# 0.19.0-alpha
Wow what a release. Thanks to all contributors. This is
our biggest release and it made me a lot of fun to see all those
contributions. From small doc changes (love those) to major rewrites
of big components or new complex features. Thanks!

* *BREAKING CHANGE* Add support for multiple roleArns (jylitalo)
```yaml
# Before
---
discovery:
  jobs:
  - type: rds
    roleArn: "arn:aws:iam::123456789012:role/Prometheus"
# After
discovery:
  jobs:
  - type: rds
    roleArns:
    - "arn:aws:iam::123456789012:role/Prometheus"
```
* Upgrade golang from 1.12 to 1.14
* Major linting of code and improving global code quality. (jylitalo)
* Improve logging (jylitalo)
* Add config validation. (daviddetorres)
* Added support for tags with '@' char included (afroschauer )
* Added Transit Gateway Attachment Metrics (rhys-evans)
* Fix information gathering if no data is retrieved by cloudwatch (daviddetorres)
* Improve docs (calvinbui)
* Add redshift support (smcavallo)
* Allow easier configuration through adding period / addCloudwatchTimestamp setting additionally
  to job level. (rrusso1982)
* Add initial unit tests (smcavallo)
* Add new configuration to allow snake case labels (rrusso1982)
* Fix complex metric dimension bug (rrusso1982)
* Upgrade golang packages (smcavallo)
* Set up correct partition for ASG for AWS China and GovCloud Regions (smcavallo)
* Add ability to set custom tags to discovery job metrics (goya)

# 0.18.0-alpha
* *BREAKING CHANGE* Add support for multiple regions (goya)
```yaml
# Before
---
discovery:
  jobs:
  - type: rds
    region: eu-west-1
# After
discovery:
  jobs:
  - type: rds
    regions:
    - eu-west-1
```
* Fix missing alb target group metrics (abhi4890 )
* Added support for step functions (smcavallo)

# 0.17.0-alpha
* Added support for sns / firehose (rhys-evans)
* Added support for fsx / appsync (arnitolog)

# 0.16.0-alpha
* Hugh rewrite: Decouple scraping and serving metrics. Thanks so much daviddetorres!
* *BREAKING CHANGE* Decoupled scraping and set scraping interval to 5 minutes.
```
The flag 'decoupled-scraping' makes the exporter to scrape Cloudwatch metrics in background in fixed intervals, in stead of each time that the '/metrics' endpoint is fetched. This protects from the abuse of API requests that can cause extra billing in AWS account. This flag is activated by default.

If the flag 'decoupled-scraping' is activated, the flag 'scraping-interval' defines the seconds between scrapes. Its default value is 300.
```
* Hugh rewrite: Rewrite of metric gathering to reduce API Limit problems. Thanks so much daviddetorres!
* Improvment of ALB data gathering and filtering (daviddetorres)
* Detect and fix bug after merge (deanrock)
* Add cloudfront support (mentos1386)

# 0.15.0-alpha
* Fixed docker run command in README.md (daviddetorres)
* Added support for Nat Gateway / Transit Gateway / Route 53 Resolver (j-nix)
* Added support for ECS/ContainerInsights (daviddetorres)
* Fix pagination for getMetricList (eminugurkenar)

# 0.14.7-alpha
* Change logging to json format (bheight-Zymergen)

# 0.14.6-alpha
* Add support for kafka (eminugurkenar)
* Add structured json logging (bheight-Zymergen)
* Increase code readability (bheight-Zymergen)
* Fix ecs scraping bug (rabunkosar-dd)
* Fix aws cloudwatch period bug (rabunkosar-dd)

# 0.14.5-alpha
* Fix sts api calls without specifying a region (nhinds)
* Update aws-sdk to v1.25.21 (nhinds)

# 0.14.4-alpha
* Fix github actions (nhinds)
* Update aws-sdk-go (deanrock)
* Avoid appending to a shared dimensions variable from inside a loop (nhinds)
* Remove hardcoded StorageType dimension from S3 metric (nhinds)

# 0.14.3-alpha
* Fix problems and crashes with ALBs and ELBs (Deepak1100)

# 0.14.2-alpha
* **BREAKING** Changing user in Docker image to be non root to adhere to potential security requirements. (whitlekx)
* Fix prometheus metric bug with new services with '-' e.g. ecs-svc.

# 0.14.1-alpha
* Was accidentally with code from 01.14.0-alpha released.

# 0.14.0-alpha
* **BREAKING** Default command in Dockerfile is changed to yace. This removes the need to add yace as command.
```yaml
# Before
        command:
          - "yace"
          - "--config.file=/tmp/config.yml"
# After
        args:
          - "--config.file=/tmp/config.yml"
```
* Add support for Elastic MapReduce (nhinds)
* Add support for SQS - (alext)
* Add support for ECS Services as ecs-svc
* Add support for NLB
* Add retries to cloudwatch api calls (Deepak1100)
* Fix dimension labels for static jobs (alext)

# 0.13.7
* Add region as exported label to metrics

# 0.13.6
* Fix errors with "=" in tags (cdchris12)
* Add curl to container for easy debugging (cdchris12)

# 0.13.5-alpha
* Limit concurrency of aws calls

# 0.13.4
* Add Autoscaling group support (wjam)
* Fix strange AWS namespace bug for static exports (AWS/EC2/API)
* Add warning if metric length of less than 300s is configured / Interminent metrics

# 0.13.3
* Fix ALB problems. Target Group metrics are now exported as aws_albtg
```
aws_albtg_request_count_sum{dimension_LoadBalancer="app/Test-ALB/fec38de4cf0cacb1",dimension_TargetGroup="targetgroup/Test/708ecba11979327b",name="arn:aws:elasticloadbalancing:eu-west-1:237935892384916:targetgroup/Test/708dcba119793234"} 0
```

# 0.13.2
* CI problem

# 0.13.1-alpha
* **BREAKING** For some metrics `cloudwatch:ListMetrics` iam permissions are needed. Please update your role!
* **BREAKING** Add 'v' to indicate it is a version number in docker tag / version output
```
# Before
 image: quay.io/invisionag/yet-another-cloudwatch-exporter:0.13.0
# After
 image: quay.io/invisionag/yet-another-cloudwatch-exporter:v0.13.0
```
* Use golang 1.12.0 to build
* Use goreleaser to release
* Update aws dependencies
* Use github actions as CI
* Migrate dependency management to golang modules

# 0.13.0-alpha
* **BREAKING** For some metrics `cloudwatch:ListMetrics` iam permissions are needed. Please update your role!
* **BREAKING** As adding cloudwatch timestamp breaks some metrics I decided to not set it as default anymore.
This should make it easier for new users to have fun with this project.
It fixes for some users `non-histogram and non-summary metrics should not have "_sum" suffix` bug.
```yaml
# Before
  metrics:
    - name: FreeStorageSpace
      disableTimestamp: true
# After
  metrics:
    - name: FreeStorageSpace

# Before
  metrics:
    - name: FreeStorageSpace
# After
  metrics:
    - name: FreeStorageSpace
      useCloudwatchTimestamp: true
```
* Add ability to specify additional dimensions on discovery jobs e.g. for BucketSizeBytes metrics on S3 (abuchananTW)
* Fix incorrect dimension value in case of alb in discovery config (GeeksWine)
* Add CLI command to debug output
* Add DynamoDB support

# 0.12.0 / 0.12.0-alpha
* **BREAKING** Add the exact timestamps from CloudWatch to the exporter Prometheus metrics (LeePorte)
* Add a new option `disableTimestamp` to not include a timestamp for a specific metric (it can be useful for sparse metrics, e.g. from S3) (LeePorte)
* Add support for kinesis (AndrewChubatiuk)

# 0.11.0
* **BREAKING** Add snake_case to prometheus metrics (sanchezpaco)
```yaml
# Before
aws_elb_requestcount_sum
# After
aws_elb_request_count_sum
```

* Add optional delay setting to scraping (Deepak1100)
```yaml
period: 60
length: 900
delay: 300
```

# 0.10.0
* Reduce usage of listMetrics calls (nhinds)
* Add support of iam roles (nhinds)
* Add optional roleArn setting, which allows scraping with different roles e.g. pull data from mulitple AWS accounts using cross-acount roles (nhinds)
```yaml
    metrics:
      - name: FreeStorageSpace
        roleArn: xxx
        statistics:
        - 'Sum'
        period: 600
        length: 60
```

# 0.9.0
* Add lambda support (nhinds)
* Fix support for listing multiple statistics per metric (nhinds)
* Add tag labels on metrics for easy querying (nhinds)
```
# Before
aws_ec2_cpuutilization_average + on (name) group_left(tag_Name) aws_ec2_info

# After, now name tags are on metrics and no grouping needed
aws_ec2_cpuutilization_average
```

* **BREAKING** Change config syntax. Now you can define tags which are exported as labels on metrics.
Before:

```yaml
discovery:
  - region: eu-west-1
    type: "es"
    searchTags:
      - Key: type
        Value: ^(easteregg|k8s)$
    metrics:
      - name: FreeStorageSpace
        statistics:
        - 'Sum'
        period: 600
        length: 60
```

New Syntax with optional exportedTagsOnMetrics:
```yaml
discovery:
  exportedTagsOnMetrics:
    ec2:
      - Name
  jobs:
    - region: eu-west-1
      type: "es"
      searchTags:
        - Key: type
          Value: ^(easteregg|k8s)$
      metrics:
        - name: FreeStorageSpace
          statistics:
          - 'Sum'
          period: 600
          length: 60
```

# 0.8.0
* Added VPN connection metrics (AndrewChubatiuk)
* Added ExtendedStatistics / percentiles (linefeedse)
* Added Average Statistic (AndrewChubatiuk)

# 0.7.0-alpha
* ALB Support (linefeedse)
* Custom lables for static metrics

Example
```yaml
static:
  - namespace: AWS/AutoScaling
    region: eu-west-1
    dimensions:
     - name: AutoScalingGroupName
       value: Test
    customTags:
      - Key: CustomTag
        Value: CustomValue
    metrics:
      - name: GroupInServiceInstances
        statistics:
        - 'Minimum'
        period: 60
        length: 300
```

# 0.6.1
* Sanitize colons in tags (linefeedse)

# 0.6.0 / 0.6.0-alpha
* **BREAKING**: Period/length uses now seconds instead of minutes
* **BREAKING**: Config file uses new syntax to support static
* Support of --debug flag which outputs some dev debug informations
* Support of metrics who are not included in tags api (e.g. autoscaling metrics)

Before
```yaml
jobs:
  - discovery:
      region: eu-west-1
      metrics:
        - name: HealthyHostCount
          statistics:
          - 'Minimum'
          period: 60
          length: 300
```

New Syntax:
```yaml
discovery:
  - region: eu-west-1
    type: elb
    searchTags:
      - Key: KubernetesCluster
        Value: production
    metrics:
      - name: HealthyHostCount
        statistics:
        - 'Minimum'
        period: 60
        length: 300
static:
  - namespace: AWS/AutoScaling
    region: eu-west-1
    dimensions:
     - name: AutoScalingGroupName
       value: Test
    metrics:
      - name: GroupInServiceInstances
        statistics:
        - 'Minimum'
        period: 60
        length: 300
```

# 0.5.0
* Support of EFS - Elastic File System
* Support of EBS - Elastic Block Storage

# 0.4.0
* **BREAKING**: Config file uses list as statistics config option,
this should reduce api calls for more than one statistics.

Before:
```yaml
jobs:
  - discovery:
    metrics:
        statistics: 'Maximum'
```
After:
```yaml
jobs:
  - discovery:
    metrics:
        statistics:
        - 'Maximum'
```
* Start to track changes in CHANGELOG.md
* Better error handling (discordianfish)
* Increase speed, not only each jobs threaded but now each metric
* Add s3 support
* Fix potential race condition during cloudwatch access
* Fix bug ignoring period in cloudwatch config
* Use interfaces for aws access and prepare code for unit tests
* Implement minimum, average, maximum, sum for cloudwatch api
* Implement way to handle multiple data returned by cloudwatch
* Update go dependencies
