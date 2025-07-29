package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// TestMain sets up and tears down for all tests in this package
func TestMain(m *testing.M) {
	// Set log level to WARN to reduce noise during tests
	testLogger := logging.NewLogger(logging.LevelWarn, os.Stderr)
	logging.SetDefaultLogger(testLogger)

	// Run tests
	code := m.Run()

	// Restore default logger
	logging.SetDefaultLogger(logging.DefaultLogger())

	os.Exit(code)
}

func TestStorageManager(t *testing.T) {
	// Create temporary directory for testing
	tempDir := filepath.Join(os.TempDir(), "muddycore-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)

	// Test initialization
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Test player creation
	player, inventory, err := manager.CreateNewPlayer("testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	if player.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", player.Username)
	}

	if inventory.OwnerID != player.ID {
		t.Errorf("Inventory owner ID doesn't match player ID")
	}

	// Test item creation
	item, err := manager.CreateNewItem("Test Sword", "A sharp sword for testing")
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	if item.Name != "Test Sword" {
		t.Errorf("Expected item name 'Test Sword', got '%s'", item.Name)
	}

	// Test giving item to player
	err = manager.GiveItemToPlayer(player.ID, item.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give item to player: %v", err)
	}

	// Reload inventory and verify item was added
	reloadedInventory, err := manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	if len(reloadedInventory.Items) != 1 {
		t.Errorf("Expected 1 item in inventory, got %d", len(reloadedInventory.Items))
	}

	if reloadedInventory.Items[0].ItemID != item.ID {
		t.Errorf("Item ID mismatch in inventory")
	}

	// Test world creation
	world, err := manager.CreateNewWorld("Test World", "A world for testing")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	// Test room creation
	room, err := manager.CreateNewRoom(world.ID, "Test Room", "A room for testing")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	if room.WorldID != world.ID {
		t.Errorf("Room world ID doesn't match")
	}

	// Test moving player to room
	err = manager.MovePlayerToRoom(player.ID, world.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to move player to room: %v", err)
	}

	// Reload player and verify location
	reloadedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if reloadedPlayer.CurrentWorldID != world.ID {
		t.Errorf("Player world ID not updated")
	}

	if reloadedPlayer.CurrentRoomID != room.ID {
		t.Errorf("Player room ID not updated")
	}
}

func TestContainerSystem(t *testing.T) {
	// Create temporary directory for testing
	tempDir := filepath.Join(os.TempDir(), "muddycore-container-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create a player
	player, inventory, err := manager.CreateNewPlayer("containeruser", "container@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Create a container item
	containerItem := storage.NewItem("container-1", "Test Bag")
	containerItem.IsContainer = true
	containerItem.MaxContainerSize = 10
	containerItem.ContainerSizeUnits = "items"
	if err := manager.Items().Save(containerItem); err != nil {
		t.Fatalf("Failed to save container item: %v", err)
	}

	// Create a regular item
	regularItem := storage.NewItem("item-1", "Test Stone")
	regularItem.CanBeContained = true
	if err := manager.Items().Save(regularItem); err != nil {
		t.Fatalf("Failed to save regular item: %v", err)
	}

	// Give both items to player
	err = manager.GiveItemToPlayer(player.ID, containerItem.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give container to player: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, regularItem.ID, 3)
	if err != nil {
		t.Fatalf("Failed to give items to player: %v", err)
	}

	// Reload inventory
	inventory, err = manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	// Find the instances
	var containerInstance, itemInstance *storage.ItemInstance
	for _, instance := range inventory.Items {
		if instance.ItemID == containerItem.ID {
			containerInstance = instance
		} else if instance.ItemID == regularItem.ID {
			itemInstance = instance
		}
	}

	if containerInstance == nil || itemInstance == nil {
		t.Fatalf("Failed to find item instances")
	}

	// Test putting item in container
	err = manager.Containers().PutItemInContainer(inventory, itemInstance.ID, containerInstance.ID)
	if err != nil {
		t.Fatalf("Failed to put item in container: %v", err)
	}

	// Verify item is in container
	contents, err := manager.Containers().GetContainerContents(inventory, containerInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get container contents: %v", err)
	}

	if len(contents) != 1 {
		t.Errorf("Expected 1 item in container, got %d", len(contents))
	}

	if contents[0].ID != itemInstance.ID {
		t.Errorf("Wrong item in container")
	}

	// Test container capacity calculation
	capacity, err := manager.Containers().GetContainerInfo(inventory, containerInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get container info: %v", err)
	}

	if capacity.TotalCapacity != 10 {
		t.Errorf("Expected total capacity 10, got %d", capacity.TotalCapacity)
	}

	if capacity.UsedCapacity != 3 {
		t.Errorf("Expected used capacity 3, got %d", capacity.UsedCapacity)
	}

	if capacity.Available != 7 {
		t.Errorf("Expected available capacity 7, got %d", capacity.Available)
	}
}

func TestIDGeneration(t *testing.T) {
	// Test that IDs are being generated correctly
	tempDir := filepath.Join(os.TempDir(), "muddycore-id-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create multiple players and ensure unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		player, _, err := manager.CreateNewPlayer("user"+string(rune(i+48)), "test@example.com")
		if err != nil {
			t.Fatalf("Failed to create player %d: %v", i, err)
		}

		if ids[player.ID] {
			t.Errorf("Duplicate ID generated: %s", player.ID)
		}
		ids[player.ID] = true

		if len(player.ID) == 0 {
			t.Errorf("Empty ID generated for player %d", i)
		}
	}
}