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
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	"golang.org/x/sync/semaphore"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/cloudwatch"
	v1 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/v1"
	v2 "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients/v2"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/logging"
)

const (
	enableFeatureFlag = "enable-feature"
	htmlVersion       = `<html>
<head><title>Yet Another CloudWatch Exporter</title></head>
<body>
<h1>Thanks for using YACE :)</h1>
Version: %s
<p><a href="/metrics">Metrics</a></p>
%s
</body>
</html>`
	htmlPprof = `<p><a href="/debug/pprof">Pprof</a><p>`
)

var version = "custom-build"

var sem = semaphore.NewWeighted(1)

const (
	defaultLogFormat = "json"
)

var (
	addr                  string
	configFile            string
	debug                 bool
	logFormat             string
	fips                  bool
	cloudwatchConcurrency cloudwatch.ConcurrencyConfig
	tagConcurrency        int
	scrapingInterval      int
	metricsPerQuery       int
	labelsSnakeCase       bool
	profilingEnabled      bool

	logger logging.Logger
)

func main() {
	app := NewYACEApp()
	if err := app.Run(os.Args); err != nil {
		// if we exit very early we'll not have set up the logger yet
		if logger == nil {
			logger = logging.NewLogger(defaultLogFormat, debug, "version", version)
		}
		logger.Error(err, "Error running yace")
		os.Exit(1)
	}
}

// NewYACEApp creates a new cli.App implementing the YACE entrypoints and CLI arguments.
func NewYACEApp() *cli.App {
	yace := cli.NewApp()
	yace.Name = "Yet Another CloudWatch Exporter"
	yace.Version = version
	yace.Usage = "YACE configured to retrieve CloudWatch metrics through the AWS API"
	yace.Description = ""
	yace.Authors = []*cli.Author{
		{Name: "", Email: ""},
	}

	yace.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "listen-address",
			Value:       ":5000",
			Usage:       "The address to listen on",
			Destination: &addr,
			EnvVars:     []string{"listen-address"},
		},
		&cli.StringFlag{
			Name:        "config.file",
			Value:       "config.yml",
			Usage:       "Path to configuration file",
			Destination: &configFile,
			EnvVars:     []string{"config.file"},
		},
		&cli.BoolFlag{
			Name:        "debug",
			Value:       false,
			Usage:       "Verbose logging",
			Destination: &debug,
			EnvVars:     []string{"debug"},
		},
		&cli.StringFlag{
			Name:        "log.format",
			Value:       defaultLogFormat,
			Usage:       "Output format of log messages. One of: [logfmt, json]. Default: [json].",
			Destination: &logFormat,
			Action: func(_ *cli.Context, s string) error {
				switch s {
				case "logfmt", "json":
					break
				default:
					return fmt.Errorf("unrecognized log format %q", s)
				}
				return nil
			},
		},
		&cli.BoolFlag{
			Name:        "fips",
			Value:       false,
			Usage:       "Use FIPS compliant AWS API endpoints",
			Destination: &fips,
		},
		&cli.IntFlag{
			Name:        "cloudwatch-concurrency",
			Value:       exporter.DefaultCloudwatchConcurrency.SingleLimit,
			Usage:       "Maximum number of concurrent requests to CloudWatch API.",
			Destination: &cloudwatchConcurrency.SingleLimit,
		},
		&cli.BoolFlag{
			Name:        "cloudwatch-concurrency.per-api-limit-enabled",
			Value:       exporter.DefaultCloudwatchConcurrency.PerAPILimitEnabled,
			Usage:       "Whether to enable the per API CloudWatch concurrency limiter. When enabled, the concurrency `-cloudwatch-concurrency` flag will be ignored.",
			Destination: &cloudwatchConcurrency.PerAPILimitEnabled,
		},
		&cli.IntFlag{
			Name:        "cloudwatch-concurrency.list-metrics-limit",
			Value:       exporter.DefaultCloudwatchConcurrency.ListMetrics,
			Usage:       "Maximum number of concurrent requests to ListMetrics CloudWatch API. Used if the -cloudwatch-concurrency.per-api-limit-enabled concurrency limiter is enabled.",
			Destination: &cloudwatchConcurrency.ListMetrics,
		},
		&cli.IntFlag{
			Name:        "cloudwatch-concurrency.get-metric-data-limit",
			Value:       exporter.DefaultCloudwatchConcurrency.GetMetricData,
			Usage:       "Maximum number of concurrent requests to GetMetricData CloudWatch API. Used if the -cloudwatch-concurrency.per-api-limit-enabled concurrency limiter is enabled.",
			Destination: &cloudwatchConcurrency.GetMetricData,
		},
		&cli.IntFlag{
			Name:        "cloudwatch-concurrency.get-metric-statistics-limit",
			Value:       exporter.DefaultCloudwatchConcurrency.GetMetricStatistics,
			Usage:       "Maximum number of concurrent requests to GetMetricStatistics CloudWatch API. Used if the -cloudwatch-concurrency.per-api-limit-enabled concurrency limiter is enabled.",
			Destination: &cloudwatchConcurrency.GetMetricStatistics,
		},
		&cli.IntFlag{
			Name:        "tag-concurrency",
			Value:       exporter.DefaultTaggingAPIConcurrency,
			Usage:       "Maximum number of concurrent requests to Resource Tagging API.",
			Destination: &tagConcurrency,
		},
		&cli.IntFlag{
			Name:        "scraping-interval",
			Value:       300,
			Usage:       "Seconds to wait between scraping the AWS metrics",
			Destination: &scrapingInterval,
			EnvVars:     []string{"scraping-interval"},
		},
		&cli.IntFlag{
			Name:        "metrics-per-query",
			Value:       exporter.DefaultMetricsPerQuery,
			Usage:       "Number of metrics made in a single GetMetricsData request",
			Destination: &metricsPerQuery,
			EnvVars:     []string{"metrics-per-query"},
		},
		&cli.BoolFlag{
			Name:        "labels-snake-case",
			Value:       exporter.DefaultLabelsSnakeCase,
			Usage:       "Whether labels should be output in snake case instead of camel case",
			Destination: &labelsSnakeCase,
		},
		&cli.BoolFlag{
			Name:        "profiling.enabled",
			Value:       false,
			Usage:       "Enable pprof endpoints",
			Destination: &profilingEnabled,
		},
		&cli.StringSliceFlag{
			Name:  enableFeatureFlag,
			Usage: "Comma-separated list of enabled features",
		},
	}

	yace.Commands = []*cli.Command{
		{
			Name:    "verify-config",
			Aliases: []string{"vc"},
			Usage:   "Loads and attempts to parse config file, then exits. Useful for CI/CD validation",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "config.file", Value: "config.yml", Usage: "Path to configuration file.", Destination: &configFile},
			},
			Action: func(_ *cli.Context) error {
				logger = logging.NewLogger(logFormat, debug, "version", version)
				logger.Info("Parsing config")
				cfg := config.ScrapeConf{}
				if _, err := cfg.Load(configFile, logger); err != nil {
					logger.Error(err, "Couldn't read config file", "path", configFile)
					os.Exit(1)
				}
				logger.Info("Config file is valid", "path", configFile)
				os.Exit(0)
				return nil
			},
		},
		{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "prints current yace version.",
			Action: func(_ *cli.Context) error {
				fmt.Println(version)
				os.Exit(0)
				return nil
			},
		},
	}

	yace.Action = startScraper

	return yace
}

