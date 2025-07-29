package performance

import (
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
)

// CacheEntry represents a cached property value with TTL
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
	Hits      int64
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// PlayerPropertyCache provides high-performance caching for frequently accessed player properties
type PlayerPropertyCache struct {
	cache        map[string]map[string]*CacheEntry // playerID -> propertyKey -> entry
	mutex        sync.RWMutex
	defaultTTL   time.Duration
	maxEntries   int
	hits         int64
	misses       int64
	cleanupTimer *time.Ticker
	stopCleanup  chan bool
}

// NewPlayerPropertyCache creates a new player property cache
func NewPlayerPropertyCache(defaultTTL time.Duration, maxEntries int) *PlayerPropertyCache {
	cache := &PlayerPropertyCache{
		cache:        make(map[string]map[string]*CacheEntry),
		defaultTTL:   defaultTTL,
		maxEntries:   maxEntries,
		cleanupTimer: time.NewTicker(time.Minute), // Cleanup every minute
		stopCleanup:  make(chan bool),
	}

	// Start background cleanup
	go cache.runCleanup()

	return cache
}

// Get retrieves a cached property value
func (c *PlayerPropertyCache) Get(playerID, propertyKey string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	playerCache, exists := c.cache[playerID]
	if !exists {
		c.misses++
		return nil, false
	}

	entry, exists := playerCache[propertyKey]
	if !exists || entry.IsExpired() {
		c.misses++
		return nil, false
	}

	entry.Hits++
	c.hits++
	return entry.Value, true
}

// Set stores a property value in the cache
func (c *PlayerPropertyCache) Set(playerID, propertyKey string, value interface{}) {
	c.SetWithTTL(playerID, propertyKey, value, c.defaultTTL)
}

// SetWithTTL stores a property value with a custom TTL
func (c *PlayerPropertyCache) SetWithTTL(playerID, propertyKey string, value interface{}, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Ensure player cache exists
	if c.cache[playerID] == nil {
		c.cache[playerID] = make(map[string]*CacheEntry)
	}

	// Check if we need to evict entries
	if c.getTotalEntries() >= c.maxEntries {
		c.evictLRU()
	}

	// Store the entry
	c.cache[playerID][propertyKey] = &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		Hits:      0,
	}
}

// Invalidate removes a specific property from cache
func (c *PlayerPropertyCache) Invalidate(playerID, propertyKey string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if playerCache, exists := c.cache[playerID]; exists {
		delete(playerCache, propertyKey)
		
		// Clean up empty player cache
		if len(playerCache) == 0 {
			delete(c.cache, playerID)
		}
	}
}

// InvalidatePlayer removes all cached properties for a player
func (c *PlayerPropertyCache) InvalidatePlayer(playerID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.cache, playerID)
}

// GetStats returns cache performance statistics
func (c *PlayerPropertyCache) GetStats() CacheStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	totalEntries := c.getTotalEntries()
	hitRate := float64(0)
	if c.hits+c.misses > 0 {
		hitRate = float64(c.hits) / float64(c.hits+c.misses)
	}

	return CacheStats{
		Hits:         c.hits,
		Misses:       c.misses,
		HitRate:      hitRate,
		TotalEntries: totalEntries,
		Players:      len(c.cache),
	}
}

// CacheStats represents cache performance statistics
type CacheStats struct {
	Hits         int64   `json:"hits"`
	Misses       int64   `json:"misses"`
	HitRate      float64 `json:"hit_rate"`
	TotalEntries int     `json:"total_entries"`
	Players      int     `json:"players"`
}

// getTotalEntries returns the total number of cached entries (not thread safe)
func (c *PlayerPropertyCache) getTotalEntries() int {
	total := 0
	for _, playerCache := range c.cache {
		total += len(playerCache)
	}
	return total
}

// evictLRU removes the least recently used entries (not thread safe)
func (c *PlayerPropertyCache) evictLRU() {
	// Find entry with lowest hit count
	var lruPlayerID, lruPropertyKey string
	var lruHits int64 = -1

	for playerID, playerCache := range c.cache {
		for propertyKey, entry := range playerCache {
			if lruHits == -1 || entry.Hits < lruHits {
				lruHits = entry.Hits
				lruPlayerID = playerID
				lruPropertyKey = propertyKey
			}
		}
	}

	// Remove the LRU entry
	if lruPlayerID != "" {
		delete(c.cache[lruPlayerID], lruPropertyKey)
		if len(c.cache[lruPlayerID]) == 0 {
			delete(c.cache, lruPlayerID)
		}
		
		logging.Debug("Evicted LRU cache entry", 
			"player_id", lruPlayerID, 
			"property_key", lruPropertyKey, 
			"hits", lruHits)
	}
}

