package load_test

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	"github.com/stretchr/testify/require"
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

// createHighCapacityServer creates a server with higher connection limits for load testing
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
	config.MaxConnectionsPerIP = 500    // Increase from default 10 to 500
	config.MaxConnectionsPerSecond = 1000 // Increase rate limit
	return server.NewServerWithConfig(config)
}

// TestConcurrentConnections tests the server's ability to handle many concurrent connections
func TestConcurrentConnections(t *testing.T) {
	tests := []struct {
		name              string
		numClients        int
		connectionType    connection.ConnectionType
		messagesPerClient int
		testDuration      time.Duration
	}{
		{
			name:              "10 TCP clients",
			numClients:        10,
			connectionType:    connection.ConnectionTypeTCP,
			messagesPerClient: 5,
			testDuration:      3 * time.Second,
		},
		{
			name:              "50 TCP clients",
			numClients:        50,
			connectionType:    connection.ConnectionTypeTCP,
			messagesPerClient: 3,
			testDuration:      3 * time.Second,
		},
		{
			name:              "10 WebSocket clients",
			numClients:        10,
			connectionType:    connection.ConnectionTypeWebSocket,
			messagesPerClient: 5,
			testDuration:      3 * time.Second,
		},
		{
			name:              "25 Mixed connections",
			numClients:        25,
			connectionType:    "mixed", // Special case for mixed testing
			messagesPerClient: 3,
			testDuration:      3 * time.Second,
		},
		{
			name:              "1000 TCP clients - High Load",
			numClients:        1000,
			connectionType:    connection.ConnectionTypeTCP,
			messagesPerClient: 1,
			testDuration:      5 * time.Second,
		},
		{
			name:              "2000 TCP clients - Extreme Load",
			numClients:        2000,
			connectionType:    connection.ConnectionTypeTCP,
			messagesPerClient: 1,
			testDuration:      5 * time.Second,
		},
		{
			name:              "500 WebSocket clients - High Load",
			numClients:        500,
			connectionType:    connection.ConnectionTypeWebSocket,
			messagesPerClient: 1,
			testDuration:      3 * time.Second,
		},
		{
			name:              "1500 Mixed connections - Extreme Load",
			numClients:        1500,
			connectionType:    "mixed",
			messagesPerClient: 1,
			testDuration:      4 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.connectionType == "mixed" {
				testMixedConnections(t, tt.numClients, tt.messagesPerClient, tt.testDuration)
			} else {
				testConcurrentConnectionLoad(t, tt.numClients, tt.connectionType, tt.messagesPerClient, tt.testDuration)
			}
		})
	}
}

func testConcurrentConnectionLoad(t *testing.T, numClients int, connType connection.ConnectionType, messagesPerClient int, duration time.Duration) {
	var srv *server.ConnServer
	var serverAddr string

	if connType == connection.ConnectionTypeWebSocket {
		// Create server with WebSocket support on port 8081
		srv = createHighCapacityTCPAndWebSocketServer("localhost", "8080", "8081")
		serverAddr = "ws://localhost:8081/ws"
	} else {
		// Create TCP-only server on port 8080
		srv = createHighCapacityServer("localhost", "8080")
		serverAddr = "localhost:8080"
	}

	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Metrics tracking
	var (
		successfulConnections int64
		failedConnections     int64
		messagesSent          int64
		messagesReceived      int64
		totalLatency          int64
		clients               []*client.Client
		clientsMu             sync.Mutex
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup

	// Create and connect clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Create client configuration
			config := client.DefaultClientConfig(serverAddr)
			config.ConnectionType = connType
			config.ReconnectEnabled = false
			config.ConnectTimeout = 5 * time.Second

			c := client.NewClientWithConfig(config)

			// Track client
			clientsMu.Lock()
			clients = append(clients, c)
			clientsMu.Unlock()

			// Set up message handler
			c.OnMessage = func(msg *message.Message) {
				atomic.AddInt64(&messagesReceived, 1)
			}

			// Connection tracking
			connected := make(chan bool, 1)
			c.OnConnect = func() {
				connected <- true
			}

			c.OnError = func(err error) {
				t.Logf("Client %d error: %v", clientID, err)
			}

			// Attempt connection
			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&failedConnections, 1)
				t.Logf("Client %d failed to connect: %v", clientID, err)
				return
			}

			// Wait for connection with timeout
			select {
			case <-connected:
				atomic.AddInt64(&successfulConnections, 1)
			case <-time.After(5 * time.Second):
				atomic.AddInt64(&failedConnections, 1)
				t.Logf("Client %d connection timeout", clientID)
				c.Disconnect()
				return
			case <-ctx.Done():
				c.Disconnect()
				return
			}

			// Send messages periodically
			// Use 80% of duration to ensure messages are sent before context timeout
			messageDuration := time.Duration(float64(duration) * 0.8)
			ticker := time.NewTicker(messageDuration / time.Duration(messagesPerClient))
			defer ticker.Stop()

			messageCount := 0
			for messageCount < messagesPerClient {
				select {
				case <-ticker.C:
					startTime := time.Now()
					msg := message.Chat().
						FromUser(fmt.Sprintf("client%d", clientID)).
						WithText(fmt.Sprintf("Load test message %d from client %d", messageCount, clientID)).
						Build()

					err := c.SendMessage(msg)
					if err != nil {
						t.Logf("Client %d failed to send message %d: %v", clientID, messageCount, err)
					} else {
						atomic.AddInt64(&messagesSent, 1)
						latency := time.Since(startTime).Nanoseconds()
						atomic.AddInt64(&totalLatency, latency)
						messageCount++
					}
				case <-ctx.Done():
					c.Disconnect()
					return
				}
			}

			// Keep connection alive for remaining duration
			<-ctx.Done()
			c.Disconnect()
		}(i)
	}

	// Wait for all clients to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All clients completed
	case <-time.After(duration + 10*time.Second):
		t.Logf("Load test timed out")
	}

	// Clean up any remaining clients
	clientsMu.Lock()
	for _, c := range clients {
		if c.GetState() == client.ClientStateConnected {
			c.Disconnect()
		}
	}
	clientsMu.Unlock()

	// Report results
	successConnections := atomic.LoadInt64(&successfulConnections)
	sentMessages := atomic.LoadInt64(&messagesSent)

	// Load test results available in variables for assertions

	// Assertions for test success
	if successConnections == 0 {
		t.Error("No successful connections made")
	}

	connectionSuccessRate := float64(successConnections) / float64(numClients)
	if connectionSuccessRate < 0.8 { // Expect at least 80% success rate
		t.Errorf("Connection success rate too low: %.2f%% (expected >= 80%%)", connectionSuccessRate*100)
	}

	if sentMessages == 0 {
		t.Error("No messages were sent")
	}
}

