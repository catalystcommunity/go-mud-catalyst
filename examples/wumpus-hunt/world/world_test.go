package world

import (
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/storage"
)

func TestSetGetRoomType(t *testing.T) {
	room := storage.NewRoom(ids.NewEntityID(), "test_world", "Test Room")
	
	// Test setting and getting room type
	SetRoomType(room, WumpusRoomTypeInn)
	roomType := GetRoomType(room)
	
	if roomType != WumpusRoomTypeInn {
		t.Errorf("Expected room type %s, got %s", WumpusRoomTypeInn, roomType)
	}
}

func TestSetGetInstanceID(t *testing.T) {
	room := storage.NewRoom(ids.NewEntityID(), "test_world", "Test Room")
	expectedID := "test_instance_123"
	
	// Test setting and getting instance ID
	SetInstanceID(room, expectedID)
	instanceID := GetInstanceID(room)
	
	if instanceID != expectedID {
		t.Errorf("Expected instance ID %s, got %s", expectedID, instanceID)
	}
}

func TestMarkForCleanup(t *testing.T) {
	room := storage.NewRoom(ids.NewEntityID(), "test_world", "Test Room")
	cleanupTime := time.Now().Add(time.Hour)
	
	// Test marking room for cleanup
	MarkForCleanup(room, cleanupTime)
	
	if !IsTemporaryRoom(room) {
		t.Error("Expected room to be marked as temporary")
	}
	
	roomData := GetWumpusRoomData(room)
	if roomData.TempCleanup == "" {
		t.Error("Expected cleanup timestamp to be set")
	}
}

func TestGetWumpusRoomData(t *testing.T) {
	room := storage.NewRoom(ids.NewEntityID(), "test_world", "Test Room")
	
	// Test empty room data
	roomData := GetWumpusRoomData(room)
	if roomData.RoomType != "" {
		t.Errorf("Expected empty room type, got %s", roomData.RoomType)
	}
	
	// Test with data
	SetRoomType(room, WumpusRoomTypePortal)
	SetInstanceID(room, "test_instance")
	
	roomData = GetWumpusRoomData(room)
	if roomData.RoomType != WumpusRoomTypePortal {
		t.Errorf("Expected room type %s, got %s", WumpusRoomTypePortal, roomData.RoomType)
	}
	if roomData.InstanceID != "test_instance" {
		t.Errorf("Expected instance ID test_instance, got %s", roomData.InstanceID)
	}
}

func TestGetWumpusWorldData(t *testing.T) {
	world := storage.NewWorld(ids.NewEntityID(), "Test World")
	
	// Test empty world data
	worldData := GetWumpusWorldData(world)
	if worldData.WorldType != "" {
		t.Errorf("Expected empty world type, got %s", worldData.WorldType)
	}
	
	// Test with data
	testData := WumpusWorldData{
		WorldType:  WumpusWorldTypePermanent,
		InstanceID: "test_instance",
		CreatedBy:  "test_player",
	}
	SetWumpusWorldData(world, testData)
	
	worldData = GetWumpusWorldData(world)
	if worldData.WorldType != WumpusWorldTypePermanent {
		t.Errorf("Expected world type %s, got %s", WumpusWorldTypePermanent, worldData.WorldType)
	}
	if worldData.InstanceID != "test_instance" {
		t.Errorf("Expected instance ID test_instance, got %s", worldData.InstanceID)
	}
	if worldData.CreatedBy != "test_player" {
		t.Errorf("Expected created by test_player, got %s", worldData.CreatedBy)
	}
}

func TestIsWumpusRoom(t *testing.T) {
	room := storage.NewRoom(ids.NewEntityID(), "test_world", "Test Room")
	
	// Test room without wumpus properties
	if IsWumpusRoom(room) {
		t.Error("Expected room to not be a wumpus room")
	}
	
	// Test room with wumpus properties
	SetRoomType(room, WumpusRoomTypeInn)
	if !IsWumpusRoom(room) {
		t.Error("Expected room to be a wumpus room")
	}
}

func TestCreateMainWorld(t *testing.T) {
	// Create a test storage manager
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Test creating main world
	world, err := CreateMainWorld(storageMgr)
	if err != nil {
		t.Fatalf("Failed to create main world: %v", err)
	}
	
	// Verify world properties
	worldData := GetWumpusWorldData(world)
	if worldData.WorldType != WumpusWorldTypePermanent {
		t.Errorf("Expected world type %s, got %s", WumpusWorldTypePermanent, worldData.WorldType)
	}
	
	// Verify world has rooms
	roomIDs, err := storageMgr.Worlds().ListRooms(world.ID)
	if err != nil {
		t.Fatalf("Failed to list rooms: %v", err)
	}
	if len(roomIDs) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(roomIDs))
	}
	
	// Verify rooms exist and have correct types
	innFound := false
	portalFound := false
	
	for _, roomID := range roomIDs {
		room, err := storageMgr.Worlds().LoadRoom(world.ID, roomID)
		if err != nil {
			t.Errorf("Failed to get room %s: %v", roomID, err)
			continue
		}
		
		roomType := GetRoomType(room)
		switch roomType {
		case WumpusRoomTypeInn:
			innFound = true
			// Verify Inn has connection to Portal
			if len(room.Exits) != 1 {
				t.Errorf("Expected Inn to have 1 exit, got %d", len(room.Exits))
			}
			if exit := room.GetExit("north"); exit == nil {
				t.Error("Expected Inn to have north exit")
			}
		case WumpusRoomTypePortal:
			portalFound = true
			// Verify Portal has connection to Inn
			if len(room.Exits) != 1 {
				t.Errorf("Expected Portal to have 1 exit, got %d", len(room.Exits))
			}
			if exit := room.GetExit("south"); exit == nil {
				t.Error("Expected Portal to have south exit")
			}
		}
	}
	
	if !innFound {
		t.Error("Inn room not found")
	}
	if !portalFound {
		t.Error("Portal room not found")
	}
}

func TestValidateWorldStructure(t *testing.T) {
	// Create a test storage manager
	storageConfig := storage.DefaultStorageConfig()
	storageConfig.DataRoot = t.TempDir()
	storageMgr := storage.NewManagerWithConfig(storageConfig)
	
	if err := storageMgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	
	// Create a valid world
	world, err := CreateMainWorld(storageMgr)
	if err != nil {
		t.Fatalf("Failed to create main world: %v", err)
	}
	
	// Test validation of valid world
	if err := ValidateWorldStructure(world, storageMgr); err != nil {
		t.Errorf("Expected valid world to pass validation: %v", err)
	}
	
	// Test validation of invalid world (wrong type)
	invalidWorld := storage.NewWorld(ids.NewEntityID(), "Invalid World")
	SetWumpusWorldData(invalidWorld, WumpusWorldData{
		WorldType: WumpusWorldTypeTemporary,
	})
	
	if err := ValidateWorldStructure(invalidWorld, storageMgr); err == nil {
		t.Error("Expected invalid world to fail validation")
	}
	
	// Test validation of world with no rooms
	emptyWorld := storage.NewWorld(ids.NewEntityID(), "Empty World")
	SetWumpusWorldData(emptyWorld, WumpusWorldData{
		WorldType: WumpusWorldTypePermanent,
	})
	
	if err := ValidateWorldStructure(emptyWorld, storageMgr); err == nil {
		t.Error("Expected world with no rooms to fail validation")
	}
}