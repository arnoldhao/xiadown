import { Call,System } from "@wailsio/runtime";
import * as React from "react";

import { useOpenLibraryPath } from "@/shared/query/library";
import { REALTIME_TOPICS,registerTopic } from "@/shared/realtime";

import { fetchDreamFMArtist,fetchDreamFMLibrary,fetchDreamFMLiveCatalog,fetchDreamFMLiveStatuses,fetchDreamFMPlaylistPage,fetchDreamFMPlaylistQueue,fetchDreamFMRadio,fetchDreamFMSearch,fetchDreamFMTrackFavorite,fetchDreamFMTrackFavoriteStatuses,fetchDreamFMTrackInfo,getDreamFMErrorCode,getDreamFMErrorMessage,updateDreamFMArtistSubscription,updateDreamFMPlaylistLibrary,updateDreamFMTrackFavorite } from "@/app/main/dreamfm/api";
import { DREAM_FM_LIKED_SONGS_SHELF_ID,DREAM_FM_LIVE_PLAYER_SERVICE,DREAM_FM_NATIVE_PLAYER_SERVICE } from "@/app/main/dreamfm/catalog";
import { clampVolume,matchesQuery,normalizeSearch,resolveAdjacentIndex,resolveDreamFMLiveSelectionId,resolveQueueIndex,resolveRandomIndex,useDreamFMLocalTracks } from "@/app/main/dreamfm/local-library";
import { DreamFMPageView } from "@/app/main/dreamfm/PageView";
import { buildDreamFMHighQualityThumbnailURL,buildDreamFMImageCacheURL,buildYouTubePosterURL,createDefaultDreamFMOnlineQueueState,createInitialDreamFMOnlineQueueState,dedupeLibraryShelves,dedupeOnlineItems,dedupePlaylistItems,readDreamFMStorageState,updateDreamFMProgressMap,writeDreamFMStorageState } from "@/app/main/dreamfm/storage";
import type { DreamFMArtistBrowseState,DreamFMArtistItem,DreamFMCategoryItem,DreamFMLibraryShelf,DreamFMLiveGroup,DreamFMLiveStatus,DreamFMMode,DreamFMNativePlayerEvent,DreamFMNowPlayingStatus,DreamFMOnlineBrowseDetail,DreamFMOnlineBrowseSource,DreamFMOnlineItem,DreamFMOnlineQueueState,DreamFMPageProps,DreamFMPlayMode,DreamFMPlaybackProgressState,DreamFMPlayerCommand,DreamFMPlaylistItem,DreamFMPlaylistLibraryAction,DreamFMRemotePlaybackState,DreamFMSidebarView } from "@/app/main/dreamfm/types";
export { DreamFMLocalPreviewPlayer } from "@/app/main/dreamfm/LocalPreviewPlayer";
export type { DreamFMExternalCommand,DreamFMLocalPreviewTrack,DreamFMMode,DreamFMNowPlayingStatus } from "@/app/main/dreamfm/types";

const DREAM_FM_UNKNOWN_ARTIST = "Unknown Artist";

function isMissingDreamFMArtist(value: string) {
  const artist = value.trim();
  return (
    !artist ||
    artist === "YouTube Music" ||
    artist === DREAM_FM_UNKNOWN_ARTIST
  );
}

function mergeDreamFMNativeTrackItem(
  incoming: DreamFMOnlineItem,
  current: DreamFMOnlineItem,
): DreamFMOnlineItem {
  const videoId = current.videoId || incoming.videoId;
  const incomingTitle = incoming.title.trim();
  const currentTitle = current.title.trim();
  const incomingChannel = incoming.channel.trim();
  const currentChannel = current.channel.trim();
  return {
    ...current,
    videoId,
    title:
      incomingTitle && incomingTitle !== videoId
        ? incoming.title
        : currentTitle || videoId,
    channel:
      !isMissingDreamFMArtist(incomingChannel)
        ? incoming.channel
        : !isMissingDreamFMArtist(currentChannel)
          ? current.channel
          : currentChannel || incoming.channel || "YouTube Music",
    artistBrowseId: incoming.artistBrowseId || current.artistBrowseId,
    description: incoming.description || current.description,
    durationLabel: incoming.durationLabel || current.durationLabel,
    playCountLabel: incoming.playCountLabel || current.playCountLabel,
    thumbnailUrl: incoming.thumbnailUrl || current.thumbnailUrl,
  };
}

const DREAM_FM_REMOTE_PLAYBACK_STATES: DreamFMRemotePlaybackState[] = [
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
  fallback: DreamFMRemotePlaybackState,
): DreamFMRemotePlaybackState {
  const state = stringFromNativeStatus(value) as DreamFMRemotePlaybackState;
  return DREAM_FM_REMOTE_PLAYBACK_STATES.includes(state) ? state : fallback;
}

function createNativeOnlineItem(params: {
  videoId: string;
  title?: string;
  artist?: string;
  thumbnailUrl?: string;
}): DreamFMOnlineItem {
  const videoId = params.videoId.trim();
  const title = params.title?.trim() || videoId;
  const artist = params.artist?.trim() || "YouTube Music";
  const thumbnailUrl = params.thumbnailUrl?.trim() || "";
  return {
    id: `ytmusic-native-${videoId}`,
    group: "playlist",
    videoId,
    title,
    channel: artist,
    description: "",
    durationLabel: "",
    thumbnailUrl: thumbnailUrl,
  };
}

