import {
  BrushCleaning,
  Download,
  Gauge,
  Loader2,
  Pause,
  Play,
  Radar,
  SkipBack,
  SkipForward,
  X,
} from "lucide-react";
import * as React from "react";

import type {
  ListenNowPlayingStatus,
  ListenPlaybackSource,
} from "@/app/main/Listen";
import { ListenPlayerSourceIcon } from "@/app/main/listen/workspace-player-shared";
import { ListenSidebarArtwork } from "@/app/main/sidebar";
import {
  resolveOperationThumbnailCoverURL,
  type OperationActivityItem,
  type OperationActivitySnapshot,
} from "@/shared/activity/operations";
import type { SniffStatusSnapshot } from "@/shared/activity/sniff";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { GlassSurface } from "@/shared/ui/glass-surface";
import { SiteFavicon } from "@/shared/ui/site-favicon";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

export type WorkspacePlaybackCommand = "previous" | "toggle" | "next";

type WorkspaceStatusTone =
  | "idle"
  | "busy"
  | "success"
  | "error"
  | "orphan";

type WorkspaceStatusSurfaceProps = Omit<
  React.ComponentProps<typeof GlassSurface>,
  "elevation" | "material" | "shape" | "surfaceRole" | "tint"
>;

export type WorkspaceActivityMenuAction = {
  key: string;
  label: string;
  icon: React.ReactNode;
  onSelect: () => void;
  disabled?: boolean;
  tone?: "default" | "destructive";
};

type WorkspaceActivityContextTargetProps = Pick<
  React.HTMLAttributes<HTMLElement>,
  "onContextMenu" | "onKeyDown"
>;

const WORKSPACE_ACTIVITY_FOCUSABLE_SELECTOR =
  'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';
const WORKSPACE_ACTIVITY_FALLBACK_FOCUS_SELECTOR =
  '.app-workspace-nav-button[data-active="true"], .app-workspace-account-profile';

export function isWorkspaceActivityContextMenuKey(
  key: string,
  shiftKey = false,
) {
  return key === "ContextMenu" || (shiftKey && key === "F10");
}

export function resolveWorkspaceActivityPointerMenuPoint(
  clientX: number,
  clientY: number,
) {
  return {
    x: Number.isFinite(clientX) ? clientX : 0,
    y: Number.isFinite(clientY) ? clientY : 0,
  };
}

export function resolveWorkspaceActivityKeyboardMenuPoint(
  rect: Pick<DOMRect, "left" | "bottom" | "width">,
) {
  return {
    x: rect.left + rect.width / 2,
    y: rect.bottom,
  };
}

