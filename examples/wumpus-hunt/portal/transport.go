package portal

import (
	"errors"
	"fmt"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/storage"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/auth"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/instance"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/maze"
	"github.com/catalystcommunity/muddycore/examples/wumpus-hunt/world"
)

// TransportHandler handles portal transportation logic
type TransportHandler struct {
	storageMgr      *storage.Manager
	instanceManager *instance.InstanceManager
}

// TransportResult represents the result of a portal transport operation
type TransportResult struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	TargetWorldID   string `json:"target_world_id,omitempty"`
	TargetRoomID    string `json:"target_room_id,omitempty"`
	InstanceID      string `json:"instance_id,omitempty"`
	IsNewInstance   bool   `json:"is_new_instance,omitempty"`
}

// NewTransportHandler creates a new portal transport handler
func NewTransportHandler(storageMgr *storage.Manager, instanceManager *instance.InstanceManager) *TransportHandler {
	return &TransportHandler{
		storageMgr:      storageMgr,
		instanceManager: instanceManager,
	}
}

// HandlePortalEnter handles when a player enters the portal from the Inn
func (th *TransportHandler) HandlePortalEnter(player *storage.Player) (*TransportResult, error) {
	// Check if player is authenticated
	if !auth.IsPlayerAuthenticated(player) {
		return &TransportResult{
			Success: false,
			Message: "You must be authenticated to enter the portal.",
		}, nil
	}

	// Check if player already has an active instance
	if th.instanceManager.HasPlayerInstance(player.ID) {
		// Player has existing instance, transport them to it
		return th.transportToExistingInstance(player)
	}

	// Create new maze instance for the player
	return th.createNewInstance(player)
}

// HandlePortalExit handles when a player exits back to the Inn
func (th *TransportHandler) HandlePortalExit(player *storage.Player) (*TransportResult, error) {
	// Get the main world
	mainWorld, err := world.GetMainWorld(th.storageMgr)
	if err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error accessing the main world.",
		}, fmt.Errorf("failed to get main world: %w", err)
	}

	// Get the Inn room
	innRoom, err := world.GetInnRoom(th.storageMgr, mainWorld.ID)
	if err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error finding the Inn.",
		}, fmt.Errorf("failed to get Inn room: %w", err)
	}

	// Update player state
	playerState := auth.GetWumpusState(player)
	playerState.IsInMaze = false
	playerState.CurrentInstance = ""
	playerState.Health = 100 // Heal player when returning to Inn
	auth.UpdateWumpusState(player, playerState)

	// Save player state
	if err := th.storageMgr.Players().Save(player); err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error saving player state.",
		}, fmt.Errorf("failed to save player: %w", err)
	}

	return &TransportResult{
		Success:       true,
		Message:       "You step back through the portal and emerge in the warm, welcoming Inn. Your wounds heal as you breathe the safe air.",
		TargetWorldID: mainWorld.ID,
		TargetRoomID:  innRoom.ID,
	}, nil
}

// HandleMazeExit handles when a player exits a maze (either victory or death)
func (th *TransportHandler) HandleMazeExit(player *storage.Player, reason string) (*TransportResult, error) {
	// Get player's current instance
	currentInstance, err := th.instanceManager.GetPlayerInstance(player.ID)
	if err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error finding your current maze instance.",
		}, fmt.Errorf("failed to get player instance: %w", err)
	}

	// Update player statistics based on exit reason
	stats := auth.GetWumpusStats(player)
	playerState := auth.GetWumpusState(player)

	switch reason {
	case "victory":
		stats.GamesWon++
		stats.WumpusPelts++
		playerState.Health = 100
		// TODO: Award wumpus pelt item to player
	case "death":
		stats.TotalDeaths++
		playerState.Health = 100 // Heal on death
	case "quit":
		// No stat changes for voluntary exit
		playerState.Health = 100
	}

	stats.GamesPlayed++
	auth.UpdateWumpusStats(player, stats)

	// Reset player state
	playerState.IsInMaze = false
	playerState.CurrentInstance = ""
	auth.UpdateWumpusState(player, playerState)

	// Save player
	if err := th.storageMgr.Players().Save(player); err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error saving player state.",
		}, fmt.Errorf("failed to save player: %w", err)
	}

	// Clean up the maze instance
	if err := th.instanceManager.CleanupPlayerInstance(player.ID); err != nil {
		// Log error but don't fail the transport
		fmt.Printf("Warning: Failed to cleanup instance %s: %v\n", currentInstance.Config.InstanceID, err)
	}

	// Transport player back to Inn
	return th.HandlePortalExit(player)
}

