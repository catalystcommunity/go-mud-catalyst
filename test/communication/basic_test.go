package communication_test

import (
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/stretchr/testify/assert"
)

// TestBasicCommunication tests basic communication functionality
func TestBasicCommunication(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	defer eventManager.Close()
	
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
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
	
	t.Run("MessageBuilders", func(t *testing.T) {
		playerID := "player1"
		playerName := "Alice"
		text := "Hello world!"
		
		// Test Say builder
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
		
		// Test message formatting
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice says: Hello world!", formatted)
	})
	
	t.Run("BasicMessageHandling", func(t *testing.T) {
		// Set up player location
		playerID := "player1"
		playerName := "Alice"
		worldID := "world1"
		roomID := "room1"
		
		cm.UpdatePlayerLocation(playerID, worldID, roomID)
		
		// Track events
		eventCount := 0
		eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
			eventCount++
			return events.EventResultContinue
		}, 0)
		
		// Create and handle say message
		msg := communication.CreateSayMessage(playerID, playerName, "Hello everyone!")
		err := cm.HandleMessage(msg, playerID)
		assert.NoError(t, err)
		
		// Check event was triggered
		assert.Equal(t, 1, eventCount)
	})
	
	t.Run("PlayersInRoomAndWorld", func(t *testing.T) {
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
		
		// Check world1/room1
		playersInRoom := cm.GetPlayersInRoom(world1, room1)
		assert.Len(t, playersInRoom, 2)
		assert.Contains(t, playersInRoom, players[0])
		assert.Contains(t, playersInRoom, players[1])
		
		// Check world1/room2
		playersInRoom = cm.GetPlayersInRoom(world1, room2)
		assert.Len(t, playersInRoom, 1)
		assert.Contains(t, playersInRoom, players[2])
		
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
	})
	
	t.Run("MessageValidation", func(t *testing.T) {
		// Valid say message
		msg := communication.CreateSayMessage("player1", "Alice", "Hello world!")
		err := cm.ValidateMessage(msg, "player1")
		assert.NoError(t, err)
		
		// Invalid message - nil
		err = cm.ValidateMessage(nil, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "message is nil")
		
		// Invalid message - empty sender
		err = cm.ValidateMessage(msg, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sender ID is empty")
	})
}

// TestCommunicationTypes tests different communication message types
func TestCommunicationTypes(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	defer eventManager.Close()
	
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Set up players
	player1ID := "player1"
	player1Name := "Alice"
	player2ID := "player2"
	player2Name := "Bob"
	worldID := "world1"
	roomID := "room1"
	
	cm.UpdatePlayerLocation(player1ID, worldID, roomID)
	cm.UpdatePlayerLocation(player2ID, worldID, roomID)
	
	t.Run("SayMessage", func(t *testing.T) {
		msg := communication.CreateSayMessage(player1ID, player1Name, "Hello everyone!")
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Format test
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice says: Hello everyone!", formatted)
	})
	
	t.Run("ShoutMessage", func(t *testing.T) {
		msg := communication.CreateShoutMessage(player1ID, player1Name, "Can everyone hear me?")
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Format test
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice shouts: Can everyone hear me?", formatted)
	})
	
	t.Run("EmoteMessage", func(t *testing.T) {
		msg := communication.CreateEmoteMessage(player1ID, player1Name, "waves at everyone")
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Format test
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice waves at everyone", formatted)
	})
	
	t.Run("TellMessage", func(t *testing.T) {
		msg := communication.CreateTellMessage(player1ID, player1Name, player2ID, player2Name, "Secret message")
		err := cm.HandleMessage(msg, player1ID)
		assert.NoError(t, err)
		
		// Format test
		formatted, err := communication.FormatCommunicationMessage(msg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice tells you: Secret message", formatted)
	})
}

// TestBasicErrorHandling tests basic error handling scenarios
func TestBasicErrorHandling(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	defer eventManager.Close()
	
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("MessageHandlingErrors", func(t *testing.T) {
		// Test handling message without location
		msg := communication.CreateSayMessage("player1", "Alice", "Hello")
		err := cm.HandleMessage(msg, "player1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "location not found")
		
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
	})
}