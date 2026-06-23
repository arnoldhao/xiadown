import { Call,Events,System } from "@wailsio/runtime";
import * as React from "react";
import "./listen/listen.css";

import { messageBus } from "@/shared/message";
import {
  useAppSessionConnectSession,
  useAppSessions,
  useCancelAppSessionConnect,
  useClearAppSession,
  useStartAppSessionConnect,
} from "@/shared/query/appSessions";
import { useOpenLibraryPath } from "@/shared/query/library";
import { REALTIME_TOPICS,registerTopic } from "@/shared/realtime";

import { fetchListenArtist,fetchListenLibrary,fetchListenLiveCatalog,fetchListenLiveStatuses,fetchListenLiveUserCatalog,fetchListenPlaylistPage,fetchListenPlaylistQueue,fetchListenSearch,fetchListenTrackFavorite,fetchListenTrackFavoriteStatuses,fetchListenTrackInfo,getListenErrorCode,getListenErrorMessage,saveListenLiveUserCatalog,updateListenArtistSubscription,updateListenPlaylistLibrary,updateListenTrackFavorite } from "@/app/main/listen/api";
import type { ListenLiveUserCatalog } from "@/app/main/listen/api";
import { LISTEN_LIKED_SONGS_SHELF_ID,LISTEN_LIVE_PLAYER_SERVICE,LISTEN_NATIVE_PLAYER_SERVICE,LISTEN_YOUTUBE_APP_SESSION_CHANGED_EVENT } from "@/app/main/listen/catalog";
import { clampVolume,matchesQuery,normalizeSearch,resolveListenLiveSelectionId,resolveQueueIndex,useListenLocalTracks } from "@/app/main/listen/local-library";
import { ListenLocalRelinkRepair } from "@/app/main/listen/LocalRelinkRepair";
import { ListenPageView } from "@/app/main/listen/PageView";
import { callListenPlaybackAppendToQueue,callListenPlaybackClearQueue,callListenPlaybackInsertNextInQueue,callListenPlaybackMergeTrackMetadata,callListenPlaybackMoveQueueItem,callListenPlaybackNext,callListenPlaybackObserveNativeEvent,callListenPlaybackPause,callListenPlaybackPlayPause,callListenPlaybackPlayQueue,callListenPlaybackPlayTrack,callListenPlaybackPrevious,callListenPlaybackRedoQueue,callListenPlaybackRemoveFromQueue,callListenPlaybackResume,callListenPlaybackSeek,callListenPlaybackSetRepeatMode,callListenPlaybackSetShuffle,callListenPlaybackSetVolume,callListenPlaybackUndoQueue,listenRepeatModeFromPlayMode,type ListenPlaybackSnapshot } from "@/app/main/listen/playback-api";
import { hasTrustedListenOnlineArtist,isMissingListenArtistLabel } from "@/app/main/listen/playback-helpers";
import { useListenPlaybackStore } from "@/app/main/listen/playback-store";
import { pushListenForwardSkipIndex,resolveListenQueueNextAction,resolveListenQueuePreviousAction } from "@/app/main/listen/queue";
import { buildListenHighQualityThumbnailURL,buildListenImageCacheURL,buildYouTubePosterURL,dedupeLibraryShelves,dedupeOnlineItems,dedupePlaylistItems,readListenStorageState,updateListenProgressMap,writeListenStorageState } from "@/app/main/listen/storage";
import type { ListenArtistBrowseState,ListenArtistItem,ListenCategoryItem,ListenLibraryShelf,ListenLiveGroup,ListenLiveStatus,ListenMode,ListenNativePlayerEvent,ListenNowPlayingStatus,ListenOnlineBrowseDetail,ListenOnlineBrowseSource,ListenOnlineItem,ListenPageProps,ListenPlayMode,ListenPlaybackProgressState,ListenPlayerCommand,ListenPlaylistItem,ListenPlaylistLibraryAction,ListenRemotePlaybackState,ListenSidebarView,ListenTrackArtist } from "@/app/main/listen/types";
export { ListenLocalPreviewPlayer } from "@/app/main/listen/LocalPreviewPlayer";
export type { ListenExternalCommand,ListenLocalPreviewTrack,ListenMode,ListenNowPlayingStatus } from "@/app/main/listen/types";

const LISTEN_UNKNOWN_ARTIST = "Unknown Artist";
const LISTEN_LIVE_STATUS_POLL_MS = 60_000;
const LISTEN_LIVE_STATUS_WARM_POLL_MS = 4_000;
const LISTEN_ARTIST_SHELF_CONTINUATION_MAX_PAGES = 20;

type ListenLibraryPageCacheEntry = {
  playlists: ListenPlaylistItem[];
  artists: ListenArtistItem[];
  shelves: ListenLibraryShelf[];
  continuation: string;
  reloadToken: number;
};

function resolveListenLibraryPageCacheKey(
  source: ListenOnlineBrowseSource,
  detail: ListenOnlineBrowseDetail | null,
  language: string,
) {
  const locale = language.trim() || "en";
  if (!detail) {
    return `source:${source}:locale:${locale}`;
  }
  return [
    "detail",
    source,
    locale,
    detail.browseId.trim(),
    detail.params.trim(),
  ].join(":");
}

function mergeListenNativeTrackItem(
  incoming: ListenOnlineItem,
  current: ListenOnlineItem,
): ListenOnlineItem {
  const videoId = current.videoId || incoming.videoId;
  const incomingTitle = incoming.title.trim();
  const currentTitle = current.title.trim();
  const incomingArtistTrusted = hasTrustedListenOnlineArtist(incoming);
  const currentArtistTrusted = hasTrustedListenOnlineArtist(current);
  const incomingVideoKnown = incoming.videoAvailabilityKnown === true;
  const currentVideoKnown = current.videoAvailabilityKnown === true;
  const incomingArtists = normalizeListenTrackArtists(incoming.artists);
  const currentArtists = normalizeListenTrackArtists(current.artists);
  return {
    ...current,
    videoId,
    title:
      incomingTitle && incomingTitle !== videoId
        ? incoming.title
        : currentTitle || videoId,
    channel: incomingArtistTrusted
      ? incoming.channel
      : currentArtistTrusted
        ? current.channel
        : LISTEN_UNKNOWN_ARTIST,
    artists: incomingArtists ?? currentArtists,
    artistBrowseId: incoming.artistBrowseId || current.artistBrowseId,
    artistSource: incomingArtistTrusted
      ? incoming.artistSource || (incoming.artistBrowseId ? "api-linked" : undefined)
      : currentArtistTrusted
        ? current.artistSource || (current.artistBrowseId ? "api-linked" : undefined)
        : incoming.artistSource || current.artistSource,
    description: incoming.description || current.description,
    durationLabel: incoming.durationLabel || current.durationLabel,
    playCountLabel: incoming.playCountLabel || current.playCountLabel,
    thumbnailUrl: incoming.thumbnailUrl || current.thumbnailUrl,
    musicVideoType: incoming.musicVideoType || current.musicVideoType,
    hasVideo: incomingVideoKnown
      ? incoming.hasVideo === true
      : incoming.hasVideo === true || current.hasVideo === true,
    videoAvailabilityKnown:
      incomingVideoKnown || currentVideoKnown ? true : undefined,
  };
}

function normalizeListenTrackArtists(
  artists: ListenTrackArtist[] | undefined,
): ListenTrackArtist[] | undefined {
  if (!Array.isArray(artists) || artists.length === 0) {
    return undefined;
  }
  const result: ListenTrackArtist[] = [];
  const seen = new Set<string>();
  for (const artist of artists) {
    const name = artist.name.trim();
    const browseId = artist.browseId?.trim() ?? "";
    if (!name) {
      continue;
    }
    const key = browseId || name.toLocaleLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push({
      name,
      browseId: browseId || undefined,
      thumbnailUrl: artist.thumbnailUrl?.trim() || undefined,
    });
  }
  return result.length > 0 ? result : undefined;
}

const LISTEN_REMOTE_PLAYBACK_STATES: ListenRemotePlaybackState[] = [
  "idle",
  "loading",
  "playing",
  "paused",
  "buffering",
  "ended",
  "error",
];

