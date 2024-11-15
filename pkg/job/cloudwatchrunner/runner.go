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
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/resourcemetadata"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type ResourceEnrichment interface {
	Create(logger logging.Logger) resourcemetadata.MetricResourceEnricher
}

type Job interface {
	Namespace() string
	CustomTags() []model.Tag
	listMetricsParams() listmetrics.ProcessingParams
	resourceEnrichment() ResourceEnrichment
}
