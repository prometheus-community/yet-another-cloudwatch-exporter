package cache

import (
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// separator is NULL byte (0x00)
const Separator = "\x00"

func GetClientKey(region string, role model.Role) string {
	return region + Separator + role.RoleArn + Separator + role.ExternalID
}
