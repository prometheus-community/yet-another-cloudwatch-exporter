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
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/grafana/regexp"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// ArnFromDimensionsFunc builds a resource ARN directly from a ListMetrics result's dimensions,
// without requiring the resource to have first been discovered through the Tagging API (GetResources).
// dimensions is keyed by the same names used in this file's DimensionRegexps named capture groups
// (e.g. "FunctionName", "TableName", "Directory_ID"), which matches the real AWS CloudWatch dimension
// name for that namespace with spaces replaced by underscores.
//
// Returns false when the supplied dimensions don't contain enough information to build a valid ARN.
//
// These are best-effort, statically derived from documented AWS ARN formats and have not been
// validated against live AWS API responses; verify against real output before relying on them.
type ArnFromDimensionsFunc func(region, accountID string, dimensions map[string]string) (string, bool)

// partitionForRegion infers the AWS partition from a region name, the same way the AWS SDK's own
// partition resolver does: region prefix determines partition. Region is always known at the call
// site (it's what ListMetrics/GetResources were just queried against), so there's no need for a
// caller-supplied partition that could disagree with it.
func partitionForRegion(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	default:
		return "aws"
	}
}

// arnFromDimensions returns an ArnFromDimensionsFunc that builds a standard
// "arn:{partition}:{service}:{region}:{account}:{resource}" ARN, substituting the named dimensions (in
// order) into resourceFormat's %s verbs. Returns false if any of dimensionNames is missing or empty.
func arnFromDimensions(service, resourceFormat string, dimensionNames ...string) ArnFromDimensionsFunc {
	return func(region, accountID string, dimensions map[string]string) (string, bool) {
		resource, ok := formatResource(resourceFormat, dimensions, dimensionNames)
		if !ok {
			return "", false
		}
		return fmt.Sprintf("arn:%s:%s:%s:%s:%s", partitionForRegion(region), service, region, accountID, resource), true
	}
}

// arnFromDimensionsNoRegion is like arnFromDimensions, but for global services whose ARNs omit the
// region segment (e.g. CloudFront, Global Accelerator, Network Manager). Partition is still inferred
// from region, since the region the metric was fetched from still tells us which partition it's in.
func arnFromDimensionsNoRegion(service, resourceFormat string, dimensionNames ...string) ArnFromDimensionsFunc {
	return func(region, accountID string, dimensions map[string]string) (string, bool) {
		resource, ok := formatResource(resourceFormat, dimensions, dimensionNames)
		if !ok {
			return "", false
		}
		return fmt.Sprintf("arn:%s:%s::%s:%s", partitionForRegion(region), service, accountID, resource), true
	}
}

// arnFromDimensionsNoAccount is like arnFromDimensions, but for services whose ARNs omit both the
// region and account segments (e.g. S3, Route 53).
func arnFromDimensionsNoAccount(service, resourceFormat string, dimensionNames ...string) ArnFromDimensionsFunc {
	return func(region, _ string, dimensions map[string]string) (string, bool) {
		resource, ok := formatResource(resourceFormat, dimensions, dimensionNames)
		if !ok {
			return "", false
		}
		return fmt.Sprintf("arn:%s:%s:::%s", partitionForRegion(region), service, resource), true
	}
}

func formatResource(resourceFormat string, dimensions map[string]string, dimensionNames []string) (string, bool) {
	args := make([]any, 0, len(dimensionNames))
	for _, name := range dimensionNames {
		v, ok := dimensions[name]
		if !ok || v == "" {
			return "", false
		}
		args = append(args, v)
	}
	return fmt.Sprintf(resourceFormat, args...), true
}

// identityArn returns an ArnFromDimensionsFunc for namespaces where CloudWatch already publishes the
// full resource ARN as the dimension's value (e.g. CertificateArn, StateMachineArn).
func identityArn(dimensionName string) ArnFromDimensionsFunc {
	return func(_, _ string, dimensions map[string]string) (string, bool) {
		v, ok := dimensions[dimensionName]
		return v, ok && v != ""
	}
}

