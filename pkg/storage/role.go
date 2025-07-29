package storage

import (
	"fmt"
	"path/filepath"
	"time"
)

// PlayerRoles represents the roles assigned to a specific player
type PlayerRoles struct {
	PlayerID  string    `json:"player_id"`
	Roles     []string  `json:"roles"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by,omitempty"` // Admin who last updated roles
	
	// Custom properties for role-specific data
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// NewPlayerRoles creates a new PlayerRoles instance
func NewPlayerRoles(playerID string) *PlayerRoles {
	return &PlayerRoles{
		PlayerID:   playerID,
		Roles:      make([]string, 0),
		UpdatedAt:  time.Now(),
		Properties: make(map[string]interface{}),
	}
}

// Update updates the timestamp
func (pr *PlayerRoles) Update() {
	pr.UpdatedAt = time.Now()
}

// AddRole adds a role to the player if not already present
func (pr *PlayerRoles) AddRole(role string) bool {
	if pr.HasRole(role) {
		return false // Role already exists
	}
	pr.Roles = append(pr.Roles, role)
	pr.Update()
	return true
}

// RemoveRole removes a role from the player
func (pr *PlayerRoles) RemoveRole(role string) bool {
	for i, r := range pr.Roles {
		if r == role {
			pr.Roles = append(pr.Roles[:i], pr.Roles[i+1:]...)
			pr.Update()
			return true
		}
	}
	return false // Role not found
}

// HasRole checks if the player has a specific role
func (pr *PlayerRoles) HasRole(role string) bool {
	for _, r := range pr.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the player has any of the specified roles
func (pr *PlayerRoles) HasAnyRole(roles []string) bool {
	for _, role := range roles {
		if pr.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the player has all of the specified roles
func (pr *PlayerRoles) HasAllRoles(roles []string) bool {
	for _, role := range roles {
		if !pr.HasRole(role) {
			return false
		}
	}
	return true
}

// SetRoles replaces all roles with the provided list
func (pr *PlayerRoles) SetRoles(roles []string, updatedBy string) {
	pr.Roles = make([]string, len(roles))
	copy(pr.Roles, roles)
	pr.UpdatedBy = updatedBy
	pr.Update()
}

// ClearRoles removes all roles from the player
func (pr *PlayerRoles) ClearRoles(updatedBy string) {
	pr.Roles = make([]string, 0)
	pr.UpdatedBy = updatedBy
	pr.Update()
}

// GetProperty gets a custom property value
func (pr *PlayerRoles) GetProperty(key string) (interface{}, bool) {
	if pr.Properties == nil {
		return nil, false
	}
	value, exists := pr.Properties[key]
	return value, exists
}

// SetProperty sets a custom property value
func (pr *PlayerRoles) SetProperty(key string, value interface{}) {
	if pr.Properties == nil {
		pr.Properties = make(map[string]interface{})
	}
	pr.Properties[key] = value
	pr.Update()
}

// RoleManager manages role storage operations
type RoleManager struct {
	storage *StorageManager
}

// NewRoleManager creates a new role manager
func NewRoleManager(storage *StorageManager) *RoleManager {
	return &RoleManager{
		storage: storage,
	}
}

// Save saves player roles to storage
func (rm *RoleManager) Save(playerRoles *PlayerRoles) error {
	playerRoles.Update()
	filename := fmt.Sprintf("%s.json", playerRoles.PlayerID)
	return rm.storage.SaveJSON(filepath.Join("roles", filename), playerRoles)
}

// Load loads player roles from storage by player ID
func (rm *RoleManager) Load(playerID string) (*PlayerRoles, error) {
	var playerRoles PlayerRoles
	filename := fmt.Sprintf("%s.json", playerID)
	err := rm.storage.LoadJSON(filepath.Join("roles", filename), &playerRoles)
	if err != nil {
		return nil, fmt.Errorf("failed to load roles for player %s: %w", playerID, err)
	}
	return &playerRoles, nil
}

// LoadOrCreate loads player roles or creates new empty roles if not found
func (rm *RoleManager) LoadOrCreate(playerID string) (*PlayerRoles, error) {
	playerRoles, err := rm.Load(playerID)
	if err != nil {
		// Create new empty roles
		playerRoles = NewPlayerRoles(playerID)
		err = rm.Save(playerRoles)
		if err != nil {
			return nil, fmt.Errorf("failed to create roles for player %s: %w", playerID, err)
		}
	}
	return playerRoles, nil
}

// Exists checks if player roles exist by player ID
func (rm *RoleManager) Exists(playerID string) bool {
	filename := fmt.Sprintf("%s.json", playerID)
	return rm.storage.FileExists(filepath.Join("roles", filename))
}

// Delete deletes player roles from storage
func (rm *RoleManager) Delete(playerID string) error {
	filename := fmt.Sprintf("%s.json", playerID)
	return rm.storage.DeleteFile(filepath.Join("roles", filename))
}

// ListAll lists all player IDs that have role assignments
func (rm *RoleManager) ListAll() ([]string, error) {
	files, err := rm.storage.ListFiles("roles")
	if err != nil {
		return nil, fmt.Errorf("failed to list role files: %w", err)
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

// FindPlayersByRole finds all players that have a specific role
func (rm *RoleManager) FindPlayersByRole(role string) ([]*PlayerRoles, error) {
	playerIDs, err := rm.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list players for role search: %w", err)
	}

	var playersWithRole []*PlayerRoles
	for _, playerID := range playerIDs {
		playerRoles, err := rm.Load(playerID)
		if err != nil {
			continue // Skip corrupted files
		}

		if playerRoles.HasRole(role) {
			playersWithRole = append(playersWithRole, playerRoles)
		}
	}

	return playersWithRole, nil
}

// AddRoleToPlayer adds a role to a player (convenience method)
func (rm *RoleManager) AddRoleToPlayer(playerID, role, updatedBy string) error {
	playerRoles, err := rm.LoadOrCreate(playerID)
	if err != nil {
		return fmt.Errorf("failed to load/create roles for player %s: %w", playerID, err)
	}

	if playerRoles.AddRole(role) {
		playerRoles.UpdatedBy = updatedBy
		return rm.Save(playerRoles)
	}

	return nil // Role already exists, no need to save
}

// RemoveRoleFromPlayer removes a role from a player (convenience method)
func (rm *RoleManager) RemoveRoleFromPlayer(playerID, role, updatedBy string) error {
	playerRoles, err := rm.Load(playerID)
	if err != nil {
		return fmt.Errorf("failed to load roles for player %s: %w", playerID, err)
	}

	if playerRoles.RemoveRole(role) {
		playerRoles.UpdatedBy = updatedBy
		return rm.Save(playerRoles)
	}

	return nil // Role didn't exist, no need to save
}

// SetPlayerRoles sets all roles for a player (convenience method)
func (rm *RoleManager) SetPlayerRoles(playerID string, roles []string, updatedBy string) error {
	playerRoles, err := rm.LoadOrCreate(playerID)
	if err != nil {
		return fmt.Errorf("failed to load/create roles for player %s: %w", playerID, err)
	}

	playerRoles.SetRoles(roles, updatedBy)
	return rm.Save(playerRoles)
}

// HasRole checks if a player has a specific role (convenience method)
func (rm *RoleManager) HasRole(playerID, role string) bool {
	playerRoles, err := rm.Load(playerID)
	if err != nil {
		return false // Player has no roles file or error loading
	}

	return playerRoles.HasRole(role)
}

// HasAnyRole checks if a player has any of the specified roles (convenience method)
func (rm *RoleManager) HasAnyRole(playerID string, roles []string) bool {
	playerRoles, err := rm.Load(playerID)
	if err != nil {
		return false // Player has no roles file or error loading
	}

	return playerRoles.HasAnyRole(roles)
}

// HasAllRoles checks if a player has all of the specified roles (convenience method)
func (rm *RoleManager) HasAllRoles(playerID string, roles []string) bool {
	playerRoles, err := rm.Load(playerID)
	if err != nil {
		return false // Player has no roles file or error loading
	}

	return playerRoles.HasAllRoles(roles)
}

// GetPlayerRoles gets all roles for a player (convenience method)
func (rm *RoleManager) GetPlayerRoles(playerID string) ([]string, error) {
	playerRoles, err := rm.Load(playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load roles for player %s: %w", playerID, err)
	}

	// Return a copy to prevent external modification
	roles := make([]string, len(playerRoles.Roles))
	copy(roles, playerRoles.Roles)
	return roles, nil
}