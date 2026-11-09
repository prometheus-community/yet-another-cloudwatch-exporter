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

// Package cwexporter converts a prometheus/cloudwatch_exporter configuration
// into a YACE configuration.
package cwexporter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v2"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
)

// cloudwatch_exporter defaults, see CloudWatchCollector.loadConfig.
const (
	defaultPeriodSeconds = 60
	defaultRangeSeconds  = 600
	defaultDelaySeconds  = 600
	defaultSetTimestamp  = true

	// maxStaticCombinations caps how many static jobs a single aws_dimension_select
	// may expand into before falling back to a custom namespace job.
	maxStaticCombinations = 100
)

var defaultStatistics = []string{"Sum", "SampleCount", "Minimum", "Maximum", "Average"}

type sourceConfig struct {
	Region                    string       `yaml:"region"`
	RoleArn                   string       `yaml:"role_arn"`
	PeriodSeconds             *int64       `yaml:"period_seconds"`
	RangeSeconds              *int64       `yaml:"range_seconds"`
	DelaySeconds              *int64       `yaml:"delay_seconds"`
	SetTimestamp              *bool        `yaml:"set_timestamp"`
	UseGetMetricData          *bool        `yaml:"use_get_metric_data"`
	ListMetricsCacheTTL       *int64       `yaml:"list_metrics_cache_ttl"`
	WarnOnEmptyListDimensions *bool        `yaml:"warn_on_empty_list_dimensions"`
	Metrics                   []sourceRule `yaml:"metrics"`
}

type sourceRule struct {
	Namespace                 string              `yaml:"aws_namespace"`
	MetricName                string              `yaml:"aws_metric_name"`
	Help                      string              `yaml:"help"`
	Dimensions                []string            `yaml:"aws_dimensions"`
	DimensionSelect           map[string][]string `yaml:"aws_dimension_select"`
	DimensionSelectRegex      map[string][]string `yaml:"aws_dimension_select_regex"`
	Statistics                []string            `yaml:"aws_statistics"`
	ExtendedStatistics        []string            `yaml:"aws_extended_statistics"`
	TagSelect                 *sourceTagSelect    `yaml:"aws_tag_select"`
	PeriodSeconds             *int64              `yaml:"period_seconds"`
	RangeSeconds              *int64              `yaml:"range_seconds"`
	DelaySeconds              *int64              `yaml:"delay_seconds"`
	SetTimestamp              *bool               `yaml:"set_timestamp"`
	UseGetMetricData          *bool               `yaml:"use_get_metric_data"`
	ListMetricsCacheTTL       *int64              `yaml:"list_metrics_cache_ttl"`
	WarnOnEmptyListDimensions *bool               `yaml:"warn_on_empty_list_dimensions"`
}

type sourceTagSelect struct {
	TagSelections         map[string][]string `yaml:"tag_selections"`
	ResourceTypeSelection string              `yaml:"resource_type_selection"`
	ResourceIDDimension   string              `yaml:"resource_id_dimension"`
	ArnResourceIDRegexp   string              `yaml:"arn_resource_id_regexp"`
}

// Output is the generated YACE configuration. It mirrors the YAML layout of
// config.ScrapeConf, but leaves out empty fields so the result stays readable.
type Output struct {
	APIVersion      string             `yaml:"apiVersion"`
	Discovery       *Discovery         `yaml:"discovery,omitempty"`
	Static          []*StaticJob       `yaml:"static,omitempty"`
	CustomNamespace []*CustomNamespace `yaml:"customNamespace,omitempty"`
}

type Discovery struct {
	Jobs []*DiscoveryJob `yaml:"jobs"`
}

type Role struct {
	RoleArn string `yaml:"roleArn"`
}

type Tag struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type Dimension struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type DiscoveryJob struct {
	Type                      string    `yaml:"type"`
	Regions                   []string  `yaml:"regions"`
	Roles                     []Role    `yaml:"roles,omitempty"`
	SearchTags                []Tag     `yaml:"searchTags,omitempty"`
	DimensionNameRequirements []string  `yaml:"dimensionNameRequirements,omitempty"`
	Period                    int64     `yaml:"period"`
	Length                    int64     `yaml:"length"`
	Delay                     int64     `yaml:"delay,omitempty"`
	AddCloudwatchTimestamp    bool      `yaml:"addCloudwatchTimestamp"`
	Metrics                   []*Metric `yaml:"metrics"`
}

