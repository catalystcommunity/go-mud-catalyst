package command

import (
	"errors"
	"strings"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/command"
	"github.com/catalystcommunity/muddycore/pkg/message"
)

// Test error handling in command handlers
func TestCommandHandlerErrors(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Register handlers that return errors
	cm.RegisterHandler("fail", func(cmd *command.Command) error {
		return errors.New("command failed")
	})

	cm.RegisterHandler("panic", func(cmd *command.Command) error {
		panic("command panicked")
	})

	// Test command that returns error
	err := cm.ProcessCommand("fail", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	err = cm.ProcessQueue(playerID)
	if err == nil {
		t.Error("Expected error from failed command handler")
	}

	if err.Error() != "command failed" {
		t.Errorf("Expected 'command failed', got %q", err.Error())
	}

	// Test command that panics (should be caught and handled)
	err = cm.ProcessCommand("panic", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing panic command: %v", err)
	}

	// This should panic - if it doesn't, the test environment is different
	defer func() {
		if r := recover(); r != nil {
			// Expected panic was caught
			return
		}
		t.Error("Expected panic from command handler")
	}()

	err = cm.ProcessQueue(playerID)
	t.Error("Should not reach this point due to panic")
}

// Test error events
func TestCommandHandlerErrorEvents(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Register error handler
	cm.RegisterHandler("error", func(cmd *command.Command) error {
		return errors.New("test error")
	})

	// Clear events
	mockEventManager.ClearEvents()

	// Process command
	err := cm.ProcessCommand("error", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	err = cm.ProcessQueue(playerID)
	if err == nil {
		t.Error("Expected error from command handler")
	}

	// Check events
	events := mockEventManager.GetEvents()
	foundExecuting := false
	foundExecuted := false
	foundError := false

	for _, event := range events {
		if event.Data()["event"] == "command_executing" {
			foundExecuting = true
		}
		if event.Data()["event"] == "command_executed" {
			foundExecuted = true
			if event.Data()["success"] != false {
				t.Error("Expected success to be false for failed command")
			}
			if event.Data()["error"] != "test error" {
				t.Errorf("Expected error 'test error', got %v", event.Data()["error"])
			}
		}
	}

	if !foundExecuting {
		t.Error("Expected to find command_executing event")
	}
	if !foundExecuted {
		t.Error("Expected to find command_executed event")
	}
	if foundError {
		t.Error("Should not find separate error event")
	}
}

// Test command not found error
func TestCommandNotFoundError(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Clear events
	mockEventManager.ClearEvents()

	// Process unknown command
	err := cm.ProcessCommand("unknown", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing unknown command: %v", err)
	}

	err = cm.ProcessQueue(playerID)
	if err != command.ErrCommandNotFound {
		t.Errorf("Expected ErrCommandNotFound, got %v", err)
	}

	// Check events
	events := mockEventManager.GetEvents()
	foundError := false

	for _, event := range events {
		if event.Data()["error"] == "command_not_found" {
			foundError = true
			if event.Data()["command"] != "unknown" {
				t.Errorf("Expected command 'unknown', got %v", event.Data()["command"])
			}
		}
	}

	if !foundError {
		t.Error("Expected to find command_not_found error event")
	}
}

// Test message creation and parsing errors
func TestCommandMessageErrors(t *testing.T) {
	// Test parsing non-command message
	nonCommandMsg := &message.Message{
		Type:     message.MessageTypeChat,
		Contents: []byte("test"),
	}

	_, err := command.ParseCommandMessage(nonCommandMsg)
	if err == nil {
		t.Error("Expected error parsing non-command message")
	}

	if !strings.Contains(err.Error(), "not a command") {
		t.Errorf("Expected error message about 'not a command', got: %v", err)
	}

	// Test parsing message with invalid CBOR
	invalidMsg := &message.Message{
		Type:     command.MessageTypeCommand,
		Contents: []byte("invalid cbor data"),
	}

	_, err = command.ParseCommandMessage(invalidMsg)
	if err == nil {
		t.Error("Expected error parsing invalid CBOR message")
	}

	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("Expected error message about 'unmarshal', got: %v", err)
	}
}

// Test edge cases with nil values
func TestCommandNilValues(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)

	// Test with nil event manager
	cm2 := command.NewCommandManager(nil, nil)
	err := cm2.ProcessCommand("test", "player1")
	if err != nil {
		t.Errorf("Unexpected error with nil event manager: %v", err)
	}

	// Test with empty player ID
	_, err = cm.ParseCommand("test", "")
	if err != nil {
		t.Errorf("Unexpected error with empty player ID: %v", err)
	}

	// Test queue operations with non-existent player
	size := cm.GetQueueSize("non-existent")
	if size != 0 {
		t.Errorf("Expected queue size 0 for non-existent player, got %d", size)
	}

	isFull := cm.IsQueueFull("non-existent")
	if isFull {
		t.Error("Non-existent player queue should not be full")
	}

	queue := cm.GetQueue("non-existent")
	if queue != nil {
		t.Error("Expected nil queue for non-existent player")
	}
}

