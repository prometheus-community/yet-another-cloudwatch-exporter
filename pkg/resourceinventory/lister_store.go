package resourceinventory

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"golang.org/x/sync/singleflight"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

// Lister defines a pluggable function that can enumerate resources for a given
// discovery job/region.  A lister should return only resources that belong to
// the job.Namespace it was registered for.
//
// It should be safe for concurrent use by multiple goroutines.
//
// If it has nothing to return it MUST return (nil, nil).
// If the listing fails it MUST return (nil, err).
//
// A concrete implementation can use the AWS SDK (v1 or v2) or any other means
// to build the resource slice.
type Lister interface {
	// Namespace returns the CloudWatch namespace this lister covers, e.g. "AWS/SQS".
	Namespace() string

	// List enumerates resources for the given AWS region. It should return nil, nil
	// if no resources are found.
	List(ctx context.Context, region, roleArn string) ([]*model.TaggedResource, error)
}

// ListerStore is a resourceinventory.Store implementation that delegates the
// actual listing work to service-specific Listers.  Each List operation is
// executed under a semaphore to limit concurrency.
//
// Register listers once at startup and then share this store across scrapes.
type ListerStore struct {
	listers map[string]Lister // keyed by job.Namespace (e.g. "AWS/SQS")

	sem chan struct{}

	cache *ttlcache.Cache[string, []*model.TaggedResource]
	group singleflight.Group
}

// NewListerStore returns a Store that will allow up to maxConcurrency List
// calls to be in-flight at the same time. If ttl is >0 the results will be
// cached for that duration.
func NewListerStore(maxConcurrency int, ttl time.Duration, listers ...Lister) *ListerStore {
	var cache *ttlcache.Cache[string, []*model.TaggedResource]
	if ttl > 0 {
		cache = ttlcache.New[string, []*model.TaggedResource](
			ttlcache.WithTTL[string, []*model.TaggedResource](ttl),
		)
		go cache.Start()
	}

	storedListers := make(map[string]Lister)
	for _, lister := range listers {
		storedListers[lister.Namespace()] = lister
	}

	return &ListerStore{
		listers: storedListers,
		sem:     make(chan struct{}, maxConcurrency),
		cache:   cache,
	}
}

// GetResources satisfies the resourceinventory.Store interface.  It looks up a
// lister for job.Namespace and invokes it if present.
func (s *ListerStore) GetResources(ctx context.Context, job model.DiscoveryJob, region string) ([]*model.TaggedResource, error) {
	lister, ok := s.listers[job.Namespace]
	if !ok {
		return nil, nil // no lister for this service
	}

	key := job.Namespace + "|" + region
	// Return cached copy if available
	if s.cache != nil {
		if item := s.cache.Get(key); item != nil {
			return item.Value(), nil
		}
	}

	// Use singleflight to avoid duplicate work
	v, err, _ := s.group.Do(key, func() (interface{}, error) {
		dedupedResources := make(map[string]*model.TaggedResource)
		// Concurrency control per-store to avoid hammering AWS APIs.
		s.sem <- struct{}{}
		defer func() { <-s.sem }()

		for _, role := range job.Roles {
			res, err := lister.List(ctx, region, role.RoleArn)
			if err != nil {
				return nil, err
			}

			for _, resource := range res {
				dedupedResources[resource.ARN] = resource
			}
		}

		return dedupedResources, nil
	})
	if err != nil {
		return nil, err
	}

	dedupeMap := v.(map[string]*model.TaggedResource)
	resources := make([]*model.TaggedResource, 0, len(dedupeMap))
	for _, resource := range dedupeMap {
		resources = append(resources, resource)
	}

	return resources, nil
}
