import { Browser,Events,System } from "@wailsio/runtime";
import {
AlertCircle,
ArrowUpCircle,
CheckCircle2,
CircleHelp,
Cog,
Download,
ExternalLink,
FolderOpen,
Github,
Globe,
Headphones,
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
Trash2,
Twitter,
} from "lucide-react";
import * as React from "react";

import { ACCENT_SWATCHES,CORE_DEPENDENCY_ORDER,DependencySettingsItem,InlineSwitch,SYSTEM_THEME_COLOR,TabButton,formatHostPort,normalizeProxy,parseNoProxy,previewFontStack,resetProxyTestState,resolveAccentColor,resolveTabFromSection,resolveThemeColorPreview,resolveThemeColorSelection } from "@/app/settings/settings-helpers";
import { WindowControls } from "@/components/layout/WindowControls";
import { EqualizerSection } from "@/features/settings/equalizer";
import { getXiaText } from "@/features/xiadown/shared";
import {
XIA_THEME_PACKS,
mergeXiaAppearanceConfig,
readXiaAppearance,
resolveThemePack,
type XiaAccentMode,
type XiaAppearanceSettings,
type XiaSidebarStyle,
} from "@/shared/styles/xiadown-theme";
import { cn } from "@/lib/utils";
import type { BrowserCandidate, PlaybackAudioQualityPreference, ProxySettings, ResourceSniffScope } from "@/shared/contracts/settings";
import { DialogMarkdown } from "@/shared/markdown/dialog-markdown";
import {
useDependencies,
useDependencyUpdates
} from "@/shared/query/dependencies";
import { useOpenLibraryPath } from "@/shared/query/library";
import {
useBrowserCandidates,
useClearSniffProfile,
useOpenLogDirectory,
useOpenSniffProfile,
useRefreshBrowserCandidates,
useSelectDownloadDirectory,
useSettings,
useSniffProfileInfo,
useSystemProxyInfo,
useTestProxy,
useUpdateSettings,
} from "@/shared/query/settings";
import { useFontFamilies,useLyricsTranscriptionAvailable } from "@/shared/query/system";
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
import { formatBytes } from "@/shared/utils/formatBytes";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogScrollArea,
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
const DREAM_CREATOR_ICON_SRC = "/dreamcreator.png";
const HUSH_ICON_SRC = "/hush.png";
const DREAM_APP_ICON_FALLBACK_SRC = "/appicon.png";

function formatSniffProfileBytes(value?: number | null): string {
  return value && value > 0 ? formatBytes(value) : "0 MB";
}

function DreamAppIcon(props: {
  src: string;
}) {
  const [source, setSource] = React.useState(props.src);

  React.useEffect(() => {
    setSource(props.src);
  }, [props.src]);

  return (
    <div className="app-settings-dream-app-icon" aria-hidden="true">
      <img
        key={source}
        src={source}
        alt=""
        draggable={false}
        decoding="async"
        onError={() => {
          if (source !== DREAM_APP_ICON_FALLBACK_SRC) {
            setSource(DREAM_APP_ICON_FALLBACK_SRC);
          }
        }}
      />
    </div>
  );
}

function sortBrowserCandidates(candidates: BrowserCandidate[]) {
  return [...candidates]
    .filter((candidate) => candidate.available && candidate.id.trim() && candidate.label.trim())
    .sort((left, right) => {
      const labelCompare = left.label.localeCompare(right.label, undefined, { sensitivity: "base" });
      if (labelCompare !== 0) {
        return labelCompare;
      }
      return left.id.localeCompare(right.id);
    });
}

function resolveSniffBrowserID(candidates: BrowserCandidate[]) {
  const chrome = candidates.find((candidate) => candidate.id === "chrome");
  return chrome?.id ?? candidates[0]?.id ?? "";
}

