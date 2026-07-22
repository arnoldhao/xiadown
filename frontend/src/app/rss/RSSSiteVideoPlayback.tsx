import { System } from "@wailsio/runtime";
import { LoaderCircle, Play } from "lucide-react";
import * as React from "react";

import { useListenNativeVideoUnderlay } from "@/app/main/listen/native-video-underlay";
import type { RSSEntry } from "@/app/rss/types";
import { YouTubeWorkspaceTransportBar } from "@/app/youtube/YouTubeWorkspaceTransportBar";
import { getXiaText } from "@/features/xiadown/shared";
import { useI18n } from "@/shared/i18n";
import { GlassSurface } from "@/shared/ui/glass-surface";

import {
  acceptRSSSiteVideoPrepare,
  cancelRSSSiteVideoPrepare,
  closeRSSSiteVideo,
  hideRSSSiteVideo,
  prepareRSSSiteVideo,
  showRSSSiteVideo,
  type RSSSitePlaybackDescriptor,
} from "./api";
import { RSSSitePrepareLifecycle } from "./site-player-lifecycle";
import { nextRSSSiteVideoSurfaceSequence } from "./site-video-surface-sequence";
import type { RSSVideoExperience } from "./video-platform";
import { createRSSWebVideoTransportPlayback } from "./video-transport";

const RSS_VIDEO_SCROLL_VIEWPORT_SELECTOR =
  ".rss-video-watch-page, .rss-focused-entry__content, .rss-entry-detail-pane";
const RSS_NATIVE_OVERLAY_BLOCKER_SELECTOR = [
  '[role="dialog"][data-state="open"]',
  '[role="alertdialog"][data-state="open"]',
  '[role="menu"][data-state="open"]',
  '[role="listbox"][data-state="open"]',
  ".app-secondary-reveal__positioner",
].join(", ");
const RSS_SITE_NATIVE_SHOW_RETRY_LIMIT = 4;

export interface RSSSiteVideoPlaybackProps {
  active: boolean;
  entry: RSSEntry;
  experience: RSSVideoExperience;
  geometrySuspended?: boolean;
  onDownload?: () => void;
}

/**
 * Hosts unoptimized video sites in a dedicated, permission-restricted native
 * page. The page is revealed through React's native-video aperture but keeps
 * pointer input so each site's own media controls remain usable.
 */
