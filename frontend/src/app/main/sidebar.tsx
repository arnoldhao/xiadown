import {
Loader2,
Pause,
Play,
SkipBack,
SkipForward,
X
} from "lucide-react";
import * as React from "react";

import {
type DreamFMNowPlayingStatus
} from "@/app/main/DreamFM";
import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import { Button } from "@/shared/ui/button";
import { Tooltip,TooltipContent,TooltipTrigger } from "@/shared/ui/tooltip";
import {
DREAM_FM_MINI_PRIMARY_CONTROL_CLASS,
DREAM_FM_MINI_SIDE_CONTROL_CLASS,
DREAM_FM_NOW_PLAYING_PANEL_CLASS,
} from "@/shared/styles/dreamfm";
import {
MAIN_SIDEBAR_ACTION_CLASS,
resolveXiaMainSidebarSurface,
} from "@/shared/styles/xiadown";

type DreamFMNowPlayingControlCommand = "previous" | "toggle" | "next";

export const resolveSidebarSurface = resolveXiaMainSidebarSurface;

export type SidebarIconButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  label: string;
  active?: boolean;
  children: React.ReactNode;
};

export const SidebarIconButton = React.forwardRef<
  HTMLButtonElement,
  SidebarIconButtonProps
>(function SidebarIconButton(
  { label, active, className, children, "aria-label": ariaLabel, ...props },
  ref,
) {
  return (
    <Button
      ref={ref}
      type="button"
      variant="ghost"
      size="icon"
      className={cn(
        "app-main-sidebar-action",
        MAIN_SIDEBAR_ACTION_CLASS,
        "relative border border-transparent bg-transparent text-sidebar-foreground/72 transition [&_svg]:!h-[var(--app-main-sidebar-icon-size)] [&_svg]:!w-[var(--app-main-sidebar-icon-size)]",
        active
          ? "bg-sidebar-accent text-sidebar-primary shadow-sm"
          : "hover:bg-sidebar-accent/75 hover:text-sidebar-accent-foreground",
        className,
      )}
      data-active={active ? "true" : undefined}
      aria-label={ariaLabel ?? label}
      {...props}
    >
      {children}
    </Button>
  );
});

export function dreamFMStatusLabel(
  status: DreamFMNowPlayingStatus | null,
  text: ReturnType<typeof getXiaText>,
) {
  switch (status?.state) {
    case "idle":
      return text.dreamFm.idleStatus;
    case "playing":
      return text.dreamFm.playingStatus;
    case "paused":
      return text.dreamFm.pausedStatus;
    case "loading":
      return text.dreamFm.loadingStatus;
    case "error":
      return text.dreamFm.errorStatus;
    default:
      return text.views.dreamFM;
  }
}

function resolveDreamFMProgress(status: DreamFMNowPlayingStatus) {
  const duration = Number.isFinite(status.progress.duration)
    ? Math.max(0, status.progress.duration)
    : 0;
  if (duration <= 0) {
    return null;
  }

  const currentTime = Number.isFinite(status.progress.currentTime)
    ? Math.max(0, Math.min(status.progress.currentTime, duration))
    : 0;
  const bufferedTime = Number.isFinite(status.progress.bufferedTime)
    ? Math.max(0, Math.min(status.progress.bufferedTime, duration))
    : 0;

  return {
    currentTime,
    duration,
    progressPercent: (currentTime / duration) * 100,
    bufferedPercent: (bufferedTime / duration) * 100,
  };
}

function resolveMiniPanelText(
  status: DreamFMNowPlayingStatus | null,
  text: ReturnType<typeof getXiaText>,
) {
  if (!status || status.state === "idle") {
    return {
      title: text.views.dreamFM,
      subtitle: text.dreamFm.idleSubtitle,
    };
  }

  return {
    title: status.title.trim() || text.dreamFm.nowPlaying,
    subtitle: status.subtitle.trim() || text.dreamFm.nowPlaying,
  };
}

function DreamFMNowPlayingPanelArtwork(props: {
  status: DreamFMNowPlayingStatus | null;
}) {
  if (!props.status || props.status.state === "idle") {
    return (
      <img
        src={DEFAULT_COVER_IMAGE_URL}
        alt=""
        className="h-full w-full object-cover"
        loading="lazy"
      />
    );
  }

  return <DreamFMSidebarArtwork status={props.status} />;
}

