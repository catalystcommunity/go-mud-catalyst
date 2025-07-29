package client

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "math"
    "os"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/catalystcommunity/muddycore/pkg/connection"
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

// ClientState represents the current state of the client
type ClientState int

const (
    ClientStateDisconnected ClientState = iota
    ClientStateConnecting
    ClientStateConnected
    ClientStateReconnecting
)

// ClientConfig holds configuration for the client
type ClientConfig struct {
    Address           string
    ConnectionType    connection.ConnectionType
    ReconnectEnabled  bool
    MaxReconnectDelay time.Duration
    ReconnectBackoff  float64
    MaxRetries        int
    PingInterval      time.Duration
    // Connection timeouts
    ConnectTimeout    time.Duration
    ReadTimeout       time.Duration
    WriteTimeout      time.Duration
    // Message handling
    MessageQueueSize  int
    AckTimeout        time.Duration
    // Heartbeat configuration
    HeartbeatEnabled  bool
    HeartbeatInterval time.Duration
    // Protocol-specific options
    UserAgent         string
    Headers           map[string]string
}

// DefaultClientConfig returns sensible default configuration
func DefaultClientConfig(address string) *ClientConfig {
    // Auto-detect connection type based on address
    connectionType := connection.ConnectionTypeTCP
    if strings.HasPrefix(address, "ws://") || strings.HasPrefix(address, "wss://") {
        connectionType = connection.ConnectionTypeWebSocket
    }
    
    return &ClientConfig{
        Address:           address,
        ConnectionType:    connectionType,
        ReconnectEnabled:  true,
        MaxReconnectDelay: 30 * time.Second,
        ReconnectBackoff:  2.0,
        MaxRetries:        -1, // Infinite retries
        PingInterval:      30 * time.Second,
        // Connection timeouts
        ConnectTimeout:    10 * time.Second,
        ReadTimeout:       30 * time.Second,
        WriteTimeout:      10 * time.Second,
        // Message handling
        MessageQueueSize:  1000,
        AckTimeout:        30 * time.Second,
        // Heartbeat configuration
        HeartbeatEnabled:  true,
        HeartbeatInterval: 30 * time.Second,
        // Protocol-specific options
        UserAgent:         "MuddyCore-Client/1.0",
        Headers:           make(map[string]string),
    }
}

type Client struct {
    Config          *ClientConfig
    Conn            connection.GameConnection
    connMutex       sync.RWMutex  // Protects Conn field
    ConnFactory     connection.ConnectionFactory
    AckManager      *message.AckManager
    MsgRouter       *message.MessageRouter
    State           ClientState
    StateMutex      sync.RWMutex
    Ctx             context.Context
    Cancel          context.CancelFunc
    ReconnectCtx    context.Context
    ReconnectCancel context.CancelFunc
    OnMessage       func(*message.Message)
    OnStateChange   func(ClientState)
    OnConnect       func()
    OnDisconnect    func(error)
    OnError         func(error)
    callbackMutex   sync.RWMutex  // Protects callback functions
    RetryCount      int
    LastConnected   time.Time
    metaMutex       sync.RWMutex  // Protects RetryCount and LastConnected
    MessageQueue    []*message.Message
    QueueMutex      sync.Mutex
    Logger          *logging.Logger
    // Heartbeat management
    heartbeatMutex  sync.Mutex
    heartbeatTicker *time.Ticker
    heartbeatStop   chan struct{}
    // Statistics
    messagesSent     int64
    messagesReceived int64
    bytesTransferred int64
    // Goroutine tracking
    wg              sync.WaitGroup
    closed          bool
    closedMutex     sync.Mutex
}

// NewClient creates a new client with default configuration
func NewClient(address string) *Client {
    return NewClientWithConfig(DefaultClientConfig(address))
}

