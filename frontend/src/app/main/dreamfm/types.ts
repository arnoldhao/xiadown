import {
getXiaText
} from "@/features/xiadown/shared";
import type { LibraryDTO } from "@/shared/contracts/library";
import type { Sprite } from "@/shared/contracts/sprites";


export type DreamFMMode = "live" | "online" | "local";
export type DreamFMSidebarView = "browse" | "queue";
export type DreamFMOnlineBrowseSource =
  | "home"
  | "explore"
  | "charts"
  | "moods"
  | "new"
  | "history";
export type DreamFMOnlineBrowseDetail = {
  id: string;
  source: DreamFMOnlineBrowseSource;
  browseId: string;
  params: string;
  title: string;
};
export type DreamFMOnlineGroup = "live" | "playlist";
export type DreamFMLiveStatusValue =
  | "checking"
  | "live"
  | "offline"
  | "upcoming"
  | "unavailable"
  | "unknown";
export type DreamFMLiveStatus = {
  videoId: string;
  status: DreamFMLiveStatusValue;
  detail?: string;
};
export type DreamFMLivePlaybackKind = "youtube_music" | "youtube" | "stream" | "hls";
export type DreamFMLivePlayback = {
  kind: DreamFMLivePlaybackKind;
  videoId?: string;
  url?: string;
};
export type DreamFMPlayMode = "order" | "repeat" | "shuffle";
export type DreamFMPlayerCommand = {
  id: number;
  command: "play" | "pause" | "replay";
};
export type DreamFMNativePlayerEvent = {
  source?: string;
  type?: string;
  state?: DreamFMRemotePlaybackState;
  reason?: string;
  videoId?: string;
  observedVideoId?: string;
  requestedVideoId?: string;
  title?: string;
  artist?: string;
  thumbnailUrl?: string;
  likeStatus?: string;
  videoAvailable?: boolean;
  videoAvailabilityKnown?: boolean;
  trackChanged?: boolean;
  metadataSource?: string;
  currentTime?: number;
  duration?: number;
  bufferedTime?: number;
  advertising?: boolean;
  ad?: boolean;
  adLabel?: string;
  adSkippable?: boolean;
  adSkipLabel?: string;
  errorCode?: string;
  errorMessage?: string;
  readyState?: number;
  networkState?: number;
  url?: string;
  code?: number | string;
  message?: string;
};
export type DreamFMLibraryShelfKind =
  | "tracks"
  | "playlists"
  | "categories"
  | "artists";
export type DreamFMPlaylistLibraryAction = "add" | "remove";
export type DreamFMOnlineQueueKind = "none" | "radio" | "playlist";
export type DreamFMRemotePlaybackState =
  | "idle"
  | "loading"
  | "playing"
  | "paused"
  | "buffering"
  | "ended"
  | "error";
export type DreamFMPlaybackProgressState = {
  currentTime: number;
  duration: number;
  bufferedTime: number;
};
export type DreamFMLyricsKind = "synced" | "plain" | "unavailable";
export type DreamFMLyricWord = {
  startMs: number;
  text: string;
};
export type DreamFMLyricLine = {
  startMs: number;
  durationMs: number;
  text: string;
  words?: DreamFMLyricWord[];
};
export type DreamFMLyricsData = {
  videoId: string;
  kind: DreamFMLyricsKind;
  source: string;
  text: string;
  lines: DreamFMLyricLine[];
};
export type DreamFMOnlineQueueState =
  | { kind: "none"; title: string; items: DreamFMOnlineItem[] }
  | {
      kind: "radio";
      title: string;
      items: DreamFMOnlineItem[];
      seedVideoId: string;
    }
  | {
      kind: "playlist";
      title: string;
      items: DreamFMOnlineItem[];
      playlistId: string;
    };
