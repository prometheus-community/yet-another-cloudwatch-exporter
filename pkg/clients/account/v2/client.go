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
package v2

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/account"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
)

type client struct {
	logger    logging.Logger
	stsClient *sts.Client
	iamClient *iam.Client
}

func NewClient(logger logging.Logger, stsClient *sts.Client, iamClient *iam.Client) account.Client {
	return &client{
		logger:    logger,
		stsClient: stsClient,
		iamClient: iamClient,
	}
}

func (c client) GetAccount(ctx context.Context) (string, error) {
	result, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	if result.Account == nil {
		return "", errors.New("aws sts GetCallerIdentityWithContext returned no account")
	}
	return *result.Account, nil
}

func (c client) GetAccountAlias(ctx context.Context) (string, error) {
	acctAliasOut, err := c.iamClient.ListAccountAliases(ctx, &iam.ListAccountAliasesInput{})
	if err != nil {
		return "", err
	}

	possibleAccountAlias := ""

	// Since a single account can only have one alias, and an authenticated SDK session corresponds to a single account,
	// the output can have at most one alias.
	// https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListAccountAliases.html
	if len(acctAliasOut.AccountAliases) > 0 {
		possibleAccountAlias = acctAliasOut.AccountAliases[0]
	}

	return possibleAccountAlias, nil
}
