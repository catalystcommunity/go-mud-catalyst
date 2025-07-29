package scripting

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// ScriptWatcher monitors script files for changes and triggers reloads
type ScriptWatcher struct {
	vmManager    *VMManager
	watchedFiles map[string]*WatchedScript
	watchDirs    map[string]bool
	mutex        sync.RWMutex
	
	// Polling configuration
	pollInterval time.Duration
	stopChannel  chan struct{}
	isRunning    bool
}

// WatchedScript represents a script file being monitored
type WatchedScript struct {
	FilePath     string
	VMID         string
	LastModified time.Time
	LastHash     string
	LoadFunction string // Lua function to call after reload
	AutoReload   bool
	ReloadCount  int64
	LastReload   time.Time
	LastError    error
}

// HotReloadConfig configures hot reloading behavior
type HotReloadConfig struct {
	PollInterval    time.Duration
	AutoReload      bool
	ValidateOnReload bool
	BackupOnReload  bool
	MaxReloadAttempts int
	ReloadTimeout   time.Duration
}

// DefaultHotReloadConfig returns a safe default configuration
func DefaultHotReloadConfig() *HotReloadConfig {
	return &HotReloadConfig{
		PollInterval:     2 * time.Second,
		AutoReload:       true,
		ValidateOnReload: true,
		BackupOnReload:   false, // Don't create backups by default
		MaxReloadAttempts: 3,
		ReloadTimeout:    10 * time.Second,
	}
}

// NewScriptWatcher creates a new script file watcher
func NewScriptWatcher(vmManager *VMManager, config *HotReloadConfig) *ScriptWatcher {
	if config == nil {
		config = DefaultHotReloadConfig()
	}
	
	return &ScriptWatcher{
		vmManager:    vmManager,
		watchedFiles: make(map[string]*WatchedScript),
		watchDirs:    make(map[string]bool),
		pollInterval: config.PollInterval,
		stopChannel:  make(chan struct{}),
	}
}

// WatchScript adds a script file to the watch list
func (sw *ScriptWatcher) WatchScript(filePath, vmID, loadFunction string, autoReload bool) error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	
	// Verify VM exists
	_, exists := sw.vmManager.GetVM(vmID)
	if !exists {
		return fmt.Errorf("VM %s does not exist", vmID)
	}
	
	// Get initial file info
	hash, modTime, err := getFileInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", filePath, err)
	}
	
	watchedScript := &WatchedScript{
		FilePath:     filePath,
		VMID:         vmID,
		LastModified: modTime,
		LastHash:     hash,
		LoadFunction: loadFunction,
		AutoReload:   autoReload,
		LastReload:   time.Now(),
	}
	
	sw.watchedFiles[filePath] = watchedScript
	
	// Add directory to watch list
	dir := filepath.Dir(filePath)
	sw.watchDirs[dir] = true
	
	return nil
}

// UnwatchScript removes a script file from the watch list
func (sw *ScriptWatcher) UnwatchScript(filePath string) error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	
	delete(sw.watchedFiles, filePath)
	
	// Remove directory if no more files are watched in it
	dir := filepath.Dir(filePath)
	hasOtherFiles := false
	for watchedPath := range sw.watchedFiles {
		if filepath.Dir(watchedPath) == dir {
			hasOtherFiles = true
			break
		}
	}
	
	if !hasOtherFiles {
		delete(sw.watchDirs, dir)
	}
	
	return nil
}

// Start begins monitoring watched script files
func (sw *ScriptWatcher) Start() error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	
	if sw.isRunning {
		return fmt.Errorf("script watcher is already running")
	}
	
	sw.isRunning = true
	go sw.watchLoop()
	
	return nil
}

// Stop stops monitoring script files
func (sw *ScriptWatcher) Stop() error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	
	if !sw.isRunning {
		return fmt.Errorf("script watcher is not running")
	}
	
	close(sw.stopChannel)
	sw.isRunning = false
	
	return nil
}

// watchLoop is the main monitoring loop
func (sw *ScriptWatcher) watchLoop() {
	ticker := time.NewTicker(sw.pollInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sw.checkForChanges()
		case <-sw.stopChannel:
			return
		}
	}
}

// checkForChanges checks all watched files for modifications
func (sw *ScriptWatcher) checkForChanges() {
	sw.mutex.RLock()
	filesToCheck := make(map[string]*WatchedScript)
	for path, script := range sw.watchedFiles {
		filesToCheck[path] = script
	}
	sw.mutex.RUnlock()
	
	for filePath, watchedScript := range filesToCheck {
		if sw.hasFileChanged(filePath, watchedScript) {
			if watchedScript.AutoReload {
				sw.reloadScript(filePath, watchedScript)
			}
		}
	}
}

// hasFileChanged checks if a file has been modified since last check
func (sw *ScriptWatcher) hasFileChanged(filePath string, watchedScript *WatchedScript) bool {
	hash, modTime, err := getFileInfo(filePath)
	if err != nil {
		// File might have been deleted or is temporarily inaccessible
		return false
	}
	
	// Check if modification time or hash has changed
	if modTime.After(watchedScript.LastModified) || hash != watchedScript.LastHash {
		sw.mutex.Lock()
		watchedScript.LastModified = modTime
		watchedScript.LastHash = hash
		sw.mutex.Unlock()
		return true
	}
	
	return false
}

