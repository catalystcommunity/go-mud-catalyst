package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestStorageErrorHandling(t *testing.T) {
	// Test with invalid data root
	tempDir := filepath.Join(os.TempDir(), "muddycore-error-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)

	// Test loading non-existent player
	_, err := manager.Players().Load("non-existent-id")
	if err == nil {
		t.Error("Expected error when loading non-existent player")
	}

	// Test loading player by non-existent username
	_, err = manager.Players().LoadByUsername("non-existent-user")
	if err == nil {
		t.Error("Expected error when loading player by non-existent username")
	}

	// Test loading non-existent item
	_, err = manager.Items().Load("non-existent-item")
	if err == nil {
		t.Error("Expected error when loading non-existent item")
	}

	// Test loading non-existent inventory
	_, err = manager.Inventories().Load("non-existent-inventory")
	if err == nil {
		t.Error("Expected error when loading non-existent inventory")
	}

	// Test loading inventory by non-existent owner
	_, err = manager.Inventories().LoadByOwner("non-existent-owner")
	if err == nil {
		t.Error("Expected error when loading inventory by non-existent owner")
	}

	// Test loading non-existent world
	_, err = manager.Worlds().LoadWorld("non-existent-world")
	if err == nil {
		t.Error("Expected error when loading non-existent world")
	}

	// Test loading room from non-existent world
	_, err = manager.Worlds().LoadRoom("non-existent-world", "non-existent-room")
	if err == nil {
		t.Error("Expected error when loading room from non-existent world")
	}
}

func TestPlayerValidation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-validation-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Test creating player with duplicate username
	_, _, err := manager.CreateNewPlayer("testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create first player: %v", err)
	}

	_, _, err = manager.CreateNewPlayer("testuser", "test2@example.com")
	if err == nil {
		t.Error("Expected error when creating player with duplicate username")
	}

	// Test moving player to non-existent room
	player, _, err := manager.CreateNewPlayer("testuser2", "test2@example.com")
	if err != nil {
		t.Fatalf("Failed to create second player: %v", err)
	}

	err = manager.MovePlayerToRoom(player.ID, "non-existent-world", "non-existent-room")
	if err == nil {
		t.Error("Expected error when moving player to non-existent room")
	}

	// Test ban/unban functionality
	err = manager.BanPlayer(player.ID, "Test ban", "admin-id")
	if err != nil {
		t.Errorf("Failed to ban player: %v", err)
	}

	// Verify player is banned
	reloadedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if !reloadedPlayer.IsBanned {
		t.Error("Player should be banned")
	}

	if reloadedPlayer.BanReason != "Test ban" {
		t.Errorf("Expected ban reason 'Test ban', got '%s'", reloadedPlayer.BanReason)
	}

	// Test moving banned player
	world, err := manager.CreateNewWorld("Test World", "Test world")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	room, err := manager.CreateNewRoom(world.ID, "Test Room", "Test room")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	err = manager.MovePlayerToRoom(player.ID, world.ID, room.ID)
	if err == nil {
		t.Error("Expected error when moving banned player")
	}

	// Test unban
	err = manager.UnbanPlayer(player.ID)
	if err != nil {
		t.Errorf("Failed to unban player: %v", err)
	}

	// Verify player is unbanned
	reloadedPlayer, err = manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if reloadedPlayer.IsBanned {
		t.Error("Player should not be banned")
	}
}

