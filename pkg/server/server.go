package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	websocks "github.com/catalystcommunity/websocks/v1"
	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/ids"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
)

// isExpectedDisconnectionError checks if an error is expected during normal disconnection
func isExpectedDisconnectionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// Common expected disconnection errors
	expectedErrors := []string{
		"eof",
		"connection reset by peer",
		"broken pipe",
		"use of closed network connection",
		"websocket connection closed",
		"connection closed by remote peer",
		"connection is closed",
		"connection timed out",
		"is not connected",
		"not found",
		"write: broken pipe",
		"read: connection reset by peer",
		"websocket: close",
		"context deadline exceeded",
		"operation was cancelled",
		"no such file or directory",
		"protocol error",
		"abnormal closure",
		"going away",
		"remote error",
		"connection refused",
		"network is unreachable",
		"timeout",
		"client disconnected",
		"connection closed",
		"connection aborted",
		"socket not connected",
		"write tcp",
		"read tcp",
		"i/o timeout",
		"operation timed out",
		"forcibly closed",
		"connection forcibly closed",
		"network connection aborted",
		"software caused connection abort",
		"connection reset",
		"closed by peer",
		"closed network connection",
		"graceful shutdown",
		"shutdown: socket not connected",
	}
	
	for _, expected := range expectedErrors {
		if strings.Contains(errStr, expected) {
			return true
		}
	}
	
	// Check for io.EOF specifically
	if err == io.EOF {
		return true
	}
	
	return false
}

// ServerConfig holds configuration options for the server
type ServerConfig struct {
	Host                    string
	Port                    string
	WebSocketPort          string // Optional WebSocket port
	MaxConnections         int
	ConnectionTimeout      time.Duration
	ReadTimeout            time.Duration
	WriteTimeout           time.Duration
	HeartbeatInterval      time.Duration
	ClientInactivityTimeout time.Duration
	MaxRooms               int
	MaxClientsPerRoom      int
	EnableWebSocket        bool // Enable WebSocket server
	
	// Rate limiting configuration
	RateLimitEnabled       bool
	MaxConnectionsPerIP    int
	RateLimitWindow        time.Duration
	MaxConnectionsPerSecond int
}

// Represents a client connection with enhanced session management
type ConnClient struct {
	Id         string
	Name       string
	Conn       connection.GameConnection
	AckManager *message.AckManager
	State      ClientState
	ConnectedAt time.Time
	LastActivity time.Time
	Rooms      map[string]bool
	mutex      sync.RWMutex
}

// Room represents a channel/room for targeted messaging
type Room struct {
	ID          string
	Name        string
	Description string
	Clients     map[string]*ConnClient
	CreatedAt   time.Time
	mutex       sync.RWMutex
}

// ClientState represents the current state of a client connection
type ClientState int

const (
	ClientStateConnecting ClientState = iota
	ClientStateConnected
	ClientStateAuthenticated
	ClientStateDisconnected
)

// Represents a Server Connection, of which we could have several
// with different ports or encodings, like TCP vs websocket server
type ConnServer struct {
	config            *ServerConfig
	clients           map[string]*ConnClient
	rooms             map[string]*Room
	connManager       connection.ConnectionManager
	connFactory       connection.ConnectionFactory
	msgRouter         *message.MessageRouter
	eventManager      events.EventManager
	listener          net.Listener
	wsServer          *http.Server  // WebSocket server
	ctx               context.Context
	cancel            context.CancelFunc
	mutex             sync.RWMutex
	shutdownComplete  chan struct{}
	logger            *logging.Logger
	
	// Rate limiting
	ipConnections     map[string]int // Track connections per IP
	connectionTimes   []time.Time    // Track connection times for rate limiting
	rateLimitMutex    sync.RWMutex
}

// DefaultServerConfig returns a server configuration with sensible defaults
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:                    "localhost",
		Port:                    "7777",
		WebSocketPort:          "7778",
		MaxConnections:         1000,
		ConnectionTimeout:      30 * time.Second,
		ReadTimeout:            5 * time.Minute,
		WriteTimeout:           30 * time.Second,
		HeartbeatInterval:      30 * time.Second,
		ClientInactivityTimeout: 10 * time.Minute,
		MaxRooms:               100,
		MaxClientsPerRoom:      100,
		EnableWebSocket:        false, // Disabled by default
		
		// Rate limiting defaults
		RateLimitEnabled:       true,
		MaxConnectionsPerIP:    10,
		RateLimitWindow:        1 * time.Minute,
		MaxConnectionsPerSecond: 50,
	}
}

func NewServer(host, port string) *ConnServer {
	config := DefaultServerConfig()
	if host != "" {
		config.Host = host
	}
	if port != "" {
		config.Port = port
	}
	return NewServerWithConfig(config)
}

// NewWebSocketServer creates a server with WebSocket support enabled
func NewWebSocketServer(host, tcpPort, wsPort string) *ConnServer {
	config := DefaultServerConfig()
	if host != "" {
		config.Host = host
	}
	if tcpPort != "" {
		config.Port = tcpPort
	}
	if wsPort != "" {
		config.WebSocketPort = wsPort
	}
	config.EnableWebSocket = true
	return NewServerWithConfig(config)
}

