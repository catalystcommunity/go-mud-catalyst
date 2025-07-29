package errors

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/game"
)

func setupTestEnvironment() (*storage.Manager, *game.StateManager, *ErrorRecoverySystem) {
	// Initialize storage manager with test configuration
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = "/tmp/wumpus_errors_test_" + ids.NewEntityID()
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	storageMgr.Initialize()

	// Initialize game state manager
	gameStateManager := game.NewStateManager(storageMgr)

	// Initialize error recovery system
	ers := NewErrorRecoverySystem(storageMgr, gameStateManager)

	return storageMgr, gameStateManager, ers
}

func cleanupTestEnvironment(storageMgr *storage.Manager) {
	// Storage manager doesn't have a Shutdown method, so we don't need to clean up
	_ = storageMgr
}

func TestNewWumpusError(t *testing.T) {
	err := NewWumpusError(
		ErrorTypePropertiesCorruption,
		"Test error",
		"User friendly message",
		nil,
	)

	assert.Equal(t, ErrorTypePropertiesCorruption, err.Type)
	assert.Equal(t, "Test error", err.Message)
	assert.Equal(t, "User friendly message", err.UserMessage)
	assert.True(t, err.Recoverable)
	assert.False(t, err.Timestamp.IsZero())
	assert.Equal(t, "Test error", err.Error())
}

func TestWumpusErrorWithCause(t *testing.T) {
	cause := assert.AnError
	err := NewWumpusError(
		ErrorTypeSystem,
		"System error",
		"System unavailable",
		cause,
	)

	assert.Contains(t, err.Error(), "System error")
	assert.Contains(t, err.Error(), cause.Error())
}

func TestNewErrorRecoverySystem(t *testing.T) {
	storageMgr, gameStateManager, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	assert.NotNil(t, ers)
	assert.Equal(t, storageMgr, ers.storageManager)
	assert.Equal(t, gameStateManager, ers.gameStateManager)
}

func TestRecoverFromAuthPropertiesCorruption(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	// Create a player with corrupted auth properties
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	player.SetProperty("wumpus_auth", "invalid_data_type")

	// Attempt recovery
	wumpusErr := ers.RecoverFromPropertiesCorruption(player, "wumpus_auth")

	// Should return an error indicating player needs to be kicked
	require.NotNil(t, wumpusErr)
	assert.Equal(t, ErrorTypePropertiesCorruption, wumpusErr.Type)
	assert.Contains(t, wumpusErr.RecoveryActions, RecoveryActionKickPlayer)
	assert.Equal(t, player.ID, wumpusErr.PlayerID)

	// Auth data should be cleared
	assert.False(t, auth.IsPlayerAuthenticated(player))
}

func TestRecoverFromStatsPropertiesCorruption(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	// Create a player with corrupted stats properties
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	player.SetProperty("wumpus_stats", "invalid_data_type")

	// Attempt recovery
	wumpusErr := ers.RecoverFromPropertiesCorruption(player, "wumpus_stats")

	// Should return an error but allow continued play
	require.NotNil(t, wumpusErr)
	assert.Equal(t, ErrorTypePropertiesCorruption, wumpusErr.Type)
	assert.Contains(t, wumpusErr.RecoveryActions, RecoveryActionNone)

	// Stats should be reset to defaults
	stats := auth.GetWumpusStats(player)
	assert.Equal(t, 0, stats.GamesPlayed)
	assert.Equal(t, 0, stats.GamesWon)
	assert.Equal(t, 0, stats.WumpusPelts)
	assert.Equal(t, 0, stats.TotalDeaths)
}

func TestRecoverFromStatePropertiesCorruption(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	// Create a player with corrupted state properties
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	player.SetProperty("wumpus_state", "invalid_data_type")

	// Attempt recovery
	wumpusErr := ers.RecoverFromPropertiesCorruption(player, "wumpus_state")

	// Should return an error and suggest returning to Inn
	require.NotNil(t, wumpusErr)
	assert.Equal(t, ErrorTypePropertiesCorruption, wumpusErr.Type)
	assert.Contains(t, wumpusErr.RecoveryActions, RecoveryActionReturnToInn)

	// State should be reset to defaults
	state := auth.GetWumpusState(player)
	assert.Equal(t, "", state.CurrentInstance)
	assert.Equal(t, 100, state.Health)
	assert.False(t, state.IsInMaze)
}

func TestRecoverFromUnknownPropertiesCorruption(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	// Create a player with unknown corrupted property
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	player.SetProperty("unknown_property", "some_data")

	// Attempt recovery
	wumpusErr := ers.RecoverFromPropertiesCorruption(player, "unknown_property")

	// Should return nil (no error) and remove the property
	assert.Nil(t, wumpusErr)
	_, exists := player.GetProperty("unknown_property")
	assert.False(t, exists)
}

func TestRecoverFromInterruptedGame(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	tests := []struct {
		name         string
		setupPlayer  func(*storage.Player)
		expectError  bool
		expectAction RecoveryAction
	}{
		{
			name: "player_not_in_maze",
			setupPlayer: func(player *storage.Player) {
				state := auth.WumpusState{
					CurrentInstance: "",
					Health:          50,
					IsInMaze:        false,
				}
				auth.UpdateWumpusState(player, state)
			},
			expectError: false,
		},
		{
			name: "player_in_interrupted_maze",
			setupPlayer: func(player *storage.Player) {
				state := auth.WumpusState{
					CurrentInstance: "interrupted_instance",
					Health:          30,
					IsInMaze:        true,
				}
				auth.UpdateWumpusState(player, state)
			},
			expectError:  true,
			expectAction: RecoveryActionReturnToInn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			wumpusErr := ers.RecoverFromInterruptedGame(player)

			if tt.expectError {
				require.NotNil(t, wumpusErr)
				assert.Equal(t, ErrorTypeGameStateCorruption, wumpusErr.Type)
				assert.Contains(t, wumpusErr.RecoveryActions, tt.expectAction)

				// Player should be reset to Inn state
				state := auth.GetWumpusState(player)
				assert.Equal(t, "", state.CurrentInstance)
				assert.Equal(t, 100, state.Health) // Fully healed
				assert.False(t, state.IsInMaze)
			} else {
				assert.Nil(t, wumpusErr)
			}
		})
	}
}

