import { Check,ChevronLeft,ChevronRight,Columns3,Loader2,Pencil,Play,Plus,RefreshCw,Trash2 } from "lucide-react";
import * as React from "react";
import { createPortal } from "react-dom";

import { fetchListenLiveChannelPreview } from "@/app/main/listen/api";
import type { ListenLiveChannelPreview,ListenLiveUserCatalog,ListenLiveUserChannel,ListenLiveUserColumn } from "@/app/main/listen/api";
import type { ListenLiveGroup,ListenLiveStatus,ListenLiveStatusValue,ListenOnlineItem } from "@/app/main/listen/types";
import type { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { ListenCoverArtwork } from "@/shared/assets/listen-cover-artwork";
import {
LISTEN_CONTROL_ICON_BUTTON_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS,
LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS,
} from "@/shared/styles/listen";
import { buildListenPosterCandidates,buildListenTrackThumbnailCandidates } from "@/app/main/listen/storage";
import { Button } from "@/shared/ui/button";
import {
Dialog,
DialogContent,
DialogFooter,
DialogHeader,
DialogListCard,
DialogListCardContent,
DialogRow,
DialogScrollArea,
DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import { getXiaSurfaceAttributes } from "@/shared/ui/surface-contract";
import { StatusBadge } from "@/shared/ui/status-badge";
import { Tooltip,TooltipContent,TooltipProvider,TooltipTrigger } from "@/shared/ui/tooltip";
import {
  WorkspacePrimaryHeaderAction,
  WorkspacePrimaryHeaderActionGroup,
} from "@/shared/ui/workspace-primary-header-action";

type TextBundle = ReturnType<typeof getXiaText>;

type AddChannelDraft = {
  kind: "add";
  url: string;
  preview: ListenLiveChannelPreview | null;
  columnId: string;
  title: string;
  error: string;
  loading: boolean;
};

type EditChannelDraft = {
  kind: "edit";
  id: string;
  columnId: string;
  title: string;
  channel: string;
  videoId: string;
  description: string;
  thumbnailUrl: string;
  error: string;
};

type ChannelDraft = AddChannelDraft | EditChannelDraft;

export function ListenHushLiveActionGroup(props: {
  httpBaseURL: string;
  text: TextBundle;
  liveGroups: ListenLiveGroup[];
  liveCatalogLoading: boolean;
  liveUserCatalog: ListenLiveUserCatalog;
  liveUserCatalogLoading: boolean;
  liveUserCatalogSaving: boolean;
  onReloadCatalog: () => void;
  onSaveUserCatalog: (catalog: ListenLiveUserCatalog) => Promise<void>;
}) {
  const [channelDialog, setChannelDialog] = React.useState<ChannelDraft | null>(null);
  const [columnsOpen, setColumnsOpen] = React.useState(false);
  const userCatalogBusy =
    props.liveUserCatalogLoading ||
    props.liveUserCatalogSaving ||
    props.liveCatalogLoading;

  const saveCatalog = React.useCallback(
    (catalog: ListenLiveUserCatalog) =>
      props.onSaveUserCatalog(normalizeUserCatalog(catalog, props.liveGroups)),
    [props],
  );

  const openAddChannel = React.useCallback(() => {
    setChannelDialog({
      kind: "add",
      url: "",
      preview: null,
      columnId: resolveDefaultChannelColumnId(props.liveGroups, props.liveUserCatalog),
      title: "",
      error: "",
      loading: false,
    });
  }, [props.liveGroups, props.liveUserCatalog]);

  return (
    <>
      <WorkspacePrimaryHeaderActionGroup
        className="shrink-0"
        label={props.text.listen.hush}
      >
        <WorkspacePrimaryHeaderAction
          label={props.text.listen.addChannel}
          disabled={props.liveUserCatalogSaving}
          onClick={openAddChannel}
        >
          <Plus className="h-4 w-4" />
        </WorkspacePrimaryHeaderAction>
        <WorkspacePrimaryHeaderAction
          label={props.text.listen.manageColumns}
          disabled={props.liveUserCatalogSaving}
          onClick={() => setColumnsOpen(true)}
        >
          <Columns3 className="h-4 w-4" />
        </WorkspacePrimaryHeaderAction>
        <WorkspacePrimaryHeaderAction
          label={props.text.listen.refresh}
          disabled={userCatalogBusy}
          onClick={props.onReloadCatalog}
        >
          <RefreshCw className={cn("h-4 w-4", userCatalogBusy ? "listen-loading-spinner" : "")} />
        </WorkspacePrimaryHeaderAction>
      </WorkspacePrimaryHeaderActionGroup>

      <ListenHushChannelDialog
        httpBaseURL={props.httpBaseURL}
        draft={channelDialog}
        liveGroups={props.liveGroups}
        catalog={props.liveUserCatalog}
        saving={props.liveUserCatalogSaving}
        text={props.text}
        onChange={setChannelDialog}
        onCancel={() => setChannelDialog(null)}
        onResolve={async (draft) => {
          const url = draft.url.trim();
          if (!url) {
            setChannelDialog({ ...draft, error: props.text.listen.channelVideoIdRequired });
            return;
          }
          setChannelDialog({ ...draft, url, error: "", loading: true });
          try {
            const preview = await fetchListenLiveChannelPreview(props.httpBaseURL, url);
            setChannelDialog((current) =>
              current?.kind === "add" && current.url.trim() === url
                ? {
                    ...current,
                    url,
                    preview,
                    title: preview.title,
                    error: "",
                    loading: false,
                  }
                : current,
            );
          } catch (error) {
            setChannelDialog((current) =>
              current?.kind === "add" && current.url.trim() === url
                ? {
                    ...current,
                    url,
                    error: resolveChannelDialogError(error, props.text),
                    loading: false,
                  }
                : current,
            );
          }
        }}
        onSave={async (draft) => {
          const next = buildCatalogWithChannel(
            props.liveUserCatalog,
            props.liveGroups,
            draft,
            props.text,
          );
          if ("error" in next) {
            setChannelDialog({ ...draft, error: next.error });
            return;
          }
          try {
            await saveCatalog(next.catalog);
            setChannelDialog(null);
          } catch (error) {
            setChannelDialog({
              ...draft,
              error: resolveChannelDialogError(error, props.text),
            });
          }
        }}
      />
      <ListenHushColumnsDialog
        open={columnsOpen}
        text={props.text}
        liveGroups={props.liveGroups}
        catalog={props.liveUserCatalog}
        saving={props.liveUserCatalogSaving}
        onOpenChange={setColumnsOpen}
        onSave={saveCatalog}
      />
    </>
  );
}

export function ListenHushLiveList(props: {
  httpBaseURL: string;
  text: TextBundle;
  liveGroups: ListenLiveGroup[];
  liveStatusByVideoId: Record<string, ListenLiveStatus>;
  liveCatalogLoading: boolean;
  liveCatalogError: boolean;
  liveCatalogMessage: string;
  liveUserCatalog: ListenLiveUserCatalog;
  liveUserCatalogLoading: boolean;
  liveUserCatalogSaving: boolean;
  liveUserCatalogError: string;
  curatedLiveItems: ListenOnlineItem[];
  liveSelectionArmed: boolean;
  selectedLiveId: string;
  normalizedQuery: string;
  liveSearchNotice: string;
  onReloadCatalog: () => void;
  onSaveUserCatalog: (catalog: ListenLiveUserCatalog) => Promise<void>;
  onSelect: (item: ListenOnlineItem) => void;
}) {
  const [channelDialog, setChannelDialog] = React.useState<ChannelDraft | null>(null);
  const [confirmRemoveChannelId, setConfirmRemoveChannelId] = React.useState("");
  const userChannelByID = React.useMemo(
    () => new Map(props.liveUserCatalog.channels.map((channel) => [channel.id, channel])),
    [props.liveUserCatalog.channels],
  );

  const openEditChannel = React.useCallback(
    (item: ListenOnlineItem) => {
      const channel = userChannelByID.get(item.id);
      if (!channel) {
        return;
      }
      setConfirmRemoveChannelId("");
      setChannelDialog({
        kind: "edit",
        id: channel.id,
        columnId: channel.columnId,
        title: channel.title,
        channel: channel.channel,
        videoId: channel.videoId,
        description: channel.description,
        thumbnailUrl: channel.thumbnailUrl,
        error: "",
      });
    },
    [userChannelByID],
  );

  const saveCatalog = React.useCallback(
    (catalog: ListenLiveUserCatalog) =>
      props.onSaveUserCatalog(normalizeUserCatalog(catalog, props.liveGroups)),
    [props],
  );

  const removeChannel = React.useCallback(
    async (item: ListenOnlineItem) => {
      const channel = userChannelByID.get(item.id);
      if (!channel) {
        return;
      }
      if (confirmRemoveChannelId !== channel.id) {
        setConfirmRemoveChannelId(channel.id);
        return;
      }
      try {
        await saveCatalog({
          columns: props.liveUserCatalog.columns,
          channels: props.liveUserCatalog.channels.filter((entry) => entry.id !== channel.id),
        });
        setConfirmRemoveChannelId("");
      } catch {
        // The parent state exposes the save error in the toolbar notice.
      }
    },
    [confirmRemoveChannelId, props.liveUserCatalog, saveCatalog, userChannelByID],
  );

  return (
    <TooltipProvider delayDuration={0}>
      <div className="space-y-7">
        {props.liveCatalogLoading && props.liveGroups.length === 0 ? (
          null
        ) : props.liveCatalogError || props.liveGroups.length === 0 ? (
          null
        ) : (
          <>
            {props.normalizedQuery ? (
              <ListenHushLiveCardGroup
                title={props.text.listen.liveStations}
                hideTitle
                items={props.curatedLiveItems}
                selectedId={props.liveSelectionArmed ? props.selectedLiveId : ""}
                httpBaseURL={props.httpBaseURL}
                text={props.text}
                liveStatuses={props.liveStatusByVideoId}
                onSelect={props.onSelect}
              />
            ) : (
              props.liveGroups.map((group) => {
                return (
                  <ListenHushLiveCardGroup
                    key={group.id}
                    title={resolveHushLiveGroupTitle(group, props.text)}
                    items={group.items}
                    selectedId={props.liveSelectionArmed ? props.selectedLiveId : ""}
                    httpBaseURL={props.httpBaseURL}
                    text={props.text}
                    liveStatuses={props.liveStatusByVideoId}
                    editLabel={props.text.listen.editChannel}
                    removeLabel={props.text.listen.removeChannel}
                    confirmRemoveLabel={props.text.listen.confirmRemoveChannel}
                    canEdit={(item) => userChannelByID.has(item.id)}
                    onEdit={openEditChannel}
                    canRemove={(item) => userChannelByID.has(item.id)}
                    isRemoveConfirming={(item) => confirmRemoveChannelId === item.id}
                    onRemove={(item) => void removeChannel(item)}
                    onSelect={props.onSelect}
                  />
                );
              })
            )}
          </>
        )}
      </div>

      <ListenHushChannelDialog
        httpBaseURL={props.httpBaseURL}
        draft={channelDialog}
        liveGroups={props.liveGroups}
        catalog={props.liveUserCatalog}
        saving={props.liveUserCatalogSaving}
        text={props.text}
        onChange={setChannelDialog}
        onCancel={() => setChannelDialog(null)}
        onResolve={async () => undefined}
        onSave={async (draft) => {
          const next = buildCatalogWithChannel(
            props.liveUserCatalog,
            props.liveGroups,
            draft,
            props.text,
          );
          if ("error" in next) {
            setChannelDialog({ ...draft, error: next.error });
            return;
          }
          try {
            await saveCatalog(next.catalog);
            setChannelDialog(null);
          } catch (error) {
            setChannelDialog({
              ...draft,
              error: resolveUserCatalogError(error, props.text),
            });
          }
        }}
      />
    </TooltipProvider>
  );
}

function ListenHushLiveCardGroup(props: {
  title: string;
  hideTitle?: boolean;
  items: ListenOnlineItem[];
  selectedId: string;
  httpBaseURL: string;
  text: TextBundle;
  liveStatuses?: Record<string, ListenLiveStatus>;
  canEdit?: (item: ListenOnlineItem) => boolean;
  onEdit?: (item: ListenOnlineItem) => void;
  editLabel?: string;
  canRemove?: (item: ListenOnlineItem) => boolean;
  onRemove?: (item: ListenOnlineItem) => void;
  removeLabel?: string;
  confirmRemoveLabel?: string;
  isRemoveConfirming?: (item: ListenOnlineItem) => boolean;
  onSelect: (item: ListenOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }
  const headerTitle = props.hideTitle ? "" : props.title.trim();
  return (
    <section className="listen-hush-live-group min-w-0 space-y-3 overflow-hidden">
      {headerTitle ? (
        <div className="listen-hush-live-group__title wails-drag px-2">
          {headerTitle}
        </div>
      ) : null}
      <HushLiveCardRow
        scrollNextLabel={props.text.actions.next}
        scrollPreviousLabel={props.text.actions.previous}
      >
        {props.items.map((item) => {
          const status = resolveVisibleHushLiveStatus(props.liveStatuses?.[item.videoId]);
          const canEdit = Boolean(props.onEdit && (!props.canEdit || props.canEdit(item)));
          const canRemove = Boolean(props.onRemove && (!props.canRemove || props.canRemove(item)));
          const removeConfirming = Boolean(props.isRemoveConfirming?.(item));
          const hasActions = canEdit || canRemove;
          const removeButtonLabel = removeConfirming
            ? props.confirmRemoveLabel ?? props.text.listen.confirmRemoveChannel
            : props.removeLabel ?? props.text.listen.removeChannel;
          return (
            <div
              key={item.id}
              className="listen-hush-card group/hush-card relative w-[10rem] shrink-0 snap-start"
              data-selected={item.id === props.selectedId ? "true" : undefined}
            >
              <div className="relative">
                <button
                  type="button"
                  className="listen-hush-card__artwork-button block w-full"
                  onClick={() => props.onSelect(item)}
                >
                  <HushLiveCardArtwork
                    httpBaseURL={props.httpBaseURL}
                    item={item}
                    soften={Boolean(status)}
                  >
                    <HushLiveCoverOverlay status={status} text={props.text} />
                  </HushLiveCardArtwork>
                </button>
                {hasActions ? (
                  <HushLiveCardActionGroup
                    canEdit={canEdit}
                    editLabel={props.editLabel ?? props.text.listen.editChannel}
                    canRemove={canRemove}
                    removeLabel={removeButtonLabel}
                    removeConfirming={removeConfirming}
                    onEdit={() => props.onEdit?.(item)}
                    onRemove={() => props.onRemove?.(item)}
                  />
                ) : null}
              </div>
              <button
                type="button"
                className="listen-hush-card__identity-button block w-full"
                onClick={() => props.onSelect(item)}
              >
                <div className="min-w-0 px-0.5 pt-2">
                  <div className="listen-hush-card__title truncate">
                    {item.title}
                  </div>
                  <div className="listen-hush-card__channel truncate">
                    {item.channel}
                  </div>
                </div>
              </button>
            </div>
          );
        })}
      </HushLiveCardRow>
    </section>
  );
}

function HushLiveCardRow(props: {
  scrollNextLabel: string;
  scrollPreviousLabel: string;
  children: React.ReactNode;
}) {
  const scrollRef = React.useRef<HTMLDivElement | null>(null);
  const [canScrollLeft, setCanScrollLeft] = React.useState(false);
  const [canScrollRight, setCanScrollRight] = React.useState(false);

  const updateScrollState = React.useCallback(() => {
    const element = scrollRef.current;
    if (!element) {
      setCanScrollRight(false);
      return;
    }
    const lastChild = element.lastElementChild;
    setCanScrollLeft(element.scrollLeft > 1);
    if (!lastChild) {
      setCanScrollRight(false);
      return;
    }
    const rowRect = element.getBoundingClientRect();
    const childRect = lastChild.getBoundingClientRect();
    setCanScrollRight(childRect.right - rowRect.right > 1);
  }, []);

  React.useLayoutEffect(() => {
    const element = scrollRef.current;
    if (!element || typeof window === "undefined") {
      return;
    }

    let frame = 0;
    const scheduleUpdate = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(updateScrollState);
    };
    const resizeObserver =
      typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(scheduleUpdate)
        : null;

    updateScrollState();
    element.addEventListener("scroll", scheduleUpdate, { passive: true });
    window.addEventListener("resize", scheduleUpdate);
    resizeObserver?.observe(element);
    Array.from(element.children).forEach((child) => resizeObserver?.observe(child));

    return () => {
      window.cancelAnimationFrame(frame);
      element.removeEventListener("scroll", scheduleUpdate);
      window.removeEventListener("resize", scheduleUpdate);
      resizeObserver?.disconnect();
    };
  }, [props.children, updateScrollState]);

  const scrollByPage = React.useCallback((direction: 1 | -1) => {
    const element = scrollRef.current;
    if (!element) {
      return;
    }
    element.scrollBy({
      left: direction * Math.max(element.clientWidth * 0.82, 160),
      behavior: "smooth",
    });
  }, []);

  return (
    <div className="listen-horizontal-card-row relative min-w-0 overflow-hidden">
      <div
        ref={scrollRef}
        className="flex min-w-0 snap-x snap-mandatory gap-4 overflow-x-auto overflow-y-hidden pb-1 pr-10 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
      >
        {props.children}
      </div>
      {canScrollLeft ? (
        <HushLiveScrollButton
          side="left"
          label={props.scrollPreviousLabel}
          onClick={() => scrollByPage(-1)}
        />
      ) : null}
      {canScrollRight ? (
        <HushLiveScrollButton
          side="right"
          label={props.scrollNextLabel}
          onClick={() => scrollByPage(1)}
        />
      ) : null}
    </div>
  );
}

function HushLiveScrollButton(props: {
  side: "left" | "right";
  label: string;
  onClick: () => void;
}) {
  const isLeft = props.side === "left";
  return (
    <div
      className={cn(
        "listen-horizontal-scroll-fade absolute top-0 z-30 h-[10rem] w-20",
        isLeft ? "left-0" : "right-0",
      )}
      data-side={props.side}
      onPointerDown={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        className={cn(
          "listen-horizontal-scroll-button flex h-full w-full items-center",
          isLeft ? "justify-start pl-1.5" : "justify-end pr-1.5",
        )}
        aria-label={props.label}
        title={props.label}
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          props.onClick();
        }}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <span className="app-listen-horizontal-scroll-control flex h-10 w-10 items-center justify-center">
          {isLeft ? (
            <ChevronLeft className="h-4 w-4" />
          ) : (
            <ChevronRight className="h-4 w-4" />
          )}
        </span>
      </button>
    </div>
  );
}

function HushLiveCardArtwork(props: {
  httpBaseURL: string;
  item: ListenOnlineItem;
  soften?: boolean;
  children?: React.ReactNode;
}) {
  const candidates = React.useMemo(
    () => buildListenPosterCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item.thumbnailUrl, props.item.videoId],
  );
  return (
    <div className="listen-hush-card-artwork relative aspect-square w-full overflow-hidden">
      <ListenCoverArtwork
        alt=""
        candidates={candidates}
        className="h-full w-full"
        imageClassName="listen-hush-card-image"
        softenClassName={
          props.soften
            ? "listen-hush-card-soften--visible"
            : "listen-hush-card-soften--hidden"
        }
      />
      {props.children}
    </div>
  );
}

