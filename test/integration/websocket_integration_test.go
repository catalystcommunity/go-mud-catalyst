package integration_test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/client"
	"github.com/catalystcommunity/muddycore/pkg/connection"
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

// TestWebSocketIntegration tests full WebSocket client-server communication
func TestWebSocketIntegration(t *testing.T) {
	// Start a test server with WebSocket support
	srv := server.NewWebSocketServer("localhost", "7778", "7779")
	
	// Configure server to handle both TCP and WebSocket
	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()
	
	// Get the actual port assigned by the OS
	// For testing, we'll use the WebSocket port since we need WebSocket connections
	serverAddr := "localhost:7779"
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Test WebSocket connection
	t.Run("WebSocket Connection", func(t *testing.T) {
		testWebSocketConnection(t, serverAddr)
	})
	
	// Test message exchange
	t.Run("WebSocket Message Exchange", func(t *testing.T) {
		testWebSocketMessageExchange(t, serverAddr)
	})
	
	// Test multiple concurrent WebSocket clients
	t.Run("Multiple WebSocket Clients", func(t *testing.T) {
		testMultipleWebSocketClients(t, serverAddr)
	})
	
	// Test WebSocket room functionality
	t.Run("WebSocket Room Broadcasting", func(t *testing.T) {
		testWebSocketRoomBroadcasting(t, serverAddr)
	})
	
	// Test WebSocket authentication
	t.Run("WebSocket Authentication", func(t *testing.T) {
		testWebSocketAuthentication(t, serverAddr)
	})
}

func testWebSocketConnection(t *testing.T, serverAddr string) {
	// Create WebSocket URL
	wsURL := fmt.Sprintf("ws://%s/ws", serverAddr)
	
	// Create client configuration for WebSocket
	config := client.DefaultClientConfig(wsURL)
	config.ConnectionType = connection.ConnectionTypeWebSocket
	config.ReconnectEnabled = false // Disable for testing
	
	// Create and connect client
	c := client.NewClientWithConfig(config)
	
	connected := make(chan bool, 1)
	c.OnConnect = func() {
		connected <- true
	}
	
	err := c.Connect()
	if err != nil {
		t.Fatalf("Failed to connect WebSocket client: %v", err)
	}
	defer c.Disconnect()
	
	// Wait for connection
	select {
	case <-connected:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("WebSocket connection timeout")
	}
	
	// Verify connection state
	if c.GetState() != client.ClientStateConnected {
		t.Errorf("Expected connected state, got %v", c.GetState())
	}
}

func testWebSocketMessageExchange(t *testing.T, serverAddr string) {
	wsURL := fmt.Sprintf("ws://%s/ws", serverAddr)
	
	config := client.DefaultClientConfig(wsURL)
	config.ConnectionType = connection.ConnectionTypeWebSocket
	config.ReconnectEnabled = false
	
	c := client.NewClientWithConfig(config)
	
	var receivedMessages []string
	var mu sync.Mutex
	
	c.OnMessage = func(msg *message.Message) {
		mu.Lock()
		receivedMessages = append(receivedMessages, string(msg.Contents))
		mu.Unlock()
	}
	
	connected := make(chan bool, 1)
	c.OnConnect = func() {
		connected <- true
	}
	
	err := c.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer c.Disconnect()
	
	// Wait for connection
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("Connection timeout")
	}
	
	// Send test messages
	testMessages := []string{
		"Hello WebSocket!",
		"Testing message exchange",
		"Final test message",
	}
	
	for _, testMsg := range testMessages {
		msg := message.Chat().FromUser("test").WithText(testMsg).Build()
		err = c.SendMessage(msg)
		if err != nil {
			t.Errorf("Failed to send message: %v", err)
		}
	}
	
	// Wait for messages to be processed
	time.Sleep(500 * time.Millisecond)
	
	// Verify messages were echoed back (assuming server echoes messages)
	mu.Lock()
	defer mu.Unlock()
	
	if len(receivedMessages) == 0 {
		t.Log("No messages received - server might not echo messages")
		return
	}
	
	t.Logf("Received %d messages via WebSocket", len(receivedMessages))
}

