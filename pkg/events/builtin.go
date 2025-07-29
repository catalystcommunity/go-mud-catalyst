package events

import (
	"context"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/message"
)

// Built-in event types with specific data structures

// ConnectionEvent represents connection-related events
type ConnectionEvent struct {
	*BaseEvent
	ConnectionID   string
	Connection     connection.GameConnection
	RemoteAddr     string
	ConnectionType connection.ConnectionType
	Error          error
}

// NewConnectionEvent creates a new connection event
func NewConnectionEvent(eventType EventType, connectionID string, conn connection.GameConnection, err error) *ConnectionEvent {
	data := map[string]interface{}{
		"connection_id":   connectionID,
		"remote_addr":     "",
		"connection_type": "",
	}
	
	remoteAddr := ""
	connType := connection.ConnectionTypeTCP
	if conn != nil {
		remoteAddr = conn.RemoteAddrWithProtocol()
		connType = conn.ConnectionType()
		data["remote_addr"] = remoteAddr
		data["connection_type"] = string(connType)
	}
	
	if err != nil {
		data["error"] = err.Error()
	}
	
	return &ConnectionEvent{
		BaseEvent:      NewEvent(eventType, conn, data),
		ConnectionID:   connectionID,
		Connection:     conn,
		RemoteAddr:     remoteAddr,
		ConnectionType: connType,
		Error:          err,
	}
}

// AuthEvent represents authentication-related events
type AuthEvent struct {
	*BaseEvent
	ClientID     string
	Username     string
	AuthMethod   string
	Success      bool
	Error        error
	Metadata     map[string]interface{}
}

// NewAuthEvent creates a new authentication event
func NewAuthEvent(eventType EventType, clientID, username, authMethod string, success bool, err error, metadata map[string]interface{}) *AuthEvent {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	
	data := map[string]interface{}{
		"client_id":   clientID,
		"username":    username,
		"auth_method": authMethod,
		"success":     success,
		"metadata":    metadata,
	}
	
	if err != nil {
		data["error"] = err.Error()
	}
	
	return &AuthEvent{
		BaseEvent:  NewEvent(eventType, nil, data),
		ClientID:   clientID,
		Username:   username,
		AuthMethod: authMethod,
		Success:    success,
		Error:      err,
		Metadata:   metadata,
	}
}

// MessageEvent represents message-related events
type MessageEvent struct {
	*BaseEvent
	ClientID      string
	MessageType   int64
	Message       *message.Message
	Direction     string // "inbound" or "outbound"
	Size          int
	ProcessingTime time.Duration
}

// NewMessageEvent creates a new message event
func NewMessageEvent(eventType EventType, clientID string, msg *message.Message, direction string) *MessageEvent {
	var msgData []byte
	var msgType int64
	var size int
	
	if msg != nil {
		msgType = msg.Type
		msgData, _ = message.MessageWrap(msg)
		size = len(msgData)
	}
	
	data := map[string]interface{}{
		"client_id":    clientID,
		"message_type": msgType,
		"direction":    direction,
		"size":         size,
	}
	
	if msg != nil && msg.ID != "" {
		data["message_id"] = msg.ID
	}
	
	return &MessageEvent{
		BaseEvent:   NewEvent(eventType, msg, data),
		ClientID:    clientID,
		MessageType: msgType,
		Message:     msg,
		Direction:   direction,
		Size:        size,
	}
}

// PlayerEvent represents player/game-related events
type PlayerEvent struct {
	*BaseEvent
	PlayerID   string
	PlayerName string
	RoomID     string
	Action     string
	Target     string
	GameData   map[string]interface{}
}

// NewPlayerEvent creates a new player event
func NewPlayerEvent(eventType EventType, playerID, playerName, roomID, action, target string, gameData map[string]interface{}) *PlayerEvent {
	if gameData == nil {
		gameData = make(map[string]interface{})
	}
	
	data := map[string]interface{}{
		"player_id":   playerID,
		"player_name": playerName,
		"room_id":     roomID,
		"action":      action,
		"target":      target,
		"game_data":   gameData,
	}
	
	return &PlayerEvent{
		BaseEvent:  NewEvent(eventType, nil, data),
		PlayerID:   playerID,
		PlayerName: playerName,
		RoomID:     roomID,
		Action:     action,
		Target:     target,
		GameData:   gameData,
	}
}

// ServerEvent represents server lifecycle events
type ServerEvent struct {
	*BaseEvent
	ServerID string
	Host     string
	Port     string
	Config   map[string]interface{}
	Error    error
}

// NewServerEvent creates a new server event
func NewServerEvent(eventType EventType, serverID, host, port string, config map[string]interface{}, err error) *ServerEvent {
	if config == nil {
		config = make(map[string]interface{})
	}
	
	data := map[string]interface{}{
		"server_id": serverID,
		"host":      host,
		"port":      port,
		"config":    config,
	}
	
	if err != nil {
		data["error"] = err.Error()
	}
	
	return &ServerEvent{
		BaseEvent: NewEvent(eventType, nil, data),
		ServerID:  serverID,
		Host:      host,
		Port:      port,
		Config:    config,
		Error:     err,
	}
}

