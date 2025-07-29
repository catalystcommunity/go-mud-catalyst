package connection

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionState represents the current state of a WebSocket connection
type ConnectionState int32

const (
	StateConnecting ConnectionState = iota
	StateConnected
	StateClosing
	StateClosed
)

// WebSocketConnection wraps a WebSocket net.Conn to implement GameConnection
type WebSocketConnection struct {
	conn            net.Conn
	ctx             context.Context
	cancel          context.CancelFunc
	state           int32 // atomic ConnectionState
	readTimeout     time.Duration
	writeTimeout    time.Duration
	bytesRead       uint64
	bytesWritten    uint64
	messagesRead    uint64
	messagesWritten uint64
	lastActivity    time.Time
	mutex           sync.RWMutex
	options         *ConnectionOptions
	readBuffer      []byte
	bufferedReader  *bufio.Reader // Optional buffered reader for small frequent reads
}

// NewWebSocketConnection creates a new WebSocket GameConnection wrapper
func NewWebSocketConnection(conn net.Conn, options *ConnectionOptions) *WebSocketConnection {
	if options == nil {
		options = DefaultConnectionOptions()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	wsc := &WebSocketConnection{
		conn:           conn,
		ctx:            ctx,
		cancel:         cancel,
		state:          int32(StateConnected),
		readTimeout:    options.ReadTimeout,
		writeTimeout:   options.WriteTimeout,
		lastActivity:   time.Now(),
		options:        options,
		readBuffer:     make([]byte, options.ReadBufferSize),
		bufferedReader: bufio.NewReader(conn), // Initialize buffered reader for better performance
	}
	
	// Start connection monitoring
	go wsc.monitor()
	
	return wsc
}

// ConnectionType returns the connection type
func (wsc *WebSocketConnection) ConnectionType() ConnectionType {
	return ConnectionTypeWebSocket
}

// IsConnected returns true if the connection is active
func (wsc *WebSocketConnection) IsConnected() bool {
	state := ConnectionState(atomic.LoadInt32(&wsc.state))
	return state == StateConnected
}

// GetState returns the current connection state
func (wsc *WebSocketConnection) GetState() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&wsc.state))
}

// setState atomically sets the connection state
func (wsc *WebSocketConnection) setState(newState ConnectionState) bool {
	oldState := ConnectionState(atomic.LoadInt32(&wsc.state))
	
	// Valid state transitions
	validTransitions := map[ConnectionState][]ConnectionState{
		StateConnecting: {StateConnected, StateClosed},
		StateConnected:  {StateClosing, StateClosed},
		StateClosing:    {StateClosed},
		StateClosed:     {}, // No transitions from closed state
	}
	
	// Check if transition is valid
	if allowed, exists := validTransitions[oldState]; exists {
		for _, allowedState := range allowed {
			if allowedState == newState {
				return atomic.CompareAndSwapInt32(&wsc.state, int32(oldState), int32(newState))
			}
		}
	}
	
	return false
}

// WriteMessage writes a complete message using WebSocket framing
func (wsc *WebSocketConnection) WriteMessage(data []byte) error {
	if !wsc.IsConnected() {
		return fmt.Errorf("connection is closed")
	}
	
	wsc.mutex.Lock()
	defer wsc.mutex.Unlock()
	
	if wsc.writeTimeout > 0 {
		wsc.conn.SetWriteDeadline(time.Now().Add(wsc.writeTimeout))
	}
	
	// WebSocket handles message framing automatically
	n, err := wsc.conn.Write(data)
	if err != nil {
		wsc.close()
		// Enhanced error handling per websocks documentation
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("websocket write timeout: %w", err)
		}
		return fmt.Errorf("websocket write error: %w", err)
	}
	
	atomic.AddUint64(&wsc.bytesWritten, uint64(n))
	atomic.AddUint64(&wsc.messagesWritten, 1)
	wsc.lastActivity = time.Now()
	
	return nil
}