function WorkspaceActivityContextMenu(props: {
  actions?: readonly WorkspaceActivityMenuAction[];
  ariaLabel: string;
  children: (targetProps: WorkspaceActivityContextTargetProps) => React.ReactNode;
}) {
  const [anchor, setAnchor] = React.useState<{ x: number; y: number } | null>(
    null,
  );
  const returnFocusRef = React.useRef<HTMLElement | null>(null);
  const actions = props.actions?.filter(Boolean) ?? [];

  const openAt = React.useCallback(
    (x: number, y: number, returnFocus: HTMLElement | null) => {
      returnFocusRef.current = returnFocus;
      setAnchor({ x, y });
    },
    [],
  );
  const resolveReturnFocus = React.useCallback(
    (target: EventTarget | null, fallback: HTMLElement) => {
      const targetControl =
        target instanceof Element
          ? target.closest<HTMLElement>(WORKSPACE_ACTIVITY_FOCUSABLE_SELECTOR)
          : null;
      if (targetControl) {
        return targetControl;
      }
      if (fallback.matches(WORKSPACE_ACTIVITY_FOCUSABLE_SELECTOR)) {
        return fallback;
      }
      return document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    },
    [],
  );

  if (actions.length === 0) {
    return props.children({});
  }

  return (
    <DropdownMenu
      modal={false}
      open={anchor !== null}
      onOpenChange={(open) => {
        if (!open) {
          setAnchor(null);
        }
      }}
    >
      {props.children({
        onContextMenu: (event) => {
          event.preventDefault();
          event.stopPropagation();
          const point = resolveWorkspaceActivityPointerMenuPoint(
            event.clientX,
            event.clientY,
          );
          openAt(
            point.x,
            point.y,
            resolveReturnFocus(event.target, event.currentTarget),
          );
        },
        onKeyDown: (event) => {
          if (!isWorkspaceActivityContextMenuKey(event.key, event.shiftKey)) {
            return;
          }
          event.preventDefault();
          event.stopPropagation();
          const focusTarget =
            resolveReturnFocus(event.target, event.currentTarget) ??
            event.currentTarget;
          const rect = focusTarget.getBoundingClientRect();
          const point = resolveWorkspaceActivityKeyboardMenuPoint(rect);
          openAt(point.x, point.y, focusTarget);
        },
      })}
      {anchor ? (
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-hidden="true"
            tabIndex={-1}
            className="app-workspace-status-context-anchor fixed z-50 h-px w-px"
            style={{ left: anchor.x, top: anchor.y }}
          />
        </DropdownMenuTrigger>
      ) : null}
      <DropdownMenuContent
        aria-label={props.ariaLabel}
        align="start"
        side="bottom"
        sideOffset={2}
        className="app-workspace-status-context-menu"
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          const returnFocus = returnFocusRef.current;
          if (returnFocus?.isConnected) {
            returnFocus.focus();
            return;
          }
          document
            .querySelector<HTMLElement>(
              WORKSPACE_ACTIVITY_FALLBACK_FOCUS_SELECTOR,
            )
            ?.focus();
        }}
      >
        {actions.map((action) => (
          <DropdownMenuItem
            key={action.key}
            className="app-workspace-status-context-menu__item"
            data-tone={action.tone}
            disabled={action.disabled}
            onSelect={action.onSelect}
          >
            <span
              className="app-workspace-status-context-menu__icon"
              aria-hidden="true"
            >
              {action.icon}
            </span>
            <span className="app-workspace-status-context-menu__label">
              {action.label}
            </span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * The single material boundary for sidebar status surfaces. Variants own
 * layout and content only; Glass/Contrast recipes key off the shared semantic
 * role instead of sniff/player/operation class names.
 */
function WorkspaceStatusSurface(props: WorkspaceStatusSurfaceProps) {
  return (
    <GlassSurface
      {...props}
      elevation="floating"
      shape="card"
      surfaceRole="status"
      tint="neutral"
    />
  );
}

export type WorkspaceActivityLabels = {
  sniff: string;
  stopSniff: string;
  sniffState: {
    idle: string;
    starting: string;
    active: string;
    closing: string;
    error: string;
    orphan: string;
  };
  resources: string;
  downloadable: string;
  session: string;
  updated: string;
  clear: string;
  operations: string;
  downloads: string;
  transcodes: string;
  nowPlaying: string;
  previous: string;
  play: string;
  pause: string;
  next: string;
  noActivity: string;
};

function resolveSniffStatusTone(
  status: SniffStatusSnapshot,
  busy = false,
): WorkspaceStatusTone {
  if (status.state === "error") {
    return "error";
  }
  if (busy || status.state === "starting" || status.state === "closing") {
    return "busy";
  }
  if (status.runtime === "orphan") {
    return "orphan";
  }
  return status.state === "active" ? "success" : "idle";
}

function resolvePlaybackStatusTone(
  status: ListenNowPlayingStatus,
): WorkspaceStatusTone {
  switch (status.state) {
    case "loading":
      return "busy";
    case "playing":
      return "success";
    case "error":
      return "error";
    default:
      return "idle";
  }
}

function resolvePlaybackSource(
  status: ListenNowPlayingStatus,
): ListenPlaybackSource {
  if (status.playbackSource) {
    return status.playbackSource;
  }
  if (status.mode === "hush") {
    return "radio";
  }
  if (status.mode === "linger") {
    return "local";
  }
  return "youtube_music";
}

function resolvePlaybackSourceLabel(status: ListenNowPlayingStatus) {
  const source = resolvePlaybackSource(status);
  return (
    status.playbackSourceLabel?.trim() ||
    source
      .split("_")
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
      .join(" ")
  );
}

function PlaybackSourceBadge(props: { status: ListenNowPlayingStatus }) {
  const source = resolvePlaybackSource(props.status);
  const label = resolvePlaybackSourceLabel(props.status);
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          className="app-workspace-player-wide__source"
          role="img"
          tabIndex={0}
          aria-label={label}
        >
          <ListenPlayerSourceIcon source={source} />
        </span>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

function Favicon(props: { source: string; title: string }) {
  return (
    <SiteFavicon
      source={props.source}
      fallback={<Radar aria-hidden="true" />}
    />
  );
}

function WorkspaceStatusArtworkBackdrop(props: {
  children: React.ReactNode;
  className?: string;
  fallback?: boolean;
  placement?: "center" | "end";
}) {
  return (
    <span
      className={`app-workspace-status-card__artwork-backdrop${
        props.className ? ` ${props.className}` : ""
      }`}
      data-fallback={props.fallback ? "true" : undefined}
      data-placement={props.placement ?? "center"}
      aria-hidden="true"
    >
      {props.children}
    </span>
  );
}

function OperationStatusArtworkBackdrop(props: {
  httpBaseURL: string;
  item: OperationActivityItem;
  className?: string;
}) {
  const source = resolveOperationThumbnailCoverURL(
    props.httpBaseURL,
    props.item.operation,
  );
  return (
    <WorkspaceStatusArtworkBackdrop
      className={`app-workspace-operation-artwork-backdrop${
        props.className ? ` ${props.className}` : ""
      }`}
      fallback={!source}
      placement="end"
    >
      {source ? (
        <img
          src={source}
          alt=""
          loading="lazy"
          decoding="async"
          draggable={false}
        />
      ) : props.item.kind === "transcode" ? (
        <Gauge />
      ) : (
        <Download />
      )}
    </WorkspaceStatusArtworkBackdrop>
  );
}

function resolveSniffStateLabel(
  status: SniffStatusSnapshot,
  labels: WorkspaceActivityLabels,
) {
  return status.runtime === "orphan"
    ? labels.sniffState.orphan
    : labels.sniffState[status.state];
}

function SniffStatusAction(props: {
  className: string;
  status: SniffStatusSnapshot;
  label: string;
  stopping?: boolean;
  onStop: () => void;
}) {
  const busy =
    props.stopping === true ||
    props.status.state === "starting" ||
    props.status.state === "closing";

  if (props.status.canStop) {
    return (
      <button
        type="button"
        className={`app-workspace-status-card__action ${props.className}`}
        aria-label={props.label}
        data-tone="destructive"
        disabled={busy}
        onClick={props.onStop}
        title={props.label}
      >
        {busy ? <Loader2 className="app-motion-spin" /> : <X />}
      </button>
    );
  }

  return (
    <span
      className="app-workspace-status-card__state"
      aria-label={props.label}
      data-state={props.status.state}
      data-tone={resolveSniffStatusTone(props.status, busy)}
      role="status"
      title={props.label}
    >
      {busy ? <Loader2 className="app-motion-spin" /> : <span aria-hidden="true" />}
    </span>
  );
}

export function WideSniffActivity(props: {
  status: SniffStatusSnapshot;
  labels: WorkspaceActivityLabels;
  menuActions?: readonly WorkspaceActivityMenuAction[];
  stopping?: boolean;
  onOpen: () => void;
  onStop: () => void;
}) {
  if (props.status.runtime === "none") {
    return null;
  }
  const title = props.status.title || props.labels.sniff;
  return (
    <WorkspaceActivityContextMenu
      actions={props.menuActions}
      ariaLabel={`${props.labels.sniff}: ${title}`}
    >
      {(contextTargetProps) => (
        <WorkspaceStatusSurface
          {...contextTargetProps}
          className="app-workspace-activity-card app-workspace-status-card app-workspace-sniff-wide group"
          data-artwork="true"
          data-runtime={props.status.runtime}
          data-state={props.status.state}
          data-tone={resolveSniffStatusTone(props.status, props.stopping)}
        >
          <WorkspaceStatusArtworkBackdrop className="app-workspace-sniff-wide__backdrop">
            <Favicon source={props.status.favicon} title={title} />
          </WorkspaceStatusArtworkBackdrop>
          <button
            type="button"
            className="app-workspace-sniff-wide__open"
            aria-label={`${props.labels.sniff}: ${title}`}
            onClick={props.onOpen}
          >
            <span className="app-workspace-activity-artwork">
              <Favicon source={props.status.favicon} title={title} />
            </span>
            <span className="app-workspace-activity-details">
              <span className="app-workspace-activity-title">{title}</span>
              <span className="app-workspace-activity-subtitle">
                {props.status.url}
              </span>
            </span>
          </button>
          <SniffStatusAction
            className="app-workspace-sniff-wide__stop"
            status={props.status}
            label={props.labels.stopSniff}
            stopping={props.stopping}
            onStop={props.onStop}
          />
        </WorkspaceStatusSurface>
      )}
    </WorkspaceActivityContextMenu>
  );
}

export function SniffWorkspaceSessionActivity(props: {
  status: SniffStatusSnapshot;
  labels: {
    sniff: string;
    session: string;
    resources: string;
    downloadable: string;
    status: string;
    updated: string;
  };
}) {
  if (props.status.runtime === "none") {
    return null;
  }

  const title = props.status.title || props.labels.sniff;
  const identity = sniffSessionIdentity(props.status.url, title);

  return (
    <WorkspaceStatusSurface asChild>
      <section
        className="app-workspace-activity-card app-workspace-status-card app-workspace-sniff-session"
        aria-label={props.labels.sniff}
        data-details-open="true"
        data-runtime={props.status.runtime}
        data-state={props.status.state}
        data-tone={resolveSniffStatusTone(props.status)}
      >
        <div className="app-workspace-sniff-session__control">
          <Tooltip>
            <TooltipTrigger asChild>
              <div
                className="app-workspace-sniff-session__identity"
                tabIndex={0}
                aria-label={`${title}${props.status.url ? `, ${props.status.url}` : ""}`}
              >
                <span
                  className="app-workspace-sniff-session__favicon"
                  data-state={props.status.state}
                >
                  <Favicon source={props.status.favicon} title={title} />
                </span>
                <span className="app-workspace-sniff-session__copy">
                  <span className="app-workspace-sniff-session__name">
                    {title}
                  </span>
                  <span className="app-workspace-sniff-session__address">
                    {props.status.url || identity}
                  </span>
                </span>
                <span className="sr-only" role="status">
                  {props.labels.status}
                </span>
              </div>
            </TooltipTrigger>
            <TooltipContent side="top" multiline className="max-w-72">
              <div className="app-workspace-sniff-session-tooltip__title">{title}</div>
              {props.status.url ? (
                <div className="app-workspace-sniff-session-tooltip__address mt-1 break-all">
                  {props.status.url}
                </div>
              ) : null}
            </TooltipContent>
          </Tooltip>
        </div>
        <div className="app-workspace-sniff-session__details">
          <div className="app-workspace-sniff-session__metrics">
            <SniffSessionMetric
              label={props.labels.session}
              value={props.labels.status}
            />
            <SniffSessionMetric
              label={props.labels.resources}
              value={String(props.status.resourceCount)}
            />
            <SniffSessionMetric
              label={props.labels.downloadable}
              value={String(props.status.downloadableCount)}
            />
            <SniffSessionMetric
              label={props.labels.updated}
              value={formatActivityDate(props.status.lastCaptureAt)}
            />
          </div>
        </div>
      </section>
    </WorkspaceStatusSurface>
  );
}

function sniffSessionIdentity(url: string, fallback: string) {
  try {
    return new URL(url).hostname.replace(/^www\./i, "") || fallback;
  } catch {
    return fallback;
  }
}

function SniffSessionMetric(props: { label: string; value: string }) {
  return (
    <span className="app-workspace-sniff-session__metric">
      <strong>{props.value || "—"}</strong>
      <small>{props.label}</small>
    </span>
  );
}

function formatActivityDate(value?: string) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function PlaybackTitleMarquee(props: { text: string }) {
  const containerRef = React.useRef<HTMLSpanElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const [overflow, setOverflow] = React.useState(0);
  const text = props.text.trim();
  const scrolling = overflow > 1;

  React.useEffect(() => {
    const container = containerRef.current;
    const content = contentRef.current;
    if (!container || !content) {
      return;
    }
    const measure = () => {
      setOverflow(Math.max(0, content.scrollWidth - container.clientWidth));
    };
    measure();
    if (typeof ResizeObserver === "undefined") {
      return;
    }
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    observer.observe(content);
    return () => observer.disconnect();
  }, [text]);

  return (
    <span
      ref={containerRef}
      className="app-workspace-activity-title app-workspace-player-wide__title-marquee"
      data-overflow={scrolling ? "true" : "false"}
      title={text}
    >
      <span
        ref={contentRef}
        style={
          scrolling
            ? ({
                "--app-workspace-title-shift": `-${Math.ceil(overflow + 8)}px`,
                "--app-workspace-title-duration": `${Math.min(
                  13,
                  Math.max(6, (overflow + 140) / 26),
                )}s`,
              } as React.CSSProperties)
            : undefined
        }
      >
        {text}
      </span>
    </span>
  );
}

export function WidePlaybackActivity(props: {
  status: ListenNowPlayingStatus | null;
  labels: WorkspaceActivityLabels;
  menuActions?: readonly WorkspaceActivityMenuAction[];
  onOpen: () => void;
  onCommand: (command: WorkspacePlaybackCommand) => void;
}) {
  const status = props.status;
  if (!status || status.state === "idle") {
    return null;
  }
  const duration = Math.max(0, status.progress.duration || 0);
  const current = Math.max(0, status.progress.currentTime || 0);
  const live = status.live === true || status.mode === "hush";
  const title = status.title.trim() || props.labels.nowPlaying;
  const percent = live
    ? 100
    : duration > 0
      ? Math.min(100, (current / duration) * 100)
      : 0;
  const playing = status.state === "playing";
  return (
    <WorkspaceActivityContextMenu
      actions={props.menuActions}
      ariaLabel={`${props.labels.nowPlaying}: ${title}`}
    >
      {(contextTargetProps) => (
        <WorkspaceStatusSurface
          {...contextTargetProps}
          className="app-workspace-activity-card app-workspace-status-card app-workspace-player-wide group"
          data-artwork="true"
          data-controllable={status.canControl ? "true" : "false"}
          data-playback="timeline"
          data-live={live ? "true" : "false"}
          data-state={status.state}
          data-tone={resolvePlaybackStatusTone(status)}
        >
          <WorkspaceStatusArtworkBackdrop
            className="app-workspace-player-wide__backdrop"
          >
            <ListenSidebarArtwork status={status} />
          </WorkspaceStatusArtworkBackdrop>
          <button
            type="button"
            className="app-workspace-player-wide__open"
            aria-label={`${props.labels.nowPlaying}: ${status.title}`}
            onClick={props.onOpen}
          >
            <span className="app-workspace-activity-artwork">
              <ListenSidebarArtwork status={status} />
            </span>
            <span className="app-workspace-activity-details app-workspace-player-wide__details">
              <PlaybackTitleMarquee text={status.title} />
              <span
                className="app-workspace-player-wide__progress"
                data-live={live ? "true" : "false"}
                aria-hidden="true"
              >
                <span style={{ width: `${percent}%` }} />
              </span>
            </span>
          </button>
          <PlaybackSourceBadge status={status} />
          {status.canControl ? (
            <div
              className="app-workspace-player-wide__controls"
              role="group"
              aria-label={props.labels.nowPlaying}
            >
              <ActivityIconButton
                disabled={status.canPrevious === false}
                label={props.labels.previous}
                onClick={() => props.onCommand("previous")}
              >
                <SkipBack />
              </ActivityIconButton>
              <ActivityIconButton
                label={playing ? props.labels.pause : props.labels.play}
                onClick={() => props.onCommand("toggle")}
              >
                {playing ? <Pause /> : <Play />}
              </ActivityIconButton>
              <ActivityIconButton
                disabled={status.canNext === false}
                label={props.labels.next}
                onClick={() => props.onCommand("next")}
              >
                <SkipForward />
              </ActivityIconButton>
            </div>
          ) : null}
        </WorkspaceStatusSurface>
      )}
    </WorkspaceActivityContextMenu>
  );
}

function ActivityIconButton(props: {
  label: string;
  onClick: () => void;
  children: React.ReactNode;
  disabled?: boolean;
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={props.label}
      title={props.label}
      disabled={props.disabled}
      onClick={props.onClick}
    >
      {props.children}
    </Button>
  );
}

export function WideOperationActivity(props: {
  snapshot: OperationActivitySnapshot;
  labels: WorkspaceActivityLabels;
  httpBaseURL: string;
  menuActions?: readonly WorkspaceActivityMenuAction[];
  onOpen: () => void;
}) {
  if (!props.snapshot.hasActivity) {
    return null;
  }
  const artworkItem =
    props.snapshot.items.find((item) =>
      Boolean(item.operation.thumbnailPreviewPath?.trim()),
    ) ?? props.snapshot.primary;
  return (
    <WorkspaceActivityContextMenu
      actions={props.menuActions}
      ariaLabel={props.labels.operations}
    >
      {(contextTargetProps) => (
        <WorkspaceStatusSurface asChild>
          <button
            {...contextTargetProps}
            type="button"
            className="app-workspace-activity-card app-workspace-status-card app-workspace-operation-wide"
            data-artwork="true"
            data-state={props.snapshot.runningCount > 0 ? "running" : "queued"}
            data-tone={props.snapshot.runningCount > 0 ? "busy" : "idle"}
            onClick={props.onOpen}
          >
            {artworkItem ? (
              <OperationStatusArtworkBackdrop
                className="app-workspace-operation-wide__backdrop"
                httpBaseURL={props.httpBaseURL}
                item={artworkItem}
              />
            ) : null}
            {props.snapshot.download.activeCount > 0 ? (
              <OperationKindRow
                icon={<Download />}
                label={props.labels.downloads}
                count={props.snapshot.download.activeCount}
                percent={props.snapshot.download.progressPercent}
                speed={formatActivitySpeed(props.snapshot.download.speed)}
                indeterminate={props.snapshot.download.hasIndeterminateProgress}
              />
            ) : null}
            {props.snapshot.transcode.activeCount > 0 ? (
              <OperationKindRow
                icon={<Gauge />}
                label={props.labels.transcodes}
                count={props.snapshot.transcode.activeCount}
                percent={props.snapshot.transcode.progressPercent}
                speed={formatActivitySpeed(props.snapshot.transcode.speed)}
                indeterminate={props.snapshot.transcode.hasIndeterminateProgress}
              />
            ) : null}
          </button>
        </WorkspaceStatusSurface>
      )}
    </WorkspaceActivityContextMenu>
  );
}

function OperationKindRow(props: {
  icon: React.ReactNode;
  label: string;
  count: number;
  percent?: number;
  speed: string;
  indeterminate: boolean;
}) {
  return (
    <span className="app-workspace-operation-row">
      <span className="app-workspace-operation-row__main">
        <span className="app-workspace-operation-row__heading">
          <span className="app-workspace-operation-row__icon">{props.icon}</span>
          <span>{props.label} · {props.count}</span>
        </span>
        <span
          className="app-workspace-operation-row__progress"
          data-indeterminate={props.indeterminate && props.percent === undefined}
        >
          <span style={{ width: `${props.percent ?? 24}%` }} />
        </span>
      </span>
      <span className="app-workspace-operation-row__speed">{props.speed}</span>
    </span>
  );
}

export function PlayerCompanionView(props: {
  status: ListenNowPlayingStatus | null;
  labels: WorkspaceActivityLabels;
}) {
  const status = props.status;
  if (!status || status.state === "idle") {
    return <CompanionEmpty icon={<Play />} label={props.labels.noActivity} />;
  }
  const duration = Math.max(0, status.progress.duration || 0);
  const current = Math.max(0, status.progress.currentTime || 0);
  const percent = duration > 0 ? Math.min(100, (current / duration) * 100) : 0;
  return (
    <div
      className="app-workspace-player-companion"
      data-companion-scroll-owner="playback-summary"
    >
      <div className="app-workspace-player-companion__artwork">
        <ListenSidebarArtwork status={status} />
      </div>
      <div className="app-workspace-player-companion__title">{status.title}</div>
      <div className="app-workspace-player-companion__subtitle">{status.subtitle}</div>
      <div className="app-workspace-player-companion__timeline">
        <span style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

export function PlayerCompanionFooter(props: {
  status: ListenNowPlayingStatus;
  labels: WorkspaceActivityLabels;
  onCommand: (command: WorkspacePlaybackCommand) => void;
}) {
  return (
    <div className="app-workspace-player-companion__controls">
      <ActivityIconButton
        disabled={props.status.canPrevious === false}
        label={props.labels.previous}
        onClick={() => props.onCommand("previous")}
      >
        <SkipBack />
      </ActivityIconButton>
      <ActivityIconButton
        label={
          props.status.state === "playing"
            ? props.labels.pause
            : props.labels.play
        }
        onClick={() => props.onCommand("toggle")}
      >
        {props.status.state === "playing" ? <Pause /> : <Play />}
      </ActivityIconButton>
      <ActivityIconButton
        disabled={props.status.canNext === false}
        label={props.labels.next}
        onClick={() => props.onCommand("next")}
      >
        <SkipForward />
      </ActivityIconButton>
    </div>
  );
}

export function SniffCompanionView(props: {
  status: SniffStatusSnapshot;
  labels: WorkspaceActivityLabels;
  clearing?: boolean;
  stopping?: boolean;
}) {
  if (props.status.runtime === "none") {
    return <CompanionEmpty icon={<Radar />} label={props.labels.noActivity} />;
  }
  const title = props.status.title || props.labels.sniff;
  return (
    <div
      className="app-workspace-sniff-companion"
      data-companion-scroll-owner="sniff"
      data-runtime={props.status.runtime}
      data-state={props.status.state}
      data-tone={resolveSniffStatusTone(
        props.status,
        props.stopping || props.clearing,
      )}
    >
      <div className="app-workspace-sniff-companion__identity">
        <span className="app-workspace-activity-artwork">
          <Favicon source={props.status.favicon} title={title} />
        </span>
        <span className="app-workspace-activity-details">
          <span className="app-workspace-activity-title" title={title}>
            {title}
          </span>
          <span
            className="app-workspace-activity-subtitle"
            title={props.status.url}
          >
            {props.status.url}
          </span>
        </span>
      </div>
      <div className="app-workspace-sniff-companion__metrics">
        <Metric
          label={props.labels.session}
          value={resolveSniffStateLabel(props.status, props.labels)}
        />
        <Metric
          label={props.labels.resources}
          value={props.status.resourceCount}
        />
        <Metric
          label={props.labels.downloadable}
          value={props.status.downloadableCount}
        />
        <Metric
          label={props.labels.updated}
          value={formatActivityDate(props.status.lastCaptureAt)}
        />
      </div>
    </div>
  );
}

export function SniffCompanionFooter(props: {
  status: SniffStatusSnapshot;
  labels: WorkspaceActivityLabels;
  onClear: () => void;
  onStop: () => void;
  clearing?: boolean;
  stopping?: boolean;
}) {
  const busy =
    props.status.state === "starting" || props.status.state === "closing";
  const clearDisabled =
    !props.status.canClear ||
    props.clearing === true ||
    props.stopping === true ||
    busy;
  const stopDisabled =
    !props.status.canStop || props.stopping === true || busy;

  return (
    <div className="app-workspace-sniff-companion__actions">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={props.labels.clear}
            disabled={clearDisabled}
            onClick={props.onClear}
          >
            {props.clearing ? (
              <Loader2 className="app-motion-spin" />
            ) : (
              <BrushCleaning />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="top">{props.labels.clear}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            tone="destructive"
            size="icon"
            aria-label={props.labels.stopSniff}
            disabled={stopDisabled}
            onClick={props.onStop}
          >
            {props.stopping || props.status.state === "closing" ? (
              <Loader2 className="app-motion-spin" />
            ) : (
              <X />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="top">{props.labels.stopSniff}</TooltipContent>
      </Tooltip>
    </div>
  );
}

export function OperationsCompanionView(props: {
  snapshot: OperationActivitySnapshot;
  labels: WorkspaceActivityLabels;
  httpBaseURL: string;
}) {
  if (!props.snapshot.hasActivity) {
    return <CompanionEmpty icon={<Download />} label={props.labels.noActivity} />;
  }
  return (
    <div
      className="app-workspace-operations-companion"
      data-companion-scroll-owner="operations"
    >
      {props.snapshot.items.map((item) => (
        <WorkspaceStatusSurface key={item.operationId} asChild>
          <div
            className="app-workspace-operation-item app-workspace-status-card"
            data-artwork="true"
            data-state={item.status}
            data-tone={item.status === "running" ? "busy" : "idle"}
          >
            <OperationStatusArtworkBackdrop
              className="app-workspace-operation-item__backdrop"
              httpBaseURL={props.httpBaseURL}
              item={item}
            />
            <div className="app-workspace-operation-item__heading">
              <span>{item.operation.name || item.operationId}</span>
              <span>
                {item.percent === undefined ? "…" : `${Math.round(item.percent)}%`}
              </span>
            </div>
            <div className="app-workspace-operation-item__meta">
              <span>
                {item.kind === "download"
                  ? props.labels.downloads
                  : props.labels.transcodes}
              </span>
              <span>{item.speed?.label}</span>
            </div>
            <div
              className="app-workspace-operation-row__progress"
              data-indeterminate={item.progressMode === "indeterminate"}
            >
              <span style={{ width: `${item.percent ?? 24}%` }} />
            </div>
          </div>
        </WorkspaceStatusSurface>
      ))}
    </div>
  );
}

function Metric(props: { label: string; value: React.ReactNode }) {
  return (
    <div className="app-workspace-sniff-companion__metric">
      <strong>{props.value}</strong>
      <span>{props.label}</span>
    </div>
  );
}

function CompanionEmpty(props: { icon: React.ReactNode; label: string }) {
  return (
    <div className="app-workspace-companion-empty">
      {props.icon}
      <span>{props.label}</span>
    </div>
  );
}

function formatActivitySpeed(speed: {
  bytesPerSecond?: number;
  framesPerSecond?: number;
  factor?: number;
  labels: string[];
}) {
  if (speed.bytesPerSecond) {
    const value = speed.bytesPerSecond;
    if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(1)} MB/s`;
    if (value >= 1024) return `${(value / 1024).toFixed(1)} KB/s`;
    return `${Math.round(value)} B/s`;
  }
  if (speed.framesPerSecond) return `${speed.framesPerSecond.toFixed(1)} fps`;
  if (speed.factor) return `${speed.factor.toFixed(1)}x`;
  return speed.labels[0] ?? "";
}
