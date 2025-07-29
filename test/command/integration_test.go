package command

import (
	"fmt"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/command"
	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// Test integration with message system
func TestCommandMessageIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Create a command
	cmd, err := cm.ParseCommand("test argument", playerID)
	if err != nil {
		t.Errorf("Unexpected error parsing command: %v", err)
	}

	// Convert to message
	msg, err := command.CreateCommandMessage(cmd)
	if err != nil {
		t.Errorf("Unexpected error creating message: %v", err)
	}

	// Verify message properties
	if msg.Type != command.MessageTypeCommand {
		t.Errorf("Expected message type %d, got %d", command.MessageTypeCommand, msg.Type)
	}

	if msg.ID != cmd.ID {
		t.Errorf("Expected message ID %s, got %s", cmd.ID, msg.ID)
	}

	if !msg.RequiresAck {
		t.Error("Command message should require acknowledgment")
	}

	if msg.AckTimeout != 5*time.Second {
		t.Errorf("Expected ack timeout 5s, got %v", msg.AckTimeout)
	}

	// Parse message back to command
	parsedCmd, err := command.ParseCommandMessage(msg)
	if err != nil {
		t.Errorf("Unexpected error parsing message: %v", err)
	}

	// Verify command properties
	if parsedCmd.Name != cmd.Name {
		t.Errorf("Expected command name %s, got %s", cmd.Name, parsedCmd.Name)
	}

	if parsedCmd.Argument != cmd.Argument {
		t.Errorf("Expected argument %s, got %s", cmd.Argument, parsedCmd.Argument)
	}

	if parsedCmd.PlayerID != cmd.PlayerID {
		t.Errorf("Expected player ID %s, got %s", cmd.PlayerID, parsedCmd.PlayerID)
	}

	if parsedCmd.Raw != cmd.Raw {
		t.Errorf("Expected raw %s, got %s", cmd.Raw, parsedCmd.Raw)
	}
}

// Test integration with event system
func TestCommandEventIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Register a test handler
	executed := false
	cm.RegisterHandler("test", func(cmd *command.Command) error {
		executed = true
		return nil
	})

	// Process a command
	err := cm.ProcessCommand("test", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	// Process the queue
	err = cm.ProcessQueue(playerID)
	if err != nil {
		t.Errorf("Unexpected error processing queue: %v", err)
	}

	// Verify handler was executed
	if !executed {
		t.Error("Command handler should have been executed")
	}

	// Check events
	events := mockEventManager.GetEvents()
	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

	// Find specific events
	foundQueued := false
	foundExecuting := false
	foundExecuted := false

	for _, event := range events {
		if event.Data()["event"] == "command_queued" {
			foundQueued = true
		}
		if event.Data()["event"] == "command_executing" {
			foundExecuting = true
		}
		if event.Data()["event"] == "command_executed" {
			foundExecuted = true
		}
	}

	if !foundQueued {
		t.Error("Expected to find command_queued event")
	}
	if !foundExecuting {
		t.Error("Expected to find command_executing event")
	}
	if !foundExecuted {
		t.Error("Expected to find command_executed event")
	}
}

// Test integration with storage system
func TestCommandStorageIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	
	// Create storage manager
	storageManager := storage.NewManager()

	// Create communication manager
	commManager := communication.NewCommunicationManager(storageManager, mockEventManager)

	// Create command manager
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Register a command that interacts with storage
	var capturedCmd *command.Command
	cm.RegisterHandler("create_player", func(cmd *command.Command) error {
		capturedCmd = cmd
		// Simulate creating a player
		player, _, err := storageManager.CreateNewPlayer(cmd.Argument, "test@example.com")
		if err != nil {
			return err
		}
		player.ID = cmd.PlayerID
		player.CurrentWorldID = "world1"
		player.CurrentRoomID = "room1"
		return storageManager.Players().Save(player)
	})

	// Process command with unique username
	uniqueUsername := fmt.Sprintf("TestPlayer%d", time.Now().UnixNano())
	err := cm.ProcessCommand("create_player "+uniqueUsername, playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	// Execute command
	err = cm.ProcessQueue(playerID)
	if err != nil {
		t.Errorf("Unexpected error executing command: %v", err)
	}

	// Verify command was executed
	if capturedCmd == nil {
		t.Error("Command should have been executed")
	}

	// Verify player was created in storage
	player, err := storageManager.Players().Load(playerID)
	if err != nil {
		t.Errorf("Unexpected error loading player: %v", err)
	}

	if player.Username != uniqueUsername {
		t.Errorf("Expected player username '%s', got %s", uniqueUsername, player.Username)
	}

	// Update player location through communication manager
	commManager.UpdatePlayerLocation(playerID, "world1", "room1")

	// Verify location was updated
	location, exists := commManager.GetPlayerLocation(playerID)
	if !exists {
		t.Error("Player location should exist")
	}

	if location.WorldID != "world1" || location.RoomID != "room1" {
		t.Errorf("Expected location world1/room1, got %s/%s", location.WorldID, location.RoomID)
	}
}

