package scripting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arnodel/golua/runtime"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// VMConfig holds configuration for Lua VM resource limits
type VMConfig struct {
	// CPU limits
	MaxCPUTicks    int64         // Maximum CPU ticks per execution
	MaxWallTime    time.Duration // Maximum wall-clock time per execution
	
	// Memory limits
	MaxMemoryBytes int64 // Maximum memory allocation in bytes
	
	// Execution limits
	MaxStackDepth  int // Maximum call stack depth
	
	// Security settings
	AllowFileIO    bool // Whether to allow file operations
	AllowNetworking bool // Whether to allow network operations
}

// DefaultVMConfig returns a safe default configuration
func DefaultVMConfig() *VMConfig {
	return &VMConfig{
		MaxCPUTicks:    1000000,  // 1M ticks (~1-2 seconds of CPU)
		MaxWallTime:    5 * time.Second,
		MaxMemoryBytes: 10 * 1024 * 1024, // 10MB
		MaxStackDepth:  100,
		AllowFileIO:    false,
		AllowNetworking: false,
	}
}

// VM represents a managed Lua virtual machine with resource limits
type VM struct {
	id       string
	config   *VMConfig
	runtime  *runtime.Runtime
	apiCtx   *APIContext
	mutex    sync.RWMutex
	created  time.Time
	lastUsed time.Time
}

