package performance

import (
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/game"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/maze"
)

// PoolConfig defines configuration for connection/resource pooling
type PoolConfig struct {
	// Instance management pools
	MaxConcurrentInstances int
	InstancePoolSize       int
	InstanceTimeout        time.Duration
	
	// Player management pools
	MaxConcurrentPlayers   int
	PlayerPoolSize         int
	
	// World management pools
	WorldCacheSize         int
	RoomCacheSize          int
	
	// Performance tuning
	EnablePrefetch         bool
	PrefetchDistance       int
	BatchSize              int
}

// DefaultPoolConfig returns reasonable default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MaxConcurrentInstances: 50,
		InstancePoolSize:       20,
		InstanceTimeout:        30 * time.Minute,
		
		MaxConcurrentPlayers:   200,
		PlayerPoolSize:         100,
		
		WorldCacheSize:         10,
		RoomCacheSize:          500,
		
		EnablePrefetch:         true,
		PrefetchDistance:       2,
		BatchSize:              20,
	}
}

// InstancePool manages a pool of game instances for better performance
type InstancePool struct {
	config        *PoolConfig
	storageMgr    *storage.Manager
	instanceMgr   *instance.InstanceManager
	gameStateMgr  *game.StateManager
	
	// Pool management
	availableInstances chan *PooledInstance
	activeInstances    map[string]*PooledInstance
	instanceLock       sync.RWMutex
	
	// Statistics
	stats         PoolStats
	statsLock     sync.RWMutex
}

// PooledInstance represents an instance in the pool
type PooledInstance struct {
	InstanceID    string
	WorldID       string
	CreatedAt     time.Time
	LastUsed      time.Time
	PlayerCount   int
	Status        InstanceStatus
	
	// Pre-cached data for performance
	World         *storage.World
	Rooms         map[string]*storage.Room
	
	// Performance metrics
	TotalRequests int64
	AverageLatency time.Duration
}

// InstanceStatus represents the status of a pooled instance
type InstanceStatus int

const (
	InstanceStatusAvailable InstanceStatus = iota
	InstanceStatusActive
	InstanceStatusExpiring
	InstanceStatusCleaning
)

// PoolStats tracks pool performance statistics
type PoolStats struct {
	TotalInstances      int     `json:"total_instances"`
	AvailableInstances  int     `json:"available_instances"`
	ActiveInstances     int     `json:"active_instances"`
	PoolHitRate         float64 `json:"pool_hit_rate"`
	AverageWaitTime     time.Duration `json:"average_wait_time"`
	TotalRequests       int64   `json:"total_requests"`
	PoolMisses          int64   `json:"pool_misses"`
	InstancesCreated    int64   `json:"instances_created"`
	InstancesDestroyed  int64   `json:"instances_destroyed"`
}

// NewInstancePool creates a new instance pool
func NewInstancePool(config *PoolConfig, storageMgr *storage.Manager, instanceMgr *instance.InstanceManager, gameStateMgr *game.StateManager) *InstancePool {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool := &InstancePool{
		config:             config,
		storageMgr:         storageMgr,
		instanceMgr:        instanceMgr,
		gameStateMgr:       gameStateMgr,
		availableInstances: make(chan *PooledInstance, config.InstancePoolSize),
		activeInstances:    make(map[string]*PooledInstance),
	}

	// Pre-populate the pool
	go pool.populatePool()

	logging.Info("Created instance pool", 
		"pool_size", config.InstancePoolSize,
		"max_concurrent", config.MaxConcurrentInstances)

	return pool
}

// AcquireInstance gets an available instance from the pool
func (pool *InstancePool) AcquireInstance(playerID string) (*PooledInstance, error) {
	start := time.Now()
	
	pool.statsLock.Lock()
	pool.stats.TotalRequests++
	pool.statsLock.Unlock()

	// Try to get from available pool first
	select {
	case instance := <-pool.availableInstances:
		// Found available instance
		pool.instanceLock.Lock()
		instance.LastUsed = time.Now()
		instance.PlayerCount++
		instance.Status = InstanceStatusActive
		pool.activeInstances[instance.InstanceID] = instance
		pool.instanceLock.Unlock()

		// Update stats
		latency := time.Since(start)
		pool.updateAcquireStats(latency, false)
		
		logging.Debug("Acquired pooled instance", 
			"instance_id", instance.InstanceID,
			"player_id", playerID,
			"latency", latency)

		return instance, nil

	default:
		// No available instance, create new one
		instance, err := pool.createNewInstance(playerID)
		if err != nil {
			pool.statsLock.Lock()
			pool.stats.PoolMisses++
			pool.statsLock.Unlock()
			return nil, err
		}

		// Update stats
		latency := time.Since(start)
		pool.updateAcquireStats(latency, true)

		logging.Info("Created new instance (pool miss)", 
			"instance_id", instance.InstanceID,
			"player_id", playerID,
			"latency", latency)

		return instance, nil
	}
}

