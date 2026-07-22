import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Call } from "@wailsio/runtime";

import type {
  AppSessionBrowserImportRequest,
  AppSessionBrowserImportResult,
  AppSessionBrowserScanItem,
  AppSessionBrowserScanResult,
  BrowserSourceBrowser,
  BrowserSourceCatalog,
  BrowserSourceIntent,
  BrowserSourceProfile,
  BrowserSourceSelection,
  SniffProfileRequest,
} from "@/shared/contracts/browserSources";

import { loadAppSessionsHandlerBindings } from "./appSessionsBindings";

const SETTINGS_HANDLER = "xiadown/internal/presentation/wails.SettingsHandler";
const APP_SESSION_BROWSER_IDS = new Set([
  "chrome",
  "safari",
  "edge",
  "brave",
  "arc",
  "vivaldi",
  "opera",
]);

export const BROWSER_SOURCE_QUERY_KEY = ["browser-sources"] as const;
export const APP_SESSION_BROWSER_PROFILE_SOURCES_QUERY_KEY = ["app-session-browser-profile-sources"] as const;
export const SNIFF_PROFILES_QUERY_KEY = ["settings", "sniff-profiles"] as const;
const APP_SESSIONS_QUERY_KEY = ["app-sessions"] as const;

function arrayValue(value: unknown, key: string): unknown[] {
  if (Array.isArray(value)) {
    return value;
  }
  if (value && typeof value === "object") {
    const nested = (value as Record<string, unknown>)[key];
    return Array.isArray(nested) ? nested : [];
  }
  return [];
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function numberValue(value: unknown): number | undefined {
  const result = Number(value);
  return Number.isFinite(result) && result >= 0 ? result : undefined;
}

export function normalizeBrowserSourceProfile(
  raw: unknown,
  fallbackBrowserId = "",
): BrowserSourceProfile | null {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const value = raw as Record<string, unknown>;
  const id = stringValue(value.id ?? value.profileId ?? value.name);
  if (!id) {
    return null;
  }
  const state = stringValue(value.state).toLowerCase();
  const unavailableState = new Set([
    "missing",
    "unavailable",
    "permission_required",
    "permission_denied",
    "no_profile_data",
    "invalid_profile_data",
    "browser_running",
    "access_required",
    "protected_unsupported",
  ]).has(state);
  return {
    id,
    label: stringValue(value.label ?? value.displayName ?? value.name),
    browserId: stringValue(value.browserId ?? value.browser ?? fallbackBrowserId),
    browserLabel: stringValue(value.browserLabel),
    subtitle: stringValue(value.subtitle ?? value.description),
    displayPath: stringValue(value.displayPath),
    path: stringValue(value.path ?? value.directory),
    isDefault: value.isDefault === true || value.default === true,
    redundant: value.redundant === true,
    available: value.available !== false && !unavailableState,
    error: stringValue(value.error ?? value.reason),
    sizeBytes: numberValue(value.sizeBytes ?? value.totalBytes),
    state,
  };
}

export type BrowserProfileAvailabilityReason =
  | ""
  | "permission_required"
  | "no_profile_data"
  | "invalid_profile_data"
  | "browser_running"
  | "access_required"
  | "protected_unsupported"
  | "unavailable";

export function browserProfileAvailabilityReason(
  profile: BrowserSourceProfile,
): BrowserProfileAvailabilityReason {
  if (profile.available) {
    return "";
  }
  switch (profile.state?.trim().toLowerCase()) {
    case "permission_required":
    case "permission_denied":
      return "permission_required";
    case "no_profile_data":
    case "missing":
      return "no_profile_data";
    case "invalid_profile_data":
      return "invalid_profile_data";
    case "browser_running":
      return "browser_running";
    case "access_required":
      return "access_required";
    case "protected_unsupported":
      return "protected_unsupported";
    default:
      return "unavailable";
  }
}

function normalizeProfiles(raw: unknown, fallbackBrowserId = "") {
  return arrayValue(raw, "profiles")
    .map((item) => normalizeBrowserSourceProfile(item, fallbackBrowserId))
    .filter((item): item is BrowserSourceProfile => item !== null);
}

function normalizeBrowsers(raw: unknown): BrowserSourceBrowser[] {
  return arrayValue(raw, "browsers")
    .map((item): BrowserSourceBrowser | null => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const value = item as Record<string, unknown>;
      const id = stringValue(value.id ?? value.browserId);
      const label = stringValue(value.label ?? value.name);
      if (!id || !label) {
        return null;
      }
      const profiles = normalizeProfiles(value.profiles, id);
      return {
        id,
        label,
        available: value.available !== false,
        state: stringValue(value.state),
        error: stringValue(value.error ?? value.reason),
        profiles,
      };
    })
    .filter((item): item is BrowserSourceBrowser => item !== null)
    .sort((left, right) => left.label.localeCompare(right.label));
}

