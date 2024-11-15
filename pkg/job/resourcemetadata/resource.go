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
package resourcemetadata

import (
	"context"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Resource struct {
	// Name is an identifiable value for the resource and is variable dependent on the match made
	//	It will be the AWS ARN (Amazon Resource Name) if a unique resource was found
	//  It will be "global" if a unique resource was not found
	//  CustomNamespaces will have the custom namespace Name
	Name string
	// Tags is a set of tags associated to the resource
	Tags []model.Tag
}

type Resources struct {
	StaticResource      *Resource
	AssociatedResources []*Resource
}

type MetricResourceEnricher interface {
	Enrich(ctx context.Context, metrics []*model.Metric) ([]*model.Metric, Resources)
}
