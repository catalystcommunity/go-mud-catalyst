package maze

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

func setupTestStorage(t *testing.T) (*storage.Manager, func()) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wumpus_maze_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create storage manager
	config := storage.DefaultStorageConfig()
	config.DataRoot = tempDir
	storageMgr := storage.NewManagerWithConfig(config)

	if err := storageMgr.Initialize(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	cleanup := func() {
		// Storage manager doesn't have a Close method
		os.RemoveAll(tempDir)
	}

	return storageMgr, cleanup
}

func TestDefaultMazeConfig(t *testing.T) {
	config := DefaultMazeConfig()
	
	if config.RoomCount != 12 {
		t.Errorf("Expected RoomCount = 12, got %d", config.RoomCount)
	}
	
	if config.MinRooms != 8 {
		t.Errorf("Expected MinRooms = 8, got %d", config.MinRooms)
	}
	
	if config.MaxRooms != 20 {
		t.Errorf("Expected MaxRooms = 20, got %d", config.MaxRooms)
	}
	
	if config.CleanupTime != 2*time.Hour {
		t.Errorf("Expected CleanupTime = 2h, got %v", config.CleanupTime)
	}
}

func TestGenerateMaze(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	config := DefaultMazeConfig()
	config.RoomCount = 10 // Use smaller maze for testing

	// Generate maze
	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Verify basic properties
	if instance == nil {
		t.Fatal("Generated maze instance is nil")
	}

	if instance.World == nil {
		t.Fatal("Maze world is nil")
	}

	if len(instance.Rooms) != config.RoomCount {
		t.Errorf("Expected %d rooms, got %d", config.RoomCount, len(instance.Rooms))
	}

	if instance.StartRoom == nil {
		t.Fatal("Start room is nil")
	}

	if instance.WumpusRoom == nil {
		t.Fatal("Wumpus room is nil")
	}

	// Verify start room
	if !instance.StartRoom.IsStart {
		t.Error("Start room is not marked as start")
	}

	// Verify wumpus room
	if !instance.WumpusRoom.HasWumpus {
		t.Error("Wumpus room does not have wumpus")
	}

	// Verify wumpus is not in start room
	if instance.StartRoom.HasWumpus {
		t.Error("Wumpus should not be in start room")
	}

	// Verify world properties
	worldData := world.GetWumpusWorldData(instance.World)
	if worldData.WorldType != world.WumpusWorldTypeTemporary {
		t.Error("World should be temporary")
	}

	if worldData.InstanceID != instance.Config.InstanceID {
		t.Error("World instance ID does not match config")
	}

	if worldData.CreatedBy != playerID {
		t.Error("World created by does not match player ID")
	}
}

func TestGenerateMazeRoomConnections(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	config := DefaultMazeConfig()
	config.RoomCount = 8 // Use smaller maze for testing

	// Generate maze
	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Verify all rooms have at least one connection
	for i, room := range instance.Rooms {
		if len(room.Room.Exits) == 0 {
			t.Errorf("Room %d has no exits", i)
		}
	}

	// Verify connectivity by checking if all rooms are reachable from start
	visited := make(map[string]bool)
	visitRoom(instance.StartRoom.Room, instance.World.ID, storageMgr, visited)

	if len(visited) != len(instance.Rooms) {
		t.Errorf("Not all rooms are reachable from start. Visited: %d, Total: %d", len(visited), len(instance.Rooms))
	}
}

func TestGenerateMazeWithCustomConfig(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	config := MazeConfig{
		RoomCount:   15,
		MinRooms:    10,
		MaxRooms:    20,
		InstanceID:  "custom_maze_123",
		CreatedBy:   "custom_player",
		CleanupTime: 1 * time.Hour,
	}

	// Generate maze
	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Verify custom configuration
	if len(instance.Rooms) != config.RoomCount {
		t.Errorf("Expected %d rooms, got %d", config.RoomCount, len(instance.Rooms))
	}

	if instance.Config.InstanceID != config.InstanceID {
		t.Error("Instance ID does not match custom config")
	}

	if instance.Config.CreatedBy != config.CreatedBy {
		t.Error("Created by does not match custom config")
	}
}

