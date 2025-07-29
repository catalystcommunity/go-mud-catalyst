package connection

import (
	"fmt"
	"sync"
	"time"
)

// DefaultConnectionManager implements ConnectionManager
type DefaultConnectionManager struct {
	connections map[string]GameConnection
	connInfo    map[string]*ConnectionInfo
	mutex       sync.RWMutex
	eventHandler ConnectionEventHandler
	options     *ConnectionOptions
	wg          sync.WaitGroup
	closed      bool
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(options *ConnectionOptions, eventHandler ConnectionEventHandler) *DefaultConnectionManager {
	if options == nil {
		options = DefaultConnectionOptions()
	}
	
	return &DefaultConnectionManager{
		connections:  make(map[string]GameConnection),
		connInfo:     make(map[string]*ConnectionInfo),
		eventHandler: eventHandler,
		options:      options,
		closed:       false,
	}
}

// AddConnection adds a connection to be managed
func (cm *DefaultConnectionManager) AddConnection(id string, conn GameConnection) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	if cm.closed {
		return fmt.Errorf("connection manager is closed")
	}
	
	if _, exists := cm.connections[id]; exists {
		return fmt.Errorf("connection with ID %s already exists", id)
	}
	
	cm.connections[id] = conn
	cm.connInfo[id] = &ConnectionInfo{
		Type:        conn.ConnectionType(),
		RemoteAddr:  conn.RemoteAddrWithProtocol(),
		LocalAddr:   conn.LocalAddrWithProtocol(),
		ConnectedAt: time.Now(),
	}
	
	// Fire connection opened event
	if cm.eventHandler != nil {
		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()
			cm.eventHandler(&ConnectionEvent{
				Type:       EventConnectionOpened,
				Connection: conn,
				Timestamp:  time.Now(),
			})
		}()
	}
	
	// Start monitoring this connection
	cm.wg.Add(1)
	go cm.monitorConnection(id, conn)
	
	return nil
}

// RemoveConnection removes a connection from management
func (cm *DefaultConnectionManager) RemoveConnection(id string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	conn, exists := cm.connections[id]
	if !exists {
		return fmt.Errorf("connection with ID %s not found", id)
	}
	
	// Close the connection
	conn.Close()
	
	// Remove from maps
	delete(cm.connections, id)
	delete(cm.connInfo, id)
	
	// Fire connection closed event
	if cm.eventHandler != nil {
		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()
			cm.eventHandler(&ConnectionEvent{
				Type:       EventConnectionClosed,
				Connection: conn,
				Timestamp:  time.Now(),
			})
		}()
	}
	
	return nil
}

// GetConnection retrieves a connection by ID
func (cm *DefaultConnectionManager) GetConnection(id string) (GameConnection, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	conn, exists := cm.connections[id]
	return conn, exists
}

// ListConnections returns all active connection IDs
func (cm *DefaultConnectionManager) ListConnections() []string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	ids := make([]string, 0, len(cm.connections))
	for id := range cm.connections {
		ids = append(ids, id)
	}
	return ids
}

// GetConnectionInfo returns information about a connection
func (cm *DefaultConnectionManager) GetConnectionInfo(id string) (*ConnectionInfo, bool) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	info, exists := cm.connInfo[id]
	if !exists {
		return nil, false
	}
	
	// Update with current stats if connection exists
	if conn, connExists := cm.connections[id]; connExists {
		if tcpConn, ok := conn.(*TCPConnection); ok {
			currentInfo := tcpConn.GetConnectionInfo()
			info.BytesRead = currentInfo.BytesRead
			info.BytesWritten = currentInfo.BytesWritten
			info.MessagesRead = currentInfo.MessagesRead
			info.MessagesWritten = currentInfo.MessagesWritten
			info.LastActivity = currentInfo.LastActivity
		} else if wsConn, ok := conn.(*WebSocketConnection); ok {
			currentInfo := wsConn.GetConnectionInfo()
			info.BytesRead = currentInfo.BytesRead
			info.BytesWritten = currentInfo.BytesWritten
			info.MessagesRead = currentInfo.MessagesRead
			info.MessagesWritten = currentInfo.MessagesWritten
			info.LastActivity = currentInfo.LastActivity
		}
	}
	
	return info, true
}

// Broadcast sends a message to all connections
func (cm *DefaultConnectionManager) Broadcast(data []byte) error {
	cm.mutex.RLock()
	connections := make([]GameConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		if conn.IsConnected() {
			connections = append(connections, conn)
		}
	}
	cm.mutex.RUnlock()
	
	var lastErr error
	successCount := 0
	
	for _, conn := range connections {
		if err := conn.WriteMessage(data); err != nil {
			lastErr = err
			// Fire error event
			if cm.eventHandler != nil {
				cm.wg.Add(1)
				go func() {
					defer cm.wg.Done()
					cm.eventHandler(&ConnectionEvent{
						Type:       EventConnectionError,
						Connection: conn,
						Error:      err,
						Timestamp:  time.Now(),
					})
				}()
			}
		} else {
			successCount++
			// Fire message sent event
			if cm.eventHandler != nil {
				cm.wg.Add(1)
				go func() {
					defer cm.wg.Done()
					cm.eventHandler(&ConnectionEvent{
						Type:       EventMessageSent,
						Connection: conn,
						Data:       data,
						Timestamp:  time.Now(),
					})
				}()
			}
		}
	}
	
	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("broadcast failed to all connections, last error: %v", lastErr)
	}
	
	return nil
}

