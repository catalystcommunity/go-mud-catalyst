# Wumpus Hunt - Code Examples and Patterns

This document provides practical examples of implementing game features using MuddyCore's Properties extension system.

## Table of Contents

1. [Properties Extension Patterns](#properties-extension-patterns)
2. [Authentication Examples](#authentication-examples)
3. [World and Room Extensions](#world-and-room-extensions)
4. [Item System Examples](#item-system-examples)
5. [Game State Management](#game-state-management)
6. [Lua Scripting Integration](#lua-scripting-integration)
7. [Error Handling Patterns](#error-handling-patterns)
8. [Performance Optimization](#performance-optimization)

## Properties Extension Patterns

### Basic Property Access Pattern

The foundation of extending MuddyCore without modification is the Properties extension pattern. Here's how to implement it safely:

```go
package extensions

import (
    "errors"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

// Define your data structures
type GameData struct {
    Level    int    `json:"level"`
    Score    int    `json:"score"`
    Status   string `json:"status"`
    LastSeen int64  `json:"last_seen"`
}

// Safe getter with validation
func GetGameData(player *storage.Player) (*GameData, error) {
    // Get the property
    data, exists := player.GetProperty("game_data")
    if !exists {
        // Return default values for new players
        return &GameData{
            Level:  1,
            Score:  0,
            Status: "active",
        }, nil
    }
    
    // Type assertion with validation
    dataMap, ok := data.(map[string]interface{})
    if !ok {
        return nil, errors.New("corrupted game data format")
    }
    
    // Extract values with defaults
    gameData := &GameData{
        Level:    getIntOrDefault(dataMap, "level", 1),
        Score:    getIntOrDefault(dataMap, "score", 0),
        Status:   getStringOrDefault(dataMap, "status", "active"),
        LastSeen: getInt64OrDefault(dataMap, "last_seen", 0),
    }
    
    return gameData, nil
}

// Safe setter with validation
func SetGameData(player *storage.Player, data *GameData) error {
    if data == nil {
        return errors.New("game data cannot be nil")
    }
    
    // Validate data before storing
    if data.Level < 1 {
        return errors.New("level must be at least 1")
    }
    
    if data.Score < 0 {
        return errors.New("score cannot be negative")
    }
    
    // Convert to map for storage
    dataMap := map[string]interface{}{
        "level":     data.Level,
        "score":     data.Score,
        "status":    data.Status,
        "last_seen": data.LastSeen,
    }
    
    // Store the property
    return player.SetProperty("game_data", dataMap)
}

// Helper functions for safe type conversion
func getIntOrDefault(m map[string]interface{}, key string, defaultVal int) int {
    if val, ok := m[key]; ok {
        if intVal, ok := val.(int); ok {
            return intVal
        }
        if floatVal, ok := val.(float64); ok {
            return int(floatVal)
        }
    }
    return defaultVal
}

func getStringOrDefault(m map[string]interface{}, key string, defaultVal string) string {
    if val, ok := m[key]; ok {
        if strVal, ok := val.(string); ok {
            return strVal
        }
    }
    return defaultVal
}

func getInt64OrDefault(m map[string]interface{}, key string, defaultVal int64) int64 {
    if val, ok := m[key]; ok {
        if intVal, ok := val.(int64); ok {
            return intVal
        }
        if floatVal, ok := val.(float64); ok {
            return int64(floatVal)
        }
    }
    return defaultVal
}
```

### Property Namespace Pattern

Use consistent naming conventions to avoid conflicts:

```go
// Good: Game-specific prefixes
const (
    WumpusAuthProperty    = "wumpus_auth"
    WumpusStatsProperty   = "wumpus_stats"
    WumpusStateProperty   = "wumpus_state"
    ChessGameProperty     = "chess_game"
    ChessStatsProperty    = "chess_stats"
)

// Bad: Generic names that could conflict
const (
    AuthProperty   = "auth"      // Too generic
    StatsProperty  = "stats"     // Could conflict
    StateProperty  = "state"     // Very generic
)
```

## Authentication Examples

### Complete Authentication System

```go
package auth

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "errors"
    "time"
    
    "golang.org/x/crypto/bcrypt"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

type WumpusAuth struct {
    PasswordHash string    `json:"password_hash"`
    Salt         string    `json:"salt"`
    AuthMethod   string    `json:"auth_method"`
    CreatedAt    time.Time `json:"created_at"`
    LastLogin    time.Time `json:"last_login"`
    LoginCount   int       `json:"login_count"`
}

// Create new player account
func CreatePlayerAccount(player *storage.Player, username, password string) error {
    // Validate input
    if len(username) < 3 {
        return errors.New("username must be at least 3 characters")
    }
    if len(password) < 6 {
        return errors.New("password must be at least 6 characters")
    }
    
    // Check if player already has auth data
    if _, err := GetWumpusAuth(player); err == nil {
        return errors.New("player already has an account")
    }
    
    // Generate salt and hash password
    salt, err := generateSalt()
    if err != nil {
        return err
    }
    
    passwordHash, err := bcrypt.GenerateFromPassword(
        []byte(password+salt), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    
    // Create auth data
    auth := &WumpusAuth{
        PasswordHash: string(passwordHash),
        Salt:         salt,
        AuthMethod:   "password",
        CreatedAt:    time.Now(),
        LoginCount:   0,
    }
    
    // Store in player properties
    return SetWumpusAuth(player, auth)
}

// Authenticate player login
func AuthenticatePlayer(player *storage.Player, password string) error {
    auth, err := GetWumpusAuth(player)
    if err != nil {
        return errors.New("no account found for player")
    }
    
    // Verify password
    err = bcrypt.CompareHashAndPassword(
        []byte(auth.PasswordHash), 
        []byte(password+auth.Salt))
    if err != nil {
        return errors.New("invalid password")
    }
    
    // Update login statistics
    auth.LastLogin = time.Now()
    auth.LoginCount++
    
    return SetWumpusAuth(player, auth)
}

// Change player password
func ChangePlayerPassword(player *storage.Player, oldPassword, newPassword string) error {
    // Authenticate with old password first
    if err := AuthenticatePlayer(player, oldPassword); err != nil {
        return err
    }
    
    // Validate new password
    if len(newPassword) < 6 {
        return errors.New("new password must be at least 6 characters")
    }
    
    auth, err := GetWumpusAuth(player)
    if err != nil {
        return err
    }
    
    // Generate new salt and hash
    newSalt, err := generateSalt()
    if err != nil {
        return err
    }
    
    newPasswordHash, err := bcrypt.GenerateFromPassword(
        []byte(newPassword+newSalt), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    
    // Update auth data
    auth.PasswordHash = string(newPasswordHash)
    auth.Salt = newSalt
    
    return SetWumpusAuth(player, auth)
}

func generateSalt() (string, error) {
    bytes := make([]byte, 32)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(bytes), nil
}
```

### Session Management

```go
package auth

import (
    "time"
    "sync"
)

type SessionManager struct {
    sessions map[string]*Session
    mutex    sync.RWMutex
    timeout  time.Duration
}

type Session struct {
    PlayerID   string
    ClientID   string
    CreatedAt  time.Time
    LastAccess time.Time
    Data       map[string]interface{}
}

func NewSessionManager(timeout time.Duration) *SessionManager {
    sm := &SessionManager{
        sessions: make(map[string]*Session),
        timeout:  timeout,
    }
    
    // Start cleanup goroutine
    go sm.cleanupExpiredSessions()
    
    return sm
}

func (sm *SessionManager) CreateSession(playerID, clientID string) *Session {
    sm.mutex.Lock()
    defer sm.mutex.Unlock()
    
    session := &Session{
        PlayerID:   playerID,
        ClientID:   clientID,
        CreatedAt:  time.Now(),
        LastAccess: time.Now(),
        Data:       make(map[string]interface{}),
    }
    
    sm.sessions[clientID] = session
    return session
}

func (sm *SessionManager) GetSession(clientID string) (*Session, bool) {
    sm.mutex.RLock()
    defer sm.mutex.RUnlock()
    
    session, exists := sm.sessions[clientID]
    if exists {
        session.LastAccess = time.Now()
    }
    
    return session, exists
}

func (sm *SessionManager) cleanupExpiredSessions() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        sm.mutex.Lock()
        now := time.Now()
        for clientID, session := range sm.sessions {
            if now.Sub(session.LastAccess) > sm.timeout {
                delete(sm.sessions, clientID)
            }
        }
        sm.mutex.Unlock()
    }
}
```

## World and Room Extensions

### Room Type Management

```go
package world

import (
    "errors"
    "time"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

type RoomType string

const (
    RoomTypeInn    RoomType = "inn"
    RoomTypePortal RoomType = "portal"
    RoomTypeMaze   RoomType = "maze"
    RoomTypeArena  RoomType = "arena"
)

type WumpusRoomData struct {
    RoomType     RoomType  `json:"room_type"`
    InstanceID   string    `json:"instance_id"`
    HasWumpus    bool      `json:"has_wumpus"`
    HasPit       bool      `json:"has_pit"`
    HasBreeze    bool      `json:"has_breeze"`
    HasStench    bool      `json:"has_stench"`
    Position     Position  `json:"position"`
    CleanupAt    time.Time `json:"cleanup_at"`
    Description  string    `json:"description"`
    Exits        []string  `json:"exits"`
}

type Position struct {
    X int `json:"x"`
    Y int `json:"y"`
}

// Set room as specific type with data
func SetupRoom(room *storage.Room, roomType RoomType, instanceID string) error {
    roomData := &WumpusRoomData{
        RoomType:   roomType,
        InstanceID: instanceID,
        Position:   Position{X: 0, Y: 0},
        Exits:      make([]string, 0),
    }
    
    // Set room-specific defaults
    switch roomType {
    case RoomTypeInn:
        roomData.Description = "A cozy inn where adventurers gather to rest and share tales."
        room.Name = "The Adventurer's Inn"
        room.Description = roomData.Description
        
    case RoomTypePortal:
        roomData.Description = "A mystical portal chamber crackling with arcane energy."
        room.Name = "The Portal Chamber"
        room.Description = roomData.Description
        
    case RoomTypeMaze:
        roomData.Description = "A dark, twisting passage carved from ancient stone."
        room.Name = "Maze Passage"
        room.Description = roomData.Description
        
    case RoomTypeArena:
        roomData.Description = "A grand arena where epic battles are fought."
        room.Name = "Combat Arena"
        room.Description = roomData.Description
    }
    
    return SetWumpusRoomData(room, roomData)
}

// Add hazards to maze room
func AddHazardsToRoom(room *storage.Room, hasWumpus, hasPit bool) error {
    roomData, err := GetWumpusRoomData(room)
    if err != nil {
        return err
    }
    
    roomData.HasWumpus = hasWumpus
    roomData.HasPit = hasPit
    
    // Set atmospheric effects
    if hasWumpus {
        roomData.HasStench = true
        roomData.Description += " A terrible stench fills the air."
    }
    
    if hasPit {
        roomData.HasBreeze = true
        roomData.Description += " You feel a cool breeze from somewhere below."
    }
    
    // Update room description
    room.Description = roomData.Description
    
    return SetWumpusRoomData(room, roomData)
}

// Connect rooms with exits
func ConnectRooms(room1, room2 *storage.Room, direction1, direction2 string) error {
    // Add exit from room1 to room2
    if err := AddExit(room1, direction1, room2.ID); err != nil {
        return err
    }
    
    // Add exit from room2 to room1  
    if err := AddExit(room2, direction2, room1.ID); err != nil {
        return err
    }
    
    return nil
}

func AddExit(room *storage.Room, direction, targetRoomID string) error {
    roomData, err := GetWumpusRoomData(room)
    if err != nil {
        return err
    }
    
    // Add to room's exits list
    roomData.Exits = append(roomData.Exits, direction+":"+targetRoomID)
    
    // Also add to MuddyCore's connection system
    room.Connections = append(room.Connections, storage.Connection{
        Direction: direction,
        RoomID:    targetRoomID,
    })
    
    return SetWumpusRoomData(room, roomData)
}
```

### Instance Management

```go
package instance

import (
    "fmt"
    "time"
    "sync"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

type InstanceManager struct {
    instances map[string]*GameInstance
    mutex     sync.RWMutex
    cleanup   chan string
    storage   *storage.StorageManager
}

type GameInstance struct {
    ID          string
    WorldID     string
    CreatedBy   string
    CreatedAt   time.Time
    CleanupAt   time.Time
    PlayerCount int
    Rooms       []string
    Status      InstanceStatus
}

type InstanceStatus string

const (
    StatusActive    InstanceStatus = "active"
    StatusCompleted InstanceStatus = "completed"
    StatusExpired   InstanceStatus = "expired"
)

func NewInstanceManager(storage *storage.StorageManager) *InstanceManager {
    im := &InstanceManager{
        instances: make(map[string]*GameInstance),
        cleanup:   make(chan string, 100),
        storage:   storage,
    }
    
    // Start cleanup worker
    go im.cleanupWorker()
    
    return im
}

// Create new game instance
func (im *InstanceManager) CreateInstance(playerID string, config InstanceConfig) (*GameInstance, error) {
    im.mutex.Lock()
    defer im.mutex.Unlock()
    
    instanceID := generateInstanceID()
    
    instance := &GameInstance{
        ID:        instanceID,
        CreatedBy: playerID,
        CreatedAt: time.Now(),
        CleanupAt: time.Now().Add(config.MaxDuration),
        Status:    StatusActive,
        Rooms:     make([]string, 0),
    }
    
    // Create world for this instance
    world, err := im.createInstanceWorld(instance, config)
    if err != nil {
        return nil, err
    }
    
    instance.WorldID = world.ID
    im.instances[instanceID] = instance
    
    return instance, nil
}

// Add player to instance
func (im *InstanceManager) JoinInstance(instanceID, playerID string) error {
    im.mutex.Lock()
    defer im.mutex.Unlock()
    
    instance, exists := im.instances[instanceID]
    if !exists {
        return errors.New("instance not found")
    }
    
    if instance.Status != StatusActive {
        return errors.New("instance is not active")
    }
    
    instance.PlayerCount++
    return nil
}

// Remove player from instance
func (im *InstanceManager) LeaveInstance(instanceID, playerID string) error {
    im.mutex.Lock()
    defer im.mutex.Unlock()
    
    instance, exists := im.instances[instanceID]
    if !exists {
        return errors.New("instance not found")
    }
    
    instance.PlayerCount--
    
    // Schedule cleanup if no players remain
    if instance.PlayerCount <= 0 {
        instance.Status = StatusCompleted
        im.cleanup <- instanceID
    }
    
    return nil
}

// Background cleanup worker
func (im *InstanceManager) cleanupWorker() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case instanceID := <-im.cleanup:
            im.cleanupInstance(instanceID)
            
        case <-ticker.C:
            im.cleanupExpiredInstances()
        }
    }
}

func (im *InstanceManager) cleanupInstance(instanceID string) {
    im.mutex.Lock()
    defer im.mutex.Unlock()
    
    instance, exists := im.instances[instanceID]
    if !exists {
        return
    }
    
    // Remove all rooms in this instance
    for _, roomID := range instance.Rooms {
        if err := im.storage.DeleteRoom(roomID); err != nil {
            log.Errorf("Failed to delete room %s: %v", roomID, err)
        }
    }
    
    // Remove world
    if err := im.storage.DeleteWorld(instance.WorldID); err != nil {
        log.Errorf("Failed to delete world %s: %v", instance.WorldID, err)
    }
    
    delete(im.instances, instanceID)
    
    log.Infof("Cleaned up instance %s", instanceID)
}
```

## Item System Examples

### Wumpus Pelt Trophy System

```go
package items

import (
    "time"
    "fmt"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

type WumpusItem struct {
    ItemType          string    `json:"item_type"`
    Rarity           string    `json:"rarity"`
    VictoryTimestamp time.Time `json:"victory_timestamp"`
    DefeatedInstance string    `json:"defeated_instance"`
    PlayerName       string    `json:"player_name"`
    WumpusLevel      int       `json:"wumpus_level"`
}

// Create wumpus pelt trophy
func CreateWumpusPelt(playerID, instanceID string, wumpusLevel int) (*storage.ItemInstance, error) {
    // Create base item if it doesn't exist
    peltItem, err := getOrCreateWumpusPeltItem()
    if err != nil {
        return nil, err
    }
    
    // Create item instance
    instance := &storage.ItemInstance{
        ID:       generateItemInstanceID(),
        ItemID:   peltItem.ID,
        OwnerID:  playerID,
        Quantity: 1,
        Durability: 100, // Trophy items don't degrade
        Properties: make(map[string]interface{}),
    }
    
    // Add wumpus-specific data
    wumpusData := &WumpusItem{
        ItemType:          "trophy",
        Rarity:           getRarityForLevel(wumpusLevel),
        VictoryTimestamp: time.Now(),
        DefeatedInstance: instanceID,
        WumpusLevel:      wumpusLevel,
    }
    
    return instance, SetWumpusItemData(instance, wumpusData)
}

// Get or create the base wumpus pelt item
func getOrCreateWumpusPeltItem() (*storage.Item, error) {
    // Try to find existing item
    items, err := storage.GetItemsByName("Wumpus Pelt")
    if err == nil && len(items) > 0 {
        return &items[0], nil
    }
    
    // Create new base item
    item := &storage.Item{
        ID:          "wumpus_pelt",
        Name:        "Wumpus Pelt",
        Description: "A fearsome trophy from a defeated wumpus",
        Category:    "trophy",
        Weight:      5.0,
        Value:       1000,
        Stackable:   false,
        Properties:  make(map[string]interface{}),
    }
    
    return item, storage.SaveItem(item)
}

// Create inventory for new player
func CreatePlayerInventory(playerID string) (*storage.Inventory, error) {
    inventory := &storage.Inventory{
        ID:       playerID + "_inventory",
        OwnerID:  playerID,
        MaxSlots: 20,
        Items:    make([]*storage.ItemInstance, 0),
        Properties: make(map[string]interface{}),
    }
    
    // Add starting items
    startingItems := []struct {
        ItemID   string
        Quantity int
    }{
        {"torch", 3},
        {"arrow", 5},
        {"healing_potion", 2},
    }
    
    for _, startingItem := range startingItems {
        item, err := createStartingItem(startingItem.ItemID, playerID, startingItem.Quantity)
        if err != nil {
            return nil, err
        }
        inventory.Items = append(inventory.Items, item)
    }
    
    return inventory, storage.SaveInventory(inventory)
}

// Award wumpus pelt to player
func AwardWumpusPelt(playerID, instanceID string, wumpusLevel int) error {
    // Get player inventory
    inventory, err := storage.GetInventoryByOwner(playerID)
    if err != nil {
        return err
    }
    
    // Create wumpus pelt
    pelt, err := CreateWumpusPelt(playerID, instanceID, wumpusLevel)
    if err != nil {
        return err
    }
    
    // Add to inventory
    return AddItemToInventory(inventory, pelt)
}

func AddItemToInventory(inventory *storage.Inventory, item *storage.ItemInstance) error {
    // Check if inventory has space
    if len(inventory.Items) >= inventory.MaxSlots {
        return errors.New("inventory is full")
    }
    
    // For stackable items, try to stack with existing
    if isStackable(item) {
        for _, existingItem := range inventory.Items {
            if existingItem.ItemID == item.ItemID && canStack(existingItem, item) {
                existingItem.Quantity += item.Quantity
                return storage.SaveInventory(inventory)
            }
        }
    }
    
    // Add as new item
    inventory.Items = append(inventory.Items, item)
    return storage.SaveInventory(inventory)
}

func getRarityForLevel(level int) string {
    switch {
    case level >= 10:
        return "legendary"
    case level >= 7:
        return "epic"
    case level >= 4:
        return "rare"
    default:
        return "common"
    }
}
```

## Game State Management

### Player State Machine

```go
package game

import (
    "errors"
    "time"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

type PlayerState string

const (
    StateInn        PlayerState = "inn"
    StatePortal     PlayerState = "portal" 
    StateMaze       PlayerState = "maze"
    StateCombat     PlayerState = "combat"
    StateDead       PlayerState = "dead"
    StateVictorious PlayerState = "victorious"
)

type StateManager struct {
    transitions map[PlayerState][]PlayerState
    handlers    map[PlayerState]StateHandler
}

type StateHandler func(*storage.Player, PlayerState, PlayerState) error

func NewStateManager() *StateManager {
    sm := &StateManager{
        transitions: make(map[PlayerState][]PlayerState),
        handlers:    make(map[PlayerState]StateHandler),
    }
    
    // Define valid state transitions
    sm.transitions[StateInn] = []PlayerState{StatePortal}
    sm.transitions[StatePortal] = []PlayerState{StateInn, StateMaze}
    sm.transitions[StateMaze] = []PlayerState{StateCombat, StateInn, StateDead, StateVictorious}
    sm.transitions[StateCombat] = []PlayerState{StateMaze, StateDead, StateVictorious}
    sm.transitions[StateDead] = []PlayerState{StateInn}
    sm.transitions[StateVictorious] = []PlayerState{StateInn}
    
    // Register state handlers
    sm.handlers[StateInn] = sm.handleInnState
    sm.handlers[StatePortal] = sm.handlePortalState
    sm.handlers[StateMaze] = sm.handleMazeState
    sm.handlers[StateCombat] = sm.handleCombatState
    sm.handlers[StateDead] = sm.handleDeathState
    sm.handlers[StateVictorious] = sm.handleVictoryState
    
    return sm
}

// Change player state with validation
func (sm *StateManager) ChangeState(player *storage.Player, newState PlayerState) error {
    currentState, err := GetPlayerState(player)
    if err != nil {
        return err
    }
    
    // Validate transition
    if !sm.isValidTransition(currentState, newState) {
        return fmt.Errorf("invalid state transition from %s to %s", currentState, newState)
    }
    
    // Execute state handler
    if handler, exists := sm.handlers[newState]; exists {
        if err := handler(player, currentState, newState); err != nil {
            return err
        }
    }
    
    // Update player state
    return SetPlayerState(player, newState)
}

func (sm *StateManager) isValidTransition(from, to PlayerState) bool {
    validTransitions, exists := sm.transitions[from]
    if !exists {
        return false
    }
    
    for _, validTo := range validTransitions {
        if validTo == to {
            return true
        }
    }
    
    return false
}

// State Handlers
func (sm *StateManager) handleInnState(player *storage.Player, from, to PlayerState) error {
    // Heal player when entering inn
    playerState, err := GetWumpusState(player)
    if err != nil {
        return err
    }
    
    playerState.Health = 100
    playerState.IsInMaze = false
    playerState.CurrentInstance = ""
    
    return SetWumpusState(player, playerState)
}

func (sm *StateManager) handleMazeState(player *storage.Player, from, to PlayerState) error {
    playerState, err := GetWumpusState(player)
    if err != nil {
        return err
    }
    
    playerState.IsInMaze = true
    return SetWumpusState(player, playerState)
}

func (sm *StateManager) handleCombatState(player *storage.Player, from, to PlayerState) error {
    // Combat state doesn't change health/instance, just state
    return nil
}

func (sm *StateManager) handleDeathState(player *storage.Player, from, to PlayerState) error {
    // Update death statistics
    stats, err := GetWumpusStats(player)
    if err != nil {
        return err
    }
    
    stats.TotalDeaths++
    return SetWumpusStats(player, stats)
}

func (sm *StateManager) handleVictoryState(player *storage.Player, from, to PlayerState) error {
    // Update victory statistics
    stats, err := GetWumpusStats(player)
    if err != nil {
        return err
    }
    
    stats.GamesWon++
    stats.WumpusPelts++
    
    return SetWumpusStats(player, stats)
}
```

## Lua Scripting Integration

### Wumpus AI Script Template

```lua
-- wumpus_ai.lua - Template for Wumpus AI behavior

-- Global state for this Wumpus instance
local wumpus_state = {
    current_room = "",
    last_move_time = 0,
    aggression_level = 1,
    player_encounters = 0,
    move_pattern = "random"
}

-- Movement patterns
local PATTERNS = {
    random = "move_randomly",
    hunting = "hunt_player", 
    patrolling = "patrol_territory",
    fleeing = "flee_from_player"
}

-- Initialize Wumpus AI
function initialize(initial_room, config)
    wumpus_state.current_room = initial_room
    wumpus_state.aggression_level = config.aggression or 1
    wumpus_state.move_pattern = config.pattern or "random"
    
    log_info("Wumpus AI initialized in room: " .. initial_room)
    return true
end

-- Handle player movement events
function on_player_enter_room(player_id, room_id)
    local distance = get_room_distance(wumpus_state.current_room, room_id)
    
    if distance == 0 then
        -- Player entered wumpus room - combat!
        initiate_combat(player_id)
    elseif distance == 1 then
        -- Player is adjacent - increase aggression
        wumpus_state.aggression_level = wumpus_state.aggression_level + 1
        wumpus_state.player_encounters = wumpus_state.player_encounters + 1
        
        -- Maybe move toward or away from player
        if should_move_toward_player() then
            wumpus_state.move_pattern = "hunting"
        else
            wumpus_state.move_pattern = "fleeing"
        end
        
        log_debug("Player detected nearby, aggression now: " .. wumpus_state.aggression_level)
    end
end

-- Periodic movement decision
function on_movement_tick()
    local current_time = get_current_time()
    
    -- Don't move too frequently
    if current_time - wumpus_state.last_move_time < 30 then
        return
    end
    
    local pattern_func = PATTERNS[wumpus_state.move_pattern]
    if pattern_func and _G[pattern_func] then
        _G[pattern_func]()
    else
        move_randomly()
    end
    
    wumpus_state.last_move_time = current_time
end

-- Movement pattern implementations
function move_randomly()
    local adjacent_rooms = get_adjacent_rooms(wumpus_state.current_room)
    if #adjacent_rooms > 0 then
        local target_room = adjacent_rooms[math.random(#adjacent_rooms)]
        move_wumpus_to_room(target_room)
        log_debug("Wumpus moved randomly to: " .. target_room)
    end
end

function hunt_player()
    local players_in_maze = get_players_in_instance()
    if #players_in_maze == 0 then
        move_randomly()
        return
    end
    
    -- Find closest player
    local closest_player = nil
    local closest_distance = 999
    
    for _, player_id in ipairs(players_in_maze) do
        local player_room = get_player_room(player_id)
        local distance = get_room_distance(wumpus_state.current_room, player_room)
        
        if distance < closest_distance then
            closest_distance = distance
            closest_player = player_id
        end
    end
    
    if closest_player then
        local player_room = get_player_room(closest_player)
        local path = find_path_to_room(wumpus_state.current_room, player_room)
        
        if #path > 1 then
            move_wumpus_to_room(path[2]) -- Move one step toward player
            log_debug("Wumpus hunting player, moved to: " .. path[2])
        end
    end
end

function flee_from_player()
    local players_in_maze = get_players_in_instance()
    if #players_in_maze == 0 then
        move_randomly()
        return
    end
    
    -- Find room furthest from all players
    local adjacent_rooms = get_adjacent_rooms(wumpus_state.current_room)
    local best_room = nil
    local best_total_distance = 0
    
    for _, room_id in ipairs(adjacent_rooms) do
        local total_distance = 0
        
        for _, player_id in ipairs(players_in_maze) do
            local player_room = get_player_room(player_id)
            total_distance = total_distance + get_room_distance(room_id, player_room)
        end
        
        if total_distance > best_total_distance then
            best_total_distance = total_distance
            best_room = room_id
        end
    end
    
    if best_room then
        move_wumpus_to_room(best_room)
        log_debug("Wumpus fled to: " .. best_room)
    end
end

-- Combat initiation
function initiate_combat(player_id)
    wumpus_state.player_encounters = wumpus_state.player_encounters + 1
    
    local combat_data = {
        wumpus_room = wumpus_state.current_room,
        wumpus_level = wumpus_state.aggression_level,
        encounter_count = wumpus_state.player_encounters
    }
    
    trigger_combat_event(player_id, combat_data)
    log_info("Combat initiated with player: " .. player_id)
end

-- Helper functions
function should_move_toward_player()
    -- More aggressive wumpus more likely to hunt
    local hunt_chance = wumpus_state.aggression_level * 0.3
    return math.random() < hunt_chance
end

function get_current_time()
    -- This would be provided by the Go runtime
    return os.time()
end

-- Cleanup function called when instance is destroyed
function cleanup()
    log_info("Wumpus AI shutting down")
    wumpus_state = nil
end
```

### Lua-Go Integration Example

```go
package scripting

import (
    "context"
    "fmt"
    "time"
    
    lua "github.com/yuin/gopher-lua"
    "github.com/catalystcommunity/muddycore/pkg/scripting"
)

type WumpusAI struct {
    instanceID string
    scriptPath string
    luaVM      *scripting.UserVM
    context    context.Context
    cancel     context.CancelFunc
}

func NewWumpusAI(instanceID, scriptPath string) (*WumpusAI, error) {
    ctx, cancel := context.WithCancel(context.Background())
    
    ai := &WumpusAI{
        instanceID: instanceID,
        scriptPath: scriptPath,
        context:    ctx,
        cancel:     cancel,
    }
    
    // Create Lua VM
    vm, err := scripting.NewUserVM(fmt.Sprintf("wumpus_%s", instanceID), "wumpus_hunt_server")
    if err != nil {
        cancel()
        return nil, err
    }
    
    ai.luaVM = vm
    
    // Register Go functions for Lua
    ai.registerLuaFunctions()
    
    // Load and execute the script
    if err := ai.luaVM.ExecuteFile(scriptPath); err != nil {
        cancel()
        return nil, err
    }
    
    return ai, nil
}

func (ai *WumpusAI) registerLuaFunctions() {
    // Room and movement functions
    ai.luaVM.RegisterFunction("get_adjacent_rooms", ai.luaGetAdjacentRooms)
    ai.luaVM.RegisterFunction("get_room_distance", ai.luaGetRoomDistance)
    ai.luaVM.RegisterFunction("move_wumpus_to_room", ai.luaMoveWumpusToRoom)
    ai.luaVM.RegisterFunction("find_path_to_room", ai.luaFindPathToRoom)
    
    // Player functions
    ai.luaVM.RegisterFunction("get_players_in_instance", ai.luaGetPlayersInInstance)
    ai.luaVM.RegisterFunction("get_player_room", ai.luaGetPlayerRoom)
    
    // Combat functions
    ai.luaVM.RegisterFunction("trigger_combat_event", ai.luaTriggerCombatEvent)
    
    // Utility functions
    ai.luaVM.RegisterFunction("log_info", ai.luaLogInfo)
    ai.luaVM.RegisterFunction("log_debug", ai.luaLogDebug)
    ai.luaVM.RegisterFunction("get_current_time", ai.luaGetCurrentTime)
}

// Lua function implementations
func (ai *WumpusAI) luaGetAdjacentRooms(L *lua.LState) int {
    roomID := L.ToString(1)
    
    // Get adjacent rooms from the maze
    adjacentRooms, err := ai.getAdjacentRooms(roomID)
    if err != nil {
        L.Push(lua.LNil)
        return 1
    }
    
    // Convert to Lua table
    table := L.NewTable()
    for i, room := range adjacentRooms {
        table.RawSetInt(i+1, lua.LString(room))
    }
    
    L.Push(table)
    return 1
}

func (ai *WumpusAI) luaMoveWumpusToRoom(L *lua.LState) int {
    targetRoom := L.ToString(1)
    
    err := ai.moveWumpusToRoom(targetRoom)
    L.Push(lua.LBool(err == nil))
    return 1
}

func (ai *WumpusAI) luaTriggerCombatEvent(L *lua.LState) int {
    playerID := L.ToString(1)
    
    // Get combat data from Lua table
    combatDataTable := L.ToTable(2)
    combatData := make(map[string]interface{})
    
    combatDataTable.ForEach(func(key, value lua.LValue) {
        if keyStr, ok := key.(lua.LString); ok {
            combatData[string(keyStr)] = value.String()
        }
    })
    
    err := ai.triggerCombatEvent(playerID, combatData)
    L.Push(lua.LBool(err == nil))
    return 1
}

// Start AI processing
func (ai *WumpusAI) Start() error {
    // Initialize the AI script
    err := ai.luaVM.CallFunction("initialize", ai.instanceID, map[string]interface{}{
        "aggression": 1,
        "pattern":    "random",
    })
    if err != nil {
        return err
    }
    
    // Start movement tick goroutine
    go ai.movementTicker()
    
    return nil
}

func (ai *WumpusAI) movementTicker() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ai.context.Done():
            return
        case <-ticker.C:
            ai.luaVM.CallFunction("on_movement_tick")
        }
    }
}

// Handle events
func (ai *WumpusAI) OnPlayerEnterRoom(playerID, roomID string) {
    ai.luaVM.CallFunction("on_player_enter_room", playerID, roomID)
}

func (ai *WumpusAI) Shutdown() error {
    ai.cancel()
    
    // Call cleanup function
    ai.luaVM.CallFunction("cleanup")
    
    // Close Lua VM
    return ai.luaVM.Close()
}
```

## Error Handling Patterns

### Comprehensive Error Recovery

```go
package errors

import (
    "fmt"
    "log"
    "github.com/catalystcommunity/muddycore/pkg/storage"
)

type RecoveryManager struct {
    storage *storage.StorageManager
    logger  *log.Logger
}

type RecoveryAction string

const (
    ActionHealProperties RecoveryAction = "heal_properties"
    ActionResetToDefaults RecoveryAction = "reset_to_defaults"
    ActionRestoreBackup   RecoveryAction = "restore_backup"
    ActionCreateNew      RecoveryAction = "create_new"
)

// Recover from properties corruption
func (rm *RecoveryManager) RecoverPlayerProperties(player *storage.Player) error {
    recoveryPlan := rm.analyzeCorruption(player)
    
    for _, action := range recoveryPlan {
        if err := rm.executeRecoveryAction(player, action); err != nil {
            rm.logger.Printf("Recovery action failed: %v", err)
            continue
        }
        
        // Test if recovery was successful
        if rm.validatePlayerProperties(player) {
            rm.logger.Printf("Player %s properties recovered using action: %s", 
                player.ID, action)
            return nil
        }
    }
    
    return fmt.Errorf("all recovery attempts failed for player %s", player.ID)
}

func (rm *RecoveryManager) analyzeCorruption(player *storage.Player) []RecoveryAction {
    var actions []RecoveryAction
    
    // Check what's corrupted
    _, authErr := GetWumpusAuth(player)
    _, statsErr := GetWumpusStats(player)
    _, stateErr := GetWumpusState(player)
    
    if authErr != nil && statsErr != nil && stateErr != nil {
        // Everything is corrupted - try restore from backup first
        actions = append(actions, ActionRestoreBackup)
        actions = append(actions, ActionResetToDefaults)
    } else if authErr != nil {
        // Just auth is corrupted - try healing
        actions = append(actions, ActionHealProperties)
        actions = append(actions, ActionResetToDefaults)
    } else {
        // Partial corruption - try healing first
        actions = append(actions, ActionHealProperties)
    }
    
    return actions
}

func (rm *RecoveryManager) executeRecoveryAction(player *storage.Player, action RecoveryAction) error {
    switch action {
    case ActionHealProperties:
        return rm.healProperties(player)
    case ActionResetToDefaults:
        return rm.resetToDefaults(player)
    case ActionRestoreBackup:
        return rm.restoreFromBackup(player)
    case ActionCreateNew:
        return rm.createNewProperties(player)
    default:
        return fmt.Errorf("unknown recovery action: %s", action)
    }
}

func (rm *RecoveryManager) healProperties(player *storage.Player) error {
    // Try to fix each property individually
    
    // Heal auth data
    if _, err := GetWumpusAuth(player); err != nil {
        auth := &WumpusAuth{
            AuthMethod: "password",
            CreatedAt:  time.Now(),
        }
        if err := SetWumpusAuth(player, auth); err != nil {
            return err
        }
    }
    
    // Heal stats data
    if _, err := GetWumpusStats(player); err != nil {
        stats := &WumpusStats{
            GamesPlayed: 0,
            GamesWon:    0,
            WumpusPelts: 0,
            TotalDeaths: 0,
        }
        if err := SetWumpusStats(player, stats); err != nil {
            return err
        }
    }
    
    // Heal state data
    if _, err := GetWumpusState(player); err != nil {
        state := &WumpusState{
            CurrentInstance: "",
            Health:          100,
            IsInMaze:        false,
        }
        if err := SetWumpusState(player, state); err != nil {
            return err
        }
    }
    
    return nil
}
```

This comprehensive documentation provides practical examples for implementing game features using MuddyCore's Properties extension system. Each pattern demonstrates type-safe property access, error handling, and proper integration with the framework's existing systems.