// SystemEvent represents system-level events (signals, shutdowns, etc.)
type SystemEvent struct {
	*BaseEvent
	Signal     string
	ProcessID  int
	Reason     string
	SystemData map[string]interface{}
}

// NewSystemEvent creates a new system event
func NewSystemEvent(eventType EventType, signal string, processID int, reason string, ctx map[string]interface{}) *SystemEvent {
	if ctx == nil {
		ctx = make(map[string]interface{})
	}
	
	data := map[string]interface{}{
		"signal":     signal,
		"process_id": processID,
		"reason":     reason,
		"context":    ctx,
	}
	
	return &SystemEvent{
		BaseEvent:  NewEvent(eventType, nil, data),
		Signal:     signal,
		ProcessID:  processID,
		Reason:     reason,
		SystemData: ctx,
	}
}

// RoomEvent represents room/channel events
type RoomEvent struct {
	*BaseEvent
	RoomID      string
	RoomName    string
	ClientID    string
	ClientName  string
	Action      string // "join", "leave", "create", "delete"
	ClientCount int
}

// NewRoomEvent creates a new room event
func NewRoomEvent(eventType EventType, roomID, roomName, clientID, clientName, action string, clientCount int) *RoomEvent {
	data := map[string]interface{}{
		"room_id":      roomID,
		"room_name":    roomName,
		"client_id":    clientID,
		"client_name":  clientName,
		"action":       action,
		"client_count": clientCount,
	}
	
	return &RoomEvent{
		BaseEvent:   NewEvent(eventType, nil, data),
		RoomID:      roomID,
		RoomName:    roomName,
		ClientID:    clientID,
		ClientName:  clientName,
		Action:      action,
		ClientCount: clientCount,
	}
}

// Convenience functions for creating common events

// CreateConnectionOpenEvent creates a connection open event
func CreateConnectionOpenEvent(connectionID string, conn connection.GameConnection) *ConnectionEvent {
	return NewConnectionEvent(EventTypeConnectionOpen, connectionID, conn, nil)
}

// CreateConnectionCloseEvent creates a connection close event
func CreateConnectionCloseEvent(connectionID string, conn connection.GameConnection, err error) *ConnectionEvent {
	return NewConnectionEvent(EventTypeConnectionClose, connectionID, conn, err)
}

// CreateConnectionErrorEvent creates a connection error event
func CreateConnectionErrorEvent(connectionID string, conn connection.GameConnection, err error) *ConnectionEvent {
	return NewConnectionEvent(EventTypeConnectionError, connectionID, conn, err)
}

// CreateAuthAttemptEvent creates an authentication attempt event
func CreateAuthAttemptEvent(clientID, username, authMethod string, metadata map[string]interface{}) *AuthEvent {
	return NewAuthEvent(EventTypeAuthAttempt, clientID, username, authMethod, false, nil, metadata)
}

// CreateAuthSuccessEvent creates a successful authentication event
func CreateAuthSuccessEvent(clientID, username, authMethod string, metadata map[string]interface{}) *AuthEvent {
	return NewAuthEvent(EventTypeAuthSuccess, clientID, username, authMethod, true, nil, metadata)
}

// CreateAuthFailureEvent creates a failed authentication event
func CreateAuthFailureEvent(clientID, username, authMethod string, err error, metadata map[string]interface{}) *AuthEvent {
	return NewAuthEvent(EventTypeAuthFailure, clientID, username, authMethod, false, err, metadata)
}

// CreateMessageReceivedEvent creates a message received event
func CreateMessageReceivedEvent(clientID string, msg *message.Message) *MessageEvent {
	return NewMessageEvent(EventTypeMessageReceived, clientID, msg, "inbound")
}

// CreateMessageSentEvent creates a message sent event
func CreateMessageSentEvent(clientID string, msg *message.Message) *MessageEvent {
	return NewMessageEvent(EventTypeMessageSent, clientID, msg, "outbound")
}

// CreatePlayerJoinEvent creates a player join event
func CreatePlayerJoinEvent(playerID, playerName, roomID string, gameData map[string]interface{}) *PlayerEvent {
	return NewPlayerEvent(EventTypePlayerJoin, playerID, playerName, roomID, "join", "", gameData)
}

// CreatePlayerLeaveEvent creates a player leave event
func CreatePlayerLeaveEvent(playerID, playerName, roomID string, gameData map[string]interface{}) *PlayerEvent {
	return NewPlayerEvent(EventTypePlayerLeave, playerID, playerName, roomID, "leave", "", gameData)
}

// CreatePlayerMoveEvent creates a player move event
func CreatePlayerMoveEvent(playerID, playerName, fromRoom, toRoom string, gameData map[string]interface{}) *PlayerEvent {
	if gameData == nil {
		gameData = make(map[string]interface{})
	}
	gameData["from_room"] = fromRoom
	gameData["to_room"] = toRoom
	
	return NewPlayerEvent(EventTypePlayerMove, playerID, playerName, toRoom, "move", "", gameData)
}

