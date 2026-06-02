import { CheckCircle2, CircleSlash, Clock3, Copy, FileCog, FolderOpen, Loader2, PencilLine, RotateCcw, XCircle } from "lucide-react";
import * as React from "react";

import { MediaPreviewDialog, MediaPreviewSurface } from "@/app/media";
import { ListenLocalPreviewPlayer } from "@/app/main/Listen";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { OperationListItemDTO } from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";
import { getLanguage } from "@/shared/i18n";
import { messageBus } from "@/shared/message";
import { useOpenLibraryFileLocation, useOpenLibraryPath, useResumeOperation } from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogListCard, DialogListCardContent, DialogScrollArea, DialogTitle } from "@/shared/ui/dialog";
import { Select } from "@/shared/ui/select";
import { PetDisplay } from "@/shared/ui/pet-player";
import { SiteBrandIcon } from "@/shared/ui/site-brand-icon";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { formatBytes } from "@/shared/utils/formatBytes";

import { COMPLETED_FILE_TYPE_ORDER,COMPLETED_TASK_FILE_TYPE_LIMIT,COMPLETED_TEXT_PREVIEW_MAX_BYTES,canPreviewCompletedFile,formatCompletedTranscodedFromLabel,formatLocalDateTime,formatRelativeTime,isCompletedPreviewTooLarge,resolveCompletedFileDetailFooterItems,resolveCompletedFileDetailInfo,resolveCompletedFileFormatLabel,resolveCompletedImagePreviewURL,resolveCompletedPreviewGroupIcon,resolveCompletedPreviewGroupKind,resolveCompletedPreviewGroupLabel,resolveCompletedPreviewKind,resolveCompletedStatusLabel,resolveCompletedTaskSourceLabel,resolveOperationKindLabel,resolveSiteKeyForDomain,resolveUnknownErrorMessage } from "@/app/main/helpers";
import type { CompletedFileEntry,CompletedPreviewGroupKind,CompletedTaskEntry } from "@/app/main/types";

const TASK_DETAIL_GROUP_ORDER: CompletedPreviewGroupKind[] =
  COMPLETED_FILE_TYPE_ORDER;

type TaskDTOInfoRow = {
  key?: string;
  label: string;
  value: string;
  valueTooltip?: string;
  always?: boolean;
  copyValue?: string;
};

async function copyTextToClipboard(value: string) {
  const text = value.trim();
  if (!text) {
    return;
  }
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch {
      // Fall back to document selection for embedded WebViews.
    }
  }
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-10000px";
  textarea.style.top = "0";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    const clipboardCommand = ["co", "py"].join("");
    if (!document.execCommand(clipboardCommand)) {
      throw new Error("clipboard command failed");
    }
  } finally {
    document.body.removeChild(textarea);
  }
}

function formatTaskDTOValue(value: unknown): string {
  if (value === undefined || value === null || value === "") {
    return "-";
  }
  if (typeof value === "string") {
    return value.trim() || "-";
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  const serialized = JSON.stringify(value);
  return serialized && serialized.length > 0 ? serialized : "-";
}

function formatTaskDTODateTimeValue(value?: string): string {
  return formatTaskDTOValue(formatLocalDateTime(value) || value);
}

class PreviewTooLargeError extends Error {
  constructor() {
    super("preview is too large");
    this.name = "PreviewTooLargeError";
  }
}

async function readLimitedPreviewText(response: Response, maxBytes: number) {
  const contentLength = Number(response.headers.get("content-length") ?? "");
  if (Number.isFinite(contentLength) && contentLength > maxBytes) {
    throw new PreviewTooLargeError();
  }
  if (!response.body) {
    const buffer = await response.arrayBuffer();
    if (buffer.byteLength > maxBytes) {
      throw new PreviewTooLargeError();
    }
    return new TextDecoder().decode(buffer);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let totalBytes = 0;
  let text = "";
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      totalBytes += value.byteLength;
      if (totalBytes > maxBytes) {
        throw new PreviewTooLargeError();
      }
      text += decoder.decode(value, { stream: true });
    }
    return text + decoder.decode();
  } catch (error) {
    try {
      await reader.cancel();
    } catch {
      // Ignore cancellation races after fetch aborts.
    }
    throw error;
  }
}

function buildTaskDTOInfoRows(
  operation: OperationListItemDTO,
  text: ReturnType<typeof getXiaText>,
  transcodeSourceName?: string,
): TaskDTOInfoRow[] {
  const labels = text.completed.taskDataFields;
  const request = operation.request;
  const scale =
    request?.scale ||
    (request?.width && request?.height
      ? `${request.width} x ${request.height}`
      : "");
  const deleteSourceLabel = request?.deleteSourceFileAfterTranscode
    ? text.completed.taskDataFields.enabled
    : "";
  const status = (operation.status ?? "").trim().toLowerCase();
  const taskKind = (operation.kind ?? "").trim().toLowerCase();
  const transcodeSourceLabel =
    taskKind === "transcode" ? transcodeSourceName?.trim() ?? "" : "";
  const errorCode = (operation.errorCode ?? "").trim();
  const errorMessage = (operation.errorMessage ?? "").trim();
  const failureReason = errorMessage || errorCode || text.common.unknown;
  return [
    {
      key: "name",
      label: labels.name,
      value: formatTaskDTOValue(operation.name),
      always: true,
    },
    {
      label: labels.url,
      value: formatTaskDTOValue(request?.url),
      copyValue: request?.url?.trim() || undefined,
    },
    { label: labels.inputPath, value: formatTaskDTOValue(request?.inputPath) },
    {
      label: text.completed.transcodedFrom,
      value: formatTaskDTOValue(transcodeSourceLabel),
    },
    { label: labels.format, value: formatTaskDTOValue(request?.format) },
    { label: labels.preset, value: formatTaskDTOValue(request?.presetId) },
    { label: labels.videoCodec, value: formatTaskDTOValue(request?.videoCodec) },
    { label: labels.audioCodec, value: formatTaskDTOValue(request?.audioCodec) },
    {
      label: labels.qualityMode,
      value: formatTaskDTOValue(request?.qualityMode),
    },
    { label: labels.scale, value: formatTaskDTOValue(scale) },
    {
      label: labels.deleteSourceFileAfterTranscode,
      value: formatTaskDTOValue(deleteSourceLabel),
    },
    {
      label: labels.kind,
      value: resolveOperationKindLabel(text, operation.kind),
      always: true,
    },
    {
      label: labels.status,
      value: resolveCompletedStatusLabel(text, operation.status),
      always: true,
    },
    ...(status === "failed"
      ? [
          {
            label: labels.failureReason,
            value: formatTaskDTOValue(failureReason),
            valueTooltip: failureReason,
            always: true,
          },
        ]
      : []),
    ...(status === "failed" && errorMessage && errorCode
      ? [
          {
            label: labels.errorCode,
            value: formatTaskDTOValue(errorCode),
          },
        ]
      : []),
    { label: labels.domain, value: formatTaskDTOValue(operation.domain) },
    { label: labels.platform, value: formatTaskDTOValue(operation.platform) },
    { label: labels.uploader, value: formatTaskDTOValue(operation.uploader) },
    {
      label: labels.publishTime,
      value: formatTaskDTOValue(operation.publishTime),
    },
    { label: labels.startedAt, value: formatTaskDTODateTimeValue(operation.startedAt) },
    { label: labels.finishedAt, value: formatTaskDTODateTimeValue(operation.finishedAt) },
    {
      label: labels.createdAt,
      value: formatTaskDTODateTimeValue(operation.createdAt),
      always: true,
    },
  ].filter((row) => row.always || row.value !== "-");
}

