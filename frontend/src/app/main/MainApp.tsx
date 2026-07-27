import { Events,System } from "@wailsio/runtime";
import {
ArrowUpCircle,
Activity,
Bell,
ChartNoAxesColumnIncreasing,
Check,
ChevronsUpDown,
Clock3,
Clapperboard,
Compass,
Disc3,
Download,
Eye,
FileCog,
FileText,
Globe2,
HardDrive,
History,
House,
Link2,
LoaderCircle,
ListMusic,
ListVideo,
Music2,
LibraryBig,
Video,
BookOpen,
Images,
PackageOpen,
Podcast,
Plus,
Radar,
Radio,
RefreshCcw,
Rss,
Search,
Settings2,
Shapes,
Smartphone,
Sparkles,
Square,
Star,
ThumbsUp,
Trash2,
PawPrint,
UsersRound,
Wrench,
X,
Youtube
} from "lucide-react";
import * as React from "react";

import { useWindowMaterialMode } from "@/shared/styles/window-material";

import {
type ListenExternalCommand,
type ListenNowPlayingStatus,
type ListenPlaybackSource
} from "@/app/main/Listen";
import { forceRefreshListenOnline } from "@/app/main/listen/api";
import { adaptCatalogItems } from "@/app/library/catalog-adapter";
import {
  adaptAvailableLegacyImageFiles,
  adaptLegacyLibraryFiles,
  adaptLegacyLibraryWorkspace,
} from "@/app/library/legacy-adapter";
import { LibraryDeletedCompanion } from "@/app/library/LibraryDeletedCompanion";
import {
  LibraryPreviewCompanion,
  LibraryPreviewCompanionFooter,
  type LibraryPreviewTab,
} from "@/app/library/LibraryPreviewCompanion";
import type { LibrarySortMode } from "@/app/library/LibraryWorkspacePage";
import { libraryCatalogCategory } from "@/app/library/library-catalog-query";
import type {
  LibraryOtherGroup,
  LibraryWorkspaceItem,
  LibraryWorkspaceRoute,
} from "@/app/library/types";
import {
setPendingSettingsTab,
type XiaSettingsTabId,
} from "@/app/settings/sectionStorage";
import { LibraryPairingSheet } from "@/app/settings/LibraryPairingSheet";
import type { PetsGalleryNavigation } from "@/app/pets-gallery";
import {
  SniffWorkspaceKindSelect,
  SniffWorkspaceResourceSelect,
  SniffWorkspaceSearchField,
  SniffWorkspaceSourceSelect,
} from "@/app/sniff-desk/workspace-filters";
import {
  forceRefreshYouTubeWorkspace,
  type YouTubeWorkspaceExternalCommand,
  type YouTubeWorkspacePlaybackState,
} from "@/app/youtube";
import {
  useRSSDeleteSubscription,
  useRSSCollectionUnreadCounts,
  useRSSCategories,
  useRSSCollections,
  useRSSMarkAllRead,
  useRSSSubscriptions,
} from "@/app/rss/api";
import { RSSSubscriptionDialog } from "@/app/rss/RSSSubscriptionDialog";
import { createRSSSubscriptionRouteId } from "@/app/rss/rss-routes";
import {
  dismissRSSPreviewNotice,
  readRSSPreviewNoticeDismissed,
} from "@/app/rss/preview-notice";
import type { RSSEntry, RSSSubscription } from "@/app/rss/types";
import type { RSSVideoBatchDownloadTarget } from "@/app/rss/video-platform";
import { AccountDock, AccountDockProfile } from "@/app/workspace/AccountDock";
import { ActivityDock } from "@/app/workspace/ActivityDock";
import { AppShell } from "@/app/workspace/AppShell";
import { CompanionPanel } from "@/app/workspace/CompanionPanel";
import { LibraryWorkspaceSidebar } from "@/app/workspace/LibraryWorkspaceSidebar";
import { MusicWorkspaceSidebar } from "@/app/workspace/MusicWorkspaceSidebar";
import { PrimaryPane } from "@/app/workspace/PrimaryPane";
import { RSSWorkspaceSidebar } from "@/app/workspace/RSSWorkspaceSidebar";
import { SniffWorkspaceSidebar } from "@/app/workspace/SniffWorkspaceSidebar";
import { WorkspaceStage } from "@/app/workspace/WorkspaceStage";
import { YouTubeWorkspaceSidebar } from "@/app/workspace/YouTubeWorkspaceSidebar";
import {
  createCompanionSelectionDestination,
  defineCompanionSelectionContract,
  resolveActiveCompanionSelectionFromMap,
  resolveActiveCompanionSelectionId,
} from "@/app/workspace/companion-selection";
import { PRIMARY_PANE_DEFAULT_MIN_WIDTH } from "@/app/workspace/layout";
import { resolveWorkspaceSwitchStations } from "@/app/workspace/station-navigation";
import { useAppWorkspaceStore } from "@/app/workspace/store";
import {
  APP_WORKSPACE_IDS,
  type AppWorkspaceId,
} from "@/app/workspace/types";
import {
resolveActivePet,
useRunningPetAnimation,
} from "@/features/pets/shared";
import {
getXiaText,
resolveLibraryCoverURL
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { LibraryDTO,OperationListItemDTO } from "@/shared/contracts/library";
import { useI18n } from "@/shared/i18n";
import { messageBus, publishOSNotification } from "@/shared/message";
import { trackStationWhenMainWindowActive } from "@/shared/telemetry/station-events";
import {
useDependencies,
useDependencyUpdates
} from "@/shared/query/dependencies";
import {
useListLibraries,
useListOperations,
useEndedOperations,
useCancelResourceSniff,
useClearResourceSniffResources,
useStopCDPBrowserRuntime
} from "@/shared/query/library";
import { sortAndDedupeOperations } from "@/shared/query/complete-operations";
import { useCatalogItems, useCompleteCatalogItems } from "@/shared/query/catalog";
import {
  useLibraryAccessConfig,
  useLibraryAccessStatus,
  useUpdateLibraryAccessConfig,
} from "@/shared/query/library-access";
import { useResourceSniffStatus } from "@/shared/query/activity";
import { projectOperationActivitySnapshot } from "@/shared/activity/operations";
import { projectSniffStatusSnapshot } from "@/shared/activity/sniff";
import { useHttpBaseURL } from "@/shared/query/runtime";
import { useAppSessions } from "@/shared/query/appSessions";
import {
setWelcomeWindowChromeHidden,
useShowSettingsWindow
} from "@/shared/query/settings";
import { usePets } from "@/shared/query/pets";
import { openExternalURL,useCurrentUserProfile } from "@/shared/query/system";
import {
useRestartToApply,
useUpdateState
} from "@/shared/query/update";
import { useSettingsStore } from "@/shared/store/settings";
import {
displayUpdateVersion,
hasPreparedUpdate,
hasRemoteUpdate,
useUpdateStore,
} from "@/shared/store/update";
import {
DropdownMenu,
DropdownMenuCheckboxItem,
DropdownMenuContent,
DropdownMenuItem,
DropdownMenuSeparator,
DropdownMenuTrigger
} from "@/shared/ui/dropdown-menu";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import {
Dialog,
DialogClose,
DialogContent,
DialogDescription,
DialogFooter,
DialogHeader,
DialogRow,
DialogTitle,
} from "@/shared/ui/dialog";
import { DreamInlineSwitchVisual } from "@/shared/ui/dream-inline-switch";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { StatusBadge } from "@/shared/ui/status-badge";
import {
resolveUserDisplayName,
UserAvatar,
} from "@/shared/ui/user-avatar";
import {
buildAssetPreviewURL
} from "@/shared/utils/resourceHelpers";
import {
readXiaAppearance,
resolveThemePack,
} from "@/shared/styles/xiadown-theme";
import { WindowControls } from "@/components/layout/WindowControls";

import { WhatsNewFeatureDialog } from "@/app/main/dialogs";
import {
LISTEN_NOW_PLAYING_EVENT,
LISTEN_NOW_PLAYING_STORAGE_KEY,
LISTEN_TRAY_COMMAND_EVENT,
} from "@/app/main/listen/catalog";
import { formatVersionBadge,normalizeDependencyVersion,resolveCompletedStatusLabel } from "@/app/main/helpers";
import { CORE_DEPENDENCIES,SETUP_STORAGE_KEY,SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,SIDEBAR_DROPDOWN_ITEM_CLASS_NAME,useSetupState } from "@/app/main/main-constants";
import { resolveMusicWorkspaceRoute,resolveMusicWorkspaceScopeRoute } from "@/app/main/listen/workspace-routes";
import { resolveSidebarSurface } from "@/app/main/sidebar";
import {
  OperationsCompanionView,
  PlayerCompanionFooter,
  PlayerCompanionView,
  SniffCompanionFooter,
  SniffCompanionView,
  SniffWorkspaceSessionActivity,
  WideOperationActivity,
  WidePlaybackActivity,
  WideSniffActivity,
  type WorkspaceActivityMenuAction,
  type WorkspaceActivityLabels,
} from "@/app/main/WorkspaceActivitySurfaces";
import { useWorkspaceWindowFit } from "@/app/main/useWorkspaceWindowFit";
import { workspaceCompanionAffectsLayout } from "@/app/main/workspace-window-fit";
import {
  projectCoordinatorPlaybackStatus,
  recoverYouTubeWorkspacePlayback,
  resolveGlobalPlaybackCommandRoute,
  resolveListenFallbackPlaybackCommand,
  shouldPresentMusicWorkspaceTransport,
  shouldShowWorkspacePlaybackActivity,
  type GlobalPlaybackCommand,
} from "@/app/main/workspace-playback";
import { MusicWorkspaceTransportBar } from "@/app/main/WorkspaceTransportBar";
import {
  resolveLibraryAccessStatusTone,
  safeLibraryAccessBackendErrorMessage,
} from "@/app/settings/library-access-ui";
import { usePlaybackCoordinator } from "@/shared/playback";
import { STARTUP_SURFACE_READY_EVENT } from "@/startup-presentation";

const loadRunningPage = () => import("@/app/main/RunningPage");
const loadLibraryWorkspacePage = () => import("@/app/library/LibraryWorkspacePage");
const loadAppSessionsSection = () => import("@/features/settings/app-sessions");
const loadPetsGalleryPage = () => import("@/app/pets-gallery/PetsGalleryPage");
const loadSniffDeskPage = () => import("@/app/sniff-desk/SniffDeskPage");
const loadRSSWorkspacePage = () => import("@/app/rss/RSSWorkspacePage");
const loadYouTubeWorkspacePage = () => import("@/app/youtube/YouTubeWorkspacePage");
const loadListenPage = () => import("@/app/main/Listen");

const RunningPage = React.lazy(() =>
  loadRunningPage().then(({ RunningPage: Component }) => ({ default: Component })),
);
const LibraryWorkspacePage = React.lazy(() =>
  loadLibraryWorkspacePage().then(({ LibraryWorkspacePage: Component }) => ({ default: Component })),
);
const AppSessionsSection = React.lazy(() =>
  loadAppSessionsSection().then(({ AppSessionsSection: Component }) => ({ default: Component })),
);
const PetsGalleryPage = React.lazy(() =>
  loadPetsGalleryPage().then(({ PetsGalleryPage: Component }) => ({ default: Component })),
);
const SniffDeskPage = React.lazy(() =>
  loadSniffDeskPage().then(({ SniffDeskPage: Component }) => ({ default: Component })),
);
const RSSWorkspacePage = React.lazy(() =>
  loadRSSWorkspacePage().then(({ RSSWorkspacePage: Component }) => ({ default: Component })),
);
const YouTubeWorkspacePage = React.lazy(() =>
  loadYouTubeWorkspacePage().then(({ YouTubeWorkspacePage: Component }) => ({ default: Component })),
);
const ListenPage = React.lazy(() =>
  loadListenPage().then(({ ListenPage: Component }) => ({ default: Component })),
);
const NewTaskDialog = React.lazy(() =>
  import("@/app/main/NewTaskDialog").then(({ NewTaskDialog: Component }) => ({ default: Component })),
);

export async function preloadMainAppInitialSurface() {
  const state = useAppWorkspaceStore.getState();
  const workspaceId = state.activeWorkspaceId;
  const routeId = state.locations[workspaceId]?.routeId || "all";

  if (workspaceId === APP_WORKSPACE_IDS.music) {
    await loadListenPage();
    return;
  }
  if (workspaceId === APP_WORKSPACE_IDS.sniff) {
    await loadSniffDeskPage();
    return;
  }
  if (workspaceId === APP_WORKSPACE_IDS.rss) {
    await loadRSSWorkspacePage();
    return;
  }
  if (workspaceId === APP_WORKSPACE_IDS.youtube) {
    await loadYouTubeWorkspacePage();
    return;
  }
  if (routeId === "running") {
    await loadRunningPage();
    return;
  }
  if (routeId === "app-sessions") {
    await loadAppSessionsSection();
    return;
  }
  if (routeId === "pet-gallery") {
    await loadPetsGalleryPage();
    return;
  }
  await loadLibraryWorkspacePage();
}

function MainStartupSurfaceReady() {
  React.useLayoutEffect(() => {
    const root = document.documentElement;
    if (root.dataset.startupSurface === "ready") return;
    root.dataset.startupSurface = "ready";
    window.dispatchEvent(new Event(STARTUP_SURFACE_READY_EVENT));
  }, []);
  return null;
}
import type {
  NewTaskDialogDownloadTarget,
  NewTaskDialogMode,
  NewTaskDialogTranscodeSource,
} from "@/app/main/types";
import {
WELCOME_DEBUG_EVENT,
WelcomeScreen,
type WelcomeDebugCommand,
type WelcomeDebugStep,
} from "@/app/main/WelcomeScreen";

const NOTIFIABLE_OPERATION_STATUSES = new Set(["succeeded", "failed"]);
const MAIN_NEW_DOWNLOAD_EVENT = "main:new-download";
const DOCUMENTATION_ORIGIN = "https://xiadown.app";
const LIBRARY_PREVIEW_SELECTION_CONTRACT = defineCompanionSelectionContract({
  destinationId: "library-preview",
  contextKey: "itemId",
});
const DOCUMENTATION_PATH_BY_LANGUAGE: Record<string, string> = {
  "zh-CN": "/docs/",
  "zh-TW": "/zh-tw/docs/",
  en: "/en/docs/",
  "ja-JP": "/ja-jp/docs/",
  "ko-KR": "/ko-kr/docs/",
  "es-419": "/es-419/docs/",
  "pt-BR": "/pt-br/docs/",
  "id-ID": "/id-id/docs/",
  "vi-VN": "/vi-vn/docs/",
};

function normalizeOperationStatus(status?: string) {
  return (status ?? "").trim().toLowerCase();
}

function publishRSSActionFailure(title: string, error: unknown) {
  messageBus.publishToast({
    intent: "danger",
    title,
    description: error instanceof Error ? error.message : String(error ?? ""),
  });
}

function resolveDocumentationLanguage(language?: string | null) {
  const normalized = language?.trim() || "zh-CN";
  return DOCUMENTATION_PATH_BY_LANGUAGE[normalized] ? normalized : "zh-CN";
}

function normalizeDocumentationPagePath(path?: string) {
  const normalized = (path ?? "").trim().replace(/^\/+|\/+$/g, "");
  if (!normalized || normalized === "docs") {
    return "";
  }
  return normalized
    .replace(/^(?:zh-tw|en|ja-jp|ko-kr|es-419|pt-br|id-id|vi-vn)\/docs\/?/i, "")
    .replace(/^docs\/?/i, "")
    .replace(/\/+$/g, "");
}

function buildDocumentationURL(language?: string | null, path?: string) {
  const url = new URL(DOCUMENTATION_ORIGIN);
  const languageRoot = DOCUMENTATION_PATH_BY_LANGUAGE[resolveDocumentationLanguage(language)];
  const pagePath = normalizeDocumentationPagePath(path);
  url.pathname = pagePath ? `${languageRoot}${pagePath}/` : languageRoot;
  return url.toString();
}

function resolveOperationNotificationCoverURL(
  baseURL: string,
  operation: OperationListItemDTO,
  filesById: Map<string, LibraryDTO["files"][number]>,
  librariesById: Map<string, LibraryDTO>,
) {
  const outputCoverURL = (operation.outputFiles ?? [])
    .map((output) => {
      const kind = normalizeOperationStatus(output.kind);
      if (kind !== "thumbnail" && kind !== "image") {
        return "";
      }
      const path = filesById.get(output.fileId)?.storage.localPath?.trim() ?? "";
      return path ? buildAssetPreviewURL(baseURL, path) : "";
    })
    .find(Boolean);
  if (outputCoverURL) {
    return outputCoverURL;
  }

  const operationCoverURL = [...filesById.values()]
    .filter((file) => file.libraryId === operation.libraryId)
    .filter((file) => {
      const kind = normalizeOperationStatus(file.kind);
      return (kind === "thumbnail" || kind === "image") && !file.state.deleted;
    })
    .filter((file) => {
      const operationId = operation.operationId.trim();
      return file.latestOperationId === operationId || file.origin.operationId === operationId;
    })
    .map((file) => {
      const path = file.storage.localPath?.trim() ?? "";
      return path ? buildAssetPreviewURL(baseURL, path) : "";
    })
    .find(Boolean);
  if (operationCoverURL) {
    return operationCoverURL;
  }

  return resolveLibraryCoverURL(baseURL, librariesById.get(operation.libraryId));
}

function resolveOperationNotificationTitle(operation: OperationListItemDTO) {
  return operation.name.trim() || operation.operationId.trim();
}

function libraryCatalogSort(sort: LibrarySortMode): string {
  if (sort === "oldest") return "created_asc";
  if (sort === "name") return "title_asc";
  return "updated_desc";
}

function usesServerLibraryPagination(
  route: LibraryWorkspaceRoute,
  sort: LibrarySortMode,
  query: string,
) {
  return route !== "ended" &&
    route !== "images" &&
    route !== "others" &&
    sort !== "size" &&
    (route !== "search" || query.trim().length > 0);
}

function WorkspaceSessionAvatar(props: { name: string; src: string }) {
  const [failed, setFailed] = React.useState(false);
  const src = props.src.trim();
  React.useEffect(() => setFailed(false), [src]);
  const initials = props.name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0] ?? "")
    .join("")
    .toUpperCase() || "YT";
  return (
    <span
      aria-label={props.name}
      className="app-workspace-session-avatar"
    >
      {src && !failed ? (
        <img
          alt=""
          className="h-full w-full object-cover"
          onError={() => setFailed(true)}
          src={src}
        />
      ) : (
        initials
      )}
    </span>
  );
}

