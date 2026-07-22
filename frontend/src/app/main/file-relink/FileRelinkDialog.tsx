import { Check, CheckCircle2, FolderOpen, Loader2, Trash2 } from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { Badge } from "@/shared/ui/badge";
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
import { StatusBadge } from "@/shared/ui/status-badge";

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
        className="app-file-relink-dialog grid max-w-none overflow-hidden"
      >
        <DialogHeader>
          <DialogTitle className="app-file-relink-dialog-title">{props.text.completed.relinkDialogTitle}</DialogTitle>
        </DialogHeader>
        <div className="min-h-0">
          {props.loading ? (
            <div className="app-file-relink-feedback flex min-h-48 items-center justify-center">
              <Loader2 className="h-5 w-5 app-motion-spin" />
            </div>
          ) : props.missing.length === 0 ? (
            <div className="app-file-relink-feedback flex min-h-48 flex-col items-center justify-center gap-3">
              <CheckCircle2 className="h-8 w-8" />
              <div>{props.text.completed.relinkNoMissing}</div>
            </div>
          ) : (
            <>
              <DialogListCard className="app-file-relink-list overflow-hidden">
                <div className="app-file-relink-scroll-area">
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
                    <Loader2 className="h-4 w-4 app-motion-spin" />
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
                  {props.relinking ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Check className="h-4 w-4" />}
                  {props.text.completed.relinkApply}
                </Button>
                <Button
                  type="button"
                  size="compact"
                  variant="destructive"
                  disabled={busy}
                  onClick={props.onClearMissing}
                >
                  {props.clearing ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <Trash2 className="h-4 w-4" />}
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
  const statusTone = props.scanning
    ? "busy"
    : hasNewPath
      ? "success"
      : "danger";
  const sizeLabel =
    props.match?.sizeBytes || props.file.sizeBytes
      ? formatBytes(props.match?.sizeBytes ?? props.file.sizeBytes)
      : "";

  return (
    <DialogRow className="app-file-relink-row block p-0" data-divider={props.divider || undefined}>
      <div className="app-file-relink-grid grid min-w-0 items-center gap-3 px-3 py-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <StatusBadge
            tone={statusTone}
            icon={props.scanning ? <Loader2 className="app-motion-spin" /> : undefined}
          >
            {statusLabel}
          </StatusBadge>
          <span className="app-file-relink-title min-w-0 truncate" title={title}>
            {title}
          </span>
        </div>
        <div className="app-file-relink-meta flex shrink-0 items-center justify-end gap-2">
          {props.file.format ? (
            <Badge variant="subtle" className="app-file-relink-format">
              {props.file.format}
            </Badge>
          ) : null}
          {sizeLabel ? <span>{sizeLabel}</span> : null}
        </div>
      </div>
    </DialogRow>
  );
}
