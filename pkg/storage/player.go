package storage

import (
	"fmt"
	"path/filepath"
	"time"
)

// Player represents a player/character in the game
type Player struct {
	ID          string            `json:"id"`
	Username    string            `json:"username"`
	DisplayName string            `json:"display_name"`
	Email       string            `json:"email,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastLogin   time.Time         `json:"last_login"`
	
	// Game state
	CurrentWorldID string `json:"current_world_id"`
	CurrentRoomID  string `json:"current_room_id"`
	
	// Status
	IsBanned    bool   `json:"is_banned"`
	BanReason   string `json:"ban_reason,omitempty"`
	BannedAt    time.Time `json:"banned_at,omitempty"`
	BannedBy    string `json:"banned_by,omitempty"`
	
	// Custom properties (for game-specific data)
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// NewPlayer creates a new player with the given username
func NewPlayer(id, username string) *Player {
	now := time.Now()
	return &Player{
		ID:          id,
		Username:    username,
		DisplayName: username,
		CreatedAt:   now,
		UpdatedAt:   now,
		Properties:  make(map[string]interface{}),
	}
}

// Update updates the player's timestamp
func (p *Player) Update() {
	p.UpdatedAt = time.Now()
}

// Ban bans the player with the given reason and admin ID
func (p *Player) Ban(reason, adminID string) {
	p.IsBanned = true
	p.BanReason = reason
	p.BannedAt = time.Now()
	p.BannedBy = adminID
	p.Update()
}

// Unban unbans the player
func (p *Player) Unban() {
	p.IsBanned = false
	p.BanReason = ""
	p.BannedAt = time.Time{}
	p.BannedBy = ""
	p.Update()
}

// SetLocation sets the player's current location
func (p *Player) SetLocation(worldID, roomID string) {
	p.CurrentWorldID = worldID
	p.CurrentRoomID = roomID
	p.Update()
}

// GetProperty gets a custom property value
func (p *Player) GetProperty(key string) (interface{}, bool) {
	if p.Properties == nil {
		return nil, false
	}
	value, exists := p.Properties[key]
	return value, exists
}

// SetProperty sets a custom property value
func (p *Player) SetProperty(key string, value interface{}) {
	if p.Properties == nil {
		p.Properties = make(map[string]interface{})
	}
	p.Properties[key] = value
	p.Update()
}

// DeleteProperty removes a custom property
func (p *Player) DeleteProperty(key string) {
	if p.Properties != nil {
		delete(p.Properties, key)
		p.Update()
	}
}

// PlayerManager manages player storage operations
type PlayerManager struct {
	storage *StorageManager
}

// NewPlayerManager creates a new player manager
func NewPlayerManager(storage *StorageManager) *PlayerManager {
	return &PlayerManager{
		storage: storage,
	}
}

// Save saves a player to storage
func (pm *PlayerManager) Save(player *Player) error {
	player.Update()
	filename := fmt.Sprintf("%s.json", player.ID)
	return pm.storage.SaveJSON(filepath.Join("players", filename), player)
}

// Load loads a player from storage by ID
func (pm *PlayerManager) Load(playerID string) (*Player, error) {
	var player Player
	filename := fmt.Sprintf("%s.json", playerID)
	err := pm.storage.LoadJSON(filepath.Join("players", filename), &player)
	if err != nil {
		return nil, fmt.Errorf("failed to load player %s: %w", playerID, err)
	}
	return &player, nil
}

// LoadByUsername loads a player by username (requires scanning all player files)
func (pm *PlayerManager) LoadByUsername(username string) (*Player, error) {
	files, err := pm.storage.ListFiles("players")
	if err != nil {
		return nil, fmt.Errorf("failed to list player files: %w", err)
	}

	for _, filename := range files {
		if filepath.Ext(filename) != ".json" {
			continue
		}

		var player Player
		err := pm.storage.LoadJSON(filepath.Join("players", filename), &player)
		if err != nil {
			continue // Skip corrupted files
		}

		if player.Username == username {
			return &player, nil
		}
	}

	return nil, fmt.Errorf("player with username %s not found", username)
}

// Exists checks if a player exists by ID
func (pm *PlayerManager) Exists(playerID string) bool {
	filename := fmt.Sprintf("%s.json", playerID)
	return pm.storage.FileExists(filepath.Join("players", filename))
}

// ExistsByUsername checks if a player exists by username
func (pm *PlayerManager) ExistsByUsername(username string) bool {
	_, err := pm.LoadByUsername(username)
	return err == nil
}

// Delete deletes a player from storage
func (pm *PlayerManager) Delete(playerID string) error {
	filename := fmt.Sprintf("%s.json", playerID)
	return pm.storage.DeleteFile(filepath.Join("players", filename))
}

// ListAll lists all player IDs
func (pm *PlayerManager) ListAll() ([]string, error) {
	files, err := pm.storage.ListFiles("players")
	if err != nil {
		return nil, fmt.Errorf("failed to list player files: %w", err)
	}

	var playerIDs []string
	for _, filename := range files {
		if filepath.Ext(filename) == ".json" {
			playerID := filename[:len(filename)-5] // Remove .json extension
			playerIDs = append(playerIDs, playerID)
		}
	}

	return playerIDs, nil
}

// GetOnlinePlayers returns players currently in any world
// This method should be called on the SessionManager instead for accurate results
func (pm *PlayerManager) GetOnlinePlayers() ([]*Player, error) {
	// This method is deprecated in favor of SessionManager.GetOnlinePlayers()
	// Return empty slice to maintain backwards compatibility
	return []*Player{}, nil
}