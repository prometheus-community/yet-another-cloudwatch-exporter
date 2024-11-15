# Embedding YACE in your application

It is possible to embed YACE into an external Go application. This mode might be useful to you if you would like to scrape on demand or run in a stateless manner.

See [`exporter.UpdateMetrics()`](https://pkg.go.dev/github.com/prometheus-community/yet-another-cloudwatch-exporter@v0.50.0/pkg#UpdateMetrics) for the documentation of the exporter entrypoint.

Applications embedding YACE:
- [Grafana Agent](https://github.com/grafana/agent/tree/release-v0.33/pkg/integrations/cloudwatch_exporter)
