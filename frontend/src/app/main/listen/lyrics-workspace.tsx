import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { ListenLyricsCandidateTrack } from "@/app/main/listen/lyrics-api";
import {
  ListenLyricsSurface,
  type ListenLyricsSurfaceProps,
} from "@/app/main/listen/lyrics";
import {
  useListenLyricsFocusStylePreference,
  useListenLyricsOffsetPreference,
  useListenLyricsRendererPreference,
  type ListenLyricsTrackIdentity,
} from "@/app/main/listen/lyrics-preferences";
import { resolveListenLyricsRenderTimeMs } from "@/app/main/listen/lyrics-workspace-state";
import type { ListenLyricsData } from "@/app/main/listen/types";

type LyricsText = ReturnType<typeof getXiaText>;

export type ListenLyricsWorkspaceTrack = ListenLyricsCandidateTrack &
  ListenLyricsTrackIdentity;

export type ListenLyricsWorkspaceCurrentState = {
  lyrics?: ListenLyricsData | null;
  loading?: boolean;
  error?: string;
  errorCode?: string;
  errorRetryable?: boolean;
  onRetry?: () => void;
};

export type ListenLyricsWorkspaceProps = Pick<
  ListenLyricsSurfaceProps,
  | "onSeek"
  | "pinyin"
  | "playbackRate"
  | "romanized"
  | "timelineRunning"
  | "variant"
> & {
  text: LyricsText;
  track: ListenLyricsWorkspaceTrack;
  current: ListenLyricsWorkspaceCurrentState;
  currentTimeMs: number;
  className?: string;
  surfaceActive?: boolean;
  controls?: React.ReactNode;
};

export function ListenLyricsWorkspace(props: ListenLyricsWorkspaceProps) {
  const lyrics = props.current.lyrics;
  const renderer = useListenLyricsRendererPreference();
  const focusStyle = useListenLyricsFocusStylePreference();
  const offsetMs = useListenLyricsOffsetPreference(props.track, lyrics);
  const timingAvailable =
    lyrics?.kind === "synced" && lyrics.lines.length > 0;
  const effectiveRenderer =
    !lyrics ||
    lyrics.kind === "unavailable" ||
    (lyrics.kind === "synced" && lyrics.lines.length === 0)
    ? "empty"
    : timingAvailable
      ? renderer
      : "plain";
  const adjustedTimeMs = resolveListenLyricsRenderTimeMs(
    props.currentTimeMs,
    offsetMs,
  );
  const timelineRunning =
    props.surfaceActive !== false && props.timelineRunning === true;
  const adjustedSeek = props.onSeek
    ? (seconds: number) =>
        props.onSeek?.(Math.max(0, seconds - offsetMs / 1000))
    : undefined;

  return (
    <div
      className={cn("listen-lyrics-workspace", props.className)}
      data-listen-lyrics-workspace="true"
      data-renderer-effective={effectiveRenderer}
      data-renderer-preference={renderer}
      data-focus-style-preference={focusStyle}
      data-controls-placement={props.controls ? "overlay" : "none"}
    >
      <div className="listen-lyrics-workspace__surface">
        <ListenLyricsSurface
          variant={props.variant}
          renderer={renderer}
          focusStyle={focusStyle}
          text={props.text}
          lyrics={lyrics}
          loading={props.current.loading}
          error={props.current.error}
          errorCode={props.current.errorCode}
          errorRetryable={props.current.errorRetryable}
          onRetry={
            props.current.errorRetryable === false
              ? undefined
              : props.current.onRetry
          }
          currentTimeMs={adjustedTimeMs}
          clockKey={`offset:${offsetMs}`}
          timelineRunning={timelineRunning}
          playbackRate={props.playbackRate}
          romanized={props.romanized}
          pinyin={props.pinyin}
          onSeek={adjustedSeek}
        />
      </div>
      {props.controls ? (
        <div className="listen-lyrics-workspace__controls">
          {props.controls}
        </div>
      ) : null}
    </div>
  );
}
