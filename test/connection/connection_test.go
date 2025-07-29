package connection_test

import (
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/connection"
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

func TestConnectionTypes(t *testing.T) {
	// Test connection.ConnectionType constants
	if connection.ConnectionTypeTCP != "tcp" {
		t.Errorf("Expected TCP type to be 'tcp', got %s", connection.ConnectionTypeTCP)
	}
	
	if connection.ConnectionTypeWebSocket != "websocket" {
		t.Errorf("Expected WebSocket type to be 'websocket', got %s", connection.ConnectionTypeWebSocket)
	}
}

func TestDefaultConnectionOptions(t *testing.T) {
	opts := connection.DefaultConnectionOptions()
	
	if opts.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", opts.ReadTimeout)
	}
	
	if opts.WriteTimeout != 10*time.Second {
		t.Errorf("Expected write timeout 10s, got %v", opts.WriteTimeout)
	}
	
	if opts.ReadBufferSize != 4096 {
		t.Errorf("Expected read buffer size 4096, got %d", opts.ReadBufferSize)
	}
	
	if opts.WriteBufferSize != 4096 {
		t.Errorf("Expected write buffer size 4096, got %d", opts.WriteBufferSize)
	}
	
	if opts.MaxMessageSize != 1024*1024 {
		t.Errorf("Expected max message size 1MB, got %d", opts.MaxMessageSize)
	}
}

func TestConnectionTypeFromString(t *testing.T) {
	tests := map[string]connection.ConnectionType{
		"tcp":       connection.ConnectionTypeTCP,
		"TCP":       connection.ConnectionTypeTCP,
		"websocket": connection.ConnectionTypeWebSocket,
		"WEBSOCKET": connection.ConnectionTypeWebSocket,
		"ws":        connection.ConnectionTypeWebSocket,
		"WS":        connection.ConnectionTypeWebSocket,
	}
	
	for input, expected := range tests {
		result, err := connection.ConnectionTypeFromString(input)
		if err != nil {
			t.Errorf("Unexpected error for input %s: %v", input, err)
		}
		if result != expected {
			t.Errorf("For input %s, expected %s, got %s", input, expected, result)
		}
	}
	
	// Test invalid input
	_, err := connection.ConnectionTypeFromString("invalid")
	if err == nil {
		t.Error("Expected error for invalid connection type")
	}
}

func TestConnectionEventTypes(t *testing.T) {
	events := []connection.ConnectionEventType{
		connection.EventConnectionOpened,
		connection.EventConnectionClosed,
		connection.EventMessageReceived,
		connection.EventMessageSent,
		connection.EventConnectionError,
		connection.EventConnectionTimeout,
	}
	
	for _, event := range events {
		if string(event) == "" {
			t.Errorf("Event type should not be empty: %v", event)
		}
	}
}

