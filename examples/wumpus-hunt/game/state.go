package game

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/items"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// HealingType represents different types of healing available
type HealingType int

const (
	HealingTypeNormal HealingType = iota
	HealingTypeFull
	HealingTypeRegeneration
	HealingTypeRespawn
)

// HealingContext provides context for healing operations
type HealingContext struct {
	Type     HealingType
	Amount   int
	Location string
	Source   string
}

// GameResult represents the outcome of a game action
type GameResult struct {
	Success       bool
	Message       string
	PlayerDied    bool
	PlayerWon     bool
	ItemAwarded   *storage.ItemInstance
	StatsUpdated  bool
	NewLocation   string
	NewWorldID    string
	HealthChanged bool
	OldHealth     int
	NewHealth     int
}

// StateManager handles game state transitions, death, and victory
type StateManager struct {
	storageMgr *storage.Manager
}

// NewStateManager creates a new game state manager
func NewStateManager(storageMgr *storage.Manager) *StateManager {
	return &StateManager{
		storageMgr: storageMgr,
	}
}

// DamagePlayer reduces player health and handles death if health reaches 0
func (sm *StateManager) DamagePlayer(player *storage.Player, damage int, source string) (*GameResult, error) {
	if damage < 0 {
		return nil, errors.New("damage cannot be negative")
	}

	// Get current player state
	state := auth.GetWumpusState(player)
	
	// Apply damage
	state.Health -= damage
	if state.Health < 0 {
		state.Health = 0
	}

	// Save updated state
	auth.UpdateWumpusState(player, state)
	if err := sm.storageMgr.Players().Save(player); err != nil {
		return nil, fmt.Errorf("failed to save player state: %w", err)
	}

	result := &GameResult{
		Success:       true,
		Message:       fmt.Sprintf("You take %d damage from %s! Health: %d/100", damage, source, state.Health),
		HealthChanged: damage > 0,
		OldHealth:     state.Health + damage,
		NewHealth:     state.Health,
	}

	// Check for death
	if state.Health <= 0 {
		deathResult, err := sm.HandlePlayerDeath(player, source)
		if err != nil {
			return nil, fmt.Errorf("failed to handle player death: %w", err)
		}
		
		// Merge death result
		result.PlayerDied = true
		result.Message = deathResult.Message
		result.NewLocation = deathResult.NewLocation
		result.NewWorldID = deathResult.NewWorldID
		result.StatsUpdated = deathResult.StatsUpdated
	}

	return result, nil
}

// HealPlayer restores player health up to maximum
func (sm *StateManager) HealPlayer(player *storage.Player, healAmount int) (*GameResult, error) {
	if healAmount < 0 {
		return nil, errors.New("heal amount cannot be negative")
	}

	// Get current player state
	state := auth.GetWumpusState(player)
	
	// Check if player is dead - can't heal a dead player normally
	if state.Health <= 0 {
		return &GameResult{
			Success: false,
			Message: "You cannot heal while dead. You need to respawn first.",
		}, nil
	}

	// Apply healing
	oldHealth := state.Health
	state.Health += healAmount
	if state.Health > 100 {
		state.Health = 100
	}

	// Save updated state
	auth.UpdateWumpusState(player, state)
	if err := sm.storageMgr.Players().Save(player); err != nil {
		return nil, fmt.Errorf("failed to save player state: %w", err)
	}

	actualHealing := state.Health - oldHealth
	
	// Different messages based on healing amount
	var message string
	if actualHealing == 0 {
		message = "You are already at full health!"
	} else if actualHealing < healAmount {
		message = fmt.Sprintf("You heal for %d health and reach maximum! Health: %d/100", actualHealing, state.Health)
	} else {
		message = fmt.Sprintf("You heal for %d health! Health: %d/100", actualHealing, state.Health)
	}
	
	return &GameResult{
		Success:       true,
		Message:       message,
		HealthChanged: actualHealing > 0,
		OldHealth:     oldHealth,
		NewHealth:     state.Health,
	}, nil
}

// FullyHealPlayer restores player to full health
func (sm *StateManager) FullyHealPlayer(player *storage.Player) (*GameResult, error) {
	// Get current player state
	state := auth.GetWumpusState(player)
	
	if state.Health >= 100 {
		return &GameResult{
			Success: true,
			Message: "You are already at full health!",
		}, nil
	}

	// Store old health for message
	oldHealth := state.Health

	// Set to full health
	state.Health = 100

	// Save updated state
	auth.UpdateWumpusState(player, state)
	if err := sm.storageMgr.Players().Save(player); err != nil {
		return nil, fmt.Errorf("failed to save player state: %w", err)
	}

	healingReceived := 100 - oldHealth
	message := fmt.Sprintf("You feel refreshed and restored to full health! (+%d health) Health: 100/100", healingReceived)

	return &GameResult{
		Success:       true,
		Message:       message,
		HealthChanged: healingReceived > 0,
		OldHealth:     oldHealth,
		NewHealth:     100,
	}, nil
}