func TestItemConstraints(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-item-constraints-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create a quantity-limited item
	limitedItem := storage.NewItem("limited-item", "Limited Item")
	limitedItem.IsQuantityLimited = true
	limitedItem.MaxQuantity = 5
	if err := manager.Items().Save(limitedItem); err != nil {
		t.Fatalf("Failed to save limited item: %v", err)
	}

	player, _, err := manager.CreateNewPlayer("testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Test giving item within limits
	err = manager.GiveItemToPlayer(player.ID, limitedItem.ID, 3)
	if err != nil {
		t.Errorf("Failed to give item within limits: %v", err)
	}

	// Test giving item exceeding limits
	err = manager.GiveItemToPlayer(player.ID, limitedItem.ID, 3)
	if err == nil {
		t.Error("Expected error when giving item exceeding quantity limits")
	}

	// Test world-limited items
	worldLimitedItem := storage.NewItem("world-limited-item", "World Limited Item")
	worldLimitedItem.IsWorldLimited = true
	worldLimitedItem.AllowedWorlds = []string{"world-1"}
	if err := manager.Items().Save(worldLimitedItem); err != nil {
		t.Fatalf("Failed to save world limited item: %v", err)
	}

	// Test giving world-limited item
	err = manager.GiveItemToPlayer(player.ID, worldLimitedItem.ID, 1)
	if err != nil {
		t.Errorf("Failed to give world-limited item: %v", err)
	}

	// Test world change filtering
	world2, err := manager.CreateNewWorld("World 2", "Second world")
	if err != nil {
		t.Fatalf("Failed to create second world: %v", err)
	}

	room2, err := manager.CreateNewRoom(world2.ID, "Room 2", "Second room")
	if err != nil {
		t.Fatalf("Failed to create second room: %v", err)
	}

	// Move player to different world - should filter out world-limited items
	err = manager.MovePlayerToRoom(player.ID, world2.ID, room2.ID)
	if err != nil {
		t.Errorf("Failed to move player to different world: %v", err)
	}

	// Verify world-limited item was removed from inventory
	inventory, err := manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to load inventory: %v", err)
	}

	hasWorldLimitedItem := false
	for _, item := range inventory.Items {
		if item.ItemID == worldLimitedItem.ID {
			hasWorldLimitedItem = true
			break
		}
	}

	if hasWorldLimitedItem {
		t.Error("World-limited item should have been removed when changing worlds")
	}
}

func TestInventoryOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-inventory-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	player, inventory, err := manager.CreateNewPlayer("testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	item, err := manager.CreateNewItem("Test Item", "A test item")
	if err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// Test giving multiple quantities
	err = manager.GiveItemToPlayer(player.ID, item.ID, 5)
	if err != nil {
		t.Errorf("Failed to give multiple items: %v", err)
	}

	// Reload inventory
	inventory, err = manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	// Test total quantity calculation
	totalQuantity := inventory.GetTotalQuantity(item.ID)
	if totalQuantity != 5 {
		t.Errorf("Expected total quantity 5, got %d", totalQuantity)
	}

	// Test HasItem
	if !inventory.HasItem(item.ID, 3) {
		t.Error("Inventory should have at least 3 items")
	}

	if inventory.HasItem(item.ID, 10) {
		t.Error("Inventory should not have 10 items")
	}

	// Test removing items by type
	err = inventory.RemoveItemsByType(item.ID, 2)
	if err != nil {
		t.Errorf("Failed to remove items by type: %v", err)
	}

	if inventory.GetTotalQuantity(item.ID) != 3 {
		t.Errorf("Expected 3 items remaining, got %d", inventory.GetTotalQuantity(item.ID))
	}

	// Test removing more items than available
	err = inventory.RemoveItemsByType(item.ID, 5)
	if err == nil {
		t.Error("Expected error when removing more items than available")
	}

	// Test taking item from player (through manager)
	if len(inventory.Items) == 0 {
		t.Fatal("No items in inventory to test removal")
	}

	instanceID := inventory.Items[0].ID
	err = manager.TakeItemFromPlayer(player.ID, instanceID)
	if err != nil {
		t.Errorf("Failed to take item from player: %v", err)
	}

	// Test taking non-existent item
	err = manager.TakeItemFromPlayer(player.ID, "non-existent-instance")
	if err == nil {
		t.Error("Expected error when taking non-existent item")
	}
}