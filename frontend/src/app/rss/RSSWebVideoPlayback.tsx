import { Play } from "lucide-react";
import * as React from "react";

import type { RSSEntry } from "@/app/rss/types";
import { YouTubeWorkspaceTransportBar } from "@/app/youtube/YouTubeWorkspaceTransportBar";
import { getXiaText } from "@/features/xiadown/shared";
import { useI18n } from "@/shared/i18n";
import { GlassSurface } from "@/shared/ui/glass-surface";

import { controlledRSSResourceURL } from "./remote-resource";
import { useRSSHostVideoFullscreen } from "./host-video-fullscreen";
import type { RSSVideoExperience } from "./video-platform";
import { createRSSWebVideoTransportPlayback } from "./video-transport";
import { rssEntryImageCandidates, shouldAcceptRSSResumedPlaybackProgress } from "./workspace-utils";

export const RSS_VIDEO_IFRAME_PERMISSIONS =
  "autoplay; encrypted-media; picture-in-picture";

export interface RSSWebVideoPlaybackProps {
  entry: RSSEntry;
  experience: RSSVideoExperience;
  onDownload?: () => void;
  onProgress?: (currentTime: number, duration: number) => void;
}

function firstControlledRSSResourceURL(
  candidates: readonly (string | undefined)[],
) {
  for (const candidate of candidates) {
    const controlled = controlledRSSResourceURL(candidate);
    if (controlled) return controlled;
  }
  return "";
}

/**
 * Shared workspace transport for controllable direct media and honest
 * capability fallbacks for cross-origin embeds or unsupported entries.
 */
