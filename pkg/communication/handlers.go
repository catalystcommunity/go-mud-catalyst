package communication

import (
	"fmt"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/fxamacker/cbor/v2"
)

// handleSay handles "say" messages - speak to players in the same room
func (cm *CommunicationManager) handleSay(msg *message.Message, senderID string) error {
	var commData CommunicationData
	if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
		return fmt.Errorf("failed to unmarshal say message: %w", err)
	}
	
	// Get sender's location
	location, exists := cm.GetPlayerLocation(senderID)
	if !exists {
		return fmt.Errorf("sender %s location not found", senderID)
	}
	
	// Set location info in the message
	commData.WorldID = location.WorldID
	commData.RoomID = location.RoomID
	commData.SenderID = senderID
	commData.Timestamp = time.Now().Unix()
	
	// Create new message with updated data
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated say message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     MessageTypeSay,
		Contents: updatedContents,
	}
	
	// Send to all players in the room
	return cm.SendToRoom(location.WorldID, location.RoomID, updatedMsg)
}

// handleShout handles "shout" messages - speak to all players in the world
func (cm *CommunicationManager) handleShout(msg *message.Message, senderID string) error {
	var commData CommunicationData
	if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
		return fmt.Errorf("failed to unmarshal shout message: %w", err)
	}
	
	// Get sender's location
	location, exists := cm.GetPlayerLocation(senderID)
	if !exists {
		return fmt.Errorf("sender %s location not found", senderID)
	}
	
	// Set location info in the message
	commData.WorldID = location.WorldID
	commData.RoomID = location.RoomID
	commData.SenderID = senderID
	commData.Timestamp = time.Now().Unix()
	
	// Create new message with updated data
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated shout message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     MessageTypeShout,
		Contents: updatedContents,
	}
	
	// Send to all players in the world
	return cm.SendToWorld(location.WorldID, updatedMsg)
}

// handleEmote handles "emote" messages - action messages to players in the same room
func (cm *CommunicationManager) handleEmote(msg *message.Message, senderID string) error {
	var commData CommunicationData
	if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
		return fmt.Errorf("failed to unmarshal emote message: %w", err)
	}
	
	// Get sender's location
	location, exists := cm.GetPlayerLocation(senderID)
	if !exists {
		return fmt.Errorf("sender %s location not found", senderID)
	}
	
	// Set location info in the message
	commData.WorldID = location.WorldID
	commData.RoomID = location.RoomID
	commData.SenderID = senderID
	commData.Timestamp = time.Now().Unix()
	
	// Create new message with updated data
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated emote message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     MessageTypeEmote,
		Contents: updatedContents,
	}
	
	// Send to all players in the room
	return cm.SendToRoom(location.WorldID, location.RoomID, updatedMsg)
}

// handleTell handles "tell" messages - private messages between players
func (cm *CommunicationManager) handleTell(msg *message.Message, senderID string) error {
	var commData CommunicationData
	if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
		return fmt.Errorf("failed to unmarshal tell message: %w", err)
	}
	
	if commData.TargetID == "" {
		return fmt.Errorf("tell message missing target player ID")
	}
	
	// Get sender's location for context
	location, exists := cm.GetPlayerLocation(senderID)
	if exists {
		commData.WorldID = location.WorldID
		commData.RoomID = location.RoomID
	}
	
	commData.SenderID = senderID
	commData.Timestamp = time.Now().Unix()
	
	// Create new message with updated data
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated tell message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     MessageTypeTell,
		Contents: updatedContents,
	}
	
	// Send to target player
	return cm.SendToPlayer(commData.TargetID, updatedMsg)
}

// handleChat handles standard chat messages and routes them through the world system
func (cm *CommunicationManager) handleChat(msg *message.Message, senderID string) error {
	var chatData message.ChatData
	if err := cbor.Unmarshal(msg.Contents, &chatData); err != nil {
		return fmt.Errorf("failed to unmarshal chat message: %w", err)
	}
	
	// Convert to communication data
	commData := CommunicationData{
		Message:    chatData.Message,
		SenderID:   senderID,
		SenderName: chatData.Username,
		Timestamp:  time.Now().Unix(),
	}
	
	// If channel is specified, send to channel
	if chatData.Channel != "" {
		// Set channel info
		commData.RoomID = chatData.Channel // Use channel as room ID for compatibility
		
		// Create new message
		updatedContents, err := cbor.Marshal(commData)
		if err != nil {
			return fmt.Errorf("failed to marshal updated chat message: %w", err)
		}
		
		updatedMsg := &message.Message{
			Type:     message.MessageTypeChat,
			Contents: updatedContents,
		}
		
		return cm.SendToChannel(chatData.Channel, updatedMsg)
	}
	
	// Otherwise, send to current room
	location, exists := cm.GetPlayerLocation(senderID)
	if !exists {
		return fmt.Errorf("sender %s location not found", senderID)
	}
	
	commData.WorldID = location.WorldID
	commData.RoomID = location.RoomID
	
	// Create new message
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated chat message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     message.MessageTypeChat,
		Contents: updatedContents,
	}
	
	return cm.SendToRoom(location.WorldID, location.RoomID, updatedMsg)
}

