import * as React from "react";
import { Copy } from "lucide-react";

import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Button } from "@/shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { cn } from "@/lib/utils";

import {
  MediaPreviewSurface,
  type MediaPreviewSurfaceProps,
} from "./MediaPreviewSurface";

export type MediaPreviewDialogProps = MediaPreviewSurfaceProps & {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  dialogTitle: string;
  description?: React.ReactNode;
  actionSlot?: React.ReactNode;
  actionsClassName?: string;
  closeLabel?: string;
  descriptionCopyLabel?: string;
  descriptionCopyValue?: string;
  descriptionClassName?: string;
  dialogClassName?: string;
  onDescriptionCopy?: (value: string) => Promise<void> | void;
  preventDismiss?: boolean;
  showCloseButton?: boolean;
  stageClassName?: string;
};

async function copyTextToClipboard(value: string) {
  const text = value.trim();
  if (!text) {
    return;
  }
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
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

export function MediaPreviewDialog(props: MediaPreviewDialogProps) {
  const isWide =
    props.kind === "video" || props.kind === "live" || props.kind === "flv";
  const preventDismiss = props.preventDismiss ?? false;
  const showCloseButton = props.showCloseButton ?? false;
  const descriptionCopyValue = props.descriptionCopyValue?.trim() ?? "";
  const descriptionCopyLabel = props.descriptionCopyLabel?.trim() || "Copy";
  const [presentationModeActive, setPresentationModeActive] =
    React.useState(false);

  const handleCopyDescription = React.useCallback(async () => {
    if (!descriptionCopyValue) {
      return;
    }
    if (props.onDescriptionCopy) {
      await props.onDescriptionCopy(descriptionCopyValue);
      return;
    }
    await copyTextToClipboard(descriptionCopyValue);
  }, [descriptionCopyValue, props.onDescriptionCopy]);

  const handlePresentationModeChange = React.useCallback(
    (active: boolean) => {
      setPresentationModeActive(active);
      props.onPresentationModeChange?.(active);
    },
    [props.onPresentationModeChange],
  );

  React.useEffect(() => {
    if (!props.open) {
      setPresentationModeActive(false);
    }
  }, [props.open]);

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        showCloseButton={showCloseButton}
        onEscapeKeyDown={(event) => {
          if (preventDismiss) {
            event.preventDefault();
          }
        }}
        onInteractOutside={(event) => {
          if (preventDismiss) {
            event.preventDefault();
          }
        }}
        onPointerDownOutside={(event) => {
          if (preventDismiss) {
            event.preventDefault();
          }
        }}
        className={cn(
          "app-media-preview-dialog min-w-0 max-w-none",
          isWide
            ? "w-[min(56rem,calc(100vw-2rem))]"
            : "w-[min(42rem,calc(100vw-2rem))]",
          props.dialogClassName,
        )}
        data-preview-presentation={
          presentationModeActive ? "true" : undefined
        }
      >
        <DialogHeader
          className={cn(
            "min-w-0 text-left",
            showCloseButton && "pr-8",
          )}
        >
          <DialogTitle className="min-w-0 truncate">
            {props.dialogTitle}
          </DialogTitle>
          {props.description ? (
            <div className="flex min-w-0 items-center gap-1.5">
              <DialogDescription
                className={cn(
                  "app-media-preview-dialog-url block min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap",
                  props.descriptionClassName,
                )}
              >
                {props.description}
              </DialogDescription>
              {descriptionCopyValue ? (
                <Tooltip>
                  <TooltipTrigger asChild openOnFocus={false}>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compactIcon"
                      className="h-6 w-6 shrink-0 rounded-full"
                      aria-label={descriptionCopyLabel}
                      title={descriptionCopyLabel}
                      onClick={() => void handleCopyDescription()}
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="top">
                    {descriptionCopyLabel}
                  </TooltipContent>
                </Tooltip>
              ) : null}
            </div>
          ) : null}
        </DialogHeader>
        <MediaPreviewSurface
          {...props}
          onPresentationModeChange={handlePresentationModeChange}
          className={cn(
            "app-media-preview-dialog-stage",
            props.stageClassName,
          )}
        />
        {props.actionSlot || props.closeLabel ? (
          <div className="app-dialog-footer flex flex-nowrap items-center justify-center gap-2">
            <div
              className={cn(
                "app-media-preview-actions inline-flex max-w-full flex-nowrap items-center justify-center gap-2",
                props.actionsClassName,
              )}
            >
              {props.actionSlot}
              {props.closeLabel ? (
                <DialogClose asChild>
                  <Button type="button" variant="outline">
                    {props.closeLabel}
                  </Button>
                </DialogClose>
              ) : null}
            </div>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
