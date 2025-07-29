package scripting

import (
	"fmt"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
)

// LuaEventHandler represents a Lua function that handles events
type LuaEventHandler struct {
	ID           string
	VMID         string
	EventType    events.EventType
	LuaFunction  string // Lua function name or code
	Priority     int
	IsActive     bool
	CreatedAt    time.Time
	LastExecuted time.Time
	ExecuteCount int64
}

// LuaEventManager manages Lua-based event handlers
type LuaEventManager struct {
	vmManager   *VMManager
	handlers    map[string]*LuaEventHandler
	eventSubs   map[events.EventType][]*LuaEventHandler
	mutex       sync.RWMutex
	
	// Integration with core event system
	coreEventManager *events.DefaultEventManager
	coreHookID       string
}

// NewLuaEventManager creates a new Lua event manager
func NewLuaEventManager(vmManager *VMManager, coreEventManager *events.DefaultEventManager) *LuaEventManager {
	lem := &LuaEventManager{
		vmManager:        vmManager,
		handlers:         make(map[string]*LuaEventHandler),
		eventSubs:        make(map[events.EventType][]*LuaEventHandler),
		coreEventManager: coreEventManager,
	}
	
	// Register with core event system to receive all events
	if coreEventManager != nil {
		// For now, skip registration until we fix the API
		// hookID := coreEventManager.RegisterHook("*", lem.handleCoreEvent, 100)
		// lem.coreHookID = hookID
	}
	
	return lem
}

