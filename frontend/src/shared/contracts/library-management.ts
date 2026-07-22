export type LibraryImportSelectionKind = "files" | "directory";
export type LibraryImportStorageStrategy = "referenced" | "copy";
export const LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY: LibraryImportStorageStrategy = "copy";
export type LibraryImportHiddenPolicy = "exclude" | "include";
export type LibraryImportSymlinkPolicy = "skip" | "follow_files";
export type LibraryImportBatchStatus =
  | "scanning"
  | "ready"
  | "running"
  | "cancelling"
  | "cancelled"
  | "completed"
  | "failed";
export type LibraryImportCandidateStatus =
  | "ready"
  | "duplicate"
  | "skipped"
  | "importing"
  | "copied"
  | "registered"
  | "succeeded"
  | "failed"
  | "cancelled";
export type LibraryImportCategory = "video" | "audio" | "book" | "image" | "other";

/**
 * This is the complete command accepted by the trusted desktop picker
 * boundary. Source and destination paths are deliberately absent.
 */
export interface SelectLibraryImportCommand {
  selectionKind: LibraryImportSelectionKind;
  mode: LibraryImportStorageStrategy;
  libraryId?: string;
  hiddenPolicy: LibraryImportHiddenPolicy;
  symlinkPolicy: LibraryImportSymlinkPolicy;
}

export interface LibraryImportBatchCommand {
  batchId: string;
}

export interface ListLibraryImportBatchesQuery {
  limit?: number;
}

export interface LibraryImportBatchCounts {
  total: number;
  ready: number;
  duplicate: number;
  skipped: number;
  succeeded: number;
  failed: number;
  totalBytes: number;
}

export interface LibraryImportCandidate {
  id: string;
  sourcePath: string;
  relativePath?: string;
  displayName: string;
  extension?: string;
  category: LibraryImportCategory;
  mimeType?: string;
  mediaProbed: boolean;
  wasSymlink: boolean;
  sizeBytes: number;
  modifiedAt?: string;
  hashAlgorithm?: string;
  contentHash?: string;
  status: LibraryImportCandidateStatus;
  duplicateFileId?: string;
  duplicateCandidateId?: string;
  managedPath?: string;
  fileId?: string;
  errorCode?: string;
  errorMessage?: string;
  attempts: number;
  createdAt: string;
  updatedAt: string;
}

export interface LibraryImportBatch {
  id: string;
  requestKey: string;
  libraryId: string;
  mode: LibraryImportStorageStrategy;
  managedRoot?: string;
  hiddenPolicy: LibraryImportHiddenPolicy;
  symlinkPolicy: LibraryImportSymlinkPolicy;
  status: LibraryImportBatchStatus;
  counts: LibraryImportBatchCounts;
  lastErrorCode?: string;
  lastError?: string;
  cancelRequested: boolean;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
  updatedAt: string;
  candidates?: LibraryImportCandidate[];
}

export interface LibraryBackupDatabaseIdentity {
  fileName: string;
  sha256: string;
  sizeBytes: number;
  applicationId: number;
  schemaVersion: number;
}

export interface LibraryBackupStorageRootInventory {
  id: string;
  name: string;
  mode: string;
  status: string;
  assetCount: number;
}

export interface LibraryBackupCatalogInventory {
  id: string;
  isDefault: boolean;
  itemCount: number;
  assetCount: number;
  storageRoots: LibraryBackupStorageRootInventory[];
}

export interface LibraryBackupFileInventory {
  catalogId?: string;
  itemId?: string;
  assetId?: string;
  fileId: string;
  kind: string;
  storageMode: string;
  storageRootId?: string;
  relativePath?: string;
  role?: string;
  position?: number;
}

export interface LibraryBackupManifest {
  formatVersion: number;
  backupId: string;
  purpose: string;
  appName: string;
  appVersion: string;
  createdAt: string;
  metadataOnly: boolean;
  contentIncluded: boolean;
  database: LibraryBackupDatabaseIdentity;
  catalogs: LibraryBackupCatalogInventory[];
  files: LibraryBackupFileInventory[];
}

export interface LibraryBackupSummary {
  backupId: string;
  purpose: string;
  appVersion: string;
  schemaVersion: number;
  catalogIds: string[];
  createdAt: string;
  sizeBytes: number;
  metadataOnly: boolean;
  contentIncluded: boolean;
  state: string;
  error?: string;
}

export interface LibraryBackupVerification {
  backupId: string;
  verifiedAt: string;
  valid: boolean;
  applicationId: number;
  schemaVersion: number;
  databaseSha256: string;
}

export interface LibraryBackupRestorePlan {
  backupId: string;
  rollbackBackupId: string;
  requestedAt: string;
  appliesOnLaunch: boolean;
}

