package connection

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	websocks "github.com/catalystcommunity/websocks/v1"
)

// DefaultConnectionFactory implements ConnectionFactory
type DefaultConnectionFactory struct {
	options *ConnectionOptions
}

// NewConnectionFactory creates a new connection factory with options
func NewConnectionFactory(options *ConnectionOptions) *DefaultConnectionFactory {
	if options == nil {
		options = DefaultConnectionOptions()
	}
	return &DefaultConnectionFactory{
		options: options,
	}
}

// CreateTCPConnection creates a GameConnection from a TCP net.Conn
func (f *DefaultConnectionFactory) CreateTCPConnection(conn net.Conn) GameConnection {
	return NewTCPConnection(conn, f.options)
}

// CreateWebSocketConnection creates a GameConnection from a WebSocket connection
func (f *DefaultConnectionFactory) CreateWebSocketConnection(conn interface{}) (GameConnection, error) {
	// The websocks package returns net.Conn, so we expect conn to be net.Conn
	netConn, ok := conn.(net.Conn)
	if !ok {
		return nil, fmt.Errorf("websocket connection must implement net.Conn")
	}
	
	return NewWebSocketConnection(netConn, f.options), nil
}

// DetectConnectionType attempts to detect the connection type from a net.Conn
// This is used for server-side connections where we need to determine the protocol
func (f *DefaultConnectionFactory) DetectConnectionType(conn net.Conn) (ConnectionType, error) {
	// Set a short timeout for protocol detection
	oldDeadline := time.Now().Add(5 * time.Second)
	conn.SetReadDeadline(oldDeadline)
	defer conn.SetReadDeadline(time.Time{}) // Reset deadline
	
	// Create a buffered reader to peek at the data
	reader := bufio.NewReader(conn)
	
	// Peek at the first few bytes to determine protocol
	data, err := reader.Peek(128) // Peek at first 128 bytes
	if err != nil {
		return "", fmt.Errorf("failed to peek connection data: %v", err)
	}
	
	dataStr := string(data)
	
	// Check for HTTP/WebSocket upgrade request
	if strings.Contains(dataStr, "GET ") && 
	   strings.Contains(dataStr, "HTTP/") && 
	   (strings.Contains(strings.ToLower(dataStr), "upgrade: websocket") || 
	    strings.Contains(strings.ToLower(dataStr), "connection: upgrade")) {
		return ConnectionTypeWebSocket, nil
	}
	
	// Check for other HTTP methods that might indicate WebSocket
	httpMethods := []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS "}
	for _, method := range httpMethods {
		if strings.HasPrefix(dataStr, method) {
			// Likely HTTP, could be WebSocket upgrade
			if strings.Contains(strings.ToLower(dataStr), "websocket") {
				return ConnectionTypeWebSocket, nil
			}
		}
	}
	
	// If we don't see HTTP headers, assume TCP
	return ConnectionTypeTCP, nil
}

// CreateConnection creates a GameConnection with automatic protocol detection
func (f *DefaultConnectionFactory) CreateConnection(conn net.Conn) (GameConnection, error) {
	connType, err := f.DetectConnectionType(conn)
	if err != nil {
		// If detection fails, default to TCP
		connType = ConnectionTypeTCP
	}
	
	switch connType {
	case ConnectionTypeTCP:
		return f.CreateTCPConnection(conn), nil
	case ConnectionTypeWebSocket:
		// For detected WebSocket connections, we need to handle the upgrade
		return f.handleWebSocketUpgrade(conn)
	default:
		return nil, fmt.Errorf("unsupported connection type: %s", connType)
	}
}

// handleWebSocketUpgrade handles WebSocket protocol upgrade
func (f *DefaultConnectionFactory) handleWebSocketUpgrade(conn net.Conn) (GameConnection, error) {
	// The websocks package should handle the upgrade for us
	// For now, we'll create a WebSocket connection wrapper directly
	// In a real implementation, you'd use websocks to handle the HTTP upgrade
	
	// TODO: Implement proper WebSocket upgrade using websocks package
	// For now, assume the connection is already upgraded
	return NewWebSocketConnection(conn, f.options), nil
}