// Helper function to create a mock net.Conn for testing
type mockConn struct {
	data   []byte
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	n = copy(b, m.data)
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.closed {
		return 0, net.ErrClosed
	}
	m.data = append(m.data, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func createMockConnection() net.Conn {
	return &mockConn{}
}

func TestTCPConnection(t *testing.T) {
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	
	// Test connection type
	if conn.ConnectionType() != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP connection type, got %s", conn.ConnectionType())
	}
	
	// Test initial state
	if !conn.IsConnected() {
		t.Error("Connection should be connected initially")
	}
	
	// Test address methods
	remoteAddr := conn.RemoteAddrWithProtocol()
	if remoteAddr != "tcp://127.0.0.1:9090" {
		t.Errorf("Expected remote address tcp://127.0.0.1:9090, got %s", remoteAddr)
	}
	
	localAddr := conn.LocalAddrWithProtocol()
	if localAddr != "tcp://127.0.0.1:8080" {
		t.Errorf("Expected local address tcp://127.0.0.1:8080, got %s", localAddr)
	}
	
	// Test timeout methods
	if err := conn.SetReadTimeout(5 * time.Second); err != nil {
		t.Errorf("Unexpected error setting read timeout: %v", err)
	}
	
	if err := conn.SetWriteTimeout(3 * time.Second); err != nil {
		t.Errorf("Unexpected error setting write timeout: %v", err)
	}
	
	// Test context
	ctx := conn.Context()
	if ctx == nil {
		t.Error("Connection context should not be nil")
	}
	
	// Test close
	if err := conn.Close(); err != nil {
		t.Errorf("Unexpected error closing connection: %v", err)
	}
	
	if conn.IsConnected() {
		t.Error("Connection should not be connected after close")
	}
}

func TestWebSocketConnection(t *testing.T) {
	mockNet := createMockConnection()
	conn := connection.NewWebSocketConnection(mockNet, nil)
	
	// Test connection type
	if conn.ConnectionType() != connection.ConnectionTypeWebSocket {
		t.Errorf("Expected WebSocket connection type, got %s", conn.ConnectionType())
	}
	
	// Test initial state
	if !conn.IsConnected() {
		t.Error("Connection should be connected initially")
	}
	
	// Test address methods
	remoteAddr := conn.RemoteAddrWithProtocol()
	if remoteAddr != "websocket://127.0.0.1:9090" {
		t.Errorf("Expected remote address websocket://127.0.0.1:9090, got %s", remoteAddr)
	}
	
	localAddr := conn.LocalAddrWithProtocol()
	if localAddr != "websocket://127.0.0.1:8080" {
		t.Errorf("Expected local address websocket://127.0.0.1:8080, got %s", localAddr)
	}
	
	// Test close
	if err := conn.Close(); err != nil {
		t.Errorf("Unexpected error closing connection: %v", err)
	}
	
	if conn.IsConnected() {
		t.Error("Connection should not be connected after close")
	}
}

func TestConnectionFactory(t *testing.T) {
	factory := connection.NewConnectionFactory(nil)
	
	// Test TCP connection creation
	mockNet := createMockConnection()
	tcpConn := factory.CreateTCPConnection(mockNet)
	
	if tcpConn.ConnectionType() != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP connection, got %s", tcpConn.ConnectionType())
	}
	
	// Test WebSocket connection creation
	wsConn, err := factory.CreateWebSocketConnection(mockNet)
	if err != nil {
		t.Errorf("Unexpected error creating WebSocket connection: %v", err)
	}
	
	if wsConn.ConnectionType() != connection.ConnectionTypeWebSocket {
		t.Errorf("Expected WebSocket connection, got %s", wsConn.ConnectionType())
	}
	
	// Test invalid WebSocket connection
	_, err = factory.CreateWebSocketConnection("invalid")
	if err == nil {
		t.Error("Expected error for invalid WebSocket connection")
	}
}

func TestClientFactory(t *testing.T) {
	factory := connection.NewClientFactory(nil)
	
	// Test address parsing
	testCases := []struct {
		address     string
		expectError bool
	}{
		{"localhost:8080", false},       // TCP
		{"ws://localhost:8080/ws", true}, // WebSocket (will fail without real server)
		{"127.0.0.1:9090", false},       // TCP
	}
	
	for _, tc := range testCases {
		_, err := factory.Connect(tc.address)
		if tc.expectError && err == nil {
			t.Errorf("Expected error for address %s", tc.address)
		}
		if !tc.expectError && err == nil {
			// Connection succeeded, but we should close it
			// Since we're using mock connections, this won't actually connect
		}
	}
}

func TestProtocolDetection(t *testing.T) {
	// Test HTTP request detection
	httpData := []byte("GET /path HTTP/1.1\r\nHost: example.com\r\n\r\n")
	if !connection.IsHTTPRequest(httpData) {
		t.Error("Should detect HTTP GET request")
	}
	
	postData := []byte("POST /api HTTP/1.1\r\nContent-Type: application/json\r\n\r\n")
	if !connection.IsHTTPRequest(postData) {
		t.Error("Should detect HTTP POST request")
	}
	
	nonHTTPData := []byte("Hello, this is not HTTP")
	if connection.IsHTTPRequest(nonHTTPData) {
		t.Error("Should not detect non-HTTP data as HTTP")
	}
	
	// Test WebSocket upgrade detection
	wsUpgradeData := []byte("GET /ws HTTP/1.1\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	if !connection.IsWebSocketUpgrade(wsUpgradeData) {
		t.Error("Should detect WebSocket upgrade request")
	}
	
	regularHTTPData := []byte("GET /page HTTP/1.1\r\nHost: example.com\r\n\r\n")
	if connection.IsWebSocketUpgrade(regularHTTPData) {
		t.Error("Should not detect regular HTTP as WebSocket upgrade")
	}
	
	// Test upgrade protocol extraction
	protocol := connection.GetHTTPUpgradeProtocol(wsUpgradeData)
	if protocol != "websocket" {
		t.Errorf("Expected upgrade protocol 'websocket', got '%s'", protocol)
	}
}

