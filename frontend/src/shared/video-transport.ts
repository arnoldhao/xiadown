/**
 * Provider-neutral playback contract consumed by the shared workspace video
 * transport. Providers may omit controls and option collections they cannot
 * support; the transport keeps those actions visible but unavailable.
 */
export interface WorkspaceVideoTransportOption {
  id: string;
  label: string;
}

export interface WorkspaceVideoTransportDescriptor {
  title: string;
  durationSeconds?: number;
}

export interface WorkspaceVideoTransportSelections {
  captionId?: string;
  audioTrackId?: string;
  qualityId?: string;
  playbackRateId?: string;
}

export type WorkspaceVideoTransportControl =
  | "download"
  | "playbackRate"
  | "captions"
  | "audioTrack"
  | "quality"
  | "danmaku"
  | "upNext"
  | "volume"
  | "fullscreen";

export interface WorkspaceVideoTransportStatus {
  state?: string;
  title?: string;
  currentTime?: number;
  duration?: number;
  danmakuEnabled?: boolean;
  captionOptions?: WorkspaceVideoTransportOption[];
  audioTrackOptions?: WorkspaceVideoTransportOption[];
  qualityOptions?: WorkspaceVideoTransportOption[];
  playbackRateOptions?: WorkspaceVideoTransportOption[];
  selections?: WorkspaceVideoTransportSelections;
}

export interface WorkspaceVideoTransportCapabilities {
  playPause: boolean;
  /** Omitted by legacy providers means supported. */
  seek?: boolean;
  fullscreen: boolean;
  captions: boolean;
  audioTrack: boolean;
  quality: boolean;
  danmaku?: boolean;
  volume: boolean;
  playbackRate?: boolean;
}

export interface WorkspaceVideoTransportPlayback {
  descriptor: WorkspaceVideoTransportDescriptor;
  status: WorkspaceVideoTransportStatus;
  muted: boolean;
  volume: number;
  capabilities: WorkspaceVideoTransportCapabilities;
}
