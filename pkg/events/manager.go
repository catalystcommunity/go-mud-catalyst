package events

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// DefaultEventManager implements EventManager
type DefaultEventManager struct {
	hooks       map[EventType][]*EventHookRegistration
	hooksById   map[string]*EventHookRegistration
	mutex       sync.RWMutex
	logger      *logging.Logger
	asyncQueue  chan Event
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	closed      bool
}

// NewEventManager creates a new event manager
func NewEventManager() *DefaultEventManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	em := &DefaultEventManager{
		hooks:      make(map[EventType][]*EventHookRegistration),
		hooksById:  make(map[string]*EventHookRegistration),
		logger:     logging.GetDefaultLogger().WithComponent("events"),
		asyncQueue: make(chan Event, 10000), // Buffer for async events - increased for high load
		ctx:        ctx,
		cancel:     cancel,
		closed:     false,
	}
	
	// Start async event processor
	em.wg.Add(1)
	go em.processAsyncEvents()
	
	return em
}

// RegisterHook registers an event hook with auto-generated ID
func (em *DefaultEventManager) RegisterHook(eventType EventType, hook EventHook, priority int) string {
	id := ids.RandStringRunes(16)
	if err := em.RegisterHookWithID(id, eventType, hook, priority); err != nil {
		// Generate a new ID if collision occurred
		id = ids.RandStringRunes(24)
		em.RegisterHookWithID(id, eventType, hook, priority)
	}
	return id
}

// RegisterHookOnce registers an event hook that runs only once
func (em *DefaultEventManager) RegisterHookOnce(eventType EventType, hook EventHook, priority int) string {
	id := ids.RandStringRunes(16)
	
	registration := &EventHookRegistration{
		ID:        id,
		EventType: eventType,
		Hook:      hook,
		Priority:  priority,
		Once:      true,
	}
	
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	if em.closed {
		em.logger.Warn("Attempted to register hook on closed event manager")
		return ""
	}
	
	// Check for ID collision
	if _, exists := em.hooksById[id]; exists {
		id = ids.RandStringRunes(24)
		registration.ID = id
	}
	
	// Add to hooks map
	em.hooks[eventType] = append(em.hooks[eventType], registration)
	em.hooksById[id] = registration
	
	// Sort hooks by priority (lower number = higher priority)
	em.sortHooks(eventType)
	
	em.logger.Debug("Registered one-time event hook", 
		"id", id, "event_type", eventType, "priority", priority)
	
	return id
}

// RegisterHookWithID registers an event hook with a specific ID
func (em *DefaultEventManager) RegisterHookWithID(id string, eventType EventType, hook EventHook, priority int) error {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	if em.closed {
		return fmt.Errorf("event manager is closed")
	}
	
	// Check for existing ID
	if _, exists := em.hooksById[id]; exists {
		return fmt.Errorf("hook with ID %s already exists", id)
	}
	
	registration := &EventHookRegistration{
		ID:        id,
		EventType: eventType,
		Hook:      hook,
		Priority:  priority,
		Once:      false,
	}
	
	// Add to hooks map
	em.hooks[eventType] = append(em.hooks[eventType], registration)
	em.hooksById[id] = registration
	
	// Sort hooks by priority (lower number = higher priority)
	em.sortHooks(eventType)
	
	em.logger.Debug("Registered event hook", 
		"id", id, "event_type", eventType, "priority", priority)
	
	return nil
}

// UnregisterHook removes an event hook by ID
func (em *DefaultEventManager) UnregisterHook(id string) bool {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	registration, exists := em.hooksById[id]
	if !exists {
		return false
	}
	
	// Remove from hooksById
	delete(em.hooksById, id)
	
	// Remove from hooks slice
	hooks := em.hooks[registration.EventType]
	for i, hook := range hooks {
		if hook.ID == id {
			em.hooks[registration.EventType] = append(hooks[:i], hooks[i+1:]...)
			break
		}
	}
	
	em.logger.Debug("Unregistered event hook", "id", id, "event_type", registration.EventType)
	return true
}