func testMixedConnections(t *testing.T, totalClients, messagesPerClient int, duration time.Duration) {
	// Start server with both TCP and WebSocket support
	srv := createHighCapacityTCPAndWebSocketServer("localhost", "8080", "8081")
	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start mixed server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(200 * time.Millisecond)

	// Split clients between TCP and WebSocket
	tcpClients := totalClients / 2
	wsClients := totalClients - tcpClients

	var wg sync.WaitGroup

	// Metrics
	var (
		tcpConnections int64
		wsConnections  int64
		totalMessages  int64
	)

	// Start TCP clients (connecting to port 8080)
	for i := 0; i < tcpClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			runLoadTestClient(t, clientID, "localhost:8080", connection.ConnectionTypeTCP,
				messagesPerClient, duration, &tcpConnections, &totalMessages)
		}(i)
	}

	// Start WebSocket clients (connecting to port 8081)
	for i := 0; i < wsClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			runLoadTestClient(t, tcpClients+clientID, "ws://localhost:8081/ws",
				connection.ConnectionTypeWebSocket, messagesPerClient, duration,
				&wsConnections, &totalMessages)
		}(i)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Mixed connection test completed
	case <-time.After(duration + 10*time.Second):
		t.Logf("Mixed connection test timed out")
	}

	// Mixed connection test results available in variables for assertions
}

func runLoadTestClient(t *testing.T, clientID int, address string, connType connection.ConnectionType,
	messagesPerClient int, duration time.Duration, connectionCounter, messageCounter *int64) {

	config := client.DefaultClientConfig(address)
	config.ConnectionType = connType
	config.ReconnectEnabled = false
	config.ConnectTimeout = 5 * time.Second

	c := client.NewClientWithConfig(config)
	defer c.Disconnect()

	connected := make(chan bool, 1)
	c.OnConnect = func() {
		connected <- true
	}

	err := c.Connect()
	if err != nil {
		t.Logf("Client %d (%s) failed to connect: %v", clientID, connType, err)
		return
	}

	// Wait for connection
	select {
	case <-connected:
		atomic.AddInt64(connectionCounter, 1)
	case <-time.After(5 * time.Second):
		t.Logf("Client %d (%s) connection timeout", clientID, connType)
		return
	}

	// Send messages
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	for i := 0; i < messagesPerClient; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			msg := message.Chat().
				FromUser(fmt.Sprintf("client%d", clientID)).
				WithText(fmt.Sprintf("Mixed test message %d", i)).
				Build()

			err := c.SendMessage(msg)
			if err != nil {
				t.Logf("Client %d (%s) failed to send message: %v", clientID, connType, err)
			} else {
				atomic.AddInt64(messageCounter, 1)
			}

			// Small delay between messages
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// BenchmarkConnectionThroughput benchmarks connection establishment throughput
func BenchmarkConnectionThroughput(b *testing.B) {
	srv := createHighCapacityServer("localhost", "8082")
	err := srv.StartServer()
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		clientID := 0
		for pb.Next() {
			config := client.DefaultClientConfig("localhost:8082")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false

			c := client.NewClientWithConfig(config)

			connected := make(chan bool, 1)
			c.OnConnect = func() {
				connected <- true
			}

			err := c.Connect()
			if err != nil {
				b.Logf("Client %d failed to connect: %v", clientID, err)
				continue
			}

			select {
			case <-connected:
				// Success
			case <-time.After(2 * time.Second):
				b.Logf("Client %d connection timeout", clientID)
			}

			c.Disconnect()
			clientID++
		}
	})
}