export function DetailValueTooltip(props: {
  label: string;
  side?: React.ComponentProps<typeof TooltipContent>["side"];
  value?: string;
  children: React.ReactElement;
}) {
  const label = props.label?.trim();
  if (!label) {
    return props.children;
  }
  const value = props.value?.trim() ?? "";

  return (
    <Tooltip>
      <TooltipTrigger asChild>{props.children}</TooltipTrigger>
      {value ? (
        <TooltipContent
          side={props.side ?? "top"}
          className="app-completed-detail-value-tooltip !max-w-[min(42rem,calc(100vw-1rem))] !px-2.5 !py-1.5"
        >
          {label} {value}
        </TooltipContent>
      ) : (
        <TooltipContent side={props.side ?? "top"}>{label}</TooltipContent>
      )}
    </Tooltip>
  );
}

export function CompletedFileInfoSegmentGroup(props: {
  file: CompletedFileEntry | null;
  text: ReturnType<typeof getXiaText>;
  className?: string;
  onTranscodeFile?: (file: CompletedFileEntry) => void;
}) {
  const openFileLocation = useOpenLibraryFileLocation();
  const openPath = useOpenLibraryPath();
  const fileId = props.file?.id ?? "";
  const path = (props.file?.path ?? "").trim();
  const canOpenStoredFile = Boolean(fileId && props.file?.canDelete);
  const canOpenLocation = Boolean(props.file && (canOpenStoredFile || path));
  const isOpeningLocation = openFileLocation.isPending || openPath.isPending;
  const previewKind = props.file
    ? resolveCompletedPreviewKind(props.file)
    : "other";
  const footerItems = props.file
    ? resolveCompletedFileDetailFooterItems(props.file, props.text)
    : [];
  const footerInfoValue =
    footerItems.length > 0
      ? footerItems.map((item) => item.value).join(" · ")
      : props.text.completed.noSelectedFile;
  const footerInfoTooltip =
    footerItems.length > 0
      ? footerItems.map((item) => `${item.label} ${item.value}`).join("\n")
      : "";
  const canTranscode =
    Boolean(props.file && path && props.onTranscodeFile) &&
    (previewKind === "video" || previewKind === "audio");

  const handleOpenFolder = async () => {
    if (!props.file || !canOpenLocation) {
      return;
    }
    let openError: unknown = null;
    try {
      if (canOpenStoredFile) {
        await openFileLocation.mutateAsync({ fileId });
        return;
      }
      if (path) {
        await openPath.mutateAsync({ path });
        return;
      }
    } catch (error) {
      openError = error;
    }

    if (path && canOpenStoredFile) {
      try {
        await openPath.mutateAsync({ path });
        return;
      } catch (error) {
        openError = error;
      }
    }

    if (openError) {
      messageBus.publishToast({
        intent: "danger",
        title: props.text.actions.openDirectory,
        description: resolveUnknownErrorMessage(
          openError,
          props.text.common.unknown,
        ),
      });
    }
  };

  return (
    <div className={cn("flex justify-center", props.className)}>
      <div className="app-completed-detail-meta-bar flex max-w-full min-w-0 items-center overflow-hidden">
        <div className="flex min-w-0 flex-1 items-center overflow-hidden">
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="app-completed-detail-meta-cell inline-flex h-[var(--app-control-height-compact)] min-w-0 flex-1 items-center px-2.5 text-xs font-medium">
                <span className="truncate">{footerInfoValue}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent
              side="top"
              multiline={footerItems.length > 0}
              className="app-completed-detail-value-tooltip !max-w-[min(42rem,calc(100vw-1rem))] !px-2.5 !py-1.5"
            >
              {footerItems.length > 0
                ? footerInfoTooltip
                : props.text.completed.info}
            </TooltipContent>
          </Tooltip>
        </div>

        {(previewKind === "video" || previewKind === "audio") &&
        props.onTranscodeFile ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="app-completed-detail-meta-action !h-[var(--app-control-height-compact)] !w-[var(--app-control-height-compact)] shrink-0 rounded-none border-l border-border/70 p-0"
                aria-label={props.text.actions.transcode}
                title={props.text.actions.transcode}
                disabled={!canTranscode}
                onClick={() => {
                  if (props.file && canTranscode) {
                    props.onTranscodeFile?.(props.file);
                  }
                }}
              >
                <FileCog className="h-3.5 w-3.5" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">
              {props.text.actions.transcode}
            </TooltipContent>
          </Tooltip>
        ) : null}

        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="app-completed-detail-meta-action !h-[var(--app-control-height-compact)] !w-[var(--app-control-height-compact)] shrink-0 rounded-none border-l border-border/70 p-0"
              aria-label={props.text.actions.openDirectory}
              title={props.text.actions.openDirectory}
              disabled={!canOpenLocation || isOpeningLocation}
              onClick={() => void handleOpenFolder()}
            >
              {isOpeningLocation ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <FolderOpen className="h-3.5 w-3.5" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">
            {props.text.actions.openDirectory}
          </TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

export function SelectionCheckbox(props: { checked: boolean; className?: string }) {
  return (
    <span
      className={cn(
        "app-completed-selection-checkbox relative flex h-5 w-5 shrink-0 items-center justify-center backdrop-blur-sm",
        props.className,
      )}
      data-checked={props.checked ? "true" : undefined}
    >
      {props.checked ? (
        <span className="h-[0.45rem] w-[0.24rem] -translate-y-[0.03rem] rotate-45 border-r-[1.8px] border-b-[1.8px] border-current" />
      ) : null}
    </span>
  );
}

export function CompletedSubtitlePreview(props: {
  file: CompletedFileEntry;
  emptyLabel: string;
  tooLargeLabel: string;
}) {
  const [content, setContent] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    if (isCompletedPreviewTooLarge(props.file)) {
      setContent("");
      setLoading(false);
      setError(props.tooLargeLabel);
      return;
    }
    if (!props.file.previewURL) {
      setContent("");
      setLoading(false);
      setError("");
      return;
    }

    const controller = new AbortController();
    setLoading(true);
    setError("");
    fetch(props.file.previewURL, { signal: controller.signal })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`subtitle preview ${response.status}`);
        }
        const text = await readLimitedPreviewText(
          response,
          COMPLETED_TEXT_PREVIEW_MAX_BYTES,
        );
        setContent(text);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setContent("");
        setError(
          error instanceof PreviewTooLargeError
            ? props.tooLargeLabel
            : resolveUnknownErrorMessage(error, props.emptyLabel),
        );
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, [props.emptyLabel, props.file, props.file.id, props.file.previewURL, props.tooLargeLabel]);

  return (
    <div className="app-completed-preview-text-shell h-full w-full overflow-hidden">
      <div className="h-full overflow-auto px-4 py-3">
        {loading ? (
          <div className="flex h-full min-h-[16rem] items-center justify-center text-muted-foreground">
            <Loader2 className="h-5 w-5 animate-spin" />
          </div>
        ) : (
          <pre className="min-h-full whitespace-pre-wrap break-words font-mono text-xs leading-5 text-foreground">
            {content || error || props.emptyLabel}
          </pre>
        )}
      </div>
    </div>
  );
}

