package ai

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// MovementAI handles Wumpus movement logic and room tracking
type MovementAI struct {
	logger           *logging.Logger
	eventManager     *events.DefaultEventManager
	storageManager   *storage.Manager
	instanceID       string
	currentRoomID    string
	lastMoveTime     time.Time
	movementInterval time.Duration
	isActive         bool
	aggressionLevel  int
	roomGraph        map[string][]string // adjacency list of room connections
}

// MovementConfig configures the movement AI behavior
type MovementConfig struct {
	InstanceID        string
	InitialRoomID     string
	MovementInterval  time.Duration
	BaseMovementChance float64
	AggressionDecay    time.Duration
}

// NewMovementAI creates a new movement AI instance
func NewMovementAI(config MovementConfig, logger *logging.Logger, eventManager *events.DefaultEventManager, storageManager *storage.Manager) *MovementAI {
	return &MovementAI{
		logger:           logger,
		eventManager:     eventManager,
		storageManager:   storageManager,
		instanceID:       config.InstanceID,
		currentRoomID:    config.InitialRoomID,
		lastMoveTime:     time.Now(),
		movementInterval: config.MovementInterval,
		isActive:         false,
		aggressionLevel:  1,
		roomGraph:        make(map[string][]string),
	}
}

// Initialize sets up the movement AI and builds the room graph
func (m *MovementAI) Initialize() error {
	m.logger.Info("Initializing Wumpus Movement AI", 
		"instance_id", m.instanceID,
		"initial_room", m.currentRoomID)

	// Build room adjacency graph from the maze
	err := m.buildRoomGraph()
	if err != nil {
		return fmt.Errorf("failed to build room graph: %w", err)
	}

	// Set initial room properties
	err = m.setWumpusPresence(m.currentRoomID, true)
	if err != nil {
		return fmt.Errorf("failed to set initial Wumpus presence: %w", err)
	}

	// Update scent trails
	m.updateScentTrails()

	m.isActive = true
	m.logger.Info("Wumpus Movement AI initialized", 
		"instance_id", m.instanceID,
		"room_count", len(m.roomGraph))

	return nil
}

// buildRoomGraph constructs the adjacency list for room connections
func (m *MovementAI) buildRoomGraph() error {
	worldManager := m.storageManager.Worlds()
	
	// Get all worlds
	worldIDs, err := worldManager.ListWorlds()
	if err != nil {
		return fmt.Errorf("failed to list worlds: %w", err)
	}

	var mazeWorldID string
	for _, worldID := range worldIDs {
		world, err := worldManager.LoadWorld(worldID)
		if err != nil {
			continue
		}
		
		// Check if this world belongs to our Wumpus instance
		if instanceID, exists := world.GetProperty("wumpus_instance_id"); exists {
			if instanceID == m.instanceID {
				if worldType, exists := world.GetProperty("world_type"); exists && worldType == "temporary" {
					mazeWorldID = worldID
					break
				}
			}
		}
	}

	if mazeWorldID == "" {
		return fmt.Errorf("no maze world found for instance %s", m.instanceID)
	}

	// Get all rooms in the maze world
	roomIDs, err := worldManager.ListRooms(mazeWorldID)
	if err != nil {
		return fmt.Errorf("failed to get rooms for world %s: %w", mazeWorldID, err)
	}

	// Build adjacency list from room connections
	for _, roomID := range roomIDs {
		room, err := worldManager.LoadRoom(mazeWorldID, roomID)
		if err != nil {
			continue // Skip corrupted rooms
		}

		m.roomGraph[roomID] = make([]string, 0)

		// Get connected rooms through exits
		for _, exit := range room.Exits {
			if exit.TargetRoomID != "" {
				m.roomGraph[roomID] = append(m.roomGraph[roomID], exit.TargetRoomID)
			}
		}
	}

	m.logger.Debug("Built room graph", 
		"instance_id", m.instanceID,
		"rooms", len(m.roomGraph))

	return nil
}

// GetAdjacentRooms returns the rooms adjacent to the given room
func (m *MovementAI) GetAdjacentRooms(roomID string) []string {
	if adjacent, exists := m.roomGraph[roomID]; exists {
		return adjacent
	}
	return []string{}
}

// CalculateRoomDistance calculates the shortest path distance between two rooms
func (m *MovementAI) CalculateRoomDistance(fromRoom, toRoom string) int {
	if fromRoom == toRoom {
		return 0
	}

	// Breadth-first search to find shortest path
	queue := []string{fromRoom}
	visited := make(map[string]bool)
	distance := make(map[string]int)
	
	visited[fromRoom] = true
	distance[fromRoom] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == toRoom {
			return distance[toRoom]
		}

		for _, neighbor := range m.GetAdjacentRooms(current) {
			if !visited[neighbor] {
				visited[neighbor] = true
				distance[neighbor] = distance[current] + 1
				queue = append(queue, neighbor)
			}
		}
	}

	// No path found
	return -1
}