// ReleaseInstance returns an instance to the pool
func (pool *InstancePool) ReleaseInstance(instanceID string, playerID string) error {
	pool.instanceLock.Lock()
	defer pool.instanceLock.Unlock()

	instance, exists := pool.activeInstances[instanceID]
	if !exists {
		return nil // Already released or cleaned up
	}

	instance.PlayerCount--
	instance.LastUsed = time.Now()

	// If no players left, return to available pool
	if instance.PlayerCount <= 0 {
		instance.Status = InstanceStatusAvailable
		delete(pool.activeInstances, instanceID)

		// Try to add back to available pool
		select {
		case pool.availableInstances <- instance:
			logging.Debug("Returned instance to pool", 
				"instance_id", instanceID,
				"player_id", playerID)
		default:
			// Pool is full, clean up instance
			go pool.cleanupInstance(instance)
			logging.Debug("Pool full, cleaning up instance", 
				"instance_id", instanceID)
		}
	}

	return nil
}

// createNewInstance creates a new pooled instance
func (pool *InstancePool) createNewInstance(playerID string) (*PooledInstance, error) {
	// Create new instance through instance manager with proper config
	config := maze.DefaultMazeConfig()
	config.CleanupTime = 15 * time.Minute
	mazeInstance, err := pool.instanceMgr.CreateInstance(playerID, config)
	if err != nil {
		return nil, err
	}

	// World data is already loaded in the maze instance
	world := mazeInstance.World

	// Pre-load rooms for performance
	rooms, err := pool.preloadRooms(world.ID)
	if err != nil {
		logging.Warn("Failed to preload rooms for instance", 
			"instance_id", mazeInstance.Config.InstanceID, 
			"error", err)
		rooms = make(map[string]*storage.Room)
	}

	pooledInstance := &PooledInstance{
		InstanceID:  mazeInstance.Config.InstanceID,
		WorldID:     world.ID,
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
		PlayerCount: 1,
		Status:      InstanceStatusActive,
		World:       world,
		Rooms:       rooms,
	}

	// Add to active instances
	pool.instanceLock.Lock()
	pool.activeInstances[mazeInstance.Config.InstanceID] = pooledInstance
	pool.instanceLock.Unlock()

	// Update stats
	pool.statsLock.Lock()
	pool.stats.InstancesCreated++
	pool.statsLock.Unlock()

	return pooledInstance, nil
}

// preloadRooms loads and caches rooms for better performance
func (pool *InstancePool) preloadRooms(worldID string) (map[string]*storage.Room, error) {
	roomIDs, err := pool.storageMgr.Worlds().ListRooms(worldID)
	if err != nil {
		return nil, err
	}

	rooms := make(map[string]*storage.Room, len(roomIDs))
	for _, roomID := range roomIDs {
		room, err := pool.storageMgr.Worlds().LoadRoom(worldID, roomID)
		if err != nil {
			logging.Warn("Failed to preload room", 
				"world_id", worldID, 
				"room_id", roomID, 
				"error", err)
			continue
		}
		rooms[roomID] = room
	}

	logging.Debug("Preloaded rooms for world", 
		"world_id", worldID, 
		"room_count", len(rooms))

	return rooms, nil
}

// populatePool pre-creates instances to populate the pool
func (pool *InstancePool) populatePool() {
	// Don't pre-populate with actual instances since they're expensive
	// Instead, this could prepare other resources or warm up systems
	logging.Debug("Instance pool population complete")
}

