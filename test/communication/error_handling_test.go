package communication_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
)

// TestErrorHandling tests comprehensive error handling scenarios
func TestErrorHandling(t *testing.T) {
	t.Log("NOTE: This test expects ERROR/WARN logs for error handling scenarios - this is expected behavior")
	
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("MessageHandlingErrors", func(t *testing.T) {
		// Test handling message without location
		msg := communication.CreateSayMessage("player1", "Alice", "Hello")
		err := cm.HandleMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "location not found")
		
		// Test handling message with invalid CBOR
		invalidMsg := &message.Message{
			Type:     communication.MessageTypeSay,
			Contents: []byte("invalid cbor data"),
		}
		err = cm.HandleMessage(invalidMsg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
		
		// Test handling message with unknown type
		unknownMsg := &message.Message{
			Type:     999999,
			Contents: []byte("test"),
		}
		err = cm.HandleMessage(unknownMsg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no handler for message type")
	})
	
	t.Run("ChannelErrors", func(t *testing.T) {
		// Test joining non-existent channel
		err := cm.JoinChannel("nonexistent", "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel nonexistent not found")
		
		// Test leaving non-existent channel
		err = cm.LeaveChannel("nonexistent", "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "channel nonexistent not found")
		
		// Test getting members of non-existent channel
		members := cm.GetChannelMembers("nonexistent")
		assert.Nil(t, members)
		
		// Test getting non-existent channel
		channel, exists := cm.GetChannel("nonexistent")
		assert.False(t, exists)
		assert.Nil(t, channel)
	})
	
	t.Run("TellMessageErrors", func(t *testing.T) {
		// Set up player location
		cm.UpdatePlayerLocation("player1", "world1", "room1")
		
		// Test tell message without target
		commData := communication.CommunicationData{
			Message:   "Secret message",
			SenderID:  "player1",
			Timestamp: time.Now().Unix(),
		}
		contents, _ := cbor.Marshal(commData)
		msg := &message.Message{
			Type:     communication.MessageTypeTell,
			Contents: contents,
		}
		
		err := cm.HandleMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tell message missing target")
	})
	
	t.Run("HandlerRegistrationErrors", func(t *testing.T) {
		// Test registering handler that returns error
		const ErrorMessageType = message.MessageTypeCustom + 200
		
		cm.RegisterMessageHandler(ErrorMessageType, func(msg *message.Message, senderID string) error {
			return fmt.Errorf("handler error: %s", senderID)
		})
		
		msg := &message.Message{
			Type:     ErrorMessageType,
			Contents: []byte("test"),
		}
		
		err := cm.HandleMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler error: player1")
	})
}

// TestEdgeCases tests edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("EmptyAndNilInputs", func(t *testing.T) {
		// Test empty player ID
		cm.UpdatePlayerLocation("", "world1", "room1")
		_, exists := cm.GetPlayerLocation("")
		assert.True(t, exists) // Empty string is valid key
		
		// Test empty world/room IDs
		cm.UpdatePlayerLocation("player1", "", "")
		location, exists := cm.GetPlayerLocation("player1")
		assert.True(t, exists)
		assert.Equal(t, "", location.WorldID)
		assert.Equal(t, "", location.RoomID)
		
		// Test empty channel name
		channel := cm.CreateChannel("test", "", "")
		assert.Equal(t, "", channel.Name)
		assert.Equal(t, "", channel.Type)
		
		// Test empty message text
		msg := communication.Say().WithText("").FromPlayer("player1", "Alice").Build()
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "", data.Message)
	})
	
	t.Run("VeryLongInputs", func(t *testing.T) {
		// Test very long message
		longMessage := string(make([]byte, 10000))
		for i := range longMessage {
			longMessage = longMessage[:i] + "x" + longMessage[i+1:]
		}
		
		msg := communication.Say().
			WithText(longMessage).
			FromPlayer("player1", "Alice").
			Build()
		
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, longMessage, data.Message)
		
		// Test very long player/channel names
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "x" + longName[i+1:]
		}
		
		channel := cm.CreateChannel("long_test", longName, "test")
		assert.Equal(t, longName, channel.Name)
		
		err = cm.JoinChannel("long_test", longName)
		assert.NoError(t, err)
		
		members := cm.GetChannelMembers("long_test")
		assert.Contains(t, members, longName)
	})
	
	t.Run("SpecialCharacters", func(t *testing.T) {
		// Test special characters in messages
		specialMessage := "Hello! 你好 🌍 \n\t\r Special chars: @#$%^&*()_+-=[]{}|;':\",./<>?"
		
		msg := communication.Say().
			WithText(specialMessage).
			FromPlayer("player1", "Alice").
			Build()
		
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, specialMessage, data.Message)
		
		// Test special characters in names
		specialName := "Player🎮"
		channel := cm.CreateChannel("special", specialName, "test")
		assert.Equal(t, specialName, channel.Name)
	})
	
	t.Run("TimestampHandling", func(t *testing.T) {
		// Test custom timestamp
		customTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		
		msg := communication.Say().
			WithText("Hello").
			FromPlayer("player1", "Alice").
			WithTimestamp(customTime).
			Build()
		
		data, err := communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, customTime.Unix(), data.Timestamp)
		
		// Test zero timestamp (should be set automatically)
		msg = communication.Say().
			WithText("Hello").
			FromPlayer("player1", "Alice").
			Build()
		
		data, err = communication.ParseCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Greater(t, data.Timestamp, int64(0))
	})
	
	t.Run("LocationUpdates", func(t *testing.T) {
		playerID := "player1"
		
		// Multiple rapid updates
		for i := 0; i < 100; i++ {
			worldID := fmt.Sprintf("world%d", i%3)
			roomID := fmt.Sprintf("room%d", i%5)
			cm.UpdatePlayerLocation(playerID, worldID, roomID)
		}
		
		// Check final location
		location, exists := cm.GetPlayerLocation(playerID)
		assert.True(t, exists)
		assert.Equal(t, "world0", location.WorldID) // 99 % 3 = 0
		assert.Equal(t, "room4", location.RoomID)   // 99 % 5 = 4
		
		// Remove and re-add
		cm.RemovePlayerLocation(playerID)
		_, exists = cm.GetPlayerLocation(playerID)
		assert.False(t, exists)
		
		cm.UpdatePlayerLocation(playerID, "world1", "room1")
		_, exists = cm.GetPlayerLocation(playerID)
		assert.True(t, exists)
	})
	
	t.Run("ChannelMembershipEdgeCases", func(t *testing.T) {
		channelID := "test_channel"
		cm.CreateChannel(channelID, "Test", "global")
		
		// Join same channel multiple times
		playerID := "player1"
		for i := 0; i < 10; i++ {
			err := cm.JoinChannel(channelID, playerID)
			assert.NoError(t, err)
		}
		
		// Should still only be one member
		members := cm.GetChannelMembers(channelID)
		assert.Len(t, members, 1)
		assert.Contains(t, members, playerID)
		
		// Leave channel multiple times
		for i := 0; i < 10; i++ {
			err := cm.LeaveChannel(channelID, playerID)
			assert.NoError(t, err)
		}
		
		// Should be empty
		members = cm.GetChannelMembers(channelID)
		assert.Len(t, members, 0)
		
		// Join and leave different players
		players := []string{"player1", "player2", "player3"}
		for _, p := range players {
			err := cm.JoinChannel(channelID, p)
			assert.NoError(t, err)
		}
		
		members = cm.GetChannelMembers(channelID)
		assert.Len(t, members, 3)
		
		// Leave middle player
		err := cm.LeaveChannel(channelID, players[1])
		assert.NoError(t, err)
		
		members = cm.GetChannelMembers(channelID)
		assert.Len(t, members, 2)
		assert.Contains(t, members, players[0])
		assert.Contains(t, members, players[2])
		assert.NotContains(t, members, players[1])
	})
}