// CreatePlayerActionEvent creates a player action event
func CreatePlayerActionEvent(playerID, playerName, roomID, action, target string, gameData map[string]interface{}) *PlayerEvent {
	return NewPlayerEvent(EventTypePlayerAction, playerID, playerName, roomID, action, target, gameData)
}

// CreateRoomJoinEvent creates a room join event
func CreateRoomJoinEvent(roomID, roomName, clientID, clientName string, clientCount int) *RoomEvent {
	return NewRoomEvent(EventTypeRoomJoin, roomID, roomName, clientID, clientName, "join", clientCount)
}

// CreateRoomLeaveEvent creates a room leave event
func CreateRoomLeaveEvent(roomID, roomName, clientID, clientName string, clientCount int) *RoomEvent {
	return NewRoomEvent(EventTypeRoomLeave, roomID, roomName, clientID, clientName, "leave", clientCount)
}

// CreateServerStartEvent creates a server start event
func CreateServerStartEvent(serverID, host, port string, config map[string]interface{}) *ServerEvent {
	return NewServerEvent(EventTypeServerStart, serverID, host, port, config, nil)
}

// CreateServerStopEvent creates a server stop event
func CreateServerStopEvent(serverID, host, port string, reason error) *ServerEvent {
	return NewServerEvent(EventTypeServerStop, serverID, host, port, nil, reason)
}

// CreateServerShutdownEvent creates a server shutdown event
func CreateServerShutdownEvent(serverID, host, port string, graceful bool) *ServerEvent {
	config := map[string]interface{}{
		"graceful": graceful,
	}
	return NewServerEvent(EventTypeServerShutdown, serverID, host, port, config, nil)
}

// EventBuilder provides a fluent API for building events
type EventBuilder struct {
	eventType EventType
	source    interface{}
	data      map[string]interface{}
	ctx       context.Context
}

// NewEventBuilder creates a new event builder
func NewEventBuilder(eventType EventType) *EventBuilder {
	return &EventBuilder{
		eventType: eventType,
		data:      make(map[string]interface{}),
		ctx:       context.Background(),
	}
}

// WithSource sets the event source
func (eb *EventBuilder) WithSource(source interface{}) *EventBuilder {
	eb.source = source
	return eb
}

// WithData sets event data
func (eb *EventBuilder) WithData(key string, value interface{}) *EventBuilder {
	eb.data[key] = value
	return eb
}

// WithContext sets the event context
func (eb *EventBuilder) WithContext(ctx context.Context) *EventBuilder {
	eb.ctx = ctx
	return eb
}

// WithClientID sets the client ID
func (eb *EventBuilder) WithClientID(clientID string) *EventBuilder {
	eb.data["client_id"] = clientID
	return eb
}

// WithConnectionID sets the connection ID
func (eb *EventBuilder) WithConnectionID(connectionID string) *EventBuilder {
	eb.data["connection_id"] = connectionID
	return eb
}

// WithError sets an error
func (eb *EventBuilder) WithError(err error) *EventBuilder {
	if err != nil {
		eb.data["error"] = err.Error()
	}
	return eb
}

// Build creates the event
func (eb *EventBuilder) Build() Event {
	return NewEventWithContext(eb.ctx, eb.eventType, eb.source, eb.data)
}

// Predefined event builders

// ConnectionOpen creates a connection open event builder
func ConnectionOpen() *EventBuilder {
	return NewEventBuilder(EventTypeConnectionOpen)
}

// ConnectionClose creates a connection close event builder
func ConnectionClose() *EventBuilder {
	return NewEventBuilder(EventTypeConnectionClose)
}

// AuthAttempt creates an auth attempt event builder
func AuthAttempt() *EventBuilder {
	return NewEventBuilder(EventTypeAuthAttempt)
}

// AuthSuccess creates an auth success event builder
func AuthSuccess() *EventBuilder {
	return NewEventBuilder(EventTypeAuthSuccess)
}

// AuthFailure creates an auth failure event builder
func AuthFailure() *EventBuilder {
	return NewEventBuilder(EventTypeAuthFailure)
}

// MessageReceived creates a message received event builder
func MessageReceived() *EventBuilder {
	return NewEventBuilder(EventTypeMessageReceived)
}

// MessageSent creates a message sent event builder
func MessageSent() *EventBuilder {
	return NewEventBuilder(EventTypeMessageSent)
}

// PlayerJoin creates a player join event builder
func PlayerJoin() *EventBuilder {
	return NewEventBuilder(EventTypePlayerJoin)
}

// PlayerLeave creates a player leave event builder
func PlayerLeave() *EventBuilder {
	return NewEventBuilder(EventTypePlayerLeave)
}

// RoomJoin creates a room join event builder
func RoomJoin() *EventBuilder {
	return NewEventBuilder(EventTypeRoomJoin)
}

// RoomLeave creates a room leave event builder
func RoomLeave() *EventBuilder {
	return NewEventBuilder(EventTypeRoomLeave)
}