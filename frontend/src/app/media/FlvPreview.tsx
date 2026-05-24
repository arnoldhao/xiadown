import * as React from "react";
import { Events, Window } from "@wailsio/runtime";
import { Loader2, Maximize, Minimize, VideoOff } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import type { VidstackPreviewLabels } from "@/app/media/VidstackPreview";

type FlvPlayer = {
  attachMediaElement: (mediaElement: HTMLMediaElement) => void;
  detachMediaElement: () => void;
  destroy: () => void;
  load: () => void;
  off: (event: string, listener: (...args: unknown[]) => void) => void;
  on: (event: string, listener: (...args: unknown[]) => void) => void;
  unload: () => void;
};

type FlvModule = {
  Events: {
    ERROR: string;
    MEDIA_INFO: string;
  };
  createPlayer: (
    mediaDataSource: {
      hasAudio?: boolean;
      hasVideo?: boolean;
      isLive?: boolean;
      type: "flv";
      url: string;
    },
    config?: {
      autoCleanupSourceBuffer?: boolean;
      enableStashBuffer?: boolean;
      enableWorker?: boolean;
      isLive?: boolean;
    },
  ) => FlvPlayer;
  isSupported: () => boolean;
};

export type FlvPreviewLabels = VidstackPreviewLabels & {
  loading?: string;
  unsupported?: string;
};

export type FlvPreviewProps = {
  labels: FlvPreviewLabels;
  mediaUrl: string;
  title: string;
  className?: string;
  posterUrl?: string;
  streamType?: "live" | "on-demand";
  onPresentationModeChange?: (active: boolean) => void;
};

type FlvLoadState = "loading" | "ready" | "unavailable";
type FlvFullscreenMode = "dom" | "wails";

function formatFlvError(args: unknown[]) {
  return args
    .map((item) => {
      if (typeof item === "string") {
        return item;
      }
      try {
        return JSON.stringify(item);
      } catch {
        return String(item);
      }
    })
    .filter(Boolean)
    .join(" ");
}

