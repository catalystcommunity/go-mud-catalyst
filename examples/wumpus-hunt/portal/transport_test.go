package portal

import (
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

func setupTestEnvironment(t *testing.T) (*storage.Manager, *instance.InstanceManager, *TransportHandler, *storage.Player) {
	// Initialize storage
	storageMgr := storage.NewManagerWithConfig(storage.DefaultStorageConfig())
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create main world
	_, err := world.CreateMainWorld(storageMgr)
	if err != nil {
		t.Fatalf("Failed to create main world: %v", err)
	}

	// Create instance manager
	instanceManager := instance.NewInstanceManager(storageMgr)

	// Create transport handler
	transportHandler := NewTransportHandler(storageMgr, instanceManager)

	// Create test player
	player := storage.NewPlayer(ids.NewEntityID(), "test_player")
	player.DisplayName = "Test Player"
	
	// Set up player authentication
	if err := auth.SetPlayerPassword(player, "password123"); err != nil {
		t.Fatalf("Failed to set player password: %v", err)
	}

	// Save player
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save player: %v", err)
	}

	return storageMgr, instanceManager, transportHandler, player
}

func TestNewTransportHandler(t *testing.T) {
	storageMgr, instanceManager, transportHandler, _ := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	if transportHandler == nil {
		t.Fatal("NewTransportHandler returned nil")
	}

	if transportHandler.storageMgr != storageMgr {
		t.Error("Storage manager not set correctly")
	}

	if transportHandler.instanceManager != instanceManager {
		t.Error("Instance manager not set correctly")
	}
}

func TestHandlePortalEnterUnauthenticated(t *testing.T) {
	_, instanceManager, transportHandler, _ := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Create unauthenticated player
	unauthPlayer := storage.NewPlayer(ids.NewEntityID(), "unauth_player")
	unauthPlayer.DisplayName = "Unauthenticated Player"

	result, err := transportHandler.HandlePortalEnter(unauthPlayer)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	if result.Success {
		t.Error("Expected failure for unauthenticated player")
	}

	if result.Message != "You must be authenticated to enter the portal." {
		t.Errorf("Unexpected message: %s", result.Message)
	}
}

func TestHandlePortalEnterNewInstance(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	result, err := transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	if !result.IsNewInstance {
		t.Error("Expected new instance flag to be true")
	}

	if result.InstanceID == "" {
		t.Error("Expected instance ID to be set")
	}

	if result.TargetWorldID == "" {
		t.Error("Expected target world ID to be set")
	}

	if result.TargetRoomID == "" {
		t.Error("Expected target room ID to be set")
	}

	// Check that player state was updated
	playerState := auth.GetWumpusState(player)
	if !playerState.IsInMaze {
		t.Error("Player should be marked as in maze")
	}

	if playerState.CurrentInstance != result.InstanceID {
		t.Error("Player current instance should match result instance ID")
	}
}

func TestHandlePortalEnterExistingInstance(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// First entry to create instance
	result1, err := transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("First HandlePortalEnter failed: %v", err)
	}

	if !result1.IsNewInstance {
		t.Error("First entry should create new instance")
	}

	// Second entry should use existing instance
	result2, err := transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("Second HandlePortalEnter failed: %v", err)
	}

	if !result2.Success {
		t.Errorf("Expected success, got failure: %s", result2.Message)
	}

	if result2.IsNewInstance {
		t.Error("Second entry should not create new instance")
	}

	if result2.InstanceID != result1.InstanceID {
		t.Error("Second entry should use same instance ID")
	}
}

func TestHandlePortalExit(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// First enter the portal
	enterResult, err := transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	if !enterResult.Success {
		t.Fatalf("Portal enter should succeed")
	}

	// Then exit the portal
	exitResult, err := transportHandler.HandlePortalExit(player)
	if err != nil {
		t.Fatalf("HandlePortalExit failed: %v", err)
	}

	if !exitResult.Success {
		t.Errorf("Expected success, got failure: %s", exitResult.Message)
	}

	if exitResult.TargetWorldID == "" {
		t.Error("Expected target world ID to be set")
	}

	if exitResult.TargetRoomID == "" {
		t.Error("Expected target room ID to be set")
	}

	// Check that player state was updated
	playerState := auth.GetWumpusState(player)
	if playerState.IsInMaze {
		t.Error("Player should not be marked as in maze after exit")
	}

	if playerState.CurrentInstance != "" {
		t.Error("Player current instance should be empty after exit")
	}

	if playerState.Health != 100 {
		t.Error("Player should be healed after exit")
	}
}

