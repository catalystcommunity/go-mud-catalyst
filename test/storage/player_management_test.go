package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestPlayerCreationAndManagement(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-player-management-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Test player creation with all fields
	player1, inventory1, err := manager.CreateNewPlayer("testuser1", "test1@example.com")
	if err != nil {
		t.Fatalf("Failed to create player 1: %v", err)
	}

	// Verify player fields
	if player1.Username != "testuser1" {
		t.Errorf("Expected username 'testuser1', got '%s'", player1.Username)
	}

	if player1.DisplayName != "testuser1" {
		t.Errorf("Expected display name 'testuser1', got '%s'", player1.DisplayName)
	}

	if player1.Email != "test1@example.com" {
		t.Errorf("Expected email 'test1@example.com', got '%s'", player1.Email)
	}

	if player1.IsBanned {
		t.Error("New player should not be banned")
	}

	if player1.CreatedAt.IsZero() {
		t.Error("Player should have creation timestamp")
	}

	// Verify inventory was created
	if inventory1 == nil {
		t.Fatal("Inventory should have been created")
	}

	if inventory1.OwnerID != player1.ID {
		t.Error("Inventory owner should match player ID")
	}

	// Test player properties
	player1.SetProperty("level", 5)
	player1.SetProperty("class", "warrior")
	player1.SetProperty("experience", 1250.5)

	if err := manager.Players().Save(player1); err != nil {
		t.Fatalf("Failed to save player with properties: %v", err)
	}

	// Reload and verify properties
	reloadedPlayer1, err := manager.Players().Load(player1.ID)
	if err != nil {
		t.Fatalf("Failed to reload player 1: %v", err)
	}

	level, exists := reloadedPlayer1.GetProperty("level")
	if !exists || level != float64(5) { // JSON unmarshaling converts to float64
		t.Errorf("Expected level 5, got %v (exists: %v)", level, exists)
	}

	class, exists := reloadedPlayer1.GetProperty("class")
	if !exists || class != "warrior" {
		t.Errorf("Expected class 'warrior', got %v (exists: %v)", class, exists)
	}

	experience, exists := reloadedPlayer1.GetProperty("experience")
	if !exists || experience != 1250.5 {
		t.Errorf("Expected experience 1250.5, got %v (exists: %v)", experience, exists)
	}

	// Test property deletion
	reloadedPlayer1.DeleteProperty("experience")
	if err := manager.Players().Save(reloadedPlayer1); err != nil {
		t.Fatalf("Failed to save player after property deletion: %v", err)
	}

	reloadedPlayer1, err = manager.Players().Load(player1.ID)
	if err != nil {
		t.Fatalf("Failed to reload player after property deletion: %v", err)
	}

	_, exists = reloadedPlayer1.GetProperty("experience")
	if exists {
		t.Error("Experience property should have been deleted")
	}

	// Test player listing
	player2, _, err := manager.CreateNewPlayer("testuser2", "test2@example.com")
	if err != nil {
		t.Fatalf("Failed to create player 2: %v", err)
	}

	allPlayers, err := manager.Players().ListAll()
	if err != nil {
		t.Fatalf("Failed to list all players: %v", err)
	}

	if len(allPlayers) != 2 {
		t.Errorf("Expected 2 players, got %d", len(allPlayers))
	}

	// Verify both player IDs are in the list
	foundPlayer1, foundPlayer2 := false, false
	for _, playerID := range allPlayers {
		if playerID == player1.ID {
			foundPlayer1 = true
		}
		if playerID == player2.ID {
			foundPlayer2 = true
		}
	}

	if !foundPlayer1 || !foundPlayer2 {
		t.Error("Both players should be in the list")
	}

	// Test loading by username
	loadedByUsername, err := manager.Players().LoadByUsername("testuser1")
	if err != nil {
		t.Fatalf("Failed to load player by username: %v", err)
	}

	if loadedByUsername.ID != player1.ID {
		t.Error("Loaded player should match original player")
	}

	// Test existence checks
	if !manager.Players().Exists(player1.ID) {
		t.Error("Player 1 should exist")
	}

	if !manager.Players().ExistsByUsername("testuser1") {
		t.Error("Player with username 'testuser1' should exist")
	}

	if manager.Players().ExistsByUsername("nonexistent") {
		t.Error("Player with username 'nonexistent' should not exist")
	}

	// Test display name modification
	player1.DisplayName = "Test User One"
	if err := manager.Players().Save(player1); err != nil {
		t.Fatalf("Failed to save player with new display name: %v", err)
	}

	reloadedPlayer1, err = manager.Players().Load(player1.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if reloadedPlayer1.DisplayName != "Test User One" {
		t.Errorf("Expected display name 'Test User One', got '%s'", reloadedPlayer1.DisplayName)
	}

	// Test timestamp updates
	originalUpdateTime := reloadedPlayer1.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	reloadedPlayer1.SetProperty("test", "value")
	if err := manager.Players().Save(reloadedPlayer1); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	reloadedPlayer1, err = manager.Players().Load(player1.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if !reloadedPlayer1.UpdatedAt.After(originalUpdateTime) {
		t.Error("UpdatedAt should be updated when player is modified")
	}
}

func TestPlayerBanSystem(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-player-ban-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test players
	player, _, err := manager.CreateNewPlayer("bantest", "ban@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	admin, _, err := manager.CreateNewPlayer("admin", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	// Test initial state
	if player.IsBanned {
		t.Error("Player should not be banned initially")
	}

	if !player.BannedAt.IsZero() {
		t.Error("BannedAt should be zero for unbanned player")
	}

	if player.BanReason != "" {
		t.Error("BanReason should be empty for unbanned player")
	}

	if player.BannedBy != "" {
		t.Error("BannedBy should be empty for unbanned player")
	}

	// Test banning player
	banReason := "Inappropriate behavior"
	err = manager.BanPlayer(player.ID, banReason, admin.ID)
	if err != nil {
		t.Fatalf("Failed to ban player: %v", err)
	}

	// Reload and verify ban
	bannedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload banned player: %v", err)
	}

	if !bannedPlayer.IsBanned {
		t.Error("Player should be banned")
	}

	if bannedPlayer.BanReason != banReason {
		t.Errorf("Expected ban reason '%s', got '%s'", banReason, bannedPlayer.BanReason)
	}

	if bannedPlayer.BannedBy != admin.ID {
		t.Errorf("Expected banned by '%s', got '%s'", admin.ID, bannedPlayer.BannedBy)
	}

	if bannedPlayer.BannedAt.IsZero() {
		t.Error("BannedAt should not be zero for banned player")
	}

	// Verify ban timestamp is recent
	timeSinceBan := time.Since(bannedPlayer.BannedAt)
	if timeSinceBan > time.Minute {
		t.Error("Ban timestamp should be recent")
	}

	// Test that banned player cannot be moved to rooms
	world, err := manager.CreateNewWorld("Test World", "Test world")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	room, err := manager.CreateNewRoom(world.ID, "Test Room", "Test room")
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	err = manager.MovePlayerToRoom(bannedPlayer.ID, world.ID, room.ID)
	if err == nil {
		t.Error("Banned player should not be able to move to rooms")
	}

	// Test unbanning player
	err = manager.UnbanPlayer(player.ID)
	if err != nil {
		t.Fatalf("Failed to unban player: %v", err)
	}

	// Reload and verify unban
	unbannedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload unbanned player: %v", err)
	}

	if unbannedPlayer.IsBanned {
		t.Error("Player should not be banned after unban")
	}

	if unbannedPlayer.BanReason != "" {
		t.Error("BanReason should be empty after unban")
	}

	if unbannedPlayer.BannedBy != "" {
		t.Error("BannedBy should be empty after unban")
	}

	if !unbannedPlayer.BannedAt.IsZero() {
		t.Error("BannedAt should be zero after unban")
	}

	// Test that unbanned player can now move to rooms
	err = manager.MovePlayerToRoom(unbannedPlayer.ID, world.ID, room.ID)
	if err != nil {
		t.Errorf("Unbanned player should be able to move to rooms: %v", err)
	}

	// Verify location was updated
	movedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload moved player: %v", err)
	}

	if movedPlayer.CurrentWorldID != world.ID {
		t.Errorf("Expected current world '%s', got '%s'", world.ID, movedPlayer.CurrentWorldID)
	}

	if movedPlayer.CurrentRoomID != room.ID {
		t.Errorf("Expected current room '%s', got '%s'", room.ID, movedPlayer.CurrentRoomID)
	}

	// Test banning non-existent player
	err = manager.BanPlayer("non-existent-player", "test", admin.ID)
	if err == nil {
		t.Error("Should not be able to ban non-existent player")
	}

	// Test unbanning non-existent player
	err = manager.UnbanPlayer("non-existent-player")
	if err == nil {
		t.Error("Should not be able to unban non-existent player")
	}
}

