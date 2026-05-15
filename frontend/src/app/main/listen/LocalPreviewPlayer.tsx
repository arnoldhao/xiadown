import {
MediaPlayer,
MediaProvider,
MediaRemoteControl,
getTimeRangesEnd
} from "@vidstack/react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_HIDDEN_ENGINE_STYLE } from "@/shared/styles/listen";

import { clampVolume,resolveAudioSource } from "@/app/main/listen/local-library";
import type { ListenLocalPreviewTrack,ListenPlaybackProgressState } from "@/app/main/listen/types";
import { ListenProgressBar,ListenTransportActions } from "@/app/main/listen/ui";

type LocalPreviewProgressState = {
  position: number;
  duration: number;
  volume: number;
  muted: boolean;
  updatedAt: number;
};

const LOCAL_PREVIEW_PROGRESS_STORAGE_PREFIX =
  "xiadown:local-preview-progress:v1:";
const LOCAL_PREVIEW_PROGRESS_SAVE_INTERVAL_MS = 2000;
const LOCAL_PREVIEW_PROGRESS_MAX_AGE_MS = 1000 * 60 * 60 * 24 * 90;
const LOCAL_PREVIEW_RESUME_MIN_POSITION = 1;
const LOCAL_PREVIEW_RESUME_END_GAP = 5;

function resolveLocalPreviewProgressKey(persistKey?: string) {
  const source = (persistKey ?? "").trim();
  return source ? `${LOCAL_PREVIEW_PROGRESS_STORAGE_PREFIX}${source}` : "";
}

function readLocalPreviewProgress(key: string): LocalPreviewProgressState | null {
  if (!key || typeof window === "undefined") {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as Partial<LocalPreviewProgressState>;
    const updatedAt = Number(parsed.updatedAt ?? 0);
    if (
      !Number.isFinite(updatedAt) ||
      Date.now() - updatedAt > LOCAL_PREVIEW_PROGRESS_MAX_AGE_MS
    ) {
      window.localStorage.removeItem(key);
      return null;
    }
    return {
      position: Math.max(0, Number(parsed.position ?? 0)),
      duration: Math.max(0, Number(parsed.duration ?? 0)),
      volume: clampVolume(Number(parsed.volume ?? 1)),
      muted: Boolean(parsed.muted),
      updatedAt,
    };
  } catch {
    return null;
  }
}

function writeLocalPreviewProgress(key: string, state: LocalPreviewProgressState) {
  if (!key || typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(key, JSON.stringify(state));
  } catch {
    // Storage can be unavailable in restricted WebViews.
  }
}