// UnregisterAllHooks removes all hooks for an event type
func (em *DefaultEventManager) UnregisterAllHooks(eventType EventType) int {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	
	hooks := em.hooks[eventType]
	count := len(hooks)
	
	// Remove from hooksById
	for _, hook := range hooks {
		delete(em.hooksById, hook.ID)
	}
	
	// Clear hooks for this event type
	delete(em.hooks, eventType)
	
	em.logger.Debug("Unregistered all hooks for event type", 
		"event_type", eventType, "count", count)
	
	return count
}

// TriggerEvent triggers an event and runs all registered hooks (synchronously)
func (em *DefaultEventManager) TriggerEvent(event Event) EventResult {
	return em.TriggerEventSync(event)
}

// TriggerEventSync triggers an event synchronously
func (em *DefaultEventManager) TriggerEventSync(event Event) EventResult {
	em.mutex.RLock()
	if em.closed {
		em.mutex.RUnlock()
		return EventResultCancel
	}
	em.mutex.RUnlock()
	
	em.mutex.RLock()
	hooks := make([]*EventHookRegistration, len(em.hooks[event.Type()]))
	copy(hooks, em.hooks[event.Type()])
	em.mutex.RUnlock()
	
	if len(hooks) == 0 {
		return EventResultContinue
	}
	
	em.logger.Debug("Triggering event", 
		"event_type", event.Type(), "hook_count", len(hooks))
	
	var hooksToRemove []string
	
	for _, registration := range hooks {
		// Check if event was cancelled
		if event.IsCancelled() {
			em.logger.Debug("Event was cancelled, stopping hook execution", 
				"event_type", event.Type())
			break
		}
		
		// Check context cancellation
		select {
		case <-event.Context().Done():
			em.logger.Debug("Event context cancelled", "event_type", event.Type())
			return EventResultCancel
		case <-em.ctx.Done():
			em.logger.Debug("Event manager context cancelled")
			return EventResultCancel
		default:
		}
		
		// Execute hook
		start := time.Now()
		result := registration.Hook(event)
		duration := time.Since(start)
		
		em.logger.Debug("Executed event hook", 
			"hook_id", registration.ID,
			"event_type", event.Type(),
			"duration", duration,
			"result", result)
		
		// Mark for removal if it's a one-time hook
		if registration.Once {
			hooksToRemove = append(hooksToRemove, registration.ID)
		}
		
		// Handle hook result
		switch result {
		case EventResultStop:
			// Stop processing other hooks, but don't cancel event
			em.logger.Debug("Hook requested stop", 
				"hook_id", registration.ID, "event_type", event.Type())
			break
		case EventResultCancel:
			// Cancel the event
			em.logger.Debug("Hook requested cancellation", 
				"hook_id", registration.ID, "event_type", event.Type())
			event.Cancel()
			break
		case EventResultContinue:
			// Continue to next hook
			continue
		}
		
		// If we got Stop or Cancel, break out of loop
		if result == EventResultStop || result == EventResultCancel {
			break
		}
	}
	
	// Remove one-time hooks
	for _, hookId := range hooksToRemove {
		em.UnregisterHook(hookId)
	}
	
	if event.IsCancelled() {
		return EventResultCancel
	}
	
	return EventResultContinue
}

// TriggerEventAsync triggers an event asynchronously
func (em *DefaultEventManager) TriggerEventAsync(event Event) {
	em.mutex.RLock()
	if em.closed {
		em.mutex.RUnlock()
		return
	}
	em.mutex.RUnlock()
	
	select {
	case em.asyncQueue <- event:
		// Event queued successfully
	default:
		// Queue is full, log warning
		em.logger.Warn("Async event queue is full, dropping event", 
			"event_type", event.Type())
	}
}

// CreateEvent creates a new event
func (em *DefaultEventManager) CreateEvent(eventType EventType, source interface{}, data map[string]interface{}) Event {
	return NewEventWithContext(em.ctx, eventType, source, data)
}