// BenchmarkMessageThroughput benchmarks message sending throughput
func BenchmarkMessageThroughput(b *testing.B) {
	srv := createHighCapacityServer("localhost", "8083")
	err := srv.StartServer()
	if err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Create a persistent connection for the benchmark
	config := client.DefaultClientConfig("localhost:8083")
	config.ConnectionType = connection.ConnectionTypeTCP
	config.ReconnectEnabled = false

	c := client.NewClientWithConfig(config)
	defer c.Disconnect()

	connected := make(chan bool, 1)
	c.OnConnect = func() {
		connected <- true
	}

	err = c.Connect()
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}

	select {
	case <-connected:
		// Connected successfully
	case <-time.After(5 * time.Second):
		b.Fatal("Connection timeout")
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		messageID := 0
		for pb.Next() {
			msg := message.Chat().
				FromUser("benchmark").
				WithText(fmt.Sprintf("Benchmark message %d", messageID)).
				Build()

			err := c.SendMessage(msg)
			if err != nil {
				b.Logf("Failed to send message %d: %v", messageID, err)
			}
			messageID++
		}
	})
}

// TestServerStability tests server stability under sustained load
func TestServerStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stability test in short mode")
	}

	srv := createHighCapacityServer("localhost", "8084")
	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Run for 30 seconds with 20 concurrent clients
	duration := 30 * time.Second
	numClients := 20

	var wg sync.WaitGroup
	var totalErrors int64

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("localhost:8084")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = true
			config.MaxReconnectDelay = 1 * time.Second

			c := client.NewClientWithConfig(config)
			defer c.Disconnect()

			c.OnError = func(err error) {
				atomic.AddInt64(&totalErrors, 1)
				t.Logf("Client %d error: %v", clientID, err)
			}

			err := c.Connect()
			if err != nil {
				t.Logf("Client %d initial connection failed: %v", clientID, err)
				return
			}

			// Send messages continuously
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			messageCount := 0
			for {
				select {
				case <-ticker.C:
					msg := message.Chat().
						FromUser(fmt.Sprintf("stable%d", clientID)).
						WithText(fmt.Sprintf("Stability message %d", messageCount)).
						Build()

					err := c.SendMessage(msg)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
					}
					messageCount++

				case <-ctx.Done():
					t.Logf("Client %d sent %d messages", clientID, messageCount)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	errors := atomic.LoadInt64(&totalErrors)
	t.Logf("Stability test completed with %d total errors", errors)

	// Some errors are acceptable under sustained load, but not too many
	if errors > int64(numClients*10) { // Allow up to 10 errors per client
		t.Errorf("Too many errors during stability test: %d", errors)
	}
}