export function CompletedPreviewSurface(props: {
  file: CompletedFileEntry | null;
  coverURL?: string;
  emptyLabel: string;
  appName: string;
  pet?: Pet | null;
  petImageURL?: string;
  onPreviewPresentationModeChange?: (active: boolean) => void;
  onOpenPreviewDialog?: (file: CompletedFileEntry) => void;
}) {
  if (!props.file) {
    return (
      <div className="relative flex h-full min-h-[16rem] items-center justify-center">
        <PetDisplay
          pet={props.pet ?? null}
          imageUrl={props.petImageURL ?? ""}
          animation="waiting"
          alt={props.appName}
          fallbackSrc="/appicon.png"
        />
      </div>
    );
  }

  const text = getXiaText(getLanguage());
  const previewLabels = text.completed;
  const previewKind = resolveCompletedPreviewKind(props.file);
  const previewTooLarge = isCompletedPreviewTooLarge(props.file);
  if (!canPreviewCompletedFile(props.file)) {
    return (
      <div className="flex h-full min-h-[16rem] flex-col items-center justify-center gap-3 text-center text-muted-foreground">
        <img
          src="/appicon.png"
          alt={props.appName}
          className="app-completed-preview-icon h-14 w-14"
        />
        <div className="text-sm">
          {previewTooLarge ? text.completed.previewTooLarge : props.emptyLabel}
        </div>
      </div>
    );
  }

  if (previewKind === "video" && props.file.previewURL) {
    return (
      <MediaPreviewSurface
        kind="video"
        labels={previewLabels}
        className="h-full"
        mediaUrl={props.file.previewURL}
        title={props.file.name}
        persistKey={props.file.id || props.file.path || props.file.previewURL}
        posterUrl={
          props.file.coverURL || props.coverURL || DEFAULT_COVER_IMAGE_URL
        }
        durationMs={props.file.media?.durationMs}
        onPresentationModeChange={props.onPreviewPresentationModeChange}
      />
    );
  }

  if (previewKind === "audio" && props.file.previewURL) {
    return (
      <MediaPreviewSurface
        kind="audio"
        labels={previewLabels}
        className="h-full"
        audioPreview={
          <ListenLocalPreviewPlayer
            track={{
              id: props.file.id,
              title: props.file.title || props.file.name,
              author: props.file.author || props.file.libraryName,
              path: props.file.path,
              previewURL: props.file.previewURL,
              coverURL: props.file.coverURL || props.coverURL,
            }}
            text={text}
            persistKey={props.file.id || props.file.path || props.file.previewURL}
          />
        }
      />
    );
  }

  if (previewKind === "image") {
    const file = props.file;
    const previewSurface = (
      <MediaPreviewSurface
        kind="image"
        labels={previewLabels}
        className="h-full"
        mediaUrl={resolveCompletedImagePreviewURL(file)}
        title={file.name}
        imageAlt={file.name}
      />
    );
    if (!props.onOpenPreviewDialog) {
      return previewSurface;
    }
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            className="block h-full w-full cursor-zoom-in appearance-none border-0 bg-transparent p-0 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={file.name || text.completed.fileDetail}
            onClick={() => props.onOpenPreviewDialog?.(file)}
          >
            {previewSurface}
          </button>
        </TooltipTrigger>
        <TooltipContent side="top">
          {file.name || text.completed.fileDetail}
        </TooltipContent>
      </Tooltip>
    );
  }

  if (previewKind === "subtitle") {
    return (
      <MediaPreviewSurface
        kind="subtitle"
        labels={previewLabels}
        className="h-full"
        subtitlePreview={
          <CompletedSubtitlePreview
            file={props.file}
            emptyLabel={props.emptyLabel}
            tooLargeLabel={text.completed.previewTooLarge}
          />
        }
      />
    );
  }

  return (
    <div className="flex h-full min-h-[16rem] flex-col items-center justify-center gap-3 text-center text-muted-foreground">
      <img
        src="/appicon.png"
        alt={props.appName}
        className="app-completed-preview-icon h-14 w-14"
      />
      <div className="text-sm">{props.emptyLabel}</div>
    </div>
  );
}

