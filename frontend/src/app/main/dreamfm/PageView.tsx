import { ArrowLeft,Coffee,Compass,Download,Gamepad2,History,Home,Loader2,Moon,PanelLeftClose,PanelLeftOpen,Play,Radio,Search,Shuffle,Sparkles,Tags,Target,Trophy,UserCheck,UserPlus,X } from "lucide-react";
import * as React from "react";

import { WindowControls } from "@/components/layout/WindowControls";
import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import { SidebarMenu,SidebarMenuButton,SidebarMenuItem } from "@/shared/ui/sidebar";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
import {
DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
DREAM_FM_CONTROL_SURFACE_CLASS,
DREAM_FM_LIST_ITEM_BUTTON_CLASS,
DREAM_FM_NOTICE_CARD_CLASS,
DREAM_FM_PILL_BUTTON_CLASS,
} from "@/shared/styles/dreamfm";

import { DREAM_FM_LIKED_SONGS_SHELF_ID } from "@/app/main/dreamfm/catalog";
import { resolveDreamFMLibraryErrorPrompt } from "@/app/main/dreamfm/error-prompts";
import { DreamFMPlayback } from "@/app/main/dreamfm/Playback";
import { buildDreamFMImageCandidates,buildDreamFMTrackThumbnailCandidates } from "@/app/main/dreamfm/storage";
import type { DreamFMArtistBrowseState,DreamFMArtistItem,DreamFMCategoryItem,DreamFMLibraryShelf,DreamFMLiveGroup,DreamFMLiveStatus,DreamFMLocalItem,DreamFMMode,DreamFMNativePlayerEvent,DreamFMOnlineBrowseDetail,DreamFMOnlineBrowseSource,DreamFMOnlineItem,DreamFMPageProps,DreamFMPlayMode,DreamFMPlaybackProgressState,DreamFMPlayerCommand,DreamFMPlaylistItem,DreamFMPlaylistLibraryAction,DreamFMRemotePlaybackState,DreamFMSidebarView } from "@/app/main/dreamfm/types";
import { DreamFMArtistGroup,DreamFMCategoryGroup,DreamFMConnectionPromptCard,DreamFMLocalArtwork,DreamFMLocalListControls,DreamFMModeTabs,DreamFMOnlineGroup,DreamFMPlaylistGroup } from "@/app/main/dreamfm/ui";

type SetState<T> = React.Dispatch<React.SetStateAction<T>>;

const DREAM_FM_HOME_IMAGE_PREFETCH_LIMIT = 48;

type DreamFMPageViewState = {
  isWindows: boolean;
  isMac: boolean;
  listOpen: boolean;
  query: string;
  searchPlaceholder: string;
  mode: DreamFMMode;
  sidebarView: DreamFMSidebarView;
  effectiveSidebarView: DreamFMSidebarView;
  onlineBrowseSource: DreamFMOnlineBrowseSource;
  onlineBrowseDetail: DreamFMOnlineBrowseDetail | null;
  liveGroups: DreamFMLiveGroup[];
  selectedLiveGroupId: string;
  liveStatusByVideoId: Record<string, DreamFMLiveStatus>;
  liveCatalogLoading: boolean;
  liveCatalogError: boolean;
  liveCatalogMessage: string;
  curatedLiveItems: DreamFMOnlineItem[];
  liveSelectionArmed: boolean;
  selectedLiveId: string;
  filteredOnlineQueueItems: DreamFMOnlineItem[];
  onlineQueueTitle: string;
  selectedOnlineId: string;
  filteredLocalTracks: DreamFMLocalItem[];
  selectedLocalId: string;
  localPlaying: boolean;
  liveSearchNotice: string;
  showArtistDetail: boolean;
  artistBrowsePage: DreamFMArtistBrowseState | null;
  artistActionBusy: "" | "mix" | "subscribe";
  filteredArtistShelves: DreamFMLibraryShelf[];
  browsePlaylistId: string;
  savedPlaylistIds: Set<string>;
  playlistMutationAction: DreamFMPlaylistLibraryAction | null;
  playlistMutationPlaylistId: string;
  filteredArtistTracks: DreamFMOnlineItem[];
  showPlaylistDetail: boolean;
  selectedPlaylist: DreamFMPlaylistItem | null | undefined;
  playlistLoading: boolean;
  playlistAppending: boolean;
  playlistTracks: DreamFMOnlineItem[];
  filteredPlaylistTracks: DreamFMOnlineItem[];
  playlistContinuation: string;
  normalizedQuery: string;
  libraryLoading: boolean;
  libraryAppending: boolean;
  libraryError: boolean;
  libraryErrorCode: string;
  searchItems: DreamFMOnlineItem[];
  searchArtists: DreamFMArtistItem[];
  searchPlaylists: DreamFMPlaylistItem[];
  libraryArtists: DreamFMArtistItem[];
  displayedLibraryPlaylists: DreamFMPlaylistItem[];
  showLibraryPlaylistGroup: boolean;
  homeShelves: DreamFMLibraryShelf[];
  libraryContinuation: string;
  onlineSearchNotice: string;
  localTracks: DreamFMLocalItem[];
  localTracksLoading: boolean;
  localTracksRefreshing: boolean;
  localTracksClearingMissing: boolean;
  activeOnline: DreamFMOnlineItem | null;
  selectedLocal: DreamFMLocalItem | null;
  onlinePlayerCommand: DreamFMPlayerCommand | null;
  localPlayerCommand: DreamFMPlayerCommand | null;
  onlineQueueItems: DreamFMOnlineItem[];
  onlinePlaying: boolean;
  onlinePlaybackArmed: boolean;
  selectedLocalResumeTime: number;
  activeOnlineResumeTime: number;
  onlineProgress: DreamFMPlaybackProgressState & { videoId: string };
  onlineState: DreamFMRemotePlaybackState;
  activeOnlineFavorite: boolean;
  activeOnlineFavoriteBusy: boolean;
  localProgress: DreamFMPlaybackProgressState;
  muted: boolean;
  volume: number;
  playMode: DreamFMPlayMode;
};

