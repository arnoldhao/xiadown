//go:build darwin && cgo && !ios

package libraryrootsync

/*
#cgo LDFLAGS: -framework CoreServices
#include <stdlib.h>
#include "watcher_darwin.h"
*/
import "C"

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime/cgo"
	"unsafe"
)

const (
	fseventMustScanSubDirs = 0x00000001
	fseventUserDropped     = 0x00000002
	fseventKernelDropped   = 0x00000004
	fseventEventIDsWrapped = 0x00000008
	fseventRootChanged     = 0x00000020
	fseventItemIsDir       = 0x00020000
)

type darwinNativeWatcher struct{}

type darwinWatchBridge struct {
	ctx  context.Context
	emit func(watchEvent)
}

func platformNativeWatcher() nativeWatcher {
	return darwinNativeWatcher{}
}

func (darwinNativeWatcher) Available() bool      { return true }
func (darwinNativeWatcher) SupportsReplay() bool { return true }

func (darwinNativeWatcher) Watch(
	ctx context.Context,
	rootPath string,
	since uint64,
	emit func(watchEvent),
) error {
	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	cPath := C.CString(rootPath)
	defer C.free(unsafe.Pointer(cPath))
	if since == 0 {
		since = uint64(C.xiadown_fsevents_current_event_id())
		if since > 0 {
			emit(watchEvent{cursor: since, checkpoint: true})
		}
	}
	handle := cgo.NewHandle(&darwinWatchBridge{ctx: ctx, emit: emit})
	defer handle.Delete()
	watcher := C.xiadown_fsevents_start(
		cPath,
		C.uint64_t(since),
		C.uintptr_t(handle),
	)
	if watcher == nil {
		return fmt.Errorf("%w: FSEventStreamCreate failed", errNativeWatcherUnavailable)
	}
	<-ctx.Done()
	C.xiadown_fsevents_stop(watcher)
	return ctx.Err()
}

//export xiadownRootSyncFSEvent
func xiadownRootSyncFSEvent(
	goHandle C.uintptr_t,
	path *C.char,
	cursor C.uint64_t,
	flags C.uint32_t,
) {
	handle := cgo.Handle(goHandle)
	bridge, ok := handle.Value().(*darwinWatchBridge)
	if !ok || bridge == nil || bridge.emit == nil {
		return
	}
	select {
	case <-bridge.ctx.Done():
		return
	default:
	}
	rawFlags := uint32(flags)
	bridge.emit(watchEvent{
		path:      C.GoString(path),
		cursor:    uint64(cursor),
		directory: rawFlags&fseventItemIsDir != 0,
		overflow: rawFlags&(fseventMustScanSubDirs|
			fseventUserDropped|
			fseventKernelDropped|
			fseventEventIDsWrapped|
			fseventRootChanged) != 0,
	})
}
