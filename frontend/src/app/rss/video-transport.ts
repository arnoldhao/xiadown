import type { RSSEntry } from "./types";
import type {
  RSSBilibiliPlaybackDescriptor,
  RSSBilibiliPlayerStatus,
} from "./api";
import type {
  WorkspaceVideoTransportControl,
  WorkspaceVideoTransportPlayback,
} from "@/shared/video-transport";

const RSS_VIDEO_PLAYBACK_RATES = [0.5, 0.75, 1, 1.25, 1.5, 2] as const;

export const RSS_BILIBILI_TRANSPORT_CONTROLS = [
  "download",
  "playbackRate",
  "captions",
  "quality",
  "danmaku",
  "volume",
  "fullscreen",
] as const satisfies readonly WorkspaceVideoTransportControl[];

export function isRSSBilibiliVideoStatusForSession(
  status: RSSBilibiliPlayerStatus | null | undefined,
  sessionId: string,
): status is RSSBilibiliPlayerStatus {
  const expectedSessionID = sessionId.trim();
  return Boolean(status) &&
    status?.provider === "bilibili" &&
    Boolean(expectedSessionID) &&
    status.sessionId?.trim() === expectedSessionID;
}

/**
 * The native underlay must stay hidden until the capability event belongs to
 * the descriptor that won the current Prepare generation. A prepared URL on
 * its own is not proof that the page's HTMLMediaElement bridge is ready.
 */
export function isRSSBilibiliNativeReady(
  descriptor: RSSBilibiliPlaybackDescriptor | null | undefined,
  status: RSSBilibiliPlayerStatus | null | undefined,
) {
  const sessionID = descriptor?.sessionId?.trim() || "";
  return isRSSBilibiliVideoStatusForSession(status, sessionID) &&
    status.available === true &&
    status.controls?.playPause === true;
}

export function createBilibiliTransportPlayback(
  descriptor: RSSBilibiliPlaybackDescriptor | null | undefined,
  status: RSSBilibiliPlayerStatus,
  entry: Pick<RSSEntry, "title" | "videoDurationSeconds">,
): WorkspaceVideoTransportPlayback {
  const sessionID = descriptor?.sessionId?.trim() || "";
  const bridgeAvailable =
    isRSSBilibiliVideoStatusForSession(status, sessionID) &&
    status.available === true;
  const controls = bridgeAvailable ? status.controls : undefined;
  return {
    descriptor: {
      title: entry.title,
      durationSeconds: Math.max(
        0,
        Number(status.duration) || Number(entry.videoDurationSeconds) || 0,
      ),
    },
    status: {
      state: status.state,
      title: status.title || entry.title,
      currentTime: Math.max(0, Number(status.currentTime) || 0),
      duration: Math.max(0, Number(status.duration) || 0),
      danmakuEnabled: status.danmakuEnabled === true,
      captionOptions: status.captionOptions || [],
      qualityOptions: status.qualityOptions || [],
      playbackRateOptions: status.playbackRateOptions || [],
      selections: {
        playbackRateId:
          status.selections?.playbackRateId || String(status.playbackRate || 1),
        captionId: status.selections?.captionId || "",
        qualityId: status.selections?.qualityId || "",
      },
    },
    muted: Boolean(status.muted),
    volume: clamp01(Number(status.volume)),
    capabilities: {
      playPause: controls?.playPause === true,
      seek: controls?.seek === true,
      fullscreen: controls?.fullscreen === true,
      captions: controls?.captions === true,
      audioTrack: false,
      quality: controls?.quality === true,
      danmaku: controls?.danmaku === true,
      volume: controls?.volume === true,
      playbackRate: controls?.playbackRate === true,
    },
  };
}

export function createRSSWebVideoTransportPlayback(options: {
  direct: boolean;
  title: string;
  state: string;
  currentTime: number;
  duration: number;
  volume: number;
  muted: boolean;
  playbackRate: number;
  fullscreenAvailable: boolean;
}): WorkspaceVideoTransportPlayback {
  return {
    descriptor: {
      title: options.title,
      durationSeconds: options.duration,
    },
    status: {
      state: options.state,
      title: options.title,
      currentTime: options.currentTime,
      duration: options.duration,
      playbackRateOptions: options.direct
        ? RSS_VIDEO_PLAYBACK_RATES.map((rate) => ({ id: String(rate), label: String(rate) }))
        : [],
      selections: { playbackRateId: String(options.playbackRate) },
    },
    muted: options.muted,
    volume: options.volume,
    capabilities: {
      playPause: options.direct,
      seek: options.direct && options.duration > 0,
      fullscreen: options.fullscreenAvailable,
      captions: false,
      audioTrack: false,
      quality: false,
      danmaku: false,
      volume: options.direct,
      playbackRate: options.direct,
    },
  };
}

function clamp01(value: number) {
  return Math.max(0, Math.min(1, Number.isFinite(value) ? value : 0));
}