function cleanDreamFMPlaylistPlaybackArtist(value: string) {
  let artist = value.trim();
  if (artist === "Album") {
    return DREAM_FM_UNKNOWN_ARTIST;
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
  return artist;
}

function isDreamFMPodcastPlaylistId(value: string) {
  return value.trim().startsWith("MPSPP");
}

function applyDreamFMPlaylistPlaybackFallback(
  items: DreamFMOnlineItem[],
  fallbackArtist: string,
) {
  const cleanedFallback = cleanDreamFMPlaylistPlaybackArtist(fallbackArtist);
  return items.map((item) => {
    let channel = item.channel.trim();
    if (channel === "Album") {
      channel = "";
    } else if (channel.startsWith("Album, ")) {
      channel = channel.slice(7).trim();
    }
    if (!channel && cleanedFallback) {
      channel = cleanedFallback;
    }
    return channel === item.channel ? item : { ...item, channel };
  });
}

function nativeStatusToPlayerEvent(
  value: unknown,
  source = "dreamfm-youtube-music-player",
): DreamFMNativePlayerEvent | null {
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
    videoAvailable: record.videoAvailable === true,
    videoAvailabilityKnown: record.videoAvailabilityKnown === true,
    currentTime: secondsFromNativeStatus(record.currentTime),
    duration: secondsFromNativeStatus(record.duration),
    bufferedTime: secondsFromNativeStatus(record.bufferedTime),
    advertising: record.advertising === true,
    adLabel: stringFromNativeStatus(record.adLabel),
    adSkippable: record.adSkippable === true,
    adSkipLabel: stringFromNativeStatus(record.adSkipLabel),
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
  groups: Array<readonly DreamFMOnlineItem[]>,
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

function resolveDreamFMLiveCatalogEventKey(payload: unknown) {
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

function resolveDreamFMShelfQueueTitle(
  shelf: DreamFMLibraryShelf,
  text: DreamFMPageProps["text"],
  fallback: string,
) {
  if (shelf.id === DREAM_FM_LIKED_SONGS_SHELF_ID) {
    return text.dreamFm.likedMusic;
  }
  return shelf.title.trim() || fallback;
}

function shuffleDreamFMOnlineItems(items: DreamFMOnlineItem[]) {
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

export function DreamFMPage(props: DreamFMPageProps) {
  const isWindows = System.IsWindows();
  const isMac = System.IsMac();
  const initialPersistedState = React.useMemo(
    () => readDreamFMStorageState(),
    [],
  );
  const initialOnlineQueueState = React.useMemo(
    () => {
      const queue = createInitialDreamFMOnlineQueueState(initialPersistedState);
      return queue.kind === "playlist" &&
        isDreamFMPodcastPlaylistId(queue.playlistId)
        ? createDefaultDreamFMOnlineQueueState()
        : queue;
    },
    [initialPersistedState],
  );
  const openLibraryPath = useOpenLibraryPath();
  const [mode, setMode] = React.useState<DreamFMMode>(
    initialPersistedState.mode,
  );
  const [sidebarView, setSidebarView] =
    React.useState<DreamFMSidebarView>("browse");
  const [onlineBrowseSource, setOnlineBrowseSource] =
    React.useState<DreamFMOnlineBrowseSource>("home");
  const [onlineBrowseDetail, setOnlineBrowseDetail] =
    React.useState<DreamFMOnlineBrowseDetail | null>(null);
  const [query, setQuery] = React.useState("");
  const [listOpen, setListOpen] = React.useState(
    initialPersistedState.listOpen,
  );
  const [selectedLiveId, setSelectedLiveId] = React.useState(
    initialPersistedState.selectedLiveId,
  );
  const [liveSelectionArmed, setLiveSelectionArmed] = React.useState(false);
  const [liveGroups, setLiveGroups] = React.useState<DreamFMLiveGroup[]>([]);
  const [selectedLiveGroupId, setSelectedLiveGroupId] = React.useState("");
  const [liveCatalogLoading, setLiveCatalogLoading] = React.useState(false);
  const [liveCatalogError, setLiveCatalogError] = React.useState(false);
  const [liveCatalogMessage, setLiveCatalogMessage] = React.useState("");
  const [liveCatalogReloadToken, setLiveCatalogReloadToken] = React.useState(0);
  const [liveStatusByVideoId, setLiveStatusByVideoId] = React.useState<
    Record<string, DreamFMLiveStatus>
  >({});
  const [selectedOnlineId, setSelectedOnlineId] = React.useState(
    initialPersistedState.selectedOnlineId,
  );
  const [browsePlaylistId, setBrowsePlaylistId] = React.useState(
    isDreamFMPodcastPlaylistId(initialPersistedState.browsePlaylistId)
      ? ""
      : initialPersistedState.browsePlaylistId,
  );
  const [selectedLocalId, setSelectedLocalId] = React.useState(
    initialPersistedState.selectedLocalId,
  );
  const [playMode, setPlayMode] = React.useState<DreamFMPlayMode>(
    initialPersistedState.playMode,
  );
  const [searchItems, setSearchItems] = React.useState<DreamFMOnlineItem[]>([]);
  const [searchArtists, setSearchArtists] = React.useState<DreamFMArtistItem[]>([]);
  const [searchPlaylists, setSearchPlaylists] = React.useState<
    DreamFMPlaylistItem[]
  >([]);
  const [searchLoading, setSearchLoading] = React.useState(false);
  const [searchError, setSearchError] = React.useState(false);
  const [homeShelves, setHomeShelves] = React.useState<DreamFMLibraryShelf[]>(
    [],
  );
  const [libraryPlaylists, setLibraryPlaylists] = React.useState<
    DreamFMPlaylistItem[]
  >([]);
  const [libraryArtists, setLibraryArtists] = React.useState<DreamFMArtistItem[]>([]);
  const [libraryContinuation, setLibraryContinuation] = React.useState("");
  const [libraryLoading, setLibraryLoading] = React.useState(false);
  const [libraryAppending, setLibraryAppending] = React.useState(false);
  const [libraryError, setLibraryError] = React.useState(false);
  const [libraryErrorCode, setLibraryErrorCode] = React.useState("");
  const [libraryReloadToken, setLibraryReloadToken] = React.useState(0);
  const [playlistTracks, setPlaylistTracks] = React.useState<
    DreamFMOnlineItem[]
  >([]);
  const [playlistContinuation, setPlaylistContinuation] = React.useState("");
  const [playlistDetailAuthor, setPlaylistDetailAuthor] = React.useState("");
  const [playlistDetailTitle, setPlaylistDetailTitle] = React.useState("");
  const [playlistLoading, setPlaylistLoading] = React.useState(false);
  const [playlistAppending, setPlaylistAppending] = React.useState(false);
  const [playlistMutationPlaylistId, setPlaylistMutationPlaylistId] =
    React.useState("");
  const [playlistMutationAction, setPlaylistMutationAction] =
    React.useState<DreamFMPlaylistLibraryAction | null>(null);
  const [artistBrowsePage, setArtistBrowsePage] =
    React.useState<DreamFMArtistBrowseState | null>(null);
  const [artistActionBusy, setArtistActionBusy] = React.useState<
    "" | "mix" | "subscribe"
  >("");
  const [onlineQueueState, setOnlineQueueState] =
    React.useState<DreamFMOnlineQueueState>(initialOnlineQueueState);
  const [onlinePlaying, setOnlinePlaying] = React.useState(false);
  const [onlineState, setOnlineState] =
    React.useState<DreamFMRemotePlaybackState>("idle");
  const [onlinePlayerCommand, setOnlinePlayerCommand] =
    React.useState<DreamFMPlayerCommand | null>(null);
  const [localPlaying, setLocalPlaying] = React.useState(false);
  const [localPlayerCommand, setLocalPlayerCommand] =
    React.useState<DreamFMPlayerCommand | null>(null);
  const [muted, setMuted] = React.useState(initialPersistedState.muted);
  const [volume, setVolume] = React.useState(initialPersistedState.volume);
  const lastNonZeroVolumeRef = React.useRef(
    initialPersistedState.volume > 0 ? initialPersistedState.volume : 1,
  );
  const [onlineProgress, setOnlineProgress] = React.useState<
    DreamFMPlaybackProgressState & { videoId: string }
  >({ videoId: "", currentTime: 0, duration: 0, bufferedTime: 0 });
  const [onlineFavoriteByVideoId, setOnlineFavoriteByVideoId] = React.useState<
    Record<string, boolean>
  >({});
  const [favoriteLoadingVideoId, setFavoriteLoadingVideoId] =
    React.useState("");
  const [favoriteMutationVideoId, setFavoriteMutationVideoId] =
    React.useState("");
  const [localProgress, setLocalProgress] =
    React.useState<DreamFMPlaybackProgressState>({
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
  const [onlinePlaybackArmed, setOnlinePlaybackArmed] = React.useState(false);
  const handledExternalCommandRef = React.useRef(0);
  const liveCatalogRealtimeKeyRef = React.useRef("");
  const nativeTrackLookupRef = React.useRef<Map<string, AbortController>>(
    new Map(),
  );
  const nativeStatusRestoreAttemptedRef = React.useRef(false);

  const localTrackIndex = useDreamFMLocalTracks(
    props.libraries,
    props.httpBaseURL,
  );
  const localTracks = localTrackIndex.tracks;

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
    if (!selectedLocalId && localTracks[0]) {
      setSelectedLocalId(localTracks[0].id);
    }
  }, [localTracks, selectedLocalId]);

  React.useEffect(() => {
    if (
      selectedLocalId &&
      !localTracks.some((item) => item.id === selectedLocalId)
    ) {
      setSelectedLocalId(localTracks[0]?.id ?? "");
    }
  }, [localTracks, selectedLocalId]);

  const normalizedQuery = normalizeSearch(query);
  React.useEffect(() => {
    if (mode !== "online" || artistBrowsePage || normalizedQuery.length < 2) {
      setSearchItems([]);
      setSearchArtists([]);
      setSearchPlaylists([]);
      setSearchLoading(false);
      setSearchError(false);
      return;
    }

    const controller = new AbortController();
    setSearchLoading(false);
    setSearchError(false);
    const timer = window.setTimeout(() => {
      setSearchLoading(true);
      void fetchDreamFMSearch(props.httpBaseURL, query, controller.signal)
        .then((payload) => {
          if (!controller.signal.aborted) {
            setSearchItems(payload.items);
            setSearchArtists(payload.artists);
            setSearchPlaylists(payload.playlists);
          }
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            setSearchItems([]);
            setSearchArtists([]);
            setSearchPlaylists([]);
            setSearchError(true);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) {
            setSearchLoading(false);
          }
        });
    }, 350);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [artistBrowsePage, mode, normalizedQuery, props.httpBaseURL, query]);

  React.useEffect(() => {
    if (mode !== "online") {
      return;
    }
    const controller = new AbortController();
    setLibraryLoading(true);
    setLibraryContinuation("");
    setLibraryError(false);
    setLibraryErrorCode("");
    void fetchDreamFMLibrary(
      props.httpBaseURL,
      controller.signal,
      onlineBrowseSource,
      onlineBrowseDetail
        ? {
            browseId: onlineBrowseDetail.browseId,
            params: onlineBrowseDetail.params,
          }
        : {},
    )
      .then((payload) => {
        if (!controller.signal.aborted) {
          setLibraryPlaylists(payload.playlists);
          setLibraryArtists(payload.artists);
          setHomeShelves(payload.shelves);
          setLibraryContinuation(payload.continuation);
        }
      })
      .catch((error) => {
        if (!controller.signal.aborted) {
          setLibraryPlaylists([]);
          setLibraryArtists([]);
          setHomeShelves([]);
          setLibraryContinuation("");
          setLibraryError(true);
          setLibraryErrorCode(getDreamFMErrorCode(error));
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLibraryLoading(false);
        }
      });
    return () => controller.abort();
  }, [libraryReloadToken, mode, onlineBrowseDetail, onlineBrowseSource, props.httpBaseURL]);

  const artistBrowseId = artistBrowsePage?.id ?? "";
  const artistBrowseName = artistBrowsePage?.name ?? "";
  React.useEffect(() => {
    if (mode !== "online" || (!artistBrowseId && !artistBrowseName)) {
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
    void fetchDreamFMArtist(
      props.httpBaseURL,
      { id: artistBrowseId, name: artistBrowseName },
      controller.signal,
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
      });
    return () => controller.abort();
  }, [artistBrowseId, artistBrowseName, mode, props.httpBaseURL]);

  const requestOnlineAutoplay = React.useCallback(() => {
    setPlaybackSessionStarted(true);
    setOnlinePlaybackArmed(true);
    setOnlinePlaying(false);
    setOnlineState("loading");
    setOnlinePlayerCommand({
      id: Date.now(),
      command: "play",
    });
  }, []);

  const changeOnlineBrowseSource = React.useCallback(
    (source: DreamFMOnlineBrowseSource) => {
      setOnlineBrowseSource(source);
      setOnlineBrowseDetail(null);
      setBrowsePlaylistId("");
      setArtistBrowsePage(null);
    },
    [],
  );

  const openOnlineBrowseCategory = React.useCallback(
    (item: DreamFMCategoryItem) => {
      const browseId = item.browseId.trim();
      if (!browseId) {
        return;
      }
      setMode("online");
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
    const controller = new AbortController();
    setLibraryAppending(true);
    void fetchDreamFMLibrary(
      props.httpBaseURL,
      controller.signal,
      onlineBrowseSource,
      {
        browseId: onlineBrowseDetail?.browseId,
        params: onlineBrowseDetail?.params,
        continuation,
      },
    )
      .then((payload) => {
        if (controller.signal.aborted) {
          return;
        }
        setHomeShelves((current) =>
          dedupeLibraryShelves([...current, ...payload.shelves]),
        );
        setLibraryContinuation(payload.continuation);
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setLibraryContinuation("");
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLibraryAppending(false);
        }
      });
  }, [
    libraryAppending,
    libraryContinuation,
    libraryLoading,
    onlineBrowseDetail?.browseId,
    onlineBrowseDetail?.params,
    onlineBrowseSource,
    props.httpBaseURL,
  ]);

  const reloadLiveCatalog = React.useCallback(() => {
    setLiveCatalogReloadToken((current) => current + 1);
  }, []);
  React.useEffect(() => {
    return registerTopic(REALTIME_TOPICS.dreamfm.liveCatalog, (event) => {
      if (event.type && event.type !== "catalog-updated" && event.type !== "resync-required") {
        return;
      }
      const key =
        resolveDreamFMLiveCatalogEventKey(event.payload) ||
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
    void fetchDreamFMLiveCatalog(props.httpBaseURL, controller.signal)
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
        setLiveCatalogMessage(getDreamFMErrorMessage(error));
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLiveCatalogLoading(false);
        }
      });
    return () => controller.abort();
  }, [liveCatalogReloadToken, props.httpBaseURL]);

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
    (itemOrId: DreamFMOnlineItem | string) => {
      const nextId = resolveDreamFMLiveSelectionId(
        liveQueue,
        typeof itemOrId === "string" ? itemOrId : itemOrId.id,
      );
      if (!nextId) {
        return;
      }
      setSelectedLiveId(nextId);
      setLiveSelectionArmed(true);
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
    if (!props.active || liveSelectionArmed || mode !== "live") {
      return;
    }
    const nextId = resolveDreamFMLiveSelectionId(liveQueue, selectedLiveId);
    if (!nextId) {
      return;
    }
    setSelectedLiveId(nextId);
    setLiveSelectionArmed(true);
  }, [liveQueue, liveSelectionArmed, mode, props.active, selectedLiveId]);

  React.useEffect(() => {
    if (
      !props.active ||
      mode !== "live" ||
      liveStatusVideoIds.length === 0 ||
      !props.httpBaseURL.trim()
    ) {
      return;
    }
    const controller = new AbortController();
    setLiveStatusByVideoId((current) => {
      const next = { ...current };
      liveStatusVideoIds.forEach((videoId) => {
        next[videoId] = {
          videoId,
          status: "checking",
        };
      });
      return next;
    });
    void fetchDreamFMLiveStatuses(
      props.httpBaseURL,
      liveStatusVideoIds,
      controller.signal,
    )
      .then((statuses) => {
        if (controller.signal.aborted) {
          return;
        }
        setLiveStatusByVideoId((current) => ({
          ...current,
          ...statuses,
        }));
      })
      .catch(() => {
        if (controller.signal.aborted) {
          return;
        }
        setLiveStatusByVideoId((current) => {
          const next = { ...current };
          liveStatusVideoIds.forEach((videoId) => {
            next[videoId] = {
              videoId,
              status: "unknown",
            };
          });
          return next;
        });
      });
    return () => controller.abort();
  }, [
    liveStatusVideoIdKey,
    liveStatusVideoIds,
    mode,
    props.active,
    props.httpBaseURL,
  ]);
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
  const onlinePlaybackQueue = onlineQueueState.items;
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
    mode === "online" &&
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
    onlinePlaybackQueue.find((item) => item.id === selectedOnlineId) ??
    onlinePlaybackQueue[0] ??
    null;
  const selectedPlaylist =
    allOnlinePlaylists.find((item) => item.playlistId === browsePlaylistId) ??
    null;
  const selectedLocal =
    localTracks.find((item) => item.id === selectedLocalId) ??
    localTracks[0] ??
    null;
  const activeOnline =
    mode === "live" ? selectedLive : mode === "online" ? selectedOnline : null;
  const selectedLocalResumeTime = selectedLocal?.path
    ? (localProgressByPath[selectedLocal.path] ?? 0)
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
  const activeModeTitle =
    mode === "live"
      ? props.text.dreamFm.live
      : mode === "online"
        ? props.text.dreamFm.online
        : props.text.dreamFm.local;
  const externalPlayRequested =
    mode === "local"
      ? props.controlCommand?.command === "play" &&
        props.controlCommand.id === handledExternalCommandRef.current
      : props.controlCommand?.command === "play" &&
        props.controlCommand.id === handledExternalCommandRef.current;
  const dreamFMNowPlayingStatus = React.useMemo<DreamFMNowPlayingStatus>(() => {
    const onlineLoading =
      mode !== "local" &&
      onlineState === "loading" &&
      !onlinePlaying &&
      onlinePlayerCommand?.command === "play";
    const localLoading =
      mode === "local" && externalPlayRequested && !localPlaying;
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
        mode,
        canControl: false,
        progress: { currentTime: 0, duration: 0, bufferedTime: 0 },
      };
    }
    if (mode === "local") {
      return {
        state: localLoading ? "loading" : localPlaying ? "playing" : "paused",
        title: selectedLocal?.title ?? props.text.dreamFm.local,
        subtitle: selectedLocal?.author ?? "",
        artworkURL: selectedLocal?.coverURL ?? "",
        mode,
        canControl: Boolean(selectedLocal),
        progress: localProgress,
      };
    }
    const onlineArtworkURL = activeOnline
      ? buildDreamFMImageCacheURL(
          props.httpBaseURL,
          buildDreamFMHighQualityThumbnailURL(activeOnline.thumbnailUrl ?? ""),
        ) ||
        buildDreamFMImageCacheURL(
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
      mode,
      canControl: Boolean(activeOnline),
      progress: onlineProgress,
    };
  }, [
    activeModeTitle,
    activeOnline,
    externalPlayRequested,
    localPlaying,
    localProgress.bufferedTime,
    localProgress.currentTime,
    localProgress.duration,
    mode,
    onlinePlayerCommand?.command,
    onlinePlaying,
    onlineProgress.bufferedTime,
    onlineProgress.currentTime,
    onlineProgress.duration,
    onlineState,
    playbackSessionStarted,
    props.text.dreamFm.local,
    props.httpBaseURL,
    selectedLocal,
  ]);

  React.useEffect(() => {
    props.onNowPlayingChange?.(dreamFMNowPlayingStatus);
  }, [dreamFMNowPlayingStatus, props.onNowPlayingChange]);

  React.useEffect(() => {
    if (!activeOnline) {
      setOnlineProgress({
        videoId: "",
        currentTime: 0,
        duration: 0,
        bufferedTime: 0,
      });
      return;
    }
    setOnlineProgress((current) => {
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
  }, [activeOnline?.group, activeOnline?.videoId, activeOnlineResumeTime]);

  React.useEffect(() => {
    if (mode === "local" || !activeOnline) {
      setOnlineState("idle");
      return;
    }
    if (!onlinePlaybackArmed) {
      setOnlineState("idle");
      return;
    }
    setOnlineState((current) => {
      if (onlinePlayerCommand?.command === "play") {
        return "loading";
      }
      if (onlinePlayerCommand?.command === "replay") {
        return "buffering";
      }
      if (onlinePlayerCommand?.command === "pause") {
        return "paused";
      }
      return current === "idle" || current === "ended" ? "paused" : current;
    });
  }, [
    activeOnline?.videoId,
    mode,
    onlinePlaybackArmed,
    onlinePlayerCommand?.command,
  ]);

  React.useEffect(() => {
    if (mode !== "online" || !onlineFavoriteSeedKey) {
      return;
    }
    const controller = new AbortController();
    void fetchDreamFMTrackFavoriteStatuses(
      props.httpBaseURL,
      onlineFavoriteSeedVideoIds,
      controller.signal,
    )
      .then((statuses) => {
        if (controller.signal.aborted || Object.keys(statuses).length === 0) {
          return;
        }
        setOnlineFavoriteByVideoId((current) => ({
          ...current,
          ...statuses,
        }));
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, [
    mode,
    onlineFavoriteSeedKey,
    onlineFavoriteSeedVideoIds,
    props.httpBaseURL,
  ]);

  React.useEffect(() => {
    if (mode !== "online" || !activeOnline || activeOnline.group === "live") {
      setFavoriteLoadingVideoId("");
      return;
    }
    if (onlineFavoriteByVideoId[activeOnline.videoId] !== undefined) {
      return;
    }
    const controller = new AbortController();
    setFavoriteLoadingVideoId(activeOnline.videoId);
    void fetchDreamFMTrackFavorite(
      props.httpBaseURL,
      activeOnline.videoId,
      controller.signal,
    )
      .then((status) => {
        if (!controller.signal.aborted && status.known) {
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
  }, [activeOnline, mode, onlineFavoriteByVideoId, props.httpBaseURL]);

  React.useEffect(() => {
    if (mode !== "online" && browsePlaylistId === "") {
      return;
    }
    if (browsePlaylistId === "") {
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
    void fetchDreamFMPlaylistPage(
      props.httpBaseURL,
      browsePlaylistId,
      controller.signal,
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
        }
      });
    return () => controller.abort();
  }, [browsePlaylistId, mode, props.httpBaseURL]);

  React.useEffect(() => {
    if (mode !== "online" || onlineQueueState.items.length === 0) {
      return;
    }
    if (
      selectedOnlineId &&
      onlineQueueState.items.some((item) => item.id === selectedOnlineId)
    ) {
      return;
    }
    setSelectedOnlineId(onlineQueueState.items[0]?.id ?? "");
  }, [mode, onlineQueueState.items, selectedOnlineId]);

  React.useEffect(() => {
    if (onlineQueueState.kind !== "radio" || !onlineQueueState.seedVideoId) {
      return;
    }
    if (onlineQueueState.items.length > 1) {
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void fetchDreamFMRadio(
        props.httpBaseURL,
        onlineQueueState.seedVideoId,
        controller.signal,
      )
        .then((items) => {
          if (!controller.signal.aborted) {
            setOnlineQueueState((current) => {
              if (
                current.kind !== "radio" ||
                current.seedVideoId !== onlineQueueState.seedVideoId
              ) {
                return current;
              }
              return {
                ...current,
                items: dedupeOnlineItems([...current.items, ...items]),
              };
            });
          }
        })
        .catch(() => {
          // Keep the seed-only queue when radio expansion fails.
        });
    }, 450);

    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [onlineQueueState, props.httpBaseURL]);

  React.useEffect(() => {
    if (
      onlineQueueState.kind !== "playlist" ||
      !onlineQueueState.playlistId ||
      onlineQueueState.items.length > 0
    ) {
      return;
    }
    const controller = new AbortController();
    void fetchDreamFMPlaylistQueue(
      props.httpBaseURL,
      onlineQueueState.playlistId,
      controller.signal,
    )
      .then((items) => {
        if (!controller.signal.aborted) {
          setOnlineQueueState((current) => {
            if (
              current.kind !== "playlist" ||
              current.playlistId !== onlineQueueState.playlistId
            ) {
              return current;
            }
            return {
              ...current,
              items,
            };
          });
        }
      })
      .catch(() => {});
    return () => controller.abort();
  }, [onlineQueueState, props.httpBaseURL]);

  const currentQueue =
    mode === "live"
      ? liveQueue
      : mode === "online"
        ? onlinePlaybackQueue
        : localTracks;
  const currentIndex =
    mode === "live"
      ? resolveQueueIndex(liveQueue, liveSelectionArmed ? selectedLiveId : "")
      : mode === "online"
        ? resolveQueueIndex(onlinePlaybackQueue, selectedOnlineId)
        : resolveQueueIndex(localTracks, selectedLocalId);

  const selectLocalQueueTrack = React.useCallback(
    (item: { id: string }) => {
      if (!item.id) {
        return;
      }
      setSelectedLocalId(item.id);
      if (localPlaying) {
        setLocalPlayerCommand({
          id: Date.now(),
          command: "play",
        });
      }
    },
    [localPlaying],
  );

  const selectQueueIndex = React.useCallback(
    (index: number) => {
      if (mode === "live") {
        const next = liveQueue[index];
        if (next) {
          activateLiveSelection(next.id);
        }
        return;
      }
      if (mode === "online") {
        const next = onlinePlaybackQueue[index];
        if (next) {
          setSelectedOnlineId(next.id);
          requestOnlineAutoplay();
        }
        return;
      }
      const next = localTracks[index];
      if (next) {
        selectLocalQueueTrack(next);
      }
    },
    [
      activateLiveSelection,
      liveQueue,
      localTracks,
      mode,
      onlinePlaybackQueue,
      requestOnlineAutoplay,
      selectLocalQueueTrack,
    ],
  );

  const replayCurrent = React.useCallback(() => {
    if (mode === "local") {
      if (!selectedLocal) {
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
    setOnlinePlayerCommand({
      id: Date.now(),
      command: "replay",
    });
  }, [activeOnline, mode, selectedLocal]);

  const playNext = React.useCallback(() => {
    if (currentQueue.length === 0) {
      return;
    }
    const effectivePlayMode =
      mode === "live" && playMode === "repeat" ? "order" : playMode;
    if (effectivePlayMode === "repeat") {
      replayCurrent();
      return;
    }
    const nextIndex =
      effectivePlayMode === "shuffle"
        ? resolveRandomIndex(currentQueue.length, currentIndex)
        : resolveAdjacentIndex(currentQueue.length, currentIndex, 1);
    selectQueueIndex(nextIndex);
  }, [
    currentIndex,
    currentQueue,
    currentQueue.length,
    mode,
    playMode,
    replayCurrent,
    selectQueueIndex,
  ]);

  const playPrevious = React.useCallback(() => {
    if (currentQueue.length === 0) {
      return;
    }
    selectQueueIndex(
      resolveAdjacentIndex(currentQueue.length, currentIndex, -1),
    );
  }, [currentIndex, currentQueue, currentQueue.length, selectQueueIndex]);

  const togglePlayMode = React.useCallback(() => {
    setPlayMode((current) => {
      if (current === "order") {
        return "shuffle";
      }
      if (mode === "live") {
        return "order";
      }
      if (current === "shuffle") {
        return "repeat";
      }
      return "order";
    });
  }, [mode]);

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
      if (!selectedLocal?.path) {
        return;
      }
      const resumeSeconds =
        duration > 0 && duration - currentTime <= 1.5 ? 0 : currentTime;
      setLocalProgressByPath((current) =>
        updateDreamFMProgressMap(current, selectedLocal.path, resumeSeconds),
      );
    },
    [selectedLocal?.path],
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
      setOnlineProgress((current) => {
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
      if (!transient && activeOnline?.group !== "live") {
        const resumeSeconds =
          duration > 0 && duration - currentTime <= 1.5 ? 0 : currentTime;
        setOnlineProgressByVideoId((current) =>
          updateDreamFMProgressMap(current, videoId, resumeSeconds),
        );
      }
    },
    [activeOnline?.group, activeOnline?.videoId],
  );

  const enrichOnlineTrackMetadata = React.useCallback(
    (
      videoId: string,
      seed: { title?: string; artist?: string; thumbnailUrl?: string } = {},
    ) => {
      const trimmedVideoId = videoId.trim();
      if (!trimmedVideoId || nativeTrackLookupRef.current.has(trimmedVideoId)) {
        return;
      }
      const controller = new AbortController();
      nativeTrackLookupRef.current.set(trimmedVideoId, controller);
      void fetchDreamFMTrackInfo(
        props.httpBaseURL,
        trimmedVideoId,
        controller.signal,
      )
        .then((item) => {
          if (controller.signal.aborted || !item) {
            return;
          }
          setOnlineQueueState((current) => {
            const incoming = mergeDreamFMNativeTrackItem(
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
            if (current.kind === "none") {
              return {
                kind: "radio",
                title: props.text.dreamFm.upNext,
                items: [incoming],
                seedVideoId: trimmedVideoId,
              };
            }
            let found = false;
            const items = current.items.map((queueItem) => {
              if (queueItem.videoId !== trimmedVideoId) {
                return queueItem;
              }
              found = true;
              return mergeDreamFMNativeTrackItem(incoming, queueItem);
            });
            return {
              ...current,
              items: dedupeOnlineItems(found ? items : [...items, incoming]),
            };
          });
        })
        .catch(() => undefined)
        .finally(() => {
          if (nativeTrackLookupRef.current.get(trimmedVideoId) === controller) {
            nativeTrackLookupRef.current.delete(trimmedVideoId);
          }
        });
    },
    [props.text.dreamFm.upNext, props.httpBaseURL],
  );

  React.useEffect(() => {
    if (mode !== "online" || !activeOnline || activeOnline.group === "live") {
      return;
    }
    const needsMetadata =
      !activeOnline.durationLabel.trim() ||
      !activeOnline.thumbnailUrl?.trim() ||
      activeOnline.title.trim() === activeOnline.videoId ||
      isMissingDreamFMArtist(activeOnline.channel);
    if (!needsMetadata) {
      return;
    }
    enrichOnlineTrackMetadata(activeOnline.videoId, {
      title: activeOnline.title,
      artist: activeOnline.channel,
      thumbnailUrl: activeOnline.thumbnailUrl,
    });
  }, [activeOnline, enrichOnlineTrackMetadata, mode]);

  const handleOnlineNativeTrackChange = React.useCallback(
    (event: DreamFMNativePlayerEvent) => {
      if (mode !== "online") {
        return;
      }
      const videoId = String(
        event.observedVideoId || event.videoId || "",
      ).trim();
      if (!videoId) {
        return;
      }
      const observedFavorite = favoriteFromLikeStatus(event.likeStatus);
      if (observedFavorite !== null) {
        setOnlineFavoriteByVideoId((current) => ({
          ...current,
          [videoId]: observedFavorite,
        }));
      }
      const existing = onlinePlaybackQueue.find(
        (item) => item.videoId === videoId,
      );
      const rawEventTitle = String(event.title || "").trim();
      const rawEventArtist = String(event.artist || "").trim();
      const nativeMetadataTrusted = event.metadataSource === "api";
      const eventTitle =
        nativeMetadataTrusted || !existing || existing.title.trim() === videoId
          ? rawEventTitle
          : "";
      const eventArtist =
        nativeMetadataTrusted ||
        !existing ||
        isMissingDreamFMArtist(existing.channel)
          ? rawEventArtist
          : "";
      const eventThumbnail = String(event.thumbnailUrl || "").trim();
      if (existing) {
        setSelectedOnlineId(existing.id);
        if (
          eventTitle ||
          eventArtist ||
          eventThumbnail ||
          isMissingDreamFMArtist(existing.channel)
        ) {
          const observedItem = createNativeOnlineItem({
            videoId,
            title: eventTitle,
            artist: eventArtist,
            thumbnailUrl: eventThumbnail,
          });
          setOnlineQueueState((current) => ({
            ...current,
            items: current.items.map((queueItem) =>
              queueItem.videoId === videoId
                ? mergeDreamFMNativeTrackItem(observedItem, queueItem)
                : queueItem,
            ),
          }));
        }
      } else {
        const generatedItem = createNativeOnlineItem({
          videoId,
          title: eventTitle,
          artist: eventArtist,
          thumbnailUrl: eventThumbnail,
        });
        setOnlineQueueState((current) => {
          if (current.kind === "none") {
            return {
              kind: "radio",
              title: props.text.dreamFm.upNext,
              items: [generatedItem],
              seedVideoId: videoId,
            };
          }
          if (current.items.some((item) => item.videoId === videoId)) {
            return current;
          }
          return {
            ...current,
            items: dedupeOnlineItems([...current.items, generatedItem]),
          };
        });
        setSelectedOnlineId(generatedItem.id);
      }
      const needsMetadata =
        !existing ||
        !existing.durationLabel.trim() ||
        !existing.thumbnailUrl?.trim() ||
        existing.title.trim() === videoId ||
        isMissingDreamFMArtist(existing.channel);
      if (needsMetadata) {
        enrichOnlineTrackMetadata(videoId, {
          title: eventTitle,
          artist: eventArtist,
          thumbnailUrl: eventThumbnail,
        });
      }
      setOnlineState(event.state || "playing");
      setOnlinePlaying(
        event.state === "buffering" ||
          event.state === "playing" ||
          event.state === undefined,
      );
      handleOnlineProgressChange(
        videoId,
        Number(event.currentTime || 0),
        Number(event.duration || 0),
        Number(event.bufferedTime || 0),
        true,
      );
    },
    [
      handleOnlineProgressChange,
      enrichOnlineTrackMetadata,
      mode,
      onlinePlaybackQueue,
      props.text.dreamFm.upNext,
    ],
  );

  const restoreNativePlaybackSession = React.useCallback(
    (event: DreamFMNativePlayerEvent) => {
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
      if (observedFavorite !== null) {
        setOnlineFavoriteByVideoId((current) => ({
          ...current,
          [videoId]: observedFavorite,
        }));
      }
      setPlaybackSessionStarted(true);
      setOnlinePlaybackArmed(true);
      setOnlineState(event.state);
      setOnlinePlaying(
        event.state === "playing" || event.state === "buffering",
      );
      handleOnlineProgressChange(
        videoId,
        Number.isFinite(currentTime) ? currentTime : 0,
        Number.isFinite(duration) ? duration : 0,
        Number.isFinite(bufferedTime) ? bufferedTime : 0,
        true,
      );

      const liveItem = liveQueue.find((item) => item.videoId === videoId);
      if (liveItem) {
        setMode("live");
        setSelectedLiveId(liveItem.id);
        setLiveSelectionArmed(true);
        return;
      }
      if (event.source === "dreamfm-youtube-live-player") {
        setMode("live");
        setSelectedLiveId(videoId);
        setLiveSelectionArmed(false);
        return;
      }

      setMode("online");
      const existing = onlinePlaybackQueue.find(
        (item) => item.videoId === videoId,
      );
      if (existing) {
        setSelectedOnlineId(existing.id);
        return;
      }

      const generatedItem = createNativeOnlineItem({
        videoId,
        title: event.title,
        artist: event.artist,
        thumbnailUrl: event.thumbnailUrl,
      });
      setOnlineQueueState((current) => {
        if (current.items.some((item) => item.videoId === videoId)) {
          return current;
        }
        if (current.kind === "none") {
          return {
            kind: "radio",
            title: props.text.dreamFm.upNext,
            items: [generatedItem],
            seedVideoId: videoId,
          };
        }
        return {
          ...current,
          items: dedupeOnlineItems([...current.items, generatedItem]),
        };
      });
      setSelectedOnlineId(generatedItem.id);
    },
    [
      handleOnlineProgressChange,
      liveQueue,
      onlinePlaybackQueue,
      props.text.dreamFm.upNext,
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
      Call.ByName(`${DREAM_FM_LIVE_PLAYER_SERVICE}.Status`).then((status) =>
        nativeStatusToPlayerEvent(status, "dreamfm-youtube-live-player"),
      ),
      Call.ByName(`${DREAM_FM_NATIVE_PLAYER_SERVICE}.Status`).then((status) =>
        nativeStatusToPlayerEvent(status, "dreamfm-youtube-music-player"),
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
    if (mode !== "online" || !activeOnline || activeOnline.group === "live") {
      return;
    }
    const videoId = activeOnline.videoId;
    const previousLiked = onlineFavoriteByVideoId[videoId] === true;
    const nextLiked = !previousLiked;
    const controller = new AbortController();
    setFavoriteMutationVideoId(videoId);
    setOnlineFavoriteByVideoId((current) => ({
      ...current,
      [videoId]: nextLiked,
    }));
    void updateDreamFMTrackFavorite(
      props.httpBaseURL,
      videoId,
      nextLiked,
      controller.signal,
    )
      .then((liked) => {
        if (!controller.signal.aborted) {
          setOnlineFavoriteByVideoId((current) => ({
            ...current,
            [videoId]: liked,
          }));
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
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
  }, [activeOnline, mode, onlineFavoriteByVideoId, props.httpBaseURL]);

  const openSelectedLocalDirectory = React.useCallback(() => {
    const path = selectedLocal?.path?.trim();
    if (!path) {
      return;
    }
    void openLibraryPath.mutateAsync({ path }).catch(() => {});
  }, [openLibraryPath, selectedLocal?.path]);

  React.useEffect(() => {
    if (mode === "local") {
      setOnlinePlaying(false);
    }
  }, [mode]);

  React.useEffect(() => {
    if (mode !== "local") {
      setLocalPlaying(false);
    }
  }, [mode]);

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
    if (mode === "local") {
      if (selectedLocal?.path) {
        setLocalProgressByPath((current) =>
          updateDreamFMProgressMap(current, selectedLocal.path, 0),
        );
      }
      setLocalProgress({ currentTime: 0, duration: 0, bufferedTime: 0 });
    } else if (activeOnline?.group === "live") {
      return;
    } else if (activeOnline?.videoId) {
      setOnlineProgress((current) =>
        current.videoId === activeOnline.videoId
          ? {
              videoId: current.videoId,
              currentTime: 0,
              duration: 0,
              bufferedTime: 0,
            }
          : current,
      );
      setOnlineProgressByVideoId((current) =>
        updateDreamFMProgressMap(current, activeOnline.videoId, 0),
      );
    }
    playNext();
  }, [
    activeOnline?.group,
    activeOnline?.videoId,
    mode,
    playNext,
    selectedLocal?.path,
  ]);

  React.useEffect(() => {
    const timer = window.setTimeout(() => {
      writeDreamFMStorageState({
        version: 1,
        mode,
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
    if (mode === "local") {
      if (!selectedLocal) {
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
    setOnlinePlaybackArmed(true);
    const command = onlinePlaying ? "pause" : "play";
    const commandId = Date.now();
    if (!onlinePlaying) {
      setOnlineState("loading");
    }
    setOnlinePlayerCommand({
      id: commandId,
      command,
    });
  }, [
    activeOnline,
    localPlaying,
    mode,
    onlinePlaying,
    selectedLocal,
  ]);

  React.useEffect(() => {
    const command = props.controlCommand;
    if (!command || handledExternalCommandRef.current === command.id) {
      return;
    }
    handledExternalCommandRef.current = command.id;
    const isPlaying = mode === "local" ? localPlaying : onlinePlaying;
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
    mode,
    onlinePlaying,
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

  const playOnlineItemsQueue = React.useCallback(
    (
      items: DreamFMOnlineItem[],
      title: string,
      selectedItem?: DreamFMOnlineItem,
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
      setOnlineQueueState({
        kind: "radio",
        title,
        items: queueItems,
        seedVideoId: selectedQueueItem.videoId,
      });
      setSelectedOnlineId(selectedQueueItem.id);
      requestOnlineAutoplay();
    },
    [requestOnlineAutoplay],
  );

  const playOnlineRadioSeed = React.useCallback(
    (item: DreamFMOnlineItem) => {
      setOnlineQueueState({
        kind: "radio",
        title: props.text.dreamFm.groupRadio,
        items: [item],
        seedVideoId: item.videoId,
      });
      setSelectedOnlineId(item.id);
      requestOnlineAutoplay();
    },
    [props.text.dreamFm.groupRadio, requestOnlineAutoplay],
  );

  const playOnlineShelfTrack = React.useCallback(
    (shelf: DreamFMLibraryShelf, item: DreamFMOnlineItem) => {
      const shelfItems = shelf.tracks.some((track) => track.id === item.id)
        ? shelf.tracks
        : [item, ...shelf.tracks];
      playOnlineItemsQueue(
        shelfItems,
        resolveDreamFMShelfQueueTitle(
          shelf,
          props.text,
          props.text.dreamFm.groupRecommendations,
        ),
        item,
      );
    },
    [playOnlineItemsQueue, props.text, props.text.dreamFm.groupRecommendations],
  );

  const playOnlineShelfAll = React.useCallback(
    (shelf: DreamFMLibraryShelf) => {
      playOnlineItemsQueue(
        shelf.tracks,
        resolveDreamFMShelfQueueTitle(
          shelf,
          props.text,
          props.text.dreamFm.groupRecommendations,
        ),
      );
    },
    [playOnlineItemsQueue, props.text, props.text.dreamFm.groupRecommendations],
  );

  const shuffleOnlineShelf = React.useCallback(
    (shelf: DreamFMLibraryShelf) => {
      playOnlineItemsQueue(
        shuffleDreamFMOnlineItems(shelf.tracks),
        resolveDreamFMShelfQueueTitle(
          shelf,
          props.text,
          props.text.dreamFm.groupRecommendations,
        ),
      );
    },
    [playOnlineItemsQueue, props.text, props.text.dreamFm.groupRecommendations],
  );

  const playOnlineSearchResults = React.useCallback(() => {
    playOnlineItemsQueue(searchItems, props.text.dreamFm.groupRecommendations);
  }, [playOnlineItemsQueue, props.text.dreamFm.groupRecommendations, searchItems]);

  const playOnlineSearchTrack = React.useCallback(
    (item: DreamFMOnlineItem) => {
      playOnlineItemsQueue(
        searchItems.some((track) => track.id === item.id)
          ? searchItems
          : [item, ...searchItems],
        props.text.dreamFm.searchSongs,
        item,
      );
    },
    [playOnlineItemsQueue, props.text.dreamFm.searchSongs, searchItems],
  );

  const shuffleOnlineSearchResults = React.useCallback(() => {
    playOnlineItemsQueue(
      shuffleDreamFMOnlineItems(searchItems),
      props.text.dreamFm.groupRecommendations,
    );
  }, [playOnlineItemsQueue, props.text.dreamFm.groupRecommendations, searchItems]);

  const selectOnlineQueueTrack = React.useCallback(
    (item: DreamFMOnlineItem) => {
      setSelectedOnlineId(item.id);
      if (mode === "online") {
        requestOnlineAutoplay();
      }
    },
    [mode, requestOnlineAutoplay],
  );

  const clearOnlineQueue = React.useCallback(() => {
    setOnlineQueueState(createDefaultDreamFMOnlineQueueState());
    setSelectedOnlineId("");
    setOnlinePlaybackArmed(false);
    setOnlinePlaying(false);
    setOnlineState("idle");
    setOnlineProgress({
      videoId: "",
      currentTime: 0,
      duration: 0,
      bufferedTime: 0,
    });
  }, []);

  const removeOnlineQueueItem = React.useCallback(
    (item: DreamFMOnlineItem) => {
      const removeIndex = onlineQueueState.items.findIndex(
        (track) => track.id === item.id,
      );
      if (removeIndex < 0) {
        return;
      }
      const nextItems = onlineQueueState.items.filter(
        (track) => track.id !== item.id,
      );
      if (nextItems.length === 0) {
        clearOnlineQueue();
        return;
      }
      const shouldAdvanceSelection = selectedOnlineId === item.id;
      const nextSelectedItem = shouldAdvanceSelection
        ? nextItems[Math.min(removeIndex, nextItems.length - 1)]
        : null;
      setOnlineQueueState((current) => {
        if (!current.items.some((track) => track.id === item.id)) {
          return current;
        }
        const currentNextItems = current.items.filter(
          (track) => track.id !== item.id,
        );
        if (currentNextItems.length === 0) {
          return createDefaultDreamFMOnlineQueueState();
        }
        return { ...current, items: currentNextItems };
      });
      if (nextSelectedItem) {
        setSelectedOnlineId(nextSelectedItem.id);
        if (
          onlinePlaying ||
          onlineState === "loading" ||
          onlineState === "playing" ||
          onlineState === "buffering"
        ) {
          requestOnlineAutoplay();
        }
      }
    },
    [
      clearOnlineQueue,
      onlinePlaying,
      onlineQueueState.items,
      onlineState,
      requestOnlineAutoplay,
      selectedOnlineId,
    ],
  );

  const openPlaylistBrowse = React.useCallback((item: DreamFMPlaylistItem) => {
    const playlistId = item.playlistId.trim();
    if (!playlistId || isDreamFMPodcastPlaylistId(playlistId)) {
      return;
    }
    setBrowsePlaylistId(playlistId);
  }, []);

  const openOnlineArtistBrowse = React.useCallback(
    (item: DreamFMOnlineItem) => {
      const artistName = item.channel.trim();
      if (!artistName) {
        return;
      }
      setListOpen(true);
      setMode("online");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setQuery("");
      setArtistBrowsePage({
        id: item.artistBrowseId?.trim() ?? "",
        name: artistName,
        title: artistName,
        subtitle: "",
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
    (item: DreamFMArtistItem) => {
      const artistName = item.name.trim();
      const artistId = item.browseId.trim();
      if (!artistName && !artistId) {
        return;
      }
      setMode("online");
      setSidebarView("browse");
      setBrowsePlaylistId("");
      setQuery("");
      setArtistBrowsePage({
        id: artistId,
        name: artistName,
        title: artistName || artistId,
        subtitle: item.subtitle,
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
      setOnlineQueueState({
        kind: "radio",
        title: page.title || page.name,
        items: page.items,
        seedVideoId: next.videoId,
      });
      setSelectedOnlineId(next.id);
      requestOnlineAutoplay();
    },
    [artistBrowsePage, requestOnlineAutoplay],
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
    setOnlineQueueState({
      kind: "radio",
      title: page.title || page.name,
      items,
      seedVideoId: first.videoId,
    });
    setSelectedOnlineId(first.id);
    requestOnlineAutoplay();
  }, [artistBrowsePage, requestOnlineAutoplay]);

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
    void fetchDreamFMArtist(
      props.httpBaseURL,
      { id: page.id, name: page.name },
      controller.signal,
      { continuation },
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
  }, [artistBrowsePage, props.httpBaseURL]);

  const playArtistMix = React.useCallback(() => {
    const page = artistBrowsePage;
    const playlistId = page?.mixPlaylistId.trim() ?? "";
    if (!page || !playlistId) {
      return;
    }
    const controller = new AbortController();
    setArtistActionBusy("mix");
    void fetchDreamFMPlaylistQueue(
      props.httpBaseURL,
      playlistId,
      controller.signal,
    )
      .then((items) => {
        const nextItems = items.length > 0 ? items : page.items;
        const first = nextItems[0];
        if (!first) {
          return;
        }
        setOnlineQueueState({
          kind: "playlist",
          title: page.title || page.name,
          items: nextItems,
          playlistId,
        });
        setSelectedOnlineId(first.id);
        requestOnlineAutoplay();
      })
      .catch(() => {})
      .finally(() => {
        if (!controller.signal.aborted) {
          setArtistActionBusy("");
        }
      });
  }, [artistBrowsePage, props.httpBaseURL, requestOnlineAutoplay]);

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
    void updateDreamFMArtistSubscription(
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
      const queueItems = applyDreamFMPlaylistPlaybackFallback(
        playlistTracks,
        playlistDetailAuthor || playlist.channel,
      );
      const selectedQueueItem =
        queueItems.find((item) => item.id === next.id) ?? next;
      setOnlineQueueState({
        kind: "playlist",
        title: playlistDetailTitle || playlist.title,
        items: queueItems,
        playlistId: playlist.playlistId,
      });
      setSelectedOnlineId(selectedQueueItem.id);
      requestOnlineAutoplay();
    },
    [
      playlistDetailAuthor,
      playlistDetailTitle,
      playlistTracks,
      requestOnlineAutoplay,
      selectedPlaylist,
    ],
  );

  const loadMorePlaylist = React.useCallback(() => {
    const continuation = playlistContinuation.trim();
    const playlistId = browsePlaylistId.trim();
    if (!continuation || playlistAppending || playlistLoading) {
      return;
    }
    const controller = new AbortController();
    setPlaylistAppending(true);
    void fetchDreamFMPlaylistPage(
      props.httpBaseURL,
      playlistId,
      controller.signal,
      continuation,
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
    playlistAppending,
    playlistContinuation,
    playlistLoading,
    props.httpBaseURL,
  ]);

  const updatePlaylistLibrary = React.useCallback(
    (item: DreamFMPlaylistItem, action: DreamFMPlaylistLibraryAction) => {
      const controller = new AbortController();
      setPlaylistMutationPlaylistId(item.playlistId);
      setPlaylistMutationAction(action);
      void updateDreamFMPlaylistLibrary(
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
    if (mode === "live") {
      const first = curatedLiveItems[0];
      if (first) {
        activateLiveSelection(first);
      }
      return;
    }
    if (mode === "online") {
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
    mode === "live" && normalizedQuery && curatedLiveItems.length === 0
      ? props.text.dreamFm.searchEmpty
      : "";
  const onlineSearchNotice =
    mode === "online" && !artistBrowsePage && normalizedQuery
      ? searchLoading
        ? props.text.dreamFm.searchLoading
        : searchError
          ? props.text.dreamFm.searchUnavailable
          : searchItems.length === 0 &&
              searchArtists.length === 0 &&
              searchPlaylists.length === 0 &&
              displayedLibraryPlaylists.length === 0
            ? props.text.dreamFm.searchEmpty
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
        props.text.dreamFm.groupPlaylist
      : onlineQueueState.kind === "radio"
        ? onlineQueueState.title || props.text.dreamFm.groupRadio
        : props.text.dreamFm.upNext;
  const effectiveSidebarView: DreamFMSidebarView =
    mode === "online" ? sidebarView : "browse";
  const showArtistDetail =
    mode === "online" &&
    effectiveSidebarView === "browse" &&
    artistBrowsePage !== null &&
    browsePlaylistId === "";
  const showPlaylistDetail =
    mode === "online" &&
    effectiveSidebarView === "browse" &&
    browsePlaylistId !== "";
  const searchPlaceholder =
    mode === "live"
      ? props.text.dreamFm.searchLive
      : mode === "online"
        ? props.text.dreamFm.searchOnline
        : props.text.dreamFm.searchLocal;

  return (
    <DreamFMPageView
      page={props}
      state={{ isWindows, isMac, listOpen, query, searchPlaceholder, mode, sidebarView, effectiveSidebarView, onlineBrowseSource, onlineBrowseDetail, liveGroups, selectedLiveGroupId, liveStatusByVideoId, liveCatalogLoading, liveCatalogError, liveCatalogMessage, curatedLiveItems, liveSelectionArmed, selectedLiveId, filteredOnlineQueueItems, onlineQueueTitle, selectedOnlineId, filteredLocalTracks, selectedLocalId, localPlaying, liveSearchNotice, showArtistDetail, artistBrowsePage, artistActionBusy, filteredArtistShelves, browsePlaylistId, savedPlaylistIds, playlistMutationAction, playlistMutationPlaylistId, filteredArtistTracks, showPlaylistDetail, selectedPlaylist, playlistLoading, playlistAppending, playlistTracks, filteredPlaylistTracks, playlistContinuation, normalizedQuery, libraryLoading, libraryAppending, libraryError, libraryErrorCode, searchItems, searchArtists, searchPlaylists, libraryArtists, displayedLibraryPlaylists, showLibraryPlaylistGroup, homeShelves, libraryContinuation, onlineSearchNotice, localTracks, localTracksLoading: localTrackIndex.loading, localTracksRefreshing: localTrackIndex.refreshing, localTracksClearingMissing: localTrackIndex.clearingMissing, activeOnline, selectedLocal, onlinePlayerCommand, localPlayerCommand, onlineQueueItems: mode === "live" ? liveQueue : onlinePlaybackQueue, onlinePlaying, onlinePlaybackArmed, selectedLocalResumeTime, activeOnlineResumeTime, onlineProgress, onlineState, activeOnlineFavorite, activeOnlineFavoriteBusy, localProgress, muted, volume, playMode }}
      actions={{ setListOpen, setQuery, selectFirstResult, setMode, setSidebarView, setSelectedLiveGroupId, reloadLiveCatalog, reloadLibrary: () => setLibraryReloadToken((current) => current + 1), changeOnlineBrowseSource, openOnlineBrowseCategory, closeOnlineBrowseDetail, loadMoreLibrary, activateLiveSelection, selectOnlineQueueTrack, selectLocalQueueTrack, setSelectedLocalId, setLocalPlayerCommand, closeArtistBrowse, playArtistFromIndex, shuffleArtist, loadMoreArtist, playArtistMix, toggleArtistSubscription, openPlaylistBrowse, updatePlaylistLibrary, setBrowsePlaylistId, playPlaylistFromIndex, loadMorePlaylist, playOnlineShelfTrack, playOnlineShelfAll, shuffleOnlineShelf, playOnlineSearchTrack, playOnlineSearchResults, shuffleOnlineSearchResults, openSearchArtistBrowse, clearOnlineQueue, removeOnlineQueueItem, refreshLocalTracks: localTrackIndex.refresh, clearMissingLocalTracks: localTrackIndex.clearMissing, handlePlaybackEnded, setOnlinePlaying, setOnlineState, handleOnlineProgressChange, handleOnlineNativeTrackChange, setLocalPlaying, handleLocalProgressChange, setPlaybackSessionStarted, playPrevious, playNext, togglePlayMode, setPlayMode, togglePlayback, toggleMute, handleVolumeChange, toggleOnlineFavorite, openOnlineArtistBrowse, openSelectedLocalDirectory }}
    />
  );

}