// NewTCPAndWebSocketServer creates a server with both TCP and WebSocket support
func NewTCPAndWebSocketServer(host, tcpPort, wsPort string) *ConnServer {
	return NewWebSocketServer(host, tcpPort, wsPort)
}

func NewServerWithConfig(config *ServerConfig) *ConnServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	s := &ConnServer{
		config:           config,
		clients:          make(map[string]*ConnClient),
		rooms:            make(map[string]*Room),
		connFactory:      connection.NewConnectionFactory(connection.DefaultConnectionOptions()),
		msgRouter:        message.NewMessageRouter(),
		eventManager:     events.NewEventManager(),
		ctx:              ctx,
		cancel:           cancel,
		shutdownComplete: make(chan struct{}),
		logger:           logging.GetDefaultLogger().WithComponent("server"),
		
		// Rate limiting initialization
		ipConnections:    make(map[string]int),
		connectionTimes:  make([]time.Time, 0),
	}
	
	// Create connection manager with event handler
	s.connManager = connection.NewConnectionManager(
		connection.DefaultConnectionOptions(),
		s.handleConnectionEvent,
	)
	
	// Register default message handlers
	s.registerDefaultHandlers()
	
	// Register event hooks
	s.registerEventHooks()
	
	return s
}

// StartServer starts the server and begins accepting connections
func (s *ConnServer) StartServer() error {
	var err error
	
	// Start TCP server
	s.listener, err = net.Listen("tcp", s.config.Host+":"+s.config.Port)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %v", err)
	}

	s.logger.Info("TCP Server started", "host", s.config.Host, "port", s.config.Port)

	// Start accepting TCP connections in a goroutine
	go s.acceptConnections()
	
	// Start WebSocket server if enabled
	if s.config.EnableWebSocket {
		err = s.startWebSocketServer()
		if err != nil {
			s.listener.Close()
			return fmt.Errorf("failed to start WebSocket server: %v", err)
		}
		s.logger.Info("WebSocket Server started", "host", s.config.Host, "port", s.config.WebSocketPort)
	}
	
	return nil
}

// acceptConnections accepts and handles incoming connections
func (s *ConnServer) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			rawConn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.ctx.Done():
					return // Server is shutting down
				default:
					s.logger.Error("Failed to accept connection", "error", err)
					continue
				}
			}
			
			go s.handleNewConnection(rawConn)
		}
	}
}

// handleNewConnection processes a new incoming connection
func (s *ConnServer) handleNewConnection(rawConn net.Conn) {
	// Check connection limit
	if s.GetClientCount() >= s.config.MaxConnections {
		s.logger.Warn("Connection rejected: max connections reached", 
			"current", s.GetClientCount(), "max", s.config.MaxConnections)
		rawConn.Close()
		return
	}
	
	// Rate limiting checks
	if s.config.RateLimitEnabled {
		clientIP := s.getClientIP(rawConn)
		
		// Check per-IP connection limit
		if s.checkIPLimit(clientIP) {
			s.logger.Warn("Connection rejected: too many connections from IP", 
				"ip", clientIP, "max_per_ip", s.config.MaxConnectionsPerIP)
			rawConn.Close()
			return
		}
		
		// Check global rate limit
		if s.checkGlobalRateLimit() {
			s.logger.Warn("Connection rejected: global rate limit exceeded", 
				"max_per_second", s.config.MaxConnectionsPerSecond)
			rawConn.Close()
			return
		}
		
		// Record this connection
		s.recordConnection(clientIP)
	}
	
	// Set connection timeout
	if s.config.ConnectionTimeout > 0 {
		rawConn.SetDeadline(time.Now().Add(s.config.ConnectionTimeout))
	}
	
	// Create GameConnection from raw connection
	gameConn, err := s.connFactory.CreateConnection(rawConn)
	if err != nil {
		s.logger.Error("Failed to create game connection", "error", err)
		rawConn.Close()
		return
	}
	
	// Generate client ID
	clientID := ids.RandStringRunes(12)
	
	// Create client session
	client := &ConnClient{
		Id:           clientID,
		Name:         "",
		Conn:         gameConn,
		AckManager:   message.NewAckManager(),
		State:        ClientStateConnecting,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		Rooms:        make(map[string]bool),
	}
	
	// Add to client map
	s.mutex.Lock()
	s.clients[clientID] = client
	s.mutex.Unlock()
	
	// Add to connection manager
	if err := s.connManager.AddConnection(clientID, gameConn); err != nil {
		s.logger.Error("Failed to add connection to manager", "client_id", clientID, "error", err)
		s.removeClient(clientID)
		return
	}
	
	// Set client state to connected
	client.mutex.Lock()
	client.State = ClientStateConnected
	client.mutex.Unlock()
	
	s.logger.Info("Client connected", 
		"client_id", clientID,
		"connection_type", gameConn.ConnectionType(),
		"remote_addr", gameConn.RemoteAddrWithProtocol())
	
	// Trigger connection open event
	connEvent := events.CreateConnectionOpenEvent(clientID, gameConn)
	s.eventManager.TriggerEventAsync(connEvent)
	
	// Start handling messages for this client
	go s.handleClientMessages(client)
}

