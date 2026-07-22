import { Browser,Events,System } from "@wailsio/runtime";
import {
AlertCircle,
ArrowUpCircle,
CheckCircle2,
ChevronRight,
CircleHelp,
Cog,
Database,
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
Network,
Palette,
Pencil,
RefreshCcw,
RefreshCw,
Sparkles,
Sun,
Twitter,
} from "lucide-react";
import * as React from "react";

import { ACCENT_SWATCHES,CORE_DEPENDENCY_ORDER,DependencySettingsItem,InlineSwitch,SYSTEM_THEME_COLOR,TabButton,formatHostPort,normalizeProxy,parseNoProxy,previewFontStack,resetProxyTestState,resolveAccentColor,resolveTabFromSection,resolveThemeColorPreview,resolveThemeColorSelection } from "@/app/settings/settings-helpers";
import { WindowControls } from "@/components/layout/WindowControls";
import { EqualizerSection } from "@/features/settings/equalizer";
import { LibraryAccessSettingsCard } from "@/app/settings/LibraryAccessSettingsCard";
import { BrowserProfilesSheet, DataManagementSheet } from "@/app/settings/SettingsDataSheets";
import { getXiaText } from "@/features/xiadown/shared";
import {
XIA_THEME_PACKS,
mergeXiaAppearanceConfig,
readXiaAppearance,
type XiaAccentMode,
type XiaAppearanceSettings,
type XiaSurfaceStyle,
} from "@/shared/styles/xiadown-theme";
import {
  readWindowSurfaceStyleHint,
  useWindowMaterialMode,
} from "@/shared/styles/window-material";
import { cn } from "@/lib/utils";
import type { PlaybackAudioQualityPreference, ProxySettings, ResourceSniffScope } from "@/shared/contracts/settings";
import { useI18n } from "@/shared/i18n";
import { DialogMarkdown } from "@/shared/markdown/dialog-markdown";
import {
useDependencies,
useDependencyUpdates
} from "@/shared/query/dependencies";
import { useDataManagementSnapshot } from "@/shared/query/dataManagement";
import { useOpenLibraryPath } from "@/shared/query/library";
import {
useOpenLogDirectory,
useRefreshSystemProxyInfo,
useSelectDownloadDirectory,
useSettings,
useSystemProxyInfo,
useTestProxy,
useUpdateSettings,
} from "@/shared/query/settings";
import { openExternalURL, useFontFamilies } from "@/shared/query/system";
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
import { Button } from "@/shared/ui/button";
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
  StatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";
import {
SettingsCompactListCard,
SettingsCompactRow,
SettingsCompactSeparator,
} from "@/shared/ui/settings-layout";
import {
  defineWorkspacePageContract,
  WorkspacePage,
  WorkspacePageContent,
} from "@/shared/ui/workspace-page";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
import { formatBytes } from "@/shared/utils/formatBytes";
import {
consumePendingSettingsTab,
listenPendingSettingsTab,
type XiaSettingsTabId
} from "./sectionStorage";

const ABOUT_AUTHOR_NAME = "Arnold HAO";
const ABOUT_CONTACT_EMAIL_URL = "mailto:xunruhao@gmail.com";
const DREAM_CREATOR_ICON_SRC = "/dreamcreator.png";
const HUSH_ICON_SRC = "/hush.png";
const DREAM_APP_ICON_FALLBACK_SRC = "/appicon.png";
const THIRD_PARTY_NOTICES_SRC = "/THIRD_PARTY_NOTICES.txt";

