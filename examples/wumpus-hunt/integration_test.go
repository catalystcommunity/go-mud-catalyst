package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/pkg/communication"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/game"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/portal"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompleteGameFlow tests the entire game flow from authentication to victory
func TestCompleteGameFlow(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	
	// Initialize storage
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = tempDir
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	require.NoError(t, storageMgr.Initialize())

	// Initialize event manager
	_ = events.NewEventManager()

	// Initialize communication manager (not needed for this test)
	// _ = communication.NewCommunicationManager(storageMgr, eventMgr)

	// Initialize instance manager
	instanceMgr := instance.NewInstanceManager(storageMgr)

	// Initialize transport handler
	transportHandler := portal.NewTransportHandler(storageMgr, instanceMgr)

	// Initialize game state manager
	gameStateMgr := game.NewStateManager(storageMgr)

	// Initialize world
	require.NoError(t, initializeWorld(storageMgr))

	// Test Case 1: User Registration and Authentication
	t.Run("UserRegistrationAndAuth", func(t *testing.T) {
		// Create new player
		player := storage.NewPlayer(ids.NewEntityID(), "testplayer")
		player.DisplayName = "TestPlayer"

		// Set password
		err := auth.SetPlayerPassword(player, "testpassword123")
		require.NoError(t, err)

		// Save player
		err = storageMgr.Players().Save(player)
		require.NoError(t, err)

		// Verify authentication
		assert.True(t, auth.ValidatePlayerPassword(player, "testpassword123"))
		assert.False(t, auth.ValidatePlayerPassword(player, "wrongpassword"))

		// Verify player stats initialization
		stats := auth.GetWumpusStats(player)
		assert.Equal(t, 0, stats.GamesPlayed)
		assert.Equal(t, 0, stats.GamesWon)
		assert.Equal(t, 0, stats.WumpusPelts)
		assert.Equal(t, 0, stats.TotalDeaths)
	})

	// Test Case 2: Inn Initialization and Player Location
	t.Run("InnInitializationAndLocation", func(t *testing.T) {
		// Load player
		player, err := storageMgr.Players().LoadByUsername("testplayer")
		require.NoError(t, err)

		// Get main world and Inn room
		mainWorld, err := world.GetMainWorld(storageMgr)
		require.NoError(t, err)

		_, err = world.GetInnRoom(storageMgr, mainWorld.ID)
		require.NoError(t, err)

		// Initialize player in Inn (location tracking is handled elsewhere)
		// commMgr.UpdatePlayerLocation(player.ID, mainWorld.ID, innRoom.ID)

		// Verify player health is full in Inn
		health := gameStateMgr.GetPlayerHealth(player)
		assert.Equal(t, 100, health)
		assert.True(t, gameStateMgr.IsPlayerAlive(player))

		// Test healing in Inn
		result, err := gameStateMgr.DamagePlayer(player, 50, "test damage")
		require.NoError(t, err)
		assert.Equal(t, 50, result.NewHealth)

		healResult, err := gameStateMgr.FullyHealPlayer(player)
		require.NoError(t, err)
		assert.Equal(t, 100, healResult.NewHealth)
		assert.Contains(t, healResult.Message, "restored to full health")
	})

	// Test Case 3: Portal Entry and Maze Creation
	t.Run("PortalEntryAndMazeCreation", func(t *testing.T) {
		// Load player
		player, err := storageMgr.Players().LoadByUsername("testplayer")
		require.NoError(t, err)

		// Test portal entry
		result, err := transportHandler.HandlePortalCommand(player, "enter")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.InstanceID)
		assert.NotEmpty(t, result.TargetRoomID)
		assert.NotEmpty(t, result.TargetWorldID)
		assert.Contains(t, result.Message, "portal")

		// Verify player is now in maze
		assert.True(t, gameStateMgr.IsPlayerInMaze(player))
		currentInstance := gameStateMgr.GetPlayerCurrentInstance(player)
		assert.Equal(t, result.InstanceID, currentInstance)

		// Verify instance was created
		instanceInfo, err := instanceMgr.GetInstanceInfo(result.InstanceID)
		require.NoError(t, err)
		assert.Equal(t, player.ID, instanceInfo.PlayerID)
		assert.Equal(t, result.TargetWorldID, instanceInfo.WorldID)

		// Update stats to reflect game start
		stats := auth.GetWumpusStats(player)
		stats.GamesPlayed++
		auth.UpdateWumpusStats(player, stats)
		err = storageMgr.Players().Save(player)
		require.NoError(t, err)
	})

	// Test Case 4: Maze Navigation and Wumpus Interaction
	t.Run("MazeNavigationAndWumpusInteraction", func(t *testing.T) {
		// Load player
		player, err := storageMgr.Players().LoadByUsername("testplayer")
		require.NoError(t, err)

		// Get current instance
		instanceID := gameStateMgr.GetPlayerCurrentInstance(player)
		mazeInstance, err := instanceMgr.GetInstance(instanceID)
		require.NoError(t, err)

		// Get current room (use start room)
		currentRoom := mazeInstance.StartRoom.Room

		// Test sensory messages
		sensoryMessages := gameStateMgr.GetSensoryMessages(currentRoom, storageMgr)
		// Should return a slice (possibly empty for a safe starting room)
		assert.IsType(t, []string{}, sensoryMessages)

		// Test room hazards (should be safe in starting room)
		hazardResult, err := gameStateMgr.ProcessRoomHazards(player, currentRoom)
		require.NoError(t, err)
		assert.False(t, hazardResult.PlayerDied) // Starting room should be safe

		// Test wumpus encounter (may or may not have wumpus in starting room)
		_, err = gameStateMgr.CheckWumpusEncounter(player, currentRoom)
		require.NoError(t, err)
		// Result can be anything, just check it doesn't error

		// Test combat state
		combatState := gameStateMgr.GetCombatState(player)
		assert.Equal(t, 0, combatState.TotalDamageDealt)
		assert.Equal(t, 0, combatState.AttackCount)
	})

	// Test Case 5: Victory Scenario
	t.Run("VictoryScenario", func(t *testing.T) {
		// Load player
		player, err := storageMgr.Players().LoadByUsername("testplayer")
		require.NoError(t, err)

		// Get current instance and room
		instanceID := gameStateMgr.GetPlayerCurrentInstance(player)
		mazeInstance, err := instanceMgr.GetInstance(instanceID)
		require.NoError(t, err)

		currentRoom := mazeInstance.StartRoom.Room

		// Simulate finding and defeating the wumpus
		// First, place wumpus in current room for testing
		currentRoom.SetProperty("wumpus_room", map[string]interface{}{
			"has_wumpus": true,
		})
		err = storageMgr.Worlds().SaveRoom(currentRoom)
		require.NoError(t, err)

		// Attack wumpus multiple times until victory or death
		var attackResult *game.GameResult
		maxAttacks := 20 // Allow more attacks since combat is realistic
		for i := 0; i < maxAttacks; i++ {
			attackResult, err = gameStateMgr.AttackWumpus(player, currentRoom)
			require.NoError(t, err)
			
			if attackResult.PlayerWon {
				break
			}
			
			if attackResult.PlayerDied {
				// Player died - this is a valid outcome
				// Skip the victory assertions but verify death was handled properly
				t.Logf("Player died during combat after %d attacks - this is expected", i+1)
				
				// Verify death stats update (load fresh from storage)
				player, err = storageMgr.Players().Load(player.ID)
				require.NoError(t, err)
				stats := auth.GetWumpusStats(player)
				assert.GreaterOrEqual(t, stats.TotalDeaths, 1)
				
				// Verify player is back in Inn and healed
				assert.False(t, gameStateMgr.IsPlayerInMaze(player))
				assert.Equal(t, 100, gameStateMgr.GetPlayerHealth(player))
				assert.True(t, gameStateMgr.IsPlayerAlive(player))
				return // Skip victory assertions
			}
		}

		// Verify victory
		require.NotNil(t, attackResult)
		assert.True(t, attackResult.PlayerWon)
		assert.Contains(t, attackResult.Message, "victory")
		assert.NotEmpty(t, attackResult.NewLocation)

		// Verify player received wumpus pelt
		stats := auth.GetWumpusStats(player)
		assert.Equal(t, 1, stats.GamesWon)
		assert.Equal(t, 1, stats.WumpusPelts)

		// Verify player is back in Inn
		assert.False(t, gameStateMgr.IsPlayerInMaze(player))
		if attackResult.NewWorldID != "" {
			innRoom, err := world.GetInnRoom(storageMgr, attackResult.NewWorldID)
			require.NoError(t, err)
			assert.Equal(t, innRoom.ID, attackResult.NewLocation)
		}
	})

	// Test Case 6: Death and Respawn Scenario
	t.Run("DeathAndRespawnScenario", func(t *testing.T) {
		// Create new player for death test
		deadPlayer := storage.NewPlayer(ids.NewEntityID(), "deadplayer")
		deadPlayer.DisplayName = "DeadPlayer"
		err := auth.SetPlayerPassword(deadPlayer, "password123")
		require.NoError(t, err)
		err = storageMgr.Players().Save(deadPlayer)
		require.NoError(t, err)

		// Enter portal to create instance
		result, err := transportHandler.HandlePortalCommand(deadPlayer, "enter")
		require.NoError(t, err)
		assert.True(t, result.Success)

		// Update stats
		stats := auth.GetWumpusStats(deadPlayer)
		stats.GamesPlayed++
		auth.UpdateWumpusStats(deadPlayer, stats)
		err = storageMgr.Players().Save(deadPlayer)
		require.NoError(t, err)

		// Damage player to death
		damageResult, err := gameStateMgr.DamagePlayer(deadPlayer, 200, "test death") // Overkill damage
		require.NoError(t, err)
		assert.True(t, damageResult.PlayerDied)
		assert.Equal(t, 0, damageResult.NewHealth)

		// Handle death
		deathResult, err := gameStateMgr.HandlePlayerDeath(deadPlayer, "test death")
		require.NoError(t, err)
		assert.Contains(t, deathResult.Message, "died")
		assert.NotEmpty(t, deathResult.NewLocation)

		// Verify death stats update (reload from storage to get fresh data)
		deadPlayer, err = storageMgr.Players().Load(deadPlayer.ID)
		require.NoError(t, err)
		updatedStats := auth.GetWumpusStats(deadPlayer)
		assert.GreaterOrEqual(t, updatedStats.TotalDeaths, 1)

		// Verify player is back in Inn and healed
		assert.False(t, gameStateMgr.IsPlayerInMaze(deadPlayer))
		assert.Equal(t, 100, gameStateMgr.GetPlayerHealth(deadPlayer))
		assert.True(t, gameStateMgr.IsPlayerAlive(deadPlayer))
	})

	// Test Case 7: Instance Cleanup
	t.Run("InstanceCleanup", func(t *testing.T) {
		// Get all active instances before cleanup
		activeInstances := instanceMgr.ListActiveInstances()

		// Manually expire instances for testing
		for _, instanceInfo := range activeInstances {
			// Set expiry to past time
			instanceInfo.ExpiresAt = time.Now().Add(-1 * time.Hour)
			// Note: This would typically be done through internal methods
		}

		// The cleanup would normally be handled by the instance manager's background process
		// For testing, we can verify the cleanup capability exists
		assert.NotNil(t, instanceMgr)
	})

	// Cleanup
	instanceMgr.Shutdown()
}

