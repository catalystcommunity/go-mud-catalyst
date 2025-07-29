package storage

import (
	"fmt"
	"sync"

	"github.com/catalystcommunity/muddycore/pkg/ids"
)

// Manager provides a unified interface to all storage operations
type Manager struct {
	storage           *StorageManager
	playerManager     *PlayerManager
	itemManager       *ItemManager
	inventoryManager  *InventoryManager
	worldManager      *WorldManager
	containerOps      *ContainerOperations
	sessionManager    *SessionManager
	roleManager       *RoleManager
	operationMutex    sync.RWMutex // High-level lock for compound operations
}

// NewManager creates a new unified storage manager
func NewManager() *Manager {
	storage := NewStorageManager()
	return NewManagerWithConfig(storage.GetConfig())
}

// NewManagerWithConfig creates a new unified storage manager with custom config
func NewManagerWithConfig(config *StorageConfig) *Manager {
	storage := NewStorageManagerWithConfig(config)
	playerManager := NewPlayerManager(storage)
	itemManager := NewItemManager(storage)
	inventoryManager := NewInventoryManager(storage)
	worldManager := NewWorldManager(storage)
	containerOps := NewContainerOperations(itemManager, inventoryManager)
	roleManager := NewRoleManager(storage)

	manager := &Manager{
		storage:          storage,
		playerManager:    playerManager,
		itemManager:      itemManager,
		inventoryManager: inventoryManager,
		worldManager:     worldManager,
		containerOps:     containerOps,
		roleManager:      roleManager,
	}
	
	// Initialize session manager with reference to this manager
	manager.sessionManager = NewSessionManager(manager)

	return manager
}

// Initialize initializes the storage system
func (m *Manager) Initialize() error {
	return m.storage.Initialize()
}

// GetConfig returns the storage configuration
func (m *Manager) GetConfig() *StorageConfig {
	return m.storage.GetConfig()
}

// UpdateConfig updates the storage configuration
func (m *Manager) UpdateConfig(config *StorageConfig) error {
	return m.storage.UpdateConfig(config)
}

// Player operations
func (m *Manager) Players() *PlayerManager {
	return m.playerManager
}

// Item operations
func (m *Manager) Items() *ItemManager {
	return m.itemManager
}

// Inventory operations
func (m *Manager) Inventories() *InventoryManager {
	return m.inventoryManager
}

// World operations
func (m *Manager) Worlds() *WorldManager {
	return m.worldManager
}

// Container operations
func (m *Manager) Containers() *ContainerOperations {
	return m.containerOps
}

// Session operations
func (m *Manager) Sessions() *SessionManager {
	return m.sessionManager
}

// Role operations
func (m *Manager) Roles() *RoleManager {
	return m.roleManager
}

// CreateNewPlayer creates a new player with inventory
func (m *Manager) CreateNewPlayer(username, email string) (*Player, *Inventory, error) {
	// Validate username
	if username == "" {
		return nil, nil, fmt.Errorf("username cannot be empty")
	}
	
	// Use operation-level lock for atomic username check and creation
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	// Check if username already exists
	if m.playerManager.ExistsByUsername(username) {
		return nil, nil, fmt.Errorf("username %s already exists", username)
	}

	// Create player
	playerID := ids.NewEntityID()
	player := NewPlayer(playerID, username)
	player.Email = email

	// Create inventory
	inventoryID := ids.NewEntityID()
	inventory := NewInventory(inventoryID, playerID, "")

	// Save both
	if err := m.playerManager.Save(player); err != nil {
		return nil, nil, fmt.Errorf("failed to save player: %w", err)
	}

	if err := m.inventoryManager.Save(inventory); err != nil {
		// Rollback player creation
		m.playerManager.Delete(playerID)
		return nil, nil, fmt.Errorf("failed to save inventory: %w", err)
	}

	return player, inventory, nil
}

