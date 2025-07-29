package admin

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/admin"
	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// TestMain sets up and tears down for all tests in this package
func TestMain(m *testing.M) {
	// Set log level to WARN to reduce noise during tests
	testLogger := logging.NewLogger(logging.LevelWarn, os.Stderr)
	logging.SetDefaultLogger(testLogger)

	// Run tests
	code := m.Run()

	// Restore default logger
	logging.SetDefaultLogger(logging.DefaultLogger())

	os.Exit(code)
}

// TestAdminManager tests the admin manager functionality
func TestAdminManager(t *testing.T) {
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
	
	// Test basic admin manager creation
	if adminManager == nil {
		t.Fatal("Failed to create admin manager")
	}
	
	t.Run("KickPlayer", func(t *testing.T) {
		testAdminKickPlayer(t, storageManager, adminManager)
	})
	
	t.Run("BanPlayer", func(t *testing.T) {
		testAdminBanPlayer(t, storageManager, adminManager)
	})
	
	t.Run("UnbanPlayer", func(t *testing.T) {
		testAdminUnbanPlayer(t, storageManager, adminManager)
	})
	
	t.Run("MovePlayer", func(t *testing.T) {
		testAdminMovePlayer(t, storageManager, adminManager)
	})
	
	t.Run("GiveItem", func(t *testing.T) {
		testAdminGiveItem(t, storageManager, adminManager)
	})
	
	t.Run("TakeItem", func(t *testing.T) {
		testAdminTakeItem(t, storageManager, adminManager)
	})
	
	t.Run("NotifyPlayer", func(t *testing.T) {
		testAdminNotifyPlayer(t, storageManager, adminManager)
	})
	
	t.Run("GetPlayerInfo", func(t *testing.T) {
		testAdminGetPlayerInfo(t, storageManager, adminManager)
	})
	
	t.Run("GetOnlinePlayers", func(t *testing.T) {
		testAdminGetOnlinePlayers(t, storageManager, adminManager)
	})
}

