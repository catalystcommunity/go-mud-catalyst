package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// StorageConfig holds configuration for the storage system
type StorageConfig struct {
	DataRoot string
}

// DefaultStorageConfig returns default storage configuration
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		DataRoot: "./data",
	}
}

// StorageManager manages file-based storage with configurable data root
type StorageManager struct {
	config *StorageConfig
	mutex  sync.RWMutex
}

// NewStorageManager creates a new storage manager with default config
func NewStorageManager() *StorageManager {
	return NewStorageManagerWithConfig(DefaultStorageConfig())
}

// NewStorageManagerWithConfig creates a new storage manager with custom config
func NewStorageManagerWithConfig(config *StorageConfig) *StorageManager {
	return &StorageManager{
		config: config,
	}
}

// Initialize creates the necessary directory structure
func (sm *StorageManager) Initialize() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	return sm.initializeDirectories()
}

// initializeDirectories creates the necessary directory structure without locking
func (sm *StorageManager) initializeDirectories() error {
	directories := []string{
		"players",
		"items",
		"inventories",
		"worlds",
		"roles",
	}

	for _, dir := range directories {
		fullPath := filepath.Join(sm.config.DataRoot, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
	}

	return nil
}

// GetDataRoot returns the configured data root path
func (sm *StorageManager) GetDataRoot() string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.config.DataRoot
}

// GetPlayersPath returns the path to the players directory
func (sm *StorageManager) GetPlayersPath() string {
	return filepath.Join(sm.GetDataRoot(), "players")
}

// GetItemsPath returns the path to the items directory
func (sm *StorageManager) GetItemsPath() string {
	return filepath.Join(sm.GetDataRoot(), "items")
}

// GetInventoriesPath returns the path to the inventories directory
func (sm *StorageManager) GetInventoriesPath() string {
	return filepath.Join(sm.GetDataRoot(), "inventories")
}

// GetWorldsPath returns the path to the worlds directory
func (sm *StorageManager) GetWorldsPath() string {
	return filepath.Join(sm.GetDataRoot(), "worlds")
}

// GetRolesPath returns the path to the roles directory
func (sm *StorageManager) GetRolesPath() string {
	return filepath.Join(sm.GetDataRoot(), "roles")
}

// SaveJSON saves an object as JSON to the specified path
func (sm *StorageManager) SaveJSON(relativePath string, data interface{}) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	fullPath := filepath.Join(sm.config.DataRoot, relativePath)
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write to file
	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	return nil
}

// LoadJSON loads JSON data from the specified path into the provided interface
func (sm *StorageManager) LoadJSON(relativePath string, data interface{}) error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	fullPath := filepath.Join(sm.config.DataRoot, relativePath)
	
	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", fullPath)
	}

	// Read file
	fileData, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}

	// Unmarshal JSON
	if err := json.Unmarshal(fileData, data); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %w", fullPath, err)
	}

	return nil
}

// FileExists checks if a file exists at the specified path
func (sm *StorageManager) FileExists(relativePath string) bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	fullPath := filepath.Join(sm.config.DataRoot, relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// DeleteFile deletes a file at the specified path
func (sm *StorageManager) DeleteFile(relativePath string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	fullPath := filepath.Join(sm.config.DataRoot, relativePath)
	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", fullPath, err)
	}

	return nil
}

// ListFiles lists all files in the specified directory
func (sm *StorageManager) ListFiles(relativePath string) ([]string, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	fullPath := filepath.Join(sm.config.DataRoot, relativePath)
	
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", fullPath, err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ListDirectories lists all directories in the specified directory
func (sm *StorageManager) ListDirectories(relativePath string) ([]string, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	fullPath := filepath.Join(sm.config.DataRoot, relativePath)
	
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", fullPath, err)
	}

	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, entry.Name())
		}
	}

	return directories, nil
}

// UpdateConfig updates the storage configuration
func (sm *StorageManager) UpdateConfig(config *StorageConfig) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.config = config
	return sm.initializeDirectories()
}

// GetConfig returns the current storage configuration
func (sm *StorageManager) GetConfig() *StorageConfig {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return &StorageConfig{
		DataRoot: sm.config.DataRoot,
	}
}