func TestGenerateMazeBoundaryConditions(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"

	// Test minimum rooms
	config := DefaultMazeConfig()
	config.RoomCount = 5 // Below minimum
	config.MinRooms = 8

	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	if len(instance.Rooms) != config.MinRooms {
		t.Errorf("Expected room count to be adjusted to minimum %d, got %d", config.MinRooms, len(instance.Rooms))
	}

	// Test maximum rooms
	config.RoomCount = 25 // Above maximum
	config.MaxRooms = 20
	config.InstanceID = "test_max_rooms"

	instance, err = GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	if len(instance.Rooms) != config.MaxRooms {
		t.Errorf("Expected room count to be adjusted to maximum %d, got %d", config.MaxRooms, len(instance.Rooms))
	}
}

func TestGenerateMazePitPlacement(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	config := DefaultMazeConfig()
	config.RoomCount = 12

	// Generate maze
	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Count pits
	pitCount := 0
	breezeCount := 0
	for _, room := range instance.Rooms {
		if room.HasPit {
			pitCount++
			// Verify pit is not in start room
			if room.IsStart {
				t.Error("Pit should not be in start room")
			}
			// Verify pit is not in wumpus room
			if room.HasWumpus {
				t.Error("Pit should not be in wumpus room")
			}
		}
		if room.HasBreeze {
			breezeCount++
		}
	}

	// Should have at least one pit
	if pitCount < 1 {
		t.Error("Maze should have at least one pit")
	}

	// Should have breezes near pits
	if breezeCount < 1 {
		t.Error("Maze should have at least one breeze near pits")
	}
}

func TestGenerateMazeRoomProperties(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	config := DefaultMazeConfig()
	config.RoomCount = 10

	// Generate maze
	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Verify all rooms have proper properties
	for _, room := range instance.Rooms {
		roomData := world.GetWumpusRoomData(room.Room)
		
		// Check room type
		if roomData.RoomType != world.WumpusRoomTypeMaze {
			t.Errorf("Room %s should be maze type", room.Room.ID)
		}

		// Check instance ID
		if roomData.InstanceID != instance.Config.InstanceID {
			t.Errorf("Room %s instance ID does not match", room.Room.ID)
		}

		// Check cleanup time
		if roomData.TempCleanup == "" {
			t.Errorf("Room %s should have cleanup time set", room.Room.ID)
		}

		// Check maze position
		if roomData.MazePosition == nil {
			t.Errorf("Room %s should have maze position", room.Room.ID)
		}

		// Verify wumpus room properties
		if room.HasWumpus {
			if !roomData.HasWumpus {
				t.Error("Wumpus room properties not set correctly")
			}
		}

		// Verify pit room properties
		if room.HasPit {
			if !roomData.HasPit {
				t.Error("Pit room properties not set correctly")
			}
		}

		// Verify breeze room properties
		if room.HasBreeze {
			if !roomData.HasBreeze {
				t.Error("Breeze room properties not set correctly")
			}
		}
	}
}

