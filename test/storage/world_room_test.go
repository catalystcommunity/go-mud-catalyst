package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestWorldRoomOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-room-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Test world creation
	world1, err := manager.CreateNewWorld("Test World 1", "First test world")
	if err != nil {
		t.Fatalf("Failed to create world 1: %v", err)
	}

	world2, err := manager.CreateNewWorld("Test World 2", "Second test world")
	if err != nil {
		t.Fatalf("Failed to create world 2: %v", err)
	}

	// Test room creation
	room1, err := manager.CreateNewRoom(world1.ID, "Room 1", "First room in world 1")
	if err != nil {
		t.Fatalf("Failed to create room 1: %v", err)
	}

	_, err = manager.CreateNewRoom(world1.ID, "Room 2", "Second room in world 1")
	if err != nil {
		t.Fatalf("Failed to create room 2: %v", err)
	}

	_, err = manager.CreateNewRoom(world2.ID, "Room 3", "First room in world 2")
	if err != nil {
		t.Fatalf("Failed to create room 3: %v", err)
	}

	// Test world listing
	worlds, err := manager.Worlds().ListWorlds()
	if err != nil {
		t.Fatalf("Failed to list worlds: %v", err)
	}

	if len(worlds) != 2 {
		t.Errorf("Expected 2 worlds, got %d", len(worlds))
	}

	// Test room listing
	roomsInWorld1, err := manager.Worlds().ListRooms(world1.ID)
	if err != nil {
		t.Fatalf("Failed to list rooms in world 1: %v", err)
	}

	if len(roomsInWorld1) != 2 {
		t.Errorf("Expected 2 rooms in world 1, got %d", len(roomsInWorld1))
	}

	roomsInWorld2, err := manager.Worlds().ListRooms(world2.ID)
	if err != nil {
		t.Fatalf("Failed to list rooms in world 2: %v", err)
	}

	if len(roomsInWorld2) != 1 {
		t.Errorf("Expected 1 room in world 2, got %d", len(roomsInWorld2))
	}

	// Test world/room existence
	if !manager.Worlds().WorldExists(world1.ID) {
		t.Error("World 1 should exist")
	}

	if !manager.Worlds().RoomExists(world1.ID, room1.ID) {
		t.Error("Room 1 should exist in world 1")
	}

	if manager.Worlds().RoomExists(world1.ID, "non-existent-room") {
		t.Error("Non-existent room should not exist")
	}

	// Test room property management
	room1.SetProperty("temperature", 72)
	room1.SetProperty("lighting", "bright")

	if err := manager.Worlds().SaveRoom(room1); err != nil {
		t.Fatalf("Failed to save room with properties: %v", err)
	}

	// Reload and verify properties
	reloadedRoom1, err := manager.Worlds().LoadRoom(world1.ID, room1.ID)
	if err != nil {
		t.Fatalf("Failed to reload room 1: %v", err)
	}

	temp, exists := reloadedRoom1.GetProperty("temperature")
	if !exists || temp != float64(72) { // JSON unmarshaling converts to float64
		t.Errorf("Expected temperature 72, got %v (exists: %v)", temp, exists)
	}

	lighting, exists := reloadedRoom1.GetProperty("lighting")
	if !exists || lighting != "bright" {
		t.Errorf("Expected lighting 'bright', got %v (exists: %v)", lighting, exists)
	}

	// Test world property management
	world1.SetProperty("climate", "temperate")
	world1.SetProperty("danger_level", 3)

	if err := manager.Worlds().SaveWorld(world1); err != nil {
		t.Fatalf("Failed to save world with properties: %v", err)
	}

	reloadedWorld1, err := manager.Worlds().LoadWorld(world1.ID)
	if err != nil {
		t.Fatalf("Failed to reload world 1: %v", err)
	}

	climate, exists := reloadedWorld1.GetProperty("climate")
	if !exists || climate != "temperate" {
		t.Errorf("Expected climate 'temperate', got %v (exists: %v)", climate, exists)
	}

	// Test world activation
	if !reloadedWorld1.IsActive {
		t.Error("World should be active by default")
	}

	reloadedWorld1.IsActive = false
	if err := manager.Worlds().SaveWorld(reloadedWorld1); err != nil {
		t.Fatalf("Failed to save deactivated world: %v", err)
	}

	// Test getting active worlds
	activeWorlds, err := manager.Worlds().GetActiveWorlds()
	if err != nil {
		t.Fatalf("Failed to get active worlds: %v", err)
	}

	// Should only have world2 now
	if len(activeWorlds) != 1 {
		t.Errorf("Expected 1 active world, got %d", len(activeWorlds))
	}

	if len(activeWorlds) > 0 && activeWorlds[0].ID != world2.ID {
		t.Errorf("Expected active world to be world 2, got %s", activeWorlds[0].ID)
	}
}

