package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestSetPlayerPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{
			name:     "valid password",
			password: "testpassword123",
			wantErr:  false,
		},
		{
			name:     "minimum length password",
			password: "123456",
			wantErr:  false,
		},
		{
			name:     "password too short",
			password: "12345",
			wantErr:  true,
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
		},
		{
			name:     "complex password",
			password: "MyC0mplex!P@ssw0rd",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			player := storage.NewPlayer(ids.NewEntityID(), "testuser")
			err := SetPlayerPassword(player, tt.password)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Verify auth data was stored
			auth, err := GetWumpusAuth(player)
			require.NoError(t, err)
			assert.NotEmpty(t, auth.PasswordHash)
			assert.NotEmpty(t, auth.Salt)
			assert.Equal(t, "password", auth.AuthMethod)

			// Verify password validation works
			assert.True(t, ValidatePlayerPassword(player, tt.password))
			assert.False(t, ValidatePlayerPassword(player, "wrongpassword"))
		})
	}
}

func TestValidatePlayerPassword(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	password := "testpassword123"

	// Test validation without password set
	assert.False(t, ValidatePlayerPassword(player, password))

	// Set password
	err := SetPlayerPassword(player, password)
	require.NoError(t, err)

	// Test correct password
	assert.True(t, ValidatePlayerPassword(player, password))

	// Test incorrect password
	assert.False(t, ValidatePlayerPassword(player, "wrongpassword"))
	assert.False(t, ValidatePlayerPassword(player, ""))
	assert.False(t, ValidatePlayerPassword(player, "testpassword124"))
}

func TestGetWumpusAuth(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Test with no auth data
	_, err := GetWumpusAuth(player)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no authentication data found")

	// Set password and test retrieval
	err = SetPlayerPassword(player, "testpassword123")
	require.NoError(t, err)

	auth, err := GetWumpusAuth(player)
	require.NoError(t, err)
	assert.NotEmpty(t, auth.PasswordHash)
	assert.NotEmpty(t, auth.Salt)
	assert.Equal(t, "password", auth.AuthMethod)

	// Test with corrupted data
	player.SetProperty("wumpus_auth", "invalid_data")
	_, err = GetWumpusAuth(player)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid authentication data format")

	// Test with map[string]interface{} format (JSON deserialization)
	authMap := map[string]interface{}{
		"password_hash": "test_hash",
		"salt":          "test_salt",
		"auth_method":   "password",
	}
	player.SetProperty("wumpus_auth", authMap)

	auth, err = GetWumpusAuth(player)
	require.NoError(t, err)
	assert.Equal(t, "test_hash", auth.PasswordHash)
	assert.Equal(t, "test_salt", auth.Salt)
	assert.Equal(t, "password", auth.AuthMethod)
}

func TestWumpusStats(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Test default stats
	stats := GetWumpusStats(player)
	assert.Equal(t, 0, stats.GamesPlayed)
	assert.Equal(t, 0, stats.GamesWon)
	assert.Equal(t, 0, stats.WumpusPelts)
	assert.Equal(t, 0, stats.TotalDeaths)

	// Test updating stats
	newStats := WumpusStats{
		GamesPlayed: 5,
		GamesWon:    2,
		WumpusPelts: 2,
		TotalDeaths: 3,
	}
	UpdateWumpusStats(player, newStats)

	retrievedStats := GetWumpusStats(player)
	assert.Equal(t, newStats, retrievedStats)

	// Test with map[string]interface{} format (JSON deserialization)
	statsMap := map[string]interface{}{
		"games_played": float64(10),
		"games_won":    float64(7),
		"wumpus_pelts": float64(7),
		"total_deaths": float64(3),
	}
	player.SetProperty("wumpus_stats", statsMap)

	stats = GetWumpusStats(player)
	assert.Equal(t, 10, stats.GamesPlayed)
	assert.Equal(t, 7, stats.GamesWon)
	assert.Equal(t, 7, stats.WumpusPelts)
	assert.Equal(t, 3, stats.TotalDeaths)

	// Test with corrupted data
	player.SetProperty("wumpus_stats", "invalid_data")
	stats = GetWumpusStats(player)
	// Should return default stats
	assert.Equal(t, 0, stats.GamesPlayed)
	assert.Equal(t, 0, stats.GamesWon)
	assert.Equal(t, 0, stats.WumpusPelts)
	assert.Equal(t, 0, stats.TotalDeaths)
}