// TestHighScaleRaceDetection specifically tests for race conditions under high load
func TestHighScaleRaceDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-scale race detection test in short mode")
	}

	srv := createHighCapacityTCPAndWebSocketServer("localhost", "9000", "9001")
	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(200 * time.Millisecond) // Give server time to start

	// Test parameters for high-scale race detection
	numTCPClients := 1000
	numWSClients := 500
	messagesPerClient := 2
	duration := 45 * time.Second

	// Metrics tracking with atomic operations to avoid races in testing code
	var (
		tcpConnections     int64
		wsConnections      int64
		tcpConnectionFails int64
		wsConnectionFails  int64
		tcpMessagesSent    int64
		wsMessagesSent     int64
		totalErrors        int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup

	// TCP clients stress test
	for i := 0; i < numTCPClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("localhost:9000")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			config.ConnectTimeout = 5 * time.Second
			config.HeartbeatEnabled = true
			config.HeartbeatInterval = 10 * time.Second

			c := client.NewClientWithConfig(config)

			// Set error handler to track issues
			c.OnError = func(err error) {
				atomic.AddInt64(&totalErrors, 1)
			}

			// Attempt connection
			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&tcpConnectionFails, 1)
				t.Logf("TCP Client %d failed to connect: %v", clientID, err)
				return
			}

			atomic.AddInt64(&tcpConnections, 1)
			defer c.Disconnect()

			// Send messages to stress the system
			for j := 0; j < messagesPerClient; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					msg := message.Chat().
						FromUser(fmt.Sprintf("tcp%d", clientID)).
						WithText(fmt.Sprintf("High-scale race test %d", j)).
						Build()

					err := c.SendMessage(msg)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						t.Logf("TCP Client %d message %d failed: %v", clientID, j, err)
					} else {
						atomic.AddInt64(&tcpMessagesSent, 1)
					}

					// Brief delay to spread load
					time.Sleep(time.Duration(100+clientID%200) * time.Millisecond)
				}
			}

			// Keep connection alive for remaining duration
			<-ctx.Done()
		}(i)
	}

	// WebSocket clients stress test
	for i := 0; i < numWSClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("ws://localhost:9001/ws")
			config.ConnectionType = connection.ConnectionTypeWebSocket
			config.ReconnectEnabled = false
			config.ConnectTimeout = 5 * time.Second
			config.HeartbeatEnabled = true
			config.HeartbeatInterval = 15 * time.Second

			c := client.NewClientWithConfig(config)

			// Set error handler to track issues
			c.OnError = func(err error) {
				atomic.AddInt64(&totalErrors, 1)
			}

			// Attempt connection
			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&wsConnectionFails, 1)
				t.Logf("WS Client %d failed to connect: %v", clientID, err)
				return
			}

			atomic.AddInt64(&wsConnections, 1)
			defer c.Disconnect()

			// Send messages to stress the system
			for j := 0; j < messagesPerClient; j++ {
				select {
				case <-ctx.Done():
					return
				default:
					msg := message.Chat().
						FromUser(fmt.Sprintf("ws%d", clientID)).
						WithText(fmt.Sprintf("High-scale WS race test %d", j)).
						Build()

					err := c.SendMessage(msg)
					if err != nil {
						atomic.AddInt64(&totalErrors, 1)
						t.Logf("WS Client %d message %d failed: %v", clientID, j, err)
					} else {
						atomic.AddInt64(&wsMessagesSent, 1)
					}

					// Brief delay to spread load
					time.Sleep(time.Duration(150+clientID%300) * time.Millisecond)
				}
			}

			// Keep connection alive for remaining duration
			<-ctx.Done()
		}(i)
	}

	// Wait for all clients to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All high-scale clients completed
	case <-time.After(duration + 30*time.Second):
		t.Logf("High-scale test timed out")
		cancel() // Signal goroutines to stop
	}

	// Collect and report results
	tcpConns := atomic.LoadInt64(&tcpConnections)
	wsConns := atomic.LoadInt64(&wsConnections)
	errors := atomic.LoadInt64(&totalErrors)

	// High-scale race detection results available in variables for assertions

	// Assertions for minimum acceptable performance
	totalExpectedConns := int64(numTCPClients + numWSClients)
	totalActualConns := tcpConns + wsConns
	successRate := float64(totalActualConns) / float64(totalExpectedConns)

	if successRate < 0.7 { // Expect at least 70% success rate under high load
		t.Errorf("Connection success rate too low under high load: %.2f%% (expected >= 70%%)", successRate*100)
	}

	if tcpConns == 0 && wsConns == 0 {
		t.Error("No successful connections made during high-scale test")
	}

	// Check for race conditions through excessive errors (logged as error if detected)
	errorRate := float64(errors) / float64(totalActualConns)
	if errorRate > 0.2 { // More than 20% error rate suggests race conditions
		t.Errorf("High error rate (%.2f%%) may indicate race conditions", errorRate*100)
	}
}

// TestConcurrentConnectionChurn tests rapid connection/disconnection cycles
func TestConcurrentConnectionChurn(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection churn test in short mode")
	}

	srv := createHighCapacityServer("localhost", "9002")
	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test rapid connect/disconnect cycles to stress connection management
	numCycles := 1000
	concurrentClients := 50

	var wg sync.WaitGroup
	var (
		successfulConnections int64
		failedConnections     int64
		totalCycles           int64
	)

	// Starting connection churn test

	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			cyclesPerWorker := numCycles / concurrentClients
			for j := 0; j < cyclesPerWorker; j++ {
				config := client.DefaultClientConfig("localhost:9002")
				config.ConnectionType = connection.ConnectionTypeTCP
				config.ReconnectEnabled = false
				config.ConnectTimeout = 2 * time.Second
				config.HeartbeatEnabled = false // Disable to reduce complexity

				c := client.NewClientWithConfig(config)

				// Quick connect
				err := c.Connect()
				if err != nil {
					atomic.AddInt64(&failedConnections, 1)
					continue
				}

				atomic.AddInt64(&successfulConnections, 1)

				// Brief activity (optional message)
				if j%10 == 0 { // Send message every 10th cycle
					msg := message.Chat().
						FromUser(fmt.Sprintf("churn%d", workerID)).
						WithText("Churn test").
						Build()
					c.SendMessage(msg) // Ignore errors for churn test
				}

				// Quick disconnect
				c.Disconnect()

				atomic.AddInt64(&totalCycles, 1)

				// Very brief pause to avoid overwhelming
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Report churn test results
	successConns := atomic.LoadInt64(&successfulConnections)
	failConns := atomic.LoadInt64(&failedConnections)

	// Connection churn test results available in variables for assertions

	// Basic assertions
	if successConns == 0 {
		t.Error("No successful connections during churn test")
	}

	successRate := float64(successConns) / float64(successConns+failConns)
	if successRate < 0.8 { // Expect 80% success rate for churn test
		t.Errorf("Connection success rate too low during churn test: %.2f%% (expected >= 80%%)", successRate*100)
	}
}

