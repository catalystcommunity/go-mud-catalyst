package message_test


import (
	"github.com/catalystcommunity/muddycore/pkg/message"
)
import (
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
)

func TestBasicMessageBuilder(t *testing.T) {
	// Test basic message building
	msg := message.NewMessage(message.MessageTypeChat).
		WithStringContent("Hello, World!").
		Build()

	if msg.Type != message.MessageTypeChat {
		t.Errorf("Expected MessageTypeChat, got %d", msg.Type)
	}

	if string(msg.Contents) != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %s", string(msg.Contents))
	}

	// Chat messages shouldn't require ack by default
	if msg.RequiresAck {
		t.Error("Chat messages should not require ack by default")
	}
}

func TestMessageBuilderWithAck(t *testing.T) {
	timeout := 15 * time.Second
	msg := message.NewMessage(message.MessageTypeAuth).
		WithStringContent("auth-data").
		RequireAck(timeout).
		Build()

	if !msg.RequiresAck {
		t.Error("Message should require acknowledgment")
	}

	if msg.AckTimeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, msg.AckTimeout)
	}

	if msg.ID == "" {
		t.Error("Message requiring ack should have ID")
	}
}

func TestMessageBuilderNoAck(t *testing.T) {
	msg := message.NewMessage(message.MessageTypeAuth). // Auth normally requires ack
						WithStringContent("auth-data").
						NoAck().
						Build()

	if msg.RequiresAck {
		t.Error("Message should not require acknowledgment")
	}

	if msg.ID != "" {
		t.Error("Message not requiring ack should not have ID")
	}
}

func TestAuthBuilder(t *testing.T) {
	// Test password authentication
	msg := message.Auth().
		WithCredentials("testuser", "testpass").
		Build()

	if msg.Type != message.MessageTypeAuth {
		t.Errorf("Expected MessageTypeAuth, got %d", msg.Type)
	}

	var authData message.AuthData
	err := cbor.Unmarshal(msg.Contents, &authData)
	if err != nil {
		t.Fatalf("Failed to unmarshal auth data: %v", err)
	}

	if authData.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", authData.Username)
	}

	if authData.Password != "testpass" {
		t.Errorf("Expected password 'testpass', got %s", authData.Password)
	}

	if authData.Method != "password" {
		t.Errorf("Expected method 'password', got %s", authData.Method)
	}

	// Test token authentication
	tokenMsg := message.Auth().
		WithToken("tokenuser", "abc123").
		Build()

	var tokenData message.AuthData
	err = cbor.Unmarshal(tokenMsg.Contents, &tokenData)
	if err != nil {
		t.Fatalf("Failed to unmarshal token auth data: %v", err)
	}

	if tokenData.Method != "token" {
		t.Errorf("Expected method 'token', got %s", tokenData.Method)
	}

	if tokenData.Token != "abc123" {
		t.Errorf("Expected token 'abc123', got %s", tokenData.Token)
	}

	// Test guest authentication
	guestMsg := message.Auth().
		AsGuest("guestuser").
		Build()

	var guestData message.AuthData
	err = cbor.Unmarshal(guestMsg.Contents, &guestData)
	if err != nil {
		t.Fatalf("Failed to unmarshal guest auth data: %v", err)
	}

	if guestData.Method != "guest" {
		t.Errorf("Expected method 'guest', got %s", guestData.Method)
	}

	if guestData.Username != "guestuser" {
		t.Errorf("Expected username 'guestuser', got %s", guestData.Username)
	}
}

func TestChatBuilder(t *testing.T) {
	// Test basic chat message
	msg := message.Chat().
		WithText("Hello everyone!").
		Build()

	if msg.Type != message.MessageTypeChat {
		t.Errorf("Expected MessageTypeChat, got %d", msg.Type)
	}

	var chatData message.ChatData
	err := cbor.Unmarshal(msg.Contents, &chatData)
	if err != nil {
		t.Fatalf("Failed to unmarshal chat data: %v", err)
	}

	if chatData.Message != "Hello everyone!" {
		t.Errorf("Expected message 'Hello everyone!', got %s", chatData.Message)
	}

	// Test chat with channel
	channelMsg := message.Chat().
		WithText("Channel message").
		InChannel("general").
		FromUser("testuser").
		Build()

	var channelData message.ChatData
	err = cbor.Unmarshal(channelMsg.Contents, &channelData)
	if err != nil {
		t.Fatalf("Failed to unmarshal channel chat data: %v", err)
	}

	if channelData.Channel != "general" {
		t.Errorf("Expected channel 'general', got %s", channelData.Channel)
	}

	if channelData.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", channelData.Username)
	}

	// Test whisper
	whisperMsg := message.Whisper().
		WithText("Secret message").
		ToUser("targetuser").
		FromUser("sender").
		Build()

	if whisperMsg.Type != message.MessageTypeWhisper {
		t.Errorf("Expected MessageTypeWhisper, got %d", whisperMsg.Type)
	}

	var whisperData message.ChatData
	err = cbor.Unmarshal(whisperMsg.Contents, &whisperData)
	if err != nil {
		t.Fatalf("Failed to unmarshal whisper data: %v", err)
	}

	if whisperData.Target != "targetuser" {
		t.Errorf("Expected target 'targetuser', got %s", whisperData.Target)
	}
}