func TestConnectionManager(t *testing.T) {
	// Test with nil event handler first
	manager := connection.NewConnectionManager(nil, nil)
	
	// Test initial state
	if manager.ConnectionCount() != 0 {
		t.Errorf("Expected 0 connections initially, got %d", manager.ConnectionCount())
	}
	
	// Test adding connections
	mockNet1 := createMockConnection()
	conn1 := connection.NewTCPConnection(mockNet1, nil)
	
	err := manager.AddConnection("conn1", conn1)
	if err != nil {
		t.Errorf("Unexpected error adding connection: %v", err)
	}
	
	if manager.ConnectionCount() != 1 {
		t.Errorf("Expected 1 connection after adding, got %d", manager.ConnectionCount())
	}
	
	// Test duplicate connection ID
	err = manager.AddConnection("conn1", conn1)
	if err == nil {
		t.Error("Expected error when adding duplicate connection ID")
	}
	
	// Test getting connection
	retrievedConn, exists := manager.GetConnection("conn1")
	if !exists {
		t.Error("Connection should exist")
	}
	if retrievedConn != conn1 {
		t.Error("Retrieved connection should match added connection")
	}
	
	// Test connection info
	info, exists := manager.GetConnectionInfo("conn1")
	if !exists {
		t.Error("Connection info should exist")
	}
	if info.Type != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP connection type in info, got %s", info.Type)
	}
	
	// Test listing connections
	connections := manager.ListConnections()
	if len(connections) != 1 {
		t.Errorf("Expected 1 connection in list, got %d", len(connections))
	}
	if connections[0] != "conn1" {
		t.Errorf("Expected connection ID 'conn1', got '%s'", connections[0])
	}
	
	// Test connection counts by type
	counts := manager.ConnectionCountByType()
	if counts[connection.ConnectionTypeTCP] != 1 {
		t.Errorf("Expected 1 TCP connection, got %d", counts[connection.ConnectionTypeTCP])
	}
	
	// Test broadcast
	testMessage := []byte("test message")
	err = manager.Broadcast(testMessage)
	if err != nil {
		t.Errorf("Unexpected error broadcasting: %v", err)
	}
	
	// Test send message to specific connection
	err = manager.SendMessage("conn1", testMessage)
	if err != nil {
		t.Errorf("Unexpected error sending message: %v", err)
	}
	
	// Test send message to non-existent connection
	err = manager.SendMessage("nonexistent", testMessage)
	if err == nil {
		t.Error("Expected error sending to non-existent connection")
	}
	
	// Test removing connection
	err = manager.RemoveConnection("conn1")
	if err != nil {
		t.Errorf("Unexpected error removing connection: %v", err)
	}
	
	if manager.ConnectionCount() != 0 {
		t.Errorf("Expected 0 connections after removal, got %d", manager.ConnectionCount())
	}
	
	// Test removing non-existent connection
	err = manager.RemoveConnection("nonexistent")
	if err == nil {
		t.Error("Expected error removing non-existent connection")
	}
	
	// Test close
	err = manager.Close()
	if err != nil {
		t.Errorf("Unexpected error closing manager: %v", err)
	}
}

