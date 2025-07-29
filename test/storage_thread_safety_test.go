package test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

var testCounter int64

// TestConcurrentPlayerCreation tests concurrent player creation with username checking
func TestConcurrentPlayerCreation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-concurrent-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}
	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage manager: %v", err)
	}

	const numGoroutines = 50
	
	var wg sync.WaitGroup
	results := make(chan error, numGoroutines)
	
	// All goroutines try to create players with unique usernames
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			username := fmt.Sprintf("testuser_%d_%d_%d", time.Now().UnixNano(), id, atomic.AddInt64(&testCounter, 1))
			email := fmt.Sprintf("test%d@example.com", id)
			_, _, err := manager.CreateNewPlayer(username, email)
			results <- err
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	// Count successful and failed creations
	successCount := 0
	failureCount := 0
	
	for err := range results {
		if err == nil {
			successCount++
		} else {
			failureCount++
			t.Logf("Creation failed: %v", err)
		}
	}
	
	// All should succeed with unique usernames
	if successCount != numGoroutines {
		t.Errorf("Expected %d successful creations, got %d", numGoroutines, successCount)
	}
	
	if failureCount != 0 {
		t.Errorf("Expected 0 failures, got %d", failureCount)
	}
}

// TestConcurrentItemQuantityOperations tests concurrent item quantity increment/decrement
func TestConcurrentItemQuantityOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-item-quantity-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}
	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage manager: %v", err)
	}

	// Create a test item with quantity limits
	item, err := manager.CreateNewItem("TestItem", "Test item for concurrency")
	if err != nil {
		t.Fatalf("Failed to create test item: %v", err)
	}
	
	// Set quantity limits
	item.IsQuantityLimited = true
	item.MaxQuantity = 1000
	if err := manager.Items().Save(item); err != nil {
		t.Fatalf("Failed to save item: %v", err)
	}

	const numGoroutines = 100
	const incrementAmount = 5
	
	var wg sync.WaitGroup
	
	// Half the goroutines increment, half decrement
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.IncrementItemQuantity(item.ID, incrementAmount)
		}()
	}
	
	// Add a small delay to ensure some increments happen first
	time.Sleep(10 * time.Millisecond)
	
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.DecrementItemQuantity(item.ID, incrementAmount)
		}()
	}
	
	wg.Wait()
	
	// Check final quantity - should be consistent
	finalItem, err := manager.Items().Load(item.ID)
	if err != nil {
		t.Fatalf("Failed to load final item: %v", err)
	}
	
	// Final quantity should be 0 (equal increments and decrements)
	if finalItem.CurrentQuantity != 0 {
		t.Errorf("Expected final quantity to be 0, got %d", finalItem.CurrentQuantity)
	}
}

// TestConcurrentInventoryOperations tests concurrent inventory modifications
func TestConcurrentInventoryOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-inventory-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}
	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage manager: %v", err)
	}

	// Create test player and item
	username := fmt.Sprintf("inventorytest_%d_%d", time.Now().UnixNano(), atomic.AddInt64(&testCounter, 1))
	player, _, err := manager.CreateNewPlayer(username, "test@example.com")
	if err != nil {
		t.Fatalf("Failed to create test player: %v", err)
	}
	
	item, err := manager.CreateNewItem("TestInventoryItem", "Test item for inventory")
	if err != nil {
		t.Fatalf("Failed to create test item: %v", err)
	}
	
	// Set high quantity limits
	item.IsQuantityLimited = true
	item.MaxQuantity = 10000
	if err := manager.Items().Save(item); err != nil {
		t.Fatalf("Failed to save item: %v", err)
	}

	const numGoroutines = 50
	const itemQuantity = 10
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	
	// All goroutines try to give items to the player
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := manager.GiveItemToPlayer(player.ID, item.ID, itemQuantity)
			errors <- err
		}()
	}
	
	wg.Wait()
	close(errors)
	
	// Count errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Error during inventory operation: %v", err)
		}
	}
	
	// Load final inventory
	inventory, err := manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to load final inventory: %v", err)
	}
	
	// Calculate total items in inventory
	totalItems := 0
	for _, instance := range inventory.Items {
		if instance.ItemID == item.ID {
			totalItems += instance.Quantity
		}
	}
	
	// Check item quantity consistency
	finalItem, err := manager.Items().Load(item.ID)
	if err != nil {
		t.Fatalf("Failed to load final item: %v", err)
	}
	
	// Total quantity should match what's in inventory
	if finalItem.CurrentQuantity != totalItems {
		t.Errorf("Quantity mismatch: item shows %d, inventory has %d", 
			finalItem.CurrentQuantity, totalItems)
	}
	
	// Should have expected total if no errors
	if errorCount == 0 {
		expectedTotal := numGoroutines * itemQuantity
		if totalItems != expectedTotal {
			t.Errorf("Expected %d total items, got %d", expectedTotal, totalItems)
		}
	}
}