// Test integration with communication system
func TestCommandCommunicationIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	
	// Create storage manager
	storageManager := storage.NewManager()

	// Create communication manager
	commManager := communication.NewCommunicationManager(storageManager, mockEventManager)

	// Create command manager
	cm := command.NewCommandManager(nil, mockEventManager)
	
	player1 := "player1"
	player2 := "player2"

	// Set up player locations
	commManager.UpdatePlayerLocation(player1, "world1", "room1")
	commManager.UpdatePlayerLocation(player2, "world1", "room1")

	// Register communication commands
	cm.RegisterHandler("say", func(cmd *command.Command) error {
		// Create say message
		commData := communication.CommunicationData{
			Message:    cmd.Argument,
			SenderID:   cmd.PlayerID,
			SenderName: "TestPlayer",
			RoomID:     "room1",
			WorldID:    "world1",
			Timestamp:  time.Now().Unix(),
		}
		
		msg := message.NewMessage(communication.MessageTypeSay).WithContent(commData).Build()
		
		return commManager.SendToRoom("world1", "room1", msg)
	})

	// Clear previous events
	mockEventManager.ClearEvents()

	// Process say command
	err := cm.ProcessCommand("say Hello everyone!", player1)
	if err != nil {
		t.Errorf("Unexpected error processing say command: %v", err)
	}

	// Execute command
	err = cm.ProcessQueue(player1)
	if err != nil {
		t.Errorf("Unexpected error executing say command: %v", err)
	}

	// Verify communication event was triggered
	events := mockEventManager.GetEvents()
	foundMessageSent := false
	for _, event := range events {
		if event.Data()["message_type"] == "room" {
			foundMessageSent = true
			if event.Data()["world_id"] != "world1" {
				t.Errorf("Expected world_id 'world1', got %v", event.Data()["world_id"])
			}
			if event.Data()["room_id"] != "room1" {
				t.Errorf("Expected room_id 'room1', got %v", event.Data()["room_id"])
			}
		}
	}

	if !foundMessageSent {
		t.Error("Expected to find message_sent event")
	}
}

// Test command handler registration and execution
func TestCommandHandlerIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Test multiple command handlers
	handlers := map[string]bool{
		"look":      false,
		"inventory": false,
		"help":      false,
		"quit":      false,
	}

	// Register handlers
	for cmdName := range handlers {
		cmdName := cmdName // capture for closure
		cm.RegisterHandler(cmdName, func(cmd *command.Command) error {
			handlers[cmdName] = true
			return nil
		})
	}

	// Execute commands
	for cmdName := range handlers {
		err := cm.ProcessCommand(cmdName, playerID)
		if err != nil {
			t.Errorf("Unexpected error processing %s: %v", cmdName, err)
		}

		err = cm.ProcessQueue(playerID)
		if err != nil {
			t.Errorf("Unexpected error executing %s: %v", cmdName, err)
		}
	}

	// Verify all handlers were executed
	for cmdName, executed := range handlers {
		if !executed {
			t.Errorf("Handler for %s was not executed", cmdName)
		}
	}

	// Test command not found
	err := cm.ProcessCommand("nonexistent", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing nonexistent command: %v", err)
	}

	err = cm.ProcessQueue(playerID)
	if err != command.ErrCommandNotFound {
		t.Errorf("Expected ErrCommandNotFound, got %v", err)
	}
}

