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
package config

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/grafana/regexp"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// ServiceConfig defines a namespace supported by discovery jobs.
type ServiceConfig struct {
	// Namespace is the formal AWS namespace identification string
	Namespace string
	// Alias is the formal AWS namespace alias
	Alias string
	// ResourceFilters is a list of strings used as filters in the
	// resourcegroupstaggingapi.GetResources request. It should always
	// be provided, except for those few namespaces where resources can't
	// be tagged.
	ResourceFilters []*string
	// DimensionRegexps is an optional list of regexes that allow to
	// extract dimensions names from a resource ARN. The regex should
	// use named groups that correspond to AWS dimensions names.
	// In cases where the dimension name has a space, it should be
	// replaced with an underscore (`_`).
	DimensionRegexps []*regexp.Regexp
}

func (sc ServiceConfig) ToModelDimensionsRegexp() []model.DimensionsRegexp {
	dr := []model.DimensionsRegexp{}

	for _, regexp := range sc.DimensionRegexps {
		names := regexp.SubexpNames()
		dimensionNames := make([]string, 0, len(names)-1)

		// skip first name, it's always an empty string
		for i := 1; i < len(names); i++ {
			// in the regex names we use underscores where AWS dimensions have spaces
			dimensionNames = append(dimensionNames, strings.ReplaceAll(names[i], "_", " "))
		}

		dr = append(dr, model.DimensionsRegexp{
			Regexp:          regexp,
			DimensionsNames: dimensionNames,
		})
	}

	return dr
}

type serviceConfigs []ServiceConfig

func (sc serviceConfigs) GetService(serviceType string) *ServiceConfig {
	for _, sf := range sc {
		if sf.Namespace == serviceType {
			return &sf
		}
	}
	return nil
}

func (sc serviceConfigs) getServiceByAlias(alias string) *ServiceConfig {
	for _, sf := range sc {
		if sf.Alias == alias {
			return &sf
		}
	}
	return nil
}

