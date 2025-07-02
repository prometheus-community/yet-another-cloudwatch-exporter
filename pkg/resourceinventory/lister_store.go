package resourceinventory

import (
	"context"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"golang.org/x/sync/singleflight"

	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
)

type Lister interface {
	Namespace() string
	List(ctx context.Context, region, roleArn string) ([]*model.TaggedResource, error)
}

type ListerStore struct {
	listers map[string]Lister

	sem chan struct{}

	cache *ttlcache.Cache[string, []*model.TaggedResource]
	group singleflight.Group
}

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
