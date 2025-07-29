package race_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/client"
	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/logging"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/server"
	"github.com/stretchr/testify/assert"
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

// createHighCapacityServer creates a server with higher connection limits for stress testing
func createHighCapacityServer(host, port string) *server.ConnServer {
	config := server.DefaultServerConfig()
	config.Host = host
	config.Port = port
	config.MaxConnectionsPerIP = 500    // Increase from default 10 to 500
	config.MaxConnectionsPerSecond = 1000 // Increase rate limit
	return server.NewServerWithConfig(config)
}

// createHighCapacityTCPAndWebSocketServer creates a dual-protocol server with higher limits
func createHighCapacityTCPAndWebSocketServer(host, tcpPort, wsPort string) *server.ConnServer {
	config := server.DefaultServerConfig()
	config.Host = host
	config.Port = tcpPort
	config.WebSocketPort = wsPort
	config.EnableWebSocket = true      // Enable WebSocket support
	config.MaxConnectionsPerIP = 500    // Increase from default 10 to 500
	config.MaxConnectionsPerSecond = 1000 // Increase rate limit
	return server.NewServerWithConfig(config)
}

// TestClientConnectRaceCondition specifically tests the race condition
// found in the client Connect method where multiple goroutines access
// the err variable without synchronization
func TestClientConnectRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8090")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond) // Give server time to start

	// Test concurrent connection attempts to trigger the race condition
	var wg sync.WaitGroup
	numClients := 50
	
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("localhost:8090")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			config.ConnectTimeout = 2 * time.Second
			
			c := client.NewClientWithConfig(config)
			
			// Rapidly connect and disconnect to trigger race
			err := c.Connect()
			if err != nil {
				t.Logf("Client %d connection failed: %v", clientID, err)
			} else {
				// Immediately disconnect to create more stress
				c.Close()
			}
		}(i)
	}
	
	wg.Wait()
}

// TestClientHeartbeatRaceCondition tests the race condition in heartbeat
// management where multiple goroutines access heartbeatTicker
func TestClientHeartbeatRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8091")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Create multiple clients to test concurrent heartbeat operations
	var wg sync.WaitGroup
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			
			// Create separate client instance for each goroutine
			config := client.DefaultClientConfig("localhost:8091")
			config.HeartbeatEnabled = true
			config.HeartbeatInterval = 50 * time.Millisecond // Fast heartbeat to trigger race
			
			testClient := client.NewClientWithConfig(config)
			
			err := testClient.Connect()
			if err != nil {
				t.Logf("Iteration %d: Connect failed: %v", iteration, err)
				return
			}
			
			// Let heartbeat start
			time.Sleep(25 * time.Millisecond)
			
			// Disconnect immediately
			testClient.Close()
			
			// Small delay before next iteration
			time.Sleep(10 * time.Millisecond)
		}(i)
	}
	
	wg.Wait()
}

// TestConcurrentServerClientManagement tests race conditions in 
// server client management operations
func TestConcurrentServerClientManagement(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8092")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test concurrent client connections and disconnections
	var wg sync.WaitGroup
	numOperations := 100
	
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("localhost:8092")
			config.ReconnectEnabled = false
			config.ConnectTimeout = 1 * time.Second
			
			c := client.NewClientWithConfig(config)
			
			// Connect
			err := c.Connect()
			if err != nil {
				t.Logf("Operation %d: Connect failed: %v", opID, err)
				return
			}
			
			// Send a message to ensure the client is fully registered
			msg := message.Chat().
				FromUser("racetester").
				WithText("Race test message").
				Build()
			
			sendErr := c.SendMessage(msg)
			if sendErr != nil {
				t.Logf("Operation %d: Send message failed: %v", opID, sendErr)
			}
			
			// Disconnect
			c.Close()
		}(i)
	}
	
	wg.Wait()
}

