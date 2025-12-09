# CLAUDE.md - AI Assistant Guide for YACE

This document provides context and guidance for AI assistants (like Claude Code) working with the Yet Another CloudWatch Exporter (YACE) codebase.

## Project Overview

YACE is a Prometheus exporter for AWS CloudWatch metrics, written in Go. It's part of the [prometheus-community](https://github.com/prometheus-community) organization as of November 2024.

**Key characteristics:**
- Can be used both as a **standalone CLI application** and as a **library** embedded in other Go applications
- Supports auto-discovery of AWS resources via tags
- Handles 100+ AWS services (ECS, RDS, Lambda, S3, etc.)
- Implements decoupled scraping to protect against AWS API abuse
- Supports multiple AWS accounts via cross-account roles

## Architecture

### Dual-Purpose Design

The project is structured to serve two purposes:

1. **Library** (`pkg/`): Core functionality that can be embedded in other applications
   - Main entry point: `pkg/exporter.go` - `UpdateMetrics()` function
   - Can be imported as: `github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg`

2. **CLI Application** (`cmd/yace/`): Standalone Prometheus exporter server
   - Main entry point: `cmd/yace/main.go`
   - Runs HTTP server on port 5000 (default) exposing `/metrics` endpoint
   - Supports configuration file, feature flags, and CLI arguments

### Key Components

```
pkg/
├── clients/           # AWS SDK client factories (v1 and v2)
│   ├── v1/           # AWS SDK v1 implementation
│   ├── v2/           # AWS SDK v2 implementation (default)
│   └── cloudwatch/   # CloudWatch API client with concurrency control
├── config/           # Configuration parsing and validation
├── job/              # Job orchestration and scraping logic
├── model/            # Data models (metrics, tags, dimensions)
├── promutil/         # Prometheus metric building and utilities
└── exporter.go       # Main library entry point

cmd/yace/
├── main.go           # CLI entry point and HTTP server
└── scraper.go        # Scraping coordinator (decoupled mode)
```

## Key Concepts

### Discovery Jobs vs Static Jobs

- **Discovery Jobs**: Auto-discover AWS resources by tags and fetch their metrics
- **Static Jobs**: Explicitly specify CloudWatch metrics to fetch (no auto-discovery)

### Decoupled Scraping

The exporter scrapes CloudWatch in the background at a fixed interval (default: 300s) rather than on every `/metrics` request. This:
- Prevents AWS API abuse and unexpected billing
- Provides consistent scraping intervals
- Protects against scrape timeouts

### Feature Flags

The project uses feature flags for backwards compatibility and experimental features. See `docs/feature_flags.md` for details.

Example: `aws-sdk-v1` feature flag switches from AWS SDK v2 (default) to v1.

### Concurrency Control

YACE implements careful concurrency limiting to avoid AWS API throttling:
- CloudWatch API concurrency (default: 5 concurrent requests)
- Per-API concurrency limits (ListMetrics, GetMetricData, GetMetricStatistics)
- Resource Tagging API concurrency (default: 5 concurrent requests)

## Working with the Codebase

### Configuration Files

Configuration is YAML-based. Key sections:
- `apiVersion`: Config schema version (currently `v1alpha1`)
- `sts-region`: AWS STS region for assuming roles
- `discovery`: Discovery job definitions
- `static`: Static metric definitions

See `docs/configuration.md` for comprehensive configuration documentation.

### Common Development Tasks

**Reading Configuration:**
```go
cfg := config.ScrapeConf{}
jobsCfg, err := cfg.Load(configFile, logger)
```

**Using the Library:**
```go
import exporter "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg"

// Create registry and factory
registry := prometheus.NewRegistry()
factory, err := v2.NewFactory(logger, jobsCfg, false)

// Scrape metrics
err = exporter.UpdateMetrics(ctx, logger, jobsCfg, registry, factory,
    exporter.MetricsPerQuery(500),
    exporter.CloudWatchAPIConcurrency(5),
)
```

**CLI Commands:**
```bash
# Run exporter
yace --config.file config.yml

# Verify configuration
yace verify-config --config.file config.yml

# Version info
yace version
```

### Testing

- Unit tests: `go test ./...`
- Integration tests may require AWS credentials
- Use `AWS_ENDPOINT_URL` environment variable for local testing with AWS mocks

