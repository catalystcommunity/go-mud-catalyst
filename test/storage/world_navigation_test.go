package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/storage"
)

// TestMultiWorldNavigation tests complex navigation scenarios across multiple worlds
func TestMultiWorldNavigation(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-navigation-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create a complex multi-world setup
	// World 1: Starting world with town and dungeon entrance
	// World 2: Dungeon world with multiple levels
	// World 3: Magical realm accessible from dungeon
	
	world1, err := manager.CreateNewWorld("Overworld", "The main world with towns and landscapes")
	if err != nil {
		t.Fatalf("Failed to create overworld: %v", err)
	}

	world2, err := manager.CreateNewWorld("Dungeon", "A dangerous underground dungeon")
	if err != nil {
		t.Fatalf("Failed to create dungeon world: %v", err)
	}

	world3, err := manager.CreateNewWorld("Magical Realm", "A mystical world of magic and wonder")
	if err != nil {
		t.Fatalf("Failed to create magical realm: %v", err)
	}

	// Create rooms in World 1 (Overworld)
	townSquare, err := manager.CreateNewRoom(world1.ID, "Town Square", "The bustling center of the town")
	if err != nil {
		t.Fatalf("Failed to create town square: %v", err)
	}

	inn, err := manager.CreateNewRoom(world1.ID, "The Prancing Pony Inn", "A cozy inn for weary travelers")
	if err != nil {
		t.Fatalf("Failed to create inn: %v", err)
	}

	dungeonEntrance, err := manager.CreateNewRoom(world1.ID, "Dungeon Entrance", "A foreboding entrance to the underground")
	if err != nil {
		t.Fatalf("Failed to create dungeon entrance: %v", err)
	}

	// Create rooms in World 2 (Dungeon)
	dungeonLevel1, err := manager.CreateNewRoom(world2.ID, "Dungeon Level 1", "The first level of the dungeon")
	if err != nil {
		t.Fatalf("Failed to create dungeon level 1: %v", err)
	}

	dungeonLevel2, err := manager.CreateNewRoom(world2.ID, "Dungeon Level 2", "Deeper into the dungeon's depths")
	if err != nil {
		t.Fatalf("Failed to create dungeon level 2: %v", err)
	}

	bossRoom, err := manager.CreateNewRoom(world2.ID, "Boss Chamber", "The lair of the dungeon boss")
	if err != nil {
		t.Fatalf("Failed to create boss room: %v", err)
	}

	// Create rooms in World 3 (Magical Realm)
	magicalGateway, err := manager.CreateNewRoom(world3.ID, "Magical Gateway", "Entry point to the magical realm")
	if err != nil {
		t.Fatalf("Failed to create magical gateway: %v", err)
	}

	enchantedForest, err := manager.CreateNewRoom(world3.ID, "Enchanted Forest", "A forest filled with magical creatures")
	if err != nil {
		t.Fatalf("Failed to create enchanted forest: %v", err)
	}

	t.Run("Basic World Navigation Setup", func(t *testing.T) {
		// Set up navigation within World 1
		townSquare.AddExit("north", world1.ID, inn.ID)
		townSquare.AddExit("south", world1.ID, dungeonEntrance.ID)
		
		inn.AddExit("south", world1.ID, townSquare.ID)
		dungeonEntrance.AddExit("north", world1.ID, townSquare.ID)

		// Set up cross-world navigation from overworld to dungeon
		dungeonEntrance.AddExit("down", world2.ID, dungeonLevel1.ID)

		// Set up navigation within World 2
		dungeonLevel1.AddExit("down", world2.ID, dungeonLevel2.ID)
		dungeonLevel1.AddExit("up", world1.ID, dungeonEntrance.ID) // Back to overworld
		
		dungeonLevel2.AddExit("up", world2.ID, dungeonLevel1.ID)
		dungeonLevel2.AddExit("east", world2.ID, bossRoom.ID)
		
		bossRoom.AddExit("west", world2.ID, dungeonLevel2.ID)
		bossRoom.AddExit("portal", world3.ID, magicalGateway.ID) // Secret portal to magical realm

		// Set up navigation within World 3
		magicalGateway.AddExit("north", world3.ID, enchantedForest.ID)
		magicalGateway.AddExit("return", world2.ID, bossRoom.ID) // Return portal
		
		enchantedForest.AddExit("south", world3.ID, magicalGateway.ID)

		// Save all rooms with their exits
		rooms := []*storage.Room{
			townSquare, inn, dungeonEntrance,
			dungeonLevel1, dungeonLevel2, bossRoom,
			magicalGateway, enchantedForest,
		}

		for _, room := range rooms {
			if err := manager.Worlds().SaveRoom(room); err != nil {
				t.Fatalf("Failed to save room %s: %v", room.Name, err)
			}
		}

		// Verify exits were created correctly
		reloadedTownSquare, err := manager.Worlds().LoadRoom(world1.ID, townSquare.ID)
		if err != nil {
			t.Fatalf("Failed to reload town square: %v", err)
		}

		if len(reloadedTownSquare.Exits) != 2 {
			t.Errorf("Town square should have 2 exits, got %d", len(reloadedTownSquare.Exits))
		}

		northExit := reloadedTownSquare.GetExit("north")
		if northExit == nil || northExit.TargetRoomID != inn.ID {
			t.Error("Town square north exit should lead to inn")
		}

		reloadedBossRoom, err := manager.Worlds().LoadRoom(world2.ID, bossRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload boss room: %v", err)
		}

		portalExit := reloadedBossRoom.GetExit("portal")
		if portalExit == nil || portalExit.TargetWorldID != world3.ID || portalExit.TargetRoomID != magicalGateway.ID {
			t.Error("Boss room portal should lead to magical gateway in world 3")
		}
	})

	t.Run("Secure Cross-World Navigation", func(t *testing.T) {
		// Now set up the secure inlet system for cross-world travel
		
		// Allow dungeon entrance to connect to dungeon level 1
		dungeonLevel1.AddAllowedInlet(dungeonEntrance.ID)
		
		// Allow returns from dungeon level 1 to entrance
		dungeonEntrance.AddAllowedInlet(dungeonLevel1.ID)
		
		// Allow boss room to connect to magical gateway
		magicalGateway.AddAllowedInlet(bossRoom.ID)
		
		// Allow return from magical gateway to boss room
		bossRoom.AddAllowedInlet(magicalGateway.ID)

		// Save rooms with inlet permissions
		roomsToSave := []*storage.Room{dungeonLevel1, dungeonEntrance, magicalGateway, bossRoom}
		for _, room := range roomsToSave {
			if err := manager.Worlds().SaveRoom(room); err != nil {
				t.Fatalf("Failed to save room %s with inlets: %v", room.Name, err)
			}
		}

		// Test cross-world exit validation
		err := manager.Worlds().ValidateExit(world1.ID, dungeonEntrance.ID, world2.ID, dungeonLevel1.ID)
		if err != nil {
			t.Errorf("Dungeon entrance to level 1 should be valid: %v", err)
		}

		err = manager.Worlds().ValidateExit(world2.ID, dungeonLevel1.ID, world1.ID, dungeonEntrance.ID)
		if err != nil {
			t.Errorf("Dungeon level 1 to entrance should be valid: %v", err)
		}

		err = manager.Worlds().ValidateExit(world2.ID, bossRoom.ID, world3.ID, magicalGateway.ID)
		if err != nil {
			t.Errorf("Boss room to magical gateway should be valid: %v", err)
		}

		err = manager.Worlds().ValidateExit(world3.ID, magicalGateway.ID, world2.ID, bossRoom.ID)
		if err != nil {
			t.Errorf("Magical gateway to boss room should be valid: %v", err)
		}

		// Test unauthorized cross-world connections
		err = manager.Worlds().ValidateExit(world1.ID, townSquare.ID, world3.ID, magicalGateway.ID)
		if err == nil {
			t.Error("Town square to magical gateway should not be valid without permission")
		}

		err = manager.Worlds().ValidateExit(world3.ID, enchantedForest.ID, world1.ID, inn.ID)
		if err == nil {
			t.Error("Enchanted forest to inn should not be valid without permission")
		}
	})

	t.Run("Complex Navigation Paths", func(t *testing.T) {
		// Test creating a full path from overworld to magical realm
		// Path: Town Square -> Dungeon Entrance -> Dungeon Level 1 -> Dungeon Level 2 -> Boss Room -> Magical Gateway

		// Verify each step of the journey is valid
		journey := []struct {
			fromWorld, fromRoom, toWorld, toRoom, direction string
		}{
			{world1.ID, townSquare.ID, world1.ID, dungeonEntrance.ID, "south"},
			{world1.ID, dungeonEntrance.ID, world2.ID, dungeonLevel1.ID, "down"},
			{world2.ID, dungeonLevel1.ID, world2.ID, dungeonLevel2.ID, "down"},
			{world2.ID, dungeonLevel2.ID, world2.ID, bossRoom.ID, "east"},
			{world2.ID, bossRoom.ID, world3.ID, magicalGateway.ID, "portal"},
		}

		for i, step := range journey {
			// Load the source room
			sourceRoom, err := manager.Worlds().LoadRoom(step.fromWorld, step.fromRoom)
			if err != nil {
				t.Fatalf("Failed to load source room for step %d: %v", i, err)
			}

			// Check if the exit exists
			exit := sourceRoom.GetExit(step.direction)
			if exit == nil {
				t.Errorf("Step %d: Exit '%s' does not exist in room", i, step.direction)
				continue
			}

			// Verify exit targets
			if exit.TargetWorldID != step.toWorld {
				t.Errorf("Step %d: Exit world mismatch, expected %s, got %s", i, step.toWorld, exit.TargetWorldID)
			}

			if exit.TargetRoomID != step.toRoom {
				t.Errorf("Step %d: Exit room mismatch, expected %s, got %s", i, step.toRoom, exit.TargetRoomID)
			}

			// Validate the exit (if cross-world)
			if step.fromWorld != step.toWorld {
				err := manager.Worlds().ValidateExit(step.fromWorld, step.fromRoom, step.toWorld, step.toRoom)
				if err != nil {
					t.Errorf("Step %d: Cross-world exit validation failed: %v", i, err)
				}
			}
		}
	})

	t.Run("Bidirectional Navigation", func(t *testing.T) {
		// Test that we can navigate both directions on each connection
		bidirectionalPairs := []struct {
			world1, room1, direction1, world2, room2, direction2 string
		}{
			{world1.ID, townSquare.ID, "north", world1.ID, inn.ID, "south"},
			{world1.ID, townSquare.ID, "south", world1.ID, dungeonEntrance.ID, "north"},
			{world1.ID, dungeonEntrance.ID, "down", world2.ID, dungeonLevel1.ID, "up"},
			{world2.ID, dungeonLevel1.ID, "down", world2.ID, dungeonLevel2.ID, "up"},
			{world2.ID, dungeonLevel2.ID, "east", world2.ID, bossRoom.ID, "west"},
			{world2.ID, bossRoom.ID, "portal", world3.ID, magicalGateway.ID, "return"},
			{world3.ID, magicalGateway.ID, "north", world3.ID, enchantedForest.ID, "south"},
		}

		for i, pair := range bidirectionalPairs {
			// Test direction 1 -> 2
			room1, err := manager.Worlds().LoadRoom(pair.world1, pair.room1)
			if err != nil {
				t.Fatalf("Failed to load room1 for pair %d: %v", i, err)
			}

			exit1 := room1.GetExit(pair.direction1)
			if exit1 == nil {
				t.Errorf("Pair %d: Exit '%s' missing from room1", i, pair.direction1)
				continue
			}

			if exit1.TargetWorldID != pair.world2 || exit1.TargetRoomID != pair.room2 {
				t.Errorf("Pair %d: Direction1 exit target mismatch", i)
			}

			// Test direction 2 -> 1
			room2, err := manager.Worlds().LoadRoom(pair.world2, pair.room2)
			if err != nil {
				t.Fatalf("Failed to load room2 for pair %d: %v", i, err)
			}

			exit2 := room2.GetExit(pair.direction2)
			if exit2 == nil {
				t.Errorf("Pair %d: Exit '%s' missing from room2", i, pair.direction2)
				continue
			}

			if exit2.TargetWorldID != pair.world1 || exit2.TargetRoomID != pair.room1 {
				t.Errorf("Pair %d: Direction2 exit target mismatch", i)
			}
		}
	})

	t.Run("Exit Properties and Metadata", func(t *testing.T) {
		// Test that exits can have custom properties for gameplay
		bossRoomReloaded, err := manager.Worlds().LoadRoom(world2.ID, bossRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload boss room: %v", err)
		}

		portalExit := bossRoomReloaded.GetExit("portal")
		if portalExit == nil {
			t.Fatal("Portal exit should exist")
		}

		// Add properties to the portal exit
		if portalExit.Properties == nil {
			portalExit.Properties = make(map[string]interface{})
		}
		portalExit.Properties["requires_boss_defeat"] = true
		portalExit.Properties["magic_level_required"] = 5
		portalExit.Properties["activation_message"] = "The portal shimmers and opens..."
		portalExit.IsHidden = true
		portalExit.Description = "A shimmering portal that appeared after defeating the boss"

		if err := manager.Worlds().SaveRoom(bossRoomReloaded); err != nil {
			t.Fatalf("Failed to save boss room with portal properties: %v", err)
		}

		// Reload and verify properties
		finalBossRoom, err := manager.Worlds().LoadRoom(world2.ID, bossRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload final boss room: %v", err)
		}

		finalPortal := finalBossRoom.GetExit("portal")
		if finalPortal == nil {
			t.Fatal("Portal should still exist after reload")
		}

		if !finalPortal.IsHidden {
			t.Error("Portal should be hidden")
		}

		if finalPortal.Description != "A shimmering portal that appeared after defeating the boss" {
			t.Error("Portal description mismatch")
		}

		requiresBoss, exists := finalPortal.Properties["requires_boss_defeat"]
		if !exists || requiresBoss != true {
			t.Error("Portal should require boss defeat")
		}

		magicLevel, exists := finalPortal.Properties["magic_level_required"]
		if !exists || magicLevel != float64(5) { // JSON numbers become float64
			t.Error("Portal should require magic level 5")
		}
	})

	t.Run("World Connectivity Analysis", func(t *testing.T) {
		// Analyze connectivity between worlds
		
		// Get all worlds
		worldIDs, err := manager.Worlds().ListWorlds()
		if err != nil {
			t.Fatalf("Failed to list worlds: %v", err)
		}

		if len(worldIDs) < 3 {
			t.Errorf("Expected at least 3 worlds, got %d", len(worldIDs))
		}

		// Check that each world has the expected number of rooms
		for _, worldID := range worldIDs {
			roomIDs, err := manager.Worlds().ListRooms(worldID)
			if err != nil {
				t.Errorf("Failed to list rooms for world %s: %v", worldID, err)
				continue
			}

			var expectedRoomCount int
			switch worldID {
			case world1.ID:
				expectedRoomCount = 3 // town square, inn, dungeon entrance
			case world2.ID:
				expectedRoomCount = 3 // level 1, level 2, boss room
			case world3.ID:
				expectedRoomCount = 2 // gateway, forest
			}

			if len(roomIDs) != expectedRoomCount {
				t.Errorf("World %s should have %d rooms, got %d", worldID, expectedRoomCount, len(roomIDs))
			}
		}

		// Verify cross-world connections exist
		crossWorldConnections := 0
		
		for _, worldID := range worldIDs {
			roomIDs, err := manager.Worlds().ListRooms(worldID)
			if err != nil {
				continue
			}

			for _, roomID := range roomIDs {
				room, err := manager.Worlds().LoadRoom(worldID, roomID)
				if err != nil {
					continue
				}

				for _, exit := range room.Exits {
					if exit.TargetWorldID != worldID {
						crossWorldConnections++
					}
				}
			}
		}

		if crossWorldConnections < 4 { // We should have at least 4 cross-world exits
			t.Errorf("Expected at least 4 cross-world connections, found %d", crossWorldConnections)
		}
	})
}