function CompletedImagePreviewDialog(props: {
  file: CompletedFileEntry | null;
  open: boolean;
  text: ReturnType<typeof getXiaText>;
  onOpenChange: (open: boolean) => void;
}) {
  if (
    !props.file ||
    resolveCompletedPreviewKind(props.file) !== "image" ||
    !canPreviewCompletedFile(props.file)
  ) {
    return null;
  }

  const mediaUrl = resolveCompletedImagePreviewURL(props.file);
  return (
    <MediaPreviewDialog
      open={props.open && Boolean(mediaUrl)}
      onOpenChange={props.onOpenChange}
      dialogTitle={props.file.name || props.text.completed.fileDetail}
      description={props.file.path || props.file.libraryName || ""}
      labels={props.text.completed}
      kind="image"
      mediaUrl={mediaUrl}
      title={props.file.name}
      imageAlt={props.file.name}
      closeLabel={props.text.actions.close}
    />
  );
}

function buildFileDetailInfoRows(
  file: CompletedFileEntry,
  text: ReturnType<typeof getXiaText>,
): TaskDTOInfoRow[] {
  const labels = text.completed.taskDataFields;
  const infoLabel = resolveCompletedFileDetailInfo(file, text).join(" / ");
  const sourceLabel =
    file.operationName ||
    formatCompletedTranscodedFromLabel(text, file.sourceFileName) ||
    file.libraryName;
  return [
    {
      key: "name",
      label: labels.name,
      value: formatTaskDTOValue(file.name),
      always: true,
    },
    {
      label: text.completed.source,
      value: formatTaskDTOValue(sourceLabel),
      valueTooltip: sourceLabel,
    },
    {
      label: text.completed.fileInfo,
      value: formatTaskDTOValue(infoLabel),
    },
    {
      label: text.completed.fileFormat,
      value: resolveCompletedFileFormatLabel(file, text),
    },
    {
      label: text.completed.fileSize,
      value: file.sizeBytes > 0 ? formatBytes(file.sizeBytes) : "-",
    },
    {
      label: labels.inputPath,
      value: formatTaskDTOValue(file.path),
      valueTooltip: file.path,
    },
    {
      label: text.completed.updatedAt,
      value: formatTaskDTODateTimeValue(file.updatedAt),
      always: true,
    },
  ].filter((row) => row.always || row.value !== "-");
}

function resolveCompletedTaskHeaderStatusIcon(status?: string) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "succeeded":
      return CheckCircle2;
    case "failed":
      return XCircle;
    case "canceled":
      return CircleSlash;
    default:
      return Clock3;
  }
}

function resolveCompletedTaskHeaderStatusIconTone(status?: string) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "succeeded":
      return "app-completed-status-icon-success";
    case "failed":
      return "app-completed-status-icon-danger";
    case "canceled":
      return "app-completed-status-icon-warning";
    default:
      return "app-completed-status-icon-muted";
  }
}

function isResourceSniffDownloadOperation(operation: OperationListItemDTO) {
  const downloadMethod = (operation.request?.downloadMethod ?? "")
    .trim()
    .toLowerCase();
  if (
    downloadMethod === "resource-sniff" ||
    downloadMethod === "resource_sniff" ||
    downloadMethod === "sniff"
  ) {
    return true;
  }
  return [operation.request?.extractor, operation.platform].some((value) =>
    (value ?? "").trim().toLowerCase().startsWith("resource:"),
  );
}

function useCompletedTaskDetailFileGroups(
  task: CompletedTaskEntry,
  selectedPreviewFileId: string,
) {
  const groupedFiles = React.useMemo(() => {
    const map = new Map<CompletedPreviewGroupKind, CompletedFileEntry[]>();
    TASK_DETAIL_GROUP_ORDER.forEach((kind) => {
      map.set(kind, []);
    });
    task.files.forEach((file) => {
      const kind = resolveCompletedPreviewGroupKind(file);
      const current = map.get(kind);
      if (current) {
        current.push(file);
        return;
      }
      map.set(kind, [file]);
    });
    return map;
  }, [task.files]);
  const previewGroups = React.useMemo(
    () =>
      TASK_DETAIL_GROUP_ORDER.map((kind) => ({
        kind,
        files: groupedFiles.get(kind) ?? [],
      })).filter((group) => group.files.length > 0),
    [groupedFiles],
  );
  const visibleGroups = React.useMemo(
    () => previewGroups.slice(0, COMPLETED_TASK_FILE_TYPE_LIMIT),
    [previewGroups],
  );
  const visibleGroupKinds = React.useMemo(
    () => new Set(visibleGroups.map((group) => group.kind)),
    [visibleGroups],
  );

  const selectedFile =
    task.files.find(
      (file) =>
        file.id === selectedPreviewFileId &&
        visibleGroupKinds.has(resolveCompletedPreviewGroupKind(file)),
    ) ??
    visibleGroups[0]?.files[0] ??
    null;
  const activeGroup =
    visibleGroups.find((group) =>
      group.files.some((file) => file.id === selectedFile?.id),
    ) ??
    visibleGroups[0] ??
    null;

  return {
    groupedFiles,
    previewGroups: visibleGroups,
    selectedFile,
    activeGroup,
    activeGroupFiles: activeGroup?.files ?? [],
  };
}

