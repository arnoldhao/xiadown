import { ArrowLeft,BookmarkPlus,Compass,History,Home,Link2,ListEnd,ListStart,Loader2,LogOut,PanelLeftClose,PanelLeftOpen,Play,Radio,RefreshCw,Search,Shuffle,Sparkles,Tags,Trophy,UserCheck,UserPlus,UserRound,Wrench,X } from "lucide-react";
import * as React from "react";
import { siYoutube } from "simple-icons";

import { WindowControls } from "@/components/layout/WindowControls";
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
import { SidebarMenuItem } from "@/shared/ui/sidebar";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
import {
LISTEN_CONTROL_ICON_BUTTON_CLASS,
} from "@/shared/styles/listen";

import type { ListenLiveUserCatalog } from "@/app/main/listen/api";
import { LISTEN_LIKED_SONGS_SHELF_ID } from "@/app/main/listen/catalog";
import { resolveListenLibraryErrorPrompt } from "@/app/main/listen/error-prompts";
import { ListenHushLiveActionGroup,ListenHushLiveList } from "@/app/main/listen/HushLiveList";
import { ListenPlayback } from "@/app/main/listen/Playback";
import { buildListenImageCandidates,buildListenTrackThumbnailCandidates } from "@/app/main/listen/storage";
import type { ListenArtistBrowseState,ListenArtistItem,ListenCategoryItem,ListenLibraryShelf,ListenLiveGroup,ListenLiveStatus,ListenLocalItem,ListenMode,ListenNativePlayerEvent,ListenObservedPlaybackAudioQuality,ListenOnlineBrowseDetail,ListenOnlineBrowseSource,ListenOnlineItem,ListenPageProps,ListenPlayMode,ListenPlaybackProgressState,ListenPlayerCommand,ListenPlaylistItem,ListenPlaylistLibraryAction,ListenRemotePlaybackState,ListenSidebarView } from "@/app/main/listen/types";
import { ListenAvatar,ListenLocalArtwork,ListenModeTabs,ListenMuseArtistGroup,ListenMuseCategoryGroup,ListenMusePlaylistGroup,ListenMuseTrackGroup,ListenMuseTrackList,ListenMuseTrackListGroup,ListenOnlineGroup } from "@/app/main/listen/ui";

type SetState<T> = React.Dispatch<React.SetStateAction<T>>;

const LISTEN_HOME_IMAGE_PREFETCH_LIMIT = 48;
const LISTEN_HOME_IMAGE_PREFETCH_CONCURRENCY = 4;
const LISTEN_HEADER_GAP_REM = 0.5;
const LISTEN_HEADER_SEARCH_EXPANDED_REM = 12;
const LISTEN_HEADER_ACCOUNT_BUTTON_REM = 2.25;
const LISTEN_HEADER_FULL_TABS_REM = 14.625;
const LISTEN_HEADER_COMPACT_TABS_REM = 6.75;
const LISTEN_HEADER_HUSH_ACTIONS_REM = 6.25;
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
  playlistTracks: ListenOnlineItem[];
  filteredPlaylistTracks: ListenOnlineItem[];
  playlistContinuation: string;
  normalizedQuery: string;
  libraryLoading: boolean;
  libraryAppending: boolean;
  libraryError: boolean;
  libraryErrorCode: string;
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
  localTracksLoading: boolean;
  localTracksRefreshing: boolean;
  localTracksClearingMissing: boolean;
  activeOnline: ListenOnlineItem | null;
  selectedLocal: ListenLocalItem | null;
  onlinePlayerCommand: ListenPlayerCommand | null;
  localPlayerCommand: ListenPlayerCommand | null;
  onlineQueueItems: ListenOnlineItem[];
  onlinePlaying: boolean;
  onlinePlaybackArmed: boolean;
  selectedLocalResumeTime: number;
  activeOnlineResumeTime: number;
  onlineProgress: ListenPlaybackProgressState & { videoId: string };
  onlineState: ListenRemotePlaybackState;
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
  openRepairMissingLocalTracks: () => void;
  handlePlaybackEnded: () => void;
  setOnlinePlaying: (playing: boolean) => void;
  setOnlineState: (state: ListenRemotePlaybackState) => void;
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

function resolveListenPlaylistHeaderMetadata(options: {
  playlist: ListenPlaylistItem | null | undefined;
  detailAuthor: string;
  loadedTrackCount: number;
  hasMoreTracks: boolean;
  text: ListenPageProps["text"];
}) {
  const playlist = options.playlist;
  if (!playlist) {
    return "";
  }
  const typeLabel = resolveListenPlaylistTypeLabel(playlist, options.text);
  const authorLabel = resolveUsefulListenPlaylistAuthor(
    playlist,
    options.detailAuthor,
    options.text,
  );
  const countLabel =
    options.loadedTrackCount > 0
      ? formatListenPlaylistTrackCount(
          options.loadedTrackCount,
          options.hasMoreTracks,
          options.text,
        )
      : "";
  return [typeLabel, authorLabel, countLabel].filter(Boolean).join(" · ");
}