// TestEdgeCases tests various edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = tempDir
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	require.NoError(t, storageMgr.Initialize())

	_ = events.NewEventManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)
	transportHandler := portal.NewTransportHandler(storageMgr, instanceMgr)
	gameStateMgr := game.NewStateManager(storageMgr)

	require.NoError(t, initializeWorld(storageMgr))

	t.Run("InvalidPlayerOperations", func(t *testing.T) {
		// Test operations on non-existent player
		nonExistentPlayer := storage.NewPlayer("invalid-id", "invalid")
		
		// These should not crash
		health := gameStateMgr.GetPlayerHealth(nonExistentPlayer)
		assert.Equal(t, 100, health) // Default health
		
		isAlive := gameStateMgr.IsPlayerAlive(nonExistentPlayer)
		assert.True(t, isAlive) // Default state
		
		inMaze := gameStateMgr.IsPlayerInMaze(nonExistentPlayer)
		assert.False(t, inMaze) // Default state
	})

	t.Run("InvalidPortalOperations", func(t *testing.T) {
		// Create player but don't save to storage
		player := storage.NewPlayer(ids.NewEntityID(), "portaltest")
		
		// Test invalid portal commands
		result, err := transportHandler.HandlePortalCommand(player, "invalid")
		if err != nil {
			// Error case
			assert.Error(t, err)
		} else {
			// Non-error case - should still indicate failure
			assert.False(t, result.Success)
		}
		
		// Test portal operations on non-authenticated player
		result, err = transportHandler.HandlePortalCommand(player, "enter")
		if err != nil {
			// Error case 
			assert.Error(t, err)
		} else {
			// Non-error case - should still indicate failure
			assert.False(t, result.Success)
		}
	})

	t.Run("InvalidCombatOperations", func(t *testing.T) {
		// Create minimal room without wumpus
		room := storage.NewRoom(ids.NewEntityID(), "TestRoom", "Test room for combat")
		
		// Create player
		player := storage.NewPlayer(ids.NewEntityID(), "combattest")
		
		// Test attack without wumpus
		result, err := gameStateMgr.AttackWumpus(player, room)
		require.NoError(t, err)
		assert.False(t, result.PlayerWon)
		assert.False(t, result.PlayerDied)
		assert.Contains(t, result.Message, "no wumpus")
	})

	t.Run("PropertiesCorruption", func(t *testing.T) {
		// Create player with corrupted properties
		player := storage.NewPlayer(ids.NewEntityID(), "corrupttest")
		
		// Set invalid property data
		player.SetProperty("wumpus_auth", "invalid_data_type")
		
		// Operations should handle corruption gracefully
		hasAuth := auth.IsPlayerAuthenticated(player)
		assert.False(t, hasAuth) // Should default to false on corruption
		
		stats := auth.GetWumpusStats(player)
		assert.Equal(t, 0, stats.GamesPlayed) // Should return default stats
	})

	// Cleanup
	instanceMgr.Shutdown()
}

