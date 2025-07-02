package resourceinventory

import (
	"context"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Store interface {
	GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error)
}

type nopStore struct{}

func (nopStore) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	return nil, nil
}

func Nop() Store { return nopStore{} }