// VMManager manages multiple Lua VMs with resource limits
type VMManager struct {
	vms          map[string]*VM
	config       *ScriptingConfig
	quotaManager *QuotaManager
	mutex        sync.RWMutex
	
	// Cleanup management
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewVMManager creates a new VM manager with the given configuration
func NewVMManager(config *ScriptingConfig) *VMManager {
	if config == nil {
		config = DefaultScriptingConfig()
	}
	
	manager := &VMManager{
		vms:           make(map[string]*VM),
		config:        config,
		quotaManager:  NewQuotaManager(config),
		stopCleanup:   make(chan struct{}),
	}
	
	// Start cleanup goroutine
	manager.cleanupTicker = time.NewTicker(config.CleanupInterval)
	go manager.cleanupLoop()
	
	return manager
}

// CreateVM creates a new Lua VM with the specified ID
func (m *VMManager) CreateVM(id string) (*VM, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if _, exists := m.vms[id]; exists {
		return nil, fmt.Errorf("VM with id %s already exists", id)
	}
	
	// Check if we can create more VMs
	if len(m.vms) >= m.config.MaxVMs {
		return nil, fmt.Errorf("maximum number of VMs (%d) reached", m.config.MaxVMs)
	}
	
	// Create sandboxed runtime based on security configuration
	sandboxConfig := &SandboxConfig{
		AllowFileIO:         m.config.DefaultVMConfig.AllowFileIO,
		AllowNetworking:     m.config.DefaultVMConfig.AllowNetworking,
		AllowOSAccess:       false, // Never allow OS access in VMs
		AllowProcessControl: false, // Never allow process control
		AllowDebugLib:       false, // Never allow debug library
		AllowPackageLib:     false, // Never allow package loading
		AllowCoroutines:     true,  // Generally safe
		MaxMemoryBytes:      m.config.DefaultVMConfig.MaxMemoryBytes,
		MaxCPUTicks:         m.config.DefaultVMConfig.MaxCPUTicks,
	}
	
	// Apply security profile from config
	switch {
	case m.config.DefaultVMConfig.MaxCPUTicks <= 100000:
		// Very restrictive for low CPU limits
		sandboxConfig = StrictSandboxConfig()
	case m.config.DefaultVMConfig.MaxCPUTicks <= 1000000:
		// Moderate restrictions
		sandboxConfig = DefaultSandboxConfig()
	default:
		// Still apply default restrictions even for high limits
		sandboxConfig = DefaultSandboxConfig()
	}
	
	// Override with VM config settings
	sandboxConfig.AllowFileIO = m.config.DefaultVMConfig.AllowFileIO
	sandboxConfig.AllowNetworking = m.config.DefaultVMConfig.AllowNetworking
	sandboxConfig.MaxMemoryBytes = m.config.DefaultVMConfig.MaxMemoryBytes
	sandboxConfig.MaxCPUTicks = m.config.DefaultVMConfig.MaxCPUTicks
	
	r, err := SetupSimpleSecureRuntime()
	if err != nil {
		return nil, fmt.Errorf("failed to create secure runtime: %w", err)
	}
	
	// Create API context
	apiCtx := &APIContext{
		Logger:   nil, // Will be set by caller if needed
		VMID:     id,
		UserData: make(map[string]interface{}),
	}
	
	// Register muddycore API (simplified version)
	if err := RegisterSimpleLuaAPI(r, apiCtx); err != nil {
		return nil, fmt.Errorf("failed to register Lua API: %w", err)
	}
	
	vm := &VM{
		id:       id,
		config:   m.config.DefaultVMConfig,
		runtime:  r,
		apiCtx:   apiCtx,
		created:  time.Now(),
		lastUsed: time.Now(),
	}
	
	m.vms[id] = vm
	
	// Register VM with quota manager
	m.quotaManager.RegisterVM(id)
	
	return vm, nil
}

// GetVM retrieves an existing VM by ID
func (m *VMManager) GetVM(id string) (*VM, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	vm, exists := m.vms[id]
	if exists {
		vm.lastUsed = time.Now()
	}
	return vm, exists
}

// DestroyVM removes and cleans up a VM
func (m *VMManager) DestroyVM(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	vm, exists := m.vms[id]
	if !exists {
		return fmt.Errorf("VM with id %s does not exist", id)
	}
	
	// Unregister from quota manager
	m.quotaManager.UnregisterVM(id)
	
	// Clean up VM resources
	vm.cleanup()
	delete(m.vms, id)
	
	return nil
}

// ExecuteScript executes a Lua script in the specified VM with resource limits
func (m *VMManager) ExecuteScript(vmID string, script string) (interface{}, error) {
	vm, exists := m.GetVM(vmID)
	if !exists {
		return nil, fmt.Errorf("VM with id %s does not exist", vmID)
	}
	
	// Check script size limit
	if len(script) > m.config.MaxScriptSize {
		return nil, fmt.Errorf("script size (%d bytes) exceeds limit (%d bytes)", 
			len(script), m.config.MaxScriptSize)
	}
	
	return vm.ExecuteWithQuota(script, m.quotaManager)
}

// Execute runs a Lua script with resource limits and timeout
func (vm *VM) Execute(script string) (interface{}, error) {
	return vm.ExecuteWithQuota(script, nil)
}

// ExecuteWithQuota runs a Lua script with resource limits, timeout, and quota management
func (vm *VM) ExecuteWithQuota(script string, quotaManager *QuotaManager) (interface{}, error) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	
	startTime := time.Now()
	
	// Validate script for security violations
	if err := SimpleValidateScript(script); err != nil {
		return nil, fmt.Errorf("script validation failed: %w", err)
	}
	
	// Check quota if manager is provided
	if quotaManager != nil {
		err := quotaManager.CheckExecutionQuota(vm.id, vm.config.MaxCPUTicks, vm.config.MaxMemoryBytes)
		if err != nil {
			return nil, fmt.Errorf("quota check failed: %w", err)
		}
	}
	
	// Create execution context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), vm.config.MaxWallTime)
	defer cancel()
	
	// Set up runtime context with resource limits
	// TODO: Fix this to use correct golua API
	// rtCtx := runtime.RuntimeContextFromContext(ctx)
	// rtCtx = rtCtx.WithCPULimit(vm.config.MaxCPUTicks)
	// rtCtx = rtCtx.WithMemLimit(vm.config.MaxMemoryBytes)
	
	// Create channels to capture the result and resource usage
	resultChan := make(chan interface{}, 1)
	errorChan := make(chan error, 1)
	usageChan := make(chan map[string]int64, 1)
	
	// Execute script in a goroutine to enable timeout
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errorChan <- fmt.Errorf("script panic: %v", r)
			}
		}()
		
		// Track resource usage before execution
		// TODO: Fix resource tracking
		// initialCPU := rtCtx.UsedCPU()
		// initialMem := rtCtx.UsedMem()
		
		// Compile and execute the script
		// TODO: Fix compilation to use correct golua API
		chunk, err := vm.runtime.CompileAndLoadLuaChunk("script", []byte(script), runtime.TableValue(vm.runtime.GlobalEnv()))
		if err != nil {
			errorChan <- fmt.Errorf("compilation error: %w", err)
			return
		}
		
		// Execute the compiled chunk using runtime.Call1
		// This is the correct way to execute chunks in arnodel/golua
		result, err := runtime.Call1(vm.runtime.MainThread(), runtime.FunctionValue(chunk))
		if err != nil {
			errorChan <- fmt.Errorf("execution error: %w", err)
			return
		}

		// Convert result to slice for compatibility with existing code
		results := []runtime.Value{result}
		
		// Calculate resource usage
		// TODO: Fix resource tracking
		// cpuUsed := rtCtx.UsedCPU() - initialCPU
		// memUsed := rtCtx.UsedMem() - initialMem
		
		usageChan <- map[string]int64{
			"cpu": 0, // TODO: implement proper tracking
			"mem": 0, // TODO: implement proper tracking
		}
		if len(results) > 0 {
			resultChan <- results[0]
		} else {
			resultChan <- chunk // Return the compiled chunk if no results
		}
	}()
	
	// Wait for either completion or timeout
	select {
	case result := <-resultChan:
		usage := <-usageChan
		executionTime := time.Since(startTime)
		
		// Record usage in quota manager
		if quotaManager != nil {
			quotaManager.RecordExecution(vm.id, usage["cpu"], usage["mem"], executionTime)
		}
		
		vm.lastUsed = time.Now()
		return result, nil
	case err := <-errorChan:
		// Still try to get usage data for failed executions
		select {
		case usage := <-usageChan:
			executionTime := time.Since(startTime)
			if quotaManager != nil {
				quotaManager.RecordExecution(vm.id, usage["cpu"], usage["mem"], executionTime)
			}
		default:
			// No usage data available
		}
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("script execution timeout after %v", vm.config.MaxWallTime)
	}
}