// Test command parsing error edge cases
func TestCommandParsingErrorEdgeCases(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Test extremely long command
	longCmd := strings.Repeat("a", 100000)
	cmd, err := cm.ParseCommand(longCmd, playerID)
	if err != nil {
		t.Errorf("Unexpected error with long command: %v", err)
	}
	if cmd.Name != longCmd {
		t.Error("Long command name should be preserved")
	}

	// Test command with null bytes
	nullCmd := "test\x00null"
	cmd, err = cm.ParseCommand(nullCmd, playerID)
	if err != nil {
		t.Errorf("Unexpected error with null bytes: %v", err)
	}
	if cmd.Name != "test\x00null" {
		t.Errorf("Expected command name 'test\\x00null', got %q", cmd.Name)
	}

	// Test command with unicode
	unicodeCmd := "测试 unicode argument"
	cmd, err = cm.ParseCommand(unicodeCmd, playerID)
	if err != nil {
		t.Errorf("Unexpected error with unicode: %v", err)
	}
	if cmd.Name != "测试" {
		t.Errorf("Expected command name '测试', got %q", cmd.Name)
	}
	if cmd.Argument != "unicode argument" {
		t.Errorf("Expected argument 'unicode argument', got %q", cmd.Argument)
	}

	// Test command with only punctuation
	punctCmd := "!@#$%^&*()"
	cmd, err = cm.ParseCommand(punctCmd, playerID)
	if err != nil {
		t.Errorf("Unexpected error with punctuation: %v", err)
	}
	if cmd.Name != punctCmd {
		t.Errorf("Expected command name %q, got %q", punctCmd, cmd.Name)
	}
}

// Test concurrent error scenarios
func TestCommandConcurrentErrors(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Register handler that sometimes fails
	counter := 0
	cm.RegisterHandler("flaky", func(cmd *command.Command) error {
		counter++
		if counter%2 == 0 {
			return errors.New("flaky error")
		}
		return nil
	})

	// Process multiple commands concurrently - some may be rejected due to queue limits
	numCommands := 10
	errChan := make(chan error, numCommands)

	for i := 0; i < numCommands; i++ {
		go func() {
			err := cm.ProcessCommand("flaky", playerID)
			errChan <- err
		}()
	}

	// Collect processing errors (some may be queue full errors)
	queueFullErrors := 0
	for i := 0; i < numCommands; i++ {
		err := <-errChan
		if err != nil {
			if err == command.ErrQueueFull {
				queueFullErrors++
			} else {
				t.Errorf("Unexpected error type processing command %d: %v", i, err)
			}
		}
	}

	// Execute all commands
	successCount := 0
	errorCount := 0
	for cm.GetQueueSize(playerID) > 0 {
		err := cm.ProcessQueue(playerID)
		if err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	// Should have some successes and some errors
	if successCount == 0 {
		t.Error("Expected some successful commands")
	}
	if errorCount == 0 {
		t.Error("Expected some failed commands")
	}
}

// Test handler registration edge cases
func TestCommandHandlerRegistrationEdgeCases(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)

	// Test registering handler with empty name
	cm.RegisterHandler("", func(cmd *command.Command) error {
		return nil
	})

	// Test registering nil handler
	cm.RegisterHandler("test", nil)

	// Test executing command with nil handler
	err := cm.ProcessCommand("test", "player1")
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	// This should panic due to nil handler
	defer func() {
		if r := recover(); r != nil {
			// Expected panic
			return
		}
		t.Error("Expected panic from nil handler")
	}()

	err = cm.ProcessQueue("player1")
	t.Error("Should not reach this point due to panic")
}

