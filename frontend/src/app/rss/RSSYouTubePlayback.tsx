import * as React from "react";

import type { RSSEntry } from "@/app/rss/types";
import { shouldAcceptRSSResumedPlaybackProgress } from "@/app/rss/workspace-utils";
import {
  acceptYouTubeWorkspacePlay,
  cancelYouTubeWorkspacePlay,
  getYouTubeWorkspacePlayerStatus,
  isYouTubePlayerStatusForSession,
  pauseYouTubeWorkspaceVideo,
  playYouTubeWorkspaceVideo,
  requestYouTubeEmbeddedVideoFullscreen,
  resumeYouTubeWorkspaceVideo,
  seekYouTubeWorkspaceVideo,
  selectYouTubeWorkspaceAudioTrack,
  selectYouTubeWorkspaceCaption,
  selectYouTubeWorkspacePlaybackRate,
  selectYouTubeWorkspaceQuality,
  setYouTubeWorkspaceVolume,
  subscribeYouTubeEmbeddedVideoFullscreen,
  subscribeYouTubePlayerStatus,
  toggleYouTubeWorkspaceCaptions,
} from "@/app/youtube/api";
import { YouTubeNativeVideoSurface } from "@/app/youtube/YouTubeNativeVideoSurface";
import { resolveYouTubeVolumeCapability } from "@/app/youtube/page-state";
import type {
  YouTubePlaybackDescriptor,
  YouTubePlayerStatus,
  YouTubeWorkspacePlaybackState,
  YouTubeWorkspaceVideo,
} from "@/app/youtube/types";
import { YouTubeWorkspaceTransportBar } from "@/app/youtube/YouTubeWorkspaceTransportBar";
import { getXiaText } from "@/features/xiadown/shared";
import { useI18n } from "@/shared/i18n";
import { GlassSurface } from "@/shared/ui/glass-surface";

import { controlledRSSResourceURL } from "./remote-resource";
import {
  canonicalRSSVideoTarget,
  normalizedYouTubeVideoID,
  siteKeyForRSSVideo,
} from "./video-platform";

export interface RSSYouTubePlaybackProps {
  active: boolean;
  entry: RSSEntry;
  onDownload: () => void;
  onProgress?: (currentTime: number, duration: number) => void;
}

const RSS_VIDEO_SCROLL_VIEWPORT_SELECTOR =
  ".rss-video-watch-page, .rss-focused-entry__content, .rss-entry-detail-pane";

export interface RSSYouTubeProgressBarrier {
  sessionId: string;
  resumeAt: number;
  expiresAt: number;
}

interface RSSYouTubeProgressSnapshot {
  sessionId: string;
  currentTime: number;
  duration: number;
}

