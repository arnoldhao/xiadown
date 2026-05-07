export type XiaSettingsTabId = "general" | "appearance" | "pets" | "dependencies" | "about";

const STORAGE_KEY = "xiadown:settings-tab";

export function isSettingsTab(value: string | null | undefined): value is XiaSettingsTabId {
  return value === "general" || value === "appearance" || value === "pets" || value === "dependencies" || value === "about";
}

export function resolveSettingsTab(value: string | null | undefined): XiaSettingsTabId {
  if (value === "proxy") {
    return "general";
  }
  return isSettingsTab(value) ? value : "general";
}

export function setPendingSettingsTab(tab: XiaSettingsTabId) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, tab);
  } catch {
    // ignore storage errors
  }
}

export function consumePendingSettingsTab(): XiaSettingsTabId | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    const tab = isSettingsTab(stored) ? stored : null;
    if (stored) {
      window.localStorage.removeItem(STORAGE_KEY);
    }
    return tab;
  } catch {
    return null;
  }
}

export function listenPendingSettingsTab(onTab: (tab: XiaSettingsTabId) => void) {
  if (typeof window === "undefined") {
    return () => undefined;
  }

  const handler = (event: StorageEvent) => {
    if (event.key !== STORAGE_KEY) {
      return;
    }
    const tab = isSettingsTab(event.newValue) ? event.newValue : null;
    if (!tab) {
      return;
    }
    onTab(tab);
    try {
      window.localStorage.removeItem(STORAGE_KEY);
    } catch {
      // ignore storage errors
    }
  };

  window.addEventListener("storage", handler);
  return () => window.removeEventListener("storage", handler);
}
