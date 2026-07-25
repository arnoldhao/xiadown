import { ArrowLeft,Compass,History,Home,Link2,Loader2,LogOut,PanelLeftClose,PanelLeftOpen,Pencil,RefreshCw,Search,Sparkles,Tags,Trophy,UserRound,Wrench,X } from "lucide-react";
import * as React from "react";
import { createPortal } from "react-dom";
import { siYoutube } from "simple-icons";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import {
DreamSegmentSwitch,
type DreamSegmentSwitchItem,
} from "@/shared/ui/dream-segment-switch";
import { DropdownMenu,DropdownMenuContent,DropdownMenuTrigger } from "@/shared/ui/dropdown-menu";
import { Input } from "@/shared/ui/input";
import { PetDisplay } from "@/shared/ui/pet-player";
import type { PetAnimation } from "@/shared/pets/animation";
import { openExternalURL } from "@/shared/query/system";
import { SidebarMenuItem } from "@/shared/ui/sidebar";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
import {
  defineWorkspacePageContract,
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageTopBar,
  type WorkspacePageContract,
} from "@/shared/ui/workspace-page";
import { WorkspaceSearchControl } from "@/shared/ui/workspace-search-control";
import {
LISTEN_CONTROL_ICON_BUTTON_CLASS,
} from "@/shared/styles/listen";

import type { ListenLiveUserCatalog } from "@/app/main/listen/api";
import { createListenArtistIdentity,isListenArtistShelfViewRequestCurrent,type ListenArtistShelfViewRequest } from "@/app/main/listen/artist-request-race";
import { ListenMuseArtistHero } from "@/app/main/listen/ArtistDetailHero";
import { LISTEN_LIKED_SONGS_SHELF_ID } from "@/app/main/listen/catalog";
import { resolveListenLibraryErrorPrompt } from "@/app/main/listen/error-prompts";
import { ListenHushLiveActionGroup,ListenHushLiveList } from "@/app/main/listen/HushLiveList";
import { ListenInfiniteScrollSentinel } from "@/app/main/listen/infinite-scroll-sentinel";
import { shouldAutoLoadListenLibraryPage } from "@/app/main/listen/library-pagination";
import { resolveListenLibraryViewPhase } from "@/app/main/listen/library-view-state";
import { ListenLocalLibraryWorkspace } from "@/app/main/listen/LocalLibraryWorkspace";
import { ListenLocalMetadataEditor } from "@/app/main/listen/LocalMetadataEditor";
import { formatListenLocalProbeWarning,ListenLocalProbeWarning } from "@/app/main/listen/LocalProbeWarning";
import { parseListenLocalWorkspaceRoute } from "@/app/main/listen/local-workspace";
import { ListenMusicCollectionDetail } from "@/app/main/listen/MusicCollectionDetail";
import { ListenPlayback } from "@/app/main/listen/Playback";
import { isListenPlaylistPlaybackDisabled } from "@/app/main/listen/playlist-playback";
import {
  ListenPrimaryLoadingBoundary,
  ListenPrimaryLoadingOverlay,
  ListenPrimaryStatusOverlay,
} from "@/app/main/listen/PrimaryLoadingOverlay";
import { buildListenImageCandidates,buildListenTrackThumbnailCandidates } from "@/app/main/listen/storage";
import type { ListenArtistBrowseState,ListenArtistItem,ListenCategoryItem,ListenLibraryShelf,ListenLiveGroup,ListenLiveStatus,ListenLocalItem,ListenMode,ListenNativePlayerEvent,ListenObservedPlaybackAudioQuality,ListenOnlineBrowseDetail,ListenOnlineBrowseSource,ListenOnlineItem,ListenPageProps,ListenPlayMode,ListenPlaybackProgressState,ListenPlayerCommand,ListenPlayerPresentation,ListenPlaylistItem,ListenPlaylistLibraryAction,ListenRemotePlaybackState,ListenSidebarView } from "@/app/main/listen/types";
import { ListenLocalArtwork,ListenModeTabs,ListenMuseArtistGroup,ListenMuseCategoryGroup,ListenMusePlaylistGroup,ListenMuseTrackGroup,ListenMuseTrackList,ListenMuseTrackListGroup,ListenOnlineGroup } from "@/app/main/listen/ui";
import { resolveListenPlayerSurfaceActive } from "@/app/main/listen/workspace-player-shared";
import { isListenWorkspaceLikedMusicRoute,isListenWorkspaceOnlinePlaylistsRoute,resolveMusicWorkspaceRoute,selectListenWorkspaceHomeShelves } from "@/app/main/listen/workspace-routes";

type SetState<T> = React.Dispatch<React.SetStateAction<T>>;

function resolveMusicWorkspacePageTitle(
  routeId: string | null | undefined,
  text: ListenPageProps["text"],
) {
  const route = routeId?.trim() || "home";
  if (route.startsWith("playlist:")) {
    return text.workspace.playlists;
  }
  switch (route) {
    case "search":
    case "local-search":
      return text.workspace.search;
    case "home":
    case "local-home":
      return text.workspace.home;
    case "radio":
      return text.workspace.radio;
    case "new-releases":
      return text.workspace.newReleases;
    case "charts":
      return text.workspace.charts;
    case "moods":
      return text.workspace.moods;
    case "podcasts":
      return text.workspace.podcasts;
    case "recent":
      return text.workspace.recent;
    case "history":
      return text.workspace.history;
    case "online-playlists":
      return text.workspace.playlists;
    case "recently-added":
      return text.workspace.recentlyAdded;
    case "artists":
      return text.workspace.artists;
    case "albums":
      return text.workspace.albums;
    case "songs":
      return text.workspace.songs;
    default:
      return text.workspace.music;
  }
}

function resolveMusicWorkspacePageContract(options: {
  workspaceLayout: boolean | undefined;
  routeId: string | undefined;
  routeLabel: string;
  search: boolean;
  local: boolean;
  radio: boolean;
  detail: boolean;
  detailLabel: string;
}): WorkspacePageContract | null {
  if (!options.workspaceLayout) {
    return null;
  }
  if (options.detail) {
    return defineWorkspacePageContract({
      presentation: "primary",
      recipe: "detail",
      routeLabel: options.detailLabel || options.routeLabel,
      topBar: "host-owned",
      heading: "host-owned",
      contentLayout: "shelves",
      footer: "none",
      scroll: "content",
      density: "comfortable",
      immersion: "standard",
    });
  }
  if (options.search) {
    return defineWorkspacePageContract({
      presentation: "primary",
      recipe: "search",
      routeLabel: options.routeLabel,
      topBar: "search",
      heading: "assistive",
      contentLayout: "list",
      footer: "none",
      scroll: "content",
      density: "regular",
      immersion: "standard",
    });
  }
  if (options.local || options.radio) {
    const cardGridRoute =
      options.routeId === "local-home" ||
      options.routeId === "artists" ||
      options.routeId === "albums";
    return defineWorkspacePageContract({
      presentation: "primary",
      recipe: "collection",
      routeLabel: options.routeLabel,
      topBar: "actions",
      heading: "assistive",
      contentLayout: cardGridRoute ? "card-grid" : "list",
      footer: "none",
      scroll: "content",
      density: "regular",
      immersion: "standard",
    });
  }
  return defineWorkspacePageContract({
    presentation: "primary",
    recipe: "browse",
    routeLabel: options.routeLabel,
    topBar: "drag",
    heading: "display",
    contentLayout: "shelves",
    footer: "none",
    scroll: "content",
    density: "comfortable",
    immersion: "standard",
  });
}

const LISTEN_HOME_IMAGE_PREFETCH_LIMIT = 48;
const LISTEN_HOME_IMAGE_PREFETCH_CONCURRENCY = 4;
const LISTEN_HEADER_GAP_REM = 0.5;
const LISTEN_HEADER_SEARCH_EXPANDED_REM = 12;
const LISTEN_HEADER_ACCOUNT_BUTTON_REM = 2.25;
const LISTEN_HEADER_FULL_TABS_REM = 14.625;
const LISTEN_HEADER_COMPACT_TABS_REM = 6.75;
const LISTEN_HEADER_HUSH_ACTIONS_REM = 7.25;
const LISTEN_HEADER_MUSE_ACTIONS_REM = 12.5;
const LISTEN_HEADER_LINGER_ACTIONS_REM = 2.25;
const LISTEN_ARTIST_TOP_SONGS_PREVIEW_LIMIT = 5;
const LISTEN_SEARCH_SONGS_PREVIEW_LIMIT = 5;
const LISTEN_LOCAL_MODIFIED_DAY_MS = 24 * 60 * 60 * 1000;

type ListenLocalModifiedGroup = {
  id: string;
  title: string;
  items: Array<{
    index: number;
    track: ListenLocalItem;
  }>;
};

type ListenPageViewState = {
  isWindows: boolean;
  isMac: boolean;
  listOpen: boolean;
  query: string;
  searchPlaceholder: string;
  mode: ListenMode;
  playbackMode: ListenMode;
  sidebarView: ListenSidebarView;
  effectiveSidebarView: ListenSidebarView;
  onlineBrowseSource: ListenOnlineBrowseSource;
  onlineBrowseDetail: ListenOnlineBrowseDetail | null;
  liveGroups: ListenLiveGroup[];
  selectedLiveGroupId: string;
  liveStatusByVideoId: Record<string, ListenLiveStatus>;
  liveCatalogLoading: boolean;
  liveCatalogError: boolean;
  liveCatalogMessage: string;
  liveUserCatalog: ListenLiveUserCatalog;
  liveUserCatalogLoading: boolean;
  liveUserCatalogSaving: boolean;
  liveUserCatalogError: string;
  curatedLiveItems: ListenOnlineItem[];
  liveSelectionArmed: boolean;
  selectedLiveId: string;
  filteredOnlineQueueItems: ListenOnlineItem[];
  onlineQueueTitle: string;
  onlineQueueCanUndo: boolean;
  onlineQueueCanRedo: boolean;
  selectedOnlineId: string;
  filteredLocalTracks: ListenLocalItem[];
  selectedLocalId: string;
  localPlaying: boolean;
  liveSearchNotice: string;
  showArtistDetail: boolean;
  artistBrowsePage: ListenArtistBrowseState | null;
  artistActionBusy: "" | "mix" | "subscribe";
  filteredArtistShelves: ListenLibraryShelf[];
  browsePlaylistId: string;
  savedPlaylistIds: Set<string>;
  playlistMutationAction: ListenPlaylistLibraryAction | null;
  playlistMutationPlaylistId: string;
  filteredArtistTracks: ListenOnlineItem[];
  showPlaylistDetail: boolean;
  selectedPlaylist: ListenPlaylistItem | null | undefined;
  playlistLoading: boolean;
  playlistAppending: boolean;
  playlistDetailAuthor: string;
  playlistDetailAuthorBrowseId: string;
  playlistDetailTrackCountLabel: string;
  playlistDetailDurationLabel: string;
  playlistDetailTitle: string;
  playlistDetailDescription: string;
  playlistDetailThumbnailURL: string;
  playlistTracks: ListenOnlineItem[];
  playlistContinuation: string;
  normalizedQuery: string;
  libraryLoading: boolean;
  libraryAppending: boolean;
  libraryError: boolean;
  libraryErrorCode: string;
  libraryErrorRetryable: boolean;
  libraryRequestReady: boolean;
  librarySettled: boolean;
  searchLoading: boolean;
  searchAppending: boolean;
  searchItems: ListenOnlineItem[];
  searchArtists: ListenArtistItem[];
  searchPlaylists: ListenPlaylistItem[];
  searchContinuation: string;
  libraryArtists: ListenArtistItem[];
  displayedLibraryPlaylists: ListenPlaylistItem[];
  showLibraryPlaylistGroup: boolean;
  homeShelves: ListenLibraryShelf[];
  libraryContinuation: string;
  onlineSearchNotice: string;
  localTracks: ListenLocalItem[];
  localPlaybackQueue: ListenLocalItem[];
  localQueueCanUndo: boolean;
  localQueueCanRedo: boolean;
  localTracksLoading: boolean;
  localTracksRefreshing: boolean;
  localTracksClearingMissing: boolean;
  localTracksError: string;
  activeOnline: ListenOnlineItem | null;
  selectedLocal: ListenLocalItem | null;
  onlinePlayerCommand: ListenPlayerCommand | null;
  localPlayerCommand: ListenPlayerCommand | null;
  onlineQueueItems: ListenOnlineItem[];
  onlinePlaying: boolean;
  onlinePlaybackArmed: boolean;
  onlinePlaybackActionPending: boolean;
  selectedLocalResumeTime: number;
  activeOnlineResumeTime: number;
  onlineProgress: ListenPlaybackProgressState & { videoId: string };
  onlineState: ListenRemotePlaybackState;
  onlinePlaybackErrorCode: string;
  onlinePlaybackErrorMessage: string;
  onlineObservedPlaybackAudioQuality: ListenObservedPlaybackAudioQuality | "";
  activeOnlineFavorite: boolean;
  activeOnlineFavoriteBusy: boolean;
  localProgress: ListenPlaybackProgressState;
  muted: boolean;
  volume: number;
  playMode: ListenPlayMode;
  museConnectBusy: boolean;
  museAccountName: string;
  museAccountAvatarURL: string;
  museAccountConnected: boolean;
  museAccountBusy: boolean;
  museManualRefreshKind: "" | "artist" | "library" | "playlist" | "search";
};

