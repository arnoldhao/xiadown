import {
MediaPlayer,
MediaProvider,
MediaRemoteControl,
getTimeRangesEnd
} from "@vidstack/react";
import { Call,Events } from "@wailsio/runtime";
import {
Airplay,
Captions,
Copy,
Download,
ExternalLink,
FolderOpen,
Heart,
ListMusic,
Loader2,
MoreHorizontal,
Pause,
Play,
Repeat2,
RotateCcw,
Shuffle,
SkipBack,
SkipForward,
Trash2,
Video,
Volume2,
VolumeX,
X
} from "lucide-react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { Sprite } from "@/shared/contracts/sprites";
import { messageBus } from "@/shared/message";
import { openExternalURL } from "@/shared/query/system";
import {
DropdownMenu,
DropdownMenuContent,
DropdownMenuItem,
DropdownMenuTrigger
} from "@/shared/ui/dropdown-menu";
import { SpriteDisplay } from "@/shared/ui/sprite-player";
import {
Tooltip,
TooltipContent,
TooltipProvider,
TooltipTrigger
} from "@/shared/ui/tooltip";
import {
DREAM_FM_DROPDOWN_CONTENT_CLASS,
DREAM_FM_DROPDOWN_ICON_SLOT_CLASS,
DREAM_FM_DROPDOWN_ITEM_CLASS,
DREAM_FM_HIDDEN_ENGINE_STYLE,
DREAM_FM_PLAYER_FOOTER_ICON_BUTTON_CLASS,
DREAM_FM_PLAYER_ICON_BUTTON_CLASS,
DREAM_FM_PLAYER_SURFACE_WIDTH_CLASS,
} from "@/shared/styles/dreamfm";

import { DREAM_FM_LIVE_PLAYER_EVENT,DREAM_FM_LIVE_PLAYER_SERVICE,DREAM_FM_NATIVE_PLAYER_EVENT,DREAM_FM_NATIVE_PLAYER_SERVICE } from "@/app/main/dreamfm/catalog";
import { fetchDreamFMTrackInfo,fetchDreamFMTrackLyrics,getDreamFMErrorCode,getDreamFMErrorMessage,getDreamFMErrorRetryable } from "@/app/main/dreamfm/api";
import { clampVolume,formatProgressSeconds,resolveAudioSource } from "@/app/main/dreamfm/local-library";
import { buildDreamFMPosterCandidates,buildYouTubeWatchURL } from "@/app/main/dreamfm/storage";
import type { DreamFMLocalItem,DreamFMLyricsData,DreamFMMode,DreamFMNativePlayerEvent,DreamFMOnlineItem,DreamFMPlayMode,DreamFMPlayerCommand,DreamFMRemotePlaybackState } from "@/app/main/dreamfm/types";
import { DreamFMArtworkShell,DreamFMOnlineArtwork } from "@/app/main/dreamfm/ui";

type DreamFMMediaMode = "cover" | "lyrics";
type DreamFMVideoAvailability = "checking" | "available" | "unavailable";
type DreamFMAirPlayAnchor = {
  x: number;
  y: number;
  width: number;
  height: number;
};
type DreamFMLyricsTrackRequest = Parameters<typeof fetchDreamFMTrackLyrics>[1];
type DreamFMLocalAirPlayMediaElement = HTMLMediaElement & {
  webkitShowPlaybackTargetPicker?: () => void;
  remote?: {
    prompt?: () => Promise<void>;
  };
};
type DreamFMLyricLineView = DreamFMLyricsData["lines"][number];
type DreamFMLyricWordView = NonNullable<DreamFMLyricLineView["words"]>[number];
type DreamFMLyricTimelineLine = {
  sourceIndex: number;
  startMs: number;
  endMs: number;
  activeStartMs: number;
  activeEndMs: number;
  text: string;
  words: DreamFMLyricWordView[];
};
const DREAM_FM_MEDIA_SPLIT_MIN_WIDTH = 760;
const DREAM_FM_LYRICS_CACHE_TTL_MS = 24 * 60 * 60 * 1000;
const DREAM_FM_LYRICS_CACHE_MAX_ITEMS = 120;
const DREAM_FM_LYRICS_SCROLL_DURATION_MS = 560;
const DREAM_FM_LYRICS_LINE_LEAD_MS = 160;
const DREAM_FM_LYRICS_LINE_GRACE_MS = 420;
const DREAM_FM_LYRICS_WORD_LEAD_MS = 60;
const DREAM_FM_LYRICS_MANUAL_SCROLL_LOCK_MS = 4200;
const DREAM_FM_LYRICS_AUTO_RETRY_DELAYS_MS = [700, 1600] as const;
const dreamFMLyricsCache = new Map<
  string,
  { data: DreamFMLyricsData; updatedAt: number }
>();
const dreamFMLyricsRequests = new Map<string, Promise<DreamFMLyricsData>>();

function readDreamFMNativeEventURLVideoId(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  try {
    const parsed = new URL(trimmed);
    const queryVideoId = parsed.searchParams.get("v")?.trim() ?? "";
    if (queryVideoId) {
      return queryVideoId;
    }
    const embedMatch = parsed.pathname.match(/\/embed\/([A-Za-z0-9_-]{11})/);
    return embedMatch?.[1] ?? "";
  } catch {
    return "";
  }
}