function CompletedDetailInfoDialog(props: {
  text: ReturnType<typeof getXiaText>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  rows: TaskDTOInfoRow[];
  onRenameName?: () => void;
  renameNameDisabled?: boolean;
  renameLabel?: string;
}) {
  const handleCopyValue = React.useCallback(
    async (value: string) => {
      try {
        await copyTextToClipboard(value);
        messageBus.publishToast({
          id: "completed-detail-data-clipboard",
          intent: "success",
          title: props.text.completed.downloadUrlCopied,
          source: "xiadown.completed",
          autoCloseMs: 2200,
        });
      } catch (error) {
        messageBus.publishToast({
          id: "completed-detail-data-clipboard-failed",
          intent: "danger",
          title: props.text.completed.copyFailed,
          description: resolveUnknownErrorMessage(
            error,
            props.text.common.unknown,
          ),
          source: "xiadown.completed",
          autoCloseMs: 2600,
        });
      }
    },
    [props.text],
  );

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className="grid h-[min(30rem,calc(100vh-2rem))] w-[min(34rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden">
        <DialogHeader className="min-w-0">
          <DialogTitle className="truncate pr-6 text-left">
            {props.title}
          </DialogTitle>
          <DialogDescription className="sr-only">
            {props.description}
          </DialogDescription>
        </DialogHeader>
        <DialogScrollArea className="min-h-0">
          <DialogListCard className="app-completed-info-card shadow-none">
            <DialogListCardContent>
              {props.rows.map((row, index) => {
                const copyValue = row.copyValue;
                const canRenameName =
                  row.key === "name" && Boolean(props.onRenameName);
                const renameLabel =
                  props.renameLabel ?? props.text.completed.renameTask;
                return (
                  <div
                    key={`${row.label}-${index}`}
                    className="app-dialog-row grid grid-cols-[minmax(0,0.42fr)_minmax(0,0.58fr)] items-center gap-4 px-3 py-2.5 text-sm"
                  >
                    <span className="min-w-0 truncate text-left text-muted-foreground">
                      {row.label}
                    </span>
                    <div className="flex min-w-0 items-center justify-end gap-1.5">
                      {row.valueTooltip ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span
                              tabIndex={0}
                              className="min-w-0 cursor-help truncate rounded-sm text-right font-medium text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                            >
                              {row.value}
                            </span>
                          </TooltipTrigger>
                          <TooltipContent
                            side="top"
                            align="end"
                            sideOffset={6}
                            multiline
                            className="text-left"
                          >
                            {row.valueTooltip}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        <span className="min-w-0 truncate text-right font-medium text-foreground">
                          {row.value}
                        </span>
                      )}
                      {copyValue ? (
                        <Tooltip>
                          <TooltipTrigger asChild openOnFocus={false}>
                            <Button
                              type="button"
                              variant="ghost"
                              size="compactIcon"
                              className="app-completed-clipboard-action !h-6 !w-6 shrink-0"
                              aria-label={props.text.completed.copyDownloadUrl}
                              onClick={() => void handleCopyValue(copyValue)}
                            >
                              <Copy className="h-3.5 w-3.5" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top" align="center" sideOffset={6}>
                            {props.text.completed.copyDownloadUrl}
                          </TooltipContent>
                        </Tooltip>
                      ) : null}
                      {canRenameName ? (
                        <Tooltip>
                          <TooltipTrigger asChild openOnFocus={false}>
                            <Button
                              type="button"
                              variant="ghost"
                              size="compactIcon"
                              className="app-completed-clipboard-action !h-6 !w-6 shrink-0"
                              aria-label={renameLabel}
                              disabled={props.renameNameDisabled}
                              onClick={props.onRenameName}
                            >
                              <PencilLine className="h-3.5 w-3.5" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent side="top" align="center" sideOffset={6}>
                            {renameLabel}
                          </TooltipContent>
                        </Tooltip>
                      ) : null}
                    </div>
                  </div>
                );
              })}
            </DialogListCardContent>
          </DialogListCard>
        </DialogScrollArea>
        <DialogFooter className="shrink-0">
          <Button
            type="button"
            variant="outline"
            onClick={() => props.onOpenChange(false)}
          >
            {props.text.actions.close}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function CompletedTaskDetailHeaderMeta(props: {
  text: ReturnType<typeof getXiaText>;
  task: CompletedTaskEntry;
  className?: string;
}) {
  const resumeOperation = useResumeOperation();
  const sourceLabel = resolveCompletedTaskSourceLabel(props.task.operation);
  const sourceSiteKey = resolveSiteKeyForDomain(
    props.task.operation.domain,
  );
  const updatedLabel = props.task.updatedAt
    ? formatRelativeTime(props.task.updatedAt)
    : props.text.common.unknown;
  const taskStatus = (props.task.operation.status ?? "").trim().toLowerCase();
  const taskKind = (props.task.operation.kind ?? "").trim().toLowerCase();
  const isResourceSniffDownload =
    taskKind === "download" &&
    isResourceSniffDownloadOperation(props.task.operation);
  const canResumeTask =
    (taskStatus === "failed" || taskStatus === "canceled") &&
    (taskKind === "download" || taskKind === "transcode") &&
    !isResourceSniffDownload;
  const StatusIcon = resolveCompletedTaskHeaderStatusIcon(
    props.task.operation.status,
  );

  const handleResumeTask = async () => {
    if (!canResumeTask || resumeOperation.isPending) {
      return;
    }
    try {
      await resumeOperation.mutateAsync({
        operationId: props.task.operation.operationId,
      });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: props.text.actions.resume,
        description: resolveUnknownErrorMessage(
          error,
          props.text.common.unknown,
        ),
      });
    }
  };

  return (
    <div
      className={cn(
        "app-completed-detail-inline-meta flex min-w-0 items-center gap-2 text-xs font-medium",
        props.className,
      )}
    >
      <DetailValueTooltip label={props.text.completed.source}>
        <span
          className="app-completed-detail-inline-meta-item flex min-w-0 items-center gap-1.5 text-left transition focus-visible:outline-none"
        >
          {taskKind === "transcode" ? (
            <FileCog className="h-3.5 w-3.5 shrink-0" />
          ) : sourceSiteKey ? (
            <SiteBrandIcon
              siteKey={sourceSiteKey}
              fallback="none"
              className="h-3.5 w-3.5 shrink-0"
            />
          ) : null}
          <span className="truncate">
            {sourceLabel || props.text.common.unknown}
          </span>
        </span>
      </DetailValueTooltip>

      <DetailValueTooltip
        label={`${props.text.completed.taskStatus} / ${props.text.completed.updatedAt}`}
        value={`${resolveCompletedStatusLabel(props.text, props.task.operation.status)} ${updatedLabel}`}
      >
        <span
          className="app-completed-detail-inline-meta-item app-completed-detail-inline-status-time flex min-w-0 items-center gap-1.5 text-left transition focus-visible:outline-none"
        >
          <StatusIcon
            className={cn(
              "h-3.5 w-3.5 shrink-0",
              resolveCompletedTaskHeaderStatusIconTone(
                props.task.operation.status,
              ),
            )}
          />
          <span className="min-w-0 truncate">{updatedLabel}</span>
        </span>
      </DetailValueTooltip>

      {canResumeTask ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="app-completed-detail-inline-action !h-6 !w-6 shrink-0 p-0"
              aria-label={props.text.actions.resume}
              disabled={resumeOperation.isPending}
              onClick={() => void handleResumeTask()}
            >
              {resumeOperation.isPending ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <RotateCcw className="h-3.5 w-3.5" />
              )}
            </Button>
          </TooltipTrigger>
          <TooltipContent side="top">
            {props.text.actions.resume}
          </TooltipContent>
        </Tooltip>
      ) : null}
    </div>
  );
}