func TestConnectionManagerWithEvents(t *testing.T) {
	var eventCount int64
	var lastEvent *connection.ConnectionEvent
	var lastEventMutex sync.RWMutex
	
	eventHandler := func(event *connection.ConnectionEvent) {
		atomic.AddInt64(&eventCount, 1)
		lastEventMutex.Lock()
		lastEvent = event
		lastEventMutex.Unlock()
	}
	
	manager := connection.NewConnectionManager(nil, eventHandler)
	
	// Add a connection and check for event
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	
	manager.AddConnection("test", conn)
	
	// Give event handler time to run
	time.Sleep(10 * time.Millisecond)
	
	finalEventCount := atomic.LoadInt64(&eventCount)
	if finalEventCount == 0 {
		t.Error("Expected connection opened event")
	}
	
	lastEventMutex.RLock()
	currentLastEvent := lastEvent
	lastEventMutex.RUnlock()
	
	if currentLastEvent.Type != connection.EventConnectionOpened {
		t.Errorf("Expected connection opened event, got %s", currentLastEvent.Type)
	}
	
	// Test broadcast with events
	testMessage := []byte("test")
	manager.Broadcast(testMessage)
	
	// Give event handler time to run
	time.Sleep(10 * time.Millisecond)
	
	lastEventMutex.RLock()
	currentLastEvent2 := lastEvent
	lastEventMutex.RUnlock()
	
	if currentLastEvent2.Type != connection.EventMessageSent {
		t.Errorf("Expected message sent event, got %s", currentLastEvent2.Type)
	}
}

func TestTCPConnectionMessageOperations(t *testing.T) {
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, connection.DefaultConnectionOptions())
	defer conn.Close()
	
	// Test WriteMessage
	testMsg := []byte("Hello, World!")
	err := conn.WriteMessage(testMsg)
	if err != nil {
		t.Errorf("Unexpected error writing message: %v", err)
	}
	
	// Test connection info
	info := conn.GetConnectionInfo()
	if info == nil {
		t.Error("Connection info should not be nil")
	}
	
	if info.Type != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP connection type, got %s", info.Type)
	}
	
	if info.MessagesWritten != 1 {
		t.Errorf("Expected 1 message written, got %d", info.MessagesWritten)
	}
}

func TestWebSocketConnectionMessageOperations(t *testing.T) {
	mockNet := createMockConnection()
	conn := connection.NewWebSocketConnection(mockNet, connection.DefaultConnectionOptions())
	defer conn.Close()
	
	// Test WriteMessage
	testMsg := []byte("WebSocket Message")
	err := conn.WriteMessage(testMsg)
	if err != nil {
		t.Errorf("Unexpected error writing message: %v", err)
	}
	
	// Test connection info
	info := conn.GetConnectionInfo()
	if info == nil {
		t.Error("Connection info should not be nil")
	}
	
	if info.Type != connection.ConnectionTypeWebSocket {
		t.Errorf("Expected WebSocket connection type, got %s", info.Type)
	}
}

// Enhanced mock connection with more realistic behavior
type enhancedMockConn struct {
	*mockConn
	readData  []byte
	writeData []byte
	readPos   int
}

func newEnhancedMockConn(data []byte) *enhancedMockConn {
	return &enhancedMockConn{
		mockConn: &mockConn{},
		readData: data,
		writeData: make([]byte, 0),
	}
}

func (e *enhancedMockConn) Read(b []byte) (n int, err error) {
	if e.closed {
		return 0, net.ErrClosed
	}
	
	if e.readPos >= len(e.readData) {
		return 0, nil // EOF-like behavior
	}
	
	n = copy(b, e.readData[e.readPos:])
	e.readPos += n
	return n, nil
}

func (e *enhancedMockConn) Write(b []byte) (n int, err error) {
	if e.closed {
		return 0, net.ErrClosed
	}
	
	e.writeData = append(e.writeData, b...)
	return len(b), nil
}

func TestTCPConnectionReadMessage(t *testing.T) {
	testData := []byte("test message\n")
	enhancedMock := newEnhancedMockConn(testData)
	conn := connection.NewTCPConnection(enhancedMock, connection.DefaultConnectionOptions())
	defer conn.Close()
	
	// Test ReadMessage
	message, err := conn.ReadMessage()
	if err != nil {
		t.Errorf("Unexpected error reading message: %v", err)
	}
	
	expected := "test message"
	if string(message) != expected {
		t.Errorf("Expected message '%s', got '%s'", expected, string(message))
	}
}

