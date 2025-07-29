package instance

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/maze"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

// InstanceManager manages temporary maze instances
type InstanceManager struct {
	storageMgr *storage.Manager
	instances  map[string]*InstanceInfo
	mu         sync.RWMutex
	cleanupTicker *time.Ticker
	stopChan   chan struct{}
}

// InstanceInfo represents information about a maze instance
type InstanceInfo struct {
	InstanceID  string
	WorldID     string
	PlayerID    string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	IsActive    bool
	PlayerCount int
}

// NewInstanceManager creates a new instance manager
func NewInstanceManager(storageMgr *storage.Manager) *InstanceManager {
	manager := &InstanceManager{
		storageMgr: storageMgr,
		instances:  make(map[string]*InstanceInfo),
		stopChan:   make(chan struct{}),
	}
	
	// Start cleanup routine - runs every 5 minutes
	manager.cleanupTicker = time.NewTicker(5 * time.Minute)
	go manager.cleanupRoutine()
	
	return manager
}

// CreateInstance creates a new maze instance for a player
func (im *InstanceManager) CreateInstance(playerID string, config maze.MazeConfig) (*maze.MazeInstance, error) {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	// Check if player already has an active instance
	if existingInstance := im.getPlayerInstance(playerID); existingInstance != nil {
		return nil, fmt.Errorf("player %s already has an active instance: %s", playerID, existingInstance.InstanceID)
	}
	
	// Generate maze instance
	mazeInstance, err := maze.GenerateMaze(im.storageMgr, playerID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate maze: %w", err)
	}
	
	// Create instance info
	instanceInfo := &InstanceInfo{
		InstanceID:  mazeInstance.Config.InstanceID,
		WorldID:     mazeInstance.World.ID,
		PlayerID:    playerID,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(mazeInstance.Config.CleanupTime),
		IsActive:    true,
		PlayerCount: 1,
	}
	
	// Store instance info
	im.instances[instanceInfo.InstanceID] = instanceInfo
	
	return mazeInstance, nil
}

// GetInstance retrieves a maze instance by ID
func (im *InstanceManager) GetInstance(instanceID string) (*maze.MazeInstance, error) {
	im.mu.RLock()
	instanceInfo, exists := im.instances[instanceID]
	im.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}
	
	if !instanceInfo.IsActive {
		return nil, fmt.Errorf("instance %s is not active", instanceID)
	}
	
	// Load maze instance from storage
	mazeInstance, err := maze.GetMazeInstance(im.storageMgr, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load maze instance: %w", err)
	}
	
	return mazeInstance, nil
}

// GetPlayerInstance returns the active instance for a player
func (im *InstanceManager) GetPlayerInstance(playerID string) (*maze.MazeInstance, error) {
	im.mu.RLock()
	instanceInfo := im.getPlayerInstance(playerID)
	im.mu.RUnlock()
	
	if instanceInfo == nil {
		return nil, fmt.Errorf("player %s has no active instance", playerID)
	}
	
	return im.GetInstance(instanceInfo.InstanceID)
}

// getPlayerInstance finds an active instance for a player (must be called with lock held)
func (im *InstanceManager) getPlayerInstance(playerID string) *InstanceInfo {
	for _, instance := range im.instances {
		if instance.PlayerID == playerID && instance.IsActive {
			return instance
		}
	}
	return nil
}

// HasPlayerInstance checks if a player has an active instance
func (im *InstanceManager) HasPlayerInstance(playerID string) bool {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	return im.getPlayerInstance(playerID) != nil
}

// CleanupInstance marks an instance for cleanup and removes it
func (im *InstanceManager) CleanupInstance(instanceID string) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	instanceInfo, exists := im.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}
	
	// Mark as inactive
	instanceInfo.IsActive = false
	
	// Delete the maze world from storage
	if err := im.storageMgr.Worlds().DeleteWorld(instanceInfo.WorldID); err != nil {
		return fmt.Errorf("failed to delete world %s: %w", instanceInfo.WorldID, err)
	}
	
	// Remove from instances map
	delete(im.instances, instanceID)
	
	return nil
}

// CleanupPlayerInstance cleans up the active instance for a player
func (im *InstanceManager) CleanupPlayerInstance(playerID string) error {
	im.mu.RLock()
	instanceInfo := im.getPlayerInstance(playerID)
	im.mu.RUnlock()
	
	if instanceInfo == nil {
		return fmt.Errorf("player %s has no active instance to cleanup", playerID)
	}
	
	return im.CleanupInstance(instanceInfo.InstanceID)
}

