import {
CassetteTape,
Loader2,
Pause,
Play,
Radar,
SkipBack,
SkipForward,
X
} from "lucide-react";
import * as React from "react";

import {
type ListenNowPlayingStatus
} from "@/app/main/Listen";
import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { LISTEN_DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { CDPBrowserStatus } from "@/shared/contracts/library";
import { Button } from "@/shared/ui/button";
import { Tooltip,TooltipContent,TooltipTrigger } from "@/shared/ui/tooltip";
import {
LISTEN_MINI_PRIMARY_CONTROL_CLASS,
LISTEN_MINI_SIDE_CONTROL_CLASS,
LISTEN_NOW_PLAYING_PANEL_CLASS,
} from "@/shared/styles/listen";
import {
MAIN_SIDEBAR_ACTION_CLASS,
resolveXiaMainSidebarSurface,
} from "@/shared/styles/xiadown";

type ListenNowPlayingControlCommand = "previous" | "toggle" | "next";
type ListenMiniPanelVariant = "hush" | "timeline";
export type ListenNowPlayingPanelSurface = "white" | "dark" | "tray";

export const resolveSidebarSurface = resolveXiaMainSidebarSurface;

function formatTemplate(template: string, params: Record<string, string>) {
  return Object.entries(params).reduce(
    (output, [key, value]) => output.split(`{${key}}`).join(value),
    template,
  );
}

function normalizeStatus(value?: string) {
  return (value ?? "").trim().toLowerCase();
}

function resolveCDPBrowserStatusLabel(
  text: ReturnType<typeof getXiaText>,
  status?: string,
) {
  switch (normalizeStatus(status)) {
    case "open":
      return text.sniffDesk.statusOpen;
    case "closing":
      return text.sniffDesk.statusClosing;
    case "tab_closed":
      return text.sniffDesk.statusTabClosed;
    case "browser_closed":
      return text.sniffDesk.statusClosed;
    default:
      return text.common.unknown;
  }
}

export type SidebarIconButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  label: string;
  active?: boolean;
  children: React.ReactNode;
};

export const SidebarIconButton = React.forwardRef<
  HTMLButtonElement,
  SidebarIconButtonProps
>(function SidebarIconButton(
  {
    label,
    active,
    className,
    children,
    "aria-current": ariaCurrent,
    "aria-label": ariaLabel,
    ...props
  },
  ref,
) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
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
          aria-current={ariaCurrent ?? (active ? "page" : undefined)}
          aria-label={ariaLabel ?? label}
          {...props}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
  );
});

