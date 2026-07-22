import { describe, expect, test } from "bun:test";

import {
  LIBRARY_VIEW_MODE_STORAGE_KEY,
  parseLibraryViewMode,
  readLibraryViewMode,
  resolveInitialLibraryViewMode,
  writeLibraryViewMode,
} from "./view-preference";

function memoryStorage(initial?: string) {
  const values = new Map<string, string>();
  if (initial !== undefined) values.set(LIBRARY_VIEW_MODE_STORAGE_KEY, initial);
  return {
    getItem(key: string) {
      return values.get(key) ?? null;
    },
    setItem(key: string, value: string) {
      values.set(key, value);
    },
  };
}

describe("Library view preference", () => {
  test("accepts only canonical view modes", () => {
    expect(parseLibraryViewMode("grid")).toBe("grid");
    expect(parseLibraryViewMode("list")).toBe("list");
    expect(parseLibraryViewMode("tiles")).toBeUndefined();
    expect(parseLibraryViewMode(null)).toBeUndefined();
  });

  test("restores the persisted mode and lets an explicit initial mode win", () => {
    const storage = memoryStorage("list");
    expect(readLibraryViewMode(storage)).toBe("list");
    expect(resolveInitialLibraryViewMode(undefined, storage)).toBe("list");
    expect(resolveInitialLibraryViewMode("grid", storage)).toBe("grid");
  });

  test("persists changes and safely falls back for invalid storage data", () => {
    const storage = memoryStorage("unknown");
    expect(resolveInitialLibraryViewMode(undefined, storage)).toBe("grid");
    writeLibraryViewMode("list", storage);
    expect(storage.getItem(LIBRARY_VIEW_MODE_STORAGE_KEY)).toBe("list");
  });
});