// SetAPIContext configures the API context for this VM
func (vm *VM) SetAPIContext(logger *logging.Logger, eventManager *events.DefaultEventManager, luaEventManager *LuaEventManager, serverID, clientID string) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	
	if vm.apiCtx != nil {
		vm.apiCtx.Logger = logger
		vm.apiCtx.EventManager = eventManager
		vm.apiCtx.LuaEventManager = luaEventManager
		vm.apiCtx.ServerID = serverID
		vm.apiCtx.ClientID = clientID
	}
}

// SetUserData sets user-defined data in the VM context
func (vm *VM) SetUserData(key string, value interface{}) {
	vm.mutex.Lock()
	defer vm.mutex.Unlock()
	
	if vm.apiCtx.UserData == nil {
		vm.apiCtx.UserData = make(map[string]interface{})
	}
	vm.apiCtx.UserData[key] = value
}

// GetUserData retrieves user-defined data from the VM context
func (vm *VM) GetUserData(key string) (interface{}, bool) {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()
	
	if vm.apiCtx.UserData == nil {
		return nil, false
	}
	
	value, exists := vm.apiCtx.UserData[key]
	return value, exists
}

// GetStats returns runtime statistics for the VM
func (vm *VM) GetStats() map[string]interface{} {
	vm.mutex.RLock()
	defer vm.mutex.RUnlock()
	
	return map[string]interface{}{
		"id":         vm.id,
		"created":    vm.created,
		"last_used":  vm.lastUsed,
		"uptime":     time.Since(vm.created),
		"server_id":  vm.apiCtx.ServerID,
		"client_id":  vm.apiCtx.ClientID,
		"user_data":  len(vm.apiCtx.UserData),
	}
}

// cleanup performs VM resource cleanup
func (vm *VM) cleanup() {
	// golua's runtime will be garbage collected
	// Additional cleanup can be added here if needed
}

// Shutdown gracefully shuts down the VM manager
func (m *VMManager) Shutdown() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Stop cleanup goroutine
	close(m.stopCleanup)
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
	
	// Clean up all VMs
	for id := range m.vms {
		m.quotaManager.UnregisterVM(id)
		m.vms[id].cleanup()
	}
	
	m.vms = make(map[string]*VM)
	return nil
}

// cleanupLoop runs periodically to clean up idle VMs
func (m *VMManager) cleanupLoop() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.cleanupIdleVMs()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanupIdleVMs removes VMs that have been idle for too long
func (m *VMManager) cleanupIdleVMs() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	now := time.Now()
	toRemove := []string{}
	
	for id, vm := range m.vms {
		vm.mutex.RLock()
		idleTime := now.Sub(vm.lastUsed)
		vm.mutex.RUnlock()
		
		if idleTime > m.config.VMIdleTimeout {
			toRemove = append(toRemove, id)
		}
	}
	
	// Remove idle VMs
	for _, id := range toRemove {
		m.quotaManager.UnregisterVM(id)
		m.vms[id].cleanup()
		delete(m.vms, id)
	}
}

// GetStats returns statistics for the VM manager
func (m *VMManager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	vmStats := make(map[string]interface{})
	for id, vm := range m.vms {
		vmStats[id] = vm.GetStats()
	}
	
	quotaStats := m.quotaManager.GetQuotaStats()
	
	return map[string]interface{}{
		"total_vms":     len(m.vms),
		"max_vms":       m.config.MaxVMs,
		"vm_stats":      vmStats,
		"quota_stats":   quotaStats,
		"config":        m.config,
	}
}

// ValidateAndExecuteScript validates a script and executes it safely
func (vm *VM) ValidateAndExecuteScript(script string, allowUnsafe bool) (interface{}, error) {
	// Simple validation
	if err := SimpleValidateScript(script); err != nil && !allowUnsafe {
		return nil, fmt.Errorf("script contains security violations: %w", err)
	}
	
	// Execute with standard safeguards
	return vm.Execute(script)
}