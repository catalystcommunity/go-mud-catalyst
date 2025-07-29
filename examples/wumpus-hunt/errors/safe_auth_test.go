package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
)

func setupSafeAuthTest() (*SafeAuthOperations, *storage.Manager) {
	storageMgr, _, ers := setupTestEnvironment()
	safeAuth := NewSafeAuthOperations(ers)
	return safeAuth, storageMgr
}

func cleanupSafeAuthTest(storageMgr *storage.Manager) {
	cleanupTestEnvironment(storageMgr)
}

func TestNewSafeAuthOperations(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	assert.NotNil(t, safeAuth)
	assert.NotNil(t, safeAuth.recoverySystem)
}

func TestSafeSetPlayerPassword(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name         string
		setupPlayer  func(*storage.Player)
		password     string
		expectError  bool
		errorType    ErrorType
	}{
		{
			name: "valid_password",
			setupPlayer: func(player *storage.Player) {
				// Clean player, no setup needed
			},
			password:    "validpassword123",
			expectError: false,
		},
		{
			name: "password_too_short",
			setupPlayer: func(player *storage.Player) {
				// Clean player
			},
			password:    "short",
			expectError: true,
			errorType:   ErrorTypeAuthenticationFailure,
		},
		{
			name: "player_with_corrupted_auth",
			setupPlayer: func(player *storage.Player) {
				player.SetProperty("wumpus_auth", "corrupted_data")
			},
			password:    "validpassword123",
			expectError: true,
			errorType:   ErrorTypePropertiesCorruption,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			wumpusErr := safeAuth.SafeSetPlayerPassword(player, tt.password)

			if tt.expectError {
				require.NotNil(t, wumpusErr)
				assert.Equal(t, tt.errorType, wumpusErr.Type)
				assert.Equal(t, player.ID, wumpusErr.PlayerID)
			} else {
				assert.Nil(t, wumpusErr)
				// Verify password was set
				assert.True(t, auth.ValidatePlayerPassword(player, tt.password))
			}
		})
	}
}

func TestSafeValidatePlayerPassword(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name         string
		setupPlayer  func(*storage.Player)
		password     string
		expectValid  bool
		expectError  bool
		errorType    ErrorType
	}{
		{
			name: "valid_password",
			setupPlayer: func(player *storage.Player) {
				auth.SetPlayerPassword(player, "testpassword123")
			},
			password:    "testpassword123",
			expectValid: true,
			expectError: false,
		},
		{
			name: "invalid_password",
			setupPlayer: func(player *storage.Player) {
				auth.SetPlayerPassword(player, "testpassword123")
			},
			password:    "wrongpassword",
			expectValid: false,
			expectError: true,
			errorType:   ErrorTypeAuthenticationFailure,
		},
		{
			name: "corrupted_auth_data",
			setupPlayer: func(player *storage.Player) {
				player.SetProperty("wumpus_auth", "corrupted_data")
			},
			password:    "anypassword",
			expectValid: false,
			expectError: true,
			errorType:   ErrorTypePropertiesCorruption,
		},
		{
			name: "no_auth_data",
			setupPlayer: func(player *storage.Player) {
				// No auth data set
			},
			password:    "anypassword",
			expectValid: false,
			expectError: true,
			errorType:   ErrorTypeAuthenticationFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			isValid, wumpusErr := safeAuth.SafeValidatePlayerPassword(player, tt.password)

			assert.Equal(t, tt.expectValid, isValid)

			if tt.expectError {
				require.NotNil(t, wumpusErr)
				assert.Equal(t, tt.errorType, wumpusErr.Type)
				assert.Equal(t, player.ID, wumpusErr.PlayerID)
			} else {
				assert.Nil(t, wumpusErr)
			}
		})
	}
}

func TestSafeGetWumpusAuth(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name        string
		setupPlayer func(*storage.Player)
		expectError bool
		errorType   ErrorType
	}{
		{
			name: "valid_auth_data",
			setupPlayer: func(player *storage.Player) {
				auth.SetPlayerPassword(player, "testpassword123")
			},
			expectError: false,
		},
		{
			name: "corrupted_auth_data",
			setupPlayer: func(player *storage.Player) {
				player.SetProperty("wumpus_auth", "corrupted_data")
			},
			expectError: true,
			errorType:   ErrorTypePropertiesCorruption,
		},
		{
			name: "no_auth_data",
			setupPlayer: func(player *storage.Player) {
				// No auth data set
			},
			expectError: true,
			errorType:   ErrorTypeAuthenticationFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			authData, wumpusErr := safeAuth.SafeGetWumpusAuth(player)

			if tt.expectError {
				require.NotNil(t, wumpusErr)
				assert.Nil(t, authData)
				assert.Equal(t, tt.errorType, wumpusErr.Type)
				assert.Equal(t, player.ID, wumpusErr.PlayerID)
			} else {
				assert.Nil(t, wumpusErr)
				require.NotNil(t, authData)
				assert.Equal(t, "password", authData.AuthMethod)
			}
		})
	}
}

