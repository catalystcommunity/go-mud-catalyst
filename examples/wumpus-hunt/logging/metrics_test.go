package logging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMetricsCollector_Creation(t *testing.T) {
	config := DefaultMetricsConfig()
	config.OutputDirectory = filepath.Join(os.TempDir(), "test_metrics")
	config.EnableFileOutput = true
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	if collector == nil {
		t.Fatal("Collector should not be nil")
	}

	// Verify metrics directory was created
	if _, err := os.Stat(config.OutputDirectory); os.IsNotExist(err) {
		t.Error("Metrics directory should have been created")
	}

	// Cleanup
	os.RemoveAll(config.OutputDirectory)
}

func TestMetricsCollector_Counter(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Test increment
	collector.Increment("test.counter", map[string]string{"type": "test"})
	collector.Increment("test.counter", map[string]string{"type": "test"})

	// Test add
	collector.Add("test.counter", 5, map[string]string{"type": "test"})

	// Get snapshot
	snapshot := collector.GetSnapshot()
	if len(snapshot) == 0 {
		t.Error("Snapshot should contain metrics")
	}

	// Verify counter value
	key := "test.counter,type=test"
	if metric, exists := snapshot[key]; exists {
		if metricData, ok := metric.(map[string]interface{}); ok {
			if value, ok := metricData["value"].(float64); ok {
				expectedValue := 7.0 // 1 + 1 + 5
				if value != expectedValue {
					t.Errorf("Expected counter value %f, got %f", expectedValue, value)
				}
			} else {
				t.Error("Counter value should be a float64")
			}
		} else {
			t.Error("Metric data should be a map[string]interface{}")
		}
	} else {
		t.Error("Counter metric should exist in snapshot")
	}
}

func TestMetricsCollector_Gauge(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Test gauge set
	collector.Set("test.gauge", 42.5, map[string]string{"type": "test"})
	collector.Set("test.gauge", 100.0, map[string]string{"type": "test"})

	// Get snapshot
	snapshot := collector.GetSnapshot()

	// Verify gauge value (should be last set value)
	key := "test.gauge,type=test"
	if metric, exists := snapshot[key]; exists {
		if metricData, ok := metric.(map[string]interface{}); ok {
			if value, ok := metricData["value"].(float64); ok {
				expectedValue := 100.0
				if value != expectedValue {
					t.Errorf("Expected gauge value %f, got %f", expectedValue, value)
				}
			} else {
				t.Error("Gauge value should be a float64")
			}
		} else {
			t.Error("Metric data should be a map[string]interface{}")
		}
	} else {
		t.Error("Gauge metric should exist in snapshot")
	}
}

func TestMetricsCollector_Histogram(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Test histogram recording
	values := []float64{10, 20, 30, 40, 50}
	for _, value := range values {
		collector.Record("test.histogram", value, map[string]string{"type": "test"})
	}

	// Get snapshot
	snapshot := collector.GetSnapshot()

	// Verify histogram data
	key := "test.histogram,type=test"
	if metric, exists := snapshot[key]; exists {
		if metricData, ok := metric.(map[string]interface{}); ok {
			// Check count
			if count, ok := metricData["count"].(int64); ok {
				if count != int64(len(values)) {
					t.Errorf("Expected count %d, got %d", len(values), count)
				}
			} else {
				t.Error("Histogram count should be an int64")
			}

			// Check sum
			if sum, ok := metricData["sum"].(float64); ok {
				expectedSum := 150.0 // 10+20+30+40+50
				if sum != expectedSum {
					t.Errorf("Expected sum %f, got %f", expectedSum, sum)
				}
			} else {
				t.Error("Histogram sum should be a float64")
			}

			// Check min/max
			if min, ok := metricData["min"].(float64); ok {
				if min != 10.0 {
					t.Errorf("Expected min %f, got %f", 10.0, min)
				}
			} else {
				t.Error("Histogram min should be a float64")
			}

			if max, ok := metricData["max"].(float64); ok {
				if max != 50.0 {
					t.Errorf("Expected max %f, got %f", 50.0, max)
				}
			} else {
				t.Error("Histogram max should be a float64")
			}
		} else {
			t.Error("Metric data should be a map[string]interface{}")
		}
	} else {
		t.Error("Histogram metric should exist in snapshot")
	}
}