// handleWhisper handles whisper messages as private tells
func (cm *CommunicationManager) handleWhisper(msg *message.Message, senderID string) error {
	var chatData message.ChatData
	if err := cbor.Unmarshal(msg.Contents, &chatData); err != nil {
		return fmt.Errorf("failed to unmarshal whisper message: %w", err)
	}
	
	if chatData.Target == "" {
		return fmt.Errorf("whisper message missing target")
	}
	
	// Convert to communication data
	commData := CommunicationData{
		Message:    chatData.Message,
		SenderID:   senderID,
		SenderName: chatData.Username,
		TargetName: chatData.Target,
		Timestamp:  time.Now().Unix(),
	}
	
	// Get sender's location for context
	location, exists := cm.GetPlayerLocation(senderID)
	if exists {
		commData.WorldID = location.WorldID
		commData.RoomID = location.RoomID
	}
	
	// Create new message
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated whisper message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     message.MessageTypeWhisper,
		Contents: updatedContents,
	}
	
	// For now, we'll need to resolve the target name to an ID
	// In a full implementation, this would query the player storage
	// For testing, we'll trigger an event to let the system handle resolution
	event := cm.eventManager.CreateEvent(events.EventTypeCustomPrefix+"resolve_whisper_target", cm, map[string]interface{}{
		"action":      "resolve_whisper_target",
		"target_name": chatData.Target,
		"message":     updatedMsg,
		"sender_id":   senderID,
	})
	
	cm.eventManager.TriggerEvent(event)
	
	return nil
}

// handleBroadcast handles broadcast messages - send to all connected players
func (cm *CommunicationManager) handleBroadcast(msg *message.Message, senderID string) error {
	var chatData message.ChatData
	if err := cbor.Unmarshal(msg.Contents, &chatData); err != nil {
		return fmt.Errorf("failed to unmarshal broadcast message: %w", err)
	}
	
	// Convert to communication data
	commData := CommunicationData{
		Message:    chatData.Message,
		SenderID:   senderID,
		SenderName: chatData.Username,
		Timestamp:  time.Now().Unix(),
	}
	
	// Create new message
	updatedContents, err := cbor.Marshal(commData)
	if err != nil {
		return fmt.Errorf("failed to marshal updated broadcast message: %w", err)
	}
	
	updatedMsg := &message.Message{
		Type:     message.MessageTypeBroadcast,
		Contents: updatedContents,
	}
	
	// Trigger event for global broadcast
	event := cm.eventManager.CreateEvent(events.EventTypeMessageSent, cm, map[string]interface{}{
		"message_type": "global_broadcast",
		"message":      updatedMsg,
		"sender_id":    senderID,
	})
	
	cm.eventManager.TriggerEvent(event)
	
	return nil
}

// FilterMessage filters messages based on player permissions or settings
func (cm *CommunicationManager) FilterMessage(msg *message.Message, recipientID string) (*message.Message, error) {
	// This is a placeholder for message filtering logic
	// In a full implementation, this would check:
	// - Player ignore lists
	// - Channel permissions
	// - Mute status
	// - Content filters
	
	// For now, just return the message unchanged
	return msg, nil
}

// GetMessageHistory returns recent messages for a specific context
func (cm *CommunicationManager) GetMessageHistory(contextType, contextID string, limit int) ([]*message.Message, error) {
	// This is a placeholder for message history functionality
	// In a full implementation, this would:
	// - Query stored messages from persistent storage
	// - Filter by context (room, world, channel, private)
	// - Return limited results
	
	return nil, fmt.Errorf("message history not implemented")
}

// ValidateMessage validates a message before processing
func (cm *CommunicationManager) ValidateMessage(msg *message.Message, senderID string) error {
	// Basic validation
	if msg == nil {
		return fmt.Errorf("message is nil")
	}
	
	if senderID == "" {
		return fmt.Errorf("sender ID is empty")
	}
	
	if len(msg.Contents) == 0 {
		return fmt.Errorf("message contents are empty")
	}
	
	// Message type specific validation
	switch msg.Type {
	case MessageTypeSay, MessageTypeShout, MessageTypeEmote:
		var commData CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
			return fmt.Errorf("invalid communication data: %w", err)
		}
		
		if commData.Message == "" {
			return fmt.Errorf("message text is empty")
		}
		
	case MessageTypeTell:
		var commData CommunicationData
		if err := cbor.Unmarshal(msg.Contents, &commData); err != nil {
			return fmt.Errorf("invalid communication data: %w", err)
		}
		
		if commData.Message == "" {
			return fmt.Errorf("message text is empty")
		}
		
		if commData.TargetID == "" && commData.TargetName == "" {
			return fmt.Errorf("tell message missing target")
		}
		
	case message.MessageTypeChat, message.MessageTypeWhisper, message.MessageTypeBroadcast:
		var chatData message.ChatData
		if err := cbor.Unmarshal(msg.Contents, &chatData); err != nil {
			return fmt.Errorf("invalid chat data: %w", err)
		}
		
		if chatData.Message == "" {
			return fmt.Errorf("chat message text is empty")
		}
	}
	
	return nil
}