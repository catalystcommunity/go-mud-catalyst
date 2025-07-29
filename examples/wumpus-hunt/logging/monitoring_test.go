package logging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
)

func TestMonitoringServer_Creation(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = true
	config.Port = 0 // Use any available port for testing

	// Create test dependencies
	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)
	if server == nil {
		t.Fatal("Monitoring server should not be nil")
	}

	if server.config.Enabled != config.Enabled {
		t.Error("Configuration not properly applied")
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringServer_ClientTracking(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false // Don't start HTTP server

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)

	// Test adding clients
	server.AddClient("client-1", "player-1", "TestPlayer1")
	server.AddClient("client-2", "player-2", "TestPlayer2")

	// Verify client tracking
	if len(server.activeClients) != 2 {
		t.Errorf("Expected 2 active clients, got %d", len(server.activeClients))
	}

	// Test updating client location
	server.UpdateClientLocation("client-1", "world-1", "room-1", "instance-1", true)

	client := server.activeClients["client-1"]
	if client.WorldID != "world-1" || client.RoomID != "room-1" || !client.IsInMaze {
		t.Error("Client location update failed")
	}

	// Test updating client health
	server.UpdateClientHealth("client-1", 75)
	if client.Health != 75 {
		t.Error("Client health update failed")
	}

	// Test removing clients
	server.RemoveClient("client-1")
	if len(server.activeClients) != 1 {
		t.Errorf("Expected 1 active client after removal, got %d", len(server.activeClients))
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringServer_LoginTracking(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)

	// Test login tracking
	server.RecordLogin("TestPlayer", true)
	server.RecordLogin("BadPlayer", false)
	server.RecordLogin("TestPlayer", true)

	// Verify login metrics
	if server.clientMetrics.TotalLogins != 2 {
		t.Errorf("Expected 2 successful logins, got %d", server.clientMetrics.TotalLogins)
	}
	if server.clientMetrics.FailedLogins != 1 {
		t.Errorf("Expected 1 failed login, got %d", server.clientMetrics.FailedLogins)
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringServer_HTTPHandlers(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false
	config.EnableAuth = false // Disable auth for testing

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)

	// Setup HTTP server for testing
	server.setupHTTPServer()

	// Add some test data
	server.AddClient("client-1", "player-1", "TestPlayer")
	server.UpdateClientLocation("client-1", "world-1", "room-1", "", false)

	// Test status endpoint
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	server.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var statusResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&statusResp); err != nil {
		t.Fatalf("Failed to decode status response: %v", err)
	}

	if statusResp["status"] != "healthy" {
		t.Error("Expected healthy status")
	}

	// Test stats endpoint
	req = httptest.NewRequest("GET", "/api/stats", nil)
	w = httptest.NewRecorder()
	server.handleStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var statsResp SystemStats
	if err := json.NewDecoder(w.Body).Decode(&statsResp); err != nil {
		t.Fatalf("Failed to decode stats response: %v", err)
	}

	if statsResp.ActivePlayers != 1 {
		t.Errorf("Expected 1 active player, got %d", statsResp.ActivePlayers)
	}

	// Test players endpoint
	req = httptest.NewRequest("GET", "/api/players", nil)
	w = httptest.NewRecorder()
	server.handlePlayers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var playersResp []*ClientInfo
	if err := json.NewDecoder(w.Body).Decode(&playersResp); err != nil {
		t.Fatalf("Failed to decode players response: %v", err)
	}

	if len(playersResp) != 1 {
		t.Errorf("Expected 1 player in response, got %d", len(playersResp))
	}

	if playersResp[0].PlayerName != "TestPlayer" {
		t.Error("Player name mismatch")
	}

	// Test instances endpoint
	req = httptest.NewRequest("GET", "/api/instances", nil)
	w = httptest.NewRecorder()
	server.handleInstances(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test metrics endpoint
	req = httptest.NewRequest("GET", "/api/metrics", nil)
	w = httptest.NewRecorder()
	server.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test dashboard endpoint
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	server.handleDashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/html" {
		t.Error("Expected HTML content type for dashboard")
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringServer_Authentication(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false
	config.EnableAuth = true
	config.AdminToken = "test-token"

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)
	server.setupHTTPServer()

	// Test unauthorized request
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthorized request, got %d", w.Code)
	}

	// Test authorized request
	req = httptest.NewRequest("GET", "/api/status", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for authorized request, got %d", w.Code)
	}

	// Test dashboard (should not require auth)
	req = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for dashboard without auth, got %d", w.Code)
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringServer_CORS(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false
	config.EnableAuth = false
	config.EnableCORS = true

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)
	server.setupHTTPServer()

	// Test CORS preflight request
	req := httptest.NewRequest("OPTIONS", "/api/status", nil)
	w := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS request, got %d", w.Code)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header to be set")
	}

	// Test regular request has CORS headers
	req = httptest.NewRequest("GET", "/api/status", nil)
	w = httptest.NewRecorder()
	server.server.Handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header to be set on regular request")
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringConfig_Default(t *testing.T) {
	config := DefaultMonitoringConfig()

	if !config.Enabled {
		t.Error("Monitoring should be enabled by default")
	}
	if config.Host != "localhost" {
		t.Error("Default host should be localhost")
	}
	if config.Port != 8080 {
		t.Error("Default port should be 8080")
	}
	if !config.EnableAuth {
		t.Error("Auth should be enabled by default")
	}
	if !config.EnableCORS {
		t.Error("CORS should be enabled by default")
	}
	if config.AdminToken == "" {
		t.Error("Admin token should have a default value")
	}
}

