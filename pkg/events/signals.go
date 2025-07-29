package events

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/catalystcommunity/muddycore/pkg/logging"
)

// SignalHandler handles OS signals and converts them to events
type SignalHandler struct {
	eventManager EventManager
	logger       *logging.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	signals      chan os.Signal
	running      bool
	mutex        sync.Mutex
}

// NewSignalHandler creates a new signal handler
func NewSignalHandler(eventManager EventManager) *SignalHandler {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &SignalHandler{
		eventManager: eventManager,
		logger:       logging.GetDefaultLogger().WithComponent("signals"),
		ctx:          ctx,
		cancel:       cancel,
		signals:      make(chan os.Signal, 10),
		running:      false,
	}
}

// Start begins listening for OS signals
func (sh *SignalHandler) Start() {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()
	
	if sh.running {
		return
	}
	
	// Register signal handlers for common signals
	signal.Notify(sh.signals,
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // Termination signal
		syscall.SIGUSR1, // User-defined signal 1
		syscall.SIGUSR2, // User-defined signal 2
		syscall.SIGHUP,  // Hangup signal
		syscall.SIGQUIT, // Quit signal
	)
	
	sh.running = true
	sh.wg.Add(1)
	go sh.handleSignals()
	
	sh.logger.Info("Signal handler started")
}

// Stop stops listening for OS signals
func (sh *SignalHandler) Stop() {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()
	
	if !sh.running {
		return
	}
	
	sh.running = false
	signal.Stop(sh.signals)
	sh.cancel()
	sh.wg.Wait()
	
	sh.logger.Info("Signal handler stopped")
}

// handleSignals processes incoming OS signals
func (sh *SignalHandler) handleSignals() {
	defer sh.wg.Done()
	
	for {
		select {
		case sig := <-sh.signals:
			sh.processSignal(sig)
		case <-sh.ctx.Done():
			return
		}
	}
}

// processSignal converts an OS signal to an event
func (sh *SignalHandler) processSignal(sig os.Signal) {
	signalName := sig.String()
	processID := os.Getpid()
	
	sh.logger.Info("Received OS signal", "signal", signalName, "pid", processID)
	
	// Create context for signal processing
	ctx := map[string]interface{}{
		"timestamp": sh.getCurrentTimestamp(),
		"handler":   "muddycore_signal_handler",
	}
	
	// Determine the reason based on signal type
	reason := sh.getSignalReason(sig)
	
	// Create system event
	event := NewSystemEvent(EventTypeSystemSignal, signalName, processID, reason, ctx)
	
	// Add signal-specific data
	event.SetData("signal_number", int(sig.(syscall.Signal)))
	event.SetData("signal_description", sh.getSignalDescription(sig))
	
	// Trigger event synchronously for signals (important for shutdown handling)
	result := sh.eventManager.TriggerEventSync(event)
	
	// Handle special signals
	switch sig {
	case syscall.SIGINT, syscall.SIGTERM:
		if result != EventResultCancel {
			// If not cancelled by handlers, trigger shutdown
			sh.triggerShutdownEvent(signalName, false)
		}
	case syscall.SIGQUIT:
		// Force quit - trigger immediate shutdown
		sh.triggerShutdownEvent(signalName, true)
	case syscall.SIGHUP:
		// Hangup - could be used for reload/restart
		sh.triggerReloadEvent()
	}
}

// triggerShutdownEvent creates and triggers a shutdown event
func (sh *SignalHandler) triggerShutdownEvent(signal string, immediate bool) {
	ctx := map[string]interface{}{
		"trigger":   "signal",
		"signal":    signal,
		"immediate": immediate,
	}
	
	reason := "Signal-triggered shutdown"
	if immediate {
		reason = "Signal-triggered immediate shutdown"
	}
	
	event := NewSystemEvent(EventTypeSystemShutdown, signal, os.Getpid(), reason, ctx)
	event.SetData("graceful", !immediate)
	
	// Trigger shutdown event
	sh.eventManager.TriggerEventSync(event)
}

// triggerReloadEvent creates and triggers a reload event
func (sh *SignalHandler) triggerReloadEvent() {
	ctx := map[string]interface{}{
		"trigger": "sighup",
		"action":  "reload",
	}
	
	event := NewSystemEvent(EventType("system.reload"), "SIGHUP", os.Getpid(), "Configuration reload requested", ctx)
	sh.eventManager.TriggerEventAsync(event)
}

// getSignalReason returns a human-readable reason for the signal
func (sh *SignalHandler) getSignalReason(sig os.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "Interrupt signal (Ctrl+C)"
	case syscall.SIGTERM:
		return "Termination signal"
	case syscall.SIGUSR1:
		return "User-defined signal 1"
	case syscall.SIGUSR2:
		return "User-defined signal 2"
	case syscall.SIGHUP:
		return "Hangup signal"
	case syscall.SIGQUIT:
		return "Quit signal"
	default:
		return "Unknown signal"
	}
}