export function CDPBrowserStatusMiniButton(props: {
  status: CDPBrowserStatus | null | undefined;
  text: ReturnType<typeof getXiaText>;
  active?: boolean;
  stopping?: boolean;
  onOpenSniffDesk: () => void;
  onCloseOrphan: (runtimeId: string) => void;
}) {
  const status = props.status;
  if (!status?.active) {
    return null;
  }
  const isOrphan = status.mode === "orphan";
  const label = isOrphan
    ? props.text.sniffDesk.cdpOrphan
    : props.text.sniffDesk.cdpStatus;
  const actionLabel = isOrphan
    ? props.text.sniffDesk.cdpClose
    : props.text.sniffDesk.title;
  const title =
    status.title ||
    status.session?.title ||
    status.session?.unoptimizedDomain ||
    label;
  const currentURL =
    status.currentUrl || status.session?.currentUrl || status.session?.url || "";
  const tabText =
    typeof status.tabCount === "number" && status.tabCount > 0
      ? formatTemplate(props.text.sniffDesk.tabCount, {
          count: String(status.tabCount),
        })
      : "";
  const processText =
    typeof status.processCount === "number" && status.processCount > 0
      ? formatTemplate(props.text.sniffDesk.cdpProcessCount, {
          count: String(status.processCount),
        })
      : "";
  const pidText =
    typeof status.pid === "number" && status.pid > 0
      ? formatTemplate(props.text.sniffDesk.cdpPid, {
          pid: String(status.pid),
        })
      : "";
  const browserStatusText = resolveCDPBrowserStatusLabel(
    props.text,
    status.browserStatus,
  );

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={cn(
            "app-cdp-status-button app-main-sidebar-action",
            MAIN_SIDEBAR_ACTION_CLASS,
            "relative border border-transparent bg-transparent text-sidebar-foreground/72 transition",
            props.active
              ? "bg-sidebar-accent text-sidebar-primary shadow-sm"
              : "hover:bg-sidebar-accent/75 hover:text-sidebar-accent-foreground",
            isOrphan && "app-cdp-status-button-orphan",
          )}
          data-active={props.active ? "true" : undefined}
          data-mode={isOrphan ? "orphan" : "resource-sniff"}
          aria-label={label}
          disabled={props.stopping}
          onClick={() => {
            if (isOrphan && status.runtimeId) {
              props.onCloseOrphan(status.runtimeId);
              return;
            }
            props.onOpenSniffDesk();
          }}
        >
          {props.stopping ? (
            <Loader2 className="h-[var(--app-main-sidebar-icon-size)] w-[var(--app-main-sidebar-icon-size)] animate-spin" />
          ) : isOrphan ? (
            <X className="h-[var(--app-main-sidebar-icon-size)] w-[var(--app-main-sidebar-icon-size)]" />
          ) : (
            <Radar className="h-[var(--app-main-sidebar-icon-size)] w-[var(--app-main-sidebar-icon-size)]" />
          )}
          <span className="app-cdp-status-dot pointer-events-none absolute right-2.5 top-2.5 z-[2] h-2 w-2 rounded-full" />
        </Button>
      </TooltipTrigger>
      <TooltipContent
        side="right"
        multiline
        className="app-cdp-status-tooltip min-w-[15rem] max-w-[22rem] px-3 py-2 text-left"
      >
        <div className="space-y-1.5">
          <div className="flex min-w-0 items-center justify-between gap-3 text-[11px] font-semibold text-background">
            <span className="min-w-0 truncate">{label}</span>
            <span className="min-w-0 max-w-[10rem] truncate text-[10px] text-background/78">
              {actionLabel}
            </span>
          </div>
          <div className="truncate text-[11px] font-medium text-background/86">
            {title}
          </div>
          {currentURL ? (
            <div className="line-clamp-2 break-all text-[10px] font-medium text-background/72">
              {currentURL}
            </div>
          ) : null}
          <div className="flex flex-wrap gap-1.5 text-[10px] text-background/70">
            <span>{browserStatusText}</span>
            {tabText ? <span>{tabText}</span> : null}
            {processText ? <span>{processText}</span> : null}
            {pidText ? <span>{pidText}</span> : null}
          </div>
        </div>
      </TooltipContent>
    </Tooltip>
  );
}

export function listenStatusLabel(
  status: ListenNowPlayingStatus | null,
  text: ReturnType<typeof getXiaText>,
) {
  switch (status?.state) {
    case "idle":
      return text.listen.idleStatus;
    case "playing":
      return text.listen.playingStatus;
    case "paused":
      return text.listen.pausedStatus;
    case "loading":
      return text.listen.loadingStatus;
    case "error":
      return text.listen.errorStatus;
    default:
      return text.views.listen;
  }
}

function resolveListenProgress(status: ListenNowPlayingStatus) {
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

function resolveListenMiniPanelVariant(
  status: ListenNowPlayingStatus | null,
): ListenMiniPanelVariant {
  return status?.mode === "hush" ? "hush" : "timeline";
}

function renderListenMiniControlIcon(
  state: ListenNowPlayingStatus["state"],
  isPlaying: boolean,
) {
  if (state === "loading") {
    return <Loader2 className="h-3.5 w-3.5 animate-spin" />;
  }
  if (state === "error") {
    return <X className="h-3.5 w-3.5" />;
  }
  if (isPlaying) {
    return <Pause className="h-3.5 w-3.5 fill-current" />;
  }
  return <Play className="ml-0.5 h-3.5 w-3.5 fill-current" />;
}

function ListenMiniScrollingText(props: {
  text: string;
  className?: string;
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
          12,
          Math.max(6, (overflow + 150) / 28),
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
  }, [normalizedText]);

  return (
    <div
      ref={containerRef}
      className={cn(
        "listen-mini-marquee relative block max-w-full min-w-0 overflow-hidden whitespace-nowrap",
        scrolling ? "text-left" : "text-center",
        props.className,
      )}
      data-overflow={scrolling ? "true" : "false"}
      title={normalizedText}
    >
      <span
        ref={contentRef}
        className={cn(
          "inline-block max-w-none align-top",
          scrolling ? "listen-marquee-text pr-4" : "max-w-full truncate",
        )}
        style={style}
      >
        {normalizedText}
      </span>
    </div>
  );
}