// ReadMessage reads a complete WebSocket message
func (wsc *WebSocketConnection) ReadMessage() ([]byte, error) {
	if !wsc.IsConnected() {
		return nil, fmt.Errorf("connection is closed")
	}
	
	if wsc.readTimeout > 0 {
		wsc.conn.SetReadDeadline(time.Now().Add(wsc.readTimeout))
	}
	
	// Read the message using a buffer
	var messageData []byte
	buffer := make([]byte, wsc.options.ReadBufferSize)
	
	for {
		n, err := wsc.conn.Read(buffer)
		if err != nil {
			if err == io.EOF && len(messageData) > 0 {
				// Complete message received
				break
			}
			wsc.close()
			// Enhanced error handling per websocks documentation
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("websocket read timeout: %w", err)
			}
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("websocket connection closed by remote peer: %w", err)
			}
			return nil, fmt.Errorf("websocket read error: %w", err)
		}
		
		messageData = append(messageData, buffer[:n]...)
		
		// Check if we've exceeded max message size
		if wsc.options.MaxMessageSize > 0 && int64(len(messageData)) > wsc.options.MaxMessageSize {
			wsc.close()
			return nil, fmt.Errorf("message size exceeds maximum allowed size")
		}
		
		// For WebSocket, each read operation typically returns a complete message
		// The websocks package should handle message boundaries for us
		if n < len(buffer) {
			// Likely a complete message
			break
		}
	}
	
	atomic.AddUint64(&wsc.bytesRead, uint64(len(messageData)))
	atomic.AddUint64(&wsc.messagesRead, 1)
	wsc.mutex.Lock()
	wsc.lastActivity = time.Now()
	wsc.mutex.Unlock()
	
	return messageData, nil
}

// SetReadTimeout sets the read timeout
func (wsc *WebSocketConnection) SetReadTimeout(timeout time.Duration) error {
	wsc.mutex.Lock()
	defer wsc.mutex.Unlock()
	wsc.readTimeout = timeout
	return nil
}

// SetWriteTimeout sets the write timeout
func (wsc *WebSocketConnection) SetWriteTimeout(timeout time.Duration) error {
	wsc.mutex.Lock()
	defer wsc.mutex.Unlock()
	wsc.writeTimeout = timeout
	return nil
}

// Context returns the connection context
func (wsc *WebSocketConnection) Context() context.Context {
	return wsc.ctx
}

// RemoteAddrWithProtocol returns remote address with WebSocket prefix
func (wsc *WebSocketConnection) RemoteAddrWithProtocol() string {
	return fmt.Sprintf("websocket://%s", wsc.conn.RemoteAddr().String())
}

// LocalAddrWithProtocol returns local address with WebSocket prefix
func (wsc *WebSocketConnection) LocalAddrWithProtocol() string {
	return fmt.Sprintf("websocket://%s", wsc.conn.LocalAddr().String())
}

// Read implements net.Conn.Read
func (wsc *WebSocketConnection) Read(b []byte) (n int, err error) {
	if !wsc.IsConnected() {
		return 0, fmt.Errorf("connection is closed")
	}
	
	n, err = wsc.conn.Read(b)
	if err != nil {
		wsc.close()
	} else {
		atomic.AddUint64(&wsc.bytesRead, uint64(n))
		wsc.mutex.Lock()
		wsc.lastActivity = time.Now()
		wsc.mutex.Unlock()
	}
	return n, err
}

// Write implements net.Conn.Write
func (wsc *WebSocketConnection) Write(b []byte) (n int, err error) {
	if !wsc.IsConnected() {
		return 0, fmt.Errorf("connection is closed")
	}
	
	n, err = wsc.conn.Write(b)
	if err != nil {
		wsc.close()
	} else {
		atomic.AddUint64(&wsc.bytesWritten, uint64(n))
		wsc.mutex.Lock()
		wsc.lastActivity = time.Now()
		wsc.mutex.Unlock()
	}
	return n, err
}

