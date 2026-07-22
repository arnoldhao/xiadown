//go:build windows

package librarybackup

// Windows does not expose a portable directory FlushFileBuffers operation
// through os.File. Files themselves are flushed before publication and every
// namespace transition uses MoveFileEx(MOVEFILE_WRITE_THROUGH).
func syncDirectory(_ string) error {
	return nil
}
