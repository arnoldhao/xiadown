import { describe, expect, test } from "bun:test";

import type {
  FileEventRecordDTO,
  LibraryDTO,
  LibraryFileDTO,
  OperationListItemDTO,
} from "@/shared/contracts/library";

import {
  fileEventsForLibraryItem,
  operationHistoryEventsForTask,
  operationRenameTransitionsForTask,
  projectTaskOutputVersions,
} from "./library-preview-history";
import type { LibraryWorkspaceItem } from "./types";

function file(id: string, deleted = false): LibraryFileDTO {
  return {
    id,
    libraryId: "library-one",
    kind: "thumbnail",
    name: `${id}.webp`,
    storage: { mode: "local_path", localPath: `/Library/${id}.webp` },
    origin: { kind: "download", operationId: "operation-one" },
    lineage: {},
    metadata: {},
    media: { format: "webp", sizeBytes: 2048 },
    state: { status: deleted ? "deleted" : "available", deleted, archived: false },
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z",
  };
}

function event(
  id: string,
  fileId: string,
  eventType: string,
  createdAt: string,
  changes: FileEventRecordDTO["detail"]["changes"] = [],
): FileEventRecordDTO {
  return {
    id,
    libraryId: "library-one",
    fileId,
    operationId: "operation-one",
    eventType,
    detail: {
      cause: { category: "task_output", operationId: "operation-one" },
      before: {
        fileId,
        kind: "thumbnail",
        name: `${fileId}.webp`,
        localPath: `/Library/${fileId}.webp`,
      },
      changes,
    },
    createdAt,
  };
}

function taskItem(
  files: LibraryFileDTO[],
  events: FileEventRecordDTO[],
  operationOverrides: Partial<OperationListItemDTO> = {},
): LibraryWorkspaceItem {
  const operation: OperationListItemDTO = {
    operationId: "operation-one",
    libraryId: "library-one",
    name: "Download stamps",
    kind: "download",
    status: "succeeded",
    correlation: {},
    outputFiles: [{ fileId: "current", kind: "thumbnail", format: "webp" }],
    metrics: { fileCount: 1 },
    createdAt: "2026-01-01T00:00:00Z",
    ...operationOverrides,
  };
  const library: LibraryDTO = {
    version: "current",
    id: "library-one",
    name: "Library",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-02-01T00:00:00Z",
    createdBy: { source: "test" },
    files,
    records: {
      history: [{
        recordId: "history-one",
        libraryId: "library-one",
        category: "operation",
        action: "download",
        displayName: "Download stamps",
        status: "succeeded",
        source: { kind: "download" },
        refs: { operationId: "operation-one", fileIds: ["current", "detached"] },
        files: [
          { fileId: "current", kind: "thumbnail", format: "webp" },
          { fileId: "detached", kind: "thumbnail", format: "webp" },
        ],
        metrics: { fileCount: 2 },
        occurredAt: "2026-01-02T00:00:00Z",
        createdAt: "2026-01-01T00:00:00Z",
      }],
      workspaceStates: [],
      fileEvents: events,
    },
  };
  return {
    id: "task:operation-one",
    source: "task",
    libraryId: "library-one",
    libraryName: "Library",
    title: "Download stamps",
    subtitle: "download",
    category: "task",
    status: "succeeded",
    format: "DOWNLOAD",
    createdAt: operation.createdAt,
    updatedAt: operation.finishedAt ?? operation.createdAt,
    path: "https://example.com",
    coverURL: "cover.svg",
    rootId: operation.operationId,
    searchText: "download stamps",
    operation,
    library,
  };
}

