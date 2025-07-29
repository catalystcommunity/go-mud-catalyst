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

func TestWumpusAI_Creation(t *testing.T) {
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

	// Create temporary script directory  
	scriptPath := filepath.Join(tempDir, "wumpus.lua")

	// Create a very basic Lua script that won't cause issues
	basicScript := `print("Hello from Wumpus AI!")`

	err = os.WriteFile(scriptPath, []byte(basicScript), 0644)
	require.NoError(t, err)

	wumpusConfig := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	// Test creating WumpusAI instance
	ai, err := NewWumpusAI(wumpusConfig, scriptingSystem, logger, eventManager, storageManager)
	assert.NoError(t, err)
	assert.NotNil(t, ai)
	assert.Equal(t, "test_instance", ai.instanceID)
	assert.Equal(t, scriptPath, ai.scriptPath)
	assert.Equal(t, "room_1", ai.wumpusRoomID)
	assert.False(t, ai.isActive)

	// Test configuration validation
	invalidConfig := WumpusAIConfig{
		ScriptPath:      "",
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	_, err = NewWumpusAI(invalidConfig, scriptingSystem, logger, eventManager, storageManager)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "script path is required")

	invalidConfig2 := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	_, err = NewWumpusAI(invalidConfig2, scriptingSystem, logger, eventManager, storageManager)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "instance ID is required")
}

func TestWumpusAI_BasicMethods(t *testing.T) {
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

	// Create temporary script directory
	scriptPath := filepath.Join(tempDir, "wumpus.lua")

	// Create a basic Lua script
	basicScript := `print("Basic Wumpus AI script")`

	err = os.WriteFile(scriptPath, []byte(basicScript), 0644)
	require.NoError(t, err)

	wumpusConfig := WumpusAIConfig{
		ScriptPath:      scriptPath,
		InstanceID:      "test_instance",
		InitialRoomID:   "room_1",
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(wumpusConfig, scriptingSystem, logger, eventManager, storageManager)
	require.NoError(t, err)

	// Test getters and setters
	assert.False(t, ai.IsActive())
	assert.Equal(t, "room_1", ai.GetCurrentRoom())

	ai.SetCurrentRoom("room_2")
	assert.Equal(t, "room_2", ai.GetCurrentRoom())
}

func TestWumpusAIManager_Creation(t *testing.T) {
	// Create test logger
	logger := logging.NewLogger(logging.LevelDebug, os.Stderr)

	// Create event manager
	eventManager := events.NewEventManager()

	// Create scripting system
	config := scripting.DefaultScriptingConfig()
	scriptingSystem, err := scripting.NewScriptingSystem(config, logger, eventManager)
	require.NoError(t, err)

	// Create storage manager
	tempDir := t.TempDir()
	storageConfig := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(storageConfig)
	err = storageManager.Initialize()
	require.NoError(t, err)
	
	// Create manager
	manager := NewWumpusAIManager(scriptingSystem, logger, eventManager, storageManager, tempDir)
	assert.NotNil(t, manager)
	assert.Equal(t, tempDir, manager.scriptDir)
	assert.Equal(t, scriptingSystem, manager.scriptingSystem)
	assert.Equal(t, logger, manager.logger)
	assert.Equal(t, eventManager, manager.eventManager)
	assert.NotNil(t, manager.instances)
	assert.Equal(t, 0, len(manager.instances))

	// Test shutdown (should work even with no instances)
	err = manager.Shutdown()
	assert.NoError(t, err)
}