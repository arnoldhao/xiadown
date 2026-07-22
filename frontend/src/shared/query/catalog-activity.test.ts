import { describe, expect, mock, test } from "bun:test";
import { QueryClient } from "@tanstack/react-query";

const calls: Array<{ name: string; request: unknown }> = [];
const response = [{
  action: "catalog_item_restored",
  revision: 4,
  actor: "desktop-user",
  occurredAt: "2026-07-20T16:00:00Z",
}];

mock.module("@wailsio/runtime", () => ({
  Call: {
    ByID: () => Promise.resolve(undefined),
    ByName: (name: string, request: unknown) => {
      calls.push({ name, request });
      return Promise.resolve(response);
    },
  },
  Create: {
    Any: (value: unknown) => value,
    Array: (create: (value: unknown) => unknown) => (values: unknown[]) => values.map(create),
    Nullable: (create: (value: unknown) => unknown) => (value: unknown) => value == null ? value : create(value),
  },
}));

const {
  catalogKeys,
  listCatalogItemActivity,
  refreshCatalogItemQueries,
} = await import("./catalog");

describe("Catalog item activity query", () => {
  test("uses the handwritten Call.ByName contract and stable item-scoped key", async () => {
    calls.length = 0;
    const result = await listCatalogItemActivity({ itemId: "item-1", limit: 12 });

    expect(result).toEqual(response);
    expect(calls).toEqual([{
      name: "xiadown/internal/presentation/wails.CatalogHandler.ListCatalogItemActivity",
      request: { itemId: "item-1", limit: 12 },
    }]);
    expect(catalogKeys.activity("item-1", 12)).toEqual([
      "catalog", "item", "item-1", "activity", 12,
    ]);
  });

  test("refreshing item detail never overwrites activity-array cache", async () => {
    const queryClient = new QueryClient();
    const detailKey = catalogKeys.item("item-1", "");
    const activityKey = catalogKeys.activity("item-1", 12);
    const oldDetail = { item: { id: "item-1", title: "Old" } };
    const newDetail = { item: { id: "item-1", title: "New" } };
    queryClient.setQueryData(detailKey, oldDetail);
    queryClient.setQueryData(activityKey, response);

    await refreshCatalogItemQueries(queryClient, "item-1", newDetail as never);

    expect(queryClient.getQueryData(detailKey)).toEqual(newDetail);
    expect(queryClient.getQueryData(activityKey)).toEqual(response);
    expect(queryClient.getQueryState(activityKey)?.isInvalidated).toBe(true);
  });
});