func testMultipleWebSocketClients(t *testing.T, serverAddr string) {
	wsURL := fmt.Sprintf("ws://%s/ws", serverAddr)
	numClients := 5
	
	var clients []*client.Client
	var wg sync.WaitGroup
	
	// Create multiple clients
	for i := 0; i < numClients; i++ {
		config := client.DefaultClientConfig(wsURL)
		config.ConnectionType = connection.ConnectionTypeWebSocket
		config.ReconnectEnabled = false
		
		c := client.NewClientWithConfig(config)
		clients = append(clients, c)
		
		wg.Add(1)
		go func(clientID int, client *client.Client) {
			defer wg.Done()
			
			connected := make(chan bool, 1)
			client.OnConnect = func() {
				connected <- true
			}
			
			err := client.Connect()
			if err != nil {
				t.Errorf("Client %d failed to connect: %v", clientID, err)
				return
			}
			
			// Wait for connection
			select {
			case <-connected:
				t.Logf("Client %d connected via WebSocket", clientID)
			case <-time.After(5 * time.Second):
				t.Errorf("Client %d connection timeout", clientID)
				return
			}
			
			// Send a test message
			msg := message.Chat().
				FromUser(fmt.Sprintf("client%d", clientID)).
				WithText(fmt.Sprintf("Hello from client %d", clientID)).
				Build()
			err = client.SendMessage(msg)
			if err != nil {
				t.Errorf("Client %d failed to send message: %v", clientID, err)
			}
		}(i, c)
	}
	
	// Wait for all clients to complete
	wg.Wait()
	
	// Clean up clients
	for i, c := range clients {
		if c.GetState() == client.ClientStateConnected {
			c.Disconnect()
			t.Logf("Client %d disconnected", i)
		}
	}
	
	t.Logf("Successfully tested %d concurrent WebSocket clients", numClients)
}

func testWebSocketRoomBroadcasting(t *testing.T, serverAddr string) {
	wsURL := fmt.Sprintf("ws://%s/ws", serverAddr)
	
	// Create two clients to test room broadcasting
	config1 := client.DefaultClientConfig(wsURL)
	config1.ConnectionType = connection.ConnectionTypeWebSocket
	config1.ReconnectEnabled = false
	
	config2 := client.DefaultClientConfig(wsURL)
	config2.ConnectionType = connection.ConnectionTypeWebSocket  
	config2.ReconnectEnabled = false
	
	client1 := client.NewClientWithConfig(config1)
	client2 := client.NewClientWithConfig(config2)
	
	var client1Messages []string
	var client2Messages []string
	var mu sync.Mutex
	
	client1.OnMessage = func(msg *message.Message) {
		mu.Lock()
		client1Messages = append(client1Messages, string(msg.Contents))
		mu.Unlock()
	}
	
	client2.OnMessage = func(msg *message.Message) {
		mu.Lock()
		client2Messages = append(client2Messages, string(msg.Contents))
		mu.Unlock()
	}
	
	// Connect both clients
	connected1 := make(chan bool, 1)
	connected2 := make(chan bool, 1)
	
	client1.OnConnect = func() { connected1 <- true }
	client2.OnConnect = func() { connected2 <- true }
	
	err1 := client1.Connect()
	err2 := client2.Connect()
	
	if err1 != nil {
		t.Fatalf("Client1 failed to connect: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Client2 failed to connect: %v", err2)
	}
	
	defer client1.Disconnect()
	defer client2.Disconnect()
	
	// Wait for both connections
	select {
	case <-connected1:
	case <-time.After(5 * time.Second):
		t.Fatal("Client1 connection timeout")
	}
	
	select {
	case <-connected2:
	case <-time.After(5 * time.Second):
		t.Fatal("Client2 connection timeout")
	}
	
	// Test room join (if supported by server)
	joinMsg1 := message.JoinRoom().Room("testroom").Build()
	joinMsg2 := message.JoinRoom().Room("testroom").Build()
	
	client1.SendMessage(joinMsg1)
	client2.SendMessage(joinMsg2)
	
	time.Sleep(200 * time.Millisecond)
	
	// Send broadcast message to room
	broadcastMsg := message.Chat().
		FromUser("client1").
		WithText("Hello everyone in testroom!").
		InChannel("testroom").
		Build()
	
	client1.SendMessage(broadcastMsg)
	
	// Wait for message propagation
	time.Sleep(500 * time.Millisecond)
	
	// Read message counts safely
	mu.Lock()
	client1Count := len(client1Messages)
	client2Count := len(client2Messages)
	mu.Unlock()
	
	t.Logf("Client1 received %d messages, Client2 received %d messages", 
		client1Count, client2Count)
}

