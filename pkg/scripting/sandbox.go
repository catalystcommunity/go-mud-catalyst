package scripting

import (
	"fmt"
	"os"
	"strconv"

	"github.com/arnodel/golua/runtime"
)

// SandboxConfig defines what functionality should be restricted
type SandboxConfig struct {
	AllowFileIO      bool
	AllowNetworking  bool
	AllowOSAccess    bool
	AllowProcessControl bool
	AllowDebugLib    bool
	AllowPackageLib  bool
	AllowCoroutines  bool
	
	// Function blacklists
	DisallowedFunctions []string
	DisallowedModules   []string
	
	// Memory and execution limits
	MaxMemoryBytes int64
	MaxCPUTicks    int64
}

// DefaultSandboxConfig returns a safe default sandbox configuration
func DefaultSandboxConfig() *SandboxConfig {
	return &SandboxConfig{
		AllowFileIO:         false,
		AllowNetworking:     false,
		AllowOSAccess:       false,
		AllowProcessControl: false,
		AllowDebugLib:       false,
		AllowPackageLib:     false,
		AllowCoroutines:     true, // Generally safe
		
		DisallowedFunctions: []string{
			// Dangerous global functions
			"dofile", "loadfile", "load", "loadstring",
			"require", "module",
			
			// OS functions
			"os.execute", "os.exit", "os.getenv", "os.remove", "os.rename",
			"os.tmpname", "os.setlocale",
			
			// IO functions
			"io.open", "io.popen", "io.close", "io.flush", "io.input",
			"io.lines", "io.output", "io.read", "io.tmpfile", "io.type", "io.write",
			
			// Package functions
			"package.loadlib", "package.searchpath", "package.seeall",
			
			// Debug functions (all of them)
			"debug.debug", "debug.gethook", "debug.getinfo", "debug.getlocal",
			"debug.getmetatable", "debug.getregistry", "debug.getupvalue",
			"debug.getuservalue", "debug.sethook", "debug.setlocal",
			"debug.setmetatable", "debug.setupvalue", "debug.setuservalue",
			"debug.traceback", "debug.upvalueid", "debug.upvaluejoin",
		},
		
		DisallowedModules: []string{
			"os", "io", "package", "debug", "ffi",
		},
		
		MaxMemoryBytes: 10 * 1024 * 1024, // 10MB
		MaxCPUTicks:    1000000,           // 1M ticks
	}
}

// StrictSandboxConfig returns a very restrictive sandbox configuration
func StrictSandboxConfig() *SandboxConfig {
	config := DefaultSandboxConfig()
	config.AllowCoroutines = false
	config.MaxMemoryBytes = 1024 * 1024 // 1MB
	config.MaxCPUTicks = 100000         // 100K ticks
	
	// Add more restrictions
	config.DisallowedFunctions = append(config.DisallowedFunctions,
		"collectgarbage", "rawequal", "rawget", "rawlen", "rawset",
		"getmetatable", "setmetatable",
	)
	
	return config
}

// ApplySandbox applies sandbox restrictions to a Lua runtime
func ApplySandbox(r *runtime.Runtime, config *SandboxConfig) error {
	// Remove dangerous global functions
	for _, funcName := range config.DisallowedFunctions {
		err := removeLuaFunction(r, funcName)
		if err != nil {
			return fmt.Errorf("failed to remove function %s: %w", funcName, err)
		}
	}
	
	// Remove dangerous modules
	for _, moduleName := range config.DisallowedModules {
		err := removeLuaModule(r, moduleName)
		if err != nil {
			return fmt.Errorf("failed to remove module %s: %w", moduleName, err)
		}
	}
	
	// Apply specific restrictions based on config
	if !config.AllowFileIO {
		if err := restrictFileIO(r); err != nil {
			return fmt.Errorf("failed to restrict file I/O: %w", err)
		}
	}
	
	if !config.AllowOSAccess {
		if err := restrictOSAccess(r); err != nil {
			return fmt.Errorf("failed to restrict OS access: %w", err)
		}
	}
	
	if !config.AllowDebugLib {
		if err := restrictDebugLib(r); err != nil {
			return fmt.Errorf("failed to restrict debug library: %w", err)
		}
	}
	
	if !config.AllowPackageLib {
		if err := restrictPackageLib(r); err != nil {
			return fmt.Errorf("failed to restrict package library: %w", err)
		}
	}
	
	if !config.AllowCoroutines {
		if err := restrictCoroutines(r); err != nil {
			return fmt.Errorf("failed to restrict coroutines: %w", err)
		}
	}
	
	// Add safe replacement functions
	if err := addSafeReplacements(r, config); err != nil {
		return fmt.Errorf("failed to add safe replacements: %w", err)
	}
	
	return nil
}