export function MainApp() {
  const { t } = useI18n();
  const windowMaterial = useWindowMaterialMode();
  const settings = useSettingsStore((state) => state.settings);
  const playbackCoordinator = usePlaybackCoordinator();
  const activeWorkspaceId = useAppWorkspaceStore(
    (state) => state.activeWorkspaceId,
  );
  React.useEffect(() => {
    const trackFocusedStation = () => {
      trackStationWhenMainWindowActive(activeWorkspaceId);
    };
    trackFocusedStation();
    window.addEventListener("focus", trackFocusedStation);
    document.addEventListener("visibilitychange", trackFocusedStation);
    return () => {
      window.removeEventListener("focus", trackFocusedStation);
      document.removeEventListener("visibilitychange", trackFocusedStation);
    };
  }, [activeWorkspaceId]);
  const [mountedWorkspaceIds, setMountedWorkspaceIds] = React.useState<Set<AppWorkspaceId>>(
    () => new Set([activeWorkspaceId]),
  );
  React.useEffect(() => {
    setMountedWorkspaceIds((current) => {
      if (current.has(activeWorkspaceId)) return current;
      const next = new Set(current);
      next.add(activeWorkspaceId);
      return next;
    });
  }, [activeWorkspaceId]);
  const shouldMountRSSWorkspace =
    activeWorkspaceId === APP_WORKSPACE_IDS.rss || mountedWorkspaceIds.has(APP_WORKSPACE_IDS.rss);
  const shouldMountYouTubeWorkspace =
    activeWorkspaceId === APP_WORKSPACE_IDS.youtube || mountedWorkspaceIds.has(APP_WORKSPACE_IDS.youtube);
  const shouldMountMusicWorkspace =
    activeWorkspaceId === APP_WORKSPACE_IDS.music || mountedWorkspaceIds.has(APP_WORKSPACE_IDS.music);
  const workspaceLocations = useAppWorkspaceStore((state) => state.locations);
  const libraryWorkspaceRoute = workspaceLocations[APP_WORKSPACE_IDS.library]?.routeId || "all";
  const libraryContentRoute = (["search", "ended", "all", "video", "audio", "books", "images", "others"] as const)
    .includes(libraryWorkspaceRoute as LibraryWorkspaceRoute)
      ? libraryWorkspaceRoute as LibraryWorkspaceRoute
      : "all";
  const [libraryBrowseQuery, setLibraryBrowseQuery] = React.useState("");
  const [libraryBrowseSort, setLibraryBrowseSort] = React.useState<LibrarySortMode>("updated");
  const [libraryOtherGroup, setLibraryOtherGroup] = React.useState<LibraryOtherGroup>("document");
  const [libraryPage, setLibraryPage] = React.useState(1);
  const [libraryPageSize, setLibraryPageSize] = React.useState(48);
  const [newTaskDialogOpen, setNewTaskDialogOpen] = React.useState(false);
  const [newTaskDialogMode, setNewTaskDialogMode] =
    React.useState<NewTaskDialogMode>("download");
  const stations = useAppWorkspaceStore((state) => state.stations);
  const companion = useAppWorkspaceStore((state) => state.companion);
  const activateWorkspace = useAppWorkspaceStore(
    (state) => state.activateWorkspace,
  );
  const navigateWorkspace = useAppWorkspaceStore((state) => state.navigate);
  const openCompanion = useAppWorkspaceStore((state) => state.openCompanion);
  const closeCompanion = useAppWorkspaceStore((state) => state.closeCompanion);
  const toggleCompanion = useAppWorkspaceStore(
    (state) => state.toggleCompanion,
  );
  const profile = useCurrentUserProfile().data;
  const appSessions = useAppSessions();
  const showSettingsWindow = useShowSettingsWindow();
  const { data: httpBaseURL = "" } = useHttpBaseURL();
  const petsQuery = usePets();
  const toolsQuery = useDependencies({ refetchInterval: 3_000 });
  const dependencyUpdatesQuery = useDependencyUpdates();
  const updateStateQuery = useUpdateState();
  const updateInfo = useUpdateStore((state) => state.info);
  const setUpdateInfo = useUpdateStore((state) => state.setInfo);
  const restartToApply = useRestartToApply();
  const librariesQuery = useListLibraries();
  const serverLibraryPagination = usesServerLibraryPagination(
    libraryContentRoute,
    libraryBrowseSort,
    libraryBrowseQuery,
  );
  const completeCatalogNeeded =
    activeWorkspaceId === APP_WORKSPACE_IDS.library &&
    !serverLibraryPagination &&
    libraryContentRoute !== "ended" &&
    !(libraryContentRoute === "search" && libraryBrowseQuery.trim().length === 0);
  const catalogItemsQuery = useCompleteCatalogItems(
    { status: "all", excludeTrashed: true },
    completeCatalogNeeded,
  );
  const transcodeCatalogItemsQuery = useCompleteCatalogItems(
    { category: "video", status: "active" },
    newTaskDialogOpen && newTaskDialogMode === "transcode",
  );
  const libraryCatalogPageQuery = useCatalogItems({
    category: libraryCatalogCategory(libraryContentRoute),
    status: "all",
    excludeTrashed: true,
    query: libraryBrowseQuery.trim(),
    sort: libraryCatalogSort(libraryBrowseSort),
    limit: libraryPageSize,
    offset: (libraryPage - 1) * libraryPageSize,
  }, activeWorkspaceId === APP_WORKSPACE_IDS.library && serverLibraryPagination);
  const rssSubscriptionsQuery = useRSSSubscriptions(
    activeWorkspaceId === APP_WORKSPACE_IDS.rss,
  );
  const rssCollectionUnreadCounts = useRSSCollectionUnreadCounts(
    activeWorkspaceId === APP_WORKSPACE_IDS.rss,
  );
  const rssCategoriesQuery = useRSSCategories(
    activeWorkspaceId === APP_WORKSPACE_IDS.rss,
  );
  const rssCollectionsQuery = useRSSCollections(
    activeWorkspaceId === APP_WORKSPACE_IDS.rss,
  );
  const rssMarkAllRead = useRSSMarkAllRead();
  const rssDeleteSubscription = useRSSDeleteSubscription();
  const [rssEditingSubscription, setRSSEditingSubscription] =
    React.useState<{
      subscription: RSSSubscription;
      returnFocusTarget: HTMLElement | null;
    } | null>(null);
  const [rssPendingUnsubscribe, setRSSPendingUnsubscribe] =
    React.useState<RSSSubscription | null>(null);
  const [rssPreviewNoticeOpen, setRSSPreviewNoticeOpen] =
    React.useState(false);
  const [rssPreviewNoticeDontShowAgain, setRSSPreviewNoticeDontShowAgain] =
    React.useState(false);
  const previousWorkspaceIdRef = React.useRef<AppWorkspaceId | null>(null);
  const rssUnsubscribeReturnFocusTargetRef =
    React.useRef<HTMLElement | null>(null);
  const libraryAccessConfigQuery = useLibraryAccessConfig();
  const libraryAccessStatusQuery = useLibraryAccessStatus(
    libraryAccessConfigQuery.data?.remoteEnabled === true,
  );
  const updateLibraryAccess = useUpdateLibraryAccessConfig();
  const runningQuery = useListOperations({
    status: ["queued", "running"],
    limit: 200,
  });
  const terminalQuery = useListOperations({
    status: ["succeeded", "failed", "canceled"],
    limit: 300,
  });
  const endedOperationsQuery = useEndedOperations({
    enabled:
      activeWorkspaceId === APP_WORKSPACE_IDS.library &&
      libraryWorkspaceRoute === "ended",
  });
  const sniffStatusQuery = useResourceSniffStatus(true);
  const cancelResourceSniff = useCancelResourceSniff();
  const clearResourceSniffResources = useClearResourceSniffResources();
  const stopCDPBrowserRuntime = useStopCDPBrowserRuntime();
  const [setupState, setSetupState] = useSetupState();
  const [debugWelcomeOpen, setDebugWelcomeOpen] = React.useState(false);
  const [petsGalleryNavigation, setPetsGalleryNavigation] =
    React.useState<PetsGalleryNavigation | null>(null);
  const [libraryPreviewLoadingSnapshot, setLibraryPreviewLoadingSnapshot] =
    React.useState<LibraryWorkspaceItem | null>(null);
  const [libraryPreviewTab, setLibraryPreviewTab] =
    React.useState<LibraryPreviewTab>("preview");
  const [workspaceAccountMenuOpen, setWorkspaceAccountMenuOpen] = React.useState(false);
  const [mobilePairingSheetOpen, setMobilePairingSheetOpen] = React.useState(false);
  const [prefilledDownloadURL, setPrefilledDownloadURL] = React.useState("");
  const [prefilledDownloadOrigin, setPrefilledDownloadOrigin] = React.useState<{
    source: string;
    caller: string;
  } | null>(null);
  const [prefilledDownloadTargets, setPrefilledDownloadTargets] = React.useState<
    NewTaskDialogDownloadTarget[]
  >([]);
  const [prefilledTranscodeSource, setPrefilledTranscodeSource] =
    React.useState<NewTaskDialogTranscodeSource | null>(null);
  const [listenNowPlaying, setListenNowPlaying] =
    React.useState<ListenNowPlayingStatus | null>(null);
  const [listenControlCommand, setListenControlCommand] =
    React.useState<ListenExternalCommand | null>(null);
  const [listenPlayerPortalTarget, setListenPlayerPortalTarget] =
    React.useState<HTMLDivElement | null>(null);
  const [youtubeUpNextPortalTarget, setYouTubeUpNextPortalTarget] =
    React.useState<HTMLDivElement | null>(null);
  const [youtubeWatchOpen, setYouTubeWatchOpen] = React.useState(false);
  const [youtubeWatchSurfaceVisible, setYouTubeWatchSurfaceVisible] =
    React.useState(false);
  const [youtubeWatchRevealRequest, setYouTubeWatchRevealRequest] =
    React.useState(0);
  const [youtubePlayback, setYouTubePlayback] =
    React.useState<YouTubeWorkspacePlaybackState | null>(null);
  const [youtubeControlCommand, setYouTubeControlCommand] =
    React.useState<YouTubeWorkspaceExternalCommand | null>(null);
  const [musicForceRefreshToken, setMusicForceRefreshToken] =
    React.useState(0);
  const [youtubeForceRefreshToken, setYouTubeForceRefreshToken] =
    React.useState(0);
  const [workspaceRouteContextMenu, setWorkspaceRouteContextMenu] =
    React.useState<{
      workspaceId: typeof APP_WORKSPACE_IDS.music | typeof APP_WORKSPACE_IDS.youtube;
      routeId: string;
      x: number;
      y: number;
    } | null>(null);
  const workspaceRouteContextReturnFocusRef =
    React.useRef<HTMLButtonElement | null>(null);
  const coordinatorPlaybackSession = playbackCoordinator.snapshot.active;
  const recoveredYouTubePlayback = React.useMemo(
    () => recoverYouTubeWorkspacePlayback(coordinatorPlaybackSession),
    [coordinatorPlaybackSession],
  );
  const youtubePlaybackForPage =
    recoveredYouTubePlayback &&
    youtubePlayback?.descriptor.sessionId !==
      recoveredYouTubePlayback.descriptor.sessionId
      ? recoveredYouTubePlayback
      : youtubePlayback ?? recoveredYouTubePlayback;
  const globalPlaybackStatus = React.useMemo(
    () =>
      projectCoordinatorPlaybackStatus(
        coordinatorPlaybackSession,
        listenNowPlaying,
        youtubePlaybackForPage,
      ) ??
      listenNowPlaying,
    [coordinatorPlaybackSession, listenNowPlaying, youtubePlaybackForPage],
  );
  const [fullscreenPlayer, setFullscreenPlayer] = React.useState<
    "music" | null
  >(null);
  const lastMusicRouteByScopeRef = React.useRef({
    online: "home",
    local: "local-home",
  });
  const playerFullscreen = fullscreenPlayer !== null;
  const listenCommandIdRef = React.useRef(0);
  const youtubeCommandIdRef = React.useRef(0);
  const fullscreenCloseButtonRef = React.useRef<HTMLButtonElement | null>(null);
  const listenNotificationKeyRef = React.useRef("");
  const activeOperationSnapshotRef = React.useRef<Map<string, OperationListItemDTO>>(new Map());
  const notifiedOperationIdsRef = React.useRef<Set<string>>(new Set());
  const companionAffectsLayout = workspaceCompanionAffectsLayout(
    companion.open,
    playerFullscreen,
  );
  const { companionPresentation, fitMinimumWidth } =
    useWorkspaceWindowFit(companionAffectsLayout);

  const playbackCompanionOpen =
    companion.open &&
    (companion.destination?.id === "player" ||
      companion.destination?.id === "lyrics" ||
      companion.destination?.id === "queue");
  const youtubeUpNextOpen =
    companion.open && companion.destination?.id === "youtube-up-next";

  const text = getXiaText(settings?.language);
  const sniffStatus = sniffStatusQuery.data ?? projectSniffStatusSnapshot();
  const sniffWorkspaceFiltersVisible =
    sniffStatus.runtime === "managed" && Boolean(sniffStatus.sessionId);
  const appearance = readXiaAppearance(settings);
  const theme = resolveThemePack(appearance.themePackId);
  const isWindows = System.IsWindows();
  const welcomeOpen = !setupState.completed || debugWelcomeOpen;
  React.useEffect(() => {
    const previousWorkspaceId = previousWorkspaceIdRef.current;
    previousWorkspaceIdRef.current = activeWorkspaceId;
    if (
      activeWorkspaceId === APP_WORKSPACE_IDS.rss &&
      previousWorkspaceId !== APP_WORKSPACE_IDS.rss &&
      !readRSSPreviewNoticeDismissed()
    ) {
      setRSSPreviewNoticeDontShowAgain(false);
      setRSSPreviewNoticeOpen(true);
      return;
    }
    if (activeWorkspaceId !== APP_WORKSPACE_IDS.rss) {
      setRSSPreviewNoticeOpen(false);
    }
  }, [activeWorkspaceId]);
  const closeRSSPreviewNotice = React.useCallback(() => {
    if (rssPreviewNoticeDontShowAgain) {
      dismissRSSPreviewNotice();
    }
    setRSSPreviewNoticeOpen(false);
  }, [rssPreviewNoticeDontShowAgain]);
  const stopSniffActivity = React.useCallback(
    async () => {
      try {
        if (sniffStatus.runtime === "managed" && sniffStatus.sessionId) {
          await cancelResourceSniff.mutateAsync({ sessionId: sniffStatus.sessionId });
          return;
        }
        if (sniffStatus.runtime === "orphan" && sniffStatus.runtimeId) {
          await stopCDPBrowserRuntime.mutateAsync({ runtimeId: sniffStatus.runtimeId });
        }
      } catch (error) {
        messageBus.publishToast({
          intent: "danger",
          title: text.sniffDesk.cdpClose,
          description: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [cancelResourceSniff, sniffStatus, stopCDPBrowserRuntime, text.sniffDesk],
  );
  const clearSniffActivity = React.useCallback(async () => {
    if (!sniffStatus.sessionId || !sniffStatus.canClear) {
      return;
    }
    try {
      await clearResourceSniffResources.mutateAsync({
        sessionId: sniffStatus.sessionId,
      });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: text.sniffDesk.clearResourcesFailed,
        description: error instanceof Error ? error.message : String(error),
      });
    }
  }, [clearResourceSniffResources, sniffStatus, text.sniffDesk]);
  const runningOperations = runningQuery.data ?? [];
  const operationActivity = React.useMemo(
    () => projectOperationActivitySnapshot(runningOperations),
    [runningOperations],
  );
  const terminalOperations = terminalQuery.data ?? [];
  const endedOperations = React.useMemo(
    () => endedOperationsQuery.data
      ? sortAndDedupeOperations([
        endedOperationsQuery.data,
        terminalOperations,
      ])
      : [],
    [endedOperationsQuery.data, terminalOperations],
  );
  React.useEffect(() => {
    setLibraryBrowseQuery("");
    setLibraryPage(1);
  }, [libraryContentRoute]);
  React.useEffect(() => {
    if (!playerFullscreen) {
      return;
    }
    const previousFocus = document.activeElement as HTMLElement | null;
    const backgroundRegions = Array.from(
      document.querySelectorAll<HTMLElement>(
        ".app-workspace-sidebar, .app-workspace-primary-pane",
      ),
    );
    backgroundRegions.forEach((element) => {
      element.inert = true;
    });
    fullscreenCloseButtonRef.current?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) {
        return;
      }
      const nestedDialogOpen = document.querySelector(
        '[role="dialog"][data-state="open"]',
      );
      if (nestedDialogOpen) {
        return;
      }
      setFullscreenPlayer(null);
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      backgroundRegions.forEach((element) => {
        element.inert = false;
      });
      previousFocus?.focus();
    };
  }, [playerFullscreen]);

  const libraries = librariesQuery.data ?? [];
  const filesById = React.useMemo(
    () =>
      new Map(
        libraries.flatMap((library) =>
          library.files.map((file) => [file.id, file] as const),
        ),
      ),
    [libraries],
  );
  const companionLegacyFileItems = React.useMemo(
    () => adaptLegacyLibraryFiles(libraries, httpBaseURL),
    [httpBaseURL, libraries],
  );
  const availableLegacyImageFiles = React.useMemo(() => {
    if (!catalogItemsQuery.data) return [];
    const catalogImageFileIds = new Set(
      catalogItemsQuery.data.items
        .filter((item) => item.category === "image")
        .map((item) => item.primaryFileId?.trim() ?? "")
        .filter(Boolean),
    );
    return adaptAvailableLegacyImageFiles(
      libraries,
      httpBaseURL,
      catalogImageFileIds,
    );
  }, [catalogItemsQuery.data?.items, httpBaseURL, libraries]);
  const libraryWorkspaceItems = React.useMemo(() => {
    const adapted = adaptLegacyLibraryWorkspace(
      libraries,
      endedOperations,
      httpBaseURL,
    );
    const catalogFiles = catalogItemsQuery.data
      ? adaptCatalogItems(catalogItemsQuery.data.items, {
          filesById,
          httpBaseURL,
        })
      : adapted.files;
    return [...catalogFiles, ...availableLegacyImageFiles, ...adapted.tasks];
  }, [
    availableLegacyImageFiles,
    catalogItemsQuery.data,
    filesById,
    httpBaseURL,
    libraries,
    endedOperations,
  ]);
  const pagedLibraryWorkspaceItems = React.useMemo(
    () => adaptCatalogItems(libraryCatalogPageQuery.data?.items ?? [], {
      filesById,
      httpBaseURL,
    }),
    [filesById, httpBaseURL, libraryCatalogPageQuery.data?.items],
  );
  const visibleLibraryWorkspaceItems = serverLibraryPagination
    ? pagedLibraryWorkspaceItems
    : libraryWorkspaceItems;
  const librarySearchLanding =
    libraryContentRoute === "search" && libraryBrowseQuery.trim().length === 0;
  const libraryPreviewCandidatesById = React.useMemo(() => {
    const candidates = librarySearchLanding
      ? []
      : serverLibraryPagination
        ? pagedLibraryWorkspaceItems
        : libraryWorkspaceItems;
    return new Map(
      [...companionLegacyFileItems, ...candidates]
        .map((item) => [item.id.trim(), item] as const),
    );
  }, [
    companionLegacyFileItems,
    librarySearchLanding,
    libraryWorkspaceItems,
    pagedLibraryWorkspaceItems,
    serverLibraryPagination,
  ]);
  const libraryPreviewCanonicalLoading = serverLibraryPagination
    ? libraryCatalogPageQuery.isFetching
    : libraryContentRoute === "ended"
      ? endedOperationsQuery.isFetching
      : completeCatalogNeeded && catalogItemsQuery.isFetching;
  const libraryPreviewCanonicalAuthoritative = librarySearchLanding
    ? true
    : serverLibraryPagination
      ? libraryCatalogPageQuery.isSuccess &&
        libraryCatalogPageQuery.isPlaceholderData !== true
      : libraryContentRoute === "ended"
        ? endedOperationsQuery.isSuccess
        : completeCatalogNeeded
          ? catalogItemsQuery.isSuccess
          : librariesQuery.isSuccess;
  const libraryPreviewSelection = resolveActiveCompanionSelectionFromMap(
    companion,
    LIBRARY_PREVIEW_SELECTION_CONTRACT,
    libraryPreviewCandidatesById,
    (item) => item.id,
    {
      loading: libraryPreviewCanonicalLoading,
      authoritative: libraryPreviewCanonicalAuthoritative,
      loadingSnapshot: libraryPreviewLoadingSnapshot,
    },
  );
  const libraryPreviewSelectionRef = React.useRef(libraryPreviewSelection);
  libraryPreviewSelectionRef.current = libraryPreviewSelection;
  const activeLibraryPreviewItem = libraryPreviewSelection.item;

  React.useEffect(() => {
    if (
      libraryPreviewSelection.status !== "missing" ||
      !libraryPreviewSelection.id
    ) {
      return;
    }
    const missingId = libraryPreviewSelection.id;
    const latestSelection = libraryPreviewSelectionRef.current;
    if (
      latestSelection.status !== "missing" ||
      latestSelection.id !== missingId
    ) {
      return;
    }
    const currentCompanion = useAppWorkspaceStore.getState().companion;
    if (
      resolveActiveCompanionSelectionId(
        currentCompanion,
        LIBRARY_PREVIEW_SELECTION_CONTRACT,
      ) !== missingId
    ) {
      return;
    }
    setLibraryPreviewLoadingSnapshot((snapshot) =>
      snapshot?.id.trim() === missingId ? null : snapshot,
    );
    closeCompanion(true);
  }, [
    closeCompanion,
    libraryPreviewSelection.id,
    libraryPreviewSelection.status,
  ]);
  const transcodeLibrarySources = React.useMemo<NewTaskDialogTranscodeSource[]>(() => {
    const transcodeWorkspaceItems = adaptCatalogItems(
      transcodeCatalogItemsQuery.data?.items ?? [],
      { filesById, httpBaseURL },
    );
    const workspaceItemById = new Map(
      transcodeWorkspaceItems
        .map((item) => [item.id, item] as const),
    );
    return (transcodeCatalogItemsQuery.data?.items ?? [])
      .filter(
        (item) => Boolean(item.primaryFileId?.trim()),
      )
      .map((item) => {
        const workspaceItem = workspaceItemById.get(item.id);
        return {
          fileId: item.primaryFileId,
          displayLabel: item.title,
          title: item.title,
          libraryId: item.catalogId,
          libraryName: workspaceItem?.libraryName || text.workspace.libraryStation,
          coverURL: workspaceItem?.coverURL,
          format: item.format,
          durationMs: item.durationMs,
          sizeBytes: item.sizeBytes,
        };
      });
  }, [
    filesById,
    httpBaseURL,
    text.workspace.libraryStation,
    transcodeCatalogItemsQuery.data?.items,
  ]);
  const librariesById = React.useMemo(
    () => new Map(libraries.map((item) => [item.id, item])),
    [libraries],
  );
  const runningPetAnimation = useRunningPetAnimation(
    runningOperations,
    terminalOperations,
    terminalQuery.isFetched,
  );
  const dependencyItems = React.useMemo(
    () =>
      (toolsQuery.data ?? []).filter((item) =>
        CORE_DEPENDENCIES.includes(
          item.name as (typeof CORE_DEPENDENCIES)[number],
        ),
      ),
    [toolsQuery.data],
  );
  const dependencyUpdatesByName = React.useMemo(
    () =>
      new Map(
        (dependencyUpdatesQuery.data ?? []).map((item) => [item.name, item]),
      ),
    [dependencyUpdatesQuery.data],
  );
  const dependencyUpdateCount = React.useMemo(
    () =>
      dependencyItems.filter((item) => {
        if ((item.status ?? "").trim().toLowerCase() !== "installed") {
          return false;
        }
        const latest = normalizeDependencyVersion(
          dependencyUpdatesByName.get(item.name)?.latestVersion,
          item.name,
        );
        const current = normalizeDependencyVersion(item.version, item.name);
        return Boolean(current && latest && current !== latest);
      }).length,
    [dependencyItems, dependencyUpdatesByName],
  );
  const hasDependencyUpdate = dependencyUpdateCount > 0;
  const hasPreparedAppUpdate =
    updateInfo.status === "ready_to_restart" && hasPreparedUpdate(updateInfo);
  const hasAppUpdateMenu =
    hasPreparedAppUpdate ||
    hasRemoteUpdate(updateInfo) ||
    updateInfo.status === "downloading" ||
    updateInfo.status === "installing";
  const libraryAccessCopy = text.settings.libraryAccess;
  const libraryAccessRemote = libraryAccessConfigQuery.data?.remoteEnabled === true;
  const libraryAccessTone = resolveLibraryAccessStatusTone(libraryAccessStatusQuery.data);
  const libraryAccessError = safeLibraryAccessBackendErrorMessage(updateLibraryAccess.error, libraryAccessCopy)
    || safeLibraryAccessBackendErrorMessage(libraryAccessConfigQuery.error, libraryAccessCopy)
    || safeLibraryAccessBackendErrorMessage(libraryAccessStatusQuery.error, libraryAccessCopy)
    || safeLibraryAccessBackendErrorMessage(libraryAccessStatusQuery.data?.lan.lastError, libraryAccessCopy)
    || safeLibraryAccessBackendErrorMessage(libraryAccessStatusQuery.data?.tailscale.lastError, libraryAccessCopy);
  const libraryAccessStatusLabel = libraryAccessError
    ? libraryAccessCopy.remoteUnavailable
    : updateLibraryAccess.isPending || libraryAccessConfigQuery.isLoading
      ? libraryAccessCopy.starting
      : !libraryAccessRemote
        ? libraryAccessCopy.localOnly
        : libraryAccessTone === "success"
          ? libraryAccessCopy.remoteReady
          : libraryAccessTone === "danger"
            ? libraryAccessCopy.remoteUnavailable
            : libraryAccessTone === "pending"
              ? libraryAccessCopy.starting
              : libraryAccessCopy.remoteUnavailable;
  const shellTheme = "dream";
  const isLibraryWorkspace = activeWorkspaceId === APP_WORKSPACE_IDS.library;
  const activeWorkspaceRoute =
    workspaceLocations[activeWorkspaceId]?.routeId ||
    (activeWorkspaceId === APP_WORKSPACE_IDS.library
      ? libraryWorkspaceRoute
      : activeWorkspaceId === APP_WORKSPACE_IDS.music
      ? "home"
      : activeWorkspaceId === APP_WORKSPACE_IDS.sniff
        ? "resources"
        : activeWorkspaceId === APP_WORKSPACE_IDS.youtube
          ? "home"
          : activeWorkspaceId === APP_WORKSPACE_IDS.rss
            ? "all"
          : "running");
  const musicWorkspaceRouteId =
    workspaceLocations[APP_WORKSPACE_IDS.music]?.routeId || "home";
  const resolvedMusicWorkspaceRoute = resolveMusicWorkspaceRoute(
    musicWorkspaceRouteId,
  );
  const musicWorkspaceScope = resolvedMusicWorkspaceRoute.scope;
  const showWorkspacePlaybackActivity = shouldShowWorkspacePlaybackActivity(
    globalPlaybackStatus?.playbackSource,
    activeWorkspaceId,
    musicWorkspaceScope,
    youtubeWatchSurfaceVisible,
  );
  React.useEffect(() => {
    if (activeWorkspaceId === APP_WORKSPACE_IDS.music) {
      lastMusicRouteByScopeRef.current[musicWorkspaceScope] =
        activeWorkspaceRoute;
    }
  }, [activeWorkspaceId, activeWorkspaceRoute, musicWorkspaceScope]);
  const workspaceUsesYouTubeSession =
    activeWorkspaceId === APP_WORKSPACE_IDS.youtube ||
    (activeWorkspaceId === APP_WORKSPACE_IDS.music &&
      musicWorkspaceScope === "online");
  const youtubeAppSession = React.useMemo(
    () =>
      (appSessions.data ?? []).find(
        (session) => session.siteKey.trim().toLowerCase() === "youtube",
      ) ?? null,
    [appSessions.data],
  );
  const workspaceSessionAccount = workspaceUsesYouTubeSession
    ? youtubeAppSession?.account ?? null
    : null;
  const workspaceIdentityName = workspaceUsesYouTubeSession
    ? workspaceSessionAccount?.displayName?.trim() ||
      youtubeAppSession?.label?.trim() ||
      (activeWorkspaceId === APP_WORKSPACE_IDS.youtube
        ? text.workspace.youtube
        : text.workspace.youtubeMusic)
    : resolveUserDisplayName(profile);
  const workspaceIdentityUsername = workspaceUsesYouTubeSession
    ? workspaceSessionAccount?.handle?.trim() ||
      text.listen.museAccountDisconnectedName
    : profile?.username?.trim() || text.sidebar.profileHint;
  const workspaceIdentityAvatar = workspaceUsesYouTubeSession ? (
    <WorkspaceSessionAvatar
      name={workspaceIdentityName}
      src={workspaceSessionAccount?.avatarURL?.trim() || ""}
    />
  ) : (
    <UserAvatar
      profile={profile}
      tone="neutral"
      className="app-workspace-session-avatar"
      shape="circle"
    />
  );
  const workspacePrimaryMinWidth = PRIMARY_PANE_DEFAULT_MIN_WIDTH;
  const activityLabels = React.useMemo<WorkspaceActivityLabels>(
    () => ({
      sniff: text.sniffDesk.title,
      stopSniff: text.sniffDesk.stopSniff,
      sniffState: {
        idle: text.sniffDesk.statusClosed,
        starting: text.sniffDesk.loading,
        active: text.sniffDesk.statusOpen,
        closing: text.sniffDesk.statusClosing,
        error: text.listen.errorStatus,
        orphan: text.sniffDesk.cdpOrphan,
      },
      resources: text.sniffDesk.resources,
      downloadable: text.sniffDesk.downloadableOnly,
      session: text.sniffDesk.sessions,
      updated: text.sniffDesk.updatedAt,
      clear: text.sniffDesk.clearResources,
      operations: text.running.title,
      downloads: text.actions.download,
      transcodes: text.actions.transcode,
      nowPlaying: text.listen.nowPlaying,
      previous: text.actions.previous,
      play: text.actions.play,
      pause: text.actions.pause,
      next: text.actions.nextTrack,
      noActivity: text.running.empty,
    }),
    [text],
  );

  const activePet = React.useMemo(
    () => resolveActivePet(petsQuery.data ?? [], settings),
    [settings, petsQuery.data],
  );
  const activePetImageURL = React.useMemo(
    () =>
      activePet
        ? buildAssetPreviewURL(httpBaseURL, activePet.spritesheetPath, activePet.updatedAt)
        : "",
    [activePet, httpBaseURL],
  );

  React.useEffect(() => {
    if (updateStateQuery.data) {
      setUpdateInfo(updateStateQuery.data);
    }
  }, [setUpdateInfo, updateStateQuery.data]);

  React.useEffect(() => {
    void setWelcomeWindowChromeHidden(welcomeOpen).catch(() => {
      // Browser preview and older runtimes can ignore the native chrome bridge.
    });
  }, [welcomeOpen]);

  React.useEffect(() => {
    const emitWelcomeCommand = (detail: WelcomeDebugCommand) => {
      window.dispatchEvent(new CustomEvent(WELCOME_DEBUG_EVENT, { detail }));
    };
    const openWelcomeAndEmit = (detail: WelcomeDebugCommand) => {
      setDebugWelcomeOpen(true);
      window.setTimeout(() => emitWelcomeCommand(detail), 40);
    };
    const api = {
      show: () => openWelcomeAndEmit({ type: "show" }),
      hide: () => {
        emitWelcomeCommand({ type: "hide" });
        setDebugWelcomeOpen(false);
      },
      reset: () => {
        window.localStorage.removeItem(SETUP_STORAGE_KEY);
        setSetupState({ completed: false });
        setDebugWelcomeOpen(false);
        window.setTimeout(
          () => emitWelcomeCommand({ type: "step", step: "proxy" }),
          40,
        );
      },
      step: (step: WelcomeDebugStep) => {
        openWelcomeAndEmit({ type: "step", step });
      },
      proxy: (mode: "none" | "system") => {
        openWelcomeAndEmit({ type: "proxy", mode });
      },
    };

    window.xiadownWelcome = api;
    return () => {
      if (window.xiadownWelcome === api) {
        delete window.xiadownWelcome;
      }
    };
  }, [setSetupState]);

  React.useEffect(() => {
    const snapshots = activeOperationSnapshotRef.current;
    runningOperations.forEach((operation) => {
      const id = operation.operationId.trim();
      if (!id) {
        return;
      }
      snapshots.set(id, operation);
    });
  }, [runningOperations]);

  React.useEffect(() => {
    const snapshots = activeOperationSnapshotRef.current;
    const notified = notifiedOperationIdsRef.current;
    terminalOperations.forEach((operation) => {
      const operationId = operation.operationId.trim();
      if (!operationId) {
        return;
      }
      const status = normalizeOperationStatus(operation.status);
      if (status === "canceled") {
        snapshots.delete(operationId);
        return;
      }
      if (!NOTIFIABLE_OPERATION_STATUSES.has(status)) {
        return;
      }
      if (notified.has(operationId) || !snapshots.has(operationId)) {
        return;
      }
      notified.add(operationId);
      snapshots.delete(operationId);

      const title = resolveOperationNotificationTitle(operation);
      const statusLabel = resolveCompletedStatusLabel(text, status);
      const coverURL = resolveOperationNotificationCoverURL(
        httpBaseURL,
        operation,
        filesById,
        librariesById,
      );
      void publishOSNotification({
        id: `operation_${operationId}_${status}`,
        title,
        body: statusLabel,
        iconUrl: coverURL,
        imageUrl: coverURL,
        source: "XiaDown",
        data: {
          source: "operation",
          operationId,
          status,
          title,
          libraryId: operation.libraryId,
          libraryName: operation.libraryName ?? "",
        },
      });
    });
  }, [text, filesById, httpBaseURL, librariesById, terminalOperations]);

  React.useEffect(() => {
    try {
      if (globalPlaybackStatus) {
        localStorage.setItem(
          LISTEN_NOW_PLAYING_STORAGE_KEY,
          JSON.stringify(globalPlaybackStatus),
        );
      } else {
        localStorage.removeItem(LISTEN_NOW_PLAYING_STORAGE_KEY);
      }
    } catch {
      // noop
    }
    void Events.Emit(LISTEN_NOW_PLAYING_EVENT, globalPlaybackStatus);
  }, [globalPlaybackStatus]);

  React.useEffect(() => {
    if (!listenNowPlaying || listenNowPlaying.state !== "playing") {
      return;
    }
    const title = listenNowPlaying.title.trim();
    if (!title) {
      return;
    }
    const artist = listenNowPlaying.subtitle.trim();
    const artworkURL = listenNowPlaying.artworkURL.trim();
    const notificationKey = [
      listenNowPlaying.mode,
      title,
      artist,
      artworkURL,
    ].join("::");
    if (listenNotificationKeyRef.current === notificationKey) {
      return;
    }
    listenNotificationKeyRef.current = notificationKey;

    void publishOSNotification({
      id: `listen_${Date.now()}`,
      title,
      body: artist || text.listen.nowPlaying,
      iconUrl: artworkURL,
      imageUrl: artworkURL,
      source: "Listen",
      data: {
        source: "listen",
        mode: listenNowPlaying.mode,
        title,
        artist,
        artworkURL,
      },
    });
  }, [text.listen.nowPlaying, listenNowPlaying]);

  const openNewTaskDialog = React.useCallback((
    mode: NewTaskDialogMode,
    url = "",
    origin: { source: string; caller: string } | null = null,
    targets: readonly NewTaskDialogDownloadTarget[] = [],
  ) => {
    setNewTaskDialogMode(mode);
    setPrefilledDownloadURL(mode === "download" ? url : "");
    setPrefilledDownloadOrigin(mode === "download" ? origin : null);
    setPrefilledDownloadTargets(mode === "download" ? [...targets] : []);
    setPrefilledTranscodeSource(null);
    setNewTaskDialogOpen(true);
  }, []);

  const openDownloadDialog = React.useCallback((url = "") => {
    openNewTaskDialog("download", url);
  }, [openNewTaskDialog]);

  const openRSSDownloadDialog = React.useCallback((
    url: string | undefined,
    entry: RSSEntry,
    batchTargets: readonly RSSVideoBatchDownloadTarget[] = [],
  ) => {
    openNewTaskDialog(
      "download",
      url ?? "",
      {
        source: "xiadown.rss",
        caller: `rss-entry:${entry.id}`,
      },
      batchTargets.map(({ url: targetURL, source, caller }) => ({
        url: targetURL,
        source,
        caller,
      })),
    );
  }, [openNewTaskDialog]);

  const sendListenCommand = React.useCallback(
    (
      command: ListenExternalCommand["command"],
      value?: number,
      artist?: ListenExternalCommand["artist"],
      backendStopped?: boolean,
    ) => {
      listenCommandIdRef.current += 1;
      setListenControlCommand({
        id: listenCommandIdRef.current,
        command,
        value,
        artist,
        backendStopped,
      });
    },
    [],
  );

	const openYouTubeWatch = React.useCallback(() => {
		setYouTubeWatchRevealRequest((request) => request + 1);
		setYouTubeWatchOpen(true);
		activateWorkspace(
			APP_WORKSPACE_IDS.youtube,
			workspaceLocations[APP_WORKSPACE_IDS.youtube] ?? { routeId: "home" },
		);
	}, [activateWorkspace, workspaceLocations]);

  const setYouTubeWatchSurfaceOpen = React.useCallback(
    (open: boolean) => {
      setYouTubeWatchOpen(open);
      if (!open && companion.destination?.id === "youtube-up-next") {
        closeCompanion();
      }
    },
    [closeCompanion, companion.destination?.id],
  );

  const toggleYouTubeUpNext = React.useCallback(() => {
    toggleCompanion({
      id: "youtube-up-next",
      scope: {
        kind: "workspace",
        workspaceId: APP_WORKSPACE_IDS.youtube,
      },
    });
  }, [toggleCompanion]);

  const sendGlobalPlaybackCommand = React.useCallback(
    (
      command: GlobalPlaybackCommand,
      options: { revealYouTube?: boolean } = {},
    ) => {
      const session = coordinatorPlaybackSession;
      if (
        session?.focus === "persistent" &&
        session.item.source.provider === "youtube" &&
        options.revealYouTube !== false &&
        (command !== "toggle" || session.state !== "playing")
      ) {
        openYouTubeWatch();
      }

      const route = resolveGlobalPlaybackCommandRoute(session, command);
      if (route.target === "listen") {
        sendListenCommand(
          resolveListenFallbackPlaybackCommand(
            globalPlaybackStatus,
            route.command,
          ),
        );
        return;
      }
      if (route.target === "youtube-queue") {
        youtubeCommandIdRef.current += 1;
        setYouTubeControlCommand({
          id: youtubeCommandIdRef.current,
          command: route.command,
          revealWatch: options.revealYouTube !== false,
        });
        return;
      }
      if (route.target === "coordinator") {
        void playbackCoordinator.commands[route.command]().catch(() => {});
      }
    },
    [
      coordinatorPlaybackSession,
      globalPlaybackStatus,
      openYouTubeWatch,
      playbackCoordinator.commands,
      sendListenCommand,
    ],
  );

  const sendMusicWorkspaceTransportCommand = React.useCallback(
    (command: ListenExternalCommand["command"], value?: number) => {
      if (
        command === "previous" ||
        command === "toggle" ||
        command === "next"
      ) {
        sendGlobalPlaybackCommand(command);
        return;
      }
      sendListenCommand(command, value);
    },
    [sendGlobalPlaybackCommand, sendListenCommand],
  );

  React.useEffect(() => {
    if (companion.destination?.id !== "youtube-player") {
      return;
    }
    const shouldRestoreWatch =
      companion.open &&
      (coordinatorPlaybackSession?.item.source.provider === "youtube" ||
        globalPlaybackStatus?.playbackSource === "youtube");
    closeCompanion(true);
    if (shouldRestoreWatch) {
      openYouTubeWatch();
    }
  }, [
    closeCompanion,
    companion.destination?.id,
    companion.open,
    coordinatorPlaybackSession?.item.source.provider,
    globalPlaybackStatus?.playbackSource,
    openYouTubeWatch,
  ]);

  const openGlobalCompanion = React.useCallback(
    (id: "player" | "operations" | "sniff") => {
      openCompanion({ id, scope: { kind: "global" } });
    },
    [openCompanion],
  );

  const toggleMusicCompanion = React.useCallback(
    (id: "lyrics" | "queue") => {
      const currentScope = companion.destination?.scope;
      const sameDestination =
        companion.destination?.id === id &&
        currentScope?.kind === "workspace" &&
        currentScope.workspaceId === APP_WORKSPACE_IDS.music;
      const opening =
        !companion.open || !sameDestination;
      toggleCompanion({
        id,
        scope: {
          kind: "workspace",
          workspaceId: APP_WORKSPACE_IDS.music,
        },
      });
      return opening;
    },
    [companion.destination, companion.open, toggleCompanion],
  );

  const requestMusicCompanionMode = React.useCallback(
    (mode: "lyrics" | "queue") => {
      if (toggleMusicCompanion(mode)) {
        sendListenCommand(mode === "lyrics" ? "show-lyrics" : "show-queue");
      }
    },
    [sendListenCommand, toggleMusicCompanion],
  );

  const openCurrentMusicArtist = React.useCallback(
    (artist?: ListenExternalCommand["artist"]) => {
      const sendOpenArtistCommand = () => {
        if (artist) {
          sendListenCommand("open-artist", undefined, artist);
          return;
        }
        sendListenCommand("open-artist");
      };
      if (resolvedMusicWorkspaceRoute.mode === "muse") {
        sendOpenArtistCommand();
        return;
      }
      const rememberedOnlineRoute = lastMusicRouteByScopeRef.current.online;
      const targetRoute =
        resolveMusicWorkspaceRoute(rememberedOnlineRoute).mode === "muse"
          ? rememberedOnlineRoute
          : "home";
      navigateWorkspace({ routeId: targetRoute });
      window.requestAnimationFrame(sendOpenArtistCommand);
    },
    [navigateWorkspace, resolvedMusicWorkspaceRoute.mode, sendListenCommand],
  );

  const openActivePlaybackSurface = React.useCallback(() => {
    if (
      coordinatorPlaybackSession?.item.source.provider === "youtube" ||
      globalPlaybackStatus?.playbackSource === "youtube"
    ) {
      openYouTubeWatch();
      return;
    }
    if (coordinatorPlaybackSession?.focus === "transient_preview") {
      openCompanion({ id: "playback-summary", scope: { kind: "global" } });
      return;
    }
    openGlobalCompanion("player");
  }, [
    coordinatorPlaybackSession,
    globalPlaybackStatus?.playbackSource,
    openCompanion,
    openGlobalCompanion,
    openYouTubeWatch,
  ]);

  const openWorkspace = React.useCallback(
    (workspaceId: AppWorkspaceId, fallbackRoute: string) => {
      activateWorkspace(
        workspaceId,
        workspaceLocations[workspaceId] ?? { routeId: fallbackRoute },
      );
    },
    [activateWorkspace, workspaceLocations],
  );

  const openMusicPlaybackSource = React.useCallback(
    (source: ListenPlaybackSource) => {
      switch (source) {
        case "youtube_music":
          activateWorkspace(APP_WORKSPACE_IDS.music, { routeId: "home" });
          return;
        case "radio":
          activateWorkspace(APP_WORKSPACE_IDS.music, { routeId: "radio" });
          return;
        case "local":
          activateWorkspace(APP_WORKSPACE_IDS.music, { routeId: "local-home" });
          return;
        case "youtube":
          activateWorkspace(APP_WORKSPACE_IDS.youtube, { routeId: "home" });
          return;
        case "library_preview":
          activateWorkspace(APP_WORKSPACE_IDS.library, { routeId: "all" });
          return;
        default:
          return;
      }
    },
    [activateWorkspace],
  );

  const requestMusicPlayerFullscreen = React.useCallback(() => {
    setFullscreenPlayer("music");
  }, []);
  const exitMusicPlayerFullscreen = React.useCallback(() => {
    setFullscreenPlayer((current) => (current === "music" ? null : current));
  }, []);

  const openLibraryRoute = React.useCallback(
    (routeId: string) => {
      activateWorkspace(APP_WORKSPACE_IDS.library, { routeId });
    },
    [activateWorkspace],
  );

  const stopActivePlayback = React.useCallback(async () => {
    const status = globalPlaybackStatus;
    if (!status || status.state === "idle") {
      return;
    }
    const source = status.playbackSource ?? "unknown";
    const activeSession = playbackCoordinator.snapshot.active;
    try {
      if (activeSession) {
        await playbackCoordinator.commands.closeSession(activeSession.id);
        if (source === "youtube") {
          setYouTubePlayback(null);
          youtubeCommandIdRef.current += 1;
          setYouTubeControlCommand({
            id: youtubeCommandIdRef.current,
            command: "stop",
            revealWatch: false,
          });
        }
        if (
          source === "youtube_music" ||
          source === "radio" ||
          source === "local"
        ) {
          sendListenCommand("stop", undefined, undefined, true);
        }
        return;
      }
      if (source === "youtube_music" || source === "radio") {
        // Legacy Music/Radio playback can predate the coordinator snapshot.
        // Listen performs the native Reset first and only then clears its UI.
        sendListenCommand("stop");
      }
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: text.listen.stop,
        description: error instanceof Error ? error.message : String(error),
      });
    }
  }, [
    globalPlaybackStatus,
    playbackCoordinator.commands,
    playbackCoordinator.snapshot.active,
    sendListenCommand,
    text.listen.stop,
  ]);

  const operationActivityMenuActions = React.useMemo<
    WorkspaceActivityMenuAction[]
  >(
    () => [
      {
        key: "view",
        label: text.actions.view,
        icon: <Eye />,
        onSelect: () => openGlobalCompanion("operations"),
      },
      {
        key: "station",
        label: text.running.title,
        icon: <Activity />,
        onSelect: () => openLibraryRoute("running"),
      },
    ],
    [openGlobalCompanion, openLibraryRoute, text.actions.view, text.running.title],
  );

  const sniffActivityMenuActions = React.useMemo<
    WorkspaceActivityMenuAction[]
  >(
    () => [
      {
        key: "view",
        label: text.actions.view,
        icon: <Eye />,
        onSelect: () => openGlobalCompanion("sniff"),
      },
      {
        key: "stop",
        label: text.listen.stop,
        icon: <Square />,
        disabled:
          !sniffStatus.canStop ||
          cancelResourceSniff.isPending ||
          stopCDPBrowserRuntime.isPending,
        tone: "destructive",
        onSelect: () => void stopSniffActivity(),
      },
    ],
    [
      cancelResourceSniff.isPending,
      openGlobalCompanion,
      sniffStatus.canStop,
      stopCDPBrowserRuntime.isPending,
      stopSniffActivity,
      text.actions.view,
      text.listen.stop,
    ],
  );

  const playbackActivityMenuActions = React.useMemo<
    WorkspaceActivityMenuAction[]
  >(() => {
    const status = globalPlaybackStatus;
    if (!status || status.state === "idle") {
      return [];
    }
    const source = status.playbackSource ?? "unknown";
    const actions: WorkspaceActivityMenuAction[] = [];
    if (source !== "youtube") {
      actions.push({
        key: "view",
        label: text.actions.view,
        icon: <Eye />,
        onSelect: openActivePlaybackSurface,
      });
    }
    const station = (() => {
      switch (source) {
        case "youtube_music":
          return { label: text.workspace.music, icon: <Music2 /> };
        case "youtube":
          return { label: text.workspace.youtube, icon: <Youtube /> };
        case "radio":
          return { label: text.workspace.radio, icon: <Radio /> };
        case "local":
          return { label: text.workspace.local, icon: <HardDrive /> };
        case "library_preview":
          return {
            label: text.workspace.libraryStation,
            icon: <LibraryBig />,
          };
        default:
          return null;
      }
    })();
    if (station) {
      actions.push({
        key: "station",
        label: station.label,
        icon: station.icon,
        onSelect: () => openMusicPlaybackSource(source),
      });
    }
    actions.push({
      key: "stop",
      label: text.listen.stop,
      icon: <Square />,
      disabled:
        !playbackCoordinator.snapshot.active &&
        source !== "youtube_music" &&
        source !== "radio",
      tone: "destructive",
      onSelect: () => void stopActivePlayback(),
    });
    return actions;
  }, [
    globalPlaybackStatus,
    openActivePlaybackSurface,
    openMusicPlaybackSource,
    playbackCoordinator.snapshot.active,
    stopActivePlayback,
    text.actions.view,
    text.listen.stop,
    text.workspace.libraryStation,
    text.workspace.local,
    text.workspace.music,
    text.workspace.radio,
    text.workspace.youtube,
  ]);

  const openLibraryPreview = React.useCallback(
    (item: LibraryWorkspaceItem) => {
      setLibraryPreviewLoadingSnapshot(item);
      setLibraryPreviewTab("preview");
      openCompanion(
        createCompanionSelectionDestination(
          LIBRARY_PREVIEW_SELECTION_CONTRACT,
          {
            kind: "route",
            workspaceId: APP_WORKSPACE_IDS.library,
            routeId: activeWorkspaceRoute,
          },
          item.id,
        ),
      );
    },
    [activeWorkspaceRoute, openCompanion],
  );
  const openLibraryDeletedItems = React.useCallback(() => {
    openCompanion({
      id: "library-deleted",
      scope: {
        kind: "workspace",
        workspaceId: APP_WORKSPACE_IDS.library,
      },
    });
  }, [openCompanion]);
  const openLibraryPreviewById = React.useCallback(
    (itemId: string) => {
      const item = libraryPreviewCandidatesById.get(itemId.trim());
      if (item) openLibraryPreview(item);
    },
    [libraryPreviewCandidatesById, openLibraryPreview],
  );

  React.useEffect(() => {
    const offTrayCommand = Events.On(LISTEN_TRAY_COMMAND_EVENT, (event: any) => {
      const payload = event?.data ?? event;
      const command =
        typeof payload === "string"
          ? payload
          : payload && typeof payload === "object" && typeof payload.command === "string"
            ? payload.command
            : "";
      if (
        command === "previous" ||
        command === "toggle" ||
        command === "play" ||
        command === "pause" ||
        command === "next"
      ) {
        sendGlobalPlaybackCommand(command, { revealYouTube: false });
      }
    });
    return () => {
      offTrayCommand();
    };
  }, [sendGlobalPlaybackCommand]);

  React.useEffect(() => {
    const offNewDownload = Events.On(MAIN_NEW_DOWNLOAD_EVENT, () => {
      openDownloadDialog();
    });
    return () => {
      offNewDownload();
    };
  }, [openDownloadDialog]);

  const openPetsGallery = React.useCallback((navigation?: Omit<PetsGalleryNavigation, "nonce">) => {
    openLibraryRoute("pet-gallery");
    setPetsGalleryNavigation({
      action: navigation?.action ?? "gallery",
      petId: navigation?.petId,
      nonce: Date.now(),
    });
  }, [openLibraryRoute]);

  const openDocumentation = React.useCallback((path = "") => {
    const url = buildDocumentationURL(settings?.language, path);
    void openExternalURL(url).catch((error) => {
      console.warn("[Main] open documentation unavailable", { url, error });
    });
  }, [settings?.language]);

  const openSettingsTab = React.useCallback(
    (tab: XiaSettingsTabId) => {
      setPendingSettingsTab(tab);
      void showSettingsWindow.mutateAsync().finally(() => {
        void Events.Emit("settings:navigate", tab);
      });
    },
    [showSettingsWindow],
  );

  React.useEffect(() => {
    const offNavigate = Events.On("pets:gallery:navigate", (event: any) => {
      const payload = event?.data ?? event;
      const record =
        payload && typeof payload === "object"
          ? (payload as Record<string, unknown>)
          : {};
      const action = record.action === "detail" ? record.action : "gallery";
      const petId = typeof record.petId === "string" ? record.petId.trim() : "";
      openPetsGallery({ action, petId });
    });
    return () => {
      offNavigate();
    };
  }, [openPetsGallery]);

  const handleRestartPreparedUpdate = React.useCallback(async () => {
    try {
      const next = await restartToApply.mutateAsync();
      setUpdateInfo(next);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      messageBus.publishToast({
        intent: "warning",
        title: text.about.restartAfterUpdate,
        description: message,
      });
    }
  }, [text.about.restartAfterUpdate, restartToApply, setUpdateInfo]);

  const userMenuUpdateItems = React.useMemo(
    () =>
      [
        hasAppUpdateMenu
          ? {
              key: "app-update",
              label: hasPreparedAppUpdate
                ? text.about.restartAfterUpdate
                : text.sidebar.appUpdate,
              meta: formatVersionBadge(displayUpdateVersion(updateInfo)),
              Icon: hasPreparedAppUpdate ? RefreshCcw : ArrowUpCircle,
              onSelect: () => {
                if (hasPreparedAppUpdate) {
                  void handleRestartPreparedUpdate();
                  return;
                }
                openSettingsTab("about");
              },
              disabled: restartToApply.isPending,
            }
          : null,
        hasDependencyUpdate
          ? {
              key: "dependency-update",
              label: text.sidebar.dependencyUpdate,
              meta: String(dependencyUpdateCount),
              Icon: Wrench,
              onSelect: () => openSettingsTab("download"),
              disabled: false,
            }
          : null,
      ].filter((item): item is NonNullable<typeof item> => Boolean(item)),
    [
      text.about.restartAfterUpdate,
      text.sidebar.appUpdate,
      text.sidebar.dependencyUpdate,
      dependencyUpdateCount,
      handleRestartPreparedUpdate,
      hasAppUpdateMenu,
      hasDependencyUpdate,
      hasPreparedAppUpdate,
      openSettingsTab,
      restartToApply.isPending,
      updateInfo,
    ],
  );

  const libraryAccessDisplayRemote = updateLibraryAccess.isPending
    ? updateLibraryAccess.variables?.remoteEnabled === true
    : libraryAccessRemote;
  const accountIdentityPanel = (
    <div className="app-workspace-account-menu__identity-panel">
      <div className="app-workspace-account-menu__identity">
        <UserAvatar
          profile={profile}
          tone="neutral"
          className="app-workspace-account-menu__identity-avatar h-9 w-9"
          shape="circle"
        />
        <div className="app-workspace-account-menu__identity-details">
          <div className="app-workspace-account-menu__identity-name">
            {resolveUserDisplayName(profile)}
          </div>
          <div className="app-workspace-account-menu__identity-id">
            {profile?.username?.trim() || text.sidebar.profileHint}
          </div>
        </div>
        <DropdownMenuItem
          className="app-workspace-account-menu__mobile-action"
          aria-label={libraryAccessCopy.mobileTitle}
          title={libraryAccessCopy.mobileTitle}
          onSelect={() => {
            setWorkspaceAccountMenuOpen(false);
            setMobilePairingSheetOpen(true);
          }}
        >
          <Smartphone className="h-4 w-4" />
        </DropdownMenuItem>
      </div>
    </div>
  );
  const libraryAccessControlRow = (
    <DropdownMenuCheckboxItem
      aria-busy={
        updateLibraryAccess.isPending || libraryAccessConfigQuery.isLoading
          ? true
          : undefined
      }
      aria-describedby="app-workspace-account-menu-access-status"
      aria-invalid={libraryAccessError ? true : undefined}
      checked={libraryAccessDisplayRemote}
      className="app-workspace-account-menu__access-row"
      disabled={updateLibraryAccess.isPending || libraryAccessConfigQuery.isLoading}
      onCheckedChange={(remoteEnabled) => {
        if (remoteEnabled !== libraryAccessRemote) {
          updateLibraryAccess.mutate({ remoteEnabled });
        }
      }}
      onSelect={(event) => event.preventDefault()}
      title={libraryAccessError || libraryAccessStatusLabel}
    >
      <span className="app-workspace-account-menu__access-name">
        {libraryAccessCopy.remote}
      </span>
      <DreamInlineSwitchVisual
        checked={libraryAccessDisplayRemote}
        className="app-workspace-account-menu__access-switch"
      />
      <span
        aria-live={libraryAccessError ? "assertive" : "polite"}
        className="sr-only"
        id="app-workspace-account-menu-access-status"
        role={libraryAccessError ? "alert" : "status"}
      >
        {libraryAccessError || libraryAccessStatusLabel}
      </span>
    </DropdownMenuCheckboxItem>
  );

  const resolveStationDisplayLabel = React.useCallback(
    (station: (typeof stations)[number]) =>
      station.workspaceId === APP_WORKSPACE_IDS.library
        ? text.workspace.libraryStation
        : station.workspaceId === APP_WORKSPACE_IDS.music &&
      station.label.toLowerCase() === "music"
        ? text.workspace.music
        : station.workspaceId === APP_WORKSPACE_IDS.sniff &&
            station.label.toLowerCase() === "sniff"
          ? text.workspace.sniff
          : station.workspaceId === APP_WORKSPACE_IDS.youtube &&
              station.label.toLowerCase() === "youtube"
            ? text.workspace.youtube
            : station.label,
    [text.workspace.libraryStation, text.workspace.music, text.workspace.sniff, text.workspace.youtube],
  );

  const showOperationStatus =
    operationActivity.hasActivity &&
    !(isLibraryWorkspace && activeWorkspaceRoute === "running");
  const hasExpandedActivity =
    (activeWorkspaceId !== APP_WORKSPACE_IDS.sniff &&
      sniffStatus.runtime !== "none") ||
    showOperationStatus ||
    (showWorkspacePlaybackActivity &&
      Boolean(globalPlaybackStatus) &&
      globalPlaybackStatus?.state !== "idle");
  const expandedActivityDock = hasExpandedActivity ? (
    <ActivityDock aria-label={activityLabels.nowPlaying}>
      <div className="flex flex-col gap-2">
        {showOperationStatus ? (
          <WideOperationActivity
            snapshot={operationActivity}
            labels={activityLabels}
            httpBaseURL={httpBaseURL}
            menuActions={operationActivityMenuActions}
            onOpen={() => openGlobalCompanion("operations")}
          />
        ) : null}
        {activeWorkspaceId !== APP_WORKSPACE_IDS.sniff ? (
          <WideSniffActivity
            status={sniffStatus}
            labels={activityLabels}
            menuActions={sniffActivityMenuActions}
            stopping={
              cancelResourceSniff.isPending || stopCDPBrowserRuntime.isPending
            }
            onOpen={() => openGlobalCompanion("sniff")}
            onStop={() => void stopSniffActivity()}
          />
        ) : null}
        <WidePlaybackActivity
          status={showWorkspacePlaybackActivity ? globalPlaybackStatus : null}
          labels={activityLabels}
          menuActions={playbackActivityMenuActions}
          onOpen={openActivePlaybackSurface}
          onCommand={sendGlobalPlaybackCommand}
        />
      </div>
    </ActivityDock>
  ) : null;
  const sniffWorkspaceControlPanel =
    activeWorkspaceId === APP_WORKSPACE_IDS.sniff &&
    sniffStatus.runtime !== "none" ? (
      <div className="flex flex-col gap-2">
        <SniffWorkspaceSessionActivity
          status={sniffStatus}
          labels={{
            sniff: text.workspace.sniff,
            session: text.sniffDesk.sessions,
            resources: text.sniffDesk.resources,
            downloadable: text.sniffDesk.downloadableOnly,
            status:
              sniffStatus.runtime === "orphan"
                ? activityLabels.sniffState.orphan
                : activityLabels.sniffState[sniffStatus.state],
            updated: text.sniffDesk.updatedAt,
          }}
        />
      </div>
    ) : undefined;

  const workspaceHeader = (
    <div className={cn("min-h-[28px]", isWindows && "wails-drag")} />
  );
  const workspaceSwitchStations = React.useMemo(() => {
    return resolveWorkspaceSwitchStations(stations);
  }, [stations]);
  const workspaceUpdateStatusLabel = userMenuUpdateItems
    .map((item) => item.meta ? `${item.label} ${item.meta}` : item.label)
    .join(", ");
  const workspaceAccountMenu = (
    <>
      <DropdownMenu open={workspaceAccountMenuOpen} onOpenChange={setWorkspaceAccountMenuOpen}>
        <DropdownMenuTrigger asChild>
          <AccountDockProfile
            aria-label={workspaceUpdateStatusLabel
              ? `${text.workspace.switchStation}: ${workspaceUpdateStatusLabel}`
              : text.workspace.switchStation}
            title={workspaceUpdateStatusLabel || text.workspace.switchStation}
            avatar={workspaceIdentityAvatar}
            disclosure={<ChevronsUpDown />}
            statusIndicator={userMenuUpdateItems.length > 0 ? (
              <span
                className="app-workspace-account-profile__update-dot"
                data-progress={updateInfo.status === "downloading" || updateInfo.status === "installing" ? "true" : undefined}
                data-ready={hasPreparedAppUpdate ? "true" : undefined}
              />
            ) : null}
            name={workspaceIdentityName}
            username={workspaceIdentityUsername}
          />
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side="top"
          align="start"
          sideOffset={8}
          className="app-workspace-account-menu"
        >
          {accountIdentityPanel}
          <DropdownMenuSeparator />
          <div className="grid">
            {workspaceSwitchStations.map((station) => {
              const canonicalLabel = resolveStationDisplayLabel(station);
              const iconKey = station.iconKey || station.workspaceId;
              const active = activeWorkspaceId === station.workspaceId;
              return (
                <DropdownMenuItem
                  aria-current={active ? "page" : undefined}
                  className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
                  key={`switch-${station.id}`}
                  onSelect={() =>
                    openWorkspace(
                      station.workspaceId,
                      station.defaultRouteId ||
                        (station.workspaceId === APP_WORKSPACE_IDS.sniff
                          ? "resources"
                          : "home"),
                    )
                  }
                >
                  {iconKey === "library" ? (
                    <LibraryBig className="h-4 w-4 shrink-0" />
                  ) : iconKey === "sniff" ? (
                    <Radar className="h-4 w-4 shrink-0" />
                  ) : iconKey === "youtube" ? (
                    <Youtube className="h-4 w-4 shrink-0" />
                  ) : iconKey === "rss" ? (
                    <Rss className="h-4 w-4 shrink-0" />
                  ) : (
                    <Music2 className="h-4 w-4 shrink-0" />
                  )}
                  <span className="min-w-0 flex-1 truncate">
                    {canonicalLabel}
                  </span>
                  {station.workspaceId === APP_WORKSPACE_IDS.rss ? (
                    <StatusBadge
                      className="shrink-0"
                      data-menu-indicator="true"
                      tone="accent"
                    >
                      {t("xiadown.rss.preview")}
                    </StatusBadge>
                  ) : null}
                  {active ? (
                    <Check
                      className="h-3.5 w-3.5 shrink-0"
                      data-menu-indicator="true"
                    />
                  ) : null}
                </DropdownMenuItem>
              );
            })}
          </div>
          <DropdownMenuSeparator />
          {libraryAccessControlRow}
          <DropdownMenuSeparator />
          <div className="app-workspace-account-menu__quick-actions">
            <DropdownMenuItem
              aria-label={text.actions.settings}
              className="app-workspace-account-menu__quick-action"
              onSelect={() => showSettingsWindow.mutate()}
              title={text.actions.settings}
            >
              <Settings2 />
            </DropdownMenuItem>
            <DropdownMenuItem
              aria-label={text.sidebar.documentation}
              className="app-workspace-account-menu__quick-action"
              onSelect={() => openDocumentation()}
              title={text.sidebar.documentation}
            >
              <FileText />
            </DropdownMenuItem>
            {userMenuUpdateItems.map((item) => {
              const UpdateIcon = item.Icon;
              const label = item.meta ? `${item.label} ${item.meta}` : item.label;
              return (
                <DropdownMenuItem
                  aria-label={label}
                  className="app-workspace-account-menu__quick-action app-workspace-account-menu__quick-action--update"
                  disabled={item.disabled}
                  key={item.key}
                  onSelect={item.onSelect}
                  title={label}
                >
                  <UpdateIcon />
                </DropdownMenuItem>
              );
            })}
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
      <LibraryPairingSheet
        language={settings?.language}
        open={mobilePairingSheetOpen}
        onOpenChange={setMobilePairingSheetOpen}
      />
    </>
  );
  const workspaceNewAction = (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          aria-label={text.sidebar.newTask}
          className="app-workspace-account-profile__action"
          title={text.sidebar.newTask}
        >
          <Plus />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="top"
        align="center"
        sideOffset={8}
        className={SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME}
      >
        <DropdownMenuItem
          className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
          onSelect={() => openNewTaskDialog("download")}
        >
          <Download className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{text.sidebar.newDownload}</span>
        </DropdownMenuItem>
        <DropdownMenuItem
          className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
          onSelect={() => openNewTaskDialog("transcode")}
        >
          <FileCog className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{text.sidebar.newTranscode}</span>
        </DropdownMenuItem>
        <DropdownMenuItem
          className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
          onSelect={() => openNewTaskDialog("sniff")}
        >
          <Radar className="h-4 w-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate">{text.sidebar.newSniff}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
  const musicSourceSwitcher = (
    <DreamSegmentSwitch
      ariaLabel={`${text.workspace.online} / ${text.workspace.local}`}
      className="app-music-workspace-source-switcher w-full"
      items={[
        {
          value: "online",
          label: text.workspace.online,
          icon: <Globe2 className="h-3.5 w-3.5 shrink-0" />,
        },
        {
          value: "local",
          label: text.workspace.local,
          icon: <HardDrive className="h-3.5 w-3.5 shrink-0" />,
        },
      ]}
      onValueChange={(scope) =>
        navigateWorkspace({
          routeId: resolveMusicWorkspaceScopeRoute(
            scope,
            lastMusicRouteByScopeRef.current[scope],
          ),
        })
      }
      value={musicWorkspaceScope}
    />
  );
  const expandedAccountDock = (
    <AccountDock aria-label={text.workspace.navigation}>
      <div className="app-workspace-account-row">
        {workspaceAccountMenu}
        <div className="app-workspace-account-profile__actions">
          {workspaceNewAction}
        </div>
      </div>
    </AccountDock>
  );
  const workspaceSidebarClassName = cn(
    "app-main-sidebar",
    resolveSidebarSurface(theme.id, appearance.surfaceStyle, shellTheme),
  );
  const workspaceSidebarChrome = {
    activeRouteId: activeWorkspaceRoute,
    header: workspaceHeader,
    controlPanel:
      activeWorkspaceId === APP_WORKSPACE_IDS.music
        ? musicSourceSwitcher
        : activeWorkspaceId === APP_WORKSPACE_IDS.sniff
          ? sniffWorkspaceControlPanel || undefined
          : undefined,
    activity: expandedActivityDock,
    account: expandedAccountDock,
    className: workspaceSidebarClassName,
    onNavigate: (routeId: string) => navigateWorkspace({ routeId }),
  };
  const forceRefreshWorkspaceRoute = React.useCallback(async () => {
    const target = workspaceRouteContextMenu;
    if (!target) {
      return;
    }
    setWorkspaceRouteContextMenu(null);
    navigateWorkspace({ routeId: target.routeId }, target.workspaceId);
    if (target.workspaceId === APP_WORKSPACE_IDS.music) {
      setListenNowPlaying(null);
    } else {
      setYouTubePlayback(null);
    }
    try {
      if (target.workspaceId === APP_WORKSPACE_IDS.music) {
        await forceRefreshListenOnline();
      } else {
        await forceRefreshYouTubeWorkspace();
      }
    } catch (error) {
      messageBus.publishToast({
        intent: "warning",
        title: text.actions.forceRefresh,
        description: error instanceof Error ? error.message : String(error),
      });
    } finally {
      if (target.workspaceId === APP_WORKSPACE_IDS.music) {
        setMusicForceRefreshToken((value) => value + 1);
      } else {
        setYouTubeForceRefreshToken((value) => value + 1);
      }
    }
  }, [
    navigateWorkspace,
    text.actions.forceRefresh,
    workspaceRouteContextMenu,
  ]);
  const workspaceNavigation =
    activeWorkspaceId === APP_WORKSPACE_IDS.library ? (
      <LibraryWorkspaceSidebar
        {...workspaceSidebarChrome}
        catalog={{
          sidebarAriaLabel: text.workspace.libraryStation,
          sections: {
            library: { label: text.workspace.libraryStation },
            more: { label: text.workspace.more },
          },
          routes: {
            search: { icon: <Search />, label: text.workspace.search },
            running: { icon: <Activity />, label: text.views.running },
            ended: { icon: <History />, label: text.views.ended },
            appSessions: { icon: <Link2 />, label: text.views.connections },
            all: { icon: <LibraryBig />, label: text.workspace.all },
            video: { icon: <Video />, label: text.workspace.video },
            audio: { icon: <Music2 />, label: text.workspace.audio },
            books: { icon: <BookOpen />, label: text.workspace.books },
            images: { icon: <Images />, label: text.workspace.images },
            others: { icon: <PackageOpen />, label: text.workspace.others },
            petGallery: { icon: <PawPrint />, label: text.petGallery.title },
          },
        }}
      />
    ) : activeWorkspaceId === APP_WORKSPACE_IDS.music ? (
      <MusicWorkspaceSidebar
        {...workspaceSidebarChrome}
        scope={musicWorkspaceScope}
        onRouteContextMenu={
          musicWorkspaceScope === "online"
            ? (routeId, point, returnFocus) => {
                workspaceRouteContextReturnFocusRef.current = returnFocus;
                setWorkspaceRouteContextMenu({
                  workspaceId: APP_WORKSPACE_IDS.music,
                  routeId,
                  ...point,
                });
              }
            : undefined
        }
        catalog={{
          sidebarAriaLabel: text.workspace.music,
          sections: {
            explore: { label: text.workspace.explore },
            library: { label: text.workspace.library },
            playlists: { label: text.workspace.playlists },
          },
          routes: {
            search: { icon: <Search />, label: text.workspace.search },
            home: { icon: <House />, label: text.workspace.home },
            radio: { icon: <Radio />, label: text.workspace.radio },
            newReleases: {
              icon: <Sparkles />,
              label: text.workspace.newReleases,
            },
            charts: {
              icon: <ChartNoAxesColumnIncreasing />,
              label: text.workspace.charts,
            },
            moods: { icon: <Shapes />, label: text.workspace.moods },
            podcasts: { icon: <Podcast />, label: text.workspace.podcasts },
            recent: { icon: <Clock3 />, label: text.workspace.recent },
            history: { icon: <History />, label: text.workspace.history },
            onlinePlaylists: {
              icon: <ListMusic />,
              label: text.workspace.playlists,
            },
            localSearch: { icon: <Search />, label: text.workspace.search },
            localHome: { icon: <House />, label: text.workspace.home },
            recentlyAdded: {
              icon: <Clock3 />,
              label: text.workspace.recentlyAdded,
            },
            artists: { icon: <UsersRound />, label: text.workspace.artists },
            albums: { icon: <Disc3 />, label: text.workspace.albums },
            songs: { icon: <Music2 />, label: text.workspace.songs },
          },
        }}
      />
    ) : activeWorkspaceId === APP_WORKSPACE_IDS.sniff ? (
      <SniffWorkspaceSidebar
        {...workspaceSidebarChrome}
        filtersVisible={sniffWorkspaceFiltersVisible}
        pet={activePet}
        petImageURL={activePetImageURL}
        waitingLabel={text.sniffDesk.waitingSniff}
        catalog={{
          sidebarAriaLabel: text.workspace.sniff,
          sections: {
            types: { label: text.workspace.types },
            sources: { label: text.workspace.sources },
            resources: { label: text.workspace.resources },
          },
        }}
        searchControl={
          <SniffWorkspaceSearchField text={text} />
        }
        typesFilter={<SniffWorkspaceKindSelect text={text} />}
        sourcesFilter={<SniffWorkspaceSourceSelect text={text} />}
        resourcesFilter={<SniffWorkspaceResourceSelect text={text} />}
      />
    ) : activeWorkspaceId === APP_WORKSPACE_IDS.rss ? (
      <RSSWorkspaceSidebar
        {...workspaceSidebarChrome}
        subscriptions={rssSubscriptionsQuery.data ?? []}
        categories={rssCategoriesQuery.data ?? []}
        collections={rssCollectionsQuery.data ?? []}
        collectionUnreadCounts={rssCollectionUnreadCounts.data}
        markAllReadPending={rssMarkAllRead.isPending}
        onEditSubscription={(subscription, returnFocusTarget) => {
          setRSSEditingSubscription({ subscription, returnFocusTarget });
        }}
        onMarkSubscriptionRead={(subscription) => {
          void rssMarkAllRead
            .mutateAsync({ subscriptionId: subscription.id })
            .catch((error) => publishRSSActionFailure(
              t("xiadown.rss.markAllRead"),
              error,
            ));
        }}
        onUnsubscribe={(subscription, returnFocusTarget) => {
          rssUnsubscribeReturnFocusTargetRef.current = returnFocusTarget;
          setRSSPendingUnsubscribe(subscription);
        }}
        catalog={{
          sidebarAriaLabel: "RSS",
          unreadLabel: t("xiadown.rss.unread"),
          sections: {
            collections: { label: t("xiadown.rss.lists") },
            discovery: { label: t("xiadown.rss.discovery") },
            subscriptions: { label: text.workspace.subscriptions },
          },
          routes: {
            search: { icon: <Search />, label: text.workspace.search },
            all: { icon: <Rss />, label: text.workspace.all },
            articles: { icon: <FileText />, label: t("xiadown.rss.articles") },
            social: { icon: <UsersRound />, label: t("xiadown.rss.socialMedia") },
            images: { icon: <Images />, label: text.workspace.images },
            videos: { icon: <Video />, label: t("xiadown.rss.videos") },
            starred: { icon: <Star />, label: t("xiadown.rss.favorites") },
            manageSubscriptions: { icon: <Settings2 />, label: t("xiadown.rss.manager") },
            discoveryBrowse: { icon: <Compass />, label: t("xiadown.rss.discoveryBrowse") },
          },
        }}
      />
    ) : (
      <YouTubeWorkspaceSidebar
        {...workspaceSidebarChrome}
        onRouteContextMenu={(routeId, point, returnFocus) => {
          workspaceRouteContextReturnFocusRef.current = returnFocus;
          setWorkspaceRouteContextMenu({
            workspaceId: APP_WORKSPACE_IDS.youtube,
            routeId,
            ...point,
          });
        }}
        catalog={{
          sidebarAriaLabel: text.workspace.youtube,
          sections: {
            discover: { label: text.workspace.discover },
            collections: { label: text.workspace.collections },
          },
          routes: {
            search: { icon: <Search />, label: text.workspace.search },
            home: { icon: <House />, label: text.workspace.home },
            subscriptions: {
              icon: <Bell />,
              label: text.workspace.subscriptions,
            },
            explore: { icon: <Compass />, label: text.workspace.explore },
            shorts: { icon: <Clapperboard />, label: text.workspace.shorts },
            likedVideos: {
              icon: <ThumbsUp />,
              label: text.workspace.likedVideos,
            },
            watchLater: {
              icon: <Clock3 />,
              label: text.workspace.watchLater,
            },
            playlists: {
              icon: <ListVideo />,
              label: text.workspace.playlists,
            },
            history: { icon: <History />, label: text.workspace.history },
          },
        }}
      />
    );

  const companionTitle =
    fullscreenPlayer === "music"
      ? activityLabels.nowPlaying
      : companion.destination?.id === "youtube-up-next"
        ? text.listen.upNext
        : companion.destination?.id === "library-preview"
          ? t("xiadown.libraryCatalog.preview")
        : companion.destination?.id === "library-deleted"
          ? t("xiadown.libraryCatalog.deletedItemsTitle")
        : companion.destination?.id === "operations"
          ? activityLabels.operations
          : companion.destination?.id === "sniff"
            ? activityLabels.sniff
            : companion.destination?.id === "lyrics"
              ? text.listen.lyrics
              : companion.destination?.id === "queue"
                ? text.listen.upNext
                : activityLabels.nowPlaying;
  const listenSourceURL = listenNowPlaying?.sourceURL;
  const companionFooter =
    !playerFullscreen &&
    companion.destination?.id === "library-preview" &&
    activeLibraryPreviewItem ? (
      <LibraryPreviewCompanionFooter
        item={activeLibraryPreviewItem}
        activeTab={libraryPreviewTab}
        onActiveTabChange={setLibraryPreviewTab}
      />
    ) : !playerFullscreen &&
    companion.destination?.id === "sniff" &&
    sniffStatus.runtime !== "none" ? (
      <SniffCompanionFooter
        status={sniffStatus}
        labels={activityLabels}
        clearing={clearResourceSniffResources.isPending}
        stopping={
          cancelResourceSniff.isPending || stopCDPBrowserRuntime.isPending
        }
        onClear={() => void clearSniffActivity()}
        onStop={() => void stopSniffActivity()}
      />
    ) : !playerFullscreen &&
      (companion.destination?.id === "playback-summary" ||
        companion.destination?.id === "youtube-up-next") &&
      globalPlaybackStatus &&
      globalPlaybackStatus.state !== "idle" ? (
      <PlayerCompanionFooter
        status={globalPlaybackStatus}
        labels={activityLabels}
        onCommand={sendGlobalPlaybackCommand}
      />
    ) : undefined;
  const primaryWindowsChromeVisible =
    isWindows && !companion.open && !playerFullscreen;
  const primaryWindowsDragRailVisible =
    primaryWindowsChromeVisible &&
    activeWorkspaceId !== APP_WORKSPACE_IDS.music;

  return (
    <div
      data-shell-theme={shellTheme}
      data-surface-style={appearance.surfaceStyle}
      data-window-material={windowMaterial}
      className={cn(
        "app-main-shell relative flex h-screen overflow-hidden",
        "app-dream-frame app-dream-window",
      )}
    >
      <AppShell
        ambientArtworkURL={
          activeWorkspaceId === APP_WORKSPACE_IDS.music && !playerFullscreen
            ? listenNowPlaying?.artworkURL.trim()
            : undefined
        }
        data-player-fullscreen={playerFullscreen || undefined}
        navigation={workspaceNavigation}
        primaryMinWidth={workspacePrimaryMinWidth}
        surfaceStyle={appearance.surfaceStyle}
        companionOpen={companionAffectsLayout}
        companionPresentation={companionPresentation}
        onMinimumWidthChange={fitMinimumWidth}
        className="app-main-shell-layout min-w-0 flex-1"
      >
      <WorkspaceStage
        primaryMinWidth={workspacePrimaryMinWidth}
        companionOpen={companionAffectsLayout}
        companionPresentation={companionPresentation}
        className={cn(
          playerFullscreen && "app-workspace-stage--player-fullscreen",
        )}
      >
      <PrimaryPane
        minimumWidth={workspacePrimaryMinWidth}
        className="app-main-content relative flex min-w-0 flex-1 flex-col"
      >
        {primaryWindowsDragRailVisible ? (
          <div
            className="wails-drag absolute left-0 right-[var(--app-windows-caption-control-width)] top-0 z-30 h-[var(--app-windows-caption-button-height)]"
            aria-hidden="true"
          />
        ) : null}
        <div
          className={cn(
            "app-main-view-viewport min-h-0 flex-1",
            (isLibraryWorkspace &&
              ["running", "ended", "app-sessions", "all", "video", "audio", "books", "images", "others", "search", "pet-gallery"].includes(activeWorkspaceRoute)) ||
              activeWorkspaceId === APP_WORKSPACE_IDS.music ||
              activeWorkspaceId === APP_WORKSPACE_IDS.sniff ||
              activeWorkspaceId === APP_WORKSPACE_IDS.youtube ||
              activeWorkspaceId === APP_WORKSPACE_IDS.rss
              ? "flex overflow-hidden"
              : "overflow-auto px-5 pb-5",
          )}
        >
          <React.Suspense
            fallback={(
              <div className="flex h-full w-full items-center justify-center" role="status">
                <LoaderCircle className="app-motion-spin h-5 w-5" aria-hidden="true" />
              </div>
            )}
          >
          {isLibraryWorkspace && activeWorkspaceRoute === "running" ? (
            <RunningPage
              text={text}
              operations={runningOperations}
              filesById={filesById}
              httpBaseURL={httpBaseURL}
              pet={activePet}
              petImageURL={activePetImageURL}
              petAnimation={runningPetAnimation}
              loading={
                runningOperations.length === 0 &&
                !runningQuery.isFetched &&
                runningQuery.isFetching
              }
              reserveWindowControls={primaryWindowsChromeVisible}
              onNewDownload={() => openDownloadDialog()}
            />
          ) : isLibraryWorkspace && ["ended", "all", "video", "audio", "books", "images", "others", "search"].includes(activeWorkspaceRoute) ? (
            <LibraryWorkspacePage
              route={activeWorkspaceRoute as LibraryWorkspaceRoute}
              items={visibleLibraryWorkspaceItems}
              query={libraryBrowseQuery}
              sort={libraryBrowseSort}
              otherGroup={libraryOtherGroup}
              pagination={{
                page: libraryPage,
                pageSize: libraryPageSize,
                total: serverLibraryPagination
                  ? libraryCatalogPageQuery.data?.total ?? 0
                  : undefined,
                itemsArePage: serverLibraryPagination,
                onPageChange: setLibraryPage,
                onPageSizeChange: setLibraryPageSize,
              }}
              reserveWindowControls={primaryWindowsChromeVisible}
              loading={
                serverLibraryPagination
                  ? libraryCatalogPageQuery.isFetching && (
                      libraryCatalogPageQuery.data === undefined ||
                      libraryCatalogPageQuery.isPlaceholderData
                    )
                  : activeWorkspaceRoute === "ended"
                    ? endedOperationsQuery.data === undefined && endedOperationsQuery.isFetching
                    : catalogItemsQuery.data === undefined && catalogItemsQuery.isFetching
              }
              loadError={
                serverLibraryPagination
                  ? libraryCatalogPageQuery.data === undefined && libraryCatalogPageQuery.isError
                  : activeWorkspaceRoute === "ended"
                    ? endedOperationsQuery.data === undefined && endedOperationsQuery.isError
                    : catalogItemsQuery.data === undefined && catalogItemsQuery.isError
              }
              onRetry={() => void (
                serverLibraryPagination
                  ? libraryCatalogPageQuery.refetch()
                  : activeWorkspaceRoute === "ended"
                    ? endedOperationsQuery.refetch()
                    : catalogItemsQuery.refetch()
              )}
              onQueryChange={setLibraryBrowseQuery}
              onSortChange={setLibraryBrowseSort}
              onOtherGroupChange={setLibraryOtherGroup}
              selectedItemId={activeLibraryPreviewItem?.id}
              labels={{
                search: text.workspace.search,
                ended: text.views.ended,
                all: text.workspace.all,
                tasks: text.views.tasks,
                video: text.workspace.video,
                audio: text.workspace.audio,
                books: text.workspace.books,
                images: text.workspace.images,
                others: text.workspace.others,
                library: text.workspace.libraryStation,
              }}
              onItemClick={openLibraryPreview}
              onOpenDeletedItems={openLibraryDeletedItems}
            />
          ) : isLibraryWorkspace && activeWorkspaceRoute === "app-sessions" ? (
            <AppSessionsSection
              reserveWindowControls={primaryWindowsChromeVisible}
            />
          ) : isLibraryWorkspace && activeWorkspaceRoute === "pet-gallery" ? (
            <PetsGalleryPage
              text={text}
              settings={settings}
              navigation={petsGalleryNavigation}
              reserveWindowControls={primaryWindowsChromeVisible}
              onOpenDocumentation={openDocumentation}
            />
          ) : activeWorkspaceId === APP_WORKSPACE_IDS.sniff ? (
            <SniffDeskPage
              text={text}
              active={activeWorkspaceId === APP_WORKSPACE_IDS.sniff}
              httpBaseURL={httpBaseURL}
              workspaceLayout
              workspaceRouteId={activeWorkspaceRoute}
              reserveWindowControls={primaryWindowsChromeVisible}
              onStartSniff={() => openNewTaskDialog("sniff")}
            />
          ) : null}
          {shouldMountRSSWorkspace ? <RSSWorkspacePage
            active={activeWorkspaceId === APP_WORKSPACE_IDS.rss}
            categories={rssCategoriesQuery.data ?? []}
            collections={rssCollectionsQuery.data ?? []}
            routeId={workspaceLocations[APP_WORKSPACE_IDS.rss]?.routeId ?? "all"}
            subscriptions={rssSubscriptionsQuery.data ?? []}
            reserveWindowControls={primaryWindowsChromeVisible}
            onNavigate={(routeId) => navigateWorkspace({ routeId })}
            onDownload={openRSSDownloadDialog}
          /> : null}
          {shouldMountYouTubeWorkspace ? <YouTubeWorkspacePage
            key={`youtube-workspace:${youtubeForceRefreshToken}`}
            active={activeWorkspaceId === APP_WORKSPACE_IDS.youtube}
            externalCommand={youtubeControlCommand}
            routeId={workspaceLocations[APP_WORKSPACE_IDS.youtube]?.routeId ?? "home"}
            text={text}
            watchOpen={youtubeWatchOpen}
            revealWatchRequest={youtubeWatchRevealRequest}
            onWatchOpenChange={setYouTubeWatchSurfaceOpen}
            onWatchSurfaceVisibleChange={setYouTubeWatchSurfaceVisible}
            upNextPortalTarget={youtubeUpNextPortalTarget}
            upNextOpen={youtubeUpNextOpen}
            onToggleUpNext={toggleYouTubeUpNext}
            onDownload={openDownloadDialog}
            initialPlayback={youtubePlaybackForPage}
            onPlaybackChange={setYouTubePlayback}
            reserveWindowControls={primaryWindowsChromeVisible}
          /> : null}
          {shouldMountMusicWorkspace ? <ListenPage
            key={`music-workspace:${musicForceRefreshToken}`}
            active={activeWorkspaceId === APP_WORKSPACE_IDS.music}
            workspaceLayout
            workspaceRouteId={musicWorkspaceRouteId}
            reserveWindowControls={primaryWindowsChromeVisible}
            playerPortalTarget={listenPlayerPortalTarget}
            playerFullscreen={fullscreenPlayer === "music"}
            playerPresentation={
              fullscreenPlayer === "music" ? "fullscreen" : "companion"
            }
            playerCompanionMode={
              fullscreenPlayer === "music"
                ? "player"
                : companion.destination?.id === "lyrics"
                  ? "lyrics"
                  : companion.destination?.id === "queue"
                    ? "queue"
                    : "player"
            }
            playerSurfaceVisible={
              fullscreenPlayer === "music" ||
              (!playerFullscreen && playbackCompanionOpen)
            }
            text={text}
            libraries={libraries}
            httpBaseURL={httpBaseURL}
            pet={activePet}
            petImageURL={activePetImageURL}
            controlCommand={listenControlCommand}
            onNowPlayingChange={setListenNowPlaying}
            onOpenPlaybackSource={openMusicPlaybackSource}
            onRequestPlayerFullscreen={requestMusicPlayerFullscreen}
            onExitPlayerFullscreen={exitMusicPlayerFullscreen}
            onOpenConnections={() => openLibraryRoute("app-sessions")}
            onDownloadTrack={openDownloadDialog}
          /> : null}
          <MainStartupSurfaceReady />
          </React.Suspense>
        </div>
        {activeWorkspaceId === APP_WORKSPACE_IDS.music &&
        shouldPresentMusicWorkspaceTransport(coordinatorPlaybackSession) &&
        !playerFullscreen &&
        !welcomeOpen &&
        !newTaskDialogOpen ? (
          <MusicWorkspaceTransportBar
            status={globalPlaybackStatus}
            labels={{
              idleStatus: text.listen.idleStatus,
              idleSubtitle: text.listen.idleSubtitle,
              shuffle: text.listen.playModeShuffle,
              previous: text.listen.previous,
              play: text.listen.play,
              pause: text.listen.pause,
              next: text.listen.next,
              repeatOne: text.listen.playModeRepeat,
              live: text.listen.liveBadge,
              lyrics: text.listen.lyrics,
              upNext: text.listen.upNext,
              volume: text.listen.volume,
              fullscreen: text.completed.previewEnterFullscreen,
              more: text.listen.more,
              favorite: text.listen.favorite,
              download: text.actions.download,
              openURL: text.listen.openPage,
            }}
            onCommand={sendMusicWorkspaceTransportCommand}
            onOpenArtist={
              listenNowPlaying?.playbackSource === "youtube_music" &&
              listenNowPlaying.live !== true
                ? openCurrentMusicArtist
                : undefined
            }
            onOpenPlayer={() => openGlobalCompanion("player")}
            onOpenLyrics={() => requestMusicCompanionMode("lyrics")}
            onOpenQueue={() => requestMusicCompanionMode("queue")}
            onDownload={
              listenSourceURL
                ? () => openDownloadDialog(listenSourceURL)
                : undefined
            }
            onFavorite={
              listenNowPlaying?.canFavorite
                ? () => sendListenCommand("favorite")
                : undefined
            }
            onOpenURL={
              listenSourceURL
                ? () => void openExternalURL(listenSourceURL)
                : undefined
            }
            onFullscreen={() => {
              requestMusicPlayerFullscreen();
            }}
            volume={listenNowPlaying?.volume}
            muted={listenNowPlaying?.muted}
            onVolumeChange={(value) => sendListenCommand("set-volume", value)}
            onToggleMute={() => sendListenCommand("toggle-mute")}
          />
        ) : null}
      </PrimaryPane>
      <CompanionPanel
        open={companion.open || playerFullscreen}
        destination={companion.destination}
        presentation={companionPresentation}
        scrollChrome={
          playerFullscreen ||
          companion.destination?.id === "player" ||
          companion.destination?.id === "lyrics"
            ? "off"
            : "auto"
        }
        aria-label={companionTitle}
        className={cn(
          playerFullscreen &&
            "app-workspace-companion--player-fullscreen",
        )}
        data-platform={isWindows ? "windows" : "macos"}
        role={playerFullscreen ? "dialog" : "complementary"}
        aria-modal={playerFullscreen || undefined}
        footer={companionFooter}
        header={
          <div
            className="app-workspace-companion__titlebar app-workspace-companion__titlebar--controls-only"
          >
            <Button
              ref={playerFullscreen ? fullscreenCloseButtonRef : undefined}
              type="button"
              variant={playerFullscreen ? "secondary" : "ghost"}
              size="icon"
              className={cn(
                "wails-no-drag",
                playerFullscreen &&
                  "app-workspace-companion__fullscreen-close h-8 w-8",
              )}
              aria-label={text.actions.close}
              onClick={() => {
                if (playerFullscreen) {
                  setFullscreenPlayer(null);
                  return;
                }
                closeCompanion();
              }}
            >
              <X className="h-4 w-4" />
            </Button>
            <div className="app-workspace-companion__title">
              {companionTitle}
            </div>
            <span
              className="app-workspace-companion__titlebar-spacer"
              aria-hidden="true"
            />
            {isWindows && (companion.open || playerFullscreen) ? (
              <WindowControls
                platform="windows"
                owner={playerFullscreen ? "fullscreen" : "companion"}
              />
            ) : null}
          </div>
        }
      >
        <div
          ref={setListenPlayerPortalTarget}
          className={cn(
            "h-full min-h-0 w-full flex-col",
            fullscreenPlayer === "music" ||
              (!playerFullscreen && playbackCompanionOpen)
              ? "flex"
              : "hidden",
          )}
        />
        <div
          ref={setYouTubeUpNextPortalTarget}
          className={cn(
            "h-full min-h-0 w-full",
            !playerFullscreen && youtubeUpNextOpen ? "block" : "hidden",
          )}
        />
        {!playerFullscreen && companion.destination?.id === "operations" ? (
          <OperationsCompanionView
            snapshot={operationActivity}
            labels={activityLabels}
            httpBaseURL={httpBaseURL}
          />
        ) : !playerFullscreen && companion.destination?.id === "library-deleted" ? (
          <LibraryDeletedCompanion />
        ) : !playerFullscreen && companion.destination?.id === "library-preview" ? (
          <LibraryPreviewCompanion
            item={activeLibraryPreviewItem}
            httpBaseURL={httpBaseURL}
            activeTab={libraryPreviewTab}
            onActiveTabChange={setLibraryPreviewTab}
            onOpenItem={openLibraryPreviewById}
            tabsPlacement="external"
          />
        ) : !playerFullscreen && companion.destination?.id === "sniff" ? (
          <SniffCompanionView
            status={sniffStatus}
            labels={activityLabels}
            clearing={clearResourceSniffResources.isPending}
            stopping={
              cancelResourceSniff.isPending || stopCDPBrowserRuntime.isPending
            }
          />
        ) : !playerFullscreen &&
          companion.destination?.id === "playback-summary" ? (
          <PlayerCompanionView
            status={globalPlaybackStatus}
            labels={activityLabels}
          />
        ) : null}
      </CompanionPanel>
      </WorkspaceStage>
      </AppShell>

      {primaryWindowsChromeVisible ? (
        <div className="fixed right-0 top-0 z-[var(--app-layer-window-controls)]">
          <WindowControls platform="windows" owner="primary" />
        </div>
      ) : null}

      {welcomeOpen ? (
        <WelcomeScreen
          open={welcomeOpen}
          settings={settings}
          onComplete={() => {
            setSetupState({ completed: true });
            setDebugWelcomeOpen(false);
          }}
        />
      ) : null}
      <WhatsNewFeatureDialog
        blocked={welcomeOpen || rssPreviewNoticeOpen}
        language={settings?.language}
      />
      <Dialog
        open={rssPreviewNoticeOpen && !welcomeOpen}
        onOpenChange={(open) => {
          if (!open) closeRSSPreviewNotice();
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("xiadown.rss.previewNoticeTitle")}</DialogTitle>
            <DialogDescription>
              {t("xiadown.rss.previewNoticeDescription")}
            </DialogDescription>
          </DialogHeader>
          <DialogRow>
            <label htmlFor="rss-preview-notice-dismissed">
              {t("xiadown.rss.previewNoticeDontShowAgain")}
            </label>
            <Checkbox
              checked={rssPreviewNoticeDontShowAgain}
              id="rss-preview-notice-dismissed"
              onChange={(event) =>
                setRSSPreviewNoticeDontShowAgain(event.currentTarget.checked)
              }
            />
          </DialogRow>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button">{t("common.close")}</Button>
            </DialogClose>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      {newTaskDialogOpen ? (
        <React.Suspense fallback={null}>
          <NewTaskDialog
            open
            onOpenChange={setNewTaskDialogOpen}
            initialMode={newTaskDialogMode}
            initialUrl={prefilledDownloadURL}
            initialDownloadSource={prefilledDownloadOrigin?.source}
            initialDownloadCaller={prefilledDownloadOrigin?.caller}
            initialDownloadTargets={prefilledDownloadTargets}
            initialTranscodeSource={prefilledTranscodeSource}
            transcodeLibrarySources={transcodeLibrarySources}
            transcodeLibraryLoading={transcodeCatalogItemsQuery.isFetching}
            transcodeLibraryError={
              transcodeCatalogItemsQuery.isError
                ? t("xiadown.libraryCatalog.loadFailed")
                : ""
            }
            onRetryTranscodeLibrary={() => void transcodeCatalogItemsQuery.refetch()}
            settings={settings}
            onOpenConnections={() => openLibraryRoute("app-sessions")}
            onOpenSniffDesk={() => openWorkspace(APP_WORKSPACE_IDS.sniff, "resources")}
          />
        </React.Suspense>
      ) : null}
      <DropdownMenu
        modal={false}
        open={workspaceRouteContextMenu !== null}
        onOpenChange={(open) => {
          if (!open) {
            setWorkspaceRouteContextMenu(null);
          }
        }}
      >
        {workspaceRouteContextMenu ? (
          <DropdownMenuTrigger asChild>
            <button
              aria-hidden="true"
              className="fixed z-50 h-px w-px"
              style={{
                left: workspaceRouteContextMenu.x,
                top: workspaceRouteContextMenu.y,
              }}
              tabIndex={-1}
              type="button"
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          align="start"
          aria-label={text.actions.forceRefresh}
          className={SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME}
          side="bottom"
          sideOffset={2}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            const target = workspaceRouteContextReturnFocusRef.current;
            workspaceRouteContextReturnFocusRef.current = null;
            if (target?.isConnected) {
              target.focus();
            }
          }}
        >
          <DropdownMenuItem
            className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
            onSelect={() => void forceRefreshWorkspaceRoute()}
          >
            <RefreshCcw className="h-4 w-4 shrink-0" />
            <span>{text.actions.forceRefresh}</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {rssEditingSubscription ? (
        <RSSSubscriptionDialog
          categories={rssCategoriesQuery.data ?? []}
          subscriptions={rssSubscriptionsQuery.data ?? []}
          returnFocusTarget={rssEditingSubscription.returnFocusTarget}
          target={{
            kind: "edit",
            subscription: rssEditingSubscription.subscription,
          }}
          onClose={() => setRSSEditingSubscription(null)}
        />
      ) : null}
      <Dialog
        open={Boolean(rssPendingUnsubscribe)}
        onOpenChange={(open) => {
          if (!open && !rssDeleteSubscription.isPending) {
            setRSSPendingUnsubscribe(null);
          }
        }}
      >
        <DialogContent
          className="rss-confirm-dialog"
          onCloseAutoFocus={(event) => {
            const preferredTarget = rssUnsubscribeReturnFocusTargetRef.current;
            rssUnsubscribeReturnFocusTargetRef.current = null;
            const target = preferredTarget?.isConnected
              ? preferredTarget
              : document.querySelector<HTMLButtonElement>(
                  'button[data-route-id="all"]',
                );
            if (!target) return;
            event.preventDefault();
            target.focus();
          }}
          showCloseButton={false}
        >
          <DialogTitle>{t("xiadown.rss.unsubscribe")}</DialogTitle>
          <DialogDescription className="rss-confirm-dialog__description">
            <span>{t("xiadown.rss.deleteConfirm")}</span>
            {rssPendingUnsubscribe ? (
              <strong>
                {rssPendingUnsubscribe.title || rssPendingUnsubscribe.feedUrl}
              </strong>
            ) : null}
          </DialogDescription>
          <div className="rss-confirm-dialog__actions">
            <DialogClose asChild>
              <Button
                disabled={rssDeleteSubscription.isPending}
                type="button"
                variant="outline"
              >
                {t("xiadown.rss.cancel")}
              </Button>
            </DialogClose>
            <Button
              disabled={!rssPendingUnsubscribe || rssDeleteSubscription.isPending}
              onClick={() => {
                const subscription = rssPendingUnsubscribe;
                if (!subscription) return;
                void rssDeleteSubscription
                  .mutateAsync({ id: subscription.id })
                  .then(() => {
                    setRSSPendingUnsubscribe(null);
                    const latestRSSRoute = useAppWorkspaceStore.getState()
                      .locations[APP_WORKSPACE_IDS.rss]?.routeId ?? "all";
                    if (
                      latestRSSRoute ===
                      createRSSSubscriptionRouteId(subscription.id)
                    ) {
                      navigateWorkspace(
                        { routeId: "all" },
                        APP_WORKSPACE_IDS.rss,
                      );
                    }
                  })
                  .catch((error) => publishRSSActionFailure(
                    t("xiadown.rss.unsubscribe"),
                    error,
                  ));
              }}
              type="button"
              variant="destructive"
            >
              {rssDeleteSubscription.isPending
                ? <LoaderCircle className="app-motion-spin" />
                : <Trash2 />}
              {t("xiadown.rss.unsubscribe")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
