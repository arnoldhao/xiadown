import * as React from "react";
import { Events, Window } from "@wailsio/runtime";
import { MediaPlayer, MediaProvider, type VideoSrc } from "@vidstack/react";
import {
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
  posterUrl?: string;
  durationMs?: number;
};

type MediaPlayerElement = React.ElementRef<typeof MediaPlayer>;
type PreviewFullscreenMode = "dom" | "wails";

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

export function CompletedVidstackPreview(props: CompletedVidstackPreviewProps) {
  const shellRef = React.useRef<HTMLDivElement | null>(null);
  const fullscreenModeRef = React.useRef<PreviewFullscreenMode | null>(null);
  const previousWindowedFullscreenRef = React.useRef(false);
  const lastNonZeroVolumeRef = React.useRef(1);
  const animationFrameRef = React.useRef<number>();
  const [playerElement, setPlayerElement] =
    React.useState<MediaPlayerElement | null>(null);
  const [videoElement, setVideoElement] =
    React.useState<HTMLVideoElement | null>(null);
  const [currentTimeMs, setCurrentTimeMs] = React.useState(0);
  const [resolvedDurationMs, setResolvedDurationMs] = React.useState(() =>
    Math.max(0, props.durationMs ?? 0),
  );
  const [isPlaying, setIsPlaying] = React.useState(false);
  const [volume, setVolume] = React.useState(1);
  const [muted, setMuted] = React.useState(false);
  const [windowedFullscreen, setWindowedFullscreen] = React.useState(false);
  const [screenFullscreen, setScreenFullscreen] = React.useState(false);

  const effectiveDurationMs = Math.max(resolvedDurationMs, 0);
  const playerSource = React.useMemo(
    () => resolveVideoSource(props.mediaUrl, props.title),
    [props.mediaUrl, props.title],
  );

  const syncCurrentTime = React.useCallback(() => {
    const activeMedia = videoElement ?? playerElement;
    const next = activeMedia ? Number(activeMedia.currentTime || 0) * 1000 : 0;
    setCurrentTimeMs((current) => {
      const normalized = clampMs(next, effectiveDurationMs || next || 0);
      return Math.abs(current - normalized) < 50 ? current : normalized;
    });
  }, [effectiveDurationMs, playerElement, videoElement]);

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
    setVideoElement(null);
  }, [props.durationMs, props.mediaUrl]);

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
    const playerHost = player.el;
    if (!(playerHost instanceof HTMLElement)) {
      return;
    }

    const syncVideoNode = () => {
      const nextVideo = playerHost.querySelector("video");
      if (!(nextVideo instanceof HTMLVideoElement)) {
        return false;
      }
      setVideoElement((current) =>
        current === nextVideo ? current : nextVideo,
      );
      return true;
    };

    if (syncVideoNode()) {
      return;
    }

    const observer = new MutationObserver(() => {
      if (syncVideoNode()) {
        observer.disconnect();
      }
    });
    observer.observe(playerHost, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, [playerElement, props.mediaUrl]);

  const handleLoadedMetadata = React.useCallback(
    (event: React.SyntheticEvent<HTMLMediaElement>) => {
      const media = event.currentTarget;
      if (media instanceof HTMLVideoElement) {
        setVideoElement(media);
      }
      const nextDurationMs = Number.isFinite(media.duration)
        ? Math.round(media.duration * 1000)
        : 0;
      if (nextDurationMs > 0) {
        setResolvedDurationMs((current) => Math.max(current, nextDurationMs));
      }
    },
    [],
  );

  React.useEffect(() => {
    const media = videoElement ?? playerElement;
    if (!media) {
      return;
    }

    const handlePlay = () => setIsPlaying(true);
    const handlePause = () => setIsPlaying(false);
    const handleEnded = () => {
      setIsPlaying(false);
      syncCurrentTime();
    };
    const handleVolumeChange = () => {
      const nextVolume = clampVolume(Number(media.volume ?? 1));
      setVolume(nextVolume);
      setMuted(Boolean(media.muted));
      if (nextVolume > 0) {
        lastNonZeroVolumeRef.current = nextVolume;
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

    media.addEventListener("play", handlePlay);
    media.addEventListener("pause", handlePause);
    media.addEventListener("ended", handleEnded);
    media.addEventListener("volumechange", handleVolumeChange);
    media.addEventListener("durationchange", handleDurationChange);
    media.addEventListener("timeupdate", syncCurrentTime);

    return () => {
      media.removeEventListener("play", handlePlay);
      media.removeEventListener("pause", handlePause);
      media.removeEventListener("ended", handleEnded);
      media.removeEventListener("volumechange", handleVolumeChange);
      media.removeEventListener("durationchange", handleDurationChange);
      media.removeEventListener("timeupdate", syncCurrentTime);
    };
  }, [playerElement, syncCurrentTime, videoElement]);

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

  const togglePlay = async () => {
    const activeMedia = videoElement;
    if (!activeMedia) {
      return;
    }
    if (isPlaying) {
      activeMedia.pause();
      return;
    }
    try {
      await Promise.resolve(activeMedia.play());
    } catch {
      // Ignore autoplay or playback failures and leave the player idle.
    }
  };

  const handleSeek = (value: number) => {
    const activeMedia = videoElement;
    if (!activeMedia) {
      return;
    }
    const next = clampMs(value, effectiveDurationMs);
    activeMedia.currentTime = next / 1000;
    setCurrentTimeMs(next);
  };

  const toggleMute = () => {
    const activeMedia = videoElement;
    if (!activeMedia) {
      return;
    }
    if (muted || volume <= 0) {
      const restoredVolume = volume > 0 ? volume : lastNonZeroVolumeRef.current;
      activeMedia.volume = restoredVolume;
      activeMedia.muted = false;
      setVolume(restoredVolume);
      setMuted(false);
      return;
    }
    lastNonZeroVolumeRef.current = volume;
    activeMedia.muted = true;
    setMuted(true);
  };

  const handleVolumeChange = (value: number) => {
    const activeMedia = videoElement;
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
              ref={handlePlayerRef}
              src={playerSource}
              controls={false}
              crossOrigin="anonymous"
              playsInline
              preload="metadata"
              style={{ aspectRatio: "auto" }}
              className="h-full w-full bg-black"
            >
              <MediaProvider
                className="h-full w-full overflow-hidden bg-black"
                mediaProps={{
                  className: "h-full w-full object-contain object-center",
                  onLoadedMetadata: handleLoadedMetadata,
                }}
              />
            </MediaPlayer>
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
            onClick={() => void togglePlay()}
            aria-label={playLabel}
            title={playLabel}
            disabled={!videoElement}
          >
            {isPlaying ? (
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
