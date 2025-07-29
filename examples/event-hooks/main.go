package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/server"
)

// This example demonstrates how to use the MuddyCore event system
// as a library to add custom behavior without modifying core code

func main() {
	fmt.Println("MuddyCore Event Hooks Example")
	fmt.Println("==============================")
	
	// Create a server
	srv := server.NewServer("localhost", "7777")
	eventManager := srv.GetEventManager()
	
	// Register custom event hooks to add functionality
	registerCustomHooks(eventManager)
	
	// Set up graceful shutdown
	setupGracefulShutdown(eventManager, srv)
	
	// Start the server
	if err := srv.StartServer(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	
	fmt.Println("Server started on localhost:7777")
	fmt.Println("Try connecting with telnet or a WebSocket client")
	fmt.Println("Press Ctrl+C to shutdown gracefully")
	
	// Wait for shutdown signal
	srv.WaitForShutdown()
	fmt.Println("Server has shut down")
}

// registerCustomHooks demonstrates how to add custom behavior via event hooks
func registerCustomHooks(eventManager events.EventManager) {
	// 1. Connection Monitoring Hook
	eventManager.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🔗 New connection from %s (ID: %s)\n", 
			data["remote_addr"], data["connection_id"])
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventTypeConnectionClose, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("❌ Connection closed: %s (ID: %s)\n", 
			data["remote_addr"], data["connection_id"])
		return events.EventResultContinue
	}, 0)
	
	// 2. Authentication Logging Hook
	eventManager.RegisterHook(events.EventTypeAuthAttempt, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🔐 Auth attempt: %s trying to login as '%s'\n", 
			data["client_id"], data["username"])
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventTypeAuthSuccess, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("✅ Auth success: %s logged in as '%s'\n", 
			data["client_id"], data["username"])
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventTypeAuthFailure, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🚫 Auth failed: %s failed to login as '%s' - %s\n", 
			data["client_id"], data["username"], data["error"])
		return events.EventResultContinue
	}, 0)
	
	// 3. Message Filtering Hook (high priority to run first)
	eventManager.RegisterHook(events.EventTypeMessageReceived, func(event events.Event) events.EventResult {
		data := event.Data()
		msgType := data["message_type"]
		
		// Example: Block certain message types during maintenance
		if msgType == message.MessageTypeAction {
			fmt.Printf("🛡️  Blocked action message from %s (maintenance mode)\n", data["client_id"])
			event.Cancel() // Cancel the message processing
			return events.EventResultCancel
		}
		
		fmt.Printf("📨 Message received: type=%d from=%s size=%d\n", 
			msgType, data["client_id"], data["size"])
		return events.EventResultContinue
	}, -100) // High priority (negative number)
	
	// 4. Room Activity Hook
	eventManager.RegisterHook(events.EventTypeRoomJoin, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🏠 %s joined room '%s' (now %d clients)\n", 
			data["client_name"], data["room_id"], data["client_count"])
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventTypeRoomLeave, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🚪 %s left room '%s' (now %d clients)\n", 
			data["client_name"], data["room_id"], data["client_count"])
		return events.EventResultContinue
	}, 0)
	
	// 5. Custom Game Event Hook
	eventManager.RegisterHook(events.EventTypePlayerAction, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🎮 Player action: %s performed '%s' on '%s'\n", 
			data["player_name"], data["action"], data["target"])
		return events.EventResultContinue
	}, 0)
	
	// 6. Message Statistics Hook (low priority to run last)
	messageCount := 0
	eventManager.RegisterHook(events.EventTypeMessageSent, func(event events.Event) events.EventResult {
		messageCount++
		if messageCount%10 == 0 {
			fmt.Printf("📊 Statistics: %d messages sent so far\n", messageCount)
		}
		return events.EventResultContinue
	}, 100) // Low priority (high number)
	
	// 7. System Signal Handler
	eventManager.RegisterHook(events.EventTypeSystemSignal, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("⚠️  System signal received: %s (reason: %s)\n", 
			data["signal"], data["reason"])
		return events.EventResultContinue
	}, 0)
	
	// 8. Server Lifecycle Hook
	eventManager.RegisterHook(events.EventTypeServerStart, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🚀 Server started on %s:%s\n", data["host"], data["port"])
		return events.EventResultContinue
	}, 0)
	
	eventManager.RegisterHook(events.EventTypeServerShutdown, func(event events.Event) events.EventResult {
		data := event.Data()
		graceful := data["graceful"].(bool)
		if graceful {
			fmt.Printf("🛑 Server shutting down gracefully...\n")
		} else {
			fmt.Printf("⚡ Server performing emergency shutdown!\n")
		}
		return events.EventResultContinue
	}, 0)
	
	// 9. One-time Welcome Hook (only runs once)
	eventManager.RegisterHookOnce(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		fmt.Printf("🎉 First connection received! Server is ready.\n")
		return events.EventResultContinue
	}, -1000) // Very high priority
	
	fmt.Println("✅ Registered custom event hooks")
}

// setupGracefulShutdown demonstrates using the signal handling system
func setupGracefulShutdown(eventManager events.EventManager, srv *server.ConnServer) {
	// Create graceful shutdown manager
	shutdown := events.NewGracefulShutdown(eventManager)
	
	// Add cleanup hooks (called in reverse order)
	shutdown.AddShutdownHook(func() error {
		fmt.Println("🧹 Cleaning up resources...")
		time.Sleep(500 * time.Millisecond) // Simulate cleanup
		return nil
	})
	
	shutdown.AddShutdownHook(func() error {
		fmt.Println("💾 Saving state...")
		time.Sleep(300 * time.Millisecond) // Simulate saving
		return nil
	})
	
	shutdown.AddShutdownHook(func() error {
		fmt.Println("📨 Notifying clients of shutdown...")
		// In a real app, you'd send shutdown notifications to clients
		return nil
	})
	
	shutdown.AddShutdownHook(func() error {
		fmt.Println("🛑 Stopping server...")
		srv.Shutdown()
		return nil
	})
	
	// Start the signal handler
	shutdown.Start()
	
	// Also handle manual shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		fmt.Printf("\n📡 Received signal: %v\n", sig)
		shutdown.TriggerShutdown(fmt.Sprintf("Manual shutdown via %v", sig))
	}()
	
	fmt.Println("⚡ Graceful shutdown system configured")
}

// Example: Custom event for a game feature
func demonstrateCustomEvents(eventManager events.EventManager) {
	// Create a custom event type
	customEventType := events.EventType("game.player.levelup")
	
	// Register a handler for it
	eventManager.RegisterHook(customEventType, func(event events.Event) events.EventResult {
		data := event.Data()
		fmt.Printf("🎊 Player %s leveled up to level %d!\n", 
			data["player_name"], data["new_level"])
		return events.EventResultContinue
	}, 0)
	
	// Trigger the custom event
	event := eventManager.CreateEvent(customEventType, nil, map[string]interface{}{
		"player_name": "TestPlayer",
		"new_level":   42,
		"experience":  125000,
	})
	
	eventManager.TriggerEventAsync(event)
}