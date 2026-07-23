export type XiaSettingsTabId = "general" | "network" | "appearance" | "player" | "download" | "ai" | "about";

const STORAGE_KEY = "xiadown:settings-tab";

export function isSettingsTab(value: string | null | undefined): value is XiaSettingsTabId {
  return value === "general" || value === "network" || value === "appearance" || value === "player" || value === "download" || value === "ai" || value === "about";
}

export function resolveSettingsTab(value: string | null | undefined): XiaSettingsTabId {
  const normalized = value?.trim().toLowerCase();
  if (
    normalized === "proxy"
    || normalized === "network"
    || normalized === "library"
    || normalized === "library-access"
    || normalized === "library_access"
    || normalized === "libraryaccess"
    || normalized === "remote-access"
    || normalized === "tailscale"
  ) {
    return "network";
  }
  if (normalized === "equalizer") {
    return "player";
  }
  return isSettingsTab(normalized) ? normalized : "general";
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
