import {
  ArchiveRestore,
  Check,
  CheckCircle2,
  CircleAlert,
  DatabaseBackup,
  Eye,
  EyeOff,
  FileSearch,
  FolderOpen,
  ListChecks,
  Loader2,
  RefreshCcw,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
  Square,
  Trash2,
  Wrench,
  X,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import type {
  LibraryBackupRestorePlan,
  LibraryBackupSummary,
  LibraryBackupVerification,
  LibraryDataManagementLabels,
  LibraryImportBatch,
  LibraryImportBatchStatus,
  LibraryImportCandidateStatus,
  LibraryImportHiddenPolicy,
  LibraryImportStorageStrategy,
  LibraryImportSelectionKind,
  LibraryImportSymlinkPolicy,
} from "@/shared/contracts/library-management";
import { LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY } from "@/shared/contracts/library-management";
import type {
  CatalogItem,
} from "@/shared/contracts/catalog";
import type {
  DatabaseIntegrityStatusDTO,
  MissingLibraryFileDTO,
} from "@/shared/contracts/library";
import {
  cancelLibraryImport,
  cancelPendingLibraryMetadataRestore,
  clearSelectedMissingLibraryFiles,
  commitLibraryImport,
  createLibraryMetadataBackup,
  deleteMaintenanceTasks,
  getLibraryImportBatch,
  getLibraryDatabaseIntegrityStatus,
  getPendingLibraryMetadataRestore,
  isLibraryManagementRuntimeAvailable,
  listLibraryImportBatches,
  listLibraryMetadataBackups,
  planLibraryMetadataRestore,
  restoreDeletedLibraryFiles,
  restoreTrashedCatalogItem,
  resumeLibraryImport,
  scanLibraryMaintenance,
  selectLibraryImportAndDryRun,
  verifyLibraryMetadataBackup,
  type LibraryMaintenanceSnapshot,
} from "@/shared/query/library-management";
import { Button } from "@/shared/ui/button";
import { Select } from "@/shared/ui/select";
import { useRovingTabs } from "@/shared/ui/roving-tabs";
import {
  StatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";

import {
  LibraryImportOperationController,
  reconcileLibraryImportBatch,
  resolveLibraryImportResultNotice,
  type LibraryImportOperationToken,
} from "./library-import-operation";

import "./LibraryDataManagement.css";

export type LibraryDataManagementSection = "import" | "backup" | "maintenance";

export interface LibraryDataManagementProps {
  labels: LibraryDataManagementLabels;
  libraryId?: string;
  className?: string;
  initialSection?: LibraryDataManagementSection;
  embedded?: boolean;
  /** Test/host override; normal application code should rely on bridge detection. */
  runtimeAvailable?: boolean;
  /** Refresh host-level Catalog and Library caches after maintenance mutations. */
  onMaintenanceChanged?: () => void | Promise<void>;
  /** Localize Catalog categories owned by the host workspace. */
  categoryLabel?: (value: string) => string;
}

type BusyAction =
  | "refresh"
  | "scan"
  | "commit"
  | "resume"
  | "cancel"
  | "create-backup"
  | "verify-backup"
  | "plan-restore"
  | "cancel-restore"
  | "scan-missing"
  | "clear-missing"
  | "restore-deleted"
  | "restore-trashed"
  | "clear-tasks"
  | null;

interface InlineNotice {
  tone: "success" | "error";
  message: string;
}

const POLL_INTERVAL_MS = 2_000;
const POLLED_IMPORT_STATUSES: ReadonlySet<LibraryImportBatchStatus> = new Set([
  "scanning",
  "running",
  "cancelling",
]);

export function canResumeLibraryImport(batch: LibraryImportBatch): boolean {
  if (batch.status === "failed" && batch.lastErrorCode === "scan_failed") {
    return false;
  }
  return batch.status === "failed"
    || batch.status === "cancelled"
    || batch.status === "running"
    || batch.status === "cancelling";
}

export function canCancelLibraryImport(batch: LibraryImportBatch): boolean {
  return !batch.cancelRequested
    && (batch.status === "ready" || batch.status === "running");
}

export function isLibraryRestoreConfirmationValid(value: string, phrase: string): boolean {
  return phrase.length > 0 && value === phrase;
}

function formatBytes(
  value: number,
  labels: Pick<
    LibraryDataManagementLabels,
    "bytes" | "kilobytes" | "megabytes" | "gigabytes"
  >,
): string {
  const safeValue = Number.isFinite(value) && value > 0 ? value : 0;
  if (safeValue < 1_024) {
    return `${Math.round(safeValue)} ${labels.bytes}`;
  }
  if (safeValue < 1_048_576) {
    return `${(safeValue / 1_024).toFixed(1)} ${labels.kilobytes}`;
  }
  if (safeValue < 1_073_741_824) {
    return `${(safeValue / 1_048_576).toFixed(1)} ${labels.megabytes}`;
  }
  return `${(safeValue / 1_073_741_824).toFixed(1)} ${labels.gigabytes}`;
}

function formatTimestamp(value: string): string {
  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) {
    return value;
  }
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(timestamp);
}

function formatCountTemplate(template: string, values: Record<string, number>): string {
  return Object.entries(values).reduce(
    (message, [key, value]) => message.split(`{${key}}`).join(String(value)),
    template,
  );
}

function operationError(error: unknown, labels: LibraryDataManagementLabels): InlineNotice {
  const detail = error instanceof Error && error.message.trim()
    ? error.message.trim()
    : labels.unknownError;
  return { tone: "error", message: `${labels.operationFailed}: ${detail}` };
}

function ImportStatusBadge(props: {
  status: LibraryImportBatchStatus;
  labels: LibraryDataManagementLabels["import"]["status"];
}) {
  return (
    <StatusBadge
      className="library-data-management__status"
      data-status={props.status}
      tone={resolveImportBatchStatusTone(props.status)}
    >
      {props.labels[props.status]}
    </StatusBadge>
  );
}

function resolveImportBatchStatusTone(
  status: LibraryImportBatchStatus,
): DreamStatusTone {
  if (status === "failed") {
    return "danger";
  }
  if (
    status === "running" ||
    status === "scanning" ||
    status === "cancelling"
  ) {
    return "busy";
  }
  if (status === "ready" || status === "completed") {
    return "success";
  }
  return "neutral";
}

function resolveImportCandidateStatusTone(
  status: LibraryImportCandidateStatus,
): DreamStatusTone {
  if (status === "failed") {
    return "danger";
  }
  if (status === "importing") {
    return "busy";
  }
  if (status === "succeeded") {
    return "success";
  }
  return "neutral";
}

function CountCard(props: { label: string; value: React.ReactNode }) {
  return (
    <div className="library-data-management__count-card">
      <span>{props.label}</span>
      <strong>{props.value}</strong>
    </div>
  );
}

export interface LibraryBackupRestoreConfirmationProps {
  labels: LibraryDataManagementLabels["backup"];
  value: string;
  busy: boolean;
  disabled: boolean;
  onValueChange: (value: string) => void;
  onConfirm: () => void;
  onCancel: () => void;
}

export function LibraryBackupRestoreConfirmation(
  props: LibraryBackupRestoreConfirmationProps,
) {
  const titleId = React.useId();
  const confirmed = isLibraryRestoreConfirmationValid(
    props.value,
    props.labels.restoreConfirmationPhrase,
  );
  return (
    <div
      className="library-data-management__restore-confirmation"
      role="alertdialog"
      aria-modal="false"
      aria-labelledby={titleId}
      data-next-launch-restore-confirmation
    >
      <h5 id={titleId}>{props.labels.restoreConfirmationTitle}</h5>
      <p>{props.labels.restoreNextLaunchWarning}</p>
      <p>{props.labels.restoreScopeWarning}</p>
      <p>{props.labels.restoreRollbackWarning}</p>
      <label>
        <span>
          {props.labels.restoreConfirmationPrompt}{" "}
          <strong>{props.labels.restoreConfirmationPhrase}</strong>
        </span>
        <input
          value={props.value}
          placeholder={props.labels.restoreConfirmationPlaceholder}
          onChange={(event) => props.onValueChange(event.target.value)}
          autoComplete="off"
          spellCheck={false}
        />
      </label>
      <div>
        <Button
          type="button"
          variant="outline"
          disabled={props.disabled || props.busy}
          onClick={props.onCancel}
        >
          {props.labels.cancelConfirmation}
        </Button>
        <Button
          type="button"
          variant="destructive"
          disabled={props.disabled || props.busy || !confirmed}
          onClick={props.onConfirm}
        >
          {props.busy
            ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
            : <ArchiveRestore className="h-4 w-4" aria-hidden="true" />}
          {props.busy ? props.labels.schedulingRestore : props.labels.scheduleRestore}
        </Button>
      </div>
    </div>
  );
}

export function LibraryDataManagement(props: LibraryDataManagementProps) {
  const { labels } = props;
  const tabsId = React.useId();
  const importTabId = `${tabsId}-import-tab`;
  const importPanelId = `${tabsId}-import-panel`;
  const backupTabId = `${tabsId}-backup-tab`;
  const backupPanelId = `${tabsId}-backup-panel`;
  const maintenanceTabId = `${tabsId}-maintenance-tab`;
  const maintenancePanelId = `${tabsId}-maintenance-panel`;
  const labelsRef = React.useRef(labels);
  labelsRef.current = labels;
  const available = props.runtimeAvailable ?? isLibraryManagementRuntimeAvailable();
  const [section, setSection] = React.useState<LibraryDataManagementSection>(
    props.initialSection ?? "import",
  );
  const [busy, setBusy] = React.useState<BusyAction>(null);
  const [cancelInFlight, setCancelInFlight] = React.useState(false);
  const [notice, setNotice] = React.useState<InlineNotice | null>(null);
  const [initialLoading, setInitialLoading] = React.useState(
    () => available && typeof window !== "undefined",
  );

  const [selectionKind, setSelectionKind] = React.useState<LibraryImportSelectionKind>("files");
  const [mode, setMode] = React.useState<LibraryImportStorageStrategy>("referenced");
  const [hiddenPolicy, setHiddenPolicy] = React.useState<LibraryImportHiddenPolicy>("exclude");
  const [symlinkPolicy, setSymlinkPolicy] = React.useState<LibraryImportSymlinkPolicy>("skip");
  const [batches, setBatches] = React.useState<LibraryImportBatch[]>([]);
  const [selectedBatch, setSelectedBatch] = React.useState<LibraryImportBatch | null>(null);
  const selectedBatchRef = React.useRef<LibraryImportBatch | null>(null);
  selectedBatchRef.current = selectedBatch;
  const [showCandidatePaths, setShowCandidatePaths] = React.useState(false);
  const importOperationsRef = React.useRef<LibraryImportOperationController>();
  if (!importOperationsRef.current) {
    importOperationsRef.current = new LibraryImportOperationController();
  }

  const [backups, setBackups] = React.useState<LibraryBackupSummary[]>([]);
  const [pendingRestore, setPendingRestore] = React.useState<LibraryBackupRestorePlan | null>(null);
  const [verifications, setVerifications] = React.useState<
    Record<string, LibraryBackupVerification>
  >({});
  const [restoreTarget, setRestoreTarget] = React.useState<string | null>(null);
  const [restoreConfirmation, setRestoreConfirmation] = React.useState("");

  const [maintenanceResult, setMaintenanceResult] = React.useState<
    LibraryMaintenanceSnapshot | null
  >(null);
  const [databaseIntegrity, setDatabaseIntegrity] = React.useState<
    DatabaseIntegrityStatusDTO | null
  >(null);
  const [restoringDeletedFileId, setRestoringDeletedFileId] = React.useState<string | null>(null);
  const [restoreTrashedConfirmingId, setRestoreTrashedConfirmingId] = React.useState<string | null>(null);
  const [selectedMissingFileIds, setSelectedMissingFileIds] = React.useState<string[]>([]);
  const [selectedTaskIds, setSelectedTaskIds] = React.useState<string[]>([]);
  const [showMissingPaths, setShowMissingPaths] = React.useState(false);
  const [cleanupConfirming, setCleanupConfirming] = React.useState(false);
  const [taskCleanupConfirming, setTaskCleanupConfirming] = React.useState(false);
  const managementTabs = [
    {
      value: "import" as const,
      id: importTabId,
      panelId: importPanelId,
      label: labels.importTab,
      icon: <FileSearch aria-hidden="true" />,
    },
    {
      value: "backup" as const,
      id: backupTabId,
      panelId: backupPanelId,
      label: labels.backupTab,
      icon: <DatabaseBackup aria-hidden="true" />,
    },
    {
      value: "maintenance" as const,
      id: maintenanceTabId,
      panelId: maintenancePanelId,
      label: labels.maintenanceTab,
      icon: <Wrench aria-hidden="true" />,
    },
  ];
  const selectManagementSection = (nextSection: LibraryDataManagementSection) => {
    setSection(nextSection);
    if (nextSection === "maintenance") {
      setCleanupConfirming(false);
    }
  };
  const sectionTabs = useRovingTabs({
    items: managementTabs,
    value: section,
    onValueChange: selectManagementSection,
  });

  const updateBatch = React.useCallback((
    batch: LibraryImportBatch,
    options: { select?: boolean } = {},
  ) => {
    let selectedBatchApplied = false;
    if (options.select || selectedBatchRef.current?.id === batch.id) {
      const reconciled = reconcileLibraryImportBatch(selectedBatchRef.current, batch);
      selectedBatchRef.current = reconciled;
      selectedBatchApplied = reconciled === batch;
    }
    setSelectedBatch((current) => {
      if (!options.select && current?.id !== batch.id) return current;
      return reconcileLibraryImportBatch(current, batch);
    });
    setBatches((current) => {
      const existingIndex = current.findIndex((item) => item.id === batch.id);
      if (existingIndex < 0) {
        return [{ ...batch, candidates: undefined }, ...current];
      }
      return current.map((item, index) => {
        if (index !== existingIndex) return item;
        return {
          ...reconcileLibraryImportBatch(item, batch),
          candidates: undefined,
        };
      });
    });
    return selectedBatchApplied;
  }, []);

  const finishImportOperation = React.useCallback((
    token: LibraryImportOperationToken,
  ) => {
    if (!importOperationsRef.current?.settle(token)) return;
    if (token.lane === "cancellation") {
      setCancelInFlight(false);
      return;
    }
    const action: BusyAction = token.kind === "select" ? "refresh" : token.kind;
    setBusy((current) => current === action ? null : current);
  }, []);

  React.useEffect(() => () => {
    importOperationsRef.current?.invalidate();
  }, []);

  const loadImportData = React.useCallback(async (preferredBatchId?: string) => {
    const listed = await listLibraryImportBatches();
    setBatches(Array.isArray(listed) ? listed : []);
    const targetId = preferredBatchId
      || selectedBatch?.id
      || listed?.[0]?.id;
    if (!targetId) {
      setSelectedBatch(null);
      return;
    }
    const detail = await getLibraryImportBatch(targetId);
    setSelectedBatch(detail);
  }, [selectedBatch?.id]);

  const loadBackupData = React.useCallback(async () => {
    const [listed, pending] = await Promise.all([
      listLibraryMetadataBackups(),
      getPendingLibraryMetadataRestore(),
    ]);
    setBackups(Array.isArray(listed) ? listed : []);
    setPendingRestore(pending ?? null);
  }, []);

  const loadMaintenanceData = React.useCallback(async () => {
    const result = await scanLibraryMaintenance();
    setMaintenanceResult({
      checkedFiles: Number.isFinite(result?.checkedFiles) ? result.checkedFiles : 0,
      missingFiles: Array.isArray(result?.missingFiles) ? result.missingFiles : [],
      deletedFiles: Array.isArray(result?.deletedFiles) ? result.deletedFiles : [],
      checkedTasks: Number.isFinite(result?.checkedTasks) ? result.checkedTasks : 0,
      taskIssues: Array.isArray(result?.taskIssues) ? result.taskIssues : [],
      databaseIntegrity: result.databaseIntegrity,
      trashedItems: Array.isArray(result?.trashedItems) ? result.trashedItems : [],
    });
    setDatabaseIntegrity(result.databaseIntegrity);
    setSelectedMissingFileIds([]);
    setSelectedTaskIds([]);
    setCleanupConfirming(false);
    setTaskCleanupConfirming(false);
    setRestoreTrashedConfirmingId(null);
    return result;
  }, []);

  React.useEffect(() => {
    if (!available || section !== "maintenance") return;
    let active = true;
    void getLibraryDatabaseIntegrityStatus()
      .then((status) => {
        if (active) setDatabaseIntegrity(status);
      })
      .catch(() => {
        if (active) setDatabaseIntegrity({ state: "unavailable" });
      });
    return () => {
      active = false;
    };
  }, [available, section]);

  React.useEffect(() => {
    if (!available) {
      setInitialLoading(false);
      return;
    }
    let active = true;
    setInitialLoading(true);
    Promise.all([listLibraryImportBatches(), listLibraryMetadataBackups(), getPendingLibraryMetadataRestore()])
      .then(async ([listedBatches, listedBackups, pending]) => {
        if (!active) return;
        const normalizedBatches = Array.isArray(listedBatches) ? listedBatches : [];
        setBatches(normalizedBatches);
        setBackups(Array.isArray(listedBackups) ? listedBackups : []);
        setPendingRestore(pending ?? null);
        const firstBatchId = normalizedBatches[0]?.id;
        if (firstBatchId) {
          const detail = await getLibraryImportBatch(firstBatchId);
          if (active) setSelectedBatch(detail);
        }
      })
      .catch((error: unknown) => {
        if (active) setNotice(operationError(error, labelsRef.current));
      })
      .finally(() => {
        if (active) setInitialLoading(false);
      });
    return () => {
      active = false;
    };
  }, [available]);

  React.useEffect(() => {
    const batchId = selectedBatch?.id;
    if (!available || !batchId || !POLLED_IMPORT_STATUSES.has(selectedBatch.status)) {
      return;
    }
    let active = true;
    const timer = window.setInterval(() => {
      getLibraryImportBatch(batchId)
        .then((batch) => {
          if (active) updateBatch(batch);
        })
        .catch((error: unknown) => {
          if (active) setNotice(operationError(error, labelsRef.current));
        });
    }, POLL_INTERVAL_MS);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [available, selectedBatch?.id, selectedBatch?.status, updateBatch]);

  const refresh = async () => {
    setBusy("refresh");
    setNotice(null);
    try {
      if (section === "import") {
        await loadImportData();
      } else if (section === "backup") {
        await loadBackupData();
      } else {
        await loadMaintenanceData();
      }
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const scan = async () => {
    if (busy !== null || cancelInFlight) return;
    const operation = importOperationsRef.current?.begin("scan");
    if (!operation) return;
    setBusy("scan");
    setNotice(null);
    setShowCandidatePaths(false);
    try {
      const batch = await selectLibraryImportAndDryRun({
        selectionKind,
        mode,
        libraryId: props.libraryId,
        hiddenPolicy,
        symlinkPolicy,
      });
      if (!importOperationsRef.current?.isCurrent(operation)) return;
      const applied = updateBatch(batch, { select: true });
      const message = resolveLibraryImportResultNotice(
        "scan",
        batch,
        labels.import,
      );
      if (message && applied && importOperationsRef.current?.canAnnounce(operation)) {
        setNotice({ tone: "success", message });
      }
    } catch (error) {
      if (importOperationsRef.current?.isCurrent(operation)) {
        setNotice(operationError(error, labels));
      }
    } finally {
      finishImportOperation(operation);
    }
  };

  const commit = async () => {
    if (!selectedBatch || busy !== null || cancelInFlight) return;
    const target = selectedBatch;
    const operation = importOperationsRef.current?.begin("commit", target.id);
    if (!operation) return;
    setBusy("commit");
    setNotice(null);
    updateBatch({ ...target, status: "running", cancelRequested: false });
    try {
      const batch = await commitLibraryImport(target.id);
      if (!importOperationsRef.current?.isCurrent(operation)) return;
      const applied = updateBatch(batch);
      const message = resolveLibraryImportResultNotice(
        "commit",
        batch,
        labels.import,
      );
      if (message && applied && importOperationsRef.current?.canAnnounce(operation)) {
        setNotice({ tone: "success", message });
      }
    } catch (error) {
      if (importOperationsRef.current?.isCurrent(operation)) {
        setNotice(operationError(error, labels));
      }
    } finally {
      finishImportOperation(operation);
    }
  };

  const resume = async () => {
    if (!selectedBatch || busy !== null || cancelInFlight) return;
    const target = selectedBatch;
    const operation = importOperationsRef.current?.begin("resume", target.id);
    if (!operation) return;
    setBusy("resume");
    setNotice(null);
    updateBatch({ ...target, status: "running", cancelRequested: false });
    try {
      const batch = await resumeLibraryImport(target.id);
      if (!importOperationsRef.current?.isCurrent(operation)) return;
      const applied = updateBatch(batch);
      const message = resolveLibraryImportResultNotice(
        "resume",
        batch,
        labels.import,
      );
      if (message && applied && importOperationsRef.current?.canAnnounce(operation)) {
        setNotice({ tone: "success", message });
      }
    } catch (error) {
      if (importOperationsRef.current?.isCurrent(operation)) {
        setNotice(operationError(error, labels));
      }
    } finally {
      finishImportOperation(operation);
    }
  };

  const cancel = async () => {
    if (!selectedBatch) return;
    const executionIsInFlight = busy === "commit" || busy === "resume";
    if (cancelInFlight || (busy !== null && !executionIsInFlight)) return;
    const target = selectedBatch;
    const operation = importOperationsRef.current?.beginCancellation(target.id);
    if (!operation) return;
    if (operation.lane === "cancellation") {
      setCancelInFlight(true);
    } else {
      setBusy("cancel");
    }
    setNotice(null);
    try {
      const batch = await cancelLibraryImport(target.id);
      if (!importOperationsRef.current?.isCurrent(operation)) return;
      const applied = updateBatch(batch);
      const message = resolveLibraryImportResultNotice(
        "cancel",
        batch,
        labels.import,
      );
      if (message && applied && importOperationsRef.current?.canAnnounce(operation)) {
        setNotice({ tone: "success", message });
      }
    } catch (error) {
      if (importOperationsRef.current?.isCurrent(operation)) {
        setNotice(operationError(error, labels));
      }
    } finally {
      finishImportOperation(operation);
    }
  };

  const selectBatch = async (batchId: string) => {
    if (busy !== null || cancelInFlight) return;
    const operation = importOperationsRef.current?.begin("select", batchId);
    if (!operation) return;
    setBusy("refresh");
    setNotice(null);
    setShowCandidatePaths(false);
    try {
      const batch = await getLibraryImportBatch(batchId);
      if (!importOperationsRef.current?.isCurrent(operation)) return;
      updateBatch(batch, { select: true });
    } catch (error) {
      if (importOperationsRef.current?.isCurrent(operation)) {
        setNotice(operationError(error, labels));
      }
    } finally {
      finishImportOperation(operation);
    }
  };

  const createBackup = async () => {
    setBusy("create-backup");
    setNotice(null);
    try {
      const manifest = await createLibraryMetadataBackup();
      const verification = await verifyLibraryMetadataBackup(manifest.backupId);
      setVerifications((current) => ({
        ...current,
        [manifest.backupId]: verification,
      }));
      await loadBackupData();
      if (!verification.valid) {
        throw new Error(labels.backup.verificationInvalid);
      }
      setNotice({ tone: "success", message: labels.backup.createCompletedNotice });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const verifyBackup = async (backupId: string) => {
    setBusy("verify-backup");
    setNotice(null);
    try {
      const result = await verifyLibraryMetadataBackup(backupId);
      setVerifications((current) => ({ ...current, [backupId]: result }));
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const scheduleRestore = async () => {
    if (!restoreTarget || !isLibraryRestoreConfirmationValid(
      restoreConfirmation,
      labels.backup.restoreConfirmationPhrase,
    )) {
      return;
    }
    setBusy("plan-restore");
    setNotice(null);
    try {
      const plan = await planLibraryMetadataRestore(restoreTarget);
      setPendingRestore(plan);
      setRestoreTarget(null);
      setRestoreConfirmation("");
      setNotice({ tone: "success", message: labels.backup.restoreScheduledNotice });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const cancelPendingRestore = async () => {
    setBusy("cancel-restore");
    setNotice(null);
    try {
      await cancelPendingLibraryMetadataRestore();
      setPendingRestore(null);
      setNotice({ tone: "success", message: labels.backup.pendingCancelledNotice });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const scanForMissingFiles = async () => {
    setBusy("scan-missing");
    setNotice(null);
    try {
      const result = await loadMaintenanceData();
      setNotice({
        tone: "success",
        message: formatCountTemplate(labels.maintenance.scanCompletedNotice, {
          checked: Number.isFinite(result?.checkedFiles) ? result.checkedFiles : 0,
          missing: Array.isArray(result?.missingFiles) ? result.missingFiles.length : 0,
        }),
      });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const clearSelectedMissing = async () => {
    if (selectedMissingFileIds.length === 0) return;
    setBusy("clear-missing");
    setNotice(null);
    try {
      const result = await clearSelectedMissingLibraryFiles(selectedMissingFileIds);
      await loadMaintenanceData();
      await props.onMaintenanceChanged?.();
      setNotice({
        tone: "success",
        message: result.removed > 0
          ? formatCountTemplate(labels.maintenance.removedNotice, { count: result.removed })
          : labels.maintenance.noneRemovedNotice,
      });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  const toggleMissingSelection = (file: MissingLibraryFileDTO) => {
    setCleanupConfirming(false);
    setSelectedMissingFileIds((current) => current.includes(file.fileId)
      ? current.filter((fileId) => fileId !== file.fileId)
      : [...current, file.fileId]);
  };

  const restoreDeletedFile = async (fileId: string) => {
    setBusy("restore-deleted");
    setRestoringDeletedFileId(fileId);
    setNotice(null);
    try {
      const result = await restoreDeletedLibraryFiles([fileId]);
      await loadMaintenanceData();
      await props.onMaintenanceChanged?.();
      setNotice({
        tone: result.restored > 0 ? "success" : "error",
        message: result.restored > 0
          ? formatCountTemplate(labels.maintenance.restoredNotice, { count: result.restored })
          : labels.maintenance.restoreUnavailable,
      });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setRestoringDeletedFileId(null);
      setBusy(null);
    }
  };

  const restoreTrashedItem = async (item: CatalogItem) => {
    setBusy("restore-trashed");
    setNotice(null);
    try {
      await restoreTrashedCatalogItem(item);
      await loadMaintenanceData();
      await props.onMaintenanceChanged?.();
      setNotice({ tone: "success", message: labels.maintenance.catalogRestoredNotice });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setRestoreTrashedConfirmingId(null);
      setBusy(null);
    }
  };

  const toggleTaskSelection = (operationId: string) => {
    setTaskCleanupConfirming(false);
    setSelectedTaskIds((current) => current.includes(operationId)
      ? current.filter((item) => item !== operationId)
      : [...current, operationId]);
  };

  const clearSelectedTasks = async () => {
    if (selectedTaskIds.length === 0) return;
    const removed = selectedTaskIds.length;
    setBusy("clear-tasks");
    setNotice(null);
    try {
      await deleteMaintenanceTasks(selectedTaskIds);
      await loadMaintenanceData();
      await props.onMaintenanceChanged?.();
      setNotice({
        tone: "success",
        message: formatCountTemplate(labels.maintenance.tasksRemoved, { count: removed }),
      });
    } catch (error) {
      setNotice(operationError(error, labels));
    } finally {
      setBusy(null);
    }
  };

  if (!available) {
    return (
      <section
        className={cn("library-data-management library-data-management--unavailable", props.className)}
        data-library-management-unavailable
      >
        <DatabaseBackup aria-hidden="true" />
        <div>
          <h2>{labels.unavailableTitle}</h2>
          <p>{labels.unavailableDescription}</p>
        </div>
      </section>
    );
  }

  const candidates = selectedBatch?.candidates ?? [];
  const anyBusy = busy !== null || cancelInFlight;
  const cancelIsBusy = busy === "cancel" || cancelInFlight;
  const cancelIsDisabled = cancelIsBusy
    || (busy !== null && busy !== "commit" && busy !== "resume");
  const missingFiles = maintenanceResult?.missingFiles ?? [];
  const deletedFiles = maintenanceResult?.deletedFiles ?? [];
  const trashedItems = maintenanceResult?.trashedItems ?? [];
  const taskIssues = maintenanceResult?.taskIssues ?? [];
  const allMissingSelected = missingFiles.length > 0
    && missingFiles.every((file) => selectedMissingFileIds.includes(file.fileId));
  const allTasksSelected = taskIssues.length > 0
    && taskIssues.every((task) => selectedTaskIds.includes(task.operationId));

  return (
    <section
      className={cn("library-data-management", props.className)}
      aria-busy={anyBusy || initialLoading}
      data-library-data-management
      data-embedded={props.embedded ? "true" : undefined}
    >
      {!props.embedded ? (
        <header className="library-data-management__header">
          <div>
            <h2>{labels.title}</h2>
            <p>{labels.description}</p>
          </div>
        </header>
      ) : null}

      <div
        className="library-data-management__toolbar"
        data-library-management-toolbar
        role="toolbar"
        aria-label={labels.title}
      >
        <div
          className="library-data-management__tabs"
          role="tablist"
          aria-label={labels.title}
          aria-orientation="horizontal"
        >
          {managementTabs.map((tab, index) => (
            <button
              key={tab.value}
              ref={(node) => sectionTabs.setTabRef(index, node)}
              id={tab.id}
              type="button"
              role="tab"
              aria-controls={tab.panelId}
              aria-selected={section === tab.value}
              tabIndex={sectionTabs.focusableIndex === index ? 0 : -1}
              data-active={section === tab.value}
              onClick={() => selectManagementSection(tab.value)}
              onKeyDown={(event) => sectionTabs.onKeyDown(event, index)}
            >
              {tab.icon}
              {tab.label}
            </button>
          ))}
        </div>
        <Button
          type="button"
          variant="outline"
          size="compact"
          disabled={anyBusy || initialLoading}
          onClick={() => void refresh()}
        >
          <RefreshCcw className={cn("h-4 w-4", busy === "refresh" && "app-motion-spin")} aria-hidden="true" />
          {labels.refresh}
        </Button>
      </div>

      {notice ? (
        <div
          className="library-data-management__notice"
          data-tone={notice.tone}
          role={notice.tone === "error" ? "alert" : "status"}
          aria-live="polite"
        >
          {notice.tone === "success"
            ? <CheckCircle2 aria-hidden="true" />
            : <CircleAlert aria-hidden="true" />}
          <span>{notice.message}</span>
          <button type="button" onClick={() => setNotice(null)} aria-label={labels.closeNotice}>
            <X aria-hidden="true" />
          </button>
        </div>
      ) : null}

      {initialLoading ? (
        <div className="library-data-management__loading" role="status">
          <Loader2 className="app-motion-spin" aria-hidden="true" />
          {labels.loading}
        </div>
      ) : section === "import" ? (
        <div
          className="library-data-management__import"
          id={importPanelId}
          role="tabpanel"
          aria-labelledby={importTabId}
        >
          <div className="library-data-management__section-heading">
            <div>
              <h3>{labels.import.title}</h3>
              <p>{labels.import.description}</p>
            </div>
          </div>

          <div className="library-data-management__import-layout">
            <div className="library-data-management__primary-column">
              <form
                className="library-data-management__config-card"
                onSubmit={(event) => {
                  event.preventDefault();
                  void scan();
                }}
              >
                <fieldset>
                  <legend>{labels.import.selectionKind}</legend>
                  <div className="library-data-management__segmented">
                    <label>
                      <input
                        type="radio"
                        name="library-import-selection"
                        value="files"
                        checked={selectionKind === "files"}
                        onChange={() => setSelectionKind("files")}
                      />
                      <FileSearch aria-hidden="true" />
                      {labels.import.selectFiles}
                    </label>
                    <label>
                      <input
                        type="radio"
                        name="library-import-selection"
                        value="directory"
                        checked={selectionKind === "directory"}
                        onChange={() => setSelectionKind("directory")}
                      />
                      <FolderOpen aria-hidden="true" />
                      {labels.import.selectFolder}
                    </label>
                  </div>
                </fieldset>

                <fieldset>
                  <legend>{labels.import.mode}</legend>
                  <div className="library-data-management__choice-grid">
                    <label data-selected={mode === "referenced"}>
                      <input
                        type="radio"
                        name="library-import-mode"
                        value="referenced"
                        checked={mode === "referenced"}
                        onChange={() => setMode("referenced")}
                      />
                      <span className="library-data-management__choice-content">
                        <strong>{labels.import.referencedMode}</strong>
                        <small>{labels.import.referencedModeDescription}</small>
                      </span>
                      <span className="library-data-management__choice-indicator" aria-hidden="true">
                        <Check />
                      </span>
                    </label>
                    <label data-selected={mode === LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY}>
                      <input
                        type="radio"
                        name="library-import-mode"
                        value={LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY}
                        checked={mode === LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY}
                        onChange={() => setMode(LIBRARY_IMPORT_MANAGED_STORAGE_STRATEGY)}
                      />
                      <span className="library-data-management__choice-content">
                        <strong>{labels.import.copyMode}</strong>
                        <small>{labels.import.copyModeDescription}</small>
                      </span>
                      <span className="library-data-management__choice-indicator" aria-hidden="true">
                        <Check />
                      </span>
                    </label>
                  </div>
                </fieldset>

                <div className="library-data-management__policy-grid">
                  <label>
                    <span>{labels.import.hiddenPolicy}</span>
                    <Select
                      className="library-data-management__policy-select"
                      value={hiddenPolicy}
                      onChange={(event) => setHiddenPolicy(event.target.value as LibraryImportHiddenPolicy)}
                    >
                      <option value="exclude">{labels.import.excludeHidden}</option>
                      <option value="include">{labels.import.includeHidden}</option>
                    </Select>
                  </label>
                  <label>
                    <span>{labels.import.symlinkPolicy}</span>
                    <Select
                      className="library-data-management__policy-select"
                      value={symlinkPolicy}
                      onChange={(event) => setSymlinkPolicy(event.target.value as LibraryImportSymlinkPolicy)}
                    >
                      <option value="skip">{labels.import.skipSymlinks}</option>
                      <option value="follow_files">{labels.import.followFileSymlinks}</option>
                    </Select>
                  </label>
                </div>

                <Button
                  className="library-data-management__scan-action"
                  type="submit"
                  disabled={anyBusy}
                >
                  {busy === "scan" ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <FileSearch className="h-4 w-4" aria-hidden="true" />}
                  {busy === "scan" ? labels.import.choosingAndScanning : labels.import.chooseAndScan}
                </Button>
              </form>

              {selectedBatch ? (
                <article className="library-data-management__batch-detail" data-batch-status={selectedBatch.status}>
                  <div className="library-data-management__card-heading">
                    <div>
                      <span>{labels.import.dryRunTitle}</span>
                      <code>{selectedBatch.id}</code>
                    </div>
                    <ImportStatusBadge status={selectedBatch.status} labels={labels.import.status} />
                  </div>
                  <p>{labels.import.dryRunDescription}</p>

                  <div className="library-data-management__counts">
                    <CountCard label={labels.import.total} value={selectedBatch.counts.total} />
                    <CountCard label={labels.import.ready} value={selectedBatch.counts.ready} />
                    <CountCard label={labels.import.duplicate} value={selectedBatch.counts.duplicate} />
                    <CountCard label={labels.import.skipped} value={selectedBatch.counts.skipped} />
                    <CountCard label={labels.import.succeeded} value={selectedBatch.counts.succeeded} />
                    <CountCard label={labels.import.failed} value={selectedBatch.counts.failed} />
                    <CountCard
                      label={labels.import.totalSize}
                      value={formatBytes(selectedBatch.counts.totalBytes, labels)}
                    />
                  </div>

                  {selectedBatch.lastError ? (
                    <div className="library-data-management__batch-error" role="alert">
                      <CircleAlert aria-hidden="true" />
                      <div>
                        <strong>{labels.import.lastError}</strong>
                        <p>{selectedBatch.lastError}</p>
                      </div>
                    </div>
                  ) : null}

                  <div className="library-data-management__batch-actions">
                    {selectedBatch.status === "ready" ? (
                      <Button type="button" disabled={anyBusy} onClick={() => void commit()}>
                        {busy === "commit" ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <CheckCircle2 className="h-4 w-4" aria-hidden="true" />}
                        {busy === "commit" ? labels.import.committing : labels.import.commit}
                      </Button>
                    ) : null}
                    {canResumeLibraryImport(selectedBatch) ? (
                      <Button type="button" variant="outline" disabled={anyBusy} onClick={() => void resume()}>
                        {busy === "resume" ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <RotateCcw className="h-4 w-4" aria-hidden="true" />}
                        {busy === "resume" ? labels.import.resuming : labels.import.resume}
                      </Button>
                    ) : null}
                    {canCancelLibraryImport(selectedBatch) ? (
                      <Button type="button" variant="outline" disabled={cancelIsDisabled} onClick={() => void cancel()}>
                        {cancelIsBusy ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <Square className="h-3.5 w-3.5" aria-hidden="true" />}
                        {cancelIsBusy ? labels.import.cancelling : labels.import.cancel}
                      </Button>
                    ) : null}
                  </div>

                  <div className="library-data-management__candidate-heading">
                    <div>
                      <h4>{labels.import.candidates}</h4>
                      <p>{labels.import.localPathPrivacy}</p>
                    </div>
                    {candidates.length > 0 ? (
                      <Button
                        type="button"
                        variant="ghost"
                        size="compact"
                        onClick={() => setShowCandidatePaths((current) => !current)}
                      >
                        {showCandidatePaths
                          ? <EyeOff className="h-4 w-4" aria-hidden="true" />
                          : <Eye className="h-4 w-4" aria-hidden="true" />}
                        {showCandidatePaths ? labels.import.hideLocalPaths : labels.import.revealLocalPaths}
                      </Button>
                    ) : null}
                  </div>

                  {candidates.length > 0 ? (
                    <ul className="library-data-management__candidates">
                      {candidates.map((candidate) => (
                        <li key={candidate.id}>
                          <div className="library-data-management__candidate-main">
                            <strong>{candidate.displayName}</strong>
                            <span>{formatBytes(candidate.sizeBytes, labels)}</span>
                          </div>
                          <StatusBadge
                            className="library-data-management__candidate-status"
                            data-status={candidate.status}
                            tone={resolveImportCandidateStatusTone(candidate.status)}
                          >
                            {labels.import.candidateStatus[candidate.status]}
                          </StatusBadge>
                          {showCandidatePaths ? (
                            <dl className="library-data-management__paths">
                              <div>
                                <dt>{labels.import.sourcePath}</dt>
                                <dd>{candidate.sourcePath}</dd>
                              </div>
                              {candidate.managedPath ? (
                                <div>
                                  <dt>{labels.import.managedPath}</dt>
                                  <dd>{candidate.managedPath}</dd>
                                </div>
                              ) : null}
                            </dl>
                          ) : null}
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="library-data-management__empty">{labels.import.noCandidates}</p>
                  )}
                </article>
              ) : null}
            </div>

            <aside className="library-data-management__history" aria-label={labels.import.batchHistory}>
              <h4>{labels.import.batchHistory}</h4>
              {batches.length > 0 ? (
                <ul>
                  {batches.map((batch) => (
                    <li key={batch.id}>
                      <button
                        type="button"
                        data-selected={selectedBatch?.id === batch.id}
                        disabled={anyBusy || initialLoading}
                        onClick={() => void selectBatch(batch.id)}
                      >
                        <span>
                          <strong>
                            <span className="sr-only">{labels.import.updatedAt}: </span>
                            {formatTimestamp(batch.updatedAt)}
                          </strong>
                          <code>{batch.id}</code>
                        </span>
                        <ImportStatusBadge status={batch.status} labels={labels.import.status} />
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <p>{labels.import.noBatches}</p>
              )}
            </aside>
          </div>
        </div>
      ) : section === "backup" ? (
        <div
          className="library-data-management__backup"
          id={backupPanelId}
          role="tabpanel"
          aria-labelledby={backupTabId}
        >
          <div className="library-data-management__section-heading">
            <div>
              <h3>{labels.backup.title}</h3>
              <p>{labels.backup.description}</p>
            </div>
            <Button type="button" disabled={anyBusy} onClick={() => void createBackup()}>
              {busy === "create-backup" ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <DatabaseBackup className="h-4 w-4" aria-hidden="true" />}
              {busy === "create-backup" ? labels.backup.creating : labels.backup.create}
            </Button>
          </div>

          <aside className="library-data-management__privacy" role="note" data-library-backup-privacy>
            <ShieldAlert aria-hidden="true" />
            <div>
              <h4>{labels.backup.privateSnapshotTitle}</h4>
              <p>{labels.backup.privateSnapshotWarning}</p>
              <p>{labels.backup.sensitiveMetadataWarning}</p>
              <p>{labels.backup.noMediaWarning}</p>
              <p>{labels.backup.localPermissionsNote}</p>
            </div>
          </aside>

          {pendingRestore ? (
            <section className="library-data-management__pending" aria-labelledby="library-pending-restore-title">
              <ArchiveRestore aria-hidden="true" />
              <div>
                <h4 id="library-pending-restore-title">{labels.backup.pendingRestoreTitle}</h4>
                <p>{labels.backup.pendingRestoreDescription}</p>
                <code>{pendingRestore.backupId}</code>
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={anyBusy}
                onClick={() => void cancelPendingRestore()}
              >
                {busy === "cancel-restore" ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <X className="h-4 w-4" aria-hidden="true" />}
                {busy === "cancel-restore"
                  ? labels.backup.cancellingPendingRestore
                  : labels.backup.cancelPendingRestore}
              </Button>
            </section>
          ) : null}

          <section className="library-data-management__backup-list" aria-labelledby="library-backups-title">
            <h4 id="library-backups-title">{labels.backup.backups}</h4>
            {backups.length > 0 ? (
              <ul>
                {backups.map((backup) => {
                  const verification = verifications[backup.backupId];
                  const confirming = restoreTarget === backup.backupId;
                  return (
                    <li key={backup.backupId}>
                      <div className="library-data-management__backup-summary">
                        <div className="library-data-management__backup-title">
                          <DatabaseBackup aria-hidden="true" />
                          <div>
                            <strong>
                              <span className="sr-only">{labels.backup.createdAt}: </span>
                              {formatTimestamp(backup.createdAt)}
                            </strong>
                            <code>
                              <span className="sr-only">{labels.backup.backupId}: </span>
                              {backup.backupId}
                            </code>
                          </div>
                        </div>
                        <dl>
                          <div>
                            <dt>{labels.backup.size}</dt>
                            <dd>{formatBytes(backup.sizeBytes, labels)}</dd>
                          </div>
                          <div>
                            <dt>{labels.backup.schemaVersion}</dt>
                            <dd>{backup.schemaVersion}</dd>
                          </div>
                          <div>
                            <dt>{labels.backup.applicationVersion}</dt>
                            <dd>{backup.appVersion}</dd>
                          </div>
                          <div>
                            <dt>{labels.backup.state}</dt>
                            <dd>{backup.state}</dd>
                          </div>
                        </dl>
                        <div className="library-data-management__backup-actions">
                          {verification ? (
                            <span data-valid={verification.valid} role="status">
                              {verification.valid
                                ? <CheckCircle2 aria-hidden="true" />
                                : <CircleAlert aria-hidden="true" />}
                              {verification.valid
                                ? labels.backup.verificationValid
                                : labels.backup.verificationInvalid}
                            </span>
                          ) : null}
                          <Button
                            type="button"
                            variant="outline"
                            size="compact"
                            disabled={anyBusy}
                            onClick={() => void verifyBackup(backup.backupId)}
                          >
                            {busy === "verify-backup" ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" /> : <CheckCircle2 className="h-4 w-4" aria-hidden="true" />}
                            {busy === "verify-backup" ? labels.backup.verifying : labels.backup.verify}
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="compact"
                            disabled={anyBusy || pendingRestore !== null}
                            onClick={() => {
                              setRestoreTarget(backup.backupId);
                              setRestoreConfirmation("");
                            }}
                          >
                            <ArchiveRestore className="h-4 w-4" aria-hidden="true" />
                            {labels.backup.restore}
                          </Button>
                        </div>
                      </div>

                      {confirming ? (
                        <LibraryBackupRestoreConfirmation
                          labels={labels.backup}
                          value={restoreConfirmation}
                          busy={busy === "plan-restore"}
                          disabled={anyBusy}
                          onValueChange={setRestoreConfirmation}
                          onConfirm={() => void scheduleRestore()}
                          onCancel={() => {
                            setRestoreTarget(null);
                            setRestoreConfirmation("");
                          }}
                        />
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            ) : (
              <p className="library-data-management__empty">{labels.backup.noBackups}</p>
            )}
          </section>
        </div>
      ) : (
        <div
          className="library-data-management__maintenance"
          id={maintenancePanelId}
          role="tabpanel"
          aria-labelledby={maintenanceTabId}
        >
          <div className="library-data-management__section-heading">
            <div>
              <h3>{labels.maintenance.title}</h3>
              <p>{labels.maintenance.description}</p>
            </div>
            <Button
              type="button"
              disabled={anyBusy}
              onClick={() => void scanForMissingFiles()}
            >
              {busy === "scan-missing"
                ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
                : <ListChecks className="h-4 w-4" aria-hidden="true" />}
              {busy === "scan-missing"
                ? labels.maintenance.scanning
                : labels.maintenance.scan}
            </Button>
          </div>

          <aside className="library-data-management__maintenance-safety" role="note">
            <ShieldCheck aria-hidden="true" />
            <div>
              <h4>{labels.maintenance.safeCleanupTitle}</h4>
              <p>{labels.maintenance.safeCleanupDescription}</p>
            </div>
          </aside>

          {databaseIntegrity?.state === "failed" ? (
            <aside
              className="library-data-management__maintenance-safety"
              data-state="danger"
              role="alert"
            >
              <ShieldAlert aria-hidden="true" />
              <div>
                <h4>{labels.maintenance.databaseIntegrityFailedTitle}</h4>
                <p>
                  {labels.maintenance.databaseIntegrityFailedDescription}
                  {databaseIntegrity.checkedAt
                    ? ` ${labels.maintenance.lastChecked}: ${formatTimestamp(databaseIntegrity.checkedAt)}.`
                    : ""}
                  {databaseIntegrity.detail ? ` ${databaseIntegrity.detail}` : ""}
                </p>
              </div>
            </aside>
          ) : null}

          {maintenanceResult ? (
            <div className="library-data-management__maintenance-counts">
              <CountCard label={labels.maintenance.checked} value={maintenanceResult.checkedFiles} />
              <CountCard label={labels.maintenance.missing} value={missingFiles.length} />
              <CountCard label={labels.maintenance.deleted} value={deletedFiles.length} />
              <CountCard label={labels.maintenance.trashedItems} value={trashedItems.length} />
              <CountCard label={labels.maintenance.taskIssues} value={taskIssues.length} />
            </div>
          ) : null}

          {!maintenanceResult ? (
            <section className="library-data-management__maintenance-state">
              <FileSearch aria-hidden="true" />
              <div>
                <h4>{labels.maintenance.notScannedTitle}</h4>
                <p>{labels.maintenance.notScannedDescription}</p>
              </div>
            </section>
          ) : missingFiles.length === 0 ? (
            deletedFiles.length === 0 && trashedItems.length === 0 && taskIssues.length === 0 ? (
            <section className="library-data-management__maintenance-state" data-state="healthy">
              <CheckCircle2 aria-hidden="true" />
              <div>
                <h4>{labels.maintenance.healthyTitle}</h4>
                <p>{labels.maintenance.healthyDescription}</p>
              </div>
            </section>
            ) : null
          ) : (
            <section className="library-data-management__missing-list" aria-labelledby="library-missing-files-title">
              <div className="library-data-management__missing-heading">
                <div>
                  <h4 id="library-missing-files-title">{labels.maintenance.missingTitle}</h4>
                  <p>{labels.maintenance.selectionHint}</p>
                </div>
                <div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    disabled={anyBusy}
                    onClick={() => {
                      setCleanupConfirming(false);
                      setSelectedMissingFileIds(allMissingSelected
                        ? []
                        : missingFiles.map((file) => file.fileId));
                    }}
                  >
                    <Check aria-hidden="true" className="h-4 w-4" />
                    {allMissingSelected
                      ? labels.maintenance.clearSelection
                      : labels.maintenance.selectAll}
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={() => setShowMissingPaths((current) => !current)}
                  >
                    {showMissingPaths
                      ? <EyeOff className="h-4 w-4" aria-hidden="true" />
                      : <Eye className="h-4 w-4" aria-hidden="true" />}
                    {showMissingPaths
                      ? labels.maintenance.hidePaths
                      : labels.maintenance.showPaths}
                  </Button>
                </div>
              </div>

              <ul>
                {missingFiles.map((file) => {
                  const selected = selectedMissingFileIds.includes(file.fileId);
                  return (
                    <li key={file.fileId} data-selected={selected}>
                      <label>
                        <input
                          type="checkbox"
                          checked={selected}
                          disabled={anyBusy}
                          onChange={() => toggleMissingSelection(file)}
                        />
                        <span className="library-data-management__missing-check" aria-hidden="true">
                          <Check />
                        </span>
                        <span className="library-data-management__missing-main">
                          <strong>{file.name}</strong>
                          <span>
                            {[
                              props.categoryLabel?.(file.kind) ?? file.kind,
                              file.format?.toUpperCase(),
                            ].filter(Boolean).join(" · ")}
                          </span>
                        </span>
                      </label>
                      {file.lastChecked ? (
                        <span className="library-data-management__missing-checked">
                          {labels.maintenance.lastChecked}: {formatTimestamp(file.lastChecked)}
                        </span>
                      ) : null}
                      {showMissingPaths ? (
                        <dl className="library-data-management__paths">
                          <div>
                            <dt>{labels.maintenance.localPath}</dt>
                            <dd>{file.oldPath}</dd>
                          </div>
                        </dl>
                      ) : null}
                    </li>
                  );
                })}
              </ul>

              {cleanupConfirming ? (
                <div className="library-data-management__cleanup-confirmation" role="alertdialog" aria-modal="false">
                  <CircleAlert aria-hidden="true" />
                  <div>
                    <h5>{labels.maintenance.confirmTitle}</h5>
                    <p>{formatCountTemplate(labels.maintenance.confirmDescription, {
                      count: selectedMissingFileIds.length,
                    })}</p>
                  </div>
                  <div>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={anyBusy}
                      onClick={() => setCleanupConfirming(false)}
                    >
                      {labels.maintenance.cancelConfirmation}
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      disabled={anyBusy || selectedMissingFileIds.length === 0}
                      onClick={() => void clearSelectedMissing()}
                    >
                      {busy === "clear-missing"
                        ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
                        : <Trash2 className="h-4 w-4" aria-hidden="true" />}
                      {busy === "clear-missing"
                        ? labels.maintenance.removing
                        : labels.maintenance.confirmRemove}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="library-data-management__missing-actions">
                  <Button
                    type="button"
                    variant="destructive"
                    disabled={anyBusy || selectedMissingFileIds.length === 0}
                    onClick={() => setCleanupConfirming(true)}
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                    {labels.maintenance.removeSelected}
                    {selectedMissingFileIds.length > 0
                      ? ` (${selectedMissingFileIds.length})`
                      : ""}
                  </Button>
                </div>
              )}
            </section>
          )}

          {deletedFiles.length > 0 ? (
            <section
              className="library-data-management__missing-list"
              aria-labelledby="library-deleted-files-title"
              data-maintenance-kind="deleted-files"
            >
              <div className="library-data-management__missing-heading">
                <div>
                  <h4 id="library-deleted-files-title">{labels.maintenance.deletedTitle}</h4>
                  <p>{labels.maintenance.deletedDescription}</p>
                </div>
                <div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={() => setShowMissingPaths((current) => !current)}
                  >
                    {showMissingPaths
                      ? <EyeOff className="h-4 w-4" aria-hidden="true" />
                      : <Eye className="h-4 w-4" aria-hidden="true" />}
                    {showMissingPaths
                      ? labels.maintenance.hidePaths
                      : labels.maintenance.showPaths}
                  </Button>
                </div>
              </div>
              <ul>
                {deletedFiles.map((file) => (
                  <li key={file.fileId} data-restorable={file.canRestore ? "true" : "false"}>
                    <span className="library-data-management__missing-main">
                      <strong>{file.name}</strong>
                      <span>{[
                        props.categoryLabel?.(file.kind) ?? file.kind,
                        file.format?.toUpperCase(),
                      ].filter(Boolean).join(" · ")}</span>
                    </span>
                    <span>
                      <StatusBadge tone={file.canRestore ? "success" : "muted"}>
                        {file.canRestore
                          ? labels.maintenance.restoreDeleted
                          : labels.maintenance.restoreUnavailable}
                      </StatusBadge>
                      {" "}
                      {file.canRestore ? (
                        <Button
                          type="button"
                          variant="outline"
                          size="compact"
                          disabled={anyBusy}
                          onClick={() => void restoreDeletedFile(file.fileId)}
                        >
                          {busy === "restore-deleted" && restoringDeletedFileId === file.fileId
                            ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
                            : <RotateCcw className="h-4 w-4" aria-hidden="true" />}
                          {labels.maintenance.restoreDeleted}
                        </Button>
                      ) : null}
                    </span>
                    {showMissingPaths && file.oldPath ? (
                      <dl className="library-data-management__paths">
                        <div>
                          <dt>{labels.maintenance.localPath}</dt>
                          <dd>{file.oldPath}</dd>
                        </div>
                      </dl>
                    ) : null}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {trashedItems.length > 0 ? (
            <section
              className="library-data-management__missing-list"
              aria-labelledby="library-trashed-items-title"
              data-maintenance-kind="trashed-items"
            >
              <div className="library-data-management__missing-heading">
                <div>
                  <h4 id="library-trashed-items-title">{labels.maintenance.trashedTitle}</h4>
                  <p>{labels.maintenance.trashedDescription}</p>
                </div>
              </div>
              <ul>
                {trashedItems.map((item) => (
                  <li key={item.id}>
                    <span className="library-data-management__missing-main">
                      <strong>{item.title}</strong>
                      <span>{props.categoryLabel?.(item.category) ?? item.category}</span>
                    </span>
                    <span>
                      <StatusBadge tone="muted">{labels.maintenance.trashedItems}</StatusBadge>
                      {" "}
                      <Button
                        type="button"
                        variant="outline"
                        size="compact"
                        disabled={anyBusy}
                        onClick={() => setRestoreTrashedConfirmingId(item.id)}
                      >
                        <RotateCcw className="h-4 w-4" aria-hidden="true" />
                        {labels.maintenance.restoreDeleted}
                      </Button>
                    </span>
                  </li>
                ))}
              </ul>
              {restoreTrashedConfirmingId ? (
                <div
                  className="library-data-management__cleanup-confirmation"
                  role="alertdialog"
                  aria-modal="false"
                >
                  <CircleAlert aria-hidden="true" />
                  <div>
                    <h5>{labels.maintenance.confirmCatalogRestoreTitle}</h5>
                    <p>{labels.maintenance.confirmCatalogRestoreDescription}</p>
                  </div>
                  <div>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={anyBusy}
                      onClick={() => setRestoreTrashedConfirmingId(null)}
                    >
                      {labels.maintenance.cancelConfirmation}
                    </Button>
                    <Button
                      type="button"
                      disabled={anyBusy}
                      onClick={() => {
                        const item = trashedItems.find(
                          (candidate) => candidate.id === restoreTrashedConfirmingId,
                        );
                        if (item) void restoreTrashedItem(item);
                      }}
                    >
                      {busy === "restore-trashed"
                        ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
                        : <RotateCcw className="h-4 w-4" aria-hidden="true" />}
                      {labels.maintenance.confirmCatalogRestore}
                    </Button>
                  </div>
                </div>
              ) : null}
            </section>
          ) : null}

          {taskIssues.length > 0 ? (
            <section
              className="library-data-management__missing-list"
              aria-labelledby="library-task-issues-title"
              data-maintenance-kind="task-issues"
            >
              <div className="library-data-management__missing-heading">
                <div>
                  <h4 id="library-task-issues-title">{labels.maintenance.taskIssuesTitle}</h4>
                  <p>{labels.maintenance.taskIssuesDescription}</p>
                </div>
                <div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    disabled={anyBusy}
                    onClick={() => {
                      setTaskCleanupConfirming(false);
                      setSelectedTaskIds(allTasksSelected
                        ? []
                        : taskIssues.map((task) => task.operationId));
                    }}
                  >
                    <Check aria-hidden="true" className="h-4 w-4" />
                    {allTasksSelected
                      ? labels.maintenance.clearSelection
                      : labels.maintenance.selectAll}
                  </Button>
                </div>
              </div>
              <ul>
                {taskIssues.map((task) => {
                  const selected = selectedTaskIds.includes(task.operationId);
                  return (
                    <li key={task.operationId} data-selected={selected}>
                      <label>
                        <input
                          type="checkbox"
                          checked={selected}
                          disabled={anyBusy}
                          onChange={() => toggleTaskSelection(task.operationId)}
                        />
                        <span className="library-data-management__missing-check" aria-hidden="true">
                          <Check />
                        </span>
                        <span className="library-data-management__missing-main">
                          <strong>{task.name}</strong>
                          <span>
                            {labels.maintenance.availableOutputs}: {task.availableOutputCount}/{task.outputCount}
                          </span>
                          <span>
                            {labels.maintenance.executionStatus}: {task.executionStatus === "succeeded"
                              ? labels.maintenance.executionSucceeded
                              : task.executionStatus}
                          </span>
                        </span>
                      </label>
                      <StatusBadge tone="danger">{labels.maintenance.healthUnavailable}</StatusBadge>
                    </li>
                  );
                })}
              </ul>
              {taskCleanupConfirming ? (
                <div
                  className="library-data-management__cleanup-confirmation"
                  role="alertdialog"
                  aria-modal="false"
                >
                  <CircleAlert aria-hidden="true" />
                  <div>
                    <h5>{labels.maintenance.confirmTaskCleanupTitle}</h5>
                    <p>{formatCountTemplate(labels.maintenance.confirmTaskCleanupDescription, {
                      count: selectedTaskIds.length,
                    })}</p>
                  </div>
                  <div>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={anyBusy}
                      onClick={() => setTaskCleanupConfirming(false)}
                    >
                      {labels.maintenance.cancelConfirmation}
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      disabled={anyBusy || selectedTaskIds.length === 0}
                      onClick={() => void clearSelectedTasks()}
                    >
                      {busy === "clear-tasks"
                        ? <Loader2 className="h-4 w-4 app-motion-spin" aria-hidden="true" />
                        : <Trash2 className="h-4 w-4" aria-hidden="true" />}
                      {busy === "clear-tasks"
                        ? labels.maintenance.removingTasks
                        : labels.maintenance.confirmTaskRemove}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="library-data-management__missing-actions">
                  <Button
                    type="button"
                    variant="destructive"
                    disabled={anyBusy || selectedTaskIds.length === 0}
                    onClick={() => setTaskCleanupConfirming(true)}
                  >
                    <Trash2 className="h-4 w-4" aria-hidden="true" />
                    {labels.maintenance.removeSelectedTasks}
                    {selectedTaskIds.length > 0 ? ` (${selectedTaskIds.length})` : ""}
                  </Button>
                </div>
              )}
            </section>
          ) : null}
        </div>
      )}
    </section>
  );
}
