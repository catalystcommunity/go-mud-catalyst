# Event Hooks Example

This example demonstrates how to use MuddyCore's Event Hook System to add custom behavior to your server without modifying the core library code.

## What is the Event Hook System?

The Event Hook System allows you to register callback functions that are triggered when specific events occur in your MuddyCore server. This enables you to:

- Add logging and monitoring
- Implement custom authentication logic
- Filter or modify messages
- Add game mechanics
- Handle system signals gracefully
- Create custom events for your application

## Running the Example

```bash
go run main.go
```

The server will start on `localhost:7777`. You can connect using:

```bash
# TCP connection
telnet localhost 7777

# Or use a WebSocket client to ws://localhost:7777
```

## Key Features Demonstrated

### 1. Connection Monitoring
```go
eventManager.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
    data := event.Data()
    fmt.Printf("🔗 New connection from %s\n", data["remote_addr"])
    return events.EventResultContinue
}, 0)
```

### 2. Authentication Hooks
```go
eventManager.RegisterHook(events.EventTypeAuthAttempt, func(event events.Event) events.EventResult {
    data := event.Data()
    fmt.Printf("🔐 Auth attempt: %s\n", data["username"])
    return events.EventResultContinue
}, 0)
```

### 3. Message Filtering (with Cancellation)
```go
eventManager.RegisterHook(events.EventTypeMessageReceived, func(event events.Event) events.EventResult {
    data := event.Data()
    msgType := data["message_type"]
    
    if msgType == message.MessageTypeAction {
        event.Cancel() // Block this message
        return events.EventResultCancel
    }
    
    return events.EventResultContinue
}, -100) // High priority (runs first)
```

### 4. Hook Priorities
- **Negative numbers** = Higher priority (run first)
- **Positive numbers** = Lower priority (run last)
- **0** = Normal priority

### 5. One-Time Hooks
```go
eventManager.RegisterHookOnce(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
    fmt.Println("🎉 First connection received!")
    return events.EventResultContinue
}, 0)
```

### 6. Graceful Shutdown
```go
shutdown := events.NewGracefulShutdown(eventManager)
shutdown.AddShutdownHook(func() error {
    fmt.Println("🧹 Cleaning up...")
    return nil
})
shutdown.Start()
```

### 7. Custom Events
```go
customEventType := events.EventType("game.player.levelup")
eventManager.RegisterHook(customEventType, func(event events.Event) events.EventResult {
    data := event.Data()
    fmt.Printf("Player %s leveled up!\n", data["player_name"])
    return events.EventResultContinue
}, 0)

// Trigger the custom event
event := eventManager.CreateEvent(customEventType, nil, map[string]interface{}{
    "player_name": "TestPlayer",
    "new_level":   42,
})
eventManager.TriggerEventAsync(event)
```

## Available Event Types

### Connection Events
- `EventTypeConnectionOpen` - New connection established
- `EventTypeConnectionClose` - Connection closed
- `EventTypeConnectionError` - Connection error occurred

### Authentication Events
- `EventTypeAuthAttempt` - Authentication attempt started
- `EventTypeAuthSuccess` - Authentication succeeded
- `EventTypeAuthFailure` - Authentication failed

### Message Events
- `EventTypeMessageReceived` - Message received from client
- `EventTypeMessageSent` - Message sent to client

### Room Events
- `EventTypeRoomJoin` - Client joined a room
- `EventTypeRoomLeave` - Client left a room

### System Events
- `EventTypeSystemSignal` - OS signal received (SIGINT, SIGTERM, etc.)
- `EventTypeSystemShutdown` - Server shutting down

### Game Events
- `EventTypePlayerJoin` - Player joined the game
- `EventTypePlayerLeave` - Player left the game
- `EventTypePlayerMove` - Player moved
- `EventTypePlayerAction` - Player performed an action

## Event Results

Your hook functions should return one of:

- `EventResultContinue` - Continue processing other hooks
- `EventResultStop` - Stop processing hooks, but don't cancel the event
- `EventResultCancel` - Cancel the event (prevents default action)

## Event Data Access

```go
func myHook(event events.Event) events.EventResult {
    data := event.Data()
    
    // Access specific fields
    clientID := data["client_id"].(string)
    username := data["username"].(string)
    
    // Modify event data (if the event is modifiable)
    event.SetData("custom_field", "custom_value")
    
    // Check if event was cancelled
    if event.IsCancelled() {
        return events.EventResultStop
    }
    
    return events.EventResultContinue
}
```

## Integration with Your Application

1. **Get the event manager** from your server:
```go
srv := server.NewServer("localhost", "7777")
eventManager := srv.GetEventManager()
```

2. **Register your hooks** before starting the server:
```go
eventManager.RegisterHook(events.EventTypeConnectionOpen, myConnectionHandler, 0)
```

3. **Create custom events** for your application logic:
```go
customEvent := events.EventType("myapp.custom.event")
eventManager.RegisterHook(customEvent, myCustomHandler, 0)
```

This approach allows you to extend MuddyCore's functionality without modifying the library itself, making your code modular and maintainable.