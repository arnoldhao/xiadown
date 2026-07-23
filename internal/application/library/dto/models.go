package dto

const LibrarySchemaVersion = "current"
const WorkspaceProjectSchemaVersion = "current"

type LibraryDTO struct {
	Version   string               `json:"version"`
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	CreatedAt string               `json:"createdAt"`
	UpdatedAt string               `json:"updatedAt"`
	CreatedBy LibraryCreateMetaDTO `json:"createdBy"`
	Files     []LibraryFileDTO     `json:"files"`
	Records   LibraryRecordsDTO    `json:"records"`
}

type LibraryCreateMetaDTO struct {
	Source             string `json:"source"`
	TriggerOperationID string `json:"triggerOperationId,omitempty"`
	ImportBatchID      string `json:"importBatchId,omitempty"`
	Actor              string `json:"actor,omitempty"`
}

type LibraryFileDTO struct {
	ID                string                `json:"id"`
	LibraryID         string                `json:"libraryId"`
	Kind              string                `json:"kind"`
	Name              string                `json:"name"`
	DisplayName       string                `json:"displayName,omitempty"`
	FileName          string                `json:"fileName,omitempty"`
	DisplayLabel      string                `json:"displayLabel,omitempty"`
	Storage           LibraryFileStorageDTO `json:"storage"`
	Origin            LibraryFileOriginDTO  `json:"origin"`
	Lineage           LibraryFileLineageDTO `json:"lineage"`
	Metadata          LibraryFileMetaDTO    `json:"metadata"`
	LatestOperationID string                `json:"latestOperationId,omitempty"`
	Media             *LibraryMediaInfoDTO  `json:"media,omitempty"`
	State             LibraryFileStateDTO   `json:"state"`
	CreatedAt         string                `json:"createdAt"`
	UpdatedAt         string                `json:"updatedAt"`
}

type LibraryFileMetaDTO struct {
	Title     string `json:"title,omitempty"`
	Author    string `json:"author,omitempty"`
	Extractor string `json:"extractor,omitempty"`
}

type LibraryFileStorageDTO struct {
	Mode       string `json:"mode"`
	LocalPath  string `json:"localPath,omitempty"`
	DocumentID string `json:"documentId,omitempty"`
}

type LibraryFileOriginDTO struct {
	Kind        string                  `json:"kind"`
	OperationID string                  `json:"operationId,omitempty"`
	Import      *LibraryImportOriginDTO `json:"import,omitempty"`
}

type LibraryImportOriginDTO struct {
	BatchID        string `json:"batchId"`
	ImportPath     string `json:"importPath"`
	ImportedAt     string `json:"importedAt"`
	KeepSourceFile bool   `json:"keepSourceFile"`
}

type LibraryFileLineageDTO struct {
	RootFileID string `json:"rootFileId,omitempty"`
}

type LibraryMediaInfoDTO struct {
	Format           string   `json:"format,omitempty"`
	Codec            string   `json:"codec,omitempty"`
	VideoCodec       string   `json:"videoCodec,omitempty"`
	AudioCodec       string   `json:"audioCodec,omitempty"`
	DurationMs       *int64   `json:"durationMs,omitempty"`
	Width            *int     `json:"width,omitempty"`
	Height           *int     `json:"height,omitempty"`
	FrameRate        *float64 `json:"frameRate,omitempty"`
	BitrateKbps      *int     `json:"bitrateKbps,omitempty"`
	VideoBitrateKbps *int     `json:"videoBitrateKbps,omitempty"`
	AudioBitrateKbps *int     `json:"audioBitrateKbps,omitempty"`
	Channels         *int     `json:"channels,omitempty"`
	SizeBytes        *int64   `json:"sizeBytes,omitempty"`
	DPI              *int     `json:"dpi,omitempty"`
	Language         string   `json:"language,omitempty"`
	CueCount         *int     `json:"cueCount,omitempty"`
}

type LibraryFileStateDTO struct {
	Status      string `json:"status"`
	Deleted     bool   `json:"deleted"`
	Archived    bool   `json:"archived"`
	LastError   string `json:"lastError,omitempty"`
	LastChecked string `json:"lastChecked,omitempty"`
}

type LibraryOperationDTO struct {
	ID                    string                      `json:"id"`
	LibraryID             string                      `json:"libraryId"`
	Kind                  string                      `json:"kind"`
	Status                string                      `json:"status"`
	DisplayName           string                      `json:"displayName"`
	Correlation           OperationCorrelationDTO     `json:"correlation"`
	InputJSON             string                      `json:"inputJson"`
	OutputJSON            string                      `json:"outputJson"`
	SourceDomain          string                      `json:"sourceDomain,omitempty"`
	SourceIcon            string                      `json:"sourceIcon,omitempty"`
	Meta                  OperationMetaDTO            `json:"meta"`
	Request               *OperationRequestPreviewDTO `json:"request,omitempty"`
	Progress              *OperationProgressDTO       `json:"progress,omitempty"`
	OutputFiles           []OperationOutputFileDTO    `json:"outputFiles,omitempty"`
	DetachedOutputFileIDs []string                    `json:"detachedOutputFileIds,omitempty"`
	ThumbnailPreviewPath  string                      `json:"thumbnailPreviewPath,omitempty"`
	Metrics               OperationMetricsDTO         `json:"metrics"`
	ErrorCode             string                      `json:"errorCode,omitempty"`
	ErrorMessage          string                      `json:"errorMessage,omitempty"`
	CreatedAt             string                      `json:"createdAt"`
	StartedAt             string                      `json:"startedAt,omitempty"`
	FinishedAt            string                      `json:"finishedAt,omitempty"`
}

type OperationMetaDTO struct {
	Platform    string `json:"platform,omitempty"`
	Uploader    string `json:"uploader,omitempty"`
	PublishTime string `json:"publishTime,omitempty"`
}

