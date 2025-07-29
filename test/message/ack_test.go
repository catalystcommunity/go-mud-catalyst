package message_test


import (
	"github.com/catalystcommunity/muddycore/pkg/message"
)
import (
	"testing"
	"time"
)

func TestCreateAckMessage(t *testing.T) {
	messageID := "test-message-123"
	
	// Test successful acknowledgment
	ackMsg := message.CreateAckMessage(messageID, true, "")
	if ackMsg == nil {
		t.Fatal("CreateAckMessage returned nil for success ack")
	}
	
	if ackMsg.Type != message.MessageTypeAck {
		t.Errorf("Expected MessageTypeAck, got %d", ackMsg.Type)
	}
	
	if ackMsg.RequiresAck {
		t.Error("Ack messages should not require acknowledgment")
	}
	
	// Parse the ack message
	ackData, err := message.ParseAckMessage(ackMsg)
	if err != nil {
		t.Fatalf("Failed to parse ack message: %v", err)
	}
	
	if ackData.MessageID != messageID {
		t.Errorf("Expected message ID %s, got %s", messageID, ackData.MessageID)
	}
	
	if !ackData.Success {
		t.Error("Expected successful ack")
	}
	
	// Test negative acknowledgment
	errorMsg := "validation failed"
	nackMsg := message.CreateAckMessage(messageID, false, errorMsg)
	if nackMsg == nil {
		t.Fatal("CreateAckMessage returned nil for nack")
	}
	
	if nackMsg.Type != message.MessageTypeNack {
		t.Errorf("Expected MessageTypeNack, got %d", nackMsg.Type)
	}
	
	nackData, err := message.ParseAckMessage(nackMsg)
	if err != nil {
		t.Fatalf("Failed to parse nack message: %v", err)
	}
	
	if nackData.Success {
		t.Error("Expected unsuccessful ack")
	}
	
	if nackData.Error != errorMsg {
		t.Errorf("Expected error message %s, got %s", errorMsg, nackData.Error)
	}
}

func TestRequiresAcknowledgment(t *testing.T) {
	// Messages that should require acknowledgment
	requiresAck := []int64{
		message.MessageTypeAuth,
		message.MessageTypeAuthSuccess,
		message.MessageTypeAuthFailure,
		message.MessageTypeMove,
		message.MessageTypeAction,
		message.MessageTypeInventory,
		message.MessageTypeJoinRoom,
		message.MessageTypeLeaveRoom,
	}
	
	for _, msgType := range requiresAck {
		if !message.RequiresAcknowledgment(msgType) {
			t.Errorf("Message type %d should require acknowledgment", msgType)
		}
	}
	
	// Messages that should not require acknowledgment
	noAck := []int64{
		message.MessageTypeHeartbeat,
		message.MessageTypeChat,
		message.MessageTypeWhisper,
		message.MessageTypeBroadcast,
		message.MessageTypeAck,
		message.MessageTypeNack,
	}
	
	for _, msgType := range noAck {
		if message.RequiresAcknowledgment(msgType) {
			t.Errorf("Message type %d should not require acknowledgment", msgType)
		}
	}
}

func TestAckManager(t *testing.T) {
	am := message.NewAckManager()
	defer am.Close()
	
	// Create a test message
	msg := &message.Message{
		ID:          message.GenerateMessageID(),
		Type:        message.MessageTypeAuth,
		Contents:    []byte("test auth"),
		RequiresAck: true,
		AckTimeout:  time.Second,
	}
	
	// Track the message
	am.TrackMessage(msg)
	
	if am.PendingCount() != 1 {
		t.Errorf("Expected 1 pending message, got %d", am.PendingCount())
	}
	
	// Get the pending message
	retrieved := am.GetPendingMessage(msg.ID)
	if retrieved == nil {
		t.Fatal("Failed to retrieve pending message")
	}
	
	if retrieved.ID != msg.ID {
		t.Errorf("Expected message ID %s, got %s", msg.ID, retrieved.ID)
	}
	
	// Process acknowledgment
	ackData := &message.AckMessage{
		MessageID: msg.ID,
		Success:   true,
		Error:     "",
	}
	
	success := am.ProcessAcknowledgment(ackData)
	if !success {
		t.Error("Expected successful acknowledgment processing")
	}
	
	if am.PendingCount() != 0 {
		t.Errorf("Expected 0 pending messages after ack, got %d", am.PendingCount())
	}
	
	// Try to get the message again (should be nil)
	retrieved = am.GetPendingMessage(msg.ID)
	if retrieved != nil {
		t.Error("Message should be removed after acknowledgment")
	}
}

func TestAckManagerTimeout(t *testing.T) {
	t.Log("NOTE: This test expects WARN/ERROR logs for message timeouts - this is expected behavior")
	
	am := message.NewAckManager()
	defer am.Close()
	
	// Create a test message with short timeout
	msg := &message.Message{
		ID:          message.GenerateMessageID(),
		Type:        message.MessageTypeAuth,
		Contents:    []byte("test auth"),
		RequiresAck: true,
		AckTimeout:  100 * time.Millisecond,
	}
	
	// Track the message
	am.TrackMessage(msg)
	
	// Wait for timeout
	time.Sleep(150 * time.Millisecond)
	
	// Check for timeout messages
	timeouts := am.GetTimeoutMessages()
	if len(timeouts) == 0 {
		t.Error("Expected timeout message")
	}
	
	// The message should still be pending (marked for retry)
	if am.PendingCount() != 1 {
		t.Errorf("Expected 1 pending message after timeout, got %d", am.PendingCount())
	}
	
	// The retry count should be incremented
	pending := am.GetPendingMessage(msg.ID)
	if pending == nil {
		t.Fatal("Message should still be pending")
	}
	
	// Note: The actual retry count increment happens in the timeout handling
	// which is tested by the timeout message retrieval
}

func TestMessageTypeToString(t *testing.T) {
	tests := map[int64]string{
		message.MessageTypeHeartbeat:    "HEARTBEAT",
		message.MessageTypeAuth:         "AUTH",
		message.MessageTypeChat:         "CHAT",
		message.MessageTypeAck:          "ACK",
		message.MessageTypeNack:         "NACK",
		message.MessageTypeCustom:       "CUSTOM_1000",
		message.MessageTypeCustom + 5:   "CUSTOM_1005",
		999:                     "UNKNOWN_999",
	}
	
	for msgType, expected := range tests {
		result := message.MessageTypeToString(msgType)
		if result != expected {
			t.Errorf("MessageTypeToString(%d) = %s, expected %s", msgType, result, expected)
		}
	}
}

func TestGenerateMessageID(t *testing.T) {
	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	
	for i := 0; i < 1000; i++ {
		id := message.GenerateMessageID()
		if id == "" {
			t.Error("GenerateMessageID returned empty string")
		}
		
		if ids[id] {
			t.Errorf("Duplicate message ID generated: %s", id)
		}
		
		ids[id] = true
	}
}

func TestParseAckMessageError(t *testing.T) {
	// Test parsing non-ack message
	msg := &message.Message{
		Type:     message.MessageTypeChat,
		Contents: []byte("hello"),
	}
	
	_, err := message.ParseAckMessage(msg)
	if err == nil {
		t.Error("Expected error when parsing non-ack message")
	}
	
	// Test parsing malformed ack message
	ackMsg := &message.Message{
		Type:     message.MessageTypeAck,
		Contents: []byte("invalid cbor data"),
	}
	
	_, err = message.ParseAckMessage(ackMsg)
	if err == nil {
		t.Error("Expected error when parsing malformed ack message")
	}
}