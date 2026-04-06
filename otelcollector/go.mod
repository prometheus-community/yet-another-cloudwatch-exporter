module github.com/prometheus-community/yet-another-cloudwatch-exporter/otelcollector

go 1.25.0

require (
	github.com/prometheus-community/yet-another-cloudwatch-exporter v0.0.0
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/opentelemetry-collector-bridge v0.0.0-20260317204527-5fc426455618
	go.opentelemetry.io/collector/component v1.55.0
	go.opentelemetry.io/collector/consumer/consumertest v0.149.0
	go.opentelemetry.io/collector/receiver v1.55.0
	go.opentelemetry.io/collector/receiver/receivertest v0.149.0
)

require (
	github.com/aws/aws-sdk-go-v2 v1.41.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.7 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/amp v1.42.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.38.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.33.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.63.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.53.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/databasemigrationservice v1.61.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.53.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.280.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.51.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/iam v1.53.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/lambda v1.87.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/rds v1.114.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi v1.31.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/shield v1.34.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/storagegateway v1.43.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/regexp v0.0.0-20240607082908-2cb410fa05da // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/component/componenttest v0.149.0 // indirect
	go.opentelemetry.io/collector/consumer v1.55.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.149.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.149.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.55.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.149.0 // indirect
	go.opentelemetry.io/collector/pdata v1.55.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.149.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.55.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.149.0 // indirect
	go.opentelemetry.io/contrib/bridges/prometheus v0.67.0 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk v1.42.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	golang.org/x/exp v0.0.0-20240823005443-9b4947da3948 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/prometheus-community/yet-another-cloudwatch-exporter => ../
