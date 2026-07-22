package libraryimport

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidBatch       = errors.New("invalid library import batch")
	ErrInvalidCandidate   = errors.New("invalid library import candidate")
	ErrBatchNotFound      = errors.New("library import batch not found")
	ErrCandidateNotFound  = errors.New("library import candidate not found")
	ErrInvalidTransition  = errors.New("invalid library import state transition")
	ErrImportAlreadyRuns  = errors.New("library import batch is already running")
	ErrSourceChanged      = errors.New("library import source changed after dry run")
	ErrDestinationExists  = errors.New("library import destination already exists")
	ErrManagedRootMissing = errors.New("managed import root is required for copy mode")
)

type Mode string

const (
	ModeReferenced Mode = "referenced"
	ModeCopy       Mode = "copy"
)

type HiddenPolicy string

const (
	HiddenExclude HiddenPolicy = "exclude"
	HiddenInclude HiddenPolicy = "include"
)

type SymlinkPolicy string

const (
	SymlinkSkip        SymlinkPolicy = "skip"
	SymlinkFollowFiles SymlinkPolicy = "follow_files"
)

type BatchStatus string

const (
	BatchScanning   BatchStatus = "scanning"
	BatchReady      BatchStatus = "ready"
	BatchRunning    BatchStatus = "running"
	BatchCancelling BatchStatus = "cancelling"
	BatchCancelled  BatchStatus = "cancelled"
	BatchCompleted  BatchStatus = "completed"
	BatchFailed     BatchStatus = "failed"
)

type CandidateStatus string

const (
	CandidateReady      CandidateStatus = "ready"
	CandidateDuplicate  CandidateStatus = "duplicate"
	CandidateSkipped    CandidateStatus = "skipped"
	CandidateImporting  CandidateStatus = "importing"
	CandidateCopied     CandidateStatus = "copied"
	CandidateRegistered CandidateStatus = "registered"
	CandidateSucceeded  CandidateStatus = "succeeded"
	CandidateFailed     CandidateStatus = "failed"
	CandidateCancelled  CandidateStatus = "cancelled"
)

type Category string

const (
	CategoryVideo Category = "video"
	CategoryAudio Category = "audio"
	CategoryBook  Category = "book"
	CategoryImage Category = "image"
	CategoryOther Category = "other"
)

type BatchCounts struct {
	Total      int
	Ready      int
	Duplicate  int
	Skipped    int
	Succeeded  int
	Failed     int
	TotalBytes int64
}

