# Feature flags

List of features or changes that are disabled by default since they are breaking changes or are considered experimental. Their behavior can change in future releases which will be communicated via the release changelog.

You can enable them using the `-enable-feature` flag with a comma separated list of features. They may be enabled by default in future versions.

## AWS SDK v1

`-enable-feature=aws-sdk-v1`

Uses the v1 version of the aws sdk for go for backward compatibility. By default, YACE now uses AWS SDK v2 which was released in Jan 2021 and comes with large performance gains.

## Always return info metrics

`-enable-feature=always-return-info-metrics`

Return info metrics even if there are no CloudWatch metrics for the resource. This is useful if you want to get a complete picture of your estate, for example if you have some resources which have not yet been used.
