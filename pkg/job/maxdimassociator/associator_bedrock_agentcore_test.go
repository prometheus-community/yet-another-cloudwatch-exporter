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
package maxdimassociator

import (
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var agentCoreRuntime = &model.TaggedResource{
	ARN:       "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/my_agent-AbCdEf1234",
	Namespace: "AWS/Bedrock-AgentCore",
}

// The tagging API returns runtime endpoints under the same resource type as
// runtimes, but metrics only ever carry the runtime ARN.
var agentCoreRuntimeEndpoint = &model.TaggedResource{
	ARN:       "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/my_agent-AbCdEf1234/runtime-endpoint/DEFAULT",
	Namespace: "AWS/Bedrock-AgentCore",
}

var agentCoreMemory = &model.TaggedResource{
	ARN:       "arn:aws:bedrock-agentcore:us-east-1:123456789012:memory/my_memory-AbCdEf1234",
	Namespace: "AWS/Bedrock-AgentCore",
}

var agentCoreGateway = &model.TaggedResource{
	ARN:       "arn:aws:bedrock-agentcore:us-east-1:123456789012:gateway/my-gateway-abcdef1234",
	Namespace: "AWS/Bedrock-AgentCore",
}

var agentCoreResources = []*model.TaggedResource{
	agentCoreRuntimeEndpoint,
	agentCoreRuntime,
	agentCoreMemory,
	agentCoreGateway,
}

func TestAssociatorBedrockAgentCore(t *testing.T) {
	type args struct {
		dimensionRegexps []model.DimensionsRegexp
		resources        []*model.TaggedResource
		metric           *model.Metric
	}

	type testCase struct {
		name             string
		args             args
		expectedSkip     bool
		expectedResource *model.TaggedResource
	}

	testcases := []testCase{
		{
			name: "should match runtime invocation metric with Resource, Operation and Name dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Resource", Value: "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/my_agent-AbCdEf1234"},
						{Name: "Operation", Value: "InvokeAgentRuntime"},
						{Name: "Name", Value: "my_agent::DEFAULT"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: agentCoreRuntime,
		},
		{
			name: "should match runtime resource usage metric with Service and Resource dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "CPUUsed-vCPUHours",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Service", Value: "AgentCore.Runtime"},
						{Name: "Resource", Value: "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/my_agent-AbCdEf1234"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: agentCoreRuntime,
		},
		{
			name: "should match memory metric with Resource and Operation dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "Latency",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Resource", Value: "arn:aws:bedrock-agentcore:us-east-1:123456789012:memory/my_memory-AbCdEf1234"},
						{Name: "Operation", Value: "GetMemory"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: agentCoreMemory,
		},
		{
			name: "should match gateway metric with Resource, Operation, Protocol and Method dimensions",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Resource", Value: "arn:aws:bedrock-agentcore:us-east-1:123456789012:gateway/my-gateway-abcdef1234"},
						{Name: "Operation", Value: "InvokeGateway"},
						{Name: "Protocol", Value: "MCP"},
						{Name: "Method", Value: "tools/list"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: agentCoreGateway,
		},
		{
			name: "should skip runtime metric for a runtime that is not tagged",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Resource", Value: "arn:aws:bedrock-agentcore:us-east-1:123456789012:runtime/other_agent-ZyXwVu9876"},
						{Name: "Operation", Value: "InvokeAgentRuntime"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should skip browser metric whose Resource dimension is an ID rather than an ARN",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "Invocations",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Resource", Value: "my_browser-AbCdEf1234"},
						{Name: "Operation", Value: "StartBrowserSession"},
					},
				},
			},
			expectedSkip:     true,
			expectedResource: nil,
		},
		{
			name: "should not skip account-level metric without Resource dimension",
			args: args{
				dimensionRegexps: config.SupportedServices.GetService("AWS/Bedrock-AgentCore").ToModelDimensionsRegexp(),
				resources:        agentCoreResources,
				metric: &model.Metric{
					MetricName: "ActiveSessionCount",
					Namespace:  "AWS/Bedrock-AgentCore",
					Dimensions: []model.Dimension{
						{Name: "Service", Value: "AgentCore.Runtime"},
					},
				},
			},
			expectedSkip:     false,
			expectedResource: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			associator := NewAssociator(promslog.NewNopLogger(), tc.args.dimensionRegexps, tc.args.resources)
			res, skip := associator.AssociateMetricToResource(tc.args.metric)
			require.Equal(t, tc.expectedSkip, skip)
			require.Equal(t, tc.expectedResource, res)
		})
	}
}