// TestConcurrentMessageSending tests race conditions in message sending
func TestConcurrentMessageSending(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8093")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	config := client.DefaultClientConfig("localhost:8093")
	config.ReconnectEnabled = false
	
	c := client.NewClientWithConfig(config)
	err = c.Connect()
	assert.NoError(t, err)
	defer c.Close()

	// Wait for connection to be established
	time.Sleep(100 * time.Millisecond)

	// Test concurrent message sending
	var wg sync.WaitGroup
	numMessages := 100
	
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(msgID int) {
			defer wg.Done()
			
			msg := message.Chat().
				FromUser("racetester").
				WithText("Concurrent message").
				Build()
			
			err := c.SendMessage(msg)
			if err != nil {
				t.Logf("Message %d send failed: %v", msgID, err)
			}
		}(i)
	}
	
	wg.Wait()
}

// TestServerShutdownRaceCondition tests race conditions during server shutdown
func TestServerShutdownRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8094")
	err := srv.StartServer()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Connect multiple clients
	var clients []*client.Client
	var clientsMutex sync.Mutex
	var wg sync.WaitGroup
	
	numClients := 20
	
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("localhost:8094")
			config.ReconnectEnabled = false
			
			c := client.NewClientWithConfig(config)
			
			clientsMutex.Lock()
			clients = append(clients, c)
			clientsMutex.Unlock()
			
			err := c.Connect()
			if err != nil {
				t.Logf("Client %d connect failed: %v", clientID, err)
				return
			}
			
			// Start sending messages continuously
			go func() {
				for {
					msg := message.Chat().
						FromUser("shutdowntester").
						WithText("Pre-shutdown message").
						Build()
					
					err := c.SendMessage(msg)
					if err != nil {
						// Expected during shutdown
						return
					}
					
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}(i)
	}
	
	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Let clients stabilize
	
	// Now shutdown server while clients are active
	srv.Shutdown()
	
	// Clean up clients
	clientsMutex.Lock()
	for _, c := range clients {
		if c != nil {
			c.Close()
		}
	}
	clientsMutex.Unlock()
}

// TestConnectionManagerRaceCondition tests race conditions in connection management
func TestConnectionManagerRaceCondition(t *testing.T) {
	srv := createHighCapacityTCPAndWebSocketServer("localhost", "8095", "8096")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test concurrent connections of different types
	var wg sync.WaitGroup
	numTCPClients := 25
	numWSClients := 25
	
	// TCP clients
	for i := 0; i < numTCPClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("localhost:8095")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			
			c := client.NewClientWithConfig(config)
			defer c.Close()
			
			err := c.Connect()
			if err != nil {
				t.Logf("TCP Client %d connect failed: %v", clientID, err)
				return
			}
			
			// Brief activity
			time.Sleep(50 * time.Millisecond)
		}(i)
	}
	
	// WebSocket clients
	for i := 0; i < numWSClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("ws://localhost:8096/ws")
			config.ConnectionType = connection.ConnectionTypeWebSocket
			config.ReconnectEnabled = false
			
			c := client.NewClientWithConfig(config)
			defer c.Close()
			
			err := c.Connect()
			if err != nil {
				t.Logf("WS Client %d connect failed: %v", clientID, err)
				return
			}
			
			// Brief activity
			time.Sleep(50 * time.Millisecond)
		}(i)
	}
	
	wg.Wait()
}

// TestReconnectHandlerRaceCondition tests the race condition in reconnect logic
func TestReconnectHandlerRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8097")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	config := client.DefaultClientConfig("localhost:8097")
	config.ReconnectEnabled = true
	config.MaxReconnectDelay = 100 * time.Millisecond
	config.MaxRetries = 3
	
	c := client.NewClientWithConfig(config)
	defer c.Close()

	// Test concurrent reconnect attempts
	var wg sync.WaitGroup
	
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			
			// Connect
			err := c.Connect()
			if err != nil {
				t.Logf("Iteration %d: Connect failed: %v", iteration, err)
				return
			}
			
			// Force disconnect to trigger reconnect
			c.Close()
			
			// Small delay
			time.Sleep(50 * time.Millisecond)
		}(i)
	}
	
	wg.Wait()
}

