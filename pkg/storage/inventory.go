package storage

import (
	"fmt"
	"path/filepath"
	"time"
)

// Inventory represents a player's inventory
type Inventory struct {
	ID       string          `json:"id"`
	OwnerID  string          `json:"owner_id"` // Player ID
	WorldID  string          `json:"world_id"` // Current world (for world-limited items)
	Items    []*ItemInstance `json:"items"`
	UpdatedAt time.Time      `json:"updated_at"`
	
	// Custom properties
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// NewInventory creates a new inventory for a player
func NewInventory(id, ownerID, worldID string) *Inventory {
	return &Inventory{
		ID:         id,
		OwnerID:    ownerID,
		WorldID:    worldID,
		Items:      make([]*ItemInstance, 0),
		UpdatedAt:  time.Now(),
		Properties: make(map[string]interface{}),
	}
}

// Update updates the inventory's timestamp
func (inv *Inventory) Update() {
	inv.UpdatedAt = time.Now()
}

// AddItem adds an item instance to the inventory
func (inv *Inventory) AddItem(instance *ItemInstance) {
	instance.OwnerID = inv.OwnerID
	instance.WorldID = inv.WorldID
	inv.Items = append(inv.Items, instance)
	inv.Update()
}

// RemoveItem removes an item instance from the inventory by instance ID
func (inv *Inventory) RemoveItem(instanceID string) bool {
	for i, item := range inv.Items {
		if item.ID == instanceID {
			inv.Items = append(inv.Items[:i], inv.Items[i+1:]...)
			inv.Update()
			return true
		}
	}
	return false
}

// FindItem finds an item instance by instance ID
func (inv *Inventory) FindItem(instanceID string) *ItemInstance {
	for _, item := range inv.Items {
		if item.ID == instanceID {
			return item
		}
	}
	return nil
}

// FindItemsByType finds all item instances of a specific item type
func (inv *Inventory) FindItemsByType(itemID string) []*ItemInstance {
	var items []*ItemInstance
	for _, item := range inv.Items {
		if item.ItemID == itemID {
			items = append(items, item)
		}
	}
	return items
}

// GetItemsInContainer returns all items that are inside a specific container
func (inv *Inventory) GetItemsInContainer(containerInstanceID string) []*ItemInstance {
	var items []*ItemInstance
	for _, item := range inv.Items {
		if item.ContainerID == containerInstanceID {
			items = append(items, item)
		}
	}
	return items
}

// GetTopLevelItems returns all items not inside any container
func (inv *Inventory) GetTopLevelItems() []*ItemInstance {
	var items []*ItemInstance
	for _, item := range inv.Items {
		if item.ContainerID == "" {
			items = append(items, item)
		}
	}
	return items
}

// MoveItemToContainer moves an item into a container
func (inv *Inventory) MoveItemToContainer(instanceID, containerInstanceID string) error {
	item := inv.FindItem(instanceID)
	if item == nil {
		return fmt.Errorf("item instance %s not found in inventory", instanceID)
	}

	// Check if target container exists
	if containerInstanceID != "" {
		container := inv.FindItem(containerInstanceID)
		if container == nil {
			return fmt.Errorf("container instance %s not found in inventory", containerInstanceID)
		}
	}

	item.ContainerID = containerInstanceID
	inv.Update()
	return nil
}

// GetTotalQuantity returns the total quantity of a specific item type
func (inv *Inventory) GetTotalQuantity(itemID string) int {
	total := 0
	for _, item := range inv.Items {
		if item.ItemID == itemID {
			total += item.Quantity
		}
	}
	return total
}

// HasItem checks if the inventory contains at least the specified quantity of an item
func (inv *Inventory) HasItem(itemID string, quantity int) bool {
	return inv.GetTotalQuantity(itemID) >= quantity
}

// RemoveItemsByType removes a specific quantity of items by type
func (inv *Inventory) RemoveItemsByType(itemID string, quantityToRemove int) error {
	if !inv.HasItem(itemID, quantityToRemove) {
		return fmt.Errorf("not enough items of type %s (need: %d, have: %d)", 
			itemID, quantityToRemove, inv.GetTotalQuantity(itemID))
	}

	remaining := quantityToRemove
	for i := len(inv.Items) - 1; i >= 0 && remaining > 0; i-- {
		item := inv.Items[i]
		if item.ItemID != itemID {
			continue
		}

		if item.Quantity <= remaining {
			// Remove entire stack
			remaining -= item.Quantity
			inv.Items = append(inv.Items[:i], inv.Items[i+1:]...)
		} else {
			// Reduce stack quantity
			item.Quantity -= remaining
			remaining = 0
		}
	}

	inv.Update()
	return nil
}

// FilterItemsForWorld returns only items that can exist in the current world
func (inv *Inventory) FilterItemsForWorld(items map[string]*Item) []*ItemInstance {
	var validItems []*ItemInstance
	for _, instance := range inv.Items {
		if item, exists := items[instance.ItemID]; exists {
			if item.CanExistInWorld(inv.WorldID) {
				validItems = append(validItems, instance)
			}
		}
	}
	return validItems
}

// ChangeWorld updates the inventory's world and filters out world-limited items
func (inv *Inventory) ChangeWorld(newWorldID string, items map[string]*Item) []*ItemInstance {
	var removedItems []*ItemInstance
	var validItems []*ItemInstance

	for _, instance := range inv.Items {
		if item, exists := items[instance.ItemID]; exists {
			if item.CanExistInWorld(newWorldID) {
				instance.WorldID = newWorldID
				validItems = append(validItems, instance)
			} else {
				removedItems = append(removedItems, instance)
			}
		} else {
			// Item definition doesn't exist, remove it
			removedItems = append(removedItems, instance)
		}
	}

	inv.WorldID = newWorldID
	inv.Items = validItems
	inv.Update()
	
	return removedItems
}

// GetProperty gets a custom property value
func (inv *Inventory) GetProperty(key string) (interface{}, bool) {
	if inv.Properties == nil {
		return nil, false
	}
	value, exists := inv.Properties[key]
	return value, exists
}

// SetProperty sets a custom property value
func (inv *Inventory) SetProperty(key string, value interface{}) {
	if inv.Properties == nil {
		inv.Properties = make(map[string]interface{})
	}
	inv.Properties[key] = value
	inv.Update()
}

// InventoryManager manages inventory storage operations
type InventoryManager struct {
	storage *StorageManager
}

// NewInventoryManager creates a new inventory manager
func NewInventoryManager(storage *StorageManager) *InventoryManager {
	return &InventoryManager{
		storage: storage,
	}
}

// Save saves an inventory to storage
func (im *InventoryManager) Save(inventory *Inventory) error {
	inventory.Update()
	filename := fmt.Sprintf("%s.json", inventory.ID)
	return im.storage.SaveJSON(filepath.Join("inventories", filename), inventory)
}

// Load loads an inventory from storage by ID
func (im *InventoryManager) Load(inventoryID string) (*Inventory, error) {
	var inventory Inventory
	filename := fmt.Sprintf("%s.json", inventoryID)
	err := im.storage.LoadJSON(filepath.Join("inventories", filename), &inventory)
	if err != nil {
		return nil, fmt.Errorf("failed to load inventory %s: %w", inventoryID, err)
	}
	return &inventory, nil
}

// LoadByOwner loads an inventory by owner (player) ID
func (im *InventoryManager) LoadByOwner(ownerID string) (*Inventory, error) {
	files, err := im.storage.ListFiles("inventories")
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory files: %w", err)
	}

	for _, filename := range files {
		if filepath.Ext(filename) != ".json" {
			continue
		}

		var inventory Inventory
		err := im.storage.LoadJSON(filepath.Join("inventories", filename), &inventory)
		if err != nil {
			continue // Skip corrupted files
		}

		if inventory.OwnerID == ownerID {
			return &inventory, nil
		}
	}

	return nil, fmt.Errorf("inventory for owner %s not found", ownerID)
}

