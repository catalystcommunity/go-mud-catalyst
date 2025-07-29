package items

import (
	"errors"
	"fmt"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// PlayerInventory represents a player's inventory using Properties extension
type PlayerInventory struct {
	Items map[string]*InventoryItem `json:"items"`
}

// InventoryItem represents an item in a player's inventory
type InventoryItem struct {
	ItemID         string                 `json:"item_id"`
	Quantity       int                    `json:"quantity"`
	ItemProperties map[string]interface{} `json:"item_properties"`
}

// GetPlayerInventory retrieves the player's inventory from Properties
func GetPlayerInventory(player *storage.Player) *PlayerInventory {
	inventoryData, exists := player.GetProperty("wumpus_inventory")
	if !exists {
		return &PlayerInventory{
			Items: make(map[string]*InventoryItem),
		}
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := inventoryData.(type) {
	case PlayerInventory:
		return &v
	case *PlayerInventory:
		return v
	case map[string]interface{}:
		inventory := &PlayerInventory{
			Items: make(map[string]*InventoryItem),
		}
		
		if itemsData, ok := v["items"].(map[string]interface{}); ok {
			for itemID, itemData := range itemsData {
				if itemMap, ok := itemData.(map[string]interface{}); ok {
					item := &InventoryItem{
						ItemID:         itemID,
						ItemProperties: make(map[string]interface{}),
					}
					
					if quantity, ok := itemMap["quantity"].(float64); ok {
						item.Quantity = int(quantity)
					}
					if id, ok := itemMap["item_id"].(string); ok {
						item.ItemID = id
					}
					if props, ok := itemMap["item_properties"].(map[string]interface{}); ok {
						item.ItemProperties = props
					}
					
					inventory.Items[itemID] = item
				}
			}
		}
		
		return inventory
	default:
		return &PlayerInventory{
			Items: make(map[string]*InventoryItem),
		}
	}
}

// SavePlayerInventory saves the player's inventory to Properties
func SavePlayerInventory(player *storage.Player, inventory *PlayerInventory) {
	player.SetProperty("wumpus_inventory", inventory)
}

// AddItemToInventory adds an item to a player's inventory
func AddItemToInventory(player *storage.Player, item *storage.Item, quantity int) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	
	inventory := GetPlayerInventory(player)
	
	// Check if item already exists in inventory
	if existingItem, exists := inventory.Items[item.ID]; exists {
		existingItem.Quantity += quantity
	} else {
		// Create new inventory item
		inventoryItem := &InventoryItem{
			ItemID:         item.ID,
			Quantity:       quantity,
			ItemProperties: make(map[string]interface{}),
		}
		
		// Copy item properties
		for key, value := range item.Properties {
			inventoryItem.ItemProperties[key] = value
		}
		
		inventory.Items[item.ID] = inventoryItem
	}
	
	SavePlayerInventory(player, inventory)
	return nil
}

// RemoveItemFromInventory removes an item from a player's inventory
func RemoveItemFromInventory(player *storage.Player, itemID string, quantity int) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	
	inventory := GetPlayerInventory(player)
	
	existingItem, exists := inventory.Items[itemID]
	if !exists {
		return errors.New("item not found in inventory")
	}
	
	if existingItem.Quantity < quantity {
		return errors.New("insufficient quantity")
	}
	
	existingItem.Quantity -= quantity
	
	// Remove item completely if quantity reaches 0
	if existingItem.Quantity == 0 {
		delete(inventory.Items, itemID)
	}
	
	SavePlayerInventory(player, inventory)
	return nil
}

// GetItemQuantity returns the quantity of a specific item in inventory
func GetItemQuantity(player *storage.Player, itemID string) int {
	inventory := GetPlayerInventory(player)
	
	if item, exists := inventory.Items[itemID]; exists {
		return item.Quantity
	}
	
	return 0
}

// HasItem checks if a player has a specific item in their inventory
func HasItem(player *storage.Player, itemID string) bool {
	return GetItemQuantity(player, itemID) > 0
}

