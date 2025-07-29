package events

import (
	"context"
	"sync"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	// Connection events
	EventTypeConnectionOpen    EventType = "connection.open"
	EventTypeConnectionClose   EventType = "connection.close"
	EventTypeConnectionError   EventType = "connection.error"
	EventTypeConnectionTimeout EventType = "connection.timeout"
	
	// Authentication events
	EventTypeAuthAttempt  EventType = "auth.attempt"
	EventTypeAuthSuccess  EventType = "auth.success"
	EventTypeAuthFailure  EventType = "auth.failure"
	EventTypeAuthLogout   EventType = "auth.logout"
	
	// Message events
	EventTypeMessageReceived EventType = "message.received"
	EventTypeMessageSent     EventType = "message.sent"
	EventTypeMessageDropped  EventType = "message.dropped"
	
	// Server lifecycle events
	EventTypeServerStart    EventType = "server.start"
	EventTypeServerStop     EventType = "server.stop"
	EventTypeServerShutdown EventType = "server.shutdown"
	
	// Client lifecycle events
	EventTypeClientConnect    EventType = "client.connect"
	EventTypeClientDisconnect EventType = "client.disconnect"
	EventTypeClientReconnect  EventType = "client.reconnect"
	
	// Game events (extensible)
	EventTypePlayerJoin    EventType = "game.player.join"
	EventTypePlayerLeave   EventType = "game.player.leave"
	EventTypePlayerMove    EventType = "game.player.move"
	EventTypePlayerAction  EventType = "game.player.action"
	EventTypeRoomJoin      EventType = "game.room.join"
	EventTypeRoomLeave     EventType = "game.room.leave"
	
	// System events
	EventTypeSystemSignal   EventType = "system.signal"
	EventTypeSystemShutdown EventType = "system.shutdown"
	EventTypeSystemError    EventType = "system.error"
	
	// Custom events start with this prefix
	EventTypeCustomPrefix EventType = "custom."
)

// EventResult represents the result of event processing
type EventResult int

const (
	EventResultContinue EventResult = iota // Continue processing other handlers
	EventResultStop                        // Stop processing, but don't cancel the event
	EventResultCancel                      // Cancel the event (prevent default action)
)

// Event represents a system or game event that can be hooked
type Event interface {
	// Type returns the event type
	Type() EventType
	
	// Source returns the source that generated this event (optional)
	Source() interface{}
	
	// Data returns event-specific data
	Data() map[string]interface{}
	
	// SetData sets event data (for modification)
	SetData(key string, value interface{})
	
	// Timestamp returns when the event was created
	Timestamp() time.Time
	
	// Context returns the event context (for cancellation)
	Context() context.Context
	
	// IsCancelled returns whether the event has been cancelled
	IsCancelled() bool
	
	// Cancel cancels the event (prevents default action)
	Cancel()
	
	// IsModifiable returns whether this event can be modified
	IsModifiable() bool
}

// EventHook represents a function that handles events
type EventHook func(event Event) EventResult

// EventHookRegistration represents a registered event hook
type EventHookRegistration struct {
	ID       string
	EventType EventType
	Hook     EventHook
	Priority int  // Lower numbers = higher priority (run first)
	Once     bool // If true, hook is removed after first execution
}

// EventManager manages event hooks and dispatching
type EventManager interface {
	// RegisterHook registers an event hook
	RegisterHook(eventType EventType, hook EventHook, priority int) string
	
	// RegisterHookOnce registers an event hook that runs only once
	RegisterHookOnce(eventType EventType, hook EventHook, priority int) string
	
	// RegisterHookWithID registers an event hook with a specific ID
	RegisterHookWithID(id string, eventType EventType, hook EventHook, priority int) error
	
	// UnregisterHook removes an event hook by ID
	UnregisterHook(id string) bool
	
	// UnregisterAllHooks removes all hooks for an event type
	UnregisterAllHooks(eventType EventType) int
	
	// TriggerEvent triggers an event and runs all registered hooks
	TriggerEvent(event Event) EventResult
	
	// TriggerEventSync triggers an event synchronously
	TriggerEventSync(event Event) EventResult
	
	// TriggerEventAsync triggers an event asynchronously
	TriggerEventAsync(event Event)
	
	// CreateEvent creates a new event
	CreateEvent(eventType EventType, source interface{}, data map[string]interface{}) Event
	
	// ListHooks returns all registered hooks for an event type
	ListHooks(eventType EventType) []*EventHookRegistration
	
	// GetHookCount returns the number of registered hooks for an event type
	GetHookCount(eventType EventType) int
	
	// Close shuts down the event manager
	Close() error
}