// handleClientMessages handles incoming messages from a client
func (s *ConnServer) handleClientMessages(client *ConnClient) {
	defer func() {
		s.logger.Info("Client message handling ended", "client_id", client.Id)
		s.removeClient(client.Id)
	}()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-client.Conn.Context().Done():
			s.logger.Info("Client connection closed", "client_id", client.Id)
			return
		default:
			// Read message from client
			data, err := client.Conn.ReadMessage()
			if err != nil {
				// Check if this is an expected disconnection error
				if isExpectedDisconnectionError(err) {
					s.logger.Debug("Client disconnected", "client_id", client.Id, "reason", err)
				} else {
					s.logger.Error("Client read error", "client_id", client.Id, "error", err)
				}
				return
			}
			
			// Update last activity
			client.mutex.Lock()
			client.LastActivity = time.Now()
			client.mutex.Unlock()
			
			// Try to parse as CBOR message
			if msg, err := message.MessageUnwrap(data); err == nil {
				// Trigger message received event
				msgEvent := events.CreateMessageReceivedEvent(client.Id, msg)
				s.eventManager.TriggerEventAsync(msgEvent)
				
				// Handle CBOR message
				if err := s.handleMessage(client, msg); err != nil {
					s.logger.Error("Error handling message from client", "client_id", client.Id, "error", err)
				}
			} else {
				// Handle plain text message (for backwards compatibility)
				s.handlePlainText(client, data)
			}
		}
	}
}

// handleMessage processes a CBOR message using the message router
func (s *ConnServer) handleMessage(client *ConnClient, msg *message.Message) error {
	// Handle acknowledgment messages
	if msg.Type == message.MessageTypeAck || msg.Type == message.MessageTypeNack {
		if ackData, err := message.ParseAckMessage(msg); err == nil {
			client.AckManager.ProcessAcknowledgment(ackData)
		}
		return nil
	}
	
	var processingErr error
	
	// Handle room messages that require client context
	if msg.Type == message.MessageTypeJoinRoom {
		processingErr = s.handleJoinRoomMessage(client, msg)
	} else if msg.Type == message.MessageTypeLeaveRoom {
		processingErr = s.handleLeaveRoomMessage(client, msg)
	} else if contextHandler, hasContextHandler := contextHandlers[msg.Type]; hasContextHandler {
		// Handle with context-aware handler
		processingErr = contextHandler(client.Id, msg)
	} else {
		// Route message through message router
		if err := s.msgRouter.RouteMessage(msg); err != nil {
			// If no handler found, echo back for now (backwards compatibility)
			s.logger.Debug("No handler for message type, echoing back", "message_type", message.MessageTypeToString(msg.Type))
			processingErr = s.sendMessageToClient(client.Id, msg)
		}
	}
	
	// Send acknowledgment if required, after processing
	if msg.RequiresAck && msg.ID != "" {
		success := processingErr == nil
		errorMsg := ""
		if processingErr != nil {
			errorMsg = processingErr.Error()
		}
		
		ackMsg := message.CreateAckMessage(msg.ID, success, errorMsg)
		if ackMsg != nil {
			if err := s.sendMessageToClient(client.Id, ackMsg); err != nil {
				s.logger.Error("Failed to send ack to client", "client_id", client.Id, "error", err)
			}
		}
	}
	
	return processingErr
}

// handlePlainText handles plain text messages for backwards compatibility
func (s *ConnServer) handlePlainText(client *ConnClient, data []byte) {
	if err := client.Conn.WriteMessage(data); err != nil {
		if isExpectedDisconnectionError(err) {
			s.logger.Debug("Failed to write to client (connection closed)", "client_id", client.Id, "error", err)
		} else {
			s.logger.Error("Failed to write to client", "client_id", client.Id, "error", err)
		}
	}
}

// sendMessageToClient sends a message to a specific client
func (s *ConnServer) sendMessageToClient(clientID string, msg *message.Message) error {
	s.mutex.RLock()
	client, exists := s.clients[clientID]
	s.mutex.RUnlock()
	
	if !exists {
		return &message.MessageError{Type: msg.Type, Err: "client not found"}
	}
	
	// Set message ID if not set and requires acknowledgment
	if msg.RequiresAck && msg.ID == "" {
		msg.ID = message.GenerateMessageID()
	}
	
	// Track message for acknowledgment if needed
	if msg.RequiresAck {
		client.AckManager.TrackMessage(msg)
	}
	
	// Send message using connection manager
	data, err := message.MessageWrap(msg)
	if err != nil {
		return err
	}
	
	err = s.connManager.SendMessage(clientID, data)
	if err == nil {
		// Trigger message sent event
		msgEvent := events.CreateMessageSentEvent(clientID, msg)
		s.eventManager.TriggerEventAsync(msgEvent)
	}
	
	return err
}

// SendMessage sends a message to a client with acknowledgment tracking (public method)
func (s *ConnServer) SendMessage(clientID string, msg *message.Message) error {
	return s.sendMessageToClient(clientID, msg)
}

