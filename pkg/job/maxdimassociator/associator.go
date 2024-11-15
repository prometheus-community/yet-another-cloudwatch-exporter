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
package maxdimassociator

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/regexp"
	prom_model "github.com/prometheus/common/model"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var amazonMQBrokerSuffix = regexp.MustCompile("-[0-9]+$")

// Associator implements a "best effort" algorithm to automatically map the output
// of the ListMetrics API to the list of resources retrieved from the Tagging API.
// The core logic is based on a manually maintained list of regexes that extract
// dimensions names from ARNs (see services.go). YACE supports auto-discovery for
// those AWS namespaces where the ARN regexes are correctly defined.
type Associator struct {
	// mappings is a slice of dimensions-based mappings, one for each regex of a given namespace
	mappings []*dimensionsRegexpMapping

	logger logging.Logger
}

type dimensionsRegexpMapping struct {
	// dimensions is a slice of dimensions names in a regex (normally 1 name is enough
	// to identify the resource type by its ARN, sometimes 2 or 3 dimensions names are
	// needed to identify sub-resources)
	dimensions []string

	// dimensionsMapping maps the set of dimensions (names and values) to a resource.
	// Dimensions names and values are encoded as a uint64 fingerprint.
	dimensionsMapping map[uint64]*model.TaggedResource
}

func (rm dimensionsRegexpMapping) toString() string {
	sb := strings.Builder{}
	sb.WriteString("{dimensions=[")
	for _, dim := range rm.dimensions {
		sb.WriteString(dim)
	}
	sb.WriteString("], dimensions_mappings={")
	for sign, res := range rm.dimensionsMapping {
		sb.WriteString(fmt.Sprintf("%d", sign))
		sb.WriteString("=")
		sb.WriteString(res.ARN)
		sb.WriteString(",")
	}
	sb.WriteString("}}")
	return sb.String()
}

// NewAssociator builds all mappings for the given dimensions regexps and list of resources.
func NewAssociator(logger logging.Logger, dimensionsRegexps []model.DimensionsRegexp, resources []*model.TaggedResource) Associator {
	assoc := Associator{
		mappings: []*dimensionsRegexpMapping{},
		logger:   logger,
	}

	// Keep track of resources that have already been mapped.
	// Each resource will be matched against at most one regex.
	// TODO(cristian): use a more memory-efficient data structure
	mappedResources := make([]bool, len(resources))

	for _, dr := range dimensionsRegexps {
		m := &dimensionsRegexpMapping{
			dimensions:        dr.DimensionsNames,
			dimensionsMapping: map[uint64]*model.TaggedResource{},
		}

		for idx, r := range resources {
			if mappedResources[idx] {
				continue
			}

			match := dr.Regexp.FindStringSubmatch(r.ARN)
			if match == nil {
				continue
			}

			labels := make(map[string]string, len(match))
			for i := 1; i < len(match); i++ {
				labels[dr.DimensionsNames[i-1]] = match[i]
			}
			signature := prom_model.LabelsToSignature(labels)
			m.dimensionsMapping[signature] = r
			mappedResources[idx] = true
		}

		if len(m.dimensionsMapping) > 0 {
			assoc.mappings = append(assoc.mappings, m)
		}

		// The mapping might end up as empty in cases e.g. where
		// one of the regexps defined for the namespace doesn't match
		// against any of the tagged resources. This might happen for
		// example when we define multiple regexps (to capture sibling
		// or sub-resources) and one of them doesn't match any resource.
		// This behaviour is ok, we just want to debug log to keep track of it.
		if logger.IsDebugEnabled() {
			logger.Debug("unable to define a regex mapping", "regex", dr.Regexp.String())
		}
	}

	// sort all mappings by decreasing number of dimensions names
	// (this is essential so that during matching we try to find the metric
	// with the most specific set of dimensions)
	slices.SortStableFunc(assoc.mappings, func(a, b *dimensionsRegexpMapping) int {
		return -1 * cmp.Compare(len(a.dimensions), len(b.dimensions))
	})

	if logger.IsDebugEnabled() {
		for idx, regexpMapping := range assoc.mappings {
			logger.Debug("associator mapping", "mapping_idx", idx, "mapping", regexpMapping.toString())
		}
	}

	return assoc
}

