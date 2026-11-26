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
package clients

import (
	"sync"
	"testing"

	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

// Static jobs that share a role and region request the account client
// concurrently after Refresh, which does not create it for them.
func TestCachingFactory_GetAccountClient_ConcurrentStaticJobs(t *testing.T) {
	role := model.Role{}
	region := "us-east-1"
	factory, err := NewFactory(promslog.NewNopLogger(), promutil.Discard, model.JobsConfig{
		StaticJobs: []model.StaticJob{{Roles: []model.Role{role}, Regions: []string{region}}},
	}, false)
	require.NoError(t, err)

	for range 20 {
		factory.Refresh()

		const workers = 32
		clients := make([]account.Client, workers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				clients[i] = factory.GetAccountClient(region, role)
			}()
		}
		close(start)
		wg.Wait()

		for _, client := range clients {
			require.NotNil(t, client)
			assert.Same(t, clients[0], client)
		}

		factory.Clear()
	}
}
