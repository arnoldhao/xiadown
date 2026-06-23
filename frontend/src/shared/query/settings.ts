import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { Call, Events } from "@wailsio/runtime";

import type { BrowserCandidate, ProxySettings, Settings, SniffProfileInfo, SniffProfileRequest, SystemProxyInfo, UpdateSettingsRequest } from "@/shared/contracts/settings";
import { normalizeColorScheme } from "@/lib/theme/color-schemes";
import {
  GetBrowserCandidates,
  OpenLogDirectory,
  RefreshSystemProxy,
  RefreshBrowserCandidates,
  SelectDownloadDirectory,
  HideSettingsWindow,
  ShowMainWindow,
  ShowSettingsWindow,
  TestProxy,
  GetSniffProfileInfo,
  OpenSniffProfile,
  ClearSniffProfile,
} from "../../../bindings/xiadown/internal/presentation/wails/settingshandler";
import {
  Proxy as BindingsProxy,
  SniffProfileRequest as BindingsSniffProfileRequest,
  SystemProxyInfo as BindingsSystemProxyInfo,
} from "../../../bindings/xiadown/internal/application/settings/dto/models";

export const SETTINGS_QUERY_KEY = ["settings"];
export const BROWSER_CANDIDATES_QUERY_KEY = ["browser-candidates"];
export const SNIFF_PROFILE_QUERY_KEY = ["settings", "sniff-profile"];

export function useSettings() {
  return useQuery({
    queryKey: SETTINGS_QUERY_KEY,
    queryFn: async (): Promise<Settings> => {
      const result = await Call.ByName("xiadown/internal/presentation/wails.SettingsHandler.GetSettings");
      return toSettings(result as Partial<Settings>);
    },
    staleTime: Infinity,
  });
}

export function useUpdateSettings() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: UpdateSettingsRequest): Promise<Settings> => {
      const payload = toSettingsUpdatePayload(request);
      const result = await Call.ByName("xiadown/internal/presentation/wails.SettingsHandler.UpdateSettings", payload);
      return toSettings(result as Partial<Settings>);
    },
    onSuccess: (data) => {
      setLatestSettingsQueryData(queryClient, data);
      void Events.Emit("settings:updated", data);
    },
  });
}

export function useBrowserCandidates() {
  return useQuery({
    queryKey: BROWSER_CANDIDATES_QUERY_KEY,
    queryFn: async (): Promise<BrowserCandidate[]> => {
      return fetchBrowserCandidates(GetBrowserCandidates);
    },
    staleTime: 10_000,
  });
}

export function useRefreshBrowserCandidates() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (): Promise<BrowserCandidate[]> => {
      return fetchBrowserCandidates(RefreshBrowserCandidates);
    },
    onSuccess: (data) => {
      queryClient.setQueryData(BROWSER_CANDIDATES_QUERY_KEY, data);
    },
  });
}

export function setLatestSettingsQueryData(queryClient: QueryClient, raw: Partial<Settings> | Settings): Settings | null {
  const next = toSettings(raw as Partial<Settings>);
  let applied = false;

  queryClient.setQueryData(SETTINGS_QUERY_KEY, (current: Settings | undefined) => {
    if (!shouldAdoptSettingsSnapshot(current, next)) {
      return current;
    }
    applied = true;
    return next;
  });

  return applied ? next : null;
}

export function useShowSettingsWindow() {
  return useMutation({
    mutationFn: async () => {
      await ShowSettingsWindow();
    },
  });
}

export function useShowMainWindow() {
  return useMutation({
    mutationFn: async () => {
      await ShowMainWindow();
    },
  });
}

export function useHideSettingsWindow() {
  return useMutation({
    mutationFn: async () => {
      await HideSettingsWindow();
    },
  });
}

export async function setWelcomeWindowChromeHidden(hidden: boolean) {
  await Call.ByName("xiadown/internal/presentation/wails.SettingsHandler.SetWelcomeWindowChromeHidden", hidden);
}