// BaseEvent provides a basic implementation of Event
type BaseEvent struct {
	eventType   EventType
	source      interface{}
	data        map[string]interface{}
	timestamp   time.Time
	ctx         context.Context
	cancel      context.CancelFunc
	cancelled   bool
	modifiable  bool
	mutex       sync.RWMutex
}

// NewEvent creates a new base event
func NewEvent(eventType EventType, source interface{}, data map[string]interface{}) *BaseEvent {
	if data == nil {
		data = make(map[string]interface{})
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &BaseEvent{
		eventType:  eventType,
		source:     source,
		data:       data,
		timestamp:  time.Now(),
		ctx:        ctx,
		cancel:     cancel,
		cancelled:  false,
		modifiable: true,
	}
}

// NewEventWithContext creates a new base event with a parent context
func NewEventWithContext(ctx context.Context, eventType EventType, source interface{}, data map[string]interface{}) *BaseEvent {
	if data == nil {
		data = make(map[string]interface{})
	}
	
	eventCtx, cancel := context.WithCancel(ctx)
	
	return &BaseEvent{
		eventType:  eventType,
		source:     source,
		data:       data,
		timestamp:  time.Now(),
		ctx:        eventCtx,
		cancel:     cancel,
		cancelled:  false,
		modifiable: true,
	}
}

// Type returns the event type
func (e *BaseEvent) Type() EventType {
	return e.eventType
}

// Source returns the event source
func (e *BaseEvent) Source() interface{} {
	return e.source
}

// Data returns event data
func (e *BaseEvent) Data() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	result := make(map[string]interface{})
	for k, v := range e.data {
		result[k] = v
	}
	return result
}

// SetData sets event data
func (e *BaseEvent) SetData(key string, value interface{}) {
	if !e.modifiable {
		return
	}
	
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.data[key] = value
}

// Timestamp returns the event timestamp
func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// Context returns the event context
func (e *BaseEvent) Context() context.Context {
	return e.ctx
}

// IsCancelled returns whether the event is cancelled
func (e *BaseEvent) IsCancelled() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.cancelled
}

// Cancel cancels the event
func (e *BaseEvent) Cancel() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	if !e.cancelled {
		e.cancelled = true
		e.cancel()
	}
}

// IsModifiable returns whether the event can be modified
func (e *BaseEvent) IsModifiable() bool {
	return e.modifiable
}

// SetModifiable sets whether the event can be modified
func (e *BaseEvent) SetModifiable(modifiable bool) {
	e.modifiable = modifiable
}

// GetString returns a string value from event data
func (e *BaseEvent) GetString(key string) (string, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	if value, exists := e.data[key]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetInt returns an int value from event data
func (e *BaseEvent) GetInt(key string) (int, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	if value, exists := e.data[key]; exists {
		if i, ok := value.(int); ok {
			return i, true
		}
	}
	return 0, false
}

// GetBool returns a bool value from event data
func (e *BaseEvent) GetBool(key string) (bool, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	if value, exists := e.data[key]; exists {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// Clone creates a copy of the event
func (e *BaseEvent) Clone() *BaseEvent {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	data := make(map[string]interface{})
	for k, v := range e.data {
		data[k] = v
	}
	
	return NewEvent(e.eventType, e.source, data)
}