// removeClient removes a client and cleans up resources
func (s *ConnServer) removeClient(id string) {
	s.mutex.Lock()
	client, exists := s.clients[id]
	if exists {
		// Remove client from all rooms
		client.mutex.RLock()
		roomsToLeave := make([]string, 0, len(client.Rooms))
		for roomID := range client.Rooms {
			roomsToLeave = append(roomsToLeave, roomID)
		}
		client.mutex.RUnlock()
		
		// Remove from rooms (need to do this outside the client lock to avoid deadlock)
		for _, roomID := range roomsToLeave {
			if room, roomExists := s.rooms[roomID]; roomExists {
				room.mutex.Lock()
				delete(room.Clients, id)
				room.mutex.Unlock()
			}
		}
		
		delete(s.clients, id)
	}
	s.mutex.Unlock()
	
	if exists {
		// Close acknowledgment manager
		client.AckManager.Close()
		
		// Set client state to disconnected
		client.mutex.Lock()
		client.State = ClientStateDisconnected
		client.mutex.Unlock()
		
		// Close the connection - the monitor will handle cleanup automatically
		if conn, exists := s.connManager.GetConnection(id); exists {
			// Clean up IP connection tracking
			if s.config.RateLimitEnabled {
				clientIP := s.getClientIP(conn)
				s.cleanupIPConnection(clientIP)
			}
			conn.Close()
		}
		
		s.logger.Info("Client removed and cleaned up", "client_id", id)
	}
}

// registerDefaultHandlers registers default message handlers
func (s *ConnServer) registerDefaultHandlers() {
	// Register chat message handler
	s.msgRouter.RegisterHandler(message.MessageTypeChat, s.handleChatMessage)
	
	// Register heartbeat handler
	s.msgRouter.RegisterHandler(message.MessageTypeHeartbeat, s.handleHeartbeat)
	
	// Register connection handlers
	s.msgRouter.RegisterHandler(message.MessageTypeConnect, s.handleConnectMessage)
	s.msgRouter.RegisterHandler(message.MessageTypeDisconnect, s.handleDisconnectMessage)
}

// handleChatMessage handles chat messages by broadcasting them
func (s *ConnServer) handleChatMessage(msg *message.Message) error {
	// Broadcast to all connected clients
	data, err := message.MessageWrap(msg)
	if err != nil {
		return err
	}
	
	return s.connManager.Broadcast(data)
}

// handleHeartbeat handles heartbeat messages
func (s *ConnServer) handleHeartbeat(msg *message.Message) error {
	// Heartbeats don't need special handling, just acknowledge
	return nil
}

// handleConnectMessage handles connection messages
func (s *ConnServer) handleConnectMessage(msg *message.Message) error {
	s.logger.Info("Received connect message", "content", string(msg.Contents))
	return nil
}

// handleDisconnectMessage handles disconnect messages
func (s *ConnServer) handleDisconnectMessage(msg *message.Message) error {
	s.logger.Info("Received disconnect message", "content", string(msg.Contents))
	return nil
}

// handleJoinRoomMessage handles JOIN_ROOM messages
func (s *ConnServer) handleJoinRoomMessage(client *ConnClient, msg *message.Message) error {
	// Parse room data from message contents
	var roomData message.RoomData
	if err := cbor.Unmarshal(msg.Contents, &roomData); err != nil {
		s.logger.Error("Failed to parse room data from JOIN_ROOM message", "client_id", client.Id, "error", err)
		return fmt.Errorf("invalid room data: %v", err)
	}
	
	s.logger.Info("Processing JOIN_ROOM message", "client_id", client.Id, "room_id", roomData.RoomID)
	
	// Call JoinRoom method
	if err := s.JoinRoom(client.Id, roomData.RoomID); err != nil {
		s.logger.Error("Failed to join room", "client_id", client.Id, "room_id", roomData.RoomID, "error", err)
		return err
	}
	
	s.logger.Info("Client joined room", "client_id", client.Id, "room_id", roomData.RoomID)
	return nil
}

// handleLeaveRoomMessage handles LEAVE_ROOM messages
func (s *ConnServer) handleLeaveRoomMessage(client *ConnClient, msg *message.Message) error {
	// Parse room data from message contents
	var roomData message.RoomData
	if err := cbor.Unmarshal(msg.Contents, &roomData); err != nil {
		s.logger.Error("Failed to parse room data from LEAVE_ROOM message", "client_id", client.Id, "error", err)
		return fmt.Errorf("invalid room data: %v", err)
	}
	
	// Call LeaveRoom method
	if err := s.LeaveRoom(client.Id, roomData.RoomID); err != nil {
		s.logger.Error("Failed to leave room", "client_id", client.Id, "room_id", roomData.RoomID, "error", err)
		return err
	}
	
	s.logger.Info("Client left room", "client_id", client.Id, "room_id", roomData.RoomID)
	return nil
}

// handleConnectionEvent handles connection manager events
func (s *ConnServer) handleConnectionEvent(event *connection.ConnectionEvent) {
	switch event.Type {
	case connection.EventConnectionOpened:
		s.logger.Debug("Connection opened", "remote_addr", event.Connection.RemoteAddrWithProtocol())
	case connection.EventConnectionClosed:
		s.logger.Debug("Connection closed", "remote_addr", event.Connection.RemoteAddrWithProtocol())
	case connection.EventConnectionError:
		if event.Error != nil && isExpectedDisconnectionError(event.Error) {
			s.logger.Debug("Connection error (expected)", "error", event.Error)
		} else {
			s.logger.Error("Connection error", "error", event.Error)
		}
	case connection.EventMessageReceived:
		s.logger.Debug("Message received", "remote_addr", event.Connection.RemoteAddrWithProtocol())
	case connection.EventMessageSent:
		s.logger.Debug("Message sent", "remote_addr", event.Connection.RemoteAddrWithProtocol())
	}
}

