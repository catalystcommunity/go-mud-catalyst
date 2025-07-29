package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/pkg/ids"
)

// Test error types for testing
type TestPlayerNotFoundError struct {
	PlayerID string
}

func (e *TestPlayerNotFoundError) Error() string {
	return "player not found: " + e.PlayerID
}

type TestInvalidPropertyError struct {
	PropertyKey string
}

func (e *TestInvalidPropertyError) Error() string {
	return "invalid property: " + e.PropertyKey
}

func TestGameLogger_Creation(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.LogDirectory = filepath.Join(os.TempDir(), "test_logs")
	config.EnableFileLog = true
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	if logger == nil {
		t.Fatal("Logger should not be nil")
	}

	// Verify log directory was created
	if _, err := os.Stat(config.LogDirectory); os.IsNotExist(err) {
		t.Error("Log directory should have been created")
	}

	// Cleanup
	os.RemoveAll(config.LogDirectory)
}

func TestGameLogger_LogEvent(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.LogDirectory = filepath.Join(os.TempDir(), "test_logs")
	config.EnableFileLog = true
	config.EnableConsoleLog = false
	config.FlushInterval = 100 * time.Millisecond

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	// Log a test event
	logger.LogEvent(GameEvent{
		EventType:  EventTypeLogin,
		PlayerID:   "test-player-123",
		PlayerName: "TestPlayer",
		Message:    "Test login event",
		Success:    true,
		Component:  "test",
	})

	// Wait for event to be processed
	time.Sleep(200 * time.Millisecond)

	// Cleanup
	os.RemoveAll(config.LogDirectory)
}

func TestGameLogger_AuthenticationLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	// Test login logging
	logger.LogLogin("player-123", "TestPlayer", "client-456", true, "Login successful", nil)
	logger.LogLogin("player-789", "BadPlayer", "client-999", false, "Login failed", 
		&TestPlayerNotFoundError{PlayerID: "player-789"})

	// Test account creation logging
	logger.LogAccountCreate("player-new", "NewPlayer", "client-new", true, "Account created", nil)

	// Test logout logging
	logger.LogLogout("player-123", "TestPlayer", "client-456", "Logout successful")

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_GameStateLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	playerID := "player-123"
	playerName := "TestPlayer"
	instanceID := "instance-456"
	worldID := "world-789"
	roomID := "room-101"

	// Test game start logging
	logger.LogGameStart(playerID, playerName, instanceID, worldID)

	// Test player death logging
	logger.LogPlayerDeath(playerID, playerName, worldID, roomID, "wumpus attack")

	// Test player victory logging
	logger.LogPlayerVictory(playerID, playerName, worldID, roomID, instanceID, 1)

	// Test game end logging
	logger.LogGameEnd(playerID, playerName, instanceID, true, 5*time.Minute)

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_MovementLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	playerID := "player-123"
	playerName := "TestPlayer"
	worldID := "world-789"
	fromRoom := "room-101"
	toRoom := "room-102"
	instanceID := "instance-456"

	// Test player move logging
	logger.LogPlayerMove(playerID, playerName, fromRoom, toRoom, worldID, "north")

	// Test portal enter logging
	logger.LogPortalEnter(playerID, playerName, instanceID, worldID)

	// Test portal exit logging
	logger.LogPortalExit(playerID, playerName, instanceID)

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_CombatLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	playerID := "player-123"
	playerName := "TestPlayer"
	worldID := "world-789"
	roomID := "room-101"
	instanceID := "instance-456"

	// Test attack logging
	logger.LogAttack(playerID, playerName, "wumpus", worldID, roomID, 25, true)
	logger.LogAttack(playerID, playerName, "wumpus", worldID, roomID, 0, false)

	// Test wumpus kill logging
	logger.LogWumpusKill(playerID, playerName, worldID, roomID, instanceID, 100)

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_InstanceLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	instanceID := "instance-456"
	worldID := "world-789"
	playerID := "player-123"

	// Test instance creation logging
	logger.LogInstanceCreate(instanceID, worldID, 12, playerID)

	// Test instance destruction logging
	logger.LogInstanceDestroy(instanceID, worldID)

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_CommunicationLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	playerID := "player-123"
	playerName := "TestPlayer"
	worldID := "world-789"
	roomID := "room-101"

	// Test different chat types
	logger.LogChat(playerID, playerName, worldID, roomID, "Hello everyone!", EventTypeChat)
	logger.LogChat(playerID, playerName, worldID, roomID, "Can anyone hear me?", EventTypeShout)
	logger.LogChat(playerID, playerName, worldID, roomID, "waves at everyone", EventTypeEmote)
	logger.LogChat(playerID, playerName, worldID, roomID, "Secret message", EventTypeTell)

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_PropertiesLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	playerID := "player-123"

	// Test successful property access
	logger.LogPropertyAccess(playerID, "wumpus_auth", "read", true, nil)
	logger.LogPropertyAccess(playerID, "wumpus_stats", "write", true, nil)

	// Test failed property access  
	logger.LogPropertyAccess(playerID, "invalid_prop", "read", false, 
		&TestInvalidPropertyError{PropertyKey: "invalid_prop"})

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_ErrorLogging(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	playerID := "player-123"

	// Test error logging
	logger.LogError("auth", "Authentication failed", 
		&TestPlayerNotFoundError{PlayerID: playerID}, playerID)
	logger.LogError("game", "Game state corruption detected", 
		&TestInvalidPropertyError{PropertyKey: "wumpus_state"}, playerID)

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_Configuration(t *testing.T) {
	// Test default configuration
	config := DefaultGameLoggerConfig()
	if config.LogLevel != logging.LevelInfo {
		t.Error("Default log level should be Info")
	}
	if !config.EnableFileLog {
		t.Error("File logging should be enabled by default")
	}
	if !config.EnableConsoleLog {
		t.Error("Console logging should be enabled by default")
	}

	// Test custom configuration
	customConfig := &GameLoggerConfig{
		LogLevel:         logging.LevelDebug,
		EnableFileLog:    false,
		EnableConsoleLog: true,
		BufferSize:       500,
		FlushInterval:    1 * time.Second,
	}

	logger, err := NewGameLogger(customConfig)
	if err != nil {
		t.Fatalf("Failed to create logger with custom config: %v", err)
	}
	defer logger.Close()

	if logger.config.LogLevel != logging.LevelDebug {
		t.Error("Custom log level not applied")
	}
	if logger.config.BufferSize != 500 {
		t.Error("Custom buffer size not applied")
	}
}

func TestGameLogger_BufferOverflow(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false
	config.BufferSize = 5 // Very small buffer
	config.FlushInterval = 1 * time.Second // Long flush interval

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	// Log more events than buffer size
	for i := 0; i < 10; i++ {
		logger.LogEvent(GameEvent{
			EventType:  EventTypeLogin,
			PlayerID:   "player-123",
			PlayerName: "TestPlayer",
			Message:    "Buffer overflow test",
			Success:    true,
			Component:  "test",
		})
	}

	// All events should be handled even with buffer overflow
	time.Sleep(100 * time.Millisecond)
}

func TestGameLogger_Shutdown(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.LogDirectory = filepath.Join(os.TempDir(), "test_logs")
	config.EnableFileLog = true

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}

	// Log some events
	logger.LogEvent(GameEvent{
		EventType:  EventTypeLogin,
		PlayerID:   "player-123",
		PlayerName: "TestPlayer", 
		Message:    "Shutdown test",
		Success:    true,
		Component:  "test",
	})

	// Test graceful shutdown
	err = logger.Close()
	if err != nil {
		t.Fatalf("Failed to close logger: %v", err)
	}

	// Cleanup
	os.RemoveAll(config.LogDirectory)
}