// TestWorldIntegrationWithItems tests integration between world system and item system
func TestWorldIntegrationWithItems(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "muddycore-world-item-integration-test")
	defer os.RemoveAll(tempDir)

	config := &storage.StorageConfig{
		DataRoot: tempDir,
	}

	manager := storage.NewManagerWithConfig(config)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}

	// Create test world and rooms
	world, err := manager.CreateNewWorld("Item Test World", "World for testing item integration")
	if err != nil {
		t.Fatalf("Failed to create world: %v", err)
	}

	treasureRoom, err := manager.CreateNewRoom(world.ID, "Treasure Room", "A room filled with treasures")
	if err != nil {
		t.Fatalf("Failed to create treasure room: %v", err)
	}

	shopRoom, err := manager.CreateNewRoom(world.ID, "Magic Shop", "A shop with magical items")
	if err != nil {
		t.Fatalf("Failed to create shop room: %v", err)
	}

	// Create test items
	goldCoin, err := manager.CreateNewItem("Gold Coin", "A shiny gold coin")
	if err != nil {
		t.Fatalf("Failed to create gold coin: %v", err)
	}

	magicSword, err := manager.CreateNewItem("Magic Sword", "A sword imbued with magical power")
	if err != nil {
		t.Fatalf("Failed to create magic sword: %v", err)
	}

	t.Run("Items in Rooms", func(t *testing.T) {
		// Create item instances and place them in rooms
		goldInstance, err := manager.CreateItemInstance(goldCoin.ID, 50)
		if err != nil {
			t.Fatalf("Failed to create gold instance: %v", err)
		}

		swordInstance, err := manager.CreateItemInstance(magicSword.ID, 1)
		if err != nil {
			t.Fatalf("Failed to create sword instance: %v", err)
		}

		// Add items to treasure room
		treasureRoom.AddItem(goldInstance)
		treasureRoom.AddItem(swordInstance)

		if err := manager.Worlds().SaveRoom(treasureRoom); err != nil {
			t.Fatalf("Failed to save treasure room with items: %v", err)
		}

		// Verify items are in the room
		reloadedRoom, err := manager.Worlds().LoadRoom(world.ID, treasureRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload treasure room: %v", err)
		}

		if len(reloadedRoom.Items) != 2 {
			t.Errorf("Expected 2 items in treasure room, got %d", len(reloadedRoom.Items))
		}

		// Verify item properties are set correctly
		for _, item := range reloadedRoom.Items {
			if item.WorldID != world.ID {
				t.Errorf("Item world ID should be %s, got %s", world.ID, item.WorldID)
			}

			if item.RoomID != treasureRoom.ID {
				t.Errorf("Item room ID should be %s, got %s", treasureRoom.ID, item.RoomID)
			}

			if item.OwnerID != "" {
				t.Error("Items in rooms should not have an owner")
			}
		}

		// Test finding specific items
		foundGold := reloadedRoom.FindItem(goldInstance.ID)
		if foundGold == nil {
			t.Error("Should have found gold in treasure room")
		}

		if foundGold != nil && foundGold.Quantity != 50 {
			t.Errorf("Gold quantity should be 50, got %d", foundGold.Quantity)
		}

		foundSword := reloadedRoom.FindItem(swordInstance.ID)
		if foundSword == nil {
			t.Error("Should have found sword in treasure room")
		}

		// Test removing items from room
		if !reloadedRoom.RemoveItem(goldInstance.ID) {
			t.Error("Should have successfully removed gold from room")
		}

		if len(reloadedRoom.Items) != 1 {
			t.Errorf("Expected 1 item after removal, got %d", len(reloadedRoom.Items))
		}

		if reloadedRoom.FindItem(goldInstance.ID) != nil {
			t.Error("Gold should no longer be in room after removal")
		}
	})

	t.Run("Moving Items Between Rooms", func(t *testing.T) {
		// Create another item instance
		moreGold, err := manager.CreateItemInstance(goldCoin.ID, 25)
		if err != nil {
			t.Fatalf("Failed to create more gold: %v", err)
		}

		// Add to treasure room first
		treasureRoom.AddItem(moreGold)
		if err := manager.Worlds().SaveRoom(treasureRoom); err != nil {
			t.Fatalf("Failed to save treasure room: %v", err)
		}

		// Move item from treasure room to shop room
		reloadedTreasureRoom, err := manager.Worlds().LoadRoom(world.ID, treasureRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload treasure room: %v", err)
		}

		// Find and remove the item
		foundItem := reloadedTreasureRoom.FindItem(moreGold.ID)
		if foundItem == nil {
			t.Fatal("Should have found gold in treasure room")
		}

		if !reloadedTreasureRoom.RemoveItem(moreGold.ID) {
			t.Fatal("Should have removed gold from treasure room")
		}

		// Add to shop room
		shopRoom.AddItem(foundItem)

		// Save both rooms
		if err := manager.Worlds().SaveRoom(reloadedTreasureRoom); err != nil {
			t.Fatalf("Failed to save updated treasure room: %v", err)
		}

		if err := manager.Worlds().SaveRoom(shopRoom); err != nil {
			t.Fatalf("Failed to save shop room with moved item: %v", err)
		}

		// Verify the move
		finalTreasureRoom, err := manager.Worlds().LoadRoom(world.ID, treasureRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload final treasure room: %v", err)
		}

		if finalTreasureRoom.FindItem(moreGold.ID) != nil {
			t.Error("Gold should no longer be in treasure room")
		}

		finalShopRoom, err := manager.Worlds().LoadRoom(world.ID, shopRoom.ID)
		if err != nil {
			t.Fatalf("Failed to reload shop room: %v", err)
		}

		movedItem := finalShopRoom.FindItem(moreGold.ID)
		if movedItem == nil {
			t.Error("Gold should now be in shop room")
		}

		// Verify location was updated
		if movedItem != nil && movedItem.RoomID != shopRoom.ID {
			t.Errorf("Moved item room ID should be %s, got %s", shopRoom.ID, movedItem.RoomID)
		}
	})

	t.Run("Item Persistence Across World Reloads", func(t *testing.T) {
		// Create a new manager instance to simulate application restart
		newManager := storage.NewManagerWithConfig(config)
		if err := newManager.Initialize(); err != nil {
			t.Fatalf("Failed to initialize new manager: %v", err)
		}

		// Verify world and rooms still exist
		if !newManager.Worlds().WorldExists(world.ID) {
			t.Error("World should persist across manager restarts")
		}

		if !newManager.Worlds().RoomExists(world.ID, treasureRoom.ID) {
			t.Error("Treasure room should persist across manager restarts")
		}

		if !newManager.Worlds().RoomExists(world.ID, shopRoom.ID) {
			t.Error("Shop room should persist across manager restarts")
		}

		// Load rooms and verify items are still there
		persistentTreasureRoom, err := newManager.Worlds().LoadRoom(world.ID, treasureRoom.ID)
		if err != nil {
			t.Fatalf("Failed to load persistent treasure room: %v", err)
		}

		persistentShopRoom, err := newManager.Worlds().LoadRoom(world.ID, shopRoom.ID)
		if err != nil {
			t.Fatalf("Failed to load persistent shop room: %v", err)
		}

		// Verify items are still in the correct rooms
		if len(persistentTreasureRoom.Items) == 0 {
			t.Error("Treasure room should still have items after restart")
		}

		if len(persistentShopRoom.Items) == 0 {
			t.Error("Shop room should still have items after restart")
		}

		// Verify item properties are still correct
		for _, item := range persistentTreasureRoom.Items {
			if item.WorldID != world.ID || item.RoomID != treasureRoom.ID {
				t.Error("Item location properties should persist correctly")
			}
		}

		for _, item := range persistentShopRoom.Items {
			if item.WorldID != world.ID || item.RoomID != shopRoom.ID {
				t.Error("Item location properties should persist correctly")
			}
		}
	})
}