// TestTenThousandConcurrentConnections tests the server's ability to handle 10,000+ concurrent connections
func TestTenThousandConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 10K connections test in short mode")
	}

	// Create server with maximum capacity configuration
	config := server.DefaultServerConfig()
	config.Host = "localhost"
	config.Port = "9100"
	config.MaxConnectionsPerIP = 15000
	config.MaxConnectionsPerSecond = 5000
	srv := server.NewServerWithConfig(config)

	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server for 10K connections test: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(500 * time.Millisecond)

	// Test parameters
	numClients := 10000
	batchSize := 100
	testDuration := 60 * time.Second

	// Metrics tracking
	var (
		successfulConnections int64
		failedConnections     int64
		totalErrors           int64
		messagesSent          int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var wg sync.WaitGroup
	var allClients []*client.Client
	var clientsMu sync.Mutex

	// Connect clients in batches
	for batch := 0; batch < numClients/batchSize; batch++ {
		batchClients := make([]*client.Client, 0, batchSize)
		
		for i := 0; i < batchSize; i++ {
			clientID := batch*batchSize + i
			
			wg.Add(1)
			go func(id int, clients *[]*client.Client) {
				defer wg.Done()

				config := client.DefaultClientConfig("localhost:9100")
				config.ConnectionType = connection.ConnectionTypeTCP
				config.ReconnectEnabled = false
				config.ConnectTimeout = 10 * time.Second
				config.HeartbeatEnabled = false

				c := client.NewClientWithConfig(config)

				c.OnError = func(err error) {
					atomic.AddInt64(&totalErrors, 1)
				}

				connected := make(chan bool, 1)
				c.OnConnect = func() {
					connected <- true
				}

				err := c.Connect()
				if err != nil {
					atomic.AddInt64(&failedConnections, 1)
					return
				}

				select {
				case <-connected:
					atomic.AddInt64(&successfulConnections, 1)
					
					clientsMu.Lock()
					*clients = append(*clients, c)
					clientsMu.Unlock()

					// Send message from every 1000th client
					if id%1000 == 0 {
						msg := message.Chat().
							FromUser(fmt.Sprintf("client%d", id)).
							WithText(fmt.Sprintf("10K test message from client %d", id)).
							Build()
						
						if err := c.SendMessage(msg); err == nil {
							atomic.AddInt64(&messagesSent, 1)
						}
					}

				case <-time.After(15 * time.Second):
					atomic.AddInt64(&failedConnections, 1)
					c.Disconnect()
					return

				case <-ctx.Done():
					c.Disconnect()
					return
				}
			}(clientID, &batchClients)
		}

		wg.Wait()

		clientsMu.Lock()
		allClients = append(allClients, batchClients...)
		clientsMu.Unlock()

		time.Sleep(100 * time.Millisecond)

		// Stop if too many failures
		if atomic.LoadInt64(&failedConnections) > atomic.LoadInt64(&successfulConnections) && batch > 10 {
			break
		}
	}

	// Wait for remaining duration
	select {
	case <-ctx.Done():
	case <-time.After(testDuration):
	}

	// Cleanup
	clientsMu.Lock()
	for _, c := range allClients {
		if c.GetState() == client.ClientStateConnected {
			c.Disconnect()
		}
	}
	clientsMu.Unlock()

	time.Sleep(2 * time.Second)

	// Final metrics
	finalSuccess := atomic.LoadInt64(&successfulConnections)
	finalFailed := atomic.LoadInt64(&failedConnections)

	// Assertions
	require.Greater(t, finalSuccess, int64(0), "Should have successful connections")

	totalAttempts := finalSuccess + finalFailed
	if totalAttempts > 0 {
		successRate := float64(finalSuccess) / float64(totalAttempts)
		assert.GreaterOrEqual(t, successRate, 0.7, "Success rate should be >= 70%")
	}

	assert.GreaterOrEqual(t, finalSuccess, int64(5000), "Should reach at least 5000 successful connections")
}

