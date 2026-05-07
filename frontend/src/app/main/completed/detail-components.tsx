import { Copy, FolderOpen, Loader2, RotateCcw } from "lucide-react";
import * as React from "react";

import { CompletedVidstackPreview } from "@/app/main/CompletedVidstackPreview";
import { DreamFMLocalPreviewPlayer } from "@/app/main/DreamFM";
import { ConnectorBrandIcon } from "@/features/settings/connectors";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { DEFAULT_COVER_IMAGE_URL } from "@/shared/assets/default-cover";
import type { OperationListItemDTO } from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";
import { getLanguage } from "@/shared/i18n";
import { messageBus } from "@/shared/message";
import { useOpenLibraryFileLocation, useOpenLibraryPath, useResumeOperation } from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import { Card, CardContent } from "@/shared/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { Select } from "@/shared/ui/select";
import { PetDisplay } from "@/shared/ui/pet-player";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

import { canPreviewCompletedFile,formatRelativeTime,resolveCompletedFileDetailFooterMeta,resolveCompletedFileDetailInfo,resolveCompletedFileFooterTooltipLabels,resolveCompletedFileFormatLabel,resolveCompletedImagePreviewURL,resolveCompletedPreviewGroupIcon,resolveCompletedPreviewGroupKind,resolveCompletedPreviewGroupLabel,resolveCompletedPreviewKind,resolveCompletedStatusLabel,resolveCompletedTaskSourceLabel,resolveConnectorTypeForDomain,resolveOperationKindLabel,resolveStatusTone,resolveUnknownErrorMessage } from "@/app/main/helpers";
import type { CompletedFileEntry,CompletedPreviewGroupKind,CompletedTaskEntry } from "@/app/main/types";

const TASK_DETAIL_GROUP_ORDER: CompletedPreviewGroupKind[] = [
  "media",
  "subtitle",
  "image",
  "other",
];
const TASK_DETAIL_TAB_KINDS: CompletedPreviewGroupKind[] = [
  "media",
  "subtitle",
  "image",
];

