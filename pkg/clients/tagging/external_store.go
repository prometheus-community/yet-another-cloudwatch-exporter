package tagging

import (
	"context"
	"fmt"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/resourceinventory"
)

func WithExternalStore(primary Client, store resourceinventory.Store) Client {
	if store == nil {
		store = resourceinventory.Nop()
	}
	return &taggingWithExternalStore{
		primary:       primary,
		externalStore: store,
	}
}

type taggingWithExternalStore struct {
	primary       Client
	externalStore resourceinventory.Store
}

func (f *taggingWithExternalStore) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	primRes, primErr := f.primary.GetResources(ctx, job, region)

	storeRes, storeErr := f.externalStore.GetResources(ctx, job, region)

	if primErr != nil && storeErr != nil {
		return nil, fmt.Errorf("failed to get resources from primary and store: %w", primErr)
	}

	// merge dedupe is nil safe
	return mergeDedupResources(primRes, storeRes), nil
}

func mergeDedupResources(resourcesLists ...[]*model.TaggedResource) []*model.TaggedResource {
	dedup := make(map[string]*model.TaggedResource)

	for _, resources := range resourcesLists {
		for _, r := range resources {
			dedup[r.ARN] = r
		}
	}

	deduped := make([]*model.TaggedResource, 0, len(dedup))
	for _, r := range dedup {
		deduped = append(deduped, r)
	}
	return deduped
}
