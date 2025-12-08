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
package enhanced

import (
	"context"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/enhanced/rds"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// Processor orchestrates enhanced metric collection
type Processor struct {
	services map[string]Service
	logger   *slog.Logger
}

// NewProcessor creates a new enhanced metrics processor
func NewProcessor(logger *slog.Logger) *Processor {
	p := &Processor{
		services: make(map[string]Service),
		logger:   logger,
	}

	// Register RDS service
	p.RegisterService(rds.NewService(logger))

	// Additional services will be registered here as they are implemented
	// p.RegisterService(lambda.NewService(logger))
	// p.RegisterService(dynamodb.NewService(logger))

	return p
}

// RegisterService registers an enhanced metrics service
func (p *Processor) RegisterService(svc Service) {
	p.services[svc.GetNamespace()] = svc
}

// GetService returns the service for a given namespace
func (p *Processor) GetService(namespace string) Service {
	return p.services[namespace]
}

// Process fetches enhanced metrics for the given resources
func (p *Processor) Process(
	ctx context.Context,
	namespace string,
	resources []*model.TaggedResource,
	requestedMetrics []string,
	exportedTags []string,
) ([]*model.CloudwatchData, error) {
	svc, ok := p.services[namespace]
	if !ok {
		// No enhanced metrics service for this namespace
		return nil, nil
	}

	// Filter to only resources in this namespace
	namespacedResources := filterResourcesByNamespace(resources, namespace)
	if len(namespacedResources) == 0 {
		return nil, nil
	}

	// Get enabled metrics that this service supports
	supportedMetrics := svc.GetSupportedMetrics()
	metricsToFetch := intersect(requestedMetrics, supportedMetrics)
	if len(metricsToFetch) == 0 {
		p.logger.Debug("No supported enhanced metrics requested", "namespace", namespace)
		return nil, nil
	}

	p.logger.Debug("Fetching enhanced metrics", "namespace", namespace, "metrics", metricsToFetch, "resources", len(namespacedResources))

	// Fetch enhanced metrics for these resources
	return svc.FetchEnhancedMetrics(ctx, namespacedResources, metricsToFetch, exportedTags)
}

func filterResourcesByNamespace(resources []*model.TaggedResource, namespace string) []*model.TaggedResource {
	filtered := make([]*model.TaggedResource, 0, len(resources))
	for _, r := range resources {
		if r.Namespace == namespace {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func intersect(requested, supported []string) []string {
	supportedSet := make(map[string]struct{}, len(supported))
	for _, s := range supported {
		supportedSet[s] = struct{}{}
	}

	result := make([]string, 0, len(requested))
	for _, r := range requested {
		if _, ok := supportedSet[r]; ok {
			result = append(result, r)
		}
	}
	return result
}
