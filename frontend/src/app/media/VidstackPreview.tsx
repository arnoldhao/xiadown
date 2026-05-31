import * as React from "react";
import { Events, Window } from "@wailsio/runtime";
import {
  MediaPlayer,
  MediaProvider,
  MediaRemoteControl,
  type MediaStreamType,
  type PlayerSrc,
} from "@vidstack/react";
import {
  Loader2,
  Maximize,
  Maximize2,
  Minimize,
  Minimize2,
  Pause,
  Play,
  VideoOff,
  Volume2,
  VolumeX,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import {
  VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS,
  VIDSTACK_PREVIEW_CONTROL_RANGE_CLASS,
  VIDSTACK_PREVIEW_SHELL_CLASS,
  VIDSTACK_PREVIEW_VOLUME_RANGE_CLASS,
} from "@/shared/styles/xiadown";

export type VidstackPreviewLabels = {
  loading?: string;
  noPreview: string;
  previewEnterFullscreen: string;
  previewExitFullscreen: string;
  previewLoading?: string;
  previewLoadingStream?: string;
  previewMute: string;
  previewPause: string;
  previewPlay: string;
  previewPlaybackStalled: string;
  previewSeek: string;
  previewUnmute: string;
  previewVolume: string;
  previewWindowFullscreen: string;
  previewWindowRestore: string;
};

export type VidstackPreviewProps = {
  labels: VidstackPreviewLabels;
  mediaUrl: string;
  title: string;
  persistKey?: string;
  posterUrl?: string;
  durationMs?: number;
  streamType?: MediaStreamType;
  sourceType?: "hls" | "dash";
  persistProgress?: boolean;
  onPresentationModeChange?: (active: boolean) => void;
};

type MediaPlayerElement = React.ElementRef<typeof MediaPlayer>;
type PreviewLoadState = "loading" | "ready" | "unavailable";
type PreviewFullscreenMode = "dom" | "wails";
type VidstackPreviewProgressState = {
  positionMs: number;
  durationMs: number;
  volume: number;
  muted: boolean;
  updatedAt: number;
};
type VidstackPreviewVolumeState = {
  volume: number;
  muted: boolean;
  updatedAt: number;
};
type MediaProviderHandle = {
  audio?: unknown;
  media?: unknown;
  video?: unknown;
};

const VIDSTACK_PREVIEW_PROGRESS_STORAGE_PREFIX =
  "xiadown:completed-preview-progress:v1:";
const VIDSTACK_PREVIEW_VOLUME_STORAGE_KEY =
  "xiadown:completed-preview-volume:v1";
const VIDSTACK_PREVIEW_PROGRESS_SAVE_INTERVAL_MS = 2000;
const VIDSTACK_PREVIEW_PROGRESS_MAX_AGE_MS = 1000 * 60 * 60 * 24 * 90;
const VIDSTACK_PREVIEW_RESUME_MIN_POSITION_MS = 1000;
const VIDSTACK_PREVIEW_RESUME_END_GAP_MS = 5000;
const VIDSTACK_PREVIEW_RESUME_SEEK_TOLERANCE_MS = 750;
const VIDSTACK_PREVIEW_RESUME_RETRY_WINDOW_MS = 8000;
const VIDSTACK_PREVIEW_LOAD_TIMEOUT_MS = 5000;
const VIDSTACK_PREVIEW_STREAM_LOAD_TIMEOUT_MS = 30000;

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
  sourceType?: VidstackPreviewProps["sourceType"],
): PlayerSrc {
  if (sourceType === "hls") {
    return { src: mediaUrl, type: "application/vnd.apple.mpegurl" };
  }
  if (sourceType === "dash") {
    return { src: mediaUrl, type: "application/dash+xml" };
  }
  switch (resolveMediaFileExtension(mediaUrl, title)) {
    case "m3u8":
      return { src: mediaUrl, type: "application/vnd.apple.mpegurl" };
    case "mpd":
      return { src: mediaUrl, type: "application/dash+xml" };
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
  const message = normalizePlaybackErrorText(
    error instanceof Error ? error.message : "",
  );
  return message || fallback;
}

function normalizePlaybackErrorText(message: string) {
  return message.replace(/\s+/g, " ").trim().slice(0, 180);
}

function resolvePreviewPlaybackErrorMessage(params: {
  detail?: string;
  fallback: string;
  labels: VidstackPreviewLabels;
  mediaError?: MediaError | null;
  reason?: "play" | "stalled";
}) {
  if (params.reason === "stalled") {
    return params.labels.previewPlaybackStalled;
  }
  return (
    normalizePlaybackErrorText(
      params.mediaError?.message || params.detail || "",
    ) || params.fallback
  );
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
  return source ? `${VIDSTACK_PREVIEW_PROGRESS_STORAGE_PREFIX}${source}` : "";
}

function readStoredPreviewProgress(
  key: string,
): VidstackPreviewProgressState | null {
  if (!key || typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<VidstackPreviewProgressState>;
    const updatedAt = Number(parsed.updatedAt ?? 0);
    if (
      !Number.isFinite(updatedAt) ||
      Date.now() - updatedAt > VIDSTACK_PREVIEW_PROGRESS_MAX_AGE_MS
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

function readStoredPreviewVolume(): VidstackPreviewVolumeState | null {
  if (typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(VIDSTACK_PREVIEW_VOLUME_STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<VidstackPreviewVolumeState>;
    const updatedAt = Number(parsed.updatedAt ?? 0);
    return {
      volume: clampVolume(Number(parsed.volume ?? 1)),
      muted: Boolean(parsed.muted),
      updatedAt: Number.isFinite(updatedAt) ? updatedAt : 0,
    };
  } catch {
    return null;
  }
}

function writeStoredPreviewProgress(
  key: string,
  state: VidstackPreviewProgressState,
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

function writeStoredPreviewVolume(
  state: Pick<VidstackPreviewVolumeState, "volume" | "muted"> &
    Partial<Pick<VidstackPreviewVolumeState, "updatedAt">>,
) {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(
      VIDSTACK_PREVIEW_VOLUME_STORAGE_KEY,
      JSON.stringify({
        volume: clampVolume(Number(state.volume)),
        muted: Boolean(state.muted),
        updatedAt: Number.isFinite(state.updatedAt)
          ? state.updatedAt
          : Date.now(),
      }),
    );
  } catch {
    // Storage can be unavailable in restricted WebViews.
  }
}

export function VidstackPreview(props: VidstackPreviewProps) {
  const [initialVolumeState] = React.useState(() => readStoredPreviewVolume());
  const shellRef = React.useRef<HTMLDivElement | null>(null);
  const fullscreenModeRef = React.useRef<PreviewFullscreenMode | null>(null);
  const previousWindowedFullscreenRef = React.useRef(false);
  const lastNonZeroVolumeRef = React.useRef(
    initialVolumeState && initialVolumeState.volume > 0
      ? initialVolumeState.volume
      : 1,
  );
  const animationFrameRef = React.useRef<number>();
  const loadWatchdogRef = React.useRef<number>();
  const playbackWatchdogRef = React.useRef<number>();
  const pendingPlayRef = React.useRef(false);
  const playRequestSerialRef = React.useRef(0);
  const playRequestInFlightRef = React.useRef(false);
  const restoredProgressKeyRef = React.useRef("");
  const restoredVolumeMediaRef = React.useRef<HTMLMediaElement | null>(null);
  const resumeTargetMsRef = React.useRef<number | null>(null);
  const resumeRetryUntilRef = React.useRef(0);
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
  const [volume, setVolume] = React.useState(
    initialVolumeState?.volume ?? 1,
  );
  const [muted, setMuted] = React.useState(
    initialVolumeState?.muted ?? false,
  );
  const [playbackError, setPlaybackError] = React.useState("");
  const [previewLoadState, setPreviewLoadState] =
    React.useState<PreviewLoadState>(() =>
      props.mediaUrl ? "loading" : "unavailable",
    );
  const [windowedFullscreen, setWindowedFullscreen] = React.useState(false);
  const [screenFullscreen, setScreenFullscreen] = React.useState(false);
  const presentationModeActive = windowedFullscreen || screenFullscreen;

  const effectiveDurationMs = Math.max(resolvedDurationMs, 0);
  const playerSource = React.useMemo(
    () => resolveVideoSource(props.mediaUrl, props.title, props.sourceType),
    [props.mediaUrl, props.sourceType, props.title],
  );
  const isStreamSource =
    props.sourceType === "hls" ||
    props.sourceType === "dash" ||
    String(props.streamType ?? "").startsWith("live");
  const shouldPersistProgress =
    props.persistProgress ?? !String(props.streamType ?? "").startsWith("live");
  const progressStorageKey = React.useMemo(
    () =>
      shouldPersistProgress
        ? resolveProgressStorageKey(props.persistKey, props.mediaUrl)
        : "",
    [props.mediaUrl, props.persistKey, shouldPersistProgress],
  );

  const getActiveMediaElement = React.useCallback(() => {
    const current = resolvePlayerMediaElement(playerElement);
    if (current) {
      return current;
    }
    return mediaElement?.isConnected ? mediaElement : null;
  }, [mediaElement, playerElement]);

  const persistPreviewVolumeState = React.useCallback(
    (nextVolume: number, nextMuted: boolean) => {
      writeStoredPreviewVolume({
        volume: clampVolume(nextVolume),
        muted: nextMuted,
      });
    },
    [],
  );

  const applyPersistedPreviewVolume = React.useCallback(
    (
      media: HTMLMediaElement,
      fallback?: VidstackPreviewVolumeState | null,
    ) => {
      const storedVolume = readStoredPreviewVolume();
      const nextVolumeState = storedVolume ?? fallback;
      if (!nextVolumeState) {
        return;
      }
      const alreadySynced =
        restoredVolumeMediaRef.current === media &&
        Math.abs(media.volume - nextVolumeState.volume) < 0.001 &&
        media.muted === nextVolumeState.muted;
      if (alreadySynced) {
        return;
      }

      restoredVolumeMediaRef.current = media;
      media.volume = nextVolumeState.volume;
      media.muted = nextVolumeState.muted;
      if (playerElement) {
        playerElement.volume = nextVolumeState.volume;
        playerElement.muted = nextVolumeState.muted;
      }
      setVolume(nextVolumeState.volume);
      setMuted(nextVolumeState.muted);
      if (nextVolumeState.volume > 0) {
        lastNonZeroVolumeRef.current = nextVolumeState.volume;
      }
      if (!storedVolume && fallback) {
        writeStoredPreviewVolume(nextVolumeState);
      }
    },
    [playerElement],
  );

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

  const clearLoadWatchdog = React.useCallback(() => {
    if (loadWatchdogRef.current) {
      window.clearTimeout(loadWatchdogRef.current);
      loadWatchdogRef.current = undefined;
    }
  }, []);

  const markPreviewReady = React.useCallback(
    (media?: HTMLMediaElement | null) => {
      clearLoadWatchdog();
      if (media) {
        setMediaElement(media);
      }
      setPreviewLoadState("ready");
      setPlaybackError("");
    },
    [clearLoadWatchdog],
  );

  const markPreviewUnavailable = React.useCallback(
    (message: string) => {
      clearLoadWatchdog();
      cancelPendingPlay();
      setIsPlaying(false);
      setPlaybackError(message);
      setPreviewLoadState("unavailable");
    },
    [cancelPendingPlay, clearLoadWatchdog],
  );

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
      markPreviewUnavailable(
        resolvePreviewPlaybackErrorMessage({
          fallback: props.labels.noPreview,
          labels: props.labels,
          reason: "stalled",
        }),
      );
    }, 8000);
  }, [
    finishPendingPlay,
    getActiveMediaElement,
    markPreviewUnavailable,
    props.labels,
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
        markPreviewUnavailable(
          resolvePreviewPlaybackErrorMessage({
            detail: resolvePlaybackErrorMessage(error, ""),
            fallback: props.labels.noPreview,
            labels: props.labels,
            reason: "play",
          }),
        );
      }
    },
    [
      finishPendingPlay,
      markPreviewUnavailable,
      props.labels,
      startPlaybackWatchdog,
    ],
  );

  const applyPendingResumeTarget = React.useCallback(
    (media: HTMLMediaElement, force = false) => {
      const targetMs = resumeTargetMsRef.current;
      if (
        !targetMs ||
        targetMs < VIDSTACK_PREVIEW_RESUME_MIN_POSITION_MS
      ) {
        return false;
      }

      const now = Date.now();
      if (now > resumeRetryUntilRef.current) {
        if (!force) {
          resumeTargetMsRef.current = null;
          resumeRetryUntilRef.current = 0;
          return false;
        }
        resumeRetryUntilRef.current =
          now + VIDSTACK_PREVIEW_RESUME_RETRY_WINDOW_MS;
      }

      const currentMs = Math.max(0, Math.round(media.currentTime * 1000));
      if (currentMs > targetMs + VIDSTACK_PREVIEW_RESUME_SEEK_TOLERANCE_MS) {
        resumeTargetMsRef.current = null;
        resumeRetryUntilRef.current = 0;
        return false;
      }

      if (
        Math.abs(currentMs - targetMs) <=
          VIDSTACK_PREVIEW_RESUME_SEEK_TOLERANCE_MS
      ) {
        return false;
      }

      try {
        media.currentTime = targetMs / 1000;
        setCurrentTimeMs(targetMs);
        return true;
      } catch {
        return false;
      }
    },
    [],
  );

  const shouldDeferResumeProgressPersistence = React.useCallback(
    (media: HTMLMediaElement) => {
      const targetMs = resumeTargetMsRef.current;
      if (
        !targetMs ||
        targetMs < VIDSTACK_PREVIEW_RESUME_MIN_POSITION_MS
      ) {
        return false;
      }
      if (Date.now() > resumeRetryUntilRef.current) {
        resumeTargetMsRef.current = null;
        resumeRetryUntilRef.current = 0;
        return false;
      }
      const currentMs = Math.max(0, Math.round(media.currentTime * 1000));
      return (
        currentMs + VIDSTACK_PREVIEW_RESUME_SEEK_TOLERANCE_MS < targetMs
      );
    },
    [],
  );

  const persistPlaybackState = React.useCallback(
    (media: HTMLMediaElement, force = false) => {
      if (!progressStorageKey) {
        return;
      }
      if (
        applyPendingResumeTarget(media) ||
        shouldDeferResumeProgressPersistence(media)
      ) {
        return;
      }
      const now = Date.now();
      if (
        !force &&
        now - lastProgressSaveAtRef.current <
          VIDSTACK_PREVIEW_PROGRESS_SAVE_INTERVAL_MS
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
          Math.max(0, durationMs - VIDSTACK_PREVIEW_RESUME_END_GAP_MS);
      writeStoredPreviewProgress(progressStorageKey, {
        positionMs: nearEnd ? 0 : rawPositionMs,
        durationMs,
        volume: clampVolume(Number(media.volume ?? volume)),
        muted: Boolean(media.muted ?? muted),
        updatedAt: now,
      });
    },
    [
      applyPendingResumeTarget,
      effectiveDurationMs,
      muted,
      progressStorageKey,
      shouldDeferResumeProgressPersistence,
      volume,
    ],
  );

  const restorePersistedPlaybackState = React.useCallback(
    (media: HTMLMediaElement) => {
      if (!progressStorageKey) {
        applyPersistedPreviewVolume(media);
        return;
      }
      if (restoredProgressKeyRef.current === progressStorageKey) {
        applyPersistedPreviewVolume(media);
        applyPendingResumeTarget(media, true);
        return;
      }
      const stored = readStoredPreviewProgress(progressStorageKey);
      restoredProgressKeyRef.current = progressStorageKey;
      applyPersistedPreviewVolume(
        media,
        stored
          ? {
              volume: stored.volume,
              muted: stored.muted,
              updatedAt: stored.updatedAt,
            }
          : null,
      );
      if (!stored) {
        resumeTargetMsRef.current = null;
        resumeRetryUntilRef.current = 0;
        return;
      }

      const durationMs = Math.max(
        stored.durationMs,
        props.durationMs ?? 0,
        Number.isFinite(media.duration) && media.duration > 0
          ? Math.round(media.duration * 1000)
          : 0,
      );
      const shouldResume =
        stored.positionMs >= VIDSTACK_PREVIEW_RESUME_MIN_POSITION_MS &&
        (durationMs <= 0 ||
          stored.positionMs <
            durationMs - VIDSTACK_PREVIEW_RESUME_END_GAP_MS);
      if (!shouldResume) {
        resumeTargetMsRef.current = null;
        resumeRetryUntilRef.current = 0;
      } else {
        resumeTargetMsRef.current = durationMs
          ? clampMs(stored.positionMs, durationMs)
          : stored.positionMs;
        resumeRetryUntilRef.current =
          Date.now() + VIDSTACK_PREVIEW_RESUME_RETRY_WINDOW_MS;
      }

      if (shouldResume) {
        applyPendingResumeTarget(media, true);
      }
    },
    [
      applyPendingResumeTarget,
      applyPersistedPreviewVolume,
      progressStorageKey,
      props.durationMs,
    ],
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
    restoredVolumeMediaRef.current = null;
    resumeTargetMsRef.current = null;
    resumeRetryUntilRef.current = 0;
    lastProgressSaveAtRef.current = 0;
    setMediaElement(null);
    setPlaybackError("");
  }, [cancelPendingPlay, progressStorageKey, props.durationMs, props.mediaUrl]);

  React.useEffect(() => {
    clearLoadWatchdog();
    if (!props.mediaUrl) {
      setPreviewLoadState("unavailable");
      return;
    }

    setPreviewLoadState("loading");
    loadWatchdogRef.current = window.setTimeout(() => {
      markPreviewUnavailable(
        resolvePreviewPlaybackErrorMessage({
          fallback: props.labels.noPreview,
          labels: props.labels,
          reason: "stalled",
        }),
      );
    }, isStreamSource
      ? VIDSTACK_PREVIEW_STREAM_LOAD_TIMEOUT_MS
      : VIDSTACK_PREVIEW_LOAD_TIMEOUT_MS);

    return clearLoadWatchdog;
  }, [clearLoadWatchdog, isStreamSource, markPreviewUnavailable, props.mediaUrl]);

  React.useEffect(() => {
    mediaRemote.setPlayer(playerElement);
    return () => mediaRemote.setPlayer(null);
  }, [mediaRemote, playerElement]);

  React.useEffect(() => {
    const player = playerElement;
    if (!player) {
      return;
    }
    const storedVolume = readStoredPreviewVolume() ?? initialVolumeState;
    const nextVolume = storedVolume
      ? storedVolume.volume
      : clampVolume(Number(player.volume ?? 1));
    const nextMuted = storedVolume ? storedVolume.muted : Boolean(player.muted);
    player.volume = nextVolume;
    player.muted = nextMuted;
    setVolume(nextVolume);
    if (nextVolume > 0) {
      lastNonZeroVolumeRef.current = nextVolume;
    }
    setMuted(nextMuted);
  }, [initialVolumeState, playerElement]);

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
      markPreviewReady(media);
      const nextDurationMs = Number.isFinite(media.duration)
        ? Math.round(media.duration * 1000)
        : 0;
      if (nextDurationMs > 0) {
        setResolvedDurationMs((current) => Math.max(current, nextDurationMs));
      }
      restorePersistedPlaybackState(media);
    },
    [markPreviewReady, restorePersistedPlaybackState],
  );

  const handleMediaCanPlay = React.useCallback(
    (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const media = event.currentTarget;
      markPreviewReady(media);
      restorePersistedPlaybackState(media);
      if (pendingPlayRef.current && !playRequestInFlightRef.current) {
        void playNativeMedia(media);
      }
    },
    [markPreviewReady, playNativeMedia, restorePersistedPlaybackState],
  );

  const handleMediaError = React.useCallback(
    (event: React.SyntheticEvent<HTMLMediaElement>) => {
      markPreviewUnavailable(
        resolvePreviewPlaybackErrorMessage({
          fallback: props.labels.noPreview,
          labels: props.labels,
          mediaError: event.currentTarget.error,
        }),
      );
    },
    [markPreviewUnavailable, props.labels],
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
      if (isHTMLMediaElement(media)) {
        markPreviewReady(media);
        restorePersistedPlaybackState(media);
      } else {
        markPreviewReady();
      }
      setPlaybackError("");
      setIsPlaying(true);
    };
    const handlePlaying = () => {
      if (isHTMLMediaElement(media)) {
        markPreviewReady(media);
        restorePersistedPlaybackState(media);
      } else {
        markPreviewReady();
      }
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
      const nextMuted = Boolean(media.muted);
      setVolume(nextVolume);
      setMuted(nextMuted);
      if (nextVolume > 0) {
        lastNonZeroVolumeRef.current = nextVolume;
      }
      persistPreviewVolumeState(nextVolume, nextMuted);
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
      const nativeMedia = isHTMLMediaElement(media) ? media : mediaElement;
      if (nativeMedia) {
        markPreviewReady(nativeMedia);
        restorePersistedPlaybackState(nativeMedia);
      } else {
        markPreviewReady();
      }
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
      markPreviewUnavailable(
        resolvePreviewPlaybackErrorMessage({
          fallback: props.labels.noPreview,
          labels: props.labels,
          mediaError: nativeMedia?.error,
        }),
      );
    };
    const handleTimeUpdate = () => {
      if (pendingPlayRef.current) {
        finishPendingPlay();
      }
      if (isHTMLMediaElement(media)) {
        markPreviewReady(media);
        restorePersistedPlaybackState(media);
      } else {
        markPreviewReady();
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
    markPreviewReady,
    markPreviewUnavailable,
    mediaElement,
    persistPlaybackState,
    persistPreviewVolumeState,
    playNativeMedia,
    playerElement,
    props.labels,
    restorePersistedPlaybackState,
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
    const restoreWindowedFullscreenAfterScreenExit = () => {
      setWindowedFullscreen(previousWindowedFullscreenRef.current);
      previousWindowedFullscreenRef.current = false;
    };

    const handleDomFullscreenChange = () => {
      if (fullscreenModeRef.current !== "dom") {
        return;
      }
      const isActive = document.fullscreenElement === shellRef.current;
      setScreenFullscreen(isActive);
      if (!isActive) {
        fullscreenModeRef.current = null;
        restoreWindowedFullscreenAfterScreenExit();
      }
    };

    document.addEventListener("fullscreenchange", handleDomFullscreenChange);
    return () => {
      document.removeEventListener(
        "fullscreenchange",
        handleDomFullscreenChange,
      );
    };
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

  React.useLayoutEffect(() => {
    props.onPresentationModeChange?.(presentationModeActive);
    return () => props.onPresentationModeChange?.(false);
  }, [presentationModeActive, props.onPresentationModeChange]);

  const previewReady = Boolean(props.mediaUrl) && previewLoadState === "ready";
  const previewLoading =
    Boolean(props.mediaUrl) && previewLoadState === "loading";
  const previewUnavailable =
    Boolean(props.mediaUrl) && previewLoadState === "unavailable";
  const previewControlsDisabled = !previewReady;

  const togglePlay = (trigger?: Event) => {
    if (previewControlsDisabled) {
      return;
    }
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

    if (activeMedia) {
      restorePersistedPlaybackState(activeMedia);
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
    if (previewControlsDisabled) {
      return;
    }
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
    if (previewControlsDisabled) {
      return;
    }
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
      persistPreviewVolumeState(restoredVolume, false);
      if (isHTMLMediaElement(activeMedia)) {
        persistPlaybackState(activeMedia, true);
      }
      return;
    }
    lastNonZeroVolumeRef.current = volume;
    activeMedia.muted = true;
    setMuted(true);
    persistPreviewVolumeState(volume, true);
    if (isHTMLMediaElement(activeMedia)) {
      persistPlaybackState(activeMedia, true);
    }
  };

  const handleVolumeChange = (value: number) => {
    if (previewControlsDisabled) {
      return;
    }
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
    persistPreviewVolumeState(nextVolume, nextVolume <= 0);
    if (isHTMLMediaElement(activeMedia)) {
      persistPlaybackState(activeMedia, true);
    }
  };

  const restoreWindowedFullscreenAfterScreenExit = () => {
    setWindowedFullscreen(previousWindowedFullscreenRef.current);
    previousWindowedFullscreenRef.current = false;
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
        restoreWindowedFullscreenAfterScreenExit();
      }
      return;
    }
    setScreenFullscreen(false);
    restoreWindowedFullscreenAfterScreenExit();
  }, []);

  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") {
        return;
      }
      if (screenFullscreen) {
        event.preventDefault();
        event.stopPropagation();
        void exitScreenFullscreen().catch(() => {
          setScreenFullscreen(false);
          restoreWindowedFullscreenAfterScreenExit();
        });
        return;
      }
      if (windowedFullscreen) {
        setWindowedFullscreen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [exitScreenFullscreen, screenFullscreen, windowedFullscreen]);

  const toggleWindowedFullscreen = () => {
    if (previewControlsDisabled || screenFullscreen) {
      return;
    }
    setWindowedFullscreen((value) => !value);
  };

  const toggleScreenFullscreen = () => {
    if (previewControlsDisabled) {
      return;
    }
    if (screenFullscreen) {
      void exitScreenFullscreen().catch(() => {
        setScreenFullscreen(false);
        restoreWindowedFullscreenAfterScreenExit();
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
              restoreWindowedFullscreenAfterScreenExit();
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
        restoreWindowedFullscreenAfterScreenExit();
      });
  };

  const visibleVolume = muted ? 0 : volume;
  const playLabel = isPlaying
    ? props.labels.previewPause
    : props.labels.previewPlay;
  const muteLabel =
    muted || volume <= 0
      ? props.labels.previewUnmute
      : props.labels.previewMute;
  const progressRangePercent =
    previewReady && effectiveDurationMs > 0
      ? Math.max(0, Math.min(100, (currentTimeMs / effectiveDurationMs) * 100))
      : 0;
  const volumeRangePercent = Math.max(0, Math.min(100, visibleVolume * 100));
  const currentTimeLabel = previewReady ? formatMediaTime(currentTimeMs) : "--:--";
  const durationLabel =
    previewReady && effectiveDurationMs > 0
      ? formatMediaTime(effectiveDurationMs)
      : "--:--";
  const loadingLabel =
    isStreamSource
      ? props.labels.previewLoadingStream ||
        props.labels.loading ||
        props.labels.noPreview
      : props.labels.previewLoading ||
        props.labels.loading ||
        props.labels.noPreview;
  const windowedFullscreenLabel = windowedFullscreen
    ? props.labels.previewWindowRestore
    : props.labels.previewWindowFullscreen;
  const screenFullscreenLabel = screenFullscreen
    ? props.labels.previewExitFullscreen
    : props.labels.previewEnterFullscreen;

  return (
    <div
      ref={shellRef}
      className={cn(
        VIDSTACK_PREVIEW_SHELL_CLASS,
        windowedFullscreen &&
          "fixed inset-0 z-[200] rounded-none border-0 shadow-none",
        screenFullscreen && "rounded-none border-0 shadow-none",
      )}
      data-preview-presentation={presentationModeActive ? "true" : undefined}
    >
        <div
          className={cn(
            "app-completed-preview-stage relative min-h-0 flex-1",
            (windowedFullscreen || screenFullscreen) && "rounded-none",
          )}
        >
          {props.mediaUrl ? (
            <div
              className={cn(
                "app-completed-preview-media-frame relative h-full w-full overflow-hidden",
                (windowedFullscreen || screenFullscreen) && "rounded-none",
              )}
            >
              <MediaPlayer
                key={props.mediaUrl}
                ref={handlePlayerRef}
                src={playerSource}
                title={props.title}
                viewType="video"
                streamType={props.streamType ?? "on-demand"}
                load="eager"
                volume={volume}
                muted={muted}
                controls={false}
                playsInline
                preload="auto"
                style={{ aspectRatio: "auto" }}
                className={cn(
                  "app-completed-preview-player h-full w-full transition-opacity duration-200",
                  !previewReady && "opacity-0",
                )}
              >
                <MediaProvider
                  className="app-completed-preview-provider h-full w-full overflow-hidden"
                  mediaProps={{
                    className: "h-full w-full object-contain object-center",
                    onLoadedMetadata: handleLoadedMetadata,
                    onLoadedData: handleMediaCanPlay,
                    onCanPlay: handleMediaCanPlay,
                    onError: handleMediaError,
                  }}
                />
              </MediaPlayer>
              {previewLoading ? (
                <div className="app-completed-preview-state pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 px-8 text-center">
                  <Loader2 className="h-6 w-6 animate-spin" />
                  <div className="app-completed-preview-state-text text-sm font-medium">
                    {loadingLabel}
                  </div>
                </div>
              ) : null}
              {previewUnavailable ? (
                <div
                  className="app-completed-preview-state pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 px-8 text-center"
                  aria-label={playbackError || props.labels.noPreview}
                  role="status"
                  title={playbackError || props.labels.noPreview}
                >
                  <VideoOff className="app-completed-preview-state-icon h-10 w-10" />
                  <div className="app-completed-preview-state-text text-sm font-medium">
                    {props.labels.noPreview}
                  </div>
                </div>
              ) : null}
            </div>
          ) : (
            <div
              className={cn(
                "app-completed-preview-empty flex h-full items-center justify-center px-8 text-center",
                (windowedFullscreen || screenFullscreen) && "rounded-none",
              )}
            >
              <div className="app-completed-preview-empty-text text-sm">
                {props.labels.noPreview}
              </div>
            </div>
          )}
        </div>

      <div
        className="app-completed-preview-control-bar shrink-0 px-3 py-1.5"
        data-disabled={previewControlsDisabled ? "true" : "false"}
      >
        <div className="flex min-w-0 items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="compactIcon"
            className={VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS}
            onClick={(event) => togglePlay(event.nativeEvent)}
            aria-label={playLabel}
            title={playLabel}
            disabled={previewControlsDisabled}
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
            <span className="app-completed-preview-time w-[3.25rem] shrink-0 text-right font-mono text-[11px] tabular-nums">
              {currentTimeLabel}
            </span>
            <input
              type="range"
              min={0}
              max={effectiveDurationMs || 1}
              value={
                previewReady ? Math.min(currentTimeMs, effectiveDurationMs || 1) : 0
              }
              onChange={(event) => handleSeek(Number(event.target.value))}
              aria-label={props.labels.previewSeek}
              disabled={previewControlsDisabled}
              className={`${VIDSTACK_PREVIEW_CONTROL_RANGE_CLASS} min-w-0 flex-1`}
              style={
                {
                  "--completed-preview-range-value": `${progressRangePercent}%`,
                } as React.CSSProperties
              }
            />
            <span className="app-completed-preview-time w-[3.25rem] shrink-0 text-left font-mono text-[11px] tabular-nums">
              {durationLabel}
            </span>
          </div>
          <div className="group/volume relative flex shrink-0 items-center">
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              className={VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS}
              onClick={toggleMute}
              aria-label={muteLabel}
              title={muteLabel}
              disabled={previewControlsDisabled}
            >
              {muted || volume <= 0 ? (
                <VolumeX className="h-3 w-3" />
              ) : (
                <Volume2 className="h-3 w-3" />
              )}
            </Button>
            <div className="app-completed-preview-volume-popover pointer-events-none absolute bottom-full left-1/2 z-30 flex h-28 w-8 -translate-x-1/2 translate-y-1 items-center justify-center rounded-full opacity-0 transition-[opacity,transform] duration-150 ease-out group-hover/volume:pointer-events-auto group-hover/volume:translate-y-0 group-hover/volume:opacity-100 group-focus-within/volume:pointer-events-auto group-focus-within/volume:translate-y-0 group-focus-within/volume:opacity-100">
              <input
                type="range"
                min={0}
                max={1}
                step={0.01}
                value={visibleVolume}
                onChange={(event) =>
                  handleVolumeChange(Number(event.target.value))
                }
                aria-label={props.labels.previewVolume}
                title={props.labels.previewVolume}
                disabled={previewControlsDisabled}
                className={VIDSTACK_PREVIEW_VOLUME_RANGE_CLASS}
                style={
                  {
                    "--completed-preview-range-value": `${volumeRangePercent}%`,
                  } as React.CSSProperties
                }
              />
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              className={VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS}
              onClick={toggleWindowedFullscreen}
              aria-label={windowedFullscreenLabel}
              title={windowedFullscreenLabel}
              disabled={previewControlsDisabled || screenFullscreen}
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
              className={VIDSTACK_PREVIEW_CONTROL_BUTTON_CLASS}
              onClick={toggleScreenFullscreen}
              aria-label={screenFullscreenLabel}
              title={screenFullscreenLabel}
              disabled={previewControlsDisabled}
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