func TestGetMazeInstance(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	config := DefaultMazeConfig()
	config.InstanceID = "test_get_instance"

	// Generate maze
	originalInstance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Retrieve maze instance
	retrievedInstance, err := GetMazeInstance(storageMgr, config.InstanceID)
	if err != nil {
		t.Fatalf("Failed to get maze instance: %v", err)
	}

	// Verify retrieved instance
	if retrievedInstance == nil {
		t.Fatal("Retrieved instance is nil")
	}

	if retrievedInstance.World.ID != originalInstance.World.ID {
		t.Error("Retrieved instance world ID does not match original")
	}

	if len(retrievedInstance.Rooms) != len(originalInstance.Rooms) {
		t.Errorf("Retrieved instance has %d rooms, expected %d", len(retrievedInstance.Rooms), len(originalInstance.Rooms))
	}

	// Test non-existent instance
	_, err = GetMazeInstance(storageMgr, "non_existent_instance")
	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestCleanupExpiredMazes(t *testing.T) {
	storageMgr, cleanup := setupTestStorage(t)
	defer cleanup()

	playerID := "test_player"
	
	// Create maze with very short cleanup time
	config := DefaultMazeConfig()
	config.CleanupTime = 1 * time.Millisecond
	config.InstanceID = "test_cleanup"

	// Generate maze
	instance, err := GenerateMaze(storageMgr, playerID, config)
	if err != nil {
		t.Fatalf("Failed to generate maze: %v", err)
	}

	// Verify maze exists
	_, err = GetMazeInstance(storageMgr, config.InstanceID)
	if err != nil {
		t.Fatalf("Maze should exist before cleanup: %v", err)
	}

	// Wait for cleanup time to pass
	time.Sleep(10 * time.Millisecond)

	// Run cleanup
	err = CleanupExpiredMazes(storageMgr)
	if err != nil {
		t.Fatalf("Failed to cleanup expired mazes: %v", err)
	}

	// Verify maze is gone
	_, err = GetMazeInstance(storageMgr, config.InstanceID)
	if err == nil {
		t.Error("Maze should be deleted after cleanup")
	}

	// Verify world is actually deleted
	worlds, err := storageMgr.Worlds().GetActiveWorlds()
	if err != nil {
		t.Fatalf("Failed to get active worlds: %v", err)
	}

	for _, world := range worlds {
		if world.ID == instance.World.ID {
			t.Error("World should be deleted after cleanup")
		}
	}
}

func TestGenerateRandomPositions(t *testing.T) {
	count := 10
	positions := generateRandomPositions(count)

	if len(positions) != count {
		t.Errorf("Expected %d positions, got %d", count, len(positions))
	}

	// Verify all positions are unique
	seen := make(map[string]bool)
	for _, pos := range positions {
		key := fmt.Sprintf("%d,%d", pos.X, pos.Y)
		if seen[key] {
			t.Errorf("Duplicate position found: %v", pos)
		}
		seen[key] = true
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test getOppositeDirection
	tests := map[string]string{
		"north":     "south",
		"south":     "north",
		"east":      "west",
		"west":      "east",
		"northeast": "southwest",
		"southwest": "northeast",
		"northwest": "southeast",
		"southeast": "northwest",
	}

	for direction, expected := range tests {
		result := getOppositeDirection(direction)
		if result != expected {
			t.Errorf("getOppositeDirection(%s) = %s, expected %s", direction, result, expected)
		}
	}

	// Test calculateDistance
	pos1 := world.Position{X: 0, Y: 0}
	pos2 := world.Position{X: 3, Y: 4}
	distance := calculateDistance(pos1, pos2)
	if distance != 7 { // Manhattan distance
		t.Errorf("calculateDistance returned %d, expected 7", distance)
	}

	// Test contains
	if !contains("hello world", "world") {
		t.Error("contains should return true for 'hello world' contains 'world'")
	}

	if contains("hello", "world") {
		t.Error("contains should return false for 'hello' contains 'world'")
	}
}

// Helper function to visit rooms recursively for connectivity testing
func visitRoom(room *storage.Room, worldID string, storageMgr *storage.Manager, visited map[string]bool) {
	if visited[room.ID] {
		return
	}
	
	visited[room.ID] = true
	
	for _, exit := range room.Exits {
		if exit.TargetWorldID == worldID {
			targetRoom, err := storageMgr.Worlds().LoadRoom(worldID, exit.TargetRoomID)
			if err == nil {
				visitRoom(targetRoom, worldID, storageMgr, visited)
			}
		}
	}
}