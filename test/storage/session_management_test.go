package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestSessionCreationAndManagement(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	sessionManager := manager.Sessions()

	// Test session creation
	conn1 := NewMockGameConnection("conn1")
	session1 := sessionManager.CreateSession("conn1", conn1)

	if session1 == nil {
		t.Fatal("Session should have been created")
	}

	if session1.ConnectionID != "conn1" {
		t.Errorf("Expected connection ID 'conn1', got '%s'", session1.ConnectionID)
	}

	if session1.State != storage.SessionStateConnected {
		t.Errorf("Expected state CONNECTED, got %s", storage.SessionStateToString(session1.State))
	}

	if session1.PlayerID != "" {
		t.Error("New session should not have player ID")
	}

	if session1.Player != nil {
		t.Error("New session should not have player object")
	}

	// Test getting session
	retrievedSession, exists := sessionManager.GetSession("conn1")
	if !exists {
		t.Error("Session should exist")
	}

	if retrievedSession.ConnectionID != session1.ConnectionID {
		t.Error("Retrieved session should match original")
	}

	// Test session count
	if sessionManager.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session, got %d", sessionManager.GetSessionCount())
	}

	if sessionManager.GetAuthenticatedSessionCount() != 0 {
		t.Errorf("Expected 0 authenticated sessions, got %d", sessionManager.GetAuthenticatedSessionCount())
	}

	// Test activity update
	originalActivity := session1.LastActivity
	time.Sleep(10 * time.Millisecond)
	sessionManager.UpdateActivity("conn1")

	if !session1.LastActivity.After(originalActivity) {
		t.Error("Activity time should have been updated")
	}

	// Test session removal
	sessionManager.RemoveSession("conn1")

	if sessionManager.GetSessionCount() != 0 {
		t.Errorf("Expected 0 sessions after removal, got %d", sessionManager.GetSessionCount())
	}

	_, exists = sessionManager.GetSession("conn1")
	if exists {
		t.Error("Session should not exist after removal")
	}
}

func TestSessionAuthentication(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-auth-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test player
	player, _, err := manager.CreateNewPlayer("testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	sessionManager := manager.Sessions()

	// Create session
	conn1 := NewMockGameConnection("conn1")
	session1 := sessionManager.CreateSession("conn1", conn1)

	// Test authentication with valid player
	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}

	// Verify session state
	if session1.State != storage.SessionStateAuthenticated {
		t.Errorf("Expected state AUTHENTICATED, got %s", storage.SessionStateToString(session1.State))
	}

	if session1.PlayerID != player.ID {
		t.Errorf("Expected player ID '%s', got '%s'", player.ID, session1.PlayerID)
	}

	if session1.Player == nil {
		t.Error("Session should have player object after authentication")
	}

	if session1.Player.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", session1.Player.Username)
	}

	if session1.AuthedAt.IsZero() {
		t.Error("AuthedAt should be set after authentication")
	}

	// Test authenticated session count
	if sessionManager.GetAuthenticatedSessionCount() != 1 {
		t.Errorf("Expected 1 authenticated session, got %d", sessionManager.GetAuthenticatedSessionCount())
	}

	// Test getting session by player ID
	retrievedSession, exists := sessionManager.GetSessionByPlayerID(player.ID)
	if !exists {
		t.Error("Should be able to get session by player ID")
	}

	if retrievedSession.ConnectionID != "conn1" {
		t.Error("Retrieved session should match original")
	}

	// Test online player tracking
	onlinePlayers := sessionManager.GetOnlinePlayers()
	if len(onlinePlayers) != 1 {
		t.Errorf("Expected 1 online player, got %d", len(onlinePlayers))
	}

	if onlinePlayers[0].ID != player.ID {
		t.Error("Online player should match authenticated player")
	}

	// Test IsPlayerOnline
	if !sessionManager.IsPlayerOnline(player.ID) {
		t.Error("Player should be online")
	}

	if sessionManager.IsPlayerOnline("non-existent") {
		t.Error("Non-existent player should not be online")
	}

	// Test authentication with non-existent player
	err = sessionManager.AuthenticateSession("conn1", "non-existent-player")
	if err == nil {
		t.Error("Should not be able to authenticate with non-existent player")
	}

	// Test authentication with non-existent session
	err = sessionManager.AuthenticateSession("non-existent-conn", player.ID)
	if err == nil {
		t.Error("Should not be able to authenticate non-existent session")
	}
}

