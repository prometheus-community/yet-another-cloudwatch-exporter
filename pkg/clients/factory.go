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
package clients

import (
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	cloudwatch_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/tagging"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// Factory is an interface to abstract away all logic required to produce the different
// YACE specific clients which wrap AWS clients
type Factory interface {
	GetCloudwatchClient(region string, role model.Role, concurrency cloudwatch_client.ConcurrencyConfig) cloudwatch_client.Client
	GetTaggingClient(region string, role model.Role, concurrencyLimit int) tagging.Client
	GetAccountClient(region string, role model.Role) account.Client
}
