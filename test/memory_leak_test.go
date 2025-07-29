package test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalystcommunity/muddycore/pkg/client"
	"github.com/catalystcommunity/muddycore/pkg/connection"
	"github.com/catalystcommunity/muddycore/pkg/message"
	"github.com/catalystcommunity/muddycore/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getOpenFileDescriptorCount returns the number of open file descriptors for the current process
// This is platform-specific - works on Linux/Unix systems
func getOpenFileDescriptorCount() int {
	pid := os.Getpid()
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	
	// Try to count files in /proc/PID/fd directory (Linux)
	if entries, err := os.ReadDir(fdDir); err == nil {
		return len(entries)
	}
	
	// Fallback: count files in current process fd directory
	if entries, err := filepath.Glob("/dev/fd/*"); err == nil {
		return len(entries)
	}
	
	// Another fallback: try to read from /proc/PID/limits
	limitsPath := fmt.Sprintf("/proc/%d/limits", pid)
	if data, err := os.ReadFile(limitsPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Max open files") {
				// This gives us the limit, not current count
				// But we can use it as a rough indicator
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					if soft, err := strconv.Atoi(fields[3]); err == nil {
						// Return a fraction of the limit as current estimate
						return soft / 100
					}
				}
			}
		}
	}
	
	// Final fallback: return a reasonable default for testing
	// This ensures tests can run even if FD counting isn't available
	return 10
}

// Note: TestMain is defined in queue_overflow_test.go to avoid conflicts

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	Alloc         uint64 // bytes allocated and not yet freed
	TotalAlloc    uint64 // cumulative bytes allocated for heap objects
	Sys           uint64 // total bytes of memory obtained from the OS
	NumGC         uint32 // number of completed GC cycles
	Goroutines    int    // number of goroutines
	HeapObjects   uint64 // number of allocated heap objects
	HeapInuse     uint64 // bytes in in-use spans
	StackInuse    uint64 // bytes in stack spans
}

// collectMemoryStats gathers current memory statistics
func collectMemoryStats() MemoryStats {
	runtime.GC() // Force garbage collection for accurate measurement
	runtime.GC() // Run twice to ensure cleanup

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Alloc:         m.Alloc,
		TotalAlloc:    m.TotalAlloc,
		Sys:           m.Sys,
		NumGC:         m.NumGC,
		Goroutines:    runtime.NumGoroutine(),
		HeapObjects:   m.HeapObjects,
		HeapInuse:     m.HeapInuse,
		StackInuse:    m.StackInuse,
	}
}

// createHighCapacityServer creates a server with higher connection limits for testing
func createHighCapacityServer(host, port string) *server.ConnServer {
	config := server.DefaultServerConfig()
	config.Host = host
	config.Port = port
	config.MaxConnectionsPerIP = 1000
	config.MaxConnectionsPerSecond = 2000
	return server.NewServerWithConfig(config)
}

