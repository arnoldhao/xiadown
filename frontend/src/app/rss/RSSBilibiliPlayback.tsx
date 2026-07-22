import * as React from "react";

import type { RSSEntry } from "@/app/rss/types";
import { YouTubeWorkspaceTransportBar } from "@/app/youtube/YouTubeWorkspaceTransportBar";
import { getXiaText } from "@/features/xiadown/shared";
import { useI18n } from "@/shared/i18n";
import { GlassSurface } from "@/shared/ui/glass-surface";

import {
  acceptRSSBilibiliVideoPrepare,
  cancelRSSBilibiliVideoPrepare,
  closeRSSBilibiliVideo,
  exitRSSBilibiliVideoFullscreen,
  getRSSBilibiliVideoStatus,
  pauseRSSBilibiliVideo,
  playRSSBilibiliVideo,
  prepareRSSBilibiliVideo,
  requestRSSBilibiliVideoFullscreen,
  selectRSSBilibiliVideoCaption,
  selectRSSBilibiliVideoQuality,
  seekRSSBilibiliVideo,
  setRSSBilibiliVideoPlaybackRate,
  setRSSBilibiliVideoVolume,
  subscribeRSSBilibiliVideoStatus,
  toggleRSSBilibiliVideoCaptions,
  toggleRSSBilibiliVideoDanmaku,
  type RSSBilibiliPlaybackDescriptor,
  type RSSBilibiliPlayerStatus,
} from "./api";
import type { RSSBilibiliPageMetadata } from "./bilibili-page-metadata";
import { RSSBilibiliPrepareLifecycle } from "./bilibili-prepare-lifecycle";
import { RSSBilibiliVideoSurface } from "./RSSBilibiliVideoSurface";
import {
  createBilibiliTransportPlayback,
  isRSSBilibiliNativeReady,
  isRSSBilibiliVideoStatusForSession,
  RSS_BILIBILI_TRANSPORT_CONTROLS,
} from "./video-transport";
import { rssEntryImageCandidates, shouldAcceptRSSResumedPlaybackProgress } from "./workspace-utils";

export interface RSSBilibiliPlaybackProps {
  active: boolean;
  entry: RSSEntry;
  platformVideoId: string;
  onDownload: () => void;
  onMetadata?: (metadata: RSSBilibiliPageMetadata) => void;
  onProgress?: (currentTime: number, duration: number) => void;
}

interface RSSBilibiliProgressBarrier {
  resumeAt: number;
  expiresAt: number;
}

interface RSSBilibiliProgressSnapshot {
  sessionId: string;
  currentTime: number;
  duration: number;
}

const RSS_VIDEO_SCROLL_VIEWPORT_SELECTOR =
  ".rss-video-watch-page, .rss-focused-entry__content, .rss-entry-detail-pane";

function loadingBilibiliStatus(
  platformVideoId: string,
  title: string,
): RSSBilibiliPlayerStatus {
  return {
    provider: "bilibili",
    sessionId: "",
    available: false,
    platformVideoId,
    state: "loading",
    title,
    currentTime: 0,
    duration: 0,
    bufferedTime: 0,
    volume: 1,
    muted: false,
    playbackRate: 1,
    fullscreen: false,
    danmakuEnabled: false,
    controls: {
      playPause: false,
      seek: false,
      volume: false,
      playbackRate: false,
      fullscreen: false,
      captions: false,
      quality: false,
      danmaku: false,
    },
    captionOptions: [],
    qualityOptions: [],
    playbackRateOptions: [],
    selections: { playbackRateId: "1" },
  };
}

/**
 * Adapts Bilibili's native HTMLMediaElement bridge to the same transport used
 * by the YouTube station. App Session credentials remain native-only.
 */