func TestSecureExitSystem(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-secure-exit-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create worlds and rooms
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

	room3, err := manager.CreateNewRoom(world2.ID, "Room 3", "Cross-world room")
	if err != nil {
		t.Fatalf("Failed to create room 3: %v", err)
	}

	// Test basic exit creation (within same world)
	room1.AddExit("north", world1.ID, room2.ID)
	if err := manager.Worlds().SaveRoom(room1); err != nil {
		t.Fatalf("Failed to save room 1 with exit: %v", err)
	}

	// Verify exit was added
	exit := room1.GetExit("north")
	if exit == nil {
		t.Fatal("Exit 'north' should exist")
	}

	if exit.TargetRoomID != room2.ID {
		t.Errorf("Expected exit to target room 2, got %s", exit.TargetRoomID)
	}

	// Test allowed inlet system
	room2.AddAllowedInlet(room1.ID)
	if err := manager.Worlds().SaveRoom(room2); err != nil {
		t.Fatalf("Failed to save room 2 with allowed inlet: %v", err)
	}

	// Test exit validation (should succeed now)
	err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world1.ID, room2.ID)
	if err != nil {
		t.Errorf("Exit validation should succeed with allowed inlet: %v", err)
	}

	// Test cross-world exit without permission (should fail)
	err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world2.ID, room3.ID)
	if err == nil {
		t.Error("Cross-world exit validation should fail without allowed inlet")
	}

	// Add cross-world permission
	room3.AddAllowedInlet(room1.ID)
	if err := manager.Worlds().SaveRoom(room3); err != nil {
		t.Fatalf("Failed to save room 3 with cross-world inlet: %v", err)
	}

	// Now validation should succeed
	err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world2.ID, room3.ID)
	if err != nil {
		t.Errorf("Cross-world exit validation should succeed with allowed inlet: %v", err)
	}

	// Test creating exit with validation
	err = manager.Worlds().CreateExitWithValidation(world1.ID, room1.ID, "portal", world2.ID, room3.ID)
	if err != nil {
		t.Errorf("Failed to create validated exit: %v", err)
	}

	// Reload room and verify exit was created
	reloadedRoom1, err := manager.Worlds().LoadRoom(world1.ID, room1.ID)
	if err != nil {
		t.Fatalf("Failed to reload room 1: %v", err)
	}

	portalExit := reloadedRoom1.GetExit("portal")
	if portalExit == nil {
		t.Error("Portal exit should exist")
	}

	if portalExit != nil && portalExit.TargetWorldID != world2.ID {
		t.Errorf("Portal should target world 2, got %s", portalExit.TargetWorldID)
	}

	// Test removing allowed inlet
	if !room3.RemoveAllowedInlet(room1.ID) {
		t.Error("Should have successfully removed allowed inlet")
	}

	if err := manager.Worlds().SaveRoom(room3); err != nil {
		t.Fatalf("Failed to save room 3 after removing inlet: %v", err)
	}

	// Now validation should fail again
	err = manager.Worlds().ValidateExit(world1.ID, room1.ID, world2.ID, room3.ID)
	if err == nil {
		t.Error("Exit validation should fail after removing allowed inlet")
	}

	// Test exit removal
	if !room1.RemoveExit("north") {
		t.Error("Should have successfully removed north exit")
	}

	if room1.GetExit("north") != nil {
		t.Error("North exit should no longer exist")
	}

	// Test exit properties
	room1.AddExit("secret", world1.ID, room2.ID)
	secretExit := room1.GetExit("secret")
	if secretExit == nil {
		t.Fatal("Secret exit should exist")
	}

	secretExit.IsHidden = true
	secretExit.Description = "A hidden passage"
	if secretExit.Properties == nil {
		secretExit.Properties = make(map[string]interface{})
	}
	secretExit.Properties["requires_key"] = true

	if err := manager.Worlds().SaveRoom(room1); err != nil {
		t.Fatalf("Failed to save room with exit properties: %v", err)
	}

	// Reload and verify exit properties
	reloadedRoom1, err = manager.Worlds().LoadRoom(world1.ID, room1.ID)
	if err != nil {
		t.Fatalf("Failed to reload room 1: %v", err)
	}

	reloadedSecretExit := reloadedRoom1.GetExit("secret")
	if reloadedSecretExit == nil {
		t.Fatal("Secret exit should exist after reload")
	}

	if !reloadedSecretExit.IsHidden {
		t.Error("Secret exit should be hidden")
	}

	if reloadedSecretExit.Description != "A hidden passage" {
		t.Errorf("Expected description 'A hidden passage', got '%s'", reloadedSecretExit.Description)
	}
}

