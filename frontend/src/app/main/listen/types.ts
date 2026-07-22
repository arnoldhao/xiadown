import {
getXiaText
} from "@/features/xiadown/shared";
import type { LibraryDTO } from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";


export type ListenMode = "hush" | "muse" | "linger";
export type ListenPlayerCompanionMode = "player" | "lyrics" | "queue";
export type ListenPlayerPresentation = "page" | "companion" | "fullscreen";
export type ListenSidebarView = "browse" | "queue";
export type ListenOnlineBrowseSource =
  | "home"
  | "explore"
  | "charts"
  | "moods"
  | "new"
  | "podcasts"
  | "recent"
  | "playlists"
  | "history";
export type ListenOnlineBrowseDetail = {
  id: string;
  source: ListenOnlineBrowseSource;
  browseId: string;
  params: string;
  title: string;
};
export type ListenOnlineGroup = "live" | "playlist";
export type ListenLiveStatusValue =
  | "checking"
  | "live"
  | "offline"
  | "upcoming"
  | "unavailable"
  | "unknown";
export type ListenLiveStatus = {
  videoId: string;
  status: ListenLiveStatusValue;
  detail?: string;
};
export type ListenLivePlaybackKind = "youtube_music" | "youtube" | "stream" | "hls";
export type ListenLivePlayback = {
  kind: ListenLivePlaybackKind;
  videoId?: string;
  url?: string;
};
export type ListenPlayMode = "order" | "repeat" | "shuffle";
export type ListenObservedPlaybackAudioQuality =
  | "AUDIO_QUALITY_LOW"
  | "AUDIO_QUALITY_MEDIUM"
  | "AUDIO_QUALITY_HIGH";
export type ListenPlayerCommand = {
  id: number;
  command: "play" | "pause" | "replay" | "resume" | "seek";
  startSeconds?: number;
  forceReload?: boolean;
};
export type ListenNativePlayerEvent = {
  source?: string;
  provider?: "stream" | "youtube" | "youtube_music" | "local";
  sessionId?: string;
  type?: string;
	active?: boolean;
  category?: string;
  action?: string;
  details?: Record<string, unknown>;
  state?: ListenRemotePlaybackState;
  reason?: string;
  videoId?: string;
  observedVideoId?: string;
  requestedVideoId?: string;
  title?: string;
  artist?: string;
  thumbnailUrl?: string;
  likeStatus?: string;
  trackChanged?: boolean;
  metadataSource?: string;
  currentTime?: number;
  playbackRate?: number;
  duration?: number;
  bufferedTime?: number;
  paused?: boolean;
  ended?: boolean;
  videoWidth?: number;
  videoHeight?: number;
  advertising?: boolean;
  ad?: boolean;
  adLabel?: string;
  errorCode?: string;
  errorMessage?: string;
  readyState?: number;
  networkState?: number;
  url?: string;
  code?: number | string;
  message?: string;
};
export type ListenLibraryShelfKind =
  | "tracks"
  | "playlists"
  | "categories"
  | "artists";
export type ListenPlaylistLibraryAction = "add" | "remove";
export type ListenOnlineQueueKind = "none" | "radio" | "playlist";
export type ListenRemotePlaybackState =
  | "idle"
  | "loading"
  | "playing"
  | "paused"
  | "buffering"
  | "ended"
  | "error";
export type ListenPlaybackProgressState = {
  currentTime: number;
  duration: number;
  bufferedTime: number;
};
export type ListenLyricsKind = "synced" | "plain" | "unavailable";
export type ListenLyricTimingQuality =
  | "plain"
  | "line"
  | "word"
  | "syllable"
  | "estimated";
