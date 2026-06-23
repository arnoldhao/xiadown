import { useQueryClient } from "@tanstack/react-query";
import { Loader2, Wrench } from "lucide-react";
import * as React from "react";

import { FileRelinkDialog } from "@/app/main/file-relink/FileRelinkDialog";
import { getXiaText } from "@/features/xiadown/shared";
import { messageBus } from "@/shared/message";
import {
  invalidateLibraryQueries,
  selectLibraryDirectory,
  useApplyLibraryRelinks,
  useListMissingLibraryFiles,
  useScanMissingLibraryFiles,
} from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

import { resolveUnknownErrorMessage } from "@/app/main/helpers";

type ClearMissingFilesResponse = {
  checked?: number;
  removed?: number;
};

function formatCountMessage(template: string, count: number) {
  return template.replace("{count}", String(count));
}

export function CompletedFileMaintenanceControls(props: {
  text: ReturnType<typeof getXiaText>;
  httpBaseURL: string;
}) {
  const [action, setAction] = React.useState<"" | "verify" | "clear">("");
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const queryClient = useQueryClient();
  const missingFiles = useListMissingLibraryFiles();
  const scanMissing = useScanMissingLibraryFiles();
  const applyRelinks = useApplyLibraryRelinks();
  const [selectedMatches, setSelectedMatches] = React.useState<Record<string, string>>({});
  const [clearConfirming, setClearConfirming] = React.useState(false);

  const loadMissingFiles = React.useCallback(async () => {
    const result = await missingFiles.mutateAsync();
    setSelectedMatches({});
    setClearConfirming(false);
    scanMissing.reset();
    return result;
  }, [missingFiles, scanMissing]);

  const runClearMissingFiles = async () => {
    const baseURL = props.httpBaseURL.trim().replace(/\/+$/, "");
    if (!baseURL || action) {
      return;
    }
    setAction("clear");
    try {
      const response = await fetch(`${baseURL}/api/library/files/clear-missing`, {
        method: "POST",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`library file maintenance failed: ${response.status}`);
      }
      const result = (await response.json()) as ClearMissingFilesResponse;
      const removed = Number.isFinite(result.removed) ? Number(result.removed) : 0;
      messageBus.publishToast({
        intent: removed > 0 ? "success" : "info",
        description:
          removed > 0
            ? formatCountMessage(props.text.completed.clearMissingFilesRemovedToast, removed)
            : props.text.completed.clearMissingFilesNoneToast,
      });
      invalidateLibraryQueries(queryClient);
      if (dialogOpen) {
        await loadMissingFiles();
      }
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.fileMaintenanceFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    } finally {
      setAction("");
    }
  };

  const handleOpenRelinkDialog = async () => {
    if (action) {
      return;
    }
    setDialogOpen(true);
    setAction("verify");
    try {
      await loadMissingFiles();
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.fileMaintenanceFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    } finally {
      setAction("");
    }
  };

  const handleScanFolder = async () => {
    const currentMissing = missingFiles.data?.missing ?? [];
    const initialPath = currentMissing[0]?.oldPath ?? "";
    try {
      const selected = await selectLibraryDirectory(props.text.completed.relinkChooseFolderTitle, initialPath);
      if (!selected) {
        return;
      }
      setClearConfirming(false);
      const result = await scanMissing.mutateAsync({
        directory: selected,
        fileIds: currentMissing.map((file) => file.fileId),
      });
      const nextSelections: Record<string, string> = {};
      for (const match of result.matches) {
        if (!nextSelections[match.fileId]) {
          nextSelections[match.fileId] = match.newPath;
        }
      }
      setSelectedMatches(nextSelections);
      messageBus.publishToast({
        intent: result.matches.length > 0 ? "success" : "info",
        description:
          result.matches.length > 0
            ? formatCountMessage(props.text.completed.relinkScanFoundToast, result.matches.length)
            : props.text.completed.relinkScanNoneToast,
      });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.relinkScanFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    }
  };

  const handleApplySelectedMatches = async () => {
    const matches = Object.entries(selectedMatches)
      .filter(([, path]) => path.trim())
      .map(([fileId, path]) => ({ fileId, path }));
    if (matches.length === 0) {
      return;
    }
    setClearConfirming(false);
    try {
      const result = await applyRelinks.mutateAsync({ matches });
      messageBus.publishToast({
        intent: "success",
        description: formatCountMessage(props.text.completed.relinkApplySuccessToast, result.relinked),
      });
      await loadMissingFiles();
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.relinkApplyFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    }
  };

  const handleClearMissingFromDialog = async () => {
    if (!clearConfirming) {
      setClearConfirming(true);
      return;
    }
    await runClearMissingFiles();
    setClearConfirming(false);
  };

  return (
    <>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="app-completed-toolbar-button h-8 w-8 p-0"
            aria-label={props.text.completed.relinkDialogTitle}
            disabled={action !== ""}
            onClick={() => void handleOpenRelinkDialog()}
          >
            {action === "verify" ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Wrench className="h-4 w-4" />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          {props.text.completed.relinkDialogTitle}
        </TooltipContent>
      </Tooltip>
      <FileRelinkDialog
        open={dialogOpen}
        text={props.text}
        loading={missingFiles.isPending || action === "verify"}
        relinking={applyRelinks.isPending}
        scanning={scanMissing.isPending}
        clearing={action === "clear"}
        clearConfirming={clearConfirming}
        clearLabel={props.text.completed.clearMissingFiles}
        confirmClearLabel={props.text.completed.confirmClearMissingFiles}
        missing={missingFiles.data?.missing ?? []}
        matches={scanMissing.data?.matches ?? []}
        selectedMatches={selectedMatches}
        onOpenChange={setDialogOpen}
        onScanFolder={handleScanFolder}
        onApplyMatches={handleApplySelectedMatches}
        onClearMissing={handleClearMissingFromDialog}
      />
    </>
  );
}
