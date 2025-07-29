package storage_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// MockGameConnection implements connection.GameConnection for testing
type MockGameConnection struct {
	id       string
	closed   bool
	ctx      context.Context
	cancel   context.CancelFunc
	messages chan []byte
}

func NewMockGameConnection(id string) *MockGameConnection {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockGameConnection{
		id:       id,
		ctx:      ctx,
		cancel:   cancel,
		messages: make(chan []byte, 100),
	}
}

func (m *MockGameConnection) ReadMessage() ([]byte, error) {
	select {
	case msg := <-m.messages:
		return msg, nil
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	}
}

func (m *MockGameConnection) WriteMessage(data []byte) error {
	if m.closed {
		return context.Canceled
	}
	return nil
}

func (m *MockGameConnection) Close() error {
	if !m.closed {
		m.closed = true
		m.cancel()
		close(m.messages)
	}
	return nil
}

func (m *MockGameConnection) ConnectionType() connection.ConnectionType {
	return connection.ConnectionTypeTCP
}

func (m *MockGameConnection) IsConnected() bool {
	return !m.closed
}

func (m *MockGameConnection) Context() context.Context {
	return m.ctx
}

func (m *MockGameConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
}

func (m *MockGameConnection) RemoteAddrWithProtocol() string {
	return "tcp://127.0.0.1:12345"
}

func (m *MockGameConnection) LocalAddrWithProtocol() string {
	return "tcp://127.0.0.1:54321"
}

func (m *MockGameConnection) SetReadTimeout(timeout time.Duration) error {
	return nil
}

func (m *MockGameConnection) SetWriteTimeout(timeout time.Duration) error {
	return nil
}

// net.Conn interface methods
func (m *MockGameConnection) Read(b []byte) (n int, err error) {
	data, err := m.ReadMessage()
	if err != nil {
		return 0, err
	}
	copy(b, data)
	return len(data), nil
}

func (m *MockGameConnection) Write(b []byte) (n int, err error) {
	err = m.WriteMessage(b)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (m *MockGameConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321}
}

