export type LibraryViewMode = "grid" | "list";

export const LIBRARY_VIEW_MODE_STORAGE_KEY = "xiadown.library.view-mode.v1";

type ReadableStorage = Pick<Storage, "getItem">;
type WritableStorage = Pick<Storage, "setItem">;

function browserStorage(): Storage | undefined {
  if (typeof window === "undefined") return undefined;
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

export function parseLibraryViewMode(value: unknown): LibraryViewMode | undefined {
  return value === "grid" || value === "list" ? value : undefined;
}

export function readLibraryViewMode(
  storage: ReadableStorage | undefined = browserStorage(),
): LibraryViewMode | undefined {
  if (!storage) return undefined;
  try {
    return parseLibraryViewMode(storage.getItem(LIBRARY_VIEW_MODE_STORAGE_KEY));
  } catch {
    return undefined;
  }
}

export function resolveInitialLibraryViewMode(
  initialView?: LibraryViewMode,
  storage?: ReadableStorage,
): LibraryViewMode {
  return initialView ?? readLibraryViewMode(storage ?? browserStorage()) ?? "grid";
}

export function writeLibraryViewMode(
  viewMode: LibraryViewMode,
  storage: WritableStorage | undefined = browserStorage(),
) {
  if (!storage) return;
  try {
    storage.setItem(LIBRARY_VIEW_MODE_STORAGE_KEY, viewMode);
  } catch {
    // A view preference must never make Library unusable when storage is
    // disabled, full, or unavailable in an embedded WebView.
  }
}