// TestConcurrentWorldOperations tests concurrent world and room operations
func TestConcurrentWorldOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}
	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage manager: %v", err)
	}

	// Create a test world
	world, err := manager.CreateNewWorld("TestWorld", "Test world for concurrency")
	if err != nil {
		t.Fatalf("Failed to create test world: %v", err)
	}
	
	// Create source and target rooms
	sourceRoom, err := manager.CreateNewRoom(world.ID, "SourceRoom", "Source room")
	if err != nil {
		t.Fatalf("Failed to create source room: %v", err)
	}
	
	targetRoom, err := manager.CreateNewRoom(world.ID, "TargetRoom", "Target room")
	if err != nil {
		t.Fatalf("Failed to create target room: %v", err)
	}
	
	// Allow connection from source to target
	targetRoom.AddAllowedInlet(sourceRoom.ID)
	if err := manager.Worlds().SaveRoom(targetRoom); err != nil {
		t.Fatalf("Failed to save target room: %v", err)
	}

	const numGoroutines = 20
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)
	
	// All goroutines try to create the same exit
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			direction := fmt.Sprintf("north_%d", id)
			err := manager.Worlds().CreateExitWithValidation(
				world.ID, sourceRoom.ID, direction, world.ID, targetRoom.ID)
			errors <- err
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Error during world operation: %v", err)
		}
	}
	
	// Load final source room
	finalSourceRoom, err := manager.Worlds().LoadRoom(world.ID, sourceRoom.ID)
	if err != nil {
		t.Fatalf("Failed to load final source room: %v", err)
	}
	
	// Should have created multiple exits (one per goroutine if no errors)
	if errorCount == 0 && len(finalSourceRoom.Exits) != numGoroutines {
		t.Errorf("Expected %d exits, got %d", numGoroutines, len(finalSourceRoom.Exits))
	}
}

// TestConcurrentPlayerMovement tests concurrent player movement operations
func TestConcurrentPlayerMovement(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-movement-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}
	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage manager: %v", err)
	}

	// Create world and rooms
	world, err := manager.CreateNewWorld("MovementWorld", "World for movement testing")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}
	
	room1, err := manager.CreateNewRoom(world.ID, "Room1", "First room")
	if err != nil {
		t.Fatalf("Failed to create room1: %v", err)
	}
	
	room2, err := manager.CreateNewRoom(world.ID, "Room2", "Second room")
	if err != nil {
		t.Fatalf("Failed to create room2: %v", err)
	}
	
	// Create multiple players
	const numPlayers = 20
	players := make([]*storage.Player, numPlayers)
	
	for i := 0; i < numPlayers; i++ {
		username := fmt.Sprintf("movementtest%d_%d_%d", time.Now().UnixNano(), i, atomic.AddInt64(&testCounter, 1))
		player, _, err := manager.CreateNewPlayer(username, fmt.Sprintf("test%d@example.com", i))
		if err != nil {
			t.Fatalf("Failed to create player %d: %v", i, err)
		}
		players[i] = player
	}
	
	var wg sync.WaitGroup
	errors := make(chan error, numPlayers*2)
	
	// Move all players between rooms concurrently
	for i := 0; i < numPlayers; i++ {
		wg.Add(2)
		go func(player *storage.Player) {
			defer wg.Done()
			err := manager.MovePlayerToRoom(player.ID, world.ID, room1.ID)
			errors <- err
		}(players[i])
		
		go func(player *storage.Player) {
			defer wg.Done()
			err := manager.MovePlayerToRoom(player.ID, world.ID, room2.ID)
			errors <- err
		}(players[i])
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Error during player movement: %v", err)
		}
	}
	
	// Verify all players are in valid rooms
	for i, player := range players {
		finalPlayer, err := manager.Players().Load(player.ID)
		if err != nil {
			t.Errorf("Failed to load final player %d: %v", i, err)
			continue
		}
		
		if finalPlayer.CurrentWorldID != world.ID {
			t.Errorf("Player %d not in correct world: %s", i, finalPlayer.CurrentWorldID)
		}
		
		if finalPlayer.CurrentRoomID != room1.ID && finalPlayer.CurrentRoomID != room2.ID {
			t.Errorf("Player %d in invalid room: %s", i, finalPlayer.CurrentRoomID)
		}
	}
	
	if errorCount > 0 {
		t.Logf("Total movement errors: %d", errorCount)
	}
}

// TestStorageSystemStability tests the overall stability under concurrent load
func TestStorageSystemStability(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-stability-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}
	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage manager: %v", err)
	}

	const duration = 5 * time.Second
	const numWorkers = 10
	
	var wg sync.WaitGroup
	stop := make(chan struct{})
	
	// Worker that creates players
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				username := fmt.Sprintf("stresstest%d_%d_%d", time.Now().UnixNano(), counter, atomic.AddInt64(&testCounter, 1))
				manager.CreateNewPlayer(username, fmt.Sprintf("stress%d@example.com", counter))
				counter++
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()
	
	// Worker that creates items
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				itemName := fmt.Sprintf("StressItem%d", counter)
				manager.CreateNewItem(itemName, "Stress test item")
				counter++
				time.Sleep(15 * time.Millisecond)
			}
		}
	}()
	
	// Worker that creates worlds
	wg.Add(1)
	go func() {
		defer wg.Done()
		counter := 0
		for {
			select {
			case <-stop:
				return
			default:
				worldName := fmt.Sprintf("StressWorld%d", counter)
				manager.CreateNewWorld(worldName, "Stress test world")
				counter++
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()
	
	// Let the workers run
	time.Sleep(duration)
	close(stop)
	wg.Wait()
	
	// System should still be responsive
	finalUsername := fmt.Sprintf("finaltest_%d_%d", time.Now().UnixNano(), atomic.AddInt64(&testCounter, 1))
	_, _, err := manager.CreateNewPlayer(finalUsername, "final@example.com")
	if err != nil {
		t.Errorf("System became unresponsive after stress test: %v", err)
	}
}