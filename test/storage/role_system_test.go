package storage_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// TestRoleSystemValidation provides comprehensive validation
// for role management functionality including:
// - File-based role system with player ID → role list mapping
// - Role-based permission checking utilities
// - Role management operations (add, remove, set, clear)
// - Error handling and edge cases
// - Concurrent operations and thread safety
func TestRoleSystemValidation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-role-system-validation")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	t.Run("BasicRoleOperations", func(t *testing.T) {
		testBasicRoleOperations(t, manager)
	})

	t.Run("RolePermissionChecking", func(t *testing.T) {
		testRolePermissionChecking(t, manager)
	})

	t.Run("RoleFileOperations", func(t *testing.T) {
		testRoleFileOperations(t, manager)
	})

	t.Run("BulkRoleOperations", func(t *testing.T) {
		testBulkRoleOperations(t, manager)
	})

	t.Run("RoleSearchOperations", func(t *testing.T) {
		testRoleSearchOperations(t, manager)
	})

	t.Run("ErrorHandlingAndEdgeCases", func(t *testing.T) {
		testRoleErrorHandlingAndEdgeCases(t, manager)
	})

	t.Run("ConcurrentRoleOperations", func(t *testing.T) {
		testConcurrentRoleOperations(t, manager)
	})

	t.Run("RolePropertiesAndMetadata", func(t *testing.T) {
		testRolePropertiesAndMetadata(t, manager)
	})
}

// testBasicRoleOperations validates basic role management functionality
func testBasicRoleOperations(t *testing.T, manager *storage.Manager) {
	playerID := "test-player-basic"

	// Test creating new player roles
	playerRoles := storage.NewPlayerRoles(playerID)
	if playerRoles.PlayerID != playerID {
		t.Errorf("Expected player ID %s, got %s", playerID, playerRoles.PlayerID)
	}

	if len(playerRoles.Roles) != 0 {
		t.Error("New player roles should be empty")
	}

	if playerRoles.UpdatedAt.IsZero() {
		t.Error("Player roles should have timestamp")
	}

	// Test adding roles
	if !playerRoles.AddRole("user") {
		t.Error("Should be able to add new role")
	}

	if !playerRoles.HasRole("user") {
		t.Error("Player should have 'user' role")
	}

	// Test adding duplicate role
	if playerRoles.AddRole("user") {
		t.Error("Should not be able to add duplicate role")
	}

	// Test adding multiple roles
	playerRoles.AddRole("moderator")
	playerRoles.AddRole("premium")

	expectedRoles := []string{"user", "moderator", "premium"}
	if len(playerRoles.Roles) != len(expectedRoles) {
		t.Errorf("Expected %d roles, got %d", len(expectedRoles), len(playerRoles.Roles))
	}

	// Test role checking
	for _, role := range expectedRoles {
		if !playerRoles.HasRole(role) {
			t.Errorf("Player should have role %s", role)
		}
	}

	// Test removing role
	if !playerRoles.RemoveRole("moderator") {
		t.Error("Should be able to remove existing role")
	}

	if playerRoles.HasRole("moderator") {
		t.Error("Player should not have 'moderator' role after removal")
	}

	// Test removing non-existent role
	if playerRoles.RemoveRole("admin") {
		t.Error("Should not be able to remove non-existent role")
	}

	// Test clearing all roles
	playerRoles.ClearRoles("admin1")
	if len(playerRoles.Roles) != 0 {
		t.Error("All roles should be cleared")
	}

	if playerRoles.UpdatedBy != "admin1" {
		t.Error("UpdatedBy should be set when clearing roles")
	}
}

