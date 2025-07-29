package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
)

// MonitoringServer provides HTTP endpoints for admin monitoring
type MonitoringServer struct {
	server         *http.Server
	gameLogger     *GameLogger
	metrics        *MetricsCollector
	instanceMgr    *instance.InstanceManager
	storageMgr     *storage.Manager
	mu             sync.RWMutex
	config         *MonitoringConfig
	activeClients  map[string]*ClientInfo
	clientMetrics  *ClientMetrics
}

// MonitoringConfig configures the monitoring server
type MonitoringConfig struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	EnableAuth  bool   `json:"enable_auth"`
	AdminToken  string `json:"admin_token"`
	EnableCORS  bool   `json:"enable_cors"`
}

// ClientInfo tracks information about connected clients
type ClientInfo struct {
	ClientID     string    `json:"client_id"`
	PlayerID     string    `json:"player_id"`
	PlayerName   string    `json:"player_name"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastActivity time.Time `json:"last_activity"`
	WorldID      string    `json:"world_id"`
	RoomID       string    `json:"room_id"`
	InstanceID   string    `json:"instance_id,omitempty"`
	IsInMaze     bool      `json:"is_in_maze"`
	Health       int       `json:"health"`
	Status       string    `json:"status"`
}

// ClientMetrics tracks client-related metrics
type ClientMetrics struct {
	TotalConnections    int64     `json:"total_connections"`
	ActiveConnections   int       `json:"active_connections"`
	PeakConnections     int       `json:"peak_connections"`
	TotalLogins         int64     `json:"total_logins"`
	FailedLogins        int64     `json:"failed_logins"`
	LastReset           time.Time `json:"last_reset"`
	ConnectionsByHour   [24]int   `json:"connections_by_hour"`
	LoginsByHour        [24]int   `json:"logins_by_hour"`
	mu                  sync.RWMutex
}

// SystemStats represents overall system statistics
type SystemStats struct {
	Uptime            time.Duration         `json:"uptime"`
	ActivePlayers     int                   `json:"active_players"`
	ActiveInstances   int                   `json:"active_instances"`
	TotalGames        int64                 `json:"total_games"`
	TotalVictories    int64                 `json:"total_victories"`
	TotalDeaths       int64                 `json:"total_deaths"`
	MemoryUsage       int64                 `json:"memory_usage_bytes"`
	PropertiesMetrics map[string]interface{} `json:"properties_metrics"`
	GameMetrics       map[string]interface{} `json:"game_metrics"`
	LastUpdated       time.Time             `json:"last_updated"`
}

// InstanceInfo provides detailed instance information
type InstanceInfo struct {
	InstanceID    string                 `json:"instance_id"`
	WorldID       string                 `json:"world_id"`
	CreatedAt     time.Time             `json:"created_at"`
	CreatedBy     string                `json:"created_by"`
	RoomCount     int                   `json:"room_count"`
	ActivePlayers []string              `json:"active_players"`
	WumpusRoom    string                `json:"wumpus_room,omitempty"`
	StartRoom     string                `json:"start_room,omitempty"`
	Status        string                `json:"status"`
	Properties    map[string]interface{} `json:"properties,omitempty"`
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		Enabled:    true,
		Host:       "localhost",
		Port:       8080,
		EnableAuth: true,
		AdminToken: "admin-token-change-me",
		EnableCORS: true,
	}
}

// NewMonitoringServer creates a new monitoring server
func NewMonitoringServer(config *MonitoringConfig, gameLogger *GameLogger, metrics *MetricsCollector, instanceMgr *instance.InstanceManager, storageMgr *storage.Manager) *MonitoringServer {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	ms := &MonitoringServer{
		gameLogger:    gameLogger,
		metrics:       metrics,
		instanceMgr:   instanceMgr,
		storageMgr:    storageMgr,
		config:        config,
		activeClients: make(map[string]*ClientInfo),
		clientMetrics: &ClientMetrics{
			LastReset: time.Now(),
		},
	}

	if config.Enabled {
		ms.setupHTTPServer()
	}

	return ms
}

// setupHTTPServer configures the HTTP server and routes
func (ms *MonitoringServer) setupHTTPServer() {
	mux := http.NewServeMux()

	// Add middleware for CORS and auth
	handler := ms.addMiddleware(mux)

	// Register routes
	mux.HandleFunc("/", ms.handleDashboard)
	mux.HandleFunc("/api/status", ms.handleStatus)
	mux.HandleFunc("/api/stats", ms.handleStats)
	mux.HandleFunc("/api/players", ms.handlePlayers)
	mux.HandleFunc("/api/instances", ms.handleInstances)
	mux.HandleFunc("/api/metrics", ms.handleMetrics)
	mux.HandleFunc("/api/logs", ms.handleLogs)
	mux.HandleFunc("/api/properties", ms.handleProperties)

	ms.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", ms.config.Host, ms.config.Port),
		Handler: handler,
	}
}

// addMiddleware adds CORS and authentication middleware
func (ms *MonitoringServer) addMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers
		if ms.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Authentication
		if ms.config.EnableAuth && r.URL.Path != "/" {
			token := r.Header.Get("Authorization")
			if token != "Bearer "+ms.config.AdminToken {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// Start starts the monitoring server
func (ms *MonitoringServer) Start() error {
	if !ms.config.Enabled {
		return nil
	}

	return ms.server.ListenAndServe()
}

// Stop stops the monitoring server
func (ms *MonitoringServer) Stop() error {
	if ms.server != nil {
		return ms.server.Close()
	}
	return nil
}

// Client tracking methods
func (ms *MonitoringServer) AddClient(clientID, playerID, playerName string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	ms.activeClients[clientID] = &ClientInfo{
		ClientID:     clientID,
		PlayerID:     playerID,
		PlayerName:   playerName,
		ConnectedAt:  now,
		LastActivity: now,
		Status:       "connected",
	}

	// Update client metrics
	ms.clientMetrics.mu.Lock()
	ms.clientMetrics.TotalConnections++
	ms.clientMetrics.ActiveConnections++
	if ms.clientMetrics.ActiveConnections > ms.clientMetrics.PeakConnections {
		ms.clientMetrics.PeakConnections = ms.clientMetrics.ActiveConnections
	}
	ms.clientMetrics.ConnectionsByHour[now.Hour()]++
	ms.clientMetrics.mu.Unlock()
}

func (ms *MonitoringServer) RemoveClient(clientID string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.activeClients[clientID]; exists {
		delete(ms.activeClients, clientID)
		
		ms.clientMetrics.mu.Lock()
		ms.clientMetrics.ActiveConnections--
		ms.clientMetrics.mu.Unlock()
	}
}

func (ms *MonitoringServer) UpdateClientLocation(clientID, worldID, roomID, instanceID string, isInMaze bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if client, exists := ms.activeClients[clientID]; exists {
		client.WorldID = worldID
		client.RoomID = roomID
		client.InstanceID = instanceID
		client.IsInMaze = isInMaze
		client.LastActivity = time.Now()
	}
}

func (ms *MonitoringServer) UpdateClientHealth(clientID string, health int) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if client, exists := ms.activeClients[clientID]; exists {
		client.Health = health
		client.LastActivity = time.Now()
	}
}

func (ms *MonitoringServer) RecordLogin(playerName string, success bool) {
	ms.clientMetrics.mu.Lock()
	defer ms.clientMetrics.mu.Unlock()
	
	now := time.Now()
	if success {
		ms.clientMetrics.TotalLogins++
		ms.clientMetrics.LoginsByHour[now.Hour()]++
	} else {
		ms.clientMetrics.FailedLogins++
	}
}

// HTTP handlers
func (ms *MonitoringServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard := `
<!DOCTYPE html>
<html>
<head>
    <title>Wumpus Hunt - Admin Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .metric { display: inline-block; margin: 10px; padding: 10px; border: 1px solid #ccc; }
        .metric-value { font-size: 24px; font-weight: bold; color: #007acc; }
        .metric-label { font-size: 12px; color: #666; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .status-connected { color: green; }
        .status-in-maze { color: orange; }
        .status-dead { color: red; }
    </style>
    <script>
        function loadData() {
            fetch('/api/stats')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('active-players').innerText = data.active_players;
                    document.getElementById('active-instances').innerText = data.active_instances;
                    document.getElementById('total-games').innerText = data.total_games;
                    document.getElementById('total-victories').innerText = data.total_victories;
                });
            
            fetch('/api/players')
                .then(response => response.json())
                .then(data => {
                    const tbody = document.getElementById('players-table');
                    tbody.innerHTML = '';
                    data.forEach(player => {
                        const row = tbody.insertRow();
                        row.innerHTML = ` + "`" + `
                            <td>${player.player_name}</td>
                            <td>${player.world_id}</td>
                            <td>${player.room_id}</td>
                            <td>${player.health}</td>
                            <td class="status-${player.is_in_maze ? 'in-maze' : 'connected'}">${player.status}</td>
                            <td>${new Date(player.last_activity).toLocaleString()}</td>
                        ` + "`" + `;
                    });
                });
        }
        
        setInterval(loadData, 5000);
        window.onload = loadData;
    </script>
</head>
<body>
    <h1>Wumpus Hunt - Admin Dashboard</h1>
    
    <div>
        <div class="metric">
            <div class="metric-value" id="active-players">0</div>
            <div class="metric-label">Active Players</div>
        </div>
        <div class="metric">
            <div class="metric-value" id="active-instances">0</div>
            <div class="metric-label">Active Instances</div>
        </div>
        <div class="metric">
            <div class="metric-value" id="total-games">0</div>
            <div class="metric-label">Total Games</div>
        </div>
        <div class="metric">
            <div class="metric-value" id="total-victories">0</div>
            <div class="metric-label">Total Victories</div>
        </div>
    </div>
    
    <h2>Active Players</h2>
    <table>
        <thead>
            <tr>
                <th>Player</th>
                <th>World</th>
                <th>Room</th>
                <th>Health</th>
                <th>Status</th>
                <th>Last Activity</th>
            </tr>
        </thead>
        <tbody id="players-table">
        </tbody>
    </table>
    
    <p><a href="/api/stats">System Stats (JSON)</a> | 
       <a href="/api/metrics">Metrics (JSON)</a> | 
       <a href="/api/instances">Instances (JSON)</a></p>
</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, dashboard)
}