function HushLiveCardActionGroup(props: {
  canEdit: boolean;
  editLabel: string;
  canRemove: boolean;
  removeLabel: string;
  removeConfirming: boolean;
  onEdit: () => void;
  onRemove: () => void;
}) {
  const anchorRef = React.useRef<HTMLDivElement | null>(null);
  return (
    <div className="app-listen-hush-card-action-reveal pointer-events-none absolute bottom-1.5 left-1/2 z-20 -translate-x-1/2">
      <div
        ref={anchorRef}
        className="app-dream-button-group app-listen-hush-card-action-group inline-flex h-8 items-center p-0.5"
      >
        {props.canEdit ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="compactIcon"
                shape="square"
                className="listen-hush-card-edit-action !h-7 !min-h-7 !w-7 !min-w-7"
                aria-label={props.editLabel}
                onClick={props.onEdit}
              >
                <Pencil className="h-3 w-3" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">{props.editLabel}</TooltipContent>
          </Tooltip>
        ) : null}
        {props.canRemove ? (
          <Button
            type="button"
            variant="ghost"
            size="compactIcon"
            tone="destructive"
            className="listen-destructive-icon-button !h-7 !min-h-7 !w-7 !min-w-7"
            data-confirming={props.removeConfirming ? "true" : undefined}
            aria-label={props.removeLabel}
            onClick={props.onRemove}
          >
            {props.removeConfirming ? (
              <Check className="h-3 w-3" />
            ) : (
              <Trash2 className="h-3 w-3" />
            )}
          </Button>
        ) : null}
      </div>
      <ListenConfirmPopup open={props.removeConfirming} anchorRef={anchorRef}>
        <div className="listen-status-text listen-hush-confirm-message flex items-center gap-2" data-tone="error">
          <Check className="h-3.5 w-3.5 shrink-0" />
          <span className="whitespace-nowrap">{props.removeLabel}</span>
        </div>
      </ListenConfirmPopup>
    </div>
  );
}