func TestMetricsCollector_Timer(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Test timer
	duration := 100 * time.Millisecond
	collector.Time("test.timer", duration, map[string]string{"type": "test"})

	// Get snapshot
	snapshot := collector.GetSnapshot()

	// Verify timer was recorded as histogram
	key := "test.timer,type=test"
	if metric, exists := snapshot[key]; exists {
		if metricData, ok := metric.(map[string]interface{}); ok {
			if value, ok := metricData["value"].(float64); ok {
				expectedValue := float64(duration.Milliseconds())
				if value != expectedValue {
					t.Errorf("Expected timer value %f, got %f", expectedValue, value)
				}
			} else {
				t.Error("Timer value should be a float64")
			}
		}
	} else {
		t.Error("Timer metric should exist in snapshot")
	}
}

func TestMetricsCollector_DisabledState(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Try to record metrics when disabled
	collector.Increment("test.counter", nil)
	collector.Set("test.gauge", 42.0, nil)
	collector.Record("test.histogram", 10.0, nil)

	// Get snapshot - should be empty
	snapshot := collector.GetSnapshot()
	if len(snapshot) != 0 {
		t.Error("Snapshot should be empty when metrics are disabled")
	}
}

func TestPropertyMetrics(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	propertyMetrics := NewPropertyMetrics(collector)

	// Test property read metrics
	propertyMetrics.RecordPropertyRead("player-123", "wumpus_auth", true, 10*time.Millisecond)
	propertyMetrics.RecordPropertyRead("player-123", "wumpus_auth", false, 5*time.Millisecond)

	// Test property write metrics
	propertyMetrics.RecordPropertyWrite("player-123", "wumpus_stats", true, 15*time.Millisecond)

	// Test cache metrics
	propertyMetrics.RecordPropertyCacheHit("player-123", "wumpus_auth")
	propertyMetrics.RecordPropertyCacheMiss("player-123", "wumpus_stats")

	// Get snapshot and verify metrics exist
	snapshot := collector.GetSnapshot()
	
	expectedMetrics := []string{
		"wumpus.properties.reads.total",
		"wumpus.properties.reads.success",
		"wumpus.properties.reads.errors",
		"wumpus.properties.writes.total",
		"wumpus.properties.cache.hits",
		"wumpus.properties.cache.misses",
	}

	for _, expectedMetric := range expectedMetrics {
		found := false
		for key := range snapshot {
			if contains(key, expectedMetric) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected metric %s not found in snapshot", expectedMetric)
		}
	}
}

func TestGameMetrics(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	gameMetrics := NewGameMetrics(collector)

	// Test various game metrics
	gameMetrics.RecordPlayerLogin("TestPlayer", true)
	gameMetrics.RecordPlayerLogin("BadPlayer", false)
	gameMetrics.RecordGameStart("TestPlayer", "instance-123")
	gameMetrics.RecordGameEnd("TestPlayer", "instance-123", true, 5*time.Minute)
	gameMetrics.RecordPlayerMove("TestPlayer", "north")
	gameMetrics.RecordWumpusKill("TestPlayer", "instance-123", 75)
	gameMetrics.UpdateActivePlayersGauge(5)
	gameMetrics.UpdateActiveInstancesGauge(2)

	// Get snapshot and verify metrics exist
	snapshot := collector.GetSnapshot()

	expectedMetrics := []string{
		"wumpus.players.logins",
		"wumpus.games.started",
		"wumpus.games.ended",
		"wumpus.player.moves",
		"wumpus.wumpus.kills",
		"wumpus.players.active",
		"wumpus.instances.active",
	}

	for _, expectedMetric := range expectedMetrics {
		found := false
		for key := range snapshot {
			if contains(key, expectedMetric) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected metric %s not found in snapshot", expectedMetric)
		}
	}
}

func TestMetricsCollector_MetricKeyGeneration(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Test with no tags
	collector.Increment("test.metric", nil)
	key1 := collector.getMetricKey("test.metric", nil)
	if key1 != "test.metric" {
		t.Errorf("Expected key 'test.metric', got '%s'", key1)
	}

	// Test with single tag
	tags := map[string]string{"type": "test"}
	collector.Increment("test.metric", tags)
	key2 := collector.getMetricKey("test.metric", tags)
	if key2 != "test.metric,type=test" {
		t.Errorf("Expected key 'test.metric,type=test', got '%s'", key2)
	}

	// Test with multiple tags
	tags = map[string]string{"type": "test", "env": "prod"}
	collector.Increment("test.metric", tags)
	key3 := collector.getMetricKey("test.metric", tags)
	// Note: map iteration order is not guaranteed, so we check for both possibilities
	if key3 != "test.metric,type=test,env=prod" && key3 != "test.metric,env=prod,type=test" {
		t.Errorf("Expected key with both tags, got '%s'", key3)
	}
}

