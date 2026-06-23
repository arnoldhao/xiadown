import { Check, CheckCircle2, FolderOpen, Loader2, Trash2 } from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogListCard,
  DialogListCardContent,
  DialogRow,
  DialogTitle,
} from "@/shared/ui/dialog";
import { formatBytes } from "@/shared/utils/formatBytes";
import { getPathBaseName } from "@/shared/utils/resourceHelpers";
import type { LibraryRelinkMatchDTO, MissingLibraryFileDTO } from "@/shared/contracts/library";

export function FileRelinkDialog(props: {
  open: boolean;
  text: ReturnType<typeof getXiaText>;
  loading: boolean;
  relinking: boolean;
  scanning: boolean;
  clearing: boolean;
  clearConfirming: boolean;
  clearLabel: string;
  confirmClearLabel: string;
  missing: MissingLibraryFileDTO[];
  matches: LibraryRelinkMatchDTO[];
  selectedMatches: Record<string, string>;
  onOpenChange: (open: boolean) => void;
  onScanFolder: () => void;
  onApplyMatches: () => void;
  onClearMissing: () => void;
}) {
  const firstMatchByFileId = React.useMemo(() => {
    const map = new Map<string, LibraryRelinkMatchDTO>();
    for (const match of props.matches) {
      if (!map.has(match.fileId)) {
        map.set(match.fileId, match);
      }
    }
    return map;
  }, [props.matches]);
  const busy = props.loading || props.scanning || props.relinking || props.clearing;
  const selectedCount = Object.values(props.selectedMatches).filter((path) => path.trim()).length;
  const applyEnabled = selectedCount > 0 && !busy;

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent
        onEscapeKeyDown={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
        className="grid max-h-[min(38rem,calc(100vh-2rem))] w-[min(42rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)] overflow-hidden"
      >
        <DialogHeader>
          <DialogTitle className="text-left">{props.text.completed.relinkDialogTitle}</DialogTitle>
        </DialogHeader>
        <div className="min-h-0">
          {props.loading ? (
            <div className="flex min-h-48 items-center justify-center text-muted-foreground">
              <Loader2 className="h-5 w-5 animate-spin" />
            </div>
          ) : props.missing.length === 0 ? (
            <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-muted-foreground">
              <CheckCircle2 className="h-8 w-8" />
              <div className="text-sm">{props.text.completed.relinkNoMissing}</div>
            </div>
          ) : (
            <>
              <DialogListCard className="max-h-[min(24rem,calc(100vh-13rem))] overflow-hidden shadow-none">
                <div className="app-file-relink-scroll-area max-h-[min(24rem,calc(100vh-13rem))]">
                  <DialogListCardContent>
                    {props.missing.map((file, index) => (
                      <FileRelinkRow
                        key={file.fileId}
                        file={file}
                        text={props.text}
                        divider={index > 0}
                        scanning={props.scanning}
                        match={firstMatchByFileId.get(file.fileId) ?? null}
                        selectedPath={props.selectedMatches[file.fileId] ?? ""}
                      />
                    ))}
                  </DialogListCardContent>
                </div>
              </DialogListCard>
              <div className="mt-3 flex flex-wrap justify-center gap-2">
                <Button
                  type="button"
                  size="compact"
                  disabled={busy}
                  onClick={props.onScanFolder}
                >
                  {props.scanning ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <FolderOpen className="h-4 w-4" />
                  )}
                  {props.text.completed.relinkScanFolder}
                </Button>
                <Button
                  type="button"
                  size="compact"
                  variant={applyEnabled ? "default" : "outline"}
                  disabled={!applyEnabled}
                  onClick={props.onApplyMatches}
                >
                  {props.relinking ? <Loader2 className="h-4 w-4 animate-spin" /> : <Check className="h-4 w-4" />}
                  {props.text.completed.relinkApply}
                </Button>
                <Button
                  type="button"
                  size="compact"
                  variant="destructive"
                  disabled={busy}
                  onClick={props.onClearMissing}
                >
                  {props.clearing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                  {props.clearConfirming ? props.confirmClearLabel : props.clearLabel}
                </Button>
              </div>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function FileRelinkRow(props: {
  file: MissingLibraryFileDTO;
  text: ReturnType<typeof getXiaText>;
  divider: boolean;
  scanning: boolean;
  match: LibraryRelinkMatchDTO | null;
  selectedPath: string;
}) {
  const hasNewPath = props.selectedPath.trim() !== "";
  const statusLabel = props.scanning
    ? props.text.completed.relinkStatusChecking
    : hasNewPath
      ? props.text.completed.relinkStatusNewPath
      : props.text.completed.relinkStatusMissing;
  const title = hasNewPath
    ? props.selectedPath
    : props.file.name || getPathBaseName(props.file.oldPath);
  const statusClassName = props.scanning
    ? "bg-amber-500/15 text-amber-700 dark:text-amber-300"
    : hasNewPath
      ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-300"
      : "bg-destructive/10 text-destructive";
  const sizeLabel =
    props.match?.sizeBytes || props.file.sizeBytes
      ? formatBytes(props.match?.sizeBytes ?? props.file.sizeBytes)
      : "";

  return (
    <DialogRow
      className={cn("block p-0", props.divider ? "border-t border-border/60" : "")}
    >
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-3 px-3 py-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className={cn(
              "shrink-0 rounded-md px-1.5 py-0.5 text-2xs font-medium",
              statusClassName,
            )}
          >
            {props.scanning ? <Loader2 className="mr-1 inline h-3 w-3 animate-spin" /> : null}
            {statusLabel}
          </span>
          <span className="min-w-0 truncate text-sm font-medium" title={title}>
            {title}
          </span>
        </div>
        <div className="flex shrink-0 items-center justify-end gap-2 text-xs text-muted-foreground">
          {props.file.format ? (
            <span className="rounded-md bg-muted px-1.5 py-0.5 uppercase">
              {props.file.format}
            </span>
          ) : null}
          {sizeLabel ? <span>{sizeLabel}</span> : null}
        </div>
      </div>
    </DialogRow>
  );
}
