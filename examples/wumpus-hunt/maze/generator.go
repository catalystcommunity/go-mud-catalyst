package maze

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

// MazeConfig represents the configuration for generating a maze
type MazeConfig struct {
	RoomCount   int    `json:"room_count"`
	MinRooms    int    `json:"min_rooms"`
	MaxRooms    int    `json:"max_rooms"`
	InstanceID  string `json:"instance_id"`
	CreatedBy   string `json:"created_by"`
	CleanupTime time.Duration `json:"cleanup_time"`
}

// MazeRoom represents a room in the generated maze
type MazeRoom struct {
	Room     *storage.Room
	Position world.Position
	HasWumpus bool
	HasPit    bool
	HasBreeze bool
	IsStart   bool
}

// MazeInstance represents a complete generated maze
type MazeInstance struct {
	World      *storage.World
	Rooms      []*MazeRoom
	StartRoom  *MazeRoom
	WumpusRoom *MazeRoom
	Config     MazeConfig
}

// DefaultMazeConfig returns a default maze configuration
func DefaultMazeConfig() MazeConfig {
	return MazeConfig{
		RoomCount:   12,
		MinRooms:    8,
		MaxRooms:    20,
		CleanupTime: 2 * time.Hour, // Cleanup after 2 hours
	}
}

// GenerateMaze creates a new maze instance with random layout
func GenerateMaze(storageMgr *storage.Manager, playerID string, config MazeConfig) (*MazeInstance, error) {
	// Generate instance ID if not provided
	if config.InstanceID == "" {
		config.InstanceID = fmt.Sprintf("maze_%s_%d", playerID, time.Now().Unix())
	}
	
	// Set created by if not provided
	if config.CreatedBy == "" {
		config.CreatedBy = playerID
	}
	
	// Validate room count
	if config.RoomCount < config.MinRooms {
		config.RoomCount = config.MinRooms
	} else if config.RoomCount > config.MaxRooms {
		config.RoomCount = config.MaxRooms
	}
	
	// Create temporary world for this maze instance
	worldName := fmt.Sprintf("Maze Instance %s", config.InstanceID)
	worldDesc := fmt.Sprintf("A dark, twisted maze created for %s. The air is thick with danger and the scent of the wumpus.", config.CreatedBy)
	
	mazeWorld, err := storageMgr.CreateNewWorld(worldName, worldDesc)
	if err != nil {
		return nil, fmt.Errorf("failed to create maze world: %w", err)
	}
	
	// Set world properties for cleanup
	cleanupTime := time.Now().Add(config.CleanupTime)
	worldData := world.WumpusWorldData{
		WorldType:  world.WumpusWorldTypeTemporary,
		InstanceID: config.InstanceID,
		CreatedBy:  config.CreatedBy,
		CleanupAt:  cleanupTime.Format(time.RFC3339),
		MazeConfig: &world.MazeConfig{
			RoomCount: config.RoomCount,
		},
	}
	world.SetWumpusWorldData(mazeWorld, worldData)
	
	// Save world with properties
	if err := storageMgr.Worlds().SaveWorld(mazeWorld); err != nil {
		return nil, fmt.Errorf("failed to save maze world: %w", err)
	}
	
	// Generate maze layout
	instance := &MazeInstance{
		World:  mazeWorld,
		Rooms:  make([]*MazeRoom, 0, config.RoomCount),
		Config: config,
	}
	
	// Create rooms with random positions
	positions := generateRandomPositions(config.RoomCount)
	
	for i, pos := range positions {
		roomName := fmt.Sprintf("Maze Room %d", i+1)
		roomDesc := generateRoomDescription(pos, i == 0)
		
		room, err := storageMgr.CreateNewRoom(mazeWorld.ID, roomName, roomDesc)
		if err != nil {
			return nil, fmt.Errorf("failed to create maze room %d: %w", i+1, err)
		}
		
		// Create maze room wrapper
		mazeRoom := &MazeRoom{
			Room:     room,
			Position: pos,
			IsStart:  i == 0, // First room is start room
		}
		
		// Set room properties
		roomData := world.WumpusRoomData{
			RoomType:     world.WumpusRoomTypeMaze,
			InstanceID:   config.InstanceID,
			MazePosition: &pos,
			TempCleanup:  cleanupTime.Format(time.RFC3339),
		}
		
		// Set as start room if first
		if i == 0 {
			instance.StartRoom = mazeRoom
			mazeWorld.DefaultRoomID = room.ID
		}
		
		room.SetProperty("wumpus_room", roomData)
		instance.Rooms = append(instance.Rooms, mazeRoom)
	}
	
	// Connect rooms with passages
	if err := connectRooms(instance); err != nil {
		return nil, fmt.Errorf("failed to connect maze rooms: %w", err)
	}
	
	// Place wumpus randomly (not in start room)
	if err := placeWumpus(instance); err != nil {
		return nil, fmt.Errorf("failed to place wumpus: %w", err)
	}
	
	// Add stench near wumpus
	if err := addStench(instance); err != nil {
		return nil, fmt.Errorf("failed to add stench: %w", err)
	}
	
	// Place pits randomly
	if err := placePits(instance); err != nil {
		return nil, fmt.Errorf("failed to place pits: %w", err)
	}
	
	// Add breezes near pits
	if err := addBreezes(instance); err != nil {
		return nil, fmt.Errorf("failed to add breezes: %w", err)
	}
	
	// Save all rooms
	for _, mazeRoom := range instance.Rooms {
		if err := storageMgr.Worlds().SaveRoom(mazeRoom.Room); err != nil {
			return nil, fmt.Errorf("failed to save maze room: %w", err)
		}
	}
	
	// Update world with final maze config
	worldData.MazeConfig.WumpusRoom = instance.WumpusRoom.Room.ID
	worldData.MazeConfig.StartRoom = instance.StartRoom.Room.ID
	world.SetWumpusWorldData(mazeWorld, worldData)
	
	if err := storageMgr.Worlds().SaveWorld(mazeWorld); err != nil {
		return nil, fmt.Errorf("failed to save final maze world: %w", err)
	}
	
	return instance, nil
}

