import {
  Download,
  ExternalLink,
  Heart,
  ListEnd,
  Loader2,
  Maximize2,
  MessageSquareQuote,
  MoreHorizontal,
  Pause,
  Play,
  Repeat1,
  Shuffle,
  SkipBack,
  SkipForward,
  UserRound,
  Volume2,
  VolumeX,
} from "lucide-react";
import * as React from "react";

import type {
  ListenExternalCommand,
  ListenNowPlayingStatus,
} from "@/app/main/Listen";
import { ListenSidebarArtwork } from "@/app/main/sidebar";
import { splitListenArtistLabel } from "@/app/main/listen/playback-helpers";
import type { ListenTrackArtist } from "@/app/main/listen/types";
import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { GlassGroup } from "@/shared/ui/glass-surface";


export type WorkspaceTransportLabels = {
  idleStatus: string;
  idleSubtitle: string;
  shuffle: string;
  previous: string;
  play: string;
  pause: string;
  next: string;
  repeatOne: string;
  live: string;
  lyrics: string;
  upNext: string;
  volume: string;
  fullscreen: string;
  more: string;
  favorite: string;
  download: string;
  openURL: string;
};

export type WorkspaceTransportArtistPart =
  | { kind: "artist"; artist: ListenTrackArtist }
  | { kind: "separator"; text: string };

export function resolveWorkspaceTransportStatus(
  status: ListenNowPlayingStatus | null,
  labels: Pick<WorkspaceTransportLabels, "idleStatus" | "idleSubtitle">,
): ListenNowPlayingStatus {
  if (status && status.state !== "idle") {
    return status;
  }
  return {
    state: "idle",
    live: false,
    mediaId: "",
    title: labels.idleStatus,
    subtitle: labels.idleSubtitle,
    artists: [],
    artworkURL: "",
    artworkCandidates: [],
    playbackSource: "unknown",
    mode: "muse",
    canControl: false,
    progress: { currentTime: 0, duration: 0, bufferedTime: 0 },
  };
}

export function resolveWorkspaceTransportArtistParts(
  status: Pick<ListenNowPlayingStatus, "artists" | "subtitle">,
): WorkspaceTransportArtistPart[] {
  const artists: ListenTrackArtist[] = [];
  const seen = new Set<string>();
  for (const candidate of status.artists ?? []) {
    const name = candidate.name.trim();
    const browseId = candidate.browseId?.trim() ?? "";
    if (!name) {
      continue;
    }
    const key = browseId || name.toLocaleLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    artists.push({
      name,
      browseId: browseId || undefined,
      thumbnailUrl: candidate.thumbnailUrl?.trim() || undefined,
    });
  }
  if (artists.length > 0) {
    const parts: WorkspaceTransportArtistPart[] = [];
    artists.forEach((artist, index) => {
      if (index > 0) {
        parts.push({ kind: "separator", text: ", " });
      }
      parts.push({ kind: "artist", artist });
    });
    return parts;
  }
  return splitListenArtistLabel(status.subtitle).map((part) =>
    part.kind === "artist"
      ? { kind: "artist", artist: { name: part.text } }
      : part,
  );
}

export function resolveWorkspaceTransportMenuArtists(
  parts: WorkspaceTransportArtistPart[],
): ListenTrackArtist[] {
  return parts
    .flatMap((part) => (part.kind === "artist" ? [part.artist] : []))
    .slice(0, 3);
}

export type TransportVolumeEditorState = {
  draft: number;
  dragging: boolean;
};

export type TransportVolumeEditorAction =
  | { type: "sync"; volume: number }
  | { type: "input"; volume: number }
  | { type: "drag-start" }
  | { type: "drag-end" };

function clampTransportVolume(value: number) {
  return Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0));
}