function HushLiveCoverOverlay(props: {
  status: ListenLiveStatusValue | "";
  text: TextBundle;
}) {
  return (
    <div
      className={cn(
        "listen-hush-live-cover-overlay absolute inset-0 z-10 flex items-center justify-center",
        !props.status && "listen-playback-hover-layer",
      )}
    >
      {props.status ? (
        <HushLiveStatusPill status={props.status} text={props.text} />
      ) : (
        <span
          className={cn(
            LISTEN_PRIMARY_PLAY_BUTTON_CLASS,
            LISTEN_PRIMARY_PLAY_BUTTON_SIZE_CLASS.small,
            "listen-playback-hover-button",
          )}
        >
          <Play className={cn("listen-playback-icon--filled ml-0.5", LISTEN_PRIMARY_PLAY_ICON_SIZE_CLASS.small)} />
        </span>
      )}
    </div>
  );
}

function HushLiveStatusPill(props: {
  status: ListenLiveStatusValue;
  text: TextBundle;
}) {
  const label =
    props.status === "offline"
      ? props.text.listen.liveStatusOffline
      : props.status === "upcoming"
        ? props.text.listen.liveStatusUpcoming
        : props.text.listen.liveStatusUnavailable;
  return (
    <StatusBadge
      className="listen-hush-live-status-badge max-w-[calc(100%-1rem)]"
      data-status={props.status}
      tone={
        props.status === "upcoming"
          ? "warning"
          : props.status === "unavailable"
            ? "danger"
            : "muted"
      }
    >
      {label}
    </StatusBadge>
  );
}