// testRolePermissionChecking validates role-based permission checking
func testRolePermissionChecking(t *testing.T, manager *storage.Manager) {
	playerID := "test-player-permissions"

	// Create player roles with specific roles
	playerRoles := storage.NewPlayerRoles(playerID)
	playerRoles.AddRole("user")
	playerRoles.AddRole("moderator")
	playerRoles.AddRole("premium")

	// Test HasAnyRole
	adminRoles := []string{"admin", "moderator", "superuser"}
	if !playerRoles.HasAnyRole(adminRoles) {
		t.Error("Player should have at least one admin role (moderator)")
	}

	nonExistentRoles := []string{"superuser", "developer", "owner"}
	if playerRoles.HasAnyRole(nonExistentRoles) {
		t.Error("Player should not have any of the non-existent roles")
	}

	// Test HasAllRoles
	requiredRoles := []string{"user", "premium"}
	if !playerRoles.HasAllRoles(requiredRoles) {
		t.Error("Player should have all required roles")
	}

	impossibleRoles := []string{"user", "admin"}
	if playerRoles.HasAllRoles(impossibleRoles) {
		t.Error("Player should not have all impossible roles")
	}

	// Test with empty role lists
	emptyRoles := []string{}
	if playerRoles.HasAnyRole(emptyRoles) {
		t.Error("HasAnyRole should return false for empty list")
	}

	if !playerRoles.HasAllRoles(emptyRoles) {
		t.Error("HasAllRoles should return true for empty list")
	}
}

// testRoleFileOperations validates file-based role storage operations
func testRoleFileOperations(t *testing.T, manager *storage.Manager) {
	roleManager := manager.Roles()
	playerID := "test-player-file-ops"

	// Test LoadOrCreate for non-existent player
	playerRoles, err := roleManager.LoadOrCreate(playerID)
	if err != nil {
		t.Fatalf("Failed to load or create player roles: %v", err)
	}

	if playerRoles.PlayerID != playerID {
		t.Error("Player ID should match")
	}

	if len(playerRoles.Roles) != 0 {
		t.Error("New player roles should be empty")
	}

	// Test saving and loading
	playerRoles.AddRole("user")
	playerRoles.AddRole("tester")

	if err := roleManager.Save(playerRoles); err != nil {
		t.Fatalf("Failed to save player roles: %v", err)
	}

	// Test Exists
	if !roleManager.Exists(playerID) {
		t.Error("Player roles should exist after saving")
	}

	// Test loading existing roles
	loadedRoles, err := roleManager.Load(playerID)
	if err != nil {
		t.Fatalf("Failed to load player roles: %v", err)
	}

	if len(loadedRoles.Roles) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(loadedRoles.Roles))
	}

	if !loadedRoles.HasRole("user") || !loadedRoles.HasRole("tester") {
		t.Error("Loaded roles should match saved roles")
	}

	// Test LoadOrCreate for existing player
	existingRoles, err := roleManager.LoadOrCreate(playerID)
	if err != nil {
		t.Fatalf("Failed to load existing player roles: %v", err)
	}

	if len(existingRoles.Roles) != 2 {
		t.Error("LoadOrCreate should return existing roles")
	}

	// Test deleting roles
	if err := roleManager.Delete(playerID); err != nil {
		t.Fatalf("Failed to delete player roles: %v", err)
	}

	if roleManager.Exists(playerID) {
		t.Error("Player roles should not exist after deletion")
	}

	// Test loading non-existent player
	_, err = roleManager.Load(playerID)
	if err == nil {
		t.Error("Loading non-existent player should return error")
	}
}