type DreamFMPageViewActions = {
  setListOpen: SetState<boolean>;
  setQuery: SetState<string>;
  selectFirstResult: () => void;
  setMode: SetState<DreamFMMode>;
  setSidebarView: SetState<DreamFMSidebarView>;
  setSelectedLiveGroupId: SetState<string>;
  reloadLiveCatalog: () => void;
  reloadLibrary: () => void;
  changeOnlineBrowseSource: (source: DreamFMOnlineBrowseSource) => void;
  openOnlineBrowseCategory: (item: DreamFMCategoryItem) => void;
  closeOnlineBrowseDetail: () => void;
  loadMoreLibrary: () => void;
  activateLiveSelection: (item: DreamFMOnlineItem) => void;
  selectOnlineQueueTrack: (item: DreamFMOnlineItem) => void;
  selectLocalQueueTrack: (item: DreamFMLocalItem) => void;
  setSelectedLocalId: SetState<string>;
  setLocalPlayerCommand: SetState<DreamFMPlayerCommand | null>;
  closeArtistBrowse: () => void;
  playArtistFromIndex: (index: number) => void;
  shuffleArtist: () => void;
  loadMoreArtist: () => void;
  playArtistMix: () => void;
  toggleArtistSubscription: () => void;
  openPlaylistBrowse: (item: DreamFMPlaylistItem) => void;
  updatePlaylistLibrary: (item: DreamFMPlaylistItem, action: DreamFMPlaylistLibraryAction) => void;
  setBrowsePlaylistId: SetState<string>;
  playPlaylistFromIndex: (index: number) => void;
  loadMorePlaylist: () => void;
  playOnlineShelfTrack: (shelf: DreamFMLibraryShelf, item: DreamFMOnlineItem) => void;
  playOnlineShelfAll: (shelf: DreamFMLibraryShelf) => void;
  shuffleOnlineShelf: (shelf: DreamFMLibraryShelf) => void;
  playOnlineSearchTrack: (item: DreamFMOnlineItem) => void;
  playOnlineSearchResults: () => void;
  shuffleOnlineSearchResults: () => void;
  openSearchArtistBrowse: (item: DreamFMArtistItem) => void;
  clearOnlineQueue: () => void;
  removeOnlineQueueItem: (item: DreamFMOnlineItem) => void;
  refreshLocalTracks: () => void;
  clearMissingLocalTracks: () => void;
  handlePlaybackEnded: () => void;
  setOnlinePlaying: (playing: boolean) => void;
  setOnlineState: (state: DreamFMRemotePlaybackState) => void;
  handleOnlineProgressChange: (videoId: string, currentTime: number, duration: number, bufferedTime: number, transient?: boolean) => void;
  handleOnlineNativeTrackChange: (event: DreamFMNativePlayerEvent) => void;
  setLocalPlaying: (playing: boolean) => void;
  handleLocalProgressChange: (currentTime: number, duration: number, bufferedTime: number) => void;
  setPlaybackSessionStarted: SetState<boolean>;
  playPrevious: () => void;
  playNext: () => void;
  togglePlayMode: () => void;
  setPlayMode: SetState<DreamFMPlayMode>;
  togglePlayback: () => void;
  toggleMute: () => void;
  handleVolumeChange: (value: number) => void;
  toggleOnlineFavorite: () => void;
  openOnlineArtistBrowse: (track: DreamFMOnlineItem) => void;
  openSelectedLocalDirectory: () => void;
};

function resolveDreamFMShelfTitle(
  shelf: DreamFMLibraryShelf,
  text: DreamFMPageProps["text"],
  fallback: string,
) {
  if (shelf.id === DREAM_FM_LIKED_SONGS_SHELF_ID) {
    return text.dreamFm.likedMusic;
  }
  return resolveDreamFMLocalizedShelfTitle(shelf.title, text) || shelf.title || fallback;
}

function resolveDreamFMLocalizedShelfTitle(
  title: string,
  text: DreamFMPageProps["text"],
) {
  switch (normalizeDreamFMShelfTitle(title)) {
    case "quick picks":
      return text.dreamFm.shelfQuickPicks;
    case "listen again":
      return text.dreamFm.shelfListenAgain;
    case "mixed for you":
    case "mixes for you":
      return text.dreamFm.shelfMixedForYou;
    case "recommended":
    case "recommendations":
      return text.dreamFm.groupRecommendations;
    case "featured playlists":
      return text.dreamFm.shelfFeaturedPlaylists;
    case "video charts":
      return text.dreamFm.shelfVideoCharts;
    case "top songs":
      return text.dreamFm.shelfTopSongs;
    case "top music videos":
      return text.dreamFm.shelfTopMusicVideos;
    case "top artists":
      return text.dreamFm.shelfTopArtists;
    case "trending":
      return text.dreamFm.shelfTrending;
    case "artists":
      return text.dreamFm.groupArtists;
    case "playlists":
      return text.dreamFm.groupPlaylist;
    case "categories":
      return text.dreamFm.groupCategories;
    case "new albums":
      return text.dreamFm.shelfNewAlbums;
    case "new singles":
      return text.dreamFm.shelfNewSingles;
    case "new music videos":
    case "new videos":
      return text.dreamFm.shelfNewVideos;
    case "new releases":
      return text.dreamFm.shelfNewReleases;
    case "moods and genres":
      return text.dreamFm.shelfMoodsAndGenres;
    default:
      return "";
  }
}

function normalizeDreamFMShelfTitle(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/&/g, " and ")
    .replace(/[^a-z0-9]+/g, " ")
    .trim();
}

function resolveDreamFMLiveGroupTitle(
  group: DreamFMLiveGroup,
  text: DreamFMPageProps["text"],
) {
  const normalized = normalizeDreamFMShelfTitle(group.title);
  if (normalized === "live") {
    return text.dreamFm.liveStations;
  }
  return resolveDreamFMLocalizedShelfTitle(group.title, text) || group.title;
}

const DREAM_FM_ONLINE_BROWSE_SOURCES: DreamFMOnlineBrowseSource[] = [
  "home",
  "explore",
  "charts",
  "moods",
  "new",
  "history",
];

function dreamFMOnlineBrowseSourceLabel(
  source: DreamFMOnlineBrowseSource,
  text: DreamFMPageProps["text"],
) {
  switch (source) {
    case "explore":
      return text.dreamFm.sourceExplore;
    case "charts":
      return text.dreamFm.sourceCharts;
    case "moods":
      return text.dreamFm.sourceMoods;
    case "new":
      return text.dreamFm.sourceNew;
    case "history":
      return text.dreamFm.sourceHistory;
    default:
      return text.dreamFm.sourceHome;
  }
}

