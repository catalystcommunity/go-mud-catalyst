package communication_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/stretchr/testify/assert"
)

// TestFullIntegration tests the complete communication system integration
func TestFullIntegration(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	// Create a realistic game scenario
	t.Run("MUDGameScenario", func(t *testing.T) {
		// Set up game world structure (commented for reference)
		// worlds: town, dungeon, forest
		// rooms per world: 3 each (square/tavern/shop, entrance/corridor/chamber, clearing/path/grove)
		
		// Set up players
		players := map[string]struct {
			name     string
			worldID  string
			roomID   string
		}{
			"player1": {"Alice", "town", "square"},
			"player2": {"Bob", "town", "tavern"},
			"player3": {"Charlie", "dungeon", "entrance"},
			"player4": {"Diana", "forest", "clearing"},
			"player5": {"Eve", "town", "square"},
		}
		
		// Place players in their starting locations
		for playerID, info := range players {
			cm.UpdatePlayerLocation(playerID, info.worldID, info.roomID)
		}
		
		// Create communication channels
		channels := map[string]struct {
			name        string
			channelType string
		}{
			"ooc":    {"Out of Character", "global"},
			"trade":  {"Trade Channel", "global"},
			"newbie": {"Newbie Help", "global"},
			"guild":  {"Guild Chat", "private"},
		}
		
		for channelID, info := range channels {
			cm.CreateChannel(channelID, info.name, info.channelType)
		}
		
		// Players join channels
		allPlayers := []string{"player1", "player2", "player3", "player4", "player5"}
		for _, playerID := range allPlayers {
			// Everyone joins OOC and trade
			err := cm.JoinChannel("ooc", playerID)
			assert.NoError(t, err)
			err = cm.JoinChannel("trade", playerID)
			assert.NoError(t, err)
		}
		
		// Only some players join guild and newbie channels
		guildMembers := []string{"player1", "player2", "player5"}
		for _, playerID := range guildMembers {
			err := cm.JoinChannel("guild", playerID)
			assert.NoError(t, err)
		}
		
		newbieMembers := []string{"player3", "player4"}
		for _, playerID := range newbieMembers {
			err := cm.JoinChannel("newbie", playerID)
			assert.NoError(t, err)
		}
		
		// Track all communication events
		var communicationLog []map[string]interface{}
		eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
			communicationLog = append(communicationLog, event.Data())
			return events.EventResultContinue
		}, 0)
		
		// Simulate game communication scenarios
		
		// 1. Alice says something in town square
		msg := communication.CreateSayMessage("player1", "Alice", "Welcome to Riverside Town!")
		err := cm.HandleMessage(msg, "player1")
		assert.NoError(t, err)
		
		// Should reach Alice and Eve (both in town square)
		assert.Len(t, communicationLog, 1)
		event := communicationLog[0]
		assert.Equal(t, "room", event["message_type"])
		assert.Equal(t, "town", event["world_id"])
		assert.Equal(t, "square", event["room_id"])
		recipients := event["recipients"].([]string)
		assert.Len(t, recipients, 2)
		assert.Contains(t, recipients, "player1")
		assert.Contains(t, recipients, "player5")
		
		// 2. Charlie shouts from the dungeon
		communicationLog = nil
		msg = communication.CreateShoutMessage("player3", "Charlie", "Anyone else brave enough for the dungeon?")
		err = cm.HandleMessage(msg, "player3")
		assert.NoError(t, err)
		
		// Should reach only Charlie (alone in dungeon world)
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		assert.Equal(t, "world", event["message_type"])
		assert.Equal(t, "dungeon", event["world_id"])
		recipients = event["recipients"].([]string)
		assert.Len(t, recipients, 1)
		assert.Contains(t, recipients, "player3")
		
		// 3. Diana tells Alice privately
		communicationLog = nil
		msg = communication.CreateTellMessage("player4", "Diana", "player1", "Alice", "I found rare herbs in the forest!")
		err = cm.HandleMessage(msg, "player4")
		assert.NoError(t, err)
		
		// Should reach Alice directly
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		assert.Equal(t, "private", event["message_type"])
		assert.Equal(t, "player1", event["recipient"])
		
		// 4. Bob emotes in the tavern
		communicationLog = nil
		msg = communication.CreateEmoteMessage("player2", "Bob", "raises his mug in celebration")
		err = cm.HandleMessage(msg, "player2")
		assert.NoError(t, err)
		
		// Should reach only Bob (alone in tavern)
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		assert.Equal(t, "room", event["message_type"])
		assert.Equal(t, "town", event["world_id"])
		assert.Equal(t, "tavern", event["room_id"])
		recipients = event["recipients"].([]string)
		assert.Len(t, recipients, 1)
		assert.Contains(t, recipients, "player2")
		
		// 5. Channel communication
		communicationLog = nil
		
		// Send message to OOC channel
		oocMsg := communication.Say().
			WithText("Anyone know where to find iron ore?").
			FromPlayer("player3", "Charlie").
			Build()
		err = cm.SendToChannel("ooc", oocMsg)
		assert.NoError(t, err)
		
		// Send message to guild channel
		guildMsg := communication.Say().
			WithText("Guild meeting tonight at 8 PM").
			FromPlayer("player1", "Alice").
			Build()
		err = cm.SendToChannel("guild", guildMsg)
		assert.NoError(t, err)
		
		// Should have 2 channel messages
		assert.Len(t, communicationLog, 2)
		
		// Verify channel membership
		oocMembers := cm.GetChannelMembers("ooc")
		assert.Len(t, oocMembers, 5) // All players
		
		guildMembers = cm.GetChannelMembers("guild")
		assert.Len(t, guildMembers, 3) // Alice, Bob, Eve
		
		tradeMembers := cm.GetChannelMembers("trade")
		assert.Len(t, tradeMembers, 5) // All players
		
		newbieMembers = cm.GetChannelMembers("newbie")
		assert.Len(t, newbieMembers, 2) // Charlie, Diana
		
		// 6. Player movement and communication
		communicationLog = nil
		
		// Eve moves from town square to tavern
		cm.UpdatePlayerLocation("player5", "town", "tavern")
		
		// Now Bob says something in tavern
		msg = communication.CreateSayMessage("player2", "Bob", "Welcome to the tavern!")
		err = cm.HandleMessage(msg, "player2")
		assert.NoError(t, err)
		
		// Should reach both Bob and Eve
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		recipients = event["recipients"].([]string)
		assert.Len(t, recipients, 2)
		assert.Contains(t, recipients, "player2")
		assert.Contains(t, recipients, "player5")
		
		// 7. Cross-world tell
		communicationLog = nil
		
		// Alice (in town) tells Charlie (in dungeon)
		msg = communication.CreateTellMessage("player1", "Alice", "player3", "Charlie", "Be careful in there!")
		err = cm.HandleMessage(msg, "player1")
		assert.NoError(t, err)
		
		// Should reach Charlie regardless of location
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		assert.Equal(t, "private", event["message_type"])
		assert.Equal(t, "player3", event["recipient"])
		
		// 8. Test player queries
		
		// Players in town world
		townPlayers := cm.GetPlayersInWorld("town")
		assert.Len(t, townPlayers, 3) // Alice (square), Bob (tavern), Eve (tavern)
		assert.Contains(t, townPlayers, "player1")
		assert.Contains(t, townPlayers, "player2")
		assert.Contains(t, townPlayers, "player5")
		
		// Players in town square
		squarePlayers := cm.GetPlayersInRoom("town", "square")
		assert.Len(t, squarePlayers, 1) // Only Alice
		assert.Contains(t, squarePlayers, "player1")
		
		// Players in tavern
		tavernPlayers := cm.GetPlayersInRoom("town", "tavern")
		assert.Len(t, tavernPlayers, 2) // Bob and Eve
		assert.Contains(t, tavernPlayers, "player2")
		assert.Contains(t, tavernPlayers, "player5")
		
		// Players in dungeon
		dungeonPlayers := cm.GetPlayersInWorld("dungeon")
		assert.Len(t, dungeonPlayers, 1) // Only Charlie
		assert.Contains(t, dungeonPlayers, "player3")
		
		// Players in forest
		forestPlayers := cm.GetPlayersInWorld("forest")
		assert.Len(t, forestPlayers, 1) // Only Diana
		assert.Contains(t, forestPlayers, "player4")
		
		// 9. Test message formatting
		sayMsg := communication.CreateSayMessage("player1", "Alice", "Test message")
		formatted, err := communication.FormatCommunicationMessage(sayMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice says: Test message", formatted)
		
		shoutMsg := communication.CreateShoutMessage("player1", "Alice", "Test shout")
		formatted, err = communication.FormatCommunicationMessage(shoutMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice shouts: Test shout", formatted)
		
		emoteMsg := communication.CreateEmoteMessage("player1", "Alice", "waves")
		formatted, err = communication.FormatCommunicationMessage(emoteMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice waves", formatted)
		
		tellMsg := communication.CreateTellMessage("player1", "Alice", "player2", "Bob", "Secret")
		formatted, err = communication.FormatCommunicationMessage(tellMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Alice tells you: Secret", formatted)
		
		// 10. Test legacy chat integration
		communicationLog = nil
		
		// Legacy chat message
		legacyMsg := message.Chat().
			WithText("This is a legacy chat message").
			FromUser("Alice").
			Build()
		err = cm.HandleMessage(legacyMsg, "player1")
		assert.NoError(t, err)
		
		// Should reach players in same room as Alice (town square)
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		assert.Equal(t, "room", event["message_type"])
		assert.Equal(t, "town", event["world_id"])
		assert.Equal(t, "square", event["room_id"])
		
		// Legacy whisper (triggers custom event, not MessageSent)
		communicationLog = nil
		legacyWhisper := message.Whisper().
			WithText("Legacy whisper").
			FromUser("Alice").
			ToUser("Bob").
			Build()
		err = cm.HandleMessage(legacyWhisper, "player1")
		assert.NoError(t, err)
		
		// Whisper triggers custom event, not MessageSent, so no events in communicationLog
		assert.Len(t, communicationLog, 0)
		
		// Legacy broadcast
		communicationLog = nil
		legacyBroadcast := message.Broadcast().
			WithText("Server announcement").
			FromUser("Admin").
			Build()
		err = cm.HandleMessage(legacyBroadcast, "admin")
		assert.NoError(t, err)
		
		// Should trigger global broadcast event
		assert.Len(t, communicationLog, 1)
		event = communicationLog[0]
		assert.Equal(t, "global_broadcast", event["message_type"])
	})
}

// TestPerformance tests performance characteristics
func TestPerformance(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("LargeScaleLocationTracking", func(t *testing.T) {
		const numPlayers = 10000
		const numWorlds = 100
		const numRooms = 1000
		
		start := time.Now()
		
		// Add players
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("perf_player_%d", i)
			worldID := fmt.Sprintf("world_%d", i%numWorlds)
			roomID := fmt.Sprintf("room_%d", i%numRooms)
			cm.UpdatePlayerLocation(playerID, worldID, roomID)
		}
		
		addDuration := time.Since(start)
		t.Logf("Added %d players in %v (%.2f players/sec)", 
			numPlayers, addDuration, float64(numPlayers)/addDuration.Seconds())
		
		// Query players
		start = time.Now()
		
		for i := 0; i < 1000; i++ {
			worldID := fmt.Sprintf("world_%d", i%numWorlds)
			players := cm.GetPlayersInWorld(worldID)
			assert.Greater(t, len(players), 0)
		}
		
		queryDuration := time.Since(start)
		t.Logf("Performed 1000 world queries in %v (%.2f queries/sec)", 
			queryDuration, 1000.0/queryDuration.Seconds())
		
		// Remove players
		start = time.Now()
		
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("perf_player_%d", i)
			cm.RemovePlayerLocation(playerID)
		}
		
		removeDuration := time.Since(start)
		t.Logf("Removed %d players in %v (%.2f players/sec)", 
			numPlayers, removeDuration, float64(numPlayers)/removeDuration.Seconds())
	})
	
	t.Run("ChannelOperationsPerformance", func(t *testing.T) {
		const numChannels = 1000
		const numMembers = 100
		
		// Create channels
		start := time.Now()
		
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("perf_channel_%d", i)
			cm.CreateChannel(channelID, fmt.Sprintf("Channel %d", i), "test")
		}
		
		createDuration := time.Since(start)
		t.Logf("Created %d channels in %v (%.2f channels/sec)", 
			numChannels, createDuration, float64(numChannels)/createDuration.Seconds())
		
		// Add members
		start = time.Now()
		
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("perf_channel_%d", i)
			
			for j := 0; j < numMembers; j++ {
				playerID := fmt.Sprintf("player_%d_%d", i, j)
				err := cm.JoinChannel(channelID, playerID)
				assert.NoError(t, err)
			}
		}
		
		joinDuration := time.Since(start)
		totalJoins := numChannels * numMembers
		t.Logf("Performed %d channel joins in %v (%.2f joins/sec)", 
			totalJoins, joinDuration, float64(totalJoins)/joinDuration.Seconds())
		
		// Query membership
		start = time.Now()
		
		for i := 0; i < numChannels; i++ {
			channelID := fmt.Sprintf("perf_channel_%d", i)
			members := cm.GetChannelMembers(channelID)
			assert.Len(t, members, numMembers)
		}
		
		queryDuration := time.Since(start)
		t.Logf("Queried %d channel memberships in %v (%.2f queries/sec)", 
			numChannels, queryDuration, float64(numChannels)/queryDuration.Seconds())
	})
	
	t.Run("MessageHandlingPerformance", func(t *testing.T) {
		const numPlayers = 1000
		const numMessages = 100
		
		// Set up players
		for i := 0; i < numPlayers; i++ {
			playerID := fmt.Sprintf("msg_perf_player_%d", i)
			worldID := fmt.Sprintf("world_%d", i%10)
			roomID := fmt.Sprintf("room_%d", i%50)
			cm.UpdatePlayerLocation(playerID, worldID, roomID)
		}
		
		// Handle messages
		start := time.Now()
		
		for i := 0; i < numMessages; i++ {
			for j := 0; j < numPlayers; j++ {
				playerID := fmt.Sprintf("msg_perf_player_%d", j)
				playerName := fmt.Sprintf("Player%d", j)
				messageText := fmt.Sprintf("Message %d from %s", i, playerName)
				
				msg := communication.CreateSayMessage(playerID, playerName, messageText)
				err := cm.HandleMessage(msg, playerID)
				assert.NoError(t, err)
			}
		}
		
		totalMessages := numMessages * numPlayers
		duration := time.Since(start)
		t.Logf("Handled %d messages in %v (%.2f messages/sec)", 
			totalMessages, duration, float64(totalMessages)/duration.Seconds())
	})
}