function DreamFMNowPlayingPanelTransport(props: {
  status: DreamFMNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  onControlCommand?: (command: DreamFMNowPlayingControlCommand) => void;
}) {
  const state = props.status?.state ?? "idle";
  const canControl = Boolean(
    props.onControlCommand &&
      props.status?.canControl &&
      state !== "idle" &&
      state !== "loading",
  );
  const isPlaying = state === "playing";
  const playLabel = isPlaying ? props.text.dreamFm.pause : props.text.dreamFm.play;

  return (
    <div className="flex h-9 items-center justify-center gap-1.5">
      <button
        type="button"
        className={DREAM_FM_MINI_SIDE_CONTROL_CLASS}
        aria-label={props.text.dreamFm.previous}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("previous")}
      >
        <SkipBack className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        className={DREAM_FM_MINI_PRIMARY_CONTROL_CLASS}
        aria-label={state === "loading" ? props.text.dreamFm.loading : playLabel}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("toggle")}
      >
        {state === "loading" ? (
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
        ) : isPlaying ? (
          <Pause className="h-3.5 w-3.5 fill-current" />
        ) : (
          <Play className="ml-0.5 h-3.5 w-3.5 fill-current" />
        )}
      </button>
      <button
        type="button"
        className={DREAM_FM_MINI_SIDE_CONTROL_CLASS}
        aria-label={props.text.dreamFm.next}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("next")}
      >
        <SkipForward className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

function DreamFMNowPlayingPanelProgress(props: {
  status: DreamFMNowPlayingStatus | null;
}) {
  const progress =
    props.status && props.status.state !== "idle"
      ? resolveDreamFMProgress(props.status)
      : null;

  return (
    <div className="flex h-[18px] items-center">
      <div
        className="relative h-1.5 w-full overflow-hidden rounded-full bg-[hsl(var(--tray-control-foreground)/0.16)]"
        role={progress ? "progressbar" : undefined}
        aria-valuemin={progress ? 0 : undefined}
        aria-valuemax={progress ? Math.round(progress.duration) : undefined}
        aria-valuenow={progress ? Math.round(progress.currentTime) : undefined}
      >
        {progress ? (
          <>
            <span
              aria-hidden="true"
              className="absolute inset-y-0 left-0 rounded-full bg-[hsl(var(--tray-control-foreground)/0.22)] transition-[width] duration-300"
              style={{ width: `${progress.bufferedPercent}%` }}
            />
            <span
              aria-hidden="true"
              className="absolute inset-y-0 left-0 rounded-full bg-[hsl(var(--tray-control-foreground))] transition-[width] duration-150"
              style={{ width: `${progress.progressPercent}%` }}
            />
          </>
        ) : props.status?.state === "loading" ? (
          <span
            aria-hidden="true"
            className="absolute inset-y-0 left-0 w-1/3 animate-pulse rounded-full bg-[hsl(var(--tray-control-foreground)/0.34)]"
          />
        ) : null}
      </div>
    </div>
  );
}

export function DreamFMSidebarSourceBadge(props: {
  status: DreamFMNowPlayingStatus | null;
}) {
  switch (props.status?.state) {
    case "loading":
      return (
        <span className="pointer-events-none absolute right-1 top-1 flex h-3.5 w-3.5 items-center justify-center rounded-full border border-sidebar-background bg-sidebar-background/92 text-sidebar-foreground shadow-sm backdrop-blur-sm">
          <span className="h-2 w-2 animate-spin rounded-full border border-sidebar-foreground/30 border-t-sidebar-foreground/75" />
        </span>
      );
    case "playing":
      return (
        <span className="pointer-events-none absolute right-1 top-1 flex h-3.5 w-3.5 items-end justify-center gap-[1px] rounded-full border border-sidebar-background bg-primary/18 px-0.5 pb-0.5 shadow-sm backdrop-blur-sm">
          <span className="h-1 w-0.5 animate-pulse rounded-full bg-primary" />
          <span className="h-2 w-0.5 animate-pulse rounded-full bg-primary [animation-delay:120ms]" />
          <span className="h-1.5 w-0.5 animate-pulse rounded-full bg-primary [animation-delay:240ms]" />
        </span>
      );
    case "paused":
      return (
        <span className="pointer-events-none absolute right-1 top-1 flex h-3.5 w-3.5 items-center justify-center gap-[2px] rounded-full border border-sidebar-background bg-sidebar-background/92 shadow-sm backdrop-blur-sm">
          <span className="h-2 w-[2px] rounded-full bg-sidebar-foreground/75" />
          <span className="h-2 w-[2px] rounded-full bg-sidebar-foreground/75" />
        </span>
      );
    case "error":
      return (
        <span className="pointer-events-none absolute right-1 top-1 flex h-3.5 w-3.5 items-center justify-center rounded-full border border-sidebar-background bg-destructive/18 shadow-sm backdrop-blur-sm">
          <span className="absolute h-2 w-[2px] rotate-45 rounded-full bg-destructive" />
          <span className="absolute h-2 w-[2px] -rotate-45 rounded-full bg-destructive" />
        </span>
      );
    default:
      return null;
  }
}

