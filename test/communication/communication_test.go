package communication_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

// TestMain sets up and tears down for all tests in this package
func TestMain(m *testing.M) {
	// Set log level to WARN to reduce noise during tests
	testLogger := logging.NewLogger(logging.LevelWarn, os.Stderr)
	logging.SetDefaultLogger(testLogger)

	// Run tests
	code := m.Run()

	// Restore default logger
	logging.SetDefaultLogger(logging.DefaultLogger())

	os.Exit(code)
}

// TestCommunicationManager_Basic tests basic communication manager functionality
func TestCommunicationManager_Basic(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Test player location management
	t.Run("PlayerLocationTracking", func(t *testing.T) {
		playerID := "player1"
		worldID := "world1"
		roomID := "room1"
		
		// Initially no location
		_, exists := cm.GetPlayerLocation(playerID)
		assert.False(t, exists)
		
		// Update location
		cm.UpdatePlayerLocation(playerID, worldID, roomID)
		
		// Check location
		location, exists := cm.GetPlayerLocation(playerID)
		assert.True(t, exists)
		assert.Equal(t, playerID, location.PlayerID)
		assert.Equal(t, worldID, location.WorldID)
		assert.Equal(t, roomID, location.RoomID)
		
		// Remove location
		cm.RemovePlayerLocation(playerID)
		_, exists = cm.GetPlayerLocation(playerID)
		assert.False(t, exists)
	})
	
	// Test channel management
	t.Run("ChannelManagement", func(t *testing.T) {
		channelID := "test_channel"
		channelName := "Test Channel"
		channelType := "global"
		
		// Create channel
		channel := cm.CreateChannel(channelID, channelName, channelType)
		assert.NotNil(t, channel)
		assert.Equal(t, channelID, channel.ID)
		assert.Equal(t, channelName, channel.Name)
		assert.Equal(t, channelType, channel.Type)
		
		// Get channel
		retrievedChannel, exists := cm.GetChannel(channelID)
		assert.True(t, exists)
		assert.Equal(t, channelID, retrievedChannel.ID)
		
		// Join channel
		playerID := "player1"
		err := cm.JoinChannel(channelID, playerID)
		assert.NoError(t, err)
		
		// Check membership
		members := cm.GetChannelMembers(channelID)
		assert.Contains(t, members, playerID)
		
		// Leave channel
		err = cm.LeaveChannel(channelID, playerID)
		assert.NoError(t, err)
		
		// Check membership removed
		members = cm.GetChannelMembers(channelID)
		assert.NotContains(t, members, playerID)
	})
}

// TestCommunicationMessages tests communication message handling
func TestCommunicationMessages(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Set up test players
	player1ID := "player1"
	player1Name := "Alice"
	player2ID := "player2"
	player2Name := "Bob"
	worldID := "world1"
	roomID := "room1"
	
	cm.UpdatePlayerLocation(player1ID, worldID, roomID)
	cm.UpdatePlayerLocation(player2ID, worldID, roomID)
	
	// Track events
	var receivedEvents []events.Event
	eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
		receivedEvents = append(receivedEvents, event)
		return events.EventResultContinue
	}, 0)
	
	t.Run("SayMessage", func(t *testing.T) {
		// Create say message
		msg := communication.CreateSayMessage(player1ID, player1Name, "Hello everyone!")
		
		// Handle message
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		eventData := event.Data()
		assert.Equal(t, events.EventTypeMessageSent, event.Type())
		assert.Equal(t, "room", eventData["message_type"])
		assert.Equal(t, worldID, eventData["world_id"])
		assert.Equal(t, roomID, eventData["room_id"])
		
		// Check recipients
		recipients, ok := eventData["recipients"].([]string)
		assert.True(t, ok)
		assert.Contains(t, recipients, player1ID)
		assert.Contains(t, recipients, player2ID)
		
		// Clear events
		receivedEvents = nil
	})
	
	t.Run("ShoutMessage", func(t *testing.T) {
		// Create shout message
		msg := communication.CreateShoutMessage(player1ID, player1Name, "Can everyone hear me?")
		
		// Handle message
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		eventData := event.Data()
		assert.Equal(t, events.EventTypeMessageSent, event.Type())
		assert.Equal(t, "world", eventData["message_type"])
		assert.Equal(t, worldID, eventData["world_id"])
		
		// Clear events
		receivedEvents = nil
	})
	
	t.Run("EmoteMessage", func(t *testing.T) {
		// Create emote message
		msg := communication.CreateEmoteMessage(player1ID, player1Name, "waves at everyone")
		
		// Handle message
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		eventData := event.Data()
		assert.Equal(t, events.EventTypeMessageSent, event.Type())
		assert.Equal(t, "room", eventData["message_type"])
		
		// Clear events
		receivedEvents = nil
	})
	
	t.Run("TellMessage", func(t *testing.T) {
		// Create tell message
		msg := communication.CreateTellMessage(player1ID, player1Name, player2ID, player2Name, "Secret message")
		
		// Handle message
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		eventData := event.Data()
		assert.Equal(t, events.EventTypeMessageSent, event.Type())
		assert.Equal(t, "private", eventData["message_type"])
		assert.Equal(t, player2ID, eventData["recipient"])
		
		// Clear events
		receivedEvents = nil
	})
}

