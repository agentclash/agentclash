package connection

import (
	"sync"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/google/uuid"
)

// modelsCacheTTL is how long a live model list is served before a refetch.
const modelsCacheTTL = time.Hour

// modelsCache is a small in-memory, per-process cache of provider model lists
// keyed by provider account id. The API server is single-process, so this is a
// plain mutex-guarded map. Entries are kept past their TTL so they can be served
// stale when a live refetch fails (see Service.ListModels).
type modelsCache struct {
	mu      sync.Mutex
	entries map[uuid.UUID]modelsCacheEntry
	ttl     time.Duration
	now     func() time.Time
}

type modelsCacheEntry struct {
	models    []provider.ModelInfo
	fetchedAt time.Time
}

func newModelsCache() *modelsCache {
	return &modelsCache{
		entries: make(map[uuid.UUID]modelsCacheEntry),
		ttl:     modelsCacheTTL,
		now:     time.Now,
	}
}

// get returns the cached models for an account when present and fresh.
func (c *modelsCache) get(id uuid.UUID) ([]provider.ModelInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok || c.now().Sub(entry.fetchedAt) >= c.ttl {
		return nil, false
	}
	return entry.models, true
}

// getStale returns the cached models regardless of age. Used to keep serving a
// last-known-good list when a live refetch fails.
func (c *modelsCache) getStale(id uuid.UUID) ([]provider.ModelInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok {
		return nil, false
	}
	return entry.models, true
}

func (c *modelsCache) set(id uuid.UUID, models []provider.ModelInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[id] = modelsCacheEntry{models: models, fetchedAt: c.now()}
}

func (c *modelsCache) invalidate(id uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}
