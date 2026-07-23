import { describe, expect, test } from "bun:test";

import {
  normalizeDataManagementSnapshot,
  settleDataManagementCleanResults,
} from "./dataManagement";

describe("data management adapter", () => {
  test("normalizes a safety-aware snapshot", () => {
    const result = normalizeDataManagementSnapshot({
      totalBytes: 4096,
      safeReclaimableBytes: 1024,
      scannedAt: "2026-07-17T10:00:00Z",
      categories: [
        {
          id: "obsolete",
          totalBytes: 32,
          items: [
            { id: "legacy.app-sessions", sizeBytes: 32 },
            {
              id: "legacy.sniff-profiles",
              sizeBytes: 0,
              itemCount: 0,
              state: "empty",
            },
          ],
        },
        {
          id: "unknown-category",
          totalBytes: 2048,
          items: [{ id: "must-not-render", sizeBytes: 2048 }],
        },
        {
          id: "reclaimable",
          totalBytes: 1024,
          items: [
            {
              id: "caches",
              sizeBytes: 1024,
              itemCount: 3,
              risk: "safe",
              clearable: true,
            },
          ],
        },
      ],
    });

    expect(result.totalBytes).toBe(4096);
    expect(result.safeReclaimableBytes).toBe(1024);
    expect(result.categories.map((category) => category.id)).toEqual([
      "core",
      "reclaimable",
      "obsolete",
    ]);
    expect(result.categories[0]).toMatchObject({
      id: "core",
      totalBytes: 0,
      items: [],
    });
    expect(result.categories[1]?.items[0]).toMatchObject({
      id: "caches",
      sizeBytes: 1024,
      itemCount: 3,
      risk: "safe",
      clearable: true,
    });
    expect(
      result.categories.flatMap((category) => category.items).some(
        (item) => item.id === "must-not-render",
      ),
    ).toBe(false);
    expect(
      result.categories.flatMap((category) => category.items).some(
        (item) => item.id === "legacy.sniff-profiles",
      ),
    ).toBe(false);
  });

  test("keeps all three categories when the backend omits empty categories", () => {
    const result = normalizeDataManagementSnapshot({ categories: [] });

    expect(result.categories).toEqual([
      { id: "core", totalBytes: 0, items: [] },
      { id: "reclaimable", totalBytes: 0, items: [] },
      { id: "obsolete", totalBytes: 0, items: [] },
    ]);
  });

  test("settles successful cleanup while retaining failed and missing results", () => {
    expect(
      settleDataManagementCleanResults(
        ["image-cache", "rss-cache", "archived-logs", "legacy.app-sessions"],
        [
          { resourceId: "image-cache", status: "cleared", bytesFreed: 128 },
          { resourceId: "rss-cache", status: "failed", bytesFreed: 0 },
          { resourceId: "archived-logs", status: "denied", bytesFreed: 0 },
          { resourceId: "unrequested", status: "cleared", bytesFreed: 64 },
        ],
      ),
    ).toEqual({
      succeededIds: ["image-cache"],
      failedIds: ["rss-cache", "archived-logs", "legacy.app-sessions"],
    });
  });

  test("treats already-missing resources as successfully cleaned", () => {
    expect(
      settleDataManagementCleanResults(
        ["favicon-cache"],
        [{ resourceId: "favicon-cache", status: "already_missing", bytesFreed: 0 }],
      ),
    ).toEqual({ succeededIds: ["favicon-cache"], failedIds: [] });
  });
});
