export const LIBRARY_SCHEMA_VERSION = "current"

export type TranscodePresetOutputType = "video" | "audio"
export type TranscodeQualityMode = "crf" | "bitrate"
export type TranscodeScaleMode = "original" | "2160p" | "1080p" | "720p" | "480p" | "custom"
export type FFmpegSpeedPreset = "ultrafast" | "fast" | "medium" | "slow"

export interface LibraryCreateMetaDTO {
  source: string
  triggerOperationId?: string
  importBatchId?: string
  actor?: string
}

export interface LibraryWorkspaceConfigDTO {
  fastReadLatestState: boolean
}

export interface LibraryModuleConfigDTO {
  workspace: LibraryWorkspaceConfigDTO
}

export interface LibraryFileStorageDTO {
  mode: "local_path" | "db_document" | "hybrid" | string
  localPath?: string
  documentId?: string
}

export interface LibraryImportOriginDTO {
  batchId: string
  importPath: string
  importedAt: string
  keepSourceFile: boolean
}

export interface LibraryFileOriginDTO {
  kind: string
  operationId?: string
  import?: LibraryImportOriginDTO
}

export interface LibraryFileLineageDTO {
  rootFileId?: string
}

export interface LibraryFileMetaDTO {
  title?: string
  author?: string
  extractor?: string
}

export interface LibraryMediaInfoDTO {
  format?: string
  codec?: string
  videoCodec?: string
  audioCodec?: string
  durationMs?: number
  width?: number
  height?: number
  frameRate?: number
  bitrateKbps?: number
  videoBitrateKbps?: number
  audioBitrateKbps?: number
  channels?: number
  sizeBytes?: number
  dpi?: number
  language?: string
  cueCount?: number
}

export interface LibraryFileStateDTO {
  status: string
  deleted: boolean
  archived: boolean
  lastError?: string
  lastChecked?: string
}

export interface LibraryFileDTO {
  id: string
  libraryId: string
  kind: string
  name: string
  displayName?: string
  fileName?: string
  displayLabel?: string
  storage: LibraryFileStorageDTO
  origin: LibraryFileOriginDTO
  lineage: LibraryFileLineageDTO
  metadata: LibraryFileMetaDTO
  latestOperationId?: string
  media?: LibraryMediaInfoDTO
  state: LibraryFileStateDTO
  createdAt: string
  updatedAt: string
}

export interface OperationCorrelationDTO {
  requestId?: string
  runId?: string
  parentOperationId?: string
}

export interface OperationMetaDTO {
  platform?: string
  uploader?: string
  publishTime?: string
}

export interface OperationRequestPreviewDTO {
  url?: string
  caller?: string
  extractor?: string
  author?: string
  thumbnailUrl?: string
  inputPath?: string
  presetId?: string
  format?: string
  videoCodec?: string
  audioCodec?: string
  qualityMode?: string
  scale?: string
  width?: number
  height?: number
  deleteSourceFileAfterTranscode?: boolean
}

export interface OperationProgressDTO {
  stage?: string
  percent?: number
  current?: number
  total?: number
  speed?: string
  speedMetric?: OperationSpeedMetricDTO
  message?: string
  updatedAt?: string
}

export interface OperationSpeedMetricDTO {
  kind?: string
  label?: string
  bytesPerSecond?: number
  framesPerSecond?: number
  factor?: number
}

export interface OperationOutputFileDTO {
  fileId: string
  kind: string
  format?: string
  sizeBytes?: number
  isPrimary?: boolean
  deleted?: boolean
}

export interface OperationMetricsDTO {
  fileCount: number
  totalSizeBytes?: number
  durationMs?: number
}

export interface LibraryOperationDTO {
  id: string
  libraryId: string
  kind: string
  status: string
  displayName: string
  correlation: OperationCorrelationDTO
  inputJson: string
  outputJson: string
  sourceDomain?: string
  sourceIcon?: string
  meta: OperationMetaDTO
  request?: OperationRequestPreviewDTO
  progress?: OperationProgressDTO
  outputFiles?: OperationOutputFileDTO[]
  thumbnailPreviewPath?: string
  metrics: OperationMetricsDTO
  errorCode?: string
  errorMessage?: string
  createdAt: string
  startedAt?: string
  finishedAt?: string
}

export interface OperationListItemDTO {
  operationId: string
  libraryId: string
  libraryName?: string
  name: string
  kind: string
  status: string
  domain?: string
  sourceIcon?: string
  platform?: string
  uploader?: string
  publishTime?: string
  request?: OperationRequestPreviewDTO
  progress?: OperationProgressDTO
  outputFiles?: OperationOutputFileDTO[]
  thumbnailPreviewPath?: string
  metrics: OperationMetricsDTO
  errorCode?: string
  errorMessage?: string
  startedAt?: string
  finishedAt?: string
  createdAt: string
}

