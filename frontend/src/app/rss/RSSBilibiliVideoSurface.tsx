import { LoaderCircle } from "lucide-react";
import * as React from "react";

import { useListenNativeVideoUnderlay } from "@/app/main/listen/native-video-underlay";
import { GlassSurface } from "@/shared/ui/glass-surface";
import {
  hideRSSBilibiliVideo,
  showRSSBilibiliVideo,
} from "./api";
import { nextRSSBilibiliVideoSurfaceSequence } from "./bilibili-video-surface-sequence";
import { RSSRemoteImage } from "./RSSRemoteImage";

export interface RSSBilibiliVideoSurfaceProps {
  active: boolean;
  geometrySuspended?: boolean;
  loading?: boolean;
  loadingLabel: string;
  posterSources: readonly string[];
  scrollViewportSelector?: string;
}

/**
 * Geometry-only React surface for the dedicated native Bilibili player.
 * Session lifecycle and media controls live in RSSBilibiliPlayback. The full
 * Bilibili site page stays behind this video-only surface and never becomes a
 * second, competing React transport.
 */
export function RSSBilibiliVideoSurface({
  active,
  geometrySuspended = false,
  loading = true,
  loadingLabel,
  posterSources,
  scrollViewportSelector = ".rss-video-watch-page",
}: RSSBilibiliVideoSurfaceProps) {
  const surfaceRef = React.useRef<HTMLDivElement | null>(null);
  const requestRef = React.useRef(0);
  const geometrySuspendedRef = React.useRef(geometrySuspended);
  const resumeGeometryRef = React.useRef<() => void>(() => {});
  geometrySuspendedRef.current = geometrySuspended;
  const [shown, setShown] = React.useState(false);
  const { resetHole, setHole } = useListenNativeVideoUnderlay(active);

  React.useLayoutEffect(() => {
    if (!active || !shown) {
      delete document.documentElement.dataset.rssBilibiliVideoActive;
      return;
    }
    document.documentElement.dataset.rssBilibiliVideoActive = "true";
    return () => {
      delete document.documentElement.dataset.rssBilibiliVideoActive;
    };
  }, [active, shown]);

  React.useLayoutEffect(() => {
    const surface = surfaceRef.current;
    if (!active || !surface) return;
    let disposed = false;
    let pending = false;
    let queued = false;
    let hiddenForGeometry = false;
    let animationFrame = 0;
    let retryTimer = 0;

    const hide = (force = false) => {
      if (!force && geometrySuspendedRef.current) {
        queued = true;
        return;
      }
      if (!force && hiddenForGeometry) return;
      hiddenForGeometry = true;
      requestRef.current += 1;
      setShown(false);
      resetHole();
      void hideRSSBilibiliVideo(nextRSSBilibiliVideoSurfaceSequence()).catch(() => {});
    };

    const show = () => {
      if (disposed) return;
      if (geometrySuspendedRef.current) {
        queued = true;
        return;
      }
      if (pending) {
        queued = true;
        return;
      }
      const rect = surface.getBoundingClientRect();
      const viewport = scrollViewportSelector
        ? surface.closest<HTMLElement>(scrollViewportSelector)
        : null;
      if (
        rect.width < 2 ||
        rect.height < 2 ||
        (viewport && !surfaceRectVisible(rect, viewport.getBoundingClientRect()))
      ) {
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
      void showRSSBilibiliVideo({
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
        viewportWidth: Math.max(1, window.innerWidth),
        viewportHeight: Math.max(1, window.innerHeight),
        radius,
        interactive: false,
        sequence: nextRSSBilibiliVideoSurfaceSequence(),
      })
        .then((visible) => {
          if (disposed || requestRef.current !== request) return;
          setShown(Boolean(visible));
          if (visible) setHole(rect, radius);
          else {
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
          if (disposed) return;
          if (queued) {
            queued = false;
            schedule();
          } else if (retryDelay > 0) {
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
    schedule();
    return () => {
      disposed = true;
      resumeGeometryRef.current = () => {};
      window.clearTimeout(retryTimer);
      window.cancelAnimationFrame(animationFrame);
      resizeObserver.disconnect();
      window.removeEventListener("resize", schedule);
      window.removeEventListener("scroll", schedule, true);
      hide(true);
      delete document.documentElement.dataset.rssBilibiliVideoActive;
    };
  }, [active, resetHole, scrollViewportSelector, setHole]);

  React.useLayoutEffect(() => {
    if (geometrySuspended) {
      requestRef.current += 1;
      return;
    }
    resumeGeometryRef.current();
  }, [geometrySuspended]);

  return (
    <div
      className="youtube-workspace-video-surface rss-bilibili-video-surface"
      data-native-visible={shown}
      ref={surfaceRef}
    >
      <RSSRemoteImage alt="" sources={posterSources} />
      {loading ? (
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
      ) : null}
    </div>
  );
}

export function surfaceRectVisible(
  surface: Pick<DOMRect, "top" | "right" | "bottom" | "left">,
  viewport: Pick<DOMRect, "top" | "right" | "bottom" | "left">,
) {
  const tolerance = 1;
  return surface.top >= viewport.top - tolerance &&
    surface.left >= viewport.left - tolerance &&
    surface.right <= viewport.right + tolerance &&
    surface.bottom <= viewport.bottom + tolerance;
}
