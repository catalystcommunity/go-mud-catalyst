package game

import (
	"fmt"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

// Helper function to initialize world for testing
func initializeTestWorld(storageMgr *storage.Manager) error {
	// Create main world with Inn and Portal
	_, err := world.CreateMainWorld(storageMgr)
	return err
}

func TestNewStateManager(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)
	if sm == nil {
		t.Fatal("Expected StateManager, got nil")
	}

	if sm.storageMgr != storageMgr {
		t.Error("StateManager storage manager not set correctly")
	}
}

func TestDamagePlayer(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Initialize world for testing
	if err := initializeTestWorld(storageMgr); err != nil {
		t.Fatalf("Failed to initialize test world: %v", err)
	}

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	// Set initial health
	state := auth.WumpusState{
		Health:          100,
		IsInMaze:        false,
		CurrentInstance: "",
	}
	auth.UpdateWumpusState(player, state)

	// Save player
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	tests := []struct {
		name           string
		damage         int
		source         string
		expectedHealth int
		expectDeath    bool
		expectError    bool
	}{
		{
			name:           "Normal damage",
			damage:         25,
			source:         "test enemy",
			expectedHealth: 75,
			expectDeath:    false,
			expectError:    false,
		},
		{
			name:           "Lethal damage",
			damage:         100,
			source:         "wumpus",
			expectedHealth: 100, // Player respawns with full health after death
			expectDeath:    true,
			expectError:    false,
		},
		{
			name:           "Negative damage",
			damage:         -10,
			source:         "healing",
			expectedHealth: 0,
			expectDeath:    false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset player health for each test
			state := auth.GetWumpusState(player)
			state.Health = 100
			auth.UpdateWumpusState(player, state)
			storageMgr.Players().Save(player)

			result, err := sm.DamagePlayer(player, tt.damage, tt.source)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			// Check health
			finalState := auth.GetWumpusState(player)
			if finalState.Health != tt.expectedHealth {
				t.Errorf("Expected health %d, got %d", tt.expectedHealth, finalState.Health)
			}

			// Check death flag
			if result.PlayerDied != tt.expectDeath {
				t.Errorf("Expected PlayerDied %v, got %v", tt.expectDeath, result.PlayerDied)
			}

			// If player died, check that they were respawned properly
			if result.PlayerDied {
				if result.NewLocation == "" {
					t.Error("Expected NewLocation to be set for dead player")
				}
				if result.NewWorldID == "" {
					t.Error("Expected NewWorldID to be set for dead player")
				}
				// Health should be restored to 100 after death
				if finalState.Health != 100 {
					t.Errorf("Expected health to be restored to 100 after death, got %d", finalState.Health)
				}
			}
		})
	}
}

func TestHealPlayer(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	tests := []struct {
		name           string
		initialHealth  int
		healAmount     int
		expectedHealth int
		expectError    bool
	}{
		{
			name:           "Normal healing",
			initialHealth:  50,
			healAmount:     25,
			expectedHealth: 75,
			expectError:    false,
		},
		{
			name:           "Overheal",
			initialHealth:  90,
			healAmount:     20,
			expectedHealth: 100,
			expectError:    false,
		},
		{
			name:           "Full health healing",
			initialHealth:  100,
			healAmount:     10,
			expectedHealth: 100,
			expectError:    false,
		},
		{
			name:           "Negative healing",
			initialHealth:  50,
			healAmount:     -10,
			expectedHealth: 50,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set initial health
			state := auth.WumpusState{
				Health:          tt.initialHealth,
				IsInMaze:        false,
				CurrentInstance: "",
			}
			auth.UpdateWumpusState(player, state)
			storageMgr.Players().Save(player)

			result, err := sm.HealPlayer(player, tt.healAmount)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			// Check final health
			finalState := auth.GetWumpusState(player)
			if finalState.Health != tt.expectedHealth {
				t.Errorf("Expected health %d, got %d", tt.expectedHealth, finalState.Health)
			}

			if !result.Success {
				t.Error("Expected successful healing")
			}

			// Test enhanced GameResult tracking
			if tt.expectedHealth > tt.initialHealth {
				if !result.HealthChanged {
					t.Error("Expected HealthChanged to be true")
				}
				if result.OldHealth != tt.initialHealth {
					t.Errorf("Expected OldHealth %d, got %d", tt.initialHealth, result.OldHealth)
				}
				if result.NewHealth != tt.expectedHealth {
					t.Errorf("Expected NewHealth %d, got %d", tt.expectedHealth, result.NewHealth)
				}
			} else {
				if result.HealthChanged {
					t.Error("Expected HealthChanged to be false when no healing occurs")
				}
			}
		})
	}
}