// ShouldMove determines if the Wumpus should move based on timing and aggression
func (m *MovementAI) ShouldMove() bool {
	if !m.isActive {
		return false
	}

	// Check if enough time has passed
	if time.Since(m.lastMoveTime) < m.movementInterval {
		return false
	}

	// Base movement chance increases with aggression
	baseChance := 0.3 // 30% base chance
	aggressionBonus := float64(m.aggressionLevel-1) * 0.2 // +20% per aggression level above 1
	moveChance := baseChance + aggressionBonus

	return rand.Float64() < moveChance
}

// ExecuteMovement performs a Wumpus movement if conditions are met
func (m *MovementAI) ExecuteMovement() (bool, error) {
	if !m.ShouldMove() {
		return false, nil
	}

	// Get adjacent rooms
	adjacentRooms := m.GetAdjacentRooms(m.currentRoomID)
	if len(adjacentRooms) == 0 {
		m.logger.Warn("Wumpus has no adjacent rooms to move to", 
			"instance_id", m.instanceID,
			"current_room", m.currentRoomID)
		return false, nil
	}

	// Choose random adjacent room
	targetRoom := adjacentRooms[rand.Intn(len(adjacentRooms))]

	// Execute the move
	err := m.MoveTo(targetRoom)
	if err != nil {
		return false, fmt.Errorf("failed to move Wumpus: %w", err)
	}

	m.lastMoveTime = time.Now()
	
	m.logger.Info("Wumpus moved", 
		"instance_id", m.instanceID,
		"from", m.currentRoomID,
		"to", targetRoom)

	return true, nil
}

// MoveTo moves the Wumpus to a specific room
func (m *MovementAI) MoveTo(roomID string) error {
	if !m.isActive {
		return fmt.Errorf("movement AI is not active")
	}

	// Validate target room exists in our graph
	if _, exists := m.roomGraph[roomID]; !exists {
		return fmt.Errorf("target room %s not found in maze", roomID)
	}

	// Clear presence from current room
	err := m.setWumpusPresence(m.currentRoomID, false)
	if err != nil {
		m.logger.Warn("Failed to clear Wumpus presence from old room", 
			"room_id", m.currentRoomID, 
			"error", err)
	}

	// Set presence in new room
	oldRoom := m.currentRoomID
	m.currentRoomID = roomID
	
	err = m.setWumpusPresence(m.currentRoomID, true)
	if err != nil {
		return fmt.Errorf("failed to set Wumpus presence in new room: %w", err)
	}

	// Update scent trails
	m.updateScentTrails()

	// Emit movement event
	m.eventManager.TriggerEventAsync(
		m.eventManager.CreateEvent(
			"wumpus_moved",
			m,
			map[string]interface{}{
				"from_room":   oldRoom,
				"to_room":     roomID,
				"instance_id": m.instanceID,
			},
		),
	)

	return nil
}

// FleeFromPlayer causes the Wumpus to move away from a player's location
func (m *MovementAI) FleeFromPlayer(playerRoomID string) error {
	if !m.isActive {
		return fmt.Errorf("movement AI is not active")
	}

	adjacentRooms := m.GetAdjacentRooms(m.currentRoomID)
	if len(adjacentRooms) == 0 {
		return fmt.Errorf("no adjacent rooms to flee to")
	}

	// Find the room furthest from the player
	bestRoom := ""
	maxDistance := -1

	for _, roomID := range adjacentRooms {
		distance := m.CalculateRoomDistance(roomID, playerRoomID)
		if distance > maxDistance {
			maxDistance = distance
			bestRoom = roomID
		}
	}

	if bestRoom == "" {
		// Fall back to random movement
		bestRoom = adjacentRooms[rand.Intn(len(adjacentRooms))]
	}

	err := m.MoveTo(bestRoom)
	if err != nil {
		return err
	}

	// Increase aggression after fleeing
	m.IncreaseAggression()

	m.logger.Info("Wumpus fled from player", 
		"instance_id", m.instanceID,
		"player_room", playerRoomID,
		"fled_to", bestRoom,
		"distance", maxDistance)

	return nil
}

// setWumpusPresence updates room properties to indicate Wumpus presence
func (m *MovementAI) setWumpusPresence(roomID string, present bool) error {
	// Find the world that contains this room
	worldManager := m.storageManager.Worlds()
	worldIDs, err := worldManager.ListWorlds()
	if err != nil {
		return fmt.Errorf("failed to list worlds: %w", err)
	}

	var room *storage.Room
	for _, wid := range worldIDs {
		if worldManager.RoomExists(wid, roomID) {
			room, err = worldManager.LoadRoom(wid, roomID)
			if err != nil {
				continue
			}
			break
		}
	}

	if room == nil {
		return fmt.Errorf("room %s not found", roomID)
	}

	// Update Wumpus presence using room properties
	room.SetProperty("has_wumpus", present)

	err = worldManager.SaveRoom(room)
	if err != nil {
		return fmt.Errorf("failed to save room %s: %w", roomID, err)
	}

	return nil
}