export function RSSBilibiliPlayback({
  active,
  entry,
  platformVideoId,
  onDownload,
  onMetadata,
  onProgress,
}: RSSBilibiliPlaybackProps) {
  const { language } = useI18n();
  const text = React.useMemo(() => getXiaText(language), [language]);
  const [descriptor, setDescriptor] = React.useState<RSSBilibiliPlaybackDescriptor | null>(null);
  const [status, setStatus] = React.useState<RSSBilibiliPlayerStatus>(() =>
    loadingBilibiliStatus(platformVideoId, entry.title));
  const [fullscreenRequestPending, setFullscreenRequestPending] =
    React.useState(false);
  const nativeReady = isRSSBilibiliNativeReady(descriptor, status);
  const fullscreenOperationRef = React.useRef(0);
  const statusRef = React.useRef(status);
  const onMetadataRef = React.useRef(onMetadata);
  const onProgressRef = React.useRef(onProgress);
  const metadataSignatureRef = React.useRef("");
  const progressRef = React.useRef({ second: -1, reportedAt: 0 });
  const progressSnapshotRef = React.useRef<RSSBilibiliProgressSnapshot | null>(null);
  const resumeBarrierRef = React.useRef<RSSBilibiliProgressBarrier | null>(null);
  const prepareLifecycleRef = React.useRef<RSSBilibiliPrepareLifecycle | null>(null);
  if (!prepareLifecycleRef.current) {
    prepareLifecycleRef.current = new RSSBilibiliPrepareLifecycle();
  }
  statusRef.current = status;
  onMetadataRef.current = onMetadata;
  onProgressRef.current = onProgress;

  React.useEffect(() => {
    if (!active || !platformVideoId) return;
    const prepareLifecycle = prepareLifecycleRef.current;
    if (!prepareLifecycle) return;
    const prepareToken = prepareLifecycle.begin();
    let disposed = false;
    let sessionID = "";
    const resumeAt = entry.videoCompleted
      ? 0
      : Math.max(0, Number(entry.videoProgressSeconds) || 0);
    const savedDuration = Math.max(0, Number(entry.videoDurationSeconds) || 0);
    const initialStatus = loadingBilibiliStatus(platformVideoId, entry.title);
    initialStatus.currentTime = resumeAt;
    initialStatus.duration = savedDuration;
    statusRef.current = initialStatus;
    setStatus(initialStatus);
    setDescriptor(null);
    metadataSignatureRef.current = "";
    progressRef.current = { second: Math.floor(resumeAt), reportedAt: Date.now() };
    progressSnapshotRef.current = null;
    resumeBarrierRef.current = resumeAt > 1
      ? { resumeAt, expiresAt: Date.now() + 10_000 }
      : null;

    void (async () => {
      try {
        const nextDescriptor = await prepareRSSBilibiliVideo({
          requestId: prepareToken.requestId,
          platformVideoId,
          startSeconds: resumeAt,
          volume: 1,
          muted: false,
        });
        if (!prepareLifecycle.isCurrent(prepareToken) || disposed) {
          if (prepareLifecycle.cancel(prepareToken)) {
            await cancelRSSBilibiliVideoPrepare(prepareToken.requestId).catch(() => {});
          }
          return;
        }
        sessionID = nextDescriptor.sessionId;
        try {
          await acceptRSSBilibiliVideoPrepare(prepareToken.requestId);
        } catch (reason) {
          const settlement = prepareLifecycle.settle(prepareToken);
          await cancelRSSBilibiliVideoPrepare(prepareToken.requestId).catch(() => {});
          sessionID = "";
          if (!disposed && settlement.current) {
            const failedStatus = {
              ...statusRef.current,
              state: "error",
              errorMessage: playbackErrorText(reason),
            };
            statusRef.current = failedStatus;
            setStatus(failedStatus);
          }
          return;
        }
        const settlement = prepareLifecycle.settle(prepareToken);
        if (!settlement.current || disposed) {
          await closeRSSBilibiliVideo(sessionID).catch(() => {});
          return;
        }
        setDescriptor(nextDescriptor);
        const preparedStatus = {
          ...statusRef.current,
          sessionId: sessionID,
          available: true,
        };
        statusRef.current = preparedStatus;
        setStatus(preparedStatus);
        progressSnapshotRef.current = {
          sessionId: sessionID,
          currentTime: resumeAt,
          duration: savedDuration,
        };
      } catch (reason) {
        const settlement = prepareLifecycle.settle(prepareToken);
        if (!disposed && settlement.current) {
          const failedStatus = {
            ...statusRef.current,
            state: "error",
            errorMessage: playbackErrorText(reason),
          };
          statusRef.current = failedStatus;
          setStatus(failedStatus);
        }
      }
    })();

    return () => {
      disposed = true;
      if (prepareLifecycle.cancel(prepareToken)) {
        void cancelRSSBilibiliVideoPrepare(prepareToken.requestId).catch(() => {});
      }
      const snapshot = progressSnapshotRef.current;
      if (snapshot?.sessionId === sessionID && snapshot.duration > 0) {
        onProgressRef.current?.(snapshot.currentTime, snapshot.duration);
      }
      progressSnapshotRef.current = null;
      resumeBarrierRef.current = null;
      if (sessionID) {
        void closeRSSBilibiliVideo(sessionID).catch(() => {});
      }
    };
  // Progress-only entry updates must not recreate the native player session.
  }, [active, entry.id, platformVideoId]);

  React.useEffect(() => {
    const sessionID = descriptor?.sessionId || "";
    if (!active || !sessionID) return;
    let disposed = false;
    const apply = (next: RSSBilibiliPlayerStatus) => {
      if (disposed || !isRSSBilibiliVideoStatusForSession(next, sessionID)) return;
      statusRef.current = next;
      setStatus(next);
      const metadata: RSSBilibiliPageMetadata = {
        sessionId: sessionID,
        platformVideoId: next.platformVideoId.trim() || platformVideoId,
        publisher: next.publisher?.trim() || undefined,
        publishedAt: next.publishedAt?.trim() || undefined,
        viewCount: positiveMetadataCount(next.viewCount),
        likeCount: positiveMetadataCount(next.likeCount),
      };
      const metadataSignature = JSON.stringify(metadata);
      if (metadataSignatureRef.current !== metadataSignature) {
        metadataSignatureRef.current = metadataSignature;
        onMetadataRef.current?.(metadata);
      }
      const barrier = resumeBarrierRef.current;
      if (
        barrier &&
        !shouldAcceptRSSResumedPlaybackProgress(
          Math.max(0, Number(next.currentTime) || 0),
          barrier.resumeAt,
          barrier.expiresAt,
        )
      ) {
        return;
      }
      resumeBarrierRef.current = null;
      const snapshot = {
        sessionId: sessionID,
        currentTime: Math.max(0, Number(next.currentTime) || 0),
        duration: Math.max(
          0,
          Number(next.duration) ||
            progressSnapshotRef.current?.duration ||
            Number(entry.videoDurationSeconds) ||
            0,
        ),
      };
      progressSnapshotRef.current = snapshot;
      reportBilibiliProgress(snapshot, onProgressRef.current, progressRef);
    };
    void getRSSBilibiliVideoStatus().then(apply).catch(() => {});
    const unsubscribe = subscribeRSSBilibiliVideoStatus(sessionID, apply);
    return () => {
      disposed = true;
      unsubscribe();
    };
  }, [active, descriptor?.sessionId]);

  React.useEffect(() => {
    fullscreenOperationRef.current += 1;
    setFullscreenRequestPending(false);
  }, [active, descriptor?.sessionId]);

  const playback = createBilibiliTransportPlayback(
    descriptor,
    {
      ...status,
      controls: { ...status.controls, fullscreen: nativeReady },
    },
    entry,
  );
  const sessionID = descriptor?.sessionId || "";
  const run = (operation: Promise<unknown>) => {
    const operationSessionID = sessionID;
    void operation.catch((reason) => {
      setStatus((current) => {
        if (
          !operationSessionID ||
          current.sessionId !== operationSessionID
        ) {
          return current;
        }
        const next = {
          ...current,
          state: "error",
          errorMessage: playbackErrorText(reason),
        };
        statusRef.current = next;
        return next;
      });
    });
  };
  const toggleFullscreen = () => {
    if (!sessionID || !nativeReady) return;
    const operation = fullscreenOperationRef.current + 1;
    fullscreenOperationRef.current = operation;
    setFullscreenRequestPending(true);
    const command = status.fullscreen
      ? exitRSSBilibiliVideoFullscreen(sessionID)
      : requestRSSBilibiliVideoFullscreen(sessionID);
    void command
      .catch((reason) => {
        setStatus((current) => {
          if (
            fullscreenOperationRef.current !== operation ||
            current.sessionId !== sessionID
          ) {
            return current;
          }
          const next = {
            ...current,
            errorMessage: playbackErrorText(reason),
          };
          statusRef.current = next;
          return next;
        });
      })
      .finally(() => {
        if (fullscreenOperationRef.current === operation) {
          setFullscreenRequestPending(false);
        }
      });
  };

  return (
    <div
      className="rss-bilibili-playback"
      data-native-ready={nativeReady ? "true" : "false"}
      data-state={status.state || "loading"}
    >
      <div className="youtube-workspace-watch-video-region">
        <div className="youtube-workspace-watch-player-shell">
          <div className="youtube-workspace-player-card">
            <RSSBilibiliVideoSurface
              active={active && nativeReady}
              geometrySuspended={status.fullscreen || fullscreenRequestPending}
              loading={status.state !== "error"}
              loadingLabel={text.listen.loadingStatus}
              posterSources={rssEntryImageCandidates(entry)}
              scrollViewportSelector={RSS_VIDEO_SCROLL_VIEWPORT_SELECTOR}
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
          playback={playback}
          fullscreenActive={status.fullscreen}
          visibleControls={RSS_BILIBILI_TRANSPORT_CONTROLS}
          onPrevious={() => {}}
          onTogglePlayback={() => run(
            status.state === "playing"
              ? pauseRSSBilibiliVideo(sessionID)
              : playRSSBilibiliVideo(sessionID),
          )}
          onNext={() => {}}
          onDownload={onDownload}
          onFullscreen={toggleFullscreen}
          onToggleMute={() => {
            const muted = !status.muted;
            setStatus((current) => ({ ...current, muted }));
            run(setRSSBilibiliVideoVolume(sessionID, status.volume, muted));
          }}
          onToggleCaptions={() => run(toggleRSSBilibiliVideoCaptions(sessionID))}
          onSelectCaption={(value) => run(selectRSSBilibiliVideoCaption(sessionID, value))}
          onSelectAudioTrack={() => {}}
          onSelectQuality={(value) => run(selectRSSBilibiliVideoQuality(sessionID, value))}
          onToggleDanmaku={() => run(toggleRSSBilibiliVideoDanmaku(sessionID))}
          onSelectPlaybackRate={(value) => {
            const rate = Number.parseFloat(value);
            if (Number.isFinite(rate) && rate > 0) {
              setStatus((current) => ({
                ...current,
                playbackRate: rate,
                selections: { ...current.selections, playbackRateId: String(rate) },
              }));
              run(setRSSBilibiliVideoPlaybackRate(sessionID, rate));
            }
          }}
          onVolumeChange={(value) => {
            const volume = clamp01(value);
            setStatus((current) => ({ ...current, volume }));
            run(setRSSBilibiliVideoVolume(sessionID, volume, status.muted));
          }}
          onSeek={(seconds) => {
            resumeBarrierRef.current = null;
            const currentTime = Math.max(0, seconds);
            setStatus((current) => ({ ...current, currentTime }));
            progressSnapshotRef.current = {
              sessionId: sessionID,
              currentTime,
              duration: Math.max(0, status.duration),
            };
            run(seekRSSBilibiliVideo(sessionID, currentTime));
          }}
      />
    </div>
  );
}

function reportBilibiliProgress(
  snapshot: RSSBilibiliProgressSnapshot,
  onProgress: RSSBilibiliPlaybackProps["onProgress"],
  ref: React.MutableRefObject<{ second: number; reportedAt: number }>,
) {
  const second = Math.floor(snapshot.currentTime);
  const now = Date.now();
  if (
    !onProgress ||
    snapshot.duration <= 0 ||
    second === ref.current.second ||
    now - ref.current.reportedAt < 5_000
  ) {
    return;
  }
  ref.current = { second, reportedAt: now };
  onProgress(snapshot.currentTime, snapshot.duration);
}

function clamp01(value: number) {
  return Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
}

function positiveMetadataCount(value: number | null | undefined) {
  const count = Number(value);
  return Number.isFinite(count) && count > 0 ? Math.floor(count) : undefined;
}

function playbackErrorText(reason: unknown) {
  if (reason instanceof Error) return reason.message;
  if (typeof reason === "string") return reason;
  return "Playback unavailable";
}
