import type {
  FileEventRecordDTO,
  LibraryFileDTO,
  LibraryHistoryRecordDTO,
  OperationOutputFileDTO,
} from "@/shared/contracts/library";

import type { LibraryWorkspaceItem } from "./types";

export type TaskOutputVersionState = "current" | "historical" | "detached" | "deleted" | "missing";

export interface TaskOutputVersion {
  fileId: string;
  name: string;
  kind: string;
  format: string;
  sizeBytes?: number;
  path?: string;
  state: TaskOutputVersionState;
  changedAt?: string;
}

function eventOperationId(event: FileEventRecordDTO) {
  return event.operationId?.trim() || event.detail.cause.operationId?.trim() || "";
}

function eventTime(event: FileEventRecordDTO) {
  return event.occurredAt?.trim() || event.createdAt;
}

function compareEventTime(left: FileEventRecordDTO, right: FileEventRecordDTO) {
  return eventTime(left).localeCompare(eventTime(right));
}

export function fileEventsForLibraryItem(item: LibraryWorkspaceItem): FileEventRecordDTO[] {
  const events = item.library?.records.fileEvents ?? [];
  const filtered = item.source === "task"
    ? events.filter((event) => eventOperationId(event) === item.operation?.operationId)
    : events.filter((event) => event.fileId === item.file?.id);
  return [...filtered].sort((left, right) => compareEventTime(right, left));
}

export function operationHistoryEventsForTask(
  item: LibraryWorkspaceItem,
): LibraryHistoryRecordDTO[] {
  if (item.source !== "task" || !item.operation) return [];
  return [...(item.library?.records.history ?? [])]
    .filter((record) =>
      record.category === "operation_event" &&
      record.refs.operationId === item.operation?.operationId,
    )
    .sort((left, right) => right.occurredAt.localeCompare(left.occurredAt));
}

export interface OperationRenameTransition {
  recordId: string;
  before: string;
  after: string;
}

/**
 * Rename lifecycle rows persist the name immediately before each mutation.
 * Walk newest-to-oldest from the live title to reconstruct both sides without
 * changing the immutable backend event contract.
 */
export function operationRenameTransitionsForTask(
  item: LibraryWorkspaceItem,
): OperationRenameTransition[] {
  let after = item.title.trim() || item.operation?.name.trim() || "";
  const transitions: OperationRenameTransition[] = [];
  for (const record of operationHistoryEventsForTask(item)) {
    if (record.action !== "operation_renamed") continue;
    const before = record.displayName.trim();
    transitions.push({ recordId: record.recordId, before, after });
    if (before) after = before;
  }
  return transitions;
}

function eventDeletedFile(event: FileEventRecordDTO) {
  if (event.eventType === "file_deleted") return true;
  return event.detail.changes?.some((change) =>
    change.field === "fileLifecycle" && change.after === "deleted",
  ) ?? false;
}

function eventRestoredFile(event: FileEventRecordDTO) {
  if (event.eventType === "file_restored") return true;
  return event.detail.changes?.some((change) =>
    change.field === "fileLifecycle" && change.after === "active",
  ) ?? false;
}

function mostRecentSnapshot(events: readonly FileEventRecordDTO[]) {
  for (const event of events) {
    if (event.detail.after) return event.detail.after;
    if (event.detail.before) return event.detail.before;
  }
  return undefined;
}

function outputMap(outputs: readonly OperationOutputFileDTO[] | undefined) {
  return new Map((outputs ?? []).map((output) => [output.fileId, output] as const));
}

function fileName(file: LibraryFileDTO | undefined) {
  return file?.displayName?.trim() || file?.displayLabel?.trim() ||
    file?.fileName?.trim() || file?.name.trim() || "";
}

/**
 * Projects a Task's immutable output lineage. The operation only contains its
 * currently attached outputs, so the original history snapshot and file event
 * stream are deliberately included to keep detached/deleted outputs visible.
 */
export function projectTaskOutputVersions(item: LibraryWorkspaceItem): TaskOutputVersion[] {
  if (item.source !== "task" || !item.operation || !item.library) return [];

  const operationId = item.operation.operationId;
  const histories = item.library.records.history.filter(
    (record) => record.refs.operationId === operationId,
  );
  const historicalOutputs = histories.flatMap((record) => record.files ?? []);
  const historicalById = outputMap(historicalOutputs);
  const currentById = outputMap(item.operation.outputFiles);
  const detachedIds = new Set(item.operation.detachedOutputFileIds ?? []);
  const libraryFiles = new Map(item.library.files.map((file) => [file.id, file] as const));
  const events = fileEventsForLibraryItem(item);
  const eventsByFile = new Map<string, FileEventRecordDTO[]>();
  for (const event of events) {
    const grouped = eventsByFile.get(event.fileId) ?? [];
    grouped.push(event);
    eventsByFile.set(event.fileId, grouped);
  }

  const orderedIds: string[] = [];
  const seen = new Set<string>();
  const include = (fileId: string) => {
    const id = fileId.trim();
    if (!id || seen.has(id)) return;
    seen.add(id);
    orderedIds.push(id);
  };
  historicalOutputs.forEach((output) => include(output.fileId));
  (item.operation.outputFiles ?? []).forEach((output) => include(output.fileId));
  (item.operation.detachedOutputFileIds ?? []).forEach(include);
  events.forEach((event) => include(event.fileId));

  return orderedIds.map((fileId) => {
    const file = libraryFiles.get(fileId);
    const current = currentById.get(fileId);
    const historical = historicalById.get(fileId);
    const fileEvents = eventsByFile.get(fileId) ?? [];
    const snapshot = mostRecentSnapshot(fileEvents);
    const detachEvent = fileEvents.find((event) => event.eventType === "operation_output_detached");
    const latestLifecycleEvent = fileEvents.find((event) =>
      eventDeletedFile(event) || eventRestoredFile(event),
    );
    const deleted = file
      ? file.state.deleted
      : latestLifecycleEvent
        ? eventDeletedFile(latestLifecycleEvent)
        : Boolean(current?.deleted || historical?.deleted);
    const detached = detachedIds.has(fileId) || Boolean(detachEvent);
    const unavailable = file && (
      Boolean(file.state.lastError?.trim()) ||
      ["missing", "offline", "unavailable", "error"].includes(file.state.status.trim().toLowerCase())
    );
    const state: TaskOutputVersionState = deleted
      ? "deleted"
      : unavailable
        ? "missing"
        : detached
          ? "detached"
          : current
            ? "current"
            : "historical";

    return {
      fileId,
      name: fileName(file) || snapshot?.name?.trim() || fileId,
      kind: file?.kind?.trim() || snapshot?.kind?.trim() || current?.kind || historical?.kind || "file",
      format: file?.media?.format?.trim() || current?.format?.trim() || historical?.format?.trim() || "",
      sizeBytes: file?.media?.sizeBytes ?? current?.sizeBytes ?? historical?.sizeBytes,
      path: file?.storage.localPath?.trim() || snapshot?.localPath?.trim() || undefined,
      state,
      changedAt: fileEvents[0] ? eventTime(fileEvents[0]) : undefined,
    };
  });
}