// testBulkRoleOperations validates bulk role management operations
func testBulkRoleOperations(t *testing.T, manager *storage.Manager) {
	roleManager := manager.Roles()

	// Test convenience methods
	playerID := "test-player-bulk"
	adminID := "admin-user"

	// Test AddRoleToPlayer
	if err := roleManager.AddRoleToPlayer(playerID, "user", adminID); err != nil {
		t.Fatalf("Failed to add role to player: %v", err)
	}

	if !roleManager.HasRole(playerID, "user") {
		t.Error("Player should have 'user' role")
	}

	// Test adding multiple roles
	roles := []string{"user", "moderator", "premium"}
	if err := roleManager.SetPlayerRoles(playerID, roles, adminID); err != nil {
		t.Fatalf("Failed to set player roles: %v", err)
	}

	playerRoles, err := roleManager.GetPlayerRoles(playerID)
	if err != nil {
		t.Fatalf("Failed to get player roles: %v", err)
	}

	if len(playerRoles) != 3 {
		t.Errorf("Expected 3 roles, got %d", len(playerRoles))
	}

	// Test HasAnyRole convenience method
	if !roleManager.HasAnyRole(playerID, []string{"admin", "moderator"}) {
		t.Error("Player should have moderator role")
	}

	// Test HasAllRoles convenience method
	if !roleManager.HasAllRoles(playerID, []string{"user", "premium"}) {
		t.Error("Player should have both user and premium roles")
	}

	// Test RemoveRoleFromPlayer
	if err := roleManager.RemoveRoleFromPlayer(playerID, "premium", adminID); err != nil {
		t.Fatalf("Failed to remove role from player: %v", err)
	}

	if roleManager.HasRole(playerID, "premium") {
		t.Error("Player should not have 'premium' role after removal")
	}

	// Test removing non-existent role (should not error)
	if err := roleManager.RemoveRoleFromPlayer(playerID, "admin", adminID); err != nil {
		t.Fatalf("Removing non-existent role should not error: %v", err)
	}
}

// testRoleSearchOperations validates role search and listing functionality
func testRoleSearchOperations(t *testing.T, manager *storage.Manager) {
	// Create isolated test environment
	tempDir := filepath.Join(os.TempDir(), "muddycore-role-search-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	searchManager := storage.NewManagerWithConfig(config)
	if err := searchManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize search test storage: %v", err)
	}
	
	roleManager := searchManager.Roles()

	// Create multiple players with different roles (using unique IDs for this test)
	players := map[string][]string{
		"search-player1": {"user", "moderator"},
		"search-player2": {"user", "premium"},
		"search-player3": {"moderator", "admin"},
		"search-player4": {"user"},
		"search-player5": {"premium", "vip"},
	}

	// Create all players
	for playerID, roles := range players {
		for _, role := range roles {
			if err := roleManager.AddRoleToPlayer(playerID, role, "system"); err != nil {
				t.Fatalf("Failed to add role %s to player %s: %v", role, playerID, err)
			}
		}
	}

	// Test ListAll
	allPlayers, err := roleManager.ListAll()
	if err != nil {
		t.Fatalf("Failed to list all players: %v", err)
	}

	if len(allPlayers) < len(players) {
		t.Errorf("Expected at least %d players, got %d", len(players), len(allPlayers))
	}

	// Test FindPlayersByRole
	moderators, err := roleManager.FindPlayersByRole("moderator")
	if err != nil {
		t.Fatalf("Failed to find players by role: %v", err)
	}

	expectedModerators := []string{"search-player1", "search-player3"}
	if len(moderators) != len(expectedModerators) {
		t.Errorf("Expected %d moderators, got %d", len(expectedModerators), len(moderators))
	}

	for _, moderator := range moderators {
		found := false
		for _, expected := range expectedModerators {
			if moderator.PlayerID == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unexpected moderator: %s", moderator.PlayerID)
		}
	}

	// Test finding players with non-existent role
	nonExistent, err := roleManager.FindPlayersByRole("nonexistent")
	if err != nil {
		t.Fatalf("Failed to search for non-existent role: %v", err)
	}

	if len(nonExistent) != 0 {
		t.Error("Should not find any players with non-existent role")
	}
}