func TestValidatePlayerIntegrity(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	// Create a player with multiple corruption issues
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	
	// Corrupt auth properties
	player.SetProperty("wumpus_auth", "invalid_auth")
	
	// Corrupt stats with negative values
	badStats := auth.WumpusStats{
		GamesPlayed: -1,
		GamesWon:    -1,
		WumpusPelts: -1,
		TotalDeaths: -1,
	}
	auth.UpdateWumpusStats(player, badStats)
	
	// Corrupt state with invalid health
	badState := auth.WumpusState{
		CurrentInstance: "test",
		Health:          -50, // Invalid health
		IsInMaze:        false,
	}
	auth.UpdateWumpusState(player, badState)

	// Validate and recover
	errors := ers.ValidatePlayerIntegrity(player)

	// Should find and fix multiple issues
	assert.Len(t, errors, 3) // Auth, stats, and state corruption

	// Check that all properties are now valid
	assert.False(t, auth.IsPlayerAuthenticated(player)) // Auth should be cleared
	
	stats := auth.GetWumpusStats(player)
	assert.GreaterOrEqual(t, stats.GamesPlayed, 0)
	assert.GreaterOrEqual(t, stats.GamesWon, 0)
	assert.GreaterOrEqual(t, stats.WumpusPelts, 0)
	assert.GreaterOrEqual(t, stats.TotalDeaths, 0)
	
	state := auth.GetWumpusState(player)
	assert.GreaterOrEqual(t, state.Health, 0)
	assert.LessOrEqual(t, state.Health, 100)
}

func TestSafePropertyAccess(t *testing.T) {
	storageMgr, _, ers := setupTestEnvironment()
	defer cleanupTestEnvironment(storageMgr)

	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	tests := []struct {
		name          string
		propertyKey   string
		setupProperty func()
		defaultValue  interface{}
		expectError   bool
	}{
		{
			name:        "missing_property",
			propertyKey: "wumpus_stats",
			setupProperty: func() {
				// Don't set the property
			},
			defaultValue: auth.WumpusStats{},
			expectError:  false,
		},
		{
			name:        "valid_stats_property",
			propertyKey: "wumpus_stats",
			setupProperty: func() {
				stats := auth.WumpusStats{GamesPlayed: 5}
				auth.UpdateWumpusStats(player, stats)
			},
			defaultValue: auth.WumpusStats{},
			expectError:  false,
		},
		{
			name:        "corrupted_auth_property",
			propertyKey: "wumpus_auth",
			setupProperty: func() {
				player.SetProperty("wumpus_auth", "invalid_data")
			},
			defaultValue: nil,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset player
			player = storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupProperty()

			value, wumpusErr := ers.SafePropertyAccess(player, tt.propertyKey, tt.defaultValue)

			if tt.expectError {
				assert.NotNil(t, wumpusErr)
				assert.Equal(t, tt.defaultValue, value)
			} else {
				assert.Nil(t, wumpusErr)
				assert.NotNil(t, value)
			}
		})
	}
}

func TestGetUserFriendlyErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected string
	}{
		{
			name: "wumpus_error",
			error: &WumpusError{
				UserMessage: "Custom user message",
			},
			expected: "Custom user message",
		},
		{
			name:     "player_not_found",
			error:    fmt.Errorf("player test123 not found"),
			expected: "Player not found. Please check your username and try again.",
		},
		{
			name:     "world_not_found",
			error:    fmt.Errorf("world main_world not found"),
			expected: "Game world not found. Please contact an administrator.",
		},
		{
			name:     "room_not_found",
			error:    fmt.Errorf("room inn_room not found"),
			expected: "Location not found. You have been returned to a safe area.",
		},
		{
			name:     "file_does_not_exist",
			error:    fmt.Errorf("file does not exist: /path/to/file"),
			expected: "Data not found. This may be normal for new accounts.",
		},
		{
			name:     "failed_to_load",
			error:    fmt.Errorf("failed to load player data"),
			expected: "Failed to load game data. Please try again or contact support.",
		},
		{
			name:     "failed_to_save",
			error:    fmt.Errorf("failed to save player data"),
			expected: "Failed to save game data. Please try again or contact support.",
		},
		{
			name:     "unknown_error",
			error:    assert.AnError,
			expected: "An unexpected error occurred. Please try again or contact support if the problem persists.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserFriendlyErrorMessage(tt.error)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogError(t *testing.T) {
	// Test that LogError doesn't panic with various error types
	
	// Test with WumpusError
	wumpusErr := &WumpusError{
		Type:        ErrorTypePropertiesCorruption,
		Message:     "Test message",
		UserMessage: "User message",
		PlayerID:    "player123",
		Recoverable: true,
		Timestamp:   time.Now(),
	}
	
	context := map[string]interface{}{
		"test_context": "test_value",
	}
	
	// Should not panic
	assert.NotPanics(t, func() {
		LogError(wumpusErr, context)
	})
	
	// Test with standard error
	assert.NotPanics(t, func() {
		LogError(assert.AnError, context)
	})
	
	// Test with nil context
	assert.NotPanics(t, func() {
		LogError(wumpusErr, nil)
	})
}