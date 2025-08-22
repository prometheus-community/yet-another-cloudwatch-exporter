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
	"context"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/grafana/regexp"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

var amazonMQBrokerSuffix = regexp.MustCompile("-[0-9]+$")


// Associator implements a "best effort" algorithm to automatically map the output
// of the ListMetrics API to the list of resources retrieved from the Tagging API.
// The core logic is based on a manually maintained list of regexes that extract
// dimensions names from ARNs (see services.go). YACE supports auto-discovery for
// those AWS namespaces where the ARN regexes are correctly defined.
type Associator struct {
	// mapping from join key to tagged resource. The join key is build using the namespace and the required dimensions of the metric.
	mappings map[uint64]*model.TaggedResource

	logger       *slog.Logger
	debugEnabled bool

	// mapping from namespace to diffrent join key hash functions for a metric. The functions are sorted by the number of dimensions.
	namespaceToJoinKeyHashFunction map[string][]hashKeyFunction
}

type hashKeyFunction struct {
	dimensions []string
}

func newHashKeyFunction(dimensions []string) hashKeyFunction {
	sort.Strings(dimensions)
	return hashKeyFunction{
		dimensions: dimensions,
	}	
}

func (hf hashKeyFunction) hash(namespace string, cwMetric *model.Metric, shouldTryFixDimension bool) (uint64, bool, bool) {	
	h := xxhash.New()

	h.WriteString(namespace)

	dimensions := make([]string, 0, len(cwMetric.Dimensions))
	for _, dimension := range cwMetric.Dimensions {
		dimensions = append(dimensions, dimension.Name)
	}
	sort.Strings(dimensions)

	if !containsAll(dimensions, hf.dimensions) {
		return 0, false, false
	}

	dimFixApplied := false
	for _, dimension := range hf.dimensions {
		h.WriteString(dimension)

		for _, mDimension := range cwMetric.Dimensions {

			if shouldTryFixDimension {
				mDimension, dimFixApplied = fixDimension(namespace, mDimension)
			}

			if mDimension.Name == dimension {
				h.WriteString(mDimension.Value)
			}
		}
	}

	return h.Sum64(), true, dimFixApplied
}