type OperationRequestPreviewDTO struct {
	URL                            string `json:"url,omitempty"`
	Caller                         string `json:"caller,omitempty"`
	Extractor                      string `json:"extractor,omitempty"`
	Author                         string `json:"author,omitempty"`
	ThumbnailURL                   string `json:"thumbnailUrl,omitempty"`
	DownloadMethod                 string `json:"downloadMethod,omitempty"`
	FileID                         string `json:"fileId,omitempty"`
	InputPath                      string `json:"inputPath,omitempty"`
	RootFileID                     string `json:"rootFileId,omitempty"`
	PresetID                       string `json:"presetId,omitempty"`
	Format                         string `json:"format,omitempty"`
	VideoCodec                     string `json:"videoCodec,omitempty"`
	AudioCodec                     string `json:"audioCodec,omitempty"`
	QualityMode                    string `json:"qualityMode,omitempty"`
	Scale                          string `json:"scale,omitempty"`
	Width                          int    `json:"width,omitempty"`
	Height                         int    `json:"height,omitempty"`
	DeleteSourceFileAfterTranscode bool   `json:"deleteSourceFileAfterTranscode,omitempty"`
}

type OperationCorrelationDTO struct {
	RequestID         string `json:"requestId,omitempty"`
	RunID             string `json:"runId,omitempty"`
	ParentOperationID string `json:"parentOperationId,omitempty"`
}

type OperationProgressDTO struct {
	Stage       string                   `json:"stage,omitempty"`
	Percent     *int                     `json:"percent,omitempty"`
	Current     *int64                   `json:"current,omitempty"`
	Total       *int64                   `json:"total,omitempty"`
	Speed       string                   `json:"speed,omitempty"`
	SpeedMetric *OperationSpeedMetricDTO `json:"speedMetric,omitempty"`
	Message     string                   `json:"message,omitempty"`
	UpdatedAt   string                   `json:"updatedAt,omitempty"`
}

type OperationSpeedMetricDTO struct {
	Kind            string   `json:"kind,omitempty"`
	Label           string   `json:"label,omitempty"`
	BytesPerSecond  *float64 `json:"bytesPerSecond,omitempty"`
	FramesPerSecond *float64 `json:"framesPerSecond,omitempty"`
	Factor          *float64 `json:"factor,omitempty"`
}

type OperationOutputFileDTO struct {
	FileID    string `json:"fileId"`
	Kind      string `json:"kind"`
	Format    string `json:"format,omitempty"`
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
	IsPrimary bool   `json:"isPrimary,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
}

type OperationMetricsDTO struct {
	FileCount      int    `json:"fileCount"`
	TotalSizeBytes *int64 `json:"totalSizeBytes,omitempty"`
	DurationMs     *int64 `json:"durationMs,omitempty"`
}

type OperationListItemDTO struct {
	OperationID           string                      `json:"operationId"`
	LibraryID             string                      `json:"libraryId"`
	LibraryName           string                      `json:"libraryName,omitempty"`
	Name                  string                      `json:"name"`
	Kind                  string                      `json:"kind"`
	Status                string                      `json:"status"`
	Correlation           OperationCorrelationDTO     `json:"correlation"`
	Domain                string                      `json:"domain,omitempty"`
	SourceIcon            string                      `json:"sourceIcon,omitempty"`
	Platform              string                      `json:"platform,omitempty"`
	Uploader              string                      `json:"uploader,omitempty"`
	PublishTime           string                      `json:"publishTime,omitempty"`
	Request               *OperationRequestPreviewDTO `json:"request,omitempty"`
	Progress              *OperationProgressDTO       `json:"progress,omitempty"`
	OutputFiles           []OperationOutputFileDTO    `json:"outputFiles,omitempty"`
	DetachedOutputFileIDs []string                    `json:"detachedOutputFileIds,omitempty"`
	ThumbnailPreviewPath  string                      `json:"thumbnailPreviewPath,omitempty"`
	Metrics               OperationMetricsDTO         `json:"metrics"`
	ErrorCode             string                      `json:"errorCode,omitempty"`
	ErrorMessage          string                      `json:"errorMessage,omitempty"`
	StartedAt             string                      `json:"startedAt,omitempty"`
	FinishedAt            string                      `json:"finishedAt,omitempty"`
	CreatedAt             string                      `json:"createdAt"`
}

type LibraryRecordsDTO struct {
	History            []LibraryHistoryRecordDTO `json:"history"`
	WorkspaceStateHead *WorkspaceStateRecordDTO  `json:"workspaceStateHead,omitempty"`
	WorkspaceStates    []WorkspaceStateRecordDTO `json:"workspaceStates"`
	FileEvents         []FileEventRecordDTO      `json:"fileEvents"`
}

type WorkspaceStateRecordDTO struct {
	ID           string `json:"id"`
	LibraryID    string `json:"libraryId"`
	StateVersion int    `json:"stateVersion"`
	StateJSON    string `json:"stateJson"`
	OperationID  string `json:"operationId,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type WorkspaceProjectDTO struct {
	Version        string                      `json:"version"`
	LibraryID      string                      `json:"libraryId"`
	Title          string                      `json:"title"`
	UpdatedAt      string                      `json:"updatedAt"`
	ViewStateHead  *WorkspaceStateRecordDTO    `json:"viewStateHead,omitempty"`
	VideoTracks    []WorkspaceVideoTrackDTO    `json:"videoTracks"`
	SubtitleTracks []WorkspaceSubtitleTrackDTO `json:"subtitleTracks"`
}

type WorkspaceTrackDisplayDTO struct {
	Label  string   `json:"label"`
	Hint   string   `json:"hint,omitempty"`
	Badges []string `json:"badges,omitempty"`
}

type WorkspaceTaskSummaryDTO struct {
	OperationID string `json:"operationId"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	DisplayName string `json:"displayName"`
	Stage       string `json:"stage,omitempty"`
	Current     int64  `json:"current,omitempty"`
	Total       int64  `json:"total,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type WorkspaceTrackTasksDTO struct {
	Transcode *WorkspaceTaskSummaryDTO `json:"transcode,omitempty"`
}