func TestConnectionManagerBroadcastToType(t *testing.T) {
	manager := connection.NewConnectionManager(nil, nil)
	defer manager.Close()
	
	// Add TCP connection
	tcpMock := createMockConnection()
	tcpConn := connection.NewTCPConnection(tcpMock, nil)
	manager.AddConnection("tcp1", tcpConn)
	
	// Add WebSocket connection
	wsMock := createMockConnection()
	wsConn := connection.NewWebSocketConnection(wsMock, nil)
	manager.AddConnection("ws1", wsConn)
	
	// Test broadcast to TCP only
	testMessage := []byte("tcp only")
	err := manager.BroadcastToType(connection.ConnectionTypeTCP, testMessage)
	if err != nil {
		t.Errorf("Unexpected error broadcasting to TCP: %v", err)
	}
	
	// Test broadcast to WebSocket only
	err = manager.BroadcastToType(connection.ConnectionTypeWebSocket, testMessage)
	if err != nil {
		t.Errorf("Unexpected error broadcasting to WebSocket: %v", err)
	}
	
	// Test GetConnectionsByType
	tcpConnections := manager.GetConnectionsByType(connection.ConnectionTypeTCP)
	if len(tcpConnections) != 1 {
		t.Errorf("Expected 1 TCP connection, got %d", len(tcpConnections))
	}
	
	wsConnections := manager.GetConnectionsByType(connection.ConnectionTypeWebSocket)
	if len(wsConnections) != 1 {
		t.Errorf("Expected 1 WebSocket connection, got %d", len(wsConnections))
	}
}

func TestConnectionManagerUpdateConnectionInfo(t *testing.T) {
	manager := connection.NewConnectionManager(nil, nil)
	defer manager.Close()
	
	// Add connection
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	manager.AddConnection("test", conn)
	
	// Update connection info
	manager.UpdateConnectionInfo("test")
	
	// Test updating non-existent connection
	manager.UpdateConnectionInfo("nonexistent")
	
	// Get updated info
	info, exists := manager.GetConnectionInfo("test")
	if !exists {
		t.Error("Connection info should exist after update")
	}
	
	if info.Type != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP connection type, got %s", info.Type)
	}
}

func TestConnectionFactoryCreateConnection(t *testing.T) {
	factory := connection.NewConnectionFactory(connection.DefaultConnectionOptions())
	
	// Test with TCP-like connection (no HTTP headers)
	plainMock := newEnhancedMockConn([]byte("plain tcp data"))
	conn, err := factory.CreateConnection(plainMock)
	if err != nil {
		t.Errorf("Unexpected error creating connection: %v", err)
	}
	
	if conn.ConnectionType() != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP connection, got %s", conn.ConnectionType())
	}
	
	// Test with HTTP-like connection
	httpData := []byte("GET /ws HTTP/1.1\r\nUpgrade: websocket\r\nConnection: upgrade\r\n\r\n")
	httpMock := newEnhancedMockConn(httpData)
	conn, err = factory.CreateConnection(httpMock)
	if err != nil {
		t.Errorf("Unexpected error creating HTTP connection: %v", err)
	}
	
	// Note: This might be detected as WebSocket due to upgrade headers
	if conn == nil {
		t.Error("Connection should not be nil")
	}
}

func TestServerFactory(t *testing.T) {
	factory := connection.NewServerFactory(connection.DefaultConnectionOptions())
	
	// Test HandleConnection with mock connection
	mockNet := createMockConnection()
	conn, err := factory.HandleConnection(mockNet)
	if err != nil {
		t.Errorf("Unexpected error handling connection: %v", err)
	}
	
	if conn == nil {
		t.Error("Connection should not be nil")
	}
}

func TestConnectionOptionsEdgeCases(t *testing.T) {
	// Test with nil options
	conn := connection.NewTCPConnection(createMockConnection(), nil)
	if !conn.IsConnected() {
		t.Error("Connection should be connected with nil options")
	}
	conn.Close()
	
	// Test with custom options
	opts := &connection.ConnectionOptions{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 3 * time.Second,
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
		MaxMessageSize: 512,
	}
	
	conn = connection.NewTCPConnection(createMockConnection(), opts)
	if !conn.IsConnected() {
		t.Error("Connection should be connected with custom options")
	}
	conn.Close()
}