func TestFullyHealPlayer(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	tests := []struct {
		name          string
		initialHealth int
	}{
		{"Heal from low health", 25},
		{"Already at full health", 100},
		{"Heal from critical health", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set initial health
			state := auth.WumpusState{
				Health:          tt.initialHealth,
				IsInMaze:        false,
				CurrentInstance: "",
			}
			auth.UpdateWumpusState(player, state)
			storageMgr.Players().Save(player)

			result, err := sm.FullyHealPlayer(player)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			// Check final health is always 100
			finalState := auth.GetWumpusState(player)
			if finalState.Health != 100 {
				t.Errorf("Expected health 100, got %d", finalState.Health)
			}

			if !result.Success {
				t.Error("Expected successful healing")
			}

			// Test enhanced GameResult tracking
			if tt.initialHealth < 100 {
				if !result.HealthChanged {
					t.Error("Expected HealthChanged to be true")
				}
				if result.OldHealth != tt.initialHealth {
					t.Errorf("Expected OldHealth %d, got %d", tt.initialHealth, result.OldHealth)
				}
				if result.NewHealth != 100 {
					t.Errorf("Expected NewHealth 100, got %d", result.NewHealth)
				}
			} else {
				if result.HealthChanged {
					t.Error("Expected HealthChanged to be false when already at full health")
				}
			}
		})
	}
}

func TestHandlePlayerDeath(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Initialize world for testing
	if err := initializeTestWorld(storageMgr); err != nil {
		t.Fatalf("Failed to initialize test world: %v", err)
	}

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	// Set initial state
	initialStats := auth.WumpusStats{
		GamesPlayed: 2,
		GamesWon:    1,
		WumpusPelts: 1,
		TotalDeaths: 3,
	}
	auth.UpdateWumpusStats(player, initialStats)

	initialState := auth.WumpusState{
		Health:          0,
		IsInMaze:        true,
		CurrentInstance: "test-instance-123",
	}
	auth.UpdateWumpusState(player, initialState)

	// Save player
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	result, err := sm.HandlePlayerDeath(player, "test cause")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check result flags
	if !result.Success {
		t.Error("Expected successful death handling")
	}
	if !result.PlayerDied {
		t.Error("Expected PlayerDied to be true")
	}
	if !result.StatsUpdated {
		t.Error("Expected StatsUpdated to be true")
	}
	if result.NewLocation == "" {
		t.Error("Expected NewLocation to be set")
	}
	if result.NewWorldID == "" {
		t.Error("Expected NewWorldID to be set")
	}

	// Check stats were updated
	finalStats := auth.GetWumpusStats(player)
	if finalStats.TotalDeaths != 4 {
		t.Errorf("Expected TotalDeaths 4, got %d", finalStats.TotalDeaths)
	}
	if finalStats.GamesPlayed != 3 {
		t.Errorf("Expected GamesPlayed 3, got %d", finalStats.GamesPlayed)
	}

	// Check state was reset
	finalState := auth.GetWumpusState(player)
	if finalState.Health != 100 {
		t.Errorf("Expected Health 100, got %d", finalState.Health)
	}
	if finalState.IsInMaze {
		t.Error("Expected IsInMaze to be false")
	}
	if finalState.CurrentInstance != "" {
		t.Errorf("Expected CurrentInstance to be empty, got %s", finalState.CurrentInstance)
	}
}