// HandlePlayerDeath processes player death, updates stats, and handles respawn
func (sm *StateManager) HandlePlayerDeath(player *storage.Player, deathCause string) (*GameResult, error) {
	logging.Info("Processing player death", "player", player.DisplayName, "cause", deathCause)

	// Get current stats and state
	stats := auth.GetWumpusStats(player)
	state := auth.GetWumpusState(player)

	// Update death statistics
	stats.TotalDeaths++
	if state.IsInMaze {
		stats.GamesPlayed++
	}

	// Reset player state
	state.Health = 100
	state.IsInMaze = false
	state.CurrentInstance = ""

	// Reset combat state on death
	sm.ResetCombatState(player)

	// Save updated data
	auth.UpdateWumpusStats(player, stats)
	auth.UpdateWumpusState(player, state)
	if err := sm.storageMgr.Players().Save(player); err != nil {
		return nil, fmt.Errorf("failed to save player data: %w", err)
	}

	// Get Inn location for respawn
	mainWorld, err := world.GetMainWorld(sm.storageMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to get main world for respawn: %w", err)
	}

	innRoom, err := world.GetInnRoom(sm.storageMgr, mainWorld.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Inn room for respawn: %w", err)
	}

	message := fmt.Sprintf("You have died from %s!\n\n"+
		"The world fades to black... but death is not the end.\n"+
		"You awaken in The Inn, your wounds healed but your pride wounded.\n"+
		"Your adventure statistics have been updated.\n\n"+
		"Total Deaths: %d", deathCause, stats.TotalDeaths)

	return &GameResult{
		Success:      true,
		Message:      message,
		PlayerDied:   true,
		StatsUpdated: true,
		NewLocation:  innRoom.ID,
		NewWorldID:   mainWorld.ID,
	}, nil
}

// HandlePlayerVictory processes wumpus defeat, awards pelt, and handles victory
func (sm *StateManager) HandlePlayerVictory(player *storage.Player, instanceID string) (*GameResult, error) {
	logging.Info("Processing player victory", "player", player.DisplayName, "instance", instanceID)

	// Get current stats and state
	stats := auth.GetWumpusStats(player)
	state := auth.GetWumpusState(player)

	// Update victory statistics
	stats.GamesPlayed++
	stats.GamesWon++
	stats.WumpusPelts++

	// Create wumpus pelt trophy
	wumpusPelt := items.CreateWumpusPelt(instanceID, player.DisplayName)
	peltInstance := items.GivePlayerItem(player, wumpusPelt, 1)

	// TODO: Add pelt to player's actual inventory when inventory system is implemented
	// For now, we'll track it in stats

	// Reset maze state but keep health
	state.IsInMaze = false
	state.CurrentInstance = ""

	// Reset combat state on victory
	sm.ResetCombatState(player)

	// Save updated data
	auth.UpdateWumpusStats(player, stats)
	auth.UpdateWumpusState(player, state)
	if err := sm.storageMgr.Players().Save(player); err != nil {
		return nil, fmt.Errorf("failed to save player data: %w", err)
	}

	// Get Inn location for return
	mainWorld, err := world.GetMainWorld(sm.storageMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to get main world for victory return: %w", err)
	}

	innRoom, err := world.GetInnRoom(sm.storageMgr, mainWorld.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Inn room for victory return: %w", err)
	}

	message := fmt.Sprintf("🎉 VICTORY! 🎉\n\n"+
		"You have slain the mighty Wumpus!\n"+
		"The beast falls with a thunderous crash, and you claim your prize.\n\n"+
		"You have been awarded: %s\n"+
		"Your triumph echoes through the maze before it fades away...\n\n"+
		"You find yourself back in The Inn, victorious and celebrated!\n\n"+
		"Victory Statistics:\n"+
		"- Games Won: %d\n"+
		"- Total Wumpus Pelts: %d\n"+
		"- Games Played: %d",
		wumpusPelt.Name, stats.GamesWon, stats.WumpusPelts, stats.GamesPlayed)

	return &GameResult{
		Success:      true,
		Message:      message,
		PlayerWon:    true,
		ItemAwarded:  peltInstance,
		StatsUpdated: true,
		NewLocation:  innRoom.ID,
		NewWorldID:   mainWorld.ID,
	}, nil
}