export interface LibraryBackupIDCommand {
  backupId: string;
}

export interface LibraryDataManagementLabels {
  title: string;
  description: string;
  importTab: string;
  backupTab: string;
  maintenanceTab: string;
  unavailableTitle: string;
  unavailableDescription: string;
  loading: string;
  refresh: string;
  operationFailed: string;
  unknownError: string;
  closeNotice: string;
  bytes: string;
  kilobytes: string;
  megabytes: string;
  gigabytes: string;
  import: {
    title: string;
    description: string;
    selectionKind: string;
    selectFiles: string;
    selectFolder: string;
    mode: string;
    referencedMode: string;
    referencedModeDescription: string;
    copyMode: string;
    copyModeDescription: string;
    hiddenPolicy: string;
    excludeHidden: string;
    includeHidden: string;
    symlinkPolicy: string;
    skipSymlinks: string;
    followFileSymlinks: string;
    chooseAndScan: string;
    choosingAndScanning: string;
    dryRunTitle: string;
    dryRunDescription: string;
    total: string;
    ready: string;
    duplicate: string;
    skipped: string;
    succeeded: string;
    failed: string;
    totalSize: string;
    commit: string;
    committing: string;
    resume: string;
    resuming: string;
    cancel: string;
    cancelling: string;
    batchHistory: string;
    noBatches: string;
    candidates: string;
    noCandidates: string;
    revealLocalPaths: string;
    hideLocalPaths: string;
    localPathPrivacy: string;
    sourcePath: string;
    managedPath: string;
    lastError: string;
    updatedAt: string;
    scanReadyNotice: string;
    commitCompletedNotice: string;
    resumeCompletedNotice: string;
    cancelRequestedNotice: string;
    status: Record<LibraryImportBatchStatus, string>;
    candidateStatus: Record<LibraryImportCandidateStatus, string>;
  };
  backup: {
    title: string;
    description: string;
    create: string;
    creating: string;
    createCompletedNotice: string;
    backups: string;
    noBackups: string;
    privateSnapshotTitle: string;
    privateSnapshotWarning: string;
    sensitiveMetadataWarning: string;
    noMediaWarning: string;
    localPermissionsNote: string;
    verify: string;
    verifying: string;
    verificationValid: string;
    verificationInvalid: string;
    restore: string;
    restoreConfirmationTitle: string;
    restoreConfirmationPrompt: string;
    restoreConfirmationPhrase: string;
    restoreConfirmationPlaceholder: string;
    restoreNextLaunchWarning: string;
    restoreScopeWarning: string;
    restoreRollbackWarning: string;
    scheduleRestore: string;
    schedulingRestore: string;
    cancelConfirmation: string;
    restoreScheduledNotice: string;
    pendingRestoreTitle: string;
    pendingRestoreDescription: string;
    cancelPendingRestore: string;
    cancellingPendingRestore: string;
    pendingCancelledNotice: string;
    backupId: string;
    createdAt: string;
    size: string;
    schemaVersion: string;
    applicationVersion: string;
    state: string;
  };
  maintenance: {
    title: string;
    description: string;
    scan: string;
    scanning: string;
    checked: string;
    missing: string;
    deleted: string;
    trashedItems: string;
    taskIssues: string;
    notScannedTitle: string;
    notScannedDescription: string;
    healthyTitle: string;
    healthyDescription: string;
    databaseIntegrityFailedTitle: string;
    databaseIntegrityFailedDescription: string;
    missingTitle: string;
    selectionHint: string;
    selectAll: string;
    clearSelection: string;
    showPaths: string;
    hidePaths: string;
    localPath: string;
    lastChecked: string;
    safeCleanupTitle: string;
    safeCleanupDescription: string;
    removeSelected: string;
    removing: string;
    confirmTitle: string;
    confirmDescription: string;
    confirmRemove: string;
    cancelConfirmation: string;
    scanCompletedNotice: string;
    removedNotice: string;
    noneRemovedNotice: string;
    deletedTitle: string;
    deletedDescription: string;
    restoreDeleted: string;
    restoreUnavailable: string;
    restoredNotice: string;
    trashedTitle: string;
    trashedDescription: string;
    taskIssuesTitle: string;
    taskIssuesDescription: string;
    availableOutputs: string;
    executionStatus: string;
    executionSucceeded: string;
    healthUnavailable: string;
    confirmCatalogRestoreTitle: string;
    confirmCatalogRestoreDescription: string;
    confirmCatalogRestore: string;
    catalogRestoredNotice: string;
    removeSelectedTasks: string;
    removingTasks: string;
    confirmTaskCleanupTitle: string;
    confirmTaskCleanupDescription: string;
    confirmTaskRemove: string;
    tasksRemoved: string;
  };
}