// generateRandomPositions creates random positions for maze rooms
func generateRandomPositions(count int) []world.Position {
	positions := make([]world.Position, count)
	used := make(map[string]bool)
	
	for i := 0; i < count; i++ {
		var pos world.Position
		var key string
		
		// Generate unique position
		for {
			x := randomInt(0, 10)
			y := randomInt(0, 10)
			key = fmt.Sprintf("%d,%d", x, y)
			
			if !used[key] {
				pos = world.Position{X: x, Y: y}
				used[key] = true
				break
			}
		}
		
		positions[i] = pos
	}
	
	return positions
}

// connectRooms creates passages between maze rooms
func connectRooms(instance *MazeInstance) error {
	// Connect rooms in a way that ensures all rooms are reachable
	// Start with a minimum spanning tree approach
	
	for i, room := range instance.Rooms {
		// Connect to next room in sequence to ensure connectivity
		if i < len(instance.Rooms)-1 {
			nextRoom := instance.Rooms[i+1]
			direction := getRandomDirection()
			oppositeDirection := getOppositeDirection(direction)
			
			// Add exit from current room to next room
			room.Room.AddExit(direction, instance.World.ID, nextRoom.Room.ID)
			
			// Add return exit from next room to current room
			nextRoom.Room.AddExit(oppositeDirection, instance.World.ID, room.Room.ID)
		}
		
		// Add some random additional connections for complexity
		if randomInt(0, 100) < 30 { // 30% chance of extra connection
			// Find a random room that's not too far away
			for j, otherRoom := range instance.Rooms {
				if i != j && !hasConnection(room.Room, otherRoom.Room) {
					distance := calculateDistance(room.Position, otherRoom.Position)
					if distance <= 3 { // Only connect nearby rooms
						direction := getRandomDirection()
						oppositeDirection := getOppositeDirection(direction)
						
						// Only add if the direction is not already used
						if room.Room.GetExit(direction) == nil && otherRoom.Room.GetExit(oppositeDirection) == nil {
							room.Room.AddExit(direction, instance.World.ID, otherRoom.Room.ID)
							otherRoom.Room.AddExit(oppositeDirection, instance.World.ID, room.Room.ID)
							break
						}
					}
				}
			}
		}
	}
	
	return nil
}