func TestPlayerLocationTracking(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-player-location-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create player
	player, _, err := manager.CreateNewPlayer("locationtest", "location@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Initially player should have no location
	if player.CurrentWorldID != "" {
		t.Error("New player should have no world ID")
	}

	if player.CurrentRoomID != "" {
		t.Error("New player should have no room ID")
	}

	// Create worlds and rooms
	world1, err := manager.CreateNewWorld("World 1", "First world")
	if err != nil {
		t.Fatalf("Failed to create world 1: %v", err)
	}

	world2, err := manager.CreateNewWorld("World 2", "Second world")
	if err != nil {
		t.Fatalf("Failed to create world 2: %v", err)
	}

	room1, err := manager.CreateNewRoom(world1.ID, "Room 1", "First room")
	if err != nil {
		t.Fatalf("Failed to create room 1: %v", err)
	}

	room2, err := manager.CreateNewRoom(world2.ID, "Room 2", "Second room")
	if err != nil {
		t.Fatalf("Failed to create room 2: %v", err)
	}

	// Move player to first location
	err = manager.MovePlayerToRoom(player.ID, world1.ID, room1.ID)
	if err != nil {
		t.Fatalf("Failed to move player to room 1: %v", err)
	}

	// Verify location update
	movedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if movedPlayer.CurrentWorldID != world1.ID {
		t.Errorf("Expected world ID '%s', got '%s'", world1.ID, movedPlayer.CurrentWorldID)
	}

	if movedPlayer.CurrentRoomID != room1.ID {
		t.Errorf("Expected room ID '%s', got '%s'", room1.ID, movedPlayer.CurrentRoomID)
	}

	// Move player to different world
	err = manager.MovePlayerToRoom(player.ID, world2.ID, room2.ID)
	if err != nil {
		t.Fatalf("Failed to move player to room 2: %v", err)
	}

	// Verify location update
	movedPlayer, err = manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player after world change: %v", err)
	}

	if movedPlayer.CurrentWorldID != world2.ID {
		t.Errorf("Expected world ID '%s', got '%s'", world2.ID, movedPlayer.CurrentWorldID)
	}

	if movedPlayer.CurrentRoomID != room2.ID {
		t.Errorf("Expected room ID '%s', got '%s'", room2.ID, movedPlayer.CurrentRoomID)
	}

	// Test direct location setting
	player.SetLocation(world1.ID, room1.ID)
	if err := manager.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player with direct location: %v", err)
	}

	reloadedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	if reloadedPlayer.CurrentWorldID != world1.ID {
		t.Errorf("Expected world ID '%s' after direct set, got '%s'", world1.ID, reloadedPlayer.CurrentWorldID)
	}

	if reloadedPlayer.CurrentRoomID != room1.ID {
		t.Errorf("Expected room ID '%s' after direct set, got '%s'", room1.ID, reloadedPlayer.CurrentRoomID)
	}
}