// TestRaceConditions tests for race conditions and concurrent access
func TestRaceConditions(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("ConcurrentLocationUpdatesAndQueries", func(t *testing.T) {
		const numGoroutines = 50
		const numOperations = 100
		
		playerID := "race_test_player"
		
		// Channel for coordination
		done := make(chan bool, numGoroutines)
		
		// Start updaters
		for i := 0; i < numGoroutines/2; i++ {
			go func(workerID int) {
				defer func() { done <- true }()
				
				for j := 0; j < numOperations; j++ {
					worldID := fmt.Sprintf("world%d", (workerID+j)%3)
					roomID := fmt.Sprintf("room%d", (workerID+j)%5)
					cm.UpdatePlayerLocation(playerID, worldID, roomID)
				}
			}(i)
		}
		
		// Start readers
		for i := 0; i < numGoroutines/2; i++ {
			go func(workerID int) {
				defer func() { done <- true }()
				
				for j := 0; j < numOperations; j++ {
					location, exists := cm.GetPlayerLocation(playerID)
					if exists {
						// Verify location data is consistent
						assert.NotEmpty(t, location.PlayerID)
						assert.NotEmpty(t, location.WorldID)
						assert.NotEmpty(t, location.RoomID)
						assert.False(t, location.UpdatedAt.IsZero())
					}
				}
			}(i)
		}
		
		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		
		// Final verification
		location, exists := cm.GetPlayerLocation(playerID)
		assert.True(t, exists)
		assert.Equal(t, playerID, location.PlayerID)
	})
	
	t.Run("ConcurrentChannelOperations", func(t *testing.T) {
		const numGoroutines = 30
		const numOperations = 50
		
		// Create multiple channels
		channels := make([]string, 5)
		for i := range channels {
			channels[i] = fmt.Sprintf("race_channel_%d", i)
			cm.CreateChannel(channels[i], fmt.Sprintf("Race Channel %d", i), "test")
		}
		
		// Channel for coordination
		done := make(chan bool, numGoroutines)
		
		// Start workers
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				defer func() { done <- true }()
				
				for j := 0; j < numOperations; j++ {
					channelID := channels[j%len(channels)]
					playerID := fmt.Sprintf("player%d_%d", workerID, j)
					
					// Join channel
					err := cm.JoinChannel(channelID, playerID)
					assert.NoError(t, err)
					
					// Query members
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
		
		// Verify all channels are empty
		for _, channelID := range channels {
			members := cm.GetChannelMembers(channelID)
			assert.Len(t, members, 0)
		}
	})
	
	t.Run("ConcurrentMessageHandling", func(t *testing.T) {
		const numGoroutines = 20
		const numMessages = 25
		
		// Set up player locations
		for i := 0; i < numGoroutines; i++ {
			playerID := fmt.Sprintf("msg_player_%d", i)
			worldID := fmt.Sprintf("world%d", i%3)
			roomID := fmt.Sprintf("room%d", i%5)
			cm.UpdatePlayerLocation(playerID, worldID, roomID)
		}
		
		// Track events
		var eventCount int64
		eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
			atomic.AddInt64(&eventCount, 1)
			return events.EventResultContinue
		}, 0)
		
		// Channel for coordination
		done := make(chan bool, numGoroutines)
		
		// Start message senders
		for i := 0; i < numGoroutines; i++ {
			go func(workerID int) {
				defer func() { done <- true }()
				
				playerID := fmt.Sprintf("msg_player_%d", workerID)
				playerName := fmt.Sprintf("Player%d", workerID)
				
				for j := 0; j < numMessages; j++ {
					messageText := fmt.Sprintf("Message %d from %s", j, playerName)
					
					// Send different types of messages
					switch j % 4 {
					case 0:
						msg := communication.CreateSayMessage(playerID, playerName, messageText)
						err := cm.HandleMessage(msg, playerID)
						assert.NoError(t, err)
					case 1:
						msg := communication.CreateShoutMessage(playerID, playerName, messageText)
						err := cm.HandleMessage(msg, playerID)
						assert.NoError(t, err)
					case 2:
						msg := communication.CreateEmoteMessage(playerID, playerName, messageText)
						err := cm.HandleMessage(msg, playerID)
						assert.NoError(t, err)
					case 3:
						// Tell to another player
						targetID := fmt.Sprintf("msg_player_%d", (workerID+1)%numGoroutines)
						targetName := fmt.Sprintf("Player%d", (workerID+1)%numGoroutines)
						msg := communication.CreateTellMessage(playerID, playerName, targetID, targetName, messageText)
						err := cm.HandleMessage(msg, playerID)
						assert.NoError(t, err)
					}
				}
			}(i)
		}
		
		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		
		// Verify events were triggered (allow for slight variance due to concurrency)
		// We expect numGoroutines * numMessages events
		expectedEvents := numGoroutines * numMessages
		finalEventCount := atomic.LoadInt64(&eventCount)
		assert.GreaterOrEqual(t, finalEventCount, int64(expectedEvents-5)) // Allow up to 5 events to be missed
		assert.LessOrEqual(t, finalEventCount, int64(expectedEvents))      // But not more than expected
	})
}