// startWebSocketServer starts the WebSocket server
func (s *ConnServer) startWebSocketServer() error {
	mux := http.NewServeMux()
	
	// WebSocket endpoint handler using websocks
	wsHandler := websocks.NewHandler(func(conn net.Conn) error {
		// Handle the WebSocket connection using our existing logic
		go s.handleWebSocketConnection(conn, nil)
		return nil
	})
	
	mux.Handle("/ws", wsHandler)
	
	// Create HTTP server
	s.wsServer = &http.Server{
		Addr:    s.config.Host + ":" + s.config.WebSocketPort,
		Handler: mux,
	}
	
	// Start server in a goroutine
	go func() {
		if err := s.wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("WebSocket server error", "error", err)
		}
	}()
	
	return nil
}

// handleWebSocketConnection processes a WebSocket connection
func (s *ConnServer) handleWebSocketConnection(wsConn net.Conn, r *http.Request) {
	// Check connection limit
	if s.GetClientCount() >= s.config.MaxConnections {
		s.logger.Warn("WebSocket connection rejected: max connections reached", 
			"current", s.GetClientCount(), "max", s.config.MaxConnections)
		wsConn.Close()
		return
	}
	
	// Set connection timeout
	if s.config.ConnectionTimeout > 0 {
		wsConn.SetDeadline(time.Now().Add(s.config.ConnectionTimeout))
	}
	
	// Create GameConnection from WebSocket connection
	gameConn, err := s.connFactory.CreateWebSocketConnection(wsConn)
	if err != nil {
		s.logger.Error("Failed to create WebSocket game connection", "error", err)
		wsConn.Close()
		return
	}
	
	// Generate client ID
	clientID := ids.RandStringRunes(12)
	
	// Create client session
	client := &ConnClient{
		Id:           clientID,
		Name:         "",
		Conn:         gameConn,
		AckManager:   message.NewAckManager(),
		State:        ClientStateConnecting,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
		Rooms:        make(map[string]bool),
	}
	
	// Add to client map
	s.mutex.Lock()
	s.clients[clientID] = client
	s.mutex.Unlock()
	
	// Add to connection manager
	if err := s.connManager.AddConnection(clientID, gameConn); err != nil {
		s.logger.Error("Failed to add WebSocket connection to manager", "client_id", clientID, "error", err)
		s.removeClient(clientID)
		return
	}
	
	// Set client state to connected
	client.mutex.Lock()
	client.State = ClientStateConnected
	client.mutex.Unlock()
	
	s.logger.Info("WebSocket client connected", 
		"client_id", clientID,
		"connection_type", gameConn.ConnectionType(),
		"remote_addr", gameConn.RemoteAddrWithProtocol())
	
	// Trigger connection open event
	connEvent := events.CreateConnectionOpenEvent(clientID, gameConn)
	s.eventManager.TriggerEventAsync(connEvent)
	
	// Start handling messages for this client
	go s.handleClientMessages(client)
}

// Shutdown gracefully shuts down the server
func (s *ConnServer) Shutdown() {
	s.logger.Info("Shutting down server")
	
	// Cancel context to stop accepting new connections
	s.cancel()
	
	// Close TCP listener
	if s.listener != nil {
		s.listener.Close()
	}
	
	// Close WebSocket server
	if s.wsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.wsServer.Shutdown(ctx)
	}
	
	// Close all client connections
	if s.connManager != nil {
		s.connManager.Close()
	}
	
	// Clean up clients
	s.mutex.Lock()
	for id, client := range s.clients {
		client.AckManager.Close()
		delete(s.clients, id)
	}
	s.mutex.Unlock()
	
	// Close event manager
	if s.eventManager != nil {
		s.eventManager.Close()
	}
	
	s.logger.Info("Server shutdown complete")
	close(s.shutdownComplete)
}

// WaitForShutdown waits for the server to complete shutdown
func (s *ConnServer) WaitForShutdown() {
	<-s.shutdownComplete
}

// GetConnectedClients returns a list of connected client IDs
func (s *ConnServer) GetConnectedClients() []string {
	return s.connManager.ListConnections()
}

// GetClientCount returns the number of connected clients
func (s *ConnServer) GetClientCount() int {
	return s.connManager.ConnectionCount()
}

// BroadcastMessage broadcasts a message to all connected clients
func (s *ConnServer) BroadcastMessage(msg *message.Message) error {
	data, err := message.MessageWrap(msg)
	if err != nil {
		return err
	}
	
	return s.connManager.Broadcast(data)
}

// GetClient returns a client by ID
func (s *ConnServer) GetClient(id string) (*ConnClient, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	client, exists := s.clients[id]
	return client, exists
}

// ContextMessageHandler is a message handler that receives client context
type ContextMessageHandler func(clientID string, msg *message.Message) error

// contextHandlers stores message handlers that need client context
var contextHandlers = make(map[int64]ContextMessageHandler)

