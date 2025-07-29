package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// WumpusAuth represents authentication data for a wumpus hunt player
type WumpusAuth struct {
	PasswordHash string `json:"password_hash"`
	Salt         string `json:"salt"`
	AuthMethod   string `json:"auth_method"`
}

// WumpusStats represents game statistics for a player
type WumpusStats struct {
	GamesPlayed int `json:"games_played"`
	GamesWon    int `json:"games_won"`
	WumpusPelts int `json:"wumpus_pelts"`
	TotalDeaths int `json:"total_deaths"`
}

// WumpusState represents current game state for a player
type WumpusState struct {
	CurrentInstance string `json:"current_instance"`
	Health          int    `json:"health"`
	IsInMaze        bool   `json:"is_in_maze"`
}

// generateSalt generates a random salt for password hashing
func generateSalt() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// SetPlayerPassword sets a password for a player using bcrypt with salt
func SetPlayerPassword(player *storage.Player, password string) error {
	if len(password) < 6 {
		return errors.New("password must be at least 6 characters long")
	}

	// Generate salt
	salt, err := generateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password with salt
	saltedPassword := salt + password
	hash, err := bcrypt.GenerateFromPassword([]byte(saltedPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Store auth data in player properties
	authData := WumpusAuth{
		PasswordHash: string(hash),
		Salt:         salt,
		AuthMethod:   "password",
	}

	player.SetProperty("wumpus_auth", authData)
	return nil
}

// ValidatePlayerPassword validates a player's password
func ValidatePlayerPassword(player *storage.Player, password string) bool {
	auth, err := GetWumpusAuth(player)
	if err != nil {
		return false
	}

	// Reconstruct salted password
	saltedPassword := auth.Salt + password

	// Compare with stored hash
	err = bcrypt.CompareHashAndPassword([]byte(auth.PasswordHash), []byte(saltedPassword))
	return err == nil
}

// GetWumpusAuth retrieves authentication data from player properties
func GetWumpusAuth(player *storage.Player) (*WumpusAuth, error) {
	authData, exists := player.GetProperty("wumpus_auth")
	if !exists {
		return nil, errors.New("no authentication data found")
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := authData.(type) {
	case WumpusAuth:
		return &v, nil
	case map[string]interface{}:
		auth := &WumpusAuth{}
		if hash, ok := v["password_hash"].(string); ok {
			auth.PasswordHash = hash
		} else {
			return nil, errors.New("invalid password hash format")
		}
		if salt, ok := v["salt"].(string); ok {
			auth.Salt = salt
		} else {
			return nil, errors.New("invalid salt format")
		}
		if method, ok := v["auth_method"].(string); ok {
			auth.AuthMethod = method
		} else {
			return nil, errors.New("invalid auth method format")
		}
		return auth, nil
	default:
		return nil, errors.New("invalid authentication data format")
	}
}

// GetWumpusStats retrieves game statistics from player properties
func GetWumpusStats(player *storage.Player) WumpusStats {
	statsData, exists := player.GetProperty("wumpus_stats")
	if !exists {
		// Return default stats for new players
		return WumpusStats{
			GamesPlayed: 0,
			GamesWon:    0,
			WumpusPelts: 0,
			TotalDeaths: 0,
		}
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := statsData.(type) {
	case WumpusStats:
		return v
	case map[string]interface{}:
		stats := WumpusStats{}
		if played, ok := v["games_played"].(float64); ok {
			stats.GamesPlayed = int(played)
		}
		if won, ok := v["games_won"].(float64); ok {
			stats.GamesWon = int(won)
		}
		if pelts, ok := v["wumpus_pelts"].(float64); ok {
			stats.WumpusPelts = int(pelts)
		}
		if deaths, ok := v["total_deaths"].(float64); ok {
			stats.TotalDeaths = int(deaths)
		}
		return stats
	default:
		// Return default stats if data is corrupted
		return WumpusStats{
			GamesPlayed: 0,
			GamesWon:    0,
			WumpusPelts: 0,
			TotalDeaths: 0,
		}
	}
}

// UpdateWumpusStats updates game statistics in player properties
func UpdateWumpusStats(player *storage.Player, stats WumpusStats) {
	player.SetProperty("wumpus_stats", stats)
}

// GetWumpusState retrieves current game state from player properties
func GetWumpusState(player *storage.Player) WumpusState {
	stateData, exists := player.GetProperty("wumpus_state")
	if !exists {
		// Return default state for new players
		return WumpusState{
			CurrentInstance: "",
			Health:          100,
			IsInMaze:        false,
		}
	}

	// Handle both direct struct and map[string]interface{} cases
	switch v := stateData.(type) {
	case WumpusState:
		return v
	case map[string]interface{}:
		state := WumpusState{}
		if instance, ok := v["current_instance"].(string); ok {
			state.CurrentInstance = instance
		}
		if health, ok := v["health"].(float64); ok {
			state.Health = int(health)
		}
		if inMaze, ok := v["is_in_maze"].(bool); ok {
			state.IsInMaze = inMaze
		}
		return state
	default:
		// Return default state if data is corrupted
		return WumpusState{
			CurrentInstance: "",
			Health:          100,
			IsInMaze:        false,
		}
	}
}

// UpdateWumpusState updates current game state in player properties
func UpdateWumpusState(player *storage.Player, state WumpusState) {
	player.SetProperty("wumpus_state", state)
}

// IsPlayerAuthenticated checks if a player has authentication data
func IsPlayerAuthenticated(player *storage.Player) bool {
	_, err := GetWumpusAuth(player)
	return err == nil
}

// HasPlayerPassword checks if a player has a password set (constant time)
func HasPlayerPassword(player *storage.Player) bool {
	auth, err := GetWumpusAuth(player)
	if err != nil {
		return false
	}
	// Use constant time comparison to avoid timing attacks
	return subtle.ConstantTimeCompare([]byte(auth.AuthMethod), []byte("password")) == 1
}

// ClearPlayerAuth removes all authentication data from a player
func ClearPlayerAuth(player *storage.Player) {
	player.DeleteProperty("wumpus_auth")
}

// ResetPlayerState resets a player's game state to defaults
func ResetPlayerState(player *storage.Player) {
	defaultState := WumpusState{
		CurrentInstance: "",
		Health:          100,
		IsInMaze:        false,
	}
	UpdateWumpusState(player, defaultState)
}