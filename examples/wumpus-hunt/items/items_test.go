package items

import (
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestCreateWumpusPelt(t *testing.T) {
	instanceID := "test_instance_123"
	playerName := "TestPlayer"
	
	pelt := CreateWumpusPelt(instanceID, playerName)
	
	if pelt.Name != "Wumpus Pelt" {
		t.Errorf("Expected name 'Wumpus Pelt', got %s", pelt.Name)
	}
	
	if pelt.Description == "" {
		t.Error("Expected non-empty description")
	}
	
	// Check wumpus-specific properties
	data, err := GetWumpusItemData(pelt)
	if err != nil {
		t.Fatalf("Failed to get wumpus item data: %v", err)
	}
	
	if data.ItemType != WumpusItemTypeTrophy {
		t.Errorf("Expected item type %s, got %s", WumpusItemTypeTrophy, data.ItemType)
	}
	
	if data.Rarity != WumpusItemRarityRare {
		t.Errorf("Expected rarity %s, got %s", WumpusItemRarityRare, data.Rarity)
	}
	
	if data.DefeatedInInstance != instanceID {
		t.Errorf("Expected instance ID %s, got %s", instanceID, data.DefeatedInInstance)
	}
	
	if data.PlayerName != playerName {
		t.Errorf("Expected player name %s, got %s", playerName, data.PlayerName)
	}
	
	if data.VictoryTimestamp == "" {
		t.Error("Expected non-empty victory timestamp")
	}
	
	// Verify timestamp is valid
	if _, err := time.Parse(time.RFC3339, data.VictoryTimestamp); err != nil {
		t.Errorf("Invalid victory timestamp format: %v", err)
	}
}

func TestCreateWumpusSpear(t *testing.T) {
	spear := CreateWumpusSpear()
	
	if spear.Name != "Hunting Spear" {
		t.Errorf("Expected name 'Hunting Spear', got %s", spear.Name)
	}
	
	data, err := GetWumpusItemData(spear)
	if err != nil {
		t.Fatalf("Failed to get wumpus item data: %v", err)
	}
	
	if data.ItemType != WumpusItemTypeWeapon {
		t.Errorf("Expected item type %s, got %s", WumpusItemTypeWeapon, data.ItemType)
	}
	
	if data.Rarity != WumpusItemRarityCommon {
		t.Errorf("Expected rarity %s, got %s", WumpusItemRarityCommon, data.Rarity)
	}
	
	if data.DamageBonus != 10 {
		t.Errorf("Expected damage bonus 10, got %d", data.DamageBonus)
	}
}

func TestCreateWumpusArmor(t *testing.T) {
	armor := CreateWumpusArmor()
	
	if armor.Name != "Leather Armor" {
		t.Errorf("Expected name 'Leather Armor', got %s", armor.Name)
	}
	
	data, err := GetWumpusItemData(armor)
	if err != nil {
		t.Fatalf("Failed to get wumpus item data: %v", err)
	}
	
	if data.ItemType != WumpusItemTypeArmor {
		t.Errorf("Expected item type %s, got %s", WumpusItemTypeArmor, data.ItemType)
	}
	
	if data.DefenseBonus != 5 {
		t.Errorf("Expected defense bonus 5, got %d", data.DefenseBonus)
	}
}

func TestGetWumpusItemData(t *testing.T) {
	// Test with regular item (should fail)
	regularItem := storage.NewItem(ids.NewEntityID(), "Regular Item")
	regularItem.Description = "A regular item"
	_, err := GetWumpusItemData(regularItem)
	if err == nil {
		t.Error("Expected error for regular item, got nil")
	}
	
	// Test with wumpus item (should succeed)
	wumpusItem := CreateWumpusPelt("test_instance", "TestPlayer")
	data, err := GetWumpusItemData(wumpusItem)
	if err != nil {
		t.Fatalf("Expected no error for wumpus item, got %v", err)
	}
	
	if data == nil {
		t.Error("Expected non-nil data")
	}
}

func TestIsWumpusItem(t *testing.T) {
	// Test with regular item
	regularItem := storage.NewItem(ids.NewEntityID(), "Regular Item")
	regularItem.Description = "A regular item"
	if IsWumpusItem(regularItem) {
		t.Error("Expected false for regular item")
	}
	
	// Test with wumpus item
	wumpusItem := CreateWumpusPelt("test_instance", "TestPlayer")
	if !IsWumpusItem(wumpusItem) {
		t.Error("Expected true for wumpus item")
	}
}

func TestIsWumpusPelt(t *testing.T) {
	// Test with wumpus pelt
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	if !IsWumpusPelt(pelt) {
		t.Error("Expected true for wumpus pelt")
	}
	
	// Test with weapon
	weapon := CreateWumpusSpear()
	if IsWumpusPelt(weapon) {
		t.Error("Expected false for weapon")
	}
	
	// Test with regular item
	regularItem := storage.NewItem(ids.NewEntityID(), "Regular Item")
	regularItem.Description = "A regular item"
	if IsWumpusPelt(regularItem) {
		t.Error("Expected false for regular item")
	}
}

func TestGivePlayerItem(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	item := CreateWumpusPelt("test_instance", "TestPlayer")
	quantity := 1
	
	instance := GivePlayerItem(player, item, quantity)
	
	if instance.ItemID != item.ID {
		t.Errorf("Expected item ID %s, got %s", item.ID, instance.ItemID)
	}
	
	if instance.Quantity != quantity {
		t.Errorf("Expected quantity %d, got %d", quantity, instance.Quantity)
	}
	
	// Check if properties were copied
	_, err := GetWumpusItemData(&storage.Item{
		ID:         instance.ItemID,
		Properties: instance.Properties,
	})
	if err != nil {
		t.Errorf("Expected wumpus item properties to be copied: %v", err)
	}
}

func TestFormatItemDescription(t *testing.T) {
	pelt := CreateWumpusPelt("test_instance", "TestPlayer")
	description := FormatItemDescription(pelt)
	
	if description == "" {
		t.Error("Expected non-empty description")
	}
	
	// Should contain original description
	if !contains(description, pelt.Description) {
		t.Error("Expected description to contain original item description")
	}
	
	// Should contain victory timestamp
	if !contains(description, "Defeated on:") {
		t.Error("Expected description to contain victory timestamp")
	}
	
	// Should contain player name
	if !contains(description, "TestPlayer") {
		t.Error("Expected description to contain player name")
	}
}

func TestGetRarityDisplayName(t *testing.T) {
	tests := []struct {
		rarity   WumpusItemRarity
		expected string
	}{
		{WumpusItemRarityCommon, "Common"},
		{WumpusItemRarityUncommon, "Uncommon"},
		{WumpusItemRarityRare, "Rare"},
		{WumpusItemRarityLegendary, "Legendary"},
		{WumpusItemRarity("invalid"), "Unknown"},
	}
	
	for _, test := range tests {
		result := GetRarityDisplayName(test.rarity)
		if result != test.expected {
			t.Errorf("Expected %s for rarity %s, got %s", test.expected, test.rarity, result)
		}
	}
}

func TestGetTypeDisplayName(t *testing.T) {
	tests := []struct {
		itemType WumpusItemType
		expected string
	}{
		{WumpusItemTypeTrophy, "Trophy"},
		{WumpusItemTypeWeapon, "Weapon"},
		{WumpusItemTypeArmor, "Armor"},
		{WumpusItemType("invalid"), "Unknown"},
	}
	
	for _, test := range tests {
		result := GetTypeDisplayName(test.itemType)
		if result != test.expected {
			t.Errorf("Expected %s for type %s, got %s", test.expected, test.itemType, result)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) > len(substr) && contains(s[1:], substr)
}