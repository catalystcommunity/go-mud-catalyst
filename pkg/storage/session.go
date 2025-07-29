package storage

import (
	"fmt"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
)

// SessionState represents the current state of a player session
type SessionState int

const (
	SessionStateConnected SessionState = iota
	SessionStateAuthenticated
	SessionStateDisconnected
)

// SessionManager manages player sessions and bridges the gap between
// server connections and storage player data
type SessionManager struct {
	storage      *Manager
	sessions     map[string]*PlayerSession // connectionID -> session
	playerToConn map[string]string         // playerID -> connectionID
	mutex        sync.RWMutex
}

// PlayerSession represents an active player session
type PlayerSession struct {
	ConnectionID string
	PlayerID     string
	Player       *Player
	State        SessionState
	ConnectedAt  time.Time
	AuthedAt     time.Time
	LastActivity time.Time
	Connection   connection.GameConnection
	mutex        sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(storage *Manager) *SessionManager {
	return &SessionManager{
		storage:      storage,
		sessions:     make(map[string]*PlayerSession),
		playerToConn: make(map[string]string),
	}
}

// CreateSession creates a new player session for a connection
func (sm *SessionManager) CreateSession(connectionID string, conn connection.GameConnection) *PlayerSession {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	session := &PlayerSession{
		ConnectionID: connectionID,
		State:        SessionStateConnected,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		Connection:   conn,
	}
	
	sm.sessions[connectionID] = session
	return session
}

// AuthenticateSession authenticates a session with a player
func (sm *SessionManager) AuthenticateSession(connectionID, playerID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	session, exists := sm.sessions[connectionID]
	if !exists {
		return fmt.Errorf("session not found for connection %s", connectionID)
	}
	
	// Check if player is already connected
	if existingConnID, isConnected := sm.playerToConn[playerID]; isConnected {
		if existingConnID != connectionID {
			return fmt.Errorf("player %s is already connected on another session", playerID)
		}
	}
	
	// Load player data
	player, err := sm.storage.Players().Load(playerID)
	if err != nil {
		return fmt.Errorf("failed to load player %s: %w", playerID, err)
	}
	
	// Check if player is banned
	if player.IsBanned {
		return fmt.Errorf("player %s is banned: %s", playerID, player.BanReason)
	}
	
	// Update session
	session.mutex.Lock()
	session.PlayerID = playerID
	session.Player = player
	session.State = SessionStateAuthenticated
	session.AuthedAt = time.Now()
	session.mutex.Unlock()
	
	// Update login time
	player.LastLogin = time.Now()
	player.Update()
	if err := sm.storage.Players().Save(player); err != nil {
		return fmt.Errorf("failed to update player login time: %w", err)
	}
	
	// Track player connection
	sm.playerToConn[playerID] = connectionID
	
	return nil
}

// GetSession returns a session by connection ID
func (sm *SessionManager) GetSession(connectionID string) (*PlayerSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	session, exists := sm.sessions[connectionID]
	return session, exists
}

// GetSessionByPlayerID returns a session by player ID
func (sm *SessionManager) GetSessionByPlayerID(playerID string) (*PlayerSession, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	if connectionID, exists := sm.playerToConn[playerID]; exists {
		if session, sessionExists := sm.sessions[connectionID]; sessionExists {
			return session, true
		}
	}
	return nil, false
}

// RemoveSession removes a session and cleans up
func (sm *SessionManager) RemoveSession(connectionID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	if session, exists := sm.sessions[connectionID]; exists {
		// Remove player connection mapping
		if session.PlayerID != "" {
			delete(sm.playerToConn, session.PlayerID)
		}
		
		// Update session state
		session.mutex.Lock()
		session.State = SessionStateDisconnected
		session.mutex.Unlock()
		
		// Remove session
		delete(sm.sessions, connectionID)
	}
}

// UpdateActivity updates the last activity time for a session
func (sm *SessionManager) UpdateActivity(connectionID string) {
	sm.mutex.RLock()
	session, exists := sm.sessions[connectionID]
	sm.mutex.RUnlock()
	
	if exists {
		session.mutex.Lock()
		session.LastActivity = time.Now()
		session.mutex.Unlock()
	}
}

// GetAuthenticatedSessions returns all authenticated sessions
func (sm *SessionManager) GetAuthenticatedSessions() []*PlayerSession {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	var sessions []*PlayerSession
	for _, session := range sm.sessions {
		session.mutex.RLock()
		if session.State == SessionStateAuthenticated {
			sessions = append(sessions, session)
		}
		session.mutex.RUnlock()
	}
	return sessions
}

// GetOnlinePlayers returns all online players
func (sm *SessionManager) GetOnlinePlayers() []*Player {
	sessions := sm.GetAuthenticatedSessions()
	players := make([]*Player, 0, len(sessions))
	
	for _, session := range sessions {
		session.mutex.RLock()
		if session.Player != nil {
			players = append(players, session.Player)
		}
		session.mutex.RUnlock()
	}
	return players
}

// IsPlayerOnline checks if a player is currently online
func (sm *SessionManager) IsPlayerOnline(playerID string) bool {
	_, exists := sm.GetSessionByPlayerID(playerID)
	return exists
}

// KickPlayer kicks a player by disconnecting their session
func (sm *SessionManager) KickPlayer(playerID, reason string) error {
	session, exists := sm.GetSessionByPlayerID(playerID)
	if !exists {
		return fmt.Errorf("player %s is not online", playerID)
	}
	
	// Send kick message if possible
	if session.Connection != nil {
		kickMsg := &message.Message{
			Type:     message.MessageTypeError,
			Contents: []byte(fmt.Sprintf("You have been kicked: %s", reason)),
		}
		if data, err := message.MessageWrap(kickMsg); err == nil {
			session.Connection.WriteMessage(data)
		}
	}
	
	// Close connection
	session.Connection.Close()
	
	return nil
}

// BanAndKickPlayer bans a player and kicks them if online
func (sm *SessionManager) BanAndKickPlayer(playerID, reason, adminID string) error {
	// Ban the player in storage
	if err := sm.storage.BanPlayer(playerID, reason, adminID); err != nil {
		return fmt.Errorf("failed to ban player: %w", err)
	}
	
	// Kick if online
	if sm.IsPlayerOnline(playerID) {
		kickReason := fmt.Sprintf("Banned: %s", reason)
		if err := sm.KickPlayer(playerID, kickReason); err != nil {
			// Log error but don't fail the ban
			return fmt.Errorf("player banned but failed to kick: %w", err)
		}
	}
	
	return nil
}

// GetSessionCount returns the total number of active sessions
func (sm *SessionManager) GetSessionCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.sessions)
}