describe("library preview history projection", () => {
  test("keeps an output in Versions after it is detached from the task", () => {
    const detachedEvent = event(
      "event-detached",
      "detached",
      "operation_output_detached",
      "2026-03-01T00:00:00Z",
      [{ field: "taskAssociation", before: "attached", after: "detached" }],
    );
    const item = taskItem(
      [file("current"), file("detached")],
      [detachedEvent],
      { detachedOutputFileIds: ["detached"] },
    );

    expect(projectTaskOutputVersions(item)).toEqual([
      expect.objectContaining({ fileId: "current", state: "current" }),
      expect.objectContaining({ fileId: "detached", name: "detached.webp", state: "detached" }),
    ]);
  });

  test("uses current file lifecycle over stale history flags", () => {
    const restored = event(
      "event-restored",
      "detached",
      "file_restored",
      "2026-04-01T00:00:00Z",
      [{ field: "fileLifecycle", before: "deleted", after: "active" }],
    );
    const item = taskItem(
      [file("current"), file("detached", false)],
      [restored],
      { detachedOutputFileIds: ["detached"] },
    );
    const detached = projectTaskOutputVersions(item).find((version) => version.fileId === "detached");

    expect(detached?.state).toBe("detached");
  });

  test("distinguishes a previous attempt output from an explicitly detached output", () => {
    const item = taskItem([file("current"), file("detached")], []);
    const previous = projectTaskOutputVersions(item).find((version) => version.fileId === "detached");

    expect(previous?.state).toBe("historical");
  });

  test("filters and orders activity events by task operation", () => {
    const older = event("older", "current", "file_renamed", "2026-02-01T00:00:00Z");
    const newer = event("newer", "detached", "operation_output_detached", "2026-03-01T00:00:00Z");
    const unrelated = { ...event("other", "other", "file_deleted", "2026-04-01T00:00:00Z"), operationId: "operation-two", detail: { cause: { category: "file", operationId: "operation-two" } } };
    const item = taskItem([file("current"), file("detached")], [older, unrelated, newer]);

    expect(fileEventsForLibraryItem(item).map((value) => value.id)).toEqual(["newer", "older"]);
  });

  test("separates immutable operation events from the mutable task snapshot", () => {
    const item = taskItem([file("current")], []);
    item.library!.records.history.push({
      recordId: "resume-event",
      libraryId: "library-one",
      category: "operation_event",
      action: "operation_resumed",
      displayName: "Download stamps",
      status: "queued",
      source: { kind: "user_action" },
      refs: { operationId: "operation-one" },
      files: [],
      metrics: { fileCount: 0 },
      occurredAt: "2026-04-01T00:00:00Z",
      createdAt: "2026-04-01T00:00:00Z",
    });

    expect(operationHistoryEventsForTask(item).map((record) => record.action)).toEqual([
      "operation_resumed",
    ]);
  });

  test("reconstructs every immutable task rename as old to new", () => {
    const item = taskItem([file("current")], []);
    item.title = "Final title";
    item.operation!.name = "Final title";
    item.library!.records.history.push({
      recordId: "rename-one",
      libraryId: "library-one",
      category: "operation_event",
      action: "operation_renamed",
      displayName: "Download stamps",
      status: "succeeded",
      source: { kind: "user_action" },
      refs: { operationId: "operation-one" },
      metrics: { fileCount: 1 },
      occurredAt: "2026-03-01T00:00:00Z",
      createdAt: "2026-03-01T00:00:00Z",
    }, {
      recordId: "rename-two",
      libraryId: "library-one",
      category: "operation_event",
      action: "operation_renamed",
      displayName: "Second title",
      status: "succeeded",
      source: { kind: "user_action" },
      refs: { operationId: "operation-one" },
      metrics: { fileCount: 1 },
      occurredAt: "2026-04-01T00:00:00Z",
      createdAt: "2026-04-01T00:00:00Z",
    });

    expect(operationRenameTransitionsForTask(item)).toEqual([
      {
        recordId: "rename-two",
        before: "Second title",
        after: "Final title",
      },
      {
        recordId: "rename-one",
        before: "Download stamps",
        after: "Second title",
      },
    ]);
  });
});
