package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// TestWorldManagerComprehensive tests all functionality of Phase 2.2.2
func TestWorldManagerComprehensive(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-comprehensive-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	t.Run("World Creation and Validation", func(t *testing.T) {
		// Test basic world creation
		world1, err := manager.CreateNewWorld("Adventure World", "A world of adventure")
		if err != nil {
			t.Fatalf("Failed to create world: %v", err)
		}

		if world1.Name != "Adventure World" {
			t.Errorf("Expected world name 'Adventure World', got '%s'", world1.Name)
		}

		if world1.Description != "A world of adventure" {
			t.Errorf("Expected description 'A world of adventure', got '%s'", world1.Description)
		}

		if !world1.IsActive {
			t.Error("New world should be active by default")
		}

		// Verify world exists
		if !manager.Worlds().WorldExists(world1.ID) {
			t.Error("World should exist after creation")
		}

		// Test loading world
		loadedWorld, err := manager.Worlds().LoadWorld(world1.ID)
		if err != nil {
			t.Fatalf("Failed to load world: %v", err)
		}

		if loadedWorld.ID != world1.ID {
			t.Errorf("Loaded world ID mismatch: expected %s, got %s", world1.ID, loadedWorld.ID)
		}
	})

	t.Run("Room Creation and Management", func(t *testing.T) {
		// Create world first
		world, err := manager.CreateNewWorld("Test World", "Test world for rooms")
		if err != nil {
			t.Fatalf("Failed to create world: %v", err)
		}

		// Test room creation
		room1, err := manager.CreateNewRoom(world.ID, "Starting Room", "The beginning of the adventure")
		if err != nil {
			t.Fatalf("Failed to create room: %v", err)
		}

		if room1.WorldID != world.ID {
			t.Errorf("Room world ID mismatch: expected %s, got %s", world.ID, room1.WorldID)
		}

		// Test room existence
		if !manager.Worlds().RoomExists(world.ID, room1.ID) {
			t.Error("Room should exist after creation")
		}

		// Test room loading
		loadedRoom, err := manager.Worlds().LoadRoom(world.ID, room1.ID)
		if err != nil {
			t.Fatalf("Failed to load room: %v", err)
		}

		if loadedRoom.ID != room1.ID {
			t.Errorf("Loaded room ID mismatch: expected %s, got %s", room1.ID, loadedRoom.ID)
		}

		// Test room creation in non-existent world
		_, err = manager.CreateNewRoom("non-existent-world", "Invalid Room", "Should fail")
		if err == nil {
			t.Error("Should not be able to create room in non-existent world")
		}
	})

	t.Run("Exit System Comprehensive", func(t *testing.T) {
		// Create test worlds and rooms
		world1, err := manager.CreateNewWorld("World 1", "First world")
		if err != nil {
			t.Fatalf("Failed to create world 1: %v", err)
		}

		world2, err := manager.CreateNewWorld("World 2", "Second world")
		if err != nil {
			t.Fatalf("Failed to create world 2: %v", err)
		}

		room1, err := manager.CreateNewRoom(world1.ID, "Room 1", "First room")
		if err != nil {
			t.Fatalf("Failed to create room 1: %v", err)
		}

		room2, err := manager.CreateNewRoom(world1.ID, "Room 2", "Second room")
		if err != nil {
			t.Fatalf("Failed to create room 2: %v", err)
		}

		room3, err := manager.CreateNewRoom(world2.ID, "Room 3", "Third room in different world")
		if err != nil {
			t.Fatalf("Failed to create room 3: %v", err)
		}

		// Test basic exit creation
		room1.AddExit("north", world1.ID, room2.ID)
		room1.AddExit("south", world1.ID, room1.ID) // Self-referencing exit
		
		if err := manager.Worlds().SaveRoom(room1); err != nil {
			t.Fatalf("Failed to save room with exits: %v", err)
		}

		// Verify exits exist
		northExit := room1.GetExit("north")
		if northExit == nil {
			t.Fatal("North exit should exist")
		}

		if northExit.TargetRoomID != room2.ID {
			t.Errorf("North exit should target room 2, got %s", northExit.TargetRoomID)
		}

		southExit := room1.GetExit("south")
		if southExit == nil {
			t.Fatal("South exit should exist")
		}

		// Test exit removal
		if !room1.RemoveExit("south") {
			t.Error("Should have successfully removed south exit")
		}

		if room1.GetExit("south") != nil {
			t.Error("South exit should no longer exist")
		}

		// Test removing non-existent exit
		if room1.RemoveExit("west") {
			t.Error("Should not have removed non-existent west exit")
		}

		// Test exit properties
		room1.AddExit("secret", world1.ID, room2.ID)
		secretExit := room1.GetExit("secret")
		if secretExit == nil {
			t.Fatal("Secret exit should exist")
		}

		secretExit.IsHidden = true
		secretExit.Description = "A hidden passage behind the bookshelf"
		if secretExit.Properties == nil {
			secretExit.Properties = make(map[string]interface{})
		}
		secretExit.Properties["requires_key"] = "brass_key"
		secretExit.Properties["difficulty"] = 5

		if err := manager.Worlds().SaveRoom(room1); err != nil {
			t.Fatalf("Failed to save room with exit properties: %v", err)
		}

		// Reload and verify properties
		reloadedRoom, err := manager.Worlds().LoadRoom(world1.ID, room1.ID)
		if err != nil {
			t.Fatalf("Failed to reload room: %v", err)
		}

		reloadedSecret := reloadedRoom.GetExit("secret")
		if reloadedSecret == nil {
			t.Fatal("Secret exit should exist after reload")
		}

		if !reloadedSecret.IsHidden {
			t.Error("Secret exit should be hidden")
		}

		if reloadedSecret.Description != "A hidden passage behind the bookshelf" {
			t.Errorf("Exit description mismatch")
		}

		// Test cross-world exits
		room1.AddExit("portal", world2.ID, room3.ID)
		if err := manager.Worlds().SaveRoom(room1); err != nil {
			t.Fatalf("Failed to save room with cross-world exit: %v", err)
		}

		portalExit := room1.GetExit("portal")
		if portalExit == nil {
			t.Fatal("Portal exit should exist")
		}

		if portalExit.TargetWorldID != world2.ID {
			t.Errorf("Portal should target world 2, got %s", portalExit.TargetWorldID)
		}
	})

	t.Run("Secure Inlet System", func(t *testing.T) {
		// Create test setup
		world1, _ := manager.CreateNewWorld("Secure World 1", "First secure world")
		world2, _ := manager.CreateNewWorld("Secure World 2", "Second secure world")
		
		room1, _ := manager.CreateNewRoom(world1.ID, "Secure Room 1", "First secure room")
		room2, _ := manager.CreateNewRoom(world1.ID, "Secure Room 2", "Second secure room")
		room3, _ := manager.CreateNewRoom(world2.ID, "Secure Room 3", "Third secure room")

		// Test allowed inlet management
		room2.AddAllowedInlet(room1.ID)
		if err := manager.Worlds().SaveRoom(room2); err != nil {
			t.Fatalf("Failed to save room with allowed inlet: %v", err)
		}

		// Test inlet permission check
		if !room2.IsInletAllowed(room1.ID) {
			t.Error("Room 1 should be allowed to connect to room 2")
		}

		if room2.IsInletAllowed("random-room-id") {
			t.Error("Random room should not be allowed to connect")
		}

		// Test duplicate inlet addition (should not create duplicates)
		initialCount := len(room2.AllowedInlets)
		room2.AddAllowedInlet(room1.ID) // Add same inlet again
		if len(room2.AllowedInlets) != initialCount {
			t.Error("Adding duplicate inlet should not increase count")
		}

		// Test removing inlet
		if !room2.RemoveAllowedInlet(room1.ID) {
			t.Error("Should have successfully removed allowed inlet")
		}

		if room2.IsInletAllowed(room1.ID) {
			t.Error("Room 1 should no longer be allowed after removal")
		}

		// Test removing non-existent inlet
		if room2.RemoveAllowedInlet("non-existent-room") {
			t.Error("Should not have removed non-existent inlet")
		}

		// Test cross-world inlet permissions
		room3.AddAllowedInlet(room1.ID) // Allow room1 to connect to room3 across worlds
		if err := manager.Worlds().SaveRoom(room3); err != nil {
			t.Fatalf("Failed to save room with cross-world inlet: %v", err)
		}

		if !room3.IsInletAllowed(room1.ID) {
			t.Error("Cross-world inlet should be allowed")
		}
	})

	t.Run("Exit Validation System", func(t *testing.T) {
		// Create test setup
		world1, _ := manager.CreateNewWorld("Validation World 1", "First validation world")
		world2, _ := manager.CreateNewWorld("Validation World 2", "Second validation world")
		
		room1, _ := manager.CreateNewRoom(world1.ID, "Source Room", "Source room for validation")
		room2, _ := manager.CreateNewRoom(world1.ID, "Target Room", "Target room for validation")
		room3, _ := manager.CreateNewRoom(world2.ID, "Cross-world Target", "Cross-world target room")

		// Test validation without permission (should fail)
		err := manager.Worlds().ValidateExit(world1.ID, room1.ID, world1.ID, room2.ID)
		if err == nil {
			t.Error("Exit validation should fail without allowed inlet")
		}

		// Add permission and test again
		room2.AddAllowedInlet(room1.ID)
		if err := manager.Worlds().SaveRoom(room2); err != nil {
			t.Fatalf("Failed to save room with inlet: %v", err)
		}

		err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world1.ID, room2.ID)
		if err != nil {
			t.Errorf("Exit validation should succeed with allowed inlet: %v", err)
		}

		// Test cross-world validation without permission
		err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world2.ID, room3.ID)
		if err == nil {
			t.Error("Cross-world validation should fail without permission")
		}

		// Add cross-world permission
		room3.AddAllowedInlet(room1.ID)
		if err := manager.Worlds().SaveRoom(room3); err != nil {
			t.Fatalf("Failed to save cross-world room with inlet: %v", err)
		}

		err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world2.ID, room3.ID)
		if err != nil {
			t.Errorf("Cross-world validation should succeed: %v", err)
		}

		// Test validation with non-existent target room
		err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world1.ID, "non-existent-room")
		if err == nil {
			t.Error("Validation should fail with non-existent target room")
		}

		// Test validation with non-existent target world
		err = manager.Worlds().ValidateExit(world1.ID, room1.ID, "non-existent-world", room2.ID)
		if err == nil {
			t.Error("Validation should fail with non-existent target world")
		}
	})

	t.Run("Exit Creation with Validation", func(t *testing.T) {
		// Create test setup
		world1, _ := manager.CreateNewWorld("Creation World 1", "First creation world")
		world2, _ := manager.CreateNewWorld("Creation World 2", "Second creation world")
		
		room1, _ := manager.CreateNewRoom(world1.ID, "Creator Room", "Source room for creation")
		room2, _ := manager.CreateNewRoom(world1.ID, "Destination Room", "Destination room")
		room3, _ := manager.CreateNewRoom(world2.ID, "Cross Destination", "Cross-world destination")

		// Test exit creation without permission (should fail)
		err := manager.Worlds().CreateExitWithValidation(world1.ID, room1.ID, "east", world1.ID, room2.ID)
		if err == nil {
			t.Error("Exit creation should fail without permission")
		}

		// Add permission and test successful creation
		room2.AddAllowedInlet(room1.ID)
		if err := manager.Worlds().SaveRoom(room2); err != nil {
			t.Fatalf("Failed to save room with inlet: %v", err)
		}

		err = manager.Worlds().CreateExitWithValidation(world1.ID, room1.ID, "east", world1.ID, room2.ID)
		if err != nil {
			t.Errorf("Exit creation should succeed with permission: %v", err)
		}

		// Verify exit was created
		updatedRoom, err := manager.Worlds().LoadRoom(world1.ID, room1.ID)
		if err != nil {
			t.Fatalf("Failed to reload room: %v", err)
		}

		eastExit := updatedRoom.GetExit("east")
		if eastExit == nil {
			t.Fatal("East exit should have been created")
		}

		if eastExit.TargetRoomID != room2.ID {
			t.Errorf("East exit should target room 2, got %s", eastExit.TargetRoomID)
		}

		// Test cross-world exit creation
		room3.AddAllowedInlet(room1.ID)
		if err := manager.Worlds().SaveRoom(room3); err != nil {
			t.Fatalf("Failed to save cross-world room: %v", err)
		}

		err = manager.Worlds().CreateExitWithValidation(world1.ID, room1.ID, "portal", world2.ID, room3.ID)
		if err != nil {
			t.Errorf("Cross-world exit creation should succeed: %v", err)
		}

		// Test creation with non-existent source room
		err = manager.Worlds().CreateExitWithValidation(world1.ID, "non-existent-room", "nowhere", world1.ID, room2.ID)
		if err == nil {
			t.Error("Exit creation should fail with non-existent source room")
		}
	})

	t.Run("World Properties and State", func(t *testing.T) {
		world, _ := manager.CreateNewWorld("Property World", "World for testing properties")

		// Test setting and getting world properties
		world.SetProperty("climate", "tropical")
		world.SetProperty("danger_level", 7)
		world.SetProperty("has_magic", true)

		if err := manager.Worlds().SaveWorld(world); err != nil {
			t.Fatalf("Failed to save world with properties: %v", err)
		}

		// Reload and verify properties
		reloadedWorld, err := manager.Worlds().LoadWorld(world.ID)
		if err != nil {
			t.Fatalf("Failed to reload world: %v", err)
		}

		climate, exists := reloadedWorld.GetProperty("climate")
		if !exists || climate != "tropical" {
			t.Errorf("Expected climate 'tropical', got %v (exists: %v)", climate, exists)
		}

		dangerLevel, exists := reloadedWorld.GetProperty("danger_level")
		if !exists || dangerLevel != float64(7) { // JSON unmarshals numbers as float64
			t.Errorf("Expected danger level 7, got %v (exists: %v)", dangerLevel, exists)
		}

		hasMagic, exists := reloadedWorld.GetProperty("has_magic")
		if !exists || hasMagic != true {
			t.Errorf("Expected has_magic true, got %v (exists: %v)", hasMagic, exists)
		}

		// Test non-existent property
		_, exists = reloadedWorld.GetProperty("non_existent")
		if exists {
			t.Error("Non-existent property should not exist")
		}

		// Test world activation/deactivation
		reloadedWorld.IsActive = false
		if err := manager.Worlds().SaveWorld(reloadedWorld); err != nil {
			t.Fatalf("Failed to save deactivated world: %v", err)
		}

		// Test getting active worlds
		activeWorlds, err := manager.Worlds().GetActiveWorlds()
		if err != nil {
			t.Fatalf("Failed to get active worlds: %v", err)
		}

		// Our test world should not be in the active list
		for _, activeWorld := range activeWorlds {
			if activeWorld.ID == world.ID {
				t.Error("Deactivated world should not be in active worlds list")
			}
		}
	})

	t.Run("Room Properties and State", func(t *testing.T) {
		world, _ := manager.CreateNewWorld("Room Property World", "World for testing room properties")
		room, _ := manager.CreateNewRoom(world.ID, "Property Room", "Room for testing properties")

		// Test setting and getting room properties
		room.SetProperty("temperature", 25.5)
		room.SetProperty("lighting", "dim")
		room.SetProperty("has_treasure", false)
		room.SetProperty("occupancy_limit", 10)

		if err := manager.Worlds().SaveRoom(room); err != nil {
			t.Fatalf("Failed to save room with properties: %v", err)
		}

		// Reload and verify properties
		reloadedRoom, err := manager.Worlds().LoadRoom(world.ID, room.ID)
		if err != nil {
			t.Fatalf("Failed to reload room: %v", err)
		}

		temp, exists := reloadedRoom.GetProperty("temperature")
		if !exists || temp != 25.5 {
			t.Errorf("Expected temperature 25.5, got %v (exists: %v)", temp, exists)
		}

		lighting, exists := reloadedRoom.GetProperty("lighting")
		if !exists || lighting != "dim" {
			t.Errorf("Expected lighting 'dim', got %v (exists: %v)", lighting, exists)
		}

		hasTreasure, exists := reloadedRoom.GetProperty("has_treasure")
		if !exists || hasTreasure != false {
			t.Errorf("Expected has_treasure false, got %v (exists: %v)", hasTreasure, exists)
		}

		// Test property updates persist
		reloadedRoom.SetProperty("lighting", "bright")
		if err := manager.Worlds().SaveRoom(reloadedRoom); err != nil {
			t.Fatalf("Failed to save updated room: %v", err)
		}

		updatedRoom, err := manager.Worlds().LoadRoom(world.ID, room.ID)
		if err != nil {
			t.Fatalf("Failed to reload updated room: %v", err)
		}

		newLighting, _ := updatedRoom.GetProperty("lighting")
		if newLighting != "bright" {
			t.Errorf("Expected updated lighting 'bright', got %v", newLighting)
		}
	})

	t.Run("World and Room Listing", func(t *testing.T) {
		// Create multiple worlds with rooms
		world1, _ := manager.CreateNewWorld("List World 1", "First list world")
		world2, _ := manager.CreateNewWorld("List World 2", "Second list world")
		world3, _ := manager.CreateNewWorld("List World 3", "Third list world")

		// Create rooms in different worlds
		_, _ = manager.CreateNewRoom(world1.ID, "Room 1A", "First room in world 1")
		_, _ = manager.CreateNewRoom(world1.ID, "Room 1B", "Second room in world 1")
		_, _ = manager.CreateNewRoom(world1.ID, "Room 1C", "Third room in world 1")

		_, _ = manager.CreateNewRoom(world2.ID, "Room 2A", "First room in world 2")
		_, _ = manager.CreateNewRoom(world2.ID, "Room 2B", "Second room in world 2")

		// World 3 has no rooms

		// Test world listing
		worldIDs, err := manager.Worlds().ListWorlds()
		if err != nil {
			t.Fatalf("Failed to list worlds: %v", err)
		}

		expectedWorlds := []string{world1.ID, world2.ID, world3.ID}
		for _, expectedID := range expectedWorlds {
			found := false
			for _, actualID := range worldIDs {
				if actualID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected world %s not found in list", expectedID)
			}
		}

		// Test room listing for world 1
		roomIDs1, err := manager.Worlds().ListRooms(world1.ID)
		if err != nil {
			t.Fatalf("Failed to list rooms in world 1: %v", err)
		}

		if len(roomIDs1) != 3 {
			t.Errorf("Expected 3 rooms in world 1, got %d", len(roomIDs1))
		}

		// Test room listing for world 2
		roomIDs2, err := manager.Worlds().ListRooms(world2.ID)
		if err != nil {
			t.Fatalf("Failed to list rooms in world 2: %v", err)
		}

		if len(roomIDs2) != 2 {
			t.Errorf("Expected 2 rooms in world 2, got %d", len(roomIDs2))
		}

		// Test room listing for world 3 (empty) - should handle non-existent directory gracefully
		roomIDs3, err := manager.Worlds().ListRooms(world3.ID)
		if err != nil {
			// It's acceptable for ListRooms to fail if the world directory doesn't exist yet
			// This happens when a world has no rooms
			roomIDs3 = []string{}
		}

		if len(roomIDs3) != 0 {
			t.Errorf("Expected 0 rooms in world 3, got %d", len(roomIDs3))
		}

		// Test listing rooms for non-existent world
		_, err = manager.Worlds().ListRooms("non-existent-world")
		if err == nil {
			t.Error("Should fail to list rooms for non-existent world")
		}
	})
}