// TestMixedProtocolStress tests heavy load with both TCP and WebSocket protocols simultaneously
func TestMixedProtocolStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mixed protocol stress test in short mode")
	}

	// Create server with both TCP and WebSocket support
	config := server.DefaultServerConfig()
	config.Host = "localhost"
	config.Port = "9200"
	config.WebSocketPort = "9201"
	config.MaxConnectionsPerIP = 8000
	config.MaxConnectionsPerSecond = 3000
	srv := server.NewServerWithConfig(config)

	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start mixed protocol server: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(500 * time.Millisecond)

	// Test parameters
	numTCPClients := 3000
	numWSClients := 2000
	messagesPerClient := 5
	testDuration := 45 * time.Second

	// Metrics tracking
	var (
		tcpConnections     int64
		wsConnections      int64
		tcpConnectionFails int64
		wsConnectionFails  int64
		tcpMessagesSent    int64
		wsMessagesSent     int64
		tcpMessagesReceived int64
		wsMessagesReceived  int64
		totalErrors        int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var wg sync.WaitGroup
	var allClients []*client.Client
	var clientsMu sync.Mutex

	// TCP clients
	for i := 0; i < numTCPClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("localhost:9200")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			config.ConnectTimeout = 8 * time.Second
			config.HeartbeatEnabled = true
			config.HeartbeatInterval = 30 * time.Second

			c := client.NewClientWithConfig(config)

			c.OnMessage = func(msg *message.Message) {
				atomic.AddInt64(&tcpMessagesReceived, 1)
			}

			c.OnError = func(err error) {
				atomic.AddInt64(&totalErrors, 1)
			}

			connected := make(chan bool, 1)
			c.OnConnect = func() {
				connected <- true
			}

			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&tcpConnectionFails, 1)
				return
			}

			select {
			case <-connected:
				atomic.AddInt64(&tcpConnections, 1)
				
				clientsMu.Lock()
				allClients = append(allClients, c)
				clientsMu.Unlock()

				// Send messages with staggered timing
				for j := 0; j < messagesPerClient; j++ {
					select {
					case <-ctx.Done():
						return
					default:
						msg := message.Chat().
							FromUser(fmt.Sprintf("tcp%d", clientID)).
							WithText(fmt.Sprintf("Mixed protocol stress test TCP %d", j)).
							Build()

						if err := c.SendMessage(msg); err == nil {
							atomic.AddInt64(&tcpMessagesSent, 1)
						}

						time.Sleep(time.Duration(2000+clientID%3000) * time.Millisecond)
					}
				}

			case <-time.After(10 * time.Second):
				atomic.AddInt64(&tcpConnectionFails, 1)
				c.Disconnect()
				return

			case <-ctx.Done():
				c.Disconnect()
				return
			}

			<-ctx.Done()
			c.Disconnect()
		}(i)
	}

	// WebSocket clients
	for i := 0; i < numWSClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("ws://localhost:9201/ws")
			config.ConnectionType = connection.ConnectionTypeWebSocket
			config.ReconnectEnabled = false
			config.ConnectTimeout = 8 * time.Second
			config.HeartbeatEnabled = true
			config.HeartbeatInterval = 30 * time.Second

			c := client.NewClientWithConfig(config)

			c.OnMessage = func(msg *message.Message) {
				atomic.AddInt64(&wsMessagesReceived, 1)
			}

			c.OnError = func(err error) {
				atomic.AddInt64(&totalErrors, 1)
			}

			connected := make(chan bool, 1)
			c.OnConnect = func() {
				connected <- true
			}

			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&wsConnectionFails, 1)
				return
			}

			select {
			case <-connected:
				atomic.AddInt64(&wsConnections, 1)
				
				clientsMu.Lock()
				allClients = append(allClients, c)
				clientsMu.Unlock()

				// Send messages with staggered timing
				for j := 0; j < messagesPerClient; j++ {
					select {
					case <-ctx.Done():
						return
					default:
						msg := message.Chat().
							FromUser(fmt.Sprintf("ws%d", clientID)).
							WithText(fmt.Sprintf("Mixed protocol stress test WS %d", j)).
							Build()

						if err := c.SendMessage(msg); err == nil {
							atomic.AddInt64(&wsMessagesSent, 1)
						}

						time.Sleep(time.Duration(2500+clientID%3500) * time.Millisecond)
					}
				}

			case <-time.After(10 * time.Second):
				atomic.AddInt64(&wsConnectionFails, 1)
				c.Disconnect()
				return

			case <-ctx.Done():
				c.Disconnect()
				return
			}

			<-ctx.Done()
			c.Disconnect()
		}(i)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(testDuration + 15*time.Second):
		cancel()
	}

	// Cleanup
	clientsMu.Lock()
	for _, c := range allClients {
		if c.GetState() == client.ClientStateConnected {
			c.Disconnect()
		}
	}
	clientsMu.Unlock()

	time.Sleep(2 * time.Second)

	// Final metrics
	finalTCPConns := atomic.LoadInt64(&tcpConnections)
	finalWSConns := atomic.LoadInt64(&wsConnections)
	finalTCPFails := atomic.LoadInt64(&tcpConnectionFails)
	finalWSFails := atomic.LoadInt64(&wsConnectionFails)
	finalTCPMsgsSent := atomic.LoadInt64(&tcpMessagesSent)
	finalWSMsgsSent := atomic.LoadInt64(&wsMessagesSent)

	// Assertions
	require.Greater(t, finalTCPConns, int64(0), "Should have TCP connections")
	require.Greater(t, finalWSConns, int64(0), "Should have WebSocket connections")

	// Check success rates for both protocols
	tcpTotal := finalTCPConns + finalTCPFails
	wsTotal := finalWSConns + finalWSFails
	
	if tcpTotal > 0 {
		tcpSuccessRate := float64(finalTCPConns) / float64(tcpTotal)
		assert.GreaterOrEqual(t, tcpSuccessRate, 0.6, "TCP success rate should be >= 60%")
	}

	if wsTotal > 0 {
		wsSuccessRate := float64(finalWSConns) / float64(wsTotal)
		assert.GreaterOrEqual(t, wsSuccessRate, 0.6, "WebSocket success rate should be >= 60%")
	}

	// Check message throughput
	assert.Greater(t, finalTCPMsgsSent, int64(0), "Should send TCP messages")
	assert.Greater(t, finalWSMsgsSent, int64(0), "Should send WebSocket messages")

	// Check combined scale
	totalConnections := finalTCPConns + finalWSConns
	assert.GreaterOrEqual(t, totalConnections, int64(3000), "Should achieve at least 3000 combined connections")
}