type WorkspaceVideoTrackDTO struct {
	TrackID string                   `json:"trackId"`
	File    LibraryFileDTO           `json:"file"`
	Display WorkspaceTrackDisplayDTO `json:"display"`
}

type WorkspaceSubtitleTrackDTO struct {
	TrackID      string                   `json:"trackId"`
	Role         string                   `json:"role"`
	File         LibraryFileDTO           `json:"file"`
	Display      WorkspaceTrackDisplayDTO `json:"display"`
	RunningTasks WorkspaceTrackTasksDTO   `json:"runningTasks"`
}

type LibraryHistoryRecordDTO struct {
	RecordID      string                         `json:"recordId"`
	LibraryID     string                         `json:"libraryId"`
	Category      string                         `json:"category"`
	Action        string                         `json:"action"`
	DisplayName   string                         `json:"displayName"`
	Status        string                         `json:"status"`
	Source        LibraryHistoryRecordSourceDTO  `json:"source"`
	Refs          LibraryHistoryRecordRefsDTO    `json:"refs"`
	Files         []OperationOutputFileDTO       `json:"files,omitempty"`
	Metrics       OperationMetricsDTO            `json:"metrics"`
	ImportMeta    *LibraryImportRecordMetaDTO    `json:"importMeta,omitempty"`
	OperationMeta *LibraryOperationRecordMetaDTO `json:"operationMeta,omitempty"`
	OccurredAt    string                         `json:"occurredAt"`
	CreatedAt     string                         `json:"createdAt"`
}

type LibraryHistoryRecordSourceDTO struct {
	Kind   string `json:"kind"`
	Caller string `json:"caller,omitempty"`
	RunID  string `json:"runId,omitempty"`
	Actor  string `json:"actor,omitempty"`
}

type LibraryHistoryRecordRefsDTO struct {
	OperationID   string   `json:"operationId,omitempty"`
	ImportBatchID string   `json:"importBatchId,omitempty"`
	FileIDs       []string `json:"fileIds,omitempty"`
	FileEventIDs  []string `json:"fileEventIds,omitempty"`
}

type LibraryImportRecordMetaDTO struct {
	ImportPath     string `json:"importPath,omitempty"`
	KeepSourceFile bool   `json:"keepSourceFile"`
	ImportedAt     string `json:"importedAt"`
}

type LibraryOperationRecordMetaDTO struct {
	Kind         string `json:"kind"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type FileEventRecordDTO struct {
	ID          string             `json:"id"`
	LibraryID   string             `json:"libraryId"`
	FileID      string             `json:"fileId"`
	EventType   string             `json:"eventType"`
	OperationID string             `json:"operationId,omitempty"`
	Detail      FileEventDetailDTO `json:"detail"`
	// OccurredAt is the immutable business-event timestamp. CreatedAt remains
	// available for wire compatibility; file events are append-only today, so
	// both values intentionally refer to the same instant.
	OccurredAt string `json:"occurredAt"`
	CreatedAt  string `json:"createdAt"`
}

type FileEventDetailDTO struct {
	Cause   FileEventCauseDTO         `json:"cause"`
	Before  *FileEventFileSnapshotDTO `json:"before,omitempty"`
	After   *FileEventFileSnapshotDTO `json:"after,omitempty"`
	Changes []FileFieldChangeDTO      `json:"changes,omitempty"`
	Import  *LibraryImportOriginDTO   `json:"import,omitempty"`
	// DeleteFile distinguishes a metadata/association-only mutation from an
	// explicit request to remove the local file as part of the same action.
	DeleteFile bool `json:"deleteFile"`
}

type FileEventCauseDTO struct {
	Category      string `json:"category"`
	OperationID   string `json:"operationId,omitempty"`
	ImportBatchID string `json:"importBatchId,omitempty"`
	Actor         string `json:"actor,omitempty"`
}

type FileEventFileSnapshotDTO struct {
	FileID     string `json:"fileId"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	LocalPath  string `json:"localPath,omitempty"`
	DocumentID string `json:"documentId,omitempty"`
}

type FileFieldChangeDTO struct {
	Field  string `json:"field"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

type LibraryModuleConfigDTO struct {
	Workspace LibraryWorkspaceConfigDTO `json:"workspace"`
}

type LibraryWorkspaceConfigDTO struct {
	FastReadLatestState bool `json:"fastReadLatestState"`
}

type GetLibraryRequest struct {
	LibraryID string `json:"libraryId"`
}

type RenameLibraryRequest struct {
	LibraryID string `json:"libraryId"`
	Name      string `json:"name"`
}

type DeleteLibraryRequest struct {
	LibraryID string `json:"libraryId"`
}

type UpdateLibraryModuleConfigRequest struct {
	Config LibraryModuleConfigDTO `json:"config"`
}

type ListOperationsRequest struct {
	LibraryID string   `json:"libraryId,omitempty"`
	Status    []string `json:"status,omitempty"`
	Kinds     []string `json:"kinds,omitempty"`
	Query     string   `json:"query,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Offset    int      `json:"offset,omitempty"`
}

type GetOperationRequest struct {
	OperationID string `json:"operationId"`
}

type RenameOperationRequest struct {
	OperationID string `json:"operationId"`
	Name        string `json:"name"`
}

type RenameFileRequest struct {
	FileID string `json:"fileId"`
	Name   string `json:"name"`
}

type CancelOperationRequest struct {
	OperationID string `json:"operationId"`
}

type ResumeOperationRequest struct {
	OperationID string `json:"operationId"`
}

type DeleteOperationRequest struct {
	OperationID  string `json:"operationId"`
	CascadeFiles bool   `json:"cascadeFiles,omitempty"`
}

