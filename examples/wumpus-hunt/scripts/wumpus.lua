-- Wumpus AI Script for Hunt the Wumpus Game
-- This script controls the Wumpus behavior, movement, and combat mechanics
-- Uses MuddyCore's scripting system with Room/Player Properties for state management

-- Global state for this Wumpus instance
local wumpus_state = {
    current_room = nil,
    health = 100,
    is_alive = true,
    movement_timer = 0,
    movement_interval = 30, -- seconds between random movements
    last_player_room = nil,
    aggression_level = 1 -- 1 = passive, 2 = defensive, 3 = aggressive
}

-- Wumpus configuration
local wumpus_config = {
    base_damage = 50,
    movement_chance = 0.7, -- 70% chance to move when timer expires
    scent_range = 2, -- how many rooms away players can sense the Wumpus
    combat_damage = 75, -- damage dealt to players in same room
    flee_chance = 0.3 -- 30% chance to flee when attacked
}

-- Initialize the Wumpus AI system
function initialize_wumpus(room_id, instance_id)
    wumpus_state.current_room = room_id
    
    -- Set up the room properties to indicate Wumpus presence
    local room_data = {
        has_wumpus = true,
        wumpus_health = wumpus_state.health,
        scent_strength = 1.0
    }
    
    -- Log initialization
    log("INFO", string.format("Wumpus initialized in room %s (instance: %s)", room_id, instance_id))
    
    -- Schedule first movement check
    schedule_movement_check()
    
    return true
end

-- Handle Wumpus movement logic
function handle_wumpus_movement()
    if not wumpus_state.is_alive or not wumpus_state.current_room then
        return false
    end
    
    -- Check if it's time to move
    if wumpus_state.movement_timer > 0 then
        wumpus_state.movement_timer = wumpus_state.movement_timer - 1
        return false
    end
    
    -- Reset movement timer
    wumpus_state.movement_timer = wumpus_state.movement_interval
    
    -- Determine if Wumpus should move
    local should_move = math.random() < wumpus_config.movement_chance
    
    if should_move then
        return move_wumpus_randomly()
    end
    
    return false
end