function resolveMiniPanelText(
  status: ListenNowPlayingStatus | null,
  text: ReturnType<typeof getXiaText>,
) {
  if (!status || status.state === "idle") {
    return {
      title: text.views.listen,
      subtitle: text.listen.idleSubtitle,
    };
  }

  return {
    title: status.title.trim() || text.listen.nowPlaying,
    subtitle: status.subtitle.trim() || text.listen.nowPlaying,
  };
}

function ListenNowPlayingPanelArtwork(props: {
  status: ListenNowPlayingStatus | null;
}) {
  if (!props.status || props.status.state === "idle") {
    return (
      <img
        src={LISTEN_DEFAULT_COVER_IMAGE_URL}
        alt=""
        className="h-full w-full object-cover"
        loading="lazy"
      />
    );
  }

  return <ListenSidebarArtwork status={props.status} />;
}

function ListenNowPlayingPanelTransport(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  onControlCommand?: (command: ListenNowPlayingControlCommand) => void;
}) {
  const state = props.status?.state ?? "idle";
  const variant = resolveListenMiniPanelVariant(props.status);
  const canControl = Boolean(
    props.onControlCommand &&
      props.status?.canControl &&
      state !== "idle" &&
      state !== "loading",
  );
  const isPlaying = state === "playing";
  const playLabel = isPlaying ? props.text.listen.pause : props.text.listen.play;
  const primaryLabel =
    state === "loading"
      ? props.text.listen.loading
      : state === "error"
        ? props.text.listen.errorStatus
        : playLabel;

  if (variant === "hush") {
    return (
      <div
        className="listen-mini-transport flex h-9 items-center justify-center gap-1.5"
        data-variant="hush"
      >
        <button
          type="button"
          className={LISTEN_MINI_PRIMARY_CONTROL_CLASS}
          aria-label={primaryLabel}
          disabled={!canControl}
          onClick={() => props.onControlCommand?.("toggle")}
        >
          {renderListenMiniControlIcon(state, isPlaying)}
        </button>
      </div>
    );
  }

  return (
    <div
      className="listen-mini-transport flex h-9 items-center justify-center gap-1.5"
      data-variant="timeline"
    >
      <button
        type="button"
        className={LISTEN_MINI_SIDE_CONTROL_CLASS}
        aria-label={props.text.listen.previous}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("previous")}
      >
        <SkipBack className="h-3.5 w-3.5" />
      </button>
      <button
        type="button"
        className={LISTEN_MINI_PRIMARY_CONTROL_CLASS}
        aria-label={primaryLabel}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("toggle")}
      >
        {renderListenMiniControlIcon(state, isPlaying)}
      </button>
      <button
        type="button"
        className={LISTEN_MINI_SIDE_CONTROL_CLASS}
        aria-label={props.text.listen.next}
        disabled={!canControl}
        onClick={() => props.onControlCommand?.("next")}
      >
        <SkipForward className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}

function ListenNowPlayingPanelProgress(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
}) {
  const variant = resolveListenMiniPanelVariant(props.status);
  if (variant === "hush") {
    const state = props.status?.state ?? "idle";
    return (
      <div
        className="listen-mini-progress-row flex h-[18px] items-center"
        data-variant="hush"
      >
        <div
          className="listen-mini-live-progress relative flex h-3.5 w-full items-center overflow-hidden rounded-full"
          data-state={state}
          role="status"
          aria-label={listenStatusLabel(props.status, props.text)}
        >
          <span
            aria-hidden="true"
            className="listen-mini-live-line relative min-w-0 flex-1 rounded-full"
          />
          <span
            aria-hidden="true"
            className="listen-mini-live-dot absolute right-0 top-1/2 rounded-full"
          />
        </div>
      </div>
    );
  }

  const progress =
    props.status && props.status.state !== "idle"
      ? resolveListenProgress(props.status)
      : null;

  return (
    <div
      className="listen-mini-progress-row flex h-[18px] items-center"
      data-variant="timeline"
    >
      <div
        className="listen-mini-progress-track relative h-1 w-full overflow-hidden rounded-full bg-[hsl(var(--tray-control-foreground)/0.16)]"
        data-state={
          progress
            ? "ready"
            : props.status?.state === "loading"
              ? "loading"
              : "empty"
        }
        role={progress ? "progressbar" : undefined}
        aria-valuemin={progress ? 0 : undefined}
        aria-valuemax={progress ? Math.round(progress.duration) : undefined}
        aria-valuenow={progress ? Math.round(progress.currentTime) : undefined}
      >
        {progress ? (
          <>
            <span
              aria-hidden="true"
              className="listen-mini-progress-buffer absolute inset-y-0 left-0 rounded-full bg-[hsl(var(--tray-control-foreground)/0.22)] transition-[width] duration-300"
              style={{ width: `${progress.bufferedPercent}%` }}
            />
            <span
              aria-hidden="true"
              className="listen-mini-progress-value absolute inset-y-0 left-0 rounded-full bg-sidebar-primary transition-[width] duration-150"
              style={{ width: `${progress.progressPercent}%` }}
            />
          </>
        ) : props.status?.state === "loading" ? (
          <span
            aria-hidden="true"
            className="listen-mini-progress-loading absolute inset-y-0 left-0 w-1/3 animate-pulse rounded-full bg-sidebar-primary/45"
          />
        ) : null}
      </div>
    </div>
  );
}

