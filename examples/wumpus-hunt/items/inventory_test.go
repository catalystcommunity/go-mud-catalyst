package items

import (
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestGetPlayerInventory(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Test with no inventory (should return empty)
	inventory := GetPlayerInventory(player)
	if inventory == nil {
		t.Error("Expected non-nil inventory")
	}
	
	if len(inventory.Items) != 0 {
		t.Errorf("Expected empty inventory, got %d items", len(inventory.Items))
	}
}

func TestAddItemToInventory(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	quantity := 1
	
	// Add item to inventory
	err := AddItemToInventory(player, item, quantity)
	if err != nil {
		t.Fatalf("Failed to add item to inventory: %v", err)
	}
	
	// Check inventory
	inventory := GetPlayerInventory(player)
	if len(inventory.Items) != 1 {
		t.Errorf("Expected 1 item in inventory, got %d", len(inventory.Items))
	}
	
	inventoryItem, exists := inventory.Items[item.ID]
	if !exists {
		t.Error("Item not found in inventory")
	}
	
	if inventoryItem.Quantity != quantity {
		t.Errorf("Expected quantity %d, got %d", quantity, inventoryItem.Quantity)
	}
	
	// Add same item again (should increase quantity)
	err = AddItemToInventory(player, item, 2)
	if err != nil {
		t.Fatalf("Failed to add item to inventory: %v", err)
	}
	
	inventory = GetPlayerInventory(player)
	inventoryItem = inventory.Items[item.ID]
	if inventoryItem.Quantity != 3 {
		t.Errorf("Expected quantity 3, got %d", inventoryItem.Quantity)
	}
}

func TestAddItemToInventoryInvalidQuantity(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	
	// Test with zero quantity
	err := AddItemToInventory(player, item, 0)
	if err == nil {
		t.Error("Expected error for zero quantity")
	}
	
	// Test with negative quantity
	err = AddItemToInventory(player, item, -1)
	if err == nil {
		t.Error("Expected error for negative quantity")
	}
}

func TestRemoveItemFromInventory(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	
	// Add item first
	err := AddItemToInventory(player, item, 5)
	if err != nil {
		t.Fatalf("Failed to add item to inventory: %v", err)
	}
	
	// Remove some items
	err = RemoveItemFromInventory(player, item.ID, 2)
	if err != nil {
		t.Fatalf("Failed to remove item from inventory: %v", err)
	}
	
	// Check remaining quantity
	inventory := GetPlayerInventory(player)
	inventoryItem := inventory.Items[item.ID]
	if inventoryItem.Quantity != 3 {
		t.Errorf("Expected quantity 3, got %d", inventoryItem.Quantity)
	}
	
	// Remove all remaining items
	err = RemoveItemFromInventory(player, item.ID, 3)
	if err != nil {
		t.Fatalf("Failed to remove item from inventory: %v", err)
	}
	
	// Check item is completely removed
	inventory = GetPlayerInventory(player)
	if _, exists := inventory.Items[item.ID]; exists {
		t.Error("Item should be completely removed from inventory")
	}
}

func TestRemoveItemFromInventoryErrors(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	
	// Test removing non-existent item
	err := RemoveItemFromInventory(player, item.ID, 1)
	if err == nil {
		t.Error("Expected error for non-existent item")
	}
	
	// Add item and test insufficient quantity
	err = AddItemToInventory(player, item, 2)
	if err != nil {
		t.Fatalf("Failed to add item to inventory: %v", err)
	}
	
	err = RemoveItemFromInventory(player, item.ID, 5)
	if err == nil {
		t.Error("Expected error for insufficient quantity")
	}
	
	// Test invalid quantity
	err = RemoveItemFromInventory(player, item.ID, 0)
	if err == nil {
		t.Error("Expected error for zero quantity")
	}
}

func TestGetItemQuantity(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	
	// Test with no item
	quantity := GetItemQuantity(player, item.ID)
	if quantity != 0 {
		t.Errorf("Expected quantity 0, got %d", quantity)
	}
	
	// Add item and test
	err := AddItemToInventory(player, item, 3)
	if err != nil {
		t.Fatalf("Failed to add item to inventory: %v", err)
	}
	
	quantity = GetItemQuantity(player, item.ID)
	if quantity != 3 {
		t.Errorf("Expected quantity 3, got %d", quantity)
	}
}

func TestHasItem(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	
	// Test with no item
	if HasItem(player, item.ID) {
		t.Error("Expected false for non-existent item")
	}
	
	// Add item and test
	err := AddItemToInventory(player, item, 1)
	if err != nil {
		t.Fatalf("Failed to add item to inventory: %v", err)
	}
	
	if !HasItem(player, item.ID) {
		t.Error("Expected true for existing item")
	}
}

func TestGetInventoryItemsByType(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Add different types of items
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	spear := CreateWumpusSpear()
	armor := CreateWumpusArmor()
	
	err := AddItemToInventory(player, pelt, 1)
	if err != nil {
		t.Fatalf("Failed to add pelt: %v", err)
	}
	
	err = AddItemToInventory(player, spear, 1)
	if err != nil {
		t.Fatalf("Failed to add spear: %v", err)
	}
	
	err = AddItemToInventory(player, armor, 1)
	if err != nil {
		t.Fatalf("Failed to add armor: %v", err)
	}
	
	// Test getting trophies
	trophies := GetInventoryItemsByType(player, WumpusItemTypeTrophy)
	if len(trophies) != 1 {
		t.Errorf("Expected 1 trophy, got %d", len(trophies))
	}
	
	// Test getting weapons
	weapons := GetInventoryItemsByType(player, WumpusItemTypeWeapon)
	if len(weapons) != 1 {
		t.Errorf("Expected 1 weapon, got %d", len(weapons))
	}
	
	// Test getting armor
	armorItems := GetInventoryItemsByType(player, WumpusItemTypeArmor)
	if len(armorItems) != 1 {
		t.Errorf("Expected 1 armor, got %d", len(armorItems))
	}
}

func TestGetWumpusPeltQuantity(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Test with no pelts
	quantity := GetWumpusPeltQuantity(player)
	if quantity != 0 {
		t.Errorf("Expected 0 pelts, got %d", quantity)
	}
	
	// Add pelts
	pelt1 := CreateWumpusPelt("instance1", "TestPlayer")
	pelt2 := CreateWumpusPelt("instance2", "TestPlayer")
	
	err := AddItemToInventory(player, pelt1, 2)
	if err != nil {
		t.Fatalf("Failed to add pelt1: %v", err)
	}
	
	err = AddItemToInventory(player, pelt2, 1)
	if err != nil {
		t.Fatalf("Failed to add pelt2: %v", err)
	}
	
	quantity = GetWumpusPeltQuantity(player)
	if quantity != 3 {
		t.Errorf("Expected 3 pelts, got %d", quantity)
	}
}

func TestFormatInventoryList(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Test with empty inventory
	inventoryText := FormatInventoryList(player)
	if inventoryText != "Your inventory is empty." {
		t.Errorf("Expected empty inventory message, got %s", inventoryText)
	}
	
	// Add items
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	spear := CreateWumpusSpear()
	
	err := AddItemToInventory(player, pelt, 1)
	if err != nil {
		t.Fatalf("Failed to add pelt: %v", err)
	}
	
	err = AddItemToInventory(player, spear, 1)
	if err != nil {
		t.Fatalf("Failed to add spear: %v", err)
	}
	
	inventoryText = FormatInventoryList(player)
	if inventoryText == "Your inventory is empty." {
		t.Error("Expected non-empty inventory message")
	}
	
	// Should contain header
	if !contains(inventoryText, "=== Your Inventory ===") {
		t.Error("Expected inventory header")
	}
}

func TestClearInventory(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Add items
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	err := AddItemToInventory(player, pelt, 1)
	if err != nil {
		t.Fatalf("Failed to add pelt: %v", err)
	}
	
	// Clear inventory
	ClearInventory(player)
	
	// Check inventory is empty
	inventory := GetPlayerInventory(player)
	if len(inventory.Items) != 0 {
		t.Errorf("Expected empty inventory after clear, got %d items", len(inventory.Items))
	}
}

func TestGetInventorySize(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Test with empty inventory
	size := GetInventorySize(player)
	if size != 0 {
		t.Errorf("Expected size 0, got %d", size)
	}
	
	// Add items
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	spear := CreateWumpusSpear()
	
	err := AddItemToInventory(player, pelt, 5)
	if err != nil {
		t.Fatalf("Failed to add pelt: %v", err)
	}
	
	err = AddItemToInventory(player, spear, 1)
	if err != nil {
		t.Fatalf("Failed to add spear: %v", err)
	}
	
	size = GetInventorySize(player)
	if size != 2 {
		t.Errorf("Expected size 2, got %d", size)
	}
}

func TestGetTotalItemCount(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	
	// Test with empty inventory
	count := GetTotalItemCount(player)
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}
	
	// Add items
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	spear := CreateWumpusSpear()
	
	err := AddItemToInventory(player, pelt, 5)
	if err != nil {
		t.Fatalf("Failed to add pelt: %v", err)
	}
	
	err = AddItemToInventory(player, spear, 3)
	if err != nil {
		t.Fatalf("Failed to add spear: %v", err)
	}
	
	count = GetTotalItemCount(player)
	if count != 8 {
		t.Errorf("Expected count 8, got %d", count)
	}
}

