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

import { fetchCompleteListenPlaylistQueue,fetchListenArtist,fetchListenLibrary,fetchListenLiveCatalog,fetchListenLiveStatuses,fetchListenLiveUserCatalog,fetchListenPlaylistPage,fetchListenPlaylistQueue,fetchListenSearch,fetchListenTrackFavorite,fetchListenTrackFavoriteStatuses,fetchListenTrackInfo,getListenErrorCode,getListenErrorMessage,getListenErrorRetryable,saveListenLiveUserCatalog,updateListenArtistSubscription,updateListenPlaylistLibrary,updateListenTrackFavorite } from "@/app/main/listen/api";
import type { ListenLiveUserCatalog } from "@/app/main/listen/api";
import { handleListenYouTubeAppSessionEvent } from "@/app/main/listen/app-session-event";
import { beginListenArtistRequest,createListenArtistIdentity,createListenArtistRequestRegistry,finishListenArtistRequest,invalidateListenArtistRequests,isListenArtistRequestCurrent,synchronizeListenArtistRequestIdentity } from "@/app/main/listen/artist-request-race";
import { LISTEN_LIKED_SONGS_SHELF_ID,LISTEN_LIVE_PLAYER_SERVICE,LISTEN_NATIVE_PLAYER_SERVICE,LISTEN_YOUTUBE_APP_SESSION_CHANGED_EVENT,LISTEN_YOUTUBE_REGION_UNAVAILABLE_ERROR_CODE,LISTEN_YOUTUBE_VERIFICATION_REQUIRED_ERROR_CODE } from "@/app/main/listen/catalog";
import { clampVolume,matchesQuery,normalizeSearch,resolveListenLiveSelectionId,resolveQueueIndex,useListenLocalTracks } from "@/app/main/listen/local-library";
import { ListenLocalRelinkRepair } from "@/app/main/listen/LocalRelinkRepair";
import { mergeListenLibraryPagePlaylists } from "@/app/main/listen/library-pagination";
import { isListenLibraryPageRequestCurrent,isListenLibraryRequestReady,isSameListenArtistBrowseIdentity,resolveListenLibraryPageCacheKey,type ListenLibraryPageCacheEntry } from "@/app/main/listen/library-view-state";
import { createNativeOnlineItem,mergeListenNativeTrackItem,nativeStatusToPlayerEvent,normalizeListenTrackArtists } from "@/app/main/listen/native-playback-projection";
import { abortListenPaginationRequests,abortStaleListenPaginationRequests,beginListenPaginationRequest,createListenPaginationContextKey,finishListenPaginationRequest,isListenPaginationContextCurrent,resolveListenNextContinuation,type ListenPaginationKind,type ListenPaginationRequestRegistry } from "@/app/main/listen/pagination-race";
import { ListenPageView } from "@/app/main/listen/PageView";
import { applyListenPlaylistPlaybackFallback,placeListenPlaylistQueueContinuation,resolveListenPlaylistPlaybackFallbackArtist,resolveListenPlaylistQueueAction,startListenPlaylistPlayback,startListenPlaylistPlaybackFromIndex,startListenPlaylistQueueAction } from "@/app/main/listen/playlist-playback";
import { resolveListenWorkspaceViewMode,resolveMusicWorkspaceRoute,shouldLoadListenWorkspaceBrowse } from "@/app/main/listen/workspace-routes";
import { callListenPlaybackAppendToQueue,callListenPlaybackClearQueue,callListenPlaybackInsertNextInQueue,callListenPlaybackMergeTrackMetadata,callListenPlaybackMoveQueueItem,callListenPlaybackNext,callListenPlaybackObserveNativeEvent,callListenPlaybackPause,callListenPlaybackPlayFromQueue,callListenPlaybackPlayPause,callListenPlaybackPlayQueue,callListenPlaybackPlayTrack,callListenPlaybackPrevious,callListenPlaybackRedoQueue,callListenPlaybackRemoveFromQueue,callListenPlaybackResume,callListenPlaybackSeek,callListenPlaybackSetLanguage,callListenPlaybackSetRepeatMode,callListenPlaybackSetShuffle,callListenPlaybackSetVolume,callListenPlaybackUndoQueue,createListenPlaybackQueueIdentity,listenRepeatModeFromPlayMode,type ListenPlaybackSnapshot } from "@/app/main/listen/playback-api";
import { hasTrustedListenOnlineArtist,isListenLiveEventForSession,resolveListenPlaybackActivity } from "@/app/main/listen/playback-helpers";
import { useListenPlaybackStore } from "@/app/main/listen/playback-store";
import { pushListenForwardSkipIndex,resolveListenQueueNextAction,resolveListenQueuePreviousAction } from "@/app/main/listen/queue";
import { buildListenPosterCandidates,dedupeLibraryShelves,dedupeOnlineItems,dedupePlaylistItems,readListenStorageState,updateListenProgressMap,writeListenStorageState } from "@/app/main/listen/storage";
import type { ListenArtistBrowseState,ListenArtistItem,ListenCategoryItem,ListenLibraryShelf,ListenLiveGroup,ListenLiveStatus,ListenMode,ListenNativePlayerEvent,ListenNowPlayingStatus,ListenOnlineBrowseDetail,ListenOnlineBrowseSource,ListenOnlineItem,ListenPageProps,ListenPlayMode,ListenPlaybackProgressState,ListenPlayerCommand,ListenPlaylistItem,ListenPlaylistLibraryAction,ListenRemotePlaybackState,ListenSidebarView } from "@/app/main/listen/types";
import { useListenLocalQueue } from "@/app/main/listen/useListenLocalQueue";
import { openListenArtistFromPlayerSurface } from "@/app/main/listen/workspace-player-shared";
export { ListenLocalPreviewPlayer } from "@/app/main/listen/LocalPreviewPlayer";
export type { ListenExternalCommand,ListenLocalPreviewTrack,ListenMode,ListenNowPlayingStatus,ListenPlaybackSource } from "@/app/main/listen/types";