// TestMessageBuilders tests the message builder functionality
func TestMessageBuilders(t *testing.T) {
	t.Run("SayBuilder", func(t *testing.T) {
		playerID := "player1"
		playerName := "Alice"
		text := "Hello world!"
		
		msg := communication.Say().
			WithText(text).
			FromPlayer(playerID, playerName).
			Build()
		
		assert.Equal(t, int64(communication.MessageTypeSay), msg.Type)
		assert.False(t, msg.RequiresAck)
		
		// Parse content
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, text, data.Message)
		assert.Equal(t, playerID, data.SenderID)
		assert.Equal(t, playerName, data.SenderName)
		assert.Greater(t, data.Timestamp, int64(0))
	})
	
	t.Run("ShoutBuilder", func(t *testing.T) {
		playerID := "player1"
		playerName := "Alice"
		text := "Can everyone hear me?"
		
		msg := communication.Shout().
			WithText(text).
			FromPlayer(playerID, playerName).
			Build()
		
		assert.Equal(t, int64(communication.MessageTypeShout), msg.Type)
		
		// Parse content
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, text, data.Message)
		assert.Equal(t, playerID, data.SenderID)
		assert.Equal(t, playerName, data.SenderName)
	})
	
	t.Run("EmoteBuilder", func(t *testing.T) {
		playerID := "player1"
		playerName := "Alice"
		action := "waves at everyone"
		
		msg := communication.Emote().
			WithText(action).
			FromPlayer(playerID, playerName).
			Build()
		
		assert.Equal(t, int64(communication.MessageTypeEmote), msg.Type)
		
		// Parse content
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, action, data.Message)
		assert.Equal(t, playerID, data.SenderID)
		assert.Equal(t, playerName, data.SenderName)
	})
	
	t.Run("TellBuilder", func(t *testing.T) {
		senderID := "player1"
		senderName := "Alice"
		targetID := "player2"
		targetName := "Bob"
		text := "Secret message"
		
		msg := communication.Tell().
			WithText(text).
			FromPlayer(senderID, senderName).
			ToPlayer(targetID, targetName).
			Build()
		
		assert.Equal(t, int64(communication.MessageTypeTell), msg.Type)
		
		// Parse content
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, text, data.Message)
		assert.Equal(t, senderID, data.SenderID)
		assert.Equal(t, senderName, data.SenderName)
		assert.Equal(t, targetID, data.TargetID)
		assert.Equal(t, targetName, data.TargetName)
	})
}

// TestMessageFormatting tests message formatting functionality
func TestMessageFormatting(t *testing.T) {
	t.Run("FormatSayMessage", func(t *testing.T) {
		msg := communication.Say().
			WithText("Hello world!").
			FromPlayer("player1", "Alice").
			Build()
		
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice says: Hello world!", formatted)
	})
	
	t.Run("FormatShoutMessage", func(t *testing.T) {
		msg := communication.Shout().
			WithText("Can everyone hear me?").
			FromPlayer("player1", "Alice").
			Build()
		
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice shouts: Can everyone hear me?", formatted)
	})
	
	t.Run("FormatEmoteMessage", func(t *testing.T) {
		msg := communication.Emote().
			WithText("waves at everyone").
			FromPlayer("player1", "Alice").
			Build()
		
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice waves at everyone", formatted)
	})
	
	t.Run("FormatTellMessage", func(t *testing.T) {
		msg := communication.Tell().
			WithText("Secret message").
			FromPlayer("player1", "Alice").
			ToPlayer("player2", "Bob").
			Build()
		
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice tells you: Secret message", formatted)
	})
}