func TestRoomBuilder(t *testing.T) {
	// Test join room
	msg := message.JoinRoom().
		Room("lobby", "Main Lobby").
		Build()

	if msg.Type != message.MessageTypeJoinRoom {
		t.Errorf("Expected MessageTypeJoinRoom, got %d", msg.Type)
	}

	var roomData message.RoomData
	err := cbor.Unmarshal(msg.Contents, &roomData)
	if err != nil {
		t.Fatalf("Failed to unmarshal room data: %v", err)
	}

	if roomData.RoomID != "lobby" {
		t.Errorf("Expected room ID 'lobby', got %s", roomData.RoomID)
	}

	if roomData.RoomName != "Main Lobby" {
		t.Errorf("Expected room name 'Main Lobby', got %s", roomData.RoomName)
	}

	// Test room with password
	passwordMsg := message.JoinRoom().
		Room("private").
		WithPassword("secret123").
		Build()

	var passwordData message.RoomData
	err = cbor.Unmarshal(passwordMsg.Contents, &passwordData)
	if err != nil {
		t.Fatalf("Failed to unmarshal password room data: %v", err)
	}

	if passwordData.Password != "secret123" {
		t.Errorf("Expected password 'secret123', got %s", passwordData.Password)
	}
}

func TestMoveBuilder(t *testing.T) {
	// Test directional movement
	msg := message.Move().
		InDirection("north").
		Build()

	if msg.Type != message.MessageTypeMove {
		t.Errorf("Expected MessageTypeMove, got %d", msg.Type)
	}

	var moveData message.MoveData
	err := cbor.Unmarshal(msg.Contents, &moveData)
	if err != nil {
		t.Fatalf("Failed to unmarshal move data: %v", err)
	}

	if moveData.Direction != "north" {
		t.Errorf("Expected direction 'north', got %s", moveData.Direction)
	}

	// Test coordinate movement
	coordMsg := message.Move().
		ToCoordinates(10, 20, 5).
		Build()

	var coordData message.MoveData
	err = cbor.Unmarshal(coordMsg.Contents, &coordData)
	if err != nil {
		t.Fatalf("Failed to unmarshal coordinate move data: %v", err)
	}

	if coordData.X != 10 || coordData.Y != 20 || coordData.Z != 5 {
		t.Errorf("Expected coordinates (10,20,5), got (%d,%d,%d)", coordData.X, coordData.Y, coordData.Z)
	}

	// Test room movement
	roomMsg := message.Move().
		InDirection("portal").
		ToRoom("dungeon_entrance").
		Build()

	var roomData message.MoveData
	err = cbor.Unmarshal(roomMsg.Contents, &roomData)
	if err != nil {
		t.Fatalf("Failed to unmarshal room move data: %v", err)
	}

	if roomData.RoomID != "dungeon_entrance" {
		t.Errorf("Expected room ID 'dungeon_entrance', got %s", roomData.RoomID)
	}
}

func TestActionBuilder(t *testing.T) {
	// Test basic action
	msg := message.Action().
		Do("attack").
		OnTarget("goblin").
		Build()

	if msg.Type != message.MessageTypeAction {
		t.Errorf("Expected MessageTypeAction, got %d", msg.Type)
	}

	var actionData message.ActionData
	err := cbor.Unmarshal(msg.Contents, &actionData)
	if err != nil {
		t.Fatalf("Failed to unmarshal action data: %v", err)
	}

	if actionData.Action != "attack" {
		t.Errorf("Expected action 'attack', got %s", actionData.Action)
	}

	if actionData.Target != "goblin" {
		t.Errorf("Expected target 'goblin', got %s", actionData.Target)
	}

	// Test action with arguments
	args := map[string]interface{}{
		"weapon": "sword",
		"power":  50,
	}

	argMsg := message.Action().
		Do("cast").
		OnTarget("enemy").
		WithArgs(args).
		Build()

	var argData message.ActionData
	err = cbor.Unmarshal(argMsg.Contents, &argData)
	if err != nil {
		t.Fatalf("Failed to unmarshal action with args data: %v", err)
	}

	if argData.Args["weapon"] != "sword" {
		t.Errorf("Expected weapon 'sword', got %v", argData.Args["weapon"])
	}

	// CBOR may unmarshal numbers as different types, so check value conversion
	if power, ok := argData.Args["power"].(int); !ok || power != 50 {
		if power64, ok := argData.Args["power"].(int64); !ok || power64 != 50 {
			if powerUint, ok := argData.Args["power"].(uint64); !ok || powerUint != 50 {
				t.Errorf("Expected power 50, got %v (type %T)", argData.Args["power"], argData.Args["power"])
			}
		}
	}
}

