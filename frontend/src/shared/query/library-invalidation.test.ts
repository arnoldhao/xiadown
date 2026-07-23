import { describe, expect, mock, test } from "bun:test";

mock.module("@wailsio/runtime", () => ({
  Call: { ByID: () => Promise.resolve(undefined), ByName: () => Promise.resolve(undefined) },
  Create: {
    Any: (value: unknown) => value,
    Array: (create: (value: unknown) => unknown) => (values: unknown[]) => values.map(create),
    Nullable: (create: (value: unknown) => unknown) => (value: unknown) => value == null ? value : create(value),
  },
}));

const {
  invalidateLibraryQueries,
  invalidateOperationQueries,
  LIBRARY_COMPLETE_OPERATIONS_QUERY_KEY,
  LIBRARY_DELETED_ITEMS_QUERY_KEY,
  LIBRARY_LIST_QUERY_KEY,
} = await import("./library");

describe("library mutation invalidation", () => {
  test("refreshes Catalog and legacy Library projections together", () => {
    const invalidated: unknown[][] = [];
    const queryClient = {
      invalidateQueries: ({ queryKey }: { queryKey: unknown[] }) => {
        invalidated.push(queryKey);
        return Promise.resolve();
      },
    };

    invalidateLibraryQueries(queryClient as never);

    expect(invalidated).toContainEqual(["catalog"]);
    expect(invalidated).toContainEqual([...LIBRARY_LIST_QUERY_KEY]);
  });

  test("refreshes task output projections after a task-file mutation settles", () => {
    const invalidated: unknown[][] = [];
    const queryClient = {
      invalidateQueries: ({ queryKey }: { queryKey: unknown[] }) => {
        invalidated.push(queryKey);
        return Promise.resolve();
      },
    };

    invalidateOperationQueries(queryClient as never, "library-one");

    expect(invalidated).toContainEqual(["catalog"]);
    expect(invalidated).toContainEqual([...LIBRARY_COMPLETE_OPERATIONS_QUERY_KEY]);
    expect(invalidated).toContainEqual([...LIBRARY_DELETED_ITEMS_QUERY_KEY]);
    expect(invalidated).toContainEqual(["library", "workspace", "library-one"]);
  });

  test("reconciles a Deleted restore mutation after every outcome", async () => {
    const source = await Bun.file(new URL("./library.ts", import.meta.url)).text();
    const start = source.indexOf("export function useRestoreDeletedLibraryItem");
    const end = source.indexOf(
      "export function useOpenLibraryPath",
      start,
    );
    const mutation = source.slice(start, end);

    expect(start).toBeGreaterThan(-1);
    expect(end).toBeGreaterThan(start);
    expect(mutation).toContain("onSettled:");
    expect(mutation).toContain("invalidateOperationQueries(queryClient)");
    expect(mutation).not.toContain("onSuccess:");
  });

  test("reconciles a Catalog restore after every outcome", async () => {
    const source = await Bun.file(new URL("./catalog.ts", import.meta.url)).text();
    const start = source.indexOf("export function useRestoreCatalogItem");
    const end = source.indexOf("export function useSaveCatalogTag", start);
    const mutation = source.slice(start, end);

    expect(start).toBeGreaterThan(-1);
    expect(end).toBeGreaterThan(start);
    expect(mutation).toContain("onSettled:");
    expect(mutation).toContain("refresh(request.id, detail)");
    expect(mutation).not.toContain("onSuccess:");
  });
});