// TestPlayersInRoomAndWorld tests player location queries
func TestPlayersInRoomAndWorld(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Set up test data
	world1 := "world1"
	world2 := "world2"
	room1 := "room1"
	room2 := "room2"
	
	players := []string{"player1", "player2", "player3", "player4"}
	
	// Place players in different locations
	cm.UpdatePlayerLocation(players[0], world1, room1) // player1 in world1/room1
	cm.UpdatePlayerLocation(players[1], world1, room1) // player2 in world1/room1
	cm.UpdatePlayerLocation(players[2], world1, room2) // player3 in world1/room2
	cm.UpdatePlayerLocation(players[3], world2, room1) // player4 in world2/room1
	
	// Test GetPlayersInRoom
	t.Run("GetPlayersInRoom", func(t *testing.T) {
		// Check world1/room1
		playersInRoom := cm.GetPlayersInRoom(world1, room1)
		assert.Len(t, playersInRoom, 2)
		assert.Contains(t, playersInRoom, players[0])
		assert.Contains(t, playersInRoom, players[1])
		
		// Check world1/room2
		playersInRoom = cm.GetPlayersInRoom(world1, room2)
		assert.Len(t, playersInRoom, 1)
		assert.Contains(t, playersInRoom, players[2])
		
		// Check world2/room1
		playersInRoom = cm.GetPlayersInRoom(world2, room1)
		assert.Len(t, playersInRoom, 1)
		assert.Contains(t, playersInRoom, players[3])
		
		// Check empty room
		playersInRoom = cm.GetPlayersInRoom(world1, "empty_room")
		assert.Len(t, playersInRoom, 0)
	})
	
	// Test GetPlayersInWorld
	t.Run("GetPlayersInWorld", func(t *testing.T) {
		// Check world1
		playersInWorld := cm.GetPlayersInWorld(world1)
		assert.Len(t, playersInWorld, 3)
		assert.Contains(t, playersInWorld, players[0])
		assert.Contains(t, playersInWorld, players[1])
		assert.Contains(t, playersInWorld, players[2])
		
		// Check world2
		playersInWorld = cm.GetPlayersInWorld(world2)
		assert.Len(t, playersInWorld, 1)
		assert.Contains(t, playersInWorld, players[3])
		
		// Check empty world
		playersInWorld = cm.GetPlayersInWorld("empty_world")
		assert.Len(t, playersInWorld, 0)
	})
}

// TestLegacyChatIntegration tests integration with existing chat system
func TestLegacyChatIntegration(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Set up test player
	playerID := "player1"
	playerName := "Alice"
	worldID := "world1"
	roomID := "room1"
	
	cm.UpdatePlayerLocation(playerID, worldID, roomID)
	
	// Track events
	var receivedEvents []events.Event
	eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
		receivedEvents = append(receivedEvents, event)
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventTypeCustomPrefix+"resolve_whisper_target", func(event events.Event) events.EventResult {
		receivedEvents = append(receivedEvents, event)
		return events.EventResultContinue
	}, 0)
	
	t.Run("ChatMessage", func(t *testing.T) {
		// Create legacy chat message
		msg := message.Chat().
			WithText("Hello from chat!").
			FromUser(playerName).
			Build()
		
		// Handle message
		err := cm.HandleMessage(msg, playerID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		eventData := event.Data()
		assert.Equal(t, events.EventTypeMessageSent, event.Type())
		assert.Equal(t, "room", eventData["message_type"])
		
		// Clear events
		receivedEvents = nil
	})
	
	t.Run("WhisperMessage", func(t *testing.T) {
		// Create legacy whisper message
		msg := message.Whisper().
			WithText("Secret message").
			FromUser(playerName).
			ToUser("Bob").
			Build()
		
		// Handle message
		err := cm.HandleMessage(msg, playerID)
		assert.NoError(t, err)
		
		// Check event was triggered (whisper resolution)
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		assert.Equal(t, events.EventTypeCustomPrefix+"resolve_whisper_target", event.Type())
		eventData := event.Data()
		assert.Equal(t, "resolve_whisper_target", eventData["action"])
		
		// Clear events
		receivedEvents = nil
	})
	
	t.Run("BroadcastMessage", func(t *testing.T) {
		// Create legacy broadcast message
		msg := message.Broadcast().
			WithText("Server announcement").
			FromUser(playerName).
			Build()
		
		// Handle message
		err := cm.HandleMessage(msg, playerID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Len(t, receivedEvents, 1)
		event := receivedEvents[0]
		assert.Equal(t, events.EventTypeMessageSent, event.Type())
		eventData := event.Data()
		assert.Equal(t, "global_broadcast", eventData["message_type"])
		
		// Clear events
		receivedEvents = nil
	})
}

// TestConcurrentOperations tests thread safety
func TestConcurrentOperations(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Test concurrent location updates
	t.Run("ConcurrentLocationUpdates", func(t *testing.T) {
		const numGoroutines = 100
		const numUpdates = 10
		
		// Use channels to coordinate
		done := make(chan bool, numGoroutines)
		
		// Start goroutines
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				defer func() { done <- true }()
				
				for j := 0; j < numUpdates; j++ {
					playerID := fmt.Sprintf("player%d", workerID)
					worldID := fmt.Sprintf("world%d", j%3)
					roomID := fmt.Sprintf("room%d", j%5)
					
					cm.UpdatePlayerLocation(playerID, worldID, roomID)
					
					// Verify location
					location, exists := cm.GetPlayerLocation(playerID)
					assert.True(t, exists)
					assert.Equal(t, playerID, location.PlayerID)
				}
			}(i)
		}
		
		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		
		// Verify final state
		for i := 0; i < numGoroutines; i++ {
			playerID := fmt.Sprintf("player%d", i)
			location, exists := cm.GetPlayerLocation(playerID)
			assert.True(t, exists)
			assert.Equal(t, playerID, location.PlayerID)
		}
	})
	
	// Test concurrent channel operations
	t.Run("ConcurrentChannelOperations", func(t *testing.T) {
		const numGoroutines = 50
		const numOperations = 10
		
		// Create channel first
		channelID := "test_channel"
		cm.CreateChannel(channelID, "Test Channel", "global")
		
		// Use channels to coordinate
		done := make(chan bool, numGoroutines)
		
		// Start goroutines
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				defer func() { done <- true }()
				
				for j := 0; j < numOperations; j++ {
					playerID := fmt.Sprintf("player%d_%d", workerID, j)
					
					// Join channel
					err := cm.JoinChannel(channelID, playerID)
					assert.NoError(t, err)
					
					// Check membership
					members := cm.GetChannelMembers(channelID)
					assert.Contains(t, members, playerID)
					
					// Leave channel
					err = cm.LeaveChannel(channelID, playerID)
					assert.NoError(t, err)
				}
			}(i)
		}
		
		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		
		// Verify final state - channel should be empty
		members := cm.GetChannelMembers(channelID)
		assert.Len(t, members, 0)
	})
}