export function CompletedFileDetailHeaderMeta(props: {
  text: ReturnType<typeof getXiaText>;
  file: CompletedFileEntry;
  className?: string;
}) {
  const transcodeSourceLabel = formatCompletedTranscodedFromLabel(
    props.text,
    props.file.sourceFileName,
  );
  const sourceLabel =
    props.file.operationName || transcodeSourceLabel || props.file.libraryName;
  const sourceTooltipLabel = props.file.operationName
    ? props.text.views.tasks
    : transcodeSourceLabel
      ? props.text.completed.transcodedFrom
      : props.text.completed.source;
  const updatedLabel = props.file.updatedAt
    ? formatRelativeTime(props.file.updatedAt)
    : props.text.common.unknown;

  return (
    <div
      className={cn(
        "app-completed-detail-inline-meta flex min-w-0 items-center gap-2 text-xs font-medium",
        props.className,
      )}
    >
      <DetailValueTooltip label={sourceTooltipLabel} value={sourceLabel}>
        <span
          className="app-completed-detail-inline-meta-item flex min-w-0 items-center gap-1.5 text-left transition focus-visible:outline-none"
        >
          {transcodeSourceLabel ? (
            <FileCog className="h-3.5 w-3.5 shrink-0" />
          ) : null}
          <span className="truncate">
            {sourceLabel || props.text.common.unknown}
          </span>
        </span>
      </DetailValueTooltip>
      <DetailValueTooltip label={props.text.completed.updatedAt}>
        <span
          className="app-completed-detail-inline-meta-item flex min-w-0 items-center text-left transition focus-visible:outline-none"
        >
          <span className="truncate">{updatedLabel}</span>
        </span>
      </DetailValueTooltip>
    </div>
  );
}

export function CompletedTaskFilePicker(props: {
  text: ReturnType<typeof getXiaText>;
  task: CompletedTaskEntry;
  selectedPreviewFileId: string;
  onSelectedPreviewFileIdChange: (fileId: string) => void;
  className?: string;
}) {
  const {
    previewGroups,
    selectedFile,
    activeGroup,
    activeGroupFiles,
  } = useCompletedTaskDetailFileGroups(
    props.task,
    props.selectedPreviewFileId,
  );

  return (
    <div
      className={cn(
        "app-completed-task-file-picker overflow-hidden text-xs font-medium",
        props.className,
      )}
    >
      <div className="grid h-[var(--app-control-height-compact)] grid-cols-2">
        <div
          role="tablist"
          className="app-completed-task-file-tabs grid min-w-0 items-center overflow-hidden"
          style={{
            gridTemplateColumns: `repeat(${Math.max(previewGroups.length, 1)}, minmax(0, 1fr))`,
          }}
        >
          {previewGroups.map(({ kind, files }) => {
            const Icon = resolveCompletedPreviewGroupIcon(kind);
            const active = activeGroup?.kind === kind;
            return (
              <Tooltip key={kind}>
                <TooltipTrigger asChild>
                  <Button
                    type="button"
                    role="tab"
                    aria-selected={active}
                    variant="ghost"
                    size="compact"
                    className={cn(
                      "app-completed-task-file-tab !h-full w-full min-w-0 justify-center px-1 text-2xs",
                      active && "app-completed-task-file-tab-active",
                    )}
                    onClick={() =>
                      props.onSelectedPreviewFileIdChange(
                        files.find((file) =>
                          canPreviewCompletedFile(file),
                        )?.id ?? files[0].id,
                      )
                    }
                  >
                    <Icon className="!h-2.5 !w-2.5" />
                    <span className="tabular-nums">{files.length}</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  {resolveCompletedPreviewGroupLabel(kind, props.text)}
                </TooltipContent>
              </Tooltip>
            );
          })}
        </div>

        <div className="app-completed-task-file-select-slot flex min-w-0 items-center">
          <Select
            value={selectedFile?.id ?? activeGroupFiles[0]?.id ?? ""}
            onChange={(event) =>
              props.onSelectedPreviewFileIdChange(event.target.value)
            }
            disabled={activeGroupFiles.length === 0}
            className="app-completed-task-file-select !h-full w-full min-w-0 rounded-none border-0 bg-transparent px-2.5 pr-6 text-xs font-medium shadow-none"
          >
            {activeGroupFiles.length > 0 ? (
              activeGroupFiles.map((file, index) => (
                <option key={file.id} value={file.id}>
                  {file.name?.trim() ||
                    `${resolveCompletedPreviewGroupLabel(activeGroup?.kind ?? "other", props.text)} ${index + 1}`}
                </option>
              ))
            ) : (
              <option value="" />
            )}
          </Select>
        </div>
      </div>
    </div>
  );
}