// RegisterMessageHandler registers a custom message handler for a specific message type
func (s *ConnServer) RegisterMessageHandler(messageType int64, handler func(*message.Message) error) {
	s.msgRouter.RegisterHandler(messageType, handler)
}

// RegisterContextMessageHandler registers a custom message handler that includes client context
func (s *ConnServer) RegisterContextMessageHandler(messageType int64, handler ContextMessageHandler) {
	contextHandlers[messageType] = handler
}

// SetClientName sets the name for a client
func (s *ConnServer) SetClientName(id, name string) error {
	s.mutex.RLock()
	client, exists := s.clients[id]
	s.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("client %s not found", id)
	}
	
	client.mutex.Lock()
	client.Name = name
	client.mutex.Unlock()
	
	return nil
}

// SetClientState sets the state for a client
func (s *ConnServer) SetClientState(id string, state ClientState) error {
	s.mutex.RLock()
	client, exists := s.clients[id]
	s.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("client %s not found", id)
	}
	
	client.mutex.Lock()
	client.State = state
	client.mutex.Unlock()
	
	return nil
}

// ClientStateToString converts a ClientState to string
func ClientStateToString(state ClientState) string {
	switch state {
	case ClientStateConnecting:
		return "CONNECTING"
	case ClientStateConnected:
		return "CONNECTED"
	case ClientStateAuthenticated:
		return "AUTHENTICATED"
	case ClientStateDisconnected:
		return "DISCONNECTED"
	default:
		return "UNKNOWN"
	}
}

// CreateRoom creates a new room/channel
func (s *ConnServer) CreateRoom(id, name, description string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if _, exists := s.rooms[id]; exists {
		return fmt.Errorf("room %s already exists", id)
	}
	
	if len(s.rooms) >= s.config.MaxRooms {
		return fmt.Errorf("maximum number of rooms (%d) reached", s.config.MaxRooms)
	}
	
	room := &Room{
		ID:          id,
		Name:        name,
		Description: description,
		Clients:     make(map[string]*ConnClient),
		CreatedAt:   time.Now(),
	}
	
	s.rooms[id] = room
	s.logger.Info("Room created", "room_id", id, "name", name)
	return nil
}

// DeleteRoom removes a room and removes all clients from it
func (s *ConnServer) DeleteRoom(roomID string) error {
	s.mutex.Lock()
	room, exists := s.rooms[roomID]
	if !exists {
		s.mutex.Unlock()
		return fmt.Errorf("room %s not found", roomID)
	}
	
	// Remove all clients from the room
	room.mutex.Lock()
	for clientID := range room.Clients {
		if client, clientExists := s.clients[clientID]; clientExists {
			client.mutex.Lock()
			delete(client.Rooms, roomID)
			client.mutex.Unlock()
		}
	}
	room.mutex.Unlock()
	
	delete(s.rooms, roomID)
	s.mutex.Unlock()
	
	s.logger.Info("Room deleted", "room_id", roomID)
	return nil
}

// JoinRoom adds a client to a room
func (s *ConnServer) JoinRoom(clientID, roomID string) error {
	s.mutex.RLock()
	client, clientExists := s.clients[clientID]
	room, roomExists := s.rooms[roomID]
	s.mutex.RUnlock()
	
	if !clientExists {
		return fmt.Errorf("client %s not found", clientID)
	}
	if !roomExists {
		return fmt.Errorf("room %s not found", roomID)
	}
	
	// Check room capacity
	room.mutex.RLock()
	currentClients := len(room.Clients)
	room.mutex.RUnlock()
	
	if currentClients >= s.config.MaxClientsPerRoom {
		return fmt.Errorf("room %s is full (max %d clients)", roomID, s.config.MaxClientsPerRoom)
	}
	
	// Add client to room
	room.mutex.Lock()
	room.Clients[clientID] = client
	newClientCount := len(room.Clients)
	room.mutex.Unlock()
	
	// Add room to client
	client.mutex.Lock()
	client.Rooms[roomID] = true
	clientName := client.Name
	client.mutex.Unlock()
	
	s.logger.Info("Client joined room", "client_id", clientID, "room_id", roomID)
	
	// Trigger room join event
	roomEvent := events.CreateRoomJoinEvent(roomID, room.Name, clientID, clientName, newClientCount)
	s.eventManager.TriggerEventAsync(roomEvent)
	
	return nil
}

// LeaveRoom removes a client from a room
func (s *ConnServer) LeaveRoom(clientID, roomID string) error {
	s.mutex.RLock()
	client, clientExists := s.clients[clientID]
	room, roomExists := s.rooms[roomID]
	s.mutex.RUnlock()
	
	if !clientExists {
		return fmt.Errorf("client %s not found", clientID)
	}
	if !roomExists {
		return fmt.Errorf("room %s not found", roomID)
	}
	
	// Remove client from room
	room.mutex.Lock()
	delete(room.Clients, clientID)
	newClientCount := len(room.Clients)
	room.mutex.Unlock()
	
	// Remove room from client
	client.mutex.Lock()
	clientName := client.Name
	delete(client.Rooms, roomID)
	client.mutex.Unlock()
	
	s.logger.Info("Client left room", "client_id", clientID, "room_id", roomID)
	
	// Trigger room leave event
	roomEvent := events.CreateRoomLeaveEvent(roomID, room.Name, clientID, clientName, newClientCount)
	s.eventManager.TriggerEventAsync(roomEvent)
	
	return nil
}

