import {
Download,
FolderOpen,
Loader2,
RefreshCcw
} from "lucide-react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { ProxySettings } from "@/shared/contracts/settings";
import {
useDependencyInstallState,
useInstallDependency,
useOpenDependencyDirectory,
useRemoveDependency,
useVerifyDependency
} from "@/shared/query/dependencies";
import type { Dependency,DependencyUpdateInfo } from "@/shared/contracts/dependencies";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import {
resolveSettingsTab,
type XiaSettingsTabId
} from "./sectionStorage";

export const ACCENT_SWATCHES = [
  { id: "blue", value: "#2563eb" },
  { id: "indigo", value: "#4f46e5" },
  { id: "violet", value: "#7c3aed" },
  { id: "rose", value: "#db2777" },
  { id: "red", value: "#e11d48" },
  { id: "orange", value: "#ea580c" },
  { id: "amber", value: "#d97706" },
  { id: "teal", value: "#0f766e" },
] as const;
export const SYSTEM_THEME_COLOR = "system";
export const CORE_DEPENDENCY_ORDER = ["yt-dlp", "ffmpeg", "bun"];

export function isHexColor(value?: string) {
  return /^#[0-9a-f]{6}$/i.test((value ?? "").trim());
}

export function normalizeProxy(settingsProxy?: ProxySettings | null): ProxySettings {
  return {
    mode: settingsProxy?.mode ?? "system",
    scheme: settingsProxy?.scheme ?? "http",
    host: settingsProxy?.host ?? "",
    port: settingsProxy?.port ?? 0,
    username: settingsProxy?.username ?? "",
    password: settingsProxy?.password ?? "",
    noProxy: [...(settingsProxy?.noProxy ?? [])],
    timeoutSeconds: settingsProxy?.timeoutSeconds ?? 30,
    testedAt: settingsProxy?.testedAt ?? "",
    testSuccess: settingsProxy?.testSuccess ?? false,
    testMessage: settingsProxy?.testMessage ?? "",
  };
}

export function resolveAccentColor(value?: string, fallback = "#2563eb") {
  const trimmed = (value ?? "").trim();
  return isHexColor(trimmed) ? trimmed : fallback;
}

export function resolveThemeColorSelection(value?: string) {
  const trimmed = (value ?? "").trim();
  if (!trimmed || trimmed.toLowerCase() === SYSTEM_THEME_COLOR) {
    return SYSTEM_THEME_COLOR;
  }
  return resolveAccentColor(trimmed);
}

export function resolveThemeColorPreview(value: string | undefined, fallbackColor: string, systemThemeColor?: string) {
  const trimmed = (value ?? "").trim();
  return !trimmed || trimmed.toLowerCase() === SYSTEM_THEME_COLOR
    ? resolveAccentColor(systemThemeColor, fallbackColor)
    : resolveAccentColor(trimmed, fallbackColor);
}

export function resolveTabFromSection(section: string | null | undefined): XiaSettingsTabId {
  const normalized = (section ?? "").trim();
  return normalized === "tools" || normalized === "external-tools" ? "dependencies" : resolveSettingsTab(normalized);
}

export function parseNoProxy(text: string) {
  return text
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

export function resetProxyTestState(next: ProxySettings): ProxySettings {
  return {
    ...next,
    testSuccess: false,
    testMessage: "",
    testedAt: "",
  };
}

export function quoteFontFamily(value: string) {
  const escaped = value.replace(/\\/g, "\\\\").replace(/\"/g, '\\"');
  return `"${escaped}"`;
}

export function previewFontStack(family: string) {
  const trimmed = family.trim();
  if (!trimmed) {
    return undefined;
  }
  return `${quoteFontFamily(trimmed)}, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`;
}

export function formatHostPort(host: string, port: number) {
  if (!host || port <= 0) {
    return "";
  }
  const normalizedHost = host.includes(":") && !host.startsWith("[") ? `[${host}]` : host;
  return `${normalizedHost}:${port}`;
}

export function formatDependencyStatus(text: ReturnType<typeof getXiaText>, dependency?: Dependency | null) {
  const status = (dependency?.status ?? "missing").trim().toLowerCase();
  switch (status) {
    case "installed":
      return text.dependencies.installed;
    case "invalid":
      return text.dependencies.invalid;
    default:
      return text.dependencies.missing;
  }
}

export function formatDependencyDisplayName(value: string) {
  return value
    .trim()
    .toUpperCase()
    .replace(/[!-~]/g, (char) => String.fromCharCode(char.charCodeAt(0) + 0xfee0));
}

export function formatDependencyInstallStage(text: ReturnType<typeof getXiaText>, stage?: string) {
  switch ((stage ?? "").trim().toLowerCase()) {
    case "downloading":
      return text.dependencies.downloading;
    case "extracting":
      return text.dependencies.extracting;
    case "verifying":
      return text.dependencies.verifying;
    default:
      return text.dependencies.installing;
  }
}

export function isDependencyInstallActive(stage?: string) {
  switch ((stage ?? "").trim().toLowerCase()) {
    case "downloading":
    case "extracting":
    case "verifying":
      return true;
    default:
      return false;
  }
}

export function clampProgress(value: number | undefined) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return 0;
  }
  return Math.min(Math.max(value, 0), 100);
}

export function resolveDependencyTone(status?: string) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "installed":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-200";
    case "invalid":
      return "border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-200";
    default:
      return "border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-200";
  }
}