func (ms *MonitoringServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"component": "wumpus-hunt",
	}

	ms.writeJSON(w, status)
}

func (ms *MonitoringServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	activePlayerCount := len(ms.activeClients)
	ms.mu.RUnlock()

	stats := SystemStats{
		Uptime:          time.Since(time.Now().Add(-24 * time.Hour)), // Placeholder
		ActivePlayers:   activePlayerCount,
		ActiveInstances: ms.getActiveInstanceCount(),
		TotalGames:      ms.getTotalGames(),
		TotalVictories:  ms.getTotalVictories(),
		TotalDeaths:     ms.getTotalDeaths(),
		MemoryUsage:     ms.getMemoryUsage(),
		LastUpdated:     time.Now(),
	}

	if ms.metrics != nil {
		stats.PropertiesMetrics = ms.getPropertiesMetrics()
		stats.GameMetrics = ms.getGameMetrics()
	}

	ms.writeJSON(w, stats)
}

func (ms *MonitoringServer) handlePlayers(w http.ResponseWriter, r *http.Request) {
	ms.mu.RLock()
	players := make([]*ClientInfo, 0, len(ms.activeClients))
	for _, client := range ms.activeClients {
		players = append(players, client)
	}
	ms.mu.RUnlock()

	// Sort by last activity
	sort.Slice(players, func(i, j int) bool {
		return players[i].LastActivity.After(players[j].LastActivity)
	})

	ms.writeJSON(w, players)
}