// BroadcastToRoom sends a message to all clients in a specific room
func (s *ConnServer) BroadcastToRoom(roomID string, msg *message.Message) error {
	s.mutex.RLock()
	room, exists := s.rooms[roomID]
	s.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("room %s not found", roomID)
	}
	
	data, err := message.MessageWrap(msg)
	if err != nil {
		return err
	}
	
	room.mutex.RLock()
	clientIDs := make([]string, 0, len(room.Clients))
	for clientID := range room.Clients {
		clientIDs = append(clientIDs, clientID)
	}
	room.mutex.RUnlock()
	
	// Send to each client in the room
	for _, clientID := range clientIDs {
		if err := s.connManager.SendMessage(clientID, data); err != nil {
			if isExpectedDisconnectionError(err) {
				s.logger.Debug("Failed to send message to client in room (connection closed)", 
					"client_id", clientID, "room_id", roomID, "error", err)
			} else {
				s.logger.Error("Failed to send message to client in room", 
					"client_id", clientID, "room_id", roomID, "error", err)
			}
		}
	}
	
	return nil
}

// GetRoom returns a room by ID
func (s *ConnServer) GetRoom(roomID string) (*Room, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	room, exists := s.rooms[roomID]
	return room, exists
}

// ListRooms returns all room IDs
func (s *ConnServer) ListRooms() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	rooms := make([]string, 0, len(s.rooms))
	for roomID := range s.rooms {
		rooms = append(rooms, roomID)
	}
	return rooms
}

// GetClientRooms returns all rooms a client is in
func (s *ConnServer) GetClientRooms(clientID string) []string {
	s.mutex.RLock()
	client, exists := s.clients[clientID]
	s.mutex.RUnlock()
	
	if !exists {
		return nil
	}
	
	client.mutex.RLock()
	defer client.mutex.RUnlock()
	
	rooms := make([]string, 0, len(client.Rooms))
	for roomID := range client.Rooms {
		rooms = append(rooms, roomID)
	}
	return rooms
}

// GetConfig returns the server configuration
func (s *ConnServer) GetConfig() *ServerConfig {
	return s.config
}

// UpdateConfig updates the server configuration (some settings require restart)
func (s *ConnServer) UpdateConfig(config *ServerConfig) {
	s.config = config
}

// GetEventManager returns the event manager
func (s *ConnServer) GetEventManager() events.EventManager {
	return s.eventManager
}

// registerEventHooks registers event hooks for server operations
func (s *ConnServer) registerEventHooks() {
	// Register authentication hooks
	s.eventManager.RegisterHook(events.EventTypeAuthAttempt, s.handleAuthAttempt, 0)
	s.eventManager.RegisterHook(events.EventTypeAuthSuccess, s.handleAuthSuccess, 0)
	s.eventManager.RegisterHook(events.EventTypeAuthFailure, s.handleAuthFailure, 0)
	
	// Register connection lifecycle hooks
	s.eventManager.RegisterHook(events.EventTypeConnectionOpen, s.handleConnectionOpen, 0)
	s.eventManager.RegisterHook(events.EventTypeConnectionClose, s.handleConnectionClose, 0)
	s.eventManager.RegisterHook(events.EventTypeConnectionError, s.handleConnectionError, 0)
	
	// Register room hooks
	s.eventManager.RegisterHook(events.EventTypeRoomJoin, s.handleRoomJoin, 0)
	s.eventManager.RegisterHook(events.EventTypeRoomLeave, s.handleRoomLeave, 0)
	
	// Register message hooks
	s.eventManager.RegisterHook(events.EventTypeMessageReceived, s.handleMessageReceived, 0)
	s.eventManager.RegisterHook(events.EventTypeMessageSent, s.handleMessageSent, 0)
}

// Event handlers for authentication
func (s *ConnServer) handleAuthAttempt(event events.Event) events.EventResult {
	s.logger.Info("Authentication attempt", 
		"client_id", event.Data()["client_id"],
		"username", event.Data()["username"],
		"method", event.Data()["auth_method"])
	return events.EventResultContinue
}

func (s *ConnServer) handleAuthSuccess(event events.Event) events.EventResult {
	data := event.Data()
	clientID, _ := data["client_id"].(string)
	username, _ := data["username"].(string)
	
	s.logger.Info("Authentication successful", 
		"client_id", clientID,
		"username", username)
	
	// Update client state to authenticated
	if client, exists := s.GetClient(clientID); exists {
		client.mutex.Lock()
		client.Name = username
		client.State = ClientStateAuthenticated
		client.mutex.Unlock()
	}
	
	return events.EventResultContinue
}

func (s *ConnServer) handleAuthFailure(event events.Event) events.EventResult {
	s.logger.Warn("Authentication failed", 
		"client_id", event.Data()["client_id"],
		"username", event.Data()["username"],
		"error", event.Data()["error"])
	return events.EventResultContinue
}