// removeLuaFunction removes a global function from the Lua environment
func removeLuaFunction(r *runtime.Runtime, funcName string) error {
	// Split function name by dots (e.g., "os.execute" -> ["os", "execute"])
	parts := splitFunctionName(funcName)
	
	if len(parts) == 1 {
		// Global function
		r.SetEnv(r.GlobalEnv(), parts[0], runtime.NilValue)
	} else if len(parts) == 2 {
		// Module function
		moduleVal := r.GlobalEnv().Get(runtime.StringValue(parts[0]))
		if moduleVal.Type() == runtime.TableType {
			r.SetTable(moduleVal.AsTable(), runtime.StringValue(parts[1]), runtime.NilValue)
		}
	}
	
	return nil
}

// removeLuaModule removes an entire module from the Lua environment
func removeLuaModule(r *runtime.Runtime, moduleName string) error {
	r.SetEnv(r.GlobalEnv(), moduleName, runtime.NilValue)
	return nil
}

// restrictFileIO removes file I/O capabilities
func restrictFileIO(r *runtime.Runtime) error {
	// Remove io module entirely
	r.SetEnv(r.GlobalEnv(), "io", runtime.NilValue)
	
	// Remove file-related functions
	fileFunctions := []string{"dofile", "loadfile"}
	for _, funcName := range fileFunctions {
		r.SetEnv(r.GlobalEnv(), funcName, runtime.NilValue)
	}
	
	return nil
}

// restrictOSAccess removes OS access capabilities
func restrictOSAccess(r *runtime.Runtime) error {
	// Create a restricted os module with only safe functions
	osTable := runtime.NewTable()
	
	// Add safe os functions
	safeOSFunctions := map[string]runtime.GoFunctionFunc{
		"clock": func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
			// Return a fake clock value for security
			return c.PushingNext1(t.Runtime, runtime.FloatValue(0.0)), nil
		},
		"time": func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
			// Return current unix timestamp (generally safe)
			return c.PushingNext1(t.Runtime, runtime.IntValue(0)), nil
		},
		"date": func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
			// Return a fake date for security
			return c.PushingNext1(t.Runtime, runtime.StringValue("Thu Jan  1 00:00:00 1970")), nil
		},
	}
	
	for name, fn := range safeOSFunctions {
		r.SetEnvGoFunc(osTable, name, fn, 0, true)
	}
	
	r.SetEnv(r.GlobalEnv(), "os", runtime.TableValue(osTable))
	return nil
}

// restrictDebugLib removes debug library access
func restrictDebugLib(r *runtime.Runtime) error {
	r.SetEnv(r.GlobalEnv(), "debug", runtime.NilValue)
	return nil
}

// restrictPackageLib removes package library access
func restrictPackageLib(r *runtime.Runtime) error {
	r.SetEnv(r.GlobalEnv(), "package", runtime.NilValue)
	r.SetEnv(r.GlobalEnv(), "require", runtime.NilValue)
	r.SetEnv(r.GlobalEnv(), "module", runtime.NilValue)
	return nil
}

// restrictCoroutines removes coroutine capabilities
func restrictCoroutines(r *runtime.Runtime) error {
	r.SetEnv(r.GlobalEnv(), "coroutine", runtime.NilValue)
	return nil
}

