import {
  Activity,
  AlertTriangle,
  BookOpen,
  Check,
  Clock3,
  Copy,
  File,
  FileStack,
  FolderOpen,
  Image as ImageIcon,
  Info,
  ListTodo,
  LoaderCircle,
  Music2,
  PencilLine,
  RotateCcw,
  ScanEye,
  Trash2,
  Video,
  X,
  type LucideIcon,
} from "lucide-react";
import * as React from "react";

import type {
  CatalogItemAsset,
  CatalogItemCategory,
  CatalogItemDetail,
  CatalogRepresentation,
} from "@/shared/contracts/catalog";
import type { FileEventRecordDTO, LibraryDTO } from "@/shared/contracts/library";
import { useI18n } from "@/shared/i18n";
import { messageBus } from "@/shared/message/store";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { StatusBadge, type DreamStatusTone } from "@/shared/ui/status-badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import {
  LIBRARY_CATALOG_ACTOR_ID,
  useCatalogItem,
  useCatalogItemActivity,
  useRestoreCatalogItem,
  useTrashCatalogItem,
  useUpdateCatalogItem,
} from "@/shared/query/catalog";
import {
  useDeleteFiles,
  useDeleteOperationOutput,
  useDeleteOperation,
  useListLibraries,
  useOpenLibraryFileLocation,
  useOpenLibraryPath,
  useRenameFile,
  useRenameOperation,
} from "@/shared/query/library";
import { formatBytes } from "@/shared/utils/formatBytes";
import {
  composeProtectedFileDisplayName,
  splitProtectedFileDisplayName,
  validateRenameDisplayName,
} from "@/shared/utils/renameDisplayName";
import {
  buildAssetPreviewURL,
  extractExtensionFromPath,
} from "@/shared/utils/resourceHelpers";

import {
  createLibraryWorkspaceLabels,
  libraryItemAvailability,
  libraryItemDisplayStatus,
  shouldShowLibraryStatusBadge,
  type LibraryWorkspaceItem,
  type LibraryWorkspaceLabels,
} from "./types";
import {
  buildCatalogCardPreviewURL,
  isBrowserImagePreviewPath,
} from "./catalog-adapter";
import {
  isLibraryDefaultArtworkURL,
  LibraryArtwork,
} from "./LibraryArtwork";
import { TaskFolderArtwork } from "./TaskFolderArtwork";
import { LibraryIpodPreview } from "./LibraryIpodPreview";
import { LibraryLogPreview } from "./LibraryLogPreview";
import { LibraryPdfPreview } from "./LibraryPdfPreview";
import {
  fileEventsForLibraryItem,
  operationHistoryEventsForTask,
  operationRenameTransitionsForTask,
  projectTaskOutputVersions,
  type TaskOutputVersionState,
} from "./library-preview-history";
import "./library.css";

const TABS = ["preview", "info", "versions", "activity"] as const;
export type LibraryPreviewTab = (typeof TABS)[number];

const TAB_ICONS = {
  preview: ScanEye,
  info: Info,
  versions: FileStack,
  activity: Activity,
} satisfies Record<LibraryPreviewTab, LucideIcon>;

function normalizePreviewTab(value: unknown): LibraryPreviewTab {
  return TABS.includes(value as LibraryPreviewTab) ? value as LibraryPreviewTab : "preview";
}

const PREVIEW_CATEGORY_ICONS = {
  task: ListTodo,
  video: Video,
  audio: Music2,
  book: BookOpen,
  image: ImageIcon,
  other: File,
} satisfies Record<LibraryWorkspaceItem["category"], LucideIcon>;

interface PreviewFact {
  label: string;
  value: React.ReactNode;
  title?: string;
}

export interface LibraryPreviewCompanionProps {
  item: LibraryWorkspaceItem | null;
  httpBaseURL?: string;
  labels?: Partial<LibraryWorkspaceLabels>;
  initialTab?: LibraryPreviewTab;
  activeTab?: LibraryPreviewTab;
  onActiveTabChange?: (tab: LibraryPreviewTab) => void;
  onOpenItem?: (itemId: string) => void;
  tabsPlacement?: "footer" | "external";
}

function mergeLabels(base: LibraryWorkspaceLabels, overrides?: Partial<LibraryWorkspaceLabels>) {
  return {
    ...base,
    ...overrides,
    otherGroups: { ...base.otherGroups, ...overrides?.otherGroups },
  };
}