export async function listSniffProfiles(): Promise<BrowserSourceProfile[]> {
  const result = await Call.ByName(`${SETTINGS_HANDLER}.ListSniffProfiles`);
  return normalizeProfiles(result);
}

export async function listBrowserSources(
  intent: BrowserSourceIntent,
  refresh = false,
): Promise<BrowserSourceCatalog> {
  const candidatesMethod = refresh
    ? "RefreshBrowserCandidates"
    : "GetBrowserCandidates";
  const [browserResult, sniffProfiles] = await Promise.all([
    Call.ByName(`${SETTINGS_HANDLER}.${candidatesMethod}`),
    listSniffProfiles(),
  ]);
  const browsers = normalizeBrowsers(browserResult);
  browsers.sort((left, right) => left.label.localeCompare(right.label));
  const xiadownProfiles = sniffProfiles.length > 0
    ? sniffProfiles
    : (() => {
        const browser =
          browsers.find((item) => item.available && item.id === "chrome") ??
          browsers.find((item) => item.available);
        return browser
          ? [{
              id: "",
              browserId: browser.id,
              label: browser.label,
              isDefault: true,
              available: true,
              virtual: true,
            }]
          : [];
      })();
  return {
    browsers:
      intent === "app_session"
        ? browsers.filter((browser) => APP_SESSION_BROWSER_IDS.has(browser.id))
        : browsers,
    xiadownProfiles,
  };
}

export function useBrowserSources(intent: BrowserSourceIntent, enabled: boolean) {
  return useQuery({
    queryKey: [...BROWSER_SOURCE_QUERY_KEY, intent],
    queryFn: () => listBrowserSources(intent),
    enabled,
    staleTime: 10_000,
    refetchOnWindowFocus: false,
  });
}

export function useRefreshBrowserSources(intent: BrowserSourceIntent) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => listBrowserSources(intent, true),
    onSuccess: (catalog) => {
      queryClient.setQueryData([...BROWSER_SOURCE_QUERY_KEY, intent], catalog);
    },
  });
}

export function normalizeDiscoveredBrowserProfiles(
  raw: unknown,
  browserId: string,
  fallbackLabel: string,
): BrowserSourceBrowser {
  const normalizedBrowserId = stringValue(browserId).toLowerCase();
  const discovery = raw && typeof raw === "object" && !Array.isArray(raw)
    ? raw as Record<string, unknown>
    : undefined;
  const matchingProfiles = normalizeProfiles(discovery?.profiles ?? raw, normalizedBrowserId).filter(
    (profile) => profile.browserId === normalizedBrowserId,
  );
  const availableProfiles = matchingProfiles.filter((profile) => profile.available);
  const unavailableProfile =
    matchingProfiles.find(
      (profile) => browserProfileAvailabilityReason(profile) === "access_required",
    ) ??
    matchingProfiles.find(
      (profile) => browserProfileAvailabilityReason(profile) === "protected_unsupported",
    ) ??
    matchingProfiles.find(
      (profile) => browserProfileAvailabilityReason(profile) === "permission_required",
    ) ??
    matchingProfiles.find(
      (profile) => browserProfileAvailabilityReason(profile) === "browser_running",
    ) ??
    matchingProfiles.find(
      (profile) => browserProfileAvailabilityReason(profile) === "invalid_profile_data",
    ) ??
    matchingProfiles.find((profile) => !profile.available);
  const hasAvailableProfile = availableProfiles.length > 0;
  return {
    id: normalizedBrowserId,
    label:
      stringValue(discovery?.browserLabel) ||
      matchingProfiles.find((profile) => profile.browserLabel)?.browserLabel ||
      fallbackLabel ||
      normalizedBrowserId,
    available: hasAvailableProfile,
    state: hasAvailableProfile
      ? "ready"
      : stringValue(discovery?.state) || unavailableProfile?.state || "no_profile_data",
    error: hasAvailableProfile
      ? ""
      : stringValue(discovery?.error) || unavailableProfile?.error,
    profiles: matchingProfiles,
  };
}

export function useDiscoverAppSessionBrowserProfiles() {
  return useMutation({
    mutationFn: async (input: { browserId: string; browserLabel: string }) => {
      const bindings = await loadAppSessionsHandlerBindings();
      return normalizeDiscoveredBrowserProfiles(
        await bindings.DiscoverBrowserProfiles({ browserId: input.browserId }),
        input.browserId,
        input.browserLabel,
      );
    },
  });
}

export function normalizeAppSessionBrowserProfileSources(raw: unknown): BrowserSourceBrowser[] {
  return arrayValue(raw, "sources")
    .map((item): BrowserSourceBrowser | null => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const source = item as Record<string, unknown>;
      const id = stringValue(source.id ?? source.browserId).toLowerCase();
      const label = stringValue(source.label ?? source.browserLabel);
      if (!id || !label) {
        return null;
      }
      return {
        id,
        label,
        available: source.available !== false,
        error: stringValue(source.error),
        profiles: [],
      };
    })
    .filter((item): item is BrowserSourceBrowser => item !== null);
}

