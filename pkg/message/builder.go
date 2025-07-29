package message

import (
	"time"
	
	"github.com/fxamacker/cbor/v2"
)

// MessageBuilder provides a fluent API for building messages
type MessageBuilder struct {
	message *Message
}

// NewMessage creates a new message builder
func NewMessage(messageType int64) *MessageBuilder {
	msg := &Message{
		Type:        messageType,
		RequiresAck: RequiresAcknowledgment(messageType),
		AckTimeout:  DefaultAckTimeout,
	}
	
	// Generate ID if acknowledgment is required
	if msg.RequiresAck {
		msg.ID = GenerateMessageID()
	}
	
	return &MessageBuilder{message: msg}
}

// WithContent sets the message content (automatically CBOR encoded)
func (b *MessageBuilder) WithContent(data interface{}) *MessageBuilder {
	if content, err := em.Marshal(data); err == nil {
		b.message.Contents = content
	}
	return b
}

// WithRawContent sets raw byte content without encoding
func (b *MessageBuilder) WithRawContent(data []byte) *MessageBuilder {
	b.message.Contents = data
	return b
}

// WithStringContent sets string content as bytes
func (b *MessageBuilder) WithStringContent(text string) *MessageBuilder {
	b.message.Contents = []byte(text)
	return b
}

// WithID sets a custom message ID
func (b *MessageBuilder) WithID(id string) *MessageBuilder {
	b.message.ID = id
	return b
}

// RequireAck enables acknowledgment requirement
func (b *MessageBuilder) RequireAck(timeout time.Duration) *MessageBuilder {
	b.message.RequiresAck = true
	b.message.AckTimeout = timeout
	if b.message.ID == "" {
		b.message.ID = GenerateMessageID()
	}
	return b
}

// NoAck disables acknowledgment requirement
func (b *MessageBuilder) NoAck() *MessageBuilder {
	b.message.RequiresAck = false
	b.message.AckTimeout = 0
	b.message.ID = ""
	return b
}

// WithTimeout sets acknowledgment timeout
func (b *MessageBuilder) WithTimeout(timeout time.Duration) *MessageBuilder {
	b.message.AckTimeout = timeout
	return b
}

// Build returns the constructed message
func (b *MessageBuilder) Build() *Message {
	return b.message
}

// Common data structures for message content
type AuthData struct {
	Username string `cbor:"username"`
	Password string `cbor:"password,omitempty"`
	Token    string `cbor:"token,omitempty"`
	Method   string `cbor:"method"` // "password", "token", "guest"
}

type ChatData struct {
	Message  string `cbor:"message"`
	Channel  string `cbor:"channel,omitempty"`
	Target   string `cbor:"target,omitempty"` // For whispers
	Username string `cbor:"username,omitempty"`
}

type RoomData struct {
	RoomID   string `cbor:"room_id"`
	RoomName string `cbor:"room_name,omitempty"`
	Password string `cbor:"password,omitempty"`
}

type MoveData struct {
	Direction string `cbor:"direction"` // "north", "south", "east", "west", "up", "down"
	RoomID    string `cbor:"room_id,omitempty"`
	X         int    `cbor:"x,omitempty"`
	Y         int    `cbor:"y,omitempty"`
	Z         int    `cbor:"z,omitempty"`
}

type ActionData struct {
	Action string                 `cbor:"action"`
	Target string                 `cbor:"target,omitempty"`
	Args   map[string]interface{} `cbor:"args,omitempty"`
}

type StatusData struct {
	Health    int                    `cbor:"health,omitempty"`
	Mana      int                    `cbor:"mana,omitempty"`
	Level     int                    `cbor:"level,omitempty"`
	Location  string                 `cbor:"location,omitempty"`
	Status    string                 `cbor:"status,omitempty"` // "idle", "fighting", "moving"
	Inventory []string               `cbor:"inventory,omitempty"`
	Stats     map[string]interface{} `cbor:"stats,omitempty"`
}

// Convenience builders for common message types

// Auth creates an authentication message builder
func Auth() *AuthBuilder {
	return &AuthBuilder{NewMessage(MessageTypeAuth)}
}

// Chat creates a chat message builder  
func Chat() *ChatBuilder {
	return &ChatBuilder{NewMessage(MessageTypeChat)}
}

// Whisper creates a whisper message builder
func Whisper() *ChatBuilder {
	return &ChatBuilder{NewMessage(MessageTypeWhisper)}
}

// Broadcast creates a broadcast message builder
func Broadcast() *ChatBuilder {
	return &ChatBuilder{NewMessage(MessageTypeBroadcast)}
}

// JoinRoom creates a join room message builder
func JoinRoom() *RoomBuilder {
	return &RoomBuilder{NewMessage(MessageTypeJoinRoom)}
}

// LeaveRoom creates a leave room message builder
func LeaveRoom() *RoomBuilder {
	return &RoomBuilder{NewMessage(MessageTypeLeaveRoom)}
}

// Move creates a movement message builder
func Move() *MoveBuilder {
	return &MoveBuilder{NewMessage(MessageTypeMove)}
}

// Action creates a game action message builder
func Action() *ActionBuilder {
	return &ActionBuilder{NewMessage(MessageTypeAction)}
}

// Status creates a status message builder
func Status() *StatusBuilder {
	return &StatusBuilder{NewMessage(MessageTypeStatus)}
}