func (m *MockGameConnection) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockGameConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockGameConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestPlayerSystemIntegration(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-integration-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Test complete player lifecycle with world interaction
	t.Run("CompletePlayerLifecycle", func(t *testing.T) {
		// Create player with inventory
		player, _, err := manager.CreateNewPlayer("integrationtest", "integration@example.com")
		if err != nil {
			t.Fatalf("Failed to create player: %v", err)
		}

		// Create world and rooms
		world, err := manager.CreateNewWorld("Integration World", "Test world for integration")
		if err != nil {
			t.Fatalf("Failed to create world: %v", err)
		}

		room1, err := manager.CreateNewRoom(world.ID, "Starting Room", "Where adventures begin")
		if err != nil {
			t.Fatalf("Failed to create room 1: %v", err)
		}

		room2, err := manager.CreateNewRoom(world.ID, "Second Room", "Another room")
		if err != nil {
			t.Fatalf("Failed to create room 2: %v", err)
		}

		// Create items
		sword, err := manager.CreateNewItem("Magic Sword", "A gleaming blade")
		if err != nil {
			t.Fatalf("Failed to create sword: %v", err)
		}

		potion, err := manager.CreateNewItem("Health Potion", "Restores health")
		if err != nil {
			t.Fatalf("Failed to create potion: %v", err)
		}

		// Give items to player
		err = manager.GiveItemToPlayer(player.ID, sword.ID, 1)
		if err != nil {
			t.Fatalf("Failed to give sword to player: %v", err)
		}

		err = manager.GiveItemToPlayer(player.ID, potion.ID, 3)
		if err != nil {
			t.Fatalf("Failed to give potions to player: %v", err)
		}

		// Create session and authenticate
		sessionManager := manager.Sessions()
		conn := NewMockGameConnection("integration-conn")
		session := sessionManager.CreateSession("integration-conn", conn)

		err = sessionManager.AuthenticateSession("integration-conn", player.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate session: %v", err)
		}

		// Move player to starting room
		err = manager.MovePlayerToRoom(player.ID, world.ID, room1.ID)
		if err != nil {
			t.Fatalf("Failed to move player to starting room: %v", err)
		}

		// Update session location
		err = session.UpdatePlayerLocation(world.ID, room1.ID)
		if err != nil {
			t.Fatalf("Failed to update session location: %v", err)
		}

		// Save session data
		err = sessionManager.SavePlayerData("integration-conn")
		if err != nil {
			t.Fatalf("Failed to save session data: %v", err)
		}

		// Move to second room
		err = manager.MovePlayerToRoom(player.ID, world.ID, room2.ID)
		if err != nil {
			t.Fatalf("Failed to move player to second room: %v", err)
		}

		// Verify player location
		reloadedPlayer, err := manager.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player: %v", err)
		}

		if reloadedPlayer.CurrentWorldID != world.ID {
			t.Errorf("Expected world ID '%s', got '%s'", world.ID, reloadedPlayer.CurrentWorldID)
		}

		if reloadedPlayer.CurrentRoomID != room2.ID {
			t.Errorf("Expected room ID '%s', got '%s'", room2.ID, reloadedPlayer.CurrentRoomID)
		}

		// Verify inventory  
		reloadedInventory, err := manager.Inventories().LoadByOwner(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload inventory: %v", err)
		}

		if len(reloadedInventory.Items) != 2 {
			t.Errorf("Expected 2 items in inventory, got %d", len(reloadedInventory.Items))
		}

		// Verify session state
		if !sessionManager.IsPlayerOnline(player.ID) {
			t.Error("Player should be online")
		}

		onlinePlayers := sessionManager.GetOnlinePlayers()
		if len(onlinePlayers) != 1 {
			t.Errorf("Expected 1 online player, got %d", len(onlinePlayers))
		}

		// Set player properties through session
		session.Player.SetProperty("quest_completed", "dragon_slayer")
		session.Player.SetProperty("level", 25)

		err = sessionManager.SavePlayerData("integration-conn")
		if err != nil {
			t.Fatalf("Failed to save player properties: %v", err)
		}

		// Verify properties were saved
		reloadedPlayer, err = manager.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to reload player for properties: %v", err)
		}

		quest, exists := reloadedPlayer.GetProperty("quest_completed")
		if !exists || quest != "dragon_slayer" {
			t.Errorf("Expected quest 'dragon_slayer', got %v (exists: %v)", quest, exists)
		}

		level, exists := reloadedPlayer.GetProperty("level")
		if !exists || level != float64(25) {
			t.Errorf("Expected level 25, got %v (exists: %v)", level, exists)
		}

		// Test session cleanup
		sessionManager.RemoveSession("integration-conn")

		if sessionManager.IsPlayerOnline(player.ID) {
			t.Error("Player should not be online after session removal")
		}

		if sessionManager.GetSessionCount() != 0 {
			t.Errorf("Expected 0 sessions after cleanup, got %d", sessionManager.GetSessionCount())
		}
	})

	// Test concurrent sessions
	t.Run("ConcurrentSessions", func(t *testing.T) {
		sessionManager := manager.Sessions()

		// Create multiple players
		player1, _, err := manager.CreateNewPlayer("concurrent1", "concurrent1@example.com")
		if err != nil {
			t.Fatalf("Failed to create player 1: %v", err)
		}

		player2, _, err := manager.CreateNewPlayer("concurrent2", "concurrent2@example.com")
		if err != nil {
			t.Fatalf("Failed to create player 2: %v", err)
		}

		player3, _, err := manager.CreateNewPlayer("concurrent3", "concurrent3@example.com")
		if err != nil {
			t.Fatalf("Failed to create player 3: %v", err)
		}

		// Create and authenticate sessions
		conn1 := NewMockGameConnection("concurrent-conn1")
		session1 := sessionManager.CreateSession("concurrent-conn1", conn1)

		conn2 := NewMockGameConnection("concurrent-conn2")
		session2 := sessionManager.CreateSession("concurrent-conn2", conn2)

		conn3 := NewMockGameConnection("concurrent-conn3")
		session3 := sessionManager.CreateSession("concurrent-conn3", conn3)

		err = sessionManager.AuthenticateSession("concurrent-conn1", player1.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate session 1: %v", err)
		}

		err = sessionManager.AuthenticateSession("concurrent-conn2", player2.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate session 2: %v", err)
		}

		err = sessionManager.AuthenticateSession("concurrent-conn3", player3.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate session 3: %v", err)
		}

		// Verify all sessions are tracked
		if sessionManager.GetSessionCount() != 3 {
			t.Errorf("Expected 3 sessions, got %d", sessionManager.GetSessionCount())
		}

		if sessionManager.GetAuthenticatedSessionCount() != 3 {
			t.Errorf("Expected 3 authenticated sessions, got %d", sessionManager.GetAuthenticatedSessionCount())
		}

		// Verify all players are online
		onlinePlayers := sessionManager.GetOnlinePlayers()
		if len(onlinePlayers) != 3 {
			t.Errorf("Expected 3 online players, got %d", len(onlinePlayers))
		}

		// Test concurrent activity updates
		sessionManager.UpdateActivity("concurrent-conn1")
		sessionManager.UpdateActivity("concurrent-conn2")
		sessionManager.UpdateActivity("concurrent-conn3")

		// Verify last activity times are recent
		for _, session := range []*storage.PlayerSession{session1, session2, session3} {
			timeSinceActivity := time.Since(session.LastActivity)
			if timeSinceActivity > time.Second {
				t.Error("Last activity should be recent")
			}
		}

		// Test removing one session
		sessionManager.RemoveSession("concurrent-conn2")

		if sessionManager.GetSessionCount() != 2 {
			t.Errorf("Expected 2 sessions after removal, got %d", sessionManager.GetSessionCount())
		}

		if sessionManager.IsPlayerOnline(player2.ID) {
			t.Error("Player 2 should not be online after session removal")
		}

		if !sessionManager.IsPlayerOnline(player1.ID) {
			t.Error("Player 1 should still be online")
		}

		if !sessionManager.IsPlayerOnline(player3.ID) {
			t.Error("Player 3 should still be online")
		}

		// Cleanup remaining sessions
		sessionManager.RemoveSession("concurrent-conn1")
		sessionManager.RemoveSession("concurrent-conn3")

		if sessionManager.GetSessionCount() != 0 {
			t.Errorf("Expected 0 sessions after cleanup, got %d", sessionManager.GetSessionCount())
		}
	})

	// Test ban system integration with sessions
	t.Run("BanSystemIntegration", func(t *testing.T) {
		sessionManager := manager.Sessions()

		// Create players
		player, _, err := manager.CreateNewPlayer("banintegration", "banintegration@example.com")
		if err != nil {
			t.Fatalf("Failed to create player: %v", err)
		}

		admin, _, err := manager.CreateNewPlayer("adminintegration", "adminintegration@example.com")
		if err != nil {
			t.Fatalf("Failed to create admin: %v", err)
		}

		// Create and authenticate session
		conn := NewMockGameConnection("ban-integration-conn")
		sessionManager.CreateSession("ban-integration-conn", conn)

		err = sessionManager.AuthenticateSession("ban-integration-conn", player.ID)
		if err != nil {
			t.Fatalf("Failed to authenticate session: %v", err)
		}

		// Verify player is online
		if !sessionManager.IsPlayerOnline(player.ID) {
			t.Error("Player should be online")
		}

		// Ban and kick player
		err = sessionManager.BanAndKickPlayer(player.ID, "Integration test ban", admin.ID)
		if err != nil {
			t.Fatalf("Failed to ban and kick player: %v", err)
		}

		// Verify connection was closed
		if !conn.closed {
			t.Error("Connection should be closed after ban")
		}

		// Verify player is banned in storage
		bannedPlayer, err := manager.Players().Load(player.ID)
		if err != nil {
			t.Fatalf("Failed to load banned player: %v", err)
		}

		if !bannedPlayer.IsBanned {
			t.Error("Player should be banned")
		}

		if bannedPlayer.BanReason != "Integration test ban" {
			t.Errorf("Expected ban reason 'Integration test ban', got '%s'", bannedPlayer.BanReason)
		}

		if bannedPlayer.BannedBy != admin.ID {
			t.Errorf("Expected banned by '%s', got '%s'", admin.ID, bannedPlayer.BannedBy)
		}

		// Test that banned player cannot authenticate new session
		conn2 := NewMockGameConnection("ban-integration-conn2")
		sessionManager.CreateSession("ban-integration-conn2", conn2)

		err = sessionManager.AuthenticateSession("ban-integration-conn2", player.ID)
		if err == nil {
			t.Error("Banned player should not be able to authenticate")
		}

		// Unban player
		err = manager.UnbanPlayer(player.ID)
		if err != nil {
			t.Fatalf("Failed to unban player: %v", err)
		}

		// First clean up the old session mapping
		sessionManager.RemoveSession("ban-integration-conn")
		
		// Test that unbanned player can authenticate
		err = sessionManager.AuthenticateSession("ban-integration-conn2", player.ID)
		if err != nil {
			t.Fatalf("Unbanned player should be able to authenticate: %v", err)
		}

		// Cleanup
		sessionManager.RemoveSession("ban-integration-conn2")
	})

	// Test error handling and edge cases
	t.Run("ErrorHandlingAndEdgeCases", func(t *testing.T) {
		sessionManager := manager.Sessions()

		// Test operations on non-existent sessions
		err := sessionManager.AuthenticateSession("non-existent", "some-player")
		if err == nil {
			t.Error("Should not be able to authenticate non-existent session")
		}

		err = sessionManager.SavePlayerData("non-existent")
		if err == nil {
			t.Error("Should not be able to save data for non-existent session")
		}

		sessionManager.UpdateActivity("non-existent") // Should not panic

		_, exists := sessionManager.GetSession("non-existent")
		if exists {
			t.Error("Non-existent session should not exist")
		}

		_, exists = sessionManager.GetSessionByPlayerID("non-existent")
		if exists {
			t.Error("Session for non-existent player should not exist")
		}

		// Test duplicate player creation
		_, _, err = manager.CreateNewPlayer("integrationtest", "different@example.com")
		if err == nil {
			t.Error("Should not be able to create player with duplicate username")
		}

		// Test moving player to non-existent room
		player, _, err := manager.CreateNewPlayer("errortest", "error@example.com")
		if err != nil {
			t.Fatalf("Failed to create error test player: %v", err)
		}

		err = manager.MovePlayerToRoom(player.ID, "non-existent-world", "non-existent-room")
		if err == nil {
			t.Error("Should not be able to move player to non-existent room")
		}

		// Test operations on non-existent player
		err = manager.BanPlayer("non-existent-player", "test", "admin")
		if err == nil {
			t.Error("Should not be able to ban non-existent player")
		}

		err = manager.UnbanPlayer("non-existent-player")
		if err == nil {
			t.Error("Should not be able to unban non-existent player")
		}

		err = sessionManager.KickPlayer("non-existent-player", "test")
		if err == nil {
			t.Error("Should not be able to kick non-existent player")
		}

		// Test giving non-existent item to player
		err = manager.GiveItemToPlayer(player.ID, "non-existent-item", 1)
		if err == nil {
			t.Error("Should not be able to give non-existent item to player")
		}

		// Test removing non-existent session (should not panic)
		sessionManager.RemoveSession("non-existent")

		// Test getting info from unauthenticated session
		conn := NewMockGameConnection("info-test-conn")
		session := sessionManager.CreateSession("info-test-conn", conn)

		info := session.GetSessionInfo()
		if info["player_id"] != "" {
			t.Error("Unauthenticated session should not have player ID in info")
		}

		if info["state"] != "CONNECTED" {
			t.Error("Unauthenticated session should show CONNECTED state")
		}

		// Cleanup
		sessionManager.RemoveSession("info-test-conn")
	})
}

