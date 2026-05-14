# Embedding YACE in your application

It is possible to embed YACE into an external Go application. This mode might be useful to you if you would like to schedule scrapes yourself, run in a stateless manner, or post-process the generated metrics before exporting them.

The YACE binary scrapes CloudWatch in the background on a fixed interval. Each scrape builds a fresh Prometheus registry and, after the scrape completes, swaps `/metrics` to serve that latest registry. Fetching `/metrics` does not trigger a CloudWatch scrape.

The embedding API exposes a one-shot scrape primitive. Embedders can call it from their own scheduler or request path.

```go
import (
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/clients"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/metrics"
)

scrapeConf := config.ScrapeConf{}
jobsCfg, err := scrapeConf.Load("<config-file-path>", logger)
if err != nil {
	return err
}

factory, err := clients.NewFactory(logger, jobsCfg, false)
if err != nil {
	return err
}

cfg := config.DefaultConfig()
cfg.MetricsPerQuery = 500

scraper, err := metrics.NewScraper(logger, cfg, jobsCfg, factory)
if err != nil {
	return err
}

generatedMetrics, err := scraper.Scrape(ctx)
if err != nil {
	return err
}

// Inspect, filter, transform, or forward generatedMetrics.
```

Callers that want to expose one scrape through a fresh Prometheus registry can register the scraper collectors and generated metrics explicitly.

```go
import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/promutil"
)

registry := prometheus.NewRegistry()
scraper, err := metrics.NewScraper(logger, cfg, jobsCfg, factory)
if err != nil {
	return err
}
if err := scraper.RegisterCollectors(registry); err != nil {
	return err
}

generatedMetrics, err := scraper.Scrape(ctx)
if err != nil {
	return err
}
registry.MustRegister(promutil.NewPrometheusCollector(generatedMetrics))
```

Applications embedding YACE:
- [Grafana Agent](https://github.com/grafana/agent/tree/release-v0.33/pkg/integrations/cloudwatch_exporter)
- [Prometheus OpenTelemetry Collector](https://github.com/prometheus/prometheus-opentelemetry-collector)