// NewClientWithConfig creates a new client with custom configuration
func NewClientWithConfig(config *ClientConfig) *Client {
    ctx, cancel := context.WithCancel(context.Background())
    
    c := &Client{
        Config:       config,
        ConnFactory:  connection.NewClientFactory(connection.DefaultConnectionOptions()),
        AckManager:   message.NewAckManager(),
        MsgRouter:    message.NewMessageRouter(),
        State:        ClientStateDisconnected,
        Ctx:          ctx,
        Cancel:       cancel,
        MessageQueue: make([]*message.Message, 0),
        Logger:       logging.GetDefaultLogger().WithComponent("client"),
        closed:       false,
    }
    
    // Register default message handlers
    c.registerDefaultHandlers()
    
    return c
}

// SetMessageHandler sets a callback for received messages
func (c *Client) SetMessageHandler(handler func(*message.Message)) {
    c.callbackMutex.Lock()
    c.OnMessage = handler
    c.callbackMutex.Unlock()
}

// SetStateChangeHandler sets a callback for state changes
func (c *Client) SetStateChangeHandler(handler func(ClientState)) {
    c.callbackMutex.Lock()
    c.OnStateChange = handler
    c.callbackMutex.Unlock()
}

// SetConnectHandler sets a callback for successful connections
func (c *Client) SetConnectHandler(handler func()) {
    c.callbackMutex.Lock()
    c.OnConnect = handler
    c.callbackMutex.Unlock()
}

// SetDisconnectHandler sets a callback for disconnections
func (c *Client) SetDisconnectHandler(handler func(error)) {
    c.callbackMutex.Lock()
    c.OnDisconnect = handler
    c.callbackMutex.Unlock()
}

// SetErrorHandler sets a callback for errors
func (c *Client) SetErrorHandler(handler func(error)) {
    c.callbackMutex.Lock()
    c.OnError = handler
    c.callbackMutex.Unlock()
}

// Connect establishes a connection to the server
func (c *Client) Connect() error {
    c.closedMutex.Lock()
    if c.closed {
        c.closedMutex.Unlock()
        return fmt.Errorf("client is closed")
    }
    c.closedMutex.Unlock()
    
    c.SetState(ClientStateConnecting)
    
    // Create connection context with timeout
    connectCtx, cancel := context.WithTimeout(c.Ctx, c.Config.ConnectTimeout)
    defer cancel()
    
    clientFactory, ok := c.ConnFactory.(*connection.ClientFactory)
    if !ok {
        err := fmt.Errorf("invalid connection factory type")
        c.SetState(ClientStateDisconnected)
        c.callErrorHandler(err)
        return err
    }
    
    // Use result channel to avoid race condition
    type connectResult struct {
        conn connection.GameConnection
        err  error
    }
    
    resultChan := make(chan connectResult, 1)
    go func() {
        var conn connection.GameConnection
        var err error
        
        switch c.Config.ConnectionType {
        case connection.ConnectionTypeTCP:
            conn, err = clientFactory.ConnectTCP(c.Config.Address)
        case connection.ConnectionTypeWebSocket:
            conn, err = clientFactory.ConnectWebSocket(connectCtx, c.Config.Address)
        default:
            conn, err = clientFactory.Connect(c.Config.Address)
        }
        
        resultChan <- connectResult{conn: conn, err: err}
    }()
    
    var conn connection.GameConnection
    var err error
    
    select {
    case result := <-resultChan:
        conn = result.conn
        err = result.err
    case <-connectCtx.Done():
        err = fmt.Errorf("connection timeout after %v", c.Config.ConnectTimeout)
    }
    
    if err != nil {
        c.SetState(ClientStateDisconnected)
        c.callErrorHandler(err)
        return fmt.Errorf("failed to connect: %v", err)
    }
    
    // Set connection timeouts
    if c.Config.ReadTimeout > 0 {
        conn.SetReadTimeout(c.Config.ReadTimeout)
    }
    if c.Config.WriteTimeout > 0 {
        conn.SetWriteTimeout(c.Config.WriteTimeout)
    }
    
    c.connMutex.Lock()
    c.Conn = conn
    c.connMutex.Unlock()
    
    c.SetState(ClientStateConnected)
    c.metaMutex.Lock()
    c.LastConnected = time.Now()
    c.RetryCount = 0
    c.metaMutex.Unlock()
    
    c.Logger.Info("Connected to server", 
        "address", c.Config.Address,
        "connection_type", conn.ConnectionType())
    
    // Start heartbeat if enabled
    if c.Config.HeartbeatEnabled {
        c.startHeartbeat()
    }
    
    // Start message handling
    c.wg.Add(1)
    go c.ReadFromServer()
    
    // Start acknowledgment timeout monitoring
    c.wg.Add(1)
    go c.monitorAckTimeouts()
    
    // Send any queued messages
    c.SendQueuedMessages()
    
    // Call connect handler
    c.callConnectHandler()
    
    return nil
}

