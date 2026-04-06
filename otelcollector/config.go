// Copyright 2026 The Prometheus Authors
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

package otelcollector

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"
	yaceconfig "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	prombridge "github.com/prometheus/opentelemetry-collector-bridge"
)

const defaultAWSScrapeInterval = "60s"

// Config maps OTel receiver configuration into YACE's runtime configuration.
type Config struct {
	StsRegion             string                `mapstructure:"sts_region"`
	FIPSEnabled           bool                  `mapstructure:"fips_enabled"`
	MetricsPerQuery       int                   `mapstructure:"metrics_per_query"`
	LabelsSnakeCase       bool                  `mapstructure:"labels_snake_case"`
	CloudwatchConcurrency CloudwatchConcurrency `mapstructure:"cloudwatch_concurrency"`
	TaggingAPIConcurrency int                   `mapstructure:"tagging_api_concurrency"`
	FeatureFlags          []string              `mapstructure:"feature_flags"`
	AWSScrapeInterval     string                `mapstructure:"aws_scrape_interval"`
	Discovery             Discovery             `mapstructure:"discovery"`
	Static                []*Static             `mapstructure:"static"`
	CustomNamespace       []*CustomNamespace    `mapstructure:"custom_namespace"`
}

type CloudwatchConcurrency struct {
	SingleLimit         int  `mapstructure:"single_limit"`
	PerAPILimitEnabled  bool `mapstructure:"per_api_limit_enabled"`
	ListMetrics         int  `mapstructure:"list_metrics"`
	GetMetricData       int  `mapstructure:"get_metric_data"`
	GetMetricStatistics int  `mapstructure:"get_metric_statistics"`
}

type Discovery struct {
	ExportedTagsOnMetrics map[string][]string `mapstructure:"exported_tags_on_metrics"`
	Jobs                  []*Job              `mapstructure:"jobs"`
}

type JobMetricFields struct {
	Statistics             []string `mapstructure:"statistics"`
	Period                 int64    `mapstructure:"period"`
	Length                 int64    `mapstructure:"length"`
	Delay                  int64    `mapstructure:"delay"`
	NilToZero              *bool    `mapstructure:"nil_to_zero"`
	AddCloudwatchTimestamp *bool    `mapstructure:"add_cloudwatch_timestamp"`
	ExportAllDataPoints    *bool    `mapstructure:"export_all_data_points"`
}

type Job struct {
	Regions                     []string          `mapstructure:"regions"`
	Type                        string            `mapstructure:"type"`
	Roles                       []Role            `mapstructure:"roles"`
	SearchTags                  []Tag             `mapstructure:"search_tags"`
	CustomTags                  []Tag             `mapstructure:"custom_tags"`
	DimensionNameRequirements   []string          `mapstructure:"dimension_name_requirements"`
	Metrics                     []*Metric         `mapstructure:"metrics"`
	RoundingPeriod              *int64            `mapstructure:"rounding_period"`
	RecentlyActiveOnly          bool              `mapstructure:"recently_active_only"`
	IncludeContextOnInfoMetrics bool              `mapstructure:"include_context_on_info_metrics"`
	EnhancedMetrics             []*EnhancedMetric `mapstructure:"enhanced_metrics"`
	JobMetricFields             `mapstructure:",squash"`
}

type Static struct {
	Name       string      `mapstructure:"name"`
	Regions    []string    `mapstructure:"regions"`
	Roles      []Role      `mapstructure:"roles"`
	Namespace  string      `mapstructure:"namespace"`
	CustomTags []Tag       `mapstructure:"custom_tags"`
	Dimensions []Dimension `mapstructure:"dimensions"`
	Metrics    []*Metric   `mapstructure:"metrics"`
}

type CustomNamespace struct {
	Regions                   []string  `mapstructure:"regions"`
	Name                      string    `mapstructure:"name"`
	Namespace                 string    `mapstructure:"namespace"`
	RecentlyActiveOnly        bool      `mapstructure:"recently_active_only"`
	Roles                     []Role    `mapstructure:"roles"`
	Metrics                   []*Metric `mapstructure:"metrics"`
	CustomTags                []Tag     `mapstructure:"custom_tags"`
	DimensionNameRequirements []string  `mapstructure:"dimension_name_requirements"`
	RoundingPeriod            *int64    `mapstructure:"rounding_period"`
	JobMetricFields           `mapstructure:",squash"`
}