function stringFromNativeStatus(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function secondsFromNativeStatus(value: unknown) {
  return typeof value === "number" && Number.isFinite(value)
    ? Math.max(0, value)
    : 0;
}

function normalizeNativePlaybackState(
  value: unknown,
  fallback: ListenRemotePlaybackState,
): ListenRemotePlaybackState {
  const state = stringFromNativeStatus(value) as ListenRemotePlaybackState;
  return LISTEN_REMOTE_PLAYBACK_STATES.includes(state) ? state : fallback;
}

function createNativeOnlineItem(params: {
  videoId: string;
  title?: string;
  artist?: string;
  thumbnailUrl?: string;
}): ListenOnlineItem {
  const videoId = params.videoId.trim();
  const title = params.title?.trim() || videoId;
  const rawArtist = params.artist?.trim() || "YouTube Music";
  const artist = isMissingListenArtistLabel(rawArtist)
    ? LISTEN_UNKNOWN_ARTIST
    : rawArtist;
  const thumbnailUrl = params.thumbnailUrl?.trim() || "";
  return {
    id: `ytmusic-native-${videoId}`,
    group: "playlist",
    videoId,
    title,
    channel: artist,
    artistSource: "observed",
    description: "",
    durationLabel: "",
    thumbnailUrl: thumbnailUrl,
  };
}

function cleanListenPlaylistPlaybackArtist(value: string) {
  let artist = value.trim();
  if (!artist) {
    return "";
  }
  if (artist === "Album") {
    return LISTEN_UNKNOWN_ARTIST;
  }
  if (artist.startsWith("Album, ")) {
    artist = artist.slice(7).trim();
  }
  if (artist.includes("Album,")) {
    const parts = artist.split(/,(.*)/s);
    if (parts[1]) {
      artist = parts[1].trim();
    }
  }
  if (isMissingListenArtistLabel(artist)) {
    return LISTEN_UNKNOWN_ARTIST;
  }
  return artist;
}

function applyListenPlaylistPlaybackFallback(
  items: ListenOnlineItem[],
  fallbackArtist: string,
) {
  const cleanedFallback = cleanListenPlaylistPlaybackArtist(fallbackArtist);
  return items.map((item) => {
    let channel = item.channel.trim();
    if (channel === "Album") {
      channel = "";
    } else if (channel.startsWith("Album, ")) {
      channel = channel.slice(7).trim();
    }
    if (isMissingListenArtistLabel(channel)) {
      channel = "";
    }
    if (!channel && cleanedFallback) {
      channel = cleanedFallback;
    }
    return channel === item.channel ? item : { ...item, channel };
  });
}

function resolveListenPlaylistPlaybackFallbackArtist(
  playlist: ListenPlaylistItem,
  detailAuthor: string,
) {
  const author = detailAuthor.trim();
  if (author && !isMissingListenArtistLabel(author)) {
    return author;
  }
  const channel = playlist.channel.trim();
  if (isMissingListenArtistLabel(channel)) {
    return playlist.description.trim();
  }
  const normalizedChannel = channel.toLowerCase();
  if (
    normalizedChannel === "album" ||
    normalizedChannel === "专辑" ||
    normalizedChannel === "專輯" ||
    normalizedChannel === "single" ||
    normalizedChannel === "单曲" ||
    normalizedChannel === "單曲" ||
    normalizedChannel === "ep"
  ) {
    return playlist.description.trim() || channel;
  }
  return channel;
}

function nativeStatusToPlayerEvent(
  value: unknown,
  source = "listen-youtube-music-player",
): ListenNativePlayerEvent | null {
  const record =
    value && typeof value === "object"
      ? (value as Record<string, unknown>)
      : null;
  if (!record || record.available !== true) {
    return null;
  }
  const videoId =
    stringFromNativeStatus(record.observedVideoId) ||
    stringFromNativeStatus(record.videoId);
  if (!videoId) {
    return null;
  }
  const state = normalizeNativePlaybackState(record.state, "paused");
  if (state === "idle") {
    return null;
  }
  return {
    source,
    type: "status",
    state,
    videoId,
    observedVideoId: videoId,
    requestedVideoId: stringFromNativeStatus(record.videoId),
    title: stringFromNativeStatus(record.title),
    artist: stringFromNativeStatus(record.artist),
    thumbnailUrl: stringFromNativeStatus(record.thumbnailUrl),
    likeStatus: stringFromNativeStatus(record.likeStatus),
    currentTime: secondsFromNativeStatus(record.currentTime),
    duration: secondsFromNativeStatus(record.duration),
    bufferedTime: secondsFromNativeStatus(record.bufferedTime),
    advertising: record.advertising === true,
    adLabel: stringFromNativeStatus(record.adLabel),
    errorCode: stringFromNativeStatus(record.errorCode),
    errorMessage: stringFromNativeStatus(record.errorMessage),
  };
}

function favoriteFromLikeStatus(value?: string) {
  switch ((value ?? "").trim().toUpperCase()) {
    case "LIKE":
    case "LIKE_STATUS_LIKE":
    case "LIKED":
      return true;
    case "DISLIKE":
    case "LIKE_STATUS_DISLIKE":
    case "INDIFFERENT":
    case "LIKE_STATUS_INDIFFERENT":
    case "NONE":
      return false;
    default:
      return null;
  }
}

function collectFavoriteSeedVideoIds(
  groups: Array<readonly ListenOnlineItem[]>,
  known: Record<string, boolean>,
) {
  const ids: string[] = [];
  const seen = new Set<string>();
  for (const group of groups) {
    for (const item of group) {
      const videoId = item.videoId.trim();
      if (!videoId || item.group === "live" || known[videoId] !== undefined) {
        continue;
      }
      if (seen.has(videoId)) {
        continue;
      }
      seen.add(videoId);
      ids.push(videoId);
      if (ids.length >= 50) {
        return ids;
      }
    }
  }
  return ids;
}

const LISTEN_FAVORITE_OBSERVATION_GRACE_MS = 30_000;

type ListenFavoriteOverride = {
  liked: boolean;
  expiresAt: number;
};

function resolveListenLiveCatalogEventKey(payload: unknown) {
  if (!payload || typeof payload !== "object") {
    return "";
  }
  const record = payload as Record<string, unknown>;
  return [
    record.fingerprint,
    record.version,
    record.updatedAt,
    record.sha256,
    record.hash,
    record.url,
  ]
    .map((value) => (typeof value === "string" ? value.trim() : ""))
    .filter(Boolean)
    .join("|");
}

function resolveListenShelfQueueTitle(
  shelf: ListenLibraryShelf,
  text: ListenPageProps["text"],
  fallback: string,
) {
  if (shelf.id === LISTEN_LIKED_SONGS_SHELF_ID) {
    return text.listen.likedMusic;
  }
  return shelf.title.trim() || fallback;
}

function shuffleListenOnlineItems(items: ListenOnlineItem[]) {
  const nextItems = [...items];
  for (let index = nextItems.length - 1; index > 0; index -= 1) {
    const swapIndex = Math.floor(Math.random() * (index + 1));
    [nextItems[index], nextItems[swapIndex]] = [
      nextItems[swapIndex],
      nextItems[index],
    ];
  }
  return nextItems;
}

export function ListenPage(props: ListenPageProps) {
  const isWindows = System.IsWindows();
  const isMac = System.IsMac();
  const initialPersistedState = React.useMemo(
    () => readListenStorageState(),
    [],
  );
  const openLibraryPath = useOpenLibraryPath();
  const appSessions = useAppSessions();
  const startAppSessionConnect = useStartAppSessionConnect();
  const cancelAppSessionConnect = useCancelAppSessionConnect();
  const clearAppSession = useClearAppSession();
  const [youtubeConnectSessionId, setYouTubeConnectSessionId] =
    React.useState("");
  const youtubeConnectSession = useAppSessionConnectSession(
    { sessionId: youtubeConnectSessionId },
    youtubeConnectSessionId.trim().length > 0,
  );
  const [youtubeLoggedOutOverride, setYouTubeLoggedOutOverride] =
    React.useState(false);
  const [mode, setMode] = React.useState<ListenMode>(
    initialPersistedState.mode,
  );
  const [playbackMode, setPlaybackMode] = React.useState<ListenMode>(
    initialPersistedState.playbackMode,
  );
  const modeRef = React.useRef(mode);
  const playbackModeRef = React.useRef(playbackMode);
  const [sidebarView, setSidebarView] =
    React.useState<ListenSidebarView>("browse");
  const [onlineBrowseSource, setOnlineBrowseSource] =
    React.useState<ListenOnlineBrowseSource>("home");
  const [onlineBrowseDetail, setOnlineBrowseDetail] =
    React.useState<ListenOnlineBrowseDetail | null>(null);
  const [query, setQuery] = React.useState("");
  const [listOpen, setListOpen] = React.useState(
    initialPersistedState.listOpen,
  );
  const [selectedLiveId, setSelectedLiveId] = React.useState(
    initialPersistedState.selectedLiveId,
  );
  const [liveSelectionArmed, setLiveSelectionArmed] = React.useState(false);
  const [liveGroups, setLiveGroups] = React.useState<ListenLiveGroup[]>([]);
  const [selectedLiveGroupId, setSelectedLiveGroupId] = React.useState("");
  const [liveCatalogLoading, setLiveCatalogLoading] = React.useState(false);
  const [liveCatalogError, setLiveCatalogError] = React.useState(false);
  const [liveCatalogMessage, setLiveCatalogMessage] = React.useState("");
  const [liveCatalogReloadToken, setLiveCatalogReloadToken] = React.useState(0);
  const [liveUserCatalog, setLiveUserCatalog] =
    React.useState<ListenLiveUserCatalog>({ columns: [], channels: [] });
  const [liveUserCatalogLoading, setLiveUserCatalogLoading] =
    React.useState(false);
  const [liveUserCatalogSaving, setLiveUserCatalogSaving] =
    React.useState(false);
  const [liveUserCatalogError, setLiveUserCatalogError] = React.useState("");
  const [liveUserCatalogReloadToken, setLiveUserCatalogReloadToken] =
    React.useState(0);
  const [liveStatusByVideoId, setLiveStatusByVideoId] = React.useState<
    Record<string, ListenLiveStatus>
  >({});
  const [browsePlaylistId, setBrowsePlaylistId] = React.useState(
    initialPersistedState.browsePlaylistId,
  );
  const [selectedLocalId, setSelectedLocalId] = React.useState(
    initialPersistedState.selectedLocalId,
  );
  const [localPlayMode, setLocalPlayMode] = React.useState<ListenPlayMode>(
    initialPersistedState.playMode,
  );
  const [searchItems, setSearchItems] = React.useState<ListenOnlineItem[]>([]);
  const [searchArtists, setSearchArtists] = React.useState<ListenArtistItem[]>([]);
  const [searchPlaylists, setSearchPlaylists] = React.useState<
    ListenPlaylistItem[]
  >([]);
  const [searchContinuation, setSearchContinuation] = React.useState("");
  const [searchAppending, setSearchAppending] = React.useState(false);
  const [searchLoading, setSearchLoading] = React.useState(false);
  const [searchError, setSearchError] = React.useState(false);
  const [homeShelves, setHomeShelves] = React.useState<ListenLibraryShelf[]>(
    [],
  );
  const [libraryPlaylists, setLibraryPlaylists] = React.useState<
    ListenPlaylistItem[]
  >([]);
  const [libraryArtists, setLibraryArtists] = React.useState<ListenArtistItem[]>([]);
  const [libraryContinuation, setLibraryContinuation] = React.useState("");
  const [libraryLoading, setLibraryLoading] = React.useState(false);
  const [libraryAppending, setLibraryAppending] = React.useState(false);
  const [libraryError, setLibraryError] = React.useState(false);
  const [libraryErrorCode, setLibraryErrorCode] = React.useState("");
  const [libraryReloadToken, setLibraryReloadToken] = React.useState(0);
  const [museAccountReloadToken, setMuseAccountReloadToken] = React.useState(0);
  const [museManualRefreshKind, setMuseManualRefreshKind] = React.useState<
    "" | "artist" | "library" | "playlist" | "search"
  >("");
  const [playlistTracks, setPlaylistTracks] = React.useState<
    ListenOnlineItem[]
  >([]);
  const [playlistContinuation, setPlaylistContinuation] = React.useState("");
  const [playlistDetailAuthor, setPlaylistDetailAuthor] = React.useState("");
  const [playlistDetailTitle, setPlaylistDetailTitle] = React.useState("");
  const [playlistLoading, setPlaylistLoading] = React.useState(false);
  const [playlistAppending, setPlaylistAppending] = React.useState(false);
  const [playlistMutationPlaylistId, setPlaylistMutationPlaylistId] =
    React.useState("");
  const [playlistMutationAction, setPlaylistMutationAction] =
    React.useState<ListenPlaylistLibraryAction | null>(null);
  const [artistBrowsePage, setArtistBrowsePage] =
    React.useState<ListenArtistBrowseState | null>(null);
  const [artistActionBusy, setArtistActionBusy] = React.useState<
    "" | "mix" | "subscribe"
  >("");
  const [onlinePlayerCommand, setOnlinePlayerCommand] =
    React.useState<ListenPlayerCommand | null>(null);
  const [livePlaying, setLivePlaying] = React.useState(false);
  const [liveState, setLiveState] =
    React.useState<ListenRemotePlaybackState>("idle");
  const [liveProgress, setLiveProgress] = React.useState<
    ListenPlaybackProgressState & { videoId: string }
  >({
    videoId: "",
    currentTime: 0,
    duration: 0,
    bufferedTime: 0,
  });
  const [localPlaying, setLocalPlaying] = React.useState(false);
  const [localRelinkDialogOpen, setLocalRelinkDialogOpen] =
    React.useState(false);
  const [localPlayerCommand, setLocalPlayerCommand] =
    React.useState<ListenPlayerCommand | null>(null);
  const [muted, setMuted] = React.useState(initialPersistedState.muted);
  const [volume, setVolume] = React.useState(initialPersistedState.volume);
  const lastNonZeroVolumeRef = React.useRef(
    initialPersistedState.volume > 0 ? initialPersistedState.volume : 1,
  );
  const [onlineFavoriteByVideoId, setOnlineFavoriteByVideoId] = React.useState<
    Record<string, boolean>
  >({});
  const [favoriteLoadingVideoId, setFavoriteLoadingVideoId] =
    React.useState("");
  const [favoriteMutationVideoId, setFavoriteMutationVideoId] =
    React.useState("");
  const [localProgress, setLocalProgress] =
    React.useState<ListenPlaybackProgressState>({
      currentTime: 0,
      duration: 0,
      bufferedTime: 0,
    });
  const [localProgressByPath, setLocalProgressByPath] = React.useState<
    Record<string, number>
  >(() => initialPersistedState.localProgressByPath);
  const [onlineProgressByVideoId, setOnlineProgressByVideoId] = React.useState<
    Record<string, number>
  >(() => initialPersistedState.onlineProgressByVideoId);
  const [playbackSessionStarted, setPlaybackSessionStarted] =
    React.useState(false);
  const handledExternalCommandRef = React.useRef(0);
  const liveCatalogRealtimeKeyRef = React.useRef("");
  const nativeTrackLookupRef = React.useRef<Map<string, AbortController>>(
    new Map(),
  );
  const nativeStatusRestoreAttemptedRef = React.useRef(false);
  const forwardSkipIndexStackRef = React.useRef<number[]>([]);
  const onlinePlaybackActionEpochRef = React.useRef(0);
  const onlinePlaybackActionPendingRef = React.useRef(false);
  const museAccountConnectedRef = React.useRef(false);
  const resetOnlinePlaybackProjectionRef = React.useRef<() => void>(() => {});
  const favoriteOverrideByVideoIdRef = React.useRef<
    Record<string, ListenFavoriteOverride>
  >({});
  const libraryPageCacheRef = React.useRef<
    Map<string, ListenLibraryPageCacheEntry>
  >(new Map());
  const activeLibraryPageCacheKeyRef = React.useRef("");

  const localTrackIndex = useListenLocalTracks(
    props.libraries,
    props.httpBaseURL,
  );
  const localTracks = localTrackIndex.tracks;
  const listenLanguage = props.text.locale;
  const rememberFavoriteOverride = React.useCallback(
    (videoId: string, liked: boolean) => {
      const trimmed = videoId.trim();
      if (!trimmed) {
        return;
      }
      favoriteOverrideByVideoIdRef.current[trimmed] = {
        liked,
        expiresAt: Date.now() + LISTEN_FAVORITE_OBSERVATION_GRACE_MS,
      };
    },
    [],
  );
  const clearFavoriteOverride = React.useCallback((videoId: string) => {
    const trimmed = videoId.trim();
    if (trimmed) {
      delete favoriteOverrideByVideoIdRef.current[trimmed];
    }
  }, []);
  const shouldIgnoreObservedFavorite = React.useCallback(
    (videoId: string, observedLiked: boolean) => {
      const trimmed = videoId.trim();
      const override = favoriteOverrideByVideoIdRef.current[trimmed];
      if (!override) {
        return false;
      }
      if (Date.now() > override.expiresAt) {
        delete favoriteOverrideByVideoIdRef.current[trimmed];
        return false;
      }
      if (override.liked === observedLiked) {
        delete favoriteOverrideByVideoIdRef.current[trimmed];
        return false;
      }
      return true;
    },
    [],
  );

  const reloadMuseAccountData = React.useCallback(() => {
    libraryPageCacheRef.current.clear();
    activeLibraryPageCacheKeyRef.current = "";
    favoriteOverrideByVideoIdRef.current = {};
    setHomeShelves([]);
    setLibraryPlaylists([]);
    setLibraryArtists([]);
    setLibraryContinuation("");
    setLibraryAppending(false);
    setLibraryError(false);
    setLibraryErrorCode("");
    setSearchItems([]);
    setSearchArtists([]);
    setSearchPlaylists([]);
    setSearchContinuation("");
    setSearchAppending(false);
    setSearchLoading(false);
    setSearchError(false);
    setPlaylistTracks([]);
    setPlaylistContinuation("");
    setPlaylistDetailAuthor("");
    setPlaylistDetailTitle("");
    setPlaylistAppending(false);
    setPlaylistLoading(false);
    setArtistActionBusy("");
    setArtistBrowsePage((current) =>
      current
        ? {
            ...current,
            items: [],
            shelves: [],
            continuation: "",
            loading: true,
            appending: false,
            error: false,
          }
        : current,
    );
    setOnlineFavoriteByVideoId({});
    setFavoriteLoadingVideoId("");
    setFavoriteMutationVideoId("");
    setLibraryReloadToken((current) => current + 1);
    setMuseAccountReloadToken((current) => current + 1);
  }, []);

  const resetMuseAccountViewForLogout = React.useCallback(() => {
    libraryPageCacheRef.current.clear();
    activeLibraryPageCacheKeyRef.current = "";
    favoriteOverrideByVideoIdRef.current = {};
    museAccountConnectedRef.current = false;
    onlinePlaybackActionEpochRef.current += 1;
    onlinePlaybackActionPendingRef.current = false;
    resetOnlinePlaybackProjectionRef.current();
    setMuseManualRefreshKind("");
    setPlaybackSessionStarted(false);
    setQuery("");
    setSidebarView("browse");
    setOnlineBrowseSource("home");
    setOnlineBrowseDetail(null);
    setBrowsePlaylistId("");
    setArtistBrowsePage(null);
    setOnlinePlayerCommand(null);
    setHomeShelves([]);
    setLibraryPlaylists([]);
    setLibraryArtists([]);
    setLibraryContinuation("");
    setLibraryAppending(false);
    setLibraryLoading(false);
    setLibraryError(false);
    setLibraryErrorCode("");
    setPlaylistTracks([]);
    setPlaylistContinuation("");
    setPlaylistDetailAuthor("");
    setPlaylistDetailTitle("");
    setPlaylistAppending(false);
    setPlaylistLoading(false);
    setArtistActionBusy("");
    setSearchItems([]);
    setSearchArtists([]);
    setSearchPlaylists([]);
    setSearchContinuation("");
    setSearchAppending(false);
    setSearchLoading(false);
    setSearchError(false);
    setOnlineFavoriteByVideoId({});
    setFavoriteLoadingVideoId("");
    setFavoriteMutationVideoId("");
  }, []);

  const activateMusePlayback = React.useCallback(() => {
    setMode("muse");
    setPlaybackMode("muse");
  }, []);
  const shouldAcceptMuseSnapshot = React.useCallback(
    (snapshot: ListenPlaybackSnapshot) => {
      const hasMuseSession =
        Boolean(snapshot.currentTrack) ||
        snapshot.queue.length > 0 ||
        Boolean(snapshot.pendingPlayVideoId);
      return (
        museAccountConnectedRef.current &&
        (hasMuseSession || playbackModeRef.current === "muse")
      );
    },
    [],
  );
  const shouldActivateMuseSnapshot = React.useCallback(
    (snapshot: ListenPlaybackSnapshot) =>
      modeRef.current === "muse" &&
      playbackModeRef.current === "muse" &&
      (snapshot.state === "loading" ||
        snapshot.state === "buffering" ||
        snapshot.state === "playing"),
    [],
  );
  const {
    projection: musePlayback,
    applySnapshot: applyOnlinePlaybackSnapshot,
    reset: resetOnlinePlaybackProjection,
  } = useListenPlaybackStore({
      httpBaseURL: props.httpBaseURL,
      shouldAcceptSnapshot: shouldAcceptMuseSnapshot,
      shouldActivateSnapshot: shouldActivateMuseSnapshot,
      onActivate: activateMusePlayback,
    });
  resetOnlinePlaybackProjectionRef.current = resetOnlinePlaybackProjection;
  const onlineQueueState = musePlayback.queueState;
  const onlinePlaybackQueue = musePlayback.queueItems;
  const selectedOnlineId = musePlayback.selectedId;
  const onlineQueueCanUndo = musePlayback.canUndoQueue;
  const onlineQueueCanRedo = musePlayback.canRedoQueue;
  const onlinePlaybackArmed =
    playbackMode === "hush" ? Boolean(selectedLiveId || liveProgress.videoId) : musePlayback.armed;
  const onlinePlaying =
    playbackMode === "hush" ? livePlaying : musePlayback.playing;
  const onlineState = playbackMode === "hush" ? liveState : musePlayback.state;
  const onlineProgress =
    playbackMode === "hush" ? liveProgress : musePlayback.progress;
  const onlineObservedPlaybackAudioQuality =
    playbackMode === "muse" ? musePlayback.observedPlaybackAudioQuality : "";
  const playMode =
    playbackMode === "muse" ? musePlayback.playMode : localPlayMode;
  const youtubeAppSession = React.useMemo(
    () =>
      (appSessions.data ?? []).find(
        (session) => session.siteKey.trim().toLowerCase() === "youtube",
      ) ?? null,
    [appSessions.data],
  );
  const youtubeConnectBusy =
    startAppSessionConnect.isPending ||
    cancelAppSessionConnect.isPending ||
    youtubeConnectSessionId.trim().length > 0;
  const youtubeAppSessionConnected =
    Boolean(youtubeAppSession) &&
    (youtubeAppSession?.status === "connected" ||
      youtubeAppSession?.credentialState === "app_session" ||
      Boolean(youtubeAppSession?.account?.displayName?.trim()) ||
      Boolean(youtubeAppSession?.account?.avatarURL?.trim()));
  const museAccountConnected =
    !youtubeLoggedOutOverride && youtubeAppSessionConnected;
  museAccountConnectedRef.current = museAccountConnected;
  React.useEffect(() => {
    if (!museAccountConnected) {
      resetMuseAccountViewForLogout();
    }
  }, [museAccountConnected, resetMuseAccountViewForLogout]);
  const museAccountName = museAccountConnected
    ? youtubeAppSession?.account?.displayName?.trim() ||
      props.text.listen.museAccountFallbackName
    : props.text.listen.museAccountDisconnectedName;
  const museAccountAvatarURL = museAccountConnected
    ? youtubeAppSession?.account?.avatarURL?.trim() || ""
    : "";
  const museAccountBusy =
    youtubeConnectBusy ||
    clearAppSession.isPending ||
    museManualRefreshKind !== "";
  const listenMuseLabel = props.text.listen.muse;
  const onlineAuthRequiredLabel = props.text.listen.onlineAuthRequired;
  const connectYouTube = React.useCallback(async () => {
    if (youtubeConnectBusy) {
      return;
    }
    const appSession = youtubeAppSession;
    if (!appSession) {
      props.onOpenConnections();
      return;
    }
    setMode("muse");
    setPlaybackMode("muse");
    try {
      const result = await startAppSessionConnect.mutateAsync({ id: appSession.id });
      setYouTubeConnectSessionId(result.sessionId);
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: listenMuseLabel,
        description:
          error instanceof Error && error.message.trim()
            ? error.message
            : onlineAuthRequiredLabel,
      });
    }
  }, [
    listenMuseLabel,
    onlineAuthRequiredLabel,
    props.onOpenConnections,
    startAppSessionConnect,
    youtubeConnectBusy,
    youtubeAppSession,
  ]);
  const signOutMuseAccount = React.useCallback(async () => {
    if (clearAppSession.isPending) {
      return;
    }
    const appSession = youtubeAppSession;
    if (!appSession) {
      return;
    }
    try {
      await clearAppSession.mutateAsync({ id: appSession.id });
      setYouTubeLoggedOutOverride(true);
      resetMuseAccountViewForLogout();
      void appSessions.refetch();
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: listenMuseLabel,
        description:
          error instanceof Error && error.message.trim()
            ? error.message
            : onlineAuthRequiredLabel,
      });
    }
  }, [
    appSessions,
    clearAppSession,
    listenMuseLabel,
    onlineAuthRequiredLabel,
    resetMuseAccountViewForLogout,
    youtubeAppSession,
  ]);

  React.useEffect(() => {
    modeRef.current = mode;
    playbackModeRef.current = playbackMode;
  }, [mode, playbackMode]);

  React.useEffect(() => {
    const session = youtubeConnectSession.data;
    const sessionId = youtubeConnectSessionId.trim();
    if (!session || !sessionId || session.sessionId !== sessionId) {
      return;
    }
    if (session.state === "running") {
      return;
    }
    setYouTubeConnectSessionId("");
    void cancelAppSessionConnect.mutateAsync({ sessionId }).catch(() => {
      // The app-session window may already have finalized and removed itself.
    });
    if (session.state === "completed" && session.saved) {
      setYouTubeLoggedOutOverride(false);
      reloadMuseAccountData();
      void appSessions.refetch();
      return;
    }
    messageBus.publishToast({
      intent: "danger",
      title: listenMuseLabel,
      description: session.error || onlineAuthRequiredLabel,
    });
  }, [
    appSessions,
    cancelAppSessionConnect,
    listenMuseLabel,
    onlineAuthRequiredLabel,
    reloadMuseAccountData,
    youtubeConnectSession.data,
    youtubeConnectSessionId,
  ]);

  React.useEffect(() => {
    const sessionId = youtubeConnectSessionId.trim();
    if (!sessionId || !youtubeConnectSession.error) {
      return;
    }
    setYouTubeConnectSessionId("");
    messageBus.publishToast({
      intent: "danger",
      title: listenMuseLabel,
      description: onlineAuthRequiredLabel,
    });
  }, [
    listenMuseLabel,
    onlineAuthRequiredLabel,
    youtubeConnectSession.error,
    youtubeConnectSessionId,
  ]);

  React.useEffect(() => {
    const offYouTubeAppSessionChanged = Events.On(
      LISTEN_YOUTUBE_APP_SESSION_CHANGED_EVENT,
      (event: any) => {
        const payload = event?.data ?? event;
        const siteKey =
          typeof payload?.siteKey === "string"
            ? payload.siteKey.trim().toLowerCase()
            : "";
        if (siteKey && siteKey !== "youtube") {
          return;
        }
        const action =
          typeof payload?.action === "string"
            ? payload.action.trim().toLowerCase()
            : "";
        const status =
          typeof payload?.status === "string"
            ? payload.status.trim().toLowerCase()
            : "";
        if (action === "clear" || status === "disconnected") {
          setYouTubeLoggedOutOverride(true);
          resetMuseAccountViewForLogout();
        } else if (action === "finish" || status === "connected") {
          setYouTubeLoggedOutOverride(false);
        }
        reloadMuseAccountData();
      },
    );
    return () => offYouTubeAppSessionChanged();
  }, [reloadMuseAccountData, resetMuseAccountViewForLogout]);

  React.useEffect(
    () => () => {
      nativeTrackLookupRef.current.forEach((controller) =>
        controller.abort(),
      );
      nativeTrackLookupRef.current.clear();
    },
    [],
  );

  React.useEffect(() => {
    libraryPageCacheRef.current.clear();
    activeLibraryPageCacheKeyRef.current = "";
  }, [listenLanguage, props.httpBaseURL]);

  React.useEffect(() => {
    setOnlineFavoriteByVideoId({});
    setFavoriteLoadingVideoId("");
    setFavoriteMutationVideoId("");
  }, [listenLanguage, props.httpBaseURL]);

  React.useEffect(() => {
    if (
      selectedLocalId &&
      !localTrackIndex.loading &&
      !localTracks.some((item) => item.id === selectedLocalId)
    ) {
      setSelectedLocalId("");
    }
  }, [localTrackIndex.loading, localTracks, selectedLocalId]);

  const normalizedQuery = normalizeSearch(query);
  const refreshMusePage = React.useCallback(() => {
    if (mode !== "muse") {
      return;
    }
    if (artistBrowsePage) {
      setMuseManualRefreshKind("artist");
      setMuseAccountReloadToken((current) => current + 1);
      setArtistBrowsePage((current) =>
        current
          ? {
              ...current,
              items: [],
              shelves: [],
              continuation: "",
              loading: true,
              appending: false,
              error: false,
            }
          : current,
      );
      return;
    }
    if (browsePlaylistId.trim()) {
      setMuseManualRefreshKind("playlist");
      setMuseAccountReloadToken((current) => current + 1);
      setPlaylistTracks([]);
      setPlaylistContinuation("");
      setPlaylistDetailAuthor("");
      setPlaylistDetailTitle("");
      setPlaylistAppending(false);
      setPlaylistLoading(true);
      return;
    }
    if (normalizedQuery.length >= 2) {
      setMuseManualRefreshKind("search");
      setMuseAccountReloadToken((current) => current + 1);
      setSearchItems([]);
      setSearchArtists([]);
      setSearchPlaylists([]);
      setSearchContinuation("");
      setSearchAppending(false);
      setSearchLoading(true);
      setSearchError(false);
      return;
    }
    setMuseManualRefreshKind("library");
    setMuseAccountReloadToken((current) => current + 1);
    const activeCacheKey = activeLibraryPageCacheKeyRef.current;
    if (activeCacheKey) {
      libraryPageCacheRef.current.delete(activeCacheKey);
    } else {
      libraryPageCacheRef.current.clear();
    }
    setLibraryPlaylists([]);
    setLibraryArtists([]);
    setHomeShelves([]);
    setLibraryContinuation("");
    setLibraryAppending(false);
    setLibraryLoading(true);
    setLibraryError(false);
    setLibraryErrorCode("");
    setLibraryReloadToken((current) => current + 1);
  }, [artistBrowsePage, browsePlaylistId, mode, normalizedQuery.length]);

  React.useEffect(() => {
    if (
      mode !== "muse" ||
      !museAccountConnected ||
      artistBrowsePage ||
      normalizedQuery.length < 2
    ) {
      setMuseManualRefreshKind((current) =>
        current === "search" ? "" : current,
      );
      setSearchItems([]);
      setSearchArtists([]);
      setSearchPlaylists([]);
      setSearchContinuation("");
      setSearchAppending(false);
      setSearchLoading(false);
      setSearchError(false);
      return;
    }

    const controller = new AbortController();
    setSearchLoading(false);
    setSearchError(false);
    const timer = window.setTimeout(() => {
      setSearchLoading(true);
      void fetchListenSearch(props.httpBaseURL, query, controller.signal, listenLanguage)
        .then((payload) => {
          if (!controller.signal.aborted) {
            setSearchItems(payload.items);
            setSearchArtists(payload.artists);
            setSearchPlaylists(payload.playlists);
            setSearchContinuation(payload.continuation);
          }
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            setSearchItems([]);
            setSearchArtists([]);
            setSearchPlaylists([]);
            setSearchContinuation("");
            setSearchError(true);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) {
            setSearchLoading(false);
            setMuseManualRefreshKind((current) =>
              current === "search" ? "" : current,
            );
          }
        });
    }, 350);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [artistBrowsePage, listenLanguage, mode, museAccountConnected, museAccountReloadToken, normalizedQuery, props.httpBaseURL, query]);

  React.useEffect(() => {
    if (mode !== "muse") {
      activeLibraryPageCacheKeyRef.current = "";
      setMuseManualRefreshKind((current) =>
        current === "library" ? "" : current,
      );
      return;
    }
    if (!museAccountConnected) {
      activeLibraryPageCacheKeyRef.current = "";
      libraryPageCacheRef.current.clear();
      setHomeShelves([]);
      setLibraryPlaylists([]);
      setLibraryArtists([]);
      setLibraryContinuation("");
      setLibraryLoading(false);
      setLibraryAppending(false);
      setLibraryError(false);
      setLibraryErrorCode("");
      setMuseManualRefreshKind((current) =>
        current === "library" ? "" : current,
      );
      return;
    }
    const cacheKey = resolveListenLibraryPageCacheKey(
      onlineBrowseSource,
      onlineBrowseDetail,
      listenLanguage,
    );
    activeLibraryPageCacheKeyRef.current = cacheKey;
    const cachedPage = libraryPageCacheRef.current.get(cacheKey);
    if (cachedPage && cachedPage.reloadToken === libraryReloadToken) {
      setLibraryPlaylists(cachedPage.playlists);
      setLibraryArtists(cachedPage.artists);
      setHomeShelves(cachedPage.shelves);
      setLibraryContinuation(cachedPage.continuation);
      setLibraryLoading(false);
      setLibraryAppending(false);
      setLibraryError(false);
      setLibraryErrorCode("");
      setMuseManualRefreshKind((current) =>
        current === "library" ? "" : current,
      );
      return;
    }
    const controller = new AbortController();
    setLibraryLoading(true);
    setLibraryAppending(false);
    setLibraryContinuation("");
    setLibraryError(false);
    setLibraryErrorCode("");
    void fetchListenLibrary(
      props.httpBaseURL,
      controller.signal,
      onlineBrowseSource,
      onlineBrowseDetail
        ? {
            browseId: onlineBrowseDetail.browseId,
            params: onlineBrowseDetail.params,
            language: listenLanguage,
          }
        : { language: listenLanguage },
    )
      .then((payload) => {
        if (!controller.signal.aborted) {
          libraryPageCacheRef.current.set(cacheKey, {
            playlists: payload.playlists,
            artists: payload.artists,
            shelves: payload.shelves,
            continuation: payload.continuation,
            reloadToken: libraryReloadToken,
          });
          setLibraryPlaylists(payload.playlists);
          setLibraryArtists(payload.artists);
          setHomeShelves(payload.shelves);
          setLibraryContinuation(payload.continuation);
        }
      })
      .catch((error) => {
        if (!controller.signal.aborted) {
          libraryPageCacheRef.current.delete(cacheKey);
          setLibraryPlaylists([]);
          setLibraryArtists([]);
          setHomeShelves([]);
          setLibraryContinuation("");
          setLibraryError(true);
          setLibraryErrorCode(getListenErrorCode(error));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLibraryLoading(false);
          setMuseManualRefreshKind((current) =>
            current === "library" ? "" : current,
          );
        }
      });
    return () => controller.abort();
  }, [libraryReloadToken, listenLanguage, mode, museAccountConnected, museAccountReloadToken, onlineBrowseDetail, onlineBrowseSource, props.httpBaseURL]);

  const artistBrowseId = artistBrowsePage?.id ?? "";
  const artistBrowseName = artistBrowsePage?.name ?? "";
  React.useEffect(() => {
    if (mode !== "muse" || (!artistBrowseId && !artistBrowseName)) {
      setMuseManualRefreshKind((current) =>
        current === "artist" ? "" : current,
      );
      return;
    }
    const controller = new AbortController();
    setArtistBrowsePage((current) =>
      current &&
      current.id === artistBrowseId &&
      current.name === artistBrowseName
        ? { ...current, loading: true, appending: false, error: false }
        : current,
    );
    void fetchListenArtist(
      props.httpBaseURL,
      { id: artistBrowseId, name: artistBrowseName },
      controller.signal,
      { language: listenLanguage },
    )
      .then((payload) => {
        if (!controller.signal.aborted) {
          setArtistBrowsePage((current) =>
            current &&
            current.id === artistBrowseId &&
            current.name === artistBrowseName
              ? {
                  ...current,
                  id: payload.id || current.id,
                  title: payload.title || current.title,
                  subtitle: payload.subtitle,
                  thumbnailUrl: payload.thumbnailUrl || current.thumbnailUrl,
                  channelId: payload.channelId,
                  isSubscribed: payload.isSubscribed,
                  mixPlaylistId: payload.mixPlaylistId,
                  mixVideoId: payload.mixVideoId,
                  items: payload.items,
                  shelves: payload.shelves,
                  continuation: payload.continuation,
                  loading: false,
                  appending: false,
                  error: false,
                }
              : current,
          );
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setArtistBrowsePage((current) =>
            current &&
            current.id === artistBrowseId &&
            current.name === artistBrowseName
              ? {
                  ...current,
                  items: [],
                  shelves: [],
                  continuation: "",
                  loading: false,
                  appending: false,
                  error: true,
                }
              : current,
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setMuseManualRefreshKind((current) =>
            current === "artist" ? "" : current,
          );
        }
      });
    return () => controller.abort();
  }, [artistBrowseId, artistBrowseName, listenLanguage, mode, museAccountReloadToken, props.httpBaseURL]);

  const requestOnlineAutoplay = React.useCallback((options: {
    startSeconds?: number;
    forceReload?: boolean;
    restoredLoad?: boolean;
    videoId?: string;
  } = {}) => {
    const startSeconds = Math.max(0, options.startSeconds ?? 0);
    const videoId = options.videoId?.trim() ?? "";
    if (videoId && startSeconds <= 0.5) {
      setOnlineProgressByVideoId((current) =>
        updateListenProgressMap(current, videoId, 0),
      );
      setLiveProgress((current) =>
        current.videoId === videoId
          ? { ...current, currentTime: 0, bufferedTime: 0 }
          : current,
      );
    }
    setPlaybackSessionStarted(true);
    setLivePlaying(false);
    setLiveState("loading");
    setOnlinePlayerCommand({
      id: Date.now(),
      command: "play",
      startSeconds,
      forceReload: options.forceReload === true,
    });
  }, []);

  const changeOnlineBrowseSource = React.useCallback(
    (source: ListenOnlineBrowseSource) => {
      setOnlineBrowseSource(source);
      setOnlineBrowseDetail(null);
      setBrowsePlaylistId("");
      setArtistBrowsePage(null);
    },
    [],
  );

  const openOnlineBrowseCategory = React.useCallback(
    (item: ListenCategoryItem) => {
      const browseId = item.browseId.trim();
      if (!browseId) {
        return;
      }
      setMode("muse");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setArtistBrowsePage(null);
      setQuery("");
      setOnlineBrowseDetail({
        id: item.id,
        source: onlineBrowseSource,
        browseId,
        params: item.params.trim(),
        title: item.title.trim() || browseId,
      });
    },
    [onlineBrowseSource],
  );

  const closeOnlineBrowseDetail = React.useCallback(() => {
    setOnlineBrowseDetail(null);
  }, []);

  const loadMoreLibrary = React.useCallback(() => {
    const continuation = libraryContinuation.trim();
    if (!continuation || libraryAppending || libraryLoading) {
      return;
    }
    const cacheKey = resolveListenLibraryPageCacheKey(
      onlineBrowseSource,
      onlineBrowseDetail,
      listenLanguage,
    );
    const cachedPage = libraryPageCacheRef.current.get(cacheKey);
    const basePage: ListenLibraryPageCacheEntry =
      cachedPage && cachedPage.reloadToken === libraryReloadToken
        ? cachedPage
        : {
            playlists: libraryPlaylists,
            artists: libraryArtists,
            shelves: homeShelves,
            continuation,
            reloadToken: libraryReloadToken,
          };
    const controller = new AbortController();
    setLibraryAppending(true);
    void fetchListenLibrary(
      props.httpBaseURL,
      controller.signal,
      onlineBrowseSource,
      {
        browseId: onlineBrowseDetail?.browseId,
        params: onlineBrowseDetail?.params,
        continuation,
        language: listenLanguage,
      },
    )
      .then((payload) => {
        if (controller.signal.aborted) {
          return;
        }
        const nextPage: ListenLibraryPageCacheEntry = {
          playlists: basePage.playlists,
          artists: basePage.artists,
          shelves: dedupeLibraryShelves([
            ...basePage.shelves,
            ...payload.shelves,
          ]),
          continuation: payload.continuation,
          reloadToken: libraryReloadToken,
        };
        libraryPageCacheRef.current.set(cacheKey, nextPage);
        if (activeLibraryPageCacheKeyRef.current === cacheKey) {
          setLibraryPlaylists(nextPage.playlists);
          setLibraryArtists(nextPage.artists);
          setHomeShelves(nextPage.shelves);
          setLibraryContinuation(nextPage.continuation);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          const currentPage = libraryPageCacheRef.current.get(cacheKey);
          if (currentPage) {
            libraryPageCacheRef.current.set(cacheKey, {
              ...currentPage,
              continuation: "",
            });
          }
          if (activeLibraryPageCacheKeyRef.current === cacheKey) {
            setLibraryContinuation("");
          }
        }
      })
      .finally(() => {
        if (
          !controller.signal.aborted &&
          activeLibraryPageCacheKeyRef.current === cacheKey
        ) {
          setLibraryAppending(false);
        }
      });
  }, [
    homeShelves,
    listenLanguage,
    libraryArtists,
    libraryAppending,
    libraryContinuation,
    libraryLoading,
    libraryPlaylists,
    libraryReloadToken,
    onlineBrowseDetail?.browseId,
    onlineBrowseDetail?.params,
    onlineBrowseSource,
    props.httpBaseURL,
  ]);

  const reloadLiveCatalog = React.useCallback(() => {
    setLiveCatalogReloadToken((current) => current + 1);
    setLiveUserCatalogReloadToken((current) => current + 1);
  }, []);
  React.useEffect(() => {
    return registerTopic(REALTIME_TOPICS.listen.liveCatalog, (event) => {
      if (event.type && event.type !== "catalog-updated" && event.type !== "resync-required") {
        return;
      }
      const key =
        resolveListenLiveCatalogEventKey(event.payload) ||
        String(event.seq ?? event.ts ?? Date.now());
      if (key === liveCatalogRealtimeKeyRef.current) {
        return;
      }
      liveCatalogRealtimeKeyRef.current = key;
      reloadLiveCatalog();
    });
  }, [reloadLiveCatalog]);

  React.useEffect(() => {
    const controller = new AbortController();
    setLiveCatalogLoading(true);
    setLiveCatalogError(false);
    setLiveCatalogMessage("");
    void fetchListenLiveCatalog(props.httpBaseURL, controller.signal)
      .then((catalog) => {
        if (controller.signal.aborted) {
          return;
        }
        setLiveGroups(catalog.groups);
        setSelectedLiveGroupId((current) => {
          if (current && catalog.groups.some((group) => group.id === current)) {
            return current;
          }
          return catalog.groups[0]?.id ?? "";
        });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setLiveGroups([]);
        setSelectedLiveGroupId("");
        setLiveSelectionArmed(false);
        setLiveCatalogError(true);
        setLiveCatalogMessage(getListenErrorMessage(error));
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLiveCatalogLoading(false);
        }
      });
    return () => controller.abort();
  }, [liveCatalogReloadToken, props.httpBaseURL]);

  React.useEffect(() => {
    const controller = new AbortController();
    setLiveUserCatalogLoading(true);
    setLiveUserCatalogError("");
    void fetchListenLiveUserCatalog(props.httpBaseURL, controller.signal)
      .then((catalog) => {
        if (!controller.signal.aborted) {
          setLiveUserCatalog(catalog);
        }
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setLiveUserCatalog({ columns: [], channels: [] });
        setLiveUserCatalogError(getListenErrorMessage(error));
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLiveUserCatalogLoading(false);
        }
      });
    return () => controller.abort();
  }, [liveUserCatalogReloadToken, props.httpBaseURL]);

  const saveLiveUserCatalog = React.useCallback(
    async (catalog: ListenLiveUserCatalog) => {
      setLiveUserCatalogSaving(true);
      setLiveUserCatalogError("");
      try {
        await saveListenLiveUserCatalog(props.httpBaseURL, catalog);
        setLiveUserCatalog(catalog);
        setLiveCatalogReloadToken((current) => current + 1);
      } catch (error) {
        setLiveUserCatalogError(getListenErrorMessage(error));
        throw error;
      } finally {
        setLiveUserCatalogSaving(false);
      }
    },
    [props.httpBaseURL],
  );

  const liveQueue = React.useMemo(
    () => liveGroups.flatMap((group) => group.items),
    [liveGroups],
  );
  const liveStatusVideoIds = React.useMemo(
    () =>
      Array.from(
        new Set(
          liveQueue
            .map((item) => item.videoId.trim())
            .filter(Boolean),
        ),
      ),
    [liveQueue],
  );
  const liveStatusVideoIdKey = liveStatusVideoIds.join(",");
  const activeLiveGroup =
    liveGroups.find((group) => group.id === selectedLiveGroupId) ??
    liveGroups[0] ??
    null;
  const activateLiveSelection = React.useCallback(
    (itemOrId: ListenOnlineItem | string) => {
      const nextId = resolveListenLiveSelectionId(
        liveQueue,
        typeof itemOrId === "string" ? itemOrId : itemOrId.id,
      );
      if (!nextId) {
        return;
      }
      setSelectedLiveId(nextId);
      setLiveSelectionArmed(true);
      setPlaybackMode("hush");
      requestOnlineAutoplay();
    },
    [liveQueue, requestOnlineAutoplay],
  );
  const curatedLiveItems = React.useMemo(
    () => {
      const sourceItems = normalizedQuery
        ? liveQueue
        : activeLiveGroup?.items ?? [];
      return sourceItems.filter((item) =>
        matchesQuery(normalizedQuery, [
          item.title,
          item.channel,
          item.description,
          item.durationLabel,
          item.playCountLabel ?? "",
        ]),
      );
    },
    [activeLiveGroup?.items, liveQueue, normalizedQuery],
  );
  React.useEffect(() => {
    if (!props.active || liveSelectionArmed || mode !== "hush") {
      return;
    }
    const nextId = resolveListenLiveSelectionId(liveQueue, selectedLiveId);
    if (!nextId) {
      return;
    }
    setSelectedLiveId(nextId);
    setLiveSelectionArmed(true);
  }, [liveQueue, liveSelectionArmed, mode, props.active, selectedLiveId]);

  React.useEffect(() => {
    if (
      !props.active ||
      mode !== "hush" ||
      liveStatusVideoIds.length === 0 ||
      !props.httpBaseURL.trim()
    ) {
      return;
    }
    let disposed = false;
    const controllers = new Set<AbortController>();
    const liveStatusVideoIdSet = new Set(liveStatusVideoIds);
    const refreshStatuses = () => {
      const controller = new AbortController();
      controllers.add(controller);
      void fetchListenLiveStatuses(
        props.httpBaseURL,
        liveStatusVideoIds,
        controller.signal,
      )
        .then((statuses) => {
          if (disposed || controller.signal.aborted) {
            return;
          }
          setLiveStatusByVideoId((current) => {
            const next: Record<string, ListenLiveStatus> = {};
            Object.entries(current).forEach(([videoId, status]) => {
              if (liveStatusVideoIdSet.has(videoId)) {
                next[videoId] = status;
              }
            });
            Object.entries(statuses).forEach(([videoId, status]) => {
              if (liveStatusVideoIdSet.has(videoId)) {
                next[videoId] = status;
              }
            });
            return next;
          });
        })
        .catch(() => {
          // Availability checks run on the backend timer; failed reads should not surface as channel state.
        })
        .finally(() => {
          controllers.delete(controller);
        });
    };
    refreshStatuses();
    const warmPoll = window.setTimeout(refreshStatuses, LISTEN_LIVE_STATUS_WARM_POLL_MS);
    const poll = window.setInterval(refreshStatuses, LISTEN_LIVE_STATUS_POLL_MS);
    return () => {
      disposed = true;
      window.clearTimeout(warmPoll);
      window.clearInterval(poll);
      controllers.forEach((controller) => controller.abort());
    };
  }, [
    liveStatusVideoIdKey,
    liveStatusVideoIds,
    mode,
    props.active,
    props.httpBaseURL,
  ]);

  React.useEffect(() => {
    if (mode === "hush" && liveStatusVideoIds.length > 0) {
      return;
    }
    setLiveStatusByVideoId((current) => {
      if (Object.keys(current).length === 0) {
        return current;
      }
      return {};
    });
  }, [liveStatusVideoIds.length, mode]);
  const homeRecommendations = React.useMemo(
    () =>
      dedupeOnlineItems(
        homeShelves.flatMap((shelf) =>
          shelf.kind === "tracks" ? shelf.tracks : [],
        ),
      ),
    [homeShelves],
  );
  const firstHomeTrackShelf = React.useMemo(
    () =>
      homeShelves.find(
        (shelf) => shelf.kind === "tracks" && shelf.tracks.length > 0,
      ) ?? null,
    [homeShelves],
  );
  const homeShelfPlaylists = React.useMemo(
    () =>
      dedupePlaylistItems(
        homeShelves.flatMap((shelf) =>
          shelf.kind === "playlists" ? shelf.playlists : [],
        ),
      ),
    [homeShelves],
  );
  const homeShelfCategories = React.useMemo(
    () =>
      homeShelves.flatMap((shelf) =>
        shelf.kind === "categories" ? shelf.categories : [],
      ),
    [homeShelves],
  );
  const artistBrowsePlaylists = React.useMemo(
    () =>
      dedupePlaylistItems(
        (artistBrowsePage?.shelves ?? []).flatMap((shelf) =>
          shelf.kind === "playlists" ? shelf.playlists : [],
        ),
      ),
    [artistBrowsePage?.shelves],
  );
  const allOnlinePlaylists = React.useMemo(
    () =>
      dedupePlaylistItems([
        ...libraryPlaylists,
        ...homeShelfPlaylists,
        ...artistBrowsePlaylists,
        ...searchPlaylists,
      ]),
    [
      artistBrowsePlaylists,
      homeShelfPlaylists,
      libraryPlaylists,
      searchPlaylists,
    ],
  );
  const savedPlaylistIds = React.useMemo(
    () => new Set(libraryPlaylists.map((item) => item.playlistId)),
    [libraryPlaylists],
  );
  const displayedLibraryPlaylists = React.useMemo(
    () =>
      normalizedQuery
        ? dedupePlaylistItems(
            libraryPlaylists.filter((item) =>
              matchesQuery(normalizedQuery, [
                item.title,
                item.channel,
                item.description,
              ]),
            ),
          )
        : libraryPlaylists,
    [libraryPlaylists, normalizedQuery],
  );
  const showLibraryPlaylistGroup =
    mode === "muse" &&
    !onlineBrowseDetail &&
    (normalizedQuery.length > 0 || onlineBrowseSource === "home");
  const filteredLocalTracks = React.useMemo(
    () =>
      localTracks.filter((item) =>
        matchesQuery(normalizedQuery, [
          item.title,
          item.author,
          item.durationLabel,
        ]),
      ),
    [localTracks, normalizedQuery],
  );
  const artistBrowseTracks = React.useMemo(
    () =>
      dedupeOnlineItems([
        ...(artistBrowsePage?.items ?? []),
        ...((artistBrowsePage?.shelves ?? []).flatMap((shelf) =>
          shelf.kind === "tracks" ? shelf.tracks : [],
        )),
      ]),
    [artistBrowsePage?.items, artistBrowsePage?.shelves],
  );
  const onlineFavoriteSeedVideoIds = React.useMemo(
    () =>
      collectFavoriteSeedVideoIds(
        [
          onlinePlaybackQueue,
          homeRecommendations,
          searchItems,
          playlistTracks,
          artistBrowseTracks,
        ],
        onlineFavoriteByVideoId,
      ),
    [
      artistBrowseTracks,
      homeRecommendations,
      onlineFavoriteByVideoId,
      onlinePlaybackQueue,
      playlistTracks,
      searchItems,
    ],
  );
  const onlineFavoriteSeedKey = onlineFavoriteSeedVideoIds.join(",");

  const selectedLive = liveSelectionArmed
    ? (liveQueue.find((item) => item.id === selectedLiveId) ?? null)
    : null;
  const selectedOnline =
    musePlayback.currentItem ??
    onlinePlaybackQueue.find((item) => item.id === selectedOnlineId) ??
    onlinePlaybackQueue[0] ??
    null;
  const selectedPlaylist =
    allOnlinePlaylists.find((item) => item.playlistId === browsePlaylistId) ??
    null;
  const selectedLocal =
    selectedLocalId
      ? (localTracks.find((item) => item.id === selectedLocalId) ?? null)
      : null;
  const activeOnline =
    playbackMode === "hush"
      ? selectedLive
      : playbackMode === "muse"
        ? selectedOnline
        : null;
  const activeLocal = playbackMode === "linger" ? selectedLocal : null;
  const selectedLocalResumeTime = activeLocal?.path
    ? (localProgressByPath[activeLocal.path] ?? 0)
    : 0;
  const activeOnlineResumeTime =
    activeOnline && activeOnline.group !== "live"
      ? (onlineProgressByVideoId[activeOnline.videoId] ?? 0)
      : 0;
  const activeOnlineFavorite =
    activeOnline && activeOnline.group !== "live"
      ? onlineFavoriteByVideoId[activeOnline.videoId] === true
      : false;
  const activeOnlineFavoriteBusy =
    activeOnline && activeOnline.group !== "live"
      ? favoriteLoadingVideoId === activeOnline.videoId ||
        favoriteMutationVideoId === activeOnline.videoId
      : false;

  React.useEffect(() => {
    if (playbackMode !== "muse" || !musePlayback.hydrated) {
      return;
    }
    const nextVolume = clampVolume(musePlayback.volume);
    setVolume(nextVolume);
    setMuted(musePlayback.muted || nextVolume <= 0);
    if (musePlayback.volumeBeforeMute > 0) {
      lastNonZeroVolumeRef.current = musePlayback.volumeBeforeMute;
    }
  }, [
    musePlayback.hydrated,
    musePlayback.muted,
    musePlayback.volume,
    musePlayback.volumeBeforeMute,
    playbackMode,
  ]);

  React.useEffect(() => {
    if (!musePlayback.hydrated) {
      return;
    }
    void callListenPlaybackSetVolume(volume, muted).catch(() => {});
  }, [muted, musePlayback.hydrated, volume]);

  React.useEffect(() => {
    const progress = musePlayback.progress;
    if (!progress.videoId) {
      return;
    }
    const resumeSeconds =
      progress.duration > 0 && progress.duration - progress.currentTime <= 1.5
        ? 0
        : progress.currentTime;
    setOnlineProgressByVideoId((current) =>
      updateListenProgressMap(current, progress.videoId, resumeSeconds),
    );
  }, [
    musePlayback.progress.currentTime,
    musePlayback.progress.duration,
    musePlayback.progress.videoId,
  ]);

  const activeModeTitle =
    playbackMode === "hush"
      ? props.text.listen.hush
      : playbackMode === "muse"
        ? props.text.listen.muse
        : props.text.listen.linger;
  const externalPlayRequested =
    playbackMode === "linger"
      ? props.controlCommand?.command === "play" &&
        props.controlCommand.id === handledExternalCommandRef.current
      : props.controlCommand?.command === "play" &&
        props.controlCommand.id === handledExternalCommandRef.current;
  const listenNowPlayingStatus = React.useMemo<ListenNowPlayingStatus>(() => {
    const onlineHasPlayableBuffer =
      onlinePlaying &&
      (playbackMode === "hush" ||
        onlineProgress.currentTime > 0.15 ||
        onlineProgress.bufferedTime > 0.15);
    const onlineLoading =
      playbackMode !== "linger" &&
      (onlineState === "loading" ||
        (onlineState === "buffering" && !onlineHasPlayableBuffer));
    const localLoading =
      playbackMode === "linger" && externalPlayRequested && !localPlaying;
    const hasVisibleSession =
      playbackSessionStarted ||
      localPlaying ||
      onlinePlaying ||
      onlineLoading ||
      localLoading ||
      onlineState === "error" ||
      localProgress.currentTime > 0.5 ||
      onlineProgress.currentTime > 0.5;
    if (!hasVisibleSession) {
      return {
        state: "idle",
        title: "",
        subtitle: "",
        artworkURL: "",
        mode: playbackMode,
        canControl: false,
        progress: { currentTime: 0, duration: 0, bufferedTime: 0 },
      };
    }
    if (playbackMode === "linger") {
      return {
        state: localLoading ? "loading" : localPlaying ? "playing" : "paused",
        title: activeLocal?.title ?? props.text.listen.linger,
        subtitle: activeLocal?.author ?? "",
        artworkURL: activeLocal?.coverURL ?? "",
        mode: playbackMode,
        canControl: Boolean(activeLocal),
        progress: localProgress,
      };
    }
    const onlineArtworkURL = activeOnline
      ? buildListenImageCacheURL(
          props.httpBaseURL,
          buildListenHighQualityThumbnailURL(activeOnline.thumbnailUrl ?? ""),
        ) ||
        buildListenImageCacheURL(
          props.httpBaseURL,
          buildYouTubePosterURL(activeOnline.videoId),
        )
      : "";
    return {
      state:
        onlineState === "error"
          ? "error"
          : onlineLoading
            ? "loading"
            : onlinePlaying ||
                onlineState === "playing" ||
                onlineState === "buffering"
              ? "playing"
              : "paused",
      title: activeOnline?.title ?? activeModeTitle,
      subtitle: activeOnline?.channel ?? activeModeTitle,
      artworkURL: onlineArtworkURL,
      mode: playbackMode,
      canControl: Boolean(activeOnline),
      progress: onlineProgress,
    };
  }, [
    activeModeTitle,
    activeOnline,
    externalPlayRequested,
    activeLocal,
    localPlaying,
    localProgress.bufferedTime,
    localProgress.currentTime,
    localProgress.duration,
    onlinePlayerCommand?.command,
    onlinePlaying,
    onlineProgress.bufferedTime,
    onlineProgress.currentTime,
    onlineProgress.duration,
    onlineState,
    playbackSessionStarted,
    playbackMode,
    props.text.listen.linger,
    props.httpBaseURL,
  ]);

  React.useEffect(() => {
    props.onNowPlayingChange?.(listenNowPlayingStatus);
  }, [listenNowPlayingStatus, props.onNowPlayingChange]);

  React.useEffect(() => {
    if (playbackMode !== "hush" || !activeOnline) {
      setLiveProgress({
        videoId: "",
        currentTime: 0,
        duration: 0,
        bufferedTime: 0,
      });
      return;
    }
    setLiveProgress((current) => {
      if (current.videoId === activeOnline.videoId) {
        return current;
      }
      return {
        videoId: activeOnline.videoId,
        currentTime: activeOnlineResumeTime,
        duration: 0,
        bufferedTime: 0,
      };
    });
  }, [activeOnline?.videoId, activeOnlineResumeTime, playbackMode]);

  React.useEffect(() => {
    if (playbackMode !== "hush" || !activeOnline) {
      setLiveState("idle");
      return;
    }
    if (!onlinePlaybackArmed) {
      setLiveState("idle");
      return;
    }
    setLiveState((current) => {
      if (onlinePlayerCommand?.command === "play") {
        return "loading";
      }
      if (
        onlinePlayerCommand?.command === "replay" ||
        onlinePlayerCommand?.command === "resume" ||
        onlinePlayerCommand?.command === "seek"
      ) {
        return "buffering";
      }
      if (onlinePlayerCommand?.command === "pause") {
        return "paused";
      }
      return current === "idle" || current === "ended" ? "paused" : current;
    });
  }, [
    activeOnline?.videoId,
    onlinePlaybackArmed,
    onlinePlayerCommand?.command,
    playbackMode,
  ]);

  React.useEffect(() => {
    if (mode !== "muse" || !onlineFavoriteSeedKey) {
      return;
    }
    const controller = new AbortController();
    void fetchListenTrackFavoriteStatuses(
      props.httpBaseURL,
      onlineFavoriteSeedVideoIds,
      controller.signal,
      listenLanguage,
    )
      .then((statuses) => {
        if (controller.signal.aborted || Object.keys(statuses).length === 0) {
          return;
        }
        setOnlineFavoriteByVideoId((current) => {
          const next = { ...current };
          for (const [videoId, liked] of Object.entries(statuses)) {
            if (shouldIgnoreObservedFavorite(videoId, liked)) {
              continue;
            }
            next[videoId] = liked;
          }
          return next;
        });
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [
    mode,
    listenLanguage,
    museAccountReloadToken,
    onlineFavoriteSeedKey,
    onlineFavoriteSeedVideoIds,
    props.httpBaseURL,
    shouldIgnoreObservedFavorite,
  ]);

  React.useEffect(() => {
    if (playbackMode !== "muse" || !activeOnline || activeOnline.group === "live") {
      setFavoriteLoadingVideoId("");
      return;
    }
    if (onlineFavoriteByVideoId[activeOnline.videoId] !== undefined) {
      return;
    }
    const controller = new AbortController();
    setFavoriteLoadingVideoId(activeOnline.videoId);
    void fetchListenTrackFavorite(
      props.httpBaseURL,
      activeOnline.videoId,
      controller.signal,
      listenLanguage,
    )
      .then((status) => {
        if (!controller.signal.aborted && status.known) {
          if (shouldIgnoreObservedFavorite(activeOnline.videoId, status.liked)) {
            return;
          }
          setOnlineFavoriteByVideoId((current) => ({
            ...current,
            [activeOnline.videoId]: status.liked,
          }));
        }
      })
      .catch(() => undefined)
      .finally(() => {
        if (!controller.signal.aborted) {
          setFavoriteLoadingVideoId((current) =>
            current === activeOnline.videoId ? "" : current,
          );
        }
      });
    return () => controller.abort();
  }, [activeOnline, listenLanguage, museAccountReloadToken, playbackMode, onlineFavoriteByVideoId, props.httpBaseURL, shouldIgnoreObservedFavorite]);

  React.useEffect(() => {
    if (mode !== "muse" && browsePlaylistId === "") {
      setMuseManualRefreshKind((current) =>
        current === "playlist" ? "" : current,
      );
      return;
    }
    if (browsePlaylistId === "") {
      setMuseManualRefreshKind((current) =>
        current === "playlist" ? "" : current,
      );
      setPlaylistTracks([]);
      setPlaylistContinuation("");
      setPlaylistDetailAuthor("");
      setPlaylistDetailTitle("");
      setPlaylistLoading(false);
      setPlaylistAppending(false);
      return;
    }
    const controller = new AbortController();
    setPlaylistLoading(true);
    setPlaylistContinuation("");
    void fetchListenPlaylistPage(
      props.httpBaseURL,
      browsePlaylistId,
      controller.signal,
      "",
      listenLanguage,
    )
      .then((payload) => {
        if (!controller.signal.aborted) {
          setPlaylistTracks(payload.items);
          setPlaylistContinuation(payload.continuation);
          setPlaylistDetailAuthor(payload.author);
          setPlaylistDetailTitle(payload.title);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setPlaylistTracks([]);
          setPlaylistContinuation("");
          setPlaylistDetailAuthor("");
          setPlaylistDetailTitle("");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setPlaylistLoading(false);
          setMuseManualRefreshKind((current) =>
            current === "playlist" ? "" : current,
          );
        }
      });
    return () => controller.abort();
  }, [browsePlaylistId, listenLanguage, mode, museAccountReloadToken, props.httpBaseURL]);

  const currentQueue =
    playbackMode === "hush"
      ? liveQueue
      : playbackMode === "muse"
        ? onlinePlaybackQueue
        : localTracks;
  const currentIndex =
    playbackMode === "hush"
      ? resolveQueueIndex(liveQueue, liveSelectionArmed ? selectedLiveId : "")
      : playbackMode === "muse"
        ? resolveQueueIndex(onlinePlaybackQueue, selectedOnlineId)
        : resolveQueueIndex(localTracks, selectedLocalId);
  const clearForwardSkipNavigationStack = React.useCallback(() => {
    forwardSkipIndexStackRef.current = [];
  }, []);

  React.useEffect(() => {
    clearForwardSkipNavigationStack();
  }, [clearForwardSkipNavigationStack, playbackMode]);

  const runOnlinePlaybackCommand = React.useCallback(
    (
      operation: () => Promise<ListenPlaybackSnapshot>,
      options: {
        clearForwardStack?: boolean;
        loading?: boolean;
        syncVolume?: boolean;
      } = {},
    ) => {
      if (options.clearForwardStack !== false) {
        clearForwardSkipNavigationStack();
      }
      const epoch = onlinePlaybackActionEpochRef.current + 1;
      onlinePlaybackActionEpochRef.current = epoch;
      onlinePlaybackActionPendingRef.current = true;
      setOnlinePlayerCommand(null);
      setPlaybackMode("muse");
      if (options.loading) {
        setPlaybackSessionStarted(true);
      }
      let request: Promise<ListenPlaybackSnapshot>;
      try {
        const shouldSyncVolume = options.syncVolume ?? options.loading === true;
        request = (async () => {
          if (shouldSyncVolume) {
            await callListenPlaybackSetVolume(volume, muted).catch(() => undefined);
          }
          return operation();
        })();
      } catch {
        if (onlinePlaybackActionEpochRef.current === epoch) {
          onlinePlaybackActionPendingRef.current = false;
        }
        return;
      }
      void request
        .then((snapshot) => {
          if (onlinePlaybackActionEpochRef.current === epoch) {
            applyOnlinePlaybackSnapshot(snapshot);
          }
        })
        .catch(() => {})
        .finally(() => {
          if (onlinePlaybackActionEpochRef.current === epoch) {
            onlinePlaybackActionPendingRef.current = false;
          }
        });
    },
    [applyOnlinePlaybackSnapshot, clearForwardSkipNavigationStack, muted, volume],
  );

  const selectLocalQueueTrack = React.useCallback(
    (
      item: { id: string },
      options: { forcePlay?: boolean; preserveForwardSkipStack?: boolean } = {
        forcePlay: true,
      },
    ) => {
      if (!item.id) {
        return;
      }
      if (
        options.forcePlay !== false &&
        options.preserveForwardSkipStack !== true
      ) {
        clearForwardSkipNavigationStack();
      }
      setSelectedLocalId(item.id);
      setPlaybackMode("linger");
      if (localPlaying || options.forcePlay) {
        setLocalPlayerCommand({
          id: Date.now(),
          command: "play",
        });
      }
    },
    [clearForwardSkipNavigationStack, localPlaying],
  );

  const replayCurrent = React.useCallback(() => {
    if (playbackMode === "linger") {
      if (!activeLocal) {
        return;
      }
      setLocalPlayerCommand({
        id: Date.now(),
        command: "replay",
      });
      return;
    }
    if (!activeOnline) {
      return;
    }
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(async () => {
        await callListenPlaybackSeek(0);
        return callListenPlaybackResume();
      }, { loading: true });
      return;
    }
    setOnlinePlayerCommand({
      id: Date.now(),
      command: "replay",
    });
  }, [activeLocal, activeOnline, playbackMode, runOnlinePlaybackCommand]);

  const seekCurrentToStart = React.useCallback(() => {
    if (playbackMode === "linger") {
      if (!activeLocal?.path) {
        return;
      }
      setLocalProgress((current) => ({
        currentTime: 0,
        duration: current.duration,
        bufferedTime: 0,
      }));
      setLocalProgressByPath((current) =>
        updateListenProgressMap(current, activeLocal.path, 0),
      );
      setLocalPlayerCommand({
        id: Date.now(),
        command: "seek",
        startSeconds: 0,
      });
      return;
    }
    if (!activeOnline || activeOnline.group === "live") {
      return;
    }
    setOnlineProgressByVideoId((current) =>
      updateListenProgressMap(current, activeOnline.videoId, 0),
    );
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(() => callListenPlaybackSeek(0), {
        clearForwardStack: false,
      });
      return;
    }
    setOnlinePlayerCommand({
      id: Date.now(),
      command: "seek",
      startSeconds: 0,
    });
  }, [activeLocal?.path, activeOnline, playbackMode, runOnlinePlaybackCommand]);

  const playNext = React.useCallback((_options: { forcePlay?: boolean } = {}) => {
    if (currentQueue.length === 0) {
      return;
    }
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(() => callListenPlaybackNext(), {
        clearForwardStack: false,
        syncVolume: true,
      });
      return;
    }
    const action = resolveListenQueueNextAction({
      length: currentQueue.length,
      currentIndex,
      playMode,
      reason: "manual",
    });
    if (action.type === "select") {
      forwardSkipIndexStackRef.current = pushListenForwardSkipIndex(
        forwardSkipIndexStackRef.current,
        currentIndex,
        action.index,
        currentQueue.length,
      );
      if (playbackMode === "hush") {
        const next = liveQueue[action.index];
        if (next) {
          activateLiveSelection(next.id);
        }
        return;
      }
      const next = localTracks[action.index];
      if (next) {
        selectLocalQueueTrack(next, {
          forcePlay: true,
          preserveForwardSkipStack: true,
        });
      }
    } else if (action.type === "replay") {
      replayCurrent();
    }
  }, [
    activateLiveSelection,
    currentIndex,
    currentQueue.length,
    liveQueue,
    localTracks,
    playMode,
    playbackMode,
    replayCurrent,
    runOnlinePlaybackCommand,
    selectLocalQueueTrack,
  ]);

  const playPrevious = React.useCallback(() => {
    if (currentQueue.length === 0) {
      return;
    }
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(() => callListenPlaybackPrevious(), {
        clearForwardStack: false,
        syncVolume: true,
      });
      return;
    }
    const currentTime =
      playbackMode === "linger"
        ? localProgress.currentTime
        : onlineProgress.currentTime;
    const action = resolveListenQueuePreviousAction({
      length: currentQueue.length,
      currentIndex,
      currentTime,
      forwardStack: forwardSkipIndexStackRef.current,
    });
    forwardSkipIndexStackRef.current = action.forwardStack;
    if (action.type === "select") {
      if (playbackMode === "hush") {
        const next = liveQueue[action.index];
        if (next) {
          activateLiveSelection(next.id);
        }
        return;
      }
      const next = localTracks[action.index];
      if (next) {
        selectLocalQueueTrack(next, {
          forcePlay: true,
          preserveForwardSkipStack: true,
        });
      }
      return;
    }
    if (action.type === "seek-start" || action.type === "restart") {
      seekCurrentToStart();
    }
  }, [
    activateLiveSelection,
    currentIndex,
    currentQueue.length,
    liveQueue,
    localTracks,
    localProgress.currentTime,
    onlineProgress.currentTime,
    playbackMode,
    runOnlinePlaybackCommand,
    seekCurrentToStart,
    selectLocalQueueTrack,
  ]);

  const updatePlayMode = React.useCallback(
    (nextMode: ListenPlayMode) => {
      const resolvedMode =
        playbackMode === "hush" && nextMode === "repeat" ? "order" : nextMode;
      if (playbackMode === "muse") {
        void Promise.all([
          callListenPlaybackSetShuffle(resolvedMode === "shuffle"),
          callListenPlaybackSetRepeatMode(listenRepeatModeFromPlayMode(resolvedMode)),
        ]).catch(() => {});
        return;
      }
      setLocalPlayMode(resolvedMode);
    },
    [playbackMode],
  );

  const setPlayModeFromView = React.useCallback(
    (value: React.SetStateAction<ListenPlayMode>) => {
      const nextMode = typeof value === "function" ? value(playMode) : value;
      updatePlayMode(nextMode);
    },
    [playMode, updatePlayMode],
  );

  const togglePlayMode = React.useCallback(() => {
    const nextMode =
      playMode === "order"
        ? "shuffle"
        : playbackMode === "hush"
          ? "order"
          : playMode === "shuffle"
            ? "repeat"
            : "order";
    updatePlayMode(nextMode);
  }, [playMode, playbackMode, updatePlayMode]);

  const handleLocalProgressChange = React.useCallback(
    (currentTime: number, duration: number, bufferedTime: number) => {
      setLocalProgress((current) => {
        if (
          Math.abs(current.currentTime - currentTime) < 0.05 &&
          Math.abs(current.duration - duration) < 0.05 &&
          Math.abs(current.bufferedTime - bufferedTime) < 0.25
        ) {
          return current;
        }
        return { currentTime, duration, bufferedTime };
      });
      if (!activeLocal?.path) {
        return;
      }
      const resumeSeconds =
        duration > 0 && duration - currentTime <= 1.5 ? 0 : currentTime;
      setLocalProgressByPath((current) =>
        updateListenProgressMap(current, activeLocal.path, resumeSeconds),
      );
    },
    [activeLocal?.path],
  );

  const handleOnlineProgressChange = React.useCallback(
    (
      videoId: string,
      currentTime: number,
      duration: number,
      bufferedTime: number,
      transient = false,
    ) => {
      if (!videoId) {
        return;
      }
      if (
        !transient &&
        activeOnline?.videoId &&
        videoId !== activeOnline.videoId
      ) {
        return;
      }
      setLiveProgress((current) => {
        if (
          current.videoId === videoId &&
          Math.abs(current.currentTime - currentTime) < 0.15 &&
          Math.abs(current.duration - duration) < 0.15 &&
          Math.abs(current.bufferedTime - bufferedTime) < 0.35
        ) {
          return current;
        }
        return { videoId, currentTime, duration, bufferedTime };
      });
    },
    [activeOnline?.videoId],
  );

  const enrichOnlineTrackMetadata = React.useCallback(
    (
      videoId: string,
      seed: { title?: string; artist?: string; thumbnailUrl?: string } = {},
    ) => {
      const trimmedVideoId = videoId.trim();
      if (
        !trimmedVideoId ||
        nativeTrackLookupRef.current.has(trimmedVideoId)
      ) {
        return;
      }
      const controller = new AbortController();
      nativeTrackLookupRef.current.set(trimmedVideoId, controller);
      void fetchListenTrackInfo(
        props.httpBaseURL,
        trimmedVideoId,
        controller.signal,
        listenLanguage,
      )
        .then((item) => {
          if (controller.signal.aborted || !item) {
            return;
          }
          const incoming = mergeListenNativeTrackItem(
            {
              ...item,
              id: `ytmusic-native-${trimmedVideoId}`,
            },
            createNativeOnlineItem({
              videoId: trimmedVideoId,
              title: seed.title,
              artist: seed.artist,
              thumbnailUrl: seed.thumbnailUrl,
            }),
          );
          void callListenPlaybackMergeTrackMetadata(incoming).catch(() => {});
        })
        .catch(() => undefined)
        .finally(() => {
          if (nativeTrackLookupRef.current.get(trimmedVideoId) === controller) {
            nativeTrackLookupRef.current.delete(trimmedVideoId);
          }
        });
    },
    [listenLanguage, props.httpBaseURL],
  );

  React.useEffect(() => {
    if (playbackMode !== "muse" || !activeOnline || activeOnline.group === "live") {
      return;
    }
    const needsMetadata =
      !activeOnline.durationLabel.trim() ||
      !activeOnline.thumbnailUrl?.trim() ||
      activeOnline.title.trim() === activeOnline.videoId ||
      !hasTrustedListenOnlineArtist(activeOnline);
    if (!needsMetadata) {
      return;
    }
    enrichOnlineTrackMetadata(activeOnline.videoId, {
      title: activeOnline.title,
      artist: activeOnline.channel,
      thumbnailUrl: activeOnline.thumbnailUrl,
    });
  }, [activeOnline, enrichOnlineTrackMetadata, playbackMode]);

  const handleOnlineNativeTrackChange = React.useCallback(
    (event: ListenNativePlayerEvent) => {
      if (playbackMode !== "muse") {
        return;
      }
      const videoId = String(
        event.observedVideoId || event.videoId || "",
      ).trim();
      if (!videoId) {
        return;
      }
      const observedFavorite = favoriteFromLikeStatus(event.likeStatus);
      if (
        observedFavorite !== null &&
        !shouldIgnoreObservedFavorite(videoId, observedFavorite)
      ) {
        setOnlineFavoriteByVideoId((current) => ({
          ...current,
          [videoId]: observedFavorite,
        }));
      }
    },
    [playbackMode, shouldIgnoreObservedFavorite],
  );

  const restoreNativePlaybackSession = React.useCallback(
    (event: ListenNativePlayerEvent) => {
      const videoId = String(
        event.observedVideoId || event.videoId || "",
      ).trim();
      if (!videoId || !event.state || event.state === "idle") {
        return;
      }

      const currentTime = Number(event.currentTime || 0);
      const duration = Number(event.duration || 0);
      const bufferedTime = Number(event.bufferedTime || 0);
      const observedFavorite = favoriteFromLikeStatus(event.likeStatus);
      if (
        observedFavorite !== null &&
        !shouldIgnoreObservedFavorite(videoId, observedFavorite)
      ) {
        setOnlineFavoriteByVideoId((current) => ({
          ...current,
          [videoId]: observedFavorite,
        }));
      }
      const liveItem = liveQueue.find((item) => item.videoId === videoId);
      if (liveItem) {
        setPlaybackSessionStarted(true);
        setLiveState(event.state);
        setLivePlaying(
          event.state === "playing" || event.state === "buffering",
        );
        handleOnlineProgressChange(
          videoId,
          Number.isFinite(currentTime) ? currentTime : 0,
          Number.isFinite(duration) ? duration : 0,
          Number.isFinite(bufferedTime) ? bufferedTime : 0,
          true,
        );
        setMode("hush");
        setPlaybackMode("hush");
        setSelectedLiveId(liveItem.id);
        setLiveSelectionArmed(true);
        return;
      }
      if (event.source === "listen-youtube-live-player") {
        setPlaybackSessionStarted(true);
        setLiveState(event.state);
        setLivePlaying(
          event.state === "playing" || event.state === "buffering",
        );
        handleOnlineProgressChange(
          videoId,
          Number.isFinite(currentTime) ? currentTime : 0,
          Number.isFinite(duration) ? duration : 0,
          Number.isFinite(bufferedTime) ? bufferedTime : 0,
          true,
        );
        setMode("hush");
        setPlaybackMode("hush");
        setSelectedLiveId(videoId);
        setLiveSelectionArmed(false);
        return;
      }

      setMode("muse");
      void callListenPlaybackObserveNativeEvent(event).catch(() => {});
    },
    [
      handleOnlineProgressChange,
      liveQueue,
      shouldIgnoreObservedFavorite,
    ],
  );

  const restoreNativePlaybackSessionRef = React.useRef(
    restoreNativePlaybackSession,
  );

  React.useEffect(() => {
    restoreNativePlaybackSessionRef.current = restoreNativePlaybackSession;
  }, [restoreNativePlaybackSession]);

  React.useEffect(() => {
    if (nativeStatusRestoreAttemptedRef.current) {
      return;
    }
    nativeStatusRestoreAttemptedRef.current = true;
    let cancelled = false;
    void Promise.allSettled([
      Call.ByName(`${LISTEN_LIVE_PLAYER_SERVICE}.Status`).then((status) =>
        nativeStatusToPlayerEvent(status, "listen-youtube-live-player"),
      ),
      Call.ByName(`${LISTEN_NATIVE_PLAYER_SERVICE}.Status`).then((status) =>
        nativeStatusToPlayerEvent(status, "listen-youtube-music-player"),
      ),
    ]).then((results) => {
      if (cancelled) {
        return;
      }
      const event = results
        .map((result) => result.status === "fulfilled" ? result.value : null)
        .find(Boolean);
      if (event) {
        restoreNativePlaybackSessionRef.current(event);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const toggleOnlineFavorite = React.useCallback(() => {
    if (playbackMode !== "muse" || !activeOnline || activeOnline.group === "live") {
      return;
    }
    const videoId = activeOnline.videoId;
    const previousLiked = onlineFavoriteByVideoId[videoId] === true;
    const nextLiked = !previousLiked;
    const controller = new AbortController();
    rememberFavoriteOverride(videoId, nextLiked);
    setFavoriteMutationVideoId(videoId);
    setOnlineFavoriteByVideoId((current) => ({
      ...current,
      [videoId]: nextLiked,
    }));
    void updateListenTrackFavorite(
      props.httpBaseURL,
      videoId,
      nextLiked,
      controller.signal,
    )
      .then((liked) => {
        if (!controller.signal.aborted) {
          rememberFavoriteOverride(videoId, liked);
          setOnlineFavoriteByVideoId((current) => ({
            ...current,
            [videoId]: liked,
          }));
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          clearFavoriteOverride(videoId);
          setOnlineFavoriteByVideoId((current) => ({
            ...current,
            [videoId]: previousLiked,
          }));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setFavoriteMutationVideoId((current) =>
            current === videoId ? "" : current,
          );
        }
      });
  }, [
    activeOnline,
    clearFavoriteOverride,
    onlineFavoriteByVideoId,
    playbackMode,
    props.httpBaseURL,
    rememberFavoriteOverride,
  ]);

  const openSelectedLocalDirectory = React.useCallback(() => {
    const path = activeLocal?.path?.trim();
    if (!path) {
      return;
    }
    void openLibraryPath.mutateAsync({ path }).catch(() => {});
  }, [activeLocal?.path, openLibraryPath]);

  React.useEffect(() => {
    if (playbackMode === "linger") {
      setLivePlaying(false);
    }
  }, [playbackMode]);

  React.useEffect(() => {
    if (playbackMode !== "linger") {
      setLocalPlaying(false);
    }
  }, [playbackMode]);

  React.useEffect(() => {
    if (
      localPlaying ||
      onlinePlaying ||
      onlineState === "playing" ||
      onlineState === "buffering"
    ) {
      setPlaybackSessionStarted(true);
    }
  }, [localPlaying, onlinePlaying, onlineState]);

  const handlePlaybackEnded = React.useCallback(() => {
    if (playbackMode === "linger") {
      if (activeLocal?.path) {
        setLocalProgressByPath((current) =>
          updateListenProgressMap(current, activeLocal.path, 0),
        );
      }
      setLocalProgress({ currentTime: 0, duration: 0, bufferedTime: 0 });
    } else if (activeOnline?.group === "live") {
      return;
    } else if (activeOnline?.videoId) {
      setOnlineProgressByVideoId((current) =>
        updateListenProgressMap(current, activeOnline.videoId, 0),
      );
      return;
    }
    if (playbackMode === "linger") {
      const action = resolveListenQueueNextAction({
        length: currentQueue.length,
        currentIndex,
        playMode,
        reason: "ended",
      });
      if (action.type === "select") {
        const next = localTracks[action.index];
        if (next) {
          selectLocalQueueTrack(next, { forcePlay: true });
          return;
        }
      } else if (action.type === "replay") {
        replayCurrent();
        return;
      }
      setLocalPlaying(false);
    } else {
      setLivePlaying(false);
      setLiveState("ended");
    }
  }, [
    activeOnline?.group,
    activeOnline?.videoId,
    activeLocal?.path,
    currentIndex,
    currentQueue.length,
    localTracks,
    playbackMode,
    playMode,
    replayCurrent,
    selectLocalQueueTrack,
  ]);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      writeListenStorageState({
        version: 2,
        mode,
        playbackMode,
        listOpen,
        playMode,
        selectedLiveId,
        selectedOnlineId,
        browsePlaylistId,
        selectedLocalId,
        onlineQueueKind: onlineQueueState.kind,
        onlineQueueTitle: onlineQueueState.title,
        onlineQueueSeedVideoId:
          onlineQueueState.kind === "radio" ? onlineQueueState.seedVideoId : "",
        onlineQueuePlaylistId:
          onlineQueueState.kind === "playlist"
            ? onlineQueueState.playlistId
            : "",
        onlineQueueItems: onlineQueueState.items,
        muted,
        volume,
        localProgressByPath,
        onlineProgressByVideoId,
      });
    }, 160);
    return () => window.clearTimeout(timer);
  }, [
    listOpen,
    localProgressByPath,
    mode,
    playbackMode,
    muted,
    onlineQueueState,
    onlineProgressByVideoId,
    playMode,
    browsePlaylistId,
    selectedLiveId,
    selectedLocalId,
    selectedOnlineId,
    volume,
  ]);

  const togglePlayback = React.useCallback(() => {
    setPlaybackSessionStarted(true);
    if (playbackMode === "linger") {
      if (!activeLocal) {
        return;
      }
      setLocalPlayerCommand({
        id: Date.now(),
        command: localPlaying ? "pause" : "play",
      });
      return;
    }
    if (!activeOnline) {
      return;
    }
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(
        () =>
          onlinePlaying
            ? callListenPlaybackPause()
            : onlineState === "paused"
              ? callListenPlaybackResume()
              : callListenPlaybackPlayPause(),
        { loading: !onlinePlaying },
      );
      return;
    }
    const command = onlinePlaying
      ? "pause"
      : onlineState === "paused"
        ? "resume"
        : "play";
    const commandId = Date.now();
    if (!onlinePlaying) {
      setLiveState(command === "resume" ? "buffering" : "loading");
    }
    setOnlinePlayerCommand({
      id: commandId,
      command,
      startSeconds:
        command === "play" && activeOnline.group !== "live"
          ? activeOnlineResumeTime
          : undefined,
    });
  }, [
    activeOnline,
    activeOnlineResumeTime,
    activeLocal,
    localPlaying,
    onlinePlaying,
    onlineState,
    playbackMode,
    runOnlinePlaybackCommand,
  ]);

  React.useEffect(() => {
    const command = props.controlCommand;
    if (!command || handledExternalCommandRef.current === command.id) {
      return;
    }
    handledExternalCommandRef.current = command.id;
    const isPlaying = playbackMode === "linger" ? localPlaying : onlinePlaying;
    if (command.command === "previous") {
      playPrevious();
      return;
    }
    if (command.command === "next") {
      playNext();
      return;
    }
    if (command.command === "toggle") {
      togglePlayback();
      return;
    }
    if (command.command === "play" && !isPlaying) {
      togglePlayback();
      return;
    }
    if (command.command === "pause" && isPlaying) {
      togglePlayback();
    }
  }, [
    localPlaying,
    onlinePlaying,
    playbackMode,
    playNext,
    playPrevious,
    props.controlCommand,
    togglePlayback,
  ]);

  const toggleMute = React.useCallback(() => {
    if (muted || volume <= 0) {
      const restoredVolume =
        lastNonZeroVolumeRef.current > 0 ? lastNonZeroVolumeRef.current : 1;
      setVolume(restoredVolume);
      setMuted(false);
      return;
    }
    lastNonZeroVolumeRef.current = volume;
    setMuted(true);
  }, [muted, volume]);

  const handleVolumeChange = React.useCallback((value: number) => {
    const nextVolume = clampVolume(value);
    setVolume(nextVolume);
    setMuted(nextVolume <= 0);
    if (nextVolume > 0) {
      lastNonZeroVolumeRef.current = nextVolume;
    }
  }, []);

  const undoOnlineQueueEdit = React.useCallback(() => {
    if (!onlineQueueCanUndo) {
      return;
    }
    runOnlinePlaybackCommand(() => callListenPlaybackUndoQueue());
  }, [onlineQueueCanUndo, runOnlinePlaybackCommand]);

  const redoOnlineQueueEdit = React.useCallback(() => {
    if (!onlineQueueCanRedo) {
      return;
    }
    runOnlinePlaybackCommand(() => callListenPlaybackRedoQueue());
  }, [onlineQueueCanRedo, runOnlinePlaybackCommand]);

  const playOnlineItemsQueue = React.useCallback(
    (
      items: ListenOnlineItem[],
      title: string,
      selectedItem?: ListenOnlineItem,
    ) => {
      const queueItems = dedupeOnlineItems(items);
      if (queueItems.length === 0) {
        return;
      }
      const selectedQueueItem =
        (selectedItem
          ? queueItems.find((item) => item.id === selectedItem.id) ??
            queueItems.find((item) => item.videoId === selectedItem.videoId)
          : null) ?? queueItems[0];
      if (!selectedQueueItem) {
        return;
      }
      runOnlinePlaybackCommand(
        () =>
          callListenPlaybackPlayQueue({
            tracks: queueItems,
            startingAt: Math.max(0, queueItems.indexOf(selectedQueueItem)),
            title,
            kind: "radio",
          }),
        { loading: true },
      );
    },
    [runOnlinePlaybackCommand],
  );

  const playOnlineRadioSeed = React.useCallback(
    (item: ListenOnlineItem) => {
      runOnlinePlaybackCommand(
        () =>
          callListenPlaybackPlayQueue({
            tracks: [item],
            startingAt: 0,
            title: props.text.listen.groupRadio,
            kind: "radio",
          }),
        { loading: true },
      );
    },
    [props.text.listen.groupRadio, runOnlinePlaybackCommand],
  );

  const playOnlineShelfTrack = React.useCallback(
    (shelf: ListenLibraryShelf, item: ListenOnlineItem) => {
      const shelfItems = shelf.tracks.some((track) => track.id === item.id)
        ? shelf.tracks
        : [item, ...shelf.tracks];
      playOnlineItemsQueue(
        shelfItems,
        resolveListenShelfQueueTitle(
          shelf,
          props.text,
          props.text.listen.groupRecommendations,
        ),
        item,
      );
    },
    [playOnlineItemsQueue, props.text, props.text.listen.groupRecommendations],
  );

  const playOnlineShelfAll = React.useCallback(
    (shelf: ListenLibraryShelf) => {
      playOnlineItemsQueue(
        shelf.tracks,
        resolveListenShelfQueueTitle(
          shelf,
          props.text,
          props.text.listen.groupRecommendations,
        ),
      );
    },
    [playOnlineItemsQueue, props.text, props.text.listen.groupRecommendations],
  );

  const shuffleOnlineShelf = React.useCallback(
    (shelf: ListenLibraryShelf) => {
      playOnlineItemsQueue(
        shuffleListenOnlineItems(shelf.tracks),
        resolveListenShelfQueueTitle(
          shelf,
          props.text,
          props.text.listen.groupRecommendations,
        ),
      );
    },
    [playOnlineItemsQueue, props.text, props.text.listen.groupRecommendations],
  );

  const playOnlineSearchResults = React.useCallback(() => {
    playOnlineItemsQueue(searchItems, props.text.listen.groupRecommendations);
  }, [playOnlineItemsQueue, props.text.listen.groupRecommendations, searchItems]);

  const playOnlineSearchTrack = React.useCallback(
    (item: ListenOnlineItem) => {
      playOnlineItemsQueue(
        searchItems.some((track) => track.id === item.id)
          ? searchItems
          : [item, ...searchItems],
        props.text.listen.searchSongs,
        item,
      );
    },
    [playOnlineItemsQueue, props.text.listen.searchSongs, searchItems],
  );

  const shuffleOnlineSearchResults = React.useCallback(() => {
    playOnlineItemsQueue(
      shuffleListenOnlineItems(searchItems),
      props.text.listen.groupRecommendations,
    );
  }, [playOnlineItemsQueue, props.text.listen.groupRecommendations, searchItems]);

  const loadMoreSearch = React.useCallback(() => {
    const continuation = searchContinuation.trim();
    if (!continuation || searchAppending || searchLoading || normalizedQuery.length < 2) {
      return;
    }
    const controller = new AbortController();
    setSearchAppending(true);
    void fetchListenSearch(
      props.httpBaseURL,
      query,
      controller.signal,
      listenLanguage,
      continuation,
    )
      .then((payload) => {
        if (controller.signal.aborted) {
          return;
        }
        setSearchItems((current) =>
          dedupeOnlineItems([...current, ...payload.items]),
        );
        setSearchContinuation(payload.continuation);
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setSearchContinuation("");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setSearchAppending(false);
        }
      });
  }, [
    listenLanguage,
    normalizedQuery.length,
    props.httpBaseURL,
    query,
    searchAppending,
    searchContinuation,
    searchLoading,
  ]);

  const selectOnlineQueueTrack = React.useCallback(
    (item: ListenOnlineItem) => {
      let queueIndex = onlinePlaybackQueue.findIndex(
        (queueItem) => queueItem.id === item.id,
      );
      if (queueIndex < 0 && item.videoId) {
        queueIndex = onlinePlaybackQueue.findIndex(
          (queueItem) => queueItem.videoId === item.videoId,
        );
      }
      const queueItems = queueIndex >= 0 ? onlinePlaybackQueue : [item];
      runOnlinePlaybackCommand(
        () =>
          queueItems.length > 1 || queueIndex >= 0
            ? callListenPlaybackPlayQueue({
                tracks: queueItems,
                startingAt: Math.max(0, queueIndex),
                title: onlineQueueState.title,
                kind: onlineQueueState.kind === "radio" ? "radio" : "playlist",
                playlistId:
                  onlineQueueState.kind === "playlist"
                    ? onlineQueueState.playlistId
                    : "",
              })
            : callListenPlaybackPlayTrack(item),
        { loading: true },
      );
    },
    [
      onlinePlaybackQueue,
      onlineQueueState,
      runOnlinePlaybackCommand,
    ],
  );

  const clearOnlineQueue = React.useCallback(() => {
    runOnlinePlaybackCommand(() => callListenPlaybackClearQueue());
  }, [runOnlinePlaybackCommand]);

  const removeOnlineQueueItem = React.useCallback(
    (item: ListenOnlineItem) => {
      if (!item.videoId) {
        return;
      }
      runOnlinePlaybackCommand(() =>
        callListenPlaybackRemoveFromQueue([item]),
      );
    },
    [runOnlinePlaybackCommand],
  );

  const moveOnlineQueueItem = React.useCallback(
    (item: ListenOnlineItem, direction: -1 | 1) => {
      const currentIndex = onlineQueueState.items.findIndex(
        (queueItem) => queueItem.id === item.id,
      );
      if (currentIndex < 0) {
        return;
      }
      const destination = currentIndex + direction + (direction > 0 ? 1 : 0);
      runOnlinePlaybackCommand(() =>
        callListenPlaybackMoveQueueItem(currentIndex, destination),
      );
    },
    [onlineQueueState, runOnlinePlaybackCommand],
  );

  const openPlaylistBrowse = React.useCallback((item: ListenPlaylistItem) => {
    const playlistId = item.playlistId.trim();
    if (!playlistId) {
      return;
    }
    setBrowsePlaylistId(playlistId);
  }, []);

  const openOnlineArtistBrowse = React.useCallback(
    (item: ListenOnlineItem) => {
      const artistName = item.channel.trim();
      if (!artistName) {
        return;
      }
      const artistBrowseId = item.artistBrowseId?.trim() ?? "";
      const artist = item.artists?.find((candidate) => {
        const candidateBrowseId = candidate.browseId?.trim() ?? "";
        return (
          (artistBrowseId && candidateBrowseId === artistBrowseId) ||
          candidate.name.trim() === artistName
        );
      });
      setListOpen(true);
      setMode("muse");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setQuery("");
      setArtistBrowsePage({
        id: artistBrowseId,
        name: artistName,
        title: artistName,
        subtitle: "",
        thumbnailUrl: artist?.thumbnailUrl,
        channelId: "",
        isSubscribed: false,
        mixPlaylistId: "",
        mixVideoId: "",
        items: [],
        shelves: [],
        continuation: "",
        loading: true,
        appending: false,
        error: false,
      });
    },
    [],
  );

  const openSearchArtistBrowse = React.useCallback(
    (item: ListenArtistItem) => {
      const artistName = item.name.trim();
      const artistId = item.browseId.trim();
      if (!artistName && !artistId) {
        return;
      }
      setMode("muse");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setQuery("");
      setArtistBrowsePage({
        id: artistId,
        name: artistName,
        title: artistName || artistId,
        subtitle: item.subtitle,
        thumbnailUrl: item.thumbnailUrl,
        channelId: "",
        isSubscribed: false,
        mixPlaylistId: "",
        mixVideoId: "",
        items: [],
        shelves: [],
        continuation: "",
        loading: true,
        appending: false,
        error: false,
      });
    },
    [],
  );

  const closeArtistBrowse = React.useCallback(() => {
    setBrowsePlaylistId("");
    setArtistBrowsePage(null);
  }, []);

  const playArtistFromIndex = React.useCallback(
    (index: number) => {
      const page = artistBrowsePage;
      const next = page?.items[index];
      if (!page || !next) {
        return;
      }
      runOnlinePlaybackCommand(
        () =>
          callListenPlaybackPlayQueue({
            tracks: page.items,
            startingAt: index,
            title: page.title || page.name,
            kind: "radio",
          }),
        { loading: true },
      );
    },
    [artistBrowsePage, runOnlinePlaybackCommand],
  );

  const shuffleArtist = React.useCallback(() => {
    const page = artistBrowsePage;
    if (!page || page.items.length === 0) {
      return;
    }
    const items = [...page.items].sort(() => Math.random() - 0.5);
    const first = items[0];
    if (!first) {
      return;
    }
    runOnlinePlaybackCommand(
      () =>
        callListenPlaybackPlayQueue({
          tracks: items,
          startingAt: 0,
          title: page.title || page.name,
          kind: "radio",
        }),
      { loading: true },
    );
  }, [artistBrowsePage, runOnlinePlaybackCommand]);

  const loadMoreArtist = React.useCallback(() => {
    const page = artistBrowsePage;
    const continuation = page?.continuation.trim() ?? "";
    if (!page || !continuation || page.appending || page.loading) {
      return;
    }
    const controller = new AbortController();
    setArtistBrowsePage((current) =>
      current && current.id === page.id
        ? { ...current, appending: true }
        : current,
    );
    void fetchListenArtist(
      props.httpBaseURL,
      { id: page.id, name: page.name },
      controller.signal,
      { continuation, language: listenLanguage },
    )
      .then((payload) => {
        if (controller.signal.aborted) {
          return;
        }
        setArtistBrowsePage((current) =>
          current && current.id === page.id
            ? {
                ...current,
                items: dedupeOnlineItems([...current.items, ...payload.items]),
                shelves: dedupeLibraryShelves([
                  ...current.shelves,
                  ...payload.shelves,
                ]),
                continuation: payload.continuation,
                appending: false,
              }
            : current,
        );
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setArtistBrowsePage((current) =>
            current && current.id === page.id
              ? { ...current, continuation: "", appending: false }
              : current,
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setArtistBrowsePage((current) =>
            current && current.id === page.id
              ? { ...current, appending: false }
              : current,
          );
        }
      });
  }, [artistBrowsePage, listenLanguage, props.httpBaseURL]);

  const loadArtistShelfTracks = React.useCallback(
    async (shelf: ListenLibraryShelf) => {
      const page = artistBrowsePage;
      if (!page) {
        return shelf.tracks;
      }
      const controller = new AbortController();
      const fetchedTracks: ListenOnlineItem[] = [];
      const fetchedShelves: ListenLibraryShelf[] = [];
      const seenContinuations = new Set<string>();
      let nextContinuation = shelf.continuation.trim();
      if (shelf.browseId.trim()) {
        const payload = await fetchListenArtist(
          props.httpBaseURL,
          { id: page.id, name: page.name },
          controller.signal,
          {
            browseId: shelf.browseId,
            params: shelf.params,
            language: listenLanguage,
          },
        );
        fetchedTracks.push(...payload.items);
        fetchedShelves.push(...payload.shelves);
        nextContinuation = payload.continuation.trim();
      } else if (!nextContinuation) {
        nextContinuation = page.continuation.trim();
      }

      for (
        let pageIndex = 0;
        nextContinuation && pageIndex < LISTEN_ARTIST_SHELF_CONTINUATION_MAX_PAGES;
        pageIndex += 1
      ) {
        if (seenContinuations.has(nextContinuation)) {
          nextContinuation = "";
          break;
        }
        seenContinuations.add(nextContinuation);
        const payload = await fetchListenArtist(
          props.httpBaseURL,
          { id: page.id, name: page.name },
          controller.signal,
          { continuation: nextContinuation, language: listenLanguage },
        );
        fetchedTracks.push(...payload.items);
        fetchedShelves.push(...payload.shelves);
        nextContinuation = payload.continuation.trim();
      }

      const fetchedTrackItems = dedupeOnlineItems([
        ...fetchedTracks,
        ...fetchedShelves.flatMap((item) =>
          item.kind === "tracks" ? item.tracks : [],
        ),
      ]);
      if (fetchedTrackItems.length === 0) {
        return shelf.tracks;
      }
      const tracks = dedupeOnlineItems([...shelf.tracks, ...fetchedTrackItems]);
      setArtistBrowsePage((current) => {
        if (!current || current.id !== page.id) {
          return current;
        }
        const shelves = current.shelves.map((item) =>
          item.id === shelf.id
            ? {
                ...item,
                tracks,
                continuation: nextContinuation,
              }
            : item,
        );
        return {
          ...current,
          items: dedupeOnlineItems([...current.items, ...tracks]),
          shelves,
          continuation: nextContinuation || current.continuation,
        };
      });
      return tracks;
    },
    [artistBrowsePage, listenLanguage, props.httpBaseURL],
  );

  const playArtistMix = React.useCallback(() => {
    const page = artistBrowsePage;
    const playlistId = page?.mixPlaylistId.trim() ?? "";
    if (!page || !playlistId) {
      return;
    }
    const controller = new AbortController();
    setArtistActionBusy("mix");
    void fetchListenPlaylistQueue(
      props.httpBaseURL,
      playlistId,
      controller.signal,
      listenLanguage,
    )
      .then((items) => {
        const nextItems = items.length > 0 ? items : page.items;
        const first = nextItems[0];
        if (!first) {
          return;
        }
        runOnlinePlaybackCommand(
          () =>
            callListenPlaybackPlayQueue({
              tracks: nextItems,
              startingAt: 0,
              title: page.title || page.name,
              kind: "mix",
              playlistId,
              startVideoId: page.mixVideoId,
            }),
          { loading: true },
        );
      })
      .catch(() => {})
      .finally(() => {
        if (!controller.signal.aborted) {
          setArtistActionBusy("");
        }
      });
  }, [
    artistBrowsePage,
    listenLanguage,
    props.httpBaseURL,
    runOnlinePlaybackCommand,
  ]);

  const toggleArtistSubscription = React.useCallback(() => {
    const page = artistBrowsePage;
    const channelId = page?.channelId.trim() ?? "";
    if (!page || !channelId) {
      return;
    }
    const nextSubscribed = !page.isSubscribed;
    const controller = new AbortController();
    setArtistActionBusy("subscribe");
    setArtistBrowsePage((current) =>
      current && current.id === page.id
        ? { ...current, isSubscribed: nextSubscribed }
        : current,
    );
    void updateListenArtistSubscription(
      props.httpBaseURL,
      channelId,
      nextSubscribed,
      controller.signal,
    )
      .then((subscribed) => {
        if (!controller.signal.aborted) {
          setArtistBrowsePage((current) =>
            current && current.id === page.id
              ? { ...current, isSubscribed: subscribed }
              : current,
          );
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setArtistBrowsePage((current) =>
            current && current.id === page.id
              ? { ...current, isSubscribed: page.isSubscribed }
              : current,
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setArtistActionBusy("");
        }
      });
  }, [artistBrowsePage, props.httpBaseURL]);

  const playPlaylistFromIndex = React.useCallback(
    (index: number) => {
      const playlist = selectedPlaylist;
      const next = playlistTracks[index];
      if (!playlist || !next) {
        return;
      }
      const queueItems = applyListenPlaylistPlaybackFallback(
        playlistTracks,
        resolveListenPlaylistPlaybackFallbackArtist(
          playlist,
          playlistDetailAuthor,
        ),
      );
      const selectedQueueItem =
        queueItems.find((item) => item.id === next.id) ?? next;
      runOnlinePlaybackCommand(
        () =>
          callListenPlaybackPlayQueue({
            tracks: queueItems,
            startingAt: Math.max(0, queueItems.indexOf(selectedQueueItem)),
            title: playlistDetailTitle || playlist.title,
            kind: "playlist",
            playlistId: playlist.playlistId,
          }),
        { loading: true },
      );
    },
    [
      playlistDetailAuthor,
      playlistDetailTitle,
      playlistTracks,
      runOnlinePlaybackCommand,
      selectedPlaylist,
    ],
  );

  const queueSelectedPlaylistTracks = React.useCallback(
    (placement: "next" | "end") => {
      const playlist = selectedPlaylist;
      if (!playlist || playlistTracks.length === 0) {
        return;
      }
      const queueItems = applyListenPlaylistPlaybackFallback(
        playlistTracks,
        resolveListenPlaylistPlaybackFallbackArtist(
          playlist,
          playlistDetailAuthor,
        ),
      );
      const firstItem = queueItems[0];
      if (!firstItem) {
        return;
      }
      const shouldStartQueuedItems =
        playbackMode !== "muse" || onlineQueueState.items.length === 0;
      const existingIDs = new Set(onlineQueueState.items.map((item) => item.id));
      const existingVideoIDs = new Set(
        onlineQueueState.items.map((item) => item.videoId).filter(Boolean),
      );
      const queueItemsToAdd = queueItems.filter(
        (item) =>
          !existingIDs.has(item.id) &&
          (!item.videoId || !existingVideoIDs.has(item.videoId)),
      );
      if (shouldStartQueuedItems) {
        runOnlinePlaybackCommand(
          () =>
            callListenPlaybackPlayQueue({
              tracks: queueItems,
              startingAt: 0,
              title: playlistDetailTitle || playlist.title,
              kind: "playlist",
              playlistId: playlist.playlistId,
            }),
          { loading: true },
        );
      } else if (queueItemsToAdd.length > 0) {
        runOnlinePlaybackCommand(() =>
          placement === "next"
            ? callListenPlaybackInsertNextInQueue(queueItemsToAdd)
            : callListenPlaybackAppendToQueue(queueItemsToAdd),
        );
      }
    },
    [
      onlineQueueState.items,
      playlistDetailAuthor,
      playlistDetailTitle,
      playlistTracks,
      playbackMode,
      runOnlinePlaybackCommand,
      selectedPlaylist,
    ],
  );

  const playPlaylistNext = React.useCallback(() => {
    queueSelectedPlaylistTracks("next");
  }, [queueSelectedPlaylistTracks]);

  const addPlaylistToQueue = React.useCallback(() => {
    queueSelectedPlaylistTracks("end");
  }, [queueSelectedPlaylistTracks]);

  const loadMorePlaylist = React.useCallback(() => {
    const continuation = playlistContinuation.trim();
    const playlistId = browsePlaylistId.trim();
    if (!continuation || playlistAppending || playlistLoading) {
      return;
    }
    const controller = new AbortController();
    setPlaylistAppending(true);
    void fetchListenPlaylistPage(
      props.httpBaseURL,
      playlistId,
      controller.signal,
      continuation,
      listenLanguage,
    )
      .then((payload) => {
        if (controller.signal.aborted) {
          return;
        }
        setPlaylistTracks((current) =>
          dedupeOnlineItems([...current, ...payload.items]),
        );
        setPlaylistContinuation(payload.continuation);
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setPlaylistContinuation("");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setPlaylistAppending(false);
        }
      });
  }, [
    browsePlaylistId,
    listenLanguage,
    playlistAppending,
    playlistContinuation,
    playlistLoading,
    props.httpBaseURL,
  ]);

  const updatePlaylistLibrary = React.useCallback(
    (item: ListenPlaylistItem, action: ListenPlaylistLibraryAction) => {
      const controller = new AbortController();
      setPlaylistMutationPlaylistId(item.playlistId);
      setPlaylistMutationAction(action);
      void updateListenPlaylistLibrary(
        props.httpBaseURL,
        item.playlistId,
        action,
        controller.signal,
      )
        .then(() => {
          if (!controller.signal.aborted) {
            setLibraryReloadToken((current) => current + 1);
          }
        })
        .catch(() => {})
        .finally(() => {
          if (!controller.signal.aborted) {
            setPlaylistMutationPlaylistId("");
            setPlaylistMutationAction(null);
          }
        });
    },
    [props.httpBaseURL],
  );

  const filteredArtistTracks = React.useMemo(
    () =>
      (artistBrowsePage?.items ?? []).filter((item) =>
        matchesQuery(normalizedQuery, [
          item.title,
          item.channel,
          item.description,
          item.durationLabel,
          item.playCountLabel ?? "",
        ]),
      ),
    [artistBrowsePage?.items, normalizedQuery],
  );
  const filteredArtistShelves = React.useMemo(
    () =>
      (artistBrowsePage?.shelves ?? [])
        .map((shelf) => {
          if (shelf.kind === "artists") {
            return {
              ...shelf,
              artists: shelf.artists.filter((item) =>
                matchesQuery(normalizedQuery, [item.name, item.subtitle]),
              ),
            };
          }
          if (shelf.kind === "playlists") {
            return {
              ...shelf,
              playlists: shelf.playlists.filter((item) =>
                matchesQuery(normalizedQuery, [
                  item.title,
                  item.channel,
                  item.description,
                ]),
              ),
            };
          }
          return {
            ...shelf,
            tracks: shelf.tracks.filter((item) =>
              matchesQuery(normalizedQuery, [
                item.title,
                item.channel,
                item.description,
                item.durationLabel,
                item.playCountLabel ?? "",
              ]),
            ),
          };
        })
        .filter((shelf) =>
          shelf.kind === "artists"
            ? shelf.artists.length > 0
            : shelf.kind === "playlists"
            ? shelf.playlists.length > 0
            : shelf.tracks.length > 0,
        ),
    [artistBrowsePage?.shelves, normalizedQuery],
  );

  const selectFirstResult = React.useCallback(() => {
    if (mode === "hush") {
      const first = curatedLiveItems[0];
      if (first) {
        activateLiveSelection(first);
      }
      return;
    }
    if (mode === "muse") {
      if (artistBrowsePage) {
        const firstArtistTrack =
          filteredArtistTracks[0] ?? artistBrowsePage.items[0];
        if (firstArtistTrack) {
          const index = artistBrowsePage.items.findIndex(
            (track) => track.id === firstArtistTrack.id,
          );
          playArtistFromIndex(index >= 0 ? index : 0);
        }
        return;
      }
      const firstTrack = browsePlaylistId
        ? playlistTracks[0]
        : normalizedQuery
          ? searchItems[0]
          : firstHomeTrackShelf?.tracks[0] ?? homeRecommendations[0];
      if (firstTrack) {
        if (browsePlaylistId) {
          playPlaylistFromIndex(0);
        } else if (!normalizedQuery && firstHomeTrackShelf) {
          playOnlineShelfTrack(firstHomeTrackShelf, firstTrack);
        } else if (normalizedQuery) {
          playOnlineSearchTrack(firstTrack);
        } else {
          playOnlineRadioSeed(firstTrack);
        }
        return;
      }
      const firstArtist = normalizedQuery ? searchArtists[0] : null;
      if (firstArtist) {
        openSearchArtistBrowse(firstArtist);
        return;
      }
      const firstPlaylist =
        (normalizedQuery ? searchPlaylists[0] : null) ??
        displayedLibraryPlaylists[0] ??
        homeShelfPlaylists[0];
      if (firstPlaylist) {
        openPlaylistBrowse(firstPlaylist);
        return;
      }
      const firstCategory = !normalizedQuery ? homeShelfCategories[0] : null;
      if (firstCategory) {
        openOnlineBrowseCategory(firstCategory);
      }
      return;
    }
    const first = filteredLocalTracks[0];
    if (first) {
      setSelectedLocalId(first.id);
      setPlaybackMode("linger");
    }
  }, [
    activateLiveSelection,
    artistBrowsePage,
    browsePlaylistId,
    curatedLiveItems,
    displayedLibraryPlaylists,
    filteredArtistTracks,
    filteredLocalTracks,
    firstHomeTrackShelf,
    homeRecommendations,
    homeShelfPlaylists,
    homeShelfCategories,
    mode,
    normalizedQuery,
    openPlaylistBrowse,
    openOnlineBrowseCategory,
    openSearchArtistBrowse,
    playArtistFromIndex,
    playOnlineRadioSeed,
    playOnlineSearchTrack,
    playOnlineShelfTrack,
    playPlaylistFromIndex,
    playlistTracks,
    searchArtists,
    searchItems,
    searchPlaylists,
  ]);

  const liveSearchNotice =
    mode === "hush" && normalizedQuery && curatedLiveItems.length === 0
      ? props.text.listen.searchEmpty
      : "";
  const onlineSearchNotice =
    mode === "muse" && !artistBrowsePage && normalizedQuery
      ? searchLoading
        ? props.text.listen.searchLoading
        : searchError
          ? props.text.listen.searchUnavailable
          : searchItems.length === 0 &&
              searchArtists.length === 0 &&
              searchPlaylists.length === 0 &&
              displayedLibraryPlaylists.length === 0
            ? props.text.listen.searchEmpty
            : ""
      : "";
  const filteredPlaylistTracks = React.useMemo(
    () =>
      playlistTracks.filter((item) =>
        matchesQuery(normalizedQuery, [
          item.title,
          item.channel,
          item.description,
          item.durationLabel,
          item.playCountLabel ?? "",
        ]),
      ),
    [normalizedQuery, playlistTracks],
  );
  const filteredOnlineQueueItems = React.useMemo(
    () =>
      onlinePlaybackQueue.filter((item) =>
        matchesQuery(normalizedQuery, [
          item.title,
          item.channel,
          item.description,
          item.durationLabel,
          item.playCountLabel ?? "",
        ]),
      ),
    [normalizedQuery, onlinePlaybackQueue],
  );
  const onlineQueueTitle =
    onlineQueueState.kind === "playlist"
      ? onlineQueueState.title ||
        selectedPlaylist?.title ||
        props.text.listen.groupPlaylist
      : onlineQueueState.kind === "radio"
        ? onlineQueueState.title || props.text.listen.groupRadio
        : props.text.listen.upNext;
  const effectiveSidebarView: ListenSidebarView =
    mode === "muse" ? sidebarView : "browse";
  const showArtistDetail =
    mode === "muse" &&
    effectiveSidebarView === "browse" &&
    artistBrowsePage !== null &&
    browsePlaylistId === "";
  const showPlaylistDetail =
    mode === "muse" &&
    effectiveSidebarView === "browse" &&
    browsePlaylistId !== "";
  const searchPlaceholder =
    mode === "hush"
      ? props.text.listen.searchLive
      : mode === "muse"
        ? props.text.listen.searchOnline
        : props.text.listen.searchLocal;

  return (
    <>
      <ListenPageView
        page={props}
        state={{ isWindows, isMac, listOpen, query, searchPlaceholder, mode, playbackMode, sidebarView, effectiveSidebarView, onlineBrowseSource, onlineBrowseDetail, liveGroups, selectedLiveGroupId, liveStatusByVideoId, liveCatalogLoading, liveCatalogError, liveCatalogMessage, liveUserCatalog, liveUserCatalogLoading, liveUserCatalogSaving, liveUserCatalogError, curatedLiveItems, liveSelectionArmed, selectedLiveId, filteredOnlineQueueItems, onlineQueueTitle, onlineQueueCanUndo, onlineQueueCanRedo, selectedOnlineId, filteredLocalTracks, selectedLocalId, localPlaying, liveSearchNotice, showArtistDetail, artistBrowsePage, artistActionBusy, filteredArtistShelves, browsePlaylistId, savedPlaylistIds, playlistMutationAction, playlistMutationPlaylistId, filteredArtistTracks, showPlaylistDetail, selectedPlaylist, playlistLoading, playlistAppending, playlistDetailAuthor, playlistTracks, filteredPlaylistTracks, playlistContinuation, normalizedQuery, libraryLoading, libraryAppending, libraryError, libraryErrorCode, searchLoading, searchAppending, searchItems, searchArtists, searchPlaylists, searchContinuation, libraryArtists, displayedLibraryPlaylists, showLibraryPlaylistGroup, homeShelves, libraryContinuation, onlineSearchNotice, localTracks, localTracksLoading: localTrackIndex.loading, localTracksRefreshing: localTrackIndex.refreshing, localTracksClearingMissing: localTrackIndex.clearingMissing, activeOnline, selectedLocal: activeLocal, onlinePlayerCommand, localPlayerCommand, onlineQueueItems: playbackMode === "hush" ? liveQueue : onlinePlaybackQueue, onlinePlaying, onlinePlaybackArmed, selectedLocalResumeTime, activeOnlineResumeTime, onlineProgress, onlineState, onlineObservedPlaybackAudioQuality, activeOnlineFavorite, activeOnlineFavoriteBusy, localProgress, muted, volume, playMode, museConnectBusy: youtubeConnectBusy, museAccountName, museAccountAvatarURL, museAccountConnected, museAccountBusy, museManualRefreshKind }}
        actions={{ setListOpen, setQuery, selectFirstResult, setMode, setSidebarView, reloadLiveCatalog, saveLiveUserCatalog, reloadLibrary: () => setLibraryReloadToken((current) => current + 1), changeOnlineBrowseSource, openOnlineBrowseCategory, closeOnlineBrowseDetail, loadMoreLibrary, activateLiveSelection, selectOnlineQueueTrack, selectLocalQueueTrack, setSelectedLocalId, setLocalPlayerCommand, closeArtistBrowse, playArtistFromIndex, shuffleArtist, loadMoreArtist, loadArtistShelfTracks, playArtistMix, toggleArtistSubscription, openPlaylistBrowse, updatePlaylistLibrary, setBrowsePlaylistId, playPlaylistFromIndex, playPlaylistNext, addPlaylistToQueue, loadMorePlaylist, playOnlineShelfTrack, playOnlineShelfAll, shuffleOnlineShelf, playOnlineSearchTrack, playOnlineSearchResults, shuffleOnlineSearchResults, loadMoreSearch, openSearchArtistBrowse, clearOnlineQueue, removeOnlineQueueItem, moveOnlineQueueItem, undoOnlineQueueEdit, redoOnlineQueueEdit, openRepairMissingLocalTracks: () => setLocalRelinkDialogOpen(true), handlePlaybackEnded, setOnlinePlaying: setLivePlaying, setOnlineState: setLiveState, handleOnlineProgressChange, handleOnlineNativeTrackChange, setLocalPlaying, handleLocalProgressChange, setPlaybackSessionStarted, connectYouTube, refreshMusePage, signOutMuseAccount, playPrevious, playNext, togglePlayMode, setPlayMode: setPlayModeFromView, togglePlayback, toggleMute, handleVolumeChange, toggleOnlineFavorite, openOnlineArtistBrowse, openSelectedLocalDirectory }}
      />
      <ListenLocalRelinkRepair
        open={localRelinkDialogOpen}
        text={props.text}
        clearingMissing={localTrackIndex.clearingMissing}
        onOpenChange={setLocalRelinkDialogOpen}
        onClearMissing={localTrackIndex.clearMissing}
        onRefreshLocalTracks={localTrackIndex.refresh}
      />
    </>
  );

}
