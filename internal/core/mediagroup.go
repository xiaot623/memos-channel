package core

import (
	"sync"
	"time"

	v1pb "github.com/usememos/memos/proto/gen/api/v1"
)

type groupCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]groupEntry
	now   func() time.Time
}

type groupEntry struct {
	memo      *v1pb.Memo
	expiresAt time.Time
}

func newGroupCache(ttl time.Duration) *groupCache {
	return &groupCache{
		ttl:   ttl,
		items: make(map[string]groupEntry),
		now:   time.Now,
	}
}

func (c *groupCache) getOrCreate(key string, create func() (*v1pb.Memo, error)) (*v1pb.Memo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	c.evictLocked(now)
	if entry, ok := c.items[key]; ok && now.Before(entry.expiresAt) {
		return entry.memo, nil
	}

	memo, err := create()
	if err != nil {
		return nil, err
	}
	c.items[key] = groupEntry{memo: memo, expiresAt: now.Add(c.ttl)}
	return memo, nil
}

func (c *groupCache) evictLocked(now time.Time) {
	for key, entry := range c.items {
		if !now.Before(entry.expiresAt) {
			delete(c.items, key)
		}
	}
}