type Metric struct {
	Name                   string   `mapstructure:"name"`
	Statistics             []string `mapstructure:"statistics"`
	Period                 int64    `mapstructure:"period"`
	Length                 int64    `mapstructure:"length"`
	Delay                  int64    `mapstructure:"delay"`
	NilToZero              *bool    `mapstructure:"nil_to_zero"`
	AddCloudwatchTimestamp *bool    `mapstructure:"add_cloudwatch_timestamp"`
	ExportAllDataPoints    *bool    `mapstructure:"export_all_data_points"`
}

type Tag struct {
	Key   string `mapstructure:"key"`
	Value string `mapstructure:"value"`
}

type Role struct {
	RoleArn    string `mapstructure:"role_arn"`
	ExternalID string `mapstructure:"external_id"`
}

type Dimension struct {
	Name  string `mapstructure:"name"`
	Value string `mapstructure:"value"`
}

type EnhancedMetric struct {
	Name string `mapstructure:"name"`
}

var _ prombridge.Config = (*Config)(nil)

func defaultConfig() *Config {
	return &Config{
		MetricsPerQuery:       exporter.DefaultMetricsPerQuery,
		LabelsSnakeCase:       exporter.DefaultLabelsSnakeCase,
		CloudwatchConcurrency: defaultCloudwatchConcurrency(),
		TaggingAPIConcurrency: exporter.DefaultTaggingAPIConcurrency,
		FeatureFlags:          []string{},
		AWSScrapeInterval:     defaultAWSScrapeInterval,
	}
}

func defaultCloudwatchConcurrency() CloudwatchConcurrency {
	return CloudwatchConcurrency{
		SingleLimit:         exporter.DefaultCloudwatchConcurrency.SingleLimit,
		PerAPILimitEnabled:  exporter.DefaultCloudwatchConcurrency.PerAPILimitEnabled,
		ListMetrics:         exporter.DefaultCloudwatchConcurrency.ListMetrics,
		GetMetricData:       exporter.DefaultCloudwatchConcurrency.GetMetricData,
		GetMetricStatistics: exporter.DefaultCloudwatchConcurrency.GetMetricStatistics,
	}
}

func defaultComponentDefaults() map[string]interface{} {
	cfg := defaultConfig()
	return map[string]interface{}{
		"fips_enabled":            cfg.FIPSEnabled,
		"metrics_per_query":       cfg.MetricsPerQuery,
		"labels_snake_case":       cfg.LabelsSnakeCase,
		"tagging_api_concurrency": cfg.TaggingAPIConcurrency,
		"feature_flags":           cfg.FeatureFlags,
		"aws_scrape_interval":     cfg.AWSScrapeInterval,
		"cloudwatch_concurrency": map[string]interface{}{
			"single_limit":          cfg.CloudwatchConcurrency.SingleLimit,
			"per_api_limit_enabled": cfg.CloudwatchConcurrency.PerAPILimitEnabled,
			"list_metrics":          cfg.CloudwatchConcurrency.ListMetrics,
			"get_metric_data":       cfg.CloudwatchConcurrency.GetMetricData,
			"get_metric_statistics": cfg.CloudwatchConcurrency.GetMetricStatistics,
		},
	}
}

func (c *Config) Validate() error {
	if c.MetricsPerQuery <= 0 {
		return fmt.Errorf("metrics_per_query must be greater than 0")
	}
	if c.TaggingAPIConcurrency <= 0 {
		return fmt.Errorf("tagging_api_concurrency must be greater than 0")
	}
	if c.CloudwatchConcurrency.SingleLimit <= 0 {
		return fmt.Errorf("cloudwatch_concurrency.single_limit must be greater than 0")
	}
	if c.CloudwatchConcurrency.ListMetrics <= 0 {
		return fmt.Errorf("cloudwatch_concurrency.list_metrics must be greater than 0")
	}
	if c.CloudwatchConcurrency.GetMetricData <= 0 {
		return fmt.Errorf("cloudwatch_concurrency.get_metric_data must be greater than 0")
	}
	if c.CloudwatchConcurrency.GetMetricStatistics <= 0 {
		return fmt.Errorf("cloudwatch_concurrency.get_metric_statistics must be greater than 0")
	}
	if _, err := c.awsScrapeInterval(); err != nil {
		return err
	}

	scrapeConf := c.toScrapeConf()
	normalizeScrapeConf(&scrapeConf)
	_, err := scrapeConf.Validate(discardLogger())
	return err
}

