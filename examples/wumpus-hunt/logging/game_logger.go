package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
)

// GameEventType represents different types of game events
type GameEventType string

const (
	// Authentication Events
	EventTypeLogin        GameEventType = "login"
	EventTypeLogout       GameEventType = "logout"
	EventTypeAuthFailure  GameEventType = "auth_failure"
	EventTypeAccountCreate GameEventType = "account_create"

	// Game State Events
	EventTypeGameStart    GameEventType = "game_start"
	EventTypeGameEnd      GameEventType = "game_end"
	EventTypePlayerDeath  GameEventType = "player_death"
	EventTypePlayerVictory GameEventType = "player_victory"
	EventTypePlayerRespawn GameEventType = "player_respawn"

	// Movement Events
	EventTypePlayerMove   GameEventType = "player_move"
	EventTypePortalEnter  GameEventType = "portal_enter"
	EventTypePortalExit   GameEventType = "portal_exit"
	EventTypeRoomEnter    GameEventType = "room_enter"

	// Combat Events
	EventTypeCombatStart  GameEventType = "combat_start"
	EventTypeCombatEnd    GameEventType = "combat_end"
	EventTypeAttack       GameEventType = "attack"
	EventTypeWumpusKill   GameEventType = "wumpus_kill"
	EventTypeWumpusMove   GameEventType = "wumpus_move"

	// Instance Management Events
	EventTypeInstanceCreate GameEventType = "instance_create"
	EventTypeInstanceDestroy GameEventType = "instance_destroy"
	EventTypeInstanceCleanup GameEventType = "instance_cleanup"

	// Communication Events
	EventTypeChat        GameEventType = "chat"
	EventTypeShout       GameEventType = "shout"
	EventTypeEmote       GameEventType = "emote"
	EventTypeTell        GameEventType = "tell"

	// Properties Events
	EventTypePropertyRead  GameEventType = "property_read"
	EventTypePropertyWrite GameEventType = "property_write"
	EventTypePropertyError GameEventType = "property_error"

	// Error Events
	EventTypeError        GameEventType = "error"
	EventTypeException    GameEventType = "exception"
	EventTypeSystemError  GameEventType = "system_error"
)

// GameEvent represents a structured game event
type GameEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   GameEventType          `json:"event_type"`
	PlayerID    string                 `json:"player_id,omitempty"`
	PlayerName  string                 `json:"player_name,omitempty"`
	ClientID    string                 `json:"client_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	WorldID     string                 `json:"world_id,omitempty"`
	RoomID      string                 `json:"room_id,omitempty"`
	InstanceID  string                 `json:"instance_id,omitempty"`
	Message     string                 `json:"message"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Success     bool                   `json:"success"`
	Component   string                 `json:"component,omitempty"`
}

// GameLogger handles game-specific logging
type GameLogger struct {
	logger       *logging.Logger
	fileLogger   *os.File
	eventChannel chan GameEvent
	stopChannel  chan struct{}
	wg           sync.WaitGroup
	config       *GameLoggerConfig
	mu           sync.RWMutex
}

// GameLoggerConfig configures the game logger
type GameLoggerConfig struct {
	LogLevel         logging.LogLevel `json:"log_level"`
	EnableFileLog    bool             `json:"enable_file_log"`
	LogDirectory     string           `json:"log_directory"`
	MaxFileSize      int64            `json:"max_file_size"`
	MaxFiles         int              `json:"max_files"`
	EnableConsoleLog bool             `json:"enable_console_log"`
	EnableJSONLog    bool             `json:"enable_json_log"`
	BufferSize       int              `json:"buffer_size"`
	FlushInterval    time.Duration    `json:"flush_interval"`
}

// DefaultGameLoggerConfig returns default configuration
func DefaultGameLoggerConfig() *GameLoggerConfig {
	return &GameLoggerConfig{
		LogLevel:         logging.LevelInfo,
		EnableFileLog:    true,
		LogDirectory:     "./logs",
		MaxFileSize:      100 * 1024 * 1024, // 100MB
		MaxFiles:         10,
		EnableConsoleLog: true,
		EnableJSONLog:    true,
		BufferSize:       1000,
		FlushInterval:    5 * time.Second,
	}
}

