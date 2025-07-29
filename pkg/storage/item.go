package storage

import (
	"fmt"
	"path/filepath"
	"time"
)

// Item represents an item in the game
type Item struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	
	// World limitations
	IsWorldLimited bool     `json:"is_world_limited"`
	AllowedWorlds  []string `json:"allowed_worlds,omitempty"`
	
	// Quantity limitations
	IsQuantityLimited bool `json:"is_quantity_limited"`
	MaxQuantity       int  `json:"max_quantity,omitempty"`
	CurrentQuantity   int  `json:"current_quantity"`
	
	// Container properties
	IsContainer        bool  `json:"is_container"`
	CanBeContained     bool  `json:"can_be_contained"`
	MaxContainerSize   int   `json:"max_container_size,omitempty"`
	ContainerSizeUnits string `json:"container_size_units,omitempty"` // "items", "weight", "volume", etc.
	
	// Item properties
	Size   int    `json:"size,omitempty"`        // Size when being contained
	Weight int    `json:"weight,omitempty"`      // Weight when being contained
	Volume int    `json:"volume,omitempty"`      // Volume when being contained
	
	// Custom properties (for game-specific data)
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// ItemInstance represents an instance of an item in the world/inventory
type ItemInstance struct {
	ID       string `json:"id"`
	ItemID   string `json:"item_id"`
	OwnerID  string `json:"owner_id,omitempty"`  // Player ID if in inventory
	WorldID  string `json:"world_id,omitempty"`  // World ID if in world
	RoomID   string `json:"room_id,omitempty"`   // Room ID if in room
	
	// Container information
	ContainerID string `json:"container_id,omitempty"` // ID of container item if inside one
	
	// Instance-specific properties
	Quantity   int                    `json:"quantity"`
	CreatedAt  time.Time              `json:"created_at"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// NewItem creates a new item with the given ID and name
func NewItem(id, name string) *Item {
	now := time.Now()
	return &Item{
		ID:               id,
		Name:             name,
		CreatedAt:        now,
		UpdatedAt:        now,
		CanBeContained:   true, // Default to can be contained
		CurrentQuantity:  0,
		Properties:       make(map[string]interface{}),
	}
}

// NewItemInstance creates a new item instance
func NewItemInstance(id, itemID string, quantity int) *ItemInstance {
	return &ItemInstance{
		ID:         id,
		ItemID:     itemID,
		Quantity:   quantity,
		CreatedAt:  time.Now(),
		Properties: make(map[string]interface{}),
	}
}

// Update updates the item's timestamp
func (i *Item) Update() {
	i.UpdatedAt = time.Now()
}

// CanExistInWorld checks if the item can exist in the specified world
func (i *Item) CanExistInWorld(worldID string) bool {
	if !i.IsWorldLimited {
		return true
	}
	
	for _, allowedWorld := range i.AllowedWorlds {
		if allowedWorld == worldID {
			return true
		}
	}
	
	return false
}

// CanCreateMore checks if more instances of this item can be created
func (i *Item) CanCreateMore(requestedQuantity int) bool {
	if !i.IsQuantityLimited {
		return true
	}
	
	return i.CurrentQuantity + requestedQuantity <= i.MaxQuantity
}

// AddToWorld adds the item to allowed worlds
func (i *Item) AddToWorld(worldID string) {
	if !i.IsWorldLimited {
		return
	}
	
	for _, world := range i.AllowedWorlds {
		if world == worldID {
			return // Already allowed
		}
	}
	
	i.AllowedWorlds = append(i.AllowedWorlds, worldID)
	i.Update()
}

// RemoveFromWorld removes the item from allowed worlds
func (i *Item) RemoveFromWorld(worldID string) {
	if !i.IsWorldLimited {
		return
	}
	
	for idx, world := range i.AllowedWorlds {
		if world == worldID {
			i.AllowedWorlds = append(i.AllowedWorlds[:idx], i.AllowedWorlds[idx+1:]...)
			break
		}
	}
	i.Update()
}

// GetProperty gets a custom property value
func (i *Item) GetProperty(key string) (interface{}, bool) {
	if i.Properties == nil {
		return nil, false
	}
	value, exists := i.Properties[key]
	return value, exists
}

// SetProperty sets a custom property value
func (i *Item) SetProperty(key string, value interface{}) {
	if i.Properties == nil {
		i.Properties = make(map[string]interface{})
	}
	i.Properties[key] = value
	i.Update()
}

// GetContainerCapacity returns the remaining capacity of the container
func (i *Item) GetContainerCapacity(currentContents []*ItemInstance, items map[string]*Item) int {
	if !i.IsContainer || i.MaxContainerSize == 0 {
		return 0
	}
	
	usedCapacity := 0
	for _, instance := range currentContents {
		if item, exists := items[instance.ItemID]; exists {
			switch i.ContainerSizeUnits {
			case "weight":
				usedCapacity += item.Weight * instance.Quantity
			case "volume":
				usedCapacity += item.Volume * instance.Quantity
			default: // "items" or unspecified
				usedCapacity += instance.Quantity
			}
		}
	}
	
	return i.MaxContainerSize - usedCapacity
}

// ItemManager manages item storage operations
type ItemManager struct {
	storage *StorageManager
}

// NewItemManager creates a new item manager
func NewItemManager(storage *StorageManager) *ItemManager {
	return &ItemManager{
		storage: storage,
	}
}

// Save saves an item to storage
func (im *ItemManager) Save(item *Item) error {
	item.Update()
	filename := fmt.Sprintf("%s.json", item.ID)
	return im.storage.SaveJSON(filepath.Join("items", filename), item)
}

// Load loads an item from storage by ID
func (im *ItemManager) Load(itemID string) (*Item, error) {
	var item Item
	filename := fmt.Sprintf("%s.json", itemID)
	err := im.storage.LoadJSON(filepath.Join("items", filename), &item)
	if err != nil {
		return nil, fmt.Errorf("failed to load item %s: %w", itemID, err)
	}
	return &item, nil
}

// Exists checks if an item exists by ID
func (im *ItemManager) Exists(itemID string) bool {
	filename := fmt.Sprintf("%s.json", itemID)
	return im.storage.FileExists(filepath.Join("items", filename))
}

// Delete deletes an item from storage
func (im *ItemManager) Delete(itemID string) error {
	filename := fmt.Sprintf("%s.json", itemID)
	return im.storage.DeleteFile(filepath.Join("items", filename))
}

// ListAll lists all item IDs
func (im *ItemManager) ListAll() ([]string, error) {
	files, err := im.storage.ListFiles("items")
	if err != nil {
		return nil, fmt.Errorf("failed to list item files: %w", err)
	}

	var itemIDs []string
	for _, filename := range files {
		if filepath.Ext(filename) == ".json" {
			itemID := filename[:len(filename)-5] // Remove .json extension
			itemIDs = append(itemIDs, itemID)
		}
	}

	return itemIDs, nil
}

// ListForWorld lists all items available in the specified world
func (im *ItemManager) ListForWorld(worldID string) ([]*Item, error) {
	itemIDs, err := im.ListAll()
	if err != nil {
		return nil, err
	}

	var items []*Item
	for _, itemID := range itemIDs {
		item, err := im.Load(itemID)
		if err != nil {
			continue // Skip corrupted items
		}

		if item.CanExistInWorld(worldID) {
			items = append(items, item)
		}
	}

	return items, nil
}

// IncrementQuantity increases the current quantity of an item
// Note: This method assumes it's called within an operation-level lock
func (im *ItemManager) IncrementQuantity(itemID string, amount int) error {
	item, err := im.Load(itemID)
	if err != nil {
		return err
	}

	if !item.CanCreateMore(amount) {
		return fmt.Errorf("cannot create %d more instances of item %s (current: %d, max: %d)", 
			amount, itemID, item.CurrentQuantity, item.MaxQuantity)
	}

	item.CurrentQuantity += amount
	return im.Save(item)
}

// DecrementQuantity decreases the current quantity of an item
// Note: This method assumes it's called within an operation-level lock
func (im *ItemManager) DecrementQuantity(itemID string, amount int) error {
	item, err := im.Load(itemID)
	if err != nil {
		return err
	}

	if item.CurrentQuantity < amount {
		return fmt.Errorf("cannot remove %d instances of item %s (current: %d)", 
			amount, itemID, item.CurrentQuantity)
	}

	item.CurrentQuantity -= amount
	return im.Save(item)
}