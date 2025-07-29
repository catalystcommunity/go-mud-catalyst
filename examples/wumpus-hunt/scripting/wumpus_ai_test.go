package scripting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/scripting"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (*scripting.ScriptingSystem, *logging.Logger, *events.DefaultEventManager, *storage.Manager, string) {
	// Create test logger
	logger := logging.NewLogger(logging.LevelDebug, os.Stderr)

	// Create event manager
	eventManager := events.NewEventManager()

	// Create scripting system with test configuration
	config := scripting.DefaultScriptingConfig()
	config.MaxVMs = 10

	scriptingSystem, err := scripting.NewScriptingSystem(config, logger, eventManager)
	require.NoError(t, err)

	// Create storage manager
	tempDir := t.TempDir()
	storageConfig := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(storageConfig)
	err = storageManager.Initialize()
	require.NoError(t, err)

	// Create test worlds and rooms for MovementAI
	// Create for common test instance IDs
	testInstances := []string{"test_instance", "instance_1", "instance_2", "instance_3"}
	for _, instanceID := range testInstances {
		err = setupTestMazeWorld(storageManager, instanceID)
		require.NoError(t, err)
	}

	// Create temporary script directory
	scriptPath := filepath.Join(tempDir, "wumpus.lua")

	// Create a minimal test Lua script
	testScript := `-- Empty script for testing VM creation`

	err = os.WriteFile(scriptPath, []byte(testScript), 0644)
	require.NoError(t, err)

	return scriptingSystem, logger, eventManager, storageManager, scriptPath
}

// setupTestMazeWorld creates a minimal test world and rooms for MovementAI testing
func setupTestMazeWorld(storageManager *storage.Manager, instanceID string) error {
	// Create a test world with the expected properties
	worldName := "Test Maze World for " + instanceID
	world, err := storageManager.CreateNewWorld(worldName, "Test maze for movement AI")
	if err != nil {
		return err
	}
	
	// Set required properties for MovementAI
	world.SetProperty("world_type", "temporary")
	world.SetProperty("wumpus_instance_id", instanceID)
	
	// Save the world with properties
	worldManager := storageManager.Worlds()
	err = worldManager.SaveWorld(world)
	if err != nil {
		return err
	}

	// Create a simple 4-room maze for testing
	roomConnections := map[string][]string{
		"room_1": {"room_2", "room_3"},
		"room_2": {"room_1", "room_4"},
		"room_3": {"room_1", "room_4"},
		"room_4": {"room_2", "room_3"},
	}

	// Create rooms with specific IDs
	createdRooms := make(map[string]*storage.Room)
	for roomID := range roomConnections {
		// Use lower-level API to create rooms with specific IDs
		room := storage.NewRoom(roomID, world.ID, "Test Room " + roomID)
		room.Description = "Test room for MovementAI"
		
		err := worldManager.SaveRoom(room)
		if err != nil {
			return err
		}
		
		createdRooms[roomID] = room
	}

	// Add exits between rooms
	for roomID, adjacentRooms := range roomConnections {
		room := createdRooms[roomID]
		
		// Add exits to adjacent rooms
		for _, adjRoomID := range adjacentRooms {
			// Use the specific room IDs we created
			room.AddExit("to_" + adjRoomID, adjRoomID, "")
		}
		
		// Save the room with exits
		err := worldManager.SaveRoom(room)
		if err != nil {
			return err
		}
	}

	return nil
}

func TestNewWumpusAI(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	tests := []struct {
		name        string
		config      WumpusAIConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: WumpusAIConfig{
				ScriptPath:      scriptPath,
				InstanceID:      "test_instance",
				InitialRoomID:   "room_1",
				SecurityProfile: "moderate",
			},
			expectError: false,
		},
		{
			name: "empty script path",
			config: WumpusAIConfig{
				ScriptPath:      "",
				InstanceID:      "test_instance",
				InitialRoomID:   "room_1",
				SecurityProfile: "moderate",
			},
			expectError: true,
		},
		{
			name: "empty instance ID",
			config: WumpusAIConfig{
				ScriptPath:      scriptPath,
				InstanceID:      "",
				InitialRoomID:   "room_1",
				SecurityProfile: "moderate",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ai, err := NewWumpusAI(tt.config, scriptingSystem, logger, eventManager, storageManager)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, ai)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ai)
				assert.Equal(t, tt.config.InstanceID, ai.instanceID)
				assert.Equal(t, tt.config.ScriptPath, ai.scriptPath)
				assert.Equal(t, tt.config.InitialRoomID, ai.wumpusRoomID)
				assert.False(t, ai.isActive)
			}
		})
	}
}