func TestHandlePlayerVictory(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Initialize world for testing
	if err := initializeTestWorld(storageMgr); err != nil {
		t.Fatalf("Failed to initialize test world: %v", err)
	}

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	// Set initial state
	initialStats := auth.WumpusStats{
		GamesPlayed: 2,
		GamesWon:    1,
		WumpusPelts: 1,
		TotalDeaths: 0,
	}
	auth.UpdateWumpusStats(player, initialStats)

	initialState := auth.WumpusState{
		Health:          75,
		IsInMaze:        true,
		CurrentInstance: "test-instance-123",
	}
	auth.UpdateWumpusState(player, initialState)

	// Save player
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	result, err := sm.HandlePlayerVictory(player, "test-instance-123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	// Check result flags
	if !result.Success {
		t.Error("Expected successful victory handling")
	}
	if !result.PlayerWon {
		t.Error("Expected PlayerWon to be true")
	}
	if !result.StatsUpdated {
		t.Error("Expected StatsUpdated to be true")
	}
	if result.ItemAwarded == nil {
		t.Error("Expected ItemAwarded to be set")
	}
	if result.NewLocation == "" {
		t.Error("Expected NewLocation to be set")
	}
	if result.NewWorldID == "" {
		t.Error("Expected NewWorldID to be set")
	}

	// Check stats were updated
	finalStats := auth.GetWumpusStats(player)
	if finalStats.GamesPlayed != 3 {
		t.Errorf("Expected GamesPlayed 3, got %d", finalStats.GamesPlayed)
	}
	if finalStats.GamesWon != 2 {
		t.Errorf("Expected GamesWon 2, got %d", finalStats.GamesWon)
	}
	if finalStats.WumpusPelts != 2 {
		t.Errorf("Expected WumpusPelts 2, got %d", finalStats.WumpusPelts)
	}

	// Check state was updated
	finalState := auth.GetWumpusState(player)
	if finalState.Health != 75 {
		t.Errorf("Expected Health to remain 75, got %d", finalState.Health)
	}
	if finalState.IsInMaze {
		t.Error("Expected IsInMaze to be false")
	}
	if finalState.CurrentInstance != "" {
		t.Errorf("Expected CurrentInstance to be empty, got %s", finalState.CurrentInstance)
	}
}

func TestPlayerStateQueries(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	// Test GetPlayerHealth
	state := auth.WumpusState{Health: 75, IsInMaze: false, CurrentInstance: ""}
	auth.UpdateWumpusState(player, state)

	health := sm.GetPlayerHealth(player)
	if health != 75 {
		t.Errorf("Expected health 75, got %d", health)
	}

	// Test IsPlayerAlive
	if !sm.IsPlayerAlive(player) {
		t.Error("Expected player to be alive")
	}

	state.Health = 0
	auth.UpdateWumpusState(player, state)
	if sm.IsPlayerAlive(player) {
		t.Error("Expected player to be dead")
	}

	// Test IsPlayerInMaze
	state.IsInMaze = true
	auth.UpdateWumpusState(player, state)
	if !sm.IsPlayerInMaze(player) {
		t.Error("Expected player to be in maze")
	}

	// Test SetPlayerInMaze
	err := sm.SetPlayerInMaze(player, false, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if sm.IsPlayerInMaze(player) {
		t.Error("Expected player to not be in maze")
	}

	// Test GetPlayerCurrentInstance
	err = sm.SetPlayerInMaze(player, true, "test-instance")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	instance := sm.GetPlayerCurrentInstance(player)
	if instance != "test-instance" {
		t.Errorf("Expected instance 'test-instance', got '%s'", instance)
	}
}

func TestProcessRoomHazards(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"
	state := auth.WumpusState{Health: 100, IsInMaze: true, CurrentInstance: "test"}
	auth.UpdateWumpusState(player, state)
	storageMgr.Players().Save(player)

	// Create test room with pit
	room := storage.NewRoom(ids.NewEntityID(), "test-world", "Test Room")
	room.SetProperty("wumpus_room", map[string]interface{}{
		"has_pit": true,
		"room_type": "maze",
	})

	_, err := sm.ProcessRoomHazards(player, room)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have taken damage from pit
	finalState := auth.GetWumpusState(player)
	if finalState.Health >= 100 {
		t.Error("Expected player to take damage from pit")
	}

	// Test room without hazards
	normalRoom := storage.NewRoom(ids.NewEntityID(), "test-world", "Normal Room")
	normalRoom.SetProperty("wumpus_room", map[string]interface{}{
		"room_type": "maze",
	})

	// Reset health
	state.Health = 100
	auth.UpdateWumpusState(player, state)
	storageMgr.Players().Save(player)

	_, err = sm.ProcessRoomHazards(player, normalRoom)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should not take damage
	finalState = auth.GetWumpusState(player)
	if finalState.Health != 100 {
		t.Error("Expected player to not take damage in normal room")
	}
}

func TestGetSensoryMessages(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Test room with breeze
	room := storage.NewRoom(ids.NewEntityID(), "test-world", "Breezy Room")
	room.SetProperty("wumpus_room", map[string]interface{}{
		"has_breeze": true,
		"room_type": "maze",
	})

	messages := sm.GetSensoryMessages(room, storageMgr)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if len(messages) > 0 && messages[0] != "You feel a cool breeze. There must be a pit nearby." {
		t.Errorf("Unexpected breeze message: %s", messages[0])
	}

	// Test room with stench
	stenchRoom := storage.NewRoom(ids.NewEntityID(), "test-world", "Stinky Room")
	stenchRoom.SetProperty("wumpus_room", map[string]interface{}{
		"has_stench": true,
		"room_type": "maze",
	})

	messages = sm.GetSensoryMessages(stenchRoom, storageMgr)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if len(messages) > 0 && messages[0] != "You smell something terrible. The wumpus must be close by." {
		t.Errorf("Unexpected stench message: %s", messages[0])
	}

	// Test room with both
	bothRoom := storage.NewRoom(ids.NewEntityID(), "test-world", "Dangerous Room")
	bothRoom.SetProperty("wumpus_room", map[string]interface{}{
		"has_breeze": true,
		"has_stench": true,
		"room_type": "maze",
	})

	messages = sm.GetSensoryMessages(bothRoom, storageMgr)
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
}

func TestCanPlayerHeal(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	tests := []struct {
		name         string
		health       int
		location     string
		expectSuccess bool
		expectedMsg  string
	}{
		{
			name:         "Can heal in inn with low health",
			health:       50,
			location:     "inn",
			expectSuccess: true,
			expectedMsg:  "You can receive healing here.",
		},
		{
			name:         "Cannot heal when dead",
			health:       0,
			location:     "inn",
			expectSuccess: false,
			expectedMsg:  "You cannot heal while dead. Return to The Inn to respawn.",
		},
		{
			name:         "Cannot heal at full health",
			health:       100,
			location:     "inn",
			expectSuccess: false,
			expectedMsg:  "You are already at full health!",
		},
		{
			name:         "Cannot heal outside inn",
			health:       50,
			location:     "maze",
			expectSuccess: false,
			expectedMsg:  "Healing is only available in The Inn. Find your way back to safety.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set player health
			state := auth.WumpusState{
				Health:          tt.health,
				IsInMaze:        false,
				CurrentInstance: "",
			}
			auth.UpdateWumpusState(player, state)
			storageMgr.Players().Save(player)

			result, err := sm.CanPlayerHeal(player, tt.location)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if result.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, result.Message)
			}
		})
	}
}