func TestStatusBuilder(t *testing.T) {
	// Test comprehensive status
	inventory := []string{"sword", "potion", "key"}
	stats := map[string]interface{}{
		"strength":     15,
		"intelligence": 12,
		"dexterity":    18,
	}

	msg := message.Status().
		WithHealth(100).
		WithMana(50).
		WithLevel(5).
		AtLocation("castle_courtyard").
		WithStatus("idle").
		WithInventory(inventory).
		WithStats(stats).
		Build()

	if msg.Type != message.MessageTypeStatus {
		t.Errorf("Expected MessageTypeStatus, got %d", msg.Type)
	}

	var statusData message.StatusData
	err := cbor.Unmarshal(msg.Contents, &statusData)
	if err != nil {
		t.Fatalf("Failed to unmarshal status data: %v", err)
	}

	if statusData.Health != 100 {
		t.Errorf("Expected health 100, got %d", statusData.Health)
	}

	if statusData.Mana != 50 {
		t.Errorf("Expected mana 50, got %d", statusData.Mana)
	}

	if statusData.Level != 5 {
		t.Errorf("Expected level 5, got %d", statusData.Level)
	}

	if statusData.Location != "castle_courtyard" {
		t.Errorf("Expected location 'castle_courtyard', got %s", statusData.Location)
	}

	if statusData.Status != "idle" {
		t.Errorf("Expected status 'idle', got %s", statusData.Status)
	}

	if len(statusData.Inventory) != 3 {
		t.Errorf("Expected 3 inventory items, got %d", len(statusData.Inventory))
	}

	// CBOR may unmarshal numbers as different types, so check value conversion
	if strength, ok := statusData.Stats["strength"].(int); !ok || strength != 15 {
		if strength64, ok := statusData.Stats["strength"].(int64); !ok || strength64 != 15 {
			if strengthUint, ok := statusData.Stats["strength"].(uint64); !ok || strengthUint != 15 {
				t.Errorf("Expected strength 15, got %v (type %T)", statusData.Stats["strength"], statusData.Stats["strength"])
			}
		}
	}
}

func TestConvenienceMessages(t *testing.T) {
	// Test heartbeat
	heartbeat := message.Heartbeat()
	if heartbeat.Type != message.MessageTypeHeartbeat {
		t.Errorf("Expected MessageTypeHeartbeat, got %d", heartbeat.Type)
	}

	if heartbeat.RequiresAck {
		t.Error("Heartbeat should not require acknowledgment")
	}

	if string(heartbeat.Contents) != "ping" {
		t.Errorf("Expected 'ping', got %s", string(heartbeat.Contents))
	}

	// Test error message
	errorMsg := message.Error("Something went wrong")
	if errorMsg.Type != message.MessageTypeError {
		t.Errorf("Expected MessageTypeError, got %d", errorMsg.Type)
	}

	if errorMsg.RequiresAck {
		t.Error("Error message should not require acknowledgment")
	}

	if string(errorMsg.Contents) != "Something went wrong" {
		t.Errorf("Expected 'Something went wrong', got %s", string(errorMsg.Contents))
	}
}

func TestBuilderChaining(t *testing.T) {
	// Test complex builder chaining
	msg := message.NewMessage(message.MessageTypeCustom).
		WithStringContent("custom data").
		WithID("custom-123").
		RequireAck(10 * time.Second).
		Build()

	if msg.Type != message.MessageTypeCustom {
		t.Errorf("Expected MessageTypeCustom, got %d", msg.Type)
	}

	if msg.ID != "custom-123" {
		t.Errorf("Expected ID 'custom-123', got %s", msg.ID)
	}

	if !msg.RequiresAck {
		t.Error("Message should require acknowledgment")
	}

	if msg.AckTimeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", msg.AckTimeout)
	}
}

func TestBuilderWithCustomContent(t *testing.T) {
	// Test with custom struct
	customData := struct {
		Name  string `cbor:"name"`
		Value int    `cbor:"value"`
	}{
		Name:  "test",
		Value: 42,
	}

	msg := message.NewMessage(message.MessageTypeCustom).
		WithContent(customData).
		Build()

	// Verify we can unmarshal it back
	var decoded struct {
		Name  string `cbor:"name"`
		Value int    `cbor:"value"`
	}

	err := cbor.Unmarshal(msg.Contents, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal custom content: %v", err)
	}

	if decoded.Name != "test" {
		t.Errorf("Expected name 'test', got %s", decoded.Name)
	}

	if decoded.Value != 42 {
		t.Errorf("Expected value 42, got %d", decoded.Value)
	}
}