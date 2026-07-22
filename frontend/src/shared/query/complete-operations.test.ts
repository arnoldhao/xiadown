import { describe, expect, test } from "bun:test";

import type { OperationListItemDTO } from "@/shared/contracts/library";

import {
  collectCompleteOperations,
  shouldRefreshCompleteOperations,
  sortAndDedupeOperations,
} from "./complete-operations";

function operation(index: number, overrides: Partial<OperationListItemDTO> = {}): OperationListItemDTO {
  return {
    operationId: `operation-${String(index).padStart(4, "0")}`,
    libraryId: "library-1",
    name: `Operation ${index}`,
    kind: "download",
    status: "succeeded",
    correlation: {},
    metrics: { fileCount: 1 },
    createdAt: new Date(Date.UTC(2026, 0, 1, 0, index)).toISOString(),
    ...overrides,
  };
}

describe("complete operation history pagination", () => {
  test("keeps the heavy history query independent and enables it only for Library Tasks", async () => {
    const [querySource, providerSource, mainSource] = await Promise.all([
      Bun.file(new URL("./library.ts", import.meta.url)).text(),
      Bun.file(new URL("../../app/providers/AppProviders.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../app/main/MainApp.tsx", import.meta.url)).text(),
    ]);
    expect(querySource).toContain('LIBRARY_COMPLETE_OPERATIONS_QUERY_KEY = ["library", "complete-operations"]');
    expect(querySource).toContain("enabled: options.enabled === true");
    expect(querySource).toContain("refetchOnWindowFocus: false");
    expect(mainSource).toMatch(/useCompleteOperations\(\{[\s\S]*?libraryWorkspaceRoute === "tasks"/);
    expect(providerSource).toContain("shouldRefreshCompleteOperations(event)");
    expect(providerSource).toContain("scheduleCompleteOperationsRefresh()");
  });

  test("loads, deduplicates and sorts more than 500 operations", async () => {
    const source = Array.from({ length: 625 }, (_, index) => operation(index));
    const requests: Array<{ limit?: number; offset?: number }> = [];

    const result = await collectCompleteOperations({}, async (request) => {
      requests.push({ limit: request.limit, offset: request.offset });
      return source.slice(request.offset, (request.offset ?? 0) + (request.limit ?? 0));
    });

    expect(requests).toEqual([
      { limit: 500, offset: 0 },
      { limit: 500, offset: 492 },
    ]);
    expect(result).toHaveLength(625);
    expect(new Set(result.map((item) => item.operationId)).size).toBe(625);
    expect(result[0]?.operationId).toBe("operation-0624");
    expect(result.at(-1)?.operationId).toBe("operation-0000");
  });

  test("fails explicitly instead of returning a silently truncated safety-limit page", async () => {
    await expect(collectCompleteOperations(
      {},
      async (request) => [
        operation((request.offset ?? 0) * 2 + 1),
        operation((request.offset ?? 0) * 2 + 2),
      ],
      { pageSize: 2, maxPages: 2 },
    )).rejects.toThrow("pagination safety limit");
  });

  test("refreshes complete history only for list-semantic realtime changes", () => {
    expect(shouldRefreshCompleteOperations({ type: "delete", payload: { id: "operation-1" } })).toBe(true);
    expect(shouldRefreshCompleteOperations({ type: "upsert", payload: { status: "queued" } })).toBe(true);
    expect(shouldRefreshCompleteOperations({ type: "upsert", payload: { status: "succeeded" } })).toBe(true);
    expect(shouldRefreshCompleteOperations({ type: "upsert", payload: { status: "running" } })).toBe(false);
  });

  test("detects a backend page that stalls instead of looping over duplicates", async () => {
    const repeated = Array.from({ length: 10 }, (_, index) => operation(index));
    await expect(collectCompleteOperations(
      {},
      async () => repeated,
      { pageSize: 10, maxPages: 10 },
    )).rejects.toThrow("no forward progress");
  });

  test("lets fresher live snapshots replace the same operation from history", () => {
    const history = operation(1, { status: "queued" });
    const live = operation(1, { status: "running" });
    expect(sortAndDedupeOperations([[history], [live]])[0]?.status).toBe("running");
  });
});
