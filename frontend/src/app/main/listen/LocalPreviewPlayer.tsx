import { Maximize2, Minimize2 } from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import {
  playbackSessionByID,
  usePlaybackCoordinator,
  type PlaybackMediaKind,
} from "@/shared/playback";
import { Button } from "@/shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

import { clampVolume } from "@/app/main/listen/local-library";
import type {
  ListenLocalPreviewTrack,
  ListenPlaybackProgressState,
} from "@/app/main/listen/types";
import {
  ListenProgressBar,
  ListenTransportActions,
} from "@/app/main/listen/ui";

type LocalPreviewProgressState = {
  position: number;
  duration: number;
  volume: number;
  muted: boolean;
  updatedAt: number;
};

const LOCAL_PREVIEW_PROGRESS_STORAGE_PREFIX =
  "xiadown:local-preview-progress:v2:";
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
      muted: parsed.muted === true,
      updatedAt,
    };
  } catch {
    return null;
  }
}

function writeLocalPreviewProgress(
  key: string,
  state: Omit<LocalPreviewProgressState, "updatedAt">,
) {
  if (!key || typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(
      key,
      JSON.stringify({ ...state, updatedAt: Date.now() }),
    );
  } catch {
    // Storage can be unavailable in restricted WebViews.
  }
}

