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
import type { ProxySettings } from "@/shared/contracts/settings";
import {
useDependencyInstallState,
useInstallDependency,
useOpenDependencyDirectory,
useRemoveDependency,
useVerifyDependency
} from "@/shared/query/dependencies";
import type { Dependency,DependencyUpdateInfo } from "@/shared/contracts/dependencies";
import { Button } from "@/shared/ui/button";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import { Progress } from "@/shared/ui/progress";
import {
  StatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
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

export function resolveAccentColor(value?: string, fallback?: string) {
  const trimmed = (value ?? "").trim();
  if (isHexColor(trimmed)) {
    return trimmed;
  }
  const safeFallback = (fallback ?? "").trim();
  return isHexColor(safeFallback) ? safeFallback : "";
}

export function resolveThemeColorSelection(value?: string) {
  const trimmed = (value ?? "").trim();
  if (!trimmed || trimmed.toLowerCase() === SYSTEM_THEME_COLOR) {
    return SYSTEM_THEME_COLOR;
  }
  return resolveAccentColor(trimmed) || SYSTEM_THEME_COLOR;
}

export function resolveThemeColorPreview(
  value: string | undefined,
  systemThemeColor?: string,
) {
  const trimmed = (value ?? "").trim();
  return !trimmed || trimmed.toLowerCase() === SYSTEM_THEME_COLOR
    ? resolveAccentColor(systemThemeColor)
    : resolveAccentColor(trimmed);
}

export function resolveTabFromSection(section: string | null | undefined): XiaSettingsTabId {
  const normalized = (section ?? "").trim();
  return normalized === "tools" || normalized === "external-tools" ? "download" : resolveSettingsTab(normalized);
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
  return `${quoteFontFamily(trimmed)}, var(--app-font-system)`;
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

export function resolveDependencyTone(status?: string): DreamStatusTone {
  switch ((status ?? "").trim().toLowerCase()) {
    case "installed":
      return "success";
    case "invalid":
      return "danger";
    default:
      return "warning";
  }
}

export function InlineSwitch(props: {
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
  ariaLabel: string;
}) {
  return (
    <DreamInlineSwitch
      ariaLabel={props.ariaLabel}
      checked={props.checked}
      disabled={props.disabled}
      onCheckedChange={props.onChange}
    />
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
    <Button
      type="button"
      variant="ghost"
      tone={props.active ? "accent" : "neutral"}
      title={props.label}
      onClick={() => props.onClick(props.id)}
      className="app-settings-tab-button"
      data-active={props.active ? "true" : undefined}
    >
      <span className="app-settings-tab-icon flex h-9 w-9 items-center justify-center">{props.icon}</span>
      <span className="app-settings-tab-label w-full truncate">{props.label}</span>
    </Button>
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
        <div className="app-settings-dependency-title min-w-0 truncate">
          {formatDependencyDisplayName(dependency.name)}
        </div>
        <TooltipProvider delayDuration={0}>
          <div className="app-dream-button-group app-dream-button-group-icon">
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="inline-flex">
                  <Button
                    type="button"
                    variant="ghost"
                    size="compactIcon"
                    onClick={() => void handleInstallOrReinstall()}
                    disabled={isPrimaryPending}
                    aria-label={installLabel}
                  >
                    {isPrimaryPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Download className="h-4 w-4" />}
                  </Button>
                </span>
              </TooltipTrigger>
              <TooltipContent side="top">{installLabel}</TooltipContent>
            </Tooltip>
            {showMaintenanceActions ? (
              <>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex">
                      <Button
                        type="button"
                        variant="ghost"
                        size="compactIcon"
                        onClick={() => void verifyDependency.mutateAsync({ name: dependency.name })}
                        disabled={verifyDependency.isPending || isInstalling}
                        aria-label={text.actions.verify}
                      >
                        {verifyDependency.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <RefreshCcw className="h-4 w-4" />}
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="top">{text.actions.verify}</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <span className="inline-flex">
                      <Button
                        type="button"
                        variant="ghost"
                        size="compactIcon"
                        onClick={() => void openDependencyDirectory.mutateAsync({ name: dependency.name })}
                        disabled={!canOpenDirectory || openDependencyDirectory.isPending}
                        aria-label={text.actions.openDirectory}
                      >
                        {openDependencyDirectory.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <FolderOpen className="h-4 w-4" />}
                      </Button>
                    </span>
                  </TooltipTrigger>
                  <TooltipContent side="top">{text.actions.openDirectory}</TooltipContent>
                </Tooltip>
              </>
            ) : null}
          </div>
        </TooltipProvider>
      </div>

      <div className="flex justify-start">
        {isInstalling ? (
          <div className="app-settings-dependency-progress min-w-0 flex-1 space-y-1.5">
            <Progress aria-label={formatDependencyInstallStage(text, installStage)} value={installProgress} />
            <div className="app-settings-progress-meta flex items-center justify-between gap-3">
              <span className="truncate">{formatDependencyInstallStage(text, installStage)}</span>
              <span className="app-settings-progress-value shrink-0">{Math.round(installProgress)}%</span>
            </div>
          </div>
        ) : (
          <StatusBadge className="app-settings-dependency-status" tone={resolveDependencyTone(status)}>
            <span className="min-w-0 truncate px-2 py-0.5">{formatDependencyStatus(text, dependency)}</span>
            <span className="min-w-0 truncate px-2 py-0.5">
              {text.dependencies.currentVersion}: {currentVersion}
            </span>
            <span className="min-w-0 truncate px-2 py-0.5">
              {text.dependencies.latestVersion}: {latestVersion}
            </span>
          </StatusBadge>
        )}
      </div>
    </div>
  );
}
