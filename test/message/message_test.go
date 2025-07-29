package message_test

import (
	"os"
	"testing"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
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

func TestMessageWrapUnwrap(t *testing.T) {
	msg := &message.Message{
		Type:     message.MessageTypeChat,
		Contents: []byte("hello world"),
	}
	
	// Test wrapping
	wrapped, err := message.MessageWrap(msg)
	if err != nil {
		t.Fatalf("Failed to wrap message: %v", err)
	}
	
	// Test unwrapping
	unwrapped, err := message.MessageUnwrap(wrapped)
	if err != nil {
		t.Fatalf("Failed to unwrap message: %v", err)
	}
	
	if unwrapped.Type != msg.Type {
		t.Errorf("Expected type %d, got %d", msg.Type, unwrapped.Type)
	}
	
	if string(unwrapped.Contents) != string(msg.Contents) {
		t.Errorf("Expected contents '%s', got '%s'", string(msg.Contents), string(unwrapped.Contents))
	}
}

func TestMessageRouter(t *testing.T) {
	router := message.NewMessageRouter()
	
	// Test handler registration and routing
	handlerCalled := false
	handler := func(msg *message.Message) error {
		handlerCalled = true
		return nil
	}
	
	router.RegisterHandler(message.MessageTypeChat, handler)
	
	msg := &message.Message{Type: message.MessageTypeChat, Contents: []byte("test")}
	err := router.RouteMessage(msg)
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if !handlerCalled {
		t.Error("Handler was not called")
	}
}

func TestMessageRouterNoHandler(t *testing.T) {
	router := message.NewMessageRouter()
	
	msg := &message.Message{Type: message.MessageTypeChat, Contents: []byte("test")}
	err := router.RouteMessage(msg)
	
	if err == nil {
		t.Error("Expected error when no handler is registered")
	}
	
	if _, ok := err.(*message.MessageError); !ok {
		t.Error("Expected message.MessageError type")
	}
}

func TestMessageRouterWithValidator(t *testing.T) {
	router := message.NewMessageRouter()
	
	// Register validator that always fails
	validator := func(msg *message.Message) bool {
		return false
	}
	router.RegisterValidator(message.MessageTypeChat, validator)
	
	// Register handler (should not be called due to validation failure)
	handlerCalled := false
	handler := func(msg *message.Message) error {
		handlerCalled = true
		return nil
	}
	router.RegisterHandler(message.MessageTypeChat, handler)
	
	msg := &message.Message{Type: message.MessageTypeChat, Contents: []byte("test")}
	err := router.RouteMessage(msg)
	
	if err == nil {
		t.Error("Expected validation error")
	}
	
	if handlerCalled {
		t.Error("Handler should not have been called due to validation failure")
	}
}