// TestHighFrequencyMessageScenarios tests high-frequency message scenarios
func TestHighFrequencyMessageScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-frequency message test in short mode")
	}

	// Create server
	config := server.DefaultServerConfig()
	config.Host = "localhost"
	config.Port = "9300"
	config.MaxConnectionsPerIP = 1000
	config.MaxConnectionsPerSecond = 2000
	srv := server.NewServerWithConfig(config)

	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server for high-frequency message test: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(500 * time.Millisecond)

	// Test parameters
	numClients := 200
	messagesPerSecond := 50 // Per client
	testDuration := 30 * time.Second

	// Metrics tracking
	var (
		successfulConnections int64
		failedConnections     int64
		messagesSent          int64
		messagesReceived      int64
		messageSendErrors     int64
		totalErrors           int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var wg sync.WaitGroup
	var allClients []*client.Client
	var clientsMu sync.Mutex

	// Create clients
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("localhost:9300")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			config.ConnectTimeout = 8 * time.Second
			config.HeartbeatEnabled = false // Disable to focus on message throughput

			c := client.NewClientWithConfig(config)

			c.OnMessage = func(msg *message.Message) {
				atomic.AddInt64(&messagesReceived, 1)
			}

			c.OnError = func(err error) {
				atomic.AddInt64(&totalErrors, 1)
			}

			connected := make(chan bool, 1)
			c.OnConnect = func() {
				connected <- true
			}

			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&failedConnections, 1)
				return
			}

			select {
			case <-connected:
				atomic.AddInt64(&successfulConnections, 1)
				
				clientsMu.Lock()
				allClients = append(allClients, c)
				clientsMu.Unlock()

				// High-frequency message sending
				messageInterval := time.Second / time.Duration(messagesPerSecond)
				ticker := time.NewTicker(messageInterval)
				defer ticker.Stop()

				messageCount := 0
				for {
					select {
					case <-ticker.C:
						msg := message.Chat().
							FromUser(fmt.Sprintf("hf%d", clientID)).
							WithText(fmt.Sprintf("High-frequency message %d", messageCount)).
							Build()

						if err := c.SendMessage(msg); err != nil {
							atomic.AddInt64(&messageSendErrors, 1)
						} else {
							atomic.AddInt64(&messagesSent, 1)
						}
						messageCount++

					case <-ctx.Done():
						c.Disconnect()
						return
					}
				}

			case <-time.After(10 * time.Second):
				atomic.AddInt64(&failedConnections, 1)
				c.Disconnect()
				return

			case <-ctx.Done():
				c.Disconnect()
				return
			}
		}(i)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(testDuration + 10*time.Second):
		cancel()
	}

	// Cleanup
	clientsMu.Lock()
	for _, c := range allClients {
		if c.GetState() == client.ClientStateConnected {
			c.Disconnect()
		}
	}
	clientsMu.Unlock()

	time.Sleep(2 * time.Second)

	// Final metrics
	finalSuccess := atomic.LoadInt64(&successfulConnections)
	finalFailed := atomic.LoadInt64(&failedConnections)
	finalSent := atomic.LoadInt64(&messagesSent)
	finalSendErrors := atomic.LoadInt64(&messageSendErrors)
	_ = atomic.LoadInt64(&messagesReceived) // Not used in assertions

	// Assertions
	require.Greater(t, finalSuccess, int64(0), "Should have successful connections")

	// Check connection success rate
	totalAttempts := finalSuccess + finalFailed
	if totalAttempts > 0 {
		successRate := float64(finalSuccess) / float64(totalAttempts)
		assert.GreaterOrEqual(t, successRate, 0.8, "Connection success rate should be >= 80%")
	}

	// Check message throughput
	assert.Greater(t, finalSent, int64(0), "Should send messages")
	
	// Calculate expected messages and check we're in reasonable range
	expectedMessages := int64(finalSuccess) * int64(messagesPerSecond) * int64(testDuration.Seconds())
	assert.GreaterOrEqual(t, finalSent, expectedMessages/3, "Should send at least 1/3 of expected messages")

	// Check error rate is reasonable
	totalMessageAttempts := finalSent + finalSendErrors
	if totalMessageAttempts > 0 {
		errorRate := float64(finalSendErrors) / float64(totalMessageAttempts)
		assert.LessOrEqual(t, errorRate, 0.3, "Message error rate should be <= 30%")
	}

	// Check message throughput rate
	actualRate := float64(finalSent) / testDuration.Seconds()
	minExpectedRate := float64(finalSuccess) * float64(messagesPerSecond) * 0.3 // 30% of target
	assert.GreaterOrEqual(t, actualRate, minExpectedRate, "Should achieve minimum message rate")
}

