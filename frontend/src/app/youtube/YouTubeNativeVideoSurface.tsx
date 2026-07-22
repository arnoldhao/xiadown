import { LoaderCircle } from "lucide-react";
import * as React from "react";

import { useListenNativeVideoUnderlay } from "@/app/main/listen/native-video-underlay";
import {
  hideYouTubeEmbeddedVideo,
  showYouTubeEmbeddedVideo,
} from "@/app/youtube/api";
import { YouTubeImage } from "@/app/youtube/YouTubeImage";
import { GlassSurface } from "@/shared/ui/glass-surface";

export interface YouTubeNativeVideoSurfaceProps {
  active: boolean;
  geometrySuspended?: boolean;
  videoId: string;
  poster?: string;
  allowRemotePosterCandidates?: boolean;
  loadingLabel: string;
  scrollViewportSelector?: string;
  className?: string;
}

/**
 * Shared renderer for every XiaDown surface that plays a YouTube video.
 * The visible React node only reserves geometry; playback stays in the
 * Keychain-backed native player used by the YouTube station.
 */
export function YouTubeNativeVideoSurface({
  active,
  geometrySuspended = false,
  videoId,
  poster,
  allowRemotePosterCandidates = true,
  loadingLabel,
  scrollViewportSelector = ".youtube-workspace-watch-video-region",
  className,
}: YouTubeNativeVideoSurfaceProps) {
  const surfaceRef = React.useRef<HTMLDivElement | null>(null);
  const requestRef = React.useRef(0);
  const geometrySuspendedRef = React.useRef(geometrySuspended);
  const resumeGeometryRef = React.useRef<() => void>(() => {});
  geometrySuspendedRef.current = geometrySuspended;
  const hasVideo = Boolean(videoId);
  const [shown, setShown] = React.useState(false);
  const { resetHole, setHole } = useListenNativeVideoUnderlay(active);

  React.useLayoutEffect(() => {
    const surface = surfaceRef.current;
    if (!active || !surface || !hasVideo) {
      return;
    }
    document.documentElement.dataset.youtubeWorkspaceVideoActive = "true";
    let disposed = false;
    let pending = false;
    let queued = false;
    let hiddenForGeometry = false;
    let animationFrame = 0;
    let retryTimer = 0;
    const observers: Array<() => void> = [];

    const hide = (force = false) => {
      if (!force && geometrySuspendedRef.current) {
        queued = true;
        return;
      }
      if (!force && hiddenForGeometry) {
        return;
      }
      hiddenForGeometry = true;
      requestRef.current += 1;
      setShown(false);
      resetHole();
      void hideYouTubeEmbeddedVideo(createVideoSequence(requestRef.current)).catch(
        () => {},
      );
    };

    const show = () => {
      if (disposed) {
        return;
      }
      if (geometrySuspendedRef.current) {
        queued = true;
        return;
      }
      if (pending) {
        queued = true;
        return;
      }
      const rect = surface.getBoundingClientRect();
      if (rect.width < 2 || rect.height < 2) {
        hide();
        return;
      }
      const scrollViewport = scrollViewportSelector
        ? surface.closest<HTMLElement>(scrollViewportSelector)
        : null;
      if (
        scrollViewport &&
        !isYouTubeNativeSurfaceRectVisible(
          rect,
          scrollViewport.getBoundingClientRect(),
        )
      ) {
        queued = false;
        hide();
        return;
      }
      pending = true;
      queued = false;
      hiddenForGeometry = false;
      requestRef.current += 1;
      const request = requestRef.current;
      const radius = Number.parseFloat(getComputedStyle(surface).borderRadius) || 0;
      let retryDelay = 0;
      void showYouTubeEmbeddedVideo({
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
        viewportWidth: Math.max(1, window.innerWidth),
        viewportHeight: Math.max(1, window.innerHeight),
        radius,
        // React owns playback and menu controls. Keeping the native surface
        // non-interactive lets Windows place it below the transparent main
        // WebView, where portals can actually paint above the video HWND.
        interactive: false,
        sequence: createVideoSequence(request),
      })
        .then((visible) => {
          if (disposed || requestRef.current !== request) {
            return;
          }
          setShown(Boolean(visible));
          if (visible) {
            setHole(rect, radius);
          } else {
            resetHole();
            retryDelay = 500;
          }
        })
        .catch(() => {
          if (!disposed) {
            setShown(false);
            resetHole();
            retryDelay = 800;
          }
        })
        .finally(() => {
          pending = false;
          if (disposed) {
            return;
          }
          if (queued) {
            queued = false;
            schedule();
            return;
          }
          if (retryDelay > 0) {
            retryTimer = window.setTimeout(show, retryDelay);
          }
        });
    };

    const schedule = () => {
      if (geometrySuspendedRef.current) {
        queued = true;
        return;
      }
      queued = false;
      window.clearTimeout(retryTimer);
      window.cancelAnimationFrame(animationFrame);
      animationFrame = window.requestAnimationFrame(show);
    };
    resumeGeometryRef.current = schedule;
    const resizeObserver = new ResizeObserver(schedule);
    resizeObserver.observe(surface);
    window.addEventListener("resize", schedule);
    window.addEventListener("scroll", schedule, true);
    observers.push(() => resizeObserver.disconnect());
    observers.push(() => window.removeEventListener("resize", schedule));
    observers.push(() => window.removeEventListener("scroll", schedule, true));
    schedule();

    return () => {
      disposed = true;
      resumeGeometryRef.current = () => {};
      window.cancelAnimationFrame(animationFrame);
      window.clearTimeout(retryTimer);
      observers.forEach((dispose) => dispose());
      hide(true);
      delete document.documentElement.dataset.youtubeWorkspaceVideoActive;
    };
  }, [active, hasVideo, resetHole, scrollViewportSelector, setHole]);

  React.useLayoutEffect(() => {
    if (geometrySuspended) {
      requestRef.current += 1;
      return;
    }
    resumeGeometryRef.current();
  }, [geometrySuspended]);

  return (
    <div
      ref={surfaceRef}
      className={["youtube-workspace-video-surface", className]
        .filter(Boolean)
        .join(" ")}
      data-native-visible={shown}
    >
      {allowRemotePosterCandidates ? (
        <YouTubeImage
          source={poster}
          videoId={videoId}
          alt=""
          draggable={false}
        />
      ) : poster ? (
        <img
          src={poster}
          alt=""
          decoding="async"
          draggable={false}
          loading="lazy"
          referrerPolicy="no-referrer"
        />
      ) : null}
      <GlassSurface
        className="youtube-workspace-video-loading"
        elevation="floating"
        shape="capsule"
        surfaceRole="status"
        role="status"
      >
        <LoaderCircle className="h-5 w-5 app-motion-spin" />
        <span>{loadingLabel}</span>
      </GlassSurface>
    </div>
  );
}

function createVideoSequence(request: number) {
  return Date.now() * 1_000 + (request % 1_000);
}

export function isYouTubeNativeSurfaceRectVisible(
  surface: Pick<DOMRect, "top" | "right" | "bottom" | "left">,
  viewport: Pick<DOMRect, "top" | "right" | "bottom" | "left">,
) {
  const tolerance = 1;
  return (
    surface.top >= viewport.top - tolerance &&
    surface.left >= viewport.left - tolerance &&
    surface.right <= viewport.right + tolerance &&
    surface.bottom <= viewport.bottom + tolerance
  );
}