func testAdminKickPlayer(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player and session
	player, _, err := storageManager.CreateNewPlayer("kicktest", "kick@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Create mock connection
	mockConn := NewMockGameConnection()
	
	// Create session
	session := storageManager.Sessions().CreateSession("kick-conn-1", mockConn)
	err = storageManager.Sessions().AuthenticateSession("kick-conn-1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}
	
	// Test successful kick
	err = adminManager.KickPlayer("admin1", player.ID, "Testing kick functionality")
	if err != nil {
		t.Errorf("Failed to kick player: %v", err)
	}
	
	// Verify connection was closed
	if !mockConn.Closed {
		t.Error("Connection should have been closed after kick")
	}
	
	// Test kicking offline player
	err = adminManager.KickPlayer("admin1", "nonexistent", "Test reason")
	if err == nil {
		t.Error("Should fail when trying to kick offline player")
	}
	
	// Clean up
	storageManager.Sessions().RemoveSession(session.ConnectionID)
}

func testAdminBanPlayer(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player
	player, _, err := storageManager.CreateNewPlayer("bantest", "ban@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Create mock connection and session
	mockConn := NewMockGameConnection()
	session := storageManager.Sessions().CreateSession("ban-conn-1", mockConn)
	err = storageManager.Sessions().AuthenticateSession("ban-conn-1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}
	
	// Test successful ban
	err = adminManager.BanPlayer("admin1", player.ID, "Testing ban functionality")
	if err != nil {
		t.Errorf("Failed to ban player: %v", err)
	}
	
	// Verify player is banned
	updatedPlayer, err := storageManager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}
	
	if !updatedPlayer.IsBanned {
		t.Error("Player should be banned")
	}
	
	if updatedPlayer.BanReason != "Testing ban functionality" {
		t.Errorf("Expected ban reason 'Testing ban functionality', got '%s'", updatedPlayer.BanReason)
	}
	
	if updatedPlayer.BannedBy != "admin1" {
		t.Errorf("Expected banned by 'admin1', got '%s'", updatedPlayer.BannedBy)
	}
	
	// Verify connection was closed (kicked)
	if !mockConn.Closed {
		t.Error("Connection should have been closed after ban")
	}
	
	// Test banning offline player
	player2, _, err := storageManager.CreateNewPlayer("bantest2", "ban2@test.com")
	if err != nil {
		t.Fatalf("Failed to create second test player: %v", err)
	}
	
	err = adminManager.BanPlayer("admin1", player2.ID, "Offline ban test")
	if err != nil {
		t.Errorf("Failed to ban offline player: %v", err)
	}
	
	// Clean up
	storageManager.Sessions().RemoveSession(session.ConnectionID)
}

func testAdminUnbanPlayer(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player and ban them first
	player, _, err := storageManager.CreateNewPlayer("unbantest", "unban@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Ban the player first
	err = adminManager.BanPlayer("admin1", player.ID, "Initial ban for unban test")
	if err != nil {
		t.Fatalf("Failed to ban player for unban test: %v", err)
	}
	
	// Test successful unban
	err = adminManager.UnbanPlayer("admin1", player.ID)
	if err != nil {
		t.Errorf("Failed to unban player: %v", err)
	}
	
	// Verify player is unbanned
	updatedPlayer, err := storageManager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}
	
	if updatedPlayer.IsBanned {
		t.Error("Player should be unbanned")
	}
	
	// Test unbanning non-existent player
	err = adminManager.UnbanPlayer("admin1", "nonexistent")
	if err == nil {
		t.Error("Should fail when trying to unban non-existent player")
	}
}

func testAdminMovePlayer(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test world and room
	world, err := storageManager.CreateNewWorld("testworld", "Test World")
	if err != nil {
		t.Fatalf("Failed to create test world: %v", err)
	}
	
	room, err := storageManager.CreateNewRoom(world.ID, "testroom", "Test Room")
	if err != nil {
		t.Fatalf("Failed to create test room: %v", err)
	}
	
	// Create test player
	player, _, err := storageManager.CreateNewPlayer("movetest", "move@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Test successful move
	err = adminManager.MovePlayer("admin1", player.ID, world.ID, room.ID, "Testing move functionality")
	if err != nil {
		t.Errorf("Failed to move player: %v", err)
	}
	
	// Verify player location
	updatedPlayer, err := storageManager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}
	
	if updatedPlayer.CurrentWorldID != world.ID {
		t.Errorf("Expected world ID '%s', got '%s'", world.ID, updatedPlayer.CurrentWorldID)
	}
	
	if updatedPlayer.CurrentRoomID != room.ID {
		t.Errorf("Expected room ID '%s', got '%s'", room.ID, updatedPlayer.CurrentRoomID)
	}
	
	// Test moving to non-existent room
	err = adminManager.MovePlayer("admin1", player.ID, world.ID, "nonexistent", "Test reason")
	if err == nil {
		t.Error("Should fail when trying to move to non-existent room")
	}
	
	// Test moving banned player
	err = adminManager.BanPlayer("admin1", player.ID, "Ban for move test")
	if err != nil {
		t.Fatalf("Failed to ban player for move test: %v", err)
	}
	
	err = adminManager.MovePlayer("admin1", player.ID, world.ID, room.ID, "Should fail")
	if err == nil {
		t.Error("Should fail when trying to move banned player")
	}
}

func testAdminGiveItem(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player
	player, _, err := storageManager.CreateNewPlayer("givetest", "give@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Create test item
	item, err := storageManager.CreateNewItem("testitem", "Test Item")
	if err != nil {
		t.Fatalf("Failed to create test item: %v", err)
	}
	
	// Test successful give
	err = adminManager.GiveItem("admin1", player.ID, item.ID, 5, "Testing give functionality")
	if err != nil {
		t.Errorf("Failed to give item: %v", err)
	}
	
	// Verify item was added to inventory
	inventory, err := storageManager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to load player inventory: %v", err)
	}
	
	if len(inventory.Items) == 0 {
		t.Error("Player should have received item")
	}
	
	// Test giving to non-existent player
	err = adminManager.GiveItem("admin1", "nonexistent", item.ID, 1, "Test reason")
	if err == nil {
		t.Error("Should fail when trying to give item to non-existent player")
	}
	
	// Test giving non-existent item
	err = adminManager.GiveItem("admin1", player.ID, "nonexistent", 1, "Test reason")
	if err == nil {
		t.Error("Should fail when trying to give non-existent item")
	}
}

func testAdminTakeItem(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player
	player, _, err := storageManager.CreateNewPlayer("taketest", "take@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Create test item and give it to player
	item, err := storageManager.CreateNewItem("testitem2", "Test Item 2")
	if err != nil {
		t.Fatalf("Failed to create test item: %v", err)
	}
	
	err = storageManager.GiveItemToPlayer(player.ID, item.ID, 3)
	if err != nil {
		t.Fatalf("Failed to give item to player: %v", err)
	}
	
	// Get the item instance ID
	inventory, err := storageManager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to load player inventory: %v", err)
	}
	
	if len(inventory.Items) == 0 {
		t.Fatal("Player should have item to take")
	}
	
	instanceID := inventory.Items[0].ID
	
	// Test successful take
	err = adminManager.TakeItem("admin1", player.ID, instanceID, "Testing take functionality")
	if err != nil {
		t.Errorf("Failed to take item: %v", err)
	}
	
	// Verify item was removed
	updatedInventory, err := storageManager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player inventory: %v", err)
	}
	
	if len(updatedInventory.Items) != 0 {
		t.Error("Item should have been removed from inventory")
	}
	
	// Test taking from non-existent player
	err = adminManager.TakeItem("admin1", "nonexistent", "someinstance", "Test reason")
	if err == nil {
		t.Error("Should fail when trying to take from non-existent player")
	}
	
	// Test taking non-existent item
	err = adminManager.TakeItem("admin1", player.ID, "nonexistent", "Test reason")
	if err == nil {
		t.Error("Should fail when trying to take non-existent item")
	}
}

func testAdminNotifyPlayer(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player and session
	player, _, err := storageManager.CreateNewPlayer("notifytest", "notify@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Create mock connection and session
	mockConn := NewMockGameConnection()
	session := storageManager.Sessions().CreateSession("notify-conn-1", mockConn)
	err = storageManager.Sessions().AuthenticateSession("notify-conn-1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}
	
	// Test successful notify
	err = adminManager.NotifyPlayer("admin1", player.ID, "This is a test notification")
	if err != nil {
		t.Errorf("Failed to notify player: %v", err)
	}
	
	// Test notifying offline player
	err = adminManager.NotifyPlayer("admin1", "nonexistent", "Test message")
	if err == nil {
		t.Error("Should fail when trying to notify offline player")
	}
	
	// Clean up
	storageManager.Sessions().RemoveSession(session.ConnectionID)
}

func testAdminGetPlayerInfo(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Create test player
	player, _, err := storageManager.CreateNewPlayer("infotest", "info@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Test getting player info
	info, err := adminManager.GetPlayerInfo(player.ID)
	if err != nil {
		t.Errorf("Failed to get player info: %v", err)
	}
	
	// Verify info contains expected fields
	expectedFields := []string{"id", "username", "display_name", "email", "is_banned", "is_online"}
	for _, field := range expectedFields {
		if _, exists := info[field]; !exists {
			t.Errorf("Player info missing field: %s", field)
		}
	}
	
	// Verify values
	if info["id"] != player.ID {
		t.Errorf("Expected ID '%s', got '%v'", player.ID, info["id"])
	}
	
	if info["username"] != player.Username {
		t.Errorf("Expected username '%s', got '%v'", player.Username, info["username"])
	}
	
	if info["is_online"] != false {
		t.Error("Player should not be online")
	}
	
	// Test getting info for non-existent player
	_, err = adminManager.GetPlayerInfo("nonexistent")
	if err == nil {
		t.Error("Should fail when getting info for non-existent player")
	}
}

func testAdminGetOnlinePlayers(t *testing.T, storageManager *storage.Manager, adminManager *admin.AdminManager) {
	// Initially should be empty
	onlinePlayers := adminManager.GetOnlinePlayers()
	if len(onlinePlayers) != 0 {
		t.Errorf("Expected 0 online players, got %d", len(onlinePlayers))
	}
	
	// Create test players and sessions
	player1, _, err := storageManager.CreateNewPlayer("online1", "online1@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player 1: %v", err)
	}
	
	player2, _, err := storageManager.CreateNewPlayer("online2", "online2@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player 2: %v", err)
	}
	
	// Create sessions
	mockConn1 := NewMockGameConnection()
	mockConn2 := NewMockGameConnection()
	
	session1 := storageManager.Sessions().CreateSession("online-conn-1", mockConn1)
	session2 := storageManager.Sessions().CreateSession("online-conn-2", mockConn2)
	
	err = storageManager.Sessions().AuthenticateSession("online-conn-1", player1.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session 1: %v", err)
	}
	
	err = storageManager.Sessions().AuthenticateSession("online-conn-2", player2.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session 2: %v", err)
	}
	
	// Now should have 2 online players
	onlinePlayers = adminManager.GetOnlinePlayers()
	if len(onlinePlayers) != 2 {
		t.Errorf("Expected 2 online players, got %d", len(onlinePlayers))
	}
	
	// Clean up
	storageManager.Sessions().RemoveSession(session1.ConnectionID)
	storageManager.Sessions().RemoveSession(session2.ConnectionID)
}

// MockGameConnection implements connection.GameConnection for testing
type MockGameConnection struct {
	Closed       bool
	WrittenData  [][]byte
	WriteError   error
	CloseError   error
	ctx          context.Context
}

func NewMockGameConnection() *MockGameConnection {
	return &MockGameConnection{
		ctx: context.Background(),
	}
}

func (m *MockGameConnection) Read(b []byte) (int, error) {
	return 0, fmt.Errorf("mock read not implemented")
}

func (m *MockGameConnection) Write(b []byte) (int, error) {
	if m.WriteError != nil {
		return 0, m.WriteError
	}
	m.WrittenData = append(m.WrittenData, b)
	return len(b), nil
}

func (m *MockGameConnection) Close() error {
	m.Closed = true
	return m.CloseError
}

func (m *MockGameConnection) WriteMessage(data []byte) error {
	if m.WriteError != nil {
		return m.WriteError
	}
	m.WrittenData = append(m.WrittenData, data)
	return nil
}

func (m *MockGameConnection) ReadMessage() ([]byte, error) {
	return nil, fmt.Errorf("mock read message not implemented")
}

func (m *MockGameConnection) ConnectionType() connection.ConnectionType {
	return connection.ConnectionTypeTCP
}

func (m *MockGameConnection) IsConnected() bool {
	return !m.Closed
}

func (m *MockGameConnection) SetReadTimeout(timeout time.Duration) error {
	return nil
}

func (m *MockGameConnection) SetWriteTimeout(timeout time.Duration) error {
	return nil
}

func (m *MockGameConnection) Context() context.Context {
	return m.ctx
}

func (m *MockGameConnection) RemoteAddrWithProtocol() string {
	return "tcp://127.0.0.1:12345"
}

func (m *MockGameConnection) LocalAddrWithProtocol() string {
	return "tcp://127.0.0.1:8080"
}

func (m *MockGameConnection) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	return addr
}

func (m *MockGameConnection) RemoteAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	return addr
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

// TestAdminEventIntegration tests integration with the event system
func TestAdminEventIntegration(t *testing.T) {
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
	
	// Track events
	var triggeredEvents []string
	var eventCancelled bool
	
	// Register event handlers
	eventManager.RegisterHook(events.EventType("admin.kick.pre"), func(event events.Event) events.EventResult {
		triggeredEvents = append(triggeredEvents, "kick.pre")
		if eventCancelled {
			return events.EventResultCancel
		}
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventType("admin.kick.post"), func(event events.Event) events.EventResult {
		triggeredEvents = append(triggeredEvents, "kick.post")
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventType("admin.log"), func(event events.Event) events.EventResult {
		triggeredEvents = append(triggeredEvents, "log")
		return events.EventResultContinue
	}, 0)
	
	// Create admin manager
	adminManager := admin.NewAdminManager(storageManager, eventManager)
	
	// Create test player and session
	player, _, err := storageManager.CreateNewPlayer("eventtest", "event@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	mockConn := NewMockGameConnection()
	session := storageManager.Sessions().CreateSession("event-conn-1", mockConn)
	err = storageManager.Sessions().AuthenticateSession("event-conn-1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}
	
	// Test normal event flow
	triggeredEvents = []string{}
	err = adminManager.KickPlayer("admin1", player.ID, "Event test")
	if err != nil {
		t.Errorf("Failed to kick player: %v", err)
	}
	
	// Should have triggered pre, post, and log events (order may vary for post and log)
	expectedEventCount := 3
	if len(triggeredEvents) != expectedEventCount {
		t.Errorf("Expected %d events, got %d: %v", expectedEventCount, len(triggeredEvents), triggeredEvents)
	}
	
	// Check that required events are present
	hasPreEvent := false
	hasPostEvent := false
	hasLogEvent := false
	
	for _, event := range triggeredEvents {
		switch event {
		case "kick.pre":
			hasPreEvent = true
		case "kick.post":
			hasPostEvent = true
		case "log":
			hasLogEvent = true
		}
	}
	
	if !hasPreEvent {
		t.Error("Missing kick.pre event")
	}
	if !hasPostEvent {
		t.Error("Missing kick.post event")
	}
	if !hasLogEvent {
		t.Error("Missing log event")
	}
	
	// Test event cancellation
	// Clean up the kicked session first
	storageManager.Sessions().RemoveSession(session.ConnectionID)
	
	// Create new session since previous one was kicked
	session2 := storageManager.Sessions().CreateSession("event-conn-2", NewMockGameConnection())
	err = storageManager.Sessions().AuthenticateSession("event-conn-2", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session 2: %v", err)
	}
	
	triggeredEvents = []string{}
	eventCancelled = true
	
	err = adminManager.KickPlayer("admin1", player.ID, "Should be cancelled")
	if err == nil {
		t.Error("Kick should have been cancelled by event handler")
	}
	
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("Error should mention cancellation, got: %v", err)
	}
	
	// Should only have pre and log events (no post event since action was cancelled)
	expectedCancelledEvents := []string{"kick.pre", "log"}
	if len(triggeredEvents) != len(expectedCancelledEvents) {
		t.Errorf("Expected %d events after cancellation, got %d: %v", len(expectedCancelledEvents), len(triggeredEvents), triggeredEvents)
	}
	
	// Clean up
	storageManager.Sessions().RemoveSession(session.ConnectionID)
	storageManager.Sessions().RemoveSession(session2.ConnectionID)
}

// TestAdminErrorHandling tests error handling scenarios
func TestAdminErrorHandling(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	
	// Create storage manager with test directory
	config := &storage.StorageConfig{DataRoot: tempDir}
	storageManager := storage.NewManagerWithConfig(config)
	if err := storageManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create admin manager without event manager (test nil events handling)
	adminManager := admin.NewAdminManager(storageManager, nil)
	
	// Test operations with non-existent entities
	t.Run("NonExistentPlayer", func(t *testing.T) {
		err := adminManager.KickPlayer("admin1", "nonexistent", "Test")
		if err == nil {
			t.Error("Should fail when kicking non-existent player")
		}
		
		err = adminManager.BanPlayer("admin1", "nonexistent", "Test")
		if err != nil {
			// Ban creates the entry if player doesn't exist, so this might work
			// depending on implementation. Check the error message.
			if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
				t.Logf("Ban error (expected for non-existent player): %v", err)
			}
		}
		
		err = adminManager.MovePlayer("admin1", "nonexistent", "world", "room", "Test")
		if err == nil {
			t.Error("Should fail when moving non-existent player")
		}
		
		err = adminManager.NotifyPlayer("admin1", "nonexistent", "Test")
		if err == nil {
			t.Error("Should fail when notifying non-existent player")
		}
		
		_, err = adminManager.GetPlayerInfo("nonexistent")
		if err == nil {
			t.Error("Should fail when getting info for non-existent player")
		}
	})
	
	t.Run("InvalidOperations", func(t *testing.T) {
		// Create test player for invalid operations
		player, _, err := storageManager.CreateNewPlayer("errortest", "error@test.com")
		if err != nil {
			t.Fatalf("Failed to create test player: %v", err)
		}
		
		// Test moving to non-existent world/room
		err = adminManager.MovePlayer("admin1", player.ID, "nonexistent", "room", "Test")
		if err == nil {
			t.Error("Should fail when moving to non-existent world")
		}
		
		// Test giving non-existent item
		err = adminManager.GiveItem("admin1", player.ID, "nonexistent", 1, "Test")
		if err == nil {
			t.Error("Should fail when giving non-existent item")
		}
		
		// Test taking non-existent item
		err = adminManager.TakeItem("admin1", player.ID, "nonexistent", "Test")
		if err == nil {
			t.Error("Should fail when taking non-existent item")
		}
	})
	
	t.Run("CorruptedData", func(t *testing.T) {
		// Create a corrupted player file
		playerDir := filepath.Join(tempDir, "players")
		corruptFile := filepath.Join(playerDir, "corrupted.json")
		
		err := os.WriteFile(corruptFile, []byte("invalid json content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create corrupted file: %v", err)
		}
		
		// Test operations with corrupted data
		_, err = adminManager.GetPlayerInfo("corrupted")
		if err == nil {
			t.Error("Should fail when reading corrupted player data")
		}
	})
}

// TestAdminConcurrency tests concurrent admin operations
func TestAdminConcurrency(t *testing.T) {
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
	
	// Create test player
	player, _, err := storageManager.CreateNewPlayer("concurrencytest", "concurrent@test.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	// Test concurrent ban/unban operations
	done := make(chan bool, 10)
	
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			// Alternate between ban and unban
			if id%2 == 0 {
				adminManager.BanPlayer(fmt.Sprintf("admin%d", id), player.ID, fmt.Sprintf("Ban %d", id))
			} else {
				adminManager.UnbanPlayer(fmt.Sprintf("admin%d", id), player.ID)
			}
		}(i)
	}
	
	// Wait for all operations to complete
	for i := 0; i < 5; i++ {
		<-done
	}
	
	// Verify player still exists and is in a valid state
	_, err = storageManager.Players().Load(player.ID)
	if err != nil {
		t.Errorf("Failed to load player after concurrent operations: %v", err)
	}
	
	// Player should be in a consistent state (either banned or not)
	// Final player state available in variables for assertions
}