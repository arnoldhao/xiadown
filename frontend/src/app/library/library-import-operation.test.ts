import { describe, expect, test } from "bun:test";

import type { LibraryImportBatch } from "@/shared/contracts/library-management";

import {
  LibraryImportOperationController,
  reconcileLibraryImportBatch,
  resolveLibraryImportResultNotice,
} from "./library-import-operation";

function batch(id: string, status: LibraryImportBatch["status"], updatedAt: string) {
  return {
    id,
    status,
    updatedAt,
    cancelRequested: status === "cancelling" || status === "cancelled",
  } as LibraryImportBatch;
}

describe("library import operation controller", () => {
  test("blocks history changes and another batch while an import is active", () => {
    const operations = new LibraryImportOperationController();
    const commit = operations.begin("commit", "batch-a");

    expect(commit).not.toBeNull();
    expect(operations.begin("select", "batch-b")).toBeNull();
    expect(operations.begin("resume", "batch-b")).toBeNull();
    expect(operations.isBusy()).toBeTrue();
  });

  test("allows one cancellation only for the executing batch", () => {
    const operations = new LibraryImportOperationController();
    const commit = operations.begin("commit", "batch-a");

    expect(commit).not.toBeNull();
    expect(operations.beginCancellation("batch-b")).toBeNull();
    const cancellation = operations.beginCancellation("batch-a");
    expect(cancellation?.lane).toBe("cancellation");
    expect(operations.beginCancellation("batch-a")).toBeNull();
    expect(operations.isCurrent(commit!)).toBeTrue();
    expect(operations.canAnnounce(commit!)).toBeFalse();

    expect(operations.settle(commit!)).toBeTrue();
    expect(operations.isBusy()).toBeTrue();
    expect(operations.begin("select", "batch-b")).toBeNull();
    expect(operations.settle(cancellation!)).toBeTrue();
    expect(operations.isBusy()).toBeFalse();
  });

  test("a stale settlement cannot clear a newer operation", () => {
    const operations = new LibraryImportOperationController();
    const first = operations.begin("select", "batch-a")!;
    expect(operations.settle(first)).toBeTrue();
    const second = operations.begin("select", "batch-b")!;

    expect(operations.settle(first)).toBeFalse();
    expect(operations.isCurrent(second)).toBeTrue();
    expect(operations.isBusy()).toBeTrue();
  });
});

describe("library import result reconciliation", () => {
  const labels = {
    scanReadyNotice: "scan ready",
    commitCompletedNotice: "completed",
    resumeCompletedNotice: "resumed",
    cancelRequestedNotice: "cancel requested",
  };

  test("never reports a cancelled commit as completed", () => {
    expect(resolveLibraryImportResultNotice("commit", batch("a", "cancelled", "2026-01-01T00:00:00Z"), labels)).toBe(
      "cancel requested",
    );
    expect(resolveLibraryImportResultNotice("commit", batch("a", "cancelling", "2026-01-01T00:00:00Z"), labels)).toBe(
      "cancel requested",
    );
    expect(resolveLibraryImportResultNotice("commit", batch("a", "completed", "2026-01-01T00:00:00Z"), labels)).toBe(
      "completed",
    );
  });

  test("applies but does not announce a completion response while cancellation is pending", () => {
    const operations = new LibraryImportOperationController();
    const commit = operations.begin("commit", "batch-a")!;
    const cancellation = operations.beginCancellation("batch-a")!;

    expect(operations.isCurrent(commit)).toBeTrue();
    expect(operations.canAnnounce(commit)).toBeFalse();
    expect(operations.isCurrent(cancellation)).toBeTrue();
    expect(resolveLibraryImportResultNotice("commit", batch("a", "completed", "2026-01-01T00:00:00Z"), labels)).toBe(
      "completed",
    );
    // The component checks isCurrent before applying the result or this notice.
    expect(operations.settle(commit)).toBeTrue();
    expect(operations.isCurrent(cancellation)).toBeTrue();
  });

  test("does not claim cancellation succeeded when the backend says completion won", () => {
    expect(resolveLibraryImportResultNotice(
      "cancel",
      batch("a", "completed", "2026-01-01T00:00:00Z"),
      labels,
    )).toBeNull();
  });

  test("does not let an older response replace a newer batch snapshot", () => {
    const newer = batch("batch-a", "cancelled", "2026-07-19T10:00:02Z");
    const older = batch("batch-a", "running", "2026-07-19T10:00:01Z");

    expect(reconcileLibraryImportBatch(newer, older)).toBe(newer);
    expect(reconcileLibraryImportBatch(older, newer)).toBe(newer);
  });
});
