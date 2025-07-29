package command

import (
	"strings"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/command"
)

// Test command rejection scenarios
func TestCommandRejectionScenarios(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Fill the queue to capacity
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing first command: %v", err)
	}

	err = cm.ProcessCommand("test2", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing second command: %v", err)
	}

	// Verify queue is full
	if !cm.IsQueueFull(playerID) {
		t.Error("Queue should be full after adding 2 commands")
	}

	// Try to add a third command - should be rejected
	err = cm.ProcessCommand("test3", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Queue size should remain the same
	if cm.GetQueueSize(playerID) != 2 {
		t.Errorf("Expected queue size 2, got %d", cm.GetQueueSize(playerID))
	}

	// Multiple rejection attempts
	for i := 0; i < 5; i++ {
		err = cm.ProcessCommand("overflow", playerID)
		if err != command.ErrQueueFull {
			t.Errorf("Expected ErrQueueFull on attempt %d, got %v", i, err)
		}
	}

	// Queue should still be full with original commands
	if cm.GetQueueSize(playerID) != 2 {
		t.Errorf("Expected queue size 2 after multiple rejections, got %d", cm.GetQueueSize(playerID))
	}
}

// Test overflow handling with different queue sizes
func TestCommandOverflowDifferentSizes(t *testing.T) {
	mockEventManager := &MockEventManager{}
	
	testCases := []struct {
		name      string
		queueSize int
	}{
		{"Single command queue", 1},
		{"Small queue", 3},
		{"Medium queue", 10},
		{"Large queue", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &command.CommandConfig{
				QueueSize: tc.queueSize,
				Enabled:   true,
			}
			cm := command.NewCommandManager(config, mockEventManager)
			playerID := "test-player-1"

			// Fill queue to capacity
			for i := 0; i < tc.queueSize; i++ {
				err := cm.ProcessCommand("test", playerID)
				if err != nil {
					t.Errorf("Unexpected error filling queue at position %d: %v", i, err)
				}
			}

			// Verify queue is full
			if !cm.IsQueueFull(playerID) {
				t.Error("Queue should be full")
			}

			// Try to add one more command
			err := cm.ProcessCommand("overflow", playerID)
			if err != command.ErrQueueFull {
				t.Errorf("Expected ErrQueueFull, got %v", err)
			}

			// Queue size should remain at capacity
			if cm.GetQueueSize(playerID) != tc.queueSize {
				t.Errorf("Expected queue size %d, got %d", tc.queueSize, cm.GetQueueSize(playerID))
			}
		})
	}
}

// Test rejection events and monitoring
func TestCommandRejectionEvents(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 1,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Fill the queue
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	// Clear previous events
	mockEventManager.ClearEvents()

	// Try to add commands that will be rejected
	rejectedCommands := []string{"rejected1", "rejected2", "rejected3"}
	for _, cmdName := range rejectedCommands {
		err = cm.ProcessCommand(cmdName, playerID)
		if err != command.ErrQueueFull {
			t.Errorf("Expected ErrQueueFull for command %s, got %v", cmdName, err)
		}
	}

	// Check that rejection events were triggered
	events := mockEventManager.GetEvents()
	if len(events) != len(rejectedCommands) {
		t.Errorf("Expected %d rejection events, got %d", len(rejectedCommands), len(events))
	}

	// Verify each event
	for i, event := range events {
		if event.Data()["error"] != "command_queue_full" {
			t.Errorf("Expected event %d to be 'command_queue_full', got %v", i, event.Data()["error"])
		}
		if event.Data()["player_id"] != playerID {
			t.Errorf("Expected event %d player_id to be %s, got %v", i, playerID, event.Data()["player_id"])
		}
	}
}

// Test rejection with different players
func TestCommandRejectionMultiplePlayers(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	
	player1 := "player1"
	player2 := "player2"

	// Fill player1's queue
	err := cm.ProcessCommand("cmd1", player1)
	if err != nil {
		t.Errorf("Unexpected error for player1: %v", err)
	}
	err = cm.ProcessCommand("cmd2", player1)
	if err != nil {
		t.Errorf("Unexpected error for player1: %v", err)
	}

	// Player1's queue should be full
	if !cm.IsQueueFull(player1) {
		t.Error("Player1's queue should be full")
	}

	// Player2's queue should be empty
	if cm.IsQueueFull(player2) {
		t.Error("Player2's queue should not be full")
	}

	// Player1 should be rejected
	err = cm.ProcessCommand("overflow", player1)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull for player1, got %v", err)
	}

	// Player2 should be accepted
	err = cm.ProcessCommand("accepted", player2)
	if err != nil {
		t.Errorf("Unexpected error for player2: %v", err)
	}

	// Verify final queue states
	if cm.GetQueueSize(player1) != 2 {
		t.Errorf("Expected player1 queue size 2, got %d", cm.GetQueueSize(player1))
	}
	if cm.GetQueueSize(player2) != 1 {
		t.Errorf("Expected player2 queue size 1, got %d", cm.GetQueueSize(player2))
	}
}

