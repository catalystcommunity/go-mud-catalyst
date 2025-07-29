package command

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/fxamacker/cbor/v2"
)

// Message type for commands
const (
	MessageTypeCommand = message.MessageTypeCustom + 100
)

var (
	ErrQueueFull     = errors.New("command queue is full")
	ErrInvalidCommand = errors.New("invalid command format")
	ErrCommandNotFound = errors.New("command not found")
)

// Command represents a parsed command
type Command struct {
	ID        string    `cbor:"id"`
	Name      string    `cbor:"name"`
	Argument  string    `cbor:"argument"`
	PlayerID  string    `cbor:"player_id"`
	Timestamp time.Time `cbor:"timestamp"`
	Raw       string    `cbor:"raw"`
}

// CommandHandler is a function that handles a command
type CommandHandler func(cmd *Command) error

// CommandConfig contains configuration for the command system
type CommandConfig struct {
	QueueSize int  // Maximum number of commands in queue per player
	Enabled   bool // Whether command processing is enabled
}

// DefaultCommandConfig returns default configuration
func DefaultCommandConfig() *CommandConfig {
	return &CommandConfig{
		QueueSize: 2,
		Enabled:   true,
	}
}

// CommandQueue manages commands for a single player
type CommandQueue struct {
	PlayerID string
	Commands []*Command
	MaxSize  int
	mutex    sync.RWMutex
}

// NewCommandQueue creates a new command queue for a player
func NewCommandQueue(playerID string, maxSize int) *CommandQueue {
	if maxSize < 0 {
		maxSize = 0
	}
	return &CommandQueue{
		PlayerID: playerID,
		Commands: make([]*Command, 0, maxSize),
		MaxSize:  maxSize,
	}
}

// Enqueue adds a command to the queue
func (q *CommandQueue) Enqueue(cmd *Command) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if len(q.Commands) >= q.MaxSize {
		return ErrQueueFull
	}
	
	q.Commands = append(q.Commands, cmd)
	return nil
}

// Dequeue removes and returns the next command from the queue
func (q *CommandQueue) Dequeue() *Command {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	
	if len(q.Commands) == 0 {
		return nil
	}
	
	cmd := q.Commands[0]
	q.Commands = q.Commands[1:]
	return cmd
}

// Size returns the current number of commands in the queue
func (q *CommandQueue) Size() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.Commands)
}

// IsFull returns true if the queue is at capacity
func (q *CommandQueue) IsFull() bool {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return len(q.Commands) >= q.MaxSize
}

// Clear removes all commands from the queue
func (q *CommandQueue) Clear() {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.Commands = q.Commands[:0]
}

// CommandManager handles command parsing, queueing, and processing
type CommandManager struct {
	config       *CommandConfig
	handlers     map[string]CommandHandler
	queues       map[string]*CommandQueue
	eventManager events.EventManager
	
	handlerMutex sync.RWMutex
	queueMutex   sync.RWMutex
}

// NewCommandManager creates a new command manager
func NewCommandManager(config *CommandConfig, eventManager events.EventManager) *CommandManager {
	if config == nil {
		config = DefaultCommandConfig()
	}
	
	return &CommandManager{
		config:       config,
		handlers:     make(map[string]CommandHandler),
		queues:       make(map[string]*CommandQueue),
		eventManager: eventManager,
	}
}

// ParseCommand parses a command string into a Command struct
func (cm *CommandManager) ParseCommand(input string, playerID string) (*Command, error) {
	if !cm.config.Enabled {
		return nil, errors.New("command processing is disabled")
	}
	
	// Trim whitespace
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrInvalidCommand
	}
	
	// Split on first whitespace to get command and argument
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil, ErrInvalidCommand
	}
	
	commandName := strings.ToLower(parts[0])
	argument := ""
	
	if len(parts) > 1 {
		// Reconstruct argument from remaining parts
		argument = strings.Join(parts[1:], " ")
	}
	
	cmd := &Command{
		ID:        ids.NewEntityID(),
		Name:      commandName,
		Argument:  argument,
		PlayerID:  playerID,
		Timestamp: time.Now(),
		Raw:       input,
	}
	
	return cmd, nil
}

// QueueCommand adds a command to the player's queue
func (cm *CommandManager) QueueCommand(cmd *Command) error {
	if !cm.config.Enabled {
		return errors.New("command processing is disabled")
	}
	
	cm.queueMutex.Lock()
	queue, exists := cm.queues[cmd.PlayerID]
	if !exists {
		queue = NewCommandQueue(cmd.PlayerID, cm.config.QueueSize)
		cm.queues[cmd.PlayerID] = queue
	}
	cm.queueMutex.Unlock()
	
	// Try to enqueue the command
	err := queue.Enqueue(cmd)
	if err != nil {
		// Trigger event for queue full
		if cm.eventManager != nil {
			event := cm.eventManager.CreateEvent(events.EventTypeSystemError, cm, map[string]interface{}{
				"error":     "command_queue_full",
				"player_id": cmd.PlayerID,
				"command":   cmd.Name,
				"queue_size": queue.Size(),
			})
			cm.eventManager.TriggerEvent(event)
		}
		return err
	}
	
	// Trigger event for command queued
	if cm.eventManager != nil {
		event := cm.eventManager.CreateEvent(events.EventTypePlayerAction, cm, map[string]interface{}{
			"event":     "command_queued",
			"player_id": cmd.PlayerID,
			"command":   cmd.Name,
			"queue_size": queue.Size(),
		})
		cm.eventManager.TriggerEvent(event)
	}
	
	return nil
}

