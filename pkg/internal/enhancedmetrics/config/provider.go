package config

import (
	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// RegionalConfigProvider is an interface for providing AWS regional configurations based on region and role.
// Factory interface implementations should implement this interface in order to support enhanced metrics.
type RegionalConfigProvider interface {
	// GetAWSRegionalConfig returns the AWS configuration for a given region and role.
	// It will be used to create AWS service clients for enhanced metrics processing.
	GetAWSRegionalConfig(region string, role model.Role) *aws.Config
}