// Disconnect closes the connection
func (c *Client) Disconnect() {
    c.Disconnect_WithError(nil)
}

// DisconnectWithError closes the connection with an error reason
func (c *Client) Disconnect_WithError(err error) {
    c.closedMutex.Lock()
    if c.closed {
        c.closedMutex.Unlock()
        return
    }
    c.closed = true
    c.closedMutex.Unlock()
    
    c.SetState(ClientStateDisconnected)
    
    // Stop heartbeat
    c.stopHeartbeat()
    
    if c.ReconnectCancel != nil {
        c.ReconnectCancel()
    }
    
    c.connMutex.Lock()
    if c.Conn != nil {
        c.gracefulClose()
        c.Conn = nil
    }
    c.connMutex.Unlock()
    
    // Give a brief moment for goroutines to detect the closed connection
    time.Sleep(100 * time.Millisecond)
    
    // Wait for all goroutines to finish with timeout
    done := make(chan struct{})
    go func() {
        c.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        // All goroutines finished
        c.Logger.Debug("All goroutines finished cleanly")
    case <-time.After(2 * time.Second):
        // Timeout reached
        c.Logger.Warn("Timeout waiting for goroutines to finish during disconnect")
    }
    
    // Call disconnect handler
    c.callDisconnectHandler(err)
}

// gracefulClose performs a graceful shutdown of the connection
func (c *Client) gracefulClose() {
    if c.Conn == nil {
        return
    }
    
    // Give time for any pending acknowledgments with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
    defer cancel()
    
    // Wait for pending acknowledgments if available
    if c.AckManager != nil {
        // Create a simple wait mechanism since we don't have WaitForPendingAcks
        select {
        case <-ctx.Done():
            // Timeout reached, proceed with close
            c.Logger.Debug("Graceful close timeout reached, proceeding with close")
        case <-time.After(100 * time.Millisecond):
            // Give a brief moment for any pending operations
        }
    }
    
    // Check if it's a WebSocket connection and handle it appropriately
    if wsConn, ok := c.Conn.(interface{ WriteCloseMessage() error }); ok {
        // Send WebSocket close frame
        if err := wsConn.WriteCloseMessage(); err != nil {
            c.Logger.Debug("Failed to send WebSocket close frame", "error", err)
        } else {
            c.Logger.Debug("WebSocket close frame sent")
        }
        // Give a brief moment for the close frame to be sent
        select {
        case <-time.After(50 * time.Millisecond):
            // Close frame sent timeout
        case <-ctx.Done():
            // Overall timeout reached
        }
    }
    
    // Close the underlying connection
    if err := c.Conn.Close(); err != nil {
        c.Logger.Debug("Error closing connection", "error", err)
    }
}