export type DreamFMArtistBrowseState = {
  id: string;
  name: string;
  title: string;
  subtitle: string;
  channelId: string;
  isSubscribed: boolean;
  mixPlaylistId: string;
  mixVideoId: string;
  items: DreamFMOnlineItem[];
  shelves: DreamFMLibraryShelf[];
  continuation: string;
  loading: boolean;
  appending: boolean;
  error: boolean;
};
export type DreamFMStorageState = {
  version: 1;
  mode: DreamFMMode;
  listOpen: boolean;
  playMode: DreamFMPlayMode;
  selectedLiveId: string;
  selectedOnlineId: string;
  browsePlaylistId: string;
  selectedLocalId: string;
  onlineQueueKind: DreamFMOnlineQueueKind;
  onlineQueueTitle: string;
  onlineQueueSeedVideoId: string;
  onlineQueuePlaylistId: string;
  muted: boolean;
  volume: number;
  localProgressByPath: Record<string, number>;
  onlineProgressByVideoId: Record<string, number>;
};

export type DreamFMPlaylistItem = {
  id: string;
  playlistId: string;
  title: string;
  channel: string;
  description: string;
  thumbnailUrl?: string;
};

export type DreamFMArtistItem = {
  id: string;
  browseId: string;
  name: string;
  subtitle: string;
  thumbnailUrl?: string;
};

export type DreamFMCategoryItem = {
  id: string;
  browseId: string;
  params: string;
  title: string;
  colorHex?: string;
  thumbnailUrl?: string;
};

export type DreamFMOnlineItem = {
  id: string;
  group: DreamFMOnlineGroup;
  source?: string;
  videoId: string;
  title: string;
  channel: string;
  artistBrowseId?: string;
  description: string;
  durationLabel: string;
  playCountLabel?: string;
  thumbnailUrl?: string;
  musicVideoType?: string;
  hasVideo?: boolean;
  videoAvailabilityKnown?: boolean;
  playback?: DreamFMLivePlayback;
};

export type DreamFMLiveGroup = {
  id: string;
  title: string;
  items: DreamFMOnlineItem[];
};

export type DreamFMLiveCatalog = {
  schemaVersion: number;
  id: string;
  version: string;
  updatedAt: string;
  ttlSeconds: number;
  groups: DreamFMLiveGroup[];
};

export type DreamFMLibraryShelf = {
  id: string;
  title: string;
  kind: DreamFMLibraryShelfKind;
  tracks: DreamFMOnlineItem[];
  playlists: DreamFMPlaylistItem[];
  categories: DreamFMCategoryItem[];
  podcasts: DreamFMPlaylistItem[];
  artists: DreamFMArtistItem[];
};

export type DreamFMSearchItemDTO = {
  id: string;
  group: string;
  source?: string;
  videoId: string;
  title: string;
  channel: string;
  artistBrowseId?: string;
  description: string;
  durationLabel: string;
  playCountLabel?: string;
  thumbnailUrl?: string;
  musicVideoType?: string;
  hasVideo?: boolean;
  videoAvailabilityKnown?: boolean;
  playback?: Partial<DreamFMLivePlayback>;
};

export type DreamFMLiveCatalogDTO = {
  schemaVersion?: number;
  id?: string;
  version?: string;
  updatedAt?: string;
  ttlSeconds?: number;
  groups?: DreamFMLiveGroupDTO[];
};

export type DreamFMLiveStatusResponseDTO = {
  statuses?: DreamFMLiveStatusDTO[];
};

export type DreamFMLiveStatusDTO = {
  videoId?: string;
  status?: string;
  detail?: string;
};

export type DreamFMLiveGroupDTO = {
  id?: string;
  title?: string;
  items?: DreamFMSearchItemDTO[];
};

export type DreamFMSearchResponseDTO = {
  items?: DreamFMSearchItemDTO[];
  artists?: DreamFMArtistItemDTO[];
  playlists?: DreamFMPlaylistItemDTO[];
  continuation?: string;
  title?: string;
  author?: string;
};

