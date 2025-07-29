package errors

import (
	"fmt"
	"strings"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/game"
)

// ErrorType represents different categories of errors
type ErrorType int

const (
	ErrorTypePropertiesCorruption ErrorType = iota
	ErrorTypeAuthenticationFailure
	ErrorTypeGameStateCorruption
	ErrorTypeWorldCorruption
	ErrorTypeInstanceCorruption
	ErrorTypeNetworkFailure
	ErrorTypeUserInput
	ErrorTypeSystem
)

// RecoveryAction represents actions that can be taken to recover from errors
type RecoveryAction int

const (
	RecoveryActionNone RecoveryAction = iota
	RecoveryActionResetProperties
	RecoveryActionRespawnPlayer
	RecoveryActionReturnToInn
	RecoveryActionCreateNewInstance
	RecoveryActionFullReset
	RecoveryActionKickPlayer
)

// WumpusError represents an error with recovery information
type WumpusError struct {
	Type            ErrorType
	Message         string
	UserMessage     string
	Cause           error
	RecoveryActions []RecoveryAction
	PlayerID        string
	InstanceID      string
	Timestamp       time.Time
	Recoverable     bool
}

// Error implements the error interface
func (e *WumpusError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// NewWumpusError creates a new WumpusError
func NewWumpusError(errType ErrorType, message, userMessage string, cause error) *WumpusError {
	return &WumpusError{
		Type:        errType,
		Message:     message,
		UserMessage: userMessage,
		Cause:       cause,
		Timestamp:   time.Now(),
		Recoverable: true,
	}
}

// ErrorRecoverySystem handles error recovery for the Wumpus Hunt game
type ErrorRecoverySystem struct {
	storageManager   *storage.Manager
	gameStateManager *game.StateManager
}

// NewErrorRecoverySystem creates a new error recovery system
func NewErrorRecoverySystem(storageManager *storage.Manager, gameStateManager *game.StateManager) *ErrorRecoverySystem {
	return &ErrorRecoverySystem{
		storageManager:   storageManager,
		gameStateManager: gameStateManager,
	}
}

// RecoverFromPropertiesCorruption attempts to recover from corrupted player properties
func (ers *ErrorRecoverySystem) RecoverFromPropertiesCorruption(player *storage.Player, propertyKey string) *WumpusError {
	logging.Warn("Attempting recovery from properties corruption", 
		"player_id", player.ID, 
		"property_key", propertyKey)

	switch propertyKey {
	case "wumpus_auth":
		return ers.recoverAuthProperties(player)
	case "wumpus_stats":
		return ers.recoverStatsProperties(player)
	case "wumpus_state":
		return ers.recoverStateProperties(player)
	default:
		// For unknown properties, just remove them
		player.DeleteProperty(propertyKey)
		logging.Info("Removed corrupted unknown property", 
			"player_id", player.ID, 
			"property_key", propertyKey)
		return nil
	}
}

// recoverAuthProperties handles corrupted authentication properties
func (ers *ErrorRecoverySystem) recoverAuthProperties(player *storage.Player) *WumpusError {
	// Authentication corruption is serious - we need to kick the player
	// They'll need to re-authenticate
	auth.ClearPlayerAuth(player)
	
	wumpusErr := NewWumpusError(
		ErrorTypePropertiesCorruption,
		"Player authentication data corrupted",
		"Your authentication data was corrupted. Please log in again.",
		nil,
	)
	wumpusErr.PlayerID = player.ID
	wumpusErr.RecoveryActions = []RecoveryAction{RecoveryActionKickPlayer}
	
	logging.Error("Authentication properties corrupted - kicking player", 
		"player_id", player.ID)
	
	return wumpusErr
}

// recoverStatsProperties handles corrupted stats properties
func (ers *ErrorRecoverySystem) recoverStatsProperties(player *storage.Player) *WumpusError {
	// Stats corruption is less serious - reset to defaults
	player.DeleteProperty("wumpus_stats")
	
	// GetWumpusStats will now return defaults
	defaultStats := auth.GetWumpusStats(player)
	auth.UpdateWumpusStats(player, defaultStats)
	
	logging.Warn("Stats properties corrupted - reset to defaults", 
		"player_id", player.ID)
	
	wumpusErr := NewWumpusError(
		ErrorTypePropertiesCorruption,
		"Player statistics corrupted - reset to defaults",
		"Your game statistics were corrupted and have been reset. You can continue playing normally.",
		nil,
	)
	wumpusErr.PlayerID = player.ID
	wumpusErr.RecoveryActions = []RecoveryAction{RecoveryActionNone}
	
	return wumpusErr
}

// recoverStateProperties handles corrupted game state properties
func (ers *ErrorRecoverySystem) recoverStateProperties(player *storage.Player) *WumpusError {
	// State corruption - reset and return to Inn
	auth.ResetPlayerState(player)
	
	logging.Warn("Game state properties corrupted - resetting player", 
		"player_id", player.ID)
	
	wumpusErr := NewWumpusError(
		ErrorTypePropertiesCorruption,
		"Player game state corrupted - returned to Inn",
		"Your game state was corrupted. You have been healed and returned to The Inn.",
		nil,
	)
	wumpusErr.PlayerID = player.ID
	wumpusErr.RecoveryActions = []RecoveryAction{RecoveryActionReturnToInn}
	
	return wumpusErr
}

// RecoverFromInterruptedGame handles recovery when a player's game was interrupted
func (ers *ErrorRecoverySystem) RecoverFromInterruptedGame(player *storage.Player) *WumpusError {
	playerState := auth.GetWumpusState(player)
	
	// If player was in a maze instance, we need to handle the interruption
	if playerState.IsInMaze && playerState.CurrentInstance != "" {
		logging.Info("Recovering player from interrupted game", 
			"player_id", player.ID, 
			"instance_id", playerState.CurrentInstance)
		
		// Fix the interrupted game state - but preserve health corruption for integrity validation
		newState := auth.WumpusState{
			CurrentInstance: "", // Clear interrupted instance
			Health:          playerState.Health, // Keep original health (may be corrupted)
			IsInMaze:        false, // Remove from maze
		}
		
		// Only heal if health is valid, otherwise let integrity validation handle it
		if playerState.Health >= 0 && playerState.Health <= 100 {
			newState.Health = 100 // Heal if health was valid
		}
		
		auth.UpdateWumpusState(player, newState)
		
		wumpusErr := NewWumpusError(
			ErrorTypeGameStateCorruption,
			"Recovered from interrupted game",
			"Your previous game session was interrupted. You are already at full health!",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		wumpusErr.RecoveryActions = []RecoveryAction{RecoveryActionReturnToInn}
		
		return wumpusErr
	}
	
	// Player wasn't in an interrupted state, no recovery needed
	return nil
}

// ValidatePlayerIntegrity performs comprehensive validation of player data
func (ers *ErrorRecoverySystem) ValidatePlayerIntegrity(player *storage.Player) []*WumpusError {
	var errors []*WumpusError
	
	// Validate authentication properties
	if _, err := auth.GetWumpusAuth(player); err != nil {
		wumpusErr := ers.RecoverFromPropertiesCorruption(player, "wumpus_auth")
		if wumpusErr != nil {
			errors = append(errors, wumpusErr)
		}
	}
	
	// Validate stats (this is more forgiving and auto-recovers)
	stats := auth.GetWumpusStats(player)
	if stats.GamesPlayed < 0 || stats.GamesWon < 0 || stats.WumpusPelts < 0 || stats.TotalDeaths < 0 {
		wumpusErr := ers.recoverStatsProperties(player)
		if wumpusErr != nil {
			errors = append(errors, wumpusErr)
		}
	}
	
	// Validate game state
	state := auth.GetWumpusState(player)
	if state.Health < 0 || state.Health > 100 {
		wumpusErr := ers.recoverStateProperties(player)
		if wumpusErr != nil {
			errors = append(errors, wumpusErr)
		}
	}
	
	return errors
}

// SafePropertyAccess provides safe access to player properties with automatic recovery
func (ers *ErrorRecoverySystem) SafePropertyAccess(player *storage.Player, propertyKey string, defaultValue interface{}) (interface{}, *WumpusError) {
	value, exists := player.GetProperty(propertyKey)
	if !exists {
		return defaultValue, nil
	}
	
	// Basic type validation
	switch propertyKey {
	case "wumpus_auth":
		if _, err := auth.GetWumpusAuth(player); err != nil {
			wumpusErr := ers.RecoverFromPropertiesCorruption(player, propertyKey)
			return defaultValue, wumpusErr
		}
		return value, nil
		
	case "wumpus_stats":
		// Check if the property value is valid by trying to parse it
		if _, ok := value.(auth.WumpusStats); ok {
			// Direct struct - this is valid
			stats := auth.GetWumpusStats(player)
			return stats, nil
		} else if statsMap, ok := value.(map[string]interface{}); ok {
			// JSON-deserialized map - validate the structure
			_, hasGamesPlayed := statsMap["games_played"]
			_, hasGamesWon := statsMap["games_won"]
			_, hasWumpusPelts := statsMap["wumpus_pelts"]
			_, hasTotalDeaths := statsMap["total_deaths"]
			
			if !hasGamesPlayed || !hasGamesWon || !hasWumpusPelts || !hasTotalDeaths {
				// Incomplete structure - trigger recovery
				wumpusErr := ers.RecoverFromPropertiesCorruption(player, propertyKey)
				stats := auth.GetWumpusStats(player) // Get defaults after recovery
				return stats, wumpusErr
			}
			// Valid map structure
			stats := auth.GetWumpusStats(player)
			return stats, nil
		} else {
			// Invalid type - trigger recovery
			wumpusErr := ers.RecoverFromPropertiesCorruption(player, propertyKey)
			stats := auth.GetWumpusStats(player) // Get defaults after recovery
			return stats, wumpusErr
		}
		
	case "wumpus_state":
		// Check if the property value is valid by trying to parse it  
		if _, ok := value.(auth.WumpusState); ok {
			// Direct struct - this is valid
			state := auth.GetWumpusState(player)
			return state, nil
		} else if stateMap, ok := value.(map[string]interface{}); ok {
			// JSON-deserialized map - validate the structure
			_, hasCurrentInstance := stateMap["current_instance"]
			_, hasHealth := stateMap["health"]
			_, hasIsInMaze := stateMap["is_in_maze"]
			
			if !hasCurrentInstance || !hasHealth || !hasIsInMaze {
				// Incomplete structure - trigger recovery
				wumpusErr := ers.RecoverFromPropertiesCorruption(player, propertyKey)
				state := auth.GetWumpusState(player) // Get defaults after recovery
				return state, wumpusErr
			}
			// Valid map structure
			state := auth.GetWumpusState(player)
			return state, nil
		} else {
			// Invalid type - trigger recovery
			wumpusErr := ers.RecoverFromPropertiesCorruption(player, propertyKey)
			state := auth.GetWumpusState(player) // Get defaults after recovery
			return state, wumpusErr
		}
		
	default:
		// For unknown properties, return as-is
		return value, nil
	}
}

// GetUserFriendlyErrorMessage converts technical errors to user-friendly messages
func GetUserFriendlyErrorMessage(err error) string {
	if wumpusErr, ok := err.(*WumpusError); ok {
		return wumpusErr.UserMessage
	}
	
	// Handle common error patterns by examining error messages
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found") && strings.Contains(errMsg, "player"):
		return "Player not found. Please check your username and try again."
	case strings.Contains(errMsg, "not found") && strings.Contains(errMsg, "world"):
		return "Game world not found. Please contact an administrator."
	case strings.Contains(errMsg, "not found") && strings.Contains(errMsg, "room"):
		return "Location not found. You have been returned to a safe area."
	case strings.Contains(errMsg, "file does not exist"):
		return "Data not found. This may be normal for new accounts."
	case strings.Contains(errMsg, "failed to load"):
		return "Failed to load game data. Please try again or contact support."
	case strings.Contains(errMsg, "failed to save") || strings.Contains(errMsg, "failed to write"):
		return "Failed to save game data. Please try again or contact support."
	default:
		// Generic fallback for unknown errors
		return "An unexpected error occurred. Please try again or contact support if the problem persists."
	}
}

// LogError logs errors with appropriate context
func LogError(err error, context map[string]interface{}) {
	if wumpusErr, ok := err.(*WumpusError); ok {
		logFields := []interface{}{
			"error_type", wumpusErr.Type,
			"message", wumpusErr.Message,
			"user_message", wumpusErr.UserMessage,
			"recoverable", wumpusErr.Recoverable,
			"timestamp", wumpusErr.Timestamp,
		}
		
		if wumpusErr.PlayerID != "" {
			logFields = append(logFields, "player_id", wumpusErr.PlayerID)
		}
		if wumpusErr.InstanceID != "" {
			logFields = append(logFields, "instance_id", wumpusErr.InstanceID)
		}
		
		// Add context fields
		for key, value := range context {
			logFields = append(logFields, key, value)
		}
		
		if wumpusErr.Recoverable {
			logging.Warn("Recoverable wumpus error", logFields...)
		} else {
			logging.Error("Unrecoverable wumpus error", logFields...)
		}
	} else {
		// Standard error logging
		logFields := []interface{}{"error", err}
		for key, value := range context {
			logFields = append(logFields, key, value)
		}
		logging.Error("Standard error", logFields...)
	}
}