export type ListenLyricAlternateText = {
  role: string;
  language?: string;
  text: string;
};
export type ListenLyricWord = {
  startMs: number;
  endMs?: number;
  text: string;
  endsWithSpace?: boolean;
  syllables?: ListenLyricWord[];
};
export type ListenLyricLine = {
  startMs: number;
  durationMs: number;
  endEstimated?: boolean;
  text: string;
  translationText?: string;
  romanizedText?: string;
  romanizedKind?: "romanized" | "pinyin";
  alternateTexts?: ListenLyricAlternateText[];
  words?: ListenLyricWord[];
};
export type ListenLyricsData = {
  videoId: string;
  kind: ListenLyricsKind;
  /** Legacy display label retained for older API and Wails clients. */
  source: string;
  providerId?: string;
  providerTrackId?: string;
  attribution?: string;
  timingQuality?: ListenLyricTimingQuality;
  confidence?: number;
  text: string;
  lines: ListenLyricLine[];
};
export type ListenLyricsCandidate = {
  providerId: string;
  providerTrackId: string;
  title: string;
  artist: string;
  album?: string;
  durationSeconds?: number;
  instrumental?: boolean;
  hasSynced?: boolean;
  hasPlain?: boolean;
  timingQuality?: ListenLyricTimingQuality;
  attribution?: string;
  confidence: number;
  titleScore: number;
  artistScore: number;
  albumScore: number;
  durationScore: number;
  durationDiff?: number;
  accepted: boolean;
  rejection?: string;
};
export type ListenOnlineQueueState =
  | { kind: "none"; title: string; items: ListenOnlineItem[] }
  | {
      kind: "radio";
      title: string;
      items: ListenOnlineItem[];
      seedVideoId: string;
    }
  | {
      kind: "playlist";
      title: string;
      items: ListenOnlineItem[];
      playlistId: string;
    };
export type ListenArtistBrowseState = {
  id: string;
  name: string;
  title: string;
  subtitle: string;
  description: string;
  thumbnailUrl?: string;
  heroThumbnailUrl?: string;
  channelId: string;
  isSubscribed: boolean;
  mixPlaylistId: string;
  mixVideoId: string;
  items: ListenOnlineItem[];
  shelves: ListenLibraryShelf[];
  continuation: string;
  loading: boolean;
  appending: boolean;
  error: boolean;
};
export type ListenStorageState = {
  version: 2;
  mode: ListenMode;
  playbackMode: ListenMode;
  listOpen: boolean;
  playMode: ListenPlayMode;
  selectedLiveId: string;
  selectedOnlineId: string;
  browsePlaylistId: string;
  selectedLocalId: string;
  localPlaybackQueueIds: string[] | null;
  onlineQueueKind: ListenOnlineQueueKind;
  onlineQueueTitle: string;
  onlineQueueSeedVideoId: string;
  onlineQueuePlaylistId: string;
  onlineQueueItems: ListenOnlineItem[];
  muted: boolean;
  volume: number;
  localProgressByPath: Record<string, number>;
  onlineProgressByVideoId: Record<string, number>;
};

export type ListenPlaylistItem = {
  id: string;
  playlistId: string;
  title: string;
  channel: string;
  description: string;
  thumbnailUrl?: string;
};

export type ListenArtistItem = {
  id: string;
  browseId: string;
  name: string;
  subtitle: string;
  thumbnailUrl?: string;
};

export type ListenTrackArtist = {
  name: string;
  browseId?: string;
  thumbnailUrl?: string;
};

export type ListenCategoryItem = {
  id: string;
  browseId: string;
  params: string;
  title: string;
  colorHex?: string;
  thumbnailUrl?: string;
};

export type ListenOnlineItem = {
  id: string;
  group: ListenOnlineGroup;
  source?: string;
  videoId: string;
  title: string;
  channel: string;
  artists?: ListenTrackArtist[];
  artistBrowseId?: string;
  artistSource?: string;
  description: string;
  durationLabel: string;
  durationSeconds?: number;
  playCountLabel?: string;
  thumbnailUrl?: string;
  musicVideoType?: string;
  hasVideo?: boolean;
  videoAvailabilityKnown?: boolean;
  playback?: ListenLivePlayback;
};

export type ListenLiveGroup = {
  id: string;
  title: string;
  items: ListenOnlineItem[];
};

export type ListenLiveCatalog = {
  schemaVersion: number;
  id: string;
  version: string;
  updatedAt: string;
  ttlSeconds: number;
  groups: ListenLiveGroup[];
};

export type ListenLibraryShelf = {
  id: string;
  title: string;
  kind: ListenLibraryShelfKind;
  continuation: string;
  browseId: string;
  params: string;
  tracks: ListenOnlineItem[];
  playlists: ListenPlaylistItem[];
  categories: ListenCategoryItem[];
  artists: ListenArtistItem[];
};