type DeleteOperationsRequest struct {
	OperationIDs []string `json:"operationIds"`
	CascadeFiles bool     `json:"cascadeFiles,omitempty"`
}

// DeleteOperationOutputRequest removes one output from a task without
// implicitly deleting the Library file. DeleteFile opts into the destructive
// file lifecycle as a separate, explicit choice.
type DeleteOperationOutputRequest struct {
	OperationID string `json:"operationId"`
	FileID      string `json:"fileId"`
	DeleteFile  bool   `json:"deleteFile,omitempty"`
}

type DeleteFileRequest struct {
	FileID      string `json:"fileId"`
	DeleteFiles bool   `json:"deleteFiles,omitempty"`
}

type DeleteFilesRequest struct {
	FileIDs     []string `json:"fileIds"`
	DeleteFiles bool     `json:"deleteFiles,omitempty"`
}

type ListLibraryHistoryRequest struct {
	LibraryID  string   `json:"libraryId"`
	Categories []string `json:"categories,omitempty"`
	Actions    []string `json:"actions,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

type ListFileEventsRequest struct {
	LibraryID string `json:"libraryId"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

type SaveWorkspaceStateRequest struct {
	LibraryID   string `json:"libraryId"`
	StateJSON   string `json:"stateJson"`
	OperationID string `json:"operationId,omitempty"`
}

type GetWorkspaceStateRequest struct {
	LibraryID string `json:"libraryId"`
}

type GetWorkspaceProjectRequest struct {
	LibraryID string `json:"libraryId"`
}

type OpenFileLocationRequest struct {
	FileID string `json:"fileId"`
}
type OpenPathRequest struct {
	Path string `json:"path"`
}

type ListListenLocalTracksRequest struct {
	Query              string `json:"query,omitempty"`
	Artist             string `json:"artist,omitempty"`
	Album              string `json:"album,omitempty"`
	Sort               string `json:"sort,omitempty"`
	IncludeUnavailable bool   `json:"includeUnavailable,omitempty"`
	Limit              int    `json:"limit,omitempty"`
	Offset             int    `json:"offset,omitempty"`
}

type ListenLocalTrackDTO struct {
	ID               string `json:"id"`
	FileID           string `json:"fileId"`
	LibraryID        string `json:"libraryId"`
	Title            string `json:"title"`
	Author           string `json:"author,omitempty"`
	Album            string `json:"album,omitempty"`
	AlbumArtist      string `json:"albumArtist,omitempty"`
	Genre            string `json:"genre,omitempty"`
	TrackNumber      int    `json:"trackNumber,omitempty"`
	DiscNumber       int    `json:"discNumber,omitempty"`
	Year             int    `json:"year,omitempty"`
	LocalPath        string `json:"localPath"`
	CoverLocalPath   string `json:"coverLocalPath,omitempty"`
	Format           string `json:"format,omitempty"`
	AudioCodec       string `json:"audioCodec,omitempty"`
	DurationMs       *int64 `json:"durationMs,omitempty"`
	SizeBytes        *int64 `json:"sizeBytes,omitempty"`
	ModTimeUnix      int64  `json:"modTimeUnix,omitempty"`
	Availability     string `json:"availability"`
	LastCheckedAt    string `json:"lastCheckedAt,omitempty"`
	ProbeError       string `json:"probeError,omitempty"`
	CreatedAt        string `json:"createdAt,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	MetadataWritable bool   `json:"metadataWritable"`
}

type UpdateListenLocalTrackMetadataRequest struct {
	FileID      string `json:"fileId"`
	Title       string `json:"title"`
	Author      string `json:"author,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"albumArtist,omitempty"`
	Genre       string `json:"genre,omitempty"`
	TrackNumber int    `json:"trackNumber,omitempty"`
	DiscNumber  int    `json:"discNumber,omitempty"`
	Year        int    `json:"year,omitempty"`
}

type ListenLocalPlaylistDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Revision  int64  `json:"revision"`
	ItemCount int    `json:"itemCount"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ListenLocalPlaylistItemDTO struct {
	ID       string              `json:"id"`
	Position int                 `json:"position"`
	AddedAt  string              `json:"addedAt"`
	Track    ListenLocalTrackDTO `json:"track"`
}

type ListenLocalPlaylistDetailDTO struct {
	Playlist ListenLocalPlaylistDTO       `json:"playlist"`
	Items    []ListenLocalPlaylistItemDTO `json:"items"`
}

type CreateListenLocalPlaylistRequest struct {
	Name string `json:"name"`
}

type UpdateListenLocalPlaylistRequest struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type DeleteListenLocalPlaylistRequest struct {
	ID               string `json:"id"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type AddListenLocalPlaylistItemsRequest struct {
	ID               string   `json:"id"`
	FileIDs          []string `json:"fileIds"`
	ExpectedRevision int64    `json:"expectedRevision"`
}

type ReplaceListenLocalPlaylistItemsRequest struct {
	ID               string   `json:"id"`
	FileIDs          []string `json:"fileIds"`
	ItemIDs          []string `json:"itemIds,omitempty"`
	ExpectedRevision int64    `json:"expectedRevision"`
}

type RemoveListenLocalPlaylistItemRequest struct {
	ID               string `json:"id"`
	FileID           string `json:"fileId"`
	ItemID           string `json:"itemId,omitempty"`
	ExpectedRevision int64  `json:"expectedRevision"`
}

type RefreshListenLocalIndexRequest struct {
	FileID    string `json:"fileId,omitempty"`
	LibraryID string `json:"libraryId,omitempty"`
}

type ListenLocalIndexRefreshResponse struct {
	Scanned int `json:"scanned"`
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Missing int `json:"missing"`
	Failed  int `json:"failed"`
}

type RemoveListenLocalTrackRequest struct {
	FileID string `json:"fileId"`
}

type ClearMissingListenLocalTracksResponse struct {
	Removed int `json:"removed"`
}

type VerifyLibraryFilesResponse struct {
	Checked int `json:"checked"`
	Missing int `json:"missing"`
}

type ClearMissingLibraryFilesResponse struct {
	Checked int `json:"checked"`
	Removed int `json:"removed"`
}

// ClearSelectedMissingLibraryFilesRequest deliberately carries only durable
// file identities. The service reloads every record and rechecks the current
// path while holding the per-file mutation lock before clearing it.
type ClearSelectedMissingLibraryFilesRequest struct {
	FileIDs []string `json:"fileIds"`
}

type MissingLibraryFileDTO struct {
	FileID      string `json:"fileId"`
	LibraryID   string `json:"libraryId"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	OldPath     string `json:"oldPath"`
	Format      string `json:"format,omitempty"`
	SizeBytes   *int64 `json:"sizeBytes,omitempty"`
	DurationMs  *int64 `json:"durationMs,omitempty"`
	LastChecked string `json:"lastChecked,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type ListMissingLibraryFilesResponse struct {
	Checked int                     `json:"checked"`
	Missing []MissingLibraryFileDTO `json:"missing"`
}

type DeletedLibraryFileDTO struct {
	FileID     string `json:"fileId"`
	LibraryID  string `json:"libraryId"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	OldPath    string `json:"oldPath,omitempty"`
	Format     string `json:"format,omitempty"`
	CanRestore bool   `json:"canRestore"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
}

// LibraryTaskMaintenanceDTO keeps historical execution status separate from
// current output availability. A succeeded task is only actionable when none
// of its playable outputs remain available; deleted intermediate sources and
// auxiliary artwork do not make an otherwise playable task unhealthy.
type LibraryTaskMaintenanceDTO struct {
	OperationID            string `json:"operationId"`
	Name                   string `json:"name"`
	ExecutionStatus        string `json:"executionStatus"`
	Health                 string `json:"health"`
	OutputCount            int    `json:"outputCount"`
	AvailableOutputCount   int    `json:"availableOutputCount"`
	DeletedOutputCount     int    `json:"deletedOutputCount"`
	UnavailableOutputCount int    `json:"unavailableOutputCount"`
}

type LibraryMaintenanceSnapshotDTO struct {
	CheckedFiles      int                         `json:"checkedFiles"`
	MissingFiles      []MissingLibraryFileDTO     `json:"missingFiles"`
	DeletedFiles      []DeletedLibraryFileDTO     `json:"deletedFiles"`
	CheckedTasks      int                         `json:"checkedTasks"`
	TaskIssues        []LibraryTaskMaintenanceDTO `json:"taskIssues"`
	DatabaseIntegrity DatabaseIntegrityStatusDTO  `json:"databaseIntegrity"`
}

// DatabaseIntegrityStatusDTO exposes the background SQLite integrity result
// without rerunning an expensive full check from the UI.
type DatabaseIntegrityStatusDTO struct {
	State     string `json:"state"`
	CheckedAt string `json:"checkedAt,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type RestoreDeletedLibraryFilesRequest struct {
	FileIDs []string `json:"fileIds"`
}

type RestoreDeletedLibraryFilesResponse struct {
	Checked  int `json:"checked"`
	Restored int `json:"restored"`
	Skipped  int `json:"skipped"`
}

// DeletedLibraryItemDTO is the read model for the user-facing Deleted
// companion. Task entries are immutable operation-deletion snapshots; file
// entries are recoverable legacy file tombstones. Catalog trash remains owned
// by CatalogHandler and is intentionally not folded into this boundary.
type DeletedLibraryItemDTO struct {
	ID         string                   `json:"id"`
	Kind       string                   `json:"kind"`
	Source     string                   `json:"source"`
	LibraryID  string                   `json:"libraryId"`
	Title      string                   `json:"title"`
	Category   string                   `json:"category"`
	Status     string                   `json:"status"`
	DeletedAt  string                   `json:"deletedAt"`
	CanRestore bool                     `json:"canRestore"`
	Detail     DeletedLibraryItemDetail `json:"detail"`
}

type DeletedLibraryItemDetail struct {
	TaskHistory *LibraryHistoryRecordDTO `json:"taskHistory,omitempty"`
	File        *LibraryFileDTO          `json:"file,omitempty"`
}

type ListDeletedLibraryItemsRequest struct {
	Kinds     []string `json:"kinds,omitempty"`
	LibraryID string   `json:"libraryId,omitempty"`
	Category  string   `json:"category,omitempty"`
	Query     string   `json:"query,omitempty"`
	Limit     int      `json:"limit,omitempty"`
	Offset    int      `json:"offset,omitempty"`
}

type ListDeletedLibraryItemsResponse struct {
	Items  []DeletedLibraryItemDTO `json:"items"`
	Total  int                     `json:"total"`
	Limit  int                     `json:"limit"`
	Offset int                     `json:"offset"`
}

type DeletedLibraryItemMutationRequest struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type DeletedLibraryItemMutationResponse struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Restored bool   `json:"restored"`
}

type ScanMissingLibraryFilesRequest struct {
	Directory string   `json:"directory"`
	FileIDs   []string `json:"fileIds,omitempty"`
}

type ScanMissingLibraryFilesResponse struct {
	Directory    string                  `json:"directory"`
	Checked      int                     `json:"checked"`
	MissingCount int                     `json:"missingCount"`
	ScannedFiles int                     `json:"scannedFiles"`
	Matches      []LibraryRelinkMatchDTO `json:"matches"`
}

type ApplyLibraryRelinksRequest struct {
	Matches []LibraryRelinkSelectionDTO `json:"matches"`
}

type LibraryRelinkSelectionDTO struct {
	FileID string `json:"fileId"`
	Path   string `json:"path"`
}

type ApplyLibraryRelinksResponse struct {
	Relinked int              `json:"relinked"`
	Files    []LibraryFileDTO `json:"files"`
}

type LibraryRelinkMatchDTO struct {
	FileID     string   `json:"fileId"`
	LibraryID  string   `json:"libraryId"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	OldPath    string   `json:"oldPath"`
	NewPath    string   `json:"newPath"`
	Format     string   `json:"format,omitempty"`
	SizeBytes  *int64   `json:"sizeBytes,omitempty"`
	DurationMs *int64   `json:"durationMs,omitempty"`
	Score      int      `json:"score"`
	Confidence string   `json:"confidence"`
	Reasons    []string `json:"reasons,omitempty"`
}

type CreateYTDLPJobRequest struct {
	URL                            string   `json:"url"`
	LibraryID                      string   `json:"libraryId,omitempty"`
	Title                          string   `json:"title"`
	Extractor                      string   `json:"extractor,omitempty"`
	Author                         string   `json:"author,omitempty"`
	ThumbnailURL                   string   `json:"thumbnailUrl,omitempty"`
	WriteThumbnail                 bool     `json:"writeThumbnail,omitempty"`
	CookiesPath                    string   `json:"cookiesPath,omitempty"`
	Source                         string   `json:"source,omitempty"`
	Caller                         string   `json:"caller,omitempty"`
	SessionKey                     string   `json:"sessionKey,omitempty"`
	RunID                          string   `json:"runId,omitempty"`
	RetryOf                        string   `json:"retryOf,omitempty"`
	RetryCount                     int      `json:"retryCount,omitempty"`
	Mode                           string   `json:"mode,omitempty"`
	LogPolicy                      string   `json:"logPolicy,omitempty"`
	Quality                        string   `json:"quality,omitempty"`
	FormatID                       string   `json:"formatId,omitempty"`
	AudioFormatID                  string   `json:"audioFormatId,omitempty"`
	SubtitleLangs                  []string `json:"subtitleLangs,omitempty"`
	SubtitleAuto                   bool     `json:"subtitleAuto,omitempty"`
	SubtitleAll                    bool     `json:"subtitleAll,omitempty"`
	SubtitleFormat                 string   `json:"subtitleFormat,omitempty"`
	TranscodePresetID              string   `json:"transcodePresetId,omitempty"`
	DeleteSourceFileAfterTranscode bool     `json:"deleteSourceFileAfterTranscode,omitempty"`
	AppSessionID                   string   `json:"appSessionId,omitempty"`
	UseAppSession                  bool     `json:"useAppSession,omitempty"`
	ResourceSessionID              string   `json:"resourceSessionId,omitempty"`
	ResourceMediaID                string   `json:"resourceMediaId,omitempty"`
	// RestrictedProxyURL is execution-only state. It is deliberately excluded
	// from persisted operation input so queued/retried public API jobs must
	// establish a fresh enforcing proxy at the real fetch boundary.
	RestrictedProxyURL string `json:"-"`
}

type CreateYTDLPBatchJobsRequest struct {
	Items []CreateYTDLPJobRequest `json:"items"`
}

type CreateYTDLPBatchJobsResponse struct {
	Operations []LibraryOperationDTO `json:"operations"`
}

type CheckYTDLPOperationFailureRequest struct {
	OperationID string `json:"operationId"`
}

type CheckYTDLPOperationFailureItem struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Action  string `json:"action,omitempty"`
}