type CustomNamespace struct {
	Name                      string    `yaml:"name"`
	Namespace                 string    `yaml:"namespace"`
	Regions                   []string  `yaml:"regions"`
	Roles                     []Role    `yaml:"roles,omitempty"`
	DimensionNameRequirements []string  `yaml:"dimensionNameRequirements,omitempty"`
	Period                    int64     `yaml:"period"`
	Length                    int64     `yaml:"length"`
	Delay                     int64     `yaml:"delay,omitempty"`
	AddCloudwatchTimestamp    bool      `yaml:"addCloudwatchTimestamp"`
	Metrics                   []*Metric `yaml:"metrics"`
}

type StaticJob struct {
	Name       string      `yaml:"name"`
	Namespace  string      `yaml:"namespace"`
	Regions    []string    `yaml:"regions"`
	Roles      []Role      `yaml:"roles,omitempty"`
	Dimensions []Dimension `yaml:"dimensions"`
	Metrics    []*Metric   `yaml:"metrics"`
}

type Metric struct {
	Name                   string   `yaml:"name"`
	Statistics             []string `yaml:"statistics"`
	Period                 int64    `yaml:"period,omitempty"`
	Length                 int64    `yaml:"length,omitempty"`
	AddCloudwatchTimestamp *bool    `yaml:"addCloudwatchTimestamp,omitempty"`
}

// Options changes how Convert fills in values the source config doesn't have.
type Options struct {
	// Region is used when the source config has no region set.
	Region string
}

// Result is a converted config together with the parts of the source config
// that YACE can't express exactly.
type Result struct {
	Config   *Output
	Warnings []string
}

// YAML renders the converted config.
func (r *Result) YAML() ([]byte, error) {
	return yaml.Marshal(r.Config)
}

type timing struct {
	period, length, delay int64
	timestamp             bool
}

type converter struct {
	src      sourceConfig
	regions  []string
	roles    []Role
	out      *Output
	jobs     map[string]any
	names    map[string]int
	warnings []string
	warned   map[string]bool
}

// Convert parses a cloudwatch_exporter config and returns the equivalent YACE config.
func Convert(data []byte, opts Options) (*Result, error) {
	c := &converter{
		out:    &Output{APIVersion: "v1alpha1"},
		jobs:   map[string]any{},
		names:  map[string]int{},
		warned: map[string]bool{},
	}
	if err := yaml.Unmarshal(data, &c.src); err != nil {
		return nil, fmt.Errorf("parsing cloudwatch_exporter config: %w", err)
	}
	if len(c.src.Metrics) == 0 {
		return nil, fmt.Errorf("cloudwatch_exporter config has no metrics")
	}

	region := c.src.Region
	if region == "" {
		region = opts.Region
	}
	if region == "" {
		return nil, fmt.Errorf("cloudwatch_exporter config has no region; set one with the region option")
	}
	c.regions = []string{region}
	if c.src.RoleArn != "" {
		c.roles = []Role{{RoleArn: c.src.RoleArn}}
	}

	c.warnIgnoredGlobal()
	for i := range c.src.Metrics {
		if err := c.convertRule(i, &c.src.Metrics[i]); err != nil {
			return nil, err
		}
	}

	return &Result{Config: c.out, Warnings: c.warnings}, nil
}