// TestResourceExhaustion tests behavior at system limits
func TestResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	// Create server with intentionally low limits to trigger exhaustion
	config := server.DefaultServerConfig()
	config.Host = "localhost"
	config.Port = "9400"
	config.MaxConnectionsPerIP = 50 // Low limit to trigger exhaustion
	config.MaxConnectionsPerSecond = 100
	srv := server.NewServerWithConfig(config)

	err := srv.StartServer()
	if err != nil {
		t.Fatalf("Failed to start server for resource exhaustion test: %v", err)
	}
	defer srv.Shutdown()

	time.Sleep(500 * time.Millisecond)

	// Test parameters - try to exceed limits
	numClients := 200 // Exceeds MaxConnectionsPerIP
	testDuration := 30 * time.Second

	// Metrics tracking
	var (
		successfulConnections int64
		failedConnections     int64
		connectionRefused     int64
		timeoutErrors         int64
		otherErrors          int64
		messagesSent         int64
		messagesReceived     int64
	)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var wg sync.WaitGroup
	var allClients []*client.Client
	var clientsMu sync.Mutex

	// Attempt to create more clients than the server can handle
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			config := client.DefaultClientConfig("localhost:9400")
			config.ConnectionType = connection.ConnectionTypeTCP
			config.ReconnectEnabled = false
			config.ConnectTimeout = 5 * time.Second
			config.HeartbeatEnabled = false

			c := client.NewClientWithConfig(config)

			c.OnMessage = func(msg *message.Message) {
				atomic.AddInt64(&messagesReceived, 1)
			}

			c.OnError = func(err error) {
				// Categorize errors
				errStr := err.Error()
				if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "refused") {
					atomic.AddInt64(&connectionRefused, 1)
				} else if strings.Contains(errStr, "timeout") {
					atomic.AddInt64(&timeoutErrors, 1)
				} else {
					atomic.AddInt64(&otherErrors, 1)
				}
			}

			connected := make(chan bool, 1)
			c.OnConnect = func() {
				connected <- true
			}

			err := c.Connect()
			if err != nil {
				atomic.AddInt64(&failedConnections, 1)
				
				// Categorize connection errors
				errStr := err.Error()
				if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "refused") {
					atomic.AddInt64(&connectionRefused, 1)
				} else if strings.Contains(errStr, "timeout") {
					atomic.AddInt64(&timeoutErrors, 1)
				} else {
					atomic.AddInt64(&otherErrors, 1)
				}
				return
			}

			select {
			case <-connected:
				atomic.AddInt64(&successfulConnections, 1)
				
				clientsMu.Lock()
				allClients = append(allClients, c)
				clientsMu.Unlock()

				// Send periodic messages to maintain connection
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()

				messageCount := 0
				for messageCount < 3 { // Send up to 3 messages
					select {
					case <-ticker.C:
						msg := message.Chat().
							FromUser(fmt.Sprintf("exhaust%d", clientID)).
							WithText(fmt.Sprintf("Resource exhaustion test %d", messageCount)).
							Build()

						if err := c.SendMessage(msg); err == nil {
							atomic.AddInt64(&messagesSent, 1)
						}
						messageCount++

					case <-ctx.Done():
						c.Disconnect()
						return
					}
				}

				// Keep connection alive
				<-ctx.Done()
				c.Disconnect()

			case <-time.After(8 * time.Second):
				atomic.AddInt64(&failedConnections, 1)
				atomic.AddInt64(&timeoutErrors, 1)
				c.Disconnect()
				return

			case <-ctx.Done():
				c.Disconnect()
				return
			}
		}(i)

		// Slight delay to create connection pressure
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for completion
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(testDuration + 10*time.Second):
		cancel()
	}

	// Cleanup
	clientsMu.Lock()
	for _, c := range allClients {
		if c.GetState() == client.ClientStateConnected {
			c.Disconnect()
		}
	}
	clientsMu.Unlock()

	time.Sleep(2 * time.Second)

	// Final metrics
	finalSuccess := atomic.LoadInt64(&successfulConnections)
	finalFailed := atomic.LoadInt64(&failedConnections)
	finalRefused := atomic.LoadInt64(&connectionRefused)
	finalSent := atomic.LoadInt64(&messagesSent)
	_ = atomic.LoadInt64(&timeoutErrors) // Not used in assertions
	_ = atomic.LoadInt64(&otherErrors) // Not used in assertions

	// Assertions - we expect resource exhaustion behavior
	require.Greater(t, finalSuccess, int64(0), "Should have some successful connections")
	
	// Check total attempts first
	totalAttempts := finalSuccess + finalFailed
	require.Equal(t, totalAttempts, int64(numClients), "Should account for all connection attempts")
	
	// Resource exhaustion should cause failures
	require.Greater(t, finalFailed, int64(0), "Should have some failed connections due to resource limits")

	// Check that we hit the configured limit
	assert.LessOrEqual(t, finalSuccess, int64(config.MaxConnectionsPerIP), 
		"Should not exceed MaxConnectionsPerIP limit")

	// Check that resource exhaustion manifests as connection refusals
	assert.Greater(t, finalRefused, int64(0), "Should have connection refused errors")

	// Check that the server didn't crash - successful connections should work normally
	if finalSuccess > 0 {
		assert.Greater(t, finalSent, int64(0), "Successful connections should be able to send messages")
	}

	// Check that failure rate is high (indicating resource exhaustion)
	failureRate := float64(finalFailed) / float64(totalAttempts)
	assert.GreaterOrEqual(t, failureRate, 0.5, "Should have high failure rate due to resource exhaustion")
}
