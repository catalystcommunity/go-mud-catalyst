package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestAdvancedContainerOperations(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-advanced-container-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	player, inventory, err := manager.CreateNewPlayer("containeruser", "container@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Create different types of containers
	weightBasedBag := storage.NewItem("weight-bag", "Weight-based Bag")
	weightBasedBag.IsContainer = true
	weightBasedBag.MaxContainerSize = 100 // 100 weight units
	weightBasedBag.ContainerSizeUnits = "weight"
	if err := manager.Items().Save(weightBasedBag); err != nil {
		t.Fatalf("Failed to save weight-based bag: %v", err)
	}

	volumeBasedBox := storage.NewItem("volume-box", "Volume-based Box")
	volumeBasedBox.IsContainer = true
	volumeBasedBox.MaxContainerSize = 50 // 50 volume units
	volumeBasedBox.ContainerSizeUnits = "volume"
	if err := manager.Items().Save(volumeBasedBox); err != nil {
		t.Fatalf("Failed to save volume-based box: %v", err)
	}

	// Create items with different properties
	heavyItem := storage.NewItem("heavy-item", "Heavy Item")
	heavyItem.Weight = 20
	heavyItem.Volume = 5
	heavyItem.CanBeContained = true
	if err := manager.Items().Save(heavyItem); err != nil {
		t.Fatalf("Failed to save heavy item: %v", err)
	}

	bulkyItem := storage.NewItem("bulky-item", "Bulky Item")
	bulkyItem.Weight = 5
	bulkyItem.Volume = 30
	bulkyItem.CanBeContained = true
	if err := manager.Items().Save(bulkyItem); err != nil {
		t.Fatalf("Failed to save bulky item: %v", err)
	}

	nonContainableItem := storage.NewItem("non-containable", "Non-containable Item")
	nonContainableItem.CanBeContained = false
	if err := manager.Items().Save(nonContainableItem); err != nil {
		t.Fatalf("Failed to save non-containable item: %v", err)
	}

	// Give items to player
	err = manager.GiveItemToPlayer(player.ID, weightBasedBag.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give weight bag: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, volumeBasedBox.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give volume box: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, heavyItem.ID, 3)
	if err != nil {
		t.Fatalf("Failed to give heavy items: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, bulkyItem.ID, 2)
	if err != nil {
		t.Fatalf("Failed to give bulky items: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, nonContainableItem.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give non-containable item: %v", err)
	}

	// Reload inventory
	inventory, err = manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	// Find instances
	var weightBagInstance, volumeBoxInstance, heavyItemInstance, bulkyItemInstance, nonContainableInstance *storage.ItemInstance
	for _, instance := range inventory.Items {
		switch instance.ItemID {
		case weightBasedBag.ID:
			weightBagInstance = instance
		case volumeBasedBox.ID:
			volumeBoxInstance = instance
		case heavyItem.ID:
			heavyItemInstance = instance
		case bulkyItem.ID:
			bulkyItemInstance = instance
		case nonContainableItem.ID:
			nonContainableInstance = instance
		}
	}

	// Test weight-based container capacity
	err = manager.Containers().PutItemInContainer(inventory, heavyItemInstance.ID, weightBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to put heavy item in weight bag: %v", err)
	}

	// Check capacity calculation
	capacity, err := manager.Containers().GetContainerInfo(inventory, weightBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get weight bag capacity: %v", err)
	}

	expectedUsed := heavyItem.Weight * heavyItemInstance.Quantity // 20 * 3 = 60
	if capacity.UsedCapacity != expectedUsed {
		t.Errorf("Expected used capacity %d, got %d", expectedUsed, capacity.UsedCapacity)
	}

	if capacity.Available != 40 { // 100 - 60
		t.Errorf("Expected available capacity 40, got %d", capacity.Available)
	}

	if capacity.Unit != "weight" {
		t.Errorf("Expected unit 'weight', got '%s'", capacity.Unit)
	}

	// Test volume-based container (should fail - 2 items * 30 volume = 60 > 50 capacity)
	err = manager.Containers().PutItemInContainer(inventory, bulkyItemInstance.ID, volumeBoxInstance.ID)
	if err == nil {
		t.Error("Should not be able to put bulky items exceeding volume capacity")
	}

	capacity, err = manager.Containers().GetContainerInfo(inventory, volumeBoxInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get volume box capacity: %v", err)
	}

	// Since the operation failed, capacity should be 0
	if capacity.UsedCapacity != 0 {
		t.Errorf("Expected 0 used capacity after failed operation, got %d", capacity.UsedCapacity)
	}

	// Test putting non-containable item
	err = manager.Containers().PutItemInContainer(inventory, nonContainableInstance.ID, weightBagInstance.ID)
	if err == nil {
		t.Error("Expected error when putting non-containable item in container")
	}

	// Test getting container contents
	contents, err := manager.Containers().GetContainerContents(inventory, weightBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get container contents: %v", err)
	}

	if len(contents) != 1 {
		t.Errorf("Expected 1 item in weight bag, got %d", len(contents))
	}

	// Test removing item from container
	err = manager.Containers().RemoveItemFromContainer(inventory, heavyItemInstance.ID)
	if err != nil {
		t.Fatalf("Failed to remove item from container: %v", err)
	}

	// Verify item is no longer in container
	contents, err = manager.Containers().GetContainerContents(inventory, weightBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get container contents after removal: %v", err)
	}

	if len(contents) != 0 {
		t.Errorf("Expected 0 items in weight bag after removal, got %d", len(contents))
	}

	// Test removing item that's not in a container
	err = manager.Containers().RemoveItemFromContainer(inventory, heavyItemInstance.ID)
	if err == nil {
		t.Error("Expected error when removing item that's not in a container")
	}
}

func TestNestedContainers(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-nested-container-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	player, inventory, err := manager.CreateNewPlayer("nesteduser", "nested@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Create containers
	largeBag := storage.NewItem("large-bag", "Large Bag")
	largeBag.IsContainer = true
	largeBag.MaxContainerSize = 20
	largeBag.CanBeContained = true
	if err := manager.Items().Save(largeBag); err != nil {
		t.Fatalf("Failed to save large bag: %v", err)
	}

	smallBox := storage.NewItem("small-box", "Small Box")
	smallBox.IsContainer = true
	smallBox.MaxContainerSize = 5
	smallBox.CanBeContained = true
	if err := manager.Items().Save(smallBox); err != nil {
		t.Fatalf("Failed to save small box: %v", err)
	}

	coin := storage.NewItem("coin", "Gold Coin")
	coin.CanBeContained = true
	if err := manager.Items().Save(coin); err != nil {
		t.Fatalf("Failed to save coin: %v", err)
	}

	// Give items to player
	err = manager.GiveItemToPlayer(player.ID, largeBag.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give large bag: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, smallBox.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give small box: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, coin.ID, 3)
	if err != nil {
		t.Fatalf("Failed to give coins: %v", err)
	}

	// Reload inventory
	inventory, err = manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	// Find instances
	var largeBagInstance, smallBoxInstance, coinInstance *storage.ItemInstance
	for _, instance := range inventory.Items {
		switch instance.ItemID {
		case largeBag.ID:
			largeBagInstance = instance
		case smallBox.ID:
			smallBoxInstance = instance
		case coin.ID:
			coinInstance = instance
		}
	}

	// Put small box in large bag
	err = manager.Containers().PutItemInContainer(inventory, smallBoxInstance.ID, largeBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to put small box in large bag: %v", err)
	}

	// Put coins in small box
	err = manager.Containers().PutItemInContainer(inventory, coinInstance.ID, smallBoxInstance.ID)
	if err != nil {
		t.Fatalf("Failed to put coins in small box: %v", err)
	}

	// Test getting all nested items
	allNestedItems, err := manager.Containers().GetAllNestedItems(inventory, largeBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get all nested items: %v", err)
	}

	// Should contain the small box and the coins
	if len(allNestedItems) != 2 {
		t.Errorf("Expected 2 nested items (box + coins), got %d", len(allNestedItems))
	}

	// Test calculating total container weight
	totalWeight, err := manager.Containers().CalculateTotalContainerWeight(inventory, largeBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to calculate total container weight: %v", err)
	}

	// Large bag weight + small box weight + coins weight
	expectedWeight := largeBag.Weight + smallBox.Weight + (coin.Weight * coinInstance.Quantity)
	if totalWeight != expectedWeight {
		t.Errorf("Expected total weight %d, got %d", expectedWeight, totalWeight)
	}
}

func TestContainerLoopPrevention(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-loop-prevention-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	player, inventory, err := manager.CreateNewPlayer("loopuser", "loop@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Create multiple containers
	bagA := storage.NewItem("bag-a", "Bag A")
	bagA.IsContainer = true
	bagA.MaxContainerSize = 10
	bagA.CanBeContained = true
	if err := manager.Items().Save(bagA); err != nil {
		t.Fatalf("Failed to save bag A: %v", err)
	}

	bagB := storage.NewItem("bag-b", "Bag B")
	bagB.IsContainer = true
	bagB.MaxContainerSize = 10
	bagB.CanBeContained = true
	if err := manager.Items().Save(bagB); err != nil {
		t.Fatalf("Failed to save bag B: %v", err)
	}

	bagC := storage.NewItem("bag-c", "Bag C")
	bagC.IsContainer = true
	bagC.MaxContainerSize = 10
	bagC.CanBeContained = true
	if err := manager.Items().Save(bagC); err != nil {
		t.Fatalf("Failed to save bag C: %v", err)
	}

	// Give bags to player
	err = manager.GiveItemToPlayer(player.ID, bagA.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give bag A: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, bagB.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give bag B: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, bagC.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give bag C: %v", err)
	}

	// Reload inventory
	inventory, err = manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	// Find instances
	var bagAInstance, bagBInstance, bagCInstance *storage.ItemInstance
	for _, instance := range inventory.Items {
		switch instance.ItemID {
		case bagA.ID:
			bagAInstance = instance
		case bagB.ID:
			bagBInstance = instance
		case bagC.ID:
			bagCInstance = instance
		}
	}

	// Create a chain: A contains B, B contains C
	err = manager.Containers().PutItemInContainer(inventory, bagBInstance.ID, bagAInstance.ID)
	if err != nil {
		t.Fatalf("Failed to put bag B in bag A: %v", err)
	}

	err = manager.Containers().PutItemInContainer(inventory, bagCInstance.ID, bagBInstance.ID)
	if err != nil {
		t.Fatalf("Failed to put bag C in bag B: %v", err)
	}

	// Now try to create a loop: put A in C (should fail)
	err = manager.Containers().PutItemInContainer(inventory, bagAInstance.ID, bagCInstance.ID)
	if err == nil {
		t.Error("Expected error when creating container loop")
	}

	// Test direct self-containment (bagA is not in any container, so we can test directly)
	err = manager.Containers().PutItemInContainer(inventory, bagAInstance.ID, bagAInstance.ID)
	if err == nil {
		t.Error("Expected error when putting container in itself")
	}
}

func TestContainerCapacityValidation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-capacity-validation-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	player, inventory, err := manager.CreateNewPlayer("capacityuser", "capacity@example.com")
	if err != nil {
		t.Fatalf("Failed to create player: %v", err)
	}

	// Create a small container
	smallBag := storage.NewItem("small-bag", "Small Bag")
	smallBag.IsContainer = true
	smallBag.MaxContainerSize = 3 // Only holds 3 items
	if err := manager.Items().Save(smallBag); err != nil {
		t.Fatalf("Failed to save small bag: %v", err)
	}

	// Create many small items
	pebble := storage.NewItem("pebble", "Small Pebble")
	pebble.CanBeContained = true
	if err := manager.Items().Save(pebble); err != nil {
		t.Fatalf("Failed to save pebble: %v", err)
	}

	// Give items to player
	err = manager.GiveItemToPlayer(player.ID, smallBag.ID, 1)
	if err != nil {
		t.Fatalf("Failed to give small bag: %v", err)
	}

	err = manager.GiveItemToPlayer(player.ID, pebble.ID, 5)
	if err != nil {
		t.Fatalf("Failed to give pebbles: %v", err)
	}

	// Reload inventory
	inventory, err = manager.Inventories().LoadByOwner(player.ID)
	if err != nil {
		t.Fatalf("Failed to reload inventory: %v", err)
	}

	// Find instances
	var smallBagInstance, pebbleInstance *storage.ItemInstance
	for _, instance := range inventory.Items {
		switch instance.ItemID {
		case smallBag.ID:
			smallBagInstance = instance
		case pebble.ID:
			pebbleInstance = instance
		}
	}

	// Put pebbles in bag (should fail because 5 > 3 capacity)
	err = manager.Containers().PutItemInContainer(inventory, pebbleInstance.ID, smallBagInstance.ID)
	if err == nil {
		t.Error("Expected error when putting too many items in small container")
	}

	// Test with individual pebbles
	// Split the pebble stack into individual items for testing
	for i := 0; i < 3; i++ {
		pebbleInstanceSingle := storage.NewItemInstance("pebble-"+string(rune(i+48)), pebble.ID, 1)
		inventory.AddItem(pebbleInstanceSingle)

		err = manager.Containers().PutItemInContainer(inventory, pebbleInstanceSingle.ID, smallBagInstance.ID)
		if err != nil {
			t.Errorf("Failed to put pebble %d in bag: %v", i+1, err)
		}
	}

	// Try to put one more (should fail)
	extraPebble := storage.NewItemInstance("extra-pebble", pebble.ID, 1)
	inventory.AddItem(extraPebble)

	err = manager.Containers().PutItemInContainer(inventory, extraPebble.ID, smallBagInstance.ID)
	if err == nil {
		t.Error("Expected error when exceeding container capacity")
	}

	// Verify container is full
	capacity, err := manager.Containers().GetContainerInfo(inventory, smallBagInstance.ID)
	if err != nil {
		t.Fatalf("Failed to get container capacity: %v", err)
	}

	if capacity.Available != 0 {
		t.Errorf("Expected 0 available capacity, got %d", capacity.Available)
	}

	if capacity.UsedCapacity != 3 {
		t.Errorf("Expected 3 used capacity, got %d", capacity.UsedCapacity)
	}
}