// Heartbeat creates a heartbeat message
func Heartbeat() *Message {
	return NewMessage(MessageTypeHeartbeat).
		WithStringContent("ping").
		NoAck().
		Build()
}

// Error creates an error message
func Error(errorMsg string) *Message {
	return NewMessage(MessageTypeError).
		WithStringContent(errorMsg).
		NoAck().
		Build()
}

// AuthBuilder provides specialized building for authentication messages
type AuthBuilder struct {
	*MessageBuilder
}

// WithCredentials sets username/password authentication
func (b *AuthBuilder) WithCredentials(username, password string) *AuthBuilder {
	data := AuthData{
		Username: username,
		Password: password,
		Method:   "password",
	}
	b.WithContent(data)
	return b
}

// WithToken sets token-based authentication
func (b *AuthBuilder) WithToken(username, token string) *AuthBuilder {
	data := AuthData{
		Username: username,
		Token:    token,
		Method:   "token",
	}
	b.WithContent(data)
	return b
}

// AsGuest sets guest authentication
func (b *AuthBuilder) AsGuest(username string) *AuthBuilder {
	data := AuthData{
		Username: username,
		Method:   "guest",
	}
	b.WithContent(data)
	return b
}

// ChatBuilder provides specialized building for chat messages
type ChatBuilder struct {
	*MessageBuilder
}

// WithText sets the chat message text
func (b *ChatBuilder) WithText(text string) *ChatBuilder {
	data := ChatData{Message: text}
	b.WithContent(data)
	return b
}

// InChannel sets the channel for the message
func (b *ChatBuilder) InChannel(channel string) *ChatBuilder {
	// Get existing data or create new
	var data ChatData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Channel = channel
	b.WithContent(data)
	return b
}

// ToUser sets the target user for whispers
func (b *ChatBuilder) ToUser(username string) *ChatBuilder {
	var data ChatData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Target = username
	b.WithContent(data)
	return b
}

// FromUser sets the sender username
func (b *ChatBuilder) FromUser(username string) *ChatBuilder {
	var data ChatData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Username = username
	b.WithContent(data)
	return b
}

// RoomBuilder provides specialized building for room messages
type RoomBuilder struct {
	*MessageBuilder
}

// Room sets the room ID and optional name
func (b *RoomBuilder) Room(roomID string, roomName ...string) *RoomBuilder {
	data := RoomData{RoomID: roomID}
	if len(roomName) > 0 {
		data.RoomName = roomName[0]
	}
	b.WithContent(data)
	return b
}

// WithPassword sets room password
func (b *RoomBuilder) WithPassword(password string) *RoomBuilder {
	var data RoomData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Password = password
	b.WithContent(data)
	return b
}

// MoveBuilder provides specialized building for movement messages
type MoveBuilder struct {
	*MessageBuilder
}

// InDirection sets directional movement
func (b *MoveBuilder) InDirection(direction string) *MoveBuilder {
	data := MoveData{Direction: direction}
	b.WithContent(data)
	return b
}

// ToRoom sets room-based movement
func (b *MoveBuilder) ToRoom(roomID string) *MoveBuilder {
	var data MoveData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.RoomID = roomID
	b.WithContent(data)
	return b
}

// ToCoordinates sets coordinate-based movement
func (b *MoveBuilder) ToCoordinates(x, y, z int) *MoveBuilder {
	var data MoveData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.X, data.Y, data.Z = x, y, z
	b.WithContent(data)
	return b
}

// ActionBuilder provides specialized building for game action messages
type ActionBuilder struct {
	*MessageBuilder
}

// Do sets the action to perform
func (b *ActionBuilder) Do(action string) *ActionBuilder {
	data := ActionData{Action: action}
	b.WithContent(data)
	return b
}

// OnTarget sets the target of the action
func (b *ActionBuilder) OnTarget(target string) *ActionBuilder {
	var data ActionData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Target = target
	b.WithContent(data)
	return b
}

// WithArgs sets action arguments
func (b *ActionBuilder) WithArgs(args map[string]interface{}) *ActionBuilder {
	var data ActionData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Args = args
	b.WithContent(data)
	return b
}

// StatusBuilder provides specialized building for status messages
type StatusBuilder struct {
	*MessageBuilder
}

// WithHealth sets health value
func (b *StatusBuilder) WithHealth(health int) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Health = health
	b.WithContent(data)
	return b
}

// WithMana sets mana value
func (b *StatusBuilder) WithMana(mana int) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Mana = mana
	b.WithContent(data)
	return b
}

// WithLevel sets level value
func (b *StatusBuilder) WithLevel(level int) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Level = level
	b.WithContent(data)
	return b
}

// AtLocation sets location
func (b *StatusBuilder) AtLocation(location string) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Location = location
	b.WithContent(data)
	return b
}

// WithStatus sets status string
func (b *StatusBuilder) WithStatus(status string) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Status = status
	b.WithContent(data)
	return b
}

// WithInventory sets inventory items
func (b *StatusBuilder) WithInventory(items []string) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Inventory = items
	b.WithContent(data)
	return b
}

// WithStats sets additional stats
func (b *StatusBuilder) WithStats(stats map[string]interface{}) *StatusBuilder {
	var data StatusData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Stats = stats
	b.WithContent(data)
	return b
}