async function copyDreamFMTextToClipboard(value: string) {
  const text = value.trim();
  if (!text) {
    return;
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall through to the textarea text path for embedded WebViews.
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-10000px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    if (!document.execCommand("text")) {
      throw new Error("text command failed");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

function normalizeDreamFMLiveNativeState(
  state: DreamFMRemotePlaybackState,
  event: DreamFMNativePlayerEvent,
): DreamFMRemotePlaybackState {
  if (state !== "buffering" && state !== "loading") {
    return state;
  }
  const readyState = Number(event.readyState ?? 0);
  const currentTime = Number(event.currentTime ?? 0);
  const bufferedTime = Number(event.bufferedTime ?? 0);
  if (readyState >= 2 || currentTime > 0.15 || bufferedTime > 0.15) {
    return "playing";
  }
  return state;
}

function hasDreamFMMusicVideoContent(musicVideoType: string | undefined) {
  return musicVideoType?.trim() === "MUSIC_VIDEO_TYPE_OMV";
}

function resolveDreamFMTrackVideoAvailability(
  track: DreamFMOnlineItem,
  live: boolean,
): DreamFMVideoAvailability {
  const videoId = track.videoId.trim();
  if (!videoId) {
    return "unavailable";
  }
  if (live) {
    return "available";
  }
  if (track.videoAvailabilityKnown === true) {
    return track.hasVideo === true ? "available" : "unavailable";
  }
  const musicVideoType = track.musicVideoType?.trim();
  if (musicVideoType) {
    return hasDreamFMMusicVideoContent(musicVideoType)
      ? "available"
      : "unavailable";
  }
  if (track.hasVideo === true) {
    return "available";
  }
  return "checking";
}

function normalizeDreamFMLyricsCacheKey(videoId: string) {
  return videoId.trim();
}

function readDreamFMLyricsCache(videoId: string) {
  const key = normalizeDreamFMLyricsCacheKey(videoId);
  if (!key) {
    return null;
  }
  const entry = dreamFMLyricsCache.get(key);
  if (!entry) {
    return null;
  }
  if (Date.now() - entry.updatedAt > DREAM_FM_LYRICS_CACHE_TTL_MS) {
    dreamFMLyricsCache.delete(key);
    return null;
  }
  dreamFMLyricsCache.delete(key);
  dreamFMLyricsCache.set(key, entry);
  return entry.data;
}

function writeDreamFMLyricsCache(data: DreamFMLyricsData) {
  const key = normalizeDreamFMLyricsCacheKey(data.videoId);
  if (!key) {
    return;
  }
  dreamFMLyricsCache.delete(key);
  dreamFMLyricsCache.set(key, { data, updatedAt: Date.now() });
  while (dreamFMLyricsCache.size > DREAM_FM_LYRICS_CACHE_MAX_ITEMS) {
    const oldestKey = dreamFMLyricsCache.keys().next().value;
    if (!oldestKey) {
      break;
    }
    dreamFMLyricsCache.delete(oldestKey);
  }
}

function forgetDreamFMLyricsCache(videoId: string) {
  const key = normalizeDreamFMLyricsCacheKey(videoId);
  if (!key) {
    return;
  }
  dreamFMLyricsCache.delete(key);
  dreamFMLyricsRequests.delete(key);
}

function isDreamFMLyricsDataAvailable(data: DreamFMLyricsData | null | undefined) {
  if (!data || data.kind === "unavailable") {
    return false;
  }
  if (data.kind === "plain") {
    return data.text.trim().length > 0;
  }
  return data.lines.some((line) => {
    if (line.text.trim()) {
      return true;
    }
    return (line.words ?? []).some((word) => word.text.trim());
  });
}

function easeOutDreamFMLyricsScroll(progress: number) {
  const clamped = Math.max(0, Math.min(1, progress));
  return 1 - Math.pow(1 - clamped, 3);
}

function isDreamFMLyricsAbortError(error: unknown) {
  return error instanceof DOMException && error.name === "AbortError";
}

function shouldRetryDreamFMLyricsError(
  error: unknown,
  track: DreamFMLyricsTrackRequest,
) {
  if (isDreamFMLyricsAbortError(error)) {
    return false;
  }
  if (!track.videoId?.trim()) {
    return false;
  }
  if (getDreamFMErrorRetryable(error)) {
    return true;
  }
  const code = getDreamFMErrorCode(error);
  return code === "youtube_network_unavailable" ||
    code === "youtube_timeout" ||
    error instanceof TypeError;
}

function waitDreamFMLyricsRetryDelay(delayMs: number, signal: AbortSignal) {
  if (delayMs <= 0) {
    return Promise.resolve();
  }
  return new Promise<void>((resolve, reject) => {
    let timer = 0;
    const cleanup = () => {
      window.clearTimeout(timer);
      signal.removeEventListener("abort", handleAbort);
    };
    const handleResolve = () => {
      cleanup();
      resolve();
    };
    const handleAbort = () => {
      cleanup();
      reject(new DOMException("Lyrics request aborted", "AbortError"));
    };
    if (signal.aborted) {
      handleAbort();
      return;
    }
    timer = window.setTimeout(handleResolve, delayMs);
    signal.addEventListener("abort", handleAbort, { once: true });
  });
}

async function fetchDreamFMLyricsWithRetry(
  httpBaseURL: string,
  track: DreamFMLyricsTrackRequest,
  durationSeconds: number,
  signal: AbortSignal,
) {
  for (let attempt = 0; ; attempt += 1) {
    try {
      return await fetchDreamFMTrackLyrics(
        httpBaseURL,
        track,
        signal,
        durationSeconds,
      );
    } catch (error) {
      const delay = DREAM_FM_LYRICS_AUTO_RETRY_DELAYS_MS[attempt];
      if (
        delay === undefined ||
        !shouldRetryDreamFMLyricsError(error, track)
      ) {
        throw error;
      }
      await waitDreamFMLyricsRetryDelay(delay, signal);
    }
  }
}

function fetchDreamFMLyricsCached(
  httpBaseURL: string,
  track: DreamFMLyricsTrackRequest,
  durationSeconds: number,
  options: { force?: boolean } = {},
) {
  const key = normalizeDreamFMLyricsCacheKey(
    track.lyricsId || track.videoId || "",
  );
  if (!options.force) {
    const cached = readDreamFMLyricsCache(key);
    if (cached) {
      return Promise.resolve(cached);
    }
    const pending = dreamFMLyricsRequests.get(key);
    if (pending) {
      return pending;
    }
  }
  const controller = new AbortController();
  const request = fetchDreamFMLyricsWithRetry(
    httpBaseURL,
    track,
    durationSeconds,
    controller.signal,
  )
    .then((data) => {
      if (isDreamFMLyricsDataAvailable(data)) {
        writeDreamFMLyricsCache(data);
      }
      return data;
    })
    .finally(() => {
      if (dreamFMLyricsRequests.get(key) === request) {
        dreamFMLyricsRequests.delete(key);
      }
    });
  if (!options.force) {
    dreamFMLyricsRequests.set(key, request);
  }
  return request;
}

export function DreamFMPlayback(props: {
  mode: DreamFMMode;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  selectedOnline: DreamFMOnlineItem | null;
  selectedLocal: DreamFMLocalItem | null;
  httpBaseURL: string;
  onlineCommand: DreamFMPlayerCommand | null;
  onlinePlaybackEnabled: boolean;
  localCommand: DreamFMPlayerCommand | null;
  onlineQueueItems: DreamFMOnlineItem[];
  onlineQueueTitle: string;
  selectedOnlineId: string;
  localQueueItems: DreamFMLocalItem[];
  selectedLocalId: string;
  onlinePlaying: boolean;
  localPlaying: boolean;
  localResumeTime: number;
  onlineResumeTime: number;
  onlineProgress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  onlineState: DreamFMRemotePlaybackState;
  favoriteActive: boolean;
  favoriteBusy: boolean;
  sprite: Sprite | null;
  spriteImageURL: string;
  localProgress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  muted: boolean;
  volume: number;
  playMode: DreamFMPlayMode;
  text: ReturnType<typeof getXiaText>;
  onEnded: () => void;
  onOnlinePlayingChange: (playing: boolean) => void;
  onOnlineStateChange: (state: DreamFMRemotePlaybackState) => void;
  onOnlineProgressChange: (
    videoId: string,
    currentTime: number,
    duration: number,
    bufferedTime: number,
    transient?: boolean,
  ) => void;
  onOnlineNativeTrackChange: (event: DreamFMNativePlayerEvent) => void;
  onSelectOnlineQueueTrack: (item: DreamFMOnlineItem) => void;
  onClearOnlineQueue: () => void;
  onRemoveOnlineQueueItem: (item: DreamFMOnlineItem) => void;
  onSelectLocalQueueTrack: (item: DreamFMLocalItem) => void;
  onLocalPlayingChange: (playing: boolean) => void;
  onLocalProgressChange: (
    currentTime: number,
    duration: number,
    bufferedTime: number,
  ) => void;
  onLocalPlaybackIntent: () => void;
  onPrevious: () => void;
  onNext: () => void;
  onTogglePlayMode: () => void;
  onPlayModeChange: (mode: DreamFMPlayMode) => void;
  onTogglePlayback: () => void;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
  onToggleFavorite: () => void;
  onOpenOnlineArtist: (track: DreamFMOnlineItem) => void;
  onDownloadTrack: (url: string) => void;
  onOpenLocalDirectory: () => void;
}) {
  const playerRef = React.useRef<React.ElementRef<typeof MediaPlayer> | null>(
    null,
  );
  const localRemote = React.useMemo(() => new MediaRemoteControl(), []);
  const localResumeRef = React.useRef(props.localResumeTime);
  const localRestoreAppliedRef = React.useRef("");
  const localReplayStartedAtRef = React.useRef<number | null>(null);
  const pendingLocalCommandRef = React.useRef<
    DreamFMPlayerCommand["command"] | null
  >(null);
  const localTrack = props.selectedLocal;
  const [localMediaMode, setLocalMediaMode] =
    React.useState<DreamFMMediaMode>("cover");
  const [localQueueOpen, setLocalQueueOpen] = React.useState(false);
  const [localLyricsState, setLocalLyricsState] = React.useState<{
    lyricsId: string;
    loading: boolean;
    data: DreamFMLyricsData | null;
    error: string;
  }>({
    lyricsId: "",
    loading: false,
    data: null,
    error: "",
  });
  const localLyricsRetryKeyRef = React.useRef("");
  const [localLyricsRetryToken, setLocalLyricsRetryToken] = React.useState(0);

  const localLyricsTrack = React.useMemo<DreamFMLyricsTrackRequest | null>(() => {
    if (!localTrack) {
      return null;
    }
    return {
      lyricsId: `local:${localTrack.id}`,
      title: localTrack.lyricsTitle || localTrack.title,
      artist: localTrack.lyricsArtist || localTrack.author,
      durationLabel: localTrack.durationLabel,
    };
  }, [
    localTrack?.author,
    localTrack?.durationLabel,
    localTrack?.id,
    localTrack?.lyricsArtist,
    localTrack?.lyricsTitle,
    localTrack?.title,
  ]);
  const retryLocalLyrics = React.useCallback(() => {
    const lyricsId = String(localLyricsTrack?.lyricsId || "").trim();
    if (!lyricsId) {
      return;
    }
    localLyricsRetryKeyRef.current = lyricsId;
    forgetDreamFMLyricsCache(lyricsId);
    setLocalLyricsRetryToken((value) => value + 1);
  }, [localLyricsTrack?.lyricsId]);

  const getLocalMediaElement = React.useCallback(() => {
    const provider = playerRef.current?.provider as
      | {
          media?: HTMLMediaElement;
          audio?: HTMLAudioElement;
        }
      | null
      | undefined;
    if (provider?.media instanceof HTMLMediaElement) {
      return provider.media;
    }
    if (provider?.audio instanceof HTMLAudioElement) {
      return provider.audio;
    }
    const root = playerRef.current as unknown as
      | {
          querySelector?: (selector: string) => Element | null;
        }
      | null
      | undefined;
    const element =
      typeof root?.querySelector === "function"
        ? root.querySelector("audio,video")
        : null;
    return element instanceof HTMLMediaElement ? element : null;
  }, []);

  const handleLocalAirPlay = React.useCallback(() => {
    const media = getLocalMediaElement() as
      | DreamFMLocalAirPlayMediaElement
      | null;
    if (!media) {
      return;
    }
    if (typeof media.webkitShowPlaybackTargetPicker === "function") {
      media.webkitShowPlaybackTargetPicker();
      return;
    }
    if (typeof media.remote?.prompt === "function") {
      void media.remote.prompt().catch((error) => {
        console.warn("[DreamFM] local AirPlay picker unavailable", error);
      });
    }
  }, [getLocalMediaElement]);

  const runLocalPlayerCommand = React.useCallback(
    (command: DreamFMPlayerCommand["command"]) => {
      const player = playerRef.current;
      if (!player) {
        pendingLocalCommandRef.current =
          command === "play" || command === "replay" ? command : null;
        return;
      }
      if (command === "replay") {
        const media = getLocalMediaElement();
        pendingLocalCommandRef.current = null;
        if (media) {
          media.currentTime = 0;
          void media.play().catch(() => {});
        }
        player.currentTime = 0;
        player.paused = false;
        return;
      }
      if (command === "play") {
        const media = getLocalMediaElement();
        pendingLocalCommandRef.current = null;
        if (media) {
          void media.play().catch(() => {});
        }
        player.paused = false;
        return;
      }
      const media = getLocalMediaElement();
      pendingLocalCommandRef.current = null;
      localReplayStartedAtRef.current = null;
      media?.pause();
      player.paused = true;
    },
    [getLocalMediaElement],
  );

  const handleLocalTogglePlayback = React.useCallback<
    React.MouseEventHandler<HTMLButtonElement>
  >((event) => {
    props.onLocalPlaybackIntent();
    if (props.localPlaying) {
      localRemote.pause(event.nativeEvent);
      return;
    }
    localRemote.play(event.nativeEvent);
  }, [localRemote, props.localPlaying, props.onLocalPlaybackIntent]);

  React.useEffect(() => {
    if (props.mode !== "local" || !props.localCommand) {
      return;
    }
    runLocalPlayerCommand(props.localCommand.command);
  }, [props.localCommand, props.mode, runLocalPlayerCommand]);

  React.useEffect(() => {
    const player = playerRef.current;
    if (!player) {
      return;
    }
    localRemote.setPlayer(player);
  }, [localRemote, localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "local") {
      return;
    }
    const media = getLocalMediaElement();
    if (!media) {
      return;
    }
    media.setAttribute("x-webkit-airplay", "allow");
    media.disableRemotePlayback = false;
  }, [getLocalMediaElement, localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "local") {
      return;
    }
    const player = playerRef.current;
    const media = getLocalMediaElement();
    if (!player && !media) {
      return;
    }
    const nextVolume = clampVolume(props.volume);
    const nextMuted = props.muted || props.volume <= 0;
    if (media) {
      media.volume = nextVolume;
      media.muted = nextMuted;
    }
    if (player) {
      player.volume = nextVolume;
      player.muted = nextMuted;
    }
  }, [
    getLocalMediaElement,
    props.mode,
    props.muted,
    props.volume,
    localTrack?.id,
  ]);

  React.useEffect(() => {
    if (props.mode !== "local" || !localTrack) {
      localReplayStartedAtRef.current = null;
      props.onLocalProgressChange(0, 0, 0);
      return;
    }

    const syncProgress = () => {
      const player = playerRef.current;
      const media = getLocalMediaElement();
      const source = media ?? player;
      let currentTime =
        source && Number.isFinite(source.currentTime)
          ? Math.max(0, source.currentTime)
          : 0;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, props.localProgress.duration);
      const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
      const bufferedTime = buffered
        ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
        : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
      const replayStartedAt = localReplayStartedAtRef.current;
      const paused =
        media?.paused ?? (player ? Boolean(player.paused) : true);
      if (currentTime > 0.05) {
        localReplayStartedAtRef.current = null;
      } else if (
        replayStartedAt !== null &&
        props.playMode === "repeat" &&
        !paused
      ) {
        currentTime = Math.max(
          0,
          Math.min((performance.now() - replayStartedAt) / 1000, duration),
        );
      }
      props.onLocalProgressChange(currentTime, duration, bufferedTime);
    };

    syncProgress();
    const timer = window.setInterval(syncProgress, 250);
    return () => window.clearInterval(timer);
  }, [
    getLocalMediaElement,
    localTrack?.id,
    props.localProgress.bufferedTime,
    props.localProgress.duration,
    props.mode,
    props.onLocalProgressChange,
    props.playMode,
  ]);

  const handleLocalSeek = React.useCallback(
    (seconds: number) => {
      if (props.mode !== "local" || !localTrack) {
        return;
      }
      const player = playerRef.current;
      const media = getLocalMediaElement();
      const source = media ?? player;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, props.localProgress.duration);
      if (duration <= 0) {
        return;
      }
      const nextTime = Math.max(0, Math.min(seconds, duration));
      localReplayStartedAtRef.current = null;
      if (media) {
        media.currentTime = nextTime;
      }
      if (player) {
        player.currentTime = nextTime;
      }
      const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
      const bufferedTime = buffered
        ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
        : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
      props.onLocalProgressChange(nextTime, duration, bufferedTime);
    },
    [
      getLocalMediaElement,
      localTrack,
      props.localProgress.bufferedTime,
      props.localProgress.duration,
      props.mode,
      props.onLocalProgressChange,
    ],
  );

  const handleLocalTimeUpdate = React.useCallback(
    (currentTime: number) => {
      if (props.mode !== "local" || !localTrack) {
        return;
      }
      const player = playerRef.current;
      const media = getLocalMediaElement();
      const source = media ?? player;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, props.localProgress.duration);
      if (duration <= 0) {
        return;
      }
      const sourceTime =
        source && Number.isFinite(source.currentTime)
          ? Math.max(0, source.currentTime)
          : currentTime;
      const nextTime = Math.max(0, Math.min(sourceTime, duration));
      if (nextTime > 0.05) {
        localReplayStartedAtRef.current = null;
      }
      const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
      const bufferedTime = buffered
        ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
        : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
      props.onLocalProgressChange(nextTime, duration, bufferedTime);
    },
    [
      getLocalMediaElement,
      localTrack,
      props.localProgress.bufferedTime,
      props.localProgress.duration,
      props.mode,
      props.onLocalProgressChange,
    ],
  );

  const syncLocalReplayState = React.useCallback(() => {
    const player = playerRef.current;
    const media = getLocalMediaElement();
    const source = media ?? player;
    const duration =
      source && Number.isFinite(source.duration)
        ? Math.max(0, source.duration)
        : Math.max(0, props.localProgress.duration);
    const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
    const bufferedTime = buffered
      ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
      : Math.max(0, Math.min(props.localProgress.bufferedTime, duration));
    if (media) {
      media.currentTime = 0;
    }
    if (player) {
      player.currentTime = 0;
    }
    localReplayStartedAtRef.current = performance.now();
    props.onLocalProgressChange(0, duration, bufferedTime);
    props.onLocalPlayingChange(true);
  }, [
    getLocalMediaElement,
    props.localProgress.bufferedTime,
    props.localProgress.duration,
    props.onLocalPlayingChange,
    props.onLocalProgressChange,
  ]);

  React.useEffect(() => {
    localReplayStartedAtRef.current = null;
    localRestoreAppliedRef.current = "";
    localResumeRef.current = props.localResumeTime;
  }, [localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "local" || !localTrack) {
      return;
    }
    const resumeSeconds = Math.max(0, localResumeRef.current);
    if (
      resumeSeconds <= 0 ||
      localRestoreAppliedRef.current === localTrack.id
    ) {
      return;
    }
    let attempts = 0;
    const timer = window.setInterval(() => {
      const player = playerRef.current;
      if (!player) {
        return;
      }
      const duration = Number.isFinite(player.duration)
        ? Math.max(0, player.duration)
        : 0;
      if (duration <= 0 && attempts < 30) {
        attempts += 1;
        return;
      }
      const target =
        duration > 0
          ? Math.min(resumeSeconds, Math.max(duration - 1, 0))
          : resumeSeconds;
      if (target > 0.5) {
        player.currentTime = target;
      }
      localRestoreAppliedRef.current = localTrack.id;
      window.clearInterval(timer);
    }, 160);
    return () => window.clearInterval(timer);
  }, [localTrack?.id, props.mode]);

  React.useEffect(() => {
    if (props.mode !== "local" || !localTrack || !localLyricsTrack) {
      setLocalLyricsState({
        lyricsId: "",
        loading: false,
        data: null,
        error: "",
      });
      setLocalMediaMode("cover");
      setLocalQueueOpen(false);
      return;
    }
    const lyricsId = String(localLyricsTrack.lyricsId || "").trim();
    if (!lyricsId || !localLyricsTrack.title.trim()) {
      setLocalLyricsState({
        lyricsId,
        loading: false,
        data: null,
        error: "",
      });
      setLocalMediaMode("cover");
      return;
    }
    const forceRequest = localLyricsRetryKeyRef.current === lyricsId;
    if (forceRequest) {
      localLyricsRetryKeyRef.current = "";
    }
    const cachedLyrics = forceRequest ? null : readDreamFMLyricsCache(lyricsId);
    if (cachedLyrics) {
      setLocalLyricsState({
        lyricsId,
        loading: false,
        data: cachedLyrics,
        error: "",
      });
      return;
    }
    let cancelled = false;
    setLocalLyricsState({
      lyricsId,
      loading: true,
      data: null,
      error: "",
    });
    void fetchDreamFMLyricsCached(
      props.httpBaseURL,
      localLyricsTrack,
      props.localProgress.duration,
      { force: forceRequest },
    )
      .then((data) => {
        if (cancelled) {
          return;
        }
        setLocalLyricsState({
          lyricsId,
          loading: false,
          data,
          error: "",
        });
      })
      .catch((error: unknown) => {
        if (cancelled) {
          return;
        }
        setLocalLyricsState({
          lyricsId,
          loading: false,
          data: null,
          error: getDreamFMErrorMessage(error) || props.text.dreamFm.lyricsEmpty,
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    localLyricsTrack,
    props.text.dreamFm.lyricsEmpty,
    props.httpBaseURL,
    localTrack?.id,
    localLyricsRetryToken,
    props.mode,
  ]);

  if (props.mode !== "local") {
    const track = props.selectedOnline;
    if (!track) {
      return (
        <div className="flex h-full items-center justify-center">
          <div className="relative shrink-0">
            <div className="relative flex flex-col items-center justify-center gap-3">
              <SpriteDisplay
                sprite={props.sprite}
                imageUrl={props.spriteImageURL}
                animation="seeking"
                alt={props.text.dreamFm.selectStation}
              />
              <div className="text-sm font-medium text-sidebar-foreground/58">
                {props.text.dreamFm.contentEmpty}
              </div>
            </div>
          </div>
        </div>
      );
    }

    return (
      <DreamFMYouTubePlayback
        mode={props.mode}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        track={track}
        httpBaseURL={props.httpBaseURL}
        command={props.onlineCommand}
        enabled={props.onlinePlaybackEnabled}
        queueItems={props.onlineQueueItems}
        queueTitle={props.onlineQueueTitle}
        selectedQueueId={props.selectedOnlineId}
        resumeSeconds={props.onlineResumeTime}
        progress={props.onlineProgress}
        playing={props.onlinePlaying}
        playMode={props.playMode}
        favoriteActive={props.favoriteActive}
        favoriteBusy={props.favoriteBusy}
        muted={props.muted}
        volume={props.volume}
        state={props.onlineState}
        text={props.text}
        onEnded={props.onEnded}
        onPlayingChange={props.onOnlinePlayingChange}
        onStateChange={props.onOnlineStateChange}
        onProgressChange={props.onOnlineProgressChange}
        onNativeTrackChange={props.onOnlineNativeTrackChange}
        onSelectQueueTrack={props.onSelectOnlineQueueTrack}
        onClearQueue={props.onClearOnlineQueue}
        onRemoveQueueItem={props.onRemoveOnlineQueueItem}
        onPrevious={props.onPrevious}
        onNext={props.onNext}
        onTogglePlayMode={props.onTogglePlayMode}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={props.onTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleFavorite={props.onToggleFavorite}
        onOpenArtist={props.onOpenOnlineArtist}
        onDownloadTrack={props.onDownloadTrack}
      />
    );
  }

  const track = localTrack;
  if (!track) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="relative flex flex-col items-center justify-center gap-3">
          <SpriteDisplay
            sprite={props.sprite}
            imageUrl={props.spriteImageURL}
            animation="seeking"
            alt={props.text.dreamFm.local}
          />
          <div className="text-sm font-medium text-sidebar-foreground/58">
            {props.text.dreamFm.contentEmpty}
          </div>
        </div>
      </div>
    );
  }

  const localLyricsAvailable = isDreamFMLyricsDataAvailable(localLyricsState.data);

  return (
    <div className="relative h-full min-h-0 overflow-hidden">
      <MediaPlayer
        ref={playerRef}
        key={track.id}
        src={resolveAudioSource(track.previewURL, track.path)}
        title={track.title}
        viewType="audio"
        streamType="on-demand"
        load="eager"
        preload="metadata"
        loop={props.playMode === "repeat"}
        playsInline
        onPlay={() => props.onLocalPlayingChange(true)}
        onPause={() => {
          localReplayStartedAtRef.current = null;
          props.onLocalPlayingChange(false);
        }}
        onReplay={() => syncLocalReplayState()}
        onTimeUpdate={(detail) => handleLocalTimeUpdate(detail.currentTime)}
        onEnded={() => {
          if (props.playMode === "repeat") {
            runLocalPlayerCommand("replay");
            syncLocalReplayState();
            return;
          }
          props.onLocalPlayingChange(false);
          props.onEnded();
        }}
        onCanPlay={() => {
          if (pendingLocalCommandRef.current) {
            runLocalPlayerCommand(pendingLocalCommandRef.current);
          }
        }}
        className="pointer-events-none"
        style={DREAM_FM_HIDDEN_ENGINE_STYLE}
      >
        <MediaProvider />
      </MediaPlayer>

      <DreamFMPlayerChrome
        mediaMode={localMediaMode}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        cover={
          <DreamFMLocalCoverSurface
            key={track.id}
            src={track.coverURL || DEFAULT_COVER_IMAGE_URL}
            title={track.title}
          />
        }
        lyrics={
          <DreamFMLyricsSurface
            text={props.text}
            lyrics={localLyricsState.data}
            loading={localLyricsState.loading}
            error={localLyricsState.error}
            onRetry={localLyricsState.error ? retryLocalLyrics : undefined}
            currentTimeMs={Math.max(0, props.localProgress.currentTime * 1000)}
            timelineRunning={props.localPlaying}
          />
        }
        hasVideo={false}
        lyricsAvailable={localLyricsAvailable || Boolean(localLyricsState.error)}
        lyricsLoading={!localLyricsAvailable && localLyricsState.loading}
        title={track.title}
        subtitle={track.author}
        infoActions={
          <>
            <DreamFMPlayerIconButton
              label={props.text.actions.openDirectory}
              disabled={!track.path}
              onClick={props.onOpenLocalDirectory}
            >
              <FolderOpen className="h-4 w-4" />
            </DreamFMPlayerIconButton>
          </>
        }
        progress={props.localProgress}
        onSeek={handleLocalSeek}
        playing={props.localPlaying}
        loading={false}
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        text={props.text}
        onAirPlay={props.airPlaySupported ? handleLocalAirPlay : undefined}
        onMediaModeChange={setLocalMediaMode}
        onPrevious={props.onPrevious}
        onNext={props.onNext}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={handleLocalTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleQueue={() => setLocalQueueOpen((current) => !current)}
        queueOpen={localQueueOpen}
        queueOverlay={
          localQueueOpen ? (
            <DreamFMLocalPlaybackQueuePopup
              queueTitle={props.text.dreamFm.upNext}
              queueItems={props.localQueueItems}
              selectedQueueId={props.selectedLocalId}
              text={props.text}
              onSelectQueueTrack={props.onSelectLocalQueueTrack}
              onClose={() => setLocalQueueOpen(false)}
            />
          ) : null
        }
      />
    </div>
  );
}

export function DreamFMYouTubePlayback(props: {
  mode: Exclude<DreamFMMode, "local">;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  track: DreamFMOnlineItem;
  httpBaseURL: string;
  command: DreamFMPlayerCommand | null;
  enabled: boolean;
  queueItems: DreamFMOnlineItem[];
  queueTitle: string;
  selectedQueueId: string;
  resumeSeconds: number;
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  playing: boolean;
  playMode: DreamFMPlayMode;
  favoriteActive: boolean;
  favoriteBusy: boolean;
  muted: boolean;
  volume: number;
  state: DreamFMRemotePlaybackState;
  text: ReturnType<typeof getXiaText>;
  onEnded: () => void;
  onPlayingChange: (playing: boolean) => void;
  onStateChange: (state: DreamFMRemotePlaybackState) => void;
  onProgressChange: (
    videoId: string,
    currentTime: number,
    duration: number,
    bufferedTime: number,
    transient?: boolean,
  ) => void;
  onNativeTrackChange: (event: DreamFMNativePlayerEvent) => void;
  onSelectQueueTrack: (item: DreamFMOnlineItem) => void;
  onClearQueue: () => void;
  onRemoveQueueItem: (item: DreamFMOnlineItem) => void;
  onPrevious: () => void;
  onNext: () => void;
  onTogglePlayMode: () => void;
  onPlayModeChange: (mode: DreamFMPlayMode) => void;
  onTogglePlayback: () => void;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
  onToggleFavorite: () => void;
  onOpenArtist: (track: DreamFMOnlineItem) => void;
  onDownloadTrack: (url: string) => void;
}) {
  const resumeRef = React.useRef(props.resumeSeconds);
  const lastPlayRequestRef = React.useRef("");
  const handledNativeCommandRef = React.useRef("");
  const intendedVideoSinceRef = React.useRef(Date.now());
  const liveMismatchReplayAtRef = React.useRef(0);
  const isLive = props.track.group === "live";
  const playerService = isLive
    ? DREAM_FM_LIVE_PLAYER_SERVICE
    : DREAM_FM_NATIVE_PLAYER_SERVICE;
  const inactivePlayerService = isLive
    ? DREAM_FM_NATIVE_PLAYER_SERVICE
    : DREAM_FM_LIVE_PLAYER_SERVICE;
  const playerEventName = isLive
    ? DREAM_FM_LIVE_PLAYER_EVENT
    : DREAM_FM_NATIVE_PLAYER_EVENT;
  const playerEventSource = isLive
    ? "dreamfm-youtube-live-player"
    : "dreamfm-youtube-music-player";
  const artistName = props.track.channel.trim();
  const showFavoriteAction =
    props.mode === "online" && !isLive;
  const trackVideoId = props.track.videoId.trim();
  const canCheckVideoAvailability = !isLive && trackVideoId !== "";
  const showDownloadAction = !isLive && props.track.videoId.trim() !== "";
  const trackPageURL = React.useMemo(() => {
    const videoId = props.track.videoId.trim();
    return videoId ? buildYouTubeWatchURL(videoId) : "";
  }, [props.track.videoId]);
  const [mediaMode, setMediaMode] = React.useState<DreamFMMediaMode>("cover");
  const [videoOpen, setVideoOpen] = React.useState(false);
  const videoOpenRef = React.useRef(false);
  const [videoAvailability, setVideoAvailability] =
    React.useState<DreamFMVideoAvailability>(() =>
      resolveDreamFMTrackVideoAvailability(props.track, isLive),
    );
  const [playbackAdvertising, setPlaybackAdvertising] = React.useState(false);
  const [playbackAdvertisingLabel, setPlaybackAdvertisingLabel] =
    React.useState("");
  const [playbackAdSkippable, setPlaybackAdSkippable] =
    React.useState(false);
  const [playbackErrorLabel, setPlaybackErrorLabel] = React.useState("");
  const [playbackErrorMessage, setPlaybackErrorMessage] = React.useState("");
  const [queueOpen, setQueueOpen] = React.useState(false);
  const [lyricsState, setLyricsState] = React.useState<{
    videoId: string;
    loading: boolean;
    data: DreamFMLyricsData | null;
    error: string;
  }>({
    videoId: props.track.videoId,
    loading: false,
    data: null,
    error: "",
  });
  const lyricsRetryKeyRef = React.useRef("");
  const [lyricsRetryToken, setLyricsRetryToken] = React.useState(0);
  const hasPlayableBuffer =
    props.state === "playing" ||
    (isLive && props.playing && props.state === "buffering") ||
    props.progress.currentTime > 0.15 ||
    props.progress.bufferedTime > 0.15;
  const transportLoading =
    props.enabled &&
    (props.state === "loading" || props.state === "buffering") &&
    !hasPlayableBuffer;
  const progressLoading =
    !playbackAdvertising &&
    props.state !== "error" &&
    (props.state === "loading" || props.state === "buffering");
  const lyricsAvailable =
    !isLive && isDreamFMLyricsDataAvailable(lyricsState.data);
  const hasVideo = isLive
    ? trackVideoId !== ""
    : videoAvailability === "available";
  const videoLoading =
    canCheckVideoAvailability && videoAvailability === "checking";
  const retryLyrics = React.useCallback(() => {
    const videoId = props.track.videoId.trim();
    if (!videoId) {
      return;
    }
    lyricsRetryKeyRef.current = videoId;
    forgetDreamFMLyricsCache(videoId);
    setLyricsRetryToken((value) => value + 1);
  }, [props.track.videoId]);

  React.useEffect(() => {
    resumeRef.current = props.resumeSeconds;
  }, [props.resumeSeconds, props.track.videoId]);

  React.useEffect(() => {
    intendedVideoSinceRef.current = Date.now();
    setQueueOpen(false);
    setVideoAvailability(resolveDreamFMTrackVideoAvailability(props.track, isLive));
    setPlaybackAdvertising(false);
    setPlaybackAdvertisingLabel("");
    setPlaybackAdSkippable(false);
    setPlaybackErrorLabel("");
    setPlaybackErrorMessage("");
    if (isLive) {
      setLyricsState({
        videoId: props.track.videoId,
        loading: false,
        data: null,
        error: "",
      });
      setMediaMode("cover");
      return;
    }
    const videoId = props.track.videoId.trim();
    if (!videoId) {
      setLyricsState({
        videoId,
        loading: false,
        data: null,
        error: "",
      });
      return;
    }
    const forceRequest = lyricsRetryKeyRef.current === videoId;
    if (forceRequest) {
      lyricsRetryKeyRef.current = "";
    }
    const cachedLyrics = forceRequest ? null : readDreamFMLyricsCache(videoId);
    if (cachedLyrics) {
      setLyricsState({
        videoId,
        loading: false,
        data: cachedLyrics,
        error: "",
      });
      return;
    }
    let cancelled = false;
    setLyricsState({
      videoId,
      loading: true,
      data: null,
      error: "",
    });
    void fetchDreamFMLyricsCached(
      props.httpBaseURL,
      props.track,
      props.progress.duration,
      { force: forceRequest },
    )
      .then((data) => {
        if (cancelled) {
          return;
        }
        setLyricsState({
          videoId,
          loading: false,
          data,
          error: "",
        });
      })
      .catch((error: unknown) => {
        if (cancelled) {
          return;
        }
        setLyricsState({
          videoId,
          loading: false,
          data: null,
          error: getDreamFMErrorMessage(error) || props.text.dreamFm.lyricsEmpty,
        });
      });
    return () => {
      cancelled = true;
    };
  }, [
    isLive,
    props.text.dreamFm.lyricsEmpty,
    props.httpBaseURL,
    props.track.channel,
    props.track.durationLabel,
    props.track.title,
    props.track.videoId,
    lyricsRetryToken,
  ]);

  const callNativePlayer = React.useCallback(
    (method: string, ...args: unknown[]) =>
      Call.ByName(`${playerService}.${method}`, ...args),
    [playerService],
  );

  React.useEffect(() => {
    videoOpenRef.current = videoOpen;
  }, [videoOpen]);

  React.useEffect(() => {
    if (hasVideo || !videoOpen) {
      return;
    }
    setVideoOpen(false);
    void callNativePlayer("HideVideoWindow").catch(() => {});
  }, [callNativePlayer, hasVideo, videoOpen]);

  React.useEffect(() => {
    if (!props.enabled || !canCheckVideoAvailability || videoAvailability !== "checking") {
      return;
    }
    const controller = new AbortController();
    const videoId = trackVideoId;
    void fetchDreamFMTrackInfo(props.httpBaseURL, videoId, controller.signal)
      .then((item) => {
        if (!item || controller.signal.aborted || item.videoId.trim() !== videoId) {
          return;
        }
        const resolved = resolveDreamFMTrackVideoAvailability(item, false);
        setVideoAvailability(resolved === "checking" ? "unavailable" : resolved);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        console.warn("[DreamFM] video availability metadata unavailable", {
          videoId,
          error,
        });
        setVideoAvailability("unavailable");
      });
    return () => controller.abort();
  }, [
    canCheckVideoAvailability,
    props.enabled,
    props.httpBaseURL,
    trackVideoId,
    videoAvailability,
  ]);

  React.useEffect(() => {
    if (!videoOpen || !hasVideo) {
      return;
    }
    void callNativePlayer("ShowVideoWindow").catch((error) => {
      console.warn("[DreamFM] native video window unavailable", error);
    });
  }, [callNativePlayer, hasVideo, props.track.videoId, videoOpen]);

  React.useEffect(() => {
    return () => {
      if (videoOpenRef.current) {
        void Call.ByName(`${playerService}.HideVideoWindow`).catch(
          () => {},
        );
      }
    };
  }, [playerService]);

  const markNativePlayerUnavailable = React.useCallback(
    (error: unknown) => {
      console.warn(
        isLive
          ? "[DreamFM] YouTube live native player unavailable"
          : "[DreamFM] YouTube Music native player unavailable",
        {
          videoId: props.track.videoId,
          title: props.track.title,
          error,
        },
      );
      props.onStateChange("error");
      props.onPlayingChange(false);
    },
    [
      isLive,
      props.onPlayingChange,
      props.onStateChange,
      props.track.title,
      props.track.videoId,
    ],
  );

  const handleAirPlay = React.useCallback((anchor: DreamFMAirPlayAnchor) => {
    if (!props.airPlaySupported) {
      return;
    }
    void callNativePlayer("ShowAirPlayPicker", anchor).catch((error) => {
      console.warn("[DreamFM] AirPlay picker unavailable", error);
    });
  }, [callNativePlayer, props.airPlaySupported]);

  const handleSkipAd = React.useCallback(() => {
    if (!playbackAdvertising || !playbackAdSkippable) {
      return;
    }
    void callNativePlayer("SkipAd").catch((error) => {
      console.warn("[DreamFM] native ad skip unavailable", error);
    });
  }, [callNativePlayer, playbackAdSkippable, playbackAdvertising]);

  const handleToggleVideo = React.useCallback(() => {
    if (!hasVideo) {
      return;
    }
    const nextOpen = !videoOpen;
    setVideoOpen(nextOpen);
    void callNativePlayer(nextOpen ? "ShowVideoWindow" : "HideVideoWindow").catch(
      (error) => {
        setVideoOpen(!nextOpen);
        console.warn("[DreamFM] native video window unavailable", error);
      },
    );
  }, [callNativePlayer, hasVideo, videoOpen]);

  const handleOpenTrackPage = React.useCallback(() => {
    if (!trackPageURL) {
      return;
    }
    void openExternalURL(trackPageURL).catch((error) => {
      console.warn("[DreamFM] open track page unavailable", {
        url: trackPageURL,
        error,
      });
    });
  }, [trackPageURL]);

  const handleCopyTrackLink = React.useCallback(() => {
    if (!trackPageURL) {
      return;
    }
    void copyDreamFMTextToClipboard(trackPageURL)
      .then(() => {
        messageBus.publishToast({
          id: "dreamfm-text-link",
          intent: "success",
          title: props.text.dreamFm.linkCopied,
          source: "dreamfm",
          autoCloseMs: 2200,
        });
      })
      .catch((error) => {
        console.warn("[DreamFM] text track link unavailable", {
          url: trackPageURL,
          error,
        });
      });
  }, [props.text.dreamFm.linkCopied, trackPageURL]);

  const playNativeTrack = React.useCallback(
    (commandId: number, startSeconds: number) => {
      const requestKey = `${props.track.videoId}:${commandId}`;
      if (lastPlayRequestRef.current === requestKey) {
        return;
      }
      lastPlayRequestRef.current = requestKey;
      setPlaybackAdvertising(false);
      setPlaybackAdvertisingLabel("");
      setPlaybackAdSkippable(false);
      setPlaybackErrorLabel("");
      setPlaybackErrorMessage("");
      props.onStateChange("loading");
      props.onPlayingChange(false);
      void Call.ByName(`${inactivePlayerService}.Pause`).catch(() => {});
      void Call.ByName(`${inactivePlayerService}.HideVideoWindow`).catch(() => {});
      void callNativePlayer("Play", {
        videoId: props.track.videoId,
        title: props.track.title,
        artist: props.track.channel,
        startSeconds: startSeconds > 1 ? startSeconds : 0,
        volume: clampVolume(props.volume),
        muted: props.muted,
      }).catch(markNativePlayerUnavailable);
    },
    [
      callNativePlayer,
      inactivePlayerService,
      isLive,
      markNativePlayerUnavailable,
      props.muted,
      props.onPlayingChange,
      props.onStateChange,
      props.track.channel,
      props.track.title,
      props.track.videoId,
      props.volume,
    ],
  );

  React.useEffect(() => {
    if (!props.command) {
      return;
    }
    const commandKey = `${props.track.videoId}:${props.command.id}:${props.command.command}`;
    if (handledNativeCommandRef.current === commandKey) {
      return;
    }
    handledNativeCommandRef.current = commandKey;
    if (!props.enabled) {
      return;
    }
    if (props.command.command === "play") {
      playNativeTrack(props.command.id, resumeRef.current);
      return;
    }
    if (props.command.command === "pause") {
      props.onStateChange("paused");
      props.onPlayingChange(false);
      void callNativePlayer("Pause").catch(markNativePlayerUnavailable);
      return;
    }
    props.onProgressChange(
      props.track.videoId,
      0,
      Math.max(0, props.progress.duration),
      0,
    );
    props.onStateChange("buffering");
    props.onPlayingChange(true);
    void callNativePlayer("Replay").catch(markNativePlayerUnavailable);
  }, [
    callNativePlayer,
    markNativePlayerUnavailable,
    playNativeTrack,
    props.command,
    props.enabled,
    props.onPlayingChange,
    props.onProgressChange,
    props.onStateChange,
    props.progress.duration,
    props.track.videoId,
  ]);

  React.useEffect(() => {
    if (!props.enabled) {
      return;
    }
    void callNativePlayer("SetVolume", {
      volume: clampVolume(props.volume),
      muted: props.muted,
    }).catch(() => {});
  }, [callNativePlayer, props.enabled, props.muted, props.volume]);

  React.useEffect(
    () => {
      if (!props.enabled) {
        return;
      }
      return () => {
        void Call.ByName(`${playerService}.Pause`).catch(
          () => {},
        );
      };
    },
    [playerService, props.enabled],
  );

  React.useEffect(() => {
    if (!props.enabled) {
      return;
    }
    const offPlayerEvent = Events.On(playerEventName, (event) => {
      const data = ((event as { data?: unknown }).data ??
        event) as DreamFMNativePlayerEvent;
      if (!data || data.source !== playerEventSource) {
        return;
      }
      if (data.type === "video-closed") {
        setVideoOpen(false);
        return;
      }
      const eventVideoId = String(data.observedVideoId || data.videoId || "").trim();
      const requestedVideoId = String(data.requestedVideoId || "").trim();
      const eventURLVideoId = readDreamFMNativeEventURLVideoId(
        String(data.url || ""),
      );
      const liveEventBelongsToTrack =
        isLive &&
        (requestedVideoId
          ? requestedVideoId === props.track.videoId
          : eventURLVideoId === props.track.videoId) &&
        (!eventURLVideoId || eventURLVideoId === props.track.videoId);
      const eventBelongsToCurrentTrack = isLive
        ? liveEventBelongsToTrack ||
          !eventVideoId ||
          eventVideoId === props.track.videoId
        : !eventVideoId ||
          eventVideoId === props.track.videoId ||
          requestedVideoId === props.track.videoId ||
          eventURLVideoId === props.track.videoId;
      const nextState = isLive
        ? normalizeDreamFMLiveNativeState(data.state || "idle", data)
        : data.state || "idle";

      const syncVideoAvailabilityState = () => {
        if (isLive || !eventBelongsToCurrentTrack) {
          return;
        }
        if (data.videoAvailabilityKnown === true) {
          setVideoAvailability(
            data.videoAvailable === true ? "available" : "unavailable",
          );
          return;
        }
        if (data.videoAvailable === true) {
          setVideoAvailability("available");
        }
      };

      const syncPlaybackPresentationState = () => {
        syncVideoAvailabilityState();
        if (nextState === "error") {
          const errorCode = String(data.errorCode ?? data.code ?? "").trim();
          const errorMessage = String(data.errorMessage || data.message || "").trim();
          setPlaybackAdvertising(false);
          setPlaybackAdvertisingLabel("");
          setPlaybackAdSkippable(false);
          setPlaybackErrorLabel(errorCode);
          setPlaybackErrorMessage(errorMessage);
        } else {
          const advertising = Boolean(data.advertising || data.ad);
          setPlaybackAdvertising(advertising);
          setPlaybackAdvertisingLabel(advertising ? String(data.adLabel || "").trim() : "");
          setPlaybackAdSkippable(advertising && Boolean(data.adSkippable));
          setPlaybackErrorLabel("");
          setPlaybackErrorMessage("");
        }
      };

      if (isLive && eventVideoId && eventVideoId !== props.track.videoId) {
        const switchedRecently = Date.now() - intendedVideoSinceRef.current < 1800;
        if (liveEventBelongsToTrack) {
          syncPlaybackPresentationState();
          props.onProgressChange(
            props.track.videoId,
            Number(data.currentTime || 0),
            Number(data.duration || 0),
            Number(data.bufferedTime || 0),
          );
          if (data.type !== "ready") {
            props.onStateChange(nextState);
            props.onPlayingChange(
              nextState === "playing" || nextState === "buffering",
            );
          }
          return;
        }
        if (
          !switchedRecently &&
          (nextState === "playing" || nextState === "buffering")
        ) {
          const now = Date.now();
          if (now - liveMismatchReplayAtRef.current > 5000) {
            liveMismatchReplayAtRef.current = now;
            props.onProgressChange(props.track.videoId, 0, 0, 0, true);
            props.onStateChange("buffering");
            props.onPlayingChange(true);
            playNativeTrack(now, 0);
          }
        }
        return;
      }

      syncPlaybackPresentationState();

      if (data.type === "track-ended") {
        if (nextState === "error") {
          props.onStateChange("error");
          props.onPlayingChange(false);
          return;
        }
        if (data.advertising || data.ad) {
          return;
        }
        if (eventVideoId && eventVideoId !== props.track.videoId) {
          return;
        }
        props.onProgressChange(
          eventVideoId || props.track.videoId,
          Number(data.currentTime || props.progress.duration || 0),
          Number(data.duration || props.progress.duration || 0),
          Number(data.bufferedTime || 0),
        );
        if (props.playMode === "repeat") {
          props.onStateChange("buffering");
          props.onPlayingChange(true);
          void callNativePlayer("Replay").catch(markNativePlayerUnavailable);
          return;
        }
        props.onStateChange("ended");
        props.onPlayingChange(false);
        props.onEnded();
        return;
      }
      if (eventVideoId && eventVideoId !== props.track.videoId) {
        const switchedRecently = Date.now() - intendedVideoSinceRef.current < 1800;
        if (data.advertising || data.ad) {
          return;
        }
        if (
          switchedRecently ||
          data.type === "ready" ||
          nextState === "paused" ||
          nextState === "ended" ||
          nextState === "idle"
        ) {
          return;
        }
        if (props.playMode === "repeat") {
          props.onProgressChange(props.track.videoId, 0, 0, 0);
          props.onStateChange("buffering");
          props.onPlayingChange(true);
          playNativeTrack(Date.now(), 0);
          return;
        }
        if (props.playMode === "shuffle") {
          props.onEnded();
          return;
        }
        props.onNativeTrackChange(data);
        return;
      }
      if (typeof data.currentTime === "number" || typeof data.duration === "number") {
        props.onProgressChange(
          eventVideoId || props.track.videoId,
          Number(data.currentTime || 0),
          Number(data.duration || 0),
          Number(data.bufferedTime || 0),
        );
      }
      if (data.type === "ready") {
        return;
      }
      if (nextState === "ended" && props.playMode === "repeat") {
        props.onProgressChange(
          eventVideoId || props.track.videoId,
          0,
          Math.max(0, props.progress.duration),
          0,
        );
        props.onStateChange("buffering");
        props.onPlayingChange(true);
        void callNativePlayer("Replay").catch(markNativePlayerUnavailable);
        return;
      }
      props.onStateChange(nextState);
      props.onPlayingChange(
        nextState === "playing" || nextState === "buffering",
      );
      if (nextState === "ended") {
        props.onEnded();
      } else if (nextState === "error") {
        console.warn(
          isLive
            ? "[DreamFM] YouTube live playback error"
            : "[DreamFM] YouTube Music playback error",
          {
            code: data.code,
            message: data.message,
            reason: data.reason,
            videoId: eventVideoId || props.track.videoId,
            title: props.track.title,
          },
        );
      }
    });
    return () => offPlayerEvent();
  }, [
    callNativePlayer,
    markNativePlayerUnavailable,
    props.enabled,
    props.onEnded,
    props.onNativeTrackChange,
    props.onPlayingChange,
    props.onProgressChange,
    props.onStateChange,
    props.playMode,
    props.text.dreamFm.errorStatus,
    props.progress.duration,
    props.state,
    props.track.videoId,
    props.track.title,
    isLive,
    playerEventName,
    playerEventSource,
    playNativeTrack,
  ]);

  React.useEffect(() => {
    if (
      !props.enabled ||
      props.state !== "loading" ||
      props.playing ||
      props.progress.currentTime > 0.15 ||
      props.progress.bufferedTime > 0.15 ||
      props.progress.duration > 0
    ) {
      return;
    }
    const videoId = props.track.videoId;
    const title = props.track.title;
    const timer = window.setTimeout(() => {
      console.warn("[DreamFM] YouTube playback start timed out", {
        videoId,
        title,
      });
      props.onStateChange("error");
      props.onPlayingChange(false);
    }, 25000);
    return () => window.clearTimeout(timer);
  }, [
    props.enabled,
    props.onPlayingChange,
    props.onStateChange,
    props.playing,
    props.progress.bufferedTime,
    props.progress.currentTime,
    props.progress.duration,
    props.state,
    props.track.title,
    props.track.videoId,
  ]);

  const handleOnlineSeek = React.useCallback(
    (seconds: number) => {
      const duration = Number.isFinite(props.progress.duration)
        ? Math.max(0, props.progress.duration)
        : 0;
      if (!props.enabled || duration <= 0) {
        return;
      }
      const nextTime = Math.max(0, Math.min(seconds, duration));
      props.onProgressChange(
        props.track.videoId,
        nextTime,
        duration,
        Math.max(0, props.progress.bufferedTime),
      );
      if (props.playing) {
        props.onStateChange("buffering");
      }
      void callNativePlayer("Seek", { seconds: nextTime }).catch(
        markNativePlayerUnavailable,
      );
    },
    [
      callNativePlayer,
      markNativePlayerUnavailable,
      props.enabled,
      props.onProgressChange,
      props.onStateChange,
      props.playing,
      props.progress.bufferedTime,
      props.progress.duration,
      props.track.videoId,
    ],
  );

  return (
    <div className="relative h-full min-h-0 overflow-hidden">
      <DreamFMPlayerChrome
        mediaMode={mediaMode}
        reserveWindowControls={props.reserveWindowControls}
        airPlaySupported={props.airPlaySupported}
        cover={
          <DreamFMOnlineArtwork
            key={props.track.id}
            httpBaseURL={props.httpBaseURL}
            track={props.track}
            liveLabel={props.text.dreamFm.liveBadge}
            className="!w-full"
          />
        }
        lyrics={
          <DreamFMLyricsSurface
            text={props.text}
            lyrics={lyricsState.data}
            loading={lyricsState.loading}
            error={lyricsState.error}
            onRetry={lyricsState.error ? retryLyrics : undefined}
            currentTimeMs={Math.max(0, props.progress.currentTime * 1000)}
            timelineRunning={
              props.playing &&
              props.state === "playing" &&
              !playbackAdvertising
            }
          />
        }
        hasVideo={hasVideo}
        videoLoading={videoLoading}
        live={isLive}
        lyricsAvailable={lyricsAvailable || Boolean(lyricsState.error)}
        lyricsLoading={!lyricsAvailable && lyricsState.loading}
        title={props.track.title}
        subtitle={artistName}
        onSubtitleClick={
          artistName && !isLive
            ? () => props.onOpenArtist(props.track)
            : undefined
        }
        infoActions={
          <>
            {showFavoriteAction ? (
              <DreamFMPlayerIconButton
                label={props.text.dreamFm.favorite}
                active={props.favoriteActive}
                className={cn(
                  props.favoriteActive && "text-sidebar-primary",
                )}
                disabled={props.favoriteBusy}
                onClick={props.onToggleFavorite}
              >
                <Heart
                  className={cn(
                    "h-4 w-4",
                    props.favoriteActive && "fill-current",
                  )}
                />
              </DreamFMPlayerIconButton>
            ) : null}
            {showDownloadAction ? (
              <DreamFMPlayerIconButton
                label={props.text.actions.download}
                onClick={() =>
                  props.onDownloadTrack(buildYouTubeWatchURL(props.track.videoId))
                }
              >
                <Download className="h-4 w-4" />
              </DreamFMPlayerIconButton>
            ) : null}
            <DreamFMPlayerMoreMenu
              text={props.text}
              disabled={!trackPageURL}
              onOpenPage={handleOpenTrackPage}
              onCopyLink={handleCopyTrackLink}
            />
          </>
        }
        progress={props.progress}
        advertising={playbackAdvertising}
        advertisingLabel={playbackAdvertisingLabel}
        adSkippable={playbackAdSkippable}
        progressLoading={progressLoading}
        errorActive={props.state === "error"}
        errorLabel={
          props.state === "error"
            ? playbackErrorLabel
            : ""
        }
        errorTitle={props.state === "error" ? playbackErrorMessage : ""}
        onSkipAd={handleSkipAd}
        onSeek={isLive ? undefined : handleOnlineSeek}
        playing={props.playing}
        loading={transportLoading}
        muted={props.muted}
        volume={props.volume}
        playMode={props.playMode}
        text={props.text}
        onAirPlay={props.airPlaySupported ? handleAirPlay : undefined}
        videoOpen={videoOpen}
        onToggleVideo={hasVideo ? handleToggleVideo : undefined}
        onMediaModeChange={setMediaMode}
        onPrevious={props.onPrevious}
        onNext={props.onNext}
        onPlayModeChange={props.onPlayModeChange}
        onTogglePlayback={props.onTogglePlayback}
        onToggleMute={props.onToggleMute}
        onVolumeChange={props.onVolumeChange}
        onToggleQueue={() => setQueueOpen((current) => !current)}
        queueOpen={queueOpen}
        queueOverlay={
          queueOpen ? (
            <DreamFMPlaybackQueuePopup
              queueTitle={props.queueTitle}
              queueItems={props.queueItems}
              selectedQueueId={props.selectedQueueId}
              httpBaseURL={props.httpBaseURL}
              text={props.text}
              onClearQueue={isLive ? undefined : props.onClearQueue}
              onRemoveQueueItem={isLive ? undefined : props.onRemoveQueueItem}
              onSelectQueueTrack={props.onSelectQueueTrack}
              onClose={() => setQueueOpen(false)}
            />
          ) : null
        }
      />
    </div>
  );
}

function DreamFMPlayerChrome(props: {
  mediaMode: DreamFMMediaMode;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  cover: React.ReactNode;
  lyrics: React.ReactNode;
  hasVideo: boolean;
  videoLoading?: boolean;
  live?: boolean;
  lyricsAvailable?: boolean;
  lyricsLoading?: boolean;
  videoOpen?: boolean;
  title: string;
  subtitle: string;
  onSubtitleClick?: () => void;
  infoActions?: React.ReactNode;
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  advertising?: boolean;
  advertisingLabel?: string;
  adSkippable?: boolean;
  progressLoading?: boolean;
  errorActive?: boolean;
  errorLabel?: string;
  errorTitle?: string;
  onSkipAd?: () => void;
  onSeek?: (seconds: number) => void;
  playing: boolean;
  loading: boolean;
  muted: boolean;
  volume: number;
  playMode: DreamFMPlayMode;
  text: ReturnType<typeof getXiaText>;
  onMediaModeChange: (mode: DreamFMMediaMode) => void;
  onAirPlay?: (anchor: DreamFMAirPlayAnchor) => void;
  onToggleVideo?: () => void;
  onPrevious: () => void;
  onNext: () => void;
  onPlayModeChange: (mode: DreamFMPlayMode) => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
  onToggleQueue?: () => void;
  queueOpen?: boolean;
  queueOverlay?: React.ReactNode;
}) {
  const splitMode = props.mediaMode !== "cover";
  const rootRef = React.useRef<HTMLDivElement | null>(null);
  const playerStackRef = React.useRef<HTMLDivElement | null>(null);
  const [layoutWidth, setLayoutWidth] = React.useState(0);
  const [playerStackHeight, setPlayerStackHeight] = React.useState(0);
  const splitEnabled =
    splitMode && layoutWidth >= DREAM_FM_MEDIA_SPLIT_MIN_WIDTH;
  const activeMedia =
    props.mediaMode === "lyrics"
      ? props.lyrics
      : props.cover;
  const mediaStage =
    props.mediaMode === "cover" ? (
      props.cover
    ) : (
      <div
        className={cn(
          "w-full transition-[opacity,transform] duration-300 ease-out",
          splitEnabled ? "h-full" : "aspect-square",
        )}
      >
        {activeMedia}
      </div>
    );

  React.useLayoutEffect(() => {
    const root = rootRef.current;
    const stack = playerStackRef.current;
    if (!root || !stack || typeof ResizeObserver === "undefined") {
      return;
    }
    const syncLayout = () => {
      const rootRect = root.getBoundingClientRect();
      const stackRect = stack.getBoundingClientRect();
      setLayoutWidth(rootRect.width);
      setPlayerStackHeight(stackRect.height);
    };
    syncLayout();
    const observer = new ResizeObserver(syncLayout);
    observer.observe(root);
    observer.observe(stack);
    return () => observer.disconnect();
  }, [props.mediaMode]);

  return (
    <TooltipProvider delayDuration={0}>
      <div ref={rootRef} className="relative flex h-full min-h-0 flex-col overflow-hidden">
        <div className="min-h-0 flex-1 overflow-hidden px-3 pb-2 pt-1 sm:px-5 sm:pb-4">
          <div
            className={cn(
              "mx-auto grid h-full min-h-0 w-full items-center gap-6 transition-[max-width,gap] duration-300 ease-out",
              splitMode
                ? splitEnabled
                  ? "max-w-7xl grid-cols-[minmax(16rem,22rem)_minmax(0,1fr)] gap-6 lg:grid-cols-[minmax(18rem,25rem)_minmax(0,1fr)] lg:gap-8"
                  : "max-w-[min(25rem,100%)] justify-center"
                : "max-w-[min(25rem,100%)] justify-center",
            )}
          >
            <div
              className={cn(
                "min-w-0 transition-transform duration-300 ease-out",
                splitEnabled ? "justify-self-start" : "justify-self-center",
              )}
            >
              <div
                ref={playerStackRef}
                className={cn("mx-auto", DREAM_FM_PLAYER_SURFACE_WIDTH_CLASS)}
              >
                <div className={cn(splitMode && splitEnabled && "hidden")}>
                  {splitMode && splitEnabled ? null : mediaStage}
                </div>
                {splitMode && splitEnabled ? (
                  <div className={cn(splitEnabled ? "block animate-in fade-in-0 zoom-in-95 duration-300" : "hidden")}>
                    {props.cover}
                  </div>
                ) : null}
                <DreamFMTrackInfoRow
                  title={props.title}
                  subtitle={props.subtitle}
                  onSubtitleClick={props.onSubtitleClick}
                  actions={props.infoActions}
                />
                <DreamFMPlayerProgress
                  progress={props.progress}
                  text={props.text}
                  live={props.live}
                  playing={props.playing}
                  advertising={props.advertising}
                  advertisingLabel={props.advertisingLabel}
                  adSkippable={props.adSkippable}
                  loading={props.progressLoading}
                  errorActive={props.errorActive}
                  errorLabel={props.errorLabel}
                  errorTitle={props.errorTitle}
                  onSkipAd={props.onSkipAd}
                  onSeek={props.onSeek}
                />
                <DreamFMPlayerTransport
                  playing={props.playing}
                  loading={props.loading}
                  playMode={props.playMode}
                  live={props.live}
                  text={props.text}
                  onPrevious={props.onPrevious}
                  onNext={props.onNext}
                  onPlayModeChange={props.onPlayModeChange}
                  onTogglePlayback={props.onTogglePlayback}
                />
                <DreamFMPlayerVolume
                  muted={props.muted}
                  volume={props.volume}
                  text={props.text}
                  onToggleMute={props.onToggleMute}
                  onVolumeChange={props.onVolumeChange}
                />
              </div>
            </div>
            {splitMode ? (
              <div
                className={cn(
                  "min-h-0 transition-[opacity,transform] duration-300 ease-out",
                  splitEnabled
                    ? "flex h-full translate-x-0 items-center justify-center opacity-100"
                    : "pointer-events-none hidden translate-x-4 opacity-0",
                )}
                style={
                  splitEnabled && playerStackHeight > 0
                    ? { height: playerStackHeight }
                    : undefined
                }
              >
                <div className="h-full w-full max-w-[46rem] animate-in fade-in-0 slide-in-from-right-3 duration-300">
                  {splitEnabled ? mediaStage : null}
                </div>
              </div>
            ) : null}
          </div>
        </div>
        {props.queueOverlay}
        <DreamFMPlayerFooter
          mediaMode={props.mediaMode}
          reserveWindowControls={props.reserveWindowControls}
          airPlaySupported={props.airPlaySupported}
          hasVideo={props.hasVideo}
          videoLoading={props.videoLoading}
          lyricsAvailable={props.lyricsAvailable !== false}
          lyricsLoading={props.lyricsLoading}
          videoOpen={props.videoOpen}
          queueOpen={props.queueOpen}
          text={props.text}
          onAirPlay={props.onAirPlay}
          onToggleVideo={props.onToggleVideo}
          onMediaModeChange={props.onMediaModeChange}
          onToggleQueue={props.onToggleQueue}
        />
      </div>
    </TooltipProvider>
  );
}

function DreamFMTrackInfoRow(props: {
  title: string;
  subtitle: string;
  onSubtitleClick?: () => void;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mt-5 flex min-h-14 items-center justify-between gap-4">
      <div className="min-w-0 flex-1 text-left">
        <DreamFMScrollingText
          text={props.title}
          className="text-lg font-semibold leading-6 text-sidebar-foreground"
        />
        <DreamFMScrollingText
          text={props.subtitle}
          className="mt-0.5 text-sm leading-5 text-sidebar-foreground/58"
          onClick={props.onSubtitleClick}
        />
      </div>
      {props.actions ? (
        <div className="flex shrink-0 items-center gap-1.5">{props.actions}</div>
      ) : null}
    </div>
  );
}

function DreamFMScrollingText(props: {
  text: string;
  className?: string;
  onClick?: () => void;
}) {
  const containerRef = React.useRef<HTMLElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const normalizedText = props.text.trim();
  const scrolling = overflow > 1;
  const style = scrolling
    ? ({
        "--dream-fm-marquee-shift": `-${Math.ceil(overflow + 18)}px`,
        "--dream-fm-marquee-duration": `${Math.min(
          14,
          Math.max(7, (overflow + 180) / 30),
        )}s`,
      } as React.CSSProperties)
    : undefined;
  const className = cn(
    "group/dream-fm-marquee relative block min-w-0 overflow-hidden whitespace-nowrap text-left",
    props.onClick &&
      "rounded-md underline-offset-4 transition hover:text-sidebar-foreground hover:underline focus-visible:outline-none",
    props.className,
  );
  const content = (
    <span
      ref={contentRef}
      className={cn(
        "inline-block max-w-none pr-4 align-top",
        scrolling && "dream-fm-marquee-text",
      )}
      style={style}
    >
      {normalizedText}
    </span>
  );

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    const contentElement = contentRef.current;
    if (!container || !contentElement) {
      return;
    }
    const syncOverflow = () => {
      setOverflow(
        Math.max(0, contentElement.scrollWidth - container.clientWidth),
      );
    };
    syncOverflow();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(syncOverflow);
    observer.observe(container);
    observer.observe(contentElement);
    return () => observer.disconnect();
  }, [normalizedText]);

  if (props.onClick) {
    return (
      <button
        ref={containerRef as React.RefObject<HTMLButtonElement>}
        type="button"
        className={className}
        title={normalizedText}
        onClick={props.onClick}
      >
        {content}
      </button>
    );
  }

  return (
    <div
      ref={containerRef as React.RefObject<HTMLDivElement>}
      className={className}
      title={normalizedText}
    >
      {content}
    </div>
  );
}

function DreamFMPlayerProgress(props: {
  progress: {
    currentTime: number;
    duration: number;
    bufferedTime: number;
  };
  text: ReturnType<typeof getXiaText>;
  live?: boolean;
  playing?: boolean;
  advertising?: boolean;
  advertisingLabel?: string;
  adSkippable?: boolean;
  loading?: boolean;
  errorActive?: boolean;
  errorLabel?: string;
  errorTitle?: string;
  onSkipAd?: () => void;
  onSeek?: (seconds: number) => void;
}) {
  const duration = Number.isFinite(props.progress.duration)
    ? Math.max(0, props.progress.duration)
    : 0;
  const currentTime = Math.max(
    0,
    Math.min(
      Number.isFinite(props.progress.currentTime)
        ? props.progress.currentTime
        : 0,
      duration,
    ),
  );
  const bufferedPercent =
    duration > 0
      ? Math.max(0, Math.min(100, (props.progress.bufferedTime / duration) * 100))
      : 0;
  const playedPercent =
    duration > 0 ? Math.max(0, Math.min(100, (currentTime / duration) * 100)) : 0;
  const canSeek = duration > 0 && Boolean(props.onSeek);
  const remainingTime = Math.max(0, duration - currentTime);
  const errorCode = props.errorLabel?.trim() || "";
  const errorMessage = props.errorTitle?.trim() || "";
  const hasError = Boolean(props.errorActive || errorCode || errorMessage);
  const advertising = Boolean(props.advertising && !hasError);
  const loading = Boolean(props.loading && !hasError && !advertising);
  const statusActive = props.live || hasError || advertising || loading;
  const errorLabel = errorCode
    ? `${props.text.dreamFm.errorCodeLabel}: ${errorCode}`
    : props.text.dreamFm.errorStatus;
  const errorTooltip = errorMessage || errorLabel;
  const label = advertising
    ? props.advertisingLabel?.trim() || props.text.dreamFm.adBadge
    : loading
      ? props.text.dreamFm.loading
      : props.text.dreamFm.liveBadge;
  const skipAvailable =
    !hasError && !loading && advertising && props.adSkippable && props.onSkipAd;
  const hasTimedAdProgress =
    advertising &&
    duration > 0 &&
    (playedPercent > 0 || bufferedPercent > 0);
  const livePlaying = Boolean(props.playing && !loading && !advertising && !hasError);

  if (statusActive) {
    return (
      <div className="mt-4">
        <div className="relative flex h-6 items-center">
          <div className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 rounded-full bg-sidebar-foreground/10">
            {hasError ? null : advertising && hasTimedAdProgress ? (
              <>
                <div
                  className="h-full rounded-full bg-sidebar-foreground/12"
                  style={{ width: `${bufferedPercent}%` }}
                />
                <div
                  className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
                  style={{ width: `${playedPercent}%` }}
                />
              </>
            ) : advertising ? (
              <div className="h-full w-full rounded-full bg-red-500/42 dark:bg-red-400/42" />
            ) : loading ? (
              <div className="h-full w-full animate-pulse rounded-full bg-sidebar-primary/45" />
            ) : props.live ? (
              <div
                className={cn(
                  "relative h-full w-full rounded-full",
                  livePlaying
                    ? "bg-sidebar-primary/72"
                    : "bg-sidebar-primary/34",
                )}
              >
                <span
                  className={cn(
                    "absolute right-0 top-1/2 block h-2.5 w-2.5 -translate-y-1/2 translate-x-1/2 rounded-full bg-sidebar-primary shadow-[0_0_0_3px_hsl(var(--sidebar-background)/0.88)]",
                    livePlaying ? "opacity-100" : "opacity-55",
                  )}
                >
                  {livePlaying ? (
                    <span className="absolute inset-0 rounded-full bg-sidebar-primary/55 animate-ping" />
                  ) : null}
                </span>
              </div>
            ) : (
              <div
                className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
                style={{ width: `${playedPercent}%` }}
              />
            )}
          </div>
        </div>
        <div className="mt-0.5 grid h-4 grid-cols-[1fr_auto_1fr] items-center text-[11px] font-medium tabular-nums text-sidebar-foreground/46">
          {hasError ? (
            <>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="min-w-0 truncate text-left font-semibold text-red-600 dark:text-red-300">
                    {errorLabel}
                  </span>
                </TooltipTrigger>
                <TooltipContent side="top" multiline className="text-left text-xs leading-snug">
                  {errorTooltip}
                </TooltipContent>
              </Tooltip>
              <span aria-hidden="true" />
              <span aria-hidden="true" />
            </>
          ) : advertising ? (
            <>
              <span className="min-w-0 truncate text-left font-semibold text-red-600 dark:text-red-300">
                {label}
              </span>
              <span aria-hidden="true" />
              {skipAvailable ? (
                <button
                  type="button"
                  className="justify-self-end rounded px-1.5 text-[10px] font-semibold leading-4 text-sidebar-primary transition hover:bg-sidebar-primary/10 hover:text-sidebar-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-primary/35"
                  onClick={() => props.onSkipAd?.()}
                >
                  {props.text.dreamFm.skipAd}
                </button>
              ) : (
                <span aria-hidden="true" />
              )}
            </>
          ) : loading ? (
            <>
              <span aria-hidden="true" />
              <span className="justify-self-center font-semibold text-sidebar-foreground/55">
                {label}
              </span>
              <span aria-hidden="true" />
            </>
          ) : (
            <>
              <span aria-hidden="true" />
              <span className="justify-self-center font-semibold uppercase tracking-[0.12em] text-red-600 dark:text-red-300">
                {label}
              </span>
              <span aria-hidden="true" />
            </>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="mt-4">
      <div className="group/progress relative flex h-6 items-center">
        <div className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full bg-sidebar-foreground/10">
          <div
            className="h-full rounded-full bg-sidebar-foreground/12"
            style={{ width: `${bufferedPercent}%` }}
          />
          <div
            className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary"
            style={{ width: `${playedPercent}%` }}
          />
        </div>
        {canSeek ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 scale-75 rounded-full border border-sidebar-background bg-sidebar-primary opacity-0 shadow-[0_5px_14px_-8px_hsl(var(--sidebar-primary)/0.9)] transition-[left,opacity,transform] duration-150 ease-out group-hover/progress:scale-100 group-hover/progress:opacity-100 group-focus-within/progress:scale-100 group-focus-within/progress:opacity-100"
            style={{ left: `${playedPercent}%` }}
          />
        ) : null}
        <input
          type="range"
          min={0}
          max={duration || 0}
          step={0.01}
          value={currentTime}
          disabled={!canSeek}
          aria-label={props.text.dreamFm.nowPlaying}
          className="relative z-10 h-6 w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
          onChange={(event) => props.onSeek?.(Number(event.target.value))}
        />
      </div>
      <div className="mt-0.5 flex items-center justify-between text-[11px] font-medium tabular-nums text-sidebar-foreground/46">
        <span>{formatProgressSeconds(currentTime)}</span>
        <span>-{formatProgressSeconds(remainingTime)}</span>
      </div>
    </div>
  );
}

function DreamFMPlayerTransport(props: {
  playing: boolean;
  loading: boolean;
  playMode: DreamFMPlayMode;
  live?: boolean;
  text: ReturnType<typeof getXiaText>;
  onPrevious: () => void;
  onNext: () => void;
  onPlayModeChange: (mode: DreamFMPlayMode) => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const shuffleActive = props.playMode === "shuffle";
  const repeatActive = !props.live && props.playMode === "repeat";
  const playLabel = props.playing ? props.text.dreamFm.pause : props.text.dreamFm.play;

  return (
    <div className="mt-3 grid h-14 grid-cols-[3.5rem_1fr_3.5rem] items-center">
      <div className="justify-self-start">
        <DreamFMTransportIconButton
          label={props.text.dreamFm.playModeShuffle}
          active={shuffleActive}
          size="small"
          onClick={() => props.onPlayModeChange(shuffleActive ? "order" : "shuffle")}
        >
          <Shuffle className="h-4 w-4" />
        </DreamFMTransportIconButton>
      </div>
      <div className="flex items-center justify-center gap-3">
        <DreamFMTransportIconButton label={props.text.dreamFm.previous} onClick={props.onPrevious}>
          <SkipBack className="h-5 w-5" />
        </DreamFMTransportIconButton>
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="flex h-12 w-12 items-center justify-center rounded-full bg-sidebar-primary text-sidebar-primary-foreground shadow-[0_18px_42px_-28px_hsl(var(--sidebar-primary)/0.72)] transition-[transform,box-shadow,background-color] duration-200 ease-out hover:scale-[1.04] hover:shadow-[0_20px_46px_-26px_hsl(var(--sidebar-primary)/0.82)] active:scale-95 focus-visible:outline-none"
              aria-label={playLabel}
              title={playLabel}
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2 className="h-5 w-5 animate-spin" />
              ) : props.playing ? (
                <Pause className="h-5 w-5 fill-current" />
              ) : (
                <Play className="ml-0.5 h-5 w-5 fill-current" />
              )}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{playLabel}</TooltipContent>
        </Tooltip>
        <DreamFMTransportIconButton label={props.text.dreamFm.next} onClick={props.onNext}>
          <SkipForward className="h-5 w-5" />
        </DreamFMTransportIconButton>
      </div>
      <div className="justify-self-end">
        <DreamFMTransportIconButton
          label={props.text.dreamFm.playModeRepeat}
          active={repeatActive}
          disabled={props.live}
          size="small"
          onClick={() => props.onPlayModeChange(repeatActive ? "order" : "repeat")}
        >
          <Repeat2 className="h-4 w-4" />
        </DreamFMTransportIconButton>
      </div>
    </div>
  );
}

function DreamFMTransportIconButton(props: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  size?: "normal" | "small";
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const small = props.size === "small";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          data-active={props.active ? "true" : "false"}
          disabled={props.disabled}
          className={cn(
            "relative flex items-center justify-center rounded-full text-sidebar-foreground/55 transition-[transform,background-color,color,opacity] duration-200 ease-out hover:scale-[1.05] hover:bg-sidebar-background/54 hover:text-sidebar-foreground active:scale-95 focus-visible:outline-none",
            "data-[active=true]:text-sidebar-primary",
            "disabled:pointer-events-none disabled:opacity-35",
            small ? "h-8 w-8" : "h-10 w-10",
          )}
          aria-label={props.label}
          title={props.label}
          onClick={props.onClick}
        >
          {props.children}
          {props.active ? (
            <span className="absolute bottom-0 h-1 w-1 rounded-full bg-sidebar-primary" />
          ) : null}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top">
        {props.label}
      </TooltipContent>
    </Tooltip>
  );
}

function DreamFMPlayerVolume(props: {
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const visibleVolume = props.muted ? 0 : clampVolume(props.volume);
  const volumePercent = Math.round(visibleVolume * 1000) / 10;

  return (
    <div className="mt-4 flex h-8 items-center gap-3 text-sidebar-foreground/48">
      <DreamFMPlayerIconButton
        label={props.muted || props.volume <= 0 ? props.text.dreamFm.unmute : props.text.dreamFm.mute}
        className="h-8 w-8 shadow-none"
        onClick={props.onToggleMute}
      >
        {props.muted || props.volume <= 0 ? (
          <VolumeX className="h-4 w-4" />
        ) : (
          <Volume2 className="h-4 w-4" />
        )}
      </DreamFMPlayerIconButton>
      <div className="group/volume-slider relative flex h-6 min-w-0 flex-1 items-center">
        <div className="pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden rounded-full bg-sidebar-foreground/10">
          <div
            className="absolute inset-y-0 left-0 rounded-full bg-sidebar-primary transition-[width] duration-150 ease-out"
            style={{ width: `${volumePercent}%` }}
          />
        </div>
        <span
          aria-hidden="true"
          className="pointer-events-none absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 scale-75 rounded-full border border-sidebar-background bg-sidebar-primary opacity-0 shadow-[0_5px_14px_-8px_hsl(var(--sidebar-primary)/0.9)] transition-[left,opacity,transform] duration-150 ease-out group-hover/volume-slider:scale-100 group-hover/volume-slider:opacity-100 group-focus-within/volume-slider:scale-100 group-focus-within/volume-slider:opacity-100"
          style={{ left: `${volumePercent}%` }}
        />
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={visibleVolume}
          aria-label={props.text.dreamFm.volume}
          title={props.text.dreamFm.volume}
          className="relative z-10 h-6 w-full cursor-pointer opacity-0"
          onChange={(event) => props.onVolumeChange(Number(event.target.value))}
        />
      </div>
      <DreamFMPlayerIconButton
        label={props.text.dreamFm.volume}
        className="h-8 w-8 shadow-none"
        onClick={() => props.onVolumeChange(1)}
      >
        <Volume2 className="h-4 w-4" />
      </DreamFMPlayerIconButton>
    </div>
  );
}

function DreamFMPlayerFooter(props: {
  mediaMode: DreamFMMediaMode;
  reserveWindowControls: boolean;
  airPlaySupported: boolean;
  hasVideo: boolean;
  videoLoading?: boolean;
  lyricsAvailable: boolean;
  lyricsLoading?: boolean;
  videoOpen?: boolean;
  queueOpen?: boolean;
  text: ReturnType<typeof getXiaText>;
  onAirPlay?: (anchor: DreamFMAirPlayAnchor) => void;
  onToggleVideo?: () => void;
  onMediaModeChange: (mode: DreamFMMediaMode) => void;
  onToggleQueue?: () => void;
}) {
  const toggleMediaMode = (mode: DreamFMMediaMode) => {
    props.onMediaModeChange(props.mediaMode === mode ? "cover" : mode);
  };
  const handleAirPlayClick: React.MouseEventHandler<HTMLButtonElement> = (event) => {
    const rect = event.currentTarget.getBoundingClientRect();
    props.onAirPlay?.({
      x: rect.left,
      y: rect.top,
      width: rect.width,
      height: rect.height,
    });
  };

  return (
    <footer className="relative z-20 shrink-0 px-0 pb-1 pt-2 sm:pb-2">
      <div
        className="flex h-12 w-full items-center justify-between gap-3 pl-3 pr-3"
      >
        <div className="flex shrink-0 items-center gap-1">
          <DreamFMPlayerIconButton
            label={props.text.dreamFm.airPlay}
            disabled={!props.airPlaySupported || !props.onAirPlay}
            className={DREAM_FM_PLAYER_FOOTER_ICON_BUTTON_CLASS}
            onClick={handleAirPlayClick}
          >
            <Airplay className="h-4 w-4" />
          </DreamFMPlayerIconButton>
          <DreamFMPlayerIconButton
            label={
              props.videoLoading
                ? props.text.dreamFm.loading
                : props.hasVideo
                ? props.text.dreamFm.video
                : props.text.dreamFm.noVideo
            }
            active={props.hasVideo && props.videoOpen}
            disabled={props.videoLoading || !props.hasVideo || !props.onToggleVideo}
            className={DREAM_FM_PLAYER_FOOTER_ICON_BUTTON_CLASS}
            onClick={props.onToggleVideo}
          >
            {props.videoLoading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Video className="h-4 w-4" />
            )}
          </DreamFMPlayerIconButton>
        </div>
        <div className="flex min-w-0 items-center justify-end gap-1">
          <DreamFMPlayerIconButton
            label={props.text.dreamFm.lyrics}
            active={props.mediaMode === "lyrics"}
            disabled={
              !props.lyricsAvailable && props.mediaMode !== "lyrics"
            }
            className={DREAM_FM_PLAYER_FOOTER_ICON_BUTTON_CLASS}
            onClick={() => toggleMediaMode("lyrics")}
          >
            {props.lyricsLoading ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Captions className="h-4 w-4" />
            )}
          </DreamFMPlayerIconButton>
          <DreamFMPlayerIconButton
            label={props.text.dreamFm.upNext}
            active={props.queueOpen}
            disabled={!props.onToggleQueue}
            className={DREAM_FM_PLAYER_FOOTER_ICON_BUTTON_CLASS}
            onClick={props.onToggleQueue}
          >
            <ListMusic className="h-4 w-4" />
          </DreamFMPlayerIconButton>
        </div>
      </div>
    </footer>
  );
}