function ListenConfirmPopup(props: {
  open: boolean;
  anchorRef: React.RefObject<HTMLElement | null>;
  children: React.ReactNode;
}) {
  const popupRef = React.useRef<HTMLDivElement | null>(null);
  const [position, setPosition] = React.useState<{ left: number; top: number } | null>(null);

  React.useLayoutEffect(() => {
    if (!props.open || typeof window === "undefined") {
      setPosition(null);
      return;
    }

    const updatePosition = () => {
      const anchor = props.anchorRef.current;
      const popup = popupRef.current;
      if (!anchor || !popup) {
        return;
      }
      const anchorRect = anchor.getBoundingClientRect();
      const popupRect = popup.getBoundingClientRect();
      const boundaryRect =
        anchor.closest(".listen-list-surface, .app-dialog-content")?.getBoundingClientRect() ??
        document.documentElement.getBoundingClientRect();
      const margin = 8;
      const boundaryLeft = boundaryRect.left + margin;
      const boundaryRight = boundaryRect.right - margin;
      const maxLeft = Math.max(boundaryLeft, boundaryRight - popupRect.width);
      const desiredLeft = anchorRect.left + anchorRect.width / 2 - popupRect.width / 2;
      const left = Math.min(Math.max(desiredLeft, boundaryLeft), maxLeft);
      const top =
        anchorRect.top - popupRect.height - 6 >= margin
          ? anchorRect.top - popupRect.height - 6
          : anchorRect.bottom + 6;
      setPosition({ left, top });
    };

    updatePosition();
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [props.anchorRef, props.open, props.children]);

  if (!props.open || typeof document === "undefined") {
    return null;
  }

  return createPortal(
    <div
      ref={popupRef}
      className="app-glass-surface app-menu-content listen-confirm-popup pointer-events-none fixed z-[var(--app-layer-popover)] w-max min-w-0 max-w-[15rem] px-2.5 py-2"
      data-elevation="floating"
      data-positioned={position ? "true" : "false"}
      data-shape="control"
      data-tint="neutral"
      style={{
        left: position?.left ?? 0,
        top: position?.top ?? 0,
      }}
      {...getXiaSurfaceAttributes("overlay")}
    >
      {props.children}
    </div>,
    document.body,
  );
}

function resolveVisibleHushLiveStatus(status: ListenLiveStatus | undefined): ListenLiveStatusValue | "" {
  if (!status) {
    return "";
  }
  return status.status === "offline" ||
    status.status === "upcoming" ||
    status.status === "unavailable"
    ? status.status
    : "";
}

function ListenHushChannelDialog(props: {
  httpBaseURL: string;
  draft: ChannelDraft | null;
  liveGroups: ListenLiveGroup[];
  catalog: ListenLiveUserCatalog;
  saving: boolean;
  text: TextBundle;
  onChange: (draft: ChannelDraft | null) => void;
  onCancel: () => void;
  onResolve: (draft: AddChannelDraft) => Promise<void>;
  onSave: (draft: ChannelDraft) => Promise<void>;
}) {
  const draft = props.draft;
  const title = props.text.listen.editChannel;
  const preview = draft ? resolveChannelDialogPreview(draft) : null;
  const columnOptions = React.useMemo(
    () => buildChannelColumnOptions(props.liveGroups, props.catalog, props.text),
    [props.liveGroups, props.catalog, props.text],
  );
  const busy = props.saving || (draft?.kind === "add" && draft.loading);
  const formId = "listen-hush-channel-form";
  return (
    <Dialog open={draft !== null} onOpenChange={(open) => !open && props.onCancel()}>
      <DialogContent
        showCloseButton={draft?.kind !== "add"}
        className={cn(
          "grid max-h-[min(34rem,calc(100vh-2rem))] w-[min(24rem,calc(100vw-2rem))] max-w-none gap-4 overflow-hidden",
          draft?.kind === "edit"
            ? "grid-rows-[auto_minmax(0,1fr)_auto]"
            : "grid-rows-[minmax(0,1fr)_auto]",
        )}
      >
        {draft?.kind === "edit" ? (
          <DialogHeader>
            <DialogTitle className="listen-hush-dialog-title">{title}</DialogTitle>
          </DialogHeader>
        ) : null}
        {draft ? (
          <DialogScrollArea
            as="form"
            id={formId}
            className="min-h-0 space-y-3"
            onSubmit={(event) => {
              event.preventDefault();
              if (draft.kind === "add" && !draft.preview) {
                void props.onResolve(draft);
                return;
              }
              void props.onSave(draft);
            }}
          >
            {draft.kind === "add" && !draft.preview ? (
              <Input
                autoFocus
                value={draft.url}
                placeholder={props.text.listen.addChannelPlaceholder}
                onChange={(event) =>
                  props.onChange({
                    ...draft,
                    url: event.currentTarget.value,
                    error: "",
                  })
                }
              />
            ) : preview ? (
              <>
                <ChannelDialogPreview
                  httpBaseURL={props.httpBaseURL}
                  preview={preview}
                />
                <Field label={props.text.listen.channelTitle}>
                  <Input
                    autoFocus
                    value={draft.title}
                    placeholder={props.text.listen.channelTitle}
                    onChange={(event) =>
                      props.onChange({ ...draft, title: event.currentTarget.value, error: "" })
                    }
                  />
                </Field>
                <Field label={props.text.listen.channelColumn}>
                  <Select
                    value={draft.columnId}
                    className="h-9 w-full px-3"
                    onChange={(event) =>
                      props.onChange({ ...draft, columnId: event.currentTarget.value })
                    }
                  >
                    <option value="">{props.text.listen.noColumn}</option>
                    {columnOptions.map((option) => (
                      <option key={option.id} value={option.id}>
                        {option.title}
                      </option>
                    ))}
                  </Select>
                </Field>
              </>
            ) : null}
            {draft.error ? (
              <div className="listen-status-panel px-3 py-2" data-tone="error">
                {draft.error}
              </div>
            ) : null}
          </DialogScrollArea>
        ) : null}
        <DialogFooter>
          <Button type="button" variant="outline" size="compact" onClick={props.onCancel}>
            {props.text.actions.close}
          </Button>
          <Button
            type="submit"
            form={formId}
            size="compact"
            disabled={!draft || busy}
          >
            {busy ? <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" /> : null}
            {draft?.kind === "edit" ? props.text.actions.save : props.text.listen.addChannel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ChannelDialogPreview(props: {
  httpBaseURL: string;
  preview: Pick<ListenLiveChannelPreview, "videoId" | "title" | "channel" | "description" | "durationLabel" | "thumbnailUrl">;
}) {
  return (
    <div className="listen-hush-channel-preview grid justify-items-center gap-2">
      <ChannelDialogPreviewArtwork
        httpBaseURL={props.httpBaseURL}
        preview={props.preview}
      />
      <div className="min-w-0 max-w-full">
        <div className="listen-hush-channel-preview__title truncate">
          {props.preview.channel || props.preview.title}
        </div>
        <div className="listen-hush-channel-preview__description mt-1 line-clamp-2">
          {props.preview.description || props.preview.durationLabel}
        </div>
      </div>
    </div>
  );
}

function ChannelDialogPreviewArtwork(props: {
  httpBaseURL: string;
  preview: Pick<ListenLiveChannelPreview, "videoId" | "thumbnailUrl" | "title" | "channel">;
}) {
  const candidates = React.useMemo(
    () => buildListenTrackThumbnailCandidates(props.httpBaseURL, {
      videoId: props.preview.videoId,
      thumbnailUrl: props.preview.thumbnailUrl,
    }),
    [props.httpBaseURL, props.preview.thumbnailUrl, props.preview.videoId],
  );
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  React.useEffect(() => {
    setCandidateIndex(0);
  }, [candidates]);
  const src = candidates[candidateIndex] ?? "";
  return (
    <div className="listen-hush-channel-preview-artwork flex h-20 w-20 items-center justify-center overflow-hidden">
      {src ? (
        <img
          src={src}
          alt=""
          className="h-full w-full object-cover"
          onError={() => setCandidateIndex((current) => current + 1)}
        />
      ) : (
        <span className="listen-hush-channel-preview-artwork__fallback">
          {(props.preview.channel || props.preview.title || "Y").slice(0, 1).toUpperCase()}
        </span>
      )}
    </div>
  );
}

function resolveChannelDialogPreview(draft: ChannelDraft): ListenLiveChannelPreview {
  if (draft.kind === "add" && draft.preview) {
    return draft.preview;
  }
  if (draft.kind === "edit") {
    return {
      videoId: draft.videoId,
      title: draft.title,
      channel: draft.channel,
      description: draft.description,
      durationLabel: "LIVE",
      thumbnailUrl: draft.thumbnailUrl,
    };
  }
  return {
    videoId: "",
    title: "",
    channel: "",
    description: "",
    durationLabel: "",
    thumbnailUrl: "",
  };
}

function ListenHushColumnsDialog(props: {
  open: boolean;
  text: TextBundle;
  liveGroups: ListenLiveGroup[];
  catalog: ListenLiveUserCatalog;
  saving: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (catalog: ListenLiveUserCatalog) => Promise<void>;
}) {
  const [draftTitle, setDraftTitle] = React.useState("");
  const [edits, setEdits] = React.useState<Record<string, string>>({});
  const [editingColumnId, setEditingColumnId] = React.useState("");
  const [error, setError] = React.useState("");
  const [busyId, setBusyId] = React.useState("");
  const [confirmRemoveId, setConfirmRemoveId] = React.useState("");
  const builtInGroups = props.liveGroups.filter((group) => !isUserLiveGroup(group));
  const channelCountByColumn = React.useMemo(() => {
    const counts = new Map<string, number>();
    props.catalog.channels.forEach((channel) => {
      counts.set(channel.columnId, (counts.get(channel.columnId) ?? 0) + 1);
    });
    return counts;
  }, [props.catalog.channels]);

  React.useEffect(() => {
    if (!props.open) {
      setDraftTitle("");
      setEdits({});
      setEditingColumnId("");
      setError("");
      setBusyId("");
      setConfirmRemoveId("");
    }
  }, [props.open]);

  const save = React.useCallback(
    async (catalog: ListenLiveUserCatalog, id: string) => {
      setBusyId(id);
      setError("");
      try {
        await props.onSave(catalog);
      } catch (nextError) {
        setError(resolveUserCatalogError(nextError, props.text));
      } finally {
        setBusyId("");
      }
    },
    [props],
  );

  const startEditColumn = React.useCallback((column: ListenLiveUserColumn) => {
    setConfirmRemoveId("");
    setEditingColumnId(column.id);
    setEdits((current) => ({
      ...current,
      [column.id]: current[column.id] ?? column.title,
    }));
  }, []);

  const cancelEditColumn = React.useCallback((columnId: string) => {
    setEditingColumnId("");
    setEdits((current) => {
      const next = { ...current };
      delete next[columnId];
      return next;
    });
    setError("");
  }, []);

  const addColumn = React.useCallback(async () => {
    const title = draftTitle.trim();
    if (!title) {
      setError(props.text.listen.columnTitleRequired);
      return;
    }
    if (hasColumnTitle(props.catalog, props.liveGroups, title)) {
      setError(props.text.listen.columnAlreadyExists);
      return;
    }
    await save({
      columns: [
        ...props.catalog.columns,
        { id: createUserCatalogId("listen-live-column"), title, sortOrder: props.catalog.columns.length },
      ],
      channels: props.catalog.channels,
    }, "new");
    setDraftTitle("");
  }, [draftTitle, props.catalog, props.liveGroups, props.text, save]);

  const updateColumn = React.useCallback(
    async (column: ListenLiveUserColumn) => {
      const title = (edits[column.id] ?? column.title).trim();
      if (!title) {
        setError(props.text.listen.columnTitleRequired);
        return;
      }
      if (title === column.title) {
        cancelEditColumn(column.id);
        return;
      }
      if (hasColumnTitle(props.catalog, props.liveGroups, title, column.id)) {
        setError(props.text.listen.columnAlreadyExists);
        return;
      }
      await save({
        columns: props.catalog.columns.map((entry) =>
          entry.id === column.id ? { ...entry, title } : entry,
        ),
        channels: props.catalog.channels,
      }, column.id);
      setEditingColumnId("");
      setEdits((current) => {
        const next = { ...current };
        delete next[column.id];
        return next;
      });
    },
    [cancelEditColumn, edits, props.catalog, props.liveGroups, props.text, save],
  );

  const removeColumn = React.useCallback(
    async (column: ListenLiveUserColumn) => {
      if (confirmRemoveId !== column.id) {
        cancelEditColumn(column.id);
        setConfirmRemoveId(column.id);
        return;
      }
      await save({
        columns: props.catalog.columns.filter((entry) => entry.id !== column.id),
        channels: props.catalog.channels.filter((channel) => channel.columnId !== column.id),
      }, column.id);
      setConfirmRemoveId("");
    },
    [cancelEditColumn, confirmRemoveId, props.catalog, save],
  );

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className="grid max-h-[min(36rem,calc(100vh-2rem))] w-[min(31rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-4 overflow-hidden">
        <DialogHeader>
          <DialogTitle className="listen-hush-dialog-title">{props.text.listen.manageColumns}</DialogTitle>
        </DialogHeader>
        <DialogScrollArea className="min-h-0 space-y-4">
          <div className="grid grid-cols-[minmax(0,1fr)_2rem] gap-2">
            <Input
              autoFocus
              value={draftTitle}
              placeholder={props.text.listen.customColumnPlaceholder}
              onChange={(event) => {
                setDraftTitle(event.currentTarget.value);
                setError("");
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void addColumn();
                }
              }}
            />
            <Button
              type="button"
              variant="outline"
              size="compactIcon"
              shape="circle"
              className={cn("h-8 w-8", LISTEN_CONTROL_ICON_BUTTON_CLASS)}
              aria-label={props.text.listen.addColumn}
              disabled={props.saving || busyId === "new"}
              onClick={() => void addColumn()}
            >
              {busyId === "new" ? (
                <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
              ) : (
                <Plus className="h-3.5 w-3.5" />
              )}
            </Button>
          </div>

          <ColumnSection title={props.text.listen.builtInColumns}>
            {builtInGroups.length > 0 ? (
              builtInGroups.map((group) => (
                <DialogRow
                  key={group.id}
                  className="listen-hush-column-row grid min-h-9 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 px-3"
                >
                  <span className="listen-hush-column-row__title truncate">
                    {resolveHushLiveGroupTitle(group, props.text)}
                  </span>
                  <em className="listen-hush-column-row__count">
                    {group.items.length}
                  </em>
                  <small className="listen-hush-column-row__readonly">
                    {props.text.listen.readonlyColumn}
                  </small>
                </DialogRow>
              ))
            ) : (
              <EmptyColumns text={props.text} />
            )}
          </ColumnSection>

          <ColumnSection title={props.text.listen.customColumns}>
            {props.catalog.columns.length > 0 ? (
              props.catalog.columns.map((column) => {
                const value = edits[column.id] ?? column.title;
                const changed = value.trim() !== column.title;
                const busy = busyId === column.id;
                const editing = editingColumnId === column.id;
                const confirming = confirmRemoveId === column.id;
                const editButtonLabel = editing
                  ? props.text.listen.saveColumn
                  : props.text.listen.editColumn;
                const removeButtonLabel = confirming
                  ? props.text.listen.confirmRemoveColumn
                  : props.text.listen.removeColumn;
                return (
                  <DialogRow
                    key={column.id}
                    className="listen-hush-column-row relative grid min-h-9 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 px-3 py-1.5"
                  >
                    <div className="grid min-h-8 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
                      {editing ? (
                        <Input
                          autoFocus
                          value={value}
                          className="listen-hush-column-edit-input h-8"
                          onChange={(event) => {
                            setEdits((current) => ({
                              ...current,
                              [column.id]: event.currentTarget.value,
                            }));
                            setError("");
                          }}
                          onKeyDown={(event) => {
                            if (event.key === "Enter") {
                              event.preventDefault();
                              void updateColumn(column);
                            }
                            if (event.key === "Escape") {
                              event.preventDefault();
                              cancelEditColumn(column.id);
                            }
                          }}
                        />
                      ) : (
                        <button
                          type="button"
                          className="listen-hush-column-edit-trigger -ml-2 flex h-8 min-w-0 items-center px-2"
                          aria-label={props.text.listen.editColumn}
                          onClick={() => startEditColumn(column)}
                        >
                          <span className="truncate">{column.title}</span>
                        </button>
                      )}
                      <em className="listen-hush-column-row__count">
                        {channelCountByColumn.get(column.id) ?? 0}
                      </em>
                    </div>
                    <HushColumnActionGroup
                      editing={editing}
                      changed={changed}
                      busy={busy}
                      saving={props.saving}
                      confirming={confirming}
                      editLabel={editButtonLabel}
                      removeLabel={removeButtonLabel}
                      onEditOrSave={() =>
                        editing ? void updateColumn(column) : startEditColumn(column)
                      }
                      onRemove={() => void removeColumn(column)}
                    />
                  </DialogRow>
                );
              })
            ) : (
              <EmptyColumns text={props.text} />
            )}
          </ColumnSection>

          {error ? (
            <div className="listen-status-panel px-3 py-2" data-tone="error">
              {error}
            </div>
          ) : null}
        </DialogScrollArea>
        <DialogFooter>
          <Button type="button" size="compact" onClick={() => props.onOpenChange(false)}>
            {props.text.actions.close}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Field(props: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="listen-hush-field-label">{props.label}</span>
      {props.children}
    </label>
  );
}

function HushColumnActionGroup(props: {
  editing: boolean;
  changed: boolean;
  busy: boolean;
  saving: boolean;
  confirming: boolean;
  editLabel: string;
  removeLabel: string;
  onEditOrSave: () => void;
  onRemove: () => void;
}) {
  const anchorRef = React.useRef<HTMLDivElement | null>(null);
  return (
    <div className="relative">
      <div
        ref={anchorRef}
        className="app-dream-button-group inline-flex h-9 shrink-0 items-center p-0.5"
      >
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="app-completed-toolbar-button h-8 w-8 p-0"
          aria-label={props.editLabel}
          disabled={props.saving || props.busy}
          onClick={props.onEditOrSave}
        >
          {props.busy && props.editing && props.changed ? (
            <Loader2 className="h-3 w-3 listen-loading-spinner" />
          ) : props.editing ? (
            <Check className="h-3 w-3" />
          ) : (
            <Pencil className="h-3 w-3" />
          )}
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          tone="destructive"
          className="app-completed-toolbar-button listen-destructive-icon-button h-8 w-8 p-0"
          data-confirming={props.confirming ? "true" : undefined}
          aria-label={props.removeLabel}
          disabled={props.saving || props.busy}
          onClick={props.onRemove}
        >
          {props.confirming ? (
            <Check className="h-3 w-3" />
          ) : (
            <Trash2 className="h-3 w-3" />
          )}
        </Button>
      </div>
      <ListenConfirmPopup open={props.confirming} anchorRef={anchorRef}>
        <div className="listen-status-text listen-hush-confirm-message flex items-center gap-2" data-tone="error">
          <Check className="h-3.5 w-3.5 shrink-0" />
          <span className="whitespace-nowrap">{props.removeLabel}</span>
        </div>
      </ListenConfirmPopup>
    </div>
  );
}

function ColumnSection(props: { title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2">
      <h3 className="listen-hush-column-section__title px-1">
        {props.title}
      </h3>
      <DialogListCard className="listen-hush-column-card">
        <DialogListCardContent>{props.children}</DialogListCardContent>
      </DialogListCard>
    </section>
  );
}

function EmptyColumns(props: { text: TextBundle }) {
  return (
    <div className="listen-hush-empty-columns m-3 px-3 py-3">
      {props.text.listen.noColumns}
    </div>
  );
}

function buildCatalogWithChannel(
  catalog: ListenLiveUserCatalog,
  liveGroups: ListenLiveGroup[],
  draft: ChannelDraft,
  text: TextBundle,
): { catalog: ListenLiveUserCatalog } | { error: string } {
  const preview = resolveChannelDialogPreview(draft);
  const videoId = preview.videoId.trim();
  const title = draft.title.trim();
  if (!videoId) {
    return { error: text.listen.channelVideoIdRequired };
  }
  if (!title) {
    return { error: text.listen.channelTitleRequired };
  }
  const columnResult = resolveDraftColumn(catalog, liveGroups, draft.columnId, text);
  const sortOrder =
    draft.kind === "edit"
      ? catalog.channels.find((channel) => channel.id === draft.id)?.sortOrder ?? 0
      : catalog.channels.filter((channel) => channel.columnId === columnResult.columnId).length;
  const channel: ListenLiveUserChannel = {
    id: draft.kind === "edit" && draft.id ? draft.id : createUserCatalogId("listen-live-channel"),
    columnId: columnResult.columnId,
    title,
    channel: preview.channel.trim(),
    description: preview.description.trim(),
    source: "youtube_music",
    videoId,
    thumbnailUrl: preview.thumbnailUrl.trim(),
    enabled: true,
    sortOrder,
  };
  const channels =
    draft.kind === "edit"
      ? catalog.channels.map((entry) => (entry.id === channel.id ? channel : entry))
      : [...catalog.channels, channel];
  return {
    catalog: {
      columns: columnResult.columns,
      channels: draft.kind === "edit" && !catalog.channels.some((entry) => entry.id === channel.id)
        ? [...catalog.channels, channel]
        : channels,
    },
  };
}

function resolveDraftColumn(
  catalog: ListenLiveUserCatalog,
  liveGroups: ListenLiveGroup[],
  columnId: string,
  text: TextBundle,
): { columns: ListenLiveUserColumn[]; columnId: string } {
  const trimmedColumnId = columnId.trim();
  const existing = catalog.columns.find((column) => column.id === trimmedColumnId);
  if (existing) {
    return { columns: catalog.columns, columnId: existing.id };
  }
  if (liveGroups.some((group) => !isUserLiveGroup(group) && group.id === trimmedColumnId)) {
    return { columns: catalog.columns, columnId: trimmedColumnId };
  }
  const fallbackTitle = text.listen.customColumns;
  const fallback =
    catalog.columns.find((column) => normalizeTitleKey(column.title) === normalizeTitleKey(fallbackTitle)) ??
    {
      id: createUserCatalogId("listen-live-column"),
      title: fallbackTitle,
      sortOrder: catalog.columns.length,
    };
  return {
    columns: catalog.columns.some((column) => column.id === fallback.id)
      ? catalog.columns
      : [...catalog.columns, fallback],
    columnId: fallback.id,
  };
}

function normalizeUserCatalog(
  catalog: ListenLiveUserCatalog,
  liveGroups: ListenLiveGroup[],
): ListenLiveUserCatalog {
  const columns = [...catalog.columns]
    .filter((column) => column.id.trim() && column.title.trim())
    .map((column, index) => ({
      id: column.id.trim(),
      title: column.title.trim(),
      sortOrder: Number.isFinite(column.sortOrder) ? column.sortOrder : index,
    }))
    .sort((left, right) => left.sortOrder - right.sortOrder || left.title.localeCompare(right.title))
    .map((column, index) => ({ ...column, sortOrder: index }));
  const columnIDs = new Set([
    ...columns.map((column) => column.id),
    ...liveGroups
      .filter((group) => !isUserLiveGroup(group))
      .map((group) => group.id.trim())
      .filter(Boolean),
  ]);
  const nextSortOrderByColumn = new Map<string, number>();
  const channels = [...catalog.channels]
    .map((channel) => ({
      ...channel,
      columnId: channel.columnId.trim(),
    }))
    .filter((channel) =>
      channel.id.trim() &&
      columnIDs.has(channel.columnId) &&
      channel.title.trim() &&
      channel.videoId.trim(),
    )
    .sort((left, right) =>
      left.columnId.localeCompare(right.columnId) ||
      left.sortOrder - right.sortOrder ||
      left.title.localeCompare(right.title),
    )
    .map((channel) => {
      const sortOrder = nextSortOrderByColumn.get(channel.columnId) ?? 0;
      nextSortOrderByColumn.set(channel.columnId, sortOrder + 1);
      return {
        ...channel,
        id: channel.id.trim(),
        columnId: channel.columnId.trim(),
        title: channel.title.trim(),
        channel: channel.channel.trim(),
        description: channel.description.trim(),
        source: channel.source.trim() || "youtube_music",
        videoId: channel.videoId.trim(),
        thumbnailUrl: channel.thumbnailUrl.trim(),
        enabled: channel.enabled !== false,
        sortOrder,
      };
    });
  return { columns, channels };
}

function resolveDefaultChannelColumnId(
  liveGroups: ListenLiveGroup[],
  catalog: ListenLiveUserCatalog,
) {
  return (
    liveGroups.find((group) => !isUserLiveGroup(group))?.id ||
    catalog.columns[0]?.id ||
    ""
  );
}

function buildChannelColumnOptions(
  liveGroups: ListenLiveGroup[],
  catalog: ListenLiveUserCatalog,
  text: TextBundle,
) {
  const seen = new Set<string>();
  const options: Array<{ id: string; title: string }> = [];
  liveGroups
    .filter((group) => !isUserLiveGroup(group))
    .forEach((group) => {
      const id = group.id.trim();
      if (!id || seen.has(id)) {
        return;
      }
      seen.add(id);
      options.push({ id, title: resolveHushLiveGroupTitle(group, text) });
    });
  catalog.columns.forEach((column) => {
    const id = column.id.trim();
    if (!id || seen.has(id)) {
      return;
    }
    seen.add(id);
    options.push({ id, title: column.title });
  });
  return options;
}

function hasColumnTitle(
  catalog: ListenLiveUserCatalog,
  liveGroups: ListenLiveGroup[],
  title: string,
  currentColumnId = "",
) {
  const key = normalizeTitleKey(title);
  return (
    catalog.columns.some(
      (column) => column.id !== currentColumnId && normalizeTitleKey(column.title) === key,
    ) ||
    liveGroups
      .filter((group) => !isUserLiveGroup(group))
      .some((group) => normalizeTitleKey(group.title) === key)
  );
}

function resolveHushLiveGroupTitle(group: ListenLiveGroup, text: TextBundle) {
  switch (group.id) {
    case "youtube":
      return text.listen.groupLive;
    case "stations":
      return text.listen.liveStations;
    default:
      return group.title || text.listen.liveStations;
  }
}

function isUserLiveGroup(group: ListenLiveGroup) {
  return group.id.startsWith("user-");
}

function normalizeTitleKey(value: string) {
  return value.trim().toLocaleLowerCase();
}

function createUserCatalogId(prefix: string) {
  const random =
    typeof crypto !== "undefined" && "randomUUID" in crypto
      ? crypto.randomUUID()
      : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 9)}`;
  return `${prefix}-${random}`;
}

function resolveUserCatalogError(error: unknown, text: TextBundle) {
  if (error instanceof Error && error.message.trim()) {
    return error.message.trim();
  }
  return text.listen.userCatalogSaveFailed;
}

function resolveChannelDialogError(error: unknown, text: TextBundle) {
  const message = error instanceof Error ? error.message.trim() : "";
  if (message.includes("Invalid YouTube live link")) {
    return text.listen.channelVideoIdRequired;
  }
  if (message.includes("not a live stream")) {
    return text.listen.channelNotLive;
  }
  return resolveUserCatalogError(error, text);
}