export function RSSSiteVideoPlayback({
  active,
  entry,
  experience,
  geometrySuspended = false,
  onDownload,
}: RSSSiteVideoPlaybackProps) {
  const { language, t } = useI18n();
  const text = React.useMemo(() => getXiaText(language), [language]);
  const surfaceRef = React.useRef<HTMLDivElement | null>(null);
  const prepareLifecycleRef = React.useRef<RSSSitePrepareLifecycle | null>(null);
  const loadErrorLabelRef = React.useRef("");
  const [descriptor, setDescriptor] =
    React.useState<RSSSitePlaybackDescriptor | null>(null);
  const [nativeVisible, setNativeVisible] = React.useState(false);
  const [loadError, setLoadError] = React.useState("");
  const portalOverlayOpen = useRSSPortalOverlayOpen();
  const nativeUnderlayActive = active && System.IsWindows();
  const { resetHole, setHole } = useListenNativeVideoUnderlay(
    nativeUnderlayActive,
  );
  loadErrorLabelRef.current = t("xiadown.rss.noPlayableVideo");
  if (!prepareLifecycleRef.current) {
    prepareLifecycleRef.current = new RSSSitePrepareLifecycle();
  }

  const pageURL = experience.mode === "site"
    ? experience.playbackUrl?.trim() || ""
    : "";

  React.useLayoutEffect(() => {
    if (!nativeUnderlayActive || !nativeVisible) {
      delete document.documentElement.dataset.rssSiteVideoActive;
      return;
    }
    document.documentElement.dataset.rssSiteVideoActive = "true";
    return () => {
      delete document.documentElement.dataset.rssSiteVideoActive;
    };
  }, [nativeUnderlayActive, nativeVisible]);

  React.useEffect(() => {
    const lifecycle = prepareLifecycleRef.current;
    if (!active || !pageURL || !lifecycle) return;
    const prepareToken = lifecycle.begin();
    let disposed = false;
    let sessionID = "";
    setDescriptor(null);
    setNativeVisible(false);
    setLoadError("");

    void (async () => {
      try {
        const nextDescriptor = await prepareRSSSiteVideo({
          requestId: prepareToken.requestId,
          url: pageURL,
        });
        sessionID = nextDescriptor.sessionId?.trim() || "";
        if (!sessionID) {
          throw new Error(loadErrorLabelRef.current);
        }
        if (!lifecycle.isCurrent(prepareToken) || disposed) {
          if (lifecycle.cancel(prepareToken)) {
            await cancelRSSSiteVideoPrepare(prepareToken.requestId).catch(() => {});
          }
          if (sessionID) await closeRSSSiteVideo(sessionID).catch(() => {});
          return;
        }
        await acceptRSSSiteVideoPrepare(prepareToken.requestId);
        const settlement = lifecycle.settle(prepareToken);
        if (!settlement.current || disposed) {
          await closeRSSSiteVideo(sessionID).catch(() => {});
          return;
        }
        setDescriptor(nextDescriptor);
      } catch {
        if (lifecycle.cancel(prepareToken)) {
          await cancelRSSSiteVideoPrepare(prepareToken.requestId).catch(() => {});
        }
        if (sessionID) await closeRSSSiteVideo(sessionID).catch(() => {});
        if (!disposed) {
          setLoadError(loadErrorLabelRef.current);
        }
      }
    })();

    return () => {
      disposed = true;
      setNativeVisible(false);
      if (lifecycle.cancel(prepareToken)) {
        void cancelRSSSiteVideoPrepare(prepareToken.requestId).catch(() => {});
      }
      if (sessionID) void closeRSSSiteVideo(sessionID).catch(() => {});
    };
  }, [active, pageURL]);

  React.useLayoutEffect(() => {
    const surface = surfaceRef.current;
    const sessionID = descriptor?.sessionId?.trim() || "";
    if (
      !active ||
      !surface ||
      !sessionID ||
      loadError ||
      geometrySuspended ||
      portalOverlayOpen
    ) return;
    let disposed = false;
    let pending = false;
    let queued = false;
    let hiddenForGeometry = false;
    let animationFrame = 0;
    let retryTimer = 0;
    let showFailures = 0;

    const hide = () => {
      if (hiddenForGeometry) return;
      hiddenForGeometry = true;
      setNativeVisible(false);
      resetHole();
      void hideRSSSiteVideo(
        sessionID,
        nextRSSSiteVideoSurfaceSequence(),
      ).catch(() => {});
    };

    const show = () => {
      if (disposed || pending) {
        queued = !disposed;
        return;
      }
      if (document.querySelector(RSS_NATIVE_OVERLAY_BLOCKER_SELECTOR)) {
        hide();
        return;
      }
      const rect = surface.getBoundingClientRect();
      const viewport = surface.closest<HTMLElement>(RSS_VIDEO_SCROLL_VIEWPORT_SELECTOR);
      if (
        rect.width < 2 ||
        rect.height < 2 ||
        !surfaceRectFullyVisible(rect, new DOMRect(
          0,
          0,
          Math.max(1, window.innerWidth),
          Math.max(1, window.innerHeight),
        )) ||
        (viewport && !surfaceRectFullyVisible(rect, viewport.getBoundingClientRect()))
      ) {
        hide();
        return;
      }
      pending = true;
      queued = false;
      hiddenForGeometry = false;
      const radius = Number.parseFloat(getComputedStyle(surface).borderRadius) || 0;
      void showRSSSiteVideo(sessionID, {
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
        viewportWidth: Math.max(1, window.innerWidth),
        viewportHeight: Math.max(1, window.innerHeight),
        radius,
        interactive: true,
        sequence: nextRSSSiteVideoSurfaceSequence(),
      })
        .then((visible) => {
          if (disposed) return;
          setNativeVisible(Boolean(visible));
          if (visible) {
            showFailures = 0;
            setHole(rect, radius);
            return;
          }
          resetHole();
          showFailures += 1;
          if (showFailures < RSS_SITE_NATIVE_SHOW_RETRY_LIMIT) {
            retryTimer = window.setTimeout(show, 500);
            return;
          }
          setLoadError(loadErrorLabelRef.current);
          void closeRSSSiteVideo(sessionID).catch(() => {});
        })
        .catch(() => {
          if (!disposed) {
            setNativeVisible(false);
            resetHole();
            showFailures += 1;
            if (showFailures < RSS_SITE_NATIVE_SHOW_RETRY_LIMIT) {
              retryTimer = window.setTimeout(show, 800);
              return;
            }
            setLoadError(loadErrorLabelRef.current);
            void closeRSSSiteVideo(sessionID).catch(() => {});
          }
        })
        .finally(() => {
          pending = false;
          if (disposed) return;
          if (queued) schedule();
        });
    };

    const schedule = () => {
      queued = false;
      window.clearTimeout(retryTimer);
      window.cancelAnimationFrame(animationFrame);
      animationFrame = window.requestAnimationFrame(show);
    };
    const resizeObserver = new ResizeObserver(schedule);
    resizeObserver.observe(surface);
    window.addEventListener("resize", schedule);
    window.addEventListener("scroll", schedule, true);
    schedule();
    return () => {
      disposed = true;
      window.clearTimeout(retryTimer);
      window.cancelAnimationFrame(animationFrame);
      resizeObserver.disconnect();
      window.removeEventListener("resize", schedule);
      window.removeEventListener("scroll", schedule, true);
      setNativeVisible(false);
      resetHole();
      void hideRSSSiteVideo(
        sessionID,
        nextRSSSiteVideoSurfaceSequence(),
      ).catch(() => {});
    };
  }, [
    active,
    descriptor,
    geometrySuspended,
    loadError,
    portalOverlayOpen,
    resetHole,
    setHole,
  ]);

  const disabledPlayback = createRSSWebVideoTransportPlayback({
    direct: false,
    title: entry.title,
    state: loadError ? "error" : "loading",
    currentTime: 0,
    duration: 0,
    volume: 1,
    muted: false,
    playbackRate: 1,
    fullscreenAvailable: false,
  });
  // React 18 needs the empty-string form for the native inert attribute.
  const inertAttribute = { inert: "" } as React.HTMLAttributes<HTMLDivElement>;
  const noop = () => {};

  return (
    <div className="rss-site-video-playback">
      <div className="youtube-workspace-watch-video-region">
        <div className="youtube-workspace-watch-player-shell">
          <div className="youtube-workspace-player-card">
            <div
              className="youtube-workspace-video-surface rss-site-video-surface"
              data-native-visible={nativeVisible ? "true" : "false"}
              ref={surfaceRef}
            >
              <div className="rss-video-player__empty" role="status">
                {loadError ? <Play aria-hidden="true" /> : (
                  <LoaderCircle aria-hidden="true" className="app-motion-spin" />
                )}
                <span>{loadError || text.listen.loadingStatus}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div className="rss-site-video-transport-shell">
        <div
          aria-hidden="true"
          className="rss-site-video-transport"
          {...inertAttribute}
        >
          <YouTubeWorkspaceTransportBar
            labels={{
              player: text.listen.nowPlaying,
              previous: text.listen.previous,
              play: text.listen.play,
              pause: text.listen.pause,
              next: text.listen.next,
              fullscreen: text.completed.previewEnterFullscreen,
              exitFullscreen: text.completed.previewExitFullscreen,
              captions: text.dialogs.subtitles,
              audioTrack: text.dialogs.audioTrack,
              quality: text.dialogs.quality,
              danmaku: text.dialogs.danmaku,
              playbackSpeed: text.youtube.playbackSpeed,
              volume: text.listen.volume,
              mute: text.listen.mute,
              unmute: text.listen.unmute,
              download: text.actions.download,
              upNext: text.listen.upNext,
              unavailable: text.youtube.errors.controlUnavailable,
              off: text.settings.equalizer.status.off,
            }}
            playback={disabledPlayback}
            onPrevious={noop}
            onTogglePlayback={noop}
            onNext={noop}
            onDownload={onDownload}
            onFullscreen={noop}
            onToggleMute={noop}
            onToggleCaptions={noop}
            onSelectCaption={noop}
            onSelectAudioTrack={noop}
            onSelectQuality={noop}
            onSelectPlaybackRate={noop}
            onVolumeChange={noop}
            onSeek={noop}
          />
        </div>
        <GlassSurface
          className="rss-site-video-transport-notice"
          elevation="floating"
          shape="control"
          surfaceRole="status"
          role="status"
        >
          <span>{t("xiadown.rss.sitePlaybackUnoptimized")}</span>
        </GlassSurface>
      </div>
    </div>
  );
}