// transportToExistingInstance transports a player to their existing maze instance
func (th *TransportHandler) transportToExistingInstance(player *storage.Player) (*TransportResult, error) {
	// Get player's existing instance
	mazeInstance, err := th.instanceManager.GetPlayerInstance(player.ID)
	if err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error accessing your maze instance.",
		}, fmt.Errorf("failed to get player instance: %w", err)
	}

	// Validate the instance is still accessible
	if err := th.instanceManager.ValidateInstance(mazeInstance.Config.InstanceID); err != nil {
		// Instance is no longer valid, clean it up and create a new one
		th.instanceManager.CleanupPlayerInstance(player.ID)
		return th.createNewInstance(player)
	}

	// Update player state
	playerState := auth.GetWumpusState(player)
	playerState.IsInMaze = true
	playerState.CurrentInstance = mazeInstance.Config.InstanceID
	auth.UpdateWumpusState(player, playerState)

	// Save player
	if err := th.storageMgr.Players().Save(player); err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error saving player state.",
		}, fmt.Errorf("failed to save player: %w", err)
	}

	return &TransportResult{
		Success:       true,
		Message:       "You step through the portal and find yourself back in the familiar maze. The air is thick with danger, and you can sense the wumpus lurking somewhere in the shadows.",
		TargetWorldID: mazeInstance.World.ID,
		TargetRoomID:  mazeInstance.StartRoom.Room.ID,
		InstanceID:    mazeInstance.Config.InstanceID,
		IsNewInstance: false,
	}, nil
}

// createNewInstance creates a new maze instance for the player
func (th *TransportHandler) createNewInstance(player *storage.Player) (*TransportResult, error) {
	// Create maze configuration
	config := maze.DefaultMazeConfig()
	config.CreatedBy = player.ID
	config.CleanupTime = 2 * time.Hour // Instance expires in 2 hours

	// Create the maze instance
	mazeInstance, err := th.instanceManager.CreateInstance(player.ID, config)
	if err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error creating maze instance. Please try again.",
		}, fmt.Errorf("failed to create maze instance: %w", err)
	}

	// Update player state
	playerState := auth.GetWumpusState(player)
	playerState.IsInMaze = true
	playerState.CurrentInstance = mazeInstance.Config.InstanceID
	auth.UpdateWumpusState(player, playerState)

	// Save player
	if err := th.storageMgr.Players().Save(player); err != nil {
		// Clean up the instance if we can't save player state
		th.instanceManager.CleanupInstance(mazeInstance.Config.InstanceID)
		return &TransportResult{
			Success: false,
			Message: "Error saving player state.",
		}, fmt.Errorf("failed to save player: %w", err)
	}

	return &TransportResult{
		Success:       true,
		Message:       "You step through the portal and emerge in a dark, twisted maze. The air is thick with the scent of danger, and you can hear the distant sounds of the wumpus lurking somewhere in the shadows. Your quest begins now!",
		TargetWorldID: mazeInstance.World.ID,
		TargetRoomID:  mazeInstance.StartRoom.Room.ID,
		InstanceID:    mazeInstance.Config.InstanceID,
		IsNewInstance: true,
	}, nil
}

// GetPlayerCurrentLocation returns the player's current location information
func (th *TransportHandler) GetPlayerCurrentLocation(player *storage.Player) (*PlayerLocationInfo, error) {
	playerState := auth.GetWumpusState(player)
	
	locationInfo := &PlayerLocationInfo{
		PlayerID:        player.ID,
		IsInMaze:        playerState.IsInMaze,
		CurrentInstance: playerState.CurrentInstance,
		Health:          playerState.Health,
	}

	if playerState.IsInMaze && playerState.CurrentInstance != "" {
		// Player is in a maze, get maze information
		mazeInstance, err := th.instanceManager.GetInstance(playerState.CurrentInstance)
		if err != nil {
			// Instance not found, player state is inconsistent
			locationInfo.IsInMaze = false
			locationInfo.CurrentInstance = ""
			locationInfo.Error = "Maze instance not found"
		} else {
			locationInfo.WorldID = mazeInstance.World.ID
			locationInfo.WorldName = mazeInstance.World.Name
			locationInfo.InstanceID = mazeInstance.Config.InstanceID
		}
	} else {
		// Player is in the main world (Inn or Portal)
		mainWorld, err := world.GetMainWorld(th.storageMgr)
		if err != nil {
			locationInfo.Error = "Cannot access main world"
		} else {
			locationInfo.WorldID = mainWorld.ID
			locationInfo.WorldName = mainWorld.Name
		}
	}

	return locationInfo, nil
}