// TestStateConcurrentAccess tests concurrent state modifications
func TestStateConcurrentAccess(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8098")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test concurrent state reads while one client connects/disconnects
	var wg sync.WaitGroup
	numReaders := 20
	numOperations := 10
	numClients := 3
	
	// Create multiple clients to test state access concurrency
	var clients []*client.Client
	for i := 0; i < numClients; i++ {
		config := client.DefaultClientConfig("localhost:8098")
		config.ReconnectEnabled = false
		c := client.NewClientWithConfig(config)
		clients = append(clients, c)
	}
	
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()
	
	// State readers across multiple clients
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			
			clientIdx := readerID % len(clients)
			c := clients[clientIdx]
			
			for j := 0; j < numOperations; j++ {
				state := c.GetState()
				_ = state // Use the state to prevent optimization
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}
	
	// State modifiers (each client gets connected/disconnected once)
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			c := clients[clientID]
			
			err := c.Connect()
			if err != nil {
				t.Logf("Client %d connect failed: %v", clientID, err)
				return
			}
			
			time.Sleep(10 * time.Millisecond)
			c.Close()
			time.Sleep(10 * time.Millisecond)
		}(i)
	}
	
	wg.Wait()
}

// TestMessageQueueRaceCondition tests race conditions in message queue operations
func TestMessageQueueRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8099")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	config := client.DefaultClientConfig("localhost:8099")
	config.MessageQueueSize = 100
	config.ReconnectEnabled = false
	
	c := client.NewClientWithConfig(config)
	defer c.Close()
	
	err = c.Connect()
	assert.NoError(t, err)

	// Test concurrent queue operations
	var wg sync.WaitGroup
	numProducers := 10
	messagesPerProducer := 20
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Message producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerProducer; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					msg := message.Chat().
						FromUser("producer").
						WithText("Queue race test").
						Build()
					
					err := c.SendMessage(msg)
					if err != nil {
						t.Logf("Producer %d message %d failed: %v", producerID, j, err)
					}
					
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}
	
	// Message consumers (via message handler)
	var messagesReceived int64
	c.SetMessageHandler(func(msg *message.Message) {
		// Simulate processing time
		time.Sleep(1 * time.Millisecond)
		atomic.AddInt64(&messagesReceived, 1)
	})
	
	wg.Wait()
	
	// Give time for message processing
	time.Sleep(500 * time.Millisecond)
	
	// Messages received count available in variable for assertions
}

// TestServerClientManagementRaceCondition tests race conditions in server client management
func TestServerClientManagementRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8100")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test concurrent client connections, disconnections, and management operations
	var wg sync.WaitGroup
	numOperations := 200
	
	// Track metrics atomically to avoid races in test code
	var (
		successfulConnections int64
		failedConnections     int64
		successfulDisconnections int64
		totalOperations       int64
	)

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()
			defer func() { atomic.AddInt64(&totalOperations, 1) }()
			
			config := client.DefaultClientConfig("localhost:8100")
			config.ReconnectEnabled = false
			config.ConnectTimeout = 3 * time.Second
			config.HeartbeatEnabled = false // Reduce complexity for race testing
			
			c := client.NewClientWithConfig(config)
			
			// Attempt connection
			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&failedConnections, 1)
				t.Logf("Operation %d: Connect failed: %v", opID, err)
				return
			}
			
			atomic.AddInt64(&successfulConnections, 1)
			
			// Brief activity to ensure client is fully registered
			time.Sleep(time.Duration(1+opID%10) * time.Millisecond)
			
			// Send a message to stress the system
			msg := message.Chat().
				FromUser(fmt.Sprintf("client%d", opID)).
				WithText("Client management race test").
				Build()
			
			err = c.SendMessage(msg)
			if err != nil {
				t.Logf("Operation %d: Send message failed: %v", opID, err)
			}
			
			// Small random delay before disconnect
			time.Sleep(time.Duration(1+opID%5) * time.Millisecond)
			
			// Disconnect
			c.Close()
			atomic.AddInt64(&successfulDisconnections, 1)
		}(i)
	}
	
	wg.Wait()

	// Report results
	successConns := atomic.LoadInt64(&successfulConnections)
	totalOps := atomic.LoadInt64(&totalOperations)

	// Client management race test results available in variables for assertions

	// Basic assertions
	if successConns == 0 {
		t.Error("No successful connections in client management race test")
	}

	successRate := float64(successConns) / float64(totalOps)
	if successRate < 0.8 { // Expect 80% success rate
		t.Errorf("Connection success rate too low: %.2f%% (expected >= 80%%)", successRate*100)
	}
}