// Test command unregistration
func TestCommandHandlerUnregistration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Register handler
	executed := false
	cm.RegisterHandler("test", func(cmd *command.Command) error {
		executed = true
		return nil
	})

	// Test handler works
	err := cm.ProcessCommand("test", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = cm.ProcessQueue(playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !executed {
		t.Error("Handler should have been executed")
	}

	// Unregister handler
	cm.UnregisterHandler("test")

	// Test handler no longer works
	executed = false
	err = cm.ProcessCommand("test", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = cm.ProcessQueue(playerID)
	if err != command.ErrCommandNotFound {
		t.Errorf("Expected ErrCommandNotFound, got %v", err)
	}

	if executed {
		t.Error("Handler should not have been executed after unregistration")
	}
}

// Test memory leaks and resource cleanup
func TestCommandMemoryLeaks(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)

	// Create many players and commands
	numPlayers := 1000
	for i := 0; i < numPlayers; i++ {
		playerID := "player" + string(rune(i))
		
		// Add commands
		err := cm.ProcessCommand("test", playerID)
		if err != nil {
			t.Errorf("Unexpected error for player %d: %v", i, err)
		}

		// Remove player immediately
		cm.RemovePlayer(playerID)
	}

	// Check that queues were cleaned up
	for i := 0; i < numPlayers; i++ {
		playerID := "player" + string(rune(i))
		if cm.GetQueueSize(playerID) != 0 {
			t.Errorf("Expected empty queue for removed player %s", playerID)
		}
	}
}

// Test timestamp precision and ordering
func TestCommandTimestampPrecision(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Create commands rapidly
	var commands []*command.Command
	for i := 0; i < 1000; i++ {
		cmd, err := cm.ParseCommand("test", playerID)
		if err != nil {
			t.Errorf("Unexpected error creating command %d: %v", i, err)
		}
		commands = append(commands, cmd)
	}

	// Check that timestamps are unique and ordered
	timestamps := make(map[int64]bool)
	for i, cmd := range commands {
		timestamp := cmd.Timestamp.UnixNano()
		
		if timestamps[timestamp] {
			t.Errorf("Duplicate timestamp found at command %d", i)
		}
		timestamps[timestamp] = true
		
		if i > 0 && cmd.Timestamp.Before(commands[i-1].Timestamp) {
			t.Errorf("Timestamp ordering violated at command %d", i)
		}
	}
}

// Test command system with extreme configurations
func TestCommandExtremeConfigurations(t *testing.T) {
	mockEventManager := &MockEventManager{}
	
	// Test with very large queue size
	config := &command.CommandConfig{
		QueueSize: 1000000,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Should be able to handle large queue
	err := cm.ProcessCommand("test", playerID)
	if err != nil {
		t.Errorf("Unexpected error with large queue: %v", err)
	}

	// Test with zero queue size
	config = &command.CommandConfig{
		QueueSize: 0,
		Enabled:   true,
	}
	cm = command.NewCommandManager(config, mockEventManager)

	// Should immediately reject commands
	err = cm.ProcessCommand("test", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull with zero queue, got %v", err)
	}

	// Test with negative queue size (should be handled gracefully)
	config = &command.CommandConfig{
		QueueSize: -1,
		Enabled:   true,
	}
	cm = command.NewCommandManager(config, mockEventManager)

	// Should handle negative size gracefully
	err = cm.ProcessCommand("test", playerID)
	if err == nil {
		t.Error("Expected error with negative queue size")
	}
}