// TestMessageValidation tests message validation
func TestMessageValidation(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("ValidMessages", func(t *testing.T) {
		// Valid say message
		msg := communication.CreateSayMessage("player1", "Alice", "Hello world!")
		err := cm.ValidateMessage(msg, "player1")
		assert.NoError(t, err)
		
		// Valid tell message
		msg = communication.CreateTellMessage("player1", "Alice", "player2", "Bob", "Secret")
		err = cm.ValidateMessage(msg, "player1")
		assert.NoError(t, err)
		
		// Valid chat message
		msg = message.Chat().WithText("Hello").FromUser("Alice").Build()
		err = cm.ValidateMessage(msg, "player1")
		assert.NoError(t, err)
	})
	
	t.Run("InvalidMessages", func(t *testing.T) {
		// Nil message
		err := cm.ValidateMessage(nil, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message is nil")
		
		// Empty sender ID
		msg := communication.CreateSayMessage("player1", "Alice", "Hello")
		err = cm.ValidateMessage(msg, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sender ID is empty")
		
		// Empty contents
		msg = &message.Message{Type: communication.MessageTypeSay, Contents: []byte{}}
		err = cm.ValidateMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message contents are empty")
		
		// Invalid communication data
		msg = &message.Message{
			Type:     communication.MessageTypeSay,
			Contents: []byte("invalid cbor"),
		}
		err = cm.ValidateMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid communication data")
		
		// Empty message text
		commData := communication.CommunicationData{
			Message:   "",
			SenderID:  "player1",
			Timestamp: time.Now().Unix(),
		}
		contents, _ := cbor.Marshal(commData)
		msg = &message.Message{
			Type:     communication.MessageTypeSay,
			Contents: contents,
		}
		err = cm.ValidateMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message text is empty")
		
		// Tell message missing target
		commData = communication.CommunicationData{
			Message:   "Secret",
			SenderID:  "player1",
			Timestamp: time.Now().Unix(),
		}
		contents, _ = cbor.Marshal(commData)
		msg = &message.Message{
			Type:     communication.MessageTypeTell,
			Contents: contents,
		}
		err = cm.ValidateMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tell message missing target")
	})
}

// TestCustomMessageHandlers tests custom message handler registration
func TestCustomMessageHandlers(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Register custom handler
	const CustomMessageType = message.MessageTypeCustom + 100
	var handledMessage *message.Message
	var handledSender string
	
	cm.RegisterMessageHandler(CustomMessageType, func(msg *message.Message, senderID string) error {
		handledMessage = msg
		handledSender = senderID
		return nil
	})
	
	// Test custom message
	msg := &message.Message{
		Type:     CustomMessageType,
		Contents: []byte("custom message"),
	}
	
	err := cm.HandleMessage(msg, "player1")
	assert.NoError(t, err)
	assert.Equal(t, msg, handledMessage)
	assert.Equal(t, "player1", handledSender)
	
	// Test unhandled message type
	msg = &message.Message{
		Type:     message.MessageTypeCustom + 999,
		Contents: []byte("unhandled"),
	}
	
	err = cm.HandleMessage(msg, "player1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no handler for message type")
}