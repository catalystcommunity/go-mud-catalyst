package scripting

import (
	"fmt"
	"os"
	"strings"

	"github.com/arnodel/golua/runtime"
)

// SimpleValidateScript performs basic validation on Lua scripts
func SimpleValidateScript(script string) error {
	// Check for dangerous patterns
	dangerousPatterns := []string{
		"os.", "io.", "require", "dofile", "loadfile", 
		"package.", "debug.", "load(", "loadstring",
		"getfenv", "setfenv", "rawget", "rawset",
	}
	
	scriptLower := strings.ToLower(script)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(scriptLower, pattern) {
			return fmt.Errorf("script contains potentially dangerous pattern: %s", pattern)
		}
	}
	
	// Check script size
	if len(script) > 1024*1024 { // 1MB limit
		return fmt.Errorf("script too large: %d bytes", len(script))
	}
	
	return nil
}

// SetupSimpleSecureRuntime creates a basic secure runtime
func SetupSimpleSecureRuntime() (*runtime.Runtime, error) {
	r := runtime.New(os.Stdout)
	
	// Remove dangerous globals by setting them to nil
	dangerousGlobals := []string{
		"os", "io", "package", "debug", "require", "dofile", "loadfile",
		"load", "loadstring", "module", "getfenv", "setfenv",
	}
	
	for _, name := range dangerousGlobals {
		r.SetEnv(r.GlobalEnv(), name, runtime.NilValue)
	}
	
	return r, nil
}