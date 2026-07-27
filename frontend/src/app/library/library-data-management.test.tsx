import { describe, expect, mock, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type {
  LibraryDataManagementLabels,
  LibraryImportBatch,
  SelectLibraryImportCommand,
} from "@/shared/contracts/library-management";
import { LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY } from "@/shared/contracts/library-management";
import type {
  CatalogStorageRoot,
  CatalogStorageVolume,
} from "@/shared/contracts/catalog";

const calls: Array<{ name: string; args: unknown[] }> = [];

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByID: () => Promise.resolve(undefined),
    ByName: (name: string, ...args: unknown[]) => {
      calls.push({ name, args });
      if (name.endsWith(".ListBatches") || name.endsWith(".ListLibraryMetadataBackups")) {
        return Promise.resolve([]);
      }
      if (name.endsWith(".GetPendingLibraryMetadataRestore")) {
        return Promise.resolve(null);
      }
      return Promise.resolve({});
    },
  },
  Create: {
    Any: (value: unknown) => value,
    Array: (create: (value: unknown) => unknown) =>
      (values: unknown[]) => values.map(create),
    Nullable: (create: (value: unknown) => unknown) =>
      (value: unknown) => value == null ? value : create(value),
  },
  Events: {
    On: () => () => {},
    Types: { Common: { WindowFullscreen: "window-fullscreen", WindowUnFullscreen: "window-unfullscreen" } },
  },
  Window: { Fullscreen: () => Promise.resolve(), UnFullscreen: () => Promise.resolve() },
}));

const query = await import("@/shared/query/library-management");
const {
  LibraryBackupRestoreConfirmation,
  LibraryDataManagement,
  canCancelLibraryImport,
  canResumeLibraryImport,
  isLibraryRestoreConfirmationValid,
} = await import("./LibraryDataManagement");
const { buildCatalogStorageSummary } = await import(
  "./CatalogStorageSurfaces"
);

