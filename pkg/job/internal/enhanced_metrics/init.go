package enhanced_metrics

import "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/job/internal/enhanced_metrics/service/rds"

var DefaultRegistry = &Registry{}

// init function to register enhanced metric services
func init() {
	// This is where you would register default enhanced metric services if any.
	DefaultRegistry.Register(rds.NewRDSService(nil))
}