func (c *Config) jobsConfig(logger *slog.Logger) (model.JobsConfig, error) {
	scrapeConf := c.toScrapeConf()
	normalizeScrapeConf(&scrapeConf)
	if logger == nil {
		logger = discardLogger()
	}
	return scrapeConf.Validate(logger)
}

func (c *Config) exporterOptions() ([]exporter.OptionsFunc, error) {
	options := []exporter.OptionsFunc{
		exporter.MetricsPerQuery(c.MetricsPerQuery),
		exporter.LabelsSnakeCase(c.LabelsSnakeCase),
		exporter.TaggingAPIConcurrency(c.TaggingAPIConcurrency),
		exporter.EnableFeatureFlag(c.FeatureFlags...),
	}

	if c.CloudwatchConcurrency.PerAPILimitEnabled {
		options = append(options,
			exporter.CloudWatchPerAPILimitConcurrency(
				c.CloudwatchConcurrency.ListMetrics,
				c.CloudwatchConcurrency.GetMetricData,
				c.CloudwatchConcurrency.GetMetricStatistics,
			),
		)
	} else {
		options = append(options, exporter.CloudWatchAPIConcurrency(c.CloudwatchConcurrency.SingleLimit))
	}

	return options, nil
}

func (c *Config) awsScrapeInterval() (time.Duration, error) {
	raw := c.AWSScrapeInterval
	if raw == "" {
		raw = defaultAWSScrapeInterval
	}

	interval, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("aws_scrape_interval: invalid duration %q: %w", raw, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("aws_scrape_interval must be greater than 0")
	}
	return interval, nil
}

func (c *Config) toScrapeConf() yaceconfig.ScrapeConf {
	scrapeConf := yaceconfig.ScrapeConf{
		APIVersion: "v1alpha1",
		StsRegion:  c.StsRegion,
		Discovery: yaceconfig.Discovery{
			ExportedTagsOnMetrics: c.Discovery.ExportedTagsOnMetrics,
			Jobs:                  make([]*yaceconfig.Job, 0, len(c.Discovery.Jobs)),
		},
		Static:          make([]*yaceconfig.Static, 0, len(c.Static)),
		CustomNamespace: make([]*yaceconfig.CustomNamespace, 0, len(c.CustomNamespace)),
	}

	for _, job := range c.Discovery.Jobs {
		scrapeConf.Discovery.Jobs = append(scrapeConf.Discovery.Jobs, job.toYACE())
	}
	for _, job := range c.Static {
		scrapeConf.Static = append(scrapeConf.Static, job.toYACE())
	}
	for _, job := range c.CustomNamespace {
		scrapeConf.CustomNamespace = append(scrapeConf.CustomNamespace, job.toYACE())
	}

	return scrapeConf
}