// CreateNewItem creates a new item definition
func (m *Manager) CreateNewItem(name, description string) (*Item, error) {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	itemID := ids.NewEntityID()
	item := NewItem(itemID, name)
	item.Description = description

	if err := m.itemManager.Save(item); err != nil {
		return nil, fmt.Errorf("failed to save item: %w", err)
	}

	return item, nil
}

// CreateItemInstance creates a new instance of an item
func (m *Manager) CreateItemInstance(itemID string, quantity int) (*ItemInstance, error) {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	// Verify item exists
	item, err := m.itemManager.Load(itemID)
	if err != nil {
		return nil, fmt.Errorf("item %s does not exist: %w", itemID, err)
	}

	// Check quantity limits
	if !item.CanCreateMore(quantity) {
		return nil, fmt.Errorf("cannot create %d instances of item %s (current: %d, max: %d)",
			quantity, itemID, item.CurrentQuantity, item.MaxQuantity)
	}

	// Create instance
	instanceID := ids.NewEntityID()
	instance := NewItemInstance(instanceID, itemID, quantity)

	// Update item quantity
	if err := m.itemManager.IncrementQuantity(itemID, quantity); err != nil {
		return nil, fmt.Errorf("failed to update item quantity: %w", err)
	}

	return instance, nil
}

// CreateNewWorld creates a new world
func (m *Manager) CreateNewWorld(name, description string) (*World, error) {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	worldID := ids.NewEntityID()
	world := NewWorld(worldID, name)
	world.Description = description

	if err := m.worldManager.SaveWorld(world); err != nil {
		return nil, fmt.Errorf("failed to save world: %w", err)
	}

	return world, nil
}

// CreateNewRoom creates a new room in a world
func (m *Manager) CreateNewRoom(worldID, name, description string) (*Room, error) {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	// Verify world exists
	if !m.worldManager.WorldExists(worldID) {
		return nil, fmt.Errorf("world %s does not exist", worldID)
	}

	roomID := ids.NewEntityID()
	room := NewRoom(roomID, worldID, name)
	room.Description = description

	if err := m.worldManager.SaveRoom(room); err != nil {
		return nil, fmt.Errorf("failed to save room: %w", err)
	}

	return room, nil
}

// GiveItemToPlayer adds an item to a player's inventory
func (m *Manager) GiveItemToPlayer(playerID, itemID string, quantity int) error {
	// Use operation-level lock for atomic operation
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	// Load player inventory
	inventory, err := m.inventoryManager.LoadByOwner(playerID)
	if err != nil {
		return fmt.Errorf("failed to load player inventory: %w", err)
	}

	// Create item instance directly (avoid nested lock)
	item, err := m.itemManager.Load(itemID)
	if err != nil {
		return fmt.Errorf("item %s does not exist: %w", itemID, err)
	}

	// Check quantity limits
	if !item.CanCreateMore(quantity) {
		return fmt.Errorf("cannot create %d instances of item %s (current: %d, max: %d)",
			quantity, itemID, item.CurrentQuantity, item.MaxQuantity)
	}

	// Create instance
	instanceID := ids.NewEntityID()
	instance := NewItemInstance(instanceID, itemID, quantity)

	// Update item quantity
	if err := m.itemManager.IncrementQuantity(itemID, quantity); err != nil {
		return fmt.Errorf("failed to update item quantity: %w", err)
	}

	// Add to inventory
	inventory.AddItem(instance)

	// Save inventory
	if err := m.inventoryManager.Save(inventory); err != nil {
		// Rollback item quantity
		m.itemManager.DecrementQuantity(itemID, quantity)
		return fmt.Errorf("failed to save inventory: %w", err)
	}

	return nil
}