function surfaceRectFullyVisible(surface: DOMRect, viewport: DOMRect) {
  const tolerance = 1;
  return surface.left >= viewport.left - tolerance &&
    surface.top >= viewport.top - tolerance &&
    surface.right <= viewport.right + tolerance &&
    surface.bottom <= viewport.bottom + tolerance;
}

function useRSSPortalOverlayOpen() {
  const [open, setOpen] = React.useState(false);
  React.useLayoutEffect(() => {
    if (typeof MutationObserver === "undefined") return;
    let animationFrame = 0;
    const readOpen = () => Boolean(
      document.querySelector(RSS_NATIVE_OVERLAY_BLOCKER_SELECTOR),
    );
    const update = () => {
      animationFrame = 0;
      setOpen(readOpen());
    };
    const schedule = () => {
      if (readOpen()) {
        window.cancelAnimationFrame(animationFrame);
        animationFrame = 0;
        setOpen(true);
        return;
      }
      if (animationFrame) return;
      animationFrame = window.requestAnimationFrame(update);
    };
    const observer = new MutationObserver(schedule);
    observer.observe(document.body, {
      subtree: true,
      childList: true,
      attributes: true,
      attributeFilter: ["data-state", "role"],
    });
    update();
    return () => {
      observer.disconnect();
      window.cancelAnimationFrame(animationFrame);
    };
  }, []);
  return open;
}
