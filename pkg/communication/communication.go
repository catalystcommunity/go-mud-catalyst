package communication

import (
	"fmt"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// Message types for world communication
const (
	MessageTypeSay   = message.MessageTypeCustom + 1
	MessageTypeShout = message.MessageTypeCustom + 2
	MessageTypeEmote = message.MessageTypeCustom + 3
	MessageTypeTell  = message.MessageTypeCustom + 4
)

// CommunicationData represents the data structure for communication messages
type CommunicationData struct {
	Message    string `cbor:"message"`
	SenderID   string `cbor:"sender_id"`
	SenderName string `cbor:"sender_name"`
	RoomID     string `cbor:"room_id,omitempty"`
	WorldID    string `cbor:"world_id,omitempty"`
	TargetID   string `cbor:"target_id,omitempty"`  // For tells
	TargetName string `cbor:"target_name,omitempty"` // For tells
	Timestamp  int64  `cbor:"timestamp"`
}

// CommunicationManager handles world-aware communication
type CommunicationManager struct {
	storageManager *storage.Manager
	eventManager   events.EventManager
	
	// Player location tracking
	playerLocations map[string]PlayerLocation // playerID -> location
	locationMutex   sync.RWMutex
	
	// Communication channels
	channels map[string]*Channel
	channelMutex sync.RWMutex
	
	// Message handlers
	messageHandlers map[int64]func(*message.Message, string) error
	handlerMutex    sync.RWMutex
}

// PlayerLocation tracks where a player is located
type PlayerLocation struct {
	PlayerID string
	WorldID  string
	RoomID   string
	UpdatedAt time.Time
}

// Channel represents a communication channel
type Channel struct {
	ID          string
	Name        string
	Type        string // "room", "world", "global", "private"
	WorldID     string // For world-specific channels
	RoomID      string // For room-specific channels
	Members     map[string]bool // playerID -> active
	CreatedAt   time.Time
	Properties  map[string]interface{}
}

// NewCommunicationManager creates a new communication manager
func NewCommunicationManager(storageManager *storage.Manager, eventManager events.EventManager) *CommunicationManager {
	cm := &CommunicationManager{
		storageManager:  storageManager,
		eventManager:    eventManager,
		playerLocations: make(map[string]PlayerLocation),
		channels:        make(map[string]*Channel),
		messageHandlers: make(map[int64]func(*message.Message, string) error),
	}
	
	// Register default message handlers
	cm.registerDefaultHandlers()
	
	return cm
}

// registerDefaultHandlers registers the default communication message handlers
func (cm *CommunicationManager) registerDefaultHandlers() {
	cm.messageHandlers[MessageTypeSay] = cm.handleSay
	cm.messageHandlers[MessageTypeShout] = cm.handleShout
	cm.messageHandlers[MessageTypeEmote] = cm.handleEmote
	cm.messageHandlers[MessageTypeTell] = cm.handleTell
	
	// Also handle standard chat messages for integration
	cm.messageHandlers[message.MessageTypeChat] = cm.handleChat
	cm.messageHandlers[message.MessageTypeWhisper] = cm.handleWhisper
	cm.messageHandlers[message.MessageTypeBroadcast] = cm.handleBroadcast
}

// UpdatePlayerLocation updates a player's location
func (cm *CommunicationManager) UpdatePlayerLocation(playerID, worldID, roomID string) {
	cm.locationMutex.Lock()
	defer cm.locationMutex.Unlock()
	
	cm.playerLocations[playerID] = PlayerLocation{
		PlayerID:  playerID,
		WorldID:   worldID,
		RoomID:    roomID,
		UpdatedAt: time.Now(),
	}
}

// GetPlayerLocation returns a player's current location
func (cm *CommunicationManager) GetPlayerLocation(playerID string) (PlayerLocation, bool) {
	cm.locationMutex.RLock()
	defer cm.locationMutex.RUnlock()
	
	location, exists := cm.playerLocations[playerID]
	return location, exists
}

// RemovePlayerLocation removes a player's location (when they disconnect)
func (cm *CommunicationManager) RemovePlayerLocation(playerID string) {
	cm.locationMutex.Lock()
	defer cm.locationMutex.Unlock()
	
	delete(cm.playerLocations, playerID)
}

// GetPlayersInRoom returns all players in the specified room
func (cm *CommunicationManager) GetPlayersInRoom(worldID, roomID string) []string {
	cm.locationMutex.RLock()
	defer cm.locationMutex.RUnlock()
	
	var players []string
	for playerID, location := range cm.playerLocations {
		if location.WorldID == worldID && location.RoomID == roomID {
			players = append(players, playerID)
		}
	}
	return players
}

// GetPlayersInWorld returns all players in the specified world
func (cm *CommunicationManager) GetPlayersInWorld(worldID string) []string {
	cm.locationMutex.RLock()
	defer cm.locationMutex.RUnlock()
	
	var players []string
	for playerID, location := range cm.playerLocations {
		if location.WorldID == worldID {
			players = append(players, playerID)
		}
	}
	return players
}

// HandleMessage processes a communication message
func (cm *CommunicationManager) HandleMessage(msg *message.Message, senderID string) error {
	cm.handlerMutex.RLock()
	handler, exists := cm.messageHandlers[msg.Type]
	cm.handlerMutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("no handler for message type %d", msg.Type)
	}
	
	return handler(msg, senderID)
}

