package command

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/command"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
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

// MockEventManager for testing
type MockEventManager struct {
	mu     sync.RWMutex
	events []events.Event
}

func (m *MockEventManager) CreateEvent(eventType events.EventType, source interface{}, data map[string]interface{}) events.Event {
	return events.NewEvent(eventType, source, data)
}

func (m *MockEventManager) TriggerEvent(event events.Event) events.EventResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return events.EventResultContinue
}

func (m *MockEventManager) GetEvents() []events.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]events.Event, len(m.events))
	copy(result, m.events)
	return result
}

func (m *MockEventManager) ClearEvents() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}

// Implement required EventManager interface methods
func (m *MockEventManager) RegisterHook(eventType events.EventType, hook events.EventHook, priority int) string {
	return "mock-hook-id"
}

func (m *MockEventManager) RegisterHookOnce(eventType events.EventType, hook events.EventHook, priority int) string {
	return "mock-hook-id"
}

func (m *MockEventManager) RegisterHookWithID(id string, eventType events.EventType, hook events.EventHook, priority int) error {
	return nil
}

func (m *MockEventManager) UnregisterHook(id string) bool {
	return true
}

func (m *MockEventManager) UnregisterAllHooks(eventType events.EventType) int {
	return 0
}

func (m *MockEventManager) TriggerEventSync(event events.Event) events.EventResult {
	return m.TriggerEvent(event)
}

func (m *MockEventManager) TriggerEventAsync(event events.Event) {
	m.TriggerEvent(event)
}

func (m *MockEventManager) ListHooks(eventType events.EventType) []*events.EventHookRegistration {
	return nil
}

func (m *MockEventManager) GetHookCount(eventType events.EventType) int {
	return 0
}

func (m *MockEventManager) Close() error {
	return nil
}

// Test command parsing functionality
func TestCommandParsing(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedCmd string
		expectedArg string
	}{
		{
			name:        "Simple command",
			input:       "look",
			expectError: false,
			expectedCmd: "look",
			expectedArg: "",
		},
		{
			name:        "Command with argument",
			input:       "say hello world",
			expectError: false,
			expectedCmd: "say",
			expectedArg: "hello world",
		},
		{
			name:        "Command with extra spaces",
			input:       "  move   north  ",
			expectError: false,
			expectedCmd: "move",
			expectedArg: "north",
		},
		{
			name:        "Empty input",
			input:       "",
			expectError: true,
			expectedCmd: "",
			expectedArg: "",
		},
		{
			name:        "Whitespace only",
			input:       "   ",
			expectError: true,
			expectedCmd: "",
			expectedArg: "",
		},
		{
			name:        "Mixed case command",
			input:       "LOOK Around",
			expectError: false,
			expectedCmd: "look",
			expectedArg: "Around",
		},
		{
			name:        "Command with multiple spaces in argument",
			input:       "tell bob hello there friend",
			expectError: false,
			expectedCmd: "tell",
			expectedArg: "bob hello there friend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := cm.ParseCommand(tt.input, playerID)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if cmd.Name != tt.expectedCmd {
				t.Errorf("Expected command %q, got %q", tt.expectedCmd, cmd.Name)
			}
			
			if cmd.Argument != tt.expectedArg {
				t.Errorf("Expected argument %q, got %q", tt.expectedArg, cmd.Argument)
			}
			
			if cmd.PlayerID != playerID {
				t.Errorf("Expected player ID %q, got %q", playerID, cmd.PlayerID)
			}
			
			if cmd.Raw != strings.TrimSpace(tt.input) {
				t.Errorf("Expected raw %q, got %q", strings.TrimSpace(tt.input), cmd.Raw)
			}
			
			if cmd.ID == "" {
				t.Error("Command ID should not be empty")
			}
			
			if cmd.Timestamp.IsZero() {
				t.Error("Command timestamp should not be zero")
			}
		})
	}
}

// Test command parsing when disabled
func TestCommandParsingDisabled(t *testing.T) {
	mockEventManager := &MockEventManager{}
	config := &command.CommandConfig{
		QueueSize: 2,
		Enabled:   false,
	}
	cm := command.NewCommandManager(config, mockEventManager)
	playerID := "test-player-1"

	_, err := cm.ParseCommand("look", playerID)
	if err == nil {
		t.Error("Expected error when command processing is disabled")
	}
	
	if !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Expected error message to contain 'disabled', got: %v", err)
	}
}

// Test command ID uniqueness
func TestCommandIDUniqueness(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		cmd, err := cm.ParseCommand("test", playerID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		
		if ids[cmd.ID] {
			t.Errorf("Duplicate command ID found: %s", cmd.ID)
		}
		ids[cmd.ID] = true
	}
}

