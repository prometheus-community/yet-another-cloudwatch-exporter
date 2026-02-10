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
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/oam"
)

const usage = `Usage:
  oam-linked-accounts-check --sink-identifier SINK_ARN [--region REGION]

Lists all source account links attached to the given OAM sink and prints
each source account ID with its resolved label.

Notes:
  - Requires oam:ListAttachedLinks permission.
  - Caller must be in the monitoring account that owns the sink.
`

func main() {
	var sinkIdentifier string
	var region string
	flag.StringVar(&sinkIdentifier, "sink-identifier", "", "ARN of the OAM sink (required)")
	flag.StringVar(&region, "region", "", "AWS region for OAM API (optional)")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
	}
	flag.Parse()

	if sinkIdentifier == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()
	loadOptions := []func(*config.LoadOptions) error{}
	if region != "" {
		loadOptions = append(loadOptions, config.WithRegion(region))
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load AWS config: %v\n", err)
		os.Exit(1)
	}

	client := oam.NewFromConfig(awsConfig)

	paginator := oam.NewListAttachedLinksPaginator(client, &oam.ListAttachedLinksInput{
		SinkIdentifier: &sinkIdentifier,
	})

	count := 0
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ListAttachedLinks error: %v\n", err)
			os.Exit(1)
		}
		for _, item := range page.Items {
			linkArn := ""
			label := ""
			if item.LinkArn != nil {
				linkArn = *item.LinkArn
			}
			if item.Label != nil {
				label = *item.Label
			}
			fmt.Printf("link_arn=%s label=%s\n", linkArn, label)
			count++
		}
	}
	fmt.Printf("\nTotal linked accounts: %d\n", count)
}
