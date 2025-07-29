package world

import (
	"errors"
	"fmt"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// WumpusRoomType represents the type of room in the wumpus world
type WumpusRoomType string

const (
	WumpusRoomTypeInn    WumpusRoomType = "inn"
	WumpusRoomTypePortal WumpusRoomType = "portal"
	WumpusRoomTypeMaze   WumpusRoomType = "maze"
)

// WumpusWorldType represents the type of world
type WumpusWorldType string

const (
	WumpusWorldTypePermanent WumpusWorldType = "permanent"
	WumpusWorldTypeTemporary WumpusWorldType = "temporary"
)

// WumpusRoomData represents game-specific room data
type WumpusRoomData struct {
	RoomType       WumpusRoomType `json:"room_type"`
	InstanceID     string         `json:"instance_id,omitempty"`
	HasWumpus      bool           `json:"has_wumpus,omitempty"`
	HasPit         bool           `json:"has_pit,omitempty"`
	HasBreeze      bool           `json:"has_breeze,omitempty"`
	HasStench      bool           `json:"has_stench,omitempty"`
	MazePosition   *Position      `json:"maze_position,omitempty"`
	TempCleanup    string         `json:"temp_cleanup_at,omitempty"`
	ScentStrength  float64        `json:"scent_strength,omitempty"`
	ScentMessage   string         `json:"scent_message,omitempty"`
}

// WumpusWorldData represents game-specific world data
type WumpusWorldData struct {
	WorldType   WumpusWorldType `json:"world_type"`
	InstanceID  string          `json:"instance_id,omitempty"`
	CreatedBy   string          `json:"created_by,omitempty"`
	CleanupAt   string          `json:"cleanup_at,omitempty"`
	MazeConfig  *MazeConfig     `json:"maze_config,omitempty"`
}

// Position represents a position in the maze
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// MazeConfig represents configuration for a maze instance
type MazeConfig struct {
	RoomCount int    `json:"room_count"`
	WumpusRoom string `json:"wumpus_room"`
	StartRoom  string `json:"start_room"`
}

// SetRoomType sets the room type for a wumpus room
func SetRoomType(room *storage.Room, roomType WumpusRoomType) {
	roomData := GetWumpusRoomData(room)
	roomData.RoomType = roomType
	room.SetProperty("wumpus_room", roomData)
}

// GetRoomType gets the room type for a wumpus room
func GetRoomType(room *storage.Room) WumpusRoomType {
	roomData := GetWumpusRoomData(room)
	return roomData.RoomType
}

// SetInstanceID sets the instance ID for a room
func SetInstanceID(room *storage.Room, instanceID string) {
	roomData := GetWumpusRoomData(room)
	roomData.InstanceID = instanceID
	room.SetProperty("wumpus_room", roomData)
}

// GetInstanceID gets the instance ID for a room
func GetInstanceID(room *storage.Room) string {
	roomData := GetWumpusRoomData(room)
	return roomData.InstanceID
}

// MarkForCleanup marks a room for cleanup at a specific time
func MarkForCleanup(room *storage.Room, cleanupTime time.Time) {
	roomData := GetWumpusRoomData(room)
	roomData.TempCleanup = cleanupTime.Format(time.RFC3339)
	room.SetProperty("wumpus_room", roomData)
}

// IsTemporaryRoom checks if a room is temporary and should be cleaned up
func IsTemporaryRoom(room *storage.Room) bool {
	roomData := GetWumpusRoomData(room)
	return roomData.TempCleanup != ""
}

// GetWumpusRoomData retrieves wumpus room data from a room's properties
func GetWumpusRoomData(room *storage.Room) WumpusRoomData {
	roomData, exists := room.GetProperty("wumpus_room")
	if !exists {
		return WumpusRoomData{}
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := roomData.(type) {
	case WumpusRoomData:
		return v
	case map[string]interface{}:
		data := WumpusRoomData{}
		
		if roomType, ok := v["room_type"].(string); ok {
			data.RoomType = WumpusRoomType(roomType)
		}
		if instanceID, ok := v["instance_id"].(string); ok {
			data.InstanceID = instanceID
		}
		if hasWumpus, ok := v["has_wumpus"].(bool); ok {
			data.HasWumpus = hasWumpus
		}
		if hasPit, ok := v["has_pit"].(bool); ok {
			data.HasPit = hasPit
		}
		if hasBreeze, ok := v["has_breeze"].(bool); ok {
			data.HasBreeze = hasBreeze
		}
		if tempCleanup, ok := v["temp_cleanup_at"].(string); ok {
			data.TempCleanup = tempCleanup
		}
		
		// Handle maze position
		if mazePos, ok := v["maze_position"].(map[string]interface{}); ok {
			position := &Position{}
			if x, ok := mazePos["x"].(float64); ok {
				position.X = int(x)
			}
			if y, ok := mazePos["y"].(float64); ok {
				position.Y = int(y)
			}
			data.MazePosition = position
		}
		
		return data
	default:
		return WumpusRoomData{}
	}
}

// SetWumpusWorldData sets wumpus world data for a world
func SetWumpusWorldData(world *storage.World, worldData WumpusWorldData) {
	world.SetProperty("wumpus_world", worldData)
}

// GetWumpusWorldData retrieves wumpus world data from a world's properties
func GetWumpusWorldData(world *storage.World) WumpusWorldData {
	worldData, exists := world.GetProperty("wumpus_world")
	if !exists {
		return WumpusWorldData{}
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := worldData.(type) {
	case WumpusWorldData:
		return v
	case map[string]interface{}:
		data := WumpusWorldData{}
		
		if worldType, ok := v["world_type"].(string); ok {
			data.WorldType = WumpusWorldType(worldType)
		}
		if instanceID, ok := v["instance_id"].(string); ok {
			data.InstanceID = instanceID
		}
		if createdBy, ok := v["created_by"].(string); ok {
			data.CreatedBy = createdBy
		}
		if cleanupAt, ok := v["cleanup_at"].(string); ok {
			data.CleanupAt = cleanupAt
		}
		
		// Handle maze config
		if mazeConfig, ok := v["maze_config"].(map[string]interface{}); ok {
			config := &MazeConfig{}
			if roomCount, ok := mazeConfig["room_count"].(float64); ok {
				config.RoomCount = int(roomCount)
			}
			if wumpusRoom, ok := mazeConfig["wumpus_room"].(string); ok {
				config.WumpusRoom = wumpusRoom
			}
			if startRoom, ok := mazeConfig["start_room"].(string); ok {
				config.StartRoom = startRoom
			}
			data.MazeConfig = config
		}
		
		return data
	default:
		return WumpusWorldData{}
	}
}

// CreateMainWorld creates the main permanent world with Inn and Portal
func CreateMainWorld(storageMgr *storage.Manager) (*storage.World, error) {
	// Create the main world
	world, err := storageMgr.CreateNewWorld("Wumpus Hunt World", "The main world of the Wumpus Hunt, containing The Inn and The Portal.")
	if err != nil {
		return nil, fmt.Errorf("failed to create main world: %w", err)
	}
	
	// Set world properties
	worldData := WumpusWorldData{
		WorldType: WumpusWorldTypePermanent,
	}
	SetWumpusWorldData(world, worldData)
	
	// Save world with properties
	if err := storageMgr.Worlds().SaveWorld(world); err != nil {
		return nil, fmt.Errorf("failed to save main world: %w", err)
	}
	
	// Create The Inn room
	innRoom, err := storageMgr.CreateNewRoom(world.ID, "The Inn", 
		"A warm and welcoming inn where adventurers gather before embarking on dangerous quests. " +
		"The air is filled with the scent of hearty stew and the sound of laughter. " +
		"A cozy fireplace crackles in the corner, casting dancing shadows on the walls. " +
		"To the north, you can see a mysterious portal glowing with otherworldly energy.")
	if err != nil {
		return nil, fmt.Errorf("failed to create Inn room: %w", err)
	}
	
	// Set inn room properties
	SetRoomType(innRoom, WumpusRoomTypeInn)
	
	// Create The Portal room
	portalRoom, err := storageMgr.CreateNewRoom(world.ID, "The Portal",
		"A mysterious portal crackling with arcane energy, leading to unknown dangers. " +
		"The air hums with magical power, and you can feel the pull of adventure beyond. " +
		"Ancient runes are carved into the stone archway, glowing with a soft blue light. " +
		"Through the shimmering gateway, you can barely make out the entrance to a dark maze.")
	if err != nil {
		return nil, fmt.Errorf("failed to create Portal room: %w", err)
	}
	
	// Set portal room properties
	SetRoomType(portalRoom, WumpusRoomTypePortal)
	
	// Create directional connections between rooms
	innRoom.AddExit("north", world.ID, portalRoom.ID)
	portalRoom.AddExit("south", world.ID, innRoom.ID)
	
	// Save rooms with their connections
	if err := storageMgr.Worlds().SaveRoom(innRoom); err != nil {
		return nil, fmt.Errorf("failed to save Inn room: %w", err)
	}
	
	if err := storageMgr.Worlds().SaveRoom(portalRoom); err != nil {
		return nil, fmt.Errorf("failed to save Portal room: %w", err)
	}
	
	// Set the default room to the Inn
	world.DefaultRoomID = innRoom.ID
	if err := storageMgr.Worlds().SaveWorld(world); err != nil {
		return nil, fmt.Errorf("failed to update world with default room: %w", err)
	}
	
	return world, nil
}

// GetMainWorld retrieves the main permanent world
func GetMainWorld(storageMgr *storage.Manager) (*storage.World, error) {
	// Get list of all worlds and find the main one
	worlds, err := storageMgr.Worlds().GetActiveWorlds()
	if err != nil {
		return nil, fmt.Errorf("failed to get active worlds: %w", err)
	}
	
	// Look for the main world by checking properties
	for _, world := range worlds {
		worldData := GetWumpusWorldData(world)
		if worldData.WorldType == WumpusWorldTypePermanent {
			return world, nil
		}
	}
	
	return nil, errors.New("main world not found - needs to be created")
}

// GetInnRoom retrieves The Inn room from the main world
func GetInnRoom(storageMgr *storage.Manager, worldID string) (*storage.Room, error) {
	// Get all rooms in the world
	roomIDs, err := storageMgr.Worlds().ListRooms(worldID)
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms: %w", err)
	}
	
	// Find the Inn room by checking properties
	for _, roomID := range roomIDs {
		room, err := storageMgr.Worlds().LoadRoom(worldID, roomID)
		if err != nil {
			continue
		}
		
		if GetRoomType(room) == WumpusRoomTypeInn {
			return room, nil
		}
	}
	
	return nil, errors.New("inn room not found")
}

// GetPortalRoom retrieves The Portal room from the main world
func GetPortalRoom(storageMgr *storage.Manager, worldID string) (*storage.Room, error) {
	// Get all rooms in the world
	roomIDs, err := storageMgr.Worlds().ListRooms(worldID)
	if err != nil {
		return nil, fmt.Errorf("failed to list rooms: %w", err)
	}
	
	// Find the Portal room by checking properties
	for _, roomID := range roomIDs {
		room, err := storageMgr.Worlds().LoadRoom(worldID, roomID)
		if err != nil {
			continue
		}
		
		if GetRoomType(room) == WumpusRoomTypePortal {
			return room, nil
		}
	}
	
	return nil, errors.New("portal room not found")
}

// CleanupTempRooms removes expired temporary rooms
func CleanupTempRooms(storageMgr *storage.Manager) error {
	// Get all active worlds
	worlds, err := storageMgr.Worlds().GetActiveWorlds()
	if err != nil {
		return fmt.Errorf("failed to get active worlds: %w", err)
	}
	
	// Check each world for temporary rooms that need cleanup
	for _, world := range worlds {
		worldData := GetWumpusWorldData(world)
		if worldData.WorldType == WumpusWorldTypeTemporary {
			// Check if this world should be cleaned up
			if worldData.CleanupAt != "" {
				if cleanupTime, err := time.Parse(time.RFC3339, worldData.CleanupAt); err == nil {
					if time.Now().After(cleanupTime) {
						// Delete the entire temporary world
						if err := storageMgr.Worlds().DeleteWorld(world.ID); err != nil {
							return fmt.Errorf("failed to delete temporary world %s: %w", world.ID, err)
						}
					}
				}
			}
		} else {
			// For permanent worlds, check individual rooms for cleanup
			roomIDs, err := storageMgr.Worlds().ListRooms(world.ID)
			if err != nil {
				continue
			}
			
			for _, roomID := range roomIDs {
				room, err := storageMgr.Worlds().LoadRoom(world.ID, roomID)
				if err != nil {
					continue
				}
				
				if IsTemporaryRoom(room) {
					roomData := GetWumpusRoomData(room)
					if cleanupTime, err := time.Parse(time.RFC3339, roomData.TempCleanup); err == nil {
						if time.Now().After(cleanupTime) {
							// Delete the temporary room
							if err := storageMgr.Worlds().DeleteRoom(world.ID, roomID); err != nil {
								return fmt.Errorf("failed to delete temporary room %s: %w", roomID, err)
							}
						}
					}
				}
			}
		}
	}
	
	return nil
}

// IsWumpusRoom checks if a room is a wumpus game room
func IsWumpusRoom(room *storage.Room) bool {
	_, exists := room.GetProperty("wumpus_room")
	return exists
}

// ValidateWorldStructure validates that the world structure is correct
func ValidateWorldStructure(world *storage.World, storageMgr *storage.Manager) error {
	worldData := GetWumpusWorldData(world)
	
	// Check if it's a permanent world
	if worldData.WorldType != WumpusWorldTypePermanent {
		return errors.New("world is not a permanent world")
	}
	
	// Get all rooms in the world
	roomIDs, err := storageMgr.Worlds().ListRooms(world.ID)
	if err != nil {
		return fmt.Errorf("failed to list rooms: %w", err)
	}
	
	// Check if world has the required rooms
	if len(roomIDs) < 2 {
		return errors.New("world must have at least 2 rooms (Inn and Portal)")
	}
	
	// Validate that required rooms exist
	innFound := false
	portalFound := false
	
	for _, roomID := range roomIDs {
		room, err := storageMgr.Worlds().LoadRoom(world.ID, roomID)
		if err != nil {
			continue
		}
		
		roomType := GetRoomType(room)
		switch roomType {
		case WumpusRoomTypeInn:
			innFound = true
		case WumpusRoomTypePortal:
			portalFound = true
		}
	}
	
	if !innFound {
		return errors.New("world is missing The Inn room")
	}
	
	if !portalFound {
		return errors.New("world is missing The Portal room")
	}
	
	return nil
}