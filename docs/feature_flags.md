# Feature flags

List of features or changes that are disabled by default since they are breaking changes or are considered experimental. Their behavior can change in future releases which will be communicated via the release changelog.

You can enable them using the `-enable-feature` flag with a comma separated list of features. They may be enabled by default in future versions.

## Always return info metrics

`-enable-feature=always-return-info-metrics`

Return info metrics even if there are no CloudWatch metrics for the resource. This is useful if you want to get a complete picture of your estate, for example if you have some resources which have not yet been used.

## Omit empty labels

`-enable-feature=omit-empty-labels`

Drop empty-valued labels instead of adding every label seen on any series of a metric to all of its series with an empty value. Prometheus treats an empty label value the same as a missing one, so queries return the same results.

Without this flag, when a label key exists on only some resources (for example a tag key on the `aws_<namespace>_info` metrics), every series of that metric is re-exposed with or without `key=""` whenever the last resource carrying the key disappears or the first one appears. The scrape loop then sends stale markers that collide with the new samples, which a remote write receiver can reject as duplicate samples. With this flag, the other series are exposed unchanged.