// Event handlers for connections
func (s *ConnServer) handleConnectionOpen(event events.Event) events.EventResult {
	s.logger.Debug("Connection opened event handled",
		"connection_id", event.Data()["connection_id"],
		"remote_addr", event.Data()["remote_addr"])
	return events.EventResultContinue
}

func (s *ConnServer) handleConnectionClose(event events.Event) events.EventResult {
	s.logger.Debug("Connection closed event handled",
		"connection_id", event.Data()["connection_id"],
		"remote_addr", event.Data()["remote_addr"])
	return events.EventResultContinue
}

func (s *ConnServer) handleConnectionError(event events.Event) events.EventResult {
	s.logger.Error("Connection error event handled",
		"connection_id", event.Data()["connection_id"],
		"error", event.Data()["error"])
	return events.EventResultContinue
}

// Event handlers for rooms
func (s *ConnServer) handleRoomJoin(event events.Event) events.EventResult {
	s.logger.Info("Player joined room",
		"client_id", event.Data()["client_id"],
		"room_id", event.Data()["room_id"],
		"client_count", event.Data()["client_count"])
	return events.EventResultContinue
}

func (s *ConnServer) handleRoomLeave(event events.Event) events.EventResult {
	s.logger.Info("Player left room",
		"client_id", event.Data()["client_id"],
		"room_id", event.Data()["room_id"],
		"client_count", event.Data()["client_count"])
	return events.EventResultContinue
}

// Event handlers for messages
func (s *ConnServer) handleMessageReceived(event events.Event) events.EventResult {
	s.logger.Debug("Message received",
		"client_id", event.Data()["client_id"],
		"message_type", event.Data()["message_type"],
		"size", event.Data()["size"])
	return events.EventResultContinue
}

func (s *ConnServer) handleMessageSent(event events.Event) events.EventResult {
	s.logger.Debug("Message sent",
		"client_id", event.Data()["client_id"],
		"message_type", event.Data()["message_type"],
		"size", event.Data()["size"])
	return events.EventResultContinue
}

// AuthenticateClient performs authentication and triggers events
func (s *ConnServer) AuthenticateClient(clientID, username, password, method string) error {
	// Trigger auth attempt event
	authAttemptEvent := events.CreateAuthAttemptEvent(clientID, username, method, nil)
	s.eventManager.TriggerEventAsync(authAttemptEvent)
	
	// Simple authentication logic (replace with real auth)
	// For demo purposes, accept any non-empty username
	if username == "" {
		authFailureEvent := events.CreateAuthFailureEvent(clientID, username, method, 
			fmt.Errorf("username cannot be empty"), nil)
		s.eventManager.TriggerEventSync(authFailureEvent)
		return fmt.Errorf("authentication failed: username cannot be empty")
	}
	
	// Trigger auth success event
	authSuccessEvent := events.CreateAuthSuccessEvent(clientID, username, method, nil)
	s.eventManager.TriggerEventSync(authSuccessEvent)
	
	return nil
}

// getClientIP extracts the client IP address from a connection
func (s *ConnServer) getClientIP(conn net.Conn) string {
	if addr := conn.RemoteAddr(); addr != nil {
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			return tcpAddr.IP.String()
		}
		// Fallback to string representation
		return strings.Split(addr.String(), ":")[0]
	}
	return "unknown"
}

// checkIPLimit checks if the IP has exceeded the per-IP connection limit
func (s *ConnServer) checkIPLimit(clientIP string) bool {
	s.rateLimitMutex.RLock()
	count := s.ipConnections[clientIP]
	s.rateLimitMutex.RUnlock()
	
	return count >= s.config.MaxConnectionsPerIP
}

// checkGlobalRateLimit checks if the global rate limit has been exceeded
func (s *ConnServer) checkGlobalRateLimit() bool {
	s.rateLimitMutex.RLock()
	defer s.rateLimitMutex.RUnlock()
	
	now := time.Now()
	cutoff := now.Add(-time.Second) // Look at last second
	
	count := 0
	for _, connTime := range s.connectionTimes {
		if connTime.After(cutoff) {
			count++
		}
	}
	
	return count >= s.config.MaxConnectionsPerSecond
}

// recordConnection records a new connection for rate limiting
func (s *ConnServer) recordConnection(clientIP string) {
	s.rateLimitMutex.Lock()
	defer s.rateLimitMutex.Unlock()
	
	// Increment IP connection count
	s.ipConnections[clientIP]++
	
	// Record connection time
	now := time.Now()
	s.connectionTimes = append(s.connectionTimes, now)
	
	// Clean up old connection times (keep only last minute)
	cutoff := now.Add(-s.config.RateLimitWindow)
	newTimes := make([]time.Time, 0, len(s.connectionTimes))
	for _, connTime := range s.connectionTimes {
		if connTime.After(cutoff) {
			newTimes = append(newTimes, connTime)
		}
	}
	s.connectionTimes = newTimes
}

// cleanupIPConnection removes a connection from IP tracking when client disconnects
func (s *ConnServer) cleanupIPConnection(clientIP string) {
	s.rateLimitMutex.Lock()
	defer s.rateLimitMutex.Unlock()
	
	if count := s.ipConnections[clientIP]; count > 0 {
		s.ipConnections[clientIP]--
		if s.ipConnections[clientIP] == 0 {
			delete(s.ipConnections, clientIP)
		}
	}
}