type ListenPageViewActions = {
  setListOpen: SetState<boolean>;
  setQuery: SetState<string>;
  selectFirstResult: () => void;
  setMode: SetState<ListenMode>;
  setSidebarView: SetState<ListenSidebarView>;
  reloadLiveCatalog: () => void;
  saveLiveUserCatalog: (catalog: ListenLiveUserCatalog) => Promise<void>;
  reloadLibrary: () => void;
  changeOnlineBrowseSource: (source: ListenOnlineBrowseSource) => void;
  openOnlineBrowseCategory: (item: ListenCategoryItem) => void;
  closeOnlineBrowseDetail: () => void;
  loadMoreLibrary: () => void;
  activateLiveSelection: (item: ListenOnlineItem) => void;
  selectOnlineQueueTrack: (item: ListenOnlineItem) => void;
  selectLocalQueueTrack: (item: ListenLocalItem) => void;
  clearLocalQueue: () => void;
  removeLocalQueueItem: (item: ListenLocalItem) => void;
  moveLocalQueueItem: (item: ListenLocalItem, direction: -1 | 1) => void;
  undoLocalQueueEdit: () => void;
  redoLocalQueueEdit: () => void;
  retryLocalTracks: () => void;
  playLocalBrowseTrack: (
    item: ListenLocalItem,
    queue: ListenLocalItem[],
  ) => void;
  setSelectedLocalId: SetState<string>;
  setLocalPlayerCommand: SetState<ListenPlayerCommand | null>;
  closeArtistBrowse: () => void;
  playArtistFromIndex: (index: number) => void;
  shuffleArtist: () => void;
  loadMoreArtist: () => void;
  loadArtistShelfTracks: (shelf: ListenLibraryShelf) => Promise<ListenOnlineItem[]>;
  playArtistMix: () => void;
  toggleArtistSubscription: () => void;
  openPlaylistBrowse: (item: ListenPlaylistItem) => void;
  updatePlaylistLibrary: (item: ListenPlaylistItem, action: ListenPlaylistLibraryAction) => void;
  setBrowsePlaylistId: SetState<string>;
  playPlaylistFromIndex: (index: number) => void;
  shufflePlaylist: () => void;
  playPlaylistNext: () => void;
  addPlaylistToQueue: () => void;
  loadMorePlaylist: () => void;
  playOnlineShelfTrack: (shelf: ListenLibraryShelf, item: ListenOnlineItem) => void;
  playOnlineShelfAll: (shelf: ListenLibraryShelf) => void;
  shuffleOnlineShelf: (shelf: ListenLibraryShelf) => void;
  playOnlineSearchTrack: (item: ListenOnlineItem) => void;
  playOnlineSearchResults: () => void;
  shuffleOnlineSearchResults: () => void;
  loadMoreSearch: () => void;
  openSearchArtistBrowse: (item: ListenArtistItem) => void;
  clearOnlineQueue: () => void;
  removeOnlineQueueItem: (item: ListenOnlineItem) => void;
  moveOnlineQueueItem: (item: ListenOnlineItem, direction: -1 | 1) => void;
  undoOnlineQueueEdit: () => void;
  redoOnlineQueueEdit: () => void;
  refreshLocalTracks: () => void;
  openRepairMissingLocalTracks: () => void;
  handlePlaybackEnded: () => void;
  setOnlinePlaying: (playing: boolean) => void;
  setOnlineState: (state: ListenRemotePlaybackState) => void;
  setOnlinePlaybackErrorCode: (code: string) => void;
  setOnlinePlaybackErrorMessage: (message: string) => void;
  handleOnlineProgressChange: (videoId: string, currentTime: number, duration: number, bufferedTime: number, transient?: boolean) => void;
  handleOnlineNativeTrackChange: (event: ListenNativePlayerEvent) => void;
  setLocalPlaying: (playing: boolean) => void;
  handleLocalProgressChange: (currentTime: number, duration: number, bufferedTime: number) => void;
  setPlaybackSessionStarted: SetState<boolean>;
  connectYouTube: () => void;
  refreshMusePage: () => void;
  signOutMuseAccount: () => void;
  playPrevious: () => void;
  playNext: () => void;
  togglePlayMode: () => void;
  setPlayMode: SetState<ListenPlayMode>;
  togglePlayback: () => void;
  toggleMute: () => void;
  handleVolumeChange: (value: number) => void;
  toggleOnlineFavorite: () => void;
  openOnlineArtistBrowse: (track: ListenOnlineItem) => void;
  openSelectedLocalDirectory: () => void;
};

function resolveListenShelfTitle(
  shelf: ListenLibraryShelf,
  text: ListenPageProps["text"],
  fallback: string,
) {
  if (shelf.id === LISTEN_LIKED_SONGS_SHELF_ID) {
    return text.listen.likedMusic;
  }
  return shelf.title || fallback;
}

function isListenTopSongsShelf(
  shelf: ListenLibraryShelf,
  title: string,
  text: ListenPageProps["text"],
) {
  if (shelf.kind !== "tracks") {
    return false;
  }
  const normalizedTitle = title.trim().toLocaleLowerCase();
  const normalizedShelfTitle = shelf.title.trim().toLocaleLowerCase();
  const normalizedLocalizedTitle = text.listen.shelfTopSongs
    .trim()
    .toLocaleLowerCase();
  return (
    normalizedTitle === "top songs" ||
    normalizedShelfTitle === "top songs" ||
    normalizedTitle === normalizedLocalizedTitle ||
    normalizedShelfTitle === normalizedLocalizedTitle
  );
}

function listenRemToPixels(value: number) {
  if (typeof window === "undefined") {
    return value * 16;
  }
  const rootFontSize = Number.parseFloat(
    window.getComputedStyle(document.documentElement).fontSize,
  );
  return value * (Number.isFinite(rootFontSize) ? rootFontSize : 16);
}

function useMeasuredElementWidth<T extends HTMLElement>() {
  const [element, setElement] = React.useState<T | null>(null);
  const [width, setWidth] = React.useState<number | null>(null);
  const ref = React.useCallback((nextElement: T | null) => {
    setElement(nextElement);
  }, []);

  React.useLayoutEffect(() => {
    if (!element) {
      setWidth(null);
      return;
    }

    const updateWidth = () => {
      const nextWidth = element.getBoundingClientRect().width;
      setWidth((current) =>
        current === null || Math.abs(current - nextWidth) > 0.5
          ? nextWidth
          : current,
      );
    };

    updateWidth();

    if (typeof ResizeObserver === "undefined") {
      if (typeof window === "undefined") {
        return;
      }
      window.addEventListener("resize", updateWidth);
      return () => window.removeEventListener("resize", updateWidth);
    }

    const observer = new ResizeObserver(updateWidth);
    observer.observe(element);
    return () => observer.disconnect();
  }, [element]);

  return [ref, width] as const;
}

function resolveListenHeaderActionWidthRem(mode: ListenMode) {
  if (mode === "hush") {
    return LISTEN_HEADER_HUSH_ACTIONS_REM;
  }
  if (mode === "muse") {
    return LISTEN_HEADER_MUSE_ACTIONS_REM;
  }
  if (mode === "linger") {
    return LISTEN_HEADER_LINGER_ACTIONS_REM;
  }
  return 0;
}

const LISTEN_ONLINE_BROWSE_SOURCES: ListenOnlineBrowseSource[] = [
  "home",
  "explore",
  "charts",
  "moods",
  "new",
  "history",
];

function listenOnlineBrowseSourceLabel(
  source: ListenOnlineBrowseSource,
  text: ListenPageProps["text"],
) {
  switch (source) {
    case "explore":
      return text.listen.sourceExplore;
    case "charts":
      return text.listen.sourceCharts;
    case "moods":
      return text.listen.sourceMoods;
    case "new":
      return text.listen.sourceNew;
    case "history":
      return text.listen.sourceHistory;
    default:
      return text.listen.sourceHome;
  }
}

function listenOnlineBrowseSourceIcon(source: ListenOnlineBrowseSource) {
  switch (source) {
    case "explore":
      return <Compass className="h-3.5 w-3.5" />;
    case "charts":
      return <Trophy className="h-3.5 w-3.5" />;
    case "moods":
      return <Tags className="h-3.5 w-3.5" />;
    case "new":
      return <Sparkles className="h-3.5 w-3.5" />;
    case "history":
      return <History className="h-3.5 w-3.5" />;
    default:
      return <Home className="h-3.5 w-3.5" />;
  }
}

function isListenAlbumPlaylistItem(item: ListenPlaylistItem | null | undefined) {
  const channel = item?.channel.trim().toLocaleLowerCase() ?? "";
  const playlistId = item?.playlistId.trim() ?? "";
  return (
    playlistId.startsWith("MPRE") ||
    playlistId.startsWith("OLAK") ||
    channel === "album" ||
    channel === "专辑" ||
    channel === "專輯" ||
    channel === "single" ||
    channel === "单曲" ||
    channel === "單曲" ||
    channel === "ep"
  );
}

function resolveListenPlaylistTypeLabel(
  playlist: ListenPlaylistItem,
  text: ListenPageProps["text"],
) {
  const normalized = playlist.channel.trim().toLocaleLowerCase();
  const playlistId = playlist.playlistId.trim();
  if (
    normalized === "single" ||
    normalized === "单曲" ||
    normalized === "單曲"
  ) {
    return text.listen.playlistTypeSingle;
  }
  if (normalized === "ep") {
    return text.listen.playlistTypeEP;
  }
  if (
    playlistId.startsWith("MPRE") ||
    playlistId.startsWith("OLAK") ||
    normalized === "album" ||
    normalized === "专辑" ||
    normalized === "專輯"
  ) {
    return text.listen.playlistTypeAlbum;
  }
  return text.listen.playlistTypePlaylist;
}

function resolveUsefulListenPlaylistAuthor(
  playlist: ListenPlaylistItem,
  detailAuthor: string,
  text: ListenPageProps["text"],
) {
  const candidates = [detailAuthor, playlist.channel, playlist.description];
  return (
    candidates.find((candidate) =>
      isUsefulListenPlaylistAuthor(candidate, playlist, text),
    )?.trim() ?? ""
  );
}

function isUsefulListenPlaylistAuthor(
  value: string,
  playlist: ListenPlaylistItem,
  text: ListenPageProps["text"],
) {
  const normalized = value.trim().toLocaleLowerCase();
  if (!normalized) {
    return false;
  }
  const ignored = new Set([
    "youtube music",
    "playlist",
    "album",
    "single",
    "ep",
    "专辑",
    "專輯",
    "单曲",
    "單曲",
    text.listen.groupPlaylist.trim().toLocaleLowerCase(),
    text.listen.playlistTypePlaylist.trim().toLocaleLowerCase(),
    text.listen.playlistTypeAlbum.trim().toLocaleLowerCase(),
    text.listen.playlistTypeSingle.trim().toLocaleLowerCase(),
    text.listen.playlistTypeEP.trim().toLocaleLowerCase(),
    playlist.title.trim().toLocaleLowerCase(),
  ]);
  return !ignored.has(normalized);
}

function formatListenPlaylistTrackCount(
  count: number,
  hasMoreTracks: boolean,
  text: ListenPageProps["text"],
) {
  const template = hasMoreTracks
    ? text.listen.playlistTrackCountMore
    : text.listen.playlistTrackCount;
  return template.replace("{count}", String(count));
}

function resolveListenPlaylistDescription(
  playlist: ListenPlaylistItem,
  author: string,
  typeLabel: string,
) {
  const description = playlist.description.trim();
  if (!description) {
    return "";
  }
  const normalized = description.toLocaleLowerCase();
  const ignored = new Set(
    [playlist.title, playlist.channel, author, typeLabel]
      .map((value) => value.trim().toLocaleLowerCase())
      .filter(Boolean),
  );
  return ignored.has(normalized) ? "" : description;
}

function resolveListenPlaylistDurationLabel(items: ListenOnlineItem[]) {
  const durations = items.map((item) => Number(item.durationSeconds ?? 0));
  if (
    durations.length === 0 ||
    durations.some((duration) => !Number.isFinite(duration) || duration <= 0)
  ) {
    return "";
  }
  const totalSeconds = durations.reduce((total, duration) => total + duration, 0);
  if (totalSeconds <= 0) {
    return "";
  }
  const rounded = Math.round(totalSeconds);
  const hours = Math.floor(rounded / 3600);
  const minutes = Math.floor((rounded % 3600) / 60);
  const seconds = rounded % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
    : `${minutes}:${String(seconds).padStart(2, "0")}`;
}

function ListenOnlineSourceTabs(props: {
  sources: ListenOnlineBrowseSource[];
  value: ListenOnlineBrowseSource;
  text: ListenPageProps["text"];
  onChange: (source: ListenOnlineBrowseSource) => void;
}) {
  const items = props.sources.map((source) => {
    const label = listenOnlineBrowseSourceLabel(source, props.text);
    return {
      value: source,
      label,
      tooltip: label,
      icon: listenOnlineBrowseSourceIcon(source),
    };
  }) satisfies DreamSegmentSwitchItem<ListenOnlineBrowseSource>[];

  return (
    <DreamSegmentSwitch
      value={props.value}
      items={items}
      compact
      ariaLabel={props.sources
        .map((source) => listenOnlineBrowseSourceLabel(source, props.text))
        .join(" / ")}
      className="listen-online-source-switch"
      onValueChange={props.onChange}
    />
  );
}

function ListenHeaderActionButton(props: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="app-completed-toolbar-button h-8 w-8 p-0"
          aria-label={props.label}
          data-active={props.active ? "true" : "false"}
          disabled={props.disabled}
          onClick={props.onClick}
        >
          {props.children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{props.label}</TooltipContent>
    </Tooltip>
  );
}

function ListenLingerHeaderActionGroup(props: {
  text: ListenPageProps["text"];
  refreshing: boolean;
  clearingMissing: boolean;
  onRefresh: () => void;
  onRepairMissing: () => void;
}) {
  return (
    <div className="app-dream-button-group app-completed-toolbar-actions inline-flex h-9 shrink-0 items-center p-0.5">
      <ListenHeaderActionButton
        label={props.text.listen.localRefresh}
        active={props.refreshing}
        disabled={props.refreshing}
        onClick={props.onRefresh}
      >
        <RefreshCw
          className={cn("h-4 w-4", props.refreshing && "listen-loading-spinner")}
        />
      </ListenHeaderActionButton>
      <ListenHeaderActionButton
        label={props.text.completed.relinkDialogTitle}
        disabled={props.clearingMissing}
        onClick={props.onRepairMissing}
      >
        {props.clearingMissing ? (
          <Loader2 className="h-4 w-4 listen-loading-spinner" />
        ) : (
          <Wrench className="h-4 w-4" />
        )}
      </ListenHeaderActionButton>
    </div>
  );
}

