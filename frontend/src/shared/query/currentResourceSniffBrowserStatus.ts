import type {
  CurrentResourceSniffBrowserState,
  CurrentResourceSniffBrowserStatus,
} from "@/shared/contracts/library";

const CURRENT_RESOURCE_SNIFF_BROWSER_STATES =
  new Set<CurrentResourceSniffBrowserState>([
    "ready",
    "not_installed",
    "not_running",
    "remote_debugging_disabled",
    "permission_denied",
    "unsupported_version",
    "endpoint_unavailable",
    "unsupported_browser",
  ]);

export function normalizeCurrentResourceSniffBrowserStatus(
  raw: unknown,
  fallbackBrowserId = "chrome",
): CurrentResourceSniffBrowserStatus {
  const value = raw && typeof raw === "object"
    ? raw as Record<string, unknown>
    : {};
  const rawState = typeof value.state === "string"
    ? value.state.trim().toLowerCase()
    : "";
  const state = CURRENT_RESOURCE_SNIFF_BROWSER_STATES.has(
    rawState as CurrentResourceSniffBrowserState,
  )
    ? rawState as CurrentResourceSniffBrowserState
    : "endpoint_unavailable";
  const minimumVersion = Number(value.minimumVersion);
  return {
    browserId:
      typeof value.browserId === "string" && value.browserId.trim()
        ? value.browserId.trim().toLowerCase()
        : fallbackBrowserId,
    state,
    installed: value.installed === true,
    running: value.running === true,
    supported: value.supported === true,
    ready: value.ready === true && state === "ready",
    version:
      typeof value.version === "string" && value.version.trim()
        ? value.version.trim()
        : undefined,
    minimumVersion:
      Number.isFinite(minimumVersion) && minimumVersion > 0
        ? minimumVersion
        : undefined,
    profileName:
      typeof value.profileName === "string" && value.profileName.trim()
        ? value.profileName.trim()
        : undefined,
  };
}
