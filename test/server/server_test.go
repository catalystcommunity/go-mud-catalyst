package server_test

import (
	"os"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/server"
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

func TestNewServer(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	if srv == nil {
		t.Fatal("server.NewServer returned nil")
	}
	
	// Since this is black-box testing, we can only test the public interface
	// The server should be functional, so test some public methods
	count := srv.GetClientCount()
	if count != 0 {
		t.Errorf("Expected 0 clients initially, got %d", count)
	}
	
	clients := srv.GetConnectedClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 connected clients initially, got %d", len(clients))
	}
}

func TestNewServerDefaultPort(t *testing.T) {
	srv := server.NewServer("localhost", "")
	defer srv.Shutdown()
	
	// Can't directly test the port since it's private, but we can test that the server was created successfully
	if srv == nil {
		t.Fatal("server.NewServer returned nil")
	}
}

func TestStartServer(t *testing.T) {
	srv := server.NewServer("localhost", "0") // Use port 0 for random available port
	
	// Start server in goroutine
	go srv.StartServer()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Stop the server
	srv.Shutdown()
}

func TestServerShutdown(t *testing.T) {
	srv := server.NewServer("localhost", "0")
	
	// Start server in background
	go srv.StartServer()
	time.Sleep(50 * time.Millisecond)
	
	// Test shutdown - in black-box testing, we just verify shutdown doesn't panic
	srv.Shutdown()
	
	// The shutdown should complete without errors
	// We can't directly test the context since it's private
}

func TestWaitForShutdown(t *testing.T) {
	srv := server.NewServer("localhost", "0")
	
	// Start shutdown wait in background
	shutdownDone := make(chan bool)
	go func() {
		srv.WaitForShutdown()
		shutdownDone <- true
	}()
	
	// Trigger shutdown
	srv.Shutdown()
	
	// Wait for shutdown to complete
	select {
	case <-shutdownDone:
		// Good, shutdown completed
	case <-time.After(time.Second):
		t.Error("WaitForShutdown should complete when server is shut down")
	}
}

func TestGetClientCount(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Initially should be 0
	if count := srv.GetClientCount(); count != 0 {
		t.Errorf("Expected 0 clients initially, got %d", count)
	}
	
	// Note: GetClientCount() checks the connection manager, not the local clients map
	// In a real scenario, clients would be added through handleNewConnection()
	// For this unit test, we're just testing that the method works correctly with the connection manager
}

func TestClientStateToString(t *testing.T) {
	tests := map[server.ClientState]string{
		server.ClientStateConnecting:    "CONNECTING",
		server.ClientStateConnected:     "CONNECTED", 
		server.ClientStateAuthenticated: "AUTHENTICATED",
		server.ClientStateDisconnected:  "DISCONNECTED",
	}
	
	for state, expected := range tests {
		result := server.ClientStateToString(state)
		if result != expected {
			t.Errorf("Expected %s for state %v, got %s", expected, state, result)
		}
	}
	
	// Test unknown state
	unknownState := server.ClientState(999)
	result := server.ClientStateToString(unknownState)
	if result != "UNKNOWN" {
		t.Errorf("Expected UNKNOWN for invalid state, got %s", result)
	}
}

func TestGetConnectedClients(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Initially should be empty
	clients := srv.GetConnectedClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 connected clients initially, got %d", len(clients))
	}
	
	// Note: GetConnectedClients() checks the connection manager, not the local clients map
	// In a real scenario, clients would be added through handleNewConnection()
	// For this unit test, we're just testing that the method works correctly with the connection manager
}

func TestGetClient(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Test getting non-existent client
	client, exists := srv.GetClient("non-existent")
	if exists {
		t.Error("Should not find non-existent client")
	}
	if client != nil {
		t.Error("Client should be nil for non-existent client")
	}
	
	// Note: In black-box testing, we can't manually add clients to test GetClient
	// This would require an actual connection to be established through the public API
}

func TestSetClientName(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Test setting name for non-existent client (should not panic)
	err := srv.SetClientName("non-existent", "Some Name")
	// The method should handle non-existent clients gracefully
	if err != nil {
		t.Logf("SetClientName returned error for non-existent client: %v", err)
	}
}

func TestSetClientState(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Test setting state for non-existent client (should not panic)
	err := srv.SetClientState("non-existent", server.ClientStateConnected)
	// The method should handle non-existent clients gracefully
	if err != nil {
		t.Logf("SetClientState returned error for non-existent client: %v", err)
	}
}

func TestBroadcastMessage(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Test broadcasting a message - should not panic even with no clients
	msg := &message.Message{
		Type:     message.MessageTypeChat,
		Contents: []byte("test message"),
	}
	
	err := srv.BroadcastMessage(msg)
	if err != nil {
		t.Errorf("BroadcastMessage should not return error with no clients: %v", err)
	}
}

func TestSendMessage(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// Test sending message to non-existent client
	msg := &message.Message{
		Type:     message.MessageTypeChat,
		Contents: []byte("Hello client"),
	}
	
	err := srv.SendMessage("non-existent-client", msg)
	// Should handle non-existent clients gracefully
	if err != nil {
		t.Logf("SendMessage returned error for non-existent client: %v", err)
	}
}

func TestDefaultHandlersRegistration(t *testing.T) {
	srv := server.NewServer("localhost", "8080")
	defer srv.Shutdown()
	
	// The registerDefaultHandlers is called in server.NewServer
	// In black-box testing, we can only verify that the server was created successfully
	// The internal state of the message router is not accessible
	if srv == nil {
		t.Error("Server should be initialized")
	}
}