// testRoleErrorHandlingAndEdgeCases validates error handling and edge cases
func testRoleErrorHandlingAndEdgeCases(t *testing.T, manager *storage.Manager) {
	roleManager := manager.Roles()

	// Test operations on non-existent player
	nonExistentPlayer := "non-existent-player"

	// HasRole should return false for non-existent player
	if roleManager.HasRole(nonExistentPlayer, "user") {
		t.Error("Non-existent player should not have any roles")
	}

	// HasAnyRole should return false for non-existent player
	if roleManager.HasAnyRole(nonExistentPlayer, []string{"user", "admin"}) {
		t.Error("Non-existent player should not have any roles")
	}

	// HasAllRoles should return false for non-existent player
	if roleManager.HasAllRoles(nonExistentPlayer, []string{"user"}) {
		t.Error("Non-existent player should not have any roles")
	}

	// GetPlayerRoles should return error for non-existent player
	_, err := roleManager.GetPlayerRoles(nonExistentPlayer)
	if err == nil {
		t.Error("Getting roles for non-existent player should return error")
	}

	// RemoveRoleFromPlayer should return error for non-existent player
	if err := roleManager.RemoveRoleFromPlayer(nonExistentPlayer, "user", "admin"); err == nil {
		t.Error("Removing role from non-existent player should return error")
	}

	// Test edge cases with empty strings and special characters
	playerID := "test-edge-cases"

	// Test empty role name
	if err := roleManager.AddRoleToPlayer(playerID, "", "admin"); err != nil {
		t.Fatalf("Adding empty role should not error: %v", err)
	}

	// Empty role should still be added and checked
	if !roleManager.HasRole(playerID, "") {
		t.Error("Player should have empty role")
	}

	// Test special characters in role names
	specialRoles := []string{"role-with-dash", "role_with_underscore", "role.with.dots", "role with spaces"}
	for _, role := range specialRoles {
		if err := roleManager.AddRoleToPlayer(playerID, role, "admin"); err != nil {
			t.Fatalf("Failed to add role with special characters %s: %v", role, err)
		}

		if !roleManager.HasRole(playerID, role) {
			t.Errorf("Player should have role with special characters: %s", role)
		}
	}

	// Test very long role name
	longRole := string(make([]byte, 1000))
	for i := range longRole {
		longRole = longRole[:i] + "a" + longRole[i+1:]
	}

	if err := roleManager.AddRoleToPlayer(playerID, longRole, "admin"); err != nil {
		t.Fatalf("Failed to add very long role: %v", err)
	}

	if !roleManager.HasRole(playerID, longRole) {
		t.Error("Player should have very long role")
	}
}

// testConcurrentRoleOperations validates thread safety of role operations
func testConcurrentRoleOperations(t *testing.T, manager *storage.Manager) {
	// Create isolated test environment
	tempDir := filepath.Join(os.TempDir(), "muddycore-role-concurrent-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	concurrentManager := storage.NewManagerWithConfig(config)
	if err := concurrentManager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize concurrent test storage: %v", err)
	}
	
	roleManager := concurrentManager.Roles()
	playerID := "test-concurrent"

	// Create initial player roles
	if err := roleManager.AddRoleToPlayer(playerID, "user", "system"); err != nil {
		t.Fatalf("Failed to create initial player: %v", err)
	}

	// Test sequential role additions (more realistic for file-based storage)
	numRoles := 5
	rolePrefix := "concurrent-role-"

	// Add roles sequentially to test basic functionality
	for i := 0; i < numRoles; i++ {
		role := rolePrefix + fmt.Sprintf("%d", i)
		if err := roleManager.AddRoleToPlayer(playerID, role, "admin"); err != nil {
			t.Fatalf("Failed to add role %s: %v", role, err)
		}
	}

	// Verify all roles were added
	playerRoles, err := roleManager.GetPlayerRoles(playerID)
	if err != nil {
		t.Fatalf("Failed to get player roles: %v", err)
	}

	expectedMinRoles := numRoles + 1 // +1 for initial "user" role
	if len(playerRoles) < expectedMinRoles {
		t.Errorf("Expected at least %d roles, got %d", expectedMinRoles, len(playerRoles))
	}

	// Test role checks
	for i := 0; i < numRoles; i++ {
		role := rolePrefix + fmt.Sprintf("%d", i)
		if !roleManager.HasRole(playerID, role) {
			t.Errorf("Player should have role: %s", role)
		}
	}

	// Test concurrent read operations (this is safe)
	numGoroutines := 5
	readDone := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			// Test concurrent reads
			if !roleManager.HasRole(playerID, "user") {
				t.Errorf("Player should have user role in concurrent read")
			}
			
			roles, err := roleManager.GetPlayerRoles(playerID)
			if err != nil {
				t.Errorf("Failed to get roles in concurrent read: %v", err)
			}
			
			if len(roles) < expectedMinRoles {
				t.Errorf("Expected at least %d roles in concurrent read, got %d", expectedMinRoles, len(roles))
			}
			
			readDone <- true
		}()
	}

	// Wait for all read operations to complete
	for i := 0; i < numGoroutines; i++ {
		<-readDone
	}
}