export function ListenLocalPreviewPlayer(props: {
  track: ListenLocalPreviewTrack;
  text: ReturnType<typeof getXiaText>;
  className?: string;
  persistKey?: string;
  kind?: PlaybackMediaKind;
  posterURL?: string;
  durationMs?: number;
  onPresentationModeChange?: (active: boolean) => void;
}) {
  const kind = props.kind ?? "audio";
  const track = props.track;
  const author = track.author?.trim() || props.text.listen.linger;
  const progressStorageKey = React.useMemo(
    () => resolveLocalPreviewProgressKey(props.persistKey),
    [props.persistKey],
  );
  const sessionID = React.useMemo(
    () => `completed-preview:${track.id || props.persistKey || track.previewURL}`,
    [props.persistKey, track.id, track.previewURL],
  );
  const playback = usePlaybackCoordinator();
  const session = playbackSessionByID(playback.snapshot, sessionID);
  const sessionIsActive = playback.snapshot.active?.id === sessionID;
  const playing =
    sessionIsActive &&
    (session?.state === "playing" || session?.state === "buffering");
  const loading = sessionIsActive && session?.state === "loading";
  const startedSessionIDsRef = React.useRef(new Set<string>());
  const lastProgressSaveAtRef = React.useRef(0);
  const videoRef = React.useRef<HTMLVideoElement | null>(null);
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const lastNonZeroVolumeRef = React.useRef(1);
  const [storedState, setStoredState] = React.useState<LocalPreviewProgressState | null>(
    () => readLocalPreviewProgress(progressStorageKey),
  );
  const [draftPosition, setDraftPosition] = React.useState(
    () => storedState?.position ?? 0,
  );
  const [draftDuration, setDraftDuration] = React.useState(
    () => storedState?.duration ?? Math.max(0, (props.durationMs ?? 0) / 1000),
  );
  const [volume, setVolume] = React.useState(() => storedState?.volume ?? 1);
  const [muted, setMuted] = React.useState(() => storedState?.muted ?? false);
  const [presentationActive, setPresentationActive] = React.useState(false);

  const position = session?.position ?? draftPosition;
  const duration = Math.max(
    session?.duration ?? 0,
    session?.item.duration ?? 0,
    draftDuration,
  );
  const progress = React.useMemo<ListenPlaybackProgressState>(
    () => ({
      currentTime: position,
      duration,
      bufferedTime: position,
    }),
    [duration, position],
  );
  const effectiveVolume = session?.volume ?? volume;
  const effectiveMuted = session?.muted ?? muted;

  React.useEffect(() => {
    const restored = readLocalPreviewProgress(progressStorageKey);
    setStoredState(restored);
    setDraftPosition(restored?.position ?? 0);
    setDraftDuration(
      restored?.duration ?? Math.max(0, (props.durationMs ?? 0) / 1000),
    );
    setVolume(restored?.volume ?? 1);
    setMuted(restored?.muted ?? false);
    lastProgressSaveAtRef.current = 0;
    if ((restored?.volume ?? 1) > 0) {
      lastNonZeroVolumeRef.current = restored?.volume ?? 1;
    }
  }, [progressStorageKey, props.durationMs]);

  React.useEffect(() => {
    if (!session) {
      return;
    }
    setDraftPosition(session.position);
    setDraftDuration(Math.max(session.duration, session.item.duration ?? 0));
    setVolume(session.volume);
    setMuted(session.muted);
    if (session.volume > 0) {
      lastNonZeroVolumeRef.current = session.volume;
    }
    const now = Date.now();
    if (
      session.state === "playing" &&
      now - lastProgressSaveAtRef.current <
        LOCAL_PREVIEW_PROGRESS_SAVE_INTERVAL_MS
    ) {
      return;
    }
    lastProgressSaveAtRef.current = now;
    const nearEnd =
      session.duration > 0 &&
      session.position >=
        Math.max(0, session.duration - LOCAL_PREVIEW_RESUME_END_GAP);
    writeLocalPreviewProgress(progressStorageKey, {
      position: nearEnd ? 0 : session.position,
      duration: session.duration,
      volume: session.volume,
      muted: session.muted,
    });
  }, [progressStorageKey, session]);

  React.useEffect(() => {
    return () => {
      if (startedSessionIDsRef.current.delete(sessionID)) {
        void playback.commands.closeSession(sessionID).catch(() => {});
      }
    };
  }, [playback.commands, sessionID]);

  React.useEffect(() => {
    const handleFullscreenChange = () => {
      const active = document.fullscreenElement === containerRef.current;
      setPresentationActive(active);
      props.onPresentationModeChange?.(active);
    };
    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => {
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
      props.onPresentationModeChange?.(false);
    };
  }, [props.onPresentationModeChange]);

  React.useEffect(() => {
    if (kind !== "video") {
      return;
    }
    const video = videoRef.current;
    if (!video) {
      return;
    }
    // The coordinator owns the only audible media element. This frontend
    // element is a deliberately muted presentation plane for video frames.
    video.muted = true;
    video.defaultMuted = true;
    video.volume = 0;
    if (sessionIsActive && Math.abs(video.currentTime - position) > 0.6) {
      try {
        video.currentTime = position;
      } catch {
        // Seeking can be rejected until metadata is available.
      }
    }
    if (playing) {
      void video.play().catch(() => {});
    } else {
      video.pause();
    }
  }, [kind, playing, position, sessionIsActive]);

  const startPreview = React.useCallback(
    async (forceReload = false) => {
      const uri = track.path.trim() || track.previewURL.trim();
      if (!uri) {
        return;
      }
      const resumePosition =
        draftPosition >= LOCAL_PREVIEW_RESUME_MIN_POSITION &&
        (duration <= 0 ||
          draftPosition < duration - LOCAL_PREVIEW_RESUME_END_GAP)
          ? draftPosition
          : 0;
      startedSessionIDsRef.current.add(sessionID);
      await playback.commands.startTransientPreview({
        sessionId: sessionID,
        item: {
          id: track.id || sessionID,
          kind,
          source: { provider: "local", uri },
          title: track.title,
          artist: track.author,
          artworkUrl: props.posterURL || track.coverURL,
          duration: Math.max(0, (props.durationMs ?? 0) / 1000),
          metadata: {
            presentation:
              kind === "video" ? "frontend-muted-video-plane" : "audio",
          },
        },
        startSeconds: forceReload ? 0 : resumePosition,
        volume: clampVolume(volume),
        muted: muted || volume <= 0,
        forceReload,
        previewResumePolicy: "resume_if_previously_playing",
      });
    },
    [
      draftPosition,
      duration,
      kind,
      muted,
      playback.commands,
      props.durationMs,
      props.posterURL,
      sessionID,
      track.author,
      track.coverURL,
      track.id,
      track.path,
      track.previewURL,
      track.title,
      volume,
    ],
  );

  const handleTogglePlayback = React.useCallback<
    React.MouseEventHandler<HTMLButtonElement>
  >(() => {
    if (playing) {
      void playback.commands.pause().catch(() => {});
      return;
    }
    if (sessionIsActive && session?.capabilities.playPause) {
      void playback.commands.play().catch(() => {});
      return;
    }
    void startPreview(session?.state === "ended").catch(() => {});
  }, [playback.commands, playing, session, sessionIsActive, startPreview]);

  const handleSeek = React.useCallback(
    (seconds: number) => {
      const nextPosition = duration > 0
        ? Math.max(0, Math.min(seconds, duration))
        : Math.max(0, seconds);
      setDraftPosition(nextPosition);
      writeLocalPreviewProgress(progressStorageKey, {
        position: nextPosition,
        duration,
        volume,
        muted,
      });
      if (sessionIsActive && session?.capabilities.seek) {
        void playback.commands.seek(nextPosition).catch(() => {});
      }
    }, [
      duration,
      muted,
      playback.commands,
      progressStorageKey,
      session?.capabilities.seek,
      sessionIsActive,
      volume,
    ],
  );

  const handleToggleMute = React.useCallback(() => {
    const nextMuted = !(effectiveMuted || effectiveVolume <= 0);
    const nextVolume = nextMuted
      ? effectiveVolume
      : Math.max(effectiveVolume, lastNonZeroVolumeRef.current);
    setMuted(nextMuted);
    setVolume(nextVolume);
    if (sessionIsActive && session?.capabilities.volume) {
      void playback.commands.setVolume(nextVolume, nextMuted).catch(() => {});
    }
  }, [
    effectiveMuted,
    effectiveVolume,
    playback.commands,
    session?.capabilities.volume,
    sessionIsActive,
  ]);

  const handleVolumeChange = React.useCallback(
    (value: number) => {
      const nextVolume = clampVolume(value);
      const nextMuted = nextVolume <= 0;
      if (nextVolume > 0) {
        lastNonZeroVolumeRef.current = nextVolume;
      }
      setVolume(nextVolume);
      setMuted(nextMuted);
      if (sessionIsActive && session?.capabilities.volume) {
        void playback.commands
          .setVolume(nextVolume, nextMuted)
          .catch(() => {});
      }
    }, [playback.commands, session?.capabilities.volume, sessionIsActive],
  );

  const toggleFullscreen = React.useCallback(() => {
    if (document.fullscreenElement === containerRef.current) {
      void document.exitFullscreen().catch(() => {});
      return;
    }
    const container = containerRef.current;
    if (!container?.requestFullscreen) {
      return;
    }
    void container.requestFullscreen().catch(() => {});
  }, []);

  return (
    <div
      ref={containerRef}
      className={cn(
        "listen-local-preview-player relative flex h-full min-h-[16rem] flex-col overflow-hidden",
        props.className,
      )}
      data-presentation-active={presentationActive ? "true" : undefined}
      data-playback-audio-owner="coordinator"
      data-playback-video-surface={
        kind === "video" ? "muted-mirror" : undefined
      }
    >
      <div
        className={cn(
          "listen-local-preview-player__media flex min-h-0 flex-1 items-center justify-center overflow-hidden",
          kind !== "video" && "p-6 sm:p-8",
        )}
        data-media-kind={kind}
      >
        {kind === "video" ? (
          <div className="group/video relative flex h-full w-full items-center justify-center">
            <video
              ref={videoRef}
              key={track.previewURL}
              src={track.previewURL}
              poster={props.posterURL || track.coverURL}
              title={track.title}
              muted
              playsInline
              preload="metadata"
              className="h-full w-full object-contain"
              onLoadedMetadata={(event) => {
                const mediaDuration = Number(event.currentTarget.duration);
                if (Number.isFinite(mediaDuration) && mediaDuration > 0) {
                  setDraftDuration(mediaDuration);
                }
              }}
            />
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="secondary"
                  size="compactIcon"
                  shape="circle"
                  className="listen-local-preview-player__fullscreen-action absolute right-3 top-3 z-10"
                  data-active={presentationActive ? "true" : "false"}
                  aria-label={
                    presentationActive
                      ? props.text.completed.previewExitFullscreen
                      : props.text.completed.previewEnterFullscreen
                  }
                  onClick={toggleFullscreen}
                >
                  {presentationActive ? (
                    <Minimize2 className="h-4 w-4" />
                  ) : (
                    <Maximize2 className="h-4 w-4" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="left">
                {presentationActive
                  ? props.text.completed.previewExitFullscreen
                  : props.text.completed.previewEnterFullscreen}
              </TooltipContent>
            </Tooltip>
          </div>
        ) : (
          <div className="listen-local-preview-player__identity flex w-full max-w-lg flex-col items-center justify-center">
            <div className="min-w-0">
              <div className="listen-local-preview-player__title overflow-hidden [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
                {track.title}
              </div>
              <div className="listen-local-preview-player__artist mt-3 truncate">
                {author}
              </div>
            </div>
          </div>
        )}
      </div>

      <div className="shrink-0 px-4 pb-4 pt-3">
        <div className="mx-auto flex w-full max-w-2xl flex-col gap-3">
          <ListenProgressBar
            currentTime={progress.currentTime}
            duration={progress.duration}
            bufferedTime={progress.bufferedTime}
            ariaLabel={props.text.listen.seek}
            onSeek={handleSeek}
          />
          <ListenTransportActions
            hasTrack={Boolean(track.path || track.previewURL)}
            playing={playing}
            loading={loading}
            showQueueControls={false}
            muted={effectiveMuted}
            volume={effectiveVolume}
            playMode="order"
            text={props.text}
            onPrevious={() => {}}
            onNext={() => {}}
            onTogglePlayMode={() => {}}
            onTogglePlayback={handleTogglePlayback}
            onToggleMute={handleToggleMute}
            onVolumeChange={handleVolumeChange}
          />
          {session?.state === "error" && session.errorMessage ? (
            <div className="listen-status-text listen-local-preview-player__error truncate" data-tone="error">
              {session.errorMessage}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