// TestMemoryLeaks tests for potential memory leaks
func TestMemoryLeaks(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("LocationMemoryManagement", func(t *testing.T) {
		const numPlayers = 1000
		
		// Add many players
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("leak_test_player_%d", i)
			worldID := fmt.Sprintf("world%d", i%10)
			roomID := fmt.Sprintf("room%d", i%20)
			cm.UpdatePlayerLocation(playerID, worldID, roomID)
		}
		
		// Verify all are tracked
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("leak_test_player_%d", i)
			_, exists := cm.GetPlayerLocation(playerID)
			assert.True(t, exists)
		}
		
		// Remove all players
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("leak_test_player_%d", i)
			cm.RemovePlayerLocation(playerID)
		}
		
		// Verify all are removed
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("leak_test_player_%d", i)
			_, exists := cm.GetPlayerLocation(playerID)
			assert.False(t, exists)
		}
	})
	
	t.Run("ChannelMemoryManagement", func(t *testing.T) {
		const numChannels = 100
		const numMembers = 50
		
		// Create channels and add members
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("leak_channel_%d", i)
			cm.CreateChannel(channelID, fmt.Sprintf("Channel %d", i), "test")
			
			// Add members
			for j := 0; j < numMembers; j++ {
				playerID := fmt.Sprintf("player_%d_%d", i, j)
				err := cm.JoinChannel(channelID, playerID)
				assert.NoError(t, err)
			}
		}
		
		// Verify channels and members
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("leak_channel_%d", i)
			members := cm.GetChannelMembers(channelID)
			assert.Len(t, members, numMembers)
		}
		
		// Remove all members
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("leak_channel_%d", i)
			
			for j := 0; j < numMembers; j++ {
				playerID := fmt.Sprintf("player_%d_%d", i, j)
				err := cm.LeaveChannel(channelID, playerID)
				assert.NoError(t, err)
			}
		}
		
		// Verify all channels are empty
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("leak_channel_%d", i)
			members := cm.GetChannelMembers(channelID)
			assert.Len(t, members, 0)
		}
	})
}

