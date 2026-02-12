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
package v1

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aws/aws-sdk-go/service/oam"
	"github.com/aws/aws-sdk-go/service/oam/oamiface"

	oam_client "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/oam"
)

type client struct {
	logger    *slog.Logger
	oamClient oamiface.OAMAPI
}

func NewClient(logger *slog.Logger, oamClient oamiface.OAMAPI) oam_client.Client {
	return &client{
		logger:    logger,
		oamClient: oamClient,
	}
}

func (c client) ListLinkedAccounts(ctx context.Context, sinkIdentifier string) (map[string]string, error) {
	accounts := make(map[string]string)

	input := &oam.ListAttachedLinksInput{
		SinkIdentifier: &sinkIdentifier,
	}

	err := c.oamClient.ListAttachedLinksPagesWithContext(ctx, input, func(page *oam.ListAttachedLinksOutput, lastPage bool) bool {
		for _, item := range page.Items {
			if item.LinkArn == nil || item.Label == nil {
				continue
			}
			accountID := accountIDFromLinkArn(*item.LinkArn)
			if accountID != "" {
				accounts[accountID] = *item.Label
			}
		}
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("OAM ListAttachedLinks for sink %s: %w", sinkIdentifier, err)
	}

	return accounts, nil
}

// accountIDFromLinkArn extracts the source account ID from a link ARN.
// Link ARN format: arn:aws:oam:REGION:ACCOUNT_ID:link/LINK_ID
func accountIDFromLinkArn(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}