/** Uses the same native session, surface and transport as the YouTube station. */
export function RSSYouTubePlayback({
  active,
  entry,
  onDownload,
  onProgress,
}: RSSYouTubePlaybackProps) {
  const { language } = useI18n();
  const text = React.useMemo(() => getXiaText(language), [language]);
  const [playback, setPlayback] = React.useState<YouTubePlaybackDescriptor | null>(null);
  const [status, setStatus] = React.useState<YouTubePlayerStatus>({
    provider: "youtube",
    available: false,
    videoId: entry.platformVideoId,
    state: "loading",
    title: entry.title,
    thumbnailUrl: entry.thumbnailUrl,
    currentTime: 0,
  });
  const [volume, setVolume] = React.useState(1);
  const [muted, setMuted] = React.useState(false);
  const [fullscreen, setFullscreen] = React.useState(false);
  const [controlsReady, setControlsReady] = React.useState(false);
  const progressRef = React.useRef({ second: -1, reportedAt: 0 });
  const statusRef = React.useRef(status);
  const onProgressRef = React.useRef(onProgress);
  const progressSessionRef = React.useRef("");
  const progressSnapshotRef = React.useRef<RSSYouTubeProgressSnapshot | null>(null);
  const resumeBarrierRef = React.useRef<RSSYouTubeProgressBarrier | null>(null);

  statusRef.current = status;
  onProgressRef.current = onProgress;

  const video = React.useMemo(
    () => rssEntryToYouTubeVideo(entry),
    [
      entry.author,
      entry.downloadTarget,
      entry.mediaUrl,
      entry.platform,
      entry.platformVideoId,
      entry.playbackUrl,
      entry.thumbnailUrl,
      entry.title,
      entry.url,
    ],
  );

  React.useEffect(() => {
    if (!active || !video) {
      return;
    }
    let disposed = false;
    let descriptorSessionID = "";
    let accepted = false;
    let initialized = false;
    const requestID = createRSSYouTubeRequestID();
    const resumeAt = entry.videoCompleted
      ? 0
      : Math.max(0, entry.videoProgressSeconds || 0);
    const savedDuration = Math.max(0, entry.videoDurationSeconds || 0);
    setPlayback(null);
    setControlsReady(false);
    setFullscreen(false);
    progressSessionRef.current = "";
    progressSnapshotRef.current = null;
    resumeBarrierRef.current = null;
    progressRef.current = { second: Math.floor(resumeAt), reportedAt: Date.now() };
    const loadingStatus: YouTubePlayerStatus = {
      provider: "youtube",
      available: false,
      videoId: video.videoId,
      state: "loading",
      title: video.title,
      artist: video.channel,
      thumbnailUrl: video.thumbnailUrl,
      duration: video.durationSeconds,
      currentTime: 0,
    };
    statusRef.current = loadingStatus;
    setStatus(loadingStatus);
    void (async () => {
      try {
        const descriptor = await playYouTubeWorkspaceVideo(video, requestID);
        if (disposed) {
          await cancelYouTubeWorkspacePlay(requestID).catch(() => {});
          return;
        }
        descriptorSessionID = descriptor.sessionId;
        setPlayback(descriptor);
        const descriptorStatus: YouTubePlayerStatus = {
          ...statusRef.current,
          provider: "youtube",
          sessionId: descriptor.sessionId,
          available: true,
          videoId: descriptor.videoId,
          title: descriptor.title,
          artist: descriptor.artist,
          thumbnailUrl: descriptor.thumbnailUrl,
          duration: descriptor.durationSeconds,
        };
        statusRef.current = descriptorStatus;
        setStatus(descriptorStatus);

        await acceptYouTubeWorkspacePlay(requestID);
        accepted = true;
        if (disposed) {
          await pauseYouTubeWorkspaceVideo(descriptor.sessionId).catch(() => {});
          return;
        }

        if (resumeAt > 1) {
          try {
            await seekYouTubeWorkspaceVideo(descriptor.sessionId, resumeAt);
          } catch {
            // Resume is best-effort. The progress barrier below preserves the
            // saved position until the player reports a credible live value.
          }
        }
        if (disposed) {
          await pauseYouTubeWorkspaceVideo(descriptor.sessionId).catch(() => {});
          return;
        }

        const duration = Math.max(
          0,
          descriptor.durationSeconds || statusRef.current.duration || savedDuration,
        );
        const resumedStatus: YouTubePlayerStatus = {
          ...statusRef.current,
          provider: "youtube",
          sessionId: descriptor.sessionId,
          available: true,
          currentTime: resumeAt,
          ...(duration > 0 ? { duration } : {}),
        };
        statusRef.current = resumedStatus;
        setStatus(resumedStatus);
        progressSessionRef.current = descriptor.sessionId;
        progressSnapshotRef.current = {
          sessionId: descriptor.sessionId,
          currentTime: resumeAt,
          duration,
        };
        resumeBarrierRef.current = resumeAt > 1
          ? {
              sessionId: descriptor.sessionId,
              resumeAt,
              expiresAt: Date.now() + 10_000,
            }
          : null;
        initialized = true;
        setControlsReady(true);
      } catch (reason) {
        if (!accepted) {
          await cancelYouTubeWorkspacePlay(requestID).catch(() => {});
        }
        if (!disposed) {
          setPlayback(null);
          setControlsReady(false);
          const errorStatus: YouTubePlayerStatus = {
            ...statusRef.current,
            state: "error",
            errorMessage: readPlaybackError(reason),
          };
          statusRef.current = errorStatus;
          setStatus(errorStatus);
        }
      }
    })();
    return () => {
      disposed = true;
      const snapshot = progressSnapshotRef.current;
      if (
        initialized &&
        snapshot?.sessionId === descriptorSessionID &&
        snapshot.duration > 0
      ) {
        onProgressRef.current?.(snapshot.currentTime, snapshot.duration);
      }
      if (progressSessionRef.current === descriptorSessionID) {
        progressSessionRef.current = "";
        progressSnapshotRef.current = null;
        resumeBarrierRef.current = null;
      }
      if (!accepted || !descriptorSessionID) {
        void cancelYouTubeWorkspacePlay(requestID).catch(() => {});
      } else {
        void pauseYouTubeWorkspaceVideo(descriptorSessionID).catch(() => {});
      }
    };
  // State-only progress responses must not restart the native playback session.
  // A different entry is keyed at the detail boundary and therefore remounts.
  }, [active, video]);

  React.useEffect(() => {
    if (!active || !playback?.sessionId) {
      return;
    }
    let disposed = false;
    const apply = (next: YouTubePlayerStatus) => {
      if (disposed || !isYouTubePlayerStatusForSession(next, playback.sessionId)) {
        return;
      }
      statusRef.current = next;
      setStatus(next);
      if (Number.isFinite(next.volume)) {
        setVolume(clamp01(Number(next.volume)));
      }
      if (typeof next.muted === "boolean") {
        setMuted(next.muted);
      }
      if (progressSessionRef.current !== playback.sessionId) {
        return;
      }
      const barrier = resumeBarrierRef.current;
      if (
        barrier?.sessionId === playback.sessionId &&
        !shouldAcceptRSSYouTubeProgress(next.currentTime || 0, barrier)
      ) {
        return;
      }
      resumeBarrierRef.current = null;
      progressSnapshotRef.current = {
        sessionId: playback.sessionId,
        currentTime: Math.max(0, Number(next.currentTime) || 0),
        duration: Math.max(
          0,
          Number(next.duration) ||
            progressSnapshotRef.current?.duration ||
            playback.durationSeconds ||
            0,
        ),
      };
      reportProgress(next, onProgress, progressRef);
    };
    void getYouTubeWorkspacePlayerStatus().then(apply).catch(() => {});
    const unsubscribe = subscribeYouTubePlayerStatus(playback.sessionId, apply);
    return () => {
      disposed = true;
      unsubscribe();
    };
  }, [active, onProgress, playback?.sessionId]);

  React.useEffect(
    () => subscribeYouTubeEmbeddedVideoFullscreen((next, sessionID) => {
      if (sessionID === playback?.sessionId) {
        setFullscreen(next);
      }
    }),
    [playback?.sessionId],
  );

  if (!video) {
    return null;
  }

  const playbackState = playback && controlsReady
    ? createRSSYouTubePlaybackState(playback, status, muted, volume, video)
    : null;
  const sessionID = playback?.sessionId || "";
  const run = (operation: Promise<unknown>) => {
    void operation.catch((reason) => {
      setStatus((current) => ({
        ...current,
        state: "error",
        errorMessage: readPlaybackError(reason),
      }));
    });
  };

  return (
    <div className="rss-youtube-playback" data-state={status.state || "loading"}>
      <div className="youtube-workspace-watch-video-region">
        <div className="youtube-workspace-watch-player-shell">
          <div className="youtube-workspace-player-card">
            <YouTubeNativeVideoSurface
              active={active && Boolean(playback) && controlsReady}
              allowRemotePosterCandidates={false}
              geometrySuspended={fullscreen}
              loadingLabel={text.listen.loadingStatus}
              poster={
                controlledRSSResourceURL(status.thumbnailUrl) ||
                controlledRSSResourceURL(entry.thumbnailUrl) ||
                undefined
              }
              scrollViewportSelector={RSS_VIDEO_SCROLL_VIEWPORT_SELECTOR}
              videoId={video.videoId}
            />
            {status.state === "error" ? (
              <GlassSurface
                asChild
                elevation="floating"
                shape="control"
                surfaceRole="status"
              >
                <p
                  className="youtube-workspace-player-error app-dream-status-message"
                  data-intent="danger"
                  role="alert"
                >
                  {status.errorMessage || text.youtube.errors.playerUnavailable}
                </p>
              </GlassSurface>
            ) : null}
          </div>
        </div>
      </div>
      {playbackState ? (
        <YouTubeWorkspaceTransportBar
          labels={{
            player: text.listen.nowPlaying,
            previous: text.listen.previous,
            play: text.listen.play,
            pause: text.listen.pause,
            next: text.listen.next,
            fullscreen: text.completed.previewEnterFullscreen,
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
          playback={playbackState}
          onPrevious={() => {}}
          onTogglePlayback={() => run(
            status.state === "playing"
              ? pauseYouTubeWorkspaceVideo(sessionID)
              : resumeYouTubeWorkspaceVideo(sessionID),
          )}
          onNext={() => {}}
          onDownload={onDownload}
          onFullscreen={() => run(requestYouTubeEmbeddedVideoFullscreen(sessionID))}
          onToggleMute={() => {
            const next = !muted;
            setMuted(next);
            run(setYouTubeWorkspaceVolume(sessionID, volume, next));
          }}
          onToggleCaptions={() => run(toggleYouTubeWorkspaceCaptions(sessionID))}
          onSelectCaption={(value) => run(selectYouTubeWorkspaceCaption(sessionID, value))}
          onSelectAudioTrack={(value) => run(selectYouTubeWorkspaceAudioTrack(sessionID, value))}
          onSelectQuality={(value) => run(selectYouTubeWorkspaceQuality(sessionID, value))}
          onSelectPlaybackRate={(value) => run(selectYouTubeWorkspacePlaybackRate(sessionID, value))}
          onVolumeChange={(next) => {
            const normalized = clamp01(next);
            setVolume(normalized);
            run(setYouTubeWorkspaceVolume(sessionID, normalized, muted));
          }}
          onSeek={(seconds) => {
            resumeBarrierRef.current = null;
            progressSnapshotRef.current = {
              sessionId: sessionID,
              currentTime: Math.max(0, seconds),
              duration: Math.max(0, Number(status.duration) || 0),
            };
            run(seekYouTubeWorkspaceVideo(sessionID, seconds));
          }}
        />
      ) : null}
    </div>
  );
}

export function rssEntryToYouTubeVideo(entry: RSSEntry): YouTubeWorkspaceVideo | null {
  const explicitPlatform = entry.platform?.trim().toLowerCase() || "";
  const detectedPlatform = siteKeyForRSSVideo(entry.url || entry.playbackUrl || "");
  if (
    (explicitPlatform && explicitPlatform !== "youtube") ||
    (!explicitPlatform && detectedPlatform !== "youtube")
  ) {
    return null;
  }
  const videoId = normalizedYouTubeVideoID(entry);
  if (!videoId) {
    return null;
  }
  return {
    itemKind: "video",
    videoId,
    title: entry.title,
    channel: entry.author,
    thumbnailUrl: entry.thumbnailUrl,
    webUrl: canonicalRSSVideoTarget(entry),
  };
}

function createRSSYouTubePlaybackState(
  descriptor: YouTubePlaybackDescriptor,
  status: YouTubePlayerStatus,
  muted: boolean,
  volume: number,
  video: YouTubeWorkspaceVideo,
): YouTubeWorkspacePlaybackState {
  return {
    descriptor,
    status,
    currentIndex: 0,
    queue: [video],
    muted,
    volume,
    capabilities: {
      previous: false,
      next: false,
      playPause: true,
      like: status.controls?.like === true,
      dislike: status.controls?.dislike === true,
      fullscreen: true,
      captions: status.controls?.captions === true,
      audioTrack: status.controls?.audioTrack === true,
      quality: status.controls?.quality === true,
      volume: resolveYouTubeVolumeCapability(descriptor, status),
      playbackRate: status.controls?.playbackRate === true,
    },
  };
}

function reportProgress(
  status: YouTubePlayerStatus,
  onProgress: RSSYouTubePlaybackProps["onProgress"],
  ref: React.MutableRefObject<{ second: number; reportedAt: number }>,
) {
  const currentTime = Math.max(0, Number(status.currentTime) || 0);
  const duration = Math.max(0, Number(status.duration) || 0);
  const second = Math.floor(currentTime);
  const now = Date.now();
  if (!onProgress || duration <= 0 || second === ref.current.second || now - ref.current.reportedAt < 5_000) {
    return;
  }
  ref.current = { second, reportedAt: now };
  onProgress(currentTime, duration);
}

/** Rejects the transient zero-time status commonly emitted before resume seek. */
export function shouldAcceptRSSYouTubeProgress(
  currentTime: number,
  barrier: RSSYouTubeProgressBarrier,
  now = Date.now(),
) {
  return shouldAcceptRSSResumedPlaybackProgress(
    currentTime,
    barrier.resumeAt,
    barrier.expiresAt,
    now,
  );
}

function createRSSYouTubeRequestID() {
  return Date.now() * 1_000 + Math.floor(Math.random() * 1_000);
}

function clamp01(value: number) {
  return Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
}

function readPlaybackError(reason: unknown) {
  if (reason instanceof Error) return reason.message;
  if (typeof reason === "string") return reason;
  return "Playback unavailable";
}