type TaskDTOInfoRow = {
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

function buildTaskDTOInfoRows(
  operation: OperationListItemDTO,
  text: ReturnType<typeof getXiaText>,
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
  const errorCode = (operation.errorCode ?? "").trim();
  const errorMessage = (operation.errorMessage ?? "").trim();
  const failureReason = errorMessage || errorCode || text.common.unknown;
  return [
    {
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
    { label: labels.startedAt, value: formatTaskDTOValue(operation.startedAt) },
    { label: labels.finishedAt, value: formatTaskDTOValue(operation.finishedAt) },
    {
      label: labels.createdAt,
      value: formatTaskDTOValue(operation.createdAt),
      always: true,
    },
  ].filter((row) => row.always || row.value !== "-");
}

export function DetailValueTooltip(props: {
  label: string;
  side?: React.ComponentProps<typeof TooltipContent>["side"];
  children: React.ReactElement;
}) {
  const label = props.label?.trim();
  if (!label) {
    return props.children;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{props.children}</TooltipTrigger>
      <TooltipContent side={props.side ?? "top"}>{label}</TooltipContent>
    </Tooltip>
  );
}

export function CompletedFileInfoSegmentGroup(props: {
  file: CompletedFileEntry | null;
  text: ReturnType<typeof getXiaText>;
  className?: string;
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
  const footerMeta = props.file
    ? resolveCompletedFileDetailFooterMeta(props.file, props.text)
    : [];
  const footerTooltipLabels = resolveCompletedFileFooterTooltipLabels(
    previewKind,
    props.text,
  );

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
          {footerMeta.length > 0 ? (
            footerMeta.map((item, index) => (
              <DetailValueTooltip
                key={`${props.file?.id ?? "empty"}-footer-${index}`}
                label={footerTooltipLabels[index] ?? props.text.completed.info}
              >
                <span
                  className={cn(
                    "app-completed-detail-meta-cell inline-flex h-[var(--app-control-height-compact)] min-w-0 flex-[1_1_auto] items-center px-2.5 text-xs font-medium",
                    index > 0 && "border-l border-border/70",
                  )}
                >
                  <span className="truncate">{item}</span>
                </span>
              </DetailValueTooltip>
            ))
          ) : (
            <DetailValueTooltip label={props.text.completed.info}>
              <span className="app-completed-detail-meta-cell inline-flex h-[var(--app-control-height-compact)] min-w-0 flex-1 items-center px-2.5 text-xs font-medium">
                <span className="truncate">
                  {props.text.completed.noSelectedFile}
                </span>
              </span>
            </DetailValueTooltip>
          )}
        </div>

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
}) {
  const [content, setContent] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
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
        const text = await response.text();
        setContent(text);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }
        setContent("");
        setError(resolveUnknownErrorMessage(error, props.emptyLabel));
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      });

    return () => controller.abort();
  }, [props.emptyLabel, props.file.id, props.file.previewURL]);

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

  const previewKind = resolveCompletedPreviewKind(props.file);
  if (!canPreviewCompletedFile(props.file)) {
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

  if (previewKind === "video" && props.file.previewURL) {
    return (
      <CompletedVidstackPreview
        text={getXiaText(getLanguage())}
        mediaUrl={props.file.previewURL}
        title={props.file.name}
        posterUrl={
          props.file.coverURL || props.coverURL || DEFAULT_COVER_IMAGE_URL
        }
        durationMs={props.file.media?.durationMs}
      />
    );
  }

  if (previewKind === "audio" && props.file.previewURL) {
    return (
      <DreamFMLocalPreviewPlayer
        track={{
          id: props.file.id,
          title: props.file.title || props.file.name,
          author: props.file.author || props.file.libraryName,
          path: props.file.path,
          previewURL: props.file.previewURL,
          coverURL: props.file.coverURL || props.coverURL,
        }}
        text={getXiaText(getLanguage())}
      />
    );
  }

  if (previewKind === "image") {
    return (
      <div className="flex h-full min-h-[16rem] items-center justify-center">
        <img
          src={resolveCompletedImagePreviewURL(props.file)}
          alt={props.file.name}
          className="app-completed-preview-image max-h-full w-full object-contain"
        />
      </div>
    );
  }

  if (previewKind === "subtitle") {
    return (
      <CompletedSubtitlePreview
        file={props.file}
        emptyLabel={props.emptyLabel}
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

export function CompletedTaskDetailHeaderMeta(props: {
  text: ReturnType<typeof getXiaText>;
  task: CompletedTaskEntry;
  className?: string;
}) {
  const resumeOperation = useResumeOperation();
  const [taskInfoDialogOpen, setTaskInfoDialogOpen] = React.useState(false);
  const sourceLabel = resolveCompletedTaskSourceLabel(props.task.operation);
  const sourceConnectorType = resolveConnectorTypeForDomain(
    props.task.operation.domain,
  );
  const updatedLabel = props.task.updatedAt
    ? formatRelativeTime(props.task.updatedAt)
    : props.text.common.unknown;
  const taskStatus = (props.task.operation.status ?? "").trim().toLowerCase();
  const taskKind = (props.task.operation.kind ?? "").trim().toLowerCase();
  const canResumeTask =
    (taskStatus === "failed" || taskStatus === "canceled") &&
    (taskKind === "download" || taskKind === "transcode");
  const taskDTOInfoRows = React.useMemo(
    () => buildTaskDTOInfoRows(props.task.operation, props.text),
    [props.task.operation, props.text],
  );

  const openTaskInfoDialog = () => setTaskInfoDialogOpen(true);
  const handleCopyTaskDataValue = React.useCallback(
    async (value: string) => {
      try {
        await copyTextToClipboard(value);
        messageBus.publishToast({
          id: "completed-task-data-clipboard",
          intent: "success",
          title: props.text.completed.downloadUrlCopied,
          source: "xiadown.completed",
          autoCloseMs: 2200,
        });
      } catch (error) {
        messageBus.publishToast({
          id: "completed-task-data-clipboard-failed",
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
    <>
      <div
        className={cn(
          "app-completed-detail-meta-bar grid h-[var(--app-control-height-compact)] min-w-0 grid-cols-2 overflow-hidden text-xs font-medium",
          props.className,
        )}
      >
        <DetailValueTooltip label={props.text.completed.source}>
          <button
            type="button"
            className="app-completed-detail-meta-button flex h-full min-w-0 items-center gap-1.5 px-2.5 text-left transition focus-visible:outline-none"
            aria-label={props.text.completed.openTaskDto}
            onClick={openTaskInfoDialog}
          >
            {sourceConnectorType ? (
              <ConnectorBrandIcon
                connectorType={sourceConnectorType}
                fallback="none"
                className="h-3.5 w-3.5 shrink-0"
              />
            ) : null}
            <span className="truncate">
              {sourceLabel || props.text.common.unknown}
            </span>
          </button>
        </DetailValueTooltip>

        <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto_auto] border-l border-border/70">
          <DetailValueTooltip label={props.text.completed.updatedAt}>
            <button
              type="button"
              className="app-completed-detail-meta-button flex h-full min-w-0 items-center justify-start px-2.5 text-left transition focus-visible:outline-none"
              aria-label={props.text.completed.openTaskDto}
              onClick={openTaskInfoDialog}
            >
              <span className="truncate">{updatedLabel}</span>
            </button>
          </DetailValueTooltip>
          <DetailValueTooltip label={props.text.completed.taskStatus}>
            <button
              type="button"
              className="app-completed-detail-meta-button inline-flex h-full shrink-0 items-center border-l border-border/70 px-2 transition focus-visible:outline-none"
              aria-label={props.text.completed.openTaskDto}
              onClick={openTaskInfoDialog}
            >
              <span
                className={cn(
                  "inline-flex h-5 items-center rounded-md px-1.5 text-2xs font-semibold",
                  resolveStatusTone(props.task.operation.status),
                )}
              >
                {resolveCompletedStatusLabel(
                  props.text,
                  props.task.operation.status,
                )}
              </span>
            </button>
          </DetailValueTooltip>
          {canResumeTask ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="app-completed-detail-meta-action !h-[var(--app-control-height-compact)] !w-[var(--app-control-height-compact)] shrink-0 rounded-none border-l border-border/70 p-0"
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
      </div>

      <Dialog open={taskInfoDialogOpen} onOpenChange={setTaskInfoDialogOpen}>
        <DialogContent className="grid h-[min(30rem,calc(100vh-2rem))] w-[min(34rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle
              className="truncate pr-6 text-left"
            >
              {props.text.completed.taskDtoTitle}
            </DialogTitle>
          </DialogHeader>
          <div className="min-h-0 overflow-hidden">
            <Card className="app-completed-info-card h-full overflow-hidden shadow-none">
              <CardContent className="h-full overflow-y-auto overflow-x-hidden p-0">
                {taskDTOInfoRows.map((row, index) => {
                  const copyValue = row.copyValue;
                  return (
                    <div
                      key={row.label}
                      className={cn(
                        "grid grid-cols-[minmax(0,0.42fr)_minmax(0,0.58fr)] items-center gap-4 px-3 py-2.5 text-sm",
                        index > 0 && "border-t border-border/70",
                      )}
                    >
                      <span
                        className="min-w-0 truncate text-left text-muted-foreground"
                      >
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
                                onClick={() =>
                                  void handleCopyTaskDataValue(copyValue)
                                }
                              >
                                <Copy className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent side="top" align="center" sideOffset={6}>
                              {props.text.completed.copyDownloadUrl}
                            </TooltipContent>
                          </Tooltip>
                        ) : null}
                      </div>
                    </div>
                  );
                })}
              </CardContent>
            </Card>
          </div>
          <DialogFooter className="shrink-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setTaskInfoDialogOpen(false)}
            >
              {props.text.actions.close}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function CompletedFileDetailHeaderMeta(props: {
  text: ReturnType<typeof getXiaText>;
  file: CompletedFileEntry;
  className?: string;
}) {
  const formatLabel = resolveCompletedFileFormatLabel(props.file, props.text);
  const infoLabel = resolveCompletedFileDetailInfo(props.file, props.text).join(
    " / ",
  );
  const updatedLabel = props.file.updatedAt
    ? formatRelativeTime(props.file.updatedAt)
    : props.text.common.unknown;

  return (
    <div
      className={cn(
        "app-completed-detail-meta-bar grid h-[var(--app-control-height-compact)] min-w-0 grid-cols-2 overflow-hidden text-xs font-medium",
        props.className,
      )}
    >
      <DetailValueTooltip label={props.text.completed.fileInfo}>
        <div className="app-completed-detail-meta-cell flex min-w-0 items-center px-2.5">
          <span className="truncate">
            {infoLabel || props.text.common.unknown}
          </span>
        </div>
      </DetailValueTooltip>
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] border-l border-border/70">
        <DetailValueTooltip label={props.text.completed.updatedAt}>
          <span className="app-completed-detail-meta-cell flex h-full min-w-0 items-center justify-start px-2.5 text-left">
            <span className="truncate">{updatedLabel}</span>
          </span>
        </DetailValueTooltip>
        <DetailValueTooltip label={props.text.completed.fileFormat}>
          <span className="app-completed-detail-meta-cell inline-flex h-full max-w-[5.5rem] shrink-0 items-center border-l border-border/70 px-2.5">
            <span className="truncate">{formatLabel}</span>
          </span>
        </DetailValueTooltip>
      </div>
    </div>
  );
}

export function CompletedTaskDetailContent(props: {
  text: ReturnType<typeof getXiaText>;
  appName: string;
  task: CompletedTaskEntry;
  selectedPreviewFileId: string;
  onSelectedPreviewFileIdChange: (fileId: string) => void;
  pet: Pet | null;
  petImageURL: string;
}) {
  const groupedFiles = React.useMemo(() => {
    const map = new Map<CompletedPreviewGroupKind, CompletedFileEntry[]>();
    TASK_DETAIL_GROUP_ORDER.forEach((kind) => {
      map.set(kind, []);
    });
    props.task.files.forEach((file) => {
      const kind = resolveCompletedPreviewGroupKind(file);
      const current = map.get(kind);
      if (current) {
        current.push(file);
        return;
      }
      map.set(kind, [file]);
    });
    return map;
  }, [props.task.files]);
  const previewGroups = React.useMemo(
    () =>
      TASK_DETAIL_GROUP_ORDER.map((kind) => ({
        kind,
        files: groupedFiles.get(kind) ?? [],
      })).filter((group) => group.files.length > 0),
    [groupedFiles],
  );

  const selectedFile =
    props.task.files.find((file) => file.id === props.selectedPreviewFileId) ??
    previewGroups[0]?.files[0] ??
    null;
  const activeGroup =
    previewGroups.find((group) =>
      group.files.some((file) => file.id === selectedFile?.id),
    ) ??
    previewGroups[0] ??
    null;
  const activeGroupFiles = activeGroup?.files ?? [];

  return (
    <>
      <div className="flex h-full min-h-0 flex-col">
        <div className="shrink-0 px-4 py-4">
          <div className="app-completed-task-file-picker overflow-hidden text-xs font-medium">
            <div className="grid h-[var(--app-control-height-compact)] grid-cols-2">
              <div
                role="tablist"
                className="app-completed-task-file-tabs grid min-w-0 grid-cols-3 items-center overflow-hidden"
              >
                {TASK_DETAIL_TAB_KINDS.map((kind) => {
                  const files = groupedFiles.get(kind) ?? [];
                  const Icon = resolveCompletedPreviewGroupIcon(kind);
                  const active = activeGroup?.kind === kind;
                  const enabled = files.length > 0;
                  return (
                    <Tooltip key={kind}>
                      <TooltipTrigger asChild>
                        <Button
                          type="button"
                          role="tab"
                          aria-selected={enabled && active}
                          aria-disabled={!enabled}
                          disabled={!enabled}
                          variant="ghost"
                          size="compact"
                          className={cn(
                            "app-completed-task-file-tab !h-full w-full min-w-0 justify-center px-1 text-2xs disabled:pointer-events-auto",
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
        </div>

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
          />
        </div>

        <div className="app-completed-detail-footer shrink-0 border-t border-border/60 px-4 pt-2.5 pb-3">
          <CompletedFileInfoSegmentGroup
            file={selectedFile}
            text={props.text}
          />
        </div>
      </div>
    </>
  );
}

export function CompletedFileDetailContent(props: {
  text: ReturnType<typeof getXiaText>;
  appName: string;
  file: CompletedFileEntry;
}) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="min-h-0 flex-1 overflow-hidden px-4 py-4">
        <CompletedPreviewSurface
          file={props.file}
          emptyLabel={props.text.completed.noPreview}
          appName={props.appName}
        />
      </div>

      <div className="app-completed-detail-footer shrink-0 border-t border-border/60 px-4 pt-2.5 pb-3">
        <CompletedFileInfoSegmentGroup file={props.file} text={props.text} />
      </div>
    </div>
  );
}
