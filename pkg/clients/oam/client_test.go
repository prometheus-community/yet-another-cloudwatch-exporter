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
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_oam "github.com/aws/aws-sdk-go-v2/service/oam"
	"github.com/aws/aws-sdk-go-v2/service/oam/types"
	"github.com/prometheus/common/promslog"
	"github.com/stretchr/testify/require"
)

func Test_accountIDFromLinkArn(t *testing.T) {
	require.Equal(t, "222222222222", accountIDFromLinkArn("arn:aws:oam:eu-west-1:222222222222:link/a1b2c3d4"))
	require.Empty(t, accountIDFromLinkArn("not-an-arn"))
	require.Empty(t, accountIDFromLinkArn(""))
}

func TestListLinkedAccounts(t *testing.T) {
	pages := map[string]*aws_oam.ListAttachedLinksOutput{
		"": {
			Items: []types.ListAttachedLinksItem{
				{LinkArn: aws.String("arn:aws:oam:eu-west-1:222222222222:link/1"), Label: aws.String("Account A")},
				{LinkArn: aws.String("arn:aws:oam:eu-west-1:333333333333:link/2"), Label: nil},
			},
			NextToken: aws.String("page-2"),
		},
		"page-2": {
			Items: []types.ListAttachedLinksItem{
				{LinkArn: aws.String("arn:aws:oam:eu-west-1:444444444444:link/3"), Label: aws.String("account-b")},
			},
		},
	}

	var sinkIdentifiers []string
	c := client{
		logger: promslog.NewNopLogger(),
		oamAPI: oamClientAdapter{listAttachedLinks: func(_ context.Context, params *aws_oam.ListAttachedLinksInput, _ ...func(*aws_oam.Options)) (*aws_oam.ListAttachedLinksOutput, error) {
			sinkIdentifiers = append(sinkIdentifiers, *params.SinkIdentifier)
			return pages[aws.ToString(params.NextToken)], nil
		}},
	}

	accounts, err := c.ListLinkedAccounts(context.Background(), "sink-arn")
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"222222222222": "Account A",
		"444444444444": "account-b",
	}, accounts)
	require.Equal(t, []string{"sink-arn", "sink-arn"}, sinkIdentifiers)
}

func TestListLinkedAccountsError(t *testing.T) {
	c := client{
		logger: promslog.NewNopLogger(),
		oamAPI: oamClientAdapter{listAttachedLinks: func(context.Context, *aws_oam.ListAttachedLinksInput, ...func(*aws_oam.Options)) (*aws_oam.ListAttachedLinksOutput, error) {
			return nil, errors.New("access denied")
		}},
	}

	_, err := c.ListLinkedAccounts(context.Background(), "sink-arn")
	require.ErrorContains(t, err, "access denied")
}