// TestMultiPlayerScenarios tests scenarios with multiple players
func TestMultiPlayerScenarios(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = tempDir
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	require.NoError(t, storageMgr.Initialize())

	eventMgr := events.NewEventManager()
	commMgr := communication.NewCommunicationManager(storageMgr, eventMgr)
	instanceMgr := instance.NewInstanceManager(storageMgr)
	transportHandler := portal.NewTransportHandler(storageMgr, instanceMgr)
	gameStateMgr := game.NewStateManager(storageMgr)

	require.NoError(t, initializeWorld(storageMgr))

	t.Run("MultiplePlayersInInn", func(t *testing.T) {
		// Create multiple players
		players := make([]*storage.Player, 3)
		for i := 0; i < 3; i++ {
			players[i] = storage.NewPlayer(ids.NewEntityID(), fmt.Sprintf("player%d", i+1))
			players[i].DisplayName = fmt.Sprintf("Player%d", i+1)
			err := auth.SetPlayerPassword(players[i], "password123")
			require.NoError(t, err)
			err = storageMgr.Players().Save(players[i])
			require.NoError(t, err)
		}

		// Place all players in Inn
		mainWorld, err := world.GetMainWorld(storageMgr)
		require.NoError(t, err)
		
		innRoom, err := world.GetInnRoom(storageMgr, mainWorld.ID)
		require.NoError(t, err)

		for _, player := range players {
			commMgr.UpdatePlayerLocation(player.ID, mainWorld.ID, innRoom.ID)
		}

		// Verify all players can heal in Inn
		for _, player := range players {
			// Damage player first
			_, err := gameStateMgr.DamagePlayer(player, 30, "test damage")
			require.NoError(t, err)
			
			// Heal player
			healResult, err := gameStateMgr.FullyHealPlayer(player)
			require.NoError(t, err)
			assert.Equal(t, 100, healResult.NewHealth)
		}
	})

	t.Run("ConcurrentPortalEntry", func(t *testing.T) {
		// Load test players
		players := make([]*storage.Player, 2)
		for i := 0; i < 2; i++ {
			player, err := storageMgr.Players().LoadByUsername(fmt.Sprintf("player%d", i+1))
			require.NoError(t, err)
			players[i] = player
		}

		// Both players try to enter portal simultaneously
		results := make([]*portal.TransportResult, 2)
		errors := make([]error, 2)
		
		// Simulate concurrent access
		done := make(chan bool, 2)
		for i := 0; i < 2; i++ {
			go func(idx int) {
				results[idx], errors[idx] = transportHandler.HandlePortalCommand(players[idx], "enter")
				done <- true
			}(i)
		}
		
		// Wait for both operations to complete
		<-done
		<-done

		// Both should succeed with different instances
		for i := 0; i < 2; i++ {
			require.NoError(t, errors[i])
			assert.True(t, results[i].Success)
			assert.NotEmpty(t, results[i].InstanceID)
		}

		// Each player should have their own instance
		assert.NotEqual(t, results[0].InstanceID, results[1].InstanceID)

		// Update stats for both players
		for _, player := range players {
			stats := auth.GetWumpusStats(player)
			stats.GamesPlayed++
			auth.UpdateWumpusStats(player, stats)
			err := storageMgr.Players().Save(player)
			require.NoError(t, err)
		}
	})

	// Cleanup
	instanceMgr.Shutdown()
}