export function CompletedTaskDetailHeader(props: {
  text: ReturnType<typeof getXiaText>;
  task: CompletedTaskEntry;
  coverURL: string;
  title: string;
  fallbackIcon: React.ReactNode;
  selectedPreviewFileId: string;
  onSelectedPreviewFileIdChange: (fileId: string) => void;
  onRenameTask?: (task: CompletedTaskEntry) => void;
  renameTaskDisabled?: boolean;
}) {
  const [taskInfoDialogOpen, setTaskInfoDialogOpen] = React.useState(false);
  const taskDTOInfoRows = React.useMemo(
    () =>
      buildTaskDTOInfoRows(
        props.task.operation,
        props.text,
        props.task.sourceFileName,
      ),
    [props.task.operation, props.task.sourceFileName, props.text],
  );
  const openTaskInfoDialog = () => setTaskInfoDialogOpen(true);
  const openRenameTaskDialog = () => {
    setTaskInfoDialogOpen(false);
    props.onRenameTask?.(props.task);
  };

  return (
    <>
      <div className="app-completed-inline-detail-header grid shrink-0 gap-3 border-b border-border/60 px-4 py-3">
        <div className="flex min-w-0 gap-2">
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="app-completed-detail-cover app-completed-detail-cover-button relative flex h-12 w-12 shrink-0 self-start items-center justify-center overflow-hidden transition focus-visible:outline-none"
                aria-label={props.text.completed.openTaskDto}
                onClick={openTaskInfoDialog}
              >
                {props.coverURL ? (
                  <img
                    src={props.coverURL}
                    alt=""
                    aria-hidden="true"
                    className="h-full w-full object-cover"
                    loading="lazy"
                    decoding="async"
                    draggable={false}
                  />
                ) : (
                  props.fallbackIcon
                )}
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">
              {props.text.completed.openTaskDto}
            </TooltipContent>
          </Tooltip>
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-start gap-1.5">
              <div className="app-completed-detail-title-shell relative min-w-0 flex-1">
                <div
                  className="app-completed-detail-title-text overflow-hidden break-words text-left text-sm font-semibold leading-5 text-foreground/82 transition-colors [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]"
                  aria-hidden="true"
                >
                  {props.title}
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      className="app-completed-detail-title-button absolute inset-0 focus-visible:outline-none"
                      aria-label={props.title}
                      onClick={openTaskInfoDialog}
                    />
                  </TooltipTrigger>
                  <TooltipContent
                    side="top"
                    align="start"
                    multiline
                    className="app-completed-detail-value-tooltip !max-w-[min(42rem,calc(100vw-1rem))] !px-2.5 !py-1.5"
                  >
                    {props.title}
                  </TooltipContent>
                </Tooltip>
              </div>
              {props.onRenameTask ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="app-completed-detail-inline-action !h-6 !w-6 shrink-0 p-0"
                      aria-label={props.text.completed.renameTask}
                      disabled={props.renameTaskDisabled}
                      onClick={() => props.onRenameTask?.(props.task)}
                    >
                      <PencilLine className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="top">
                    {props.text.completed.renameTask}
                  </TooltipContent>
                </Tooltip>
              ) : null}
            </div>
            <CompletedTaskDetailHeaderMeta
              text={props.text}
              task={props.task}
              className="mt-1.5"
            />
          </div>
        </div>
        <CompletedTaskFilePicker
          text={props.text}
          task={props.task}
          selectedPreviewFileId={props.selectedPreviewFileId}
          onSelectedPreviewFileIdChange={props.onSelectedPreviewFileIdChange}
        />
      </div>
      <CompletedDetailInfoDialog
        text={props.text}
        open={taskInfoDialogOpen}
        onOpenChange={setTaskInfoDialogOpen}
        title={props.text.completed.taskDtoTitle}
        description={props.text.completed.taskDtoDescription}
        rows={taskDTOInfoRows}
        onRenameName={
          props.onRenameTask ? openRenameTaskDialog : undefined
        }
        renameNameDisabled={props.renameTaskDisabled}
        renameLabel={props.text.completed.renameTask}
      />
    </>
  );
}