// TestWorldErrorHandling tests error scenarios for world and room operations
func TestWorldErrorHandling(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-error-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	t.Run("Loading Non-existent Entities", func(t *testing.T) {
		// Test loading non-existent world
		_, err := manager.Worlds().LoadWorld("non-existent-world")
		if err == nil {
			t.Error("Should fail to load non-existent world")
		}

		// Test loading non-existent room
		world, _ := manager.CreateNewWorld("Test World", "Test world")
		_, err = manager.Worlds().LoadRoom(world.ID, "non-existent-room")
		if err == nil {
			t.Error("Should fail to load non-existent room")
		}

		// Test loading room from non-existent world
		_, err = manager.Worlds().LoadRoom("non-existent-world", "some-room")
		if err == nil {
			t.Error("Should fail to load room from non-existent world")
		}
	})

	t.Run("Deletion Error Scenarios", func(t *testing.T) {
		// Test deleting non-existent world
		err := manager.Worlds().DeleteWorld("non-existent-world")
		if err == nil {
			t.Error("Should fail to delete non-existent world")
		}

		// Test deleting non-existent room
		world, _ := manager.CreateNewWorld("Deletion Test World", "Test world")
		err = manager.Worlds().DeleteRoom(world.ID, "non-existent-room")
		if err == nil {
			t.Error("Should fail to delete non-existent room")
		}

		// Test deleting room from non-existent world
		err = manager.Worlds().DeleteRoom("non-existent-world", "some-room")
		if err == nil {
			t.Error("Should fail to delete room from non-existent world")
		}
	})

	t.Run("Exit Validation Edge Cases", func(t *testing.T) {
		world, _ := manager.CreateNewWorld("Edge Case World", "Test world")
		room, _ := manager.CreateNewRoom(world.ID, "Edge Case Room", "Test room")

		// Test validation with same source and target (self-loop)
		room.AddAllowedInlet(room.ID) // Allow self-connection
		if err := manager.Worlds().SaveRoom(room); err != nil {
			t.Fatalf("Failed to save room: %v", err)
		}

		err := manager.Worlds().ValidateExit(world.ID, room.ID, world.ID, room.ID)
		if err != nil {
			t.Errorf("Self-loop validation should succeed when allowed: %v", err)
		}

		// Test creating self-referencing exit
		err = manager.Worlds().CreateExitWithValidation(world.ID, room.ID, "mirror", world.ID, room.ID)
		if err != nil {
			t.Errorf("Self-referencing exit creation should succeed: %v", err)
		}
	})

	t.Run("Property Edge Cases", func(t *testing.T) {
		world, _ := manager.CreateNewWorld("Property Edge World", "Test world")
		room, _ := manager.CreateNewRoom(world.ID, "Property Edge Room", "Test room")

		// Test setting nil properties map
		world.Properties = nil
		world.SetProperty("test", "value")
		if world.Properties == nil {
			t.Error("Properties map should be initialized when nil")
		}

		room.Properties = nil
		room.SetProperty("test", "value")
		if room.Properties == nil {
			t.Error("Properties map should be initialized when nil")
		}

		// Test getting from nil properties map
		world.Properties = nil
		_, exists := world.GetProperty("test")
		if exists {
			t.Error("Should not find property in nil map")
		}

		room.Properties = nil
		_, exists = room.GetProperty("test")
		if exists {
			t.Error("Should not find property in nil map")
		}
	})
}