// Close shuts down the client completely
func (c *Client) Close() {
    c.Logger.Info("Shutting down client")
    
    // Cancel context to stop all goroutines
    c.Cancel()
    
    // Immediately close the connection to unblock any goroutines
    c.connMutex.Lock()
    if c.Conn != nil {
        c.Conn.Close()
    }
    c.connMutex.Unlock()
    
    // Stop heartbeat
    c.stopHeartbeat()
    
    // Disconnect from server (this will handle the rest of the cleanup)
    c.Disconnect()
    
    // Note: c.Disconnect() already waits for goroutines with timeout, so we don't need to wait again
    
    // Close acknowledgment manager
    if c.AckManager != nil {
        c.AckManager.Close()
    }
    
    // Clear message queue
    c.ClearQueue()
    
    c.Logger.Info("Client shutdown complete")
}

// Run starts the client with interactive stdin input (for examples)
func (c *Client) Run() error {
    defer c.Close()
    
    if err := c.Connect(); err != nil {
        return err
    }
    
    // Start reconnection handler if enabled
    if c.Config.ReconnectEnabled {
        c.wg.Add(1)
        go c.ReconnectHandler()
    }
    
    // Read from stdin and send to server
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Println("Enter messages (type 'quit' to exit):")
    
    for scanner.Scan() {
        select {
        case <-c.Ctx.Done():
            return nil
        default:
        }
        
        text := scanner.Text()
        if strings.ToLower(text) == "quit" {
            break
        }
        
        // Send as plain text for backwards compatibility
        if err := c.SendPlainText(text); err != nil {
            c.Logger.Error("Failed to send message", "error", err)
        }
    }
    
    if err := scanner.Err(); err != nil {
        return err
    }
    
    return nil
}

// readFromServer handles incoming messages from the server
func (c *Client) ReadFromServer() {
    defer func() {
        // Safely handle WaitGroup done - protect against negative counter
        defer func() {
            if r := recover(); r != nil {
                // If we get a panic from WaitGroup, it means Done was called more than Add
                // This can happen in test scenarios where the method is called directly
                c.Logger.Debug("WaitGroup Done called without matching Add (likely in test)")
            }
        }()
        c.wg.Done()
    }()
    defer func() {
        if c.Config.ReconnectEnabled && c.GetStateInternal() != ClientStateDisconnected {
            c.SetState(ClientStateReconnecting)
        }
    }()
    
    for {
        // Check if client context is cancelled
        select {
        case <-c.Ctx.Done():
            c.Logger.Debug("Client context cancelled, stopping ReadFromServer")
            return
        default:
        }
        
        // Check connection state
        c.connMutex.RLock()
        conn := c.Conn
        c.connMutex.RUnlock()
        
        if conn == nil {
            c.Logger.Debug("Connection is nil in ReadFromServer, stopping read loop")
            return
        }
        
        // Check if connection context is cancelled
        select {
        case <-conn.Context().Done():
            c.Logger.Info("Connection closed by server")
            return
        default:
        }
        
        // Try to read message
        data, err := conn.ReadMessage()
        if err != nil {
            // Use improved error classification
            if isExpectedDisconnectionError(err) {
                c.Logger.Debug("Connection closed by server", "reason", err)
            } else {
                c.Logger.Error("Read error", "error", err)
                c.callErrorHandler(err)
            }
            return
        }
        
        // Update statistics
        atomic.AddInt64(&c.messagesReceived, 1)
        atomic.AddInt64(&c.bytesTransferred, int64(len(data)))
        
        // Try to parse as CBOR message
        if msg, err := message.MessageUnwrap(data); err == nil {
            c.HandleMessage(msg)
        } else {
            // Handle plain text message (for backwards compatibility)
            c.HandlePlainText(data)
        }
    }
}