export function RSSWebVideoPlayback({
  entry,
  experience,
  onDownload,
  onProgress,
}: RSSWebVideoPlaybackProps) {
  const { language, t } = useI18n();
  const text = React.useMemo(() => getXiaText(language), [language]);
  const direct = experience.mode === "direct" && Boolean(experience.playbackUrl);
  const embed = experience.mode === "embed" && Boolean(experience.playbackUrl);
  const hostFullscreen = useRSSHostVideoFullscreen(direct || embed);
  const videoRef = React.useRef<HTMLVideoElement | null>(null);
  const onProgressRef = React.useRef(onProgress);
  const lastProgressRef = React.useRef({ second: -1, reportedAt: 0 });
  const resumeAtRef = React.useRef(
    entry.videoCompleted ? 0 : Math.max(0, Number(entry.videoProgressSeconds) || 0),
  );
  const resumeExpiresAtRef = React.useRef(Date.now() + 10_000);
  const [state, setState] = React.useState(direct ? "loading" : "paused");
  const [currentTime, setCurrentTime] = React.useState(resumeAtRef.current);
  const [duration, setDuration] = React.useState(
    Math.max(0, Number(entry.videoDurationSeconds) || 0),
  );
  const [volume, setVolume] = React.useState(1);
  const [muted, setMuted] = React.useState(false);
  const [playbackRate, setPlaybackRate] = React.useState(1);
  const [playbackError, setPlaybackError] = React.useState("");
  onProgressRef.current = onProgress;

  React.useEffect(() => {
    resumeAtRef.current = entry.videoCompleted
      ? 0
      : Math.max(0, Number(entry.videoProgressSeconds) || 0);
    resumeExpiresAtRef.current = Date.now() + 10_000;
    lastProgressRef.current = {
      second: Math.floor(resumeAtRef.current),
      reportedAt: Date.now(),
    };
    setCurrentTime(resumeAtRef.current);
    setDuration(Math.max(0, Number(entry.videoDurationSeconds) || 0));
    setState(direct ? "loading" : "paused");
    setPlaybackError("");
  }, [direct, entry.id, experience.playbackUrl]);

  React.useEffect(() => () => {
    const player = videoRef.current;
    if (direct && player && Number.isFinite(player.duration) && player.duration > 0) {
      onProgressRef.current?.(player.currentTime, player.duration);
    }
  }, [direct, entry.id, experience.playbackUrl]);

  const reportProgress = React.useCallback((force = false) => {
    const player = videoRef.current;
    if (!player || !Number.isFinite(player.duration) || player.duration <= 0) return;
    const now = Date.now();
    if (
      resumeAtRef.current > 1 &&
      !shouldAcceptRSSResumedPlaybackProgress(
        player.currentTime,
        resumeAtRef.current,
        resumeExpiresAtRef.current,
        now,
      )
    ) {
      return;
    }
    resumeAtRef.current = 0;
    const second = Math.floor(player.currentTime);
    if (
      !force &&
      (second === lastProgressRef.current.second ||
        now - lastProgressRef.current.reportedAt < 5_000)
    ) {
      return;
    }
    lastProgressRef.current = { second, reportedAt: now };
    onProgressRef.current?.(player.currentTime, player.duration);
  }, []);

  const playback = createRSSWebVideoTransportPlayback({
    direct,
    title: entry.title,
    state,
    currentTime,
    duration,
    volume,
    muted,
    playbackRate,
    fullscreenAvailable: direct || embed,
  });

  const run = (operation: Promise<unknown> | undefined) => {
    void operation?.catch((reason) => {
      setState("error");
      setPlaybackError(reason instanceof Error ? reason.message : String(reason));
    });
  };
  return (
    <div
      className="rss-web-video-playback rss-host-video-playback"
      data-host-fullscreen={hostFullscreen.fullscreen ? "true" : undefined}
      data-state={state}
    >
      <div className="youtube-workspace-watch-video-region">
        <div className="youtube-workspace-watch-player-shell">
          <div className="youtube-workspace-player-card">
            <div className="rss-video-player">
              {embed ? (
                <iframe
                  allow={RSS_VIDEO_IFRAME_PERMISSIONS}
                  referrerPolicy="no-referrer"
                  sandbox="allow-scripts allow-same-origin allow-presentation"
                  src={experience.playbackUrl}
                  title={entry.title}
                />
              ) : direct ? (
                <video
                  playsInline
                  poster={firstControlledRSSResourceURL(rssEntryImageCandidates(entry)) || undefined}
                  ref={videoRef}
                  src={experience.playbackUrl}
                  onCanPlay={() => setState((current) => current === "playing" ? current : "paused")}
                  onDurationChange={(event) => setDuration(
                    Number.isFinite(event.currentTarget.duration)
                      ? Math.max(0, event.currentTarget.duration)
                      : 0,
                  )}
                  onEnded={() => {
                    setState("ended");
                    reportProgress(true);
                  }}
                  onError={() => {
                    setState("error");
                    setPlaybackError(t("xiadown.rss.noPlayableVideo"));
                  }}
                  onLoadedMetadata={(event) => {
                    const player = event.currentTarget;
                    const nextDuration = Number.isFinite(player.duration)
                      ? Math.max(0, player.duration)
                      : 0;
                    setDuration(nextDuration);
                    if (resumeAtRef.current > 1 && resumeAtRef.current < nextDuration) {
                      player.currentTime = resumeAtRef.current;
                      setCurrentTime(resumeAtRef.current);
                    } else {
                      resumeAtRef.current = 0;
                    }
                  }}
                  onPause={() => {
                    setState("paused");
                    reportProgress(true);
                  }}
                  onPlay={() => setState("playing")}
                  onRateChange={(event) => setPlaybackRate(event.currentTarget.playbackRate)}
                  onTimeUpdate={(event) => {
                    setCurrentTime(Math.max(0, event.currentTarget.currentTime));
                    reportProgress();
                  }}
                  onVolumeChange={(event) => {
                    setVolume(event.currentTarget.volume);
                    setMuted(event.currentTarget.muted);
                  }}
                  onWaiting={() => setState("buffering")}
                />
              ) : (
                <div className="rss-video-player__empty">
                  <Play />
                  <span>{playbackError || t("xiadown.rss.noPlayableVideo")}</span>
                </div>
              )}
            </div>
            {playbackError && direct ? (
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
                  {playbackError}
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
        fullscreenActive={hostFullscreen.fullscreen}
        onPrevious={() => {}}
        onTogglePlayback={() => {
          const player = videoRef.current;
          if (!player) return;
          if (player.paused) run(player.play());
          else player.pause();
        }}
        onNext={() => {}}
        onDownload={onDownload}
        onFullscreen={() => run(hostFullscreen.toggle())}
        onToggleMute={() => {
          const player = videoRef.current;
          if (!player) return;
          player.muted = !player.muted;
          setMuted(player.muted);
        }}
        onToggleCaptions={() => {}}
        onSelectCaption={() => {}}
        onSelectAudioTrack={() => {}}
        onSelectQuality={() => {}}
        onSelectPlaybackRate={(value) => {
          const player = videoRef.current;
          const rate = Number.parseFloat(value);
          if (!player || !Number.isFinite(rate) || rate <= 0) return;
          player.playbackRate = rate;
          setPlaybackRate(rate);
        }}
        onVolumeChange={(value) => {
          const player = videoRef.current;
          if (!player) return;
          const next = Math.max(0, Math.min(1, value));
          player.volume = next;
          setVolume(next);
        }}
        onSeek={(seconds) => {
          const player = videoRef.current;
          if (!player || !Number.isFinite(seconds)) return;
          resumeAtRef.current = 0;
          player.currentTime = Math.max(0, Math.min(player.duration || seconds, seconds));
          setCurrentTime(player.currentTime);
        }}
      />
    </div>
  );
}
