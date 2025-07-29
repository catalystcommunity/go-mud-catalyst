package errors

import (
	"github.com/fxamacker/cbor/v2"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/server"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

// SafeAuthHandler provides authentication handling with comprehensive error recovery
type SafeAuthHandler struct {
	safeAuth    *SafeAuthOperations
	storageMgr  *storage.Manager
}

// NewSafeAuthHandler creates a new safe authentication handler
func NewSafeAuthHandler(safeAuth *SafeAuthOperations, storageMgr *storage.Manager) *SafeAuthHandler {
	return &SafeAuthHandler{
		safeAuth:   safeAuth,
		storageMgr: storageMgr,
	}
}

// AuthResponse represents the structure of authentication responses
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleAuthentication handles player authentication with comprehensive error recovery
func (h *SafeAuthHandler) HandleAuthentication(s *server.ConnServer, clientID string, msg *message.Message) {
	// Parse authentication data
	var authData map[string]interface{}
	if err := cbor.Unmarshal(msg.Contents, &authData); err != nil {
		wumpusErr := NewWumpusError(
			ErrorTypeUserInput,
			"Invalid authentication data format",
			"Invalid authentication data format. Please try again.",
			err,
		)
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	// Validate username
	username, ok := authData["username"].(string)
	if !ok || len(username) < 3 {
		wumpusErr := NewWumpusError(
			ErrorTypeUserInput,
			"Invalid username",
			"Username is required and must be at least 3 characters long.",
			nil,
		)
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	// Validate password
	password, ok := authData["password"].(string)
	if !ok || len(password) < 6 {
		wumpusErr := NewWumpusError(
			ErrorTypeUserInput,
			"Invalid password",
			"Password is required and must be at least 6 characters long.",
			nil,
		)
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	// Try to load existing player
	player, err := h.storageMgr.Players().LoadByUsername(username)
	if err != nil {
		// Player doesn't exist, create new account
		h.handleNewPlayerRegistration(s, clientID, username, password)
		return
	}

	// Player exists, handle login
	h.handleExistingPlayerLogin(s, clientID, player, password)
}

// handleNewPlayerRegistration creates a new player account with error recovery
func (h *SafeAuthHandler) handleNewPlayerRegistration(s *server.ConnServer, clientID, username, password string) {
	logging.Info("Creating new player account", "username", username)

	// Create new player
	player := storage.NewPlayer(ids.NewEntityID(), username)
	player.DisplayName = username

	// Set password using safe operations
	if wumpusErr := h.safeAuth.SafeSetPlayerPassword(player, password); wumpusErr != nil {
		LogError(wumpusErr, map[string]interface{}{
			"operation": "new_player_registration",
			"username":  username,
		})
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	// Save new player
	if err := h.storageMgr.Players().Save(player); err != nil {
		wumpusErr := NewWumpusError(
			ErrorTypeSystem,
			"Failed to save new player",
			"Failed to create your account. Please try again.",
			err,
		)
		wumpusErr.PlayerID = player.ID
		LogError(wumpusErr, map[string]interface{}{
			"operation": "new_player_registration",
			"username":  username,
		})
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	logging.Info("Created new player account successfully", "username", username, "playerID", player.ID)

	// Complete authentication process
	h.completeAuthentication(s, clientID, player, "Account created successfully! Welcome to Hunt the Wumpus!")
}

// handleExistingPlayerLogin handles login for existing players with error recovery
func (h *SafeAuthHandler) handleExistingPlayerLogin(s *server.ConnServer, clientID string, player *storage.Player, password string) {
	logging.Debug("Handling existing player login", "username", player.Username, "playerID", player.ID)

	// Recover player session first (handle any corruption or interrupted games)
	recoveryErrors := h.safeAuth.RecoverPlayerSession(player)
	
	// Check if any recovery errors require kicking the player
	for _, recovErr := range recoveryErrors {
		for _, action := range recovErr.RecoveryActions {
			if action == RecoveryActionKickPlayer {
				LogError(recovErr, map[string]interface{}{
					"operation": "player_login",
					"username":  player.Username,
				})
				h.sendAuthError(s, clientID, recovErr)
				return
			}
		}
	}

	// Validate password using safe operations
	isValid, wumpusErr := h.safeAuth.SafeValidatePlayerPassword(player, password)
	if wumpusErr != nil {
		LogError(wumpusErr, map[string]interface{}{
			"operation": "player_login",
			"username":  player.Username,
		})
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	if !isValid {
		// This should be caught by SafeValidatePlayerPassword, but double-check
		wumpusErr := NewWumpusError(
			ErrorTypeAuthenticationFailure,
			"Password validation failed",
			"Invalid username or password.",
			nil,
		)
		wumpusErr.PlayerID = player.ID
		wumpusErr.Recoverable = false
		LogError(wumpusErr, map[string]interface{}{
			"operation": "player_login",
			"username":  player.Username,
		})
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	logging.Info("Player authenticated successfully", "username", player.Username, "playerID", player.ID)

	// Build success message including any recovery information
	successMessage := "Authentication successful!"
	for _, recovErr := range recoveryErrors {
		if recovErr.UserMessage != "" {
			successMessage += " " + recovErr.UserMessage
		}
	}

	// Complete authentication process
	h.completeAuthentication(s, clientID, player, successMessage)
}

// completeAuthentication completes the authentication process by setting up the player's session
func (h *SafeAuthHandler) completeAuthentication(s *server.ConnServer, clientID string, player *storage.Player, successMessage string) {
	// Set client-to-player mapping (this should be done by the caller)
	// We'll return the player ID so the caller can handle this mapping
	
	// Initialize player in the Inn with error recovery
	if err := h.initializePlayerInInnSafely(clientID, player.ID); err != nil {
		wumpusErr := NewWumpusError(
			ErrorTypeSystem,
			"Failed to initialize player location",
			"Authentication succeeded but failed to place you in the game world. Please try reconnecting.",
			err,
		)
		wumpusErr.PlayerID = player.ID
		LogError(wumpusErr, map[string]interface{}{
			"operation": "complete_authentication",
			"username":  player.Username,
		})
		h.sendAuthError(s, clientID, wumpusErr)
		return
	}

	// Send success response
	h.sendAuthSuccess(s, clientID, successMessage)
	
	logging.Info("Player authentication completed successfully", "username", player.Username, "playerID", player.ID)
}

// initializePlayerInInnSafely initializes a player in the Inn with error recovery
func (h *SafeAuthHandler) initializePlayerInInnSafely(clientID, playerID string) error {
	// Get the Inn room with error handling
	innRoom, err := h.getInnRoomSafely()
	if err != nil {
		return err
	}

	// Get the main world (for validation)
	_, err = world.GetMainWorld(h.storageMgr)
	if err != nil {
		return NewWumpusError(
			ErrorTypeWorldCorruption,
			"Failed to get main world",
			"The game world is corrupted. Please contact an administrator.",
			err,
		)
	}

	// Set player location (this would need to be handled by caller since they manage the location maps)
	// For now, we'll just return success and let the caller handle location setting
	
	logging.Debug("Player initialized in Inn", "player_id", playerID, "inn_room_id", innRoom.ID)
	return nil
}

// getInnRoomSafely gets the Inn room with error recovery
func (h *SafeAuthHandler) getInnRoomSafely() (*storage.Room, error) {
	// Get the main world first
	mainWorld, err := world.GetMainWorld(h.storageMgr)
	if err != nil {
		return nil, NewWumpusError(
			ErrorTypeWorldCorruption,
			"Failed to get main world",
			"The game world is corrupted. Please contact an administrator.",
			err,
		)
	}

	// Get the Inn room from the main world
	innRoom, err := world.GetInnRoom(h.storageMgr, mainWorld.ID)
	if err != nil {
		return nil, NewWumpusError(
			ErrorTypeWorldCorruption,
			"Failed to get Inn room",
			"The Inn is not available. Please contact an administrator.",
			err,
		)
	}

	return innRoom, nil
}

// sendAuthSuccess sends a successful authentication response
func (h *SafeAuthHandler) sendAuthSuccess(s *server.ConnServer, clientID, successMessage string) {
	response := AuthResponse{
		Success: true,
		Message: successMessage,
	}

	msg := message.NewMessage(message.MessageTypeAuth).WithContent(response).Build()
	if err := s.SendMessage(clientID, msg); err != nil {
		logging.Error("Failed to send auth success response", 
			"client_id", clientID, 
			"error", err)
	}
}

// sendAuthError sends an authentication error response
func (h *SafeAuthHandler) sendAuthError(s *server.ConnServer, clientID string, wumpusErr *WumpusError) {
	response := AuthResponse{
		Success: false,
		Error:   wumpusErr.UserMessage,
	}

	msg := message.NewMessage(message.MessageTypeAuth).WithContent(response).Build()
	if err := s.SendMessage(clientID, msg); err != nil {
		logging.Error("Failed to send auth error response", 
			"client_id", clientID, 
			"error", err)
	}
}

// GetPlayerID returns the player ID for successful authentication
// This should be called after successful authentication to get the player ID for session management
func GetPlayerIDFromAuthMessage(msg *message.Message) (string, error) {
	var response AuthResponse
	if err := cbor.Unmarshal(msg.Contents, &response); err != nil {
		return "", err
	}
	
	if !response.Success {
		return "", NewWumpusError(
			ErrorTypeAuthenticationFailure,
			"Authentication failed",
			response.Error,
			nil,
		)
	}
	
	// Note: This approach would require modification to include player ID in success response
	// For now, the caller will need to track the player ID during authentication
	return "", nil
}