// PlayerLocationInfo represents a player's current location information
type PlayerLocationInfo struct {
	PlayerID        string `json:"player_id"`
	IsInMaze        bool   `json:"is_in_maze"`
	CurrentInstance string `json:"current_instance,omitempty"`
	Health          int    `json:"health"`
	WorldID         string `json:"world_id,omitempty"`
	WorldName       string `json:"world_name,omitempty"`
	InstanceID      string `json:"instance_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

// CanPlayerEnterPortal checks if a player can enter the portal
func (th *TransportHandler) CanPlayerEnterPortal(player *storage.Player) (bool, string) {
	// Check authentication
	if !auth.IsPlayerAuthenticated(player) {
		return false, "You must be authenticated to enter the portal."
	}

	// Check player health
	playerState := auth.GetWumpusState(player)
	if playerState.Health <= 0 {
		return false, "You are too weak to enter the portal. Rest in the Inn to recover."
	}

	return true, ""
}

// ExtendInstanceLifetime extends the lifetime of a player's current instance
func (th *TransportHandler) ExtendInstanceLifetime(player *storage.Player, duration time.Duration) error {
	if !th.instanceManager.HasPlayerInstance(player.ID) {
		return errors.New("player has no active instance")
	}

	playerState := auth.GetWumpusState(player)
	if playerState.CurrentInstance == "" {
		return errors.New("player has no current instance")
	}

	return th.instanceManager.ExtendInstanceLifetime(playerState.CurrentInstance, duration)
}

// GetInstanceTimeRemaining returns the time remaining for a player's current instance
func (th *TransportHandler) GetInstanceTimeRemaining(player *storage.Player) (time.Duration, error) {
	if !th.instanceManager.HasPlayerInstance(player.ID) {
		return 0, errors.New("player has no active instance")
	}

	playerState := auth.GetWumpusState(player)
	if playerState.CurrentInstance == "" {
		return 0, errors.New("player has no current instance")
	}

	instanceInfo, err := th.instanceManager.GetInstanceInfo(playerState.CurrentInstance)
	if err != nil {
		return 0, fmt.Errorf("failed to get instance info: %w", err)
	}

	remaining := time.Until(instanceInfo.ExpiresAt)
	if remaining < 0 {
		return 0, nil
	}

	return remaining, nil
}

// HandlePortalCommand processes portal-related commands
func (th *TransportHandler) HandlePortalCommand(player *storage.Player, command string) (*TransportResult, error) {
	switch command {
	case "enter":
		return th.HandlePortalEnter(player)
	case "exit":
		return th.HandlePortalExit(player)
	case "status":
		return th.getPortalStatus(player)
	default:
		return &TransportResult{
			Success: false,
			Message: "Unknown portal command. Available commands: enter, exit, status",
		}, nil
	}
}

// getPortalStatus returns the current portal status for a player
func (th *TransportHandler) getPortalStatus(player *storage.Player) (*TransportResult, error) {
	locationInfo, err := th.GetPlayerCurrentLocation(player)
	if err != nil {
		return &TransportResult{
			Success: false,
			Message: "Error getting location information.",
		}, fmt.Errorf("failed to get location info: %w", err)
	}

	var message string
	if locationInfo.IsInMaze {
		timeRemaining, err := th.GetInstanceTimeRemaining(player)
		if err != nil {
			message = fmt.Sprintf("You are currently in maze instance %s. Time remaining: Unknown", locationInfo.CurrentInstance)
		} else {
			message = fmt.Sprintf("You are currently in maze instance %s. Time remaining: %v", locationInfo.CurrentInstance, timeRemaining.Round(time.Minute))
		}
	} else {
		if th.instanceManager.HasPlayerInstance(player.ID) {
			message = "You are in the main world, but you have a maze instance waiting. Use 'portal enter' to return to your maze."
		} else {
			message = "You are in the main world. Use 'portal enter' to create a new maze instance."
		}
	}

	return &TransportResult{
		Success: true,
		Message: message,
	}, nil
}