// Close implements net.Conn.Close
func (wsc *WebSocketConnection) Close() error {
	return wsc.close()
}

// WriteCloseMessage sends a WebSocket close frame gracefully
func (wsc *WebSocketConnection) WriteCloseMessage() error {
	if !wsc.IsConnected() {
		return fmt.Errorf("connection is closed")
	}
	
	// For WebSocket connections, we should send a proper close frame
	// This is a simplified version - in a real implementation, we'd need
	// to format a proper WebSocket close frame with status code
	closeFrame := []byte{0x88, 0x00} // WebSocket close frame (FIN=1, opcode=8, no payload)
	
	wsc.mutex.Lock()
	defer wsc.mutex.Unlock()
	
	if wsc.writeTimeout > 0 {
		wsc.conn.SetWriteDeadline(time.Now().Add(wsc.writeTimeout))
	}
	
	_, err := wsc.conn.Write(closeFrame)
	return err
}

// close internal close method
func (wsc *WebSocketConnection) close() error {
	// Try to transition to closing state first
	if !wsc.setState(StateClosing) {
		// If we can't transition to closing, check if already closed
		if wsc.GetState() == StateClosed {
			return nil // already closed
		}
		// Force close if in invalid state
		atomic.StoreInt32(&wsc.state, int32(StateClosed))
	}
	
	// Cancel context to signal shutdown
	wsc.cancel()
	
	// Actually close the connection
	err := wsc.conn.Close()
	
	// Set final state
	atomic.StoreInt32(&wsc.state, int32(StateClosed))
	
	return err
}

// LocalAddr implements net.Conn.LocalAddr
func (wsc *WebSocketConnection) LocalAddr() net.Addr {
	return wsc.conn.LocalAddr()
}

// RemoteAddr implements net.Conn.RemoteAddr
func (wsc *WebSocketConnection) RemoteAddr() net.Addr {
	return wsc.conn.RemoteAddr()
}

// SetDeadline implements net.Conn.SetDeadline
func (wsc *WebSocketConnection) SetDeadline(t time.Time) error {
	return wsc.conn.SetDeadline(t)
}

// SetReadDeadline implements net.Conn.SetReadDeadline
func (wsc *WebSocketConnection) SetReadDeadline(t time.Time) error {
	return wsc.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline
func (wsc *WebSocketConnection) SetWriteDeadline(t time.Time) error {
	return wsc.conn.SetWriteDeadline(t)
}

// GetConnectionInfo returns connection statistics
func (wsc *WebSocketConnection) GetConnectionInfo() *ConnectionInfo {
	wsc.mutex.RLock()
	defer wsc.mutex.RUnlock()
	
	return &ConnectionInfo{
		Type:            ConnectionTypeWebSocket,
		RemoteAddr:      wsc.RemoteAddrWithProtocol(),
		LocalAddr:       wsc.LocalAddrWithProtocol(),
		LastActivity:    wsc.lastActivity,
		BytesRead:       atomic.LoadUint64(&wsc.bytesRead),
		BytesWritten:    atomic.LoadUint64(&wsc.bytesWritten),
		MessagesRead:    atomic.LoadUint64(&wsc.messagesRead),
		MessagesWritten: atomic.LoadUint64(&wsc.messagesWritten),
	}
}

// monitor runs connection health monitoring
func (wsc *WebSocketConnection) monitor() {
	ticker := time.NewTicker(wsc.options.PingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-wsc.ctx.Done():
			return
		case <-ticker.C:
			// For WebSocket connections, we can implement ping/pong
			// For now, just check if connection is still alive
			if err := wsc.conn.SetDeadline(time.Now().Add(time.Nanosecond)); err != nil {
				wsc.close()
				return
			}
			// Reset deadline
			wsc.conn.SetDeadline(time.Time{})
		}
	}
}