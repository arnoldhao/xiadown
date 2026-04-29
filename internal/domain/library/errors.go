package library

import "errors"

var (
	ErrLibraryNotFound          = errors.New("library not found")
	ErrFileNotFound             = errors.New("library file not found")
	ErrOperationNotFound        = errors.New("library operation not found")
	ErrHistoryRecordNotFound    = errors.New("library history record not found")
	ErrWorkspaceStateNotFound   = errors.New("library workspace state not found")
	ErrFileEventNotFound        = errors.New("library file event not found")
	ErrSubtitleDocumentNotFound = errors.New("library subtitle document not found")
	ErrInvalidLibrary           = errors.New("invalid library")
	ErrInvalidLibraryFile       = errors.New("invalid library file")
	ErrInvalidLibraryOperation  = errors.New("invalid library operation")
	ErrInvalidOperationChunk    = errors.New("invalid library operation chunk")
	ErrInvalidHistoryRecord     = errors.New("invalid library history record")
	ErrInvalidWorkspaceState    = errors.New("invalid library workspace state")
	ErrInvalidFileEvent         = errors.New("invalid library file event")
	ErrInvalidSubtitleDocument  = errors.New("invalid library subtitle document")
	ErrInvalidOperationOutput   = errors.New("invalid library operation output")
	ErrPresetNotFound           = errors.New("transcode preset not found")
	ErrInvalidPreset            = errors.New("invalid transcode preset")
)
