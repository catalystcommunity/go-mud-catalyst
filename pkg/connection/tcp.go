package connection

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TCPConnection wraps a TCP net.Conn to implement GameConnection
type TCPConnection struct {
	conn            net.Conn
	reader          *bufio.Reader
	writer          *bufio.Writer
	ctx             context.Context
	cancel          context.CancelFunc
	connected       int32 // atomic
	readTimeout     time.Duration
	writeTimeout    time.Duration
	bytesRead       uint64
	bytesWritten    uint64
	messagesRead    uint64
	messagesWritten uint64
	lastActivity    time.Time
	mutex           sync.RWMutex
	options         *ConnectionOptions
}

// NewTCPConnection creates a new TCP GameConnection wrapper
func NewTCPConnection(conn net.Conn, options *ConnectionOptions) *TCPConnection {
	if options == nil {
		options = DefaultConnectionOptions()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	tc := &TCPConnection{
		conn:         conn,
		reader:       bufio.NewReaderSize(conn, options.ReadBufferSize),
		writer:       bufio.NewWriterSize(conn, options.WriteBufferSize),
		ctx:          ctx,
		cancel:       cancel,
		connected:    1,
		readTimeout:  options.ReadTimeout,
		writeTimeout: options.WriteTimeout,
		lastActivity: time.Now(),
		options:      options,
	}
	
	// Start connection monitoring
	go tc.monitor()
	
	return tc
}

// ConnectionType returns the connection type
func (tc *TCPConnection) ConnectionType() ConnectionType {
	return ConnectionTypeTCP
}

// IsConnected returns true if the connection is active
func (tc *TCPConnection) IsConnected() bool {
	return atomic.LoadInt32(&tc.connected) == 1
}

// WriteMessage writes a complete message with newline termination
func (tc *TCPConnection) WriteMessage(data []byte) error {
	if !tc.IsConnected() {
		return fmt.Errorf("connection is closed")
	}
	
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	
	if tc.writeTimeout > 0 {
		tc.conn.SetWriteDeadline(time.Now().Add(tc.writeTimeout))
	}
	
	// Write data
	n, err := tc.writer.Write(data)
	if err != nil {
		tc.close()
		return err
	}
	
	// Write newline terminator
	err = tc.writer.WriteByte('\n')
	if err != nil {
		tc.close()
		return err
	}
	
	// Flush the buffer
	err = tc.writer.Flush()
	if err != nil {
		tc.close()
		return err
	}
	
	atomic.AddUint64(&tc.bytesWritten, uint64(n+1))
	atomic.AddUint64(&tc.messagesWritten, 1)
	tc.lastActivity = time.Now()
	
	return nil
}

// ReadMessage reads a complete message (until newline)
func (tc *TCPConnection) ReadMessage() ([]byte, error) {
	if !tc.IsConnected() {
		return nil, fmt.Errorf("connection is closed")
	}
	
	if tc.readTimeout > 0 {
		tc.conn.SetReadDeadline(time.Now().Add(tc.readTimeout))
	}
	
	data, err := tc.reader.ReadBytes('\n')
	if err != nil {
		tc.close()
		return nil, err
	}
	
	// Remove the trailing newline
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	
	atomic.AddUint64(&tc.bytesRead, uint64(len(data)+1))
	atomic.AddUint64(&tc.messagesRead, 1)
	tc.mutex.Lock()
	tc.lastActivity = time.Now()
	tc.mutex.Unlock()
	
	return data, nil
}

// SetReadTimeout sets the read timeout
func (tc *TCPConnection) SetReadTimeout(timeout time.Duration) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.readTimeout = timeout
	return nil
}

// SetWriteTimeout sets the write timeout
func (tc *TCPConnection) SetWriteTimeout(timeout time.Duration) error {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.writeTimeout = timeout
	return nil
}

// Context returns the connection context
func (tc *TCPConnection) Context() context.Context {
	return tc.ctx
}