export function SettingsApp() {
  const settings = useSettingsStore((state) => state.settings);
  const { data: liveSettings } = useSettings();
  const browserCandidatesQuery = useBrowserCandidates();
  const refreshBrowserCandidates = useRefreshBrowserCandidates();
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
  const lyricsTranscriptionAvailability = useLyricsTranscriptionAvailable(isMac);
  const lyricsTranscriptionAvailable =
    isMac && lyricsTranscriptionAvailability.data === true;
  const [activeTab, setActiveTab] = React.useState<XiaSettingsTabId>("general");
  const resolveVisibleSettingsTab = React.useCallback(
    (tab: XiaSettingsTabId) => tab,
    [],
  );
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
      setActiveTab(resolveVisibleSettingsTab(pending));
    }
    const unsubscribe = listenPendingSettingsTab((tab) =>
      setActiveTab(resolveVisibleSettingsTab(tab)),
    );
    const offNavigate = Events.On("settings:navigate", (event: any) => {
      const target = typeof (event?.data ?? event) === "string" ? (event?.data ?? event) : "";
      setActiveTab(resolveVisibleSettingsTab(resolveTabFromSection(target)));
    });
    return () => {
      unsubscribe();
      offNavigate();
    };
  }, [resolveVisibleSettingsTab]);

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
  const browserOptions = React.useMemo(
    () => sortBrowserCandidates(browserCandidatesQuery.data ?? []),
    [browserCandidatesQuery.data],
  );
  const selectedSniffBrowser = React.useMemo(() => {
    const saved = (currentSettings?.sniffBrowser ?? "").trim();
    if (saved && browserOptions.some((candidate) => candidate.id === saved)) {
      return saved;
    }
    return resolveSniffBrowserID(browserOptions);
  }, [browserOptions, currentSettings?.sniffBrowser]);
  const sniffProfileInfo = useSniffProfileInfo(selectedSniffBrowser);
  const openSniffProfile = useOpenSniffProfile();
  const clearSniffProfile = useClearSniffProfile();
  const browserDataSizeLabel = formatSniffProfileBytes(sniffProfileInfo.data?.sizeBytes);

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
      return "app-dream-status-badge-primary";
    }
    if (isUpdateError) {
      return "app-dream-status-badge-danger";
    }
    return "app-dream-status-badge-success";
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

  async function handleRefreshBrowserCandidates() {
    try {
      await refreshBrowserCandidates.mutateAsync();
    } catch (error) {
      console.warn(error);
    }
  }

  function renderLyricsTranscriptionSwitch(props: {
    checked: boolean;
    onChange: (checked: boolean) => void;
    ariaLabel: string;
  }) {
    const switchElement = (
      <InlineSwitch
        checked={lyricsTranscriptionAvailable && props.checked}
        disabled={!lyricsTranscriptionAvailable}
        onChange={props.onChange}
        ariaLabel={props.ariaLabel}
      />
    );
    if (lyricsTranscriptionAvailable) {
      return switchElement;
    }
    return (
      <TooltipProvider delayDuration={0}>
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="inline-flex">{switchElement}</span>
          </TooltipTrigger>
          <TooltipContent side="top">{text.settings.macOSOnly}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    );
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
    { id: "general", label: text.settings.tabs.general, icon: <Cog className="h-[26px] w-[26px]" /> },
    { id: "appearance", label: text.settings.tabs.appearance, icon: <Palette className="h-[26px] w-[26px]" /> },
    { id: "player", label: text.settings.tabs.player, icon: <Headphones className="h-[26px] w-[26px]" /> },
    { id: "download", label: text.settings.tabs.download, icon: <Download className="h-[26px] w-[26px]" /> },
    { id: "about", label: text.settings.tabs.about, icon: <Info className="h-[26px] w-[26px]" /> },
  ];
  const visibleTabs = tabs;
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
  const statusDotValue =
    proxyCheckStatus === "available"
      ? "available"
      : proxyCheckStatus === "unavailable"
        ? "unavailable"
        : "idle";
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
  const activeSegmentStyle: React.CSSProperties = {
    backgroundColor: "hsl(var(--primary) / 0.13)",
    color: "hsl(var(--primary))",
  };
  const resourceSniffScopeOptions: Array<{
    value: ResourceSniffScope;
    label: string;
  }> = [
    {
      value: "default",
      label: text.settings.resourceSniffScopeOptions.default,
    },
    {
      value: "advanced",
      label: text.settings.resourceSniffScopeOptions.advanced,
    },
    {
      value: "all",
      label: text.settings.resourceSniffScopeOptions.all,
    },
  ];
  const playbackAudioQualityOptions: Array<{
    value: PlaybackAudioQualityPreference;
    label: string;
  }> = [
    { value: "AUDIO_QUALITY_AUTO", label: text.settings.playbackAudioQualityOptions.auto },
    { value: "AUDIO_QUALITY_LOW", label: text.settings.playbackAudioQualityOptions.low },
    { value: "AUDIO_QUALITY_MEDIUM", label: text.settings.playbackAudioQualityOptions.medium },
    { value: "AUDIO_QUALITY_HIGH", label: text.settings.playbackAudioQualityOptions.high },
  ];
  const resourceSniffMinBytesOptions = [
    { value: 8 * 1024, label: text.settings.resourceSniffMinBytesOptions.kb8 },
    { value: 16 * 1024, label: text.settings.resourceSniffMinBytesOptions.kb16 },
    { value: 64 * 1024, label: text.settings.resourceSniffMinBytesOptions.kb64 },
    { value: 256 * 1024, label: text.settings.resourceSniffMinBytesOptions.kb256 },
  ];
  const resourceSniffRetainOptions = [
    { value: 500, label: "500" },
    { value: 1000, label: "1000" },
    { value: 2000, label: "2000" },
    { value: 5000, label: "5000" },
  ];
  const ytdlpConcurrentFragmentOptions = [1, 2, 4, 8, 16].map((value) => ({
    value,
    label: String(value),
  }));
  const ytdlpConcurrentDownloadOptions = [1, 2, 3, 4, 5].map((value) => ({
    value,
    label: String(value),
  }));
  const dreamApps = [
    {
      id: "dreamcreator",
      name: text.about.dreamCreator,
      description: text.about.dreamCreatorDescription,
      url: "https://dreamcreator.dreamapp.cc/",
      iconSrc: DREAM_CREATOR_ICON_SRC,
    },
    {
      id: "hush",
      name: text.about.hush,
      description: text.about.hushDescription,
      url: "https://dreamapp.cc/",
      iconSrc: HUSH_ICON_SRC,
    },
  ];
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
      <SettingsCompactRow label={text.settings.proxy} contentClassName="min-w-0">
        <div className="grid min-w-0 max-w-full grid-cols-3 gap-2">
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
              className="min-w-0 px-2"
              onClick={() => handleProxyModeChange(option.value)}
              style={proxyDraft.mode === option.value ? activeSegmentStyle : undefined}
            >
              <span className="min-w-0 truncate">{option.label}</span>
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
                <span className="app-settings-status-badge shrink-0">
                  {systemSourceLabel}
                </span>
              ) : null}
              <span className="app-settings-path-value min-w-0 max-w-[260px] flex-1 truncate text-right font-mono">
                {statusAddressDisplay}
              </span>
              {hasStatusAddress ? (
                <span className="inline-flex items-center">
                <span className={cn("app-settings-status-dot h-2 w-2 rounded-full", isChecking ? "animate-pulse" : "")} data-status={statusDotValue} aria-hidden="true" />
                </span>
              ) : null}
              {showRefreshButton ? (
                <Button
                  type="button"
                  variant="outline"
                  size="compactIcon"
                  className="shrink-0"
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
                  className="shrink-0"
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
        <DialogScrollArea className="min-h-0">
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
              <div className="app-dream-status-message overflow-hidden break-words px-3 py-2 text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]" data-intent={!proxyDraft.testSuccess ? "danger" : "success"}>
                {proxyTestFeedback}
              </div>
            ) : null}
            <div className="app-dialog-footer flex flex-nowrap items-center justify-between gap-2">
              <div className="flex min-w-0 items-center gap-2">
                <Button variant="destructive" disabled={testProxy.isPending || updateSettings.isPending} onClick={() => void handleProxyClear()}>
                  {text.actions.clear}
                </Button>
                <DialogClose asChild>
                  <Button variant="outline">
                    {text.actions.close}
                  </Button>
                </DialogClose>
              </div>
              <Button
                variant={proxyDraft.testSuccess ? "secondary" : "outline"}
                disabled={!manualProxyReady || testProxy.isPending || updateSettings.isPending}
                onClick={() => void handleProxyTestAndSave()}
              >
                {testProxy.isPending || updateSettings.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                {text.actions.testProxy}
              </Button>
            </div>
          </div>
        </DialogScrollArea>
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
    <div className="app-dream-window app-settings-window flex h-screen flex-col overflow-hidden text-foreground">
      <header className="app-dream-header">
        <div
          className={cn(
            "wails-drag grid h-[var(--app-page-top-drag-height)] items-center px-4",
            isWindows
              ? "grid-cols-[minmax(var(--app-windows-caption-control-width),1fr)_auto_minmax(var(--app-windows-caption-control-width),1fr)]"
              : "grid-cols-[1fr_auto_1fr]",
          )}
        >
          <div className="justify-self-start">
            {isMac ? <div className="h-4 w-[var(--app-macos-traffic-lights-gap)]" /> : null}
          </div>

          <div aria-hidden="true" />

          <div className="justify-self-end">
            {isWindows ? <WindowControls platform="windows" /> : null}
          </div>
        </div>

        <div className="app-dream-tabs-bar -mt-1 flex flex-wrap items-center justify-center px-4 pt-0">
          {visibleTabs.map((tab) => (
            <TabButton key={tab.id} id={tab.id} label={tab.label} icon={tab.icon} active={activeTab === tab.id} onClick={setActiveTab} />
          ))}
        </div>
      </header>

      <div className="app-dream-content min-h-0 flex-1 overflow-auto">
        <div
          key={activeTab}
          className="app-settings-tab-content mx-auto max-w-4xl space-y-6"
          data-tab={activeTab}
        >
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
                    <option value="zh-TW">{text.common.languages.zhTW}</option>
                    <option value="ja-JP">{text.common.languages.jaJP}</option>
                    <option value="ko-KR">{text.common.languages.koKR}</option>
                    <option value="es-419">{text.common.languages.es419}</option>
                    <option value="pt-BR">{text.common.languages.ptBR}</option>
                    <option value="id-ID">{text.common.languages.idID}</option>
                    <option value="vi-VN">{text.common.languages.viVN}</option>
                  </Select>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              {proxySettingsCard}

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
                              className="app-settings-theme-pack-button app-motion-surface flex h-11 min-w-0 items-center gap-2 overflow-hidden px-2 text-left"
                              data-active={active ? "true" : undefined}
                              style={active ? activeSegmentStyle : undefined}
                            >
                              <span
                                className="app-settings-theme-preview grid h-6 w-11 shrink-0 grid-cols-[1.15fr_1fr_0.8fr] overflow-hidden"
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
                  <div className="grid min-w-0 max-w-full grid-cols-3 gap-2">
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
                        className={cn("min-w-0 px-2 text-[11px]", (currentSettings?.appearance ?? "auto") === item.value ? "border-transparent" : "")}
                        onClick={() => void saveSettingsPatch({ appearance: item.value as "auto" | "light" | "dark" })}
                        style={currentSettings?.appearance === item.value ? activeSegmentStyle : undefined}
                      >
                        <span className="shrink-0">{item.icon}</span>
                        <span className="min-w-0 truncate">{item.label}</span>
                      </Button>
                    ))}
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.sidebarStyle}>
                  <div className="grid min-w-0 max-w-full grid-cols-3 gap-2">
                    {([
                      { value: "glass", label: text.settings.sidebarStyleOptions.glass },
                      { value: "contrast", label: text.settings.sidebarStyleOptions.contrast },
                      { value: "pixel", label: text.settings.sidebarStyleOptions.pixel },
                    ] as const satisfies Array<{ value: XiaSidebarStyle; label: string }>).map((option) => (
                      <Button
                        key={option.value}
                        type="button"
                        variant="outline"
                        size="compact"
                        className={cn("min-w-0 px-2 text-[11px]", appearanceDraft.sidebarStyle === option.value ? "border-transparent" : "")}
                        onClick={() => void saveAppearancePatch({ sidebarStyle: option.value })}
                        style={appearanceDraft.sidebarStyle === option.value ? activeSegmentStyle : undefined}
                      >
                        <span className="min-w-0 truncate">{option.label}</span>
                      </Button>
                    ))}
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.accent}>
                  <div className="grid min-w-0 max-w-full grid-cols-2 gap-2">
                    {([
                      { value: "theme", label: text.settings.accentOptions.theme },
                      { value: "color", label: text.settings.accentOptions.color },
                    ] as const).map((option) => (
                      <Button
                        key={option.value}
                        type="button"
                        variant="outline"
                        size="compact"
                        className={cn("min-w-0 px-2 text-[11px]", appearanceDraft.accentMode === option.value ? "border-transparent" : "")}
                        onClick={() => void saveAccentMode(option.value)}
                        style={appearanceDraft.accentMode === option.value ? activeSegmentStyle : undefined}
                      >
                        <span className="min-w-0 truncate">{option.label}</span>
                      </Button>
                    ))}
                  </div>
                </SettingsCompactRow>

                {appearanceDraft.accentMode === "color" ? (
                  <>
                    <SettingsCompactSeparator />

                    <SettingsCompactRow label={text.settings.accentColor}>
                      <TooltipProvider delayDuration={0}>
                        <div className="flex min-w-0 flex-nowrap items-center justify-end gap-2 overflow-hidden">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type="button"
                                onClick={() => {
                                  setThemeColorDraft(SYSTEM_THEME_COLOR);
                                  void saveSettingsPatch({ themeColor: SYSTEM_THEME_COLOR });
                                }}
                                className="app-settings-swatch flex h-4 w-4 items-center justify-center transition"
                                data-active={usesSystemAccentColor ? "true" : undefined}
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
                                    className="app-settings-swatch flex h-4 w-4 items-center justify-center transition"
                                    data-active={active ? "true" : undefined}
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
                                className="app-settings-swatch h-4 w-4 cursor-pointer bg-transparent p-0"
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

          {activeTab === "player" ? (
            <div className="space-y-6">
              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.playbackAudioQuality}>
                  <Select
                    value={currentSettings?.playbackAudioQuality ?? "AUDIO_QUALITY_AUTO"}
                    onChange={(event) =>
                      void saveSettingsPatch({
                        playbackAudioQuality: event.target.value as PlaybackAudioQualityPreference,
                      })
                    }
                    className="w-48"
                  >
                    {playbackAudioQualityOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.syncedLyrics}>
                  <InlineSwitch
                    checked={currentSettings?.syncedLyricsEnabled !== false}
                    onChange={(checked) => void saveSettingsPatch({ syncedLyricsEnabled: checked })}
                    ariaLabel={text.settings.syncedLyrics}
                  />
                </SettingsCompactRow>

                {!isWindows ? (
                  <>
                    <SettingsCompactSeparator />

                    <SettingsCompactRow label={text.settings.romanizedLyrics}>
                      {renderLyricsTranscriptionSwitch({
                        checked: currentSettings?.romanizedLyrics !== false,
                        onChange: (checked) => void saveSettingsPatch({ romanizedLyrics: checked }),
                        ariaLabel: text.settings.romanizedLyrics,
                      })}
                    </SettingsCompactRow>

                    <SettingsCompactSeparator />

                    <SettingsCompactRow label={text.settings.pinyinLyrics}>
                      {renderLyricsTranscriptionSwitch({
                        checked: currentSettings?.pinyinLyrics !== false,
                        onChange: (checked) => void saveSettingsPatch({ pinyinLyrics: checked }),
                        ariaLabel: text.settings.pinyinLyrics,
                      })}
                    </SettingsCompactRow>
                  </>
                ) : null}
              </SettingsCompactListCard>

              <EqualizerSection isMac={isMac} isWindows={isWindows} text={text} />
            </div>
          ) : null}

          {activeTab === "download" ? (
            <>
              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.sniffBrowser}>
                  <div className="flex items-center gap-2">
                    <TooltipProvider delayDuration={0}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            onClick={() => void handleRefreshBrowserCandidates()}
                            disabled={refreshBrowserCandidates.isPending}
                            aria-label={text.settings.refreshBrowsers}
                          >
                            {refreshBrowserCandidates.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{text.settings.refreshBrowsers}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                    <Select
                      value={selectedSniffBrowser}
                      onChange={(event) => void saveSettingsPatch({ sniffBrowser: event.target.value })}
                      className="w-48"
                      disabled={browserOptions.length === 0}
                    >
                      {browserOptions.length === 0 ? (
                        <option value="">{browserCandidatesQuery.isLoading ? text.settings.checking : text.settings.unavailable}</option>
                      ) : (
                        browserOptions.map((candidate) => (
                          <option key={candidate.id} value={candidate.id}>
                            {candidate.label}
                          </option>
                        ))
                      )}
                    </Select>
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.browserData} contentClassName="min-w-0">
                  <div className="flex min-w-0 items-center justify-end gap-2">
                    <TooltipProvider delayDuration={0}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span
                            tabIndex={0}
                            className="app-settings-path-value min-w-0 max-w-[260px] flex-1 truncate text-right font-mono outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
                            aria-label={`${text.settings.browserDataSize}: ${sniffProfileInfo.isFetching ? "..." : browserDataSizeLabel}`}
                          >
                            {sniffProfileInfo.isFetching ? "..." : browserDataSizeLabel}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>{text.settings.browserDataSize}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                    <TooltipProvider delayDuration={0}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            className="shrink-0"
                            onClick={() => void openSniffProfile.mutateAsync({ browser: selectedSniffBrowser })}
                            disabled={!selectedSniffBrowser || openSniffProfile.isPending}
                            aria-label={text.settings.browserDataOpen}
                          >
                            {openSniffProfile.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <FolderOpen className="h-4 w-4" />}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{text.settings.browserDataOpen}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                    <TooltipProvider delayDuration={0}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="outline"
                            size="compactIcon"
                            className="shrink-0"
                            onClick={() =>
                              void clearSniffProfile
                                .mutateAsync({ browser: selectedSniffBrowser })
                                .then(() => sniffProfileInfo.refetch())
                            }
                            disabled={!selectedSniffBrowser || clearSniffProfile.isPending}
                            aria-label={text.settings.browserDataClear}
                          >
                            {clearSniffProfile.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>{text.settings.browserDataClear}</TooltipContent>
                      </Tooltip>
                    </TooltipProvider>
                  </div>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.resourceSniffScope}>
                  <Select
                    value={currentSettings?.resourceSniffScope ?? "default"}
                    onChange={(event) => void saveSettingsPatch({ resourceSniffScope: event.target.value as ResourceSniffScope })}
                    className="w-48"
                  >
                    {resourceSniffScopeOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.resourceSniffMinBytes}>
                  <Select
                    value={String(currentSettings?.resourceSniffMinBytes ?? 8 * 1024)}
                    onChange={(event) => void saveSettingsPatch({ resourceSniffMinBytes: Number(event.target.value) })}
                    className="w-48"
                  >
                    {resourceSniffMinBytesOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.resourceSniffRetain}>
                  <Select
                    value={String(currentSettings?.resourceSniffRetain ?? 1000)}
                    onChange={(event) => void saveSettingsPatch({ resourceSniffRetain: Number(event.target.value) })}
                    className="w-48"
                  >
                    {resourceSniffRetainOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.settings.downloadDirectory} contentClassName="min-w-0">
                  <div className="flex min-w-0 items-center justify-end gap-2">
                    <span className="app-settings-path-value min-w-0 max-w-[260px] flex-1 truncate text-right font-mono">
                      {currentSettings?.downloadDirectory ?? ""}
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      size="compactIcon"
                      className="shrink-0"
                      onClick={() => void chooseDownloadDir()}
                      disabled={selectDownloadDirectory.isPending}
                      title={text.actions.chooseFolder}
                      aria-label={text.actions.chooseFolder}
                    >
                      {selectDownloadDirectory.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Pencil className="h-4 w-4" />
                      )}
                    </Button>
                    {(currentSettings?.downloadDirectory ?? "").trim() ? (
                      <Button
                        type="button"
                        variant="outline"
                        size="compactIcon"
                        className="shrink-0"
                        onClick={() =>
                          void openLibraryPath.mutateAsync({
                            path: currentSettings?.downloadDirectory ?? "",
                          })
                        }
                        title={text.actions.open}
                        aria-label={text.actions.open}
                      >
                        <ExternalLink className="h-4 w-4" />
                      </Button>
                    ) : null}
                  </div>
                </SettingsCompactRow>
                <SettingsCompactSeparator />
                <SettingsCompactRow
                  label={
                    <span className="inline-flex min-w-0 items-center gap-1.5">
                      <span className="truncate">
                        {text.settings.ytdlpConcurrentDownloads}
                      </span>
                      <TooltipProvider delayDuration={0}>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button
                              type="button"
                              className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-muted-foreground transition hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                              aria-label={text.settings.ytdlpConcurrentDownloadsHelp}
                            >
                              <CircleHelp className="h-3.5 w-3.5" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent
                            side="top"
                            multiline
                            className="!max-w-[15rem] text-left leading-relaxed"
                          >
                            {text.settings.ytdlpConcurrentDownloadsHelp}
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </span>
                  }
                >
                  <Select
                    value={String(
                      currentSettings?.ytdlpConcurrentDownloads ?? 3,
                    )}
                    onChange={(event) =>
                      void saveSettingsPatch({
                        ytdlpConcurrentDownloads: Number(event.target.value),
                      })
                    }
                    className="w-48"
                  >
                    {ytdlpConcurrentDownloadOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>
                <SettingsCompactSeparator />
                <SettingsCompactRow
                  label={
                    <span className="inline-flex min-w-0 items-center gap-1.5">
                      <span className="truncate">{text.settings.ytdlpConcurrentFragments}</span>
                      <TooltipProvider delayDuration={0}>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button
                              type="button"
                              className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-muted-foreground transition hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                              aria-label={text.settings.ytdlpConcurrentFragmentsHelp}
                            >
                              <CircleHelp className="h-3.5 w-3.5" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top" multiline className="!max-w-[15rem] text-left leading-relaxed">
                            {text.settings.ytdlpConcurrentFragmentsHelp}
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </span>
                  }
                >
                  <Select
                    value={String(currentSettings?.ytdlpConcurrentFragments ?? 1)}
                    onChange={(event) => void saveSettingsPatch({ ytdlpConcurrentFragments: Number(event.target.value) })}
                    className="w-48"
                  >
                    {ytdlpConcurrentFragmentOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                </SettingsCompactRow>
              </SettingsCompactListCard>

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
            </>
          ) : null}

          {activeTab === "about" ? (
            <div className="space-y-6">
              <div className="flex flex-col items-center gap-2 text-center">
                <img src="/appicon.png" alt={text.appName} className="app-settings-app-icon h-16 w-16" />
                <div className="text-lg font-semibold text-foreground">{text.appName}</div>
              </div>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.about.currentVersion}>
                  <span className="text-sm font-semibold text-foreground">{aboutVersion}</span>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.latestVersion}>
                  <div className="flex min-w-0 items-center justify-end">
                    <Badge variant="outline" className={cn("gap-1 text-sm font-medium", latestUpdateBadgeClass)}>
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
                          <div className="app-dream-progress-track h-2 w-full">
                            <div
                              className="app-dream-progress-value"
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
                            onClick={() => openExternalURL("https://xiadown.app/")}
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
                <div className="app-settings-dream-app-title">{text.settings.otherSoftware}</div>
                <SettingsCompactListCard contentClassName="app-settings-dream-app-card">
                  {dreamApps.map((app, index) => (
                    <div
                      key={app.id}
                      className={cn(
                        "app-settings-dream-app-item",
                        index > 0 ? "app-settings-dream-app-item-bordered" : "",
                      )}
                    >
                      <DreamAppIcon src={app.iconSrc} />
                      <div className="app-settings-dream-app-text">
                        <div className="app-settings-dream-app-name">{app.name}</div>
                        <div className="app-settings-dream-app-description">{app.description}</div>
                      </div>
                      <Button
                        type="button"
                        variant="outline"
                        size="compact"
                        className="app-settings-dream-app-link"
                        onClick={() => openExternalURL(app.url)}
                      >
                        <Globe className="h-3.5 w-3.5" />
                        {text.about.website}
                      </Button>
                    </div>
                  ))}
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
          <DialogScrollArea className="min-h-0">
            <DialogMarkdown content={releaseNotes} className="max-h-none overflow-visible" />
          </DialogScrollArea>
          <DialogFooter className="shrink-0 items-end sm:items-center">
            <Button type="button" variant="ghost" size="compact" onClick={() => setReleaseNotesOpen(false)}>
              {text.about.releaseNotesClose}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