func TestConnectionErrorHandling(t *testing.T) {
	// Test operations on closed connection
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	
	// Close the connection
	conn.Close()
	
	// Test operations on closed connection
	err := conn.WriteMessage([]byte("test"))
	if err == nil {
		t.Error("Expected error writing to closed connection")
	}
	
	_, err = conn.ReadMessage()
	if err == nil {
		t.Error("Expected error reading from closed connection")
	}
	
	// Test double close
	err = conn.Close()
	if err != nil {
		t.Errorf("Double close should not return error, got: %v", err)
	}
}

// Broken mock connection for testing error conditions
type brokenMockConn struct {
	*mockConn
	readError  error
	writeError error
}

func (b *brokenMockConn) Read(p []byte) (n int, err error) {
	if b.readError != nil {
		return 0, b.readError
	}
	return b.mockConn.Read(p)
}

func (b *brokenMockConn) Write(p []byte) (n int, err error) {
	if b.writeError != nil {
		return 0, b.writeError
	}
	return b.mockConn.Write(p)
}

func TestConnectionErrors(t *testing.T) {
	// Test TCP connection with read error
	brokenConn := &brokenMockConn{
		mockConn:  &mockConn{},
		readError: net.ErrClosed,
	}
	
	tcpConn := connection.NewTCPConnection(brokenConn, nil)
	_, err := tcpConn.ReadMessage()
	if err == nil {
		t.Error("Expected error reading from broken connection")
	}
	
	// Test WebSocket connection with write error
	brokenConn2 := &brokenMockConn{
		mockConn:   &mockConn{},
		writeError: net.ErrClosed,
	}
	
	wsConn := connection.NewWebSocketConnection(brokenConn2, nil)
	err = wsConn.WriteMessage([]byte("test"))
	if err == nil {
		t.Error("Expected error writing to broken connection")
	}
}

func TestFactoryDetectionEdgeCases(t *testing.T) {
	factory := connection.NewConnectionFactory(nil)
	
	// Test with error during detection
	brokenConn := &brokenMockConn{
		mockConn:  &mockConn{},
		readError: net.ErrClosed,
	}
	
	// Should still create a connection (defaults to TCP on error)
	conn, err := factory.CreateConnection(brokenConn)
	if err != nil {
		t.Errorf("Unexpected error with broken connection: %v", err)
	}
	
	if conn.ConnectionType() != connection.ConnectionTypeTCP {
		t.Errorf("Expected TCP fallback, got %s", conn.ConnectionType())
	}
}

func TestConnectionManagerErrorScenarios(t *testing.T) {
	manager := connection.NewConnectionManager(nil, nil)
	defer manager.Close()
	
	// Add a broken connection
	brokenConn := &brokenMockConn{
		mockConn:   &mockConn{},
		writeError: net.ErrClosed,
	}
	
	conn := connection.NewTCPConnection(brokenConn, nil)
	manager.AddConnection("broken", conn)
	
	// Test broadcast with broken connection (should handle error gracefully)
	err := manager.Broadcast([]byte("test"))
	if err == nil {
		t.Error("Expected error broadcasting to broken connection")
	}
	
	// Test send to broken connection
	err = manager.SendMessage("broken", []byte("test"))
	if err == nil {
		t.Error("Expected error sending to broken connection")
	}
	
	// Test broadcast to type with no connections of that type
	// This should succeed but do nothing (empty broadcast)
	err = manager.BroadcastToType("nonexistent-type", []byte("test"))
	if err != nil {
		t.Errorf("Unexpected error broadcasting to non-existent type: %v", err)
	}
}

func TestWebSocketReadMessage(t *testing.T) {
	// Test WebSocket ReadMessage with larger data
	testData := []byte("large websocket message data")
	enhancedMock := newEnhancedMockConn(testData)
	conn := connection.NewWebSocketConnection(enhancedMock, connection.DefaultConnectionOptions())
	defer conn.Close()
	
	message, err := conn.ReadMessage()
	if err != nil {
		t.Errorf("Unexpected error reading WebSocket message: %v", err)
	}
	
	if string(message) != string(testData) {
		t.Errorf("Expected message '%s', got '%s'", string(testData), string(message))
	}
}

