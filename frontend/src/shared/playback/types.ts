export type PlaybackProvider =
  | "youtube_music"
  | "youtube"
  | "local"
  | "stream";

export type PlaybackMediaKind = "audio" | "video";
export type PlaybackFocus = "persistent" | "transient_preview";
export type PlaybackState =
  | "idle"
  | "loading"
  | "playing"
  | "paused"
  | "buffering"
  | "ended"
  | "error";

export type PlaybackPreviewResumePolicy =
  | "resume_if_previously_playing"
  | "keep_persistent_paused";

export type PlaybackSource = {
  provider: PlaybackProvider;
  id?: string;
  uri?: string;
  live?: boolean;
};

export type PlaybackMediaItem = {
  id: string;
  kind: PlaybackMediaKind;
  source: PlaybackSource;
  title: string;
  artist?: string;
  artists?: string[];
  artworkUrl?: string;
  canonicalUrl?: string;
  duration?: number;
  metadata?: Record<string, string>;
};

export type PlaybackCapabilities = {
  available: boolean;
  unsupportedReason?: string;
  mediaKinds: PlaybackMediaKind[];
  playPause: boolean;
  stop: boolean;
  seek: boolean;
  previous: boolean;
  next: boolean;
  volume: boolean;
  queue: boolean;
  shuffle: boolean;
  repeat: boolean;
  lyrics: boolean;
  video: boolean;
  like: boolean;
  dislike: boolean;
  captions: boolean;
  audioTracks: boolean;
  quality: boolean;
  fullscreen: boolean;
};

export type PlaybackSessionRequest = {
  sessionId?: string;
  focus?: PlaybackFocus;
  item: PlaybackMediaItem;
  startSeconds?: number;
  volume?: number;
  muted?: boolean;
  forceReload?: boolean;
  previewResumePolicy?: PlaybackPreviewResumePolicy;
};

export type PlaybackSessionSnapshot = {
  id: string;
  focus: PlaybackFocus;
  state: PlaybackState;
  errorMessage?: string;
  item: PlaybackMediaItem;
  capabilities: PlaybackCapabilities;
  position: number;
  duration: number;
  volume: number;
  muted: boolean;
  queue: PlaybackMediaItem[];
  currentIndex: number;
  shuffleEnabled: boolean;
  repeatMode: "off" | "all" | "one";
};

export type PlaybackSnapshot = {
  version: number;
  audibleSessionId?: string;
  active: PlaybackSessionSnapshot | null;
  suspendedPersistent: PlaybackSessionSnapshot | null;
};

export const EMPTY_PLAYBACK_SNAPSHOT: PlaybackSnapshot = {
  version: 0,
  active: null,
  suspendedPersistent: null,
};