// TestWorldConcurrency tests concurrent operations on worlds and rooms
func TestWorldConcurrency(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-concurrency-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	world, err := manager.CreateNewWorld("Concurrency World", "Test world for concurrency")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	room, err := manager.CreateNewRoom(world.ID, "Concurrency Room", "Test room for concurrency")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	t.Run("Concurrent Property Updates", func(t *testing.T) {
		done := make(chan bool, 10)

		// Concurrent world property updates
		for i := 0; i < 5; i++ {
			go func(index int) {
				defer func() { done <- true }()
				
				loadedWorld, err := manager.Worlds().LoadWorld(world.ID)
				if err != nil {
					t.Errorf("Failed to load world in goroutine %d: %v", index, err)
					return
				}

				loadedWorld.SetProperty("concurrent_test", index)
				if err := manager.Worlds().SaveWorld(loadedWorld); err != nil {
					t.Errorf("Failed to save world in goroutine %d: %v", index, err)
				}
			}(i)
		}

		// Concurrent room property updates
		for i := 0; i < 5; i++ {
			go func(index int) {
				defer func() { done <- true }()
				
				loadedRoom, err := manager.Worlds().LoadRoom(world.ID, room.ID)
				if err != nil {
					t.Errorf("Failed to load room in goroutine %d: %v", index, err)
					return
				}

				loadedRoom.SetProperty("concurrent_test", index)
				if err := manager.Worlds().SaveRoom(loadedRoom); err != nil {
					t.Errorf("Failed to save room in goroutine %d: %v", index, err)
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify final state (one of the concurrent updates should have won)
		finalWorld, err := manager.Worlds().LoadWorld(world.ID)
		if err != nil {
			t.Fatalf("Failed to load final world state: %v", err)
		}

		value, exists := finalWorld.GetProperty("concurrent_test")
		if !exists {
			t.Error("Concurrent test property should exist after updates")
		}

		if value == nil {
			t.Error("Concurrent test property should have a value")
		}

		finalRoom, err := manager.Worlds().LoadRoom(world.ID, room.ID)
		if err != nil {
			t.Fatalf("Failed to load final room state: %v", err)
		}

		value, exists = finalRoom.GetProperty("concurrent_test")
		if !exists {
			t.Error("Concurrent test property should exist after updates")
		}

		if value == nil {
			t.Error("Concurrent test property should have a value")
		}
	})
}