const LISTEN_LIVE_STATUS_POLL_MS = 60_000;
const LISTEN_LIVE_STATUS_WARM_POLL_MS = 4_000;
const LISTEN_ARTIST_SHELF_CONTINUATION_MAX_PAGES = 20;

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
  const refetchAppSessions = appSessions.refetch;
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
  const activeViewMode = resolveListenWorkspaceViewMode({
    workspaceLayout: props.workspaceLayout,
    workspaceRouteId: props.workspaceRouteId,
    fallbackMode: mode,
  });
  const setLegacyBrowseMode = React.useCallback(
    (nextMode: React.SetStateAction<ListenMode>) => {
      if (!props.workspaceLayout) {
        setMode(nextMode);
      }
    },
    [props.workspaceLayout],
  );
  const museBrowseActive = shouldLoadListenWorkspaceBrowse({
    active: props.active,
    viewMode: activeViewMode,
    targetMode: "muse",
  });
  const hushBrowseActive = shouldLoadListenWorkspaceBrowse({
    active: props.active,
    viewMode: activeViewMode,
    targetMode: "hush",
  });
  const [playbackMode, setPlaybackMode] = React.useState<ListenMode>(
    initialPersistedState.playbackMode,
  );
  const modeRef = React.useRef(activeViewMode);
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
  const [libraryErrorRetryable, setLibraryErrorRetryable] =
    React.useState(false);
  const [librarySettledCacheKey, setLibrarySettledCacheKey] =
    React.useState("");
  const [libraryReloadToken, setLibraryReloadToken] = React.useState(0);
  const [museAccountReloadToken, setMuseAccountReloadToken] = React.useState(0);
  const [museManualRefreshKind, setMuseManualRefreshKind] = React.useState<
    "" | "artist" | "library" | "playlist" | "search"
  >("");
  const [playlistTracks, setPlaylistTracks] = React.useState<
    ListenOnlineItem[]
  >([]);
  const [playlistContinuation, setPlaylistContinuation] = React.useState("");
  const [playlistDetailAuthor, setPlaylistDetailAuthor] = React.useState(""); const [playlistDetailMetadata, setPlaylistDetailMetadata] = React.useState({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
  const [playlistDetailTitle, setPlaylistDetailTitle] = React.useState("");
  const [playlistDetailDescription, setPlaylistDetailDescription] =
    React.useState("");
  const [playlistDetailThumbnailURL, setPlaylistDetailThumbnailURL] =
    React.useState("");
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
  const [onlinePlaybackActionPending, setOnlinePlaybackActionPending] =
    React.useState(false);
  const [livePlaying, setLivePlaying] = React.useState(false);
  const [liveState, setLiveState] =
    React.useState<ListenRemotePlaybackState>("idle");
  const [onlinePlaybackErrorCode, setOnlinePlaybackErrorCode] =
    React.useState("");
  const [onlinePlaybackErrorMessage, setOnlinePlaybackErrorMessage] =
    React.useState("");
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
  const museVolumeObservedKeyRef = React.useRef("");
  const museVolumeDesiredKeyRef = React.useRef("");
  const museVolumeWriteTimerRef = React.useRef<number | null>(null);
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
  const onlineBrowseSourceRef = React.useRef(onlineBrowseSource);
  const onlineBrowseDetailRef = React.useRef(onlineBrowseDetail);
  const listenLanguageRef = React.useRef(props.text.locale);
  onlineBrowseSourceRef.current = onlineBrowseSource;
  onlineBrowseDetailRef.current = onlineBrowseDetail;
  listenLanguageRef.current = props.text.locale;
  const paginationRequestsRef = React.useRef<ListenPaginationRequestRegistry>(
    new Map(),
  );
  const playlistQueueLoadRef = React.useRef<{
    context: string;
    controller: AbortController;
    promise: Promise<ListenOnlineItem[]>;
  } | null>(null);
  const artistRequestsRef = React.useRef(
    createListenArtistRequestRegistry(),
  );
  const paginationContextsRef = React.useRef<
    Record<ListenPaginationKind, string>
  >({
    artist: "",
    library: "",
    playlist: "",
    search: "",
  });
  const clearForwardSkipNavigationStack = React.useCallback(() => {
    forwardSkipIndexStackRef.current = [];
  }, []);

  const localTrackIndex = useListenLocalTracks(
    props.libraries,
    props.httpBaseURL,
  );
  const localTracks = localTrackIndex.tracks;
  const listenLanguage = props.text.locale;
  React.useEffect(() => {
    void callListenPlaybackSetLanguage(listenLanguage).catch(() => {});
  }, [listenLanguage]);
  const currentLibraryPageCacheKey = resolveListenLibraryPageCacheKey(
    onlineBrowseSource,
    onlineBrowseDetail,
    listenLanguage,
  );
  const librarySettled =
    librarySettledCacheKey !== "" &&
    librarySettledCacheKey === currentLibraryPageCacheKey;
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
    invalidateListenArtistRequests(
      artistRequestsRef.current,
      artistRequestsRef.current.artistIdentity,
    );
    libraryPageCacheRef.current.clear();
    activeLibraryPageCacheKeyRef.current = "";
    setLibrarySettledCacheKey("");
    favoriteOverrideByVideoIdRef.current = {};
    setHomeShelves([]);
    setLibraryPlaylists([]);
    setLibraryArtists([]);
    setLibraryContinuation("");
    setLibraryAppending(false);
    setLibraryLoading(true);
    setLibraryError(false);
    setLibraryErrorCode("");
    setLibraryErrorRetryable(false);
    setSearchItems([]);
    setSearchArtists([]);
    setSearchPlaylists([]);
    setSearchContinuation("");
    setSearchAppending(false);
    setSearchLoading(false);
    setSearchError(false);
    setPlaylistTracks([]);
    setPlaylistContinuation("");
    setPlaylistDetailAuthor(""); setPlaylistDetailMetadata({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
    setPlaylistDetailTitle("");
    setPlaylistDetailDescription("");
    setPlaylistDetailThumbnailURL("");
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
    invalidateListenArtistRequests(artistRequestsRef.current, "");
    libraryPageCacheRef.current.clear();
    activeLibraryPageCacheKeyRef.current = "";
    setLibrarySettledCacheKey("");
    onlineBrowseSourceRef.current = "home";
    onlineBrowseDetailRef.current = null;
    favoriteOverrideByVideoIdRef.current = {};
    museAccountConnectedRef.current = false;
    onlinePlaybackActionEpochRef.current += 1;
    onlinePlaybackActionPendingRef.current = false;
    setOnlinePlaybackActionPending(false);
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
    setLibraryErrorRetryable(false);
    setPlaylistTracks([]);
    setPlaylistContinuation("");
    setPlaylistDetailAuthor(""); setPlaylistDetailMetadata({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
    setPlaylistDetailTitle("");
    setPlaylistDetailDescription("");
    setPlaylistDetailThumbnailURL("");
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
    setLegacyBrowseMode("muse");
    setPlaybackMode("muse");
  }, [setLegacyBrowseMode]);
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
  const onlineQueueStateRef = React.useRef(onlineQueueState);
  onlineQueueStateRef.current = onlineQueueState;
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
  const libraryRequestReady = isListenLibraryRequestReady({
    accountConnected: museAccountConnected,
    httpBaseURL: props.httpBaseURL,
  });
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
    setLegacyBrowseMode("muse");
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
    setLegacyBrowseMode,
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
    modeRef.current = activeViewMode;
    playbackModeRef.current = playbackMode;
  }, [activeViewMode, playbackMode]);

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
        handleListenYouTubeAppSessionEvent(event, {
          onConnected: () => setYouTubeLoggedOutOverride(false),
          onDisconnected: () => {
            setYouTubeLoggedOutOverride(true);
            resetMuseAccountViewForLogout();
          },
          onReload: reloadMuseAccountData,
          onRefetch: () => {
            void refetchAppSessions();
          },
        });
      },
    );
    return () => offYouTubeAppSessionChanged();
  }, [refetchAppSessions, reloadMuseAccountData, resetMuseAccountViewForLogout]);

  React.useEffect(
    () => () => {
      nativeTrackLookupRef.current.forEach((controller) =>
        controller.abort(),
      );
      nativeTrackLookupRef.current.clear();
      abortListenPaginationRequests(paginationRequestsRef.current);
      playlistQueueLoadRef.current?.controller.abort();
      playlistQueueLoadRef.current = null;
    },
    [],
  );

  React.useEffect(() => {
    libraryPageCacheRef.current.clear();
    activeLibraryPageCacheKeyRef.current = "";
    setLibrarySettledCacheKey("");
  }, [listenLanguage, props.httpBaseURL]);

  React.useEffect(() => {
    setOnlineFavoriteByVideoId({});
    setFavoriteLoadingVideoId("");
    setFavoriteMutationVideoId("");
  }, [listenLanguage, props.httpBaseURL]);

  const {
    selectedLocalId,
    setSelectedLocalId,
    localPlaybackQueueIds,
    localPlaybackQueue,
    localQueueCanUndo,
    localQueueCanRedo,
    selectLocalQueueTrack,
    clearLocalQueue,
    removeLocalQueueItem,
    moveLocalQueueItem,
    undoLocalQueueEdit,
    redoLocalQueueEdit,
    playLocalBrowseTrack,
  } = useListenLocalQueue({
    tracks: localTracks,
    initialQueueIds: initialPersistedState.localPlaybackQueueIds,
    initialSelectedId: initialPersistedState.selectedLocalId,
    loading: localTrackIndex.loading,
    error: localTrackIndex.error,
    playing: localPlaying,
    setPlaying: setLocalPlaying,
    setPlaybackMode,
    setPlayerCommand: setLocalPlayerCommand,
    clearForwardSkipNavigationStack,
  });

  const normalizedQuery = normalizeSearch(query);
  const searchPaginationContext = createListenPaginationContextKey([
      "search",
      props.active ? "active" : "inactive",
      activeViewMode,
      museAccountConnected,
      props.httpBaseURL,
      listenLanguage,
      query,
      museAccountReloadToken,
    ]);
  const playlistPaginationContext = createListenPaginationContextKey([
      "playlist",
      props.active ? "active" : "inactive",
      activeViewMode,
      museAccountConnected,
      props.httpBaseURL,
      listenLanguage,
      browsePlaylistId.trim(),
      museAccountReloadToken,
    ]);
  const artistPaginationContext = createListenPaginationContextKey([
      "artist",
      props.active ? "active" : "inactive",
      activeViewMode,
      museAccountConnected,
      props.httpBaseURL,
      listenLanguage,
      artistBrowsePage?.id.trim() ?? "",
      artistBrowsePage?.name.trim() ?? "",
      museAccountReloadToken,
    ]);
  const libraryPaginationContext = createListenPaginationContextKey([
      "library",
      props.active ? "active" : "inactive",
      activeViewMode,
      museAccountConnected,
      props.httpBaseURL,
      resolveListenLibraryPageCacheKey(
        onlineBrowseSource,
        onlineBrowseDetail,
        listenLanguage,
      ),
      libraryReloadToken,
      museAccountReloadToken,
    ]);
  React.useLayoutEffect(() => {
    const contexts = paginationContextsRef.current;
    const commitContext = (
      kind: ListenPaginationKind,
      nextContext: string,
    ) => {
      if (contexts[kind] === nextContext) {
        return;
      }
      contexts[kind] = nextContext;
      abortStaleListenPaginationRequests(
        paginationRequestsRef.current,
        kind,
        nextContext,
      );
    };
    commitContext("search", searchPaginationContext);
    commitContext("playlist", playlistPaginationContext);
    commitContext("artist", artistPaginationContext);
    commitContext("library", libraryPaginationContext);
  }, [
    artistPaginationContext,
    libraryPaginationContext,
    playlistPaginationContext,
    searchPaginationContext,
  ]);
  const refreshMusePage = React.useCallback(() => {
    if (!museBrowseActive) {
      return;
    }
    if (artistBrowsePage) {
      invalidateListenArtistRequests(
        artistRequestsRef.current,
        createListenArtistIdentity(artistBrowsePage),
      );
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
      setPlaylistDetailAuthor(""); setPlaylistDetailMetadata({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
      setPlaylistDetailTitle("");
      setPlaylistDetailDescription("");
      setPlaylistDetailThumbnailURL("");
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
    setLibraryErrorRetryable(false);
    setLibrarySettledCacheKey("");
    setLibraryReloadToken((current) => current + 1);
  }, [artistBrowsePage, browsePlaylistId, museBrowseActive, normalizedQuery.length]);

  React.useEffect(() => {
    if (
      !museBrowseActive ||
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
  }, [artistBrowsePage, listenLanguage, museAccountConnected, museAccountReloadToken, museBrowseActive, normalizedQuery, props.httpBaseURL, query]);

  React.useEffect(() => {
    if (!museBrowseActive) {
      activeLibraryPageCacheKeyRef.current = "";
      setMuseManualRefreshKind((current) =>
        current === "library" ? "" : current,
      );
      return;
    }
    if (!museAccountConnected) {
      activeLibraryPageCacheKeyRef.current = "";
      libraryPageCacheRef.current.clear();
      setLibrarySettledCacheKey("");
      setHomeShelves([]);
      setLibraryPlaylists([]);
      setLibraryArtists([]);
      setLibraryContinuation("");
      setLibraryLoading(false);
      setLibraryAppending(false);
      setLibraryError(false);
      setLibraryErrorCode("");
      setLibraryErrorRetryable(false);
      setMuseManualRefreshKind((current) =>
        current === "library" ? "" : current,
      );
      return;
    }
    if (!libraryRequestReady) {
      activeLibraryPageCacheKeyRef.current = "";
      libraryPageCacheRef.current.clear();
      setLibrarySettledCacheKey("");
      setHomeShelves([]);
      setLibraryPlaylists([]);
      setLibraryArtists([]);
      setLibraryContinuation("");
      setLibraryLoading(false);
      setLibraryAppending(false);
      setLibraryError(false);
      setLibraryErrorCode("");
      setLibraryErrorRetryable(false);
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
      setLibrarySettledCacheKey(cacheKey);
      setLibraryPlaylists(cachedPage.playlists);
      setLibraryArtists(cachedPage.artists);
      setHomeShelves(cachedPage.shelves);
      setLibraryContinuation(cachedPage.continuation);
      setLibraryLoading(false);
      setLibraryAppending(false);
      setLibraryError(false);
      setLibraryErrorCode("");
      setLibraryErrorRetryable(false);
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
    setLibraryErrorRetryable(false);
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
        if (
          isListenLibraryPageRequestCurrent({
            activeCacheKey: activeLibraryPageCacheKeyRef.current,
            requestCacheKey: cacheKey,
            aborted: controller.signal.aborted,
          })
        ) {
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
          setLibrarySettledCacheKey(cacheKey);
        }
      })
      .catch((error) => {
        if (
          isListenLibraryPageRequestCurrent({
            activeCacheKey: activeLibraryPageCacheKeyRef.current,
            requestCacheKey: cacheKey,
            aborted: controller.signal.aborted,
          })
        ) {
          libraryPageCacheRef.current.delete(cacheKey);
          setLibraryPlaylists([]);
          setLibraryArtists([]);
          setHomeShelves([]);
          setLibraryContinuation("");
          setLibraryError(true);
          setLibraryErrorCode(getListenErrorCode(error));
          setLibraryErrorRetryable(getListenErrorRetryable(error));
          setLibrarySettledCacheKey(cacheKey);
        }
      })
      .finally(() => {
        if (
          isListenLibraryPageRequestCurrent({
            activeCacheKey: activeLibraryPageCacheKeyRef.current,
            requestCacheKey: cacheKey,
            aborted: controller.signal.aborted,
          })
        ) {
          setLibraryLoading(false);
          setMuseManualRefreshKind((current) =>
            current === "library" ? "" : current,
          );
        }
      });
    return () => controller.abort();
  }, [libraryReloadToken, libraryRequestReady, listenLanguage, museAccountConnected, museAccountReloadToken, museBrowseActive, onlineBrowseDetail, onlineBrowseSource, props.httpBaseURL]);

  const artistBrowseId = artistBrowsePage?.id ?? "";
  const artistBrowseName = artistBrowsePage?.name ?? "";
  const artistRequestIdentity = createListenArtistIdentity(artistBrowsePage);
  React.useLayoutEffect(() => {
    if (!museBrowseActive) {
      invalidateListenArtistRequests(
        artistRequestsRef.current,
        artistRequestIdentity,
      );
      setArtistActionBusy("");
      return;
    }
    if (
      synchronizeListenArtistRequestIdentity(
        artistRequestsRef.current,
        artistRequestIdentity,
      )
    ) {
      setArtistActionBusy("");
    }
  }, [artistRequestIdentity, museBrowseActive]);
  React.useEffect(
    () => () => {
      invalidateListenArtistRequests(artistRequestsRef.current, "");
    },
    [],
  );
  React.useEffect(() => {
    if (
      !museBrowseActive ||
      (!artistBrowseId && !artistBrowseName)
    ) {
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
                  description: payload.description,
                  thumbnailUrl: payload.thumbnailUrl || current.thumbnailUrl,
                  heroThumbnailUrl:
                    payload.heroThumbnailUrl ||
                    payload.thumbnailUrl ||
                    current.heroThumbnailUrl ||
                    current.thumbnailUrl,
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
  }, [artistBrowseId, artistBrowseName, listenLanguage, museAccountReloadToken, museBrowseActive, props.httpBaseURL]);

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

  const beginListenLibraryNavigation = React.useCallback(
    (
      source: ListenOnlineBrowseSource,
      detail: ListenOnlineBrowseDetail | null,
    ) => {
      const nextCacheKey = resolveListenLibraryPageCacheKey(
        source,
        detail,
        listenLanguageRef.current,
      );
      const currentCacheKey = resolveListenLibraryPageCacheKey(
        onlineBrowseSourceRef.current,
        onlineBrowseDetailRef.current,
        listenLanguageRef.current,
      );
      onlineBrowseSourceRef.current = source;
      onlineBrowseDetailRef.current = detail;
      if (
        currentCacheKey === nextCacheKey &&
        activeLibraryPageCacheKeyRef.current === nextCacheKey
      ) {
        return;
      }

      // Invalidate the previous request immediately. The source state and the
      // fetch effect commit on separate renders, so loading must begin here to
      // keep the intermediate render out of the empty-success state.
      activeLibraryPageCacheKeyRef.current = nextCacheKey;
      setLibraryLoading(true);
      setLibraryAppending(false);
      setLibraryError(false);
      setLibraryErrorCode("");
      setLibraryErrorRetryable(false);
    },
    [],
  );

  const changeOnlineBrowseSource = React.useCallback(
    (source: ListenOnlineBrowseSource) => {
      beginListenLibraryNavigation(source, null);
      setOnlineBrowseSource(source);
      setOnlineBrowseDetail(null);
      setBrowsePlaylistId("");
      setArtistBrowsePage(null);
    },
    [beginListenLibraryNavigation],
  );

  React.useEffect(() => {
    if (!props.workspaceLayout) {
      return;
    }
    const route = props.workspaceRouteId?.trim() || "home";
    const descriptor = resolveMusicWorkspaceRoute(route);
    setListOpen(true);
    setSidebarView("browse");
    setMode(descriptor.mode);
    if (descriptor.content !== "search" && descriptor.content !== "local-search") {
      setQuery("");
    }
    if (descriptor.browseSource) {
      changeOnlineBrowseSource(descriptor.browseSource);
    }
  }, [
    changeOnlineBrowseSource,
    props.workspaceLayout,
    props.workspaceRouteId,
  ]);

  const openOnlineBrowseCategory = React.useCallback(
    (item: ListenCategoryItem) => {
      const browseId = item.browseId.trim();
      if (!browseId) {
        return;
      }
      setLegacyBrowseMode("muse");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setArtistBrowsePage(null);
      setQuery("");
      const detail: ListenOnlineBrowseDetail = {
        id: item.id,
        source: onlineBrowseSource,
        browseId,
        params: item.params.trim(),
        title: item.title.trim() || browseId,
      };
      beginListenLibraryNavigation(onlineBrowseSource, detail);
      setOnlineBrowseDetail(detail);
    },
    [beginListenLibraryNavigation, onlineBrowseSource],
  );

  const closeOnlineBrowseDetail = React.useCallback(() => {
    beginListenLibraryNavigation(onlineBrowseSourceRef.current, null);
    setOnlineBrowseDetail(null);
  }, [beginListenLibraryNavigation]);

  const loadMoreLibrary = React.useCallback(() => {
    const continuation = libraryContinuation.trim();
    if (
      !museBrowseActive ||
      !libraryRequestReady ||
      !continuation ||
      libraryAppending ||
      libraryLoading
    ) {
      return;
    }
    const cacheKey = resolveListenLibraryPageCacheKey(
      onlineBrowseSource,
      onlineBrowseDetail,
      listenLanguage,
    );
    const request = beginListenPaginationRequest(
      paginationRequestsRef.current,
      "library",
      libraryPaginationContext,
      continuation,
    );
    if (!request) {
      return;
    }
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
    setLibraryAppending(true);
    void fetchListenLibrary(
      props.httpBaseURL,
      request.controller.signal,
      onlineBrowseSource,
      {
        browseId: onlineBrowseDetail?.browseId,
        params: onlineBrowseDetail?.params,
        continuation,
        language: listenLanguage,
      },
    )
      .then((payload) => {
        if (
          request.controller.signal.aborted ||
          !isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.library,
          ) ||
          activeLibraryPageCacheKeyRef.current !== cacheKey
        ) {
          return;
        }
        const nextPage: ListenLibraryPageCacheEntry = {
          playlists: mergeListenLibraryPagePlaylists(
            onlineBrowseSource,
            basePage.playlists,
            payload.playlists,
          ),
          artists: basePage.artists,
          shelves: dedupeLibraryShelves([
            ...basePage.shelves,
            ...payload.shelves,
          ]),
          continuation: resolveListenNextContinuation(
            continuation,
            payload.continuation,
          ),
          reloadToken: libraryReloadToken,
        };
        libraryPageCacheRef.current.set(cacheKey, nextPage);
        setLibraryPlaylists(nextPage.playlists);
        setLibraryArtists(nextPage.artists);
        setHomeShelves(nextPage.shelves);
        setLibraryContinuation(nextPage.continuation);
      })
      .catch(() => {
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.library,
          ) &&
          activeLibraryPageCacheKeyRef.current === cacheKey
        ) {
          const currentPage = libraryPageCacheRef.current.get(cacheKey);
          if (currentPage) {
            libraryPageCacheRef.current.set(cacheKey, {
              ...currentPage,
              continuation: "",
            });
          }
          setLibraryContinuation("");
        }
      })
      .finally(() => {
        finishListenPaginationRequest(
          paginationRequestsRef.current,
          request,
        );
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.library,
          ) &&
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
    libraryRequestReady,
    museBrowseActive,
    libraryPlaylists,
    libraryPaginationContext,
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
    if (!hushBrowseActive) {
      setLiveCatalogLoading(false);
      return;
    }
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
  }, [hushBrowseActive, liveCatalogReloadToken, props.httpBaseURL]);

  React.useEffect(() => {
    if (!hushBrowseActive) {
      setLiveUserCatalogLoading(false);
      return;
    }
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
  }, [hushBrowseActive, liveUserCatalogReloadToken, props.httpBaseURL]);

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
    if (!hushBrowseActive || liveSelectionArmed) {
      return;
    }
    const nextId = resolveListenLiveSelectionId(liveQueue, selectedLiveId);
    if (!nextId) {
      return;
    }
    setSelectedLiveId(nextId);
    setLiveSelectionArmed(true);
  }, [hushBrowseActive, liveQueue, liveSelectionArmed, selectedLiveId]);

  React.useEffect(() => {
    if (
      !hushBrowseActive ||
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
    hushBrowseActive,
    props.httpBaseURL,
  ]);

  React.useEffect(() => {
    if (hushBrowseActive && liveStatusVideoIds.length > 0) {
      return;
    }
    setLiveStatusByVideoId((current) => {
      if (Object.keys(current).length === 0) {
        return current;
      }
      return {};
    });
  }, [hushBrowseActive, liveStatusVideoIds.length]);
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
    activeViewMode === "muse" &&
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
  const selectedPlaylistSource =
    allOnlinePlaylists.find((item) => item.playlistId === browsePlaylistId) ??
    null;
  const selectedPlaylist = browsePlaylistId.trim()
    ? {
        id: selectedPlaylistSource?.id || `playlist-detail-${browsePlaylistId}`,
        playlistId: browsePlaylistId,
        title:
          playlistDetailTitle ||
          selectedPlaylistSource?.title ||
          props.text.listen.groupPlaylist,
        channel: selectedPlaylistSource?.channel || "",
        description:
          playlistDetailDescription || selectedPlaylistSource?.description || "",
        thumbnailUrl:
          playlistDetailThumbnailURL || selectedPlaylistSource?.thumbnailUrl,
      }
    : null;
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
    setOnlinePlaybackErrorCode("");
    setOnlinePlaybackErrorMessage("");
  }, [activeOnline?.videoId, playbackMode]);

  React.useEffect(() => {
    if (playbackMode !== "muse" || !musePlayback.hydrated) {
      return;
    }
    const nextVolume = clampVolume(musePlayback.volume);
    const nextMuted = musePlayback.muted || nextVolume <= 0;
    const nextKey = `${nextVolume}:${nextMuted}`;
    if (
      museVolumeDesiredKeyRef.current &&
      museVolumeDesiredKeyRef.current !== nextKey
    ) {
      return;
    }
    if (museVolumeDesiredKeyRef.current === nextKey) {
      museVolumeDesiredKeyRef.current = "";
    }
    museVolumeObservedKeyRef.current = nextKey;
    setVolume(nextVolume);
    setMuted(nextMuted);
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
    const nextKey = `${volume}:${muted}`;
    if (museVolumeObservedKeyRef.current === nextKey) {
      return;
    }
    museVolumeDesiredKeyRef.current = nextKey;
    if (museVolumeWriteTimerRef.current !== null) {
      window.clearTimeout(museVolumeWriteTimerRef.current);
    }
    museVolumeWriteTimerRef.current = window.setTimeout(() => {
      museVolumeWriteTimerRef.current = null;
      void callListenPlaybackSetVolume(volume, muted)
        .then((snapshot) => applyOnlinePlaybackSnapshot(snapshot))
        .catch(() => {
          if (museVolumeDesiredKeyRef.current === nextKey) {
            museVolumeDesiredKeyRef.current = "";
          }
        });
    }, 40);
    return () => {
      if (museVolumeWriteTimerRef.current !== null) {
        window.clearTimeout(museVolumeWriteTimerRef.current);
        museVolumeWriteTimerRef.current = null;
      }
    };
  }, [
    applyOnlinePlaybackSnapshot,
    muted,
    musePlayback.hydrated,
    volume,
  ]);

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
    const youtubePlaybackErrorSubtitle =
      onlinePlaybackErrorCode ===
      LISTEN_YOUTUBE_VERIFICATION_REQUIRED_ERROR_CODE
        ? props.text.listen.youtubeVerificationRequired
        : onlinePlaybackErrorCode ===
            LISTEN_YOUTUBE_REGION_UNAVAILABLE_ERROR_CODE
          ? props.text.listen.youtubeRegionUnavailable
          : onlineState === "error"
            ? onlinePlaybackErrorMessage || onlinePlaybackErrorCode
            : "";
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
        live: false,
        mediaId: "",
        title: "",
        subtitle: "",
        artworkURL: "",
        playbackSource: "unknown",
        playbackSourceLabel: props.text.common.unknown,
        mode: playbackMode,
        canControl: false,
        progress: { currentTime: 0, duration: 0, bufferedTime: 0 },
        muted,
        volume,
      };
    }
    if (playbackMode === "linger") {
      return {
        state: localLoading ? "loading" : localPlaying ? "playing" : "paused",
        live: false,
        mediaId: activeLocal?.id ?? "",
        title: activeLocal?.title ?? props.text.listen.linger,
        subtitle: activeLocal?.author ?? "",
        artworkURL: activeLocal?.coverURL ?? "",
        playbackSource: "local",
        playbackSourceLabel: props.text.workspace.local,
        mode: playbackMode,
        canControl: Boolean(activeLocal),
        progress: localProgress,
        muted,
        volume,
      };
    }
    const onlineArtworkCandidates = activeOnline
      ? buildListenPosterCandidates(props.httpBaseURL, activeOnline)
      : [];
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
      live: playbackMode === "hush" || activeOnline?.group === "live",
      mediaId: activeOnline?.videoId ?? "",
      title: activeOnline?.title ?? activeModeTitle,
      subtitle:
        youtubePlaybackErrorSubtitle ||
        activeOnline?.channel ||
        activeModeTitle,
      subtitleTone: youtubePlaybackErrorSubtitle ? "danger" : "default",
      artists: youtubePlaybackErrorSubtitle
        ? []
        : normalizeListenTrackArtists(activeOnline?.artists),
      artworkURL: onlineArtworkCandidates[0] ?? "",
      artworkCandidates: onlineArtworkCandidates,
      playbackSource: playbackMode === "hush" ? "radio" : "youtube_music",
      playbackSourceLabel:
        playbackMode === "hush"
          ? props.text.workspace.radio
          : props.text.workspace.youtubeMusic,
      mode: playbackMode,
      canControl: Boolean(activeOnline),
      progress: onlineProgress,
      muted,
      volume,
      sourceURL: activeOnline?.videoId
        ? `${playbackMode === "muse" ? "https://music.youtube.com" : "https://www.youtube.com"}/watch?v=${encodeURIComponent(activeOnline.videoId)}`
        : undefined,
      favoriteActive: activeOnline
        ? onlineFavoriteByVideoId[activeOnline.videoId] === true
        : false,
      canFavorite:
        playbackMode === "muse" &&
        Boolean(activeOnline && activeOnline.group !== "live"),
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
    muted,
    onlinePlayerCommand?.command,
    onlinePlaying,
    onlineProgress.bufferedTime,
    onlineProgress.currentTime,
    onlineProgress.duration,
    onlineFavoriteByVideoId,
    onlinePlaybackErrorCode,
    onlinePlaybackErrorMessage,
    onlineState,
    playbackSessionStarted,
    playbackMode,
    props.text.listen.linger,
    props.text.listen.youtubeRegionUnavailable,
    props.text.listen.youtubeVerificationRequired,
    props.text.common.unknown,
    props.text.workspace.local,
    props.text.workspace.radio,
    props.text.workspace.youtubeMusic,
    props.httpBaseURL,
    volume,
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
    if (
      !museBrowseActive ||
      !onlineFavoriteSeedKey
    ) {
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
    listenLanguage,
    museAccountReloadToken,
    onlineFavoriteSeedKey,
    onlineFavoriteSeedVideoIds,
    museBrowseActive,
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
    playlistQueueLoadRef.current?.controller.abort();
    playlistQueueLoadRef.current = null;
    setPlaylistAppending(false);
    if (!museBrowseActive) {
      setMuseManualRefreshKind((current) =>
        current === "playlist" ? "" : current,
      );
      setPlaylistLoading(false);
      return;
    }
    if (browsePlaylistId === "") {
      setMuseManualRefreshKind((current) =>
        current === "playlist" ? "" : current,
      );
      setPlaylistTracks([]);
      setPlaylistContinuation("");
      setPlaylistDetailAuthor(""); setPlaylistDetailMetadata({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
      setPlaylistDetailTitle("");
      setPlaylistDetailDescription("");
      setPlaylistDetailThumbnailURL("");
      setPlaylistLoading(false);
      setPlaylistAppending(false);
      return;
    }
    const controller = new AbortController();
    setPlaylistLoading(true);
    setPlaylistContinuation("");
    setPlaylistDetailAuthor(""); setPlaylistDetailMetadata({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
    setPlaylistDetailTitle("");
    setPlaylistDetailDescription("");
    setPlaylistDetailThumbnailURL("");
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
          setPlaylistDetailAuthor(payload.author); setPlaylistDetailMetadata({ authorBrowseId: payload.authorBrowseId, trackCountLabel: payload.trackCountLabel, durationLabel: payload.durationLabel });
          setPlaylistDetailTitle(payload.title);
          setPlaylistDetailDescription(payload.description);
          setPlaylistDetailThumbnailURL(payload.thumbnailUrl);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setPlaylistTracks([]);
          setPlaylistContinuation("");
          setPlaylistDetailAuthor(""); setPlaylistDetailMetadata({ authorBrowseId: "", trackCountLabel: "", durationLabel: "" });
          setPlaylistDetailTitle("");
          setPlaylistDetailDescription("");
          setPlaylistDetailThumbnailURL("");
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
  }, [
    browsePlaylistId,
    listenLanguage,
    museBrowseActive,
    museAccountConnected,
    museAccountReloadToken,
    props.httpBaseURL,
  ]);

  const currentQueue =
    playbackMode === "hush"
      ? liveQueue
      : playbackMode === "muse"
        ? onlinePlaybackQueue
        : localPlaybackQueue;
  const currentIndex =
    playbackMode === "hush"
      ? resolveQueueIndex(liveQueue, liveSelectionArmed ? selectedLiveId : "")
      : playbackMode === "muse"
        ? resolveQueueIndex(onlinePlaybackQueue, selectedOnlineId)
        : resolveQueueIndex(localPlaybackQueue, selectedLocalId);
  React.useEffect(() => {
    clearForwardSkipNavigationStack();
  }, [clearForwardSkipNavigationStack, playbackMode]);

  const runOnlinePlaybackCommand = React.useCallback(
    (
      operation: () => Promise<ListenPlaybackSnapshot>,
      options: {
        clearForwardStack?: boolean;
        loading?: boolean;
        reportError?: boolean;
        syncVolume?: boolean;
      } = {},
    ) => {
      if (options.clearForwardStack !== false) {
        clearForwardSkipNavigationStack();
      }
      const epoch = onlinePlaybackActionEpochRef.current + 1;
      onlinePlaybackActionEpochRef.current = epoch;
      onlinePlaybackActionPendingRef.current = true;
      setOnlinePlaybackActionPending(true);
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
          setOnlinePlaybackActionPending(false);
        }
        return null;
      }
      const result = request
        .then((snapshot) => {
          if (onlinePlaybackActionEpochRef.current === epoch) {
            applyOnlinePlaybackSnapshot(snapshot);
          }
          return snapshot;
        })
        .catch((error: unknown) => {
          if (
            (options.reportError ?? options.loading === true) &&
            onlinePlaybackActionEpochRef.current === epoch
          ) {
            messageBus.publishToast({
              id: "listen-playback-start-error",
              intent: "danger",
              title: props.text.listen.errorStatus,
              description:
                getListenErrorMessage(error) || props.text.listen.errorStatus,
              source: "listen",
            });
          }
          return null;
        })
        .finally(() => {
          if (onlinePlaybackActionEpochRef.current === epoch) {
            onlinePlaybackActionPendingRef.current = false;
            setOnlinePlaybackActionPending(false);
          }
        });
      const completed = result.then((snapshot) => snapshot !== null);
      void result;
      return { epoch, completed, result };
    },
    [
      applyOnlinePlaybackSnapshot,
      clearForwardSkipNavigationStack,
      muted,
      props.text.listen.errorStatus,
      volume,
    ],
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

  const seekCurrentToPosition = React.useCallback(
    (seconds: number) => {
      const target = Number.isFinite(seconds) ? Math.max(0, seconds) : 0;
      if (playbackMode === "linger") {
        if (!activeLocal?.path) {
          return;
        }
        setLocalProgress((current) => ({
          ...current,
          currentTime:
            current.duration > 0 ? Math.min(target, current.duration) : target,
        }));
        setLocalProgressByPath((current) =>
          updateListenProgressMap(current, activeLocal.path, target),
        );
        setLocalPlayerCommand({
          id: Date.now(),
          command: "seek",
          startSeconds: target,
        });
        return;
      }
      if (!activeOnline || activeOnline.group === "live") {
        return;
      }
      setOnlineProgressByVideoId((current) =>
        updateListenProgressMap(current, activeOnline.videoId, target),
      );
      if (playbackMode === "muse") {
        runOnlinePlaybackCommand(() => callListenPlaybackSeek(target), {
          clearForwardStack: false,
        });
        return;
      }
      setLiveProgress((current) =>
        current.videoId === activeOnline.videoId
          ? { ...current, currentTime: target }
          : current,
      );
      setOnlinePlayerCommand({
        id: Date.now(),
        command: "seek",
        startSeconds: target,
      });
    },
    [activeLocal?.path, activeOnline, playbackMode, runOnlinePlaybackCommand],
  );

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
      const next = localPlaybackQueue[action.index];
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
    localPlaybackQueue,
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
      const next = localPlaybackQueue[action.index];
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
    localPlaybackQueue,
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
          void callListenPlaybackMergeTrackMetadata(incoming)
            .then((snapshot) => applyOnlinePlaybackSnapshot(snapshot))
            .catch(() => {});
        })
        .catch(() => undefined)
        .finally(() => {
          if (nativeTrackLookupRef.current.get(trimmedVideoId) === controller) {
            nativeTrackLookupRef.current.delete(trimmedVideoId);
          }
        });
    },
    [applyOnlinePlaybackSnapshot, listenLanguage, props.httpBaseURL],
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
        setLegacyBrowseMode("hush");
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
        setLegacyBrowseMode("hush");
        setPlaybackMode("hush");
        setSelectedLiveId(videoId);
        setLiveSelectionArmed(false);
        return;
      }

      setLegacyBrowseMode("muse");
      void callListenPlaybackObserveNativeEvent(event).catch(() => {});
    },
    [
      handleOnlineProgressChange,
      liveQueue,
      setLegacyBrowseMode,
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
      Call.ByName(`${LISTEN_LIVE_PLAYER_SERVICE}.Status`).then((status) => {
        const event = nativeStatusToPlayerEvent(
          status,
          "listen-youtube-live-player",
        );
        return isListenLiveEventForSession(event, "stream") ? event : null;
      }),
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
        const next = localPlaybackQueue[action.index];
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
    localPlaybackQueue,
    playbackMode,
    playMode,
    replayCurrent,
    selectLocalQueueTrack,
  ]);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      writeListenStorageState({
        version: 2,
        mode: activeViewMode,
        playbackMode,
        listOpen,
        playMode,
        selectedLiveId,
        selectedOnlineId,
        browsePlaylistId,
        selectedLocalId,
        localPlaybackQueueIds,
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
    localPlaybackQueueIds,
    localProgressByPath,
    activeViewMode,
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

  const pausePlayback = React.useCallback(() => {
    if (playbackMode === "linger") {
      if (activeLocal) {
        setLocalPlayerCommand({ id: Date.now(), command: "pause" });
      }
      return;
    }
    if (!activeOnline) return;
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(() => callListenPlaybackPause());
      return;
    }
    setOnlinePlayerCommand({ id: Date.now(), command: "pause" });
  }, [activeLocal, activeOnline, playbackMode, runOnlinePlaybackCommand]);

  const togglePlayback = React.useCallback(() => {
    setPlaybackSessionStarted(true);
    if (playbackMode === "linger") {
      if (!activeLocal) {
        return;
      }
      const localPlaybackActive =
        localPlaying || listenNowPlayingStatus.state === "loading";
      if (localPlaybackActive) {
        pausePlayback();
        return;
      }
      setLocalPlayerCommand({ id: Date.now(), command: "play" });
      return;
    }
    if (!activeOnline) {
      return;
    }
    const onlinePlaybackActive =
      onlinePlaying || resolveListenPlaybackActivity(onlineState).transportActive;
    if (onlinePlaybackActive) {
      pausePlayback();
      return;
    }
    if (playbackMode === "muse") {
      runOnlinePlaybackCommand(
        () =>
          onlineState === "paused"
            ? callListenPlaybackResume()
            : callListenPlaybackPlayPause(),
        { loading: true },
      );
      return;
    }
    const command = onlineState === "paused" ? "resume" : "play";
    setLiveState(command === "resume" ? "buffering" : "loading");
    setOnlinePlayerCommand({
      id: Date.now(),
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
    listenNowPlayingStatus.state,
    onlinePlaying,
    onlineState,
    pausePlayback,
    playbackMode,
    runOnlinePlaybackCommand,
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

  React.useEffect(() => {
    const command = props.controlCommand;
    if (!command || handledExternalCommandRef.current === command.id) {
      return;
    }
    handledExternalCommandRef.current = command.id;
    const playbackActive =
      playbackMode === "linger"
        ? localPlaying || listenNowPlayingStatus.state === "loading"
        : onlinePlaying ||
          resolveListenPlaybackActivity(onlineState).transportActive;
    if (command.command === "stop") {
      const clearStoppedPlayback = () => {
        setPlaybackSessionStarted(false);
        setOnlinePlayerCommand(null);
        setLocalPlaying(false);
        setLocalPlayerCommand(null);
        setLocalProgress({
          currentTime: 0,
          duration: 0,
          bufferedTime: 0,
        });
        setLivePlaying(false);
        setLiveState("idle");
        setLiveProgress({
          videoId: "",
          currentTime: 0,
          duration: 0,
          bufferedTime: 0,
        });
        resetOnlinePlaybackProjection();
      };
      if (command.backendStopped) {
        clearStoppedPlayback();
        return;
      }
      if (playbackMode === "linger") {
        console.warn("[Listen] local stop requires an active coordinator session");
        return;
      }
      const service =
        playbackMode === "hush"
          ? LISTEN_LIVE_PLAYER_SERVICE
          : LISTEN_NATIVE_PLAYER_SERVICE;
      void Call.ByName(`${service}.Reset`)
        .then(clearStoppedPlayback)
        .catch((error) => {
          console.warn("[Listen] stop playback unavailable", error);
        });
      return;
    }
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
    if (command.command === "shuffle") {
      updatePlayMode(playMode === "shuffle" ? "order" : "shuffle");
      return;
    }
    if (command.command === "repeat") {
      updatePlayMode(playMode === "repeat" ? "order" : "repeat");
      return;
    }
    if (command.command === "favorite") {
      toggleOnlineFavorite();
      return;
    }
    if (command.command === "toggle-mute") {
      toggleMute();
      return;
    }
    if (
      command.command === "set-volume" &&
      typeof command.value === "number" &&
      Number.isFinite(command.value)
    ) {
      handleVolumeChange(command.value);
      return;
    }
    if (
      command.command === "seek" &&
      typeof command.value === "number" &&
      Number.isFinite(command.value)
    ) {
      seekCurrentToPosition(command.value);
      return;
    }
    if (command.command === "pause") {
      pausePlayback();
      return;
    }
    if (command.command === "play" && !playbackActive) {
      togglePlayback();
    }
  }, [
    handleVolumeChange,
    localPlaying,
    listenNowPlayingStatus.state,
    onlinePlaying,
    onlineState,
    playbackMode,
    playMode,
    playNext,
    playPrevious,
    pausePlayback,
    props.controlCommand,
    resetOnlinePlaybackProjection,
    seekCurrentToPosition,
    toggleMute,
    togglePlayback,
    toggleOnlineFavorite,
    updatePlayMode,
  ]);

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
            language: listenLanguage,
            kind: "radio",
          }),
        { loading: true },
      );
    },
    [listenLanguage, runOnlinePlaybackCommand],
  );

  const playOnlineRadioSeed = React.useCallback(
    (item: ListenOnlineItem) => {
      runOnlinePlaybackCommand(
        () =>
          callListenPlaybackPlayQueue({
            tracks: [item],
            startingAt: 0,
            title: props.text.listen.groupRadio,
            language: listenLanguage,
            kind: "radio",
          }),
        { loading: true },
      );
    },
    [listenLanguage, props.text.listen.groupRadio, runOnlinePlaybackCommand],
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
    if (
      !museBrowseActive ||
      !continuation ||
      searchAppending ||
      searchLoading ||
      normalizedQuery.length < 2
    ) {
      return;
    }
    const request = beginListenPaginationRequest(
      paginationRequestsRef.current,
      "search",
      searchPaginationContext,
      continuation,
    );
    if (!request) {
      return;
    }
    setSearchAppending(true);
    void fetchListenSearch(
      props.httpBaseURL,
      query,
      request.controller.signal,
      listenLanguage,
      continuation,
    )
      .then((payload) => {
        if (
          request.controller.signal.aborted ||
          !isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.search,
          )
        ) {
          return;
        }
        setSearchItems((current) =>
          dedupeOnlineItems([...current, ...payload.items]),
        );
        setSearchContinuation(
          resolveListenNextContinuation(
            continuation,
            payload.continuation,
          ),
        );
      })
      .catch(() => {
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.search,
          )
        ) {
          setSearchContinuation("");
        }
      })
      .finally(() => {
        finishListenPaginationRequest(
          paginationRequestsRef.current,
          request,
        );
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.search,
          )
        ) {
          setSearchAppending(false);
        }
      });
  }, [
    listenLanguage,
    museBrowseActive,
    normalizedQuery.length,
    props.httpBaseURL,
    query,
    searchAppending,
    searchContinuation,
    searchLoading,
    searchPaginationContext,
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
      runOnlinePlaybackCommand(
        () =>
          queueIndex >= 0
            ? callListenPlaybackPlayFromQueue(queueIndex, item, listenLanguage)
            : callListenPlaybackPlayTrack(item, { language: listenLanguage }),
        { loading: true },
      );
    },
    [
      onlinePlaybackQueue,
      listenLanguage,
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
      invalidateListenArtistRequests(
        artistRequestsRef.current,
        createListenArtistIdentity({ id: artistBrowseId, name: artistName }),
      );
      setArtistActionBusy("");
      setListOpen(true);
      setLegacyBrowseMode("muse");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setQuery("");
      setArtistBrowsePage({
        id: artistBrowseId,
        name: artistName,
        title: artistName,
        subtitle: "",
        description: "",
        thumbnailUrl: artist?.thumbnailUrl,
        heroThumbnailUrl: artist?.thumbnailUrl,
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
    [setLegacyBrowseMode],
  );

  const openOnlineArtistFromPlayer = React.useCallback(
    (item: ListenOnlineItem) => {
      openListenArtistFromPlayerSurface({
        workspaceActive: props.active,
        workspaceLayout: props.workspaceLayout === true,
        openPlaybackSource: props.onOpenPlaybackSource,
        openArtist: () => openOnlineArtistBrowse(item),
        schedule: (openArtist) => window.requestAnimationFrame(openArtist),
      });
    },
    [
      openOnlineArtistBrowse,
      props.active,
      props.onOpenPlaybackSource,
      props.workspaceLayout,
    ],
  );

  const openSearchArtistBrowse = React.useCallback(
    (item: ListenArtistItem) => {
      const artistName = item.name.trim();
      const artistId = item.browseId.trim();
      if (!artistName && !artistId) {
        return;
      }
      invalidateListenArtistRequests(
        artistRequestsRef.current,
        createListenArtistIdentity({ id: artistId, name: artistName }),
      );
      setArtistActionBusy("");
      setLegacyBrowseMode("muse");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setQuery("");
      setArtistBrowsePage({
        id: artistId,
        name: artistName,
        title: artistName || artistId,
        subtitle: item.subtitle,
        description: "",
        thumbnailUrl: item.thumbnailUrl,
        heroThumbnailUrl: item.thumbnailUrl,
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
    [setLegacyBrowseMode],
  );

  const closeArtistBrowse = React.useCallback(() => {
    invalidateListenArtistRequests(artistRequestsRef.current, "");
    setArtistActionBusy("");
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
            language: listenLanguage,
            kind: "radio",
          }),
        { loading: true },
      );
    },
    [artistBrowsePage, listenLanguage, runOnlinePlaybackCommand],
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
          language: listenLanguage,
          kind: "radio",
        }),
      { loading: true },
    );
  }, [artistBrowsePage, listenLanguage, runOnlinePlaybackCommand]);

  const loadMoreArtist = React.useCallback(() => {
    const page = artistBrowsePage;
    const continuation = page?.continuation.trim() ?? "";
    if (
      !museBrowseActive ||
      !page ||
      !continuation ||
      page.appending ||
      page.loading
    ) {
      return;
    }
    const request = beginListenPaginationRequest(
      paginationRequestsRef.current,
      "artist",
      artistPaginationContext,
      continuation,
    );
    if (!request) {
      return;
    }
    setArtistBrowsePage((current) =>
      current && isSameListenArtistBrowseIdentity(current, page)
        ? { ...current, appending: true }
        : current,
    );
    void fetchListenArtist(
      props.httpBaseURL,
      { id: page.id, name: page.name },
      request.controller.signal,
      { continuation, language: listenLanguage },
    )
      .then((payload) => {
        if (
          request.controller.signal.aborted ||
          !isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.artist,
          )
        ) {
          return;
        }
        setArtistBrowsePage((current) =>
          current && isSameListenArtistBrowseIdentity(current, page)
            ? {
                ...current,
                items: dedupeOnlineItems([...current.items, ...payload.items]),
                shelves: dedupeLibraryShelves([
                  ...current.shelves,
                  ...payload.shelves,
                ]),
                continuation: resolveListenNextContinuation(
                  continuation,
                  payload.continuation,
                ),
                appending: false,
              }
            : current,
        );
      })
      .catch(() => {
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.artist,
          )
        ) {
          setArtistBrowsePage((current) =>
            current && isSameListenArtistBrowseIdentity(current, page)
              ? { ...current, continuation: "", appending: false }
              : current,
          );
        }
      })
      .finally(() => {
        finishListenPaginationRequest(
          paginationRequestsRef.current,
          request,
        );
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.artist,
          )
        ) {
          setArtistBrowsePage((current) =>
            current && isSameListenArtistBrowseIdentity(current, page)
              ? { ...current, appending: false }
              : current,
          );
        }
      });
  }, [artistBrowsePage, artistPaginationContext, listenLanguage, museBrowseActive, props.httpBaseURL]);

  const loadArtistShelfTracks = React.useCallback(
    async (shelf: ListenLibraryShelf) => {
      if (!museBrowseActive) {
        return shelf.tracks;
      }
      const page = artistBrowsePage;
      if (!page) {
        return shelf.tracks;
      }
      const artistIdentity = createListenArtistIdentity(page);
      const request = beginListenArtistRequest(
        artistRequestsRef.current,
        "shelf",
        artistIdentity,
      );
      const assertCurrent = () => {
        if (!isListenArtistRequestCurrent(artistRequestsRef.current, request)) {
          throw new DOMException("", "AbortError");
        }
      };
      try {
        const fetchedTracks: ListenOnlineItem[] = [];
        const fetchedShelves: ListenLibraryShelf[] = [];
        const seenContinuations = new Set<string>();
        let nextContinuation = shelf.continuation.trim();
        if (shelf.browseId.trim()) {
          const payload = await fetchListenArtist(
            props.httpBaseURL,
            { id: page.id, name: page.name },
            request.controller.signal,
            {
              browseId: shelf.browseId,
              params: shelf.params,
              language: listenLanguage,
            },
          );
          assertCurrent();
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
            request.controller.signal,
            { continuation: nextContinuation, language: listenLanguage },
          );
          assertCurrent();
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
        assertCurrent();
        if (fetchedTrackItems.length === 0) {
          return shelf.tracks;
        }
        const tracks = dedupeOnlineItems([
          ...shelf.tracks,
          ...fetchedTrackItems,
        ]);
        setArtistBrowsePage((current) => {
          if (
            !isListenArtistRequestCurrent(artistRequestsRef.current, request) ||
            !current ||
            !isSameListenArtistBrowseIdentity(current, page)
          ) {
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
        assertCurrent();
        return tracks;
      } finally {
        finishListenArtistRequest(artistRequestsRef.current, request);
      }
    },
    [artistBrowsePage, listenLanguage, museBrowseActive, props.httpBaseURL],
  );

  const playArtistMix = React.useCallback(() => {
    const page = artistBrowsePage;
    const playlistId = page?.mixPlaylistId.trim() ?? "";
    if (!page || !playlistId) {
      return;
    }
    const request = beginListenArtistRequest(
      artistRequestsRef.current,
      "action",
      createListenArtistIdentity(page),
    );
    setArtistActionBusy("mix");
    void fetchListenPlaylistQueue(
      props.httpBaseURL,
      playlistId,
      request.controller.signal,
      listenLanguage,
    )
      .then((items) => {
        if (!isListenArtistRequestCurrent(artistRequestsRef.current, request)) {
          return;
        }
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
              language: listenLanguage,
              kind: "mix",
              playlistId,
              startVideoId: page.mixVideoId,
            }),
          { loading: true },
        );
      })
      .catch(() => {})
      .finally(() => {
        if (isListenArtistRequestCurrent(artistRequestsRef.current, request)) {
          setArtistActionBusy("");
        }
        finishListenArtistRequest(artistRequestsRef.current, request);
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
    const request = beginListenArtistRequest(
      artistRequestsRef.current,
      "action",
      createListenArtistIdentity(page),
    );
    setArtistActionBusy("subscribe");
    setArtistBrowsePage((current) =>
      current && isSameListenArtistBrowseIdentity(current, page)
        ? { ...current, isSubscribed: nextSubscribed }
        : current,
    );
    void updateListenArtistSubscription(
      props.httpBaseURL,
      channelId,
      nextSubscribed,
      request.controller.signal,
    )
      .then((subscribed) => {
        if (isListenArtistRequestCurrent(artistRequestsRef.current, request)) {
          setArtistBrowsePage((current) =>
            current && isSameListenArtistBrowseIdentity(current, page)
              ? { ...current, isSubscribed: subscribed }
              : current,
          );
        }
      })
      .catch(() => {
        if (isListenArtistRequestCurrent(artistRequestsRef.current, request)) {
          setArtistBrowsePage((current) =>
            current && isSameListenArtistBrowseIdentity(current, page)
              ? { ...current, isSubscribed: page.isSubscribed }
              : current,
          );
        }
      })
      .finally(() => {
        if (isListenArtistRequestCurrent(artistRequestsRef.current, request)) {
          setArtistActionBusy("");
        }
        finishListenArtistRequest(artistRequestsRef.current, request);
      });
  }, [artistBrowsePage, props.httpBaseURL]);

  const loadCompleteSelectedPlaylistTracks = React.useCallback(
    (): Promise<ListenOnlineItem[]> => {
      const playlistId = browsePlaylistId.trim();
      if (!playlistId || playlistTracks.length === 0) {
        return Promise.resolve([]);
      }
      const continuation = playlistContinuation.trim();
      if (!continuation) {
        return Promise.resolve(playlistTracks);
      }
      const activeLoad = playlistQueueLoadRef.current;
      if (activeLoad?.context === playlistPaginationContext) {
        return activeLoad.promise;
      }
      activeLoad?.controller.abort();
      playlistQueueLoadRef.current = null;

      paginationRequestsRef.current.forEach((request, key) => {
        if (
          request.kind === "playlist" &&
          request.context === playlistPaginationContext
        ) {
          request.controller.abort();
          paginationRequestsRef.current.delete(key);
        }
      });

      const controller = new AbortController();
      const expectedContext = playlistPaginationContext;
      const seedTracks = playlistTracks;
      const load: {
        context: string;
        controller: AbortController;
        promise: Promise<ListenOnlineItem[]>;
      } = {
        context: expectedContext,
        controller,
        promise: Promise.resolve([]),
      };
      setPlaylistAppending(true);
      load.promise = (async () => {
        try {
          const result = await fetchCompleteListenPlaylistQueue(
            props.httpBaseURL,
            playlistId,
            controller.signal,
            {
              continuation,
              initialItems: seedTracks,
              language: listenLanguage,
            },
          );
          if (
            controller.signal.aborted ||
            !isListenPaginationContextCurrent(
              expectedContext,
              paginationContextsRef.current.playlist,
            )
          ) {
            return [];
          }
          setPlaylistTracks(result.items);
          setPlaylistContinuation(result.continuation);
          if (result.continuation) {
            messageBus.publishToast({
              intent: "danger",
              title: props.text.library.progressDetail.operationFailed,
              description:
                playlistDetailTitle || props.text.listen.groupPlaylist,
            });
            return [];
          }
          return result.items;
        } catch {
          if (
            controller.signal.aborted ||
            !isListenPaginationContextCurrent(
              expectedContext,
              paginationContextsRef.current.playlist,
            )
          ) {
            return [];
          }
          return seedTracks;
        } finally {
          if (playlistQueueLoadRef.current === load) {
            playlistQueueLoadRef.current = null;
            setPlaylistAppending(false);
          }
        }
      })();
      playlistQueueLoadRef.current = load;
      return load.promise;
    },
    [
      browsePlaylistId,
      listenLanguage,
      playlistContinuation,
      playlistDetailTitle,
      playlistPaginationContext,
      playlistTracks,
      props.httpBaseURL,
      props.text.library.progressDetail.operationFailed,
      props.text.listen.groupPlaylist,
    ],
  );

  const playPlaylistFromIndex = React.useCallback(
    (index: number) => {
      const playlist = selectedPlaylist;
      if (!playlist) {
        return;
      }
      const fallbackArtist = resolveListenPlaylistPlaybackFallbackArtist(
        playlist,
        playlistDetailAuthor,
      );
      startListenPlaylistPlaybackFromIndex({
        initialItems: applyListenPlaylistPlaybackFallback(
          playlistTracks,
          fallbackArtist,
        ),
        startingAt: index,
        hasContinuation: Boolean(playlistContinuation.trim()),
        title: playlistDetailTitle || playlist.title,
        language: listenLanguage,
        playlistId: playlist.playlistId,
        loadComplete: () =>
          loadCompleteSelectedPlaylistTracks().then((tracks) =>
            applyListenPlaylistPlaybackFallback(tracks, fallbackArtist),
          ),
        isCurrent: (epoch) =>
          onlinePlaybackActionEpochRef.current === epoch,
        runCommand: runOnlinePlaybackCommand,
      });
    },
    [
      loadCompleteSelectedPlaylistTracks,
      playlistDetailAuthor,
      playlistDetailTitle,
      playlistContinuation,
      playlistTracks,
      listenLanguage,
      runOnlinePlaybackCommand,
      selectedPlaylist,
    ],
  );

  const shufflePlaylist = React.useCallback(() => {
    const playlist = selectedPlaylist;
    if (!playlist || playlistTracks.length === 0) {
      return;
    }
    const fallbackArtist = resolveListenPlaylistPlaybackFallbackArtist(
      playlist,
      playlistDetailAuthor,
    );
    const visibleItems = shuffleListenOnlineItems(
      applyListenPlaylistPlaybackFallback(playlistTracks, fallbackArtist),
    );
    const queueIdentity = createListenPlaybackQueueIdentity();
    startListenPlaylistPlayback({
      initialItems: visibleItems,
      hasContinuation: Boolean(playlistContinuation.trim()),
      playInitial: (items) => {
        const command = runOnlinePlaybackCommand(
          () =>
            callListenPlaybackPlayQueue({
              tracks: items,
              startingAt: 0,
              title: playlistDetailTitle || playlist.title,
              language: listenLanguage,
              kind: "playlist",
              playlistId: playlist.playlistId,
              queueIdentity,
            }),
          { loading: true },
        );
        return command ? { ...command, queueIdentity } : null;
      },
      loadComplete: () =>
        loadCompleteSelectedPlaylistTracks().then((tracks) =>
          shuffleListenOnlineItems(
            applyListenPlaylistPlaybackFallback(tracks, fallbackArtist),
          ),
        ),
      isCurrent: (epoch) => onlinePlaybackActionEpochRef.current === epoch,
      appendRemaining: (items, expectedQueueIdentity) => {
        runOnlinePlaybackCommand(
          () =>
            callListenPlaybackAppendToQueue(items, {
              expectedQueueIdentity,
            }),
          {
            clearForwardStack: false,
            reportError: true,
            syncVolume: false,
          },
        );
      },
    });
  }, [
    loadCompleteSelectedPlaylistTracks,
    playlistDetailAuthor,
    playlistDetailTitle,
    playlistContinuation,
    playlistTracks,
    listenLanguage,
    runOnlinePlaybackCommand,
    selectedPlaylist,
  ]);

  const queueSelectedPlaylistTracks = React.useCallback(
    (placement: "next" | "end") => {
      const playlist = selectedPlaylist;
      if (!playlist || playlistTracks.length === 0) {
        return;
      }
      const fallbackArtist = resolveListenPlaylistPlaybackFallbackArtist(
        playlist,
        playlistDetailAuthor,
      );
      const visibleItems = applyListenPlaylistPlaybackFallback(
        playlistTracks,
        fallbackArtist,
      );
      const action = resolveListenPlaylistQueueAction({
        items: visibleItems,
        hasActiveQueue:
          playbackModeRef.current === "muse" &&
          onlineQueueStateRef.current.items.length > 0,
        placement,
      });
      if (!action) {
        return;
      }
      const queueIdentity =
        action.kind === "start" ? createListenPlaybackQueueIdentity() : "";
      startListenPlaylistQueueAction({
        initialItems: action.items,
        hasContinuation: Boolean(playlistContinuation.trim()),
        enqueueInitial: (items) => {
          const command = runOnlinePlaybackCommand(
            () => {
              if (action.kind === "start") {
                return callListenPlaybackPlayQueue({
                  tracks: items,
                  startingAt: 0,
                  title: playlistDetailTitle || playlist.title,
                  language: listenLanguage,
                  kind: "playlist",
                  playlistId: playlist.playlistId,
                  queueIdentity,
                });
              }
              return action.kind === "insert-next"
                ? callListenPlaybackInsertNextInQueue(items)
                : callListenPlaybackAppendToQueue(items);
            },
            action.kind === "start"
              ? { loading: true }
              : { reportError: true, syncVolume: false },
          );
          return command
            ? { epoch: command.epoch, result: command.result }
            : null;
        },
        loadComplete: () =>
          loadCompleteSelectedPlaylistTracks().then((tracks) =>
            applyListenPlaylistPlaybackFallback(tracks, fallbackArtist),
          ),
        isCurrent: (epoch) => onlinePlaybackActionEpochRef.current === epoch,
        enqueueRemaining: (items, initialSnapshot) => {
          runOnlinePlaybackCommand(
            () =>
              placeListenPlaylistQueueContinuation({
                kind: action.kind,
                initialItemCount: action.items.length,
                remainingItems: items,
                initialSnapshot,
              }),
            {
              clearForwardStack: false,
              reportError: true,
              syncVolume: false,
            },
          );
        },
      });
    },
    [
      loadCompleteSelectedPlaylistTracks,
      playlistDetailAuthor,
      playlistDetailTitle,
      playlistContinuation,
      playlistTracks,
      listenLanguage,
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
    if (
      !museBrowseActive ||
      !playlistId ||
      !continuation ||
      playlistAppending ||
      playlistLoading ||
      playlistQueueLoadRef.current
    ) {
      return;
    }
    const request = beginListenPaginationRequest(
      paginationRequestsRef.current,
      "playlist",
      playlistPaginationContext,
      continuation,
    );
    if (!request) {
      return;
    }
    setPlaylistAppending(true);
    void fetchListenPlaylistPage(
      props.httpBaseURL,
      playlistId,
      request.controller.signal,
      continuation,
      listenLanguage,
    )
      .then((payload) => {
        if (
          request.controller.signal.aborted ||
          !isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.playlist,
          )
        ) {
          return;
        }
        setPlaylistTracks((current) =>
          dedupeOnlineItems([...current, ...payload.items]),
        );
        setPlaylistContinuation(
          resolveListenNextContinuation(
            continuation,
            payload.continuation,
          ),
        );
      })
      .catch(() => {
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.playlist,
          )
        ) {
          setPlaylistContinuation("");
        }
      })
      .finally(() => {
        finishListenPaginationRequest(
          paginationRequestsRef.current,
          request,
        );
        if (
          !request.controller.signal.aborted &&
          isListenPaginationContextCurrent(
            request.context,
            paginationContextsRef.current.playlist,
          )
        ) {
          setPlaylistAppending(false);
        }
      });
  }, [
    browsePlaylistId,
    listenLanguage,
    museBrowseActive,
    playlistAppending,
    playlistContinuation,
    playlistLoading,
    playlistPaginationContext,
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
    if (activeViewMode === "hush") {
      const first = curatedLiveItems[0];
      if (first) {
        activateLiveSelection(first);
      }
      return;
    }
    if (activeViewMode === "muse") {
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
    activeViewMode,
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
    activeViewMode === "hush" && normalizedQuery && curatedLiveItems.length === 0
      ? props.text.listen.searchEmpty
      : "";
  const onlineSearchNotice =
    activeViewMode === "muse" && !artistBrowsePage && normalizedQuery
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
    activeViewMode === "muse" ? sidebarView : "browse";
  const showArtistDetail =
    activeViewMode === "muse" &&
    effectiveSidebarView === "browse" &&
    artistBrowsePage !== null &&
    browsePlaylistId === "";
  const showPlaylistDetail =
    activeViewMode === "muse" &&
    effectiveSidebarView === "browse" &&
    browsePlaylistId !== "";
  const searchPlaceholder =
    activeViewMode === "hush"
      ? props.text.listen.searchLive
      : activeViewMode === "muse"
        ? props.text.listen.searchOnline
        : props.text.listen.searchLocal;

  return (
    <>
      <ListenPageView
        page={props}
        state={{ isWindows, isMac, listOpen, query, searchPlaceholder, mode: activeViewMode, playbackMode, sidebarView, effectiveSidebarView, onlineBrowseSource, onlineBrowseDetail, liveGroups, selectedLiveGroupId, liveStatusByVideoId, liveCatalogLoading, liveCatalogError, liveCatalogMessage, liveUserCatalog, liveUserCatalogLoading, liveUserCatalogSaving, liveUserCatalogError, curatedLiveItems, liveSelectionArmed, selectedLiveId, filteredOnlineQueueItems, onlineQueueTitle, onlineQueueCanUndo, onlineQueueCanRedo, selectedOnlineId, filteredLocalTracks, selectedLocalId, localPlaying, liveSearchNotice, showArtistDetail, artistBrowsePage, artistActionBusy, filteredArtistShelves, browsePlaylistId, savedPlaylistIds, playlistMutationAction, playlistMutationPlaylistId, filteredArtistTracks, showPlaylistDetail, selectedPlaylist, playlistLoading, playlistAppending, playlistDetailAuthor, playlistDetailAuthorBrowseId: playlistDetailMetadata.authorBrowseId, playlistDetailTrackCountLabel: playlistDetailMetadata.trackCountLabel, playlistDetailDurationLabel: playlistDetailMetadata.durationLabel, playlistDetailTitle, playlistDetailDescription, playlistDetailThumbnailURL, playlistTracks, playlistContinuation, normalizedQuery, libraryLoading, libraryAppending, libraryError, libraryErrorCode, libraryErrorRetryable, libraryRequestReady, librarySettled, searchLoading, searchAppending, searchItems, searchArtists, searchPlaylists, searchContinuation, libraryArtists, displayedLibraryPlaylists, showLibraryPlaylistGroup, homeShelves, libraryContinuation, onlineSearchNotice, localTracks, localPlaybackQueue, localQueueCanUndo, localQueueCanRedo, localTracksLoading: localTrackIndex.loading, localTracksRefreshing: localTrackIndex.refreshing, localTracksClearingMissing: localTrackIndex.clearingMissing, localTracksError: localTrackIndex.error, activeOnline, selectedLocal: activeLocal, onlinePlayerCommand, localPlayerCommand, onlineQueueItems: playbackMode === "hush" ? liveQueue : onlinePlaybackQueue, onlinePlaying, onlinePlaybackArmed, onlinePlaybackActionPending, selectedLocalResumeTime, activeOnlineResumeTime, onlineProgress, onlineState, onlinePlaybackErrorCode, onlinePlaybackErrorMessage, onlineObservedPlaybackAudioQuality, activeOnlineFavorite, activeOnlineFavoriteBusy, localProgress, muted, volume, playMode, museConnectBusy: youtubeConnectBusy, museAccountName, museAccountAvatarURL, museAccountConnected, museAccountBusy, museManualRefreshKind }}
        actions={{ setListOpen, setQuery, selectFirstResult, setMode: setLegacyBrowseMode, setSidebarView, reloadLiveCatalog, saveLiveUserCatalog, reloadLibrary: () => setLibraryReloadToken((current) => current + 1), changeOnlineBrowseSource, openOnlineBrowseCategory, closeOnlineBrowseDetail, loadMoreLibrary, activateLiveSelection, selectOnlineQueueTrack, selectLocalQueueTrack, clearLocalQueue, removeLocalQueueItem, moveLocalQueueItem, undoLocalQueueEdit, redoLocalQueueEdit, retryLocalTracks: localTrackIndex.retry, playLocalBrowseTrack, setSelectedLocalId, setLocalPlayerCommand, closeArtistBrowse, playArtistFromIndex, shuffleArtist, loadMoreArtist, loadArtistShelfTracks, playArtistMix, toggleArtistSubscription, openPlaylistBrowse, updatePlaylistLibrary, setBrowsePlaylistId, playPlaylistFromIndex, shufflePlaylist, playPlaylistNext, addPlaylistToQueue, loadMorePlaylist, playOnlineShelfTrack, playOnlineShelfAll, shuffleOnlineShelf, playOnlineSearchTrack, playOnlineSearchResults, shuffleOnlineSearchResults, loadMoreSearch, openSearchArtistBrowse, clearOnlineQueue, removeOnlineQueueItem, moveOnlineQueueItem, undoOnlineQueueEdit, redoOnlineQueueEdit, refreshLocalTracks: localTrackIndex.refresh, openRepairMissingLocalTracks: () => setLocalRelinkDialogOpen(true), handlePlaybackEnded, setOnlinePlaying: setLivePlaying, setOnlineState: setLiveState, setOnlinePlaybackErrorCode, setOnlinePlaybackErrorMessage, handleOnlineProgressChange, handleOnlineNativeTrackChange, setLocalPlaying, handleLocalProgressChange, setPlaybackSessionStarted, connectYouTube, refreshMusePage, signOutMuseAccount, playPrevious, playNext, togglePlayMode, setPlayMode: setPlayModeFromView, togglePlayback, toggleMute, handleVolumeChange, toggleOnlineFavorite, openOnlineArtistBrowse: openOnlineArtistFromPlayer, openSelectedLocalDirectory }}
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