export function ListenLocalPreviewPlayer(props: {
  track: ListenLocalPreviewTrack;
  text: ReturnType<typeof getXiaText>;
  className?: string;
  persistKey?: string;
}) {
  const playerRef = React.useRef<React.ElementRef<typeof MediaPlayer> | null>(
    null,
  );
  const localRemote = React.useMemo(() => new MediaRemoteControl(), []);
  const restoredProgressKeyRef = React.useRef("");
  const lastProgressSaveAtRef = React.useRef(0);
  const [playing, setPlaying] = React.useState(false);
  const [muted, setMuted] = React.useState(false);
  const [volume, setVolume] = React.useState(1);
  const lastNonZeroVolumeRef = React.useRef(1);
  const [progress, setProgress] = React.useState<ListenPlaybackProgressState>({
    currentTime: 0,
    duration: 0,
    bufferedTime: 0,
  });
  const track = props.track;
  const author = track.author?.trim() || props.text.listen.linger;
  const progressStorageKey = React.useMemo(
    () => resolveLocalPreviewProgressKey(props.persistKey),
    [props.persistKey],
  );

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

  const persistLocalPreviewState = React.useCallback(
    (
      force = false,
      overrides?: {
        muted?: boolean;
        volume?: number;
      },
    ) => {
      if (!progressStorageKey) {
        return;
      }
      const media = getLocalMediaElement();
      if (!media) {
        return;
      }
      const now = Date.now();
      if (
        !force &&
        now - lastProgressSaveAtRef.current <
          LOCAL_PREVIEW_PROGRESS_SAVE_INTERVAL_MS
      ) {
        return;
      }
      lastProgressSaveAtRef.current = now;
      const duration = Math.max(
        0,
        Number.isFinite(media.duration) ? media.duration : progress.duration,
      );
      const rawPosition = Math.max(0, media.currentTime);
      const nearEnd =
        duration > 0 &&
        rawPosition >= Math.max(0, duration - LOCAL_PREVIEW_RESUME_END_GAP);
      writeLocalPreviewProgress(progressStorageKey, {
        position: nearEnd ? 0 : rawPosition,
        duration,
        volume: clampVolume(overrides?.volume ?? media.volume ?? volume),
        muted: Boolean(overrides?.muted ?? media.muted ?? muted),
        updatedAt: now,
      });
    },
    [
      getLocalMediaElement,
      muted,
      progress.duration,
      progressStorageKey,
      volume,
    ],
  );

  const restoreLocalPreviewState = React.useCallback(() => {
    if (!progressStorageKey) {
      return;
    }
    if (restoredProgressKeyRef.current === progressStorageKey) {
      return;
    }
    const media = getLocalMediaElement();
    if (!media) {
      return;
    }
    restoredProgressKeyRef.current = progressStorageKey;
    const stored = readLocalPreviewProgress(progressStorageKey);
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
    const duration = Math.max(
      stored.duration,
      Number.isFinite(media.duration) ? media.duration : 0,
    );
    const shouldResume =
      stored.position >= LOCAL_PREVIEW_RESUME_MIN_POSITION &&
      (duration <= 0 ||
        stored.position < duration - LOCAL_PREVIEW_RESUME_END_GAP);
    if (!shouldResume) {
      return;
    }
    try {
      media.currentTime = duration
        ? Math.max(0, Math.min(stored.position, duration))
        : stored.position;
      setProgress((current) => ({
        ...current,
        currentTime: media.currentTime,
        duration: duration || current.duration,
      }));
    } catch {
      // Some media elements reject seeking until more data is buffered.
    }
  }, [getLocalMediaElement, progressStorageKey]);

  const readLocalProgress = React.useCallback(() => {
    const player = playerRef.current;
    const media = getLocalMediaElement();
    const source = media ?? player;
    const currentTime =
      source && Number.isFinite(source.currentTime)
        ? Math.max(0, source.currentTime)
        : 0;
    const duration =
      source && Number.isFinite(source.duration)
        ? Math.max(0, source.duration)
        : 0;
    const buffered = (source as { buffered?: TimeRanges } | null)?.buffered;
    const bufferedTime = buffered
      ? Math.max(0, getTimeRangesEnd(buffered) ?? 0)
      : 0;

    setProgress((current) => {
      if (
        Math.abs(current.currentTime - currentTime) < 0.05 &&
        Math.abs(current.duration - duration) < 0.05 &&
        Math.abs(current.bufferedTime - bufferedTime) < 0.25
      ) {
        return current;
      }
      return { currentTime, duration, bufferedTime };
    });
    if (media) {
      persistLocalPreviewState();
    }
  }, [getLocalMediaElement, persistLocalPreviewState]);

  React.useEffect(() => {
    setPlaying(false);
    setProgress({ currentTime: 0, duration: 0, bufferedTime: 0 });
    restoredProgressKeyRef.current = "";
    lastProgressSaveAtRef.current = 0;
  }, [progressStorageKey, track.id]);

  React.useEffect(() => {
    const player = playerRef.current;
    if (!player) {
      return;
    }
    localRemote.setPlayer(player);
  }, [localRemote, track.id]);

  React.useEffect(() => {
    const media = getLocalMediaElement();
    const player = playerRef.current;
    const nextVolume = clampVolume(volume);
    const nextMuted = muted || volume <= 0;
    if (media) {
      media.volume = nextVolume;
      media.muted = nextMuted;
    }
    if (player) {
      player.volume = nextVolume;
      player.muted = nextMuted;
    }
  }, [getLocalMediaElement, muted, track.id, volume]);

  React.useEffect(() => {
    const timer = window.setInterval(readLocalProgress, 250);
    return () => window.clearInterval(timer);
  }, [readLocalProgress]);

  const handleSeek = React.useCallback(
    (seconds: number) => {
      const media = getLocalMediaElement();
      const player = playerRef.current;
      const source = media ?? player;
      const duration =
        source && Number.isFinite(source.duration)
          ? Math.max(0, source.duration)
          : Math.max(0, progress.duration);
      if (duration <= 0) {
        return;
      }
      const nextTime = Math.max(0, Math.min(seconds, duration));
      if (media) {
        media.currentTime = nextTime;
      }
      if (player) {
        player.currentTime = nextTime;
      }
      setProgress((current) => ({
        ...current,
        currentTime: nextTime,
        duration,
      }));
      persistLocalPreviewState(true);
    },
    [getLocalMediaElement, persistLocalPreviewState, progress.duration],
  );

  const handleTogglePlayback = React.useCallback<
    React.MouseEventHandler<HTMLButtonElement>
  >(
    (event) => {
      const media = getLocalMediaElement();
      const player = playerRef.current;
      if (playing) {
        persistLocalPreviewState(true);
        media?.pause();
        if (player) {
          player.paused = true;
        }
        localRemote.pause(event.nativeEvent);
        return;
      }
      if (media) {
        void media.play().catch(() => {});
      }
      if (player) {
        player.paused = false;
      }
      localRemote.play(event.nativeEvent);
    },
    [getLocalMediaElement, localRemote, persistLocalPreviewState, playing],
  );

  const handleToggleMute = React.useCallback(() => {
    setMuted((current) => {
      if (current || volume <= 0) {
        const restoredVolume = lastNonZeroVolumeRef.current;
        setVolume(restoredVolume);
        persistLocalPreviewState(true, {
          muted: false,
          volume: restoredVolume,
        });
        return false;
      }
      lastNonZeroVolumeRef.current = volume > 0 ? volume : 1;
      persistLocalPreviewState(true, { muted: true, volume });
      return true;
    });
  }, [persistLocalPreviewState, volume]);

  const handleVolumeChange = React.useCallback((value: number) => {
    const nextVolume = clampVolume(value);
    if (nextVolume > 0) {
      lastNonZeroVolumeRef.current = nextVolume;
    }
    setVolume(nextVolume);
    setMuted(nextVolume <= 0);
    persistLocalPreviewState(true, {
      muted: nextVolume <= 0,
      volume: nextVolume,
    });
  }, [persistLocalPreviewState]);

  return (
    <div
      className={cn(
        "relative h-full min-h-[16rem] overflow-hidden",
        props.className,
      )}
    >
      <MediaPlayer
        ref={playerRef}
        key={track.id}
        src={resolveAudioSource(track.previewURL, track.path)}
        title={track.title}
        viewType="audio"
        streamType="on-demand"
        load="eager"
        preload="metadata"
        playsInline
        onPlay={() => setPlaying(true)}
        onPause={() => {
          setPlaying(false);
          persistLocalPreviewState(true);
        }}
        onTimeUpdate={() => readLocalProgress()}
        onCanPlay={() => {
          restoreLocalPreviewState();
          readLocalProgress();
        }}
        onEnded={() => {
          setPlaying(false);
          readLocalProgress();
          persistLocalPreviewState(true);
        }}
        className="pointer-events-none"
        style={LISTEN_HIDDEN_ENGINE_STYLE}
      >
        <MediaProvider />
      </MediaPlayer>

      <div className="flex h-full min-h-0 flex-1 items-center justify-center overflow-hidden p-6 sm:p-8">
        <div className="flex w-full max-w-lg flex-col items-center justify-center text-center">
          <div className="min-w-0">
            <div className="overflow-hidden text-balance text-xl font-semibold leading-tight text-sidebar-foreground [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] sm:text-2xl">
              {track.title}
            </div>
            <div className="mt-3 truncate text-sm text-sidebar-foreground/58 sm:text-base">
              {author}
            </div>
          </div>

          <div className="mt-8 flex min-h-[2.75rem] w-full max-w-md items-start">
            <ListenProgressBar
              currentTime={progress.currentTime}
              duration={progress.duration}
              bufferedTime={progress.bufferedTime}
              ariaLabel={props.text.listen.seek}
              onSeek={handleSeek}
              className="mx-auto max-w-md"
            />
          </div>

          <div className="mt-6">
            <ListenTransportActions
              hasTrack
              playing={playing}
              showQueueControls={false}
              loading={false}
              muted={muted}
              volume={volume}
              playMode="order"
              text={props.text}
              onPrevious={() => {}}
              onNext={() => {}}
              onTogglePlayMode={() => {}}
              onTogglePlayback={handleTogglePlayback}
              onToggleMute={handleToggleMute}
              onVolumeChange={handleVolumeChange}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