// GetAuthenticatedSessionCount returns the number of authenticated sessions
func (sm *SessionManager) GetAuthenticatedSessionCount() int {
	return len(sm.GetAuthenticatedSessions())
}

// SessionStateToString converts a SessionState to string
func SessionStateToString(state SessionState) string {
	switch state {
	case SessionStateConnected:
		return "CONNECTED"
	case SessionStateAuthenticated:
		return "AUTHENTICATED"
	case SessionStateDisconnected:
		return "DISCONNECTED"
	default:
		return "UNKNOWN"
	}
}

// GetSessionInfo returns session information for debugging/monitoring
func (ps *PlayerSession) GetSessionInfo() map[string]interface{} {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()
	
	info := map[string]interface{}{
		"connection_id":  ps.ConnectionID,
		"player_id":      ps.PlayerID,
		"state":          SessionStateToString(ps.State),
		"connected_at":   ps.ConnectedAt,
		"last_activity":  ps.LastActivity,
	}
	
	if ps.State == SessionStateAuthenticated {
		info["authed_at"] = ps.AuthedAt
	}
	
	if ps.Player != nil {
		info["username"] = ps.Player.Username
		info["display_name"] = ps.Player.DisplayName
		info["current_world"] = ps.Player.CurrentWorldID
		info["current_room"] = ps.Player.CurrentRoomID
		info["is_banned"] = ps.Player.IsBanned
	}
	
	return info
}

// UpdatePlayerLocation updates the player's location and saves to storage
func (ps *PlayerSession) UpdatePlayerLocation(worldID, roomID string) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()
	
	if ps.Player == nil {
		return fmt.Errorf("session not authenticated")
	}
	
	ps.Player.SetLocation(worldID, roomID)
	// Note: This doesn't save to storage directly - use SessionManager.SavePlayerData for that
	return nil
}

// SavePlayerData saves the session's player data to storage
func (sm *SessionManager) SavePlayerData(connectionID string) error {
	session, exists := sm.GetSession(connectionID)
	if !exists {
		return fmt.Errorf("session not found")
	}
	
	session.mutex.RLock()
	player := session.Player
	session.mutex.RUnlock()
	
	if player == nil {
		return fmt.Errorf("session not authenticated")
	}
	
	return sm.storage.Players().Save(player)
}

// CreateSessionEvent creates a custom event for session activities
func CreateSessionEvent(eventType string, connectionID, playerID string, data map[string]interface{}) events.Event {
	eventData := map[string]interface{}{
		"connection_id": connectionID,
		"player_id":     playerID,
		"timestamp":     time.Now(),
	}
	
	// Merge in additional data
	for k, v := range data {
		eventData[k] = v
	}
	
	customEventType := events.EventType(string(events.EventTypeCustomPrefix) + eventType)
	return events.NewEvent(customEventType, nil, eventData)
}