function ListenLocalTrackGroupList(props: {
  tracks: ListenLocalItem[];
  selectedId: string;
  httpBaseURL: string;
  text: ListenPageProps["text"];
  onSelect: (item: ListenLocalItem) => void;
  onMetadataSaved: () => void | Promise<void>;
}) {
  const [editorTrack, setEditorTrack] = React.useState<ListenLocalItem | null>(
    null,
  );
  const groups = React.useMemo(
    () => buildListenLocalModifiedGroups(props.tracks, props.text),
    [props.text, props.tracks],
  );
  return (
    <div className="listen-local-file-list space-y-5">
      {groups.map((group) => (
        <section key={group.id} className="min-w-0 space-y-2">
          <div className="listen-local-file-group-header wails-drag flex items-center justify-between gap-3 px-2">
            <span className="min-w-0 truncate">{group.title}</span>
            <span className="listen-local-file-group-count shrink-0">
              {group.items.length}
            </span>
          </div>
          <div className="space-y-1.5">
            {group.items.map(({ index, track }) => {
              const selected = track.id === props.selectedId;
              const probeWarning = formatListenLocalProbeWarning(
                props.text.listen.localProbeFailed,
                track.probeError,
              );
              return (
                <SidebarMenuItem key={track.id}>
                  <div className="listen-local-file-row group relative min-w-0">
                  <button
                    type="button"
                    aria-label={
                      !track.playbackSupported
                        ? `${track.title}: ${props.text.listen.localPlaybackUnsupported}`
                        : probeWarning
                          ? `${track.title}: ${probeWarning}`
                          : track.title
                    }
                    data-active={selected ? "true" : "false"}
                    className="listen-local-file-card grid min-h-14 w-full grid-cols-[2rem_minmax(0,1fr)_3.25rem] items-center gap-2 px-2 py-2"
                    disabled={!track.playbackSupported}
                    onClick={() => {
                      props.onSelect(track);
                    }}
                    title={
                      !track.playbackSupported
                        ? props.text.listen.localPlaybackUnsupported
                        : probeWarning || undefined
                    }
                  >
                    <span className="listen-local-file-index flex h-7 w-7 shrink-0 items-center justify-center">
                      {index}
                    </span>
                    <div className="flex min-w-0 items-center gap-2">
                      <ListenLocalArtwork
                        track={track}
                        className="listen-local-file-artwork"
                      />
                      <div className="min-w-0 flex-1">
                        <div className="listen-local-file-title truncate">
                          {track.title}
                        </div>
                        <div
                          className="listen-local-file-subtitle truncate"
                          data-playback-supported={track.playbackSupported ? "true" : "false"}
                        >
                          {!track.playbackSupported
                            ? `${props.text.listen.localPlaybackUnsupported}${
                                track.format || track.audioCodec
                                  ? ` · ${(track.format || track.audioCodec).toUpperCase()}`
                                  : ""
                              }`
                            : track.probeError
                              ? (
                                  <ListenLocalProbeWarning
                                    error={track.probeError}
                                    message={props.text.listen.localProbeFailed}
                                  />
                                )
                              : track.author || props.text.listen.linger}
                        </div>
                      </div>
                    </div>
                    <span className="listen-local-file-duration justify-self-end">
                      {track.durationLabel}
                    </span>
                  </button>
                  <Button
                    aria-label={props.text.listen.localMetadataEdit}
                    variant="ghost"
                    size="icon"
                    shape="square"
                    className="listen-local-file-edit absolute right-2 top-1/2 h-7 w-7 -translate-y-1/2 [&>svg]:h-3.5 [&>svg]:w-3.5"
                    disabled={!track.metadataWritable}
                    onClick={() => setEditorTrack(track)}
                    title={
                      track.metadataWritable
                        ? props.text.listen.localMetadataEdit
                        : props.text.listen.localMetadataUnsupported
                    }
                    type="button"
                  >
                    <Pencil />
                  </Button>
                  </div>
                </SidebarMenuItem>
              );
            })}
          </div>
        </section>
      ))}
      <ListenLocalMetadataEditor
        httpBaseURL={props.httpBaseURL}
        onOpenChange={(open) => {
          if (!open) {
            setEditorTrack(null);
          }
        }}
        onSaved={async () => {
          await props.onMetadataSaved();
        }}
        open={Boolean(editorTrack)}
        text={props.text}
        track={editorTrack}
      />
    </div>
  );
}

function buildListenLocalModifiedGroups(
  tracks: ListenLocalItem[],
  text: ListenPageProps["text"],
): ListenLocalModifiedGroup[] {
  const now = new Date();
  const todayStart = startOfLocalDay(now.getTime());
  const sortedTracks = [...tracks].sort((first, second) => {
    const timeDelta = second.modTimeUnix - first.modTimeUnix;
    if (timeDelta !== 0) {
      return timeDelta;
    }
    return (
      first.title.localeCompare(second.title) ||
      first.author.localeCompare(second.author) ||
      first.id.localeCompare(second.id)
    );
  });
  const groupSpecs: Array<{
    id: string;
    title: string;
    accepts: (item: ListenLocalItem) => boolean;
  }> = [
    {
      id: "today",
      title: text.listen.localModifiedToday,
      accepts: (item: ListenLocalItem) =>
        resolveListenLocalModifiedDayDiff(item, todayStart) <= 0,
    },
    {
      id: "yesterday",
      title: text.listen.localModifiedYesterday,
      accepts: (item: ListenLocalItem) =>
        resolveListenLocalModifiedDayDiff(item, todayStart) === 1,
    },
    {
      id: "last-7-days",
      title: text.listen.localModifiedLast7Days,
      accepts: (item: ListenLocalItem) => {
        const dayDiff = resolveListenLocalModifiedDayDiff(item, todayStart);
        return dayDiff > 1 && dayDiff < 7;
      },
    },
    {
      id: "last-30-days",
      title: text.listen.localModifiedLast30Days,
      accepts: (item: ListenLocalItem) => {
        const dayDiff = resolveListenLocalModifiedDayDiff(item, todayStart);
        return dayDiff >= 7 && dayDiff < 30;
      },
    },
    {
      id: "this-year",
      title: text.listen.localModifiedThisYear,
      accepts: (item: ListenLocalItem) =>
        item.modTimeUnix > 0 &&
        new Date(item.modTimeUnix * 1000).getFullYear() === now.getFullYear(),
    },
    {
      id: "older",
      title: text.listen.localModifiedOlder,
      accepts: (_item: ListenLocalItem) => true,
    },
  ];
  const groups = groupSpecs.map((group) => ({
    id: group.id,
    title: group.title,
    items: [] as ListenLocalModifiedGroup["items"],
  }));
  sortedTracks.forEach((track, index) => {
    const groupIndex = groupSpecs.findIndex((group) => group.accepts(track));
    groups[groupIndex >= 0 ? groupIndex : groups.length - 1]?.items.push({
      index: index + 1,
      track,
    });
  });
  return groups.filter((group) => group.items.length > 0);
}

function resolveListenLocalModifiedDayDiff(
  item: ListenLocalItem,
  todayStart: number,
) {
  if (item.modTimeUnix <= 0) {
    return Number.POSITIVE_INFINITY;
  }
  const itemDayStart = startOfLocalDay(item.modTimeUnix * 1000);
  return Math.floor((todayStart - itemDayStart) / LISTEN_LOCAL_MODIFIED_DAY_MS);
}