func TestWumpusState(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Test default state
	state := GetWumpusState(player)
	assert.Equal(t, "", state.CurrentInstance)
	assert.Equal(t, 100, state.Health)
	assert.False(t, state.IsInMaze)

	// Test updating state
	newState := WumpusState{
		CurrentInstance: "instance_123",
		Health:          75,
		IsInMaze:        true,
	}
	UpdateWumpusState(player, newState)

	retrievedState := GetWumpusState(player)
	assert.Equal(t, newState, retrievedState)

	// Test with map[string]interface{} format (JSON deserialization)
	stateMap := map[string]interface{}{
		"current_instance": "instance_456",
		"health":           float64(50),
		"is_in_maze":       true,
	}
	player.SetProperty("wumpus_state", stateMap)

	state = GetWumpusState(player)
	assert.Equal(t, "instance_456", state.CurrentInstance)
	assert.Equal(t, 50, state.Health)
	assert.True(t, state.IsInMaze)

	// Test with corrupted data
	player.SetProperty("wumpus_state", "invalid_data")
	state = GetWumpusState(player)
	// Should return default state
	assert.Equal(t, "", state.CurrentInstance)
	assert.Equal(t, 100, state.Health)
	assert.False(t, state.IsInMaze)
}

func TestIsPlayerAuthenticated(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Test without auth data
	assert.False(t, IsPlayerAuthenticated(player))

	// Test with auth data
	err := SetPlayerPassword(player, "testpassword123")
	require.NoError(t, err)
	assert.True(t, IsPlayerAuthenticated(player))

	// Test with corrupted auth data
	player.SetProperty("wumpus_auth", "invalid_data")
	assert.False(t, IsPlayerAuthenticated(player))
}

func TestHasPlayerPassword(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Test without auth data
	assert.False(t, HasPlayerPassword(player))

	// Test with password auth
	err := SetPlayerPassword(player, "testpassword123")
	require.NoError(t, err)
	assert.True(t, HasPlayerPassword(player))

	// Test with different auth method
	authData := WumpusAuth{
		PasswordHash: "hash",
		Salt:         "salt",
		AuthMethod:   "token",
	}
	player.SetProperty("wumpus_auth", authData)
	assert.False(t, HasPlayerPassword(player))

	// Test with corrupted auth data
	player.SetProperty("wumpus_auth", "invalid_data")
	assert.False(t, HasPlayerPassword(player))
}

func TestClearPlayerAuth(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Set auth data
	err := SetPlayerPassword(player, "testpassword123")
	require.NoError(t, err)
	assert.True(t, IsPlayerAuthenticated(player))

	// Clear auth data
	ClearPlayerAuth(player)
	assert.False(t, IsPlayerAuthenticated(player))

	// Test clearing when no auth data exists
	ClearPlayerAuth(player)
	assert.False(t, IsPlayerAuthenticated(player))
}

func TestResetPlayerState(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")

	// Set non-default state
	state := WumpusState{
		CurrentInstance: "instance_123",
		Health:          25,
		IsInMaze:        true,
	}
	UpdateWumpusState(player, state)

	// Verify state was set
	retrievedState := GetWumpusState(player)
	assert.Equal(t, state, retrievedState)

	// Reset state
	ResetPlayerState(player)

	// Verify state was reset to defaults
	resetState := GetWumpusState(player)
	assert.Equal(t, "", resetState.CurrentInstance)
	assert.Equal(t, 100, resetState.Health)
	assert.False(t, resetState.IsInMaze)
}

func TestPasswordSecurity(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	password := "testpassword123"

	// Set password
	err := SetPlayerPassword(player, password)
	require.NoError(t, err)

	// Verify that different salts are generated each time
	auth1, err := GetWumpusAuth(player)
	require.NoError(t, err)

	// Set password again
	err = SetPlayerPassword(player, password)
	require.NoError(t, err)

	auth2, err := GetWumpusAuth(player)
	require.NoError(t, err)

	// Salts should be different
	assert.NotEqual(t, auth1.Salt, auth2.Salt)
	// Hashes should be different (due to different salts)
	assert.NotEqual(t, auth1.PasswordHash, auth2.PasswordHash)

	// But both should validate the same password
	assert.True(t, ValidatePlayerPassword(player, password))
}

func TestPasswordTiming(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	password := "testpassword123"

	// Set password
	err := SetPlayerPassword(player, password)
	require.NoError(t, err)

	// Multiple validation attempts should be consistent
	for i := 0; i < 10; i++ {
		assert.True(t, ValidatePlayerPassword(player, password))
		assert.False(t, ValidatePlayerPassword(player, "wrongpassword"))
	}
}

func TestConcurrentAccess(t *testing.T) {
	player := storage.NewPlayer(ids.NewEntityID(), "testuser")
	password := "testpassword123"

	// Set password
	err := SetPlayerPassword(player, password)
	require.NoError(t, err)

	// Test concurrent access to auth functions
	// This is a basic test - real concurrent testing would require goroutines
	for i := 0; i < 100; i++ {
		assert.True(t, ValidatePlayerPassword(player, password))
		assert.True(t, IsPlayerAuthenticated(player))
		assert.True(t, HasPlayerPassword(player))

		stats := GetWumpusStats(player)
		stats.GamesPlayed++
		UpdateWumpusStats(player, stats)

		state := GetWumpusState(player)
		state.Health = 100 - i
		UpdateWumpusState(player, state)
	}

	// Verify final state
	finalStats := GetWumpusStats(player)
	assert.Equal(t, 100, finalStats.GamesPlayed)

	finalState := GetWumpusState(player)
	assert.Equal(t, 1, finalState.Health)
}