func (ms *MonitoringServer) handleInstances(w http.ResponseWriter, r *http.Request) {
	instances := ms.getActiveInstances()
	ms.writeJSON(w, instances)
}

func (ms *MonitoringServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if ms.metrics == nil {
		http.Error(w, "Metrics not available", http.StatusServiceUnavailable)
		return
	}

	snapshot := ms.metrics.GetSnapshot()
	ms.writeJSON(w, snapshot)
}

func (ms *MonitoringServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	// This would return recent log entries in a real implementation
	logs := map[string]interface{}{
		"message": "Log endpoint not implemented yet",
		"note":    "Logs are currently written to files in the logs directory",
	}
	ms.writeJSON(w, logs)
}

func (ms *MonitoringServer) handleProperties(w http.ResponseWriter, r *http.Request) {
	if ms.metrics == nil {
		http.Error(w, "Metrics not available", http.StatusServiceUnavailable)
		return
	}

	propertiesData := ms.getPropertiesMetrics()
	ms.writeJSON(w, propertiesData)
}

// Helper methods
func (ms *MonitoringServer) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (ms *MonitoringServer) getActiveInstanceCount() int {
	if ms.instanceMgr == nil {
		return 0
	}
	// This would call instanceMgr.GetActiveInstanceCount() in a real implementation
	return 0
}