// TakeItemFromPlayer removes an item from a player's inventory
func (m *Manager) TakeItemFromPlayer(playerID, instanceID string) error {
	// Use operation-level lock for atomic operation
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	// Load player inventory
	inventory, err := m.inventoryManager.LoadByOwner(playerID)
	if err != nil {
		return fmt.Errorf("failed to load player inventory: %w", err)
	}

	// Find the item instance
	instance := inventory.FindItem(instanceID)
	if instance == nil {
		return fmt.Errorf("item instance %s not found in player's inventory", instanceID)
	}

	// Remove from inventory
	if !inventory.RemoveItem(instanceID) {
		return fmt.Errorf("failed to remove item from inventory")
	}

	// Update item quantity
	if err := m.itemManager.DecrementQuantity(instance.ItemID, instance.Quantity); err != nil {
		// Rollback inventory change
		inventory.AddItem(instance)
		m.inventoryManager.Save(inventory)
		return fmt.Errorf("failed to update item quantity: %w", err)
	}

	// Save inventory
	if err := m.inventoryManager.Save(inventory); err != nil {
		return fmt.Errorf("failed to save inventory: %w", err)
	}

	return nil
}

// MovePlayerToRoom moves a player to a specific room
func (m *Manager) MovePlayerToRoom(playerID, worldID, roomID string) error {
	// Use operation-level lock for atomic operation
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	// Verify room exists
	if !m.worldManager.RoomExists(worldID, roomID) {
		return fmt.Errorf("room %s does not exist in world %s", roomID, worldID)
	}

	// Load player
	player, err := m.playerManager.Load(playerID)
	if err != nil {
		return fmt.Errorf("failed to load player: %w", err)
	}

	// Check if player is banned
	if player.IsBanned {
		return fmt.Errorf("player %s is banned and cannot be moved", playerID)
	}

	// Store old world ID to check if it changed
	oldWorldID := player.CurrentWorldID

	// Update player location
	player.SetLocation(worldID, roomID)

	// Save player
	if err := m.playerManager.Save(player); err != nil {
		return fmt.Errorf("failed to save player: %w", err)
	}

	// Update inventory world if it changed
	if oldWorldID != worldID {
		inventory, err := m.inventoryManager.LoadByOwner(playerID)
		if err == nil { // Don't fail if inventory doesn't exist
			// Load all item definitions to check world limitations
			itemDefs := make(map[string]*Item)
			for _, instance := range inventory.Items {
				if _, exists := itemDefs[instance.ItemID]; !exists {
					if item, err := m.itemManager.Load(instance.ItemID); err == nil {
						itemDefs[instance.ItemID] = item
					}
				}
			}
			
			removedItems := inventory.ChangeWorld(worldID, itemDefs)
			if len(removedItems) > 0 {
				// Log or handle removed items
				fmt.Printf("Removed %d world-limited items when moving to world %s\n", len(removedItems), worldID)
			}
			m.inventoryManager.Save(inventory)
		}
	}

	return nil
}

// IncrementItemQuantity increases the current quantity of an item
func (m *Manager) IncrementItemQuantity(itemID string, amount int) error {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	return m.itemManager.IncrementQuantity(itemID, amount)
}

// DecrementItemQuantity decreases the current quantity of an item
func (m *Manager) DecrementItemQuantity(itemID string, amount int) error {
	m.operationMutex.Lock()
	defer m.operationMutex.Unlock()
	
	return m.itemManager.DecrementQuantity(itemID, amount)
}

// BanPlayer bans a player
func (m *Manager) BanPlayer(playerID, reason, adminID string) error {
	player, err := m.playerManager.Load(playerID)
	if err != nil {
		return fmt.Errorf("failed to load player: %w", err)
	}

	player.Ban(reason, adminID)
	return m.playerManager.Save(player)
}

// UnbanPlayer unbans a player
func (m *Manager) UnbanPlayer(playerID string) error {
	player, err := m.playerManager.Load(playerID)
	if err != nil {
		return fmt.Errorf("failed to load player: %w", err)
	}

	player.Unban()
	return m.playerManager.Save(player)
}