package scripting

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/arnodel/golua/runtime"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/scripting"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/ai"
)

// WumpusAI manages the Lua-based Wumpus artificial intelligence
type WumpusAI struct {
	scriptingSystem *scripting.ScriptingSystem
	logger          *logging.Logger
	eventManager    *events.DefaultEventManager
	storageManager  *storage.Manager
	movementAI      *ai.MovementAI
	scriptPath      string
	instanceID      string
	isActive        bool
	wumpusRoomID    string
	userID          string
	eventHookIDs    []string
}

// WumpusAIConfig holds configuration for the Wumpus AI
type WumpusAIConfig struct {
	ScriptPath      string
	InstanceID      string
	InitialRoomID   string
	SecurityProfile string
}

// NewWumpusAI creates a new Wumpus AI instance
func NewWumpusAI(config WumpusAIConfig, scriptingSystem *scripting.ScriptingSystem, logger *logging.Logger, eventManager *events.DefaultEventManager, storageManager *storage.Manager) (*WumpusAI, error) {
	if config.ScriptPath == "" {
		return nil, fmt.Errorf("script path is required")
	}
	
	if config.InstanceID == "" {
		return nil, fmt.Errorf("instance ID is required")
	}

	// Create movement AI
	movementConfig := ai.MovementConfig{
		InstanceID:         config.InstanceID,
		InitialRoomID:      config.InitialRoomID,
		MovementInterval:   time.Second * 30,
		BaseMovementChance: 0.3,
		AggressionDecay:    time.Minute * 5,
	}
	movementAI := ai.NewMovementAI(movementConfig, logger, eventManager, storageManager)

	wumpusAI := &WumpusAI{
		scriptingSystem: scriptingSystem,
		logger:          logger,
		eventManager:    eventManager,
		storageManager:  storageManager,
		movementAI:      movementAI,
		scriptPath:      config.ScriptPath,
		instanceID:      config.InstanceID,
		isActive:        false,
		wumpusRoomID:    config.InitialRoomID,
		userID:          fmt.Sprintf("wumpus_%s_%d", config.InstanceID, time.Now().UnixNano()),
		eventHookIDs:    make([]string, 0),
	}

	return wumpusAI, nil
}

// Initialize sets up the Wumpus AI system and loads the Lua script
func (ai *WumpusAI) Initialize() error {
	ai.logger.Info("Initializing Wumpus AI", 
		"instance_id", ai.instanceID,
		"script_path", ai.scriptPath)

	// Create a VM for the Wumpus AI
	_, err := ai.scriptingSystem.CreateUserVM(
		ai.userID,
		"wumpus_hunt_server",
		ai.eventManager,
	)
	if err != nil {
		return fmt.Errorf("failed to create Wumpus AI VM: %w", err)
	}

	// Load the Wumpus Lua script
	err = ai.scriptingSystem.LoadUserScriptFromFile(
		ai.userID,
		ai.scriptPath,
		true, // enable hot reloading for development
	)
	if err != nil {
		return fmt.Errorf("failed to load Wumpus script: %w", err)
	}

	// Wait a moment for script to fully load before executing
	// Note: In production, we might want to use a callback or event system instead
	time.Sleep(10 * time.Millisecond)

	// Skip script initialization for now to test basic VM/script loading
	// TODO: Re-enable once threading issues are resolved
	
	// Initialize the Wumpus in the script
	/*
	initScript := fmt.Sprintf(`
		if initialize_wumpus then 
			return initialize_wumpus("%s", "%s")
		else
			return false
		end
	`, ai.wumpusRoomID, ai.instanceID)

	result, err := ai.scriptingSystem.ExecuteUserScript(
		ai.userID,
		initScript,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Wumpus in script: %w", err)
	}

	if result == nil || fmt.Sprintf("%v", result) != "true" {
		return fmt.Errorf("Wumpus initialization failed in script")
	}
	*/

	// Initialize movement AI
	err = ai.movementAI.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize movement AI: %w", err)
	}

	// Register event handlers
	err = ai.registerEventHandlers()
	if err != nil {
		return fmt.Errorf("failed to register event handlers: %w", err)
	}

	ai.isActive = true
	ai.logger.Info("Wumpus AI initialized successfully", "instance_id", ai.instanceID)

	return nil
}