// TestRoomMembershipRaceCondition tests race conditions in room join/leave operations
func TestRoomMembershipRaceCondition(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8101")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Create multiple persistent clients for room operations
	numClients := 50
	var clients []*client.Client
	
	// Connect all clients first
	for i := 0; i < numClients; i++ {
		config := client.DefaultClientConfig("localhost:8101")
		config.ReconnectEnabled = false
		config.HeartbeatEnabled = false
		
		c := client.NewClientWithConfig(config)
		err := c.Connect()
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		
		clients = append(clients, c)
		// Note: We can't easily get the server-assigned client ID, 
		// so we'll use the join/leave room message interface instead
	}
	
	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	// Let all clients settle
	time.Sleep(200 * time.Millisecond)

	// Test concurrent room join/leave operations
	var wg sync.WaitGroup
	numRoomOperations := 500
	roomIDs := []string{"lobby", "general", "test1", "test2", "test3"}
	
	var (
		roomJoinAttempts   int64
		roomLeaveAttempts  int64
		messagesSent       int64
		operationErrors    int64
	)

	for i := 0; i < numRoomOperations; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()
			
			// Pick a random client and room
			clientIdx := opID % len(clients)
			roomIdx := opID % len(roomIDs)
			c := clients[clientIdx]
			roomID := roomIDs[roomIdx]
			
			// Randomly choose between join and leave operations
			if opID%2 == 0 {
				// Join room operation
				atomic.AddInt64(&roomJoinAttempts, 1)
				msg := message.JoinRoom().
					Room(roomID, "Test Room").
					Build()
				
				err := c.SendMessage(msg)
				if err != nil {
					atomic.AddInt64(&operationErrors, 1)
					t.Logf("Join room operation %d failed: %v", opID, err)
				} else {
					atomic.AddInt64(&messagesSent, 1)
				}
			} else {
				// Leave room operation
				atomic.AddInt64(&roomLeaveAttempts, 1)
				msg := message.LeaveRoom().
					Room(roomID).
					Build()
				
				err := c.SendMessage(msg)
				if err != nil {
					atomic.AddInt64(&operationErrors, 1)
					t.Logf("Leave room operation %d failed: %v", opID, err)
				} else {
					atomic.AddInt64(&messagesSent, 1)
				}
			}
			
			// Small delay to spread operations
			time.Sleep(time.Duration(1+opID%3) * time.Millisecond)
		}(i)
	}
	
	wg.Wait()

	// Report results
	joinAttempts := atomic.LoadInt64(&roomJoinAttempts)
	leaveAttempts := atomic.LoadInt64(&roomLeaveAttempts)
	msgsSent := atomic.LoadInt64(&messagesSent)
	errors := atomic.LoadInt64(&operationErrors)

	// Room membership race test results available in variables for assertions

	// Basic assertions
	if msgsSent == 0 {
		t.Error("No room operation messages were sent")
	}

	errorRate := float64(errors) / float64(joinAttempts + leaveAttempts)
	if errorRate > 0.1 { // Allow up to 10% error rate
		t.Errorf("Room operation error rate too high: %.2f%% (expected <= 10%%)", errorRate*100)
	}
}