func TestHandleMazeExitVictory(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Enter portal first
	_, err := transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	// Get initial stats
	initialStats := auth.GetWumpusStats(player)

	// Exit with victory
	result, err := transportHandler.HandleMazeExit(player, "victory")
	if err != nil {
		t.Fatalf("HandleMazeExit failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	// Check that stats were updated
	updatedStats := auth.GetWumpusStats(player)
	if updatedStats.GamesWon != initialStats.GamesWon+1 {
		t.Error("Games won should be incremented")
	}

	if updatedStats.WumpusPelts != initialStats.WumpusPelts+1 {
		t.Error("Wumpus pelts should be incremented")
	}

	if updatedStats.GamesPlayed != initialStats.GamesPlayed+1 {
		t.Error("Games played should be incremented")
	}

	// Check that player state was reset
	playerState := auth.GetWumpusState(player)
	if playerState.IsInMaze {
		t.Error("Player should not be in maze after victory")
	}

	if playerState.CurrentInstance != "" {
		t.Error("Player current instance should be empty after victory")
	}
}

func TestHandleMazeExitDeath(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Enter portal first
	_, err := transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	// Get initial stats
	initialStats := auth.GetWumpusStats(player)

	// Exit with death
	result, err := transportHandler.HandleMazeExit(player, "death")
	if err != nil {
		t.Fatalf("HandleMazeExit failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %s", result.Message)
	}

	// Check that stats were updated
	updatedStats := auth.GetWumpusStats(player)
	if updatedStats.TotalDeaths != initialStats.TotalDeaths+1 {
		t.Error("Total deaths should be incremented")
	}

	if updatedStats.GamesPlayed != initialStats.GamesPlayed+1 {
		t.Error("Games played should be incremented")
	}

	// Games won should not change
	if updatedStats.GamesWon != initialStats.GamesWon {
		t.Error("Games won should not change on death")
	}
}

func TestCanPlayerEnterPortal(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Test authenticated player
	canEnter, message := transportHandler.CanPlayerEnterPortal(player)
	if !canEnter {
		t.Errorf("Authenticated player should be able to enter portal: %s", message)
	}

	// Test unauthenticated player
	unauthPlayer := storage.NewPlayer(ids.NewEntityID(), "unauth_player")
	canEnter, message = transportHandler.CanPlayerEnterPortal(unauthPlayer)
	if canEnter {
		t.Error("Unauthenticated player should not be able to enter portal")
	}

	if message != "You must be authenticated to enter the portal." {
		t.Errorf("Unexpected message: %s", message)
	}

	// Test player with low health
	playerState := auth.GetWumpusState(player)
	playerState.Health = 0
	auth.UpdateWumpusState(player, playerState)

	canEnter, message = transportHandler.CanPlayerEnterPortal(player)
	if canEnter {
		t.Error("Player with zero health should not be able to enter portal")
	}

	if message != "You are too weak to enter the portal. Rest in the Inn to recover." {
		t.Errorf("Unexpected message: %s", message)
	}
}

func TestGetPlayerCurrentLocation(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Test player in main world
	locationInfo, err := transportHandler.GetPlayerCurrentLocation(player)
	if err != nil {
		t.Fatalf("GetPlayerCurrentLocation failed: %v", err)
	}

	if locationInfo.IsInMaze {
		t.Error("Player should not be in maze initially")
	}

	if locationInfo.WorldID == "" {
		t.Error("World ID should be set")
	}

	// Enter portal
	_, err = transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	// Test player in maze
	locationInfo, err = transportHandler.GetPlayerCurrentLocation(player)
	if err != nil {
		t.Fatalf("GetPlayerCurrentLocation failed: %v", err)
	}

	if !locationInfo.IsInMaze {
		t.Error("Player should be in maze after entering portal")
	}

	if locationInfo.CurrentInstance == "" {
		t.Error("Current instance should be set")
	}

	if locationInfo.InstanceID == "" {
		t.Error("Instance ID should be set")
	}
}

func TestExtendInstanceLifetime(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Test extending non-existent instance
	err := transportHandler.ExtendInstanceLifetime(player, time.Hour)
	if err == nil {
		t.Error("Expected error when extending non-existent instance")
	}

	// Enter portal to create instance
	_, err = transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	// Test extending existing instance
	err = transportHandler.ExtendInstanceLifetime(player, time.Hour)
	if err != nil {
		t.Fatalf("ExtendInstanceLifetime failed: %v", err)
	}
}

func TestGetInstanceTimeRemaining(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Test getting time for non-existent instance
	_, err := transportHandler.GetInstanceTimeRemaining(player)
	if err == nil {
		t.Error("Expected error when getting time for non-existent instance")
	}

	// Enter portal to create instance
	_, err = transportHandler.HandlePortalEnter(player)
	if err != nil {
		t.Fatalf("HandlePortalEnter failed: %v", err)
	}

	// Test getting time for existing instance
	remaining, err := transportHandler.GetInstanceTimeRemaining(player)
	if err != nil {
		t.Fatalf("GetInstanceTimeRemaining failed: %v", err)
	}

	if remaining <= 0 {
		t.Error("Time remaining should be positive")
	}

	// Should be close to 2 hours (default cleanup time)
	if remaining > 2*time.Hour+time.Minute {
		t.Error("Time remaining should be close to 2 hours")
	}
}

func TestHandlePortalCommand(t *testing.T) {
	_, instanceManager, transportHandler, player := setupTestEnvironment(t)
	defer instanceManager.Shutdown()

	// Test enter command
	result, err := transportHandler.HandlePortalCommand(player, "enter")
	if err != nil {
		t.Fatalf("HandlePortalCommand(enter) failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Enter command should succeed: %s", result.Message)
	}

	// Test status command
	result, err = transportHandler.HandlePortalCommand(player, "status")
	if err != nil {
		t.Fatalf("HandlePortalCommand(status) failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Status command should succeed: %s", result.Message)
	}

	// Test exit command
	result, err = transportHandler.HandlePortalCommand(player, "exit")
	if err != nil {
		t.Fatalf("HandlePortalCommand(exit) failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Exit command should succeed: %s", result.Message)
	}

	// Test unknown command
	result, err = transportHandler.HandlePortalCommand(player, "unknown")
	if err != nil {
		t.Fatalf("HandlePortalCommand(unknown) failed: %v", err)
	}

	if result.Success {
		t.Error("Unknown command should fail")
	}
}