// cleanupInstance removes an instance and its associated data
func (pool *InstancePool) cleanupInstance(instance *PooledInstance) {
	instance.Status = InstanceStatusCleaning
	
	// Clean up through instance manager
	if err := pool.instanceMgr.CleanupInstance(instance.InstanceID); err != nil {
		logging.Error("Failed to cleanup pooled instance", 
			"instance_id", instance.InstanceID, 
			"error", err)
	}

	// Update stats
	pool.statsLock.Lock()
	pool.stats.InstancesDestroyed++
	pool.statsLock.Unlock()

	logging.Debug("Cleaned up pooled instance", "instance_id", instance.InstanceID)
}

// updateAcquireStats updates statistics for instance acquisition
func (pool *InstancePool) updateAcquireStats(latency time.Duration, wasPoolMiss bool) {
	pool.statsLock.Lock()
	defer pool.statsLock.Unlock()

	if wasPoolMiss {
		pool.stats.PoolMisses++
	}

	// Update average wait time
	totalRequests := pool.stats.TotalRequests
	if totalRequests > 1 {
		pool.stats.AverageWaitTime = ((pool.stats.AverageWaitTime * time.Duration(totalRequests-1)) + latency) / time.Duration(totalRequests)
	} else {
		pool.stats.AverageWaitTime = latency
	}

	// Update hit rate
	if totalRequests > 0 {
		pool.stats.PoolHitRate = float64(totalRequests-pool.stats.PoolMisses) / float64(totalRequests)
	}
}

// GetStats returns current pool statistics
func (pool *InstancePool) GetStats() PoolStats {
	pool.statsLock.RLock()
	defer pool.statsLock.RUnlock()

	stats := pool.stats

	// Update real-time counts
	pool.instanceLock.RLock()
	stats.ActiveInstances = len(pool.activeInstances)
	stats.AvailableInstances = len(pool.availableInstances)
	stats.TotalInstances = stats.ActiveInstances + stats.AvailableInstances
	pool.instanceLock.RUnlock()

	return stats
}

// GetCachedRoom retrieves a room from the instance cache
func (instance *PooledInstance) GetCachedRoom(roomID string) (*storage.Room, bool) {
	room, exists := instance.Rooms[roomID]
	return room, exists
}

// UpdateRoomCache updates a room in the instance cache
func (instance *PooledInstance) UpdateRoomCache(room *storage.Room) {
	instance.Rooms[room.ID] = room
}

// CleanupExpiredInstances removes instances that have expired
func (pool *InstancePool) CleanupExpiredInstances() int {
	cutoffTime := time.Now().Add(-pool.config.InstanceTimeout)
	var cleaned int

	pool.instanceLock.Lock()
	defer pool.instanceLock.Unlock()

	// Check active instances for expiration
	for instanceID, instance := range pool.activeInstances {
		if instance.LastUsed.Before(cutoffTime) && instance.PlayerCount <= 0 {
			instance.Status = InstanceStatusExpiring
			delete(pool.activeInstances, instanceID)
			go pool.cleanupInstance(instance)
			cleaned++
		}
	}

	// Check available instances for expiration
	availableCount := len(pool.availableInstances)
	for i := 0; i < availableCount; i++ {
		select {
		case instance := <-pool.availableInstances:
			if instance.LastUsed.Before(cutoffTime) {
				go pool.cleanupInstance(instance)
				cleaned++
			} else {
				// Put it back
				select {
				case pool.availableInstances <- instance:
				default:
					// Pool is full, clean it up anyway
					go pool.cleanupInstance(instance)
					cleaned++
				}
			}
		default:
			break
		}
	}

	if cleaned > 0 {
		logging.Info("Cleaned up expired instances", "count", cleaned)
	}

	return cleaned
}

// Shutdown shuts down the instance pool
func (pool *InstancePool) Shutdown() {
	logging.Info("Shutting down instance pool")

	// Clean up all active instances
	pool.instanceLock.Lock()
	for _, instance := range pool.activeInstances {
		go pool.cleanupInstance(instance)
	}
	pool.activeInstances = make(map[string]*PooledInstance)
	pool.instanceLock.Unlock()

	// Clean up available instances
	for {
		select {
		case instance := <-pool.availableInstances:
			go pool.cleanupInstance(instance)
		default:
			goto cleanup_complete
		}
	}

cleanup_complete:
	logging.Info("Instance pool shutdown complete")
}