//go:build windows

package librarybackup

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// durableRename asks Windows to flush the move before returning. SQLite and
// restore artifacts always stay on one volume, so MoveFileEx supplies the
// atomic namespace transition required by the restore journal.
func durableRename(source, destination string, replace bool) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return fmt.Errorf("encode rename source: %w", err)
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return fmt.Errorf("encode rename destination: %w", err)
	}
	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if replace {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	if err := windows.MoveFileEx(from, to, flags); err != nil {
		return fmt.Errorf("durable Windows rename: %w", err)
	}
	return nil
}