// ListHooks returns all registered hooks for an event type
func (em *DefaultEventManager) ListHooks(eventType EventType) []*EventHookRegistration {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	hooks := em.hooks[eventType]
	result := make([]*EventHookRegistration, len(hooks))
	copy(result, hooks)
	return result
}

// GetHookCount returns the number of registered hooks for an event type
func (em *DefaultEventManager) GetHookCount(eventType EventType) int {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	return len(em.hooks[eventType])
}

// Close shuts down the event manager
func (em *DefaultEventManager) Close() error {
	em.mutex.Lock()
	if em.closed {
		em.mutex.Unlock()
		return nil
	}
	em.closed = true
	em.mutex.Unlock()
	
	em.logger.Info("Shutting down event manager")
	
	// Cancel context to stop async processing
	em.cancel()
	
	// Wait for async processor to finish
	em.wg.Wait()
	
	// Clear all hooks
	em.mutex.Lock()
	em.hooks = make(map[EventType][]*EventHookRegistration)
	em.hooksById = make(map[string]*EventHookRegistration)
	em.mutex.Unlock()
	
	em.logger.Info("Event manager shutdown complete")
	return nil
}

// processAsyncEvents processes events from the async queue
func (em *DefaultEventManager) processAsyncEvents() {
	defer em.wg.Done()
	
	for {
		select {
		case event := <-em.asyncQueue:
			em.TriggerEventSync(event)
		case <-em.ctx.Done():
			// Process remaining events in queue before shutting down
			for {
				select {
				case event := <-em.asyncQueue:
					em.TriggerEventSync(event)
				default:
					em.logger.Debug("Async event processor shutting down")
					return
				}
			}
		}
	}
}

// sortHooks sorts hooks by priority (lower number = higher priority)
func (em *DefaultEventManager) sortHooks(eventType EventType) {
	hooks := em.hooks[eventType]
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Priority < hooks[j].Priority
	})
}

// Helper methods for common hook patterns

// RegisterPreHook registers a hook with high priority (runs early)
func (em *DefaultEventManager) RegisterPreHook(eventType EventType, hook EventHook) string {
	return em.RegisterHook(eventType, hook, -100)
}

// RegisterPostHook registers a hook with low priority (runs late)
func (em *DefaultEventManager) RegisterPostHook(eventType EventType, hook EventHook) string {
	return em.RegisterHook(eventType, hook, 100)
}

// RegisterValidationHook registers a validation hook (very high priority)
func (em *DefaultEventManager) RegisterValidationHook(eventType EventType, hook EventHook) string {
	return em.RegisterHook(eventType, hook, -1000)
}

// RegisterCleanupHook registers a cleanup hook (very low priority)
func (em *DefaultEventManager) RegisterCleanupHook(eventType EventType, hook EventHook) string {
	return em.RegisterHook(eventType, hook, 1000)
}

// GetStats returns statistics about the event manager
func (em *DefaultEventManager) GetStats() map[string]interface{} {
	em.mutex.RLock()
	defer em.mutex.RUnlock()
	
	totalHooks := len(em.hooksById)
	hooksByType := make(map[string]int)
	
	for eventType, hooks := range em.hooks {
		hooksByType[string(eventType)] = len(hooks)
	}
	
	queueLength := len(em.asyncQueue)
	queueCapacity := cap(em.asyncQueue)
	queueUtilization := float64(queueLength) / float64(queueCapacity)
	
	// Log warning if queue utilization is high
	if queueUtilization > 0.8 {
		em.logger.Warn("Event queue utilization high", 
			"utilization", fmt.Sprintf("%.2f%%", queueUtilization*100),
			"length", queueLength,
			"capacity", queueCapacity)
	}
	
	return map[string]interface{}{
		"total_hooks":        totalHooks,
		"hooks_by_type":      hooksByType,
		"queue_length":       queueLength,
		"queue_capacity":     queueCapacity,
		"queue_utilization":  queueUtilization,
		"closed":             em.closed,
	}
}