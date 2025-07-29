package scripting

import (
	"os"

	"github.com/arnodel/golua/runtime"
	"github.com/catalystcommunity/muddycore/pkg/events"
	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// APIContext holds references to muddycore systems that should be exposed to Lua
type APIContext struct {
	Logger           *logging.Logger
	EventManager     *events.DefaultEventManager
	LuaEventManager  *LuaEventManager
	
	// Server/Client context - these can be nil depending on usage
	ServerID string
	ClientID string
	VMID     string
	
	// Custom data that can be accessed from Lua
	UserData map[string]interface{}
}

// RegisterSimpleLuaAPI registers a simplified version of the muddycore API
func RegisterSimpleLuaAPI(r *runtime.Runtime, ctx *APIContext) error {
	// Create a simple logging function
	logFunc := func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		msg, err := c.StringArg(0)
		if err != nil {
			return nil, err
		}
		if ctx.Logger != nil {
			ctx.Logger.Info("Lua script: " + msg)
		}
		return c.Next(), nil
	}
	
	// Register the log function globally
	r.SetEnvGoFunc(r.GlobalEnv(), "log", logFunc, 1, false)
	
	// Create a simple time function
	timeFunc := func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		// Return current unix timestamp
		return c.PushingNext1(t.Runtime, runtime.IntValue(1704067200)), nil
	}
	
	r.SetEnvGoFunc(r.GlobalEnv(), "current_time", timeFunc, 0, false)
	
	// Create a simple event emit function
	emitFunc := func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		eventType, err := c.StringArg(0)
		if err != nil {
			return nil, err
		}
		message, err := c.StringArg(1)
		if err != nil {
			return nil, err
		}
		
		if ctx.EventManager != nil {
			event := events.NewEvent(events.EventType(eventType), "lua_script", map[string]interface{}{"message": message})
			ctx.EventManager.TriggerEvent(event)
		}
		return c.Next(), nil
	}
	
	r.SetEnvGoFunc(r.GlobalEnv(), "emit_event", emitFunc, 2, false)
	
	return nil
}

// SetupBasicSafeRuntime creates a basic runtime with minimal safe functions
func SetupBasicSafeRuntime() (*runtime.Runtime, error) {
	// Create new runtime with basic settings
	r := runtime.New(os.Stdout)
	
	// Remove dangerous global functions
	dangerousFunctions := []string{
		"dofile", "loadfile", "load", "require", "module",
		"os", "io", "package", "debug",
	}
	
	for _, funcName := range dangerousFunctions {
		r.SetEnv(r.GlobalEnv(), funcName, runtime.NilValue)
	}
	
	// Add basic safe print function
	printFunc := func(t *runtime.Thread, c *runtime.GoCont) (runtime.Cont, error) {
		// Collect all arguments and ignore them (safe print)
		for i := 0; i < c.NArgs(); i++ {
			_ = c.Arg(i) // Just consume the arguments
		}
		return c.Next(), nil
	}
	
	r.SetEnvGoFunc(r.GlobalEnv(), "print", printFunc, 0, true)
	
	return r, nil
}