func TestGetPlayerHealthStatus(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	tests := []struct {
		health         int
		expectedStatus string
	}{
		{0, "Dead"},
		{10, "Critical"},
		{30, "Badly Wounded"},
		{50, "Wounded"},
		{70, "Injured"},
		{90, "Healthy"},
		{100, "Full Health"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Health %d", tt.health), func(t *testing.T) {
			// Set player health
			state := auth.WumpusState{
				Health:          tt.health,
				IsInMaze:        false,
				CurrentInstance: "",
			}
			auth.UpdateWumpusState(player, state)
			storageMgr.Players().Save(player)

			status := sm.GetPlayerHealthStatus(player)
			if status != tt.expectedStatus {
				t.Errorf("Expected status '%s', got '%s'", tt.expectedStatus, status)
			}
		})
	}
}

func TestRegenerateHealth(t *testing.T) {
	config := storage.DefaultStorageConfig()
	config.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(config)
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sm := NewStateManager(storageMgr)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
	player.DisplayName = "Test Player"

	tests := []struct {
		name           string
		initialHealth  int
		regenAmount    int
		expectedHealth int
		expectSuccess  bool
		expectError    bool
	}{
		{
			name:           "Normal regeneration",
			initialHealth:  50,
			regenAmount:    10,
			expectedHealth: 60,
			expectSuccess:  true,
			expectError:    false,
		},
		{
			name:           "Regeneration with overheal",
			initialHealth:  95,
			regenAmount:    10,
			expectedHealth: 100,
			expectSuccess:  true,
			expectError:    false,
		},
		{
			name:           "Cannot regenerate when dead",
			initialHealth:  0,
			regenAmount:    10,
			expectedHealth: 0,
			expectSuccess:  false,
			expectError:    false,
		},
		{
			name:           "Cannot regenerate at full health",
			initialHealth:  100,
			regenAmount:    5,
			expectedHealth: 100,
			expectSuccess:  false,
			expectError:    false,
		},
		{
			name:           "Invalid regeneration amount",
			initialHealth:  50,
			regenAmount:    0,
			expectedHealth: 50,
			expectSuccess:  false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set player health
			state := auth.WumpusState{
				Health:          tt.initialHealth,
				IsInMaze:        false,
				CurrentInstance: "",
			}
			auth.UpdateWumpusState(player, state)
			storageMgr.Players().Save(player)

			result, err := sm.RegenerateHealth(player, tt.regenAmount)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			// Check final health
			finalState := auth.GetWumpusState(player)
			if finalState.Health != tt.expectedHealth {
				t.Errorf("Expected health %d, got %d", tt.expectedHealth, finalState.Health)
			}

			// Check health tracking in result
			if tt.expectSuccess && result.HealthChanged {
				if result.NewHealth != tt.expectedHealth {
					t.Errorf("Expected NewHealth %d, got %d", tt.expectedHealth, result.NewHealth)
				}
				if result.OldHealth != tt.initialHealth {
					t.Errorf("Expected OldHealth %d, got %d", tt.initialHealth, result.OldHealth)
				}
			}
		})
	}
}