function dreamFMOnlineBrowseSourceIcon(source: DreamFMOnlineBrowseSource) {
  switch (source) {
    case "explore":
      return <Compass className="h-4 w-4" />;
    case "charts":
      return <Trophy className="h-4 w-4" />;
    case "moods":
      return <Tags className="h-4 w-4" />;
    case "new":
      return <Sparkles className="h-4 w-4" />;
    case "history":
      return <History className="h-4 w-4" />;
    default:
      return <Home className="h-4 w-4" />;
  }
}

function DreamFMOnlineSourceTabs(props: {
  sources: DreamFMOnlineBrowseSource[];
  value: DreamFMOnlineBrowseSource;
  text: DreamFMPageProps["text"];
  onChange: (source: DreamFMOnlineBrowseSource) => void;
}) {
  const activeIndex = Math.max(0, props.sources.indexOf(props.value));
  const indicatorStyle = {
    left: "0.375rem",
    width: `calc((100% - 0.75rem - ${(props.sources.length - 1) * 0.25}rem) / ${props.sources.length})`,
    transform: `translateX(calc(${activeIndex * 100}% + ${activeIndex * 0.25}rem))`,
  };
  return (
    <TooltipProvider delayDuration={0}>
      <div className="dream-fm-list-control-surface dream-fm-list-control-surface-bottom dream-fm-footer-tabs-surface pointer-events-auto relative flex gap-1 rounded-[1.35rem] p-1.5">
        <span
          aria-hidden="true"
          className="pointer-events-none absolute bottom-1.5 top-1.5 z-0 rounded-2xl bg-[hsl(var(--dream-shell-top)/0.68)] shadow-[0_10px_28px_-20px_hsl(var(--foreground)/0.62),inset_0_0_0_1px_hsl(var(--foreground)/0.07)] transition-transform duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)] dark:bg-white/10"
          style={indicatorStyle}
        />
        {props.sources.map((source) => {
          const label = dreamFMOnlineBrowseSourceLabel(source, props.text);
          return (
            <Tooltip key={source}>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  data-active={props.value === source ? "true" : "false"}
                  className={cn(
                    "relative z-10 flex h-9 min-w-0 flex-1 items-center justify-center rounded-2xl text-sidebar-foreground/55 transition-[color,transform,opacity] duration-200 ease-out active:scale-95",
                    "after:absolute after:bottom-1 after:h-1 after:w-1 after:scale-0 after:rounded-full after:bg-sidebar-primary after:opacity-0 after:transition",
                    "hover:text-sidebar-foreground focus-visible:outline-none",
                    "data-[active=true]:text-sidebar-foreground data-[active=true]:after:scale-100 data-[active=true]:after:opacity-100",
                  )}
                  aria-label={label}
                  onClick={() => props.onChange(source)}
                >
                  {dreamFMOnlineBrowseSourceIcon(source)}
                </button>
              </TooltipTrigger>
              <TooltipContent side="top">{label}</TooltipContent>
            </Tooltip>
          );
        })}
      </div>
    </TooltipProvider>
  );
}

function DreamFMLiveGroupTabs(props: {
  groups: DreamFMLiveGroup[];
  value: string;
  text: DreamFMPageProps["text"];
  onChange: (groupId: string) => void;
}) {
  if (props.groups.length <= 1) {
    return null;
  }
  const activeIndex = Math.max(
    0,
    props.groups.findIndex((group) => group.id === props.value),
  );
  const indicatorStyle = {
    left: "0.375rem",
    width: `calc((100% - 0.75rem - ${(props.groups.length - 1) * 0.25}rem) / ${props.groups.length})`,
    transform: `translateX(calc(${activeIndex * 100}% + ${activeIndex * 0.25}rem))`,
  };
  return (
    <TooltipProvider delayDuration={0}>
      <div className="dream-fm-list-control-surface dream-fm-list-control-surface-bottom dream-fm-footer-tabs-surface pointer-events-auto relative flex gap-1 rounded-[1.35rem] p-1.5">
        <span
          aria-hidden="true"
          className="pointer-events-none absolute bottom-1.5 top-1.5 z-0 rounded-2xl bg-[hsl(var(--dream-shell-top)/0.68)] shadow-[0_10px_28px_-20px_hsl(var(--foreground)/0.62),inset_0_0_0_1px_hsl(var(--foreground)/0.07)] transition-transform duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)] dark:bg-white/10"
          style={indicatorStyle}
        />
        {props.groups.map((group) => {
          const title = resolveDreamFMLiveGroupTitle(group, props.text);
          return (
            <Tooltip key={group.id}>
              <TooltipTrigger asChild>
                <button
                  type="button"
                  data-active={props.value === group.id ? "true" : "false"}
                  className={cn(
                    "relative z-10 flex h-9 min-w-0 flex-1 items-center justify-center rounded-2xl text-sidebar-foreground/55 transition-[color,transform,opacity] duration-200 ease-out active:scale-95",
                    "after:absolute after:bottom-1 after:h-1 after:w-1 after:scale-0 after:rounded-full after:bg-sidebar-primary after:opacity-0 after:transition",
                    "hover:text-sidebar-foreground focus-visible:outline-none",
                    "data-[active=true]:text-sidebar-foreground data-[active=true]:after:scale-100 data-[active=true]:after:opacity-100",
                  )}
                  aria-label={title}
                  onClick={() => props.onChange(group.id)}
                >
                  {dreamFMLiveGroupIcon(group)}
                </button>
              </TooltipTrigger>
              <TooltipContent side="top">{title}</TooltipContent>
            </Tooltip>
          );
        })}
      </div>
    </TooltipProvider>
  );
}

function dreamFMLiveGroupIcon(group: DreamFMLiveGroup) {
  const key = `${group.id} ${group.title}`.toLowerCase();
  if (key.includes("focus") || key.includes("study") || key.includes("work")) {
    return <Target className="h-4 w-4" />;
  }
  if (key.includes("sleep") || key.includes("chill")) {
    return <Moon className="h-4 w-4" />;
  }
  if (key.includes("coffee") || key.includes("jazz")) {
    return <Coffee className="h-4 w-4" />;
  }
  if (key.includes("night") || key.includes("game")) {
    return <Gamepad2 className="h-4 w-4" />;
  }
  if (key.includes("new") || key.includes("mix")) {
    return <Sparkles className="h-4 w-4" />;
  }
  return <Radio className="h-4 w-4" />;
}