// TestCompatibility tests compatibility with existing systems
func TestCompatibility(t *testing.T) {
	// Setup
	storageManager := storage.NewManager()
	
	eventManager := events.NewEventManager()
	cm := communication.NewCommunicationManager(storageManager, eventManager)
	
	t.Run("MessageTypeCompatibility", func(t *testing.T) {
		// Ensure new message types don't conflict with existing ones
		existingTypes := []int64{
			message.MessageTypeHeartbeat,
			message.MessageTypeConnect,
			message.MessageTypeDisconnect,
			message.MessageTypeError,
			message.MessageTypeAck,
			message.MessageTypeNack,
			message.MessageTypeAuth,
			message.MessageTypeAuthSuccess,
			message.MessageTypeAuthFailure,
			message.MessageTypeChat,
			message.MessageTypeWhisper,
			message.MessageTypeBroadcast,
			message.MessageTypeJoinRoom,
			message.MessageTypeLeaveRoom,
			message.MessageTypeMove,
			message.MessageTypeAction,
			message.MessageTypeStatus,
			message.MessageTypeInventory,
		}
		
		newTypes := []int64{
			communication.MessageTypeSay,
			communication.MessageTypeShout,
			communication.MessageTypeEmote,
			communication.MessageTypeTell,
		}
		
		// Check no conflicts
		for _, existingType := range existingTypes {
			for _, newType := range newTypes {
				assert.NotEqual(t, existingType, newType, 
					"New message type %d conflicts with existing type %d", newType, existingType)
			}
		}
		
		// Check new types are in custom range
		for _, newType := range newTypes {
			assert.GreaterOrEqual(t, int64(newType), int64(message.MessageTypeCustom),
				"New message type %d should be >= MessageTypeCustom (%d)", newType, message.MessageTypeCustom)
		}
	})
	
	t.Run("EventIntegration", func(t *testing.T) {
		// Test that communication events integrate properly with existing event system
		var receivedEvents []events.Event
		
		// Register handlers for all event types
		eventTypes := []events.EventType{
			events.EventTypeMessageSent,
			events.EventTypeMessageReceived,
			events.EventTypeCustomPrefix + "test",
		}
		
		for _, eventType := range eventTypes {
			eventManager.RegisterHook(eventType, func(event events.Event) events.EventResult {
				receivedEvents = append(receivedEvents, event)
				return events.EventResultContinue
			}, 0)
		}
		
		// Set up player
		cm.UpdatePlayerLocation("player1", "world1", "room1")
		
		// Send various messages
		messages := []*message.Message{
			communication.CreateSayMessage("player1", "Alice", "Say test"),
			communication.CreateShoutMessage("player1", "Alice", "Shout test"),
			communication.CreateEmoteMessage("player1", "Alice", "emotes"),
			communication.CreateTellMessage("player1", "Alice", "player2", "Bob", "Tell test"),
		}
		
		for _, msg := range messages {
			err := cm.HandleMessage(msg, "player1")
			assert.NoError(t, err)
		}
		
		// Send legacy messages
		legacyMessages := []*message.Message{
			message.Chat().WithText("Chat test").FromUser("Alice").Build(),
			message.Whisper().WithText("Whisper test").FromUser("Alice").ToUser("Bob").Build(),
			message.Broadcast().WithText("Broadcast test").FromUser("Alice").Build(),
		}
		
		for _, msg := range legacyMessages {
			err := cm.HandleMessage(msg, "player1")
			assert.NoError(t, err)
		}
		
		// Should have received events for most messages (whisper triggers different event type)
		expectedEvents := len(messages) + len(legacyMessages) - 1 // -1 for whisper
		assert.GreaterOrEqual(t, len(receivedEvents), expectedEvents)
		
		// Verify events have proper structure
		for _, event := range receivedEvents {
			assert.NotNil(t, event.Type())
			eventData := event.Data()
			assert.NotNil(t, eventData)
			
			if event.Type() == events.EventTypeMessageSent {
				assert.Contains(t, eventData, "message_type")
				assert.Contains(t, eventData, "message")
			}
		}
	})
}