// BroadcastToType sends a message to all connections of a specific type
func (cm *DefaultConnectionManager) BroadcastToType(connType ConnectionType, data []byte) error {
	cm.mutex.RLock()
	connections := make([]GameConnection, 0)
	for _, conn := range cm.connections {
		if conn.IsConnected() && conn.ConnectionType() == connType {
			connections = append(connections, conn)
		}
	}
	cm.mutex.RUnlock()
	
	var lastErr error
	successCount := 0
	
	for _, conn := range connections {
		if err := conn.WriteMessage(data); err != nil {
			lastErr = err
			// Fire error event
			if cm.eventHandler != nil {
				cm.wg.Add(1)
				go func() {
					defer cm.wg.Done()
					cm.eventHandler(&ConnectionEvent{
						Type:       EventConnectionError,
						Connection: conn,
						Error:      err,
						Timestamp:  time.Now(),
					})
				}()
			}
		} else {
			successCount++
			// Fire message sent event
			if cm.eventHandler != nil {
				cm.wg.Add(1)
				go func() {
					defer cm.wg.Done()
					cm.eventHandler(&ConnectionEvent{
						Type:       EventMessageSent,
						Connection: conn,
						Data:       data,
						Timestamp:  time.Now(),
					})
				}()
			}
		}
	}
	
	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("broadcast to %s connections failed, last error: %v", connType, lastErr)
	}
	
	return nil
}

// Close closes all managed connections
func (cm *DefaultConnectionManager) Close() error {
	cm.mutex.Lock()
	if cm.closed {
		cm.mutex.Unlock()
		return nil
	}
	cm.closed = true
	
	var lastErr error
	
	for id, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(cm.connections, id)
		delete(cm.connInfo, id)
	}
	cm.mutex.Unlock()
	
	// Wait for all monitoring goroutines to finish
	cm.wg.Wait()
	
	return lastErr
}

// ConnectionCount returns the number of active connections
func (cm *DefaultConnectionManager) ConnectionCount() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	count := 0
	for _, conn := range cm.connections {
		if conn.IsConnected() {
			count++
		}
	}
	return count
}

// ConnectionCountByType returns the number of connections by type
func (cm *DefaultConnectionManager) ConnectionCountByType() map[ConnectionType]int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	counts := make(map[ConnectionType]int)
	for _, conn := range cm.connections {
		if conn.IsConnected() {
			counts[conn.ConnectionType()]++
		}
	}
	return counts
}

// monitorConnection monitors a connection for disconnection and events
func (cm *DefaultConnectionManager) monitorConnection(id string, conn GameConnection) {
	defer cm.wg.Done() // Signal completion when goroutine exits
	
	ctx := conn.Context()
	
	// Wait for connection to close
	<-ctx.Done()
	
	// Remove from manager when connection closes (only if still exists)
	cm.mutex.Lock()
	if cm.closed {
		cm.mutex.Unlock()
		return
	}
	_, exists := cm.connections[id]
	if exists {
		delete(cm.connections, id)
		delete(cm.connInfo, id)
	}
	cm.mutex.Unlock()
	
	// Fire connection closed event (only if we removed it)
	if exists && cm.eventHandler != nil {
		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()
			cm.eventHandler(&ConnectionEvent{
				Type:       EventConnectionClosed,
				Connection: conn,
				Timestamp:  time.Now(),
			})
		}()
	}
}

// SendMessage sends a message to a specific connection
func (cm *DefaultConnectionManager) SendMessage(id string, data []byte) error {
	conn, exists := cm.GetConnection(id)
	if !exists {
		return fmt.Errorf("connection with ID %s not found", id)
	}
	
	if !conn.IsConnected() {
		return fmt.Errorf("connection with ID %s is not connected", id)
	}
	
	err := conn.WriteMessage(data)
	if err != nil {
		// Fire error event
		if cm.eventHandler != nil {
			cm.wg.Add(1)
			go func() {
				defer cm.wg.Done()
				cm.eventHandler(&ConnectionEvent{
					Type:       EventConnectionError,
					Connection: conn,
					Error:      err,
					Timestamp:  time.Now(),
				})
			}()
		}
		return err
	}
	
	// Fire message sent event
	if cm.eventHandler != nil {
		cm.wg.Add(1)
		go func() {
			defer cm.wg.Done()
			cm.eventHandler(&ConnectionEvent{
				Type:       EventMessageSent,
				Connection: conn,
				Data:       data,
				Timestamp:  time.Now(),
			})
		}()
	}
	
	return nil
}

// GetConnectionsByType returns all connections of a specific type
func (cm *DefaultConnectionManager) GetConnectionsByType(connType ConnectionType) map[string]GameConnection {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	
	result := make(map[string]GameConnection)
	for id, conn := range cm.connections {
		if conn.ConnectionType() == connType {
			result[id] = conn
		}
	}
	return result
}

// UpdateConnectionInfo updates the stored connection info
func (cm *DefaultConnectionManager) UpdateConnectionInfo(id string) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	conn, exists := cm.connections[id]
	if !exists {
		return
	}
	
	info, exists := cm.connInfo[id]
	if !exists {
		return
	}
	
	// Update with current stats
	if tcpConn, ok := conn.(*TCPConnection); ok {
		currentInfo := tcpConn.GetConnectionInfo()
		info.BytesRead = currentInfo.BytesRead
		info.BytesWritten = currentInfo.BytesWritten
		info.MessagesRead = currentInfo.MessagesRead
		info.MessagesWritten = currentInfo.MessagesWritten
		info.LastActivity = currentInfo.LastActivity
	} else if wsConn, ok := conn.(*WebSocketConnection); ok {
		currentInfo := wsConn.GetConnectionInfo()
		info.BytesRead = currentInfo.BytesRead
		info.BytesWritten = currentInfo.BytesWritten
		info.MessagesRead = currentInfo.MessagesRead
		info.MessagesWritten = currentInfo.MessagesWritten
		info.LastActivity = currentInfo.LastActivity
	}
}