// GetPlayerHealth returns the current health of a player
func (sm *StateManager) GetPlayerHealth(player *storage.Player) int {
	state := auth.GetWumpusState(player)
	return state.Health
}

// IsPlayerAlive returns true if player health is above 0
func (sm *StateManager) IsPlayerAlive(player *storage.Player) bool {
	return sm.GetPlayerHealth(player) > 0
}

// IsPlayerInMaze returns true if player is currently in a maze instance
func (sm *StateManager) IsPlayerInMaze(player *storage.Player) bool {
	state := auth.GetWumpusState(player)
	return state.IsInMaze
}

// SetPlayerInMaze updates the player's maze status
func (sm *StateManager) SetPlayerInMaze(player *storage.Player, inMaze bool, instanceID string) error {
	state := auth.GetWumpusState(player)
	state.IsInMaze = inMaze
	if inMaze {
		state.CurrentInstance = instanceID
	} else {
		state.CurrentInstance = ""
	}

	auth.UpdateWumpusState(player, state)
	return sm.storageMgr.Players().Save(player)
}

// GetPlayerCurrentInstance returns the instance ID the player is currently in
func (sm *StateManager) GetPlayerCurrentInstance(player *storage.Player) string {
	state := auth.GetWumpusState(player)
	return state.CurrentInstance
}

// CheckWumpusEncounter handles wumpus encounter logic (passive detection only)
func (sm *StateManager) CheckWumpusEncounter(player *storage.Player, room *storage.Room) (*GameResult, error) {
	// Check if room has wumpus
	roomData, exists := room.GetProperty("wumpus_room")
	if !exists {
		return &GameResult{Success: true, Message: ""}, nil
	}

	roomMap, ok := roomData.(map[string]interface{})
	if !ok {
		return &GameResult{Success: true, Message: ""}, nil
	}

	hasWumpus, ok := roomMap["has_wumpus"].(bool)
	if !ok || !hasWumpus {
		return &GameResult{Success: true, Message: ""}, nil
	}

	// Player encountered the wumpus - just show encounter message
	// Combat is now initiated by explicit player attack commands
	logging.Info("Player encountered wumpus", "player", player.DisplayName, "room", room.ID)

	return &GameResult{
		Success: true,
		Message: "🐾 You have encountered the mighty Wumpus! 🐾\n" +
			"The beast towers before you, its eyes glowing with malevolent intelligence.\n" +
			"You can 'attack wumpus' to engage in combat, or flee to another room!\n" +
			"Choose wisely - the Wumpus is dangerous and will attack if provoked.",
	}, nil
}

