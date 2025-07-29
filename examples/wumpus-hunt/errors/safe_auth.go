package errors

import (
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
)

// SafeAuthOperations provides authentication operations with built-in error recovery
type SafeAuthOperations struct {
	recoverySystem *ErrorRecoverySystem
}

// NewSafeAuthOperations creates a new safe authentication operations handler
func NewSafeAuthOperations(recoverySystem *ErrorRecoverySystem) *SafeAuthOperations {
	return &SafeAuthOperations{
		recoverySystem: recoverySystem,
	}
}

// SafeSetPlayerPassword sets a player password with error recovery
func (ops *SafeAuthOperations) SafeSetPlayerPassword(player *storage.Player, password string) *WumpusError {
	// Check if player already has auth data - only validate integrity if they do
	_, hasAuth := player.GetProperty("wumpus_auth")
	if hasAuth {
		// Player has existing auth data, validate integrity
		if errs := ops.recoverySystem.ValidatePlayerIntegrity(player); len(errs) > 0 {
			// Log all validation errors
			for _, err := range errs {
				LogError(err, map[string]interface{}{
					"operation": "set_password",
					"player_id": player.ID,
				})
			}
			
			// If there were auth errors, player needs to re-authenticate
			for _, err := range errs {
				if err.Type == ErrorTypePropertiesCorruption {
					for _, action := range err.RecoveryActions {
						if action == RecoveryActionKickPlayer {
							return err // Return the kick error
						}
					}
				}
			}
		}
	}
	
	// Attempt to set password
	if err := auth.SetPlayerPassword(player, password); err != nil {
		wumpusErr := NewWumpusError(
			ErrorTypeAuthenticationFailure,
			"Failed to set player password",
			"Failed to set password. Please try again.",
			err,
		)
		wumpusErr.PlayerID = player.ID
		
		LogError(wumpusErr, map[string]interface{}{
			"operation": "set_password",
		})
		
		return wumpusErr
	}
	
	logging.Info("Password set successfully", "player_id", player.ID)
	return nil
}