func (c *converter) warnf(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

// warnOnce reports an option that has no YACE equivalent only the first time it shows up.
func (c *converter) warnOnce(key, format string, args ...any) {
	if c.warned[key] {
		return
	}
	c.warned[key] = true
	c.warnf(format, args...)
}

func (c *converter) warnIgnoredGlobal() {
	if c.src.UseGetMetricData != nil {
		c.warnUseGetMetricData()
	}
	if c.src.ListMetricsCacheTTL != nil && *c.src.ListMetricsCacheTTL > 0 {
		c.warnListMetricsCache()
	}
	if c.src.WarnOnEmptyListDimensions != nil {
		c.warnEmptyListDimensions()
	}
}

func (c *converter) warnUseGetMetricData() {
	c.warnOnce("use_get_metric_data", "use_get_metric_data is ignored: YACE uses GetMetricData for discovery and custom namespace jobs and GetMetricStatistics for static jobs")
}

func (c *converter) warnListMetricsCache() {
	c.warnOnce("list_metrics_cache_ttl", "list_metrics_cache_ttl is ignored: YACE has no ListMetrics cache setting")
}

func (c *converter) warnEmptyListDimensions() {
	c.warnOnce("warn_on_empty_list_dimensions", "warn_on_empty_list_dimensions is ignored")
}

func (c *converter) timingFor(r *sourceRule) timing {
	t := timing{
		period:    pick(r.PeriodSeconds, c.src.PeriodSeconds, defaultPeriodSeconds),
		length:    pick(r.RangeSeconds, c.src.RangeSeconds, defaultRangeSeconds),
		delay:     pick(r.DelaySeconds, c.src.DelaySeconds, defaultDelaySeconds),
		timestamp: defaultSetTimestamp,
	}
	if c.src.SetTimestamp != nil {
		t.timestamp = *c.src.SetTimestamp
	}
	if r.SetTimestamp != nil {
		t.timestamp = *r.SetTimestamp
	}
	return t
}

func pick(rule, global *int64, def int64) int64 {
	if rule != nil {
		return *rule
	}
	if global != nil {
		return *global
	}
	return def
}

func (c *converter) convertRule(idx int, r *sourceRule) error {
	if r.Namespace == "" || r.MetricName == "" {
		return fmt.Errorf("metrics[%d]: aws_namespace and aws_metric_name are required", idx)
	}
	if r.DimensionSelect != nil && r.DimensionSelectRegex != nil {
		return fmt.Errorf("metrics[%d]: aws_dimension_select and aws_dimension_select_regex can't be used together", idx)
	}
	ref := fmt.Sprintf("metrics[%d] (%s %s)", idx, r.Namespace, r.MetricName)

	if r.Help != "" {
		c.warnOnce("help", "help is ignored: YACE generates the metric help text")
	}
	if r.UseGetMetricData != nil {
		c.warnUseGetMetricData()
	}
	if r.ListMetricsCacheTTL != nil && *r.ListMetricsCacheTTL > 0 {
		c.warnListMetricsCache()
	}
	if r.WarnOnEmptyListDimensions != nil {
		c.warnEmptyListDimensions()
	}

	stats := statistics(r)
	t := c.timingFor(r)

	if r.TagSelect != nil {
		if config.SupportedServices.GetService(r.Namespace) != nil {
			if len(r.DimensionSelect) > 0 || len(r.DimensionSelectRegex) > 0 {
				c.warnf("%s: aws_dimension_select and aws_dimension_select_regex are dropped, the discovery job selects resources by tag only", ref)
			}
			if r.TagSelect.ArnResourceIDRegexp != "" {
				c.warnf("%s: arn_resource_id_regexp is dropped, YACE maps resources to dimensions itself", ref)
			}
			c.addDiscovery(r, stats, t)
			return nil
		}
		c.warnf("%s: YACE can't discover %s resources by tag, so aws_tag_select is dropped and every resource is exported", ref, r.Namespace)
		c.addCustomNamespace(r, stats, t)
		return nil
	}

	if len(r.Dimensions) == 0 {
		c.addStatic(ref, r, nil, stats, t)
		return nil
	}

	if len(r.DimensionSelectRegex) > 0 {
		c.warnf("%s: YACE has no dimension value filter, so every value of %s is exported; filter on the dimension labels with metric_relabel_configs instead", ref, strings.Join(sortedKeys(r.DimensionSelectRegex), ", "))
		c.addCustomNamespace(r, stats, t)
		return nil
	}

	if len(r.DimensionSelect) > 0 {
		combos, total, ok := combinations(r.Dimensions, r.DimensionSelect)
		switch {
		case !ok:
			c.warnf("%s: aws_dimension_select doesn't give values for every dimension, and YACE has no dimension value filter, so every value of %s is exported; filter on the dimension labels with metric_relabel_configs instead", ref, strings.Join(sortedKeys(r.DimensionSelect), ", "))
		case total > maxStaticCombinations:
			c.warnf("%s: aws_dimension_select expands to more than %d dimension combinations, so it is converted to a custom namespace job that exports every value; filter on the dimension labels with metric_relabel_configs instead", ref, maxStaticCombinations)
		default:
			for _, dims := range combos {
				c.addStatic(ref, r, dims, stats, t)
			}
			return nil
		}
	}

	c.addCustomNamespace(r, stats, t)
	return nil
}

func statistics(r *sourceRule) []string {
	var stats []string
	switch {
	case len(r.Statistics) > 0:
		stats = append(stats, r.Statistics...)
	case len(r.ExtendedStatistics) == 0:
		stats = append(stats, defaultStatistics...)
	}
	return append(stats, r.ExtendedStatistics...)
}

// combinations returns every dimension combination selected by
// aws_dimension_select and how many there are. It returns false when a
// dimension has no selected values, and no combinations when there are more
// than maxStaticCombinations.
func combinations(names []string, sel map[string][]string) ([][]Dimension, int, bool) {
	total := 1
	for _, name := range names {
		if len(sel[name]) == 0 {
			return nil, 0, false
		}
		total *= len(sel[name])
		if total > maxStaticCombinations {
			return nil, total, true
		}
	}

	combos := [][]Dimension{{}}
	for _, name := range names {
		next := make([][]Dimension, 0, len(combos)*len(sel[name]))
		for _, combo := range combos {
			for _, v := range sel[name] {
				next = append(next, append(append([]Dimension{}, combo...), Dimension{Name: name, Value: v}))
			}
		}
		combos = next
	}
	return combos, total, true
}

func (c *converter) addDiscovery(r *sourceRule, stats []string, t timing) {
	tags := searchTags(r.TagSelect.TagSelections)
	key := fmt.Sprintf("discovery|%s|%v|%v|%v", r.Namespace, tags, r.Dimensions, t)
	job, ok := c.jobs[key].(*DiscoveryJob)
	if !ok {
		job = &DiscoveryJob{
			Type:                      r.Namespace,
			Regions:                   c.regions,
			Roles:                     c.roles,
			SearchTags:                tags,
			DimensionNameRequirements: r.Dimensions,
			Period:                    t.period,
			Length:                    t.length,
			Delay:                     t.delay,
			AddCloudwatchTimestamp:    t.timestamp,
		}
		c.jobs[key] = job
		if c.out.Discovery == nil {
			c.out.Discovery = &Discovery{}
		}
		c.out.Discovery.Jobs = append(c.out.Discovery.Jobs, job)
	}
	job.Metrics = append(job.Metrics, &Metric{Name: r.MetricName, Statistics: stats})
}

func (c *converter) addCustomNamespace(r *sourceRule, stats []string, t timing) {
	key := fmt.Sprintf("custom|%s|%v|%v", r.Namespace, r.Dimensions, t)
	job, ok := c.jobs[key].(*CustomNamespace)
	if !ok {
		job = &CustomNamespace{
			Name:                      c.uniqueName(r.Namespace),
			Namespace:                 r.Namespace,
			Regions:                   c.regions,
			Roles:                     c.roles,
			DimensionNameRequirements: r.Dimensions,
			Period:                    t.period,
			Length:                    t.length,
			Delay:                     t.delay,
			AddCloudwatchTimestamp:    t.timestamp,
		}
		c.jobs[key] = job
		c.out.CustomNamespace = append(c.out.CustomNamespace, job)
	}
	job.Metrics = append(job.Metrics, &Metric{Name: r.MetricName, Statistics: stats})
}

func (c *converter) addStatic(ref string, r *sourceRule, dims []Dimension, stats []string, t timing) {
	if t.delay > 0 {
		c.warnOnce("static-delay", "static jobs (first one from %s): YACE static jobs don't support a delay, so they request the most recent data instead of data that is delay_seconds old", ref)
	}
	if dims == nil {
		dims = []Dimension{}
	}
	key := fmt.Sprintf("static|%s|%v", r.Namespace, dims)
	job, ok := c.jobs[key].(*StaticJob)
	if !ok {
		name := r.Namespace
		if len(dims) > 0 {
			parts := make([]string, len(dims))
			for i, d := range dims {
				parts[i] = d.Name + "=" + d.Value
			}
			name += " " + strings.Join(parts, ",")
		}
		job = &StaticJob{
			Name:       c.uniqueName(name),
			Namespace:  r.Namespace,
			Regions:    c.regions,
			Roles:      c.roles,
			Dimensions: dims,
		}
		c.jobs[key] = job
		c.out.Static = append(c.out.Static, job)
	}
	ts := t.timestamp
	job.Metrics = append(job.Metrics, &Metric{
		Name:                   r.MetricName,
		Statistics:             stats,
		Period:                 t.period,
		Length:                 t.length,
		AddCloudwatchTimestamp: &ts,
	})
}

func (c *converter) uniqueName(name string) string {
	c.names[name]++
	if n := c.names[name]; n > 1 {
		return fmt.Sprintf("%s (%d)", name, n)
	}
	return name
}

// searchTags turns tag_selections into YACE search tags. cloudwatch_exporter
// matches any of the listed values of a key, so they become one anchored regex.
func searchTags(sel map[string][]string) []Tag {
	tags := make([]Tag, 0, len(sel))
	for _, key := range sortedKeys(sel) {
		values := sel[key]
		value := ".*"
		if len(values) > 0 {
			quoted := make([]string, len(values))
			for i, v := range values {
				quoted[i] = regexp.QuoteMeta(v)
			}
			value = "^(?:" + strings.Join(quoted, "|") + ")$"
		}
		tags = append(tags, Tag{Key: key, Value: value})
	}
	return tags
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