// NewAssociator builds all mappings for the given dimensions regexps and list of resources.
func NewAssociator(logger *slog.Logger, dimensionsRegexps []model.DimensionsRegexp, resources []*model.TaggedResource) Associator {
	assoc := Associator{
		mappings:     map[uint64]*model.TaggedResource{},
		logger:       logger,
		debugEnabled: logger.Handler().Enabled(context.Background(), slog.LevelDebug), // caching if debug is enabled

		namespaceToJoinKeyHashFunction: map[string][]hashKeyFunction{},
	}

	// Keep track of resources that have already been indexed.
	// Each resource will be matched against at most one regex.
	// TODO(cristian): use a more memory-efficient data structure
	mappedResources := make([]bool, len(resources))

	for _, dr := range dimensionsRegexps {

		for idx, r := range resources {
			if r.Namespace != dr.Namespace {
				continue
			}

			// Skip resource that are already indexed.
			if mappedResources[idx] {
				continue
			}

			match := dr.Regexp.FindStringSubmatch(r.ARN)
			if match == nil {
				continue
			}

			labels := make(map[string]string, len(match)*2)
			for i := 1; i < len(match); i++ {
				labels[dr.DimensionsNames[i-1]] = match[i]
			}
			joinKey := assoc.getJoinKeyFromTaggedResource(r.Namespace, labels)

			assoc.mappings[joinKey] = r
			sort.Strings(dr.DimensionsNames)
			hashKeyFunction := newHashKeyFunction(dr.DimensionsNames)
			assoc.namespaceToJoinKeyHashFunction[r.Namespace] = append(assoc.namespaceToJoinKeyHashFunction[r.Namespace], hashKeyFunction)
			mappedResources[idx] = true
		}

		// The mapping might end up as empty in cases e.g. where
		// one of the regexps defined for the namespace doesn't match
		// against any of the tagged resources. This might happen for
		// example when we define multiple regexps (to capture sibling
		// or sub-resources) and one of them doesn't match any resource.
		// This behaviour is ok, we just want to debug log to keep track of it.
		if assoc.debugEnabled {
			logger.Debug("unable to define a regex mapping", "regex", dr.Regexp.String())
		}
	}

	// sort all key sets by decreasing number of dimensions names
	// (this is essential so that during matching we try to find the metric
	// with the most specific set of dimensions)
    for namespace := range assoc.namespaceToJoinKeyHashFunction {
		slices.SortStableFunc(assoc.namespaceToJoinKeyHashFunction[namespace], func(a, b hashKeyFunction) int {
			return -1 * cmp.Compare(len(a.dimensions), len(b.dimensions))
		})
	}

	if assoc.debugEnabled {
		for idx, resource := range assoc.mappings {
			logger.Debug("associator mapping", "mapping_idx", idx, "mapping", resource.ARN)
		}
		// TODO log namespaceToDimensions
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

	if assoc.debugEnabled {
		logger.Debug("associate loop start", "dimensions", strings.Join(dimensions, ","))
	}

	keySets, ok := assoc.namespaceToJoinKeyHashFunction[cwMetric.Namespace]
	if !ok {
		logger.Debug("no dimensions sets found for namespace", "namespace", cwMetric.Namespace)
		return nil, false
	}

	// Attempt to find the key set which contains the most
	// (but not necessarily all) the metric's dimensions names.
	// Key sets are sorted by decreasing number of dimensions names,
	// which favours find the mapping with most dimensions.
	dimFixApplied := false
	mappingFound := false
	match := false
	joinKey := uint64(0)
	var taggedResource *model.TaggedResource
	for _, hashKeyFunc := range keySets {
		shouldTryFixDimension := true
		joinKey, mappingFound, dimFixApplied = hashKeyFunc.hash(cwMetric.Namespace, cwMetric, shouldTryFixDimension)
		// Try again without dimension fix.
		if !mappingFound {
			joinKey, mappingFound, _ = hashKeyFunc.hash(cwMetric.Namespace, cwMetric, false)
		}

		// Try next dimensions set if still no mapping found.
		if !mappingFound {
			logger.Debug("no mapping found for metric", "metric", cwMetric.MetricName)
			continue
		}

		taggedResource, match = assoc.mappings[joinKey]
		// Try again without dimension fix.
		if !match  && dimFixApplied {
			joinKey, _, _ = hashKeyFunc.hash(cwMetric.Namespace, cwMetric, false)
			taggedResource, match = assoc.mappings[joinKey]
		}

		// If we found a match we don't need to try next dimensions set.
		if match {
			break
		}
	}
	return taggedResource, mappingFound && !match
}

// TODO: use same logic for both keys.
func (assoc Associator) getJoinKeyFromTaggedResource(namespace string, labels map[string]string) uint64 {
	h := xxhash.New()

	h.WriteString(namespace)

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		h.WriteString(key)
		h.WriteString(labels[key])
	}

	return h.Sum64()
}

// fixDimension modifies the dimension value to accommodate special cases where
// the dimension value doesn't match the resource ARN.
func fixDimension(namespace string, dim model.Dimension) (model.Dimension, bool) {
	// AmazonMQ is special - for active/standby ActiveMQ brokers,
	// the value of the "Broker" dimension contains a number suffix
	// that is not part of the resource ARN
	if namespace == "AWS/AmazonMQ" && dim.Name == "Broker" {
		if amazonMQBrokerSuffix.MatchString(dim.Value) {
			dim.Value = amazonMQBrokerSuffix.ReplaceAllString(dim.Value, "")
			return dim, true
		}
	}

	// AWS Sagemaker endpoint name and inference component name may have upper case characters
	// Resource ARN is only in lower case, hence transforming
	// name value to be able to match the resource ARN
	if namespace == "AWS/SageMaker" && (dim.Name == "EndpointName" || dim.Name == "InferenceComponentName") {
		dim.Value = strings.ToLower(dim.Value)
		return dim, true
	}

	return dim, false
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
