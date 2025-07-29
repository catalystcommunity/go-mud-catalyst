package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MetricType represents different types of metrics
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeTimer     MetricType = "timer"
)

// Metric represents a single metric measurement
type Metric struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Value     float64                `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// MetricsCollector handles metrics collection and reporting
type MetricsCollector struct {
	metrics     map[string]*MetricValue
	mu          sync.RWMutex
	config      *MetricsConfig
	stopChannel chan struct{}
	wg          sync.WaitGroup
}

// MetricValue stores a metric's current value and metadata
type MetricValue struct {
	Type      MetricType
	Value     float64
	Count     int64
	Sum       float64
	Min       float64
	Max       float64
	Tags      map[string]string
	LastUpdate time.Time
	mu        sync.RWMutex
}

// MetricsConfig configures the metrics collector
type MetricsConfig struct {
	Enabled          bool          `json:"enabled"`
	CollectionInterval time.Duration `json:"collection_interval"`
	OutputDirectory  string        `json:"output_directory"`
	EnableConsoleOutput bool       `json:"enable_console_output"`
	EnableFileOutput    bool       `json:"enable_file_output"`
	MaxMetricsInMemory  int        `json:"max_metrics_in_memory"`
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:             true,
		CollectionInterval:  30 * time.Second,
		OutputDirectory:     "./metrics",
		EnableConsoleOutput: false,
		EnableFileOutput:    true,
		MaxMetricsInMemory:  10000,
	}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(config *MetricsConfig) (*MetricsCollector, error) {
	if config == nil {
		config = DefaultMetricsConfig()
	}

	mc := &MetricsCollector{
		metrics:     make(map[string]*MetricValue),
		config:      config,
		stopChannel: make(chan struct{}),
	}

	if config.EnableFileOutput {
		if err := os.MkdirAll(config.OutputDirectory, 0755); err != nil {
			return nil, fmt.Errorf("failed to create metrics directory: %w", err)
		}
	}

	// Start background metrics collection
	if config.Enabled {
		mc.wg.Add(1)
		go mc.collectMetrics()
	}

	return mc, nil
}

// collectMetrics handles periodic metrics collection and output
func (mc *MetricsCollector) collectMetrics() {
	defer mc.wg.Done()
	
	ticker := time.NewTicker(mc.config.CollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.outputMetrics()
		case <-mc.stopChannel:
			// Final metrics collection before shutdown
			mc.outputMetrics()
			return
		}
	}
}

// outputMetrics writes current metrics to configured outputs
func (mc *MetricsCollector) outputMetrics() {
	mc.mu.RLock()
	metrics := make([]Metric, 0, len(mc.metrics))
	now := time.Now()

	for name, metricValue := range mc.metrics {
		metricValue.mu.RLock()
		metric := Metric{
			Name:      name,
			Type:      metricValue.Type,
			Value:     metricValue.Value,
			Timestamp: now,
			Tags:      metricValue.Tags,
		}

		// Add additional data for histogram metrics
		if metricValue.Type == MetricTypeHistogram {
			metric.Data = map[string]interface{}{
				"count": metricValue.Count,
				"sum":   metricValue.Sum,
				"min":   metricValue.Min,
				"max":   metricValue.Max,
				"avg":   metricValue.Sum / float64(metricValue.Count),
			}
		}

		metrics = append(metrics, metric)
		metricValue.mu.RUnlock()
	}
	mc.mu.RUnlock()

	// Output to console if enabled
	if mc.config.EnableConsoleOutput {
		mc.outputToConsole(metrics)
	}

	// Output to file if enabled
	if mc.config.EnableFileOutput {
		mc.outputToFile(metrics, now)
	}
}

// outputToConsole prints metrics to console
func (mc *MetricsCollector) outputToConsole(metrics []Metric) {
	fmt.Printf("=== Metrics Report (%s) ===\n", time.Now().Format("2006-01-02 15:04:05"))
	for _, metric := range metrics {
		if metric.Type == MetricTypeHistogram && metric.Data != nil {
			fmt.Printf("%s: avg=%.2f count=%v min=%.2f max=%.2f\n", 
				metric.Name, metric.Data["avg"], metric.Data["count"], 
				metric.Data["min"], metric.Data["max"])
		} else {
			fmt.Printf("%s: %.2f\n", metric.Name, metric.Value)
		}
	}
	fmt.Println()
}

// outputToFile writes metrics to a JSON file
func (mc *MetricsCollector) outputToFile(metrics []Metric, timestamp time.Time) {
	filename := filepath.Join(mc.config.OutputDirectory, 
		fmt.Sprintf("metrics_%s.json", timestamp.Format("2006-01-02_15")))

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	// Write metrics as JSON lines
	for _, metric := range metrics {
		if data, err := json.Marshal(metric); err == nil {
			file.Write(data)
			file.Write([]byte("\n"))
		}
	}
}

// Increment increments a counter metric
func (mc *MetricsCollector) Increment(name string, tags map[string]string) {
	mc.Add(name, 1, tags)
}

// Add adds a value to a counter metric
func (mc *MetricsCollector) Add(name string, value float64, tags map[string]string) {
	if !mc.config.Enabled {
		return
	}

	key := mc.getMetricKey(name, tags)
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metric, exists := mc.metrics[key]
	if !exists {
		metric = &MetricValue{
			Type: MetricTypeCounter,
			Tags: tags,
		}
		mc.metrics[key] = metric
	}

	metric.mu.Lock()
	metric.Value += value
	metric.LastUpdate = time.Now()
	metric.mu.Unlock()
}

// Set sets a gauge metric value
func (mc *MetricsCollector) Set(name string, value float64, tags map[string]string) {
	if !mc.config.Enabled {
		return
	}

	key := mc.getMetricKey(name, tags)
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metric, exists := mc.metrics[key]
	if !exists {
		metric = &MetricValue{
			Type: MetricTypeGauge,
			Tags: tags,
		}
		mc.metrics[key] = metric
	}

	metric.mu.Lock()
	metric.Value = value
	metric.LastUpdate = time.Now()
	metric.mu.Unlock()
}

// Record records a value for a histogram metric
func (mc *MetricsCollector) Record(name string, value float64, tags map[string]string) {
	if !mc.config.Enabled {
		return
	}

	key := mc.getMetricKey(name, tags)
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metric, exists := mc.metrics[key]
	if !exists {
		metric = &MetricValue{
			Type: MetricTypeHistogram,
			Tags: tags,
			Min:  value,
			Max:  value,
		}
		mc.metrics[key] = metric
	}

	metric.mu.Lock()
	metric.Count++
	metric.Sum += value
	metric.Value = metric.Sum / float64(metric.Count) // Average
	if value < metric.Min {
		metric.Min = value
	}
	if value > metric.Max {
		metric.Max = value
	}
	metric.LastUpdate = time.Now()
	metric.mu.Unlock()
}

// Time records the duration of an operation
func (mc *MetricsCollector) Time(name string, duration time.Duration, tags map[string]string) {
	mc.Record(name, float64(duration.Milliseconds()), tags)
}

// getMetricKey generates a unique key for a metric with tags
func (mc *MetricsCollector) getMetricKey(name string, tags map[string]string) string {
	if len(tags) == 0 {
		return name
	}

	key := name
	for k, v := range tags {
		key += fmt.Sprintf(",%s=%s", k, v)
	}
	return key
}

// GetSnapshot returns a snapshot of current metrics
func (mc *MetricsCollector) GetSnapshot() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := make(map[string]interface{})
	for name, metric := range mc.metrics {
		metric.mu.RLock()
		metricData := map[string]interface{}{
			"type":        metric.Type,
			"value":       metric.Value,
			"last_update": metric.LastUpdate,
		}

		if metric.Type == MetricTypeHistogram {
			metricData["count"] = metric.Count
			metricData["sum"] = metric.Sum
			metricData["min"] = metric.Min
			metricData["max"] = metric.Max
		}

		snapshot[name] = metricData
		metric.mu.RUnlock()
	}

	return snapshot
}

// Close shuts down the metrics collector
func (mc *MetricsCollector) Close() error {
	if mc.config.Enabled {
		close(mc.stopChannel)
		mc.wg.Wait()
	}
	return nil
}

// PropertyMetrics tracks metrics related to Properties usage
type PropertyMetrics struct {
	collector *MetricsCollector
}

// NewPropertyMetrics creates a new property metrics tracker
func NewPropertyMetrics(collector *MetricsCollector) *PropertyMetrics {
	return &PropertyMetrics{
		collector: collector,
	}
}

// RecordPropertyRead records a property read operation
func (pm *PropertyMetrics) RecordPropertyRead(playerID, propertyKey string, success bool, duration time.Duration) {
	tags := map[string]string{
		"player_id":    playerID,
		"property_key": propertyKey,
		"success":      fmt.Sprintf("%t", success),
	}

	pm.collector.Increment("wumpus.properties.reads.total", tags)
	pm.collector.Time("wumpus.properties.read.duration", duration, tags)

	if success {
		pm.collector.Increment("wumpus.properties.reads.success", tags)
	} else {
		pm.collector.Increment("wumpus.properties.reads.errors", tags)
	}
}

// RecordPropertyWrite records a property write operation
func (pm *PropertyMetrics) RecordPropertyWrite(playerID, propertyKey string, success bool, duration time.Duration) {
	tags := map[string]string{
		"player_id":    playerID,
		"property_key": propertyKey,
		"success":      fmt.Sprintf("%t", success),
	}

	pm.collector.Increment("wumpus.properties.writes.total", tags)
	pm.collector.Time("wumpus.properties.write.duration", duration, tags)

	if success {
		pm.collector.Increment("wumpus.properties.writes.success", tags)
	} else {
		pm.collector.Increment("wumpus.properties.writes.errors", tags)
	}
}

// RecordPropertyCacheHit records a property cache hit
func (pm *PropertyMetrics) RecordPropertyCacheHit(playerID, propertyKey string) {
	tags := map[string]string{
		"player_id":    playerID,
		"property_key": propertyKey,
	}
	pm.collector.Increment("wumpus.properties.cache.hits", tags)
}

// RecordPropertyCacheMiss records a property cache miss
func (pm *PropertyMetrics) RecordPropertyCacheMiss(playerID, propertyKey string) {
	tags := map[string]string{
		"player_id":    playerID,
		"property_key": propertyKey,
	}
	pm.collector.Increment("wumpus.properties.cache.misses", tags)
}

// GameMetrics tracks general game metrics
type GameMetrics struct {
	collector *MetricsCollector
}

// NewGameMetrics creates a new game metrics tracker
func NewGameMetrics(collector *MetricsCollector) *GameMetrics {
	return &GameMetrics{
		collector: collector,
	}
}

// RecordPlayerLogin records a player login
func (gm *GameMetrics) RecordPlayerLogin(playerName string, success bool) {
	tags := map[string]string{
		"player": playerName,
		"success": fmt.Sprintf("%t", success),
	}
	gm.collector.Increment("wumpus.players.logins", tags)
}

// RecordGameStart records a game start
func (gm *GameMetrics) RecordGameStart(playerName, instanceID string) {
	tags := map[string]string{
		"player":     playerName,
		"instance":   instanceID,
	}
	gm.collector.Increment("wumpus.games.started", tags)
}

// RecordGameEnd records a game end
func (gm *GameMetrics) RecordGameEnd(playerName, instanceID string, victory bool, duration time.Duration) {
	tags := map[string]string{
		"player":     playerName,
		"instance":   instanceID,
		"victory":    fmt.Sprintf("%t", victory),
	}
	gm.collector.Increment("wumpus.games.ended", tags)
	gm.collector.Time("wumpus.games.duration", duration, tags)
}

// RecordPlayerMove records player movement
func (gm *GameMetrics) RecordPlayerMove(playerName, direction string) {
	tags := map[string]string{
		"player":    playerName,
		"direction": direction,
	}
	gm.collector.Increment("wumpus.player.moves", tags)
}

// RecordWumpusKill records a wumpus kill
func (gm *GameMetrics) RecordWumpusKill(playerName, instanceID string, totalDamage int) {
	tags := map[string]string{
		"player":   playerName,
		"instance": instanceID,
	}
	gm.collector.Increment("wumpus.wumpus.kills", tags)
	gm.collector.Record("wumpus.wumpus.kill.damage", float64(totalDamage), tags)
}

// UpdateActivePlayersGauge updates the active players count
func (gm *GameMetrics) UpdateActivePlayersGauge(count int) {
	gm.collector.Set("wumpus.players.active", float64(count), nil)
}

// UpdateActiveInstancesGauge updates the active instances count
func (gm *GameMetrics) UpdateActiveInstancesGauge(count int) {
	gm.collector.Set("wumpus.instances.active", float64(count), nil)
}

// Global metrics collector and trackers
var (
	globalMetricsCollector *MetricsCollector
	globalPropertyMetrics  *PropertyMetrics
	globalGameMetrics      *GameMetrics
)

// InitializeMetrics initializes the global metrics system
func InitializeMetrics(config *MetricsConfig) error {
	collector, err := NewMetricsCollector(config)
	if err != nil {
		return err
	}

	globalMetricsCollector = collector
	globalPropertyMetrics = NewPropertyMetrics(collector)
	globalGameMetrics = NewGameMetrics(collector)

	return nil
}

// GetMetricsCollector returns the global metrics collector
func GetMetricsCollector() *MetricsCollector {
	return globalMetricsCollector
}

// GetPropertyMetrics returns the global property metrics tracker
func GetPropertyMetrics() *PropertyMetrics {
	return globalPropertyMetrics
}

// GetGameMetrics returns the global game metrics tracker
func GetGameMetrics() *GameMetrics {
	return globalGameMetrics
}

// ShutdownMetrics shuts down the global metrics system
func ShutdownMetrics() error {
	if globalMetricsCollector != nil {
		return globalMetricsCollector.Close()
	}
	return nil
}