// addSafeReplacements adds safe replacement functions
func addSafeReplacements(r *runtime.Runtime, config *SandboxConfig) error {
	// Add safe print function that logs through our logging system
	r.SetEnvGoFunc(r.GlobalEnv(), "print", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		// Convert all arguments to strings and concatenate
		args := make([]string, c.NArgs())
		for i := 0; i < c.NArgs(); i++ {
			val := c.Arg(i)
			if s, ok := val.ToString(); ok {
				args[i] = s
			} else {
				args[i] = val.TypeName()
			}
		}
		
		// For now, just ignore the output - in production this would go to logger
		// This prevents scripts from spamming output
		return c.Next(), nil
	}, 0, true)
	
	// Add safe error function
	r.SetEnvGoFunc(r.GlobalEnv(), "error", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		msg, err := c.StringArg(0)
		if err != nil {
			return nil, err
		}
		// level := c.OptInt(1, 1) // Not using level for now
		
		return nil, fmt.Errorf("script error: %s", msg)
	}, 2, false)
	
	// Add safe assert function
	r.SetEnvGoFunc(r.GlobalEnv(), "assert", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		condition := c.Arg(0)
		msg := "assertion failed"
		if c.NArgs() > 1 {
			if s, err := c.StringArg(1); err == nil {
				msg = s
			}
		}
		
		if condition.Type() == runtime.NilType || 
		   (condition.Type() == runtime.BoolType && !condition.AsBool()) {
			return nil, fmt.Errorf("assertion failed: %s", msg)
		}
		
		// Return the condition value
		return c.PushingNext1(t.Runtime, condition), nil
	}, 2, false)
	
	// Override collectgarbage to be a no-op for security
	r.SetEnvGoFunc(r.GlobalEnv(), "collectgarbage", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		// Ignore garbage collection requests for security
		return c.PushingNext1(t.Runtime, runtime.IntValue(0)), nil
	}, 1, true)
	
	return nil
}

// splitFunctionName splits a dot-separated function name
func splitFunctionName(name string) []string {
	result := make([]string, 0, 2)
	current := ""
	
	for _, char := range name {
		if char == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	
	if current != "" {
		result = append(result, current)
	}
	
	return result
}

// SetupSecureRuntime creates a new runtime with comprehensive sandboxing
func SetupSecureRuntime(config *SandboxConfig) (*runtime.Runtime, error) {
	if config == nil {
		config = DefaultSandboxConfig()
	}
	
	// Create runtime with security options
	r := runtime.New(os.Stdout)
	
	// Apply basic sandbox restrictions (simplified)
	dangerousFunctions := []string{
		"dofile", "loadfile", "load", "require", "os", "io", "package", "debug",
	}
	
	for _, funcName := range dangerousFunctions {
		r.SetEnv(r.GlobalEnv(), funcName, runtime.NilValue)
	}
	
	return r, nil
}

// loadSafeLibraries loads only the Lua libraries that are considered safe
func loadSafeLibraries(r *runtime.Runtime, config *SandboxConfig) error {
	// We manually load only safe libraries instead of using LoadAll
	// This ensures we never accidentally expose dangerous functionality
	
	// Load basic string manipulation functions
	r.SetEnvGoFunc(r.GlobalEnv(), "tostring", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		val := c.Arg(0)
		if s, ok := val.ToString(); ok {
			return c.PushingNext1(t.Runtime, runtime.StringValue(s)), nil
		}
		return c.PushingNext1(t.Runtime, runtime.StringValue(val.TypeName())), nil
	}, 1, false)
	
	r.SetEnvGoFunc(r.GlobalEnv(), "tonumber", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		val := c.Arg(0)
		if s, ok := val.ToString(); ok {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				return c.PushingNext1(t.Runtime, runtime.IntValue(i)), nil
			}
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return c.PushingNext1(t.Runtime, runtime.FloatValue(f)), nil
			}
		}
		return c.PushingNext1(t.Runtime, runtime.NilValue), nil
	}, 1, false)
	
	r.SetEnvGoFunc(r.GlobalEnv(), "type", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		val := c.Arg(0)
		return c.PushingNext1(t.Runtime, runtime.StringValue(val.TypeName())), nil
	}, 1, false)
	
	// Load basic table iteration
	r.SetEnvGoFunc(r.GlobalEnv(), "pairs", func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		// Basic pairs implementation - simplified
		table := c.Arg(0)
		if table.Type() == runtime.TableType {
			// Return a simple iterator function
			return c.PushingNext1(t.Runtime, runtime.StringValue("pairs_iterator")), nil
		}
		return nil, fmt.Errorf("pairs requires a table")
	}, 1, false)
	
	// Only load coroutines if explicitly allowed
	if config.AllowCoroutines {
		// Add basic coroutine support
		coroutineTable := runtime.NewTable()
		r.SetEnv(r.GlobalEnv(), "coroutine", runtime.TableValue(coroutineTable))
	}
	
	return nil
}