// AttackWumpus handles player-initiated combat with the wumpus
func (sm *StateManager) AttackWumpus(player *storage.Player, room *storage.Room) (*GameResult, error) {
	// Check if room has wumpus
	roomData, exists := room.GetProperty("wumpus_room")
	if !exists {
		return &GameResult{
			Success: false,
			Message: "There is no wumpus here to attack.",
		}, nil
	}

	roomMap, ok := roomData.(map[string]interface{})
	if !ok {
		return &GameResult{
			Success: false,
			Message: "There is no wumpus here to attack.",
		}, nil
	}

	hasWumpus, ok := roomMap["has_wumpus"].(bool)
	if !ok || !hasWumpus {
		return &GameResult{
			Success: false,
			Message: "There is no wumpus here to attack.",
		}, nil
	}

	logging.Info("Player attacking wumpus", "player", player.DisplayName, "room", room.ID)

	// Get current player state for combat calculation
	state := auth.GetWumpusState(player)
	
	// Initialize combat state if not present
	combatState := sm.getCombatState(player)
	
	// Calculate attack damage based on player health and random factor
	rand.Seed(time.Now().UnixNano())
	baseDamage := 25 + rand.Intn(25) // 25-50 base damage
	
	// Health modifier: lower health = less effective attacks
	healthModifier := float64(state.Health) / 100.0
	if healthModifier < 0.3 {
		healthModifier = 0.3 // Minimum 30% effectiveness
	}
	
	actualDamage := int(float64(baseDamage) * healthModifier)
	combatState.TotalDamageDealt += actualDamage
	combatState.AttackCount++
	
	// Save combat state back to player
	sm.setCombatState(player, combatState)
	
	// Check if wumpus is defeated (100 HP total)
	if combatState.TotalDamageDealt >= 100 {
		// Remove wumpus from room
		roomMap["has_wumpus"] = false
		room.SetProperty("wumpus_room", roomMap)
		
		// TODO: Fire wumpus death event for AI system
		// This would notify the scripting system that the wumpus has been defeated
		
		// Player victory!
		instanceID := sm.GetPlayerCurrentInstance(player)
		return sm.HandlePlayerVictory(player, instanceID)
	}
	
	// Wumpus counter-attacks based on damage taken
	wumpusHealth := 100 - combatState.TotalDamageDealt
	counterAttackChance := 0.4 + (0.4 * (float64(100-wumpusHealth) / 100.0)) // 40-80% chance based on wumpus health
	
	if rand.Float32() < float32(counterAttackChance) {
		// Wumpus attacks back
		wumpusDamage := 20 + rand.Intn(20) // 20-40 damage
		
		// Apply wumpus damage to player
		damageResult, err := sm.DamagePlayer(player, wumpusDamage, "the enraged Wumpus")
		if err != nil {
			return nil, fmt.Errorf("failed to apply wumpus counter-attack: %w", err)
		}
		
		if damageResult.PlayerDied {
			// Player died from counter-attack
			return &GameResult{
				Success:     true,
				Message:     fmt.Sprintf("You strike the Wumpus for %d damage! (Total: %d/100)\n\n%s", actualDamage, combatState.TotalDamageDealt, damageResult.Message),
				PlayerDied:  true,
				NewLocation: damageResult.NewLocation,
				NewWorldID:  damageResult.NewWorldID,
			}, nil
		} else {
			// Player survived counter-attack
			return &GameResult{
				Success: true,
				Message: fmt.Sprintf("You strike the Wumpus for %d damage! (Wumpus Health: %d/100)\n\n"+
					"The wounded Wumpus roars in fury and counter-attacks!\n%s\n\n"+
					"The battle continues! You can 'attack wumpus' again or flee to safety.",
					actualDamage, wumpusHealth, damageResult.Message),
			}, nil
		}
	} else {
		// No counter-attack this turn
		return &GameResult{
			Success: true,
			Message: fmt.Sprintf("You strike the Wumpus for %d damage! (Wumpus Health: %d/100)\n\n"+
				"The Wumpus staggers but does not counter-attack this time.\n"+
				"You can 'attack wumpus' again or flee while you have the chance!",
				actualDamage, wumpusHealth),
		}, nil
	}
}

// CombatState tracks combat statistics for a player
type CombatState struct {
	TotalDamageDealt int `json:"total_damage_dealt"`
	AttackCount      int `json:"attack_count"`
	InCombat         bool `json:"in_combat"`
}

// getCombatState retrieves combat state from player properties
func (sm *StateManager) getCombatState(player *storage.Player) CombatState {
	// Check if combat_state exists in the properties
	if combatData, exists := player.GetProperty("wumpus_combat"); exists {
		if combatMap, ok := combatData.(map[string]interface{}); ok {
			// Handle potential type assertions safely
			totalDamage := 0
			attackCount := 0
			inCombat := false
			
			if val, ok := combatMap["total_damage_dealt"].(float64); ok {
				totalDamage = int(val)
			}
			if val, ok := combatMap["attack_count"].(float64); ok {
				attackCount = int(val)
			}
			if val, ok := combatMap["in_combat"].(bool); ok {
				inCombat = val
			}
			
			return CombatState{
				TotalDamageDealt: totalDamage,
				AttackCount:      attackCount,
				InCombat:         inCombat,
			}
		}
	}
	
	// Return default combat state
	return CombatState{
		TotalDamageDealt: 0,
		AttackCount:      0,
		InCombat:         false,
	}
}

// setCombatState saves combat state to player properties
func (sm *StateManager) setCombatState(player *storage.Player, combatState CombatState) {
	combatData := map[string]interface{}{
		"total_damage_dealt": combatState.TotalDamageDealt,
		"attack_count":       combatState.AttackCount,
		"in_combat":          combatState.InCombat,
	}
	player.SetProperty("wumpus_combat", combatData)
	
	// Save player to persist combat state
	sm.storageMgr.Players().Save(player)
}

// ResetCombatState clears combat state for a player
func (sm *StateManager) ResetCombatState(player *storage.Player) error {
	combatState := CombatState{
		TotalDamageDealt: 0,
		AttackCount:      0,
		InCombat:         false,
	}
	sm.setCombatState(player, combatState)
	return nil
}

// GetCombatState returns the current combat state for a player (public accessor)
func (sm *StateManager) GetCombatState(player *storage.Player) CombatState {
	return sm.getCombatState(player)
}