export function ListenSidebarSourceBadge(props: {
  status: ListenNowPlayingStatus | null;
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

export function ListenNowPlayingHoverPanel(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  className?: string;
  surface?: ListenNowPlayingPanelSurface;
  onControlCommand?: (command: ListenNowPlayingControlCommand) => void;
}) {
  const text = resolveMiniPanelText(props.status, props.text);
  const surface = props.surface ?? "white";

  return (
    <div
      className={cn(
        LISTEN_NOW_PLAYING_PANEL_CLASS,
        props.className,
      )}
      data-surface={surface}
      aria-label={`${props.text.listen.nowPlaying}: ${text.title}`}
    >
      <div className="listen-panel-layout relative grid h-full min-w-0 grid-cols-2 overflow-hidden rounded-[21px]">
        <div className="relative min-w-0 overflow-visible">
          <div className="listen-panel-artwork-glow absolute inset-y-[-26px] left-[-30px] w-[calc(100%+118px)] opacity-72 blur-[38px] saturate-[1.55] contrast-[1.12] [mask-image:linear-gradient(90deg,#000_0%,rgba(0,0,0,0.82)_42%,rgba(0,0,0,0.32)_72%,transparent_100%)] [-webkit-mask-image:linear-gradient(90deg,#000_0%,rgba(0,0,0,0.82)_42%,rgba(0,0,0,0.32)_72%,transparent_100%)]">
            <ListenNowPlayingPanelArtwork status={props.status} />
          </div>
          <div className="listen-panel-artwork-main absolute inset-y-0 left-0 z-[1] w-[calc(100%+42px)] overflow-hidden [mask-image:linear-gradient(90deg,#000_0%,#000_64%,rgba(0,0,0,0.72)_84%,transparent_100%)] [-webkit-mask-image:linear-gradient(90deg,#000_0%,#000_64%,rgba(0,0,0,0.72)_84%,transparent_100%)]">
            <ListenNowPlayingPanelArtwork status={props.status} />
          </div>
        </div>
        <div
          aria-hidden="true"
          className="listen-panel-color-wash pointer-events-none absolute inset-0 z-10"
        />
        <div
          aria-hidden="true"
          className="listen-panel-blur-veil pointer-events-none absolute inset-y-0 left-[44%] z-10 w-[30%]"
        />
        <div
          aria-hidden="true"
          className="listen-panel-bottom-vignette pointer-events-none absolute bottom-0 right-0 z-10 h-[58%] w-[74%] [mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.18)_20%,#000_48%,#000_100%)] [-webkit-mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.18)_20%,#000_48%,#000_100%)]"
        />
        <div
          aria-hidden="true"
          className="listen-panel-grain pointer-events-none absolute inset-0 z-10 opacity-[0.12] mix-blend-overlay [mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.35)_28%,#000_54%,#000_100%)] [-webkit-mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.35)_28%,#000_54%,#000_100%)]"
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
          className="listen-panel-ring pointer-events-none absolute inset-0 z-30 rounded-[21px]"
        />
        <div className="relative z-20 col-start-2 flex h-full min-w-0 flex-col px-2.5 py-2.5">
          <div className="flex min-h-0 flex-1 flex-col items-center justify-center text-center">
            <ListenMiniScrollingText
              text={text.title}
              className="text-[13px] font-semibold leading-5"
            />
            <ListenMiniScrollingText
              text={text.subtitle}
              className="mt-0.5 text-[11px] font-medium leading-4 text-[hsl(var(--tray-control-foreground)/0.72)]"
            />
          </div>
          <ListenNowPlayingPanelTransport
            status={props.status}
            text={props.text}
            onControlCommand={props.onControlCommand}
          />
          <ListenNowPlayingPanelProgress
            status={props.status}
            text={props.text}
          />
        </div>
      </div>
    </div>
  );
}

export function ListenSidebarArtwork(props: { status: ListenNowPlayingStatus }) {
  const [failedURL, setFailedURL] = React.useState("");
  const source =
    props.status.artworkURL && props.status.artworkURL !== failedURL
      ? props.status.artworkURL
      : LISTEN_DEFAULT_COVER_IMAGE_URL;

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

export function ListenNowPlayingMiniPlayer(props: {
  status: ListenNowPlayingStatus | null;
  text: ReturnType<typeof getXiaText>;
  active?: boolean;
  surface?: Exclude<ListenNowPlayingPanelSurface, "tray">;
  onOpen: () => void;
  onToggle: () => void;
  onControlCommand?: (command: ListenNowPlayingControlCommand) => void;
}) {
  if (!props.status || props.status.state === "idle") {
    return (
      <SidebarIconButton
        label={props.text.views.listen}
        active={props.active}
        onClick={props.onOpen}
        className="listen-sidebar-entry listen-sidebar-entry-idle"
      >
        <CassetteTape className="h-[var(--app-main-sidebar-icon-size)] w-[var(--app-main-sidebar-icon-size)]" />
      </SidebarIconButton>
    );
  }
  const statusLabel = listenStatusLabel(props.status, props.text);
  const canToggle = Boolean(
    props.status?.canControl && props.status.state !== "loading",
  );
  const isPlaying = props.status?.state === "playing";
  const toggleLabel =
    props.status.state === "loading"
      ? props.text.listen.loading
      : props.status.state === "error"
        ? props.text.listen.errorStatus
        : isPlaying
          ? props.text.listen.pause
          : props.text.listen.play;

  return (
    <div
      className={cn(
        "wails-no-drag group/listen-mini listen-sidebar-entry listen-mini-player relative flex w-[var(--app-main-sidebar-action-size)] flex-col items-center gap-1.5 rounded-2xl border border-sidebar-border/70 bg-sidebar-accent/45 p-1.5 shadow-sm backdrop-blur-xl",
        props.active && "bg-sidebar-accent text-sidebar-primary",
      )}
      data-active={props.active ? "true" : undefined}
    >
      <button
        type="button"
        className="listen-mini-artwork-button relative h-10 w-10 overflow-hidden rounded-xl bg-background/80 text-sidebar-foreground outline-none ring-1 ring-sidebar-border/70 transition hover:scale-[1.03] focus-visible:ring-2 focus-visible:ring-sidebar-ring/45"
        aria-label={`${props.text.listen.nowPlaying}: ${props.status.title}`}
        onClick={props.onOpen}
      >
        <ListenSidebarArtwork status={props.status} />
        <span className="listen-mini-artwork-overlay absolute inset-0 bg-gradient-to-t from-black/40 via-black/8 to-transparent" />
        {props.status.state === "playing" ? (
          <span className="absolute inset-x-1 bottom-1 flex items-end justify-center gap-[2px] rounded-full bg-black/24 px-1 py-0.5 backdrop-blur-sm">
            <span className="h-1.5 w-0.5 animate-pulse rounded-full bg-white" />
            <span className="h-2.5 w-0.5 animate-pulse rounded-full bg-white [animation-delay:120ms]" />
            <span className="h-2 w-0.5 animate-pulse rounded-full bg-white [animation-delay:240ms]" />
          </span>
        ) : null}
      </button>

      <div className="listen-mini-hover-panel-wrap absolute left-full top-1/2 z-50 -translate-y-1/2 pl-3">
        <div className="pointer-events-auto">
          <ListenNowPlayingHoverPanel
            status={props.status}
            text={props.text}
            surface={props.surface}
            onControlCommand={props.onControlCommand}
          />
        </div>
      </div>

      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className="listen-mini-toggle-button flex h-7 w-7 items-center justify-center rounded-full border border-sidebar-border/70 bg-sidebar-background/90 text-sidebar-foreground shadow-sm transition hover:bg-sidebar-accent focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-55"
            aria-label={toggleLabel}
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