export interface LibraryHistoryRecordSourceDTO {
  kind: string
  caller?: string
  runId?: string
  actor?: string
}

export interface LibraryHistoryRecordRefsDTO {
  operationId?: string
  importBatchId?: string
  fileIds?: string[]
  fileEventIds?: string[]
}

export interface LibraryImportRecordMetaDTO {
  importPath?: string
  keepSourceFile: boolean
  importedAt: string
}

export interface LibraryOperationRecordMetaDTO {
  kind: string
  errorCode?: string
  errorMessage?: string
}

export interface LibraryHistoryRecordDTO {
  recordId: string
  libraryId: string
  category: "operation" | "import" | string
  action: string
  displayName: string
  status: string
  source: LibraryHistoryRecordSourceDTO
  refs: LibraryHistoryRecordRefsDTO
  files?: OperationOutputFileDTO[]
  metrics: OperationMetricsDTO
  importMeta?: LibraryImportRecordMetaDTO
  operationMeta?: LibraryOperationRecordMetaDTO
  occurredAt: string
  createdAt: string
}

export interface WorkspaceStateRecordDTO {
  id: string
  libraryId: string
  stateVersion: number
  stateJson: string
  operationId?: string
  createdAt: string
}

export interface WorkspaceTrackDisplayDTO {
  label: string
  hint?: string
  badges?: string[]
}

export interface WorkspaceTaskSummaryDTO {
  operationId: string
  kind: string
  status: string
  displayName: string
  stage?: string
  current?: number
  total?: number
  updatedAt?: string
}

export interface WorkspaceTrackTasksDTO {
  transcode?: WorkspaceTaskSummaryDTO
}

export interface WorkspaceVideoTrackDTO {
  trackId: string
  file: LibraryFileDTO
  display: WorkspaceTrackDisplayDTO
}

export interface WorkspaceSubtitleTrackDTO {
  trackId: string
  role: "source" | string
  file: LibraryFileDTO
  display: WorkspaceTrackDisplayDTO
  runningTasks: WorkspaceTrackTasksDTO
}

export interface WorkspaceProjectDTO {
  version: string
  libraryId: string
  title: string
  updatedAt: string
  viewStateHead?: WorkspaceStateRecordDTO
  videoTracks: WorkspaceVideoTrackDTO[]
  subtitleTracks: WorkspaceSubtitleTrackDTO[]
}

export interface FileEventCauseDTO {
  category: string
  operationId?: string
  importBatchId?: string
  actor?: string
}

export interface FileEventFileSnapshotDTO {
  fileId: string
  kind: string
  name: string
  localPath?: string
  documentId?: string
}

export interface FileFieldChangeDTO {
  field: string
  before?: string
  after?: string
}

export interface FileEventDetailDTO {
  cause: FileEventCauseDTO
  before?: FileEventFileSnapshotDTO
  after?: FileEventFileSnapshotDTO
  changes?: FileFieldChangeDTO[]
  import?: LibraryImportOriginDTO
}

export interface FileEventRecordDTO {
  id: string
  libraryId: string
  fileId: string
  operationId?: string
  eventType: string
  detail: FileEventDetailDTO
  createdAt: string
}

export interface LibraryRecordsDTO {
  history: LibraryHistoryRecordDTO[]
  workspaceStateHead?: WorkspaceStateRecordDTO
  workspaceStates: WorkspaceStateRecordDTO[]
  fileEvents: FileEventRecordDTO[]
}

export interface LibraryDTO {
  version: typeof LIBRARY_SCHEMA_VERSION | string
  id: string
  name: string
  createdAt: string
  updatedAt: string
  createdBy: LibraryCreateMetaDTO
  files: LibraryFileDTO[]
  records: LibraryRecordsDTO
}

export interface GetLibraryRequest {
  libraryId: string
}

export interface RenameLibraryRequest {
  libraryId: string
  name: string
}

export interface DeleteLibraryRequest {
  libraryId: string
}

export interface UpdateLibraryModuleConfigRequest {
  config: LibraryModuleConfigDTO
}

export interface ListOperationsRequest {
  libraryId?: string
  status?: string[]
  kinds?: string[]
  query?: string
  limit?: number
  offset?: number
}

export interface GetOperationRequest {
  operationId: string
}

export interface CancelOperationRequest {
  operationId: string
}

export interface ResumeOperationRequest {
  operationId: string
}

export interface DeleteOperationRequest {
  operationId: string
  cascadeFiles?: boolean
}

export interface DeleteOperationsRequest {
  operationIds: string[]
  cascadeFiles?: boolean
}

export interface DeleteFileRequest {
  fileId: string
  deleteFiles?: boolean
}