// handleMessage processes incoming messages and handles acknowledgments
func (c *Client) HandleMessage(msg *message.Message) {
    // Send acknowledgment if required
    if msg.RequiresAck && msg.ID != "" {
        ackMsg := message.CreateAckMessage(msg.ID, true, "")
        if ackMsg != nil {
            c.SendMessage(ackMsg)
        }
    }
    
    // Handle acknowledgment messages
    if msg.Type == message.MessageTypeAck || msg.Type == message.MessageTypeNack {
        if ackData, err := message.ParseAckMessage(msg); err == nil {
            c.AckManager.ProcessAcknowledgment(ackData)
        }
        return
    }
    
    // Route through message router first
    if err := c.MsgRouter.RouteMessage(msg); err != nil {
        // If no handler registered, use default behavior
        c.DefaultMessageHandler(msg)
    }
    
    // Call user message handler if set
    c.callbackMutex.RLock()
    onMessage := c.OnMessage
    c.callbackMutex.RUnlock()
    
    if onMessage != nil {
        onMessage(msg)
    }
}

// handlePlainText handles plain text messages
func (c *Client) HandlePlainText(data []byte) {
    c.Logger.Info("Received plain text message", "content", string(data))
}

// defaultMessageHandler provides default handling for messages
func (c *Client) DefaultMessageHandler(msg *message.Message) {
    c.Logger.Info("Received message", 
        "type", message.MessageTypeToString(msg.Type), 
        "content", string(msg.Contents))
}

// SendMessage sends a message to the server with acknowledgment tracking
func (c *Client) SendMessage(msg *message.Message) error {
    if c.GetStateInternal() != ClientStateConnected {
        if c.Config.ReconnectEnabled {
            // Queue message for later
            c.QueueMessage(msg)
            return fmt.Errorf("not connected, message queued")
        }
        return fmt.Errorf("not connected to server")
    }
    
    // Set message ID if not set and requires acknowledgment
    if msg.RequiresAck && msg.ID == "" {
        msg.ID = message.GenerateMessageID()
    }
    
    // Set acknowledgment timeout if not already set
    if msg.RequiresAck && msg.AckTimeout == 0 {
        msg.AckTimeout = c.Config.AckTimeout
    }
    
    // Track message for acknowledgment if needed
    if msg.RequiresAck {
        c.AckManager.TrackMessage(msg)
    }
    
    // Send message
    data, err := message.MessageWrap(msg)
    if err != nil {
        return err
    }
    
    c.connMutex.RLock()
    conn := c.Conn
    c.connMutex.RUnlock()
    
    if conn == nil {
        return fmt.Errorf("connection is nil")
    }
    
    err = conn.WriteMessage(data)
    if err != nil {
        // Use improved error classification for send errors
        if isExpectedDisconnectionError(err) {
            c.Logger.Debug("Send failed due to disconnection", "error", err)
            c.SetState(ClientStateReconnecting)
        } else {
            c.Logger.Error("Send error", "error", err)
            c.callErrorHandler(err)
        }
        return err
    }
    
    // Update statistics
    atomic.AddInt64(&c.messagesSent, 1)
    atomic.AddInt64(&c.bytesTransferred, int64(len(data)))
    
    return nil
}

// SendPlainText sends plain text to the server (for backwards compatibility)
func (c *Client) SendPlainText(text string) error {
    if c.GetStateInternal() != ClientStateConnected {
        return fmt.Errorf("not connected to server")
    }
    
    data := []byte(text)
    
    c.connMutex.RLock()
    conn := c.Conn
    c.connMutex.RUnlock()
    
    if conn == nil {
        return fmt.Errorf("connection is nil")
    }
    
    err := conn.WriteMessage(data)
    if err != nil {
        return err
    }
    
    // Update statistics
    atomic.AddInt64(&c.messagesSent, 1)
    atomic.AddInt64(&c.bytesTransferred, int64(len(data)))
    
    return nil
}

// SendChatMessage sends a chat message
func (c *Client) SendChatMessage(text string) error {
    msg := &message.Message{
        Type:     message.MessageTypeChat,
        Contents: []byte(text),
    }
    return c.SendMessage(msg)
}

