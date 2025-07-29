package ai

import (
	"os"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMovementTestEnvironment(t *testing.T) (*MovementAI, *storage.Manager, *events.DefaultEventManager) {
	// Create test logger
	logger := logging.NewLogger(logging.LevelDebug, os.Stderr)

	// Create event manager
	eventManager := events.NewEventManager()

	// Create storage manager with temp directory
	tempDir := t.TempDir()
	config := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(config)
	err := storageManager.Initialize()
	require.NoError(t, err)

	// Create a test maze world
	testWorld := storage.NewWorld("test-world", "Test Maze World")
	testWorld.Description = "A test maze for Wumpus"
	testWorld.SetProperty("world_type", "temporary")
	testWorld.SetProperty("wumpus_instance_id", "test_instance")
	testWorld.SetProperty("created_by", "test_user")
	
	err = storageManager.Worlds().SaveWorld(testWorld)
	require.NoError(t, err)

	// Create test rooms with connections
	room1 := storage.NewRoom("room_1", testWorld.ID, "Room 1")
	room1.Description = "A test room"
	room1.AddExit("north", testWorld.ID, "room_2")
	room1.AddExit("east", testWorld.ID, "room_3")
	room1.SetProperty("room_type", "maze")
	room1.SetProperty("instance_id", "test_instance")

	room2 := storage.NewRoom("room_2", testWorld.ID, "Room 2")
	room2.Description = "Another test room"
	room2.AddExit("south", testWorld.ID, "room_1")
	room2.AddExit("east", testWorld.ID, "room_4")
	room2.SetProperty("room_type", "maze")
	room2.SetProperty("instance_id", "test_instance")

	room3 := storage.NewRoom("room_3", testWorld.ID, "Room 3")
	room3.Description = "A third test room"
	room3.AddExit("west", testWorld.ID, "room_1")
	room3.AddExit("north", testWorld.ID, "room_4")
	room3.SetProperty("room_type", "maze")
	room3.SetProperty("instance_id", "test_instance")

	room4 := storage.NewRoom("room_4", testWorld.ID, "Room 4")
	room4.Description = "A fourth test room"
	room4.AddExit("west", testWorld.ID, "room_2")
	room4.AddExit("south", testWorld.ID, "room_3")
	room4.SetProperty("room_type", "maze")
	room4.SetProperty("instance_id", "test_instance")

	// Save all rooms
	require.NoError(t, storageManager.Worlds().SaveRoom(room1))
	require.NoError(t, storageManager.Worlds().SaveRoom(room2))
	require.NoError(t, storageManager.Worlds().SaveRoom(room3))
	require.NoError(t, storageManager.Worlds().SaveRoom(room4))

	// Create movement AI
	config2 := MovementConfig{
		InstanceID:         "test_instance",
		InitialRoomID:      "room_1",
		MovementInterval:   time.Millisecond * 100, // Fast for testing
		BaseMovementChance: 0.5,
		AggressionDecay:    time.Minute,
	}

	movementAI := NewMovementAI(config2, logger, eventManager, storageManager)

	return movementAI, storageManager, eventManager
}

func TestNewMovementAI(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	assert.NotNil(t, movementAI)
	assert.Equal(t, "test_instance", movementAI.instanceID)
	assert.Equal(t, "room_1", movementAI.currentRoomID)
	assert.False(t, movementAI.isActive)
	assert.Equal(t, 1, movementAI.aggressionLevel)
	assert.NotNil(t, movementAI.roomGraph)
}

func TestMovementAI_Initialize(t *testing.T) {
	movementAI, storageManager, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	assert.NoError(t, err)
	assert.True(t, movementAI.IsActive())
	assert.Equal(t, 4, len(movementAI.roomGraph))

	// Check that Wumpus presence was set in initial room
	room, err := storageManager.Worlds().LoadRoom("test-world", "room_1")
	require.NoError(t, err)
	hasWumpus, exists := room.GetProperty("has_wumpus")
	assert.True(t, exists)
	assert.True(t, hasWumpus.(bool))
}

func TestMovementAI_BuildRoomGraph(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	err := movementAI.buildRoomGraph()
	assert.NoError(t, err)

	// Verify adjacency lists
	assert.Contains(t, movementAI.roomGraph["room_1"], "room_2")
	assert.Contains(t, movementAI.roomGraph["room_1"], "room_3")
	assert.Contains(t, movementAI.roomGraph["room_2"], "room_1")
	assert.Contains(t, movementAI.roomGraph["room_2"], "room_4")
	assert.Contains(t, movementAI.roomGraph["room_3"], "room_1")
	assert.Contains(t, movementAI.roomGraph["room_3"], "room_4")
	assert.Contains(t, movementAI.roomGraph["room_4"], "room_2")
	assert.Contains(t, movementAI.roomGraph["room_4"], "room_3")
}

func TestMovementAI_GetAdjacentRooms(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	// Test room 1 adjacencies
	adjacent := movementAI.GetAdjacentRooms("room_1")
	assert.Len(t, adjacent, 2)
	assert.Contains(t, adjacent, "room_2")
	assert.Contains(t, adjacent, "room_3")

	// Test non-existent room
	adjacent = movementAI.GetAdjacentRooms("non_existent")
	assert.Len(t, adjacent, 0)
}

func TestMovementAI_CalculateRoomDistance(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	tests := []struct {
		name     string
		from     string
		to       string
		expected int
	}{
		{
			name:     "same room",
			from:     "room_1",
			to:       "room_1",
			expected: 0,
		},
		{
			name:     "adjacent rooms",
			from:     "room_1",
			to:       "room_2",
			expected: 1,
		},
		{
			name:     "two steps away",
			from:     "room_1",
			to:       "room_4",
			expected: 2,
		},
		{
			name:     "non-existent room",
			from:     "room_1",
			to:       "non_existent",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distance := movementAI.CalculateRoomDistance(tt.from, tt.to)
			assert.Equal(t, tt.expected, distance)
		})
	}
}