const labels: LibraryDataManagementLabels = {
  title: "Library data management",
  description: "Manage professional imports and private backups.",
  importTab: "Import",
  backupTab: "Backup",
  maintenanceTab: "Maintenance",
  unavailableTitle: "Unavailable",
  unavailableDescription: "This feature requires the XiaDown desktop runtime.",
  loading: "Loading",
  refresh: "Refresh",
  operationFailed: "Operation failed",
  unknownError: "Unknown error",
  closeNotice: "Close notice",
  bytes: "bytes",
  kilobytes: "KB",
  megabytes: "MB",
  gigabytes: "GB",
  import: {
    title: "Professional import",
    description: "Review sources before adding them to the library.",
    selectionKind: "Source selection",
    selectFiles: "Files",
    selectFolder: "Folder",
    mode: "Storage strategy",
    referencedMode: "Reference originals",
    referencedModeDescription: "Keep content in its current location.",
    copyMode: "Managed storage",
    copyModeDescription: "Let the library manage a separate file.",
    hiddenPolicy: "Hidden files",
    excludeHidden: "Exclude",
    includeHidden: "Include",
    symlinkPolicy: "Symbolic links",
    skipSymlinks: "Skip",
    followFileSymlinks: "Follow file links",
    chooseAndScan: "Choose and scan",
    choosingAndScanning: "Scanning",
    dryRunTitle: "Import preflight",
    dryRunDescription: "Confirm the totals before committing.",
    total: "Total",
    ready: "Ready",
    duplicate: "Duplicates",
    skipped: "Skipped",
    succeeded: "Succeeded",
    failed: "Failed",
    totalSize: "Total size",
    commit: "Commit import",
    committing: "Importing",
    resume: "Resume",
    resuming: "Resuming",
    cancel: "Cancel",
    cancelling: "Cancelling",
    batchHistory: "Import history",
    noBatches: "No import history",
    candidates: "Candidates",
    noCandidates: "No candidates",
    revealLocalPaths: "Reveal local paths",
    hideLocalPaths: "Hide local paths",
    localPathPrivacy: "Paths appear only on demand in this trusted local interface.",
    sourcePath: "Source path",
    managedPath: "Managed path",
    lastError: "Last error",
    updatedAt: "Updated",
    scanReadyNotice: "Preflight complete; review the totals.",
    commitCompletedNotice: "Import processing completed.",
    resumeCompletedNotice: "Import resumed.",
    cancelRequestedNotice: "Cancellation requested.",
    status: {
      scanning: "Scanning",
      ready: "Ready",
      running: "Running",
      cancelling: "Cancelling",
      cancelled: "Cancelled",
      completed: "Completed",
      failed: "Failed",
    },
    candidateStatus: {
      ready: "Ready",
      duplicate: "Duplicate",
      skipped: "Skipped",
      importing: "Importing",
      copied: "Stored",
      registered: "Registered",
      succeeded: "Succeeded",
      failed: "Failed",
      cancelled: "Cancelled",
    },
  },
  backup: {
    title: "Library backup",
    description: "Create and verify local metadata snapshots.",
    create: "Create backup",
    creating: "Creating",
    createCompletedNotice: "Backup created.",
    backups: "Backups",
    noBackups: "No backups",
    privateSnapshotTitle: "Private full metadata snapshot",
    privateSnapshotWarning: "This is a locally stored full XiaDown database metadata snapshot.",
    sensitiveMetadataWarning: "It may contain accounts, settings, credential hashes, and other sensitive application metadata.",
    noMediaWarning: "Media content is not included.",
    localPermissionsNote: "The local directory and files use account-only 0700/0600 permissions.",
    verify: "Verify",
    verifying: "Verifying",
    verificationValid: "Verification passed",
    verificationInvalid: "Verification failed",
    restore: "Restore",
    restoreConfirmationTitle: "Confirm scheduled restore",
    restoreConfirmationPrompt: "Enter",
    restoreConfirmationPhrase: "RESTORE LIBRARY",
    restoreConfirmationPlaceholder: "Enter the confirmation phrase",
    restoreNextLaunchWarning: "The restore takes effect the next time XiaDown launches.",
    restoreScopeWarning: "Only Library content is replaced; current accounts, settings, grants, and routes stay in place.",
    restoreRollbackWarning: "A rollback backup is created automatically before the restore is applied.",
    scheduleRestore: "Schedule next-launch restore",
    schedulingRestore: "Scheduling",
    cancelConfirmation: "Back",
    restoreScheduledNotice: "Restore scheduled for next launch.",
    pendingRestoreTitle: "Restore pending",
    pendingRestoreDescription: "This plan runs on the next launch.",
    cancelPendingRestore: "Cancel pending restore",
    cancellingPendingRestore: "Cancelling",
    pendingCancelledNotice: "Pending restore cancelled.",
    backupId: "Backup ID",
    createdAt: "Created",
    size: "Size",
    schemaVersion: "Schema version",
    applicationVersion: "Application version",
    state: "State",
  },
  maintenance: {
    title: "File integrity maintenance",
    description: "Check local file references.",
    scan: "Check missing files",
    scanning: "Checking",
    checked: "Files checked",
    missing: "Confirmed missing",
    deleted: "Deleted",
    trashedItems: "Trashed items",
    taskIssues: "Task issues",
    notScannedTitle: "Not checked",
    notScannedDescription: "Run a check first.",
    healthyTitle: "No missing files",
    healthyDescription: "All checked references are accessible.",
    databaseIntegrityFailedTitle: "Database integrity check failed",
    databaseIntegrityFailedDescription: "Review recovery options before making maintenance changes.",
    missingTitle: "Missing records",
    selectionHint: "Nothing is selected by default.",
    selectAll: "Select all",
    clearSelection: "Clear selection",
    showPaths: "Show paths",
    hidePaths: "Hide paths",
    localPath: "Local path",
    lastChecked: "Last checked",
    safeCleanupTitle: "Safe cleanup",
    safeCleanupDescription: "Only records are removed. Disk files are never deleted.",
    removeSelected: "Remove selected records",
    removing: "Removing",
    confirmTitle: "Confirm cleanup",
    confirmDescription: "Remove {count} records without deleting disk files.",
    confirmRemove: "Confirm cleanup",
    cancelConfirmation: "Cancel",
    scanCompletedNotice: "Checked {checked}; missing {missing}.",
    removedNotice: "Removed {count} records.",
    noneRemovedNotice: "No records removed.",
    deletedTitle: "Deleted file records",
    deletedDescription: "Deleted records can be restored only when their files exist.",
    restoreDeleted: "Restore",
    restoreUnavailable: "Local file unavailable",
    restoredNotice: "Restored {count} records.",
    trashedTitle: "Catalog trash",
    trashedDescription: "Trashed items are hidden from normal browsing.",
    taskIssuesTitle: "Tasks without available outputs",
    taskIssuesDescription: "Execution status is separate from output health.",
    availableOutputs: "Available outputs",
    executionStatus: "Execution status",
    executionSucceeded: "Succeeded",
    healthUnavailable: "Output unavailable",
    confirmCatalogRestoreTitle: "Restore catalog item?",
    confirmCatalogRestoreDescription: "The item will return to normal browsing.",
    confirmCatalogRestore: "Confirm restore",
    catalogRestoredNotice: "Catalog item restored.",
    removeSelectedTasks: "Remove selected tasks",
    removingTasks: "Removing tasks",
    confirmTaskCleanupTitle: "Confirm task cleanup",
    confirmTaskCleanupDescription: "Remove {count} task records without deleting files.",
    confirmTaskRemove: "Confirm task cleanup",
    tasksRemoved: "Removed {count} task records.",
  },
};