export function InlineSwitch(props: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  ariaLabel: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={props.checked}
      aria-label={props.ariaLabel}
      onClick={() => props.onChange(!props.checked)}
      className={cn(
        "flex h-5 w-9 items-center rounded-full border px-0.5 transition",
        props.checked ? "justify-end border-primary/40 bg-primary/15" : "justify-start border-border/70 bg-muted",
      )}
    >
      <span className="h-4 w-4 rounded-full bg-background shadow-sm" />
    </button>
  );
}

export function TabButton(props: {
  id: XiaSettingsTabId;
  label: string;
  icon: React.ReactNode;
  active: boolean;
  onClick: (id: XiaSettingsTabId) => void;
}) {
  return (
    <button
      type="button"
      title={props.label}
      onClick={() => props.onClick(props.id)}
      className={cn(
        "flex w-[74px] min-w-[74px] max-w-[74px] flex-col items-center justify-center gap-0 rounded-2xl border px-1.5 py-1 text-center transition",
        props.active
          ? "border-border bg-muted text-foreground shadow-sm"
          : "border-transparent text-muted-foreground hover:border-border/70 hover:bg-muted/70 hover:text-foreground",
      )}
    >
      <span className="flex h-9 w-9 items-center justify-center">{props.icon}</span>
      <span className="w-full truncate text-[10px] font-medium leading-3">{props.label}</span>
    </button>
  );
}

export function DependencySettingsItem(props: {
  dependency: Dependency;
  update?: DependencyUpdateInfo;
  text: ReturnType<typeof getXiaText>;
}) {
  const { dependency, update, text } = props;
  const installStateQuery = useDependencyInstallState(dependency.name);
  const installDependency = useInstallDependency();
  const removeDependency = useRemoveDependency();
  const verifyDependency = useVerifyDependency();
  const openDependencyDirectory = useOpenDependencyDirectory();
  const status = (dependency.status ?? "missing").trim().toLowerCase();
  const isInstalled = status === "installed";
  const installLabel = isInstalled ? text.dependencies.reinstall : text.actions.install;
  const showMaintenanceActions = isInstalled;
  const currentVersion = dependency.version || "-";
  const latestVersion = update?.latestVersion || "-";
  const installStage = (installStateQuery.data?.stage ?? "idle").trim().toLowerCase();
  const isInstalling = isDependencyInstallActive(installStage);
  const installProgress = clampProgress(installStateQuery.data?.progress);
  const isPrimaryPending = installDependency.isPending || removeDependency.isPending || isInstalling;
  const canOpenDirectory = isInstalled && !isInstalling;

  async function handleInstallOrReinstall() {
    if (isInstalled) {
      await removeDependency.mutateAsync({ name: dependency.name });
    }
    await installDependency.mutateAsync({ name: dependency.name });
  }

  return (
    <div className="space-y-2 px-3 py-2.5">
      <div className="flex min-w-0 items-center justify-between gap-3">
        <div className="min-w-0 truncate text-sm font-bold tracking-[0.08em] text-foreground">
          {formatDependencyDisplayName(dependency.name)}
        </div>
        <div className="inline-flex max-w-full shrink-0 overflow-hidden rounded-md border border-input bg-background shadow-sm">
          <Button
            type="button"
            variant="ghost"
            size="compact"
            className="rounded-none border-0 shadow-none"
            onClick={() => void handleInstallOrReinstall()}
            disabled={isPrimaryPending}
          >
            {isPrimaryPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
            <span className="truncate">{installLabel}</span>
          </Button>
          {showMaintenanceActions ? (
            <>
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="rounded-none border-0 border-l border-input shadow-none"
                onClick={() => void verifyDependency.mutateAsync({ name: dependency.name })}
                disabled={verifyDependency.isPending || isInstalling}
              >
                {verifyDependency.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                <span className="truncate">{text.actions.verify}</span>
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="rounded-none border-0 border-l border-input shadow-none"
                onClick={() => void openDependencyDirectory.mutateAsync({ name: dependency.name })}
                disabled={!canOpenDirectory || openDependencyDirectory.isPending}
              >
                <FolderOpen className="h-4 w-4" />
                <span className="truncate">{text.actions.openDirectory}</span>
              </Button>
            </>
          ) : null}
        </div>
      </div>

      <div className="flex justify-start">
        {isInstalling ? (
          <div className="min-w-0 max-w-[17rem] flex-1 space-y-1.5">
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div className="h-full bg-primary transition-[width]" style={{ width: `${installProgress}%` }} />
            </div>
            <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
              <span className="truncate">{formatDependencyInstallStage(text, installStage)}</span>
              <span className="shrink-0 tabular-nums">{Math.round(installProgress)}%</span>
            </div>
          </div>
        ) : (
          <Badge className={cn("min-w-0 max-w-full gap-0 overflow-hidden p-0", resolveDependencyTone(status))}>
            <span className="min-w-0 truncate px-2 py-0.5">{formatDependencyStatus(text, dependency)}</span>
            <span className="min-w-0 truncate border-l border-current/20 px-2 py-0.5">
              {text.dependencies.currentVersion}: {currentVersion}
            </span>
            <span className="min-w-0 truncate border-l border-current/20 px-2 py-0.5">
              {text.dependencies.latestVersion}: {latestVersion}
            </span>
          </Badge>
        )}
      </div>
    </div>
  );
}