export function useAppSessionBrowserProfileSources(enabled: boolean) {
  return useQuery({
    queryKey: APP_SESSION_BROWSER_PROFILE_SOURCES_QUERY_KEY,
    queryFn: async () => {
      const bindings = await loadAppSessionsHandlerBindings();
      return normalizeAppSessionBrowserProfileSources(
        await bindings.ListBrowserProfileSources(),
      );
    },
    enabled,
    staleTime: 10_000,
    refetchOnWindowFocus: false,
  });
}

export function useOpenBrowserDataPermissionGuide() {
  return useMutation({
    mutationFn: async () => {
      const bindings = await loadAppSessionsHandlerBindings();
      return bindings.OpenBrowserDataPermissionGuide();
    },
  });
}

function normalizeScanStatus(value: unknown): AppSessionBrowserScanItem["status"] {
  switch (stringValue(value).toLowerCase()) {
    case "new":
    case "replace":
    case "unchanged":
    case "unavailable":
      return stringValue(value).toLowerCase() as AppSessionBrowserScanItem["status"];
    case "overwrite":
    case "existing":
      return "replace";
    case "same":
      return "unchanged";
    default:
      return "unavailable";
  }
}

function normalizeScanReason(value: unknown): string {
  switch (stringValue(value).toLowerCase()) {
    case "no_auth_cookies":
      return "no_auth_cookies";
    case "source_unavailable":
      return "source_unavailable";
    case "browser_cookie_access_required":
      return "browser_cookie_access_required";
    case "protected_cookies_unsupported":
      return "protected_cookies_unsupported";
    default:
      return "";
  }
}

export function normalizeAppSessionBrowserScanResult(
  raw: unknown,
  selection: BrowserSourceSelection,
): AppSessionBrowserScanResult {
  const value = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  const items = arrayValue(value.items ?? value.sessions, "items")
    .map((item): AppSessionBrowserScanItem | null => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const candidate = item as Record<string, unknown>;
      const appSessionId = stringValue(candidate.appSessionId ?? candidate.id);
      if (!appSessionId) {
        return null;
      }
      const status = normalizeScanStatus(candidate.status ?? candidate.state);
      return {
        appSessionId,
        siteKey: stringValue(candidate.siteKey),
        label: stringValue(candidate.label),
        accountLabel: stringValue(candidate.accountLabel ?? candidate.accountName),
        status,
        selectable: candidate.selectable !== false && status !== "unavailable",
        reason: normalizeScanReason(candidate.reason ?? candidate.error),
      };
    })
    .filter((item): item is AppSessionBrowserScanItem => item !== null);
  return {
    browserId: stringValue(value.browserId) || selection.browserId,
    profileId: stringValue(value.profileId) || selection.profileId,
    snapshotToken: stringValue(value.snapshotToken),
    items,
  };
}

export async function scanBrowserAppSessions(
  selection: BrowserSourceSelection,
): Promise<AppSessionBrowserScanResult> {
  const bindings = await loadAppSessionsHandlerBindings();
  const result = await bindings.ScanBrowserProfile(selection);
  return normalizeAppSessionBrowserScanResult(result, selection);
}

export function useImportBrowserAppSessions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (
      request: AppSessionBrowserImportRequest,
    ): Promise<AppSessionBrowserImportResult> => {
      const bindings = await loadAppSessionsHandlerBindings();
      const raw = await bindings.ImportBrowserProfile(request);
      return {
        importedIds: (raw.importedIds ?? []).map(stringValue).filter(Boolean),
        skippedIds: (raw.skippedIds ?? []).map(stringValue).filter(Boolean),
      };
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: APP_SESSIONS_QUERY_KEY });
    },
  });
}

export async function createSniffProfile(request: SniffProfileRequest) {
  return Call.ByName(`${SETTINGS_HANDLER}.CreateSniffProfile`, request);
}

export async function renameSniffProfile(request: SniffProfileRequest) {
  return Call.ByName(`${SETTINGS_HANDLER}.RenameSniffProfile`, request);
}

export async function deleteSniffProfile(request: SniffProfileRequest) {
  return Call.ByName(`${SETTINGS_HANDLER}.DeleteSniffProfile`, request);
}

export async function clearSniffProfile(request: SniffProfileRequest) {
  return Call.ByName(`${SETTINGS_HANDLER}.ClearSniffProfile`, request);
}

export async function openSniffProfile(request: SniffProfileRequest) {
  return Call.ByName(`${SETTINGS_HANDLER}.OpenSniffProfile`, request);
}