// Shutdown cleanly shuts down the Wumpus AI
func (ai *WumpusAI) Shutdown() error {
	if !ai.isActive {
		return nil
	}

	ai.logger.Info("Shutting down Wumpus AI", "instance_id", ai.instanceID)

	// Execute cleanup script
	cleanupScript := `
		if on_script_unload then 
			return on_script_unload()
		else
			return true
		end
	`

	_, err := ai.scriptingSystem.ExecuteUserScript(
		ai.userID,
		cleanupScript,
	)
	if err != nil {
		ai.logger.Warn("Failed to execute cleanup script", "error", err)
	}

	// Unregister event handlers
	ai.unregisterEventHandlers()

	// Shutdown movement AI
	if ai.movementAI != nil {
		err = ai.movementAI.Shutdown()
		if err != nil {
			ai.logger.Warn("Failed to shutdown movement AI", "error", err)
		}
	}

	// Clean up the VM
	err = ai.scriptingSystem.CleanupUser(ai.userID)
	if err != nil {
		ai.logger.Warn("Failed to cleanup Wumpus AI VM", "error", err)
	}

	ai.isActive = false
	ai.logger.Info("Wumpus AI shutdown complete", "instance_id", ai.instanceID)

	return nil
}

// HandlePlayerEnterRoom processes a player entering a room
func (ai *WumpusAI) HandlePlayerEnterRoom(playerID, roomID string) error {
	if !ai.isActive {
		return fmt.Errorf("Wumpus AI is not active")
	}

	script := fmt.Sprintf(`
		if on_player_enter_room then 
			return on_player_enter_room("%s", "%s")
		else
			return false
		end
	`, playerID, roomID)

	_, err := ai.scriptingSystem.ExecuteUserScript(
		ai.userID,
		script,
	)
	if err != nil {
		return fmt.Errorf("failed to handle player enter room: %w", err)
	}

	return nil
}

// HandlePlayerAttackWumpus processes a player attacking the Wumpus
func (ai *WumpusAI) HandlePlayerAttackWumpus(playerID string, damage int) (bool, error) {
	if !ai.isActive {
		return false, fmt.Errorf("Wumpus AI is not active")
	}

	script := fmt.Sprintf(`
		if on_player_attack_wumpus then 
			return on_player_attack_wumpus("%s", %d)
		else
			-- Fallback logic for testing: Wumpus dies if damage >= 100
			return %d >= 100
		end
	`, playerID, damage, damage)

	result, err := ai.scriptingSystem.ExecuteUserScript(
		ai.userID,
		script,
	)
	if err != nil {
		return false, fmt.Errorf("failed to handle player attack: %w", err)
	}

	// Check if result is a boolean true or string "true"
	if result != nil {
		// Try to cast to runtime.Value
		if val, ok := result.(runtime.Value); ok {
			// Handle boolean values from Lua
			if val.Type() == runtime.BoolType {
				return val.AsBool(), nil
			}
			// Handle string values
			if val.Type() == runtime.StringType {
				return val.AsString() == "true", nil
			}
			// Handle numeric values (1 = true, 0 = false)
			if val.Type() == runtime.IntType {
				return val.AsInt() != 0, nil
			}
		}
		
		// Fallback to string comparison
		return fmt.Sprintf("%v", result) == "true", nil
	}
	
	return false, nil
}

// Update triggers the Wumpus AI update cycle
func (ai *WumpusAI) Update(deltaTime float64) error {
	if !ai.isActive {
		return nil
	}

	// Update movement AI
	if ai.movementAI != nil {
		err := ai.movementAI.Update()
		if err != nil {
			ai.logger.Warn("Movement AI update failed", "error", err)
		}

		// Sync current room with movement AI
		currentRoom := ai.movementAI.GetCurrentRoom()
		if currentRoom != ai.wumpusRoomID {
			ai.wumpusRoomID = currentRoom
		}
	}

	// Update Lua script
	script := fmt.Sprintf(`
		if on_update then 
			return on_update(%f)
		else
			return true
		end
	`, deltaTime)

	_, err := ai.scriptingSystem.ExecuteUserScript(
		ai.userID,
		script,
	)
	if err != nil {
		return fmt.Errorf("failed to update Wumpus AI: %w", err)
	}

	return nil
}

// IsActive returns whether the Wumpus AI is currently active
func (ai *WumpusAI) IsActive() bool {
	return ai.isActive
}

// GetCurrentRoom returns the current room ID of the Wumpus
func (ai *WumpusAI) GetCurrentRoom() string {
	return ai.wumpusRoomID
}

// SetCurrentRoom updates the current room ID of the Wumpus
func (ai *WumpusAI) SetCurrentRoom(roomID string) {
	ai.wumpusRoomID = roomID
}