func TestClientMetrics_Threading(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)

	// Test concurrent client operations
	done := make(chan bool)
	numGoroutines := 10
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				clientID := "client-" + string(rune('A'+id)) + "-" + string(rune('0'+j%10))
				playerID := "player-" + string(rune('A'+id)) + "-" + string(rune('0'+j%10))
				playerName := "Player" + string(rune('A'+id))

				server.AddClient(clientID, playerID, playerName)
				server.UpdateClientLocation(clientID, "world-1", "room-1", "", false)
				server.UpdateClientHealth(clientID, 100)
				server.RecordLogin(playerName, true)
				server.RemoveClient(clientID)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state is consistent
	if server.clientMetrics.ActiveConnections != 0 {
		t.Errorf("Expected 0 active connections after cleanup, got %d", server.clientMetrics.ActiveConnections)
	}

	if server.clientMetrics.TotalLogins != int64(numGoroutines*numOperations) {
		t.Errorf("Expected %d total logins, got %d", numGoroutines*numOperations, server.clientMetrics.TotalLogins)
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestPlayerStatsCollector(t *testing.T) {
	storageMgr := createTestStorageManager()
	defer cleanupTestData(storageMgr.GetConfig().DataRoot)

	collector := NewPlayerStatsCollector(storageMgr)

	// Create a test player
	player := storage.NewPlayer("player-123", "TestPlayer")
	player.DisplayName = "TestPlayer"
	
	if err := storageMgr.Players().Save(player); err != nil {
		t.Fatalf("Failed to save test player: %v", err)
	}

	// Test getting player stats
	stats, err := collector.GetPlayerStats(player.ID)
	if err != nil {
		t.Fatalf("Failed to get player stats: %v", err)
	}

	if stats == nil {
		t.Fatal("Player stats should not be nil")
	}

	// Verify expected fields
	expectedFields := []string{"player_id", "username", "display_name", "games_played", "games_won", "wumpus_pelts", "total_deaths", "win_rate", "created_at", "last_login"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field %s not found in stats", field)
		}
	}

	// Test getting stats for non-existent player
	_, err = collector.GetPlayerStats("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent player")
	}
}

func TestSystemStats_Calculation(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Enabled = false

	gameLogger, _ := NewGameLogger(DefaultGameLoggerConfig())
	defer gameLogger.Close()

	metricsCollector, _ := NewMetricsCollector(DefaultMetricsConfig())
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	instanceMgr := instance.NewInstanceManager(storageMgr)

	server := NewMonitoringServer(config, gameLogger, metricsCollector, instanceMgr, storageMgr)

	// Add some test data
	server.AddClient("client-1", "player-1", "Player1")
	server.AddClient("client-2", "player-2", "Player2")
	server.RecordLogin("Player1", true)
	server.RecordLogin("Player2", true)

	// Test stats calculation methods
	activeCount := len(server.activeClients)
	if activeCount != 2 {
		t.Errorf("Expected 2 active players, got %d", activeCount)
	}

	instanceCount := server.getActiveInstanceCount()
	if instanceCount < 0 {
		t.Error("Instance count should not be negative")
	}

	totalGames := server.getTotalGames()
	if totalGames < 0 {
		t.Error("Total games should not be negative")
	}

	// Cleanup
	cleanupTestData(storageMgr.GetConfig().DataRoot)
}

