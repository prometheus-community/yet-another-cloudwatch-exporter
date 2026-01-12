# Configuration

YACE has two configuration mechanisms:

- [command-line flags](#command-line-flags)
- [yaml configuration file](#yaml-configuration-file)

The command-line flags configure things which cannot change at runtime, such as the listen port for the HTTP server. The yaml file is used to configure scrape jobs and can be reloaded at runtime. The configuration file path is passed to YACE through the `-config.file` command line flag.

## Command-line flags

Command-line flags are used to configure settings of the exporter which cannot be updated at runtime.

All flags may be prefixed with either one hypen or two (i.e., both `-config.file` and `--config.file` are valid).

| Flag | Description | Default value |
| --- | --- | --- |
| `-listen-address` | Network address to listen to | `127.0.0.1:5000` |
| `-config.file` | Path to the configuration file | `config.yml` |
| `-log.format` | Output format of log messages. One of: [logfmt, json] | `json` |
| `-log.level` | Log at selected level. One of: [debug, info, warn, error] | `info` |
| `-fips` | Use FIPS compliant AWS API | `false` |
| `-cloudwatch-concurrency` | Maximum number of concurrent requests to CloudWatch API | `5` |
| `-cloudwatch-concurrency.per-api-limit-enabled` | Enables a concurrency limiter, that has a specific limit per CloudWatch API call. | `false` |
| `-cloudwatch-concurrency.list-metrics-limit` | Maximum number of concurrent requests to CloudWatch `ListMetrics` API. Only applicable if `per-api-limit-enabled` is `true`. | `5` |
| `-cloudwatch-concurrency.get-metric-data-limit` | Maximum number of concurrent requests to CloudWatch `GetMetricsData` API. Only applicable if `per-api-limit-enabled` is `true`. | `5` |
| `-cloudwatch-concurrency.get-metric-statistics-limit` | Maximum number of concurrent requests to CloudWatch `GetMetricStatistics` API. Only applicable if `per-api-limit-enabled` is `true`. | `5` |
| `-tag-concurrency` | Maximum number of concurrent requests to Resource Tagging API | `5` |
| `-scraping-interval` | Seconds to wait between scraping the AWS metrics | `300` |
| `-metrics-per-query` | Number of metrics made in a single GetMetricsData request | `500` |
| `-labels-snake-case`  | Output labels on metrics in snake case instead of camel case | `false` |
| `-profiling.enabled` | Enable the /debug/pprof endpoints for profiling | `false` |

## YAML configuration file

To specify which configuration file to load, pass the `-config.file` flag at the command line. The file is written in the YAML format, defined by the scheme below. Brackets indicate that a parameter is optional.

Below are the top level fields of the YAML configuration file:

```yaml
# Configuration file version. Must be set to "v1alpha1" currently.
apiVersion: v1alpha1

# STS regional endpoint (optional)
[ sts-region: <string>]

# Note that at least one of the following blocks must be defined.

# Configurations for jobs of type "auto-discovery"
discovery: <discovery_jobs_list_config>

# Configurations for jobs of type "static"
static:
  [ - <static_job_config> ... ]

# Configurations for jobs of type "custom namespace"
customNamespace:
  [ - <custom_namespace_job_config> ... ]
```

Note that while the `discovery`, `static` and `customNamespace` blocks are all optionals, at least one of them must be defined.

### `discovery_jobs_list_config`

The `discovery_jobs_list_config` block configures jobs of type "auto-discovery".

> Note: Only [tagged resources](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html) are discovered.

```yaml
# List of tags per service to export to all metrics
[exportedTagsOnMetrics: <exported_tags_config> ]

# List of "auto-discovery" jobs
jobs:
  [ - <discovery_job_config> ... ]
```

### `discovery_job_config`

The `discovery_job_config` block specifies the details of a job of type "auto-discovery".

```yaml
# List of AWS regions
regions:
  [ - <string> ... ]

# Cloudwatch service alias ("alb", "ec2", etc) or namespace name ("AWS/EC2", "AWS/S3", etc)
type: <string>

#  List of IAM roles to assume (optional)
roles:
  [ - <role_config> ... ]

# List of Key/Value pairs to use for tag filtering (all must match). 
# The key is the AWS Tag key and is case-sensitive  
# The value will be treated as a regex
searchTags:
  [ - <search_tags_config> ... ]

# Custom tags to be added as a list of Key/Value pairs
customTags:
  [ - <custom_tags_config> ... ]

# List of metric dimensions to query. Before querying metric values, the total list of metrics will be filtered to only those that contain exactly this list of dimensions. An empty or undefined list results in all dimension combinations being included.
dimensionNameRequirements:
  [ - <string> ... ]

# Specifies how the current time is rounded before calculating start/end times for CloudWatch GetMetricData requests.
# This rounding is optimize performance of the CloudWatch request.
# This setting only makes sense to use if, for example, you specify a very long period (such as 1 day) but want your times rounded to a shorter time (such as 5 minutes). For example, a value of 300 will round the current time to the nearest 5 minutes. If not specified, the roundingPeriod defaults to the same value as shortest period in the job.
[ roundingPeriod: <int> ]

# Passes down the flag `--recently-active PT3H` to the CloudWatch API. This will only return metrics that have been active in the last 3 hours.
# This is useful for reducing the number of metrics returned by CloudWatch, which can be very large for some services. See AWS Cloudwatch API docs for [ListMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_ListMetrics.html) for more details.
[ recentlyActiveOnly: <boolean> ]

# Can be used to include contextual information (account_id, region, and customTags) on info metrics and cloudwatch metrics. This can be particularly 
# useful when cloudwatch metrics might not be present or when using info metrics to understand where your resources exist
[ includeContextOnInfoMetrics: <boolean> ]

# (optional) This is an experimental feature that can be used to enable enhanced metrics for specific services within this discovery job. It might be subject to changes in future releases.
enhancedMetrics:
    [ - <enhanced_metrics_config> ... ]

# List of statistic types, e.g. "Minimum", "Maximum", etc (General Setting for all metrics in this job)
statistics:
  [ - <string> ... ]

# Statistic period in seconds (General Setting for all metrics in this job)
[ period: <int> ]

# How far back to request data for in seconds (General Setting for all metrics in this job)
[ length: <int> ]

# If set it will request metrics up until `current_time - delay` (General Setting for all metrics in this job)
[ delay: <int> ]

# Return 0 value if Cloudwatch returns no metrics at all. By default `NaN` will be reported (General Setting for all metrics in this job)
[ nilToZero: <boolean> ]

# Export the metric with the original CloudWatch timestamp (General Setting for all metrics in this job)
[ addCloudwatchTimestamp: <boolean> ]

# Enables the inclusion of past metric data points from the CloudWatch response if available.
# This is useful when a metric is configured with a 60-second period and a 300-second duration, ensuring that all
# five data points are exposed at the metrics endpoint instead of only the latest one.
# Note: This option requires `addCloudwatchTimestamp` to be enabled.
# The metric destination must support out of order timestamps, see https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tsdb
# (General Setting for all metrics in this job)
[ exportAllDataPoints: <boolean> ]

# List of metric definitions
metrics:
  [ - <metric_config> ... ]
```

Example config file:

```yaml
apiVersion: v1alpha1
sts-region: eu-west-1
discovery:
  exportedTagsOnMetrics:
    kafka:
      - Name
  jobs:
  - type: kafka
    regions:
      - eu-west-1
    searchTags:
      - key: env
        value: dev
    metrics:
      - name: BytesOutPerSec
        statistics:
        - Average
        period: 600
        length: 600
```

### `static_job_config`

The `static_job_config` block configures jobs of type "static".

```yaml
# Name of the job (required)
name: <string>

# CloudWatch namespace
namespace: <string>

# List of AWS regions
regions:
  [ - <string> ...]

# List of IAM roles to assume (optional)
roles:
  [ - <role_config> ... ]

# Custom tags to be added as a list of Key/Value pairs
customTags:
  [ - <custom_tags_config> ... ]

# CloudWatch metric dimensions as a list of Name/Value pairs
dimensions: [ <dimensions_config> ]

# List of metric definitions
metrics:
  [ - <metric_config> ... ]
```

Example config file:

```yaml
apiVersion: v1alpha1
sts-region: eu-west-1
static:
  - namespace: AWS/AutoScaling
    name: must_be_set
    regions:
      - eu-west-1
    dimensions:
     - name: AutoScalingGroupName
       value: MyGroup
    customTags:
      - key: CustomTag
        value: CustomValue
    metrics:
      - name: GroupInServiceInstances
        statistics:
        - Minimum
        period: 60
        length: 300
```

### `custom_namespace_job_config`

The `custom_namespace_job_config` block configures jobs of type "custom namespace".

```yaml
# Name of the job (required)
name: <string>

# CloudWatch namespace
namespace: <string>

# List of AWS regions
regions:
  [ - <string> ...]

#  List of IAM roles to assume (optional)
roles:
  [ - <role_config> ... ]

# Custom tags to be added as a list of Key/Value pairs
customTags:
  [ - <custom_tags_config> ... ]

# List of metric dimensions to query. Before querying metric values, the total list of metrics will be filtered to only those that contain exactly this list of dimensions. An empty or undefined list results in all dimension combinations being included.
dimensionNameRequirements:
  [ - <string> ... ]

# Specifies how the current time is rounded before calculating start/end times for CloudWatch GetMetricData requests.
# This rounding is optimize performance of the CloudWatch request.
# This setting only makes sense to use if, for example, you specify a very long period (such as 1 day) but want your times rounded to a shorter time (such as 5 minutes). For example, a value of 300 will round the current time to the nearest 5 minutes. If not specified, the roundingPeriod defaults to the same value as shortest period in the job.
[ roundingPeriod: <int> ]

# Passes down the flag `--recently-active PT3H` to the CloudWatch API. This will only return metrics that have been active in the last 3 hours.
# This is useful for reducing the number of metrics returned by CloudWatch, which can be very large for some services. See AWS Cloudwatch API docs for [ListMetrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_ListMetrics.html) for more details.
[ recentlyActiveOnly: <boolean> ]

# List of statistic types, e.g. "Minimum", "Maximum", etc (General Setting for all metrics in this job)
statistics:
  [ - <string> ... ]

# Statistic period in seconds (General Setting for all metrics in this job)
[ period: <int> ]

# How far back to request data for in seconds (General Setting for all metrics in this job)
[ length: <int> ]

# If set it will request metrics up until `current_time - delay` (General Setting for all metrics in this job)
[ delay: <int> ]

# Return 0 value if Cloudwatch returns no metrics at all. By default `NaN` will be reported (General Setting for all metrics in this job)
[ nilToZero: <boolean> ]

# Export the metric with the original CloudWatch timestamp (General Setting for all metrics in this job)
[ addCloudwatchTimestamp: <boolean> ]

# Enables the inclusion of past metric data points from the CloudWatch response if available.
# This is useful when a metric is configured with a 60-second period and a 300-second duration, ensuring that all
# five data points are exposed at the metrics endpoint instead of only the latest one.
# Note: This option requires `addCloudwatchTimestamp` to be enabled.
# The metric destination must support out of order timestamps, see https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tsdb
# (General Setting for all metrics in this job)
[ exportAllDataPoints: <boolean> ]

# List of metric definitions
metrics:
  [ - <metric_config> ... ]
```

Example config file:

```yaml
apiVersion: v1alpha1
sts-region: eu-west-1
customNamespace:
  - name: customEC2Metrics
    namespace: CustomEC2Metrics
    regions:
      - us-east-1
    metrics:
      - name: cpu_usage_idle
        statistics:
          - Average
        period: 300
        length: 300
        nilToZero: true
      - name: disk_free
        statistics:
          - Average
        period: 300
        length: 300
        nilToZero: true
```

### `metric_config`

Some settings at the job level are overridden by settings at the metric level.
This allows for a specific setting to override a general setting.

```yaml
# CloudWatch metric name
name: <string>

# List of statistic types, e.g. "Minimum", "Maximum", etc. (Overrides job level setting)
statistics:
  [ - <string> ... ]

# Statistic period in seconds (Overrides job level setting)
[ period: <int> ]

# How far back to request data for in seconds (Overrides job level setting)
[ length: <int> ]

# If set it will request metrics up until `current_time - delay` (Overrides job level setting)
[ delay: <int> ]

# Return 0 value if Cloudwatch returns no metrics at all. By default `NaN` will be reported (Overrides job level setting)
[ nilToZero: <boolean> ]

# Export the metric with the original CloudWatch timestamp (Overrides job level setting)
[ addCloudwatchTimestamp: <boolean> ]

# Enables the inclusion of past metric data points from the CloudWatch response if available.
# This is useful when a metric is configured with a 60-second period and a 300-second duration, ensuring that all
# five data points are exposed at the metrics endpoint instead of only the latest one.
# Note: This option requires `addCloudwatchTimestamp` to be enabled.
# The metric destination must support out of order timestamps, see https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tsdb
# (General Setting for all metrics in this job)
[ exportAllDataPoints: <boolean> ]
```

Notes:
- Available statistics: `Maximum`, `Minimum`, `Sum`, `SampleCount`, `Average`, `pXX` (e.g. `p90`).

- Watch out using `addCloudwatchTimestamp` for sparse metrics, e.g from S3, since Prometheus won't scrape metrics containing timestamps older than 2-3 hours. Also the same applies when enabling `exportAllDataPoints` in any metric.

### `exported_tags_config`

This is an example of the `exported_tags_config` block:

```yaml
exportedTagsOnMetrics:
  ebs:
    - VolumeId
  kafka:
    - Name
```

### `role_config`

This is an example of the `role_config` block:

```yaml
roles:
  - roleArn: "arn:aws:iam::123456789012:role/Prometheus"
    externalId: "shared-external-identifier" # optional
```

### `search_tags_config`

This is an example of the `search_tags_config` block:

```yaml
searchTags:
  - key: env
    value: production
```

### `custom_tags_config`

This is an example of the `custom_tags_config` block:

```yaml
customTags:
  - key: CustomTag
    value: CustomValue
```

### `dimensions_config`

This is an example of the `dimensions_config` block:

```yaml
dimensions:
  - name: AutoScalingGroupName
    value: MyGroup
```

### `enhanced_metrics_config`

The `enhanced_metrics_config` block allows enabling enhanced metrics for specific metrics within a discovery job.

Currently supported enhanced metrics are:

- AWS/Lambda (Timeout) - The maximum execution duration permitted for the function before termination.
- AWS/DynamoDB (ItemCount) - The count of items in the table, updated approximately every six hours; may not reflect recent changes.
- AWS/RDS (AllocatedStorage) - The storage capacity in bytes allocated for the DB instance.
- AWS/ElastiCache (NumCacheNodes) - The count of cache nodes in the cluster; must be 1 for Valkey or Redis OSS clusters, or between 1 and 40 for Memcached clusters.

```yaml
enhancedMetrics:
    - name: ItemCount
```