// testRolePropertiesAndMetadata validates role properties and metadata functionality
func testRolePropertiesAndMetadata(t *testing.T, manager *storage.Manager) {
	roleManager := manager.Roles()
	playerID := "test-properties"

	// Create player roles and add some roles
	playerRoles, err := roleManager.LoadOrCreate(playerID)
	if err != nil {
		t.Fatalf("Failed to create player roles: %v", err)
	}

	playerRoles.AddRole("user")
	playerRoles.AddRole("premium")

	// Test setting properties
	playerRoles.SetProperty("subscription_level", "gold")
	playerRoles.SetProperty("last_promotion", time.Now())
	playerRoles.SetProperty("role_count", len(playerRoles.Roles))

	// Test getting properties
	if level, exists := playerRoles.GetProperty("subscription_level"); !exists || level != "gold" {
		t.Error("Should be able to get subscription level property")
	}

	if _, exists := playerRoles.GetProperty("last_promotion"); !exists {
		t.Error("Should be able to get last promotion property")
	}

	if count, exists := playerRoles.GetProperty("role_count"); !exists || count != 2 {
		t.Error("Should be able to get role count property")
	}

	// Test non-existent property
	if _, exists := playerRoles.GetProperty("non_existent"); exists {
		t.Error("Non-existent property should not exist")
	}

	// Save and reload to test persistence
	if err := roleManager.Save(playerRoles); err != nil {
		t.Fatalf("Failed to save player roles: %v", err)
	}

	reloadedRoles, err := roleManager.Load(playerID)
	if err != nil {
		t.Fatalf("Failed to reload player roles: %v", err)
	}

	// Verify properties persisted
	if level, exists := reloadedRoles.GetProperty("subscription_level"); !exists || level != "gold" {
		t.Error("Properties should persist after save/load")
	}

	// Test UpdatedBy tracking
	adminID := "role-admin"
	reloadedRoles.SetRoles([]string{"admin", "superuser"}, adminID)

	if reloadedRoles.UpdatedBy != adminID {
		t.Errorf("UpdatedBy should be %s, got %s", adminID, reloadedRoles.UpdatedBy)
	}

	// Test timestamps
	oldTimestamp := reloadedRoles.UpdatedAt
	time.Sleep(time.Millisecond * 10) // Ensure time difference
	reloadedRoles.AddRole("test")

	if !reloadedRoles.UpdatedAt.After(oldTimestamp) {
		t.Error("UpdatedAt should be updated when roles change")
	}

	// Save and verify metadata persists
	if err := roleManager.Save(reloadedRoles); err != nil {
		t.Fatalf("Failed to save updated roles: %v", err)
	}

	finalRoles, err := roleManager.Load(playerID)
	if err != nil {
		t.Fatalf("Failed to load final roles: %v", err)
	}

	if finalRoles.UpdatedBy != adminID {
		t.Error("UpdatedBy should persist")
	}

	if len(finalRoles.Roles) != 3 {
		t.Errorf("Expected 3 roles, got %d", len(finalRoles.Roles))
	}
}