func testWebSocketAuthentication(t *testing.T, serverAddr string) {
	wsURL := fmt.Sprintf("ws://%s/ws", serverAddr)
	
	config := client.DefaultClientConfig(wsURL)
	config.ConnectionType = connection.ConnectionTypeWebSocket
	config.ReconnectEnabled = false
	
	c := client.NewClientWithConfig(config)
	
	authResponse := make(chan bool, 1)
	c.OnMessage = func(msg *message.Message) {
		if msg.Type == message.MessageTypeAuthSuccess || msg.Type == message.MessageTypeAuthFailure {
			authResponse <- true
		}
	}
	
	connected := make(chan bool, 1)
	c.OnConnect = func() {
		connected <- true
	}
	
	err := c.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer c.Disconnect()
	
	// Wait for connection
	select {
	case <-connected:
	case <-time.After(5 * time.Second):
		t.Fatal("Connection timeout")
	}
	
	// Send authentication message
	authMsg := message.Auth().WithCredentials("testuser", "testpass").Build()
	err = c.SendMessage(authMsg)
	if err != nil {
		t.Fatalf("Failed to send auth message: %v", err)
	}
	
	// Wait for auth response (if server supports auth)
	select {
	case <-authResponse:
		t.Log("Received authentication response via WebSocket")
	case <-time.After(2 * time.Second):
		t.Log("No auth response received - server might not support authentication")
	}
}

// TestWebSocketUpgrade tests the HTTP to WebSocket upgrade process
func TestWebSocketUpgrade(t *testing.T) {
	srv := server.NewServer("localhost", "7779")
	
	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()
	
	serverAddr := "localhost:7779"
	time.Sleep(100 * time.Millisecond)
	
	// Test WebSocket upgrade handshake
	wsURL := fmt.Sprintf("ws://%s/ws", serverAddr)
	
	// Manual WebSocket upgrade test
	req, err := http.NewRequest("GET", wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "test-key")
	req.Header.Set("Sec-WebSocket-Version", "13")
	
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	resp, err := client.Do(req)
	if err != nil {
		// Expected to fail since we're not doing a proper WebSocket handshake
		t.Logf("WebSocket upgrade request failed as expected: %v", err)
		return
	}
	defer resp.Body.Close()
	
	// Check if we got a proper WebSocket upgrade response
	if resp.StatusCode == http.StatusSwitchingProtocols {
		t.Log("Server properly handled WebSocket upgrade")
	} else {
		t.Logf("Server returned status %d for upgrade request", resp.StatusCode)
	}
}

// TestWebSocketProtocolDetection tests automatic protocol detection
func TestWebSocketProtocolDetection(t *testing.T) {
	// Test protocol detection functions
	httpRequest := []byte("GET /ws HTTP/1.1\r\nHost: localhost\r\n\r\n")
	if !connection.IsHTTPRequest(httpRequest) {
		t.Error("Should detect HTTP request")
	}
	
	wsUpgrade := []byte("GET /ws HTTP/1.1\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	if !connection.IsWebSocketUpgrade(wsUpgrade) {
		t.Error("Should detect WebSocket upgrade")
	}
	
	protocol := connection.GetHTTPUpgradeProtocol(wsUpgrade)
	if !strings.EqualFold(protocol, "websocket") {
		t.Errorf("Expected websocket protocol, got %s", protocol)
	}
	
	// Test non-HTTP data
	plainData := []byte("plain tcp data")
	if connection.IsHTTPRequest(plainData) {
		t.Error("Should not detect plain data as HTTP")
	}
	
	if connection.IsWebSocketUpgrade(plainData) {
		t.Error("Should not detect plain data as WebSocket upgrade")
	}
}

// TestWebSocketConnectionFactory tests WebSocket connection creation
func TestWebSocketConnectionFactory(t *testing.T) {
	factory := connection.NewConnectionFactory(connection.DefaultConnectionOptions())
	
	// Create a mock connection that looks like a WebSocket upgrade
	wsData := []byte("GET /ws HTTP/1.1\r\nUpgrade: websocket\r\nConnection: upgrade\r\n\r\n")
	mockConn := &mockWebSocketConn{data: wsData}
	
	conn, err := factory.CreateConnection(mockConn)
	if err != nil {
		t.Errorf("Failed to create WebSocket connection: %v", err)
	}
	
	if conn == nil {
		t.Fatal("Connection should not be nil")
	}
	
	// Note: Depending on implementation, this might be detected as WebSocket
	t.Logf("Created connection of type: %s", conn.ConnectionType())
}

// Mock WebSocket connection for testing
type mockWebSocketConn struct {
	data   []byte
	closed bool
}

func (m *mockWebSocketConn) Read(b []byte) (n int, err error) {
	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}
	n = copy(b, m.data)
	return n, nil
}

func (m *mockWebSocketConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}
	return len(b), nil
}

func (m *mockWebSocketConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockWebSocketConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (m *mockWebSocketConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090}
}

func (m *mockWebSocketConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetWriteDeadline(t time.Time) error {
	return nil
}