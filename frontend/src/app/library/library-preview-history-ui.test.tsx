import { describe, expect, mock, test } from "bun:test";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";

import type { LibraryDTO, OperationListItemDTO } from "@/shared/contracts/library";
import { t } from "@/shared/i18n";

import { createLibraryWorkspaceLabels, type LibraryWorkspaceItem } from "./types";

mock.module("@wailsio/runtime", () => ({
  Call: { ByID: () => Promise.resolve(undefined), ByName: () => Promise.resolve(undefined) },
  Create: {
    Any: (value: unknown) => value,
    Array: (create: (value: unknown) => unknown) => (values: unknown[]) => values.map(create),
    Nullable: (create: (value: unknown) => unknown) => (value: unknown) => value == null ? value : create(value),
  },
  Events: {
    On: () => () => {},
    Types: { Common: { WindowFullscreen: "window-fullscreen", WindowUnFullscreen: "window-unfullscreen" } },
  },
  Window: { Fullscreen: () => Promise.resolve(), UnFullscreen: () => Promise.resolve() },
}));

const { LibraryPreviewCompanion } = await import("./LibraryPreviewCompanion");

function taskItem(): LibraryWorkspaceItem {
  const operation: OperationListItemDTO = {
    operationId: "operation-one",
    libraryId: "library-one",
    name: "Stamp task",
    kind: "download",
    status: "succeeded",
    correlation: {},
    outputFiles: [{ fileId: "current", kind: "image", format: "webp" }],
    detachedOutputFileIds: ["detached"],
    metrics: { fileCount: 1 },
    createdAt: "2026-01-01T00:00:00Z",
    finishedAt: "2026-01-02T00:00:00Z",
  };
  const library: LibraryDTO = {
    version: "current",
    id: "library-one",
    name: "Library",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-03-01T00:00:00Z",
    createdBy: { source: "test" },
    files: [{
      id: "current",
      libraryId: "library-one",
      kind: "image",
      name: "current.webp",
      storage: { mode: "local_path", localPath: "/Library/current.webp" },
      origin: { kind: "download", operationId: "operation-one" },
      lineage: {},
      metadata: {},
      media: { format: "webp", sizeBytes: 4096 },
      state: { status: "available", deleted: false, archived: false },
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    }, {
      id: "detached",
      libraryId: "library-one",
      kind: "image",
      name: "detached.webp",
      storage: { mode: "local_path", localPath: "/Library/detached.webp" },
      origin: { kind: "download", operationId: "operation-one" },
      lineage: {},
      metadata: {},
      media: { format: "webp", sizeBytes: 2048 },
      state: { status: "available", deleted: false, archived: false },
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-03-01T00:00:00Z",
    }],
    records: {
      history: [{
        recordId: "history-one",
        libraryId: "library-one",
        category: "operation",
        action: "download",
        displayName: "Stamp task",
        status: "succeeded",
        source: { kind: "download" },
        refs: { operationId: "operation-one" },
        files: [
          { fileId: "current", kind: "image", format: "webp" },
          { fileId: "detached", kind: "image", format: "webp" },
        ],
        metrics: { fileCount: 2 },
        occurredAt: "2026-01-02T00:00:00Z",
        createdAt: "2026-01-01T00:00:00Z",
      }, {
        recordId: "resume-event",
        libraryId: "library-one",
        category: "operation_event",
        action: "operation_resumed",
        displayName: "Stamp task",
        status: "canceled",
        source: { kind: "user_action", actor: "desktop-library" },
        refs: { operationId: "operation-one" },
        files: [],
        metrics: { fileCount: 0 },
        occurredAt: "2026-02-01T00:00:00Z",
        createdAt: "2026-02-01T00:00:00Z",
      }],
      workspaceStates: [],
      fileEvents: [{
        id: "event-created",
        libraryId: "library-one",
        fileId: "current",
        operationId: "operation-one",
        eventType: "file_created",
        detail: {
          cause: { category: "download", operationId: "operation-one" },
          after: { fileId: "current", kind: "image", name: "current.webp" },
          changes: [{ field: "fileLifecycle", before: "absent", after: "active" }],
        },
        occurredAt: "2026-01-01T01:00:00Z",
        createdAt: "2026-01-01T01:00:00Z",
      }, {
        id: "event-detached",
        libraryId: "library-one",
        fileId: "detached",
        operationId: "operation-one",
        eventType: "operation_output_detached",
        detail: {
          cause: { category: "task_output", operationId: "operation-one" },
          before: { fileId: "detached", kind: "image", name: "detached.webp" },
          after: { fileId: "detached", kind: "image", name: "detached.webp" },
          changes: [{ field: "taskAssociation", before: "attached", after: "detached" }],
        },
        occurredAt: "2026-03-01T00:00:00Z",
        createdAt: "2026-03-01T00:00:00Z",
      }],
    },
  };
  return {
    id: "task:operation-one",
    source: "task",
    libraryId: "library-one",
    libraryName: "Library",
    title: "Stamp task",
    subtitle: "download",
    category: "task",
    status: "succeeded",
    format: "DOWNLOAD",
    createdAt: operation.createdAt,
    updatedAt: operation.finishedAt!,
    path: "https://example.com",
    coverURL: "cover.svg",
    rootId: operation.operationId,
    searchText: "stamp task",
    operation,
    library,
  };
}

describe("library preview history UI", () => {
  test("shows current and detached Task outputs in Versions", () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "zh-CN"), "zh-CN");
    const client = new QueryClient();
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={taskItem()} labels={labels} initialTab="versions" />
      </QueryClientProvider>,
    );

    expect(markup).toContain("current.webp");
    expect(markup).toContain("detached.webp");
    expect(markup).toContain(labels.outputCurrent);
    expect(markup).toContain(labels.outputDetached);
    expect(markup).not.toContain(labels.noVersions);
  });

  test("shows the immutable output-removal event in Activity", () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "zh-CN"), "zh-CN");
    const client = new QueryClient();
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={taskItem()} labels={labels} initialTab="activity" />
      </QueryClientProvider>,
    );

    expect(markup).toContain(labels.removeTaskOutputTitle);
    expect(markup).toContain(labels.operationResumed);
    expect(markup).toContain(`${labels.created} · ${labels.source}`);
    expect(markup).toContain("lucide-check");
    expect(markup).toContain("detached.webp");
    expect(markup).not.toContain("operation_output_detached");
  });

  test("shows both sides of an immutable task rename", () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "zh-CN"), "zh-CN");
    const item = taskItem();
    item.title = "New stamp task";
    item.operation!.name = "New stamp task";
    item.library!.records.history.push({
      recordId: "rename-event",
      libraryId: "library-one",
      category: "operation_event",
      action: "operation_renamed",
      displayName: "Stamp task",
      status: "succeeded",
      source: { kind: "user_action", actor: "desktop-library" },
      refs: { operationId: "operation-one" },
      files: [],
      metrics: { fileCount: 0 },
      occurredAt: "2026-04-01T00:00:00Z",
      createdAt: "2026-04-01T00:00:00Z",
    });
    const client = new QueryClient();
    const markup = renderToStaticMarkup(
      <QueryClientProvider client={client}>
        <LibraryPreviewCompanion item={item} labels={labels} initialTab="activity" />
      </QueryClientProvider>,
    );

    expect(markup).toContain("Stamp task → New stamp task");
    expect(markup).toContain(labels.operationRenamed);
    expect(markup).toContain("lucide-pencil-line");
  });
});