function DreamFMPlayerMoreMenu(props: {
  text: ReturnType<typeof getXiaText>;
  disabled?: boolean;
  onOpenPage: () => void;
  onCopyLink: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          disabled={props.disabled}
          className={cn(DREAM_FM_PLAYER_ICON_BUTTON_CLASS)}
          aria-label={props.text.dreamFm.more}
        >
          <MoreHorizontal className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side="bottom"
        align="end"
        sideOffset={8}
        className={DREAM_FM_DROPDOWN_CONTENT_CLASS}
      >
        <div className="p-1">
          <DropdownMenuItem
            className={DREAM_FM_DROPDOWN_ITEM_CLASS}
            disabled={props.disabled}
            onSelect={props.onOpenPage}
          >
            <div className={DREAM_FM_DROPDOWN_ICON_SLOT_CLASS}>
              <ExternalLink className="h-3.5 w-3.5" />
            </div>
            <span className="truncate">
              {props.text.dreamFm.openPage}
            </span>
          </DropdownMenuItem>
          <DropdownMenuItem
            className={DREAM_FM_DROPDOWN_ITEM_CLASS}
            disabled={props.disabled}
            onSelect={props.onCopyLink}
          >
            <div className={DREAM_FM_DROPDOWN_ICON_SLOT_CLASS}>
              <Copy className="h-3.5 w-3.5" />
            </div>
            <span className="truncate">
              {props.text.dreamFm.copyLink}
            </span>
          </DropdownMenuItem>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function DreamFMPlayerIconButton(props: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  className?: string;
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex">
          <button
            type="button"
            data-active={props.active ? "true" : "false"}
            disabled={props.disabled}
            className={cn(
              DREAM_FM_PLAYER_ICON_BUTTON_CLASS,
              props.className,
            )}
            aria-label={props.label}
            title={props.label}
            onClick={props.onClick}
          >
            {props.children}
          </button>
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">
        {props.label}
      </TooltipContent>
    </Tooltip>
  );
}

function DreamFMLocalCoverSurface(props: { src: string; title: string }) {
  return (
    <DreamFMArtworkShell className="!w-full">
      <img
        src={props.src}
        alt={props.title}
        className="h-full w-full object-cover transition-transform duration-500 ease-out group-hover/dream-fm-artwork:scale-[1.035]"
        loading="eager"
      />
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.02),rgba(15,23,42,0.12))]" />
    </DreamFMArtworkShell>
  );
}

