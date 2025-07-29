package communication

import (
	"time"

	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/fxamacker/cbor/v2"
)

// CommunicationBuilder provides a fluent API for building communication messages
type CommunicationBuilder struct {
	message *message.Message
}

// NewCommunicationMessage creates a new communication message builder
func NewCommunicationMessage(messageType int64) *CommunicationBuilder {
	msg := &message.Message{
		Type:        messageType,
		RequiresAck: false, // Communication messages typically don't require acks
		AckTimeout:  0,
	}
	
	return &CommunicationBuilder{message: msg}
}

// WithText sets the message text
func (b *CommunicationBuilder) WithText(text string) *CommunicationBuilder {
	var data CommunicationData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Message = text
	b.updateContent(data)
	return b
}

// FromPlayer sets the sender information
func (b *CommunicationBuilder) FromPlayer(playerID, playerName string) *CommunicationBuilder {
	var data CommunicationData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.SenderID = playerID
	data.SenderName = playerName
	b.updateContent(data)
	return b
}

// ToPlayer sets the target player for tells
func (b *CommunicationBuilder) ToPlayer(playerID, playerName string) *CommunicationBuilder {
	var data CommunicationData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.TargetID = playerID
	data.TargetName = playerName
	b.updateContent(data)
	return b
}

// InRoom sets the room context
func (b *CommunicationBuilder) InRoom(worldID, roomID string) *CommunicationBuilder {
	var data CommunicationData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.WorldID = worldID
	data.RoomID = roomID
	b.updateContent(data)
	return b
}

// WithTimestamp sets the timestamp
func (b *CommunicationBuilder) WithTimestamp(timestamp time.Time) *CommunicationBuilder {
	var data CommunicationData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	data.Timestamp = timestamp.Unix()
	b.updateContent(data)
	return b
}

// RequireAck enables acknowledgment requirement
func (b *CommunicationBuilder) RequireAck(timeout time.Duration) *CommunicationBuilder {
	b.message.RequiresAck = true
	b.message.AckTimeout = timeout
	if b.message.ID == "" {
		b.message.ID = message.GenerateMessageID()
	}
	return b
}

// Build returns the constructed message
func (b *CommunicationBuilder) Build() *message.Message {
	// Ensure timestamp is set
	var data CommunicationData
	if len(b.message.Contents) > 0 {
		cbor.Unmarshal(b.message.Contents, &data)
	}
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().Unix()
	}
	b.updateContent(data)
	
	return b.message
}

// updateContent updates the message content with the communication data
func (b *CommunicationBuilder) updateContent(data CommunicationData) {
	if content, err := cbor.Marshal(data); err == nil {
		b.message.Contents = content
	}
}

// Convenience builders for common communication types

// Say creates a "say" message builder
func Say() *CommunicationBuilder {
	return NewCommunicationMessage(MessageTypeSay)
}

// Shout creates a "shout" message builder
func Shout() *CommunicationBuilder {
	return NewCommunicationMessage(MessageTypeShout)
}

// Emote creates an "emote" message builder
func Emote() *CommunicationBuilder {
	return NewCommunicationMessage(MessageTypeEmote)
}

// Tell creates a "tell" message builder
func Tell() *CommunicationBuilder {
	return NewCommunicationMessage(MessageTypeTell)
}

// Example usage functions for documentation/testing

// CreateSayMessage creates a say message
func CreateSayMessage(playerID, playerName, text string) *message.Message {
	return Say().
		WithText(text).
		FromPlayer(playerID, playerName).
		Build()
}

// CreateShoutMessage creates a shout message
func CreateShoutMessage(playerID, playerName, text string) *message.Message {
	return Shout().
		WithText(text).
		FromPlayer(playerID, playerName).
		Build()
}

// CreateEmoteMessage creates an emote message
func CreateEmoteMessage(playerID, playerName, action string) *message.Message {
	return Emote().
		WithText(action).
		FromPlayer(playerID, playerName).
		Build()
}

// CreateTellMessage creates a tell message
func CreateTellMessage(senderID, senderName, targetID, targetName, text string) *message.Message {
	return Tell().
		WithText(text).
		FromPlayer(senderID, senderName).
		ToPlayer(targetID, targetName).
		Build()
}

// ParseCommunicationMessage parses a communication message
func ParseCommunicationMessage(msg *message.Message) (*CommunicationData, error) {
	var data CommunicationData
	err := cbor.Unmarshal(msg.Contents, &data)
	return &data, err
}

// FormatCommunicationMessage formats a communication message for display
func FormatCommunicationMessage(msg *message.Message) (string, error) {
	data, err := ParseCommunicationMessage(msg)
	if err != nil {
		return "", err
	}
	
	switch msg.Type {
	case MessageTypeSay:
		return formatSayMessage(data), nil
	case MessageTypeShout:
		return formatShoutMessage(data), nil
	case MessageTypeEmote:
		return formatEmoteMessage(data), nil
	case MessageTypeTell:
		return formatTellMessage(data), nil
	default:
		return data.Message, nil
	}
}

// formatSayMessage formats a say message for display
func formatSayMessage(data *CommunicationData) string {
	if data.SenderName != "" {
		return data.SenderName + " says: " + data.Message
	}
	return data.SenderID + " says: " + data.Message
}

// formatShoutMessage formats a shout message for display
func formatShoutMessage(data *CommunicationData) string {
	if data.SenderName != "" {
		return data.SenderName + " shouts: " + data.Message
	}
	return data.SenderID + " shouts: " + data.Message
}

// formatEmoteMessage formats an emote message for display
func formatEmoteMessage(data *CommunicationData) string {
	if data.SenderName != "" {
		return data.SenderName + " " + data.Message
	}
	return data.SenderID + " " + data.Message
}

// formatTellMessage formats a tell message for display
func formatTellMessage(data *CommunicationData) string {
	if data.SenderName != "" {
		return data.SenderName + " tells you: " + data.Message
	}
	return data.SenderID + " tells you: " + data.Message
}