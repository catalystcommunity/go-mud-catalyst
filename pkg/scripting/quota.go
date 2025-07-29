package scripting

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// QuotaManager manages execution quotas across all VMs
type QuotaManager struct {
	config *ScriptingConfig
	
	// Global resource tracking
	totalCPUUsed    int64 // Atomic counter for total CPU ticks used
	totalMemUsed    int64 // Atomic counter for total memory used
	activeVMs       int32 // Atomic counter for active VMs
	
	// Per-VM quotas
	vmQuotas map[string]*VMQuota
	mutex    sync.RWMutex
	
	// Quota reset mechanism
	resetInterval time.Duration
	lastReset     time.Time
	resetMutex    sync.RWMutex
}

// VMQuota tracks resource usage for a specific VM
type VMQuota struct {
	VMID            string
	CPUUsed         int64     // CPU ticks used in current period
	MemUsed         int64     // Memory used in current period  
	ExecutionCount  int64     // Number of executions in current period
	LastExecution   time.Time // Time of last execution
	IsThrottled     bool      // Whether this VM is currently throttled
	ThrottleUntil   time.Time // When throttling expires
	
	mutex sync.RWMutex
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(config *ScriptingConfig) *QuotaManager {
	return &QuotaManager{
		config:        config,
		vmQuotas:      make(map[string]*VMQuota),
		resetInterval: 1 * time.Minute, // Reset quotas every minute
		lastReset:     time.Now(),
	}
}

// RegisterVM registers a new VM for quota tracking
func (qm *QuotaManager) RegisterVM(vmID string) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	qm.vmQuotas[vmID] = &VMQuota{
		VMID:          vmID,
		LastExecution: time.Now(),
	}
	
	atomic.AddInt32(&qm.activeVMs, 1)
}

// UnregisterVM removes a VM from quota tracking
func (qm *QuotaManager) UnregisterVM(vmID string) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()
	
	if _, exists := qm.vmQuotas[vmID]; exists {
		delete(qm.vmQuotas, vmID)
		atomic.AddInt32(&qm.activeVMs, -1)
	}
}

// CheckExecutionQuota checks if a VM can execute based on current quotas
func (qm *QuotaManager) CheckExecutionQuota(vmID string, estimatedCPU int64, estimatedMem int64) error {
	// Check if quota reset is needed
	qm.maybeResetQuotas()
	
	qm.mutex.RLock()
	quota, exists := qm.vmQuotas[vmID]
	qm.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("VM %s not registered for quota tracking", vmID)
	}
	
	quota.mutex.RLock()
	defer quota.mutex.RUnlock()
	
	// Check if VM is currently throttled
	if quota.IsThrottled && time.Now().Before(quota.ThrottleUntil) {
		return fmt.Errorf("VM %s is throttled until %v", vmID, quota.ThrottleUntil)
	}
	
	// Check global CPU limit
	currentTotalCPU := atomic.LoadInt64(&qm.totalCPUUsed)
	if currentTotalCPU+estimatedCPU > qm.config.GlobalCPULimit {
		return fmt.Errorf("global CPU limit would be exceeded (current: %d, estimated: %d, limit: %d)", 
			currentTotalCPU, estimatedCPU, qm.config.GlobalCPULimit)
	}
	
	// Check global memory limit
	currentTotalMem := atomic.LoadInt64(&qm.totalMemUsed)
	if currentTotalMem+estimatedMem > qm.config.GlobalMemLimit {
		return fmt.Errorf("global memory limit would be exceeded (current: %d, estimated: %d, limit: %d)",
			currentTotalMem, estimatedMem, qm.config.GlobalMemLimit)
	}
	
	// Check per-VM limits (could be configured per VM in future)
	perVMCPULimit := qm.config.DefaultVMConfig.MaxCPUTicks
	if quota.CPUUsed+estimatedCPU > perVMCPULimit {
		return fmt.Errorf("VM %s CPU limit would be exceeded (current: %d, estimated: %d, limit: %d)",
			vmID, quota.CPUUsed, estimatedCPU, perVMCPULimit)
	}
	
	return nil
}