type Batch struct {
	ID              string
	RequestKey      string
	LibraryID       string
	Mode            Mode
	ManagedRoot     string
	HiddenPolicy    HiddenPolicy
	SymlinkPolicy   SymlinkPolicy
	Status          BatchStatus
	Counts          BatchCounts
	LastErrorCode   string
	LastError       string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	CancelRequested bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Candidate struct {
	ID                   string
	BatchID              string
	SourcePath           string
	RelativePath         string
	DisplayName          string
	Extension            string
	Category             Category
	MIMEType             string
	MediaProbed          bool
	WasSymlink           bool
	SizeBytes            int64
	ModifiedAt           time.Time
	HashAlgorithm        string
	ContentHash          string
	Status               CandidateStatus
	DuplicateFileID      string
	DuplicateCandidateID string
	ManagedPath          string
	FileID               string
	ErrorCode            string
	ErrorMessage         string
	Attempts             int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func NewBatch(item Batch) (Batch, error) {
	item.ID = strings.TrimSpace(item.ID)
	item.RequestKey = strings.TrimSpace(item.RequestKey)
	item.LibraryID = strings.TrimSpace(item.LibraryID)
	item.ManagedRoot = strings.TrimSpace(item.ManagedRoot)
	item.LastErrorCode = strings.TrimSpace(item.LastErrorCode)
	item.LastError = strings.TrimSpace(item.LastError)
	if item.ID == "" || item.RequestKey == "" || !validMode(item.Mode) ||
		!validHiddenPolicy(item.HiddenPolicy) || !validSymlinkPolicy(item.SymlinkPolicy) ||
		!validBatchStatus(item.Status) {
		return Batch{}, ErrInvalidBatch
	}
	if item.Mode == ModeCopy && item.ManagedRoot == "" {
		return Batch{}, ErrManagedRootMissing
	}
	if item.Mode == ModeReferenced {
		item.ManagedRoot = ""
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
	if item.Counts.Total < 0 || item.Counts.Ready < 0 || item.Counts.Duplicate < 0 ||
		item.Counts.Skipped < 0 || item.Counts.Succeeded < 0 || item.Counts.Failed < 0 ||
		item.Counts.TotalBytes < 0 {
		return Batch{}, ErrInvalidBatch
	}
	return item, nil
}

func NewCandidate(item Candidate) (Candidate, error) {
	item.ID = strings.TrimSpace(item.ID)
	item.BatchID = strings.TrimSpace(item.BatchID)
	item.SourcePath = strings.TrimSpace(item.SourcePath)
	item.RelativePath = strings.TrimSpace(item.RelativePath)
	item.DisplayName = strings.TrimSpace(item.DisplayName)
	item.Extension = strings.ToLower(strings.TrimSpace(item.Extension))
	item.MIMEType = strings.TrimSpace(item.MIMEType)
	item.HashAlgorithm = strings.ToLower(strings.TrimSpace(item.HashAlgorithm))
	item.ContentHash = strings.ToLower(strings.TrimSpace(item.ContentHash))
	item.DuplicateFileID = strings.TrimSpace(item.DuplicateFileID)
	item.DuplicateCandidateID = strings.TrimSpace(item.DuplicateCandidateID)
	item.ManagedPath = strings.TrimSpace(item.ManagedPath)
	item.FileID = strings.TrimSpace(item.FileID)
	item.ErrorCode = strings.TrimSpace(item.ErrorCode)
	item.ErrorMessage = strings.TrimSpace(item.ErrorMessage)
	if item.ID == "" || item.BatchID == "" || item.SourcePath == "" || item.DisplayName == "" ||
		!validCategory(item.Category) || !validCandidateStatus(item.Status) || item.SizeBytes < 0 || item.Attempts < 0 {
		return Candidate{}, ErrInvalidCandidate
	}
	if item.Status != CandidateSkipped && item.HashAlgorithm != "sha256" {
		return Candidate{}, ErrInvalidCandidate
	}
	if item.Status != CandidateSkipped && len(item.ContentHash) != 64 {
		return Candidate{}, ErrInvalidCandidate
	}
	if item.Status == CandidateDuplicate && item.DuplicateFileID == "" && item.DuplicateCandidateID == "" {
		return Candidate{}, ErrInvalidCandidate
	}
	if item.Status == CandidateSucceeded && item.FileID == "" {
		return Candidate{}, ErrInvalidCandidate
	}
	if item.Status == CandidateFailed && item.ErrorMessage == "" {
		return Candidate{}, ErrInvalidCandidate
	}
	if item.Status == CandidateSkipped && item.ErrorCode == "" {
		return Candidate{}, ErrInvalidCandidate
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
	if !item.ModifiedAt.IsZero() {
		item.ModifiedAt = item.ModifiedAt.UTC()
	}
	return item, nil
}

func (item Batch) CanTransition(next BatchStatus) bool {
	if item.Status == next {
		return true
	}
	switch item.Status {
	case BatchScanning:
		return next == BatchReady || next == BatchFailed || next == BatchCancelled
	case BatchReady:
		return next == BatchRunning || next == BatchCancelled
	case BatchRunning:
		return next == BatchCancelling || next == BatchCancelled || next == BatchCompleted || next == BatchFailed
	case BatchCancelling:
		return next == BatchRunning || next == BatchCancelled || next == BatchFailed
	case BatchCancelled, BatchFailed:
		return next == BatchReady || next == BatchRunning
	case BatchCompleted:
		return false
	default:
		return false
	}
}

func (item Candidate) CanTransition(next CandidateStatus) bool {
	if item.Status == next {
		return true
	}
	switch item.Status {
	case CandidateReady:
		return next == CandidateImporting || next == CandidateCancelled || next == CandidateFailed
	case CandidateImporting:
		return next == CandidateCopied || next == CandidateRegistered || next == CandidateFailed || next == CandidateCancelled
	case CandidateCopied:
		return next == CandidateRegistered || next == CandidateFailed || next == CandidateCancelled
	case CandidateRegistered:
		return next == CandidateSucceeded || next == CandidateFailed
	case CandidateFailed, CandidateCancelled:
		return next == CandidateReady || next == CandidateImporting
	case CandidateDuplicate, CandidateSkipped, CandidateSucceeded:
		return false
	default:
		return false
	}
}

func CountsFor(candidates []Candidate) BatchCounts {
	counts := BatchCounts{Total: len(candidates)}
	for _, item := range candidates {
		if item.SizeBytes > 0 {
			counts.TotalBytes += item.SizeBytes
		}
		switch item.Status {
		case CandidateReady, CandidateImporting, CandidateCopied, CandidateRegistered, CandidateCancelled:
			counts.Ready++
		case CandidateDuplicate:
			counts.Duplicate++
		case CandidateSkipped:
			counts.Skipped++
		case CandidateSucceeded:
			counts.Succeeded++
		case CandidateFailed:
			counts.Failed++
		}
	}
	return counts
}

func validMode(value Mode) bool { return value == ModeReferenced || value == ModeCopy }
func validHiddenPolicy(value HiddenPolicy) bool {
	return value == HiddenExclude || value == HiddenInclude
}
func validSymlinkPolicy(value SymlinkPolicy) bool {
	return value == SymlinkSkip || value == SymlinkFollowFiles
}

func validBatchStatus(value BatchStatus) bool {
	switch value {
	case BatchScanning, BatchReady, BatchRunning, BatchCancelling, BatchCancelled, BatchCompleted, BatchFailed:
		return true
	default:
		return false
	}
}

func validCandidateStatus(value CandidateStatus) bool {
	switch value {
	case CandidateReady, CandidateDuplicate, CandidateSkipped, CandidateImporting, CandidateCopied,
		CandidateRegistered, CandidateSucceeded, CandidateFailed, CandidateCancelled:
		return true
	default:
		return false
	}
}

func validCategory(value Category) bool {
	switch value {
	case CategoryVideo, CategoryAudio, CategoryBook, CategoryImage, CategoryOther:
		return true
	default:
		return false
	}
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	result := value.UTC()
	return &result
}
