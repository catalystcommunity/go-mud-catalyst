package command

import (
	"sync"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/command"
)

// Test basic queue functionality
func TestCommandQueueBasic(t *testing.T) {
	playerID := "test-player-1"
	maxSize := 3
	queue := command.NewCommandQueue(playerID, maxSize)

	if queue.Size() != 0 {
		t.Errorf("Expected empty queue, got size %d", queue.Size())
	}

	if queue.IsFull() {
		t.Error("Empty queue should not be full")
	}

	// Test enqueue
	cmd1 := &command.Command{
		Name:     "test1",
		PlayerID: playerID,
	}
	
	err := queue.Enqueue(cmd1)
	if err != nil {
		t.Errorf("Unexpected error enqueuing: %v", err)
	}

	if queue.Size() != 1 {
		t.Errorf("Expected queue size 1, got %d", queue.Size())
	}

	// Test dequeue
	dequeuedCmd := queue.Dequeue()
	if dequeuedCmd == nil {
		t.Error("Expected command, got nil")
	}

	if dequeuedCmd.Name != cmd1.Name {
		t.Errorf("Expected command name %q, got %q", cmd1.Name, dequeuedCmd.Name)
	}

	if queue.Size() != 0 {
		t.Errorf("Expected empty queue after dequeue, got size %d", queue.Size())
	}

	// Test dequeue from empty queue
	emptyCmd := queue.Dequeue()
	if emptyCmd != nil {
		t.Error("Expected nil from empty queue dequeue")
	}
}

// Test queue capacity and overflow
func TestCommandQueueCapacity(t *testing.T) {
	playerID := "test-player-1"
	maxSize := 2
	queue := command.NewCommandQueue(playerID, maxSize)

	// Fill the queue
	for i := 0; i < maxSize; i++ {
		cmd := &command.Command{
			Name:     "test",
			PlayerID: playerID,
		}
		err := queue.Enqueue(cmd)
		if err != nil {
			t.Errorf("Unexpected error enqueuing command %d: %v", i, err)
		}
	}

	if !queue.IsFull() {
		t.Error("Queue should be full")
	}

	// Try to add one more command (should fail)
	overflowCmd := &command.Command{
		Name:     "overflow",
		PlayerID: playerID,
	}
	err := queue.Enqueue(overflowCmd)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Queue size should remain the same
	if queue.Size() != maxSize {
		t.Errorf("Expected queue size %d, got %d", maxSize, queue.Size())
	}
}

// Test queue FIFO behavior
func TestCommandQueueFIFO(t *testing.T) {
	playerID := "test-player-1"
	maxSize := 5
	queue := command.NewCommandQueue(playerID, maxSize)

	// Add commands in order
	commandNames := []string{"first", "second", "third", "fourth"}
	for _, name := range commandNames {
		cmd := &command.Command{
			Name:     name,
			PlayerID: playerID,
		}
		err := queue.Enqueue(cmd)
		if err != nil {
			t.Errorf("Unexpected error enqueuing %q: %v", name, err)
		}
	}

	// Dequeue commands and verify order
	for i, expectedName := range commandNames {
		cmd := queue.Dequeue()
		if cmd == nil {
			t.Errorf("Expected command at position %d, got nil", i)
			continue
		}
		if cmd.Name != expectedName {
			t.Errorf("Expected command %q at position %d, got %q", expectedName, i, cmd.Name)
		}
	}

	// Queue should be empty now
	if queue.Size() != 0 {
		t.Errorf("Expected empty queue, got size %d", queue.Size())
	}
}

// Test queue clear functionality
func TestCommandQueueClear(t *testing.T) {
	playerID := "test-player-1"
	maxSize := 3
	queue := command.NewCommandQueue(playerID, maxSize)

	// Add some commands
	for i := 0; i < maxSize; i++ {
		cmd := &command.Command{
			Name:     "test",
			PlayerID: playerID,
		}
		err := queue.Enqueue(cmd)
		if err != nil {
			t.Errorf("Unexpected error enqueuing command %d: %v", i, err)
		}
	}

	if queue.Size() != maxSize {
		t.Errorf("Expected queue size %d, got %d", maxSize, queue.Size())
	}

	// Clear the queue
	queue.Clear()

	if queue.Size() != 0 {
		t.Errorf("Expected empty queue after clear, got size %d", queue.Size())
	}

	if queue.IsFull() {
		t.Error("Queue should not be full after clear")
	}

	// Should be able to add commands again
	cmd := &command.Command{
		Name:     "after_clear",
		PlayerID: playerID,
	}
	err := queue.Enqueue(cmd)
	if err != nil {
		t.Errorf("Unexpected error enqueuing after clear: %v", err)
	}
}

// Test concurrent access to queue
func TestCommandQueueConcurrent(t *testing.T) {
	playerID := "test-player-1"
	maxSize := 100
	queue := command.NewCommandQueue(playerID, maxSize)

	var wg sync.WaitGroup
	numWorkers := 10
	commandsPerWorker := 10

	// Start enqueue workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < commandsPerWorker; j++ {
				cmd := &command.Command{
					Name:     "test",
					PlayerID: playerID,
				}
				queue.Enqueue(cmd)
			}
		}(i)
	}

	// Start dequeue workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < commandsPerWorker; j++ {
				for {
					cmd := queue.Dequeue()
					if cmd != nil {
						break
					}
					time.Sleep(time.Microsecond)
				}
			}
		}()
	}

	wg.Wait()

	// Queue should be empty after all workers finish
	if queue.Size() != 0 {
		t.Errorf("Expected empty queue after concurrent operations, got size %d", queue.Size())
	}
}