// updateScentTrails sets up scent markers in nearby rooms
func (m *MovementAI) updateScentTrails() {
	if !m.isActive {
		return
	}

	// Set strong scent in current room
	m.setScentInRoom(m.currentRoomID, 1.0, "You smell the terrible odor of the Wumpus!")

	// Set medium scent in adjacent rooms
	adjacentRooms := m.GetAdjacentRooms(m.currentRoomID)
	for _, roomID := range adjacentRooms {
		m.setScentInRoom(roomID, 0.7, "You catch a whiff of something foul nearby...")
		
		// Set weak scent in rooms 2 steps away
		secondLevel := m.GetAdjacentRooms(roomID)
		for _, farRoom := range secondLevel {
			if farRoom != m.currentRoomID {
				m.setScentInRoom(farRoom, 0.3, "There's a strange smell in the air...")
			}
		}
	}

	m.logger.Debug("Updated scent trails", 
		"instance_id", m.instanceID,
		"center_room", m.currentRoomID,
		"adjacent_count", len(adjacentRooms))
}

// setScentInRoom sets scent properties for a specific room
func (m *MovementAI) setScentInRoom(roomID string, strength float64, message string) {
	// Find the world that contains this room
	worldManager := m.storageManager.Worlds()
	worldIDs, err := worldManager.ListWorlds()
	if err != nil {
		m.logger.Warn("Failed to list worlds for scent update", 
			"room_id", roomID, 
			"error", err)
		return
	}

	var room *storage.Room
	for _, wid := range worldIDs {
		if worldManager.RoomExists(wid, roomID) {
			room, err = worldManager.LoadRoom(wid, roomID)
			if err != nil {
				continue
			}
			break
		}
	}

	if room == nil {
		m.logger.Warn("Room not found for scent update", "room_id", roomID)
		return
	}

	// Update scent properties using room properties
	room.SetProperty("scent_strength", strength)
	room.SetProperty("scent_message", message)

	err = worldManager.SaveRoom(room)
	if err != nil {
		m.logger.Warn("Failed to save room scent data", 
			"room_id", roomID, 
			"error", err)
	}
}

// GetCurrentRoom returns the current room ID of the Wumpus
func (m *MovementAI) GetCurrentRoom() string {
	return m.currentRoomID
}

// IsActive returns whether the movement AI is active
func (m *MovementAI) IsActive() bool {
	return m.isActive
}

// IncreaseAggression raises the Wumpus aggression level
func (m *MovementAI) IncreaseAggression() {
	if m.aggressionLevel < 3 {
		m.aggressionLevel++
		m.logger.Debug("Wumpus aggression increased", 
			"instance_id", m.instanceID,
			"new_level", m.aggressionLevel)
	}
}

// DecreaseAggression lowers the Wumpus aggression level
func (m *MovementAI) DecreaseAggression() {
	if m.aggressionLevel > 1 {
		m.aggressionLevel--
		m.logger.Debug("Wumpus aggression decreased", 
			"instance_id", m.instanceID,
			"new_level", m.aggressionLevel)
	}
}

// GetAggressionLevel returns the current aggression level
func (m *MovementAI) GetAggressionLevel() int {
	return m.aggressionLevel
}

// Update performs periodic movement AI updates
func (m *MovementAI) Update() error {
	if !m.isActive {
		return nil
	}

	// Try to execute movement
	moved, err := m.ExecuteMovement()
	if err != nil {
		return fmt.Errorf("movement update failed: %w", err)
	}

	if moved {
		m.logger.Debug("Wumpus completed movement during update", 
			"instance_id", m.instanceID,
			"current_room", m.currentRoomID)
	}

	// Decay aggression over time
	if time.Since(m.lastMoveTime) > time.Minute*5 {
		m.DecreaseAggression()
	}

	return nil
}

// Shutdown cleanly shuts down the movement AI
func (m *MovementAI) Shutdown() error {
	if !m.isActive {
		return nil
	}

	m.logger.Info("Shutting down Wumpus Movement AI", "instance_id", m.instanceID)

	// Clear Wumpus presence from current room
	err := m.setWumpusPresence(m.currentRoomID, false)
	if err != nil {
		m.logger.Warn("Failed to clear Wumpus presence during shutdown", 
			"room_id", m.currentRoomID, 
			"error", err)
	}

	m.isActive = false
	m.logger.Info("Wumpus Movement AI shutdown complete", "instance_id", m.instanceID)

	return nil
}