export function useOpenLogDirectory() {
  return useMutation({
    mutationFn: async () => {
      await OpenLogDirectory();
    },
  });
}

export function useSelectDownloadDirectory() {
  return useMutation({
    mutationFn: async (title: string): Promise<string> => {
      return SelectDownloadDirectory(title);
    },
  });
}

export function useTestProxy() {
  return useMutation({
    mutationFn: async (proxyConfig: ProxySettings): Promise<ProxySettings> => {
      return toProxySettings(await TestProxy(BindingsProxy.createFrom(proxyConfig)));
    },
  });
}

export function useSystemProxyInfo(enabled = true) {
  return useQuery({
    queryKey: ["system-proxy"],
    queryFn: async (): Promise<SystemProxyInfo> => {
      return toSystemProxyInfo(await RefreshSystemProxy());
    },
    enabled,
  });
}

export function useSniffProfileInfo(browser?: string) {
  const normalizedBrowser = stringOrEmpty(browser);
  return useQuery({
    queryKey: [...SNIFF_PROFILE_QUERY_KEY, normalizedBrowser],
    queryFn: async (): Promise<SniffProfileInfo> => {
      const result = await GetSniffProfileInfo(BindingsSniffProfileRequest.createFrom({ browser: normalizedBrowser }));
      return {
        browser: stringOrEmpty(result.browser),
        exists: result.exists === true,
        sizeBytes: Number(result.sizeBytes ?? 0),
        fileCount: Number(result.fileCount ?? 0),
        directoryCount: Number(result.directoryCount ?? 0),
        truncated: result.truncated === true,
        error: stringOrEmpty(result.error),
      };
    },
    staleTime: 5_000,
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
    refetchInterval: 10_000,
  });
}

export function useOpenSniffProfile() {
  return useMutation({
    mutationFn: async (request: SniffProfileRequest): Promise<void> => {
      await OpenSniffProfile(BindingsSniffProfileRequest.createFrom(request));
    },
  });
}

export function useClearSniffProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (request: SniffProfileRequest): Promise<void> => {
      await ClearSniffProfile(BindingsSniffProfileRequest.createFrom(request));
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: [...SNIFF_PROFILE_QUERY_KEY, stringOrEmpty(variables.browser)] });
    },
  });
}

function toSettings(raw: Partial<Settings>): Settings {
  return {
    ...(raw as Settings),
    appearance: normalizeAppearanceMode(raw.appearance ?? "auto"),
    sniffBrowser: stringOrEmpty(raw.sniffBrowser),
    colorScheme: normalizeColorScheme(raw.colorScheme),
    menuBarVisibility: normalizeMenuBarVisibility(raw.menuBarVisibility ?? "whenRunning"),
    syncedLyricsEnabled: raw.syncedLyricsEnabled !== false,
    romanizedLyrics: raw.romanizedLyrics !== false,
    pinyinLyrics: raw.pinyinLyrics !== false,
    playbackAudioQuality: normalizePlaybackAudioQuality(raw.playbackAudioQuality),
    resourceSniffScope: normalizeResourceSniffScope(raw.resourceSniffScope),
    resourceSniffMinBytes: normalizeResourceSniffMinBytes(raw.resourceSniffMinBytes),
    resourceSniffRetain: normalizeResourceSniffRetain(raw.resourceSniffRetain),
    ytdlpConcurrentDownloads: normalizeYTDLPConcurrentDownloads(raw.ytdlpConcurrentDownloads),
    ytdlpConcurrentFragments: normalizeYTDLPConcurrentFragments(raw.ytdlpConcurrentFragments),
    mainBounds: { ...(raw.mainBounds ?? { x: 0, y: 0, width: 0, height: 0 }) },
    settingsBounds: { ...(raw.settingsBounds ?? { x: 0, y: 0, width: 0, height: 0 }) },
    proxy: toProxySettings(BindingsProxy.createFrom(raw.proxy ?? {})),
    appearanceConfig: raw.appearanceConfig,
  };
}

function toSettingsUpdatePayload(request: UpdateSettingsRequest) {
  return { ...request };
}