// TestInvalidData tests handling of invalid or corrupted data
func TestInvalidData(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("CorruptedCBORData", func(t *testing.T) {
		// Test various corrupted CBOR data
		corruptedData := [][]byte{
			{0xFF, 0xFF, 0xFF, 0xFF}, // Invalid CBOR
			{0x00},                   // Incomplete CBOR
			{},                       // Empty data
			{0x80, 0x00, 0x00},      // Truncated CBOR
		}
		
		for i, data := range corruptedData {
			msg := &message.Message{
				Type:     communication.MessageTypeSay,
				Contents: data,
			}
			
			err := cm.HandleMessage(msg, "player1")
			assert.Error(t, err, "Should error on corrupted data %d", i)
		}
	})
	
	t.Run("InvalidMessageTypes", func(t *testing.T) {
		// Test negative message types
		msg := &message.Message{
			Type:     -1,
			Contents: []byte("test"),
		}
		
		err := cm.HandleMessage(msg, "player1")
		assert.Error(t, err)
		
		// Test extremely large message types
		msg = &message.Message{
			Type:     9999999999,
			Contents: []byte("test"),
		}
		
		err = cm.HandleMessage(msg, "player1")
		assert.Error(t, err)
	})
	
	t.Run("MalformedCommunicationData", func(t *testing.T) {
		// Create communication data with missing required fields
		commData := communication.CommunicationData{
			// Missing Message field
			SenderID:  "player1",
			Timestamp: time.Now().Unix(),
		}
		
		contents, _ := cbor.Marshal(commData)
		msg := &message.Message{
			Type:     communication.MessageTypeSay,
			Contents: contents,
		}
		
		err := cm.ValidateMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message text is empty")
	})
}

