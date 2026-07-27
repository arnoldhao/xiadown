package libraryrootsync

import (
	"errors"
	"path"
	"strings"
	"time"
)

var (
	ErrInvalidState  = errors.New("invalid Library storage root sync state")
	ErrInvalidEntry  = errors.New("invalid Library storage root sync entry")
	ErrStateNotFound = errors.New("Library storage root sync state not found")
	ErrEntryNotFound = errors.New("Library storage root sync entry not found")
)

type Status string

const (
	StatusIdle        Status = "idle"
	StatusQueued      Status = "queued"
	StatusScanning    Status = "scanning"
	StatusWatching    Status = "watching"
	StatusCancelling  Status = "cancelling"
	StatusCancelled   Status = "cancelled"
	StatusInterrupted Status = "interrupted"
	StatusFailed      Status = "failed"
)

type EntryStatus string

const (
	EntryActive    EntryStatus = "active"
	EntryDuplicate EntryStatus = "duplicate"
	EntryMissing   EntryStatus = "missing"
	EntryFailed    EntryStatus = "failed"
)

type State struct {
	RootID           string
	Status           Status
	Generation       int64
	FullScan         bool
	DiscoveredCount  int
	ProcessedCount   int
	UnchangedCount   int
	DuplicateCount   int
	MissingCount     int
	FailedCount      int
	ProcessedBytes   int64
	CancelRequested  bool
	WatcherCursor    uint64
	LastErrorCode    string
	LastError        string
	StartedAt        *time.Time
	FinishedAt       *time.Time
	LastReconciledAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Entry struct {
	RootID             string
	RelativePath       string
	SizeBytes          int64
	ModifiedUnixNano   int64
	ContentHash        string
	FileID             string
	Status             EntryStatus
	LastSeenGeneration int64
	LastError          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewState(item State) (State, error) {
	item.RootID = strings.TrimSpace(item.RootID)
	item.LastErrorCode = strings.TrimSpace(item.LastErrorCode)
	item.LastError = strings.TrimSpace(item.LastError)
	if item.RootID == "" || !validStatus(item.Status) || item.Generation < 0 ||
		item.DiscoveredCount < 0 || item.ProcessedCount < 0 ||
		item.UnchangedCount < 0 || item.DuplicateCount < 0 ||
		item.MissingCount < 0 || item.FailedCount < 0 ||
		item.ProcessedBytes < 0 || item.ProcessedCount > item.DiscoveredCount {
		return State{}, ErrInvalidState
	}
	if item.Status == StatusFailed && item.LastError == "" {
		return State{}, ErrInvalidState
	}
	if item.Status != StatusFailed {
		item.LastErrorCode = ""
		item.LastError = ""
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	} else {
		item.CreatedAt = item.CreatedAt.UTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	} else {
		item.UpdatedAt = item.UpdatedAt.UTC()
	}
	item.StartedAt = utcTimePointer(item.StartedAt)
	item.FinishedAt = utcTimePointer(item.FinishedAt)
	item.LastReconciledAt = utcTimePointer(item.LastReconciledAt)
	return item, nil
}

func NewEntry(item Entry) (Entry, error) {
	item.RootID = strings.TrimSpace(item.RootID)
	item.RelativePath = normalizeRelativePath(item.RelativePath)
	item.ContentHash = strings.ToLower(strings.TrimSpace(item.ContentHash))
	item.FileID = strings.TrimSpace(item.FileID)
	item.LastError = strings.TrimSpace(item.LastError)
	if item.RootID == "" || item.RelativePath == "" || item.RelativePath == "." ||
		item.SizeBytes < 0 || item.ModifiedUnixNano < 0 ||
		item.LastSeenGeneration < 0 || !validEntryStatus(item.Status) {
		return Entry{}, ErrInvalidEntry
	}
	if item.ContentHash != "" && len(item.ContentHash) != 64 {
		return Entry{}, ErrInvalidEntry
	}
	if item.Status == EntryActive && item.FileID == "" {
		return Entry{}, ErrInvalidEntry
	}
	if item.Status == EntryFailed && item.LastError == "" {
		return Entry{}, ErrInvalidEntry
	}
	if item.Status != EntryFailed {
		item.LastError = ""
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now().UTC()
	} else {
		item.CreatedAt = item.CreatedAt.UTC()
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	} else {
		item.UpdatedAt = item.UpdatedAt.UTC()
	}
	return item, nil
}

func (item Entry) FingerprintMatches(sizeBytes, modifiedUnixNano int64) bool {
	if item.SizeBytes != sizeBytes ||
		item.ModifiedUnixNano != modifiedUnixNano {
		return false
	}
	switch item.Status {
	case EntryActive:
		return strings.TrimSpace(item.FileID) != ""
	case EntryDuplicate:
		return strings.TrimSpace(item.ContentHash) != ""
	default:
		return false
	}
}

func validStatus(value Status) bool {
	switch value {
	case StatusIdle, StatusQueued, StatusScanning, StatusWatching,
		StatusCancelling, StatusCancelled, StatusInterrupted, StatusFailed:
		return true
	default:
		return false
	}
}

func validEntryStatus(value EntryStatus) bool {
	switch value {
	case EntryActive, EntryDuplicate, EntryMissing, EntryFailed:
		return true
	default:
		return false
	}
}

func normalizeRelativePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	if value == "" {
		return ""
	}
	value = path.Clean(value)
	if value == "." || value == ".." || path.IsAbs(value) ||
		strings.HasPrefix(value, "../") {
		return ""
	}
	return value
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}