func (ms *MonitoringServer) getTotalGames() int64 {
	// This would query the storage or logs for total games
	return 0
}

func (ms *MonitoringServer) getTotalVictories() int64 {
	// This would query the storage or logs for total victories
	return 0
}

func (ms *MonitoringServer) getTotalDeaths() int64 {
	// This would query the storage or logs for total deaths
	return 0
}

func (ms *MonitoringServer) getMemoryUsage() int64 {
	// This would get actual memory usage
	return 0
}

func (ms *MonitoringServer) getPropertiesMetrics() map[string]interface{} {
	if ms.metrics == nil {
		return nil
	}

	snapshot := ms.metrics.GetSnapshot()
	properties := make(map[string]interface{})

	for key, value := range snapshot {
		if contains(key, "properties") {
			properties[key] = value
		}
	}

	return properties
}

func (ms *MonitoringServer) getGameMetrics() map[string]interface{} {
	if ms.metrics == nil {
		return nil
	}

	snapshot := ms.metrics.GetSnapshot()
	gameMetrics := make(map[string]interface{})

	for key, value := range snapshot {
		if contains(key, "wumpus") && !contains(key, "properties") {
			gameMetrics[key] = value
		}
	}

	return gameMetrics
}

func (ms *MonitoringServer) getActiveInstances() []InstanceInfo {
	// This would query instanceMgr for active instances
	// For now, return empty slice
	return []InstanceInfo{}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) > len(substr) && s[len(s)-len(substr):] == substr ||
		   (len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// PlayerStatsCollector collects detailed player statistics
type PlayerStatsCollector struct {
	storageMgr *storage.Manager
	mu         sync.RWMutex
}

// NewPlayerStatsCollector creates a new player stats collector
func NewPlayerStatsCollector(storageMgr *storage.Manager) *PlayerStatsCollector {
	return &PlayerStatsCollector{
		storageMgr: storageMgr,
	}
}

// GetPlayerStats retrieves comprehensive player statistics
func (psc *PlayerStatsCollector) GetPlayerStats(playerID string) (map[string]interface{}, error) {
	psc.mu.RLock() 
	defer psc.mu.RUnlock()

	player, err := psc.storageMgr.Players().Load(playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load player: %w", err)
	}

	stats := auth.GetWumpusStats(player)
	
	return map[string]interface{}{
		"player_id":      player.ID,
		"username":       player.Username,
		"display_name":   player.DisplayName,
		"games_played":   stats.GamesPlayed,
		"games_won":      stats.GamesWon,
		"wumpus_pelts":   stats.WumpusPelts,
		"total_deaths":   stats.TotalDeaths,
		"win_rate":       float64(stats.GamesWon) / float64(max(stats.GamesPlayed, 1)),
		"created_at":     player.CreatedAt,
		"last_login":     time.Now(), // Would be tracked in a real implementation
	}, nil
}

// GetAllPlayersStats retrieves statistics for all players
func (psc *PlayerStatsCollector) GetAllPlayersStats() ([]map[string]interface{}, error) {
	// This would query all players from storage
	// For now, return empty slice
	return []map[string]interface{}{}, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Global monitoring server
var globalMonitoringServer *MonitoringServer

// InitializeMonitoring initializes the global monitoring server
func InitializeMonitoring(config *MonitoringConfig, gameLogger *GameLogger, metrics *MetricsCollector, instanceMgr *instance.InstanceManager, storageMgr *storage.Manager) error {
	globalMonitoringServer = NewMonitoringServer(config, gameLogger, metrics, instanceMgr, storageMgr)
	return nil
}

// GetMonitoringServer returns the global monitoring server
func GetMonitoringServer() *MonitoringServer {
	return globalMonitoringServer
}

// StartMonitoring starts the global monitoring server
func StartMonitoring() error {
	if globalMonitoringServer != nil {
		return globalMonitoringServer.Start()
	}
	return nil
}

// StopMonitoring stops the global monitoring server
func StopMonitoring() error {
	if globalMonitoringServer != nil {
		return globalMonitoringServer.Stop()
	}
	return nil
}