// ProcessCommand processes a command string for a player
func (cm *CommandManager) ProcessCommand(input string, playerID string) error {
	// Parse the command
	cmd, err := cm.ParseCommand(input, playerID)
	if err != nil {
		return err
	}
	
	// Queue the command
	return cm.QueueCommand(cmd)
}

// ProcessQueue processes the next command in a player's queue
func (cm *CommandManager) ProcessQueue(playerID string) error {
	cm.queueMutex.RLock()
	queue, exists := cm.queues[playerID]
	cm.queueMutex.RUnlock()
	
	if !exists {
		return nil // No queue for this player
	}
	
	// Get the next command
	cmd := queue.Dequeue()
	if cmd == nil {
		return nil // No commands to process
	}
	
	// Execute the command
	return cm.ExecuteCommand(cmd)
}

// ExecuteCommand executes a command by calling its handler
func (cm *CommandManager) ExecuteCommand(cmd *Command) error {
	cm.handlerMutex.RLock()
	handler, exists := cm.handlers[cmd.Name]
	cm.handlerMutex.RUnlock()
	
	if !exists {
		// Trigger event for command not found
		if cm.eventManager != nil {
			event := cm.eventManager.CreateEvent(events.EventTypeSystemError, cm, map[string]interface{}{
				"error":     "command_not_found",
				"player_id": cmd.PlayerID,
				"command":   cmd.Name,
			})
			cm.eventManager.TriggerEvent(event)
		}
		return ErrCommandNotFound
	}
	
	// Trigger event for command execution start
	if cm.eventManager != nil {
		event := cm.eventManager.CreateEvent(events.EventTypePlayerAction, cm, map[string]interface{}{
			"event":     "command_executing",
			"player_id": cmd.PlayerID,
			"command":   cmd.Name,
		})
		cm.eventManager.TriggerEvent(event)
	}
	
	// Execute the command
	err := handler(cmd)
	
	// Trigger event for command execution result
	if cm.eventManager != nil {
		eventData := map[string]interface{}{
			"event":     "command_executed",
			"player_id": cmd.PlayerID,
			"command":   cmd.Name,
			"success":   err == nil,
		}
		if err != nil {
			eventData["error"] = err.Error()
		}
		event := cm.eventManager.CreateEvent(events.EventTypePlayerAction, cm, eventData)
		cm.eventManager.TriggerEvent(event)
	}
	
	return err
}

// RegisterHandler registers a command handler
func (cm *CommandManager) RegisterHandler(commandName string, handler CommandHandler) {
	cm.handlerMutex.Lock()
	defer cm.handlerMutex.Unlock()
	
	cm.handlers[strings.ToLower(commandName)] = handler
}

// UnregisterHandler removes a command handler
func (cm *CommandManager) UnregisterHandler(commandName string) {
	cm.handlerMutex.Lock()
	defer cm.handlerMutex.Unlock()
	
	delete(cm.handlers, strings.ToLower(commandName))
}

// GetQueue returns the command queue for a player
func (cm *CommandManager) GetQueue(playerID string) *CommandQueue {
	cm.queueMutex.RLock()
	defer cm.queueMutex.RUnlock()
	
	return cm.queues[playerID]
}

// GetQueueSize returns the current size of a player's command queue
func (cm *CommandManager) GetQueueSize(playerID string) int {
	cm.queueMutex.RLock()
	queue, exists := cm.queues[playerID]
	cm.queueMutex.RUnlock()
	
	if !exists {
		return 0
	}
	
	return queue.Size()
}

// IsQueueFull returns true if a player's command queue is full
func (cm *CommandManager) IsQueueFull(playerID string) bool {
	cm.queueMutex.RLock()
	queue, exists := cm.queues[playerID]
	cm.queueMutex.RUnlock()
	
	if !exists {
		return false
	}
	
	return queue.IsFull()
}

// ClearQueue clears all commands from a player's queue
func (cm *CommandManager) ClearQueue(playerID string) {
	cm.queueMutex.RLock()
	queue, exists := cm.queues[playerID]
	cm.queueMutex.RUnlock()
	
	if exists {
		queue.Clear()
	}
}

// RemovePlayer removes a player's command queue
func (cm *CommandManager) RemovePlayer(playerID string) {
	cm.queueMutex.Lock()
	defer cm.queueMutex.Unlock()
	
	delete(cm.queues, playerID)
}

// GetRegisteredCommands returns a list of all registered command names
func (cm *CommandManager) GetRegisteredCommands() []string {
	cm.handlerMutex.RLock()
	defer cm.handlerMutex.RUnlock()
	
	commands := make([]string, 0, len(cm.handlers))
	for cmd := range cm.handlers {
		commands = append(commands, cmd)
	}
	return commands
}

// SetEnabled enables or disables command processing
func (cm *CommandManager) SetEnabled(enabled bool) {
	cm.config.Enabled = enabled
}

// IsEnabled returns true if command processing is enabled
func (cm *CommandManager) IsEnabled() bool {
	return cm.config.Enabled
}

// CreateCommandMessage creates a message containing a command
func CreateCommandMessage(cmd *Command) (*message.Message, error) {
	contents, err := cbor.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	
	return &message.Message{
		Type:        MessageTypeCommand,
		Contents:    contents,
		ID:          cmd.ID,
		RequiresAck: true,
		AckTimeout:  5 * time.Second,
	}, nil
}

// ParseCommandMessage parses a command from a message
func ParseCommandMessage(msg *message.Message) (*Command, error) {
	if msg.Type != MessageTypeCommand {
		return nil, errors.New("message is not a command")
	}
	
	var cmd Command
	err := cbor.Unmarshal(msg.Contents, &cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal command: %w", err)
	}
	
	return &cmd, nil
}