func TestTransferItem(t *testing.T) {
	player1 := storage.NewPlayer(ids.NewEntityID(), "Player1")
	player2 := storage.NewPlayer(ids.NewEntityID(), "Player2")
	
	pelt := CreateWumpusPelt("test_instance", "Player1")
	
	// Add item to player1
	err := AddItemToInventory(player1, pelt, 5)
	if err != nil {
		t.Fatalf("Failed to add item to player1: %v", err)
	}
	
	// Transfer some items to player2
	err = TransferItem(player1, player2, pelt.ID, 2)
	if err != nil {
		t.Fatalf("Failed to transfer item: %v", err)
	}
	
	// Check player1 has 3 items left
	quantity1 := GetItemQuantity(player1, pelt.ID)
	if quantity1 != 3 {
		t.Errorf("Expected player1 to have 3 items, got %d", quantity1)
	}
	
	// Check player2 has 2 items
	quantity2 := GetItemQuantity(player2, pelt.ID)
	if quantity2 != 2 {
		t.Errorf("Expected player2 to have 2 items, got %d", quantity2)
	}
}

func TestTransferItemErrors(t *testing.T) {
	player1 := storage.NewPlayer(ids.NewEntityID(), "Player1")
	player2 := storage.NewPlayer(ids.NewEntityID(), "Player2")
	
	pelt := CreateWumpusPelt("test_instance", "Player1")
	
	// Test transferring non-existent item
	err := TransferItem(player1, player2, pelt.ID, 1)
	if err == nil {
		t.Error("Expected error for non-existent item")
	}
	
	// Add item and test insufficient quantity
	err = AddItemToInventory(player1, pelt, 2)
	if err != nil {
		t.Fatalf("Failed to add item to player1: %v", err)
	}
	
	err = TransferItem(player1, player2, pelt.ID, 5)
	if err == nil {
		t.Error("Expected error for insufficient quantity")
	}
}