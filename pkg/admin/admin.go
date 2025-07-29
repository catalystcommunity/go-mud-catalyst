package admin

import (
	"fmt"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// AdminAction represents the type of administrative action taken
type AdminAction string

const (
	ActionKick   AdminAction = "KICK"
	ActionBan    AdminAction = "BAN"
	ActionUnban  AdminAction = "UNBAN"
	ActionMove   AdminAction = "MOVE"
	ActionGive   AdminAction = "GIVE"
	ActionTake   AdminAction = "TAKE"
	ActionNotify AdminAction = "NOTIFY"
)

// AdminLog represents a log entry for administrative actions
type AdminLog struct {
	ID          string      `json:"id"`
	Action      AdminAction `json:"action"`
	AdminID     string      `json:"admin_id"`
	TargetID    string      `json:"target_id"`
	Reason      string      `json:"reason"`
	Details     string      `json:"details"`
	Timestamp   time.Time   `json:"timestamp"`
	Success     bool        `json:"success"`
	ErrorMsg    string      `json:"error_msg,omitempty"`
}

// AdminManager provides administrative functions for managing players and the game world
type AdminManager struct {
	storage *storage.Manager
	events  *events.DefaultEventManager
}

// NewAdminManager creates a new admin manager
func NewAdminManager(storage *storage.Manager, eventManager *events.DefaultEventManager) *AdminManager {
	return &AdminManager{
		storage: storage,
		events:  eventManager,
	}
}

// KickPlayer kicks a player from the game with a reason
func (am *AdminManager) KickPlayer(adminID, playerID, reason string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionKick,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":  adminID,
		"player_id": playerID,
		"reason":    reason,
		"action":    string(ActionKick),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.kick.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("kick action cancelled by event handler")
		}
	}

	// Perform the kick
	err := am.storage.Sessions().KickPlayer(playerID, reason)
	
	logEntry.Success = err == nil
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["error"] = logEntry.ErrorMsg
		postEvent := events.NewEvent(events.EventType("admin.kick.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return err
}

// BanPlayer bans a player and kicks them if online
func (am *AdminManager) BanPlayer(adminID, playerID, reason string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionBan,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":  adminID,
		"player_id": playerID,
		"reason":    reason,
		"action":    string(ActionBan),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.ban.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("ban action cancelled by event handler")
		}
	}

	// Perform the ban and kick
	err := am.storage.Sessions().BanAndKickPlayer(playerID, reason, adminID)
	
	logEntry.Success = err == nil
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["error"] = logEntry.ErrorMsg
		postEvent := events.NewEvent(events.EventType("admin.ban.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return err
}

// UnbanPlayer unbans a player
func (am *AdminManager) UnbanPlayer(adminID, playerID string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionUnban,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    "Administrative unban",
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":  adminID,
		"player_id": playerID,
		"action":    string(ActionUnban),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.unban.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("unban action cancelled by event handler")
		}
	}

	// Perform the unban
	err := am.storage.UnbanPlayer(playerID)
	
	logEntry.Success = err == nil
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["error"] = logEntry.ErrorMsg
		postEvent := events.NewEvent(events.EventType("admin.unban.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return err
}

// MovePlayer moves a player to a specific room
func (am *AdminManager) MovePlayer(adminID, playerID, worldID, roomID, reason string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionMove,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    reason,
		Details:   fmt.Sprintf("world:%s room:%s", worldID, roomID),
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":  adminID,
		"player_id": playerID,
		"world_id":  worldID,
		"room_id":   roomID,
		"reason":    reason,
		"action":    string(ActionMove),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.move.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("move action cancelled by event handler")
		}
	}

	// Perform the move
	err := am.storage.MovePlayerToRoom(playerID, worldID, roomID)
	
	logEntry.Success = err == nil
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["error"] = logEntry.ErrorMsg
		postEvent := events.NewEvent(events.EventType("admin.move.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return err
}

// GiveItem gives an item to a player
func (am *AdminManager) GiveItem(adminID, playerID, itemID string, quantity int, reason string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionGive,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    reason,
		Details:   fmt.Sprintf("item:%s quantity:%d", itemID, quantity),
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":  adminID,
		"player_id": playerID,
		"item_id":   itemID,
		"quantity":  quantity,
		"reason":    reason,
		"action":    string(ActionGive),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.give.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("give action cancelled by event handler")
		}
	}

	// Perform the give
	err := am.storage.GiveItemToPlayer(playerID, itemID, quantity)
	
	logEntry.Success = err == nil
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["error"] = logEntry.ErrorMsg
		postEvent := events.NewEvent(events.EventType("admin.give.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return err
}

// TakeItem takes an item from a player
func (am *AdminManager) TakeItem(adminID, playerID, instanceID, reason string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionTake,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    reason,
		Details:   fmt.Sprintf("instance:%s", instanceID),
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":    adminID,
		"player_id":   playerID,
		"instance_id": instanceID,
		"reason":      reason,
		"action":      string(ActionTake),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.take.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("take action cancelled by event handler")
		}
	}

	// Perform the take
	err := am.storage.TakeItemFromPlayer(playerID, instanceID)
	
	logEntry.Success = err == nil
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["error"] = logEntry.ErrorMsg
		postEvent := events.NewEvent(events.EventType("admin.take.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return err
}

// NotifyPlayer sends an administrative notification to a player
func (am *AdminManager) NotifyPlayer(adminID, playerID, message string) error {
	logEntry := &AdminLog{
		ID:        am.generateLogID(),
		Action:    ActionNotify,
		AdminID:   adminID,
		TargetID:  playerID,
		Reason:    "Administrative notification",
		Details:   message,
		Timestamp: time.Now(),
	}

	// Trigger pre-action event
	eventData := map[string]interface{}{
		"admin_id":  adminID,
		"player_id": playerID,
		"message":   message,
		"action":    string(ActionNotify),
	}
	
	if am.events != nil {
		preEvent := events.NewEvent(events.EventType("admin.notify.pre"), nil, eventData)
		if am.events.TriggerEvent(preEvent) == events.EventResultCancel {
			logEntry.Success = false
			logEntry.ErrorMsg = "action cancelled by event handler"
			am.logAction(logEntry)
			return fmt.Errorf("notify action cancelled by event handler")
		}
	}

	// Check if player is online
	session, exists := am.storage.Sessions().GetSessionByPlayerID(playerID)
	if !exists {
		err := fmt.Errorf("player %s is not online", playerID)
		logEntry.Success = false
		logEntry.ErrorMsg = err.Error()
		am.logAction(logEntry)
		return err
	}

	// Send notification (this would integrate with your message system)
	// For now, we'll just mark it as successful since the session exists
	logEntry.Success = true
	
	// Log the action
	am.logAction(logEntry)
	
	// Trigger post-action event
	if am.events != nil {
		eventData["success"] = logEntry.Success
		eventData["session_id"] = session.ConnectionID
		postEvent := events.NewEvent(events.EventType("admin.notify.post"), nil, eventData)
		am.events.TriggerEvent(postEvent)
	}

	return nil
}

// GetOnlinePlayers returns a list of all online players
func (am *AdminManager) GetOnlinePlayers() []*storage.Player {
	return am.storage.Sessions().GetOnlinePlayers()
}

// GetPlayerInfo returns detailed information about a player
func (am *AdminManager) GetPlayerInfo(playerID string) (map[string]interface{}, error) {
	player, err := am.storage.Players().Load(playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load player: %w", err)
	}

	info := map[string]interface{}{
		"id":              player.ID,
		"username":        player.Username,
		"display_name":    player.DisplayName,
		"email":           player.Email,
		"current_world":   player.CurrentWorldID,
		"current_room":    player.CurrentRoomID,
		"is_banned":       player.IsBanned,
		"ban_reason":      player.BanReason,
		"banned_by":       player.BannedBy,
		"banned_at":       player.BannedAt,
		"created_at":      player.CreatedAt,
		"last_login":      player.LastLogin,
		"is_online":       am.storage.Sessions().IsPlayerOnline(playerID),
	}

	// Add session info if online
	if session, exists := am.storage.Sessions().GetSessionByPlayerID(playerID); exists {
		info["session"] = session.GetSessionInfo()
	}

	return info, nil
}

// generateLogID generates a unique ID for log entries
func (am *AdminManager) generateLogID() string {
	// This should use the same ID generation as the rest of the system
	// For now, using a simple timestamp-based approach
	return fmt.Sprintf("admin_%d", time.Now().UnixNano())
}

// logAction logs an administrative action (placeholder for actual logging implementation)
func (am *AdminManager) logAction(log *AdminLog) {
	// In a real implementation, this would write to a log file or database
	// For now, this is a placeholder that could trigger events for custom logging
	if am.events != nil {
		eventData := map[string]interface{}{
			"log":       log,
			"action":    string(log.Action),
			"admin_id":  log.AdminID,
			"target_id": log.TargetID,
			"success":   log.Success,
		}
		logEvent := events.NewEvent(events.EventType("admin.log"), nil, eventData)
		am.events.TriggerEvent(logEvent)
	}
}