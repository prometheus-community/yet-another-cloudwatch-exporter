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
package cloudwatchrunner

import (
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/listmetrics"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type DiscoveryJob struct {
	Job       model.DiscoveryJob
	Resources []*model.TaggedResource
}

func (d DiscoveryJob) Namespace() string {
	return d.Job.Type
}

func (d DiscoveryJob) CustomTags() []model.Tag {
	return d.Job.CustomTags
}

func (d DiscoveryJob) listMetricsParams() listmetrics.ProcessingParams {
	return listmetrics.ProcessingParams{
		Namespace:                 d.Job.Type,
		Metrics:                   d.Job.Metrics,
		RecentlyActiveOnly:        d.Job.RecentlyActiveOnly,
		DimensionNameRequirements: d.Job.DimensionNameRequirements,
	}
}

func (d DiscoveryJob) resourceEnrichment() ResourceEnrichment {
	// TODO add implementation in followup
	return nil
}