// GetInstanceInfo returns information about an instance
func (im *InstanceManager) GetInstanceInfo(instanceID string) (*InstanceInfo, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	instanceInfo, exists := im.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}
	
	// Return a copy to prevent external modification
	return &InstanceInfo{
		InstanceID:  instanceInfo.InstanceID,
		WorldID:     instanceInfo.WorldID,
		PlayerID:    instanceInfo.PlayerID,
		CreatedAt:   instanceInfo.CreatedAt,
		ExpiresAt:   instanceInfo.ExpiresAt,
		IsActive:    instanceInfo.IsActive,
		PlayerCount: instanceInfo.PlayerCount,
	}, nil
}

// ListActiveInstances returns all active instances
func (im *InstanceManager) ListActiveInstances() []*InstanceInfo {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	var activeInstances []*InstanceInfo
	for _, instance := range im.instances {
		if instance.IsActive {
			// Return a copy to prevent external modification
			activeInstances = append(activeInstances, &InstanceInfo{
				InstanceID:  instance.InstanceID,
				WorldID:     instance.WorldID,
				PlayerID:    instance.PlayerID,
				CreatedAt:   instance.CreatedAt,
				ExpiresAt:   instance.ExpiresAt,
				IsActive:    instance.IsActive,
				PlayerCount: instance.PlayerCount,
			})
		}
	}
	
	return activeInstances
}

// ExtendInstanceLifetime extends the lifetime of an instance
func (im *InstanceManager) ExtendInstanceLifetime(instanceID string, duration time.Duration) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	instanceInfo, exists := im.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}
	
	if !instanceInfo.IsActive {
		return fmt.Errorf("instance %s is not active", instanceID)
	}
	
	// Extend expiration time
	instanceInfo.ExpiresAt = instanceInfo.ExpiresAt.Add(duration)
	
	// Update world properties with new cleanup time
	mazeWorld, err := im.storageMgr.Worlds().LoadWorld(instanceInfo.WorldID)
	if err != nil {
		return fmt.Errorf("failed to load world: %w", err)
	}
	
	worldData := world.GetWumpusWorldData(mazeWorld)
	worldData.CleanupAt = instanceInfo.ExpiresAt.Format(time.RFC3339)
	world.SetWumpusWorldData(mazeWorld, worldData)
	
	if err := im.storageMgr.Worlds().SaveWorld(mazeWorld); err != nil {
		return fmt.Errorf("failed to save world: %w", err)
	}
	
	return nil
}

// UpdatePlayerCount updates the player count for an instance
func (im *InstanceManager) UpdatePlayerCount(instanceID string, playerCount int) error {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	instanceInfo, exists := im.instances[instanceID]
	if !exists {
		return fmt.Errorf("instance %s not found", instanceID)
	}
	
	instanceInfo.PlayerCount = playerCount
	return nil
}

// GetInstanceStats returns statistics about all instances
func (im *InstanceManager) GetInstanceStats() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()
	
	stats := make(map[string]interface{})
	
	totalInstances := len(im.instances)
	activeInstances := 0
	totalPlayers := 0
	
	for _, instance := range im.instances {
		if instance.IsActive {
			activeInstances++
			totalPlayers += instance.PlayerCount
		}
	}
	
	stats["total_instances"] = totalInstances
	stats["active_instances"] = activeInstances
	stats["total_players"] = totalPlayers
	stats["average_players_per_instance"] = float64(totalPlayers) / float64(max(activeInstances, 1))
	
	return stats
}

// cleanupRoutine runs periodically to clean up expired instances
func (im *InstanceManager) cleanupRoutine() {
	for {
		select {
		case <-im.cleanupTicker.C:
			im.cleanupExpiredInstances()
		case <-im.stopChan:
			return
		}
	}
}