// GetInventoryItemsByType returns all items of a specific type from inventory
func GetInventoryItemsByType(player *storage.Player, itemType WumpusItemType) []*InventoryItem {
	inventory := GetPlayerInventory(player)
	var items []*InventoryItem
	
	for _, item := range inventory.Items {
		if wumpusData, exists := item.ItemProperties["wumpus_item"]; exists {
			// Handle both struct and map cases
			switch v := wumpusData.(type) {
			case WumpusItemData:
				if v.ItemType == itemType {
					items = append(items, item)
				}
			case map[string]interface{}:
				if typeStr, ok := v["item_type"].(string); ok {
					if WumpusItemType(typeStr) == itemType {
						items = append(items, item)
					}
				}
			}
		}
	}
	
	return items
}

// GetWumpusPeltQuantity returns the number of wumpus pelts a player has
func GetWumpusPeltQuantity(player *storage.Player) int {
	pelts := GetInventoryItemsByType(player, WumpusItemTypeTrophy)
	count := 0
	
	for _, pelt := range pelts {
		count += pelt.Quantity
	}
	
	return count
}

// FormatInventoryList returns a formatted string of the player's inventory
func FormatInventoryList(player *storage.Player) string {
	inventory := GetPlayerInventory(player)
	
	if len(inventory.Items) == 0 {
		return "Your inventory is empty."
	}
	
	result := "=== Your Inventory ===\n"
	
	for _, item := range inventory.Items {
		name := fmt.Sprintf("Item %s", item.ItemID) // Default name
		description := "Unknown item"
		
		// Try to get item name and description from properties
		if wumpusData, exists := item.ItemProperties["wumpus_item"]; exists {
			if wumpusMap, ok := wumpusData.(map[string]interface{}); ok {
				if desc, ok := wumpusMap["description"].(string); ok {
					description = desc
				}
				if itemType, ok := wumpusMap["item_type"].(string); ok {
					if rarity, ok := wumpusMap["rarity"].(string); ok {
						name = fmt.Sprintf("%s %s", GetRarityDisplayName(WumpusItemRarity(rarity)), GetTypeDisplayName(WumpusItemType(itemType)))
					}
				}
			}
		}
		
		result += fmt.Sprintf("- %s (x%d): %s\n", name, item.Quantity, description)
	}
	
	return result
}

// ClearInventory removes all items from a player's inventory
func ClearInventory(player *storage.Player) {
	inventory := &PlayerInventory{
		Items: make(map[string]*InventoryItem),
	}
	SavePlayerInventory(player, inventory)
}

// GetInventorySize returns the number of unique items in inventory
func GetInventorySize(player *storage.Player) int {
	inventory := GetPlayerInventory(player)
	return len(inventory.Items)
}

// GetTotalItemCount returns the total number of items (including quantities)
func GetTotalItemCount(player *storage.Player) int {
	inventory := GetPlayerInventory(player)
	total := 0
	
	for _, item := range inventory.Items {
		total += item.Quantity
	}
	
	return total
}

// TransferItem transfers an item from one player to another
func TransferItem(fromPlayer, toPlayer *storage.Player, itemID string, quantity int) error {
	// Check if source player has enough items
	if GetItemQuantity(fromPlayer, itemID) < quantity {
		return errors.New("insufficient quantity to transfer")
	}
	
	// Get the item properties from source inventory
	fromInventory := GetPlayerInventory(fromPlayer)
	sourceItem, exists := fromInventory.Items[itemID]
	if !exists {
		return errors.New("item not found in source inventory")
	}
	
	// Create a temporary item object to use AddItemToInventory
	tempItem := &storage.Item{
		ID:         itemID,
		Properties: sourceItem.ItemProperties,
	}
	
	// Add to destination player
	if err := AddItemToInventory(toPlayer, tempItem, quantity); err != nil {
		return fmt.Errorf("failed to add item to destination inventory: %w", err)
	}
	
	// Remove from source player
	if err := RemoveItemFromInventory(fromPlayer, itemID, quantity); err != nil {
		return fmt.Errorf("failed to remove item from source inventory: %w", err)
	}
	
	return nil
}