// SafeValidatePlayerPassword validates a player password with error recovery
func (ops *SafeAuthOperations) SafeValidatePlayerPassword(player *storage.Player, password string) (bool, *WumpusError) {
	// First check if player has auth data at all - only validate integrity if they do
	_, hasAuth := player.GetProperty("wumpus_auth")
	if !hasAuth {
		// No auth data found - this is not corruption, just missing auth
		wumpusErr := NewWumpusError(
			ErrorTypeAuthenticationFailure,
			"Player authentication data not found",
			"No authentication data found. Please log in to continue.",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		wumpusErr.Recoverable = true
		wumpusErr.RecoveryActions = []RecoveryAction{RecoveryActionKickPlayer}
		
		LogError(wumpusErr, map[string]interface{}{
			"operation": "validate_password",
			"player_id": player.ID,
		})
		
		return false, wumpusErr
	}
	
	// Player has auth data, check for corruption
	if errs := ops.recoverySystem.ValidatePlayerIntegrity(player); len(errs) > 0 {
		// If auth is corrupted, player needs to re-authenticate
		for _, err := range errs {
			if err.Type == ErrorTypePropertiesCorruption {
				for _, action := range err.RecoveryActions {
					if action == RecoveryActionKickPlayer {
						LogError(err, map[string]interface{}{
							"operation": "validate_password",
							"player_id": player.ID,
						})
						return false, err
					}
				}
			}
		}
	}
	
	// Attempt to validate password
	isValid := auth.ValidatePlayerPassword(player, password)
	
	if !isValid {
		wumpusErr := NewWumpusError(
			ErrorTypeAuthenticationFailure,
			"Password validation failed",
			"Invalid username or password.",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		wumpusErr.Recoverable = false // Don't auto-recover from bad passwords
		
		LogError(wumpusErr, map[string]interface{}{
			"operation": "validate_password",
		})
		
		return false, wumpusErr
	}
	
	logging.Debug("Password validated successfully", "player_id", player.ID)
	return true, nil
}

// SafeGetWumpusAuth gets authentication data with error recovery
func (ops *SafeAuthOperations) SafeGetWumpusAuth(player *storage.Player) (*auth.WumpusAuth, *WumpusError) {
	value, wumpusErr := ops.recoverySystem.SafePropertyAccess(player, "wumpus_auth", nil)
	if wumpusErr != nil {
		return nil, wumpusErr
	}
	
	if value == nil {
		wumpusErr := NewWumpusError(
			ErrorTypeAuthenticationFailure,
			"No authentication data found",
			"Please log in to continue.",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		return nil, wumpusErr
	}
	
	// Try to get auth data (this should succeed since SafePropertyAccess validated it)
	authData, err := auth.GetWumpusAuth(player)
	if err != nil {
		// This shouldn't happen after SafePropertyAccess, but handle it
		wumpusErr := NewWumpusError(
			ErrorTypePropertiesCorruption,
			"Authentication data corrupted during access",
			"Your authentication data is corrupted. Please log in again.",
			err,
		)
		wumpusErr.PlayerID = player.ID
		wumpusErr.RecoveryActions = []RecoveryAction{RecoveryActionKickPlayer}
		
		LogError(wumpusErr, map[string]interface{}{
			"operation": "get_auth",
		})
		
		return nil, wumpusErr
	}
	
	return authData, nil
}

// SafeGetWumpusStats gets player stats with error recovery
func (ops *SafeAuthOperations) SafeGetWumpusStats(player *storage.Player) (auth.WumpusStats, *WumpusError) {
	value, wumpusErr := ops.recoverySystem.SafePropertyAccess(player, "wumpus_stats", auth.WumpusStats{})
	if wumpusErr != nil {
		// Log the recovery but continue with defaults
		LogError(wumpusErr, map[string]interface{}{
			"operation": "get_stats",
			"player_id": player.ID,
		})
	}
	
	if stats, ok := value.(auth.WumpusStats); ok {
		return stats, wumpusErr // Return both stats and any recovery error
	}
	
	// Fallback to direct call (handles corruption gracefully)
	stats := auth.GetWumpusStats(player)
	return stats, wumpusErr
}

// SafeUpdateWumpusStats updates player stats with validation
func (ops *SafeAuthOperations) SafeUpdateWumpusStats(player *storage.Player, stats auth.WumpusStats) *WumpusError {
	var wumpusErr *WumpusError
	
	// Validate stats before updating
	if stats.GamesPlayed < 0 || stats.GamesWon < 0 || stats.WumpusPelts < 0 || stats.TotalDeaths < 0 {
		wumpusErr = NewWumpusError(
			ErrorTypeUserInput,
			"Invalid stats values",
			"Invalid game statistics. Stats have been reset to safe values.",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		
		// Reset to safe values
		stats.GamesPlayed = max(0, stats.GamesPlayed)
		stats.GamesWon = max(0, stats.GamesWon)
		stats.WumpusPelts = max(0, stats.WumpusPelts)
		stats.TotalDeaths = max(0, stats.TotalDeaths)
		
		LogError(wumpusErr, map[string]interface{}{
			"operation": "update_stats",
		})
	}
	
	// Ensure consistency (can't win more games than played)
	if stats.GamesWon > stats.GamesPlayed {
		stats.GamesWon = stats.GamesPlayed
		
		logging.Warn("Fixed inconsistent stats", 
			"player_id", player.ID,
			"games_played", stats.GamesPlayed,
			"games_won", stats.GamesWon)
	}
	
	auth.UpdateWumpusStats(player, stats)
	return wumpusErr
}

// SafeGetWumpusState gets player state with error recovery
func (ops *SafeAuthOperations) SafeGetWumpusState(player *storage.Player) (auth.WumpusState, *WumpusError) {
	value, wumpusErr := ops.recoverySystem.SafePropertyAccess(player, "wumpus_state", auth.WumpusState{
		CurrentInstance: "",
		Health:          100,
		IsInMaze:        false,
	})
	
	if wumpusErr != nil {
		LogError(wumpusErr, map[string]interface{}{
			"operation": "get_state",
			"player_id": player.ID,
		})
	}
	
	if state, ok := value.(auth.WumpusState); ok {
		return state, wumpusErr
	}
	
	// Fallback to direct call (handles corruption gracefully)
	state := auth.GetWumpusState(player)
	return state, wumpusErr
}

// SafeUpdateWumpusState updates player state with validation
func (ops *SafeAuthOperations) SafeUpdateWumpusState(player *storage.Player, state auth.WumpusState) *WumpusError {
	var wumpusErr *WumpusError
	
	// Validate state before updating
	if state.Health < 0 || state.Health > 100 {
		wumpusErr = NewWumpusError(
			ErrorTypeUserInput,
			"Invalid health value",
			"Invalid health value detected. Health has been corrected.",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		
		// Clamp health to valid range
		if state.Health < 0 {
			state.Health = 0
		} else if state.Health > 100 {
			state.Health = 100
		}
		
		LogError(wumpusErr, map[string]interface{}{
			"operation": "update_state",
			"invalid_health": state.Health,
		})
	}
	
	auth.UpdateWumpusState(player, state)
	return wumpusErr
}

// SafeIsPlayerAuthenticated checks if player is authenticated with error recovery
func (ops *SafeAuthOperations) SafeIsPlayerAuthenticated(player *storage.Player) (bool, *WumpusError) {
	_, wumpusErr := ops.SafeGetWumpusAuth(player)
	if wumpusErr != nil {
		// If there's an authentication error, player is not authenticated
		return false, wumpusErr
	}
	return true, nil
}

// RecoverPlayerSession attempts to recover a player's session
func (ops *SafeAuthOperations) RecoverPlayerSession(player *storage.Player) []*WumpusError {
	var errors []*WumpusError
	
	// Check for interrupted game FIRST, before integrity validation resets state
	if interruptedErr := ops.recoverySystem.RecoverFromInterruptedGame(player); interruptedErr != nil {
		errors = append(errors, interruptedErr)
	}
	
	// Check for integrity issues
	integrityErrors := ops.recoverySystem.ValidatePlayerIntegrity(player)
	errors = append(errors, integrityErrors...)
	
	// Log all recovery actions
	for _, err := range errors {
		LogError(err, map[string]interface{}{
			"operation": "recover_session",
			"player_id": player.ID,
		})
	}
	
	return errors
}

// Helper function for max (Go doesn't have this built-in for integers)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}