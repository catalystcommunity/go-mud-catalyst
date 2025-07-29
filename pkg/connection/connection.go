package connection

import (
	"context"
	"net"
	"time"
)

// ConnectionType represents the type of network connection
type ConnectionType string

const (
	ConnectionTypeTCP       ConnectionType = "tcp"
	ConnectionTypeWebSocket ConnectionType = "websocket"
)

// GameConnection extends net.Conn with game-specific functionality
// It provides a unified interface for both TCP and WebSocket connections
type GameConnection interface {
	net.Conn
	
	// ConnectionType returns the type of this connection
	ConnectionType() ConnectionType
	
	// IsConnected returns true if the connection is currently active
	IsConnected() bool
	
	// WriteMessage writes a complete message to the connection
	// This handles framing for WebSocket connections and line termination for TCP
	WriteMessage(data []byte) error
	
	// ReadMessage reads a complete message from the connection
	// This handles framing for WebSocket connections and line reading for TCP
	ReadMessage() ([]byte, error)
	
	// SetReadTimeout sets a timeout for read operations
	SetReadTimeout(timeout time.Duration) error
	
	// SetWriteTimeout sets a timeout for write operations  
	SetWriteTimeout(timeout time.Duration) error
	
	// Context returns a context that is cancelled when the connection closes
	Context() context.Context
	
	// RemoteAddr returns the remote network address with protocol information
	RemoteAddrWithProtocol() string
	
	// LocalAddr returns the local network address with protocol information
	LocalAddrWithProtocol() string
}

// ConnectionInfo provides metadata about a connection
type ConnectionInfo struct {
	Type           ConnectionType
	RemoteAddr     string
	LocalAddr      string
	ConnectedAt    time.Time
	LastActivity   time.Time
	BytesRead      uint64
	BytesWritten   uint64
	MessagesRead   uint64
	MessagesWritten uint64
}

// ConnectionManager interface for managing multiple connections
type ConnectionManager interface {
	// AddConnection adds a connection to be managed
	AddConnection(id string, conn GameConnection) error
	
	// RemoveConnection removes a connection from management
	RemoveConnection(id string) error
	
	// GetConnection retrieves a connection by ID
	GetConnection(id string) (GameConnection, bool)
	
	// ListConnections returns all active connection IDs
	ListConnections() []string
	
	// GetConnectionInfo returns information about a connection
	GetConnectionInfo(id string) (*ConnectionInfo, bool)
	
	// Broadcast sends a message to all connections
	Broadcast(data []byte) error
	
	// BroadcastToType sends a message to all connections of a specific type
	BroadcastToType(connType ConnectionType, data []byte) error
	
	// SendMessage sends a message to a specific connection
	SendMessage(id string, data []byte) error
	
	// Close closes all managed connections
	Close() error
	
	// ConnectionCount returns the number of active connections
	ConnectionCount() int
	
	// ConnectionCountByType returns the number of connections by type
	ConnectionCountByType() map[ConnectionType]int
}

// ConnectionFactory interface for creating connections
type ConnectionFactory interface {
	// CreateTCPConnection creates a GameConnection from a TCP net.Conn
	CreateTCPConnection(conn net.Conn) GameConnection
	
	// CreateWebSocketConnection creates a GameConnection from a WebSocket connection
	CreateWebSocketConnection(conn interface{}) (GameConnection, error)
	
	// DetectConnectionType attempts to detect the connection type from a net.Conn
	DetectConnectionType(conn net.Conn) (ConnectionType, error)
	
	// CreateConnection creates a GameConnection with automatic protocol detection
	CreateConnection(conn net.Conn) (GameConnection, error)
}

// ConnectionOptions provides configuration for connections
type ConnectionOptions struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	ReadBufferSize    int
	WriteBufferSize   int
	MaxMessageSize    int64
	EnableCompression bool
	PingInterval      time.Duration // For WebSocket connections
	PongTimeout       time.Duration // For WebSocket connections
}

// DefaultConnectionOptions returns sensible default options
func DefaultConnectionOptions() *ConnectionOptions {
	return &ConnectionOptions{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
		MaxMessageSize:    1024 * 1024, // 1MB
		EnableCompression: false,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
	}
}

// ConnectionEvent represents events that can occur on connections
type ConnectionEvent struct {
	Type       ConnectionEventType
	Connection GameConnection
	Data       []byte
	Error      error
	Timestamp  time.Time
}

type ConnectionEventType string

const (
	EventConnectionOpened  ConnectionEventType = "opened"
	EventConnectionClosed  ConnectionEventType = "closed"
	EventMessageReceived   ConnectionEventType = "message_received"
	EventMessageSent       ConnectionEventType = "message_sent"
	EventConnectionError   ConnectionEventType = "error"
	EventConnectionTimeout ConnectionEventType = "timeout"
)

// ConnectionEventHandler is called when connection events occur
type ConnectionEventHandler func(event *ConnectionEvent)