### Important Files to Understand

1. **pkg/exporter.go**: Main library entry point - understand the `UpdateMetrics()` function
2. **pkg/job/scrape.go**: Core scraping logic (likely location)
3. **pkg/promutil/**: Metric building and label handling
4. **pkg/config/config.go**: Configuration structure and validation
5. **cmd/yace/main.go**: CLI flags and HTTP server setup
6. **cmd/yace/scraper.go**: Decoupled scraping coordinator

## Code Patterns and Conventions

### Logging

Uses `log/slog` (structured logging):
```go
logger.Info("message", "key", value, "key2", value2)
logger.Error("error occurred", "err", err)
```

### AWS SDK Versions

- **Default**: AWS SDK v2 (`pkg/clients/v2/`)
- **Optional**: AWS SDK v1 via feature flag (`pkg/clients/v1/`)

### Error Handling

- Return errors up the stack; don't panic
- Log errors with context before returning
- Validate configuration early

### Prometheus Metrics

The exporter tracks its own operations:
- `yace_cloudwatch_requests_total`: Total CloudWatch API requests
- Various API-specific counters (see `pkg/promutil/` for full list)
- These are defined in `exporter.Metrics` slice

## AWS Permissions

Minimum IAM permissions required:
```json
{
  "tag:GetResources",
  "cloudwatch:GetMetricData",
  "cloudwatch:GetMetricStatistics",
  "cloudwatch:ListMetrics"
}
```

Additional permissions needed for specific namespaces (e.g., `apigateway:GET` for API Gateway, `autoscaling:DescribeAutoScalingGroups` for Auto Scaling).

Full permissions documented in README.md.

## Common Pitfalls

1. **Intermittent Metrics**: CloudWatch metrics have delays. Use appropriate `length` and `period` values.
2. **API Throttling**: AWS CloudWatch APIs have rate limits. Tune concurrency settings carefully.
3. **Cost Management**: CloudWatch API requests cost money. The decoupled scraping mode helps control costs.
4. **Cross-Account Access**: Requires proper IAM role setup with trust relationships.

## Building and Running

**Build:**
```bash
go build -o yace ./cmd/yace
```

**Run:**
```bash
./yace --config.file config.yml
```

**Docker:**
```bash
docker run -d --rm \
  -v $PWD/config.yml:/tmp/config.yml \
  -p 5000:5000 \
  quay.io/prometheuscommunity/yet-another-cloudwatch-exporter:latest
```

## Module Information

- **Module Path**: `github.com/prometheus-community/yet-another-cloudwatch-exporter`
- **Go Version**: 1.24.0
- **Main Dependencies**:
  - `github.com/prometheus/client_golang`: Prometheus client library
  - `github.com/aws/aws-sdk-go-v2`: AWS SDK v2 (default)
  - `github.com/aws/aws-sdk-go`: AWS SDK v1 (optional)
  - `github.com/urfave/cli/v2`: CLI framework

## Documentation

- **README.md**: User-facing documentation, feature list
- **docs/configuration.md**: Detailed configuration options
- **docs/embedding.md**: Guide for using YACE as a library
- **docs/installation.md**: Installation instructions
- **docs/feature_flags.md**: Available feature flags
- **CONTRIBUTE.md**: Development setup guide

## Supported AWS Services

YACE supports 100+ AWS CloudWatch namespaces with auto-discovery, including:
- Compute: EC2, Lambda, ECS, EKS
- Storage: S3, EBS, EFS
- Database: RDS, DynamoDB, ElastiCache
- Networking: ELB, ALB, NLB, API Gateway
- And many more (see README.md for full list)

## When Making Changes

1. **Breaking Changes**: Be cautious - this is pre-1.0 software, but breaking changes should be documented in CHANGELOG.md
2. **Deprecation**: Prefer deprecating features over immediate removal
3. **Testing**: Ensure changes work with both discovery and static jobs
4. **AWS SDK**: Changes should work with both v1 and v2 SDKs unless feature-flagged
5. **Configuration**: Maintain backwards compatibility where possible

## Getting Help

- **Issues**: https://github.com/prometheus-community/yet-another-cloudwatch-exporter/issues
- **Security**: See SECURITY.md for reporting vulnerabilities