// cleanupExpiredInstances removes expired instances
func (im *InstanceManager) cleanupExpiredInstances() {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	now := time.Now()
	var expiredInstances []string
	
	// Find expired instances
	for instanceID, instance := range im.instances {
		if instance.IsActive && now.After(instance.ExpiresAt) {
			expiredInstances = append(expiredInstances, instanceID)
		}
	}
	
	// Clean up expired instances
	for _, instanceID := range expiredInstances {
		instanceInfo := im.instances[instanceID]
		
		// Mark as inactive
		instanceInfo.IsActive = false
		
		// Delete the maze world from storage
		if err := im.storageMgr.Worlds().DeleteWorld(instanceInfo.WorldID); err != nil {
			// Log error but continue cleanup
			fmt.Printf("Failed to delete expired world %s: %v\n", instanceInfo.WorldID, err)
		}
		
		// Remove from instances map
		delete(im.instances, instanceID)
	}
	
	if len(expiredInstances) > 0 {
		fmt.Printf("Cleaned up %d expired instances\n", len(expiredInstances))
	}
}

// Shutdown stops the instance manager and cleanup routine
func (im *InstanceManager) Shutdown() error {
	// Stop cleanup routine
	close(im.stopChan)
	if im.cleanupTicker != nil {
		im.cleanupTicker.Stop()
	}
	
	// Clean up all active instances
	im.mu.Lock()
	defer im.mu.Unlock()
	
	var errors []error
	for _, instance := range im.instances {
		if instance.IsActive {
			if err := im.storageMgr.Worlds().DeleteWorld(instance.WorldID); err != nil {
				errors = append(errors, fmt.Errorf("failed to delete world %s: %w", instance.WorldID, err))
			}
		}
	}
	
	// Clear instances map
	im.instances = make(map[string]*InstanceInfo)
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors", len(errors))
	}
	
	return nil
}

// LoadInstancesFromStorage loads existing instances from storage on startup
func (im *InstanceManager) LoadInstancesFromStorage() error {
	im.mu.Lock()
	defer im.mu.Unlock()
	
	// Get all active worlds
	worlds, err := im.storageMgr.Worlds().GetActiveWorlds()
	if err != nil {
		return fmt.Errorf("failed to get active worlds: %w", err)
	}
	
	// Find temporary maze worlds
	for _, mazeWorld := range worlds {
		worldData := world.GetWumpusWorldData(mazeWorld)
		if worldData.WorldType == world.WumpusWorldTypeTemporary && worldData.InstanceID != "" {
			// Parse cleanup time
			var expiresAt time.Time
			if worldData.CleanupAt != "" {
				if parsedTime, err := time.Parse(time.RFC3339, worldData.CleanupAt); err == nil {
					expiresAt = parsedTime
				} else {
					// Default to 2 hours from now if parsing fails
					expiresAt = time.Now().Add(2 * time.Hour)
				}
			} else {
				expiresAt = time.Now().Add(2 * time.Hour)
			}
			
			// Create instance info
			instanceInfo := &InstanceInfo{
				InstanceID:  worldData.InstanceID,
				WorldID:     mazeWorld.ID,
				PlayerID:    worldData.CreatedBy,
				CreatedAt:   time.Now(), // We don't have the original creation time
				ExpiresAt:   expiresAt,
				IsActive:    true,
				PlayerCount: 1, // Assume 1 player for now
			}
			
			// Check if instance is already expired
			if time.Now().After(expiresAt) {
				// Mark as expired and delete
				if err := im.storageMgr.Worlds().DeleteWorld(mazeWorld.ID); err != nil {
					fmt.Printf("Failed to delete expired world %s: %v\n", mazeWorld.ID, err)
				}
				continue
			}
			
			im.instances[instanceInfo.InstanceID] = instanceInfo
		}
	}
	
	fmt.Printf("Loaded %d instances from storage\n", len(im.instances))
	return nil
}

// Helper function to get max of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ValidateInstance checks if an instance is valid and accessible
func (im *InstanceManager) ValidateInstance(instanceID string) error {
	im.mu.RLock()
	instanceInfo, exists := im.instances[instanceID]
	im.mu.RUnlock()
	
	if !exists {
		return errors.New("instance not found")
	}
	
	if !instanceInfo.IsActive {
		return errors.New("instance is not active")
	}
	
	if time.Now().After(instanceInfo.ExpiresAt) {
		return errors.New("instance has expired")
	}
	
	// Check if world still exists in storage
	_, err := im.storageMgr.Worlds().LoadWorld(instanceInfo.WorldID)
	if err != nil {
		return fmt.Errorf("instance world not found in storage: %w", err)
	}
	
	return nil
}