// ProcessRoomHazards checks for pits and other hazards in a room
func (sm *StateManager) ProcessRoomHazards(player *storage.Player, room *storage.Room) (*GameResult, error) {
	roomData, exists := room.GetProperty("wumpus_room")
	if !exists {
		return &GameResult{Success: true, Message: ""}, nil
	}

	roomMap, ok := roomData.(map[string]interface{})
	if !ok {
		return &GameResult{Success: true, Message: ""}, nil
	}

	// Check for pit
	hasPit, ok := roomMap["has_pit"].(bool)
	if ok && hasPit {
		logging.Info("Player fell into pit", "player", player.DisplayName, "room", room.ID)
		return sm.DamagePlayer(player, 50, "falling into a pit")
	}

	return &GameResult{Success: true, Message: ""}, nil
}

// GetSensoryMessages returns atmospheric messages based on nearby hazards
func (sm *StateManager) GetSensoryMessages(room *storage.Room, storageMgr *storage.Manager) []string {
	var messages []string

	roomData, exists := room.GetProperty("wumpus_room")
	if !exists {
		return messages
	}

	roomMap, ok := roomData.(map[string]interface{})
	if !ok {
		return messages
	}

	// Check for breeze (indicates nearby pit)
	hasBreeze, ok := roomMap["has_breeze"].(bool)
	if ok && hasBreeze {
		messages = append(messages, "You feel a cool breeze. There must be a pit nearby.")
	}

	// Check for wumpus smell (indicates nearby wumpus)
	// This would be set by the maze generation system
	hasStench, ok := roomMap["has_stench"].(bool)
	if ok && hasStench {
		messages = append(messages, "You smell something terrible. The wumpus must be close by.")
	}

	return messages
}

// CanPlayerHeal checks if a player is able to receive healing
func (sm *StateManager) CanPlayerHeal(player *storage.Player, location string) (*GameResult, error) {
	state := auth.GetWumpusState(player)
	
	// Dead players can't heal normally
	if state.Health <= 0 {
		return &GameResult{
			Success: false,
			Message: "You cannot heal while dead. Return to The Inn to respawn.",
		}, nil
	}
	
	// Players at full health don't need healing
	if state.Health >= 100 {
		return &GameResult{
			Success: false,
			Message: "You are already at full health!",
		}, nil
	}
	
	// Check if location allows healing (only Inn for now)
	if location != "inn" {
		return &GameResult{
			Success: false,
			Message: "Healing is only available in The Inn. Find your way back to safety.",
		}, nil
	}
	
	return &GameResult{
		Success: true,
		Message: "You can receive healing here.",
	}, nil
}

// GetPlayerHealthStatus returns a descriptive health status
func (sm *StateManager) GetPlayerHealthStatus(player *storage.Player) string {
	health := sm.GetPlayerHealth(player)
	
	switch {
	case health <= 0:
		return "Dead"
	case health <= 20:
		return "Critical"
	case health <= 40:
		return "Badly Wounded"
	case health <= 60:
		return "Wounded"
	case health <= 80:
		return "Injured"
	case health < 100:
		return "Healthy"
	default:
		return "Full Health"
	}
}

// RegenerateHealth slowly regenerates health over time (for Inn resting)
func (sm *StateManager) RegenerateHealth(player *storage.Player, regenAmount int) (*GameResult, error) {
	if regenAmount <= 0 {
		return nil, errors.New("regeneration amount must be positive")
	}
	
	state := auth.GetWumpusState(player)
	
	// Can't regenerate if dead
	if state.Health <= 0 {
		return &GameResult{
			Success: false,
			Message: "Dead players cannot regenerate health.",
		}, nil
	}
	
	// Can't regenerate if already at full health
	if state.Health >= 100 {
		return &GameResult{
			Success: false,
			Message: "You are already at full health.",
		}, nil
	}
	
	oldHealth := state.Health
	state.Health += regenAmount
	if state.Health > 100 {
		state.Health = 100
	}
	
	// Save updated state
	auth.UpdateWumpusState(player, state)
	if err := sm.storageMgr.Players().Save(player); err != nil {
		return nil, fmt.Errorf("failed to save player state: %w", err)
	}
	
	actualRegen := state.Health - oldHealth
	message := fmt.Sprintf("You feel better as you rest. (+%d health) Health: %d/100", actualRegen, state.Health)
	
	return &GameResult{
		Success:       true,
		Message:       message,
		HealthChanged: actualRegen > 0,
		OldHealth:     oldHealth,
		NewHealth:     state.Health,
	}, nil
}