type CheckYTDLPOperationFailureResponse struct {
	Items    []CheckYTDLPOperationFailureItem `json:"items"`
	CanRetry bool                             `json:"canRetry"`
}

type RetryYTDLPOperationRequest struct {
	OperationID string `json:"operationId"`
	Source      string `json:"source,omitempty"`
	Caller      string `json:"caller,omitempty"`
	RunID       string `json:"runId,omitempty"`
}

type GetYTDLPOperationLogRequest struct {
	OperationID string `json:"operationId"`
	MaxBytes    int    `json:"maxBytes,omitempty"`
	TailLines   int    `json:"tailLines,omitempty"`
}

type GetYTDLPOperationLogResponse struct {
	OperationID string `json:"operationId"`
	Path        string `json:"path,omitempty"`
	Content     string `json:"content,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

type PrepareYTDLPDownloadRequest struct {
	URL string `json:"url"`
}
type PreparedYTDLPDownloadURL struct {
	URL                       string `json:"url"`
	Domain                    string `json:"domain"`
	Icon                      string `json:"icon,omitempty"`
	AppSessionID              string `json:"appSessionId,omitempty"`
	AppSessionAvailable       bool   `json:"appSessionAvailable"`
	AppSessionCredentialMode  string `json:"appSessionCredentialMode,omitempty"`
	AppSessionCredentialState string `json:"appSessionCredentialState,omitempty"`
	Reachable                 bool   `json:"reachable,omitempty"`
}
type PrepareYTDLPDownloadResponse struct {
	Mode                      string                     `json:"mode,omitempty"`
	URL                       string                     `json:"url"`
	Domain                    string                     `json:"domain"`
	Icon                      string                     `json:"icon,omitempty"`
	AppSessionID              string                     `json:"appSessionId,omitempty"`
	AppSessionAvailable       bool                       `json:"appSessionAvailable"`
	AppSessionCredentialMode  string                     `json:"appSessionCredentialMode,omitempty"`
	AppSessionCredentialState string                     `json:"appSessionCredentialState,omitempty"`
	Reachable                 bool                       `json:"reachable,omitempty"`
	URLs                      []PreparedYTDLPDownloadURL `json:"urls,omitempty"`
}

type ResolveDomainIconRequest struct {
	Domain string `json:"domain"`
	URL    string `json:"url,omitempty"`
}
type ResolveDomainIconResponse struct {
	Domain string `json:"domain,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

type ParseYTDLPDownloadRequest struct {
	URL           string `json:"url"`
	AppSessionID  string `json:"appSessionId,omitempty"`
	UseAppSession bool   `json:"useAppSession,omitempty"`
}

type YTDLPFormatOption struct {
	ID            string  `json:"id"`
	Label         string  `json:"label"`
	HasVideo      bool    `json:"hasVideo"`
	HasAudio      bool    `json:"hasAudio"`
	Ext           string  `json:"ext,omitempty"`
	Height        int     `json:"height,omitempty"`
	VCodec        string  `json:"vcodec,omitempty"`
	ACodec        string  `json:"acodec,omitempty"`
	FormatNote    string  `json:"formatNote,omitempty"`
	Language      string  `json:"language,omitempty"`
	TBR           float64 `json:"tbr,omitempty"`
	ABR           float64 `json:"abr,omitempty"`
	VBR           float64 `json:"vbr,omitempty"`
	AudioChannels int     `json:"audioChannels,omitempty"`
	Filesize      int64   `json:"filesize,omitempty"`
}

type YTDLPSubtitleOption struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Name     string `json:"name,omitempty"`
	IsAuto   bool   `json:"isAuto,omitempty"`
	Ext      string `json:"ext,omitempty"`
}

