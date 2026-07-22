//go:build darwin && cgo

package service

/*
#include <copyfile.h>
#include <errno.h>
#include <stdlib.h>

static int xiadown_copy_metadata(const char *source, const char *destination) {
	if (copyfile(source, destination, NULL, COPYFILE_METADATA) == 0) {
		return 0;
	}
	return errno;
}
*/
import "C"

import (
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func copyListenLocalMetadataExtendedAttributes(source string, destination string) error {
	sourceCString := C.CString(source)
	destinationCString := C.CString(destination)
	defer C.free(unsafe.Pointer(sourceCString))
	defer C.free(unsafe.Pointer(destinationCString))
	if errno := C.xiadown_copy_metadata(sourceCString, destinationCString); errno != 0 {
		return syscall.Errno(errno)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	if sourceStat, ok := sourceInfo.Sys().(*syscall.Stat_t); ok && sourceStat != nil {
		birthTime := unix.Timespec{Sec: sourceStat.Birthtimespec.Sec, Nsec: sourceStat.Birthtimespec.Nsec}
		birthTimeBytes := unsafe.Slice((*byte)(unsafe.Pointer(&birthTime)), int(unsafe.Sizeof(birthTime)))
		attributes := unix.Attrlist{
			Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
			Commonattr:  unix.ATTR_CMN_CRTIME,
		}
		if err := unix.Setattrlist(destination, &attributes, birthTimeBytes, 0); err != nil {
			return err
		}
	}
	// COPYFILE_STAT retains POSIX state and file flags; creation time is restored
	// explicitly above. An embedded-tag edit should still advance modification
	// time for file watchers.
	info, err := os.Stat(destination)
	if err != nil {
		return err
	}
	atime := info.ModTime()
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat != nil {
		atime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
	}
	if err := os.Chtimes(destination, atime, time.Now()); err != nil {
		return err
	}
	return nil
}