// Test command system with message router
func TestCommandMessageRouterIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Create message router
	router := message.NewMessageRouter()

	// Register command handler in router
	router.RegisterHandler(command.MessageTypeCommand, func(msg *message.Message) error {
		cmd, err := command.ParseCommandMessage(msg)
		if err != nil {
			return err
		}
		return cm.ExecuteCommand(cmd)
	})

	// Register a test command
	executed := false
	cm.RegisterHandler("test", func(cmd *command.Command) error {
		executed = true
		return nil
	})

	// Create command and convert to message
	cmd, err := cm.ParseCommand("test", playerID)
	if err != nil {
		t.Errorf("Unexpected error parsing command: %v", err)
	}

	msg, err := command.CreateCommandMessage(cmd)
	if err != nil {
		t.Errorf("Unexpected error creating message: %v", err)
	}

	// Route message through router
	err = router.RouteMessage(msg)
	if err != nil {
		t.Errorf("Unexpected error routing message: %v", err)
	}

	// Verify handler was executed
	if !executed {
		t.Error("Command handler should have been executed")
	}
}

// Test command system performance under load
func TestCommandIntegrationPerformance(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	
	// Register a simple handler
	cm.RegisterHandler("test", func(cmd *command.Command) error {
		return nil
	})

	// Test with multiple players - adjust for queue limits
	numPlayers := 10
	commandsPerPlayer := 2

	start := time.Now()

	for i := 0; i < numPlayers; i++ {
		playerID := fmt.Sprintf("player%d", i)
		
		for j := 0; j < commandsPerPlayer; j++ {
			err := cm.ProcessCommand("test", playerID)
			if err != nil {
				t.Errorf("Unexpected error for player %s command %d: %v", playerID, j, err)
			}
		}
	}

	// Process all queues
	for i := 0; i < numPlayers; i++ {
		playerID := fmt.Sprintf("player%d", i)
		
		for j := 0; j < commandsPerPlayer; j++ {
			err := cm.ProcessQueue(playerID)
			if err != nil {
				t.Errorf("Unexpected error processing queue for player %s: %v", playerID, err)
			}
		}
	}

	duration := time.Since(start)
	totalCommands := numPlayers * commandsPerPlayer
	commandsPerSecond := float64(totalCommands) / duration.Seconds()

	// Performance metrics available in variables for assertions

	// Should be able to handle at least 100 commands per second
	if commandsPerSecond < 100 {
		t.Errorf("Performance too low: %.2f commands/second", commandsPerSecond)
	}
}

// Test command system with real-world scenario
func TestCommandRealWorldScenario(t *testing.T) {
	mockEventManager := &MockEventManager{}
	
	// Create storage manager
	storageManager := storage.NewManager()

	// Create communication manager
	commManager := communication.NewCommunicationManager(storageManager, mockEventManager)

	// Create command manager
	cm := command.NewCommandManager(nil, mockEventManager)
	
	playerID := "test-player-1"

	// Register game commands
	cm.RegisterHandler("look", func(cmd *command.Command) error {
		// Simulate looking at the current room
		return nil
	})

	cm.RegisterHandler("move", func(cmd *command.Command) error {
		// Simulate moving to a new room
		commManager.UpdatePlayerLocation(playerID, "world1", "room2")
		return nil
	})

	cm.RegisterHandler("say", func(cmd *command.Command) error {
		// Simulate saying something
		location, exists := commManager.GetPlayerLocation(playerID)
		if !exists {
			return nil
		}
		
		commData := communication.CommunicationData{
			Message:    cmd.Argument,
			SenderID:   cmd.PlayerID,
			SenderName: "TestPlayer",
			RoomID:     location.RoomID,
			WorldID:    location.WorldID,
			Timestamp:  time.Now().Unix(),
		}
		
		msg := message.NewMessage(communication.MessageTypeSay).WithContent(commData).Build()
		
		return commManager.SendToRoom(location.WorldID, location.RoomID, msg)
	})

	// Set initial location
	commManager.UpdatePlayerLocation(playerID, "world1", "room1")

	// Simulate player session
	commands := []string{
		"look",
		"move north",
		"look",
		"say Hello everyone!",
		"look around",
	}

	for _, cmdText := range commands {
		err := cm.ProcessCommand(cmdText, playerID)
		if err != nil {
			t.Errorf("Unexpected error processing command %q: %v", cmdText, err)
		}

		err = cm.ProcessQueue(playerID)
		if err != nil {
			t.Errorf("Unexpected error executing command %q: %v", cmdText, err)
		}
	}

	// Verify final location
	location, exists := commManager.GetPlayerLocation(playerID)
	if !exists {
		t.Error("Player location should exist")
	}

	if location.RoomID != "room2" {
		t.Errorf("Expected final room 'room2', got %s", location.RoomID)
	}

	// Verify events were triggered
	events := mockEventManager.GetEvents()
	if len(events) == 0 {
		t.Error("Expected events to be triggered")
	}
}