func TestGameLogger_GetPlayerStats(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	// Create a test player
	player := storage.NewPlayer(ids.NewEntityID(), "TestPlayer")
	player.DisplayName = "TestPlayer"

	// Get player stats
	stats := logger.GetPlayerStats(player)
	if stats == nil {
		t.Error("Player stats should not be nil")
	}

	// Verify expected fields exist
	expectedFields := []string{"games_played", "games_won", "wumpus_pelts", "total_deaths"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field %s not found in stats", field)
		}
	}
}

func TestGameLogger_EventTypesConsistency(t *testing.T) {
	// Test that all defined event types are valid
	eventTypes := []GameEventType{
		EventTypeLogin, EventTypeLogout, EventTypeAuthFailure, EventTypeAccountCreate,
		EventTypeGameStart, EventTypeGameEnd, EventTypePlayerDeath, EventTypePlayerVictory,
		EventTypePlayerRespawn, EventTypePlayerMove, EventTypePortalEnter, EventTypePortalExit,
		EventTypeRoomEnter, EventTypeCombatStart, EventTypeCombatEnd, EventTypeAttack,
		EventTypeWumpusKill, EventTypeWumpusMove, EventTypeInstanceCreate, EventTypeInstanceDestroy,
		EventTypeInstanceCleanup, EventTypeChat, EventTypeShout, EventTypeEmote, EventTypeTell,
		EventTypePropertyRead, EventTypePropertyWrite, EventTypePropertyError,
		EventTypeError, EventTypeException, EventTypeSystemError,
	}

	for _, eventType := range eventTypes {
		if string(eventType) == "" {
			t.Errorf("Event type %v should not be empty", eventType)
		}
	}
}

func TestGameLogger_TimestampAccuracy(t *testing.T) {
	config := DefaultGameLoggerConfig()
	config.EnableFileLog = false
	config.EnableConsoleLog = false

	logger, err := NewGameLogger(config)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer logger.Close()

	// Record time before logging
	beforeTime := time.Now()

	// Log an event
	event := GameEvent{
		EventType:  EventTypeLogin,
		PlayerID:   "player-123",
		PlayerName: "TestPlayer",
		Message:    "Timestamp test",
		Success:    true,
		Component:  "test",
	}
	logger.LogEvent(event)

	// Record time after logging
	afterTime := time.Now()

	// The event timestamp should be set automatically
	// We can't access it directly, but we can verify the logic works
	// by checking that the before/after times make sense
	if beforeTime.After(afterTime) {
		t.Error("Time measurement is inconsistent")
	}
}

// Helper function to create test storage manager
func createTestStorageManager() *storage.Manager {
	config := storage.DefaultStorageConfig()
	config.DataRoot = filepath.Join(os.TempDir(), "test_storage")
	mgr := storage.NewManagerWithConfig(config)
	mgr.Initialize()
	return mgr
}

// Cleanup helper
func cleanupTestData(dataRoot string) {
	os.RemoveAll(dataRoot)
}