type ParseYTDLPDownloadResponse struct {
	Title             string                     `json:"title,omitempty"`
	Domain            string                     `json:"domain,omitempty"`
	Extractor         string                     `json:"extractor,omitempty"`
	Author            string                     `json:"author,omitempty"`
	ThumbnailURL      string                     `json:"thumbnailUrl,omitempty"`
	PageURL           string                     `json:"pageUrl,omitempty"`
	ResourceSessionID string                     `json:"resourceSessionId,omitempty"`
	ResourceMediaID   string                     `json:"resourceMediaId,omitempty"`
	PlaylistItems     []PreparedYTDLPDownloadURL `json:"playlistItems,omitempty"`
	Formats           []YTDLPFormatOption        `json:"formats"`
	Subtitles         []YTDLPSubtitleOption      `json:"subtitles"`
}

type StartResourceSniffRequest struct {
	URL       string `json:"url"`
	Mode      string `json:"mode,omitempty"`
	BrowserID string `json:"browserId,omitempty"`
	ProfileID string `json:"profileId,omitempty"`
}

type CurrentResourceSniffBrowserStatusRequest struct {
	BrowserID string `json:"browserId"`
}

type CurrentResourceSniffBrowserStatus struct {
	BrowserID      string `json:"browserId"`
	State          string `json:"state"`
	Installed      bool   `json:"installed"`
	Running        bool   `json:"running"`
	Supported      bool   `json:"supported"`
	Ready          bool   `json:"ready"`
	Version        string `json:"version,omitempty"`
	MinimumVersion int    `json:"minimumVersion"`
	ProfileName    string `json:"profileName,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

type StartResourceSniffResult struct {
	Session *ResourceSniffSession `json:"session,omitempty"`
}

type GetResourceSniffSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type ListResourceSniffResourcesRequest struct {
	SessionID string `json:"sessionId"`
}

type ClearResourceSniffResourcesRequest struct {
	SessionID string `json:"sessionId"`
}

type GetResourceSniffPreviewRequest struct {
	SessionID  string `json:"sessionId"`
	ResourceID string `json:"resourceId"`
}

type ResourceSniffPreviewResponse struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	MimeType   string `json:"mimeType,omitempty"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
	DataBase64 string `json:"dataBase64"`
	SeenAt     string `json:"seenAt,omitempty"`
}