export type ListenSearchItemDTO = {
  id: string;
  group: string;
  source?: string;
  videoId: string;
  title: string;
  channel: string;
  artists?: ListenTrackArtist[];
  artistBrowseId?: string;
  artistSource?: string;
  description: string;
  durationLabel: string;
  playCountLabel?: string;
  thumbnailUrl?: string;
  musicVideoType?: string;
  hasVideo?: boolean;
  videoAvailabilityKnown?: boolean;
  playback?: Partial<ListenLivePlayback>;
};

export type ListenLiveCatalogDTO = {
  schemaVersion?: number;
  id?: string;
  version?: string;
  updatedAt?: string;
  ttlSeconds?: number;
  groups?: ListenLiveGroupDTO[];
};

export type ListenLiveStatusResponseDTO = {
  statuses?: ListenLiveStatusDTO[];
};

export type ListenLiveStatusDTO = {
  videoId?: string;
  status?: string;
  detail?: string;
};

export type ListenLiveGroupDTO = {
  id?: string;
  title?: string;
  items?: ListenSearchItemDTO[];
};

export type ListenSearchResponseDTO = {
  items?: ListenSearchItemDTO[];
  artists?: ListenArtistItemDTO[];
  playlists?: ListenPlaylistItemDTO[];
  continuation?: string;
  title?: string;
  author?: string;
  authorBrowseId?: string;
  trackCountLabel?: string;
  durationLabel?: string;
  description?: string;
  thumbnailUrl?: string;
};

export type ListenTrackResponseDTO = {
  item?: ListenSearchItemDTO;
};

export type ListenLyricsResponseDTO = {
  videoId?: string;
  kind?: string;
  source?: string;
  providerId?: string;
  providerTrackId?: string;
  attribution?: string;
  timingQuality?: string;
  confidence?: number;
  text?: string;
  lines?: ListenLyricLine[];
};

export type ListenArtistResponseDTO = {
  id?: string;
  title?: string;
  subtitle?: string;
  description?: string;
  thumbnailUrl?: string;
  heroThumbnailUrl?: string;
  channelId?: string;
  isSubscribed?: boolean;
  mixPlaylistId?: string;
  mixVideoId?: string;
  items?: ListenSearchItemDTO[];
  shelves?: ListenLibraryShelfDTO[];
  continuation?: string;
};

export type ListenPlaylistItemDTO = {
  id: string;
  playlistId: string;
  title: string;
  channel: string;
  description: string;
  thumbnailUrl?: string;
};

export type ListenArtistItemDTO = {
  id: string;
  browseId: string;
  name: string;
  subtitle: string;
  thumbnailUrl?: string;
};

export type ListenLibraryResponseDTO = {
  playlists?: ListenPlaylistItemDTO[];
  artists?: ListenArtistItemDTO[];
  recommendations?: ListenSearchItemDTO[];
  shelves?: ListenLibraryShelfDTO[];
  continuation?: string;
};

export type ListenLibraryShelfDTO = {
  id: string;
  title: string;
  kind: string;
  continuation?: string;
  browseId?: string;
  params?: string;
  tracks?: ListenSearchItemDTO[];
  playlists?: ListenPlaylistItemDTO[];
  categories?: ListenCategoryItemDTO[];
  artists?: ListenArtistItemDTO[];
};

export type ListenCategoryItemDTO = {
  id: string;
  browseId: string;
  params?: string;
  title: string;
  colorHex?: string;
  thumbnailUrl?: string;
};

export type ListenPlaylistLibraryResponseDTO = {
  ok?: boolean;
};

export type ListenTrackFavoriteResponseDTO = {
  ok?: boolean;
  videoId?: string;
  liked?: boolean;
  known?: boolean;
  favorites?: ListenTrackFavoriteItemDTO[];
};

export type ListenTrackFavoriteItemDTO = {
  videoId?: string;
  liked?: boolean;
  known?: boolean;
};

export type ListenArtistSubscriptionResponseDTO = {
  ok?: boolean;
  subscribed?: boolean;
};

export type ListenLocalPreviewTrack = {
  id: string;
  title: string;
  author?: string;
  path: string;
  previewURL: string;
  coverURL?: string;
};

export type ListenLocalItem = {
  id: string;
  title: string;
  author: string;
  album: string;
  albumArtist: string;
  genre: string;
  trackNumber: number;
  discNumber: number;
  year: number;
  lyricsTitle: string;
  lyricsArtist: string;
  path: string;
  previewURL: string;
  durationLabel: string;
  durationSeconds: number;
  coverURL: string;
  format: string;
  audioCodec: string;
  sizeBytes: number;
  metadataWritable: boolean;
  playbackSupported: boolean;
  playbackUnsupportedReason: string;
  probeError: string;
  modTimeUnix: number;
  createdAtUnix: number;
};

