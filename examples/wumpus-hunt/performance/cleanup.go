package performance

import (
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// CleanupConfig defines configuration for the cleanup system
type CleanupConfig struct {
	// How often to run cleanup
	CleanupInterval time.Duration
	
	// Age thresholds for different cleanup operations
	TempWorldMaxAge       time.Duration
	PlayerSessionMaxAge   time.Duration
	OrphanedDataMaxAge    time.Duration
	
	// Batch sizes for cleanup operations
	BatchSize int
	
	// Whether to run cleanup in background
	BackgroundCleanup bool
}

// DefaultCleanupConfig returns a reasonable default configuration
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		CleanupInterval:       15 * time.Minute,
		TempWorldMaxAge:       2 * time.Hour,
		PlayerSessionMaxAge:   24 * time.Hour,
		OrphanedDataMaxAge:    7 * 24 * time.Hour,
		BatchSize:             50,
		BackgroundCleanup:     true,
	}
}

// CleanupStats tracks cleanup operation statistics
type CleanupStats struct {
	LastRun           time.Time `json:"last_run"`
	TotalRuns         int64     `json:"total_runs"`
	TempWorldsCleaned int64     `json:"temp_worlds_cleaned"`
	OrphanedDataCleaned int64   `json:"orphaned_data_cleaned"`
	TotalTimeSpent    time.Duration `json:"total_time_spent"`
	AverageRunTime    time.Duration `json:"average_run_time"`
}