export function transportVolumeEditorReducer(
  state: TransportVolumeEditorState,
  action: TransportVolumeEditorAction,
): TransportVolumeEditorState {
  switch (action.type) {
    case "sync":
      return state.dragging
        ? state
        : { ...state, draft: clampTransportVolume(action.volume) };
    case "input":
      return { ...state, draft: clampTransportVolume(action.volume) };
    case "drag-start":
      return state.dragging ? state : { ...state, dragging: true };
    case "drag-end":
      return state.dragging ? { ...state, dragging: false } : state;
  }
}

export function MusicWorkspaceTransportBar(props: {
  status: ListenNowPlayingStatus | null;
  labels: WorkspaceTransportLabels;
  onCommand: (
    command: ListenExternalCommand["command"],
    value?: number,
  ) => void;
  onOpenPlayer: () => void;
  onOpenArtist?: (artist: ListenTrackArtist) => void;
  onOpenLyrics: () => void;
  onOpenQueue: () => void;
  onFullscreen: () => void;
  onDownload?: () => void;
  onFavorite?: () => void;
  onOpenURL?: () => void;
  volume?: number;
  muted?: boolean;
  onVolumeChange?: (volume: number) => void;
  onToggleMute?: () => void;
}) {
  const idle = !props.status || props.status.state === "idle";
  const status = resolveWorkspaceTransportStatus(props.status, props.labels);
  const [volumeExpanded, setVolumeExpanded] = React.useState(false);
  const normalizedVolume = clampTransportVolume(props.volume ?? 0);
  const [volumeEditor, dispatchVolumeEditor] = React.useReducer(
    transportVolumeEditorReducer,
    { draft: normalizedVolume, dragging: false },
  );
  const volumeTriggerRef = React.useRef<HTMLButtonElement>(null);
  const volumeSliderRef = React.useRef<HTMLInputElement>(null);
  const volumeRegionRef = React.useRef<HTMLDivElement>(null);
  const volumeControlsAvailable =
    !idle &&
    typeof props.volume === "number" &&
    typeof props.muted === "boolean" &&
    typeof props.onVolumeChange === "function" &&
    typeof props.onToggleMute === "function";

  React.useEffect(() => {
    if (!volumeControlsAvailable) {
      setVolumeExpanded(false);
      dispatchVolumeEditor({ type: "drag-end" });
      return;
    }
    if (volumeExpanded) {
      volumeSliderRef.current?.focus();
    }
  }, [volumeControlsAvailable, volumeExpanded]);

  React.useEffect(() => {
    dispatchVolumeEditor({ type: "sync", volume: normalizedVolume });
  }, [normalizedVolume, volumeEditor.dragging]);

  React.useEffect(() => {
    if (!volumeExpanded || typeof document === "undefined") {
      return;
    }
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Node && !volumeRegionRef.current?.contains(target)) {
        setVolumeExpanded(false);
        dispatchVolumeEditor({ type: "drag-end" });
      }
    };
    document.addEventListener("pointerdown", closeOnOutsidePointer, true);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer, true);
    };
  }, [volumeExpanded]);

  const loading = !idle && status.state === "loading";
  const playing = !idle && status.state === "playing";
  const playbackActive = loading || playing;
  const live = !idle && (status.live === true || status.mode === "hush");
  const duration = Math.max(0, status.progress.duration || 0);
  const currentTime = Math.min(duration || Number.POSITIVE_INFINITY, Math.max(0, status.progress.currentTime || 0));
  const progress = live
    ? 100
    : duration > 0
      ? Math.min(100, (currentTime / duration) * 100)
      : 0;
  const remaining = live ? 0 : Math.max(0, duration - currentTime);
  const seekable = !live && duration > 0 && status.canControl;
  const timelineProgressbar = !idle && !seekable && (live || duration > 0);
  const timelineDecorative = !seekable && !timelineProgressbar;
  const artistParts = resolveWorkspaceTransportArtistParts(status);
  const menuArtists = resolveWorkspaceTransportMenuArtists(artistParts);
  const onOpenArtist = idle ? undefined : props.onOpenArtist;
  const onDownload = idle ? undefined : props.onDownload;
  const onFavorite = idle ? undefined : props.onFavorite;
  const onOpenURL = idle ? undefined : props.onOpenURL;
  const moreMenuAvailable = Boolean(
    onDownload ||
      onFavorite ||
      onOpenURL ||
      (onOpenArtist && menuArtists.length > 0),
  );

  return (
    <GlassGroup
      className="app-workspace-transport"
      elevation="floating"
      shape="capsule"
      surfaceRole="control"
      data-state={status.state}
      role="region"
      aria-label={status.title}
    >
      <div className="app-workspace-transport__left">
        <TransportButton
          label={props.labels.shuffle}
          disabled={idle}
          onClick={() => props.onCommand("shuffle")}
        >
          <Shuffle />
        </TransportButton>
        <TransportButton
          emphasis="step"
          label={props.labels.previous}
          disabled={idle}
          onClick={() => props.onCommand("previous")}
        >
          <SkipBack />
        </TransportButton>
        <TransportButton
          emphasis="primary"
          label={playbackActive ? props.labels.pause : props.labels.play}
          disabled={idle}
          onClick={() => props.onCommand("toggle")}
        >
          {loading ? (
            <Loader2 className="listen-loading-spinner" />
          ) : playing ? (
            <Pause />
          ) : (
            <Play />
          )}
        </TransportButton>
        <TransportButton
          emphasis="step"
          label={props.labels.next}
          disabled={idle}
          onClick={() => props.onCommand("next")}
        >
          <SkipForward />
        </TransportButton>
        <TransportButton
          label={props.labels.repeatOne}
          disabled={idle}
          onClick={() => props.onCommand("repeat")}
        >
          <Repeat1 />
        </TransportButton>
      </div>

      <div className="app-workspace-transport__center group/track">
        <div className="app-workspace-transport__artwork">
          <Button
            type="button"
            variant="ghost"
            tone="neutral"
            shape="square"
            className="app-workspace-transport__artwork-open"
            aria-label={status.title}
            disabled={idle}
            onClick={props.onOpenPlayer}
          >
            <ListenSidebarArtwork status={status} />
          </Button>
          <Button
            type="button"
            variant="ghost"
            tone="neutral"
            shape="square"
            className="app-workspace-transport__fullscreen"
            aria-label={props.labels.fullscreen}
            disabled={idle}
            onClick={props.onFullscreen}
          >
            <Maximize2 />
          </Button>
        </div>
        <div className="app-workspace-transport__track-details">
          <span className="app-workspace-transport__title-row">
            <span className="app-workspace-transport__track-title">
              <span className="app-workspace-transport__title">{status.title}</span>
            </span>
            {onFavorite ? (
              <Button
                type="button"
                size="compactIcon"
                variant="ghost"
                shape="circle"
                tone={status.favoriteActive ? "accent" : "neutral"}
                className="app-workspace-transport__favorite"
                data-active={status.favoriteActive === true}
                aria-label={props.labels.favorite}
                aria-pressed={status.favoriteActive === true}
                title={props.labels.favorite}
                onClick={onFavorite}
              >
                <Heart />
              </Button>
            ) : null}
          </span>
          <span
            className="app-workspace-transport__artists"
            aria-label={status.subtitle || status.title}
          >
            {artistParts.map((part, index) =>
              part.kind === "separator" ? (
                <span
                  aria-hidden="true"
                  className="app-workspace-transport__artist-separator"
                  key={`separator:${index}`}
                >
                  {part.text}
                </span>
              ) : onOpenArtist ? (
                <Button
                  type="button"
                  variant="link"
                  tone="neutral"
                  className="app-workspace-transport__track-artist-open"
                  aria-label={part.artist.name}
                  key={`artist:${part.artist.browseId ?? part.artist.name}:${index}`}
                  onClick={() => onOpenArtist(part.artist)}
                >
                  <span className="app-workspace-transport__artist">
                    {part.artist.name}
                  </span>
                </Button>
              ) : (
                <span
                  className="app-workspace-transport__artist"
                  key={`artist:${part.artist.browseId ?? part.artist.name}:${index}`}
                >
                  {part.artist.name}
                </span>
              ),
            )}
          </span>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              size="compactIcon"
              variant="ghost"
              tone="neutral"
              className="app-workspace-transport__icon-button app-workspace-transport__more"
              aria-label={props.labels.more}
              title={props.labels.more}
              disabled={!moreMenuAvailable}
            >
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            side="top"
            align="center"
            className="app-workspace-transport__more-menu"
          >
            <DropdownMenuItem disabled={!onDownload} onSelect={onDownload}>
              <Download className="h-4 w-4 shrink-0" />
              <span>{props.labels.download}</span>
            </DropdownMenuItem>
            <DropdownMenuItem disabled={!onFavorite} onSelect={onFavorite}>
              <Heart
                className="app-workspace-transport__menu-favorite h-4 w-4 shrink-0"
                data-active={status.favoriteActive ? "true" : "false"}
              />
              <span>{props.labels.favorite}</span>
            </DropdownMenuItem>
            <DropdownMenuItem disabled={!onOpenURL} onSelect={onOpenURL}>
              <ExternalLink className="h-4 w-4 shrink-0" />
              <span>{props.labels.openURL}</span>
            </DropdownMenuItem>
            {onOpenArtist && menuArtists.length > 0 ? (
              <DropdownMenuSeparator />
            ) : null}
            {onOpenArtist
              ? menuArtists.map((artist, index) => (
                  <DropdownMenuItem
                    key={`open-artist:${artist.browseId ?? artist.name}:${index}`}
                    onSelect={() => onOpenArtist(artist)}
                  >
                    <UserRound className="h-4 w-4 shrink-0" />
                    <span className="min-w-0 max-w-[15rem] truncate">
                      {artist.name}
                    </span>
                  </DropdownMenuItem>
                ))
              : null}
          </DropdownMenuContent>
        </DropdownMenu>
        <div
          className="app-workspace-transport__timeline"
          data-live={live ? "true" : "false"}
          role={timelineProgressbar ? "progressbar" : undefined}
          aria-hidden={timelineDecorative ? true : undefined}
          aria-label={
            !timelineProgressbar
              ? undefined
              : live
                ? `${status.title} · ${props.labels.live}`
                : status.title
          }
          aria-valuemin={timelineProgressbar ? 0 : undefined}
          aria-valuemax={
            timelineProgressbar ? (live ? 100 : duration) : undefined
          }
          aria-valuenow={
            timelineProgressbar ? (live ? 100 : currentTime) : undefined
          }
          aria-valuetext={
            timelineProgressbar && live
              ? `${props.labels.live} · -0:00`
              : undefined
          }
        >
          {seekable ? (
            <input
              className="app-workspace-transport__timeline-input"
              type="range"
              min={0}
              max={duration}
              step={1}
              value={currentTime}
              aria-label={status.title}
              onChange={(event) =>
                props.onCommand("seek", Number(event.currentTarget.value))
              }
            />
          ) : null}
          <span className="app-workspace-transport__times" aria-hidden="true">
            <span className="app-workspace-transport__time app-workspace-transport__time--current">
              {formatTransportTime(currentTime)}
            </span>
            <span className="app-workspace-transport__time app-workspace-transport__time--remaining">
              -{formatTransportTime(remaining)}
            </span>
          </span>
          <span className="app-workspace-transport__track">
            <span style={{ width: `${progress}%` }} />
          </span>
        </div>
      </div>

      <div
        ref={volumeRegionRef}
        className="app-workspace-transport__right"
        data-volume-expanded={volumeControlsAvailable && volumeExpanded}
        data-volume-dragging={volumeEditor.dragging ? "true" : "false"}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            setVolumeExpanded(false);
            dispatchVolumeEditor({ type: "drag-end" });
            requestAnimationFrame(() => volumeTriggerRef.current?.focus());
          }
        }}
      >
        <div className="app-workspace-transport__right-actions">
          <TransportButton
            label={props.labels.download}
            disabled={!onDownload}
            onClick={() => onDownload?.()}
          >
            <Download />
          </TransportButton>
          <TransportButton
            label={props.labels.lyrics}
            disabled={idle}
            onClick={props.onOpenLyrics}
          >
            <MessageSquareQuote />
          </TransportButton>
          <TransportButton
            label={props.labels.upNext}
            disabled={idle}
            onClick={props.onOpenQueue}
          >
            <ListEnd />
          </TransportButton>
          <TransportButton
            label={props.labels.volume}
            buttonRef={volumeTriggerRef}
            disabled={!volumeControlsAvailable}
            onClick={() => {
              if (volumeControlsAvailable) {
                dispatchVolumeEditor({
                  type: "sync",
                  volume: normalizedVolume,
                });
                setVolumeExpanded(true);
              }
            }}
          >
            {props.muted ? <VolumeX /> : <Volume2 />}
          </TransportButton>
        </div>
        {volumeControlsAvailable ? (
          <div className="app-workspace-transport__volume-editor">
            <TransportButton
              label={props.labels.volume}
              pressed={props.muted}
              onClick={props.onToggleMute!}
            >
              {props.muted ? <VolumeX /> : <Volume2 />}
            </TransportButton>
            <input
              ref={volumeSliderRef}
              type="range"
              min={0}
              max={100}
              step={1}
              value={Math.round(volumeEditor.draft * 100)}
              aria-label={props.labels.volume}
              aria-valuetext={`${Math.round(volumeEditor.draft * 100)}%`}
              onPointerDown={() =>
                dispatchVolumeEditor({ type: "drag-start" })
              }
              onPointerUp={() =>
                dispatchVolumeEditor({ type: "drag-end" })
              }
              onPointerCancel={() =>
                dispatchVolumeEditor({ type: "drag-end" })
              }
              onInput={(event) => {
                const nextVolume = Number(event.currentTarget.value) / 100;
                dispatchVolumeEditor({ type: "input", volume: nextVolume });
                props.onVolumeChange!(nextVolume);
              }}
              onChange={(event) =>
                dispatchVolumeEditor({
                  type: "input",
                  volume: Number(event.currentTarget.value) / 100,
                })
              }
            />
          </div>
        ) : null}
      </div>
    </GlassGroup>
  );
}

function TransportButton(props: {
  label: string;
  emphasis?: "standard" | "step" | "primary";
  pressed?: boolean;
  disabled?: boolean;
  buttonRef?: React.Ref<HTMLButtonElement>;
  onClick: () => void;
  children: React.ReactNode;
}) {
  const emphasis = props.emphasis ?? "standard";
  return (
    <Button
      ref={props.buttonRef}
      type="button"
      size="compactIcon"
      variant="ghost"
      tone="neutral"
      aria-label={props.label}
      aria-pressed={props.pressed}
      data-transport-emphasis={emphasis}
      disabled={props.disabled}
      title={props.label}
      className={cn(
        "app-workspace-transport__icon-button app-workspace-transport__button",
        emphasis === "primary" && "app-workspace-transport__button--primary",
      )}
      onClick={props.onClick}
    >
      {props.children}
    </Button>
  );
}

function formatTransportTime(seconds: number) {
  const normalized = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  const hours = Math.floor(normalized / 3600);
  const minutes = Math.floor((normalized % 3600) / 60);
  const remainder = normalized % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`
    : `${minutes}:${String(remainder).padStart(2, "0")}`;
}