func normalizeScrapeConf(scrapeConf *yaceconfig.ScrapeConf) {
	for _, job := range scrapeConf.Discovery.Jobs {
		if len(job.Roles) == 0 {
			job.Roles = []yaceconfig.Role{{}}
		}
	}
	for _, job := range scrapeConf.Static {
		if len(job.Roles) == 0 {
			job.Roles = []yaceconfig.Role{{}}
		}
	}
	for _, job := range scrapeConf.CustomNamespace {
		if len(job.Roles) == 0 {
			job.Roles = []yaceconfig.Role{{}}
		}
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (j *Job) toYACE() *yaceconfig.Job {
	if j == nil {
		return nil
	}

	return &yaceconfig.Job{
		Regions:                     append([]string(nil), j.Regions...),
		Type:                        j.Type,
		Roles:                       toYACERoles(j.Roles),
		SearchTags:                  toYACETags(j.SearchTags),
		CustomTags:                  toYACETags(j.CustomTags),
		DimensionNameRequirements:   append([]string(nil), j.DimensionNameRequirements...),
		Metrics:                     toYACEMetrics(j.Metrics),
		RoundingPeriod:              j.RoundingPeriod,
		RecentlyActiveOnly:          j.RecentlyActiveOnly,
		IncludeContextOnInfoMetrics: j.IncludeContextOnInfoMetrics,
		EnhancedMetrics:             toYACEEnhancedMetrics(j.EnhancedMetrics),
		JobLevelMetricFields: yaceconfig.JobLevelMetricFields{
			Statistics:             append([]string(nil), j.Statistics...),
			Period:                 j.Period,
			Length:                 j.Length,
			Delay:                  j.Delay,
			NilToZero:              j.NilToZero,
			AddCloudwatchTimestamp: j.AddCloudwatchTimestamp,
			ExportAllDataPoints:    j.ExportAllDataPoints,
		},
	}
}

func (s *Static) toYACE() *yaceconfig.Static {
	if s == nil {
		return nil
	}

	return &yaceconfig.Static{
		Name:       s.Name,
		Regions:    append([]string(nil), s.Regions...),
		Roles:      toYACERoles(s.Roles),
		Namespace:  s.Namespace,
		CustomTags: toYACETags(s.CustomTags),
		Dimensions: toYACEDimensions(s.Dimensions),
		Metrics:    toYACEMetrics(s.Metrics),
	}
}

func (c *CustomNamespace) toYACE() *yaceconfig.CustomNamespace {
	if c == nil {
		return nil
	}

	return &yaceconfig.CustomNamespace{
		Regions:                   append([]string(nil), c.Regions...),
		Name:                      c.Name,
		Namespace:                 c.Namespace,
		RecentlyActiveOnly:        c.RecentlyActiveOnly,
		Roles:                     toYACERoles(c.Roles),
		Metrics:                   toYACEMetrics(c.Metrics),
		CustomTags:                toYACETags(c.CustomTags),
		DimensionNameRequirements: append([]string(nil), c.DimensionNameRequirements...),
		RoundingPeriod:            c.RoundingPeriod,
		JobLevelMetricFields: yaceconfig.JobLevelMetricFields{
			Statistics:             append([]string(nil), c.Statistics...),
			Period:                 c.Period,
			Length:                 c.Length,
			Delay:                  c.Delay,
			NilToZero:              c.NilToZero,
			AddCloudwatchTimestamp: c.AddCloudwatchTimestamp,
			ExportAllDataPoints:    c.ExportAllDataPoints,
		},
	}
}

func toYACEMetrics(metrics []*Metric) []*yaceconfig.Metric {
	converted := make([]*yaceconfig.Metric, 0, len(metrics))
	for _, metric := range metrics {
		if metric == nil {
			continue
		}
		converted = append(converted, &yaceconfig.Metric{
			Name:                   metric.Name,
			Statistics:             append([]string(nil), metric.Statistics...),
			Period:                 metric.Period,
			Length:                 metric.Length,
			Delay:                  metric.Delay,
			NilToZero:              metric.NilToZero,
			AddCloudwatchTimestamp: metric.AddCloudwatchTimestamp,
			ExportAllDataPoints:    metric.ExportAllDataPoints,
		})
	}
	return converted
}

func toYACETags(tags []Tag) []yaceconfig.Tag {
	converted := make([]yaceconfig.Tag, 0, len(tags))
	for _, tag := range tags {
		converted = append(converted, yaceconfig.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	return converted
}

func toYACERoles(roles []Role) []yaceconfig.Role {
	converted := make([]yaceconfig.Role, 0, len(roles))
	for _, role := range roles {
		converted = append(converted, yaceconfig.Role{
			RoleArn:    role.RoleArn,
			ExternalID: role.ExternalID,
		})
	}
	return converted
}

func toYACEDimensions(dimensions []Dimension) []yaceconfig.Dimension {
	converted := make([]yaceconfig.Dimension, 0, len(dimensions))
	for _, dimension := range dimensions {
		converted = append(converted, yaceconfig.Dimension{
			Name:  dimension.Name,
			Value: dimension.Value,
		})
	}
	return converted
}

func toYACEEnhancedMetrics(metrics []*EnhancedMetric) []*yaceconfig.EnhancedMetric {
	converted := make([]*yaceconfig.EnhancedMetric, 0, len(metrics))
	for _, metric := range metrics {
		if metric == nil {
			continue
		}
		converted = append(converted, &yaceconfig.EnhancedMetric{Name: metric.Name})
	}
	return converted
}

type configUnmarshaler struct{}

func (configUnmarshaler) GetConfigStruct() prombridge.Config {
	return defaultConfig()
}