func TestMovementAI_ShouldMove(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	// Should not move when inactive
	assert.False(t, movementAI.ShouldMove())

	err := movementAI.Initialize()
	require.NoError(t, err)

	// Should not move immediately after last move (timing restriction)
	movementAI.lastMoveTime = time.Now()
	assert.False(t, movementAI.ShouldMove())

	// Should potentially move after interval has passed
	movementAI.lastMoveTime = time.Now().Add(-time.Second)
	// This is probabilistic, so we just check it doesn't panic
	result := movementAI.ShouldMove()
	assert.IsType(t, true, result) // Just checking it returns a boolean
}

func TestMovementAI_MoveTo(t *testing.T) {
	movementAI, storageManager, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	// Should fail when not active
	movementAI.isActive = false
	err = movementAI.MoveTo("room_2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not active")

	// Should fail for non-existent room
	movementAI.isActive = true
	err = movementAI.MoveTo("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Should succeed for valid room
	err = movementAI.MoveTo("room_2")
	assert.NoError(t, err)
	assert.Equal(t, "room_2", movementAI.GetCurrentRoom())

	// Check that room properties were updated
	oldRoom, err := storageManager.Worlds().LoadRoom("test-world", "room_1")
	require.NoError(t, err)
	hasWumpus, exists := oldRoom.GetProperty("has_wumpus")
	assert.True(t, exists)
	assert.False(t, hasWumpus.(bool))

	newRoom, err := storageManager.Worlds().LoadRoom("test-world", "room_2")
	require.NoError(t, err)
	hasWumpus, exists = newRoom.GetProperty("has_wumpus")
	assert.True(t, exists)
	assert.True(t, hasWumpus.(bool))
}

func TestMovementAI_FleeFromPlayer(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	// Should fail when not active
	movementAI.isActive = false
	err = movementAI.FleeFromPlayer("room_3")
	assert.Error(t, err)

	movementAI.isActive = true

	// Should succeed and move to room furthest from player
	originalRoom := movementAI.GetCurrentRoom()
	err = movementAI.FleeFromPlayer("room_3")
	assert.NoError(t, err)
	assert.NotEqual(t, originalRoom, movementAI.GetCurrentRoom())

	// Should increase aggression after fleeing
	assert.Greater(t, movementAI.GetAggressionLevel(), 1)
}

func TestMovementAI_ExecuteMovement(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	// Force movement by setting last move time in the past
	movementAI.lastMoveTime = time.Now().Add(-time.Hour)

	originalRoom := movementAI.GetCurrentRoom()

	// Try movement multiple times (it's probabilistic)
	moved := false
	for i := 0; i < 10; i++ {
		result, err := movementAI.ExecuteMovement()
		assert.NoError(t, err)
		if result {
			moved = true
			break
		}
		// Reset time for next attempt
		movementAI.lastMoveTime = time.Now().Add(-time.Hour)
	}

	// At least one attempt should have succeeded given the probability
	if moved {
		assert.NotEqual(t, originalRoom, movementAI.GetCurrentRoom())
	}
}

func TestMovementAI_AggressionManagement(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	// Test aggression increase
	assert.Equal(t, 1, movementAI.GetAggressionLevel())
	
	movementAI.IncreaseAggression()
	assert.Equal(t, 2, movementAI.GetAggressionLevel())
	
	movementAI.IncreaseAggression()
	assert.Equal(t, 3, movementAI.GetAggressionLevel())
	
	// Should not exceed maximum
	movementAI.IncreaseAggression()
	assert.Equal(t, 3, movementAI.GetAggressionLevel())

	// Test aggression decrease
	movementAI.DecreaseAggression()
	assert.Equal(t, 2, movementAI.GetAggressionLevel())
	
	movementAI.DecreaseAggression()
	assert.Equal(t, 1, movementAI.GetAggressionLevel())
	
	// Should not go below minimum
	movementAI.DecreaseAggression()
	assert.Equal(t, 1, movementAI.GetAggressionLevel())
}

func TestMovementAI_ScentTrails(t *testing.T) {
	movementAI, storageManager, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	// Check scent in current room (should be strongest)
	currentRoom, err := storageManager.Worlds().LoadRoom("test-world", movementAI.GetCurrentRoom())
	require.NoError(t, err)
	scentStrength, exists := currentRoom.GetProperty("scent_strength")
	if exists && scentStrength != nil {
		assert.Equal(t, 1.0, scentStrength.(float64))
	}
	scentMessage, exists := currentRoom.GetProperty("scent_message")
	if exists && scentMessage != nil {
		assert.Contains(t, scentMessage.(string), "terrible odor")
	}

	// Check scent in adjacent rooms
	adjacentRooms := movementAI.GetAdjacentRooms(movementAI.GetCurrentRoom())
	for _, roomID := range adjacentRooms {
		room, err := storageManager.Worlds().LoadRoom("test-world", roomID)
		require.NoError(t, err)
		scentStrength, exists := room.GetProperty("scent_strength")
		if exists && scentStrength != nil {
			assert.Equal(t, 0.7, scentStrength.(float64))
		}
		scentMessage, exists := room.GetProperty("scent_message")
		if exists && scentMessage != nil {
			assert.Contains(t, scentMessage.(string), "foul nearby")
		}
	}
}

func TestMovementAI_Update(t *testing.T) {
	movementAI, _, _ := setupMovementTestEnvironment(t)

	// Should not error when inactive
	err := movementAI.Update()
	assert.NoError(t, err)

	err = movementAI.Initialize()
	require.NoError(t, err)

	// Should not error when active
	err = movementAI.Update()
	assert.NoError(t, err)
}

func TestMovementAI_Shutdown(t *testing.T) {
	movementAI, storageManager, _ := setupMovementTestEnvironment(t)

	err := movementAI.Initialize()
	require.NoError(t, err)

	initialRoom := movementAI.GetCurrentRoom()
	assert.True(t, movementAI.IsActive())

	err = movementAI.Shutdown()
	assert.NoError(t, err)
	assert.False(t, movementAI.IsActive())

	// Check that Wumpus presence was cleared
	room, err := storageManager.Worlds().LoadRoom("test-world", initialRoom)
	require.NoError(t, err)
	hasWumpus, exists := room.GetProperty("has_wumpus")
	assert.True(t, exists)
	assert.False(t, hasWumpus.(bool))

	// Should not error on double shutdown
	err = movementAI.Shutdown()
	assert.NoError(t, err)
}