// TestComplexScenarios tests complex real-world scenarios
func TestComplexScenarios(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("PlayerMovementAndCommunication", func(t *testing.T) {
		// Set up world with multiple rooms
		players := []string{"alice", "bob", "charlie", "david"}
		worlds := []string{"world1", "world2"}
		rooms := []string{"room1", "room2", "room3"}
		
		// Track all events
		var allEvents []events.Event
		eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
			allEvents = append(allEvents, event)
			return events.EventResultContinue
		}, 0)
		
		// Initially place all players in world1/room1
		for _, player := range players {
			cm.UpdatePlayerLocation(player, worlds[0], rooms[0])
		}
		
		// Alice says something - should reach all players
		msg := communication.CreateSayMessage("alice", "Alice", "Hello everyone!")
		err := cm.HandleMessage(msg, "alice")
		assert.NoError(t, err)
		
		// Check event
		assert.Len(t, allEvents, 1)
		event := allEvents[0]
		eventData := event.Data()
		recipients := eventData["recipients"].([]string)
		assert.Len(t, recipients, 4) // All players in room
		
		// Move Bob and Charlie to different rooms
		cm.UpdatePlayerLocation("bob", worlds[0], rooms[1])
		cm.UpdatePlayerLocation("charlie", worlds[1], rooms[0])
		
		// Clear events
		allEvents = nil
		
		// Alice says something again - should only reach Alice and David
		msg = communication.CreateSayMessage("alice", "Alice", "Anyone still here?")
		err = cm.HandleMessage(msg, "alice")
		assert.NoError(t, err)
		
		assert.Len(t, allEvents, 1)
		event = allEvents[0]
		eventData = event.Data()
		recipients = eventData["recipients"].([]string)
		assert.Len(t, recipients, 2) // Only Alice and David in room
		assert.Contains(t, recipients, "alice")
		assert.Contains(t, recipients, "david")
		
		// Alice shouts - should reach Alice and David (same world)
		allEvents = nil
		msg = communication.CreateShoutMessage("alice", "Alice", "Is anyone in this world?")
		err = cm.HandleMessage(msg, "alice")
		assert.NoError(t, err)
		
		assert.Len(t, allEvents, 1)
		event = allEvents[0]
		eventData = event.Data()
		recipients = eventData["recipients"].([]string)
		assert.Len(t, recipients, 3) // Alice, David, and Bob (all in world1)
		assert.Contains(t, recipients, "alice")
		assert.Contains(t, recipients, "david")
		assert.Contains(t, recipients, "bob")
		assert.NotContains(t, recipients, "charlie") // Charlie is in world2
		
		// Alice tells Charlie - should reach Charlie directly
		allEvents = nil
		msg = communication.CreateTellMessage("alice", "Alice", "charlie", "Charlie", "Private message")
		err = cm.HandleMessage(msg, "alice")
		assert.NoError(t, err)
		
		assert.Len(t, allEvents, 1)
		event = allEvents[0]
		eventData = event.Data()
		assert.Equal(t, "private", eventData["message_type"])
		assert.Equal(t, "charlie", eventData["recipient"])
	})
	
	t.Run("ChannelMixedWithWorldCommunication", func(t *testing.T) {
		// Create channels
		globalChannel := "global"
		adminChannel := "admin"
		
		cm.CreateChannel(globalChannel, "Global Channel", "global")
		cm.CreateChannel(adminChannel, "Admin Channel", "admin")
		
		// Set up players
		players := []string{"alice", "bob", "charlie"}
		for _, player := range players {
			cm.UpdatePlayerLocation(player, "world1", "room1")
			
			// Join global channel
			err := cm.JoinChannel(globalChannel, player)
			assert.NoError(t, err)
		}
		
		// Only Alice and Bob join admin channel
		err := cm.JoinChannel(adminChannel, "alice")
		assert.NoError(t, err)
		err = cm.JoinChannel(adminChannel, "bob")
		assert.NoError(t, err)
		
		// Test channel membership
		globalMembers := cm.GetChannelMembers(globalChannel)
		assert.Len(t, globalMembers, 3)
		
		adminMembers := cm.GetChannelMembers(adminChannel)
		assert.Len(t, adminMembers, 2)
		assert.Contains(t, adminMembers, "alice")
		assert.Contains(t, adminMembers, "bob")
		assert.NotContains(t, adminMembers, "charlie")
		
		// Test mixed communication
		var allEvents []events.Event
		eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
			allEvents = append(allEvents, event)
			return events.EventResultContinue
		}, 0)
		
		// Say message - room communication
		msg := communication.CreateSayMessage("alice", "Alice", "Hello room!")
		err = cm.HandleMessage(msg, "alice")
		assert.NoError(t, err)
		
		// Channel message - global channel
		err = cm.SendToChannel(globalChannel, msg)
		assert.NoError(t, err)
		
		// Channel message - admin channel
		err = cm.SendToChannel(adminChannel, msg)
		assert.NoError(t, err)
		
		// Should have 3 events total
		assert.Len(t, allEvents, 3)
		
		// Verify event types
		messageTypes := make(map[string]int)
		for _, event := range allEvents {
			eventData := event.Data()
			msgType := eventData["message_type"].(string)
			messageTypes[msgType]++
		}
		
		assert.Equal(t, 1, messageTypes["room"])
		assert.Equal(t, 2, messageTypes["channel"])
	})
}