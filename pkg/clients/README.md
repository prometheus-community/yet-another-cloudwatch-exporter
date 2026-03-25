# Purpose of the clients package
The goal of this package is to abstract away as much of the AWS SDK implementation details as possible. YACE uses
[AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/) exclusively (SDK v1 support was removed in v0.64.0).

The folder structure isolates common interfaces from their implementations:

```
/clients: Factory interface and CachingFactory implementation
/clients/account: account interface and implementation for looking up AWS account info
/clients/cloudwatch: cloudwatch interface and implementation for gathering metrics data
/clients/tagging: tagging interface and implementation for discovering resources, including service-specific filters
```

## /clients/tagging/filters.go serviceFilters

`serviceFilters` are extra definitions for how to lookup or filter resources for certain CloudWatch namespaces which
cannot be done using only tag data alone. Changes to service filters include:

* Adding a service filter implementation for a new service
* Modifying the behavior of a `ResourceFunc`
* Modifying the behavior of a `FilterFunc`