// RegisterMessageHandler registers a custom message handler
func (cm *CommunicationManager) RegisterMessageHandler(messageType int64, handler func(*message.Message, string) error) {
	cm.handlerMutex.Lock()
	defer cm.handlerMutex.Unlock()
	
	cm.messageHandlers[messageType] = handler
}

// CreateChannel creates a new communication channel
func (cm *CommunicationManager) CreateChannel(id, name, channelType string) *Channel {
	cm.channelMutex.Lock()
	defer cm.channelMutex.Unlock()
	
	channel := &Channel{
		ID:         id,
		Name:       name,
		Type:       channelType,
		Members:    make(map[string]bool),
		CreatedAt:  time.Now(),
		Properties: make(map[string]interface{}),
	}
	
	cm.channels[id] = channel
	return channel
}

// GetChannel returns a channel by ID
func (cm *CommunicationManager) GetChannel(id string) (*Channel, bool) {
	cm.channelMutex.RLock()
	defer cm.channelMutex.RUnlock()
	
	channel, exists := cm.channels[id]
	return channel, exists
}

// JoinChannel adds a player to a channel
func (cm *CommunicationManager) JoinChannel(channelID, playerID string) error {
	cm.channelMutex.Lock()
	defer cm.channelMutex.Unlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return fmt.Errorf("channel %s not found", channelID)
	}
	
	channel.Members[playerID] = true
	return nil
}

// LeaveChannel removes a player from a channel
func (cm *CommunicationManager) LeaveChannel(channelID, playerID string) error {
	cm.channelMutex.Lock()
	defer cm.channelMutex.Unlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return fmt.Errorf("channel %s not found", channelID)
	}
	
	delete(channel.Members, playerID)
	return nil
}

// GetChannelMembers returns all members of a channel
func (cm *CommunicationManager) GetChannelMembers(channelID string) []string {
	cm.channelMutex.RLock()
	defer cm.channelMutex.RUnlock()
	
	channel, exists := cm.channels[channelID]
	if !exists {
		return nil
	}
	
	var members []string
	for playerID := range channel.Members {
		members = append(members, playerID)
	}
	return members
}

// SendToRoom sends a message to all players in a room
func (cm *CommunicationManager) SendToRoom(worldID, roomID string, msg *message.Message) error {
	players := cm.GetPlayersInRoom(worldID, roomID)
	
	// Trigger event for room message
	event := cm.eventManager.CreateEvent(events.EventTypeMessageSent, cm, map[string]interface{}{
		"message_type": "room",
		"world_id":     worldID,
		"room_id":      roomID,
		"recipients":   players,
		"message":      msg,
	})
	
	cm.eventManager.TriggerEvent(event)
	
	return nil
}

// SendToWorld sends a message to all players in a world
func (cm *CommunicationManager) SendToWorld(worldID string, msg *message.Message) error {
	players := cm.GetPlayersInWorld(worldID)
	
	// Trigger event for world message
	event := cm.eventManager.CreateEvent(events.EventTypeMessageSent, cm, map[string]interface{}{
		"message_type": "world",
		"world_id":     worldID,
		"recipients":   players,
		"message":      msg,
	})
	
	cm.eventManager.TriggerEvent(event)
	
	return nil
}

// SendToPlayer sends a message to a specific player
func (cm *CommunicationManager) SendToPlayer(playerID string, msg *message.Message) error {
	// Trigger event for private message
	event := cm.eventManager.CreateEvent(events.EventTypeMessageSent, cm, map[string]interface{}{
		"message_type": "private",
		"recipient":    playerID,
		"message":      msg,
	})
	
	cm.eventManager.TriggerEvent(event)
	
	return nil
}

// SendToChannel sends a message to all members of a channel
func (cm *CommunicationManager) SendToChannel(channelID string, msg *message.Message) error {
	members := cm.GetChannelMembers(channelID)
	
	// Trigger event for channel message
	event := cm.eventManager.CreateEvent(events.EventTypeMessageSent, cm, map[string]interface{}{
		"message_type": "channel",
		"channel_id":   channelID,
		"recipients":   members,
		"message":      msg,
	})
	
	cm.eventManager.TriggerEvent(event)
	
	return nil
}