export type DreamFMPageViewProps = {
  page: DreamFMPageProps;
  state: DreamFMPageViewState;
  actions: DreamFMPageViewActions;
};

function collectDreamFMHomeImagePrefetchURLs(
  httpBaseURL: string,
  options: {
    libraryArtists: DreamFMArtistItem[];
    displayedLibraryPlaylists: DreamFMPlaylistItem[];
    homeShelves: DreamFMLibraryShelf[];
  },
) {
  const seen = new Set<string>();
  const urls: string[] = [];
  const addFirstCandidate = (candidates: string[]) => {
    if (urls.length >= DREAM_FM_HOME_IMAGE_PREFETCH_LIMIT) {
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
      buildDreamFMImageCandidates(httpBaseURL, item.thumbnailUrl ?? ""),
    );
  };
  const addTrackItem = (item: { videoId: string; thumbnailUrl?: string }) => {
    addFirstCandidate(buildDreamFMTrackThumbnailCandidates(httpBaseURL, item));
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

export function DreamFMPageView(view: DreamFMPageViewProps) {
  const props = view.page;
  const { isWindows, isMac, listOpen, query, searchPlaceholder, mode, effectiveSidebarView, onlineBrowseSource, onlineBrowseDetail, liveGroups, selectedLiveGroupId, liveStatusByVideoId, liveCatalogLoading, liveCatalogError, liveCatalogMessage, curatedLiveItems, liveSelectionArmed, selectedLiveId, filteredOnlineQueueItems, onlineQueueTitle, selectedOnlineId, filteredLocalTracks, selectedLocalId, localPlaying, liveSearchNotice, showArtistDetail, artistBrowsePage, artistActionBusy, filteredArtistShelves, browsePlaylistId, savedPlaylistIds, playlistMutationAction, playlistMutationPlaylistId, filteredArtistTracks, showPlaylistDetail, selectedPlaylist, playlistLoading, playlistAppending, playlistTracks, filteredPlaylistTracks, playlistContinuation, normalizedQuery, libraryLoading, libraryAppending, libraryError, libraryErrorCode, searchItems, searchArtists, searchPlaylists, libraryArtists, displayedLibraryPlaylists, showLibraryPlaylistGroup, homeShelves, libraryContinuation, onlineSearchNotice, localTracks, localTracksLoading, localTracksRefreshing, localTracksClearingMissing, activeOnline, selectedLocal, onlinePlayerCommand, localPlayerCommand, onlineQueueItems, onlinePlaying, onlinePlaybackArmed, selectedLocalResumeTime, activeOnlineResumeTime, onlineProgress, onlineState, activeOnlineFavorite, activeOnlineFavoriteBusy, localProgress, muted, volume, playMode } = view.state;
  const { setListOpen, setQuery, selectFirstResult, setMode, setSidebarView, setSelectedLiveGroupId, reloadLiveCatalog, reloadLibrary, changeOnlineBrowseSource, openOnlineBrowseCategory, closeOnlineBrowseDetail, loadMoreLibrary, activateLiveSelection, selectOnlineQueueTrack, selectLocalQueueTrack, closeArtistBrowse, playArtistFromIndex, shuffleArtist, loadMoreArtist, playArtistMix, toggleArtistSubscription, openPlaylistBrowse, updatePlaylistLibrary, setBrowsePlaylistId, playPlaylistFromIndex, loadMorePlaylist, playOnlineShelfTrack, playOnlineShelfAll, shuffleOnlineShelf, playOnlineSearchTrack, playOnlineSearchResults, shuffleOnlineSearchResults, openSearchArtistBrowse, clearOnlineQueue, removeOnlineQueueItem, refreshLocalTracks, clearMissingLocalTracks, handlePlaybackEnded, setOnlinePlaying, setOnlineState, handleOnlineProgressChange, handleOnlineNativeTrackChange, setLocalPlaying, handleLocalProgressChange, setPlaybackSessionStarted, playPrevious, playNext, togglePlayMode, setPlayMode, togglePlayback, toggleMute, handleVolumeChange, toggleOnlineFavorite, openOnlineArtistBrowse, openSelectedLocalDirectory } = view.actions;
  const libraryErrorPrompt = resolveDreamFMLibraryErrorPrompt(libraryErrorCode, props.text);
  const [searchFocused, setSearchFocused] = React.useState(false);
  const searchInputRef = React.useRef<HTMLInputElement | null>(null);
  const imagePrefetchRef = React.useRef<HTMLImageElement[]>([]);
  const searchHasText = query.length > 0;
  const searchInputActive = searchFocused || searchHasText;
  const tabsCompact = searchInputActive;
  const activeLiveGroup =
    liveGroups.find((group) => group.id === selectedLiveGroupId) ?? null;
  const activeLiveGroupTitle = activeLiveGroup
    ? resolveDreamFMLiveGroupTitle(activeLiveGroup, props.text)
    : props.text.dreamFm.liveStations;
  const liveCatalogPrompt =
    liveCatalogError
      ? liveCatalogMessage || props.text.dreamFm.liveUnavailable
      : props.text.dreamFm.liveEmpty;

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

  const handleOnlineSourceTabChange = React.useCallback(
    (source: DreamFMOnlineBrowseSource) => {
      setSidebarView("browse");
      clearSearch();
      changeOnlineBrowseSource(source);
    },
    [changeOnlineBrowseSource, clearSearch, setSidebarView],
  );

  React.useEffect(() => {
    const shouldPrefetch =
      props.active &&
      listOpen &&
      mode === "online" &&
      effectiveSidebarView === "browse" &&
      onlineBrowseSource === "home" &&
      !onlineBrowseDetail &&
      !normalizedQuery;
    if (!shouldPrefetch) {
      imagePrefetchRef.current = [];
      return;
    }
    const urls = collectDreamFMHomeImagePrefetchURLs(props.httpBaseURL, {
      libraryArtists,
      displayedLibraryPlaylists,
      homeShelves,
    });
    imagePrefetchRef.current = urls.map((url) => {
      const image = new window.Image();
      image.decoding = "async";
      image.src = url;
      return image;
    });
    return () => {
      imagePrefetchRef.current = [];
    };
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

  return (
    <div
      className={cn(
        "min-h-0 min-w-0 flex-1 overflow-hidden bg-sidebar-background",
        props.active ? "flex" : "hidden",
        props.className,
      )}
    >
      <aside
        aria-hidden={!listOpen}
        className={cn(
          "dream-fm-list-surface relative flex min-h-0 shrink-0 overflow-hidden backdrop-blur-2xl transition-[width,opacity,transform,border-color,box-shadow] duration-300 [transition-timing-function:cubic-bezier(0.2,_0.8,_0.2,_1)]",
          listOpen
            ? "w-[340px] border-r border-[hsl(var(--foreground)/0.08)] opacity-100 shadow-[inset_-1px_0_0_hsl(var(--background)/0.14)]"
            : "pointer-events-none w-0 -translate-x-2 border-r-0 border-transparent opacity-0 shadow-none",
        )}
      >
        {listOpen ? (
          <div className="relative flex h-full w-[340px] shrink-0 flex-col overflow-hidden animate-in fade-in-0 slide-in-from-left-2 duration-300">
          <div className="pointer-events-none absolute inset-x-0 top-0 z-30 px-4 pb-10 pt-3">
            <div className="pointer-events-auto relative flex items-center justify-between gap-2">
              <div
                className={cn(
                  "dream-fm-list-control-surface dream-fm-list-control-surface-top flex h-9 items-center gap-2 rounded-2xl text-sidebar-foreground transition-[width,box-shadow,border-color,background-color] duration-200 ease-out",
                  searchInputActive
                    ? "min-w-0 flex-1 px-3"
                    : "w-10 min-w-10 shrink-0 grow-0 cursor-text justify-center px-0",
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
                <Search className="h-4 w-4 shrink-0 text-sidebar-foreground/55" />
                {searchInputActive ? (
                  <>
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
                      className="app-control-input-compact h-auto w-full rounded-none border-0 bg-transparent px-0 shadow-none"
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
                        className="flex h-5 w-5 items-center justify-center rounded-full text-sidebar-foreground/55 transition hover:bg-sidebar-background/54 hover:text-sidebar-foreground focus-visible:outline-none disabled:pointer-events-none"
                        onClick={clearSearch}
                      >
                        <X className="h-3.5 w-3.5" />
                      </button>
                    </span>
                  </>
                ) : null}
              </div>
              <TooltipProvider delayDuration={0}>
                <DreamFMModeTabs mode={mode} compact={tabsCompact} text={props.text} onChange={setMode} />
              </TooltipProvider>
            </div>
          </div>

          <div
            className={cn(
              "min-h-0 flex-1 overflow-y-auto px-3 pt-[4.75rem] animate-in fade-in-0 slide-in-from-bottom-1 duration-200",
              mode === "online" || mode === "live" || mode === "local" ? "pb-24" : "pb-4",
            )}
          >
            {effectiveSidebarView === "queue" ? (
              mode === "live" ? (
                <div className="space-y-5">
                  <DreamFMOnlineGroup
                    title={props.text.dreamFm.upNext}
                    hideTitle
                    items={curatedLiveItems}
                    selectedId={liveSelectionArmed ? selectedLiveId : ""}
                    httpBaseURL={props.httpBaseURL}
                    text={props.text}
                    liveStatuses={liveStatusByVideoId}
                    onSelect={activateLiveSelection}
                  />
                  {curatedLiveItems.length === 0 ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.upNextEmpty}
                    </div>
                  ) : null}
                </div>
              ) : mode === "online" ? (
                <div className="space-y-5">
                  {filteredOnlineQueueItems.length > 0 ? (
                    <DreamFMOnlineGroup
                      title={onlineQueueTitle}
                      items={filteredOnlineQueueItems}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onClear={clearOnlineQueue}
                      clearLabel={props.text.dreamFm.clearQueue}
                      onRemove={removeOnlineQueueItem}
                      removeLabel={props.text.dreamFm.removeFromQueue}
                      onSelect={selectOnlineQueueTrack}
                    />
                  ) : (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.upNextEmpty}
                    </div>
                  )}
                </div>
              ) : localTracksLoading ? (
                <div className={DREAM_FM_NOTICE_CARD_CLASS}>{props.text.dreamFm.localLoading}</div>
              ) : filteredLocalTracks.length > 0 ? (
                <SidebarMenu className="gap-1.5">
                  {filteredLocalTracks.map((track) => (
                    <SidebarMenuItem key={track.id}>
                      <SidebarMenuButton
                        type="button"
                        isActive={track.id === selectedLocalId}
                        className={cn("min-h-14", DREAM_FM_LIST_ITEM_BUTTON_CLASS)}
                        onClick={() => {
                          selectLocalQueueTrack(track);
                        }}
                      >
                        <DreamFMLocalArtwork track={track} />
                        <div className="min-w-0 flex-1">
                          <div className="truncate text-sm font-medium text-sidebar-foreground">
                            {track.title}
                          </div>
                          <div className="truncate text-xs text-sidebar-foreground/58">
                            {[track.author, track.durationLabel]
                              .filter(Boolean)
                              .join(" · ")}
                          </div>
                        </div>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              ) : (
                <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                  {props.text.dreamFm.upNextEmpty}
                </div>
              )
            ) : mode === "live" ? (
              <div className="space-y-5">
                {liveCatalogLoading && liveGroups.length === 0 ? (
                  <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                    {props.text.dreamFm.liveLoading}
                  </div>
                ) : liveCatalogError || liveGroups.length === 0 ? (
                  <DreamFMConnectionPromptCard
                    message={liveCatalogPrompt}
                    actionLabel={props.text.dreamFm.retry}
                    icon={<Radio className="h-5 w-5" />}
                    onAction={reloadLiveCatalog}
                  />
                ) : (
                  <>
                    <DreamFMOnlineGroup
                      title={
                        normalizedQuery
                          ? props.text.dreamFm.liveStations
                          : activeLiveGroupTitle
                      }
                      hideTitle
                      items={curatedLiveItems}
                      selectedId={liveSelectionArmed ? selectedLiveId : ""}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      liveStatuses={liveStatusByVideoId}
                      onSelect={activateLiveSelection}
                    />
                    {liveSearchNotice ? (
                      <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                        {liveSearchNotice}
                      </div>
                    ) : null}
                  </>
                )}
              </div>
            ) : mode === "online" ? (
              showArtistDetail && artistBrowsePage ? (
                <div className="space-y-4">
                  <div className="flex items-center justify-between gap-2 px-1">
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      className={cn(
                        "h-8 w-8 rounded-xl",
                        DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.actions.back}
                      title={props.text.actions.back}
                      onClick={closeArtistBrowse}
                    >
                      <ArrowLeft className="h-4 w-4" />
                    </Button>
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium text-sidebar-foreground">
                        {artistBrowsePage.name ||
                          artistBrowsePage.title ||
                          artistBrowsePage.id}
                      </div>
                      <div className="truncate text-xs text-sidebar-foreground/58">
                        {artistBrowsePage.subtitle || "YouTube Music"}
                      </div>
                    </div>
                    <div className={cn(DREAM_FM_CONTROL_SURFACE_CLASS, "rounded-2xl")}>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="compactIcon"
                            aria-label={props.text.dreamFm.artistShuffle}
                            title={props.text.dreamFm.artistShuffle}
                            disabled={
                              artistBrowsePage.loading ||
                              artistBrowsePage.items.length === 0 ||
                              artistActionBusy !== ""
                            }
                            className={cn(
                              "h-8 w-8 rounded-xl",
                              DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                            )}
                            onClick={shuffleArtist}
                          >
                            <Shuffle className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="bottom">
                          {props.text.dreamFm.artistShuffle}
                        </TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="compactIcon"
                            aria-label={props.text.dreamFm.artistMix}
                            title={props.text.dreamFm.artistMix}
                            disabled={
                              artistBrowsePage.loading ||
                              !artistBrowsePage.mixPlaylistId ||
                              artistActionBusy !== ""
                            }
                            className={cn(
                              "h-8 w-8 rounded-xl",
                              DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                            )}
                            onClick={playArtistMix}
                          >
                            {artistActionBusy === "mix" ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Radio className="h-4 w-4" />
                            )}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="bottom">
                          {props.text.dreamFm.artistMix}
                        </TooltipContent>
                      </Tooltip>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button
                            type="button"
                            variant="ghost"
                            size="compactIcon"
                            aria-label={
                              artistBrowsePage.isSubscribed
                                ? props.text.dreamFm.artistUnsubscribe
                                : props.text.dreamFm.artistSubscribe
                            }
                            title={
                              artistBrowsePage.isSubscribed
                                ? props.text.dreamFm.artistUnsubscribe
                                : props.text.dreamFm.artistSubscribe
                            }
                            disabled={
                              artistBrowsePage.loading ||
                              !artistBrowsePage.channelId ||
                              artistActionBusy !== ""
                            }
                            className={cn(
                              "h-8 w-8 rounded-xl",
                              DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                              artistBrowsePage.isSubscribed &&
                                "text-sidebar-primary hover:text-sidebar-primary",
                            )}
                            onClick={toggleArtistSubscription}
                          >
                            {artistActionBusy === "subscribe" ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : artistBrowsePage.isSubscribed ? (
                              <UserCheck className="h-4 w-4" />
                            ) : (
                              <UserPlus className="h-4 w-4" />
                            )}
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent side="bottom">
                          {artistBrowsePage.isSubscribed
                            ? props.text.dreamFm.artistUnsubscribe
                            : props.text.dreamFm.artistSubscribe}
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                  {artistBrowsePage.loading ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.artistLoading}
                    </div>
                  ) : artistBrowsePage.error ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.artistUnavailable}
                    </div>
                  ) : filteredArtistShelves.length > 0 ? (
                    <div className="space-y-5">
                      {filteredArtistShelves.map((shelf) =>
                        shelf.kind === "artists" ? (
                          <DreamFMArtistGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupArtists,
                            )}
                            items={shelf.artists}
                            selectedArtistId={artistBrowsePage?.id}
                            httpBaseURL={props.httpBaseURL}
                            onSelect={openSearchArtistBrowse}
                          />
                        ) : shelf.kind === "playlists" ? (
                          <DreamFMPlaylistGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupPlaylist,
                            )}
                            items={shelf.playlists}
                            selectedPlaylistId={browsePlaylistId}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            savedPlaylistIds={savedPlaylistIds}
                            playlistMutationAction={playlistMutationAction}
                            playlistMutationPlaylistId={
                              playlistMutationPlaylistId
                            }
                            onSelect={openPlaylistBrowse}
                            onToggleLibrary={updatePlaylistLibrary}
                          />
                        ) : (
                          <DreamFMOnlineGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupRecommendations,
                            )}
                            items={shelf.tracks}
                            selectedId={selectedOnlineId}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            onSelect={(item) => {
                              const index = artistBrowsePage.items.findIndex(
                                (track) => track.id === item.id,
                              );
                              if (index >= 0) {
                                playArtistFromIndex(index);
                              }
                            }}
                          />
                        ),
                      )}
                    </div>
                  ) : filteredArtistTracks.length > 0 ? (
                    <DreamFMOnlineGroup
                      title={artistBrowsePage.title || artistBrowsePage.name}
                      items={filteredArtistTracks}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={(item) => {
                        const index = artistBrowsePage.items.findIndex(
                          (track) => track.id === item.id,
                        );
                        if (index >= 0) {
                          playArtistFromIndex(index);
                        }
                      }}
                    />
                  ) : (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.artistEmpty}
                    </div>
                  )}
                  {!artistBrowsePage.loading &&
                  !artistBrowsePage.error &&
                  artistBrowsePage.continuation ? (
                    <div className="px-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={artistBrowsePage.appending}
                        className={cn("w-full", DREAM_FM_PILL_BUTTON_CLASS)}
                        onClick={loadMoreArtist}
                      >
                        {artistBrowsePage.appending ? (
                          <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        {props.text.dreamFm.loadMore}
                      </Button>
                    </div>
                  ) : null}
                </div>
              ) : showPlaylistDetail ? (
                <div className="space-y-4">
                  <div className="flex items-center justify-between gap-2 px-1">
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      className={cn(
                        "h-8 w-8 rounded-xl",
                        DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
                      )}
                      aria-label={props.text.actions.back}
                      title={props.text.actions.back}
                      onClick={() => setBrowsePlaylistId("")}
                    >
                      <ArrowLeft className="h-4 w-4" />
                    </Button>
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium text-sidebar-foreground">
                        {selectedPlaylist?.title ||
                          props.text.dreamFm.groupPlaylist}
                      </div>
                      <div className="truncate text-xs text-sidebar-foreground/58">
                        {selectedPlaylist?.channel}
                      </div>
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={playlistLoading || playlistTracks.length === 0}
                      className="shrink-0 rounded-full border-transparent bg-sidebar-primary/10 text-sidebar-primary shadow-[inset_0_1px_0_hsl(var(--background)/0.18)] hover:bg-sidebar-primary/14 hover:text-sidebar-primary"
                      onClick={() => playPlaylistFromIndex(0)}
                    >
                      <Play className="mr-1.5 h-3.5 w-3.5" />
                      {props.text.dreamFm.playAll}
                    </Button>
                  </div>
                  {playlistLoading ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.playlistLoading}
                    </div>
                  ) : filteredPlaylistTracks.length > 0 ? (
                    <DreamFMOnlineGroup
                      title={
                        selectedPlaylist?.title ||
                        props.text.dreamFm.groupPlaylist
                      }
                      items={filteredPlaylistTracks}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onSelect={(item) => {
                        const index = playlistTracks.findIndex(
                          (track) => track.id === item.id,
                        );
                        if (index >= 0) {
                          playPlaylistFromIndex(index);
                        }
                      }}
                    />
                  ) : (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.playlistEmpty}
                    </div>
                  )}
                  {!playlistLoading && playlistContinuation ? (
                    <div className="px-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={playlistAppending}
                        className={cn("w-full", DREAM_FM_PILL_BUTTON_CLASS)}
                        onClick={loadMorePlaylist}
                      >
                        {playlistAppending ? (
                          <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        {props.text.dreamFm.loadMore}
                      </Button>
                    </div>
                  ) : null}
                </div>
              ) : !normalizedQuery && !libraryLoading && libraryError ? (
                <DreamFMConnectionPromptCard
                  message={libraryErrorPrompt.message}
                  actionLabel={libraryErrorPrompt.actionLabel}
                  icon={libraryErrorPrompt.icon}
                  onAction={
                    libraryErrorPrompt.action === "connections"
                      ? props.onOpenConnections
                      : reloadLibrary
                  }
                />
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
                          DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
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
                          {dreamFMOnlineBrowseSourceLabel(
                            onlineBrowseDetail.source,
                            props.text,
                          )}
                        </div>
                      </div>
                    </div>
                  ) : null}
                  {normalizedQuery ? (
                    <DreamFMOnlineGroup
                      title={props.text.dreamFm.searchSongs}
                      items={searchItems}
                      selectedId={selectedOnlineId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      onPlayAll={searchItems.length > 0 ? playOnlineSearchResults : undefined}
                      onShuffle={
                        searchItems.length > 1 ? shuffleOnlineSearchResults : undefined
                      }
                      onSelect={playOnlineSearchTrack}
                    />
                  ) : null}
                  {normalizedQuery ? (
                    <DreamFMArtistGroup
                      title={props.text.dreamFm.searchArtists}
                      items={searchArtists}
                      selectedArtistId={artistBrowsePage?.id}
                      httpBaseURL={props.httpBaseURL}
                      onSelect={openSearchArtistBrowse}
                    />
                  ) : null}
                  {normalizedQuery ? (
                    <DreamFMPlaylistGroup
                      title={props.text.dreamFm.searchPlaylists}
                      items={searchPlaylists}
                      selectedPlaylistId={browsePlaylistId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      savedPlaylistIds={savedPlaylistIds}
                      playlistMutationAction={playlistMutationAction}
                      playlistMutationPlaylistId={playlistMutationPlaylistId}
                      onSelect={openPlaylistBrowse}
                      onToggleLibrary={updatePlaylistLibrary}
                    />
                  ) : null}
                  {!normalizedQuery &&
                  onlineBrowseSource === "home" &&
                  !onlineBrowseDetail ? (
                    <DreamFMArtistGroup
                      title={props.text.dreamFm.libraryArtists}
                      items={libraryArtists}
                      selectedArtistId={artistBrowsePage?.id}
                      httpBaseURL={props.httpBaseURL}
                      onSelect={openSearchArtistBrowse}
                    />
                  ) : null}
                  {showLibraryPlaylistGroup ? (
                    <DreamFMPlaylistGroup
                      title={props.text.dreamFm.groupLibrary}
                      items={displayedLibraryPlaylists}
                      selectedPlaylistId={browsePlaylistId}
                      httpBaseURL={props.httpBaseURL}
                      text={props.text}
                      savedPlaylistIds={savedPlaylistIds}
                      playlistMutationAction={playlistMutationAction}
                      playlistMutationPlaylistId={playlistMutationPlaylistId}
                      onSelect={openPlaylistBrowse}
                      onToggleLibrary={updatePlaylistLibrary}
                    />
                  ) : null}
                  {!normalizedQuery
                    ? homeShelves.map((shelf) =>
                        shelf.kind === "artists" ? (
                          <DreamFMArtistGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupArtists,
                            )}
                            items={shelf.artists}
                            selectedArtistId={artistBrowsePage?.id}
                            httpBaseURL={props.httpBaseURL}
                            onSelect={openSearchArtistBrowse}
                          />
                        ) : shelf.kind === "categories" ? (
                          <DreamFMCategoryGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupCategories,
                            )}
                            items={shelf.categories}
                            selectedCategoryId={onlineBrowseDetail?.id}
                            onSelect={openOnlineBrowseCategory}
                          />
                        ) : shelf.kind === "playlists" ? (
                          <DreamFMPlaylistGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupPlaylist,
                            )}
                            items={shelf.playlists}
                            selectedPlaylistId={browsePlaylistId}
                            httpBaseURL={props.httpBaseURL}
                            text={props.text}
                            savedPlaylistIds={savedPlaylistIds}
                            playlistMutationAction={playlistMutationAction}
                            playlistMutationPlaylistId={
                              playlistMutationPlaylistId
                            }
                            onSelect={openPlaylistBrowse}
                            onToggleLibrary={updatePlaylistLibrary}
                          />
                        ) : (
                          <DreamFMOnlineGroup
                            key={shelf.id}
                            title={resolveDreamFMShelfTitle(
                              shelf,
                              props.text,
                              props.text.dreamFm.groupRecommendations,
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
                    <div className="px-2">
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        disabled={libraryAppending || libraryLoading}
                        className={cn("w-full", DREAM_FM_PILL_BUTTON_CLASS)}
                        onClick={loadMoreLibrary}
                      >
                        {libraryAppending ? (
                          <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" />
                        ) : null}
                        {props.text.dreamFm.loadMore}
                      </Button>
                    </div>
                  ) : null}
                  {!normalizedQuery && libraryLoading ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.onlineLoading}
                    </div>
                  ) : null}
                  {!normalizedQuery &&
                  !libraryLoading &&
                  !libraryError &&
                  homeShelves.length === 0 &&
                  libraryArtists.length === 0 &&
                  (!showLibraryPlaylistGroup ||
                    displayedLibraryPlaylists.length === 0) ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {props.text.dreamFm.onlineEmpty}
                    </div>
                  ) : null}
                  {onlineSearchNotice ? (
                    <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                      {onlineSearchNotice}
                    </div>
                  ) : null}
                </div>
              )
            ) : localTracksLoading ? (
              <div className={DREAM_FM_NOTICE_CARD_CLASS}>{props.text.dreamFm.localLoading}</div>
            ) : filteredLocalTracks.length > 0 ? (
              <SidebarMenu className="gap-1.5">
                {filteredLocalTracks.map((track) => (
                  <SidebarMenuItem key={track.id}>
                    <SidebarMenuButton
                      type="button"
                      isActive={track.id === selectedLocalId}
                      className={cn("min-h-14", DREAM_FM_LIST_ITEM_BUTTON_CLASS)}
                      onClick={() => {
                        selectLocalQueueTrack(track);
                      }}
                    >
                      <DreamFMLocalArtwork track={track} />
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium text-sidebar-foreground">
                          {track.title}
                        </div>
                        <div className="truncate text-xs text-sidebar-foreground/58">
                          {[track.author, track.durationLabel]
                            .filter(Boolean)
                            .join(" · ")}
                        </div>
                      </div>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            ) : localTracks.length === 0 ? (
              <DreamFMConnectionPromptCard
                message={props.text.dreamFm.localEmptyPrompt}
                actionLabel={props.text.dreamFm.localEmptyAction}
                icon={<Download className="h-5 w-5" />}
                onAction={() => props.onDownloadTrack("")}
              />
            ) : (
              <div className={DREAM_FM_NOTICE_CARD_CLASS}>
                {props.text.dreamFm.searchEmpty}
              </div>
            )}
          </div>
          {mode === "online" ? (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 px-4 pb-3 pt-10">
              <DreamFMOnlineSourceTabs
                sources={DREAM_FM_ONLINE_BROWSE_SOURCES}
                value={onlineBrowseSource}
                text={props.text}
                onChange={handleOnlineSourceTabChange}
              />
            </div>
          ) : mode === "live" && liveGroups.length > 1 ? (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 px-4 pb-3 pt-10">
              <DreamFMLiveGroupTabs
                groups={liveGroups}
                value={selectedLiveGroupId}
                text={props.text}
                onChange={setSelectedLiveGroupId}
              />
            </div>
          ) : mode === "local" ? (
            <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20 flex justify-center px-4 pb-3 pt-10">
              <DreamFMLocalListControls
                text={props.text}
                refreshing={localTracksRefreshing}
                clearingMissing={localTracksClearingMissing}
                onRefresh={refreshLocalTracks}
                onClearMissing={clearMissingLocalTracks}
              />
            </div>
          ) : null}
          </div>
        ) : null}
      </aside>

      <section className="dream-fm-content-surface relative flex min-h-0 min-w-0 flex-1 overflow-hidden">
        {isWindows ? (
          <div
            className="wails-drag absolute left-14 right-[var(--app-windows-caption-control-width)] top-0 z-20 h-[var(--app-titlebar-height-windows)]"
            aria-hidden="true"
          />
        ) : null}
        <div
          className={cn(
            "pointer-events-none absolute left-3 top-3 z-30",
            isWindows ? "right-36" : "right-3",
          )}
        >
          <div className="dream-fm-list-control-surface dream-fm-list-control-surface-top pointer-events-auto inline-flex h-9 w-9 items-center justify-center rounded-2xl p-0.5">
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              className={cn(
                "h-8 w-8 rounded-xl transition-transform duration-200 ease-out hover:scale-105 active:scale-95",
                DREAM_FM_CONTROL_ICON_BUTTON_CLASS,
              )}
              aria-label={
                listOpen
                  ? props.text.dreamFm.collapseList
                  : props.text.dreamFm.openList
              }
              title={
                listOpen
                  ? props.text.dreamFm.collapseList
                  : props.text.dreamFm.openList
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
        {isWindows ? (
          <div className="absolute right-0 top-0 z-40">
            <WindowControls platform="windows" />
          </div>
        ) : null}

        <div className="min-h-0 flex-1 overflow-hidden px-0 pb-0 pt-14 sm:px-0 sm:pb-0 sm:pt-16">
          <DreamFMPlayback
                mode={mode}
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
                  mode === "live" ? props.text.dreamFm.liveStations : onlineQueueTitle
                }
                selectedOnlineId={mode === "live" ? selectedLiveId : selectedOnlineId}
                localQueueItems={localTracks}
                selectedLocalId={selectedLocalId}
                onlinePlaying={onlinePlaying}
                localPlaying={localPlaying}
                localResumeTime={selectedLocalResumeTime}
                onlineResumeTime={activeOnlineResumeTime}
                onlineProgress={onlineProgress}
                onlineState={onlineState}
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
                  mode === "live" ? activateLiveSelection : selectOnlineQueueTrack
                }
                onClearOnlineQueue={clearOnlineQueue}
                onRemoveOnlineQueueItem={removeOnlineQueueItem}
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