// firstOf tries each ArnFromDimensionsFunc in order and returns the first one that succeeds. Used for
// namespaces where a metric carries one of several possible resource-identifying dimension sets.
func firstOf(fns ...ArnFromDimensionsFunc) ArnFromDimensionsFunc {
	return func(region, accountID string, dimensions map[string]string) (string, bool) {
		for _, fn := range fns {
			if arn, ok := fn(region, accountID, dimensions); ok {
				return arn, true
			}
		}
		return "", false
	}
}

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
	// ArnFromDimensions builds this namespace's resource ARN directly from a metric's dimensions,
	// without needing the resource to be discovered via the Tagging API first. It is nil when there's
	// no trivial way to derive the ARN from dimensions alone -- e.g. the canonical ARN embeds an
	// internal identifier (a GUID/hash) that CloudWatch doesn't expose as a dimension, the ARN
	// structure has ambiguous or undocumented edge cases, or the namespace has no fixed resource type.
	ArnFromDimensions ArnFromDimensionsFunc
}

func (sc ServiceConfig) ToModelDimensionsRegexp() []model.DimensionsRegexp {
	dr := []model.DimensionsRegexp{}

	for _, dimensionRegexp := range sc.DimensionRegexps {
		names := dimensionRegexp.SubexpNames()
		dimensionNames := make([]string, 0, len(names)-1)

		// skip first name, it's always an empty string
		for i := 1; i < len(names); i++ {
			// in the regex names we use underscores where AWS dimensions have spaces
			dimensionNames = append(dimensionNames, strings.ReplaceAll(names[i], "_", " "))
		}

		dr = append(dr, model.DimensionsRegexp{
			Regexp:          dimensionRegexp,
			DimensionsNames: dimensionNames,
		})
	}

	return dr
}

func (sc ServiceConfig) toModelEnhancedMetricsConfig(ems []*EnhancedMetric) []*model.EnhancedMetricConfig {
	emc := make([]*model.EnhancedMetricConfig, 0, len(ems))

	for _, em := range ems {
		emc = append(emc, &model.EnhancedMetricConfig{
			Name: em.Name,
		})
	}

	return emc
}