type PrepareResourceSniffRawPreviewRequest struct {
	SessionID  string `json:"sessionId"`
	ResourceID string `json:"resourceId"`
}

type PrepareResourceSniffRawPreviewResponse struct {
	ResourceID string `json:"resourceId"`
	LeaseID    string `json:"leaseId"`
	Kind       string `json:"kind"`
	MimeType   string `json:"mimeType,omitempty"`
	FileName   string `json:"fileName,omitempty"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
	ExpiresAt  string `json:"expiresAt,omitempty"`
}

type ListResourceSniffResourcesResponse struct {
	Session   ResourceSniffSession       `json:"session"`
	Resources []ResourceSniffRawResource `json:"resources"`
	UpdatedAt string                     `json:"updatedAt"`
}

type ResourceSniffRawResource struct {
	ID                string `json:"id"`
	Source            string `json:"source"`
	Kind              string `json:"kind"`
	URL               string `json:"url"`
	PageURL           string `json:"pageUrl,omitempty"`
	Domain            string `json:"domain,omitempty"`
	MimeType          string `json:"mimeType,omitempty"`
	ContentType       string `json:"contentType,omitempty"`
	ResourceType      string `json:"resourceType,omitempty"`
	Status            int64  `json:"status,omitempty"`
	SizeBytes         int64  `json:"sizeBytes,omitempty"`
	Score             int    `json:"score,omitempty"`
	Reason            string `json:"reason,omitempty"`
	TargetID          string `json:"targetId,omitempty"`
	SeenAt            string `json:"seenAt,omitempty"`
	Downloadable      bool   `json:"downloadable"`
	PreviewAvailable  bool   `json:"previewAvailable"`
	PreviewKind       string `json:"previewKind,omitempty"`
	PreviewMimeType   string `json:"previewMimeType,omitempty"`
	PreviewSizeBytes  int64  `json:"previewSizeBytes,omitempty"`
	PreviewDataBase64 string `json:"previewDataBase64,omitempty"`
}

type PrepareResourceSniffRawDownloadRequest struct {
	SessionID  string `json:"sessionId"`
	ResourceID string `json:"resourceId"`
}

type CancelResourceSniffRequest struct {
	SessionID string `json:"sessionId"`
}

type ResourceSniffSession struct {
	SessionID      string `json:"sessionId"`
	State          string `json:"state"`
	BrowserStatus  string `json:"browserStatus"`
	URL            string `json:"url"`
	CurrentURL     string `json:"currentUrl,omitempty"`
	Title          string `json:"title,omitempty"`
	ActiveTargetID string `json:"activeTargetId,omitempty"`
	TabCount       int    `json:"tabCount"`
	Mode           string `json:"mode,omitempty"`
	BrowserID      string `json:"browserId,omitempty"`
	ProfileID      string `json:"profileId,omitempty"`
}

// ResourceSniffStatusSnapshot is the lightweight, global activity projection
// used by workspace status surfaces. It intentionally excludes raw resources
// and preview bodies.
type ResourceSniffStatusSnapshot struct {
	Runtime           string `json:"runtime"`
	State             string `json:"state"`
	SessionID         string `json:"sessionId,omitempty"`
	RuntimeID         string `json:"runtimeId,omitempty"`
	Title             string `json:"title,omitempty"`
	URL               string `json:"url,omitempty"`
	Favicon           string `json:"favicon,omitempty"`
	ResourceCount     int    `json:"resourceCount"`
	DownloadableCount int    `json:"downloadableCount"`
	LastCaptureAt     string `json:"lastCaptureAt,omitempty"`
	CanClear          bool   `json:"canClear"`
	CanStop           bool   `json:"canStop"`
}

type CDPBrowserStatus struct {
	Active        bool                  `json:"active"`
	Mode          string                `json:"mode,omitempty"`
	Session       *ResourceSniffSession `json:"session,omitempty"`
	RuntimeID     string                `json:"runtimeId,omitempty"`
	BrowserStatus string                `json:"browserStatus,omitempty"`
	CurrentURL    string                `json:"currentUrl,omitempty"`
	Title         string                `json:"title,omitempty"`
	TabCount      int                   `json:"tabCount,omitempty"`
	PID           int                   `json:"pid,omitempty"`
	ProcessCount  int                   `json:"processCount,omitempty"`
	OrphanCount   int                   `json:"orphanCount,omitempty"`
	StartedAt     string                `json:"startedAt,omitempty"`
}

type StopCDPBrowserRuntimeRequest struct {
	RuntimeID string `json:"runtimeId"`
}

type CreateVideoImportRequest struct {
	Path       string `json:"path"`
	LibraryID  string `json:"libraryId,omitempty"`
	Title      string `json:"title"`
	Source     string `json:"source,omitempty"`
	SessionKey string `json:"sessionKey,omitempty"`
	RunID      string `json:"runId,omitempty"`
}

type CreateTranscodeJobRequest struct {
	FileID                         string   `json:"fileId,omitempty"`
	InputPath                      string   `json:"inputPath,omitempty"`
	LibraryID                      string   `json:"libraryId,omitempty"`
	RootFileID                     string   `json:"rootFileId,omitempty"`
	PresetID                       string   `json:"presetId,omitempty"`
	Format                         string   `json:"format,omitempty"`
	Title                          string   `json:"title"`
	Author                         string   `json:"author,omitempty"`
	Extractor                      string   `json:"extractor,omitempty"`
	CoverPath                      string   `json:"coverPath,omitempty"`
	SubtitlePaths                  []string `json:"subtitlePaths,omitempty"`
	Source                         string   `json:"source,omitempty"`
	SessionKey                     string   `json:"sessionKey,omitempty"`
	RunID                          string   `json:"runId,omitempty"`
	VideoCodec                     string   `json:"videoCodec,omitempty"`
	QualityMode                    string   `json:"qualityMode,omitempty"`
	CRF                            int      `json:"crf,omitempty"`
	BitrateKbps                    int      `json:"bitrateKbps,omitempty"`
	Preset                         string   `json:"preset,omitempty"`
	AudioCodec                     string   `json:"audioCodec,omitempty"`
	AudioBitrateKbps               int      `json:"audioBitrateKbps,omitempty"`
	Scale                          string   `json:"scale,omitempty"`
	Width                          int      `json:"width,omitempty"`
	Height                         int      `json:"height,omitempty"`
	DeleteSourceFileAfterTranscode bool     `json:"deleteSourceFileAfterTranscode,omitempty"`
}

type ProbeTranscodeInputRequest struct {
	FileID    string `json:"fileId,omitempty"`
	InputPath string `json:"inputPath,omitempty"`
	Source    string `json:"source,omitempty"`
}

type TranscodePresetCompatibilityDTO struct {
	PresetID   string `json:"presetId"`
	Compatible bool   `json:"compatible"`
	Reason     string `json:"reason,omitempty"`
}

type ProbeTranscodeInputResponse struct {
	Media               LibraryMediaInfoDTO               `json:"media"`
	MediaType           string                            `json:"mediaType"`
	CompatiblePresetIDs []string                          `json:"compatiblePresetIds"`
	PresetCompatibility []TranscodePresetCompatibilityDTO `json:"presetCompatibility"`
	RecommendedPresetID string                            `json:"recommendedPresetId,omitempty"`
}

type ListTranscodePresetsForDownloadRequest struct {
	MediaType string `json:"mediaType"`
}

type TranscodePreset struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	OutputType       string `json:"outputType"`
	Container        string `json:"container"`
	VideoCodec       string `json:"videoCodec,omitempty"`
	AudioCodec       string `json:"audioCodec,omitempty"`
	QualityMode      string `json:"qualityMode,omitempty"`
	CRF              int    `json:"crf,omitempty"`
	BitrateKbps      int    `json:"bitrateKbps,omitempty"`
	AudioBitrateKbps int    `json:"audioBitrateKbps,omitempty"`
	Scale            string `json:"scale,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	FFmpegPreset     string `json:"ffmpegPreset,omitempty"`
	AllowUpscale     bool   `json:"allowUpscale,omitempty"`
	RequiresVideo    bool   `json:"requiresVideo,omitempty"`
	RequiresAudio    bool   `json:"requiresAudio,omitempty"`
	IsBuiltin        bool   `json:"isBuiltin,omitempty"`
	CreatedAt        string `json:"createdAt,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
}

type DeleteTranscodePresetRequest struct {
	ID string `json:"id"`
}
