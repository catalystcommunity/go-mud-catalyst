package scripting

import (
	"encoding/json"
	"fmt"
	"time"
)

// ScriptingConfig holds global configuration for the scripting system
type ScriptingConfig struct {
	// Global VM settings
	DefaultVMConfig *VMConfig `json:"default_vm_config"`
	
	// VM pool settings
	MaxVMs           int           `json:"max_vms"`            // Maximum number of VMs
	VMIdleTimeout    time.Duration `json:"vm_idle_timeout"`   // How long before idle VMs are cleaned up
	CleanupInterval  time.Duration `json:"cleanup_interval"`  // How often to run cleanup
	
	// Security settings
	GlobalCPULimit   int64         `json:"global_cpu_limit"`   // Total CPU ticks across all VMs
	GlobalMemLimit   int64         `json:"global_mem_limit"`   // Total memory across all VMs
	
	// Script management
	MaxScriptSize    int           `json:"max_script_size"`    // Maximum script size in bytes
	ScriptCacheSize  int           `json:"script_cache_size"`  // Number of compiled scripts to cache
}

// DefaultScriptingConfig returns a safe default configuration
func DefaultScriptingConfig() *ScriptingConfig {
	return &ScriptingConfig{
		DefaultVMConfig:  DefaultVMConfig(),
		MaxVMs:          100,
		VMIdleTimeout:   30 * time.Minute,
		CleanupInterval: 5 * time.Minute,
		GlobalCPULimit:  100000000, // 100M ticks total
		GlobalMemLimit:  1024 * 1024 * 1024, // 1GB total
		MaxScriptSize:   1024 * 1024, // 1MB per script
		ScriptCacheSize: 1000,
	}
}

// Validate checks if the configuration is valid
func (c *ScriptingConfig) Validate() error {
	if c.DefaultVMConfig == nil {
		return fmt.Errorf("default_vm_config cannot be nil")
	}
	
	if c.MaxVMs <= 0 {
		return fmt.Errorf("max_vms must be positive")
	}
	
	if c.VMIdleTimeout <= 0 {
		return fmt.Errorf("vm_idle_timeout must be positive")
	}
	
	if c.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup_interval must be positive")
	}
	
	if c.GlobalCPULimit <= 0 {
		return fmt.Errorf("global_cpu_limit must be positive")
	}
	
	if c.GlobalMemLimit <= 0 {
		return fmt.Errorf("global_mem_limit must be positive")
	}
	
	if c.MaxScriptSize <= 0 {
		return fmt.Errorf("max_script_size must be positive")
	}
	
	return c.DefaultVMConfig.Validate()
}

// Validate checks if the VM configuration is valid
func (c *VMConfig) Validate() error {
	if c.MaxCPUTicks <= 0 {
		return fmt.Errorf("max_cpu_ticks must be positive")
	}
	
	if c.MaxWallTime <= 0 {
		return fmt.Errorf("max_wall_time must be positive")
	}
	
	if c.MaxMemoryBytes <= 0 {
		return fmt.Errorf("max_memory_bytes must be positive")
	}
	
	if c.MaxStackDepth <= 0 {
		return fmt.Errorf("max_stack_depth must be positive")
	}
	
	return nil
}

// ToJSON serializes the configuration to JSON
func (c *ScriptingConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// FromJSON deserializes the configuration from JSON
func (c *ScriptingConfig) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// Clone creates a deep copy of the configuration
func (c *ScriptingConfig) Clone() *ScriptingConfig {
	clone := &ScriptingConfig{
		DefaultVMConfig: &VMConfig{
			MaxCPUTicks:     c.DefaultVMConfig.MaxCPUTicks,
			MaxWallTime:     c.DefaultVMConfig.MaxWallTime,
			MaxMemoryBytes:  c.DefaultVMConfig.MaxMemoryBytes,
			MaxStackDepth:   c.DefaultVMConfig.MaxStackDepth,
			AllowFileIO:     c.DefaultVMConfig.AllowFileIO,
			AllowNetworking: c.DefaultVMConfig.AllowNetworking,
		},
		MaxVMs:          c.MaxVMs,
		VMIdleTimeout:   c.VMIdleTimeout,
		CleanupInterval: c.CleanupInterval,
		GlobalCPULimit:  c.GlobalCPULimit,
		GlobalMemLimit:  c.GlobalMemLimit,
		MaxScriptSize:   c.MaxScriptSize,
		ScriptCacheSize: c.ScriptCacheSize,
	}
	
	return clone
}

// ApplyProfile applies a predefined security profile to the configuration
func (c *ScriptingConfig) ApplyProfile(profile SecurityProfile) {
	switch profile {
	case ProfileStrict:
		c.DefaultVMConfig.MaxCPUTicks = 100000     // Very limited CPU
		c.DefaultVMConfig.MaxWallTime = 1 * time.Second
		c.DefaultVMConfig.MaxMemoryBytes = 1024 * 1024 // 1MB
		c.DefaultVMConfig.AllowFileIO = false
		c.DefaultVMConfig.AllowNetworking = false
		c.MaxScriptSize = 64 * 1024 // 64KB
		
	case ProfileModerate:
		c.DefaultVMConfig.MaxCPUTicks = 1000000    // 1M ticks
		c.DefaultVMConfig.MaxWallTime = 5 * time.Second
		c.DefaultVMConfig.MaxMemoryBytes = 10 * 1024 * 1024 // 10MB
		c.DefaultVMConfig.AllowFileIO = false
		c.DefaultVMConfig.AllowNetworking = false
		c.MaxScriptSize = 1024 * 1024 // 1MB
		
	case ProfilePermissive:
		c.DefaultVMConfig.MaxCPUTicks = 10000000   // 10M ticks
		c.DefaultVMConfig.MaxWallTime = 30 * time.Second
		c.DefaultVMConfig.MaxMemoryBytes = 100 * 1024 * 1024 // 100MB
		c.DefaultVMConfig.AllowFileIO = true
		c.DefaultVMConfig.AllowNetworking = false
		c.MaxScriptSize = 10 * 1024 * 1024 // 10MB
	}
}

// SecurityProfile represents predefined security configurations
type SecurityProfile int

const (
	ProfileStrict SecurityProfile = iota
	ProfileModerate
	ProfilePermissive
)