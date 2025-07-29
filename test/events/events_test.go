package events_test

import (
	"os"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// TestMain sets up and tears down for all tests in this package
func TestMain(m *testing.M) {
	// Set log level to WARN to reduce noise during tests
	testLogger := logging.NewLogger(logging.LevelWarn, os.Stderr)
	logging.SetDefaultLogger(testLogger)

	// Run tests
	code := m.Run()

	// Restore default logger
	logging.SetDefaultLogger(logging.DefaultLogger())

	os.Exit(code)
}

func TestEventManager(t *testing.T) {
	em := events.NewEventManager()
	defer em.Close()

	// Test basic event creation and triggering
	var receivedEvents []events.Event
	
	// Register a hook
	hookID := em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		receivedEvents = append(receivedEvents, event)
		return events.EventResultContinue
	}, 0)

	// Create and trigger an event
	event := em.CreateEvent(events.EventTypeConnectionOpen, "test-source", map[string]interface{}{
		"test_data": "test_value",
	})
	
	result := em.TriggerEventSync(event)
	if result != events.EventResultContinue {
		t.Errorf("Expected EventResultContinue, got %v", result)
	}

	// Verify hook was called
	if len(receivedEvents) != 1 {
		t.Errorf("Expected 1 event, got %d", len(receivedEvents))
	}

	// Test hook unregistration
	if !em.UnregisterHook(hookID) {
		t.Error("Failed to unregister hook")
	}

	// Trigger another event - should not be received
	em.TriggerEventSync(event)
	if len(receivedEvents) != 1 {
		t.Errorf("Expected 1 event after unregistration, got %d", len(receivedEvents))
	}
}

func TestEventPriorities(t *testing.T) {
	em := events.NewEventManager()
	defer em.Close()

	var executionOrder []int

	// Register hooks with different priorities
	em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		executionOrder = append(executionOrder, 2)
		return events.EventResultContinue
	}, 10) // Lower priority

	em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		executionOrder = append(executionOrder, 1)
		return events.EventResultContinue
	}, 5) // Higher priority

	em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		executionOrder = append(executionOrder, 3)
		return events.EventResultContinue
	}, 15) // Lowest priority

	// Trigger event
	event := em.CreateEvent(events.EventTypeConnectionOpen, nil, nil)
	em.TriggerEventSync(event)

	// Verify execution order (priority 5, 10, 15)
	expected := []int{1, 2, 3}
	if len(executionOrder) != len(expected) {
		t.Errorf("Expected %d hooks to execute, got %d", len(expected), len(executionOrder))
	}

	for i, expected := range expected {
		if i >= len(executionOrder) || executionOrder[i] != expected {
			t.Errorf("Expected execution order %v, got %v", expected, executionOrder)
			break
		}
	}
}

func TestEventCancellation(t *testing.T) {
	em := events.NewEventManager()
	defer em.Close()

	var hooksCalled int

	// Register hook that cancels the event
	em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		hooksCalled++
		event.Cancel()
		return events.EventResultCancel
	}, 5)

	// Register hook that should not be called due to cancellation
	em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		hooksCalled++
		return events.EventResultContinue
	}, 10)

	// Trigger event
	event := em.CreateEvent(events.EventTypeConnectionOpen, nil, nil)
	result := em.TriggerEventSync(event)

	if result != events.EventResultCancel {
		t.Errorf("Expected EventResultCancel, got %v", result)
	}

	if !event.IsCancelled() {
		t.Error("Event should be cancelled")
	}

	if hooksCalled != 1 {
		t.Errorf("Expected 1 hook to be called, got %d", hooksCalled)
	}
}