func TestSessionBannedPlayerAuthentication(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-banned-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test players
	player, _, err := manager.CreateNewPlayer("banneduser", "banned@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	admin, _, err := manager.CreateNewPlayer("admin", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	// Ban the player
	err = manager.BanPlayer(player.ID, "Test ban", admin.ID)
	if err != nil {
		t.Fatalf("Failed to ban player: %v", err)
	}

	sessionManager := manager.Sessions()

	// Create session
	conn1 := NewMockGameConnection("conn1")
	sessionManager.CreateSession("conn1", conn1)

	// Test authentication with banned player
	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err == nil {
		t.Error("Should not be able to authenticate banned player")
	}

	// Verify session is still not authenticated
	session, _ := sessionManager.GetSession("conn1")
	if session.State != storage.SessionStateConnected {
		t.Error("Session should remain in CONNECTED state after failed authentication")
	}

	if sessionManager.GetAuthenticatedSessionCount() != 0 {
		t.Error("Should have no authenticated sessions")
	}
}

func TestSessionDuplicatePlayerAuthentication(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-duplicate-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test player
	player, _, err := manager.CreateNewPlayer("testuser", "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	sessionManager := manager.Sessions()

	// Create first session and authenticate
	conn1 := NewMockGameConnection("conn1")
	sessionManager.CreateSession("conn1", conn1)

	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate first session: %v", err)
	}

	// Create second session and try to authenticate with same player
	conn2 := NewMockGameConnection("conn2")
	sessionManager.CreateSession("conn2", conn2)

	err = sessionManager.AuthenticateSession("conn2", player.ID)
	if err == nil {
		t.Error("Should not be able to authenticate same player on different session")
	}

	// Verify only first session is authenticated
	if sessionManager.GetAuthenticatedSessionCount() != 1 {
		t.Errorf("Expected 1 authenticated session, got %d", sessionManager.GetAuthenticatedSessionCount())
	}

	// Test that re-authentication on same session works
	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err != nil {
		t.Error("Should be able to re-authenticate same player on same session")
	}
}

func TestSessionKickAndBan(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-kick-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test players
	player, _, err := manager.CreateNewPlayer("kicktest", "kick@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	admin, _, err := manager.CreateNewPlayer("admin", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	sessionManager := manager.Sessions()

	// Create session and authenticate
	conn1 := NewMockGameConnection("conn1")
	sessionManager.CreateSession("conn1", conn1)

	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}

	// Verify player is online
	if !sessionManager.IsPlayerOnline(player.ID) {
		t.Error("Player should be online")
	}

	// Test kicking player
	err = sessionManager.KickPlayer(player.ID, "Test kick")
	if err != nil {
		t.Fatalf("Failed to kick player: %v", err)
	}

	// Verify connection was closed
	if !conn1.closed {
		t.Error("Connection should have been closed after kick")
	}

	// Test kicking offline player
	err = sessionManager.KickPlayer("non-existent-player", "Test kick")
	if err == nil {
		t.Error("Should not be able to kick offline player")
	}

	// Test ban and kick
	player2, _, err := manager.CreateNewPlayer("bantest", "ban@example.com")
	if err != nil {
		t.Fatalf("Failed to create second player: %v", err)
	}

	// Create session for second player
	conn2 := NewMockGameConnection("conn2")
	sessionManager.CreateSession("conn2", conn2)

	err = sessionManager.AuthenticateSession("conn2", player2.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate second session: %v", err)
	}

	// Ban and kick player
	err = sessionManager.BanAndKickPlayer(player2.ID, "Test ban", admin.ID)
	if err != nil {
		t.Fatalf("Failed to ban and kick player: %v", err)
	}

	// Verify player was banned in storage
	bannedPlayer, err := manager.Players().Load(player2.ID)
	if err != nil {
		t.Fatalf("Failed to load banned player: %v", err)
	}

	if !bannedPlayer.IsBanned {
		t.Error("Player should be banned")
	}

	if bannedPlayer.BanReason != "Test ban" {
		t.Errorf("Expected ban reason 'Test ban', got '%s'", bannedPlayer.BanReason)
	}

	// Verify connection was closed
	if !conn2.closed {
		t.Error("Connection should have been closed after ban")
	}
}

func TestSessionInfo(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-info-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test player
	player, _, err := manager.CreateNewPlayer("infotest", "info@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	sessionManager := manager.Sessions()

	// Create session
	conn1 := NewMockGameConnection("conn1")
	session := sessionManager.CreateSession("conn1", conn1)

	// Test session info before authentication
	info := session.GetSessionInfo()

	if info["connection_id"] != "conn1" {
		t.Error("Session info should include connection ID")
	}

	if info["state"] != "CONNECTED" {
		t.Error("Session info should show CONNECTED state")
	}

	if info["player_id"] != "" {
		t.Error("Session info should not have player ID before authentication")
	}

	// Authenticate session
	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}

	// Test session info after authentication
	info = session.GetSessionInfo()

	if info["state"] != "AUTHENTICATED" {
		t.Error("Session info should show AUTHENTICATED state")
	}

	if info["player_id"] != player.ID {
		t.Error("Session info should include player ID after authentication")
	}

	if info["username"] != "infotest" {
		t.Error("Session info should include username")
	}

	if info["display_name"] != "infotest" {
		t.Error("Session info should include display name")
	}

	if info["is_banned"] != false {
		t.Error("Session info should show player is not banned")
	}

	// Test that authed_at is included
	if _, exists := info["authed_at"]; !exists {
		t.Error("Session info should include authed_at timestamp")
	}
}

func TestSessionLocationUpdate(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-session-location-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test data
	player, _, err := manager.CreateNewPlayer("locationtest", "location@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	world, err := manager.CreateNewWorld("Test World", "Test world")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	room, err := manager.CreateNewRoom(world.ID, "Test Room", "Test room")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	sessionManager := manager.Sessions()

	// Create and authenticate session
	conn1 := NewMockGameConnection("conn1")
	session := sessionManager.CreateSession("conn1", conn1)

	err = sessionManager.AuthenticateSession("conn1", player.ID)
	if err != nil {
		t.Fatalf("Failed to authenticate session: %v", err)
	}

	// Test location update on unauthenticated session
	conn2 := NewMockGameConnection("conn2")
	unauthSession := sessionManager.CreateSession("conn2", conn2)

	err = unauthSession.UpdatePlayerLocation(world.ID, room.ID)
	if err == nil {
		t.Error("Should not be able to update location on unauthenticated session")
	}

	// Test location update on authenticated session
	err = session.UpdatePlayerLocation(world.ID, room.ID)
	if err != nil {
		t.Fatalf("Failed to update player location: %v", err)
	}

	// Verify location was updated in session
	if session.Player.CurrentWorldID != world.ID {
		t.Errorf("Expected world ID '%s', got '%s'", world.ID, session.Player.CurrentWorldID)
	}

	if session.Player.CurrentRoomID != room.ID {
		t.Errorf("Expected room ID '%s', got '%s'", room.ID, session.Player.CurrentRoomID)
	}

	// Save to storage and verify persistence
	err = sessionManager.SavePlayerData("conn1")
	if err != nil {
		t.Fatalf("Failed to save player data: %v", err)
	}

	// Reload from storage and verify
	reloadedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if reloadedPlayer.CurrentWorldID != world.ID {
		t.Errorf("Expected persisted world ID '%s', got '%s'", world.ID, reloadedPlayer.CurrentWorldID)
	}

	if reloadedPlayer.CurrentRoomID != room.ID {
		t.Errorf("Expected persisted room ID '%s', got '%s'", room.ID, reloadedPlayer.CurrentRoomID)
	}

	// Test saving data for non-existent session
	err = sessionManager.SavePlayerData("non-existent")
	if err == nil {
		t.Error("Should not be able to save data for non-existent session")
	}

	// Test saving data for unauthenticated session
	err = sessionManager.SavePlayerData("conn2")
	if err == nil {
		t.Error("Should not be able to save data for unauthenticated session")
	}
}

func TestSessionStateToString(t *testing.T) {
	tests := []struct {
		state    storage.SessionState
		expected string
	}{
		{storage.SessionStateConnected, "CONNECTED"},
		{storage.SessionStateAuthenticated, "AUTHENTICATED"},
		{storage.SessionStateDisconnected, "DISCONNECTED"},
		{storage.SessionState(999), "UNKNOWN"},
	}

	for _, test := range tests {
		result := storage.SessionStateToString(test.state)
		if result != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, result)
		}
	}
}