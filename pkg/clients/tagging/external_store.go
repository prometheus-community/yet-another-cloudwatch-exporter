package tagging

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/resourceinventory"
)

func WithExternalStore(logger *slog.Logger, primary Client, store resourceinventory.Store) Client {
	if store == nil {
		store = resourceinventory.Nop()
	}
	return &taggingWithExternalStore{
		primary:       primary,
		logger:        logger,
		externalStore: store,
	}
}

type taggingWithExternalStore struct {
	primary       Client
	logger        *slog.Logger
	externalStore resourceinventory.Store
}

func (f *taggingWithExternalStore) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	primRes, primErr := f.primary.GetResources(ctx, job, region)

	storeRes, storeErr := f.externalStore.GetResources(ctx, job, region)

	if storeErr != nil {
		f.logger.Error("failed to get resources from external store", "error", storeErr)
	}

	if primErr != nil {
		f.logger.Error("failed to get resources from primary", "error", primErr)
	}

	if storeErr != nil && primErr != nil {
		return nil, fmt.Errorf("failed to get resources from primary and store: %w", storeErr)
	}

	// merge dedupe is nil safe, priority is the tagging service (the current yace implementation)
	return f.mergeDedupResources(primRes, storeRes), nil
}

func (f *taggingWithExternalStore) mergeDedupResources(taggingService, externalStore []*model.TaggedResource) []*model.TaggedResource {
	dedup := make(map[string]*model.TaggedResource)

	for _, resource := range externalStore {
		dedup[resource.ARN] = resource
	}

	for _, resource := range taggingService {
		dedup[resource.ARN] = resource
	}

	deduped := make([]*model.TaggedResource, 0, len(dedup))
	for _, r := range dedup {
		deduped = append(deduped, r)
	}
	return deduped
}