// RemoteAddrWithProtocol returns remote address with TCP prefix
func (tc *TCPConnection) RemoteAddrWithProtocol() string {
	return fmt.Sprintf("tcp://%s", tc.conn.RemoteAddr().String())
}

// LocalAddrWithProtocol returns local address with TCP prefix
func (tc *TCPConnection) LocalAddrWithProtocol() string {
	return fmt.Sprintf("tcp://%s", tc.conn.LocalAddr().String())
}

// Read implements net.Conn.Read
func (tc *TCPConnection) Read(b []byte) (n int, err error) {
	if !tc.IsConnected() {
		return 0, fmt.Errorf("connection is closed")
	}
	
	n, err = tc.conn.Read(b)
	if err != nil {
		tc.close()
	} else {
		atomic.AddUint64(&tc.bytesRead, uint64(n))
		tc.mutex.Lock()
		tc.lastActivity = time.Now()
		tc.mutex.Unlock()
	}
	return n, err
}

// Write implements net.Conn.Write
func (tc *TCPConnection) Write(b []byte) (n int, err error) {
	if !tc.IsConnected() {
		return 0, fmt.Errorf("connection is closed")
	}
	
	n, err = tc.conn.Write(b)
	if err != nil {
		tc.close()
	} else {
		atomic.AddUint64(&tc.bytesWritten, uint64(n))
		tc.mutex.Lock()
		tc.lastActivity = time.Now()
		tc.mutex.Unlock()
	}
	return n, err
}

// Close implements net.Conn.Close
func (tc *TCPConnection) Close() error {
	return tc.close()
}

// close internal close method
func (tc *TCPConnection) close() error {
	if !atomic.CompareAndSwapInt32(&tc.connected, 1, 0) {
		return nil // already closed
	}
	
	tc.cancel()
	return tc.conn.Close()
}

// LocalAddr implements net.Conn.LocalAddr
func (tc *TCPConnection) LocalAddr() net.Addr {
	return tc.conn.LocalAddr()
}

// RemoteAddr implements net.Conn.RemoteAddr
func (tc *TCPConnection) RemoteAddr() net.Addr {
	return tc.conn.RemoteAddr()
}

// SetDeadline implements net.Conn.SetDeadline
func (tc *TCPConnection) SetDeadline(t time.Time) error {
	return tc.conn.SetDeadline(t)
}

// SetReadDeadline implements net.Conn.SetReadDeadline
func (tc *TCPConnection) SetReadDeadline(t time.Time) error {
	return tc.conn.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn.SetWriteDeadline
func (tc *TCPConnection) SetWriteDeadline(t time.Time) error {
	return tc.conn.SetWriteDeadline(t)
}

// GetConnectionInfo returns connection statistics
func (tc *TCPConnection) GetConnectionInfo() *ConnectionInfo {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	
	return &ConnectionInfo{
		Type:            ConnectionTypeTCP,
		RemoteAddr:      tc.RemoteAddrWithProtocol(),
		LocalAddr:       tc.LocalAddrWithProtocol(),
		LastActivity:    tc.lastActivity,
		BytesRead:       atomic.LoadUint64(&tc.bytesRead),
		BytesWritten:    atomic.LoadUint64(&tc.bytesWritten),
		MessagesRead:    atomic.LoadUint64(&tc.messagesRead),
		MessagesWritten: atomic.LoadUint64(&tc.messagesWritten),
	}
}

// monitor runs connection health monitoring
func (tc *TCPConnection) monitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-tc.ctx.Done():
			return
		case <-ticker.C:
			// Check if connection is still alive by trying to set a deadline
			if err := tc.conn.SetDeadline(time.Now().Add(time.Nanosecond)); err != nil {
				tc.close()
				return
			}
			// Reset deadline
			tc.conn.SetDeadline(time.Time{})
		}
	}
}