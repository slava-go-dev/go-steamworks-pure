package go_steamworks_pure

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ebitengine/purego"
)

const (
	kISteamUserCallbacks       = 100
	GetTicketForWebApiCallback = kISteamUserCallbacks + 68
)

// CallbackRegistration - callback registration information
type CallbackRegistration struct {
	ID         string
	CallbackID int32
	Handler    func(ctx context.Context, msg *CallbackT) bool
}

type ManualDispatcher struct {
	pipe        HSteamPipe
	callbacks   map[int32][]*CallbackRegistration // callback ID -> list of handlers
	callbacksMu sync.RWMutex
	cancel      context.CancelFunc
	logger      logger
	running     bool
	nextRegID   int64
}

type TicketResult struct {
	Ticket []byte
	Error  error
}

var (
	SteamapiGethsteampipe                  func() HSteamPipe
	SteamapiManualdispatchInit             func()
	SteamapiManualdispatchRunframe         func(hSteamPipe HSteamPipe)
	SteamapiManualdispatchGetnextcallback  func(hSteamPipe HSteamPipe, pCallbackMsg *CallbackT) bool
	SteamapiManualdispatchFreelastcallback func(hSteamPipe HSteamPipe)
)

func registerManualDispatch(libc uintptr) {
	purego.RegisterLibFunc(&SteamapiGethsteampipe, libc, "SteamAPI_GetHSteamPipe")
	purego.RegisterLibFunc(&SteamapiManualdispatchInit, libc, "SteamAPI_ManualDispatch_Init")
	purego.RegisterLibFunc(&SteamapiManualdispatchRunframe, libc, "SteamAPI_ManualDispatch_RunFrame")
	purego.RegisterLibFunc(&SteamapiManualdispatchGetnextcallback, libc, "SteamAPI_ManualDispatch_GetNextCallback")
	purego.RegisterLibFunc(&SteamapiManualdispatchFreelastcallback, libc, "SteamAPI_ManualDispatch_FreeLastCallback")
}

func NewManualDispatcher(logger logger) *ManualDispatcher {
	return &ManualDispatcher{
		pipe:      SteamapiGethsteampipe(),
		callbacks: make(map[int32][]*CallbackRegistration),
		logger:    logger,
	}
}

// RegisterCallback registers a new callback handler
func (md *ManualDispatcher) RegisterCallback(id string, callbackID int32, handler func(ctx context.Context, msg *CallbackT) bool) error {
	if id == "" {
		return fmt.Errorf("callback ID cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	md.callbacksMu.Lock()
	defer md.callbacksMu.Unlock()

	// Check that ID is unique
	for _, handlers := range md.callbacks {
		for _, reg := range handlers {
			if reg.ID == id {
				return fmt.Errorf("callback with ID '%s' already registered", id)
			}
		}
	}

	reg := &CallbackRegistration{
		ID:         id,
		CallbackID: callbackID,
		Handler:    handler,
	}

	md.callbacks[callbackID] = append(md.callbacks[callbackID], reg)

	md.logger.Debugf("Registered callback handler '%s' for callback ID %d", id, callbackID)
	return nil
}

// UnregisterCallback unregisters a callback handler
func (md *ManualDispatcher) UnregisterCallback(id string) error {
	if id == "" {
		return fmt.Errorf("callback ID cannot be empty")
	}

	md.callbacksMu.Lock()
	defer md.callbacksMu.Unlock()

	found := false
	for callbackID, handlers := range md.callbacks {
		for i, reg := range handlers {
			if reg.ID == id {
				// Remove element from slice
				md.callbacks[callbackID] = append(handlers[:i], handlers[i+1:]...)

				// If no more handlers for this callback ID, remove entry
				if len(md.callbacks[callbackID]) == 0 {
					delete(md.callbacks, callbackID)
				}

				found = true
				md.logger.Debugf("Unregistered callback handler '%s' for callback ID %d", id, callbackID)
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("callback with ID '%s' not found", id)
	}

	return nil
}

// GetRegisteredCallbacks returns a list of registered callbacks
func (md *ManualDispatcher) GetRegisteredCallbacks() map[string]int32 {
	md.callbacksMu.RLock()
	defer md.callbacksMu.RUnlock()

	result := make(map[string]int32)
	for _, handlers := range md.callbacks {
		for _, reg := range handlers {
			result[reg.ID] = reg.CallbackID
		}
	}
	return result
}

// Start starts the callback processing loop
func (md *ManualDispatcher) Start(ctx context.Context) {
	if md.running {
		return
	}
	md.running = true

	ctx, cancel := context.WithCancel(ctx)
	md.cancel = cancel
	go md.runLoop(ctx)
}

func (md *ManualDispatcher) runLoop(ctx context.Context) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ticker := time.NewTicker(32 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			md.running = false
			return
		case <-ticker.C:
			md.processCallbacks(ctx)
		}
	}
}

func (md *ManualDispatcher) processCallbacks(ctx context.Context) {
	SteamapiManualdispatchRunframe(md.pipe)

	var msg CallbackT
	for SteamapiManualdispatchGetnextcallback(md.pipe, &msg) {
		md.handleCallback(ctx, &msg)
		SteamapiManualdispatchFreelastcallback(md.pipe)
	}

	SteamapiReleaseCurrentThreadMemory()
}

func (md *ManualDispatcher) handleCallback(ctx context.Context, msg *CallbackT) {
	md.callbacksMu.RLock()
	handlers, exists := md.callbacks[msg.Callback]
	md.callbacksMu.RUnlock()

	if !exists {
		md.logger.Debugf("No handlers registered for callback %d", msg.Callback)
		return
	}

	// Call all registered handlers in priority order
	handled := false
	for _, reg := range handlers {
		if reg.Handler(ctx, msg) {
			handled = true
			md.logger.Debugf("Callback %d handled by '%s'", msg.Callback, reg.ID)
			// Can stop after first successful handler or continue
			// Depending on needs - here we continue executing all handlers
		}
	}

	if !handled {
		md.logger.Debugf("Callback %d not handled by any registered handler", msg.Callback)
	}
}

func (md *ManualDispatcher) Close() {
	if md.cancel != nil {
		md.cancel()
	}

	md.callbacksMu.Lock()
	md.callbacks = make(map[int32][]*CallbackRegistration)
	md.callbacksMu.Unlock()

	md.running = false
}

// IsRunning checks if the dispatcher is running
func (md *ManualDispatcher) IsRunning() bool {
	return md.running
}