// Test rejection recovery scenarios
func TestCommandRejectionRecovery(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"
	
	// Register handler for test commands
	cm.RegisterHandler("test1", func(cmd *command.Command) error {
		return nil
	})
	cm.RegisterHandler("test2", func(cmd *command.Command) error {
		return nil
	})
	cm.RegisterHandler("rejected", func(cmd *command.Command) error {
		return nil
	})
	cm.RegisterHandler("accepted", func(cmd *command.Command) error {
		return nil
	})

	// Fill queue to capacity
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	err = cm.ProcessCommand("test2", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify queue is full
	if !cm.IsQueueFull(playerID) {
		t.Error("Queue should be full")
	}

	// Try to add command - should be rejected
	err = cm.ProcessCommand("rejected", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Process one command to make space
	err = cm.ProcessQueue(playerID)
	if err != nil {
		t.Errorf("Unexpected error processing queue: %v", err)
	}

	// Queue should no longer be full
	if cm.IsQueueFull(playerID) {
		t.Error("Queue should not be full after processing")
	}

	// Should be able to add command now
	err = cm.ProcessCommand("accepted", playerID)
	if err != nil {
		t.Errorf("Unexpected error after recovery: %v", err)
	}

	// Queue should be full again
	if !cm.IsQueueFull(playerID) {
		t.Error("Queue should be full after adding new command")
	}
}

// Test rejection with command manager disabled
func TestCommandRejectionDisabled(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   false,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// All commands should be rejected when disabled
	err := cm.ProcessCommand("test", playerID)
	if err == nil {
		t.Error("Expected error when command manager is disabled")
	}

	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Expected error message to contain 'disabled', got: %v", err)
	}

	// Queue should remain empty
	if cm.GetQueueSize(playerID) != 0 {
		t.Errorf("Expected empty queue, got size %d", cm.GetQueueSize(playerID))
	}
}

// Test rejection with invalid commands
func TestCommandRejectionInvalidCommands(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	invalidCommands := []string{"", "   ", "\t", "\n"}
	
	for _, invalidCmd := range invalidCommands {
		err := cm.ProcessCommand(invalidCmd, playerID)
		if err != command.ErrInvalidCommand {
			t.Errorf("Expected ErrInvalidCommand for input %q, got %v", invalidCmd, err)
		}
	}

	// Queue should remain empty
	if cm.GetQueueSize(playerID) != 0 {
		t.Errorf("Expected empty queue after invalid commands, got size %d", cm.GetQueueSize(playerID))
	}
}

// Test concurrent rejection scenarios
func TestCommandRejectionConcurrent(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 1,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Fill the queue
	err := cm.ProcessCommand("fill", playerID)
	if err != nil {
		t.Errorf("Unexpected error filling queue: %v", err)
	}

	// Start multiple goroutines trying to add commands
	numWorkers := 10
	errChan := make(chan error, numWorkers)
	
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			err := cm.ProcessCommand("concurrent", playerID)
			errChan <- err
		}(i)
	}

	// Collect all errors
	rejectionCount := 0
	for i := 0; i < numWorkers; i++ {
		err := <-errChan
		if err == command.ErrQueueFull {
			rejectionCount++
		} else if err != nil {
			t.Errorf("Unexpected error from worker: %v", err)
		}
	}

	// All concurrent attempts should have been rejected
	if rejectionCount != numWorkers {
		t.Errorf("Expected %d rejections, got %d", numWorkers, rejectionCount)
	}

	// Queue should still have only 1 command
	if cm.GetQueueSize(playerID) != 1 {
		t.Errorf("Expected queue size 1, got %d", cm.GetQueueSize(playerID))
	}
}

// Test rejection with queue management operations
func TestCommandRejectionWithQueueManagement(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Fill queue
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	err = cm.ProcessCommand("test2", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should be rejected
	err = cm.ProcessCommand("rejected1", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Clear queue
	cm.ClearQueue(playerID)

	// Should be accepted now
	err = cm.ProcessCommand("accepted", playerID)
	if err != nil {
		t.Errorf("Unexpected error after clear: %v", err)
	}

	// Fill queue again
	err = cm.ProcessCommand("test3", playerID)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should be rejected again
	err = cm.ProcessCommand("rejected2", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull after refilling, got %v", err)
	}

	// Remove player
	cm.RemovePlayer(playerID)

	// Should be accepted (creates new queue)
	err = cm.ProcessCommand("new_queue", playerID)
	if err != nil {
		t.Errorf("Unexpected error after removing player: %v", err)
	}
}

// Test overflow with zero-sized queue
func TestCommandRejectionZeroQueue(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 0,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Any command should be rejected with zero-sized queue
	err := cm.ProcessCommand("test", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull with zero-sized queue, got %v", err)
	}

	// Queue should remain empty
	if cm.GetQueueSize(playerID) != 0 {
		t.Errorf("Expected empty queue, got size %d", cm.GetQueueSize(playerID))
	}
}