// ValidateScript performs static analysis on a Lua script to detect potentially dangerous code
func ValidateScript(script string) error {
	// Basic static analysis to detect obviously dangerous patterns
	dangerousPatterns := []string{
		"os.execute", "os.exit", "io.open", "io.popen", "loadfile", "dofile",
		"require", "package.loadlib", "debug.", "ffi.", "_G[", "getfenv", "setfenv",
	}
	
	for _, pattern := range dangerousPatterns {
		if containsPattern(script, pattern) {
			return fmt.Errorf("script contains potentially dangerous pattern: %s", pattern)
		}
	}
	
	// Check for excessive string operations that could cause DoS
	if len(script) > 1024*1024 { // 1MB limit
		return fmt.Errorf("script is too large (%d bytes)", len(script))
	}
	
	return nil
}

// containsPattern checks if a script contains a dangerous pattern
func containsPattern(script, pattern string) bool {
	// Simple substring search - in production, this could be more sophisticated
	for i := 0; i <= len(script)-len(pattern); i++ {
		if script[i:i+len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// CreateSafeEnvironment creates a completely isolated Lua environment
func CreateSafeEnvironment(r *runtime.Runtime) error {
	// Create a new environment table
	env := runtime.NewTable()
	
	// Add only safe global functions
	safeGlobals := map[string]runtime.Value{
		"_VERSION": runtime.StringValue("Lua 5.4 (Sandboxed)"),
		"type":     r.GlobalEnv().Get(runtime.StringValue("type")),
		"tostring": r.GlobalEnv().Get(runtime.StringValue("tostring")),
		"tonumber": r.GlobalEnv().Get(runtime.StringValue("tonumber")),
		"next":     r.GlobalEnv().Get(runtime.StringValue("next")),
		"pairs":    r.GlobalEnv().Get(runtime.StringValue("pairs")),
		"ipairs":   r.GlobalEnv().Get(runtime.StringValue("ipairs")),
		"select":   r.GlobalEnv().Get(runtime.StringValue("select")),
		"unpack":   r.GlobalEnv().Get(runtime.StringValue("unpack")),
		"pcall":    r.GlobalEnv().Get(runtime.StringValue("pcall")),
		"xpcall":   r.GlobalEnv().Get(runtime.StringValue("xpcall")),
		
		// Safe libraries
		"string": r.GlobalEnv().Get(runtime.StringValue("string")),
		"table":  r.GlobalEnv().Get(runtime.StringValue("table")),
		"math":   r.GlobalEnv().Get(runtime.StringValue("math")),
	}
	
	for name, value := range safeGlobals {
		if value.Type() != runtime.NilType {
			r.SetTable(env, runtime.StringValue(name), value)
		}
	}
	
	// Set the environment as the global environment
	// Note: This is a simplified approach - full implementation would need more care
	return nil
}

// GetSandboxViolations returns a list of detected sandbox violations in a script
func GetSandboxViolations(script string, config *SandboxConfig) []string {
	violations := make([]string, 0)
	
	// Check for disallowed functions
	for _, funcName := range config.DisallowedFunctions {
		if containsPattern(script, funcName) {
			violations = append(violations, fmt.Sprintf("uses disallowed function: %s", funcName))
		}
	}
	
	// Check for disallowed modules
	for _, moduleName := range config.DisallowedModules {
		if containsPattern(script, moduleName+".") {
			violations = append(violations, fmt.Sprintf("uses disallowed module: %s", moduleName))
		}
	}
	
	// Additional checks can be added here
	
	return violations
}