export function DreamFMNowPlayingHoverPanel(props: {
  status: DreamFMNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  className?: string;
  onControlCommand?: (command: DreamFMNowPlayingControlCommand) => void;
}) {
  const text = resolveMiniPanelText(props.status, props.text);

  return (
    <div
      className={cn(
        DREAM_FM_NOW_PLAYING_PANEL_CLASS,
        props.className,
      )}
      aria-label={`${props.text.dreamFm.nowPlaying}: ${text.title}`}
    >
      <div className="relative grid h-full min-w-0 grid-cols-2 overflow-hidden rounded-[21px]">
        <div className="relative min-w-0 overflow-visible">
          <div className="absolute inset-y-[-26px] left-[-30px] w-[calc(100%+118px)] opacity-72 blur-[38px] saturate-[1.55] contrast-[1.12] [mask-image:linear-gradient(90deg,#000_0%,rgba(0,0,0,0.82)_42%,rgba(0,0,0,0.32)_72%,transparent_100%)] [-webkit-mask-image:linear-gradient(90deg,#000_0%,rgba(0,0,0,0.82)_42%,rgba(0,0,0,0.32)_72%,transparent_100%)]">
            <DreamFMNowPlayingPanelArtwork status={props.status} />
          </div>
          <div className="absolute inset-y-0 left-0 z-[1] w-[calc(100%+42px)] overflow-hidden [mask-image:linear-gradient(90deg,#000_0%,#000_64%,rgba(0,0,0,0.72)_84%,transparent_100%)] [-webkit-mask-image:linear-gradient(90deg,#000_0%,#000_64%,rgba(0,0,0,0.72)_84%,transparent_100%)]">
            <DreamFMNowPlayingPanelArtwork status={props.status} />
          </div>
        </div>
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 z-10 bg-[linear-gradient(90deg,transparent_0%,transparent_44%,hsl(var(--tray-control-color)/0.14)_54%,hsl(var(--tray-control-color)/0.62)_68%,hsl(var(--tray-control-color))_86%)]"
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-y-0 left-[44%] z-10 w-[30%]"
          style={{
            backdropFilter: "blur(8px) saturate(1.08)",
            WebkitBackdropFilter: "blur(8px) saturate(1.08)",
            background:
              "linear-gradient(90deg,transparent 0%,hsl(var(--tray-control-color)/0.18) 42%,hsl(var(--tray-control-color)/0.42) 100%)",
            maskImage:
              "linear-gradient(90deg,transparent 0%,rgba(0,0,0,0.28) 22%,rgba(0,0,0,0.78) 64%,transparent 100%)",
            WebkitMaskImage:
              "linear-gradient(90deg,transparent 0%,rgba(0,0,0,0.28) 22%,rgba(0,0,0,0.78) 64%,transparent 100%)",
          }}
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute bottom-0 right-0 z-10 h-[58%] w-[74%] bg-[linear-gradient(0deg,hsl(var(--tray-control-color)/0.88)_0%,hsl(var(--tray-control-color)/0.42)_46%,transparent_100%)] [mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.18)_20%,#000_48%,#000_100%)] [-webkit-mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.18)_20%,#000_48%,#000_100%)]"
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 z-10 opacity-[0.12] mix-blend-overlay [mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.35)_28%,#000_54%,#000_100%)] [-webkit-mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.35)_28%,#000_54%,#000_100%)]"
          style={{
            backgroundImage: [
              "repeating-radial-gradient(circle at 0 0,rgba(255,255,255,0.2)_0_0.55px,transparent_0.8px_3.8px)",
              "repeating-radial-gradient(circle at 1px 1px,rgba(0,0,0,0.14)_0_0.45px,transparent_0.7px_5.6px)",
            ].join(","),
            backgroundSize: "7px 7px, 11px 11px",
          }}
        />
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 z-30 rounded-[21px] ring-1 ring-inset ring-[hsl(var(--tray-control-foreground)/0.12)]"
        />
        <div className="relative z-20 col-start-2 flex h-full min-w-0 flex-col px-2.5 py-2.5">
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center text-center">
            <div className="max-w-full truncate text-[13px] font-semibold leading-5">
              {text.title}
            </div>
            <div className="mt-0.5 max-w-full truncate text-[11px] font-medium leading-4 text-[hsl(var(--tray-control-foreground)/0.72)]">
              {text.subtitle}
            </div>
          </div>
          <DreamFMNowPlayingPanelTransport
            status={props.status}
            text={props.text}
            onControlCommand={props.onControlCommand}
          />
          <DreamFMNowPlayingPanelProgress status={props.status} />
        </div>
      </div>
    </div>
  );
}