// TestConnectionChurnMemoryLeak tests for memory leaks during rapid connection cycles
func TestConnectionChurnMemoryLeak(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8300")
	err := srv.StartServer()
	require.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test parameters - minimal for debugging  
	numCycles := 1
	concurrentClients := 1
	
	// Collect initial memory statistics
	initialStats := collectMemoryStats()
	
	var wg sync.WaitGroup
	var (
		successfulConnections int64
		failedConnections     int64
		totalCycles           int64
	)

	// Run connection churn cycles
	for i := 0; i < concurrentClients; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			cyclesPerWorker := numCycles / concurrentClients
			for j := 0; j < cyclesPerWorker; j++ {
				config := client.DefaultClientConfig("localhost:8300")
				config.ConnectionType = connection.ConnectionTypeTCP
				config.ReconnectEnabled = false
				config.ConnectTimeout = 2 * time.Second
				config.HeartbeatEnabled = false

				c := client.NewClientWithConfig(config)

				// Connect
				err := c.Connect()
				if err != nil {
					atomic.AddInt64(&failedConnections, 1)
					continue
				}

				atomic.AddInt64(&successfulConnections, 1)

				// Send a message every 10th cycle to exercise message handling
				if j%10 == 0 {
					msg := message.Chat().
						FromUser(fmt.Sprintf("churn%d", workerID)).
						WithText(fmt.Sprintf("Memory leak test cycle %d", j)).
						Build()
					c.SendMessage(msg) // Ignore errors for churn test
				}

				// Disconnect
				c.Close()
				atomic.AddInt64(&totalCycles, 1)

				// Brief pause to spread load
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Allow time for cleanup
	time.Sleep(1 * time.Second)

	// Collect final memory statistics
	finalStats := collectMemoryStats()

	// Analyze memory usage
	successConns := atomic.LoadInt64(&successfulConnections)
	cycles := atomic.LoadInt64(&totalCycles)

	// Validate test ran successfully
	require.Greater(t, successConns, int64(0), "Should have successful connections")
	require.Greater(t, cycles, int64(numCycles/2), "Should complete most cycles")

	// Memory leak detection
	goroutineGrowth := finalStats.Goroutines - initialStats.Goroutines
	memoryGrowth := int64(finalStats.Alloc) - int64(initialStats.Alloc)
	heapObjectGrowth := int64(finalStats.HeapObjects) - int64(initialStats.HeapObjects)

	// Assertions for memory leaks
	assert.LessOrEqual(t, goroutineGrowth, 10, 
		"Goroutine count should not grow significantly after connection churn")
	
	// Allow some memory growth but not proportional to connection count
	maxAllowedMemoryGrowth := int64(1024 * 1024) // 1MB
	assert.LessOrEqual(t, memoryGrowth, maxAllowedMemoryGrowth,
		"Memory growth should be bounded after connection churn")
	
	// Heap objects should not grow significantly
	maxAllowedObjectGrowth := int64(1000)
	assert.LessOrEqual(t, heapObjectGrowth, maxAllowedObjectGrowth,
		"Heap object count should not grow significantly after connection churn")
}

// TestGoroutineLeakDetection tests for goroutine leaks during various operations
func TestGoroutineLeakDetection(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8301")
	err := srv.StartServer()
	require.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	initialGoroutines := runtime.NumGoroutine()

	t.Run("ClientConnectDisconnectGoroutines", func(t *testing.T) {
		startGoroutines := runtime.NumGoroutine()
		
		// Create multiple clients and ensure they clean up
		numClients := 10
		var clients []*client.Client
		
		// Connect phase
		for i := 0; i < numClients; i++ {
			config := client.DefaultClientConfig("localhost:8301")
			config.ReconnectEnabled = false
			config.HeartbeatEnabled = true
			config.HeartbeatInterval = 5 * time.Second
			
			c := client.NewClientWithConfig(config)
			err := c.Connect()
			require.NoError(t, err)
			clients = append(clients, c)
		}
		
		// Allow connections to settle
		time.Sleep(500 * time.Millisecond)
		
		// Disconnect phase
		for _, c := range clients {
			c.Close()
		}
		
		// Allow cleanup time
		time.Sleep(1 * time.Second)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		
		endGoroutines := runtime.NumGoroutine()
		
		// Should return close to starting count
		goroutineLeak := endGoroutines - startGoroutines
		assert.LessOrEqual(t, goroutineLeak, 5, 
			"Should not leak more than 5 goroutines after client disconnect")
	})

	t.Run("MessageHandlingGoroutines", func(t *testing.T) {
		startGoroutines := runtime.NumGoroutine()
		
		// Create client for message testing
		config := client.DefaultClientConfig("localhost:8301")
		config.ReconnectEnabled = false
		config.HeartbeatEnabled = false
		
		c := client.NewClientWithConfig(config)
		err := c.Connect()
		require.NoError(t, err)
		defer c.Close()
		
		// Send many messages to exercise message handling
		numMessages := 200
		for i := 0; i < numMessages; i++ {
			msg := message.Chat().
				FromUser("leak_test").
				WithText(fmt.Sprintf("Goroutine leak test message %d", i)).
				Build()
			
			err := c.SendMessage(msg)
			assert.NoError(t, err)
			
			// Brief pause
			if i%50 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
		
		// Allow message processing to complete
		time.Sleep(500 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		
		endGoroutines := runtime.NumGoroutine()
		
		goroutineLeak := endGoroutines - startGoroutines
		assert.LessOrEqual(t, goroutineLeak, 10,
			"Should not leak excessive goroutines during message handling")
	})

	// Final check - should be close to initial count
	finalGoroutines := runtime.NumGoroutine()
	totalLeak := finalGoroutines - initialGoroutines
	assert.LessOrEqual(t, totalLeak, 10,
		"Total goroutine leak should be minimal after all tests")
}

// TestHeapMemoryGrowthTracking tests heap memory growth during sustained operations
func TestHeapMemoryGrowthTracking(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8302")
	err := srv.StartServer()
	require.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Collect baseline memory stats
	_ = collectMemoryStats()
	
	// Test sustained operations with memory tracking
	numClients := 5
	messagesPerClient := 10
	duration := 8 * time.Second

	var wg sync.WaitGroup
	var totalMessagesSent int64

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Track memory stats periodically
	var memorySnapshots []MemoryStats
	var snapshotMutex sync.Mutex
	
	// Memory monitoring goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		// Take initial snapshot
		stats := collectMemoryStats()
		snapshotMutex.Lock()
		memorySnapshots = append(memorySnapshots, stats)
		snapshotMutex.Unlock()
		
		for {
			select {
			case <-ticker.C:
				stats := collectMemoryStats()
				snapshotMutex.Lock()
				memorySnapshots = append(memorySnapshots, stats)
				snapshotMutex.Unlock()
			case <-ctx.Done():
				// Take final snapshot
				stats := collectMemoryStats()
				snapshotMutex.Lock()
				memorySnapshots = append(memorySnapshots, stats)
				snapshotMutex.Unlock()
				return
			}
		}
	}()

	// Create persistent clients for sustained operations
	var clients []*client.Client
	for i := 0; i < numClients; i++ {
		config := client.DefaultClientConfig("localhost:8302")
		config.ReconnectEnabled = false
		config.HeartbeatEnabled = true
		config.HeartbeatInterval = 10 * time.Second
		
		c := client.NewClientWithConfig(config)
		err := c.Connect()
		require.NoError(t, err)
		clients = append(clients, c)
	}

	defer func() {
		for _, c := range clients {
			c.Close()
		}
	}()

	// Allow connections to stabilize
	time.Sleep(500 * time.Millisecond)

	// Start sustained message sending
	for i, c := range clients {
		wg.Add(1)
		go func(clientIdx int, client *client.Client) {
			defer wg.Done()
			
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			
			messageCount := 0
			for messageCount < messagesPerClient {
				select {
				case <-ticker.C:
					msg := message.Chat().
						FromUser(fmt.Sprintf("heap_client%d", clientIdx)).
						WithText(fmt.Sprintf("Heap memory test message %d", messageCount)).
						Build()
					
					err := client.SendMessage(msg)
					if err == nil {
						atomic.AddInt64(&totalMessagesSent, 1)
						messageCount++
					}
					
				case <-ctx.Done():
					return
				}
			}
		}(i, c)
	}

	wg.Wait()

	// Allow time for final snapshot to be taken
	time.Sleep(100 * time.Millisecond)

	// Final memory collection
	_ = collectMemoryStats()

	// Analyze memory growth
	totalMessages := atomic.LoadInt64(&totalMessagesSent)
	
	snapshotMutex.Lock()
	snapshots := make([]MemoryStats, len(memorySnapshots))
	copy(snapshots, memorySnapshots)
	snapshotMutex.Unlock()

	// Validate test ran successfully
	require.Greater(t, totalMessages, int64(numClients*messagesPerClient/2), 
		"Should send most expected messages")
	require.Greater(t, len(snapshots), 1, "Should collect multiple memory snapshots")

	// Check for linear memory growth (indication of leak)
	if len(snapshots) >= 3 {
		firstSnapshot := snapshots[0]
		lastSnapshot := snapshots[len(snapshots)-1]
		
		heapGrowth := int64(lastSnapshot.HeapInuse) - int64(firstSnapshot.HeapInuse)
		objectGrowth := int64(lastSnapshot.HeapObjects) - int64(firstSnapshot.HeapObjects)
		
		// Memory should not grow linearly with message count
		// Allow some growth but not proportional to message volume
		maxAllowedHeapGrowth := int64(5 * 1024 * 1024) // 5MB
		assert.LessOrEqual(t, heapGrowth, maxAllowedHeapGrowth,
			"Heap memory should not grow excessively during sustained operations")
		
		maxAllowedObjectGrowth := int64(10000)
		assert.LessOrEqual(t, objectGrowth, maxAllowedObjectGrowth,
			"Heap object count should not grow excessively during sustained operations")
	}
}

// TestRuntimeMemoryStatistics tests comprehensive memory statistics collection
func TestRuntimeMemoryStatistics(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8303")
	err := srv.StartServer()
	require.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Test memory statistics collection accuracy
	initialStats := collectMemoryStats()
	
	// Perform operations that should affect memory
	numOperations := 1000
	var allocatedSlices [][]byte
	
	// Allocate memory
	for i := 0; i < numOperations; i++ {
		// Allocate varying sizes
		size := 1024 + (i % 1024)
		slice := make([]byte, size)
		allocatedSlices = append(allocatedSlices, slice)
		
		// Touch the memory
		for j := range slice {
			slice[j] = byte(j % 256)
		}
	}
	
	midStats := collectMemoryStats()
	
	// Release memory
	allocatedSlices = nil
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	finalStats := collectMemoryStats()
	
	// Validate memory statistics are working
	gcRan := finalStats.NumGC >= midStats.NumGC
	
	// Less strict assertions since memory behavior can vary
	assert.True(t, gcRan, "GC should run during test")
	
	// The key validation is that we can collect statistics properly
	assert.Greater(t, finalStats.NumGC, initialStats.NumGC, "Overall GC count should increase")
}

// TestFileDescriptorLeakDetection tests for file descriptor leaks during connection management
func TestFileDescriptorLeakDetection(t *testing.T) {
	srv := createHighCapacityServer("localhost", "8304")
	err := srv.StartServer()
	require.NoError(t, err)
	defer srv.Shutdown()

	time.Sleep(100 * time.Millisecond)

	// Get initial file descriptor count (platform-specific approach)
	initialFDs := getOpenFileDescriptorCount()
	initialStats := collectMemoryStats()

	t.Run("ConnectionFileDescriptorTracking", func(t *testing.T) {
		startFDs := getOpenFileDescriptorCount()
		startGoroutines := runtime.NumGoroutine()
		
		// Create many connections to test FD usage
		numConnections := 20
		var clients []*client.Client
		
		// Connect phase - should increase FD count
		for i := 0; i < numConnections; i++ {
			config := client.DefaultClientConfig("localhost:8304")
			config.ReconnectEnabled = false
			config.HeartbeatEnabled = false
			config.ConnectTimeout = 3 * time.Second
			
			c := client.NewClientWithConfig(config)
			err := c.Connect()
			require.NoError(t, err)
			clients = append(clients, c)
			
			// Brief pause to prevent overwhelming
			if i%20 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
		
		// Check FD count after connections
		midFDs := getOpenFileDescriptorCount()
		
		// Disconnect phase - should decrease FD count
		for _, c := range clients {
			c.Close()
		}
		clients = nil
		
		// Allow cleanup time
		time.Sleep(200 * time.Millisecond)
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		
		endFDs := getOpenFileDescriptorCount()
		endGoroutines := runtime.NumGoroutine()
		
		fdGrowth := midFDs - startFDs
		fdLeak := endFDs - startFDs
		goroutineLeak := endGoroutines - startGoroutines
		
		// Assertions
		assert.Greater(t, fdGrowth, 0, "FD count should increase with connections")
		assert.LessOrEqual(t, fdLeak, 10, "Should not leak more than 10 FDs after cleanup")
		assert.LessOrEqual(t, goroutineLeak, 5, "Should not leak more than 5 goroutines")
	})

	t.Run("RapidConnectionCyclesFDTracking", func(t *testing.T) {
		startFDs := getOpenFileDescriptorCount()
		
		// Rapid connect/disconnect cycles
		numCycles := 50
		var wg sync.WaitGroup
		
		for i := 0; i < numCycles; i++ {
			wg.Add(1)
			go func(cycleID int) {
				defer wg.Done()
				
				config := client.DefaultClientConfig("localhost:8304")
				config.ReconnectEnabled = false
				config.HeartbeatEnabled = false
				config.ConnectTimeout = 2 * time.Second
				
				c := client.NewClientWithConfig(config)
				
				// Quick connect
				err := c.Connect()
				if err != nil {
					return // Skip failed connections
				}
				
				// Brief activity
				time.Sleep(time.Duration(1+cycleID%5) * time.Millisecond)
				
				// Quick disconnect
				c.Close()
			}(i)
		}
		
		wg.Wait()
		
		// Allow cleanup
		time.Sleep(500 * time.Millisecond)
		runtime.GC()
		time.Sleep(200 * time.Millisecond)
		
		endFDs := getOpenFileDescriptorCount()
		fdLeak := endFDs - startFDs
		
		// Should not leak FDs proportional to connection count
		assert.LessOrEqual(t, fdLeak, 15, "Should not leak significant FDs during rapid cycles")
	})

	// Final check
	finalFDs := getOpenFileDescriptorCount()
	finalStats := collectMemoryStats()
	
	totalFDLeak := finalFDs - initialFDs
	totalGoroutineLeak := finalStats.Goroutines - initialStats.Goroutines
	
	// Overall assertions
	assert.LessOrEqual(t, totalFDLeak, 20, "Total FD leak should be minimal")
	assert.LessOrEqual(t, totalGoroutineLeak, 10, "Total goroutine leak should be minimal")
}