func TestMonitoringServer_Integration(t *testing.T) {
	// Create a complete monitoring setup
	logConfig := DefaultGameLoggerConfig()
	logConfig.EnableFileLog = false
	logConfig.EnableConsoleLog = false

	gameLogger, err := NewGameLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create game logger: %v", err)
	}
	defer gameLogger.Close()

	metricsConfig := DefaultMetricsConfig()
	metricsConfig.EnableFileOutput = false
	metricsConfig.EnableConsoleOutput = false

	metricsCollector, err := NewMetricsCollector(metricsConfig)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	defer cleanupTestData(storageMgr.GetConfig().DataRoot)

	instanceMgr := instance.NewInstanceManager(storageMgr)

	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.Enabled = false
	monitoringConfig.EnableAuth = false

	server := NewMonitoringServer(monitoringConfig, gameLogger, metricsCollector, instanceMgr, storageMgr)

	// Simulate a complete player session
	clientID := "client-test"
	playerID := "player-test"
	playerName := "TestPlayer"

	// Player connects
	server.AddClient(clientID, playerID, playerName)
	gameLogger.LogLogin(playerID, playerName, clientID, true, "Login successful", nil)

	// Player moves around
	server.UpdateClientLocation(clientID, "world-1", "room-1", "", false)
	gameLogger.LogPlayerMove(playerID, playerName, "room-0", "room-1", "world-1", "north")

	// Player enters portal
	server.UpdateClientLocation(clientID, "world-2", "room-maze-1", "instance-1", true)
	gameLogger.LogPortalEnter(playerID, playerName, "instance-1", "world-2")

	// Player fights wumpus
	gameLogger.LogAttack(playerID, playerName, "wumpus", "world-2", "room-maze-5", 50, true)
	gameLogger.LogWumpusKill(playerID, playerName, "world-2", "room-maze-5", "instance-1", 100)

	// Player wins and disconnects
	server.UpdateClientLocation(clientID, "world-1", "room-inn", "", false)
	gameLogger.LogPlayerVictory(playerID, playerName, "world-1", "room-inn", "instance-1", 1)
	server.RemoveClient(clientID)
	gameLogger.LogLogout(playerID, playerName, clientID, "Logout successful")

	// Verify monitoring data
	if server.clientMetrics.TotalConnections != 1 {
		t.Error("Should have recorded 1 connection")
	}
	if server.clientMetrics.ActiveConnections != 0 {
		t.Error("Should have 0 active connections after logout")
	}

	// Give time for async operations
	time.Sleep(100 * time.Millisecond)
}

func TestMonitoringGlobalInitialization(t *testing.T) {
	logConfig := DefaultGameLoggerConfig()
	logConfig.EnableFileLog = false
	logConfig.EnableConsoleLog = false

	gameLogger, _ := NewGameLogger(logConfig)
	defer gameLogger.Close()

	metricsConfig := DefaultMetricsConfig()
	metricsConfig.EnableFileOutput = false
	metricsConfig.EnableConsoleOutput = false

	metricsCollector, _ := NewMetricsCollector(metricsConfig)
	defer metricsCollector.Close()

	storageMgr := createTestStorageManager()
	defer cleanupTestData(storageMgr.GetConfig().DataRoot)

	instanceMgr := instance.NewInstanceManager(storageMgr)

	monitoringConfig := DefaultMonitoringConfig()
	monitoringConfig.Enabled = false

	// Test initialization
	err := InitializeMonitoring(monitoringConfig, gameLogger, metricsCollector, instanceMgr, storageMgr)
	if err != nil {
		t.Fatalf("Failed to initialize monitoring: %v", err)
	}

	// Test global getter
	if GetMonitoringServer() == nil {
		t.Error("Global monitoring server should not be nil")
	}

	// Test shutdown (won't actually start server since Enabled = false)
	err = StopMonitoring()
	if err != nil {
		t.Fatalf("Failed to stop monitoring: %v", err)
	}
}