// reloadScript reloads a script file into its VM
func (sw *ScriptWatcher) reloadScript(filePath string, watchedScript *WatchedScript) {
	sw.mutex.Lock()
	watchedScript.ReloadCount++
	watchedScript.LastReload = time.Now()
	watchedScript.LastError = nil
	sw.mutex.Unlock()
	
	// Read script file
	script, err := readScriptFile(filePath)
	if err != nil {
		sw.setReloadError(watchedScript, fmt.Errorf("failed to read script file: %w", err))
		return
	}
	
	// Get VM
	vm, exists := sw.vmManager.GetVM(watchedScript.VMID)
	if !exists {
		sw.setReloadError(watchedScript, fmt.Errorf("VM %s no longer exists", watchedScript.VMID))
		return
	}
	
	// Execute the script
	result, err := vm.Execute(script)
	if err != nil {
		sw.setReloadError(watchedScript, fmt.Errorf("script execution failed: %w", err))
		return
	}
	
	// Call load function if specified
	if watchedScript.LoadFunction != "" {
		loadScript := fmt.Sprintf(`
if type(%s) == "function" then
  %s()
else
  error("Load function %s is not defined or not a function")
end
`, watchedScript.LoadFunction, watchedScript.LoadFunction, watchedScript.LoadFunction)
		
		_, err = vm.Execute(loadScript)
		if err != nil {
			sw.setReloadError(watchedScript, fmt.Errorf("load function execution failed: %w", err))
			return
		}
	}
	
	// Log successful reload
	if vm.apiCtx.Logger != nil {
		vm.apiCtx.Logger.Info("Script reloaded successfully", 
			"file", filePath, 
			"vm_id", watchedScript.VMID, 
			"reload_count", watchedScript.ReloadCount)
	}
	
	_ = result // Result is typically nil for script reloads
}

// setReloadError sets an error for a watched script
func (sw *ScriptWatcher) setReloadError(watchedScript *WatchedScript, err error) {
	sw.mutex.Lock()
	watchedScript.LastError = err
	sw.mutex.Unlock()
	
	// Log error if possible
	if vm, exists := sw.vmManager.GetVM(watchedScript.VMID); exists && vm.apiCtx.Logger != nil {
		vm.apiCtx.Logger.Error("Script reload failed", 
			"file", watchedScript.FilePath, 
			"vm_id", watchedScript.VMID, 
			"error", err)
	}
}

// ForceReload manually triggers a reload of a specific script
func (sw *ScriptWatcher) ForceReload(filePath string) error {
	sw.mutex.RLock()
	watchedScript, exists := sw.watchedFiles[filePath]
	sw.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("script %s is not being watched", filePath)
	}
	
	sw.reloadScript(filePath, watchedScript)
	return watchedScript.LastError
}

// GetWatchedScripts returns information about all watched scripts
func (sw *ScriptWatcher) GetWatchedScripts() map[string]*WatchedScript {
	sw.mutex.RLock()
	defer sw.mutex.RUnlock()
	
	// Return a copy to avoid concurrent access issues
	result := make(map[string]*WatchedScript)
	for path, script := range sw.watchedFiles {
		scriptCopy := *script // Shallow copy
		result[path] = &scriptCopy
	}
	
	return result
}

// GetStats returns statistics about the script watcher
func (sw *ScriptWatcher) GetStats() map[string]interface{} {
	sw.mutex.RLock()
	defer sw.mutex.RUnlock()
	
	stats := map[string]interface{}{
		"is_running":      sw.isRunning,
		"poll_interval":   sw.pollInterval,
		"watched_files":   len(sw.watchedFiles),
		"watched_dirs":    len(sw.watchDirs),
		"total_reloads":   int64(0),
		"failed_reloads":  int64(0),
	}
	
	var totalReloads, failedReloads int64
	for _, script := range sw.watchedFiles {
		totalReloads += script.ReloadCount
		if script.LastError != nil {
			failedReloads++
		}
	}
	
	stats["total_reloads"] = totalReloads
	stats["failed_reloads"] = failedReloads
	
	return stats
}

// Helper functions

func getFileInfo(filePath string) (string, time.Time, error) {
	content, err := readScriptFile(filePath)
	if err != nil {
		return "", time.Time{}, err
	}
	
	// Calculate hash
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	
	// Get modification time (simplified - in real implementation, use os.Stat)
	modTime := time.Now() // Placeholder
	
	return hash, modTime, nil
}

func readScriptFile(filePath string) (string, error) {
	// In a real implementation, this would read from the filesystem
	// For now, return a placeholder to avoid actual file I/O
	return fmt.Sprintf("-- Script content from %s\nprint('Hello from %s')", filePath, filePath), nil
}

// WatchDirectory watches all Lua files in a directory
func (sw *ScriptWatcher) WatchDirectory(dirPath, vmID string, autoReload bool) error {
	// In a real implementation, this would scan the directory for .lua files
	// and add them to the watch list
	
	luaFiles := []string{
		filepath.Join(dirPath, "init.lua"),
		filepath.Join(dirPath, "handlers.lua"),
		filepath.Join(dirPath, "utils.lua"),
	}
	
	for _, filePath := range luaFiles {
		err := sw.WatchScript(filePath, vmID, "", autoReload)
		if err != nil {
			// Log error but continue with other files
			continue
		}
	}
	
	return nil
}

// SetAutoReload enables or disables automatic reloading for a script
func (sw *ScriptWatcher) SetAutoReload(filePath string, autoReload bool) error {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	
	watchedScript, exists := sw.watchedFiles[filePath]
	if !exists {
		return fmt.Errorf("script %s is not being watched", filePath)
	}
	
	watchedScript.AutoReload = autoReload
	return nil
}

// ClearErrors clears the last error for all watched scripts
func (sw *ScriptWatcher) ClearErrors() {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	
	for _, script := range sw.watchedFiles {
		script.LastError = nil
	}
}