// CleanupManager handles efficient cleanup of temporary game data
type CleanupManager struct {
	config      *CleanupConfig
	storageMgr  *storage.Manager
	cache       *PlayerPropertyCache
	stats       CleanupStats
	statsLock   sync.RWMutex
	
	// Background cleanup control
	ticker      *time.Ticker
	stopChannel chan bool
	running     bool
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(config *CleanupConfig, storageMgr *storage.Manager, cache *PlayerPropertyCache) *CleanupManager {
	if config == nil {
		config = DefaultCleanupConfig()
	}

	manager := &CleanupManager{
		config:      config,
		storageMgr:  storageMgr,
		cache:       cache,
		stopChannel: make(chan bool, 1),
	}

	if config.BackgroundCleanup {
		manager.StartBackgroundCleanup()
	}

	return manager
}

// StartBackgroundCleanup starts the background cleanup process
func (cm *CleanupManager) StartBackgroundCleanup() {
	if cm.running {
		return
	}

	cm.ticker = time.NewTicker(cm.config.CleanupInterval)
	cm.running = true

	go func() {
		logging.Info("Started background cleanup manager", 
			"interval", cm.config.CleanupInterval,
			"temp_world_max_age", cm.config.TempWorldMaxAge)

		for {
			select {
			case <-cm.ticker.C:
				if err := cm.RunCleanup(); err != nil {
					logging.Error("Background cleanup failed", "error", err)
				}
			case <-cm.stopChannel:
				cm.ticker.Stop()
				logging.Info("Background cleanup manager stopped")
				return
			}
		}
	}()
}

// StopBackgroundCleanup stops the background cleanup process
func (cm *CleanupManager) StopBackgroundCleanup() {
	if !cm.running {
		return
	}

	cm.running = false
	cm.stopChannel <- true
}

// RunCleanup performs a full cleanup operation
func (cm *CleanupManager) RunCleanup() error {
	start := time.Now()
	logging.Info("Starting cleanup operation")

	var totalCleaned int64

	// Clean up temporary worlds
	tempWorldsCleaned, err := cm.cleanupTemporaryWorlds()
	if err != nil {
		logging.Error("Failed to cleanup temporary worlds", "error", err)
	} else {
		totalCleaned += tempWorldsCleaned
	}

	// Clean up orphaned data
	orphanedCleaned, err := cm.cleanupOrphanedData()
	if err != nil {
		logging.Error("Failed to cleanup orphaned data", "error", err)
	} else {
		totalCleaned += orphanedCleaned
	}

	// Clean up cache if provided
	if cm.cache != nil {
		cm.cache.cleanup()
	}

	// Update statistics
	duration := time.Since(start)
	cm.updateStats(tempWorldsCleaned, orphanedCleaned, duration)

	logging.Info("Cleanup operation completed", 
		"duration", duration,
		"temp_worlds_cleaned", tempWorldsCleaned,
		"orphaned_data_cleaned", orphanedCleaned,
		"total_cleaned", totalCleaned)

	return nil
}

// cleanupTemporaryWorlds removes expired temporary maze worlds
func (cm *CleanupManager) cleanupTemporaryWorlds() (int64, error) {
	cutoffTime := time.Now().Add(-cm.config.TempWorldMaxAge)
	var cleaned int64

	// Get all worlds
	worldIDs, err := cm.storageMgr.Worlds().ListWorlds()
	if err != nil {
		return 0, err
	}

	// Load world objects
	worlds := make([]*storage.World, 0, len(worldIDs))
	for _, worldID := range worldIDs {
		world, err := cm.storageMgr.Worlds().LoadWorld(worldID)
		if err != nil {
			logging.Warn("Failed to load world during cleanup", "world_id", worldID, "error", err)
			continue
		}
		worlds = append(worlds, world)
	}

	batch := make([]*storage.World, 0, cm.config.BatchSize)

	for _, world := range worlds {
		// Check if it's a temporary world that should be cleaned
		if cm.shouldCleanupWorld(world, cutoffTime) {
			batch = append(batch, world)

			// Process batch when full
			if len(batch) >= cm.config.BatchSize {
				batchCleaned := cm.processTempWorldBatch(batch)
				cleaned += batchCleaned
				batch = batch[:0] // Reset batch
			}
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		batchCleaned := cm.processTempWorldBatch(batch)
		cleaned += batchCleaned
	}

	return cleaned, nil
}

// shouldCleanupWorld determines if a world should be cleaned up
func (cm *CleanupManager) shouldCleanupWorld(world *storage.World, cutoffTime time.Time) bool {
	// Check if it's a temporary world
	worldType, exists := world.GetProperty("world_type")
	if !exists || worldType != "temporary" {
		return false
	}

	// Check if it has expired
	if cleanupTime, exists := world.GetProperty("cleanup_at"); exists {
		if cleanupTimeStr, ok := cleanupTime.(string); ok {
			if parsedTime, err := time.Parse(time.RFC3339, cleanupTimeStr); err == nil {
				return time.Now().After(parsedTime)
			}
		}
	}

	// Fallback: check creation time
	return world.CreatedAt.Before(cutoffTime)
}

// processTempWorldBatch processes a batch of worlds for cleanup
func (cm *CleanupManager) processTempWorldBatch(worlds []*storage.World) int64 {
	var cleaned int64

	for _, world := range worlds {
		if err := cm.cleanupSingleWorld(world); err != nil {
			logging.Error("Failed to cleanup world", 
				"world_id", world.ID, 
				"error", err)
		} else {
			cleaned++
			logging.Debug("Cleaned up temporary world", 
				"world_id", world.ID,
				"created_at", world.CreatedAt)
		}
	}

	return cleaned
}

// cleanupSingleWorld removes a single world and all its associated data
func (cm *CleanupManager) cleanupSingleWorld(world *storage.World) error {
	// Remove all rooms in the world
	roomIDs, err := cm.storageMgr.Worlds().ListRooms(world.ID)
	if err != nil {
		return err
	}

	for _, roomID := range roomIDs {
		if err := cm.storageMgr.Worlds().DeleteRoom(world.ID, roomID); err != nil {
			logging.Warn("Failed to delete room during world cleanup", 
				"world_id", world.ID, 
				"room_id", roomID, 
				"error", err)
		}
	}

	// Remove the world itself
	return cm.storageMgr.Worlds().DeleteWorld(world.ID)
}

// cleanupOrphanedData removes orphaned data that no longer has valid references
func (cm *CleanupManager) cleanupOrphanedData() (int64, error) {
	// This is a placeholder for more complex orphaned data cleanup
	// In a real implementation, you would:
	// 1. Find inventory items that reference deleted players
	// 2. Find sessions that reference deleted players
	// 3. Find temporary data that has expired
	// 4. Clean up log files and temporary storage
	
	var cleaned int64

	// For now, we'll implement a simple check for expired player sessions
	// This would need to be expanded based on your specific data model
	
	logging.Debug("Orphaned data cleanup completed", "items_cleaned", cleaned)
	return cleaned, nil
}

// updateStats updates cleanup statistics
func (cm *CleanupManager) updateStats(tempWorldsCleaned, orphanedCleaned int64, duration time.Duration) {
	cm.statsLock.Lock()
	defer cm.statsLock.Unlock()

	cm.stats.LastRun = time.Now()
	cm.stats.TotalRuns++
	cm.stats.TempWorldsCleaned += tempWorldsCleaned
	cm.stats.OrphanedDataCleaned += orphanedCleaned
	cm.stats.TotalTimeSpent += duration

	// Calculate average run time
	if cm.stats.TotalRuns > 0 {
		cm.stats.AverageRunTime = cm.stats.TotalTimeSpent / time.Duration(cm.stats.TotalRuns)
	}
}

// GetStats returns current cleanup statistics
func (cm *CleanupManager) GetStats() CleanupStats {
	cm.statsLock.RLock()
	defer cm.statsLock.RUnlock()
	return cm.stats
}

// Shutdown stops the cleanup manager and performs final cleanup
func (cm *CleanupManager) Shutdown() {
	logging.Info("Shutting down cleanup manager")
	
	cm.StopBackgroundCleanup()
	
	// Perform final cleanup
	if err := cm.RunCleanup(); err != nil {
		logging.Error("Final cleanup failed", "error", err)
	}
	
	logging.Info("Cleanup manager shutdown complete")
}

// ForceCleanupWorld immediately cleans up a specific world
func (cm *CleanupManager) ForceCleanupWorld(worldID string) error {
	world, err := cm.storageMgr.Worlds().LoadWorld(worldID)
	if err != nil {
		return err
	}

	return cm.cleanupSingleWorld(world)
}

// CleanupExpiredPlayerSessions removes old player session data
func (cm *CleanupManager) CleanupExpiredPlayerSessions() (int64, error) {
	cutoffTime := time.Now().Add(-cm.config.PlayerSessionMaxAge)
	var cleaned int64

	// Get all players
	playerIDs, err := cm.storageMgr.Players().ListAll()
	if err != nil {
		return 0, err
	}

	// Load player objects
	players := make([]*storage.Player, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		player, err := cm.storageMgr.Players().Load(playerID)
		if err != nil {
			logging.Warn("Failed to load player during cleanup", "player_id", playerID, "error", err)
			continue
		}
		players = append(players, player)
	}

	for _, player := range players {
		// Check if player session has expired
		if player.LastLogin.Before(cutoffTime) {
			// Clear cached data for inactive players
			if cm.cache != nil {
				cm.cache.InvalidatePlayer(player.ID)
			}
			
			// You could also clean up other session-related data here
			cleaned++
		}
	}

	return cleaned, nil
}

// GetCleanupSchedule returns information about the next scheduled cleanup
func (cm *CleanupManager) GetCleanupSchedule() CleanupScheduleInfo {
	cm.statsLock.RLock()
	defer cm.statsLock.RUnlock()

	var nextRun time.Time
	if cm.running && !cm.stats.LastRun.IsZero() {
		nextRun = cm.stats.LastRun.Add(cm.config.CleanupInterval)
	}

	return CleanupScheduleInfo{
		IsRunning:    cm.running,
		NextRun:      nextRun,
		LastRun:      cm.stats.LastRun,
		Interval:     cm.config.CleanupInterval,
		TotalRuns:    cm.stats.TotalRuns,
	}
}

// CleanupScheduleInfo provides information about cleanup scheduling
type CleanupScheduleInfo struct {
	IsRunning bool          `json:"is_running"`
	NextRun   time.Time     `json:"next_run"`
	LastRun   time.Time     `json:"last_run"`
	Interval  time.Duration `json:"interval"`
	TotalRuns int64         `json:"total_runs"`
}