export type ListenNowPlayingState =
  | "idle"
  | "loading"
  | "playing"
  | "paused"
  | "error";

/**
 * Identifies the playback surface that owns the active session. Keep this
 * separate from ListenMode: a workspace mode describes navigation, while the
 * source remains stable when the user switches between Online and Local.
 */
export type ListenPlaybackSource =
  | "youtube_music"
  | "radio"
  | "local"
  | "youtube"
  | "library_preview"
  | "unknown";

export type ListenNowPlayingStatus = {
  state: ListenNowPlayingState;
  /** Explicit stream semantic; do not infer Live from an arbitrary duration. */
  live?: boolean;
  /** Stable media identity used to reject stale metadata during handoffs. */
  mediaId?: string;
  title: string;
  subtitle: string;
  /** Structured artist targets for surfaces that support per-artist navigation. */
  artists?: ListenTrackArtist[];
  artworkURL: string;
  /** Ordered artwork fallbacks. The default cover, when present, stays last. */
  artworkCandidates?: string[];
  playbackSource?: ListenPlaybackSource;
  /** Localized source label shown by compact player surfaces. */
  playbackSourceLabel?: string;
  mode: ListenMode;
  canControl: boolean;
  canPrevious?: boolean;
  canNext?: boolean;
  progress: ListenPlaybackProgressState;
  muted?: boolean;
  volume?: number;
  sourceURL?: string;
  favoriteActive?: boolean;
  canFavorite?: boolean;
};

export type ListenExternalCommand = {
  id: number;
  command:
    | "toggle"
    | "play"
    | "pause"
    | "stop"
    | "previous"
    | "next"
    | "shuffle"
    | "repeat"
    | "favorite"
    | "toggle-mute"
    | "set-volume"
    | "seek"
    | "show-lyrics"
    | "show-queue"
    | "show-video"
    | "open-artist";
  value?: number;
  /** Set only after the coordinator has already closed the transport. */
  backendStopped?: boolean;
  /** Artist selected by a multi-artist playback surface. */
  artist?: ListenTrackArtist;
};

export type ListenPageProps = {
  text: ReturnType<typeof getXiaText>;
  libraries: LibraryDTO[];
  httpBaseURL: string;
  pet: Pet | null;
  petImageURL: string;
  active: boolean;
  workspaceLayout?: boolean;
  workspaceRouteId?: string;
  /** Native caption controls currently occupy the trailing edge of Primary. */
  reserveWindowControls?: boolean;
  playerPortalTarget?: HTMLElement | null;
  playerFullscreen?: boolean;
  /**
   * Defines the chrome contract independently from the selected companion
   * content. Workspace pages default to `companion`; legacy Listen pages use
   * `page`, and `playerFullscreen` remains a backwards-compatible override.
   */
  playerPresentation?: ListenPlayerPresentation;
  /**
   * Selects the presentation hosted by the workspace companion portal.
   * Full-screen playback always uses the player presentation.
   */
  playerCompanionMode?: ListenPlayerCompanionMode;
  /**
   * Whether the current presentation can safely host a native video surface.
   * An explicit true keeps the global Companion or fullscreen player runtime
   * active even outside the Music workspace. False detaches native underlays
   * and visualizers so a hidden portal cannot punch holes through AppShell.
   * Docked Companion panels deliberately use the safe cover presentation; the
   * native video surface remains reserved for the full-screen overlay.
   */
  playerSurfaceVisible?: boolean;
  /** Opens the workspace destination represented by the active playback source. */
  onOpenPlaybackSource?: (source: ListenPlaybackSource) => void;
  /** Promotes the docked Now Playing surface to the fullscreen player. */
  onRequestPlayerFullscreen?: () => void;
  /** Returns the fullscreen player to the docked Now Playing surface. */
  onExitPlayerFullscreen?: () => void;
  className?: string;
  controlCommand?: ListenExternalCommand | null;
  onNowPlayingChange?: (status: ListenNowPlayingStatus) => void;
  onOpenConnections: () => void;
  onDownloadTrack: (url: string) => void;
};