// NewGameLogger creates a new game logger
func NewGameLogger(config *GameLoggerConfig) (*GameLogger, error) {
	if config == nil {
		config = DefaultGameLoggerConfig()
	}

	gl := &GameLogger{
		config:       config,
		eventChannel: make(chan GameEvent, config.BufferSize),
		stopChannel:  make(chan struct{}),
	}

	// Create base logger
	if config.EnableJSONLog {
		gl.logger = logging.NewJSONLogger(config.LogLevel, os.Stdout)
	} else {
		gl.logger = logging.NewLogger(config.LogLevel, os.Stdout)
	}

	// Set up file logging if enabled
	if config.EnableFileLog {
		if err := gl.setupFileLogging(); err != nil {
			return nil, fmt.Errorf("failed to setup file logging: %w", err)
		}
	}

	// Start background event processor
	gl.wg.Add(1)
	go gl.processEvents()

	return gl, nil
}

// setupFileLogging configures file-based logging
func (gl *GameLogger) setupFileLogging() error {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(gl.config.LogDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(gl.config.LogDirectory, fmt.Sprintf("wumpus-hunt_%s.log", timestamp))

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	gl.fileLogger = file
	return nil
}

// processEvents handles background event processing
func (gl *GameLogger) processEvents() {
	defer gl.wg.Done()
	
	ticker := time.NewTicker(gl.config.FlushInterval)
	defer ticker.Stop()

	var events []GameEvent

	for {
		select {
		case event := <-gl.eventChannel:
			events = append(events, event)
			
			// If buffer is full, flush immediately
			if len(events) >= gl.config.BufferSize/2 {
				gl.flushEvents(events)
				events = events[:0]
			}

		case <-ticker.C:
			// Periodic flush
			if len(events) > 0 {
				gl.flushEvents(events)
				events = events[:0]
			}

		case <-gl.stopChannel:
			// Final flush before shutdown
			if len(events) > 0 {
				gl.flushEvents(events)
			}
			return
		}
	}
}

// flushEvents writes events to file and console
func (gl *GameLogger) flushEvents(events []GameEvent) {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	for _, event := range events {
		// Console logging
		if gl.config.EnableConsoleLog {
			gl.logToConsole(event)
		}

		// File logging
		if gl.config.EnableFileLog && gl.fileLogger != nil {
			gl.logToFile(event)
		}
	}
}

// logToConsole writes event to console using structured logging
func (gl *GameLogger) logToConsole(event GameEvent) {
	logger := gl.logger
	
	// Add contextual fields
	if event.PlayerID != "" {
		logger = logger.WithClient(event.PlayerID)
	}
	if event.Component != "" {
		logger = logger.WithComponent(event.Component)
	}

	// Choose log level based on event type
	logArgs := []any{
		"event_type", event.EventType,
		"player_name", event.PlayerName,
		"world_id", event.WorldID,
		"room_id", event.RoomID,
		"instance_id", event.InstanceID,
		"success", event.Success,
	}

	if event.Duration > 0 {
		logArgs = append(logArgs, "duration", event.Duration)
	}

	if len(event.Data) > 0 {
		logArgs = append(logArgs, "data", event.Data)
	}

	if event.Error != "" {
		logArgs = append(logArgs, "error", event.Error)
	}

	// Log at appropriate level
	switch event.EventType {
	case EventTypeError, EventTypeException, EventTypeSystemError, EventTypeAuthFailure:
		logger.Error(event.Message, logArgs...)
	case EventTypePropertyError:
		logger.Warn(event.Message, logArgs...)
	case EventTypeLogin, EventTypeLogout, EventTypeGameStart, EventTypeGameEnd, EventTypePlayerVictory:
		logger.Info(event.Message, logArgs...)
	default:
		logger.Debug(event.Message, logArgs...)
	}
}

// logToFile writes event to file as JSON
func (gl *GameLogger) logToFile(event GameEvent) {
	if gl.fileLogger == nil {
		return
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return
	}

	gl.fileLogger.Write(eventJSON)
	gl.fileLogger.Write([]byte("\n"))
}

// LogEvent logs a game event
func (gl *GameLogger) LogEvent(event GameEvent) {
	event.Timestamp = time.Now()
	select {
	case gl.eventChannel <- event:
		// Event queued successfully
	default:
		// Channel full, log directly to avoid blocking
		gl.mu.Lock()
		if gl.config.EnableConsoleLog {
			gl.logToConsole(event)
		}
		if gl.config.EnableFileLog && gl.fileLogger != nil {
			gl.logToFile(event)
		}
		gl.mu.Unlock()
	}
}

// Authentication logging methods
func (gl *GameLogger) LogLogin(playerID, playerName, clientID string, success bool, message string, err error) {
	event := GameEvent{
		EventType:  EventTypeLogin,
		PlayerID:   playerID,
		PlayerName: playerName,
		ClientID:   clientID,
		Message:    message,
		Success:    success,
		Component:  "auth",
	}
	if err != nil {
		event.Error = err.Error()
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogLogout(playerID, playerName, clientID string, message string) {
	event := GameEvent{
		EventType:  EventTypeLogout,
		PlayerID:   playerID,
		PlayerName: playerName,
		ClientID:   clientID,
		Message:    message,
		Success:    true,
		Component:  "auth",
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogAccountCreate(playerID, playerName, clientID string, success bool, message string, err error) {
	event := GameEvent{
		EventType:  EventTypeAccountCreate,
		PlayerID:   playerID,
		PlayerName: playerName,
		ClientID:   clientID,
		Message:    message,
		Success:    success,
		Component:  "auth",
	}
	if err != nil {
		event.Error = err.Error()
	}
	gl.LogEvent(event)
}

// Game state logging methods
func (gl *GameLogger) LogGameStart(playerID, playerName, instanceID, worldID string) {
	event := GameEvent{
		EventType:  EventTypeGameStart,
		PlayerID:   playerID,
		PlayerName: playerName,
		InstanceID: instanceID,
		WorldID:    worldID,
		Message:    fmt.Sprintf("Player %s started new game in instance %s", playerName, instanceID),
		Success:    true,
		Component:  "game",
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogGameEnd(playerID, playerName, instanceID string, victory bool, duration time.Duration) {
	event := GameEvent{
		EventType:  EventTypeGameEnd,
		PlayerID:   playerID,
		PlayerName: playerName,
		InstanceID: instanceID,
		Message:    fmt.Sprintf("Player %s ended game - %s", playerName, map[bool]string{true: "Victory", false: "Defeat"}[victory]),
		Success:    victory,
		Duration:   duration,
		Component:  "game",
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogPlayerDeath(playerID, playerName, worldID, roomID string, cause string) {
	event := GameEvent{
		EventType:  EventTypePlayerDeath,
		PlayerID:   playerID,
		PlayerName: playerName,
		WorldID:    worldID,
		RoomID:     roomID,
		Message:    fmt.Sprintf("Player %s died from %s", playerName, cause),
		Success:    false,
		Component:  "game",
		Data: map[string]interface{}{
			"death_cause": cause,
		},
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogPlayerVictory(playerID, playerName, worldID, roomID, instanceID string, wumpusPelts int) {
	event := GameEvent{
		EventType:  EventTypePlayerVictory,
		PlayerID:   playerID,
		PlayerName: playerName,
		WorldID:    worldID,
		RoomID:     roomID,
		InstanceID: instanceID,
		Message:    fmt.Sprintf("Player %s achieved victory - killed wumpus!", playerName),
		Success:    true,
		Component:  "game",
		Data: map[string]interface{}{
			"wumpus_pelts": wumpusPelts,
		},
	}
	gl.LogEvent(event)
}

// Movement logging methods
func (gl *GameLogger) LogPlayerMove(playerID, playerName, fromRoomID, toRoomID, worldID, direction string) {
	event := GameEvent{
		EventType:  EventTypePlayerMove,
		PlayerID:   playerID,
		PlayerName: playerName,
		WorldID:    worldID,
		RoomID:     toRoomID,
		Message:    fmt.Sprintf("Player %s moved %s", playerName, direction),
		Success:    true,
		Component:  "movement",
		Data: map[string]interface{}{
			"from_room": fromRoomID,
			"to_room":   toRoomID,
			"direction": direction,
		},
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogPortalEnter(playerID, playerName, instanceID, targetWorldID string) {
	event := GameEvent{
		EventType:  EventTypePortalEnter,
		PlayerID:   playerID,
		PlayerName: playerName,
		InstanceID: instanceID,
		WorldID:    targetWorldID,
		Message:    fmt.Sprintf("Player %s entered portal to instance %s", playerName, instanceID),
		Success:    true,
		Component:  "portal",
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogPortalExit(playerID, playerName, instanceID string) {
	event := GameEvent{
		EventType:  EventTypePortalExit,
		PlayerID:   playerID,
		PlayerName: playerName,
		InstanceID: instanceID,
		Message:    fmt.Sprintf("Player %s exited portal from instance %s", playerName, instanceID),
		Success:    true,
		Component:  "portal",
	}
	gl.LogEvent(event)
}

// Combat logging methods
func (gl *GameLogger) LogAttack(playerID, playerName, target, worldID, roomID string, damage int, success bool) {
	event := GameEvent{
		EventType:  EventTypeAttack,
		PlayerID:   playerID,
		PlayerName: playerName,
		WorldID:    worldID,
		RoomID:     roomID,
		Message:    fmt.Sprintf("Player %s attacked %s for %d damage", playerName, target, damage),
		Success:    success,
		Component:  "combat",
		Data: map[string]interface{}{
			"target": target,
			"damage": damage,
		},
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogWumpusKill(playerID, playerName, worldID, roomID, instanceID string, totalDamage int) {
	event := GameEvent{
		EventType:  EventTypeWumpusKill,
		PlayerID:   playerID,
		PlayerName: playerName,
		WorldID:    worldID,
		RoomID:     roomID,
		InstanceID: instanceID,
		Message:    fmt.Sprintf("Player %s killed the Wumpus with %d total damage", playerName, totalDamage),
		Success:    true,
		Component:  "combat",
		Data: map[string]interface{}{
			"total_damage": totalDamage,
		},
	}
	gl.LogEvent(event)
}

// Instance management logging methods
func (gl *GameLogger) LogInstanceCreate(instanceID, worldID string, roomCount int, playerID string) {
	event := GameEvent{
		EventType:  EventTypeInstanceCreate,
		InstanceID: instanceID,
		WorldID:    worldID,
		PlayerID:   playerID,
		Message:    fmt.Sprintf("Created new instance %s with %d rooms", instanceID, roomCount),
		Success:    true,
		Component:  "instance",
		Data: map[string]interface{}{
			"room_count": roomCount,
		},
	}
	gl.LogEvent(event)
}

func (gl *GameLogger) LogInstanceDestroy(instanceID, worldID string) {
	event := GameEvent{
		EventType:  EventTypeInstanceDestroy,
		InstanceID: instanceID,
		WorldID:    worldID,
		Message:    fmt.Sprintf("Destroyed instance %s", instanceID),
		Success:    true,
		Component:  "instance",
	}
	gl.LogEvent(event)
}

// Communication logging methods
func (gl *GameLogger) LogChat(playerID, playerName, worldID, roomID, message string, chatType GameEventType) {
	event := GameEvent{
		EventType:  chatType,
		PlayerID:   playerID,
		PlayerName: playerName,
		WorldID:    worldID,
		RoomID:     roomID,
		Message:    fmt.Sprintf("Player %s: %s", playerName, message),
		Success:    true,
		Component:  "chat",
		Data: map[string]interface{}{
			"chat_message": message,
		},
	}
	gl.LogEvent(event)
}

// Properties logging methods
func (gl *GameLogger) LogPropertyAccess(playerID, propertyKey string, operation string, success bool, err error) {
	eventType := EventTypePropertyRead
	if operation == "write" {
		eventType = EventTypePropertyWrite
	}

	event := GameEvent{
		EventType:  eventType,
		PlayerID:   playerID,
		Message:    fmt.Sprintf("Property %s %s for player %s", operation, propertyKey, playerID),
		Success:    success,
		Component:  "properties",
		Data: map[string]interface{}{
			"property_key": propertyKey,
			"operation":    operation,
		},
	}
	
	if err != nil {
		event.Error = err.Error()
		event.EventType = EventTypePropertyError
	}
	
	gl.LogEvent(event)
}

// Error logging methods
func (gl *GameLogger) LogError(component, message string, err error, playerID string) {
	event := GameEvent{
		EventType: EventTypeError,
		PlayerID:  playerID,
		Message:   message,
		Error:     err.Error(),
		Success:   false,
		Component: component,
	}
	gl.LogEvent(event)
}

// Close shuts down the game logger
func (gl *GameLogger) Close() error {
	close(gl.stopChannel)
	gl.wg.Wait()

	if gl.fileLogger != nil {
		return gl.fileLogger.Close()
	}

	return nil
}

// GetPlayerStats retrieves player statistics from logs (this would be more complex in a real implementation)
func (gl *GameLogger) GetPlayerStats(player *storage.Player) map[string]interface{} {
	stats := auth.GetWumpusStats(player)
	
	return map[string]interface{}{
		"games_played":   stats.GamesPlayed,
		"games_won":      stats.GamesWon,
		"wumpus_pelts":   stats.WumpusPelts,
		"total_deaths":   stats.TotalDeaths,
		"last_activity":  time.Now(), // In real implementation, this would come from logs
	}
}

// Global game logger instance
var globalGameLogger *GameLogger

// InitializeGameLogger initializes the global game logger
func InitializeGameLogger(config *GameLoggerConfig) error {
	logger, err := NewGameLogger(config)
	if err != nil {
		return err
	}
	globalGameLogger = logger
	return nil
}

// GetGameLogger returns the global game logger
func GetGameLogger() *GameLogger {
	return globalGameLogger
}

// ShutdownGameLogger shuts down the global game logger
func ShutdownGameLogger() error {
	if globalGameLogger != nil {
		return globalGameLogger.Close()
	}
	return nil
}