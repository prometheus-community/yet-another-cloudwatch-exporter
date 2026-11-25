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
package oam

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	aws_oam "github.com/aws/aws-sdk-go-v2/service/oam"
)

type Client interface {
	// ListLinkedAccounts calls the OAM ListAttachedLinks API for the given sink
	// identifier and returns a map of source account ID to the link label.
	// Requires the oam:ListAttachedLinks permission in the monitoring account.
	ListLinkedAccounts(ctx context.Context, sinkIdentifier string) (map[string]string, error)
}

// oamClientAdapter captures only the OAM operations used by this package as
// method-value closures, keeping the concrete *aws_oam.Client out of any
// interface-reachable struct field so the linker can drop unused operations.
type oamClientAdapter struct {
	listAttachedLinks func(ctx context.Context, params *aws_oam.ListAttachedLinksInput, optFns ...func(*aws_oam.Options)) (*aws_oam.ListAttachedLinksOutput, error)
}

func (a oamClientAdapter) ListAttachedLinks(ctx context.Context, params *aws_oam.ListAttachedLinksInput, optFns ...func(*aws_oam.Options)) (*aws_oam.ListAttachedLinksOutput, error) {
	return a.listAttachedLinks(ctx, params, optFns...)
}

// Compile-time check that the adapter satisfies the interface the AWS SDK
// paginator expects.
var _ aws_oam.ListAttachedLinksAPIClient = oamClientAdapter{}

type client struct {
	logger *slog.Logger
	oamAPI oamClientAdapter
}

func NewClient(logger *slog.Logger, oamClient *aws_oam.Client) Client {
	return &client{
		logger: logger,
		oamAPI: oamClientAdapter{listAttachedLinks: oamClient.ListAttachedLinks},
	}
}

func (c client) ListLinkedAccounts(ctx context.Context, sinkIdentifier string) (map[string]string, error) {
	accounts := make(map[string]string)

	paginator := aws_oam.NewListAttachedLinksPaginator(c.oamAPI, &aws_oam.ListAttachedLinksInput{
		SinkIdentifier: &sinkIdentifier,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("OAM ListAttachedLinks for sink %s: %w", sinkIdentifier, err)
		}
		for _, item := range page.Items {
			if item.LinkArn == nil || item.Label == nil {
				continue
			}
			if accountID := accountIDFromLinkArn(*item.LinkArn); accountID != "" {
				accounts[accountID] = *item.Label
			}
		}
	}

	c.logger.Debug("OAM ListAttachedLinks", "sink_identifier", sinkIdentifier, "linked_accounts", len(accounts))

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
