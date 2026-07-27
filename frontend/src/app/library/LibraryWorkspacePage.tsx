import {
  ArrowDownAZ,
  AlertTriangle,
  CalendarDays,
  Check,
  CheckCircle2,
  CheckSquare,
  Circle,
  CircleAlert,
  CircleSlash,
  CircleCheck,
  CircleDashed,
  ChartPie,
  DatabaseBackup,
  Ellipsis,
  Eye,
  Grid2X2,
  FolderCog,
  List,
  LoaderCircle,
  Search,
  Trash2,
  X,
  type LucideIcon,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import type { LibraryDTO, OperationListItemDTO } from "@/shared/contracts/library";
import { useI18n } from "@/shared/i18n";
import {
  LIBRARY_CATALOG_ACTOR_ID,
  useTrashCatalogItem,
} from "@/shared/query/catalog";
import { useDeleteFiles, useDeleteOperations } from "@/shared/query/library";
import {
  SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,
  SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME,
  SIDEBAR_DROPDOWN_ITEM_CLASS_NAME,
} from "@/shared/styles/xiadown";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { StatusBadge, type DreamStatusTone } from "@/shared/ui/status-badge";
import {
  WorkspacePrimaryHeaderAction,
  WorkspacePrimaryHeaderActionGroup,
  WorkspacePrimaryHeaderMenuContent,
} from "@/shared/ui/workspace-primary-header-action";
import {
  defineWorkspacePageContract,
  WorkspacePage,
  WorkspacePageContent,
  WorkspacePageFooter,
  WorkspacePageTopBar,
} from "@/shared/ui/workspace-page";
import { WorkspaceSearchControl } from "@/shared/ui/workspace-search-control";
import { formatBytes } from "@/shared/utils/formatBytes";

import { adaptLegacyLibraryWorkspace } from "./legacy-adapter";
import { CatalogManagementDialog } from "./CatalogManagementDialog";
import {
  isLibraryDefaultArtworkURL,
  LibraryOtherGroupIcon,
} from "./LibraryArtwork";
import { LibraryCardArtwork } from "./LibraryCardArtwork";
import { LibraryPaginationFooter } from "./LibraryPaginationFooter";
import { TaskFolderArtwork } from "./TaskFolderArtwork";
import {
  DEFAULT_LIBRARY_PAGE_SIZE,
  clampLibraryPage,
  sliceLibraryPage,
} from "./library-pagination";
import {
  createLibraryWorkspaceLabels,
  libraryItemDisplayStatus,
  shouldShowLibraryStatusBadge,
  type LibraryOtherGroup,
  type LibraryWorkspaceItem,
  type LibraryWorkspaceLabels,
  type LibraryWorkspaceRoute,
} from "./types";
import {
  resolveInitialLibraryViewMode,
  writeLibraryViewMode,
  type LibraryViewMode,
} from "./view-preference";
import "./library.css";

export type LibrarySortMode = "updated" | "oldest" | "name" | "size";

export interface LibraryWorkspacePagination {
  page: number;
  pageSize: number;
  /** Exact filtered total returned by the backing query. */
  total?: number;
  /** True when `items` already contains only the requested backend page. */
  itemsArePage?: boolean;
  onPageChange: (page: number) => void;
  onPageSizeChange: (pageSize: number) => void;
}

const OTHER_GROUPS: LibraryOtherGroup[] = [
  "document",
  "font",
  "archive",
  "subtitle",
  "manifest",
  "api",
  "unknown",
  "needs-review",
  "missing",
];

export interface LibraryWorkspacePageProps {
  route: LibraryWorkspaceRoute;
  libraries?: readonly LibraryDTO[];
  operations?: readonly OperationListItemDTO[];
  items?: readonly LibraryWorkspaceItem[];
  httpBaseURL?: string;
  selectedItemId?: string;
  labels?: Partial<LibraryWorkspaceLabels>;
  initialQuery?: string;
  initialOtherGroup?: LibraryOtherGroup;
  initialView?: LibraryViewMode;
  query?: string;
  sort?: LibrarySortMode;
  otherGroup?: LibraryOtherGroup;
  pagination?: LibraryWorkspacePagination;
  otherCounts?: Partial<Record<LibraryOtherGroup, number>>;
  /** Native caption controls currently occupy the trailing edge of Primary. */
  reserveWindowControls?: boolean;
  loading?: boolean;
  loadError?: boolean;
  onRetry?: () => void;
  onQueryChange?: (query: string) => void;
  onSortChange?: (sort: LibrarySortMode) => void;
  onOtherGroupChange?: (group: LibraryOtherGroup) => void;
  onItemClick?: (item: LibraryWorkspaceItem) => void;
  onOpenDeletedItems?: () => void;
}

export interface LibraryDeletionPlan {
  operationIds: string[];
  catalogItems: Array<{ id: string; expectedRevision: number }>;
  fileIds: string[];
}

interface LibraryDeletionMutations {
  deleteOperations: (request: {
    operationIds: string[];
    cascadeFiles: boolean;
  }) => Promise<unknown>;
  trashCatalogItem: (request: {
    id: string;
    expectedRevision: number;
    actorId: string;
  }) => Promise<unknown>;
  deleteFiles: (request: {
    fileIds: string[];
    deleteFiles: boolean;
  }) => Promise<unknown>;
}

export interface LibraryItemDeletionFailure {
  item: LibraryWorkspaceItem;
  error: unknown;
}

export interface LibraryItemDeletionBatchResult {
  deletedItemIds: string[];
  failures: LibraryItemDeletionFailure[];
}

type LibraryContextMenuPoint = { x: number; y: number };

type LibraryContextMenuTarget = LibraryContextMenuPoint & {
  item: LibraryWorkspaceItem;
};

type LibraryDeleteConfirmation = {
  items: LibraryWorkspaceItem[];
};

export function isLibraryContextMenuKey(key: string, shiftKey = false) {
  return key === "ContextMenu" || (shiftKey && key === "F10");
}

export function resolveLibraryKeyboardContextMenuPoint(
  rect: Pick<DOMRect, "left" | "bottom" | "width">,
): LibraryContextMenuPoint {
  return {
    x: rect.left + rect.width / 2,
    y: rect.bottom,
  };
}

/**
 * Keeps logical Catalog lifecycle IDs separate from physical legacy file IDs.
 * The Catalog branch intentionally precedes the legacy-file branch so a
 * normalized Catalog card can never send its opaque item id to DeleteFiles.
 */
export function buildLibraryDeletionPlan(
  items: readonly LibraryWorkspaceItem[],
): LibraryDeletionPlan {
  const operationIds = new Set<string>();
  const catalogItems = new Map<string, { id: string; expectedRevision: number }>();
  const fileIds = new Set<string>();

  items.forEach((item) => {
    const operationId = item.operation?.operationId.trim() ?? "";
    if (operationId) {
      operationIds.add(operationId);
      return;
    }

    const catalogItem = item.catalogItem;
    if (catalogItem?.id.trim()) {
      catalogItems.set(catalogItem.id, {
        id: catalogItem.id,
        expectedRevision: catalogItem.revision,
      });
      return;
    }

    const fileId = item.file?.id.trim() ?? "";
    if (fileId) fileIds.add(fileId);
  });

  return {
    operationIds: [...operationIds],
    catalogItems: [...catalogItems.values()],
    fileIds: [...fileIds],
  };
}

export async function executeLibraryDeletionPlan(
  plan: LibraryDeletionPlan,
  mutations: LibraryDeletionMutations,
) {
  if (plan.operationIds.length > 0) {
    await mutations.deleteOperations({
      operationIds: plan.operationIds,
      cascadeFiles: true,
    });
  }
  // Catalog lifecycle calls carry a per-item optimistic revision, so keep
  // them ordered and stop on the first conflict instead of racing them.
  for (const request of plan.catalogItems) {
    await mutations.trashCatalogItem({
      ...request,
      actorId: LIBRARY_CATALOG_ACTOR_ID,
    });
  }
  if (plan.fileIds.length > 0) {
    await mutations.deleteFiles({
      fileIds: plan.fileIds,
      deleteFiles: true,
    });
  }
}

/**
 * Executes one logical card at a time so optimistic Catalog failures cannot
 * make an already-deleted card part of the next retry. Legacy batch endpoints
 * remain the transport, but each call now has an unambiguous UI outcome.
 */
export async function executeLibraryItemDeletionBatch(
  items: readonly LibraryWorkspaceItem[],
  mutations: LibraryDeletionMutations,
): Promise<LibraryItemDeletionBatchResult> {
  const deletedItemIds: string[] = [];
  const failures: LibraryItemDeletionFailure[] = [];

  for (const item of items) {
    try {
      const plan = buildLibraryDeletionPlan([item]);
      if (
        plan.operationIds.length === 0 &&
        plan.catalogItems.length === 0 &&
        plan.fileIds.length === 0
      ) {
        failures.push({ item, error: undefined });
        continue;
      }
      await executeLibraryDeletionPlan(plan, mutations);
      deletedItemIds.push(item.id);
    } catch (error) {
      failures.push({ item, error });
    }
  }

  return { deletedItemIds, failures };
}

function canDeleteLibraryItem(item: LibraryWorkspaceItem) {
  const plan = buildLibraryDeletionPlan([item]);
  return (
    plan.operationIds.length > 0 ||
    plan.catalogItems.length > 0 ||
    plan.fileIds.length > 0
  );
}

function mergedLabels(base: LibraryWorkspaceLabels, overrides?: Partial<LibraryWorkspaceLabels>) {
  return {
    ...base,
    ...overrides,
    otherGroups: {
      ...base.otherGroups,
      ...overrides?.otherGroups,
    },
  };
}

function routeTitle(route: LibraryWorkspaceRoute, labels: LibraryWorkspaceLabels) {
  if (route === "search") return labels.search;
  if (route === "ended") return labels.ended;
  if (route === "all") return labels.all;
  return labels[route];
}

const ENDED_OPERATION_STATUSES = new Set([
  "succeeded",
  "failed",
  "canceled",
  "cancelled",
]);

function isEndedTask(item: LibraryWorkspaceItem) {
  const status = (item.operation?.status || item.status).trim().toLowerCase();
  return item.source === "task" && ENDED_OPERATION_STATUSES.has(status);
}

function routeItems(
  route: LibraryWorkspaceRoute,
  files: readonly LibraryWorkspaceItem[],
  tasks: readonly LibraryWorkspaceItem[],
) {
  switch (route) {
    case "ended":
      return tasks.filter(isEndedTask);
    case "video":
      return files.filter((item) => item.category === "video");
    case "audio":
      return files.filter((item) => item.category === "audio");
    case "books":
      return files.filter((item) => item.category === "book");
    case "images":
      return files.filter((item) => item.category === "image");
    case "others":
      return files.filter((item) => item.category === "other");
    case "all":
    case "search":
      return files;
  }
}

function compareItems(mode: LibrarySortMode) {
  return (left: LibraryWorkspaceItem, right: LibraryWorkspaceItem) => {
    if (mode === "name") return left.title.localeCompare(right.title);
    if (mode === "size") return (right.sizeBytes ?? 0) - (left.sizeBytes ?? 0);
    const leftTime = Date.parse(mode === "oldest" ? left.createdAt : left.updatedAt) || 0;
    const rightTime = Date.parse(mode === "oldest" ? right.createdAt : right.updatedAt) || 0;
    return mode === "oldest" ? leftTime - rightTime : rightTime - leftTime;
  };
}

export function resolveControlledLibrarySearchExpanded(
  route: LibraryWorkspaceRoute,
  query: string,
) {
  return route === "search" || query.length > 0;
}

function formatDuration(durationMs?: number) {
  if (!durationMs || durationMs < 0) return "";
  const totalSeconds = Math.floor(durationMs / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
    : `${minutes}:${String(seconds).padStart(2, "0")}`;
}

function formatLibraryType(item: LibraryWorkspaceItem, labels: LibraryWorkspaceLabels) {
  const raw = (
    item.format ||
    item.operation?.kind ||
    item.file?.kind ||
    item.category
  ).trim();
  if (!raw) return "";
  if (item.category === "task") return labels.operationKindLabel(raw);
  if (/^[A-Z0-9]{2,6}$/.test(raw)) return raw;
  return labels.catalogValueLabel(raw);
}

function LibraryItemCard(props: {
  item: LibraryWorkspaceItem;
  selected: boolean;
  selectionMode: boolean;
  batchSelected: boolean;
  view: LibraryViewMode;
  labels: LibraryWorkspaceLabels;
  onClick?: (item: LibraryWorkspaceItem) => void;
  onToggleSelection: (item: LibraryWorkspaceItem) => void;
  onOpenContextMenu: (
    item: LibraryWorkspaceItem,
    point: LibraryContextMenuPoint,
    returnFocus: HTMLButtonElement,
  ) => void;
}) {
  const duration = formatDuration(props.item.durationMs);
  const size = props.item.sizeBytes ? formatBytes(props.item.sizeBytes) : "";
  const type = formatLibraryType(props.item, props.labels);
  const relativeUpdatedAt = props.labels.relativeTimeValue(props.item.updatedAt);
  const fullUpdatedAt = relativeUpdatedAt
    ? props.labels.dateTimeValue(props.item.updatedAt)
    : "";
  const imageWidth = props.item.file?.media?.width;
  const imageHeight = props.item.file?.media?.height;
  const itemCount = props.item.operation?.metrics.fileCount;
  const taskPreviewItems = props.item.category === "task"
    ? props.item.taskPreviewItems?.length
      ? props.item.taskPreviewItems
      : props.item.coverURL && !isLibraryDefaultArtworkURL(props.item.coverURL)
        ? [{
            id: `${props.item.id}:cover`,
            kind: "thumbnail",
            previewURL: props.item.coverURL,
          }]
        : []
    : [];
  const secondaryFacts = [
    size ? { label: props.labels.size, value: size } : null,
    (props.item.category === "video" || props.item.category === "audio") && duration
      ? { label: props.labels.duration, value: duration }
      : null,
    props.item.category === "task" && Number.isFinite(itemCount)
      ? { label: props.labels.itemCount(itemCount ?? 0), value: props.labels.itemCount(itemCount ?? 0) }
      : null,
    props.item.category === "image" && imageWidth && imageHeight
      ? { label: props.labels.resolution, value: `${imageWidth} × ${imageHeight}` }
      : null,
  ].filter((fact): fact is { label: string; value: string } => Boolean(fact));
  const displayStatus = libraryItemDisplayStatus(props.item);
  const normalizedStatus = displayStatus.trim().toLocaleLowerCase();
  const statusPresentation: {
    tone: DreamStatusTone;
    icon: LucideIcon;
  } = /missing|offline|unavailable|error|failed|corrupt/.test(normalizedStatus)
    ? { tone: "danger", icon: AlertTriangle }
    : /trashed|deleted|archived|cancel|paused|stopped/.test(normalizedStatus)
      ? { tone: "muted", icon: Trash2 }
      : /needs[_ -]?review|\breview\b/.test(normalizedStatus)
        ? { tone: "warning", icon: CircleAlert }
        : /running|processing|loading|queued|pending/.test(normalizedStatus)
          ? { tone: "busy", icon: CircleDashed }
          : /active|available|ready|success|succeeded|complete/.test(normalizedStatus)
            ? { tone: "success", icon: CircleCheck }
            : { tone: "neutral", icon: Circle };
  const StatusIcon = statusPresentation.icon;
  const statusLabel = props.labels.statusLabel(displayStatus);
  return (
    <button
      type="button"
      className="app-library-item"
      data-category={props.item.category}
      data-item-id={props.item.id}
      data-selected={props.selected ? "true" : "false"}
      data-selection-mode={props.selectionMode ? "true" : "false"}
      data-batch-selected={props.batchSelected ? "true" : "false"}
      data-view={props.view}
      aria-pressed={props.selectionMode ? props.batchSelected : undefined}
      onClick={() => {
        if (props.selectionMode) {
          props.onToggleSelection(props.item);
          return;
        }
        props.onClick?.(props.item);
      }}
      onContextMenu={(event) => {
        event.preventDefault();
        event.stopPropagation();
        props.onOpenContextMenu(
          props.item,
          { x: event.clientX, y: event.clientY },
          event.currentTarget,
        );
      }}
      onKeyDown={(event) => {
        if (!isLibraryContextMenuKey(event.key, event.shiftKey)) return;
        event.preventDefault();
        event.stopPropagation();
        props.onOpenContextMenu(
          props.item,
          resolveLibraryKeyboardContextMenuPoint(
            event.currentTarget.getBoundingClientRect(),
          ),
          event.currentTarget,
        );
      }}
    >
      {props.item.category === "task" ? (
        <TaskFolderArtwork
          className="app-library-item__artwork app-library-item__artwork--task-folder"
          items={taskPreviewItems}
          totalCount={
            props.item.taskPreviewTotalCount ??
            (Number.isFinite(itemCount) ? itemCount : taskPreviewItems.length)
          }
          view={props.view}
        />
      ) : (
        <span className="app-library-item__artwork" aria-hidden="true">
          <LibraryCardArtwork item={props.item} />
        </span>
      )}
      {props.selectionMode ? (
        <span
          className="app-library-item__selection-indicator"
          data-selected={props.batchSelected ? "true" : "false"}
          aria-hidden="true"
        >
          {props.batchSelected ? <Check /> : null}
        </span>
      ) : null}
      <span className="app-library-item__copy">
        <span className="app-library-item__title">{props.item.title}</span>
        <span className="app-library-item__classification">
          {shouldShowLibraryStatusBadge(displayStatus) ? (
            <StatusBadge
              className="app-library-item__status"
              icon={<StatusIcon />}
              tone={statusPresentation.tone}
              aria-label={`${props.labels.status}: ${statusLabel}`}
              title={statusLabel}
            >
              {statusLabel}
            </StatusBadge>
          ) : null}
          {type || relativeUpdatedAt ? (
            <span className="app-library-item__classification-detail">
              {type ? <span className="app-library-item__type">{type}</span> : null}
              {relativeUpdatedAt ? (
                <time
                  className="app-library-item__time"
                  dateTime={props.item.updatedAt}
                  title={fullUpdatedAt}
                  aria-label={`${props.labels.updated}: ${fullUpdatedAt}`}
                >
                  {relativeUpdatedAt}
                </time>
              ) : null}
            </span>
          ) : null}
        </span>
        {secondaryFacts.length > 0 ? (
          <span className="app-library-item__meta">
            {secondaryFacts.map((fact, index) => (
              <span
                key={`${fact.label}-${index}`}
                className="app-library-item__datum"
                aria-label={fact.label === fact.value ? fact.label : `${fact.label}: ${fact.value}`}
              >
                {fact.value}
              </span>
            ))}
          </span>
        ) : null}
      </span>
    </button>
  );
}

interface LibraryDeleteDialogText {
  cancel: string;
  deleteItem: string;
  unknownError: string;
  deleteTaskTitle: string;
  deleteTasksTitle: string;
  deleteFileTitle: string;
  deleteFilesTitle: string;
  deleteTaskMessage: string;
  deleteTasksMessage: string;
  deleteFileMessage: string;
  deleteFilesMessage: string;
}

function formatLibraryDeleteMessage(
  template: string,
  values: { name: string; count: number },
) {
  return template
    .replace("{name}", values.name)
    .replace("{count}", String(values.count));
}

function libraryDeleteDialogCopy(
  target: LibraryDeleteConfirmation,
  text: LibraryDeleteDialogText,
) {
  const tasks = target.items.every((item) => item.source === "task");
  const count = target.items.length;
  const multiple = count > 1;
  return {
    title: tasks
      ? multiple ? text.deleteTasksTitle : text.deleteTaskTitle
      : multiple ? text.deleteFilesTitle : text.deleteFileTitle,
    message: formatLibraryDeleteMessage(
      tasks
        ? multiple ? text.deleteTasksMessage : text.deleteTaskMessage
        : multiple ? text.deleteFilesMessage : text.deleteFileMessage,
      { name: target.items[0]?.title ?? "", count },
    ),
  };
}

function libraryMutationErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) return error.message;
  if (typeof error === "string" && error.trim()) return error;
  return fallback;
}

