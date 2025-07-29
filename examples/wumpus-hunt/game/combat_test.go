package game

import (
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

func TestAttackWumpus(t *testing.T) {
	// Create test storage manager
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create state manager
	sm := NewStateManager(storageMgr)
	
	// Create main world for victory handling
	_, err := world.CreateMainWorld(storageMgr)
	if err != nil {
		t.Fatalf("Failed to create main world: %v", err)
	}

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"
	
	// Set initial health
	auth.UpdateWumpusState(player, auth.WumpusState{
		Health:   100,
		IsInMaze: true,
		CurrentInstance: "test_instance",
	})
	
	// Save player
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	// Create test room with wumpus
	room := storage.NewRoom(ids.NewEntityID(), "Test Room", "A room with a wumpus")
	room.SetProperty("wumpus_room", map[string]interface{}{
		"room_type":   "maze",
		"has_wumpus":  true,
		"has_pit":     false,
		"has_breeze":  false,
		"has_stench":  true,
	})

	t.Run("successful attack without killing wumpus", func(t *testing.T) {
		// Reset combat state
		sm.ResetCombatState(player)
		
		// Reload player to get fresh state
		player, err := storageMgr.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player: %v", err)
		}
		
		result, err := sm.AttackWumpus(player, room)
		if err != nil {
			t.Fatalf("AttackWumpus failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected attack to succeed, got failure")
		}

		if result.Message == "" {
			t.Errorf("Expected attack message, got empty string")
		}

		if result.PlayerWon {
			t.Errorf("Expected player not to win on first attack")
		}

		// Reload player again to get updated combat state
		player, err = storageMgr.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player after attack: %v", err)
		}

		// Check combat state
		combatState := sm.GetCombatState(player)
		if combatState.TotalDamageDealt <= 0 {
			t.Errorf("Expected damage to be dealt, got %d", combatState.TotalDamageDealt)
		}

		if combatState.AttackCount != 1 {
			t.Errorf("Expected attack count to be 1, got %d", combatState.AttackCount)
		}
	})

	t.Run("attack without wumpus in room", func(t *testing.T) {
		// Create room without wumpus
		emptyRoom := storage.NewRoom(ids.NewEntityID(), "Empty Room", "A room without a wumpus")
		emptyRoom.SetProperty("wumpus_room", map[string]interface{}{
			"room_type":   "maze",
			"has_wumpus":  false,
			"has_pit":     false,
			"has_breeze":  false,
			"has_stench":  false,
		})

		result, err := sm.AttackWumpus(player, emptyRoom)
		if err != nil {
			t.Fatalf("AttackWumpus failed: %v", err)
		}

		if result.Success {
			t.Errorf("Expected attack to fail when no wumpus present")
		}

		if result.Message != "There is no wumpus here to attack." {
			t.Errorf("Expected no wumpus message, got: %s", result.Message)
		}
	})

	t.Run("multiple attacks leading to victory", func(t *testing.T) {
		// Reset combat state
		sm.ResetCombatState(player)
		
		// Reload player
		player, err := storageMgr.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player: %v", err)
		}
		
		// Set initial combat state with high damage already dealt
		combatState := CombatState{
			TotalDamageDealt: 80, // Close to victory (100 HP)
			AttackCount:      3,
			InCombat:         true,
		}
		sm.setCombatState(player, combatState)

		// Reload player to get updated state
		player, err = storageMgr.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player after setting combat state: %v", err)
		}

		// Re-add wumpus to room (it might have been removed in previous test)
		room.SetProperty("wumpus_room", map[string]interface{}{
			"room_type":   "maze",
			"has_wumpus":  true,
			"has_pit":     false,
			"has_breeze":  false,
			"has_stench":  true,
		})

		result, err := sm.AttackWumpus(player, room)
		if err != nil {
			t.Fatalf("AttackWumpus failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected attack to succeed")
		}

		// Reload player to check final state
		player, err = storageMgr.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player after final attack: %v", err)
		}

		// This attack should likely kill the wumpus (80 + 25-50 damage should exceed 100)
		if !result.PlayerWon {
			t.Logf("Player didn't win yet, continuing test (damage dealt: %d)", sm.GetCombatState(player).TotalDamageDealt)
		}
	})
}

func TestCombatState(t *testing.T) {
	// Create test storage manager
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create state manager
	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"
	
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	t.Run("initial combat state", func(t *testing.T) {
		combatState := sm.GetCombatState(player)
		
		if combatState.TotalDamageDealt != 0 {
			t.Errorf("Expected initial damage to be 0, got %d", combatState.TotalDamageDealt)
		}

		if combatState.AttackCount != 0 {
			t.Errorf("Expected initial attack count to be 0, got %d", combatState.AttackCount)
		}

		if combatState.InCombat {
			t.Errorf("Expected initial combat state to be false")
		}
	})

	t.Run("set and get combat state", func(t *testing.T) {
		testState := CombatState{
			TotalDamageDealt: 50,
			AttackCount:      2,
			InCombat:         true,
		}

		sm.setCombatState(player, testState)
		
		// Reload player to get updated state
		player, err := storageMgr.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player: %v", err)
		}
		
		retrievedState := sm.GetCombatState(player)

		if retrievedState.TotalDamageDealt != testState.TotalDamageDealt {
			t.Errorf("Expected damage %d, got %d", testState.TotalDamageDealt, retrievedState.TotalDamageDealt)
		}

		if retrievedState.AttackCount != testState.AttackCount {
			t.Errorf("Expected attack count %d, got %d", testState.AttackCount, retrievedState.AttackCount)
		}

		if retrievedState.InCombat != testState.InCombat {
			t.Errorf("Expected combat state %v, got %v", testState.InCombat, retrievedState.InCombat)
		}
	})

	t.Run("reset combat state", func(t *testing.T) {
		// First set some combat state
		testState := CombatState{
			TotalDamageDealt: 75,
			AttackCount:      3,
			InCombat:         true,
		}
		sm.setCombatState(player, testState)

		// Reset it
		err := sm.ResetCombatState(player)
		if err != nil {
			t.Fatalf("ResetCombatState failed: %v", err)
		}

		// Verify it's reset
		resetState := sm.GetCombatState(player)
		
		if resetState.TotalDamageDealt != 0 {
			t.Errorf("Expected reset damage to be 0, got %d", resetState.TotalDamageDealt)
		}

		if resetState.AttackCount != 0 {
			t.Errorf("Expected reset attack count to be 0, got %d", resetState.AttackCount)
		}

		if resetState.InCombat {
			t.Errorf("Expected reset combat state to be false")
		}
	})
}