// registerDefaultHandlers registers default message handlers
func (c *Client) registerDefaultHandlers() {
    c.MsgRouter.RegisterHandler(message.MessageTypeHeartbeat, c.HandleHeartbeat)
    c.MsgRouter.RegisterHandler(message.MessageTypeError, c.HandleError)
}

// handleHeartbeat handles heartbeat messages
func (c *Client) HandleHeartbeat(msg *message.Message) error {
    // Respond to heartbeats automatically
    response := &message.Message{
        Type:     message.MessageTypeHeartbeat,
        Contents: []byte("pong"),
    }
    return c.SendMessage(response)
}

// handleError handles error messages
func (c *Client) HandleError(msg *message.Message) error {
    c.Logger.Error("Server error", "message", string(msg.Contents))
    return nil
}

// reconnectHandler handles automatic reconnection
func (c *Client) ReconnectHandler() {
    defer func() {
        // Safely handle WaitGroup done - protect against negative counter
        defer func() {
            if r := recover(); r != nil {
                // If we get a panic from WaitGroup, it means Done was called more than Add
                // This can happen in test scenarios where the method is called directly
                c.Logger.Debug("WaitGroup Done called without matching Add (likely in test)")
            }
        }()
        c.wg.Done()
    }()
    for {
        select {
        case <-c.Ctx.Done():
            return
        default:
            if c.GetStateInternal() == ClientStateReconnecting {
                delay := c.CalculateReconnectDelay()
                c.metaMutex.RLock()
                retryCount := c.RetryCount
                c.metaMutex.RUnlock()
                c.Logger.Info("Reconnecting", 
                    "delay", delay,
                    "attempt", retryCount+1,
                    "max_retries", c.Config.MaxRetries)
                
                c.ReconnectCtx, c.ReconnectCancel = context.WithCancel(c.Ctx)
                
                select {
                case <-time.After(delay):
                    if err := c.Connect(); err != nil {
                        c.metaMutex.Lock()
                        c.RetryCount++
                        retryCount := c.RetryCount
                        c.metaMutex.Unlock()
                        if c.Config.MaxRetries > 0 && retryCount >= c.Config.MaxRetries {
                            c.Logger.Warn("Max reconnection attempts reached, giving up", 
                                "max_retries", c.Config.MaxRetries)
                            c.SetState(ClientStateDisconnected)
                            return
                        }
                        c.Logger.Error("Reconnection failed", "error", err)
                    } else {
                        c.Logger.Info("Reconnection successful")
                    }
                case <-c.ReconnectCtx.Done():
                    // Reconnection cancelled
                }
            } else {
                time.Sleep(100 * time.Millisecond) // Small sleep to prevent busy loop
            }
        }
    }
}

// CalculateReconnectDelay calculates the delay before next reconnection attempt
func (c *Client) CalculateReconnectDelay() time.Duration {
    baseDelay := time.Second
    c.metaMutex.RLock()
    retryCount := c.RetryCount
    c.metaMutex.RUnlock()
    delay := time.Duration(float64(baseDelay) * math.Pow(c.Config.ReconnectBackoff, float64(retryCount)))
    
    if delay > c.Config.MaxReconnectDelay {
        delay = c.Config.MaxReconnectDelay
    }
    
    return delay
}

// SetState sets the client state and calls the state change handler
func (c *Client) SetState(state ClientState) {
    c.StateMutex.Lock()
    oldState := c.State
    c.State = state
    c.StateMutex.Unlock()
    
    if oldState != state {
        c.Logger.Info("Client state changed", 
            "from", ClientStateToString(oldState),
            "to", ClientStateToString(state))
        
        c.callbackMutex.RLock()
        onStateChange := c.OnStateChange
        c.callbackMutex.RUnlock()
        
        if onStateChange != nil {
            onStateChange(state)
        }
    }
}

// getState returns the current client state
func (c *Client) GetStateInternal() ClientState {
    c.StateMutex.RLock()
    defer c.StateMutex.RUnlock()
    return c.State
}

