import {
getXiaText
} from "@/features/xiadown/shared";
import type { LibraryDTO } from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";


export type ListenMode = "hush" | "muse" | "linger";
export type ListenSidebarView = "browse" | "queue";
export type ListenOnlineBrowseSource =
  | "home"
  | "explore"
  | "charts"
  | "moods"
  | "new"
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
  type?: string;
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
export type ListenLyricWord = {
  startMs: number;
  text: string;
};
export type ListenLyricLine = {
  startMs: number;
  durationMs: number;
  text: string;
  romanizedText?: string;
  romanizedKind?: "romanized" | "pinyin";
  words?: ListenLyricWord[];
};
export type ListenLyricsData = {
  videoId: string;
  kind: ListenLyricsKind;
  source: string;
  text: string;
  lines: ListenLyricLine[];
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
  thumbnailUrl?: string;
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
};

export type ListenTrackResponseDTO = {
  item?: ListenSearchItemDTO;
};

export type ListenLyricsResponseDTO = {
  videoId?: string;
  kind?: string;
  source?: string;
  text?: string;
  lines?: ListenLyricLine[];
};

export type ListenArtistResponseDTO = {
  id?: string;
  title?: string;
  subtitle?: string;
  thumbnailUrl?: string;
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
  lyricsTitle: string;
  lyricsArtist: string;
  path: string;
  previewURL: string;
  durationLabel: string;
  coverURL: string;
  modTimeUnix: number;
};

export type ListenNowPlayingState =
  | "idle"
  | "loading"
  | "playing"
  | "paused"
  | "error";

export type ListenNowPlayingStatus = {
  state: ListenNowPlayingState;
  title: string;
  subtitle: string;
  artworkURL: string;
  mode: ListenMode;
  canControl: boolean;
  progress: ListenPlaybackProgressState;
};

export type ListenExternalCommand = {
  id: number;
  command: "toggle" | "play" | "pause" | "previous" | "next";
};

export type ListenPageProps = {
  text: ReturnType<typeof getXiaText>;
  libraries: LibraryDTO[];
  httpBaseURL: string;
  pet: Pet | null;
  petImageURL: string;
  active: boolean;
  className?: string;
  controlCommand?: ListenExternalCommand | null;
  onNowPlayingChange?: (status: ListenNowPlayingStatus) => void;
  onOpenConnections: () => void;
  onDownloadTrack: (url: string) => void;
};
