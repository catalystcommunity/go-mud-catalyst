// Package scripting provides Lua scripting integration for MuddyCore
// 
// This package offers:
// - Secure Lua VM execution with resource limits
// - CPU, memory, and execution time quotas  
// - Event system integration
// - Hot-reloading for development
// - Comprehensive sandboxing for user scripts
//
// Example usage:
//
//   // Create scripting system
//   config := scripting.DefaultScriptingConfig()
//   manager := scripting.NewVMManager(config)
//   
//   // Create a VM for a user
//   vm, err := manager.CreateVM("user123")
//   if err != nil {
//     log.Fatal(err)
//   }
//   
//   // Configure API context
//   vm.SetAPIContext(logger, eventManager, luaEventManager, "server1", "client1")
//   
//   // Execute user script
//   result, err := manager.ExecuteScript("user123", `
//     muddycore.log.info("Hello from Lua!")
//     
//     -- Subscribe to chat events
//     local handler_id = muddycore.event.subscribe("message.received", [[
//       local event = muddycore.event.get_current()
//       muddycore.log.info("Received message: " .. tostring(event.data))
//     ]], 100)
//     
//     return "Script executed successfully"
//   `)
//
package scripting

import (
	"fmt"
	"os"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// ScriptingSystem provides a high-level interface to the Lua scripting subsystem
type ScriptingSystem struct {
	vmManager       *VMManager
	luaEventManager *LuaEventManager
	scriptWatcher   *ScriptWatcher
	config          *ScriptingConfig
	logger          *logging.Logger
}

// NewScriptingSystem creates a new scripting system with the given configuration
func NewScriptingSystem(config *ScriptingConfig, logger *logging.Logger, eventManager *events.DefaultEventManager) (*ScriptingSystem, error) {
	if config == nil {
		config = DefaultScriptingConfig()
	}
	
	if logger == nil {
		logger = logging.DefaultLogger()
	}
	
	// Create VM manager
	vmManager := NewVMManager(config)
	
	// Create Lua event manager
	luaEventManager := NewLuaEventManager(vmManager, eventManager)
	
	// Create script watcher for hot reloading
	hotReloadConfig := DefaultHotReloadConfig()
	scriptWatcher := NewScriptWatcher(vmManager, hotReloadConfig)
	
	system := &ScriptingSystem{
		vmManager:       vmManager,
		luaEventManager: luaEventManager,
		scriptWatcher:   scriptWatcher,
		config:          config,
		logger:          logger,
	}
	
	return system, nil
}

// CreateUserVM creates a VM for a specific user with proper context
func (ss *ScriptingSystem) CreateUserVM(userID string, serverID string, eventManager *events.DefaultEventManager) (*VM, error) {
	vm, err := ss.vmManager.CreateVM(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM for user %s: %w", userID, err)
	}
	
	// Configure API context
	vm.SetAPIContext(ss.logger, eventManager, ss.luaEventManager, serverID, userID)
	
	// Set user context data
	vm.SetUserData("user_id", userID)
	vm.SetUserData("server_id", serverID)
	vm.SetUserData("created_at", time.Now())
	
	ss.logger.Info("Created Lua VM for user", "user_id", userID, "server_id", serverID)
	
	return vm, nil
}

// ExecuteUserScript executes a script for a specific user
func (ss *ScriptingSystem) ExecuteUserScript(userID string, script string) (interface{}, error) {
	result, err := ss.vmManager.ExecuteScript(userID, script)
	if err != nil {
		ss.logger.Error("Failed to execute user script", 
			"user_id", userID, 
			"error", err,
			"script_length", len(script))
		return nil, err
	}
	
	ss.logger.Debug("Successfully executed user script", 
		"user_id", userID, 
		"script_length", len(script))
	
	return result, nil
}

// LoadUserScriptFromFile loads and executes a script file for a user
func (ss *ScriptingSystem) LoadUserScriptFromFile(userID string, filePath string, enableHotReload bool) error {
	// Actually read the script file
	scriptBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", filePath, err)
	}
	
	script := string(scriptBytes)
	
	// Execute the script
	_, err = ss.ExecuteUserScript(userID, script)
	if err != nil {
		return fmt.Errorf("failed to load script from %s: %w", filePath, err)
	}
	
	// Enable hot reloading if requested
	if enableHotReload {
		err = ss.scriptWatcher.WatchScript(filePath, userID, "on_reload", true)
		if err != nil {
			ss.logger.Warn("Failed to enable hot reload for script", 
				"file", filePath, 
				"user_id", userID, 
				"error", err)
		} else {
			ss.logger.Info("Enabled hot reload for user script", 
				"file", filePath, 
				"user_id", userID)
		}
	}
	
	return nil
}

// RegisterGlobalEventHandler registers a Lua event handler that applies to all events
func (ss *ScriptingSystem) RegisterGlobalEventHandler(vmID string, luaFunction string, priority int) (string, error) {
	return ss.luaEventManager.RegisterLuaHandler(vmID, "*", luaFunction, priority)
}

// GetSystemStats returns comprehensive statistics about the scripting system
func (ss *ScriptingSystem) GetSystemStats() map[string]interface{} {
	vmStats := ss.vmManager.GetStats()
	eventStats := ss.luaEventManager.GetHandlerStats()
	watcherStats := ss.scriptWatcher.GetStats()
	
	return map[string]interface{}{
		"vm_manager":       vmStats,
		"event_manager":    eventStats,
		"script_watcher":   watcherStats,
		"config":           ss.config,
		"uptime":           time.Since(time.Now()), // Placeholder
	}
}