// placeWumpus randomly places the wumpus in a room (not the start room)
func placeWumpus(instance *MazeInstance) error {
	// Find all rooms except start room
	candidates := make([]*MazeRoom, 0)
	for _, room := range instance.Rooms {
		if !room.IsStart {
			candidates = append(candidates, room)
		}
	}
	
	if len(candidates) == 0 {
		return fmt.Errorf("no suitable rooms for wumpus placement")
	}
	
	// Select random room for wumpus
	wumpusIndex := randomInt(0, len(candidates))
	wumpusRoom := candidates[wumpusIndex]
	
	// Mark room as having wumpus
	wumpusRoom.HasWumpus = true
	instance.WumpusRoom = wumpusRoom
	
	// Update room properties
	roomData := world.GetWumpusRoomData(wumpusRoom.Room)
	roomData.HasWumpus = true
	wumpusRoom.Room.SetProperty("wumpus_room", roomData)
	
	// Update room description
	wumpusRoom.Room.Description += "\n\nA terrible, musky smell fills the air here. Something large and dangerous lurks in the shadows."
	
	return nil
}

// placePits randomly places pits in some rooms
func placePits(instance *MazeInstance) error {
	// Place pits in 20-30% of rooms (excluding start room and wumpus room)
	candidates := make([]*MazeRoom, 0)
	for _, room := range instance.Rooms {
		if !room.IsStart && !room.HasWumpus {
			candidates = append(candidates, room)
		}
	}
	
	pitCount := len(candidates) * 25 / 100 // 25% of candidate rooms
	if pitCount < 1 {
		pitCount = 1
	}
	
	// Randomly select rooms for pits
	for i := 0; i < pitCount && i < len(candidates); i++ {
		// Select random candidate
		pitIndex := randomInt(0, len(candidates))
		pitRoom := candidates[pitIndex]
		
		// Mark room as having pit
		pitRoom.HasPit = true
		
		// Update room properties
		roomData := world.GetWumpusRoomData(pitRoom.Room)
		roomData.HasPit = true
		pitRoom.Room.SetProperty("wumpus_room", roomData)
		
		// Update room description
		pitRoom.Room.Description += "\n\nThe ground here feels unstable, and you can hear the sound of wind echoing from below."
		
		// Remove from candidates to avoid duplicate pits
		candidates = append(candidates[:pitIndex], candidates[pitIndex+1:]...)
	}
	
	return nil
}

// addBreezes adds breezes to rooms adjacent to pits
func addBreezes(instance *MazeInstance) error {
	// Find all rooms with pits
	pitRooms := make([]*MazeRoom, 0)
	for _, room := range instance.Rooms {
		if room.HasPit {
			pitRooms = append(pitRooms, room)
		}
	}
	
	// Add breezes to adjacent rooms
	for _, pitRoom := range pitRooms {
		// Find adjacent rooms
		for _, exit := range pitRoom.Room.Exits {
			// Find the target room
			for _, room := range instance.Rooms {
				if room.Room.ID == exit.TargetRoomID {
					// Add breeze to this room
					room.HasBreeze = true
					
					// Update room properties
					roomData := world.GetWumpusRoomData(room.Room)
					roomData.HasBreeze = true
					room.Room.SetProperty("wumpus_room", roomData)
					
					// Update room description if not already done
					if !contains(room.Room.Description, "breeze") {
						room.Room.Description += "\n\nYou feel a cool breeze coming from somewhere nearby."
					}
					break
				}
			}
		}
	}
	
	return nil
}

