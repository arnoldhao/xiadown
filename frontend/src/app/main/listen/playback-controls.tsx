import {
  Loader2,
  Pause,
  Play,
  Repeat2,
  Shuffle,
  SkipBack,
  SkipForward,
  Volume2,
  VolumeX,
} from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { clampVolume } from "@/app/main/listen/local-library";
import {
  listenArtistCountFromLabelParts,
  type ListenArtistLabelPart,
} from "@/app/main/listen/playback-helpers";
import { ListenPlayerIconButton } from "@/app/main/listen/playback-ui";
import type { ListenPlayMode } from "@/app/main/listen/types";
import {
  LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
  LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
  LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS,
  LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS,
} from "@/shared/styles/listen";
import { Button } from "@/shared/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

export function ListenTrackInfoRow(props: {
  title: string;
  subtitle: string;
  subtitleArtistParts?: ListenArtistLabelPart[];
  onSubtitleClick?: () => void;
  onSubtitleArtistClick?: (artist: string) => void;
  actions?: React.ReactNode;
}) {
  return (
    <div className="mt-5 flex min-h-14 items-center justify-between gap-4">
      <div className="listen-track-info-row__identity min-w-0 flex-1 overflow-hidden">
        <ListenScrollingText
          text={props.title}
          className="listen-track-info-row__title"
        />
        <ListenSubtitleText
          text={props.subtitle}
          artistParts={props.subtitleArtistParts}
          className="listen-track-info-row__subtitle mt-0.5"
          onClick={props.onSubtitleClick}
          onArtistClick={props.onSubtitleArtistClick}
        />
      </div>
      {props.actions ? (
        <div className="relative z-10 flex shrink-0 items-center gap-1.5">
          {props.actions}
        </div>
      ) : null}
    </div>
  );
}

export function ListenSubtitleText(props: {
  text: string;
  artistParts?: ListenArtistLabelPart[];
  className?: string;
  onClick?: () => void;
  onArtistClick?: (artist: string) => void;
}) {
  const artistParts = props.artistParts ?? [];
  if (
    props.onArtistClick &&
    listenArtistCountFromLabelParts(artistParts) > 0
  ) {
    return (
      <ListenArtistScrollingText
        text={props.text}
        artistParts={artistParts}
        className={props.className}
        onArtistClick={props.onArtistClick}
      />
    );
  }
  return (
    <ListenScrollingText
      text={props.text}
      className={props.className}
      onClick={props.onClick}
    />
  );
}

function ListenArtistScrollingText(props: {
  text: string;
  artistParts: ListenArtistLabelPart[];
  className?: string;
  onArtistClick: (artist: string) => void;
}) {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const normalizedText = props.text.trim();
  const scrolling = overflow > 1;
  const style = scrolling
    ? ({
        "--listen-marquee-shift": `-${Math.ceil(overflow + 18)}px`,
        "--listen-marquee-duration": `${Math.min(
          14,
          Math.max(7, (overflow + 180) / 30),
        )}s`,
      } as React.CSSProperties)
    : undefined;

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    const contentElement = contentRef.current;
    if (!container || !contentElement) {
      return;
    }
    const syncOverflow = () => {
      setOverflow(
        Math.max(0, contentElement.scrollWidth - container.clientWidth),
      );
    };
    syncOverflow();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(syncOverflow);
    observer.observe(container);
    observer.observe(contentElement);
    return () => observer.disconnect();
  }, [normalizedText, props.artistParts]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "listen-scrolling-text group/listen-marquee relative block w-full max-w-full min-w-0 overflow-hidden whitespace-nowrap",
        props.className,
      )}
      title={normalizedText}
    >
      <span
        ref={contentRef}
        className={cn(
          "inline-block max-w-none pr-4 align-top",
          scrolling && "listen-marquee-text",
        )}
        style={style}
      >
        {props.artistParts.map((part, index) =>
          part.kind === "separator" ? (
            <span key={`separator-${index}`} aria-hidden="true">
              {part.text}
            </span>
          ) : (
            <button
              key={`artist-${index}-${part.text}`}
              type="button"
              className="listen-scrolling-text__artist inline p-0"
              title={part.text}
              onClick={() => props.onArtistClick(part.text)}
            >
              {part.text}
            </button>
          ),
        )}
      </span>
    </div>
  );
}

