# Embedding YACE in your application

It is possible to embed YACE into an external Go application. This mode might be useful to you if you would like to scrape on demand, run in a stateless manner, or post-process the generated metrics before exporting them.

YACE exposes two embedding entrypoints:

- [`exporter.BuildPrometheusMetrics()`](https://pkg.go.dev/github.com/prometheus-community/yet-another-cloudwatch-exporter@v0.50.0/pkg#BuildPrometheusMetrics) returns the generated metrics so callers can apply their own post-scrape transformations before forwarding them elsewhere.
- [`exporter.UpdateMetrics()`](https://pkg.go.dev/github.com/prometheus-community/yet-another-cloudwatch-exporter@v0.66.0/pkg#UpdateMetrics) writes the generated metrics into a Prometheus registry directly, using the variable address pointer.
Applications embedding YACE:
- [Grafana Agent](https://github.com/grafana/agent/tree/release-v0.33/pkg/integrations/cloudwatch_exporter)
- [Prometheus OpenTelemetry Collector](https://github.com/prometheus/prometheus-opentelemetry-collector)