func TestOneTimeHooks(t *testing.T) {
	em := events.NewEventManager()
	defer em.Close()

	var callCount int

	// Register one-time hook
	em.RegisterHookOnce(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		callCount++
		return events.EventResultContinue
	}, 0)

	// Trigger event multiple times
	event := em.CreateEvent(events.EventTypeConnectionOpen, nil, nil)
	em.TriggerEventSync(event)
	em.TriggerEventSync(event)
	em.TriggerEventSync(event)

	// Should only be called once
	if callCount != 1 {
		t.Errorf("Expected hook to be called once, got %d times", callCount)
	}
}

func TestAsyncEvents(t *testing.T) {
	em := events.NewEventManager()
	defer em.Close()

	var receivedEvents []events.Event
	done := make(chan struct{})

	// Register hook
	em.RegisterHook(events.EventTypeConnectionOpen, func(event events.Event) events.EventResult {
		receivedEvents = append(receivedEvents, event)
		if len(receivedEvents) == 3 {
			close(done)
		}
		return events.EventResultContinue
	}, 0)

	// Trigger async events
	for i := 0; i < 3; i++ {
		event := em.CreateEvent(events.EventTypeConnectionOpen, nil, map[string]interface{}{
			"index": i,
		})
		em.TriggerEventAsync(event)
	}

	// Wait for events to be processed
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Timeout waiting for async events")
	}

	if len(receivedEvents) != 3 {
		t.Errorf("Expected 3 events, got %d", len(receivedEvents))
	}
}

func TestBuiltinEvents(t *testing.T) {
	// Test connection event creation
	connEvent := events.CreateConnectionOpenEvent("test-client", nil)
	if connEvent.Type() != events.EventTypeConnectionOpen {
		t.Errorf("Expected EventTypeConnectionOpen, got %v", connEvent.Type())
	}

	// Test auth event creation
	authEvent := events.CreateAuthSuccessEvent("client-1", "testuser", "password", nil)
	if authEvent.Type() != events.EventTypeAuthSuccess {
		t.Errorf("Expected EventTypeAuthSuccess, got %v", authEvent.Type())
	}

	// Test room event creation
	roomEvent := events.CreateRoomJoinEvent("room-1", "Test Room", "client-1", "testuser", 5)
	if roomEvent.Type() != events.EventTypeRoomJoin {
		t.Errorf("Expected EventTypeRoomJoin, got %v", roomEvent.Type())
	}
}

func TestEventData(t *testing.T) {
	em := events.NewEventManager()
	defer em.Close()

	// Create event with data
	event := em.CreateEvent(events.EventTypeConnectionOpen, "test-source", map[string]interface{}{
		"string_value": "test",
		"int_value":    42,
		"bool_value":   true,
	})

	// Test data access
	data := event.Data()
	if str, ok := data["string_value"].(string); !ok || str != "test" {
		t.Errorf("Expected string 'test', got %v (ok=%v)", str, ok)
	}

	if val, ok := data["int_value"].(int); !ok || val != 42 {
		t.Errorf("Expected int 42, got %v (ok=%v)", val, ok)
	}

	if val, ok := data["bool_value"].(bool); !ok || !val {
		t.Errorf("Expected bool true, got %v (ok=%v)", val, ok)
	}

	// Test modifying event data
	event.SetData("new_value", "added")
	data = event.Data()
	if data["new_value"] != "added" {
		t.Errorf("Expected 'added', got %v", data["new_value"])
	}
}

func TestEventBuilder(t *testing.T) {
	// Test connection event builder
	event := events.ConnectionOpen().
		WithConnectionID("conn-123").
		WithClientID("client-456").
		WithData("remote_addr", "192.168.1.100").
		Build()

	if event.Type() != events.EventTypeConnectionOpen {
		t.Errorf("Expected EventTypeConnectionOpen, got %v", event.Type())
	}

	data := event.Data()
	if data["connection_id"] != "conn-123" {
		t.Errorf("Expected conn-123, got %v", data["connection_id"])
	}

	if data["client_id"] != "client-456" {
		t.Errorf("Expected client-456, got %v", data["client_id"])
	}
}