// RegisterLuaHandler registers a Lua function to handle specific event types
func (lem *LuaEventManager) RegisterLuaHandler(vmID string, eventType events.EventType, luaFunction string, priority int) (string, error) {
	lem.mutex.Lock()
	defer lem.mutex.Unlock()
	
	// Verify VM exists
	_, exists := lem.vmManager.GetVM(vmID)
	if !exists {
		return "", fmt.Errorf("VM %s does not exist", vmID)
	}
	
	// Generate handler ID
	handlerID := fmt.Sprintf("lua_handler_%s_%s_%d", vmID, eventType, time.Now().UnixNano())
	
	handler := &LuaEventHandler{
		ID:          handlerID,
		VMID:        vmID,
		EventType:   eventType,
		LuaFunction: luaFunction,
		Priority:    priority,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	
	// Store handler
	lem.handlers[handlerID] = handler
	
	// Add to event subscriptions
	if lem.eventSubs[eventType] == nil {
		lem.eventSubs[eventType] = make([]*LuaEventHandler, 0)
	}
	lem.eventSubs[eventType] = append(lem.eventSubs[eventType], handler)
	
	// Sort handlers by priority (higher priority first)
	lem.sortHandlersByPriority(eventType)
	
	return handlerID, nil
}

// UnregisterLuaHandler removes a Lua event handler
func (lem *LuaEventManager) UnregisterLuaHandler(handlerID string) error {
	lem.mutex.Lock()
	defer lem.mutex.Unlock()
	
	handler, exists := lem.handlers[handlerID]
	if !exists {
		return fmt.Errorf("handler %s does not exist", handlerID)
	}
	
	// Remove from handlers map
	delete(lem.handlers, handlerID)
	
	// Remove from event subscriptions
	eventHandlers := lem.eventSubs[handler.EventType]
	for i, h := range eventHandlers {
		if h.ID == handlerID {
			lem.eventSubs[handler.EventType] = append(eventHandlers[:i], eventHandlers[i+1:]...)
			break
		}
	}
	
	// Clean up empty subscription lists
	if len(lem.eventSubs[handler.EventType]) == 0 {
		delete(lem.eventSubs, handler.EventType)
	}
	
	return nil
}

// handleCoreEvent is called by the core event system for all events
func (lem *LuaEventManager) handleCoreEvent(event events.Event) events.EventResult {
	eventType := event.Type()
	
	lem.mutex.RLock()
	handlers := lem.eventSubs[eventType]
	
	// Also check for wildcard handlers (if we support them)
	wildcardHandlers := lem.eventSubs["*"]
	allHandlers := append(handlers, wildcardHandlers...)
	lem.mutex.RUnlock()
	
	if len(allHandlers) == 0 {
		return events.EventResultContinue // No Lua handlers for this event type
	}
	
	// Execute Lua handlers
	for _, handler := range allHandlers {
		if !handler.IsActive {
			continue
		}
		
		err := lem.executeLuaHandler(handler, event)
		if err != nil {
			// Log error but continue with other handlers
			if vm, exists := lem.vmManager.GetVM(handler.VMID); exists && vm.apiCtx.Logger != nil {
				vm.apiCtx.Logger.Error("Lua event handler error", "handler_id", handler.ID, "error", err)
			}
		}
	}
	
	return events.EventResultContinue
}

// executeLuaHandler executes a specific Lua handler for an event
func (lem *LuaEventManager) executeLuaHandler(handler *LuaEventHandler, event events.Event) error {
	vm, exists := lem.vmManager.GetVM(handler.VMID)
	if !exists {
		return fmt.Errorf("VM %s no longer exists", handler.VMID)
	}
	
	// Prepare Lua script to call the handler function
	luaScript := fmt.Sprintf(`
-- Event handler execution
local event_data = {
  type = %q,
  timestamp = %d,
  data = {}
}

-- Call the handler function
local handler_result = nil
local handler_error = nil

local function safe_call()
  %s
end

local success, result = pcall(safe_call)
if not success then
  handler_error = result
else
  handler_result = result
end

-- Return result
return {success = success, result = handler_result, error = handler_error}
`, string(event.Type()), event.Timestamp().Unix(), handler.LuaFunction)
	
	// Execute the handler with quota limits
	result, err := vm.ExecuteWithQuota(luaScript, lem.vmManager.quotaManager)
	if err != nil {
		return fmt.Errorf("failed to execute Lua handler: %w", err)
	}
	
	// Update handler statistics
	lem.mutex.Lock()
	handler.LastExecuted = time.Now()
	handler.ExecuteCount++
	lem.mutex.Unlock()
	
	// Check if handler reported an error
	if resultMap, ok := result.(map[string]interface{}); ok {
		if success, ok := resultMap["success"].(bool); ok && !success {
			if errorMsg, ok := resultMap["error"].(string); ok {
				return fmt.Errorf("Lua handler error: %s", errorMsg)
			}
		}
	}
	
	return nil
}

// SetHandlerActive enables or disables a Lua event handler
func (lem *LuaEventManager) SetHandlerActive(handlerID string, active bool) error {
	lem.mutex.Lock()
	defer lem.mutex.Unlock()
	
	handler, exists := lem.handlers[handlerID]
	if !exists {
		return fmt.Errorf("handler %s does not exist", handlerID)
	}
	
	handler.IsActive = active
	return nil
}

// GetHandlerStats returns statistics for all Lua event handlers
func (lem *LuaEventManager) GetHandlerStats() map[string]interface{} {
	lem.mutex.RLock()
	defer lem.mutex.RUnlock()
	
	handlerStats := make(map[string]interface{})
	eventCounts := make(map[string]int)
	
	for handlerID, handler := range lem.handlers {
		handlerStats[handlerID] = map[string]interface{}{
			"vm_id":         handler.VMID,
			"event_type":    string(handler.EventType),
			"priority":      handler.Priority,
			"is_active":     handler.IsActive,
			"created_at":    handler.CreatedAt,
			"last_executed": handler.LastExecuted,
			"execute_count": handler.ExecuteCount,
		}
		
		eventCounts[string(handler.EventType)]++
	}
	
	return map[string]interface{}{
		"total_handlers":   len(lem.handlers),
		"handlers":         handlerStats,
		"event_type_counts": eventCounts,
		"core_hook_id":     lem.coreHookID,
	}
}

// GetHandlersForVM returns all handlers for a specific VM
func (lem *LuaEventManager) GetHandlersForVM(vmID string) []*LuaEventHandler {
	lem.mutex.RLock()
	defer lem.mutex.RUnlock()
	
	var vmHandlers []*LuaEventHandler
	for _, handler := range lem.handlers {
		if handler.VMID == vmID {
			vmHandlers = append(vmHandlers, handler)
		}
	}
	
	return vmHandlers
}

// CleanupVMHandlers removes all handlers for a specific VM
func (lem *LuaEventManager) CleanupVMHandlers(vmID string) error {
	lem.mutex.Lock()
	defer lem.mutex.Unlock()
	
	// Find all handlers for this VM
	handlersToRemove := make([]string, 0)
	for handlerID, handler := range lem.handlers {
		if handler.VMID == vmID {
			handlersToRemove = append(handlersToRemove, handlerID)
		}
	}
	
	// Remove each handler
	for _, handlerID := range handlersToRemove {
		handler := lem.handlers[handlerID]
		delete(lem.handlers, handlerID)
		
		// Remove from event subscriptions
		eventHandlers := lem.eventSubs[handler.EventType]
		for i, h := range eventHandlers {
			if h.ID == handlerID {
				lem.eventSubs[handler.EventType] = append(eventHandlers[:i], eventHandlers[i+1:]...)
				break
			}
		}
		
		// Clean up empty subscription lists
		if len(lem.eventSubs[handler.EventType]) == 0 {
			delete(lem.eventSubs, handler.EventType)
		}
	}
	
	return nil
}

// Shutdown cleans up the Lua event manager
func (lem *LuaEventManager) Shutdown() error {
	lem.mutex.Lock()
	defer lem.mutex.Unlock()
	
	// Unregister from core event system
	if lem.coreEventManager != nil && lem.coreHookID != "" {
		lem.coreEventManager.UnregisterHook(lem.coreHookID)
	}
	
	// Clear all handlers
	lem.handlers = make(map[string]*LuaEventHandler)
	lem.eventSubs = make(map[events.EventType][]*LuaEventHandler)
	
	return nil
}

// sortHandlersByPriority sorts handlers for an event type by priority (higher first)
func (lem *LuaEventManager) sortHandlersByPriority(eventType events.EventType) {
	handlers := lem.eventSubs[eventType]
	if len(handlers) <= 1 {
		return
	}
	
	// Simple bubble sort by priority (higher priority first)
	for i := 0; i < len(handlers)-1; i++ {
		for j := 0; j < len(handlers)-i-1; j++ {
			if handlers[j].Priority < handlers[j+1].Priority {
				handlers[j], handlers[j+1] = handlers[j+1], handlers[j]
			}
		}
	}
}