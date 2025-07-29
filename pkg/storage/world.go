package storage

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// Exit represents an exit from one room to another
type Exit struct {
	Direction   string `json:"direction"`   // "north", "south", "east", "west", etc.
	TargetWorldID string `json:"target_world_id"`
	TargetRoomID  string `json:"target_room_id"`
	Description string `json:"description,omitempty"`
	IsHidden    bool   `json:"is_hidden,omitempty"`
	
	// Custom properties for game-specific data
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Room represents a room in the game world
type Room struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	WorldID     string            `json:"world_id"`
	
	// Exit system with inlet approval
	Exits         []*Exit  `json:"exits"`
	AllowedInlets []string `json:"allowed_inlets"` // Room IDs that can connect to this room
	
	// Items in the room
	Items []*ItemInstance `json:"items"`
	
	// Room state
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// Custom properties for game-specific data
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// World represents a game world containing multiple rooms
type World struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	
	// World configuration
	DefaultRoomID string `json:"default_room_id,omitempty"` // Starting room for new players
	IsActive      bool   `json:"is_active"`
	
	// Custom properties for game-specific data
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// NewRoom creates a new room
func NewRoom(id, worldID, name string) *Room {
	now := time.Now()
	return &Room{
		ID:            id,
		Name:          name,
		WorldID:       worldID,
		Exits:         make([]*Exit, 0),
		AllowedInlets: make([]string, 0),
		Items:         make([]*ItemInstance, 0),
		CreatedAt:     now,
		UpdatedAt:     now,
		Properties:    make(map[string]interface{}),
	}
}

// NewWorld creates a new world
func NewWorld(id, name string) *World {
	now := time.Now()
	return &World{
		ID:         id,
		Name:       name,
		CreatedAt:  now,
		UpdatedAt:  now,
		IsActive:   true,
		Properties: make(map[string]interface{}),
	}
}

// Update updates the room's timestamp
func (r *Room) Update() {
	r.UpdatedAt = time.Now()
}

// Update updates the world's timestamp
func (w *World) Update() {
	w.UpdatedAt = time.Now()
}

// AddExit adds an exit to the room
func (r *Room) AddExit(direction, targetWorldID, targetRoomID string) {
	exit := &Exit{
		Direction:     direction,
		TargetWorldID: targetWorldID,
		TargetRoomID:  targetRoomID,
		Properties:    make(map[string]interface{}),
	}
	r.Exits = append(r.Exits, exit)
	r.Update()
}

// RemoveExit removes an exit by direction
func (r *Room) RemoveExit(direction string) bool {
	for i, exit := range r.Exits {
		if exit.Direction == direction {
			r.Exits = append(r.Exits[:i], r.Exits[i+1:]...)
			r.Update()
			return true
		}
	}
	return false
}

// GetExit gets an exit by direction
func (r *Room) GetExit(direction string) *Exit {
	for _, exit := range r.Exits {
		if exit.Direction == direction {
			return exit
		}
	}
	return nil
}

// AddAllowedInlet adds a room ID to the allowed inlets list
func (r *Room) AddAllowedInlet(roomID string) {
	for _, inlet := range r.AllowedInlets {
		if inlet == roomID {
			return // Already allowed
		}
	}
	r.AllowedInlets = append(r.AllowedInlets, roomID)
	r.Update()
}

// RemoveAllowedInlet removes a room ID from the allowed inlets list
func (r *Room) RemoveAllowedInlet(roomID string) bool {
	for i, inlet := range r.AllowedInlets {
		if inlet == roomID {
			r.AllowedInlets = append(r.AllowedInlets[:i], r.AllowedInlets[i+1:]...)
			r.Update()
			return true
		}
	}
	return false
}

// IsInletAllowed checks if a room is allowed to connect to this room
func (r *Room) IsInletAllowed(roomID string) bool {
	for _, inlet := range r.AllowedInlets {
		if inlet == roomID {
			return true
		}
	}
	return false
}

// AddItem adds an item to the room
func (r *Room) AddItem(instance *ItemInstance) {
	instance.WorldID = r.WorldID
	instance.RoomID = r.ID
	instance.OwnerID = "" // Items in rooms don't have owners
	r.Items = append(r.Items, instance)
	r.Update()
}

// RemoveItem removes an item from the room by instance ID
func (r *Room) RemoveItem(instanceID string) bool {
	for i, item := range r.Items {
		if item.ID == instanceID {
			r.Items = append(r.Items[:i], r.Items[i+1:]...)
			r.Update()
			return true
		}
	}
	return false
}

// FindItem finds an item in the room by instance ID
func (r *Room) FindItem(instanceID string) *ItemInstance {
	for _, item := range r.Items {
		if item.ID == instanceID {
			return item
		}
	}
	return nil
}

// GetProperty gets a custom property value from room
func (r *Room) GetProperty(key string) (interface{}, bool) {
	if r.Properties == nil {
		return nil, false
	}
	value, exists := r.Properties[key]
	return value, exists
}

// SetProperty sets a custom property value for room
func (r *Room) SetProperty(key string, value interface{}) {
	if r.Properties == nil {
		r.Properties = make(map[string]interface{})
	}
	r.Properties[key] = value
	r.Update()
}

// GetProperty gets a custom property value from world
func (w *World) GetProperty(key string) (interface{}, bool) {
	if w.Properties == nil {
		return nil, false
	}
	value, exists := w.Properties[key]
	return value, exists
}

// SetProperty sets a custom property value for world
func (w *World) SetProperty(key string, value interface{}) {
	if w.Properties == nil {
		w.Properties = make(map[string]interface{})
	}
	w.Properties[key] = value
	w.Update()
}

// WorldManager manages world storage operations
type WorldManager struct {
	storage *StorageManager
	mutex   sync.RWMutex // Protects compound operations like CreateExitWithValidation
}

// NewWorldManager creates a new world manager
func NewWorldManager(storage *StorageManager) *WorldManager {
	return &WorldManager{
		storage: storage,
	}
}

// SaveWorld saves a world to storage
func (wm *WorldManager) SaveWorld(world *World) error {
	world.Update()
	filename := fmt.Sprintf("%s.json", world.ID)
	return wm.storage.SaveJSON(filepath.Join("worlds", filename), world)
}

// LoadWorld loads a world from storage by ID
func (wm *WorldManager) LoadWorld(worldID string) (*World, error) {
	var world World
	filename := fmt.Sprintf("%s.json", worldID)
	err := wm.storage.LoadJSON(filepath.Join("worlds", filename), &world)
	if err != nil {
		return nil, fmt.Errorf("failed to load world %s: %w", worldID, err)
	}
	return &world, nil
}

// SaveRoom saves a room to storage
func (wm *WorldManager) SaveRoom(room *Room) error {
	room.Update()
	worldDir := filepath.Join("worlds", room.WorldID)
	filename := fmt.Sprintf("%s.json", room.ID)
	return wm.storage.SaveJSON(filepath.Join(worldDir, filename), room)
}

// LoadRoom loads a room from storage by world ID and room ID
func (wm *WorldManager) LoadRoom(worldID, roomID string) (*Room, error) {
	var room Room
	worldDir := filepath.Join("worlds", worldID)
	filename := fmt.Sprintf("%s.json", roomID)
	err := wm.storage.LoadJSON(filepath.Join(worldDir, filename), &room)
	if err != nil {
		return nil, fmt.Errorf("failed to load room %s in world %s: %w", roomID, worldID, err)
	}
	return &room, nil
}

// WorldExists checks if a world exists by ID
func (wm *WorldManager) WorldExists(worldID string) bool {
	filename := fmt.Sprintf("%s.json", worldID)
	return wm.storage.FileExists(filepath.Join("worlds", filename))
}

// RoomExists checks if a room exists by world ID and room ID
func (wm *WorldManager) RoomExists(worldID, roomID string) bool {
	worldDir := filepath.Join("worlds", worldID)
	filename := fmt.Sprintf("%s.json", roomID)
	return wm.storage.FileExists(filepath.Join(worldDir, filename))
}

// DeleteWorld deletes a world and all its rooms
func (wm *WorldManager) DeleteWorld(worldID string) error {
	// First, delete all rooms in the world
	rooms, err := wm.ListRooms(worldID)
	if err == nil {
		for _, roomID := range rooms {
			wm.DeleteRoom(worldID, roomID) // Ignore errors for individual rooms
		}
	}

	// Delete the world file
	filename := fmt.Sprintf("%s.json", worldID)
	return wm.storage.DeleteFile(filepath.Join("worlds", filename))
}

// DeleteRoom deletes a room from a world
func (wm *WorldManager) DeleteRoom(worldID, roomID string) error {
	worldDir := filepath.Join("worlds", worldID)
	filename := fmt.Sprintf("%s.json", roomID)
	return wm.storage.DeleteFile(filepath.Join(worldDir, filename))
}

// ListWorlds lists all world IDs
func (wm *WorldManager) ListWorlds() ([]string, error) {
	files, err := wm.storage.ListFiles("worlds")
	if err != nil {
		return nil, fmt.Errorf("failed to list world files: %w", err)
	}

	var worldIDs []string
	for _, filename := range files {
		if filepath.Ext(filename) == ".json" {
			worldID := filename[:len(filename)-5] // Remove .json extension
			worldIDs = append(worldIDs, worldID)
		}
	}

	return worldIDs, nil
}

// ListRooms lists all room IDs in a world
func (wm *WorldManager) ListRooms(worldID string) ([]string, error) {
	worldDir := filepath.Join("worlds", worldID)
	files, err := wm.storage.ListFiles(worldDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list room files for world %s: %w", worldID, err)
	}

	var roomIDs []string
	for _, filename := range files {
		if filepath.Ext(filename) == ".json" {
			roomID := filename[:len(filename)-5] // Remove .json extension
			roomIDs = append(roomIDs, roomID)
		}
	}

	return roomIDs, nil
}

// ValidateExit validates that an exit is allowed by checking the target room's allowed inlets
func (wm *WorldManager) ValidateExit(sourceWorldID, sourceRoomID, targetWorldID, targetRoomID string) error {
	// Load target room to check allowed inlets
	targetRoom, err := wm.LoadRoom(targetWorldID, targetRoomID)
	if err != nil {
		return fmt.Errorf("target room does not exist: %w", err)
	}

	// Check if the source room is allowed to connect
	if !targetRoom.IsInletAllowed(sourceRoomID) {
		return fmt.Errorf("room %s in world %s does not allow connections from room %s in world %s", 
			targetRoomID, targetWorldID, sourceRoomID, sourceWorldID)
	}

	return nil
}

// CreateExitWithValidation creates an exit between rooms with validation
func (wm *WorldManager) CreateExitWithValidation(sourceWorldID, sourceRoomID, direction, targetWorldID, targetRoomID string) error {
	// Lock to prevent race conditions during the load-modify-save operation
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	// Validate that the exit is allowed
	if err := wm.ValidateExit(sourceWorldID, sourceRoomID, targetWorldID, targetRoomID); err != nil {
		return fmt.Errorf("exit validation failed: %w", err)
	}

	// Load source room
	sourceRoom, err := wm.LoadRoom(sourceWorldID, sourceRoomID)
	if err != nil {
		return fmt.Errorf("source room does not exist: %w", err)
	}

	// Add the exit
	sourceRoom.AddExit(direction, targetWorldID, targetRoomID)

	// Save the updated room
	return wm.SaveRoom(sourceRoom)
}

// GetActiveWorlds returns all active worlds
func (wm *WorldManager) GetActiveWorlds() ([]*World, error) {
	worldIDs, err := wm.ListWorlds()
	if err != nil {
		return nil, err
	}

	var activeWorlds []*World
	for _, worldID := range worldIDs {
		world, err := wm.LoadWorld(worldID)
		if err != nil {
			continue // Skip corrupted worlds
		}

		if world.IsActive {
			activeWorlds = append(activeWorlds, world)
		}
	}

	return activeWorlds, nil
}