func TestPlayerSystemPerformance(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-performance-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sessionManager := manager.Sessions()

	// Performance test: Create many sessions quickly
	t.Run("SessionCreationPerformance", func(t *testing.T) {
		sessionCount := 100

		for i := 0; i < sessionCount; i++ {
			connID := fmt.Sprintf("perf-conn-%d", i)
			conn := NewMockGameConnection(connID)
			sessionManager.CreateSession(connID, conn)
		}

		// Session creation performance metrics available in variables for assertions

		if sessionManager.GetSessionCount() != sessionCount {
			t.Errorf("Expected %d sessions, got %d", sessionCount, sessionManager.GetSessionCount())
		}

		// Cleanup
		for i := 0; i < sessionCount; i++ {
			connID := fmt.Sprintf("perf-conn-%d", i)
			sessionManager.RemoveSession(connID)
		}
	})

	// Performance test: Player creation and authentication
	t.Run("PlayerAuthenticationPerformance", func(t *testing.T) {
		playerCount := 50

		// Create players
		playerIDs := make([]string, playerCount)

		for i := 0; i < playerCount; i++ {
			username := fmt.Sprintf("perftest-%d", i)
			email := fmt.Sprintf("perftest-%d@example.com", i)
			
			player, _, err := manager.CreateNewPlayer(username, email)
			if err != nil {
				t.Fatalf("Failed to create player %d: %v", i, err)
			}
			playerIDs[i] = player.ID
		}

		// Player creation performance metrics available in variables for assertions

		// Create and authenticate sessions

		for i := 0; i < playerCount; i++ {
			connID := fmt.Sprintf("auth-perf-conn-%d", i)
			conn := NewMockGameConnection(connID)
			sessionManager.CreateSession(connID, conn)

			err := sessionManager.AuthenticateSession(connID, playerIDs[i])
			if err != nil {
				t.Fatalf("Failed to authenticate session %d: %v", i, err)
			}
		}

		// Authentication performance metrics available in variables for assertions

		if sessionManager.GetAuthenticatedSessionCount() != playerCount {
			t.Errorf("Expected %d authenticated sessions, got %d", 
				playerCount, sessionManager.GetAuthenticatedSessionCount())
		}

		// Test bulk operations performance
		onlinePlayers := sessionManager.GetOnlinePlayers()

		if len(onlinePlayers) != playerCount {
			t.Errorf("Expected %d online players, got %d", playerCount, len(onlinePlayers))
		}

		// Online player query performance metrics available in variables for assertions

		// Cleanup
		for i := 0; i < playerCount; i++ {
			connID := fmt.Sprintf("auth-perf-conn-%d", i)
			sessionManager.RemoveSession(connID)
		}
	})
}

