package proxy

import (
	"errors"
	"sync"
)

// windowsAutomaticProxySessionState is the platform-independent ownership
// policy for one generation's WinHTTP automatic-proxy session. lifecycleMu
// protects handle ownership, while callMu serializes native calls made with
// that handle. Keeping the locks independent lets Close mark a generation as
// retired without waiting for (or deadlocking with) an in-flight PAC/WPAD
// evaluation.
//
// A caller must acquire before serializedCall and release only after the call
// has returned. The active lease count then guarantees that the native handle
// cannot be closed while either an executing or queued call still references
// it.
type windowsAutomaticProxySessionState struct {
	lifecycleMu sync.Mutex
	callMu      sync.Mutex
	handle      uintptr
	active      int
	closed      bool
}

func (state *windowsAutomaticProxySessionState) acquire(openHandle func() (uintptr, error)) (uintptr, error) {
	if state == nil || openHandle == nil {
		return 0, errors.New("Windows automatic proxy session is unavailable")
	}
	state.lifecycleMu.Lock()
	defer state.lifecycleMu.Unlock()
	if state.closed {
		return 0, errors.New("Windows automatic proxy session is closed")
	}
	if state.handle == 0 {
		handle, err := openHandle()
		if err != nil {
			return 0, err
		}
		if handle == 0 {
			return 0, errors.New("Windows automatic proxy session returned an invalid handle")
		}
		state.handle = handle
	}
	state.active++
	return state.handle, nil
}

func (state *windowsAutomaticProxySessionState) serializedCall(handle uintptr, operation func(uintptr) error) error {
	if state == nil || handle == 0 || operation == nil {
		return errors.New("Windows automatic proxy session call is unavailable")
	}
	state.callMu.Lock()
	defer state.callMu.Unlock()
	return operation(handle)
}

func (state *windowsAutomaticProxySessionState) release(closeHandle func(uintptr)) {
	if state == nil {
		return
	}
	var handleToClose uintptr
	state.lifecycleMu.Lock()
	if state.active > 0 {
		state.active--
	}
	if state.closed && state.active == 0 {
		handleToClose = state.handle
		state.handle = 0
	}
	state.lifecycleMu.Unlock()
	closeWindowsAutomaticProxyHandle(handleToClose, closeHandle)
}

func (state *windowsAutomaticProxySessionState) close(closeHandle func(uintptr)) {
	if state == nil {
		return
	}
	var handleToClose uintptr
	state.lifecycleMu.Lock()
	if !state.closed {
		state.closed = true
		if state.active == 0 {
			handleToClose = state.handle
			state.handle = 0
		}
	}
	state.lifecycleMu.Unlock()
	closeWindowsAutomaticProxyHandle(handleToClose, closeHandle)
}

func closeWindowsAutomaticProxyHandle(handle uintptr, closeHandle func(uintptr)) {
	if handle != 0 && closeHandle != nil {
		closeHandle(handle)
	}
}