function DreamFMLyricsSurface(props: {
  text: ReturnType<typeof getXiaText>;
  lyrics?: DreamFMLyricsData | null;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
  currentTimeMs?: number;
  timelineRunning?: boolean;
}) {
  const activeLineRef = React.useRef<HTMLDivElement | null>(null);
  const scrollContainerRef = React.useRef<HTMLDivElement | null>(null);
  const scrollAnimationRef = React.useRef<number | null>(null);
  const scrollMeasureFrameRef = React.useRef<number | null>(null);
  const programmaticScrollUntilRef = React.useRef(0);
  const manualScrollUnlockTimerRef = React.useRef<number | null>(null);
  const manualScrollLockedRef = React.useRef(false);
  const visualClockRef = React.useRef({
    sourceMs: 0,
    anchorMs: 0,
    running: false,
    key: "",
  });
  const lastCenteredLineRef = React.useRef<{
    videoId: string;
    activeIndex: number;
    timeMs: number;
  } | null>(null);
  const [lyricsViewportPadding, setLyricsViewportPadding] =
    React.useState(32);
  const [manualScrollLocked, setManualScrollLocked] = React.useState(false);
  const lyrics = props.lyrics;
  const syncedLines = lyrics?.kind === "synced" ? lyrics.lines : [];
  const currentTimeMs = Math.max(0, props.currentTimeMs ?? 0);
  const timelineRunning = props.timelineRunning === true;
  const timelineClockKey =
    lyrics?.kind === "synced" ? `${lyrics.videoId}:synced` : "";
  const [visualCurrentTimeMs, setVisualCurrentTimeMs] =
    React.useState(currentTimeMs);
  const visualCurrentTimeRef = React.useRef(visualCurrentTimeMs);
  const timelineLines = React.useMemo(
    () => buildDreamFMLyricsTimeline(syncedLines),
    [syncedLines],
  );
  const timelineClockRunning =
    timelineRunning && timelineLines.length > 0 && Boolean(timelineClockKey);
  const activeIndex = React.useMemo(() => {
    if (timelineLines.length === 0) {
      return -1;
    }
    return findDreamFMActiveLyricLineIndex(
      timelineLines,
      visualCurrentTimeMs,
    );
  }, [timelineLines, visualCurrentTimeMs]);

  React.useEffect(() => {
    manualScrollLockedRef.current = manualScrollLocked;
  }, [manualScrollLocked]);

  visualCurrentTimeRef.current = visualCurrentTimeMs;

  React.useEffect(() => {
    manualScrollLockedRef.current = false;
    setManualScrollLocked(false);
    if (manualScrollUnlockTimerRef.current !== null) {
      window.clearTimeout(manualScrollUnlockTimerRef.current);
      manualScrollUnlockTimerRef.current = null;
    }
  }, [lyrics?.videoId]);

  const cancelLyricScrollAnimation = React.useCallback(() => {
    if (scrollAnimationRef.current !== null) {
      window.cancelAnimationFrame(scrollAnimationRef.current);
      scrollAnimationRef.current = null;
    }
  }, []);

  const centerActiveLyricLine = React.useCallback(
    (behavior: "auto" | "smooth") => {
      const container = scrollContainerRef.current;
      const line = activeLineRef.current;
      if (!container || !line || activeIndex < 0) {
        return;
      }
      if (manualScrollLockedRef.current) {
        return;
      }
      const containerRect = container.getBoundingClientRect();
      const lineRect = line.getBoundingClientRect();
      const target = Math.max(
        0,
        Math.min(
          container.scrollTop +
            lineRect.top -
            containerRect.top +
            lineRect.height / 2 -
            container.clientHeight / 2,
          Math.max(0, container.scrollHeight - container.clientHeight),
        ),
      );
      if (Math.abs(container.scrollTop - target) < 0.75) {
        return;
      }
      const reducedMotion =
        typeof window.matchMedia === "function" &&
        window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      cancelLyricScrollAnimation();
      if (behavior === "auto" || reducedMotion) {
        programmaticScrollUntilRef.current = performance.now() + 220;
        container.scrollTop = target;
        return;
      }
      const startTop = container.scrollTop;
      const distance = target - startTop;
      const startedAt = performance.now();
      const tick = (now: number) => {
        const progress =
          (now - startedAt) / DREAM_FM_LYRICS_SCROLL_DURATION_MS;
        programmaticScrollUntilRef.current = now + 220;
        container.scrollTop =
          startTop + distance * easeOutDreamFMLyricsScroll(progress);
        if (progress < 1) {
          scrollAnimationRef.current = window.requestAnimationFrame(tick);
        } else {
          scrollAnimationRef.current = null;
          programmaticScrollUntilRef.current = performance.now() + 360;
          container.scrollTop = target;
        }
      };
      scrollAnimationRef.current = window.requestAnimationFrame(tick);
    },
    [activeIndex, cancelLyricScrollAnimation],
  );

  const scheduleActiveLyricCenter = React.useCallback(
    (behavior: "auto" | "smooth") => {
      if (scrollMeasureFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollMeasureFrameRef.current);
      }
      scrollMeasureFrameRef.current = window.requestAnimationFrame(() => {
        scrollMeasureFrameRef.current = null;
        centerActiveLyricLine(behavior);
      });
    },
    [centerActiveLyricLine],
  );

  React.useEffect(() => {
    const now = performance.now();
    const clock = visualClockRef.current;
    const predicted =
      clock.running && clock.key === timelineClockKey
        ? clock.sourceMs + now - clock.anchorMs
        : clock.sourceMs;
    const drift = currentTimeMs - predicted;
    const nextSourceMs =
      timelineClockRunning &&
      clock.key === timelineClockKey &&
      Math.abs(drift) < 500
        ? Math.max(0, predicted + drift * 0.35)
        : currentTimeMs;
    visualClockRef.current = {
      sourceMs: nextSourceMs,
      anchorMs: now,
      running: timelineClockRunning,
      key: timelineClockKey,
    };
    setVisualCurrentTimeMs(nextSourceMs);
  }, [currentTimeMs, timelineClockKey, timelineClockRunning]);

  React.useEffect(() => {
    if (!timelineClockRunning || !timelineClockKey) {
      return;
    }
    let frame = 0;
    let lastPaintAt = 0;
    const tick = (now: number) => {
      const clock = visualClockRef.current;
      if (
        clock.running &&
        clock.key === timelineClockKey &&
        now - lastPaintAt >= 32
      ) {
        lastPaintAt = now;
        setVisualCurrentTimeMs(
          Math.max(0, clock.sourceMs + now - clock.anchorMs),
        );
      }
      frame = window.requestAnimationFrame(tick);
    };
    frame = window.requestAnimationFrame(tick);
    return () => window.cancelAnimationFrame(frame);
  }, [timelineClockKey, timelineClockRunning]);

  React.useEffect(() => {
    return () => {
      cancelLyricScrollAnimation();
      if (scrollMeasureFrameRef.current !== null) {
        window.cancelAnimationFrame(scrollMeasureFrameRef.current);
        scrollMeasureFrameRef.current = null;
      }
      if (manualScrollUnlockTimerRef.current !== null) {
        window.clearTimeout(manualScrollUnlockTimerRef.current);
        manualScrollUnlockTimerRef.current = null;
      }
    };
  }, [cancelLyricScrollAnimation]);

  React.useEffect(() => {
    const container = scrollContainerRef.current;
    if (!container) {
      return;
    }
    const handleScroll = () => {
      if (performance.now() < programmaticScrollUntilRef.current) {
        return;
      }
      manualScrollLockedRef.current = true;
      setManualScrollLocked(true);
      if (manualScrollUnlockTimerRef.current !== null) {
        window.clearTimeout(manualScrollUnlockTimerRef.current);
      }
      manualScrollUnlockTimerRef.current = window.setTimeout(() => {
        manualScrollLockedRef.current = false;
        setManualScrollLocked(false);
        scheduleActiveLyricCenter("smooth");
      }, DREAM_FM_LYRICS_MANUAL_SCROLL_LOCK_MS);
    };
    container.addEventListener("scroll", handleScroll, { passive: true });
    return () => container.removeEventListener("scroll", handleScroll);
  }, [lyrics?.videoId, scheduleActiveLyricCenter]);

  React.useLayoutEffect(() => {
    const container = scrollContainerRef.current;
    if (!container || timelineLines.length === 0) {
      return;
    }
    const syncPadding = () => {
      const nextPadding = Math.max(24, container.clientHeight / 2 - 44);
      setLyricsViewportPadding((current) =>
        Math.abs(current - nextPadding) < 1 ? current : nextPadding,
      );
    };
    syncPadding();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(() => {
      syncPadding();
      scheduleActiveLyricCenter("auto");
    });
    observer.observe(container);
    if (activeLineRef.current) {
      observer.observe(activeLineRef.current);
    }
    return () => observer.disconnect();
  }, [activeIndex, scheduleActiveLyricCenter, timelineLines.length]);

  React.useLayoutEffect(() => {
    if (activeIndex < 0) {
      lastCenteredLineRef.current = null;
      return;
    }
    const videoId = lyrics?.videoId ?? "";
    const previous = lastCenteredLineRef.current;
    const lineChanged =
      !previous ||
      previous.videoId !== videoId ||
      previous.activeIndex !== activeIndex;
    const nearbyTimelineMove =
      previous?.videoId === videoId &&
      Math.abs(previous.activeIndex - activeIndex) <= 3 &&
      Math.abs(previous.timeMs - visualCurrentTimeRef.current) < 8000;
    lastCenteredLineRef.current = {
      videoId,
      activeIndex,
      timeMs: visualCurrentTimeRef.current,
    };
    scheduleActiveLyricCenter(
      lineChanged && nearbyTimelineMove ? "smooth" : "auto",
    );
  }, [
    activeIndex,
    lyrics?.videoId,
    lyricsViewportPadding,
    scheduleActiveLyricCenter,
  ]);

  if (props.loading) {
    return (
      <DreamFMLyricsState text={props.text}>
        <Loader2 className="h-6 w-6 animate-spin" />
        <span>{props.text.dreamFm.loading}</span>
      </DreamFMLyricsState>
    );
  }

  if (props.error) {
    return (
      <DreamFMLyricsState text={props.text}>
        <Captions className="h-6 w-6" />
        <span className="whitespace-pre-wrap">{props.error}</span>
        {props.onRetry ? (
          <button
            type="button"
            className="mt-3 inline-flex h-8 items-center justify-center gap-1.5 rounded-full bg-sidebar-primary/10 px-3 text-xs font-semibold text-sidebar-primary transition hover:bg-sidebar-primary/16 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-sidebar-primary/40"
            onClick={props.onRetry}
          >
            <RotateCcw className="h-3.5 w-3.5" />
            <span>{props.text.dreamFm.retry}</span>
          </button>
        ) : null}
      </DreamFMLyricsState>
    );
  }

  if (!lyrics || lyrics.kind === "unavailable") {
    return (
      <DreamFMLyricsState text={props.text}>
        <Captions className="h-6 w-6" />
        <span>{props.text.dreamFm.lyricsEmpty}</span>
      </DreamFMLyricsState>
    );
  }

  if (lyrics.kind === "plain") {
    const lines = lyrics.text
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean);
    if (lines.length === 0) {
      return (
        <DreamFMLyricsState text={props.text}>
          <Captions className="h-6 w-6" />
          <span>{props.text.dreamFm.lyricsEmpty}</span>
        </DreamFMLyricsState>
      );
    }
    return (
      <div className="relative h-full w-full overflow-hidden">
        <div className="h-full overflow-y-auto px-3 py-5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:px-5">
          <div className="mx-auto max-w-2xl space-y-4 pb-10 pt-2 text-left">
            {lines.map((line, index) => (
              <div
                key={`${index}-${line}`}
                className="break-words text-lg font-semibold leading-8 text-sidebar-foreground/78 sm:text-xl sm:leading-9"
              >
                {line}
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  if (timelineLines.length === 0) {
    return (
      <DreamFMLyricsState text={props.text}>
        <Captions className="h-6 w-6" />
        <span>{props.text.dreamFm.lyricsEmpty}</span>
      </DreamFMLyricsState>
    );
  }

  return (
    <div className="relative h-full w-full overflow-hidden">
      <div
        ref={scrollContainerRef}
        className="h-full overflow-y-auto px-3 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:px-5"
      >
        <div
          className="mx-auto max-w-2xl text-left"
          style={{
            paddingBottom: lyricsViewportPadding + 24,
            paddingTop: lyricsViewportPadding,
          }}
        >
          {timelineLines.map((line, index) => {
            const active = index === activeIndex;
            const previous =
              !active &&
              line.endMs + DREAM_FM_LYRICS_LINE_GRACE_MS <
                visualCurrentTimeMs;
            const text = line.text;
            const words = line.words;
            if (!text) {
              return (
                <div
                  key={`${line.startMs}-${line.sourceIndex}-${index}`}
                  className="h-8"
                />
              );
            }
            return (
              <div
                key={`${line.startMs}-${line.sourceIndex}-${text}`}
                ref={active ? activeLineRef : undefined}
                className={cn(
                  "origin-left break-words py-2.5 text-xl font-semibold leading-9 transition-[color,opacity,transform] duration-500 ease-out will-change-transform sm:text-2xl sm:leading-10",
                  active
                    ? "translate-x-2 scale-100 text-[hsl(var(--chart-1))] opacity-100"
                    : previous
                      ? "translate-x-0 scale-[0.92] text-[hsl(var(--chart-2)/0.34)] opacity-48"
                      : "translate-x-0 scale-[0.94] text-[hsl(var(--chart-3)/0.42)] opacity-62",
                )}
              >
                {words.length > 0 ? (
                  <DreamFMKaraokeLine
                    active={active}
                    currentTimeMs={visualCurrentTimeMs}
                    lineStartMs={line.startMs}
                    lineEndMs={line.endMs}
                    words={words}
                  />
                ) : (
                  text
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function buildDreamFMLyricsTimeline(
  lines: DreamFMLyricLineView[],
): DreamFMLyricTimelineLine[] {
  const normalized = lines
    .map((line, sourceIndex) => ({
      sourceIndex,
      startMs: Math.max(0, Math.floor(line.startMs)),
      durationMs: Math.max(0, Math.floor(line.durationMs)),
      text: line.text.trim(),
      words: normalizeDreamFMLyricWords(line),
    }))
    .sort(
      (left, right) =>
        left.startMs - right.startMs || left.sourceIndex - right.sourceIndex,
    );

  return normalized.map((line, index) => {
    const nextStartMs = normalized[index + 1]?.startMs;
    const durationEndMs =
      line.durationMs > 0 ? line.startMs + line.durationMs : 0;
    const naturalEndMs =
      durationEndMs > line.startMs
        ? durationEndMs
        : typeof nextStartMs === "number"
          ? nextStartMs
          : line.startMs + 5000;
    const endMs =
      typeof nextStartMs === "number" && nextStartMs > line.startMs
        ? Math.min(naturalEndMs, nextStartMs)
        : Math.max(line.startMs + 500, naturalEndMs);
    return {
      sourceIndex: line.sourceIndex,
      startMs: line.startMs,
      endMs,
      activeStartMs: Math.max(0, line.startMs - DREAM_FM_LYRICS_LINE_LEAD_MS),
      activeEndMs: Math.max(
        line.startMs + 120,
        endMs + DREAM_FM_LYRICS_LINE_GRACE_MS,
      ),
      text: line.text,
      words: line.words,
    };
  });
}

function findDreamFMActiveLyricLineIndex(
  lines: DreamFMLyricTimelineLine[],
  currentTimeMs: number,
) {
  let activeIndex = -1;
  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    if (currentTimeMs < line.activeStartMs) {
      break;
    }
    if (
      line.text &&
      currentTimeMs >= line.activeStartMs &&
      currentTimeMs < line.activeEndMs
    ) {
      activeIndex = index;
    }
  }
  return activeIndex;
}

function normalizeDreamFMLyricWords(line: DreamFMLyricLineView) {
  return [...(line.words ?? [])]
    .filter((word) => word.text.trim())
    .sort((left, right) => left.startMs - right.startMs);
}

function getDreamFMActiveLyricWordProgress(
  words: DreamFMLyricWordView[],
  currentTimeMs: number,
  lineStartMs: number,
  lineEndMs: number,
) {
  const visibleTimeMs = currentTimeMs + DREAM_FM_LYRICS_WORD_LEAD_MS;
  for (let index = 0; index < words.length; index += 1) {
    const startMs = Math.max(lineStartMs, words[index].startMs);
    const nextStartMs = words[index + 1]?.startMs;
    const endMs =
      typeof nextStartMs === "number" && nextStartMs > startMs
        ? nextStartMs
        : Math.max(startMs + 280, lineEndMs);
    if (visibleTimeMs < startMs) {
      if (index === 0 && currentTimeMs >= lineStartMs - 120) {
        return { index: 0, progress: 0 };
      }
      break;
    }
    if (visibleTimeMs <= endMs || index === words.length - 1) {
      return {
        index,
        progress: Math.max(
          0,
          Math.min(
            1,
            (visibleTimeMs - startMs) / Math.max(1, endMs - startMs),
          ),
        ),
      };
    }
  }
  return { index: -1, progress: 0 };
}

function DreamFMKaraokeLine(props: {
  active: boolean;
  currentTimeMs: number;
  lineStartMs: number;
  lineEndMs: number;
  words: DreamFMLyricWordView[];
}) {
  const activeWord = props.active
    ? getDreamFMActiveLyricWordProgress(
        props.words,
        props.currentTimeMs,
        props.lineStartMs,
        props.lineEndMs,
      )
    : { index: -1, progress: 0 };

  return (
    <span>
      {props.words.map((word, index) => {
        const wordActive = index === activeWord.index;
        const wordPast = activeWord.index >= 0 && index < activeWord.index;
        const text = word.text;
        const fillPercent = wordActive
          ? Math.round(activeWord.progress * 1000) / 10
          : wordPast
            ? 100
            : 0;
        const wordStyle =
          props.active && wordActive
            ? ({
                backgroundImage: [
                  "linear-gradient(90deg,",
                  "hsl(var(--chart-1)) 0%,",
                  `hsl(var(--chart-1)) ${fillPercent}%,`,
                  `hsl(var(--chart-3) / 0.48) ${fillPercent}%,`,
                  "hsl(var(--chart-3) / 0.48) 100%)",
                ].join(" "),
                WebkitBackgroundClip: "text",
                backgroundClip: "text",
                color: "transparent",
              } as React.CSSProperties)
            : undefined;
        return (
          <span
            key={`${word.startMs}-${index}-${text}`}
            className={cn(
              "inline transition-colors duration-200",
              props.active &&
                (wordActive
                  ? "text-[hsl(var(--chart-1))]"
                  : wordPast
                    ? "text-[hsl(var(--chart-2)/0.78)]"
                    : "text-[hsl(var(--chart-3)/0.48)]"),
            )}
            style={wordStyle}
          >
            {text}
            {/\s$/.test(text) ? "" : " "}
          </span>
        );
      })}
    </span>
  );
}

function DreamFMLyricsState(props: {
  text: ReturnType<typeof getXiaText>;
  children: React.ReactNode;
}) {
  return (
    <div className="relative flex h-full w-full flex-col items-center justify-center overflow-hidden px-6 text-center">
      <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[1.25rem] bg-sidebar-primary/10 text-sidebar-primary">
        {React.Children.toArray(props.children)[0]}
      </div>
      <div className="mt-4 text-sm font-semibold text-sidebar-foreground">
        {props.text.dreamFm.lyrics}
      </div>
      <div className="mt-1 max-w-full break-words text-sm text-sidebar-foreground/56">
        {React.Children.toArray(props.children).slice(1)}
      </div>
    </div>
  );
}

function DreamFMPlaybackQueuePopup(props: {
  queueTitle: string;
  queueItems: DreamFMOnlineItem[];
  selectedQueueId: string;
  httpBaseURL: string;
  text: ReturnType<typeof getXiaText>;
  onClearQueue?: () => void;
  onRemoveQueueItem?: (item: DreamFMOnlineItem) => void;
  onSelectQueueTrack: (item: DreamFMOnlineItem) => void;
  onClose: () => void;
}) {
  return (
    <div className="dream-fm-floating-surface absolute bottom-20 right-3 top-3 z-30 flex w-[min(22rem,calc(100%-1.5rem))] flex-col rounded-[1.65rem] p-2 animate-in fade-in-0 slide-in-from-right-3 zoom-in-95 duration-200 sm:right-5 sm:top-5 sm:w-[22rem]">
      <div className="flex h-16 shrink-0 items-center justify-between gap-3 px-3.5">
        <div className="min-w-0 pl-0.5">
          <div className="truncate text-sm font-semibold text-sidebar-foreground">
            {props.queueTitle}
          </div>
          <div className="text-xs font-medium tabular-nums text-sidebar-foreground/46">
            {props.queueItems.length}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {props.queueItems.length > 0 && props.onClearQueue ? (
            <DreamFMPlayerIconButton
              label={props.text.dreamFm.clearQueue}
              className="h-9 w-9 hover:bg-destructive/10 hover:text-destructive"
              onClick={props.onClearQueue}
            >
              <Trash2 className="h-4 w-4" />
            </DreamFMPlayerIconButton>
          ) : null}
          <DreamFMPlayerIconButton
            label={props.text.actions.close}
            className="h-9 w-9"
            onClick={props.onClose}
          >
            <X className="h-4 w-4" />
          </DreamFMPlayerIconButton>
        </div>
      </div>
      {props.queueItems.length === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 pb-3 text-center text-sm text-sidebar-foreground/58">
          {props.text.dreamFm.upNextEmpty}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto px-0 pb-0 pt-1">
          <div className="space-y-1.5">
            {props.queueItems.map((item) => {
              const selected = item.id === props.selectedQueueId;
              return (
                <div
                  key={item.id}
                  className={cn(
                    "group flex min-h-14 items-center gap-2 rounded-2xl border border-transparent px-2 py-2 transition-[transform,background-color,border-color] duration-200 ease-out active:scale-[0.99]",
                    selected
                      ? "border-sidebar-primary/18 bg-sidebar-primary/10"
                      : "hover:-translate-y-0.5 hover:bg-sidebar-background/54",
                  )}
                >
                  <button
                    type="button"
                    className="flex min-w-0 flex-1 items-center gap-2 text-left focus-visible:outline-none"
                    onClick={() => props.onSelectQueueTrack(item)}
                  >
                    <DreamFMQueueArtwork
                      httpBaseURL={props.httpBaseURL}
                      item={item}
                      selected={selected}
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-sidebar-foreground">
                        {item.title}
                      </span>
                      <span className="block truncate text-xs text-sidebar-foreground/58">
                        {[item.channel, item.durationLabel]
                          .filter(Boolean)
                          .join(" · ")}
                      </span>
                    </span>
                  </button>
                  {props.onRemoveQueueItem ? (
                    <button
                      type="button"
                      className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sidebar-foreground/55 opacity-70 transition hover:bg-destructive/10 hover:text-destructive group-hover:opacity-100 focus-visible:outline-none"
                      aria-label={props.text.dreamFm.removeFromQueue}
                      title={props.text.dreamFm.removeFromQueue}
                      onClick={() => props.onRemoveQueueItem?.(item)}
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  ) : null}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function DreamFMLocalPlaybackQueuePopup(props: {
  queueTitle: string;
  queueItems: DreamFMLocalItem[];
  selectedQueueId: string;
  text: ReturnType<typeof getXiaText>;
  onSelectQueueTrack: (item: DreamFMLocalItem) => void;
  onClose: () => void;
}) {
  return (
    <div className="dream-fm-floating-surface absolute bottom-20 right-3 top-3 z-30 flex w-[min(22rem,calc(100%-1.5rem))] flex-col rounded-[1.65rem] p-2 animate-in fade-in-0 slide-in-from-right-3 zoom-in-95 duration-200 sm:right-5 sm:top-5 sm:w-[22rem]">
      <div className="flex h-16 shrink-0 items-center justify-between gap-3 px-3.5">
        <div className="min-w-0 pl-0.5">
          <div className="truncate text-sm font-semibold text-sidebar-foreground">
            {props.queueTitle}
          </div>
          <div className="text-xs font-medium tabular-nums text-sidebar-foreground/46">
            {props.queueItems.length}
          </div>
        </div>
        <DreamFMPlayerIconButton
          label={props.text.actions.close}
          className="h-9 w-9"
          onClick={props.onClose}
        >
          <X className="h-4 w-4" />
        </DreamFMPlayerIconButton>
      </div>
      {props.queueItems.length === 0 ? (
        <div className="flex min-h-0 flex-1 items-center justify-center px-6 pb-3 text-center text-sm text-sidebar-foreground/58">
          {props.text.dreamFm.upNextEmpty}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto px-0 pb-0 pt-1">
          <div className="space-y-1.5">
            {props.queueItems.map((item) => {
              const selected = item.id === props.selectedQueueId;
              return (
                <button
                  key={item.id}
                  type="button"
                  className={cn(
                    "flex min-h-14 w-full items-center gap-2 rounded-2xl border border-transparent px-2 py-2 text-left transition-[transform,background-color,border-color] duration-200 ease-out active:scale-[0.99] focus-visible:outline-none",
                    selected
                      ? "border-sidebar-primary/18 bg-sidebar-primary/10"
                      : "hover:-translate-y-0.5 hover:bg-sidebar-background/54",
                  )}
                  onClick={() => props.onSelectQueueTrack(item)}
                >
                  <DreamFMLocalQueueArtwork item={item} selected={selected} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-sidebar-foreground">
                      {item.title}
                    </span>
                    <span className="block truncate text-xs text-sidebar-foreground/58">
                      {[item.author, item.durationLabel]
                        .filter(Boolean)
                        .join(" · ")}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function DreamFMQueueArtwork(props: {
  httpBaseURL: string;
  item: DreamFMOnlineItem;
  selected: boolean;
}) {
  const posterCandidates = React.useMemo(
    () => buildDreamFMPosterCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );
  const posterCandidateKey = posterCandidates.join("\n");
  const [posterIndex, setPosterIndex] = React.useState(0);
  const activePoster =
    posterCandidates[
      Math.min(posterIndex, Math.max(posterCandidates.length - 1, 0))
    ] || DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setPosterIndex(0);
  }, [posterCandidateKey]);

  return (
    <span
      className={cn(
        "relative flex h-10 w-10 shrink-0 overflow-hidden rounded-xl bg-muted ring-1 ring-border/70",
        props.selected && "ring-primary/30",
      )}
    >
      <img
        key={activePoster}
        src={activePoster}
        alt=""
        className="h-full w-full object-cover"
        loading="eager"
        onError={() => {
          setPosterIndex((current) => {
            if (current >= posterCandidates.length - 1) {
              return current;
            }
            return current + 1;
          });
        }}
      />
    </span>
  );
}

function DreamFMLocalQueueArtwork(props: {
  item: DreamFMLocalItem;
  selected: boolean;
}) {
  const [coverFailed, setCoverFailed] = React.useState(false);
  const coverURL =
    !coverFailed && props.item.coverURL
      ? props.item.coverURL
      : DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setCoverFailed(false);
  }, [props.item.coverURL]);

  return (
    <span
      className={cn(
        "relative flex h-10 w-10 shrink-0 overflow-hidden rounded-xl bg-muted ring-1 ring-border/70",
        props.selected && "ring-primary/30",
      )}
    >
      <img
        src={coverURL}
        alt=""
        className="h-full w-full object-cover"
        loading="eager"
        onError={() => setCoverFailed(true)}
      />
    </span>
  );
}