function batch(status: LibraryImportBatch["status"], lastErrorCode = ""): LibraryImportBatch {
  return {
    id: "batch-1",
    requestKey: "request-1",
    libraryId: "library-1",
    mode: "referenced",
    hiddenPolicy: "exclude",
    symlinkPolicy: "skip",
    status,
    counts: {
      total: 2,
      ready: 2,
      duplicate: 0,
      skipped: 0,
      succeeded: 0,
      failed: 0,
      totalBytes: 2_048,
    },
    lastErrorCode,
    cancelRequested: false,
    createdAt: "2026-07-13T10:00:00Z",
    updatedAt: "2026-07-13T10:00:00Z",
  };
}

describe("library management Wails boundary", () => {
  test("SelectAndDryRun strips every path-bearing extra field", async () => {
    calls.length = 0;
    const unsafeRequest = {
      selectionKind: "directory",
      mode: LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY,
      libraryId: "  library-1  ",
      hiddenPolicy: "exclude",
      symlinkPolicy: "follow_files",
      sourcePath: "/private/source",
      sourcePaths: ["/private/source"],
      managedRoot: "/private/managed",
      path: "/private/other",
    } as unknown as SelectLibraryImportCommand;

    await query.selectLibraryImportAndDryRun(unsafeRequest);

    expect(calls).toHaveLength(1);
    expect(calls[0]?.name).toBe(`${query.LIBRARY_IMPORT_HANDLER}.SelectAndDryRun`);
    expect(calls[0]?.args[0]).toEqual({
      selectionKind: "directory",
      mode: LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY,
      libraryId: "library-1",
      hiddenPolicy: "exclude",
      symlinkPolicy: "follow_files",
    });
    expect(JSON.stringify(calls[0]?.args[0]).toLowerCase()).not.toContain("path");
    expect(JSON.stringify(calls[0]?.args[0])).not.toContain("/private");
  });

  test("Get, List, Commit, Cancel, and Resume use the dedicated import handler", async () => {
    calls.length = 0;
    await query.getLibraryImportBatch(" batch-1 ");
    await query.listLibraryImportBatches(500);
    await query.commitLibraryImport(" batch-1 ");
    await query.cancelLibraryImport(" batch-1 ");
    await query.resumeLibraryImport(" batch-1 ");

    expect(calls.map((call) => call.name)).toEqual([
      `${query.LIBRARY_IMPORT_HANDLER}.GetBatch`,
      `${query.LIBRARY_IMPORT_HANDLER}.ListBatches`,
      `${query.LIBRARY_IMPORT_HANDLER}.Commit`,
      `${query.LIBRARY_IMPORT_HANDLER}.Cancel`,
      `${query.LIBRARY_IMPORT_HANDLER}.Resume`,
    ]);
    expect(calls[0]?.args[0]).toEqual({ batchId: "batch-1" });
    expect(calls[1]?.args[0]).toEqual({ limit: 200 });
  });

  test("backup administration covers create, list, verify, plan, pending, and cancel", async () => {
    calls.length = 0;
    await query.createLibraryMetadataBackup();
    await query.listLibraryMetadataBackups();
    await query.verifyLibraryMetadataBackup(" backup-1 ");
    await query.planLibraryMetadataRestore(" backup-1 ");
    await query.getPendingLibraryMetadataRestore();
    await query.cancelPendingLibraryMetadataRestore();

    expect(calls.map((call) => call.name)).toEqual([
      `${query.LIBRARY_BACKUP_HANDLER}.CreateLibraryMetadataBackup`,
      `${query.LIBRARY_BACKUP_HANDLER}.ListLibraryMetadataBackups`,
      `${query.LIBRARY_BACKUP_HANDLER}.VerifyLibraryMetadataBackup`,
      `${query.LIBRARY_BACKUP_HANDLER}.PlanLibraryMetadataRestore`,
      `${query.LIBRARY_BACKUP_HANDLER}.GetPendingLibraryMetadataRestore`,
      `${query.LIBRARY_BACKUP_HANDLER}.CancelPendingLibraryMetadataRestore`,
    ]);
    expect(calls[2]?.args[0]).toEqual({ backupId: "backup-1" });
    expect(calls[3]?.args[0]).toEqual({ backupId: "backup-1" });
  });

  test("maintenance scans and clears only explicit file ids through LibraryHandler", async () => {
    calls.length = 0;
    await query.scanMissingLibraryFiles();
    await query.clearSelectedMissingLibraryFiles([
      " file-1 ",
      "file-1",
      "",
      "file-2",
    ]);

    expect(calls.map((call) => call.name)).toEqual([
      `${query.LIBRARY_HANDLER}.ListMissingLibraryFiles`,
      `${query.LIBRARY_HANDLER}.ClearSelectedMissingLibraryFiles`,
    ]);
    expect(calls[1]?.args[0]).toEqual({ fileIds: ["file-1", "file-2"] });
  });

  test("database integrity health uses the lightweight Library maintenance query", async () => {
    calls.length = 0;
    const status = await query.getLibraryDatabaseIntegrityStatus();

    expect(calls.map((call) => call.name)).toEqual([
      `${query.LIBRARY_HANDLER}.GetDatabaseIntegrityStatus`,
    ]);
    expect(status).toEqual({
      state: "unavailable",
      checkedAt: undefined,
      detail: undefined,
    });
    expect(query.normalizeDatabaseIntegrityStatus({
      state: "failed",
      checkedAt: " 2026-07-20T08:00:00Z ",
      detail: " structural error ",
    })).toEqual({
      state: "failed",
      checkedAt: "2026-07-20T08:00:00Z",
      detail: "structural error",
    });
  });

  test("maintenance restore and task cleanup keep Catalog and files recoverable", async () => {
    calls.length = 0;
    await query.scanLibraryMaintenance();
    await query.restoreDeletedLibraryFiles([" file-1 ", "file-1", ""]);
    await query.restoreTrashedCatalogItem({ id: "item-1", revision: 7 });
    await query.deleteMaintenanceTasks([" task-1 ", "task-1", "task-2"]);

    expect(calls.map((call) => call.name)).toEqual([
      `${query.LIBRARY_HANDLER}.GetLibraryMaintenanceSnapshot`,
      "xiadown/internal/presentation/wails.CatalogHandler.ListCatalogItems",
      `${query.LIBRARY_HANDLER}.RestoreDeletedLibraryFiles`,
      "xiadown/internal/presentation/wails.CatalogHandler.RestoreCatalogItem",
      `${query.LIBRARY_HANDLER}.DeleteOperations`,
    ]);
    expect(calls[2]?.args[0]).toEqual({ fileIds: ["file-1"] });
    expect(calls[3]?.args[0]).toEqual({
      id: "item-1",
      expectedRevision: 7,
      actorId: "desktop-library",
    });
    expect(calls[4]?.args[0]).toEqual({
      operationIds: ["task-1", "task-2"],
      cascadeFiles: false,
    });
  });
});