function openAboutContactEmail() {
  void Browser.OpenURL(ABOUT_CONTACT_EMAIL_URL).catch((error) => {
    console.warn("[Settings] failed to open contact email", error);
  });
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

export function SettingsApp() {
  const { t } = useI18n();
  const settings = useSettingsStore((state) => state.settings);
  const { data: liveSettings } = useSettings();
  const { data: fontFamilies = [], isLoading: isFontFamiliesLoading } = useFontFamilies();
  const updateSettings = useUpdateSettings();
  const selectDownloadDirectory = useSelectDownloadDirectory();
  const openLibraryPath = useOpenLibraryPath();
  const openLogDirectory = useOpenLogDirectory();
  const testProxy = useTestProxy();
  const systemProxyQuery = useSystemProxyInfo(true);
	const refreshSystemProxyInfo = useRefreshSystemProxyInfo();
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
  const resolveVisibleSettingsTab = React.useCallback(
    (tab: XiaSettingsTabId) => tab,
    [],
  );
  const [proxyDraft, setProxyDraft] = React.useState<ProxySettings>(() => normalizeProxy(currentSettings?.proxy));
  const [proxyNoProxyText, setProxyNoProxyText] = React.useState("");
  const initialSurfaceStyle = readWindowSurfaceStyleHint();
  const [appearanceDraft, setAppearanceDraft] = React.useState<XiaAppearanceSettings>(() => {
    const appearance = readXiaAppearance(currentSettings);
    if (
      !currentSettings &&
      (initialSurfaceStyle === "glass" || initialSurfaceStyle === "contrast")
    ) {
      return { ...appearance, surfaceStyle: initialSurfaceStyle };
    }
    return appearance;
  });
  const windowMaterial = useWindowMaterialMode(appearanceDraft.surfaceStyle);
  const [fontFamilyDraft, setFontFamilyDraft] = React.useState((currentSettings?.fontFamily ?? "").trim());
  const [fontSizeDraft, setFontSizeDraft] = React.useState(currentSettings?.fontSize ?? 15);
  const [themeColorDraft, setThemeColorDraft] = React.useState(resolveThemeColorSelection(currentSettings?.themeColor));
  const [proxyDialogOpen, setProxyDialogOpen] = React.useState(false);
  const [releaseNotesOpen, setReleaseNotesOpen] = React.useState(false);
  const [thirdPartyNoticesOpen, setThirdPartyNoticesOpen] = React.useState(false);
  const [thirdPartyNotices, setThirdPartyNotices] = React.useState("");
  const [thirdPartyNoticesLoadFailed, setThirdPartyNoticesLoadFailed] = React.useState(false);
  const [dataManagementOpen, setDataManagementOpen] = React.useState(false);
  const [browserProfilesOpen, setBrowserProfilesOpen] = React.useState(false);
  const dataManagementSummary = useDataManagementSnapshot(
    activeTab === "general" || dataManagementOpen,
  );
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
    if (!thirdPartyNoticesOpen || thirdPartyNotices || thirdPartyNoticesLoadFailed) {
      return;
    }
    const controller = new AbortController();
    void fetch(THIRD_PARTY_NOTICES_SRC, { signal: controller.signal })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`notice request failed: ${response.status}`);
        }
        return response.text();
      })
      .then((content) => setThirdPartyNotices(content))
      .catch((error) => {
        if (!(error instanceof DOMException && error.name === "AbortError")) {
          setThirdPartyNoticesLoadFailed(true);
        }
      });
    return () => controller.abort();
  }, [thirdPartyNotices, thirdPartyNoticesLoadFailed, thirdPartyNoticesOpen]);

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
  const latestUpdateBadgeTone: DreamStatusTone = (() => {
    if (showLatestAppUpdate) {
      return "accent";
    }
    if (isUpdateError) {
      return "danger";
    }
    return "success";
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

    const nextColor = resolveThemeColorSelection(themeColorDraft);
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
    { id: "general", label: text.settings.tabs.general, icon: <Cog className="app-settings-tab-symbol" /> },
    { id: "appearance", label: text.settings.tabs.appearance, icon: <Palette className="app-settings-tab-symbol" /> },
    { id: "player", label: text.settings.tabs.player, icon: <Headphones className="app-settings-tab-symbol" /> },
    { id: "download", label: text.settings.tabs.download, icon: <Download className="app-settings-tab-symbol" /> },
    { id: "network", label: text.settings.tabs.network, icon: <Network className="app-settings-tab-symbol" /> },
    { id: "ai", label: text.settings.tabs.ai, icon: <Sparkles className="app-settings-tab-symbol" /> },
    { id: "about", label: text.settings.tabs.about, icon: <Info className="app-settings-tab-symbol" /> },
  ];
  const visibleTabs = tabs;
  const activeTabLabel =
    visibleTabs.find((tab) => tab.id === activeTab)?.label ??
    text.settings.tabs.general;
  const settingsPageContract = defineWorkspacePageContract({
    presentation: "standalone-window",
    recipe: "settings",
    routeLabel: activeTabLabel,
    topBar: "host-owned",
    heading: "assistive",
    contentLayout: "form",
    footer: "none",
    scroll: "content",
    density: "regular",
    immersion: "standard",
  });
  const fontOptions = fontFamilies;
  const selectedFont = fontFamilyDraft.trim();
  const hasSelectedFontInList = selectedFont.length === 0 || fontOptions.includes(selectedFont);
  const usesSystemAccentColor = themeColorDraft.trim().toLowerCase() === SYSTEM_THEME_COLOR;
  const selectedThemeColorPreview = resolveThemeColorPreview(
    themeColorDraft,
    currentSettings?.systemThemeColor,
  );
  const editableAccentColor = usesSystemAccentColor
    ? selectedThemeColorPreview
    : resolveAccentColor(themeColorDraft);
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
		const result = await refreshSystemProxyInfo.mutateAsync();
		const nextAddress = (result.address ?? "").trim();
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
	}, [hasStatusAddress, refreshSystemProxyInfo, runProxyStatusCheck, statusAddress, statusMode]);

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
              tone={proxyDraft.mode === option.value ? "accent" : "neutral"}
              size="compact"
              className="app-settings-option-button min-w-0"
              onClick={() => handleProxyModeChange(option.value)}
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
                <StatusBadge className="shrink-0" tone="muted">
                  {systemSourceLabel}
                </StatusBadge>
              ) : null}
              <span className="app-settings-path-value min-w-0 flex-1 truncate">
                {statusAddressDisplay}
              </span>
              {hasStatusAddress ? (
                <span className="inline-flex items-center">
                <span className={cn("app-settings-status-dot h-2 w-2", isChecking ? "app-motion-pulse" : "")} data-status={statusDotValue} aria-hidden="true" />
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
                  {isStatusRefreshing ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <RefreshCcw className="h-4 w-4" />}
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
      <DialogContent className="app-settings-proxy-dialog grid max-w-none gap-3 overflow-hidden">
        <DialogHeader className="min-w-0">
          <DialogTitle className="app-settings-dialog-title overflow-hidden break-words pr-6">
            {text.settings.proxyDialogTitle}
          </DialogTitle>
          <DialogDescription className="app-settings-dialog-description overflow-hidden break-words">
            {text.settings.proxyDialogHint}
          </DialogDescription>
        </DialogHeader>
        <DialogScrollArea className="min-h-0">
          <div className="grid grid-cols-2 gap-x-3 gap-y-2">
            <div className="flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.scheme}</span>
              <Select value={proxyDraft.scheme} onChange={(event) => handleProxyFieldChange("scheme", event.target.value)} className="w-full">
                <option value="http">HTTP</option>
                <option value="https">HTTPS</option>
                <option value="socks5">SOCKS5</option>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.timeout}</span>
              <Input
                type="number"
                inputMode="numeric"
                value={proxyDraft.timeoutSeconds || ""}
                onChange={(event) => handleProxyFieldChange("timeoutSeconds", event.target.value)}
                placeholder="30"
                className="app-settings-field-input"
              />
            </div>
            <div className="flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.host}</span>
              <Input value={proxyDraft.host} onChange={(event) => handleProxyFieldChange("host", event.target.value)} placeholder="127.0.0.1" className="app-settings-field-input" />
            </div>
            <div className="flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.port}</span>
              <Input
                type="number"
                inputMode="numeric"
                value={proxyDraft.port || ""}
                onChange={(event) => handleProxyFieldChange("port", event.target.value)}
                placeholder="8080"
                className="app-settings-field-input"
              />
            </div>
            <div className="flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.username}</span>
              <Input value={proxyDraft.username} onChange={(event) => handleProxyFieldChange("username", event.target.value)} className="app-settings-field-input" />
            </div>
            <div className="flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.password}</span>
              <Input type="password" value={proxyDraft.password} onChange={(event) => handleProxyFieldChange("password", event.target.value)} className="app-settings-field-input" />
            </div>
            <div className="col-span-2 flex flex-col gap-1">
              <span className="app-settings-field-label">{text.settings.noProxyList}</span>
              <Input value={proxyNoProxyText} onChange={(event) => setProxyNoProxyText(event.target.value)} className="app-settings-field-input" />
            </div>
          </div>
          <div className="flex flex-col gap-2 pt-2">
            {proxyTestFeedback ? (
              <div className="app-dream-status-message app-settings-proxy-feedback overflow-hidden break-words px-3 py-2" data-intent={!proxyDraft.testSuccess ? "danger" : "success"}>
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
                {testProxy.isPending || updateSettings.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : null}
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
    <WorkspacePage
      contract={settingsPageContract}
      className="app-dream-window app-settings-window h-screen overflow-hidden"
      data-surface-style={appearanceDraft.surfaceStyle}
      data-window-material={windowMaterial}
    >
      <header
        aria-label={text.settings.title}
        className="app-dream-header app-settings-host-header"
      >
        <div
          className="app-settings-host-drag-grid wails-drag grid items-center px-4"
          data-platform={isWindows ? "windows" : "default"}
        >
          <div className="justify-self-start">
            {isMac ? <div className="app-settings-traffic-light-gap h-4" /> : null}
          </div>

          <div aria-hidden="true" />

          <div
            className={cn(
              "justify-self-end",
              isWindows && "-mr-4 self-start",
            )}
          >
            {isWindows ? (
              <WindowControls platform="windows" owner="settings" />
            ) : null}
          </div>
        </div>

        <nav
          aria-label={text.settings.title}
          className="app-dream-tabs-bar -mt-1 flex flex-nowrap items-center justify-center px-4 pt-0"
        >
          {visibleTabs.map((tab) => (
            <TabButton key={tab.id} id={tab.id} label={tab.label} icon={tab.icon} active={activeTab === tab.id} onClick={setActiveTab} />
          ))}
        </nav>
      </header>

      <WorkspacePageContent className="app-dream-content app-settings-page-content">
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

              <SettingsCompactListCard>
                <SettingsCompactRow label={t("dataManagement.settingsRow")}>
                  <div className="flex items-center gap-3">
                    <span className="app-settings-data-total">
                      {dataManagementSummary.isLoading
                        ? "…"
                        : formatBytes(dataManagementSummary.data?.totalBytes ?? 0)}
                    </span>
                    <Button
                      type="button"
                      variant="outline"
                      size="compact"
                      onClick={() => setDataManagementOpen(true)}
                    >
                      <Database className="h-4 w-4" />
                      {t("dataManagement.manage")}
                      <ChevronRight className="h-4 w-4" />
                    </Button>
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.browserData}>
                  <Button
                    type="button"
                    variant="outline"
                    size="compact"
                    onClick={() => setBrowserProfilesOpen(true)}
                  >
                    {t("dataManagement.manageProfiles")}
                    <ChevronRight className="h-4 w-4" />
                  </Button>
                </SettingsCompactRow>
              </SettingsCompactListCard>
            </>
          ) : null}

          {activeTab === "network" ? (
            <>
              {proxySettingsCard}
              <LibraryAccessSettingsCard language={currentSettings?.language} />
            </>
          ) : null}

          {activeTab === "appearance" ? (
            <div className="space-y-4">
              <SettingsCompactListCard contentClassName="app-settings-theme-pack-card-content">
                <TooltipProvider delayDuration={0}>
                  <div className="grid grid-cols-3 gap-2">
                    {XIA_THEME_PACKS.map((pack) => {
                      const active = appearanceDraft.themePackId === pack.id;
                      return (
                        <Tooltip key={pack.id}>
                          <TooltipTrigger asChild>
                            <Button
                              type="button"
                              variant="outline"
                              tone={active ? "accent" : "neutral"}
                              onClick={() => void saveAppearancePatch({ themePackId: pack.id })}
                              className="app-settings-theme-pack-button app-motion-surface flex min-w-0 items-center overflow-hidden"
                              data-active={active ? "true" : undefined}
                            >
                              <span
                                className="app-settings-theme-preview grid h-6 w-11 shrink-0 overflow-hidden"
                                aria-hidden="true"
                                data-theme-pack-preview={pack.id}
                              >
                                <span data-preview-role="shell" />
                                <span data-preview-role="sidebar" />
                                <span data-preview-role="accent" />
                              </span>
                              <span className="app-settings-theme-pack-label block min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">
                                {text.themePacks[pack.id].label}
                              </span>
                            </Button>
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
                        tone={(currentSettings?.appearance ?? "auto") === item.value ? "accent" : "neutral"}
                        size="compact"
                        className="app-settings-option-button min-w-0"
                        onClick={() => void saveSettingsPatch({ appearance: item.value as "auto" | "light" | "dark" })}
                      >
                        <span className="shrink-0">{item.icon}</span>
                        <span className="min-w-0 truncate">{item.label}</span>
                      </Button>
                    ))}
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.surfaceStyle}>
                  <div className="grid min-w-0 max-w-full grid-cols-2 gap-2">
                    {([
                      { value: "glass", label: text.settings.surfaceStyleOptions.glass },
                      { value: "contrast", label: text.settings.surfaceStyleOptions.contrast },
                    ] as const satisfies Array<{ value: XiaSurfaceStyle; label: string }>).map((option) => (
                      <Button
                        key={option.value}
                        type="button"
                        variant="outline"
                        tone={appearanceDraft.surfaceStyle === option.value ? "accent" : "neutral"}
                        size="compact"
                        className="app-settings-option-button min-w-0"
                        onClick={() => void saveAppearancePatch({ surfaceStyle: option.value })}
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
                        tone={appearanceDraft.accentMode === option.value ? "accent" : "neutral"}
                        size="compact"
                        className="app-settings-option-button min-w-0"
                        onClick={() => void saveAccentMode(option.value)}
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
                                className="app-settings-swatch flex h-4 w-4 items-center justify-center"
                                data-active={usesSystemAccentColor ? "true" : undefined}
                                style={
                                  usesSystemAccentColor && selectedThemeColorPreview
                                    ? {
                                        "--app-settings-swatch-active-color":
                                          selectedThemeColorPreview,
                                      } as React.CSSProperties
                                    : undefined
                                }
                                aria-label={text.common.followSystem}
                              >
                                <span
                                  className="app-settings-swatch-color h-full w-full"
                                  data-theme-pack-preview={appearanceDraft.themePackId}
                                  style={
                                    selectedThemeColorPreview
                                      ? {
                                          "--app-settings-swatch-color":
                                            selectedThemeColorPreview,
                                        } as React.CSSProperties
                                      : undefined
                                  }
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
                                    className="app-settings-swatch flex h-4 w-4 items-center justify-center"
                                    data-active={active ? "true" : undefined}
                                    style={
                                      active
                                        ? {
                                            "--app-settings-swatch-active-color":
                                              color.value,
                                          } as React.CSSProperties
                                        : undefined
                                    }
                                    aria-label={text.common.colorOptions[color.id]}
                                  >
                                    <span
                                      className="app-settings-swatch-color h-full w-full"
                                      style={
                                        {
                                          "--app-settings-swatch-color": color.value,
                                        } as React.CSSProperties
                                      }
                                    />
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
                                className="app-settings-swatch h-4 w-4 p-0"
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
                      <option
                        className="app-settings-font-preview-option"
                        key={selectedFont}
                        value={selectedFont}
                        style={
                          {
                            "--app-settings-font-preview-family":
                              previewFontStack(selectedFont),
                          } as React.CSSProperties
                        }
                      >
                        {selectedFont} {text.common.current}
                      </option>
                    ) : null}
                    {fontOptions.map((family) => (
                      <option
                        className="app-settings-font-preview-option"
                        key={family}
                        value={family}
                        style={
                          {
                            "--app-settings-font-preview-family":
                              previewFontStack(family),
                          } as React.CSSProperties
                        }
                      >
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
                      className="app-settings-number-input w-full"
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
                <SettingsCompactRow label={text.settings.romanizedLyrics}>
                  <InlineSwitch
                    checked={currentSettings?.romanizedLyrics !== false}
                    onChange={(checked) => void saveSettingsPatch({ romanizedLyrics: checked })}
                    ariaLabel={text.settings.romanizedLyrics}
                  />
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.settings.pinyinLyrics}>
                  <InlineSwitch
                    checked={currentSettings?.pinyinLyrics !== false}
                    onChange={(checked) => void saveSettingsPatch({ pinyinLyrics: checked })}
                    ariaLabel={text.settings.pinyinLyrics}
                  />
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <EqualizerSection isMac={isMac} isWindows={isWindows} text={text} />
            </div>
          ) : null}

          {activeTab === "download" ? (
            <>
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
                    <span className="app-settings-path-value min-w-0 flex-1 truncate">
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
                        <Loader2 className="h-4 w-4 app-motion-spin" />
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
                              className="app-settings-help-button inline-flex h-5 w-5 shrink-0 items-center justify-center"
                              aria-label={text.settings.ytdlpConcurrentDownloadsHelp}
                            >
                              <CircleHelp className="h-3.5 w-3.5" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent
                            side="top"
                            multiline
                            className="app-settings-tooltip-long"
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
                              className="app-settings-help-button inline-flex h-5 w-5 shrink-0 items-center justify-center"
                              aria-label={text.settings.ytdlpConcurrentFragmentsHelp}
                            >
                              <CircleHelp className="h-3.5 w-3.5" />
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="top" multiline className="app-settings-tooltip-long">
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

          {activeTab === "ai" ? (
            <SettingsCompactListCard>
              <SettingsCompactRow label={text.settings.tabs.ai}>
                <StatusBadge marker tone="warning">
                  {text.settings.aiUnderConstruction}
                </StatusBadge>
              </SettingsCompactRow>
            </SettingsCompactListCard>
          ) : null}

          {activeTab === "about" ? (
            <div className="space-y-6">
              <div className="app-settings-about-header flex flex-col items-center gap-2">
                <img src="/appicon.png" alt={text.appName} className="app-settings-app-icon h-16 w-16" />
                <div className="app-settings-about-title">{text.appName}</div>
              </div>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.about.currentVersion}>
                  <span className="app-settings-about-value">{aboutVersion}</span>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.latestVersion}>
                  <div className="flex min-w-0 items-center justify-end">
                    <StatusBadge
                      className="app-settings-latest-badge"
                      icon={React.createElement(latestUpdateBadgeIcon)}
                      tone={latestUpdateBadgeTone}
                    >
                      {latestUpdateLabel}
                    </StatusBadge>
                  </div>
                </SettingsCompactRow>

                <SettingsCompactSeparator />

                <SettingsCompactRow label={text.about.viewChangelog}>
                  {hasReleaseNotes ? (
                    <Button type="button" variant="outline" size="compact" onClick={() => setReleaseNotesOpen(true)}>
                      {text.about.viewReleaseNotes}
                    </Button>
                  ) : (
                    <span className="app-settings-about-empty">{text.about.noReleaseNotes}</span>
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
                              {isCheckingUpdate ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <RefreshCw className="h-4 w-4" />}
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
                                <Loader2 className="h-4 w-4 app-motion-spin" />
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
                              {restartToApply.isPending ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <RefreshCw className="h-4 w-4" />}
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
                        <div className="app-settings-update-progress max-w-full space-y-1.5">
                          <div className="app-dream-progress-track h-2 w-full">
                            <div
                              className="app-dream-progress-value"
                              style={{ width: `${Math.min(Math.max(updateInfo.progress, 0), 100)}%` }}
                            />
                          </div>
                          <div className="app-settings-update-meta flex items-center justify-between">
                            <span>{updateInfo.status === "installing" ? text.about.installing : text.about.downloading}</span>
                            <span>{Math.round(updateInfo.progress)}%</span>
                          </div>
                        </div>
                      ) : (
                        <span className="app-settings-update-error whitespace-pre-wrap break-words">
                          {updateErrorMessage}
                        </span>
                      )}
                    </SettingsCompactRow>
                  </>
                ) : null}
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.about.thirdPartyLicenses}>
                  <Button
                    type="button"
                    variant="outline"
                    size="compact"
                    onClick={() => {
                      setThirdPartyNoticesLoadFailed(false);
                      setThirdPartyNoticesOpen(true);
                    }}
                  >
                    {text.about.viewReleaseNotes}
                  </Button>
                </SettingsCompactRow>
              </SettingsCompactListCard>

              <SettingsCompactListCard>
                <SettingsCompactRow label={text.about.craftedBy}>
                  <span className="app-settings-about-value">{ABOUT_AUTHOR_NAME}</span>
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
                            onClick={openAboutContactEmail}
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
                            onClick={() => void openExternalURL("https://x.com/ArnoldHaoCA")}
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
                            onClick={() => void openExternalURL("https://xiadown.app/")}
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
                            onClick={() => void openExternalURL("https://github.com/arnoldhao/xiadown")}
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
                          onClick={() => void openExternalURL("https://github.com/arnoldhao/xiadown/issues")}
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
                        onClick={() => void openExternalURL(app.url)}
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
      </WorkspacePageContent>

      {proxyDialog}

      <Dialog open={releaseNotesOpen} onOpenChange={setReleaseNotesOpen}>
        <DialogContent className="app-settings-release-notes-dialog grid max-w-none gap-3 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="app-settings-dialog-title overflow-hidden break-words pr-6">
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
      <Dialog open={thirdPartyNoticesOpen} onOpenChange={setThirdPartyNoticesOpen}>
        <DialogContent className="app-settings-notices-dialog grid max-w-none gap-3 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="app-settings-dialog-title overflow-hidden break-words pr-6" data-clamp="false">
              {text.about.thirdPartyLicenses}
            </DialogTitle>
          </DialogHeader>
          <DialogScrollArea className="min-h-0">
            {thirdPartyNotices ? (
              <pre className="app-settings-license-text whitespace-pre-wrap break-words">
                {thirdPartyNotices}
              </pre>
            ) : thirdPartyNoticesLoadFailed ? (
              <p className="app-settings-update-error">{text.about.thirdPartyLicensesLoadFailed}</p>
            ) : (
              <div className="flex min-h-24 items-center justify-center" aria-busy="true">
                <Loader2 className="app-settings-muted-icon h-5 w-5 app-motion-spin" />
              </div>
            )}
          </DialogScrollArea>
          <DialogFooter className="shrink-0 items-end sm:items-center">
            <Button type="button" variant="ghost" size="compact" onClick={() => setThirdPartyNoticesOpen(false)}>
              {text.about.releaseNotesClose}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <DataManagementSheet
        open={dataManagementOpen}
        onOpenChange={setDataManagementOpen}
      />
      <BrowserProfilesSheet
        open={browserProfilesOpen}
        onOpenChange={setBrowserProfilesOpen}
      />
    </WorkspacePage>
  );
}