func TestSafeGetWumpusStats(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name         string
		setupPlayer  func(*storage.Player)
		expectError  bool
		expectDefaults bool
	}{
		{
			name: "valid_stats",
			setupPlayer: func(player *storage.Player) {
				stats := auth.WumpusStats{
					GamesPlayed: 5,
					GamesWon:    3,
					WumpusPelts: 2,
					TotalDeaths: 1,
				}
				auth.UpdateWumpusStats(player, stats)
			},
			expectError:    false,
			expectDefaults: false,
		},
		{
			name: "corrupted_stats",
			setupPlayer: func(player *storage.Player) {
				player.SetProperty("wumpus_stats", "corrupted_data")
			},
			expectError:    true, // Recovery error returned but stats still work
			expectDefaults: true, // Should get defaults due to corruption
		},
		{
			name: "no_stats",
			setupPlayer: func(player *storage.Player) {
				// No stats set
			},
			expectError:    false,
			expectDefaults: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			stats, wumpusErr := safeAuth.SafeGetWumpusStats(player)

			if tt.expectError {
				assert.NotNil(t, wumpusErr)
			} else {
				assert.Nil(t, wumpusErr)
			}

			// Stats should always be returned (with defaults if needed)
			assert.GreaterOrEqual(t, stats.GamesPlayed, 0)
			assert.GreaterOrEqual(t, stats.GamesWon, 0)
			assert.GreaterOrEqual(t, stats.WumpusPelts, 0)
			assert.GreaterOrEqual(t, stats.TotalDeaths, 0)

			if tt.expectDefaults {
				assert.Equal(t, 0, stats.GamesPlayed)
				assert.Equal(t, 0, stats.GamesWon)
				assert.Equal(t, 0, stats.WumpusPelts)
				assert.Equal(t, 0, stats.TotalDeaths)
			}
		})
	}
}

func TestSafeUpdateWumpusStats(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name          string
		inputStats    auth.WumpusStats
		expectError   bool
		expectedStats auth.WumpusStats
	}{
		{
			name: "valid_stats",
			inputStats: auth.WumpusStats{
				GamesPlayed: 5,
				GamesWon:    3,
				WumpusPelts: 2,
				TotalDeaths: 1,
			},
			expectError: false,
			expectedStats: auth.WumpusStats{
				GamesPlayed: 5,
				GamesWon:    3,
				WumpusPelts: 2,
				TotalDeaths: 1,
			},
		},
		{
			name: "negative_values_corrected",
			inputStats: auth.WumpusStats{
				GamesPlayed: -1,
				GamesWon:    -2,
				WumpusPelts: -3,
				TotalDeaths: -4,
			},
			expectError: true, // Error logged but corrected
			expectedStats: auth.WumpusStats{
				GamesPlayed: 0,
				GamesWon:    0,
				WumpusPelts: 0,
				TotalDeaths: 0,
			},
		},
		{
			name: "inconsistent_wins_corrected",
			inputStats: auth.WumpusStats{
				GamesPlayed: 3,
				GamesWon:    5, // More wins than games played
				WumpusPelts: 2,
				TotalDeaths: 1,
			},
			expectError: false, // Warning logged but not error
			expectedStats: auth.WumpusStats{
				GamesPlayed: 3,
				GamesWon:    3, // Corrected to match games played
				WumpusPelts: 2,
				TotalDeaths: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")

			wumpusErr := safeAuth.SafeUpdateWumpusStats(player, tt.inputStats)

			if tt.expectError {
				assert.NotNil(t, wumpusErr)
				assert.Equal(t, ErrorTypeUserInput, wumpusErr.Type)
			} else {
				assert.Nil(t, wumpusErr)
			}

			// Verify stats were updated correctly
			savedStats := auth.GetWumpusStats(player)
			assert.Equal(t, tt.expectedStats, savedStats)
		})
	}
}

func TestSafeGetWumpusState(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name         string
		setupPlayer  func(*storage.Player)
		expectError  bool
		expectDefaults bool
	}{
		{
			name: "valid_state",
			setupPlayer: func(player *storage.Player) {
				state := auth.WumpusState{
					CurrentInstance: "test_instance",
					Health:          75,
					IsInMaze:        true,
				}
				auth.UpdateWumpusState(player, state)
			},
			expectError:    false,
			expectDefaults: false,
		},
		{
			name: "corrupted_state",
			setupPlayer: func(player *storage.Player) {
				player.SetProperty("wumpus_state", "corrupted_data")
			},
			expectError:    true, // Recovery error returned
			expectDefaults: true, // Should get defaults due to corruption
		},
		{
			name: "no_state",
			setupPlayer: func(player *storage.Player) {
				// No state set
			},
			expectError:    false,
			expectDefaults: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			state, wumpusErr := safeAuth.SafeGetWumpusState(player)

			if tt.expectError {
				assert.NotNil(t, wumpusErr)
			} else {
				assert.Nil(t, wumpusErr)
			}

			// State should always be returned (with defaults if needed)
			assert.GreaterOrEqual(t, state.Health, 0)
			assert.LessOrEqual(t, state.Health, 100)

			if tt.expectDefaults {
				assert.Equal(t, "", state.CurrentInstance)
				assert.Equal(t, 100, state.Health)
				assert.False(t, state.IsInMaze)
			}
		})
	}
}