describe("LibraryDataManagement", () => {
  test("deduplicates roots that share a physical volume in the storage overview", () => {
    const root = (
      id: string,
      overrides: Partial<CatalogStorageRoot> = {},
    ): CatalogStorageRoot => ({
      id,
      catalogId: "catalog",
      name: id,
      path: `/Volumes/Creator/${id}`,
      locationPath: `/Volumes/Creator/${id}`,
      volumeId: "creator-volume",
      mode: "referenced",
      isDefault: false,
      status: "online",
      fileCount: 1,
      assetCount: 1,
      videoCount: 1,
      audioCount: 0,
      sizeBytes: 100,
      totalBytes: 1_000,
      availableBytes: 400,
      createdAt: "2026-07-20T08:00:00Z",
      updatedAt: "2026-07-20T08:00:00Z",
      ...overrides,
    });
    const volumes: CatalogStorageVolume[] = [{
      id: "creator-volume",
      name: "Creator",
      mountPath: "/Volumes/Creator",
      readOnly: false,
      totalBytes: 1_000,
      availableBytes: 400,
    }];
    const summary = buildCatalogStorageSummary(
      volumes,
      [
        root("root-a"),
        root("root-b", { sizeBytes: 200 }),
        root("offline", {
          volumeId: "archive-volume",
          status: "offline",
          totalBytes: 2_000,
          availableBytes: 1_500,
        }),
      ],
    );

    expect(summary.totalBytes).toBe(1_000);
    expect(summary.availableBytes).toBe(400);
    expect(summary.libraryBytes).toBe(400);
    expect(summary.mountedLibraryBytes).toBe(300);
    expect(summary.assetCount).toBe(3);
    expect(summary.fileCount).toBe(3);
    expect(summary.otherBytes).toBe(300);
    expect(summary.volumes).toHaveLength(1);
    expect(summary.volumes[0]?.fileCount).toBe(2);
    expect(summary.volumes[0]?.videoCount).toBe(2);
    expect(summary.volumes[0]?.audioCount).toBe(0);
    expect(summary.mountedRootCount).toBe(2);
    expect(summary.offlineRootCount).toBe(1);
  });

  test("retains mounted volumes that do not contain storage roots", () => {
    const summary = buildCatalogStorageSummary(
      [{
        id: "system-volume",
        mountPath: "/",
        readOnly: false,
        totalBytes: 2_000,
        availableBytes: 500,
      }],
      [],
    );

    expect(summary.volumes).toHaveLength(1);
    expect(summary.volumes[0]?.rootCount).toBe(0);
    expect(summary.volumes[0]?.libraryBytes).toBe(0);
    expect(summary.totalBytes).toBe(2_000);
    expect(summary.otherBytes).toBe(1_500);
  });

  test("folds an APFS Data root into the user-facing system volume", () => {
    const summary = buildCatalogStorageSummary(
      [{
        id: "darwin-fsid-system",
        name: "",
        mountPath: "/",
        readOnly: true,
        totalBytes: 2_000,
        availableBytes: 500,
      }],
      [{
        id: "downloads",
        catalogId: "catalog",
        name: "XiaDown Downloads",
        path: "/Users/arnold/Downloads/xiadown",
        locationPath: "/Users/arnold/Downloads",
        volumeId: "darwin-fsid-data",
        mode: "managed",
        isDefault: true,
        status: "online",
        fileCount: 4,
        assetCount: 3,
        videoCount: 2,
        audioCount: 1,
        sizeBytes: 300,
        totalBytes: 2_000,
        availableBytes: 500,
        createdAt: "2026-07-20T08:00:00Z",
        updatedAt: "2026-07-20T08:00:00Z",
      }],
    );

    expect(summary.volumes).toHaveLength(1);
    expect(summary.volumes[0]?.id).toBe("darwin-fsid-system");
    expect(summary.volumes[0]?.rootCount).toBe(1);
    expect(summary.volumes[0]?.libraryBytes).toBe(300);
    expect(summary.totalBytes).toBe(2_000);
  });

  test("keeps storage roots at the overview width with one card per row", async () => {
    const [layoutCss, dreamStorageCss, dialogSource] = await Promise.all([
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/storage.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./CatalogManagementDialog.tsx", import.meta.url)).text(),
    ]);

    expect(layoutCss).toMatch(
      /\.app-catalog-management\s*\{[^}]*width:\s*min\(50rem,/s,
    );
    expect(dreamStorageCss).toMatch(
      /\.app-dream-storage-root-grid\s*\{[^}]*grid-template-columns:\s*1fr;/s,
    );
    expect(layoutCss).not.toContain("app-catalog-management--storage");
    expect(dialogSource).not.toContain("app-catalog-management--storage");
    expect(dreamStorageCss).toMatch(
      /\.app-dream-storage-volume__facts\s*\{[^}]*grid-template-columns:\s*repeat\(4,/s,
    );
  });

  test("uses reachable container breakpoints inside the management dialog", async () => {
    const css = await Bun.file(
      new URL("./LibraryDataManagement.css", import.meta.url),
    ).text();

    expect(css).toMatch(
      /\.library-data-management\s*\{[^}]*container: library-data-management \/ inline-size;/s,
    );
    expect(css).toContain(
      "@container library-data-management (max-width: 52rem)",
    );
    expect(css).toContain(
      "@container library-data-management (max-width: 40rem)",
    );
    expect(css).not.toContain("max-width: 980px");
    expect(css).not.toContain("max-width: 640px");
  });

  test("keeps the dialog body as the only vertical scroll owner", async () => {
    const css = await Bun.file(
      new URL("./LibraryDataManagement.css", import.meta.url),
    ).text();
    const nestedListRules = [
      ...css.matchAll(
        /\.library-data-management__candidates\s*\{(?<body>[^}]*)\}/g,
      ),
      ...css.matchAll(
        /\.library-data-management__history\s*\{(?<body>[^}]*)\}/g,
      ),
    ];

    expect(nestedListRules.length).toBeGreaterThan(0);
    for (const rule of nestedListRules) {
      expect(rule.groups?.body ?? "").not.toMatch(/\bmax-height\s*:/);
      expect(rule.groups?.body ?? "").not.toMatch(/\boverflow(?:-y)?\s*:\s*auto\b/);
    }
  });

  test("embedded mode removes the repeated page title and exposes one toolbar", () => {
    const markup = renderToStaticMarkup(
      <LibraryDataManagement labels={labels} runtimeAvailable embedded />,
    );
    const toolbars = markup.match(/library-data-management__toolbar/g) ?? [];

    expect(markup).toContain("data-library-data-management");
    expect(markup).toContain('data-embedded="true"');
    expect(markup).not.toContain(`<h2>${labels.title}</h2>`);
    expect(markup).not.toContain("library-data-management__header");
    expect(markup).toContain('role="toolbar"');
    expect(toolbars).toHaveLength(1);
  });

  test("shows unavailable without a Wails desktop bridge", () => {
    const markup = renderToStaticMarkup(
      <LibraryDataManagement labels={labels} runtimeAvailable={false} />,
    );
    expect(markup).toContain("data-library-management-unavailable");
    expect(markup).toContain(labels.unavailableTitle);
    expect(markup).toContain(labels.unavailableDescription);
  });

  test("always explains metadata-only privacy and local 0600 protection", () => {
    const markup = renderToStaticMarkup(
      <LibraryDataManagement
        labels={labels}
        runtimeAvailable
        initialSection="backup"
      />,
    );
    expect(markup).toContain("data-library-backup-privacy");
    expect(markup).toContain(labels.backup.privateSnapshotWarning);
    expect(markup).toContain(labels.backup.sensitiveMetadataWarning);
    expect(markup).toContain(labels.backup.noMediaWarning);
    expect(markup).toContain("0600");
  });

  test("maintenance starts read-only and explains that disk files are never deleted", () => {
    const markup = renderToStaticMarkup(
      <LibraryDataManagement
        labels={labels}
        runtimeAvailable
        initialSection="maintenance"
      />,
    );

    expect(markup).toContain(labels.maintenance.title);
    expect(markup).toContain(labels.maintenance.notScannedTitle);
    expect(markup).toContain(labels.maintenance.safeCleanupDescription);
    expect(markup).not.toContain(labels.maintenance.removeSelected);
    expect(markup).not.toContain('type="checkbox"');
  });

  test("maintenance UI separates task health and provides confirmed recovery actions", async () => {
    const source = await Bun.file(
      new URL("./LibraryDataManagement.tsx", import.meta.url),
    ).text();

    expect(source).toContain('data-maintenance-kind="deleted-files"');
    expect(source).toContain('data-maintenance-kind="trashed-items"');
    expect(source).toContain('data-maintenance-kind="task-issues"');
    expect(source).toContain("labels.maintenance.healthUnavailable");
    expect(source).toContain("labels.maintenance.executionStatus");
    expect(source).toContain("restoreTrashedCatalogItem(item)");
    expect(source).toContain("deleteMaintenanceTasks(selectedTaskIds)");
    expect(source).toContain("labels.maintenance.confirmTaskCleanupTitle");
    expect(source).toContain("props.onMaintenanceChanged?.()");
    expect(source).toContain('data-state="danger"');
    expect(source).toContain("labels.maintenance.databaseIntegrityFailedTitle");
  });

  test("catalog management localizes defaults and refreshes Library caches", async () => {
    const [source, storageSource] = await Promise.all([
      Bun.file(new URL("./CatalogManagementDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./CatalogStorageSurfaces.tsx", import.meta.url)).text(),
    ]);

    expect(source).toContain("? labels.library");
    expect(source).toContain("formatBytes(overview.data.totalSizeBytes)");
    expect(source).not.toContain("useCatalogMigrationAudit");
    expect(source).toContain("labels.statusLabel(overview.data.catalog.status)");
    expect(source).toContain("categoryLabel={labels.operationKindLabel}");
    expect(source).toContain("queryClient.invalidateQueries({ queryKey: catalogKeys.all })");
    expect(source).toContain('queryClient.invalidateQueries({ queryKey: ["library"] })');
    expect(source).toContain("<CatalogStorageOverview");
    expect(source).toContain("volumes={volumes.data ?? []}");
    expect(source).toContain("metrics={categoryCards}");
    expect(source).not.toContain("labels.itemStatuses");
    expect(source).not.toContain("labels.catalogHealth");
    expect(source).toContain("<CatalogStorageRootCard");
    expect(storageSource).toContain("root.isDefault");
    expect(storageSource).toContain("root.fileCount");
    expect(storageSource).toContain("root.assetCount");
    expect(storageSource).toContain("root.videoCount");
    expect(storageSource).toContain("root.audioCount");
    expect(storageSource).toContain("root.totalBytes");
    expect(storageSource).toContain("root.volumeId?.trim() || root.id");
    expect(storageSource).toContain("props.labels.noRootsOnVolume");
    expect(storageSource).toContain("<StorageCapacityBar");
    expect(storageSource).toContain(
      "app-dream-storage-overview__library-metrics",
    );
    expect(storageSource).toContain(
      "app-dream-storage-overview__capacity-section",
    );
    const rootCardSource = storageSource.slice(
      storageSource.indexOf("export function CatalogStorageRootCard"),
    );
    expect(rootCardSource).toContain("root.locationPath || root.path");
    expect(rootCardSource).not.toContain("root.totalBytes");
    expect(rootCardSource).not.toContain("root.availableBytes");
    expect(rootCardSource).not.toContain("<StorageCapacityBar");
    expect(rootCardSource).toContain("labels.rootDirectorySize");
    expect(rootCardSource).not.toContain("onBeginEdit");
    expect(rootCardSource).not.toContain("onSetDefault");
    expect(rootCardSource).toContain("labels.relocateManagedRoot");
    expect(rootCardSource).toContain("labels.replaceReferencedRoot");
    expect(rootCardSource).toContain("labels.scanRoot");
    expect(rootCardSource).toContain("labels.cancelRootScan");
    expect(rootCardSource).toContain(
      "app-dream-storage-root-card__sync",
    );
    expect(rootCardSource).toContain("<progress");
    expect(rootCardSource).toContain("onSelect={props.onRemove}");
    expect(source).toContain("useCatalogStorageRootSyncStates");
    expect(source).toContain("useStartCatalogStorageRootScan");
    expect(source).toContain("useCancelCatalogStorageRootScan");
    expect(source).toContain("app-catalog-root-remove-dialog");
    expect(source).not.toContain("window.confirm");
    expect(source).not.toContain("app-catalog-roots__add");
    expect(source).toContain("useUpdateCatalogStorageRoot");
    expect(source).toContain("messageBus.publishToast");
    expect(storageSource).toContain('DropdownMenuContent align="center"');
    expect(storageSource).toContain("CatalogStorageRootEmojiPicker");
    expect(storageSource).toContain("React.lazy");
    expect(storageSource).toContain(
      `closest<HTMLElement>('[role="dialog"]')`,
    );
    expect(storageSource).toContain(
      "portalContainer={emojiPortalContainer}",
    );
    expect(storageSource).not.toContain("syncState.lastError");
    expect(source).not.toContain("useSetDefaultCatalogStorageRoot");
    expect(source).toContain("selectRoot.mutate({})");
    expect(source).toContain("useRemoveCatalogStorageRoot");
    expect(source).toContain("useRelocateCatalogStorageRoot");
    expect(source).toContain("openCatalogStorageRoot");
  });

  test("next-launch restore requires the exact second confirmation and promises rollback", () => {
    const unconfirmed = renderToStaticMarkup(
      <LibraryBackupRestoreConfirmation
        labels={labels.backup}
        value="RESTORE"
        busy={false}
        disabled={false}
        onValueChange={() => {}}
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );
    const confirmed = renderToStaticMarkup(
      <LibraryBackupRestoreConfirmation
        labels={labels.backup}
        value={labels.backup.restoreConfirmationPhrase}
        busy={false}
        disabled={false}
        onValueChange={() => {}}
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );

    expect(unconfirmed).toContain("data-next-launch-restore-confirmation");
    expect(unconfirmed).toContain(labels.backup.restoreNextLaunchWarning);
    expect(unconfirmed).toContain(labels.backup.restoreScopeWarning);
    expect(unconfirmed).toContain(labels.backup.restoreRollbackWarning);
    expect(unconfirmed).toContain('type="button" disabled=""');
    expect(confirmed).not.toContain('type="button" disabled=""');
    expect(isLibraryRestoreConfirmationValid("RESTORE", labels.backup.restoreConfirmationPhrase)).toBeFalse();
    expect(isLibraryRestoreConfirmationValid(
      labels.backup.restoreConfirmationPhrase,
      labels.backup.restoreConfirmationPhrase,
    )).toBeTrue();
  });

  test("running, failed, and cancelled batches can resume while active batches can cancel", () => {
    expect(canResumeLibraryImport(batch("running"))).toBeTrue();
    expect(canResumeLibraryImport(batch("failed"))).toBeTrue();
    expect(canResumeLibraryImport(batch("cancelled"))).toBeTrue();
    expect(canResumeLibraryImport(batch("failed", "scan_failed"))).toBeFalse();
    expect(canCancelLibraryImport(batch("ready"))).toBeTrue();
    expect(canCancelLibraryImport(batch("running"))).toBeTrue();
    expect(canCancelLibraryImport({
      ...batch("running"),
      cancelRequested: true,
    })).toBeFalse();
    expect(canCancelLibraryImport(batch("completed"))).toBeFalse();
  });

  test("uses one roving tab stop for the active management section", () => {
    const markup = renderToStaticMarkup(
      <LibraryDataManagement
        labels={labels}
        runtimeAvailable
        initialSection="backup"
      />,
    );
    const tabs = markup.match(/<button[^>]*role="tab"[^>]*>/g) ?? [];

    expect(tabs).toHaveLength(3);
    expect(tabs.filter((tab) => tab.includes('tabindex="0"'))).toHaveLength(1);
    expect(tabs.filter((tab) => tab.includes('tabindex="-1"'))).toHaveLength(2);
    expect(tabs.find((tab) => tab.includes('aria-selected="true"'))).toContain(
      'tabindex="0"',
    );
  });
});
