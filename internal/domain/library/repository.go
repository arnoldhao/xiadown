package library

import "context"

type LibraryRepository interface {
	List(ctx context.Context) ([]Library, error)
	Get(ctx context.Context, id string) (Library, error)
	Save(ctx context.Context, item Library) error
	Delete(ctx context.Context, id string) error
}

type ModuleConfigRepository interface {
	Get(ctx context.Context) (ModuleConfig, error)
	Save(ctx context.Context, config ModuleConfig) error
}

type FileRepository interface {
	List(ctx context.Context) ([]LibraryFile, error)
	ListByLibraryID(ctx context.Context, libraryID string) ([]LibraryFile, error)
	Get(ctx context.Context, id string) (LibraryFile, error)
	Save(ctx context.Context, item LibraryFile) error
	Delete(ctx context.Context, id string) error
}

type DreamFMLocalTrackRepository interface {
	List(ctx context.Context, options DreamFMLocalTrackListOptions) ([]DreamFMLocalTrack, error)
	Get(ctx context.Context, fileID string) (DreamFMLocalTrack, error)
	Save(ctx context.Context, item DreamFMLocalTrack) error
	Delete(ctx context.Context, fileID string) error
	DeleteUnavailable(ctx context.Context) (int, error)
}

type OperationRepository interface {
	List(ctx context.Context) ([]LibraryOperation, error)
	ListByLibraryID(ctx context.Context, libraryID string) ([]LibraryOperation, error)
	Get(ctx context.Context, id string) (LibraryOperation, error)
	Save(ctx context.Context, item LibraryOperation) error
	Delete(ctx context.Context, id string) error
}

type OperationChunkRepository interface {
	ListByOperationID(ctx context.Context, operationID string) ([]OperationChunk, error)
	Save(ctx context.Context, item OperationChunk) error
	DeleteByOperationID(ctx context.Context, operationID string) error
}

type HistoryRepository interface {
	ListByLibraryID(ctx context.Context, libraryID string) ([]HistoryRecord, error)
	Get(ctx context.Context, id string) (HistoryRecord, error)
	Save(ctx context.Context, item HistoryRecord) error
	Delete(ctx context.Context, id string) error
	DeleteByOperationID(ctx context.Context, operationID string) error
}

type WorkspaceStateRepository interface {
	ListByLibraryID(ctx context.Context, libraryID string) ([]WorkspaceStateRecord, error)
	GetHeadByLibraryID(ctx context.Context, libraryID string) (WorkspaceStateRecord, error)
	Save(ctx context.Context, item WorkspaceStateRecord) error
}

type FileEventRepository interface {
	ListByLibraryID(ctx context.Context, libraryID string) ([]FileEventRecord, error)
	Save(ctx context.Context, item FileEventRecord) error
}

type SubtitleDocumentRepository interface {
	Get(ctx context.Context, id string) (SubtitleDocument, error)
	GetByFileID(ctx context.Context, fileID string) (SubtitleDocument, error)
	Save(ctx context.Context, document SubtitleDocument) error
	DeleteByFileID(ctx context.Context, fileID string) error
}

type TranscodePresetRepository interface {
	List(ctx context.Context) ([]TranscodePreset, error)
	Get(ctx context.Context, id string) (TranscodePreset, error)
	Save(ctx context.Context, preset TranscodePreset) error
	Delete(ctx context.Context, id string) error
}
