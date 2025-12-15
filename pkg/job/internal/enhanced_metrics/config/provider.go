package config

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type RegionalConfigProvider interface {
	GetAWSRegionalConfig(region string, role model.Role) *aws.Config
}