// GetState returns the current client state (public method)
func (c *Client) GetState() ClientState {
    return c.GetStateInternal()
}

// IsConnected returns true if the client is connected
func (c *Client) IsConnected() bool {
    return c.GetStateInternal() == ClientStateConnected
}

// queueMessage adds a message to the queue for later sending
func (c *Client) QueueMessage(msg *message.Message) {
    c.QueueMutex.Lock()
    defer c.QueueMutex.Unlock()
    
    // Check queue size limit
    if len(c.MessageQueue) >= c.Config.MessageQueueSize {
        // Remove oldest message to make room
        c.MessageQueue = c.MessageQueue[1:]
        c.Logger.Warn("Message queue full, dropping oldest message",
            "queue_size", c.Config.MessageQueueSize)
    }
    
    c.MessageQueue = append(c.MessageQueue, msg)
}

// sendQueuedMessages sends all queued messages
func (c *Client) SendQueuedMessages() {
    c.QueueMutex.Lock()
    queue := c.MessageQueue
    c.MessageQueue = make([]*message.Message, 0)
    c.QueueMutex.Unlock()
    
    for _, msg := range queue {
        if err := c.SendMessage(msg); err != nil {
            c.Logger.Error("Failed to send queued message", "error", err)
        }
    }
}

// GetConnectionInfo returns information about the current connection
func (c *Client) GetConnectionInfo() *connection.ConnectionInfo {
    c.connMutex.RLock()
    conn := c.Conn
    c.connMutex.RUnlock()
    
    if conn == nil {
        return nil
    }
    
    if tcpConn, ok := conn.(*connection.TCPConnection); ok {
        return tcpConn.GetConnectionInfo()
    } else if wsConn, ok := conn.(*connection.WebSocketConnection); ok {
        return wsConn.GetConnectionInfo()
    }
    
    return nil
}

// RegisterMessageHandler registers a handler for a specific message type
func (c *Client) RegisterMessageHandler(messageType int64, handler message.MessageHandler) {
    c.MsgRouter.RegisterHandler(messageType, handler)
}

// ClientStateToString converts a ClientState to string
func ClientStateToString(state ClientState) string {
    switch state {
    case ClientStateDisconnected:
        return "DISCONNECTED"
    case ClientStateConnecting:
        return "CONNECTING"
    case ClientStateConnected:
        return "CONNECTED"
    case ClientStateReconnecting:
        return "RECONNECTING"
    default:
        return "UNKNOWN"
    }
}

// Helper methods for callback handling

func (c *Client) callConnectHandler() {
    c.callbackMutex.RLock()
    onConnect := c.OnConnect
    c.callbackMutex.RUnlock()
    
    if onConnect != nil {
        c.wg.Add(1)
        go func() {
            defer c.wg.Done()
            onConnect()
        }()
    }
}

func (c *Client) callDisconnectHandler(err error) {
    c.callbackMutex.RLock()
    onDisconnect := c.OnDisconnect
    c.callbackMutex.RUnlock()
    
    if onDisconnect != nil {
        // Don't use wait group for disconnect handler to avoid deadlock
        go onDisconnect(err)
    }
}

func (c *Client) callErrorHandler(err error) {
    c.callbackMutex.RLock()
    onError := c.OnError
    c.callbackMutex.RUnlock()
    
    if onError != nil {
        c.wg.Add(1)
        go func() {
            defer c.wg.Done()
            onError(err)
        }()
    }
}

// Heartbeat management