export function ListenScrollingText(props: {
  text: string;
  className?: string;
  onClick?: () => void;
  as?: "div" | "span";
}) {
  const containerRef = React.useRef<HTMLElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const normalizedText = props.text.trim();
  const scrolling = overflow > 1;
  const style = scrolling
    ? ({
        "--listen-marquee-shift": `-${Math.ceil(overflow + 18)}px`,
        "--listen-marquee-duration": `${Math.min(
          14,
          Math.max(7, (overflow + 180) / 30),
        )}s`,
      } as React.CSSProperties)
    : undefined;
  const className = cn(
    "listen-scrolling-text group/listen-marquee relative block w-full max-w-full min-w-0 overflow-hidden whitespace-nowrap",
    props.onClick &&
      "listen-scrolling-text--interactive",
    props.className,
  );
  const content = (
    <span
      ref={contentRef}
      className={cn(
        "inline-block max-w-none pr-4 align-top",
        scrolling && "listen-marquee-text",
      )}
      style={style}
    >
      {normalizedText}
    </span>
  );

  React.useLayoutEffect(() => {
    const container = containerRef.current;
    const contentElement = contentRef.current;
    if (!container || !contentElement) {
      return;
    }
    const syncOverflow = () => {
      setOverflow(
        Math.max(0, contentElement.scrollWidth - container.clientWidth),
      );
    };
    syncOverflow();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(syncOverflow);
    observer.observe(container);
    observer.observe(contentElement);
    return () => observer.disconnect();
  }, [normalizedText]);

  if (props.onClick) {
    return (
      <button
        ref={containerRef as React.RefObject<HTMLButtonElement>}
        type="button"
        className={className}
        title={normalizedText}
        onClick={props.onClick}
      >
        {content}
      </button>
    );
  }

  if (props.as === "span") {
    return (
      <span
        ref={containerRef as React.RefObject<HTMLSpanElement>}
        className={className}
        title={normalizedText}
      >
        {content}
      </span>
    );
  }

  return (
    <div
      ref={containerRef as React.RefObject<HTMLDivElement>}
      className={className}
      title={normalizedText}
    >
      {content}
    </div>
  );
}