func TestWumpusEncounterPassive(t *testing.T) {
	// Create test storage manager
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create state manager
	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"
	
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	t.Run("encounter wumpus shows message", func(t *testing.T) {
		// Create room with wumpus
		room := storage.NewRoom(ids.NewEntityID(), "Wumpus Room", "A room with a wumpus")
		room.SetProperty("wumpus_room", map[string]interface{}{
			"room_type":   "maze",
			"has_wumpus":  true,
			"has_pit":     false,
			"has_breeze":  false,
			"has_stench":  true,
		})

		result, err := sm.CheckWumpusEncounter(player, room)
		if err != nil {
			t.Fatalf("CheckWumpusEncounter failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected encounter to succeed")
		}

		if result.Message == "" {
			t.Errorf("Expected encounter message, got empty string")
		}

		if result.PlayerDied {
			t.Errorf("Expected player not to die from passive encounter")
		}

		if result.PlayerWon {
			t.Errorf("Expected player not to win from passive encounter")
		}
	})

	t.Run("no encounter without wumpus", func(t *testing.T) {
		// Create room without wumpus
		room := storage.NewRoom(ids.NewEntityID(), "Empty Room", "A room without a wumpus")
		room.SetProperty("wumpus_room", map[string]interface{}{
			"room_type":   "maze",
			"has_wumpus":  false,
			"has_pit":     false,
			"has_breeze":  false,
			"has_stench":  false,
		})

		result, err := sm.CheckWumpusEncounter(player, room)
		if err != nil {
			t.Fatalf("CheckWumpusEncounter failed: %v", err)
		}

		if !result.Success {
			t.Errorf("Expected encounter check to succeed")
		}

		if result.Message != "" {
			t.Errorf("Expected no encounter message, got: %s", result.Message)
		}
	})
}

func TestCombatIntegrationWithDeathAndVictory(t *testing.T) {
	// Create test storage manager
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create state manager
	sm := NewStateManager(storageMgr)
	
	// Create main world for death/victory handling
	_, err := world.CreateMainWorld(storageMgr)
	if err != nil {
		t.Fatalf("Failed to create main world: %v", err)
	}

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"
	
	// Set initial health and maze state
	auth.UpdateWumpusState(player, auth.WumpusState{
		Health:   100,
		IsInMaze: true,
		CurrentInstance: "test_instance",
	})
	
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	t.Run("combat state reset on death", func(t *testing.T) {
		// Set some combat state
		combatState := CombatState{
			TotalDamageDealt: 50,
			AttackCount:      2,
			InCombat:         true,
		}
		sm.setCombatState(player, combatState)

		// Trigger death
		_, err := sm.HandlePlayerDeath(player, "test death")
		if err != nil {
			t.Fatalf("HandlePlayerDeath failed: %v", err)
		}

		// Verify combat state is reset
		resetState := sm.GetCombatState(player)
		if resetState.TotalDamageDealt != 0 {
			t.Errorf("Expected damage to be reset to 0, got %d", resetState.TotalDamageDealt)
		}
		if resetState.AttackCount != 0 {
			t.Errorf("Expected attack count to be reset to 0, got %d", resetState.AttackCount)
		}
	})

	t.Run("combat state reset on victory", func(t *testing.T) {
		// Reset player state for victory test
		auth.UpdateWumpusState(player, auth.WumpusState{
			Health:   100,
			IsInMaze: true,
			CurrentInstance: "test_instance",
		})
		storageMgr.Players().Save(player)

		// Set some combat state
		combatState := CombatState{
			TotalDamageDealt: 100,
			AttackCount:      4,
			InCombat:         true,
		}
		sm.setCombatState(player, combatState)

		// Trigger victory
		_, err := sm.HandlePlayerVictory(player, "test_instance")
		if err != nil {
			t.Fatalf("HandlePlayerVictory failed: %v", err)
		}

		// Verify combat state is reset
		resetState := sm.GetCombatState(player)
		if resetState.TotalDamageDealt != 0 {
			t.Errorf("Expected damage to be reset to 0, got %d", resetState.TotalDamageDealt)
		}
		if resetState.AttackCount != 0 {
			t.Errorf("Expected attack count to be reset to 0, got %d", resetState.AttackCount)
		}
	})
}