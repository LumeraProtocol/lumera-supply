package cache

import (
	"log"
	"sync"
	"time"

	"github.com/lumera-labs/lumera-supply/pkg/supply"
	"github.com/lumera-labs/lumera-supply/pkg/types"
)

type Options struct {
	TTL time.Duration
}

type SnapshotCache struct {
	mu   sync.RWMutex
	snap *types.SupplySnapshot
	etag string
	ttl  time.Duration
	comp *supply.Computer
}

func NewSnapshotCache(comp *supply.Computer, opt Options) *SnapshotCache {
	if opt.TTL <= 0 {
		opt.TTL = 60 * time.Second
	}
	return &SnapshotCache{ttl: opt.TTL, comp: comp}
}

func (c *SnapshotCache) Get() (*types.SupplySnapshot, bool) {
	c.mu.RLock()
	s := c.snap
	c.mu.RUnlock()
	if s == nil {
		return nil, false
	}
	if time.Since(s.UpdatedAt) > c.ttl {
		return s, false
	}
	return s, true
}

func (c *SnapshotCache) Update(denom string) (*types.SupplySnapshot, error) {
	s, err := c.comp.ComputeSnapshot(denom)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.snap = s
	c.etag = s.ETag
	c.mu.Unlock()
	return s, nil
}

// RunRefresher refreshes the snapshot every TTL seconds.
func (c *SnapshotCache) RunRefresher(denom string) {
	for {
		if _, err := c.Update(denom); err != nil {
			log.Printf("refresher error: %v", err)
		}
		time.Sleep(c.ttl)
	}
}