func TestMetricsCollector_Configuration(t *testing.T) {
	// Test default configuration
	config := DefaultMetricsConfig()
	if !config.Enabled {
		t.Error("Metrics should be enabled by default")
	}
	if config.CollectionInterval != 30*time.Second {
		t.Error("Default collection interval should be 30 seconds")
	}
	if !config.EnableFileOutput {
		t.Error("File output should be enabled by default")
	}
	if config.MaxMetricsInMemory != 10000 {
		t.Error("Default max metrics should be 10000")
	}

	// Test custom configuration
	customConfig := &MetricsConfig{
		Enabled:             false,
		CollectionInterval:  10 * time.Second,
		EnableFileOutput:    false,
		EnableConsoleOutput: true,
		MaxMetricsInMemory:  5000,
	}

	collector, err := NewMetricsCollector(customConfig)
	if err != nil {
		t.Fatalf("Failed to create collector with custom config: %v", err)
	}
	defer collector.Close()

	if collector.config.Enabled {
		t.Error("Custom enabled setting not applied")
	}
	if collector.config.CollectionInterval != 10*time.Second {
		t.Error("Custom collection interval not applied")
	}
}

func TestMetricsCollector_PeriodicCollection(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.CollectionInterval = 100 * time.Millisecond
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Add some metrics
	collector.Increment("test.counter", nil)
	collector.Set("test.gauge", 42.0, nil)

	// Wait for at least one collection cycle
	time.Sleep(200 * time.Millisecond)

	// Verify metrics still exist after collection
	snapshot := collector.GetSnapshot()
	if len(snapshot) == 0 {
		t.Error("Metrics should persist after collection")
	}
}

func TestMetricsCollector_Shutdown(t *testing.T) {
	config := DefaultMetricsConfig()
	config.OutputDirectory = filepath.Join(os.TempDir(), "test_metrics")
	config.EnableFileOutput = true
	config.CollectionInterval = 1 * time.Second // Long interval

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}

	// Add some metrics
	collector.Increment("test.counter", nil)

	// Test graceful shutdown
	err = collector.Close()
	if err != nil {
		t.Fatalf("Failed to close collector: %v", err)
	}

	// Cleanup
	os.RemoveAll(config.OutputDirectory)
}

func TestMetricsCollector_ConcurrentAccess(t *testing.T) {
	config := DefaultMetricsConfig()
	config.Enabled = true
	config.EnableFileOutput = false
	config.EnableConsoleOutput = false

	collector, err := NewMetricsCollector(config)
	if err != nil {
		t.Fatalf("Failed to create metrics collector: %v", err)
	}
	defer collector.Close()

	// Test concurrent metric updates
	done := make(chan bool)
	numGoroutines := 10
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				collector.Increment("test.concurrent", map[string]string{"goroutine": string(rune('A' + id))})
				collector.Set("test.gauge", float64(id*numOperations+j), nil)
				collector.Record("test.histogram", float64(j), nil)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify metrics were recorded
	snapshot := collector.GetSnapshot()
	if len(snapshot) == 0 {
		t.Error("Concurrent operations should have created metrics")
	}
}

func TestMetricsGlobalInitialization(t *testing.T) {
	config := DefaultMetricsConfig()
	config.OutputDirectory = filepath.Join(os.TempDir(), "test_metrics")
	config.EnableFileOutput = true
	config.EnableConsoleOutput = false

	// Test initialization
	err := InitializeMetrics(config)
	if err != nil {
		t.Fatalf("Failed to initialize metrics: %v", err)
	}

	// Test global getters
	if GetMetricsCollector() == nil {
		t.Error("Global metrics collector should not be nil")
	}
	if GetPropertyMetrics() == nil {
		t.Error("Global property metrics should not be nil")
	}
	if GetGameMetrics() == nil {
		t.Error("Global game metrics should not be nil")
	}

	// Test shutdown
	err = ShutdownMetrics()
	if err != nil {
		t.Fatalf("Failed to shutdown metrics: %v", err)
	}

	// Cleanup
	os.RemoveAll(config.OutputDirectory)
}