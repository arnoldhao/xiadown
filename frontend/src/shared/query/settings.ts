import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { Call, Events } from "@wailsio/runtime";

import type { ProxySettings, Settings, SystemProxyInfo, UpdateSettingsRequest } from "@/shared/contracts/settings";
import { normalizeColorScheme } from "@/lib/theme/color-schemes";
import {
  OpenLogDirectory,
  RefreshSystemProxy,
  SelectDownloadDirectory,
  HideSettingsWindow,
  ShowMainWindow,
  ShowSettingsWindow,
  TestProxy,
} from "../../../bindings/xiadown/internal/presentation/wails/settingshandler";
import {
  Proxy as BindingsProxy,
  SystemProxyInfo as BindingsSystemProxyInfo,
} from "../../../bindings/xiadown/internal/application/settings/dto/models";

export const SETTINGS_QUERY_KEY = ["settings"];

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

function toSettings(raw: Partial<Settings>): Settings {
  return {
    ...(raw as Settings),
    appearance: normalizeAppearanceMode(raw.appearance ?? "auto"),
    colorScheme: normalizeColorScheme(raw.colorScheme),
    menuBarVisibility: normalizeMenuBarVisibility(raw.menuBarVisibility ?? "whenRunning"),
    mainBounds: { ...(raw.mainBounds ?? { x: 0, y: 0, width: 0, height: 0 }) },
    settingsBounds: { ...(raw.settingsBounds ?? { x: 0, y: 0, width: 0, height: 0 }) },
    proxy: toProxySettings(BindingsProxy.createFrom(raw.proxy ?? {})),
    appearanceConfig: raw.appearanceConfig,
  };
}

function toSettingsUpdatePayload(request: UpdateSettingsRequest) {
  return { ...request };
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