// addStench adds stench to rooms adjacent to the wumpus
func addStench(instance *MazeInstance) error {
	// Make sure we have a wumpus room
	if instance.WumpusRoom == nil {
		return fmt.Errorf("no wumpus room found in instance")
	}
	
	// Add stench to adjacent rooms
	wumpusRoom := instance.WumpusRoom
	// Find adjacent rooms
	for _, exit := range wumpusRoom.Room.Exits {
		// Find the target room
		for _, room := range instance.Rooms {
			if room.Room.ID == exit.TargetRoomID {
				// Update room properties
				roomData := world.GetWumpusRoomData(room.Room)
				roomData.HasStench = true
				room.Room.SetProperty("wumpus_room", roomData)
				
				// Update room description if not already done
				if !contains(room.Room.Description, "stench") && !contains(room.Room.Description, "smell") {
					room.Room.Description += "\n\nA terrible, musky stench fills the air. Something large and dangerous must be nearby."
				}
				break
			}
		}
	}
	
	return nil
}

// generateRoomDescription creates a random description for a maze room
func generateRoomDescription(pos world.Position, isStart bool) string {
	if isStart {
		return fmt.Sprintf("You stand at the entrance to a dark maze. The walls are made of ancient stone, "+
			"worn smooth by countless years. Torches flicker in sconces along the walls, casting dancing shadows. "+
			"The air is cool and damp, and you can hear distant sounds echoing from deeper within the maze. "+
			"This is your starting point - remember it well, for you may need to find your way back. "+
			"Position: (%d, %d)", pos.X, pos.Y)
	}
	
	descriptions := []string{
		"A narrow corridor carved from living rock. The walls are slick with moisture, and strange symbols are etched into the stone.",
		"A circular chamber with a high vaulted ceiling. Shadows dance in the corners, and the air feels thick with mystery.",
		"A winding passage that seems to stretch on forever. The sound of dripping water echoes from somewhere ahead.",
		"A small alcove with ancient murals depicting scenes of legendary hunts. The artwork is faded but still haunting.",
		"A crossroads where multiple tunnels meet. The air currents here carry strange scents from different directions.",
		"A room with a low ceiling and rough-hewn walls. Strange fungi grow in patches along the stone.",
		"A long hallway lined with empty torch brackets. Your footsteps echo ominously in the silence.",
		"A chamber with a deep well in the center. The water below is black as night and still as death.",
		"A curving passage that seems to spiral deeper into the earth. The walls are covered in ancient scratch marks.",
		"A small room with a single beam of light filtering down from above. Dust motes dance in the pale illumination.",
	}
	
	index := (pos.X + pos.Y) % len(descriptions)
	return fmt.Sprintf("%s Position: (%d, %d)", descriptions[index], pos.X, pos.Y)
}

// Helper functions

// randomInt generates a random integer between min and max-1
func randomInt(min, max int) int {
	if max <= min {
		return min
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		// Fallback to time-based seed if crypto/rand fails
		return min + int(time.Now().UnixNano())%(max-min)
	}
	return min + int(n.Int64())
}

// getRandomDirection returns a random direction
func getRandomDirection() string {
	directions := []string{"north", "south", "east", "west", "northeast", "northwest", "southeast", "southwest"}
	return directions[randomInt(0, len(directions))]
}

// getOppositeDirection returns the opposite direction
func getOppositeDirection(direction string) string {
	opposites := map[string]string{
		"north":     "south",
		"south":     "north",
		"east":      "west",
		"west":      "east",
		"northeast": "southwest",
		"southwest": "northeast",
		"northwest": "southeast",
		"southeast": "northwest",
	}
	return opposites[direction]
}

