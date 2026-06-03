export type XiaSettingsTabId = "general" | "appearance" | "player" | "download" | "about";

const STORAGE_KEY = "xiadown:settings-tab";

export function isSettingsTab(value: string | null | undefined): value is XiaSettingsTabId {
  return value === "general" || value === "appearance" || value === "player" || value === "download" || value === "about";
}

export function resolveSettingsTab(value: string | null | undefined): XiaSettingsTabId {
  if (value === "proxy") {
    return "general";
  }
  if (value === "equalizer") {
    return "player";
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
    const tab = stored ? resolveSettingsTab(stored) : null;
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
    const tab = event.newValue ? resolveSettingsTab(event.newValue) : null;
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
