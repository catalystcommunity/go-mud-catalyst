package instance

import (
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/maze"
)

func TestNewInstanceManager(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	if manager == nil {
		t.Fatal("NewInstanceManager returned nil")
	}

	if manager.storageMgr != storageMgr {
		t.Error("Storage manager not set correctly")
	}

	if manager.instances == nil {
		t.Error("Instances map not initialized")
	}

	if manager.cleanupTicker == nil {
		t.Error("Cleanup ticker not initialized")
	}

	// Clean up
	manager.Shutdown()
}

func TestCreateInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()
	config.CleanupTime = 1 * time.Hour

	// Create instance
	instance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	if instance == nil {
		t.Fatal("CreateInstance returned nil instance")
	}

	if instance.Config.CreatedBy != playerID {
		t.Errorf("Instance created by wrong player: expected %s, got %s", playerID, instance.Config.CreatedBy)
	}

	// Check that instance is tracked
	if !manager.HasPlayerInstance(playerID) {
		t.Error("Player instance not tracked")
	}
}

func TestCreateInstanceDuplicate(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create first instance
	_, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("First CreateInstance failed: %v", err)
	}

	// Try to create second instance for same player
	_, err = manager.CreateInstance(playerID, config)
	if err == nil {
		t.Error("Expected error when creating duplicate instance")
	}
}

func TestGetInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create instance
	originalInstance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Get instance
	retrievedInstance, err := manager.GetInstance(originalInstance.Config.InstanceID)
	if err != nil {
		t.Fatalf("GetInstance failed: %v", err)
	}

	if retrievedInstance.Config.InstanceID != originalInstance.Config.InstanceID {
		t.Errorf("Instance ID mismatch: expected %s, got %s", 
			originalInstance.Config.InstanceID, retrievedInstance.Config.InstanceID)
	}
}

func TestGetInstanceNotFound(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	// Try to get non-existent instance
	_, err := manager.GetInstance("non_existent_instance")
	if err == nil {
		t.Error("Expected error when getting non-existent instance")
	}
}

func TestGetPlayerInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create instance
	originalInstance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Get player instance
	playerInstance, err := manager.GetPlayerInstance(playerID)
	if err != nil {
		t.Fatalf("GetPlayerInstance failed: %v", err)
	}

	if playerInstance.Config.InstanceID != originalInstance.Config.InstanceID {
		t.Errorf("Player instance ID mismatch: expected %s, got %s", 
			originalInstance.Config.InstanceID, playerInstance.Config.InstanceID)
	}
}

func TestHasPlayerInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Check before creating instance
	if manager.HasPlayerInstance(playerID) {
		t.Error("Player should not have instance before creating")
	}

	// Create instance
	_, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Check after creating instance
	if !manager.HasPlayerInstance(playerID) {
		t.Error("Player should have instance after creating")
	}
}

func TestCleanupInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create instance
	instance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Cleanup instance
	err = manager.CleanupInstance(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("CleanupInstance failed: %v", err)
	}

	// Check that instance is no longer tracked
	if manager.HasPlayerInstance(playerID) {
		t.Error("Player instance should not exist after cleanup")
	}
}

func TestCleanupPlayerInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create instance
	_, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Cleanup player instance
	err = manager.CleanupPlayerInstance(playerID)
	if err != nil {
		t.Fatalf("CleanupPlayerInstance failed: %v", err)
	}

	// Check that instance is no longer tracked
	if manager.HasPlayerInstance(playerID) {
		t.Error("Player instance should not exist after cleanup")
	}
}

func TestGetInstanceInfo(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create instance
	instance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Get instance info
	info, err := manager.GetInstanceInfo(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("GetInstanceInfo failed: %v", err)
	}

	if info.InstanceID != instance.Config.InstanceID {
		t.Errorf("Instance ID mismatch: expected %s, got %s", 
			instance.Config.InstanceID, info.InstanceID)
	}

	if info.PlayerID != playerID {
		t.Errorf("Player ID mismatch: expected %s, got %s", playerID, info.PlayerID)
	}

	if !info.IsActive {
		t.Error("Instance should be active")
	}
}