// runCleanup periodically removes expired entries
func (c *PlayerPropertyCache) runCleanup() {
	for {
		select {
		case <-c.cleanupTimer.C:
			c.cleanup()
		case <-c.stopCleanup:
			c.cleanupTimer.Stop()
			return
		}
	}
}

// cleanup removes expired entries
func (c *PlayerPropertyCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	expiredCount := 0
	now := time.Now()

	for playerID, playerCache := range c.cache {
		for propertyKey, entry := range playerCache {
			if now.After(entry.ExpiresAt) {
				delete(playerCache, propertyKey)
				expiredCount++
			}
		}

		// Remove empty player caches
		if len(playerCache) == 0 {
			delete(c.cache, playerID)
		}
	}

	if expiredCount > 0 {
		logging.Debug("Cleaned up expired cache entries", "count", expiredCount)
	}
}

// Shutdown stops the cache and cleanup processes
func (c *PlayerPropertyCache) Shutdown() {
	close(c.stopCleanup)
	
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Clear all cache entries
	c.cache = make(map[string]map[string]*CacheEntry)
	
	logging.Info("Player property cache shutdown complete")
}

// CachedPlayerOperations provides cached access to player properties
type CachedPlayerOperations struct {
	cache      *PlayerPropertyCache
	storageMgr *storage.Manager
}

// NewCachedPlayerOperations creates a new cached player operations handler
func NewCachedPlayerOperations(cache *PlayerPropertyCache, storageMgr *storage.Manager) *CachedPlayerOperations {
	return &CachedPlayerOperations{
		cache:      cache,
		storageMgr: storageMgr,
	}
}

// GetWumpusAuth retrieves authentication data with caching
func (ops *CachedPlayerOperations) GetWumpusAuth(player *storage.Player) (*auth.WumpusAuth, error) {
	cacheKey := "wumpus_auth"
	
	// Try cache first
	if cached, found := ops.cache.Get(player.ID, cacheKey); found {
		if authData, ok := cached.(*auth.WumpusAuth); ok {
			return authData, nil
		}
	}

	// Cache miss, get from storage
	authData, err := auth.GetWumpusAuth(player)
	if err != nil {
		return nil, err
	}

	// Cache the result with shorter TTL for auth data (more sensitive)
	ops.cache.SetWithTTL(player.ID, cacheKey, authData, 5*time.Minute)
	
	return authData, nil
}

// GetWumpusStats retrieves player stats with caching
func (ops *CachedPlayerOperations) GetWumpusStats(player *storage.Player) auth.WumpusStats {
	cacheKey := "wumpus_stats"
	
	// Try cache first
	if cached, found := ops.cache.Get(player.ID, cacheKey); found {
		if stats, ok := cached.(auth.WumpusStats); ok {
			return stats
		}
	}

	// Cache miss, get from storage
	stats := auth.GetWumpusStats(player)
	
	// Cache the result
	ops.cache.Set(player.ID, cacheKey, stats)
	
	return stats
}

// UpdateWumpusStats updates player stats and invalidates cache
func (ops *CachedPlayerOperations) UpdateWumpusStats(player *storage.Player, stats auth.WumpusStats) {
	// Update storage
	auth.UpdateWumpusStats(player, stats)
	
	// Invalidate cache to ensure consistency
	ops.cache.Invalidate(player.ID, "wumpus_stats")
}

// GetWumpusState retrieves player state with caching
func (ops *CachedPlayerOperations) GetWumpusState(player *storage.Player) auth.WumpusState {
	cacheKey := "wumpus_state"
	
	// Try cache first
	if cached, found := ops.cache.Get(player.ID, cacheKey); found {
		if state, ok := cached.(auth.WumpusState); ok {
			return state
		}
	}

	// Cache miss, get from storage
	state := auth.GetWumpusState(player)
	
	// Cache with shorter TTL for frequently changing state
	ops.cache.SetWithTTL(player.ID, cacheKey, state, 2*time.Minute)
	
	return state
}

// UpdateWumpusState updates player state and invalidates cache
func (ops *CachedPlayerOperations) UpdateWumpusState(player *storage.Player, state auth.WumpusState) {
	// Update storage
	auth.UpdateWumpusState(player, state)
	
	// Invalidate cache to ensure consistency
	ops.cache.Invalidate(player.ID, "wumpus_state")
}

// ValidatePlayerPassword validates password with auth data caching
func (ops *CachedPlayerOperations) ValidatePlayerPassword(player *storage.Player, password string) bool {
	// Use the existing validation logic directly
	return auth.ValidatePlayerPassword(player, password)
}

// InvalidatePlayerCache removes all cached data for a player
func (ops *CachedPlayerOperations) InvalidatePlayerCache(playerID string) {
	ops.cache.InvalidatePlayer(playerID)
}