func TestRoomItemManagement(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-room-item-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create world and room
	world, err := manager.CreateNewWorld("Test World", "Test world for items")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	room, err := manager.CreateNewRoom(world.ID, "Item Room", "A room with items")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	// Create items
	sword, err := manager.CreateNewItem("Magic Sword", "A glowing sword")
	if err != nil {
		t.Fatalf("Failed to create sword: %v", err)
	}

	shield, err := manager.CreateNewItem("Iron Shield", "A sturdy shield")
	if err != nil {
		t.Fatalf("Failed to create shield: %v", err)
	}

	// Create item instances and add to room
	swordInstance, err := manager.CreateItemInstance(sword.ID, 1)
	if err != nil {
		t.Fatalf("Failed to create sword instance: %v", err)
	}

	shieldInstance, err := manager.CreateItemInstance(shield.ID, 2)
	if err != nil {
		t.Fatalf("Failed to create shield instance: %v", err)
	}

	// Add items to room
	room.AddItem(swordInstance)
	room.AddItem(shieldInstance)

	if err := manager.Worlds().SaveRoom(room); err != nil {
		t.Fatalf("Failed to save room with items: %v", err)
	}

	// Reload room and verify items
	reloadedRoom, err := manager.Worlds().LoadRoom(world.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to reload room: %v", err)
	}

	if len(reloadedRoom.Items) != 2 {
		t.Errorf("Expected 2 items in room, got %d", len(reloadedRoom.Items))
	}

	// Verify item properties
	for _, item := range reloadedRoom.Items {
		if item.WorldID != world.ID {
			t.Errorf("Item should have world ID %s, got %s", world.ID, item.WorldID)
		}

		if item.RoomID != room.ID {
			t.Errorf("Item should have room ID %s, got %s", room.ID, item.RoomID)
		}

		if item.OwnerID != "" {
			t.Error("Items in rooms should not have owner ID")
		}
	}

	// Test finding item in room
	foundSword := reloadedRoom.FindItem(swordInstance.ID)
	if foundSword == nil {
		t.Error("Should have found sword in room")
	}

	if foundSword != nil && foundSword.ItemID != sword.ID {
		t.Errorf("Found item should be sword, got item ID %s", foundSword.ItemID)
	}

	// Test removing item from room
	if !reloadedRoom.RemoveItem(swordInstance.ID) {
		t.Error("Should have successfully removed sword from room")
	}

	if len(reloadedRoom.Items) != 1 {
		t.Errorf("Expected 1 item in room after removal, got %d", len(reloadedRoom.Items))
	}

	// Verify item was removed
	if reloadedRoom.FindItem(swordInstance.ID) != nil {
		t.Error("Sword should no longer be in room")
	}

	// Test removing non-existent item
	if reloadedRoom.RemoveItem("non-existent-item") {
		t.Error("Should not have successfully removed non-existent item")
	}
}

func TestWorldDeletion(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-deletion-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create world with multiple rooms
	world, err := manager.CreateNewWorld("Test World", "World to be deleted")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	room1, err := manager.CreateNewRoom(world.ID, "Room 1", "First room")
	if err != nil {
		t.Fatalf("Failed to create room 1: %v", err)
	}

	room2, err := manager.CreateNewRoom(world.ID, "Room 2", "Second room")
	if err != nil {
		t.Fatalf("Failed to create room 2: %v", err)
	}

	// Verify world and rooms exist
	if !manager.Worlds().WorldExists(world.ID) {
		t.Error("World should exist before deletion")
	}

	if !manager.Worlds().RoomExists(world.ID, room1.ID) {
		t.Error("Room 1 should exist before deletion")
	}

	if !manager.Worlds().RoomExists(world.ID, room2.ID) {
		t.Error("Room 2 should exist before deletion")
	}

	// Delete world
	err = manager.Worlds().DeleteWorld(world.ID)
	if err != nil {
		t.Fatalf("Failed to delete world: %v", err)
	}

	// Verify world and rooms no longer exist
	if manager.Worlds().WorldExists(world.ID) {
		t.Error("World should not exist after deletion")
	}

	if manager.Worlds().RoomExists(world.ID, room1.ID) {
		t.Error("Room 1 should not exist after world deletion")
	}

	if manager.Worlds().RoomExists(world.ID, room2.ID) {
		t.Error("Room 2 should not exist after world deletion")
	}

	// Test individual room deletion
	world2, err := manager.CreateNewWorld("World 2", "Second world")
	if err != nil {
		t.Fatalf("Failed to create world 2: %v", err)
	}

	room3, err := manager.CreateNewRoom(world2.ID, "Room 3", "Third room")
	if err != nil {
		t.Fatalf("Failed to create room 3: %v", err)
	}

	room4, err := manager.CreateNewRoom(world2.ID, "Room 4", "Fourth room")
	if err != nil {
		t.Fatalf("Failed to create room 4: %v", err)
	}

	// Delete only one room
	err = manager.Worlds().DeleteRoom(world2.ID, room3.ID)
	if err != nil {
		t.Fatalf("Failed to delete room 3: %v", err)
	}

	// Verify world still exists but room 3 is gone
	if !manager.Worlds().WorldExists(world2.ID) {
		t.Error("World 2 should still exist after room deletion")
	}

	if manager.Worlds().RoomExists(world2.ID, room3.ID) {
		t.Error("Room 3 should not exist after deletion")
	}

	if !manager.Worlds().RoomExists(world2.ID, room4.ID) {
		t.Error("Room 4 should still exist after room 3 deletion")
	}
}