// Test command timestamp ordering
func TestCommandTimestampOrdering(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	var commands []*command.Command
	for i := 0; i < 10; i++ {
		cmd, err := cm.ParseCommand("test", playerID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		commands = append(commands, cmd)
		time.Sleep(time.Millisecond) // Small delay to ensure different timestamps
	}

	// Check that timestamps are in ascending order
	for i := 1; i < len(commands); i++ {
		if commands[i].Timestamp.Before(commands[i-1].Timestamp) {
			t.Errorf("Commands are not in timestamp order: %v >= %v", 
				commands[i-1].Timestamp, commands[i].Timestamp)
		}
	}
}

// Test edge cases and special characters
func TestCommandParsingEdgeCases(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	tests := []struct {
		name        string
		input       string
		expectError bool
		expectedCmd string
		expectedArg string
	}{
		{
			name:        "Command with special characters",
			input:       "say hello@world.com",
			expectError: false,
			expectedCmd: "say",
			expectedArg: "hello@world.com",
		},
		{
			name:        "Command with numbers",
			input:       "get item123",
			expectError: false,
			expectedCmd: "get",
			expectedArg: "item123",
		},
		{
			name:        "Command with underscores",
			input:       "cast_spell fireball",
			expectError: false,
			expectedCmd: "cast_spell",
			expectedArg: "fireball",
		},
		{
			name:        "Command with hyphen",
			input:       "long-command test",
			expectError: false,
			expectedCmd: "long-command",
			expectedArg: "test",
		},
		{
			name:        "Single character command",
			input:       "i",
			expectError: false,
			expectedCmd: "i",
			expectedArg: "",
		},
		{
			name:        "Command with tabs",
			input:       "move\tnorth",
			expectError: false,
			expectedCmd: "move",
			expectedArg: "north",
		},
		{
			name:        "Command with newlines",
			input:       "say\nhello",
			expectError: false,
			expectedCmd: "say",
			expectedArg: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := cm.ParseCommand(tt.input, playerID)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if cmd.Name != tt.expectedCmd {
				t.Errorf("Expected command %q, got %q", tt.expectedCmd, cmd.Name)
			}
			
			if cmd.Argument != tt.expectedArg {
				t.Errorf("Expected argument %q, got %q", tt.expectedArg, cmd.Argument)
			}
		})
	}
}

// Test command parsing with very long input
func TestCommandParsingLongInput(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)
	playerID := "test-player-1"

	// Test with very long command name
	longCommand := strings.Repeat("a", 1000)
	cmd, err := cm.ParseCommand(longCommand, playerID)
	if err != nil {
		t.Errorf("Unexpected error with long command: %v", err)
	}
	if cmd.Name != longCommand {
		t.Error("Long command name not preserved")
	}

	// Test with very long argument
	longArg := strings.Repeat("b", 10000)
	input := "say " + longArg
	cmd, err = cm.ParseCommand(input, playerID)
	if err != nil {
		t.Errorf("Unexpected error with long argument: %v", err)
	}
	if cmd.Argument != longArg {
		t.Error("Long argument not preserved")
	}
}

// Test command parsing with different player IDs
func TestCommandParsingDifferentPlayers(t *testing.T) {
	mockEventManager := &MockEventManager{}
	cm := command.NewCommandManager(nil, mockEventManager)

	players := []string{"player1", "player2", "player3", ""}
	
	for _, playerID := range players {
		cmd, err := cm.ParseCommand("test", playerID)
		if err != nil {
			t.Errorf("Unexpected error for player %q: %v", playerID, err)
		}
		if cmd.PlayerID != playerID {
			t.Errorf("Expected player ID %q, got %q", playerID, cmd.PlayerID)
		}
	}
}

// Test command manager configuration
func TestCommandManagerConfiguration(t *testing.T) {
	mockEventManager := &MockEventManager{}
	
	// Test with nil config (should use defaults)
	cm1 := command.NewCommandManager(nil, mockEventManager)
	if !cm1.IsEnabled() {
		t.Error("Command manager should be enabled by default")
	}
	
	// Test with custom config
	config := &command.CommandConfig{
		QueueSize: 5,
		Enabled:   false,
	}
	cm2 := command.NewCommandManager(config, mockEventManager)
	if cm2.IsEnabled() {
		t.Error("Command manager should be disabled with custom config")
	}
	
	// Test enable/disable
	cm2.SetEnabled(true)
	if !cm2.IsEnabled() {
		t.Error("Command manager should be enabled after SetEnabled(true)")
	}
	
	cm2.SetEnabled(false)
	if cm2.IsEnabled() {
		t.Error("Command manager should be disabled after SetEnabled(false)")
	}
}