function resolveListenPlaylistTypeLabel(
  playlist: ListenPlaylistItem,
  text: ListenPageProps["text"],
) {
  const normalized = playlist.channel.trim().toLocaleLowerCase();
  const playlistId = playlist.playlistId.trim();
  if (
    playlistId.startsWith("MPRE") ||
    playlistId.startsWith("OLAK") ||
    normalized === "album" ||
    normalized === "专辑" ||
    normalized === "專輯"
  ) {
    return text.listen.playlistTypeAlbum;
  }
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
  clearingMissing: boolean;
  onRepairMissing: () => void;
}) {
  return (
    <div className="app-dream-button-group app-completed-toolbar-actions inline-flex h-9 shrink-0 items-center p-0.5">
      <ListenHeaderActionButton
        label={props.text.completed.relinkDialogTitle}
        disabled={props.clearingMissing}
        onClick={props.onRepairMissing}
      >
        {props.clearingMissing ? (
          <Loader2 className="h-4 w-4 animate-spin" />
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
  text: ListenPageProps["text"];
  onSelect: (item: ListenLocalItem) => void;
}) {
  const groups = React.useMemo(
    () => buildListenLocalModifiedGroups(props.tracks, props.text),
    [props.text, props.tracks],
  );
  return (
    <div className="listen-local-file-list space-y-5">
      {groups.map((group) => (
        <section key={group.id} className="min-w-0 space-y-2">
          <div className="listen-local-file-group-header wails-drag flex items-center justify-between gap-3 px-2 text-xs font-semibold">
            <span className="min-w-0 truncate">{group.title}</span>
            <span className="listen-local-file-group-count shrink-0 tabular-nums">
              {group.items.length}
            </span>
          </div>
          <div className="space-y-1.5">
            {group.items.map(({ index, track }) => {
              const selected = track.id === props.selectedId;
              return (
                <SidebarMenuItem key={track.id}>
                  <button
                    type="button"
                    data-active={selected ? "true" : "false"}
                    className="listen-local-file-card grid min-h-14 w-full grid-cols-[2rem_minmax(0,1fr)_3.25rem] items-center gap-2 px-2 py-2 text-left focus-visible:outline-none"
                    onClick={() => {
                      props.onSelect(track);
                    }}
                  >
                    <span className="listen-local-file-index flex h-7 w-7 shrink-0 items-center justify-center text-[11px] font-semibold tabular-nums">
                      {index}
                    </span>
                    <div className="flex min-w-0 items-center gap-2">
                      <ListenLocalArtwork
                        track={track}
                        className={cn(
                          "listen-local-file-artwork rounded-xl bg-muted ring-border/70",
                          selected && "ring-primary/30",
                        )}
                      />
                      <div className="min-w-0 flex-1">
                        <div className="listen-local-file-title truncate text-sm font-medium">
                          {track.title}
                        </div>
                        <div className="listen-local-file-subtitle truncate text-xs">
                          {track.author || props.text.listen.linger}
                        </div>
                      </div>
                    </div>
                    <span className="listen-local-file-duration justify-self-end text-right text-[11px] font-medium tabular-nums">
                      {track.durationLabel}
                    </span>
                  </button>
                </SidebarMenuItem>
              );
            })}
          </div>
        </section>
      ))}
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
            glowClassName="opacity-0"
          />
          <span className="min-w-0 text-xs font-medium leading-4 text-sidebar-foreground/62">
            {props.label}
          </span>
        </div>
        {hasAction ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 max-w-full justify-center rounded-full px-3 text-[11px] font-semibold text-sidebar-primary hover:bg-sidebar-primary/10"
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
              <button
                type="button"
                className="listen-muse-account-trigger pointer-events-auto"
                aria-label={props.text.listen.museAccountTooltip}
              >
                <UserRound className="h-3.5 w-3.5" strokeWidth={1.65} />
              </button>
            </DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            {props.text.listen.museAccountTooltip}
          </TooltipContent>
        </Tooltip>
      </TooltipProvider>
      <DropdownMenuContent align="center" side="bottom" className="w-64 p-3">
        <div className="flex min-w-0 flex-col items-center text-center">
          <div
            className={cn(
              "grid h-20 w-20 place-items-center overflow-hidden rounded-full shadow-[inset_0_0_0_1px_hsl(var(--sidebar-primary)/0.16)]",
              props.connected
                ? "bg-sidebar-primary/10 text-sidebar-primary"
                : "bg-red-500/10 text-red-600 dark:text-red-400",
            )}
          >
            {showAvatar ? (
              <img
                src={avatarURL}
                alt=""
                className="h-full w-full rounded-full object-cover"
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
          <div className="mt-3 max-w-full truncate text-sm font-semibold text-sidebar-foreground">
            {name}
          </div>
        </div>
        {props.connected ? (
          <div className="mt-4 grid grid-cols-2 gap-2">
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="h-9 min-w-0 justify-center px-2"
              disabled={props.busy}
              onClick={handleRefresh}
            >
              <RefreshCw className={cn("h-3.5 w-3.5", props.busy ? "animate-spin" : "")} />
              <span className="min-w-0 truncate">{props.text.listen.refresh}</span>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="compact"
              className="h-9 min-w-0 justify-center px-2 text-destructive hover:text-destructive"
              disabled={props.busy}
              onClick={handleSignOut}
            >
              {props.busy ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
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
            className="mx-auto mt-4 flex h-9 w-auto min-w-44 justify-center px-4"
            disabled={props.busy}
            onClick={handleConnect}
          >
            {props.busy ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
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
      <div className="flex w-full max-w-[22rem] flex-col items-center text-center">
        <PetDisplay
          pet={props.pet}
          imageUrl={props.petImageURL}
          animation={props.busy ? "waiting" : "waving"}
          alt=""
          size={112}
          className="mb-5 h-28 w-28 shrink-0"
          glowClassName="opacity-0"
        />
        <h2 className="max-w-full truncate text-xl font-semibold tracking-normal text-sidebar-foreground">
          {props.text.listen.museGateTitle}
        </h2>

        <div className="mt-8 flex w-full flex-col items-center gap-3">
          <Button
            type="button"
            disabled={props.busy}
            className="h-10 w-auto min-w-44 px-5"
            onClick={props.onConnect}
          >
            {props.busy ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Link2 className="h-4 w-4" />
            )}
            {props.text.listen.museGateSignIn}
          </Button>
          <p className="max-w-[18rem] text-xs leading-5 text-sidebar-foreground/56">
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
  const { isWindows, isMac, listOpen, query, searchPlaceholder, mode, playbackMode, effectiveSidebarView, onlineBrowseSource, onlineBrowseDetail, liveGroups, liveStatusByVideoId, liveCatalogLoading, liveCatalogError, liveCatalogMessage, liveUserCatalog, liveUserCatalogLoading, liveUserCatalogSaving, liveUserCatalogError, curatedLiveItems, liveSelectionArmed, selectedLiveId, filteredOnlineQueueItems, onlineQueueTitle, onlineQueueCanUndo, onlineQueueCanRedo, selectedOnlineId, filteredLocalTracks, selectedLocalId, localPlaying, liveSearchNotice, showArtistDetail, artistBrowsePage, artistActionBusy, filteredArtistShelves, browsePlaylistId, savedPlaylistIds, playlistMutationAction, playlistMutationPlaylistId, filteredArtistTracks, showPlaylistDetail, selectedPlaylist, playlistLoading, playlistAppending, playlistDetailAuthor, playlistTracks, filteredPlaylistTracks, playlistContinuation, normalizedQuery, libraryLoading, libraryAppending, libraryError, libraryErrorCode, searchLoading, searchAppending, searchItems, searchArtists, searchPlaylists, searchContinuation, libraryArtists, displayedLibraryPlaylists, showLibraryPlaylistGroup, homeShelves, libraryContinuation, onlineSearchNotice, localTracks, localTracksLoading, localTracksClearingMissing, activeOnline, selectedLocal, onlinePlayerCommand, localPlayerCommand, onlineQueueItems, onlinePlaying, onlinePlaybackArmed, selectedLocalResumeTime, activeOnlineResumeTime, onlineProgress, onlineState, onlineObservedPlaybackAudioQuality, activeOnlineFavorite, activeOnlineFavoriteBusy, localProgress, muted, volume, playMode, museConnectBusy, museAccountName, museAccountAvatarURL, museAccountConnected, museAccountBusy, museManualRefreshKind } = view.state;
  const { setListOpen, setQuery, selectFirstResult, setMode, setSidebarView, reloadLiveCatalog, saveLiveUserCatalog, reloadLibrary, changeOnlineBrowseSource, openOnlineBrowseCategory, closeOnlineBrowseDetail, loadMoreLibrary, activateLiveSelection, selectOnlineQueueTrack, selectLocalQueueTrack, closeArtistBrowse, playArtistFromIndex, shuffleArtist, loadMoreArtist, loadArtistShelfTracks, playArtistMix, toggleArtistSubscription, openPlaylistBrowse, updatePlaylistLibrary, setBrowsePlaylistId, playPlaylistFromIndex, playPlaylistNext, addPlaylistToQueue, loadMorePlaylist, playOnlineShelfTrack, playOnlineShelfAll, shuffleOnlineShelf, playOnlineSearchTrack, loadMoreSearch, openSearchArtistBrowse, clearOnlineQueue, removeOnlineQueueItem, moveOnlineQueueItem, undoOnlineQueueEdit, redoOnlineQueueEdit, openRepairMissingLocalTracks, handlePlaybackEnded, setOnlinePlaying, setOnlineState, handleOnlineProgressChange, handleOnlineNativeTrackChange, setLocalPlaying, handleLocalProgressChange, setPlaybackSessionStarted, connectYouTube, refreshMusePage, signOutMuseAccount, playPrevious, playNext, togglePlayMode, setPlayMode, togglePlayback, toggleMute, handleVolumeChange, toggleOnlineFavorite, openOnlineArtistBrowse, openSelectedLocalDirectory } = view.actions;
  const hushFullscreen = playbackMode === "hush" && !listOpen;
  const activeLocalSelectedId =
    playbackMode === "linger" && selectedLocal ? selectedLocalId : "";
  const showContentListToggle = !hushFullscreen;
  const libraryErrorPrompt = resolveListenLibraryErrorPrompt(libraryErrorCode, props.text);
  const [searchFocused, setSearchFocused] = React.useState(false);
  const [headerRef, headerWidth] = useMeasuredElementWidth<HTMLDivElement>();
  const searchInputRef = React.useRef<HTMLInputElement | null>(null);
  const imagePrefetchRef = React.useRef<HTMLImageElement[]>([]);
  const searchHasText = query.length > 0;
  const searchInputActive = searchFocused || searchHasText;
  const [searchControlMounted, setSearchControlMounted] =
    React.useState(searchInputActive);
  const headerActionWidthRem = resolveListenHeaderActionWidthRem(mode);
  const headerActionGroupVisible = headerActionWidthRem > 0;
  const playlistActionDisabled = playlistLoading || playlistTracks.length === 0;
  const selectedPlaylistSaved = selectedPlaylist
    ? savedPlaylistIds.has(selectedPlaylist.playlistId)
    : false;
  const selectedPlaylistIsAlbum = isListenAlbumPlaylistItem(selectedPlaylist);
  const playlistTrackArtistFallback = selectedPlaylistIsAlbum
    ? playlistDetailAuthor.trim() || selectedPlaylist?.description.trim() || ""
    : "";
  const selectedPlaylistHeaderMetadata = resolveListenPlaylistHeaderMetadata({
    playlist: selectedPlaylist,
    detailAuthor: playlistDetailAuthor,
    loadedTrackCount: playlistTracks.length,
    hasMoreTracks: Boolean(playlistContinuation),
    text: props.text,
  });
  const selectedPlaylistLibraryBusy = Boolean(
    selectedPlaylist &&
      playlistMutationAction !== null &&
      playlistMutationPlaylistId === selectedPlaylist.playlistId,
  );
  const [artistTrackListView, setArtistTrackListView] = React.useState<{
    shelfId: string;
    title: string;
    tracks: ListenOnlineItem[];
    loading: boolean;
  }>({ shelfId: "", title: "", tracks: [], loading: false });
  const [searchSongsListOpen, setSearchSongsListOpen] = React.useState(false);
  React.useEffect(() => {
    setArtistTrackListView({
      shelfId: "",
      title: "",
      tracks: [],
      loading: false,
    });
  }, [artistBrowsePage?.id, showArtistDetail]);
  React.useEffect(() => {
    setSearchSongsListOpen(false);
  }, [normalizedQuery]);
  const selectedArtistTrackListShelf = React.useMemo(() => {
    if (!artistTrackListView.shelfId) {
      return null;
    }
    const shelf = filteredArtistShelves.find(
      (item) => item.id === artistTrackListView.shelfId,
    );
    return shelf?.kind === "tracks" ? shelf : null;
  }, [artistTrackListView.shelfId, filteredArtistShelves]);
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
    ? artistDisplayName || "YouTube Music"
    : artistBrowsePage?.subtitle || "YouTube Music";
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
      setArtistTrackListView({
        shelfId: shelf.id,
        title,
        tracks: shelf.tracks,
        loading: true,
      });
      void loadArtistShelfTracks(shelf)
        .then((tracks) => {
          setArtistTrackListView((current) =>
            current.shelfId === shelf.id
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
            current.shelfId === shelf.id ? { ...current, loading: false } : current,
          );
        });
    },
    [loadArtistShelfTracks],
  );
  const museAccountMenuVisible = mode === "muse";
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

  const handleSearchChange = React.useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      const nextQuery = event.target.value;
      setQuery(nextQuery);
      setSearchFocused(true);
    },
    [setQuery],
  );

  const handleSearchBlur = React.useCallback(() => {
    if (!query.length) {
      setSearchFocused(false);
    }
  }, [query.length]);

  const clearSearch = React.useCallback(() => {
    setQuery("");
    setSearchFocused(false);
  }, [setQuery]);

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
    ) : mode === "linger" ? (
      <ListenLingerHeaderActionGroup
        text={props.text}
        clearingMissing={localTracksClearingMissing}
        onRepairMissing={openRepairMissingLocalTracks}
      />
    ) : mode === "muse" ? (
      <ListenOnlineSourceTabs
        sources={LISTEN_ONLINE_BROWSE_SOURCES}
        value={onlineBrowseSource}
        text={props.text}
        onChange={handleOnlineSourceTabChange}
      />
    ) : null;

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
      libraryArtists,
      displayedLibraryPlaylists,
      homeShelves,
    });
    return prefetchListenImages(props.httpBaseURL, urls, imagePrefetchRef);
  }, [
    displayedLibraryPlaylists,
    effectiveSidebarView,
    homeShelves,
    libraryArtists,
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
    (!museAccountConnected ||
      (!libraryLoading &&
        !normalizedQuery &&
        libraryError &&
        libraryErrorPrompt.action === "connections"));

  const pagePrompt: {
    label: string;
    animation?: PetAnimation;
    actionLabel?: string;
    onAction?: () => void;
  } | null = (() => {
    if (effectiveSidebarView === "queue") {
      if (mode === "linger" && localTracksLoading) {
        return { label: props.text.listen.localLoading };
      }
      if (
        (mode === "hush" && curatedLiveItems.length === 0) ||
        (mode === "muse" && filteredOnlineQueueItems.length === 0) ||
        (mode === "linger" && !localTracksLoading && filteredLocalTracks.length === 0)
      ) {
        return { label: props.text.listen.upNextEmpty, animation: "review" };
      }
      return null;
    }
    if (mode === "hush" && effectiveSidebarView === "browse") {
      if (liveUserCatalogError) {
        return { label: liveUserCatalogError, animation: "failed" };
      }
      if (liveCatalogLoading && liveGroups.length === 0) {
        return { label: props.text.listen.liveLoading };
      }
      if (liveCatalogError || liveGroups.length === 0) {
        return {
          label: liveCatalogError
            ? liveCatalogMessage || props.text.listen.liveUnavailable
            : props.text.listen.liveEmpty,
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
        return { label: props.text.listen.artistUnavailable, animation: "failed" };
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
        filteredPlaylistTracks.length === 0
      ) {
        return { label: props.text.listen.playlistEmpty, animation: "review" };
      }
      if (!normalizedQuery && !libraryLoading && libraryError) {
        if (showMuseAccountGate) {
          return null;
        }
        return {
          label: libraryErrorPrompt.message,
          animation: "failed",
          actionLabel: libraryErrorPrompt.actionLabel,
          onAction:
            libraryErrorPrompt.action === "connections"
              ? props.onOpenConnections
              : reloadLibrary,
        };
      }
      if (normalizedQuery && searchLoading) {
        return { label: props.text.listen.searchLoading };
      }
      if (!normalizedQuery && libraryLoading) {
        return { label: props.text.listen.onlineLoading };
      }
      if (onlineSearchNotice) {
        return {
          label: onlineSearchNotice,
          animation:
            onlineSearchNotice === props.text.listen.searchEmpty
              ? "review"
              : "failed",
        };
      }
      if (
        !normalizedQuery &&
        !libraryLoading &&
        !libraryError &&
        homeShelves.length === 0 &&
        libraryArtists.length === 0 &&
        (!showLibraryPlaylistGroup || displayedLibraryPlaylists.length === 0)
      ) {
        return { label: props.text.listen.onlineEmpty, animation: "review" };
      }
      return null;
    }
    if (mode === "linger" && effectiveSidebarView === "browse") {
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

  return (
    <div
      className={cn(
        "listen-page-view min-h-0 min-w-0 flex-1 overflow-hidden",
        hushFullscreen ? "bg-transparent" : "bg-sidebar-background",
        props.active ? "flex" : "hidden",
        props.className,
      )}
    >
      <aside
        aria-hidden={!listOpen}
        className={cn(
          "listen-list-surface relative flex min-h-0 overflow-hidden backdrop-blur-2xl transition-[width,opacity,transform,border-color,box-shadow,flex-basis] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)]",
          listOpen
            ? "min-w-0 flex-1 border-r border-[hsl(var(--foreground)/0.08)] opacity-100 shadow-[inset_-1px_0_0_hsl(var(--background)/0.14)]"
            : "pointer-events-none w-0 -translate-x-2 border-r-0 border-transparent opacity-0 shadow-none",
        )}
      >
        {listOpen ? (
          <div className="relative flex h-full w-full min-w-0 flex-col overflow-hidden animate-in fade-in-0 slide-in-from-left-2 duration-300">
            {isWindows ? (
              <div
                className="wails-drag absolute inset-x-0 top-0 z-20 h-[var(--app-page-top-drag-height)]"
                aria-hidden="true"
              />
            ) : null}
            <div className="pointer-events-none absolute inset-x-0 top-0 z-30 px-4 pb-10 pt-3">
              <div
                ref={headerRef}
                data-search-state={searchToolbarState}
                className="listen-list-toolbar wails-drag pointer-events-auto relative w-full min-w-0"
              >
                <div className="listen-list-toolbar-primary flex min-w-0 items-center justify-start gap-2 overflow-hidden">
                  {!hideModeTabsForSearch ? (
                    <TooltipProvider delayDuration={0}>
                      <ListenModeTabs mode={mode} compact={tabsCompact} text={props.text} onChange={setMode} />
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
                      searchInputActive ? "px-3" : "cursor-text justify-center px-0",
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
                    <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
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
                          className="app-control-input-compact h-auto min-w-0 flex-1 rounded-none border-0 bg-transparent px-0 shadow-none"
                        />
                        <span
                          className={cn(
                            "block shrink-0 overflow-hidden transition-[width,opacity,transform] duration-200 ease-out",
                            searchHasText
                              ? "w-5 translate-x-0 opacity-100"
                              : "w-0 -translate-x-1 opacity-0",
                          )}
                        >
                          <button
                            type="button"
                            aria-label={props.text.actions.clear}
                            title={props.text.actions.clear}
                            disabled={!searchHasText}
                            tabIndex={searchHasText ? 0 : -1}
                            className="app-completed-search-clear flex h-5 w-5 items-center justify-center transition focus-visible:outline-none disabled:pointer-events-none"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={clearSearch}
                          >
                            <X className="h-3.5 w-3.5" />
                          </button>
                        </span>
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
            </div>

          <div
            className={cn(
              "min-h-0 flex-1 overflow-y-auto px-3 pt-[4.75rem] animate-in fade-in-0 slide-in-from-bottom-1 duration-200",
              "pb-4",
            )}
          >
            {!showMuseAccountGate && pagePrompt ? (
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
              ) : filteredLocalTracks.length > 0 ? (
                <ListenLocalTrackGroupList
                  tracks={filteredLocalTracks}
                  selectedId={activeLocalSelectedId}
                  text={props.text}
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
                <div className="space-y-4">
                  <div className="px-1 py-1">
                    <div className="flex items-center justify-between gap-2">
                      <Button
                        type="button"
                        variant="ghost"
                        size="compactIcon"
                        className={cn(
                          "h-8 w-8 shrink-0 rounded-xl",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={props.text.actions.back}
                        title={props.text.actions.back}
                        onClick={
                          selectedArtistTrackListShelf
                            ? () =>
                                setArtistTrackListView({
                                  shelfId: "",
                                  title: "",
                                  tracks: [],
                                  loading: false,
                                })
                            : closeArtistBrowse
                        }
                      >
                        <ArrowLeft className="h-4 w-4" />
                      </Button>
                      <ListenAvatar
                        httpBaseURL={props.httpBaseURL}
                        item={{
                          channel: artistDisplayName,
                          thumbnailUrl: artistBrowsePage.thumbnailUrl,
                        }}
                        shape="circle"
                      />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-semibold text-sidebar-foreground">
                          {artistHeaderTitle}
                        </div>
                        <div className="truncate text-xs text-sidebar-foreground/58">
                          {artistHeaderSubtitle}
                        </div>
                      </div>
                      <div className="app-dream-button-group app-completed-toolbar-actions inline-flex h-9 shrink-0 items-center p-0.5">
                        <ListenHeaderActionButton
                          label={props.text.listen.artistShuffle}
                          disabled={
                            artistBrowsePage.loading ||
                            artistBrowsePage.items.length === 0 ||
                            artistActionBusy !== ""
                          }
                          onClick={shuffleArtist}
                        >
                          <Shuffle className="h-4 w-4" />
                        </ListenHeaderActionButton>
                        <ListenHeaderActionButton
                          label={props.text.listen.artistMix}
                          disabled={
                            artistBrowsePage.loading ||
                            !artistBrowsePage.mixPlaylistId ||
                            artistActionBusy !== ""
                          }
                          onClick={playArtistMix}
                        >
                          {artistActionBusy === "mix" ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Radio className="h-4 w-4" />
                          )}
                        </ListenHeaderActionButton>
                        <ListenHeaderActionButton
                          label={
                            artistBrowsePage.isSubscribed
                              ? props.text.listen.artistUnsubscribe
                              : props.text.listen.artistSubscribe
                          }
                          active={artistBrowsePage.isSubscribed}
                          disabled={
                            artistBrowsePage.loading ||
                            !artistBrowsePage.channelId ||
                            artistActionBusy !== ""
                          }
                          onClick={toggleArtistSubscription}
                        >
                          {artistActionBusy === "subscribe" ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : artistBrowsePage.isSubscribed ? (
                            <UserCheck className="h-4 w-4" />
                          ) : (
                            <UserPlus className="h-4 w-4" />
                          )}
                        </ListenHeaderActionButton>
                      </div>
                    </div>
                  </div>
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
                    <div className="flex justify-center px-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={artistBrowsePage.appending}
                        className="w-auto px-4"
                        onClick={loadMoreArtist}
                      >
                        {artistBrowsePage.appending ? (
                          <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        {props.text.listen.loadMore}
                      </Button>
                    </div>
                  ) : null}
                </div>
              ) : showPlaylistDetail ? (
                <div className="space-y-4">
                  <div className="px-1 py-1">
                    <div className="flex items-center justify-between gap-2">
                      <Button
                        type="button"
                        variant="ghost"
                        size="compactIcon"
                        className={cn(
                          "h-8 w-8 shrink-0 rounded-xl",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={props.text.actions.back}
                        title={props.text.actions.back}
                        onClick={() => setBrowsePlaylistId("")}
                      >
                        <ArrowLeft className="h-4 w-4" />
                      </Button>
                      {selectedPlaylist ? (
                        <ListenAvatar
                          httpBaseURL={props.httpBaseURL}
                          item={{
                            channel: selectedPlaylist.channel,
                            thumbnailUrl: selectedPlaylist.thumbnailUrl,
                          }}
                        />
                      ) : null}
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-semibold text-sidebar-foreground">
                          {selectedPlaylist?.title ||
                            props.text.listen.groupPlaylist}
                        </div>
                        {selectedPlaylistHeaderMetadata ? (
                          <div className="truncate text-xs font-medium text-sidebar-foreground/55">
                            {selectedPlaylistHeaderMetadata}
                          </div>
                        ) : null}
                      </div>
                      <div className="app-dream-button-group app-completed-toolbar-actions inline-flex h-9 shrink-0 items-center p-0.5">
                        <ListenHeaderActionButton
                          label={props.text.listen.playAll}
                          disabled={playlistActionDisabled}
                          onClick={() => playPlaylistFromIndex(0)}
                        >
                          <Play className="h-4 w-4" />
                        </ListenHeaderActionButton>
                        <ListenHeaderActionButton
                          label={props.text.listen.playNext}
                          disabled={playlistActionDisabled}
                          onClick={playPlaylistNext}
                        >
                          <ListStart className="h-4 w-4" />
                        </ListenHeaderActionButton>
                        <ListenHeaderActionButton
                          label={props.text.listen.addToQueue}
                          disabled={playlistActionDisabled}
                          onClick={addPlaylistToQueue}
                        >
                          <ListEnd className="h-4 w-4" />
                        </ListenHeaderActionButton>
                        <ListenHeaderActionButton
                          label={
                            selectedPlaylistSaved
                              ? props.text.listen.removePlaylist
                              : props.text.listen.addToLibrary
                          }
                          active={selectedPlaylistSaved}
                          disabled={!selectedPlaylist || selectedPlaylistLibraryBusy}
                          onClick={() => {
                            if (!selectedPlaylist) {
                              return;
                            }
                            updatePlaylistLibrary(
                              selectedPlaylist,
                              selectedPlaylistSaved ? "remove" : "add",
                            );
                          }}
                        >
                          {selectedPlaylistLibraryBusy ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : selectedPlaylistSaved ? (
                            <UserCheck className="h-4 w-4" />
                          ) : (
                            <BookmarkPlus className="h-4 w-4" />
                          )}
                        </ListenHeaderActionButton>
                      </div>
                    </div>
                  </div>
                  {playlistLoading ? null : filteredPlaylistTracks.length > 0 ? (
                    <ListenMuseTrackList
                      items={filteredPlaylistTracks}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      artistFallback={playlistTrackArtistFallback}
                      layout={selectedPlaylistIsAlbum ? "album" : "default"}
                      onSelect={(item) => {
                        const index = playlistTracks.findIndex(
                          (track) => track.id === item.id,
                        );
                        if (index >= 0) {
                          playPlaylistFromIndex(index);
                        }
                      }}
                    />
                  ) : null}
                  {!playlistLoading && playlistContinuation ? (
                    <div className="flex justify-center px-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={playlistAppending}
                        className="w-auto px-4"
                        onClick={loadMorePlaylist}
                      >
                        {playlistAppending ? (
                          <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        {props.text.listen.loadMore}
                      </Button>
                    </div>
                  ) : null}
                </div>
              ) : !normalizedQuery && !libraryLoading && libraryError ? (
                null
              ) : (
                <div className="space-y-5">
                  {!normalizedQuery && onlineBrowseDetail ? (
                    <div className="flex items-center justify-between gap-2 px-1">
                      <Button
                        type="button"
                        variant="ghost"
                        size="compactIcon"
                        className={cn(
                          "h-8 w-8 rounded-xl",
                          LISTEN_CONTROL_ICON_BUTTON_CLASS,
                        )}
                        aria-label={props.text.actions.back}
                        title={props.text.actions.back}
                        onClick={closeOnlineBrowseDetail}
                      >
                        <ArrowLeft className="h-4 w-4" />
                      </Button>
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium text-sidebar-foreground">
                          {onlineBrowseDetail.title}
                        </div>
                        <div className="truncate text-xs text-sidebar-foreground/58">
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
                      <div className="flex items-center justify-between gap-2 px-1">
                        <Button
                          type="button"
                          variant="ghost"
                          size="compactIcon"
                          className={cn(
                            "h-8 w-8 rounded-xl",
                            LISTEN_CONTROL_ICON_BUTTON_CLASS,
                          )}
                          aria-label={props.text.actions.back}
                          title={props.text.actions.back}
                          onClick={() => setSearchSongsListOpen(false)}
                        >
                          <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-semibold text-sidebar-foreground">
                            {props.text.listen.searchSongs}
                          </div>
                          <div className="truncate text-xs text-sidebar-foreground/58">
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
                        <div className="flex justify-center px-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className={cn("rounded-full", LISTEN_CONTROL_ICON_BUTTON_CLASS)}
                            disabled={searchAppending || searchLoading}
                            onClick={loadMoreSearch}
                          >
                            {searchAppending ? (
                              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                            ) : null}
                            {props.text.listen.loadMore}
                          </Button>
                        </div>
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
                        <div className="flex justify-center px-2">
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            className={cn("rounded-full", LISTEN_CONTROL_ICON_BUTTON_CLASS)}
                            disabled={searchAppending || searchLoading}
                            onClick={loadMoreSearch}
                          >
                            {searchAppending ? (
                              <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                            ) : null}
                            {props.text.listen.loadMore}
                          </Button>
                        </div>
                      ) : null}
                    </>
                  )}
                  {!normalizedQuery &&
                  onlineBrowseSource === "home" &&
                  !onlineBrowseDetail ? (
                    <ListenMuseArtistGroup
                      title={props.text.listen.libraryArtists}
                      items={libraryArtists}
                      selectedArtistId={artistBrowsePage?.id}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={openSearchArtistBrowse}
                    />
                  ) : null}
                  {showLibraryPlaylistGroup && !searchSongsListOpen ? (
                    <ListenMusePlaylistGroup
                      title={props.text.listen.groupLibrary}
                      items={displayedLibraryPlaylists}
                      selectedPlaylistId={browsePlaylistId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={openPlaylistBrowse}
                    />
                  ) : null}
                  {!normalizedQuery
                    ? homeShelves.map((shelf) =>
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
                  {!normalizedQuery && libraryContinuation ? (
                    <div className="flex justify-center px-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={libraryAppending || libraryLoading}
                        className="w-auto px-4"
                        onClick={loadMoreLibrary}
                      >
                        {libraryAppending ? (
                          <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        {props.text.listen.loadMore}
                      </Button>
                    </div>
                  ) : null}
                </div>
              )
            ) : localTracksLoading ? (
              null
            ) : filteredLocalTracks.length > 0 ? (
              <ListenLocalTrackGroupList
                tracks={filteredLocalTracks}
                selectedId={activeLocalSelectedId}
                text={props.text}
                onSelect={selectLocalQueueTrack}
              />
            ) : localTracks.length === 0 ? (
              null
            ) : null}
          </div>
          </div>
        ) : null}
      </aside>

      <section
        className={cn(
          "listen-content-surface relative flex min-h-0 min-w-0 overflow-hidden transition-[width,flex-basis] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)]",
          hushFullscreen && "listen-content-surface-hush-fullscreen",
          listOpen ? "w-[22rem] shrink-0 grow-0" : "flex-1",
        )}
      >
        {isWindows ? (
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
            <div className="listen-list-control-surface listen-list-control-surface-top pointer-events-auto inline-flex h-9 w-9 items-center justify-center rounded-2xl p-0.5">
              <Button
                type="button"
                variant="ghost"
                size="compactIcon"
                className={cn(
                  "h-8 w-8 rounded-xl transition-transform duration-200 ease-out hover:scale-105 active:scale-95",
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
        {isWindows ? (
          <div className="absolute right-0 top-0 z-40">
            <WindowControls platform="windows" />
          </div>
        ) : null}

        <div
          className={cn(
            "min-h-0 flex-1 overflow-hidden px-0 pb-0 sm:px-0 sm:pb-0",
            hushFullscreen ? "pt-0 sm:pt-0" : "pt-14 sm:pt-16",
          )}
        >
          <ListenPlayback
                mode={playbackMode}
                active={props.active}
                listOpen={listOpen}
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
                localQueueItems={localTracks}
                selectedLocalId={selectedLocalId}
                onlinePlaying={onlinePlaying}
                localPlaying={localPlaying}
                localResumeTime={selectedLocalResumeTime}
                onlineResumeTime={activeOnlineResumeTime}
                onlineProgress={onlineProgress}
                onlineState={onlineState}
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
                onToggleFavorite={toggleOnlineFavorite}
                onOpenOnlineArtist={openOnlineArtistBrowse}
                onDownloadTrack={props.onDownloadTrack}
                onOpenLocalDirectory={openSelectedLocalDirectory}
              />
        </div>
      </section>
    </div>
  );
}
