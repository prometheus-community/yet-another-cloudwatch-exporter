# Embedding YACE in your application

It is possible to embed YACE into an external Go application. This mode might be useful to you if you would like to schedule scrapes yourself, run in a stateless manner, or post-process the generated metrics before exporting them.

The YACE binary scrapes CloudWatch in the background on a fixed interval. Each scrape builds a fresh Prometheus registry and, after the scrape completes, swaps `/metrics` to serve that latest registry. Fetching `/metrics` does not trigger a CloudWatch scrape.

The embedding API exposes a one-shot scrape primitive. Embedders can call it from their own scheduler or request path.

```go
import (
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/metrics"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

cfg := config.DefaultConfig()
cfg.ScrapeConfigFile = "<config-file-path>"
cfg.MetricsPerQuery = 500

scrapeConf := config.ScrapeConf{}
jobsCfg, err := scrapeConf.Load(cfg.ScrapeConfigFile, logger)
if err != nil {
	return err
}

// Collects scrape instrumentation metrics for the lifecycle of the factory and scraper. 
// To disable, you can pass in promutil.Discard to the factory and scraper instead.
registry := prometheus.NewRegistry()
scrapeMetrics := promutil.NewScrapeMetrics(registry)

factory, err := clients.NewFactory(logger, scrapeMetrics, jobsCfg, cfg.FIPSEnabled)
if err != nil {
	return err
}

scraper, err := metrics.NewScraper(logger, scrapeMetrics, cfg, jobsCfg, factory)
if err != nil {
	return err
}

generatedMetrics, err := scraper.Scrape(ctx)
if err != nil {
	return err
}

// Inspect, filter, transform, or forward generatedMetrics.
```

Applications embedding YACE:
- [Grafana Agent](https://github.com/grafana/agent/tree/release-v0.33/pkg/integrations/cloudwatch_exporter)
- [Prometheus OpenTelemetry Collector](https://github.com/prometheus/prometheus-opentelemetry-collector)