export interface DeleteFilesRequest {
  fileIds: string[]
  deleteFiles?: boolean
}

export interface ListLibraryHistoryRequest {
  libraryId: string
  categories?: string[]
  actions?: string[]
  limit?: number
  offset?: number
}

export interface ListFileEventsRequest {
  libraryId: string
  limit?: number
  offset?: number
}

export interface SaveWorkspaceStateRequest {
  libraryId: string
  stateJson: string
  operationId?: string
}

export interface GetWorkspaceStateRequest {
  libraryId: string
}

export interface GetWorkspaceProjectRequest {
  libraryId: string
}

export interface OpenFileLocationRequest {
  fileId: string
}

export interface OpenPathRequest {
  path: string
}

export interface CreateYTDLPJobRequest {
  url: string
  libraryId?: string
  title?: string
  extractor?: string
  author?: string
  thumbnailUrl?: string
  writeThumbnail?: boolean
  cookiesPath?: string
  source?: string
  caller?: string
  sessionKey?: string
  runId?: string
  retryOf?: string
  retryCount?: number
  mode?: string
  logPolicy?: string
  quality?: string
  formatId?: string
  audioFormatId?: string
  subtitleLangs?: string[]
  subtitleAuto?: boolean
  subtitleAll?: boolean
  subtitleFormat?: string
  transcodePresetId?: string
  deleteSourceFileAfterTranscode?: boolean
  connectorId?: string
  useConnector?: boolean
}

export interface CheckYTDLPOperationFailureRequest {
  operationId: string
}

export interface CheckYTDLPOperationFailureItem {
  id: string
  label: string
  status: string
  message?: string
  action?: string
}

export interface CheckYTDLPOperationFailureResponse {
  items: CheckYTDLPOperationFailureItem[]
  canRetry: boolean
}

export interface RetryYTDLPOperationRequest {
  operationId: string
  source?: string
  caller?: string
  runId?: string
}

export interface GetYTDLPOperationLogRequest {
  operationId: string
  maxBytes?: number
  tailLines?: number
}

export interface GetYTDLPOperationLogResponse {
  operationId: string
  path?: string
  content?: string
  truncated?: boolean
}

export interface PrepareYTDLPDownloadRequest {
  url: string
}

export interface PrepareYTDLPDownloadResponse {
  url: string
  domain: string
  icon?: string
  connectorId?: string
  connectorAvailable: boolean
  reachable?: boolean
}

export interface ResolveDomainIconRequest {
  domain?: string
  url?: string
}

export interface ResolveDomainIconResponse {
  domain?: string
  icon?: string
}

export interface ParseYTDLPDownloadRequest {
  url: string
  connectorId?: string
  useConnector?: boolean
}

export interface YTDLPFormatOption {
  id: string
  label: string
  hasVideo: boolean
  hasAudio: boolean
  ext?: string
  height?: number
  vcodec?: string
  acodec?: string
  filesize?: number
}

export interface YTDLPSubtitleOption {
  id: string
  language: string
  name?: string
  isAuto?: boolean
  ext?: string
}

export interface ParseYTDLPDownloadResponse {
  title?: string
  domain?: string
  extractor?: string
  author?: string
  thumbnailUrl?: string
  formats: YTDLPFormatOption[]
  subtitles: YTDLPSubtitleOption[]
}

export interface CreateVideoImportRequest {
  path: string
  libraryId?: string
  title?: string
  source?: string
  sessionKey?: string
  runId?: string
}

export interface CreateTranscodeJobRequest {
  fileId?: string
  inputPath?: string
  libraryId?: string
  rootFileId?: string
  presetId?: string
  format?: string
  title?: string
  source?: string
  sessionKey?: string
  runId?: string
  videoCodec?: string
  qualityMode?: string
  crf?: number
  bitrateKbps?: number
  preset?: string
  audioCodec?: string
  audioBitrateKbps?: number
  scale?: string
  width?: number
  height?: number
  deleteSourceFileAfterTranscode?: boolean
}

export interface ListTranscodePresetsForDownloadRequest {
  mediaType: string
}

export interface TranscodePreset {
  id: string
  name: string
  outputType: TranscodePresetOutputType
  container: string
  videoCodec?: string
  audioCodec?: string
  qualityMode?: TranscodeQualityMode
  crf?: number
  bitrateKbps?: number
  audioBitrateKbps?: number
  scale?: TranscodeScaleMode
  width?: number
  height?: number
  ffmpegPreset?: FFmpegSpeedPreset
  allowUpscale?: boolean
  requiresVideo?: boolean
  requiresAudio?: boolean
  isBuiltin?: boolean
  createdAt?: string
  updatedAt?: string
}

export interface DeleteTranscodePresetRequest {
  id: string
}