// Test command manager queue integration
func TestCommandManagerQueueIntegration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Test that queue is created automatically
	if cm.GetQueueSize(playerID) != 0 {
		t.Error("Queue should be empty initially")
	}

	// Test command processing (should create queue)
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	if cm.GetQueueSize(playerID) != 1 {
		t.Errorf("Expected queue size 1, got %d", cm.GetQueueSize(playerID))
	}

	// Add another command
	err = cm.ProcessCommand("test2", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing second command: %v", err)
	}

	if cm.GetQueueSize(playerID) != 2 {
		t.Errorf("Expected queue size 2, got %d", cm.GetQueueSize(playerID))
	}

	// Try to add a third command (should fail due to queue size limit)
	err = cm.ProcessCommand("test3", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Queue size should remain 2
	if cm.GetQueueSize(playerID) != 2 {
		t.Errorf("Expected queue size 2 after overflow, got %d", cm.GetQueueSize(playerID))
	}
}

// Test queue management operations
func TestCommandManagerQueueManagement(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Add some commands
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	err = cm.ProcessCommand("test2", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	// Test queue size check
	if cm.GetQueueSize(playerID) != 2 {
		t.Errorf("Expected queue size 2, got %d", cm.GetQueueSize(playerID))
	}

	// Test queue full check (should be full with default size 2)
	if !cm.IsQueueFull(playerID) {
		t.Error("Queue should be full after adding 2 commands to default size 2 queue")
	}

	// Test clear queue
	cm.ClearQueue(playerID)
	if cm.GetQueueSize(playerID) != 0 {
		t.Errorf("Expected empty queue after clear, got size %d", cm.GetQueueSize(playerID))
	}

	// Test remove player
	err = cm.ProcessCommand("test3", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	cm.RemovePlayer(playerID)
	if cm.GetQueueSize(playerID) != 0 {
		t.Errorf("Expected queue size 0 after removing player, got %d", cm.GetQueueSize(playerID))
	}
}

// Test multiple players with separate queues
func TestCommandManagerMultiplePlayersQueues(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	
	player1 := "player1"
	player2 := "player2"
	player3 := "player3"

	// Add commands for different players
	err := cm.ProcessCommand("cmd1", player1)
	if err != nil {
		t.Errorf("Unexpected error for player1: %v", err)
	}

	err = cm.ProcessCommand("cmd2", player2)
	if err != nil {
		t.Errorf("Unexpected error for player2: %v", err)
	}

	err = cm.ProcessCommand("cmd3", player1)
	if err != nil {
		t.Errorf("Unexpected error for player1: %v", err)
	}

	// Check queue sizes
	if cm.GetQueueSize(player1) != 2 {
		t.Errorf("Expected player1 queue size 2, got %d", cm.GetQueueSize(player1))
	}

	if cm.GetQueueSize(player2) != 1 {
		t.Errorf("Expected player2 queue size 1, got %d", cm.GetQueueSize(player2))
	}

	if cm.GetQueueSize(player3) != 0 {
		t.Errorf("Expected player3 queue size 0, got %d", cm.GetQueueSize(player3))
	}

	// Clear one player's queue
	cm.ClearQueue(player1)
	if cm.GetQueueSize(player1) != 0 {
		t.Errorf("Expected player1 queue size 0 after clear, got %d", cm.GetQueueSize(player1))
	}

	// Other players should be unaffected
	if cm.GetQueueSize(player2) != 1 {
		t.Errorf("Expected player2 queue size 1, got %d", cm.GetQueueSize(player2))
	}
}

// Test queue operations with disabled command manager
func TestCommandManagerQueueDisabled(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   false,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Should fail to process command when disabled
	err := cm.ProcessCommand("test", playerID)
	if err == nil {
		t.Error("Expected error when processing command with disabled manager")
	}

	// Queue should remain empty
	if cm.GetQueueSize(playerID) != 0 {
		t.Errorf("Expected empty queue, got size %d", cm.GetQueueSize(playerID))
	}
}

// Test queue events
func TestCommandManagerQueueEvents(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 1,
		Enabled:   true,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	// Add a command (should trigger queue event)
	err := cm.ProcessCommand("test1", playerID)
	if err != nil {
		t.Errorf("Unexpected error processing command: %v", err)
	}

	// Try to add another command (should trigger queue full event)
	err = cm.ProcessCommand("test2", playerID)
	if err != command.ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got %v", err)
	}

	// Check events
	events := mockEventManager.GetEvents()
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// First event should be command queued
	if events[0].Data()["event"] != "command_queued" {
		t.Errorf("Expected first event to be 'command_queued', got %v", events[0].Data()["event"])
	}

	// Second event should be queue full error
	if events[1].Data()["error"] != "command_queue_full" {
		t.Errorf("Expected second event to be 'command_queue_full', got %v", events[1].Data()["error"])
	}
}