func (c *Client) startHeartbeat() {
    c.stopHeartbeat() // Stop any existing heartbeat
    
    // Set up new heartbeat with proper synchronization
    c.heartbeatMutex.Lock()
    c.heartbeatStop = make(chan struct{})
    c.heartbeatTicker = time.NewTicker(c.Config.HeartbeatInterval)
    
    // Capture channels for the goroutine to avoid races
    stopChan := c.heartbeatStop
    ticker := c.heartbeatTicker
    c.heartbeatMutex.Unlock()
    
    c.wg.Add(1)
    go func() {
        defer c.wg.Done()
        defer func() {
            // Clean shutdown of ticker
            c.heartbeatMutex.Lock()
            if c.heartbeatTicker == ticker {
                c.heartbeatTicker.Stop()
                c.heartbeatTicker = nil
            }
            c.heartbeatMutex.Unlock()
        }()
        
        for {
            select {
            case <-ticker.C:
                if c.IsConnected() {
                    heartbeat := &message.Message{
                        Type:     message.MessageTypeHeartbeat,
                        Contents: []byte("ping"),
                    }
                    if err := c.SendMessage(heartbeat); err != nil {
                        c.Logger.Error("Failed to send heartbeat", "error", err)
                    }
                }
            case <-stopChan:
                return
            case <-c.Ctx.Done():
                return
            }
        }
    }()
}

func (c *Client) stopHeartbeat() {
    c.heartbeatMutex.Lock()
    defer c.heartbeatMutex.Unlock()
    
    // Close stop channel if it exists and hasn't been closed
    if c.heartbeatStop != nil {
        select {
        case <-c.heartbeatStop:
            // Channel already closed
        default:
            close(c.heartbeatStop)
        }
        c.heartbeatStop = nil
    }
    
    // Stop ticker if it exists
    if c.heartbeatTicker != nil {
        c.heartbeatTicker.Stop()
        c.heartbeatTicker = nil
    }
}

// Statistics and monitoring

// GetStatistics returns client statistics
func (c *Client) GetStatistics() map[string]interface{} {
    c.metaMutex.RLock()
    retryCount := c.RetryCount
    lastConnected := c.LastConnected
    c.metaMutex.RUnlock()
    
    return map[string]interface{}{
        "messages_sent":     atomic.LoadInt64(&c.messagesSent),
        "messages_received": atomic.LoadInt64(&c.messagesReceived),
        "bytes_transferred": atomic.LoadInt64(&c.bytesTransferred),
        "retry_count":       retryCount,
        "last_connected":    lastConnected,
        "queue_size":        c.GetQueueSize(),
        "state":             ClientStateToString(c.GetState()),
    }
}

// GetQueueSize returns the current message queue size
func (c *Client) GetQueueSize() int {
    c.QueueMutex.Lock()
    defer c.QueueMutex.Unlock()
    return len(c.MessageQueue)
}

// ClearQueue clears the message queue
func (c *Client) ClearQueue() {
    c.QueueMutex.Lock()
    defer c.QueueMutex.Unlock()
    c.MessageQueue = make([]*message.Message, 0)
}

// monitorAckTimeouts monitors for acknowledgment timeouts and handles retries
func (c *Client) monitorAckTimeouts() {
    defer func() {
        // Safely handle WaitGroup done - protect against negative counter
        defer func() {
            if r := recover(); r != nil {
                // If we get a panic from WaitGroup, it means Done was called more than Add
                // This can happen in test scenarios where the method is called directly
                c.Logger.Debug("WaitGroup Done called without matching Add (likely in test)")
            }
        }()
        c.wg.Done()
    }()
    ticker := time.NewTicker(time.Second) // Check every second
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if c.IsConnected() {
                timeouts := c.AckManager.GetTimeoutMessages()
                for _, pendingAck := range timeouts {
                    msg := pendingAck.Message
                    c.Logger.Warn("Message acknowledgment timeout",
                        "message_id", msg.ID,
                        "type", message.MessageTypeToString(msg.Type),
                        "timeout", msg.AckTimeout,
                        "retry_count", pendingAck.RetryCount)
                    
                    c.callErrorHandler(fmt.Errorf("acknowledgment timeout for message %s", msg.ID))
                }
            }
        case <-c.Ctx.Done():
            return
        }
    }
}