func TestWumpusAI_Initialize(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	// Test successful initialization
	err = ai.Initialize()
	assert.NoError(t, err)
	assert.True(t, ai.IsActive())

	// Clean up
	err = ai.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAI_InitializeInvalidScript(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, _ := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      "/nonexistent/path/script.lua",
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	// Test initialization with invalid script path
	err = ai.Initialize()
	assert.Error(t, err)
	assert.False(t, ai.IsActive())
}

func TestWumpusAI_HandlePlayerEnterRoom(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	err = ai.Initialize()
	require.NoError(t, err)

	// Test handling player enter room
	err = ai.HandlePlayerEnterRoom("player_1", "room_2")
	assert.NoError(t, err)

	// Test with inactive AI
	ai.isActive = false
	err = ai.HandlePlayerEnterRoom("player_1", "room_2")
	assert.Error(t, err)

	// Clean up
	ai.isActive = true
	err = ai.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAI_HandlePlayerAttackWumpus(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	err = ai.Initialize()
	require.NoError(t, err)

	// Test normal attack (should not kill Wumpus)
	wumpusDied, err := ai.HandlePlayerAttackWumpus("player_1", 30)
	assert.NoError(t, err)
	assert.False(t, wumpusDied)

	// Test lethal attack (should kill Wumpus)
	wumpusDied, err = ai.HandlePlayerAttackWumpus("player_1", 100)
	assert.NoError(t, err)
	assert.True(t, wumpusDied)

	// Test with inactive AI
	ai.isActive = false
	_, err = ai.HandlePlayerAttackWumpus("player_1", 10)
	assert.Error(t, err)

	// Clean up
	ai.isActive = true
	err = ai.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAI_Update(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	err = ai.Initialize()
	require.NoError(t, err)

	// Test update
	err = ai.Update(1.0)
	assert.NoError(t, err)

	// Test update with inactive AI (should not error)
	ai.isActive = false
	err = ai.Update(1.0)
	assert.NoError(t, err)

	// Clean up
	ai.isActive = true
	err = ai.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAI_Shutdown(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	err = ai.Initialize()
	require.NoError(t, err)
	assert.True(t, ai.IsActive())

	// Test shutdown
	err = ai.Shutdown()
	assert.NoError(t, err)
	assert.False(t, ai.IsActive())

	// Test double shutdown (should not error)
	err = ai.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAI_GettersSetters(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	// Test getters before initialization
	assert.False(t, ai.IsActive())
	assert.Equal(t, "room_1", ai.GetCurrentRoom())

	// Test setter
	ai.SetCurrentRoom("room_2")
	assert.Equal(t, "room_2", ai.GetCurrentRoom())
}

func TestWumpusAIManager(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)
	scriptDir := filepath.Dir(scriptPath)

	manager := NewWumpusAIManager(scriptingSystem, logger, eventManager, storageManager, scriptDir)
	assert.NotNil(t, manager)

	// Test creating Wumpus AI
	ai, err := manager.CreateWumpusAI("instance_1", "room_1")
	assert.NoError(t, err)
	assert.NotNil(t, ai)
	assert.True(t, ai.IsActive())

	// Test duplicate creation (should fail)
	_, err = manager.CreateWumpusAI("instance_1", "room_1")
	assert.Error(t, err)

	// Test getting AI
	retrievedAI, exists := manager.GetWumpusAI("instance_1")
	assert.True(t, exists)
	assert.Equal(t, ai, retrievedAI)

	// Test getting non-existent AI
	_, exists = manager.GetWumpusAI("nonexistent")
	assert.False(t, exists)

	// Test update all
	manager.UpdateAll(1.0)

	// Test destroying AI
	err = manager.DestroyWumpusAI("instance_1")
	assert.NoError(t, err)

	// Test destroying non-existent AI
	err = manager.DestroyWumpusAI("nonexistent")
	assert.Error(t, err)

	// Test shutdown
	err = manager.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAIManager_MultipleInstances(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)
	scriptDir := filepath.Dir(scriptPath)

	manager := NewWumpusAIManager(scriptingSystem, logger, eventManager, storageManager, scriptDir)

	// Create multiple instances
	instances := []string{"instance_1", "instance_2", "instance_3"}
	for _, instanceID := range instances {
		ai, err := manager.CreateWumpusAI(instanceID, "room_1")
		assert.NoError(t, err)
		assert.NotNil(t, ai)
	}

	// Test update all
	manager.UpdateAll(1.0)

	// Verify all instances exist
	for _, instanceID := range instances {
		ai, exists := manager.GetWumpusAI(instanceID)
		assert.True(t, exists)
		assert.True(t, ai.IsActive())
	}

	// Test shutdown all
	err := manager.Shutdown()
	assert.NoError(t, err)

	// Verify all instances are cleaned up
	for _, instanceID := range instances {
		_, exists := manager.GetWumpusAI(instanceID)
		assert.False(t, exists)
	}
}

func TestWumpusAI_EventHandling(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, scriptPath := setupTestEnvironment(t)

	config := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	err = ai.Initialize()
	require.NoError(t, err)

	// Test event handling by emitting events
	testEvents := []struct {
		eventType string
		data      map[string]interface{}
	}{
		{
			"player_enter_room",
			map[string]interface{}{
				"player_id":   "player_1",
				"room_id":     "room_1",
				"instance_id": "test_instance",
			},
		},
		{
			"player_attack_wumpus",
			map[string]interface{}{
				"player_id":   "player_1",
				"damage":      25,
				"instance_id": "test_instance",
			},
		},
		{
			"wumpus_encounter",
			map[string]interface{}{
				"player_id":   "player_1",
				"room_id":     "room_1",
				"instance_id": "test_instance",
			},
		},
	}

	for _, testEvent := range testEvents {
		// Create and emit events
		event := events.NewEvent(events.EventType(testEvent.eventType), nil, testEvent.data)
		eventManager.TriggerEventAsync(event)
	}

	// Clean up
	err = ai.Shutdown()
	assert.NoError(t, err)
}

func TestWumpusAI_ScriptErrorHandling(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, _ := setupTestEnvironment(t)

	// Create a script with syntax errors
	tempDir := t.TempDir()
	badScriptPath := filepath.Join(tempDir, "bad_wumpus.lua")
	badScript := `
-- This script has syntax errors
function initialize_wumpus(room_id, instance_id
    -- Missing closing parenthesis
    return true
end
`

	err := os.WriteFile(badScriptPath, []byte(badScript), 0644)
	require.NoError(t, err)

	config := WumpusAIConfig{
		ScriptPath:      badScriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	// Test initialization with bad script
	err = ai.Initialize()
	assert.Error(t, err)
	assert.False(t, ai.IsActive())
}

func TestWumpusAI_ResourceLimits(t *testing.T) {
	scriptingSystem, logger, eventManager, storageManager, _ := setupTestEnvironment(t)

	// Create a script that might consume resources
	tempDir := t.TempDir()
	resourceScript := filepath.Join(tempDir, "resource_wumpus.lua")
	resourceHeavyScript := `
-- This script tests resource limits
function initialize_wumpus(room_id, instance_id)
    -- Simulate some work
    for i = 1, 1000 do
        local dummy = i * i
    end
    return true
end

function on_update(delta_time)
    -- Simulate ongoing work but not too heavy
    for i = 1, 100 do
        local dummy = i * i
    end
end

return {
    initialize_wumpus = initialize_wumpus,
    on_update = on_update
}
`

	err := os.WriteFile(resourceScript, []byte(resourceHeavyScript), 0644)
	require.NoError(t, err)

	config := WumpusAIConfig{
		ScriptPath:      resourceScript,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "strict", // Use strict profile for tighter limits
	}

	ai, err := NewWumpusAI(config, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	// Test initialization (should work with moderate resource usage)
	err = ai.Initialize()
	assert.NoError(t, err)
	assert.True(t, ai.IsActive())

	// Test updates (should work within limits)
	for i := 0; i < 5; i++ {
		err = ai.Update(1.0)
		assert.NoError(t, err)
	}

	// Clean up
	err = ai.Shutdown()
	assert.NoError(t, err)
}