-- Move Wumpus to a random adjacent room
function move_wumpus_randomly()
    if not wumpus_state.current_room then
        return false
    end
    
    -- Get adjacent rooms from the Go movement AI
    local adjacent_rooms = call_movement_ai("get_adjacent_rooms", wumpus_state.current_room)
    
    if not adjacent_rooms or #adjacent_rooms == 0 then
        log("WARN", "Wumpus has no adjacent rooms to move to")
        return false
    end
    
    -- Choose random adjacent room
    local new_room = adjacent_rooms[math.random(#adjacent_rooms)]
    
    -- Execute movement through Go movement AI
    local success = call_movement_ai("move_to", new_room)
    
    if success then
        local old_room = wumpus_state.current_room
        wumpus_state.current_room = new_room
        
        log("INFO", string.format("Wumpus moved from %s to %s", old_room, new_room))
        
        -- Emit movement event
        emit_event("wumpus_moved", {
            from_room = old_room,
            to_room = new_room,
            instance_id = get_current_instance()
        })
        
        return true
    else
        log("WARN", string.format("Failed to move Wumpus to room %s", new_room))
        return false
    end
end

-- Handle player entering a room
function on_player_enter_room(player_id, room_id)
    if not wumpus_state.is_alive then
        return
    end
    
    -- Track player location for AI decisions
    wumpus_state.last_player_room = room_id
    
    -- Check if player entered Wumpus room
    if room_id == wumpus_state.current_room then
        handle_wumpus_encounter(player_id, room_id)
    else
        -- Update aggression based on proximity
        local distance = calculate_room_distance(wumpus_state.current_room, room_id)
        if distance <= 2 then
            increase_aggression()
        end
    end
    
    -- Update scent information for the player
    update_player_scent_info(player_id, room_id)
end

-- Handle direct Wumpus encounter
function handle_wumpus_encounter(player_id, room_id)
    log("INFO", string.format("Player %s encountered Wumpus in room %s", player_id, room_id))
    
    -- Emit encounter event
    emit_event("wumpus_encounter", {
        player_id = player_id,
        room_id = room_id,
        wumpus_health = wumpus_state.health,
        instance_id = get_current_instance()
    })
    
    -- Determine Wumpus reaction based on aggression level
    if wumpus_state.aggression_level >= 2 then
        attack_player(player_id)
    else
        -- Passive Wumpus - chance to flee
        if math.random() < wumpus_config.flee_chance then
            flee_from_player()
        else
            attack_player(player_id)
        end
    end
end

-- Wumpus attacks player
function attack_player(player_id)
    local damage = wumpus_config.combat_damage
    
    log("INFO", string.format("Wumpus attacks player %s for %d damage", player_id, damage))
    
    -- Emit attack event (the game state manager will handle actual damage)
    emit_event("wumpus_attack", {
        player_id = player_id,
        damage = damage,
        room_id = wumpus_state.current_room,
        instance_id = get_current_instance()
    })
end

-- Handle player attacking Wumpus
function on_player_attack_wumpus(player_id, damage)
    if not wumpus_state.is_alive then
        return false
    end
    
    wumpus_state.health = wumpus_state.health - damage
    
    log("INFO", string.format("Player %s attacks Wumpus for %d damage (health: %d)", 
        player_id, damage, wumpus_state.health))
    
    if wumpus_state.health <= 0 then
        handle_wumpus_death(player_id)
        return true
    end
    
    -- Increase aggression when attacked
    wumpus_state.aggression_level = math.min(3, wumpus_state.aggression_level + 1)
    
    -- Chance to counter-attack
    if math.random() < 0.8 then -- 80% chance to counter-attack
        attack_player(player_id)
    end
    
    -- Chance to flee if badly wounded
    if wumpus_state.health < 30 and math.random() < 0.5 then
        flee_from_player()
    end
    
    -- Emit damage event
    emit_event("wumpus_damaged", {
        player_id = player_id,
        damage = damage,
        remaining_health = wumpus_state.health,
        instance_id = get_current_instance()
    })
    
    return false
end

-- Handle Wumpus death
function handle_wumpus_death(killer_player_id)
    wumpus_state.is_alive = false
    
    log("INFO", string.format("Wumpus killed by player %s", killer_player_id))
    
    -- Clear Wumpus presence from current room
    clear_wumpus_presence(wumpus_state.current_room)
    
    -- Emit death event
    emit_event("wumpus_death", {
        killer_player_id = killer_player_id,
        room_id = wumpus_state.current_room,
        instance_id = get_current_instance()
    })
end

-- Wumpus flees to adjacent room
function flee_from_player(player_room_id)
    log("INFO", string.format("Wumpus is fleeing from player in room %s", player_room_id or "unknown"))
    
    -- Use Go movement AI to flee intelligently
    local success = call_movement_ai("flee_from_player", player_room_id or wumpus_state.last_player_room)
    
    if success then
        -- Increase movement frequency when fleeing
        wumpus_state.movement_timer = math.floor(wumpus_state.movement_interval / 2)
        
        -- Get new room from movement AI
        local new_room = call_movement_ai("get_current_room")
        if new_room then
            local old_room = wumpus_state.current_room
            wumpus_state.current_room = new_room
            
            emit_event("wumpus_fled", {
                from_room = old_room,
                to_room = new_room,
                instance_id = get_current_instance()
            })
        end
    else
        log("WARN", "Failed to flee from player, attempting random movement")
        move_wumpus_randomly()
    end
end

-- Update scent trails around Wumpus location
function update_scent_trails()
    if not wumpus_state.current_room then
        return
    end
    
    -- Delegate scent trail updates to Go movement AI
    local success = call_movement_ai("update_scent_trails")
    
    if success then
        log("DEBUG", string.format("Updated scent trails centered on room %s", wumpus_state.current_room))
    else
        log("WARN", "Failed to update scent trails")
    end
end

-- Utility functions to interface with MuddyCore's room system
function set_wumpus_presence(room_id)
    -- This would interface with the room properties system
    -- Implementation depends on how we access MuddyCore's storage from Lua
    log("DEBUG", string.format("Setting Wumpus presence in room %s", room_id))
end

function clear_wumpus_presence(room_id)
    log("DEBUG", string.format("Clearing Wumpus presence from room %s", room_id))
end

function set_room_scent(room_id, strength, message)
    log("DEBUG", string.format("Setting scent in room %s: %s (strength: %.1f)", room_id, message, strength))
end

function get_adjacent_rooms(room_id)
    -- This would query the maze structure for adjacent rooms
    -- For now, return empty array - will be implemented when we have room data access
    log("DEBUG", string.format("Getting adjacent rooms for %s", room_id))
    return {}
end

function calculate_room_distance(room1, room2)
    -- Use Go movement AI for pathfinding
    local distance = call_movement_ai("calculate_room_distance", room1, room2)
    if distance and distance >= 0 then
        return distance
    end
    
    -- Fallback for error cases
    log("WARN", string.format("Failed to calculate distance between %s and %s", room1, room2))
    return 1
end

function increase_aggression()
    wumpus_state.aggression_level = math.min(3, wumpus_state.aggression_level + 1)
    log("DEBUG", string.format("Wumpus aggression increased to level %d", wumpus_state.aggression_level))
end

function schedule_movement_check()
    -- This would schedule the next movement check using MuddyCore's timer system
    log("DEBUG", "Scheduling next Wumpus movement check")
end

function update_player_scent_info(player_id, room_id)
    -- This would provide scent information to the player based on proximity to Wumpus
    log("DEBUG", string.format("Updating scent info for player %s in room %s", player_id, room_id))
end

function get_current_instance()
    -- This would return the current game instance ID
    return "current_instance"
end

-- Movement AI interface functions
function call_movement_ai(action, ...)
    -- This function interfaces with the Go movement AI system
    -- In a real implementation, this would use MuddyCore's Lua-Go bridge
    -- For now, return mock responses to prevent errors
    
    if action == "get_adjacent_rooms" then
        -- Mock adjacent rooms
        local room_id = ...
        if room_id == "room_1" then
            return {"room_2", "room_3"}
        elseif room_id == "room_2" then
            return {"room_1", "room_4"}
        elseif room_id == "room_3" then
            return {"room_1", "room_4"}
        elseif room_id == "room_4" then
            return {"room_2", "room_3"}
        end
        return {}
    elseif action == "move_to" then
        -- Mock successful movement
        return true
    elseif action == "flee_from_player" then
        -- Mock successful flee
        return true
    elseif action == "get_current_room" then
        -- Mock current room
        return wumpus_state.current_room
    elseif action == "calculate_room_distance" then
        -- Mock distance calculation
        local room1, room2 = ...
        if room1 == room2 then
            return 0
        else
            return 1
        end
    elseif action == "update_scent_trails" then
        -- Mock successful scent update
        return true
    end
    
    -- Unknown action
    log("WARN", string.format("Unknown movement AI action: %s", action))
    return false
end

-- Main event handlers that MuddyCore will call
-- These functions will be registered with the event system

-- Called when the script is loaded
function on_script_load()
    log("INFO", "Wumpus AI script loaded successfully")
    return true
end

-- Called when the script is unloaded
function on_script_unload()
    log("INFO", "Wumpus AI script unloading")
    return true
end

-- Periodic update function called by the system
function on_update(delta_time)
    if wumpus_state.is_alive then
        handle_wumpus_movement()
    end
end

-- Export public functions for external access
return {
    initialize_wumpus = initialize_wumpus,
    on_player_enter_room = on_player_enter_room,
    on_player_attack_wumpus = on_player_attack_wumpus,
    handle_wumpus_movement = handle_wumpus_movement,
    on_script_load = on_script_load,
    on_script_unload = on_script_unload,
    on_update = on_update
}