var SupportedServices = serviceConfigs{
	{
		Namespace: "CWAgent",
		Alias:     "cwagent",
	},
	{
		Namespace: "AWS/Usage",
		Alias:     "usage",
	},
	{
		Namespace: "AWS/CertificateManager",
		Alias:     "acm",
		ResourceFilters: []*string{
			aws.String("acm:certificate"),
		},
	},
	{
		Namespace: "AWS/ACMPrivateCA",
		Alias:     "acm-pca",
		ResourceFilters: []*string{
			aws.String("acm-pca:certificate-authority"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<PrivateCAArn>.*)"),
		},
	},
	{
		Namespace: "AmazonMWAA",
		Alias:     "airflow",
		ResourceFilters: []*string{
			aws.String("airflow"),
		},
	},
	{
		Namespace: "AWS/MWAA",
		Alias:     "mwaa",
	},
	{
		Namespace: "AWS/ApplicationELB",
		Alias:     "alb",
		ResourceFilters: []*string{
			aws.String("elasticloadbalancing:loadbalancer/app"),
			aws.String("elasticloadbalancing:targetgroup"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":(?P<TargetGroup>targetgroup/.+)"),
			regexp.MustCompile(":loadbalancer/(?P<LoadBalancer>.+)$"),
		},
	},
	{
		Namespace: "AWS/AppStream",
		Alias:     "appstream",
		ResourceFilters: []*string{
			aws.String("appstream"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":fleet/(?P<FleetName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Backup",
		Alias:     "backup",
		ResourceFilters: []*string{
			aws.String("backup"),
		},
	},
	{
		Namespace: "AWS/ApiGateway",
		Alias:     "apigateway",
		ResourceFilters: []*string{
			aws.String("apigateway"),
		},
		DimensionRegexps: []*regexp.Regexp{
			// DimensionRegexps starting with 'restapis' are for APIGateway V1 gateways (REST API gateways)
			regexp.MustCompile("/restapis/(?P<ApiName>[^/]+)$"),
			regexp.MustCompile("/restapis/(?P<ApiName>[^/]+)/stages/(?P<Stage>[^/]+)$"),
			// DimensionRegexps starting 'apis' are for APIGateway V2 gateways (HTTP and Websocket gateways)
			regexp.MustCompile("/apis/(?P<ApiId>[^/]+)$"),
			regexp.MustCompile("/apis/(?P<ApiId>[^/]+)/stages/(?P<Stage>[^/]+)$"),
			regexp.MustCompile("/apis/(?P<ApiId>[^/]+)/routes/(?P<Route>[^/]+)$"),
		},
	},
	{
		Namespace: "AWS/AmazonMQ",
		Alias:     "mq",
		ResourceFilters: []*string{
			aws.String("mq"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("broker:(?P<Broker>[^:]+)"),
		},
	},
	{
		Namespace: "AWS/AppRunner",
		Alias:     "apprunner",
	},
	{
		Namespace: "AWS/AppSync",
		Alias:     "appsync",
		ResourceFilters: []*string{
			aws.String("appsync"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("apis/(?P<GraphQLAPIId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Athena",
		Alias:     "athena",
		ResourceFilters: []*string{
			aws.String("athena"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("workgroup/(?P<WorkGroup>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/AutoScaling",
		Alias:     "asg",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("autoScalingGroupName/(?P<AutoScalingGroupName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/ElasticBeanstalk",
		Alias:     "beanstalk",
		ResourceFilters: []*string{
			aws.String("elasticbeanstalk:environment"),
		},
	},
	{
		Namespace: "AWS/Billing",
		Alias:     "billing",
	},
	{
		Namespace: "AWS/Cassandra",
		Alias:     "cassandra",
		ResourceFilters: []*string{
			aws.String("cassandra"),
		},
	},
	{
		Namespace: "AWS/CloudFront",
		Alias:     "cloudfront",
		ResourceFilters: []*string{
			aws.String("cloudfront:distribution"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("distribution/(?P<DistributionId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Cognito",
		Alias:     "cognito-idp",
		ResourceFilters: []*string{
			aws.String("cognito-idp:userpool"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("userpool/(?P<UserPool>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/DataSync",
		Alias:     "datasync",
		ResourceFilters: []*string{
			aws.String("datasync:task"),
			aws.String("datasync:agent"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":task/(?P<TaskId>[^/]+)"),
			regexp.MustCompile(":agent/(?P<AgentId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/DirectoryService",
		Alias:     "ds",
		ResourceFilters: []*string{
			aws.String("ds:directory"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":directory/(?P<Directory_ID>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/DMS",
		Alias:     "dms",
		ResourceFilters: []*string{
			aws.String("dms"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("rep:[^/]+/(?P<ReplicationInstanceIdentifier>[^/]+)"),
			regexp.MustCompile("task:(?P<ReplicationTaskIdentifier>[^/]+)/(?P<ReplicationInstanceIdentifier>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/DDoSProtection",
		Alias:     "shield",
		ResourceFilters: []*string{
			aws.String("shield:protection"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<ResourceArn>.+)"),
		},
	},
	{
		Namespace: "AWS/DocDB",
		Alias:     "docdb",
		ResourceFilters: []*string{
			aws.String("rds:db"),
			aws.String("rds:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("cluster:(?P<DBClusterIdentifier>[^/]+)"),
			regexp.MustCompile("db:(?P<DBInstanceIdentifier>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/DX",
		Alias:     "dx",
		ResourceFilters: []*string{
			aws.String("directconnect"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":dxcon/(?P<ConnectionId>[^/]+)"),
			regexp.MustCompile(":dxlag/(?P<LagId>[^/]+)"),
			regexp.MustCompile(":dxvif/(?P<VirtualInterfaceId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/DynamoDB",
		Alias:     "dynamodb",
		ResourceFilters: []*string{
			aws.String("dynamodb:table"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":table/(?P<TableName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/EBS",
		Alias:     "ebs",
		ResourceFilters: []*string{
			aws.String("ec2:volume"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("volume/(?P<VolumeId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/ElastiCache",
		Alias:     "ec",
		ResourceFilters: []*string{
			aws.String("elasticache:cluster"),
			aws.String("elasticache:serverlesscache"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("cluster:(?P<CacheClusterId>[^/]+)"),
			regexp.MustCompile("serverlesscache:(?P<clusterId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/MemoryDB",
		Alias:     "memorydb",
		ResourceFilters: []*string{
			aws.String("memorydb:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("cluster/(?P<ClusterName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/EC2",
		Alias:     "ec2",
		ResourceFilters: []*string{
			aws.String("ec2:instance"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("instance/(?P<InstanceId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/EC2Spot",
		Alias:     "ec2Spot",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<FleetRequestId>.*)"),
		},
	},
	{
		Namespace: "AWS/EC2CapacityReservations",
		Alias:     "ec2CapacityReservations",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":capacity-reservation/(?P<CapacityReservationId>)$"),
		},
	},
	{
		Namespace: "AWS/ECS",
		Alias:     "ecs-svc",
		ResourceFilters: []*string{
			aws.String("ecs:cluster"),
			aws.String("ecs:service"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster/(?P<ClusterName>[^/]+)$"),
			regexp.MustCompile(":service/(?P<ClusterName>[^/]+)/(?P<ServiceName>[^/]+)$"),
		},
	},
	{
		Namespace: "ECS/ContainerInsights",
		Alias:     "ecs-containerinsights",
		ResourceFilters: []*string{
			aws.String("ecs:cluster"),
			aws.String("ecs:service"),
		},
		DimensionRegexps: []*regexp.Regexp{
			// Use "new" long arns as per
			// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids
			regexp.MustCompile(":cluster/(?P<ClusterName>[^/]+)$"),
			regexp.MustCompile(":service/(?P<ClusterName>[^/]+)/(?P<ServiceName>[^/]+)$"),
		},
	},
	{
		Namespace: "ContainerInsights",
		Alias:     "containerinsights",
		ResourceFilters: []*string{
			aws.String("eks:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster/(?P<ClusterName>[^/]+)$"),
		},
	},
	{
		Namespace: "AWS/EFS",
		Alias:     "efs",
		ResourceFilters: []*string{
			aws.String("elasticfilesystem:file-system"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("file-system/(?P<FileSystemId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/ELB",
		Alias:     "elb",
		ResourceFilters: []*string{
			aws.String("elasticloadbalancing:loadbalancer"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":loadbalancer/(?P<LoadBalancerName>.+)$"),
		},
	},
	{
		Namespace: "AWS/ElasticMapReduce",
		Alias:     "emr",
		ResourceFilters: []*string{
			aws.String("elasticmapreduce:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("cluster/(?P<JobFlowId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/EMRServerless",
		Alias:     "emr-serverless",
		ResourceFilters: []*string{
			aws.String("emr-serverless:applications"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("applications/(?P<ApplicationId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/ES",
		Alias:     "es",
		ResourceFilters: []*string{
			aws.String("es:domain"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":domain/(?P<DomainName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Firehose",
		Alias:     "firehose",
		ResourceFilters: []*string{
			aws.String("firehose"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":deliverystream/(?P<DeliveryStreamName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/FSx",
		Alias:     "fsx",
		ResourceFilters: []*string{
			aws.String("fsx:file-system"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("file-system/(?P<FileSystemId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/GameLift",
		Alias:     "gamelift",
		ResourceFilters: []*string{
			aws.String("gamelift"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":fleet/(?P<FleetId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/GatewayELB",
		Alias:     "gwlb",
		ResourceFilters: []*string{
			aws.String("elasticloadbalancing:loadbalancer"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":(?P<TargetGroup>targetgroup/.+)"),
			regexp.MustCompile(":loadbalancer/(?P<LoadBalancer>.+)$"),
		},
	},
	{
		Namespace: "AWS/GlobalAccelerator",
		Alias:     "ga",
		ResourceFilters: []*string{
			aws.String("globalaccelerator"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("accelerator/(?P<Accelerator>[^/]+)$"),
			regexp.MustCompile("accelerator/(?P<Accelerator>[^/]+)/listener/(?P<Listener>[^/]+)$"),
			regexp.MustCompile("accelerator/(?P<Accelerator>[^/]+)/listener/(?P<Listener>[^/]+)/endpoint-group/(?P<EndpointGroup>[^/]+)$"),
		},
	},
	{
		Namespace: "Glue",
		Alias:     "glue",
		ResourceFilters: []*string{
			aws.String("glue:job"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":job/(?P<JobName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/IoT",
		Alias:     "iot",
		ResourceFilters: []*string{
			aws.String("iot:rule"),
			aws.String("iot:provisioningtemplate"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":rule/(?P<RuleName>[^/]+)"),
			regexp.MustCompile(":provisioningtemplate/(?P<TemplateName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Kafka",
		Alias:     "kafka",
		ResourceFilters: []*string{
			aws.String("kafka:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster/(?P<Cluster_Name>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/KafkaConnect",
		Alias:     "kafkaconnect",
		ResourceFilters: []*string{
			aws.String("kafka:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":connector/(?P<Connector_Name>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Kinesis",
		Alias:     "kinesis",
		ResourceFilters: []*string{
			aws.String("kinesis:stream"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":stream/(?P<StreamName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/KinesisAnalytics",
		Alias:     "kinesis-analytics",
		ResourceFilters: []*string{
			aws.String("kinesisanalytics:application"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":application/(?P<Application>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/KMS",
		Alias:     "kms",
		ResourceFilters: []*string{
			aws.String("kms:key"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":key/(?P<KeyId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Lambda",
		Alias:     "lambda",
		ResourceFilters: []*string{
			aws.String("lambda:function"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":function:(?P<FunctionName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Logs",
		Alias:     "logs",
		ResourceFilters: []*string{
			aws.String("logs:log-group"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":log-group:(?P<LogGroupName>.+)"),
		},
	},
	{
		Namespace: "AWS/MediaConnect",
		Alias:     "mediaconnect",
		ResourceFilters: []*string{
			aws.String("mediaconnect:flow"),
			aws.String("mediaconnect:source"),
			aws.String("mediaconnect:output"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("^(?P<FlowARN>.*:flow:.*)$"),
			regexp.MustCompile("^(?P<SourceARN>.*:source:.*)$"),
			regexp.MustCompile("^(?P<OutputARN>.*:output:.*)$"),
		},
	},
	{
		Namespace: "AWS/MediaConvert",
		Alias:     "mediaconvert",
		ResourceFilters: []*string{
			aws.String("mediaconvert"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<Queue>.*:.*:mediaconvert:.*:queues/.*)$"),
		},
	},
	{
		Namespace: "AWS/MediaPackage",
		Alias:     "mediapackage",
		ResourceFilters: []*string{
			aws.String("mediapackage"),
			aws.String("mediapackagev2"),
			aws.String("mediapackage-vod"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":channels/(?P<IngestEndpoint>.+)$"),
			regexp.MustCompile(":packaging-configurations/(?P<PackagingConfiguration>.+)$"),
		},
	},
	{
		Namespace: "AWS/MediaLive",
		Alias:     "medialive",
		ResourceFilters: []*string{
			aws.String("medialive:channel"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":channel:(?P<ChannelId>.+)$"),
		},
	},
	{
		Namespace: "AWS/MediaTailor",
		Alias:     "mediatailor",
		ResourceFilters: []*string{
			aws.String("mediatailor:playbackConfiguration"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("playbackConfiguration/(?P<ConfigurationName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Neptune",
		Alias:     "neptune",
		ResourceFilters: []*string{
			aws.String("rds:db"),
			aws.String("rds:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster:(?P<DBClusterIdentifier>[^/]+)"),
			regexp.MustCompile(":db:(?P<DBInstanceIdentifier>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/NetworkFirewall",
		Alias:     "nfw",
		ResourceFilters: []*string{
			aws.String("network-firewall:firewall"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("firewall/(?P<FirewallName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/NATGateway",
		Alias:     "ngw",
		ResourceFilters: []*string{
			aws.String("ec2:natgateway"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("natgateway/(?P<NatGatewayId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/NetworkELB",
		Alias:     "nlb",
		ResourceFilters: []*string{
			aws.String("elasticloadbalancing:loadbalancer/net"),
			aws.String("elasticloadbalancing:targetgroup"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":(?P<TargetGroup>targetgroup/.+)"),
			regexp.MustCompile(":loadbalancer/(?P<LoadBalancer>.+)$"),
		},
	},
	{
		Namespace: "AWS/PrivateLinkEndpoints",
		Alias:     "vpc-endpoint",
		ResourceFilters: []*string{
			aws.String("ec2:vpc-endpoint"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":vpc-endpoint/(?P<VPC_Endpoint_Id>.+)"),
		},
	},
	{
		Namespace: "AWS/PrivateLinkServices",
		Alias:     "vpc-endpoint-service",
		ResourceFilters: []*string{
			aws.String("ec2:vpc-endpoint-service"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":vpc-endpoint-service/(?P<Service_Id>.+)"),
		},
	},
	{
		Namespace: "AWS/Prometheus",
		Alias:     "amp",
	},
	{
		Namespace: "AWS/QLDB",
		Alias:     "qldb",
		ResourceFilters: []*string{
			aws.String("qldb"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":ledger/(?P<LedgerName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/QuickSight",
		Alias:     "quicksight",
	},
	{
		Namespace: "AWS/RDS",
		Alias:     "rds",
		ResourceFilters: []*string{
			aws.String("rds:db"),
			aws.String("rds:cluster"),
			aws.String("rds:db-proxy"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster:(?P<DBClusterIdentifier>[^/]+)"),
			regexp.MustCompile(":db:(?P<DBInstanceIdentifier>[^/]+)"),
			regexp.MustCompile(":db-proxy:(?P<ProxyIdentifier>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Redshift",
		Alias:     "redshift",
		ResourceFilters: []*string{
			aws.String("redshift:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster:(?P<ClusterIdentifier>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Route53Resolver",
		Alias:     "route53-resolver",
		ResourceFilters: []*string{
			aws.String("route53resolver"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":resolver-endpoint/(?P<EndpointId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Route53",
		Alias:     "route53",
		ResourceFilters: []*string{
			aws.String("route53"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":healthcheck/(?P<HealthCheckId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/RUM",
		Alias:     "rum",
	},
	{
		Namespace: "AWS/S3",
		Alias:     "s3",
		ResourceFilters: []*string{
			aws.String("s3"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<BucketName>[^:]+)$"),
		},
	},
	{
		Namespace: "AWS/Scheduler",
		Alias:     "scheduler",
	},
	{
		Namespace: "AWS/ECR",
		Alias:     "ecr",
	},
	{
		Namespace: "AWS/Timestream",
		Alias:     "timestream",
	},
	{
		Namespace: "AWS/SecretsManager",
		Alias:     "secretsmanager",
	},
	{
		Namespace: "AWS/SES",
		Alias:     "ses",
	},
	{
		Namespace: "AWS/States",
		Alias:     "sfn",
		ResourceFilters: []*string{
			aws.String("states"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<StateMachineArn>.*)"),
		},
	},
	{
		Namespace: "AWS/SNS",
		Alias:     "sns",
		ResourceFilters: []*string{
			aws.String("sns"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<TopicName>[^:]+)$"),
		},
	},
	{
		Namespace: "AWS/SQS",
		Alias:     "sqs",
		ResourceFilters: []*string{
			aws.String("sqs"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<QueueName>[^:]+)$"),
		},
	},
	{
		Namespace: "AWS/StorageGateway",
		Alias:     "storagegateway",
		ResourceFilters: []*string{
			aws.String("storagegateway"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":gateway/(?P<GatewayId>[^:]+)$"),
			regexp.MustCompile(":share/(?P<ShareId>[^:]+)$"),
			regexp.MustCompile("^(?P<GatewayId>[^:/]+)/(?P<GatewayName>[^:]+)$"),
		},
	},
	{
		Namespace: "AWS/TransitGateway",
		Alias:     "tgw",
		ResourceFilters: []*string{
			aws.String("ec2:transit-gateway"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":transit-gateway/(?P<TransitGateway>[^/]+)"),
			regexp.MustCompile("(?P<TransitGateway>[^/]+)/(?P<TransitGatewayAttachment>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/TrustedAdvisor",
		Alias:     "trustedadvisor",
	},
	{
		Namespace: "AWS/VPN",
		Alias:     "vpn",
		ResourceFilters: []*string{
			aws.String("ec2:vpn-connection"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":vpn-connection/(?P<VpnId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/ClientVPN",
		Alias:     "clientvpn",
		ResourceFilters: []*string{
			aws.String("ec2:client-vpn-endpoint"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":client-vpn-endpoint/(?P<Endpoint>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/WAFV2",
		Alias:     "wafv2",
		ResourceFilters: []*string{
			aws.String("wafv2"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("/webacl/(?P<WebACL>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/WorkSpaces",
		Alias:     "workspaces",
		ResourceFilters: []*string{
			aws.String("workspaces:workspace"),
			aws.String("workspaces:directory"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":workspace/(?P<WorkspaceId>[^/]+)$"),
			regexp.MustCompile(":directory/(?P<DirectoryId>[^/]+)$"),
		},
	},
	{
		Namespace: "AWS/AOSS",
		Alias:     "aoss",
		ResourceFilters: []*string{
			aws.String("aoss:collection"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":collection/(?P<CollectionId>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/SageMaker",
		Alias:     "sagemaker",
		ResourceFilters: []*string{
			aws.String("sagemaker:endpoint"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":endpoint/(?P<EndpointName>[^/]+)$"),
		},
	},
	{
		Namespace: "/aws/sagemaker/Endpoints",
		Alias:     "sagemaker-endpoints",
		ResourceFilters: []*string{
			aws.String("sagemaker:endpoint"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":endpoint/(?P<EndpointName>[^/]+)$"),
		},
	},
	{
		Namespace: "/aws/sagemaker/TrainingJobs",
		Alias:     "sagemaker-training",
		ResourceFilters: []*string{
			aws.String("sagemaker:training-job"),
		},
	},
	{
		Namespace: "/aws/sagemaker/ProcessingJobs",
		Alias:     "sagemaker-processing",
		ResourceFilters: []*string{
			aws.String("sagemaker:processing-job"),
		},
	},
	{
		Namespace: "/aws/sagemaker/TransformJobs",
		Alias:     "sagemaker-transform",
		ResourceFilters: []*string{
			aws.String("sagemaker:transform-job"),
		},
	},
	{
		Namespace: "/aws/sagemaker/InferenceRecommendationsJobs",
		Alias:     "sagemaker-inf-rec",
		ResourceFilters: []*string{
			aws.String("sagemaker:inference-recommendations-job"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":inference-recommendations-job/(?P<JobName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/Sagemaker/ModelBuildingPipeline",
		Alias:     "sagemaker-model-building-pipeline",
		ResourceFilters: []*string{
			aws.String("sagemaker:pipeline"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":pipeline/(?P<PipelineName>[^/]+)"),
		},
	},
	{
		Namespace: "AWS/IPAM",
		Alias:     "ipam",
		ResourceFilters: []*string{
			aws.String("ec2:ipam-pool"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":ipam-pool/(?P<IpamPoolId>[^/]+)$"),
		},
	},
	{
		Namespace: "AWS/Bedrock",
		Alias:     "bedrock",
	},
	{
		Namespace: "AWS/Events",
		Alias:     "event-rule",
		ResourceFilters: []*string{
			aws.String("events"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":rule/(?P<EventBusName>[^/]+)/(?P<RuleName>[^/]+)$"),
			regexp.MustCompile(":rule/aws.partner/(?P<EventBusName>.+)/(?P<RuleName>[^/]+)$"),
		},
	},
	{
		Namespace: "AWS/VpcLattice",
		Alias:     "vpc-lattice",
		ResourceFilters: []*string{
			aws.String("vpc-lattice:service"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":service/(?P<Service>[^/]+)$"),
		},
	},
	{
		Namespace: "AWS/Network Manager",
		Alias:     "networkmanager",
		ResourceFilters: []*string{
			aws.String("networkmanager:core-network"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":core-network/(?P<CoreNetwork>[^/]+)$"),
		},
	},
}