// AssociateMetricToResource finds the resource that corresponds to the given set of dimensions
// names and values of a metric. The guess is based on the mapping built from dimensions regexps.
// In case a map can't be found, the second return parameter indicates whether the metric should be
// ignored or not.
func (assoc Associator) AssociateMetricToResource(cwMetric *model.Metric) (*model.TaggedResource, bool) {
	logger := assoc.logger.With("metric_name", cwMetric.MetricName)

	if len(cwMetric.Dimensions) == 0 {
		logger.Debug("metric has no dimensions, don't skip")

		// Do not skip the metric (create a "global" metric)
		return nil, false
	}

	dimensions := make([]string, 0, len(cwMetric.Dimensions))
	for _, dimension := range cwMetric.Dimensions {
		dimensions = append(dimensions, dimension.Name)
	}

	if logger.IsDebugEnabled() {
		logger.Debug("associate loop start", "dimensions", strings.Join(dimensions, ","))
	}

	// Attempt to find the regex mapping which contains the most
	// (but not necessarily all) the metric's dimensions names.
	// Regex mappings are sorted by decreasing number of dimensions names,
	// which favours find the mapping with most dimensions.
	mappingFound := false
	for idx, regexpMapping := range assoc.mappings {
		if containsAll(dimensions, regexpMapping.dimensions) {
			if logger.IsDebugEnabled() {
				logger.Debug("found mapping", "mapping_idx", idx, "mapping", regexpMapping.toString())
			}

			// A regex mapping has been found. The metric has all (and possibly more)
			// the dimensions computed for the mapping. Now compute a signature
			// of the labels (names and values) of the dimensions of this mapping.
			mappingFound = true
			labels := buildLabelsMap(cwMetric, regexpMapping)
			signature := prom_model.LabelsToSignature(labels)

			// Check if there's an entry for the labels (names and values) of the metric,
			// and return the resource in case.
			if resource, ok := regexpMapping.dimensionsMapping[signature]; ok {
				logger.Debug("resource matched", "signature", signature)
				return resource, false
			}

			// Otherwise, continue iterating across the rest of regex mappings
			// to attempt to find another one with fewer dimensions.
			logger.Debug("resource not matched", "signature", signature)
		}
	}

	// At this point, we haven't been able to match the metric against
	// any resource based on the dimensions the associator knows.
	// If a regex mapping was ever found in the loop above but no entry
	// (i.e. matching labels names and values) matched the metric dimensions,
	// skip the metric altogether.
	// Otherwise, if we didn't find any regex mapping it means we can't
	// correctly map the dimensions names to a resource arn regex,
	// but we still want to keep the metric and create a "global" metric.
	logger.Debug("associate loop end", "skip", mappingFound)
	return nil, mappingFound
}

// buildLabelsMap returns a map of labels names and values.
// For some namespaces, values might need to be modified in order
// to match the dimension value extracted from ARN.
func buildLabelsMap(cwMetric *model.Metric, regexpMapping *dimensionsRegexpMapping) map[string]string {
	labels := make(map[string]string, len(cwMetric.Dimensions))
	for _, rDimension := range regexpMapping.dimensions {
		for _, mDimension := range cwMetric.Dimensions {
			name := mDimension.Name
			value := mDimension.Value

			// AmazonMQ is special - for active/standby ActiveMQ brokers,
			// the value of the "Broker" dimension contains a number suffix
			// that is not part of the resource ARN
			if cwMetric.Namespace == "AWS/AmazonMQ" && name == "Broker" {
				if amazonMQBrokerSuffix.MatchString(value) {
					value = amazonMQBrokerSuffix.ReplaceAllString(value, "")
				}
			}

			// AWS Sagemaker endpoint name may have upper case characters
			// Resource ARN is only in lower case, hence transforming
			// endpoint name value to be able to match the resource ARN
			if cwMetric.Namespace == "AWS/SageMaker" && name == "EndpointName" {
				value = strings.ToLower(value)
			}

			if rDimension == mDimension.Name {
				labels[name] = value
			}
		}
	}
	return labels
}

// containsAll returns true if a contains all elements of b
func containsAll(a, b []string) bool {
	for _, e := range b {
		if slices.Contains(a, e) {
			continue
		}
		return false
	}
	return true
}