func TestConnectionInfoTimestamps(t *testing.T) {
	manager := connection.NewConnectionManager(connection.DefaultConnectionOptions(), nil)
	defer manager.Close()
	
	// Add connection
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	
	startTime := time.Now()
	err := manager.AddConnection("test", conn)
	if err != nil {
		t.Errorf("Error adding connection: %v", err)
	}
	
	// Get connection info and check timestamps
	info, exists := manager.GetConnectionInfo("test")
	if !exists {
		t.Error("Connection info should exist")
	}
	
	// ConnectedAt should be recent
	if info.ConnectedAt.Before(startTime) {
		t.Error("ConnectedAt should be after start time")
	}
	
	if time.Since(info.ConnectedAt) > time.Second {
		t.Error("ConnectedAt should be very recent")
	}
}

func TestConnectionReadWrite(t *testing.T) {
	// Test basic Read/Write operations (net.Conn interface)
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	defer conn.Close()
	
	// Test Write via net.Conn interface
	testData := []byte("direct write test")
	n, err := conn.Write(testData)
	if err != nil {
		t.Errorf("Unexpected error in Write: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(testData), n)
	}
	
	// Test Read via net.Conn interface 
	readBuf := make([]byte, 100)
	n, err = conn.Read(readBuf)
	if err != nil {
		t.Errorf("Unexpected error in Read: %v", err)
	}
	if n == 0 {
		t.Error("Should have read some data")
	}
}

func TestConnectionDeadlines(t *testing.T) {
	// Test deadline operations
	mockNet := createMockConnection()
	conn := connection.NewTCPConnection(mockNet, nil)
	defer conn.Close()
	
	// Test SetDeadline
	deadline := time.Now().Add(time.Minute)
	err := conn.SetDeadline(deadline)
	if err != nil {
		t.Errorf("Unexpected error setting deadline: %v", err)
	}
	
	// Test SetReadDeadline
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		t.Errorf("Unexpected error setting read deadline: %v", err)
	}
	
	// Test SetWriteDeadline
	err = conn.SetWriteDeadline(deadline)
	if err != nil {
		t.Errorf("Unexpected error setting write deadline: %v", err)
	}
}

func TestWebSocketDeadlines(t *testing.T) {
	// Test WebSocket deadline operations
	mockNet := createMockConnection()
	conn := connection.NewWebSocketConnection(mockNet, nil)
	defer conn.Close()
	
	deadline := time.Now().Add(time.Minute)
	
	// Test all deadline methods
	err := conn.SetDeadline(deadline)
	if err != nil {
		t.Errorf("Unexpected error setting WebSocket deadline: %v", err)
	}
	
	err = conn.SetReadDeadline(deadline)
	if err != nil {
		t.Errorf("Unexpected error setting WebSocket read deadline: %v", err)
	}
	
	err = conn.SetWriteDeadline(deadline)
	if err != nil {
		t.Errorf("Unexpected error setting WebSocket write deadline: %v", err)
	}
}

func TestConnectionAddresses(t *testing.T) {
	// Test LocalAddr and RemoteAddr for both connection types
	mockNet := createMockConnection()
	
	// Test TCP connection addresses
	tcpConn := connection.NewTCPConnection(mockNet, nil)
	defer tcpConn.Close()
	
	localAddr := tcpConn.LocalAddr()
	if localAddr == nil {
		t.Error("TCP LocalAddr should not be nil")
	}
	
	remoteAddr := tcpConn.RemoteAddr()
	if remoteAddr == nil {
		t.Error("TCP RemoteAddr should not be nil")
	}
	
	// Test WebSocket connection addresses
	wsConn := connection.NewWebSocketConnection(mockNet, nil)
	defer wsConn.Close()
	
	localAddr = wsConn.LocalAddr()
	if localAddr == nil {
		t.Error("WebSocket LocalAddr should not be nil")
	}
	
	remoteAddr = wsConn.RemoteAddr()
	if remoteAddr == nil {
		t.Error("WebSocket RemoteAddr should not be nil")
	}
}