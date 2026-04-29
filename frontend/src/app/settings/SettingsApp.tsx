import { Browser,Events,System } from "@wailsio/runtime";
import {
AlertCircle,
ArrowUpCircle,
CheckCircle2,
Cog,
Download,
ExternalLink,
FolderOpen,
Github,
Globe,
Info,
Loader2,
Mail,
MessageSquare,
Monitor,
Moon,
Palette,
Pencil,
RefreshCcw,
RefreshCw,
Sun,
Twitter,
WandSparkles,
Wrench,
} from "lucide-react";
import * as React from "react";

import { ACCENT_SWATCHES,CORE_DEPENDENCY_ORDER,DependencySettingsItem,InlineSwitch,SYSTEM_THEME_COLOR,TabButton,formatHostPort,normalizeProxy,parseNoProxy,previewFontStack,resetProxyTestState,resolveAccentColor,resolveTabFromSection,resolveThemeColorPreview,resolveThemeColorSelection } from "@/app/settings/settings-helpers";
import { WindowControls } from "@/components/layout/WindowControls";
import { SpritesSection } from "@/features/settings/sprites";
import { getXiaText } from "@/features/xiadown/shared";
import {
XIA_THEME_PACKS,
mergeXiaAppearanceConfig,
readXiaAppearance,
resolveThemePack,
type XiaAccentMode,
type XiaAppearanceSettings,
} from "@/shared/styles/xiadown-theme";
import { cn } from "@/lib/utils";
import type { ProxySettings } from "@/shared/contracts/settings";
import { DialogMarkdown } from "@/shared/markdown/dialog-markdown";
import {
useDependencies,
useDependencyUpdates
} from "@/shared/query/dependencies";
import { useOpenLibraryPath } from "@/shared/query/library";
import {
useOpenLogDirectory,
useSelectDownloadDirectory,
useSettings,
useSystemProxyInfo,
useTestProxy,
useUpdateSettings,
} from "@/shared/query/settings";
import { useFontFamilies } from "@/shared/query/system";
import {
useCheckForUpdate,
useDownloadUpdate,
useRestartToApply,
useUpdateState,
} from "@/shared/query/update";
import { useSettingsStore } from "@/shared/store/settings";
import {
displayUpdateVersion,
hasPreparedUpdate,
hasRemoteUpdate,
useUpdateStore,
} from "@/shared/store/update";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import {
Dialog,
DialogClose,
DialogContent,
DialogDescription,
DialogFooter,
DialogHeader,
DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import {
SettingsCompactListCard,
SettingsCompactRow,
SettingsCompactSeparator,
} from "@/shared/ui/settings-layout";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
import {
consumePendingSettingsTab,
listenPendingSettingsTab,
type XiaSettingsTabId
} from "./sectionStorage";

const ABOUT_AUTHOR_NAME = "Arnold HAO";

export function SettingsApp() {
  const settings = useSettingsStore((state) => state.settings);
  const { data: liveSettings } = useSettings();
  const { data: fontFamilies = [], isLoading: isFontFamiliesLoading } = useFontFamilies();
  const updateSettings = useUpdateSettings();
  const selectDownloadDirectory = useSelectDownloadDirectory();
  const openLibraryPath = useOpenLibraryPath();
  const openLogDirectory = useOpenLogDirectory();
  const testProxy = useTestProxy();
  const systemProxyQuery = useSystemProxyInfo(true);
  const dependenciesQuery = useDependencies({ refetchInterval: 5_000 });
  const dependencyUpdatesQuery = useDependencyUpdates();
  const updateInfo = useUpdateStore((state) => state.info);
  const setUpdateInfo = useUpdateStore((state) => state.setInfo);
  const { data: serverUpdateInfo } = useUpdateState();
  const checkForUpdate = useCheckForUpdate();
  const downloadUpdate = useDownloadUpdate();
  const restartToApply = useRestartToApply();

  const currentSettings = liveSettings ?? settings;
  const text = getXiaText(currentSettings?.language);
  const isWindows = System.IsWindows();
  const isMac = System.IsMac();
  const [activeTab, setActiveTab] = React.useState<XiaSettingsTabId>("general");
  const [proxyDraft, setProxyDraft] = React.useState<ProxySettings>(() => normalizeProxy(currentSettings?.proxy));
  const [proxyNoProxyText, setProxyNoProxyText] = React.useState("");
  const [appearanceDraft, setAppearanceDraft] = React.useState<XiaAppearanceSettings>(() => readXiaAppearance(currentSettings));
  const [fontFamilyDraft, setFontFamilyDraft] = React.useState((currentSettings?.fontFamily ?? "").trim());
  const [fontSizeDraft, setFontSizeDraft] = React.useState(currentSettings?.fontSize ?? 15);
  const [themeColorDraft, setThemeColorDraft] = React.useState(resolveThemeColorSelection(currentSettings?.themeColor));
  const [proxyDialogOpen, setProxyDialogOpen] = React.useState(false);
  const [releaseNotesOpen, setReleaseNotesOpen] = React.useState(false);
  const [proxyCheckStatus, setProxyCheckStatus] = React.useState<"idle" | "checking" | "available" | "unavailable">("idle");
  const [proxyCheckKey, setProxyCheckKey] = React.useState("");
  const proxyCheckRequestRef = React.useRef(0);
  const proxyCheckInFlightKeyRef = React.useRef("");
  const autoRefreshUpdateRef = React.useRef(false);

  React.useEffect(() => {
    const nextProxy = normalizeProxy(currentSettings?.proxy);
    setProxyDraft(nextProxy);
    setProxyNoProxyText(nextProxy.noProxy.join(", "));
  }, [currentSettings?.proxy]);

  React.useEffect(() => {
    setAppearanceDraft(readXiaAppearance(currentSettings));
    setFontFamilyDraft((currentSettings?.fontFamily ?? "").trim());
    setFontSizeDraft(currentSettings?.fontSize ?? 15);
    setThemeColorDraft(resolveThemeColorSelection(currentSettings?.themeColor));
  }, [currentSettings]);

  React.useEffect(() => {
    const pending = consumePendingSettingsTab();
    if (pending) {
      setActiveTab(pending);
    }
    const unsubscribe = listenPendingSettingsTab(setActiveTab);
    const offNavigate = Events.On("settings:navigate", (event: any) => {
      const target = typeof (event?.data ?? event) === "string" ? (event?.data ?? event) : "";
      setActiveTab(resolveTabFromSection(target));
    });
    return () => {
      unsubscribe();
      offNavigate();
    };
  }, []);

  React.useEffect(() => {
    if (serverUpdateInfo) {
      setUpdateInfo(serverUpdateInfo);
    }
  }, [serverUpdateInfo, setUpdateInfo]);

  React.useEffect(() => {
    if (autoRefreshUpdateRef.current) {
      return;
    }
    const status = updateInfo.status;
    if (status === "checking" || status === "downloading" || status === "installing") {
      return;
    }
    const currentVersion = updateInfo.currentVersion.trim();
    if (!currentVersion) {
      return;
    }
    const checkedAt = (updateInfo.checkedAt ?? "").trim();
    let stale = true;
    if (checkedAt) {
      const checkedAtMs = Date.parse(checkedAt);
      if (Number.isFinite(checkedAtMs)) {
        stale = Date.now() - checkedAtMs >= 60 * 60 * 1000;
      }
    }
    if (!stale) {
      return;
    }
    autoRefreshUpdateRef.current = true;
    void checkForUpdate
      .mutateAsync(currentVersion)
      .then((next) => {
        setUpdateInfo(next);
      })
      .catch((error) => {
        console.warn(error);
      });
  }, [checkForUpdate, setUpdateInfo, updateInfo]);

  const sortedDependencies = React.useMemo(() => {
    const items = [...(dependenciesQuery.data ?? [])];
    items.sort((left, right) => {
      const leftRank = CORE_DEPENDENCY_ORDER.indexOf(left.name);
      const rightRank = CORE_DEPENDENCY_ORDER.indexOf(right.name);
      const normalizedLeft = leftRank === -1 ? 999 : leftRank;
      const normalizedRight = rightRank === -1 ? 999 : rightRank;
      if (normalizedLeft !== normalizedRight) {
        return normalizedLeft - normalizedRight;
      }
      return left.name.localeCompare(right.name);
    });
    return items;
  }, [dependenciesQuery.data]);

  const dependencyUpdatesByName = React.useMemo(
    () => new Map((dependencyUpdatesQuery.data ?? []).map((item) => [item.name, item])),
    [dependencyUpdatesQuery.data],
  );

  const isCheckingUpdate = updateInfo.status === "checking" || checkForUpdate.isPending;
  const isUpdateError = updateInfo.status === "error";
  const hasPreparedAppUpdate = hasPreparedUpdate(updateInfo);
  const hasRemoteAppUpdate = hasRemoteUpdate(updateInfo);
  const hasKnownPendingAppUpdate = hasPreparedAppUpdate || hasRemoteAppUpdate;
  const isDownloadingUpdate = updateInfo.status === "downloading" || updateInfo.status === "installing";
  const isReadyToRestartUpdate = updateInfo.status === "ready_to_restart" && hasPreparedAppUpdate;
  const releaseNotes = ((isReadyToRestartUpdate ? updateInfo.preparedChangelog : updateInfo.changelog) ?? "").trim();
  const hasReleaseNotes = releaseNotes.length > 0;
  const updateErrorMessage = (updateInfo.message ?? "").trim();
  const showLatestAppUpdate = hasKnownPendingAppUpdate || isDownloadingUpdate || isReadyToRestartUpdate;
  const showUpdateStatusRow = isDownloadingUpdate || (isUpdateError && updateErrorMessage.length > 0);
  const showCheckUpdateAction = !isReadyToRestartUpdate && !isDownloadingUpdate;
  const showInstallUpdateAction =
    !isReadyToRestartUpdate &&
    !isDownloadingUpdate &&
    (updateInfo.status === "available" || (isUpdateError && hasRemoteAppUpdate && !hasPreparedAppUpdate));
  const checkUpdateLabel = hasKnownPendingAppUpdate ? text.about.recheck : text.about.checkUpdates;
  const latestUpdateLabel = (() => {
    if (showLatestAppUpdate) {
      return displayUpdateVersion(updateInfo) || text.about.latestAvailable;
    }
    if (isUpdateError) {
      return text.about.latestFailed;
    }
    return text.about.latestOk;
  })();
  const latestUpdateBadgeClass = (() => {
    if (showLatestAppUpdate) {
      return "border-primary/20 bg-primary/10 text-primary";
    }
    if (isUpdateError) {
      return "border-destructive/20 bg-destructive/10 text-destructive";
    }
    return "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-700/60 dark:bg-emerald-900/40 dark:text-emerald-100";
  })();
  const latestUpdateBadgeIcon = (() => {
    if (showLatestAppUpdate) {
      return ArrowUpCircle;
    }
    if (isUpdateError) {
      return AlertCircle;
    }
    return CheckCircle2;
  })();
  const aboutVersion = updateInfo.currentVersion.trim() || "dev";

  async function saveSettingsPatch(patch: Parameters<typeof updateSettings.mutateAsync>[0]) {
    await updateSettings.mutateAsync(patch);
  }

  async function saveAppearancePatch(patch: Partial<XiaAppearanceSettings>) {
    const nextAppearance = { ...appearanceDraft, ...patch };
    setAppearanceDraft(nextAppearance);
    await saveSettingsPatch({
      appearanceConfig: mergeXiaAppearanceConfig(currentSettings, patch),
    });
  }

  async function saveAccentMode(nextMode: XiaAccentMode) {
    const nextAppearance = { ...appearanceDraft, accentMode: nextMode };
    setAppearanceDraft(nextAppearance);

    const appearanceConfig = mergeXiaAppearanceConfig(currentSettings, { accentMode: nextMode });
    if (nextMode === "theme") {
      await saveSettingsPatch({ appearanceConfig });
      return;
    }

    const nextColor =
      themeColorDraft.trim().toLowerCase() === SYSTEM_THEME_COLOR
        ? SYSTEM_THEME_COLOR
        : resolveAccentColor(themeColorDraft, resolveThemePack(nextAppearance.themePackId).preview.accent);
    setThemeColorDraft(nextColor);
    await saveSettingsPatch({
      themeColor: nextColor,
      appearanceConfig,
    });
  }

  async function chooseDownloadDir() {
    const path = await selectDownloadDirectory.mutateAsync(text.actions.chooseFolder);
    if (!path) {
      return;
    }
    await saveSettingsPatch({ downloadDirectory: path });
  }

  async function handleCheckUpdate() {
    const currentVersion = updateInfo.currentVersion.trim();
    if (!currentVersion) {
      return;
    }
    try {
      const next = await checkForUpdate.mutateAsync(currentVersion);
      setUpdateInfo(next);
    } catch (error) {
      console.warn(error);
    }
  }

  async function handleInstallUpdate() {
    try {
      const next = await downloadUpdate.mutateAsync();
      setUpdateInfo(next);
    } catch (error) {
      console.warn(error);
    }
  }

  async function handleRestartUpdate() {
    try {
      const next = await restartToApply.mutateAsync();
      setUpdateInfo(next);
    } catch (error) {
      console.warn(error);
    }
  }

  function openExternalURL(url: string) {
    void Browser.OpenURL(url);
  }

  async function saveProxySettings(next: ProxySettings) {
    try {
      const updated = await updateSettings.mutateAsync({ proxy: next });
      const normalized = normalizeProxy(updated.proxy);
      setProxyDraft(normalized);
      setProxyNoProxyText(normalized.noProxy.join(", "));
      return normalized;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setProxyDraft({
        ...next,
        testSuccess: false,
        testMessage: message,
        testedAt: "",
      });
      setProxyNoProxyText(next.noProxy.join(", "));
      throw error;
    }
  }

  function handleProxyFieldChange(field: keyof ProxySettings, value: string) {
    const isNumericField = field === "port" || field === "timeoutSeconds";
    setProxyDraft((current) =>
      resetProxyTestState({
        ...current,
        [field]: isNumericField ? Number(value) || 0 : value,
      } as ProxySettings),
    );
  }

  function handleProxyModeChange(mode: ProxySettings["mode"]) {
    const savedProxy = normalizeProxy(currentSettings?.proxy);
    const next = resetProxyTestState({
      ...savedProxy,
      mode,
      scheme: savedProxy.scheme || "http",
    });
    setProxyDraft(next);
    setProxyNoProxyText(next.noProxy.join(", "));
    if (mode === "manual") {
      setProxyDialogOpen(true);
      return;
    }
    setProxyDialogOpen(false);
    void saveProxySettings(next).catch(() => undefined);
  }

  async function handleProxyClear() {
    const savedProxy = normalizeProxy(currentSettings?.proxy);
    const cleared = resetProxyTestState({
      ...savedProxy,
      mode: "none",
      scheme: savedProxy.scheme || "http",
      host: "",
      port: 0,
      username: "",
      password: "",
      noProxy: [],
      timeoutSeconds: savedProxy.timeoutSeconds || 30,
    });
    setProxyDraft(cleared);
    setProxyNoProxyText("");
    setProxyDialogOpen(false);
    await saveProxySettings(cleared).catch(() => undefined);
  }

  async function handleProxyTestAndSave() {
    const payload = {
      ...proxyDraft,
      noProxy: parseNoProxy(proxyNoProxyText),
    };

    try {
      const result = await testProxy.mutateAsync(payload);
      setProxyDraft(result);
      setProxyNoProxyText(result.noProxy.join(", "));
      if (!result.testSuccess) {
        return;
      }
      await saveProxySettings(result);
      setProxyDialogOpen(false);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      setProxyDraft((current) =>
        resetProxyTestState({
          ...current,
          testSuccess: false,
          testMessage: message,
        }),
      );
    }
  }

  const tabs: Array<{ id: XiaSettingsTabId; label: string; icon: React.ReactNode }> = [
    { id: "general", label: text.settings.tabs.general, icon: <Cog className="h-7 w-7" /> },
    { id: "appearance", label: text.settings.tabs.appearance, icon: <Palette className="h-7 w-7" /> },
    { id: "sprites", label: text.settings.tabs.sprites, icon: <WandSparkles className="h-7 w-7" /> },
    { id: "dependencies", label: text.settings.tabs.dependencies, icon: <Wrench className="h-7 w-7" /> },
    { id: "about", label: text.settings.tabs.about, icon: <Info className="h-7 w-7" /> },
  ];
  const activeTabMeta = tabs.find((tab) => tab.id === activeTab) ?? tabs[0];
  const fontOptions = fontFamilies;
  const selectedFont = fontFamilyDraft.trim();
  const hasSelectedFontInList = selectedFont.length === 0 || fontOptions.includes(selectedFont);
  const activeThemePack = resolveThemePack(appearanceDraft.themePackId);
  const usesSystemAccentColor = themeColorDraft.trim().toLowerCase() === SYSTEM_THEME_COLOR;
  const selectedThemeColorPreview = resolveThemeColorPreview(
    themeColorDraft,
    activeThemePack.preview.accent,
    currentSettings?.systemThemeColor,
  );
  const editableAccentColor = usesSystemAccentColor
    ? selectedThemeColorPreview
    : resolveAccentColor(themeColorDraft, activeThemePack.preview.accent);
  const savedProxy = normalizeProxy(currentSettings?.proxy);
  const systemProxyAddress = (systemProxyQuery.data?.address ?? "").trim();
  const isVPNSource = systemProxyQuery.data?.source === "vpn";
  const systemSourceLabel = isVPNSource
    ? (systemProxyQuery.data?.name ?? "").trim() || text.settings.vpnSource
    : text.settings.systemSource;
  const systemProxyDisplay = systemProxyQuery.isLoading
    ? text.settings.checking
    : systemProxyQuery.isError
      ? text.settings.unavailable
      : systemProxyAddress || text.settings.notConfigured;
  const manualProxyHostPort = formatHostPort(proxyDraft.host.trim(), proxyDraft.port);
  const manualProxyAddress = manualProxyHostPort ? `${proxyDraft.scheme}://${manualProxyHostPort}` : "";
  const statusMode = proxyDraft.mode;
  const statusAddress = statusMode === "system" ? systemProxyAddress : statusMode === "manual" ? manualProxyAddress : "";
  const statusAddressDisplay =
    statusMode === "system" ? systemProxyDisplay : statusMode === "manual" ? manualProxyAddress || text.settings.notConfigured : text.settings.noProxy;
  const hasStatusAddress = statusAddress !== "";
  const statusKey = hasStatusAddress ? `${statusMode}:${statusAddress}` : "";
  const showSystemSourceBadge = statusMode === "system" && Boolean(systemSourceLabel);
  const statusDotClass =
    proxyCheckStatus === "available"
      ? "bg-emerald-500"
      : proxyCheckStatus === "unavailable"
        ? "bg-destructive"
        : "bg-muted-foreground/40";
  const isChecking = proxyCheckStatus === "checking" && proxyCheckKey === statusKey;
  const showRefreshButton = statusMode === "system" || hasStatusAddress;
  const isStatusRefreshing = statusMode === "system" ? systemProxyQuery.isFetching || isChecking : isChecking;
  const hasProxyTested = proxyDraft.testedAt && proxyDraft.testedAt !== "0001-01-01T00:00:00Z";
  const testedAt = hasProxyTested ? new Date(proxyDraft.testedAt) : null;
  const proxyTestFeedback = proxyDraft.testSuccess && testedAt
    ? `${text.actions.save} · ${testedAt.toLocaleString()}`
    : proxyDraft.testMessage
      ? proxyDraft.testMessage
      : "";
  const manualProxyReady = proxyDraft.mode === "manual" && proxyDraft.host.trim() !== "" && proxyDraft.port > 0;
  const appearanceSelectionColor = appearanceDraft.accentMode === "color" ? selectedThemeColorPreview : activeThemePack.preview.accent;
  const appearanceSelectionShadow = `0 0 0 1px hsl(var(--border)), 0 0 0 3px ${appearanceSelectionColor}`;
  const runProxyStatusCheck = React.useCallback(
    async (mode: ProxySettings["mode"], address: string) => {
      if (mode === "none" || !address) {
        return;
      }
      const nextKey = `${mode}:${address}`;
      if (proxyCheckInFlightKeyRef.current === nextKey) {
        return;
      }
      proxyCheckInFlightKeyRef.current = nextKey;
      proxyCheckRequestRef.current += 1;
      const requestId = proxyCheckRequestRef.current;
      setProxyCheckKey(nextKey);
      setProxyCheckStatus("checking");

      try {
        const result = await testProxy.mutateAsync(
          mode === "system"
            ? {
                ...resetProxyTestState(savedProxy),
                mode,
                host: "",
                port: 0,
                username: "",
                password: "",
              }
            : {
                ...resetProxyTestState({
                  ...proxyDraft,
                  noProxy: parseNoProxy(proxyNoProxyText),
                }),
                mode,
              },
        );

        if (proxyCheckRequestRef.current !== requestId) {
          return;
        }

        setProxyCheckStatus(result.testSuccess ? "available" : "unavailable");
      } catch {
        if (proxyCheckRequestRef.current !== requestId) {
          return;
        }
        setProxyCheckStatus("unavailable");
      } finally {
        if (proxyCheckInFlightKeyRef.current === nextKey) {
          proxyCheckInFlightKeyRef.current = "";
        }
      }
    },
    [proxyDraft, proxyNoProxyText, savedProxy, testProxy],
  );

  const handleProxyStatusRefresh = React.useCallback(async () => {
    if (statusMode === "system") {
      try {
        const result = await systemProxyQuery.refetch();
        const nextAddress = (result.data?.address ?? "").trim();
        if (nextAddress) {
          void runProxyStatusCheck("system", nextAddress);
        } else {
          setProxyCheckStatus("idle");
          setProxyCheckKey("");
        }
      } catch {
        setProxyCheckStatus("idle");
        setProxyCheckKey("");
      }
      return;
    }

    if (hasStatusAddress) {
      void runProxyStatusCheck(statusMode, statusAddress);
    }
  }, [hasStatusAddress, runProxyStatusCheck, statusAddress, statusMode, systemProxyQuery]);

  const proxySettingsCard = (
    <SettingsCompactListCard>
      <SettingsCompactRow label={text.settings.proxy}>
        <div className="flex flex-wrap items-center gap-2">
          {([
            { value: "none", label: text.settings.noProxy },
            { value: "system", label: text.settings.systemProxy },
            { value: "manual", label: text.settings.manualProxy },
          ] as const).map((option) => (
            <Button
              key={option.value}
              type="button"
              variant="outline"
              size="compact"
              onClick={() => handleProxyModeChange(option.value)}
              style={proxyDraft.mode === option.value ? { boxShadow: "0 0 0 1px hsl(var(--border)), 0 0 0 3px hsl(var(--primary))" } : undefined}
            >
              {option.label}
            </Button>
          ))}
        </div>
      </SettingsCompactRow>

      {proxyDraft.mode !== "none" ? (
        <>
          <SettingsCompactSeparator />

          <SettingsCompactRow label={text.settings.status} contentClassName="min-w-0">
            <div className="flex min-w-0 items-center justify-end gap-2">
              {showSystemSourceBadge ? (
                <span className="inline-flex h-7 items-center rounded-md bg-primary px-2 text-xs text-primary-foreground">
                  {systemSourceLabel}
                </span>
              ) : null}
              <span className="max-w-[260px] truncate text-right font-mono text-xs text-muted-foreground">
                {statusAddressDisplay}
              </span>
              {hasStatusAddress ? (
                <span className="inline-flex items-center">
                  <span className={cn("h-2 w-2 rounded-full", statusDotClass, isChecking ? "animate-pulse" : "")} aria-hidden="true" />
                </span>
              ) : null}
              {showRefreshButton ? (
                <Button
                  type="button"
                  variant="outline"
                  size="compactIcon"
                  disabled={isStatusRefreshing}
                  onClick={() => void handleProxyStatusRefresh()}
                  title={text.actions.testProxy}
                  aria-label={text.actions.testProxy}
                >
                  {isStatusRefreshing ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                </Button>
              ) : null}
              {proxyDraft.mode === "manual" ? (
                <Button
                  type="button"
                  variant="outline"
                  size="compactIcon"
                  onClick={() => setProxyDialogOpen(true)}
                  title={text.settings.editProxy}
                  aria-label={text.settings.editProxy}
                >
                  <Pencil className="h-4 w-4" />
                </Button>
              ) : null}
            </div>
          </SettingsCompactRow>
        </>
      ) : null}
    </SettingsCompactListCard>
  );
  const proxyDialog = proxyDraft.mode === "manual" ? (
    <Dialog open={proxyDialogOpen} onOpenChange={setProxyDialogOpen}>
      <DialogContent className="grid max-h-[min(34rem,calc(100vh-2rem))] w-[min(28rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)] gap-3 overflow-hidden">
        <DialogHeader className="min-w-0">
          <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
            {text.settings.proxyDialogTitle}
          </DialogTitle>
          <DialogDescription className="overflow-hidden break-words text-left text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
            {text.settings.proxyDialogHint}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto pr-1">
          <div className="grid grid-cols-2 gap-x-3 gap-y-2">
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.scheme}</span>
              <Select value={proxyDraft.scheme} onChange={(event) => handleProxyFieldChange("scheme", event.target.value)} className="w-full">
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
                <option value="socks5">SOCKS5</option>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.timeout}</span>
              <Input
                type="number"
                inputMode="numeric"
                value={proxyDraft.timeoutSeconds || ""}
                onChange={(event) => handleProxyFieldChange("timeoutSeconds", event.target.value)}
                placeholder="30"
                className="text-sm"
              />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.host}</span>
              <Input value={proxyDraft.host} onChange={(event) => handleProxyFieldChange("host", event.target.value)} placeholder="127.0.0.1" className="text-sm" />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.port}</span>
              <Input
                type="number"
                inputMode="numeric"
                value={proxyDraft.port || ""}
                onChange={(event) => handleProxyFieldChange("port", event.target.value)}
                placeholder="8080"
                className="text-sm"
              />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.username}</span>
              <Input value={proxyDraft.username} onChange={(event) => handleProxyFieldChange("username", event.target.value)} className="text-sm" />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.password}</span>
              <Input type="password" value={proxyDraft.password} onChange={(event) => handleProxyFieldChange("password", event.target.value)} className="text-sm" />
            </div>
            <div className="col-span-2 flex flex-col gap-1">
              <span className="text-xs text-muted-foreground">{text.settings.noProxyList}</span>
              <Input value={proxyNoProxyText} onChange={(event) => setProxyNoProxyText(event.target.value)} className="text-sm" />
            </div>
          </div>
          <div className="flex flex-col gap-2 pt-2">
            {proxyTestFeedback ? (
              <div className={cn("overflow-hidden break-words text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]", !proxyDraft.testSuccess ? "text-destructive" : "text-muted-foreground")}>
                {proxyTestFeedback}
              </div>
            ) : null}
            <div className="flex flex-nowrap items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <Button size="compact" variant="destructive" disabled={testProxy.isPending || updateSettings.isPending} onClick={() => void handleProxyClear()}>
                  {text.actions.clear}
                </Button>
                <DialogClose asChild>
                  <Button size="compact" variant="outline">
                    {text.actions.close}
                  </Button>
                </DialogClose>
              </div>
              <Button
                size="compact"
                variant={proxyDraft.testSuccess ? "secondary" : "outline"}
                disabled={!manualProxyReady || testProxy.isPending || updateSettings.isPending}
                onClick={() => void handleProxyTestAndSave()}
              >
                {testProxy.isPending || updateSettings.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {text.actions.testProxy}
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  ) : null;

  React.useEffect(() => {
    if (statusMode === "none" || !hasStatusAddress) {
      proxyCheckInFlightKeyRef.current = "";
      setProxyCheckStatus("idle");
      setProxyCheckKey("");
      return;
    }

    if (proxyCheckKey === statusKey && proxyCheckStatus !== "idle") {
      return;
    }

    void runProxyStatusCheck(statusMode, statusAddress);
  }, [hasStatusAddress, proxyCheckKey, proxyCheckStatus, runProxyStatusCheck, statusAddress, statusKey, statusMode]);

  return (
    <div className="flex h-screen flex-col overflow-hidden bg-background text-foreground">
      <header className="border-b border-border/70 bg-background/88 backdrop-blur-xl">
        <div
          className={cn(
            "wails-drag grid h-[var(--app-titlebar-height)] items-center px-4",
            isWindows
              ? "grid-cols-[minmax(var(--app-windows-caption-control-width),1fr)_auto_minmax(var(--app-windows-caption-control-width),1fr)]"
              : "grid-cols-[1fr_auto_1fr]",
          )}
        >
          <div className="justify-self-start">
            {isMac ? <div className="h-4 w-[var(--app-macos-traffic-lights-gap)]" /> : null}
          </div>

          <div className="min-w-0 px-3 text-center text-sm font-semibold text-foreground">{activeTabMeta.label}</div>

          <div className="justify-self-end">
            {isWindows ? <WindowControls platform="windows" /> : null}
          </div>
        </div>

        <div className="-mt-1 flex flex-wrap items-center justify-center gap-px px-4 pb-4 pt-0">
          {tabs.map((tab) => (
            <TabButton key={tab.id} id={tab.id} label={tab.label} icon={tab.icon} active={activeTab === tab.id} onClick={setActiveTab} />
          ))}
        </div>
      </header>

      <div className="min-h-0 flex-1 overflow-auto px-5 py-5">
        <div className="mx-auto max-w-4xl space-y-6">
          {activeTab === "general" ? (
            <>
              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.startup}>
                  <InlineSwitch
                    checked={Boolean(currentSettings?.autoStart)}
                    onChange={(checked) => void saveSettingsPatch({ autoStart: checked })}
                    ariaLabel={text.settings.startup}
                  />
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.tray}>
                  <InlineSwitch
                    checked={Boolean(currentSettings?.minimizeToTrayOnStart)}
                    onChange={(checked) => void saveSettingsPatch({ minimizeToTrayOnStart: checked })}
                    ariaLabel={text.settings.tray}
                  />
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.menuBar}>
                  <Select
                    value={currentSettings?.menuBarVisibility ?? "whenRunning"}
                    onChange={(event) => void saveSettingsPatch({ menuBarVisibility: event.target.value as "always" | "whenRunning" | "never" })}
                    className="w-48"
                  >
                    <option value="always">{text.settings.menuBarOptions.always}</option>
                    <option value="whenRunning">{text.settings.menuBarOptions.whenRunning}</option>
                    <option value="never">{text.settings.menuBarOptions.never}</option>
                  </Select>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.language}>
                  <Select
                    value={currentSettings?.language ?? "en"}
                    onChange={(event) => void saveSettingsPatch({ language: event.target.value })}
                    className="w-48"
                  >
                    <option value="en">{text.common.languages.en}</option>
                    <option value="zh-CN">{text.common.languages.zhCN}</option>
                  </Select>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              {proxySettingsCard}

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.downloadDirectory} contentClassName="min-w-0">
                  <div className="flex min-w-0 items-center justify-end gap-2">
                    <span className="max-w-[260px] truncate text-right font-mono text-xs text-muted-foreground">
                      {currentSettings?.downloadDirectory ?? ""}
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      size="compactIcon"
                      onClick={() => void chooseDownloadDir()}
                      disabled={selectDownloadDirectory.isPending}
                      title={text.actions.chooseFolder}
                      aria-label={text.actions.chooseFolder}
                    >
                      {selectDownloadDirectory.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Pencil className="h-4 w-4" />}
                    </Button>
                    {(currentSettings?.downloadDirectory ?? "").trim() ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="compactIcon"
                        onClick={() => void openLibraryPath.mutateAsync({ path: currentSettings?.downloadDirectory ?? "" })}
                        title={text.actions.open}
                        aria-label={text.actions.open}
                      >
                        <ExternalLink className="h-4 w-4" />
                      </Button>
                    ) : null}
                  </div>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.logLevel}>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="compactIcon"
                      onClick={() => void openLogDirectory.mutateAsync()}
                      title={text.actions.openLogs}
                      aria-label={text.actions.openLogs}
                    >
                      <FolderOpen className="h-4 w-4" />
                    </Button>
                    <Select
                      value={currentSettings?.logLevel ?? "info"}
                      onChange={(event) => void saveSettingsPatch({ logLevel: event.target.value })}
                      className="w-48"
                    >
                      <option value="debug">debug</option>
                      <option value="info">info</option>
                      <option value="warn">warn</option>
                      <option value="error">error</option>
                    </Select>
                  </div>
                </SettingsCompactRow>
              </SettingsCompactListCard>
            </>
          ) : null}

          {activeTab === "appearance" ? (
            <div className="space-y-4">
              <SettingsCompactListCard contentClassName="p-3">
                <TooltipProvider delayDuration={0}>
                  <div className="grid grid-cols-3 gap-2">
                    {XIA_THEME_PACKS.map((pack) => {
                      const active = appearanceDraft.themePackId === pack.id;
                      return (
                        <Tooltip key={pack.id}>
                          <TooltipTrigger asChild>
                            <button
                              type="button"
                              onClick={() => void saveAppearancePatch({ themePackId: pack.id })}
                              className={cn(
                                "app-motion-surface flex h-10 min-w-0 items-center gap-2 overflow-hidden rounded-lg border px-2 text-left",
                                active ? "border-transparent bg-accent/55" : "border-border/70 bg-background/70 hover:bg-accent/35",
                              )}
                              style={active ? { boxShadow: appearanceSelectionShadow } : undefined}
                            >
                              <span
                                className="grid h-6 w-11 shrink-0 grid-cols-[1.15fr_1fr_0.8fr] overflow-hidden rounded-md border border-black/8 bg-black/5"
                                aria-hidden="true"
                              >
                                <span style={{ backgroundColor: pack.preview.shell }} />
                                <span style={{ backgroundColor: pack.preview.sidebar }} />
                                <span style={{ backgroundColor: pack.preview.accent }} />
                              </span>
                              <span className="block min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-right text-[11px] font-medium leading-none text-foreground">
                                {text.themePacks[pack.id].label}
                              </span>
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top">{text.themePacks[pack.id].description}</TooltipContent>
                        </Tooltip>
                      );
                    })}
                  </div>
                </TooltipProvider>
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.appearanceMode}>
                  <div className="flex flex-wrap items-center gap-2">
                    {[
                      { value: "light", label: text.common.light, icon: <Sun className="h-4 w-4" /> },
                      { value: "dark", label: text.common.dark, icon: <Moon className="h-4 w-4" /> },
                      { value: "auto", label: text.common.followSystem, icon: <Monitor className="h-4 w-4" /> },
                    ].map((item) => (
                      <Button
                        key={item.value}
                        type="button"
                        variant="outline"
                        size="compact"
                        className={cn("text-[11px]", (currentSettings?.appearance ?? "auto") === item.value ? "border-transparent" : "")}
                        onClick={() => void saveSettingsPatch({ appearance: item.value as "auto" | "light" | "dark" })}
                        style={
                          currentSettings?.appearance === item.value
                            ? { boxShadow: appearanceSelectionShadow }
                            : undefined
                        }
                      >
                        {item.icon}
                        {item.label}
                      </Button>
                    ))}
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.accent}>
                  <div className="flex flex-wrap items-center gap-2">
                    {([
                      { value: "theme", label: text.settings.accentOptions.theme },
                      { value: "color", label: text.settings.accentOptions.color },
                    ] as const).map((option) => (
                      <Button
                        key={option.value}
                        type="button"
                        variant="outline"
                        size="compact"
                        className={cn("text-[11px]", appearanceDraft.accentMode === option.value ? "border-transparent" : "")}
                        onClick={() => void saveAccentMode(option.value)}
                        style={appearanceDraft.accentMode === option.value ? { boxShadow: appearanceSelectionShadow } : undefined}
                      >
                        {option.label}
                      </Button>
                    ))}
                  </div>
                </SettingsCompactRow>

                {appearanceDraft.accentMode === "color" ? (
                  <>
                    <SettingsCompactSeparator />

                    <SettingsCompactRow label={text.settings.accentColor}>
                      <TooltipProvider delayDuration={0}>
                        <div className="flex flex-wrap items-center justify-end gap-2">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type="button"
                                onClick={() => {
                                  setThemeColorDraft(SYSTEM_THEME_COLOR);
                                  void saveSettingsPatch({ themeColor: SYSTEM_THEME_COLOR });
                                }}
                                className={cn(
                                  "flex h-4 w-4 items-center justify-center rounded-full border transition hover:shadow-sm",
                                  usesSystemAccentColor ? "border-transparent" : "border-border",
                                )}
                                style={
                                  usesSystemAccentColor
                                    ? { boxShadow: `0 0 0 1px hsl(var(--border)), 0 0 0 3px ${selectedThemeColorPreview}` }
                                    : undefined
                                }
                                aria-label={text.common.followSystem}
                              >
                                <span
                                  className="h-full w-full rounded-full"
                                  style={{ backgroundColor: resolveThemeColorPreview(SYSTEM_THEME_COLOR, activeThemePack.preview.accent, currentSettings?.systemThemeColor) }}
                                />
                              </button>
                            </TooltipTrigger>
                            <TooltipContent side="top">{text.common.followSystem}</TooltipContent>
                          </Tooltip>

                          {ACCENT_SWATCHES.map((color) => {
                            const active = !usesSystemAccentColor && editableAccentColor.toLowerCase() === color.value.toLowerCase();
                            return (
                              <Tooltip key={color.value}>
                                <TooltipTrigger asChild>
                                  <button
                                    type="button"
                                    onClick={() => {
                                      setThemeColorDraft(color.value);
                                      void saveSettingsPatch({ themeColor: color.value });
                                    }}
                                    className={cn("flex h-4 w-4 items-center justify-center rounded-full border transition hover:shadow-sm", active ? "border-transparent" : "border-border")}
                                    style={active ? { boxShadow: `0 0 0 1px hsl(var(--border)), 0 0 0 3px ${color.value}` } : undefined}
                                    aria-label={text.common.colorOptions[color.id]}
                                  >
                                    <span className="h-full w-full rounded-full" style={{ backgroundColor: color.value }} />
                                  </button>
                                </TooltipTrigger>
                                <TooltipContent side="top">{text.common.colorOptions[color.id]}</TooltipContent>
                              </Tooltip>
                            );
                          })}

                          <Tooltip>
                            <TooltipTrigger asChild>
                              <input
                                type="color"
                                value={editableAccentColor}
                                onChange={(event) => {
                                  setThemeColorDraft(event.target.value);
                                  void saveSettingsPatch({ themeColor: event.target.value });
                                }}
                                className="h-4 w-4 cursor-pointer rounded-full border border-input bg-transparent p-0"
                                aria-label={text.common.customColor}
                              />
                            </TooltipTrigger>
                            <TooltipContent side="top">{text.common.customColor}</TooltipContent>
                          </Tooltip>
                        </div>
                      </TooltipProvider>
                    </SettingsCompactRow>
                  </>
                ) : null}

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.fontFamily}>
                  <Select
                    value={hasSelectedFontInList ? selectedFont : selectedFont || ""}
                    onChange={(event) => {
                      setFontFamilyDraft(event.target.value);
                      void saveSettingsPatch({ fontFamily: event.target.value });
                    }}
                    disabled={isFontFamiliesLoading}
                    className="w-48"
                  >
                    <option value="">{text.common.systemDefault}</option>
                    {!hasSelectedFontInList && selectedFont ? (
                      <option key={selectedFont} value={selectedFont} style={{ fontFamily: previewFontStack(selectedFont) }}>
                        {selectedFont} {text.common.current}
                      </option>
                    ) : null}
                    {fontOptions.map((family) => (
                      <option key={family} value={family} style={{ fontFamily: previewFontStack(family) }}>
                        {family}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.fontSize}>
                  <div className="w-24">
                    <Input
                      type="number"
                      min={12}
                      max={24}
                      step={1}
                      value={Math.min(Math.max(fontSizeDraft || 15, 12), 24)}
                      onChange={(event) => {
                        const next = Math.min(Math.max(Number.parseInt(event.target.value, 10) || 15, 12), 24);
                        setFontSizeDraft(next);
                        void saveSettingsPatch({ fontSize: next });
                      }}
                      className="w-full appearance-none text-xs"
                    />
                  </div>
                </SettingsCompactRow>
              </SettingsCompactListCard>
            </div>
          ) : null}

          {activeTab === "sprites" ? (
            <SpritesSection settings={currentSettings} text={text} saveSettingsPatch={saveSettingsPatch} />
          ) : null}

          {activeTab === "dependencies" ? (
            <SettingsCompactListCard>
              {sortedDependencies.map((dependency, index) => {
                const update = dependencyUpdatesByName.get(dependency.name);
                return (
                  <React.Fragment key={dependency.name}>
                    {index > 0 ? <SettingsCompactSeparator /> : null}
                    <DependencySettingsItem dependency={dependency} update={update} text={text} />
                  </React.Fragment>
                );
              })}
            </SettingsCompactListCard>
          ) : null}

          {activeTab === "about" ? (
            <div className="space-y-6">
              <div className="flex flex-col items-center gap-2 text-center">
                <img src="/appicon.png" alt={text.appName} className="h-16 w-16 rounded-lg shadow-sm" />
                <div className="text-lg font-semibold text-foreground">{text.appName}</div>
              </div>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.about.currentVersion}>
                  <span className="text-sm font-semibold text-foreground">{aboutVersion}</span>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.latestVersion}>
                  <div className="flex min-w-0 items-center justify-end">
                    <Badge variant="outline" className={cn("gap-1 border text-sm font-medium", latestUpdateBadgeClass)}>
                      {React.createElement(latestUpdateBadgeIcon, { className: "h-3.5 w-3.5" })}
                      {latestUpdateLabel}
                    </Badge>
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.viewChangelog}>
                  {hasReleaseNotes ? (
                    <Button type="button" variant="outline" size="compact" onClick={() => setReleaseNotesOpen(true)}>
                      {text.about.viewReleaseNotes}
                    </Button>
                  ) : (
                    <span className="text-sm text-muted-foreground">{text.about.noReleaseNotes}</span>
                  )}
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.updateStatus}>
                  <TooltipProvider delayDuration={0}>
                    <div className="flex flex-wrap items-center justify-end gap-2">
                      {showCheckUpdateAction ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="outline"
                              size="compact"
                              onClick={() => void handleCheckUpdate()}
                              disabled={!updateInfo.currentVersion.trim() || checkForUpdate.isPending || isCheckingUpdate || isDownloadingUpdate}
                              aria-label={checkUpdateLabel}
                            >
                              {isCheckingUpdate ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                              {checkUpdateLabel}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>{checkUpdateLabel}</TooltipContent>
                        </Tooltip>
                      ) : null}

                      {showInstallUpdateAction ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="outline"
                              size="compact"
                              onClick={() => void handleInstallUpdate()}
                              disabled={downloadUpdate.isPending || isDownloadingUpdate || restartToApply.isPending}
                              aria-label={text.about.downloadAndInstall}
                            >
                              {downloadUpdate.isPending || isDownloadingUpdate ? (
                                <Loader2 className="h-4 w-4 animate-spin" />
                              ) : (
                                <Download className="h-4 w-4" />
                              )}
                              {text.about.downloadAndInstall}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>{text.about.downloadAndInstall}</TooltipContent>
                        </Tooltip>
                      ) : null}

                      {isReadyToRestartUpdate ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="outline"
                              size="compact"
                              onClick={() => void handleRestartUpdate()}
                              disabled={restartToApply.isPending}
                              aria-label={text.about.restartAfterUpdate}
                            >
                              {restartToApply.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                              {text.about.restartAfterUpdate}
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>{text.about.restartAfterUpdate}</TooltipContent>
                        </Tooltip>
                      ) : null}
                    </div>
                  </TooltipProvider>
                </SettingsCompactRow>

                {showUpdateStatusRow ? (
                  <>
                    <SettingsCompactSeparator />
                    <SettingsCompactRow label={text.about.status}>
                      {isDownloadingUpdate ? (
                        <div className="w-[220px] max-w-full space-y-1.5">
                          <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                            <div
                              className="h-full bg-primary transition-[width]"
                              style={{ width: `${Math.min(Math.max(updateInfo.progress, 0), 100)}%` }}
                            />
                          </div>
                          <div className="flex items-center justify-between text-sm text-muted-foreground">
                            <span>{updateInfo.status === "installing" ? text.about.installing : text.about.downloading}</span>
                            <span>{Math.round(updateInfo.progress)}%</span>
                          </div>
                        </div>
                      ) : (
                        <span className="max-w-[280px] whitespace-pre-wrap break-words text-right text-sm text-destructive">
                          {updateErrorMessage}
                        </span>
                      )}
                    </SettingsCompactRow>
                  </>
                ) : null}
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.about.craftedBy}>
                  <span className="text-sm font-semibold text-foreground">{ABOUT_AUTHOR_NAME}</span>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.contact}>
                  <TooltipProvider delayDuration={0}>
                    <div className="flex items-center gap-2">
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            onClick={() => openExternalURL("mailto:xunruhao@gmail.com")}
                            aria-label={text.about.email}
                          >
                            <Mail className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="top">{text.about.email}</TooltipContent>
                      </Tooltip>

                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            onClick={() => openExternalURL("https://x.com/ArnoldHaoCA")}
                            aria-label={text.about.twitter}
                          >
                            <Twitter className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="top">{text.about.twitter}</TooltipContent>
                      </Tooltip>

                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            onClick={() => openExternalURL("https://xiadown.dreamapp.cc/")}
                            aria-label={text.about.website}
                          >
                            <Globe className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="top">{text.about.website}</TooltipContent>
                      </Tooltip>

                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            onClick={() => openExternalURL("https://github.com/arnoldhao/xiadown")}
                            aria-label={text.about.github}
                          >
                            <Github className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="top">{text.about.github}</TooltipContent>
                      </Tooltip>
                    </div>
                  </TooltipProvider>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.feedback}>
                  <TooltipProvider delayDuration={0}>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type="button"
                          variant="outline"
                          size="compactIcon"
                          onClick={() => openExternalURL("https://github.com/arnoldhao/xiadown/issues")}
                          aria-label={text.about.sendFeedback}
                        >
                          <MessageSquare className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent side="top">{text.about.sendFeedback}</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <div className="space-y-2">
                <div className="pl-3 text-sm font-bold text-foreground">{text.settings.otherSoftware}</div>
                <SettingsCompactListCard contentClassName="p-3">
                  <div className="flex min-w-0 items-center justify-between gap-3 rounded-xl border border-input bg-background px-3 py-2.5 shadow-sm">
                    <div className="flex min-w-0 items-center gap-3">
                      <img src="/dreamcreator.png" alt="" className="h-10 w-10 shrink-0 rounded-lg shadow-sm" />
                      <div className="min-w-0">
                        <div className="truncate text-sm font-semibold text-foreground">{text.about.dreamCreator}</div>
                        <div className="truncate text-xs text-muted-foreground">{text.about.dreamCreatorDescription}</div>
                      </div>
                    </div>
                    <Button type="button" variant="outline" size="compact" onClick={() => openExternalURL("https://dreamcreator.dreamapp.cc/")}>
                      {text.about.website}
                    </Button>
                  </div>
                </SettingsCompactListCard>
              </div>
            </div>
          ) : null}
        </div>
      </div>

      {proxyDialog}

      <Dialog open={releaseNotesOpen} onOpenChange={setReleaseNotesOpen}>
        <DialogContent className="grid max-h-[min(34rem,calc(100vh-2rem))] w-[min(28rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {text.about.viewChangelog}
            </DialogTitle>
          </DialogHeader>
          <div className="min-h-0 overflow-y-auto pr-1">
            <DialogMarkdown content={releaseNotes} className="max-h-none overflow-visible" />
          </div>
          <DialogFooter className="shrink-0">
            <Button type="button" variant="ghost" size="compact" onClick={() => setReleaseNotesOpen(false)}>
              {text.about.releaseNotesClose}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