function formatDuration(durationMs?: number) {
  if (!durationMs || durationMs < 0) return "–";
  const seconds = Math.floor(durationMs / 1000);
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remaining = seconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remaining).padStart(2, "0")}`
    : `${minutes}:${String(remaining).padStart(2, "0")}`;
}

function errorMessage(error: unknown, labels: LibraryWorkspaceLabels) {
  const message = error instanceof Error ? error.message : String(error ?? "");
  return /revision|conflict|stale/i.test(message) ? labels.revisionConflict : message || labels.loadFailed;
}

function itemCover(detail: CatalogItemDetail, fallback: string, httpBaseURL: string) {
  const artworkAssetIds = new Set(
    detail.representations
      .filter((representation) =>
        representation.availability === "available" &&
        /artwork|thumbnail/i.test(`${representation.kind} ${representation.purpose}`),
      )
      .map((representation) => representation.assetId),
  );
  const preferred = detail.assets.find((asset) =>
    asset.fileAvailable &&
    isCatalogImagePreviewAsset(asset) &&
    (
      /artwork|thumbnail|cover/i.test(`${asset.role} ${asset.label ?? ""} ${asset.file?.kind ?? ""}`) ||
      artworkAssetIds.has(asset.id)
    ),
  ) ?? (detail.item.category === "image"
    ? detail.assets.find((asset) =>
        asset.fileAvailable &&
        /original|primary/i.test(asset.role) &&
        isCatalogImagePreviewAsset(asset),
      )
    : undefined);
  const path = preferred?.file?.storage.localPath ?? "";
  return buildAssetPreviewURL(httpBaseURL, path, preferred?.updatedAt) || fallback;
}

function isCatalogImagePreviewAsset(asset: CatalogItemAsset) {
  const file = asset.file;
  const path = file?.storage.localPath?.trim() ?? "";
  const state = file?.state.status.trim().toLocaleLowerCase() ?? "";
  return Boolean(
    file &&
    !file.state.deleted &&
    !file.state.lastError?.trim() &&
    !["deleted", "missing", "offline", "error", "unavailable"].includes(state) &&
    isBrowserImagePreviewPath(path),
  );
}

function primaryAsset(detail: CatalogItemDetail) {
  const original = detail.assets.find((asset) => /original|primary/i.test(asset.role));
  if (original?.fileAvailable) return original;

  const playbackAssetIds = new Set(
    detail.representations
      .filter((representation) =>
        representation.availability === "available" &&
        /primary|playback/i.test(representation.purpose),
      )
      .map((representation) => representation.assetId),
  );
  return detail.assets.find((asset) =>
    asset.fileAvailable && playbackAssetIds.has(asset.id),
  ) ?? detail.assets.find((asset) =>
    asset.fileAvailable && /representation/i.test(asset.role),
  ) ?? original ?? detail.assets[0];
}

function previewAsset(detail: CatalogItemDetail) {
  const available = detail.assets.filter((asset) =>
    asset.fileAvailable && Boolean(asset.file?.storage.localPath?.trim()),
  );
  const preferredRepresentation = detail.representations.find((representation) =>
    representation.availability === "available" &&
    /preview|playback/i.test(representation.purpose) &&
    representationCanPreviewCategory(representation, detail.item.category) &&
    available.some((asset) => asset.id === representation.assetId),
  );
  if (preferredRepresentation) {
    return available.find((asset) => asset.id === preferredRepresentation.assetId);
  }
  return available.find((asset) => /original|primary/i.test(asset.role)) ?? available[0];
}

export function resolveCatalogPreviewMedia(
  detail: CatalogItemDetail,
  fallbackCoverURL: string,
  httpBaseURL: string,
) {
  const coverURL = itemCover(detail, fallbackCoverURL, httpBaseURL);
  const sourceAsset = previewAsset(detail);
  const sourcePath = sourceAsset?.file?.storage.localPath?.trim() ?? "";
  const sourceURL = buildAssetPreviewURL(
    httpBaseURL,
    sourcePath,
    sourceAsset?.updatedAt,
  );
  const sourceFormat =
    sourceAsset?.file?.media?.format || extractExtensionFromPath(sourcePath);
  const normalizedFormat = sourceFormat.trim().toLocaleLowerCase();
  const logPreviewURL = (
      normalizedFormat === "log" ||
      normalizedFormat === "text/x-log" ||
      extractExtensionFromPath(sourcePath) === "log"
    )
    ? buildCatalogCardPreviewURL(
        httpBaseURL,
        "log",
        detail.item.id,
        sourceAsset?.updatedAt ?? detail.item.updatedAt,
      )
    : "";
  return {
    coverURL,
    posterURL: isLibraryDefaultArtworkURL(coverURL) ? undefined : coverURL,
    sourceAsset,
    logPreviewURL,
    sourceFormat,
    sourcePath,
    sourceURL,
  };
}

function representationCanPreviewCategory(
  representation: CatalogRepresentation,
  category: CatalogItemCategory,
) {
  const kind = representation.kind.trim().toLocaleLowerCase();
  if (!["original", "optimized", "preview"].includes(kind)) return false;
  const mediaType = representation.mediaType?.trim().toLocaleLowerCase() ?? "";
  if (!mediaType) return true;
  switch (category) {
    case "video": return mediaType.startsWith("video/");
    case "audio": return mediaType.startsWith("audio/");
    case "image": return mediaType.startsWith("image/");
    case "book": return mediaType === "application/pdf";
    default: return false;
  }
}

function previewCategoryLabel(
  category: LibraryWorkspaceItem["category"],
  labels: LibraryWorkspaceLabels,
) {
  switch (category) {
    case "task": return labels.tasks;
    case "book": return labels.books;
    case "image": return labels.images;
    case "other": return labels.others;
    default: return labels[category];
  }
}

function previewStatusTone(status: string): DreamStatusTone {
  const normalized = status.trim().toLocaleLowerCase();
  if (/error|failed|corrupt|missing|offline|unavailable/.test(normalized)) return "danger";
  if (/review/.test(normalized)) return "warning";
  if (/running|processing|loading|queued|pending/.test(normalized)) return "busy";
  if (/active|available|complete|completed|ready|success/.test(normalized)) return "success";
  if (/paused|stopped|trashed|archived|cancel/.test(normalized)) return "muted";
  return "neutral";
}

function PreviewArtworkImage(props: {
  src: string;
  fallbackSrc?: string;
  alt: string;
  category: LibraryWorkspaceItem["category"];
  otherGroup?: LibraryWorkspaceItem["otherGroup"];
  className?: string;
}) {
  return (
    <LibraryArtwork
      src={props.src}
      fallbackSrc={props.fallbackSrc}
      category={props.category}
      otherGroup={props.otherGroup}
      alt={props.alt}
      className={props.className}
    />
  );
}

function PreviewIdentity(props: {
  title: string;
  titleContent?: React.ReactNode;
  description?: string;
  subtitle?: string;
  category: LibraryWorkspaceItem["category"];
  status: string;
  format?: string;
  labels: LibraryWorkspaceLabels;
  compact?: boolean;
}) {
  const CategoryIcon = PREVIEW_CATEGORY_ICONS[props.category];
  return (
    <header
      className="app-library-preview__identity"
      data-compact={props.compact ? "true" : undefined}
    >
      <div className="app-library-preview__eyebrow">
        <span className="app-library-preview__kind">
          <CategoryIcon size={13} aria-hidden="true" />
          {previewCategoryLabel(props.category, props.labels)}
        </span>
        {shouldShowLibraryStatusBadge(props.status) ? (
          <StatusBadge
            className="app-library-preview__status"
            marker
            tone={previewStatusTone(props.status)}
          >
            {props.labels.statusLabel(props.status)}
          </StatusBadge>
        ) : null}
      </div>
      {props.titleContent ?? <h2 title={props.title}>{props.title}</h2>}
      {props.description?.trim() ? <p>{props.description.trim()}</p> : null}
      {props.subtitle?.trim() || props.format?.trim() ? (
        <small title={props.subtitle}>
          {[props.subtitle?.trim(), props.format?.trim()].filter(Boolean).join(" · ")}
        </small>
      ) : null}
    </header>
  );
}

function previewTitleMotion(overflow: number) {
  if (!Number.isFinite(overflow) || overflow <= 1) {
    return { scrolling: false, shift: "0px", duration: "6s" };
  }
  const distance = Math.ceil(overflow);
  return {
    scrolling: true,
    shift: `-${distance}px`,
    duration: `${Math.min(13, Math.max(6, (distance + 140) / 26))}s`,
  };
}

function PreviewBouncingTitle(props: { title: string }) {
  const viewportRef = React.useRef<HTMLHeadingElement | null>(null);
  const contentRef = React.useRef<HTMLSpanElement | null>(null);
  const title = props.title.trim();
  const [measurement, setMeasurement] = React.useState({ title, overflow: 0 });
  const overflow = measurement.title === title ? measurement.overflow : 0;
  const motion = previewTitleMotion(overflow);

  React.useEffect(() => {
    const viewport = viewportRef.current;
    const content = contentRef.current;
    if (!viewport || !content) return;

    const measure = () => {
      const nextOverflow = Math.max(0, content.scrollWidth - viewport.clientWidth);
      setMeasurement((current) =>
        current.title === title && current.overflow === nextOverflow
          ? current
          : { title, overflow: nextOverflow },
      );
    };
    measure();

    if (typeof ResizeObserver === "undefined") {
      if (typeof window === "undefined") return;
      window.addEventListener("resize", measure);
      return () => window.removeEventListener("resize", measure);
    }

    const observer = new ResizeObserver(measure);
    observer.observe(viewport);
    observer.observe(content);
    return () => observer.disconnect();
  }, [title]);

  return (
    <h2
      ref={viewportRef}
      className="app-library-preview__title-marquee"
      data-overflow={motion.scrolling ? "true" : "false"}
      title={title}
      aria-label={title}
    >
      <span
        ref={contentRef}
        aria-hidden="true"
        style={
          motion.scrolling
            ? ({
                "--app-library-preview-title-shift": motion.shift,
                "--app-library-preview-title-duration": motion.duration,
              } as React.CSSProperties)
            : undefined
        }
      >
        {title}
      </span>
    </h2>
  );
}

type InlineRenamePlacement = "hero" | "row" | "context";

function InlineRenameField(props: {
  value: string;
  labels: LibraryWorkspaceLabels;
  editLabel: string;
  fieldLabel: string;
  placeholder: string;
  placement: InlineRenamePlacement;
  protectExtension?: boolean;
  renderValue: (value: string) => React.ReactNode;
  onSave: (value: string) => Promise<string | void>;
}) {
  const inputId = React.useId();
  const errorId = `${inputId}-error`;
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const triggerRef = React.useRef<HTMLButtonElement | null>(null);
  const restoreTriggerFocusRef = React.useRef(false);
  const [displayValue, setDisplayValue] = React.useState(props.value.trim());
  const [draft, setDraft] = React.useState("");
  const [editing, setEditing] = React.useState(false);
  const [pending, setPending] = React.useState(false);
  const [error, setError] = React.useState("");
  const protectedName = props.protectExtension
    ? splitProtectedFileDisplayName(displayValue)
    : { stem: displayValue, extension: "" };

  React.useEffect(() => {
    setDisplayValue(props.value.trim());
  }, [props.value]);

  React.useEffect(() => {
    if (editing) {
      inputRef.current?.focus();
      inputRef.current?.select();
      return;
    }
    if (restoreTriggerFocusRef.current) {
      restoreTriggerFocusRef.current = false;
      triggerRef.current?.focus();
    }
  }, [editing]);

  const beginEditing = () => {
    setDraft(protectedName.stem);
    setError("");
    setEditing(true);
  };

  const cancelEditing = () => {
    if (pending) return;
    restoreTriggerFocusRef.current = true;
    setDraft("");
    setError("");
    setEditing(false);
  };

  const executeRename = async () => {
    if (pending) return;
    const candidate = props.protectExtension
      ? composeProtectedFileDisplayName(draft, protectedName.extension)
      : draft.trim();
    const editableCandidate = protectedName.extension
      ? candidate.slice(0, -protectedName.extension.length)
      : candidate;
    const validationMessages = {
      required: props.labels.renameNameRequired,
      invalid: props.labels.renameNameInvalid,
      tooLong: props.labels.renameNameTooLong,
    };
    const validationError = validateRenameDisplayName(
      editableCandidate,
      validationMessages,
    ) || validateRenameDisplayName(candidate, validationMessages);
    if (validationError) {
      setError(validationError);
      return;
    }
    if (candidate === displayValue.trim()) {
      cancelEditing();
      return;
    }

    setPending(true);
    setError("");
    try {
      const savedValue = (await props.onSave(candidate))?.trim() || candidate;
      restoreTriggerFocusRef.current = true;
      setDisplayValue(savedValue);
      setDraft("");
      setEditing(false);
    } catch (renameError) {
      setError(errorMessage(renameError, props.labels));
    } finally {
      setPending(false);
    }
  };

  return (
    <div
      className="app-library-preview__inline-rename"
      data-placement={props.placement}
      data-state={editing ? "editing" : "display"}
    >
      {editing ? (
        <>
          <div className="app-library-preview__inline-rename-editor" aria-busy={pending}>
            <label className="sr-only" htmlFor={inputId}>{props.fieldLabel}</label>
            <Input
              id={inputId}
              ref={inputRef}
              aria-describedby={error ? errorId : undefined}
              aria-invalid={Boolean(error)}
              disabled={pending}
              placeholder={props.placeholder}
              value={draft}
              onChange={(event) => {
                setDraft(event.target.value);
                if (error) setError("");
              }}
              onKeyDown={(event) => {
                if (event.nativeEvent.isComposing) return;
                if (event.key === "Enter") {
                  event.preventDefault();
                  void executeRename();
                } else if (event.key === "Escape") {
                  event.preventDefault();
                  cancelEditing();
                }
              }}
            />
            {protectedName.extension ? (
              <StatusBadge
                aria-label={`${props.labels.format}: ${protectedName.extension} · ${props.labels.locked}`}
                title={`${props.labels.format}: ${protectedName.extension} · ${props.labels.locked}`}
                tone="muted"
              >
                {protectedName.extension}
              </StatusBadge>
            ) : null}
            <div className="app-library-preview__inline-rename-actions">
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    aria-label={props.labels.saveChanges}
                    disabled={pending}
                    onClick={() => void executeRename()}
                    size="compactIcon"
                    type="button"
                  >
                    {pending ? <LoaderCircle aria-hidden="true" className="app-motion-spin" /> : <Check aria-hidden="true" />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{props.labels.saveChanges}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    aria-label={props.labels.cancelDialog}
                    disabled={pending}
                    onClick={cancelEditing}
                    size="compactIcon"
                    type="button"
                    variant="ghost"
                  >
                    <X aria-hidden="true" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{props.labels.cancelDialog}</TooltipContent>
              </Tooltip>
            </div>
          </div>
          {error ? (
            <div
              className="app-dream-status-message app-library-preview__inline-rename-error"
              data-intent="danger"
              id={errorId}
              role="alert"
            >
              {error}
            </div>
          ) : null}
        </>
      ) : (
        <div className="app-library-preview__inline-rename-display">
          {props.placement === "hero" ? <span aria-hidden="true" /> : null}
          <div className="app-library-preview__inline-rename-value">
            {props.renderValue(displayValue)}
          </div>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                aria-label={`${props.editLabel}: ${displayValue}`}
                onClick={beginEditing}
                ref={triggerRef}
                size="compactIcon"
                type="button"
                variant="ghost"
              >
                <PencilLine aria-hidden="true" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{props.editLabel}</TooltipContent>
          </Tooltip>
        </div>
      )}
    </div>
  );
}

function TaskRenameTitle(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  placement: "hero" | "context";
}) {
  const renameOperation = useRenameOperation();
  const operationId = props.item.operation?.operationId ?? "";
  return (
    <InlineRenameField
      key={`task:${operationId}`}
      value={props.item.title}
      labels={props.labels}
      editLabel={props.labels.renameTask}
      fieldLabel={props.labels.renameTaskNameLabel}
      placeholder={props.labels.renameTaskNamePlaceholder}
      placement={props.placement}
      renderValue={(title) => props.placement === "hero"
        ? <PreviewBouncingTitle title={title} />
        : <h2 title={title}>{title}</h2>}
      onSave={async (name) => {
        const operation = await renameOperation.mutateAsync({ operationId, name });
        return operation.displayName;
      }}
    />
  );
}

function FileRenameTitle(props: {
  fileId: string;
  title: string;
  labels: LibraryWorkspaceLabels;
  placement: InlineRenamePlacement;
}) {
  const renameFile = useRenameFile();
  return (
    <InlineRenameField
      key={`file:${props.fileId}`}
      value={props.title}
      labels={props.labels}
      editLabel={props.labels.renameFile}
      fieldLabel={props.labels.renameFileNameLabel}
      placeholder={props.labels.renameFileNamePlaceholder}
      placement={props.placement}
      protectExtension
      renderValue={(title) => props.placement === "hero"
        ? <PreviewBouncingTitle title={title} />
        : props.placement === "context"
          ? <h2 title={title}>{title}</h2>
          : <strong title={title}>{title}</strong>}
      onSave={async (name) => {
        const file = await renameFile.mutateAsync({ fileId: props.fileId, name });
        return file.displayName;
      }}
    />
  );
}

function CatalogRenameTitle(props: {
  detail: CatalogItemDetail;
  labels: LibraryWorkspaceLabels;
  placement: "hero" | "context";
}) {
  const updateCatalogItem = useUpdateCatalogItem();
  return (
    <InlineRenameField
      key={`catalog:${props.detail.item.id}`}
      value={props.detail.item.title}
      labels={props.labels}
      editLabel={props.labels.renameFile}
      fieldLabel={props.labels.renameFileNameLabel}
      placeholder={props.labels.renameFileNamePlaceholder}
      placement={props.placement}
      renderValue={(title) => props.placement === "hero"
        ? <PreviewBouncingTitle title={title} />
        : <h2 title={title}>{title}</h2>}
      onSave={async (title) => {
        const detail = await updateCatalogItem.mutateAsync({
          id: props.detail.item.id,
          expectedRevision: props.detail.item.revision,
          title,
          actorId: LIBRARY_CATALOG_ACTOR_ID,
        });
        return detail.item.title;
      }}
    />
  );
}

function previewClassificationFacts(
  category: LibraryWorkspaceItem["category"],
  labels: LibraryWorkspaceLabels,
): PreviewFact[] {
  const CategoryIcon = PREVIEW_CATEGORY_ICONS[category];
  return [
    {
      label: labels.category,
      value: (
        <span className="app-library-preview__fact-kind">
          <CategoryIcon size={13} aria-hidden="true" />
          {previewCategoryLabel(category, labels)}
        </span>
      ),
    },
  ];
}

function PreviewFacts(props: {
  facts: PreviewFact[];
  labels: LibraryWorkspaceLabels;
}) {
  if (props.facts.length === 0) return null;
  return (
    <section className="app-library-preview__facts" aria-label={props.labels.itemDetails}>
      <h3 className="sr-only">{props.labels.itemDetails}</h3>
      <dl className="app-dialog-list-card app-dialog-list-card-content">
        {props.facts.map((fact) => (
          <div className="app-library-preview__fact app-dialog-row" key={fact.label}>
            <dt>{fact.label}</dt>
            <dd title={fact.title}>{fact.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function PreviewContextHeader(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  coverURL: string;
  fallbackCoverURL?: string;
  title?: string;
  status?: string;
  format?: string;
  category?: LibraryWorkspaceItem["category"];
  titleContent?: React.ReactNode;
}) {
  return (
    <div className="app-library-preview__context">
      <PreviewArtworkImage
        src={props.coverURL}
        fallbackSrc={props.fallbackCoverURL}
        category={props.category ?? props.item.category}
        otherGroup={props.item.otherGroup}
        alt=""
      />
      <PreviewIdentity
        title={props.title ?? props.item.title}
        titleContent={props.titleContent}
        subtitle={props.item.libraryName || props.labels.library}
        category={props.category ?? props.item.category}
        status={props.status ?? libraryItemDisplayStatus(props.item)}
        format={props.format ?? props.item.format}
        labels={props.labels}
        compact
      />
    </div>
  );
}

function PreviewMedia(props: {
  category: LibraryWorkspaceItem["category"];
  otherGroup?: LibraryWorkspaceItem["otherGroup"];
  title: string;
  coverURL: string;
  fallbackCoverURL?: string;
  sourceURL?: string;
  sourcePath?: string;
  format?: string;
  durationMs?: number;
  logPreviewURL?: string;
  labels: LibraryWorkspaceLabels;
}) {
  const sourceURL = props.sourceURL?.trim() ?? "";
  const extension = extractExtensionFromPath(props.sourcePath ?? "");
  const format = (props.format || extension).trim().toLowerCase();

  let media: React.ReactNode = (
    <LibraryIpodPreview
      category={props.category}
      coverURL={props.coverURL}
      fallbackCoverURL={props.fallbackCoverURL}
      labels={props.labels}
      otherGroup={props.otherGroup}
      sourceURL={sourceURL}
      title={props.title}
    />
  );
  if (
    props.category === "book" &&
    sourceURL &&
    (format === "pdf" || format.endsWith("/pdf") || extension === "pdf")
  ) {
    media = (
      <LibraryPdfPreview
        labels={props.labels}
        sourceURL={sourceURL}
        title={props.title}
      />
    );
  } else if (
    props.logPreviewURL &&
    (format === "log" || format === "text/x-log" || extension === "log")
  ) {
    media = (
      <LibraryLogPreview
        labels={props.labels}
        sourceURL={props.logPreviewURL}
        title={props.title}
      />
    );
  }

  return (
    <div className="app-library-preview__media" data-preview-kind={props.category}>
      {media}
    </div>
  );
}

function operationProgress(item: LibraryWorkspaceItem) {
  const progress = item.operation?.progress;
  if (Number.isFinite(progress?.percent)) {
    return Math.min(100, Math.max(0, progress?.percent ?? 0));
  }
  if (Number.isFinite(progress?.current) && Number.isFinite(progress?.total) && (progress?.total ?? 0) > 0) {
    return Math.min(100, Math.max(0, ((progress?.current ?? 0) / (progress?.total ?? 1)) * 100));
  }
  return undefined;
}

function TaskFileList(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  onOpenItem?: (itemId: string) => void;
}) {
  const files = props.item.taskFiles ?? [];
  return (
    <section className="app-library-preview__task-files" aria-label={props.labels.taskFiles}>
      <h3>
        <span>{props.labels.taskFiles}</span>
        <small>{files.length}</small>
      </h3>
      {files.length === 0 ? (
        <p className="app-library-preview__empty">{props.labels.taskNoFiles}</p>
      ) : (
        <div className="app-library-preview__task-file-list">
          {files.map((file) => (
            <article className="app-library-preview__task-file app-dialog-list-card" key={file.fileId}>
              <header>
                {file.file?.id && !file.file.state.deleted ? (
                  <FileRenameTitle
                    fileId={file.file.id}
                    labels={props.labels}
                    placement="row"
                    title={file.title}
                  />
                ) : (
                  <strong title={file.title}>{file.title}</strong>
                )}
                <small>{file.format || file.kind}</small>
              </header>
              <dl>
                <div>
                  <dt>{props.labels.category}</dt>
                  <dd>{previewCategoryLabel(file.category, props.labels)}</dd>
                </div>
                {shouldShowLibraryStatusBadge(file.status) ? (
                  <div>
                    <dt>{props.labels.status}</dt>
                    <dd>
                      <StatusBadge marker tone={previewStatusTone(file.status)}>
                        {props.labels.statusLabel(file.status)}
                      </StatusBadge>
                    </dd>
                  </div>
                ) : null}
              </dl>
              <div className="app-library-preview__task-file-actions">
                <button
                  disabled={!file.canView || !props.onOpenItem}
                  onClick={() => props.onOpenItem?.(file.previewItemId)}
                  type="button"
                >
                  <ScanEye aria-hidden="true" size={14} />
                  {props.labels.view}
                </button>
                <TaskFileDeleteAction
                  disabled={
                    !props.item.operation?.operationId ||
                    props.item.operation.status.trim().toLocaleLowerCase() !== "succeeded"
                  }
                  fileId={file.fileId}
                  labels={props.labels}
                  operationId={props.item.operation?.operationId ?? ""}
                  title={file.title}
                />
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function TaskPreview(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  onOpenItem?: (itemId: string) => void;
}) {
  const progress = operationProgress(props.item);
  const operation = props.item.operation;
  const progressLabel = progress === undefined ? "–" : `${Math.round(progress)}%`;
  const progressDescription = [
    props.labels.operationStageLabel(operation?.progress?.stage),
    operation?.progress?.speed,
  ]
    .filter(Boolean)
    .join(" · ");
  const progressAnnouncement = [progressLabel, progressDescription].filter(Boolean).join(" · ");
  const taskPreviewItems = props.item.taskPreviewItems?.length
    ? props.item.taskPreviewItems
    : props.item.coverURL && !isLibraryDefaultArtworkURL(props.item.coverURL)
      ? [{
          id: `${props.item.id}:cover`,
          kind: "thumbnail",
          previewURL: props.item.coverURL,
        }]
      : [];
  const taskPreviewTotalCount = props.item.taskPreviewTotalCount ??
    (Number.isFinite(operation?.metrics.fileCount)
      ? operation?.metrics.fileCount
      : taskPreviewItems.length);
  return (
    <article className="app-library-preview__overview app-library-preview__task" data-preview-kind="task">
      <div className="app-library-preview__hero">
        <TaskFolderArtwork
          className="app-library-preview__task-folder-artwork"
          items={taskPreviewItems}
          totalCount={taskPreviewTotalCount}
          presentation="companion-open"
          view="grid"
        />
        {operation?.operationId ? (
          <TaskRenameTitle item={props.item} labels={props.labels} placement="hero" />
        ) : (
          <PreviewBouncingTitle title={props.item.title} />
        )}
      </div>
      <section
        className="app-library-preview__progress-card app-dialog-list-card"
        aria-label={`${props.labels.status}: ${props.labels.statusLabel(props.item.status)} · ${progressAnnouncement}`}
      >
        <header>
          <span>{props.labels.status}</span>
          <StatusBadge marker tone={previewStatusTone(props.item.status)}>
            {props.labels.statusLabel(props.item.status)}
          </StatusBadge>
        </header>
        {progress === undefined ? null : (
          <progress max={100} value={progress} aria-label={props.labels.progress} />
        )}
        <p>{[progressLabel, progressDescription].filter(Boolean).join(" · ")}</p>
      </section>
      <TaskFileList
        item={props.item}
        labels={props.labels}
        onOpenItem={props.onOpenItem}
      />
      <TaskDeleteAction item={props.item} labels={props.labels} />
    </article>
  );
}

function CatalogPreviewOverview(props: {
  item: LibraryWorkspaceItem;
  detail: CatalogItemDetail;
  labels: LibraryWorkspaceLabels;
  coverURL: string;
  sourceURL: string;
  sourcePath: string;
  sourceFormat: string;
  logPreviewURL: string;
}) {
  const sourceAsset = previewAsset(props.detail);
  const file = sourceAsset?.file ?? primaryAsset(props.detail)?.file;
  const durationMs = file?.media?.durationMs ?? props.detail.item.durationMs;
  const facts = previewClassificationFacts(
    props.detail.item.category,
    props.labels,
  );

  return (
    <article className="app-library-preview__overview" data-preview-kind={props.detail.item.category}>
      <div className="app-library-preview__hero">
        <PreviewMedia
          category={props.detail.item.category}
          otherGroup={props.item.otherGroup}
          title={props.detail.item.title}
          coverURL={props.coverURL}
          fallbackCoverURL={props.item.fallbackCoverURL}
          sourceURL={props.sourceURL}
          sourcePath={props.sourcePath}
          format={props.sourceFormat}
          durationMs={durationMs}
          logPreviewURL={props.logPreviewURL}
          labels={props.labels}
        />
        <CatalogRenameTitle detail={props.detail} labels={props.labels} placement="hero" />
      </div>
      {libraryItemAvailability(props.detail.item) !== "available" ? (
        <CatalogAvailabilityNotice detail={props.detail} labels={props.labels} />
      ) : null}
      <PreviewFacts
        labels={props.labels}
        facts={facts}
      />
      <CatalogLifecycleAction detail={props.detail} labels={props.labels} />
    </article>
  );
}

function CatalogAvailabilityNotice(props: {
  detail: CatalogItemDetail;
  labels: LibraryWorkspaceLabels;
}) {
  const availability = libraryItemAvailability(props.detail.item);
  const checking = availability === "checking";
  const label = availability === "offline"
    ? props.labels.offlineRootStatus
    : props.labels.statusLabel(availability);
  return (
    <div
      className="app-library-preview__availability-notice app-dialog-list-card"
      data-availability={availability}
      role="status"
    >
      {checking ? (
        <LoaderCircle aria-hidden="true" className="app-motion-spin" size={16} />
      ) : (
        <AlertTriangle aria-hidden="true" size={16} />
      )}
      <span>
        <strong>{label}</strong>
        {props.detail.source?.storageRootName ? (
          <small>{props.detail.source.storageRootName}</small>
        ) : null}
      </span>
    </div>
  );
}

function LegacyPreviewOverview(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  sourceURL: string;
}) {
  const facts = previewClassificationFacts(
    props.item.category,
    props.labels,
  );
  return (
    <article className="app-library-preview__overview" data-preview-kind={props.item.category}>
      <div className="app-library-preview__hero">
        <PreviewMedia
          category={props.item.category}
          otherGroup={props.item.otherGroup}
          title={props.item.title}
          coverURL={props.item.coverURL}
          fallbackCoverURL={props.item.fallbackCoverURL}
          sourceURL={props.sourceURL}
          sourcePath={props.item.path}
          format={props.item.format}
          durationMs={props.item.durationMs}
          labels={props.labels}
        />
        {props.item.file?.id && !props.item.file.state.deleted ? (
          <FileRenameTitle
            fileId={props.item.file.id}
            labels={props.labels}
            placement="hero"
            title={props.item.title}
          />
        ) : (
          <PreviewBouncingTitle title={props.item.title} />
        )}
      </div>
      <PreviewFacts
        labels={props.labels}
        facts={facts}
      />
      <LegacyFileDeleteAction item={props.item} labels={props.labels} />
    </article>
  );
}

function previewTabId(itemId: string, tab: LibraryPreviewTab) {
  return `library-preview-tab-${encodeURIComponent(itemId)}-${tab}`;
}

function previewPanelId(itemId: string, tab: LibraryPreviewTab) {
  return `library-preview-panel-${encodeURIComponent(itemId)}-${tab}`;
}

function PreviewPanelPlaceholders(props: {
  itemId: string;
  activeTab: LibraryPreviewTab;
}) {
  return TABS.filter((tab) => tab !== props.activeTab).map((tab) => (
    <div
      aria-labelledby={previewTabId(props.itemId, tab)}
      hidden
      id={previewPanelId(props.itemId, tab)}
      key={tab}
      role="tabpanel"
    />
  ));
}

function PreviewTabs(props: {
  itemId: string;
  itemTitle: string;
  labels: LibraryWorkspaceLabels;
  tab: LibraryPreviewTab;
  onChange: (tab: LibraryPreviewTab) => void;
}) {
  const items = TABS.map((entry) => {
    const Icon = TAB_ICONS[entry];
    return {
      value: entry,
      label: props.labels[entry],
      icon: <Icon size={15} aria-hidden="true" />,
      tabId: previewTabId(props.itemId, entry),
      panelId: previewPanelId(props.itemId, entry),
    };
  });

  return (
    <div className="app-library-preview__tabs-frame">
      <DreamSegmentSwitch
        value={props.tab}
        items={items}
        onValueChange={props.onChange}
        tooltips={false}
        className="app-library-preview__tabs"
        ariaLabel={props.itemTitle}
      />
    </div>
  );
}

export interface LibraryPreviewCompanionFooterProps {
  item: LibraryWorkspaceItem | null;
  activeTab: LibraryPreviewTab;
  onActiveTabChange: (tab: LibraryPreviewTab) => void;
  labels?: Partial<LibraryWorkspaceLabels>;
}

/**
 * The Library preview follows the same structural contract as the playback
 * companions: the shell owns the title, the selected surface owns the
 * scrollable content, and navigation lives in the shell footer.
 */
export function LibraryPreviewCompanionFooter(
  props: LibraryPreviewCompanionFooterProps,
) {
  const { language, t } = useI18n();
  const localized = React.useMemo(
    () => createLibraryWorkspaceLabels(t, language),
    [language, t],
  );
  const labels = React.useMemo(
    () => mergeLabels(localized, props.labels),
    [localized, props.labels],
  );

  if (!props.item) return null;

  return (
    <PreviewTabs
      itemId={props.item.id}
      itemTitle={props.item.title}
      labels={labels}
      tab={normalizePreviewTab(props.activeTab)}
      onChange={props.onActiveTabChange}
    />
  );
}

function LoadingState(props: { labels: LibraryWorkspaceLabels }) {
  return <div className="app-library-preview__loading"><LoaderCircle size={18} className="app-motion-spin" />{props.labels.loading}</div>;
}

interface PreviewInfoRow {
  label: string;
  value: React.ReactNode;
  title?: string;
  copyValue?: string;
  openLocation?:
    | { kind: "catalog-file"; fileId: string }
    | { kind: "legacy-path"; path: string };
}

async function copyPreviewValue(value: string) {
  const text = value.trim();
  if (!text) return;
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.className = "app-clipboard-fallback-textarea";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    const command = ["co", "py"].join("");
    if (!document.execCommand(command)) throw new Error("clipboard command failed");
  } finally {
    document.body.removeChild(textarea);
  }
}

function PreviewCopyValue(props: {
  value: string;
  label: string;
}) {
  const [copied, setCopied] = React.useState(false);
  React.useEffect(() => setCopied(false), [props.value]);
  return (
    <button
      aria-label={props.label}
      className="app-library-preview__copy-value"
      data-copied={copied ? "true" : undefined}
      onClick={() => {
        void copyPreviewValue(props.value).then(() => setCopied(true));
      }}
      title={props.label}
      type="button"
    >
      {copied ? <Check aria-hidden="true" size={13} /> : <Copy aria-hidden="true" size={13} />}
    </button>
  );
}

function PreviewOpenLocationValue(props: {
  labels: LibraryWorkspaceLabels;
  location:
    | { kind: "catalog-file"; fileId: string }
    | { kind: "legacy-path"; path: string };
}) {
  const openFileLocation = useOpenLibraryFileLocation();
  const openPath = useOpenLibraryPath();
  const pending = openFileLocation.isPending || openPath.isPending;
  const locationKey = props.location.kind === "catalog-file"
    ? props.location.fileId.trim()
    : props.location.path.trim();

  const handleOpenLocation = async () => {
    if (!locationKey) return;
    try {
      if (props.location.kind === "catalog-file") {
        await openFileLocation.mutateAsync({ fileId: locationKey });
      } else {
        await openPath.mutateAsync({ path: locationKey });
      }
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: props.labels.openLocation,
        description: errorMessage(error, props.labels),
        source: "xiadown.library",
      });
    }
  };

  return (
    <button
      aria-label={props.labels.openLocation}
      className="app-library-preview__location-value"
      disabled={!locationKey || pending}
      onClick={() => void handleOpenLocation()}
      title={props.labels.openLocation}
      type="button"
    >
      {pending ? (
        <LoaderCircle aria-hidden="true" className="app-motion-spin" size={13} />
      ) : (
        <FolderOpen aria-hidden="true" size={13} />
      )}
    </button>
  );
}

function PreviewInfoRows(props: {
  rows: readonly PreviewInfoRow[];
  copyLabel: string;
  labels: LibraryWorkspaceLabels;
}) {
  return (
    <dl className="app-library-preview__info app-dialog-list-card app-dialog-list-card-content">
      {props.rows.map((row) => (
        <div key={row.label}>
          <dt>{row.label}</dt>
          <dd title={row.title}>
            <span>{row.value}</span>
            {row.copyValue?.trim() ? (
              <PreviewCopyValue label={props.copyLabel} value={row.copyValue} />
            ) : null}
            {row.openLocation ? (
              <PreviewOpenLocationValue labels={props.labels} location={row.openLocation} />
            ) : null}
          </dd>
        </div>
      ))}
    </dl>
  );
}

export function LibraryDeleteConfirmationActions(props: {
  labels: LibraryWorkspaceLabels;
  pending: boolean;
  showCascadeFiles?: boolean;
  cascadeFilesLabel?: string;
  cascadeFiles: boolean;
  mutationError?: string;
  onCascadeFilesChange: (value: boolean) => void;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <>
      {props.showCascadeFiles ? (
        <label className="app-library-preview__cascade-files">
          <input
            checked={props.cascadeFiles}
            disabled={props.pending}
            onChange={(event) => props.onCascadeFilesChange(event.currentTarget.checked)}
            type="checkbox"
          />
          <span>{props.cascadeFilesLabel ?? props.labels.deleteFiles}</span>
        </label>
      ) : null}
      {props.mutationError ? (
        <p className="app-library-preview__error" role="alert">{props.mutationError}</p>
      ) : null}
      <div className="app-dialog-footer">
        <Button disabled={props.pending} onClick={props.onCancel} type="button" variant="outline">
          {props.labels.cancelDialog}
        </Button>
        <Button disabled={props.pending} onClick={props.onConfirm} type="button" variant="destructive">
          {props.pending ? <LoaderCircle aria-hidden="true" className="app-motion-spin" size={14} /> : null}
          {props.labels.deleteItem}
        </Button>
      </div>
    </>
  );
}

function DeleteConfirmationButton(props: {
  labels: LibraryWorkspaceLabels;
  title: string;
  description: string;
  pending: boolean;
  disabled?: boolean;
  showCascadeFiles?: boolean;
  cascadeFilesLabel?: string;
  triggerClassName?: string;
  onConfirm: (cascadeFiles: boolean) => Promise<void>;
}) {
  const [open, setOpen] = React.useState(false);
  const [cascadeFiles, setCascadeFiles] = React.useState(false);
  const [mutationError, setMutationError] = React.useState("");

  const setDialogOpen = (next: boolean) => {
    if (props.pending) return;
    setOpen(next);
    if (next) {
      setCascadeFiles(false);
      setMutationError("");
    }
  };
  const confirm = async () => {
    if (props.pending) return;
    setMutationError("");
    try {
      await props.onConfirm(cascadeFiles);
      setOpen(false);
    } catch (error) {
      setMutationError(errorMessage(error, props.labels));
    }
  };

  return (
    <>
      <Button
        className={props.triggerClassName ?? "app-library-preview__delete-button"}
        disabled={props.disabled || props.pending}
        onClick={() => setDialogOpen(true)}
        type="button"
        variant="destructive"
      >
        <Trash2 aria-hidden="true" size={14} />
        {props.labels.deleteItem}
      </Button>
      <Dialog open={open} onOpenChange={setDialogOpen}>
        <DialogContent className="app-library-preview__delete-dialog">
          <DialogHeader>
            <DialogTitle className="pr-8">{props.title}</DialogTitle>
            <DialogDescription>{props.description}</DialogDescription>
          </DialogHeader>
          <LibraryDeleteConfirmationActions
            cascadeFiles={cascadeFiles}
            cascadeFilesLabel={props.cascadeFilesLabel}
            labels={props.labels}
            mutationError={mutationError}
            onCancel={() => setDialogOpen(false)}
            onCascadeFilesChange={setCascadeFiles}
            onConfirm={() => void confirm()}
            pending={props.pending}
            showCascadeFiles={props.showCascadeFiles}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}

function TaskFileDeleteAction(props: {
  disabled?: boolean;
  fileId: string;
  labels: LibraryWorkspaceLabels;
  operationId: string;
  title: string;
}) {
  const deleteOutput = useDeleteOperationOutput();
  return (
    <DeleteConfirmationButton
      cascadeFilesLabel={props.labels.alsoDeleteFile}
      description={props.labels.removeTaskOutputMessage(props.title)}
      disabled={props.disabled}
      labels={props.labels}
      onConfirm={async (deleteFile) => {
        await deleteOutput.mutateAsync({
          operationId: props.operationId,
          fileId: props.fileId,
          deleteFile,
        });
      }}
      pending={deleteOutput.isPending}
      showCascadeFiles
      title={props.labels.removeTaskOutputTitle}
      triggerClassName="app-library-preview__task-file-delete"
    />
  );
}

function CatalogLifecycleAction(props: {
  detail: CatalogItemDetail;
  labels: LibraryWorkspaceLabels;
}) {
  const trashItem = useTrashCatalogItem();
  const restoreItem = useRestoreCatalogItem();
  const pending = trashItem.isPending || restoreItem.isPending;
  if (props.detail.item.status === "trashed") {
    return (
      <Button
        className="app-library-preview__delete-button"
        disabled={pending}
        onClick={() => restoreItem.mutate({
          id: props.detail.item.id,
          expectedRevision: props.detail.item.revision,
          actorId: LIBRARY_CATALOG_ACTOR_ID,
        })}
        type="button"
        variant="outline"
      >
        <RotateCcw aria-hidden="true" size={14} />
        {props.labels.restoreItem}
      </Button>
    );
  }
  return (
    <DeleteConfirmationButton
      description={props.labels.deleteFileMessage(props.detail.item.title)}
      labels={props.labels}
      onConfirm={async () => {
        await trashItem.mutateAsync({
          id: props.detail.item.id,
          expectedRevision: props.detail.item.revision,
          actorId: LIBRARY_CATALOG_ACTOR_ID,
        });
      }}
      pending={trashItem.isPending}
      title={props.labels.deleteFileTitle}
    />
  );
}

function LegacyFileDeleteAction(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
}) {
  if (!props.item.file || props.item.file.state.deleted) return null;
  return (
    <LegacyFileDeleteMutationAction
      fileId={props.item.file.id}
      labels={props.labels}
      title={props.item.title}
    />
  );
}

function LegacyFileDeleteMutationAction(props: {
  fileId: string;
  labels: LibraryWorkspaceLabels;
  title: string;
}) {
  const deleteFiles = useDeleteFiles();
  return (
    <DeleteConfirmationButton
      description={props.labels.deleteFileMessage(props.title)}
      labels={props.labels}
      onConfirm={async () => {
        await deleteFiles.mutateAsync({
          fileIds: [props.fileId],
          deleteFiles: true,
        });
      }}
      pending={deleteFiles.isPending}
      title={props.labels.deleteFileTitle}
    />
  );
}

function TaskDeleteAction(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
}) {
  if (!props.item.operation) return null;
  return (
    <TaskDeleteMutationAction
      labels={props.labels}
      operationId={props.item.operation.operationId}
      title={props.item.title}
    />
  );
}

function TaskDeleteMutationAction(props: {
  labels: LibraryWorkspaceLabels;
  operationId: string;
  title: string;
}) {
  const deleteOperation = useDeleteOperation();
  return (
    <DeleteConfirmationButton
      description={`${props.labels.deleteTaskTitle} “${props.title}”?`}
      labels={props.labels}
      onConfirm={async (cascadeFiles) => {
        await deleteOperation.mutateAsync({
          operationId: props.operationId,
          cascadeFiles,
        });
      }}
      pending={deleteOperation.isPending}
      showCascadeFiles
      title={props.labels.deleteTaskTitle}
    />
  );
}

function CatalogInfoPanel(props: {
  detail: CatalogItemDetail;
  labels: LibraryWorkspaceLabels;
}) {
  const selectedAsset = previewAsset(props.detail) ?? primaryAsset(props.detail);
  const file = selectedAsset?.file;
  const source = props.detail.source;
  const sourceRows: PreviewInfoRow[] = [];
  if (source) {
    let sourceLabel = props.labels.unknownSource;
    switch (source.originKind.trim().toLocaleLowerCase()) {
      case "download":
        sourceLabel = props.labels.downloadedSource;
        break;
      case "import":
        sourceLabel = source.storageMode === "managed"
          ? props.labels.managedImportSource
          : props.labels.referencedImportSource;
        break;
      case "transcode":
        sourceLabel = props.labels.generatedSource;
        break;
    }
    const storageModeLabel = source.storageMode === "managed"
      ? props.labels.managedMode
      : source.storageMode === "referenced"
        ? props.labels.referencedMode
        : props.labels.unmanagedMode;
    sourceRows.push(
      { label: props.labels.source, value: sourceLabel },
      { label: props.labels.storageMode, value: storageModeLabel },
    );
    if (source.storageRootId || source.storageRootPath) {
      sourceRows.push({
        label: props.labels.storageRoot,
        value: source.storageRootName || source.storageRootPath || "–",
        title: source.storageRootPath,
      });
    }
    if (source.importPath) {
      sourceRows.push({
        label: props.labels.sourceFile,
        value: source.importPath,
        title: source.importPath,
        copyValue: source.importPath,
      });
    }
    if (source.importedAt) {
      sourceRows.push({
        label: props.labels.importedAt,
        value: props.labels.dateTimeValue(source.importedAt),
      });
    }
    if (source.importBatchId) {
      sourceRows.push({
        label: props.labels.importBatch,
        value: source.importBatchId,
        copyValue: source.importBatchId,
      });
    }
    if (source.operationId) {
      sourceRows.push({
        label: props.labels.associatedTask,
        value: source.operationId,
        copyValue: source.operationId,
      });
    }
  }
  const rows: PreviewInfoRow[] = [
    { label: props.labels.category, value: previewCategoryLabel(props.detail.item.category, props.labels) },
    { label: props.labels.status, value: props.labels.statusLabel(props.detail.item.status) },
    {
      label: props.labels.availability,
      value: libraryItemAvailability(props.detail.item) === "offline"
        ? props.labels.offlineRootStatus
        : props.labels.statusLabel(libraryItemAvailability(props.detail.item)),
    },
    ...sourceRows,
    { label: props.labels.format, value: file?.media?.format || extractExtensionFromPath(file?.storage.localPath ?? "").toUpperCase() || "–" },
    { label: props.labels.size, value: formatBytes(file?.media?.sizeBytes ?? props.detail.item.sizeBytes) },
    { label: props.labels.duration, value: formatDuration(file?.media?.durationMs ?? props.detail.item.durationMs) },
    {
      label: props.labels.location,
      value: file?.storage.localPath || "–",
      title: file?.storage.localPath,
      openLocation: selectedAsset?.fileAvailable && file?.id.trim()
        ? { kind: "catalog-file", fileId: file.id }
        : undefined,
    },
    { label: props.labels.created, value: props.labels.dateTimeValue(props.detail.item.createdAt) },
    { label: props.labels.updated, value: props.labels.dateTimeValue(props.detail.item.updatedAt) },
  ];
  return <PreviewInfoRows copyLabel={props.labels.copyDownloadURL} labels={props.labels} rows={rows} />;
}

function LegacyInfoPanel(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
}) {
  const rows: PreviewInfoRow[] = [
    { label: props.labels.category, value: previewCategoryLabel(props.item.category, props.labels) },
    { label: props.labels.status, value: props.labels.statusLabel(props.item.status) },
    { label: props.labels.format, value: props.item.format || "–" },
    { label: props.labels.size, value: formatBytes(props.item.sizeBytes) },
    { label: props.labels.duration, value: formatDuration(props.item.durationMs) },
    { label: props.labels.library, value: props.item.libraryName || props.labels.library },
    {
      label: props.labels.location,
      value: props.item.path || "–",
      title: props.item.path,
      openLocation: props.item.path.trim()
        ? { kind: "legacy-path", path: props.item.path }
        : undefined,
    },
    { label: props.labels.created, value: props.labels.dateTimeValue(props.item.createdAt) },
    { label: props.labels.updated, value: props.labels.dateTimeValue(props.item.updatedAt) },
  ];
  return <PreviewInfoRows copyLabel={props.labels.copyDownloadURL} labels={props.labels} rows={rows} />;
}

function TaskInfoPanel(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
}) {
  const operation = props.item.operation;
  const sourceURL = operation?.request?.url?.trim() ?? "";
  const inputPath = operation?.request?.inputPath?.trim() ?? "";
  const rows: PreviewInfoRow[] = [
    {
      label: props.labels.taskType,
      value: props.labels.operationKindLabel(operation?.kind ?? props.item.format),
    },
    { label: props.labels.status, value: props.labels.statusLabel(props.item.status) },
    ...(sourceURL ? [{
      label: props.labels.downloadURL,
      value: sourceURL,
      title: sourceURL,
      copyValue: sourceURL,
    }] : []),
    ...(inputPath ? [{
      label: props.labels.sourceFile,
      value: inputPath,
      title: inputPath,
      copyValue: inputPath,
    }] : []),
    ...(operation?.domain ? [{ label: props.labels.source, value: operation.domain }] : []),
    ...(operation?.platform ? [{ label: props.labels.provenance, value: operation.platform }] : []),
    ...(operation?.uploader ? [{ label: props.labels.description, value: operation.uploader }] : []),
    ...(operation?.errorMessage ? [{ label: props.labels.loadFailed, value: operation.errorMessage }] : []),
    { label: props.labels.size, value: formatBytes(operation?.metrics.totalSizeBytes) },
    { label: props.labels.assets, value: props.labels.itemCount(props.item.taskFiles?.length ?? operation?.metrics.fileCount ?? 0) },
    ...(operation?.startedAt ? [{ label: props.labels.created, value: props.labels.dateTimeValue(operation.startedAt) }] : []),
    ...(operation?.finishedAt ? [{ label: props.labels.updated, value: props.labels.dateTimeValue(operation.finishedAt) }] : []),
  ];
  return <PreviewInfoRows copyLabel={props.labels.copyDownloadURL} labels={props.labels} rows={rows} />;
}

function AssetCard(props: { asset: CatalogItemAsset; labels: LibraryWorkspaceLabels }) {
  const file = props.asset.file;
  const availability = props.asset.availability ||
    (props.asset.fileAvailable ? "available" : "missing");
  return (
    <article className="app-library-preview__record">
      <header><strong>{props.asset.label || file?.displayName || file?.name || props.labels.asset}</strong><span>{props.labels.catalogValueLabel(props.asset.role)}</span></header>
      <dl>
        <div>
          <dt>{props.labels.availability}</dt>
          <dd>
            {availability === "offline"
              ? props.labels.offlineRootStatus
              : props.labels.statusLabel(availability)}
          </dd>
        </div>
        <div><dt>{props.labels.format}</dt><dd>{file?.media?.format || "–"}</dd></div>
        <div><dt>{props.labels.size}</dt><dd>{formatBytes(file?.media?.sizeBytes)}</dd></div>
        <div><dt>{props.labels.duration}</dt><dd>{formatDuration(file?.media?.durationMs)}</dd></div>
        <div><dt>{props.labels.location}</dt><dd title={file?.storage.localPath}>{file?.storage.localPath || "–"}</dd></div>
      </dl>
    </article>
  );
}

function RepresentationCard(props: { representation: CatalogRepresentation; labels: LibraryWorkspaceLabels }) {
  const value = props.representation;
  return (
    <article className="app-library-preview__record">
      <header><strong>{props.labels.catalogValueLabel(value.kind)}</strong><span>{props.labels.catalogValueLabel(value.purpose)} · {props.labels.catalogValueLabel(value.availability)}</span></header>
      <dl>
        {value.mediaType ? <div><dt>{props.labels.mediaType}</dt><dd>{value.mediaType}</dd></div> : null}
        {value.container ? <div><dt>{props.labels.container}</dt><dd>{value.container}</dd></div> : null}
        {value.codec ? <div><dt>{props.labels.codec}</dt><dd>{value.codec}</dd></div> : null}
        {value.width && value.height ? <div><dt>{props.labels.resolution}</dt><dd>{value.width} × {value.height}</dd></div> : null}
        {value.durationMs ? <div><dt>{props.labels.duration}</dt><dd>{formatDuration(value.durationMs)}</dd></div> : null}
        {value.bitrateBps ? <div><dt>{props.labels.bitrate}</dt><dd>{formatBytes(value.bitrateBps)}/s</dd></div> : null}
        {value.sizeBytes ? <div><dt>{props.labels.size}</dt><dd>{formatBytes(value.sizeBytes)}</dd></div> : null}
        {value.language ? <div><dt>{props.labels.language}</dt><dd>{value.language}</dd></div> : null}
        {value.checksum ? <div><dt>{props.labels.checksum}</dt><dd title={value.checksum}>{value.checksumAlgorithm ? `${value.checksumAlgorithm}: ` : ""}{value.checksum}</dd></div> : null}
      </dl>
    </article>
  );
}

function CatalogVersions(props: { detail: CatalogItemDetail; labels: LibraryWorkspaceLabels }) {
  return (
    <div className="app-library-preview__versions app-library-preview__panel">
      <section>
        <h3>{props.labels.assets} <span>{props.detail.assets.length}</span></h3>
        {props.detail.assets.length === 0 ? <p className="app-library-preview__empty">{props.labels.noAssets}</p> : props.detail.assets.map((asset) => <AssetCard key={asset.id} asset={asset} labels={props.labels} />)}
      </section>
      <section>
        <h3>{props.labels.representations} <span>{props.detail.representations.length}</span></h3>
        {props.detail.representations.length === 0 ? <p className="app-library-preview__empty">{props.labels.noRepresentations}</p> : props.detail.representations.map((representation) => <RepresentationCard key={representation.id} representation={representation} labels={props.labels} />)}
      </section>
    </div>
  );
}

export function catalogItemFileEvents(
  detail: CatalogItemDetail,
  libraries: readonly LibraryDTO[],
) {
  const fileIds = new Set(
    detail.assets
      .map((asset) => asset.fileId.trim())
      .filter(Boolean),
  );
  const events = new Map<string, FileEventRecordDTO>();
  for (const library of libraries) {
    for (const event of library.records.fileEvents) {
      if (fileIds.has(event.fileId)) events.set(event.id, event);
    }
  }
  return [...events.values()].sort((left, right) => (
    (right.occurredAt?.trim() || right.createdAt)
      .localeCompare(left.occurredAt?.trim() || left.createdAt)
  ));
}

function CatalogActivity(props: { detail: CatalogItemDetail; labels: LibraryWorkspaceLabels }) {
  const activity = useCatalogItemActivity(props.detail.item.id, 20, true);
  const libraries = useListLibraries();
  const fileEvents = React.useMemo(
    () => catalogItemFileEvents(props.detail, libraries.data ?? []),
    [libraries.data, props.detail],
  );
  const catalogEntries = (activity.data ?? []).map((entry) => {
    const Icon = entry.action === "catalog_item_trashed"
      ? Trash2
      : entry.action === "catalog_item_restored"
        ? RotateCcw
        : Clock3;
    const title = entry.action === "catalog_item_trashed"
      ? props.labels.trashItem
      : entry.action === "catalog_item_restored"
        ? props.labels.restoreItem
        : props.labels.updated;
    return {
      key: `catalog:${entry.action}:${entry.revision}:${entry.occurredAt}`,
      title,
      description: `${props.labels.revision} ${entry.revision} · ${props.labels.catalogValueLabel(entry.actor)}`,
      occurredAt: entry.occurredAt,
      Icon,
    };
  });
  const fileEntries = fileEvents.map((event) => ({
    key: `file-event:${event.id}`,
    title: fileEventTitle(event, props.labels),
    description: fileEventDescription(event, props.labels),
    occurredAt: event.occurredAt?.trim() || event.createdAt,
    Icon: fileEventIcon(event),
  }));
  const recordedEntries = [...catalogEntries, ...fileEntries]
    .sort((left, right) => right.occurredAt.localeCompare(left.occurredAt));
  const recordedTimes = new Set(recordedEntries.map((entry) => entry.occurredAt));
  return (
    <section className="app-library-preview__timeline app-library-preview__panel">
      <h3 className="app-library-preview__section-title">{props.labels.activity}</h3>
      {recordedEntries.map((entry) => (
        <div key={entry.key}>
          <entry.Icon size={15} />
          <span>
            <strong>{entry.title}</strong>
            <small>
              {entry.description ? `${entry.description} · ` : ""}
              {props.labels.dateTimeValue(entry.occurredAt)}
            </small>
          </span>
        </div>
      ))}
      {!recordedTimes.has(props.detail.item.updatedAt) ? (
        <div><Clock3 size={15} /><span><strong>{props.labels.updated}</strong><small>{props.labels.dateTimeValue(props.detail.item.updatedAt)}</small></span></div>
      ) : null}
      <div><Clock3 size={15} /><span><strong>{props.labels.created}</strong><small>{props.labels.dateTimeValue(props.detail.item.createdAt)}</small></span></div>
    </section>
  );
}

function taskOutputVersionLabel(
  state: TaskOutputVersionState,
  labels: LibraryWorkspaceLabels,
) {
  switch (state) {
    case "current": return labels.outputCurrent;
    case "historical": return labels.outputHistorical;
    case "detached": return labels.outputDetached;
    case "deleted": return labels.statusLabel("deleted");
    case "missing": return labels.statusLabel("missing");
  }
}

function taskOutputVersionTone(state: TaskOutputVersionState): DreamStatusTone {
  switch (state) {
    case "current": return "success";
    case "historical":
    case "detached": return "muted";
    case "deleted":
    case "missing": return "danger";
  }
}

function TaskOutputVersions(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
}) {
  const versions = projectTaskOutputVersions(props.item);
  if (versions.length === 0) {
    return <p className="app-library-preview__empty">{props.labels.noVersions}</p>;
  }
  return (
    <div className="app-library-preview__versions app-library-preview__panel">
      <section>
        <h3>{props.labels.taskFiles} <span>{versions.length}</span></h3>
        {versions.map((version) => (
          <article className="app-library-preview__record" key={version.fileId}>
            <header>
              <strong title={version.name}>{version.name}</strong>
              <StatusBadge marker tone={taskOutputVersionTone(version.state)}>
                {taskOutputVersionLabel(version.state, props.labels)}
              </StatusBadge>
            </header>
            <dl>
              <div>
                <dt>{props.labels.category}</dt>
                <dd>{props.labels.catalogValueLabel(version.kind)}</dd>
              </div>
              <div>
                <dt>{props.labels.format}</dt>
                <dd>{version.format || "–"}</dd>
              </div>
              <div>
                <dt>{props.labels.size}</dt>
                <dd>{formatBytes(version.sizeBytes)}</dd>
              </div>
              {version.changedAt ? (
                <div>
                  <dt>{props.labels.updated}</dt>
                  <dd>{props.labels.dateTimeValue(version.changedAt)}</dd>
                </div>
              ) : null}
            </dl>
          </article>
        ))}
      </section>
    </div>
  );
}

function fileEventTitle(event: FileEventRecordDTO, labels: LibraryWorkspaceLabels) {
  switch (event.eventType) {
    case "operation_output_detached": return labels.removeTaskOutputTitle;
    case "file_created": return `${labels.created} · ${labels.source}`;
    case "file_deleted": return labels.deleteFileTitle;
    case "file_restored": return labels.restoreItem;
    case "file_renamed": return `${labels.updated} · ${labels.title}`;
    case "file_relinked": return `${labels.updated} · ${labels.location}`;
    case "file_imported": return `${labels.created} · ${labels.source}`;
    case "file_missing_detected": return labels.statusLabel("missing");
    case "file_available_again": return labels.statusLabel("active");
    default: return labels.updated;
  }
}

function fileEventIcon(event: FileEventRecordDTO): LucideIcon {
  switch (event.eventType) {
    case "operation_output_detached":
    case "file_deleted": return Trash2;
    case "file_created": return Check;
    case "file_restored": return RotateCcw;
    case "file_relinked": return FolderOpen;
    case "file_renamed": return PencilLine;
    case "file_missing_detected": return AlertTriangle;
    case "file_available_again": return Check;
    default: return Clock3;
  }
}

function fileEventDescription(event: FileEventRecordDTO, labels: LibraryWorkspaceLabels) {
  const snapshot = event.detail.after ?? event.detail.before;
  const changes = event.detail.changes ?? [];
  const changedName = changes.find((change) =>
    ["name", "fileName", "displayName"].includes(change.field),
  );
  const changedPath = changes.find((change) =>
    ["localPath", "path", "storage.localPath"].includes(change.field),
  );
  const transition = event.eventType === "file_renamed"
    ? changedName
    : event.eventType === "file_relinked"
      ? changedPath
      : undefined;
  if (transition?.before && transition.after) {
    return `${transition.before} → ${transition.after}`;
  }
  const identity = snapshot?.name?.trim() || snapshot?.localPath?.trim() || event.fileId;
  return event.eventType === "operation_output_detached" && event.detail.deleteFile
    ? `${identity} · ${labels.statusLabel("deleted")}`
    : identity;
}

function LibraryItemActivity(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
}) {
  const itemEvents = fileEventsForLibraryItem(props.item);
  const eventEntries = itemEvents.map((event) => ({
    key: event.id,
    title: fileEventTitle(event, props.labels),
    description: fileEventDescription(event, props.labels),
    occurredAt: event.occurredAt?.trim() || event.createdAt,
    Icon: fileEventIcon(event),
  }));
  // Older XiaDown builds persisted detachedOutputFileIds without an event.
  // Surface that durable association state as a compatibility entry while all
  // new mutations use the immutable event stream above.
  const recordedDetachIds = new Set(
    itemEvents
      .filter((event) => event.eventType === "operation_output_detached")
      .map((event) => event.fileId),
  );
  const legacyDetachEntries = props.item.source === "task"
    ? (props.item.operation?.detachedOutputFileIds ?? [])
      .filter((fileId) => !recordedDetachIds.has(fileId))
      .map((fileId) => {
        const file = props.item.library?.files.find((value) => value.id === fileId);
        return {
          key: `legacy-detach:${fileId}`,
          title: props.labels.removeTaskOutputTitle,
          description: file?.displayName?.trim() || file?.name.trim() || fileId,
          occurredAt: props.item.library?.updatedAt || props.item.updatedAt,
          Icon: Trash2,
        };
      })
    : [];
  const renameTransitions = new Map(
    operationRenameTransitionsForTask(props.item).map((transition) => [
      transition.recordId,
      transition,
    ]),
  );
  const operationEntries = operationHistoryEventsForTask(props.item).map((record) => {
    const title = record.action === "operation_renamed"
      ? props.labels.operationRenamed
      : record.action === "operation_canceled"
        ? props.labels.operationCanceled
        : record.action === "operation_resumed"
          ? props.labels.operationResumed
          : props.labels.updated;
    const renameTransition = renameTransitions.get(record.recordId);
    return {
      key: `operation-event:${record.recordId}`,
      title,
      description: renameTransition?.before && renameTransition.after
        ? `${renameTransition.before} → ${renameTransition.after}`
        : record.displayName,
      occurredAt: record.occurredAt,
      Icon: record.action === "operation_resumed"
        ? RotateCcw
        : record.action === "operation_renamed"
          ? PencilLine
          : Clock3,
    };
  });
  const recordedActivityTimes = new Set([
    ...eventEntries,
    ...legacyDetachEntries,
    ...operationEntries,
  ].map((entry) => entry.occurredAt));
  const entries = [
    ...eventEntries,
    ...legacyDetachEntries,
    ...operationEntries,
    ...(props.item.updatedAt && props.item.updatedAt !== props.item.createdAt &&
    !recordedActivityTimes.has(props.item.updatedAt) ? [{
      key: "item-updated",
      title: props.labels.updated,
      description: "",
      occurredAt: props.item.updatedAt,
      Icon: Clock3,
    }] : []),
    {
      key: "item-created",
      title: props.labels.created,
      description: "",
      occurredAt: props.item.createdAt,
      Icon: Clock3,
    },
  ].sort((left, right) => right.occurredAt.localeCompare(left.occurredAt));

  return (
    <section className="app-library-preview__timeline app-library-preview__panel">
      <h3 className="app-library-preview__section-title">{props.labels.activity}</h3>
      {entries.length === 0 ? <p className="app-library-preview__empty">{props.labels.noActivity}</p> : null}
      {entries.map((entry) => (
        <div key={entry.key}>
          <entry.Icon size={15} />
          <span>
            <strong>{entry.title}</strong>
            <small>
              {entry.description ? `${entry.description} · ` : ""}
              {props.labels.dateTimeValue(entry.occurredAt)}
            </small>
          </span>
        </div>
      ))}
    </section>
  );
}

function CatalogPreviewCompanion(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  httpBaseURL: string;
  tab: LibraryPreviewTab;
  onTabChange: (tab: LibraryPreviewTab) => void;
  tabsPlacement: "footer" | "external";
}) {
  const detail = useCatalogItem(props.item.catalogItem?.id ?? "", "", true);

  if (detail.isLoading || !detail.data) {
    const pendingState = detail.isError ? (
      <div className="app-library-preview__load-error" role="alert">
        <AlertTriangle size={22} /><span>{errorMessage(detail.error, props.labels)}</span>
        <button type="button" onClick={() => void detail.refetch()}>{props.labels.retry}</button>
      </div>
    ) : <LoadingState labels={props.labels} />;
    return (
      <section className="app-library-preview" data-library-preview={props.item.id}>
        <div
          className="app-library-preview__body"
          data-companion-scroll-owner="library-preview"
          data-preview-tab={props.tab}
          id={previewPanelId(props.item.id, props.tab)}
          aria-labelledby={previewTabId(props.item.id, props.tab)}
          role="tabpanel"
        >
          {props.tab === "preview" ? (
            <article
              className="app-library-preview__overview app-library-preview__overview--pending"
              data-preview-kind={props.item.category}
            >
              <div className="app-library-preview__hero">
                <PreviewMedia
                  category={props.item.category}
                  otherGroup={props.item.otherGroup}
                  title={props.item.title}
                  coverURL={props.item.coverURL}
                  fallbackCoverURL={props.item.fallbackCoverURL}
                  labels={props.labels}
                />
                <PreviewBouncingTitle title={props.item.title} />
              </div>
              <PreviewFacts
                labels={props.labels}
                facts={previewClassificationFacts(
                  props.item.category,
                  props.labels,
                )}
              />
              {pendingState}
            </article>
          ) : (
            <>
              <PreviewContextHeader
                item={props.item}
                labels={props.labels}
                coverURL={props.item.coverURL}
                fallbackCoverURL={props.item.fallbackCoverURL}
              />
              {pendingState}
            </>
          )}
        </div>
        <PreviewPanelPlaceholders itemId={props.item.id} activeTab={props.tab} />
        {props.tabsPlacement === "footer" ? (
          <footer className="app-library-preview__footer">
            <PreviewTabs
              itemId={props.item.id}
              itemTitle={props.item.title}
              labels={props.labels}
              tab={props.tab}
              onChange={props.onTabChange}
            />
          </footer>
        ) : null}
      </section>
    );
  }

  const previewMedia = resolveCatalogPreviewMedia(
    detail.data,
    // Preserve the list's generated video thumbnail when detail has no
    // dedicated artwork asset. Components still receive fallbackCoverURL
    // separately for a failed thumbnail request.
    props.item.coverURL,
    props.httpBaseURL,
  );
  const headerTitle = detail.data.item.title;
  return (
    <section className="app-library-preview" data-library-preview={props.item.id}>
      <div
        className="app-library-preview__body"
        data-companion-scroll-owner="library-preview"
        data-preview-tab={props.tab}
        id={previewPanelId(props.item.id, props.tab)}
        aria-labelledby={previewTabId(props.item.id, props.tab)}
        role="tabpanel"
      >
        {props.tab === "preview" ? (
          <CatalogPreviewOverview
            item={props.item}
            detail={detail.data}
            labels={props.labels}
            coverURL={previewMedia.coverURL}
            sourceURL={previewMedia.sourceURL}
            sourcePath={previewMedia.sourcePath}
            sourceFormat={previewMedia.sourceFormat}
            logPreviewURL={previewMedia.logPreviewURL}
          />
        ) : (
          <>
            <PreviewContextHeader
              item={props.item}
              labels={props.labels}
              coverURL={previewMedia.coverURL}
              fallbackCoverURL={props.item.fallbackCoverURL}
              title={headerTitle}
              titleContent={(
                <CatalogRenameTitle
                  detail={detail.data}
                  labels={props.labels}
                  placement="context"
                />
              )}
              status={libraryItemDisplayStatus(detail.data.item)}
              format={previewMedia.sourceFormat}
              category={detail.data.item.category}
            />
            <div className="app-library-preview__tab-content" data-library-preview-section={props.tab}>
              {props.tab === "info" ? <CatalogInfoPanel detail={detail.data} labels={props.labels} /> : null}
              {props.tab === "versions" ? <CatalogVersions detail={detail.data} labels={props.labels} /> : null}
              {props.tab === "activity" ? <CatalogActivity detail={detail.data} labels={props.labels} /> : null}
            </div>
          </>
        )}
      </div>
      <PreviewPanelPlaceholders itemId={props.item.id} activeTab={props.tab} />
      {props.tabsPlacement === "footer" ? (
        <footer className="app-library-preview__footer">
          <PreviewTabs
            itemId={props.item.id}
            itemTitle={headerTitle}
            labels={props.labels}
            tab={props.tab}
            onChange={props.onTabChange}
          />
        </footer>
      ) : null}
    </section>
  );
}

function LegacyReadonlyCompanion(props: {
  item: LibraryWorkspaceItem;
  labels: LibraryWorkspaceLabels;
  tab: LibraryPreviewTab;
  onTabChange: (tab: LibraryPreviewTab) => void;
  tabsPlacement: "footer" | "external";
  httpBaseURL: string;
  onOpenItem?: (itemId: string) => void;
}) {
  const sourceURL = props.item.source === "file"
    ? buildAssetPreviewURL(props.httpBaseURL, props.item.path, props.item.updatedAt)
    : "";
  return (
    <section className="app-library-preview" data-library-preview={props.item.id}>
      <div
        className="app-library-preview__body"
        data-companion-scroll-owner="library-preview"
        data-preview-tab={props.tab}
        id={previewPanelId(props.item.id, props.tab)}
        aria-labelledby={previewTabId(props.item.id, props.tab)}
        role="tabpanel"
      >
        {props.tab === "preview" ? (
          props.item.source === "task"
            ? <TaskPreview item={props.item} labels={props.labels} onOpenItem={props.onOpenItem} />
            : <LegacyPreviewOverview item={props.item} labels={props.labels} sourceURL={sourceURL} />
        ) : (
          <>
            <PreviewContextHeader
              item={props.item}
              labels={props.labels}
              coverURL={props.item.coverURL}
              fallbackCoverURL={props.item.fallbackCoverURL}
              titleContent={props.item.source === "task" && props.item.operation?.operationId ? (
                <TaskRenameTitle
                  item={props.item}
                  labels={props.labels}
                  placement="context"
                />
              ) : props.item.source === "file" && props.item.file?.id && !props.item.file.state.deleted ? (
                <FileRenameTitle
                  fileId={props.item.file.id}
                  labels={props.labels}
                  placement="context"
                  title={props.item.title}
                />
              ) : undefined}
              format={props.item.source === "task"
                ? props.labels.operationKindLabel(props.item.operation?.kind ?? props.item.format)
                : undefined}
            />
            <div className="app-library-preview__tab-content" data-library-preview-section={props.tab}>
              {props.tab === "info" ? (
                props.item.source === "task"
                  ? <TaskInfoPanel item={props.item} labels={props.labels} />
                  : <LegacyInfoPanel item={props.item} labels={props.labels} />
              ) : props.tab === "activity" ? (
                <LibraryItemActivity item={props.item} labels={props.labels} />
              ) : props.tab === "versions" && props.item.source === "task" ? (
                <TaskOutputVersions item={props.item} labels={props.labels} />
              ) : (
                <p className="app-library-preview__empty">
                  {props.labels.noVersions}
                </p>
              )}
            </div>
          </>
        )}
      </div>
      <PreviewPanelPlaceholders itemId={props.item.id} activeTab={props.tab} />
      {props.tabsPlacement === "footer" ? (
        <footer className="app-library-preview__footer">
          <PreviewTabs
            itemId={props.item.id}
            itemTitle={props.item.title}
            labels={props.labels}
            tab={props.tab}
            onChange={props.onTabChange}
          />
        </footer>
      ) : null}
    </section>
  );
}

export function LibraryPreviewCompanion(props: LibraryPreviewCompanionProps) {
  const { language, t } = useI18n();
  const localized = React.useMemo(
    () => createLibraryWorkspaceLabels(t, language),
    [language, t],
  );
  const labels = React.useMemo(() => mergeLabels(localized, props.labels), [localized, props.labels]);
  const [uncontrolledTab, setUncontrolledTab] = React.useState<LibraryPreviewTab>(
    normalizePreviewTab(props.initialTab),
  );
  const tab = props.activeTab === undefined
    ? uncontrolledTab
    : normalizePreviewTab(props.activeTab);
  const setTab = React.useCallback(
    (next: LibraryPreviewTab) => {
      if (props.activeTab === undefined) setUncontrolledTab(next);
      props.onActiveTabChange?.(next);
    },
    [props.activeTab, props.onActiveTabChange],
  );
  const tabsPlacement = props.tabsPlacement ?? "footer";

  if (!props.item) {
    return (
      <section className="app-library-preview app-library-preview--empty" data-library-preview="empty">
        <span><ScanEye size={26} /></span>
        <h2>{labels.noSelection}</h2>
        <p>{labels.noSelectionDescription}</p>
      </section>
    );
  }

  return props.item.catalogItem ? (
    <CatalogPreviewCompanion
      item={props.item}
      labels={labels}
      httpBaseURL={props.httpBaseURL ?? ""}
      tab={tab}
      onTabChange={setTab}
      tabsPlacement={tabsPlacement}
    />
  ) : (
    <LegacyReadonlyCompanion
      item={props.item}
      labels={labels}
      tab={tab}
      onTabChange={setTab}
      tabsPlacement={tabsPlacement}
      httpBaseURL={props.httpBaseURL ?? ""}
      onOpenItem={props.onOpenItem}
    />
  );
}