func TestSafeUpdateWumpusState(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name          string
		inputState    auth.WumpusState
		expectError   bool
		expectedState auth.WumpusState
	}{
		{
			name: "valid_state",
			inputState: auth.WumpusState{
				CurrentInstance: "test_instance",
				Health:          75,
				IsInMaze:        true,
			},
			expectError: false,
			expectedState: auth.WumpusState{
				CurrentInstance: "test_instance",
				Health:          75,
				IsInMaze:        true,
			},
		},
		{
			name: "negative_health_corrected",
			inputState: auth.WumpusState{
				CurrentInstance: "test_instance",
				Health:          -10,
				IsInMaze:        true,
			},
			expectError: true,
			expectedState: auth.WumpusState{
				CurrentInstance: "test_instance",
				Health:          0, // Corrected
				IsInMaze:        true,
			},
		},
		{
			name: "excessive_health_corrected",
			inputState: auth.WumpusState{
				CurrentInstance: "test_instance",
				Health:          150,
				IsInMaze:        true,
			},
			expectError: true,
			expectedState: auth.WumpusState{
				CurrentInstance: "test_instance",
				Health:          100, // Corrected
				IsInMaze:        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")

			wumpusErr := safeAuth.SafeUpdateWumpusState(player, tt.inputState)

			if tt.expectError {
				assert.NotNil(t, wumpusErr)
				assert.Equal(t, ErrorTypeUserInput, wumpusErr.Type)
			} else {
				assert.Nil(t, wumpusErr)
			}

			// Verify state was updated correctly
			savedState := auth.GetWumpusState(player)
			assert.Equal(t, tt.expectedState, savedState)
		})
	}
}

func TestSafeIsPlayerAuthenticated(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	tests := []struct {
		name        string
		setupPlayer func(*storage.Player)
		expectAuth  bool
		expectError bool
	}{
		{
			name: "authenticated_player",
			setupPlayer: func(player *storage.Player) {
				auth.SetPlayerPassword(player, "testpassword123")
			},
			expectAuth:  true,
			expectError: false,
		},
		{
			name: "unauthenticated_player",
			setupPlayer: func(player *storage.Player) {
				// No auth data
			},
			expectAuth:  false,
			expectError: true,
		},
		{
			name: "corrupted_auth_player",
			setupPlayer: func(player *storage.Player) {
				player.SetProperty("wumpus_auth", "corrupted_data")
			},
			expectAuth:  false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			tt.setupPlayer(player)

			isAuth, wumpusErr := safeAuth.SafeIsPlayerAuthenticated(player)

			assert.Equal(t, tt.expectAuth, isAuth)

			if tt.expectError {
				assert.NotNil(t, wumpusErr)
			} else {
				assert.Nil(t, wumpusErr)
			}
		})
	}
}

func TestRecoverPlayerSession(t *testing.T) {
	safeAuth, storageMgr := setupSafeAuthTest()
	defer cleanupSafeAuthTest(storageMgr)

	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Set up multiple issues
	player.SetProperty("wumpus_auth", "corrupted_auth")
	
	badStats := auth.WumpusStats{GamesPlayed: -1}
	auth.UpdateWumpusStats(player, badStats)
	
	badState := auth.WumpusState{
		CurrentInstance: "interrupted_instance",
		Health:          -10,
		IsInMaze:        true,
	}
	auth.UpdateWumpusState(player, badState)

	// Recover session
	errors := safeAuth.RecoverPlayerSession(player)

	// Should find and fix multiple issues
	assert.Greater(t, len(errors), 0)

	// Check specific error types
	var hasAuthError, hasStatsError, hasStateError, hasInterruptedError bool
	for _, err := range errors {
		switch err.Type {
		case ErrorTypePropertiesCorruption:
			if contains(err.RecoveryActions, RecoveryActionKickPlayer) {
				hasAuthError = true
			} else if contains(err.RecoveryActions, RecoveryActionNone) {
				hasStatsError = true
			} else if contains(err.RecoveryActions, RecoveryActionReturnToInn) {
				hasStateError = true
			}
		case ErrorTypeGameStateCorruption:
			hasInterruptedError = true
		}
	}

	assert.True(t, hasAuthError, "Should have auth corruption error")
	assert.True(t, hasStatsError, "Should have stats corruption error")
	assert.True(t, hasStateError, "Should have state corruption error")
	assert.True(t, hasInterruptedError, "Should have interrupted game error")
}

// Helper function to check if slice contains element
func contains(slice []RecoveryAction, item RecoveryAction) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}