// hasConnection checks if two rooms are already connected
func hasConnection(room1, room2 *storage.Room) bool {
	for _, exit := range room1.Exits {
		if exit.TargetRoomID == room2.ID {
			return true
		}
	}
	for _, exit := range room2.Exits {
		if exit.TargetRoomID == room1.ID {
			return true
		}
	}
	return false
}

// calculateDistance calculates the distance between two positions
func calculateDistance(pos1, pos2 world.Position) int {
	dx := pos1.X - pos2.X
	dy := pos1.Y - pos2.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx + dy // Manhattan distance
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(s) > len(substr) && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr ||
		      findSubstring(s, substr))))
}

// findSubstring is a simple substring search
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetMazeInstance retrieves a maze instance by ID
func GetMazeInstance(storageMgr *storage.Manager, instanceID string) (*MazeInstance, error) {
	// Get all active worlds
	worlds, err := storageMgr.Worlds().GetActiveWorlds()
	if err != nil {
		return nil, fmt.Errorf("failed to get active worlds: %w", err)
	}
	
	// Find the maze world with matching instance ID
	for _, mazeWorld := range worlds {
		worldData := world.GetWumpusWorldData(mazeWorld)
		if worldData.InstanceID == instanceID && worldData.WorldType == world.WumpusWorldTypeTemporary {
			// Found the maze world, now load all rooms
			instance := &MazeInstance{
				World: mazeWorld,
				Rooms: make([]*MazeRoom, 0),
				Config: MazeConfig{
					InstanceID: instanceID,
					CreatedBy:  worldData.CreatedBy,
				},
			}
			
			// Load all rooms in the world
			roomIDs, err := storageMgr.Worlds().ListRooms(mazeWorld.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to list rooms: %w", err)
			}
			
			for _, roomID := range roomIDs {
				room, err := storageMgr.Worlds().LoadRoom(mazeWorld.ID, roomID)
				if err != nil {
					continue
				}
				
				// Create maze room wrapper
				roomData := world.GetWumpusRoomData(room)
				mazeRoom := &MazeRoom{
					Room:      room,
					HasWumpus: roomData.HasWumpus,
					HasPit:    roomData.HasPit,
					HasBreeze: roomData.HasBreeze,
					IsStart:   room.ID == mazeWorld.DefaultRoomID,
				}
				
				if roomData.MazePosition != nil {
					mazeRoom.Position = *roomData.MazePosition
				}
				
				instance.Rooms = append(instance.Rooms, mazeRoom)
				
				// Set references
				if mazeRoom.IsStart {
					instance.StartRoom = mazeRoom
				}
				if mazeRoom.HasWumpus {
					instance.WumpusRoom = mazeRoom
				}
			}
			
			return instance, nil
		}
	}
	
	return nil, fmt.Errorf("maze instance %s not found", instanceID)
}

// CleanupExpiredMazes removes expired maze instances
func CleanupExpiredMazes(storageMgr *storage.Manager) error {
	// Get all active worlds
	worlds, err := storageMgr.Worlds().GetActiveWorlds()
	if err != nil {
		return fmt.Errorf("failed to get active worlds: %w", err)
	}
	
	cleaned := 0
	for _, mazeWorld := range worlds {
		worldData := world.GetWumpusWorldData(mazeWorld)
		if worldData.WorldType == world.WumpusWorldTypeTemporary && worldData.CleanupAt != "" {
			// Check if cleanup time has passed
			if cleanupTime, err := time.Parse(time.RFC3339, worldData.CleanupAt); err == nil {
				if time.Now().After(cleanupTime) {
					// Delete the entire maze world
					if err := storageMgr.Worlds().DeleteWorld(mazeWorld.ID); err != nil {
						return fmt.Errorf("failed to delete expired maze world %s: %w", mazeWorld.ID, err)
					}
					cleaned++
				}
			}
		}
	}
	
	if cleaned > 0 {
		fmt.Printf("Cleaned up %d expired maze instances\n", cleaned)
	}
	
	return nil
}