func TestListActiveInstances(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	// Check empty list
	instances := manager.ListActiveInstances()
	if len(instances) != 0 {
		t.Errorf("Expected 0 active instances, got %d", len(instances))
	}

	// Create instances
	config := maze.DefaultMazeConfig()
	_, err := manager.CreateInstance("player1", config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	_, err = manager.CreateInstance("player2", config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Check active instances
	instances = manager.ListActiveInstances()
	if len(instances) != 2 {
		t.Errorf("Expected 2 active instances, got %d", len(instances))
	}

	// Check that all instances are active
	for _, instance := range instances {
		if !instance.IsActive {
			t.Error("All listed instances should be active")
		}
	}
}

func TestExtendInstanceLifetime(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()
	config.CleanupTime = 1 * time.Hour

	// Create instance
	instance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Get initial expiration time
	initialInfo, err := manager.GetInstanceInfo(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("GetInstanceInfo failed: %v", err)
	}

	// Extend lifetime
	extension := 30 * time.Minute
	err = manager.ExtendInstanceLifetime(instance.Config.InstanceID, extension)
	if err != nil {
		t.Fatalf("ExtendInstanceLifetime failed: %v", err)
	}

	// Check that expiration time was extended
	extendedInfo, err := manager.GetInstanceInfo(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("GetInstanceInfo failed: %v", err)
	}

	expectedExpiration := initialInfo.ExpiresAt.Add(extension)
	if !extendedInfo.ExpiresAt.Equal(expectedExpiration) {
		t.Errorf("Expiration time not extended correctly: expected %v, got %v", 
			expectedExpiration, extendedInfo.ExpiresAt)
	}
}

func TestUpdatePlayerCount(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Create instance
	instance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Update player count
	newCount := 3
	err = manager.UpdatePlayerCount(instance.Config.InstanceID, newCount)
	if err != nil {
		t.Fatalf("UpdatePlayerCount failed: %v", err)
	}

	// Check that player count was updated
	info, err := manager.GetInstanceInfo(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("GetInstanceInfo failed: %v", err)
	}

	if info.PlayerCount != newCount {
		t.Errorf("Player count not updated: expected %d, got %d", newCount, info.PlayerCount)
	}
}

func TestGetInstanceStats(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	// Check empty stats
	stats := manager.GetInstanceStats()
	if stats["total_instances"] != 0 {
		t.Errorf("Expected 0 total instances, got %v", stats["total_instances"])
	}

	// Create instances
	config := maze.DefaultMazeConfig()
	_, err := manager.CreateInstance("player1", config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	_, err = manager.CreateInstance("player2", config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Check stats
	stats = manager.GetInstanceStats()
	if stats["total_instances"] != 2 {
		t.Errorf("Expected 2 total instances, got %v", stats["total_instances"])
	}
	if stats["active_instances"] != 2 {
		t.Errorf("Expected 2 active instances, got %v", stats["active_instances"])
	}
}

func TestValidateInstance(t *testing.T) {
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	manager := NewInstanceManager(storageMgr)
	defer manager.Shutdown()

	playerID := "test_player"
	config := maze.DefaultMazeConfig()

	// Validate non-existent instance
	err := manager.ValidateInstance("non_existent")
	if err == nil {
		t.Error("Expected error when validating non-existent instance")
	}

	// Create instance
	instance, err := manager.CreateInstance(playerID, config)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}

	// Validate existing instance
	err = manager.ValidateInstance(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("ValidateInstance failed: %v", err)
	}

	// Cleanup instance and validate again
	err = manager.CleanupInstance(instance.Config.InstanceID)
	if err != nil {
		t.Fatalf("CleanupInstance failed: %v", err)
	}

	err = manager.ValidateInstance(instance.Config.InstanceID)
	if err == nil {
		t.Error("Expected error when validating cleaned up instance")
	}
}