// registerEventHandlers sets up event handlers for the Wumpus AI
func (ai *WumpusAI) registerEventHandlers() error {
	// Register for player movement events
	hookID1 := ai.eventManager.RegisterHook("player_enter_room", ai.onPlayerEnterRoom, 0)
	ai.eventHookIDs = append(ai.eventHookIDs, hookID1)
	
	// Register for combat events
	hookID2 := ai.eventManager.RegisterHook("player_attack_wumpus", ai.onPlayerAttackWumpus, 0)
	ai.eventHookIDs = append(ai.eventHookIDs, hookID2)
	
	// Register for Wumpus-specific events
	hookID3 := ai.eventManager.RegisterHook("wumpus_encounter", ai.onWumpusEncounter, 0)
	ai.eventHookIDs = append(ai.eventHookIDs, hookID3)
	
	hookID4 := ai.eventManager.RegisterHook("wumpus_attack", ai.onWumpusAttack, 0)
	ai.eventHookIDs = append(ai.eventHookIDs, hookID4)
	
	hookID5 := ai.eventManager.RegisterHook("wumpus_moved", ai.onWumpusMove, 0)
	ai.eventHookIDs = append(ai.eventHookIDs, hookID5)
	
	hookID6 := ai.eventManager.RegisterHook("wumpus_death", ai.onWumpusDeath, 0)
	ai.eventHookIDs = append(ai.eventHookIDs, hookID6)

	ai.logger.Debug("Wumpus AI event handlers registered", "instance_id", ai.instanceID)
	return nil
}

// unregisterEventHandlers removes event handlers for the Wumpus AI
func (ai *WumpusAI) unregisterEventHandlers() {
	for _, hookID := range ai.eventHookIDs {
		ai.eventManager.UnregisterHook(hookID)
	}
	ai.eventHookIDs = ai.eventHookIDs[:0]

	ai.logger.Debug("Wumpus AI event handlers unregistered", "instance_id", ai.instanceID)
}

// Event handler functions

func (ai *WumpusAI) onPlayerEnterRoom(event events.Event) events.EventResult {
	data := event.Data()
	
	playerID, _ := data["player_id"].(string)
	roomID, _ := data["room_id"].(string)
	instanceID, _ := data["instance_id"].(string)

	// Only handle events for our instance
	if instanceID != ai.instanceID {
		return events.EventResultContinue
	}

	err := ai.HandlePlayerEnterRoom(playerID, roomID)
	if err != nil {
		ai.logger.Warn("Failed to handle player enter room event", "error", err)
	}
	
	return events.EventResultContinue
}

func (ai *WumpusAI) onPlayerAttackWumpus(event events.Event) events.EventResult {
	data := event.Data()

	playerID, _ := data["player_id"].(string)
	damage, _ := data["damage"].(int)
	instanceID, _ := data["instance_id"].(string)

	// Only handle events for our instance
	if instanceID != ai.instanceID {
		return events.EventResultContinue
	}

	wumpusDied, err := ai.HandlePlayerAttackWumpus(playerID, damage)
	if err != nil {
		ai.logger.Warn("Failed to handle player attack wumpus event", "error", err)
		return events.EventResultContinue
	}

	if wumpusDied {
		ai.isActive = false
		ai.logger.Info("Wumpus has been defeated", 
			"instance_id", ai.instanceID, 
			"killer", playerID)
	}

	return events.EventResultContinue
}

func (ai *WumpusAI) onWumpusEncounter(event events.Event) events.EventResult {
	data := event.Data()

	playerID, _ := data["player_id"].(string)
	roomID, _ := data["room_id"].(string)
	instanceID, _ := data["instance_id"].(string)

	// Only handle events for our instance
	if instanceID != ai.instanceID {
		return events.EventResultContinue
	}

	ai.logger.Info("Wumpus encounter initiated", 
		"player_id", playerID, 
		"room_id", roomID,
		"instance_id", instanceID)

	return events.EventResultContinue
}

func (ai *WumpusAI) onWumpusAttack(event events.Event) events.EventResult {
	data := event.Data()

	playerID, _ := data["player_id"].(string)
	damage, _ := data["damage"].(int)
	instanceID, _ := data["instance_id"].(string)

	// Only handle events for our instance
	if instanceID != ai.instanceID {
		return events.EventResultContinue
	}

	ai.logger.Info("Wumpus attack executed", 
		"target", playerID, 
		"damage", damage,
		"instance_id", instanceID)

	return events.EventResultContinue
}