func startScraper(c *cli.Context) error {
	logger = logging.NewLogger(logFormat, debug, "version", version)

	// log warning if the two concurrency limiting methods are configured via CLI
	if c.IsSet("cloudwatch-concurrency") && c.IsSet("cloudwatch-concurrency.per-api-limit-enabled") {
		logger.Warn("Both `cloudwatch-concurrency` and `cloudwatch-concurrency.per-api-limit-enabled` are set. `cloudwatch-concurrency` will be ignored, and the per-api concurrency limiting strategy will be favoured.")
	}

	logger.Info("Parsing config")

	cfg := config.ScrapeConf{}
	jobsCfg, err := cfg.Load(configFile, logger)
	if err != nil {
		return fmt.Errorf("Couldn't read %s: %w", configFile, err)
	}

	featureFlags := c.StringSlice(enableFeatureFlag)
	s := NewScraper(featureFlags)
	var cache cachingFactory = v1.NewFactory(logger, jobsCfg, fips)
	for _, featureFlag := range featureFlags {
		if featureFlag == config.AwsSdkV2 {
			var err error
			// Can't override cache while also creating err
			cache, err = v2.NewFactory(logger, jobsCfg, fips)
			if err != nil {
				return fmt.Errorf("failed to construct aws sdk v2 client cache: %w", err)
			}
		}
	}

	ctx, cancelRunningScrape := context.WithCancel(context.Background())
	go s.decoupled(ctx, logger, jobsCfg, cache)

	mux := http.NewServeMux()

	if profilingEnabled {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	mux.HandleFunc("/metrics", s.makeHandler())

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		pprofLink := ""
		if profilingEnabled {
			pprofLink = htmlPprof
		}

		_, _ = w.Write([]byte(fmt.Sprintf(htmlVersion, version, pprofLink)))
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		logger.Info("Parsing config")
		newCfg := config.ScrapeConf{}
		newJobsCfg, err := newCfg.Load(configFile, logger)
		if err != nil {
			logger.Error(err, "Couldn't read config file", "path", configFile)
			return
		}

		logger.Info("Reset clients cache")
		cache = v1.NewFactory(logger, newJobsCfg, fips)
		for _, featureFlag := range featureFlags {
			if featureFlag == config.AwsSdkV2 {
				logger.Info("Using aws sdk v2")
				var err error
				// Can't override cache while also creating err
				cache, err = v2.NewFactory(logger, newJobsCfg, fips)
				if err != nil {
					logger.Error(err, "Failed to construct aws sdk v2 client cache", "path", configFile)
					return
				}
			}
		}

		cancelRunningScrape()
		ctx, cancelRunningScrape = context.WithCancel(context.Background())
		go s.decoupled(ctx, logger, newJobsCfg, cache)
	})

	logger.Info("Yace startup completed", "version", version, "feature_flags", strings.Join(featureFlags, ","))

	srv := &http.Server{Addr: addr, Handler: mux}
	return srv.ListenAndServe()
}