export function CompletedFileDetailHeader(props: {
  text: ReturnType<typeof getXiaText>;
  file: CompletedFileEntry;
  coverURL: string;
  title: string;
  fallbackIcon: React.ReactNode;
  onRenameFile?: (file: CompletedFileEntry) => void;
  renameFileDisabled?: boolean;
}) {
  const [fileInfoDialogOpen, setFileInfoDialogOpen] = React.useState(false);
  const fileInfoRows = React.useMemo(
    () => buildFileDetailInfoRows(props.file, props.text),
    [props.file, props.text],
  );
  const openFileInfoDialog = () => setFileInfoDialogOpen(true);
  const openRenameFileDialog = () => {
    setFileInfoDialogOpen(false);
    props.onRenameFile?.(props.file);
  };

  return (
    <>
      <div className="app-completed-inline-detail-header flex shrink-0 gap-2 border-b border-border/60 px-4 py-3">
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="app-completed-detail-cover app-completed-detail-cover-button relative flex h-12 w-12 shrink-0 self-start items-center justify-center overflow-hidden transition focus-visible:outline-none"
              aria-label={props.text.completed.fileDetail}
              onClick={openFileInfoDialog}
            >
              {props.coverURL ? (
                <img
                  src={props.coverURL}
                  alt=""
                  aria-hidden="true"
                  className="h-full w-full object-cover"
                  loading="lazy"
                  decoding="async"
                  draggable={false}
                />
              ) : (
                props.fallbackIcon
              )}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">
            {props.text.completed.fileDetail}
          </TooltipContent>
        </Tooltip>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-start gap-1.5">
            <div className="app-completed-detail-title-shell relative min-w-0 flex-1">
              <div
                className="app-completed-detail-title-text overflow-hidden break-words text-left text-sm font-semibold leading-5 text-foreground/82 transition-colors [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]"
                aria-hidden="true"
              >
                {props.title}
              </div>
              <Tooltip>
                <TooltipTrigger asChild>
                  <button
                    type="button"
                    className="app-completed-detail-title-button absolute inset-0 focus-visible:outline-none"
                    aria-label={props.title}
                    onClick={openFileInfoDialog}
                  />
                </TooltipTrigger>
                <TooltipContent
                  side="top"
                  align="start"
                  multiline
                  className="app-completed-detail-value-tooltip !max-w-[min(42rem,calc(100vw-1rem))] !px-2.5 !py-1.5"
                >
                  {props.title}
                </TooltipContent>
              </Tooltip>
            </div>
            {props.onRenameFile ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="app-completed-detail-inline-action !h-6 !w-6 shrink-0 p-0"
                    aria-label={props.text.completed.renameFile}
                    disabled={props.renameFileDisabled}
                    onClick={() => props.onRenameFile?.(props.file)}
                  >
                    <PencilLine className="h-3.5 w-3.5" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="top">
                  {props.text.completed.renameFile}
                </TooltipContent>
              </Tooltip>
            ) : null}
          </div>
          <CompletedFileDetailHeaderMeta
            text={props.text}
            file={props.file}
            className="mt-1.5"
          />
        </div>
      </div>
      <CompletedDetailInfoDialog
        text={props.text}
        open={fileInfoDialogOpen}
        onOpenChange={setFileInfoDialogOpen}
        title={props.text.completed.fileDetail}
        description={props.text.completed.fileDetail}
        rows={fileInfoRows}
        onRenameName={props.onRenameFile ? openRenameFileDialog : undefined}
        renameNameDisabled={props.renameFileDisabled}
        renameLabel={props.text.completed.renameFile}
      />
    </>
  );
}

export function CompletedTaskDetailContent(props: {
  text: ReturnType<typeof getXiaText>;
  appName: string;
  task: CompletedTaskEntry;
  selectedPreviewFileId: string;
  onTranscodeFile?: (file: CompletedFileEntry) => void;
  onPreviewPresentationModeChange?: (active: boolean) => void;
  pet: Pet | null;
  petImageURL: string;
}) {
  const { selectedFile } = useCompletedTaskDetailFileGroups(
    props.task,
    props.selectedPreviewFileId,
  );
  const [previewDialogOpen, setPreviewDialogOpen] = React.useState(false);

  React.useEffect(() => {
    setPreviewDialogOpen(false);
  }, [selectedFile?.id]);

  return (
    <>
      <div className="flex h-full min-h-0 flex-col">
        <div className="min-h-0 flex-1 overflow-hidden px-4 py-4">
          <CompletedPreviewSurface
            file={selectedFile}
            coverURL={props.task.coverURL}
            emptyLabel={
              selectedFile
                ? props.text.completed.noPreview
                : props.text.completed.noSelectedFile
            }
            appName={props.appName}
            pet={props.pet}
            petImageURL={props.petImageURL}
            onPreviewPresentationModeChange={
              props.onPreviewPresentationModeChange
            }
            onOpenPreviewDialog={() => setPreviewDialogOpen(true)}
          />
        </div>

        <div className="app-completed-detail-footer shrink-0 border-t border-border/60 px-4 pt-2.5 pb-3">
          <CompletedFileInfoSegmentGroup
            file={selectedFile}
            text={props.text}
            onTranscodeFile={props.onTranscodeFile}
          />
        </div>
      </div>
      <CompletedImagePreviewDialog
        file={selectedFile}
        open={previewDialogOpen}
        text={props.text}
        onOpenChange={setPreviewDialogOpen}
      />
    </>
  );
}

export function CompletedFileDetailContent(props: {
  text: ReturnType<typeof getXiaText>;
  appName: string;
  file: CompletedFileEntry;
  onTranscodeFile?: (file: CompletedFileEntry) => void;
  onPreviewPresentationModeChange?: (active: boolean) => void;
}) {
  const [previewDialogOpen, setPreviewDialogOpen] = React.useState(false);

  React.useEffect(() => {
    setPreviewDialogOpen(false);
  }, [props.file.id]);

  return (
    <>
      <div className="flex h-full min-h-0 flex-col">
        <div className="min-h-0 flex-1 overflow-hidden px-4 py-4">
          <CompletedPreviewSurface
            file={props.file}
            emptyLabel={props.text.completed.noPreview}
            appName={props.appName}
            onPreviewPresentationModeChange={
              props.onPreviewPresentationModeChange
            }
            onOpenPreviewDialog={() => setPreviewDialogOpen(true)}
          />
        </div>

        <div className="app-completed-detail-footer shrink-0 border-t border-border/60 px-4 pt-2.5 pb-3">
          <CompletedFileInfoSegmentGroup
            file={props.file}
            text={props.text}
            onTranscodeFile={props.onTranscodeFile}
          />
        </div>
      </div>
      <CompletedImagePreviewDialog
        file={props.file}
        open={previewDialogOpen}
        text={props.text}
        onOpenChange={setPreviewDialogOpen}
      />
    </>
  );
}