export function DreamFMSidebarArtwork(props: { status: DreamFMNowPlayingStatus }) {
  const [failedURL, setFailedURL] = React.useState("");
  const source =
    props.status.artworkURL && props.status.artworkURL !== failedURL
      ? props.status.artworkURL
      : DEFAULT_COVER_IMAGE_URL;

  React.useEffect(() => {
    setFailedURL("");
  }, [props.status.artworkURL]);

  return (
    <img
      src={source}
      alt=""
      className="h-full w-full object-cover"
      loading="lazy"
      onError={() => setFailedURL(source)}
    />
  );
}

export function DreamFMNowPlayingMiniPlayer(props: {
  status: DreamFMNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  onOpen: () => void;
  onToggle: () => void;
  onControlCommand?: (command: DreamFMNowPlayingControlCommand) => void;
}) {
  if (!props.status || props.status.state === "idle") {
    return null;
  }
  const statusLabel = dreamFMStatusLabel(props.status, props.text);
  const canToggle = Boolean(
    props.status?.canControl && props.status.state !== "loading",
  );
  const isPlaying = props.status?.state === "playing";

  return (
    <div className="wails-no-drag group/dreamfm-mini relative flex w-[var(--app-main-sidebar-action-size)] flex-col items-center gap-1.5 rounded-2xl border border-sidebar-border/70 bg-sidebar-accent/45 p-1.5 shadow-sm backdrop-blur-xl">
      <button
        type="button"
        className="relative h-10 w-10 overflow-hidden rounded-xl bg-background/80 text-sidebar-foreground outline-none ring-1 ring-sidebar-border/70 transition hover:scale-[1.03] focus-visible:ring-2 focus-visible:ring-sidebar-ring/45"
        aria-label={`${props.text.dreamFm.nowPlaying}: ${props.status.title}`}
        onClick={props.onOpen}
      >
        <DreamFMSidebarArtwork status={props.status} />
        <span className="absolute inset-0 bg-gradient-to-t from-black/40 via-black/8 to-transparent" />
        {props.status.state === "playing" ? (
          <span className="absolute inset-x-1 bottom-1 flex items-end justify-center gap-[2px] rounded-full bg-black/24 px-1 py-0.5 backdrop-blur-sm">
            <span className="h-1.5 w-0.5 animate-pulse rounded-full bg-white" />
            <span className="h-2.5 w-0.5 animate-pulse rounded-full bg-white [animation-delay:120ms]" />
            <span className="h-2 w-0.5 animate-pulse rounded-full bg-white [animation-delay:240ms]" />
          </span>
        ) : null}
      </button>

      <div className="pointer-events-none absolute left-[calc(100%+12px)] top-1/2 z-50 hidden -translate-y-1/2 group-focus-within/dreamfm-mini:block group-hover/dreamfm-mini:block">
        <div className="pointer-events-auto">
          <DreamFMNowPlayingHoverPanel
            status={props.status}
            text={props.text}
            onControlCommand={props.onControlCommand}
          />
        </div>
      </div>

      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className="flex h-7 w-7 items-center justify-center rounded-full border border-sidebar-border/70 bg-sidebar-background/90 text-sidebar-foreground shadow-sm transition hover:bg-sidebar-accent focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-55"
            aria-label={
              isPlaying ? props.text.dreamFm.pause : props.text.dreamFm.play
            }
            disabled={!canToggle}
            onClick={props.onToggle}
          >
            {props.status.state === "loading" ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : isPlaying ? (
              <Pause className="h-3.5 w-3.5" />
            ) : props.status.state === "error" ? (
              <X className="h-3.5 w-3.5" />
            ) : (
              <Play className="h-3.5 w-3.5 translate-x-px" />
            )}
          </button>
        </TooltipTrigger>
        <TooltipContent side="right">{statusLabel}</TooltipContent>
      </Tooltip>
    </div>
  );
}