function startOfLocalDay(timestampMs: number) {
  const date = new Date(timestampMs);
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

function ListenMuseLoadingIndicator(props: {
  pet: ListenPageProps["pet"];
  petImageURL: string;
  label: string;
  animation?: PetAnimation;
  actionLabel?: string;
  onAction?: () => void;
}) {
  const hasAction = Boolean(props.actionLabel && props.onAction);
  return (
    <div className="flex justify-center px-2 pb-3">
      <div className="inline-flex max-w-[18rem] flex-col items-center gap-2">
        <div className="grid max-w-full grid-cols-[3rem_minmax(0,1fr)] items-center gap-2">
          <PetDisplay
            pet={props.pet}
            imageUrl={props.petImageURL}
            animation={props.animation ?? "running"}
            alt=""
            size={48}
            className="h-12 w-12 shrink-0"
            glowClassName="listen-muse-pet-glow"
          />
          <span className="listen-muse-loading-label min-w-0">
            {props.label}
          </span>
        </div>
        {hasAction ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            shape="capsule"
            className="listen-muse-loading-action h-7 max-w-full justify-center px-3"
            onClick={props.onAction}
          >
            {props.actionLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function ListenMuseAccountMenu(props: {
  text: ListenPageProps["text"];
  name: string;
  avatarURL: string;
  connected: boolean;
  busy: boolean;
  onConnect: () => void;
  onRefresh: () => void;
  onSignOut: () => void;
}) {
  const [open, setOpen] = React.useState(false);
  const [avatarFailed, setAvatarFailed] = React.useState(false);
  const name = props.connected
    ? props.name.trim() || props.text.listen.museAccountFallbackName
    : props.text.listen.museAccountDisconnectedName;
  const avatarURL = props.avatarURL.trim();
  const showAvatar = props.connected && avatarURL.length > 0 && !avatarFailed;

  React.useEffect(() => {
    setAvatarFailed(false);
  }, [avatarURL]);

  const handleConnect = React.useCallback(() => {
    setOpen(false);
    props.onConnect();
  }, [props]);

  const handleRefresh = React.useCallback(() => {
    setOpen(false);
    props.onRefresh();
  }, [props]);

  const handleSignOut = React.useCallback(() => {
    setOpen(false);
    props.onSignOut();
  }, [props]);

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <TooltipProvider delayDuration={0}>
        <Tooltip>
          <TooltipTrigger asChild>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                shape="square"
                className="listen-muse-account-trigger pointer-events-auto"
                aria-label={props.text.listen.museAccountTooltip}
              >
                <UserRound className="h-3.5 w-3.5" strokeWidth={1.65} />
              </Button>
            </DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            {props.text.listen.museAccountTooltip}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <DropdownMenuContent align="center" side="bottom" className="w-64 p-3">
        <div className="listen-muse-account-menu__identity flex min-w-0 flex-col items-center">
          <div
            className="listen-muse-account-avatar grid h-20 w-20 place-items-center overflow-hidden"
            data-connected={props.connected ? "true" : "false"}
          >
            {showAvatar ? (
              <img
                src={avatarURL}
                alt=""
                className="listen-muse-account-avatar__image h-full w-full object-cover"
                onError={() => setAvatarFailed(true)}
              />
            ) : !props.connected ? (
              <svg
                viewBox="0 0 24 24"
                fill="currentColor"
                aria-hidden="true"
                className="h-9 w-9"
              >
                <path d={siYoutube.path} />
              </svg>
            ) : (
              <UserRound className="h-8 w-8" />
            )}
          </div>
          <div className="listen-muse-account-menu__name mt-3 max-w-full truncate">
            {name}
          </div>
        </div>
        {props.connected ? (
          <div className="mt-4 grid grid-cols-2 gap-2">
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="listen-muse-account-action min-w-0 justify-center"
              disabled={props.busy}
              onClick={handleRefresh}
            >
              <RefreshCw className={cn("h-3.5 w-3.5", props.busy ? "listen-loading-spinner" : "")} />
              <span className="min-w-0 truncate">{props.text.listen.refresh}</span>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="compact"
              tone="destructive"
              className="listen-muse-account-action min-w-0 justify-center"
              disabled={props.busy}
              onClick={handleSignOut}
            >
              {props.busy ? (
                <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
              ) : (
                <LogOut className="h-3.5 w-3.5" />
              )}
              <span className="min-w-0 truncate">{props.text.listen.museAccountSignOut}</span>
            </Button>
          </div>
        ) : (
          <Button
            type="button"
            size="compact"
            className="listen-muse-account-action mx-auto mt-4 flex w-auto min-w-44 justify-center"
            disabled={props.busy}
            onClick={handleConnect}
          >
            {props.busy ? (
              <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
            ) : (
              <Link2 className="h-3.5 w-3.5" />
            )}
            <span className="min-w-0 truncate">{props.text.listen.museGateSignIn}</span>
          </Button>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ListenMuseAccountGate(props: {
  text: ListenPageProps["text"];
  pet: ListenPageProps["pet"];
  petImageURL: string;
  busy: boolean;
  onConnect: () => void;
}) {
  return (
    <div className="flex min-h-[calc(100vh-12rem)] items-center justify-center px-2 pb-6 pt-2">
      <div className="listen-muse-account-gate flex w-full max-w-[22rem] flex-col items-center">
        <PetDisplay
          pet={props.pet}
          imageUrl={props.petImageURL}
          animation={props.busy ? "waiting" : "waving"}
          alt=""
          size={112}
          className="mb-5 h-28 w-28 shrink-0"
          glowClassName="listen-muse-pet-glow"
        />
        <h2 className="listen-muse-account-gate__title max-w-full truncate">
          {props.text.listen.museGateTitle}
        </h2>

        <div className="mt-8 flex w-full flex-col items-center gap-3">
          <Button
            type="button"
            disabled={props.busy}
            className="listen-muse-account-action w-auto min-w-44"
            onClick={props.onConnect}
          >
            {props.busy ? (
              <Loader2 className="h-4 w-4 listen-loading-spinner" />
            ) : (
              <Link2 className="h-4 w-4" />
            )}
            {props.text.listen.museGateSignIn}
          </Button>
          <p className="listen-muse-account-gate__subtitle max-w-[18rem]">
            {props.text.listen.museGateSubtitle}
          </p>
        </div>
      </div>
    </div>
  );
}

export type ListenPageViewProps = {
  page: ListenPageProps;
  state: ListenPageViewState;
  actions: ListenPageViewActions;
};

function OptionalPortal(props: {
  target?: HTMLElement | null;
  children: React.ReactNode;
}) {
  return props.target ? createPortal(props.children, props.target) : props.children;
}

type ListenPrimaryPageFrameProps = {
  contract: WorkspacePageContract | null;
  reserveWindowControls: boolean;
  loading: boolean;
  covered: boolean;
  topBarActions?: React.ReactNode;
  searchControl?: React.ReactNode;
  overlay?: React.ReactNode;
  children: React.ReactNode;
};

function ListenPrimaryPageFrame(props: ListenPrimaryPageFrameProps) {
  if (!props.contract) {
    return (
      <>
        <ListenPrimaryLoadingBoundary
          loading={props.loading}
          covered={props.covered}
          data-listen-primary-scroll="true"
          className="listen-primary-page-frame__legacy min-h-0 flex-1 overflow-y-auto px-3 pb-4 pt-[4.75rem]"
        >
          {props.children}
        </ListenPrimaryLoadingBoundary>
        {props.overlay}
      </>
    );
  }
  return (
    <WorkspacePage contract={props.contract} className="listen-workspace-page">
      <WorkspacePageTopBar
        actionsLabel={props.contract.routeLabel}
        reserveWindowControls={props.reserveWindowControls}
      >
        {props.topBarActions}
      </WorkspacePageTopBar>
      <WorkspacePageContent
        className="listen-workspace-scroll"
        data-listen-primary-scroll="true"
      >
        <ListenPrimaryLoadingBoundary
          loading={props.loading}
          covered={props.covered}
          className="listen-workspace-page__boundary"
        >
          {props.searchControl}
          {props.children}
        </ListenPrimaryLoadingBoundary>
      </WorkspacePageContent>
      {props.overlay}
    </WorkspacePage>
  );
}

function collectListenHomeImagePrefetchURLs(
  httpBaseURL: string,
  options: {
    libraryArtists: ListenArtistItem[];
    displayedLibraryPlaylists: ListenPlaylistItem[];
    homeShelves: ListenLibraryShelf[];
  },
) {
  const seen = new Set<string>();
  const urls: string[] = [];
  const addFirstCandidate = (candidates: string[]) => {
    if (urls.length >= LISTEN_HOME_IMAGE_PREFETCH_LIMIT) {
      return;
    }
    const candidate = candidates.find((value) => value.trim());
    if (!candidate || seen.has(candidate)) {
      return;
    }
    seen.add(candidate);
    urls.push(candidate);
  };
  const addArtworkItem = (item: { thumbnailUrl?: string }) => {
    addFirstCandidate(
      buildListenImageCandidates(httpBaseURL, item.thumbnailUrl ?? ""),
    );
  };
  const addTrackItem = (item: { videoId: string; thumbnailUrl?: string }) => {
    addFirstCandidate(buildListenTrackThumbnailCandidates(httpBaseURL, item));
  };

  options.libraryArtists.slice(0, 10).forEach(addArtworkItem);
  options.displayedLibraryPlaylists.slice(0, 10).forEach(addArtworkItem);
  options.homeShelves.forEach((shelf) => {
    shelf.tracks.slice(0, 10).forEach(addTrackItem);
    shelf.playlists.slice(0, 10).forEach(addArtworkItem);
    shelf.artists.slice(0, 10).forEach(addArtworkItem);
  });
  return urls;
}

function prefetchListenImages(
  httpBaseURL: string,
  urls: string[],
  retainedImages: React.MutableRefObject<HTMLImageElement[]>,
) {
  void httpBaseURL;
  retainedImages.current = [];
  if (typeof window === "undefined" || urls.length === 0) {
    return undefined;
  }
  let disposed = false;
  let cursor = 0;
  const retained: HTMLImageElement[] = [];
  const startNext = () => {
    if (disposed) {
      return;
    }
    const source = urls[cursor++];
    if (!source) {
      return;
    }
    const image = new window.Image();
    image.decoding = "async";
    image.loading = "eager";
    image.onload = startNext;
    image.onerror = startNext;
    image.src = source;
    retained.push(image);
    retainedImages.current = retained;
  };
  for (
    let index = 0;
    index < Math.min(LISTEN_HOME_IMAGE_PREFETCH_CONCURRENCY, urls.length);
    index += 1
  ) {
    startNext();
  }
  return () => {
    disposed = true;
    retained.forEach((image) => {
      image.onload = null;
      image.onerror = null;
    });
    retainedImages.current = [];
  };
}

export function ListenPageView(view: ListenPageViewProps) {
  const props = view.page;
  const { isWindows, isMac, listOpen, query, searchPlaceholder, mode, playbackMode, effectiveSidebarView, onlineBrowseSource, onlineBrowseDetail, liveGroups, liveStatusByVideoId, liveCatalogLoading, liveCatalogError, liveCatalogMessage, liveUserCatalog, liveUserCatalogLoading, liveUserCatalogSaving, liveUserCatalogError, curatedLiveItems, liveSelectionArmed, selectedLiveId, filteredOnlineQueueItems, onlineQueueTitle, onlineQueueCanUndo, onlineQueueCanRedo, selectedOnlineId, filteredLocalTracks, selectedLocalId, localPlaying, liveSearchNotice, showArtistDetail, artistBrowsePage, artistActionBusy, filteredArtistShelves, browsePlaylistId, savedPlaylistIds, playlistMutationAction, playlistMutationPlaylistId, filteredArtistTracks, showPlaylistDetail, selectedPlaylist, playlistLoading, playlistAppending, playlistDetailAuthor, playlistDetailAuthorBrowseId, playlistDetailTrackCountLabel, playlistDetailDurationLabel, playlistDetailTitle, playlistDetailDescription, playlistDetailThumbnailURL, playlistTracks, playlistContinuation, normalizedQuery, libraryLoading, libraryAppending, libraryError, libraryErrorCode, libraryErrorRetryable, libraryRequestReady, librarySettled, searchLoading, searchAppending, searchItems, searchArtists, searchPlaylists, searchContinuation, libraryArtists, displayedLibraryPlaylists, showLibraryPlaylistGroup, homeShelves, libraryContinuation, onlineSearchNotice, localTracks, localPlaybackQueue, localQueueCanUndo, localQueueCanRedo, localTracksLoading, localTracksRefreshing, localTracksClearingMissing, localTracksError, activeOnline, selectedLocal, onlinePlayerCommand, localPlayerCommand, onlineQueueItems, onlinePlaying, onlinePlaybackArmed, onlinePlaybackActionPending, selectedLocalResumeTime, activeOnlineResumeTime, onlineProgress, onlineState, onlinePlaybackErrorCode, onlinePlaybackErrorMessage, onlineObservedPlaybackAudioQuality, activeOnlineFavorite, activeOnlineFavoriteBusy, localProgress, muted, volume, playMode, museConnectBusy, museAccountName, museAccountAvatarURL, museAccountConnected, museAccountBusy, museManualRefreshKind } = view.state;
  const { setListOpen, setQuery, selectFirstResult, setMode, setSidebarView, reloadLiveCatalog, saveLiveUserCatalog, reloadLibrary, changeOnlineBrowseSource, openOnlineBrowseCategory, closeOnlineBrowseDetail, loadMoreLibrary, activateLiveSelection, selectOnlineQueueTrack, selectLocalQueueTrack, clearLocalQueue, removeLocalQueueItem, moveLocalQueueItem, undoLocalQueueEdit, redoLocalQueueEdit, retryLocalTracks, playLocalBrowseTrack, closeArtistBrowse, playArtistFromIndex, shuffleArtist, loadMoreArtist, loadArtistShelfTracks, playArtistMix, toggleArtistSubscription, openPlaylistBrowse, updatePlaylistLibrary, setBrowsePlaylistId, playPlaylistFromIndex, shufflePlaylist, playPlaylistNext, addPlaylistToQueue, loadMorePlaylist, playOnlineShelfTrack, playOnlineShelfAll, shuffleOnlineShelf, loadMoreSearch, openSearchArtistBrowse, playOnlineSearchTrack, playOnlineSearchResults, shuffleOnlineSearchResults, clearOnlineQueue, removeOnlineQueueItem, moveOnlineQueueItem, undoOnlineQueueEdit, redoOnlineQueueEdit, refreshLocalTracks, openRepairMissingLocalTracks, handlePlaybackEnded, setOnlinePlaying, setOnlineState, setOnlinePlaybackErrorCode, setOnlinePlaybackErrorMessage, handleOnlineProgressChange, handleOnlineNativeTrackChange, setLocalPlaying, handleLocalProgressChange, setPlaybackSessionStarted, connectYouTube, refreshMusePage, signOutMuseAccount, playPrevious, playNext, togglePlayMode, setPlayMode, togglePlayback, toggleMute, handleVolumeChange, toggleOnlineFavorite, openOnlineArtistBrowse, openSelectedLocalDirectory } = view.actions;
  const playerPresentation: ListenPlayerPresentation =
    props.playerFullscreen === true
      ? "fullscreen"
      : props.playerPresentation ??
        (props.workspaceLayout === true ? "companion" : "page");
  const playerSurfaceActive = resolveListenPlayerSurfaceActive(
    props.active,
    props.playerSurfaceVisible,
  );
  const presentationListOpen =
    playerPresentation === "page" ? listOpen : false;
  const hushFullscreen =
    playerPresentation === "page" &&
    playbackMode === "hush" &&
    !presentationListOpen;
  const activeLocalSelectedId =
    playbackMode === "linger" && selectedLocal ? selectedLocalId : "";
  const visibleLocalQueue = React.useMemo(() => {
    if (!normalizedQuery) {
      return localPlaybackQueue;
    }
    const visibleIds = new Set(filteredLocalTracks.map((track) => track.id));
    return localPlaybackQueue.filter((track) => visibleIds.has(track.id));
  }, [filteredLocalTracks, localPlaybackQueue, normalizedQuery]);
  const localWorkspaceRoute = props.workspaceLayout
    ? parseListenLocalWorkspaceRoute(props.workspaceRouteId)
    : null;
  const musicWorkspaceRoute = props.workspaceLayout
    ? resolveMusicWorkspaceRoute(props.workspaceRouteId)
    : null;
  const workspaceSearchRoute =
    musicWorkspaceRoute?.content === "search" ||
    musicWorkspaceRoute?.content === "local-search";
  const [workspaceSearchDraft, setWorkspaceSearchDraft] =
    React.useState(query);
  React.useEffect(() => {
    setWorkspaceSearchDraft(query);
  }, [query, props.workspaceRouteId]);
  const onlinePlaylistsWorkspaceRoute = isListenWorkspaceOnlinePlaylistsRoute(
    props.workspaceLayout,
    props.workspaceRouteId,
  );
  const onlineHomeWorkspaceRoute =
    props.workspaceLayout === true &&
    (props.workspaceRouteId?.trim() || "home") === "home";
  const likedMusicWorkspaceRoute = isListenWorkspaceLikedMusicRoute(
    props.workspaceLayout,
    props.workspaceRouteId,
  );
  const visibleHomeShelves = React.useMemo(
    () =>
      workspaceSearchRoute || onlinePlaylistsWorkspaceRoute
        ? []
        : selectListenWorkspaceHomeShelves(
            homeShelves,
            props.workspaceLayout,
            props.workspaceRouteId,
          ),
    [
      homeShelves,
      onlinePlaylistsWorkspaceRoute,
      props.workspaceLayout,
      props.workspaceRouteId,
      workspaceSearchRoute,
    ],
  );
  const visibleLibraryArtists = React.useMemo(
    () =>
      likedMusicWorkspaceRoute ||
      onlinePlaylistsWorkspaceRoute ||
      onlineHomeWorkspaceRoute ||
      workspaceSearchRoute
        ? []
        : libraryArtists,
    [
      libraryArtists,
      likedMusicWorkspaceRoute,
      onlineHomeWorkspaceRoute,
      onlinePlaylistsWorkspaceRoute,
      workspaceSearchRoute,
    ],
  );
  const visibleLibraryPlaylists = React.useMemo(
    () =>
      likedMusicWorkspaceRoute || onlineHomeWorkspaceRoute || workspaceSearchRoute
        ? []
        : displayedLibraryPlaylists,
    [
      displayedLibraryPlaylists,
      likedMusicWorkspaceRoute,
      onlineHomeWorkspaceRoute,
      workspaceSearchRoute,
    ],
  );
  const visibleLibraryPlaylistGroup =
    !likedMusicWorkspaceRoute &&
    !onlineHomeWorkspaceRoute &&
    !workspaceSearchRoute &&
    (onlinePlaylistsWorkspaceRoute || showLibraryPlaylistGroup);
  const libraryViewPhase = resolveListenLibraryViewPhase({
    workspaceLayout: props.workspaceLayout,
    workspaceRouteId: props.workspaceRouteId,
    mode,
    onlineBrowseSource,
    accountConnected: museAccountConnected,
    requestReady: libraryRequestReady,
    settled: librarySettled,
    loading: libraryLoading,
    error: libraryError,
    hasVisibleContent:
      visibleHomeShelves.length > 0 ||
      visibleLibraryArtists.length > 0 ||
      (visibleLibraryPlaylistGroup && visibleLibraryPlaylists.length > 0),
  });
  const autoLoadLibraryPage = shouldAutoLoadListenLibraryPage({
    normalizedQuery,
    continuation: libraryContinuation,
    likedMusicWorkspaceRoute,
    workspaceSearchRoute,
  });
  const libraryPaginationScope = `${onlineBrowseSource}:${onlineBrowseDetail?.browseId ?? ""}:${onlineBrowseDetail?.params ?? ""}`;
  const workspacePageTitle = resolveMusicWorkspacePageTitle(
    props.workspaceRouteId,
    props.text,
  );
  const showContentListToggle =
    playerPresentation === "page" && !hushFullscreen;
  const libraryErrorPrompt = resolveListenLibraryErrorPrompt(
    libraryErrorCode,
    props.text,
    libraryErrorRetryable,
  );
  const [searchFocused, setSearchFocused] = React.useState(false);
  const [workspaceTopBarActionsTarget, setWorkspaceTopBarActionsTarget] =
    React.useState<HTMLDivElement | null>(null);
  const workspaceTopBarActionsRef = React.useCallback(
    (element: HTMLDivElement | null) => {
      setWorkspaceTopBarActionsTarget(element);
    },
    [],
  );
  const [headerRef, headerWidth] = useMeasuredElementWidth<HTMLDivElement>();
  const searchInputRef = React.useRef<HTMLInputElement | null>(null);
  const imagePrefetchRef = React.useRef<HTMLImageElement[]>([]);
  const searchHasText = query.length > 0;
  const searchInputActive = searchFocused || searchHasText;
  const [searchControlMounted, setSearchControlMounted] =
    React.useState(searchInputActive);
  const headerActionWidthRem = resolveListenHeaderActionWidthRem(mode);
  const headerActionGroupVisible =
    headerActionWidthRem > 0 && !(props.workspaceLayout && mode === "muse");
  const playlistActionDisabled =
    onlinePlaybackActionPending ||
    isListenPlaylistPlaybackDisabled({
      loading: playlistLoading,
      appending: playlistAppending,
      itemCount: playlistTracks.length,
    });
  const selectedPlaylistSaved = selectedPlaylist
    ? savedPlaylistIds.has(selectedPlaylist.playlistId)
    : false;
  const selectedPlaylistIsAlbum = isListenAlbumPlaylistItem(selectedPlaylist);
  const selectedPlaylistTitle =
    playlistDetailTitle.trim() ||
    selectedPlaylist?.title.trim() ||
    props.text.listen.groupPlaylist;
  const selectedPlaylistTypeLabel = selectedPlaylist
    ? resolveListenPlaylistTypeLabel(selectedPlaylist, props.text)
    : selectedPlaylistIsAlbum
      ? props.text.listen.playlistTypeAlbum
      : props.text.listen.playlistTypePlaylist;
  const selectedPlaylistAuthor = selectedPlaylistIsAlbum
    ? playlistDetailAuthor.trim()
    : selectedPlaylist
    ? resolveUsefulListenPlaylistAuthor(
        selectedPlaylist,
        playlistDetailAuthor,
        props.text,
      )
    : playlistDetailAuthor.trim();
  const selectedPlaylistDescription = selectedPlaylist
    ? resolveListenPlaylistDescription(
        {
          ...selectedPlaylist,
          description:
            playlistDetailDescription.trim() || selectedPlaylist.description,
        },
        selectedPlaylistAuthor,
        selectedPlaylistTypeLabel,
      )
    : playlistDetailDescription.trim();
  const selectedPlaylistTrackCount =
    playlistDetailTrackCountLabel.trim() ||
    (playlistTracks.length > 0
      ? formatListenPlaylistTrackCount(
          playlistTracks.length,
          Boolean(playlistContinuation),
          props.text,
        )
      : "");
  const selectedPlaylistDuration =
    playlistDetailDurationLabel.trim() ||
    resolveListenPlaylistDurationLabel(playlistTracks);
  const selectedPlaylistHeaderMetadata = [
    selectedPlaylistTrackCount,
    selectedPlaylistDuration,
  ]
    .filter(Boolean)
    .join(" · ");
  const selectedPlaylistLibraryBusy = Boolean(
    selectedPlaylist &&
      playlistMutationAction !== null &&
      playlistMutationPlaylistId === selectedPlaylist.playlistId,
  );
  const artistBrowseIdentity = createListenArtistIdentity(artistBrowsePage);
  const artistBrowseIdentityRef = React.useRef(artistBrowseIdentity);
  const artistTrackListGenerationRef = React.useRef(0);
  const [artistTrackListView, setArtistTrackListView] = React.useState<{
    artistIdentity: string;
    generation: number;
    shelfId: string;
    title: string;
    tracks: ListenOnlineItem[];
    loading: boolean;
  }>({
    artistIdentity: artistBrowseIdentity,
    generation: 0,
    shelfId: "",
    title: "",
    tracks: [],
    loading: false,
  });
  const [searchSongsListOpen, setSearchSongsListOpen] = React.useState(false);
  React.useLayoutEffect(() => {
    artistBrowseIdentityRef.current = artistBrowseIdentity;
    const generation = artistTrackListGenerationRef.current + 1;
    artistTrackListGenerationRef.current = generation;
    setArtistTrackListView({
      artistIdentity: artistBrowseIdentity,
      generation,
      shelfId: "",
      title: "",
      tracks: [],
      loading: false,
    });
  }, [artistBrowseIdentity, showArtistDetail]);
  React.useEffect(() => {
    setSearchSongsListOpen(false);
  }, [normalizedQuery]);
  const selectedArtistTrackListShelf = React.useMemo(() => {
    if (
      !artistTrackListView.shelfId ||
      artistTrackListView.artistIdentity !== artistBrowseIdentity
    ) {
      return null;
    }
    const shelf = filteredArtistShelves.find(
      (item) => item.id === artistTrackListView.shelfId,
    );
    return shelf?.kind === "tracks" ? shelf : null;
  }, [
    artistBrowseIdentity,
    artistTrackListView.artistIdentity,
    artistTrackListView.shelfId,
    filteredArtistShelves,
  ]);
  const artistDisplayName =
    artistBrowsePage?.name || artistBrowsePage?.title || artistBrowsePage?.id || "";
  const selectedArtistTrackListTitle = selectedArtistTrackListShelf
    ? artistTrackListView.title ||
      resolveListenShelfTitle(
        selectedArtistTrackListShelf,
        props.text,
        props.text.listen.shelfTopSongs,
      )
    : "";
  const selectedArtistTrackListTracks =
    artistTrackListView.tracks.length > 0
      ? artistTrackListView.tracks
      : selectedArtistTrackListShelf?.tracks ?? [];
  const artistHeaderTitle = selectedArtistTrackListShelf
    ? selectedArtistTrackListTitle
    : artistDisplayName;
  const artistHeaderSubtitle = selectedArtistTrackListShelf
    ? artistDisplayName
    : artistBrowsePage?.subtitle || "";
  const workspaceDetailActive =
    Boolean(showArtistDetail && artistBrowsePage) || showPlaylistDetail;
  const workspaceDetailLabel = showArtistDetail
    ? artistHeaderTitle
    : showPlaylistDetail
      ? selectedPlaylistTitle
      : "";
  const workspacePageContract = resolveMusicWorkspacePageContract({
    workspaceLayout: props.workspaceLayout,
    routeId: props.workspaceRouteId,
    routeLabel: workspacePageTitle,
    search: workspaceSearchRoute,
    local: musicWorkspaceRoute?.scope === "local",
    radio: musicWorkspaceRoute?.content === "radio",
    detail: workspaceDetailActive,
    detailLabel: workspaceDetailLabel,
  });
  const playArtistTrack = React.useCallback(
    (item: ListenOnlineItem) => {
      const index =
        artistBrowsePage?.items.findIndex(
          (track) => track.id === item.id || track.videoId === item.videoId,
        ) ?? -1;
      if (index >= 0) {
        playArtistFromIndex(index);
      }
    },
    [artistBrowsePage?.items, playArtistFromIndex],
  );
  const openArtistTrackListShelf = React.useCallback(
    (shelf: ListenLibraryShelf, title: string) => {
      const request: ListenArtistShelfViewRequest = {
        artistIdentity: artistBrowseIdentity,
        generation: artistTrackListGenerationRef.current + 1,
        shelfId: shelf.id,
      };
      artistTrackListGenerationRef.current = request.generation;
      setArtistTrackListView({
        artistIdentity: request.artistIdentity,
        generation: request.generation,
        shelfId: shelf.id,
        title,
        tracks: shelf.tracks,
        loading: true,
      });
      void loadArtistShelfTracks(shelf)
        .then((tracks) => {
          setArtistTrackListView((current) =>
            isListenArtistShelfViewRequestCurrent(
              current,
              request,
              artistBrowseIdentityRef.current,
            )
              ? {
                  ...current,
                  tracks: tracks.length > 0 ? tracks : current.tracks,
                  loading: false,
                }
              : current,
          );
        })
        .catch(() => {
          setArtistTrackListView((current) =>
            isListenArtistShelfViewRequestCurrent(
              current,
              request,
              artistBrowseIdentityRef.current,
            )
              ? { ...current, loading: false }
              : current,
          );
        });
    },
    [artistBrowseIdentity, loadArtistShelfTracks],
  );
  const closeArtistTrackListShelf = React.useCallback(() => {
    const generation = artistTrackListGenerationRef.current + 1;
    artistTrackListGenerationRef.current = generation;
    setArtistTrackListView({
      artistIdentity: artistBrowseIdentity,
      generation,
      shelfId: "",
      title: "",
      tracks: [],
      loading: false,
    });
  }, [artistBrowseIdentity]);
  const museAccountMenuVisible = mode === "muse" && !props.workspaceLayout;
  const museAccountWidthRem = museAccountMenuVisible
    ? LISTEN_HEADER_ACCOUNT_BUTTON_REM
    : 0;
  const searchHeaderGapCount =
    (headerActionGroupVisible ? 1 : 0) +
    (museAccountMenuVisible ? 1 : 0) +
    1;
  const minimumExpandedSearchHeaderWidth =
    LISTEN_HEADER_FULL_TABS_REM +
    LISTEN_HEADER_SEARCH_EXPANDED_REM +
    museAccountWidthRem +
    headerActionWidthRem +
    LISTEN_HEADER_GAP_REM * searchHeaderGapCount;
  const minimumCompactSearchHeaderWidth =
    LISTEN_HEADER_COMPACT_TABS_REM +
    LISTEN_HEADER_SEARCH_EXPANDED_REM +
    museAccountWidthRem +
    headerActionWidthRem +
    LISTEN_HEADER_GAP_REM * searchHeaderGapCount;
  const minimumNoModeTabsSearchHeaderWidth =
    LISTEN_HEADER_SEARCH_EXPANDED_REM +
    museAccountWidthRem +
    headerActionWidthRem +
    LISTEN_HEADER_GAP_REM *
      ((headerActionGroupVisible ? 1 : 0) +
        (museAccountMenuVisible ? 1 : 0));
  const headerMeasured = headerWidth !== null;
  const tabsCompact =
    searchInputActive &&
    headerMeasured &&
    headerWidth < listenRemToPixels(minimumExpandedSearchHeaderWidth);
  const hideModeTabsForSearch =
    searchInputActive &&
    headerMeasured &&
    headerWidth < listenRemToPixels(minimumCompactSearchHeaderWidth);
  const hideHeaderActionsForSearch =
    searchInputActive &&
    headerActionGroupVisible &&
    headerMeasured &&
    headerWidth < listenRemToPixels(minimumNoModeTabsSearchHeaderWidth);
  const searchToolbarState = searchInputActive
    ? "active"
    : searchControlMounted
      ? "closing"
      : "idle";
  const searchFieldMounted = searchInputActive || searchControlMounted;
  const activateSearchInput = React.useCallback(() => {
    setSearchFocused(true);
    window.requestAnimationFrame(() => {
      searchInputRef.current?.focus();
    });
  }, []);

  const handleSearchValueChange = React.useCallback(
    (nextQuery: string) => {
      setQuery(nextQuery);
      setSearchFocused(true);
    },
    [setQuery],
  );

  const handleSearchChange = React.useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      handleSearchValueChange(event.target.value);
    },
    [handleSearchValueChange],
  );

  const handleSearchBlur = React.useCallback(() => {
    if (!query.length) {
      setSearchFocused(false);
    }
  }, [query.length]);

  const clearSearch = React.useCallback(() => {
    setWorkspaceSearchDraft("");
    setQuery("");
    setSearchFocused(false);
  }, [setQuery]);

  const submitWorkspaceSearch = React.useCallback(
    (value: string) => {
      setWorkspaceSearchDraft(value);
      setQuery(value);
    },
    [setQuery],
  );

  React.useEffect(() => {
    if (searchInputActive) {
      setSearchControlMounted(true);
      return;
    }
    const timeout = window.setTimeout(() => {
      setSearchControlMounted(false);
    }, 220);
    return () => window.clearTimeout(timeout);
  }, [searchInputActive]);

  const handleOnlineSourceTabChange = React.useCallback(
    (source: ListenOnlineBrowseSource) => {
      setSidebarView("browse");
      clearSearch();
      changeOnlineBrowseSource(source);
    },
    [changeOnlineBrowseSource, clearSearch, setSidebarView],
  );

  const headerActionGroup =
    mode === "hush" ? (
      <ListenHushLiveActionGroup
        httpBaseURL={props.httpBaseURL}
        text={props.text}
        liveGroups={liveGroups}
        liveCatalogLoading={liveCatalogLoading}
        liveUserCatalog={liveUserCatalog}
        liveUserCatalogLoading={liveUserCatalogLoading}
        liveUserCatalogSaving={liveUserCatalogSaving}
        onReloadCatalog={reloadLiveCatalog}
        onSaveUserCatalog={saveLiveUserCatalog}
      />
    ) : mode === "linger" && !props.workspaceLayout ? (
      <ListenLingerHeaderActionGroup
        text={props.text}
        refreshing={localTracksRefreshing}
        clearingMissing={localTracksClearingMissing}
        onRefresh={refreshLocalTracks}
        onRepairMissing={openRepairMissingLocalTracks}
      />
    ) : mode === "muse" && !props.workspaceLayout ? (
      <ListenOnlineSourceTabs
        sources={LISTEN_ONLINE_BROWSE_SOURCES}
        value={onlineBrowseSource}
        text={props.text}
        onChange={handleOnlineSourceTabChange}
      />
    ) : null;
  const workspaceTopBarActions =
    workspacePageContract?.topBar === "actions" ? (
      <>
        {headerActionGroup}
        {localWorkspaceRoute ? (
          <div
            ref={workspaceTopBarActionsRef}
            className="listen-local-workspace-topbar-actions contents"
          />
        ) : null}
      </>
    ) : undefined;

  React.useEffect(() => {
    const shouldPrefetch =
      props.active &&
      listOpen &&
      mode === "muse" &&
      effectiveSidebarView === "browse" &&
      onlineBrowseSource === "home" &&
      !onlineBrowseDetail &&
      !normalizedQuery;
    if (!shouldPrefetch) {
      imagePrefetchRef.current = [];
      return;
    }
    const urls = collectListenHomeImagePrefetchURLs(props.httpBaseURL, {
      libraryArtists: visibleLibraryArtists,
      displayedLibraryPlaylists: visibleLibraryPlaylists,
      homeShelves: visibleHomeShelves,
    });
    return prefetchListenImages(props.httpBaseURL, urls, imagePrefetchRef);
  }, [
    visibleLibraryPlaylists,
    effectiveSidebarView,
    visibleHomeShelves,
    visibleLibraryArtists,
    listOpen,
    mode,
    normalizedQuery,
    onlineBrowseDetail,
    onlineBrowseSource,
    props.active,
    props.httpBaseURL,
  ]);

  const showMuseAccountGate =
    mode === "muse" &&
    effectiveSidebarView === "browse" &&
    !showArtistDetail &&
    !showPlaylistDetail &&
    (libraryViewPhase === "disconnected" ||
      (!normalizedQuery &&
        libraryViewPhase === "error" &&
        libraryErrorPrompt.action === "connections"));

  const pagePrompt: {
    label: string;
    tone?: "error" | "empty";
    animation?: PetAnimation;
    actionLabel?: string;
    onAction?: () => void;
  } | null = (() => {
    if (effectiveSidebarView === "queue") {
      if (mode === "linger" && localTracksError && localTracks.length === 0) {
        return {
          label: props.text.listen.localLoadFailed,
          tone: "error",
          animation: "failed",
          actionLabel: props.text.listen.retry,
          onAction: retryLocalTracks,
        };
      }
      if (mode === "linger" && localTracksLoading) {
        return { label: props.text.listen.localLoading };
      }
      if (
        (mode === "hush" && curatedLiveItems.length === 0) ||
        (mode === "muse" && filteredOnlineQueueItems.length === 0) ||
        (mode === "linger" && !localTracksLoading && visibleLocalQueue.length === 0)
      ) {
        return { label: props.text.listen.upNextEmpty, animation: "review" };
      }
      return null;
    }
    if (mode === "hush" && effectiveSidebarView === "browse") {
      if (liveUserCatalogError) {
        return {
          label: liveUserCatalogError,
          tone: "error",
          animation: "failed",
          actionLabel: props.text.listen.retry,
          onAction: reloadLiveCatalog,
        };
      }
      if (liveCatalogLoading && liveGroups.length === 0) {
        return { label: props.text.listen.liveLoading };
      }
      if (liveCatalogError || liveGroups.length === 0) {
        return {
          label: liveCatalogError
            ? liveCatalogMessage || props.text.listen.liveUnavailable
            : props.text.listen.liveEmpty,
          tone: liveCatalogError ? "error" : "empty",
          animation: liveCatalogError ? "failed" : "review",
          actionLabel: props.text.listen.retry,
          onAction: reloadLiveCatalog,
        };
      }
      if (liveSearchNotice) {
        return { label: liveSearchNotice, animation: "review" };
      }
      return null;
    }
    if (mode === "muse" && effectiveSidebarView === "browse") {
      if (!museAccountConnected) {
        return null;
      }
      if (museManualRefreshKind === "artist") {
        return { label: props.text.listen.artistLoading };
      }
      if (museManualRefreshKind === "playlist") {
        return { label: props.text.listen.playlistLoading };
      }
      if (museManualRefreshKind === "search") {
        return { label: props.text.listen.searchLoading };
      }
      if (museManualRefreshKind === "library") {
        return { label: props.text.listen.onlineLoading };
      }
      if (showArtistDetail && artistBrowsePage?.loading) {
        return { label: props.text.listen.artistLoading };
      }
      if (
        showArtistDetail &&
        selectedArtistTrackListShelf &&
        artistTrackListView.loading
      ) {
        return { label: props.text.listen.loading };
      }
      if (showArtistDetail && artistBrowsePage?.error) {
        return {
          label: props.text.listen.artistUnavailable,
          tone: "error",
          animation: "failed",
          actionLabel: props.text.listen.retry,
          onAction: refreshMusePage,
        };
      }
      if (
        showArtistDetail &&
        !artistBrowsePage?.loading &&
        !artistBrowsePage?.error &&
        selectedArtistTrackListShelf &&
        !artistTrackListView.loading &&
        selectedArtistTrackListTracks.length === 0
      ) {
        return { label: props.text.listen.artistEmpty, animation: "review" };
      }
      if (
        showArtistDetail &&
        !artistBrowsePage?.loading &&
        !artistBrowsePage?.error &&
        !selectedArtistTrackListShelf &&
        filteredArtistShelves.length === 0 &&
        filteredArtistTracks.length === 0
      ) {
        return { label: props.text.listen.artistEmpty, animation: "review" };
      }
      if (showPlaylistDetail && playlistLoading) {
        return { label: props.text.listen.playlistLoading };
      }
      if (
        showPlaylistDetail &&
        !playlistLoading &&
        playlistTracks.length === 0
      ) {
        return { label: props.text.listen.playlistEmpty, animation: "review" };
      }
      if (workspaceSearchRoute && !normalizedQuery) {
        return null;
      }
      if (!normalizedQuery && libraryViewPhase === "error") {
        if (showMuseAccountGate) {
          return null;
        }
        return {
          label: libraryErrorPrompt.message,
          tone: "error",
          animation: "failed",
          actionLabel: libraryErrorPrompt.actionLabel,
          onAction:
            libraryErrorPrompt.action === "connections"
              ? props.onOpenConnections
              : libraryErrorPrompt.action === "refresh"
                ? reloadLibrary
                : undefined,
        };
      }
      if (normalizedQuery && searchLoading) {
        return { label: props.text.listen.searchLoading };
      }
      if (!normalizedQuery && libraryViewPhase === "loading") {
        return { label: props.text.listen.onlineLoading };
      }
      if (onlineSearchNotice) {
        return {
          label: onlineSearchNotice,
          tone:
            onlineSearchNotice === props.text.listen.searchEmpty
              ? "empty"
              : "error",
          animation:
            onlineSearchNotice === props.text.listen.searchEmpty
              ? "review"
              : "failed",
          actionLabel:
            onlineSearchNotice === props.text.listen.searchEmpty
              ? undefined
              : props.text.listen.retry,
          onAction:
            onlineSearchNotice === props.text.listen.searchEmpty
              ? undefined
              : refreshMusePage,
        };
      }
      if (!normalizedQuery && libraryViewPhase === "empty") {
        return {
          label: props.text.listen.onlineEmpty,
          animation: "review",
          actionLabel: props.text.listen.retry,
          onAction: reloadLibrary,
        };
      }
      return null;
    }
    if (mode === "linger" && effectiveSidebarView === "browse") {
      if (localTracksError && localTracks.length === 0) {
        return {
          label: props.text.listen.localLoadFailed,
          tone: "error",
          animation: "failed",
          actionLabel: props.text.listen.retry,
          onAction: retryLocalTracks,
        };
      }
      if (localWorkspaceRoute) {
        return null;
      }
      if (localTracksLoading) {
        return { label: props.text.listen.localLoading };
      }
      if (localTracks.length === 0) {
        return {
          label: props.text.listen.localEmptyPrompt,
          animation: "review",
          actionLabel: props.text.listen.localEmptyAction,
          onAction: () => props.onDownloadTrack(""),
        };
      }
      if (filteredLocalTracks.length === 0) {
        return { label: props.text.listen.searchEmpty, animation: "review" };
      }
    }
    return null;
  })();
  const musePrimaryLoading =
    mode === "muse" &&
    effectiveSidebarView === "browse" &&
    museAccountConnected &&
    !showMuseAccountGate &&
    (museManualRefreshKind !== "" ||
      (showArtistDetail &&
        Boolean(artistBrowsePage?.loading || artistTrackListView.loading)) ||
      (showPlaylistDetail && playlistLoading) ||
      (Boolean(normalizedQuery) && searchLoading) ||
      (!normalizedQuery &&
        !showArtistDetail &&
        !showPlaylistDetail &&
        libraryViewPhase === "loading"));
  const musePrimaryLoadingLabel =
    pagePrompt?.label || props.text.listen.loading;
  const pagePromptIsError = pagePrompt?.tone === "error";
  const musePrimaryLoadingBackAction = showArtistDetail && artistBrowsePage
    ? selectedArtistTrackListShelf
      ? closeArtistTrackListShelf
      : closeArtistBrowse
    : showPlaylistDetail
      ? () => setBrowsePlaylistId("")
      : undefined;
  const workspaceSearchControl =
    workspacePageContract?.recipe === "search" ? (
      <WorkspaceSearchControl
        clearLabel={props.text.actions.clear}
        onClear={clearSearch}
        onSubmit={submitWorkspaceSearch}
        onValueChange={setWorkspaceSearchDraft}
        placeholder={searchPlaceholder}
        submitLabel={props.text.workspace.search}
        value={workspaceSearchDraft}
      />
    ) : undefined;
  const primaryStatusOverlay = musePrimaryLoading ? (
    <ListenPrimaryLoadingOverlay
      label={musePrimaryLoadingLabel}
      pet={props.pet}
      petImageURL={props.petImageURL}
      animation={pagePrompt?.animation}
      backLabel={
        musePrimaryLoadingBackAction ? props.text.actions.back : undefined
      }
      onBack={musePrimaryLoadingBackAction}
    />
  ) : pagePromptIsError && pagePrompt ? (
    <ListenPrimaryStatusOverlay
      kind="error"
      label={pagePrompt.label}
      pet={props.pet}
      petImageURL={props.petImageURL}
      animation={pagePrompt.animation}
      backLabel={
        musePrimaryLoadingBackAction ? props.text.actions.back : undefined
      }
      onBack={musePrimaryLoadingBackAction}
      actionLabel={pagePrompt.actionLabel}
      onAction={pagePrompt.onAction}
    />
  ) : null;

  return (
    <div
      className={cn(
        "listen-page-view min-h-0 min-w-0 flex-1 overflow-hidden",
        props.workspaceLayout
          ? "app-workspace-primary-subpane"
          : undefined,
        props.active ? "flex" : "hidden",
        props.className,
      )}
      data-listen-background={
        props.workspaceLayout
          ? "workspace"
          : hushFullscreen
            ? "transparent"
            : "sidebar"
      }
    >
      <aside
        aria-hidden={!listOpen}
        data-open={listOpen ? "true" : "false"}
        className={cn(
          "listen-list-surface app-workspace-primary-subpane relative flex min-h-0 overflow-hidden",
          listOpen &&
            !props.playerPortalTarget &&
            "app-workspace-primary-subpane--leading",
          listOpen
            ? "min-w-0 flex-1"
            : "pointer-events-none w-0 -translate-x-2",
        )}
      >
        {listOpen ? (
          <div className="listen-primary-viewport listen-primary-viewport-enter relative flex h-full w-full min-w-0 flex-col overflow-hidden">
            {isWindows && !props.workspaceLayout ? (
              <div
                className="wails-drag absolute inset-x-0 top-0 z-20 h-[var(--app-page-top-drag-height)]"
                aria-hidden="true"
              />
            ) : null}
            {!props.workspaceLayout ? (
            <div className="pointer-events-none absolute inset-x-0 top-0 z-30 px-4 pb-10 pt-3">
              <div
                ref={headerRef}
                data-search-state={searchToolbarState}
                className="listen-list-toolbar wails-drag pointer-events-auto relative w-full min-w-0"
              >
                <div className="listen-list-toolbar-primary flex min-w-0 items-center justify-start gap-2 overflow-hidden">
                  {(!props.workspaceLayout || props.workspaceRouteId === "search") &&
                  !hideModeTabsForSearch ? (
                    <TooltipProvider delayDuration={0}>
                      <ListenModeTabs
                        mode={mode}
                        compact={tabsCompact}
                        text={props.text}
                        labels={
                          props.workspaceLayout
                            ? {
                                hush: props.text.workspace.lofi,
                                muse: props.text.workspace.youtubeMusic,
                                linger: props.text.workspace.local,
                              }
                            : undefined
                        }
                        onChange={setMode}
                      />
                    </TooltipProvider>
                  ) : null}
                  {museAccountMenuVisible ? (
                    <ListenMuseAccountMenu
                      text={props.text}
                      name={museAccountName}
                      avatarURL={museAccountAvatarURL}
                      connected={museAccountConnected}
                      busy={museAccountBusy}
                      onConnect={connectYouTube}
                      onRefresh={refreshMusePage}
                      onSignOut={signOutMuseAccount}
                    />
                  ) : null}
                  {headerActionGroup && !hideHeaderActionsForSearch ? (
                    <div className="listen-list-toolbar-actions min-w-0 shrink-0">
                      {headerActionGroup}
                    </div>
                  ) : null}
                </div>
                <div className="listen-list-toolbar-search-layer wails-no-drag absolute inset-y-0 right-0 z-10 flex min-w-0 items-center justify-end">
                  <div
                    className={cn(
                      "listen-list-search-control app-dream-search-control app-dream-control-shell app-completed-search-control h-9",
                      searchInputActive ? "px-3" : "justify-center px-0",
                    )}
                    onMouseDown={(event) => {
                      if (searchInputActive) {
                        return;
                      }
                      event.preventDefault();
                      activateSearchInput();
                    }}
                    onKeyDown={(event) => {
                      if (searchInputActive) {
                        return;
                      }
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        activateSearchInput();
                      }
                    }}
                    role={searchInputActive ? undefined : "button"}
                    tabIndex={searchInputActive ? undefined : 0}
                  >
                    <Search className="listen-list-search-icon h-4 w-4 shrink-0" />
                    {searchFieldMounted ? (
                      <div className="listen-list-search-field flex min-w-0 flex-1 items-center gap-2">
                        <Input
                          ref={searchInputRef}
                          value={query}
                          onChange={handleSearchChange}
                          onFocus={() => setSearchFocused(true)}
                          onBlur={handleSearchBlur}
                          onKeyDown={(event) => {
                            if (event.key === "Enter") {
                              event.preventDefault();
                              selectFirstResult();
                            }
                          }}
                          placeholder={searchPlaceholder}
                          size="compact"
                          tabIndex={searchInputActive ? 0 : -1}
                          aria-hidden={!searchInputActive}
                          className="listen-list-search-input app-control-input-compact h-auto min-w-0 flex-1 px-0"
                        />
                        <span
                          className={cn(
                            "listen-search-clear-reveal block shrink-0 overflow-hidden",
                            searchHasText ? "w-5 translate-x-0" : "w-0 -translate-x-1",
                          )}
                          data-visible={searchHasText ? "true" : "false"}
                        >
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            shape="circle"
                            aria-label={props.text.actions.clear}
                            title={props.text.actions.clear}
                            disabled={!searchHasText}
                            tabIndex={searchHasText ? 0 : -1}
                            className="listen-search-clear h-5 w-5"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={clearSearch}
                          >
                            <X className="h-3.5 w-3.5" />
                          </Button>
                        </span>
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
            </div>
            ) : null}

          <ListenPrimaryPageFrame
            contract={workspacePageContract}
            reserveWindowControls={props.reserveWindowControls === true}
            loading={musePrimaryLoading}
            covered={pagePromptIsError}
            topBarActions={workspaceTopBarActions}
            searchControl={workspaceSearchControl}
            overlay={primaryStatusOverlay}
          >
            <div className="listen-primary-content-region">
            {mode === "linger" && localTracksError && localTracks.length > 0 ? (
              <div
                aria-live="assertive"
                className="listen-status-panel mx-1 mb-3 flex items-center justify-between gap-3 px-3 py-2"
                data-tone="error"
                role="alert"
              >
                <span>{props.text.listen.localLoadFailed}</span>
                <Button
                  variant="ghost"
                  size="compact"
                  shape="capsule"
                  tone="destructive"
                  className="listen-page-error-action shrink-0 px-2 py-1"
                  onClick={retryLocalTracks}
                  type="button"
                >
                  {props.text.listen.retry}
                </Button>
              </div>
            ) : null}
            {!showMuseAccountGate &&
            pagePrompt &&
            !pagePromptIsError &&
            !musePrimaryLoading ? (
              <ListenMuseLoadingIndicator
                pet={props.pet}
                petImageURL={props.petImageURL}
                label={pagePrompt.label}
                animation={pagePrompt.animation}
                actionLabel={pagePrompt.actionLabel}
                onAction={pagePrompt.onAction}
              />
            ) : null}
            {showMuseAccountGate ? (
              <ListenMuseAccountGate
                text={props.text}
                pet={props.pet}
                petImageURL={props.petImageURL}
                busy={museConnectBusy}
                onConnect={connectYouTube}
              />
            ) : effectiveSidebarView === "queue" ? (
              mode === "hush" ? (
                <div className="space-y-5">
                  <ListenOnlineGroup
                    title={props.text.listen.upNext}
                    hideTitle
                    items={curatedLiveItems}
                    selectedId={liveSelectionArmed ? selectedLiveId : ""}
                    httpBaseURL={props.httpBaseURL}
                    text={props.text}
                    liveStatuses={liveStatusByVideoId}
                    onSelect={activateLiveSelection}
                  />
                </div>
              ) : mode === "muse" ? (
                <div className="space-y-5">
                  {filteredOnlineQueueItems.length > 0 ? (
                    <ListenOnlineGroup
                      title={onlineQueueTitle}
                      items={filteredOnlineQueueItems}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onClear={clearOnlineQueue}
                      clearLabel={props.text.listen.clearQueue}
                      onUndo={undoOnlineQueueEdit}
                      undoDisabled={!onlineQueueCanUndo}
                      onRedo={redoOnlineQueueEdit}
                      redoDisabled={!onlineQueueCanRedo}
                      onRemove={removeOnlineQueueItem}
                      removeLabel={props.text.listen.removeFromQueue}
                      onMove={moveOnlineQueueItem}
                      onSelect={selectOnlineQueueTrack}
                    />
                  ) : null}
                </div>
              ) : localTracksLoading ? (
                null
              ) : visibleLocalQueue.length > 0 ? (
                <ListenLocalTrackGroupList
                  httpBaseURL={props.httpBaseURL}
                  tracks={visibleLocalQueue}
                  selectedId={activeLocalSelectedId}
                  text={props.text}
                  onMetadataSaved={refreshLocalTracks}
                  onSelect={selectLocalQueueTrack}
                />
              ) : null
            ) : mode === "hush" ? (
              <ListenHushLiveList
                httpBaseURL={props.httpBaseURL}
                text={props.text}
                liveGroups={liveGroups}
                liveStatusByVideoId={liveStatusByVideoId}
                liveCatalogLoading={liveCatalogLoading}
                liveCatalogError={liveCatalogError}
                liveCatalogMessage={liveCatalogMessage}
                liveUserCatalog={liveUserCatalog}
                liveUserCatalogLoading={liveUserCatalogLoading}
                liveUserCatalogSaving={liveUserCatalogSaving}
                liveUserCatalogError={liveUserCatalogError}
                curatedLiveItems={curatedLiveItems}
                liveSelectionArmed={liveSelectionArmed}
                selectedLiveId={selectedLiveId}
                normalizedQuery={normalizedQuery}
                liveSearchNotice={liveSearchNotice}
                onReloadCatalog={reloadLiveCatalog}
                onSaveUserCatalog={saveLiveUserCatalog}
                onSelect={activateLiveSelection}
              />
            ) : mode === "muse" ? (
              showArtistDetail && artistBrowsePage ? (
                <div className="listen-muse-artist-detail space-y-4">
                  <ListenMuseArtistHero
                    httpBaseURL={props.httpBaseURL}
                    title={artistHeaderTitle}
                    subtitle={artistHeaderSubtitle}
                    description={
                      selectedArtistTrackListShelf
                        ? undefined
                        : artistBrowsePage.description
                    }
                    thumbnailUrl={artistBrowsePage.thumbnailUrl}
                    heroThumbnailUrl={artistBrowsePage.heroThumbnailUrl}
                    backLabel={props.text.actions.back}
                    infoLabel={props.text.listen.artistInfo}
                    biographyLabel={props.text.listen.artistBiography}
                    closeLabel={props.text.actions.close}
                    shuffleLabel={props.text.listen.artistShuffle}
                    mixLabel={props.text.listen.artistMix}
                    subscribeLabel={props.text.listen.artistSubscribe}
                    unsubscribeLabel={props.text.listen.artistUnsubscribe}
                    showActions={!selectedArtistTrackListShelf}
                    subscribed={artistBrowsePage.isSubscribed}
                    actionBusy={artistActionBusy}
                    shuffleDisabled={
                      artistBrowsePage.loading ||
                      artistBrowsePage.items.length === 0 ||
                      artistActionBusy !== ""
                    }
                    mixDisabled={
                      artistBrowsePage.loading ||
                      !artistBrowsePage.mixPlaylistId ||
                      artistActionBusy !== ""
                    }
                    subscribeDisabled={
                      artistBrowsePage.loading ||
                      !artistBrowsePage.channelId ||
                      artistActionBusy !== ""
                    }
                    onBack={
                      selectedArtistTrackListShelf
                        ? closeArtistTrackListShelf
                        : closeArtistBrowse
                    }
                    onShuffle={shuffleArtist}
                    onMix={playArtistMix}
                    onToggleSubscription={toggleArtistSubscription}
                  />
                  {artistBrowsePage.loading ? null : artistBrowsePage.error ? (
                    null
                  ) : selectedArtistTrackListShelf ? (
                    <ListenMuseTrackList
                      items={selectedArtistTrackListTracks}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      onSelect={playArtistTrack}
                    />
                  ) : filteredArtistShelves.length > 0 ? (
                    <div className="space-y-5">
                      {filteredArtistShelves.map((shelf) => {
                        if (shelf.kind === "artists") {
                          return (
                            <ListenMuseArtistGroup
                              key={shelf.id}
                              title={resolveListenShelfTitle(
                                shelf,
                                props.text,
                                props.text.listen.groupArtists,
                              )}
                              items={shelf.artists}
                              selectedArtistId={artistBrowsePage?.id}
                              httpBaseURL={props.httpBaseURL}
                              text={props.text}
                              onSelect={openSearchArtistBrowse}
                            />
                          );
                        }
                        if (shelf.kind === "playlists") {
                          return (
                            <ListenMusePlaylistGroup
                              key={shelf.id}
                              title={resolveListenShelfTitle(
                                shelf,
                                props.text,
                                props.text.listen.groupPlaylist,
                              )}
                              items={shelf.playlists}
                              selectedPlaylistId={browsePlaylistId}
                              httpBaseURL={props.httpBaseURL}
                              text={props.text}
                              onSelect={openPlaylistBrowse}
                            />
                          );
                        }
                        const shelfTitle = resolveListenShelfTitle(
                          shelf,
                          props.text,
                          props.text.listen.groupRecommendations,
                        );
                        if (isListenTopSongsShelf(shelf, shelfTitle, props.text)) {
                          return (
                            <ListenMuseTrackListGroup
                              key={shelf.id}
                              title={shelfTitle}
                              items={shelf.tracks}
                              selectedId={selectedOnlineId}
                              httpBaseURL={props.httpBaseURL}
                              maxItems={LISTEN_ARTIST_TOP_SONGS_PREVIEW_LIMIT}
                              text={props.text}
                              onSeeAll={() => openArtistTrackListShelf(shelf, shelfTitle)}
                              onSelect={playArtistTrack}
                            />
                          );
                        }
                        return (
                          <ListenMuseTrackGroup
                            key={shelf.id}
                            title={shelfTitle}
                            items={shelf.tracks}
                            selectedId={selectedOnlineId}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            onSelect={playArtistTrack}
                          />
                        );
                      })}
                    </div>
                  ) : filteredArtistTracks.length > 0 ? (
                    <ListenMuseTrackGroup
                      title={artistBrowsePage.title || artistBrowsePage.name}
                      items={filteredArtistTracks}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={playArtistTrack}
                    />
                  ) : null}
                  {!artistBrowsePage.loading &&
                  !artistBrowsePage.error &&
                  artistBrowsePage.continuation ? (
                    <ListenInfiniteScrollSentinel
                      key={`artist:${artistBrowsePage.id || artistBrowsePage.name}`}
                      continuation={artistBrowsePage.continuation}
                      enabled={!artistBrowsePage.error}
                      loading={artistBrowsePage.appending || artistBrowsePage.loading}
                      onLoadMore={loadMoreArtist}
                    />
                  ) : null}
                </div>
              ) : showPlaylistDetail ? (
                <div className="space-y-4">
                  <ListenMusicCollectionDetail
                    httpBaseURL={props.httpBaseURL}
                    collection={
                      selectedPlaylist
                        ? {
                            ...selectedPlaylist,
                            thumbnailUrl:
                              playlistDetailThumbnailURL.trim() ||
                              selectedPlaylist.thumbnailUrl,
                          }
                        : null
                    }
                    title={selectedPlaylistTitle}
                    typeLabel={selectedPlaylistTypeLabel}
                    author={selectedPlaylistAuthor}
                    description={selectedPlaylistDescription}
                    headerMetadata={selectedPlaylistHeaderMetadata}
                    showFooter={
                      !playlistLoading &&
                      !playlistAppending &&
                      !playlistContinuation
                    }
                    isAlbum={selectedPlaylistIsAlbum}
                    items={playlistLoading ? [] : playlistTracks}
                    allItems={playlistTracks}
                    selectedId={selectedOnlineId}
                    actionDisabled={playlistActionDisabled}
                    playbackBusy={onlinePlaybackActionPending}
                    libraryBusy={selectedPlaylistLibraryBusy}
                    saved={selectedPlaylistSaved}
                    text={props.text}
                    onBack={() => setBrowsePlaylistId("")}
                    onOpenAuthor={
                      selectedPlaylistIsAlbum &&
                      (playlistDetailAuthor.trim() ||
                        playlistDetailAuthorBrowseId.trim())
                        ? () =>
                            openSearchArtistBrowse({
                              id:
                                playlistDetailAuthorBrowseId.trim() ||
                                playlistDetailAuthor.trim(),
                              browseId: playlistDetailAuthorBrowseId.trim(),
                              name:
                                playlistDetailAuthor.trim() ||
                                selectedPlaylistAuthor,
                              subtitle: "",
                            })
                        : undefined
                    }
                    onPlay={() => playPlaylistFromIndex(0)}
                    onShuffle={shufflePlaylist}
                    onToggleLibrary={() => {
                      if (!selectedPlaylist) {
                        return;
                      }
                      updatePlaylistLibrary(
                        selectedPlaylist,
                        selectedPlaylistSaved ? "remove" : "add",
                      );
                    }}
                    onPlayNext={playPlaylistNext}
                    onAddToQueue={addPlaylistToQueue}
                    onSelectTrack={(item) => {
                      const index = playlistTracks.findIndex(
                        (track) => track.id === item.id,
                      );
                      if (index >= 0) {
                        playPlaylistFromIndex(index);
                      }
                    }}
                    onDownloadTrack={(item) =>
                      props.onDownloadTrack(
                        `https://music.youtube.com/watch?v=${encodeURIComponent(item.videoId)}`,
                      )
                    }
                    onOpenTrack={(item) => {
                      const url = `https://music.youtube.com/watch?v=${encodeURIComponent(item.videoId)}`;
                      void openExternalURL(url).catch((error) => {
                        console.warn("[Listen] open track page unavailable", {
                          url,
                          error,
                        });
                      });
                    }}
                  />
                  {!playlistLoading && playlistContinuation ? (
                    <ListenInfiniteScrollSentinel
                      key={`playlist:${browsePlaylistId}`}
                      continuation={playlistContinuation}
                      enabled={Boolean(browsePlaylistId)}
                      loading={playlistAppending || playlistLoading}
                      onLoadMore={loadMorePlaylist}
                    />
                  ) : null}
                </div>
              ) : !normalizedQuery && libraryViewPhase === "error" ? (
                null
              ) : (
                <div className="space-y-5">
                  {!normalizedQuery && onlineBrowseDetail ? (
                    <div className="relative z-[22] flex items-center justify-between gap-2 px-1">
                      <Button
                        type="button"
                        variant="ghost"
                        size="compactIcon"
                        className={cn(
                          "listen-browse-detail-back wails-no-drag h-8 w-8",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={props.text.actions.back}
                        title={props.text.actions.back}
                        onClick={closeOnlineBrowseDetail}
                      >
                        <ArrowLeft className="h-4 w-4" />
                      </Button>
                      <div className="min-w-0 flex-1">
                        <div className="listen-browse-detail-title truncate">
                          {onlineBrowseDetail.title}
                        </div>
                        <div className="listen-browse-detail-subtitle truncate">
                          {listenOnlineBrowseSourceLabel(
                            onlineBrowseDetail.source,
                            props.text,
                          )}
                        </div>
                      </div>
                    </div>
                  ) : null}
                  {normalizedQuery && searchSongsListOpen ? (
                    <>
                      <div className="relative z-[22] flex items-center justify-between gap-2 px-1">
                        <Button
                          type="button"
                          variant="ghost"
                          size="compactIcon"
                          className={cn(
                            "listen-browse-detail-back wails-no-drag h-8 w-8",
                            LISTEN_CONTROL_ICON_BUTTON_CLASS,
                          )}
                          aria-label={props.text.actions.back}
                          title={props.text.actions.back}
                          onClick={() => setSearchSongsListOpen(false)}
                        >
                          <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <div className="min-w-0 flex-1">
                          <div className="listen-browse-detail-title listen-browse-detail-title--strong truncate">
                            {props.text.listen.searchSongs}
                          </div>
                          <div className="listen-browse-detail-subtitle truncate">
                            {query.trim()}
                          </div>
                        </div>
                      </div>
                      <ListenMuseTrackList
                        items={searchItems}
                        selectedId={selectedOnlineId}
                        httpBaseURL={props.httpBaseURL}
                        onSelect={playOnlineSearchTrack}
                      />
                      {searchContinuation ? (
                        <ListenInfiniteScrollSentinel
                          key={`search:${normalizedQuery}`}
                          continuation={searchContinuation}
                          enabled={normalizedQuery.length >= 2}
                          loading={searchAppending || searchLoading}
                          onLoadMore={loadMoreSearch}
                        />
                      ) : null}
                    </>
                  ) : (
                    <>
                      {normalizedQuery ? (
                        <ListenMuseTrackListGroup
                          title={props.text.listen.searchSongs}
                          items={searchItems}
                          selectedId={selectedOnlineId}
                          httpBaseURL={props.httpBaseURL}
                          maxItems={LISTEN_SEARCH_SONGS_PREVIEW_LIMIT}
                          text={props.text}
                          onPlayAll={playOnlineSearchResults}
                          onShuffle={
                            searchItems.length > 1
                              ? shuffleOnlineSearchResults
                              : undefined
                          }
                          onSeeAll={() => setSearchSongsListOpen(true)}
                          onSelect={playOnlineSearchTrack}
                        />
                      ) : null}
                      {normalizedQuery ? (
                        <ListenMuseArtistGroup
                          title={props.text.listen.searchArtists}
                          items={searchArtists}
                          selectedArtistId={artistBrowsePage?.id}
                          httpBaseURL={props.httpBaseURL}
                          text={props.text}
                          onSelect={openSearchArtistBrowse}
                        />
                      ) : null}
                      {normalizedQuery ? (
                        <ListenMusePlaylistGroup
                          title={props.text.listen.searchPlaylists}
                          items={searchPlaylists}
                          selectedPlaylistId={browsePlaylistId}
                          httpBaseURL={props.httpBaseURL}
                          text={props.text}
                          onSelect={openPlaylistBrowse}
                        />
                      ) : null}
                      {normalizedQuery && searchContinuation ? (
                        <ListenInfiniteScrollSentinel
                          key={`search:${normalizedQuery}`}
                          continuation={searchContinuation}
                          enabled={normalizedQuery.length >= 2}
                          loading={searchAppending || searchLoading}
                          onLoadMore={loadMoreSearch}
                        />
                      ) : null}
                    </>
                  )}
                  {!normalizedQuery &&
                  onlineBrowseSource === "home" &&
                  !onlineBrowseDetail ? (
                    <ListenMuseArtistGroup
                      title={props.text.listen.libraryArtists}
                      items={visibleLibraryArtists}
                      selectedArtistId={artistBrowsePage?.id}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={openSearchArtistBrowse}
                    />
                  ) : null}
                  {visibleLibraryPlaylistGroup && !searchSongsListOpen ? (
                    <ListenMusePlaylistGroup
                      title={props.text.listen.groupLibrary}
                      items={visibleLibraryPlaylists}
                      selectedPlaylistId={browsePlaylistId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={openPlaylistBrowse}
                    />
                  ) : null}
                  {!normalizedQuery
                    ? visibleHomeShelves.map((shelf) =>
                        shelf.kind === "artists" ? (
                          <ListenMuseArtistGroup
                            key={shelf.id}
                            title={resolveListenShelfTitle(
                              shelf,
                              props.text,
                              props.text.listen.groupArtists,
                            )}
                            items={shelf.artists}
                            selectedArtistId={artistBrowsePage?.id}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            onSelect={openSearchArtistBrowse}
                          />
                        ) : shelf.kind === "categories" ? (
                          <ListenMuseCategoryGroup
                            key={shelf.id}
                            title={resolveListenShelfTitle(
                              shelf,
                              props.text,
                              props.text.listen.groupCategories,
                            )}
                            items={shelf.categories}
                            selectedCategoryId={onlineBrowseDetail?.id}
                            text={props.text}
                            onSelect={openOnlineBrowseCategory}
                          />
                        ) : shelf.kind === "playlists" ? (
                          <ListenMusePlaylistGroup
                            key={shelf.id}
                            title={resolveListenShelfTitle(
                              shelf,
                              props.text,
                              props.text.listen.groupPlaylist,
                            )}
                            items={shelf.playlists}
                            selectedPlaylistId={browsePlaylistId}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            onSelect={openPlaylistBrowse}
                          />
                        ) : (
                          <ListenMuseTrackGroup
                            key={shelf.id}
                            title={resolveListenShelfTitle(
                              shelf,
                              props.text,
                              props.text.listen.groupRecommendations,
                            )}
                            items={shelf.tracks}
                            selectedId={selectedOnlineId}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            onPlayAll={() => playOnlineShelfAll(shelf)}
                            onShuffle={
                              shelf.tracks.length > 1
                                ? () => shuffleOnlineShelf(shelf)
                                : undefined
                            }
                            onSelect={(item) => playOnlineShelfTrack(shelf, item)}
                          />
                        ),
                      )
                    : null}
                  {autoLoadLibraryPage ? (
                    <ListenInfiniteScrollSentinel
                      key={`library:${libraryPaginationScope}`}
                      continuation={libraryContinuation}
                      enabled={autoLoadLibraryPage}
                      loading={libraryAppending || libraryLoading}
                      onLoadMore={loadMoreLibrary}
                    />
                  ) : null}
                </div>
              )
            ) : localWorkspaceRoute ? (
              <ListenLocalLibraryWorkspace
                hideHeader
                headerActionsTarget={workspaceTopBarActionsTarget}
                httpBaseURL={props.httpBaseURL}
                onPlayTrack={playLocalBrowseTrack}
                query={query}
                routeId={props.workspaceRouteId ?? ""}
                selectedId={activeLocalSelectedId}
                text={props.text}
                tracks={localTracks}
                tracksLoading={localTracksLoading}
                tracksRefreshing={localTracksRefreshing}
                tracksClearingMissing={localTracksClearingMissing}
                onRefreshTracks={refreshLocalTracks}
                onRepairMissingTracks={openRepairMissingLocalTracks}
              />
            ) : localTracksLoading ? (
              null
            ) : filteredLocalTracks.length > 0 ? (
              <ListenLocalTrackGroupList
                httpBaseURL={props.httpBaseURL}
                tracks={filteredLocalTracks}
                  selectedId={activeLocalSelectedId}
                  text={props.text}
                  onMetadataSaved={refreshLocalTracks}
                  onSelect={(item) =>
                    playLocalBrowseTrack(item, filteredLocalTracks)
                  }
                />
            ) : localTracks.length === 0 ? (
              null
            ) : null}
            </div>
          </ListenPrimaryPageFrame>
          </div>
        ) : null}
      </aside>

      <OptionalPortal target={props.playerPortalTarget}>
      <section
        data-player-presentation={playerPresentation}
        className={cn(
          "listen-content-surface app-workspace-primary-subpane relative flex h-full min-h-0 w-full min-w-0 overflow-hidden",
          hushFullscreen && "listen-content-surface-hush-fullscreen",
          presentationListOpen ? "w-[22rem] shrink-0 grow-0" : "flex-1",
        )}
      >
        {isWindows && playerPresentation === "page" ? (
          <div
            className="wails-drag absolute left-14 right-[var(--app-windows-caption-control-width)] top-0 z-20 h-[var(--app-page-top-drag-height)]"
            aria-hidden="true"
          />
        ) : null}
        {showContentListToggle ? (
          <div
            className={cn(
              "pointer-events-none absolute left-3 top-3 z-30",
              isWindows ? "right-36" : "right-3",
            )}
          >
            <div className="listen-list-control-surface listen-list-control-surface-top pointer-events-auto inline-flex h-9 w-9 items-center justify-center p-0.5">
              <Button
                type="button"
                variant="ghost"
                size="compactIcon"
                className={cn(
                  "listen-list-toggle-button h-8 w-8 hover:scale-105 active:scale-95",
                  LISTEN_CONTROL_ICON_BUTTON_CLASS,
                )}
                aria-label={
                  listOpen
                    ? props.text.listen.collapseList
                    : props.text.listen.openList
                }
                title={
                  listOpen
                    ? props.text.listen.collapseList
                    : props.text.listen.openList
                }
                onClick={() => setListOpen((current) => !current)}
              >
                {listOpen ? (
                  <PanelLeftClose className="h-3.5 w-3.5" />
                ) : (
                  <PanelLeftOpen className="h-3.5 w-3.5" />
                )}
              </Button>
            </div>
          </div>
        ) : null}
        <div
          className={cn(
            "min-h-0 flex-1 overflow-hidden px-0 pb-0 sm:px-0 sm:pb-0",
            playerPresentation !== "page" || hushFullscreen
              ? "pt-0 sm:pt-0"
              : "pt-14 sm:pt-16",
          )}
        >
          <ListenPlayback
                mode={playbackMode}
                active={playerSurfaceActive}
                presentation={playerPresentation}
                companionMode={
                  playerPresentation === "fullscreen"
                    ? "player"
                    : props.playerCompanionMode
                }
                workspaceFullscreen={
                  playerPresentation === "fullscreen" &&
                  props.workspaceLayout === true
                }
                presentationCommand={props.controlCommand}
                listOpen={presentationListOpen}
                onToggleList={() => setListOpen((current) => !current)}
                reserveWindowControls={isWindows}
                airPlaySupported={isMac}
                selectedOnline={activeOnline}
                selectedLocal={selectedLocal}
                httpBaseURL={props.httpBaseURL}
                onlineCommand={onlinePlayerCommand}
                onlinePlaybackEnabled={onlinePlaybackArmed}
                localCommand={localPlayerCommand}
                onlineQueueItems={onlineQueueItems}
                onlineQueueTitle={
                  playbackMode === "hush" ? props.text.listen.liveStations : onlineQueueTitle
                }
                selectedOnlineId={playbackMode === "hush" ? selectedLiveId : selectedOnlineId}
                localQueueItems={localPlaybackQueue}
                selectedLocalId={selectedLocalId}
                onlinePlaying={onlinePlaying}
                localPlaying={localPlaying}
                localResumeTime={selectedLocalResumeTime}
                onlineResumeTime={activeOnlineResumeTime}
                onlineProgress={onlineProgress}
                onlineState={onlineState}
                onlinePlaybackErrorCode={onlinePlaybackErrorCode}
                onlinePlaybackErrorMessage={onlinePlaybackErrorMessage}
                onlineObservedPlaybackAudioQuality={onlineObservedPlaybackAudioQuality}
                favoriteActive={activeOnlineFavorite}
                favoriteBusy={activeOnlineFavoriteBusy}
                pet={props.pet}
                petImageURL={props.petImageURL}
                localProgress={localProgress}
                muted={muted}
                volume={volume}
                playMode={playMode}
                text={props.text}
                onEnded={handlePlaybackEnded}
                onOnlinePlayingChange={setOnlinePlaying}
                onOnlineStateChange={setOnlineState}
                onOnlinePlaybackErrorCodeChange={setOnlinePlaybackErrorCode}
                onOnlinePlaybackErrorMessageChange={setOnlinePlaybackErrorMessage}
                onOnlineProgressChange={handleOnlineProgressChange}
                onOnlineNativeTrackChange={handleOnlineNativeTrackChange}
                onSelectOnlineQueueTrack={
                  playbackMode === "hush" ? activateLiveSelection : selectOnlineQueueTrack
                }
                onClearOnlineQueue={clearOnlineQueue}
                onRemoveOnlineQueueItem={removeOnlineQueueItem}
                onMoveOnlineQueueItem={moveOnlineQueueItem}
                onUndoOnlineQueueEdit={undoOnlineQueueEdit}
                onRedoOnlineQueueEdit={redoOnlineQueueEdit}
                onlineQueueCanUndo={onlineQueueCanUndo}
                onlineQueueCanRedo={onlineQueueCanRedo}
                onSelectLocalQueueTrack={selectLocalQueueTrack}
                onClearLocalQueue={clearLocalQueue}
                onRemoveLocalQueueItem={removeLocalQueueItem}
                onMoveLocalQueueItem={moveLocalQueueItem}
                onUndoLocalQueueEdit={undoLocalQueueEdit}
                onRedoLocalQueueEdit={redoLocalQueueEdit}
                localQueueCanUndo={localQueueCanUndo}
                localQueueCanRedo={localQueueCanRedo}
                onLocalPlayingChange={setLocalPlaying}
                onLocalProgressChange={handleLocalProgressChange}
                onLocalPlaybackIntent={() => setPlaybackSessionStarted(true)}
                onPrevious={playPrevious}
                onNext={playNext}
                onTogglePlayMode={togglePlayMode}
                onPlayModeChange={setPlayMode}
                onTogglePlayback={togglePlayback}
                onToggleMute={toggleMute}
                onVolumeChange={handleVolumeChange}
                onOpenPlaybackSource={props.onOpenPlaybackSource}
                onRequestPlayerFullscreen={props.onRequestPlayerFullscreen}
                onExitPlayerFullscreen={props.onExitPlayerFullscreen}
                onToggleFavorite={toggleOnlineFavorite}
                onOpenOnlineArtist={openOnlineArtistBrowse}
                onDownloadTrack={props.onDownloadTrack}
                onOpenLocalDirectory={openSelectedLocalDirectory}
              />
        </div>
      </section>
      </OptionalPortal>
    </div>
  );
}