export type DreamFMTrackResponseDTO = {
  item?: DreamFMSearchItemDTO;
};

export type DreamFMLyricsResponseDTO = {
  videoId?: string;
  kind?: string;
  source?: string;
  text?: string;
  lines?: DreamFMLyricLine[];
};

export type DreamFMArtistResponseDTO = {
  id?: string;
  title?: string;
  subtitle?: string;
  channelId?: string;
  isSubscribed?: boolean;
  mixPlaylistId?: string;
  mixVideoId?: string;
  items?: DreamFMSearchItemDTO[];
  shelves?: DreamFMLibraryShelfDTO[];
  continuation?: string;
};

export type DreamFMPlaylistItemDTO = {
  id: string;
  playlistId: string;
  title: string;
  channel: string;
  description: string;
  thumbnailUrl?: string;
};

export type DreamFMArtistItemDTO = {
  id: string;
  browseId: string;
  name: string;
  subtitle: string;
  thumbnailUrl?: string;
};

export type DreamFMLibraryResponseDTO = {
  playlists?: DreamFMPlaylistItemDTO[];
  artists?: DreamFMArtistItemDTO[];
  podcasts?: DreamFMPlaylistItemDTO[];
  recommendations?: DreamFMSearchItemDTO[];
  shelves?: DreamFMLibraryShelfDTO[];
  continuation?: string;
};

export type DreamFMLibraryShelfDTO = {
  id: string;
  title: string;
  kind: string;
  tracks?: DreamFMSearchItemDTO[];
  playlists?: DreamFMPlaylistItemDTO[];
  categories?: DreamFMCategoryItemDTO[];
  podcasts?: DreamFMPlaylistItemDTO[];
  artists?: DreamFMArtistItemDTO[];
};

export type DreamFMCategoryItemDTO = {
  id: string;
  browseId: string;
  params?: string;
  title: string;
  colorHex?: string;
  thumbnailUrl?: string;
};

export type DreamFMPlaylistLibraryResponseDTO = {
  ok?: boolean;
};

export type DreamFMTrackFavoriteResponseDTO = {
  ok?: boolean;
  videoId?: string;
  liked?: boolean;
  known?: boolean;
  favorites?: DreamFMTrackFavoriteItemDTO[];
};

export type DreamFMTrackFavoriteItemDTO = {
  videoId?: string;
  liked?: boolean;
  known?: boolean;
};

export type DreamFMArtistSubscriptionResponseDTO = {
  ok?: boolean;
  subscribed?: boolean;
};

export type DreamFMLocalPreviewTrack = {
  id: string;
  title: string;
  author?: string;
  path: string;
  previewURL: string;
  coverURL?: string;
};

export type DreamFMLocalItem = {
  id: string;
  title: string;
  author: string;
  lyricsTitle: string;
  lyricsArtist: string;
  path: string;
  previewURL: string;
  durationLabel: string;
  coverURL: string;
};

export type DreamFMNowPlayingState =
  | "idle"
  | "loading"
  | "playing"
  | "paused"
  | "error";

export type DreamFMNowPlayingStatus = {
  state: DreamFMNowPlayingState;
  title: string;
  subtitle: string;
  artworkURL: string;
  mode: DreamFMMode;
  canControl: boolean;
  progress: DreamFMPlaybackProgressState;
};

export type DreamFMExternalCommand = {
  id: number;
  command: "toggle" | "play" | "pause" | "previous" | "next";
};

export type DreamFMPageProps = {
  text: ReturnType<typeof getXiaText>;
  libraries: LibraryDTO[];
  httpBaseURL: string;
  sprite: Sprite | null;
  spriteImageURL: string;
  active: boolean;
  className?: string;
  controlCommand?: DreamFMExternalCommand | null;
  onNowPlayingChange?: (status: DreamFMNowPlayingStatus) => void;
  onOpenConnections: () => void;
  onDownloadTrack: (url: string) => void;
};
