import * as React from "react";
import { Events, Window } from "@wailsio/runtime";
import {
  MediaPlayer,
  MediaProvider,
  MediaRemoteControl,
  type VideoSrc,
} from "@vidstack/react";
import {
  Loader2,
  Maximize,
  Maximize2,
  Minimize,
  Minimize2,
  Pause,
  Play,
  Volume2,
  VolumeX,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import { getXiaText } from "@/features/xiadown/shared";
import {
  COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS,
  COMPLETED_PREVIEW_CONTROL_RANGE_CLASS,
  COMPLETED_PREVIEW_SHELL_CLASS,
  COMPLETED_PREVIEW_VOLUME_RANGE_CLASS,
} from "@/shared/styles/xiadown";

type CompletedVidstackPreviewProps = {
  text: ReturnType<typeof getXiaText>;
  mediaUrl: string;
  title: string;
  persistKey?: string;
  posterUrl?: string;
  durationMs?: number;
};

type MediaPlayerElement = React.ElementRef<typeof MediaPlayer>;
type PreviewFullscreenMode = "dom" | "wails";
type CompletedPreviewProgressState = {
  positionMs: number;
  durationMs: number;
  volume: number;
  muted: boolean;
  updatedAt: number;
};
type MediaProviderHandle = {
  audio?: unknown;
  media?: unknown;
  video?: unknown;
};

const COMPLETED_PREVIEW_PROGRESS_STORAGE_PREFIX =
  "xiadown:completed-preview-progress:v1:";
const COMPLETED_PREVIEW_PROGRESS_SAVE_INTERVAL_MS = 2000;
const COMPLETED_PREVIEW_PROGRESS_MAX_AGE_MS = 1000 * 60 * 60 * 24 * 90;
const COMPLETED_PREVIEW_RESUME_MIN_POSITION_MS = 1000;
const COMPLETED_PREVIEW_RESUME_END_GAP_MS = 5000;

function clampMs(value: number, durationMs: number) {
  if (!Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.min(value, Math.max(0, durationMs)));
}

function clampVolume(value: number) {
  if (!Number.isFinite(value)) {
    return 1;
  }
  return Math.min(1, Math.max(0, value));
}

function formatMediaTime(valueMs: number) {
  const totalSeconds = Math.max(0, Math.floor(valueMs / 1000));
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

function resolveMediaFileExtension(source: string, fallbackName?: string) {
  const candidates = [source, fallbackName ?? ""];
  for (const candidate of candidates) {
    const trimmed = candidate.trim();
    if (!trimmed) {
      continue;
    }
    const withoutHash = trimmed.split("#")[0] ?? trimmed;
    const withoutQuery = withoutHash.split("?")[0] ?? withoutHash;
    const leaf = withoutQuery.split("/").pop() ?? withoutQuery;
    let decodedLeaf = leaf;
    try {
      decodedLeaf = decodeURIComponent(leaf);
    } catch {
      decodedLeaf = leaf;
    }
    const match = decodedLeaf.toLowerCase().match(/\.([a-z0-9]+)$/i);
    if (match?.[1]) {
      return match[1];
    }
  }
  return "";
}

function resolveVideoSource(
  mediaUrl: string,
  title?: string,
): string | VideoSrc {
  switch (resolveMediaFileExtension(mediaUrl, title)) {
    case "webm":
      return { src: mediaUrl, type: "video/webm" };
    case "mp4":
    case "m4v":
    case "mov":
      return { src: mediaUrl, type: "video/mp4" };
    case "ogg":
    case "ogv":
      return { src: mediaUrl, type: "video/ogg" };
    case "avi":
      return { src: mediaUrl, type: "video/avi" };
    case "mpeg":
    case "mpg":
      return { src: mediaUrl, type: "video/mpeg" };
    case "3gp":
      return { src: mediaUrl, type: "video/3gp" };
    default:
      return mediaUrl;
  }
}

function isHTMLMediaElement(value: unknown): value is HTMLMediaElement {
  return (
    typeof HTMLMediaElement !== "undefined" &&
    value instanceof HTMLMediaElement
  );
}

function isHTMLElement(value: unknown): value is HTMLElement {
  return typeof HTMLElement !== "undefined" && value instanceof HTMLElement;
}

function resolvePlayerHost(player: MediaPlayerElement | null) {
  if (!player) {
    return null;
  }
  if (isHTMLElement(player.el)) {
    return player.el;
  }
  return isHTMLElement(player) ? player : null;
}

function resolvePlayerMediaElement(player: MediaPlayerElement | null) {
  const host = resolvePlayerHost(player);
  const provider = player?.provider as MediaProviderHandle | null | undefined;
  if (isHTMLMediaElement(provider?.media) && provider.media.isConnected) {
    return provider.media;
  }
  if (isHTMLMediaElement(provider?.video) && provider.video.isConnected) {
    return provider.video;
  }
  if (isHTMLMediaElement(provider?.audio) && provider.audio.isConnected) {
    return provider.audio;
  }

  const element = host?.querySelector("video,audio") ?? null;
  return isHTMLMediaElement(element) && element.isConnected ? element : null;
}

function resolvePlaybackErrorMessage(error: unknown, fallback: string) {
  const message = error instanceof Error ? error.message.trim() : "";
  return message || fallback;
}

function invokeMediaRequest(action: () => Promise<void> | void) {
  try {
    return Promise.resolve(action());
  } catch (error) {
    return Promise.reject(error);
  }
}

function ensureMediaLoadStarted(media: HTMLMediaElement) {
  media.preload = "auto";
  const networkEmpty =
    typeof HTMLMediaElement !== "undefined"
      ? HTMLMediaElement.NETWORK_EMPTY
      : 0;
  if (media.networkState !== networkEmpty) {
    return;
  }
  try {
    media.load();
  } catch {
    // Some embedded providers can reject load() while swapping sources.
  }
}

function resolveProgressStorageKey(persistKey?: string, mediaUrl?: string) {
  const source = (persistKey ?? "").trim() || (mediaUrl ?? "").trim();
  return source ? `${COMPLETED_PREVIEW_PROGRESS_STORAGE_PREFIX}${source}` : "";
}

function readStoredPreviewProgress(
  key: string,
): CompletedPreviewProgressState | null {
  if (!key || typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<CompletedPreviewProgressState>;
    const updatedAt = Number(parsed.updatedAt ?? 0);
    if (
      !Number.isFinite(updatedAt) ||
      Date.now() - updatedAt > COMPLETED_PREVIEW_PROGRESS_MAX_AGE_MS
    ) {
      window.localStorage.removeItem(key);
      return null;
    }
    return {
      positionMs: Math.max(0, Number(parsed.positionMs ?? 0)),
      durationMs: Math.max(0, Number(parsed.durationMs ?? 0)),
      volume: clampVolume(Number(parsed.volume ?? 1)),
      muted: Boolean(parsed.muted),
      updatedAt,
    };
  } catch {
    return null;
  }
}

function writeStoredPreviewProgress(
  key: string,
  state: CompletedPreviewProgressState,
) {
  if (!key || typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(key, JSON.stringify(state));
  } catch {
    // Storage can be unavailable in restricted WebViews.
  }
}

export function CompletedVidstackPreview(props: CompletedVidstackPreviewProps) {
  const shellRef = React.useRef<HTMLDivElement | null>(null);
  const fullscreenModeRef = React.useRef<PreviewFullscreenMode | null>(null);
  const previousWindowedFullscreenRef = React.useRef(false);
  const lastNonZeroVolumeRef = React.useRef(1);
  const animationFrameRef = React.useRef<number>();
  const playbackWatchdogRef = React.useRef<number>();
  const pendingPlayRef = React.useRef(false);
  const playRequestSerialRef = React.useRef(0);
  const playRequestInFlightRef = React.useRef(false);
  const restoredProgressKeyRef = React.useRef("");
  const lastProgressSaveAtRef = React.useRef(0);
  const mediaRemote = React.useMemo(() => new MediaRemoteControl(), []);
  const [playerElement, setPlayerElement] =
    React.useState<MediaPlayerElement | null>(null);
  const [mediaElement, setMediaElement] =
    React.useState<HTMLMediaElement | null>(null);
  const [currentTimeMs, setCurrentTimeMs] = React.useState(0);
  const [resolvedDurationMs, setResolvedDurationMs] = React.useState(() =>
    Math.max(0, props.durationMs ?? 0),
  );
  const [isPlaying, setIsPlaying] = React.useState(false);
  const [playPending, setPlayPending] = React.useState(false);
  const [volume, setVolume] = React.useState(1);
  const [muted, setMuted] = React.useState(false);
  const [playbackError, setPlaybackError] = React.useState("");
  const [windowedFullscreen, setWindowedFullscreen] = React.useState(false);
  const [screenFullscreen, setScreenFullscreen] = React.useState(false);

  const effectiveDurationMs = Math.max(resolvedDurationMs, 0);
  const playerSource = React.useMemo(
    () => resolveVideoSource(props.mediaUrl, props.title),
    [props.mediaUrl, props.title],
  );
  const progressStorageKey = React.useMemo(
    () => resolveProgressStorageKey(props.persistKey, props.mediaUrl),
    [props.mediaUrl, props.persistKey],
  );

  const getActiveMediaElement = React.useCallback(() => {
    const current = resolvePlayerMediaElement(playerElement);
    if (current) {
      return current;
    }
    return mediaElement?.isConnected ? mediaElement : null;
  }, [mediaElement, playerElement]);

  const finishPendingPlay = React.useCallback(() => {
    if (playbackWatchdogRef.current) {
      window.clearTimeout(playbackWatchdogRef.current);
      playbackWatchdogRef.current = undefined;
    }
    pendingPlayRef.current = false;
    playRequestInFlightRef.current = false;
    setPlayPending(false);
  }, []);

  const cancelPendingPlay = React.useCallback(() => {
    if (playbackWatchdogRef.current) {
      window.clearTimeout(playbackWatchdogRef.current);
      playbackWatchdogRef.current = undefined;
    }
    pendingPlayRef.current = false;
    playRequestInFlightRef.current = false;
    playRequestSerialRef.current += 1;
    setPlayPending(false);
  }, []);

  const startPlaybackWatchdog = React.useCallback(() => {
    if (playbackWatchdogRef.current) {
      window.clearTimeout(playbackWatchdogRef.current);
    }
    const startingTime = getActiveMediaElement()?.currentTime ?? 0;
    playbackWatchdogRef.current = window.setTimeout(() => {
      const activeMedia = getActiveMediaElement();
      if (!activeMedia || !pendingPlayRef.current) {
        return;
      }
      const advanced = activeMedia.currentTime > startingTime + 0.05;
      const isActuallyPlaying = !activeMedia.paused && advanced;
      if (isActuallyPlaying) {
        finishPendingPlay();
        setIsPlaying(true);
        return;
      }
      finishPendingPlay();
      setIsPlaying(false);
      setPlaybackError(props.text.completed.previewPlaybackStalled);
    }, 8000);
  }, [
    finishPendingPlay,
    getActiveMediaElement,
    props.text.completed.previewPlaybackStalled,
  ]);

  const requestPlayWhenMediaIsAvailable = React.useCallback(
    (media: HTMLMediaElement | null) => {
      pendingPlayRef.current = true;
      playRequestInFlightRef.current = false;
      playRequestSerialRef.current += 1;
      setPlayPending(true);
      setPlaybackError("");
      if (media) {
        ensureMediaLoadStarted(media);
      }
      startPlaybackWatchdog();
    },
    [startPlaybackWatchdog],
  );

  const playNativeMedia = React.useCallback(
    async (media: HTMLMediaElement) => {
      const requestSerial = playRequestSerialRef.current + 1;
      playRequestSerialRef.current = requestSerial;
      pendingPlayRef.current = true;
      playRequestInFlightRef.current = true;
      setPlayPending(true);
      setPlaybackError("");
      ensureMediaLoadStarted(media);
      startPlaybackWatchdog();

      try {
        await invokeMediaRequest(() => media.play());
        if (playRequestSerialRef.current !== requestSerial) {
          return;
        }
        finishPendingPlay();
        setIsPlaying(true);
      } catch (error) {
        if (playRequestSerialRef.current !== requestSerial) {
          return;
        }
        finishPendingPlay();
        setIsPlaying(false);
        setPlaybackError(
          resolvePlaybackErrorMessage(error, props.text.completed.noPreview),
        );
      }
    },
    [
      finishPendingPlay,
      props.text.completed.noPreview,
      startPlaybackWatchdog,
    ],
  );

  const persistPlaybackState = React.useCallback(
    (media: HTMLMediaElement, force = false) => {
      if (!progressStorageKey) {
        return;
      }
      const now = Date.now();
      if (
        !force &&
        now - lastProgressSaveAtRef.current <
          COMPLETED_PREVIEW_PROGRESS_SAVE_INTERVAL_MS
      ) {
        return;
      }
      lastProgressSaveAtRef.current = now;
      const durationMs = Math.max(
        effectiveDurationMs,
        Number.isFinite(media.duration) && media.duration > 0
          ? Math.round(media.duration * 1000)
          : 0,
      );
      const rawPositionMs = Math.max(0, Math.round(media.currentTime * 1000));
      const nearEnd =
        durationMs > 0 &&
        rawPositionMs >=
          Math.max(0, durationMs - COMPLETED_PREVIEW_RESUME_END_GAP_MS);
      writeStoredPreviewProgress(progressStorageKey, {
        positionMs: nearEnd ? 0 : rawPositionMs,
        durationMs,
        volume: clampVolume(Number(media.volume ?? volume)),
        muted: Boolean(media.muted ?? muted),
        updatedAt: now,
      });
    },
    [effectiveDurationMs, muted, progressStorageKey, volume],
  );

  const restorePersistedPlaybackState = React.useCallback(
    (media: HTMLMediaElement) => {
      if (!progressStorageKey) {
        return;
      }
      if (restoredProgressKeyRef.current === progressStorageKey) {
        return;
      }
      const stored = readStoredPreviewProgress(progressStorageKey);
      restoredProgressKeyRef.current = progressStorageKey;
      if (!stored) {
        return;
      }

      media.volume = stored.volume;
      media.muted = stored.muted;
      setVolume(stored.volume);
      setMuted(stored.muted);
      if (stored.volume > 0) {
        lastNonZeroVolumeRef.current = stored.volume;
      }

      const durationMs = Math.max(
        stored.durationMs,
        props.durationMs ?? 0,
        Number.isFinite(media.duration) && media.duration > 0
          ? Math.round(media.duration * 1000)
          : 0,
      );
      const shouldResume =
        stored.positionMs >= COMPLETED_PREVIEW_RESUME_MIN_POSITION_MS &&
        (durationMs <= 0 ||
          stored.positionMs <
            durationMs - COMPLETED_PREVIEW_RESUME_END_GAP_MS);
      if (!shouldResume) {
        return;
      }

      const nextTime = durationMs
        ? clampMs(stored.positionMs, durationMs) / 1000
        : stored.positionMs / 1000;
      try {
        media.currentTime = nextTime;
        setCurrentTimeMs(Math.round(nextTime * 1000));
      } catch {
        // Some media elements reject seeking until more data is buffered.
      }
    },
    [progressStorageKey, props.durationMs],
  );

  const syncCurrentTime = React.useCallback(() => {
    const activeMedia = getActiveMediaElement() ?? playerElement;
    const next = activeMedia ? Number(activeMedia.currentTime || 0) * 1000 : 0;
    setCurrentTimeMs((current) => {
      const normalized = clampMs(next, effectiveDurationMs || next || 0);
      return Math.abs(current - normalized) < 50 ? current : normalized;
    });
    if (isHTMLMediaElement(activeMedia)) {
      persistPlaybackState(activeMedia);
    }
  }, [
    effectiveDurationMs,
    getActiveMediaElement,
    persistPlaybackState,
    playerElement,
  ]);

  const handlePlayerRef = React.useCallback(
    (node: MediaPlayerElement | null) => {
      setPlayerElement(node);
    },
    [],
  );

  React.useEffect(() => {
    setCurrentTimeMs(0);
    setResolvedDurationMs(Math.max(0, props.durationMs ?? 0));
    setIsPlaying(false);
    cancelPendingPlay();
    restoredProgressKeyRef.current = "";
    lastProgressSaveAtRef.current = 0;
    setMediaElement(null);
    setPlaybackError("");
  }, [cancelPendingPlay, progressStorageKey, props.durationMs, props.mediaUrl]);

  React.useEffect(() => {
    mediaRemote.setPlayer(playerElement);
    return () => mediaRemote.setPlayer(null);
  }, [mediaRemote, playerElement]);

  React.useEffect(() => {
    const player = playerElement;
    if (!player) {
      return;
    }
    const nextVolume = clampVolume(Number(player.volume ?? 1));
    setVolume(nextVolume);
    if (nextVolume > 0) {
      lastNonZeroVolumeRef.current = nextVolume;
    }
    setMuted(Boolean(player.muted));
  }, [playerElement]);

  React.useEffect(() => {
    const player = playerElement;
    if (!player) {
      return;
    }
    const playerHost = resolvePlayerHost(player);
    if (!playerHost) {
      return;
    }

    const syncMediaNode = () => {
      const nextMedia = resolvePlayerMediaElement(player);
      setMediaElement((current) =>
        current === nextMedia ? current : nextMedia,
      );
    };

    syncMediaNode();

    const frame = requestAnimationFrame(syncMediaNode);
    const observer = new MutationObserver(syncMediaNode);
    observer.observe(playerHost, { childList: true, subtree: true });
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [playerElement, props.mediaUrl]);

  const handleLoadedMetadata = React.useCallback(
    (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const media = event.currentTarget;
      setMediaElement(media);
      setPlaybackError("");
      const nextDurationMs = Number.isFinite(media.duration)
        ? Math.round(media.duration * 1000)
        : 0;
      if (nextDurationMs > 0) {
        setResolvedDurationMs((current) => Math.max(current, nextDurationMs));
      }
      restorePersistedPlaybackState(media);
    },
    [restorePersistedPlaybackState],
  );

  const handleMediaCanPlay = React.useCallback(
    (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const media = event.currentTarget;
      setMediaElement(media);
      setPlaybackError("");
      if (pendingPlayRef.current && !playRequestInFlightRef.current) {
        void playNativeMedia(media);
      }
    },
    [playNativeMedia],
  );

  const handleMediaError = React.useCallback(
    (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const message = event.currentTarget.error?.message.trim() ?? "";
      cancelPendingPlay();
      setPlaybackError(message || props.text.completed.noPreview);
      setIsPlaying(false);
    },
    [cancelPendingPlay, props.text.completed.noPreview],
  );

  React.useEffect(() => {
    if (!mediaElement || mediaElement.readyState < 1) {
      return;
    }
    restorePersistedPlaybackState(mediaElement);
  }, [mediaElement, restorePersistedPlaybackState]);

  React.useEffect(() => {
    const media = mediaElement ?? playerElement;
    if (!media) {
      return;
    }

    const handlePlay = () => {
      setPlaybackError("");
      setIsPlaying(true);
    };
    const handlePlaying = () => {
      finishPendingPlay();
      setPlaybackError("");
      setIsPlaying(true);
      syncCurrentTime();
    };
    const handlePause = () => {
      setIsPlaying(false);
      if (isHTMLMediaElement(media)) {
        persistPlaybackState(media, true);
      }
    };
    const handleEnded = () => {
      cancelPendingPlay();
      setIsPlaying(false);
      syncCurrentTime();
      if (isHTMLMediaElement(media)) {
        persistPlaybackState(media, true);
      }
    };
    const handleVolumeChange = () => {
      const nextVolume = clampVolume(Number(media.volume ?? 1));
      setVolume(nextVolume);
      setMuted(Boolean(media.muted));
      if (nextVolume > 0) {
        lastNonZeroVolumeRef.current = nextVolume;
      }
      if (isHTMLMediaElement(media)) {
        persistPlaybackState(media, true);
      }
    };
    const handleDurationChange = () => {
      const durationSeconds = Number(media.duration ?? 0);
      if (Number.isFinite(durationSeconds) && durationSeconds > 0) {
        setResolvedDurationMs((current) =>
          Math.max(current, Math.round(durationSeconds * 1000)),
        );
      }
    };
    const handleCanPlay = () => {
      setPlaybackError("");
      const nativeMedia = isHTMLMediaElement(media) ? media : mediaElement;
      if (
        nativeMedia &&
        pendingPlayRef.current &&
        !playRequestInFlightRef.current
      ) {
        void playNativeMedia(nativeMedia);
      }
    };
    const handleError = () => {
      const nativeMedia = isHTMLMediaElement(media) ? media : mediaElement;
      const message = nativeMedia?.error?.message.trim() ?? "";
      cancelPendingPlay();
      setPlaybackError(message || props.text.completed.noPreview);
      setIsPlaying(false);
    };
    const handleTimeUpdate = () => {
      if (pendingPlayRef.current) {
        finishPendingPlay();
      }
      syncCurrentTime();
    };

    media.addEventListener("play", handlePlay);
    media.addEventListener("playing", handlePlaying);
    media.addEventListener("pause", handlePause);
    media.addEventListener("ended", handleEnded);
    media.addEventListener("volumechange", handleVolumeChange);
    media.addEventListener("durationchange", handleDurationChange);
    media.addEventListener("canplay", handleCanPlay);
    media.addEventListener("loadeddata", handleCanPlay);
    media.addEventListener("error", handleError);
    media.addEventListener("timeupdate", handleTimeUpdate);

    return () => {
      media.removeEventListener("play", handlePlay);
      media.removeEventListener("playing", handlePlaying);
      media.removeEventListener("pause", handlePause);
      media.removeEventListener("ended", handleEnded);
      media.removeEventListener("volumechange", handleVolumeChange);
      media.removeEventListener("durationchange", handleDurationChange);
      media.removeEventListener("canplay", handleCanPlay);
      media.removeEventListener("loadeddata", handleCanPlay);
      media.removeEventListener("error", handleError);
      media.removeEventListener("timeupdate", handleTimeUpdate);
    };
  }, [
    cancelPendingPlay,
    finishPendingPlay,
    mediaElement,
    persistPlaybackState,
    playNativeMedia,
    playerElement,
    props.text.completed.noPreview,
    syncCurrentTime,
  ]);

  React.useEffect(() => {
    if (
      !playPending ||
      !pendingPlayRef.current ||
      playRequestInFlightRef.current
    ) {
      return;
    }
    const activeMedia = getActiveMediaElement();
    if (!activeMedia) {
      return;
    }
    void playNativeMedia(activeMedia);
  }, [getActiveMediaElement, playNativeMedia, playPending]);

  React.useEffect(() => {
    if (!props.mediaUrl || !isPlaying) {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
        animationFrameRef.current = undefined;
      }
      return;
    }

    const tick = () => {
      syncCurrentTime();
      animationFrameRef.current = requestAnimationFrame(tick);
    };

    animationFrameRef.current = requestAnimationFrame(tick);
    return () => {
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
        animationFrameRef.current = undefined;
      }
    };
  }, [isPlaying, props.mediaUrl, syncCurrentTime]);

  React.useEffect(() => {
    const handleDomFullscreenChange = () => {
      if (fullscreenModeRef.current !== "dom") {
        return;
      }
      const isActive = document.fullscreenElement === shellRef.current;
      setScreenFullscreen(isActive);
      if (!isActive) {
        fullscreenModeRef.current = null;
        setWindowedFullscreen(previousWindowedFullscreenRef.current);
        previousWindowedFullscreenRef.current = false;
      }
    };

    document.addEventListener("fullscreenchange", handleDomFullscreenChange);
    return () =>
      document.removeEventListener(
        "fullscreenchange",
        handleDomFullscreenChange,
      );
  }, []);

  React.useEffect(() => {
    const offWindowFullscreen = Events.On(
      Events.Types.Common.WindowFullscreen,
      () => {
        if (fullscreenModeRef.current === "wails") {
          setScreenFullscreen(true);
        }
      },
    );
    const offWindowUnFullscreen = Events.On(
      Events.Types.Common.WindowUnFullscreen,
      () => {
        if (fullscreenModeRef.current === "wails") {
          fullscreenModeRef.current = null;
          setScreenFullscreen(false);
          setWindowedFullscreen(previousWindowedFullscreenRef.current);
          previousWindowedFullscreenRef.current = false;
        }
      },
    );
    return () => {
      offWindowFullscreen();
      offWindowUnFullscreen();
    };
  }, []);

  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && windowedFullscreen && !screenFullscreen) {
        setWindowedFullscreen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [screenFullscreen, windowedFullscreen]);

  const togglePlay = (trigger?: Event) => {
    const activeMedia = getActiveMediaElement();
    const player = playerElement;
    if (!activeMedia && !player) {
      return;
    }
    if (isPlaying || playPending) {
      cancelPendingPlay();
      activeMedia?.pause();
      if (player) {
        player.paused = true;
        mediaRemote.pause(trigger);
      }
      return;
    }

    requestPlayWhenMediaIsAvailable(activeMedia);
    if (player) {
      player.paused = false;
      mediaRemote.play(trigger);
    }
    if (!activeMedia) {
      return;
    }

    void playNativeMedia(activeMedia);
  };

  const handleSeek = (value: number) => {
    const activeMedia = getActiveMediaElement() ?? playerElement;
    if (!activeMedia) {
      return;
    }
    const next = clampMs(value, effectiveDurationMs);
    activeMedia.currentTime = next / 1000;
    setCurrentTimeMs(next);
    if (isHTMLMediaElement(activeMedia)) {
      persistPlaybackState(activeMedia, true);
    }
  };

  const toggleMute = () => {
    const activeMedia = getActiveMediaElement() ?? playerElement;
    if (!activeMedia) {
      return;
    }
    if (muted || volume <= 0) {
      const restoredVolume = volume > 0 ? volume : lastNonZeroVolumeRef.current;
      activeMedia.volume = restoredVolume;
      activeMedia.muted = false;
      setVolume(restoredVolume);
      setMuted(false);
      if (isHTMLMediaElement(activeMedia)) {
        persistPlaybackState(activeMedia, true);
      }
      return;
    }
    lastNonZeroVolumeRef.current = volume;
    activeMedia.muted = true;
    setMuted(true);
    if (isHTMLMediaElement(activeMedia)) {
      persistPlaybackState(activeMedia, true);
    }
  };

  const handleVolumeChange = (value: number) => {
    const activeMedia = getActiveMediaElement() ?? playerElement;
    if (!activeMedia) {
      return;
    }
    const nextVolume = clampVolume(value);
    activeMedia.volume = nextVolume;
    activeMedia.muted = nextVolume <= 0;
    setVolume(nextVolume);
    setMuted(nextVolume <= 0);
    if (nextVolume > 0) {
      lastNonZeroVolumeRef.current = nextVolume;
    }
    if (isHTMLMediaElement(activeMedia)) {
      persistPlaybackState(activeMedia, true);
    }
  };

  const exitScreenFullscreen = React.useCallback(async () => {
    const mode = fullscreenModeRef.current;
    if (mode === "dom" && document.fullscreenElement) {
      await document.exitFullscreen();
      return;
    }
    if (mode === "wails") {
      await Window.UnFullscreen();
      if (fullscreenModeRef.current === "wails") {
        fullscreenModeRef.current = null;
        setScreenFullscreen(false);
        setWindowedFullscreen(previousWindowedFullscreenRef.current);
        previousWindowedFullscreenRef.current = false;
      }
      return;
    }
    setScreenFullscreen(false);
    setWindowedFullscreen(previousWindowedFullscreenRef.current);
    previousWindowedFullscreenRef.current = false;
  }, []);

  const toggleWindowedFullscreen = () => {
    setWindowedFullscreen((value) => !value);
  };

  const toggleScreenFullscreen = () => {
    if (screenFullscreen) {
      void exitScreenFullscreen().catch(() => {
        setScreenFullscreen(false);
        setWindowedFullscreen(previousWindowedFullscreenRef.current);
        previousWindowedFullscreenRef.current = false;
      });
      return;
    }

    previousWindowedFullscreenRef.current = windowedFullscreen;
    setWindowedFullscreen(true);

    const shell = shellRef.current;
    if (shell?.requestFullscreen) {
      void shell
        .requestFullscreen()
        .then(() => {
          fullscreenModeRef.current = "dom";
          setScreenFullscreen(true);
        })
        .catch(() => {
          fullscreenModeRef.current = "wails";
          void Window.Fullscreen()
            .then(() => setScreenFullscreen(true))
            .catch(() => {
              fullscreenModeRef.current = null;
              setScreenFullscreen(false);
              setWindowedFullscreen(previousWindowedFullscreenRef.current);
              previousWindowedFullscreenRef.current = false;
            });
        });
      return;
    }

    fullscreenModeRef.current = "wails";
    void Window.Fullscreen()
      .then(() => setScreenFullscreen(true))
      .catch(() => {
        fullscreenModeRef.current = null;
        setScreenFullscreen(false);
        setWindowedFullscreen(previousWindowedFullscreenRef.current);
        previousWindowedFullscreenRef.current = false;
      });
  };

  const visibleVolume = muted ? 0 : volume;
  const playLabel = isPlaying
    ? props.text.completed.previewPause
    : props.text.completed.previewPlay;
  const muteLabel =
    muted || volume <= 0
      ? props.text.completed.previewUnmute
      : props.text.completed.previewMute;
  const windowedFullscreenLabel = windowedFullscreen
    ? props.text.completed.previewWindowRestore
    : props.text.completed.previewWindowFullscreen;
  const screenFullscreenLabel = screenFullscreen
    ? props.text.completed.previewExitFullscreen
    : props.text.completed.previewEnterFullscreen;

  return (
    <div
      ref={shellRef}
      className={cn(
        COMPLETED_PREVIEW_SHELL_CLASS,
        windowedFullscreen &&
          "fixed inset-0 z-[200] rounded-none border-0 shadow-none",
        screenFullscreen && "rounded-none border-0 shadow-none",
      )}
    >
      <div
        className={cn(
          "relative min-h-0 flex-1 bg-black p-3",
          (windowedFullscreen || screenFullscreen) && "p-0",
        )}
      >
        {props.mediaUrl ? (
          <div className="relative h-full w-full overflow-hidden bg-black">
            <MediaPlayer
              key={props.mediaUrl}
              ref={handlePlayerRef}
              src={playerSource}
              title={props.title}
              viewType="video"
              streamType="on-demand"
              load="eager"
              controls={false}
              playsInline
              preload="auto"
              style={{ aspectRatio: "auto" }}
              className="h-full w-full bg-black"
            >
              <MediaProvider
                className="h-full w-full overflow-hidden bg-black"
                mediaProps={{
                  className: "h-full w-full object-contain object-center",
                  onLoadedMetadata: handleLoadedMetadata,
                  onCanPlay: handleMediaCanPlay,
                  onError: handleMediaError,
                }}
              />
            </MediaPlayer>
            {playbackError ? (
              <div className="pointer-events-none absolute inset-x-3 top-3 z-10 rounded-lg bg-black/62 px-3 py-2 text-xs leading-5 text-white/82 shadow-[0_12px_30px_-24px_rgba(0,0,0,0.75)] backdrop-blur-md">
                {playbackError}
              </div>
            ) : null}
          </div>
        ) : (
          <div className="flex h-full items-center justify-center bg-black px-8 text-center">
            <div className="text-sm text-white/65">
              {props.text.completed.noPreview}
            </div>
          </div>
        )}
      </div>

      <div className="shrink-0 border-t border-white/5 bg-[#0f0f0f] px-3 py-1.5">
        <div className="flex min-w-0 items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="compactIcon"
            className={COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS}
            onClick={(event) => togglePlay(event.nativeEvent)}
            aria-label={playLabel}
            title={playLabel}
            disabled={!mediaElement && !playerElement}
          >
            {playPending ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : isPlaying ? (
              <Pause className="h-3 w-3" />
            ) : (
              <Play className="h-3 w-3" />
            )}
          </Button>
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            <span className="w-[3.25rem] shrink-0 text-right font-mono text-[11px] tabular-nums text-white/75">
              {formatMediaTime(currentTimeMs)}
            </span>
            <input
              type="range"
              min={0}
              max={effectiveDurationMs || 1}
              value={Math.min(currentTimeMs, effectiveDurationMs || 1)}
              onChange={(event) => handleSeek(Number(event.target.value))}
              aria-label={props.text.completed.previewSeek}
              className={`${COMPLETED_PREVIEW_CONTROL_RANGE_CLASS} min-w-0 flex-1`}
            />
            <span className="w-[3.25rem] shrink-0 text-left font-mono text-[11px] tabular-nums text-white/75">
              {formatMediaTime(effectiveDurationMs)}
            </span>
          </div>
          <div className="group/volume flex shrink-0 items-center overflow-hidden">
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              className={COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS}
              onClick={toggleMute}
              aria-label={muteLabel}
              title={muteLabel}
            >
              {muted || volume <= 0 ? (
                <VolumeX className="h-3 w-3" />
              ) : (
                <Volume2 className="h-3 w-3" />
              )}
            </Button>
            <input
              type="range"
              min={0}
              max={1}
              step={0.01}
              value={visibleVolume}
              onChange={(event) =>
                handleVolumeChange(Number(event.target.value))
              }
              aria-label={props.text.completed.previewVolume}
              title={props.text.completed.previewVolume}
              className={COMPLETED_PREVIEW_VOLUME_RANGE_CLASS}
            />
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              className={COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS}
              onClick={toggleWindowedFullscreen}
              aria-label={windowedFullscreenLabel}
              title={windowedFullscreenLabel}
            >
              {windowedFullscreen ? (
                <Minimize2 className="h-3 w-3" />
              ) : (
                <Maximize2 className="h-3 w-3" />
              )}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              className={COMPLETED_PREVIEW_CONTROL_BUTTON_CLASS}
              onClick={toggleScreenFullscreen}
              aria-label={screenFullscreenLabel}
              title={screenFullscreenLabel}
            >
              {screenFullscreen ? (
                <Minimize className="h-3 w-3" />
              ) : (
                <Maximize className="h-3 w-3" />
              )}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