// RecordExecution records resource usage after script execution
func (qm *QuotaManager) RecordExecution(vmID string, cpuUsed int64, memUsed int64, executionTime time.Duration) error {
	qm.mutex.RLock()
	quota, exists := qm.vmQuotas[vmID]
	qm.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("VM %s not registered for quota tracking", vmID)
	}
	
	quota.mutex.Lock()
	defer quota.mutex.Unlock()
	
	// Update VM-specific counters
	quota.CPUUsed += cpuUsed
	quota.MemUsed += memUsed
	quota.ExecutionCount++
	quota.LastExecution = time.Now()
	
	// Update global counters
	atomic.AddInt64(&qm.totalCPUUsed, cpuUsed)
	atomic.AddInt64(&qm.totalMemUsed, memUsed)
	
	// Check if VM should be throttled
	qm.checkThrottling(quota)
	
	return nil
}

// checkThrottling determines if a VM should be throttled based on usage patterns
func (qm *QuotaManager) checkThrottling(quota *VMQuota) {
	// Reset throttling if expired
	if quota.IsThrottled && time.Now().After(quota.ThrottleUntil) {
		quota.IsThrottled = false
	}
	
	// Simple throttling logic: if a VM uses more than 50% of its CPU quota in rapid succession
	perVMCPULimit := qm.config.DefaultVMConfig.MaxCPUTicks
	if quota.CPUUsed > perVMCPULimit/2 && quota.ExecutionCount > 10 {
		timeSinceStart := time.Since(qm.lastReset)
		if timeSinceStart < 30*time.Second { // High usage in short time
			quota.IsThrottled = true
			quota.ThrottleUntil = time.Now().Add(10 * time.Second) // Throttle for 10 seconds
		}
	}
}

// GetQuotaStats returns current quota statistics
func (qm *QuotaManager) GetQuotaStats() map[string]interface{} {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()
	
	vmStats := make(map[string]interface{})
	for vmID, quota := range qm.vmQuotas {
		quota.mutex.RLock()
		vmStats[vmID] = map[string]interface{}{
			"cpu_used":        quota.CPUUsed,
			"mem_used":        quota.MemUsed,
			"execution_count": quota.ExecutionCount,
			"last_execution":  quota.LastExecution,
			"is_throttled":    quota.IsThrottled,
			"throttle_until":  quota.ThrottleUntil,
		}
		quota.mutex.RUnlock()
	}
	
	return map[string]interface{}{
		"total_cpu_used":   atomic.LoadInt64(&qm.totalCPUUsed),
		"total_mem_used":   atomic.LoadInt64(&qm.totalMemUsed),
		"active_vms":       atomic.LoadInt32(&qm.activeVMs),
		"global_cpu_limit": qm.config.GlobalCPULimit,
		"global_mem_limit": qm.config.GlobalMemLimit,
		"last_reset":       qm.lastReset,
		"vm_stats":         vmStats,
	}
}

// maybeResetQuotas resets quotas if enough time has passed
func (qm *QuotaManager) maybeResetQuotas() {
	qm.resetMutex.Lock()
	defer qm.resetMutex.Unlock()
	
	if time.Since(qm.lastReset) >= qm.resetInterval {
		qm.resetQuotasUnsafe()
	}
}

// resetQuotasUnsafe resets all quotas (caller must hold resetMutex)
func (qm *QuotaManager) resetQuotasUnsafe() {
	// Reset global counters
	atomic.StoreInt64(&qm.totalCPUUsed, 0)
	atomic.StoreInt64(&qm.totalMemUsed, 0)
	
	// Reset per-VM quotas
	qm.mutex.Lock()
	for _, quota := range qm.vmQuotas {
		quota.mutex.Lock()
		quota.CPUUsed = 0
		quota.MemUsed = 0
		quota.ExecutionCount = 0
		// Don't reset throttling or last execution time
		quota.mutex.Unlock()
	}
	qm.mutex.Unlock()
	
	qm.lastReset = time.Now()
}

// ForceResetQuotas immediately resets all quotas
func (qm *QuotaManager) ForceResetQuotas() {
	qm.resetMutex.Lock()
	defer qm.resetMutex.Unlock()
	
	qm.resetQuotasUnsafe()
}

// SetThrottling manually sets throttling for a VM
func (qm *QuotaManager) SetThrottling(vmID string, throttled bool, duration time.Duration) error {
	qm.mutex.RLock()
	quota, exists := qm.vmQuotas[vmID]
	qm.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("VM %s not registered for quota tracking", vmID)
	}
	
	quota.mutex.Lock()
	defer quota.mutex.Unlock()
	
	quota.IsThrottled = throttled
	if throttled {
		quota.ThrottleUntil = time.Now().Add(duration)
	} else {
		quota.ThrottleUntil = time.Time{}
	}
	
	return nil
}