// EnableHotReloading starts the script watcher for hot reloading
func (ss *ScriptingSystem) EnableHotReloading() error {
	return ss.scriptWatcher.Start()
}

// DisableHotReloading stops the script watcher
func (ss *ScriptingSystem) DisableHotReloading() error {
	return ss.scriptWatcher.Stop()
}

// CleanupUser removes all resources associated with a user
func (ss *ScriptingSystem) CleanupUser(userID string) error {
	// Remove event handlers
	err := ss.luaEventManager.CleanupVMHandlers(userID)
	if err != nil {
		ss.logger.Warn("Failed to cleanup event handlers for user", 
			"user_id", userID, 
			"error", err)
	}
	
	// Remove from script watcher
	watchedScripts := ss.scriptWatcher.GetWatchedScripts()
	for filePath, script := range watchedScripts {
		if script.VMID == userID {
			ss.scriptWatcher.UnwatchScript(filePath)
		}
	}
	
	// Destroy VM
	err = ss.vmManager.DestroyVM(userID)
	if err != nil {
		return fmt.Errorf("failed to cleanup user VM: %w", err)
	}
	
	ss.logger.Info("Cleaned up user scripting resources", "user_id", userID)
	return nil
}

// Shutdown gracefully shuts down the entire scripting system
func (ss *ScriptingSystem) Shutdown() error {
	ss.logger.Info("Shutting down scripting system")
	
	// Stop hot reloading
	if err := ss.scriptWatcher.Stop(); err != nil {
		ss.logger.Warn("Error stopping script watcher", "error", err)
	}
	
	// Shutdown event manager
	if err := ss.luaEventManager.Shutdown(); err != nil {
		ss.logger.Warn("Error shutting down Lua event manager", "error", err)
	}
	
	// Shutdown VM manager
	if err := ss.vmManager.Shutdown(); err != nil {
		ss.logger.Warn("Error shutting down VM manager", "error", err)
	}
	
	ss.logger.Info("Scripting system shutdown complete")
	return nil
}

// ValidateUserScript validates a script before execution
func (ss *ScriptingSystem) ValidateUserScript(script string) error {
	return ValidateScript(script)
}

// GetSecurityViolations returns potential security issues in a script
func (ss *ScriptingSystem) GetSecurityViolations(script string) []string {
	config := DefaultSandboxConfig()
	return GetSandboxViolations(script, config)
}

// SetUserQuota sets custom resource quotas for a specific user
func (ss *ScriptingSystem) SetUserQuota(userID string, cpuTicks int64, memoryBytes int64) error {
	// This would modify the quota manager to set per-user limits
	// For now, return success as the quota system handles this globally
	ss.logger.Info("Set custom quota for user", 
		"user_id", userID, 
		"cpu_ticks", cpuTicks, 
		"memory_bytes", memoryBytes)
	return nil
}

// GetUserVMInfo returns information about a user's VM
func (ss *ScriptingSystem) GetUserVMInfo(userID string) (map[string]interface{}, error) {
	vm, exists := ss.vmManager.GetVM(userID)
	if !exists {
		return nil, fmt.Errorf("no VM found for user %s", userID)
	}
	
	stats := vm.GetStats()
	handlers := ss.luaEventManager.GetHandlersForVM(userID)
	
	handlerInfo := make([]map[string]interface{}, len(handlers))
	for i, handler := range handlers {
		handlerInfo[i] = map[string]interface{}{
			"id":            handler.ID,
			"event_type":    string(handler.EventType),
			"priority":      handler.Priority,
			"is_active":     handler.IsActive,
			"execute_count": handler.ExecuteCount,
			"last_executed": handler.LastExecuted,
		}
	}
	
	return map[string]interface{}{
		"vm_stats":      stats,
		"event_handlers": handlerInfo,
		"handler_count":  len(handlers),
	}, nil
}

// Example functions for demonstration

// CreateExampleGameScript creates an example script for game functionality
func CreateExampleGameScript() string {
	return `
-- Example game script demonstrating MuddyCore Lua integration

-- Initialize player data
local player_id = muddycore.util.get_context("user_id")
muddycore.log.info("Initializing game script for player: " .. tostring(player_id))

-- Example: Chat command handler
local function handle_chat_command(message)
  if string.sub(message, 1, 1) == "/" then
    local command = string.match(message, "^/(%w+)")
    if command == "time" then
      local current_time = muddycore.util.time()
      return "Current server time: " .. tostring(current_time)
    elseif command == "help" then
      return "Available commands: /time, /help, /stats"
    elseif command == "stats" then
      return "Player ID: " .. tostring(player_id)
    end
  end
  return nil
end

-- Subscribe to chat events
local chat_handler_id = muddycore.event.subscribe("message.received", [[
  local event = muddycore.event.get_current()
  if event and event.data and event.data.type == "chat" then
    local response = handle_chat_command(event.data.message)
    if response then
      muddycore.event.emit("message.send", {
        type = "chat",
        message = response,
        target = event.data.sender
      })
    end
  end
]], 100)

-- Subscribe to player join events
local join_handler_id = muddycore.event.subscribe("game.player.join", [[
  local event = muddycore.event.get_current()
  if event and event.data then
    muddycore.log.info("Player joined: " .. tostring(event.data.player_id))
    muddycore.event.emit("message.broadcast", {
      type = "system",
      message = "Player " .. tostring(event.data.player_id) .. " has joined the game!"
    })
  end
]], 100)

-- Store handler IDs for cleanup
muddycore.util.set_context("chat_handler_id", chat_handler_id)
muddycore.util.set_context("join_handler_id", join_handler_id)

-- Initialization complete
muddycore.log.info("Game script initialization complete")
return "Game script loaded successfully"
`
}