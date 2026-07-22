import { Call } from "@wailsio/runtime";

import type {
  LibraryBackupIDCommand,
  LibraryBackupManifest,
  LibraryBackupRestorePlan,
  LibraryBackupSummary,
  LibraryBackupVerification,
  LibraryImportBatch,
  LibraryImportBatchCommand,
  ListLibraryImportBatchesQuery,
  SelectLibraryImportCommand,
} from "@/shared/contracts/library-management";
import type {
  ClearMissingLibraryFilesResponse,
  ClearSelectedMissingLibraryFilesRequest,
  DatabaseIntegrityStatusDTO,
  DeleteOperationsRequest,
  LibraryMaintenanceSnapshotDTO,
  ListMissingLibraryFilesResponse,
  RestoreDeletedLibraryFilesRequest,
  RestoreDeletedLibraryFilesResponse,
} from "@/shared/contracts/library";
import type { CatalogItem } from "@/shared/contracts/catalog";
import {
  LIBRARY_CATALOG_ACTOR_ID,
  listCompleteCatalogItems,
  restoreCatalogItem,
} from "@/shared/query/catalog";

export const LIBRARY_IMPORT_HANDLER =
  "xiadown/internal/presentation/wails.LibraryImportHandler";
export const LIBRARY_BACKUP_HANDLER =
  "xiadown/internal/presentation/wails.LibraryBackupHandler";
export const LIBRARY_HANDLER =
  "xiadown/internal/presentation/wails.LibraryHandler";

type WailsWindow = Window & {
  _wails?: {
    dispatchWailsEvent?: unknown;
  };
};

export function isLibraryManagementRuntimeAvailable(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  return typeof (window as WailsWindow)._wails?.dispatchWailsEvent === "function";
}

/**
 * Rebuild the request at the Wails boundary. This prevents a structurally
 * compatible object with extra path-bearing fields from crossing the bridge.
 */
export function selectLibraryImportAndDryRun(
  request: SelectLibraryImportCommand,
): Promise<LibraryImportBatch> {
  const safeRequest: SelectLibraryImportCommand = {
    selectionKind: request.selectionKind,
    mode: request.mode,
    hiddenPolicy: request.hiddenPolicy,
    symlinkPolicy: request.symlinkPolicy,
  };
  const libraryId = request.libraryId?.trim();
  if (libraryId) {
    safeRequest.libraryId = libraryId;
  }
  return Call.ByName(
    `${LIBRARY_IMPORT_HANDLER}.SelectAndDryRun`,
    safeRequest,
  ) as Promise<LibraryImportBatch>;
}

export function getLibraryImportBatch(batchId: string): Promise<LibraryImportBatch> {
  const request: LibraryImportBatchCommand = { batchId: batchId.trim() };
  return Call.ByName(
    `${LIBRARY_IMPORT_HANDLER}.GetBatch`,
    request,
  ) as Promise<LibraryImportBatch>;
}

export function listLibraryImportBatches(limit = 50): Promise<LibraryImportBatch[]> {
  const normalizedLimit = Number.isFinite(limit) ? Math.trunc(limit) : 50;
  const request: ListLibraryImportBatchesQuery = {
    limit: Math.max(1, Math.min(200, normalizedLimit)),
  };
  return Call.ByName(
    `${LIBRARY_IMPORT_HANDLER}.ListBatches`,
    request,
  ) as Promise<LibraryImportBatch[]>;
}

export function commitLibraryImport(batchId: string): Promise<LibraryImportBatch> {
  const request: LibraryImportBatchCommand = { batchId: batchId.trim() };
  return Call.ByName(
    `${LIBRARY_IMPORT_HANDLER}.Commit`,
    request,
  ) as Promise<LibraryImportBatch>;
}

export function cancelLibraryImport(batchId: string): Promise<LibraryImportBatch> {
  const request: LibraryImportBatchCommand = { batchId: batchId.trim() };
  return Call.ByName(
    `${LIBRARY_IMPORT_HANDLER}.Cancel`,
    request,
  ) as Promise<LibraryImportBatch>;
}

export function resumeLibraryImport(batchId: string): Promise<LibraryImportBatch> {
  const request: LibraryImportBatchCommand = { batchId: batchId.trim() };
  return Call.ByName(
    `${LIBRARY_IMPORT_HANDLER}.Resume`,
    request,
  ) as Promise<LibraryImportBatch>;
}

export function createLibraryMetadataBackup(): Promise<LibraryBackupManifest> {
  return Call.ByName(
    `${LIBRARY_BACKUP_HANDLER}.CreateLibraryMetadataBackup`,
  ) as Promise<LibraryBackupManifest>;
}

export function listLibraryMetadataBackups(): Promise<LibraryBackupSummary[]> {
  return Call.ByName(
    `${LIBRARY_BACKUP_HANDLER}.ListLibraryMetadataBackups`,
  ) as Promise<LibraryBackupSummary[]>;
}

export function verifyLibraryMetadataBackup(
  backupId: string,
): Promise<LibraryBackupVerification> {
  const request: LibraryBackupIDCommand = { backupId: backupId.trim() };
  return Call.ByName(
    `${LIBRARY_BACKUP_HANDLER}.VerifyLibraryMetadataBackup`,
    request,
  ) as Promise<LibraryBackupVerification>;
}