func TestPlayerDeletion(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-player-deletion-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create player
	player, inventory, err := manager.CreateNewPlayer("deletetest", "delete@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Verify player and inventory exist
	if !manager.Players().Exists(player.ID) {
		t.Error("Player should exist before deletion")
	}

	if !manager.Inventories().Exists(inventory.ID) {
		t.Error("Inventory should exist before deletion")
	}

	// Delete player
	err = manager.Players().Delete(player.ID)
	if err != nil {
		t.Fatalf("Failed to delete player: %v", err)
	}

	// Verify player no longer exists
	if manager.Players().Exists(player.ID) {
		t.Error("Player should not exist after deletion")
	}

	// Try to load deleted player
	_, err = manager.Players().Load(player.ID)
	if err == nil {
		t.Error("Should not be able to load deleted player")
	}

	// Try to load by username
	_, err = manager.Players().LoadByUsername("deletetest")
	if err == nil {
		t.Error("Should not be able to load deleted player by username")
	}

	// Note: Inventory deletion would typically be handled by a higher-level
	// operation or cascade delete system, but is not automatically handled
	// by player deletion in our current implementation
}

func TestPlayerLoginTracking(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-player-login-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create player
	player, _, err := manager.CreateNewPlayer("logintest", "login@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Initially LastLogin should be zero
	if !player.LastLogin.IsZero() {
		t.Error("New player should have zero LastLogin time")
	}

	// Simulate login by setting LastLogin
	loginTime := time.Now()
	player.LastLogin = loginTime
	if err := manager.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player with login time: %v", err)
	}

	// Reload and verify login time
	reloadedPlayer, err := manager.Players().Load(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload player: %v", err)
	}

	// Allow for small time differences due to serialization
	timeDiff := reloadedPlayer.LastLogin.Sub(loginTime)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("LastLogin time differs too much: expected %v, got %v", loginTime, reloadedPlayer.LastLogin)
	}
}