func (ai *WumpusAI) onWumpusMove(event events.Event) events.EventResult {
	data := event.Data()

	fromRoom, _ := data["from_room"].(string)
	toRoom, _ := data["to_room"].(string)
	instanceID, _ := data["instance_id"].(string)

	// Only handle events for our instance
	if instanceID != ai.instanceID {
		return events.EventResultContinue
	}

	// Update our tracking of Wumpus location
	ai.wumpusRoomID = toRoom

	ai.logger.Info("Wumpus moved", 
		"from", fromRoom, 
		"to", toRoom,
		"instance_id", instanceID)

	return events.EventResultContinue
}

func (ai *WumpusAI) onWumpusDeath(event events.Event) events.EventResult {
	data := event.Data()

	killerID, _ := data["killer_player_id"].(string)
	roomID, _ := data["room_id"].(string)
	instanceID, _ := data["instance_id"].(string)

	// Only handle events for our instance
	if instanceID != ai.instanceID {
		return events.EventResultContinue
	}

	ai.logger.Info("Wumpus has died", 
		"killer", killerID, 
		"room", roomID,
		"instance_id", instanceID)

	// Deactivate the AI
	ai.isActive = false

	return events.EventResultContinue
}

// WumpusAIManager manages multiple Wumpus AI instances
type WumpusAIManager struct {
	scriptingSystem *scripting.ScriptingSystem
	logger          *logging.Logger
	eventManager    *events.DefaultEventManager
	storageManager  *storage.Manager
	instances       map[string]*WumpusAI
	scriptDir       string
}

// NewWumpusAIManager creates a new Wumpus AI manager
func NewWumpusAIManager(scriptingSystem *scripting.ScriptingSystem, logger *logging.Logger, eventManager *events.DefaultEventManager, storageManager *storage.Manager, scriptDir string) *WumpusAIManager {
	return &WumpusAIManager{
		scriptingSystem: scriptingSystem,
		logger:          logger,
		eventManager:    eventManager,
		storageManager:  storageManager,
		instances:       make(map[string]*WumpusAI),
		scriptDir:       scriptDir,
	}
}

// CreateWumpusAI creates a new Wumpus AI instance for a game instance
func (m *WumpusAIManager) CreateWumpusAI(instanceID, initialRoomID string) (*WumpusAI, error) {
	if _, exists := m.instances[instanceID]; exists {
		return nil, fmt.Errorf("Wumpus AI already exists for instance %s", instanceID)
	}

	config := WumpusAIConfig{
		ScriptPath:      filepath.Join(m.scriptDir, "wumpus.lua"),
		InstanceID:      instanceID,
		InitialRoomID:   initialRoomID,
		SecurityProfile: "moderate",
	}

	ai, err := NewWumpusAI(config, m.scriptingSystem, m.logger, m.eventManager, m.storageManager)
	if err != nil {
		return nil, err
	}

	err = ai.Initialize()
	if err != nil {
		return nil, err
	}

	m.instances[instanceID] = ai
	m.logger.Info("Created Wumpus AI instance", "instance_id", instanceID)

	return ai, nil
}

// DestroyWumpusAI removes a Wumpus AI instance
func (m *WumpusAIManager) DestroyWumpusAI(instanceID string) error {
	ai, exists := m.instances[instanceID]
	if !exists {
		return fmt.Errorf("no Wumpus AI exists for instance %s", instanceID)
	}

	err := ai.Shutdown()
	if err != nil {
		m.logger.Warn("Error shutting down Wumpus AI", "instance_id", instanceID, "error", err)
	}

	delete(m.instances, instanceID)
	m.logger.Info("Destroyed Wumpus AI instance", "instance_id", instanceID)

	return nil
}

// GetWumpusAI retrieves a Wumpus AI instance
func (m *WumpusAIManager) GetWumpusAI(instanceID string) (*WumpusAI, bool) {
	ai, exists := m.instances[instanceID]
	return ai, exists
}

// UpdateAll updates all active Wumpus AI instances
func (m *WumpusAIManager) UpdateAll(deltaTime float64) {
	for instanceID, ai := range m.instances {
		if ai.IsActive() {
			err := ai.Update(deltaTime)
			if err != nil {
				m.logger.Warn("Failed to update Wumpus AI", 
					"instance_id", instanceID, 
					"error", err)
			}
		}
	}
}

// Shutdown shuts down all Wumpus AI instances
func (m *WumpusAIManager) Shutdown() error {
	var lastError error
	
	for instanceID, ai := range m.instances {
		err := ai.Shutdown()
		if err != nil {
			m.logger.Warn("Error shutting down Wumpus AI", 
				"instance_id", instanceID, 
				"error", err)
			lastError = err
		}
	}

	m.instances = make(map[string]*WumpusAI)
	m.logger.Info("Wumpus AI Manager shutdown complete")

	return lastError
}