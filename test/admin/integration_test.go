package admin

import (
	"fmt"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/admin"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// TestAdminCommandIntegration tests admin functionality with the command system
func TestAdminCommandIntegration(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Create storage manager with test directory
	config := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(config)
	if err := storageManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create event manager
	eventManager := events.NewEventManager()
	
	// Create admin manager
	adminManager := admin.NewAdminManager(storageManager, eventManager)
	
	// Create test players
	player1, _, err := storageManager.CreateNewPlayer("admin", "admin@test.com")
	if err != nil {
		t.Fatalf("Failed to create admin player: %v", err)
	}
	
	player2, _, err := storageManager.CreateNewPlayer("target", "target@test.com")
	if err != nil {
		t.Fatalf("Failed to create target player: %v", err)
	}
	
	// Create sessions for both players
	adminConn := NewMockGameConnection()
	targetConn := NewMockGameConnection()
	
	adminSession := storageManager.Sessions().CreateSession("admin-conn", adminConn)
	targetSession := storageManager.Sessions().CreateSession("target-conn", targetConn)
	
	err = storageManager.Sessions().AuthenticateSession("admin-conn", player1.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate admin session: %v", err)
	}
	
	err = storageManager.Sessions().AuthenticateSession("target-conn", player2.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate target session: %v", err)
	}
	
	// Test admin operations through admin manager
	t.Run("KickOperation", func(t *testing.T) {
		// Simulate admin issuing kick command
		err := adminManager.KickPlayer(player1.ID, player2.ID, "test kick reason")
		if err != nil {
			t.Errorf("Failed to kick player through admin system: %v", err)
		}
		
		// Verify target connection was closed
		if !targetConn.Closed {
			t.Error("Target connection should have been closed after kick")
		}
	})
	
	t.Run("BanOperation", func(t *testing.T) {
		// Create new player for ban test
		player3, _, err := storageManager.CreateNewPlayer("banTarget", "ban@test.com")
		if err != nil {
			t.Fatalf("Failed to create ban target player: %v", err)
		}
		
		banConn := NewMockGameConnection()
		banSession := storageManager.Sessions().CreateSession("ban-conn", banConn)
		err = storageManager.Sessions().AuthenticateSession("ban-conn", player3.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate ban target session: %v", err)
		}
		
		// Simulate admin issuing ban command
		err = adminManager.BanPlayer(player1.ID, player3.ID, "test ban reason")
		if err != nil {
			t.Errorf("Failed to ban player through admin system: %v", err)
		}
		
		// Verify player is banned and connection closed
		updatedPlayer, err := storageManager.Players().Load(player3.ID)
		if err != nil {
			t.Fatalf("Failed to reload banned player: %v", err)
		}
		
		if !updatedPlayer.IsBanned {
			t.Error("Player should be banned")
		}
		
		if !banConn.Closed {
			t.Error("Banned player connection should have been closed")
		}
		
		// Clean up
		storageManager.Sessions().RemoveSession(banSession.ConnectionID)
	})
	
	// Clean up
	storageManager.Sessions().RemoveSession(adminSession.ConnectionID)
	storageManager.Sessions().RemoveSession(targetSession.ConnectionID)
}

// TestAdminWorldIntegration tests admin functionality with world/room management
func TestAdminWorldIntegration(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Create storage manager with test directory
	config := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(config)
	if err := storageManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create event manager
	eventManager := events.NewEventManager()
	
	// Create admin manager
	adminManager := admin.NewAdminManager(storageManager, eventManager)
	
	// Create test world and room
	world, err := storageManager.CreateNewWorld("testworld", "Test World")
	if err != nil {
		t.Fatalf("Failed to create test world: %v", err)
	}
	
	room, err := storageManager.CreateNewRoom(world.ID, "testroom", "Test Room")
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}
	
	// Create test players
	admin1, _, err := storageManager.CreateNewPlayer("admin1", "admin1@test.com")
	if err != nil {
		t.Fatalf("Failed to create admin player: %v", err)
	}
	
	player1, _, err := storageManager.CreateNewPlayer("player1", "player1@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Move players to the room
	err = storageManager.MovePlayerToRoom(admin1.ID, world.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to move admin to room: %v", err)
	}
	
	err = storageManager.MovePlayerToRoom(player1.ID, world.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to move player to room: %v", err)
	}
	
	// Test admin actions with world system
	t.Run("AdminMovePlayerBetweenRooms", func(t *testing.T) {
		// Create another room
		room2, err := storageManager.CreateNewRoom(world.ID, "room2", "Second Room")
		if err != nil {
			t.Fatalf("Failed to create second room: %v", err)
		}
		
		// Create new player for move test
		player2, _, err := storageManager.CreateNewPlayer("player2", "player2@test.com")
		if err != nil {
			t.Fatalf("Failed to create move test player: %v", err)
		}
		
		// Move player to first room
		err = storageManager.MovePlayerToRoom(player2.ID, world.ID, room.ID)
		if err != nil {
			t.Fatalf("Failed to move player to first room: %v", err)
		}
		
		// Admin moves player to second room
		err = adminManager.MovePlayer(admin1.ID, player2.ID, world.ID, room2.ID, "Administrative relocation")
		if err != nil {
			t.Errorf("Failed to move player: %v", err)
		}
		
		// Verify player location
		updatedPlayer, err := storageManager.Players().Load(player2.ID)
		if err != nil {
			t.Fatalf("Failed to reload moved player: %v", err)
		}
		
		if updatedPlayer.CurrentRoomID != room2.ID {
			t.Errorf("Player should be in room2, but is in %s", updatedPlayer.CurrentRoomID)
		}
	})
	
	t.Run("AdminMovePlayerBetweenWorlds", func(t *testing.T) {
		// Create another world and room
		world2, err := storageManager.CreateNewWorld("world2", "Second World")
		if err != nil {
			t.Fatalf("Failed to create second world: %v", err)
		}
		
		room3, err := storageManager.CreateNewRoom(world2.ID, "room3", "Third Room")
		if err != nil {
			t.Fatalf("Failed to create third room: %v", err)
		}
		
		// Admin moves player between worlds
		err = adminManager.MovePlayer(admin1.ID, player1.ID, world2.ID, room3.ID, "Cross-world relocation")
		if err != nil {
			t.Errorf("Failed to move player between worlds: %v", err)
		}
		
		// Verify player location
		updatedPlayer, err := storageManager.Players().Load(player1.ID)
		if err != nil {
			t.Fatalf("Failed to reload moved player: %v", err)
		}
		
		if updatedPlayer.CurrentWorldID != world2.ID {
			t.Errorf("Player should be in world2, but is in %s", updatedPlayer.CurrentWorldID)
		}
		
		if updatedPlayer.CurrentRoomID != room3.ID {
			t.Errorf("Player should be in room3, but is in %s", updatedPlayer.CurrentRoomID)
		}
	})
}

// TestAdminEventSystemIntegration tests comprehensive event system integration
func TestAdminEventSystemIntegration(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Create storage manager with test directory
	config := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(config)
	if err := storageManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create event manager
	eventManager := events.NewEventManager()
	
	// Track all admin events
	var adminEvents []string
	var eventData []map[string]interface{}
	
	// Register comprehensive event hooks
	adminEventTypes := []string{
		"admin.kick.pre", "admin.kick.post",
		"admin.ban.pre", "admin.ban.post",
		"admin.unban.pre", "admin.unban.post",
		"admin.move.pre", "admin.move.post",
		"admin.give.pre", "admin.give.post",
		"admin.take.pre", "admin.take.post",
		"admin.notify.pre", "admin.notify.post",
		"admin.log",
	}
	
	for _, eventType := range adminEventTypes {
		eventType := eventType // Capture for closure
		eventManager.RegisterHook(events.EventType(eventType), func(event events.Event) events.EventResult {
			adminEvents = append(adminEvents, eventType)
			eventData = append(eventData, event.Data())
			return events.EventResultContinue
		}, 0)
	}
	
	// Create admin manager
	adminManager := admin.NewAdminManager(storageManager, eventManager)
	
	// Create test players
	adminPlayer, _, err := storageManager.CreateNewPlayer("eventadmin", "eventadmin@test.com")
	if err != nil {
		t.Fatalf("Failed to create admin player: %v", err)
	}
	
	targetPlayer, _, err := storageManager.CreateNewPlayer("eventtarget", "eventtarget@test.com")
	if err != nil {
		t.Fatalf("Failed to create target player: %v", err)
	}
	
	// Create session for target player
	targetConn := NewMockGameConnection()
	targetSession := storageManager.Sessions().CreateSession("event-target-conn", targetConn)
	err = storageManager.Sessions().AuthenticateSession("event-target-conn", targetPlayer.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate target session: %v", err)
	}
	
	// Test various admin actions and verify events
	t.Run("KickWithEvents", func(t *testing.T) {
		adminEvents = []string{}
		eventData = []map[string]interface{}{}
		
		err := adminManager.KickPlayer(adminPlayer.ID, targetPlayer.ID, "Event test kick")
		if err != nil {
			t.Errorf("Failed to kick player: %v", err)
		}
		
		// Verify events were triggered
		expectedEvents := []string{"admin.kick.pre", "admin.kick.post", "admin.log"}
		if len(adminEvents) < len(expectedEvents) {
			t.Errorf("Expected at least %d events, got %d: %v", len(expectedEvents), len(adminEvents), adminEvents)
		}
		
		// Verify event data contains expected fields
		for i, data := range eventData {
			if i >= len(adminEvents) {
				break
			}
			
			// All admin events should have admin_id and action
			if _, hasAdminID := data["admin_id"]; !hasAdminID {
				t.Errorf("Event %s missing admin_id", adminEvents[i])
			}
			
			if _, hasAction := data["action"]; !hasAction {
				t.Errorf("Event %s missing action", adminEvents[i])
			}
		}
	})
	
	t.Run("BanWithEvents", func(t *testing.T) {
		// Create new target since previous one was kicked
		targetPlayer2, _, err := storageManager.CreateNewPlayer("eventtarget2", "eventtarget2@test.com")
		if err != nil {
			t.Fatalf("Failed to create second target player: %v", err)
		}
		
		targetConn2 := NewMockGameConnection()
		targetSession2 := storageManager.Sessions().CreateSession("event-target-conn-2", targetConn2)
		err = storageManager.Sessions().AuthenticateSession("event-target-conn-2", targetPlayer2.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate second target session: %v", err)
		}
		
		adminEvents = []string{}
		eventData = []map[string]interface{}{}
		
		err = adminManager.BanPlayer(adminPlayer.ID, targetPlayer2.ID, "Event test ban")
		if err != nil {
			t.Errorf("Failed to ban player: %v", err)
		}
		
		// Verify ban events were triggered
		hasBanPre := false
		hasBanPost := false
		hasLog := false
		
		for _, event := range adminEvents {
			switch event {
			case "admin.ban.pre":
				hasBanPre = true
			case "admin.ban.post":
				hasBanPost = true
			case "admin.log":
				hasLog = true
			}
		}
		
		if !hasBanPre {
			t.Error("Missing admin.ban.pre event")
		}
		if !hasBanPost {
			t.Error("Missing admin.ban.post event")
		}
		if !hasLog {
			t.Error("Missing admin.log event")
		}
		
		// Clean up
		storageManager.Sessions().RemoveSession(targetSession2.ConnectionID)
	})
	
	// Clean up
	storageManager.Sessions().RemoveSession(targetSession.ConnectionID)
}

// TestAdminPerformanceIntegration tests admin functionality under load
func TestAdminPerformanceIntegration(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Create storage manager with test directory
	config := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(config)
	if err := storageManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create event manager
	eventManager := events.NewEventManager()
	
	// Create admin manager
	adminManager := admin.NewAdminManager(storageManager, eventManager)
	
	// Create admin player
	adminPlayer, _, err := storageManager.CreateNewPlayer("perfadmin", "perfadmin@test.com")
	if err != nil {
		t.Fatalf("Failed to create admin player: %v", err)
	}
	
	// Create multiple test players
	const numPlayers = 100
	players := make([]*storage.Player, numPlayers)
	
	for i := 0; i < numPlayers; i++ {
		player, _, err := storageManager.CreateNewPlayer(fmt.Sprintf("player%d", i), fmt.Sprintf("player%d@test.com", i))
		if err != nil {
			t.Fatalf("Failed to create player %d: %v", i, err)
		}
		players[i] = player
	}
	
	// Test bulk admin operations
	t.Run("BulkPlayerInfo", func(t *testing.T) {
		start := time.Now()
		
		for _, player := range players {
			_, err := adminManager.GetPlayerInfo(player.ID)
			if err != nil {
				t.Errorf("Failed to get info for player %s: %v", player.ID, err)
			}
		}
		
		duration := time.Since(start)
		// Performance metrics available in variables for assertions
		
		// Should be reasonably fast
		if duration > time.Second {
			t.Errorf("Bulk player info took too long: %v", duration)
		}
	})
	
	t.Run("BulkBanOperations", func(t *testing.T) {
		start := time.Now()
		
		// Ban half the players
		for i := 0; i < numPlayers/2; i++ {
			err := adminManager.BanPlayer(adminPlayer.ID, players[i].ID, fmt.Sprintf("Bulk ban test %d", i))
			if err != nil {
				t.Errorf("Failed to ban player %s: %v", players[i].ID, err)
			}
		}
		
		duration := time.Since(start)
		// Banning performance metrics available in variables for assertions
		
		// Should be reasonably fast
		if duration > 2*time.Second {
			t.Errorf("Bulk ban operations took too long: %v", duration)
		}
	})
	
	t.Run("BulkUnbanOperations", func(t *testing.T) {
		start := time.Now()
		
		// Unban the players we just banned
		for i := 0; i < numPlayers/2; i++ {
			err := adminManager.UnbanPlayer(adminPlayer.ID, players[i].ID)
			if err != nil {
				t.Errorf("Failed to unban player %s: %v", players[i].ID, err)
			}
		}
		
		duration := time.Since(start)
		// Unbanning performance metrics available in variables for assertions
		
		// Should be reasonably fast
		if duration > 2*time.Second {
			t.Errorf("Bulk unban operations took too long: %v", duration)
		}
	})
}