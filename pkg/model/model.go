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
package model

import (
	"time"

	"github.com/grafana/regexp"
)

const (
	DefaultPeriodSeconds = int64(300)
	DefaultLengthSeconds = int64(300)
)

type JobsConfig struct {
	StsRegion           string
	DiscoveryJobs       []DiscoveryJob
	StaticJobs          []StaticJob
	CustomNamespaceJobs []CustomNamespaceJob
}

type DiscoveryJob struct {
	Regions                     []string
	Type                        string
	Roles                       []Role
	SearchTags                  []SearchTag
	CustomTags                  []Tag
	DimensionNameRequirements   []string
	Metrics                     []*MetricConfig
	RoundingPeriod              *int64
	RecentlyActiveOnly          bool
	ExportedTagsOnMetrics       []string
	IncludeContextOnInfoMetrics bool
	DimensionsRegexps           []DimensionsRegexp
}

type StaticJob struct {
	Name       string
	Regions    []string
	Roles      []Role
	Namespace  string
	CustomTags []Tag
	Dimensions []Dimension
	Metrics    []*MetricConfig
}

type CustomNamespaceJob struct {
	Regions                   []string
	Name                      string
	Namespace                 string
	RoundingPeriod            *int64
	RecentlyActiveOnly        bool
	Roles                     []Role
	Metrics                   []*MetricConfig
	CustomTags                []Tag
	DimensionNameRequirements []string
}

type Role struct {
	RoleArn    string
	ExternalID string
}

type MetricConfig struct {
	Name                   string
	Statistics             []string
	Period                 int64
	Length                 int64
	Delay                  int64
	NilToZero              bool
	AddCloudwatchTimestamp bool
}

type DimensionsRegexp struct {
	Regexp          *regexp.Regexp
	DimensionsNames []string
}

type LabelSet map[string]struct{}

type Tag struct {
	Key   string
	Value string
}

type SearchTag struct {
	Key   string
	Value *regexp.Regexp
}

type Dimension struct {
	Name  string
	Value string
}

type Metric struct {
	// The dimensions for the metric.
	Dimensions []Dimension
	MetricName string
	Namespace  string
}

type Datapoint struct {
	// The average of the metric values that correspond to the data point.
	Average *float64

	// The percentile statistic for the data point.
	ExtendedStatistics map[string]*float64

	// The maximum metric value for the data point.
	Maximum *float64

	// The minimum metric value for the data point.
	Minimum *float64

	// The number of metric values that contributed to the aggregate value of this
	// data point.
	SampleCount *float64

	// The sum of the metric values for the data point.
	Sum *float64

	// The time stamp used for the data point.
	Timestamp *time.Time
}

type CloudwatchMetricResult struct {
	Context *ScrapeContext
	Data    []*CloudwatchData
}

type TaggedResourceResult struct {
	Context *ScrapeContext
	Data    []*TaggedResource
}

type ScrapeContext struct {
	Region       string
	AccountID    string
	AccountAlias string
	CustomTags   []Tag
}

// CloudwatchData is an internal representation of a CloudWatch
// metric with attached data points, metric and resource information.
type CloudwatchData struct {
	MetricName string
	// ResourceName will have different values depending on the job type
	// DiscoveryJob = Resource ARN associated with the metric or global when it could not be associated but shouldn't be dropped
	// StaticJob = Resource Name from static job config
	// CustomNamespace = Custom Namespace job name
	ResourceName string
	Namespace    string
	Tags         []Tag
	Dimensions   []Dimension
	// GetMetricDataProcessingParams includes necessary fields to run GetMetricData
	GetMetricDataProcessingParams *GetMetricDataProcessingParams

	// MetricMigrationParams holds configuration values necessary when migrating the resulting metrics
	MetricMigrationParams MetricMigrationParams

	// GetMetricsDataResult is an optional field and will be non-nil when metric data was populated from the GetMetricsData API (Discovery and CustomNamespace jobs)
	GetMetricDataResult *GetMetricDataResult

	// GetMetricStatisticsResult is an optional field and will be non-nil when metric data was populated from the GetMetricStatistics API (static jobs)
	GetMetricStatisticsResult *GetMetricStatisticsResult
}

type GetMetricStatisticsResult struct {
	Datapoints []*Datapoint
	Statistics []string
}

type GetMetricDataProcessingParams struct {
	// QueryID is a value internal to processing used for mapping results from GetMetricData their original request
	QueryID string

	// The statistic to be used to call GetMetricData
	Statistic string

	// Fields which impact the start and endtime for
	Period int64
	Length int64
	Delay  int64
}

type MetricMigrationParams struct {
	NilToZero              bool
	AddCloudwatchTimestamp bool
}

type GetMetricDataResult struct {
	Statistic string
	Datapoint *float64
	Timestamp time.Time
}

// TaggedResource is an AWS resource with tags
type TaggedResource struct {
	// ARN is the unique AWS ARN (Amazon Resource Name) of the resource
	ARN string

	// Namespace identifies the resource type (e.g. EC2)
	Namespace string

	// Region is the AWS regions that the resource belongs to
	Region string

	// Tags is a set of tags associated to the resource
	Tags []Tag
}

// FilterThroughTags returns true if all filterTags match
// with tags of the TaggedResource, returns false otherwise.
func (r TaggedResource) FilterThroughTags(filterTags []SearchTag) bool {
	if len(filterTags) == 0 {
		return true
	}

	tagFilterMatches := 0

	for _, resourceTag := range r.Tags {
		for _, filterTag := range filterTags {
			if resourceTag.Key == filterTag.Key {
				if !filterTag.Value.MatchString(resourceTag.Value) {
					return false
				}
				// A resource needs to match all SearchTags to be returned, so we track the number of tag filter
				// matches to ensure it matches the number of tag filters at the end
				tagFilterMatches++
			}
		}
	}

	return tagFilterMatches == len(filterTags)
}

// MetricTags returns a list of tags built from the tags of
// TaggedResource, if exportedTags is not empty.
//
// Returned tags have as key the key from exportedTags, and
// as value the value from the corresponding tag of the resource,
// if it exists (otherwise an empty string).
func (r TaggedResource) MetricTags(exportedTags []string) []Tag {
	if len(exportedTags) == 0 {
		return []Tag{}
	}

	tags := make([]Tag, 0, len(exportedTags))
	for _, tagName := range exportedTags {
		tag := Tag{
			Key: tagName,
		}
		for _, resourceTag := range r.Tags {
			if resourceTag.Key == tagName {
				tag.Value = resourceTag.Value
				break
			}
		}

		// Always add the tag, even if it's empty, to ensure the same labels are present on all metrics for a single service
		tags = append(tags, tag)
	}
	return tags
}
