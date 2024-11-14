# YACE - yet another cloudwatch exporter

YACE, or `yet another cloudwatch exporter`, is a [Prometheus exporter](https://prometheus.io/docs/instrumenting/exporters/#exporters-and-integrations) for [AWS CloudWatch](http://aws.amazon.com/cloudwatch/) metrics. It is written in Go and uses the official AWS SDK.

The project was originally created by Thomas Peitz while working at InVision.de, then later moved outside of the company repo. Read the full rebranding story [here](https://medium.com/@IT_Supertramp/reorganizing-yace-79d7149b9584).

## Alternatives

Consider using the official [CloudWatch Exporter](https://github.com/prometheus/cloudwatch_exporter) if you prefer a Java implementation.


## Project Status

While YACE is at version less than 1.0.0, expect that any new release might introduce breaking changes. We'll document changes in [CHANGELOG.md](CHANGELOG.md).

Where feasible, features will be deprecated instead of being immediately changed or removed. This means that YACE will continue to work but might log warning messages. Expect deprecated features to be permanently changed/removed within the next 2/3 releases.

## Security

Read more how to report a security vulnerability in [SECURITY.md](SECURITY.md).

### Supported Versions

Only the latest version gets security updates. We won't support older versions.

## Features

* Stop worrying about your AWS IDs - Auto discovery of resources via tags
* Structured logging (json and logfmt)
* Filter monitored resources via regex
* Automatic adding of tag labels to metrics
* Automatic adding of dimension labels to metrics
* Allows to export 0 even if CloudWatch returns nil
* Allows exports metrics with CloudWatch timestamps (disabled by default)
* Static metrics support for all cloudwatch metrics without auto discovery
* Pull data from multiple AWS accounts using cross-account roles
* Can be used as a library in an external application
* Support the scraping of custom namespaces metrics with the CloudWatch Dimensions.
* Supported services with auto discovery through tags:
  * `/aws/sagemaker/Endpoints` - Sagemaker Endpoints
  * `/aws/sagemaker/InferenceRecommendationsJobs` - Sagemaker Inference Recommender Jobs
  * `/aws/sagemaker/ProcessingJobs` - Sagemaker Processing Jobs
  * `/aws/sagemaker/TrainingJobs` - Sagemaker Training Jobs
  * `/aws/sagemaker/TransformJobs` - Sagemaker Batch Transform Jobs
  * `AmazonMWAA` - Managed Apache Airflow
  * `AWS/ACMPrivateCA` - ACM Private CA
  * `AWS/AmazonMQ` - Managed Message Broker Service
  * `AWS/AOSS` - OpenSearch Serverless
  * `AWS/ApiGateway` - ApiGateway (V1 and V2)
  * `AWS/ApplicationELB` - Application Load Balancer
  * `AWS/AppRunner` - Managed Container Apps Service
  * `AWS/AppStream` - AppStream
  * `AWS/AppSync` - AppSync
  * `AWS/Athena` - Athena
  * `AWS/AutoScaling` - Auto Scaling Group
  * `AWS/Backup` - Backup
  * `AWS/Bedrock` - GenerativeAI
  * `AWS/Billing` - Billing
  * `AWS/Cassandra` - Cassandra
  * `AWS/CertificateManager` - Certificate Manager
  * `AWS/ClientVPN` - Client-based VPN
  * `AWS/CloudFront` - Cloud Front
  * `AWS/Cognito` - Cognito
  * `AWS/DataSync` - DataSync
  * `AWS/DDoSProtection` - Distributed Denial of Service (DDoS) protection service
  * `AWS/DirectoryService` - Directory Services (MicrosoftAD)
  * `AWS/DMS` - Database Migration Service
  * `AWS/DocDB` - DocumentDB (with MongoDB compatibility)
  * `AWS/DX` - Direct Connect
  * `AWS/DynamoDB` - NoSQL Key-Value Database
  * `AWS/EBS` - Elastic Block Storage
  * `AWS/EC2` - Elastic Compute Cloud
  * `AWS/EC2Spot` - Elastic Compute Cloud for Spot Instances
  * `AWS/ECR` - Elastic Container Registry
  * `AWS/ECS` - Elastic Container Service (Service Metrics)
  * `AWS/EFS` - Elastic File System
  * `AWS/ElastiCache` - ElastiCache
  * `AWS/ElasticBeanstalk` - Elastic Beanstalk
  * `AWS/ElasticMapReduce` - Elastic MapReduce
  * `AWS/ELB` - Elastic Load Balancer
  * `AWS/EMRServerless` - Amazon EMR Serverless
  * `AWS/ES` - ElasticSearch
  * `AWS/Events` - EventBridge
  * `AWS/Firehose` - Managed Streaming Service
  * `AWS/FSx` - FSx File System
  * `AWS/GameLift` - GameLift
  * `AWS/GatewayELB` - Gateway Load Balancer
  * `AWS/GlobalAccelerator` - AWS Global Accelerator
  * `AWS/IoT` - IoT
  * `AWS/IPAM` - IP address manager
  * `AWS/Kafka` - Managed Apache Kafka
  * `AWS/KafkaConnect` - AWS MSK Connectors
  * `AWS/Kinesis` - Kinesis Data Stream
  * `AWS/KinesisAnalytics` - Kinesis Data Analytics for SQL Applications
  * `AWS/KMS` - Key Management Service
  * `AWS/Lambda` - Lambda Functions
  * `AWS/Logs` - CloudWatch Logs
  * `AWS/MediaConnect` - AWS Elemental MediaConnect
  * `AWS/MediaConvert` - AWS Elemental MediaConvert
  * `AWS/MediaLive` - AWS Elemental MediaLive
  * `AWS/MediaPackage` - AWS Elemental MediaPackage
  * `AWS/MediaTailor` - AWS Elemental MediaTailor
  * `AWS/MemoryDB` - AWS MemoryDB
  * `AWS/MWAA` - Managed Apache Airflow (Container, queue, and database metrics)
  * `AWS/NATGateway` - NAT Gateway
  * `AWS/Neptune` - Neptune
  * `AWS/NetworkELB` - Network Load Balancer
  * `AWS/NetworkFirewall` - Network Firewall
  * `AWS/PrivateLinkEndpoints` - VPC Endpoint
  * `AWS/PrivateLinkServices` - VPC Endpoint Service
  * `AWS/Prometheus` - Managed Service for Prometheus
  * `AWS/QLDB` - Quantum Ledger Database
  * `AWS/QuickSight` - QuickSight (Business Intelligence)
  * `AWS/RDS` - Relational Database Service
  * `AWS/Redshift` - Redshift Database
  * `AWS/Route53` - Route53 Health Checks
  * `AWS/Route53Resolver` - Route53 Resolver
  * `AWS/RUM` - Real User Monitoring
  * `AWS/S3` - Object Storage
  * `AWS/Sagemaker/ModelBuildingPipeline` - Sagemaker Model Building Pipelines
  * `AWS/SageMaker` - Sagemaker invocations
  * `AWS/Scheduler` - EventBridge Scheduler
  * `AWS/SecretsManager` - Secrets Manager
  * `AWS/SES` - Simple Email Service
  * `AWS/SNS` - Simple Notification Service
  * `AWS/SQS` - Simple Queue Service
  * `AWS/States` - Step Functions
  * `AWS/StorageGateway` - On-premises access to cloud storage
  * `AWS/Timestream` - Time-series database service
  * `AWS/TransitGateway` - Transit Gateway
  * `AWS/TrustedAdvisor` - Trusted Advisor
  * `AWS/Usage` - Usage of some AWS resources and APIs
  * `AWS/VpcLattice` - VPC Lattice
  * `AWS/VPN` - VPN connection
  * `AWS/WAFV2` - Web Application Firewall v2
  * `AWS/WorkSpaces` - Workspaces
  * `ContainerInsights` - EKS ContainerInsights (Dependency on Cloudwatch agent)
  * `CWAgent` - CloudWatch agent
  * `ECS/ContainerInsights` - ECS/ContainerInsights (Fargate metrics)
  * `Glue` - AWS Glue Jobs

## Feature flags

To provide backwards compatibility, some of YACE's new features or breaking changes might be guarded under a feature flag. Refer to [docs/feature_flags.md](./docs/feature_flags.md) for details.

## Installing and running

Refer to the [installation guide](docs/installation.md).

## Authentication

The exporter will need to be running in an environment which has access to AWS. The exporter uses the [AWS SDK for Go](https://aws.github.io/aws-sdk-go-v2/docs/getting-started/) and supports providing authentication via [AWS's default credential chain](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials). Regardless of the method used to acquire the credentials, some permissions are needed for the exporter to work.

As a quick start, the following IAM policy can be used to grant the all permissions required by YACE
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "tag:GetResources",
        "cloudwatch:GetMetricData",
        "cloudwatch:GetMetricStatistics",
        "cloudwatch:ListMetrics",
        "apigateway:GET",
        "aps:ListWorkspaces",
        "autoscaling:DescribeAutoScalingGroups",
        "dms:DescribeReplicationInstances",
        "dms:DescribeReplicationTasks",
        "ec2:DescribeTransitGatewayAttachments",
        "ec2:DescribeSpotFleetRequests",
        "shield:ListProtections",
        "storagegateway:ListGateways",
        "storagegateway:ListTagsForResource",
        "iam:ListAccountAliases"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
```

If you would like to remove certain permissions based on your needs the policy can be adjusted based the CloudWatch namespaces you are scraping

These are the bare minimum permissions required to run Static and Discovery Jobs
```json
"tag:GetResources",
"cloudwatch:GetMetricData",
"cloudwatch:GetMetricStatistics",
"cloudwatch:ListMetrics"
```

This permission is required to discover resources for the AWS/ApiGateway namespace
```json
"apigateway:GET"
```

This permission is required to discover resources for the AWS/AutoScaling namespace
```json
"autoscaling:DescribeAutoScalingGroups"
```

These permissions are required to discover resources for the AWS/DMS namespace
```json
"dms:DescribeReplicationInstances",
"dms:DescribeReplicationTasks"
```


This permission is required to discover resources for the AWS/EC2Spot namespace
```json
"ec2:DescribeSpotFleetRequests"
```

This permission is required to discover resources for the AWS/Prometheus namespace
```json
"aps:ListWorkspaces"
```

These permissions are required to discover resources for the AWS/StorageGateway namespace
```json
"storagegateway:ListGateways",
"storagegateway:ListTagsForResource"
```

This permission is required to discover resources for the AWS/TransitGateway namespace
```json
"ec2:DescribeTransitGatewayAttachments"
```

This permission is required to discover protected resources for the AWS/DDoSProtection namespace
```json
"shield:ListProtections"
```

The AWS IAM API supports creating account aliases, which are human-friendly names that can be used to easily identify accounts. An account can have at most a single alias, see ([docs](https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListAccountAliases.html)). Each alias must be unique across an AWS network partition ([docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/console_account-alias.html#AboutAccountAlias)). The following permission is required to get the alias for an account, which is exported as a label in the `aws_account_info` metric:
```json
"iam:ListAccountAliases"
```

If running YACE inside an AWS EC2 instance, the exporter will automatically attempt to assume the associated IAM Role. If this is undesirable behavior turn off the use the metadata endpoint by setting the environment variable `AWS_EC2_METADATA_DISABLED=true`.

## Configuration

Refer to the [configuration](docs/configuration.md) docs.

## Metrics Examples

```text
### Metrics with exportedTagsOnMetrics
aws_ec2_cpuutilization_maximum{dimension_InstanceId="i-someid", name="arn:aws:ec2:eu-west-1:472724724:instance/i-someid", tag_Name="jenkins"} 57.2916666666667

### Info helper with tags
aws_elb_info{name="arn:aws:elasticloadbalancing:eu-west-1:472724724:loadbalancer/a815b16g3417211e7738a02fcc13bbf9",tag_KubernetesCluster="production-19",tag_Name="",tag_kubernetes_io_cluster_production_19="owned",tag_kubernetes_io_service_name="nginx-ingress/private-ext",region="eu-west-1"} 0
aws_ec2_info{name="arn:aws:ec2:eu-west-1:472724724:instance/i-someid",tag_Name="jenkins"} 0

### Track cloudwatch requests to calculate costs
yace_cloudwatch_requests_total 168
```

## Query Examples without exportedTagsOnMetrics

```text
# CPUUtilization + Name tag of the instance id - No more instance id needed for monitoring
aws_ec2_cpuutilization_average + on (name) group_left(tag_Name) aws_ec2_info

# Free Storage in Megabytes + tag Type of the elasticsearch cluster
(aws_es_free_storage_space_sum + on (name) group_left(tag_Type) aws_es_info) / 1024

# Add kubernetes / kops tags on 4xx elb metrics
(aws_elb_httpcode_backend_4_xx_sum + on (name) group_left(tag_KubernetesCluster,tag_kubernetes_io_service_name) aws_elb_info)

# Availability Metric for ELBs (Successful requests / Total Requests) + k8s service name
# Use nilToZero on all metrics else it won't work
((aws_elb_request_count_sum - on (name) group_left() aws_elb_httpcode_backend_4_xx_sum) - on (name) group_left() aws_elb_httpcode_backend_5_xx_sum) + on (name) group_left(tag_kubernetes_io_service_name) aws_elb_info

# Forecast your elasticsearch disk size in 7 days and report metrics with tags type and version
predict_linear(aws_es_free_storage_space_minimum[2d], 86400 * 7) + on (name) group_left(tag_type, tag_version) aws_es_info

# Forecast your cloudwatch costs for next 32 days based on last 10 minutes
# 1.000.000 Requests free
# 0.01 Dollar for 1.000 GetMetricStatistics Api Requests (https://aws.amazon.com/cloudwatch/pricing/)
((increase(yace_cloudwatch_requests_total[10m]) * 6 * 24 * 32) - 100000) / 1000 * 0.01
```

## Override AWS endpoint urls
to support local testing all AWS urls can be overridden with by setting an environment variable `AWS_ENDPOINT_URL`
```shell
docker run -d --rm -v $PWD/credentials:/exporter/.aws/credentials -v $PWD/config.yml:/tmp/config.yml \
-e AWS_ENDPOINT_URL=http://localhost:4766 -p 5000:5000 --name yace ghcr.io/nerdswords/yet-another-cloudwatch-exporter:vx.xx.x # release version as tag - Do not forget the version 'v'
```

## Options
### RoleArns

Multiple roleArns are useful, when you are monitoring multi-account setup, where all accounts are using same AWS services. For example, you are running yace in monitoring account and you have number of accounts (for example newspapers, radio and television) running ECS clusters. Each account gives yace permissions to assume local IAM role, which has all the necessary permissions for Cloudwatch metrics. On this kind of setup, you could simply list:
```yaml
apiVersion: v1alpha1
sts-region: eu-west-1
discovery:
  jobs:
    - type: AWS/ECS
      regions:
        - eu-north-1
      roles:
        - roleArn: "arn:aws:iam::1111111111111:role/prometheus" # newspaper
        - roleArn: "arn:aws:iam::2222222222222:role/prometheus" # radio
        - roleArn: "arn:aws:iam::3333333333333:role/prometheus" # television
      metrics:
        - name: MemoryReservation
          statistics:
            - Average
            - Minimum
            - Maximum
          period: 600
          length: 600
```

Additionally, if the IAM role you want to assume requires an [External ID](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user_externalid.html?icmpid=docs_iam_console) you can specify it this way:

```yaml
  roles:
    - roleArn: "arn:aws:iam::1111111111111:role/prometheus"
      externalId: "shared-external-identifier"
```

### Requests concurrency
The flags 'cloudwatch-concurrency' and 'tag-concurrency' define the number of concurrent request to cloudwatch metrics and tags. Their default value is 5.

Setting a higher value makes faster scraping times but can incur in throttling and the blocking of the API.

### Decoupled scraping
The exporter scraped cloudwatch metrics in the background in fixed interval.
This protects from the abuse of API requests that can cause extra billing in AWS account.

The flag 'scraping-interval' defines the seconds between scrapes.
The default value is 300.

## Embedding YACE in your application

YACE can be used as a library and embedded into your application, see the [embedding guide](docs/embedding.md).

## Troubleshooting / Debugging

### Help my metrics are intermittent

* Please, try out a bigger length e.g. for elb try out a length of 600 and a period of 600. Then test how low you can
go without losing data. ELB metrics on AWS are written every 5 minutes (300) in default.

### My metrics only show new values after 5 minutes

* Please, try to set a lower value for the 'scraping-interval' flag or set the 'decoupled-scraping' to false.

## Contribute

[Development Setup / Guide](/CONTRIBUTE.md)

## Thank you

* [Justin Santa Barbara](https://github.com/justinsb) - For telling me about AWS tags api which simplified a lot - Thanks!
* [Brian Brazil](https://github.com/brian-brazil) - Who gave a lot of feedback regarding UX and prometheus lib - Thanks!