// TestConnectionManagerRaceConditions tests race conditions in connection management
func TestConnectionManagerRaceConditions(t *testing.T) {
	srv := createHighCapacityTCPAndWebSocketServer("localhost", "8102", "8103")
	err := srv.StartServer()
	assert.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test concurrent mixed-type connections to stress connection manager
	var wg sync.WaitGroup
	numTCPClients := 100
	numWSClients := 100
	
	var (
		tcpConnections     int64
		wsConnections      int64
		connectionErrors   int64
		managementErrors   int64
	)

	// TCP clients
	for i := 0; i < numTCPClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("localhost:8102")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			config.ConnectTimeout = 3 * time.Second
			config.HeartbeatEnabled = false
			
			c := client.NewClientWithConfig(config)
			
			c.OnError = func(err error) {
				atomic.AddInt64(&managementErrors, 1)
			}
			
			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&connectionErrors, 1)
				t.Logf("TCP Client %d connect failed: %v", clientID, err)
				return
			}
			
			atomic.AddInt64(&tcpConnections, 1)
			
			// Brief activity
			time.Sleep(time.Duration(10+clientID%20) * time.Millisecond)
			
			// Send a message to test connection manager under load
			msg := message.Chat().
				FromUser(fmt.Sprintf("tcp%d", clientID)).
				WithText("Connection manager test").
				Build()
			
			err = c.SendMessage(msg)
			if err != nil {
				atomic.AddInt64(&managementErrors, 1)
			}
			
			c.Close()
		}(i)
	}

	// WebSocket clients
	for i := 0; i < numWSClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			
			config := client.DefaultClientConfig("ws://localhost:8103/ws")
			config.ConnectionType = connection.ConnectionTypeWebSocket
			config.ReconnectEnabled = false
			config.ConnectTimeout = 3 * time.Second
			config.HeartbeatEnabled = false
			
			c := client.NewClientWithConfig(config)
			
			c.OnError = func(err error) {
				atomic.AddInt64(&managementErrors, 1)
			}
			
			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&connectionErrors, 1)
				t.Logf("WS Client %d connect failed: %v", clientID, err)
				return
			}
			
			atomic.AddInt64(&wsConnections, 1)
			
			// Brief activity
			time.Sleep(time.Duration(15+clientID%25) * time.Millisecond)
			
			// Send a message to test connection manager under load
			msg := message.Chat().
				FromUser(fmt.Sprintf("ws%d", clientID)).
				WithText("Connection manager test").
				Build()
			
			err = c.SendMessage(msg)
			if err != nil {
				atomic.AddInt64(&managementErrors, 1)
			}
			
			c.Close()
		}(i)
	}
	
	wg.Wait()

	// Report results
	tcpConns := atomic.LoadInt64(&tcpConnections)
	wsConns := atomic.LoadInt64(&wsConnections)
	mgmtErrors := atomic.LoadInt64(&managementErrors)

	// Connection manager race test results available in variables for assertions
	
	totalExpected := int64(numTCPClients + numWSClients)
	totalActual := tcpConns + wsConns
	successRate := float64(totalActual) / float64(totalExpected)

	// Basic assertions
	if totalActual == 0 {
		t.Error("No successful connections in connection manager race test")
	}

	if successRate < 0.8 { // Expect 80% success rate
		t.Errorf("Connection manager success rate too low: %.2f%% (expected >= 80%%)", successRate*100)
	}

	// Check for excessive management errors which could indicate race conditions
	errorRate := float64(mgmtErrors) / float64(totalActual)
	if errorRate > 0.1 { // More than 10% error rate suggests problems
		t.Errorf("High management error rate (%.2f%%) may indicate race conditions", errorRate*100)
	}
}