// Exists checks if an inventory exists by ID
func (im *InventoryManager) Exists(inventoryID string) bool {
	filename := fmt.Sprintf("%s.json", inventoryID)
	return im.storage.FileExists(filepath.Join("inventories", filename))
}

// ExistsByOwner checks if an inventory exists for a specific owner
func (im *InventoryManager) ExistsByOwner(ownerID string) bool {
	_, err := im.LoadByOwner(ownerID)
	return err == nil
}

// Delete deletes an inventory from storage
func (im *InventoryManager) Delete(inventoryID string) error {
	filename := fmt.Sprintf("%s.json", inventoryID)
	return im.storage.DeleteFile(filepath.Join("inventories", filename))
}

// CreateForPlayer creates a new inventory for a player
func (im *InventoryManager) CreateForPlayer(inventoryID, playerID, worldID string) (*Inventory, error) {
	if im.ExistsByOwner(playerID) {
		return nil, fmt.Errorf("inventory already exists for player %s", playerID)
	}

	inventory := NewInventory(inventoryID, playerID, worldID)
	err := im.Save(inventory)
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory for player %s: %w", playerID, err)
	}

	return inventory, nil
}

// ListAll lists all inventory IDs
func (im *InventoryManager) ListAll() ([]string, error) {
	files, err := im.storage.ListFiles("inventories")
	if err != nil {
		return nil, fmt.Errorf("failed to list inventory files: %w", err)
	}

	var inventoryIDs []string
	for _, filename := range files {
		if filepath.Ext(filename) == ".json" {
			inventoryID := filename[:len(filename)-5] // Remove .json extension
			inventoryIDs = append(inventoryIDs, inventoryID)
		}
	}

	return inventoryIDs, nil
}