async function fetchBrowserCandidates(fetcher: () => Promise<unknown>) {
  const result = await fetcher();
  return toBrowserCandidates(Array.isArray(result) ? result : []);
}

function toBrowserCandidates(raw: unknown[]): BrowserCandidate[] {
  return raw
    .map((item) => {
      const candidate = item as Partial<BrowserCandidate>;
      return {
        id: stringOrEmpty(candidate.id),
        label: stringOrEmpty(candidate.label),
        execPath: stringOrEmpty(candidate.execPath),
        available: candidate.available === true,
        error: stringOrEmpty(candidate.error),
      };
    })
    .filter((item) => item.id && item.label);
}

function shouldAdoptSettingsSnapshot(current: Settings | undefined, next: Settings) {
  if (!current) {
    return true;
  }
  if (next.version > current.version) {
    return true;
  }
  if (next.version < current.version) {
    return false;
  }
  return JSON.stringify(current) !== JSON.stringify(next);
}

function toProxySettings(raw: BindingsProxy): ProxySettings {
  return {
    ...raw,
    mode: normalizeProxyMode(raw.mode),
    scheme: normalizeProxyScheme(raw.scheme),
    noProxy: [...raw.noProxy],
  };
}

function toSystemProxyInfo(raw: BindingsSystemProxyInfo): SystemProxyInfo {
  return {
    address: raw.address,
    source: normalizeSystemProxySource(raw.source),
    name: raw.name,
  };
}

function normalizeAppearanceMode(value?: string): Settings["appearance"] {
  switch (value) {
    case "light":
    case "dark":
    case "auto":
      return value;
    default:
      return "auto";
  }
}

function stringOrEmpty(value?: string) {
  return typeof value === "string" ? value.trim() : "";
}

function normalizeMenuBarVisibility(value?: string): Settings["menuBarVisibility"] {
  switch (value) {
    case "always":
    case "whenRunning":
    case "never":
      return value;
    default:
      return "whenRunning";
  }
}

function normalizeResourceSniffScope(value?: string): Settings["resourceSniffScope"] {
  switch (value) {
    case "advanced":
    case "all":
      return value;
    default:
      return "default";
  }
}

function normalizePlaybackAudioQuality(value?: string): Settings["playbackAudioQuality"] {
  switch (value) {
    case "AUDIO_QUALITY_AUTO":
      return "AUDIO_QUALITY_AUTO";
    case "AUDIO_QUALITY_LOW":
      return "AUDIO_QUALITY_LOW";
    case "AUDIO_QUALITY_MEDIUM":
      return "AUDIO_QUALITY_MEDIUM";
    case "AUDIO_QUALITY_HIGH":
      return "AUDIO_QUALITY_HIGH";
    default:
      return "AUDIO_QUALITY_AUTO";
  }
}

function normalizeResourceSniffMinBytes(value?: number) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return 8 * 1024;
  }
  return Math.min(Math.round(value), 10 * 1024 * 1024);
}

function normalizeResourceSniffRetain(value?: number) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return 1000;
  }
  return Math.min(Math.max(Math.round(value), 100), 10000);
}

function normalizeYTDLPConcurrentFragments(value?: number) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return 1;
  }
  return Math.min(Math.max(Math.round(value), 1), 16);
}

function normalizeYTDLPConcurrentDownloads(value?: number) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return 3;
  }
  return Math.min(Math.max(Math.round(value), 1), 5);
}

function normalizeProxyMode(value: string): ProxySettings["mode"] {
  switch (value) {
    case "none":
    case "system":
    case "manual":
      return value;
    default:
      return "none";
  }
}

function normalizeProxyScheme(value: string): ProxySettings["scheme"] {
  switch (value) {
    case "http":
    case "https":
    case "socks5":
      return value;
    default:
      return "http";
  }
}

function normalizeSystemProxySource(value?: string): SystemProxyInfo["source"] {
  switch (value) {
    case "system":
    case "vpn":
      return value;
    default:
      return undefined;
  }
}