export function FlvPreview(props: FlvPreviewProps) {
  const shellRef = React.useRef<HTMLDivElement | null>(null);
  const videoRef = React.useRef<HTMLVideoElement | null>(null);
  const fullscreenModeRef = React.useRef<FlvFullscreenMode | null>(null);
  const mediaUrl = props.mediaUrl.trim();
  const isLive = props.streamType === "live";
  const [loadState, setLoadState] = React.useState<FlvLoadState>("loading");
  const [errorText, setErrorText] = React.useState("");
  const [screenFullscreen, setScreenFullscreen] = React.useState(false);

  React.useEffect(() => {
    const handleDomFullscreenChange = () => {
      if (fullscreenModeRef.current !== "dom") {
        return;
      }
      const isActive = document.fullscreenElement === shellRef.current;
      setScreenFullscreen(isActive);
      if (!isActive) {
        fullscreenModeRef.current = null;
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
        }
      },
    );
    return () => {
      offWindowFullscreen();
      offWindowUnFullscreen();
    };
  }, []);

  React.useLayoutEffect(() => {
    props.onPresentationModeChange?.(screenFullscreen);
    return () => props.onPresentationModeChange?.(false);
  }, [props.onPresentationModeChange, screenFullscreen]);

  React.useEffect(() => {
    const video = videoRef.current;
    if (!video || !mediaUrl || typeof window === "undefined") {
      setLoadState("unavailable");
      return;
    }

    let disposed = false;
    let player: FlvPlayer | null = null;
    let cleanupListeners = () => {};

    setLoadState("loading");
    setErrorText("");

    void import("flv.js")
      .then((module) => {
        if (disposed) {
          return;
        }
        const flv = module.default as FlvModule;
        if (!flv.isSupported()) {
          setLoadState("unavailable");
          setErrorText(props.labels.unsupported || props.labels.noPreview);
          return;
        }

        const handleReady = () => {
          if (!disposed) {
            setLoadState("ready");
          }
        };
        const handleUnavailable = (...args: unknown[]) => {
          if (!disposed) {
            setLoadState("unavailable");
            setErrorText(formatFlvError(args));
          }
        };

        video.addEventListener("loadedmetadata", handleReady);
        video.addEventListener("canplay", handleReady);
        video.addEventListener("error", handleUnavailable);

        player = flv.createPlayer(
          {
            hasAudio: true,
            hasVideo: true,
            isLive,
            type: "flv",
            url: mediaUrl,
          },
          {
            autoCleanupSourceBuffer: true,
            enableStashBuffer: !isLive,
            enableWorker: false,
            isLive,
          },
        );
        player.on(flv.Events.ERROR, handleUnavailable);
        player.attachMediaElement(video);
        player.load();

        cleanupListeners = () => {
          video.removeEventListener("loadedmetadata", handleReady);
          video.removeEventListener("canplay", handleReady);
          video.removeEventListener("error", handleUnavailable);
          player?.off(flv.Events.ERROR, handleUnavailable);
        };
      })
      .catch((error) => {
        if (!disposed) {
          setLoadState("unavailable");
          setErrorText(error instanceof Error ? error.message : String(error));
        }
      });

    return () => {
      disposed = true;
      cleanupListeners();
      if (player) {
        player.unload();
        player.detachMediaElement();
        player.destroy();
      }
      video.removeAttribute("src");
      video.load();
    };
  }, [isLive, mediaUrl, props.labels.noPreview, props.labels.unsupported]);

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
      }
      return;
    }
    setScreenFullscreen(false);
  }, []);

  const toggleScreenFullscreen = React.useCallback(() => {
    if (screenFullscreen) {
      void exitScreenFullscreen().catch(() => {
        fullscreenModeRef.current = null;
        setScreenFullscreen(false);
      });
      return;
    }

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
      });
  }, [exitScreenFullscreen, screenFullscreen]);

  const unavailableText =
    errorText.trim() ||
    props.labels.previewPlaybackStalled ||
    props.labels.noPreview;
  const fullscreenLabel = screenFullscreen
    ? props.labels.previewExitFullscreen
    : props.labels.previewEnterFullscreen;
  const fullscreenDisabled = !mediaUrl || loadState !== "ready";

  return (
    <div
      ref={shellRef}
      className={cn(
        "relative flex h-full min-h-0 w-full min-w-0 items-center justify-center overflow-hidden bg-black",
        screenFullscreen && "fixed inset-0 z-[200] rounded-none",
        props.className,
      )}
    >
      <video
        ref={videoRef}
        className="h-full w-full bg-black object-contain"
        controls
        playsInline
        poster={props.posterUrl}
        title={props.title}
      />
      <Button
        type="button"
        variant="ghost"
        size="compactIcon"
        className="absolute right-2 top-1/2 z-20 h-9 w-9 -translate-y-1/2 rounded-full !bg-black/55 !text-white shadow-lg backdrop-blur hover:!bg-black/75 hover:!text-white focus-visible:!bg-black/75 focus-visible:!text-white"
        aria-label={fullscreenLabel}
        title={fullscreenLabel}
        disabled={fullscreenDisabled}
        onClick={toggleScreenFullscreen}
      >
        {screenFullscreen ? (
          <Minimize className="h-4 w-4" />
        ) : (
          <Maximize className="h-4 w-4" />
        )}
      </Button>
      {loadState === "loading" ? (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/35 text-xs font-medium text-white">
          <span className="inline-flex items-center gap-2 rounded-md bg-black/55 px-3 py-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            {props.labels.loading || props.labels.noPreview}
          </span>
        </div>
      ) : null}
      {loadState === "unavailable" ? (
        <div className="absolute inset-0 flex items-center justify-center bg-black/70 px-6 text-center text-xs font-medium text-white">
          <span className="inline-flex max-w-full items-center gap-2 rounded-md bg-black/55 px-3 py-2">
            <VideoOff className="h-4 w-4 shrink-0" />
            <span className="min-w-0 truncate">{unavailableText}</span>
          </span>
        </div>
      ) : null}
    </div>
  );
}