// getSignalDescription returns a detailed description of the signal
func (sh *SignalHandler) getSignalDescription(sig os.Signal) string {
	switch sig {
	case syscall.SIGINT:
		return "Interrupt from keyboard (usually Ctrl+C)"
	case syscall.SIGTERM:
		return "Termination request from process manager"
	case syscall.SIGUSR1:
		return "User-defined signal 1 (application specific)"
	case syscall.SIGUSR2:
		return "User-defined signal 2 (application specific)"
	case syscall.SIGHUP:
		return "Hangup detected on controlling terminal or death of controlling process"
	case syscall.SIGQUIT:
		return "Quit from keyboard (usually Ctrl+\\)"
	default:
		return "Operating system signal"
	}
}

// getCurrentTimestamp returns the current timestamp as a string
func (sh *SignalHandler) getCurrentTimestamp() string {
	return sh.eventManager.CreateEvent(EventTypeSystemSignal, nil, nil).Timestamp().Format("2006-01-02T15:04:05.000Z07:00")
}

// RegisterShutdownHook is a convenience method to register a shutdown hook
func (sh *SignalHandler) RegisterShutdownHook(hook EventHook, priority int) string {
	return sh.eventManager.RegisterHook(EventTypeSystemShutdown, hook, priority)
}

// RegisterSignalHook is a convenience method to register a signal hook
func (sh *SignalHandler) RegisterSignalHook(hook EventHook, priority int) string {
	return sh.eventManager.RegisterHook(EventTypeSystemSignal, hook, priority)
}

// RegisterReloadHook is a convenience method to register a reload hook
func (sh *SignalHandler) RegisterReloadHook(hook EventHook, priority int) string {
	return sh.eventManager.RegisterHook(EventType("system.reload"), hook, priority)
}

// GracefulShutdown performs a graceful shutdown sequence
type GracefulShutdown struct {
	eventManager  EventManager
	signalHandler *SignalHandler
	logger        *logging.Logger
	shutdownHooks []func() error
	mutex         sync.Mutex
	shutdownChan  chan struct{}
	once          sync.Once
}

// NewGracefulShutdown creates a new graceful shutdown manager
func NewGracefulShutdown(eventManager EventManager) *GracefulShutdown {
	gs := &GracefulShutdown{
		eventManager:  eventManager,
		signalHandler: NewSignalHandler(eventManager),
		logger:        logging.GetDefaultLogger().WithComponent("shutdown"),
		shutdownHooks: make([]func() error, 0),
		shutdownChan:  make(chan struct{}),
	}
	
	// Register shutdown event handler
	eventManager.RegisterHook(EventTypeSystemShutdown, gs.handleShutdownEvent, -1000) // High priority
	
	return gs
}

// Start starts the graceful shutdown manager
func (gs *GracefulShutdown) Start() {
	gs.signalHandler.Start()
}

// Stop stops the graceful shutdown manager
func (gs *GracefulShutdown) Stop() {
	gs.signalHandler.Stop()
}

// AddShutdownHook adds a function to be called during shutdown
func (gs *GracefulShutdown) AddShutdownHook(hook func() error) {
	gs.mutex.Lock()
	defer gs.mutex.Unlock()
	gs.shutdownHooks = append(gs.shutdownHooks, hook)
}

// WaitForShutdown blocks until shutdown is triggered
func (gs *GracefulShutdown) WaitForShutdown() {
	<-gs.shutdownChan
}

// TriggerShutdown manually triggers a shutdown
func (gs *GracefulShutdown) TriggerShutdown(reason string) {
	ctx := map[string]interface{}{
		"trigger": "manual",
		"reason":  reason,
	}
	
	event := NewSystemEvent(EventTypeSystemShutdown, "MANUAL", os.Getpid(), reason, ctx)
	gs.eventManager.TriggerEventSync(event)
}

// handleShutdownEvent handles shutdown events
func (gs *GracefulShutdown) handleShutdownEvent(event Event) EventResult {
	// Ensure shutdown only happens once
	gs.once.Do(func() {
		gs.logger.Info("Graceful shutdown initiated", "reason", event.Data()["reason"])
		
		// Execute shutdown hooks in reverse order (LIFO)
		gs.mutex.Lock()
		hooks := make([]func() error, len(gs.shutdownHooks))
		copy(hooks, gs.shutdownHooks)
		gs.mutex.Unlock()
		
		for i := len(hooks) - 1; i >= 0; i-- {
			if err := hooks[i](); err != nil {
				gs.logger.Error("Shutdown hook failed", "index", i, "error", err)
			}
		}
		
		gs.logger.Info("Graceful shutdown completed")
		close(gs.shutdownChan)
	})
	
	return EventResultContinue
}

// Convenience functions for creating signal-related events

// CreateSignalEvent creates a signal event
func CreateSignalEvent(signal string, processID int, reason string) *SystemEvent {
	ctx := map[string]interface{}{
		"handler": "signal_handler",
	}
	return NewSystemEvent(EventTypeSystemSignal, signal, processID, reason, ctx)
}

// CreateShutdownEvent creates a shutdown event
func CreateShutdownEvent(trigger, reason string, graceful bool) *SystemEvent {
	ctx := map[string]interface{}{
		"trigger":  trigger,
		"graceful": graceful,
	}
	return NewSystemEvent(EventTypeSystemShutdown, trigger, os.Getpid(), reason, ctx)
}