function LibraryDeleteConfirmationDialog(props: {
  target: LibraryDeleteConfirmation;
  text: LibraryDeleteDialogText;
  onCancel: () => void;
  onExecuted: (result: {
    deletedItemIds: readonly string[];
    failedItems: readonly LibraryWorkspaceItem[];
  }) => void;
}) {
  const deleteOperations = useDeleteOperations();
  const deleteFiles = useDeleteFiles();
  const trashCatalogItem = useTrashCatalogItem();
  const [executing, setExecuting] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState("");
  const dialogContent = libraryDeleteDialogCopy(props.target, props.text);
  const pending =
    executing ||
    deleteOperations.isPending ||
    deleteFiles.isPending ||
    trashCatalogItem.isPending;

  const execute = async () => {
    if (pending) return;
    const plan = buildLibraryDeletionPlan(props.target.items);
    if (
      plan.operationIds.length === 0 &&
      plan.catalogItems.length === 0 &&
      plan.fileIds.length === 0
    ) return;

    setExecuting(true);
    setErrorMessage("");
    const result = await executeLibraryItemDeletionBatch(props.target.items, {
      deleteOperations: (request) => deleteOperations.mutateAsync(request),
      trashCatalogItem: (request) => trashCatalogItem.mutateAsync(request),
      deleteFiles: (request) => deleteFiles.mutateAsync(request),
    });
    const failedItems = result.failures.map((failure) => failure.item);
    if (result.failures.length > 0) {
      setErrorMessage(
        libraryMutationErrorMessage(
          result.failures[0]?.error,
          props.text.unknownError,
        ),
      );
    }
    props.onExecuted({
      deletedItemIds: result.deletedItemIds,
      failedItems,
    });
    setExecuting(false);
  };

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !pending) props.onCancel();
      }}
    >
      <DialogContent
        className="app-library-delete-dialog"
        aria-busy={pending ? "true" : undefined}
      >
        <DialogHeader className="app-library-delete-dialog__header">
          <DialogTitle>{dialogContent.title}</DialogTitle>
          <DialogDescription>{dialogContent.message}</DialogDescription>
        </DialogHeader>
        <div className="app-library-delete-dialog__body">
          {errorMessage ? (
            <div
              className="app-dream-status-message app-library-delete-dialog__error"
              data-intent="danger"
              role="alert"
            >
              {errorMessage}
            </div>
          ) : null}
        </div>
        <DialogFooter className="app-library-delete-dialog__footer">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={props.onCancel}
          >
            {props.text.cancel}
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={pending}
            onClick={() => void execute()}
          >
            {pending ? <LoaderCircle className="app-motion-spin" /> : null}
            {props.text.deleteItem}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function LibraryWorkspacePage(props: LibraryWorkspacePageProps) {
  const { t, language } = useI18n();
  const localizedLabels = React.useMemo(
    () => createLibraryWorkspaceLabels(t, language),
    [language, t],
  );
  const labels = React.useMemo(
    () => mergedLabels(localizedLabels, props.labels),
    [localizedLabels, props.labels],
  );
  const moreLabel = t("xiadown.workspace.more");
  const selectItemsLabel = t("xiadown.completed.selectFiles");
  const selectAllLabel = t("xiadown.completed.selectAll");
  const clearSelectionLabel = t("xiadown.completed.clearSelection");
  const cancelLabel = t("xiadown.actions.cancelDialog");
  const viewItemLabel = t("xiadown.actions.view");
  const deleteItemLabel = t("xiadown.actions.deleteItem");
  const deletedItemsLabel = t("xiadown.libraryCatalog.deletedItemsAction");
  const selectionSummaryLabel = t("xiadown.completed.selectionSummary");
  const selectionUnitLabel = t("xiadown.completed.selectionUnit");
  const deleteDialogText = React.useMemo<LibraryDeleteDialogText>(() => ({
    cancel: t("xiadown.actions.cancelDialog"),
    deleteItem: t("xiadown.actions.deleteItem"),
    unknownError: t("xiadown.common.unknown"),
    deleteTaskTitle: t("xiadown.completed.deleteTaskTitle"),
    deleteTasksTitle: t("xiadown.completed.deleteTasksTitle"),
    deleteFileTitle: t("xiadown.completed.deleteFileTitle"),
    deleteFilesTitle: t("xiadown.completed.deleteFilesTitle"),
    deleteTaskMessage: t("xiadown.completed.deleteTaskMessage"),
    deleteTasksMessage: t("xiadown.completed.deleteTasksMessage"),
    deleteFileMessage: t("xiadown.completed.deleteFileMessage"),
    deleteFilesMessage: t("xiadown.completed.deleteFilesMessage"),
  }), [language, t]);
  const adapted = React.useMemo(
    () => adaptLegacyLibraryWorkspace(
      props.libraries ?? [],
      props.operations ?? [],
      props.httpBaseURL,
    ),
    [props.httpBaseURL, props.libraries, props.operations],
  );
  const suppliedItems = props.items ?? [];
  const files = props.items
    ? suppliedItems.filter((item) => item.source === "file")
    : adapted.files;
  const tasks = props.items
    ? suppliedItems.filter((item) => item.source === "task")
    : adapted.tasks;
  const [internalQuery, setInternalQuery] = React.useState(props.initialQuery ?? "");
  const query = props.query ?? internalQuery;
  const [searchExpanded, setSearchExpanded] = React.useState(
    props.query !== undefined
      ? resolveControlledLibrarySearchExpanded(props.route, props.query)
      : Boolean(props.initialQuery),
  );
  const [internalSort, setInternalSort] = React.useState<LibrarySortMode>("updated");
  const sort = props.sort ?? internalSort;
  const [view, setView] = React.useState<LibraryViewMode>(() =>
    resolveInitialLibraryViewMode(props.initialView),
  );
  const [internalOtherGroup, setInternalOtherGroup] = React.useState<LibraryOtherGroup>(
    props.initialOtherGroup ?? "document",
  );
  const otherGroup = props.otherGroup ?? internalOtherGroup;
  const [internalPage, setInternalPage] = React.useState(1);
  const [internalPageSize, setInternalPageSize] = React.useState(DEFAULT_LIBRARY_PAGE_SIZE);
  const [managementSection, setManagementSection] = React.useState<
    "summary" | "storage" | "data" | null
  >(null);
  const [selectionMode, setSelectionMode] = React.useState(false);
  const [selectedItemIds, setSelectedItemIds] = React.useState<Set<string>>(
    () => new Set(),
  );
  const [contextMenuTarget, setContextMenuTarget] =
    React.useState<LibraryContextMenuTarget | null>(null);
  const [deleteConfirmation, setDeleteConfirmation] =
    React.useState<LibraryDeleteConfirmation | null>(null);
  const searchInputRef = React.useRef<HTMLInputElement | null>(null);
  const contentRef = React.useRef<HTMLDivElement | null>(null);
  const contextReturnFocusRef = React.useRef<HTMLButtonElement | null>(null);
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const searchRoute = props.route === "search";
  const searchLanding = searchRoute && normalizedQuery.length === 0;
  const toolbarSearchExpanded =
    searchLanding || searchExpanded || query.length > 0;

  React.useEffect(() => {
    if (props.query !== undefined) {
      setSearchExpanded(resolveControlledLibrarySearchExpanded(props.route, props.query));
      return;
    }
    const nextQuery = props.initialQuery ?? "";
    setInternalQuery(nextQuery);
    setSearchExpanded(nextQuery.length > 0);
  }, [props.initialQuery, props.query, props.route]);

  React.useEffect(() => {
    if (searchExpanded) {
      searchInputRef.current?.focus();
    }
  }, [searchExpanded]);

  const filteredItems = React.useMemo(() => {
    if (props.pagination?.itemsArePage) {
      return props.items
        ? Array.from(props.items)
        : Array.from(routeItems(props.route, files, tasks));
    }
    if (props.route === "search" && !normalizedQuery) return [];
    return routeItems(props.route, files, tasks)
      .filter((item) => props.route !== "others" || item.otherGroup === otherGroup)
      .filter((item) => !normalizedQuery || item.searchText.includes(normalizedQuery))
      .slice()
      .sort(compareItems(sort));
  }, [
    files,
    normalizedQuery,
    otherGroup,
    props.items,
    props.pagination?.itemsArePage,
    props.route,
    sort,
    tasks,
  ]);

  const pageSize = props.pagination?.pageSize ?? internalPageSize;
  const total = props.pagination?.total ?? filteredItems.length;
  const page = clampLibraryPage(
    props.pagination?.page ?? internalPage,
    total,
    pageSize,
  );
  const visibleItems = React.useMemo(
    () => props.pagination?.itemsArePage
      ? filteredItems
      : sliceLibraryPage(filteredItems, page, pageSize),
    [filteredItems, page, pageSize, props.pagination?.itemsArePage],
  );
  const selectableVisibleItems = React.useMemo(
    () => visibleItems.filter(canDeleteLibraryItem),
    [visibleItems],
  );
  const selectedVisibleItems = React.useMemo(
    () => selectableVisibleItems.filter((item) => selectedItemIds.has(item.id)),
    [selectableVisibleItems, selectedItemIds],
  );
  const allVisibleItemsSelected =
    selectableVisibleItems.length > 0 &&
    selectedVisibleItems.length === selectableVisibleItems.length;
  const selectionSummary = /[\u4e00-\u9fff]/.test(
    `${selectionSummaryLabel}${selectionUnitLabel}`,
  )
    ? `${selectionSummaryLabel}${selectedVisibleItems.length}${selectionUnitLabel}`
    : `${selectionSummaryLabel} ${selectedVisibleItems.length} ${selectionUnitLabel}`.trim();

  const exitSelectionMode = React.useCallback(() => {
    setSelectionMode(false);
    setSelectedItemIds(new Set());
  }, []);

  const toggleItemSelection = React.useCallback((item: LibraryWorkspaceItem) => {
    if (!canDeleteLibraryItem(item)) return;
    setSelectedItemIds((current) => {
      const next = new Set(current);
      if (next.has(item.id)) next.delete(item.id);
      else next.add(item.id);
      return next;
    });
  }, []);

  const toggleAllVisibleItems = React.useCallback(() => {
    setSelectedItemIds(
      allVisibleItemsSelected
        ? new Set()
        : new Set(selectableVisibleItems.map((item) => item.id)),
    );
  }, [allVisibleItemsSelected, selectableVisibleItems]);

  const openItemContextMenu = React.useCallback((
    item: LibraryWorkspaceItem,
    point: LibraryContextMenuPoint,
    returnFocus: HTMLButtonElement,
  ) => {
    contextReturnFocusRef.current = returnFocus;
    setContextMenuTarget({ item, ...point });
  }, []);

  const openDeleteConfirmation = React.useCallback((
    items: readonly LibraryWorkspaceItem[],
  ) => {
    const deletableItems = items.filter(canDeleteLibraryItem);
    if (deletableItems.length === 0) return;
    setContextMenuTarget(null);
    setDeleteConfirmation({ items: [...deletableItems] });
  }, []);

  const changePage = React.useCallback((nextPage: number) => {
    const clamped = clampLibraryPage(nextPage, total, pageSize);
    if (props.pagination) props.pagination.onPageChange(clamped);
    else setInternalPage(clamped);
  }, [pageSize, props.pagination, total]);

  const resetPage = React.useCallback(() => {
    if (props.pagination) props.pagination.onPageChange(1);
    else setInternalPage(1);
  }, [props.pagination]);

  const updateSearchQuery = React.useCallback((nextQuery: string) => {
    setInternalQuery(nextQuery);
    props.onQueryChange?.(nextQuery);
    resetPage();
    setSearchExpanded(nextQuery.length > 0);
  }, [props.onQueryChange, resetPage]);

  const changePageSize = React.useCallback((nextPageSize: number) => {
    if (!Number.isFinite(nextPageSize) || nextPageSize <= 0) return;
    if (props.pagination) {
      props.pagination.onPageSizeChange(nextPageSize);
      props.pagination.onPageChange(1);
    } else {
      setInternalPageSize(nextPageSize);
      setInternalPage(1);
    }
  }, [props.pagination]);

  const paginationResetKey = `${props.route}\u0000${normalizedQuery}\u0000${sort}\u0000${otherGroup}`;
  const previousPaginationResetKey = React.useRef(paginationResetKey);
  React.useEffect(() => {
    if (previousPaginationResetKey.current === paginationResetKey) return;
    previousPaginationResetKey.current = paginationResetKey;
    if (page !== 1) resetPage();
  }, [page, paginationResetKey, resetPage]);

  const selectionScopeKey = `${paginationResetKey}\u0000${page}\u0000${pageSize}`;
  const previousSelectionScopeKey = React.useRef(selectionScopeKey);
  React.useEffect(() => {
    if (previousSelectionScopeKey.current === selectionScopeKey) return;
    previousSelectionScopeKey.current = selectionScopeKey;
    exitSelectionMode();
    setContextMenuTarget(null);
    setDeleteConfirmation(null);
  }, [exitSelectionMode, selectionScopeKey]);

  React.useEffect(() => {
    const selectableIds = new Set(selectableVisibleItems.map((item) => item.id));
    setSelectedItemIds((current) => {
      const next = new Set([...current].filter((id) => selectableIds.has(id)));
      return next.size === current.size ? current : next;
    });
  }, [selectableVisibleItems]);

  React.useEffect(() => {
    const requestedPage = props.pagination?.page ?? internalPage;
    const clampedPage = clampLibraryPage(requestedPage, total, pageSize);
    if (requestedPage !== clampedPage) changePage(clampedPage);
  }, [changePage, internalPage, pageSize, props.pagination?.page, total]);

  React.useEffect(() => {
    contentRef.current?.scrollTo({ top: 0 });
  }, [page, pageSize]);

  const computedOtherCounts = React.useMemo(() => {
    const counts = new Map<LibraryOtherGroup, number>();
    files.forEach((item) => {
      if (item.category === "other" && item.otherGroup) {
        counts.set(item.otherGroup, (counts.get(item.otherGroup) ?? 0) + 1);
      }
    });
    return counts;
  }, [files]);
  const otherCounts = React.useMemo(() => {
    const counts = new Map(computedOtherCounts);
    Object.entries(props.otherCounts ?? {}).forEach(([group, count]) => {
      if (typeof count === "number") counts.set(group as LibraryOtherGroup, count);
    });
    return counts;
  }, [computedOtherCounts, props.otherCounts]);

  const searchControl = toolbarSearchExpanded ? (
    <div className="app-library-toolbar__search-region wails-no-drag">
      <label className="app-library-search">
        <Search size={16} aria-hidden="true" />
        <input
          autoFocus={searchRoute}
          ref={searchInputRef}
          type="search"
          value={query}
          onChange={(event) => {
            updateSearchQuery(event.currentTarget.value);
          }}
          onBlur={() => {
            if (props.route !== "search" && query.length === 0) {
              setSearchExpanded(false);
            }
          }}
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              updateSearchQuery("");
            }
          }}
          placeholder={labels.searchPlaceholder}
          aria-label={labels.searchPlaceholder}
        />
      </label>
    </div>
  ) : (
    <WorkspacePrimaryHeaderAction
      label={labels.search}
      className="app-library-toolbar-button app-library-search-toggle wails-no-drag"
      onClick={() => setSearchExpanded(true)}
    >
      <Search size={16} aria-hidden="true" />
    </WorkspacePrimaryHeaderAction>
  );

  const toolbarActions = (
    <>
      <WorkspacePrimaryHeaderActionGroup
        className="app-library-selection-actions"
        label={selectItemsLabel}
      >
        {!selectionMode ? (
          <WorkspacePrimaryHeaderAction
            className="app-library-selection-toggle"
            label={selectItemsLabel}
            disabled={selectableVisibleItems.length === 0}
            onClick={() => {
              setContextMenuTarget(null);
              setSelectedItemIds(new Set());
              setSelectionMode(true);
            }}
          >
            <CheckCircle2 size={16} aria-hidden="true" />
          </WorkspacePrimaryHeaderAction>
        ) : (
          <>
            <span
              className="app-library-selection-summary"
              role="status"
              aria-live="polite"
            >
              {selectionSummary}
            </span>
            <WorkspacePrimaryHeaderAction
              className="app-library-selection-delete"
              label={deleteItemLabel}
              disabled={selectedVisibleItems.length === 0}
              onClick={() => openDeleteConfirmation(selectedVisibleItems)}
            >
              <Trash2 size={16} aria-hidden="true" />
            </WorkspacePrimaryHeaderAction>
            <WorkspacePrimaryHeaderAction
              className="app-library-selection-all"
              label={allVisibleItemsSelected ? clearSelectionLabel : selectAllLabel}
              disabled={selectableVisibleItems.length === 0}
              onClick={toggleAllVisibleItems}
            >
              {allVisibleItemsSelected ? (
                <CircleSlash size={16} aria-hidden="true" />
              ) : (
                <CheckSquare size={16} aria-hidden="true" />
              )}
            </WorkspacePrimaryHeaderAction>
            <WorkspacePrimaryHeaderAction
              className="app-library-selection-cancel"
              label={cancelLabel}
              onClick={exitSelectionMode}
            >
              <X size={16} aria-hidden="true" />
            </WorkspacePrimaryHeaderAction>
          </>
        )}
      </WorkspacePrimaryHeaderActionGroup>
      <WorkspacePrimaryHeaderActionGroup
        className="app-library-view-switch app-library-toolbar-control"
        label={t("xiadown.actions.view")}
      >
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <WorkspacePrimaryHeaderAction
              className="app-library-sort"
              label={labels.sortLabel}
            >
              {sort === "name" ? (
                <ArrowDownAZ size={16} aria-hidden="true" />
              ) : (
                <CalendarDays size={16} aria-hidden="true" />
              )}
            </WorkspacePrimaryHeaderAction>
          </DropdownMenuTrigger>
          <WorkspacePrimaryHeaderMenuContent className="app-library-sort-menu">
            <DropdownMenuRadioGroup
              value={sort}
              onValueChange={(value) => {
                const nextSort = value as LibrarySortMode;
                setInternalSort(nextSort);
                props.onSortChange?.(nextSort);
                resetPage();
              }}
            >
              <DropdownMenuRadioItem
                className="app-library-sort-menu__item"
                value="updated"
              >
                {labels.sortNewest}
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem
                className="app-library-sort-menu__item"
                value="oldest"
              >
                {labels.sortOldest}
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem
                className="app-library-sort-menu__item"
                value="name"
              >
                {labels.sortName}
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem
                className="app-library-sort-menu__item"
                value="size"
              >
                {labels.sortSize}
              </DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </WorkspacePrimaryHeaderMenuContent>
        </DropdownMenu>
        <WorkspacePrimaryHeaderAction
          label={labels.gridView}
          className="app-library-view-action"
          aria-pressed={view === "grid"}
          onClick={() => {
            setView("grid");
            writeLibraryViewMode("grid");
          }}
        >
          <Grid2X2 size={16} aria-hidden="true" />
        </WorkspacePrimaryHeaderAction>
        <WorkspacePrimaryHeaderAction
          label={labels.listView}
          className="app-library-view-action"
          aria-pressed={view === "list"}
          onClick={() => {
            setView("list");
            writeLibraryViewMode("list");
          }}
        >
          <List size={17} aria-hidden="true" />
        </WorkspacePrimaryHeaderAction>
      </WorkspacePrimaryHeaderActionGroup>
      <WorkspacePrimaryHeaderActionGroup label={moreLabel}>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <WorkspacePrimaryHeaderAction
              className="app-library-toolbar-button app-library-actions-more"
              label={moreLabel}
            >
              <Ellipsis size={16} aria-hidden="true" />
            </WorkspacePrimaryHeaderAction>
          </DropdownMenuTrigger>
          <WorkspacePrimaryHeaderMenuContent className="app-menu-content-fit app-library-actions-menu">
            <DropdownMenuItem onSelect={() => setManagementSection("summary")}>
              <ChartPie aria-hidden="true" />
              {labels.summary}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setManagementSection("storage")}>
              <FolderCog aria-hidden="true" />
              {labels.storageRoots}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setManagementSection("data")}>
              <DatabaseBackup aria-hidden="true" />
              {labels.dataManagement}
            </DropdownMenuItem>
          </WorkspacePrimaryHeaderMenuContent>
        </DropdownMenu>
      </WorkspacePrimaryHeaderActionGroup>
    </>
  );

  const toolbar = (
    <WorkspacePageTopBar
      className={cn(
        "app-library-page__header app-library-toolbar",
        searchRoute && "app-library-toolbar--search app-station-search-header",
      )}
      actionsLabel={routeTitle(props.route, labels)}
      data-search-expanded={toolbarSearchExpanded ? "true" : "false"}
      reserveWindowControls={props.reserveWindowControls}
    >
      {!searchRoute ? (
        <WorkspacePrimaryHeaderActionGroup label={deletedItemsLabel}>
          <WorkspacePrimaryHeaderAction
            className="app-library-deleted-items-action wails-no-drag"
            label={deletedItemsLabel}
            disabled={!props.onOpenDeletedItems}
            onClick={props.onOpenDeletedItems}
          >
            <Trash2 size={16} aria-hidden="true" />
          </WorkspacePrimaryHeaderAction>
        </WorkspacePrimaryHeaderActionGroup>
      ) : null}
      {!searchRoute ? (
        <>
          {toolbarActions}
          <WorkspacePrimaryHeaderActionGroup label={labels.search}>
            {searchControl}
          </WorkspacePrimaryHeaderActionGroup>
        </>
      ) : null}
    </WorkspacePageTopBar>
  );

  const primaryContentBody = (
    <>
      {searchRoute ? (
        <WorkspaceSearchControl
          clearLabel={t("xiadown.actions.clear")}
          inputRef={searchInputRef}
          onSubmit={() => setSearchExpanded(Boolean(query.trim()))}
          onValueChange={updateSearchQuery}
          placeholder={labels.searchPlaceholder}
          submitLabel={labels.search}
          value={query}
        />
      ) : null}
      {searchRoute && !searchLanding ? (
        <div
          className="app-library-search-results-toolbar wails-no-drag"
          role="group"
          aria-label={routeTitle(props.route, labels)}
        >
          {toolbarActions}
        </div>
      ) : null}
      {searchLanding ? null : props.loading ? (
        <div className="app-library-load-state" role="status">
          <LoaderCircle size={24} className="app-motion-spin" />
          <p>{labels.loading}</p>
        </div>
      ) : props.loadError ? (
        <div className="app-library-load-state app-library-load-state--error" role="alert">
          <AlertTriangle size={24} />
          <p>{labels.loadFailed}</p>
          <button type="button" onClick={props.onRetry}>{labels.retry}</button>
        </div>
      ) : visibleItems.length > 0 ? (
        <div className="app-library-items" data-view={view}>
          {visibleItems.map((item) => (
            <LibraryItemCard
              key={item.id}
              item={item}
              selected={!selectionMode && props.selectedItemId === item.id}
              selectionMode={selectionMode}
              batchSelected={selectedItemIds.has(item.id)}
              view={view}
              labels={labels}
              onClick={props.onItemClick}
              onToggleSelection={toggleItemSelection}
              onOpenContextMenu={openItemContextMenu}
            />
          ))}
        </div>
      ) : (
        <div className="app-library-empty">
          <span><Search size={24} /></span>
          <h2>{labels.emptyTitle}</h2>
          <p>{labels.emptyDescription}</p>
        </div>
      )}
    </>
  );

  const showPaginationFooter = !searchLanding && total > pageSize;
  const pageContract = defineWorkspacePageContract({
    presentation: "primary",
    recipe: searchRoute ? "search" : "collection",
    routeLabel: routeTitle(props.route, labels),
    topBar: searchRoute ? "search" : "actions",
    heading: "assistive",
    contentLayout:
      props.route === "others" ? "split" : view === "grid" ? "card-grid" : "list",
    footer: showPaginationFooter ? "pagination" : "none",
    scroll: props.route === "others" ? "panes" : "content",
    density: "regular",
    immersion: "standard",
  });

  const paginationFooter = showPaginationFooter ? (
    <WorkspacePageFooter
      className="app-library-page__footer app-library-pagination"
      aria-label={labels.itemCount(total)}
      aria-live="polite"
    >
      <LibraryPaginationFooter
        embedded
        page={page}
        pageSize={pageSize}
        total={total}
        labels={labels}
        disabled={props.loading}
        onPageChange={changePage}
        onPageSizeChange={changePageSize}
      />
    </WorkspacePageFooter>
  ) : null;

  return (
    <WorkspacePage
      className="app-library-page"
      contract={pageContract}
      data-library-route={props.route}
      data-library-view={view}
      data-search-state={searchLanding ? "landing" : "results"}
      data-selection-mode={selectionMode ? "true" : "false"}
    >
      {toolbar}
      {props.route === "others" && !searchLanding ? (
        <WorkspacePageContent className="app-library-content app-library-content--others">
          <aside className="app-library-other-pane app-main-list-pane app-main-sidebar app-workspace-primary-subpane app-workspace-primary-subpane--leading">
            <nav className="app-library-other-groups" aria-label={labels.others}>
              {OTHER_GROUPS.map((group) => (
                <button
                  key={group}
                  type="button"
                  data-active={otherGroup === group ? "true" : "false"}
                  aria-current={otherGroup === group ? "page" : undefined}
                  onClick={() => {
                    setInternalOtherGroup(group);
                    props.onOtherGroupChange?.(group);
                    resetPage();
                  }}
                >
                  <span className="app-library-other-groups__label">
                    <LibraryOtherGroupIcon group={group} />
                    <span>{labels.otherGroups[group]}</span>
                  </span>
                  <span>{otherCounts.get(group) ?? 0}</span>
                </button>
              ))}
            </nav>
          </aside>
          <div className="app-library-primary-surface app-library-primary-surface--other app-main-detail-pane app-workspace-primary-subpane">
            <div
              ref={contentRef}
              className="app-library-page__content app-library-primary app-library-other-detail"
              aria-label={labels.otherGroups[otherGroup]}
            >
              {primaryContentBody}
            </div>
          </div>
        </WorkspacePageContent>
      ) : (
        <WorkspacePageContent
          ref={contentRef}
          className="app-library-page__content app-library-primary"
        >
          {primaryContentBody}
        </WorkspacePageContent>
      )}
      {paginationFooter}
      {managementSection ? (
        <CatalogManagementDialog
          open
          initialSection={managementSection}
          labels={labels}
          onOpenChange={(open) => {
            if (!open) setManagementSection(null);
          }}
        />
      ) : null}
      <DropdownMenu
        modal={false}
        open={Boolean(contextMenuTarget)}
        onOpenChange={(open) => {
          if (!open) setContextMenuTarget(null);
        }}
      >
        {contextMenuTarget ? (
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="app-library-context-menu-anchor"
              style={{
                left: contextMenuTarget.x,
                top: contextMenuTarget.y,
              }}
              tabIndex={-1}
              aria-hidden="true"
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          className={cn(
            SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,
            "app-library-context-menu",
          )}
          align="start"
          side="bottom"
          sideOffset={2}
          aria-label={contextMenuTarget?.item.title}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            const returnFocus = contextReturnFocusRef.current;
            if (returnFocus?.isConnected) returnFocus.focus();
          }}
        >
          <DropdownMenuItem
            className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
            disabled={!contextMenuTarget || !props.onItemClick}
            onSelect={() => {
              const item = contextMenuTarget?.item;
              if (!item) return;
              setContextMenuTarget(null);
              exitSelectionMode();
              props.onItemClick?.(item);
            }}
          >
            <span className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
              <Eye aria-hidden="true" />
            </span>
            <span>{viewItemLabel}</span>
          </DropdownMenuItem>
          <DropdownMenuItem
            className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
            tone="destructive"
            disabled={
              !contextMenuTarget ||
              !canDeleteLibraryItem(contextMenuTarget.item)
            }
            onSelect={() => {
              const item = contextMenuTarget?.item;
              if (item) openDeleteConfirmation([item]);
            }}
          >
            <span className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
              <Trash2 aria-hidden="true" />
            </span>
            <span>{deleteItemLabel}</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {deleteConfirmation ? (
        <LibraryDeleteConfirmationDialog
          target={deleteConfirmation}
          text={deleteDialogText}
          onCancel={() => setDeleteConfirmation(null)}
          onExecuted={({ deletedItemIds, failedItems }) => {
            const deletedIds = new Set(deletedItemIds);
            setSelectedItemIds((current) => new Set(
              [...current].filter((itemId) => !deletedIds.has(itemId)),
            ));
            if (failedItems.length > 0) {
              setDeleteConfirmation({ items: [...failedItems] });
              return;
            }
            setDeleteConfirmation(null);
            setSelectionMode(false);
          }}
        />
      ) : null}
    </WorkspacePage>
  );
}