func (sc ServiceConfig) toInferArnFromDimensionsFunc() model.InferArnFromDimensionsFunc {
	if sc.ArnFromDimensions == nil {
		return nil
	}

	return func(region, accountID string, dimensions []model.Dimension) (string, bool) {
		dimensionsMap := make(map[string]string, len(dimensions))
		for _, d := range dimensions {
			// ArnFromDimensions is keyed by this file's capture-group names, which use underscores
			// where the real AWS dimension name has spaces (see ToModelDimensionsRegexp). Names that
			// genuinely contain an underscore, like AWS/RUM's "application_name", pass through
			// unchanged.
			dimensionsMap[strings.ReplaceAll(d.Name, " ", "_")] = d.Value
		}
		return sc.ArnFromDimensions(region, accountID, dimensionsMap)
	}
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
		// Generic namespace for custom metrics pushed by the CloudWatch Agent -- no fixed resource
		// type, so there's no ARN to reconstruct.
		Namespace: "CWAgent",
		Alias:     "cwagent",
	},
	{
		// Account/service-level API usage and quota metrics, not tied to a discrete resource.
		Namespace: "AWS/Usage",
		Alias:     "usage",
	},
	{
		Namespace: "AWS/CertificateManager",
		Alias:     "acm",
		ResourceFilters: []*string{
			aws.String("acm:certificate"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<CertificateArn>.*)"),
		},
		ArnFromDimensions: identityArn("CertificateArn"),
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
		ArnFromDimensions: identityArn("PrivateCAArn"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("elasticloadbalancing", "%s", "TargetGroup"),
			arnFromDimensions("elasticloadbalancing", "loadbalancer/%s", "LoadBalancer"),
		),
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
		ArnFromDimensions: arnFromDimensions("appstream", "fleet/%s", "FleetName"),
	},
	{
		Namespace: "AWS/Backup",
		Alias:     "backup",
		ResourceFilters: []*string{
			aws.String("backup"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":backup-vault:(?P<BackupVaultName>[^:]+)"),
		},
		ArnFromDimensions: arnFromDimensions("backup", "backup-vault:%s", "BackupVaultName"),
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
		// No trivial reconstruction: API Gateway management ARNs omit the account segment entirely
		// (e.g. "arn:aws:apigateway:region::/restapis/id") and the shape varies across REST/HTTP/
		// Websocket/stage/route variants.
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
		// No trivial reconstruction: unclear whether the "Broker" dimension value alone (name or id)
		// is sufficient to build the canonical broker ARN.
	},
	{
		Namespace: "AWS/AppRunner",
		Alias:     "apprunner",
		ResourceFilters: []*string{
			aws.String("apprunner:service"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":service/(?P<ServiceName>[^/]+)/(?P<ServiceID>[^/]+)$"),
		},
		ArnFromDimensions: arnFromDimensions("apprunner", "service/%s/%s", "ServiceName", "ServiceID"),
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
		ArnFromDimensions: arnFromDimensions("appsync", "apis/%s", "GraphQLAPIId"),
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
		ArnFromDimensions: arnFromDimensions("athena", "workgroup/%s", "WorkGroup"),
	},
	{
		Namespace: "AWS/AutoScaling",
		Alias:     "asg",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("autoScalingGroupName/(?P<AutoScalingGroupName>[^/]+)"),
		},
		// No trivial reconstruction: the canonical ASG ARN embeds an internal group ID
		// ("autoScalingGroup:{group-id}:autoScalingGroupName/{name}") that CloudWatch doesn't expose
		// as a dimension.
	},
	{
		Namespace: "AWS/ElasticBeanstalk",
		Alias:     "beanstalk",
		ResourceFilters: []*string{
			aws.String("elasticbeanstalk:environment"),
		},
		DimensionRegexps: []*regexp.Regexp{
			// arn uses /${ApplicationName}/${EnvironmentName}, but only EnvironmentName is a Metric Dimension
			regexp.MustCompile("environment/[^/]+/(?P<EnvironmentName>[^/]+)"),
		},
		// No trivial reconstruction: the ARN also requires ApplicationName, which isn't a CloudWatch
		// dimension for this namespace.
	},
	{
		// Account-level billing/cost metrics, not tied to a discrete resource.
		Namespace: "AWS/Billing",
		Alias:     "billing",
	},
	{
		Namespace: "AWS/Cassandra",
		Alias:     "cassandra",
		ResourceFilters: []*string{
			aws.String("cassandra"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("keyspace/(?P<Keyspace>[^/]+)/table/(?P<TableName>[^/]+)"),
			regexp.MustCompile("keyspace/(?P<Keyspace>[^/]+)/"),
		},
		ArnFromDimensions: firstOf(
			arnFromDimensions("cassandra", "keyspace/%s/table/%s", "Keyspace", "TableName"),
			arnFromDimensions("cassandra", "keyspace/%s", "Keyspace"),
		),
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
		// CloudFront is a global service; its ARNs omit the region segment.
		ArnFromDimensions: arnFromDimensionsNoRegion("cloudfront", "distribution/%s", "DistributionId"),
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
		ArnFromDimensions: arnFromDimensions("cognito-idp", "userpool/%s", "UserPool"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("datasync", "task/%s", "TaskId"),
			arnFromDimensions("datasync", "agent/%s", "AgentId"),
		),
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
		ArnFromDimensions: arnFromDimensions("ds", "directory/%s", "Directory_ID"),
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
		// No trivial reconstruction: the ARN contains an additional segment (before the identifier
		// captured above) that isn't exposed as a CloudWatch dimension.
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
		ArnFromDimensions: identityArn("ResourceArn"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("rds", "cluster:%s", "DBClusterIdentifier"),
			arnFromDimensions("rds", "db:%s", "DBInstanceIdentifier"),
		),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("directconnect", "dxcon/%s", "ConnectionId"),
			arnFromDimensions("directconnect", "dxlag/%s", "LagId"),
			arnFromDimensions("directconnect", "dxvif/%s", "VirtualInterfaceId"),
		),
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
		ArnFromDimensions: arnFromDimensions("dynamodb", "table/%s", "TableName"),
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
		ArnFromDimensions: arnFromDimensions("ec2", "volume/%s", "VolumeId"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("elasticache", "cluster:%s", "CacheClusterId"),
			arnFromDimensions("elasticache", "serverlesscache:%s", "clusterId"),
		),
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
		ArnFromDimensions: arnFromDimensions("memorydb", "cluster/%s", "ClusterName"),
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
		ArnFromDimensions: arnFromDimensions("ec2", "instance/%s", "InstanceId"),
	},
	{
		Namespace: "AWS/EC2Spot",
		Alias:     "ec2Spot",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<FleetRequestId>.*)"),
		},
		ArnFromDimensions: arnFromDimensions("ec2", "spot-fleet-request/%s", "FleetRequestId"),
	},
	{
		Namespace: "AWS/EC2CapacityReservations",
		Alias:     "ec2CapacityReservations",
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":capacity-reservation/(?P<CapacityReservationId>)$"),
		},
		ArnFromDimensions: arnFromDimensions("ec2", "capacity-reservation/%s", "CapacityReservationId"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("ecs", "service/%s/%s", "ClusterName", "ServiceName"),
			arnFromDimensions("ecs", "cluster/%s", "ClusterName"),
		),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("ecs", "service/%s/%s", "ClusterName", "ServiceName"),
			arnFromDimensions("ecs", "cluster/%s", "ClusterName"),
		),
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
		ArnFromDimensions: arnFromDimensions("eks", "cluster/%s", "ClusterName"),
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
		ArnFromDimensions: arnFromDimensions("elasticfilesystem", "file-system/%s", "FileSystemId"),
	},
	{
		Namespace: "AWS/EKS",
		Alias:     "eks",
		ResourceFilters: []*string{
			aws.String("eks:cluster"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":cluster/(?P<ClusterName>[^/]+)$"),
		},
		ArnFromDimensions: arnFromDimensions("eks", "cluster/%s", "ClusterName"),
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
		ArnFromDimensions: arnFromDimensions("elasticloadbalancing", "loadbalancer/%s", "LoadBalancerName"),
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
		ArnFromDimensions: arnFromDimensions("elasticmapreduce", "cluster/%s", "JobFlowId"),
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
		// No trivial reconstruction: EMR Serverless ARNs are documented with a leading slash after the
		// account segment ("arn:aws:emr-serverless:region:account:/applications/id"), which the
		// standard template doesn't produce and hasn't been verified here.
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
		ArnFromDimensions: arnFromDimensions("es", "domain/%s", "DomainName"),
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
		ArnFromDimensions: arnFromDimensions("firehose", "deliverystream/%s", "DeliveryStreamName"),
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
		ArnFromDimensions: arnFromDimensions("fsx", "file-system/%s", "FileSystemId"),
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
		ArnFromDimensions: arnFromDimensions("gamelift", "fleet/%s", "FleetId"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("elasticloadbalancing", "%s", "TargetGroup"),
			arnFromDimensions("elasticloadbalancing", "loadbalancer/%s", "LoadBalancer"),
		),
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
		// Global Accelerator is a global service; its ARNs omit the region segment. Only the top-level
		// accelerator is covered here -- listener/endpoint-group sub-resource ARN nesting isn't.
		ArnFromDimensions: arnFromDimensionsNoRegion("globalaccelerator", "accelerator/%s", "Accelerator"),
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
		ArnFromDimensions: arnFromDimensions("glue", "job/%s", "JobName"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("iot", "rule/%s", "RuleName"),
			arnFromDimensions("iot", "provisioningtemplate/%s", "TemplateName"),
		),
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
		// No trivial reconstruction: the canonical MSK cluster ARN embeds a UUID after the cluster
		// name that CloudWatch doesn't expose as a dimension.
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
		// No trivial reconstruction: same UUID-suffix issue as AWS/Kafka.
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
		ArnFromDimensions: arnFromDimensions("kinesis", "stream/%s", "StreamName"),
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
		ArnFromDimensions: arnFromDimensions("kinesisanalytics", "application/%s", "Application"),
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
		ArnFromDimensions: arnFromDimensions("kms", "key/%s", "KeyId"),
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
		ArnFromDimensions: arnFromDimensions("lambda", "function:%s", "FunctionName"),
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
		ArnFromDimensions: arnFromDimensions("logs", "log-group:%s", "LogGroupName"),
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
		ArnFromDimensions: firstOf(
			identityArn("FlowARN"),
			identityArn("SourceARN"),
			identityArn("OutputARN"),
		),
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
		ArnFromDimensions: identityArn("Queue"),
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
		// No trivial reconstruction: unclear whether the "IngestEndpoint" dimension value maps
		// directly to its parent channel's ARN identifier.
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
		ArnFromDimensions: arnFromDimensions("medialive", "channel:%s", "ChannelId"),
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
		ArnFromDimensions: arnFromDimensions("mediatailor", "playbackConfiguration/%s", "ConfigurationName"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("rds", "cluster:%s", "DBClusterIdentifier"),
			arnFromDimensions("rds", "db:%s", "DBInstanceIdentifier"),
		),
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
		ArnFromDimensions: arnFromDimensions("network-firewall", "firewall/%s", "FirewallName"),
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
		ArnFromDimensions: arnFromDimensions("ec2", "natgateway/%s", "NatGatewayId"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("elasticloadbalancing", "%s", "TargetGroup"),
			arnFromDimensions("elasticloadbalancing", "loadbalancer/%s", "LoadBalancer"),
		),
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
		ArnFromDimensions: arnFromDimensions("ec2", "vpc-endpoint/%s", "VPC_Endpoint_Id"),
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
		ArnFromDimensions: arnFromDimensions("ec2", "vpc-endpoint-service/%s", "Service_Id"),
	},
	{
		// Verified against a live workspace: the "Workspace" dimension is the workspace ID and maps
		// directly onto the ARN (service segment is "aps", not "prometheus").
		Namespace: "AWS/Prometheus",
		Alias:     "amp",
		ResourceFilters: []*string{
			aws.String("aps:workspace"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":workspace/(?P<Workspace>[^/]+)"),
		},
		ArnFromDimensions: arnFromDimensions("aps", "workspace/%s", "Workspace"),
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
		ArnFromDimensions: arnFromDimensions("qldb", "ledger/%s", "LedgerName"),
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("rds", "cluster:%s", "DBClusterIdentifier"),
			arnFromDimensions("rds", "db:%s", "DBInstanceIdentifier"),
			arnFromDimensions("rds", "db-proxy:%s", "ProxyIdentifier"),
		),
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
		ArnFromDimensions: arnFromDimensions("redshift", "cluster:%s", "ClusterIdentifier"),
	},
	{
		// Verified against a live namespace/workgroup: CloudWatch's "Namespace" and "Workgroup"
		// dimensions carry the resource *name* (e.g. "my-workgroup"), but the canonical ARN embeds an
		// internal UUID instead of the name (e.g. "workgroup/3c63945d-3bea-4863-8c5c-7d0b5b285288").
		// The UUID isn't derivable from the name without an API call, so neither DimensionRegexps nor
		// ArnFromDimensions can bridge this -- same failure mode as AWS/AutoScaling and AWS/Kafka.
		Namespace: "AWS/Redshift-Serverless",
		Alias:     "redshift",
		ResourceFilters: []*string{
			aws.String("redshift-serverless:workgroup"),
			aws.String("redshift-serverless:namespace"),
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
		ArnFromDimensions: arnFromDimensions("route53resolver", "resolver-endpoint/%s", "EndpointId"),
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
		// Route 53 is a global service; its ARNs omit both the region and account segments.
		ArnFromDimensions: arnFromDimensionsNoAccount("route53", "healthcheck/%s", "HealthCheckId"),
	},
	{
		Namespace: "AWS/RUM",
		Alias:     "rum",
		ResourceFilters: []*string{
			aws.String("rum:appmonitor"),
		},
		// Real CloudWatch dimension name is "application_name" (lowercase with underscore), not a
		// space-converted PascalCase name -- no DimensionRegexps entry to avoid this file's usual
		// underscore-means-space convention mangling it into "application name".
		ArnFromDimensions: arnFromDimensions("rum", "appmonitor/%s", "application_name"),
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
		// S3 bucket ARNs omit both the region and account segments.
		ArnFromDimensions: arnFromDimensionsNoAccount("s3", "%s", "BucketName"),
	},
	{
		// Verified against a live schedule: CloudWatch only publishes AWS/Scheduler metrics dimensioned
		// by "ScheduleGroup" -- there's no per-schedule dimension at all, so the resource this
		// reconstructs is the schedule group, not an individual schedule.
		Namespace: "AWS/Scheduler",
		Alias:     "scheduler",
		ResourceFilters: []*string{
			aws.String("scheduler:schedule-group"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":schedule-group/(?P<ScheduleGroup>[^/]+)"),
		},
		ArnFromDimensions: arnFromDimensions("scheduler", "schedule-group/%s", "ScheduleGroup"),
	},
	{
		Namespace: "AWS/ECR",
		Alias:     "ecr",
		ResourceFilters: []*string{
			aws.String("ecr:repository"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":repository/(?P<RepositoryName>.+)$"),
		},
		ArnFromDimensions: arnFromDimensions("ecr", "repository/%s", "RepositoryName"),
	},
	{
		Namespace: "AWS/Timestream",
		Alias:     "timestream",
		ResourceFilters: []*string{
			aws.String("timestream:database"),
			aws.String("timestream:table"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":database/(?P<DatabaseName>[^/]+)/table/(?P<TableName>[^/]+)"),
			regexp.MustCompile(":database/(?P<DatabaseName>[^/]+)$"),
		},
		ArnFromDimensions: firstOf(
			arnFromDimensions("timestream", "database/%s/table/%s", "DatabaseName", "TableName"),
			arnFromDimensions("timestream", "database/%s", "DatabaseName"),
		),
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
		ArnFromDimensions: identityArn("StateMachineArn"),
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
		ArnFromDimensions: arnFromDimensions("sns", "%s", "TopicName"),
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
		ArnFromDimensions: arnFromDimensions("sqs", "%s", "QueueName"),
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
		// No trivial reconstruction: the three regex variants disagree on what "GatewayId" contains
		// (a bare ID vs. a compound "id/name" string), so a single dimension-based template would be
		// wrong for at least one of them.
	},
	{
		Namespace: "AWS/Transfer",
		Alias:     "transfer",
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
		ArnFromDimensions: arnFromDimensions("ec2", "transit-gateway/%s", "TransitGateway"),
	},
	{
		// Trusted Advisor check-result metrics, not resource metrics -- no ARN to reconstruct.
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
		ArnFromDimensions: arnFromDimensions("ec2", "vpn-connection/%s", "VpnId"),
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
		ArnFromDimensions: arnFromDimensions("ec2", "client-vpn-endpoint/%s", "Endpoint"),
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
		// No trivial reconstruction: WAFV2 ARNs require a scope (REGIONAL/CLOUDFRONT) and an internal
		// UUID that CloudWatch doesn't expose as a dimension.
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
		ArnFromDimensions: firstOf(
			arnFromDimensions("workspaces", "workspace/%s", "WorkspaceId"),
			arnFromDimensions("workspaces", "directory/%s", "DirectoryId"),
		),
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
		ArnFromDimensions: arnFromDimensions("aoss", "collection/%s", "CollectionId"),
	},
	{
		Namespace: "AWS/SageMaker",
		Alias:     "sagemaker",
		ResourceFilters: []*string{
			aws.String("sagemaker:endpoint"),
			aws.String("sagemaker:inference-component"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":endpoint/(?P<EndpointName>[^/]+)$"),
			regexp.MustCompile(":inference-component/(?P<InferenceComponentName>[^/]+)$"),
		},
		ArnFromDimensions: firstOf(
			arnFromDimensions("sagemaker", "endpoint/%s", "EndpointName"),
			arnFromDimensions("sagemaker", "inference-component/%s", "InferenceComponentName"),
		),
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
		ArnFromDimensions: arnFromDimensions("sagemaker", "endpoint/%s", "EndpointName"),
	},
	{
		Namespace: "/aws/sagemaker/InferenceComponents",
		Alias:     "sagemaker-inference-components",
		ResourceFilters: []*string{
			aws.String("sagemaker:inference-component"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile(":inference-component/(?P<InferenceComponentName>[^/]+)$"),
		},
		ArnFromDimensions: arnFromDimensions("sagemaker", "inference-component/%s", "InferenceComponentName"),
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
		ArnFromDimensions: arnFromDimensions("sagemaker", "inference-recommendations-job/%s", "JobName"),
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
		ArnFromDimensions: arnFromDimensions("sagemaker", "pipeline/%s", "PipelineName"),
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
		ArnFromDimensions: arnFromDimensions("ec2", "ipam-pool/%s", "IpamPoolId"),
	},
	{
		// On-demand foundation model invocation metrics; foundation models are AWS-owned, not a
		// discrete customer resource with an ARN in this account (unlike Bedrock/Agents and
		// Bedrock/Guardrails below, which are customer-owned resources).
		Namespace: "AWS/Bedrock",
		Alias:     "bedrock",
	},
	{
		Namespace: "AWS/Bedrock/Agents",
		Alias:     "bedrock-agents",
		ResourceFilters: []*string{
			aws.String("bedrock:agent-alias"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<AgentAliasArn>.+)"),
		},
		ArnFromDimensions: identityArn("AgentAliasArn"),
	},
	{
		Namespace: "AWS/Bedrock/Guardrails",
		Alias:     "bedrock-guardrails",
		ResourceFilters: []*string{
			aws.String("bedrock:guardrail"),
		},
		DimensionRegexps: []*regexp.Regexp{
			regexp.MustCompile("(?P<GuardrailArn>.+)"),
		},
		ArnFromDimensions: identityArn("GuardrailArn"),
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
		// No trivial reconstruction: rules on the default event bus omit the event-bus segment
		// entirely from the ARN, and it's not verified here whether the "EventBusName" dimension
		// reports a sentinel value or is simply absent in that case.
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
		ArnFromDimensions: arnFromDimensions("vpc-lattice", "service/%s", "Service"),
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
		// Network Manager is a global service; its ARNs omit the region segment.
		ArnFromDimensions: arnFromDimensionsNoRegion("networkmanager", "core-network/%s", "CoreNetwork"),
	},
}
