//go:build windows

package service

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"

	"xiadown/internal/domain/library"
)

func TestClassifyListenLocalMetadataWindowsReplaceError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "sharing violation", err: windows.ERROR_SHARING_VIOLATION, want: library.ErrListenLocalFileBusy},
		{name: "mapped playback file", err: windows.ERROR_USER_MAPPED_FILE, want: library.ErrListenLocalFileBusy},
		{name: "access denied", err: windows.ERROR_ACCESS_DENIED, want: library.ErrListenLocalFilePermission},
		{name: "file vanished", err: windows.ERROR_FILE_NOT_FOUND, want: library.ErrListenLocalFileChanged},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyListenLocalMetadataWindowsReplaceError(test.err); !errors.Is(got, test.want) {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