// ConnectionTypeFromString converts a string to ConnectionType
func ConnectionTypeFromString(s string) (ConnectionType, error) {
	switch strings.ToLower(s) {
	case "tcp":
		return ConnectionTypeTCP, nil
	case "websocket", "ws":
		return ConnectionTypeWebSocket, nil
	default:
		return "", fmt.Errorf("unknown connection type: %s", s)
	}
}

// ServerFactory provides connection creation for servers
type ServerFactory struct {
	factory *DefaultConnectionFactory
}

// NewServerFactory creates a factory for server-side connections
func NewServerFactory(options *ConnectionOptions) *ServerFactory {
	return &ServerFactory{
		factory: NewConnectionFactory(options),
	}
}

// HandleConnection creates a GameConnection from an incoming server connection
// This performs protocol detection and creates the appropriate wrapper
func (sf *ServerFactory) HandleConnection(conn net.Conn) (GameConnection, error) {
	return sf.factory.CreateConnection(conn)
}

// ClientFactory provides connection creation for clients
type ClientFactory struct {
	factory *DefaultConnectionFactory
}

// NewClientFactory creates a factory for client-side connections
func NewClientFactory(options *ConnectionOptions) *ClientFactory {
	return &ClientFactory{
		factory: NewConnectionFactory(options),
	}
}

// ConnectTCP creates a TCP client connection
func (cf *ClientFactory) ConnectTCP(address string) (GameConnection, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	return cf.factory.CreateTCPConnection(conn), nil
}

// ConnectWebSocket creates a WebSocket client connection
func (cf *ClientFactory) ConnectWebSocket(ctx context.Context, url string) (GameConnection, error) {
	// Use websocks to create WebSocket connection with context
	conn, err := websocks.Connect(ctx, url)
	if err != nil {
		return nil, err
	}
	
	return cf.factory.CreateWebSocketConnection(conn)
}

// Connect creates a client connection based on the URL/address format
func (cf *ClientFactory) Connect(address string) (GameConnection, error) {
	// Determine connection type from address format
	if strings.HasPrefix(address, "ws://") || strings.HasPrefix(address, "wss://") {
		// Use background context for legacy compatibility
		return cf.ConnectWebSocket(context.Background(), address)
	}
	
	// Default to TCP for addresses like "host:port"
	return cf.ConnectTCP(address)
}

// CreateConnection creates a GameConnection from an existing net.Conn (for client factory)
func (cf *ClientFactory) CreateConnection(conn net.Conn) (GameConnection, error) {
	return cf.factory.CreateConnection(conn)
}

// DetectConnectionType detects the connection type from a net.Conn (for client factory)
func (cf *ClientFactory) DetectConnectionType(conn net.Conn) (ConnectionType, error) {
	return cf.factory.DetectConnectionType(conn)
}

// CreateTCPConnection creates a GameConnection from a TCP net.Conn (for client factory)
func (cf *ClientFactory) CreateTCPConnection(conn net.Conn) GameConnection {
	return cf.factory.CreateTCPConnection(conn)
}

// CreateWebSocketConnection creates a GameConnection from a WebSocket connection (for client factory)
func (cf *ClientFactory) CreateWebSocketConnection(conn interface{}) (GameConnection, error) {
	return cf.factory.CreateWebSocketConnection(conn)
}

// Protocol detection utilities

// IsWebSocketUpgrade checks if the connection data indicates a WebSocket upgrade request
func IsWebSocketUpgrade(data []byte) bool {
	dataStr := strings.ToLower(string(data))
	return strings.Contains(dataStr, "upgrade: websocket") ||
		   (strings.Contains(dataStr, "connection:") && strings.Contains(dataStr, "upgrade"))
}

// IsHTTPRequest checks if the data looks like an HTTP request
func IsHTTPRequest(data []byte) bool {
	dataStr := string(data)
	httpMethods := []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH "}
	
	for _, method := range httpMethods {
		if strings.HasPrefix(dataStr, method) {
			return true
		}
	}
	return false
}

// GetHTTPUpgradeProtocol extracts the upgrade protocol from HTTP headers
func GetHTTPUpgradeProtocol(data []byte) string {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.ToLower(line))
		if strings.HasPrefix(line, "upgrade:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}