export function ListenPlayerTransport(props: {
  playing: boolean;
  loading: boolean;
  playMode: ListenPlayMode;
  live?: boolean;
  disabled?: boolean;
  text: ReturnType<typeof getXiaText>;
  onPrevious: () => void;
  onNext: () => void;
  onPlayModeChange: (mode: ListenPlayMode) => void;
  onTogglePlayback: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const shuffleActive = props.playMode === "shuffle";
  const repeatActive = !props.live && props.playMode === "repeat";
  const playLabel = props.playing
    ? props.text.listen.pause
    : props.text.listen.play;

  if (props.live) {
    return (
      <div className="mt-3 flex h-14 items-center justify-center">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="default"
              size="icon"
              shape="circle"
              className={cn(
                LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
                LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.medium,
                !props.disabled && LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
              )}
              disabled={props.disabled}
              aria-label={playLabel}
              title={playLabel}
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2
                  className={cn(
                    LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium,
                    "listen-loading-spinner",
                  )}
                />
              ) : props.playing ? (
                <Pause
                  className={cn(
                    LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium,
                    "listen-playback-icon--filled",
                  )}
                />
              ) : (
                <Play
                  className={cn(
                    "listen-playback-icon--filled ml-0.5",
                    LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium,
                  )}
                />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">{playLabel}</TooltipContent>
        </Tooltip>
      </div>
    );
  }

  return (
    <div className="mt-3 grid h-14 grid-cols-[3.5rem_1fr_3.5rem] items-center">
      <div className="justify-self-start">
        <ListenTransportIconButton
          label={props.text.listen.playModeShuffle}
          active={shuffleActive}
          disabled={props.disabled}
          size="small"
          onClick={() =>
            props.onPlayModeChange(shuffleActive ? "order" : "shuffle")
          }
        >
          <Shuffle />
        </ListenTransportIconButton>
      </div>
      <div className="flex items-center justify-center gap-3">
        <ListenTransportIconButton
          label={props.text.listen.previous}
          disabled={props.disabled}
          onClick={props.onPrevious}
        >
          <SkipBack />
        </ListenTransportIconButton>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="default"
              size="icon"
              shape="circle"
              className={cn(
                LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
                LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.medium,
                !props.disabled && LISTEN_PRIMARY_PLAY_BUTTON_HOVER_CLASS,
              )}
              disabled={props.disabled}
              aria-label={playLabel}
              title={playLabel}
              onClick={props.onTogglePlayback}
            >
              {props.loading ? (
                <Loader2
                  className={cn(
                    LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium,
                    "listen-loading-spinner",
                  )}
                />
              ) : props.playing ? (
                <Pause
                  className={cn(
                    LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium,
                    "listen-playback-icon--filled",
                  )}
                />
              ) : (
                <Play
                  className={cn(
                    "listen-playback-icon--filled ml-0.5",
                    LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.medium,
                  )}
                />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">{playLabel}</TooltipContent>
        </Tooltip>
        <ListenTransportIconButton
          label={props.text.listen.next}
          disabled={props.disabled}
          onClick={props.onNext}
        >
          <SkipForward />
        </ListenTransportIconButton>
      </div>
      <div className="justify-self-end">
        <ListenTransportIconButton
          label={props.text.listen.playModeRepeat}
          active={repeatActive}
          disabled={props.live || props.disabled}
          size="small"
          onClick={() =>
            props.onPlayModeChange(repeatActive ? "order" : "repeat")
          }
        >
          <Repeat2 />
        </ListenTransportIconButton>
      </div>
    </div>
  );
}

function ListenTransportIconButton(props: {
  label: string;
  active?: boolean;
  disabled?: boolean;
  size?: "normal" | "small";
  children: React.ReactNode;
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
}) {
  const small = props.size === "small";
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          shape="circle"
          data-active={props.active ? "true" : "false"}
          data-transport-size={small ? "small" : "normal"}
          disabled={props.disabled}
          className="listen-transport-icon-button relative"
          aria-label={props.label}
          title={props.label}
          onClick={props.onClick}
        >
          {props.children}
          {props.active ? (
            <span className="listen-transport-icon-button__active-marker absolute bottom-0 h-1 w-1" />
          ) : null}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="top">{props.label}</TooltipContent>
    </Tooltip>
  );
}

export function ListenPlayerVolume(props: {
  muted: boolean;
  volume: number;
  text: ReturnType<typeof getXiaText>;
  onToggleMute: () => void;
  onVolumeChange: (value: number) => void;
}) {
  const visibleVolume = props.muted ? 0 : clampVolume(props.volume);
  const volumePercent = Math.round(visibleVolume * 1000) / 10;

  return (
    <div className="listen-player-volume mt-4 flex h-8 items-center gap-3">
      <ListenPlayerIconButton
        label={
          props.muted || props.volume <= 0
            ? props.text.listen.unmute
            : props.text.listen.mute
        }
        className="listen-player-volume__button"
        onClick={props.onToggleMute}
      >
        {props.muted || props.volume <= 0 ? (
          <VolumeX className="h-4 w-4" />
        ) : (
          <Volume2 className="h-4 w-4" />
        )}
      </ListenPlayerIconButton>
      <div className="group/volume-slider relative flex h-6 min-w-0 flex-1 items-center">
        <div className="listen-player-volume__track pointer-events-none absolute left-0 right-0 top-1/2 h-1.5 -translate-y-1/2 overflow-hidden">
          <div
            className="listen-player-volume__fill absolute inset-y-0 left-0"
            style={{ width: `${volumePercent}%` }}
          />
        </div>
        <span
          aria-hidden="true"
          className="listen-player-volume__thumb pointer-events-none absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2"
          style={{ left: `${volumePercent}%` }}
        />
        <input
          type="range"
          min={0}
          max={1}
          step={0.01}
          value={visibleVolume}
          aria-label={props.text.listen.volume}
          title={props.text.listen.volume}
          className="listen-player-volume__input relative z-10 h-6 w-full"
          onChange={(event) =>
            props.onVolumeChange(Number(event.target.value))
          }
        />
      </div>
      <ListenPlayerIconButton
        label={props.text.listen.volume}
        className="listen-player-volume__button listen-player-volume__maximum"
        onClick={() => props.onVolumeChange(1)}
      >
        <Volume2 className="h-4 w-4" />
      </ListenPlayerIconButton>
    </div>
  );
}