export function planLibraryMetadataRestore(
  backupId: string,
): Promise<LibraryBackupRestorePlan> {
  const request: LibraryBackupIDCommand = { backupId: backupId.trim() };
  return Call.ByName(
    `${LIBRARY_BACKUP_HANDLER}.PlanLibraryMetadataRestore`,
    request,
  ) as Promise<LibraryBackupRestorePlan>;
}

export function getPendingLibraryMetadataRestore(): Promise<LibraryBackupRestorePlan | null> {
  return Call.ByName(
    `${LIBRARY_BACKUP_HANDLER}.GetPendingLibraryMetadataRestore`,
  ) as Promise<LibraryBackupRestorePlan | null>;
}

export function cancelPendingLibraryMetadataRestore(): Promise<void> {
  return Call.ByName(
    `${LIBRARY_BACKUP_HANDLER}.CancelPendingLibraryMetadataRestore`,
  ) as Promise<void>;
}

export function scanMissingLibraryFiles(): Promise<ListMissingLibraryFilesResponse> {
  return Call.ByName(
    `${LIBRARY_HANDLER}.ListMissingLibraryFiles`,
  ) as Promise<ListMissingLibraryFilesResponse>;
}

export interface LibraryMaintenanceSnapshot extends LibraryMaintenanceSnapshotDTO {
  trashedItems: CatalogItem[];
}

export function normalizeDatabaseIntegrityStatus(
  raw: DatabaseIntegrityStatusDTO | null | undefined,
): DatabaseIntegrityStatusDTO {
  const state = typeof raw?.state === "string" ? raw.state.trim() : "";
  return {
    state: new Set(["pending", "healthy", "failed", "unavailable"]).has(state)
      ? state
      : "unavailable",
    checkedAt: typeof raw?.checkedAt === "string" ? raw.checkedAt.trim() : undefined,
    detail: state === "failed" && typeof raw?.detail === "string"
      ? raw.detail.trim()
      : undefined,
  };
}

export async function getLibraryDatabaseIntegrityStatus(): Promise<DatabaseIntegrityStatusDTO> {
  const raw = await Call.ByName(
    `${LIBRARY_HANDLER}.GetDatabaseIntegrityStatus`,
  ) as DatabaseIntegrityStatusDTO;
  return normalizeDatabaseIntegrityStatus(raw);
}

export async function scanLibraryMaintenance(): Promise<LibraryMaintenanceSnapshot> {
  const [raw, trash] = await Promise.all([
    Call.ByName(
      `${LIBRARY_HANDLER}.GetLibraryMaintenanceSnapshot`,
    ) as Promise<LibraryMaintenanceSnapshotDTO>,
    listCompleteCatalogItems({ status: "trashed" }),
  ]);
  return {
    checkedFiles: Number.isFinite(raw?.checkedFiles) ? raw.checkedFiles : 0,
    missingFiles: Array.isArray(raw?.missingFiles) ? raw.missingFiles : [],
    deletedFiles: Array.isArray(raw?.deletedFiles) ? raw.deletedFiles : [],
    checkedTasks: Number.isFinite(raw?.checkedTasks) ? raw.checkedTasks : 0,
    taskIssues: Array.isArray(raw?.taskIssues) ? raw.taskIssues : [],
    databaseIntegrity: normalizeDatabaseIntegrityStatus(raw?.databaseIntegrity),
    trashedItems: Array.isArray(trash?.items) ? trash.items : [],
  };
}

export function restoreDeletedLibraryFiles(
  fileIds: readonly string[],
): Promise<RestoreDeletedLibraryFilesResponse> {
  const request: RestoreDeletedLibraryFilesRequest = {
    fileIds: Array.from(new Set(fileIds.map((fileId) => fileId.trim()).filter(Boolean))),
  };
  return Call.ByName(
    `${LIBRARY_HANDLER}.RestoreDeletedLibraryFiles`,
    request,
  ) as Promise<RestoreDeletedLibraryFilesResponse>;
}

export function restoreTrashedCatalogItem(
  item: Pick<CatalogItem, "id" | "revision">,
) {
  return restoreCatalogItem({
    id: item.id,
    expectedRevision: item.revision,
    actorId: LIBRARY_CATALOG_ACTOR_ID,
  });
}

export function deleteMaintenanceTasks(operationIds: readonly string[]): Promise<void> {
  const request: DeleteOperationsRequest = {
    operationIds: Array.from(new Set(
      operationIds.map((operationId) => operationId.trim()).filter(Boolean),
    )),
    // Maintenance removes stale task history only. It never cascades into
    // files, which may already be missing or retained as Catalog tombstones.
    cascadeFiles: false,
  };
  return Call.ByName(
    `${LIBRARY_HANDLER}.DeleteOperations`,
    request,
  ) as Promise<void>;
}

export function clearSelectedMissingLibraryFiles(
  fileIds: readonly string[],
): Promise<ClearMissingLibraryFilesResponse> {
  const request: ClearSelectedMissingLibraryFilesRequest = {
    fileIds: Array.from(new Set(fileIds.map((fileId) => fileId.trim()).filter(Boolean))),
  };
  return Call.ByName(
    `${LIBRARY_HANDLER}.ClearSelectedMissingLibraryFiles`,
    request,
  ) as Promise<ClearMissingLibraryFilesResponse>;
}
